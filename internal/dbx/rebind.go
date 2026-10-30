package dbx

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// Rebind rewrites `?` placeholders to PostgreSQL's `$1, $2, ...`. It leaves
// `?` alone inside single-quoted strings, double-quoted identifiers,
// dollar-quoted bodies and comments. Queries must not use PostgreSQL's
// JSONB `?` operators; use jsonb_exists() instead.
func Rebind(query string) string {
	if !strings.Contains(query, "?") {
		return query
	}
	var b strings.Builder
	b.Grow(len(query) + 16)
	n := 0
	for i := 0; i < len(query); i++ {
		c := query[i]
		switch {
		case c == '\'' || c == '"':
			j := i + 1
			for j < len(query) {
				if query[j] == c {
					if j+1 < len(query) && query[j+1] == c { // doubled quote escape
						j += 2
						continue
					}
					break
				}
				j++
			}
			if j >= len(query) {
				j = len(query) - 1
			}
			b.WriteString(query[i : j+1])
			i = j
		case c == '-' && i+1 < len(query) && query[i+1] == '-':
			j := strings.IndexByte(query[i:], '\n')
			if j < 0 {
				b.WriteString(query[i:])
				i = len(query)
			} else {
				b.WriteString(query[i : i+j+1])
				i += j
			}
		case c == '/' && i+1 < len(query) && query[i+1] == '*':
			j := strings.Index(query[i+2:], "*/")
			if j < 0 {
				b.WriteString(query[i:])
				i = len(query)
			} else {
				b.WriteString(query[i : i+2+j+2])
				i += 2 + j + 1
			}
		case c == '$':
			// Dollar quote: $tag$ ... $tag$ (tag may be empty). A $n
			// positional placeholder already present is copied through.
			j := i + 1
			for j < len(query) && (isIdentByte(query[j])) {
				j++
			}
			if j < len(query) && query[j] == '$' {
				tag := query[i : j+1]
				end := strings.Index(query[j+1:], tag)
				if end >= 0 {
					stop := j + 1 + end + len(tag)
					b.WriteString(query[i:stop])
					i = stop - 1
					continue
				}
			}
			b.WriteByte(c)
		case c == '?':
			n++
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(n))
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

func isIdentByte(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// rebindConnector wraps the pgx stdlib connector so every query is rebound.
type rebindConnector struct {
	inner driver.Connector
}

func newRebindConnector(dsn string) (driver.Connector, error) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("dbx: parse postgres DSN: %w", redactDSNError(err))
	}
	return &rebindConnector{inner: stdlib.GetConnector(*cfg)}, nil
}

func (c *rebindConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.inner.Connect(ctx)
	if err != nil {
		return nil, redactDSNError(err)
	}
	return &rebindConn{inner: conn}, nil
}

func (c *rebindConnector) Driver() driver.Driver { return c.inner.Driver() }

// rebindConn forwards every optional driver interface pgx implements,
// rebinding query text on the way through.
type rebindConn struct {
	inner driver.Conn
}

var (
	_ driver.Conn               = (*rebindConn)(nil)
	_ driver.ConnPrepareContext = (*rebindConn)(nil)
	_ driver.ExecerContext      = (*rebindConn)(nil)
	_ driver.QueryerContext     = (*rebindConn)(nil)
	_ driver.ConnBeginTx        = (*rebindConn)(nil)
	_ driver.Pinger             = (*rebindConn)(nil)
	_ driver.SessionResetter    = (*rebindConn)(nil)
	_ driver.Validator          = (*rebindConn)(nil)
	_ driver.NamedValueChecker  = (*rebindConn)(nil)
)

var errUnsupported = errors.New("dbx: driver capability not supported")

func (c *rebindConn) Prepare(query string) (driver.Stmt, error) {
	return c.inner.Prepare(Rebind(query))
}

func (c *rebindConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if p, ok := c.inner.(driver.ConnPrepareContext); ok {
		return p.PrepareContext(ctx, Rebind(query))
	}
	return c.inner.Prepare(Rebind(query))
}

func (c *rebindConn) Close() error { return c.inner.Close() }

func (c *rebindConn) Begin() (driver.Tx, error) { //nolint:staticcheck // driver interface
	return c.inner.Begin() //nolint:staticcheck
}

func (c *rebindConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if b, ok := c.inner.(driver.ConnBeginTx); ok {
		return b.BeginTx(ctx, opts)
	}
	return nil, errUnsupported
}

func (c *rebindConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if e, ok := c.inner.(driver.ExecerContext); ok {
		return e.ExecContext(ctx, Rebind(query), args)
	}
	return nil, driver.ErrSkip
}

func (c *rebindConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if q, ok := c.inner.(driver.QueryerContext); ok {
		return q.QueryContext(ctx, Rebind(query), args)
	}
	return nil, driver.ErrSkip
}

func (c *rebindConn) Ping(ctx context.Context) error {
	if p, ok := c.inner.(driver.Pinger); ok {
		return p.Ping(ctx)
	}
	return nil
}

func (c *rebindConn) ResetSession(ctx context.Context) error {
	if r, ok := c.inner.(driver.SessionResetter); ok {
		return r.ResetSession(ctx)
	}
	return nil
}

func (c *rebindConn) IsValid() bool {
	if v, ok := c.inner.(driver.Validator); ok {
		return v.IsValid()
	}
	return true
}

func (c *rebindConn) CheckNamedValue(nv *driver.NamedValue) error {
	if n, ok := c.inner.(driver.NamedValueChecker); ok {
		return n.CheckNamedValue(nv)
	}
	return driver.ErrSkip
}

// Raw exposes the underlying pgx stdlib connection (for CopyFrom in the
// migrator via sql.Conn.Raw).
func (c *rebindConn) Raw() driver.Conn { return c.inner }
