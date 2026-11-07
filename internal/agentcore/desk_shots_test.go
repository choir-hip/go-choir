package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Desk shots (desk_shots.go, docs/design/desk-shots-and-review-2026-11-07.md
// section 4). Failure modes, decided before the code, and the test that holds
// each:
//
//   - FM1 management never answers: TestShotsGateCommitsOnceNoticesThenFailsAtTheDeadline
//   - FM2 a verdict for another host build or plan: TestShotVerdictForAnotherPlanDoesNotUnlockTheSwap
//   - FM3, FM12 a pass without seeing the shots: TestShotVerdictRefusals
//   - FM4 management cannot see images: TestShotsGateFailsAtOnceWithNoImageCapableModel
//   - FM10, FM11 too many images / image weight: TestEstimateCountsImageBlocks, TestVerifyShotRefusals
//   - FM13 a restart-closed job: restarted, never a verdict: TestShotVerdictRefusals
//   - FM15 a package bundle with screens: TestVerifyShotRefusals

// shotsGateFixture drives an operation to accepted with the gate asking.
func shotsGateFixture(t *testing.T, computerID string) (*derivableSelfDevFixture, selfdev.Operation) {
	t.Helper()
	fx, _ := newHostBuildFixture(t, computerID, "done")
	fx.configure(func(rt *Runtime) { rt.shotsAskedHook = nil })
	operation, bundleDigest, _ := fx.seedOperationToAwaitingApproval(t, "op-"+computerID)
	fx.appendDecision(t, operation, bundleDigest, "decision-"+computerID)
	return fx, fx.waitForState(t, operation.OperationID, selfdev.StateAccepted)
}

func TestShotsGateCommitsOnceNoticesThenFailsAtTheDeadline(t *testing.T) {
	fx, accepted := shotsGateFixture(t, "computer-shots-clock")
	ctx := context.Background()
	base := time.Now()
	clock := base
	fx.configure(func(rt *Runtime) { rt.shotsNow = func() time.Time { return clock } })
	pass := func() error { return fx.rt.materializeSelfDevelopmentOperation(ctx, accepted, false) }
	for i := 0; i < 2; i++ { // the record is committed once; a repeat is a replay
		if err := pass(); !errors.Is(err, errCandidateShotPending) {
			t.Fatalf("pass %d = %v, want pending", i, err)
		}
	}
	owed, err := fx.rt.shotsOwedBy(ctx, fx.computerID)
	if err != nil || len(owed) != 1 {
		t.Fatalf("owed = %v, %v", owed, err)
	}
	hostJob := owed[0].HostJobID
	if n, _ := fx.rt.shotsOwedNotices(ctx, "owner-shots", fx.computerID, accepted.OperationID, hostJob); n != 0 {
		t.Fatalf("notices before the wait = %d", n)
	}
	// Past the notice interval a notice goes out; a fresh id each, capped.
	for i := 1; i <= types.ShotsOwedMaxNotices+1; i++ {
		clock = clock.Add(shotsOwedNoticeAfter + time.Second)
		fx.configure(func(rt *Runtime) { rt.shotsNow = func() time.Time { return clock } })
		_ = pass()
	}
	if n, _ := fx.rt.shotsOwedNotices(ctx, "owner-shots", fx.computerID, accepted.OperationID, hostJob); n > types.ShotsOwedMaxNotices {
		t.Fatalf("notices = %d, over the cap", n)
	}
	// The deadline closes the operation failed; nothing swapped.
	clock = base.Add(shotsOwedDeadline + time.Minute)
	fx.configure(func(rt *Runtime) { rt.shotsNow = func() time.Time { return clock } })
	_ = pass()
	failed := fx.waitForState(t, accepted.OperationID, selfdev.StateFailed)
	if !strings.Contains(failed.TerminalError, shotsNotJudgedInTime) {
		t.Fatalf("terminal error = %q", failed.TerminalError)
	}
	if fx.applyCalls.Load() != 0 {
		t.Fatal("the swap ran without a verdict")
	}
}

func TestShotsGateFailsAtOnceWithNoImageCapableModel(t *testing.T) {
	fx, accepted := shotsGateFixture(t, "computer-shots-noimg")
	fx.configure(func(rt *Runtime) { rt.modelPolicy = nil })
	err := fx.rt.materializeSelfDevelopmentOperation(context.Background(), accepted, false)
	_ = err
	failed := fx.waitForState(t, accepted.OperationID, selfdev.StateFailed)
	if !strings.Contains(failed.TerminalError, "nothing can judge the shots") {
		t.Fatalf("terminal error = %q", failed.TerminalError)
	}
	if owed, _ := fx.rt.shotsOwedBy(context.Background(), fx.computerID); len(owed) != 0 {
		t.Fatal("a record was committed for a judge that cannot look")
	}
}

func TestShotVerdictForAnotherPlanDoesNotUnlockTheSwap(t *testing.T) {
	fx, accepted := shotsGateFixture(t, "computer-shots-plan")
	fx.configure(func(rt *Runtime) {
		rt.shotsAskedHook = func(operation selfdev.Operation, bundle transactionBundle, built hostBuiltRelease) {
			payload := computerevent.ShotVerdictPayload{HostBuildJobID: built.Job.JobID, ShotJobID: "shot-1", PlanSHA256: strings.Repeat("e", 64), Verdict: computerevent.ShotVerdictPass,
				Findings: []computerevent.ShotFindingRecord{{Pair: "p", BeforeSHA256: strings.Repeat("a", 64), AfterSHA256: strings.Repeat("b", 64)}}}
			if err := rt.appendShotVerdict(context.Background(), operation, payload); err != nil {
				t.Error(err)
			}
		}
	})
	_ = fx.rt.materializeSelfDevelopmentOperation(context.Background(), accepted, false)
	_ = fx.rt.materializeSelfDevelopmentOperation(context.Background(), accepted, false)
	failed := fx.waitForState(t, accepted.OperationID, selfdev.StateFailed)
	if !strings.Contains(failed.TerminalError, "not bound to this operation, host build and plan") || fx.applyCalls.Load() != 0 {
		t.Fatalf("terminal error = %q, applies = %d", failed.TerminalError, fx.applyCalls.Load())
	}
}

func TestShotVerdictRefusals(t *testing.T) {
	fx, accepted := shotsGateFixture(t, "computer-shots-verdict")
	fx.configure(func(rt *Runtime) { rt.cfg.StorePath = filepath.Join(t.TempDir(), "runtime.db") })
	ctx := context.Background()
	if err := fx.rt.materializeSelfDevelopmentOperation(ctx, accepted, false); !errors.Is(err, errCandidateShotPending) {
		t.Fatalf("gate = %v", err)
	}
	info, refusal, err := fx.rt.shotsOwedFor(ctx, fx.computerID, accepted.OperationID)
	if err != nil || refusal != "" {
		t.Fatalf("owed: %q %v", refusal, err)
	}
	run := types.RunRecord{RunID: "run-mgmt", OwnerID: "owner-shots", ComputerID: fx.computerID, AgentID: "management:owner-shots", AgentProfile: agentprofile.Management}
	reduce := &rlmCallReduction{active: true, mb: fx.rt, st: fx.rt.store, ledger: fx.rt.store, rec: &run, scope: ReductionScope{
		RunID: run.RunID, OwnerID: run.OwnerID, ComputerID: run.ComputerID, DeskAgentID: run.AgentID,
	}}
	sha := func(c string) string { return strings.Repeat(c, 64) }
	job := shotJobRecord{
		JobID: shotJobID(shotJobKey(accepted.OperationID, info.HostJobID), 1), Gen: 1, OperationID: accepted.OperationID, HostBuildJobID: info.HostJobID,
		BundleDigest: accepted.BundleDigest, PlanSHA256: info.PlanDigest,
		shotJobResult: shotJobResult{State: shotJobDone, Screens: []shotJobScreen{{Name: "01-before-home.png", SHA256: sha("a"), Width: 1440, Height: 900}, {Name: "01-after-home.png", SHA256: sha("b"), Width: 1440, Height: 900}}},
	}
	intent := func(verdict, reason string, findings []yaegikernel.ShotFinding) yaegikernel.StagedIntent {
		return yaegikernel.StagedIntent{LocalID: "v1", Kind: yaegikernel.IntentShotVerdict, TargetRef: accepted.OperationID, Decision: verdict, Reason: reason, ShotFindings: findings}
	}
	yes := map[string]string{"q1": "yes", "q2": "yes", "q3": "yes", "q4": "n/a", "q6": "yes"}
	full := []yaegikernel.ShotFinding{{Pair: "01-home", BeforeSHA256: sha("a"), AfterSHA256: sha("b"), Answers: yes}}
	expect := func(name string, in yaegikernel.StagedIntent, want string) {
		t.Helper()
		_, err := reduce.commitShotVerdictIntent(ctx, in)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("%s = %v, want %q", name, err, want)
		}
	}
	expect("no job", intent("pass", "", full), "call choir.ShotStart first")
	if err := fx.rt.writeShotJobFile(job); err != nil {
		t.Fatal(err)
	}
	expect("a fail without a reason", intent("fail", "", nil), "needs its reason")
	expect("not seen", intent("pass", "", full), "run's memory does not hold the images")
	expect("a missing pair", intent("pass", "", []yaegikernel.ShotFinding{{Pair: "p", BeforeSHA256: sha("a"), AfterSHA256: sha("a"), Answers: yes}}), "do not cover every screen")
	expect("a foreign shot", intent("pass", "", []yaegikernel.ShotFinding{{Pair: "p", BeforeSHA256: sha("a"), AfterSHA256: sha("c"), Answers: yes}}), "not in job")
	no := map[string]string{"q1": "yes", "q2": "no", "q3": "yes", "q4": "n/a", "q6": "yes"}
	expect("a pass with a no", intent("pass", "", []yaegikernel.ShotFinding{{Pair: "p", BeforeSHA256: sha("a"), AfterSHA256: sha("b"), Answers: no}}), "answers q2 no")
	// The run's memory holds the injected turn: the verdict is recorded once.
	msg := shotImagesMessage(job)
	if _, err := fx.rt.store.AppendRunMemoryEntry(ctx, types.RunMemoryEntry{RunID: run.RunID, OwnerID: run.OwnerID, Kind: types.RunMemoryEntryMessage, Role: "user", Message: msg}); err != nil {
		t.Fatal(err)
	}
	out, err := reduce.commitShotVerdictIntent(ctx, intent("pass", "", full))
	if err != nil || out["verdict"] != "pass" {
		t.Fatalf("verdict = %v, %v", out, err)
	}
	payload, ok := fx.rt.shotVerdictFor(ctx, accepted, info.Bundle, info.HostJobID)
	if !ok || payload.Judge.RunID != run.RunID || payload.ShotJobID != job.JobID || len(payload.Findings) != 1 {
		t.Fatalf("event = %+v, %v", payload, ok)
	}
	// A restarted job takes no verdict.
	if _, refusal, _ := fx.rt.shotsOwedFor(ctx, fx.computerID, accepted.OperationID); refusal == "" {
		t.Fatal("an operation with a verdict still owes shots")
	}
}

func TestShotsOwedGuardAsksOverTheVerdict(t *testing.T) {
	fx, accepted := shotsGateFixture(t, "computer-shots-guard")
	ctx := context.Background()
	_ = fx.rt.materializeSelfDevelopmentOperation(ctx, accepted, false)
	rec := &types.RunRecord{RunID: "run-g", OwnerID: "owner-shots", ComputerID: fx.computerID, AgentID: persistentManagementAgentID("owner-shots"), AgentProfile: agentprofile.Management}
	res, err := fx.rt.managementVerifyOwedGuard(rec)(ctx, completionStateForTest(0))
	if err != nil || !res.Continue || !strings.Contains(res.Instruction, "Shots owed on "+accepted.OperationID) {
		t.Fatalf("guard = %+v, %v", res, err)
	}
	views := fx.rt.shotsOwedWaitViews(ctx, "owner-shots", fx.computerID)
	if len(views) != 1 || views[0].Wait.Kind != types.WaitKindShotsOwed {
		t.Fatalf("wait views = %+v", views)
	}
}

func TestEstimateCountsImageBlocks(t *testing.T) {
	text, _ := json.Marshal(map[string]any{"role": "user", "content": []map[string]any{{"type": "text", "text": "x"}}})
	withImage, _ := json.Marshal(map[string]any{"role": "user", "content": []map[string]any{{"type": "text", "text": "x"}, {"type": "image", "source": map[string]string{"kind": "artifact_ref", "ref": "artifact:sha256:aa"}}}})
	if got := estimateRawMessageTokens(withImage) - estimateRawMessageTokens(text); got < imageBlockTokens {
		t.Fatalf("an image block adds %d tokens, want at least %d", got, imageBlockTokens)
	}
}

func TestVerifyShotRefusals(t *testing.T) {
	views := func(n int) *capsuleShotPlan {
		p := &capsuleShotPlan{}
		for i := 0; i < n; i++ {
			p.Views = append(p.Views, capsuleShotView{Name: "v" + string(rune('a'+i)), Path: "/", Viewport: "desktop"})
		}
		return p
	}
	platform, pkg := selfdev.Operation{}, selfdev.Operation{Kind: selfdev.KindPackage}
	screens := []capsuleScreenFile{{Name: "01-after-home.png", SHA256: strings.Repeat("a", 64), Bytes: 1, Width: 1, Height: 1}}
	if r := verifyShotRefusal(platform, bundleOf(views(shotMaxViews+1), nil)); !strings.Contains(r, "at most") {
		t.Fatalf("11 views = %q", r)
	}
	if r := verifyShotRefusal(platform, bundleOf(views(shotMaxViews), nil)); r != "" {
		t.Fatalf("10 views = %q", r)
	}
	if r := verifyShotRefusal(platform, bundleOf(nil, screens)); !strings.Contains(r, "legacy screens bundle") {
		t.Fatalf("platform screens = %q", r)
	}
	if r := verifyShotRefusal(pkg, bundleOf(nil, screens)); r != "" {
		t.Fatalf("a package bundle with screens was refused: %q", r)
	}
}

type (
	transactionBundle = transaction.CapsuleEffectBundle
	capsuleShotPlan   = capsule.ShotPlan
	capsuleShotView   = capsule.ShotView
	capsuleScreenFile = capsule.ScreenFile
)

func bundleOf(plan *capsule.ShotPlan, screens []capsule.ScreenFile) transaction.CapsuleEffectBundle {
	return transaction.CapsuleEffectBundle{ShotPlan: plan, Screens: screens}
}

func completionStateForTest(attempts int) toolregistry.ToolLoopCompletionState {
	return toolregistry.ToolLoopCompletionState{Attempts: attempts}
}
