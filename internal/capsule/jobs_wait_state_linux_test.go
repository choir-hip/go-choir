//go:build linux

package capsule

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsulejob"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The typed wait reaches the job's view and the runtime's handler
// (docs/problems/selfdev-waits-untyped-and-invisible-2026-11-04.md).
// Failure modes pinned:
//   - T1 a waiting job's view carries only free text and vmctl's raw data
//     (no typed Wait record rendered from that data);
//   - T2 the runtime is not told the wait began, or never told it ended
//     (its tally would run forever);
//   - T3 a pure waiter (choir-delegate wait --park) leases a build's
//     memory and waits for it; one that declares memory_mib does not get
//     past the lease.

func TestWaitingJobCarriesItsTypedWaitAndTellsTheRuntime(t *testing.T) {
	f, _, _ := newWaitFixture(t, hostRefusal(), SizeClassBuild)
	var mu sync.Mutex
	var events []JobWaitEvent
	f.executor.SetJobWaitHandler(func(ev JobWaitEvent) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	})
	view, err := f.executor.StartJob(context.Background(), "run-job", "handle-job", JobRequest{ID: "job-typed", Argv: []string{"npm", "run", "build"}})
	if err != nil || view.State != capsulejob.StateWaiting {
		t.Fatalf("start: %+v %v", view, err)
	}
	if view.Wait == nil || view.Wait.Kind != types.WaitKindHostMemory || view.Wait.NeedBytes != 3<<30 || view.Wait.HaveBytes != 1<<30 {
		t.Fatalf("T1: view wait = %+v", view.Wait)
	}
	if !strings.Contains(view.Wait.Sentence, "another computer") || view.Wait.Until.IsZero() {
		t.Fatalf("T1: sentence/bound = %q %v", view.Wait.Sentence, view.Wait.Until)
	}
	if _, err := f.executor.StopJob(context.Background(), "run-job", "handle-job", "job-typed"); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(events) < 2 || events[0].Ended || events[0].Wait == nil {
		t.Fatalf("T2: events = %+v", events)
	}
	last := events[len(events)-1]
	if !last.Ended || last.Outcome != capsulejob.StateStopped || last.JobID != "job-typed" {
		t.Fatalf("T2: the wait's end = %+v", last)
	}
	for _, ev := range events[:len(events)-1] {
		if ev.Ended {
			t.Fatalf("T2: one end only: %+v", events)
		}
	}
}

func TestPureWaiterJobNeverWaitsForHostMemory(t *testing.T) {
	f, _, _ := newWaitFixture(t, hostRefusal(), SizeClassBuild)
	// Control: in the same fixture a build job waits.
	if got := startWaitJob(t, f, "job-build-control"); !strings.HasPrefix(got, "waiting") {
		t.Fatalf("control: a build job did not wait: %s", got)
	}
	f2, hm, _ := newWaitFixture(t, hostRefusal(), SizeClassBuild)
	view, err := f2.executor.StartJob(context.Background(), "run-job", "handle-job",
		JobRequest{ID: "job-park", Argv: []string{"choir-delegate", "wait", "--id", "d1", "--park"}})
	// No broker in the fixture: a job that does not wait fails at launch.
	if err == nil || view.State == capsulejob.StateWaiting {
		t.Fatalf("T3: a pure waiter must go straight to its launch: %+v %v", view, err)
	}
	hm.mu.Lock()
	asks := hm.jobAsks
	hm.mu.Unlock()
	if asks != 0 {
		t.Fatalf("T3: a pure waiter asked for a lease (%d asks)", asks)
	}
	f3, _, _ := newWaitFixture(t, hostRefusal(), SizeClassBuild)
	view, err = f3.executor.StartJob(context.Background(), "run-job", "handle-job",
		JobRequest{ID: "job-park-mem", Argv: []string{"sleep", "60"}, MemoryMiB: 1024})
	if err != nil || view.State != capsulejob.StateWaiting {
		t.Fatalf("T3: a declared claim must lease like any job: %+v %v", view, err)
	}
}
