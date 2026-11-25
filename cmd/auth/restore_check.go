package main

import (
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/yusefmosiah/go-choir/internal/auth"
)

// runDeletionRestoreCheck is `auth deletion-restore-check`: after a restore of
// auth.db from backup, find the accounts whose deletion was finished since the
// backup and, with -reapply, write each its deletion record again so the
// running auth service's purge runner deletes it from every service. Run it on
// the restored auth.db BEFORE the restored host serves anyone. It prints
// opaque ids and job ids only. Exit 0: nothing to reapply (or reapplied);
// 1: accounts found and not reapplied; 2: a usage or read error.
func runDeletionRestoreCheck(args []string, out io.Writer) int {
	fs := flag.NewFlagSet("deletion-restore-check", flag.ContinueOnError)
	fs.SetOutput(out)
	dbPath := fs.String("db", "", "the restored auth.db")
	ledger := fs.String("ledger", "", "account-deletion-ledger.jsonl (kept outside the backup)")
	reapply := fs.Bool("reapply", false, "write each found account a deletion record that is already due")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *dbPath == "" || *ledger == "" {
		fmt.Fprintln(out, "usage: auth deletion-restore-check -db <restored auth.db> -ledger <ledger.jsonl> [-reapply]")
		return 2
	}
	entries, err := auth.ReadDeletionLedger(*ledger)
	if err != nil {
		fmt.Fprintln(out, err)
		return 2
	}
	store, err := auth.OpenStore(*dbPath)
	if err != nil {
		fmt.Fprintln(out, err)
		return 2
	}
	defer func() { _ = store.Close() }()
	found, err := store.RestoredDeletions(entries, *reapply, time.Now().UTC())
	if err != nil {
		fmt.Fprintln(out, err)
		return 2
	}
	for _, f := range found {
		fmt.Fprintf(out, "restored account whose deletion was finished: user=%s job=%s reapplied=%v\n", f.UserID, f.JobID, f.Reapplied)
	}
	fmt.Fprintf(out, "ledger lines: %d; restored accounts to delete again: %d\n", len(entries), len(found))
	if len(found) > 0 && !*reapply {
		return 1
	}
	return 0
}
