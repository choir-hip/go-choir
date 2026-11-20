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

// A pinned reader makes the WAL hold every commit made while it reads, and
// WALFrames counts them even when the WAL file's size does not change (the file
// is reused from its start inside the journal size limit's slack).
func TestWALFramesCountsWhatAPinnedReaderHoldsBack(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE wal_frames_probe (b BLOB)`); err != nil {
		t.Fatal(err)
	}
	tx, err := s.ReadDB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM wal_frames_probe`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	before, err := s.WALFrames(ctx)
	if err != nil {
		t.Fatal(err)
	}
	fileBefore := s.walFileBytes()
	blob := []byte(strings.Repeat("x", 64<<10))
	for i := 0; i < 20; i++ {
		if _, err := s.db.ExecContext(ctx, `INSERT INTO wal_frames_probe (b) VALUES (?)`, blob); err != nil {
			t.Fatal(err)
		}
	}
	after, err := s.WALFrames(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if after-before < 20 {
		t.Fatalf("frames %d to %d: 20 commits under a pinned reader must add at least 20 frames", before, after)
	}
	t.Logf("frames %d -> %d, WAL file %d -> %d bytes", before, after, fileBefore, s.walFileBytes())
	if _, err := (*Store)(nil).WALFrames(ctx); err == nil {
		t.Fatal("a nil store reported frames")
	}
}

// buildPinnedWAL pins the WAL with a read transaction, commits n blobs of 1 MiB
// under it and returns the function that releases the reader.
func buildPinnedWAL(t *testing.T, s *Store, table string, n int) (release func()) {
	t.Helper()
	ctx := context.Background()
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE `+table+` (b BLOB)`); err != nil {
		t.Fatal(err)
	}
	tx, err := s.ReadDB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var c int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table).Scan(&c); err != nil {
		t.Fatal(err)
	}
	blob := []byte(strings.Repeat("x", 1<<20))
	for i := 0; i < n; i++ {
		if _, err := s.db.ExecContext(ctx, `INSERT INTO `+table+` (b) VALUES (?)`, blob); err != nil {
			t.Fatal(err)
		}
	}
	return func() { _ = tx.Rollback() }
}

// Review of 41fbba5c0 (the blocking one): after a long pin, with the write
// connection's automatic checkpoint suspended, a write on the store's write
// connection and a /health-style read through it (the health surface reads
// through s.db, texture.go textureHandle) both return within a bound WHILE the
// backfill of a large WAL runs. The control without the suspension is logged,
// not asserted (its stall depends on the disk).
func TestAWriteAndAHealthReadAreNotStalledWhileTheBackfillRuns(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	resume, err := s.SuspendAutoCheckpoint(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer resume()
	release := buildPinnedWAL(t, s, "wal_stall_probe", 400)
	framesPinned, err := s.WALFrames(ctx)
	if err != nil {
		t.Fatal(err)
	}
	release()

	finished := make(chan struct{})
	go func() {
		s.CheckpointAfterLongRead(ctx, "test")
		close(finished)
	}()
	const bound = 2 * time.Second
	var during int
	var worst time.Duration
	for running := true; running; {
		select {
		case <-finished:
			running = false
		default:
		}
		t0 := time.Now()
		wctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		_, werr := s.db.ExecContext(wctx, `INSERT INTO wal_stall_probe (b) VALUES (x'00')`)
		var one int
		rerr := s.db.QueryRowContext(wctx, `SELECT 1`).Scan(&one)
		cancel()
		took := time.Since(t0)
		if werr != nil || rerr != nil {
			t.Fatalf("write %v, health-style read %v during the backfill", werr, rerr)
		}
		if running {
			during++
			if took > worst {
				worst = took
			}
		}
		time.Sleep(time.Millisecond)
	}
	if during < 3 {
		t.Fatalf("premise: only %d write+read pairs ran while the backfill of %d frames ran", during, framesPinned)
	}
	if worst > bound {
		t.Fatalf("a write plus a health-style read took %s during the backfill of %d frames (bound %s)", worst, framesPinned, bound)
	}
	t.Logf("%d write+read pairs during a backfill of %d frames, worst %s", during, framesPinned, worst)
	cp := s.WALStats().LastCheckpoint
	if cp == nil || cp.Error != "" || cp.Busy != 0 || cp.Checkpointed != cp.LogFrames || cp.LogFrames < framesPinned {
		t.Fatalf("the WAL was not backfilled: %+v", cp)
	}
}

// The control the test above is read against: with the automatic checkpoint on,
// the first commit after the pin backfills on the write connection. Logged only.
func TestControlTheFirstCommitAfterAPinBackfillsOnTheWriteConnection(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	release := buildPinnedWAL(t, s, "wal_control_probe", 400)
	release()
	t0 := time.Now()
	if _, err := s.db.ExecContext(ctx, `INSERT INTO wal_control_probe (b) VALUES (x'00')`); err != nil {
		t.Fatal(err)
	}
	t.Logf("control: the first commit after the pin took %s (the automatic checkpoint ran inside it)", time.Since(t0))
}

// SuspendAutoCheckpoint sets the write connection's automatic checkpoint to 0 and
// resume restores what it was, on every call order, idempotently.
func TestSuspendAutoCheckpointIsRestored(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	get := func() int64 {
		var v int64
		if err := s.db.QueryRowContext(ctx, `PRAGMA wal_autocheckpoint`).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	before := get()
	if before <= 0 {
		t.Fatalf("premise: default wal_autocheckpoint is %d", before)
	}
	resume, err := s.SuspendAutoCheckpoint(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := get(); got != 0 {
		t.Fatalf("suspended wal_autocheckpoint = %d, want 0", got)
	}
	resume()
	resume()
	if got := get(); got != before {
		t.Fatalf("restored wal_autocheckpoint = %d, want %d", got, before)
	}
	if _, err := (*Store)(nil).SuspendAutoCheckpoint(ctx); err == nil {
		t.Fatal("a nil store was suspended")
	}
}

// A busy checkpoint (busy=1, log=-1, checkpointed=-1) is not a frame count.
func TestABusyCheckpointIsNotACount(t *testing.T) {
	for _, tc := range []struct {
		busy, log, ck int64
		known         bool
	}{{0, 10, 10, true}, {0, 0, 0, true}, {0, 10, 4, true}, {1, -1, -1, false}, {1, 5, 2, false}, {0, -1, -1, false}} {
		if got := checkpointKnown(tc.busy, tc.log, tc.ck); got != tc.known {
			t.Fatalf("checkpointKnown(%d, %d, %d) = %v, want %v", tc.busy, tc.log, tc.ck, got, tc.known)
		}
	}
}

// The checkpoint after a long read retries until every frame is backfilled: a
// reader's mark that keeps a pass from finishing makes it wait, and it finishes
// once the reader ends.
func TestTheCheckpointAfterALongReadRetriesUntilTheWALIsBackfilled(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	release := buildPinnedWAL(t, s, "wal_retry_probe", 8)
	started := time.Now()
	finished := make(chan time.Duration, 1)
	go func() {
		s.CheckpointAfterLongRead(ctx, "test")
		finished <- time.Since(started)
	}()
	select {
	case d := <-finished:
		t.Fatalf("the checkpoint finished after %s with a reader still holding the WAL", d)
	case <-time.After(1500 * time.Millisecond):
	}
	release()
	select {
	case <-finished:
	case <-time.After(30 * time.Second):
		t.Fatal("the checkpoint did not finish after the reader ended")
	}
	cp := s.WALStats().LastCheckpoint
	if cp == nil || cp.Error != "" || cp.Checkpointed != cp.LogFrames || cp.LogFrames == 0 {
		t.Fatalf("the WAL was not backfilled: %+v", cp)
	}
}
