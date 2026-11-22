package agentcore

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/filecas"
	"github.com/yusefmosiah/go-choir/internal/snapshot"
)

// Ways restore of a page-digest (version 2) record could fail against the real
// store, written before the reader:
//
//  H1. the restored file's head row is not the head the record names (a
//      capture that recorded one head and streamed the pages of another):
//      restore must refuse, by the sequence and by the canonical head;
//  H2. a version 2 record is verified by recomputing the logical witness (the
//      record carries none, so every restore would refuse), or by nothing;
//  H3. a version 1 record stops being verified against its witness.

// v2Of is the version 2 record of ref's snapshot, built from the tape's own
// chunks, put alone on a fresh tape.
func v2Of(t *testing.T, tape *streamTape, cipher *computerevent.PrivateArtifactCipher, ref snapshot.Ref, mutate func(*snapshot.Record)) {
	t.Helper()
	dek, err := cipher.ExportKeyForEscrow(context.Background(), streamComputer)
	if err != nil {
		t.Fatal(err)
	}
	rec := ref.Record
	digest := snapshot.NewPageDigest(rec.PageSize, rec.PageCount)
	for _, d := range rec.Manifest.Files[0].Chunks {
		plain, err := filecas.OpenChunk(dek, streamComputer, d, tape.chunks[d])
		if err != nil {
			t.Fatal(err)
		}
		_, _ = digest.Write(plain)
	}
	rec.Version, rec.WitnessRoot, rec.SchemaHash, rec.PageDigest = snapshot.VersionPageDigest, "", "", digest.Sum()
	if mutate != nil {
		mutate(&rec)
	}
	plaintext, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	tape.mu.Lock()
	tape.events, tape.payloads = nil, map[string][]byte{}
	tape.mu.Unlock()
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: "event-v2", ComputerID: streamComputer,
		EventKind: computerevent.EventSnapshotRecorded, PrivacyClass: "private",
	}
	if _, _, err := tape.AppendNewPrivatePayload(context.Background(), event, computerevent.TransitionInput{}, plaintext, snapshot.RecordMediaType, cipher); err != nil {
		t.Fatal(err)
	}
}

func installV2(t *testing.T, tape *streamTape, cipher *computerevent.PrivateArtifactCipher) (snapshot.Record, string, error) {
	t.Helper()
	dest := filepath.Join(t.TempDir(), "state.db")
	rec, err := snapshot.Restore(context.Background(), snapshot.RestoreConfig{
		ComputerID: streamComputer, DestPath: dest, Source: tape, Chunks: tape, Cipher: cipher,
		Verify: VerifySnapshotFile,
	})
	return rec, dest, err
}

// H1, H2
func TestVersionTwoRecordIsVerifiedByItsHeadRowAndNotByTheWitness(t *testing.T) {
	store := newStreamStore(t, 900)
	tape := newStreamTape()
	cipher := streamCipher(t)
	ref, err := snapshot.TakeSnapshot(context.Background(), streamConfig(t, store, tape, cipher))
	if err != nil {
		t.Fatal(err)
	}
	if ref.Record.StoreHeadSequence != 900 {
		t.Fatalf("test premise: head %d", ref.Record.StoreHeadSequence)
	}

	v2Of(t, tape, cipher, ref, nil)
	rec, dest, err := installV2(t, tape, cipher)
	if err != nil {
		t.Fatalf("a version 2 record with the file's own head: %v", err)
	}
	if rec.Version != snapshot.VersionPageDigest || rec.WitnessRoot != "" || rec.SchemaHash != "" {
		t.Fatalf("restored %+v", rec)
	}
	restored, err := dbx.OpenSQLite(dest, dbx.SQLiteOptions{ReadConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var seq uint64
	var head string
	if err := restored.Read.QueryRow(`SELECT sequence, canonical_event_head FROM computer_event_projection_heads WHERE computer_id=?`, streamComputer).Scan(&seq, &head); err != nil {
		t.Fatal(err)
	}
	if seq != rec.StoreHeadSequence || head != rec.StoreHead {
		t.Fatalf("restored head %d %s, record %d %s", seq, head, rec.StoreHeadSequence, rec.StoreHead)
	}

	for name, mutate := range map[string]func(*snapshot.Record){
		"another sequence": func(r *snapshot.Record) { r.StoreHeadSequence++ },
		"another head":     func(r *snapshot.Record) { r.StoreHead = streamHead(7) },
		"an empty head":    func(r *snapshot.Record) { r.StoreHeadSequence, r.StoreHead = 0, "" },
	} {
		t.Run(name, func(t *testing.T) {
			v2Of(t, tape, cipher, ref, mutate)
			_, dest, err := installV2(t, tape, cipher)
			if err == nil || !strings.Contains(err.Error(), "head") {
				t.Fatalf("a record naming %s restored or was refused for another reason: %v", name, err)
			}
			if _, statErr := os.Lstat(dest); statErr == nil {
				t.Fatal("a refused restore left a state file")
			}
		})
	}
}

// H3: the version 1 form is still checked against its witness.
func TestVersionOneRecordIsStillVerifiedByItsWitness(t *testing.T) {
	store := newStreamStore(t, 300)
	tape := newStreamTape()
	cipher := streamCipher(t)
	ref, err := snapshot.TakeSnapshot(context.Background(), streamConfig(t, store, tape, cipher))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := installV2(t, tape, cipher); err != nil {
		t.Fatalf("version 1: %v", err)
	}
	// A record naming another witness refuses.
	v2Of(t, tape, cipher, ref, func(r *snapshot.Record) {
		r.Version, r.PageDigest = snapshot.Version, ""
		r.WitnessRoot, r.SchemaHash = strings.Repeat("a", 64), ref.Record.SchemaHash
	})
	if _, _, err := installV2(t, tape, cipher); err == nil || !strings.Contains(err.Error(), "witness root") {
		t.Fatalf("a version 1 record with another witness restored or failed otherwise: %v", err)
	}
}

// A restored file with no head row: a record that names a head is refused, and
// a genesis record (sequence 0, empty head) is accepted. The file is a real
// store whose projection head row was removed before the capture.
func TestVersionTwoRecordAgainstAFileWithNoHeadRow(t *testing.T) {
	store := newStreamStore(t, 200)
	if _, err := store.DB().Exec(`DELETE FROM computer_event_projection_heads WHERE computer_id=?`, streamComputer); err != nil {
		t.Fatal(err)
	}
	tape := newStreamTape()
	cipher := streamCipher(t)
	ref, err := snapshot.TakeSnapshot(context.Background(), streamConfig(t, store, tape, cipher))
	if err != nil {
		t.Fatal(err)
	}
	if ref.Record.StoreHeadSequence != 0 || ref.Record.StoreHead != "" {
		t.Fatalf("test premise: the capture of a store with no head row names head %d (%s)", ref.Record.StoreHeadSequence, ref.Record.StoreHead)
	}
	v2Of(t, tape, cipher, ref, nil)
	if rec, _, err := installV2(t, tape, cipher); err != nil || rec.StoreHeadSequence != 0 {
		t.Fatalf("the genesis record of a file with no head row: %+v, %v", rec, err)
	}
	v2Of(t, tape, cipher, ref, func(r *snapshot.Record) { r.StoreHeadSequence, r.StoreHead = 5, streamHead(5) })
	if _, _, err := installV2(t, tape, cipher); err == nil || !strings.Contains(err.Error(), "no head row") {
		t.Fatalf("a record naming a head against a file with no head row: %v", err)
	}
}

// Pages that match the digest but are not a database: the digest holds, the
// install's Verify opens the file and refuses it, and no state file is left.
// (No logical read of the file happens before that; the open is the check.)
func TestVersionTwoRecordWhosePagesAreNotADatabaseIsRefusedByTheOpen(t *testing.T) {
	cipher := streamCipher(t)
	tape := newStreamTape()
	dek, err := cipher.ExportKeyForEscrow(context.Background(), streamComputer)
	if err != nil {
		t.Fatal(err)
	}
	const pageSize, pageCount = 4096, 8
	plain := make([]byte, pageSize*pageCount)
	if _, err := rand.Read(plain); err != nil {
		t.Fatal(err)
	}
	sealed, digest, err := filecas.SealChunk(dek, streamComputer, plain)
	if err != nil {
		t.Fatal(err)
	}
	if err := tape.PutChunk(context.Background(), digest, sealed); err != nil {
		t.Fatal(err)
	}
	manifest, err := filecas.BuildManifest(streamComputer, []filecas.FileEntry{{Path: snapshot.StateFileName, Mode: 0o600, Size: int64(len(plain)), Chunks: []string{digest}}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	pd := snapshot.NewPageDigest(pageSize, pageCount)
	_, _ = pd.Write(plain)
	rec := snapshot.Record{
		Version: snapshot.VersionPageDigest, ComputerID: streamComputer, PlaintextSize: int64(len(plain)),
		PageSize: pageSize, PageCount: pageCount, PagesPerChunk: snapshot.ChunkSize / pageSize,
		PageDigest: pd.Sum(), Manifest: *manifest,
	}
	plaintext, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: "event-garbage", ComputerID: streamComputer,
		EventKind: computerevent.EventSnapshotRecorded, PrivacyClass: "private",
	}
	if _, _, err := tape.AppendNewPrivatePayload(context.Background(), event, computerevent.TransitionInput{}, plaintext, snapshot.RecordMediaType, cipher); err != nil {
		t.Fatal(err)
	}
	_, dest, err := installV2(t, tape, cipher)
	if err == nil || !strings.Contains(err.Error(), "verify:") {
		t.Fatalf("pages that digest correctly but are not a database: %v", err)
	}
	if strings.Contains(err.Error(), "page digest") {
		t.Fatalf("the digest should have held: %v", err)
	}
	if _, statErr := os.Lstat(dest); statErr == nil {
		t.Fatal("the refused install left a state file")
	}
}
