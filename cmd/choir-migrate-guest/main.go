// Command choir-migrate-guest runs the guest Dolt-to-SQLite migrator by hand
// against a store path, for the C4 rehearsal on a copy of a computer's data
// (scripts/cutover/guest-migrate-dryrun.sh). It does what autoputer does at
// boot when the legacy workspace exists and the state file does not, and
// prints the differential report (rows and digests per table, tables and
// columns dropped or new).
//
// -store-path is the runtime store path autoputer is configured with (the
// legacy marker); the Dolt workspace and the state file derive from it.
// Exit status: 0 migrated or skipped, 1 failed, 2 bad usage.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/yusefmosiah/go-choir/internal/migrate/guestdolt"
	"github.com/yusefmosiah/go-choir/internal/store"
)

func main() {
	storePath := flag.String("store-path", "", "runtime store path (the legacy marker; workspace and state file derive from it)")
	jsonOut := flag.Bool("json", false, "print the full JSON report instead of the summary")
	flag.Parse()
	if *storePath == "" {
		flag.Usage()
		os.Exit(2)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	rep, err := guestdolt.MigrateStore(ctx, *storePath)
	fmt.Printf("workspace: %s\nstate file: %s\nreport: %s\n", rep.DoltWorkspace, rep.SQLitePath, guestdolt.ReportPath(store.StateDBPath(*storePath)))
	if *jsonOut {
		b, _ := json.MarshalIndent(rep, "", "  ")
		fmt.Println(string(b))
	} else {
		summarize(rep)
	}
	if err != nil {
		fmt.Printf("FAILED: %v\n", err)
		os.Exit(1)
	}
}

func summarize(rep guestdolt.Report) {
	if rep.Skipped {
		fmt.Printf("skipped: %s\n", rep.SkipReason)
		return
	}
	if rep.Copy == nil {
		return
	}
	fmt.Printf("%d tables, %d rows, ok=%v\n", len(rep.Copy.Tables), rep.Copy.TotalRows, rep.Copy.OK)
	for _, t := range rep.Copy.Tables {
		status := "ok"
		if !t.OK {
			status = "FAIL: " + t.Error
		}
		fmt.Printf("  %-40s src=%-9d dst=%-9d %s\n", t.Name, t.SourceRows, t.TargetRows, status)
		if len(t.SourceOnlyColumns) > 0 {
			fmt.Printf("    dropped columns: %v\n", t.SourceOnlyColumns)
		}
		if len(t.NullCoerced) > 0 {
			fmt.Printf("    NULL coerced to zero value: %v\n", t.NullCoerced)
		}
	}
	if len(rep.Copy.SourceOnlyTables) > 0 {
		fmt.Printf("dropped tables (source only): %v\n", rep.Copy.SourceOnlyTables)
	}
	if len(rep.Copy.TargetOnlyTables) > 0 {
		fmt.Printf("new tables (target only): %v\n", rep.Copy.TargetOnlyTables)
	}
}
