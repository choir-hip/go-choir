// Package hostcopy is the one-shot host migrator of the data system cutover
// (docs/design/data-system-build-sheet-2026-10-30.md, decision 12): it reads
// Store A (the platform Dolt database) and Store B (the corpus Dolt database)
// over the MySQL protocol and writes the PostgreSQL databases control and
// graph, whose schemas already exist (the platform store bootstraps them).
//
// Reads never write the source: each table is read inside one read-only
// REPEATABLE READ transaction, so its count, its rows and its source digest
// agree even while writers continue on a live server. Each target table is
// truncated first, so a rerun starts clean. With internal/migrate/doltsrc,
// this is one of the only packages that may import the MySQL driver after the
// build; it is deleted after C5.
package hostcopy

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/yusefmosiah/go-choir/internal/dbcred"
	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/migrate/rowcopy"
)

// Config names the four databases. Source DSNs are MySQL-protocol DSNs
// (user@tcp(host:port)/database); credentials come from dbcred
// (CHOIR_DOLT_USER and the Dolt password file). Target DSNs are PostgreSQL
// URLs or key=value strings; the password comes from dbcred
// (CHOIR_PG_PASSWORD_FILE or $CREDENTIALS_DIRECTORY/choir-pg-password).
type Config struct {
	SourceControlDSN string
	SourceGraphDSN   string
	TargetControlDSN string
	TargetGraphDSN   string
	// Tables restricts the copy to the named tables (in either database).
	Tables []string
	// DryRun counts source rows and intersects columns; nothing is written.
	DryRun bool
	// Parallel is how many tables copy at once per database (default 2).
	Parallel int
	// Logf receives progress lines (default: discarded).
	Logf func(format string, args ...any)
}

// Report is the outcome for both databases.
type Report struct {
	DryRun     bool            `json:"dry_run,omitempty"`
	StartedAt  time.Time       `json:"started_at"`
	FinishedAt time.Time       `json:"finished_at"`
	Control    *rowcopy.Report `json:"control,omitempty"`
	Graph      *rowcopy.Report `json:"graph,omitempty"`
	OK         bool            `json:"ok"`
	Error      string          `json:"error,omitempty"`
}

// Run copies Store A into control and Store B into graph. A database that
// fails does not stop the other; the returned error joins the failures.
func Run(ctx context.Context, cfg Config) (*Report, error) {
	if cfg.Parallel < 1 {
		cfg.Parallel = 2
	}
	if cfg.Logf == nil {
		cfg.Logf = func(string, ...any) {}
	}
	rep := &Report{DryRun: cfg.DryRun, StartedAt: time.Now().UTC()}
	var errs []error
	for _, leg := range []struct {
		name     string
		srcDSN   string
		dstDSN   string
		reportTo **rowcopy.Report
	}{
		{"control", cfg.SourceControlDSN, cfg.TargetControlDSN, &rep.Control},
		{"graph", cfg.SourceGraphDSN, cfg.TargetGraphDSN, &rep.Graph},
	} {
		if strings.TrimSpace(leg.srcDSN) == "" && strings.TrimSpace(leg.dstDSN) == "" {
			continue
		}
		cfg.Logf("hostcopy: %s: starting", leg.name)
		r, err := copyOne(ctx, cfg, leg.name, leg.srcDSN, leg.dstDSN)
		*leg.reportTo = r
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", leg.name, err))
		}
	}
	rep.FinishedAt = time.Now().UTC()
	if len(cfg.Tables) > 0 && len(errs) == 0 {
		copied := map[string]bool{}
		for _, r := range []*rowcopy.Report{rep.Control, rep.Graph} {
			if r != nil {
				for _, t := range r.Tables {
					copied[t.Name] = true
				}
			}
		}
		for _, t := range cfg.Tables {
			if !copied[strings.TrimSpace(t)] {
				errs = append(errs, fmt.Errorf("requested table %q is in neither database pair", t))
			}
		}
	}
	err := errors.Join(errs...)
	if err != nil {
		rep.Error = err.Error()
		return rep, err
	}
	rep.OK = true
	return rep, nil
}

func copyOne(ctx context.Context, cfg Config, name, srcDSN, dstDSN string) (*rowcopy.Report, error) {
	if strings.TrimSpace(srcDSN) == "" || strings.TrimSpace(dstDSN) == "" {
		return nil, errors.New("both a source and a target DSN are required")
	}
	srcDSN, err := dbcred.ApplyDSN(srcDSN)
	if err != nil {
		return nil, err
	}
	dstDSN, err = dbcred.ApplyPGDSN(dstDSN)
	if err != nil {
		return nil, err
	}
	src, err := sql.Open("mysql", srcDSN)
	if err != nil {
		return nil, fmt.Errorf("open source: %w", redact(err))
	}
	defer src.Close()
	// One connection per table in flight, plus introspection.
	src.SetMaxOpenConns(cfg.Parallel + 1)
	src.SetMaxIdleConns(cfg.Parallel + 1)
	src.SetConnMaxLifetime(30 * time.Minute)
	if err := src.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping source: %w", redact(err))
	}
	dst, err := dbx.OpenPostgres(dstDSN, dbx.PGOptions{MaxOpenConns: 2*cfg.Parallel + 2})
	if err != nil {
		return nil, fmt.Errorf("open target: %w", err)
	}
	defer dst.Close()
	logf := func(format string, args ...any) { cfg.Logf("hostcopy: "+name+": "+format, args...) }
	return rowcopy.Run(ctx, src, dst, dbx.Postgres, rowcopy.Options{
		Tables:   cfg.Tables,
		DryRun:   cfg.DryRun,
		Parallel: cfg.Parallel,
		// Short keyset pages: one long scan through the Dolt sql-server
		// stalled (rowcopy.Options.PageRows).
		PageRows: 50000,
		Logf:     logf,
		// A --tables filter spans both databases; Run checks the union.
		LenientFilter: len(cfg.Tables) > 0,
	})
}

// redact keeps a DSN (which may carry a password) out of an error.
func redact(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if i := strings.Index(msg, "@"); i >= 0 {
		return errors.New("connection error (details redacted)")
	}
	return err
}
