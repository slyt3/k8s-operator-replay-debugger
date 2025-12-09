package storage

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/operator-replay-debugger/internal/assert"
)

const (
	maxOperationTypeLength = 20
	maxResourceKindLength  = 100
	maxNamespaceLength     = 253
	maxNameLength          = 253
	maxDataLength          = 1048576 // 1MB max per operation
	maxErrorLength         = 10000
)

// OperationType defines the type of Kubernetes operation.
type OperationType string

const (
	OperationGet    OperationType = "GET"
	OperationList   OperationType = "LIST"
	OperationCreate OperationType = "CREATE"
	OperationUpdate OperationType = "UPDATE"
	OperationPatch  OperationType = "PATCH"
	OperationDelete OperationType = "DELETE"
	OperationWatch  OperationType = "WATCH"
)

// Operation represents a recorded Kubernetes API operation.
// Rule 6: Data declared at smallest scope, no global state.
type Operation struct {
	ID             int64
	SessionID      string
	SequenceNumber int64
	Timestamp      time.Time
	OperationType  OperationType
	ResourceKind   string
	Namespace      string
	Name           string
	ResourceData   string
	Error          string
	DurationMs     int64
}

// Database handles SQLite storage for recorded operations.
// Rule 3: No dynamic allocation after initialization.
type Database struct {
	db            *sql.DB
	insertStmt    *sql.Stmt
	queryStmt     *sql.Stmt
	sessionStmt   *sql.Stmt
	maxOperations int
}

// Schema defines the SQLite database structure.
const Schema = `
CREATE TABLE IF NOT EXISTS operations (
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
    duration_ms INTEGER NOT NULL,
    CHECK(length(operation_type) <= 20),
    CHECK(length(resource_kind) <= 100),
    CHECK(length(namespace) <= 253),
    CHECK(length(name) <= 253),
    CHECK(length(resource_data) <= 1048576),
    CHECK(length(error) <= 10000)
);

CREATE INDEX IF NOT EXISTS idx_session_sequence 
ON operations(session_id, sequence_number);

CREATE INDEX IF NOT EXISTS idx_timestamp 
ON operations(timestamp);

CREATE INDEX IF NOT EXISTS idx_resource 
ON operations(resource_kind, namespace, name);
`

// ValidateOperation checks operation data meets constraints.
// Rule 5: Minimum 2 assertions per function.
func ValidateOperation(op *Operation) error {
	if op == nil {
		return fmt.Errorf("operation is nil")
	}

	err := assert.AssertNotNil(op, "operation")
	if err != nil {
		return err
	}

	if len(op.SessionID) == 0 {
		return fmt.Errorf("session_id is empty")
	}

	err = assert.AssertStringNotEmpty(op.SessionID, "session_id")
	if err != nil {
		return err
	}
	err = assert.AssertInRange(
		len(op.ResourceKind),
		1,
		maxResourceKindLength,
		"resource_kind length",
	)
	if err != nil {
		return err
	}

	if len(op.Namespace) > maxNamespaceLength {
		err = assert.Assert(false, "namespace exceeds max length")
		if err != nil {
			return err
		}
	}

	if len(op.Name) > maxNameLength {
		err = assert.Assert(false, "name exceeds max length")
		if err != nil {
			return err
		}
	}

	if len(op.ResourceData) > maxDataLength {
		err = assert.Assert(false, "resource_data exceeds max length")
		if err != nil {
			return err
		}
	}

	if len(op.Error) > maxErrorLength {
		err = assert.Assert(false, "error exceeds max length")
		if err != nil {
			return err
		}
	}

	return nil
}
