// Package doltsrc opens a legacy embedded Dolt workspace as a read source for
// the one-shot migrators. With internal/migrate/hostcopy (the MySQL driver),
// it is one of the only packages that may import the Dolt driver after the
// data system cutover; it is deleted with them after C5.
package doltsrc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	embedded "github.com/dolthub/driver/v2"
)

// DatabaseName is the database inside a legacy guest workspace.
const DatabaseName = "texture"

// Exists reports whether workspace is a directory.
func Exists(workspace string) bool {
	st, err := os.Stat(workspace)
	return err == nil && st.IsDir()
}

// Handle is an open embedded Dolt database. It issues reads only.
type Handle struct {
	DB        *sql.DB
	connector interface{ Close() error }
}

// Close closes the pool, then the embedded engine.
func (h *Handle) Close() error {
	if h == nil {
		return nil
	}
	err := h.DB.Close()
	return errors.Join(err, h.connector.Close())
}

func newBackOff() backoff.BackOff {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = 25 * time.Millisecond
	b.RandomizationFactor = 0.2
	b.Multiplier = 1.6
	b.MaxInterval = 250 * time.Millisecond
	b.MaxElapsedTime = 2 * time.Second
	b.Reset()
	return backoff.WithMaxRetries(b, 8)
}

// Open opens database in the embedded Dolt workspace directory. It does not
// create anything: a missing workspace or database is an error.
func Open(ctx context.Context, workspace, database string) (*Handle, error) {
	if !Exists(workspace) {
		return nil, fmt.Errorf("doltsrc: no workspace at %s", workspace)
	}
	// Check the database exists first, on a root connection.
	root, err := openDSN(fmt.Sprintf("file://%s?commitname=Choir&commitemail=system@choir.local&multistatements=true", workspace))
	if err != nil {
		return nil, err
	}
	found := false
	rows, err := root.DB.QueryContext(ctx, "SHOW DATABASES")
	if err == nil {
		for rows.Next() {
			var n string
			if err = rows.Scan(&n); err != nil {
				break
			}
			if strings.EqualFold(n, database) {
				found = true
			}
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
	}
	closeErr := root.Close()
	if err != nil {
		return nil, fmt.Errorf("doltsrc: list databases: %w", err)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("doltsrc: close root connection: %w", closeErr)
	}
	if !found {
		return nil, fmt.Errorf("doltsrc: workspace %s has no database %q", workspace, database)
	}

	var lastErr error
	for attempt := range 8 {
		h, err := openDSN(fmt.Sprintf("file://%s?commitname=Choir&commitemail=system@choir.local&database=%s&multistatements=true&clientfoundrows=true", workspace, database))
		if err == nil {
			if err = h.DB.PingContext(ctx); err == nil {
				return h, nil
			}
			_ = h.Close()
		}
		lastErr = err
		if !strings.Contains(strings.ToLower(err.Error()), "non 0 lock") {
			break
		}
		time.Sleep(time.Duration(attempt+1) * 25 * time.Millisecond)
	}
	return nil, lastErr
}

// Create makes a workspace directory and an empty database in it and opens
// it, for fixtures (tests and the guest dry-run tooling). Production code
// never creates a legacy workspace.
func Create(ctx context.Context, workspace, database string) (*Handle, error) {
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		return nil, err
	}
	root, err := openDSN(fmt.Sprintf("file://%s?commitname=Choir&commitemail=system@choir.local&multistatements=true", workspace))
	if err != nil {
		return nil, err
	}
	if _, err := root.DB.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS "+database); err != nil {
		_ = root.Close()
		return nil, fmt.Errorf("doltsrc: create database: %w", err)
	}
	if err := root.Close(); err != nil {
		return nil, err
	}
	return Open(ctx, workspace, database)
}

func openDSN(dsn string) (*Handle, error) {
	cfg, err := embedded.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("doltsrc: parse dsn: %w", err)
	}
	cfg.BackOff = newBackOff()
	connector, err := embedded.NewConnector(cfg)
	if err != nil {
		return nil, fmt.Errorf("doltsrc: new connector: %w", err)
	}
	db := sql.OpenDB(connector)
	// One connection: the embedded engine is not safe for concurrent use.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	return &Handle{DB: db, connector: connector}, nil
}
