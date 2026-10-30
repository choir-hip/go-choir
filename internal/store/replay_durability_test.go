package store

import (
	"context"
	"testing"
)

// Failure modes of the relaxed replay durability: the write connection stays
// at synchronous=NORMAL after the replay, so a live acknowledged write is not
// durable; the restore leaves replayed commits unsynced; a commit during the
// replay is not reported as relaxed to CommitReplay.
func TestReplayDurabilityRelaxesThenRestoresFullSync(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	mode := func() int {
		t.Helper()
		var m int
		if err := s.db.QueryRowContext(ctx, `PRAGMA synchronous`).Scan(&m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	if got := mode(); got != 2 {
		t.Fatalf("synchronous at open = %d, want 2 (FULL)", got)
	}
	restore, err := s.RelaxReplayDurability(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := mode(); got != 1 || !s.replayRelaxed.Load() {
		t.Fatalf("synchronous during replay = %d relaxed=%v, want 1 (NORMAL)", got, s.replayRelaxed.Load())
	}
	if err := s.CommitReplay(ctx); err != nil {
		t.Fatalf("a relaxed CommitReplay checkpoints the WAL: %v", err)
	}
	if err := restore(ctx); err != nil {
		t.Fatal(err)
	}
	if got := mode(); got != 2 || s.replayRelaxed.Load() {
		t.Fatalf("synchronous after replay = %d relaxed=%v, want 2 (FULL)", got, s.replayRelaxed.Load())
	}
}
