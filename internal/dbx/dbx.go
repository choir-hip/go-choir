// Package dbx opens Choir's two SQL engines and holds the few helpers that
// differ between them (docs/design/data-system-build-sheet-2026-10-30.md).
//
// Guests use SQLite (modernc.org/sqlite): one write connection, a read pool,
// WAL, synchronous=FULL. The host uses PostgreSQL (pgx stdlib) through a
// connector that rebinds `?` placeholders to `$n`, so query text keeps one
// placeholder style. Upserts are written `INSERT ... ON CONFLICT (...) DO
// UPDATE SET c = excluded.c`, which both engines accept.
package dbx

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// Kind names an engine.
type Kind string

const (
	SQLite   Kind = "sqlite"
	Postgres Kind = "postgres"
)

// SQLiteOptions tunes OpenSQLite. Zero values take the defaults.
type SQLiteOptions struct {
	// ReadConns is the read pool size (default 4).
	ReadConns int
	// BusyTimeout is how long a connection waits on a lock (default 30s).
	BusyTimeout time.Duration
	// ReadOnly opens only the read pool, with the database file read-only.
	ReadOnly bool
}

// SQLiteDB is a SQLite database opened as one serialized writer and a pool of
// readers. Write is the only handle that may write; Read never takes the
// write lock, so readers do not queue behind a long write.
type SQLiteDB struct {
	Path  string
	Write *sql.DB
	Read  *sql.DB
}

// Close closes both pools.
func (d *SQLiteDB) Close() error {
	if d == nil {
		return nil
	}
	var errs []error
	if d.Write != nil {
		errs = append(errs, d.Write.Close())
	}
	if d.Read != nil && d.Read != d.Write {
		errs = append(errs, d.Read.Close())
	}
	return errors.Join(errs...)
}

// SQLiteDSN returns the modernc DSN for path with Choir's pragmas.
func SQLiteDSN(path string, opts SQLiteOptions, readOnly bool) string {
	busy := opts.BusyTimeout
	if busy <= 0 {
		busy = 30 * time.Second
	}
	q := url.Values{}
	q.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", busy.Milliseconds()))
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "synchronous(FULL)")
	q.Add("_time_format", "sqlite")
	if readOnly {
		q.Add("mode", "ro")
	} else {
		q.Add("_txlock", "immediate")
	}
	return "file:" + path + "?" + q.Encode()
}

// OpenSQLite opens path as a SQLiteDB, creating the file if needed.
func OpenSQLite(path string, opts SQLiteOptions) (*SQLiteDB, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("dbx: sqlite path is required")
	}
	readConns := opts.ReadConns
	if readConns <= 0 {
		readConns = 4
	}
	out := &SQLiteDB{Path: path}
	if !opts.ReadOnly {
		w, err := sql.Open("sqlite", SQLiteDSN(path, opts, false))
		if err != nil {
			return nil, fmt.Errorf("dbx: open sqlite writer: %w", err)
		}
		w.SetMaxOpenConns(1)
		w.SetMaxIdleConns(1)
		w.SetConnMaxLifetime(0)
		if err := w.Ping(); err != nil {
			_ = w.Close()
			return nil, fmt.Errorf("dbx: ping sqlite writer: %w", err)
		}
		out.Write = w
	}
	r, err := sql.Open("sqlite", SQLiteDSN(path, opts, true))
	if err != nil {
		_ = out.Close()
		return nil, fmt.Errorf("dbx: open sqlite readers: %w", err)
	}
	r.SetMaxOpenConns(readConns)
	r.SetMaxIdleConns(readConns)
	if err := r.Ping(); err != nil {
		_ = r.Close()
		_ = out.Close()
		return nil, fmt.Errorf("dbx: ping sqlite readers: %w", err)
	}
	out.Read = r
	if out.Write == nil {
		out.Write = r
	}
	return out, nil
}

// PGOptions tunes OpenPostgres. Zero values take the defaults.
type PGOptions struct {
	MaxOpenConns int           // default 16
	MaxIdleConns int           // default 4
	ConnLifetime time.Duration // default 30m
}

// OpenPostgres opens dsn (a libpq URL or key=value string) through the
// rebinding connector.
func OpenPostgres(dsn string, opts PGOptions) (*sql.DB, error) {
	connector, err := newRebindConnector(dsn)
	if err != nil {
		return nil, err
	}
	db := sql.OpenDB(connector)
	maxOpen := opts.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = 16
	}
	maxIdle := opts.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = 4
	}
	life := opts.ConnLifetime
	if life <= 0 {
		life = 30 * time.Minute
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(life)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("dbx: ping postgres: %w", redactDSNError(err))
	}
	return db, nil
}

// ForUpdate is " FOR UPDATE" on PostgreSQL and "" on SQLite, whose single
// writer already serializes the transaction.
func ForUpdate(kind Kind) string {
	if kind == Postgres {
		return " FOR UPDATE"
	}
	return ""
}

// IsUniqueViolation reports a unique or primary-key violation.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	var sqErr *sqlite.Error
	if errors.As(err, &sqErr) {
		code := sqErr.Code()
		return code == sqlite3.SQLITE_CONSTRAINT_UNIQUE || code == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY
	}
	return false
}

// IsRetryable reports a lock or serialization conflict worth retrying:
// SQLITE_BUSY/LOCKED, or PostgreSQL serialization failure and deadlock.
func IsRetryable(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "40001" || pgErr.Code == "40P01"
	}
	var sqErr *sqlite.Error
	if errors.As(err, &sqErr) {
		primary := sqErr.Code() & 0xff
		return primary == sqlite3.SQLITE_BUSY || primary == sqlite3.SQLITE_LOCKED
	}
	return false
}

// Queryer is the read side shared by *sql.DB, *sql.Tx and *sql.Conn.
type Queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// TableExists reports whether table exists (PostgreSQL: in the current
// schema search path).
func TableExists(ctx context.Context, q Queryer, kind Kind, table string) (bool, error) {
	var n int
	var err error
	switch kind {
	case SQLite:
		err = q.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&n)
	case Postgres:
		err = q.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = ?`, table).Scan(&n)
	default:
		return false, fmt.Errorf("dbx: unknown kind %q", kind)
	}
	return n > 0, err
}

// ColumnExists reports whether table has column.
func ColumnExists(ctx context.Context, q Queryer, kind Kind, table, column string) (bool, error) {
	switch kind {
	case SQLite:
		rows, err := q.QueryContext(ctx, `SELECT name FROM pragma_table_info(?)`, table)
		if err != nil {
			return false, err
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return false, err
			}
			if strings.EqualFold(name, column) {
				return true, nil
			}
		}
		return false, rows.Err()
	case Postgres:
		var n int
		err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`, table, column).Scan(&n)
		return n > 0, err
	default:
		return false, fmt.Errorf("dbx: unknown kind %q", kind)
	}
}

// IndexExists reports whether an index named index exists.
func IndexExists(ctx context.Context, q Queryer, kind Kind, index string) (bool, error) {
	var n int
	var err error
	switch kind {
	case SQLite:
		err = q.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?`, index).Scan(&n)
	case Postgres:
		err = q.QueryRowContext(ctx, `SELECT COUNT(*) FROM pg_indexes WHERE schemaname = current_schema() AND indexname = ?`, index).Scan(&n)
	default:
		return false, fmt.Errorf("dbx: unknown kind %q", kind)
	}
	return n > 0, err
}

// Tables lists the user tables, sorted by name.
func Tables(ctx context.Context, q Queryer, kind Kind) ([]string, error) {
	var query string
	switch kind {
	case SQLite:
		query = `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`
	case Postgres:
		query = `SELECT table_name FROM information_schema.tables WHERE table_schema = current_schema() AND table_type = 'BASE TABLE' ORDER BY table_name`
	default:
		return nil, fmt.Errorf("dbx: unknown kind %q", kind)
	}
	rows, err := q.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// redactDSNError keeps a connection error from echoing the DSN's password.
func redactDSNError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if i := strings.Index(msg, "password="); i >= 0 {
		end := strings.IndexAny(msg[i:], " &")
		if end < 0 {
			msg = msg[:i] + "password=REDACTED"
		} else {
			msg = msg[:i] + "password=REDACTED" + msg[i+end:]
		}
	}
	return errors.New(msg)
}
