// Command choir-migrate-host copies the host's two Dolt stores into the
// PostgreSQL databases control and graph (data system cutover, C4 rehearsal
// and C5). Store A (platform) becomes control; Store B (corpus) becomes graph.
//
// It only reads the Dolt servers (read-only REPEATABLE READ transactions, one
// per table), truncates each target table before copying, verifies every table
// by row count and digest, writes a JSON report, and prints a summary. Exit
// status 0 means every table verified; 1 a copy or verification failed; 2 bad
// usage.
//
// Credentials: the Dolt side uses CHOIR_DOLT_USER and the Dolt password file
// (internal/dbcred), the PostgreSQL side CHOIR_PG_PASSWORD_FILE or
// $CREDENTIALS_DIRECTORY/choir-pg-password. Put no passwords in the DSNs.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/migrate/hostcopy"
	"github.com/yusefmosiah/go-choir/internal/migrate/rowcopy"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir-migrate-host", flag.ContinueOnError)
	fs.SetOutput(stderr)
	srcControl := fs.String("source-control-dsn", env("CHOIR_MIGRATE_SOURCE_CONTROL_DSN"), "Store A MySQL DSN, e.g. choir@tcp(127.0.0.1:13306)/platform")
	srcGraph := fs.String("source-graph-dsn", env("CHOIR_MIGRATE_SOURCE_GRAPH_DSN"), "Store B MySQL DSN, e.g. choir@tcp(127.0.0.1:13307)/corpus")
	dstControl := fs.String("target-control-dsn", env("CHOIR_MIGRATE_TARGET_CONTROL_DSN"), "PostgreSQL DSN of the control database")
	dstGraph := fs.String("target-graph-dsn", env("CHOIR_MIGRATE_TARGET_GRAPH_DSN"), "PostgreSQL DSN of the graph database")
	reportPath := fs.String("report", "", "write the JSON report to this path")
	tables := fs.String("tables", "", "comma-separated table filter (default: every table in both source and target)")
	dryRun := fs.Bool("dry-run", false, "count source rows and intersect columns; write nothing")
	parallel := fs.Int("parallel", 2, "tables copied at once per database")
	bootstrap := fs.Bool("bootstrap", true, "create the PostgreSQL schemas first through the platform store (skip with --bootstrap=false when they exist)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *dstControl == "" || *dstGraph == "" || *srcControl == "" || *srcGraph == "" {
		fmt.Fprintln(stderr, "choir-migrate-host: all four DSNs are required")
		fs.Usage()
		return 2
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	started := time.Now()
	logf := func(format string, a ...any) {
		fmt.Fprintf(stderr, "%s "+format+"\n", append([]any{time.Since(started).Round(time.Second)}, a...)...)
	}
	if *bootstrap && !*dryRun {
		logf("bootstrapping the PostgreSQL schemas")
		if err := bootstrapSchemas(ctx, *dstControl, *dstGraph); err != nil {
			fmt.Fprintf(stderr, "choir-migrate-host: bootstrap: %v\n", err)
			return 1
		}
	}
	var filter []string
	for _, t := range strings.Split(*tables, ",") {
		if t = strings.TrimSpace(t); t != "" {
			filter = append(filter, t)
		}
	}
	rep, err := hostcopy.Run(ctx, hostcopy.Config{
		SourceControlDSN: *srcControl,
		SourceGraphDSN:   *srcGraph,
		TargetControlDSN: *dstControl,
		TargetGraphDSN:   *dstGraph,
		Tables:           filter,
		DryRun:           *dryRun,
		Parallel:         *parallel,
		Logf:             logf,
	})
	if *reportPath != "" && rep != nil {
		if werr := writeJSON(*reportPath, rep); werr != nil {
			fmt.Fprintf(stderr, "choir-migrate-host: write report: %v\n", werr)
			if err == nil {
				err = werr
			}
		}
	}
	if rep != nil {
		summarize(stdout, "control", rep.Control)
		summarize(stdout, "graph", rep.Graph)
	}
	if err != nil {
		fmt.Fprintf(stdout, "FAILED: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "OK in %s\n", time.Since(started).Round(time.Second))
	return 0
}

func env(k string) string { return strings.TrimSpace(os.Getenv(k)) }

func summarize(w io.Writer, name string, r *rowcopy.Report) {
	if r == nil {
		return
	}
	fmt.Fprintf(w, "== %s (%s): %d tables, %d rows, ok=%v\n", name, r.Target, len(r.Tables), r.TotalRows, r.OK)
	for _, t := range r.Tables {
		status := "ok"
		if !t.OK {
			status = "FAIL: " + t.Error
		}
		fmt.Fprintf(w, "  %-44s src=%-9d dst=%-9d %6dms  %s\n", t.Name, t.SourceRows, t.TargetRows, t.DurationMS, status)
	}
	if len(r.SourceOnlyTables) > 0 {
		fmt.Fprintf(w, "  dropped (source only): %s\n", strings.Join(r.SourceOnlyTables, ", "))
	}
	if len(r.TargetOnlyTables) > 0 {
		fmt.Fprintf(w, "  new (target only): %s\n", strings.Join(r.TargetOnlyTables, ", "))
	}
	for _, t := range r.Tables {
		if len(t.SourceOnlyColumns) > 0 {
			fmt.Fprintf(w, "  %s dropped columns: %s\n", t.Name, strings.Join(t.SourceOnlyColumns, ", "))
		}
		if t.BadTextRows > 0 {
			fmt.Fprintf(w, "  %s: %d rows hold text PostgreSQL cannot store: %v; samples: %s\n", t.Name, t.BadTextRows, t.BadText, strings.Join(t.BadTextSamples, "; "))
		}
		if len(t.NullCoerced) > 0 {
			fmt.Fprintf(w, "  %s NULL coerced to zero value: %v\n", t.Name, t.NullCoerced)
		}
	}
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}
