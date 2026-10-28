package vmmanager

// S5 busy hibernate on a planned host stop
// (docs/problems/paper-never-checkpointed-2026-10-27.md, slice S5).
//
// A host stop (vmctl's exit at a host shutdown, or with
// VMCTL_STOP_MANAGED_ON_EXIT) used to kill every running guest, which is a
// crash: the next boot closes all open work. With HibernateOptions.HostStop
// the snapshot hibernate still asks the guest to fence first. A guest that
// fences is captured as any idle hibernate. A guest that answers busy is
// captured without the fence, when every rule below holds:
//
//   - the reply is the guest's ordinary busy outcome, with no reason (a
//     reason such as planned_restart_pending means a planned-restart marker
//     is on disk, and a resumed guest that crashed in its window would boot
//     planned and resume: refused);
//   - no self-development operation is active (selfdev_active_operations);
//     vmctl also refuses a live engineering capsule before it asks (its
//     /health live_capsules), since prepare does not count capsules;
//   - the guest runs the current guest release. A snapshot of any other
//     release would be refused at wake, so it is not written.
//
// Otherwise the guest is left running, exactly as a busy idle hibernate is,
// and the host stop takes it down as today: a crash, closed at the next boot.
//
// At wake a busy snapshot loads only onto the release it was captured from
// (busySnapshotRefusal): a guest code update cold boots instead, and that
// cold boot is a crash boot unless a planned-restart marker is present. It
// is consumed before it loads, as every snapshot, so it resumes at most once:
// a resume that crashes leads to a cold crash boot, never a second resume.
//
// The resume continues the same guest process; it is not a boot, so no
// restart pass runs. Work that was mid-call continues, and calls whose
// connections died with the host fail as ordinary errors.
//
// Owner, 2026-10-28: on by default for every computer, protected classes
// included, with no off switch. The refusals above are correctness rules.

import (
	"fmt"
	"time"
)

// DefaultBusyHibernateMaxAge bounds how long a busy snapshot stays
// resumable. A busy guest's capability was not renewed at the fence, so a
// late resume is likelier to fail its renewal; a host stop is minutes.
const DefaultBusyHibernateMaxAge = time.Hour

const (
	// Wake refusals of a busy snapshot.
	snapshotReasonBusyReleaseChanged = "busy_snapshot_release_changed"
	snapshotReasonBusyMaxAge         = "busy_snapshot_max_age_passed"

	// Why a host stop left a busy guest running (SnapshotTimeline.BusyRefused).
	busyRefusedSelfdev = "selfdev_active"
	busyRefusedReason  = "busy_reason_not_runs"
	busyRefusedRelease = "release_not_current"
)

// HibernateVMForHostStop is a snapshot hibernate for a planned host stop: a
// guest busy only with runs is captured without the fence. Only vmctl's
// host-stop pass calls it.
func (m *Manager) HibernateVMForHostStop(vmID string) error {
	return m.HibernateVMWithOptions(vmID, HibernateOptions{HostStop: true})
}

func (m *Manager) busyHibernateMaxAge() time.Duration {
	if m.cfg.BusyHibernateMaxAge > 0 {
		return m.cfg.BusyHibernateMaxAge
	}
	return DefaultBusyHibernateMaxAge
}

// busyCaptureRefusal returns "" when a guest that answered busy at the fence
// may be captured for a host stop, else why not.
func (m *Manager) busyCaptureRefusal(cfg VMConfig, resp guestHibernatePrepareResponse) string {
	if resp.SelfdevActiveOps > 0 {
		return busyRefusedSelfdev
	}
	if resp.Outcome != "busy" || resp.Reason != "" {
		return busyRefusedReason
	}
	if cfg.GuestRelease == "" || cfg.GuestRelease != m.currentReleaseID() {
		return busyRefusedRelease
	}
	return ""
}

// busySnapshotRefusal is validateSnapshot's extra rule for a busy snapshot:
// it loads only onto its own, still current, release, within its own age.
func (m *Manager) busySnapshotRefusal(meta *SnapshotMeta, now time.Time) string {
	if !meta.BusyHostStop {
		return ""
	}
	if !now.Before(meta.CreatedAt.Add(m.busyHibernateMaxAge())) {
		return snapshotReasonBusyMaxAge
	}
	if meta.GuestRelease == "" || meta.GuestRelease != m.currentReleaseID() {
		return snapshotReasonBusyReleaseChanged
	}
	return ""
}

// syncBusyDataImage flushes the guest's data image after Firecracker stopped.
// A fenced guest synced its own filesystems; a busy one did not, so the
// writes it had already handed to the VMM are made durable here, before the
// host stops.
func (m *Manager) syncBusyDataImage(vmID string) error {
	path := m.snapshotBoundPaths(vmID, VMConfig{})[0]
	if err := syncRegularFile(path); err != nil {
		return fmt.Errorf("sync the data image of busy VM %s: %w", vmID, err)
	}
	return nil
}
