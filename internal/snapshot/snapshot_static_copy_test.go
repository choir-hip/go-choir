package snapshot

// Tests of the static-copy capture, written from the failure list of
// docs/design/capture-static-copy-2026-11-15.md section 3 before the code. The
// comment above each names the failure entry it pins.

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/filecas"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
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

// newCopyFixture is newFixture with the design's configuration: a scratch
// directory, the witness on the copy's DB, and a head read from a ten-column
// table, so a copy can be tampered with in any column.
func newCopyFixture(t *testing.T, bytesApprox int) *fixture {
	t.Helper()
	f := newFixture(t, bytesApprox)
	for _, stmt := range []string{
		`CREATE TABLE heads (computer_id TEXT PRIMARY KEY, sequence INTEGER NOT NULL, canonical_event_head TEXT NOT NULL, desired_event_head TEXT NOT NULL, effective_event_head TEXT NOT NULL, pending_transition_ref TEXT, desired_state_commitment TEXT NOT NULL, effective_state_commitment TEXT NOT NULL, reducer_version INTEGER NOT NULL, credential_revocation_epoch INTEGER NOT NULL)`,
		`INSERT INTO heads VALUES ('c', 41, 'canon', 'desired', 'effective', NULL, 'dsc', 'esc', 1, 3)`,
	} {
		if _, err := f.db.Write.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	f.cfg.ScratchDir = filepath.Join(t.TempDir(), "snapshot-tmp")
	f.cfg.Witness = witnessOf
	f.cfg.Head = func(ctx context.Context, tx *sql.Tx) (*computerevent.Head, error) { return headFromTable(ctx, tx) }
	return f
}

// rewriteCopy opens the scratch copy read/write and runs stmt: a tamper.
func rewriteCopy(t *testing.T, path, stmt string) {
	t.Helper()
	db, err := dbx.OpenSQLite(path, dbx.SQLiteOptions{ReadConns: 1})
	if err != nil {
		t.Fatalf("open the copy to tamper with it: %v", err)
	}
	if _, err := db.Write.Exec(stmt); err != nil {
		t.Fatalf("tamper %q: %v", stmt, err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
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
	if _, err := os.Stat(f.cfg.ScratchDir); !os.IsNotExist(err) {
		t.Fatalf("the scratch directory is still there after a failed attempt (stat error %v)", err)
	}
	assertWALReleased(t, f)
}

// Failure 2: every column of the head, not only the sequence.
func TestACopyWithADifferentHeadColumnIsRefused(t *testing.T) {
	for _, tc := range []struct{ column, value string }{
		{"computer_id", "'other'"}, {"sequence", "42"}, {"canonical_event_head", "'x'"},
		{"desired_event_head", "'x'"}, {"effective_event_head", "'x'"}, {"pending_transition_ref", "'x'"},
		{"desired_state_commitment", "'x'"}, {"effective_state_commitment", "'x'"},
		{"reducer_version", "2"}, {"credential_revocation_epoch", "4"},
	} {
		t.Run(tc.column, func(t *testing.T) {
			f := newCopyFixture(t, ChunkSize/4)
			f.cfg.afterCopy = func(path string) {
				rewriteCopy(t, path, fmt.Sprintf(`UPDATE heads SET %s = %s`, tc.column, tc.value))
			}
			_, err := TakeSnapshot(context.Background(), f.cfg)
			if !errors.Is(err, ErrCopyHeadMismatch) {
				t.Fatalf("err = %v, want ErrCopyHeadMismatch for a changed %s", err, tc.column)
			}
			nothingRecorded(t, f)
		})
	}
}

// Failure 3: a copy that is short or long is refused.
func TestACopyOfAnotherSizeIsRefused(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(t *testing.T, path string)
	}{
		{"short", func(t *testing.T, path string) {
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Truncate(path, info.Size()-4096); err != nil {
				t.Fatal(err)
			}
		}},
		{"long", func(t *testing.T, path string) {
			fh, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer fh.Close()
			if _, err := fh.Write(make([]byte, 4096)); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCopyFixture(t, ChunkSize/4)
			f.cfg.afterCopy = func(path string) { tc.mutate(t, filepath.Join(path)) }
			if _, err := TakeSnapshot(context.Background(), f.cfg); err == nil {
				t.Fatal("a copy of another size was recorded")
			}
			nothingRecorded(t, f)
		})
	}
}

// Failure 11: nil equals nil and nothing else.
func TestANilHeadIsEqualOnlyToANilHead(t *testing.T) {
	f := newCopyFixture(t, ChunkSize/4)
	f.cfg.Head = func(context.Context, *sql.Tx) (*computerevent.Head, error) { return nil, nil }
	ref, err := TakeSnapshot(context.Background(), f.cfg)
	if err != nil {
		t.Fatalf("a store with no head failed: %v", err)
	}
	if ref.Record.StoreHeadSequence != 0 || ref.Record.StoreHead != "" {
		t.Fatalf("a nil head recorded %d %q", ref.Record.StoreHeadSequence, ref.Record.StoreHead)
	}

	g := newCopyFixture(t, ChunkSize/4)
	var calls atomic.Int32
	g.cfg.Head = func(ctx context.Context, tx *sql.Tx) (*computerevent.Head, error) {
		if calls.Add(1) == 1 {
			return nil, nil
		}
		return headFromTable(ctx, tx)
	}
	if _, err := TakeSnapshot(context.Background(), g.cfg); !errors.Is(err, ErrCopyHeadMismatch) {
		t.Fatalf("nil in the transaction and a head in the copy: err = %v, want ErrCopyHeadMismatch", err)
	}
	nothingRecorded(t, g)
}

// Failure 5: a byte changed after the copy, or after the witness, is caught by
// the digest before the root is registered.
func TestAByteChangedAfterTheCopyOrTheWitnessIsRefused(t *testing.T) {
	// flip changes one byte inside a row's blob data (found by content), a byte no
	// b-tree page and no cross-check reads, so only the digest can see it.
	flip := func(t *testing.T, f *fixture, path string) {
		var body []byte
		if err := f.db.Read.QueryRow(`SELECT body FROM blobs WHERE id = 3`).Scan(&body); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		at := bytes.Index(raw, body[2048:2112])
		if at < 0 {
			t.Fatal("the row's data is not contiguous in the copy")
		}
		fh, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer fh.Close()
		b := []byte{raw[at+10] ^ 0xff}
		if _, err := fh.WriteAt(b, int64(at+10)); err != nil {
			t.Fatal(err)
		}
	}
	for _, phase := range []string{"after the copy", "after the witness"} {
		t.Run(phase, func(t *testing.T) {
			f := newCopyFixture(t, ChunkSize/2)
			if phase == "after the copy" {
				f.cfg.afterCopy = func(path string) { flip(t, f, path) }
			} else {
				f.cfg.afterWitness = func(path string) { flip(t, f, path) }
			}
			if _, err := TakeSnapshot(context.Background(), f.cfg); !errors.Is(err, ErrCopyDigest) {
				t.Fatalf("err = %v, want ErrCopyDigest", err)
			}
			// The digest is compared at the end of the upload, so chunks may have
			// been put; none may be registered or named by an event.
			f.chunks.mu.Lock()
			roots := len(f.chunks.roots)
			f.chunks.mu.Unlock()
			if roots != 0 || len(f.tape.events) != 0 {
				t.Fatalf("a changed copy registered %d roots and %d events", roots, len(f.tape.events))
			}
			if _, err := os.Stat(f.cfg.ScratchDir); !os.IsNotExist(err) {
				t.Fatalf("scratch left: %v", err)
			}
			assertWALReleased(t, f)
		})
	}
}

// pinned reports whether a reader holds the WAL back: after a commit, a PASSIVE
// checkpoint cannot backfill every frame.
func pinned(t *testing.T, f *fixture) bool {
	t.Helper()
	if _, err := f.db.Write.Exec(`UPDATE blobs SET note = note || '.' WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	var busy, logFrames, done int
	if err := f.db.Write.QueryRow(`PRAGMA wal_checkpoint(PASSIVE)`).Scan(&busy, &logFrames, &done); err != nil {
		t.Fatal(err)
	}
	return done < logFrames
}

// Failures 12 and 15: the pin is the copy and nothing after it, observed from
// outside by order (a PASSIVE checkpoint behind a write), and writes proceed
// during every phase.
func TestThePinEndsBeforeTheWitness(t *testing.T) {
	f := newCopyFixture(t, ChunkSize/2)
	var released atomic.Bool
	var duringCopy, duringWitness atomic.Int32 // 1 pinned, 2 not
	mark := func(v *atomic.Int32, pin bool) {
		if pin {
			v.Store(1)
		} else {
			v.Store(2)
		}
	}
	f.cfg.pageHook = func(pgno int64) {
		if pgno == 3 {
			mark(&duringCopy, pinned(t, f))
		}
	}
	f.cfg.ReadReleased = func(context.Context) { released.Store(true) }
	probeStop := make(chan struct{})
	var probeWG sync.WaitGroup
	var pinnedAfterRelease atomic.Bool
	probeWG.Add(1)
	go func() {
		defer probeWG.Done()
		for {
			select {
			case <-probeStop:
				return
			case <-time.After(time.Millisecond):
			}
			wasReleased := released.Load()
			p := pinned(t, f)
			if wasReleased && p {
				pinnedAfterRelease.Store(true)
			}
		}
	}()
	f.cfg.Witness = func(ctx context.Context, db *sql.DB) (selfdevprotocol.VMLocalContentWitness, error) {
		// A write commits during the witness (failure 15) and nothing pins the WAL.
		mark(&duringWitness, pinned(t, f))
		return witnessOf(ctx, db)
	}
	ref, err := TakeSnapshot(context.Background(), f.cfg)
	close(probeStop)
	probeWG.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if duringCopy.Load() != 1 {
		t.Fatalf("no reader seen during the copy (%d): the probe cannot see a pin", duringCopy.Load())
	}
	if duringWitness.Load() != 2 {
		t.Fatalf("a reader of the capture is still open during the witness (%d)", duringWitness.Load())
	}
	if pinnedAfterRelease.Load() {
		t.Fatal("a reader was seen after ReadReleased fired")
	}
	st := ref.Stats
	if st.CopyTime <= 0 || st.PinTime < st.CopyTime {
		t.Fatalf("PinTime %s, CopyTime %s: the pin must include the copy", st.PinTime, st.CopyTime)
	}
	if st.ScratchBytes != ref.Record.PlaintextSize || st.ScratchBytes == 0 {
		t.Fatalf("ScratchBytes %d, plaintext %d", st.ScratchBytes, ref.Record.PlaintextSize)
	}
	if st.WitnessTime <= 0 || st.UploadTime <= 0 {
		t.Fatalf("witness %s upload %s", st.WitnessTime, st.UploadTime)
	}
}

// Failure 13 (and 25, 32, 24): the read-released hook fires exactly once on
// every path, with a context the caller's cancellation cannot end.
func TestReadReleasedFiresOnceOnEveryPath(t *testing.T) {
	type tc struct {
		name  string
		setup func(t *testing.T, f *fixture, cancel context.CancelFunc)
		want  error // nil: success
		any   bool  // any error
	}
	tamper := func(t *testing.T, f *fixture, _ context.CancelFunc) {
		f.cfg.afterCopy = func(path string) { rewriteCopy(t, path, `UPDATE heads SET sequence = 9`) }
	}
	for _, c := range []tc{
		{name: "success", setup: func(*testing.T, *fixture, context.CancelFunc) {}},
		{name: "head read failure", any: true, setup: func(t *testing.T, f *fixture, _ context.CancelFunc) {
			f.cfg.Head = func(context.Context, *sql.Tx) (*computerevent.Head, error) { return nil, errors.New("head failed") }
		}},
		{name: "cancel during the copy", any: true, setup: func(t *testing.T, f *fixture, cancel context.CancelFunc) {
			f.cfg.pageHook = func(pgno int64) {
				if pgno == 3 {
					cancel()
				}
			}
		}},
		{name: "time budget during the copy", want: ErrTimeBudget, setup: func(t *testing.T, f *fixture, _ context.CancelFunc) {
			f.cfg.MaxDuration = 100 * time.Millisecond
			f.cfg.pageHook = func(pgno int64) {
				if pgno == 3 {
					time.Sleep(300 * time.Millisecond)
				}
			}
		}},
		{name: "ENOSPC on write", want: syscall.ENOSPC, setup: func(t *testing.T, f *fixture, _ context.CancelFunc) {
			f.cfg.openScratch = func(path string) (scratchWriter, error) {
				fh, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
				return &faultyWriter{File: fh, failAfter: 8192, err: syscall.ENOSPC}, err
			}
		}},
		{name: "EIO on sync", want: syscall.EIO, setup: func(t *testing.T, f *fixture, _ context.CancelFunc) {
			f.cfg.openScratch = func(path string) (scratchWriter, error) {
				fh, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
				return &faultyWriter{File: fh, syncErr: syscall.EIO}, err
			}
		}},
		{name: "cross-check failure", want: ErrCopyHeadMismatch, setup: tamper},
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
			var removed atomic.Int32
			f.cfg.ScratchRemoved = func() { removed.Add(1) }
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
			if got := removed.Load(); got != 1 {
				t.Fatalf("ScratchRemoved ran %d times, want exactly once", got)
			}
			if _, statErr := os.Stat(f.cfg.ScratchDir); !os.IsNotExist(statErr) {
				t.Fatalf("scratch still present: %v", statErr)
			}
			assertWALReleased(t, f)
		})
	}
}

// faultyWriter is a scratchWriter over a file that fails on demand and records
// whether Sync ever ran with bytes not yet written to it.
type faultyWriter struct {
	*os.File
	written   int64
	failAfter int64
	err       error
	syncErr   error
	syncs     []int64
}

func (w *faultyWriter) Write(p []byte) (int, error) {
	if w.failAfter > 0 && w.written+int64(len(p)) > w.failAfter {
		return 0, w.err
	}
	n, err := w.File.Write(p)
	w.written += int64(n)
	return n, err
}

func (w *faultyWriter) Sync() error {
	w.syncs = append(w.syncs, w.written)
	if w.syncErr != nil {
		return w.syncErr
	}
	return w.File.Sync()
}

// Failure 14: the witness does not wait for the hook, and the capture joins it.
func TestTheWitnessDoesNotWaitForTheReadReleasedHook(t *testing.T) {
	f := newCopyFixture(t, ChunkSize/4)
	witnessStarted := make(chan struct{})
	var hookDone atomic.Bool
	var witnessStartedFirst atomic.Bool
	f.cfg.ReadReleased = func(context.Context) {
		select {
		case <-witnessStarted:
			witnessStartedFirst.Store(true)
		case <-time.After(10 * time.Second):
		}
		hookDone.Store(true)
	}
	f.cfg.Witness = func(ctx context.Context, db *sql.DB) (selfdevprotocol.VMLocalContentWitness, error) {
		close(witnessStarted)
		return witnessOf(ctx, db)
	}
	if _, err := TakeSnapshot(context.Background(), f.cfg); err != nil {
		t.Fatal(err)
	}
	if !witnessStartedFirst.Load() {
		t.Fatal("the witness waited for the read-released hook")
	}
	if !hookDone.Load() {
		t.Fatal("TakeSnapshot returned before the hook did: the capture must join it")
	}
}

// Failure 40 and the MaxDuration == 0 rule: a hook that blocks is ended by the
// capture's own duration and never fails the attempt; with no MaxDuration the
// hook has no deadline.
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
		t.Fatal("with MaxDuration zero the hook was given a deadline (an expired one)")
	}
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

// Failures 21, 23 (scratch part), 27, 32: the scratch is removed, with its
// sidecars, on success and on every error path, and the hook fires once.
func TestTheScratchIsRemovedOnEveryPath(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(f *fixture, cancel context.CancelFunc)
		fail  bool
	}{
		{"success", func(*fixture, context.CancelFunc) {}, false},
		{"witness error", func(f *fixture, _ context.CancelFunc) {
			f.cfg.Witness = func(context.Context, *sql.DB) (selfdevprotocol.VMLocalContentWitness, error) {
				return selfdevprotocol.VMLocalContentWitness{}, errors.New("witness failed")
			}
		}, true},
		{"upload error", func(f *fixture, _ context.CancelFunc) {
			f.cfg.Chunks = failingChunks{ChunkStore: f.chunks, err: errors.New("put failed")}
		}, true},
		{"register error", func(f *fixture, _ context.CancelFunc) { f.cfg.Chunks = failingRoots{f.chunks} }, true},
		{"append error", func(f *fixture, _ context.CancelFunc) { f.cfg.Appender = failingAppender{f.tape} }, true},
		{"cancellation", func(f *fixture, cancel context.CancelFunc) {
			f.cfg.Witness = func(ctx context.Context, db *sql.DB) (selfdevprotocol.VMLocalContentWitness, error) {
				cancel()
				<-ctx.Done()
				return selfdevprotocol.VMLocalContentWitness{}, ctx.Err()
			}
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCopyFixture(t, ChunkSize/2)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var removed atomic.Int32
			f.cfg.ScratchRemoved = func() { removed.Add(1) }
			tc.setup(f, cancel)
			_, err := TakeSnapshot(ctx, f.cfg)
			if (err != nil) != tc.fail {
				t.Fatalf("err = %v, want failure %v", err, tc.fail)
			}
			if removed.Load() != 1 {
				t.Fatalf("ScratchRemoved ran %d times", removed.Load())
			}
			for _, name := range []string{"", "state.db", "state.db-wal", "state.db-shm"} {
				if _, statErr := os.Stat(filepath.Join(f.cfg.ScratchDir, name)); !os.IsNotExist(statErr) {
					t.Fatalf("%q survived: %v", name, statErr)
				}
			}
		})
	}
}

// Failure 27: the scratch is gone before the partitions are read (it goes right
// after the state upload).
func TestTheScratchIsRemovedBeforeThePartitionsAreRead(t *testing.T) {
	f := newCopyFixture(t, ChunkSize/4)
	live := t.TempDir()
	part := partitionDB(t, live, "tool_result-20261001.db", 3000)
	f.cfg.Partitions = &PartitionSource{
		Hold: func() func() { return func() {} },
		List: func(context.Context, *sql.Tx) ([]PartitionFile, error) {
			return []PartitionFile{{Path: "telemetry/tool_result-20261001.db", DB: part.Read}}, nil
		},
	}
	var mu sync.Mutex
	var order []string
	f.cfg.ScratchRemoved = func() { mu.Lock(); order = append(order, "removed"); mu.Unlock() }
	f.cfg.Chunks = &orderChunks{memChunks: f.chunks, mu: &mu, order: &order}
	if _, err := TakeSnapshot(context.Background(), f.cfg); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	removedAt, lastPut := -1, -1
	for i, e := range order {
		if e == "removed" && removedAt < 0 {
			removedAt = i
		}
		if e == "put" {
			lastPut = i
		}
	}
	if removedAt < 0 || lastPut < removedAt {
		t.Fatalf("scratch removal at %d, last chunk put at %d: the partition chunks must be put after the scratch is gone (%v)", removedAt, lastPut, order)
	}
}

type orderChunks struct {
	*memChunks
	mu    *sync.Mutex
	order *[]string
}

func (c *orderChunks) PutChunk(ctx context.Context, digest string, sealed []byte) error {
	c.mu.Lock()
	*c.order = append(*c.order, "put")
	c.mu.Unlock()
	return c.memChunks.PutChunk(ctx, digest, sealed)
}

// Failures 24 and 44: free space is checked every scratchSyncBytes, the first
// check fails the copy at its first boundary, and an error that is not a
// low-disk verdict (a statfs error) also fails closed.
func TestFreeSpaceIsCheckedEveryScratchSyncBytes(t *testing.T) {
	old := scratchSyncBytes
	scratchSyncBytes = 8 * 4096
	t.Cleanup(func() { scratchSyncBytes = old })
	errLow := errors.New("under the floor")
	for _, tc := range []struct {
		name string
		err  error
	}{{"low", errLow}, {"statfs error", errors.New("statfs failed")}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCopyFixture(t, ChunkSize/2)
			var calls atomic.Int32
			var pagesAtFirst atomic.Int64
			var pages atomic.Int64
			f.cfg.pageHook = func(pgno int64) { pages.Store(pgno) }
			f.cfg.FreeSpaceEvery = time.Hour // only the copy's own checks
			f.cfg.FreeSpace = func() error {
				if calls.Add(1) == 1 {
					pagesAtFirst.Store(pages.Load())
					return tc.err
				}
				return nil
			}
			_, err := TakeSnapshot(context.Background(), f.cfg)
			if !errors.Is(err, tc.err) {
				t.Fatalf("err = %v, want %v", err, tc.err)
			}
			if got := pagesAtFirst.Load(); got != 8 && got != 9 {
				t.Fatalf("the first check ran after %d pages, want at the first %d-byte boundary (8 pages)", got, scratchSyncBytes)
			}
			nothingRecorded(t, f)
		})
	}
}

// Failure 38: Sync never runs with bytes still in the buffer: at each Sync the
// writer has been handed every page read so far.
func TestSyncNeverRunsWithUnflushedBytes(t *testing.T) {
	old := scratchSyncBytes
	scratchSyncBytes = 8 * 4096
	t.Cleanup(func() { scratchSyncBytes = old })
	f := newCopyFixture(t, ChunkSize/2)
	var w *faultyWriter
	f.cfg.openScratch = func(path string) (scratchWriter, error) {
		fh, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		w = &faultyWriter{File: fh}
		return w, err
	}
	if _, err := TakeSnapshot(context.Background(), f.cfg); err != nil {
		t.Fatal(err)
	}
	if w == nil || len(w.syncs) < 2 {
		t.Fatalf("expected syncs at each boundary, got %v", w)
	}
	for i, written := range w.syncs[:len(w.syncs)-1] {
		if want := int64(i+1) * scratchSyncBytes; written != want {
			t.Fatalf("sync %d ran with %d bytes written, want %d: the buffer was not flushed first", i, written, want)
		}
	}
}

type slowHash struct {
	hash.Hash
	delay time.Duration
	calls *atomic.Int64
}

func (h slowHash) Write(p []byte) (int, error) {
	h.calls.Add(1)
	time.Sleep(h.delay)
	return h.Hash.Write(p)
}

// Failure 37: the hash is inline, so a slow hasher lengthens the pin by its own
// time.
func TestTheInlineHashLengthensThePin(t *testing.T) {
	f := newCopyFixture(t, ChunkSize/8)
	var calls atomic.Int64
	const delay = 2 * time.Millisecond
	f.cfg.newHash = func() hash.Hash { return slowHash{Hash: sha256.New(), delay: delay, calls: &calls} }
	ref, err := TakeSnapshot(context.Background(), f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() == 0 {
		t.Fatal("the injected hasher was never used")
	}
	if min := time.Duration(calls.Load()/2) * delay; ref.Stats.PinTime < min {
		t.Fatalf("PinTime %s is under the hasher's own time %s over %d writes", ref.Stats.PinTime, min, calls.Load())
	}
}

// Failure 23: an existing file at the scratch path fails the attempt closed.
func TestAnExistingScratchFileFailsTheAttemptClosed(t *testing.T) {
	f := newCopyFixture(t, ChunkSize/4)
	if err := os.MkdirAll(f.cfg.ScratchDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.cfg.ScratchDir, "state.db"), []byte("left behind"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := TakeSnapshot(context.Background(), f.cfg); err == nil {
		t.Fatal("a capture ran on top of an existing scratch file")
	}
	f.chunks.mu.Lock()
	defer f.chunks.mu.Unlock()
	if len(f.chunks.puts) != 0 || len(f.tape.events) != 0 {
		t.Fatal("a capture on an existing scratch file uploaded or recorded something")
	}
}

// Failures 30 and 47: MaxDuration ends the copy (scratch gone, pin released)
// and ends a witness with nothing uploaded or probed.
func TestMaxDurationEndsTheCopyAndTheWitnessWithNothingUploaded(t *testing.T) {
	t.Run("copy", func(t *testing.T) {
		f := newCopyFixture(t, ChunkSize/2)
		f.cfg.MaxDuration = 100 * time.Millisecond
		f.cfg.pageHook = func(pgno int64) {
			if pgno == 3 {
				time.Sleep(300 * time.Millisecond)
			}
		}
		if _, err := TakeSnapshot(context.Background(), f.cfg); !errors.Is(err, ErrTimeBudget) {
			t.Fatalf("err = %v, want ErrTimeBudget", err)
		}
		nothingRecorded(t, f)
	})
	t.Run("witness", func(t *testing.T) {
		f := newCopyFixture(t, ChunkSize/2)
		f.cfg.MaxDuration = 300 * time.Millisecond
		f.cfg.Witness = func(ctx context.Context, db *sql.DB) (selfdevprotocol.VMLocalContentWitness, error) {
			<-ctx.Done()
			return selfdevprotocol.VMLocalContentWitness{}, ctx.Err()
		}
		if _, err := TakeSnapshot(context.Background(), f.cfg); !errors.Is(err, ErrTimeBudget) {
			t.Fatalf("err = %v, want ErrTimeBudget", err)
		}
		f.chunks.mu.Lock()
		puts, probes := len(f.chunks.puts), f.chunks.probes
		f.chunks.mu.Unlock()
		if puts != 0 || probes != 0 {
			t.Fatalf("a witness that died in its budget uploaded %d chunks and probed %d", puts, probes)
		}
		nothingRecorded(t, f)
	})
}

// Failures 6 and 10: the base uploaded from the copy is the page sequence of the
// head cut by ChunkSize/pageSize (same digests as sealing the live pages), and
// it installs through the unchanged Restore with the witness check.
func TestTheUploadFromTheCopyIsTheLivePageSequence(t *testing.T) {
	ctx := context.Background()
	f := newCopyFixture(t, ChunkSize*2+ChunkSize/2)
	f.cfg.ScratchDir = filepath.Join(t.TempDir(), "snapshot-tmp")
	ref, err := TakeSnapshot(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if ref.Stats.ScratchBytes != ref.Record.PlaintextSize {
		t.Fatalf("scratch %d bytes, plaintext %d", ref.Stats.ScratchBytes, ref.Record.PlaintextSize)
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
	got := ref.Record.Manifest.Files[0].Chunks
	if fmt.Sprint(got) != fmt.Sprint(digests) {
		t.Fatalf("chunk digests differ from the live page sequence:\n got %v\nwant %v", got, digests)
	}
	dest := filepath.Join(t.TempDir(), "state.db")
	cfg := f.restoreConfig(dest)
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

// Failures 7, 8 and 45: after the witness the WAL beside the copy is empty, the
// copy opened read/write (as VerifySnapshotFile opens a base) leaves its bytes
// and its witness unchanged, and the copy's witness is the live one.
func TestTheCopyOpenedReadWriteIsUnchangedAndWitnessesAlike(t *testing.T) {
	f := newCopyFixture(t, ChunkSize/2)
	live, err := witnessOf(context.Background(), f.db.Read)
	if err != nil {
		t.Fatal(err)
	}
	var checked atomic.Bool
	f.cfg.afterWitness = func(path string) {
		if info, err := os.Stat(path + "-wal"); err == nil && info.Size() != 0 {
			t.Errorf("the copy's WAL holds %d bytes after the witness", info.Size())
		}
		before := sha256File(t, path)
		clone := filepath.Join(t.TempDir(), "clone.db")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Error(err)
			return
		}
		if err := os.WriteFile(clone, raw, 0o600); err != nil {
			t.Error(err)
			return
		}
		rw, err := dbx.OpenSQLite(clone, dbx.SQLiteOptions{ReadConns: 1})
		if err != nil {
			t.Error(err)
			return
		}
		w, err := witnessOf(context.Background(), rw.Read)
		_ = rw.Close()
		if err != nil || w.ContentRoot != live.ContentRoot {
			t.Errorf("read/write witness %v %v, live %s", w.ContentRoot, err, live.ContentRoot)
		}
		if after := sha256File(t, clone); after != sha256Bytes(raw) {
			t.Error("opening the clone read/write changed its main file")
		}
		if sha256File(t, path) != before {
			t.Error("the copy changed while the clone was checked")
		}
		checked.Store(true)
	}
	ref, err := TakeSnapshot(context.Background(), f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !checked.Load() {
		t.Fatal("the afterWitness hook never ran")
	}
	if ref.Record.WitnessRoot != live.ContentRoot {
		t.Fatalf("recorded witness %s, live %s", ref.Record.WitnessRoot, live.ContentRoot)
	}
}

func sha256Bytes(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }

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
	for _, stmt := range []string{
		`CREATE TABLE heads (computer_id TEXT PRIMARY KEY, sequence INTEGER NOT NULL, canonical_event_head TEXT NOT NULL, desired_event_head TEXT NOT NULL, effective_event_head TEXT NOT NULL, pending_transition_ref TEXT, desired_state_commitment TEXT NOT NULL, effective_state_commitment TEXT NOT NULL, reducer_version INTEGER NOT NULL, credential_revocation_epoch INTEGER NOT NULL)`,
		`INSERT INTO heads VALUES ('c', 41, 'canon', 'desired', 'effective', NULL, 'dsc', 'esc', 1, 3)`,
	} {
		if _, err := db.Write.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	cipher := testCipher(t, testComputer)
	tape := &fakeTape{t: t, cipher: cipher, payloads: map[string][]byte{}}
	chunks := newMemChunks()
	return &fixture{db: db, tape: tape, chunks: chunks, cipher: cipher, cfg: Config{
		ComputerID: testComputer, DB: db.Read, Cipher: cipher, Chunks: chunks, Appender: tape,
		ScratchDir: filepath.Join(t.TempDir(), "snapshot-tmp"),
		Witness:    witnessOf,
		Head:       func(ctx context.Context, tx *sql.Tx) (*computerevent.Head, error) { return headFromTable(ctx, tx) },
	}}
}

func roundTripWitness(t *testing.T, f *fixture) {
	t.Helper()
	ctx := context.Background()
	ref, err := TakeSnapshot(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	live, err := witnessOf(ctx, f.db.Read)
	if err != nil {
		t.Fatal(err)
	}
	if ref.Record.WitnessRoot != live.ContentRoot {
		t.Fatalf("recorded witness %s, live %s", ref.Record.WitnessRoot, live.ContentRoot)
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
	if got.ContentRoot != ref.Record.WitnessRoot {
		t.Fatalf("restored witness %s, record %s", got.ContentRoot, ref.Record.WitnessRoot)
	}
}

// Failure 9: page sizes other than 4096 copy and restore; the last chunk is
// partial.
func TestOtherPageSizesCopyAndRestore(t *testing.T) {
	for _, ps := range []int{512, 8192, 65536} {
		t.Run(fmt.Sprint(ps), func(t *testing.T) {
			f := pagedFixture(t, ps, false, 40, 60<<10)
			var size int
			if err := f.db.Read.QueryRow(`PRAGMA page_size`).Scan(&size); err != nil || size != ps {
				t.Fatalf("page size %d (%v), want %d", size, err, ps)
			}
			roundTripWitness(t, f)
		})
	}
}

// Failure 8: an auto_vacuum=INCREMENTAL store whose file shrank by
// incremental_vacuum (a stale in-header size is possible) copies and restores.
func TestAnIncrementallyVacuumedStoreCopiesAndRestores(t *testing.T) {
	f := pagedFixture(t, 4096, true, 60, 60<<10)
	if _, err := f.db.Write.Exec(`DELETE FROM blobs WHERE id % 2 = 0`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Write.Exec(`PRAGMA incremental_vacuum(100000)`); err != nil {
		t.Fatal(err)
	}
	roundTripWitness(t, f)
}

// Failure 9, the lock-byte page: a store past 1 GiB copies and restores. Skipped
// under -short: it writes about 1.1 GiB twice.
func TestAStorePastOneGiBCopiesAndRestores(t *testing.T) {
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
