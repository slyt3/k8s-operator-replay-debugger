package storage

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
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

func TestVerifySQLiteMissingFile(t *testing.T) {
	_, err := VerifySQLite("does-not-exist.db", false)
	require.Error(t, err)
}

func TestVerifySQLiteMissingOptionalColumnsStrict(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "minimal.db")

	db, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE operations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		sequence_number INTEGER NOT NULL,
		timestamp INTEGER NOT NULL,
		operation_type TEXT NOT NULL,
		resource_kind TEXT NOT NULL,
		namespace TEXT,
		name TEXT,
		resource_data TEXT,
		error TEXT,
		duration_ms INTEGER NOT NULL
	)`)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	result, err := VerifySQLite(path, true)
	require.NoError(t, err)
	require.NotEmpty(t, result.Errors)
}

func TestVerifySQLiteMissingOperationsTable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.db")

	db, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE dummy (id INTEGER)`)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	result, err := VerifySQLite(path, false)
	require.NoError(t, err)
	require.NotEmpty(t, result.Errors)
}
