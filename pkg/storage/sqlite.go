package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/slyt3/kubestep/internal/assert"
)

// SQLiteStore implements OperationStore using SQLite.
type SQLiteStore struct {
	db             *sql.DB
	insertStmt     *sql.Stmt
	queryStmt      *sql.Stmt
	sessionStmt    *sql.Stmt
	insertSpanStmt *sql.Stmt
	endSpanStmt    *sql.Stmt
	querySpanStmt  *sql.Stmt
	maxOperations  int
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
		op.ActorID,
		op.UID,
		op.ResourceVersion,
		op.Generation,
		op.Verb,
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
	         resource_data, error, duration_ms, actor_id, uid,
	         resource_version, generation, verb
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

// InsertReconcileSpan inserts a reconcile span record.
func (s *SQLiteStore) InsertReconcileSpan(span *ReconcileSpan) error {
	err := assert.AssertNotNil(s, "store")
	if err != nil {
		return err
	}

	err = assert.AssertNotNil(s.insertSpanStmt, "span insert statement")
	if err != nil {
		return err
	}

	err = ValidateReconcileSpan(span)
	if err != nil {
		return fmt.Errorf("span validation failed: %w", err)
	}

	startTs := span.StartTime.Unix()
	var endTs interface{}
	if !span.EndTime.IsZero() {
		endTs = span.EndTime.Unix()
	}

	var duration interface{}
	if span.DurationMs > 0 {
		duration = span.DurationMs
	}

	_, err = s.insertSpanStmt.Exec(
		span.ID,
		span.SessionID,
		span.ActorID,
		startTs,
		endTs,
		duration,
		span.Kind,
		span.Namespace,
		span.Name,
		span.TriggerUID,
		span.TriggerResourceVersion,
		span.TriggerReason,
		span.Error,
	)
	if err != nil {
		return fmt.Errorf("failed to insert reconcile span: %w", err)
	}

	return nil
}

// EndReconcileSpan updates end time and error for a span.
func (s *SQLiteStore) EndReconcileSpan(
	spanID string,
	endTime time.Time,
	durationMs int64,
	errMsg string,
) error {
	err := assert.AssertNotNil(s, "store")
	if err != nil {
		return err
	}

	err = assert.AssertNotNil(s.endSpanStmt, "span end statement")
	if err != nil {
		return err
	}

	err = assert.AssertStringNotEmpty(spanID, "span id")
	if err != nil {
		return err
	}

	_, err = s.endSpanStmt.Exec(
		endTime.Unix(),
		durationMs,
		errMsg,
		spanID,
	)
	if err != nil {
		return fmt.Errorf("failed to update reconcile span: %w", err)
	}

	return nil
}

// QueryReconcileSpans retrieves spans for a session.
func (s *SQLiteStore) QueryReconcileSpans(sessionID string) ([]ReconcileSpan, error) {
	err := assert.AssertNotNil(s, "store")
	if err != nil {
		return nil, err
	}

	err = assert.AssertStringNotEmpty(sessionID, "session ID")
	if err != nil {
		return nil, err
	}

	rows, err := s.querySpanStmt.Query(sessionID, maxQueryResults)
	if err != nil {
		return nil, fmt.Errorf("span query failed: %w", err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			fmt.Printf("Warning: failed to close rows: %v\n", closeErr)
		}
	}()

	spans := make([]ReconcileSpan, 0, 1000)
	count := 0
	maxResults := 10000

	for rows.Next() && count < maxResults {
		var span ReconcileSpan
		var startTs int64
		var endTs sql.NullInt64
		var duration sql.NullInt64
		var namespace sql.NullString
		var name sql.NullString
		var triggerUID sql.NullString
		var triggerRV sql.NullString
		var triggerReason sql.NullString
		var errMsg sql.NullString

		err = rows.Scan(
			&span.ID,
			&span.SessionID,
			&span.ActorID,
			&startTs,
			&endTs,
			&duration,
			&span.Kind,
			&namespace,
			&name,
			&triggerUID,
			&triggerRV,
			&triggerReason,
			&errMsg,
		)
		if err != nil {
			return nil, fmt.Errorf("span scan failed: %w", err)
		}

		span.StartTime = time.Unix(startTs, 0)
		if endTs.Valid {
			span.EndTime = time.Unix(endTs.Int64, 0)
		}
		if duration.Valid {
			span.DurationMs = duration.Int64
		}
		if namespace.Valid {
			span.Namespace = namespace.String
		}
		if name.Valid {
			span.Name = name.String
		}
		if triggerUID.Valid {
			span.TriggerUID = triggerUID.String
		}
		if triggerRV.Valid {
			span.TriggerResourceVersion = triggerRV.String
		}
		if triggerReason.Valid {
			span.TriggerReason = triggerReason.String
		}
		if errMsg.Valid {
			span.Error = errMsg.String
		}

		spans = append(spans, span)
		count = count + 1
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("span row iteration failed: %w", err)
	}

	return spans, nil
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

	if s.insertSpanStmt != nil {
		err := s.insertSpanStmt.Close()
		if err != nil {
			return fmt.Errorf("failed to close span insert statement: %w", err)
		}
	}

	if s.endSpanStmt != nil {
		err := s.endSpanStmt.Close()
		if err != nil {
			return fmt.Errorf("failed to close span end statement: %w", err)
		}
	}

	if s.querySpanStmt != nil {
		err := s.querySpanStmt.Close()
		if err != nil {
			return fmt.Errorf("failed to close span query statement: %w", err)
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
		resource_kind, namespace, name, resource_data, error, duration_ms,
		actor_id, uid, resource_version, generation, verb
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	s.insertStmt, err = s.db.Prepare(insertSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %w", err)
	}

	querySQL := `SELECT id, session_id, sequence_number, timestamp,
	            operation_type, resource_kind, namespace, name,
	            resource_data, error, duration_ms, actor_id, uid, resource_version,
	            generation, verb
	            FROM operations WHERE session_id = ?
	            ORDER BY sequence_number LIMIT ?`

	s.queryStmt, err = s.db.Prepare(querySQL)
	if err != nil {
		return fmt.Errorf("failed to prepare query statement: %w", err)
	}

	spanInsertSQL := `INSERT INTO reconcile_spans (
		id, session_id, actor_id, start_ts, end_ts, duration_ms,
		kind, namespace, name, trigger_uid, trigger_resource_version,
		trigger_reason, error
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	s.insertSpanStmt, err = s.db.Prepare(spanInsertSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare span insert statement: %w", err)
	}

	spanEndSQL := `UPDATE reconcile_spans
		SET end_ts = ?, duration_ms = ?, error = ?
		WHERE id = ?`

	s.endSpanStmt, err = s.db.Prepare(spanEndSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare span end statement: %w", err)
	}

	spanQuerySQL := `SELECT id, session_id, actor_id, start_ts, end_ts, duration_ms,
		kind, namespace, name, trigger_uid, trigger_resource_version,
		trigger_reason, error
		FROM reconcile_spans WHERE session_id = ?
		ORDER BY start_ts LIMIT ?`

	s.querySpanStmt, err = s.db.Prepare(spanQuerySQL)
	if err != nil {
		return fmt.Errorf("failed to prepare span query statement: %w", err)
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
		var actorID sql.NullString
		var uid sql.NullString
		var resourceVersion sql.NullString
		var generation sql.NullInt64
		var verb sql.NullString

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
			&actorID,
			&uid,
			&resourceVersion,
			&generation,
			&verb,
		)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}

		op.Timestamp = time.Unix(timestamp, 0)
		if actorID.Valid {
			op.ActorID = actorID.String
		}
		if uid.Valid {
			op.UID = uid.String
		}
		if resourceVersion.Valid {
			op.ResourceVersion = resourceVersion.String
		}
		if generation.Valid {
			op.Generation = generation.Int64
		}
		if verb.Valid {
			op.Verb = verb.String
		}
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
	err = applySQLiteMigrations(db)
	if err != nil {
		return fmt.Errorf("schema migration failed: %w", err)
	}
	return nil
}
