package vmctl

import (
	"fmt"
	"log"
	"sort"
	"time"
)

// Durable start claims
// (docs/problems/wake-during-deploy-vmctl-restart-orphans-the-vm-2026-10-22.md).
// A start of an existing computer used to be claimed only in memory
// (r.lifecycle): the row kept its pre-start state (hibernated, stopped,
// failed, or active for a refresh) until the start published. A vmctl
// restart mid-boot left a Firecracker process that no row described. The
// new vmctl had nothing to reattach, and its wedge detector probed the row's
// stale address and recorded the orphan as the computer's own wedge.
//
// Every manager spawn of an existing computer now writes a BootClaim on the
// row before the manager is called. The publish clears it in the same write,
// and so does a failed spawn. A new computer's persisted booting row
// is its claim. At load, a row whose claim is still open is a start that a
// vmctl restart interrupted. Under the restart rule (CLAUDE.md, "Restarts End
// Work (Crash) Or Resume It (Planned Update)") a crash restart never resumes
// work. The row is stopped with stoppedByStartInterrupted, which is not a
// failure of the computer. The surviving process, if any, is powered off
// before anything else may start the computer. The next owner act, the
// always-on sweep or a deploy rescue boots a fresh realization. A vmctl
// restart carries no planned-restart marker for the computers it hosts, so
// the planned-update exception does not apply.

// BootClaim marks a start of the computer whose result is not yet published.
// It is immutable once written: row copies share it, and a claim is dropped
// only by pointer.
type BootClaim struct {
	// Kind is the start: "boot" for a fresh realization (BootVM, RecoverVM,
	// RefreshVM), "resume" for a memory-snapshot or manager resume of the
	// same realization, "first boot" for a new computer's booting row.
	Kind string `json:"kind"`
	// Epoch is the realization the start brings up: the reserved epoch for a
	// fresh boot, the row's own epoch for a resume.
	Epoch     int64  `json:"epoch"`
	ClaimedAt string `json:"claimed_at,omitempty"`
}

const (
	bootClaimBoot      = "boot"
	bootClaimResume    = "resume"
	bootClaimFirstBoot = "first boot"
)

// publishedIn reports whether own already records the claimed start's result,
// or a later one: the publish that should have dropped the claim was written.
// Every publish clears the claim. This rule only keeps a claim that outlived
// its publish from ending a running computer.
func (c *BootClaim) publishedIn(own *VMOwnership) bool {
	if c == nil || own == nil {
		return true
	}
	if own.Epoch > c.Epoch {
		return true
	}
	return own.Epoch == c.Epoch && (own.State == VMStateActive || own.State == VMStateDegraded)
}

// interruptStartAtLoadLocked turns a row whose start a vmctl restart
// interrupted into a stopped computer with an honest fate. It reports
// whether it did. A persisted booting row without a claim is a new
// computer's first boot or a platform start, and gets a claim so that a
// close that cannot finish is still open after the next restart.
func interruptStartAtLoadLocked(own *VMOwnership, now time.Time) bool {
	if own.BootClaim != nil && own.BootClaim.publishedIn(own) {
		own.BootClaim = nil
	}
	if own.BootClaim == nil {
		if own.State != VMStateBooting {
			return false
		}
		own.BootClaim = &BootClaim{Kind: bootClaimFirstBoot, Epoch: own.Epoch, ClaimedAt: now.UTC().Format(time.RFC3339Nano)}
	}
	own.State = VMStateStopped
	own.StoppedBy = stoppedByStartInterrupted
	return true
}

// spawnClaimed runs one manager spawn of an existing computer under a
// durable start claim. The claim is written before the manager is called.
// If it cannot be written, the start is refused before anything spawns. A
// failed spawn drops the claim, and a successful one leaves it for the
// publish to clear.
func (r *OwnershipRegistry) spawnClaimed(vmID, kind string, epoch int64, spawn func() (*VMInstanceInfo, error)) (*VMInstanceInfo, error) {
	r.mu.Lock()
	var claim *BootClaim
	if own := r.vmByID[vmID]; own != nil && r.persistencePath != "" {
		claim = &BootClaim{Kind: kind, Epoch: epoch, ClaimedAt: r.now().UTC().Format(time.RFC3339Nano)}
		prev := own.BootClaim
		own.BootClaim = claim
		if err := r.writePersistenceLocked(); err != nil {
			own.BootClaim = prev
			r.mu.Unlock()
			return nil, fmt.Errorf("persist start claim for VM %s: %w", vmID, err)
		}
		// This start replaces whatever an interrupted one left: the manager
		// refuses to boot over a live untracked process, so nothing of it runs.
		delete(r.interruptedStarts, vmID)
	}
	r.mu.Unlock()
	info, err := spawn()
	if err != nil || info == nil {
		r.mu.Lock()
		if own := r.vmByID[vmID]; own != nil && claim != nil && own.BootClaim == claim {
			own.BootClaim = nil
			r.saveLocked()
		}
		r.mu.Unlock()
	}
	return info, err
}

// startInterrupted reports a computer whose interrupted start is not yet
// closed: it is never adopted or probed for a wedge, and only the close
// ends its process.
func (r *OwnershipRegistry) startInterrupted(vmID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, open := r.interruptedStarts[vmID]
	return open
}

// closeInterruptedStarts powers off the process each interrupted start left
// behind and records the close. ReattachManagedVMs runs it once at startup,
// before vmctl serves, and every process reconcile retries a close whose
// process survived the kill.
func (r *OwnershipRegistry) closeInterruptedStarts() {
	r.mu.RLock()
	mgr := r.vmManager
	open := make(map[string]*BootClaim, len(r.interruptedStarts))
	for vmID, claim := range r.interruptedStarts {
		open[vmID] = claim
	}
	r.mu.RUnlock()
	if len(open) == 0 {
		return
	}
	pc, _ := mgr.(vmProcessController)
	var live map[string][]int
	if pc != nil {
		live = pc.LiveFirecrackerVMs()
	}
	vmIDs := make([]string, 0, len(open))
	for vmID := range open {
		vmIDs = append(vmIDs, vmID)
	}
	sort.Strings(vmIDs)
	for _, vmID := range vmIDs {
		claim := open[vmID]
		ended := "no process survived"
		if mgr != nil && mgr.GetVM(vmID) != nil {
			// This vmctl started the computer since; its claim replaced this one.
			ended = "started again since"
		} else if pids := live[vmID]; len(pids) > 0 {
			// The manager's per-VM operation lock serializes the kill with any
			// boot of the same VM, and it refuses a VM the manager runs.
			if err := pc.ReapUnmanagedVM(vmID); err != nil {
				log.Printf("vmctl: close of VM %s, whose %s of epoch %d a vmctl restart interrupted, failed; pids %v still run, retried by the next reconcile: %v", vmID, claim.Kind, claim.Epoch, pids, err)
				r.mu.Lock()
				r.saveLocked() // the fate is durable while the close stays open
				r.mu.Unlock()
				continue
			}
			ended = fmt.Sprintf("powered off pids %v", pids)
		}
		r.mu.Lock()
		if r.interruptedStarts[vmID] != claim {
			r.mu.Unlock()
			continue
		}
		delete(r.interruptedStarts, vmID)
		delete(r.unmanagedSince, vmID)
		delete(r.networkDeadSince, vmID)
		own := r.vmByID[vmID]
		if own != nil && own.BootClaim == claim {
			own.BootClaim = nil
		}
		if own != nil && own.State == VMStateStopped && own.StoppedBy == stoppedByStartInterrupted {
			// The interrupted start may have run the guest from its memory
			// snapshot; the next start cold boots.
			r.discardStoppedSnapshotLocked(own, stoppedByStartInterrupted)
		}
		r.saveLocked()
		r.mu.Unlock()
		userID := ""
		if own != nil {
			userID = own.UserID
		}
		log.Printf("vmctl: closed VM %s (user %s): its %s of epoch %d was interrupted by a vmctl restart (%s); stopped_by=%s, the next start boots fresh", vmID, userID, claim.Kind, claim.Epoch, ended, stoppedByStartInterrupted)
	}
}

// stoppedByRestart reports a stop that a vmctl restart caused, either of a
// running computer or of a start in flight. It is not a failure of the
// computer.
func stoppedByRestart(stoppedBy string) bool {
	return stoppedBy == stoppedByVMCTLRestart || stoppedBy == stoppedByStartInterrupted
}
