package platform

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

// A1 wiring tests: every platform tape read resolves segment first and
// legacy second, fails closed on a corrupt segment, keeps the tombstone gate
// first, and the sweeps never lose a legacy-only file. The numbered failure
// modes are section 9.4 of
// docs/reports/tape-volume-forward-fix-and-surgery-2026-10-19.md. No
// production code writes a segment; these tests build them with the fixture
// writer.

type segItem struct {
	ns     tapestore.NS
	digest string
}

func tapeNS(ns string) tapestore.NS {
	switch ns {
	case tapeNSEvent:
		return tapestore.NSEvent
	case tapeNSPayload:
		return tapestore.NSPayload
	default:
		return tapestore.NSReceipt
	}
}

// segmentize copies legacy tape files into segment segno of the computer and,
// unless keepLegacy, deletes the legacy files so the segment is the only copy.
func segmentize(t *testing.T, artifactsRoot, computerID string, segno uint64, seal, keepLegacy bool, items ...segItem) {
	t.Helper()
	w, err := tapestore.CreateSegment(artifactsRoot, computerID, segno)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		legacy := filepath.Join(artifactsRoot, filepath.FromSlash(it.ns.LegacyRel(it.digest)))
		raw, err := os.ReadFile(legacy)
		if err != nil {
			t.Fatalf("segmentize %s %s: %v", it.ns, it.digest, err)
		}
		if got, err := w.Append(it.ns, raw); err != nil || got != it.digest {
			t.Fatalf("segmentize append: %v (digest %s want %s)", err, got, it.digest)
		}
	}
	if seal {
		err = w.Seal()
	} else {
		err = w.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	if !keepLegacy {
		for _, it := range items {
			if err := os.Remove(filepath.Join(artifactsRoot, filepath.FromSlash(it.ns.LegacyRel(it.digest)))); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// corruptSegments flips a byte in the body of the first record of every
// segment file of the computer.
func corruptSegments(t *testing.T, artifactsRoot, computerID string) {
	t.Helper()
	dir := filepath.Join(artifactsRoot, tapestore.SegDirName, computerID)
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("no segments to corrupt: %v", err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".seg") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		raw[32+46+3] ^= 0x55 // 32-byte segment header, 46-byte record header
		if err := os.WriteFile(p, raw, 0o640); err != nil {
			t.Fatal(err)
		}
	}
}

func segmentFilesExist(t *testing.T, artifactsRoot, computerID string) int {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(artifactsRoot, tapestore.SegDirName, computerID))
	if err != nil {
		return 0
	}
	return len(entries)
}

// FM15, FM16, FM17: replay resolves events from segments, legacy and both,
// identically; a corrupt segment is an error naming the sequence (never a
// silent legacy answer); a tombstone is checked before any segment is opened.
func TestSegmentReadsEventsPage(t *testing.T) {
	store, root := openTestPlatformStore(t)
	service := NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "platform-signing.key"))
	artifacts, err := NewEventArtifactService(service, platformTestKeyResolver{key: service.signingKey.Public})
	if err != nil {
		t.Fatal(err)
	}
	const id = "computer-seg-replay"
	insertReplayTestChain(t, store, service, id, []uint64{1, 2, 3, 4, 5}, 5)
	baseline, err := artifacts.EventsPage(context.Background(), id, 0, 100)
	if err != nil || len(baseline) != 5 {
		t.Fatalf("legacy baseline: %d records, %v", len(baseline), err)
	}
	ev := func(i int) segItem { return segItem{tapestore.NSEvent, baseline[i].Request.EventDigest} }
	// 1 and 2: segment only. 3: legacy only. 4: both. 5: segment only, in a
	// second, unsealed (open) segment.
	segmentize(t, service.artifactsRoot, id, 1, true, false, ev(0), ev(1))
	segmentize(t, service.artifactsRoot, id, 2, true, true, ev(3))
	segmentize(t, service.artifactsRoot, id, 3, false, false, ev(4))
	for _, i := range []int{0, 1, 4} {
		if _, err := os.Stat(filepath.Join(service.artifactsRoot, "sha256", "computer-event", baseline[i].Request.EventDigest)); !os.IsNotExist(err) {
			t.Fatalf("legacy event %d still present", i)
		}
	}
	mixed, err := artifacts.EventsPage(context.Background(), id, 0, 100)
	if err != nil {
		t.Fatalf("mixed layouts: %v", err)
	}
	if !reflect.DeepEqual(mixed, baseline) {
		t.Fatal("replay from mixed layouts differs from the all-legacy replay")
	}
	if page, err := artifacts.EventsPage(context.Background(), id, 3, 2); err != nil || len(page) != 2 || page[0].Request.Event.Sequence != 4 {
		t.Fatalf("paged mixed replay: %v %v", page, err)
	}

	// A tombstone wins over the segments, which are not even opened: they are
	// corrupt here, and the error is still the tombstone.
	corruptSegments(t, service.artifactsRoot, id)
	if _, err := artifacts.EventsPage(context.Background(), id, 0, 100); !errors.Is(err, tapestore.ErrCorrupt) || !strings.Contains(err.Error(), "sequence") {
		t.Fatalf("corrupt segment = %v, want ErrCorrupt naming a sequence", err)
	}
	// Sequence 4's event is in both layouts; its segment copy is rotten, so
	// it is served from its digest-verified legacy copy and counted (round 5,
	// choir-tape-segments-a2-repack-2026-10-21.md round_5_design).
	before := service.tapeReader().Stats().SegmentErrorFallbacks
	if page, err := artifacts.EventsPage(context.Background(), id, 3, 1); err != nil || len(page) != 1 || page[0].Request.Event.Sequence != 4 {
		t.Fatalf("event in both layouts with a rotten segment copy = %v, %v, want the legacy copy", page, err)
	}
	if service.tapeReader().Stats().SegmentErrorFallbacks <= before {
		t.Fatal("the legacy fallback was not counted")
	}
	if err := store.putTapeTombstone(context.Background(), TapeTombstone{ComputerID: id, Status: tapeTombstoneArchived, HeadSequence: 5, CanonicalEventHead: platformTestDigest('1'), ManifestRun: "ts-x", ManifestSHA256: platformTestDigest('2'), ArchivePath: "/archive/x.tar.zst", ProofJSON: "{}"}); err != nil {
		t.Fatal(err)
	}
	if _, err := artifacts.EventsPage(context.Background(), id, 0, 100); !errors.Is(err, ErrComputerTapeDeleted) || errors.Is(err, tapestore.ErrCorrupt) {
		t.Fatalf("tombstoned computer with segments = %v, want only ErrComputerTapeDeleted", err)
	}
}

// FM15, FM16, FM17 for FetchPayload.
func TestSegmentReadsFetchPayload(t *testing.T) {
	store, root := openTestPlatformStore(t)
	service := NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "platform-signing.key"))
	artifacts, err := NewEventArtifactService(service, platformTestKeyResolver{key: service.signingKey.Public})
	if err != nil {
		t.Fatal(err)
	}
	const id = "computer-seg-fetch"
	put := func(ns string, data []byte) string {
		d := computerevent.DigestBytes(data)
		if err := service.writeBlob(filepath.Join("sha256", ns, d), data); err != nil {
			t.Fatal(err)
		}
		return d
	}
	segOnly := []byte("payload only in a segment")
	both := []byte("payload in both layouts")
	legacyOnly := []byte("payload only in legacy")
	evInSeg := []byte(`{"an":"event fetched as a payload"}`)
	dSeg, dBoth, dLegacy, dEv := put(tapeNSPayload, segOnly), put(tapeNSPayload, both), put(tapeNSPayload, legacyOnly), put(tapeNSEvent, evInSeg)
	base := []byte("a projection base stays legacy")
	dBase := put("projection-base", base)
	segmentize(t, service.artifactsRoot, id, 1, true, false, segItem{tapestore.NSPayload, dSeg}, segItem{tapestore.NSEvent, dEv})
	segmentize(t, service.artifactsRoot, id, 2, true, true, segItem{tapestore.NSPayload, dBoth})

	for name, c := range map[string]struct {
		digest string
		want   []byte
	}{"segment only": {dSeg, segOnly}, "both": {dBoth, both}, "legacy only": {dLegacy, legacyOnly}, "event namespace in a segment": {dEv, evInSeg}, "projection base": {dBase, base}} {
		got, err := artifacts.FetchPayload(id, c.digest)
		if err != nil || string(got) != string(c.want) {
			t.Fatalf("%s: %q, %v", name, got, err)
		}
	}
	// Another computer cannot read this computer's segment-only payload, but
	// reads legacy and shared files exactly as before.
	if _, err := artifacts.FetchPayload("computer-other", dSeg); err == nil || !strings.Contains(err.Error(), "payload not found") {
		t.Fatalf("segment payload read through another computer: %v", err)
	}
	if got, err := artifacts.FetchPayload("computer-other", dLegacy); err != nil || string(got) != string(legacyOnly) {
		t.Fatalf("legacy payload through another computer: %q, %v", got, err)
	}
	if _, err := artifacts.FetchPayload(id, platformTestDigest('7')); err == nil || !strings.Contains(err.Error(), "payload not found") {
		t.Fatalf("absent payload: %v", err)
	}

	corruptSegments(t, service.artifactsRoot, id)
	// Round 5: the payload in both layouts is served from its verified
	// legacy copy; a segment-only payload behind the rot fails closed.
	if got, err := artifacts.FetchPayload(id, dBoth); err != nil || string(got) != string(both) {
		t.Fatalf("payload in both layouts with a rotten segment = %q, %v, want the legacy copy", got, err)
	}
	if _, err := artifacts.FetchPayload(id, dSeg); !errors.Is(err, tapestore.ErrCorrupt) {
		t.Fatalf("segment-only payload behind a rotten segment = %v, want ErrCorrupt", err)
	}
	if err := store.putTapeTombstone(context.Background(), TapeTombstone{ComputerID: id, Status: tapeTombstoneDeleted, HeadSequence: 1, CanonicalEventHead: platformTestDigest('1'), ManifestRun: "ts-x", ManifestSHA256: platformTestDigest('2'), ProofJSON: "{}"}); err != nil {
		t.Fatal(err)
	}
	if _, err := artifacts.FetchPayload(id, dBoth); !errors.Is(err, ErrComputerTapeDeleted) || errors.Is(err, tapestore.ErrCorrupt) {
		t.Fatalf("tombstoned fetch = %v, want only ErrComputerTapeDeleted", err)
	}
}

// pinnedRequest pins one payload and one event with the production pin path
// and returns the CAS request plus the digests of every artifact the head
// CAS reads back.
func pinnedRequest(t *testing.T, service *Service, artifacts *EventArtifactService, computerID string) (computerevent.CASRequest, []segItem) {
	t.Helper()
	payload := []byte("public observation for " + computerID)
	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: computerID, Sequence: 1,
		PreviousHead: computerevent.ZeroHead, EventKind: computerevent.EventGenesisImported,
		OccurredAt: time.Date(2026, 7, 18, 22, 0, 0, 0, time.UTC).Format(time.RFC3339Nano), IdempotencyKey: "genesis-" + computerID,
		ActorProfile: "trusted-core", AuthorityRef: "authority:test",
		InputArtifactRefs: []string{"artifact:sha256:" + computerevent.DigestBytes(payload)},
		PayloadCommitment: platformTestDigest('a'), PrivacyClass: "public", ReducerVersion: computerevent.ReducerVersionV1,
		ExpectedDesiredEventHead: computerevent.ZeroHead, ExpectedEffectiveEventHead: computerevent.ZeroHead,
		ExpectedDesiredStateCommitment: computerevent.ZeroHead, ExpectedEffectiveStateCommitment: computerevent.ZeroHead,
		ResultingEffectiveCommitment: platformTestDigest('b'),
	}
	input := computerevent.TransitionInput{TargetStateCommitment: platformTestDigest('b')}
	pinIntent, err := computerevent.ComputePinIntentCommitment(event, input)
	if err != nil {
		t.Fatal(err)
	}
	payloadPin, err := artifacts.pinPayload(context.Background(), computerID, payload, "application/octet-stream", "public", pinIntent)
	if err != nil {
		t.Fatal(err)
	}
	payloadReceipt, _ := payloadPin.Receipt.CanonicalBytes()
	payloadReceiptDigest := computerevent.DigestBytes(payloadReceipt)
	event.RequestCommitment, err = computerevent.ComputeRequestCommitment(event, input, pinIntent, []string{payloadReceiptDigest})
	if err != nil {
		t.Fatal(err)
	}
	eventJSON, _ := event.CanonicalBytes()
	eventPin, err := artifacts.PinEvent(context.Background(), computerID, eventJSON, event.RequestCommitment)
	if err != nil {
		t.Fatal(err)
	}
	eventReceipt, _ := eventPin.Receipt.CanonicalBytes()
	eventReceiptDigest := computerevent.DigestBytes(eventReceipt)
	request := computerevent.CASRequest{
		Event: event, EventDigest: eventPin.ArtifactDigest, EventArtifactDigest: eventPin.ArtifactDigest,
		EventPinReceiptDigest: eventReceiptDigest, PayloadPinReceiptDigests: []string{payloadReceiptDigest},
		PinIntentCommitment: pinIntent, Input: input,
	}
	return request, []segItem{
		{tapestore.NSEvent, eventPin.ArtifactDigest}, {tapestore.NSReceipt, eventReceiptDigest},
		{tapestore.NSPayload, payloadPin.ArtifactDigest}, {tapestore.NSReceipt, payloadReceiptDigest},
	}
}

// FM15, FM16 for the head CAS pin check: receipts and artifacts resolve from
// segments, legacy, or a mix; a corrupt segment refuses the pin set.
func TestSegmentReadsValidateEventPins(t *testing.T) {
	store, root := openTestPlatformStore(t)
	service := NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "platform-signing.key"))
	artifacts, err := NewEventArtifactService(service, platformTestKeyResolver{key: service.signingKey.Public})
	if err != nil {
		t.Fatal(err)
	}
	for name, split := range map[string]func(items []segItem) (seg, both []segItem){
		"all in segments":      func(i []segItem) ([]segItem, []segItem) { return i, nil },
		"mixed layouts":        func(i []segItem) ([]segItem, []segItem) { return []segItem{i[0], i[3]}, nil },
		"all in both layouts":  func(i []segItem) ([]segItem, []segItem) { return nil, i },
		"receipts only in seg": func(i []segItem) ([]segItem, []segItem) { return []segItem{i[1], i[3]}, nil },
	} {
		t.Run(name, func(t *testing.T) {
			id := "computer-pins-" + strings.ReplaceAll(name, " ", "-")
			request, items := pinnedRequest(t, service, artifacts, id)
			if err := artifacts.ValidateEventPins(context.Background(), request); err != nil {
				t.Fatalf("legacy baseline: %v", err)
			}
			seg, both := split(items)
			if len(seg) > 0 {
				segmentize(t, service.artifactsRoot, id, 1, true, false, seg...)
			}
			if len(both) > 0 {
				segmentize(t, service.artifactsRoot, id, 2, true, true, both...)
			}
			if err := artifacts.ValidateEventPins(context.Background(), request); err != nil {
				t.Fatalf("segment-resident pins refused: %v", err)
			}
			corruptSegments(t, service.artifactsRoot, id)
			err := artifacts.ValidateEventPins(context.Background(), request)
			if len(seg) == 0 {
				// Round 5: every pin has a verified legacy copy.
				if err != nil {
					t.Fatalf("corrupt segment with every pin in legacy = %v, want the legacy copies", err)
				}
			} else if !errors.Is(err, tapestore.ErrCorrupt) {
				t.Fatalf("corrupt segment = %v, want ErrCorrupt", err)
			}
		})
	}
}

// FM15, FM16, FM17 for checkpoint evidence verification.
func TestSegmentReadsCheckpointVerifierEvidence(t *testing.T) {
	platformStore, root := openTestPlatformStore(t)
	service := NewService(platformStore, filepath.Join(root, "artifacts"), filepath.Join(root, "platform-signing.key"))
	artifacts, err := NewEventArtifactService(service, platformTestKeyResolver{key: service.signingKey.Public})
	if err != nil {
		t.Fatal(err)
	}
	cas, err := NewComputerEventCAS(platformStore, "corpusd", service.computerEventSigningKey(), artifacts)
	if err != nil {
		t.Fatal(err)
	}
	authority, err := NewCheckpointAuthority(cas, service)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 19, 7, 0, 0, 0, time.UTC)
	payload := map[string]any{
		"schema_version": 1, "operation_id": "operation-verify", "bundle_digest": platformTestDigest('a'),
		"decision": "pass", "verifier_refs": []string{"evidence:independent"}, "verifier_run_id": "run-verifier",
	}
	rawPayload, _ := computerevent.CanonicalJSON(payload)
	payloadDigest := computerevent.DigestBytes(rawPayload)
	eventID, _ := computerevent.NewEventID()
	const id = "computer-seg-verify"
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: id,
		Sequence: 2, PreviousHead: platformTestDigest('b'), EventKind: computerevent.EventVerificationRecorded,
		OccurredAt: now.Format(time.RFC3339Nano), IdempotencyKey: "verification-event", RequestCommitment: platformTestDigest('c'),
		TrajectoryID: "trajectory-verify", CapsuleID: "capsule-verify", ActorProfile: "engineering",
		AuthorityRef: "guest-core:self-development-verifier", OutputArtifactRefs: []string{"artifact:sha256:" + payloadDigest},
		PayloadCommitment: payloadDigest, PrivacyClass: "public", ReducerVersion: computerevent.ReducerVersionV1,
		ExpectedDesiredEventHead: platformTestDigest('b'), ExpectedEffectiveEventHead: platformTestDigest('b'),
		ExpectedDesiredStateCommitment: platformTestDigest('f'), ExpectedEffectiveStateCommitment: platformTestDigest('f'),
	}
	rawEvent, _ := event.CanonicalBytes()
	eventDigest := computerevent.DigestBytes(rawEvent)
	if err := service.writeBlob(filepath.Join("sha256", "computer-event", eventDigest), rawEvent); err != nil {
		t.Fatal(err)
	}
	if err := service.writeBlob(filepath.Join("sha256", "computer-event-payload", payloadDigest), rawPayload); err != nil {
		t.Fatal(err)
	}
	insertTestAppendReceipt(t, platformStore.db, event.ComputerID, event.IdempotencyKey, platformTestDigest('c'), event.Sequence, string(event.EventKind), eventDigest, platformTestDigest('d'), nil, now)
	request := selfdevprotocol.CheckpointRequest{ComputerID: id, VerifierCertificate: selfdevprotocol.VerifierCertificateResponse{Request: selfdevprotocol.VerifierCertificateRequest{
		Version: 1, ComputerID: id, OperationID: "operation-verify", BundleDigest: platformTestDigest('a'),
		VerificationEventDigest: eventDigest, VerifierEvidenceRefs: []string{"evidence:independent"}, DecisionEventHead: platformTestDigest('1'),
		CodeRef: "code:verify", ArtifactProgramRef: "artifact:verify", ReleaseDigest: platformTestDigest('2'), Decision: "pass",
	}}}
	if err := authority.verifyVerifierEvidence(t.Context(), request); err != nil {
		t.Fatalf("legacy baseline: %v", err)
	}
	// Event and payload live only in segments.
	segmentize(t, service.artifactsRoot, id, 1, true, false, segItem{tapestore.NSEvent, eventDigest}, segItem{tapestore.NSPayload, payloadDigest})
	if err := authority.verifyVerifierEvidence(t.Context(), request); err != nil {
		t.Fatalf("segment-resident verifier evidence refused: %v", err)
	}
	request.VerifierCertificate.Request.BundleDigest = platformTestDigest('9')
	if err := authority.verifyVerifierEvidence(t.Context(), request); err == nil {
		t.Fatal("segment-resident evidence accepted a substituted bundle")
	}
	request.VerifierCertificate.Request.BundleDigest = platformTestDigest('a')

	corruptSegments(t, service.artifactsRoot, id)
	if err := authority.verifyVerifierEvidence(t.Context(), request); err == nil {
		t.Fatal("corrupt segment accepted as verifier evidence")
	}
	if err := platformStore.putTapeTombstone(context.Background(), TapeTombstone{ComputerID: id, Status: tapeTombstoneArchived, HeadSequence: 2, CanonicalEventHead: platformTestDigest('1'), ManifestRun: "ts-x", ManifestSHA256: platformTestDigest('2'), ArchivePath: "/archive/v.tar.zst", ProofJSON: "{}"}); err != nil {
		t.Fatal(err)
	}
	if err := authority.verifyVerifierEvidence(t.Context(), request); !errors.Is(err, ErrComputerTapeDeleted) {
		t.Fatalf("tombstoned checkpoint evidence = %v, want ErrComputerTapeDeleted", err)
	}
}

// The genesis baseline branch reads the genesis event through the same
// library.
func TestSegmentReadsCheckpointGenesisEvidence(t *testing.T) {
	platformStore, root := openTestPlatformStore(t)
	service := NewService(platformStore, filepath.Join(root, "artifacts"), filepath.Join(root, "platform-signing.key"))
	artifacts, err := NewEventArtifactService(service, platformTestKeyResolver{key: service.signingKey.Public})
	if err != nil {
		t.Fatal(err)
	}
	cas, err := NewComputerEventCAS(platformStore, "corpusd", service.computerEventSigningKey(), artifacts)
	if err != nil {
		t.Fatal(err)
	}
	authority, err := NewCheckpointAuthority(cas, service)
	if err != nil {
		t.Fatal(err)
	}
	const id = "computer-seg-genesis"
	publicKey := []byte("a-verifier-public-key-of-32-byte")
	releaseDigest := platformTestDigest('2')
	keyID := "verifier-1"
	eventID, _ := computerevent.NewEventID()
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: id, Sequence: 1,
		PreviousHead: computerevent.ZeroHead, EventKind: computerevent.EventGenesisImported,
		OccurredAt: time.Date(2026, 7, 19, 7, 0, 0, 0, time.UTC).Format(time.RFC3339Nano), IdempotencyKey: "genesis", RequestCommitment: platformTestDigest('c'),
		ActorProfile: "trusted-core", AuthorityRef: "authority:test", PayloadCommitment: platformTestDigest('a'), PrivacyClass: "public",
		ReducerVersion: computerevent.ReducerVersionV1, ExpectedDesiredEventHead: computerevent.ZeroHead, ExpectedEffectiveEventHead: computerevent.ZeroHead,
		ExpectedDesiredStateCommitment: computerevent.ZeroHead, ExpectedEffectiveStateCommitment: computerevent.ZeroHead,
		ResultingEffectiveCommitment: platformTestDigest('b'),
		VerifierRefs: []string{
			"verifier-key:" + keyID + ":sha256:" + computerevent.DigestBytes(publicKey),
			"updater-key:updater-1:sha256:" + platformTestDigest('3'),
			"release:sha256:" + releaseDigest,
		},
	}
	rawEvent, err := event.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	eventDigest := computerevent.DigestBytes(rawEvent)
	if err := service.writeBlob(filepath.Join("sha256", "computer-event", eventDigest), rawEvent); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	insertTestAppendReceipt(t, platformStore.db, id, event.IdempotencyKey, platformTestDigest('c'), 1, string(event.EventKind), eventDigest, platformTestDigest('d'), nil, now)
	var response selfdevprotocol.VerifierCertificateResponse
	response.Request = selfdevprotocol.VerifierCertificateRequest{
		Version: 1, ComputerID: id, VerificationEventDigest: eventDigest, DecisionEventHead: eventDigest,
		ReleaseDigest: releaseDigest, Decision: "genesis_baseline",
	}
	response.PublicKey = base64.RawStdEncoding.EncodeToString(publicKey)
	response.Certificate.RequiredSigners = []computerevent.SignerRef{{SignerDomain: "verifier", KeyID: keyID}}
	request := selfdevprotocol.CheckpointRequest{ComputerID: id, ReleaseDigest: releaseDigest, VerifierCertificate: response}
	if err := authority.verifyVerifierEvidence(t.Context(), request); err != nil {
		t.Fatalf("legacy genesis baseline: %v", err)
	}
	segmentize(t, service.artifactsRoot, id, 1, true, false, segItem{tapestore.NSEvent, eventDigest})
	if err := authority.verifyVerifierEvidence(t.Context(), request); err != nil {
		t.Fatalf("segment-resident genesis event refused: %v", err)
	}
	corruptSegments(t, service.artifactsRoot, id)
	if err := authority.verifyVerifierEvidence(t.Context(), request); err == nil {
		t.Fatal("corrupt segment accepted as the genesis event")
	}
}

// FM18: the orphan sweep builds its reference set from chain events. A chain
// whose events live only in segments still references its legacy-only
// payloads and receipts: nothing legacy-only is listed or deleted, the sweep
// does not refuse, and a true orphan beside them is still collected.
func TestTapeSweepKeepsLegacyOnlyFilesNamedBySegmentEvents(t *testing.T) {
	f := newTapeFixture(t)
	f.live("live-1")
	live := f.chain("live-1", 3, tapeAged)
	orphan := f.put(tapeNSPayload, []byte("true orphan beside a segment chain"), tapeAged)
	var events []segItem
	for _, s := range live.seqs {
		events = append(events, segItem{tapestore.NSEvent, s.event})
	}
	segmentize(t, f.root, "live-1", 1, true, false, events...)

	dry := f.dryRun(TapeSweepRequest{AcceptUnrecordedForkLineage: true, Sweeps: []string{TapeSweepOrphans}})
	if dry.OrphansRefused != "" || dry.Orphans[tapeNSEvent].Refused != "" {
		t.Fatalf("sweep refused a chain with segment-resident events: %q", dry.OrphansRefused)
	}
	listed := listedDigests(f.manifestItems(dry), TapeSweepOrphans)
	if len(listed) != 1 || !listed[orphan] {
		t.Fatalf("orphans listed = %v, want only %s", listed, orphan)
	}
	if _, err := f.apply(dry, TapeSweepRequest{AcceptUnrecordedForkLineage: true, Sweeps: []string{TapeSweepOrphans}}); err != nil {
		t.Fatal(err)
	}
	if f.exists(tapeNSPayload, orphan) {
		t.Fatal("true orphan survived")
	}
	for _, s := range live.seqs {
		if !f.exists(tapeNSPayload, s.payload) || !f.exists(tapeNSReceipt, s.eventPin) || !f.exists(tapeNSReceipt, s.payloadPin) {
			t.Fatal("a legacy-only file named by a segment-resident event was deleted")
		}
	}
	if segmentFilesExist(t, f.root, "live-1") != 2 {
		t.Fatal("the sweep touched the segment files")
	}
	if err := f.svc.tapeReader().Scan("live-1", func(tapestore.Record) error { return nil }); err != nil {
		t.Fatalf("segments no longer verify after the sweep: %v", err)
	}
}

// FM18, the apply-time delta: a payload that was an orphan at dry-run is
// named by a new event that lands only in a segment before apply. The apply
// must keep it.
func TestTapeSweepDeltaReadsSegmentEvents(t *testing.T) {
	f := newTapeFixture(t)
	f.live("live-1")
	c := f.chain("live-1", 1, tapeAged)
	late := []byte("payload named by an event that lands after the dry-run")
	lateDigest := f.put(tapeNSPayload, late, tapeAged)
	dry := f.dryRun(TapeSweepRequest{AcceptUnrecordedForkLineage: true, Sweeps: []string{TapeSweepOrphans}})
	if listed := listedDigests(f.manifestItems(dry), TapeSweepOrphans); !listed[lateDigest] {
		t.Fatalf("setup: %s was not an orphan at dry-run: %v", lateDigest, listed)
	}
	s := f.appendSeq(c, late, tapeAged)
	if s.payload != lateDigest {
		t.Fatal("setup: appended sequence does not name the late payload")
	}
	segmentize(t, f.root, "live-1", 1, true, false, segItem{tapestore.NSEvent, s.event})
	applied, err := f.apply(dry, TapeSweepRequest{AcceptUnrecordedForkLineage: true, Sweeps: []string{TapeSweepOrphans}})
	if err != nil {
		t.Fatal(err)
	}
	if !f.exists(tapeNSPayload, lateDigest) {
		t.Fatal("apply deleted a payload named by a segment-resident event that landed after the dry-run")
	}
	if applied.Apply == nil || applied.Apply.Deleted != 0 {
		t.Fatalf("apply = %+v, want nothing deleted", applied.Apply)
	}
}

// FM18: a corrupt segment makes the sweep refuse. It never reads "unreadable"
// as "no references" and deletes.
func TestTapeSweepRefusesOnCorruptSegment(t *testing.T) {
	f := newTapeFixture(t)
	f.live("live-1")
	live := f.chain("live-1", 2, tapeAged)
	orphan := f.put(tapeNSPayload, []byte("orphan that must survive a corrupt-segment run"), tapeAged)
	var events []segItem
	for _, s := range live.seqs {
		events = append(events, segItem{tapestore.NSEvent, s.event})
	}
	segmentize(t, f.root, "live-1", 1, true, false, events...)
	corruptSegments(t, f.root, "live-1")

	dry := f.dryRun(TapeSweepRequest{AcceptUnrecordedForkLineage: true, Sweeps: []string{TapeSweepOrphans}})
	if dry.OrphansRefused == "" || !strings.Contains(dry.OrphansRefused, "corrupt") {
		t.Fatalf("OrphansRefused = %q, want a corrupt-segment refusal", dry.OrphansRefused)
	}
	if listed := listedDigests(f.manifestItems(dry), TapeSweepOrphans); len(listed) != 0 {
		t.Fatalf("a refused sweep listed %v", listed)
	}
	_, _ = f.apply(dry, TapeSweepRequest{AcceptUnrecordedForkLineage: true, Sweeps: []string{TapeSweepOrphans}})
	if !f.exists(tapeNSPayload, orphan) {
		t.Fatal("a sweep over a corrupt segment deleted a file")
	}
	f.allExistExceptEvents(live)
}

func (f *tapeFixture) allExistExceptEvents(c *tapeChain) {
	f.t.Helper()
	for _, s := range c.seqs {
		if !f.exists(tapeNSPayload, s.payload) || !f.exists(tapeNSReceipt, s.eventPin) || !f.exists(tapeNSReceipt, s.payloadPin) {
			f.t.Fatal("legacy file deleted")
		}
	}
}

// FM18: the destroyed-computer sweep does not know how to archive and delete
// segments (A2), so it refuses a computer that has any, and deletes nothing.
func TestTapeSweepDestroyedRefusesComputersWithSegments(t *testing.T) {
	f := newTapeFixture(t)
	dead := f.chain("dead-1", 2, tapeAged)
	var events []segItem
	for _, s := range dead.seqs {
		events = append(events, segItem{tapestore.NSEvent, s.event})
	}
	segmentize(t, f.root, "dead-1", 1, true, false, events...)
	plain := f.chain("dead-2", 1, tapeAged) // a destroyed computer without segments is still swept

	dry := f.dryRun(TapeSweepRequest{AcceptUnrecordedForkLineage: true, Sweeps: []string{TapeSweepDestroyed}})
	var seen bool
	for _, p := range dry.DestroyedProofs {
		if p.ComputerID == "dead-1" {
			seen = true
			if p.Deletable || !strings.Contains(p.Reason, "segment") {
				t.Fatalf("dead-1 proof = %+v, want not deletable because of segments", p)
			}
		}
	}
	if !seen {
		t.Fatal("dead-1 has no proof")
	}
	listed := listedDigests(f.manifestItems(dry), TapeSweepDestroyed)
	for _, s := range dead.seqs {
		if listed[s.payload] || listed[s.eventPin] || listed[s.payloadPin] {
			t.Fatal("files of a computer with segments were listed for deletion")
		}
	}
	if !listed[plain.seqs[0].payload] {
		t.Fatal("a destroyed computer without segments was no longer swept")
	}
	if _, err := f.apply(dry, TapeSweepRequest{AcceptUnrecordedForkLineage: true, Sweeps: []string{TapeSweepDestroyed}}); err != nil {
		t.Fatal(err)
	}
	f.allExistExceptEvents(dead)
	if segmentFilesExist(t, f.root, "dead-1") != 2 {
		t.Fatal("segment files changed")
	}
	if f.exists(tapeNSPayload, plain.seqs[0].payload) {
		t.Fatal("the segment-free destroyed computer survived apply")
	}
}

// FM19: artifact GC, active, never reads or deletes tape-seg/.
func TestArtifactGCLeavesTapeSegmentsAlone(t *testing.T) {
	store, root := openTestPlatformStore(t)
	ensureGCRouteTables(t, store)
	artifactsRoot := filepath.Join(root, "artifacts")
	service := NewService(store, artifactsRoot, "")
	w, err := tapestore.CreateSegment(artifactsRoot, "computer-gc", 1)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("a segment-resident payload nothing in SQL references")
	if _, err := w.Append(tapestore.NSPayload, body); err != nil {
		t.Fatal(err)
	}
	if err := w.Seal(); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-90 * 24 * time.Hour)
	dir := filepath.Join(artifactsRoot, tapestore.SegDirName, "computer-gc")
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if err := os.Chtimes(filepath.Join(dir, e.Name()), old, old); err != nil {
			t.Fatal(err)
		}
	}
	report, err := service.RunArtifactGC(context.Background(), ArtifactGCConfig{Mode: ArtifactGCModeActive, Grace: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	for ns := range report.Namespaces {
		if strings.Contains(ns, tapestore.SegDirName) {
			t.Fatalf("GC swept %s", ns)
		}
	}
	if got := segmentFilesExist(t, artifactsRoot, "computer-gc"); got != 2 {
		t.Fatalf("GC left %d segment files, want 2", got)
	}
	if got, src, err := service.tapeReader().Read("computer-gc", tapestore.NSPayload, computerevent.DigestBytes(body)); err != nil || src != tapestore.SourceSegment || string(got) != string(body) {
		t.Fatalf("segment unreadable after GC: %v", err)
	}
}

// FM17: the head CAS of a tombstoned computer is refused with the explicit
// error even when its pins sit in segments; the pin check never runs.
func TestSegmentTombstonedHeadCASRefused(t *testing.T) {
	store, root := openTestPlatformStore(t)
	service := NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "platform-signing.key"))
	cas, artifacts, _, err := service.ComputerEventRuntime()
	if err != nil {
		t.Fatal(err)
	}
	const id = "computer-seg-cas"
	request, items := pinnedRequest(t, service, artifacts, id)
	segmentize(t, service.artifactsRoot, id, 1, true, false, items...)
	corruptSegments(t, service.artifactsRoot, id)
	if err := store.putTapeTombstone(context.Background(), TapeTombstone{ComputerID: id, Status: tapeTombstoneArchiving, HeadSequence: 0, CanonicalEventHead: computerevent.ZeroHead, ManifestRun: "ts-x", ManifestSHA256: platformTestDigest('2'), ProofJSON: "{}"}); err != nil {
		t.Fatal(err)
	}
	if _, err := cas.CompareAndSwap(context.Background(), request); !errors.Is(err, ErrComputerTapeDeleted) || errors.Is(err, tapestore.ErrCorrupt) {
		t.Fatalf("CAS on a tombstoned computer with segments = %v, want only ErrComputerTapeDeleted", err)
	}
}
