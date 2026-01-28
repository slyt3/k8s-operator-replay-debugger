package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/operator-replay-debugger/internal/assert"
)

// SQLiteStore implements OperationStore using SQLite.
type SQLiteStore struct {
	db            *sql.DB
	insertStmt    *sql.Stmt
	queryStmt     *sql.Stmt
	sessionStmt   *sql.Stmt
	maxOperations int
}

// NewSQLiteStore creates a new SQLite-based operation store.
func NewSQLiteStore(cfg StorageConfig) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", cfg.ConnectionURI)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	err = initializeSQLiteSchema(db)
	if err != nil {
		closeErr := db.Close()
		if closeErr != nil {
			return nil, fmt.Errorf("schema init failed: %w, close failed: %v",
				err, closeErr)
		}
		return nil, err
	}

	store := &SQLiteStore{
		db:            db,
		maxOperations: cfg.MaxOperations,
	}

	err = store.prepareStatements()
	if err != nil {
		closeErr := db.Close()
		if closeErr != nil {
			return nil, fmt.Errorf("statement prep failed: %w, close failed: %v",
				err, closeErr)
		}
		return nil, err
	}

	return store, nil
}

// InsertOperation inserts a single operation record.
func (s *SQLiteStore) InsertOperation(op *Operation) error {
	err := assert.AssertNotNil(op, "operation")
	if err != nil {
		return err
	}

	err = ValidateOperation(op)
	if err != nil {
		return fmt.Errorf("invalid operation: %w", err)
	}

	_, err = s.insertStmt.Exec(
		op.SessionID,
		op.SequenceNumber,
		op.Timestamp.Unix(),
		string(op.OperationType),
		op.ResourceKind,
		op.Namespace,
		op.Name,
		op.ResourceData,
		op.Error,
		op.DurationMs,
	)
	if err != nil {
		return fmt.Errorf("failed to insert operation: %w", err)
	}

	return nil
}

// QueryOperations retrieves all operations for a session.
func (s *SQLiteStore) QueryOperations(sessionID string) ([]Operation, error) {
	err := assert.AssertStringNotEmpty(sessionID, "session ID")
	if err != nil {
		return nil, err
	}

	rows, err := s.queryStmt.Query(sessionID, maxQueryResults)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			fmt.Printf("Warning: failed to close rows: %v\n", closeErr)
		}
	}()

	return s.scanOperations(rows)
}

// QueryOperationsByRange retrieves operations within sequence range.
func (s *SQLiteStore) QueryOperationsByRange(
	sessionID string, 
	start, end int64,
) ([]Operation, error) {
	err := assert.AssertStringNotEmpty(sessionID, "session ID")
	if err != nil {
		return nil, err
	}

	err = assert.AssertInRange(int(start), 0, int(end), "start sequence")
	if err != nil {
		return nil, err
	}

	query := `SELECT id, session_id, sequence_number, timestamp, 
	         operation_type, resource_kind, namespace, name, 
	         resource_data, error, duration_ms 
	         FROM operations 
	         WHERE session_id = ? 
	         AND sequence_number BETWEEN ? AND ?
	         ORDER BY sequence_number LIMIT ?`

	rows, err := s.db.Query(query, sessionID, start, end, maxQueryResults)
	if err != nil {
		return nil, fmt.Errorf("range query failed: %w", err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			fmt.Printf("Warning: failed to close rows: %v\n", closeErr)
		}
	}()

	return s.scanOperations(rows)
}

// ListSessions returns all available sessions.
func (s *SQLiteStore) ListSessions() ([]SessionInfo, error) {
	query := `SELECT session_id, 
	         MIN(timestamp) as start_time,
	         MAX(timestamp) as end_time,
	         COUNT(*) as op_count
	         FROM operations 
	         GROUP BY session_id 
	         ORDER BY start_time DESC
	         LIMIT ?`

	rows, err := s.db.Query(query, maxQueryResults)
	if err != nil {
		return nil, fmt.Errorf("session query failed: %w", err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			fmt.Printf("Warning: failed to close rows: %v\n", closeErr)
		}
	}()

	sessions := make([]SessionInfo, 0, 100)
	for rows.Next() {
		var session SessionInfo
		err = rows.Scan(
			&session.SessionID,
			&session.StartTime,
			&session.EndTime,
			&session.OpCount,
		)
		if err != nil {
			return nil, fmt.Errorf("session scan failed: %w", err)
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// Close closes the database connection and prepared statements.
func (s *SQLiteStore) Close() error {
	if s.insertStmt != nil {
		err := s.insertStmt.Close()
		if err != nil {
			return fmt.Errorf("failed to close insert statement: %w", err)
		}
	}

	if s.queryStmt != nil {
		err := s.queryStmt.Close()
		if err != nil {
			return fmt.Errorf("failed to close query statement: %w", err)
		}
	}

	if s.sessionStmt != nil {
		err := s.sessionStmt.Close()
		if err != nil {
			return fmt.Errorf("failed to close session statement: %w", err)
		}
	}

	if s.db != nil {
		return s.db.Close()
	}

	return nil
}

// prepareStatements creates prepared statements for SQLite operations.
func (s *SQLiteStore) prepareStatements() error {
	var err error

	insertSQL := `INSERT INTO operations (
		session_id, sequence_number, timestamp, operation_type,
		resource_kind, namespace, name, resource_data, error, duration_ms
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	s.insertStmt, err = s.db.Prepare(insertSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %w", err)
	}

	querySQL := `SELECT id, session_id, sequence_number, timestamp,
	            operation_type, resource_kind, namespace, name,
	            resource_data, error, duration_ms
	            FROM operations WHERE session_id = ?
	            ORDER BY sequence_number LIMIT ?`

	s.queryStmt, err = s.db.Prepare(querySQL)
	if err != nil {
		return fmt.Errorf("failed to prepare query statement: %w", err)
	}

	return nil
}

// scanOperations scans database rows into Operation structs.
func (s *SQLiteStore) scanOperations(rows *sql.Rows) ([]Operation, error) {
	operations := make([]Operation, 0, 1000)
	count := 0
	maxResults := 10000

	for rows.Next() && count < maxResults {
		var op Operation
		var timestamp int64

		err := rows.Scan(
			&op.ID,
			&op.SessionID,
			&op.SequenceNumber,
			&timestamp,
			&op.OperationType,
			&op.ResourceKind,
			&op.Namespace,
			&op.Name,
			&op.ResourceData,
			&op.Error,
			&op.DurationMs,
		)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}

		op.Timestamp = time.Unix(timestamp, 0)
		operations = append(operations, op)
		count = count + 1
	}

	return operations, nil
}

// initializeSQLiteSchema creates the operations table.
func initializeSQLiteSchema(db *sql.DB) error {
	_, err := db.Exec(Schema)
	if err != nil {
		return fmt.Errorf("schema creation failed: %w", err)
	}
	return nil
}