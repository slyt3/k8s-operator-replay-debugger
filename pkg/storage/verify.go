package storage

import (
	"database/sql"
	"fmt"
	"os"
)

// VerifyStats summarizes basic database counts.
type VerifyStats struct {
	Sessions   int64
	Operations int64
	Spans      int64
}

// VerifyResult captures verification findings.
type VerifyResult struct {
	Errors   []string
	Warnings []string
	Stats    VerifyStats
}

// VerifySQLite checks schema and basic consistency for a SQLite database.
func VerifySQLite(path string, strict bool) (*VerifyResult, error) {
	if path == "" {
		return nil, fmt.Errorf("database path is empty")
	}

	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("database not found: %w", err)
	}

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer func() {
		_ = db.Close()
	}()

	result := &VerifyResult{
		Errors:   make([]string, 0, 8),
		Warnings: make([]string, 0, 8),
	}

	tables, err := loadSQLiteTables(db)
	if err != nil {
		return nil, err
	}

	if !tables["operations"] {
		result.Errors = append(result.Errors, "missing table: operations")
		return result, nil
	}

	if !tables["reconcile_spans"] {
		result.Warnings = append(result.Warnings, "missing table: reconcile_spans")
	}

	err = verifyOperationsSchema(db, result, strict)
	if err != nil {
		return nil, err
	}

	if tables["reconcile_spans"] {
		err = verifySpansSchema(db, result)
		if err != nil {
			return nil, err
		}
	}

	err = verifyOperationsData(db, result)
	if err != nil {
		return nil, err
	}

	if tables["reconcile_spans"] {
		err = verifySpanData(db, result)
		if err != nil {
			return nil, err
		}
	}

	err = loadStats(db, tables["reconcile_spans"], result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func verifyOperationsSchema(db *sql.DB, result *VerifyResult, strict bool) error {
	columns, err := loadSQLiteColumns(db, "operations")
	if err != nil {
		return err
	}

	required := []string{
		"id",
		"session_id",
		"sequence_number",
		"timestamp",
		"operation_type",
		"resource_kind",
		"namespace",
		"name",
		"resource_data",
		"error",
		"duration_ms",
	}

	optional := []string{
		"actor_id",
		"uid",
		"resource_version",
		"generation",
		"verb",
	}

	for i := 0; i < len(required); i++ {
		name := required[i]
		if !columns[name] {
			result.Errors = append(result.Errors, fmt.Sprintf("missing column: operations.%s", name))
		}
	}

	for i := 0; i < len(optional); i++ {
		name := optional[i]
		if !columns[name] {
			if strict {
				result.Errors = append(result.Errors, fmt.Sprintf("missing column: operations.%s", name))
			} else {
				result.Warnings = append(result.Warnings, fmt.Sprintf("missing column: operations.%s", name))
			}
		}
	}

	indexes, err := loadSQLiteIndexes(db, "operations")
	if err != nil {
		return err
	}

	if !indexes["idx_uid_rv"] {
		result.Warnings = append(result.Warnings, "missing index: idx_uid_rv")
	}

	return nil
}

func verifySpansSchema(db *sql.DB, result *VerifyResult) error {
	columns, err := loadSQLiteColumns(db, "reconcile_spans")
	if err != nil {
		return err
	}

	required := []string{
		"id",
		"session_id",
		"actor_id",
		"start_ts",
		"end_ts",
		"duration_ms",
		"kind",
		"namespace",
		"name",
		"trigger_uid",
		"trigger_resource_version",
		"trigger_reason",
		"error",
	}

	for i := 0; i < len(required); i++ {
		name := required[i]
		if !columns[name] {
			result.Errors = append(result.Errors, fmt.Sprintf("missing column: reconcile_spans.%s", name))
		}
	}

	return nil
}

func verifyOperationsData(db *sql.DB, result *VerifyResult) error {
	var dupSession string
	var dupSeq int64
	var dupCount int64

	dupRow := db.QueryRow(`SELECT session_id, sequence_number, COUNT(*)
		FROM operations
		GROUP BY session_id, sequence_number
		HAVING COUNT(*) > 1
		LIMIT 1`)
	err := dupRow.Scan(&dupSession, &dupSeq, &dupCount)
	if err == nil {
		result.Errors = append(result.Errors, fmt.Sprintf("duplicate sequence: session=%s seq=%d count=%d", dupSession, dupSeq, dupCount))
	} else if err != sql.ErrNoRows {
		return fmt.Errorf("failed to scan duplicate sequence check: %w", err)
	}

	var negDurCount int64
	err = db.QueryRow(`SELECT COUNT(*) FROM operations WHERE duration_ms < 0`).Scan(&negDurCount)
	if err != nil {
		return fmt.Errorf("failed to scan negative duration check: %w", err)
	}
	if negDurCount > 0 {
		result.Errors = append(result.Errors, "operations with negative duration_ms")
	}

	rows, err := db.Query(`SELECT session_id, MIN(sequence_number), MAX(sequence_number), COUNT(*)
		FROM operations GROUP BY session_id LIMIT 1000`)
	if err != nil {
		return fmt.Errorf("failed to scan sequence gaps: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	for rows.Next() {
		var sessionID string
		var minSeq int64
		var maxSeq int64
		var count int64
		err = rows.Scan(&sessionID, &minSeq, &maxSeq, &count)
		if err != nil {
			return fmt.Errorf("failed to read sequence stats: %w", err)
		}
		expected := maxSeq - minSeq + 1
		if expected != count {
			result.Warnings = append(result.Warnings, fmt.Sprintf("sequence gaps: session=%s expected=%d actual=%d", sessionID, expected, count))
		}
	}
	if err = rows.Err(); err != nil {
		return fmt.Errorf("sequence stats iteration failed: %w", err)
	}

	var tsSession string
	tsRow := db.QueryRow(`SELECT o1.session_id
		FROM operations o1
		JOIN operations o2
		  ON o1.session_id = o2.session_id
		 AND o1.sequence_number + 1 = o2.sequence_number
		WHERE o2.timestamp < o1.timestamp
		LIMIT 1`)
	err = tsRow.Scan(&tsSession)
	if err == nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("non-monotonic timestamps detected in session=%s", tsSession))
	} else if err != sql.ErrNoRows {
		return fmt.Errorf("failed to scan timestamp monotonicity: %w", err)
	}

	return nil
}

func verifySpanData(db *sql.DB, result *VerifyResult) error {
	var openCount int64
	err := db.QueryRow(`SELECT COUNT(*) FROM reconcile_spans WHERE end_ts IS NULL`).Scan(&openCount)
	if err != nil {
		return fmt.Errorf("failed to scan open spans: %w", err)
	}
	if openCount > 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("open spans: %d", openCount))
	}

	var negativeCount int64
	err = db.QueryRow(`SELECT COUNT(*) FROM reconcile_spans WHERE duration_ms < 0`).Scan(&negativeCount)
	if err != nil {
		return fmt.Errorf("failed to scan negative span durations: %w", err)
	}
	if negativeCount > 0 {
		result.Errors = append(result.Errors, "reconcile spans with negative duration_ms")
	}

	var invalidEndCount int64
	err = db.QueryRow(`SELECT COUNT(*) FROM reconcile_spans WHERE end_ts IS NOT NULL AND end_ts < start_ts`).Scan(&invalidEndCount)
	if err != nil {
		return fmt.Errorf("failed to scan invalid span end timestamps: %w", err)
	}
	if invalidEndCount > 0 {
		result.Errors = append(result.Errors, "reconcile spans with end_ts before start_ts")
	}

	var durationMismatch int64
	err = db.QueryRow(`SELECT COUNT(*)
		FROM reconcile_spans
		WHERE duration_ms IS NOT NULL AND end_ts IS NULL`).Scan(&durationMismatch)
	if err != nil {
		return fmt.Errorf("failed to scan span duration mismatch: %w", err)
	}
	if durationMismatch > 0 {
		result.Warnings = append(result.Warnings, "spans with duration_ms but missing end_ts")
	}

	return nil
}

func loadStats(db *sql.DB, hasSpans bool, result *VerifyResult) error {
	err := db.QueryRow(`SELECT COUNT(DISTINCT session_id), COUNT(*) FROM operations`).Scan(
		&result.Stats.Sessions,
		&result.Stats.Operations,
	)
	if err != nil {
		return fmt.Errorf("failed to load operation stats: %w", err)
	}

	if hasSpans {
		err = db.QueryRow(`SELECT COUNT(*) FROM reconcile_spans`).Scan(&result.Stats.Spans)
		if err != nil {
			return fmt.Errorf("failed to load span stats: %w", err)
		}
	}

	return nil
}

func loadSQLiteTables(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table'`)
	if err != nil {
		return nil, fmt.Errorf("failed to query sqlite_master: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	tables := make(map[string]bool, 10)
	maxRows := 200
	count := 0
	for rows.Next() && count < maxRows {
		var name string
		err = rows.Scan(&name)
		if err != nil {
			return nil, fmt.Errorf("failed to scan sqlite_master: %w", err)
		}
		tables[name] = true
		count = count + 1
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite_master iteration failed: %w", err)
	}

	return tables, nil
}

func loadSQLiteIndexes(db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA index_list(%s);", table))
	if err != nil {
		return nil, fmt.Errorf("failed to query index_list for %s: %w", table, err)
	}
	defer func() {
		_ = rows.Close()
	}()

	indexes := make(map[string]bool, 10)
	maxRows := 200
	count := 0
	for rows.Next() && count < maxRows {
		var seq int
		var name string
		var unique int
		var origin string
		var partial int

		err = rows.Scan(&seq, &name, &unique, &origin, &partial)
		if err != nil {
			return nil, fmt.Errorf("failed to scan index_list for %s: %w", table, err)
		}
		indexes[name] = true
		count = count + 1
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("index_list iteration failed for %s: %w", table, err)
	}

	return indexes, nil
}
