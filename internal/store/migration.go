package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "modernc.org/sqlite"
)

var runtimeTables = []string{
	"agents",
	"runs",
	"events",
	"channel_messages",
	"inbox_deliveries",
	"run_memory_entries",
	"promotion_candidates",
	"run_acceptances",
	"run_continuations",
	"browser_sessions",
	"research_findings",
	"worker_updates",
	"media_progress",
	"media_recents",
	"user_preferences",
	"desktop_state",
	"desktop_workspaces",
}

func (s *Store) importLegacySQLiteRuntime(dbPath string) error {
	if strings.TrimSpace(dbPath) == "" {
		return nil
	}
	info, err := os.Stat(dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat legacy sqlite path: %w", err)
	}
	if info.IsDir() || info.Size() == 0 {
		return nil
	}
	empty, err := s.runtimeTablesEmpty(context.Background())
	if err != nil {
		return err
	}
	if !empty {
		return nil
	}

	source, err := sql.Open("sqlite", dbPath+"?_busy_timeout=60000")
	if err != nil {
		return fmt.Errorf("open legacy sqlite %s: %w", dbPath, err)
	}
	defer func() { _ = source.Close() }()

	for _, table := range runtimeTables {
		exists, err := sqliteTableExists(source, table)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}
		if err := s.importSQLiteTable(context.Background(), source, table); err != nil {
			return err
		}
	}
	return s.backfillDerivedRuntimeState()
}

func (s *Store) runtimeTablesEmpty(ctx context.Context) (bool, error) {
	for _, table := range runtimeTables {
		if err := validateIdentifier(table); err != nil {
			return false, err
		}
		var count int
		if err := s.db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count); err != nil {
			return false, fmt.Errorf("count runtime table %s: %w", table, err)
		}
		if count > 0 {
			return false, nil
		}
	}
	return true, nil
}

func sqliteTableExists(db *sql.DB, table string) (bool, error) {
	var name string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect legacy sqlite table %s: %w", table, err)
	}
	return name == table, nil
}

func (s *Store) importSQLiteTable(ctx context.Context, source *sql.DB, table string) error {
	if err := validateIdentifier(table); err != nil {
		return err
	}
	cols, err := sqliteTableColumns(source, table)
	if err != nil {
		return err
	}
	if len(cols) == 0 {
		return nil
	}

	selectSQL := fmt.Sprintf("SELECT %s FROM %s", joinIdentifiers(cols), table)
	rows, err := source.QueryContext(ctx, selectSQL)
	if err != nil {
		return fmt.Errorf("query legacy sqlite table %s: %w", table, err)
	}
	defer func() { _ = rows.Close() }()

	placeholders := make([]string, len(cols))
	for i := range placeholders {
		placeholders[i] = "?"
	}
	insertSQL := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		table,
		joinIdentifiers(cols),
		strings.Join(placeholders, ", "),
	)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin import table %s: %w", table, err)
	}
	defer func() { _ = tx.Rollback() }()

	for rows.Next() {
		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return fmt.Errorf("scan legacy sqlite table %s: %w", table, err)
		}
		args := make([]any, len(values))
		for i, v := range values {
			if b, ok := v.([]byte); ok {
				args[i] = string(b)
			} else {
				args[i] = v
			}
		}
		if _, err := tx.ExecContext(ctx, insertSQL, args...); err != nil {
			return fmt.Errorf("insert migrated row into %s: %w", table, err)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate legacy sqlite table %s: %w", table, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit import table %s: %w", table, err)
	}
	return nil
}

func sqliteTableColumns(db *sql.DB, table string) ([]string, error) {
	if err := validateIdentifier(table); err != nil {
		return nil, err
	}
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return nil, fmt.Errorf("pragma legacy sqlite table_info(%s): %w", table, err)
	}
	defer func() { _ = rows.Close() }()

	var cols []string
	for rows.Next() {
		var (
			cid      int
			name     string
			colType  string
			notNull  int
			defaultV sql.NullString
			primaryK int
		)
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultV, &primaryK); err != nil {
			return nil, fmt.Errorf("scan legacy sqlite table_info(%s): %w", table, err)
		}
		if err := validateIdentifier(name); err != nil {
			return nil, err
		}
		cols = append(cols, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate legacy sqlite table_info(%s): %w", table, err)
	}
	return cols, nil
}

func joinIdentifiers(cols []string) string {
	quoted := make([]string, len(cols))
	for i, col := range cols {
		quoted[i] = "`" + col + "`"
	}
	return strings.Join(quoted, ", ")
}
