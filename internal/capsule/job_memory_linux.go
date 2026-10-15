//go:build linux

package capsule

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
)

// Job memory on the executor; the design and failure modes are in
// docs/reports/memory-hotplug-controller-design-2026-10-14.md section 9.

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
	cg := e.cgroupMemoryOf(caps)
	if cg == nil {
		return fmt.Errorf("capsule %s cgroup memory controls are unavailable", capsuleID)
	}
	if err := cg.SetMax(caps.MemoryMax + boostedBytesLocked(caps) + bytes); err != nil {
		return fmt.Errorf("raise capsule %s memory.max: %w", capsuleID, err)
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
