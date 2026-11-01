package platform

// Failure modes for sealed projection batches on the segment write path,
// written before the fix. Mutation class red: tape/evidence storage.
//
// A sealed batch reaches the host as a private envelope: the media type is
// PrivateArtifactMediaType, the inner media type (ProjectionBatchMediaType) is
// in the authenticated envelope metadata. Before the fix the host tested only
// the outer media type, so every sealed batch payload and its pin receipt was
// written as a loose file (388,985 files in 24 h on Node B).
//
//  S1  Flag write: a sealed batch payload and both receipts are records of the
//      computer's segment; no loose payload or receipt file exists. The
//      reader, FetchPayload and the head CAS pin check (ValidateEventPins,
//      which opens the envelope metadata) accept the chain, the stored bytes
//      are the envelope bytes unchanged, the receipt still says private, and
//      the guest cipher still opens the segment-resident envelope.
//  S2  An envelope of another inner media type is segmented too since
//      2026-11-01 (computerevent.TapeSegmentEligible: every payload but a
//      public non-batch one); it stayed loose while only batches were
//      eligible (section 4.2).
//  S3  Refusals are unchanged and write nothing: private class with a
//      non-envelope media type, an envelope of another computer, a malformed
//      envelope. An envelope under the owner class is never inspected and is
//      segmented like any owner payload.
//  S4  Collisions: the same envelope bytes already loose is a no-op for the
//      payload (mtime refreshed, no segment copy); the same envelope pinned
//      twice writes one payload record; different bytes under a loose digest
//      are an error and append nothing.
//  S5  A payload pinned loose before the fix (flag off here stands for the old
//      binary) still resolves after the flag is write, and a segmented one
//      still resolves after the flag is turned back off: the read path never
//      depends on the flag or on where the bytes are (mixed old and new, host
//      rollback).
//  S6  Purge and sweep: P13 in tape_purge_test.go moves a chain's payloads to
//      segments and proves the purge removes tape-seg/<id> wholesale (the
//      crypto-shred of the key escrow is independent of the layout); a
//      segmented payload has no loose file for the orphan sweep to list.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

type privateChain struct {
	pinnedChain
	envelope []byte
	cipher   *computerevent.PrivateArtifactCipher
}

func testCipher(t *testing.T, computerID string) *computerevent.PrivateArtifactCipher {
	t.Helper()
	cipher, err := computerevent.NewPrivateArtifactCipher(computerID, bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	return cipher
}

// pinPrivateChain seals plaintext as innerMediaType and pins the envelope and
// then a private projection_batch_recorded event naming it.
func (f *writeFixture) pinPrivateChain(t *testing.T, computerID, innerMediaType string, plaintext []byte) privateChain {
	t.Helper()
	cipher := testCipher(t, computerID)
	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	envelope, _, err := cipher.Encrypt(context.Background(), computerID, eventID, innerMediaType, "private", plaintext)
	if err != nil {
		t.Fatal(err)
	}
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: computerID, Sequence: 1,
		PreviousHead: computerevent.ZeroHead, EventKind: computerevent.EventProjectionBatchRecorded,
		OccurredAt: time.Date(2026, 7, 18, 22, 0, 0, 0, time.UTC).Format(time.RFC3339Nano), IdempotencyKey: "k-" + eventID,
		ActorProfile: "trusted-core", AuthorityRef: "authority:test",
		InputArtifactRefs: []string{"artifact:sha256:" + computerevent.DigestBytes(envelope)},
		PayloadCommitment: platformTestDigest('a'), PrivacyClass: "private", ReducerVersion: computerevent.ReducerVersionV1,
		ExpectedDesiredEventHead: computerevent.ZeroHead, ExpectedEffectiveEventHead: computerevent.ZeroHead,
		ExpectedDesiredStateCommitment: computerevent.ZeroHead, ExpectedEffectiveStateCommitment: computerevent.ZeroHead,
		ResultingEffectiveCommitment: platformTestDigest('b'),
	}
	input := computerevent.TransitionInput{TargetStateCommitment: platformTestDigest('b')}
	pinIntent, err := computerevent.ComputePinIntentCommitment(event, input)
	if err != nil {
		t.Fatal(err)
	}
	payloadPin, err := f.artifacts.pinPayload(context.Background(), computerID, envelope, computerevent.PrivateArtifactMediaType, "private", pinIntent)
	if err != nil {
		t.Fatalf("pin payload: %v", err)
	}
	payloadReceipt, _ := payloadPin.Receipt.CanonicalBytes()
	event.RequestCommitment, err = computerevent.ComputeRequestCommitment(event, input, pinIntent, []string{computerevent.DigestBytes(payloadReceipt)})
	if err != nil {
		t.Fatal(err)
	}
	eventJSON, _ := event.CanonicalBytes()
	eventPin, err := f.artifacts.PinEvent(context.Background(), computerID, eventJSON, event.RequestCommitment)
	if err != nil {
		t.Fatalf("pin event: %v", err)
	}
	eventReceipt, _ := eventPin.Receipt.CanonicalBytes()
	return privateChain{
		pinnedChain: pinnedChain{
			request: computerevent.CASRequest{
				Event: event, EventDigest: eventPin.ArtifactDigest, EventArtifactDigest: eventPin.ArtifactDigest,
				EventPinReceiptDigest: computerevent.DigestBytes(eventReceipt), PayloadPinReceiptDigests: []string{computerevent.DigestBytes(payloadReceipt)},
				PinIntentCommitment: pinIntent, Input: input,
			},
			eventJSON: eventJSON, payload: envelope, eventReceipt: eventReceipt, payloadReceip: payloadReceipt,
		},
		envelope: envelope, cipher: cipher,
	}
}

// S1
func TestSealedProjectionBatchPayloadIsSegmented(t *testing.T) {
	f := newWriteFixture(t, "write", true)
	const id = "computer-sealed-batch"
	chain := f.pinPrivateChain(t, id, computerevent.ProjectionBatchMediaType, projectionPayload("sealed"))
	if got := legacyTree(t, f.root); len(got) != 0 {
		t.Fatalf("loose files written for a sealed batch: %v", treeKeys(got))
	}
	if records := segmentRecords(t, f.root, id); len(records) != 4 {
		t.Fatalf("segment records = %v, want the event, the envelope and two receipts", records)
	}
	reader := f.service.tapeReader()
	for _, it := range chain.items() {
		got, src, err := reader.Read(id, it.ns, it.digest)
		if err != nil || src != tapestore.SourceSegment || !bytes.Equal(got, chain.bytesOf(it)) {
			t.Fatalf("%s %s: src %v err %v", it.ns, it.digest, src, err)
		}
	}
	got, err := f.artifacts.FetchPayload(id, computerevent.DigestBytes(chain.envelope))
	if err != nil || !bytes.Equal(got, chain.envelope) {
		t.Fatalf("FetchPayload: %v", err)
	}
	if err := f.artifacts.ValidateEventPins(context.Background(), chain.request); err != nil {
		t.Fatalf("head CAS pin check refused a segment-resident sealed chain: %v", err)
	}
	// Privacy is not lost: the receipt names the private class and envelope
	// media type, and the guest cipher opens the segment-resident bytes.
	if !bytes.Contains(chain.payloadReceip, []byte(`"privacy_class":"private"`)) || !bytes.Contains(chain.payloadReceip, []byte(computerevent.PrivateArtifactMediaType)) {
		t.Fatalf("payload receipt lost the private class: %s", chain.payloadReceip)
	}
	plain, _, err := chain.cipher.Decrypt(context.Background(), got, id, chain.request.Event.EventID)
	if err != nil || !bytes.Equal(plain, projectionPayload("sealed")) {
		t.Fatalf("decrypt of the segment-resident envelope: %v", err)
	}
	if n := f.service.TapeDiagnostics().LegacyPins; n != 0 {
		t.Fatalf("legacy pins = %d, want 0", n)
	}
}

// S2
func TestSealedNonBatchEnvelopeIsSegmented(t *testing.T) {
	f := newWriteFixture(t, "write", true)
	const id = "computer-sealed-other"
	chain := f.pinPrivateChain(t, id, "application/octet-stream", []byte("a sealed ordinary payload"))
	if legacy := legacyTree(t, f.root); len(legacy) != 0 {
		t.Fatalf("loose files = %v, want none: every kind is segmented", treeKeys(legacy))
	}
	records := segmentRecords(t, f.root, id)
	if records[segItem{tapestore.NSPayload, computerevent.DigestBytes(chain.envelope)}] != 1 {
		t.Fatalf("segment records = %v, want the sealed envelope", records)
	}
	if err := f.artifacts.ValidateEventPins(context.Background(), chain.request); err != nil {
		t.Fatal(err)
	}
}

// S3
func TestSealedPinRefusalsWriteNothing(t *testing.T) {
	f := newWriteFixture(t, "write", true)
	const id = "computer-sealed-refuse"
	cipher := testCipher(t, id)
	envelope, _, err := cipher.Encrypt(context.Background(), id, "event-1", computerevent.ProjectionBatchMediaType, "private", projectionPayload("x"))
	if err != nil {
		t.Fatal(err)
	}
	pin := func(computer string, payload []byte, mediaType, class string) error {
		_, err := f.artifacts.pinPayload(context.Background(), computer, payload, mediaType, class, platformTestDigest('c'))
		return err
	}
	if err := pin(id, projectionPayload("x"), computerevent.ProjectionBatchMediaType, "private"); err == nil {
		t.Fatal("private class with a plain media type accepted")
	}
	if err := pin("computer-someone-else", envelope, computerevent.PrivateArtifactMediaType, "private"); err == nil {
		t.Fatal("envelope of another computer accepted")
	}
	if err := pin(id, []byte(`{"metadata":{}}`), computerevent.PrivateArtifactMediaType, "private"); err == nil {
		t.Fatal("malformed envelope accepted")
	}
	if got := legacyTree(t, f.root); len(got) != 0 {
		t.Fatalf("a refusal wrote loose files: %v", treeKeys(got))
	}
	requireNoSegments(t, f.root)
	// An envelope under a non-private class is not inspected; it is
	// segmented like any other payload.
	if err := pin(id, envelope, computerevent.PrivateArtifactMediaType, "owner"); err != nil {
		t.Fatal(err)
	}
	if n := len(legacyTree(t, f.root)); n != 0 {
		t.Fatalf("an envelope under the owner class wrote %d loose files, want 0", n)
	}
	if records := segmentRecords(t, f.root, id); len(records) != 2 {
		t.Fatalf("segment records = %v, want the envelope and its receipt", records)
	}
}

// S4
func TestSealedBatchCollisions(t *testing.T) {
	const id = "computer-sealed-collide"
	seal := func(label string) []byte {
		env, _, err := testCipher(t, id).Encrypt(context.Background(), id, "event-1", computerevent.ProjectionBatchMediaType, "private", projectionPayload(label))
		if err != nil {
			t.Fatal(err)
		}
		return env
	}
	pin := func(f *writeFixture, env []byte) error {
		_, err := f.artifacts.pinPayload(context.Background(), id, env, computerevent.PrivateArtifactMediaType, "private", platformTestDigest('c'))
		return err
	}
	t.Run("same bytes already loose is a no-op for the payload", func(t *testing.T) {
		f := newWriteFixture(t, "write", true)
		env := seal("loose first")
		digest := computerevent.DigestBytes(env)
		path := filepath.Join(f.root, "sha256", "computer-event-payload", digest)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, env, 0o644); err != nil {
			t.Fatal(err)
		}
		old := time.Now().Add(-48 * time.Hour)
		_ = os.Chtimes(path, old, old)
		if err := pin(f, env); err != nil {
			t.Fatal(err)
		}
		if records := segmentRecords(t, f.root, id); records[segItem{tapestore.NSPayload, digest}] != 0 {
			t.Fatalf("payload copied into a segment beside its loose file: %v", records)
		}
		if info, err := os.Stat(path); err != nil || time.Since(info.ModTime()) > time.Hour {
			t.Fatalf("mtime not refreshed: %v %v", info, err)
		}
	})
	t.Run("same bytes pinned twice is one record", func(t *testing.T) {
		f := newWriteFixture(t, "write", true)
		env := seal("twice")
		for i := 0; i < 2; i++ {
			if err := pin(f, env); err != nil {
				t.Fatal(err)
			}
		}
		if records := segmentRecords(t, f.root, id); records[segItem{tapestore.NSPayload, computerevent.DigestBytes(env)}] != 1 {
			t.Fatalf("records = %v", records)
		}
	})
	t.Run("different bytes under a loose digest is a collision", func(t *testing.T) {
		f := newWriteFixture(t, "write", true)
		env := seal("collide")
		path := filepath.Join(f.root, "sha256", "computer-event-payload", computerevent.DigestBytes(env))
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		_ = os.WriteFile(path, []byte("not the envelope"), 0o644)
		if err := pin(f, env); err == nil {
			t.Fatal("collision accepted")
		}
		if records := segmentRecords(t, f.root, id); len(records) != 0 {
			t.Fatalf("a refused collision appended %v", records)
		}
	})
}

// S5
func TestSealedBatchMixedOldAndNewAndRollback(t *testing.T) {
	f := newWriteFixture(t, "off", true)
	const id = "computer-sealed-mixed"
	oldChain := f.pinPrivateChain(t, id, computerevent.ProjectionBatchMediaType, projectionPayload("old"))
	requireLegacyExactly(t, f.root, oldChain.legacyFiles())
	requireNoSegments(t, f.root)

	f.service.SetTapeSegmentsMode("write")
	newChain := f.pinPrivateChain(t, id, computerevent.ProjectionBatchMediaType, projectionPayload("new"))
	for _, c := range []privateChain{oldChain, newChain} {
		got, err := f.artifacts.FetchPayload(id, computerevent.DigestBytes(c.envelope))
		if err != nil || !bytes.Equal(got, c.envelope) {
			t.Fatalf("FetchPayload: %v", err)
		}
		if err := f.artifacts.ValidateEventPins(context.Background(), c.request); err != nil {
			t.Fatal(err)
		}
	}
	if _, found, err := f.service.tapeReader().ReadSegment(id, tapestore.NSPayload, computerevent.DigestBytes(newChain.envelope)); err != nil || !found {
		t.Fatalf("new envelope not in a segment: %v %v", found, err)
	}

	f.service.SetTapeSegmentsMode("off")
	if got, err := f.artifacts.FetchPayload(id, computerevent.DigestBytes(newChain.envelope)); err != nil || !bytes.Equal(got, newChain.envelope) {
		t.Fatalf("segmented envelope unreadable after the flag went off: %v", err)
	}
	if err := f.artifacts.ValidateEventPins(context.Background(), newChain.request); err != nil {
		t.Fatal(err)
	}
}
