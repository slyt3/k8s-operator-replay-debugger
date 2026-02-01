package reconciletrace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/slyt3/kubestep/internal/assert"
	"github.com/slyt3/kubestep/pkg/storage"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type spanContextKey string

const (
	defaultActorID = "unknown"
)

// Start begins a reconcile span and returns the span ID plus a context carrying span timing.
func Start(
	ctx context.Context,
	store storage.ReconcileSpanStore,
	sessionID string,
	actorID string,
	gvk schema.GroupVersionKind,
	namespace string,
	name string,
	triggerUID string,
	triggerRV string,
	triggerReason string,
) (string, context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	err := assert.AssertNotNil(store, "span store")
	if err != nil {
		return "", ctx
	}
	err = assert.AssertStringNotEmpty(sessionID, "session_id")
	if err != nil {
		return "", ctx
	}

	if len(actorID) == 0 {
		actorID = defaultActorID
	}
	err = assert.AssertStringNotEmpty(actorID, "actor_id")
	if err != nil {
		return "", ctx
	}

	kind := gvk.Kind
	if len(kind) == 0 {
		kind = "unknown"
	}

	spanID := newSpanID()
	startTime := time.Now()

	span := &storage.ReconcileSpan{
		ID:                     spanID,
		SessionID:              sessionID,
		ActorID:                actorID,
		StartTime:              startTime,
		Kind:                   kind,
		Namespace:              namespace,
		Name:                   name,
		TriggerUID:             triggerUID,
		TriggerResourceVersion: triggerRV,
		TriggerReason:          triggerReason,
	}

	err = store.InsertReconcileSpan(span)
	if err != nil {
		return "", ctx
	}

	ctx = context.WithValue(ctx, spanContextKey(spanID), startTime)
	return spanID, ctx
}

// End ends a reconcile span and records duration and error.
func End(
	ctx context.Context,
	store storage.ReconcileSpanStore,
	spanID string,
	err error,
) {
	assertErr := assert.AssertNotNil(store, "span store")
	if assertErr != nil {
		return
	}
	assertErr = assert.AssertStringNotEmpty(spanID, "span ID")
	if assertErr != nil {
		return
	}

	endTime := time.Now()
	durationMs := int64(0)

	if ctx != nil {
		if value := ctx.Value(spanContextKey(spanID)); value != nil {
			if startTime, ok := value.(time.Time); ok {
				durationMs = endTime.Sub(startTime).Milliseconds()
			}
		}
	}

	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	updateErr := store.EndReconcileSpan(spanID, endTime, durationMs, errMsg)
	if updateErr != nil {
		return
	}
}

func newSpanID() string {
	var buf [16]byte
	_, err := rand.Read(buf[:])
	if err != nil {
		return hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(buf[:])
}
