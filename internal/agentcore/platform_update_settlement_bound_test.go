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

	if kinds := fx2.countEventKinds(); kinds[computerevent.EventMaterializationApplied] != 0 {
		t.Fatalf("an unbounded settlement recorded the apply: %v", kinds)
	}
	if pending := fx2.pendingTransition(t); pending != acceptedDigest {
		t.Fatalf("pending transition = %q, want it held on %q", pending, acceptedDigest)
	}
}
