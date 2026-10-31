package snapshot

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/filecas"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	_ "modernc.org/sqlite"
)

// Ways the page-stream snapshot could fail, written before the code (the
// build sheet's rule: failure modes first):
//
//  P1. a write commits while the stream runs and changes a page the stream
//      already read, or one it has not read yet: the snapshot must still be
//      exactly the state of its head (one read transaction, WAL snapshot), and
//      the commit must not wait for the stream;
//  P2. the file reassembled from the chunks is not a valid store, or its
//      witness differs from the one the record holds (head, pages and witness
//      must be one state);
//  P3. an unchanged store uploads its chunks again, or a small change uploads
//      most of the store (chunks must be cut by fixed page ranges so unchanged
//      ranges give identical bytes and digests);
//  P4. a chunk is skipped that the host does not actually hold: a host that
//      lost a chunk since the last snapshot must get it again, a probe that
//      fails or a host with no existence check must lead to an upload, never
//      to a skip;
//  P5. a snapshot recorded in the existing format (VACUUM INTO file cut at
//      4 MiB, no page geometry in the record) no longer restores;
//  P6. a record whose page geometry contradicts its manifest restores anyway;
//  P7. the stream blocks the write connection or appends (the read
//      transaction is on the read pool and holds nothing), or outlives the
//      stream and pins the WAL;
//  P8. the time budget is ignored, or its abort leaves a registered root, an
//      appended event or an open read transaction;
//  P9. a cancelled stream leaves a recorded event or a registered root;
//  P10. a page size other than 4096 breaks the cut (chunk = pages
//      [i*N,(i+1)*N) with N = ChunkSize/pageSize), or a store whose pages
//      repeat (zeroed pages) uploads one chunk many times.

func liveWitness(t *testing.T, f *fixture) selfdevprotocol.VMLocalContentWitness {
	t.Helper()
	w, err := witnessOf(context.Background(), f.db.Read)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func restoreAndOpen(t *testing.T, f *fixture) (*dbx.SQLiteDB, Record) {
	t.Helper()
	dest := filepath.Join(t.TempDir(), "state.db")
	record, err := Restore(context.Background(), f.restoreConfig(dest))
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	restored, err := dbx.OpenSQLite(dest, dbx.SQLiteOptions{ReadConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = restored.Close() })
	return restored, record
}

// P2
func TestStreamedFileIsAValidStoreWithTheRecordedWitness(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, ChunkSize*2+ChunkSize/3)
	live := liveWitness(t, f)
	ref, err := TakeSnapshot(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	rec := ref.Record
	if rec.PageSize != 4096 || rec.PagesPerChunk != 1024 || rec.PageCount <= 2048 || rec.PlaintextSize != int64(rec.PageSize)*rec.PageCount {
		t.Fatalf("unexpected geometry %+v", rec)
	}
	if want := int((rec.PageCount + 1023) / 1024); len(rec.Manifest.Files[0].Chunks) != want || ref.Stats.Chunks != want {
		t.Fatalf("%d chunks in the manifest, want %d", len(rec.Manifest.Files[0].Chunks), want)
	}
	restored, _ := restoreAndOpen(t, f)
	var pages int64
	if err := restored.Read.QueryRow(`PRAGMA page_count`).Scan(&pages); err != nil {
		t.Fatal(err)
	}
	if pages != rec.PageCount {
		t.Fatalf("restored file has %d pages, the record %d", pages, rec.PageCount)
	}
	var check string
	if err := restored.Read.QueryRow(`PRAGMA integrity_check`).Scan(&check); err != nil || check != "ok" {
		t.Fatalf("integrity_check = %q, %v", check, err)
	}
	got, err := witnessOf(ctx, restored.Read)
	if err != nil {
		t.Fatal(err)
	}
	if err := selfdevprotocol.WitnessContentMatches(got, live); err != nil {
		t.Fatalf("restored witness differs from the live store: %v", err)
	}
	if rec.WitnessRoot != got.ContentRoot || rec.SchemaHash != got.SchemaHash {
		t.Fatalf("the recorded witness %s is not what restore recomputes %s", rec.WitnessRoot, got.ContentRoot)
	}
	// The transaction ended: the WAL can be truncated.
	assertWALReleased(t, f)
}

func assertWALReleased(t *testing.T, f *fixture) {
	t.Helper()
	var busy, logFrames, checkpointed int
	if err := f.db.Write.QueryRow(`PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &logFrames, &checkpointed); err != nil {
		t.Fatal(err)
	}
	if busy != 0 {
		t.Fatalf("wal_checkpoint(TRUNCATE) busy=%d: a read transaction of the snapshot is still open", busy)
	}
}

// P1, P7
func TestWritesInsideTheStreamStayOutOfTheSnapshot(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, ChunkSize*3)
	headsFixture(t, f)
	liveBefore := liveWitness(t, f)
	var rowsBefore int
	if err := f.db.Write.QueryRow(`SELECT COUNT(*) FROM blobs`).Scan(&rowsBefore); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	writes := 0
	f.cfg.pageHook = func(pgno int64) {
		// Commit in the middle of the stream, on the write connection, while
		// the stream's read transaction is open: new rows, a rewritten early
		// row (a page the stream already read) and a rewritten late row (a
		// page it has not read yet), and an attempt to checkpoint the WAL.
		if pgno != 40 && pgno != 1500 && pgno != 2500 {
			return
		}
		done := make(chan error, 1)
		go func() {
			var err error
			for i := 0; i < 5 && err == nil; i++ {
				mu.Lock()
				writes++
				id := 200000 + writes
				mu.Unlock()
				err = commitRow(f.db, id)
			}
			if err == nil {
				_, err = f.db.Write.Exec(`UPDATE blobs SET body = randomblob(65536), note = 'rewritten' WHERE id IN (1, ?)`, rowsBefore)
			}
			if err == nil {
				_, err = f.db.Write.Exec(`PRAGMA wal_checkpoint(PASSIVE)`)
			}
			done <- err
		}()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("write during the stream failed: %v", err)
			}
		case <-time.After(20 * time.Second):
			t.Errorf("a write during the stream waited for it (page %d)", pgno)
		}
	}
	ref, err := TakeSnapshot(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	wrote := writes
	mu.Unlock()
	if wrote == 0 {
		t.Fatal("test premise: no write ran during the stream")
	}
	// The snapshot is the state at its head: the rows before the first
	// in-stream commit.
	if ref.Record.StoreHeadSequence != uint64(rowsBefore) {
		t.Fatalf("record head %d, want %d: the head belongs to the snapshot, not to a later commit", ref.Record.StoreHeadSequence, rowsBefore)
	}
	restored, _ := restoreAndOpen(t, f)
	var inFile int
	if err := restored.Read.QueryRow(`SELECT COUNT(*) FROM blobs`).Scan(&inFile); err != nil {
		t.Fatal(err)
	}
	if inFile != rowsBefore {
		t.Fatalf("the snapshot holds %d rows, its head names %d", inFile, rowsBefore)
	}
	got, err := witnessOf(ctx, restored.Read)
	if err != nil {
		t.Fatal(err)
	}
	if got.ContentRoot != ref.Record.WitnessRoot {
		t.Fatalf("restored witness %s differs from the recorded %s", got.ContentRoot, ref.Record.WitnessRoot)
	}
	// The recorded witness is the pre-write state, not the live one.
	if ref.Record.WitnessRoot != liveBefore.ContentRoot {
		t.Fatalf("recorded witness %s, the store at the snapshot point had %s", ref.Record.WitnessRoot, liveBefore.ContentRoot)
	}
	if liveNow := liveWitness(t, f); liveNow.ContentRoot == ref.Record.WitnessRoot {
		t.Fatal("test premise: the live store should have moved on")
	}
	assertWALReleased(t, f)
}

// addSmallTable adds a table whose rows fit one page, created after the bulk
// data so its page sits at the end of the file.
func addSmallTable(t *testing.T, f *fixture) {
	t.Helper()
	if _, err := f.db.Write.Exec(`CREATE TABLE small (id INTEGER PRIMARY KEY, v TEXT)`); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 10; i++ {
		if _, err := f.db.Write.Exec(`INSERT INTO small (id, v) VALUES (?, ?)`, i, fmt.Sprintf("v%d", i)); err != nil {
			t.Fatal(err)
		}
	}
}

// P3
func TestSecondSnapshotUploadsOnlyTheChangedChunks(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, ChunkSize*6)
	addSmallTable(t, f)

	first, err := TakeSnapshot(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	total := first.Stats.Chunks
	if total < 6 || first.Stats.ChunksUploaded+first.Stats.ChunksRepeated != total || first.Stats.ChunksHeld != 0 {
		t.Fatalf("first snapshot stats %+v", first.Stats)
	}

	// Nothing changed: every chunk is held, nothing is uploaded.
	f.chunks.resetPuts()
	same, err := TakeSnapshot(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.chunks.puts) != 0 || same.Stats.ChunksUploaded != 0 || same.Stats.ChunksHeld+same.Stats.ChunksRepeated != total {
		t.Fatalf("an unchanged store uploaded %d chunks: %+v", len(f.chunks.puts), same.Stats)
	}
	if !equalStrings(same.Record.Manifest.Files[0].Chunks, first.Record.Manifest.Files[0].Chunks) {
		t.Fatal("an unchanged store gave different chunk digests")
	}

	// A small change touches a page or two: only their chunks go up.
	if _, err := f.db.Write.Exec(`UPDATE small SET v = 'changed' WHERE id = 3`); err != nil {
		t.Fatal(err)
	}
	f.chunks.resetPuts()
	second, err := TakeSnapshot(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if second.Stats.ChunksUploaded < 1 || second.Stats.ChunksUploaded > 2 || len(f.chunks.puts) != second.Stats.ChunksUploaded {
		t.Fatalf("a one-row change uploaded %d of %d chunks: %+v", second.Stats.ChunksUploaded, total, second.Stats)
	}
	if second.Stats.ChunksHeld < total-2 {
		t.Fatalf("only %d of %d chunks were found held: %+v", second.Stats.ChunksHeld, total, second.Stats)
	}
	changed := 0
	for i, digest := range second.Record.Manifest.Files[0].Chunks {
		if digest != first.Record.Manifest.Files[0].Chunks[i] {
			changed++
		}
	}
	if changed != second.Stats.ChunksUploaded {
		t.Fatalf("%d chunk digests changed, %d uploaded", changed, second.Stats.ChunksUploaded)
	}

	// Both restore to their own state.
	restored, record := restoreAndOpen(t, f)
	if record.WitnessRoot != second.Record.WitnessRoot {
		t.Fatal("restore did not take the newest snapshot")
	}
	var v string
	if err := restored.Read.QueryRow(`SELECT v FROM small WHERE id = 3`).Scan(&v); err != nil || v != "changed" {
		t.Fatalf("restored row = %q, %v", v, err)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// P4
func TestAChunkTheHostLostIsUploadedAgain(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, ChunkSize*3)
	first, err := TakeSnapshot(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	lost := first.Record.Manifest.Files[0].Chunks[1]
	delete(f.chunks.chunks, lost)
	f.chunks.resetPuts()
	second, err := TakeSnapshot(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if second.Stats.ChunksUploaded != 1 || len(f.chunks.puts) != 1 || f.chunks.puts[0] != lost {
		t.Fatalf("the lost chunk was not uploaded again: %+v puts=%v", second.Stats, f.chunks.puts)
	}
	if _, ok := f.chunks.chunks[lost]; !ok {
		t.Fatal("the lost chunk is still missing")
	}
	restoreAndOpen(t, f)
}

// failingProbe says nothing useful: every existence check errors.
type failingProbe struct{ *memChunks }

func (failingProbe) HasChunk(context.Context, string) (bool, error) {
	return false, errors.New("host does not know HEAD")
}

// plainChunks hides the existence check: a host or store that cannot say.
type plainChunks struct{ inner *memChunks }

func (p plainChunks) PutChunk(ctx context.Context, d string, b []byte) error {
	return p.inner.PutChunk(ctx, d, b)
}
func (p plainChunks) GetChunk(ctx context.Context, d string) ([]byte, error) {
	return p.inner.GetChunk(ctx, d)
}

// P4
func TestAnUnknownChunkIsUploadedNeverSkipped(t *testing.T) {
	ctx := context.Background()
	for name, wrap := range map[string]func(*memChunks) ChunkStore{
		"probe errors":     func(m *memChunks) ChunkStore { return failingProbe{m} },
		"no existence API": func(m *memChunks) ChunkStore { return plainChunks{m} },
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, ChunkSize*2)
			f.cfg.Chunks = wrap(f.chunks)
			if _, err := TakeSnapshot(ctx, f.cfg); err != nil {
				t.Fatal(err)
			}
			f.chunks.resetPuts()
			second, err := TakeSnapshot(ctx, f.cfg)
			if err != nil {
				t.Fatal(err)
			}
			if second.Stats.ChunksHeld != 0 || second.Stats.ChunksUploaded+second.Stats.ChunksRepeated != second.Stats.Chunks {
				t.Fatalf("stats %+v: with no answer from the host every chunk must be uploaded", second.Stats)
			}
			if name == "probe errors" && second.Stats.ProbeErrors == 0 {
				t.Fatal("probe errors were not counted")
			}
			restoreAndOpen(t, f)
		})
	}
}

// oldFormatSnapshot records a snapshot the way the VACUUM INTO era did: a
// copy cut at 4 MiB with no page geometry in the record.
func oldFormatSnapshot(t *testing.T, f *fixture) Record {
	t.Helper()
	ctx := context.Background()
	copyPath := filepath.Join(t.TempDir(), "copy.db")
	if _, err := f.db.Read.ExecContext(ctx, "VACUUM INTO ?", copyPath); err != nil {
		t.Fatal(err)
	}
	snap, err := dbx.OpenSQLite(copyPath, dbx.SQLiteOptions{ReadConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	witness, err := witnessOf(ctx, snap.Read)
	_ = snap.Close()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(copyPath)
	if err != nil {
		t.Fatal(err)
	}
	dek, err := f.cipher.ExportKeyForEscrow(ctx, testComputer)
	if err != nil {
		t.Fatal(err)
	}
	entry := filecas.FileEntry{Path: StateFileName, Mode: 0o600, Size: int64(len(data))}
	for off := 0; off < len(data); off += ChunkSize {
		end := min(off+ChunkSize, len(data))
		sealed, digest, err := filecas.SealChunk(dek, testComputer, data[off:end])
		if err != nil {
			t.Fatal(err)
		}
		if err := f.chunks.PutChunk(ctx, digest, sealed); err != nil {
			t.Fatal(err)
		}
		entry.Chunks = append(entry.Chunks, digest)
	}
	manifest, err := filecas.BuildManifest(testComputer, []filecas.FileEntry{entry}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	record := Record{
		Version: Version, ComputerID: testComputer, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		StoreHeadSequence: 41, StoreHead: strings.Repeat("e", 64), PlaintextSize: entry.Size,
		WitnessRoot: witness.ContentRoot, SchemaHash: witness.SchemaHash, Manifest: *manifest,
	}
	plaintext, err := jsonMarshal(record)
	if err != nil {
		t.Fatal(err)
	}
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: "event-old-format", ComputerID: testComputer,
		EventKind: computerevent.EventSnapshotRecorded, PrivacyClass: "private",
	}
	if _, _, err := f.tape.AppendNewPrivatePayload(ctx, event, computerevent.TransitionInput{}, plaintext, RecordMediaType, f.cipher); err != nil {
		t.Fatal(err)
	}
	return record
}

// P5
func TestSnapshotsRecordedBeforeThePageStreamStillRestore(t *testing.T) {
	f := newFixture(t, ChunkSize*2+ChunkSize/2)
	old := oldFormatSnapshot(t, f)
	if old.PageSize != 0 || old.PagesPerChunk != 0 {
		t.Fatal("test premise: the old record carries no geometry")
	}
	restored, record := restoreAndOpen(t, f)
	if record.WitnessRoot != old.WitnessRoot {
		t.Fatal("restored the wrong record")
	}
	got, err := witnessOf(context.Background(), restored.Read)
	if err != nil {
		t.Fatal(err)
	}
	if got.ContentRoot != old.WitnessRoot {
		t.Fatalf("old-format restore witness %s, recorded %s", got.ContentRoot, old.WitnessRoot)
	}
	// And a page-cut snapshot taken after it is the newest and restores too.
	if _, err := TakeSnapshot(context.Background(), f.cfg); err != nil {
		t.Fatal(err)
	}
	restoreAndOpen(t, f)
}

// The two formats cut the same bytes: the same store gives the same sealed
// chunks either way (only the last short chunk and page 1 may differ in a
// copy that VACUUM rewrote, so compare through the restored witness above, and
// here the geometry of a store whose file is page aligned).
func TestPageCutAndByteCutAreTheSameCut(t *testing.T) {
	f := newFixture(t, ChunkSize*2)
	ref, err := TakeSnapshot(context.Background(), f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	rec := ref.Record
	if int64(rec.PageSize)*int64(rec.PagesPerChunk) != ChunkSize {
		t.Fatalf("a page range is %d bytes, not ChunkSize", int64(rec.PageSize)*int64(rec.PagesPerChunk))
	}
	var total int64
	dek, _ := f.cipher.ExportKeyForEscrow(context.Background(), testComputer)
	for i, digest := range rec.Manifest.Files[0].Chunks {
		plain, err := filecas.OpenChunk(dek, testComputer, digest, f.chunks.chunks[digest])
		if err != nil {
			t.Fatal(err)
		}
		if i < len(rec.Manifest.Files[0].Chunks)-1 && len(plain) != ChunkSize {
			t.Fatalf("chunk %d has %d bytes", i, len(plain))
		}
		total += int64(len(plain))
	}
	if total != rec.PlaintextSize {
		t.Fatalf("chunks hold %d bytes, record says %d", total, rec.PlaintextSize)
	}
}

// P6
func TestRestoreRefusesGeometryThatContradictsTheManifest(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, ChunkSize*2)
	ref, err := TakeSnapshot(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*Record){
		"page count off by one":   func(r *Record) { r.PageCount++ },
		"pages per chunk too few": func(r *Record) { r.PagesPerChunk /= 2 },
		"page size not a multiple": func(r *Record) {
			r.PageSize = 4097
			r.PlaintextSize = int64(r.PageSize) * r.PageCount
			r.Manifest.Files[0].Size = r.PlaintextSize
		},
	} {
		t.Run(name, func(t *testing.T) {
			bad := ref.Record
			mutate(&bad)
			// Re-root the manifest so only the geometry is wrong.
			manifest, err := filecas.BuildManifest(testComputer, bad.Manifest.Files, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			bad.Manifest = *manifest
			f2 := newFixture(t, ChunkSize*2)
			f2.chunks = f.chunks
			f2.cipher = f.cipher
			f2.tape = &fakeTape{t: t, cipher: f.cipher, payloads: map[string][]byte{}}
			plaintext, err := jsonMarshal(bad)
			if err != nil {
				t.Fatal(err)
			}
			event := computerevent.Event{SchemaVersion: computerevent.SchemaVersionV1, EventID: "event-bad", ComputerID: testComputer, EventKind: computerevent.EventSnapshotRecorded, PrivacyClass: "private"}
			if _, _, err := f2.tape.AppendNewPrivatePayload(ctx, event, computerevent.TransitionInput{}, plaintext, RecordMediaType, f.cipher); err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			if _, err := Restore(ctx, f2.restoreConfig(filepath.Join(dir, "state.db"))); err == nil {
				t.Fatal("Restore accepted a record whose geometry contradicts its manifest")
			}
			if entries, _ := os.ReadDir(dir); len(entries) != 0 {
				t.Fatalf("a refused restore left %d files", len(entries))
			}
		})
	}
}

// P8
func TestTheTimeBudgetAbortsTheStreamAndRecordsNothing(t *testing.T) {
	f := newFixture(t, ChunkSize*3)
	f.cfg.MaxDuration = 100 * time.Millisecond
	f.cfg.pageHook = func(pgno int64) {
		if pgno%16 == 0 {
			time.Sleep(2 * time.Millisecond)
		}
	}
	started := time.Now()
	_, err := TakeSnapshot(context.Background(), f.cfg)
	if !errors.Is(err, ErrTimeBudget) {
		t.Fatalf("err = %v, want ErrTimeBudget", err)
	}
	if !strings.Contains(err.Error(), "pages read") {
		t.Fatalf("the abort does not say how far it got: %v", err)
	}
	if time.Since(started) > 10*time.Second {
		t.Fatalf("the stream ran %s past a 100ms budget", time.Since(started))
	}
	if len(f.tape.events) != 0 || len(f.chunks.roots) != 0 {
		t.Fatalf("an aborted stream left %d events and %d registered roots", len(f.tape.events), len(f.chunks.roots))
	}
	assertWALReleased(t, f)
}

// P9
func TestACancelledStreamLeavesNoEventAndNoRoot(t *testing.T) {
	f := newFixture(t, ChunkSize*3)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.cfg.pageHook = func(pgno int64) {
		if pgno == 1200 {
			cancel()
		}
	}
	_, err := TakeSnapshot(ctx, f.cfg)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if len(f.tape.events) != 0 || len(f.chunks.roots) != 0 {
		t.Fatalf("a cancelled stream left %d events and %d registered roots", len(f.tape.events), len(f.chunks.roots))
	}
	assertWALReleased(t, f)
}

// An upload failure also ends the stream with no root and no event.
type putFailsChunks struct {
	*memChunks
	after int
	n     int
	mu    sync.Mutex
}

func (p *putFailsChunks) PutChunk(ctx context.Context, d string, b []byte) error {
	p.mu.Lock()
	p.n++
	fail := p.n > p.after
	p.mu.Unlock()
	if fail {
		return errors.New("host unavailable")
	}
	return p.memChunks.PutChunk(ctx, d, b)
}

func TestAnUploadFailureEndsTheSnapshotWithNoRootAndNoEvent(t *testing.T) {
	f := newFixture(t, ChunkSize*4)
	f.cfg.Chunks = &putFailsChunks{memChunks: f.chunks, after: 1}
	_, err := TakeSnapshot(context.Background(), f.cfg)
	if err == nil || !strings.Contains(err.Error(), "host unavailable") {
		t.Fatalf("err = %v, want the upload failure", err)
	}
	if len(f.tape.events) != 0 || len(f.chunks.roots) != 0 {
		t.Fatalf("a failed upload left %d events and %d roots", len(f.tape.events), len(f.chunks.roots))
	}
	assertWALReleased(t, f)
}

// P10
func TestPageSizesOtherThan4096RoundTrip(t *testing.T) {
	ctx := context.Background()
	for _, pageSize := range []int{1024, 16384, 65536} {
		t.Run(fmt.Sprint(pageSize), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "sized.db")
			seed, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			seed.SetMaxOpenConns(1)
			if _, err := seed.Exec(fmt.Sprintf(`PRAGMA page_size=%d`, pageSize)); err != nil {
				t.Fatal(err)
			}
			if _, err := seed.Exec(`CREATE TABLE blobs (id INTEGER PRIMARY KEY, body BLOB NOT NULL, note TEXT)`); err != nil {
				t.Fatal(err)
			}
			for i := 1; i <= 48; i++ {
				if _, err := seed.Exec(`INSERT INTO blobs (id, body, note) VALUES (?, randomblob(100000), ?)`, i, "x"); err != nil {
					t.Fatal(err)
				}
			}
			if err := seed.Close(); err != nil {
				t.Fatal(err)
			}
			db, err := dbx.OpenSQLite(path, dbx.SQLiteOptions{ReadConns: 2})
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			f := newFixture(t, 1<<10)
			f.db = db
			f.cfg.DB = db.Read
			ref, err := TakeSnapshot(ctx, f.cfg)
			if err != nil {
				t.Fatal(err)
			}
			if ref.Record.PageSize != pageSize || ref.Record.PagesPerChunk != ChunkSize/pageSize {
				t.Fatalf("geometry %+v", ref.Record)
			}
			if ref.Stats.Chunks < 2 {
				t.Fatalf("test premise: %d chunks", ref.Stats.Chunks)
			}
			restored, _ := restoreAndOpen(t, f)
			got, err := witnessOf(ctx, restored.Read)
			if err != nil {
				t.Fatal(err)
			}
			if got.ContentRoot != ref.Record.WitnessRoot {
				t.Fatal("restored witness differs")
			}
			var check string
			if err := restored.Read.QueryRow(`PRAGMA integrity_check`).Scan(&check); err != nil || check != "ok" {
				t.Fatalf("integrity_check = %q, %v", check, err)
			}
		})
	}
}

// Stats and the register call carry the manifest's chunk list.
func TestSnapshotRegistersItsChunksBeforeTheEvent(t *testing.T) {
	f := newFixture(t, ChunkSize*2)
	ref, err := TakeSnapshot(context.Background(), f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.chunks.roots) != 1 || !equalStrings(f.chunks.roots[0], ref.Record.Manifest.Files[0].Chunks) {
		t.Fatalf("registered roots %v", f.chunks.roots)
	}
	if ref.Stats.StreamTime <= 0 || ref.Stats.ReadTxTime < ref.Stats.StreamTime || ref.Stats.Pages != ref.Record.PageCount {
		t.Fatalf("stats %+v", ref.Stats)
	}
	if !bytes.Equal([]byte(ref.Record.Manifest.ComputerID), []byte(testComputer)) {
		t.Fatal("manifest computer")
	}
}

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }
