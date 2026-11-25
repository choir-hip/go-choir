package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/purgeguard"
)

// A restore resurrects an account deleted since the backup. The ledger, kept
// outside auth.db, names it by hash; the check finds it in the restored file
// and, with -reapply, writes it a deletion record that is already due.
func TestARestoredAccountWhoseDeletionWasFinishedIsFoundAndReapplied(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "auth.db")
	store, err := auth.OpenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	back, err := store.CreateUser("back-from-the-dead", "back@example.com")
	if err != nil {
		t.Fatal(err)
	}
	stay, err := store.CreateUser("never-deleted", "stay@example.com")
	if err != nil {
		t.Fatal(err)
	}
	_ = store.Close()
	ledger := filepath.Join(dir, "ledger.jsonl")
	line, _ := json.Marshal(map[string]string{"user_hash": auth.DeletionUserHash(back.ID), "job_id": "del-1", "purged_at": "2026-11-20T00:00:00Z"})
	if err := os.WriteFile(ledger, append(line, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if code := runDeletionRestoreCheck([]string{"-db", dbPath, "-ledger", ledger}, &out); code != 1 || !strings.Contains(out.String(), back.ID) || strings.Contains(out.String(), "back@example.com") {
		t.Fatalf("check: %d %q", code, out.String())
	}
	if strings.Contains(out.String(), stay.ID) {
		t.Fatalf("an account never deleted was reported: %s", out.String())
	}
	out.Reset()
	if code := runDeletionRestoreCheck([]string{"-db", dbPath, "-ledger", ledger, "-reapply"}, &out); code != 0 {
		t.Fatalf("reapply: %d %q", code, out.String())
	}
	store, err = auth.OpenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	d, err := store.AccountDeletionFor(back.ID)
	if err != nil || d == nil || d.State != auth.DeletionPending {
		t.Fatalf("no record written: %+v %v", d, err)
	}
	if err := store.PurgeEligible(back.ID, time.Now().UTC()); err != nil {
		t.Fatalf("the reapplied record is not due: %v", err)
	}
	if !store.AccountAccessRevoked(back.ID) {
		t.Fatal("the restored account can still sign in")
	}
	if d2, _ := store.AccountDeletionFor(stay.ID); d2 != nil {
		t.Fatal("another account got a record")
	}
	_ = purgeguard.GracePeriod
}

func TestTheRestoreCheckFailsOnAMissingLedgerNotQuietly(t *testing.T) {
	var out bytes.Buffer
	dir := t.TempDir()
	if code := runDeletionRestoreCheck([]string{"-db", filepath.Join(dir, "auth.db"), "-ledger", filepath.Join(dir, "none.jsonl")}, &out); code != 2 {
		t.Fatalf("a missing ledger exited %d, not 2: %s", code, out.String())
	}
}
