// Command projection-compact compacts a closed projection store workspace in
// place: it squashes the single branch to one commit on its root, runs a full
// Dolt GC, and refuses unless the store's root value hash is unchanged. Run it only on
// a stopped computer's store after a backup (docs/runbooks/offline-guest-gc.md)
// or on a copy. It needs no keys and appends nothing to the tape.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/dolthub/dolt/go/store/util/tempfiles"

	"github.com/yusefmosiah/go-choir/internal/projectionbase"
)

func main() {
	workspace := flag.String("workspace", "", "projection store workspace directory (for example <data>/state.texture)")
	computerID := flag.String("computer-id", "", "computer the store projects")
	canonicalHead := flag.String("canonical-head", "", "canonical event head the store is at (recorded in the squash commit)")
	fullWitness := flag.Bool("full-witness", false, "also compare the full content witness before and after (reads and sorts every row; slow and memory-heavy on a large store)")
	memoryLimit := flag.Int64("memory-limit-mib", 5120, "Go soft memory limit in MiB when GOMEMLIMIT is unset; run under a MemoryMax without swap so an overrun fails instead of thrashing")
	flag.Parse()
	if *workspace == "" || *computerID == "" || *canonicalHead == "" {
		flag.Usage()
		os.Exit(2)
	}
	if os.Getenv("GOMEMLIMIT") == "" && *memoryLimit > 0 {
		debug.SetMemoryLimit(*memoryLimit << 20)
	}
	// Stage Dolt's GC output on the store's own filesystem, not the host's
	// os.TempDir() (the killed 2026-10-19 run left 4.2 GB in Node B's /tmp).
	tempDir := projectionbase.TempDir(*workspace)
	if err := os.MkdirAll(tempDir, 0o700); err != nil {
		log.Fatalf("projection-compact: temp dir: %v", err)
	}
	tempfiles.MovableTempFileProvider = tempfiles.NewTempFileProviderAt(tempDir)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	result, err := projectionbase.CompactWorkspaceWith(ctx, *computerID, *canonicalHead, *workspace, projectionbase.CompactOptions{FullWitness: *fullWitness})
	report := map[string]any{
		"workspace":           *workspace,
		"squashed":            result.Squashed,
		"commits_before":      result.CommitsBefore,
		"commits_after":       result.CommitsAfter,
		"bytes_before":        result.NomsBytesBefore,
		"bytes_after":         result.NomsBytesAfter,
		"root_before":         result.RootBefore,
		"root_after":          result.RootAfter,
		"content_root_before": result.WitnessBefore.ContentRoot,
		"content_root_after":  result.WitnessAfter.ContentRoot,
	}
	if err != nil {
		report["error"] = err.Error()
	}
	_ = json.NewEncoder(os.Stdout).Encode(report)
	if err != nil {
		log.Printf("projection-compact: %v", err)
		os.Exit(1)
	}
}
