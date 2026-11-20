package snapshot

// Tests of S1 of docs/design/snapshot-without-scratch-copy-2026-11-20.md: the
// capture streams the store's pages from the one read transaction that read the
// head, with the witness run in that same transaction before the pages. No
// scratch copy is written.
//
// Ways S1 could fail, written before the code (the design's failure list,
// numbered as there, and the entries that the history of 842e0fda6 adds):
//
//  1. a write committed during the stream (a writer commits between pages)
//     leaks into the pages, the witness or the head: the restored file's head
//     rows and witness are the transaction's;
//  2. the witness and the pages disagree: the restored file's recomputed
//     witness is the one recorded;
//  3. a witness that fails or hits the budget uploads no chunk, and the pages
//     start only after the witness returned (the witness runs first, in the
//     transaction: a reader is open during the witness and during the pages);
//  4. a stream that fails or is cancelled midway registers no root, appends no
//     event, fires ReadReleased once and leaves the WAL releasable;
//  5. a short or out-of-order page stream is refused (streamFile; the partition
//     tests exercise it);
//  6. a file appears under snapshot-tmp on any path (during the witness, during
//     the stream, after the capture, after a failure);
//  7. the stats name the witness and the upload separately, the pin covers the
//     witness, the WAL growth over the pin is reported, and no scratch is
//     reported (ScratchBytes and CopyTime are zero);
//  8. a free-space failure during the pin ends the transaction at once and
//     records nothing (TestTheCaptureAbortsWhenFreeSpaceFallsUnderTheFloor);
//  9. the format changed: the chunks are the live page sequence cut at
//     ChunkSize/pageSize, so a binary from before S1 restores what S1 made;
// 11. the pin makes the WAL hold more than the bound and nothing stops it: the
//     capture's ticker cancels it, releases the read and records nothing
//     (TestThePinWALBoundCancelsTheCaptureAndRecordsNothing);
// 12. a capture that stored chunks and is then cancelled leaves a root or an
//     event; the stream does not drop the source cache as it goes
//     (TestTheStreamDrops...);
// 13. the WAL figure is the WAL file's size, which hides growth inside its slack
//     (TestTheStatsReportTheWALFramesThePinHeld fails a file-size-only measure);
// 10. a store with an incremental-vacuum free list, or past 1 GiB (the lock-byte
//     page), streams through sqlite_dbpage to a restorable file.

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/filecas"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

// walBytesOf reports the size of the store's WAL file.
func walBytesOf(t *testing.T, f *fixture) func() int64 {
	t.Helper()
	var path string
	rows, err := f.db.Read.Query(`PRAGMA database_list`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var seq int
		var name, file string
		if err := rows.Scan(&seq, &name, &file); err != nil {
			t.Fatal(err)
		}
		if name == "main" {
			path = file
		}
	}
	_ = rows.Close()
	if path == "" {
		t.Fatal("no main database file")
	}
	return func() int64 {
		info, err := os.Stat(path + "-wal")
		if err != nil {
			return 0
		}
		return info.Size()
	}
}

// pinned reports whether a reader holds the WAL back: after a commit, a PASSIVE
// checkpoint cannot backfill every frame.
func pinned(t *testing.T, f *fixture) bool {
	t.Helper()
	if _, err := f.db.Write.Exec(`UPDATE blobs SET note = note || '.' WHERE id = 1`); err != nil {
		t.Errorf("write during the capture: %v", err)
		return false
	}
	var busy, logFrames, done int
	if err := f.db.Write.QueryRow(`PRAGMA wal_checkpoint(PASSIVE)`).Scan(&busy, &logFrames, &done); err != nil {
		t.Errorf("checkpoint during the capture: %v", err)
		return false
	}
	return done < logFrames
}

// nothingRecorded asserts a failed attempt left nothing: no chunk put, no root,
// no event, no scratch, and the read transaction released.
func nothingRecorded(t *testing.T, f *fixture) {
	t.Helper()
	f.chunks.mu.Lock()
	puts, roots := len(f.chunks.puts), len(f.chunks.roots)
	f.chunks.mu.Unlock()
	if puts != 0 || roots != 0 || len(f.tape.events) != 0 {
		t.Fatalf("a failed attempt left %d chunks, %d roots, %d events", puts, roots, len(f.tape.events))
	}
	noScratch(t, f)
	assertWALReleased(t, f)
}

// noScratch asserts nothing was ever created at the scratch path.
func noScratch(t *testing.T, f *fixture) {
	t.Helper()
	if _, err := os.Stat(f.cfg.ScratchDir); !os.IsNotExist(err) {
		t.Fatalf("the scratch path %s exists (stat error %v): S1 writes no scratch", f.cfg.ScratchDir, err)
	}
}

// Failures 1 and 2: head, witness and pages are one snapshot while a writer
// commits before the witness and between the pages. The recorded witness, the
// recorded head and the restored file are the transaction's, and the restored
// file recomputes the recorded witness.
func TestTheHeadTheWitnessAndThePagesAreOneSnapshotWhileAWriterCommits(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, ChunkSize+ChunkSize/2)
	headsFixture(t, f)
	liveBefore := liveWitness(t, f)
	var rowsBefore int
	if err := f.db.Write.QueryRow(`SELECT COUNT(*) FROM blobs`).Scan(&rowsBefore); err != nil {
		t.Fatal(err)
	}
	var writes atomic.Int32
	commit := func() {
		if err := commitRow(f.db, 500000+int(writes.Add(1))); err != nil {
			t.Errorf("a write during the capture failed: %v", err)
		}
	}
	f.cfg.Witness = func(ctx context.Context, tx *sql.Tx) (selfdevprotocol.VMLocalContentWitness, error) {
		commit() // between the head and the witness's reads
		w, err := witnessOfTx(ctx, tx)
		commit() // and after them
		return w, err
	}
	f.cfg.pageHook = func(pgno int64) {
		if pgno%16 == 0 {
			commit()
		}
	}
	ref, err := TakeSnapshot(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if writes.Load() < 10 {
		t.Fatalf("test premise: only %d commits during the capture", writes.Load())
	}
	if ref.Record.StoreHeadSequence != uint64(rowsBefore) {
		t.Fatalf("recorded head %d, the transaction's was %d", ref.Record.StoreHeadSequence, rowsBefore)
	}
	if ref.Record.WitnessRoot != liveBefore.ContentRoot {
		t.Fatalf("recorded witness %s is not the transaction's %s", ref.Record.WitnessRoot, liveBefore.ContentRoot)
	}
	if now := liveWitness(t, f); now.ContentRoot == ref.Record.WitnessRoot {
		t.Fatal("test premise: the live store should have moved on")
	}
	restored, rec := restoreAndOpen(t, f)
	var seq, blobs int
	if err := restored.Read.QueryRow(`SELECT seq FROM heads WHERE id = 1`).Scan(&seq); err != nil {
		t.Fatal(err)
	}
	if err := restored.Read.QueryRow(`SELECT COUNT(*) FROM blobs`).Scan(&blobs); err != nil {
		t.Fatal(err)
	}
	if seq != rowsBefore || blobs != rowsBefore || uint64(seq) != rec.StoreHeadSequence {
		t.Fatalf("restored file holds head %d and %d rows, the transaction had %d", seq, blobs, rowsBefore)
	}
	got, err := witnessOf(ctx, restored.Read)
	if err != nil {
		t.Fatal(err)
	}
	if got.ContentRoot != ref.Record.WitnessRoot || got.SchemaHash != ref.Record.SchemaHash {
		t.Fatalf("restored file's witness %s is not the recorded %s", got.ContentRoot, ref.Record.WitnessRoot)
	}
	noScratch(t, f)
	assertWALReleased(t, f)
}

// Failure 3 and the design's order: the witness runs first, inside the
// transaction (a reader pins the WAL during it), and the pages start only after
// it returned (the reader is still open during them).
func TestTheWitnessRunsInsideTheTransactionBeforeThePages(t *testing.T) {
	f := newFixture(t, ChunkSize/2)
	var mu sync.Mutex
	var order []string
	note := func(s string) { mu.Lock(); order = append(order, s); mu.Unlock() }
	var duringWitness, duringPages atomic.Int32 // 1 pinned, 2 not
	mark := func(v *atomic.Int32, pin bool) {
		if pin {
			v.Store(1)
		} else {
			v.Store(2)
		}
	}
	f.cfg.Witness = func(ctx context.Context, tx *sql.Tx) (selfdevprotocol.VMLocalContentWitness, error) {
		note("witness")
		mark(&duringWitness, pinned(t, f))
		return witnessOfTx(ctx, tx)
	}
	f.cfg.pageHook = func(pgno int64) {
		if pgno == 1 {
			note("first page")
		}
		if pgno == 3 {
			mark(&duringPages, pinned(t, f))
		}
	}
	var released atomic.Bool
	f.cfg.ReadReleased = func(context.Context) { released.Store(true) }
	if _, err := TakeSnapshot(context.Background(), f.cfg); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(order) != "[witness first page]" {
		t.Fatalf("order %v, want the witness before the first page", order)
	}
	if duringWitness.Load() != 1 {
		t.Fatalf("no reader of the capture was open during the witness (%d)", duringWitness.Load())
	}
	if duringPages.Load() != 1 {
		t.Fatalf("no reader of the capture was open during the pages (%d)", duringPages.Load())
	}
	if !released.Load() {
		t.Fatal("the read was never released")
	}
	assertWALReleased(t, f)
}

// Failure 6: no file appears under snapshot-tmp, at any point of a capture.
func TestNoFileAppearsUnderSnapshotTmpDuringACapture(t *testing.T) {
	f := newFixture(t, ChunkSize*2)
	parent := filepath.Dir(f.cfg.ScratchDir)
	check := func(when string) {
		if _, err := os.Stat(f.cfg.ScratchDir); !os.IsNotExist(err) {
			t.Errorf("%s: %s exists (%v)", when, f.cfg.ScratchDir, err)
		}
		if entries, _ := os.ReadDir(parent); len(entries) != 0 {
			t.Errorf("%s: %d entries beside the scratch path", when, len(entries))
		}
	}
	f.cfg.Witness = func(ctx context.Context, tx *sql.Tx) (selfdevprotocol.VMLocalContentWitness, error) {
		check("in the witness")
		return witnessOfTx(ctx, tx)
	}
	f.cfg.pageHook = func(pgno int64) {
		if pgno%256 == 0 {
			check(fmt.Sprintf("at page %d", pgno))
		}
	}
	if _, err := TakeSnapshot(context.Background(), f.cfg); err != nil {
		t.Fatal(err)
	}
	check("after the capture")
}

// Failures 4 and 6: a capture cancelled mid-stream leaves nothing and records
// nothing.
func TestACaptureCancelledMidStreamLeavesNothingAndRecordsNothing(t *testing.T) {
	f := newFixture(t, ChunkSize/2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var released atomic.Int32
	f.cfg.ReadReleased = func(context.Context) { released.Add(1) }
	f.cfg.pageHook = func(pgno int64) {
		if pgno == 3 {
			cancel()
		}
	}
	_, err := TakeSnapshot(ctx, f.cfg)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if released.Load() != 1 {
		t.Fatalf("ReadReleased ran %d times, want once", released.Load())
	}
	nothingRecorded(t, f)
}

// Failure 3: a witness that fails uploads and probes nothing.
func TestAWitnessThatFailsUploadsNothing(t *testing.T) {
	f := newFixture(t, ChunkSize*2)
	f.cfg.Witness = func(context.Context, *sql.Tx) (selfdevprotocol.VMLocalContentWitness, error) {
		return selfdevprotocol.VMLocalContentWitness{}, errors.New("witness failed")
	}
	if _, err := TakeSnapshot(context.Background(), f.cfg); err == nil {
		t.Fatal("a failed witness recorded a snapshot")
	}
	f.chunks.mu.Lock()
	probes := f.chunks.probes
	f.chunks.mu.Unlock()
	if probes != 0 {
		t.Fatalf("a failed witness probed %d chunks", probes)
	}
	nothingRecorded(t, f)
}

// Failure 4: ReadReleased fires exactly once on every path, with a context the
// caller's cancellation cannot end, and nothing is left.
func TestReadReleasedFiresOnceOnEveryPath(t *testing.T) {
	type tc struct {
		name  string
		setup func(f *fixture, cancel context.CancelFunc)
		want  error
		fail  bool
		// rootOK: the failure is after the root was registered (the append).
		rootOK bool
	}
	for _, c := range []tc{
		{name: "success", setup: func(*fixture, context.CancelFunc) {}},
		{name: "head failure", fail: true, setup: func(f *fixture, _ context.CancelFunc) {
			f.cfg.Head = func(context.Context, *sql.Tx) (*computerevent.Head, error) { return nil, errors.New("head failed") }
		}},
		{name: "witness failure", fail: true, setup: func(f *fixture, _ context.CancelFunc) {
			f.cfg.Witness = func(context.Context, *sql.Tx) (selfdevprotocol.VMLocalContentWitness, error) {
				return selfdevprotocol.VMLocalContentWitness{}, errors.New("witness failed")
			}
		}},
		{name: "cancel during the stream", want: context.Canceled, setup: func(f *fixture, cancel context.CancelFunc) {
			f.cfg.pageHook = func(pgno int64) {
				if pgno == 3 {
					cancel()
				}
			}
		}},
		{name: "time budget during the stream", want: ErrTimeBudget, setup: func(f *fixture, _ context.CancelFunc) {
			f.cfg.MaxDuration = 300 * time.Millisecond
			f.cfg.pageHook = func(pgno int64) {
				if pgno == 3 {
					time.Sleep(600 * time.Millisecond)
				}
			}
		}},
		{name: "upload failure", fail: true, setup: func(f *fixture, _ context.CancelFunc) {
			f.cfg.Chunks = failingChunks{ChunkStore: f.chunks, err: errors.New("put failed")}
		}},
		{name: "register failure", fail: true, setup: func(f *fixture, _ context.CancelFunc) { f.cfg.Chunks = failingRoots{f.chunks} }},
		{name: "append failure", fail: true, rootOK: true, setup: func(f *fixture, _ context.CancelFunc) { f.cfg.Appender = failingAppender{f.tape} }},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t, ChunkSize/2)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var calls atomic.Int32
			var hookCtxErr atomic.Value
			f.cfg.ReadReleased = func(hctx context.Context) {
				calls.Add(1)
				if err := hctx.Err(); err != nil {
					hookCtxErr.Store(err)
				}
			}
			c.setup(f, cancel)
			_, err := TakeSnapshot(ctx, f.cfg)
			switch {
			case c.want != nil:
				if !errors.Is(err, c.want) {
					t.Fatalf("err = %v, want %v", err, c.want)
				}
			case c.fail:
				if err == nil {
					t.Fatal("expected an error")
				}
			case err != nil:
				t.Fatal(err)
			}
			if got := calls.Load(); got != 1 {
				t.Fatalf("ReadReleased ran %d times, want exactly once", got)
			}
			if v := hookCtxErr.Load(); v != nil {
				t.Fatalf("the hook's context was already done: %v", v)
			}
			noScratch(t, f)
			assertWALReleased(t, f)
			// A failed capture appends no event, and registers no root except
			// when only the append failed (the root is declared before the event
			// by design; the host sweeps an event-less root, snapshots-cost-the-
			// change section 5).
			if err != nil && !c.rootOK && len(f.chunks.roots) != 0 {
				t.Fatalf("a failed capture registered %d roots", len(f.chunks.roots))
			}
			if err != nil && len(f.tape.events) != 0 {
				t.Fatalf("a failed capture appended %d events", len(f.tape.events))
			}
		})
	}
}

// A hook that blocks is ended by the capture's own duration and never fails
// the attempt; with no MaxDuration the hook has no deadline.
func TestABlockedReadReleasedHookIsBoundedByMaxDuration(t *testing.T) {
	f := newFixture(t, ChunkSize/4)
	f.cfg.MaxDuration = 500 * time.Millisecond
	var hadDeadline atomic.Bool
	f.cfg.ReadReleased = func(ctx context.Context) {
		_, ok := ctx.Deadline()
		hadDeadline.Store(ok)
		<-ctx.Done()
	}
	started := time.Now()
	if _, err := TakeSnapshot(context.Background(), f.cfg); err != nil {
		t.Fatalf("a timed-out hook failed the attempt: %v", err)
	}
	if !hadDeadline.Load() {
		t.Fatal("the hook's context has no deadline")
	}
	if took := time.Since(started); took > 20*time.Second {
		t.Fatalf("the capture waited %s for a blocked hook", took)
	}

	g := newFixture(t, ChunkSize/4)
	var gDeadline atomic.Bool
	g.cfg.ReadReleased = func(ctx context.Context) {
		_, ok := ctx.Deadline()
		gDeadline.Store(ok)
	}
	if _, err := TakeSnapshot(context.Background(), g.cfg); err != nil {
		t.Fatal(err)
	}
	if gDeadline.Load() {
		t.Fatal("with MaxDuration zero the hook was given a deadline")
	}
}

// Failure 7: the stats name the witness, the stream and the drain separately, the pin
// covers the witness, and no scratch is reported (the WAL figures are in
// TestTheStatsReportTheWALFramesThePinHeld: the file's size is not one).
func TestTheStatsNameTheWitnessTheUploadAndTheWALGrowth(t *testing.T) {
	f := newFixture(t, 3*ChunkSize)
	f.cfg.WALBytes = walBytesOf(t, f)
	f.cfg.pageHook = func(pgno int64) {
		// 1 MiB commits every 128 pages: more than the WAL file's size left over
		// from building the store, so the file itself grows.
		if pgno%128 == 0 {
			if _, err := f.db.Write.Exec(`INSERT INTO blobs (id, body, note) VALUES (?, randomblob(1048576), 'during')`, 900000+int(pgno)); err != nil {
				t.Errorf("write during the stream: %v", err)
			}
		}
	}
	ref, err := TakeSnapshot(context.Background(), f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	st := ref.Stats
	if st.WitnessTime <= 0 || st.StreamTime <= 0 || st.DrainTime < 0 || st.PinTime < st.WitnessTime+st.StreamTime {
		t.Fatalf("pin %s, witness %s, stream %s, drain %s: the phases must be named and the pin must cover the witness and the stream", st.PinTime, st.WitnessTime, st.StreamTime, st.DrainTime)
	}
	if st.PinTime < st.WitnessTime {
		t.Fatalf("pin %s is shorter than the witness %s it covers", st.PinTime, st.WitnessTime)
	}
	if st.ScratchBytes != 0 || st.CopyTime != 0 {
		t.Fatalf("scratch %d bytes, copy %s: S1 copies nothing", st.ScratchBytes, st.CopyTime)
	}
	noScratch(t, f)
}

// Failure 9: the chunks are the live page sequence cut by ChunkSize/pageSize
// (the same digests as sealing the live pages), and install through the
// unchanged Restore with the witness check, so a binary from before S1 restores
// what S1 made.
func TestTheStreamedChunksAreTheLivePageSequence(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, ChunkSize*2+ChunkSize/2)
	ref, err := TakeSnapshot(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	dek, err := f.cipher.ExportKeyForEscrow(ctx, testComputer)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := f.db.Read.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT pgno, data FROM sqlite_dbpage`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var chunk []byte
	var digests []string
	for rows.Next() {
		var pgno int64
		var data []byte
		if err := rows.Scan(&pgno, &data); err != nil {
			t.Fatal(err)
		}
		chunk = append(chunk, data...)
		if len(chunk) == ChunkSize {
			_, d, err := filecas.SealChunk(dek, testComputer, chunk)
			if err != nil {
				t.Fatal(err)
			}
			digests = append(digests, d)
			chunk = chunk[:0]
		}
	}
	if len(chunk) > 0 {
		_, d, err := filecas.SealChunk(dek, testComputer, chunk)
		if err != nil {
			t.Fatal(err)
		}
		digests = append(digests, d)
	}
	if got := ref.Record.Manifest.Files[0].Chunks; fmt.Sprint(got) != fmt.Sprint(digests) {
		t.Fatalf("chunk digests differ from the live page sequence:\n got %v\nwant %v", got, digests)
	}
	cfg := f.restoreConfig(filepath.Join(t.TempDir(), "state.db"))
	cfg.Verify = func(ctx context.Context, rec Record, path string) error {
		db, err := dbx.OpenSQLite(path, dbx.SQLiteOptions{ReadConns: 1})
		if err != nil {
			return err
		}
		defer db.Close()
		w, err := witnessOf(ctx, db.Read)
		if err != nil {
			return err
		}
		if w.ContentRoot != rec.WitnessRoot {
			return fmt.Errorf("witness %s, record %s", w.ContentRoot, rec.WitnessRoot)
		}
		return nil
	}
	if _, err := Restore(ctx, cfg); err != nil {
		t.Fatal(err)
	}
}

const pagedHeadsSQL = `CREATE TABLE heads (id INTEGER PRIMARY KEY CHECK (id = 1), seq INTEGER NOT NULL)`

// pagedFixture builds a fixture whose store has the given page size and, when
// incremental, auto_vacuum=INCREMENTAL, created before WAL mode is set.
func pagedFixture(t *testing.T, pageSize int, incremental bool, rows int, rowBytes int) *fixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state.db")
	plain, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	pragmas := []string{fmt.Sprintf(`PRAGMA page_size=%d`, pageSize)}
	if incremental {
		pragmas = append(pragmas, `PRAGMA auto_vacuum=INCREMENTAL`)
	}
	for _, q := range append(pragmas, `CREATE TABLE blobs (id INTEGER PRIMARY KEY, body BLOB NOT NULL, note TEXT)`) {
		if _, err := plain.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err := plain.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := dbx.OpenSQLite(path, dbx.SQLiteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for i := 0; i < rows; i++ {
		body := make([]byte, rowBytes)
		_, _ = rand.Read(body)
		if _, err := db.Write.Exec(`INSERT INTO blobs (id, body, note) VALUES (?, ?, ?)`, i+1, body, fmt.Sprintf("row %d", i)); err != nil {
			t.Fatal(err)
		}
	}
	cipher := testCipher(t, testComputer)
	tape := &fakeTape{t: t, cipher: cipher, payloads: map[string][]byte{}}
	chunks := newMemChunks()
	f := &fixture{db: db, tape: tape, chunks: chunks, cipher: cipher, cfg: Config{
		ComputerID: testComputer, DB: db.Read, Cipher: cipher, Chunks: chunks, Appender: tape,
		ScratchDir: filepath.Join(t.TempDir(), "snapshot-tmp"),
		Witness:    witnessOfTx,
	}}
	if _, err := db.Write.Exec(pagedHeadsSQL); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Write.Exec(`INSERT INTO heads (id, seq) VALUES (1, ?)`, rows); err != nil {
		t.Fatal(err)
	}
	f.cfg.Head = headsFixtureHead
	return f
}

func roundTripWitness(t *testing.T, f *fixture) {
	t.Helper()
	ctx := context.Background()
	ref, err := TakeSnapshot(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	live := liveWitness(t, f)
	if ref.Record.WitnessRoot != live.ContentRoot {
		t.Fatalf("recorded witness %s, live %s", ref.Record.WitnessRoot, live.ContentRoot)
	}
	restored, _ := restoreAndOpen(t, f)
	got, err := witnessOf(ctx, restored.Read)
	if err != nil {
		t.Fatal(err)
	}
	if got.ContentRoot != ref.Record.WitnessRoot {
		t.Fatalf("restored witness %s, record %s", got.ContentRoot, ref.Record.WitnessRoot)
	}
	noScratch(t, f)
}

// Failure 10: an auto_vacuum=INCREMENTAL store whose file shrank by
// incremental_vacuum (a stale in-header size is possible) streams and restores.
func TestAnIncrementallyVacuumedStoreStreamsAndRestores(t *testing.T) {
	f := pagedFixture(t, 4096, true, 60, 60<<10)
	if _, err := f.db.Write.Exec(`DELETE FROM blobs WHERE id % 2 = 0`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Write.Exec(`PRAGMA incremental_vacuum(100000)`); err != nil {
		t.Fatal(err)
	}
	roundTripWitness(t, f)
}

// Failure 10, the lock-byte page: a store past 1 GiB streams and restores.
// Skipped under -short: it writes about 1.1 GiB twice.
func TestAStorePastOneGiBStreamsAndRestores(t *testing.T) {
	if testing.Short() {
		t.Skip("writes a store past 1 GiB")
	}
	f := pagedFixture(t, 65536, false, 0, 0)
	zero := make([]byte, 1<<20)
	for i := 0; i < 1100; i++ {
		if _, err := f.db.Write.Exec(`INSERT INTO blobs (id, body, note) VALUES (?, ?, ?)`, i+1, zero, fmt.Sprintf("row %d", i)); err != nil {
			t.Fatal(err)
		}
	}
	roundTripWitness(t, f)
}

// failingChunks, failingRoots and failingAppender fail one step of the end of
// a capture.
type failingChunks struct {
	ChunkStore
	err error
}

func (c failingChunks) PutChunk(context.Context, string, []byte) error { return c.err }

type failingRoots struct {
	*memChunks
}

func (failingRoots) RegisterSnapshotRoot(context.Context, string, uint64, []string) error {
	return errors.New("register failed")
}

type failingAppender struct{ Appender }

func (failingAppender) AppendNewPrivatePayload(context.Context, computerevent.Event, computerevent.TransitionInput, []byte, string, *computerevent.PrivateArtifactCipher) (computerevent.Receipt, string, error) {
	return computerevent.Receipt{}, "", errors.New("append failed")
}

// headsFixtureHead reads the head headsFixture and pagedFixture store.
func headsFixtureHead(ctx context.Context, tx *sql.Tx) (*computerevent.Head, error) {
	var seq uint64
	if err := tx.QueryRowContext(ctx, `SELECT seq FROM heads WHERE id = 1`).Scan(&seq); err != nil {
		return nil, err
	}
	return &computerevent.Head{ComputerID: testComputer, Sequence: seq, CanonicalEventHead: "d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0"}, nil
}

// walFramesOf reads the frame count the way the store does: the log counter of
// a PASSIVE checkpoint on the write connection.
func walFramesOf(f *fixture) func(context.Context) (int64, error) {
	return func(ctx context.Context) (int64, error) {
		var busy, logFrames, done int64
		err := f.db.Write.QueryRowContext(ctx, `PRAGMA wal_checkpoint(PASSIVE)`).Scan(&busy, &logFrames, &done)
		return logFrames, err
	}
}

// Failure 7 (review of fab0d6a0e): the WAL is sampled only after the read mark
// is held (after the head, never before: a restart between the sample and the
// mark would read as negative growth), and again just before it is released,
// while the reader still pins it.
func TestTheWALIsSampledAfterTheReadMarkIsHeldAndBeforeItIsReleased(t *testing.T) {
	f := newFixture(t, ChunkSize/2)
	var mu sync.Mutex
	var order []string
	note := func(s string) { mu.Lock(); order = append(order, s); mu.Unlock() }
	head := f.cfg.Head
	f.cfg.Head = func(ctx context.Context, tx *sql.Tx) (*computerevent.Head, error) {
		note("head")
		return head(ctx, tx)
	}
	f.cfg.WALBytes = func() int64 { note("file"); return 0 }
	frames := walFramesOf(f)
	f.cfg.WALFrames = func(ctx context.Context) (int64, error) { note("frames"); return frames(ctx) }
	witness := f.cfg.Witness
	f.cfg.Witness = func(ctx context.Context, tx *sql.Tx) (selfdevprotocol.VMLocalContentWitness, error) {
		note("witness")
		return witness(ctx, tx)
	}
	f.cfg.ReadReleased = func(context.Context) { note("released") }
	if _, err := TakeSnapshot(context.Background(), f.cfg); err != nil {
		t.Fatal(err)
	}
	if got, want := fmt.Sprint(order), "[head file frames witness file frames released]"; got != want {
		t.Fatalf("order %s, want %s", got, want)
	}
}

// Failure 7: the frames the pin made the WAL hold are reported even when the WAL
// file does not grow, and the byte figure is the frames times the page size.
func TestTheStatsReportTheWALFramesThePinHeld(t *testing.T) {
	f := newFixture(t, 3*ChunkSize)
	f.cfg.WALFrames = walFramesOf(f)
	f.cfg.WALBytes = walBytesOf(t, f)
	var writes int
	f.cfg.pageHook = func(pgno int64) {
		if pgno%128 == 0 {
			writes++
			if _, err := f.db.Write.Exec(`INSERT INTO blobs (id, body, note) VALUES (?, randomblob(65536), 'during')`, 800000+int(pgno)); err != nil {
				t.Errorf("write during the stream: %v", err)
			}
		}
	}
	ref, err := TakeSnapshot(context.Background(), f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	st := ref.Stats
	if writes == 0 || !st.PinWALFramesKnown {
		t.Fatalf("premise: %d writes, frames known %v", writes, st.PinWALFramesKnown)
	}
	// The premise that makes this test fail a file-size-only measure: the small
	// writes fit inside the WAL file's existing slack, so the file does not grow
	// while the frames do.
	if st.PinWALEnd != st.PinWALStart {
		t.Fatalf("premise: the WAL file grew from %d to %d bytes, the writes must fit in its slack", st.PinWALStart, st.PinWALEnd)
	}
	delta := st.PinWALFramesEnd - st.PinWALFramesStart
	if delta < int64(writes) {
		t.Fatalf("frames %d to %d: %d commits under the pin held back only %d frames", st.PinWALFramesStart, st.PinWALFramesEnd, writes, delta)
	}
	if st.PinWALFrameBytes != delta*int64(st.PageSize) {
		t.Fatalf("frame bytes %d, want %d frames of %d bytes", st.PinWALFrameBytes, delta, st.PageSize)
	}
}

// A frame count that cannot be read is said so, and does not fail the capture.
func TestAnUnreadableWALFrameCountDoesNotFailTheCapture(t *testing.T) {
	f := newFixture(t, ChunkSize/4)
	f.cfg.WALFrames = func(context.Context) (int64, error) { return 0, errors.New("no checkpoint") }
	ref, err := TakeSnapshot(context.Background(), f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if ref.Stats.PinWALFramesKnown {
		t.Fatal("an unreadable frame count was reported as known")
	}
}

// Failure 11 (review of fab0d6a0e, the property snapshot_cache_test.go pinned):
// the stream drops the live store's page cache every sourceDropBytes of pages
// read (ranges that follow each other from page 0), drops the whole file and its
// WAL at the end, and also does on a failure.
func TestTheStreamDropsTheSourceCacheAsItGoesAndAtTheEndAndOnFailure(t *testing.T) {
	type span struct {
		path   string
		off, n int64
	}
	var mu sync.Mutex
	var spans []span
	old := dropSourceCache
	dropSourceCache = func(p string, off, n int64) {
		mu.Lock()
		spans = append(spans, span{p, off, n})
		mu.Unlock()
	}
	t.Cleanup(func() { dropSourceCache = old })
	reset := func() { mu.Lock(); spans = nil; mu.Unlock() }

	f := newFixture(t, 2*sourceDropBytes+ChunkSize)
	live := ""
	{
		rows, err := f.db.Read.Query(`PRAGMA database_list`)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var seq int
			var name, file string
			if err := rows.Scan(&seq, &name, &file); err != nil {
				t.Fatal(err)
			}
			if name == "main" {
				live = file
			}
		}
		_ = rows.Close()
	}
	if live == "" {
		t.Fatal("no live store path")
	}
	check := func(label string, wantRanges int) {
		mu.Lock()
		defer mu.Unlock()
		var next int64
		ranges, whole := 0, 0
		for _, s := range spans {
			if s.path != live {
				t.Fatalf("%s: a drop named %q, want only the live store %q", label, s.path, live)
			}
			switch {
			case s.off == 0 && s.n == 0:
				whole++
			case s.off == next && s.n >= sourceDropBytes:
				ranges++
				next += s.n
			default:
				t.Fatalf("%s: drop %+v does not continue from %d with at least %d bytes", label, s, next, sourceDropBytes)
			}
		}
		if ranges < wantRanges || whole < 1 {
			t.Fatalf("%s: %d range drops and %d whole-file drops, want at least %d and 1 (%v)", label, ranges, whole, wantRanges, spans)
		}
	}
	if _, err := TakeSnapshot(context.Background(), f.cfg); err != nil {
		t.Fatal(err)
	}
	check("success", 2)

	reset()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.cfg.pageHook = func(pgno int64) {
		if pgno == 20 {
			cancel()
		}
	}
	if _, err := TakeSnapshot(ctx, f.cfg); err == nil {
		t.Fatal("a cancelled capture succeeded")
	}
	mu.Lock()
	whole := 0
	for _, s := range spans {
		if s.off == 0 && s.n == 0 {
			whole++
		}
	}
	mu.Unlock()
	if whole < 1 {
		t.Fatalf("a failed stream dropped no whole-file cache (%v)", spans)
	}
}

// Review of fab0d6a0e (the tripwire): when the pin makes the WAL hold more than
// MaxPinWALBytes, the capture's own ticker cancels it, the pin is released,
// nothing is recorded and the error says why.
func TestThePinWALBoundCancelsTheCaptureAndRecordsNothing(t *testing.T) {
	f := newFixture(t, 3*ChunkSize)
	f.cfg.WALFrames = walFramesOf(f)
	f.cfg.FreeSpaceEvery = 10 * time.Millisecond
	f.cfg.MaxPinWALBytes = 256 << 10 // a small bound injected for the test
	var released atomic.Int32
	f.cfg.ReadReleased = func(context.Context) { released.Add(1) }
	f.cfg.pageHook = func(pgno int64) {
		if pgno == 8 {
			// 1 MiB of commits under the pin, then let the ticker see them.
			for i := 0; i < 16; i++ {
				if _, err := f.db.Write.Exec(`INSERT INTO blobs (id, body, note) VALUES (?, randomblob(65536), 'bound')`, 700000+i); err != nil {
					t.Errorf("write under the pin: %v", err)
				}
			}
			time.Sleep(300 * time.Millisecond)
		}
	}
	_, err := TakeSnapshot(context.Background(), f.cfg)
	if !errors.Is(err, ErrPinWALBound) {
		t.Fatalf("err = %v, want ErrPinWALBound", err)
	}
	if released.Load() != 1 {
		t.Fatalf("ReadReleased ran %d times, want once", released.Load())
	}
	nothingRecorded(t, f)
}

// And a pin that holds less than the bound is not cancelled.
func TestAPinUnderTheWALBoundIsNotCancelled(t *testing.T) {
	f := newFixture(t, ChunkSize/2)
	f.cfg.WALFrames = walFramesOf(f)
	f.cfg.FreeSpaceEvery = 10 * time.Millisecond
	f.cfg.MaxPinWALBytes = 64 << 20
	f.cfg.pageHook = func(pgno int64) {
		if pgno == 8 {
			time.Sleep(100 * time.Millisecond) // several ticks
		}
	}
	if _, err := TakeSnapshot(context.Background(), f.cfg); err != nil {
		t.Fatal(err)
	}
}

// cancelAfterPut cancels the capture once n chunks are stored.
type cancelAfterPut struct {
	*memChunks
	cancel context.CancelFunc
	after  int
}

func (c *cancelAfterPut) PutChunk(ctx context.Context, d string, b []byte) error {
	err := c.memChunks.PutChunk(ctx, d, b)
	c.memChunks.mu.Lock()
	n := len(c.memChunks.puts)
	c.memChunks.mu.Unlock()
	if n >= c.after {
		c.cancel()
	}
	return err
}

// Failure 4 (review of fab0d6a0e): a capture that has already stored chunks and
// is then cancelled registers no root and appends no event.
func TestACaptureCancelledAfterStoringChunksRegistersNoRootAndNoEvent(t *testing.T) {
	f := newFixture(t, ChunkSize*3)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.cfg.Chunks = &cancelAfterPut{memChunks: f.chunks, cancel: cancel, after: 1}
	_, err := TakeSnapshot(ctx, f.cfg)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	f.chunks.mu.Lock()
	puts, roots := len(f.chunks.puts), len(f.chunks.roots)
	f.chunks.mu.Unlock()
	if puts < 1 {
		t.Fatalf("premise: %d chunks stored before the cancel", puts)
	}
	if roots != 0 || len(f.tape.events) != 0 {
		t.Fatalf("a cancelled capture left %d roots and %d events", roots, len(f.tape.events))
	}
	noScratch(t, f)
	assertWALReleased(t, f)
}

// Review of 41fbba5c0: samples that cannot read the WAL's frame count (a busy
// checkpoint, an error) neither disarm the tripwire silently nor report a
// figure: maxUnknownWALTicks in a row cancel the capture as unable to check its
// bound, and nothing is recorded.
func TestUnknownWALSamplesInARowCancelTheCapture(t *testing.T) {
	f := newFixture(t, 3*ChunkSize)
	f.cfg.FreeSpaceEvery = 5 * time.Millisecond
	f.cfg.MaxPinWALBytes = 1 << 30
	var calls atomic.Int32
	frames := walFramesOf(f)
	f.cfg.WALFrames = func(ctx context.Context) (int64, error) {
		if calls.Add(1) == 1 {
			return frames(ctx) // the start sample is known
		}
		return 0, errors.New("the checkpoint lock was busy")
	}
	f.cfg.pageHook = func(pgno int64) {
		if pgno == 8 {
			time.Sleep(500 * time.Millisecond) // many ticks
		}
	}
	_, err := TakeSnapshot(context.Background(), f.cfg)
	if !errors.Is(err, ErrPinWALUnchecked) {
		t.Fatalf("err = %v, want ErrPinWALUnchecked", err)
	}
	nothingRecorded(t, f)
}

// An unknown sample followed by a known one does not cancel (the run of unknowns
// is consecutive), and a known sample below the start is unknown, never a
// negative figure.
func TestAnIsolatedUnknownWALSampleDoesNotCancelAndNeverReportsNegative(t *testing.T) {
	f := newFixture(t, ChunkSize)
	f.cfg.FreeSpaceEvery = 5 * time.Millisecond
	f.cfg.MaxPinWALBytes = 1 << 30
	var calls atomic.Int32
	f.cfg.WALFrames = func(context.Context) (int64, error) {
		switch n := calls.Add(1); {
		case n == 1:
			return 1000, nil // start
		case n%2 == 0 && n < 20:
			return 0, errors.New("busy") // alternating unknown and known ticks
		default:
			return 1000, nil
		}
	}
	f.cfg.pageHook = func(pgno int64) {
		if pgno == 4 {
			time.Sleep(150 * time.Millisecond)
		}
	}
	ref, err := TakeSnapshot(context.Background(), f.cfg)
	if err != nil {
		t.Fatalf("alternating unknown samples cancelled the capture: %v", err)
	}
	if ref.Stats.PinWALFramesKnown && ref.Stats.PinWALFramesEnd < ref.Stats.PinWALFramesStart {
		t.Fatalf("negative growth reported: %d to %d", ref.Stats.PinWALFramesStart, ref.Stats.PinWALFramesEnd)
	}
	// An end sample below the start is unknown, not negative.
	g := newFixture(t, ChunkSize/4)
	var gc atomic.Int32
	g.cfg.WALFrames = func(context.Context) (int64, error) {
		if gc.Add(1) == 1 {
			return 5000, nil
		}
		return 10, nil
	}
	gref, err := TakeSnapshot(context.Background(), g.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if gref.Stats.PinWALFramesKnown || gref.Stats.PinWALFrameBytes != 0 {
		t.Fatalf("an end sample below the start was reported: known=%v bytes=%d", gref.Stats.PinWALFramesKnown, gref.Stats.PinWALFrameBytes)
	}
}

// The tripwire cannot fire after the pin is released, and no WAL sample is taken
// after TakeSnapshot returns (the guard is joined).
func TestNoTickMeasuresTheWALAfterThePinIsReleased(t *testing.T) {
	f := newFixture(t, 3*ChunkSize)
	f.cfg.FreeSpaceEvery = 3 * time.Millisecond
	f.cfg.MaxPinWALBytes = 1 << 20
	var released atomic.Bool
	var afterRelease, total atomic.Int32
	frames := walFramesOf(f)
	f.cfg.WALFrames = func(ctx context.Context) (int64, error) {
		total.Add(1)
		if released.Load() {
			afterRelease.Add(1)
			return 1 << 40, nil // would trip the bound if it were read
		}
		return frames(ctx)
	}
	f.cfg.ReadReleased = func(context.Context) { released.Store(true) }
	f.cfg.Chunks = slowChunks{f.chunks, 80 * time.Millisecond} // a drain after the pin
	f.cfg.pageHook = func(pgno int64) {
		if pgno == 4 {
			time.Sleep(60 * time.Millisecond)
		}
	}
	if _, err := TakeSnapshot(context.Background(), f.cfg); err != nil {
		t.Fatalf("the capture was cancelled: %v", err)
	}
	// The end sample is taken once, before the release: it is not "after".
	at := total.Load()
	time.Sleep(100 * time.Millisecond)
	if total.Load() != at {
		t.Fatalf("a WAL sample was taken after TakeSnapshot returned (%d then %d)", at, total.Load())
	}
	if afterRelease.Load() != 0 {
		t.Fatalf("%d WAL samples were taken after the pin was released", afterRelease.Load())
	}
}

// slowChunks delays every chunk put.
type slowChunks struct {
	*memChunks
	d time.Duration
}

func (c slowChunks) PutChunk(ctx context.Context, d string, b []byte) error {
	time.Sleep(c.d)
	return c.memChunks.PutChunk(ctx, d, b)
}

// WALSampled receives the bytes the pin has made the WAL hold.
func TestWALSampledReportsTheBytesThePinHolds(t *testing.T) {
	f := newFixture(t, 3*ChunkSize)
	f.cfg.FreeSpaceEvery = 5 * time.Millisecond
	f.cfg.MaxPinWALBytes = 1 << 30
	f.cfg.WALFrames = walFramesOf(f)
	var mu sync.Mutex
	var seen []int64
	f.cfg.WALSampled = func(held int64) { mu.Lock(); seen = append(seen, held); mu.Unlock() }
	f.cfg.pageHook = func(pgno int64) {
		if pgno == 8 {
			for i := 0; i < 8; i++ {
				if _, err := f.db.Write.Exec(`INSERT INTO blobs (id, body, note) VALUES (?, randomblob(65536), 's')`, 600000+i); err != nil {
					t.Error(err)
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	if _, err := TakeSnapshot(context.Background(), f.cfg); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	var max int64
	for _, h := range seen {
		if h < 0 {
			t.Fatalf("negative held bytes %v", seen)
		}
		if h > max {
			max = h
		}
	}
	if max <= 0 {
		t.Fatalf("no positive sample of the held WAL: %v", seen)
	}
}
