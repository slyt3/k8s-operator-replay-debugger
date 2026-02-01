package recorder

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/slyt3/kubestep/internal/assert"
	"github.com/slyt3/kubestep/pkg/storage"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

const (
	maxSessionIDLength = 100
	maxRetries         = 3
	maxActorIDLength   = 256
	defaultActorID     = "unknown"
)

// RecordingClient wraps a Kubernetes client to record all operations.
// Rule 6: Minimal scope, all fields private.
type RecordingClient struct {
	client      kubernetes.Interface
	db          *storage.Database
	sessionID   string
	sequenceNum int64
	enabled     bool
	maxSequence int64
	actorID     string
}

// Config holds recorder configuration.
// Rule 3: Pre-allocated configuration, no dynamic allocation.
type Config struct {
	Client      kubernetes.Interface
	Database    *storage.Database
	SessionID   string
	MaxSequence int64
	ActorID     string
}

// NewRecordingClient creates a new recording client wrapper.
// Rule 5: Multiple assertions for validation.
func NewRecordingClient(cfg Config) (*RecordingClient, error) {
	err := assert.AssertNotNil(cfg.Client, "client")
	if err != nil {
		return nil, err
	}

	err = assert.AssertNotNil(cfg.Database, "database")
	if err != nil {
		return nil, err
	}

	err = assert.AssertStringNotEmpty(cfg.SessionID, "session_id")
	if err != nil {
		return nil, err
	}

	err = assert.AssertInRange(
		len(cfg.SessionID),
		1,
		maxSessionIDLength,
		"session_id length",
	)
	if err != nil {
		return nil, err
	}

	if cfg.MaxSequence <= 0 {
		cfg.MaxSequence = 1000000
	}

	if len(cfg.ActorID) == 0 {
		cfg.ActorID = defaultActorID
	}

	err = assert.AssertInRange(
		len(cfg.ActorID),
		1,
		maxActorIDLength,
		"actor_id length",
	)
	if err != nil {
		return nil, err
	}

	return &RecordingClient{
		client:      cfg.Client,
		db:          cfg.Database,
		sessionID:   cfg.SessionID,
		sequenceNum: 0,
		enabled:     true,
		maxSequence: cfg.MaxSequence,
		actorID:     cfg.ActorID,
	}, nil
}

// Enable turns recording on.
func (r *RecordingClient) Enable() error {
	err := assert.AssertNotNil(r, "recorder")
	if err != nil {
		return err
	}

	r.enabled = true
	return nil
}

// Disable turns recording off.
func (r *RecordingClient) Disable() error {
	err := assert.AssertNotNil(r, "recorder")
	if err != nil {
		return err
	}

	r.enabled = false
	return nil
}

// GetClient returns the wrapped Kubernetes client.
func (r *RecordingClient) GetClient() kubernetes.Interface {
	return r.client
}

// recordOperation stores an operation in the database.
// Rule 2: Bounded sequence number check.
// Rule 4: Function under 60 lines.
func (r *RecordingClient) recordOperation(
	opType storage.OperationType,
	kind string,
	namespace string,
	name string,
	obj runtime.Object,
	err error,
	duration time.Duration,
) error {
	validationErr := assert.AssertNotNil(r, "recorder")
	if validationErr != nil {
		return validationErr
	}

	if !r.enabled {
		return nil
	}

	if r.sequenceNum >= r.maxSequence {
		return fmt.Errorf("max sequence number reached: %d", r.maxSequence)
	}

	r.sequenceNum = r.sequenceNum + 1

	var resourceData string
	if obj != nil {
		jsonBytes, marshalErr := json.Marshal(obj)
		if marshalErr != nil {
			resourceData = fmt.Sprintf("marshal error: %v", marshalErr)
		} else {
			resourceData = string(jsonBytes)
		}
	}

	var errorMsg string
	if err != nil {
		errorMsg = err.Error()
	}

	uid, resourceVersion, generation := extractObjectMetadata(obj)
	verb := string(opType)

	op := &storage.Operation{
		SessionID:       r.sessionID,
		SequenceNumber:  r.sequenceNum,
		Timestamp:       time.Now(),
		OperationType:   opType,
		ResourceKind:    kind,
		Namespace:       namespace,
		Name:            name,
		ResourceData:    resourceData,
		Error:           errorMsg,
		DurationMs:      duration.Milliseconds(),
		ActorID:         r.actorID,
		UID:             uid,
		ResourceVersion: resourceVersion,
		Generation:      generation,
		Verb:            verb,
	}

	insertErr := r.db.InsertOperation(op)
	if insertErr != nil {
		return fmt.Errorf("failed to record operation: %w", insertErr)
	}

	return nil
}

// RecordGet records a GET operation with timing.
// Rule 7: All return values checked.
func (r *RecordingClient) RecordGet(
	ctx context.Context,
	kind string,
	namespace string,
	name string,
	opts metav1.GetOptions,
) (runtime.Object, error) {
	err := assert.AssertNotNil(r, "recorder")
	if err != nil {
		return nil, err
	}

	err = assert.AssertStringNotEmpty(kind, "resource kind")
	if err != nil {
		return nil, err
	}

	start := time.Now()

	var obj runtime.Object
	var getErr error

	switch kind {
	case "Pod":
		obj, getErr = r.client.CoreV1().Pods(namespace).Get(ctx, name, opts)
	case "Service":
		obj, getErr = r.client.CoreV1().Services(namespace).Get(ctx, name, opts)
	case "Deployment":
		obj, getErr = r.client.AppsV1().Deployments(namespace).Get(ctx, name, opts)
	case "ConfigMap":
		obj, getErr = r.client.CoreV1().ConfigMaps(namespace).Get(ctx, name, opts)
	case "Secret":
		obj, getErr = r.client.CoreV1().Secrets(namespace).Get(ctx, name, opts)
	default:
		return nil, fmt.Errorf("unsupported resource kind: %s", kind)
	}

	duration := time.Since(start)

	recordErr := r.recordOperation(
		storage.OperationGet,
		kind,
		namespace,
		name,
		obj,
		getErr,
		duration,
	)
	if recordErr != nil {
		return obj, fmt.Errorf("record failed: %w (original error: %v)",
			recordErr, getErr)
	}

	return obj, getErr
}

// RecordCreate records a CREATE operation with timing.
// Rule 7: All return values checked.
func (r *RecordingClient) RecordCreate(
	ctx context.Context,
	kind string,
	namespace string,
	obj runtime.Object,
	opts metav1.CreateOptions,
) (runtime.Object, error) {
	err := assert.AssertNotNil(r, "recorder")
	if err != nil {
		return nil, err
	}

	err = assert.AssertStringNotEmpty(kind, "resource kind")
	if err != nil {
		return nil, err
	}

	err = assert.AssertStringNotEmpty(namespace, "namespace")
	if err != nil {
		return nil, err
	}

	err = assert.AssertNotNil(obj, "object")
	if err != nil {
		return nil, err
	}

	if kind != "ConfigMap" && kind != "Secret" {
		return nil, fmt.Errorf("unsupported resource kind: %s", kind)
	}

	name, err := extractObjectName(obj)
	if err != nil {
		return nil, err
	}

	start := time.Now()

	created, createErr := r.createObject(ctx, kind, namespace, obj, opts)

	duration := time.Since(start)

	recordErr := r.recordOperation(
		storage.OperationCreate,
		kind,
		namespace,
		name,
		created,
		createErr,
		duration,
	)
	if recordErr != nil {
		return created, fmt.Errorf("record failed: %w (original error: %v)",
			recordErr, createErr)
	}

	return created, createErr
}

// RecordUpdate records an UPDATE operation with timing.
// Rule 7: All return values checked.
func (r *RecordingClient) RecordUpdate(
	ctx context.Context,
	kind string,
	namespace string,
	obj runtime.Object,
	opts metav1.UpdateOptions,
) (runtime.Object, error) {
	err := assert.AssertNotNil(r, "recorder")
	if err != nil {
		return nil, err
	}

	err = assert.AssertStringNotEmpty(kind, "resource kind")
	if err != nil {
		return nil, err
	}

	err = assert.AssertStringNotEmpty(namespace, "namespace")
	if err != nil {
		return nil, err
	}

	err = assert.AssertNotNil(obj, "object")
	if err != nil {
		return nil, err
	}

	if kind != "ConfigMap" && kind != "Secret" {
		return nil, fmt.Errorf("unsupported resource kind: %s", kind)
	}

	name, err := extractObjectName(obj)
	if err != nil {
		return nil, err
	}

	start := time.Now()

	updated, updateErr := r.updateObject(ctx, kind, namespace, obj, opts)

	duration := time.Since(start)

	recordErr := r.recordOperation(
		storage.OperationUpdate,
		kind,
		namespace,
		name,
		updated,
		updateErr,
		duration,
	)
	if recordErr != nil {
		return updated, fmt.Errorf("record failed: %w (original error: %v)",
			recordErr, updateErr)
	}

	return updated, updateErr
}

// RecordDelete records a DELETE operation with timing.
// Rule 7: All return values checked.
func (r *RecordingClient) RecordDelete(
	ctx context.Context,
	kind string,
	namespace string,
	name string,
	opts metav1.DeleteOptions,
) error {
	err := assert.AssertNotNil(r, "recorder")
	if err != nil {
		return err
	}

	err = assert.AssertStringNotEmpty(kind, "resource kind")
	if err != nil {
		return err
	}

	err = assert.AssertStringNotEmpty(namespace, "namespace")
	if err != nil {
		return err
	}

	err = assert.AssertStringNotEmpty(name, "resource name")
	if err != nil {
		return err
	}

	start := time.Now()
	var deleteErr error

	switch kind {
	case "ConfigMap":
		deleteErr = r.client.CoreV1().ConfigMaps(namespace).Delete(ctx, name, opts)
	case "Secret":
		deleteErr = r.client.CoreV1().Secrets(namespace).Delete(ctx, name, opts)
	default:
		return fmt.Errorf("unsupported resource kind: %s", kind)
	}

	duration := time.Since(start)

	recordErr := r.recordOperation(
		storage.OperationDelete,
		kind,
		namespace,
		name,
		nil,
		deleteErr,
		duration,
	)
	if recordErr != nil {
		return fmt.Errorf("record failed: %w (original error: %v)",
			recordErr, deleteErr)
	}

	return deleteErr
}

func (r *RecordingClient) createObject(
	ctx context.Context,
	kind string,
	namespace string,
	obj runtime.Object,
	opts metav1.CreateOptions,
) (runtime.Object, error) {
	err := assert.AssertNotNil(r, "recorder")
	if err != nil {
		return nil, err
	}

	err = assert.AssertNotNil(obj, "object")
	if err != nil {
		return nil, err
	}

	switch kind {
	case "ConfigMap":
		cm, ok := obj.(*corev1.ConfigMap)
		if !ok {
			return nil, fmt.Errorf("invalid object type for ConfigMap")
		}
		return r.client.CoreV1().ConfigMaps(namespace).Create(ctx, cm, opts)
	case "Secret":
		secret, ok := obj.(*corev1.Secret)
		if !ok {
			return nil, fmt.Errorf("invalid object type for Secret")
		}
		return r.client.CoreV1().Secrets(namespace).Create(ctx, secret, opts)
	default:
		return nil, fmt.Errorf("unsupported resource kind: %s", kind)
	}
}

func (r *RecordingClient) updateObject(
	ctx context.Context,
	kind string,
	namespace string,
	obj runtime.Object,
	opts metav1.UpdateOptions,
) (runtime.Object, error) {
	err := assert.AssertNotNil(r, "recorder")
	if err != nil {
		return nil, err
	}

	err = assert.AssertNotNil(obj, "object")
	if err != nil {
		return nil, err
	}

	switch kind {
	case "ConfigMap":
		cm, ok := obj.(*corev1.ConfigMap)
		if !ok {
			return nil, fmt.Errorf("invalid object type for ConfigMap")
		}
		return r.client.CoreV1().ConfigMaps(namespace).Update(ctx, cm, opts)
	case "Secret":
		secret, ok := obj.(*corev1.Secret)
		if !ok {
			return nil, fmt.Errorf("invalid object type for Secret")
		}
		return r.client.CoreV1().Secrets(namespace).Update(ctx, secret, opts)
	default:
		return nil, fmt.Errorf("unsupported resource kind: %s", kind)
	}
}

func extractObjectName(obj runtime.Object) (string, error) {
	err := assert.AssertNotNil(obj, "object")
	if err != nil {
		return "", err
	}

	accessor, metaErr := meta.Accessor(obj)
	if metaErr != nil {
		return "", fmt.Errorf("object metadata error: %w", metaErr)
	}

	name := accessor.GetName()
	err = assert.AssertStringNotEmpty(name, "object name")
	if err != nil {
		return "", err
	}

	return name, nil
}

func extractObjectMetadata(obj runtime.Object) (string, string, int64) {
	if obj == nil {
		return "", "", 0
	}

	accessor, err := meta.Accessor(obj)
	if err != nil {
		return "", "", 0
	}

	return string(accessor.GetUID()), accessor.GetResourceVersion(), accessor.GetGeneration()
}

// GetSequenceNumber returns current sequence number.
func (r *RecordingClient) GetSequenceNumber() int64 {
	return r.sequenceNum
}
