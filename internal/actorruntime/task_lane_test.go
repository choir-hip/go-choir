package actorruntime

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/actor"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Failure list (docs/problems/captures-dropped-and-stalled-2026-11-24.md):
// T1 a management task's run waits for a shared dispatcher slot while other
// desks hold them all, so the task holds a task slot with no running run;
// T2 the task lane is narrower than the most task slots a computer may have.

// TestTaskRunsStartWhileOtherDesksHoldEverySharedSlot (T1): with the
// dispatcher the adapter really runs, every shared slot held by another
// desk, each task inbox still activates.
func TestTaskRunsStartWhileOtherDesksHoldEverySharedSlot(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "actor.db")+"?_pragma=busy_timeout(60000)&_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	l, err := actor.NewSQLiteLog(db)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	now := time.Now().UTC()
	const desks = 40 // more than any shared cap: the shared slots are full whatever their number
	for i := 0; i < desks; i++ {
		to := scopedActorMailboxID("owner", "computer", fmt.Sprintf("research:r%d", i))
		if _, err := l.Append(ctx, actor.Update{UpdateID: fmt.Sprintf("r%d", i), ToAgentID: to, FromAgentID: "test", Kind: "k", Content: "c", CreatedAt: now.Add(-time.Hour)}); err != nil {
			t.Fatal(err)
		}
	}
	tasks := provideriface.MaxManagementTaskSlots
	for i := 0; i < tasks; i++ {
		to := scopedActorMailboxID("owner", "computer", fmt.Sprintf("%s:mt-%d", types.TaskInboxDesk, i))
		if _, err := l.Append(ctx, actor.Update{UpdateID: fmt.Sprintf("t%d", i), ToAgentID: to, FromAgentID: "test", Kind: "k", Content: "c", CreatedAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	release := make(chan struct{})
	var mu sync.Mutex
	startedTasks, startedDesks := 0, 0
	h := actor.HandlerFunc(func(ctx context.Context, agentID string, u actor.Update, mem []byte) ([]byte, error) {
		mu.Lock()
		if strings.Contains(agentID, types.TaskInboxDesk+":") {
			startedTasks++
		} else {
			startedDesks++
		}
		mu.Unlock()
		select {
		case <-release:
		case <-ctx.Done():
		}
		return mem, nil
	})
	opts := kernelDispatcherOptions("sink")
	opts.PollInterval = 10 * time.Millisecond
	d := actor.NewDispatcher(l, h, opts)
	go d.Run(ctx)
	defer d.Stop()
	defer close(release) // before Stop, which waits for the held activations
	deadline := time.Now().Add(10 * time.Second)
	for {
		mu.Lock()
		gotTasks, gotDesks := startedTasks, startedDesks
		mu.Unlock()
		if gotTasks == tasks {
			if gotDesks >= desks {
				t.Fatalf("fixture: all %d desks ran, so the shared slots were never full", desks)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d of %d task runs started while %d other desks held the shared slots", gotTasks, tasks, gotDesks)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestKernelDispatcherHasTheTaskLane (T2): reverting the lane, or making it
// narrower than the most task slots, fails here.
func TestKernelDispatcherHasTheTaskLane(t *testing.T) {
	for _, lane := range kernelDispatcherOptions("sink").Lanes {
		if lane.Prefix == types.TaskInboxDesk+":" {
			if lane.MaxConcurrent < provideriface.MaxManagementTaskSlots {
				t.Fatalf("task lane MaxConcurrent = %d, below the most task slots %d: a claimed task could wait", lane.MaxConcurrent, provideriface.MaxManagementTaskSlots)
			}
			return
		}
	}
	t.Fatalf("the dispatcher has no %q lane", types.TaskInboxDesk+":")
}
