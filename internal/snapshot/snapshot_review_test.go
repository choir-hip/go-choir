package snapshot

// Tests from the two reviews of lane snapshot-capture-s3 (2026-11-23). Ways the
// change could still fail, written before the fixes:
//
//  R1. a panic in a callback the stream calls (a free-space check, a page hook)
//      leaks the four upload workers or leaves the read transaction open, state
//      file or partition (the WAL stays pinned);
//  R2. a page numbered past the counted page count is appended to a chunk and
//      indexes the digest list out of range, or is accepted;
//  R3. a stream that ends after the pin deadline passed succeeds anyway (the last
//      flush picks the arm that is not the deadline), so the documented rule
//      "past the deadline the attempt fails" is false;
//  R4. a partition's read transaction is not bounded by the pin deadline;
//  R5. a restore leaves the staging file's -wal and -shm beside the state file,
//      on a success or on a failed verify;
//  R6. a retry after a pin deadline, with the store changed between the two
//      attempts, reuses a chunk whose bytes changed or records the first
//      attempt's digest;
//  R7. a VACUUM or a truncating checkpoint during the pin changes the snapshot;
//  R8. a store past 1 GiB with the production page size of 4096 breaks the
//      stream at the lock-byte page.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

func goroutinesSettleTo(t *testing.T, base int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for runtime.NumGoroutine() > base {
		if time.Now().After(deadline) {
			buf := make([]byte, 1<<16)
			buf = buf[:runtime.Stack(buf, true)]
			t.Fatalf("%d goroutines, %d before the capture: a panic leaked workers\n%s", runtime.NumGoroutine(), base, buf)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func recovers(t *testing.T, fn func()) (panicked bool) {
	t.Helper()
	defer func() {
		if recover() != nil {
			panicked = true
		}
	}()
	fn()
	return false
}

// R1, the state file: a panic in the page hook leaves no workers and no open
// transaction.
func TestAPanicInTheStateStreamLeavesNoWorkersAndNoOpenTransaction(t *testing.T) {
	f := newCopyFixture(t, ChunkSize*3)
	if _, err := TakeSnapshot(context.Background(), f.cfg); err != nil { // warm the pools
		t.Fatal(err)
	}
	base := runtime.NumGoroutine()
	f.cfg.pageHook = func(pgno int64) {
		if pgno == 1500 {
			panic("test: the page hook panicked")
		}
	}
	if !recovers(t, func() { _, _ = TakeSnapshot(context.Background(), f.cfg) }) {
		t.Fatal("test premise: the hook did not panic")
	}
	goroutinesSettleTo(t, base)
	assertWALReleased(t, f)
}

// R1, a partition: a panic in the free-space callback while a partition streams
// leaves no workers and no open partition transaction.
func TestAPanicInAPartitionStreamLeavesNoWorkersAndNoOpenTransaction(t *testing.T) {
	prev := sourceDropBytes
	sourceDropBytes = 256 << 10
	defer func() { sourceDropBytes = prev }()
	f := newCopyFixture(t, 64<<10) // under 256 KiB: the state stream never calls FreeSpace
	part := partitionDB(t, t.TempDir(), "tool_result-20261001.db", 3000)
	f.cfg.Partitions = &PartitionSource{List: func(context.Context, *sql.Tx) ([]PartitionFile, error) {
		return []PartitionFile{{Path: "telemetry/tool_result-20261001.db", DB: part.Read}}, nil
	}}
	f.cfg.FreeSpaceEvery = time.Hour
	if _, err := TakeSnapshot(context.Background(), f.cfg); err != nil { // warm the pools
		t.Fatal(err)
	}
	base := runtime.NumGoroutine()
	f.cfg.FreeSpace = func() error { panic("test: the free-space check panicked") }
	if !recovers(t, func() { _, _ = TakeSnapshot(context.Background(), f.cfg) }) {
		t.Fatal("test premise: the callback did not panic in the partition stream")
	}
	goroutinesSettleTo(t, base)
	var busy, logFrames, checkpointed int
	if err := part.Write.QueryRow(`PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &logFrames, &checkpointed); err != nil {
		t.Fatal(err)
	}
	if busy != 0 {
		t.Fatalf("the partition's read transaction is still open after the panic (busy=%d)", busy)
	}
	assertWALReleased(t, f)
}

// A page-count liar: a driver wrapper that reports one page fewer than the file
// holds, the stream sqlite_dbpage yields then runs past the count.
type liarConnector struct {
	inner driver.Driver
	dsn   string
}

func (l liarConnector) Connect(context.Context) (driver.Conn, error) {
	c, err := l.inner.Open(l.dsn)
	if err != nil {
		return nil, err
	}
	return &liarConn{c}, nil
}
func (l liarConnector) Driver() driver.Driver { return l.inner }

type liarConn struct{ driver.Conn }

func (c *liarConn) QueryContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	rows, err := c.Conn.(driver.QueryerContext).QueryContext(ctx, q, args)
	if err == nil && strings.Contains(q, "page_count") {
		return &liarRows{rows}, nil
	}
	return rows, err
}
func (c *liarConn) ExecContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	return c.Conn.(driver.ExecerContext).ExecContext(ctx, q, args)
}
func (c *liarConn) PrepareContext(ctx context.Context, q string) (driver.Stmt, error) {
	return c.Conn.(driver.ConnPrepareContext).PrepareContext(ctx, q)
}
func (c *liarConn) BeginTx(ctx context.Context, o driver.TxOptions) (driver.Tx, error) {
	return c.Conn.(driver.ConnBeginTx).BeginTx(ctx, o)
}

type liarRows struct{ driver.Rows }

func (r *liarRows) Next(dest []driver.Value) error {
	err := r.Rows.Next(dest)
	if err == nil {
		if n, ok := dest[0].(int64); ok {
			dest[0] = n - 1
		}
	}
	return err
}

// R2
func TestAPagePastTheCountedPageCountIsRefused(t *testing.T) {
	f := newCopyFixture(t, ChunkSize*3)
	var path string
	if err := f.db.Read.QueryRow(`SELECT file FROM pragma_database_list WHERE name = 'main'`).Scan(&path); err != nil || path == "" {
		t.Fatalf("no store path: %q %v", path, err)
	}
	plain, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	inner := plain.Driver()
	_ = plain.Close()
	liar := sql.OpenDB(liarConnector{inner: inner, dsn: "file:" + path})
	t.Cleanup(func() { _ = liar.Close() })
	f.cfg.DB = liar
	_, err = TakeSnapshot(context.Background(), f.cfg)
	if err == nil || !strings.Contains(err.Error(), "past the counted") {
		t.Fatalf("err = %v, want a refusal naming a page past the counted page count", err)
	}
	nothingRecorded(t, f)
}

// R3: the stream ends after the deadline. Whatever the last flush's select
// picked, a transaction that outlived its pin deadline fails the attempt.
func TestAStreamThatEndsAfterThePinDeadlineFails(t *testing.T) {
	f := newCopyFixture(t, ChunkSize/2)
	f.cfg.PinDeadline = 150 * time.Millisecond
	f.cfg.streamEnded = func() { time.Sleep(300 * time.Millisecond) }
	_, err := TakeSnapshot(context.Background(), f.cfg)
	if !errors.Is(err, ErrPinDeadline) {
		t.Fatalf("err = %v, want ErrPinDeadline: a capture whose pin outlived its deadline succeeded", err)
	}
	nothingRecorded(t, f)
	// And a capture inside the deadline is untouched by the check.
	f.cfg.PinDeadline, f.cfg.streamEnded = time.Minute, nil
	if _, err := TakeSnapshot(context.Background(), f.cfg); err != nil {
		t.Fatalf("a capture inside its deadline failed: %v", err)
	}
}

// R4: a partition's read transaction is bounded by the pin deadline.
func TestAPartitionStreamHitsThePinDeadline(t *testing.T) {
	prev := sourceDropBytes
	sourceDropBytes = 64 << 10
	defer func() { sourceDropBytes = prev }()
	f := newCopyFixture(t, 64<<10)
	part := partitionDB(t, t.TempDir(), "tool_result-20261001.db", 3000)
	f.cfg.Partitions = &PartitionSource{List: func(context.Context, *sql.Tx) ([]PartitionFile, error) {
		return []PartitionFile{{Path: "telemetry/tool_result-20261001.db", DB: part.Read}}, nil
	}}
	f.cfg.FreeSpaceEvery = time.Hour
	f.cfg.PinDeadline = 300 * time.Millisecond
	calls := 0
	f.cfg.FreeSpace = func() error { // the state stream makes at most one call; the partition many
		calls++
		if calls > 2 {
			time.Sleep(60 * time.Millisecond)
		}
		return nil
	}
	_, err := TakeSnapshot(context.Background(), f.cfg)
	if !errors.Is(err, ErrPinDeadline) || !strings.Contains(err.Error(), "tool_result-20261001.db") {
		t.Fatalf("err = %v, want ErrPinDeadline naming the partition", err)
	}
	if calls <= 2 {
		t.Fatalf("test premise: the partition stream made %d calls", calls)
	}
	var busy, logFrames, checkpointed int
	if err := part.Write.QueryRow(`PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &logFrames, &checkpointed); err != nil || busy != 0 {
		t.Fatalf("the partition's transaction is open after the deadline: busy=%d %v", busy, err)
	}
	nothingRecorded(t, f)
}

// R5: Verify opens the staging file in WAL mode and leaves -wal and -shm; the
// install removes them whether it installs or fails. Asserted on the
// directory listing.
func TestRestoreLeavesNoSidecarsAfterAnInstallOrAFailedVerify(t *testing.T) {
	ctx := context.Background()
	listing := func(dir string) []string {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		sort.Strings(names)
		return names
	}
	for _, tc := range []struct {
		name     string
		verifyOK bool
		want     []string
	}{
		{"installed", true, []string{"state.db"}},
		{"failed verify", false, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCopyFixture(t, ChunkSize*2)
			if _, err := TakeSnapshot(ctx, f.cfg); err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			cfg := f.restoreConfig(filepath.Join(dir, "state.db"))
			cfg.Verify = func(_ context.Context, rec Record, path string) error {
				for _, side := range []string{"-wal", "-shm"} {
					if err := os.WriteFile(path+side, []byte("a sidecar verify left"), 0o600); err != nil {
						return err
					}
				}
				if !tc.verifyOK {
					return errors.New("test: verify refuses")
				}
				return testVerify(ctx, rec, path)
			}
			_, err := Restore(ctx, cfg)
			if tc.verifyOK && err != nil {
				t.Fatal(err)
			}
			if !tc.verifyOK && err == nil {
				t.Fatal("restore accepted a refused verify")
			}
			if got := listing(dir); strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("directory after the %s: %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

// R6: the store changes between a timed-out attempt and its retry. Chunk 2 was
// uploaded and its pages did not change: the retry holds it. Chunks 1 (a row)
// and 0 (page 1) were uploaded and changed: the retry uploads the new bytes,
// the old digests are not in the record, only the unchanged chunk is held, and
// the record's page digest and the restored file are the retry's state.
func TestARetryAfterAPinDeadlineWithTheStoreChangedUsesTheRetrysBytes(t *testing.T) {
	ctx := context.Background()
	f := newCopyFixture(t, ChunkSize*3)
	var last int64
	if err := f.db.Read.QueryRow(`PRAGMA page_count`).Scan(&last); err != nil || last <= 3072+12 || last > 4096 {
		t.Fatalf("test premise: %d pages (%v): chunks 0 to 2 are cut by page 3072 and a fourth chunk holds the rest", last, err)
	}
	f.cfg.PinDeadline = 600 * time.Millisecond
	f.cfg.pageHook = func(pgno int64) {
		if pgno > 3072+2 { // after chunk 2 is cut, the reader crawls
			time.Sleep(100 * time.Millisecond)
		}
	}
	if _, err := TakeSnapshot(ctx, f.cfg); !errors.Is(err, ErrPinDeadline) {
		t.Fatalf("first attempt: %v, want ErrPinDeadline", err)
	}
	kept := map[string]bool{}
	f.chunks.mu.Lock()
	for d := range f.chunks.chunks {
		kept[d] = true
	}
	f.chunks.mu.Unlock()
	if len(kept) != 3 {
		t.Fatalf("test premise: %d chunks uploaded before the deadline, want 3", len(kept))
	}
	before := liveWitness(t, f)
	// Row 100's pages are in chunk 1 (about 17 pages a row); page 1 (chunk 0)
	// changes with any commit.
	if _, err := f.db.Write.Exec(`UPDATE blobs SET body = randomblob(65536), note = 'changed' WHERE id = 100`); err != nil {
		t.Fatal(err)
	}
	f.cfg.PinDeadline, f.cfg.pageHook = 0, nil
	ref, err := TakeSnapshot(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	reused := 0
	for _, d := range ref.Record.Manifest.Files[0].Chunks {
		if kept[d] {
			reused++
		}
	}
	if reused < 1 || reused >= len(kept) || ref.Stats.ChunksHeld != reused {
		t.Fatalf("the retry reused %d of the first attempt's %d chunks and held %d: want some reused (chunk 2 did not change) and not all (chunk 1 did), held = reused: %+v", reused, len(kept), ref.Stats.ChunksHeld, ref.Stats)
	}
	restored, _ := restoreAndOpen(t, f)
	got, err := witnessOf(ctx, restored.Read)
	if err != nil {
		t.Fatal(err)
	}
	if now := liveWitness(t, f); got.ContentRoot != now.ContentRoot || got.ContentRoot == before.ContentRoot {
		t.Fatalf("restored %s, live now %s, live before the change %s", got.ContentRoot, now.ContentRoot, before.ContentRoot)
	}
	if want := restoredPageDigest(t, restored); ref.Record.PageDigest != want {
		t.Fatalf("the record's page digest %s is not the retry's pages' %s", ref.Record.PageDigest, want)
	}
}

// R7: a VACUUM and a truncating checkpoint attempted during the pin neither
// block the writer nor change the snapshot. The VACUUM runs while the stream
// is held at page 600; the truncating checkpoint, which waits for readers as
// the store's own maintenance does, runs beside the stream and is joined after.
func TestAVacuumAndATruncatingCheckpointDuringThePinLeaveTheSnapshotAlone(t *testing.T) {
	ctx := context.Background()
	f := newCopyFixture(t, ChunkSize*3)
	if _, err := f.db.Write.Exec(`DELETE FROM blobs WHERE id % 3 = 0`); err != nil { // free pages for VACUUM to move
		t.Fatal(err)
	}
	liveBefore := liveWitness(t, f)
	ran := false
	checkpointDone := make(chan error, 1)
	f.cfg.pageHook = func(pgno int64) {
		if pgno != 600 {
			return
		}
		ran = true
		done := make(chan error, 1)
		go func() {
			_, err := f.db.Write.Exec(`VACUUM`)
			done <- err
		}()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("vacuum during the pin failed: %v", err)
			}
		case <-time.After(30 * time.Second):
			t.Errorf("a VACUUM waited for the stream")
			return
		}
		// The write pool is one connection: the checkpoint, which waits for the
		// reader, starts after the VACUUM and holds it until the pin ends.
		go func() {
			var busy, logFrames, checkpointed int
			err := f.db.Write.QueryRow(`PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &logFrames, &checkpointed)
			checkpointDone <- err
		}()
	}
	ref, err := TakeSnapshot(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Fatal("test premise: the maintenance never ran")
	}
	select {
	case err := <-checkpointDone:
		if err != nil {
			t.Fatalf("the truncating checkpoint failed: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the truncating checkpoint never returned after the pin ended")
	}
	restored, _ := restoreAndOpen(t, f)
	got, err := witnessOf(ctx, restored.Read)
	if err != nil {
		t.Fatal(err)
	}
	if got.ContentRoot != liveBefore.ContentRoot {
		t.Fatalf("restored %s, the store at the snapshot point had %s", got.ContentRoot, liveBefore.ContentRoot)
	}
	if want := restoredPageDigest(t, restored); ref.Record.PageDigest != want {
		t.Fatalf("recorded page digest %s, the restored pages digest to %s", ref.Record.PageDigest, want)
	}
	assertWALReleased(t, f)
}
