package agentcore

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// The candidate-shot interlock (selfdev_candidate_shot.go, design 3.3,
// FM11). Failure modes, decided before the code:
//
//   - an unshot platform candidate swaps in: no event, a failing event, a
//     foreign event (another host build) or no plan must each keep the
//     updater uncalled;
//   - a pending pass starts materialization or holds the desks;
//   - a failing verdict leaves the operation open instead of failed with
//     "shot review: ...";
//   - a journal-only settlement (which calls no updater) runs the gate;
//   - a source-only bundle that was not host-built builds an empty plain
//     manifest instead of failing by name (FM1).

func TestCandidateShotInterlockHoldsTheSwapUntilAPassingShot(t *testing.T) {
	fx, host := newHostBuildFixture(t, "computer-shot-pending", "done")
	var started atomic.Int32
	var write atomic.Bool
	fx.configure(func(rt *Runtime) {
		rt.candidateShotStarter = func(operation selfdev.Operation, bundle transaction.CapsuleEffectBundle, built hostBuiltRelease) {
			started.Add(1)
			if write.Load() {
				testWriteCandidateShot(t, rt, operation, bundle, built, computerevent.CandidateShotPass, "")
			}
		}
	})
	operation, bundleDigest, _ := fx.seedOperationToAwaitingApproval(t, "op-shot-pending")
	fx.appendDecision(t, operation, bundleDigest, "decision-shot-pending")
	accepted := fx.waitForState(t, operation.OperationID, selfdev.StateAccepted)
	// The decision's own reconcile may already have run a pass; run one more.
	err := fx.rt.materializeSelfDevelopmentOperation(context.Background(), accepted, false)
	if !errors.Is(err, errCandidateShotPending) {
		t.Fatalf("a pass with no candidate_shot = %v, want pending", err)
	}
	if started.Load() == 0 {
		t.Fatal("the pending pass did not start the shot worker")
	}
	if kinds := fx.countEventKinds(); kinds[computerevent.EventMaterializationStarted] != 0 {
		t.Fatalf("materialization started without a passing shot: %v", kinds)
	}
	if fx.applyCalls.Load() != 0 {
		t.Fatal("the updater was called without a passing candidate_shot")
	}
	if fx.rt.SelfDevelopmentApplyMaterializing(context.Background()) {
		t.Fatal("desk work held while the shots run")
	}
	// The worker finishes: the next pass swaps.
	write.Store(true)
	fx.rt.reconcileSelfDevelopmentMaterialization(context.Background())
	fx.waitForState(t, operation.OperationID, selfdev.StateApplied)
	if host.submits.Load() != 1 {
		t.Fatalf("host builds = %d, want 1 (the shot gate does not rebuild)", host.submits.Load())
	}
	if kinds := fx.countEventKinds(); kinds[computerevent.EventCandidateShot] != 1 {
		t.Fatalf("candidate_shot events = %d (%v)", kinds[computerevent.EventCandidateShot], kinds)
	}
}

func TestCandidateShotFailVerdictClosesTheOperationFailed(t *testing.T) {
	fx, _ := newHostBuildFixture(t, "computer-shot-fail", "done")
	fx.configure(func(rt *Runtime) {
		rt.candidateShotStarter = func(operation selfdev.Operation, bundle transaction.CapsuleEffectBundle, built hostBuiltRelease) {
			testWriteCandidateShot(t, rt, operation, bundle, built, computerevent.CandidateShotFail, "05-after-home.png Q2 (the list is empty)")
		}
	})
	operation, bundleDigest, _ := fx.seedOperationToAwaitingApproval(t, "op-shot-fail")
	fx.appendDecision(t, operation, bundleDigest, "decision-shot-fail")
	failed := fx.waitForState(t, operation.OperationID, selfdev.StateFailed)
	if !strings.Contains(failed.TerminalError, "shot review: 05-after-home.png Q2") {
		t.Fatalf("terminal error = %q, want the shot review cause", failed.TerminalError)
	}
	if fx.applyCalls.Load() != 0 || fx.service.restarts != 0 {
		t.Fatalf("a failing shot swapped: applies=%d restarts=%d", fx.applyCalls.Load(), fx.service.restarts)
	}
}

func TestCandidateShotForeignHostBuildEventDoesNotUnlockTheSwap(t *testing.T) {
	fx, _ := newHostBuildFixture(t, "computer-shot-foreign", "done")
	fx.configure(func(rt *Runtime) {
		rt.candidateShotStarter = func(operation selfdev.Operation, bundle transaction.CapsuleEffectBundle, built hostBuiltRelease) {
			// A passing shot of some other host build (a prior rebase).
			other := built
			other.Job.JobID = "job-of-another-build"
			testWriteCandidateShot(t, rt, operation, bundle, other, computerevent.CandidateShotPass, "")
		}
	})
	operation, bundleDigest, _ := fx.seedOperationToAwaitingApproval(t, "op-shot-foreign")
	fx.appendDecision(t, operation, bundleDigest, "decision-shot-foreign")
	accepted := fx.waitForState(t, operation.OperationID, selfdev.StateAccepted)
	if err := fx.rt.materializeSelfDevelopmentOperation(context.Background(), accepted, false); !errors.Is(err, errCandidateShotPending) {
		t.Fatalf("a pass with only a foreign candidate_shot = %v, want pending", err)
	}
	if fx.applyCalls.Load() != 0 {
		t.Fatal("a shot of another host build unlocked this swap")
	}
}

func TestCandidateShotPlatformBundleWithNeitherScreensNorPlanFailsByName(t *testing.T) {
	fx, _ := newHostBuildFixture(t, "computer-shot-noplan", "done")
	fx.noShotPlan = true
	var started atomic.Int32
	fx.configure(func(rt *Runtime) {
		rt.candidateShotStarter = func(selfdev.Operation, transaction.CapsuleEffectBundle, hostBuiltRelease) { started.Add(1) }
	})
	operation, bundleDigest, _ := fx.seedOperationToAwaitingApproval(t, "op-shot-noplan")
	fx.appendDecision(t, operation, bundleDigest, "decision-shot-noplan")
	failed := fx.waitForState(t, operation.OperationID, selfdev.StateFailed)
	if !strings.Contains(failed.TerminalError, "shot review: this platform candidate changes") || !strings.Contains(failed.TerminalError, "neither screens nor a shot plan") {
		t.Fatalf("terminal error = %q", failed.TerminalError)
	}
	if fx.applyCalls.Load() != 0 || started.Load() != 0 {
		t.Fatalf("an unshot candidate reached the updater (%d) or a worker (%d)", fx.applyCalls.Load(), started.Load())
	}
}

// The gate is not run for a journal-only settlement: it calls no updater.
func TestCandidateShotGateSkipsJournalOnlySettle(t *testing.T) {
	fx, _ := newHostBuildFixture(t, "computer-shot-journal", "done")
	var started atomic.Int32
	fx.configure(func(rt *Runtime) {
		rt.candidateShotStarter = func(selfdev.Operation, transaction.CapsuleEffectBundle, hostBuiltRelease) { started.Add(1) }
	})
	operation, bundleDigest, _ := fx.seedOperationToAwaitingApproval(t, "op-shot-journal")
	fx.appendDecision(t, operation, bundleDigest, "decision-shot-journal")
	accepted := fx.waitForState(t, operation.OperationID, selfdev.StateAccepted)
	// configure waits out every observer-driven drain, so the counts below
	// are this call's alone.
	fx.configure(func(rt *Runtime) {
		before := started.Load()
		err := rt.materializeSelfDevelopmentOperation(context.Background(), accepted, true)
		if errors.Is(err, errCandidateShotPending) {
			t.Errorf("a journal-only settle ran the candidate-shot gate: %v", err)
		}
		if started.Load() != before {
			t.Error("a journal-only settle started a shot worker")
		}
	})
}

// Unit: the gate's decision table, without the fixture's apply machinery.
func TestCandidateShotGateExemptAndPackageOperationsAreNotGated(t *testing.T) {
	rt := &Runtime{}
	docsOnly := transaction.CapsuleEffectBundle{OrderedFileEffects: []transaction.ChangeRecord{{Path: "workspace/platform/docs/a.md", Kind: "modified", Mode: 0o644}}}
	if err := rt.candidateShotGate(context.Background(), selfdev.Operation{}, docsOnly, nil); err != nil {
		t.Fatalf("an all-exempt change was gated: %v", err)
	}
	goChange := transaction.CapsuleEffectBundle{OrderedFileEffects: []transaction.ChangeRecord{{Path: "workspace/platform/internal/x/x.go", Kind: "modified", Mode: 0o644}}}
	if err := rt.candidateShotGate(context.Background(), selfdev.Operation{Kind: selfdev.KindPackage}, goChange, nil); err != nil {
		t.Fatalf("a package operation was gated: %v", err)
	}
	var failure candidateShotFailure
	if err := rt.candidateShotGate(context.Background(), selfdev.Operation{}, goChange, nil); !errors.As(err, &failure) {
		t.Fatalf("a Go change with no plan = %v, want a candidateShotFailure", err)
	}
	// Engineer-supplied screens (the pre-plan route) stay valid.
	screened := goChange
	screened.Screens = []capsule.ScreenFile{{Name: "01-after-home.png", SHA256: strings.Repeat("a", 64), Bytes: 1, Width: 1, Height: 1}}
	if err := rt.candidateShotGate(context.Background(), selfdev.Operation{}, screened, nil); err != nil {
		t.Fatalf("a bundle with its own screens was gated: %v", err)
	}
}

// FM1: a source-only bundle that is not host-built (no consensus decision,
// no Go input) fails by name and installs nothing.
func TestMaterializerNonConsensusSourceOnlyBundleFailsByName(t *testing.T) {
	fx := joinFixture(t, "sourceonly", joinTestBaseA, joinTestBaseA)
	fx.sourceOnly = true
	operation := fx.decide(t, "op-source-only")
	failed := fx.waitForState(t, operation.OperationID, selfdev.StateFailed)
	if !strings.Contains(failed.TerminalError, "source-only bundle") {
		t.Fatalf("terminal error = %q, want the source-only named failure", failed.TerminalError)
	}
	fx.requireRefusalSettled(t)
}

// The Complete guard (tools_capsule.go completeWithoutFreezeError).
// Failure modes: a completed round with a platform diff and no frozen
// candidate ends silently and the change vanishes (apply-proof-2); the
// guard also fires on a round whose diff is only artifacts (FM10); a Freeze
// that was refused and left the operation executing is read as frozen.
func TestCompleteGuardRefusesAPlatformDiffWithNoLiveFrozenOperation(t *testing.T) {
	f := newStagingReplay(t)
	f.toolCtx.CapsuleHandle = "handle-r9"
	platform := []capsule.FileChange{{Path: "workspace/platform/internal/x/x.go", Kind: capsule.ChangeModified, Mode: 0o644}}
	f.tree.live = capsule.GrantedWorktree{Changes: platform}
	ctx := context.Background()

	// The assignment froze: Complete may end.
	if err := completeWithoutFreezeError(ctx, f.toolCtx, "assignment-r9"); err != nil {
		t.Fatalf("a frozen assignment was refused: %v", err)
	}
	// No operation at all (apply-proof-2).
	err := completeWithoutFreezeError(ctx, f.toolCtx, "assignment-never-froze")
	if err == nil || !strings.Contains(err.Error(), "call choir.Freeze first; Complete does not freeze") {
		t.Fatalf("a platform diff with no operation = %v", err)
	}
	// A refused Freeze left an executing operation with no bundle.
	_, refusedOps := replayOperations(t, "computer-refused")
	refusedCtx := *f.toolCtx
	refusedCtx.OperationStore, refusedCtx.ComputerID = refusedOps, "computer-refused"
	op, startErr := refusedOps.Start(ctx, selfdev.StartRequest{
		ComputerID: "computer-refused", IdempotencyKey: choirstore.EngineeringFreezeIdempotencyKey("assignment-refused"),
		PromptArtifactRef: "artifact:sha256:" + strings.Repeat("c", 64),
	})
	if startErr != nil {
		t.Fatal(startErr)
	}
	if _, trErr := refusedOps.Transition(ctx, "computer-refused", op.OperationID, selfdev.StateRequested, selfdev.StateExecuting, nil); trErr != nil {
		t.Fatal(trErr)
	}
	if err := completeWithoutFreezeError(ctx, &refusedCtx, "assignment-refused"); err == nil || !strings.Contains(err.Error(), "call choir.Freeze first") {
		t.Fatalf("a refused Freeze that left the capsule dirty = %v", err)
	}
	// FM10: a round whose diff is only artifacts or /tmp completes.
	f.tree.live = capsule.GrantedWorktree{Changes: []capsule.FileChange{
		{Path: "var/lib/artifact/shot-plan.json", Kind: capsule.ChangeAdded, Mode: 0o644}, {Path: "tmp/x", Kind: capsule.ChangeAdded, Mode: 0o644},
	}}
	if err := completeWithoutFreezeError(ctx, f.toolCtx, "assignment-never-froze"); err != nil {
		t.Fatalf("a no-platform-change round was refused: %v", err)
	}
	// A tree that cannot be read is not a clean one.
	f.tree.live, f.tree.err = capsule.GrantedWorktree{}, errors.New("capsule busy")
	if err := completeWithoutFreezeError(ctx, f.toolCtx, "assignment-r9"); err == nil {
		t.Fatal("an unreadable tree let Complete through")
	}
}

// A capsule that built a release tree and a patch still freezes and
// re-freezes: the source-only bundle skips the release-tree diff, and a
// bundle with a release tree keeps it (FM2).
func TestFrozenReleaseChangesSkipsTheTreeForASourceOnlyBundle(t *testing.T) {
	live := []capsule.FrozenReleaseFile{{Path: "bin/autoputer", SHA256: strings.Repeat("a", 64), Mode: 0o755}}
	sourceOnly := transaction.CapsuleEffectBundle{SourcePatchSHA256: strings.Repeat("7", 64), RuntimeFiles: []capsule.FrozenReleaseFile{}}
	if got := frozenReleaseChanges(sourceOnly, live); len(got) != 0 {
		t.Fatalf("a source-only bundle diffed the capsule's test release tree: %v", got)
	}
	withTree := transaction.CapsuleEffectBundle{SourcePatchSHA256: strings.Repeat("7", 64), RuntimeFiles: []capsule.FrozenReleaseFile{{Path: "bin/autoputer", SHA256: strings.Repeat("b", 64), Mode: 0o755}}}
	if got := frozenReleaseChanges(withTree, live); len(got) != 1 || got[0] != "modified release bin/autoputer" {
		t.Fatalf("a bundle with a release tree lost its re-Freeze diff: %v", got)
	}
}

// Verify: a plan bundle's per-shot review is deferred to the candidate
// shot, and the numbered shots the request names are checked against plan
// views with no model call (FM12).
func TestPlanShotsMissingChecksNumberedShotsAgainstPlanViews(t *testing.T) {
	plan := &capsule.ShotPlan{Views: []capsule.ShotView{
		{Name: "a", Path: "/", Viewport: "desktop"}, {Name: "b", Path: "/", Viewport: "desktop"}, {Name: "c", Path: "/", Viewport: "phone"},
	}}
	request := "Shots: 01 the home view; 02 the list; 03 on a phone; 04 the dark view; 05 settings; 06 about"
	wanted := requestedShots(request)
	if len(wanted) != 6 {
		t.Fatalf("request parsed to %v", wanted)
	}
	if got := planShotsMissing(plan, wanted); strings.Join(got, ", ") != "04, 05, 06" {
		t.Fatalf("missing = %v, want 04, 05, 06", got)
	}
	if got := planShotsMissing(plan, requestedShots("just make it blue")); len(got) != 0 {
		t.Fatalf("a request with no numbered list is checked: %v", got)
	}
	if got := planShotsMissing(nil, wanted); len(got) != 0 {
		t.Fatalf("no plan is not a plan miss: %v", got)
	}
	if shotReviewDeferred != "deferred_to_candidate_shot" {
		t.Fatalf("shotReviewDeferred = %q", shotReviewDeferred)
	}
}

// The strict decoder of a frozen bundle (decodeBoundBundle) reads a plan
// bundle: DisallowUnknownFields must know shot_plan, and the plan survives
// the round trip bound to the operation's digest.
func TestDecodeBoundBundleAcceptsAShotPlan(t *testing.T) {
	bundle := transaction.CapsuleEffectBundle{BundleVersion: 1, ComputerID: "c", RuntimeFiles: []capsule.FrozenReleaseFile{}, ShotPlan: defaultTestShotPlan()}
	raw, err := computerevent.CanonicalJSON(bundle)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeBoundBundle(raw, true, selfdev.Operation{BundleDigest: computerevent.DigestBytes(raw)})
	if err != nil {
		t.Fatalf("a plan bundle did not decode: %v", err)
	}
	if decoded.ShotPlan == nil || len(decoded.ShotPlan.Views) != 1 || decoded.ShotPlan.Views[0].Name != "home" {
		t.Fatalf("decoded plan = %+v", decoded.ShotPlan)
	}
}
