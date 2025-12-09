package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/operator-replay-debugger/internal/assert"
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

	return &Database{
		db:            db,
		insertStmt:    insertStmt,
		queryStmt:     queryStmt,
		sessionStmt:   sessionStmt,
		maxOperations: maxOps,
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
		 resource_kind, namespace, name, resource_data, error, duration_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

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
		resource_data, error, duration_ms
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
		)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}

		op.Timestamp = time.Unix(timestampUnix, 0)
		op.OperationType = OperationType(opType)

		operations = append(operations, op)
		count = count + 1
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("row iteration failed: %w", err)
	}

	return operations, nil
}
