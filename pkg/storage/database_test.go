package storage

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testDBPath = "test_recordings.db"
	testMaxOps = 1000
)

// TestDatabaseCreation tests database initialization.
func TestDatabaseCreation(t *testing.T) {
	cleanupTestDB(t)
	defer cleanupTestDB(t)

	db, err := NewDatabase(testDBPath, testMaxOps)
	require.NoError(t, err, "database creation should succeed")
	require.NotNil(t, db, "database should not be nil")

	err = db.Close()
	assert.NoError(t, err, "database close should succeed")
}

// TestInsertOperation tests operation insertion.
func TestInsertOperation(t *testing.T) {
	cleanupTestDB(t)
	defer cleanupTestDB(t)

	db, err := NewDatabase(testDBPath, testMaxOps)
	require.NoError(t, err)
	defer func() {
		closeErr := db.Close()
		assert.NoError(t, closeErr)
	}()

	op := &Operation{
		SessionID:      "test-session-001",
		SequenceNumber: 1,
		Timestamp:      time.Now(),
		OperationType:  OperationGet,
		ResourceKind:   "Pod",
		Namespace:      "default",
		Name:           "test-pod",
		ResourceData:   `{"kind":"Pod","metadata":{"name":"test-pod"}}`,
		Error:          "",
		DurationMs:     150,
	}

	err = db.InsertOperation(op)
	assert.NoError(t, err, "insert should succeed")
}

// TestQueryOperations tests operation retrieval.
func TestQueryOperations(t *testing.T) {
	cleanupTestDB(t)
	defer cleanupTestDB(t)

	db, err := NewDatabase(testDBPath, testMaxOps)
	require.NoError(t, err)
	defer func() {
		closeErr := db.Close()
		assert.NoError(t, closeErr)
	}()

	sessionID := "test-session-002"

	maxInsert := 5
	for i := 0; i < maxInsert; i = i + 1 {
		op := &Operation{
			SessionID:      sessionID,
			SequenceNumber: int64(i + 1),
			Timestamp:      time.Now(),
			OperationType:  OperationGet,
			ResourceKind:   "Pod",
			Namespace:      "default",
			Name:           "test-pod",
			ResourceData:   `{}`,
			DurationMs:     100,
		}

		err = db.InsertOperation(op)
		require.NoError(t, err)
	}

	ops, err := db.QueryOperations(sessionID)
	require.NoError(t, err, "query should succeed")
	assert.Equal(t, maxInsert, len(ops), "should retrieve all operations")

	for i := 0; i < len(ops) && i < maxInsert; i = i + 1 {
		assert.Equal(t, sessionID, ops[i].SessionID)
		assert.Equal(t, int64(i+1), ops[i].SequenceNumber)
	}
}

// TestValidateOperation tests operation validation.
func TestValidateOperation(t *testing.T) {
	tests := []struct {
		name    string
		op      *Operation
		wantErr bool
	}{
		{
			name: "valid operation",
			op: &Operation{
				SessionID:     "session-1",
				ResourceKind:  "Pod",
				Namespace:     "default",
				Name:          "pod-1",
				ResourceData:  "{}",
			},
			wantErr: false,
		},
		{
			name:    "nil operation",
			op:      nil,
			wantErr: true,
		},
		{
			name: "empty session ID",
			op: &Operation{
				SessionID:    "",
				ResourceKind: "Pod",
			},
			wantErr: true,
		},
		{
			name: "empty resource kind",
			op: &Operation{
				SessionID:    "session-1",
				ResourceKind: "",
			},
			wantErr: true,
		},
	}

	maxTests := len(tests)
	for i := 0; i < maxTests; i = i + 1 {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOperation(tt.op)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestOperationTypes tests operation type constants.
func TestOperationTypes(t *testing.T) {
	types := []OperationType{
		OperationGet,
		OperationList,
		OperationCreate,
		OperationUpdate,
		OperationPatch,
		OperationDelete,
		OperationWatch,
	}

	maxTypes := len(types)
	for i := 0; i < maxTypes; i = i + 1 {
		opType := types[i]
		assert.NotEmpty(t, string(opType), "operation type should not be empty")
	}
}

// cleanupTestDB removes test database file.
func cleanupTestDB(t *testing.T) {
	err := os.Remove(testDBPath)
	if err != nil && !os.IsNotExist(err) {
		t.Logf("Warning: failed to cleanup test db: %v", err)
	}
}
