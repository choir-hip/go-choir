package agentcore

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// seedQueuedTasks arrives n captures, one parent turn each, and returns the
// tasks that queued since.
func seedQueuedTasks(t *testing.T, f *delegationFixture, prefix string, n int) []types.ManagementTask {
	t.Helper()
	before := map[string]bool{}
	for _, task := range f.tasks(types.ManagementTaskQueued) {
		before[task.TaskID] = true
	}
	for i := 1; i <= n; i++ {
		seedArrival(t, f.s, f.owner, f.computer, fmt.Sprintf("rss:%s%d", prefix, i), []types.CaptureArrival{
			{CaptureID: fmt.Sprintf("cap-%s%d", prefix, i), Title: fmt.Sprintf("Listing %s%d", prefix, i), URL: fmt.Sprintf("https://x.example/%s%d", prefix, i)}})
		f.parentTurn()
		time.Sleep(3 * time.Millisecond)
	}
	var out []types.ManagementTask
	for _, task := range f.tasks(types.ManagementTaskQueued) {
		if !before[task.TaskID] {
			out = append(out, task)
		}
	}
	return out
}

// expireFixture seeds old then fresh queued tasks, marks the setup wakes
// projected, and sets the expiry clock so exactly the old ones are past a day.
func expireFixture(t *testing.T, owner string, old, fresh int) (*delegationFixture, []types.ManagementTask, []types.ManagementTask, chan struct{}) {
	f := newDelegationFixture(t, owner)
	f.rt.cfg.ManagementTaskSlots = 1
	f.model.mu.Lock()
	f.model.gate = make(chan struct{})
	gate := f.model.gate
	f.model.mu.Unlock()
	oldTasks := seedQueuedTasks(t, f, "old", old)
	time.Sleep(20 * time.Millisecond)
	freshTasks := seedQueuedTasks(t, f, "new", fresh)
	if len(oldTasks) != old || len(freshTasks) != fresh {
		t.Fatalf("fixture: %d old, %d fresh tasks queued", len(oldTasks), len(freshTasks))
	}
	lastOld := oldTasks[len(oldTasks)-1].CreatedAt
	f.rt.managementTaskNow = func() time.Time { return lastOld.Add(managementTaskQueueMaxAge + 5*time.Millisecond) }
	setup, err := f.s.ListUnprojectedActorWakes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range setup {
		_ = f.s.MarkActorWakeProjected(context.Background(), w.CanonicalID)
	}
	return f, oldTasks, freshTasks, gate
}

// TestStaleQueuedTasksExpire: the paper's queue held 1763 tasks, the oldest
// 70 hours old (2026-11-09). A queued task older than a day closes as
// expired_in_queue with its captures, wakes no one and starts nothing; the
// starter then runs the fresh work.
func TestStaleQueuedTasksExpire(t *testing.T) {
	f, old, fresh, gate := expireFixture(t, "owner-task-expire", 3, 2)
	defer close(gate)
	ctx := context.Background()

	rec, err := f.rt.startQueuedManagementTasks(ctx, f.owner, f.computer)
	if err != nil || rec == nil {
		t.Fatalf("starter: %v, %v", rec, err)
	}
	for _, o := range old {
		got := f.task(o.TaskID)
		if got.State != types.ManagementTaskClosed || got.ClosureReason != types.TaskClosureExpired {
			t.Fatalf("old task %s: %s/%s, want closed/expired_in_queue", o.TaskID, got.State, got.ClosureReason)
		}
		report := f.closingReport(got)
		if report.TaskOutcome == nil || len(report.TaskOutcome.Residuals) != len(o.LeasedCaptures) || report.TaskOutcome.Residuals[0].Reason != types.TaskClosureExpired {
			t.Fatalf("report outcome = %+v, want %d residuals with reason expired_in_queue", report.TaskOutcome, len(o.LeasedCaptures))
		}
		if report.Report == nil || len(report.Report.Claims) == 0 || !strings.Contains(report.Report.Claims[0].Text, o.LeasedCaptures[0]) {
			t.Fatalf("report summary does not list %s: %+v", o.LeasedCaptures[0], report.Report)
		}
		for _, c := range o.LeasedCaptures {
			ok, err := f.rt.captureDelegable(ctx, f.owner, f.computer, c)
			if err != nil || ok {
				t.Fatalf("expired capture %s delegable = %v, %v", c, ok, err)
			}
		}
	}
	running := 0
	for _, fr := range fresh {
		if f.task(fr.TaskID).State == types.ManagementTaskRunning {
			running++
		}
	}
	if running != 1 {
		t.Fatalf("fresh tasks running = %d, want 1 (one slot)", running)
	}
	// No wake to the parent for an expired close, and no closure-minted
	// start wake for any task.
	wakes, err := f.s.ListUnprojectedActorWakes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range wakes {
		if w.TargetAgentID == f.management() {
			t.Fatalf("expired close woke the parent: %+v", w)
		}
		if strings.Contains(w.Content, "start_after") {
			t.Fatalf("expired close minted a start wake: %+v", w)
		}
	}
}

// TestStaleQueuedTaskExpiryIsBatched: one wake closes at most a batch,
// oldest first, so a backlog does not hold the write pool; the next wake
// continues.
func TestStaleQueuedTaskExpiryIsBatched(t *testing.T) {
	f, old, _, gate := expireFixture(t, "owner-task-expire-batch", 3, 1)
	defer close(gate)
	ctx := context.Background()

	remaining := f.rt.expireStaleQueuedTasks(ctx, f.tasks(types.ManagementTaskQueued), 2)
	if len(remaining) != 2 { // the third old task and the fresh one
		t.Fatalf("remaining after a batch of 2 = %d, want 2", len(remaining))
	}
	for i, o := range old {
		want := types.ManagementTaskClosed
		if i == 2 {
			want = types.ManagementTaskQueued
		}
		if got := f.task(o.TaskID).State; got != want {
			t.Fatalf("old[%d] is %s after the first batch, want %s (oldest first)", i, got, want)
		}
	}
	remaining = f.rt.expireStaleQueuedTasks(ctx, f.tasks(types.ManagementTaskQueued), 2)
	if len(remaining) != 1 || f.task(old[2].TaskID).State != types.ManagementTaskClosed {
		t.Fatalf("the next wake did not finish the rest: %d remaining", len(remaining))
	}
}
