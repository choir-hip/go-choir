// Command store-split-migrate runs the header/body split migration on a
// stopped store file, the same code a guest runs at boot
// (internal/store/header_body_split.go). It is for proving the migration on a
// copy before a ring is moved (S3 of
// docs/definitions/choir-store-header-body-split-2026-11-02.md): it prints
// progress and the elapsed time, and leaves state.db.presplit beside the
// migrated file for cmd/store-split-diff.
//
//	store-split-migrate /path/state.db
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: store-split-migrate /path/state.db")
		os.Exit(2)
	}
	start := time.Now()
	last := start
	err := store.MigrateHeaderBodySplit(os.Args[1], func(n uint64) {
		if time.Since(last) >= 10*time.Second {
			last = time.Now()
			fmt.Fprintf(os.Stderr, "store-split-migrate: progress %d after %s\n", n, time.Since(start).Round(time.Second))
		}
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "store-split-migrate: %v (after %s)\n", err, time.Since(start).Round(time.Second))
		os.Exit(1)
	}
	fmt.Printf("store-split-migrate: done in %s\n", time.Since(start).Round(time.Second))
}
