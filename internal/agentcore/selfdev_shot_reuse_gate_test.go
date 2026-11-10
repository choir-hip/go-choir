package agentcore

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

// S5: the shots-owed text of a clean rebase says BEFORE already holds the
// change; an ordinary operation's text is unchanged.
func TestShotsOwedLineFramesARebase(t *testing.T) {
	if line := shotsOwedLine("clean-rebase-abc"); !strings.Contains(line, shotsOwedRebaseNote) || !strings.Contains(line, shotsOwedProcedure) {
		t.Fatalf("a rebase's line lacks the framing: %q", line)
	}
	if line := shotsOwedLine("selfdev-abc"); strings.Contains(line, shotsOwedRebaseNote) {
		t.Fatalf("an ordinary operation's line carries the rebase framing: %q", line)
	}
}

// F5, F8: a run still running answers pending and starts nothing; a run that
// passed commits the event once and proceeds; a run that was refused leaves the
// judged path.
func TestShotReuseGateAnswersFromTheRunsState(t *testing.T) {
	fx, _ := newHostBuildFixture(t, "computer-shot-reuse-runs", "done")
	var captured struct {
		operation selfdev.Operation
		bundle    transaction.CapsuleEffectBundle
		built     hostBuiltRelease
	}
	fx.configure(func(rt *Runtime) {
		rt.shotsAskedHook = func(operation selfdev.Operation, bundle transaction.CapsuleEffectBundle, built hostBuiltRelease) {
			captured.operation, captured.bundle, captured.built = operation, bundle, built
		}
	})
	operation, bundleDigest, _ := fx.seedOperationToAwaitingApproval(t, "op-shot-reuse-runs")
	fx.appendDecision(t, operation, bundleDigest, "decision-shot-reuse-runs")
	accepted := fx.waitForState(t, operation.OperationID, selfdev.StateAccepted)
	if err := fx.rt.materializeSelfDevelopmentOperation(context.Background(), accepted, false); !errors.Is(err, errCandidateShotPending) {
		t.Fatalf("setup pass = %v", err)
	}
	if captured.built.Job.JobID == "" {
		t.Fatal("the judged path did not run")
	}
	ctx := context.Background()
	planDigest, _ := captured.bundle.ShotPlan.Digest()
	key := shotReuseRunKey(captured.operation.OperationID, captured.built.Job.JobID)
	gate := func() (bool, error) {
		return fx.rt.shotReuseGate(ctx, captured.operation, captured.bundle, &captured.built, planDigest)
	}

	fx.rt.setShotReuseRun(key, &shotReuseRun{state: shotReuseRunning})
	if handled, err := gate(); !handled || !errors.Is(err, errCandidateShotPending) {
		t.Fatalf("running = handled %v, %v; want pending", handled, err)
	}
	fx.rt.setShotReuseRun(key, &shotReuseRun{state: shotReuseRefused})
	if handled, err := gate(); handled || err != nil {
		t.Fatalf("refused = handled %v, %v; want the judged path", handled, err)
	}

	claim, why := decideShotReuse(goodShotReuseFacts())
	if why != "" {
		t.Fatal(why)
	}
	fx.rt.setShotReuseRun(key, &shotReuseRun{state: shotReusePassed, claim: claim, jobID: "liveness-test",
		liveness: computerevent.ShotLiveness{Passed: true, Requests: 2, Marker: shotLivenessMarker}})
	if handled, err := gate(); !handled || err != nil {
		t.Fatalf("passed = handled %v, %v; want the swap to proceed", handled, err)
	}
	if fx.rt.getShotReuseRun(key) != nil {
		t.Fatal("a finished run stayed in the map (review round 2, N3)")
	}
	if kinds := fx.countEventKinds(); kinds[computerevent.EventShotReuse] != 1 {
		t.Fatalf("events = %v, want one shot_reuse", kinds)
	}
	// A replay on the occupied key reads it through the reader and is fine.
	if err := fx.rt.appendShotReuse(ctx, captured.operation, captured.built.Job.JobID, planDigest, shotReuseRun{claim: claim, jobID: "liveness-test",
		liveness: computerevent.ShotLiveness{Passed: true, Requests: 2, Marker: shotLivenessMarker}}); err != nil {
		t.Fatalf("a replay = %v", err)
	}
	// The gate's own lookup now reads the event back and the swap goes.
	fx.rt.reconcileSelfDevelopmentMaterialization(ctx)
	fx.waitForState(t, operation.OperationID, selfdev.StateApplied)
}
