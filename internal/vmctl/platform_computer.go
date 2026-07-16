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
// computer. It returns an error if the platform computer could not be made
// ready. Dispatch routing is handled by the sandbox proxy (UDS) — callers
// no longer need the sandbox URL directly.
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
func (r *OwnershipRegistry) WarmUniversalWirePlatformComputer(ctx context.Context, guard ComputerVersionRouteGuard) int {
	key := ownershipKey(UniversalWirePlatformOwnerID, UniversalWirePlatformDesktopID)
	r.mu.RLock()
	own, ok := r.ownerships[key]
	r.mu.RUnlock()
	if !ok || own == nil || own.IsReady() {
		return 0
	}
	if own.State == VMStateStopped || own.State == VMStateHibernated {
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
		if !ok || own == nil || own.IsReady() {
			r.mu.Unlock()
			return 0
		}
		snapshot := *own
		snapshot.State = VMStateBooting
		r.ownerships[key] = &snapshot
		mgr := r.vmManager
		r.mu.Unlock()
		if mgr != nil {
			info, err := mgr.ResumeVM(snapshot.VMID)
			if err != nil {
				log.Printf("vmctl: resume platform computer %s: %v", snapshot.VMID, err)
				return 0
			}
			r.mu.Lock()
			current, ok := r.ownerships[key]
			if ok && current != nil {
				current.SandboxURL = info.HostURL
				current.Epoch = info.Epoch
				current.State = VMStateActive
				current.LastActiveAt = time.Now()
				r.saveLocked()
			}
			r.mu.Unlock()
			return 1
		}
	}
	return 0
}

func (r *OwnershipRegistry) ensureUniversalWirePlatformOwnership(ctx context.Context) (*VMOwnership, error) {
	key := ownershipKey(UniversalWirePlatformOwnerID, UniversalWirePlatformDesktopID)

	r.mu.Lock()
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
					current.SandboxURL = info.HostURL
					current.Epoch = info.Epoch
				}
				current.State = VMStateActive
				current.LastActiveAt = time.Now()
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
			mgr := r.vmManager
			snapshot := *own
			r.mu.Unlock()
			if mgr != nil {
				info, err := r.startExistingVM(&snapshot, mgr)
				if err != nil {
					return nil, fmt.Errorf("start platform computer %s: %w", snapshot.VMID, err)
				}
				r.mu.Lock()
				current := r.ownerships[key]
				if current == nil {
					r.mu.Unlock()
					return nil, fmt.Errorf("platform computer ownership disappeared during resume")
				}
				current.SandboxURL = info.HostURL
				current.Epoch = info.Epoch
				current.State = VMStateActive
				current.LastActiveAt = time.Now()
				r.saveLocked()
				vmID := current.VMID
				r.mu.Unlock()
				r.ensureExistingGatewayCredential(vmID)
				return current, nil
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
			// of letting callers route to the placeholder sandbox URL forever.
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
					current.StoppedBy = "recovery_failed"
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
				current.SandboxURL = info.HostURL
				current.Epoch = info.Epoch
			}
			current.State = VMStateActive
			current.LastActiveAt = time.Now()
			current.StoppedBy = ""
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
			mgr := r.vmManager
			snapshot := *own
			r.mu.Unlock()
			info, err := r.startExistingVM(&snapshot, mgr)
			if err != nil {
				return nil, fmt.Errorf("recover platform computer %s: %w", snapshot.VMID, err)
			}
			r.mu.Lock()
			current := r.ownerships[key]
			if current == nil {
				r.mu.Unlock()
				return nil, fmt.Errorf("platform computer ownership disappeared during recovery")
			}
			current.SandboxURL = info.HostURL
			current.Epoch = info.Epoch
			current.State = VMStateActive
			current.LastActiveAt = time.Now()
			r.saveLocked()
			vmID := current.VMID
			r.mu.Unlock()
			r.ensureExistingGatewayCredential(vmID)
			return current, nil
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

	info, err := mgr.BootVM(vmManagerConfigForOwnership(&VMOwnership{
		UserID:        UniversalWirePlatformOwnerID,
		DesktopID:     UniversalWirePlatformDesktopID,
		VMID:          vmID,
		Kind:          VMKindInteractive,
		WarmnessClass: WarmnessClassPublicPlatform,
		Published:     true,
	}, issueGatewayTokenAt(r.gatewayURL, vmID)))
	if err != nil {
		r.mu.Lock()
		waiters := r.pendingWaiters[key]
		delete(r.pendingWaiters, key)
		r.saveLocked()
		r.mu.Unlock()
		for _, ch := range waiters {
			ch <- nil
		}
		return nil, fmt.Errorf("boot platform computer %s: %w", vmID, err)
	}
	r.mu.Lock()
	own := &VMOwnership{
		UserID:        UniversalWirePlatformOwnerID,
		DesktopID:     UniversalWirePlatformDesktopID,
		VMID:          vmID,
		SandboxURL:    info.HostURL,
		Epoch:         info.Epoch,
		Kind:          VMKindInteractive,
		WarmnessClass: WarmnessClassPublicPlatform,
		Published:     true,
		State:         VMStateActive,
		CreatedAt:     time.Now(),
		LastActiveAt:  time.Now(),
	}
	r.ownerships[key] = own
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
