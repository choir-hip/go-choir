package agentcore

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// TestSlotStarterFreesSlotsWithNoLiveRun: the paper, 2026-11-08, showed
// four running task rows, no live task run and 1759 queued tasks for two
// days. A slot is held only while its task's run is live or its claim is
// fresh; a claim whose run never started, a bound run with no record and a
// run a crash closed free the slot and their rows close. A fresh claim still holds its slot,
// so the starter never runs more than K.
func TestSlotStarterFreesSlotsWithNoLiveRun(t *testing.T) {
	f := newDelegationFixture(t, "owner-task-heal")
	ctx := context.Background()
	f.rt.cfg.ManagementTaskSlots = 2
	f.model.mu.Lock()
	f.model.gate = make(chan struct{})
	gate := f.model.gate
	f.model.mu.Unlock()
	defer close(gate)
	for i := 1; i <= 6; i++ {
		seedArrival(t, f.s, f.owner, f.computer, fmt.Sprintf("rss:h%d", i), []types.CaptureArrival{
			{CaptureID: fmt.Sprintf("cap-h%d", i), Title: fmt.Sprintf("Listing %d", i), URL: fmt.Sprintf("https://h.example/%d", i)}})
	}
	f.parentTurn()
	queued := f.tasks(types.ManagementTaskQueued)
	if len(queued) != 6 {
		t.Fatalf("queued = %d tasks, want 6", len(queued))
	}
	tctx := withKindCustodian(ctx, "choir.management_task")
	stale, missing, crashed, fresh := queued[0], queued[1], queued[2], queued[3]
	// A claim from a process that died before its run was bound.
	if _, err := f.s.ClaimManagementTask(tctx, f.owner, f.computer, stale.TaskID, time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	// A bound run whose record is gone.
	if _, err := f.s.ClaimManagementTask(tctx, f.owner, f.computer, missing.TaskID, time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.BindManagementTaskRun(tctx, f.owner, f.computer, missing.TaskID, "run-gone"); err != nil {
		t.Fatal(err)
	}
	// A bound run a crash closed.
	if _, err := f.s.ClaimManagementTask(tctx, f.owner, f.computer, crashed.TaskID, time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	crashRun := types.RunRecord{RunID: "run-crashed", OwnerID: f.owner, ComputerID: f.computer, State: types.RunCancelled, Error: runFateInterruptedByRestart, CreatedAt: time.Now().Add(-time.Hour), UpdatedAt: time.Now()}
	if err := f.s.CreateRun(ctx, crashRun); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.BindManagementTaskRun(tctx, f.owner, f.computer, crashed.TaskID, crashRun.RunID); err != nil {
		t.Fatal(err)
	}
	// A claim whose run is starting now.
	if _, err := f.s.ClaimManagementTask(tctx, f.owner, f.computer, fresh.TaskID, time.Now()); err != nil {
		t.Fatal(err)
	}

	rec, err := f.rt.startQueuedManagementTasks(ctx, f.owner, f.computer)
	if err != nil || rec == nil {
		t.Fatalf("starter with two dead slots started %v (%v), want a run", rec, err)
	}
	for id, reason := range map[string]string{
		stale.TaskID:   types.TaskClosureCancelled,
		missing.TaskID: types.TaskClosureCancelled,
		crashed.TaskID: types.ClosureReasonInterruptedByRestart,
	} {
		if got := f.task(id); got.State != types.ManagementTaskClosed || got.ClosureReason != reason {
			t.Fatalf("task %s with no live run is %s (%q), want closed (%q)", id, got.State, got.ClosureReason, reason)
		}
	}
	// The fresh claim and the new run hold both slots.
	if got := f.task(fresh.TaskID); got.State != types.ManagementTaskRunning {
		t.Fatalf("fresh claim is %s, want running", got.State)
	}
	if left := f.tasks(types.ManagementTaskQueued); len(left) != 1 {
		t.Fatalf("queued after the starter = %d, want 1 (K = 2: the fresh claim and one new run)", len(left))
	}
	if again, err := f.rt.startQueuedManagementTasks(ctx, f.owner, f.computer); err != nil || again != nil {
		t.Fatalf("starter with K slots held started %v (%v), want none", again, err)
	}
}
