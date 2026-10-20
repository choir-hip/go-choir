// Command platform-dolt-squash squashes a closed Dolt store's history to one
// commit and runs a bounded full GC, refusing unless the store's root value
// hash is unchanged. It is Store A's weekly squash (owner notes 2026-10-14
// section 23 and 27 item 7; docs/problems/store-a-weekly-squash-2026-10-20.md)
// on the method of projection-compact, sharing projectionbase.SquashAndGC.
//
// Run it only against a store no dolt sql-server has open, after a backup:
// scripts/platform-dolt-squash.sh does the stop, the reflink backup, the
// restore on failure and the restart. It needs no keys, credentials or
// network and appends nothing to the tape.
//
// Exit status: 0 squashed (or nothing to squash) with an identical root;
// 3 refused before changing anything (the store is as it was); 1 any other
// failure (the store may have been changed: restore the backup).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/projectionbase"
)

func main() {
	dir := flag.String("dir", "", "directory holding the database directory (for example /var/lib/go-choir/platform-dolt)")
	database := flag.String("database", "platform", "database directory name inside -dir")
	note := flag.String("note", "", "text recorded in the squash commit message")
	memoryLimit := flag.Int64("memory-limit-mib", 5120, "Go soft memory limit in MiB when GOMEMLIMIT is unset; run under a MemoryMax without swap so an overrun fails instead of thrashing")
	flag.Parse()
	if *dir == "" || *database == "" {
		flag.Usage()
		os.Exit(2)
	}
	if os.Getenv("GOMEMLIMIT") == "" && *memoryLimit > 0 {
		debug.SetMemoryLimit(*memoryLimit << 20)
	}
	// Stage Dolt's GC output on the store's own filesystem, not the host's
	// os.TempDir() (the killed 2026-10-19 run left 4.2 GB in Node B's /tmp).
	cleanupTemp, err := projectionbase.UseStoreTempDir(*dir, *database)
	if err != nil {
		log.Fatalf("platform-dolt-squash: %v", err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	started := time.Now()
	result, err := projectionbase.SquashAndGC(ctx, projectionbase.SquashOptions{
		Dir:      *dir,
		Database: *database,
		Label:    "platform-dolt-squash",
		Message:  commitMessage(*note),
	})
	cleanupTemp()
	report := map[string]any{
		"dir":            *dir,
		"database":       *database,
		"squashed":       result.Squashed,
		"commits_before": result.CommitsBefore,
		"commits_after":  result.CommitsAfter,
		"root_before":    result.RootBefore,
		"root_after":     result.RootAfter,
		"seconds":        int(time.Since(started).Seconds()),
	}
	if err != nil {
		report["error"] = err.Error()
		report["refused_before_mutation"] = projectionbase.IsRefusedBeforeMutation(err)
	}
	_ = json.NewEncoder(os.Stdout).Encode(report)
	if err != nil {
		log.Printf("platform-dolt-squash: %v", err)
		if projectionbase.IsRefusedBeforeMutation(err) {
			os.Exit(3)
		}
		os.Exit(1)
	}
}

func commitMessage(note string) func(int) string {
	return func(commits int) string {
		msg := fmt.Sprintf("platform-dolt-squash: history squash of %d commits", commits)
		if note != "" {
			msg += " (" + note + ")"
		}
		return msg
	}
}
