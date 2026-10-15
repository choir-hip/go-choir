package main

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// pruneSupersededScratch removes a computer's scratch directories from
// generations before generation and returns their names. A generation's
// directory is kept while its job may still retry (the rebuilder resumes
// from it), but once a later generation is claimed nothing reads it again
// (docs/problems/checkpointd-scratch-leaks-superseded-generations-2026-10-15.md).
// Only directories named exactly <computerID>-<generation> are touched;
// cleanup is best effort and never fails the job.
func pruneSupersededScratch(scratchRoot, computerID string, generation uint64) []string {
	entries, err := os.ReadDir(scratchRoot)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("checkpoint scratch prune computer=%s: %v", computerID, err)
		}
		return nil
	}
	prefix := computerID + "-"
	var removed []string
	for _, e := range entries {
		rest, ok := strings.CutPrefix(e.Name(), prefix)
		if !ok || !e.IsDir() || rest == "" || strings.TrimLeft(rest, "0123456789") != "" {
			continue
		}
		g, err := strconv.ParseUint(rest, 10, 64)
		if err != nil || g >= generation {
			continue
		}
		if err := os.RemoveAll(filepath.Join(scratchRoot, e.Name())); err != nil {
			log.Printf("checkpoint scratch prune computer=%s generation=%d: %v", computerID, g, err)
			continue
		}
		removed = append(removed, e.Name())
	}
	return removed
}
