package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/slyt3/kubestep/internal/assert"
)

const (
	maxDatabasePathLength = 4096
	defaultMaxOperations  = 1000000
	maxQueryResults       = 10000
)

// NewDatabase creates and initializes a database connection.
// Rule 3: Pre-allocates all statements, no dynamic allocation after init.
// Rule 5: Multiple assertions for validation.
func NewDatabase(path string, maxOps int) (*Database, error) {
	err := assert.AssertStringNotEmpty(path, "database path")
	if err != nil {
		return nil, err
	}

	err = assert.AssertInRange(len(path), 1, maxDatabasePathLength, "path length")
	if err != nil {
		return nil, err
	}

	err = assert.AssertInRange(maxOps, 1, defaultMaxOperations, "max operations")
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	err = initializeSchema(db)
	if err != nil {
		closeErr := db.Close()
		if closeErr != nil {
			return nil, fmt.Errorf("schema init failed: %w, close failed: %v",
				err, closeErr)
		}
		return nil, err
	}

	insertStmt, err := prepareInsertStatement(db)
	if err != nil {
		closeErr := db.Close()
		if closeErr != nil {
			return nil, fmt.Errorf("insert prep failed: %w, close failed: %v",
				err, closeErr)
		}
		return nil, err
	}

	queryStmt, err := prepareQueryStatement(db)
	if err != nil {
		closeErr := insertStmt.Close()
		if closeErr != nil {
			return nil, fmt.Errorf("query prep failed: %w, stmt close failed: %v",
				err, closeErr)
		}
		closeErr = db.Close()
		if closeErr != nil {
			return nil, fmt.Errorf("query prep failed: %w, db close failed: %v",
				err, closeErr)
		}
		return nil, err
	}

	sessionStmt, err := prepareSessionStatement(db)
	if err != nil {
		cleanupErr := cleanupOnError(db, insertStmt, queryStmt)
		if cleanupErr != nil {
			return nil, fmt.Errorf("session prep failed: %w, cleanup failed: %v", err, cleanupErr)
		}
		return nil, err
	}

	insertSpanStmt, endSpanStmt, querySpanStmt, err := prepareSpanStatements(db)
	if err != nil {
		cleanupErr := cleanupOnError(db, insertStmt, queryStmt, sessionStmt)
		if cleanupErr != nil {
			return nil, fmt.Errorf("span prep failed: %w, cleanup failed: %v", err, cleanupErr)
		}
		return nil, err
	}

	return &Database{
		db:             db,
		insertStmt:     insertStmt,
		queryStmt:      queryStmt,
		sessionStmt:    sessionStmt,
		insertSpanStmt: insertSpanStmt,
		endSpanStmt:    endSpanStmt,
		querySpanStmt:  querySpanStmt,
		maxOperations:  maxOps,
	}, nil
}

// Rule 4: Function under 60 lines.
func initializeSchema(db *sql.DB) error {
	err := assert.AssertNotNil(db, "database")
	if err != nil {
		return err
	}

	_, err = db.Exec(Schema)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	err = applySQLiteMigrations(db)
	if err != nil {
		return fmt.Errorf("schema migration failed: %w", err)
	}

	return nil
}

// Rule 4: Function under 60 lines, single responsibility.
func prepareInsertStatement(db *sql.DB) (*sql.Stmt, error) {
	err := assert.AssertNotNil(db, "database")
	if err != nil {
		return nil, err
	}

	query := `INSERT INTO operations 
		(session_id, sequence_number, timestamp, operation_type, 
		 resource_kind, namespace, name, resource_data, error, duration_ms,
		 actor_id, uid, resource_version, generation, verb)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	stmt, err := db.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare insert statement: %w", err)
	}

	return stmt, nil
}

func prepareQueryStatement(db *sql.DB) (*sql.Stmt, error) {
	err := assert.AssertNotNil(db, "database")
	if err != nil {
		return nil, err
	}

	query := `SELECT id, session_id, sequence_number, timestamp, 
		operation_type, resource_kind, namespace, name, 
		resource_data, error, duration_ms, actor_id, uid, resource_version,
		generation, verb
		FROM operations WHERE session_id = ? 
		ORDER BY sequence_number LIMIT ?`

	stmt, err := db.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare query statement: %w", err)
	}

	return stmt, nil
}

func prepareSessionStatement(db *sql.DB) (*sql.Stmt, error) {
	err := assert.AssertNotNil(db, "database")
	if err != nil {
		return nil, err
	}

	query := `SELECT DISTINCT session_id, 
		MIN(timestamp) as start_time,
		COUNT(*) as op_count
		FROM operations 
		GROUP BY session_id 
		ORDER BY start_time DESC 
		LIMIT ?`

	stmt, err := db.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare session statement: %w", err)
	}

	return stmt, nil
}

func prepareSpanStatements(db *sql.DB) (*sql.Stmt, *sql.Stmt, *sql.Stmt, error) {
	err := assert.AssertNotNil(db, "database")
	if err != nil {
		return nil, nil, nil, err
	}

	insertSQL := `INSERT INTO reconcile_spans (
		id, session_id, actor_id, start_ts, end_ts, duration_ms,
		kind, namespace, name, trigger_uid, trigger_resource_version,
		trigger_reason, error
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	insertStmt, err := db.Prepare(insertSQL)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to prepare span insert: %w", err)
	}

	endSQL := `UPDATE reconcile_spans
		SET end_ts = ?, duration_ms = ?, error = ?
		WHERE id = ?`

	endStmt, err := db.Prepare(endSQL)
	if err != nil {
		closeErr := insertStmt.Close()
		if closeErr != nil {
			return nil, nil, nil, fmt.Errorf("failed to prepare span end: %w, close failed: %v",
				err, closeErr)
		}
		return nil, nil, nil, fmt.Errorf("failed to prepare span end: %w", err)
	}

	querySQL := `SELECT id, session_id, actor_id, start_ts, end_ts, duration_ms,
		kind, namespace, name, trigger_uid, trigger_resource_version,
		trigger_reason, error
		FROM reconcile_spans WHERE session_id = ?
		ORDER BY start_ts LIMIT ?`

	queryStmt, err := db.Prepare(querySQL)
	if err != nil {
		closeErr := insertStmt.Close()
		if closeErr != nil {
			return nil, nil, nil, fmt.Errorf("failed to prepare span query: %w, close failed: %v",
				err, closeErr)
		}
		closeErr = endStmt.Close()
		if closeErr != nil {
			return nil, nil, nil, fmt.Errorf("failed to prepare span query: %w, close failed: %v",
				err, closeErr)
		}
		return nil, nil, nil, fmt.Errorf("failed to prepare span query: %w", err)
	}

	return insertStmt, endStmt, queryStmt, nil
}

func cleanupOnError(db *sql.DB, stmts ...*sql.Stmt) error {
	var lastErr error

	maxCleanup := 10
	count := 0
	for count < maxCleanup && count < len(stmts) {
		if stmts[count] != nil {
			err := stmts[count].Close()
			if err != nil {
				lastErr = err
			}
		}
		count = count + 1
	}

	err := db.Close()
	if err != nil {
		lastErr = err
	}

	return lastErr
}

// InsertOperation adds a new operation to the database.
// Rule 2: Bounded operation, no unbounded loops.
// Rule 5: Multiple assertions.
// Rule 7: All return values checked.
func (d *Database) InsertOperation(op *Operation) error {
	err := assert.AssertNotNil(d, "database")
	if err != nil {
		return err
	}

	err = assert.AssertNotNil(d.insertStmt, "insert statement")
	if err != nil {
		return err
	}

	err = ValidateOperation(op)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	timestampUnix := op.Timestamp.Unix()

	_, err = d.insertStmt.Exec(
		op.SessionID,
		op.SequenceNumber,
		timestampUnix,
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

// Close releases database resources.
// Rule 7: All return values checked and propagated.
func (d *Database) Close() error {
	err := assert.AssertNotNil(d, "database")
	if err != nil {
		return err
	}

	var lastErr error

	if d.insertStmt != nil {
		err = d.insertStmt.Close()
		if err != nil {
			lastErr = fmt.Errorf("insert stmt close failed: %w", err)
		}
	}

	if d.queryStmt != nil {
		err = d.queryStmt.Close()
		if err != nil {
			lastErr = fmt.Errorf("query stmt close failed: %w", err)
		}
	}

	if d.sessionStmt != nil {
		err = d.sessionStmt.Close()
		if err != nil {
			lastErr = fmt.Errorf("session stmt close failed: %w", err)
		}
	}

	if d.insertSpanStmt != nil {
		err = d.insertSpanStmt.Close()
		if err != nil {
			lastErr = fmt.Errorf("span insert stmt close failed: %w", err)
		}
	}

	if d.endSpanStmt != nil {
		err = d.endSpanStmt.Close()
		if err != nil {
			lastErr = fmt.Errorf("span end stmt close failed: %w", err)
		}
	}

	if d.querySpanStmt != nil {
		err = d.querySpanStmt.Close()
		if err != nil {
			lastErr = fmt.Errorf("span query stmt close failed: %w", err)
		}
	}

	if d.db != nil {
		err = d.db.Close()
		if err != nil {
			lastErr = fmt.Errorf("db close failed: %w", err)
		}
	}

	return lastErr
}

// QueryOperations retrieves operations for a session.
// Rule 2: Bounded by maxQueryResults limit.
func (d *Database) QueryOperations(sessionID string) ([]Operation, error) {
	err := assert.AssertNotNil(d, "database")
	if err != nil {
		return nil, err
	}

	err = assert.AssertStringNotEmpty(sessionID, "session_id")
	if err != nil {
		return nil, err
	}

	rows, err := d.queryStmt.Query(sessionID, maxQueryResults)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			err = closeErr
		}
	}()

	operations := make([]Operation, 0, maxQueryResults)
	count := 0

	for count < maxQueryResults && rows.Next() {
		var op Operation
		var timestampUnix int64
		var opType string
		var actorID sql.NullString
		var uid sql.NullString
		var resourceVersion sql.NullString
		var generation sql.NullInt64
		var verb sql.NullString

		err = rows.Scan(
			&op.ID,
			&op.SessionID,
			&op.SequenceNumber,
			&timestampUnix,
			&opType,
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

		op.Timestamp = time.Unix(timestampUnix, 0)
		op.OperationType = OperationType(opType)
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

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("row iteration failed: %w", err)
	}

	return operations, nil
}

// InsertReconcileSpan inserts a reconcile span record.
func (d *Database) InsertReconcileSpan(span *ReconcileSpan) error {
	err := assert.AssertNotNil(d, "database")
	if err != nil {
		return err
	}

	err = assert.AssertNotNil(d.insertSpanStmt, "span insert statement")
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

	_, err = d.insertSpanStmt.Exec(
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
func (d *Database) EndReconcileSpan(
	spanID string,
	endTime time.Time,
	durationMs int64,
	errMsg string,
) error {
	err := assert.AssertNotNil(d, "database")
	if err != nil {
		return err
	}

	err = assert.AssertNotNil(d.endSpanStmt, "span end statement")
	if err != nil {
		return err
	}

	err = assert.AssertStringNotEmpty(spanID, "span id")
	if err != nil {
		return err
	}

	_, err = d.endSpanStmt.Exec(
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
func (d *Database) QueryReconcileSpans(sessionID string) ([]ReconcileSpan, error) {
	err := assert.AssertNotNil(d, "database")
	if err != nil {
		return nil, err
	}

	err = assert.AssertStringNotEmpty(sessionID, "session_id")
	if err != nil {
		return nil, err
	}

	rows, err := d.querySpanStmt.Query(sessionID, maxQueryResults)
	if err != nil {
		return nil, fmt.Errorf("span query failed: %w", err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			fmt.Printf("Warning: failed to close rows: %v\n", closeErr)
		}
	}()

	spans := make([]ReconcileSpan, 0, maxQueryResults)
	count := 0

	for count < maxQueryResults && rows.Next() {
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
