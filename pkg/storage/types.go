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
	maxActorIDLength       = 256
	maxUIDLength           = 128
	maxResourceVersionLen  = 128
	maxVerbLength          = 20
	maxSpanIDLength        = 128
	maxDataLength          = 1048576 // 1MB max per operation
	maxErrorLength         = 10000
	maxTriggerReasonLength = 512
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
	ID              int64
	SessionID       string
	SequenceNumber  int64
	Timestamp       time.Time
	OperationType   OperationType
	ResourceKind    string
	Namespace       string
	Name            string
	ResourceData    string
	Error           string
	DurationMs      int64
	ActorID         string
	UID             string
	ResourceVersion string
	Generation      int64
	Verb            string
}

// Database handles SQLite storage for recorded operations.
// Rule 3: No dynamic allocation after initialization.
type Database struct {
	db             *sql.DB
	insertStmt     *sql.Stmt
	queryStmt      *sql.Stmt
	sessionStmt    *sql.Stmt
	insertSpanStmt *sql.Stmt
	endSpanStmt    *sql.Stmt
	querySpanStmt  *sql.Stmt
	maxOperations  int
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
    actor_id TEXT,
    uid TEXT,
    resource_version TEXT,
    generation INTEGER,
    verb TEXT,
    CHECK(length(operation_type) <= 20),
    CHECK(length(resource_kind) <= 100),
    CHECK(length(namespace) <= 253),
    CHECK(length(name) <= 253),
    CHECK(length(actor_id) <= 256),
    CHECK(length(uid) <= 128),
    CHECK(length(resource_version) <= 128),
    CHECK(length(verb) <= 20),
    CHECK(length(resource_data) <= 1048576),
    CHECK(length(error) <= 10000)
);

CREATE INDEX IF NOT EXISTS idx_session_sequence 
ON operations(session_id, sequence_number);

CREATE INDEX IF NOT EXISTS idx_timestamp 
ON operations(timestamp);

CREATE INDEX IF NOT EXISTS idx_resource 
ON operations(resource_kind, namespace, name);

CREATE TABLE IF NOT EXISTS reconcile_spans (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    start_ts INTEGER NOT NULL,
    end_ts INTEGER,
    duration_ms INTEGER,
    kind TEXT NOT NULL,
    namespace TEXT,
    name TEXT,
    trigger_uid TEXT,
    trigger_resource_version TEXT,
    trigger_reason TEXT,
    error TEXT,
    CHECK(length(actor_id) <= 256),
    CHECK(length(kind) <= 100),
    CHECK(length(namespace) <= 253),
    CHECK(length(name) <= 253),
    CHECK(length(trigger_uid) <= 128),
    CHECK(length(trigger_resource_version) <= 128),
    CHECK(length(trigger_reason) <= 512),
    CHECK(length(error) <= 10000)
);

CREATE INDEX IF NOT EXISTS idx_reconcile_session
ON reconcile_spans(session_id, start_ts);

CREATE INDEX IF NOT EXISTS idx_reconcile_trigger
ON reconcile_spans(trigger_uid, trigger_resource_version);
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

	if len(op.ActorID) > maxActorIDLength {
		err = assert.Assert(false, "actor_id exceeds max length")
		if err != nil {
			return err
		}
	}

	if len(op.UID) > maxUIDLength {
		err = assert.Assert(false, "uid exceeds max length")
		if err != nil {
			return err
		}
	}

	if len(op.ResourceVersion) > maxResourceVersionLen {
		err = assert.Assert(false, "resource_version exceeds max length")
		if err != nil {
			return err
		}
	}

	if len(op.Verb) > maxVerbLength {
		err = assert.Assert(false, "verb exceeds max length")
		if err != nil {
			return err
		}
	}

	if op.Generation < 0 {
		err = assert.Assert(false, "generation must be non-negative")
		if err != nil {
			return err
		}
	}

	return nil
}
