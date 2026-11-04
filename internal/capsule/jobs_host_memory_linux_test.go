//go:build linux

package capsule

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsulejob"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Build jobs lease host memory when they start
// (docs/problems/build-capsule-holds-peak-while-agent-thinks-2026-11-04.md).
// Failure modes pinned:
//   - W1 a job the host refuses starts, or waits silently (no state, no
//     reason, no wait data naming the holder);
//   - W2 a light capsule's job asks for a lease, or a host that cannot be
//     asked keeps a job from starting;
//   - W3 a waiting job never ends: its stop, its capsule's end, or the
//     maximum wait leaves it open, or the desk is never told;
//   - W4 a refusal that clears does not start the job;
//   - W5 a guest restart resumes a waiting job (the restart rule);
//   - W6 a lease outlives its job (an ended job, a failed start, a job
//     stopped while it waits);
//   - W7 a lease asks for more than its capsule's cgroup can grow, or the
//     job can grow past its lease (the cgroup is not held to it), or the
//     hold outlives the job.

func hostRefusal() *HostMemoryRefusal {
	return &HostMemoryRefusal{Reason: "host memory available 9216 MiB; this job needs 3072 MiB",
		Wait: &yaegikernel.MemoryWait{NeedBytes: 3 << 30, RoomBytes: 1 << 30,
			Holders: []yaegikernel.MemoryHolder{{Kind: "job", Class: "test", Bytes: 3 << 30}}}}
}

func newWaitFixture(t *testing.T, refusal *HostMemoryRefusal, class SizeClass) (*jobFixture, *fakeHostMemory, *sync.Mutex) {
	t.Helper()
	f := newJobFixture(t, capsulejob.StateRunning, 0, "")
	delete(f.executor.jobs, "job-build")
	f.caps.Spec.SizeClass = class
	// A broker that refuses: a job that reaches its launch fails there.
	f.caps.broker = NewBrokerClient(t.TempDir()+"/no-broker.sock", nil)
	hm := &fakeHostMemory{refuseJobs: refusal}
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

// leaseCgroup is a capsule cgroup a test reads: its usage, and every
// memory.max written.
type leaseCgroup struct {
	mu    sync.Mutex
	usage int64
	maxes []int64
}

func (c *leaseCgroup) Current() (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.usage, nil
}
func (c *leaseCgroup) Reclaim(int64) error { return nil }
func (c *leaseCgroup) SetMax(b int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.maxes = append(c.maxes, b)
	return nil
}
func (c *leaseCgroup) written() []int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]int64(nil), c.maxes...)
}

func TestRefusedBuildJobWaitsVisiblyNamingTheHolderAndStops(t *testing.T) {
	f, _, mu := newWaitFixture(t, hostRefusal(), SizeClassBuild)
	view, err := f.executor.StartJob(context.Background(), "run-job", "handle-job", JobRequest{ID: "job-wait", Argv: []string{"npm", "run", "build"}})
	if err != nil || view.State != capsulejob.StateWaiting || !strings.HasPrefix(view.Reason, "waiting for host memory: host memory available 9216 MiB") {
		t.Fatalf("W1: %+v %v", view, err)
	}
	if w := view.MemoryWait; w == nil || w.NeedBytes != 3<<30 || len(w.Holders) != 1 || w.Holders[0].Kind != "job" {
		t.Fatalf("W1: wait data %+v", view.MemoryWait)
	}
	if _, _, running := f.executor.RunningJob("run-job"); !running {
		t.Fatal("a waiting job does not hold its capsule's one job slot")
	}
	view, err = f.executor.StopJob(context.Background(), "run-job", "handle-job", "job-wait")
	if err != nil || view.State != capsulejob.StateStopped || view.MemoryWait != nil {
		t.Fatalf("W3: stop a waiting job: %+v %v", view, err)
	}
	if c := waitCompletion(t, f, mu); c.Status.State != capsulejob.StateStopped || c.Receipt != nil {
		t.Fatalf("W3: completion %+v", c)
	}
	// W5: a waiting record left by a crash is closed at the next boot.
	f2, _, _ := newWaitFixture(t, hostRefusal(), SizeClassBuild)
	if got := startWaitJob(t, f2, "job-crash"); !strings.HasPrefix(got, "waiting") {
		t.Fatalf("W1: %s", got)
	}
	if closed := f2.executor.closeInterruptedJobs(); len(closed) != 1 || closed[0] != "job-crash" {
		t.Fatalf("W5: closed %v", closed)
	}
}

func TestOnlyRefusedBuildJobsWait(t *testing.T) {
	// A light capsule never asks; an admitted lease or a host that cannot
	// be asked starts the job (no broker here: it fails at its launch).
	f, hm, _ := newWaitFixture(t, hostRefusal(), SizeClassLight)
	if got := startWaitJob(t, f, "job-light"); strings.HasPrefix(got, "waiting") || hm.jobAsks != 0 {
		t.Fatalf("W2 light: %s (asks %d)", got, hm.jobAsks)
	}
	f, _, _ = newWaitFixture(t, nil, SizeClassBuild)
	if got := startWaitJob(t, f, "job-room"); strings.HasPrefix(got, "waiting") {
		t.Fatalf("W2 admitted: %s", got)
	}
	f, hm, _ = newWaitFixture(t, nil, SizeClassBuild)
	hm.jobErr = errors.New("vmctl unreachable")
	if got := startWaitJob(t, f, "job-unreached"); strings.HasPrefix(got, "waiting") {
		t.Fatalf("W2 unreachable: %s", got)
	}
}

func TestWaitingJobEndsWithItsCapsuleItsMaxWaitOrStartsWhenAdmitted(t *testing.T) {
	f, _, mu := newWaitFixture(t, hostRefusal(), SizeClassBuild)
	startWaitJob(t, f, "job-cap")
	f.executor.mu.Lock()
	delete(f.executor.capsules, f.caps.ID)
	f.executor.mu.Unlock()
	if c := waitCompletion(t, f, mu); c.Status.State != capsulejob.StateStopped || !strings.Contains(c.Status.Reason, "capsule ended") {
		t.Fatalf("W3 capsule end: %+v", c.Status)
	}

	f, _, mu = newWaitFixture(t, hostRefusal(), SizeClassBuild)
	hostMemoryMaxWait = 50 * time.Millisecond
	startWaitJob(t, f, "job-max")
	if c := waitCompletion(t, f, mu); c.Status.State != capsulejob.StateStopped || !strings.Contains(c.Status.Reason, "stayed short") {
		t.Fatalf("W3 max wait: %+v", c.Status)
	}
	hostMemoryMaxWait = 30 * time.Minute

	f, hm, mu := newWaitFixture(t, hostRefusal(), SizeClassBuild)
	startWaitJob(t, f, "job-clear")
	hm.setRefusal(nil)
	// W4: the admitted lease reaches the launch (no broker here, so the
	// start fails, says it was after the wait, and W6 gives the lease back).
	if c := waitCompletion(t, f, mu); c.Status.State != capsulejob.StateFailed || !strings.Contains(c.Status.Reason, "after waiting for host memory") {
		t.Fatalf("W4: %+v", c.Status)
	}
	if _, held := hm.lease(f.caps.ID + "/job-clear"); held {
		t.Fatalf("W6: a job that failed to start keeps its lease")
	}
}

func TestJobLeaseIsItsNeedHeldToItsCgroupAndGivenBackAtItsEnd(t *testing.T) {
	for _, tc := range []struct {
		name      string
		memoryMiB int
		max       int64
		usage     int64
		lease     int64
		hold      int64 // the memory.max held while it runs (0: none)
	}{
		// An undeclared build in a 4 GiB capsule at 200 MiB leases a build's
		// peak, and its cgroup is held to 200 MiB + 3 GiB.
		{"undeclared", 0, 4 << 30, 200 << 20, 3 << 30, 200<<20 + 3<<30},
		// In a 2 GiB capsule the cgroup can grow 1.8 GiB: that is the lease,
		// and the limit stays.
		{"bounded by the cgroup", 0, 2 << 30, 200 << 20, 2<<30 - 200<<20, 0},
		// A declared 1536 MiB shot leases 1536 MiB; its boost raises the
		// capsule to 2 GiB + 1.5 GiB, held to 200 MiB + 1.5 GiB.
		{"declared", 1536, 2 << 30, 200 << 20, 1536 << 20, 200<<20 + 1536<<20},
	} {
		f, hm, _ := newWaitFixture(t, nil, SizeClassBuild)
		f.caps.MemoryMax = tc.max
		cg := &leaseCgroup{usage: tc.usage}
		f.executor.cgroupMemoryFor = func(*Capsule) cgroupMemory { return cg }
		f.executor.vmMemoryTotal = 64 << 30 // room for the declared job's boost
		_, err := f.executor.StartJob(context.Background(), "run-job", "handle-job", JobRequest{ID: "job-x", Argv: []string{"npm", "run", "build"}, MemoryMiB: tc.memoryMiB})
		if err == nil {
			t.Fatalf("%s: the fixture's broker should refuse the start", tc.name)
		}
		hm.mu.Lock()
		asks, base, asked := hm.jobAsks, hm.baselines[f.caps.ID], hm.asked
		hm.mu.Unlock()
		if asks != 1 || base != tc.usage || asked[0] != tc.lease {
			t.Fatalf("%s: asks %d for %v MiB with baseline %d MiB, want one for %d MiB with the usage", tc.name, asks, mibs(asked), base>>20, tc.lease>>20)
		}
		// The start failed after the lease: W6, it was given back.
		if _, held := hm.lease(f.caps.ID + "/job-x"); held {
			t.Fatalf("W6 %s: lease kept after a failed start", tc.name)
		}
		maxes := cg.written()
		if tc.hold == 0 {
			for _, m := range maxes {
				if m < tc.max {
					t.Fatalf("W7 %s: memory.max lowered to %d MiB", tc.name, m>>20)
				}
			}
		} else if !containsMax(maxes, tc.hold) {
			t.Fatalf("W7 %s: memory.max writes %v MiB, want the hold %d MiB", tc.name, mibs(maxes), tc.hold>>20)
		}
		// And the hold ends with the job: the last write raises it back.
		if tc.hold != 0 && maxes[len(maxes)-1] <= tc.hold {
			t.Fatalf("W7 %s: memory.max left at %d MiB after the job", tc.name, maxes[len(maxes)-1]>>20)
		}
		hm.mu.Lock()
		freed := hm.jobsFreed
		hm.mu.Unlock()
		if len(freed) != 1 {
			t.Fatalf("W6 %s: released %v", tc.name, freed)
		}
	}
}

func TestJobLeaseBytesFollowTheNeedAndTheCgroupRoom(t *testing.T) {
	f, _, _ := newWaitFixture(t, nil, SizeClassBuild)
	f.caps.MemoryMax = 2 << 30
	cg := &leaseCgroup{usage: 200 << 20}
	f.executor.cgroupMemoryFor = func(*Capsule) cgroupMemory { return cg }
	for _, tc := range []struct {
		mib  int
		want int64
	}{{0, 2<<30 - 200<<20}, {1536, 1536 << 20}, {4096, 4096 << 20}} {
		job := &capsuleJob{spec: capsulejob.Spec{MemoryMiB: tc.mib}}
		if usage, lease := f.executor.jobLeaseFor(job, f.caps); usage != 200<<20 || lease != tc.want {
			t.Fatalf("W7 memory_mib %d: usage %d lease %d MiB, want %d", tc.mib, usage>>20, lease>>20, tc.want>>20)
		}
	}
	// No cgroup to read: the need, whole.
	f.executor.cgroupMemoryFor = func(*Capsule) cgroupMemory { return nil }
	if _, lease := f.executor.jobLeaseFor(&capsuleJob{}, f.caps); lease != BuildPeakBytes {
		t.Fatalf("no cgroup: lease %d MiB", lease>>20)
	}
}

// A job that ends gives its lease back, and its capsule's cgroup is raised
// from the hold.
func TestEndedJobReleasesItsLease(t *testing.T) {
	f := newJobFixture(t, capsulejob.StateDone, 0, "ok\n")
	hm := &fakeHostMemory{leases: map[string]int64{f.caps.ID + "/job-build": 3 << 30}}
	f.executor.SetHostMemory(hm)
	cg := &leaseCgroup{usage: 200 << 20}
	f.executor.cgroupMemoryFor = func(*Capsule) cgroupMemory { return cg }
	f.caps.MemoryMax = 4 << 30
	f.job.lease, f.job.leased, f.job.clamped = 3<<30, true, true
	f.executor.finishJob(f.job, f.caps, capsulejob.StateDone, "")
	if _, held := hm.lease(f.caps.ID + "/job-build"); held {
		t.Fatal("W6: an ended job keeps its lease")
	}
	if maxes := cg.written(); len(maxes) != 1 || maxes[0] != 4<<30 {
		t.Fatalf("W7: memory.max after the job %v MiB, want 4096", mibs(maxes))
	}
	// Idempotent: a second return gives back nothing more.
	f.executor.returnJobMemory(f.job)
	if len(hm.jobsFreed) != 1 {
		t.Fatalf("W6: released %v", hm.jobsFreed)
	}
}

// W3/W6: a stop during the synchronous ask ends the job; the ask's answer
// (admitted) never revives it, and the lease it got is given back.
func TestStopDuringTheLeaseAskEndsTheJobAndGivesTheLeaseBack(t *testing.T) {
	for _, refused := range []bool{false, true} {
		var refusal *HostMemoryRefusal
		if refused {
			refusal = hostRefusal()
		}
		f, hm, mu := newWaitFixture(t, refusal, SizeClassBuild)
		hm.gate = make(chan struct{})
		got := make(chan yaegikernel.JobView, 1)
		go func() {
			view, _ := f.executor.StartJob(context.Background(), "run-job", "handle-job", JobRequest{ID: "job-ask", Argv: []string{"npm", "run", "build"}})
			got <- view
		}()
		var stopped yaegikernel.JobView
		deadline := time.Now().Add(5 * time.Second)
		for {
			f.executor.mu.RLock()
			_, present := f.executor.jobs["job-ask"]
			f.executor.mu.RUnlock()
			if present {
				var err error
				stopped, err = f.executor.StopJob(context.Background(), "run-job", "handle-job", "job-ask")
				if err != nil {
					t.Fatalf("stop during the ask: %v", err)
				}
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("the job never registered")
			}
			time.Sleep(time.Millisecond)
		}
		close(hm.gate)
		view := <-got
		if stopped.State != capsulejob.StateStopped || view.State != capsulejob.StateStopped {
			t.Fatalf("refused=%t: stop view %s, start view %s; want both stopped", refused, stopped.State, view.State)
		}
		if c := waitCompletion(t, f, mu); c.Status.State != capsulejob.StateStopped {
			t.Fatalf("refused=%t: completion %+v", refused, c.Status)
		}
		time.Sleep(50 * time.Millisecond) // a revived waiter would poll and launch here
		mu.Lock()
		n := len(f.got)
		mu.Unlock()
		if n != 1 {
			t.Fatalf("refused=%t: %d completions; the stopped job was revived", refused, n)
		}
		if _, held := hm.lease(f.caps.ID + "/job-ask"); held {
			t.Fatalf("refused=%t: the stopped job keeps its lease", refused)
		}
	}
}

func containsMax(maxes []int64, want int64) bool {
	for _, m := range maxes {
		if m == want {
			return true
		}
	}
	return false
}

func mibs(b []int64) []int64 {
	out := make([]int64, len(b))
	for i, v := range b {
		out[i] = v >> 20
	}
	return out
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

// A shot job with a cached build is defaulted to 2048 MiB and leases that;
// the view says so. An uncached one keeps the claim and, refused, waits
// with the memory_mib hint.
func TestStartJobDefaultsCachedShotMemory(t *testing.T) {
	f, hm, _ := newWaitFixture(t, nil, SizeClassBuild)
	cached := shotCapsule(t, true)
	f.caps.MergedDir = cached.MergedDir
	argv := []string{"sh", "-c", "node /workspace/platform/scripts/choir-shot.mjs --shots /tmp/s.mjs"}
	view, _ := f.executor.StartJob(context.Background(), "run-job", "handle-job", JobRequest{ID: "job-cached", Argv: argv})
	if view.State == capsulejob.StateWaiting {
		t.Fatalf("an admitted shot waited: %+v", view)
	}
	if j := f.executor.jobs["job-cached"]; j != nil && j.spec.MemoryMiB != 2048 {
		t.Fatalf("spec memory %d", j.spec.MemoryMiB)
	}
	if hm.jobAsks != 1 {
		t.Fatalf("asks %d", hm.jobAsks)
	}

	f2, _, _ := newWaitFixture(t, hostRefusal(), SizeClassBuild)
	f2.caps.MergedDir = shotCapsule(t, false).MergedDir
	view, err := f2.executor.StartJob(context.Background(), "run-job", "handle-job", JobRequest{ID: "job-fresh", Argv: argv})
	if err != nil || view.State != capsulejob.StateWaiting || !strings.HasSuffix(view.Reason, memoryMiBHint) {
		t.Fatalf("uncached shot: %+v %v", view, err)
	}

	f3, _, _ := newWaitFixture(t, hostRefusal(), SizeClassBuild)
	f3.caps.MergedDir = cached.MergedDir
	view, _ = f3.executor.StartJob(context.Background(), "run-job", "handle-job", JobRequest{ID: "job-short", Argv: argv})
	if view.State != capsulejob.StateWaiting || view.MemoryMiB != 2048 || !strings.Contains(view.Reason, shotDefaultNote) || view.MemoryNote != shotDefaultNote {
		t.Fatalf("defaulted wait view: %+v", view)
	}
}
