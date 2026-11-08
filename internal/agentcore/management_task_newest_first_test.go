package agentcore

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// TestQueuedTasksStartNewestFirst: on 2026-11-08 the paper's queue held 1774
// tasks, the oldest 50 hours old, and captures arrived faster than four
// slots drained them, so oldest-first started only two-day-old news while
// that hour's captures waited. The point of the paper is to publish sourced
// stories as soon as possible: a free slot takes the newest queued task, in
// the starter and in the durable start a closure mints. Old tasks still run
// when nothing newer waits.
func TestQueuedTasksStartNewestFirst(t *testing.T) {
	f := newDelegationFixture(t, "owner-task-newest")
	ctx := context.Background()
	f.rt.cfg.ManagementTaskSlots = 1
	f.model.mu.Lock()
	f.model.gate = make(chan struct{})
	gate := f.model.gate
	f.model.mu.Unlock()
	defer close(gate)
	for i := 1; i <= 3; i++ {
		seedArrival(t, f.s, f.owner, f.computer, fmt.Sprintf("rss:n%d", i), []types.CaptureArrival{
			{CaptureID: fmt.Sprintf("cap-n%d", i), Title: fmt.Sprintf("Listing %d", i), URL: fmt.Sprintf("https://n.example/%d", i)}})
		f.parentTurn()
		time.Sleep(5 * time.Millisecond)
	}
	queued := f.tasks(types.ManagementTaskQueued)
	if len(queued) != 3 {
		t.Fatalf("queued = %d tasks, want 3", len(queued))
	}
	oldest, middle, newest := queued[0], queued[1], queued[2]
	if !oldest.CreatedAt.Before(newest.CreatedAt) {
		t.Fatalf("fixture: tasks share a creation time: %v %v", oldest.CreatedAt, newest.CreatedAt)
	}
	setup, err := f.s.ListUnprojectedActorWakes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range setup {
		_ = f.s.MarkActorWakeProjected(ctx, w.CanonicalID)
	}

	rec, err := f.rt.startQueuedManagementTasks(ctx, f.owner, f.computer)
	if err != nil || rec == nil {
		t.Fatalf("starter: %v, %v", rec, err)
	}
	if got := f.task(newest.TaskID); got.State != types.ManagementTaskRunning {
		t.Fatalf("newest task is %s, want running (oldest is %s)", got.State, f.task(oldest.TaskID).State)
	}

	// The closure's durable start names the newest task still queued.
	f.rt.closeManagementTask(ctx, f.task(newest.TaskID), types.TaskClosureCancelled, "", "test closes it", "")
	wakes, err := f.s.ListUnprojectedActorWakes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var starts []string
	for _, w := range wakes {
		if w.Kind == store.InboxRecordWakeKind && (w.TargetAgentID == middle.TaskInbox || w.TargetAgentID == oldest.TaskInbox) {
			starts = append(starts, w.TargetAgentID)
		}
	}
	if len(starts) != 1 || starts[0] != middle.TaskInbox {
		t.Fatalf("closure minted starts %v, want only the newest queued %s", starts, middle.TaskInbox)
	}
}
