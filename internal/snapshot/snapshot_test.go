package snapshot

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/dbx/dbxtest"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

// Ways snapshot v1 could fail, written before the code:
//
//  1. the restored file differs from the live store (witness v3 root differs);
//  2. a snapshot taken while the store holds more than ChunkSize bytes loses
//     or reorders a chunk;
//  3. the sealed record is readable without the computer's key, or another
//     computer's key opens it;
//  4. Restore installs over an existing state file, or leaves a half-written
//     file behind when a chunk is missing or corrupt;
//  5. Restore picks an older snapshot while a newer one is usable, or fails
//     the whole restore because the newest one is unusable;
//  6. the event does not name the sealed artifact digest, the head sequence
//     and the witness root.

type memChunks struct {
	mu     sync.Mutex
	chunks map[string][]byte
	// puts counts PutChunk calls since the last reset; probes counts HasChunk
	// calls. roots are the declared snapshot roots. noProbe makes the store a
	// plain ChunkStore (no existence check), as an old host would be.
	puts   []string
	probes int
	roots  [][]string
}

func newMemChunks() *memChunks { return &memChunks{chunks: map[string][]byte{}} }

func (m *memChunks) PutChunk(_ context.Context, digest string, sealed []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chunks[digest] = append([]byte(nil), sealed...)
	m.puts = append(m.puts, digest)
	return nil
}

// HasChunk reports what the store actually holds.
func (m *memChunks) HasChunk(_ context.Context, digest string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.probes++
	_, ok := m.chunks[digest]
	return ok, nil
}

func (m *memChunks) RegisterSnapshotRoot(_ context.Context, _ string, _ uint64, chunks []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.roots = append(m.roots, append([]string(nil), chunks...))
	return nil
}

func (m *memChunks) resetPuts() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.puts, m.probes = nil, 0
}

func (m *memChunks) GetChunk(_ context.Context, digest string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.chunks[digest]
	if !ok {
		return nil, errors.New("missing chunk")
	}
	return append([]byte(nil), data...), nil
}

// fakeTape is an appender and a source over an in-memory tape.
type fakeTape struct {
	t        *testing.T
	cipher   *computerevent.PrivateArtifactCipher
	mu       sync.Mutex
	events   []computerevent.DurableEvent
	payloads map[string][]byte
	held     int
}

func (f *fakeTape) HoldAppends(fn func() error) error {
	f.held++
	return fn()
}

func (f *fakeTape) AppendNewPrivatePayload(ctx context.Context, event computerevent.Event, _ computerevent.TransitionInput, plaintext []byte, mediaType string, cipher *computerevent.PrivateArtifactCipher) (computerevent.Receipt, string, error) {
	envelope, _, err := cipher.Encrypt(ctx, event.ComputerID, event.EventID, mediaType, "private", plaintext)
	if err != nil {
		return computerevent.Receipt{}, "", err
	}
	digest := computerevent.DigestBytes(envelope)
	f.mu.Lock()
	defer f.mu.Unlock()
	event.Sequence = uint64(len(f.events) + 1)
	event.PayloadCommitment = digest
	f.events = append(f.events, computerevent.DurableEvent{Request: computerevent.CASRequest{Event: event}})
	f.payloads[digest] = envelope
	return computerevent.Receipt{}, digest, nil
}

func (f *fakeTape) EventsPage(_ context.Context, _ string, after uint64, size int) ([]computerevent.DurableEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []computerevent.DurableEvent
	for _, e := range f.events {
		if e.Request.Event.Sequence > after && len(out) < size {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeTape) FetchPayload(_ context.Context, _, digest string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	data, ok := f.payloads[digest]
	if !ok {
		return nil, errors.New("missing payload")
	}
	return data, nil
}

func testCipher(t *testing.T, computerID string) *computerevent.PrivateArtifactCipher {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	c, err := computerevent.NewPrivateArtifactCipher(computerID, key)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func witnessOf(ctx context.Context, db *sql.DB) (selfdevprotocol.VMLocalContentWitness, error) {
	digests, err := computerversion.SQLiteStateExtractor{DB: db, Database: "texture"}.Digests(ctx)
	if err != nil {
		return selfdevprotocol.VMLocalContentWitness{}, err
	}
	return selfdevprotocol.WitnessFromDigests(digests)
}

type fixture struct {
	db     *dbx.SQLiteDB
	tape   *fakeTape
	chunks *memChunks
	cipher *computerevent.PrivateArtifactCipher
	cfg    Config
}

const testComputer = "computer-snapshot-test"

// newFixture builds a store of about bytes bytes.
func newFixture(t *testing.T, bytesApprox int) *fixture {
	t.Helper()
	db := dbxtest.SQLite(t)
	if _, err := db.Write.Exec(`CREATE TABLE blobs (id INTEGER PRIMARY KEY, body BLOB NOT NULL, note TEXT)`); err != nil {
		t.Fatal(err)
	}
	row := make([]byte, 64<<10)
	for i := 0; i*len(row) < bytesApprox; i++ {
		if _, err := rand.Read(row); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Write.Exec(`INSERT INTO blobs (id, body, note) VALUES (?, ?, ?)`, i+1, row, fmt.Sprintf("row %d", i)); err != nil {
			t.Fatal(err)
		}
	}
	cipher := testCipher(t, testComputer)
	tape := &fakeTape{t: t, cipher: cipher, payloads: map[string][]byte{}}
	chunks := newMemChunks()
	return &fixture{
		db: db, tape: tape, chunks: chunks, cipher: cipher,
		cfg: Config{
			ComputerID: testComputer,
			DB:         db.Read,
			Head: func(context.Context, *sql.Tx) (*computerevent.Head, error) {
				return &computerevent.Head{ComputerID: testComputer, Sequence: 41, CanonicalEventHead: strings.Repeat("e", 64)}, nil
			},
			Witness: func(ctx context.Context, tx *sql.Tx) (selfdevprotocol.VMLocalContentWitness, error) {
				digests, err := computerversion.SQLiteStateExtractor{Database: "texture"}.DigestsTx(ctx, tx)
				if err != nil {
					return selfdevprotocol.VMLocalContentWitness{}, err
				}
				return selfdevprotocol.WitnessFromDigests(digests)
			},
			Cipher:   cipher,
			Chunks:   chunks,
			Appender: tape,
		},
	}
}

func (f *fixture) restoreConfig(dest string) RestoreConfig {
	return RestoreConfig{ComputerID: testComputer, DestPath: dest, Source: f.tape, Chunks: f.chunks, Cipher: f.cipher}
}

func TestSnapshotRoundTripAcrossChunks(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, ChunkSize*2+ChunkSize/2) // several chunks plus a short tail
	ref, err := TakeSnapshot(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(ref.Record.Manifest.Files[0].Chunks); got < 3 {
		t.Fatalf("%d chunks, want a multi-chunk snapshot", got)
	}

	// The event names the sealed artifact, the head, the size and the root.
	if len(f.tape.events) != 1 {
		t.Fatalf("%d events appended, want 1", len(f.tape.events))
	}
	event := f.tape.events[0].Request.Event
	if event.EventKind != computerevent.EventSnapshotRecorded || event.PayloadCommitment != ref.ArtifactDigest || event.PrivacyClass != "private" {
		t.Fatalf("unexpected event %+v", event)
	}
	live, err := witnessOf(ctx, f.db.Read)
	if err != nil {
		t.Fatal(err)
	}
	if ref.Record.StoreHeadSequence != 41 || ref.Record.WitnessRoot != live.ContentRoot || ref.Record.SchemaHash != live.SchemaHash {
		t.Fatalf("record does not capture head and witness: %+v", ref.Record)
	}
	// The store's pages may still be in its WAL, so compare with the data
	// written rather than with the main file.
	if ref.Record.PlaintextSize < 2*ChunkSize {
		t.Fatalf("plaintext size %d for a store of about %d bytes", ref.Record.PlaintextSize, 2*ChunkSize+ChunkSize/2)
	}

	// The record is sealed: the envelope does not contain the witness root.
	if bytes.Contains(f.tape.payloads[ref.ArtifactDigest], []byte(ref.Record.WitnessRoot)) {
		t.Fatal("the sealed record exposes its witness root")
	}
	// Another computer's key does not open it.
	other := testCipher(t, testComputer)
	if _, _, err := other.Decrypt(ctx, f.tape.payloads[ref.ArtifactDigest], testComputer, ref.EventID); err == nil {
		t.Fatal("a different key opened the sealed record")
	}

	// Restore installs a file with the live witness.
	dest := filepath.Join(t.TempDir(), "state.db")
	record, err := Restore(ctx, f.restoreConfig(dest))
	if err != nil {
		t.Fatal(err)
	}
	if record.StoreHeadSequence != 41 {
		t.Fatalf("restored head sequence %d", record.StoreHeadSequence)
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
	if err := selfdevprotocol.WitnessContentMatches(got, live); err != nil {
		t.Fatalf("restored file differs from the live store: %v", err)
	}

	// An existing state file is never replaced.
	if _, err := Restore(ctx, f.restoreConfig(dest)); err == nil {
		t.Fatal("Restore replaced an existing state file")
	}
}

func TestRestoreWithoutASnapshotIsErrNoSnapshot(t *testing.T) {
	f := newFixture(t, 1<<10)
	_, err := Restore(context.Background(), f.restoreConfig(filepath.Join(t.TempDir(), "state.db")))
	if !errors.Is(err, ErrNoSnapshot) {
		t.Fatalf("err = %v, want ErrNoSnapshot", err)
	}
}

func TestRestorePrefersTheNewestAndFallsBackWhenItIsUnusable(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, 64<<10)
	first, err := TakeSnapshot(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Write.Exec(`INSERT INTO blobs (id, body, note) VALUES (1000, x'00', 'later')`); err != nil {
		t.Fatal(err)
	}
	second, err := TakeSnapshot(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if first.Record.WitnessRoot == second.Record.WitnessRoot {
		t.Fatal("test premise: the two snapshots should differ")
	}

	dest := filepath.Join(t.TempDir(), "state.db")
	record, err := Restore(ctx, f.restoreConfig(dest))
	if err != nil {
		t.Fatal(err)
	}
	if record.WitnessRoot != second.Record.WitnessRoot {
		t.Fatal("Restore did not take the newest snapshot")
	}

	// Corrupt the newest snapshot's chunk; the older one is used and no
	// half-written file is left at the destination's directory.
	for _, digest := range second.Record.Manifest.Files[0].Chunks {
		if _, shared := first.Record.Manifest.Files[0].Chunks[0], true; shared && digest == first.Record.Manifest.Files[0].Chunks[0] {
			continue
		}
		f.chunks.chunks[digest][len(f.chunks.chunks[digest])-1] ^= 0xff
	}
	dest2 := filepath.Join(t.TempDir(), "state.db")
	record, err = Restore(ctx, f.restoreConfig(dest2))
	if err != nil {
		// Both snapshots may share their only chunk; then neither is usable.
		if len(first.Record.Manifest.Files[0].Chunks) == 1 && first.Record.Manifest.Files[0].Chunks[0] == second.Record.Manifest.Files[0].Chunks[0] {
			t.Skip("snapshots share their chunks")
		}
		t.Fatal(err)
	}
	if record.WitnessRoot != first.Record.WitnessRoot {
		t.Fatal("Restore did not fall back to the older snapshot")
	}
	if entries, _ := os.ReadDir(filepath.Dir(dest2)); len(entries) != 1 {
		t.Fatalf("destination directory has %d entries, want only the state file", len(entries))
	}
}

func TestRestoreRefusesAMissingChunkWithoutLeavingAFile(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, 64<<10)
	ref, err := TakeSnapshot(ctx, f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	delete(f.chunks.chunks, ref.Record.Manifest.Files[0].Chunks[0])
	dir := t.TempDir()
	if _, err := Restore(ctx, f.restoreConfig(filepath.Join(dir, "state.db"))); err == nil {
		t.Fatal("Restore succeeded with a missing chunk")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("a failed restore left %d files behind", len(entries))
	}
}

func TestRestoreMaxSequenceSkipsNewerSnapshots(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, 64<<10)
	if _, err := TakeSnapshot(ctx, f.cfg); err != nil {
		t.Fatal(err)
	}
	cfg := f.restoreConfig(filepath.Join(t.TempDir(), "state.db"))
	cfg.MaxSequence = 10 // the snapshot captured head 41
	if _, err := Restore(ctx, cfg); err == nil {
		t.Fatal("Restore took a snapshot above the restore point")
	}
}
