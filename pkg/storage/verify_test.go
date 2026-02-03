package storage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestVerifySQLiteBasic(t *testing.T) {
	cleanupTestDB(t)
	defer cleanupTestDB(t)

	db, err := NewDatabase(testDBPath, testMaxOps)
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	op := &Operation{
		SessionID:      "verify-session",
		SequenceNumber: 1,
		Timestamp:      time.Now(),
		OperationType:  OperationGet,
		ResourceKind:   "Pod",
		Namespace:      "default",
		Name:           "demo",
		ResourceData:   `{}`,
		DurationMs:     10,
	}
	err = db.InsertOperation(op)
	require.NoError(t, err)

	result, err := VerifySQLite(testDBPath, false)
	require.NoError(t, err)
	require.Len(t, result.Errors, 0)
}

func TestVerifySQLiteDetectsDuplicateSequence(t *testing.T) {
	cleanupTestDB(t)
	defer cleanupTestDB(t)

	db, err := NewDatabase(testDBPath, testMaxOps)
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	op1 := &Operation{
		SessionID:      "dup-session",
		SequenceNumber: 1,
		Timestamp:      time.Now(),
		OperationType:  OperationGet,
		ResourceKind:   "Pod",
		Namespace:      "default",
		Name:           "demo",
		ResourceData:   `{}`,
		DurationMs:     10,
	}

	op2 := &Operation{
		SessionID:      "dup-session",
		SequenceNumber: 1,
		Timestamp:      time.Now(),
		OperationType:  OperationGet,
		ResourceKind:   "Pod",
		Namespace:      "default",
		Name:           "demo-2",
		ResourceData:   `{}`,
		DurationMs:     12,
	}

	require.NoError(t, db.InsertOperation(op1))
	require.NoError(t, db.InsertOperation(op2))

	result, err := VerifySQLite(testDBPath, false)
	require.NoError(t, err)
	require.NotEmpty(t, result.Errors)
}
