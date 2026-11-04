package agentcore

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Freeze ends the build budget
// (docs/problems/frozen-candidate-dies-with-build-deadline-2026-11-04.md):
// luna's assignment froze a candidate at minute 48 and the 60-minute
// activation budget then failed it before any decision began.
//
// Ways this could fail, written first:
//  1. The build's budget ending after a Freeze still fails the frozen
//     operation (the incident).
//  2. The retention leaks into other cancellations: an owner or management
//     cancel, a trajectory cancel or a crash boot keeps a candidate frozen.
//  3. A candidate that never froze (executing) is kept instead of closed.
//  4. The cancellation's Report does not name the retained candidate and
//     its bundle, so management cannot Verify it.
//  5. management's Verify still refuses the retained candidate because its
//     assignment is cancelled; or it admits a cancelled assignment whose
//     candidate was failed.
//  6. The decision wait is not a typed wait with its own bound, or running
//     out of that bound fails the candidate instead of keeping it.
//  7. A crash restart does not close a retained candidate (the restart
//     rule), or closes a completed round's candidate that survives
//     restarts today.

func retainedFixture(t *testing.T, owner string) (*Runtime, *store.Store, types.EngineeringAssignment, types.RunRecord) {
	t.Helper()
	rt, s, _ := c1eRuntime(t)
	operations, err := selfdev.NewStore(s, s)
	if err != nil {
		t.Fatal(err)
	}
	rt.selfdevOperations = operations
	rt.selfdevUpdaterRoot = t.TempDir()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, owner, rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignment, run := c1eOpenBound(t, s, seed, 0)
	return rt, s, assignment, run
}

func operationState(t *testing.T, rt *Runtime, computerID, operationID string) selfdev.Operation {
	t.Helper()
	operation, err := rt.selfdevOperations.Get(context.Background(), computerID, operationID)
	if err != nil {
		t.Fatal(err)
	}
	return operation
}

func TestBuildBudgetEndAfterFreezeRetainsTheCandidate(t *testing.T) {
	for name, reason := range map[string]string{
		"activation budget":   activationBudgetExceededReason,
		"assignment deadline": assignmentDeadlineExpiredPrefix + "6h0m0s from bind; the assignment is cancelled",
	} {
		t.Run(name, func(t *testing.T) {
			ctx := withBuildBudgetEnd(context.Background())
			rt, s, assignment, run := retainedFixture(t, "owner-retain")
			operationID := seedAssignmentOperation(t, rt, assignment, selfdev.StateFrozen, "")
			if _, err := rt.cancelBoundEngineeringRun(ctx, run, reason); err != nil {
				t.Fatalf("budget cancel: %v", err)
			}
			got := c1eGet(t, s, assignment)
			if got.Disposition != types.EngineeringAssignmentCancelled {
				t.Fatalf("assignment after its budget = %s, want cancelled", got.Disposition)
			}
			operation := operationState(t, rt, assignment.Binding.ComputerID, operationID)
			if operation.State != selfdev.StateFrozen {
				t.Fatalf("frozen candidate after its build's budget = %s %q, want frozen (item 1)", operation.State, operation.TerminalError)
			}
			for _, want := range []string{operationID, operation.BundleDigest, "retained for Verify", "choir.Verify"} {
				if !strings.Contains(got.DispositionReason, want) {
					t.Fatalf("cancellation reason %q does not name %q (item 4)", got.DispositionReason, want)
				}
			}

			views := rt.candidateDecisionWaitViews(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID)
			if len(views) != 1 || views[0].AssignmentID != assignment.AssignmentID || views[0].Wait == nil {
				t.Fatalf("management's decision waits = %+v, want the retained candidate (item 6)", views)
			}
			w := views[0].Wait
			if w.Kind != types.WaitKindCandidateDecision || w.Class != types.WaitClassPolicy || w.Until.Sub(w.Since) != candidateDecisionBudget || w.Sentence == "" {
				t.Fatalf("decision wait = %+v, want a typed candidate_decision wait bounded by its own budget (item 6)", w)
			}
		})
	}
}

func TestOtherCancellationsStillCloseTheCandidate(t *testing.T) {
	for name, reason := range map[string]string{
		"management cancel":    "persistent Management cancelled assignment",
		"crash boot":           engineeringInterruptedByRestart,
		"raw context error":    "context deadline exceeded",
		"spoofed exact reason": activationBudgetExceededReason,
		"spoofed deadline":     assignmentDeadlineExpiredPrefix + "6h0m0s from bind",
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			rt, _, assignment, run := retainedFixture(t, "owner-close")
			operationID := seedAssignmentOperation(t, rt, assignment, selfdev.StateFrozen, "")
			if _, err := rt.cancelBoundEngineeringRun(ctx, run, reason); err != nil {
				t.Fatalf("cancel: %v", err)
			}
			if operation := operationState(t, rt, assignment.Binding.ComputerID, operationID); operation.State != selfdev.StateFailed {
				t.Fatalf("candidate after %s = %s, want failed (item 2)", name, operation.State)
			}
		})
	}
}

func TestBuildBudgetEndBeforeFreezeClosesTheOperation(t *testing.T) {
	ctx := context.Background()
	rt, _, assignment, run := retainedFixture(t, "owner-unfrozen")
	operationID := seedAssignmentOperation(t, rt, assignment, selfdev.StateExecuting, "")
	if _, err := rt.cancelBoundEngineeringRun(withBuildBudgetEnd(ctx), run, activationBudgetExceededReason); err != nil {
		t.Fatal(err)
	}
	if operation := operationState(t, rt, assignment.Binding.ComputerID, operationID); operation.State != selfdev.StateFailed {
		t.Fatalf("an unfrozen operation after the budget = %s, want failed (item 3)", operation.State)
	}
}

func verifyReduction(rt *Runtime, assignment types.EngineeringAssignment) *rlmCallReduction {
	run := types.RunRecord{RunID: "run-verify", OwnerID: assignment.Binding.OwnerID, ComputerID: assignment.Binding.ComputerID, AgentProfile: agentprofile.Management}
	return &rlmCallReduction{active: true, mb: rt, st: rt.store, ledger: rt.store, rec: &run, scope: ReductionScope{
		RunID: run.RunID, OwnerID: run.OwnerID, ComputerID: run.ComputerID, DeskAgentID: assignment.Binding.ParentAgentID,
	}}
}

func TestVerifyAdmitsOnlyARetainedCandidateOfACancelledRound(t *testing.T) {
	ctx := context.Background()
	rt, _, assignment, run := retainedFixture(t, "owner-verify")
	operationID := seedAssignmentOperation(t, rt, assignment, selfdev.StateFrozen, "")
	if _, err := rt.cancelBoundEngineeringRun(withBuildBudgetEnd(ctx), run, activationBudgetExceededReason); err != nil {
		t.Fatal(err)
	}
	// A wrong bundle stops the verify right after the admission gate, so the
	// error says which gate refused it.
	intent := yaegikernel.StagedIntent{LocalID: "v1", TargetRef: assignment.AssignmentID, Decision: "pass", BundleDigest: strings.Repeat("0", 64)}
	_, err := verifyReduction(rt, assignment).commitVerifyIntent(ctx, intent)
	if err == nil || !strings.Contains(err.Error(), "bundle digest does not match") {
		t.Fatalf("verify of the retained candidate = %v, want it past the admission gate (item 5)", err)
	}
	// Without its record (a cancel whose close did not land), it is refused.
	if err := os.Remove(retainedPath(rt.selfdevUpdaterRoot, operationID)); err != nil {
		t.Fatal(err)
	}
	_, err = verifyReduction(rt, assignment).commitVerifyIntent(ctx, intent)
	if err == nil || !strings.Contains(err.Error(), "was not retained") {
		t.Fatalf("verify of a cancelled round's unretained frozen candidate = %v, want refused at the gate (item 5)", err)
	}
}

// review r1 (both seats): a cancel whose best-effort close did not land
// leaves the candidate frozen; it must not become verifiable.
func TestVerifyRefusesAFrozenCandidateOfAnOrdinaryCancel(t *testing.T) {
	ctx := context.Background()
	rt, _, assignment, run := retainedFixture(t, "owner-verify-cancel")
	seedAssignmentOperation(t, rt, assignment, selfdev.StateFrozen, "")
	rt.selfdevOperations = nil // the close cannot reach the operation
	if _, err := rt.cancelBoundEngineeringRun(ctx, run, "persistent Management cancelled assignment"); err != nil {
		t.Fatal(err)
	}
	operations, err := selfdev.NewStore(rt.store, rt.store)
	if err != nil {
		t.Fatal(err)
	}
	rt.selfdevOperations = operations
	intent := yaegikernel.StagedIntent{LocalID: "v1", TargetRef: assignment.AssignmentID, Decision: "pass", BundleDigest: strings.Repeat("f", 64)}
	if _, err := verifyReduction(rt, assignment).commitVerifyIntent(ctx, intent); err == nil || !strings.Contains(err.Error(), "was not retained") {
		t.Fatalf("verify after an ordinary cancel left the candidate frozen = %v, want refused", err)
	}
}

func TestDecisionBudgetRunsOutAndTheCandidateStaysRetained(t *testing.T) {
	retainedAt := time.Date(2026, 11, 4, 3, 34, 58, 0, time.UTC)
	rec := retainedCandidate{OperationID: "selfdev-x", AssignmentID: "assignment-x", BundleDigest: strings.Repeat("f", 64), RetainedAt: retainedAt}

	before := candidateDecisionWait(rec, retainedAt.Add(time.Hour))
	if !before.Since.Equal(retainedAt) || !before.Until.Equal(retainedAt.Add(candidateDecisionBudget)) || strings.Contains(before.Detail, "ran out") {
		t.Fatalf("decision wait inside its budget = %+v", before)
	}
	late := retainedAt.Add(candidateDecisionBudget + time.Minute)
	after := candidateDecisionWait(rec, late)
	if !strings.Contains(after.Detail, "ran out") || !strings.Contains(after.Detail, "stays frozen and retained") || !strings.Contains(after.SentenceAt(late), "at its bound") {
		t.Fatalf("decision wait past its budget = %q / %q, want it at its bound with the candidate retained (item 6)", after.Detail, after.SentenceAt(late))
	}
}

func TestCrashBootClosesARetainedCandidateOnly(t *testing.T) {
	ctx := context.Background()
	rt, _, assignment, run := retainedFixture(t, "owner-crash")
	retainedID := seedAssignmentOperation(t, rt, assignment, selfdev.StateFrozen, "")
	if _, err := rt.cancelBoundEngineeringRun(withBuildBudgetEnd(ctx), run, activationBudgetExceededReason); err != nil {
		t.Fatal(err)
	}
	// A completed round's frozen candidate on the same computer has no record.
	completed := endedAssignment(assignment.Binding.ComputerID, "assignment-completed-elsewhere", types.EngineeringAssignmentCompleted)
	completedID := seedAssignmentOperation(t, rt, completed, selfdev.StateFrozen, "")

	rt.closeRetainedCandidatesAtCrashBoot(ctx)

	retained := operationState(t, rt, assignment.Binding.ComputerID, retainedID)
	if retained.State != selfdev.StateFailed || !strings.Contains(retained.TerminalError, engineeringInterruptedByRestart) {
		t.Fatalf("retained candidate after a crash boot = %s %q, want failed, interrupted by a restart (item 7)", retained.State, retained.TerminalError)
	}
	if _, err := os.Stat(retainedPath(rt.selfdevUpdaterRoot, retainedID)); !os.IsNotExist(err) {
		t.Fatalf("the closed candidate's record is still there: %v", err)
	}
	if other := operationState(t, rt, assignment.Binding.ComputerID, completedID); other.State != selfdev.StateFrozen {
		t.Fatalf("a completed round's candidate after a crash boot = %s, want frozen (item 7)", other.State)
	}
}

func TestBuildBudgetEndIsAContextMarkNotAReason(t *testing.T) {
	if buildBudgetEnded(context.Background()) {
		t.Fatal("an unmarked context reads as the build's end")
	}
	if !buildBudgetEnded(context.WithoutCancel(withBuildBudgetEnd(context.Background()))) {
		t.Fatal("the mark does not survive WithoutCancel, which terminalizeRun applies")
	}
}

// The incident's own path: the activation context's deadline ends the
// engineer's loop with "context deadline exceeded", and handleExecutionError
// names it as the build's budget, so the frozen candidate is retained.
func TestActivationContextDeadlineAfterFreezeRetainsTheCandidate(t *testing.T) {
	rt, s, assignment, run := retainedFixture(t, "owner-ctx-deadline")
	operationID := seedAssignmentOperation(t, rt, assignment, selfdev.StateFrozen, "")
	if !assignedEngineeringRun(&run) {
		t.Fatalf("fixture run is not an assigned engineering run: %+v", run.Metadata)
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	rt.handleExecutionError(ctx, &run, context.DeadlineExceeded)
	if got := c1eGet(t, s, assignment); got.Disposition != types.EngineeringAssignmentCancelled || !strings.HasPrefix(got.DispositionReason, activationBudgetExceededReason) {
		t.Fatalf("assignment after the activation deadline = %s %q", got.Disposition, got.DispositionReason)
	}
	if operation := operationState(t, rt, assignment.Binding.ComputerID, operationID); operation.State != selfdev.StateFrozen {
		t.Fatalf("frozen candidate after the activation deadline = %s %q, want frozen (item 1)", operation.State, operation.TerminalError)
	}
}

// review r1 (deepseek F4): a retained candidate's verify_pass precommit is
// left open for its Verify, not settled no at the cancel.
func TestRetainedCandidateLeavesVerifyPassOpen(t *testing.T) {
	rule := types.PrecommitRule{Kind: types.PrecommitRuleVerifyPass, AssignmentID: "assignment-x"}
	obs := types.EngineeringTerminalObservation{Disposition: types.EngineeringAssignmentCancelled, CandidateFrozen: true, CandidateRetained: true, At: time.Now()}
	if settled, ok := types.SettleRuleAtTerminal(rule, obs); ok {
		t.Fatalf("verify_pass of a retained candidate settled at the cancel: %+v", settled)
	}
	obs.CandidateRetained = false
	if settled, ok := types.SettleRuleAtTerminal(rule, obs); !ok || settled.Answer != types.PrecommitAnswerNo {
		t.Fatalf("verify_pass of an ordinary cancel = %+v %v, want no", settled, ok)
	}
}

// review r2 (devin): a build-end cancel that arrives after another actor's
// cancel committed (a stale snapshot) must not leave a retention record
// that makes that ordinary cancel's candidate verifiable.
func TestLateBuildEndCancelLeavesNoRetention(t *testing.T) {
	ctx := context.Background()
	rt, s, assignment, run := retainedFixture(t, "owner-late-budget")
	operationID := seedAssignmentOperation(t, rt, assignment, selfdev.StateFrozen, "")
	stale := c1eGet(t, s, assignment)
	keep := rt.selfdevOperations
	rt.selfdevOperations = nil // management's close does not land
	if _, err := rt.cancelBoundEngineeringRun(ctx, run, "persistent Management cancelled assignment"); err != nil {
		t.Fatal(err)
	}
	rt.selfdevOperations = keep
	if _, err := rt.persistSystemEngineeringCancellation(withBuildBudgetEnd(ctx), stale, activationBudgetExceededReason, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(retainedPath(rt.selfdevUpdaterRoot, operationID)); !os.IsNotExist(err) {
		t.Fatalf("a late build-end cancel left a retention record: %v", err)
	}
}

// review r3: a frozen candidate of a cancelled round with no retention
// record (its close did not land, or its record was never written) can
// never be verified and holds engineering admission; a crash boot closes it.
func TestCrashBootClosesAnOrphanedFrozenCandidate(t *testing.T) {
	ctx := context.Background()
	rt, _, assignment, run := retainedFixture(t, "owner-orphan")
	operationID := seedAssignmentOperation(t, rt, assignment, selfdev.StateFrozen, "")
	keep := rt.selfdevOperations
	rt.selfdevOperations = nil // the cancel's close does not land
	if _, err := rt.cancelBoundEngineeringRun(ctx, run, "persistent Management cancelled assignment"); err != nil {
		t.Fatal(err)
	}
	rt.selfdevOperations = keep
	rt.closeRetainedCandidatesAtCrashBoot(ctx)
	if operation := operationState(t, rt, assignment.Binding.ComputerID, operationID); operation.State != selfdev.StateFailed {
		t.Fatalf("orphaned frozen candidate after a crash boot = %s, want failed", operation.State)
	}
}

// coordinator, 2026-11-04: after a Freeze, no end of the engineer's loop (a
// run cancel, a failed loop, a missing Complete) fails the candidate. Every
// run end goes through terminalizeRun.
func TestAnyEngineerRunEndAfterFreezeRetainsTheCandidate(t *testing.T) {
	for _, reason := range []string{"run cancelled", "tool loop: completion guard exhausted", runFateNoLiveLoop, "cell terminal deadline exceeded"} {
		t.Run(reason, func(t *testing.T) {
			rt, s, assignment, run := retainedFixture(t, "owner-run-end")
			operationID := seedAssignmentOperation(t, rt, assignment, selfdev.StateFrozen, "")
			if err := rt.terminalizeRun(context.Background(), run.RunID, run.OwnerID, reason); err != nil {
				t.Fatalf("terminalize: %v", err)
			}
			if got := c1eGet(t, s, assignment); got.Disposition != types.EngineeringAssignmentCancelled {
				t.Fatalf("assignment after the run ended = %s", got.Disposition)
			}
			if operation := operationState(t, rt, assignment.Binding.ComputerID, operationID); operation.State != selfdev.StateFrozen {
				t.Fatalf("frozen candidate after the run ended (%s) = %s %q, want frozen", reason, operation.State, operation.TerminalError)
			}
			if _, ok := rt.retainedFor(operationState(t, rt, assignment.Binding.ComputerID, operationID), assignment.AssignmentID); !ok {
				t.Fatal("no retention record: management could not verify it")
			}
		})
	}
}

// Assignment-level acts still decide the candidate: management's cancel.
func TestManagementCancelStillClosesTheCandidate(t *testing.T) {
	rt, _, assignment, _ := retainedFixture(t, "owner-mgmt-cancel")
	operationID := seedAssignmentOperation(t, rt, assignment, selfdev.StateFrozen, "")
	parent := types.RunRecord{OwnerID: assignment.Binding.OwnerID, ComputerID: assignment.Binding.ComputerID, AgentID: assignment.Binding.ParentAgentID}
	if _, err := rt.cancelAssignedEngineering(context.Background(), parent, assignment.AssignmentID, assignment.Binding.Attempt, "management: wrong approach"); err != nil {
		t.Fatalf("management cancel: %v", err)
	}
	if operation := operationState(t, rt, assignment.Binding.ComputerID, operationID); operation.State != selfdev.StateFailed {
		t.Fatalf("candidate after management's cancel = %s, want failed", operation.State)
	}
}

// review r5 (devin F1): a run that persisted completed without a Complete
// reaches the run-end settle, not terminalizeRun; it retains too.
func TestRunEndSettleAfterFreezeRetainsTheCandidate(t *testing.T) {
	rt, s, assignment, run := retainedFixture(t, "owner-run-end-settle")
	operationID := seedAssignmentOperation(t, rt, assignment, selfdev.StateFrozen, "")
	run.State = types.RunCompleted
	if !rt.settleAssignmentOfEndedEngineeringRun(context.Background(), &run) {
		t.Fatal("the run-end settle did not match the assignment")
	}
	if got := c1eGet(t, s, assignment); got.Disposition != types.EngineeringAssignmentCancelled {
		t.Fatalf("assignment after the run-end settle = %s", got.Disposition)
	}
	if operation := operationState(t, rt, assignment.Binding.ComputerID, operationID); operation.State != selfdev.StateFrozen {
		t.Fatalf("frozen candidate after a run ended without Complete = %s %q, want frozen", operation.State, operation.TerminalError)
	}
}
