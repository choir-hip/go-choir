package agentcore

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	"github.com/yusefmosiah/go-choir/internal/snapshot"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// Ways the page-stream snapshot could fail against the real store (the
// snapshot package's tests list the stream's own; these are the ones that need
// the store's head table, the real witness extractor and restore's verifier):
//
//  S1. the head recorded is not the head of the pages (a write commits while
//      the stream runs): restore replays from the wrong sequence;
//  S2. the witness recorded in the stream's transaction differs from the one
//      restore recomputes from the reassembled file (verifySnapshotFile is the
//      check restore itself uses, so a mismatch is a refused restore);
//  S3. the stream queues the store's writes: with a writer committing
//      continuously, the commit count during the snapshot must keep growing;
//  S4. a second snapshot of a store with a small change re-uploads the store.

// streamTape is the one-event tape of a snapshot taken in a test.
type streamTape struct {
	mu       sync.Mutex
	events   []computerevent.DurableEvent
	payloads map[string][]byte
	chunks   map[string][]byte
	puts     int
	probes   int
}

func newStreamTape() *streamTape {
	return &streamTape{payloads: map[string][]byte{}, chunks: map[string][]byte{}}
}

func (s *streamTape) AppendNewPrivatePayload(ctx context.Context, event computerevent.Event, _ computerevent.TransitionInput, plaintext []byte, mediaType string, cipher *computerevent.PrivateArtifactCipher) (computerevent.Receipt, string, error) {
	envelope, _, err := cipher.Encrypt(ctx, event.ComputerID, event.EventID, mediaType, "private", plaintext)
	if err != nil {
		return computerevent.Receipt{}, "", err
	}
	digest := computerevent.DigestBytes(envelope)
	s.mu.Lock()
	defer s.mu.Unlock()
	event.Sequence = uint64(len(s.events) + 1)
	event.PayloadCommitment = digest
	s.events = append(s.events, computerevent.DurableEvent{Request: computerevent.CASRequest{Event: event}})
	s.payloads[digest] = envelope
	return computerevent.Receipt{}, digest, nil
}

func (s *streamTape) EventsPage(_ context.Context, _ string, after uint64, size int) ([]computerevent.DurableEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []computerevent.DurableEvent
	for _, e := range s.events {
		if e.Request.Event.Sequence > after && len(out) < size {
			out = append(out, e)
		}
	}
	return out, nil
}

func (s *streamTape) FetchPayload(_ context.Context, _, digest string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.payloads[digest]
	if !ok {
		return nil, errors.New("stream tape: payload not found")
	}
	return data, nil
}

func (s *streamTape) PutChunk(_ context.Context, digest string, sealed []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.puts++
	s.chunks[digest] = append([]byte(nil), sealed...)
	return nil
}

func (s *streamTape) HasChunk(_ context.Context, digest string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.probes++
	_, ok := s.chunks[digest]
	return ok, nil
}

func (s *streamTape) GetChunk(_ context.Context, digest string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.chunks[digest]
	if !ok {
		return nil, errors.New("stream tape: chunk not found")
	}
	return append([]byte(nil), data...), nil
}

func (s *streamTape) resetPuts() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.puts, s.probes = 0, 0
}

const streamComputer = "computer-stream"

func streamHead(n uint64) string {
	sum := sha256.Sum256([]byte{byte(n), byte(n >> 8), byte(n >> 16)})
	return hex.EncodeToString(sum[:])
}

// newStreamStore opens a real store with a projection head at sequence
// baseRows and a table of baseRows rows of about 8 KiB: the head sequence
// always equals the row count, so a snapshot's head can be checked against
// its pages.
func newStreamStore(t *testing.T, baseRows int) *choirstore.Store {
	t.Helper()
	store, err := choirstore.OpenFresh(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	db := store.DB()
	if _, err := db.Exec(`CREATE TABLE snapshot_load (id INTEGER PRIMARY KEY, body BLOB NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	body := make([]byte, 8<<10)
	for i := 1; i <= baseRows; i++ {
		if _, err := rand.Read(body); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO snapshot_load (id, body) VALUES (?, ?)`, i, body); err != nil {
			t.Fatal(err)
		}
	}
	head := streamHead(uint64(baseRows))
	if _, err := db.Exec(`INSERT INTO computer_event_projection_heads (computer_id, sequence, canonical_event_head, desired_event_head, effective_event_head, desired_state_commitment, effective_state_commitment, reducer_version, credential_revocation_epoch, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		streamComputer, baseRows, head, head, head, head, head, computerevent.ReducerVersionV1, 0, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	return store
}

// commitStreamRow is one transaction that keeps head sequence == row count.
func commitStreamRow(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var seq uint64
	if err := tx.QueryRow(`SELECT sequence FROM computer_event_projection_heads WHERE computer_id=?`, streamComputer).Scan(&seq); err != nil {
		return err
	}
	seq++
	body := make([]byte, 8<<10)
	if _, err := rand.Read(body); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO snapshot_load (id, body) VALUES (?, ?)`, seq, body); err != nil {
		return err
	}
	head := streamHead(seq)
	if _, err := tx.Exec(`UPDATE computer_event_projection_heads SET sequence=?, canonical_event_head=?, desired_event_head=?, effective_event_head=? WHERE computer_id=?`, seq, head, head, head, streamComputer); err != nil {
		return err
	}
	return tx.Commit()
}

func streamConfig(t *testing.T, store *choirstore.Store, tape *streamTape, cipher *computerevent.PrivateArtifactCipher) snapshot.Config {
	return snapshot.Config{
		ComputerID: streamComputer,
		DB:         store.ReadDB(),
		Head: func(ctx context.Context, tx *sql.Tx) (*computerevent.Head, error) {
			return store.HeadTx(ctx, tx, streamComputer)
		},
		Witness: func(ctx context.Context, tx *sql.Tx) (selfdevprotocol.VMLocalContentWitness, error) {
			return LiveStoreWitnessTx(ctx, tx)
		},
		Cipher:      cipher,
		Chunks:      tape,
		Appender:    tape,
		MaxDuration: 5 * time.Minute,
	}
}

func streamCipher(t *testing.T) *computerevent.PrivateArtifactCipher {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	cipher, err := computerevent.NewPrivateArtifactCipher(streamComputer, key)
	if err != nil {
		t.Fatal(err)
	}
	return cipher
}

// restoreStream installs the newest snapshot with restore's own verifier (the
// witness check) and opens the result.
func restoreStream(t *testing.T, tape *streamTape, cipher *computerevent.PrivateArtifactCipher) (*dbx.SQLiteDB, snapshot.Record) {
	t.Helper()
	dest := filepath.Join(t.TempDir(), "state.db")
	record, err := snapshot.Restore(context.Background(), snapshot.RestoreConfig{
		ComputerID: streamComputer, DestPath: dest, Source: tape, Chunks: tape, Cipher: cipher,
		Verify: verifySnapshotFile,
	})
	if err != nil {
		t.Fatalf("restore with the witness check: %v", err)
	}
	restored, err := dbx.OpenSQLite(dest, dbx.SQLiteOptions{ReadConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = restored.Close() })
	return restored, record
}

// S1, S2, S3: a writer commits row+head transactions without pause while the
// snapshot streams; the restore verifier recomputes the witness of the
// reassembled file and it must equal the recorded one, and the recorded head
// must be the head of the pages restored.
func TestSnapshotUnderContinuousWritesRestoresToAWitnessMatch(t *testing.T) {
	ctx := context.Background()
	store := newStreamStore(t, 1200) // ~10 MB: three chunks
	tape := newStreamTape()
	cipher := streamCipher(t)

	var commits atomic.Int64
	stop := make(chan struct{})
	writerDone := make(chan error, 1)
	go func() {
		defer close(writerDone)
		for {
			select {
			case <-stop:
				return
			default:
			}
			if err := commitStreamRow(store.DB()); err != nil {
				writerDone <- err
				return
			}
			commits.Add(1)
		}
	}()
	for commits.Load() < 3 {
		time.Sleep(time.Millisecond)
	}

	before := commits.Load()
	ref, err := snapshot.TakeSnapshot(ctx, streamConfig(t, store, tape, cipher))
	during := commits.Load() - before
	close(stop)
	if werr := <-writerDone; werr != nil {
		t.Fatalf("the writer failed during the snapshot: %v", werr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if during < 2 {
		t.Fatalf("the writer committed %d transactions during the snapshot: the stream queued the store's writes", during)
	}

	restored, record := restoreStream(t, tape, cipher)
	if record.WitnessRoot != ref.Record.WitnessRoot {
		t.Fatal("restore chose another snapshot")
	}
	var rows, headSeq uint64
	var headDigest string
	if err := restored.Read.QueryRow(`SELECT COUNT(*) FROM snapshot_load`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if err := restored.Read.QueryRow(`SELECT sequence, canonical_event_head FROM computer_event_projection_heads WHERE computer_id=?`, streamComputer).Scan(&headSeq, &headDigest); err != nil {
		t.Fatal(err)
	}
	if rows != headSeq || record.StoreHeadSequence != headSeq || record.StoreHead != headDigest || headDigest != streamHead(headSeq) {
		t.Fatalf("rows %d, restored head %d (%s), recorded head %d (%s): the head must be the head of the pages", rows, headSeq, headDigest, record.StoreHeadSequence, record.StoreHead)
	}
	var live uint64
	if err := store.DB().QueryRow(`SELECT COUNT(*) FROM snapshot_load`).Scan(&live); err != nil {
		t.Fatal(err)
	}
	if live <= rows {
		t.Fatalf("test premise: the live store (%d rows) should be past the snapshot (%d)", live, rows)
	}
	t.Logf("snapshot at head %d while the writer made %d commits; live store now at %d; %+v", record.StoreHeadSequence, during, live, ref.Stats)
}

// S4: against the real store, a second snapshot after a small change uploads a
// small part of it.
func TestSecondSnapshotOfTheRealStoreIsIncremental(t *testing.T) {
	ctx := context.Background()
	store := newStreamStore(t, 2400) // ~20 MB: about five chunks
	tape := newStreamTape()
	cipher := streamCipher(t)
	cfg := streamConfig(t, store, tape, cipher)

	first, err := snapshot.TakeSnapshot(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if first.Stats.Chunks < 4 || first.Stats.ChunksUploaded != first.Stats.Chunks {
		t.Fatalf("first snapshot %+v", first.Stats)
	}
	// One more row appends pages at the end of the table and the head page.
	if err := commitStreamRow(store.DB()); err != nil {
		t.Fatal(err)
	}
	tape.resetPuts()
	second, err := snapshot.TakeSnapshot(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if second.Stats.ChunksUploaded == 0 || second.Stats.ChunksUploaded > 3 || second.Stats.ChunksHeld < second.Stats.Chunks-3 {
		t.Fatalf("one new row uploaded %d of %d chunks: %+v", second.Stats.ChunksUploaded, second.Stats.Chunks, second.Stats)
	}
	if tape.puts != second.Stats.ChunksUploaded {
		t.Fatalf("%d PUTs for %d uploaded chunks", tape.puts, second.Stats.ChunksUploaded)
	}
	restored, record := restoreStream(t, tape, cipher)
	if record.StoreHeadSequence != 2401 {
		t.Fatalf("restored head %d, want the second snapshot's 2401", record.StoreHeadSequence)
	}
	var rows int
	if err := restored.Read.QueryRow(`SELECT COUNT(*) FROM snapshot_load`).Scan(&rows); err != nil || rows != 2401 {
		t.Fatalf("restored rows = %d, %v", rows, err)
	}
}
