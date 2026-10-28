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
// and so does a failed spawn. A new computer's persisted booting row, and
// the platform computer's first boot, is its claim. At load, a row whose claim is still open is a start that a
// vmctl restart interrupted. Under the restart rule (CLAUDE.md, "Restarts End
// Work (Crash) Or Resume It (Planned Update)") a crash restart never resumes
// work. The row is stopped with stoppedByStartInterrupted, which is not a
// failure of the computer. The surviving process, if any, is powered off
// before anything else may start the computer. The next owner act, the
// always-on sweep or a deploy rescue boots a fresh realization. A vmctl
// restart carries no planned-restart marker for the computers it hosts, so
// the planned-update exception does not apply.
//
// Review round 2 (bunch 37). The first close is free: the next start is not
// charged. The second consecutive close of the same computer, with no
// publish between, is a start that keeps crashing vmctl: it is stopped with
// stoppedByStartInterruptedRepeatedly, a failure, and the close charges the
// automatic retry, so the sweep and the platform warm leave it alone for
// failureWarmWindow (or until a deploy re-arms it) and only an owner act
// boots it. While a close is open (its process survived the kill) no start
// of the computer may spawn: spawnClaimed refuses, and so do the platform
// warm, the platform ensure and a direct recover, before they touch the row.

// BootClaim marks a start of the computer whose result is not yet published.
// It is immutable once written: row copies share it. A failed spawn drops
// its claim by pointer; a publish drops a claim only if the published row
// records the claimed realization (publishedIn), so it never drops the
// claim of a later start of the same computer.
type BootClaim struct {
	// Kind is the start: "boot" for a fresh realization (BootVM, RecoverVM,
	// RefreshVM), "resume" for a memory-snapshot or manager resume of the
	// same realization, "first boot" for a new computer's booting row.
	Kind string `json:"kind"`
	// Epoch is the realization the start brings up: the reserved epoch for a
	// fresh boot, the row's own epoch for a resume.
	Epoch     int64  `json:"epoch"`
	ClaimedAt string `json:"claimed_at,omitempty"`
	// Counted marks a claim a load already counted in the row's
	// StartInterruptions. A restart while its close is still open (the
	// process survived the kill) reloads the same claim and must not count
	// the one interrupted start twice. Set only at load, before the claim is
	// shared.
	Counted bool `json:"counted,omitempty"`
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
		own.StartInterruptions = 0
	}
	if own.BootClaim == nil {
		if own.State != VMStateBooting {
			return false
		}
		own.BootClaim = &BootClaim{Kind: bootClaimFirstBoot, Epoch: own.Epoch, ClaimedAt: now.UTC().Format(time.RFC3339Nano)}
	}
	own.State = VMStateStopped
	if !own.BootClaim.Counted {
		// One count per interrupted start, however many restarts its close
		// spans.
		own.StartInterruptions++
		own.BootClaim.Counted = true
	}
	if own.StartInterruptions < 1 {
		own.StartInterruptions = 1
	}
	own.StoppedBy = stoppedByStartInterrupted
	if own.StartInterruptions >= 2 {
		own.StoppedBy = stoppedByStartInterruptedRepeatedly
	}
	return true
}

// publishStartLocked records that a start published own (State, Epoch and
// address already written): it drops the claim the publish records, keeps a
// claim of a later realization, and ends the run of interrupted starts. The
// caller saves in the same write.
func publishStartLocked(own *VMOwnership) {
	if own.BootClaim.publishedIn(own) {
		own.BootClaim = nil
	}
	own.StartInterruptions = 0
}

// errStartInterruptedOpen refuses a start of a computer whose interrupted
// start is not yet closed. It spawned nothing and clears with the close.
func errStartInterruptedOpen(vmID string) error {
	return &RecoveryRefusal{
		Kind:              RecoveryRefusalGuestReattachPending,
		Reason:            fmt.Sprintf("VM %s: the process of a start a vmctl restart interrupted is still being powered off; retry shortly", vmID),
		RetryAfterSeconds: guestReattachRetryAfterSeconds,
	}
}

// refuseWhileCloseOpen reports why no start of vmID may spawn now: its
// interrupted start is not closed, or a Firecracker process the manager does
// not track still runs for it. nil when a start may proceed.
func (r *OwnershipRegistry) refuseWhileCloseOpen(vmID string, mgr VMManager) error {
	if r.startInterrupted(vmID) {
		return errStartInterruptedOpen(vmID)
	}
	if mgr != nil && mgr.GetVM(vmID) == nil && r.liveUnmanagedProcess(vmID, mgr) {
		return &RecoveryRefusal{
			Kind:              RecoveryRefusalGuestReattachPending,
			Reason:            "computer is still running but vmctl does not track it; retry shortly",
			RetryAfterSeconds: guestReattachRetryAfterSeconds,
		}
	}
	return nil
}

// repeatedInterruptionBoundLocked reports a computer whose starts keep
// crashing vmctl and whose charged retry has not come round again: no
// automatic start may boot it.
func (r *OwnershipRegistry) repeatedInterruptionBoundLocked(own *VMOwnership) bool {
	return own != nil && own.StoppedBy == stoppedByStartInterruptedRepeatedly && !failureWarmAllowed(own.FailureWarmedAt, r.now())
}

// spawnClaimed runs one manager spawn of an existing computer under a
// durable start claim. The claim is written before the manager is called.
// If it cannot be written, the start is refused before anything spawns. A
// failed spawn drops the claim, and a successful one leaves it for the
// publish to clear. A start of a computer whose interrupted start is still
// open is refused: the shield stays until the close ends the process.
func (r *OwnershipRegistry) spawnClaimed(vmID, kind string, epoch int64, spawn func() (*VMInstanceInfo, error)) (*VMInstanceInfo, error) {
	r.mu.Lock()
	// Every boot, resume and recover of an existing computer spawns here. A
	// planned host stop's pass hibernates computers and a spawn of one would
	// consume the snapshot it wrote, so none runs once the pass has begun.
	// The flag is set under r.mu (HibernateForHostStop), so a spawn either
	// holds its claim already or is refused.
	if r.hostStopping.Load() {
		r.mu.Unlock()
		return nil, errHostStopping
	}
	if _, open := r.interruptedStarts[vmID]; open {
		r.mu.Unlock()
		return nil, errStartInterruptedOpen(vmID)
	}
	if r.persistencePath == "" {
		// No durable registry (tests, host-process mode): the start runs
		// unclaimed, and a vmctl restart mid-boot would orphan it.
		log.Printf("vmctl: WARNING: %s of VM %s runs without a durable start claim: the ownership registry has no persistence path", kind, vmID)
	}
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
			// No start of this VM spawns while its close is open
			// (spawnClaimed). The manager's per-VM operation lock serializes
			// the kill with BootVM, RecoverVM, RefreshVM and
			// ResumeVMFromSnapshot; ResumeVM does not take it, but acts only
			// on a VM the manager tracks, and the kill refuses one.
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
		if own != nil && own.State == VMStateStopped && (own.StoppedBy == stoppedByStartInterrupted || own.StoppedBy == stoppedByStartInterruptedRepeatedly) {
			// The interrupted start may have run the guest from its memory
			// snapshot; the next start cold boots.
			r.discardStoppedSnapshotLocked(own, own.StoppedBy)
			if own.StoppedBy == stoppedByStartInterruptedRepeatedly {
				// A start that keeps crashing vmctl is charged like a failure.
				own.FailureWarmedAt = r.now().UTC().Format(time.RFC3339Nano)
				own.failureWarmGen++
			}
		}
		// Read for the log under the lock: a start released by the close
		// may write the row as soon as it is unlocked.
		userID, stoppedBy, interruptions := "", stoppedByStartInterrupted, 0
		if own != nil {
			userID, stoppedBy, interruptions = own.UserID, own.StoppedBy, own.StartInterruptions
		}
		r.saveLocked()
		r.mu.Unlock()
		if stoppedBy == stoppedByStartInterruptedRepeatedly {
			log.Printf("vmctl: closed VM %s (user %s): its %s of epoch %d was interrupted by a vmctl restart (%s), %d starts in a row; stopped_by=%s, the automatic retry is spent: the sweep leaves it for %s, a deploy of a new target re-arms it and the deploy rescue boots it, an owner act boots it", vmID, userID, claim.Kind, claim.Epoch, ended, interruptions, stoppedBy, failureWarmWindow)
			continue
		}
		log.Printf("vmctl: closed VM %s (user %s): its %s of epoch %d was interrupted by a vmctl restart (%s); stopped_by=%s, the next start boots fresh", vmID, userID, claim.Kind, claim.Epoch, ended, stoppedBy)
	}
}

// stoppedByRestart reports a stop that a vmctl restart caused, either of a
// running computer or of a start in flight. It is not a failure of the
// computer.
func stoppedByRestart(stoppedBy string) bool {
	return stoppedBy == stoppedByVMCTLRestart || stoppedBy == stoppedByStartInterrupted
}
