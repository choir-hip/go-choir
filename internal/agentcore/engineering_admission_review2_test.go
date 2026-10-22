package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Round 2 review of the FP4 candidate (bunch 30; DeepSeek findings 1 to 5,
// Devin finding 3;
// docs/problems/engineering-admission-supersedes-live-assignment-2026-10-21.md).
// Ways each can fail, written before the code:
//   R1. The clock crosses a request's deadline between the expiry sweep and
//       admission: admission refuses it as expired, the wake reads that as
//       already cancelled, and the row stays open and unbound with no wake.
//   R2. A planned restart lands between the bind commit and the deadline arm:
//       the bound assignment has no deadline wake and the boot leaves it alone.
//   R3. A woken assignment cannot freeze while an earlier candidate is open,
//       and the refusal it sees does not say why or what to do.
//   R4. The nudge picks the expired queue head and wakes nobody.
//   R5. A cancelled request's reason says it remains pending and retryable.

// R1: admission's own expiry check closes the request, as the sweep does.
func TestEngineeringAdmissionExpiryBetweenSweepAndAdmissionCancelsTheRequest(t *testing.T) {
	t.Setenv("CHOIR_ASSIGNMENT_DEADLINE", "1h")
	f := newAdmissionFixture(t, 1)
	wakes := captureAdmissionWakes(f.rt)
	only := f.open(0)
	calls := 0
	f.rt.admissionNow = func() time.Time {
		calls++
		if calls == 1 { // the expiry sweep: not yet due
			return only.CreatedAt.Add(time.Hour)
		}
		return only.CreatedAt.Add(time.Hour + time.Second) // admission: past due
	}

	content := fmt.Sprintf(`{"assignment_id":%q,"attempt":%d}`, only.AssignmentID, only.Binding.Attempt)
	if err := f.rt.HandleDelegatedAssignmentSpawnDeadline(context.Background(), f.seed.OwnerID, f.computer, f.seed.ParentAgentID, content); err != nil {
		t.Fatalf("spawn wake: %v", err)
	}
	if calls < 2 {
		t.Fatalf("fixture: admission read the clock %d times, want the sweep and admission", calls)
	}
	got := f.get(0)
	if got.Disposition != types.EngineeringAssignmentCancelled || !strings.Contains(got.DispositionReason, "deadline") {
		t.Fatalf("a request that expired between the sweep and admission was left %s (%q) with its wake consumed", got.Disposition, got.DispositionReason)
	}
	if armed := f.spawnWakesFor(*wakes, 0); len(armed) != 0 {
		t.Fatalf("an expired request armed %d wakes", len(armed))
	}
}

// presentAssignmentCapsule is a capsule runtime whose bound capsules are
// alive, so a planned boot keeps their assignments.
type presentAssignmentCapsule struct{ absentAssignmentCapsule }

func (presentAssignmentCapsule) AssignmentHandle(string, string) (string, error) {
	return "handle-present", nil
}
func (presentAssignmentCapsule) InspectCapsuleRaw(id string) (*capsule.CapsuleDiagnostics, error) {
	return &capsule.CapsuleDiagnostics{ID: id, State: capsule.StateActive}, nil
}
func (presentAssignmentCapsule) HasCapsule(string) bool                     { return true }
func (presentAssignmentCapsule) ForceDestroy(context.Context, string) error { return nil }

// R2: a planned boot re-arms the bound deadline of every bound, active
// assignment; the wake has the bind-time identity, so one that survived
// dedups.
func TestEngineeringPlannedBootRearmsTheBoundDeadline(t *testing.T) {
	f := newAdmissionFixture(t, 1)
	f.rt.assignmentRuntime = presentAssignmentCapsule{}
	wakes := captureAdmissionWakes(f.rt)
	f.open(0)
	bound := f.bind(0) // the store bind: no deadline wake, as if the arm was lost
	f.restart(true)
	*wakes = nil

	f.rt.reconcileEngineeringAssignmentCapsulesAfterRestart(context.Background())

	if got := f.get(0); got.Disposition != types.EngineeringAssignmentBound {
		t.Fatalf("fixture: the planned boot did not keep the bound assignment: %s (%q)", got.Disposition, got.DispositionReason)
	}
	want, _ := json.Marshal(assignedEngineeringFateDeadline{AssignmentID: bound.AssignmentID, Attempt: bound.Binding.Attempt, Stage: assignedEngineeringBoundDeadlineStage})
	var armed []admissionWake
	for _, wake := range *wakes {
		if wake.kind == assignedEngineeringFateDeadlineUpdateKind {
			armed = append(armed, wake)
		}
	}
	if len(armed) != 1 || armed[0].content != string(want) {
		t.Fatalf("planned boot armed %+v, want one bound deadline wake %s", armed, want)
	}
}

// R2, crash half: a crash boot arms no deadline (it closes the assignment;
// that close is covered by the boot reconcile tests, and this double cannot
// acknowledge a destroy).
func TestEngineeringCrashBootArmsNoBoundDeadline(t *testing.T) {
	f := newAdmissionFixture(t, 1)
	f.rt.assignmentRuntime = presentAssignmentCapsule{}
	wakes := captureAdmissionWakes(f.rt)
	f.open(0)
	f.bind(0)
	f.restart(false)
	*wakes = nil

	f.rt.reconcileEngineeringAssignmentCapsulesAfterRestart(context.Background())

	for _, wake := range *wakes {
		if wake.kind == assignedEngineeringFateDeadlineUpdateKind {
			t.Fatalf("a crash boot armed a bound deadline: %+v", wake)
		}
	}
}

// R3: the refusal names the open operation and says what to do.
func TestFreezeRefusedByAnOpenCandidateSaysWhy(t *testing.T) {
	ctx := context.Background()
	_, operations, computerID := freezeCandidateTestStore(t)
	authorized := func(context.Context) error { return nil }
	first, err := openFreezeCandidate(ctx, operations, authorized, computerID, "trajectory-a", "engineering:assignment-a", "first app")
	if err != nil {
		t.Fatal(err)
	}
	_, err = openFreezeCandidate(ctx, operations, authorized, computerID, "trajectory-b", "engineering:assignment-b", "second app")
	if err == nil {
		t.Fatal("a second candidate opened while the first is open")
	}
	if !errors.Is(err, selfdev.ErrOperationOpen) {
		t.Fatalf("the refusal lost its type: %v", err)
	}
	for _, want := range []string{first.OperationID, "cannot freeze until", "owner", "report"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("the refusal does not say %q: %v", want, err)
		}
	}
}

// R4: the nudge skips an expired queue head and wakes the first live request.
func TestEngineeringAdmissionNudgeSkipsAnExpiredQueueHead(t *testing.T) {
	t.Setenv("CHOIR_ASSIGNMENT_DEADLINE", "1h")
	f := newAdmissionFixture(t, 3)
	wakes := captureAdmissionWakes(f.rt)
	f.open(0)
	f.bind(0)
	f.open(1)
	younger := f.open(2)
	// The older request is past its deadline; the younger stands exactly at
	// its own, so it is still live.
	f.rt.admissionNow = func() time.Time { return younger.CreatedAt.Add(time.Hour) }
	*wakes = nil

	f.finish(0)

	if next := f.spawnWakesFor(*wakes, 2); len(next) != 1 {
		t.Fatalf("the slot freed behind an expired head and the live request got %d wakes, want one: %+v", len(next), *wakes)
	}
	if expired := f.spawnWakesFor(*wakes, 1); len(expired) != 0 {
		t.Fatalf("the expired head was nudged: %+v", expired)
	}
}

// R5: a cancelled request's reason does not claim it is pending.
func TestEngineeringCancelReasonsDoNotClaimPending(t *testing.T) {
	t.Setenv("CHOIR_ASSIGNMENT_DEADLINE", "1h")
	f := newAdmissionFixture(t, 2)
	captureAdmissionWakes(f.rt)
	queued := f.open(0)
	f.open(1)
	bound := f.bind(1)
	f.rt.admissionNow = func() time.Time { return queued.CreatedAt.Add(2 * time.Hour) }
	ctx := context.Background()

	content := fmt.Sprintf(`{"assignment_id":%q,"attempt":%d}`, queued.AssignmentID, queued.Binding.Attempt)
	if err := f.rt.HandleDelegatedAssignmentSpawnDeadline(ctx, f.seed.OwnerID, f.computer, f.seed.ParentAgentID, content); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CHOIR_ASSIGNMENT_DEADLINE", "1ms")
	time.Sleep(20 * time.Millisecond)
	deadline, _ := json.Marshal(assignedEngineeringFateDeadline{AssignmentID: bound.AssignmentID, Attempt: bound.Binding.Attempt, Stage: assignedEngineeringBoundDeadlineStage})
	if err := f.rt.HandleAssignedEngineeringFateDeadline(ctx, f.seed.OwnerID, f.computer, f.seed.ParentAgentID, string(deadline)); err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"expired queued request", "bound assignment past its deadline"} {
		got := f.get(i)
		if got.Disposition != types.EngineeringAssignmentCancelled {
			t.Fatalf("fixture: %s is %s (%q)", name, got.Disposition, got.DispositionReason)
		}
		if strings.Contains(got.DispositionReason, "pending") || strings.Contains(got.DispositionReason, "retryable") {
			t.Fatalf("%s is cancelled but its reason says otherwise: %q", name, got.DispositionReason)
		}
	}
}
