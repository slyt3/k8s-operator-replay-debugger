package recorder

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/operator-replay-debugger/internal/assert"
	"github.com/operator-replay-debugger/pkg/storage"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

const (
	maxSessionIDLength = 100
	maxRetries         = 3
)

// RecordingClient wraps a Kubernetes client to record all operations.
// Rule 6: Minimal scope, all fields private.
type RecordingClient struct {
	client        kubernetes.Interface
	db            *storage.Database
	sessionID     string
	sequenceNum   int64
	enabled       bool
	maxSequence   int64
}

// Config holds recorder configuration.
// Rule 3: Pre-allocated configuration, no dynamic allocation.
type Config struct {
	Client      kubernetes.Interface
	Database    *storage.Database
	SessionID   string
	MaxSequence int64
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

	return &RecordingClient{
		client:      cfg.Client,
		db:          cfg.Database,
		sessionID:   cfg.SessionID,
		sequenceNum: 0,
		enabled:     true,
		maxSequence: cfg.MaxSequence,
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

	op := &storage.Operation{
		SessionID:      r.sessionID,
		SequenceNumber: r.sequenceNum,
		Timestamp:      time.Now(),
		OperationType:  opType,
		ResourceKind:   kind,
		Namespace:      namespace,
		Name:           name,
		ResourceData:   resourceData,
		Error:          errorMsg,
		DurationMs:     duration.Milliseconds(),
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

// GetSequenceNumber returns current sequence number.
func (r *RecordingClient) GetSequenceNumber() int64 {
	return r.sequenceNum
}
