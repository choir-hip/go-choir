package computerevent

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
)

// Ways the no-change stand-in could fail, written before the code:
//
//	U1. a broken sealed batch still bricks replay: it finalizes as a batch
//	    with no ops and replay goes on;
//	U2. it hides a real defect: only a JSON syntax error in a private,
//	    digest-checked, authenticated payload qualifies; a public payload or
//	    a batch that parses but fails validation still fails;
//	U3. a live writer thinks its ops landed: finalize reports
//	    ErrProjectionBatchUnreadable after the head advances.
func TestSealedBatchThatDoesNotParseFinalizesAsNoChange(t *testing.T) {
	cipher, err := newPrivateArtifactCipher(testComputerID, base64.RawStdEncoding.EncodeToString(bytes.Repeat([]byte{0x55}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	eventID, err := NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	reader := NewMemoryArtifactReader()
	seal := func(plain string) string {
		envelope, _, err := cipher.Encrypt(context.Background(), testComputerID, eventID, ProjectionBatchMediaType, "private", []byte(plain))
		if err != nil {
			t.Fatal(err)
		}
		return reader.Put(envelope)
	}
	run := func(privacy, digest string) (*unreadableProbe, error) {
		probe := &unreadableProbe{}
		a := &ComputerEventAppender{computerID: testComputerID, projection: probe, replayProjection: true}
		a.SetPayloadResolver(reader, cipher)
		event := Event{EventKind: EventProjectionBatchRecorded, ComputerID: testComputerID, EventID: eventID, Sequence: 7, PayloadCommitment: digest, PrivacyClass: privacy}
		return probe, a.finalizeProjection(context.Background(), event, testDigestA, Receipt{})
	}

	// U1, U3: the shape the paper's sequence 5647041 holds.
	broken := seal(`{"version":2,"projector_version":2,"computer_id":"` + testComputerID + `","event_id":"` + eventID + `","ops":[{"kind":"x","body":{"secret-handle:v1:credential_assignment:00,"y":")"}}]}`)
	probe, err := run("private", broken)
	if !errors.Is(err, ErrProjectionBatchUnreadable) {
		t.Fatalf("finalize = %v, want ErrProjectionBatchUnreadable", err)
	}
	if probe.batch == nil || !probe.batch.Unreadable || len(probe.batch.Ops) != 0 || probe.batch.EventDigest != testDigestA {
		t.Fatalf("replay finalized %+v, want an empty unreadable stand-in", probe.batch)
	}
	if err := probe.batch.Validate(); err != nil {
		t.Fatalf("stand-in does not validate: %v", err)
	}

	// U2: a batch that parses and fails validation is still refused.
	empty := seal(`{"version":2,"projector_version":2,"computer_id":"` + testComputerID + `","event_id":"` + eventID + `","ops":[]}`)
	if _, err := run("private", empty); err == nil || errors.Is(err, ErrProjectionBatchUnreadable) {
		t.Fatalf("empty sealed batch finalize = %v, want a validation failure", err)
	}
	// U2: a public payload that does not parse is still refused.
	public := reader.Put([]byte(`{"ops":[)]}`))
	if _, err := run("public", public); err == nil || errors.Is(err, ErrProjectionBatchUnreadable) {
		t.Fatalf("public broken batch finalize = %v, want a decode failure", err)
	}
}

type unreadableProbe struct{ batch *ProjectionBatch }

func (p *unreadableProbe) Head(context.Context, string) (*Head, error)             { return nil, nil }
func (p *unreadableProbe) Prepare(context.Context, CASRequest) error               { return nil }
func (p *unreadableProbe) Prepared(context.Context, string) ([]CASRequest, error)  { return nil, nil }
func (p *unreadableProbe) Finalize(context.Context, string, string, Receipt) error { return nil }
func (p *unreadableProbe) DiscardPrepared(context.Context, string, string) error   { return nil }
func (p *unreadableProbe) FinalizeBatch(context.Context, string, string, Receipt, *ProjectionBatch) error {
	return errors.New("live finalizer used during replay")
}
func (p *unreadableProbe) FinalizeReplayBatch(_ context.Context, _ string, _ string, _ Receipt, batch *ProjectionBatch) error {
	if err := batch.Validate(); err != nil {
		return err
	}
	p.batch = batch
	return nil
}

// The dry run that guards a sealed batch reads the envelope, not the bytes
// handed in: a batch redaction changed is judged as finalize will read it.
func TestPrivateBatchDryRunReadsTheSealedBytes(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer := SigningKey{SignerRef: SignerRef{SignerDomain: "platform-control", KeyID: "platform-1"}, PrivateKey: privateKey}
	runner := &sealedDryRunner{memoryProjection: &memoryProjection{}}
	cas := &memoryCAS{signer: signer}
	appender, err := NewComputerEventAppender(testComputerID, memoryPinner{signer: signer}, runner, cas, EventHeadReceiptVerifier{Keys: staticKeyResolver{key: publicKey}})
	if err != nil {
		t.Fatal(err)
	}
	genesis := testEvent(t, nil, EventGenesisImported)
	genesis.ResultingEffectiveCommitment = testDigestA
	if _, err := appender.AppendNew(context.Background(), genesis, TransitionInput{TargetStateCommitment: testDigestA}, nil); err != nil {
		t.Fatal(err)
	}
	cipher, err := newPrivateArtifactCipher(testComputerID, base64.RawStdEncoding.EncodeToString(bytes.Repeat([]byte{0x66}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	event := testEvent(t, nil, EventProjectionBatchRecorded)
	batch := ProjectionBatch{Version: ProjectionBatchV2, ProjectorVersion: ProjectorVersionV2, ComputerID: testComputerID, EventID: event.EventID,
		Ops: []ProjectionOp{{Kind: ProjectionOpDesktopState, Body: json.RawMessage(`{"auth_token":"abcdefghijklmnop"}`)}}}
	payload, err := json.Marshal(batch)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = appender.AppendNewPrivatePayload(context.Background(), event, TransitionInput{}, payload, ProjectionBatchMediaType, cipher)
	if !errors.Is(err, ErrBatchRefused) {
		t.Fatalf("append = %v, want the sealed batch refused", err)
	}
	if !runner.sawHandle || len(cas.records) != 1 {
		t.Fatalf("dry run saw a handle=%v, records=%d; want the sealed bytes judged and nothing appended", runner.sawHandle, len(cas.records))
	}
}

type sealedDryRunner struct {
	*memoryProjection
	sawHandle bool
}

func (r *sealedDryRunner) DryRunProjectionBatch(_ context.Context, _ string, batch *ProjectionBatch) error {
	for _, op := range batch.Ops {
		if bytes.Contains(op.Body, []byte("secret-handle:")) {
			r.sawHandle = true
			return errors.New("op body changed by redaction")
		}
	}
	return nil
}
