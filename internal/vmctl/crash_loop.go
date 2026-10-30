package vmctl

import (
	"fmt"
	"log"
	"strings"
	"time"
)

// The crash-loop bound (CLAUDE.md "Restarts End Work (Crash) Or Resume It
// (Planned Update)"). A crash restart never resumes work: the next boot has no
// planned-restart marker, so it closes every open obligation with the fate
// "interrupted by a restart". The owner never asked that a crashed computer
// stay down, so the next request restarts it (resolve already cold-boots a
// degraded or failed computer; the bearer wake and the compute-panel wake now
// reach it too). What the owner did ask for is no crash-restart loop, so vmctl
// records every crash and holds a computer that crashed crashLoopMaxCrashes
// times inside crashLoopWindow until an owner start.
//
// The hold is the existing durable maintenance hold with HeldBy
// CrashLoopHeldBy: every automatic lifecycle path already refuses a held
// computer (resolve, recover, refresh, rescue, the always-on and platform
// warms), so the bound needs no second set of refusals. An owner start clears
// the history and only a hold this file set; an operator's maintenance hold is
// never cleared by it.

const (
	// crashLoopMaxCrashes crashes inside crashLoopWindow hold the computer.
	// These two numbers are the whole policy; change them here.
	crashLoopMaxCrashes = 3
	crashLoopWindow     = 30 * time.Minute

	// crashHistoryCap bounds the persisted history. Entries older than the
	// window are dropped on every write, so it is only a guard.
	crashHistoryCap = 16

	// CrashLoopHeldBy marks the hold the bound sets.
	CrashLoopHeldBy = "vmctl:crash-loop"

	// RecoveryRefusalCrashLoop refuses every automatic start of a held
	// crash-looping computer. Waiting does not clear it; only an owner start.
	RecoveryRefusalCrashLoop RecoveryRefusalKind = "crash_loop"
	// RecoveryRefusalGuestWorking refuses the automatic restart of a degraded
	// computer whose guest still answers and reports a self-development apply
	// or a live capsule in flight (the readings a planned host stop reads). It
	// spawned nothing and clears by itself.
	RecoveryRefusalGuestWorking RecoveryRefusalKind = "guest_working"

	crashLoopRetryAfterSeconds    = 300
	guestWorkingRetryAfterSeconds = 30
	crashCauseMax                 = 200

	// The kinds of crash a CrashRecord names.
	crashKindUnhealthy  = "guest_unhealthy"
	crashKindWedged     = "guest_wedged"
	crashKindBootFailed = "boot_failed"
)

// CrashRecord is one recorded crash: a running guest latched degraded, a
// wedged guest vmctl had to stop, or a restart whose boot failed.
type CrashRecord struct {
	// At is when vmctl recorded it (RFC3339Nano).
	At string `json:"at"`
	// Epoch is the realization that crashed. A realization is one crash,
	// however often it is seen unhealthy. Zero for a failed boot, which has
	// no realization.
	Epoch int64  `json:"epoch,omitempty"`
	Kind  string `json:"kind"`
	Cause string `json:"cause,omitempty"`
}

func crashLoopReason() string {
	return fmt.Sprintf("crash loop: %d crashes in %dm", crashLoopMaxCrashes, int(crashLoopWindow/time.Minute))
}

// crashLoopHeld reports a computer held by the crash-loop bound.
func crashLoopHeld(own *VMOwnership) bool {
	return own != nil && own.HoldStatus != nil && own.HoldStatus.HeldBy == CrashLoopHeldBy
}

func crashLoopRefusal(own *VMOwnership) *RecoveryRefusal {
	reason := crashLoopReason()
	if own.HoldStatus != nil && strings.TrimSpace(own.HoldStatus.Reason) != "" {
		reason = own.HoldStatus.Reason
	}
	return &RecoveryRefusal{
		ComputerID:        own.ComputerID,
		Kind:              RecoveryRefusalCrashLoop,
		Reason:            reason,
		RetryAfterSeconds: crashLoopRetryAfterSeconds,
	}
}

// crashesInWindow keeps the records inside the window ending now; a record
// whose time cannot be read is dropped.
func crashesInWindow(records []CrashRecord, now time.Time) []CrashRecord {
	kept := make([]CrashRecord, 0, len(records)+1)
	for _, record := range records {
		at, err := time.Parse(time.RFC3339Nano, record.At)
		if err != nil || now.Sub(at) >= crashLoopWindow {
			continue
		}
		kept = append(kept, record)
	}
	return kept
}

// recordCrashLocked records one crash of own and, on the crashLoopMaxCrashes-th
// inside the window, holds the computer. It reports whether this crash set the
// hold. A realization (epoch) is recorded once. The caller holds r.mu and
// saves.
func (r *OwnershipRegistry) recordCrashLocked(own *VMOwnership, kind, cause string) bool {
	if own == nil {
		return false
	}
	now := r.now()
	records := crashesInWindow(own.Crashes, now)
	epoch := int64(0)
	if kind != crashKindBootFailed {
		epoch = own.Epoch
		if epoch > 0 {
			for _, record := range records {
				if record.Kind != crashKindBootFailed && record.Epoch == epoch {
					own.Crashes = records
					return false
				}
			}
		}
	}
	cause = strings.TrimSpace(cause)
	if len(cause) > crashCauseMax {
		cause = cause[:crashCauseMax]
	}
	records = append(records, CrashRecord{At: now.UTC().Format(time.RFC3339Nano), Epoch: epoch, Kind: kind, Cause: cause})
	if len(records) > crashHistoryCap {
		records = records[len(records)-crashHistoryCap:]
	}
	own.Crashes = records
	log.Printf("vmctl: computer %s (VM %s) crashed (%s, epoch %d): %s; %d of %d inside %s",
		own.ComputerID, own.VMID, kind, own.Epoch, cause, len(records), crashLoopMaxCrashes, crashLoopWindow)
	if len(records) < crashLoopMaxCrashes || own.HoldStatus != nil {
		return false
	}
	own.HoldStatus = &MaintenanceHold{Reason: crashLoopReason(), HeldBy: CrashLoopHeldBy, HeldAt: now.UTC()}
	log.Printf("vmctl: computer %s (VM %s) held: %s; it is not restarted again until an owner start", own.ComputerID, own.VMID, own.HoldStatus.Reason)
	return true
}

// clearCrashLoopLocked drops the crash history and a hold the bound set. A
// maintenance hold an operator set stays. It reports whether anything changed.
func clearCrashLoopLocked(own *VMOwnership) bool {
	if own == nil {
		return false
	}
	changed := len(own.Crashes) > 0
	own.Crashes = nil
	if crashLoopHeld(own) {
		own.HoldStatus = nil
		changed = true
	}
	return changed
}

// ClearCrashLoopForDesktop is the owner start's effect on the bound: the
// crash history is forgotten and a crash-loop hold is cleared, so the start
// that follows boots the computer and the next crash is crash one again.
func (r *OwnershipRegistry) ClearCrashLoopForDesktop(userID, desktopID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	own := r.ownerships[ownershipKey(userID, desktopID)]
	if own == nil || !clearCrashLoopLocked(own) {
		return false
	}
	r.saveLocked()
	log.Printf("vmctl: owner start cleared the crash history and any crash-loop hold of computer %s (VM %s)", own.ComputerID, own.VMID)
	return true
}

// noteExitedActiveVM records the crash a resolve finds in an active computer
// whose guest process has exited (the manager's instance is failed): the
// browser path meets crashes here, not at the lookup latch. When the crash
// trips the bound, or the computer is already held by it, the computer is
// latched degraded so every later request meets the hold, and the refusal is
// returned instead of a restart. A stale snapshot (the realization changed) is
// not recorded.
func (r *OwnershipRegistry) noteExitedActiveVM(own *VMOwnership, managerState string) *RecoveryRefusal {
	if own == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cur := r.ownerships[ownershipKey(own.UserID, own.DesktopID)]
	if cur == nil || cur.VMID != own.VMID || cur.Epoch != own.Epoch {
		return nil
	}
	r.recordCrashLocked(cur, crashKindUnhealthy, "the guest process exited (manager state "+managerState+")")
	if crashLoopHeld(cur) {
		cur.State = VMStateDegraded
		r.saveLocked()
		return crashLoopRefusal(cur)
	}
	r.saveLocked()
	return nil
}

// guestCausedBootFailure reports a boot that failed because the guest did not
// come up: the process exited before readiness, it never became healthy, or
// its replay stalled (vmmanager.waitForGuestReady). Recognized by text, as
// liveUnmanagedBootRefusal does, because vmctl does not import vmmanager. A
// host or control-plane failure (no credential, no epoch, no tap, no room) is
// not the guest crashing, and an outage of one must not hold every computer
// that happens to be restarted during it.
func guestCausedBootFailure(err error) bool {
	if err == nil {
		return false
	}
	text := err.Error()
	for _, marker := range []string{
		"guest process exited before readiness",
		"guest did not become healthy",
		"guest replay stalled",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

// noteCrashRestartFailure records a restart of a crashed computer whose boot
// failed because the guest did not come up (a crash during boot). A refusal
// that spawned nothing, a durable recovery refusal (its own condition bounds
// those), a host or control-plane failure and a canceled request are not
// crashes.
func (r *OwnershipRegistry) noteCrashRestartFailure(key, vmID string, err error) {
	if !guestCausedBootFailure(err) || transientStartRefusal(err) || RecoveryRefusalFrom(err) != nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	own := r.ownerships[key]
	if own == nil || own.VMID != vmID {
		return
	}
	r.recordCrashLocked(own, crashKindBootFailed, "restart failed: "+err.Error())
	r.saveLocked()
}

// crashRestartRefusal is the automatic restart's last look at a degraded
// computer, outside r.mu: a guest that still answers /health and
// reports a self-development apply or a live capsule is working, not dead,
// and is not restarted (the same readings a planned host stop uses). A guest
// that does not answer is what a crash looks like, and is restarted.
func (r *OwnershipRegistry) crashRestartRefusal(own *VMOwnership) *RecoveryRefusal {
	r.mu.RLock()
	probe := r.hostStopHealthProbe
	r.mu.RUnlock()
	if probe == nil {
		probe = httpHostStopHealth()
	}
	health := probe(own)
	if !health.Answered {
		return nil
	}
	if health.SelfdevActive > 0 || (health.LiveCapsules != nil && *health.LiveCapsules > 0) {
		return &RecoveryRefusal{
			ComputerID:        stableComputerID(own.UserID, own.DesktopID, own.ComputerID),
			Kind:              RecoveryRefusalGuestWorking,
			Reason:            "the computer answers and has a self-development apply or a live capsule in flight; it is not restarted",
			RetryAfterSeconds: guestWorkingRetryAfterSeconds,
		}
	}
	return nil
}
