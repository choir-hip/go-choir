package platform

// The pin rule, computerevent.TapeSegmentEligible, at the pin path with
// PLATFORM_TAPE_SEGMENTS=write (widened from projection batches alone on
// 2026-11-01; taperepack deletes by the same rule). Written for review
// ae299a3cc, both seats, finding 1.
//
//  R1  A public non-batch payload (the capsule effect bundle a
//      self-development route applies) stays a loose file with its receipt,
//      and vmctl's LocalArtifactContentVerifier resolves the URI the
//      materializer builds for it (artifact+sha256://<d>/sha256/
//      computer-event-payload/<d>) from that file. Segmenting it broke route
//      apply: the verifier opens the loose path and has no computer id to
//      name a segment.
//  R2  An owner-class payload and a private (sealed) payload of another
//      media type than a projection batch are segment records, with their
//      receipts, and no loose file is written for either.

import (
	"bytes"
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

// R1
func TestTapeSegmentsWritePublicNonBatchPayloadStaysLooseForTheURIVerifier(t *testing.T) {
	f := newWriteFixture(t, "write", true)
	const id = "computer-capsule-bundle"
	bundle := []byte(`{"capsule_effect_bundle":"r1"}`)
	chain := f.pinChain(t, id, computerevent.EventEffectProposed, "application/vnd.choir.capsule-effect+json", bundle)
	digest := computerevent.DigestBytes(bundle)
	legacy := legacyTree(t, f.root)
	if len(legacy) != 2 || !bytes.Equal(legacy["sha256/computer-event-payload/"+digest], bundle) || legacy["sha256/pin-receipts/"+chain.request.PayloadPinReceiptDigests[0]+".json"] == nil {
		t.Fatalf("loose files = %v, want the public bundle and its receipt", treeKeys(legacy))
	}
	if records := segmentRecords(t, f.root, id); records[segItem{tapestore.NSPayload, digest}] != 0 || records[segItem{tapestore.NSEvent, chain.request.EventDigest}] != 1 {
		t.Fatalf("segment records = %v, want the event and not the bundle", records)
	}
	// The URI shape of agentcore/self_development_materializer.go.
	uri := "artifact+sha256://" + digest + "/sha256/computer-event-payload/" + digest
	if err := computerversion.NewLocalArtifactContentVerifier(f.root).VerifyArtifact(context.Background(), uri, digest); err != nil {
		t.Fatalf("vmctl's verifier does not resolve the capsule bundle URI with segments on: %v", err)
	}
	if err := f.artifacts.ValidateEventPins(context.Background(), chain.request); err != nil {
		t.Fatal(err)
	}
}

// R2
func TestTapeSegmentsWriteOwnerAndPrivateNonBatchPayloadsAreSegmented(t *testing.T) {
	f := newWriteFixture(t, "write", true)
	const ownerID = "computer-owner-payload"
	owner := f.pinChainClass(t, ownerID, computerevent.EventGenesisImported, "application/json", "owner", []byte(`{"owner":"r2"}`))
	const privateID = "computer-private-payload"
	sealed := f.pinPrivateChain(t, privateID, "application/octet-stream", []byte("a sealed ordinary payload"))
	if legacy := legacyTree(t, f.root); len(legacy) != 0 {
		t.Fatalf("loose files = %v, want none", treeKeys(legacy))
	}
	for _, c := range []struct {
		id      string
		payload []byte
		receipt string
	}{{ownerID, owner.payload, owner.request.PayloadPinReceiptDigests[0]}, {privateID, sealed.envelope, sealed.request.PayloadPinReceiptDigests[0]}} {
		records := segmentRecords(t, f.root, c.id)
		if records[segItem{tapestore.NSPayload, computerevent.DigestBytes(c.payload)}] != 1 || records[segItem{tapestore.NSReceipt, c.receipt}] != 1 {
			t.Fatalf("%s: segment records = %v, want the payload and its receipt", c.id, records)
		}
		if got, err := f.artifacts.FetchPayload(c.id, computerevent.DigestBytes(c.payload)); err != nil || !bytes.Equal(got, c.payload) {
			t.Fatalf("%s: FetchPayload: %v", c.id, err)
		}
	}
	for _, r := range []computerevent.CASRequest{owner.request, sealed.request} {
		if err := f.artifacts.ValidateEventPins(context.Background(), r); err != nil {
			t.Fatal(err)
		}
	}
}
