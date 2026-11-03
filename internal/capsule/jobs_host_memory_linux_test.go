//go:build linux

package capsule

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsulejob"
)

// Build jobs wait for host memory (S4 point 3). Failure modes pinned:
//   - W1 a build job starts while the host is short, or waits silently
//     (no state, no reason);
//   - W2 a light capsule's job, a protected computer's job, or a job with
//     stale facts waits;
//   - W3 a waiting job never ends: its stop, its capsule's end, or the
//     maximum wait leaves it open, or the desk is never told;
//   - W4 a cleared wait does not start the job;
//   - W5 a guest restart resumes a waiting job (the restart rule).

func newWaitFixture(t *testing.T, facts HostMemoryFacts, fresh bool, class SizeClass) (*jobFixture, *fakeHostMemory, *sync.Mutex) {
	t.Helper()
	f := newJobFixture(t, capsulejob.StateRunning, 0, "")
	delete(f.executor.jobs, "job-build")
	f.caps.Spec.SizeClass = class
	// A broker that refuses: a job that reaches its launch fails there.
	f.caps.broker = NewBrokerClient(t.TempDir()+"/no-broker.sock", nil)
	hm := &fakeHostMemory{facts: facts, fresh: fresh}
	f.executor.SetHostMemory(hm)
	var mu sync.Mutex
	f.executor.SetJobCompletionHandler(func(c JobCompletion) {
		mu.Lock()
		f.got = append(f.got, c)
		mu.Unlock()
	})
	oldPoll, oldMax := hostMemoryWaitPoll, hostMemoryMaxWait
	hostMemoryWaitPoll = 10 * time.Millisecond
	t.Cleanup(func() { hostMemoryWaitPoll, hostMemoryMaxWait = oldPoll, oldMax })
	return f, hm, &mu
}

func startWaitJob(t *testing.T, f *jobFixture, id string) string {
	t.Helper()
	view, err := f.executor.StartJob(context.Background(), "run-job", "handle-job", JobRequest{ID: id, Argv: []string{"npm", "run", "build"}})
	if err != nil {
		return "error: " + err.Error()
	}
	return view.State + ": " + view.Reason
}

func waitCompletion(t *testing.T, f *jobFixture, mu *sync.Mutex) JobCompletion {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		if len(f.got) > 0 {
			c := f.got[0]
			mu.Unlock()
			return c
		}
		mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("W3: the waiting job never ended")
	return JobCompletion{}
}

func TestBuildJobWaitsForHostMemoryVisiblyAndStops(t *testing.T) {
	f, _, mu := newWaitFixture(t, HostMemoryFacts{HeadroomBytes: 1 << 30}, true, SizeClassBuild)
	got := startWaitJob(t, f, "job-wait")
	if !strings.HasPrefix(got, "waiting: waiting for host memory") {
		t.Fatalf("W1: %s", got)
	}
	if _, _, running := f.executor.RunningJob("run-job"); !running {
		t.Fatal("a waiting job does not hold its capsule's one job slot")
	}
	view, err := f.executor.StopJob(context.Background(), "run-job", "handle-job", "job-wait")
	if err != nil || view.State != capsulejob.StateStopped {
		t.Fatalf("W3: stop a waiting job: %+v %v", view, err)
	}
	if c := waitCompletion(t, f, mu); c.Status.State != capsulejob.StateStopped || c.Receipt != nil {
		t.Fatalf("W3: completion %+v", c)
	}
	// W5: a waiting record left by a crash is closed at the next boot.
	f2, _, _ := newWaitFixture(t, HostMemoryFacts{PressureHigh: true}, true, SizeClassBuild)
	if got := startWaitJob(t, f2, "job-crash"); !strings.HasPrefix(got, "waiting") {
		t.Fatalf("W1 pressure: %s", got)
	}
	if closed := f2.executor.closeInterruptedJobs(); len(closed) != 1 || closed[0] != "job-crash" {
		t.Fatalf("W5: closed %v", closed)
	}
}

func TestOnlyBatchBuildJobsWithFreshFactsWait(t *testing.T) {
	for _, tc := range []struct {
		name  string
		facts HostMemoryFacts
		fresh bool
		class SizeClass
	}{
		{"light", HostMemoryFacts{PressureHigh: true}, true, SizeClassLight},
		{"protected", HostMemoryFacts{PressureHigh: true, Protected: true}, true, SizeClassBuild},
		{"stale", HostMemoryFacts{PressureHigh: true}, false, SizeClassBuild},
		{"room", HostMemoryFacts{HeadroomBytes: 8 << 30}, true, SizeClassBuild},
	} {
		f, _, _ := newWaitFixture(t, tc.facts, tc.fresh, tc.class)
		// No broker: a job that does not wait fails at its start.
		if got := startWaitJob(t, f, "job-"+tc.name); strings.HasPrefix(got, "waiting") {
			t.Fatalf("W2 %s: %s", tc.name, got)
		}
	}
}

func TestWaitingJobEndsWithItsCapsuleItsMaxWaitOrStartsWhenClear(t *testing.T) {
	f, _, mu := newWaitFixture(t, HostMemoryFacts{PressureHigh: true}, true, SizeClassBuild)
	startWaitJob(t, f, "job-cap")
	f.executor.mu.Lock()
	delete(f.executor.capsules, f.caps.ID)
	f.executor.mu.Unlock()
	if c := waitCompletion(t, f, mu); c.Status.State != capsulejob.StateStopped || !strings.Contains(c.Status.Reason, "capsule ended") {
		t.Fatalf("W3 capsule end: %+v", c.Status)
	}

	f, _, mu = newWaitFixture(t, HostMemoryFacts{PressureHigh: true}, true, SizeClassBuild)
	hostMemoryMaxWait = 50 * time.Millisecond
	startWaitJob(t, f, "job-max")
	if c := waitCompletion(t, f, mu); c.Status.State != capsulejob.StateStopped || !strings.Contains(c.Status.Reason, "stayed short") {
		t.Fatalf("W3 max wait: %+v", c.Status)
	}

	f, hm, mu := newWaitFixture(t, HostMemoryFacts{PressureHigh: true}, true, SizeClassBuild)
	startWaitJob(t, f, "job-clear")
	hm.mu.Lock()
	hm.facts = HostMemoryFacts{HeadroomBytes: 8 << 30}
	hm.mu.Unlock()
	// W4: the cleared wait reaches the launch (no broker here, so the start
	// fails, and says it was after the wait).
	if c := waitCompletion(t, f, mu); c.Status.State != capsulejob.StateFailed || !strings.Contains(c.Status.Reason, "after waiting for host memory") {
		t.Fatalf("W4: %+v", c.Status)
	}
}

// The wait text names what this computer's own jobs hold, so a desk can
// stop one (choir.Stop) instead of parking on a wait it caused.
func TestJobHoldersNoteNamesRunningJobsNotWaitingOnes(t *testing.T) {
	now := time.Now()
	run := func(id, cmd string, mib int, age time.Duration) *capsuleJob {
		return &capsuleJob{capsuleID: "cap-" + id, done: make(chan struct{}),
			spec:   capsulejob.Spec{ID: id, Argv: []string{"node", cmd, "--long-argument-that-is-cut"}, MemoryMiB: mib},
			status: capsulejob.Status{StartedAt: now.Add(-age)}}
	}
	self := run("job-self", "self.mjs", 0, 0)
	self.waiting = true
	waiting := run("job-queued", "queued.mjs", 0, time.Minute)
	waiting.waiting = true
	ended := run("job-ended", "ended.mjs", 0, time.Minute)
	close(ended.done)
	jobs := map[string]*capsuleJob{"job-self": self, "job-queued": waiting, "job-ended": ended,
		"job-shot": run("job-shot", "scripts/choir-shot.mjs", 1500, 7*time.Minute)}
	note := jobHoldersNote(jobs, self, now)
	for _, want := range []string{"job-shot", "scripts/choir-shot.mjs", "1500 MiB", "7m", "choir.Stop"} {
		if !strings.Contains(note, want) {
			t.Fatalf("holders note lacks %q: %s", want, note)
		}
	}
	for _, bad := range []string{"job-self", "job-queued", "job-ended"} {
		if strings.Contains(note, bad) {
			t.Fatalf("holders note names %s: %s", bad, note)
		}
	}
	if got := jobHoldersNote(map[string]*capsuleJob{"job-self": self}, self, now); got != "" {
		t.Fatalf("no holders should be empty, got %q", got)
	}
}

// A job that declares memory_mib waits for that much headroom, not for a
// whole build's peak; a job that declares none keeps the build claim.
func TestJobWaitsForItsDeclaredMemoryNotTheBuildPeak(t *testing.T) {
	headroom := HostMemoryFacts{HeadroomBytes: 2310 << 20}
	for _, tc := range []struct {
		name      string
		memoryMiB int
		wait      bool
	}{
		{"declared 1536 starts in 2310 MiB", 1536, false},
		{"declared none waits for the build peak", 0, true},
		{"declared above the peak (4096) is held to the peak and waits", 4096, true},
	} {
		f, _, _ := newWaitFixture(t, headroom, true, SizeClassBuild)
		view, err := f.executor.StartJob(context.Background(), "run-job", "handle-job", JobRequest{ID: "job-x", Argv: []string{"node", "choir-shot.mjs"}, MemoryMiB: tc.memoryMiB})
		got := "error"
		if err == nil {
			got = view.State + ": " + view.Reason
		}
		if waiting := strings.HasPrefix(got, "waiting"); waiting != tc.wait {
			t.Fatalf("%s: wait=%v, got %s (%v)", tc.name, waiting, got, err)
		}
	}
	if got := JobHostNeedBytes(1536); got != 1536<<20 {
		t.Fatalf("need for 1536 MiB: %d", got)
	}
	if got := JobHostNeedBytes(0); got != BuildPeakBytes {
		t.Fatalf("need for none: %d", got)
	}
}
