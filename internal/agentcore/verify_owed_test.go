package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Management ends with a frozen candidate unverified
// (docs/problems/management-ends-without-verify-2026-11-04.md): luna's
// management read the engineer's final Report, ran one image check, closed
// the round with a Report and ended; no Verify, and nothing re-woke it.
//
// Ways this could fail, written first (the problem doc's list):
//  1. The model ends its turn with a Verify owed and nothing says so.
//  2. The guard loops forever, or fails the run, when the model still ends.
//  3. An activation that ends by Done, failure or cancel arms nothing.
//  4. The engineer's Report arms nothing when management's wake is lost.
//  5. The re-check wakes management while it is working.
//  6. The re-check wakes management after the Verify, an abandon or a later
//     revision.
//  7. The re-wakes are unbounded, or come back to back.
//  8. A crash: a pre-boot re-check runs, or a run a crash ended arms one.
//  9. The guard asks for a Verify that Verify would refuse (an open
//     assignment, an ordinary cancel's candidate).
// 10. Texture's frame says nothing while the candidate is frozen.

type scheduledWake struct {
	to, kind, content string
	at                time.Time
}

type scheduleCapture struct {
	mu    sync.Mutex
	wakes []scheduledWake
}

func (c *scheduleCapture) install(rt *Runtime) {
	rt.kernelMode = true
	rt.SetScheduleActor(func(_ context.Context, _, _, to, kind, content, _, _ string, at time.Time) error {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.wakes = append(c.wakes, scheduledWake{to: to, kind: kind, content: content, at: at})
		return nil
	})
}

func (c *scheduleCapture) ofKind(kind string) []scheduledWake {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []scheduledWake
	for _, w := range c.wakes {
		if w.kind == kind {
			out = append(out, w)
		}
	}
	return out
}

// completedFrozen is a completed assignment whose candidate is frozen: the
// luna state.
func completedFrozen(t *testing.T, owner string) (*Runtime, *store.Store, types.EngineeringAssignment, string, *scheduleCapture) {
	t.Helper()
	rt, s, assignment := assignmentEndFixture(t, owner)
	capture := &scheduleCapture{}
	capture.install(rt)
	operationID := seedAssignmentOperation(t, rt, assignment, selfdev.StateFrozen, "")
	if _, err := rt.commitAssignedEngineeringReport(context.Background(), c1eGet(t, s, assignment), terminalAssignmentReport(assignment, types.EngineeringResultCompleted)); err != nil {
		t.Fatalf("terminal report: %v", err)
	}
	return rt, s, c1eGet(t, s, assignment), operationID, capture
}

func managementRun(assignment types.EngineeringAssignment, state types.RunState) *types.RunRecord {
	return &types.RunRecord{RunID: "run-mgmt-" + string(state), OwnerID: assignment.Binding.OwnerID, ComputerID: assignment.Binding.ComputerID,
		AgentID: assignment.Binding.ParentAgentID, AgentProfile: "management", AgentRole: "management", State: state}
}

func TestManagementTurnWithVerifyOwedIsSentBackBoundedly(t *testing.T) {
	ctx := context.Background()
	rt, _, assignment, operationID, _ := completedFrozen(t, "owner-guard")
	rec := managementRun(assignment, types.RunRunning)
	guard := rt.managementVerifyOwedGuard(rec)

	got, err := guard(ctx, toolregistry.ToolLoopCompletionState{Attempts: 0})
	if err != nil || !got.Continue || got.Waited {
		t.Fatalf("guard with a Verify owed = %+v, %v; want a reminder (item 1)", got, err)
	}
	want := "Verify owed on " + assignment.AssignmentID + ": call choir.Verify(" + assignment.AssignmentID + ", pass|fail, reasons) before ending."
	if !strings.HasPrefix(got.Instruction, want) || !strings.Contains(got.Instruction, operationID) || !strings.Contains(got.Instruction, strings.Repeat("f", 64)) {
		t.Fatalf("reminder = %q, want it to start %q and name the candidate and bundle (item 1)", got.Instruction, want)
	}
	if got, _ := guard(ctx, toolregistry.ToolLoopCompletionState{Attempts: 1}); !got.Continue {
		t.Fatal("second end: want the second reminder")
	}
	if got, err := guard(ctx, toolregistry.ToolLoopCompletionState{Attempts: verifyOwedGuardReminders}); err != nil || got.Continue {
		t.Fatalf("end after %d reminders = %+v, %v; want the turn accepted (item 2)", verifyOwedGuardReminders, got, err)
	}
	if verifyOwedGuardReminders > 2 {
		t.Fatal("the guard's reminders must stay under the tool loop's retry bound, or the run fails (item 2)")
	}

	// After the Verify the candidate is no longer frozen: no reminder.
	if _, err := rt.store.DB().Exec(`UPDATE self_development_operations SET state=? WHERE operation_id=?`, selfdev.StateAwaitingApproval, operationID); err != nil {
		t.Fatal(err)
	}
	if got, _ := guard(ctx, toolregistry.ToolLoopCompletionState{}); got.Continue {
		t.Fatalf("guard after the Verify = %+v, want none (item 6)", got)
	}
}

func TestVerifyOwedOnlyWhereVerifyIsAdmitted(t *testing.T) {
	ctx := context.Background()
	// An open assignment's frozen candidate is not Verify's yet.
	rt, _, assignment, run := retainedFixture(t, "owner-open")
	seedAssignmentOperation(t, rt, assignment, selfdev.StateFrozen, "")
	if owed, _ := rt.verifyOwedBy(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignment.Binding.ParentAgentID); len(owed) != 0 {
		t.Fatalf("open assignment owed = %+v, want none (item 9)", owed)
	}
	// An ordinary cancel fails its candidate; nothing is owed.
	if _, err := rt.cancelBoundEngineeringRun(ctx, run, "persistent Management cancelled assignment"); err != nil {
		t.Fatal(err)
	}
	if owed, _ := rt.verifyOwedBy(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignment.Binding.ParentAgentID); len(owed) != 0 {
		t.Fatalf("ordinary cancel owed = %+v, want none (item 9)", owed)
	}

	// A retained candidate at its build's end is owed.
	rt2, _, assignment2, run2 := retainedFixture(t, "owner-retained-owed")
	seedAssignmentOperation(t, rt2, assignment2, selfdev.StateFrozen, "")
	if _, err := rt2.cancelBoundEngineeringRun(withBuildBudgetEnd(ctx), run2, activationBudgetExceededReason); err != nil {
		t.Fatal(err)
	}
	owed, err := rt2.verifyOwedBy(ctx, assignment2.Binding.OwnerID, assignment2.Binding.ComputerID, assignment2.Binding.ParentAgentID)
	if err != nil || len(owed) != 1 || owed[0].Why != verifyOwedRetained {
		t.Fatalf("retained candidate owed = %+v, %v; want one, retained", owed, err)
	}
	// It already shows as its decision wait; the pack does not repeat it.
	if views := rt2.verifyOwedWaitViews(ctx, assignment2.Binding.OwnerID, assignment2.Binding.ComputerID, assignment2.Binding.ParentAgentID); len(views) != 0 {
		t.Fatalf("pack repeats the retained candidate: %+v", views)
	}
	// Another desk owes nothing.
	if owed, _ := rt2.verifyOwedBy(ctx, assignment2.Binding.OwnerID, assignment2.Binding.ComputerID, "management:someone-else"); len(owed) != 0 {
		t.Fatalf("another desk owes %+v", owed)
	}
}

func TestActivationEndAndReportArmTheReCheck(t *testing.T) {
	ctx := context.Background()
	rt, _, assignment, operationID, capture := completedFrozen(t, "owner-arm")
	// The engineer's Report path (recordAssignedEngineeringReport) arms;
	// commitAssignedEngineeringReport is its inner commit, so arm as it does.
	rt.armVerifyOwedAfterReport(ctx, assignment, "report-x")
	for _, state := range []types.RunState{types.RunCompleted, types.RunFailed, types.RunCancelled} {
		rt.armVerifyOwedAfterRun(ctx, managementRun(assignment, state))
	}
	crashed := managementRun(assignment, types.RunFailed)
	crashed.RunID, crashed.Error = "run-crashed", runFateInterruptedByRestart
	rt.armVerifyOwedAfterRun(ctx, crashed)

	wakes := capture.ofKind(verifyOwedUpdateKind)
	if len(wakes) != 4 {
		t.Fatalf("verify_owed arms = %d (%+v), want the Report's and three ends, none for the crash (items 3, 4, 8)", len(wakes), wakes)
	}
	for _, w := range wakes {
		var wake verifyOwedWake
		if err := json.Unmarshal([]byte(w.content), &wake); err != nil || wake.AssignmentID != assignment.AssignmentID || wake.OperationID != operationID {
			t.Fatalf("wake content = %q", w.content)
		}
		if w.to != assignment.Binding.ParentAgentID || strings.Contains(wake.ArmedBy, "run-crashed") {
			t.Fatalf("wake = %+v", w)
		}
		if d := time.Until(w.at); d < 30*time.Second || d > 2*time.Minute {
			t.Fatalf("re-check in %s, want about 1 to 2 minutes", d)
		}
	}
	if !RestartWorkKind(verifyOwedUpdateKind) {
		t.Fatal("verify_owed is not a restart work kind: a crash boot would run a pre-boot re-check (item 8)")
	}
}

func TestReCheckWakesIdleManagementBoundedly(t *testing.T) {
	ctx := context.Background()
	rt, s, assignment, operationID, capture := completedFrozen(t, "owner-wake")
	owner, computer, desk := assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignment.Binding.ParentAgentID
	content, _ := json.Marshal(verifyOwedWake{AssignmentID: assignment.AssignmentID, OperationID: operationID, ArmedBy: "run:x"})
	notices := func() []types.CommitmentRecord {
		t.Helper()
		all, err := s.ListInboxRecords(ctx, owner, computer, desk, 0, 1<<40)
		if err != nil {
			t.Fatal(err)
		}
		var out []types.CommitmentRecord
		for _, r := range all {
			if types.IsVerifyOwedNotice(r) {
				out = append(out, r)
			}
		}
		return out
	}

	// The seed's management run is running and fresh: management is working.
	base := time.Now().UTC()
	rt.admissionNow = func() time.Time { return base }
	if err := rt.HandleVerifyOwed(ctx, owner, computer, desk, string(content)); err != nil {
		t.Fatal(err)
	}
	if n := notices(); len(n) != 0 {
		t.Fatalf("notices while management works = %d, want none (item 5)", len(n))
	}

	// The run ends: management is idle.
	active, found, err := rt.activeRunByAgent(ctx, owner, desk)
	if err != nil || !found {
		t.Fatalf("seed management run: %v %v", found, err)
	}
	finished := time.Now().UTC()
	active.State, active.FinishedAt, active.UpdatedAt = types.RunCompleted, &finished, finished
	if err := s.UpdateRun(ctx, active); err != nil {
		t.Fatal(err)
	}
	now := base.Add(deadRunGrace + time.Minute)
	rt.admissionNow = func() time.Time { return now }
	if err := rt.HandleVerifyOwed(ctx, owner, computer, desk, string(content)); err != nil {
		t.Fatal(err)
	}
	got := notices()
	if len(got) != 1 || !store.RecordWakesReader(got[0]) || inboxRecordKind(got[0]) != "verify_owed" {
		t.Fatalf("notices = %+v, want one that wakes management, kind verify_owed (item 1)", got)
	}
	if body := got[0].Directive.Body; !strings.HasPrefix(body, "Verify owed on "+assignment.AssignmentID+": call choir.Verify(") || !strings.Contains(body, "Notice 1 of 2") {
		t.Fatalf("notice body = %q", body)
	}
	// A replay commits nothing new.
	last, _ := time.Parse(time.RFC3339Nano, got[0].Provenance.CommittedAt)
	if last.IsZero() {
		last = now
	}
	rt.admissionNow = func() time.Time { return last.Add(10 * time.Second) }
	if err := rt.HandleVerifyOwed(ctx, owner, computer, desk, string(content)); err != nil {
		t.Fatal(err)
	}
	if n := notices(); len(n) != 1 {
		t.Fatalf("notices 10 s after the first = %d, want still one (item 7)", len(n))
	}
	// Spaced out but notice 1 is unread. A run-end re-check (any number of
	// them) neither spends a notice nor wakes; the notice's own re-check
	// wakes management for it, once (review r1, Devin 1, 2; r1b, DeepSeek B1).
	rt.admissionNow = func() time.Time { return last.Add(2 * time.Minute) }
	for i := 0; i < 3; i++ {
		other, _ := json.Marshal(verifyOwedWake{AssignmentID: assignment.AssignmentID, OperationID: operationID, ArmedBy: "run:failed-" + string(rune('a'+i))})
		if err := rt.HandleVerifyOwed(ctx, owner, computer, desk, string(other)); err != nil {
			t.Fatal(err)
		}
	}
	if _, found, _ := rt.activeRunByAgent(ctx, owner, desk); found {
		t.Fatal("a run-end re-check woke management for an unread notice (unbounded re-wakes, item 7)")
	}
	if n := notices(); len(n) != 1 {
		t.Fatalf("notices with notice 1 unread = %d, want still one", len(n))
	}
	chained := false
	for _, w := range capture.ofKind(verifyOwedUpdateKind) {
		chained = chained || strings.Contains(w.content, "notice:"+got[0].RecordID)
	}
	if !chained {
		t.Fatal("notice 1 armed no re-check of its own")
	}
	own, _ := json.Marshal(verifyOwedWake{AssignmentID: assignment.AssignmentID, OperationID: operationID, ArmedBy: "notice:" + got[0].RecordID})
	if err := rt.HandleVerifyOwed(ctx, owner, computer, desk, string(own)); err != nil {
		t.Fatal(err)
	}
	if n := notices(); len(n) != 1 {
		t.Fatalf("notices after the re-wake = %d, want still one", len(n))
	}
	// The re-wake started a management turn, which reads the notice and
	// ends.
	woken, found, err := rt.activeRunByAgent(ctx, owner, desk)
	if err != nil || !found {
		t.Fatalf("the re-wake started no management run: %v %v", found, err)
	}
	ended := time.Now().UTC()
	woken.State, woken.FinishedAt, woken.UpdatedAt = types.RunCompleted, &ended, ended
	if err := s.UpdateRun(ctx, woken); err != nil {
		t.Fatal(err)
	}
	cursor, err := s.ReaderCursor(ctx, owner, computer, desk, desk)
	if err != nil {
		t.Fatal(err)
	}
	read := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: "read-notice-1",
		Cursor: &types.InboxCursorAdvance{ReaderAgentID: desk, Inbox: desk, ExpectedSeq: cursor, Seq: got[0].InboxSeq}}
	if read.CommandDigest, err = store.ComputeCommitInboxRecordsDigest(read); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CommitInboxRecords(ctx, read); err != nil {
		t.Fatalf("mark notice 1 read: %v", err)
	}
	// Read but inside the spacing: no notice, a spaced re-check.
	rt.admissionNow = func() time.Time { return last.Add(10 * time.Second) }
	if err := rt.HandleVerifyOwed(ctx, owner, computer, desk, string(content)); err != nil {
		t.Fatal(err)
	}
	spaced := false
	for _, w := range capture.ofKind(verifyOwedUpdateKind) {
		spaced = spaced || strings.Contains(w.content, "spaced:"+got[0].RecordID)
	}
	if n := notices(); len(n) != 1 || !spaced {
		t.Fatalf("inside the spacing: notices %d, spaced re-check %v; want one and a re-check (item 7)", len(n), spaced)
	}
	// Spaced out and read: the second, then the bound.
	for i, at := range []time.Duration{2 * time.Minute, 4 * time.Minute} {
		rt.admissionNow = func() time.Time { return last.Add(at) }
		if err := rt.HandleVerifyOwed(ctx, owner, computer, desk, string(content)); err != nil {
			t.Fatal(err)
		}
		if n := notices(); len(n) != min(2+i, types.VerifyOwedMaxNotices) {
			t.Fatalf("notices after re-check %d = %d, want at most %d (item 7)", i+2, len(n), types.VerifyOwedMaxNotices)
		}
	}
	// The pack shows the wait.
	views := rt.verifyOwedWaitViews(ctx, owner, computer, desk)
	if len(views) != 1 || views[0].Wait == nil || views[0].Wait.Kind != types.WaitKindVerifyOwed || views[0].Wait.Class != types.WaitClassPolicy ||
		!strings.Contains(views[0].Wait.Sentence, "management's Verify of frozen candidate "+operationID) {
		t.Fatalf("pack waits = %+v, want one verify_owed policy wait", views)
	}
}

func TestReCheckAfterTheVerifyDoesNothing(t *testing.T) {
	ctx := context.Background()
	rt, s, assignment, operationID, _ := completedFrozen(t, "owner-after")
	owner, computer, desk := assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignment.Binding.ParentAgentID
	rt.admissionNow = func() time.Time { return time.Now().Add(deadRunGrace + time.Minute) }
	for name, state := range map[string]string{"verified": selfdev.StateAwaitingApproval, "failed": selfdev.StateFailed} {
		if _, err := rt.store.DB().Exec(`UPDATE self_development_operations SET state=? WHERE operation_id=?`, state, operationID); err != nil {
			t.Fatal(err)
		}
		content, _ := json.Marshal(verifyOwedWake{AssignmentID: assignment.AssignmentID, OperationID: operationID, ArmedBy: "run:" + name})
		if err := rt.HandleVerifyOwed(ctx, owner, computer, desk, string(content)); err != nil {
			t.Fatal(err)
		}
	}
	// A wake naming another operation (a later revision replaced it).
	if _, err := rt.store.DB().Exec(`UPDATE self_development_operations SET state=? WHERE operation_id=?`, selfdev.StateFrozen, operationID); err != nil {
		t.Fatal(err)
	}
	content, _ := json.Marshal(verifyOwedWake{AssignmentID: assignment.AssignmentID, OperationID: "selfdev-older", ArmedBy: "run:old"})
	if err := rt.HandleVerifyOwed(ctx, owner, computer, desk, string(content)); err != nil {
		t.Fatal(err)
	}
	all, _ := s.ListInboxRecords(ctx, owner, computer, desk, 0, 1<<40)
	for _, r := range all {
		if types.IsVerifyOwedNotice(r) {
			t.Fatalf("notice %s after the Verify or for another candidate (item 6)", r.RecordID)
		}
	}
}

func TestFreezeKeyAssignment(t *testing.T) {
	for key, want := range map[string]string{
		store.EngineeringFreezeIdempotencyKey("assignment-a"):            "assignment-a",
		store.EngineeringFreezeIdempotencyKey("assignment-a") + ":rev:2": "assignment-a",
		store.EngineeringFreezeIdempotencyKey("assignment-a") + ":rev:x": "assignment-a:rev:x",
	} {
		if got, ok := freezeKeyAssignment(key); !ok || got != want {
			t.Fatalf("freezeKeyAssignment(%q) = %q, %v; want %q", key, got, ok, want)
		}
	}
	if _, ok := freezeKeyAssignment("rollback:x"); ok {
		t.Fatal("a non-freeze key names an assignment")
	}
}
