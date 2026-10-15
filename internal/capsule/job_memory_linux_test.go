//go:build linux

package capsule

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsulejob"
)

// Job memory on the executor (docs/reports/memory-hotplug-controller-
// design-2026-10-14.md section 9.4): J3 a need that does not fit is
// refused, J6 the cgroup is lowered before the lease is released, J7 an
// unreclaimable boost is retained, J8 destroy returns it once, J12 spawns
// see the boost, F18 the budget is live.

const mib = int64(1 << 20)

type jobMemoryFixture struct {
	e        *Executor
	caps     *Capsule
	cg       *fakeCgroupMemory
	budget   int64
	releases []int64 // the cgroup's memory.max at each release
}

func newJobMemoryFixture(t *testing.T) *jobMemoryFixture {
	t.Helper()
	f := &jobMemoryFixture{budget: 3072 * mib, cg: &fakeCgroupMemory{current: 512 * mib, max: 2048 * mib}}
	f.caps = &Capsule{ID: "capsule-mem", State: StateActive, MemoryMax: 2048 * mib}
	f.e = &Executor{
		capsules:      map[string]*Capsule{f.caps.ID: f.caps},
		vmMemoryTotal: 1024 * mib, // the boot reading, superseded by the live one
		vmMemoryUsed:  2048 * mib,
		memoryTotalFn: func() int64 { return f.budget },
		cgroupMemoryFor: func(c *Capsule) cgroupMemory {
			if c != f.caps {
				t.Fatalf("cgroup asked for capsule %s", c.ID)
			}
			return f.cg
		},
	}
	return f
}

func (f *jobMemoryFixture) release() func() {
	return func() { f.releases = append(f.releases, f.cg.max) }
}

func TestJobMemoryBoostFollowsTheLiveBudget(t *testing.T) {
	f := newJobMemoryFixture(t)
	// 2048 used + 1536 asked > 3072: refused, nothing written or counted.
	if err := f.e.boostJobMemory(f.caps.ID, "job-a", 1536*mib, f.release()); err == nil || !strings.Contains(err.Error(), "budget") {
		t.Fatalf("over budget: %v", err)
	}
	if f.e.vmMemoryUsed != 2048*mib || f.cg.max != 2048*mib || len(f.releases) != 0 {
		t.Fatalf("refusal changed state: used=%d max=%d releases=%v", f.e.vmMemoryUsed, f.cg.max, f.releases)
	}
	// The lease plugged 2 GiB: 3/4 of the new MemTotal admits it (F18).
	f.budget = 4608 * mib
	if err := f.e.boostJobMemory(f.caps.ID, "job-a", 1536*mib, f.release()); err != nil {
		t.Fatal(err)
	}
	if f.e.vmMemoryUsed != 3584*mib || f.cg.max != 3584*mib {
		t.Fatalf("boost: used=%d max=%d", f.e.vmMemoryUsed>>20, f.cg.max>>20)
	}
	if got := f.e.MemoryBudget(); got != 4608*mib {
		t.Fatalf("MemoryBudget = %d, want the live reading", got)
	}
	// J12: a capsule spawn sees the boost in the same ledger.
	f.e.mu.Lock()
	err := f.e.admitCapsuleMemoryLocked(1536 * mib)
	f.e.mu.Unlock()
	if err == nil {
		t.Fatal("a spawn passed the budget the job's boost holds")
	}
	// J6: lowered first, then released.
	f.e.endJobMemory(f.caps.ID, "job-a")
	if f.e.vmMemoryUsed != 2048*mib || f.cg.max != 2048*mib || len(f.releases) != 1 || f.releases[0] != 2048*mib {
		t.Fatalf("end: used=%d max=%d releases=%v", f.e.vmMemoryUsed>>20, f.cg.max>>20, f.releases)
	}
	f.e.endJobMemory(f.caps.ID, "job-a") // a second end is a no-op
	if len(f.releases) != 1 {
		t.Fatalf("released twice: %v", f.releases)
	}
}

// J7, J8: usage the kernel cannot reclaim keeps the boost (no OOM kill in
// the capsule) until the capsule ends; destroy returns it and releases the
// need exactly once.
func TestJobMemoryRetainedUntilTheCapsuleEnds(t *testing.T) {
	f := newJobMemoryFixture(t)
	f.budget = 4096 * mib
	if err := f.e.boostJobMemory(f.caps.ID, "job-b", 768*mib, f.release()); err != nil {
		t.Fatal(err)
	}
	f.cg.current, f.cg.reclaimTo = 2600*mib, 2400*mib
	f.e.endJobMemory(f.caps.ID, "job-b")
	if f.e.vmMemoryUsed != 2816*mib || f.cg.max != 2816*mib || len(f.releases) != 0 {
		t.Fatalf("retained: used=%d max=%d releases=%v", f.e.vmMemoryUsed>>20, f.cg.max>>20, f.releases)
	}
	f.e.mu.Lock()
	releases := f.e.returnMemoryBoostsLocked(f.caps)
	f.e.mu.Unlock()
	for _, r := range releases {
		r()
	}
	if f.e.vmMemoryUsed != 2048*mib || len(f.releases) != 1 {
		t.Fatalf("destroy: used=%d releases=%v", f.e.vmMemoryUsed>>20, f.releases)
	}
	f.e.endJobMemory(f.caps.ID, "job-b")
	if len(f.releases) != 1 || f.e.vmMemoryUsed != 2048*mib {
		t.Fatalf("the job's end after destroy: used=%d releases=%v", f.e.vmMemoryUsed>>20, f.releases)
	}
}

// A job's end returns its memory before the job reads as done, so the next
// job in the capsule cannot boost while the last one is lowered.
func TestFinishJobReturnsMemoryBeforeDone(t *testing.T) {
	jf := newJobFixture(t, capsulejob.StateDone, 0, "ok\n")
	cg := &fakeCgroupMemory{current: 256 * mib, max: 2048 * mib}
	jf.caps.MemoryMax = 2048 * mib
	jf.executor.vmMemoryUsed = 2048 * mib
	jf.executor.memoryTotalFn = func() int64 { return 4096 * mib }
	jf.executor.cgroupMemoryFor = func(*Capsule) cgroupMemory { return cg }
	released, doneAtRelease := 0, false
	if err := jf.executor.boostJobMemory(jf.caps.ID, jf.job.spec.ID, 512*mib, func() {
		released++
		select {
		case <-jf.job.done:
			doneAtRelease = true
		default:
		}
	}); err != nil {
		t.Fatal(err)
	}
	jf.job.boosted = true // as StartJob marks it
	jf.executor.finishJob(jf.job, nil, capsulejob.StateStopped, "the capsule ended before the job did")
	if released != 1 || doneAtRelease || cg.max != 2048*mib || jf.executor.vmMemoryUsed != 2048*mib {
		t.Fatalf("finish: released=%d done before release=%t max=%d used=%d", released, doneAtRelease, cg.max>>20, jf.executor.vmMemoryUsed>>20)
	}
}
