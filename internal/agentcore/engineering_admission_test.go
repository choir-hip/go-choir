package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Engineering admission (SDC ARCH-10 launch half, launch item L3-08;
// docs/problems/engineering-admission-supersedes-live-assignment-2026-10-21.md).
// The doctrine's admission ledger is FIFO with a retryable refusal and work
// left pending; the spawn saga used to cancel the live assignment instead.

type admissionFixture struct {
	t        *testing.T
	rt       *Runtime
	s        *store.Store
	seed     store.EngineeringAssignmentSeed
	opens    []types.OpenEngineeringAssignmentRequest
	computer string
}

func newAdmissionFixture(t *testing.T, count int) *admissionFixture {
	t.Helper()
	rt, s := testRuntime(t)
	rt.assignmentRuntime = absentAssignmentCapsule{}
	computerID := rt.TextureComputerID()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-admission", computerID, count)
	if err != nil {
		t.Fatal(err)
	}
	return &admissionFixture{t: t, rt: rt, s: s, seed: seed, computer: computerID}
}

// open commits assignment index (the durable request) and returns it.
func (f *admissionFixture) open(index int) types.EngineeringAssignment {
	f.t.Helper()
	req := store.SeedEngineeringOpenRequest(f.seed, index, fmt.Sprintf("opaque-admission-%d", index))
	opened, err := f.s.OpenEngineeringAssignment(context.Background(), req)
	if err != nil {
		f.t.Fatalf("open assignment %d: %v", index, err)
	}
	f.opens = append(f.opens, req)
	// Arrival order is the durable creation time; keep the seeds apart.
	time.Sleep(3 * time.Millisecond)
	return opened.Assignment
}

// bind makes assignment index live: bound to a run with an active capsule.
func (f *admissionFixture) bind(index int) types.EngineeringAssignment {
	f.t.Helper()
	open := f.opens[index]
	bound, err := f.s.BindEngineeringAssignment(context.Background(),
		store.SeedEngineeringBindRequest(open, "run:"+open.AssignmentID, fmt.Sprintf("opaque-admission-%d", index)))
	if err != nil {
		f.t.Fatalf("bind assignment %d: %v", index, err)
	}
	return bound.Assignment
}

func (f *admissionFixture) get(index int) types.EngineeringAssignment {
	f.t.Helper()
	open := f.opens[index]
	got, err := f.s.GetEngineeringAssignment(context.Background(), open.Binding.OwnerID, open.Binding.ComputerID, open.AssignmentID, open.Binding.Attempt)
	if err != nil {
		f.t.Fatal(err)
	}
	return got
}

func (f *admissionFixture) parent() types.RunRecord {
	return types.RunRecord{
		RunID: f.seed.ParentRunID, OwnerID: f.seed.OwnerID, ComputerID: f.computer,
		AgentID: f.seed.ParentAgentID, AgentProfile: agentprofile.Management, AgentRole: agentprofile.Management,
		State: types.RunRunning, Metadata: map[string]any{},
	}
}

// A second assignment's spawn must not cancel the live one. Before the fix,
// the reclaim cancelled every other assignment that held a capsule, "superseded
// by a fresh implementation assignment".
func TestEngineeringAdmissionDoesNotCancelLiveAssignment(t *testing.T) {
	f := newAdmissionFixture(t, 2)
	f.open(0)
	live := f.bind(0)
	if live.Disposition != types.EngineeringAssignmentBound || live.CapsuleDisposition != types.EngineeringCapsuleActive {
		t.Fatalf("fixture: live assignment is %s/%s, want bound/active", live.Disposition, live.CapsuleDisposition)
	}
	second := f.open(1)

	_ = f.rt.reclaimTerminalAssignmentCapsules(context.Background(), f.parent(), second.AssignmentID)

	after := f.get(0)
	if after.Disposition.Terminal() {
		t.Fatalf("the live assignment was cancelled by a later request: %s (%s)", after.Disposition, after.DispositionReason)
	}
	if after.CapsuleDisposition != types.EngineeringCapsuleActive {
		t.Fatalf("the live assignment's capsule was touched: %s", after.CapsuleDisposition)
	}
}

// cancelOpen cancels an open, unbound request (a request that never bound a
// capsule) through the store, as the open-failed path does.
func (f *admissionFixture) cancelOpen(index int) {
	f.t.Helper()
	open := f.opens[index]
	cancel := types.CancelEngineeringAssignmentRequest{
		CommandID: fmt.Sprintf("command-admission-cancel-%d", index), OwnerID: open.Binding.OwnerID, ComputerID: open.Binding.ComputerID,
		AssignmentID: open.AssignmentID, Attempt: open.Binding.Attempt, ExpectedLifecycleVersion: 1, Reason: "test: request withdrawn",
	}
	cancel.CommandDigest, _ = store.ComputeCancelEngineeringAssignmentDigest(cancel)
	if _, err := f.s.CancelEngineeringAssignment(context.Background(), cancel); err != nil {
		f.t.Fatalf("cancel open assignment %d: %v", index, err)
	}
}

// finish ends the live assignment through the production cancel path.
func (f *admissionFixture) finish(index int) {
	f.t.Helper()
	open := f.opens[index]
	if _, err := f.rt.cancelAssignedEngineering(context.Background(), f.parent(), open.AssignmentID, open.Binding.Attempt, "test: live assignment ended"); err != nil {
		f.t.Fatalf("end live assignment %d: %v", index, err)
	}
	if !f.get(index).Disposition.Terminal() {
		f.t.Fatalf("assignment %d is not terminal after its fate", index)
	}
}

func requireAdmitted(t *testing.T, f *admissionFixture, index int, want string) {
	t.Helper()
	if err := f.rt.admitEngineeringAssignment(context.Background(), f.get(index)); err != nil {
		t.Fatalf("%s: assignment %d refused: %v", want, index, err)
	}
}

func requireBusy(t *testing.T, f *admissionFixture, index int, blockedBy int, want string) {
	t.Helper()
	err := f.rt.admitEngineeringAssignment(context.Background(), f.get(index))
	if err == nil {
		t.Fatalf("%s: assignment %d was admitted, want a retryable refusal", want, index)
	}
	if !errors.Is(err, ErrEngineeringAdmissionBusy) {
		t.Fatalf("%s: refusal is not the typed retryable busy error: %v", want, err)
	}
	if !strings.Contains(err.Error(), f.opens[blockedBy].AssignmentID) {
		t.Fatalf("%s: refusal does not name the assignment it waits behind (%s): %v", want, f.opens[blockedBy].AssignmentID, err)
	}
}

// A request is refused, retryably, while another assignment holds the slot; it
// stays open and unbound ("work left pending"), and is admitted once the live
// assignment has a fate.
func TestEngineeringAdmissionRefusesWhileAnotherAssignmentIsLive(t *testing.T) {
	f := newAdmissionFixture(t, 2)
	f.open(0)
	f.bind(0)
	f.open(1)

	requireBusy(t, f, 1, 0, "live assignment holds the slot")
	live, queued := f.get(0), f.get(1)
	if live.Disposition != types.EngineeringAssignmentBound || live.CapsuleDisposition != types.EngineeringCapsuleActive {
		t.Fatalf("the refusal touched the live assignment: %s/%s", live.Disposition, live.CapsuleDisposition)
	}
	if queued.Disposition != types.EngineeringAssignmentOpen || queued.CapsuleDisposition != types.EngineeringCapsuleUnbound {
		t.Fatalf("the refused request is not left open and unbound: %s/%s", queued.Disposition, queued.CapsuleDisposition)
	}

	f.finish(0)
	requireAdmitted(t, f, 1, "slot freed by the live assignment's fate")
}

// Selection is FIFO among open requests, by durable arrival order, whatever
// order their retries happen to fire in.
func TestEngineeringAdmissionIsFIFO(t *testing.T) {
	f := newAdmissionFixture(t, 3)
	f.open(0)
	f.open(1)
	f.open(2)

	requireAdmitted(t, f, 0, "the oldest request goes first")
	requireBusy(t, f, 1, 0, "second request waits for the first")
	requireBusy(t, f, 2, 0, "third request waits for the first")

	f.cancelOpen(0)
	requireAdmitted(t, f, 1, "the second request is next once the first is gone")
	requireBusy(t, f, 2, 1, "third request waits for the second")
}

// A queued request past the assignment deadline expires: it no longer blocks
// the younger requests behind it (a stranded request cannot hold the line).
func TestEngineeringAdmissionExpiredQueuedRequestDoesNotBlock(t *testing.T) {
	t.Setenv("CHOIR_ASSIGNMENT_DEADLINE", "1ms")
	f := newAdmissionFixture(t, 2)
	f.open(0)
	f.open(1)
	time.Sleep(20 * time.Millisecond)

	requireAdmitted(t, f, 1, "the older request expired in the queue")
}

// The retry is a restart-work kind: a crash boot drops a queued request's
// wake, so a queue never becomes a boot-time resumer (CLAUDE.md, Restarts End
// Work).
func TestEngineeringAdmissionRetryWakeIsRestartWork(t *testing.T) {
	if !RestartWorkKinds[delegatedAssignmentSpawnDeadlineUpdateKind] {
		t.Fatalf("%s is not a restart-work kind; a crash boot would resume a queued request", delegatedAssignmentSpawnDeadlineUpdateKind)
	}
}

type admissionWake struct {
	kind      string
	content   string
	notBefore time.Time
	at        time.Time
}

func captureAdmissionWakes(rt *Runtime) *[]admissionWake {
	var wakes []admissionWake
	rt.kernelMode = true
	rt.scheduleActor = func(_ context.Context, _, _, _, kind, content, _, _ string, notBefore time.Time) error {
		wakes = append(wakes, admissionWake{kind: kind, content: content, notBefore: notBefore, at: time.Now()})
		return nil
	}
	return &wakes
}

// The spawn wake of a request that has to wait leaves the live assignment
// alone, leaves the request open, and arms exactly one retry wake with a
// higher retry count and a longer backoff each time.
func TestEngineeringAdmissionBusyWakeRearmsARetry(t *testing.T) {
	f := newAdmissionFixture(t, 2)
	wakes := captureAdmissionWakes(f.rt)
	f.open(0)
	f.bind(0)
	queued := f.open(1)
	ctx := context.Background()

	content := fmt.Sprintf(`{"assignment_id":%q,"attempt":%d}`, queued.AssignmentID, queued.Binding.Attempt)
	var delays []time.Duration
	for round := 1; round <= 3; round++ {
		before := len(*wakes)
		if err := f.rt.HandleDelegatedAssignmentSpawnDeadline(ctx, f.seed.OwnerID, f.computer, f.seed.ParentAgentID, content); err != nil {
			t.Fatalf("round %d: a busy refusal must be consumed, not failed: %v", round, err)
		}
		if got := len(*wakes) - before; got != 1 {
			t.Fatalf("round %d: %d retry wakes armed, want exactly one", round, got)
		}
		wake := (*wakes)[len(*wakes)-1]
		if wake.kind != delegatedAssignmentSpawnDeadlineUpdateKind {
			t.Fatalf("round %d: retry kind = %q", round, wake.kind)
		}
		var armed delegatedAssignmentSpawnDeadline
		if err := json.Unmarshal([]byte(wake.content), &armed); err != nil {
			t.Fatal(err)
		}
		if armed.AssignmentID != queued.AssignmentID || armed.Attempt != queued.Binding.Attempt || armed.Retry != round {
			t.Fatalf("round %d: retry content = %+v, want this assignment with retry %d", round, armed, round)
		}
		delays = append(delays, wake.notBefore.Sub(wake.at))
		content = wake.content
	}
	for i := 1; i < len(delays); i++ {
		if delays[i] <= delays[i-1] {
			t.Fatalf("retry backoff does not grow: %v", delays)
		}
	}
	if delays[0] < 8*time.Second || delays[0] > 12*time.Second {
		t.Fatalf("first retry delay = %v, want about 10s", delays[0])
	}

	if live := f.get(0); live.Disposition != types.EngineeringAssignmentBound || live.CapsuleDisposition != types.EngineeringCapsuleActive {
		t.Fatalf("retrying touched the live assignment: %s/%s", live.Disposition, live.CapsuleDisposition)
	}
	if queued := f.get(1); queued.Disposition != types.EngineeringAssignmentOpen {
		t.Fatalf("the waiting request did not stay open: %s", queued.Disposition)
	}
}

// A request that waits past the assignment deadline expires with a visible
// reason and arms nothing ("assignment deadlines fail rather than hang").
func TestEngineeringAdmissionQueuedRequestExpiresAtItsDeadline(t *testing.T) {
	t.Setenv("CHOIR_ASSIGNMENT_DEADLINE", "1ms")
	f := newAdmissionFixture(t, 2)
	wakes := captureAdmissionWakes(f.rt)
	f.open(0)
	f.bind(0)
	queued := f.open(1)
	time.Sleep(20 * time.Millisecond)

	content := fmt.Sprintf(`{"assignment_id":%q,"attempt":%d}`, queued.AssignmentID, queued.Binding.Attempt)
	if err := f.rt.HandleDelegatedAssignmentSpawnDeadline(context.Background(), f.seed.OwnerID, f.computer, f.seed.ParentAgentID, content); err != nil {
		t.Fatal(err)
	}
	expired := f.get(1)
	if expired.Disposition != types.EngineeringAssignmentCancelled || !strings.Contains(expired.DispositionReason, "deadline") {
		t.Fatalf("expired request: %s (%q), want cancelled with a deadline reason", expired.Disposition, expired.DispositionReason)
	}
	if len(*wakes) != 0 {
		t.Fatalf("an expired request armed %d wakes", len(*wakes))
	}
	if live := f.get(0); live.Disposition != types.EngineeringAssignmentBound {
		t.Fatalf("expiring the queued request touched the live one: %s", live.Disposition)
	}
}

// With the slot free the wake proceeds into the spawn saga (here it stops at
// the missing capsule executor, which is past admission) and arms no retry.
func TestEngineeringAdmissionFreeSlotProceedsToSpawn(t *testing.T) {
	f := newAdmissionFixture(t, 1)
	wakes := captureAdmissionWakes(f.rt)
	only := f.open(0)

	content := fmt.Sprintf(`{"assignment_id":%q,"attempt":%d}`, only.AssignmentID, only.Binding.Attempt)
	err := f.rt.HandleDelegatedAssignmentSpawnDeadline(context.Background(), f.seed.OwnerID, f.computer, f.seed.ParentAgentID, content)
	if err == nil || !strings.Contains(err.Error(), "capsule authority unavailable") {
		t.Fatalf("a free slot must enter the saga (which stops at the absent executor here), got: %v", err)
	}
	if len(*wakes) != 0 {
		t.Fatalf("a free slot armed %d retry wakes", len(*wakes))
	}
}
