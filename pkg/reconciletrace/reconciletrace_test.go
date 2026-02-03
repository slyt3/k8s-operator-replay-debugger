package reconciletrace

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/slyt3/kubestep/pkg/storage"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type fakeSpanStore struct {
	inserted *storage.ReconcileSpan
	ended    bool
	endID    string
	endErr   string
}

func (f *fakeSpanStore) InsertReconcileSpan(span *storage.ReconcileSpan) error {
	f.inserted = span
	return nil
}

func (f *fakeSpanStore) EndReconcileSpan(
	spanID string,
	endTime time.Time,
	durationMs int64,
	errMsg string,
) error {
	f.ended = true
	f.endID = spanID
	f.endErr = errMsg
	return nil
}

func (f *fakeSpanStore) QueryReconcileSpans(sessionID string) ([]storage.ReconcileSpan, error) {
	return nil, nil
}

func TestStartAndEndSpan(t *testing.T) {
	store := &fakeSpanStore{}
	gvk := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}

	spanID, ctx := Start(
		context.Background(),
		store,
		"session-1",
		"",
		gvk,
		"default",
		"demo",
		"uid-1",
		"rv-1",
		"update",
	)

	require.NotEmpty(t, spanID)
	require.NotNil(t, ctx)
	require.NotNil(t, store.inserted)
	require.Equal(t, "unknown", store.inserted.ActorID)
	require.Equal(t, "Deployment", store.inserted.Kind)

	End(ctx, store, spanID, errors.New("boom"))
	require.True(t, store.ended)
	require.Equal(t, spanID, store.endID)
	require.Equal(t, "boom", store.endErr)
}

func TestStartValidationFailures(t *testing.T) {
	spanID, ctx := Start(nil, nil, "session-1", "actor", schema.GroupVersionKind{}, "", "", "", "", "")
	require.Empty(t, spanID)
	require.NotNil(t, ctx)

	store := &fakeSpanStore{}
	spanID, _ = Start(context.Background(), store, "", "actor", schema.GroupVersionKind{}, "", "", "", "", "")
	require.Empty(t, spanID)
}

func TestEndValidationFailures(t *testing.T) {
	End(context.Background(), nil, "span-1", nil)
	End(context.Background(), &fakeSpanStore{}, "", nil)
}
