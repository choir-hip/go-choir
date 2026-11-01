package computerevent

import (
	"bytes"
	"context"
	"encoding/base64"
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
