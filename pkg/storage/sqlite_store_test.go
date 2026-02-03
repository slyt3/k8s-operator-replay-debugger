package storage

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSQLiteStoreOperationsAndSessions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store.db")

	store, err := NewSQLiteStore(StorageConfig{
		Type:          "sqlite",
		ConnectionURI: path,
		MaxOperations: 1000,
	})
	require.NoError(t, err)
	defer func() {
		_ = store.Close()
	}()

	sessionID := "session-1"
	for i := 1; i <= 3; i++ {
		op := &Operation{
			SessionID:      sessionID,
			SequenceNumber: int64(i),
			Timestamp:      time.Now(),
			OperationType:  OperationGet,
			ResourceKind:   "Pod",
			Namespace:      "default",
			Name:           "demo",
			ResourceData:   `{}`,
			DurationMs:     10,
		}
		require.NoError(t, store.InsertOperation(op))
	}

	ops, err := store.QueryOperations(sessionID)
	require.NoError(t, err)
	require.Len(t, ops, 3)

	ops, err = store.QueryOperationsByRange(sessionID, 2, 3)
	require.NoError(t, err)
	require.Len(t, ops, 2)

	sessions, err := store.ListSessions()
	require.NoError(t, err)
	require.NotEmpty(t, sessions)
	require.Equal(t, sessionID, sessions[0].SessionID)
}

func TestSQLiteStoreSpans(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spans.db")

	store, err := NewSQLiteStore(StorageConfig{
		Type:          "sqlite",
		ConnectionURI: path,
		MaxOperations: 1000,
	})
	require.NoError(t, err)
	defer func() {
		_ = store.Close()
	}()

	span := &ReconcileSpan{
		ID:        "span-1",
		SessionID: "session-1",
		ActorID:   "actor",
		StartTime: time.Now(),
		Kind:      "Pod",
		Namespace: "default",
		Name:      "demo",
	}

	require.NoError(t, store.InsertReconcileSpan(span))
	require.NoError(t, store.EndReconcileSpan(span.ID, time.Now(), 5, ""))

	spans, err := store.QueryReconcileSpans(span.SessionID)
	require.NoError(t, err)
	require.Len(t, spans, 1)
	require.Equal(t, span.ID, spans[0].ID)
}
