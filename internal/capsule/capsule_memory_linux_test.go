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

// Q2 and Q7, review B5: under doctrine the capsules aggregate gets
// memory.max = the live capsule budget, or doctrine admits nothing new: a
// usage it cannot read, a usage above the budget (it is never lowered under
// usage) and a write that fails are errors. Under observe it is never
// written and never fails.
func TestCapsuleAggregateFollowsTheBudget(t *testing.T) {
	agg := &fakeAggregate{current: 1024 * mib, max: -1}
	e := &Executor{vmMemoryTotal: 6144 * mib, memoryPolicy: CapsuleMemoryDoctrine,
		capsuleAggregateFor: func() cgroupMemory { return agg }}
	ensure := func(e *Executor) error {
		e.mu.Lock()
		defer e.mu.Unlock()
		return e.ensureCapsuleAggregateLocked()
	}
	if err := ensure(e); err != nil || agg.max != 6144*mib {
		t.Fatalf("aggregate memory.max %d err %v, want the budget", agg.max, err)
	}
	// The budget shrank (memory unplugged) below what the capsules hold:
	// never lowered under usage, and doctrine admits nothing new.
	agg.current = 5000 * mib
	e.vmMemoryTotal = 4096 * mib
	if err := ensure(e); err == nil || agg.max != 6144*mib {
		t.Fatalf("usage above the budget: max %d err %v", agg.max>>20, err)
	}
	agg.current = 1024 * mib
	agg.currentErr = errFake
	if err := ensure(e); err == nil {
		t.Fatal("an unreadable usage passed")
	}
	agg.currentErr = nil
	agg.setErr = errFake
	if err := ensure(e); err == nil {
		t.Fatal("a failed aggregate write passed")
	}

	obs := &fakeAggregate{currentErr: errFake, setErr: errFake, max: -1}
	e2 := &Executor{vmMemoryTotal: 6144 * mib, capsuleAggregateFor: func() cgroupMemory { return obs }}
	if err := ensure(e2); err != nil || obs.max != -1 || len(obs.calls) != 0 {
		t.Fatalf("observe touched the aggregate: err %v max=%d calls=%v", err, obs.max, obs.calls)
	}
}

// Review B4: a boost admitted against a budget that hotplug raised first
// raises the aggregate to that budget, then the leaf; if the aggregate
// cannot rise the boost is refused with nothing raised or counted.
func TestDoctrineBoostRaisesTheAggregateFirst(t *testing.T) {
	var order []string
	agg := &fakeAggregate{current: 2048 * mib, max: 3072 * mib, order: &order, name: "aggregate"}
	cg := &fakeCgroupMemoryHigh{fakeCgroupMemory: fakeCgroupMemory{current: 512 * mib, max: 4096 * mib}, high: 2048 * mib}
	caps := &Capsule{ID: "capsule-hotplug", State: StateActive, MemoryMax: 4096 * mib, MemoryHigh: 2048 * mib, memoryAdmitted: 2048 * mib}
	budget := int64(3072 * mib)
	e := &Executor{
		capsules: map[string]*Capsule{caps.ID: caps}, vmMemoryTotal: 3072 * mib, vmMemoryUsed: 2048 * mib,
		memoryPolicy: CapsuleMemoryDoctrine, memoryTotalFn: func() int64 { return budget },
		cgroupMemoryFor:     func(*Capsule) cgroupMemory { order = append(order, "leaf"); return cg },
		capsuleAggregateFor: func() cgroupMemory { return agg },
	}
	budget = 4608 * mib // a memory lease plugged 1.5 GiB
	if err := e.boostJobMemory(caps.ID, "job-a", 1536*mib, nil); err != nil {
		t.Fatal(err)
	}
	if agg.max != 4608*mib || cg.max != 5632*mib || e.vmMemoryUsed != 3584*mib {
		t.Fatalf("boost: aggregate %d leaf %d used %d", agg.max>>20, cg.max>>20, e.vmMemoryUsed>>20)
	}
	if len(order) < 2 || order[0] != "aggregate" {
		t.Fatalf("order %v: the aggregate must rise before the leaf", order)
	}
	e.endJobMemory(caps.ID, "job-a")

	agg.setErr = errFake
	if err := e.boostJobMemory(caps.ID, "job-b", 1024*mib, nil); err == nil {
		t.Fatal("a boost passed although the aggregate could not rise")
	}
	if cg.max != 4096*mib || cg.high != 2048*mib || e.vmMemoryUsed != 2048*mib || len(caps.memoryBoosts) != 0 {
		t.Fatalf("refused boost changed state: leaf %d high %d used %d boosts %d", cg.max>>20, cg.high>>20, e.vmMemoryUsed>>20, len(caps.memoryBoosts))
	}
}

type fakeAggregate struct {
	current    int64
	currentErr error
	max        int64
	setErr     error
	calls      []string
	order      *[]string
	name       string
}

func (f *fakeAggregate) Current() (int64, error) {
	f.calls = append(f.calls, "current")
	return f.current, f.currentErr
}

func (f *fakeAggregate) Reclaim(int64) error { return nil }

func (f *fakeAggregate) SetMax(n int64) error {
	f.calls = append(f.calls, "max")
	if f.order != nil {
		*f.order = append(*f.order, f.name)
	}
	if f.setErr != nil {
		return f.setErr
	}
	f.max = n
	return nil
}
