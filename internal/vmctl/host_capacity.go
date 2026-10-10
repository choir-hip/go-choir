package vmctl

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
)

// Host capacity guards. Node B ran out of memory on 2026-10-10 because
// Firecracker processes that a vmctl restart failed to reattach kept running
// outside the registry, invisible to pressure reclaim, the idle sweeper and
// stop; new computers kept booting onto the starved host; and every VM
// inherited vmctl's OOM protection
// (docs/problems/node-b-oom-from-retained-qa-computers-2026-10-10.md).

const (
	// RecoveryRefusalHostMemoryPressure refuses a start that would add a VM
	// to a host already below its memory floor. It is transient: never
	// recorded as a durable recovery condition.
	RecoveryRefusalHostMemoryPressure RecoveryRefusalKind = "host_memory_pressure"
	// RecoveryRefusalGuestReattachPending: a live guest vmctl is not tracking
	// did not answer reattach; resolve waits instead of rebooting it.
	RecoveryRefusalGuestReattachPending RecoveryRefusalKind = "guest_reattach_pending"
	hostMemoryRetryAfterSeconds                             = 60

	// The kernel kills the highest oom_score first. vmctl runs at -900
	// (nix/node-b.nix); protected computers rank below ordinary ones, so a
	// QA computer is the first VM to go and the owner computer the last.
	oomScoreAdjProtected = -400
	oomScoreAdjOrdinary  = 500
)

// vmProcessController is implemented by the Firecracker manager. It lets
// the registry see every VM process on the host, not only the ones it
// tracks, and power off the ones nothing owns.
type vmProcessController interface {
	// LiveFirecrackerVMs maps VM id to the Firecracker pids running it.
	LiveFirecrackerVMs() map[string][]int
	// ReapUnmanagedVM powers off a VM the manager does not track, keeping
	// its state directory, and fails if the process survives.
	ReapUnmanagedVM(vmID string) error
	// SetVMOOMScoreAdj sets the kernel OOM priority of the VM's processes.
	SetVMOOMScoreAdj(vmID string, adj int) error
}

// VMProcessReconcileResult summarizes one reconcile pass.
type VMProcessReconcileResult struct {
	Live               int
	Managed            int
	Reattached         int
	Reaped             int
	Pending            int
	ProtectedUnmanaged int
	Wedged             int
}

func (r *OwnershipRegistry) now() time.Time {
	if r.clock != nil {
		return r.clock()
	}
	return time.Now()
}

func protectedWarmnessClass(class WarmnessClass) bool {
	switch class {
	case WarmnessClassPremiumAlwaysOn, WarmnessClassCriticalProtected, WarmnessClassPublicPlatform:
		return true
	}
	return false
}

// ReconcileVMProcesses makes the registry account for every Firecracker
// process on the host. A process the manager tracks gets its class's OOM
// priority. A process behind a stopped ownership is reattached when its
// guest answers; otherwise, unless the computer is protected or held, it is
// powered off after the grace period. A process with no ownership at all is
// powered off on its second sighting.
func (r *OwnershipRegistry) ReconcileVMProcesses(ctx context.Context, guard ComputerVersionRouteGuard) VMProcessReconcileResult {
	var res VMProcessReconcileResult
	r.mu.RLock()
	mgr := r.vmManager
	r.mu.RUnlock()
	pc, ok := mgr.(vmProcessController)
	if !ok || pc == nil {
		return res
	}
	live := pc.LiveFirecrackerVMs()
	vmIDs := make([]string, 0, len(live))
	for vmID := range live {
		vmIDs = append(vmIDs, vmID)
	}
	sort.Strings(vmIDs)
	now := r.now()
	seen := make(map[string]bool, len(vmIDs))

	for _, vmID := range vmIDs {
		res.Live++
		r.mu.RLock()
		own := cloneOwnership(r.vmByID[vmID])
		class := WarmnessClassPrimary
		if own != nil {
			class = warmnessClassForOwnership(own, r.warmnessPolicy)
		}
		r.mu.RUnlock()

		if info := mgr.GetVM(vmID); info != nil {
			res.Managed++
			// A tracked running guest that refuses connections for the
			// wedge window is stopped; a booting one is never probed.
			if own != nil && (own.State == VMStateActive || own.State == VMStateDegraded) && !own.IsHeld() &&
				strings.EqualFold(strings.TrimSpace(info.State), "running") &&
				r.guestWedged(vmID, firstNonEmpty(info.HostURL, own.ComputerURL), now) {
				r.stopWedgedManaged(own, &res)
				continue
			}
			adj := oomScoreAdjOrdinary
			if protectedWarmnessClass(class) {
				adj = oomScoreAdjProtected
			}
			if err := pc.SetVMOOMScoreAdj(vmID, adj); err != nil {
				log.Printf("vmctl: set oom_score_adj=%d for VM %s: %v", adj, vmID, err)
			}
			continue
		}
		if own != nil && (own.State == VMStateActive || own.State == VMStateDegraded || own.State == VMStateBooting) {
			// The registry believes it is running; a start or recovery owns
			// this transition. Never race it.
			res.Pending++
			continue
		}

		seen[vmID] = true
		r.mu.Lock()
		first, known := r.unmanagedSince[vmID]
		if !known {
			r.unmanagedSince[vmID] = now
			first = now
		}
		r.mu.Unlock()

		if own == nil {
			if !known {
				res.Pending++
				continue
			}
			r.reapOrphan(pc, vmID, &res)
			continue
		}
		if own.State == VMStateStopped && own.StoppedBy == "vmctl-restart" && strings.TrimSpace(own.ComputerURL) != "" &&
			authorizeLifecycleRoute(ctx, guard, own.UserID, normalizeDesktopID(own.DesktopID)) && r.reattachOwnership(*own, mgr) {
			res.Reattached++
			delete(seen, vmID)
			continue
		}
		if !own.IsHeld() && r.guestWedged(vmID, own.ComputerURL, now) {
			// Protection covers a guest that may be working, not one that
			// refuses every connection: the bound on protection is evidence.
			r.stopWedgedUnmanaged(pc, own, &res)
			continue
		}
		// A live guest behind an ownership is a running computer, not a leak:
		// it stays until it reattaches, its owner stops it, or it is wedged
		// (docs/vmctl-360-review-2026-10-10.md, Phase 0 step 3). A grace
		// period is not evidence; admission's memory floor keeps the host
		// from booting past what these hold.
		if own.IsHeld() || protectedWarmnessClass(class) {
			res.ProtectedUnmanaged++
			log.Printf("vmctl: protected VM %s (%s) runs outside the registry; reattach pending", vmID, class)
			continue
		}
		res.Pending++
		log.Printf("vmctl: VM %s (user %s) runs outside the registry for %s; reattach pending", vmID, own.UserID, now.Sub(first).Round(time.Second))
	}

	r.forgetWedgeProbes(live)
	r.mu.Lock()
	for vmID := range r.unmanagedSince {
		if !seen[vmID] {
			delete(r.unmanagedSince, vmID)
		}
	}
	r.mu.Unlock()
	if res.Reattached > 0 {
		go r.ReconcileReadyGatewayCredentials()
	}
	if res.Reaped > 0 || res.Reattached > 0 || res.ProtectedUnmanaged > 0 || res.Wedged > 0 {
		log.Printf("vmctl: process reconcile live=%d managed=%d reattached=%d reaped=%d pending=%d protected_unmanaged=%d wedged=%d",
			res.Live, res.Managed, res.Reattached, res.Reaped, res.Pending, res.ProtectedUnmanaged, res.Wedged)
	}
	return res
}

// reapOrphan powers off a Firecracker process with no ownership at all, on
// its second sighting: no computer claims it, so nothing running is lost.
func (r *OwnershipRegistry) reapOrphan(pc vmProcessController, vmID string, res *VMProcessReconcileResult) {
	if err := pc.ReapUnmanagedVM(vmID); err != nil {
		log.Printf("vmctl: reap orphaned VM %s failed: %v", vmID, err)
		return
	}
	res.Reaped++
	r.mu.Lock()
	delete(r.unmanagedSince, vmID)
	r.mu.Unlock()
	log.Printf("vmctl: powered off orphaned VM process %s with no ownership; state kept", vmID)
}

// reapUnmanagedLocked powers off a live process behind an ownership the
// manager does not track, so stop and logout only report success once the
// VM is actually gone. Caller holds r.mu.
func (r *OwnershipRegistry) reapUnmanagedLocked(own *VMOwnership) error {
	if own == nil || r.vmManager == nil || r.vmManager.GetVM(own.VMID) != nil {
		return nil
	}
	pc, ok := r.vmManager.(vmProcessController)
	if !ok || pc == nil {
		return nil
	}
	if len(pc.LiveFirecrackerVMs()[own.VMID]) == 0 {
		return nil
	}
	if err := pc.ReapUnmanagedVM(own.VMID); err != nil {
		return fmt.Errorf("stop VM %s: unmanaged process still running: %w", own.VMID, err)
	}
	delete(r.unmanagedSince, own.VMID)
	log.Printf("vmctl: powered off unmanaged VM %s on stop", own.VMID)
	return nil
}

// admitHostMemory refuses a start that would add a VM to a host whose
// available memory would fall below the pressure floor. Protected computers
// are always admitted. Only active pressure reclaim enables it.
func (r *OwnershipRegistry) admitHostMemory(class WarmnessClass) error {
	r.mu.RLock()
	err := r.admitHostMemoryLocked(class)
	r.mu.RUnlock()
	return err
}

func (r *OwnershipRegistry) admitHostMemoryLocked(class WarmnessClass) error {
	if protectedWarmnessClass(class) {
		return nil
	}
	cfg := normalizePressureReclaimConfig(r.pressureReclaim)
	if cfg.Mode != PressureReclaimModeActive {
		return nil
	}
	sampler := r.pressureSampler
	if sampler == nil {
		sampler = sampleHostPressure
	}
	sample := sampler(cfg)
	if sample.MemoryAvailableBytes == 0 {
		return nil // unknown; never refuse on a missing sample
	}
	floor := cfg.MinMemoryAvailableBytes
	if cfg.MinMemoryAvailablePercent > 0 && sample.MemoryTotalBytes > 0 {
		if pct := uint64(float64(sample.MemoryTotalBytes) * cfg.MinMemoryAvailablePercent / 100); pct > floor {
			floor = pct
		}
	}
	_, memMib := interactiveMachineShape()
	need := uint64(memMib) * 1024 * 1024
	if sample.MemoryAvailableBytes >= floor+need {
		return nil
	}
	return &RecoveryRefusal{
		Kind: RecoveryRefusalHostMemoryPressure,
		Reason: fmt.Sprintf("host memory available %d MiB; a new computer needs %d MiB above the %d MiB floor",
			sample.MemoryAvailableBytes>>20, need>>20, floor>>20),
		RetryAfterSeconds: hostMemoryRetryAfterSeconds,
	}
}

const guestReattachRetryAfterSeconds = 15

// liveUnmanagedProcess reports whether a Firecracker process for vmID is
// running although the VM manager does not track it.
func (r *OwnershipRegistry) liveUnmanagedProcess(vmID string, mgr VMManager) bool {
	pc, ok := mgr.(vmProcessController)
	if !ok {
		return false
	}
	_, live := pc.LiveFirecrackerVMs()[vmID]
	return live
}
