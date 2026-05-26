package searchplane

import (
	"path/filepath"
	"testing"
	"time"
)

func TestFileHealthStore_PersistsOutcome(t *testing.T) {
	path := filepath.Join(t.TempDir(), "health.db")
	store, err := OpenFileHealthStore(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := store.RecordOutcome(Outcome{Provider: "brave", Class: OutcomeQuotaLimited, Error: "quota", At: time.Now()}); err != nil {
		t.Fatalf("record: %v", err)
	}
	reopened, err := OpenFileHealthStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	rec, err := reopened.Get("brave")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if rec.StrikeCount != 1 {
		t.Fatalf("strike_count = %d, want 1", rec.StrikeCount)
	}
	if rec.State != StateCoolingDown {
		t.Fatalf("state = %q, want cooling_down", rec.State)
	}
}
