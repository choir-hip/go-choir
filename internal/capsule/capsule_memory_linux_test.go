//go:build linux

package capsule

import (
	"strings"
	"testing"
)

// Q6: the bytes a destroy returns to the budget are the bytes the spawn
// admitted, under either policy; a capsule from before RES-04 (no admitted
// field) returns its MemoryMax as it always did.
func TestCapsuleAdmittedMemory(t *testing.T) {
	if got := (&Capsule{MemoryMax: 4 << 30, memoryAdmitted: 2 << 30}).admittedMemory(); got != 2<<30 {
		t.Fatalf("doctrine capsule returns %d, want the request", got)
	}
	if got := (&Capsule{MemoryMax: 2 << 30}).admittedMemory(); got != 2<<30 {
		t.Fatalf("old capsule returns %d, want MemoryMax", got)
	}
}

// fakeCgroupMemoryHigh is a capsule cgroup with memory.high (doctrine).
type fakeCgroupMemoryHigh struct {
	fakeCgroupMemory
	high int64
}

func (f *fakeCgroupMemoryHigh) SetHigh(n int64) error {
	f.calls = append(f.calls, "high")
	f.high = n
	return nil
}

// Q4 and Q5: under doctrine a job's boost raises memory.high with
// memory.max, and its end lowers both; memory.max only when usage fits.
func TestJobMemoryBoostRaisesHighUnderDoctrine(t *testing.T) {
	cg := &fakeCgroupMemoryHigh{fakeCgroupMemory: fakeCgroupMemory{current: 512 * mib, max: 4096 * mib}, high: 2048 * mib}
	caps := &Capsule{ID: "capsule-doctrine", State: StateActive, MemoryMax: 4096 * mib, MemoryHigh: 2048 * mib, memoryAdmitted: 2048 * mib}
	e := &Executor{
		capsules:        map[string]*Capsule{caps.ID: caps},
		vmMemoryTotal:   6144 * mib,
		vmMemoryUsed:    2048 * mib,
		cgroupMemoryFor: func(*Capsule) cgroupMemory { return cg },
	}
	if err := e.boostJobMemory(caps.ID, "job-a", 1024*mib, nil); err != nil {
		t.Fatal(err)
	}
	if cg.max != 5120*mib || cg.high != 3072*mib || e.vmMemoryUsed != 3072*mib {
		t.Fatalf("boost: max=%d high=%d used=%d", cg.max>>20, cg.high>>20, e.vmMemoryUsed>>20)
	}
	e.endJobMemory(caps.ID, "job-a")
	if cg.max != 4096*mib || cg.high != 2048*mib || e.vmMemoryUsed != 2048*mib {
		t.Fatalf("end: max=%d high=%d used=%d", cg.max>>20, cg.high>>20, e.vmMemoryUsed>>20)
	}

	// Observe: a capsule with no memory.high never gets one from a boost.
	cg2 := &fakeCgroupMemoryHigh{fakeCgroupMemory: fakeCgroupMemory{current: 512 * mib, max: 2048 * mib}}
	caps2 := &Capsule{ID: "capsule-observe", State: StateActive, MemoryMax: 2048 * mib}
	e2 := &Executor{capsules: map[string]*Capsule{caps2.ID: caps2}, vmMemoryTotal: 6144 * mib, vmMemoryUsed: 2048 * mib,
		cgroupMemoryFor: func(*Capsule) cgroupMemory { return cg2 }}
	if err := e2.boostJobMemory(caps2.ID, "job-b", 1024*mib, nil); err != nil {
		t.Fatal(err)
	}
	if cg2.high != 0 || strings.Contains(strings.Join(cg2.calls, ","), "high") {
		t.Fatalf("observe capsule got memory.high: %d calls %v", cg2.high, cg2.calls)
	}
}

// Q2 and Q7: under doctrine the capsules aggregate gets memory.max = the live
// capsule budget, written only when the subtree's usage fits under it; under
// observe it is never written.
func TestCapsuleAggregateFollowsTheBudget(t *testing.T) {
	agg := &fakeCgroupMemory{current: 1024 * mib, max: -1}
	e := &Executor{vmMemoryTotal: 6144 * mib, memoryPolicy: CapsuleMemoryDoctrine,
		capsuleAggregateFor: func() cgroupMemory { return agg }}
	e.mu.Lock()
	e.setCapsuleAggregateLocked()
	e.mu.Unlock()
	if agg.max != 6144*mib {
		t.Fatalf("aggregate memory.max %d, want the budget", agg.max)
	}
	// The budget shrank (memory unplugged) below what the capsules hold:
	// never lowered under usage.
	agg.current = 5000 * mib
	e.vmMemoryTotal = 4096 * mib
	e.mu.Lock()
	e.setCapsuleAggregateLocked()
	e.mu.Unlock()
	if agg.max != 6144*mib {
		t.Fatalf("aggregate lowered under usage to %d", agg.max>>20)
	}

	obs := &fakeCgroupMemory{current: 0, max: -1}
	e2 := &Executor{vmMemoryTotal: 6144 * mib, capsuleAggregateFor: func() cgroupMemory { return obs }}
	e2.mu.Lock()
	e2.setCapsuleAggregateLocked()
	e2.mu.Unlock()
	if obs.max != -1 || len(obs.calls) != 0 {
		t.Fatalf("observe wrote the aggregate: max=%d calls=%v", obs.max, obs.calls)
	}
}
