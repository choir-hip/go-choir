package vmctl

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

const (
	UniversalWirePlatformOwnerID    = "universal-wire-platform"
	UniversalWirePlatformDesktopID  = "platform"
	UniversalWirePlatformVMID       = "vm-universal-wire-platform"
	UniversalWirePlatformComputerID = "computer-universal-wire-platform"
)

// EnsureUniversalWirePlatformComputer boots or resumes the always-on platform
// computer unless its persisted ownership is under maintenance hold. A held
// platform computer is intentionally left unchanged and treated as a skipped
// automatic lifecycle action. Dispatch routing is handled by the autoputer
// proxy (UDS) — callers no longer need the autoputer URL directly.
func (r *OwnershipRegistry) EnsureUniversalWirePlatformComputer(ctx context.Context) error {
	own, err := r.ensureUniversalWirePlatformOwnership(ctx)
	if err != nil {
		return fmt.Errorf("ensure universal wire platform computer: %w", err)
	}
	if own == nil || strings.TrimSpace(own.VMID) == "" {
		return fmt.Errorf("universal wire platform computer has no VM ID")
	}
	return nil
}

// WarmUniversalWirePlatformComputer resumes a stopped platform computer during
// the idle sweeper loop. It does not create a new ownership on its own.
//
// It is a start like any other (review round 2 of
// docs/problems/wake-during-deploy-vmctl-restart-orphans-the-vm-2026-10-22.md):
// it refuses while another start holds the computer (a lifecycle claim or an
// ensure's pending boot), while an interrupted start's close is open or an
// untracked process still runs, and while a start that keeps crashing vmctl
// is inside its charged window. It takes the lifecycle claim and the pending
// slot for the whole start. A manager that does not track the VM (after a
// vmctl restart) cannot ResumeVM it, so the warm boots it like the ensure
// does (startExistingVM). A refusal that spawned nothing restores the row.
func (r *OwnershipRegistry) WarmUniversalWirePlatformComputer(ctx context.Context, guard ComputerVersionRouteGuard) int {
	key := ownershipKey(UniversalWirePlatformOwnerID, UniversalWirePlatformDesktopID)
	r.mu.RLock()
	own, ok := r.ownerships[key]
	r.mu.RUnlock()
	if !ok || own == nil || own.IsReady() {
		return 0
	}
	if own.IsHeld() {
		log.Printf("vmctl: platform computer vm=%s refused: held", own.VMID)
		return 0
	}
	if own.State != VMStateStopped && own.State != VMStateHibernated {
		return 0
	}
	if guard == nil {
		log.Printf("vmctl: refuse platform computer warm: ComputerVersion route guard is unavailable")
		return 0
	}
	if err := guard(ctx, UniversalWirePlatformOwnerID, UniversalWirePlatformDesktopID); err != nil {
		log.Printf("vmctl: refuse platform computer warm: %v", err)
		return 0
	}
	r.mu.Lock()
	own, ok = r.ownerships[key]
	if !ok || own == nil || own.IsReady() || (own.State != VMStateStopped && own.State != VMStateHibernated) {
		r.mu.Unlock()
		return 0
	}
	if own.IsHeld() {
		log.Printf("vmctl: platform computer vm=%s refused: held", own.VMID)
		r.mu.Unlock()
		return 0
	}
	if err := r.lifecycleConflictLocked(key); err != nil {
		r.mu.Unlock()
		log.Printf("vmctl: refuse platform computer warm: %v", err)
		return 0
	}
	if _, pending := r.pendingWaiters[key]; pending {
		r.mu.Unlock()
		return 0
	}
	if r.repeatedInterruptionBoundLocked(own) {
		msg := fmt.Sprintf("vmctl: refuse platform computer warm vm=%s: its starts were interrupted by vmctl restarts %d times in a row (stopped_by=%s); the automatic retry is spent until %s after %s, or until a deploy of a new target re-arms it and the deploy rescue boots it", own.VMID, own.StartInterruptions, own.StoppedBy, failureWarmWindow, own.FailureWarmedAt)
		r.mu.Unlock()
		log.Print(msg)
		return 0
	}
	mgr := r.vmManager
	vmID := own.VMID
	r.mu.Unlock()
	if mgr == nil {
		return 0
	}
	if err := r.refuseWhileCloseOpen(vmID, mgr); err != nil {
		log.Printf("vmctl: refuse platform computer warm: %v", err)
		return 0
	}

	r.mu.Lock()
	own, ok = r.ownerships[key]
	if !ok || own == nil || own.VMID != vmID || (own.State != VMStateStopped && own.State != VMStateHibernated) || r.lifecycleConflictLocked(key) != nil {
		r.mu.Unlock()
		return 0
	}
	if _, pending := r.pendingWaiters[key]; pending {
		r.mu.Unlock()
		return 0
	}
	prior := *own
	snapshot := *own
	own.State = VMStateBooting
	r.lifecycle[key] = lifecyclePlatformWarm
	r.pendingWaiters[key] = nil
	tracked := mgr.GetVM(vmID) != nil
	r.mu.Unlock()

	var info *VMInstanceInfo
	var err error
	if tracked {
		info, err = r.spawnClaimed(vmID, bootClaimResume, snapshot.Epoch, func() (*VMInstanceInfo, error) {
			return mgr.ResumeVM(vmID)
		})
	} else {
		info, err = r.startExistingVM(&snapshot, mgr)
	}

	r.mu.Lock()
	r.releaseLifecycleLocked(key, lifecyclePlatformWarm)
	waiters := r.pendingWaiters[key]
	delete(r.pendingWaiters, key)
	current, ok := r.ownerships[key]
	var result *VMOwnership
	switch {
	case !ok || current == nil || current.VMID != vmID:
	case err == nil && info != nil:
		current.ComputerURL = info.HostURL
		current.Epoch = info.Epoch
		current.State = VMStateActive
		current.LastActiveAt = time.Now()
		current.StoppedBy = ""
		publishStartLocked(current)
		r.saveLocked()
		result = cloneOwnership(current)
	case err != nil && transientStartRefusal(err):
		// Nothing spawned: the row is what it was.
		if current.State == VMStateBooting {
			current.State = prior.State
			current.StoppedBy = prior.StoppedBy
			r.saveLocked()
		}
	default:
		if current.State == VMStateBooting {
			current.State = VMStateFailed
			current.StoppedBy = stoppedByRecoveryFailed
			r.saveLocked()
		}
	}
	r.mu.Unlock()
	for _, ch := range waiters {
		if result != nil {
			ch <- cloneOwnership(result)
		} else {
			ch <- nil
		}
	}
	if result == nil {
		if err == nil {
			err = fmt.Errorf("no realization returned")
		}
		log.Printf("vmctl: resume platform computer %s: %v", vmID, err)
		return 0
	}
	return 1
}

func (r *OwnershipRegistry) ensureUniversalWirePlatformOwnership(ctx context.Context) (*VMOwnership, error) {
	key := ownershipKey(UniversalWirePlatformOwnerID, UniversalWirePlatformDesktopID)

	r.mu.Lock()
	if own, ok := r.ownerships[key]; ok && own.IsHeld() {
		log.Printf("vmctl: platform computer vm=%s refused: held", own.VMID)
		r.mu.Unlock()
		return own, nil
	}
	if own, ok := r.ownerships[key]; ok {
		switch {
		case own.IsReady():
			snapshot := *own
			mgr := r.vmManager
			if activeOwnershipNeedsReadinessCheck(&snapshot, mgr) {
				r.mu.Unlock()
				info, err := r.ensureActiveVMReady(&snapshot, mgr)
				if err != nil {
					return nil, fmt.Errorf("verify platform computer %s: %w", snapshot.VMID, err)
				}
				r.mu.Lock()
				current := r.ownerships[key]
				if current == nil {
					r.mu.Unlock()
					return r.ensureUniversalWirePlatformOwnership(ctx)
				}
				if info != nil {
					current.ComputerURL = info.HostURL
					current.Epoch = info.Epoch
				}
				current.State = VMStateActive
				current.LastActiveAt = time.Now()
				publishStartLocked(current)
				r.saveLocked()
				vmID := current.VMID
				r.mu.Unlock()
				r.ensureExistingGatewayCredential(vmID)
				return current, nil
			}
			own.LastActiveAt = time.Now()
			r.saveLocked()
			vmID := own.VMID
			r.mu.Unlock()
			r.ensureExistingGatewayCredential(vmID)
			return own, nil
		case own.State == VMStateStopped || own.State == VMStateHibernated:
			if waiters, ok := r.pendingWaiters[key]; ok {
				return r.waitForPendingAssignmentLocked(ctx, key, UniversalWirePlatformOwnerID, UniversalWirePlatformDesktopID, waiters)
			}
			if err := r.refusePlatformStartLocked(key, own); err != nil {
				r.mu.Unlock()
				return nil, err
			}
			mgr := r.vmManager
			snapshot := *own
			r.pendingWaiters[key] = nil
			own.State = VMStateBooting
			r.saveLocked()
			r.mu.Unlock()
			if mgr != nil {
				info, err := r.startExistingVM(&snapshot, mgr)
				if err != nil {
					r.mu.Lock()
					waiters := r.pendingWaiters[key]
					delete(r.pendingWaiters, key)
					if current, ok := r.ownerships[key]; ok && current != nil && current.VMID == snapshot.VMID {
						restoreOrFailPlatformLocked(current, &snapshot, err)
						r.saveLocked()
					}
					r.mu.Unlock()
					for _, ch := range waiters {
						ch <- nil
					}
					return nil, fmt.Errorf("start platform computer %s: %w", snapshot.VMID, err)
				}
				r.mu.Lock()
				waiters := r.pendingWaiters[key]
				delete(r.pendingWaiters, key)
				current := r.ownerships[key]
				if current == nil || current.VMID != snapshot.VMID {
					r.mu.Unlock()
					for _, ch := range waiters {
						ch <- nil
					}
					return nil, fmt.Errorf("platform computer ownership disappeared during resume")
				}
				if info != nil {
					current.ComputerURL = info.HostURL
					current.Epoch = info.Epoch
				}
				current.State = VMStateActive
				current.LastActiveAt = time.Now()
				current.StoppedBy = ""
				publishStartLocked(current)
				r.saveLocked()
				vmID := current.VMID
				result := cloneOwnership(current)
				r.mu.Unlock()
				r.ensureExistingGatewayCredential(vmID)
				for _, ch := range waiters {
					ch <- cloneOwnership(result)
				}
				return result, nil
			}
			r.mu.Lock()
			own.State = VMStateActive
			own.LastActiveAt = time.Now()
			r.saveLocked()
			vmID := own.VMID
			r.mu.Unlock()
			r.ensureExistingGatewayCredential(vmID)
			return own, nil
		case own.State == VMStateBooting:
			if waiters, ok := r.pendingWaiters[key]; ok {
				// Another goroutine is booting; wait for it.
				ch := make(chan *VMOwnership, 1)
				r.pendingWaiters[key] = append(waiters, ch)
				r.mu.Unlock()
				select {
				case <-ctx.Done():
					r.removePendingWaiter(key, ch)
					return nil, ctx.Err()
				case result := <-ch:
					if result == nil {
						return nil, fmt.Errorf("platform computer boot failed")
					}
					return result, nil
				}
			}

			// A persisted booting state without an in-memory pending boot can
			// survive a vmctl restart. Treat it as stale and recover it instead
			// of letting callers route to the placeholder autoputer URL forever.
			mgr := r.vmManager
			if mgr == nil {
				r.mu.Unlock()
				return nil, fmt.Errorf("no VM manager configured to recover stale booting platform computer %s", own.VMID)
			}
			snapshot := *own
			r.pendingWaiters[key] = nil
			r.mu.Unlock()

			info, err := r.ensureActiveVMReady(&snapshot, mgr)
			if err != nil {
				r.mu.Lock()
				current := r.ownerships[key]
				if current != nil && current.VMID == snapshot.VMID {
					current.State = VMStateFailed
					current.StoppedBy = stoppedByRecoveryFailed
					r.saveLocked()
				}
				waiters := r.pendingWaiters[key]
				delete(r.pendingWaiters, key)
				r.mu.Unlock()
				for _, ch := range waiters {
					ch <- nil
				}
				return nil, fmt.Errorf("recover stale booting platform computer %s: %w", snapshot.VMID, err)
			}

			r.mu.Lock()
			current := r.ownerships[key]
			if current == nil || current.VMID != snapshot.VMID {
				waiters := r.pendingWaiters[key]
				delete(r.pendingWaiters, key)
				r.mu.Unlock()
				for _, ch := range waiters {
					ch <- nil
				}
				return r.ensureUniversalWirePlatformOwnership(ctx)
			}
			if info != nil {
				current.ComputerURL = info.HostURL
				current.Epoch = info.Epoch
			}
			current.State = VMStateActive
			current.LastActiveAt = time.Now()
			current.StoppedBy = ""
			publishStartLocked(current)
			r.saveLocked()
			vmID := current.VMID
			waiters := r.pendingWaiters[key]
			delete(r.pendingWaiters, key)
			r.mu.Unlock()
			for _, ch := range waiters {
				ch <- current
			}
			r.ensureExistingGatewayCredential(vmID)
			return current, nil
		default:
			// Unknown or failed state; treat as needing recovery.
			if waiters, ok := r.pendingWaiters[key]; ok {
				return r.waitForPendingAssignmentLocked(ctx, key, UniversalWirePlatformOwnerID, UniversalWirePlatformDesktopID, waiters)
			}
			if err := r.refusePlatformStartLocked(key, own); err != nil {
				r.mu.Unlock()
				return nil, err
			}
			mgr := r.vmManager
			snapshot := *own
			r.pendingWaiters[key] = nil
			own.State = VMStateBooting
			r.saveLocked()
			r.mu.Unlock()
			info, err := r.startExistingVM(&snapshot, mgr)
			if err != nil {
				r.mu.Lock()
				waiters := r.pendingWaiters[key]
				delete(r.pendingWaiters, key)
				if current, ok := r.ownerships[key]; ok && current != nil && current.VMID == snapshot.VMID {
					restoreOrFailPlatformLocked(current, &snapshot, err)
					r.saveLocked()
				}
				r.mu.Unlock()
				for _, ch := range waiters {
					ch <- nil
				}
				return nil, fmt.Errorf("recover platform computer %s: %w", snapshot.VMID, err)
			}
			r.mu.Lock()
			waiters := r.pendingWaiters[key]
			delete(r.pendingWaiters, key)
			current := r.ownerships[key]
			if current == nil || current.VMID != snapshot.VMID {
				r.mu.Unlock()
				for _, ch := range waiters {
					ch <- nil
				}
				return nil, fmt.Errorf("platform computer ownership disappeared during recovery")
			}
			current.ComputerURL = info.HostURL
			current.Epoch = info.Epoch
			current.State = VMStateActive
			current.LastActiveAt = time.Now()
			current.StoppedBy = ""
			publishStartLocked(current)
			r.saveLocked()
			vmID := current.VMID
			result := cloneOwnership(current)
			r.mu.Unlock()
			r.ensureExistingGatewayCredential(vmID)
			for _, ch := range waiters {
				ch <- cloneOwnership(result)
			}
			return result, nil
		}
	}
	// No existing ownership — create one.
	vmID := UniversalWirePlatformVMID
	mgr := r.vmManager
	if mgr == nil {
		return nil, fmt.Errorf("no VM manager configured for %s", UniversalWirePlatformOwnerID)
	}
	r.pendingWaiters[key] = nil
	r.mu.Unlock()

	platformOwn := &VMOwnership{
		UserID:        UniversalWirePlatformOwnerID,
		DesktopID:     UniversalWirePlatformDesktopID,
		VMID:          vmID,
		Kind:          VMKindInteractive,
		WarmnessClass: WarmnessClassPublicPlatform,
	}
	if refusal := r.admitRecoveryStart(platformOwn, false); refusal != nil {
		r.mu.Lock()
		waiters := r.pendingWaiters[key]
		delete(r.pendingWaiters, key)
		r.mu.Unlock()
		for _, ch := range waiters {
			ch <- nil
		}
		return nil, refusal
	}
	// The persisted booting row is the first boot's durable start claim: a
	// vmctl restart mid-boot loads it and closes the start
	// (boot_claim.go). Without it the restart left a process no row
	// described (review round 2, F3).
	booting := &VMOwnership{
		UserID:        UniversalWirePlatformOwnerID,
		DesktopID:     UniversalWirePlatformDesktopID,
		VMID:          vmID,
		Kind:          VMKindInteractive,
		WarmnessClass: WarmnessClassPublicPlatform,
		State:         VMStateBooting,
		CreatedAt:     time.Now(),
		LastActiveAt:  time.Now(),
	}
	r.mu.Lock()
	if r.hostStopping.Load() {
		// A first boot is a spawn too (spawnClaimed gates the others).
		waiters := r.pendingWaiters[key]
		delete(r.pendingWaiters, key)
		r.mu.Unlock()
		for _, ch := range waiters {
			ch <- nil
		}
		return nil, errHostStopping
	}
	if _, taken := r.ownerships[key]; taken {
		waiters := r.pendingWaiters[key]
		delete(r.pendingWaiters, key)
		r.mu.Unlock()
		for _, ch := range waiters {
			ch <- nil
		}
		return nil, fmt.Errorf("platform computer %s ownership appeared during its first boot", vmID)
	}
	r.ownerships[key] = booting
	r.vmByID[vmID] = booting
	if err := r.writePersistenceLocked(); err != nil {
		delete(r.ownerships, key)
		delete(r.vmByID, vmID)
		waiters := r.pendingWaiters[key]
		delete(r.pendingWaiters, key)
		r.mu.Unlock()
		for _, ch := range waiters {
			ch <- nil
		}
		return nil, fmt.Errorf("persist first boot of platform computer %s: %w", vmID, err)
	}
	r.mu.Unlock()
	info, err := mgr.BootVM(vmManagerConfigForOwnership(platformOwn, issueGatewayTokenAt(r.gatewayURL, vmID)))
	if err != nil {
		r.mu.Lock()
		waiters := r.pendingWaiters[key]
		delete(r.pendingWaiters, key)
		// A failed first boot leaves no row, as before: the next ensure
		// boots it again.
		if r.ownerships[key] == booting {
			delete(r.ownerships, key)
			if r.vmByID[vmID] == booting {
				delete(r.vmByID, vmID)
			}
		}
		r.saveLocked()
		r.mu.Unlock()
		for _, ch := range waiters {
			ch <- nil
		}
		return nil, fmt.Errorf("boot platform computer %s: %w", vmID, r.noteRecoveryStartFailure(platformOwn, err))
	}
	r.clearRecoveryCondition(platformOwn)
	r.mu.Lock()
	own := booting
	own.ComputerURL = info.HostURL
	own.Epoch = info.Epoch
	own.State = VMStateActive
	own.LastActiveAt = time.Now()
	r.ownerships[key] = own
	r.vmByID[vmID] = own
	waiters := r.pendingWaiters[key]
	delete(r.pendingWaiters, key)
	r.saveLocked()
	r.mu.Unlock()
	r.transitionVM(vmID, VMStateActive)
	for _, ch := range waiters {
		ch <- own
	}
	r.ensureExistingGatewayCredential(vmID)
	log.Printf("vmctl: booted platform computer %s at %s (epoch=%d)", vmID, info.HostURL, info.Epoch)
	return own, nil
}

// refusePlatformStartLocked refuses a platform start before it touches the
// row: another path holds the computer's lifecycle claim, an interrupted
// start's close is open, or a start that keeps crashing vmctl is inside its
// charged window (boot_claim.go). Callers hold r.mu.
func (r *OwnershipRegistry) refusePlatformStartLocked(key string, own *VMOwnership) error {
	if err := r.lifecycleConflictLocked(key); err != nil {
		return err
	}
	if _, open := r.interruptedStarts[own.VMID]; open {
		return errStartInterruptedOpen(own.VMID)
	}
	if r.repeatedInterruptionBoundLocked(own) {
		return fmt.Errorf("platform computer %s: its starts were interrupted by vmctl restarts %d times in a row (stopped_by=%s); the automatic retry is spent until %s after %s, or until a deploy of a new target re-arms it and the deploy rescue boots it", own.VMID, own.StartInterruptions, own.StoppedBy, failureWarmWindow, own.FailureWarmedAt)
	}
	return nil
}

// restoreOrFailPlatformLocked settles a platform start that did not publish:
// a refusal that spawned nothing leaves the row as it was; anything else
// fails it, as before.
func restoreOrFailPlatformLocked(current, prior *VMOwnership, err error) {
	if transientStartRefusal(err) {
		current.State = prior.State
		current.StoppedBy = prior.StoppedBy
		return
	}
	current.State = VMStateFailed
}
