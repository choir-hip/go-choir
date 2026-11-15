package agentcore

import (
	"context"
	"fmt"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/recoveryplan"
)

// Review round 2 of lane/snapshot-by-replay-time: the plan offered only the
// newest four snapshot events below the target, so a live root the host kept
// a dozen snapshots lower was never tried. The plan offers every snapshot event
// below the target within the recovery bound; the install probes the host.
func TestPlanRestoreTargetOffersEverySnapshotWithinTheBound(t *testing.T) {
	ctx := context.Background()
	const computerID = "computer-restore-window"
	head := func(seq uint64) string { return fmt.Sprintf("%064x", seq) }
	bound := recoveryplan.MaxRecoveryTailEvents
	// Twelve snapshots 1,000 events apart ending at the target, two older ones
	// beyond the bound.
	var at []uint64
	at = append(at, 10, 100)
	target := bound + 200_000
	for i := uint64(0); i < 12; i++ {
		at = append(at, target-i*1000-3)
	}
	tape := &lazyChainTape{restoreTapeFake: newRestoreTapeFake(nil), n: target + 50, snapshotAt: at, head: head}
	plan, err := planRestoreTarget(ctx, tape, computerID, head(target))
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.candidates) != 12 {
		t.Fatalf("%d candidates, want the 12 inside the bound (the old newest-four rule offered 4)", len(plan.candidates))
	}
	for i := 1; i < len(plan.candidates); i++ {
		if plan.candidates[i].Sequence <= plan.candidates[i-1].Sequence {
			t.Fatalf("candidates are not oldest first: %+v", plan.candidates)
		}
	}
	if plan.candidates[0].Sequence < target-bound {
		t.Fatalf("candidate %d is older than the bound behind the target %d", plan.candidates[0].Sequence, target)
	}
}
