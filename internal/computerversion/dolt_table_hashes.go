package computerversion

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// DoltQueryer is the read surface DoltTableHashes needs: a *sql.Conn, a
// *sql.Tx or a *sql.DB.
type DoltQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// OperationalTables names the tables a hash-only witness leaves out: those
// with wall-clock columns (IgnoredContentColumns), whose Dolt hash differs
// between a live store and a replay by construction
// (docs/problems/paper-never-checkpointed-2026-10-27.md, S3), sorted.
func (e DoltStateExtractor) OperationalTables() []string {
	out := make([]string, 0, len(e.IgnoredContentColumns))
	for table := range e.IgnoredContentColumns {
		out = append(out, table)
	}
	sort.Strings(out)
	return out
}

// TableHashes reads HASHOF('HEAD') and DOLT_HASHOF_TABLE for every table the
// extractor observes, minus the operational tables, through q. It reads no
// row: each hash is the root of the table's prolly tree. The caller gives q a
// transaction when it needs one root for every table.
func (e DoltStateExtractor) TableHashes(ctx context.Context, q DoltQueryer) (string, map[string]string, error) {
	var head string
	if err := q.QueryRowContext(ctx, "SELECT HASHOF('HEAD')").Scan(&head); err != nil {
		return "", nil, fmt.Errorf("dolt table hashes: query HEAD: %w", err)
	}
	rows, err := q.QueryContext(ctx, "SHOW TABLES")
	if err != nil {
		return "", nil, fmt.Errorf("dolt table hashes: show tables: %w", err)
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return "", nil, fmt.Errorf("dolt table hashes: scan table name: %w", err)
		}
		tables = append(tables, name)
	}
	if err := rows.Close(); err != nil {
		return "", nil, err
	}
	if err := rows.Err(); err != nil {
		return "", nil, err
	}
	out := map[string]string{}
	for _, table := range tables {
		if _, ignored := e.IgnoredTables[table]; ignored {
			continue
		}
		if _, operational := e.IgnoredContentColumns[table]; operational {
			continue
		}
		if _, optional := optionalTables[table]; optional {
			var one int
			err := q.QueryRowContext(ctx, fmt.Sprintf("SELECT 1 FROM `%s` LIMIT 1", table)).Scan(&one)
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				return "", nil, fmt.Errorf("dolt table hashes: probe optional table %s: %w", table, err)
			}
		}
		var h string
		if err := q.QueryRowContext(ctx, "SELECT DOLT_HASHOF_TABLE(?)", table).Scan(&h); err != nil {
			return "", nil, fmt.Errorf("dolt table hashes: %s: %w", table, err)
		}
		out[table] = strings.TrimSpace(h)
	}
	return strings.TrimSpace(head), out, nil
}

// WorkspaceTableHashes is TableHashes over a fresh connection to the
// extractor's workspace, in one transaction.
func (e DoltStateExtractor) WorkspaceTableHashes(ctx context.Context) (string, map[string]string, error) {
	db, connector, err := openDoltWorkspace(strings.TrimSpace(e.WorkspacePath), strings.TrimSpace(e.Database), "Choir", "system@choir.local")
	if err != nil {
		return "", nil, fmt.Errorf("dolt table hashes: %w", err)
	}
	defer func() {
		_ = db.Close()
		if connector != nil {
			_ = connector.Close()
		}
	}()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", nil, fmt.Errorf("dolt table hashes: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	return e.TableHashes(ctx, tx)
}
