package vmmanager

// The launch record and the image-moved guard (D2 slice S1b;
// docs/design/density-d2-snapshots-survive-deploys-2026-10-26.md 3.2 and 4.4;
// docs/problems/guest-image-switch-window-kernel-params-and-bound-identity-
// 2026-10-26.md).
//
// Node B launches every guest from stable paths under VM_GUEST_RELEASE_LINK
// (/var/lib/go-choir/guest), a symlink the NixOS activation repoints at each
// switch before the deploy restarts vmctl. A guest launched before the switch
// has memory that describes the old store disk, but the snapshot's bound-file
// identities are sampled through the link at hibernate, so a hibernate after
// the switch records the NEW image's bytes and a later wake passes every
// check (H6). The launch record pins what the link named when the guest was
// launched; the snapshot copies it, and validation refuses a snapshot whose
// launch target is not the link's target now. Targets are compared, not file
// identities, so direct and jailed launches are both covered.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const launchRecordName = "guest-release.json"

// launchTargetUnreadable is recorded when the configured guest link could not
// be read at launch. It never equals a real target, so a snapshot of that
// guest is refused at wake (image_moved_since_launch).
const launchTargetUnreadable = "!unreadable"

// launchRecord is <state>/<vm>/guest-release.json. Release is empty until the
// release directories are in use (S3). ImageTarget is empty when no guest link
// is configured (the guard is off), launchTargetUnreadable when the configured
// link could not be read at launch (the snapshot refuses at wake).
type launchRecord struct {
	Release     string    `json:"release"`
	ImageTarget string    `json:"image_target"`
	Epoch       int64     `json:"epoch"`
	LaunchedAt  time.Time `json:"launched_at"`
}

// kernelParams returns the guest kernel parameters for a launch now: the
// contents of VM_KERNEL_PARAMS_FILE read at this moment (H1), else the value
// cached at vmctl start. Between a NixOS switch and the vmctl restart the
// cached value names the previous image's init= path, which is absent from
// the new store disk.
func (m *Manager) kernelParams() string {
	if file := strings.TrimSpace(m.cfg.KernelParamsFile); file != "" {
		if data, err := os.ReadFile(file); err == nil {
			if params := strings.TrimSpace(string(data)); params != "" {
				return params
			}
		}
	}
	return m.cfg.KernelParams
}

// guestImageTarget is the target of the guest link now. ok is false when no
// link is configured, it cannot be read, or it is not a symlink.
func (m *Manager) guestImageTarget() (string, bool) {
	link := strings.TrimSpace(m.cfg.GuestReleaseLink)
	if link == "" {
		return "", false
	}
	target, err := os.Readlink(link)
	if err != nil || target == "" {
		return "", false
	}
	return target, true
}

func (m *Manager) launchRecordPath(vmID string) string {
	return filepath.Join(m.vmStateDir(vmID), launchRecordName)
}

// writeLaunchRecord records, durably and before the spawn, what this launch
// is made of. The link is read once.
func (m *Manager) writeLaunchRecord(cfg VMConfig) error {
	target, ok := m.guestImageTarget()
	if cfg.BaseDiskPath != "" {
		// A layered launch names the layered link's target.
		if t, lok := m.layeredImageTarget(); lok {
			target, ok = t, true
		}
	}
	if !ok && strings.TrimSpace(m.cfg.GuestReleaseLink) != "" {
		// A configured link that cannot be read at launch: record that, so a
		// snapshot of this guest refuses at wake instead of turning the guard
		// off (review 318 DeepSeek 6).
		target = launchTargetUnreadable
	}
	return m.storeLaunchRecord(cfg.VMID, launchRecord{Release: cfg.GuestRelease, ImageTarget: target, Epoch: cfg.Epoch, LaunchedAt: time.Now().UTC()})
}

func (m *Manager) storeLaunchRecord(vmID string, rec launchRecord) error {
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	dir := m.vmStateDir(vmID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create state dir %s: %w", dir, err)
	}
	final := m.launchRecordPath(vmID)
	tmp := final + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("write launch record: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("write launch record: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("sync launch record: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close launch record: %w", err)
	}
	if err := os.Rename(tmp, final); err != nil {
		return fmt.Errorf("install launch record: %w", err)
	}
	return syncDirectory(dir)
}

// readLaunchRecord returns the record of the realization with this epoch. A
// missing, unreadable or other-epoch record is not trusted (ok false).
func (m *Manager) readLaunchRecord(vmID string, epoch int64) (launchRecord, bool) {
	data, err := os.ReadFile(m.launchRecordPath(vmID))
	if err != nil {
		return launchRecord{}, false
	}
	var rec launchRecord
	if err := json.Unmarshal(data, &rec); err != nil || rec.Epoch != epoch {
		return launchRecord{}, false
	}
	return rec, true
}

// imageMovedSinceLaunch reports whether a snapshot taken from a launch whose
// link target was recorded must be refused: the link names another image now,
// or cannot be read at all (review 313 Devin F4). A meta with no recorded
// target is not refused here (it validates as before this repair).
func (m *Manager) imageMovedSinceLaunch(meta *SnapshotMeta) bool {
	// A release-bound launch's paths are immutable release files, so the link
	// moving cannot change the bytes under it.
	if meta.LaunchImageTarget == "" || meta.GuestRelease != "" {
		return false
	}
	now, ok := m.guestImageTarget()
	return !ok || now != meta.LaunchImageTarget
}
