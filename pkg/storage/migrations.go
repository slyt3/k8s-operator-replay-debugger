package storage

import (
	"database/sql"
	"fmt"
)

// applySQLiteMigrations ensures newer columns and indexes exist on SQLite databases.
func applySQLiteMigrations(db *sql.DB) error {
	err := ensureOperationsColumns(db)
	if err != nil {
		return err
	}

	err = ensureOperationsIndexes(db)
	if err != nil {
		return err
	}

	return nil
}

func ensureOperationsColumns(db *sql.DB) error {
	columns, err := loadSQLiteColumns(db, "operations")
	if err != nil {
		return err
	}

	required := map[string]string{
		"actor_id":         "ALTER TABLE operations ADD COLUMN actor_id TEXT",
		"uid":              "ALTER TABLE operations ADD COLUMN uid TEXT",
		"resource_version": "ALTER TABLE operations ADD COLUMN resource_version TEXT",
		"generation":       "ALTER TABLE operations ADD COLUMN generation INTEGER",
		"verb":             "ALTER TABLE operations ADD COLUMN verb TEXT",
	}

	keys := make([]string, 0, len(required))
	count := 0
	maxKeys := len(required)
	if maxKeys > 20 {
		maxKeys = 20
	}
	for name := range required {
		if count >= maxKeys {
			break
		}
		keys = append(keys, name)
		count = count + 1
	}

	for i := 0; i < len(keys); i++ {
		name := keys[i]
		stmt := required[name]
		if columns[name] {
			continue
		}
		_, err = db.Exec(stmt)
		if err != nil {
			return fmt.Errorf("failed to add column %s: %w", name, err)
		}
	}

	return nil
}

func ensureOperationsIndexes(db *sql.DB) error {
	_, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_uid_rv
ON operations(uid, resource_version);`)
	if err != nil {
		return fmt.Errorf("failed to create idx_uid_rv: %w", err)
	}

	return nil
}

func loadSQLiteColumns(db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s);", table))
	if err != nil {
		return nil, fmt.Errorf("failed to query table info for %s: %w", table, err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			fmt.Printf("Warning: failed to close rows: %v\n", closeErr)
		}
	}()

	columns := make(map[string]bool, 20)
	maxColumns := 200
	count := 0
	for rows.Next() && count < maxColumns {
		var cid int
		var name string
		var ctype string
		var notnull int
		var dflt sql.NullString
		var pk int

		err = rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk)
		if err != nil {
			return nil, fmt.Errorf("failed to scan table info for %s: %w", table, err)
		}
		columns[name] = true
		count = count + 1
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("table info iteration failed for %s: %w", table, err)
	}

	return columns, nil
}
