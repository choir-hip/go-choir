package agentcore

import (
	"os"
	"path/filepath"
	"testing"

	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// The apply checkpoint and restore replay into a staged store; a live store
// that never upcast its deposits must be compared with (and replaced by) a
// replay that keeps the recorded bytes, or recorded tool-call events re-key
// (problems/selfdev-apply-checkpoint-starved-by-resumed-work-2026-10-10.md,
// rerun 9). Failure modes pinned: the staged store left in upcast mode for a
// live store without a ledger; an unreadable live ledger treated as "never
// upcast" instead of failing closed.
func TestMatchLiveDepositModeRetainsWhenLiveNeverUpcast(t *testing.T) {
	live, err := choirstore.Open(filepath.Join(t.TempDir(), "live.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer live.Close()
	staged, err := choirstore.OpenFresh(filepath.Join(t.TempDir(), "staged.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer staged.Close()
	mode, err := matchLiveDepositMode(live, staged)
	if err != nil || mode != replayDepositModeRetained || !staged.RetainsReplayDeposits() {
		t.Fatalf("mode = %q err = %v retains = %v; want retained", mode, err, staged.RetainsReplayDeposits())
	}

	if err := os.WriteFile(filepath.Join(live.TexturePath(), "vocab-deposit-upcast.jsonl"), []byte("not a ledger\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	other, err := choirstore.OpenFresh(filepath.Join(t.TempDir(), "other.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if mode, err := matchLiveDepositMode(live, other); err == nil {
		t.Fatalf("unreadable live ledger accepted as mode %q", mode)
	}
}
