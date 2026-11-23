package snapshot

// Tests of the capture in one pass (docs/design/snapshot-one-pass-2026-11-22.md,
// stage A2). Ways it could fail, written before the code:
//
//  O1. a write commits while the stream runs (the stream is held back by its
//      uploader, so this is the normal case) and the snapshot is not exactly the
//      head it recorded: the restored file holds rows past the head or lacks
//      rows below it;
//  O2. the digest in the record is not the digest of the pages that were
//      uploaded (it was taken on other bytes, or before a buffer was reused),
//      or a torn or truncated stream is accepted by restore;
//  O3. the read transaction outlives its deadline, because the uploader
//      stalled: the WAL stays pinned, or the attempt fails with some other cause,
//      or it leaves a root or an event, or it throws away the chunks it has
//      already uploaded so the retry starts again;
//  O4. a file the size of the store is written during the capture (the scratch
//      copy came back under another name);
//  O5. the free space is checked only once a minute, or only before the attempt,
//      and a pinned WAL fills the volume;
//  O6. a failure on any path leaves a root or an event, runs the read-released
//      hook not exactly once, or leaves the WAL pinned;
//  O7. a version 1 record, written by an older capture, no longer restores;
//  O8. a page size other than 4096, an incrementally vacuumed store or a store
//      past SQLite's 1 GiB lock-byte page breaks the stream.

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/filecas"
)

const copyHeadSelect = `SELECT computer_id, sequence, canonical_event_head, desired_event_head, effective_event_head, COALESCE(pending_transition_ref, ''), desired_state_commitment, effective_state_commitment, reducer_version, credential_revocation_epoch FROM heads WHERE computer_id = 'c'`

type headQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// headFromTable reads the ten columns store.Head reads, from a `heads` table
// with the same column names; nil for no row.
func headFromTable(ctx context.Context, q headQueryer) (*computerevent.Head, error) {
	var h computerevent.Head
	err := q.QueryRowContext(ctx, copyHeadSelect).Scan(&h.ComputerID, &h.Sequence, &h.CanonicalEventHead, &h.DesiredEventHead, &h.EffectiveEventHead, &h.PendingTransitionRef, &h.DesiredStateCommitment, &h.EffectiveStateCommitment, &h.ReducerVersion, &h.CredentialRevocationEpoch)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &h, err
}

const copyHeadSchema = `CREATE TABLE heads (computer_id TEXT PRIMARY KEY, sequence INTEGER NOT NULL, canonical_event_head TEXT NOT NULL, desired_event_head TEXT NOT NULL, effective_event_head TEXT NOT NULL, pending_transition_ref TEXT, desired_state_commitment TEXT NOT NULL, effective_state_commitment TEXT NOT NULL, reducer_version INTEGER NOT NULL, credential_revocation_epoch INTEGER NOT NULL)`

// newCopyFixture is newFixture with a head read from a ten-column table.
func newCopyFixture(t *testing.T, bytesApprox int) *fixture {
	t.Helper()
	f := newFixture(t, bytesApprox)
	for _, stmt := range []string{copyHeadSchema, `INSERT INTO heads VALUES ('c', 41, 'canon', 'desired', 'effective', NULL, 'dsc', 'esc', 1, 3)`} {
		if _, err := f.db.Write.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	f.cfg.Head = func(ctx context.Context, tx *sql.Tx) (*computerevent.Head, error) { return headFromTable(ctx, tx) }
	return f
}

// nothingRecorded asserts a failed attempt left no root and no event, and the
// read transaction released.
func nothingRecorded(t *testing.T, f *fixture) {
	t.Helper()
	f.chunks.mu.Lock()
	roots := len(f.chunks.roots)
	f.chunks.mu.Unlock()
	if roots != 0 || len(f.tape.events) != 0 {
		t.Fatalf("a failed attempt left %d roots and %d events", roots, len(f.tape.events))
	}
	assertWALReleased(t, f)
}

// failingChunks, failingRoots and failingAppender fail one step of a capture.
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

// blockingChunks never finishes a PutChunk until the capture gives up on it:
// an uploader that has stalled.
type blockingChunks struct {
	*memChunks
	entered atomic.Int32
}

func (b *blockingChunks) HasChunk(context.Context, string) (bool, error) { return false, nil }

func (b *blockingChunks) PutChunk(ctx context.Context, digest string, sealed []byte) error {
	b.entered.Add(1)
	<-ctx.Done()
	return ctx.Err()
}

// O6: the read-released hook runs once on every path, the attempt leaves nothing
// and the WAL is released.
func TestReadReleasedFiresOnceOnEveryPath(t *testing.T) {
	type tc struct {
		name  string
		setup func(t *testing.T, f *fixture, cancel context.CancelFunc)
		want  error // nil: success
		any   bool  // any error
	}
	for _, c := range []tc{
		{name: "success", setup: func(*testing.T, *fixture, context.CancelFunc) {}},
		{name: "head read failure", any: true, setup: func(t *testing.T, f *fixture, _ context.CancelFunc) {
			f.cfg.Head = func(context.Context, *sql.Tx) (*computerevent.Head, error) { return nil, errors.New("head failed") }
		}},
		{name: "cancel during the stream", any: true, setup: func(t *testing.T, f *fixture, cancel context.CancelFunc) {
			f.cfg.pageHook = func(pgno int64) {
				if pgno == 3 {
					cancel()
				}
			}
		}},
		{name: "time budget during the stream", want: ErrTimeBudget, setup: func(t *testing.T, f *fixture, _ context.CancelFunc) {
			f.cfg.MaxDuration = 100 * time.Millisecond
			f.cfg.pageHook = func(pgno int64) {
				if pgno == 3 {
					time.Sleep(300 * time.Millisecond)
				}
			}
		}},
		{name: "pin deadline during the stream", want: ErrPinDeadline, setup: func(t *testing.T, f *fixture, _ context.CancelFunc) {
			f.cfg.PinDeadline = 100 * time.Millisecond
			f.cfg.pageHook = func(pgno int64) {
				if pgno == 3 {
					time.Sleep(300 * time.Millisecond)
				}
			}
		}},
		{name: "upload failure", any: true, setup: func(t *testing.T, f *fixture, _ context.CancelFunc) {
			f.cfg.Chunks = failingChunks{ChunkStore: f.chunks, err: errors.New("put failed")}
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newCopyFixture(t, ChunkSize/2)
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
			c.setup(t, f, cancel)
			_, err := TakeSnapshot(ctx, f.cfg)
			switch {
			case c.want != nil:
				if !errors.Is(err, c.want) {
					t.Fatalf("err = %v, want %v", err, c.want)
				}
			case c.any:
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
			if err != nil {
				nothingRecorded(t, f)
			} else {
				assertWALReleased(t, f)
			}
		})
	}
}

// A hook that blocks is ended by the capture's own duration and never fails the
// attempt; with no MaxDuration the hook has no deadline.
func TestABlockedReadReleasedHookIsBoundedByMaxDuration(t *testing.T) {
	f := newCopyFixture(t, ChunkSize/4)
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

	g := newCopyFixture(t, ChunkSize/4)
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

// O6: a failure of any later step leaves no event, and a failed root
// registration or append leaves the transaction released.
func TestAFailedStepLeavesNoEvent(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(f *fixture, cancel context.CancelFunc)
	}{
		{"register error", func(f *fixture, _ context.CancelFunc) { f.cfg.Chunks = failingRoots{f.chunks} }},
		{"append error", func(f *fixture, _ context.CancelFunc) { f.cfg.Appender = failingAppender{f.tape} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCopyFixture(t, ChunkSize/2)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			tc.setup(f, cancel)
			if _, err := TakeSnapshot(ctx, f.cfg); err == nil {
				t.Fatal("expected an error")
			}
			if len(f.tape.events) != 0 {
				t.Fatalf("a failed step left %d events", len(f.tape.events))
			}
			assertWALReleased(t, f)
		})
	}
}

// O3: a slow reader hits the pin deadline. The attempt fails with
// ErrPinDeadline and nothing else, the WAL is released, no root or event is
// left, the chunks uploaded before the deadline stay, and a retry finds them
// held and uploads only the rest.
func TestThePinDeadlineReleasesTheTransactionAndTheRetrySkipsWhatWasUploaded(t *testing.T) {
	f := newFixture(t, ChunkSize*3)
	var released atomic.Int32
	f.cfg.ReadReleased = func(context.Context) { released.Add(1) }
	f.cfg.PinDeadline = 400 * time.Millisecond
	f.cfg.pageHook = func(pgno int64) {
		if pgno > 1100 { // the first chunk is cut at page 1024
			time.Sleep(20 * time.Millisecond)
		}
	}
	started := time.Now()
	_, err := TakeSnapshot(context.Background(), f.cfg)
	if !errors.Is(err, ErrPinDeadline) {
		t.Fatalf("err = %v, want ErrPinDeadline", err)
	}
	if errors.Is(err, ErrTimeBudget) {
		t.Fatalf("a pin deadline was reported as the time budget: %v", err)
	}
	if took := time.Since(started); took > 10*time.Second {
		t.Fatalf("the capture ran %s past a 400ms pin deadline", took)
	}
	if released.Load() != 1 {
		t.Fatalf("ReadReleased ran %d times", released.Load())
	}
	nothingRecorded(t, f) // includes the WAL being truncatable: the pin is gone
	kept := len(f.chunks.chunks)
	if kept == 0 {
		t.Fatal("test premise: no chunk was uploaded before the deadline")
	}

	f.cfg.PinDeadline, f.cfg.pageHook = 0, nil
	ref, err := TakeSnapshot(context.Background(), f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if ref.Stats.ChunksHeld < kept || ref.Stats.ChunksUploaded+ref.Stats.ChunksHeld+ref.Stats.ChunksRepeated != ref.Stats.Chunks {
		t.Fatalf("the retry found %d of the %d kept chunks held: %+v", ref.Stats.ChunksHeld, kept, ref.Stats)
	}
}

// O3: the case the deadline exists for. The uploader stalls, backpressure holds
// the reader, and only the deadline ends the transaction.
func TestAStalledUploadEndsAtThePinDeadline(t *testing.T) {
	f := newFixture(t, ChunkSize*14)
	blocking := &blockingChunks{memChunks: f.chunks}
	f.cfg.Chunks = blocking
	f.cfg.PinDeadline = 500 * time.Millisecond
	started := time.Now()
	_, err := TakeSnapshot(context.Background(), f.cfg)
	if !errors.Is(err, ErrPinDeadline) {
		t.Fatalf("err = %v, want ErrPinDeadline", err)
	}
	if took := time.Since(started); took > 15*time.Second {
		t.Fatalf("a stalled upload held the capture for %s past its 500ms pin deadline", took)
	}
	if blocking.entered.Load() == 0 {
		t.Fatal("test premise: the uploader never started")
	}
	nothingRecorded(t, f)
}

// O4: no file the size of the store is written during the capture: not beside
// the store, not in a snapshot-tmp directory, not in the temporary directory.
func TestTheCaptureWritesNoCopyOfTheStore(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	f := newFixture(t, ChunkSize*3)
	var storePath string
	if err := f.db.Read.QueryRow(`SELECT file FROM pragma_database_list WHERE name = 'main'`).Scan(&storePath); err != nil || storePath == "" {
		t.Fatalf("store path %q: %v", storePath, err)
	}
	storeDir := filepath.Dir(storePath)
	isStore := func(p string) bool {
		base := filepath.Base(p)
		return base == filepath.Base(storePath) || base == filepath.Base(storePath)+"-wal" || base == filepath.Base(storePath)+"-shm"
	}
	var storeSize int64
	if info, err := os.Stat(storePath); err == nil {
		storeSize = info.Size()
	}
	if storeSize < ChunkSize {
		t.Fatalf("test premise: the store is %d bytes", storeSize)
	}
	var biggest atomic.Int64
	var samples atomic.Int32
	var extra atomic.Value
	sample := func() {
		samples.Add(1)
		for _, root := range []string{storeDir, tmp, filepath.Join(storeDir, "snapshot-tmp")} {
			_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() || isStore(p) {
					return nil
				}
				if info, err := d.Info(); err == nil && info.Size() > biggest.Load() {
					biggest.Store(info.Size())
					extra.Store(p)
				}
				return nil
			})
		}
	}
	f.cfg.pageHook = func(pgno int64) {
		if pgno%128 == 1 {
			sample()
		}
	}
	if _, err := TakeSnapshot(context.Background(), f.cfg); err != nil {
		t.Fatal(err)
	}
	sample()
	if samples.Load() < 10 {
		t.Fatalf("test premise: only %d samples", samples.Load())
	}
	// No other file in the store's directory or the temporary one reaches even a
	// tenth of the store; a scratch copy would be the store's size.
	if got := biggest.Load(); got > storeSize/10 {
		p, _ := extra.Load().(string)
		t.Fatalf("a file of %d bytes (%s) was written during the capture of a %d-byte store", got, p, storeSize)
	}
	if _, err := os.Stat(filepath.Join(storeDir, "snapshot-tmp")); !os.IsNotExist(err) {
		t.Fatalf("a snapshot-tmp directory exists after the capture: %v", err)
	}
}

// O1, O2: the writer commits throughout the capture; the snapshot is exactly
// the state at the head it recorded (the restore's own head tie checks the
// restored file's head row; the row count is checked here), and the recorded
// digest is the digest of the pages that were uploaded.
func TestACaptureTakenWhileWritesContinueRestoresToExactlyItsHead(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, ChunkSize*3)
	headsFixture(t, f)
	var stop atomic.Bool
	var wg sync.WaitGroup
	var committed atomic.Int64
	wg.Add(1)
	go func() {
		defer wg.Done()
		for id := 300000; !stop.Load(); id++ {
			if err := commitRow(f.db, id); err != nil {
				t.Errorf("commit during the capture: %v", err)
				return
			}
			committed.Add(1)
			time.Sleep(time.Millisecond)
		}
	}()
	f.cfg.pageHook = func(pgno int64) {
		if pgno%64 == 0 {
			time.Sleep(time.Millisecond)
		}
	}
	ref, err := TakeSnapshot(ctx, f.cfg)
	stop.Store(true)
	wg.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if committed.Load() < 5 {
		t.Fatalf("test premise: only %d commits during the capture", committed.Load())
	}
	var rowsNow uint64
	if err := f.db.Read.QueryRow(`SELECT COUNT(*) FROM blobs`).Scan(&rowsNow); err != nil {
		t.Fatal(err)
	}
	if ref.Record.StoreHeadSequence >= rowsNow {
		t.Fatalf("test premise: the record head %d is not behind the live store (%d rows)", ref.Record.StoreHeadSequence, rowsNow)
	}
	assertRestoredHeadAgrees(t, f, ref)

	// The digest is that of the chunks as uploaded, opened and concatenated.
	dek, err := f.cipher.ExportKeyForEscrow(ctx, testComputer)
	if err != nil {
		t.Fatal(err)
	}
	d := NewPageDigest(ref.Record.PageSize, ref.Record.PageCount)
	for _, digest := range ref.Record.Manifest.Files[0].Chunks {
		sealed, err := f.chunks.GetChunk(ctx, digest)
		if err != nil {
			t.Fatal(err)
		}
		plain, err := filecas.OpenChunk(dek, testComputer, digest, sealed)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = d.Write(plain)
	}
	if got := d.Sum(); got != ref.Record.PageDigest {
		t.Fatalf("the record names digest %s, the uploaded chunks digest to %s", ref.Record.PageDigest, got)
	}
}

// O2: a stream that is torn or truncated after the capture (a chunk missing,
// half a chunk, a zeroed tail, a middle chunk of other bytes) is refused by the
// restore against a real capture's record, and no file is left.
func TestATornOrTruncatedStreamIsRefusedByRestore(t *testing.T) {
	c := newV2Case(t)
	ps := c.base.PageSize
	t.Run("the last chunk missing", func(t *testing.T) {
		c.only(t, c.v2(t, c.plain[:(len(c.plain)-1)/ChunkSize*ChunkSize], c.plain))
		c.refused(t, "manifest size does not match the record")
	})
	t.Run("half of the last chunk missing", func(t *testing.T) {
		keep := (len(c.plain)-1)/ChunkSize*ChunkSize + (len(c.plain)%ChunkSize)/2/ps*ps
		c.only(t, c.v2(t, c.plain[:keep], c.plain))
		c.refused(t, "manifest size does not match the record")
	})
	t.Run("a zeroed tail of the right length", func(t *testing.T) {
		torn := bytes.Clone(c.plain)
		clear(torn[len(torn)-4*ps:])
		c.only(t, c.v2(t, torn, c.plain))
		c.refused(t, "page digest")
	})
	t.Run("a middle chunk of other bytes", func(t *testing.T) {
		torn := bytes.Clone(c.plain)
		_, _ = rand.Read(torn[ChunkSize : 2*ChunkSize])
		c.only(t, c.v2(t, torn, c.plain))
		c.refused(t, "page digest")
	})
	t.Run("the untouched capture restores", func(t *testing.T) {
		c.only(t, c.v2(t, c.plain, c.plain))
		if _, _, calls, err := c.restore(t); err != nil || calls != 1 {
			t.Fatalf("restore = %v, verify calls %d", err, calls)
		}
	})
}

// O5: free space is checked while the stream runs, every sourceDropBytes read,
// and not only by the once-a-minute guard.
func TestFreeSpaceIsCheckedAsThePagesAreRead(t *testing.T) {
	prev := sourceDropBytes
	sourceDropBytes = 1 << 20
	defer func() { sourceDropBytes = prev }()
	f := newFixture(t, ChunkSize*3)
	f.cfg.FreeSpaceEvery = time.Hour // the minute guard cannot be what ends this
	var calls atomic.Int32
	f.cfg.FreeSpace = func() error {
		if calls.Add(1) >= 4 {
			return errTestLowDisk
		}
		return nil
	}
	_, err := TakeSnapshot(context.Background(), f.cfg)
	if !errors.Is(err, errTestLowDisk) {
		t.Fatalf("err = %v, want the free-space cause", err)
	}
	if calls.Load() != 4 {
		t.Fatalf("FreeSpace ran %d times, want 4 (the stream stops at the first failure)", calls.Load())
	}
	nothingRecorded(t, f)
}

// O7: an older capture's version 1 record restores and is verified as before.
func TestAVersionOneRecordStillRestores(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, ChunkSize*2)
	ref, err := TakeSnapshot(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	v1 := f.legacyV1(t, ref)
	cfg := f.restoreConfig(filepath.Join(t.TempDir(), "state.db"))
	var verified Record
	cfg.Verify = func(_ context.Context, rec Record, _ string) error { verified = rec; return nil }
	rec, err := Restore(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Version != Version || rec.WitnessRoot != v1.WitnessRoot || verified.WitnessRoot != v1.WitnessRoot {
		t.Fatalf("restored %+v, verified %+v", rec, verified)
	}
}

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
	for _, stmt := range []string{copyHeadSchema, `INSERT INTO heads VALUES ('c', 41, 'canon', 'desired', 'effective', NULL, 'dsc', 'esc', 1, 3)`} {
		if _, err := db.Write.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	cipher := testCipher(t, testComputer)
	tape := &fakeTape{t: t, cipher: cipher, payloads: map[string][]byte{}}
	chunks := newMemChunks()
	return &fixture{db: db, tape: tape, chunks: chunks, cipher: cipher, cfg: Config{
		ComputerID: testComputer, DB: db.Read, Cipher: cipher, Chunks: chunks, Appender: tape,
		Head: func(ctx context.Context, tx *sql.Tx) (*computerevent.Head, error) { return headFromTable(ctx, tx) },
	}}
}

// roundTripDigest captures, restores through the install's own digest check and
// head tie, and compares the restored store's logical witness with the live
// store's.
func roundTripDigest(t *testing.T, f *fixture) {
	t.Helper()
	ctx := context.Background()
	live, err := witnessOf(ctx, f.db.Read)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := TakeSnapshot(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if ref.Record.Version != VersionPageDigest || ref.Record.PageDigest == "" {
		t.Fatalf("record %+v", ref.Record)
	}
	dest := filepath.Join(t.TempDir(), "state.db")
	if _, err := Restore(ctx, f.restoreConfig(dest)); err != nil {
		t.Fatal(err)
	}
	restored, err := dbx.OpenSQLite(dest, dbx.SQLiteOptions{ReadConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	got, err := witnessOf(ctx, restored.Read)
	if err != nil {
		t.Fatal(err)
	}
	if got.ContentRoot != live.ContentRoot {
		t.Fatalf("restored witness %s, live %s", got.ContentRoot, live.ContentRoot)
	}
}

// O8
func TestOtherPageSizesStreamAndRestore(t *testing.T) {
	for _, ps := range []int{512, 8192, 65536} {
		t.Run(fmt.Sprint(ps), func(t *testing.T) {
			f := pagedFixture(t, ps, false, 40, 60<<10)
			var size int
			if err := f.db.Read.QueryRow(`PRAGMA page_size`).Scan(&size); err != nil || size != ps {
				t.Fatalf("page size %d (%v), want %d", size, err, ps)
			}
			roundTripDigest(t, f)
		})
	}
}

// O8: an auto_vacuum=INCREMENTAL store whose file shrank by incremental_vacuum.
func TestAnIncrementallyVacuumedStoreStreamsAndRestores(t *testing.T) {
	f := pagedFixture(t, 4096, true, 60, 60<<10)
	if _, err := f.db.Write.Exec(`DELETE FROM blobs WHERE id % 2 = 0`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Write.Exec(`PRAGMA incremental_vacuum(100000)`); err != nil {
		t.Fatal(err)
	}
	roundTripDigest(t, f)
}

// O8, the lock-byte page: a store past 1 GiB. Skipped under -short: it writes
// about 1.1 GiB.
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
	roundTripDigest(t, f)
}
