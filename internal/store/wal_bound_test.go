package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/dbx"
)

// Failure modes of the WAL bound, written before the code
// (docs/design/append-clustered-store-2026-10-31.md part 1):
//
//	W1 the WAL a long read pinned stays at its high-water mark on disk after
//	   the read ends (the paper's 584 MB state.db-wal);
//	W2 the bound blocks a writer while the long read is open;
//	W3 the post-read checkpoint leaves frames it could have backfilled;
//	W4 a reopen (a new write connection) comes back without the limit.
//
// walPinnedGrowth writes past the limit while a read transaction holds the
// WAL, ends the read, and returns the WAL size after one more commit.
func walPinnedGrowth(t *testing.T, s *Store) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS wal_bound_probe (b BLOB)`); err != nil {
		t.Fatal(err)
	}
	tx, err := s.ReadDB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM wal_bound_probe`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	// W2: 80 commits of 1 MiB with the reader open, each within a deadline.
	blob := []byte(strings.Repeat("x", 1<<20))
	for i := 0; i < 80; i++ {
		wctx, wcancel := context.WithTimeout(ctx, 10*time.Second)
		_, err := s.db.ExecContext(wctx, `INSERT INTO wal_bound_probe (b) VALUES (?)`, blob)
		wcancel()
		if err != nil {
			t.Fatalf("W2: write %d under the long read: %v", i, err)
		}
	}
	if pinned := s.walFileBytes(); pinned < 80<<20 {
		t.Fatalf("the reader did not pin the WAL: %d bytes", pinned)
	}
	_ = tx.Rollback()
	s.CheckpointAfterLongRead(ctx, "test")
	if _, err := s.db.ExecContext(ctx, `DELETE FROM wal_bound_probe`); err != nil {
		t.Fatal(err)
	}
	return s.walFileBytes()
}

func journalSizeLimit(t *testing.T, s *Store) int64 {
	t.Helper()
	var limit int64
	if err := s.db.QueryRow(`PRAGMA journal_size_limit`).Scan(&limit); err != nil {
		t.Fatal(err)
	}
	return limit
}

func TestWALBoundShrinksThePinnedWAL(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if got := journalSizeLimit(t, s); got != dbx.SQLiteJournalSizeLimit {
		t.Fatalf("writer journal_size_limit = %d, want %d", got, dbx.SQLiteJournalSizeLimit)
	}

	// The control: SQLite's default (no limit) keeps the high-water mark, so
	// the probe has teeth.
	if _, err := s.db.Exec(`PRAGMA journal_size_limit=-1`); err != nil {
		t.Fatal(err)
	}
	if after := walPinnedGrowth(t, s); after < 80<<20 {
		t.Fatalf("control: unbounded WAL shrank to %d bytes", after)
	}

	// W4: a reopen opens a new write connection, which carries the limit.
	path := s.Path()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Reopen(path); err != nil {
		t.Fatal(err)
	}
	if got := journalSizeLimit(t, s); got != dbx.SQLiteJournalSizeLimit {
		t.Fatalf("W4: journal_size_limit after reopen = %d", got)
	}

	// W1: with the limit the first commit after the restart truncates it.
	if after := walPinnedGrowth(t, s); after > dbx.SQLiteJournalSizeLimit {
		t.Fatalf("W1: bounded WAL is %d bytes after the read ended, limit %d", after, dbx.SQLiteJournalSizeLimit)
	}
	st := s.WALStats()
	if st.Checkpoints != 2 || st.LastCheckpoint == nil || st.LastCheckpoint.Error != "" || st.LastCheckpoint.Busy != 0 {
		t.Fatalf("stats after two checkpoints: %+v", st)
	}
	// W3: nothing pinned the WAL any more, so every frame was backfilled.
	if st.LastCheckpoint.Checkpointed != st.LastCheckpoint.LogFrames || st.LastCheckpoint.LogFrames < 20000 {
		t.Fatalf("W3: the post-read checkpoint left frames: %+v", *st.LastCheckpoint)
	}
}
