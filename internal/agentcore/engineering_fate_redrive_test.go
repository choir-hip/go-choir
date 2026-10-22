package agentcore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Review round 4c (DeepSeek 1 / Sonnet E1, High): the fate wake's deferral
// to the live saga returned nil, so the wake was consumed; the watchdog arm
// reused the consumed wake's identity and deduped into nothing; and several
// failure exits of the live saga armed nothing. Ways the fix can fail,
// written first: the deferral is still consumed; a failure exit still arms
// nothing; the arm still goes through the plain schedule hook (deduping into
// a consumed wake); the worker no longer holds the saga; the worker's lock
// leaks; a freeze committed before a failed intent is lost from commit's
// error; the saga map grows without bound.

type fateWake struct {
	hook, target, kind, content string
}

func captureFateWakes(rt *Runtime) *[]fateWake {
	var wakes []fateWake
	rt.kernelMode = true
	rt.scheduleActor = func(_ context.Context, _, _, target, kind, content, _, _ string, _ time.Time) error {
		wakes = append(wakes, fateWake{"schedule", target, kind, content})
		return nil
	}
	rt.scheduleActorRedrive = func(_ context.Context, _, _, target, kind, content, _, _ string, _ time.Time) error {
		wakes = append(wakes, fateWake{"redrive", target, kind, content})
		return nil
	}
	return &wakes
}

// stageFateProposal puts the bound assignment at freeze_requested (and, with
// frozen, at frozen) with a staged proposal for a failed report, whose
// freeze intent matches that report's proposition.
func stageFateProposal(t *testing.T, f *admissionFixture, bound types.EngineeringAssignment, frozen bool) (types.EngineeringAssignment, types.EngineeringAssignmentReport) {
	t.Helper()
	ctx := context.Background()
	report := types.EngineeringAssignmentReport{Result: types.EngineeringResultFailed, Verdict: types.EngineeringVerdictNone, Summary: "could not finish"}
	digest, err := store.ComputeTerminalPropositionDigest(bound.Binding.SubjectDigest, report.Result, report.Verdict, report.Commands, report.Outputs, report.EvidenceRefs)
	if err != nil {
		t.Fatal(err)
	}
	staged := report
	staged.ReportID = store.TerminalReportID(bound.Binding.OwnerID, bound.Binding.ComputerID, bound.AssignmentID, bound.Binding.Attempt, digest)
	staged.PropositionDigest = digest
	intent := "capsule-freeze-intent:" + digest
	freeze := types.SetEngineeringCapsuleDispositionRequest{
		CommandID: "cmd-redrive-freeze", OwnerID: bound.Binding.OwnerID, ComputerID: bound.Binding.ComputerID,
		AssignmentID: bound.AssignmentID, Attempt: bound.Binding.Attempt, ExpectedLifecycleVersion: bound.LifecycleVersion,
		Disposition: types.EngineeringCapsuleFreezeRequested, IntentRef: intent,
		PendingProposal: &types.EngineeringPendingProposal{PropositionDigest: digest, Report: staged, FreezeIntentRef: intent, CreatedAt: time.Now().UTC()},
	}
	freeze.CommandDigest, _ = store.ComputeSetEngineeringCapsuleDispositionDigest(freeze)
	result, err := f.s.SetEngineeringCapsuleDisposition(ctx, freeze)
	if err != nil {
		t.Fatal(err)
	}
	if frozen {
		ack := freeze
		ack.CommandID, ack.ExpectedLifecycleVersion = "cmd-redrive-frozen", result.Assignment.LifecycleVersion
		ack.Disposition, ack.AckRef, ack.PendingProposal = types.EngineeringCapsuleFrozen, "receipt:redrive-frozen", nil
		ack.CommandDigest, _ = store.ComputeSetEngineeringCapsuleDispositionDigest(ack)
		if result, err = f.s.SetEngineeringCapsuleDisposition(ctx, ack); err != nil {
			t.Fatal(err)
		}
	}
	if !assignedEngineeringFatePending(result.Assignment) {
		t.Fatalf("fixture: the assignment is not fate-pending: %s/%s", result.Assignment.Disposition, result.Assignment.CapsuleDisposition)
	}
	return result.Assignment, report
}

func fateDeadlineContent(a types.EngineeringAssignment) string {
	return fmt.Sprintf(`{"assignment_id":%q,"attempt":%d}`, a.AssignmentID, a.Binding.Attempt)
}

// 1. While the live saga holds the assignment, the fate wake is not
// consumed: its handler returns the held-saga error (actorruntime maps it to
// a dispatcher deferral, which re-delivers the same wake).
func TestFateWakeWhileTheSagaIsHeldIsNotConsumed(t *testing.T) {
	f := newAdmissionFixture(t, 1)
	f.open(0)
	staged, _ := stageFateProposal(t, f, f.bind(0), false)
	saga := f.rt.assignmentSagaMutex(staged.AssignmentID, staged.Binding.Attempt)
	saga.Lock()
	err := f.rt.HandleAssignedEngineeringFateDeadline(context.Background(), staged.Binding.OwnerID, staged.Binding.ComputerID, staged.Binding.ParentAgentID, fateDeadlineContent(staged))
	saga.Unlock()
	if !errors.Is(err, errAssignmentSagaHeld) {
		t.Fatalf("the fate wake under a held saga returned %v; nil consumes the wake and nothing re-drives it", err)
	}
}

// 2. A live saga that fails on an exit that armed nothing (the frozen
// capsule's capability, the exit family of the P1 incident) leaves a re-drive,
// and the re-drive goes through the redrive hook, which mints a new
// generation when the earlier fate wake of the same content was consumed.
func TestEveryLiveSagaFailureOnAFatePendingAssignmentArmsADeliverableRedrive(t *testing.T) {
	for _, frozen := range []bool{false, true} {
		t.Run(fmt.Sprintf("frozen=%v", frozen), func(t *testing.T) {
			f := newAdmissionFixture(t, 1)
			f.open(0)
			staged, report := stageFateProposal(t, f, f.bind(0), frozen)
			f.rt.capsuleExecutor = capsule.NewExecutor(t.TempDir(), t.TempDir(), "", 0) // no capsule: the capability step fails
			wakes := captureFateWakes(f.rt)
			rec := &types.RunRecord{
				RunID: staged.BoundRunID, OwnerID: staged.Binding.OwnerID, ComputerID: staged.Binding.ComputerID, AgentID: staged.Binding.AssignedAgentID,
				Metadata: map[string]any{"assignment_id": staged.AssignmentID, "assignment_attempt": fmt.Sprint(staged.Binding.Attempt)},
			}
			if _, err := f.rt.recordAssignedEngineeringReport(context.Background(), rec, "rlm-complete:test", report); err == nil {
				t.Fatal("fixture: the saga was expected to fail without a capsule")
			}
			var redrives int
			for _, w := range *wakes {
				if w.kind != assignedEngineeringFateDeadlineUpdateKind {
					continue
				}
				if w.hook != "redrive" {
					t.Fatalf("a fate re-drive went through the plain schedule hook, which dedups into the consumed G9 wake: %+v", w)
				}
				redrives++
			}
			if redrives == 0 {
				t.Fatalf("the failed live saga left no re-drive; wakes: %+v", *wakes)
			}
		})
	}
}

// 3. The worker's Complete holds the assignment's saga (review round 4c,
// DeepSeek 2 / Sonnet E3 M2): while it is held elsewhere, the Complete waits.
func TestWorkerCompleteWaitsForTheAssignmentSaga(t *testing.T) {
	f := newAdmissionFixture(t, 1)
	f.open(0)
	bound := f.bind(0)
	f.rt.capsuleExecutor = capsule.NewExecutor(t.TempDir(), t.TempDir(), "", 0)
	rec := &types.RunRecord{
		RunID: bound.BoundRunID, OwnerID: bound.Binding.OwnerID, ComputerID: bound.Binding.ComputerID, AgentID: bound.Binding.AssignedAgentID,
		Metadata: map[string]any{"assignment_id": bound.AssignmentID, "assignment_attempt": fmt.Sprint(bound.Binding.Attempt)},
	}
	r := &rlmCallReduction{active: true, rec: rec, mb: f.rt, toolCtx: &CapsuleToolCtx{Executor: f.rt.capsuleExecutor}, scope: ReductionScope{RunID: rec.RunID}}
	saga := f.rt.assignmentSagaMutex(bound.AssignmentID, bound.Binding.Attempt)
	saga.Lock()
	done := make(chan error, 1)
	go func() {
		_, err := r.commitCompleteIntent(context.Background(), yaegikernel.StagedIntent{Kind: yaegikernel.IntentComplete, Result: "partial", Verdict: "none", Summary: "progress"})
		done <- err
	}()
	select {
	case err := <-done:
		saga.Unlock()
		t.Fatalf("the worker's Complete did not wait for the held saga (returned %v)", err)
	case <-time.After(300 * time.Millisecond):
	}
	saga.Unlock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("the worker's Complete failed after the saga freed: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the worker's Complete never finished after the saga freed")
	}
	if !saga.TryLock() {
		t.Fatal("the worker left the saga locked")
	}
	saga.Unlock()
}

// 4. A freeze committed before a later intent failed is named in commit's
// own error, wherever commit is called (review round 4c, DeepSeek 3 /
// Sonnet E3 M6).
func TestCommitErrorCarriesAFreezeCommittedEarlierInTheCell(t *testing.T) {
	rt, st := testRuntime(t)
	scope := ReductionScope{
		FromAgentID: "engineering:asg-1", DeskAgentID: "engineering:asg-1", FromRole: "engineering",
		ChannelID: "engineering:asg-1", RunID: "run-1", OwnerID: "owner-1", ComputerID: "computer-1",
		ReturnTo: "management:m", CellID: "cell-1",
	}
	r := &rlmCallReduction{active: true, mb: rt, st: st, ledger: st, scope: scope}
	r.freezeResult = map[string]any{"operation_id": "selfdev-88bb", "state": "frozen"} // the cell's freeze committed
	err := r.commit(context.Background(), []yaegikernel.StagedIntent{{Kind: yaegikernel.IntentComplete, LocalID: "tray-2", Result: yaegikernel.CompleteBlocked, Summary: "blocked"}})
	if err == nil {
		t.Fatal("fixture: the Complete was expected to fail without assignment authority")
	}
	if !strings.Contains(err.Error(), committedFreezeMarker) || !strings.Contains(err.Error(), "selfdev-88bb") {
		t.Fatalf("commit's error does not carry the committed freeze: %v", err)
	}
}

// 5. The saga map does not keep an entry for an assignment that is no
// longer fate-pending (review round 4c, DeepSeek 7 / Sonnet E7).
func TestSagaEntryIsReleasedOnceTheAssignmentIsSettled(t *testing.T) {
	f := newAdmissionFixture(t, 1)
	open := f.open(0)
	f.rt.assignmentSagaMutex(open.AssignmentID, open.Binding.Attempt)
	f.cancelOpen(0)
	if err := f.rt.resumeStrandedFateAssignmentIfPending(context.Background(), open.Binding.OwnerID, open.Binding.ComputerID, open.AssignmentID, open.Binding.Attempt); err != nil {
		t.Fatal(err)
	}
	entries := 0
	f.rt.assignmentSagas.Range(func(any, any) bool { entries++; return true })
	if entries != 0 {
		t.Fatalf("a settled assignment keeps %d saga entries", entries)
	}
}
