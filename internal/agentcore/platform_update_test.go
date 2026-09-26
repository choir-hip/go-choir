package agentcore

// M9a local proof: a platform-control-signed update offer drives the guest
// updater through canonical events + checkpoint + platform-follow route
// promotion, then restores to the pinned prior head. The fixture is the M7
// derivable harness — real updater engine, real event appender, real route
// ledger — extended with the platform-follow route stub and an offer minter.

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/platformrelease"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

// mintPlatformUpdateOffer builds a structurally valid signed offer for the
// fixture computer over a one-file SPA release.
func (fx *derivableSelfDevFixture) mintPlatformUpdateOffer(t *testing.T, updateID, spaContent string, baseHead string) selfdevprotocol.PlatformUpdateOffer {
	t.Helper()
	now := time.Now().UTC()
	spaSum := sha256.Sum256([]byte(spaContent))
	spaDigest := hex.EncodeToString(spaSum[:])
	closure, err := computerversion.NewCodeClosure(spaDigest[:40], []computerversion.CodeArtifact{{
		Name: "spa-" + updateID, SHA256: spaDigest, URI: "artifact+sha256://" + spaDigest + "/spa",
	}}, now)
	if err != nil {
		t.Fatal(err)
	}
	program, err := computerversion.NewArtifactProgram([]computerversion.ArtifactProgramEntry{{
		Kind: "spa_release", ContentSHA256: spaDigest, ArtifactURI: "artifact+sha256://" + spaDigest + "/spa",
	}}, now)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := updater.FinalizeManifest(updater.ReleaseManifest{
		Version: updater.ManifestVersion, ComputerID: fx.computerID,
		CodeRef: string(closure.Ref), ArtifactProgramRef: string(program.Ref),
		EventSchemaVersion: computerevent.SchemaVersionV1, ReducerVersion: computerevent.ReducerVersionV1,
		Marker: "platform-update-" + updateID,
		Files:  []updater.ManifestFile{{Path: "frontend/index.html", SHA256: spaDigest, Mode: 0o444}},
	})
	if err != nil {
		t.Fatal(err)
	}
	verifierSum := sha256.Sum256([]byte("platform-verify-" + updateID))
	offer := selfdevprotocol.PlatformUpdateOffer{
		Version: 1, ComputerID: fx.computerID, UpdateID: updateID, Realization: "realization-derivable",
		Manifest: manifest,
		Files: []selfdevprotocol.PlatformUpdateFile{{
			Path: "frontend/index.html", SHA256: spaDigest, Mode: 0o444,
			Bytes: base64.StdEncoding.EncodeToString([]byte(spaContent)),
		}},
		CodeClosure:          closure,
		ArtifactProgram:      program,
		VerifierRefs:         []string{hex.EncodeToString(verifierSum[:])},
		DivergenceStatus:     platformrelease.DivergenceTracking,
		PlatformFollowPolicy: platformrelease.FollowPolicyAuto,
		BaseEventHead:        baseHead,
		ExpiresAt:            now.Add(4 * time.Minute).Format(time.RFC3339Nano),
	}
	offerDigest, err := offer.Digest()
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := selfdevprotocol.NewAuthorityReceipt(
		selfdevprotocol.ReceiptKindPlatformUpdate, fx.computerID,
		offerDigest, offerDigest, "corpusd",
		computerevent.SigningKey{
			SignerRef:  computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "authority-test"},
			PrivateKey: fx.platformKey,
		}, now)
	if err != nil {
		t.Fatal(err)
	}
	offer.Authorization = receipt
	return offer
}

func (fx *derivableSelfDevFixture) currentHead(t *testing.T) string {
	t.Helper()
	head, err := fx.store.Head(context.Background(), fx.computerID)
	if err != nil || head == nil {
		t.Fatalf("head: %v %#v", err, head)
	}
	return head.CanonicalEventHead
}

// TestPlatformUpdatePushAppliesAndRestores is the M9a happy path: signed offer
// -> canonical events -> checkpoint + platform-follow route promotion ->
// pinned-head restore returns the prior release.
func TestPlatformUpdatePushAppliesAndRestores(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-platform-update-1")
	ctx := context.Background()

	// Update 1: establish release A on the tracking computer.
	offerA := fx.mintPlatformUpdateOffer(t, "update-a", "<html>release-a</html>", fx.currentHead(t))
	reportA, err := fx.rt.ApplyPlatformUpdate(ctx, offerA)
	if err != nil {
		t.Fatalf("update A refused: %v", err)
	}
	if reportA.AcceptedEventHead == "" || reportA.AppliedEventHead == "" || reportA.ReleaseDigest == "" || reportA.CheckpointDigest == "" || reportA.Checkpoint == nil {
		t.Fatalf("update A report incomplete: %+v", reportA)
	}
	if reportA.RouteGeneration != 2 {
		t.Fatalf("update A route generation = %d, want 2 (promote over baseline bootstrap)", reportA.RouteGeneration)
	}
	manifest, err := updater.ReadCurrentManifest(fx.updaterRoot)
	if err != nil || manifest.ContentDigest != reportA.ReleaseDigest || !strings.Contains(manifest.Marker, "platform-update-update-a") {
		t.Fatalf("served release after A: %v %+v", err, manifest)
	}
	// Route slot promoted under platform-follow — not the owner self-dev path.
	slot, latestReceipt, err := fx.ledger.Resolve(ctx, mustRouteSlotID(t))
	if err != nil {
		t.Fatalf("route resolve: %v", err)
	}
	if slot.Generation != 2 || latestReceipt.Kind != "promote" {
		t.Fatalf("route slot after A: gen=%d kind=%s", slot.Generation, latestReceipt.Kind)
	}

	// Pin the advertised base at update-A's head: restore must reach back to a
	// historical checkpoint while the live chain continues forward.
	if err := fx.baseSource.pinAtHead(ctx, reportA.AppliedEventHead); err != nil {
		t.Fatalf("pin base at update-A head: %v", err)
	}

	// Update 2: advance to release B (SPA file change).
	offerB := fx.mintPlatformUpdateOffer(t, "update-b", "<html>release-b</html>", fx.currentHead(t))
	reportB, err := fx.rt.ApplyPlatformUpdate(ctx, offerB)
	if err != nil {
		t.Fatalf("update B refused: %v", err)
	}
	manifest, err = updater.ReadCurrentManifest(fx.updaterRoot)
	if err != nil || manifest.ContentDigest != reportB.ReleaseDigest || !strings.Contains(manifest.Marker, "platform-update-update-b") {
		t.Fatalf("served release after B: %v %+v", err, manifest)
	}
	slot, latestReceipt, err = fx.ledger.Resolve(ctx, mustRouteSlotID(t))
	if err != nil {
		t.Fatalf("route resolve after B: %v", err)
	}
	if slot.Generation != 3 {
		t.Fatalf("route slot after B: gen=%d, want 3", slot.Generation)
	}

	// Event shape: exactly one accepted/started/applied/checkpoint/route set
	// per update — the canonical chain carries the full transaction.
	kinds := fx.countEventKinds()
	for _, kind := range []computerevent.EventKind{
		computerevent.EventEffectAccepted, computerevent.EventMaterializationStarted,
		computerevent.EventMaterializationApplied, computerevent.EventCheckpointPublished,
		computerevent.EventRouteProjectionUpdated,
	} {
		if kinds[kind] != 2 {
			t.Fatalf("event %s count = %d, want 2 (one per update)", kind, kinds[kind])
		}
	}

	// Replay: re-presenting update A's offer is a no-op, not a re-apply.
	replay, err := fx.rt.ApplyPlatformUpdate(ctx, offerA)
	if err != nil || !replay.Replayed {
		t.Fatalf("update A replay: %v %+v", err, replay)
	}

	// Restore edge: the pre-push checkpoint returns the computer to release A.
	restoreReport, err := fx.rt.RematerializeFromTape(ctx, fx.computerID, reportA.Checkpoint.Checkpoint)
	if err != nil {
		t.Fatalf("restore to update-A head refused: %v", err)
	}
	if !restoreReport.FrontendRestaged {
		t.Fatalf("restore did not restage the pinned release: %+v", restoreReport)
	}
	manifest, err = updater.ReadCurrentManifest(fx.updaterRoot)
	if err != nil || manifest.ContentDigest != reportA.ReleaseDigest {
		t.Fatalf("served release after restore: %v %+v", err, manifest)
	}
	head, err := fx.store.Head(ctx, fx.computerID)
	if err != nil || head == nil || head.CanonicalEventHead != reportA.AcceptedEventHead && head.EffectiveEventHead != reportA.AppliedEventHead {
		// Restored head must equal the checkpoint's recorded heads.
		req := reportA.Checkpoint.Checkpoint.Request
		if head.CanonicalEventHead != req.AcceptedEventHead || head.EffectiveEventHead != req.EffectiveEventHead {
			t.Fatalf("restored head does not match checkpoint: got %+v want accepted=%s effective=%s", head, req.AcceptedEventHead, req.EffectiveEventHead)
		}
	}
}

// TestPlatformUpdateRefusals proves the verification-first contract: a forged,
// misbound, expired, or non-tracking offer is refused before any mutation.
func TestPlatformUpdateRefusals(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-platform-update-refusals")
	ctx := context.Background()
	eventCount := len(fx.cas.snapshot())

	refuse := func(name string, mutate func(*selfdevprotocol.PlatformUpdateOffer)) {
		t.Helper()
		offer := fx.mintPlatformUpdateOffer(t, "refusal-"+name, "<html>x</html>", fx.currentHead(t))
		mutate(&offer)
		if _, err := fx.rt.ApplyPlatformUpdate(ctx, offer); err == nil {
			t.Fatalf("%s: offer accepted, want refusal", name)
		}
		if len(fx.cas.snapshot()) != eventCount {
			t.Fatalf("%s: refusal mutated the canonical log", name)
		}
	}

	refuse("bad-signature", func(o *selfdevprotocol.PlatformUpdateOffer) {
		o.Authorization.Signature = base64.StdEncoding.EncodeToString([]byte("forged-signature"))
	})
	refuse("wrong-computer", func(o *selfdevprotocol.PlatformUpdateOffer) {
		o.ComputerID = "computer-other"
	})
	refuse("expired", func(o *selfdevprotocol.PlatformUpdateOffer) {
		o.ExpiresAt = time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano)
	})
	refuse("pinned-lineage", func(o *selfdevprotocol.PlatformUpdateOffer) {
		o.PlatformFollowPolicy = platformrelease.FollowPolicyPinned
	})
	refuse("divergent", func(o *selfdevprotocol.PlatformUpdateOffer) {
		o.DivergenceStatus = platformrelease.DivergenceDivergent
		o.DivergedComponents = []string{"desktop_shell"}
	})
	refuse("stale-head", func(o *selfdevprotocol.PlatformUpdateOffer) {
		o.BaseEventHead = strings.Repeat("0", 64)
	})
	refuse("payload-mismatch", func(o *selfdevprotocol.PlatformUpdateOffer) {
		o.Files[0].Bytes = base64.StdEncoding.EncodeToString([]byte("tampered"))
	})
	refuse("unsigned-authorization-kind", func(o *selfdevprotocol.PlatformUpdateOffer) {
		o.Authorization.Kind = selfdevprotocol.ReceiptKindRouteProjection
	})
}

// TestPlatformUpdateBootstrapsAbsentRoute proves the fresh-computer path:
// production computers carry no route slot until a committed transition, so
// the first platform update bootstraps the slot at generation 1 instead of
// promoting over a baseline.
func TestPlatformUpdateBootstrapsAbsentRoute(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-platform-update-boot")
	ctx := context.Background()

	// Drop the seeded baseline bootstrap: a fresh ledger resolves
	// ErrSlotNotFound -> route_absent, matching a new production computer.
	fx.ledger = routeledger.NewMemoryLedger()
	fx.closures = map[string]computerversion.CodeClosure{}
	fx.programs = map[string]computerversion.ArtifactProgram{}

	offer := fx.mintPlatformUpdateOffer(t, "update-boot", "<html>boot</html>", fx.currentHead(t))
	report, err := fx.rt.ApplyPlatformUpdate(ctx, offer)
	if err != nil {
		t.Fatalf("absent-route update refused: %v", err)
	}
	if report.RouteGeneration != 1 {
		t.Fatalf("absent-route generation = %d, want 1 (bootstrap)", report.RouteGeneration)
	}
	slot, latestReceipt, err := fx.ledger.Resolve(ctx, mustRouteSlotID(t))
	if err != nil {
		t.Fatalf("route resolve: %v", err)
	}
	if slot.Generation != 1 || latestReceipt.Kind != routeledger.TransitionBootstrap {
		t.Fatalf("route slot: gen=%d kind=%s, want generation-1 bootstrap", slot.Generation, latestReceipt.Kind)
	}

	// A second update promotes on top of the bootstrapped slot.
	offerB := fx.mintPlatformUpdateOffer(t, "update-boot-2", "<html>boot-2</html>", fx.currentHead(t))
	reportB, err := fx.rt.ApplyPlatformUpdate(ctx, offerB)
	if err != nil {
		t.Fatalf("post-bootstrap update refused: %v", err)
	}
	if reportB.RouteGeneration != 2 {
		t.Fatalf("post-bootstrap generation = %d, want 2 (promote)", reportB.RouteGeneration)
	}

	// Replay of the bootstrapping offer stays a no-op across the boundary.
	replay, err := fx.rt.ApplyPlatformUpdate(ctx, offer)
	if err != nil || !replay.Replayed {
		t.Fatalf("bootstrap replay: %v %+v", err, replay)
	}
}

// TestPlatformUpdateResumesPendingTransition proves the restart-resume
// contract: an update whose accepted event committed but whose apply died
// with the guest restart (pending transition still open) may re-drive the
// same offer to completion, while an offer bound to a different pending
// transition is still stale.
func TestPlatformUpdateResumesPendingTransition(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-platform-update-resume")
	ctx := context.Background()

	offer := fx.mintPlatformUpdateOffer(t, "update-resume", "<html>resume</html>", fx.currentHead(t))
	offerDigest := mustOfferDigest(t, offer)

	// Simulate the killed flight: commit only the accepted event, leaving the
	// pending transition open — the state a re-pushed offer observes after the
	// guest restarts.
	targetCommitment, err := selfdevprotocol.Digest(struct {
		ComputerID    string `json:"computer_id"`
		UpdateID      string `json:"update_id"`
		OfferDigest   string `json:"offer_digest"`
		ContentDigest string `json:"content_digest"`
	}{fx.computerID, offer.UpdateID, offerDigest, offer.Manifest.ContentDigest})
	if err != nil {
		t.Fatal(err)
	}
	head, err := fx.store.Head(ctx, fx.computerID)
	if err != nil || head == nil {
		t.Fatalf("head: %v", err)
	}
	acceptedEventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	acceptedEvent := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: acceptedEventID, ComputerID: fx.computerID,
		EventKind: computerevent.EventEffectAccepted, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: "platform-update-accepted-" + offer.UpdateID,
		ActorProfile:   agentprofile.Management, AuthorityRef: "platform-control:update",
		PrivacyClass: "owner", PayloadCommitment: computerevent.ZeroHead,
		ProposedEffectRef: offer.Manifest.ContentDigest, DecisionRef: offerDigest,
		VerifierRefs:                     offer.VerifierRefs,
		RequireExpectedHead:              true,
		PreviousHead:                     head.CanonicalEventHead,
		ExpectedDesiredEventHead:         head.DesiredEventHead,
		ExpectedEffectiveEventHead:       head.EffectiveEventHead,
		ExpectedDesiredStateCommitment:   head.DesiredStateCommitment,
		ExpectedEffectiveStateCommitment: head.EffectiveStateCommitment,
		ReducerVersion:                   computerevent.ReducerVersionV1,
	}
	if _, err := fx.appender.AppendNew(ctx, acceptedEvent, computerevent.TransitionInput{TargetStateCommitment: targetCommitment}, nil); err != nil {
		t.Fatalf("seed accepted event: %v", err)
	}

	// A different update is stale while this transition is pending.
	staleOffer := fx.mintPlatformUpdateOffer(t, "update-other", "<html>other</html>", fx.currentHead(t))
	if _, err := fx.rt.ApplyPlatformUpdate(ctx, staleOffer); err != ErrPlatformUpdateStaleHead {
		t.Fatalf("foreign-pending offer: got %v, want ErrPlatformUpdateStaleHead", err)
	}

	// Re-pushing the same offer resumes the pending transition to completion.
	report, err := fx.rt.ApplyPlatformUpdate(ctx, offer)
	if err != nil {
		t.Fatalf("resume refused: %v", err)
	}
	if report.ReleaseDigest == "" || report.CheckpointDigest == "" || report.Replayed {
		t.Fatalf("resume report incomplete: %+v", report)
	}

	// The accepted event was not duplicated; the update's event set is
	// exactly one of each phase.
	kinds := fx.countEventKinds()
	for _, kind := range []computerevent.EventKind{
		computerevent.EventEffectAccepted, computerevent.EventMaterializationStarted,
		computerevent.EventMaterializationApplied, computerevent.EventCheckpointPublished,
		computerevent.EventRouteProjectionUpdated,
	} {
		if kinds[kind] != 1 {
			t.Fatalf("event %s count = %d, want 1", kind, kinds[kind])
		}
	}
	slot, latestReceipt, err := fx.ledger.Resolve(ctx, mustRouteSlotID(t))
	if err != nil {
		t.Fatalf("route resolve: %v", err)
	}
	if slot.Generation != 2 || latestReceipt.Kind != routeledger.TransitionPromote {
		t.Fatalf("resumed route: gen=%d kind=%s, want generation-2 promote", slot.Generation, latestReceipt.Kind)
	}
}

// TestPlatformUpdateBootSweepResumesPending proves the restart-resume
// contract end-to-end: an accepted event pinning its offer commits, the guest
// restarts before the apply tail lands, and the boot sweep reconstructs the
// apply from the tape alone — no caller re-push required.
func TestPlatformUpdateBootSweepResumesPending(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-platform-update-sweep")
	ctx := context.Background()

	offer := fx.mintPlatformUpdateOffer(t, "update-sweep", "<html>sweep</html>", fx.currentHead(t))
	offerDigest := mustOfferDigest(t, offer)

	// Simulate the killed in-flight apply: only the accepted event committed,
	// offer pinned as its payload, pending transition still open.
	targetCommitment, err := selfdevprotocol.Digest(struct {
		ComputerID    string `json:"computer_id"`
		UpdateID      string `json:"update_id"`
		OfferDigest   string `json:"offer_digest"`
		ContentDigest string `json:"content_digest"`
	}{offer.ComputerID, offer.UpdateID, offerDigest, offer.Manifest.ContentDigest})
	if err != nil {
		t.Fatal(err)
	}
	head, err := fx.store.Head(ctx, fx.computerID)
	if err != nil || head == nil {
		t.Fatalf("head: %v", err)
	}
	acceptedEventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	acceptedEvent := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: acceptedEventID, ComputerID: offer.ComputerID,
		EventKind: computerevent.EventEffectAccepted, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: "platform-update-accepted-" + offer.UpdateID,
		ActorProfile:   agentprofile.Management, AuthorityRef: "platform-control:update",
		PrivacyClass:      "owner",
		ProposedEffectRef: offer.Manifest.ContentDigest, DecisionRef: offerDigest,
		VerifierRefs:                     offer.VerifierRefs,
		RequireExpectedHead:              true,
		PreviousHead:                     head.CanonicalEventHead,
		ExpectedDesiredEventHead:         head.DesiredEventHead,
		ExpectedEffectiveEventHead:       head.EffectiveEventHead,
		ExpectedDesiredStateCommitment:   head.DesiredStateCommitment,
		ExpectedEffectiveStateCommitment: head.EffectiveStateCommitment,
		ReducerVersion:                   computerevent.ReducerVersionV1,
	}
	offerBytes, err := json.Marshal(offer)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := fx.rt.eventAppender.AppendNewPayload(ctx, acceptedEvent, computerevent.TransitionInput{TargetStateCommitment: targetCommitment}, offerBytes, "application/vnd.choir.platform-update-offer+json", "owner"); err != nil {
		t.Fatalf("seed accepted+offer event: %v", err)
	}
	head, err = fx.store.Head(ctx, fx.computerID)
	if err != nil || head == nil || head.PendingTransitionRef == "" {
		t.Fatalf("pending transition did not open: %v %#v", err, head)
	}

	// Boot: the sweep resumes the update entirely from the tape.
	fx.rt.resumePendingPlatformUpdate(ctx)

	if count := fx.countEventKinds()[computerevent.EventMaterializationApplied]; count != 1 {
		t.Fatalf("materialization_applied count = %d, want 1", count)
	}
	slot, latestReceipt, err := fx.ledger.Resolve(ctx, mustRouteSlotID(t))
	if err != nil {
		t.Fatalf("route resolve: %v", err)
	}
	if slot.Generation != 2 || latestReceipt.Kind != routeledger.TransitionPromote {
		t.Fatalf("resumed route: gen=%d kind=%s, want generation-2 promote", slot.Generation, latestReceipt.Kind)
	}
}

// TestPlatformUpdateStrandedTailResumes proves the post-applied strand is
// recoverable: `applied` commits and clears the pending transition, the tail
// dies (here: control plane down at checkpoint mint), and the next push or
// boot sweep drives the missing checkpoint+route events — without
// duplicating the applied event or re-minting the checkpoint.
func TestPlatformUpdateStrandedTailResumes(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-platform-update-strand")
	ctx := context.Background()

	offer := fx.mintPlatformUpdateOffer(t, "update-strand", "<html>strand</html>", fx.currentHead(t))

	// Kill the control plane mid-update: `applied` commits, then checkpoint
	// mint is refused and the tail dies with the transition already closed.
	fx.controlFail.Store(true)
	_, err := fx.rt.ApplyPlatformUpdate(ctx, offer)
	fx.controlFail.Store(false)
	if err == nil {
		t.Fatal("update with dead control plane should fail")
	}
	kinds := fx.countEventKinds()
	if kinds[computerevent.EventMaterializationApplied] != 1 || kinds[computerevent.EventCheckpointPublished] != 0 {
		t.Fatalf("expected applied-without-checkpoint strand, got %v", kinds)
	}
	head, err := fx.store.Head(ctx, fx.computerID)
	if err != nil || head == nil || head.PendingTransitionRef != "" {
		t.Fatalf("strand must close pending: %v %#v", err, head)
	}

	// Boot sweep path: no pending transition, but the latest applied event
	// lacks its route sibling — the sweep must see and drive it.
	fx.rt.resumePendingPlatformUpdate(ctx)

	slot, latestReceipt, err := fx.ledger.Resolve(ctx, mustRouteSlotID(t))
	if err != nil {
		t.Fatalf("route resolve: %v", err)
	}
	if slot.Generation != 2 || latestReceipt.Kind != routeledger.TransitionPromote {
		t.Fatalf("resumed strand route: gen=%d kind=%s, want generation-2 promote", slot.Generation, latestReceipt.Kind)
	}
	kinds = fx.countEventKinds()
	for _, kind := range []computerevent.EventKind{
		computerevent.EventEffectAccepted, computerevent.EventMaterializationStarted,
		computerevent.EventMaterializationApplied, computerevent.EventCheckpointPublished,
		computerevent.EventRouteProjectionUpdated,
	} {
		if kinds[kind] != 1 {
			t.Fatalf("event %s count = %d, want exactly 1 (no duplicated tail)", kind, kinds[kind])
		}
	}
}

func mustOfferDigest(t *testing.T, offer selfdevprotocol.PlatformUpdateOffer) string {
	t.Helper()
	digest, err := offer.Digest()
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

func mustRouteSlotID(t *testing.T) string {
	t.Helper()
	slotID, err := routeledger.RouteSlotID("owner", "primary")
	if err != nil {
		t.Fatal(err)
	}
	return slotID
}
