// Command projection-compact compacts a closed projection store workspace in
// place: it squashes the single branch to one commit on its root, runs a full
// Dolt GC, and refuses unless the content witness is unchanged. Run it only on
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
	"syscall"

	"github.com/yusefmosiah/go-choir/internal/projectionbase"
)

func main() {
	workspace := flag.String("workspace", "", "projection store workspace directory (for example <data>/state.texture)")
	computerID := flag.String("computer-id", "", "computer the store projects")
	canonicalHead := flag.String("canonical-head", "", "canonical event head the store is at (recorded in the squash commit)")
	flag.Parse()
	if *workspace == "" || *computerID == "" || *canonicalHead == "" {
		flag.Usage()
		os.Exit(2)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	result, err := projectionbase.CompactWorkspace(ctx, *computerID, *canonicalHead, *workspace)
	report := map[string]any{
		"workspace":           *workspace,
		"squashed":            result.Squashed,
		"commits_before":      result.CommitsBefore,
		"commits_after":       result.CommitsAfter,
		"bytes_before":        result.NomsBytesBefore,
		"bytes_after":         result.NomsBytesAfter,
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
