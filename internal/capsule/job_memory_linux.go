//go:build linux

package capsule

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
)

// Job memory on the executor; the design and failure modes are in
// docs/archive/reports/memory-hotplug-controller-design-2026-10-14.md section 9.

// SetLiveMemoryBudget makes the capsule budget follow the guest's memory:
// budget is read at every admission (LiveMemoryBudget); a reading of 0
// keeps the boot budget. Without it the budget stays the boot value
// (CHOIR_CAPSULE_MEMORY_TOTAL_BYTES pins it that way).
func (e *Executor) SetLiveMemoryBudget(budget func() int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.memoryTotalFn = budget
}

// memoryBudgetLocked is the capsule budget now. Callers hold e.mu.
func (e *Executor) memoryBudgetLocked() int64 {
	if e.memoryTotalFn != nil {
		if live := e.memoryTotalFn(); live > 0 {
			return live
		}
	}
	return e.vmMemoryTotal
}

// admitCapsuleMemoryLocked checks need against the live budget, with every
// capsule and job boost already counted. Callers hold e.mu.
func (e *Executor) admitCapsuleMemoryLocked(need int64) error {
	total := e.memoryBudgetLocked()
	if total <= 0 || e.vmMemoryUsed+need > total {
		return fmt.Errorf("capsule memory budget exceeded: used=%d requested=%d total=%d", e.vmMemoryUsed, need, total)
	}
	return nil
}

func (e *Executor) cgroupMemoryOf(caps *Capsule) cgroupMemory {
	if e.cgroupMemoryFor != nil {
		return e.cgroupMemoryFor(caps)
	}
	if cg, ok := caps.Cgroup.(*CgroupManager); ok && cg != nil && cg.path != "" {
		return cgroupMemoryDir(filepath.Join("/sys/fs/cgroup", strings.TrimPrefix(cg.path, "/")))
	}
	return nil
}

func boostedBytesLocked(caps *Capsule) int64 {
	var sum int64
	for _, b := range caps.memoryBoosts {
		sum += b.bytes
	}
	return sum
}

// stuckBoostPrefix keys a refused boost whose memory.max could not be put
// back (review E6).
const stuckBoostPrefix = "stuck:"

// boostJobMemory admits a job's declared need against the live budget and
// raises its capsule's memory.max by it. release (the job's lease share)
// runs once, when the boost is returned. On error nothing was raised or
// counted and release has not run.
func (e *Executor) boostJobMemory(capsuleID, jobID string, bytes int64, release func()) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	caps := e.capsules[capsuleID]
	if caps == nil || caps.State != StateActive {
		return fmt.Errorf("capsule %s is not active", capsuleID)
	}
	if _, taken := caps.memoryBoosts[jobID]; taken {
		return fmt.Errorf("job %s already holds memory", jobID)
	}
	if err := e.admitCapsuleMemoryLocked(bytes); err != nil {
		return fmt.Errorf("job memory_mib %d: %w", bytes>>20, err)
	}
	// Under doctrine the aggregate follows the live budget the boost was
	// admitted against (a memory lease may have raised it) before any leaf
	// rises (review B4); if it cannot, the boost is refused.
	if err := e.ensureCapsuleAggregateLocked(); err != nil {
		return fmt.Errorf("job memory_mib %d: %w", bytes>>20, err)
	}
	cg := e.cgroupMemoryOf(caps)
	if cg == nil {
		return fmt.Errorf("capsule %s cgroup memory controls are unavailable", capsuleID)
	}
	if err := cg.SetMax(caps.MemoryMax + boostedBytesLocked(caps) + bytes); err != nil {
		return fmt.Errorf("raise capsule %s memory.max: %w", capsuleID, err)
	}
	// Under doctrine memory.high rises with it, or the job is throttled
	// under its own boost (RES-04, Q4). A memory.high that cannot rise
	// refuses the boost and puts memory.max back.
	if caps.MemoryHigh > 0 {
		hc, ok := cg.(cgroupMemoryHigh)
		var err error
		if !ok {
			err = fmt.Errorf("no memory.high control")
		} else {
			err = hc.SetHigh(caps.MemoryHigh + boostedBytesLocked(caps) + bytes)
		}
		if err != nil {
			if restoreErr := cg.SetMax(caps.MemoryMax + boostedBytesLocked(caps)); restoreErr != nil {
				// Review E6: memory.max stays raised. The raise is counted
				// until the capsule ends, as a boost that cannot come down
				// is (endJobMemory), under a key the job's end never reaches;
				// the job's lease share is not held by it.
				if caps.memoryBoosts == nil {
					caps.memoryBoosts = map[string]*jobMemoryBoost{}
				}
				// A second stuck raise for the same job adds to the first
				// (review F3).
				if stuck := caps.memoryBoosts[stuckBoostPrefix+jobID]; stuck != nil {
					stuck.bytes += bytes
				} else {
					caps.memoryBoosts[stuckBoostPrefix+jobID] = &jobMemoryBoost{bytes: bytes}
				}
				e.vmMemoryUsed += bytes
				log.Printf("capsule: job %s memory boost refused in capsule %s, but memory.max stays raised by %d MiB and is counted until the capsule ends: %v", jobID, capsuleID, bytes>>20, restoreErr)
				return fmt.Errorf("raise capsule %s memory.high: %w; memory.max could not be put back and stays raised by %d MiB, counted until the capsule ends: %v", capsuleID, err, bytes>>20, restoreErr)
			}
			return fmt.Errorf("raise capsule %s memory.high: %w", capsuleID, err)
		}
	}
	if caps.memoryBoosts == nil {
		caps.memoryBoosts = map[string]*jobMemoryBoost{}
	}
	caps.memoryBoosts[jobID] = &jobMemoryBoost{bytes: bytes, release: release}
	e.vmMemoryUsed += bytes
	log.Printf("capsule: job %s memory boost +%d MiB in capsule %s (memory.max %d MiB)", jobID, bytes>>20, capsuleID, (caps.MemoryMax+boostedBytesLocked(caps))>>20)
	return nil
}

// endJobMemory returns a job's boost: the cgroup is lowered first (only
// when its usage fits the lower limit, J7), then the budget, then the lease
// share is released. A boost whose cgroup cannot come down is retained
// until the capsule is destroyed. Idempotent; a capsule already destroyed
// returned its boosts then.
func (e *Executor) endJobMemory(capsuleID, jobID string) {
	e.mu.Lock()
	caps := e.capsules[capsuleID]
	var b *jobMemoryBoost
	if caps != nil {
		b = caps.memoryBoosts[jobID]
	}
	if b == nil || caps.State != StateActive {
		e.mu.Unlock()
		return
	}
	cg := e.cgroupMemoryOf(caps)
	limit := caps.MemoryMax + boostedBytesLocked(caps) - b.bytes
	high := int64(0)
	if caps.MemoryHigh > 0 {
		high = caps.MemoryHigh + boostedBytesLocked(caps) - b.bytes
	}
	e.mu.Unlock()

	// Reclaim can take a while: never under the executor lock. The job is
	// not done yet (finishJob closes it after this), so no other job in the
	// capsule can raise the cgroup meanwhile; a destroy can, and wins.
	lowered, err := lowerCgroupMemory(cg, limit)

	e.mu.Lock()
	if caps.memoryBoosts[jobID] != b {
		e.mu.Unlock()
		return // the capsule's destroy returned it meanwhile
	}
	if !lowered {
		e.mu.Unlock()
		log.Printf("capsule: job %s memory boost of %d MiB retained until capsule %s ends: its usage does not fit %d MiB (err=%v)", jobID, b.bytes>>20, capsuleID, limit>>20, err)
		return
	}
	// memory.high follows memory.max down (Q5); a failure leaves it high,
	// which throttles less, never more.
	if hc, ok := cg.(cgroupMemoryHigh); ok && high > 0 {
		if err := hc.SetHigh(high); err != nil {
			log.Printf("capsule: job %s: lower capsule %s memory.high to %d MiB: %v", jobID, capsuleID, high>>20, err)
		}
	}
	delete(caps.memoryBoosts, jobID)
	e.vmMemoryUsed -= b.bytes
	e.mu.Unlock()
	if b.release != nil {
		b.release()
	}
}

// returnMemoryBoostsLocked returns every boost a destroyed capsule held to
// the budget and hands back their releases for the caller to run outside
// the lock. Callers hold e.mu.
func (e *Executor) returnMemoryBoostsLocked(caps *Capsule) []func() {
	var releases []func()
	for jobID, b := range caps.memoryBoosts {
		e.vmMemoryUsed -= b.bytes
		if b.release != nil {
			releases = append(releases, b.release)
		}
		delete(caps.memoryBoosts, jobID)
	}
	return releases
}

// SetMemoryPolicy sets the capsule memory shape for capsules spawned from now
// on (RES-04, capsule_memory.go). Capsules already live keep theirs.
func (e *Executor) SetMemoryPolicy(policy CapsuleMemoryPolicy) {
	if policy != CapsuleMemoryDoctrine {
		policy = CapsuleMemoryObserve
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.memoryPolicy = policy
}

// capsuleAggregateMemory is the capsules aggregate cgroup, the parent of
// every /capsule/<id> (namespace.go).
func (e *Executor) capsuleAggregateMemory() cgroupMemory {
	if e.capsuleAggregateFor != nil {
		return e.capsuleAggregateFor()
	}
	return cgroupMemoryDir("/sys/fs/cgroup/capsule")
}

// ensureCapsuleAggregateLocked caps the capsules aggregate at the live
// capsule budget under doctrine, where admission counts requests and a
// capsule may reach twice its request (Q2). Doctrine admits nothing new
// without that bound (review B5): a usage it cannot read, a usage above the
// budget (the aggregate is never lowered under usage, Q7, so the kernel
// never kills across capsules for it) and a failed write are errors. Raising
// it is always safe. Under observe it is not written (admission already
// bounds every memory.max) and never fails. Callers hold e.mu.
func (e *Executor) ensureCapsuleAggregateLocked() error {
	if e.memoryPolicy != CapsuleMemoryDoctrine {
		return nil
	}
	budget := e.memoryBudgetLocked()
	agg := e.capsuleAggregateMemory()
	if budget <= 0 || agg == nil {
		return fmt.Errorf("capsules aggregate: no budget (%d) or no aggregate cgroup", budget)
	}
	current, err := agg.Current()
	if err != nil {
		return fmt.Errorf("capsules aggregate: read usage: %w", err)
	}
	if current > budget {
		return fmt.Errorf("capsules aggregate: usage %d MiB is above the budget %d MiB", current>>20, budget>>20)
	}
	if err := agg.SetMax(budget); err != nil {
		return fmt.Errorf("capsules aggregate: memory.max %d MiB: %w", budget>>20, err)
	}
	return nil
}

const memoryReceiptsKept = 16

// recordMemoryReceiptLocked reads and logs a destroyed capsule's memory
// receipt and keeps the last few for observability. Callers hold e.mu.
func (e *Executor) recordMemoryReceiptLocked(caps *Capsule) {
	policy := caps.memoryPolicy
	if policy == "" {
		policy = CapsuleMemoryObserve
	}
	plan := capsuleMemoryPlan{Admit: caps.admittedMemory(), High: caps.MemoryHigh, Max: caps.MemoryMax}
	var cg any
	if m := e.cgroupMemoryOf(caps); m != nil {
		cg = m
	}
	r := readCapsuleMemoryReceipt(policy, plan, caps.Spec.MemoryRequest, cg)
	r.CapsuleID = caps.ID
	log.Printf("capsule %s memory: %s", caps.ID, r)
	e.memoryReceipts = append(e.memoryReceipts, r)
	if n := len(e.memoryReceipts); n > memoryReceiptsKept {
		e.memoryReceipts = append([]CapsuleMemoryReceipt(nil), e.memoryReceipts[n-memoryReceiptsKept:]...)
	}
}

// MemoryReceipts returns the last capsules' memory receipts, newest last.
func (e *Executor) MemoryReceipts() []CapsuleMemoryReceipt {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return append([]CapsuleMemoryReceipt(nil), e.memoryReceipts...)
}
