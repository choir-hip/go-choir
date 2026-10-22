package platform

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/doltbatch"
)

// Store A's snapshot switch (PLATFORM_DOLT_SNAPSHOTS; docs/problems/
// platform-dolt-oldgen-rate-and-squash-window-2026-10-22.md). Failure modes
// pinned: F1 Store A still commits with the switch off; F4 the switch leaks
// into Store B's committer when the pools are separate; F5 a recovery
// boundary write fails because nothing was committed.

func storeALogCount(t *testing.T, s *Store) int {
	t.Helper()
	var n int
	if err := s.db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM dolt_log").Scan(&n); err != nil {
		t.Fatalf("count dolt_log: %v", err)
	}
	return n
}

func TestStoreASnapshotsOffNeverCommits(t *testing.T) {
	s, _ := openTestPlatformStore(t)
	s.useStoreASnapshotMode(doltbatch.ModeOff)
	base := storeALogCount(t, s)

	ctx := context.Background()
	if err := s.RecordReplayWatermark(ctx, "computer-snapshot-off", 1, "sha256:aa"); err != nil {
		t.Fatalf("record replay watermark with snapshots off: %v", err) // F5
	}
	s.markDirty("debounced write")
	if err := s.committer.Flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if got := storeALogCount(t, s); got != base {
		t.Fatalf("dolt_log = %d, want %d: Store A must not DOLT_COMMIT with snapshots off", got, base) // F1
	}
	if got := platformTableCount(t, s, "computer_replay_watermarks"); got != 1 {
		t.Fatalf("watermark rows = %d, want 1", got)
	}
}

func TestStoreASnapshotsCommitByDefault(t *testing.T) {
	s, _ := openTestPlatformStore(t)
	s.useStoreASnapshotMode(doltbatch.ModeCommit)
	base := storeALogCount(t, s)
	if err := s.RecordReplayWatermark(context.Background(), "computer-snapshot-on", 1, "sha256:aa"); err != nil {
		t.Fatalf("record replay watermark: %v", err)
	}
	if got := storeALogCount(t, s); got != base+1 {
		t.Fatalf("dolt_log = %d, want %d: the boundary commit is the default", got, base+1)
	}
}

func TestStoreASnapshotModeLeavesStoreBCommitter(t *testing.T) {
	s, _ := openTestPlatformStore(t)
	// Separate pools: Store B has its own committer (openStore with a corpus DSN).
	corpus := doltbatch.New(s.db, "platform-corpus", 0)
	s.corpusCommitter = corpus
	s.useStoreASnapshotMode(doltbatch.ModeOff)
	if s.corpusCommitter != corpus || s.corpusCommitter.Mode() != doltbatch.ModeCommit {
		t.Fatalf("Store B committer changed by Store A's switch") // F4
	}
	if s.committer.Mode() != doltbatch.ModeOff {
		t.Fatalf("Store A committer mode = %q, want off", s.committer.Mode())
	}

	// Shared pool (no corpus DSN): one database, one committer, so both follow.
	shared, _ := openTestPlatformStore(t)
	shared.useStoreASnapshotMode(doltbatch.ModeOff)
	if shared.corpusCommitter != shared.committer {
		t.Fatalf("shared pool: corpus committer must stay the Store A committer")
	}
}
