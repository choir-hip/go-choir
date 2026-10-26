package agentcore

// Review 309, findings 1 and 5 (docs/problems/apply-checkpoint-replay-fills-guest-tmp-2026-10-26.md,
// "Review 309"): a crash boot re-drove the platform update's checkpoint and
// route tail with no bound, so a tail that crashed the computer every time ran
// on every boot. Written before the code. Ways this could fail:
//
//  1. the crash boot re-drives the tail on every boot (an unbounded loop), on
//     either path: a pending transition whose journal is `applied`, and an
//     applied update whose route event never landed;
//  2. the attempt is not durable before the work, so a tail that dies in the
//     middle is not counted;
//  3. the two paths count separately, so one update gets four crash-boot tails;
//  4. a planned, marker-backed restart is bounded or counted as a crash boot
//     (it may resume once; a resume that crashes is a markerless boot);
//  5. a runtime with no durable attempt count runs the tail unbounded instead
//     of refusing.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

// useDurableAttemptCount gives the runtime a store path, so the durable
// attempt count exists beside it.
func (fx *derivableSelfDevFixture) useDurableAttemptCount(t *testing.T) {
	t.Helper()
	fx.rt.cfg.StorePath = filepath.Join(t.TempDir(), "runtime.db")
}

// guestAttemptCount reads the durable count for one reason and target.
func (fx *derivableSelfDevFixture) guestAttemptCount(t *testing.T, reason, target string) int {
	t.Helper()
	raw, err := os.ReadFile(GuestApplyAttemptsPath(fx.rt.cfg.StorePath))
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	if err := json.Unmarshal(raw, &counts); err != nil {
		t.Fatal(err)
	}
	return counts[reason+" "+target]
}

// strandPlatformUpdateTail applies an update with the control plane down:
// `applied` commits, the checkpoint is refused, and the tail is stranded with
// the transition already closed.
func (fx *derivableSelfDevFixture) strandPlatformUpdateTail(t *testing.T, updateID string) selfdevprotocol.PlatformUpdateOffer {
	t.Helper()
	offer := fx.mintPlatformUpdateOffer(t, updateID, "<html>"+updateID+"</html>", fx.currentHead(t))
	fx.controlFail.Store(true)
	if _, err := fx.rt.ApplyPlatformUpdate(context.Background(), offer); err == nil {
		t.Fatal("update with a dead control plane should fail")
	}
	kinds := fx.countEventKinds()
	if kinds[computerevent.EventMaterializationApplied] != 1 || kinds[computerevent.EventCheckpointPublished] != 0 {
		t.Fatalf("expected an applied-without-checkpoint strand, got %v", kinds)
	}
	return offer
}

func (fx *derivableSelfDevFixture) crashBootPlatformUpdate() {
	fx.rt.bootRestartPlanned = false
	fx.rt.settleOrResumePlatformUpdate(context.Background())
}

// Failure modes 1 and 2: the stranded tail re-drives on two crash boots and
// then stops, even when the control plane is back.
func TestCrashBootStrandedPlatformUpdateTailIsBoundedToTwoBoots(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-pu-strand-bound")
	offer := fx.strandPlatformUpdateTail(t, "update-strand-bound")
	fx.useDurableAttemptCount(t)

	fx.crashBootPlatformUpdate() // control plane still down: the tail fails
	fx.crashBootPlatformUpdate()
	if got := fx.countEventKinds()[computerevent.EventRecoveryRecorded]; got != 0 {
		t.Fatalf("a held event before the bound was spent: %d", got)
	}
	if n := fx.guestAttemptCount(t, platformUpdateSettlementAttemptReason, offer.UpdateID); n != 2 {
		t.Fatalf("attempts after two crash boots = %d, want 2", n)
	}

	fx.controlFail.Store(false) // the third boot could now finish, and must not try
	fx.crashBootPlatformUpdate()

	if n := fx.guestAttemptCount(t, platformUpdateSettlementAttemptReason, offer.UpdateID); n != 2 {
		t.Fatalf("attempts after the third boot = %d, want the two crash boots only", n)
	}
	kinds := fx.countEventKinds()
	if kinds[computerevent.EventCheckpointPublished] != 0 || kinds[computerevent.EventRouteProjectionUpdated] != 0 || len(fx.checkpoints) != 0 {
		t.Fatalf("the spent tail ran: events=%v checkpoints=%d", kinds, len(fx.checkpoints))
	}
	fx.requireOneSettlementHeldEvent(t, offer.UpdateID)
	fx.crashBootPlatformUpdate() // a fourth boot adds nothing
	fx.requireOneSettlementHeldEvent(t, offer.UpdateID)
}

// Failure mode 3: the pending path and the stranded path share one count for
// one update. Boot 1 records the journaled apply (the tail fails), boot 2
// re-drives the strand; the third boot does neither.
func TestCrashBootPendingAndStrandedPlatformUpdateShareOneBound(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-pu-pending-bound")
	ctx := context.Background()
	offer := fx.mintPlatformUpdateOffer(t, "update-pending-bound", "<html>pending-bound</html>", fx.currentHead(t))
	acceptedDigest := fx.seedAcceptedPlatformUpdate(t, offer)
	fx.useDurableAttemptCount(t)
	request, err := fx.rt.platformUpdateApplyRequest(ctx, offer, acceptedDigest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fx.engine.Apply(ctx, request); err != nil {
		t.Fatalf("seed journal: %v", err)
	}
	fx.controlFail.Store(true)

	fx.crashBootPlatformUpdate() // pending: records applied, the tail fails
	if kinds := fx.countEventKinds(); kinds[computerevent.EventMaterializationApplied] != 1 || kinds[computerevent.EventCheckpointPublished] != 0 {
		t.Fatalf("boot 1 events = %v, want applied without a checkpoint", kinds)
	}
	fx.crashBootPlatformUpdate() // stranded: the tail fails again
	if n := fx.guestAttemptCount(t, platformUpdateSettlementAttemptReason, offer.UpdateID); n != 2 {
		t.Fatalf("attempts after two crash boots = %d, want 2", n)
	}

	fx.controlFail.Store(false)
	fx.crashBootPlatformUpdate()

	if n := fx.guestAttemptCount(t, platformUpdateSettlementAttemptReason, offer.UpdateID); n != 2 {
		t.Fatalf("attempts after the third boot = %d, want 2", n)
	}
	if kinds := fx.countEventKinds(); kinds[computerevent.EventCheckpointPublished] != 0 || kinds[computerevent.EventRouteProjectionUpdated] != 0 {
		t.Fatalf("the spent tail ran on the third boot: %v", kinds)
	}
	fx.requireOneSettlementHeldEvent(t, offer.UpdateID)
}

// Failure mode 4: a planned restart resumes the tail once and spends no
// crash-settlement attempt; the planned path is bounded by the marker, not by
// this count.
func TestPlannedBootResumesStrandedPlatformUpdateTailWithoutSpendingTheCrashBound(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-pu-planned-tail")
	offer := fx.strandPlatformUpdateTail(t, "update-planned-tail")
	fx.useDurableAttemptCount(t)
	fx.controlFail.Store(false)

	fx.rt.bootRestartPlanned = true
	fx.rt.settleOrResumePlatformUpdate(context.Background())

	if kinds := fx.countEventKinds(); kinds[computerevent.EventRouteProjectionUpdated] != 1 {
		t.Fatalf("planned boot did not finish the tail: %v", kinds)
	}
	if n := fx.guestAttemptCount(t, platformUpdateSettlementAttemptReason, offer.UpdateID); n != 0 {
		t.Fatalf("a planned boot spent %d crash-settlement attempts", n)
	}
}

// Failure mode 5: with no durable attempt count the crash boot refuses to
// re-drive the tail, on both paths.
func TestCrashBootPlatformUpdateTailRefusesWithoutADurableAttemptCount(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-pu-unbounded")
	ctx := context.Background()
	fx.strandPlatformUpdateTail(t, "update-unbounded-strand")
	fx.controlFail.Store(false)
	fx.rt.cfg.StorePath = ""

	fx.crashBootPlatformUpdate()

	if kinds := fx.countEventKinds(); kinds[computerevent.EventCheckpointPublished] != 0 || kinds[computerevent.EventRouteProjectionUpdated] != 0 {
		t.Fatalf("an unbounded stranded tail ran: %v", kinds)
	}

	// The pending path: a journaled apply is left pending, unrecorded.
	fx2 := newDerivableSelfDevFixture(t, "computer-pu-unbounded-pending")
	offer := fx2.mintPlatformUpdateOffer(t, "update-unbounded-pending", "<html>unbounded-pending</html>", fx2.currentHead(t))
	acceptedDigest := fx2.seedAcceptedPlatformUpdate(t, offer)
	request, err := fx2.rt.platformUpdateApplyRequest(ctx, offer, acceptedDigest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fx2.engine.Apply(ctx, request); err != nil {
		t.Fatalf("seed journal: %v", err)
	}
	fx2.rt.cfg.StorePath = ""

	fx2.crashBootPlatformUpdate()

	// The apply is known (a terminal journal, verified receipts): it is
	// recorded, which closes the pending transition and unblocks later
	// updates. Only the tail, the work that can crash, is refused.
	kinds := fx2.countEventKinds()
	if kinds[computerevent.EventMaterializationApplied] != 1 || kinds[computerevent.EventCheckpointPublished] != 0 || kinds[computerevent.EventRouteProjectionUpdated] != 0 {
		t.Fatalf("an unbounded settlement: events = %v, want the apply recorded and no tail", kinds)
	}
	if pending := fx2.pendingTransition(t); pending != "" {
		t.Fatalf("pending transition %s still bound: every later offer, security updates included, would be refused", pending)
	}
}

// requireOneSettlementHeldEvent: a spent tail leaves exactly one causal event
// on the tape that names the update and the cause. It is not a failed event:
// the release is live.
func (fx *derivableSelfDevFixture) requireOneSettlementHeldEvent(t *testing.T, updateID string) {
	t.Helper()
	ctx := context.Background()
	kinds := fx.countEventKinds()
	if kinds[computerevent.EventRecoveryRecorded] != 1 {
		t.Fatalf("recovery_recorded events = %d, want exactly one held event: %v", kinds[computerevent.EventRecoveryRecorded], kinds)
	}
	if kinds[computerevent.EventMaterializationFailed] != 0 {
		t.Fatalf("a held tail wrote materialization_failed, which falsifies a live release: %v", kinds)
	}
	held, found, err := fx.store.EventByIdempotency(ctx, fx.computerID, platformUpdateIdempotencyKey("settlement-held", updateID))
	if err != nil || !found {
		t.Fatalf("held event: found=%v err=%v", found, err)
	}
	raw, err := fx.rt.eventPayloadReader.FetchPayload(ctx, fx.computerID, held.PayloadCommitment)
	if err != nil {
		t.Fatalf("held payload: %v", err)
	}
	var payload struct {
		UpdateID string `json:"update_id"`
		Cause    string `json:"cause"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil || payload.UpdateID != updateID || payload.Cause == "" {
		t.Fatalf("held payload = %s (%v), want the update id and a cause", raw, err)
	}
}

// Review 315, finding 1: the spend gates the tail, not the applied event. Two
// pre-append failures after a journaled `applied` must not spend the bound: the
// third boot still records the apply and closes the pending transition.
func TestCrashBootRecordsAppliedAfterTwoFailedAppendsWithoutSpendingTheTailBound(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-pu-append-fails")
	ctx := context.Background()
	offer := fx.mintPlatformUpdateOffer(t, "update-append-fails", "<html>append-fails</html>", fx.currentHead(t))
	acceptedDigest := fx.seedAcceptedPlatformUpdate(t, offer)
	fx.useDurableAttemptCount(t)
	request, err := fx.rt.platformUpdateApplyRequest(ctx, offer, acceptedDigest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fx.engine.Apply(ctx, request); err != nil {
		t.Fatalf("seed journal: %v", err)
	}
	real := appendEventPayload
	defer func() { appendEventPayload = real }()
	failing := true
	appendEventPayload = func(rt *Runtime, ctx context.Context, event computerevent.Event, input computerevent.TransitionInput, payload []byte, mediaType, privacyClass string) (computerevent.Receipt, string, error) {
		if failing {
			return computerevent.Receipt{}, "", fmt.Errorf("store mid-replay: append unavailable")
		}
		return real(rt, ctx, event, input, payload, mediaType, privacyClass)
	}

	fx.crashBootPlatformUpdate()
	fx.crashBootPlatformUpdate()
	if n := fx.guestAttemptCount(t, platformUpdateSettlementAttemptReason, offer.UpdateID); n != 0 {
		t.Fatalf("two failed applied appends spent %d tail attempts", n)
	}
	if fx.pendingTransition(t) != acceptedDigest {
		t.Fatal("the transition closed while the append was failing")
	}

	failing = false
	fx.crashBootPlatformUpdate()

	kinds := fx.countEventKinds()
	if kinds[computerevent.EventMaterializationApplied] != 1 || kinds[computerevent.EventRouteProjectionUpdated] != 1 {
		t.Fatalf("third boot events = %v, want the apply recorded and the tail finished", kinds)
	}
	if pending := fx.pendingTransition(t); pending != "" {
		t.Fatalf("pending transition %s still bound", pending)
	}
	if n := fx.guestAttemptCount(t, platformUpdateSettlementAttemptReason, offer.UpdateID); n != 1 {
		t.Fatalf("tail attempts = %d, want the one tail", n)
	}
}

// Review 315, finding 3: a stranded offer the apply would refuse (a retired
// realization, an expired offer) does not spend the bound.
func TestCrashBootStrandedTailOfARetiredRealizationSpendsNoAttempt(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-pu-strand-retired")
	offer := fx.strandPlatformUpdateTail(t, "update-strand-retired")
	fx.useDurableAttemptCount(t)
	fx.controlFail.Store(false)
	fx.rt.selfdevRealizationID = "realization-retired-since"

	fx.crashBootPlatformUpdate()

	if n := fx.guestAttemptCount(t, platformUpdateSettlementAttemptReason, offer.UpdateID); n != 0 {
		t.Fatalf("a tail the apply would refuse spent %d attempts", n)
	}
	if kinds := fx.countEventKinds(); kinds[computerevent.EventRouteProjectionUpdated] != 0 {
		t.Fatalf("a retired realization's tail ran: %v", kinds)
	}
	// Review 323 (both seats): a tail that cannot be re-driven is visible on
	// the tape too, once, not only in a log line every boot.
	fx.requireOneSettlementHeldEvent(t, offer.UpdateID)
	fx.crashBootPlatformUpdate()
	fx.requireOneSettlementHeldEvent(t, offer.UpdateID)
}

// Review 323 (Devin, Medium): a journaled terminal `refused` is provably
// pre-mutation. A crash boot discharges it as failed, like a missing journal;
// before, it fell to the "no verified recovery" branch and left the transition
// pending, so every later offer was refused.
func TestCrashBootSettlesAJournaledRefusalAsFailed(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-pu-refused-journal")
	ctx := context.Background()
	offer := fx.mintPlatformUpdateOffer(t, "update-refused-journal", "<html>refused-journal</html>", fx.currentHead(t))
	fx.seedAcceptedPlatformUpdate(t, offer)
	fx.useDurableAttemptCount(t)
	fx.writeTerminalRefusedJournal(t, platformUpdateIdempotencyKey("apply", offer.UpdateID))

	fx.crashBootPlatformUpdate()

	kinds := fx.countEventKinds()
	if kinds[computerevent.EventMaterializationFailed] != 1 || kinds[computerevent.EventMaterializationApplied] != 0 {
		t.Fatalf("events = %v, want one materialization_failed", kinds)
	}
	if pending := fx.pendingTransition(t); pending != "" {
		t.Fatalf("pending transition %s still bound after a refused journal", pending)
	}
	if calls := fx.applyCalls.Load(); calls != 0 {
		t.Fatalf("the crash boot called the updater %d times", calls)
	}
	next := fx.mintPlatformUpdateOffer(t, "update-after-refused", "<html>after-refused</html>", fx.currentHead(t))
	if _, err := fx.rt.ApplyPlatformUpdate(ctx, next); err != nil {
		t.Fatalf("a later offer was refused after the refused journal settled: %v", err)
	}
}
