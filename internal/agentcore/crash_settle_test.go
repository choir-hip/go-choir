package agentcore

// L1a-3a (docs/problems/crash-boots-redrive-applies-2026-10-13.md): a crash
// boot settles a pending platform update from the updater journal and never
// calls the updater again. Ways this could fail, written first:
//   - the crash boot re-drives Apply (the guest restarts again: a loop);
//   - an unstarted update stays pending, so every later offer is refused;
//   - an update the updater finished is recorded as failed (a false tape);
//   - a mid-flight journal is settled as failed while the release may be live;
//   - a planned boot stops resuming its own update;
//   - a successful return leaves the planned marker on disk, so a later
//     crash boots as planned.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

// seedAcceptedPlatformUpdate commits only the accepted event with the offer
// pinned as its payload: the state a guest is in when the runtime died
// before or during the apply.
func (fx *derivableSelfDevFixture) seedAcceptedPlatformUpdate(t *testing.T, offer selfdevprotocol.PlatformUpdateOffer) string {
	t.Helper()
	ctx := context.Background()
	offerDigest := mustOfferDigest(t, offer)
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
	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	accepted := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: offer.ComputerID,
		EventKind: computerevent.EventEffectAccepted, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: platformUpdateIdempotencyKey("accepted", offer.UpdateID),
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
	if _, _, err := fx.rt.eventAppender.AppendNewPayload(ctx, accepted, computerevent.TransitionInput{TargetStateCommitment: targetCommitment}, offerBytes, "application/vnd.choir.platform-update-offer+json", "owner"); err != nil {
		t.Fatalf("seed accepted event: %v", err)
	}
	head, err = fx.store.Head(ctx, fx.computerID)
	if err != nil || head == nil || head.PendingTransitionRef == "" {
		t.Fatalf("pending transition did not open: %v %#v", err, head)
	}
	return head.PendingTransitionRef
}

func (fx *derivableSelfDevFixture) pendingTransition(t *testing.T) string {
	t.Helper()
	head, err := fx.store.Head(context.Background(), fx.computerID)
	if err != nil || head == nil {
		t.Fatalf("head: %v", err)
	}
	return head.PendingTransitionRef
}

func TestCrashBootSettlesUnstartedPlatformUpdateAsFailed(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-crash-settle-unstarted")
	ctx := context.Background()
	offer := fx.mintPlatformUpdateOffer(t, "update-unstarted", "<html>unstarted</html>", fx.currentHead(t))
	fx.seedAcceptedPlatformUpdate(t, offer)

	fx.rt.settlePendingPlatformUpdateAfterCrash(ctx)

	if calls := fx.applyCalls.Load(); calls != 0 || fx.service.restarts != 0 {
		t.Fatalf("crash boot called the updater %d times (%d restarts); it must not", calls, fx.service.restarts)
	}
	kinds := fx.countEventKinds()
	if kinds[computerevent.EventMaterializationFailed] != 1 || kinds[computerevent.EventMaterializationApplied] != 0 {
		t.Fatalf("events = %v; want one materialization_failed and no applied", kinds)
	}
	if pending := fx.pendingTransition(t); pending != "" {
		t.Fatalf("pending transition %s still bound after crash settlement — every later offer would be refused", pending)
	}
	// The interruption is visible on the tape.
	failed, found, err := fx.store.EventByIdempotency(ctx, fx.computerID, platformUpdateIdempotencyKey("failed", offer.UpdateID))
	if err != nil || !found {
		t.Fatalf("failed event: found=%v err=%v", found, err)
	}
	raw, err := fx.rt.eventPayloadReader.FetchPayload(ctx, fx.computerID, failed.PayloadCommitment)
	if err != nil {
		t.Fatalf("failed payload: %v", err)
	}
	var result struct {
		Outcome string `json:"outcome"`
	}
	if err := json.Unmarshal(raw, &result); err != nil || result.Outcome != platformUpdateInterruptedByRestart {
		t.Fatalf("failed payload outcome = %q (%v); want %q", result.Outcome, err, platformUpdateInterruptedByRestart)
	}

	next := fx.mintPlatformUpdateOffer(t, "update-after-crash", "<html>after</html>", fx.currentHead(t))
	if _, err := fx.rt.ApplyPlatformUpdate(ctx, next); err != nil {
		t.Fatalf("a fresh offer after crash settlement was refused: %v", err)
	}
}

func TestCrashBootRecordsJournaledApplyWithoutReapplying(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-crash-settle-applied")
	ctx := context.Background()
	offer := fx.mintPlatformUpdateOffer(t, "update-journaled", "<html>journaled</html>", fx.currentHead(t))
	acceptedDigest := fx.seedAcceptedPlatformUpdate(t, offer)
	fx.useDurableAttemptCount(t)

	// The updater finished the apply after the runtime that asked for it died.
	request, err := fx.rt.platformUpdateApplyRequest(ctx, offer, acceptedDigest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fx.engine.Apply(ctx, request); err != nil {
		t.Fatalf("seed journal: %v", err)
	}
	restartsBefore := fx.service.restarts

	fx.rt.settlePendingPlatformUpdateAfterCrash(ctx)

	if calls := fx.applyCalls.Load(); calls != 0 || fx.service.restarts != restartsBefore {
		t.Fatalf("crash settlement re-applied: %d apply calls, restarts %d -> %d", calls, restartsBefore, fx.service.restarts)
	}
	kinds := fx.countEventKinds()
	if kinds[computerevent.EventMaterializationApplied] != 1 || kinds[computerevent.EventMaterializationFailed] != 0 {
		t.Fatalf("events = %v; want the journaled apply recorded as applied", kinds)
	}
	if pending := fx.pendingTransition(t); pending != "" {
		t.Fatalf("pending transition %s still bound after recording the journaled apply", pending)
	}
}

func TestCrashBootLeavesMidFlightApplyPending(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-crash-settle-midflight")
	ctx := context.Background()
	offer := fx.mintPlatformUpdateOffer(t, "update-midflight", "<html>midflight</html>", fx.currentHead(t))
	acceptedDigest := fx.seedAcceptedPlatformUpdate(t, offer)

	// A journal the updater never finished: the effective state is unknown.
	key := platformUpdateIdempotencyKey("apply", offer.UpdateID)
	sum := sha256.Sum256([]byte(key))
	dir := filepath.Join(fx.updaterRoot, "operations")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, hex.EncodeToString(sum[:])+".json"), []byte(`{"phase":"pointer_swapped","request_commitment":"x","target_release_digest":"y"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	fx.rt.settlePendingPlatformUpdateAfterCrash(ctx)

	if calls := fx.applyCalls.Load(); calls != 0 || fx.service.restarts != 0 {
		t.Fatalf("crash settlement called the updater %d times (%d restarts)", calls, fx.service.restarts)
	}
	kinds := fx.countEventKinds()
	if kinds[computerevent.EventMaterializationFailed] != 0 || kinds[computerevent.EventMaterializationApplied] != 0 {
		t.Fatalf("events = %v; a mid-flight journal must not be settled either way", kinds)
	}
	if pending := fx.pendingTransition(t); pending != acceptedDigest {
		t.Fatalf("pending transition = %q; want it left on %q (fail-closed)", pending, acceptedDigest)
	}
}

func TestPlannedBootStillResumesPlatformUpdate(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-crash-settle-planned")
	ctx := context.Background()
	offer := fx.mintPlatformUpdateOffer(t, "update-planned", "<html>planned</html>", fx.currentHead(t))
	fx.seedAcceptedPlatformUpdate(t, offer)

	fx.rt.bootRestartPlanned = true
	fx.rt.settleOrResumePlatformUpdate(ctx)

	if count := fx.countEventKinds()[computerevent.EventMaterializationApplied]; count != 1 {
		t.Fatalf("planned boot: materialization_applied = %d, want 1", count)
	}
}

func TestCrashBootDispatchesToSettlement(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-crash-settle-dispatch")
	ctx := context.Background()
	offer := fx.mintPlatformUpdateOffer(t, "update-dispatch", "<html>dispatch</html>", fx.currentHead(t))
	fx.seedAcceptedPlatformUpdate(t, offer)

	fx.rt.bootRestartPlanned = false
	fx.rt.settleOrResumePlatformUpdate(ctx)

	if fx.applyCalls.Load() != 0 || fx.countEventKinds()[computerevent.EventMaterializationFailed] != 1 {
		t.Fatalf("crash boot: restarts=%d events=%v; want settlement, not a resume", fx.service.restarts, fx.countEventKinds())
	}
}

// L1a-3b: a crash boot settles every self-development op decided before the
// boot from the updater journal and never calls the updater for it. Ways
// this could fail, written first: a decided op the updater never started is
// applied after the crash; it stays open and keeps the transition pending; a
// finished apply is applied again or recorded as failed; a mid-flight journal
// is settled either way; an op decided after the boot is held as if it were
// interrupted.

func (fx *derivableSelfDevFixture) decideWithoutActing(t *testing.T, key string) selfdev.Operation {
	t.Helper()
	operation, bundleDigest, _ := fx.seedOperationToAwaitingApproval(t, key)
	// The guest dies after the owner's decision commits and before the
	// reconciler acts on it.
	fx.rt.eventAppender.SetPostCommitObserver(func(computerevent.EventKind) {})
	fx.appendDecision(t, operation, bundleDigest, "decision-"+key)
	return operation
}

func (fx *derivableSelfDevFixture) decisionDigest(t *testing.T, key string) string {
	t.Helper()
	decision, found, err := fx.store.EventByIdempotency(context.Background(), fx.computerID, "decision-"+key)
	if err != nil || !found {
		t.Fatalf("decision %s: found=%v err=%v", key, found, err)
	}
	digest, err := decision.Digest()
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

func TestCrashBootFailsDecidedSelfDevelopmentOpTheUpdaterNeverStarted(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-crash-selfdev-unstarted")
	ctx := context.Background()
	operation := fx.decideWithoutActing(t, "op-unstarted")

	fx.rt.closeSelfDevelopmentOperationsAfterCrash(ctx)
	fx.rt.reconcileSelfDevelopmentMaterialization(ctx)

	got := fx.waitForState(t, operation.OperationID, selfdev.StateFailed)
	if got.TerminalError != selfdevInterruptedByRestart {
		t.Fatalf("terminal error = %q, want %q", got.TerminalError, selfdevInterruptedByRestart)
	}
	if calls := fx.applyCalls.Load(); calls != 0 {
		t.Fatalf("crash boot called the updater %d times for an op decided before it", calls)
	}
	if pending := fx.pendingTransition(t); pending != "" {
		t.Fatalf("pending transition %s still bound after the op failed", pending)
	}
	if kinds := fx.countEventKinds(); kinds[computerevent.EventMaterializationFailed] != 1 || kinds[computerevent.EventMaterializationApplied] != 0 {
		t.Fatalf("events = %v; want one materialization_failed", kinds)
	}
}

func TestCrashBootRecordsJournaledSelfDevelopmentApply(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-crash-selfdev-journaled")
	ctx := context.Background()
	fx.controlFail.Store(true)
	operation, bundleDigest, _ := fx.seedOperationToAwaitingApproval(t, "op-journaled")
	fx.appendDecision(t, operation, bundleDigest, "decision-op-journaled")
	// The apply finished in the updater; recording it failed (control plane
	// down at checkpoint), so the op is parked materializing. The state
	// flips before the apply call lands, so wait for the terminal journal and
	// for the reconcile pass to let go before counting calls.
	fx.waitForState(t, operation.OperationID, selfdev.StateMaterializing)
	applyKey := "selfdev-apply-" + fx.decisionDigest(t, "op-journaled")
	deadline := time.Now().Add(30 * time.Second)
	for {
		journaled, found, err := updater.ReadJournalOutcome(fx.updaterRoot, applyKey)
		if err == nil && found && journaled.Terminal {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("apply journal never became terminal (found=%v err=%v)", found, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	// The guest dies here: no further commit triggers a pass. Wait until the
	// passes already queued have drained and the call count is stable.
	fx.rt.eventAppender.SetPostCommitObserver(func(computerevent.EventKind) {})
	for stable, last := 0, int32(-1); stable < 3; {
		fx.rt.selfdevMaterializeMu.Lock()
		calls := fx.applyCalls.Load()
		fx.rt.selfdevMaterializeMu.Unlock()
		if calls == last {
			stable++
		} else {
			stable, last = 0, calls
		}
		time.Sleep(200 * time.Millisecond)
	}
	fx.controlFail.Store(false)
	fx.useDurableAttemptCount(t)

	fx.rt.closeSelfDevelopmentOperationsAfterCrash(ctx)
	callsBefore := fx.applyCalls.Load()
	fx.rt.reconcileSelfDevelopmentMaterialization(ctx)

	fx.waitForState(t, operation.OperationID, selfdev.StateApplied)
	if calls := fx.applyCalls.Load(); calls != callsBefore {
		t.Fatalf("crash settlement called the updater again (%d -> %d)", callsBefore, calls)
	}
	if kinds := fx.countEventKinds(); kinds[computerevent.EventMaterializationApplied] != 1 {
		t.Fatalf("events = %v; want the journaled apply recorded once", kinds)
	}
}

func TestCrashBootHoldsSelfDevelopmentOpWithMidFlightJournal(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-crash-selfdev-midflight")
	ctx := context.Background()
	operation := fx.decideWithoutActing(t, "op-midflight")
	sum := sha256.Sum256([]byte("selfdev-apply-" + fx.decisionDigest(t, "op-midflight")))
	dir := filepath.Join(fx.updaterRoot, "operations")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, hex.EncodeToString(sum[:])+".json"), []byte(`{"phase":"restart_requested","request_commitment":"x","target_release_digest":"y"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	fx.rt.closeSelfDevelopmentOperationsAfterCrash(ctx)
	fx.rt.reconcileSelfDevelopmentMaterialization(ctx)
	fx.rt.reconcileSelfDevelopmentMaterialization(ctx)

	if calls := fx.applyCalls.Load(); calls != 0 {
		t.Fatalf("crash settlement called the updater %d times on a mid-flight journal", calls)
	}
	got, err := fx.rt.selfdevOperations.Get(ctx, fx.computerID, operation.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State == selfdev.StateFailed || got.State == selfdev.StateApplied {
		t.Fatalf("op settled as %s on a mid-flight journal; want it held", got.State)
	}
	if kinds := fx.countEventKinds(); kinds[computerevent.EventMaterializationFailed] != 0 || kinds[computerevent.EventMaterializationApplied] != 0 {
		t.Fatalf("events = %v; a mid-flight journal settles nothing", kinds)
	}
}

func TestOpDecidedAfterCrashBootMaterializes(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-crash-selfdev-after")
	fx.rt.closeSelfDevelopmentOperationsAfterCrash(context.Background())

	operation, bundleDigest, _ := fx.seedOperationToAwaitingApproval(t, "op-after")
	fx.appendDecision(t, operation, bundleDigest, "decision-op-after")

	fx.waitForState(t, operation.OperationID, selfdev.StateApplied)
}
