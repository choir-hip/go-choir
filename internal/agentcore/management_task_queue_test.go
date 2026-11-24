package agentcore

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Failure list (docs/problems/captures-dropped-and-stalled-2026-11-24.md; the
// paper lost 1,352 captures on 2026-11-19 and 11-20):
// Q1 a free slot takes the newest queued task, so under load the oldest starve;
// Q2 the durable start a closure mints names the newest, not the oldest;
// Q3 a task that waited a day is closed expired_in_queue with its captures undecided;
// Q4 a task that waited a day never starts even when a slot is free;
// Q5 nothing says how deep the queue is or how old its oldest task is, so a starve is seen a day late.

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
		time.Sleep(5 * time.Millisecond)
	}
	var out []types.ManagementTask
	for _, task := range f.tasks(types.ManagementTaskQueued) {
		if !before[task.TaskID] {
			out = append(out, task)
		}
	}
	return out
}

// queueFixture queues n tasks behind one slot whose model call is held.
func queueFixture(t *testing.T, owner string, n int) (*delegationFixture, []types.ManagementTask, chan struct{}) {
	f := newDelegationFixture(t, owner)
	f.rt.cfg.ManagementTaskSlots = 1
	f.model.mu.Lock()
	f.model.gate = make(chan struct{})
	gate := f.model.gate
	f.model.mu.Unlock()
	queued := seedQueuedTasks(t, f, "q", n)
	if len(queued) != n || !queued[0].CreatedAt.Before(queued[n-1].CreatedAt) {
		t.Fatalf("fixture: %d tasks queued, want %d in creation order", len(queued), n)
	}
	setup, err := f.s.ListUnprojectedActorWakes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range setup {
		_ = f.s.MarkActorWakeProjected(context.Background(), w.CanonicalID)
	}
	return f, queued, gate
}

// TestQueuedTasksStartOldestFirst (Q1, Q2): a free slot takes the oldest
// queued task, in the starter and in the durable start a closure mints.
func TestQueuedTasksStartOldestFirst(t *testing.T) {
	f, queued, gate := queueFixture(t, "owner-task-oldest", 3)
	defer close(gate)
	ctx := context.Background()
	oldest, middle, newest := queued[0], queued[1], queued[2]

	rec, err := f.rt.startQueuedManagementTasks(ctx, f.owner, f.computer)
	if err != nil || rec == nil {
		t.Fatalf("starter: %v, %v", rec, err)
	}
	if got := f.task(oldest.TaskID); got.State != types.ManagementTaskRunning {
		t.Fatalf("oldest task is %s, want running (newest is %s)", got.State, f.task(newest.TaskID).State)
	}
	f.rt.closeManagementTask(ctx, f.task(oldest.TaskID), types.TaskClosureCancelled, "", "test closes it", "")
	wakes, err := f.s.ListUnprojectedActorWakes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var starts []string
	for _, w := range wakes {
		if w.Kind == store.InboxRecordWakeKind && (w.TargetAgentID == middle.TaskInbox || w.TargetAgentID == newest.TaskInbox) {
			starts = append(starts, w.TargetAgentID)
		}
	}
	if len(starts) != 1 || starts[0] != middle.TaskInbox {
		t.Fatalf("closure minted starts %v, want only the oldest queued %s", starts, middle.TaskInbox)
	}
}

// TestAQueuedTaskNeverExpires (Q3, Q4): two days on, no queued task is
// closed, every capture is still on a queued or running task, and the
// oldest takes the free slot.
func TestAQueuedTaskNeverExpires(t *testing.T) {
	f, queued, gate := queueFixture(t, "owner-task-no-expiry", 3)
	defer close(gate)
	ctx := context.Background()
	f.rt.managementTaskNow = func() time.Time { return time.Now().Add(48 * time.Hour) }

	for pass := 0; pass < 2; pass++ { // a second wake finds every slot busy
		if _, err := f.rt.startQueuedManagementTasks(ctx, f.owner, f.computer); err != nil {
			t.Fatal(err)
		}
	}
	for i, q := range queued {
		got := f.task(q.TaskID)
		want := types.ManagementTaskQueued
		if i == 0 {
			want = types.ManagementTaskRunning
		}
		if got.State != want {
			t.Fatalf("task %d, two days old, is %s (%s), want %s", i, got.State, got.ClosureReason, want)
		}
	}
}

// TestTheQueueIsLoggedOnceAMinute (Q5): the starter says how many tasks
// wait and how old the oldest is, with every slot busy too, at most once a
// minute.
func TestTheQueueIsLoggedOnceAMinute(t *testing.T) {
	f, _, gate := queueFixture(t, "owner-task-queue-log", 3)
	defer close(gate)
	ctx := context.Background()
	at := time.Now().Add(2 * time.Hour)
	f.rt.managementTaskNow = func() time.Time { return at }
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)
	line := regexp.MustCompile(`runtime: management task queue: queued=(\d+) oldest_age_s=(\d+) running=(\d+) slots=(\d+)`)

	if _, err := f.rt.startQueuedManagementTasks(ctx, f.owner, f.computer); err != nil {
		t.Fatal(err)
	}
	first := line.FindAllStringSubmatch(buf.String(), -1)
	if len(first) != 1 {
		t.Fatalf("queue lines after the first wake = %d, want 1:\n%s", len(first), buf.String())
	}
	// Read before the pass started one: three waited, the oldest two hours.
	age, _ := strconv.Atoi(first[0][2])
	if first[0][1] != "3" || age < 7190 || age > 7260 || first[0][3] != "0" || first[0][4] != "1" {
		t.Fatalf("first line = %v, want queued=3 oldest_age_s about 7200 running=0 slots=1", first[0])
	}
	if _, err := f.rt.startQueuedManagementTasks(ctx, f.owner, f.computer); err != nil {
		t.Fatal(err)
	}
	if n := len(line.FindAllString(buf.String(), -1)); n != 1 {
		t.Fatalf("a wake inside the minute logged again: %d lines", n)
	}
	at = at.Add(61 * time.Second)
	if _, err := f.rt.startQueuedManagementTasks(ctx, f.owner, f.computer); err != nil {
		t.Fatal(err)
	}
	all := line.FindAllStringSubmatch(buf.String(), -1)
	if len(all) != 2 || all[1][1] != "2" || all[1][3] != "1" {
		t.Fatalf("a minute on, with the slot busy: %v, want a second line with queued=2 running=1", all)
	}
}
