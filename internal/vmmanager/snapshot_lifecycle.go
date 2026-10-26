package vmmanager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// Hibernate and wake by Firecracker memory snapshot (S3). The pure parts
// (metadata, validation, budget, guest and API clients) are in snapshot.go
// and fcapi.go; this file sequences them on the VM lifecycle.

const (
	snapshotPingTimeout     = 2 * time.Second
	snapshotPrepareTimeout  = 90 * time.Second
	snapshotPauseTimeout    = 10 * time.Second
	snapshotCreateTimeout   = 15 * time.Minute
	snapshotAPIReadyTimeout = 5 * time.Second
	snapshotLoadTimeout     = 2 * time.Minute
	// snapshotResumedTimeout covers the guest's clock step plus, when a
	// credential is delivered, its exchange, consumption and key_revoked
	// append (three corpusd calls and one event append).
	snapshotResumedTimeout = 60 * time.Second
	// snapshotStateOverheadBytes covers the vmstate file beside the memory
	// file in the budget's worst-case reservation for a new snapshot.
	snapshotStateOverheadBytes = int64(64) << 20
)

// HibernateGuestBusyError is HibernateVM's answer when the guest's fence
// said it is working: the hibernate was abandoned and the VM keeps running.
// Callers count it as a skip. vmctl recognizes it by the HibernateGuestBusy
// method, so it need not import this package.
type HibernateGuestBusyError struct {
	VMID   string
	Detail error
}

func (e *HibernateGuestBusyError) Error() string {
	return fmt.Sprintf("hibernate of VM %s skipped: guest is busy: %v", e.VMID, e.Detail)
}

func (e *HibernateGuestBusyError) Unwrap() error { return e.Detail }

// HibernateGuestBusy marks the error for vmctl's skip handling.
func (e *HibernateGuestBusyError) HibernateGuestBusy() bool { return true }

// hibernateVMSnapshot fences the guest, captures a full snapshot and stops
// Firecracker. Any failure takes the kill path, so the VM always ends
// stopped, which is what vmctl records. The one exception is a guest that
// answers busy at the fence: it is left running and HibernateGuestBusyError
// is returned.
func (m *Manager) hibernateVMSnapshot(vmID string) error {
	unlock := m.lockVMOperation(vmID)
	defer unlock()

	m.mu.Lock()
	inst, ok := m.vms[vmID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("vm %s not found", vmID)
	}
	if inst.State != StateRunning {
		state := inst.State
		m.mu.Unlock()
		return fmt.Errorf("vm %s is not running (state=%s)", vmID, state)
	}
	cfg := inst.Config
	hostURL := inst.HostURL
	m.mu.Unlock()

	tl := newBootTimeline(vmID, "hibernate_snapshot")
	tl.Kind = "vm_hibernate_timeline"
	tl.ComputerID = cfg.ComputerID
	tl.Epoch = cfg.Epoch
	tl.MemSizeMiB = cfg.MachineMemSizeMib
	tl.VCPUCount = cfg.MachineCPUCount
	tl.HostURL = hostURL
	snap := &SnapshotTimeline{Mode: HibernateModeSnapshot}
	tl.Snapshot = snap
	tl.mark("hibernate_begin")

	meta, stopped, reason, err := m.captureSnapshot(vmID, cfg, hostURL, tl, snap)
	if reason == "" {
		snap.Outcome = "snapshot"
		snap.Vault = meta.Vault
		snap.Bytes = meta.Bytes
		snap.DiskBytes = meta.DiskBytes
		snap.DiskBytesMethod = meta.DiskBytesMethod
		snap.SnapshotType = meta.SnapshotType
		snap.CreatedAt = meta.CreatedAt.UTC().Format(time.RFC3339Nano)
		if !meta.ResumeDeadline.IsZero() {
			snap.ResumeDeadline = meta.ResumeDeadline.UTC().Format(time.RFC3339Nano)
		}
		tl.finish("hibernated", nil)
		m.persistHibernateTimeline(tl)
		log.Printf("vmmanager: hibernated VM %s by snapshot (fence=%dms pause=%dms snapshot=%dms bytes=%d disk_bytes=%d disk_bytes_method=%s resume_deadline=%s snapshot_type=%s diff_refused=%s)",
			vmID, snap.FenceMS, snap.PauseMS, snap.SnapshotMS, snap.Bytes, snap.DiskBytes, snap.DiskBytesMethod, snap.ResumeDeadline, snap.SnapshotType, snap.DiffRefused)
		return nil
	}

	// A guest that answered busy at the fence is working. It reopened
	// admission itself and nothing was paused or written, so the VM is
	// left running and the caller is told, never killed (density D1,
	// docs/problems/busy-guest-hibernate-takes-kill-path-2026-10-26.md).
	if reason == snapshotReasonGuestBusy {
		snap.Outcome = "skip"
		snap.Reason = reason
		log.Printf("vmmanager: snapshot hibernate of VM %s skipped (%s: %v); the guest is working and stays running", vmID, reason, err)
		tl.finish("skipped", err)
		m.persistHibernateTimeline(tl)
		return &HibernateGuestBusyError{VMID: vmID, Detail: err}
	}

	// Kill path: today's hibernate. The next wake cold boots, which is a crash
	// boot (no planned-restart marker is written by hibernate).
	m.discardSnapshotFiles(vmID)
	tl.BootKind = "hibernate_kill"
	snap.Outcome = "kill"
	snap.Reason = reason
	log.Printf("vmmanager: snapshot hibernate of VM %s not possible (%s: %v); using the kill path", vmID, reason, err)
	var killErr error
	if stopped {
		m.mu.Lock()
		if current := m.vms[vmID]; current != nil {
			current.State = StateHibernated
			current.Healthy = false
		}
		m.mu.Unlock()
	} else {
		killErr = m.hibernateVMKill(vmID)
	}
	tl.finish("killed", err)
	m.persistHibernateTimeline(tl)
	return killErr
}

// captureSnapshot runs the fence, pause, snapshot, stop and commit steps.
// reason is empty on success; otherwise it names why the kill path must be
// used, and stopped reports whether Firecracker was already stopped.
func (m *Manager) captureSnapshot(vmID string, cfg VMConfig, hostURL string, tl *BootTimeline, snap *SnapshotTimeline) (*SnapshotMeta, bool, string, error) {
	m.mu.Lock()
	var conf *ConfinementReceipt
	if inst := m.vms[vmID]; inst != nil {
		conf = inst.confinement.clone()
	}
	m.mu.Unlock()
	jailed := conf.jailed()
	// The snapshot vault (snapshot_vault.go): a computer the vault serves is
	// captured into the vault or not at all. These checks come before the
	// fence, so a refusal never pauses the guest. The vault directory is
	// opened and verified once and held until the commit is synced; every
	// path of the capture resolves through it (finding B1).
	if m.vaultPlacementUnknown(vmID, cfg.ComputerID) {
		return nil, false, snapshotReasonVaultUnknownComputer, fmt.Errorf("VM_SNAPSHOT_VAULT_ONLY is set and VM %s has no computer id; refusing rather than risk a plaintext capture", vmID)
	}
	if !m.vaultSelected(vmID, cfg.ComputerID) {
		return m.captureSnapshotInto(vmID, cfg, hostURL, tl, snap, conf, nil)
	}
	if jailed {
		return nil, false, snapshotReasonVaultJailed, fmt.Errorf("a jailed VMM captures inside its jail on the main file system; vault computers are not supported under the jailer")
	}
	vh, err := m.openVault(m.cfg.SnapshotVaultDir)
	if err != nil {
		return nil, false, snapshotReasonVaultUnavailable, err
	}
	defer vh.close()
	if err := m.recordVaultRoot(m.cfg.SnapshotVaultDir); err != nil {
		return nil, false, snapshotReasonVaultUnavailable, fmt.Errorf("record the vault root: %w", err)
	}
	meta, stopped, reason, err := m.captureSnapshotInto(vmID, cfg, hostURL, tl, snap, conf, vh)
	if reason != "" {
		// Whatever the capture wrote is removed where it was written: in the
		// held vault directory, even if the path now names another one.
		_ = os.RemoveAll(snapshotPartialDirIn(vh.self, vmID))
		_ = os.RemoveAll(snapshotDirIn(vh.self, vmID))
		_ = os.Remove(filepath.Join(vh.self, vmID))
	}
	return meta, stopped, reason, err
}

// captureSnapshotInto is captureSnapshot after the vault decision: vh is the
// held vault directory, or nil for the state directory.
func (m *Manager) captureSnapshotInto(vmID string, cfg VMConfig, hostURL string, tl *BootTimeline, snap *SnapshotTimeline, conf *ConfinementReceipt, vh *vaultHandle) (*SnapshotMeta, bool, string, error) {
	jailed := conf.jailed()
	root, vmmRoot := m.cfg.StateDir, m.cfg.StateDir
	if vh != nil {
		root, vmmRoot = vh.self, vh.vmm
	}
	sock := m.apiSocketPath(vmID)
	if jailed {
		sock = m.jailAPISocketHostPath(vmID)
	}
	if _, err := os.Stat(sock); err != nil {
		return nil, false, snapshotReasonNoAPISocket, err
	}
	api := newFCAPIClient(sock)
	pingCtx, cancel := context.WithTimeout(context.Background(), snapshotPingTimeout)
	err := api.Ping(pingCtx)
	cancel()
	if err != nil {
		return nil, false, snapshotReasonNoAPISocket, err
	}
	hostPort, ok := m.hostPortFromHostURL(hostURL)
	if !ok {
		return nil, false, snapshotReasonIdentity, fmt.Errorf("host URL %q names no host port", hostURL)
	}
	launchConfig, err := os.ReadFile(filepath.Join(m.vmStateDir(vmID), "fc-config.json"))
	if err != nil {
		return nil, false, snapshotReasonLaunchConfig, err
	}
	need := int64(cfg.MachineMemSizeMib)<<20 + snapshotStateOverheadBytes
	if !m.ensureSnapshotRoomFor(root, vh != nil, vmID, need) {
		return nil, false, snapshotReasonDiskBudget, fmt.Errorf("no room for %d bytes within budget %d", need, m.snapshotBudgetBytes())
	}
	tl.mark("budget_ok")

	// Fence: the guest closes run admission, confirms nothing is running and
	// syncs. A busy guest answers 409 and reopens admission itself.
	fenceStart := time.Now()
	prepareCtx, cancel := context.WithTimeout(context.Background(), snapshotPrepareTimeout)
	prepared, reason, err := guestHibernatePrepare(prepareCtx, hostURL)
	cancel()
	snap.FenceMS = time.Since(fenceStart).Milliseconds()
	if reason != "" {
		return nil, false, reason, err
	}
	tl.markDetail("guest_fenced", fmt.Sprintf("capability_expires_at=%s", prepared.CapabilityExpiresAt.UTC().Format(time.RFC3339)))

	// Shrink a hotplugged computer to its base before the snapshot, so it
	// does not carry a browser's pages (memory_hotplug.go). Bounded; the
	// snapshot proceeds with whatever the guest could not unplug.
	if launchConfigHasMemoryHotplug(launchConfig) {
		if detail := unplugBeforeSnapshot(api, vmID, cfg.MachineMemSizeMib, snapshotUnplugTimeout); detail != "" {
			tl.markDetail("hotplug_unplug", detail)
		}
	}

	// L3a (snapshot_diff.go): decide Full or Diff before the pause, so no
	// read or exec of the decision adds frozen time.
	m.mu.Lock()
	coldOrigin, pid := false, 0
	if inst := m.vms[vmID]; inst != nil {
		coldOrigin, pid = inst.coldOrigin, inst.PID
	}
	m.mu.Unlock()
	snap.ColdOrigin = coldOrigin
	snap.DiffRefused = m.diffSnapshotRefusal(cfg, coldOrigin, pid, launchConfigHasMemoryHotplug(launchConfig), func() string {
		return firecrackerBuildVersion(m.launchedVMM(vmID).Firecracker)
	})

	pauseStart := time.Now()
	pauseCtx, cancel := context.WithTimeout(context.Background(), snapshotPauseTimeout)
	err = api.Pause(pauseCtx)
	cancel()
	snap.PauseMS = time.Since(pauseStart).Milliseconds()
	if err != nil {
		return nil, false, snapshotReasonPauseFailed, err
	}
	tl.mark("vm_paused")

	partial := snapshotPartialDirIn(root, vmID)
	if err := os.RemoveAll(partial); err != nil {
		return nil, false, snapshotReasonCreateFailed, err
	}
	if err := os.MkdirAll(partial, 0o700); err != nil {
		return nil, false, snapshotReasonCreateFailed, err
	}
	if err := os.Chmod(partial, 0o700); err != nil {
		return nil, false, snapshotReasonCreateFailed, err
	}
	// The VMM resolves its paths itself; for a vault capture they go through
	// vmctl's held vault directory.
	statePath := filepath.Join(snapshotPartialDirIn(vmmRoot, vmID), snapshotStateName)
	memPath := filepath.Join(snapshotPartialDirIn(vmmRoot, vmID), snapshotMemName)
	// A jailed VMM writes inside its jail, to a capture dir its uid owns;
	// the files move to the host partial dir once it has stopped.
	jailPartial := ""
	if jailed {
		jailPartial = filepath.Join(m.jailRoot(vmID), snapshotPartialDirName)
		if err := os.RemoveAll(jailPartial); err != nil {
			return nil, false, snapshotReasonCreateFailed, err
		}
		if err := os.Mkdir(jailPartial, 0o700); err != nil {
			return nil, false, snapshotReasonCreateFailed, err
		}
		if err := m.chownFn(jailPartial, conf.UID, conf.GID); err != nil {
			return nil, false, snapshotReasonCreateFailed, err
		}
		statePath = "/" + snapshotPartialDirName + "/" + snapshotStateName
		memPath = "/" + snapshotPartialDirName + "/" + snapshotMemName
	}
	// A Diff when the decision allowed it; a Diff error or a memory file
	// of the wrong length falls back to Full inside this same pause.
	createStart := time.Now()
	createCtx, cancel := context.WithTimeout(context.Background(), snapshotCreateTimeout)
	snap.SnapshotType = snapshotTypeFull
	if snap.DiffRefused == "" {
		hostPartial := partial
		if jailed {
			hostPartial = jailPartial
		}
		err = api.CreateDiffSnapshot(createCtx, statePath, memPath)
		if err == nil {
			info, statErr := os.Stat(filepath.Join(hostPartial, snapshotMemName))
			if statErr == nil && info.Size() == diffMemoryLength(cfg) {
				snap.SnapshotType = snapshotTypeDiff
			} else {
				log.Printf("vmmanager: Diff snapshot of VM %s has the wrong memory length (%v, want %d); taking Full in the same pause", vmID, statErr, diffMemoryLength(cfg))
				snap.DiffRefused = diffRefusedLengthAtCreate
			}
		} else {
			log.Printf("vmmanager: Diff snapshot of VM %s failed (%v); taking Full in the same pause", vmID, err)
			snap.DiffRefused = diffRefusedCreateFailed
		}
		if snap.SnapshotType != snapshotTypeDiff {
			for _, name := range []string{snapshotStateName, snapshotMemName} {
				_ = os.Remove(filepath.Join(hostPartial, name))
			}
		}
	}
	if snap.SnapshotType == snapshotTypeFull {
		err = api.CreateFullSnapshot(createCtx, statePath, memPath)
	}
	cancel()
	snap.SnapshotMS = time.Since(createStart).Milliseconds()
	if err != nil {
		return nil, false, snapshotReasonCreateFailed, err
	}
	tl.mark("snapshot_created")

	// Stop Firecracker. Its memory now lives in the snapshot; the guest is
	// paused, so nothing runs between capture and stop.
	m.mu.Lock()
	if current := m.vms[vmID]; current != nil {
		m.killFirecrackerProcess(current, "hibernate-snapshot")
		current.State = StateHibernated
		current.Healthy = false
	}
	m.mu.Unlock()
	tl.mark("firecracker_stopped")

	if jailed {
		for _, name := range []string{snapshotStateName, snapshotMemName} {
			from, to := filepath.Join(jailPartial, name), filepath.Join(partial, name)
			if err := os.Rename(from, to); err != nil {
				return nil, true, snapshotReasonCommitFailed, fmt.Errorf("move jailed snapshot %s out of the jail: %w", name, err)
			}
			// Owned by the VMM uid only, as Firecracker created it.
			if err := m.chownFn(to, conf.UID, conf.GID); err != nil {
				return nil, true, snapshotReasonCommitFailed, err
			}
		}
		_ = os.Remove(jailPartial)
		tl.mark("snapshot_moved_from_jail")
	}

	meta, err := m.commitSnapshot(vmID, root, vh, cfg, hostURL, hostPort, launchConfig, prepared, snap, conf)
	if err != nil {
		return nil, true, snapshotReasonCommitFailed, err
	}
	tl.mark("snapshot_committed")
	return meta, true, "", nil
}

// commitSnapshot makes the captured files resumable: root-only modes, fsync,
// disk identities taken after Firecracker stopped, metadata, then one rename
// of snapshot.partial/ to snapshot/. An interrupted commit leaves only the
// partial dir, which is never loaded.
func (m *Manager) commitSnapshot(vmID, root string, vh *vaultHandle, cfg VMConfig, hostURL string, hostPort int, launchConfig []byte, prepared guestHibernatePrepareResponse, snap *SnapshotTimeline, conf *ConfinementReceipt) (*SnapshotMeta, error) {
	vault := vh != nil
	partial := snapshotPartialDirIn(root, vmID)
	if vault {
		// The vault must still be the encrypted mount the capture started on,
		// the held directory must still be the one at its path, and the
		// captured files must be on its device.
		if err := vh.verify(m); err != nil {
			return nil, fmt.Errorf("snapshot vault lost before the commit: %w", err)
		}
		if err := vh.onDevice(filepath.Join(partial, snapshotStateName), filepath.Join(partial, snapshotMemName)); err != nil {
			return nil, err
		}
	}
	for _, name := range []string{snapshotStateName, snapshotMemName} {
		path := filepath.Join(partial, name)
		if err := os.Chmod(path, 0o600); err != nil {
			return nil, err
		}
		if err := syncRegularFile(path); err != nil {
			return nil, err
		}
	}
	disks := make([]SnapshotDiskIdentity, 0, 6)
	for _, path := range m.snapshotBoundPaths(vmID, cfg) {
		id, err := diskIdentity(path)
		if err != nil {
			return nil, fmt.Errorf("snapshot-bound file %s: %w", path, err)
		}
		disks = append(disks, id)
	}
	// Measured after the fsyncs above, when btrfs has written the
	// compressed extents.
	diskBytes, diskMethod := dirDiskBytes(partial)
	now := time.Now().UTC()
	meta := &SnapshotMeta{
		Version:            snapshotMetaVersion,
		VMID:               vmID,
		ComputerID:         cfg.ComputerID,
		Epoch:              cfg.Epoch,
		HostPort:           hostPort,
		HostURL:            hostURL,
		FirecrackerVersion: firecrackerBuildVersion(m.launchedVMM(vmID).Firecracker),
		HostKernelRelease:  hostKernelRelease(),
		LaunchConfigSHA256: sha256Hex(launchConfig),
		Config:             snapshotStoredConfig(cfg),
		CredentialDiskPath: cfg.credentialDiskPath,
		Disks:              disks,
		MemSizeMiB:         cfg.MachineMemSizeMib,
		VCPUCount:          cfg.MachineCPUCount,
		GuestBuild:         prepared.GuestBuild,
		CreatedAt:          now,
		ResumeDeadline:     resumeDeadlineFrom(prepared.CapabilityExpiresAt),
		CredentialResume:   prepared.CredentialResume,
		Bytes:              dirLogicalBytes(partial),
		DiskBytes:          diskBytes,
		DiskBytesMethod:    diskMethod,
		FenceMS:            snap.FenceMS,
		PauseMS:            snap.PauseMS,
		SnapshotMS:         snap.SnapshotMS,
		Confinement:        ConfinementDirect,
		Vault:              vault,
		SnapshotType:       snap.SnapshotType,
		ColdOrigin:         snap.ColdOrigin,
	}
	if conf.jailed() {
		meta.Confinement = ConfinementJailer
		meta.VMMUID = conf.UID
	}
	meta.GuestRelease = cfg.GuestRelease
	// The CPU the memory image was captured on. An unreadable cpuinfo leaves
	// it empty: a release-bound snapshot then refuses at wake
	// (host_cpu_unknown); the hibernate still frees the memory.
	if fp, err := readHostCPU(); err == nil {
		meta.HostCPU = fp
	} else {
		log.Printf("vmmanager: warning: host CPU fingerprint for the snapshot of VM %s: %v", vmID, err)
	}
	// What the link named when this guest was launched (D2 S1b, H6).
	if rec, ok := m.readLaunchRecord(vmID, cfg.Epoch); ok {
		meta.LaunchImageTarget = rec.ImageTarget
	}
	if meta.FirecrackerVersion == "" || meta.HostKernelRelease == "" {
		return nil, fmt.Errorf("firecracker version or host kernel release unknown")
	}
	if err := writeSnapshotMeta(partial, meta); err != nil {
		return nil, err
	}
	final := snapshotDirIn(root, vmID)
	if err := os.RemoveAll(final); err != nil {
		return nil, err
	}
	if err := os.Rename(partial, final); err != nil {
		return nil, err
	}
	if err := syncDirectory(filepath.Join(root, vmID)); err != nil {
		return nil, err
	}
	m.reserveSnapshotPort(vmID, hostPort)
	return meta, nil
}

// ResumeVMFromSnapshot wakes a hibernated VM from its snapshot, at most once.
// expect carries what vmctl would cold boot now (machine shape, hold,
// computer id); any difference from the snapshot's launch is a refusal. On
// success the VM runs with the snapshot's epoch: no new realization, no
// restart pass in the guest. issue, when non-nil and the snapshot's guest
// advertised credential_resume, mints a fresh envelope for that same
// realization after validation; the guest installs it, so the frozen
// capability's renewal window no longer bounds the resume (max age still
// does). Every refusal or failure returns *SnapshotFallbackError and the
// caller cold boots, which is a crash boot under the restart rule.
func (m *Manager) ResumeVMFromSnapshot(vmID string, expect VMConfig, issue ResumeCredentialIssuer) (*VMInstance, error) {
	unlock := m.lockVMOperation(vmID)
	defer unlock()

	noSnapshot := &SnapshotFallbackError{VMID: vmID, Reason: snapshotReasonNoSnapshot}
	fallback := func(reason string, err error) (*VMInstance, error) {
		m.discardSnapshotFiles(vmID)
		m.setSnapshotFallback(vmID, reason)
		log.Printf("vmmanager: snapshot resume of VM %s not used (%s): %v", vmID, reason, err)
		return nil, &SnapshotFallbackError{VMID: vmID, Reason: reason, Err: err}
	}

	dir, vaultRoot := m.locateSnapshot(vmID)
	inVault := vaultRoot != ""
	meta, err := readSnapshotMeta(dir)
	if errors.Is(err, os.ErrNotExist) {
		// An interrupted capture is never resumable.
		for _, partial := range m.snapshotAnyDirs(vmID) {
			if filepath.Base(partial) == snapshotPartialDirName {
				_ = os.RemoveAll(partial)
			}
		}
		m.removeVaultVMDir(vmID)
		return nil, noSnapshot
	}
	if err != nil {
		return fallback(snapshotReasonUnreadable, err)
	}
	if m.hibernateMode() != HibernateModeSnapshot {
		// Rolled back to kill mode: never load.
		return fallback(snapshotReasonModeKill, nil)
	}
	// The snapshot vault: a snapshot in the vault loads only while the vault
	// still probes as an encrypted mount; a plaintext snapshot of a computer
	// the vault now serves is discarded, never loaded.
	computerID := expect.ComputerID
	if computerID == "" {
		computerID = meta.ComputerID
	}
	switch {
	case inVault:
		if err := m.probeVaultDir(vaultRoot); err != nil {
			return fallback(snapshotReasonVaultUnavailable, err)
		}
	case m.vaultSelected(vmID, computerID):
		return fallback(snapshotReasonNotInVault, nil)
	}

	m.mu.Lock()
	prev := m.vms[vmID]
	if prev != nil && (prev.State == StateRunning || prev.State == StatePending) {
		// A running computer's disks have moved past the snapshot.
		m.mu.Unlock()
		m.discardSnapshotFiles(vmID)
		return prev, nil
	}
	if prev == nil && m.liveFirecrackerForVMLocked(vmID) {
		m.mu.Unlock()
		return nil, fmt.Errorf("%w: vm %s", ErrLiveUnmanaged, vmID)
	}
	m.mu.Unlock()

	tl := newBootTimeline(vmID, "snapshot_resume")
	tl.mark("boot_begin")
	// The build this restore would launch; validation refuses a snapshot
	// taken under another (firecracker_version_mismatch).
	selectCfg, _ := m.expectedLaunchConfig(meta, expect)
	vmm := m.vmmFor(selectCfg)
	tl.FirecrackerBin = vmm.Firecracker
	tl.FirecrackerVer = firecrackerBuildVersion(vmm.Firecracker)
	tl.HostKernelRelease = hostKernelRelease()
	snap := &SnapshotTimeline{
		Mode:            HibernateModeSnapshot,
		FenceMS:         meta.FenceMS,
		PauseMS:         meta.PauseMS,
		SnapshotMS:      meta.SnapshotMS,
		Bytes:           meta.Bytes,
		DiskBytes:       meta.DiskBytes,
		DiskBytesMethod: meta.DiskBytesMethod,
		CreatedAt:       meta.CreatedAt.UTC().Format(time.RFC3339Nano),
		SnapshotType:    meta.SnapshotType,
		ColdOrigin:      meta.ColdOrigin,
	}
	if !meta.ResumeDeadline.IsZero() {
		snap.ResumeDeadline = meta.ResumeDeadline.UTC().Format(time.RFC3339Nano)
	}
	tl.Snapshot = snap

	delivery := issue != nil && meta.CredentialResume
	if reason := m.validateSnapshot(dir, meta, snapshotValidation{
		Now:                time.Now(),
		FirecrackerVersion: tl.FirecrackerVer,
		HostKernelRelease:  tl.HostKernelRelease,
		Expect:             expect,
		CredentialDelivery: delivery,
	}); reason != "" {
		return fallback(reason, nil)
	}
	tl.mark("snapshot_validated")
	// Re-credential: mint the envelope only now, for a snapshot that will be
	// loaded. It stays in this local variable: never in metadata, the
	// timeline, the instance config, or a log line.
	envelope := ""
	if delivery {
		issued, err := issue()
		switch {
		case err == nil && issued != "":
			envelope = issued
		case meta.resumeWindowPassed(time.Now()):
			if err == nil {
				err = fmt.Errorf("issuer returned no envelope")
			}
			return fallback(snapshotReasonCredentialIssue, err)
		default:
			log.Printf("vmmanager: credential issuance for snapshot resume of VM %s failed (%v); the guest renews its own capability inside its window", vmID, err)
		}
	}
	snap.Credential = snapshotCredentialRenewOnly
	if envelope != "" {
		snap.Credential = snapshotCredentialDelivered
		tl.mark("credential_issued")
	}
	cfg, _ := m.expectedLaunchConfig(meta, expect)
	cfg.Epoch = meta.Epoch
	cfg.BootKind = "snapshot_resume"
	tl.ComputerID = cfg.ComputerID
	tl.Epoch = cfg.Epoch
	tl.KernelImagePath = cfg.KernelImagePath
	tl.InitrdPath = cfg.InitrdPath
	tl.StoreDiskPath = cfg.StoreDiskPath
	tl.MemSizeMiB = cfg.MachineMemSizeMib
	tl.VCPUCount = cfg.MachineCPUCount
	tl.HostURL = meta.HostURL
	tl.GuestIP, _ = m.guestAndHostIP(meta.HostPort)

	m.mu.Lock()
	for id, other := range m.vms {
		if id != vmID && other.HostURL == meta.HostURL && (other.State == StateRunning || other.State == StatePending) {
			m.mu.Unlock()
			return fallback(snapshotReasonSubnetTaken, fmt.Errorf("VM %s routes %s", id, meta.HostURL))
		}
	}
	if !m.reclaimHostPortIfStale(vmID, meta.HostPort) {
		m.mu.Unlock()
		return fallback(snapshotReasonSubnetTaken, fmt.Errorf("a live interface holds the subnet of %s", meta.HostURL))
	}
	// The running guest keeps the launch it had: the record is rewritten from
	// the meta BEFORE the snapshot is consumed, so a write that fails falls
	// back to a cold boot instead of resuming a guest whose next hibernate
	// would carry no launch target (review 318 DeepSeek 4).
	if err := m.storeLaunchRecord(vmID, launchRecord{Release: meta.GuestRelease, ImageTarget: meta.LaunchImageTarget, Epoch: meta.Epoch, LaunchedAt: time.Now().UTC()}); err != nil {
		m.mu.Unlock()
		return fallback(snapshotReasonLaunchRecord, err)
	}
	// One-shot: durable before any load. From here a crash leads to a cold
	// boot, never a second load.
	if err := consumeSnapshot(dir, meta, time.Now()); err != nil {
		m.mu.Unlock()
		return fallback(snapshotReasonConsumeFailed, err)
	}
	tl.mark("snapshot_consumed")
	m.recordLaunchedVMM(vmID, vmm)
	m.reserveHostURLLocked(meta.HostURL)
	inst := &VMInstance{
		Config:       cfg,
		State:        StatePending,
		HostURL:      meta.HostURL,
		StartedAt:    time.Now(),
		done:         make(chan struct{}),
		BootTimeline: tl,
	}
	m.vms[vmID] = inst
	m.mu.Unlock()

	restoreFailed := func(reason string, err error) (*VMInstance, error) {
		m.mu.Lock()
		m.killFirecrackerProcess(inst, "snapshot-resume-failed")
		if prev != nil {
			m.vms[vmID] = prev
		} else {
			delete(m.vms, vmID)
		}
		m.mu.Unlock()
		snap.Outcome = "fallback_cold"
		snap.Reason = reason
		tl.finish("failed", err)
		if perr := tl.persist(m.cfg.StateDir); perr != nil {
			log.Printf("vmmanager: warning: could not persist snapshot resume timeline for VM %s: %v", vmID, perr)
		}
		return fallback(reason, err)
	}

	// Validation proved the snapshot's confinement is the one this restore
	// uses. A jailed restore rebuilds the jail with the launch's layout (the
	// vmstate names jail paths) and links the snapshot files into it.
	mode, modeReason := m.decideConfinement(cfg)
	receipt := &ConfinementReceipt{Mode: mode, Reason: modeReason}
	sock := ""
	statePath, memPath := filepath.Join(dir, snapshotStateName), filepath.Join(dir, snapshotMemName)
	if receipt.jailed() {
		receipt.UID, receipt.GID = meta.VMMUID, meta.VMMUID
		receipt.JailRoot = m.jailRoot(vmID)
		receipt.Cgroup = m.cgroupLeafRel(vmID)
		_, files, err := rewriteFirecrackerConfigForJail(m.buildFirecrackerConfig(cfg, meta.HostPort))
		if err != nil {
			return restoreFailed(snapshotReasonSpawnFailed, err)
		}
		notes, err := m.prepareJail(vmID, receipt.UID, files, nil)
		if err != nil {
			return restoreFailed(snapshotReasonSpawnFailed, err)
		}
		receipt.JailFiles = notes
		if err := m.linkSnapshotIntoJail(vmID, dir, receipt.UID); err != nil {
			return restoreFailed(snapshotReasonSpawnFailed, err)
		}
		sock = m.jailAPISocketHostPath(vmID)
		statePath = "/" + snapshotDirName + "/" + snapshotStateName
		memPath = "/" + snapshotDirName + "/" + snapshotMemName
		m.mu.Lock()
		m.recordConfinementLocked(vmID, receipt)
		if err = m.prepareVMNetwork(vmID, meta.HostPort, cfg.GuestPort, receipt.UID); err == nil {
			err = m.startJailedFirecracker(vmID, receipt, jailFirecrackerRestoreArgs())
		}
		m.mu.Unlock()
	} else {
		var ok bool
		sock, ok = m.prepareAPISocket(vmID)
		if !ok {
			return restoreFailed(snapshotReasonSpawnFailed, fmt.Errorf("API socket unavailable"))
		}
		m.mu.Lock()
		m.recordConfinementLocked(vmID, receipt)
		m.reclaimDirectOwnership(vmID)
		if err = m.prepareVMNetwork(vmID, meta.HostPort, cfg.GuestPort, -1); err == nil {
			err = m.startFirecrackerProcess(vmID, firecrackerRestoreArgs(vmID, sock))
		}
		m.mu.Unlock()
	}
	if err != nil {
		return restoreFailed(snapshotReasonSpawnFailed, err)
	}
	tl.mark("firecracker_spawned")

	api := newFCAPIClient(sock)
	readyCtx, cancel := context.WithTimeout(context.Background(), snapshotAPIReadyTimeout)
	err = api.WaitReady(readyCtx)
	cancel()
	if err != nil {
		return restoreFailed(snapshotReasonSpawnFailed, err)
	}
	loadStart := time.Now()
	loadCtx, cancel := context.WithTimeout(context.Background(), snapshotLoadTimeout)
	err = api.LoadSnapshot(loadCtx, statePath, memPath)
	cancel()
	snap.LoadMS = time.Since(loadStart).Milliseconds()
	if err != nil {
		return restoreFailed(snapshotReasonLoadFailed, err)
	}
	tl.mark("snapshot_loaded")

	// The guest steps its clock (stale since the snapshot), refreshes
	// process-local state, installs the delivered credential (or renews),
	// and reopens run admission.
	resumedCtx, cancel := context.WithTimeout(context.Background(), snapshotResumedTimeout)
	resumed, err := guestHibernateResumed(resumedCtx, meta.HostURL, time.Now(), envelope)
	cancel()
	if err != nil {
		return restoreFailed(snapshotReasonGuestResumed, err)
	}
	snap.ClockStepMS = resumed.ClockStepMS
	tl.markDetail("guest_resumed", fmt.Sprintf("clock_step_ms=%d", resumed.ClockStepMS))

	alive := func() bool {
		m.mu.Lock()
		pid := inst.PID
		m.mu.Unlock()
		if pid <= 0 {
			return true
		}
		return processExists(pid)
	}
	if err := m.waitForGuestReady(meta.HostURL, tl, alive); err != nil {
		return restoreFailed(snapshotReasonHealthFailed, err)
	}
	m.mu.Lock()
	inst.State = StateRunning
	inst.Healthy = true
	inst.LastHealthCheck = time.Now()
	inst.LastHealthyAt = inst.LastHealthCheck
	m.mu.Unlock()
	tl.firstHealthy()
	snap.Outcome = "snapshot"
	snap.WakeToHealthyMS = tl.FirstHealthyOffsetMS
	if meta.GuestRelease != "" {
		snap.GuestRelease = meta.GuestRelease
		snap.CurrentRelease = m.currentReleaseID()
		snap.Behind = snap.CurrentRelease != "" && snap.CurrentRelease != meta.GuestRelease
		m.mu.Lock()
		inst.Behind = snap.Behind
		m.mu.Unlock()
	}

	// The resumed VM maps the memory file MAP_PRIVATE for its whole life.
	// Removing the names is safe (the mapping holds the inode); the blocks are
	// freed when this Firecracker exits.
	m.discardSnapshotFiles(vmID)
	tl.finish("healthy", nil)
	if perr := tl.persist(m.cfg.StateDir); perr != nil {
		log.Printf("vmmanager: warning: could not persist snapshot resume timeline for VM %s: %v", vmID, perr)
	}
	log.Printf("vmmanager: resumed VM %s from snapshot (host=%s epoch=%d load=%dms wake_to_healthy=%dms clock_step=%dms credential=%s)",
		vmID, meta.HostURL, cfg.Epoch, snap.LoadMS, snap.WakeToHealthyMS, snap.ClockStepMS, snap.Credential)
	return inst, nil
}

// setSnapshotFallback remembers why a wake did not use a snapshot so the
// cold boot that follows records it.
func (m *Manager) setSnapshotFallback(vmID, reason string) {
	m.snapshotPortMu.Lock()
	defer m.snapshotPortMu.Unlock()
	if m.snapshotFallback == nil {
		m.snapshotFallback = map[string]string{}
	}
	m.snapshotFallback[vmID] = reason
}

func (m *Manager) takeSnapshotFallback(vmID string) string {
	m.snapshotPortMu.Lock()
	defer m.snapshotPortMu.Unlock()
	reason := m.snapshotFallback[vmID]
	delete(m.snapshotFallback, vmID)
	return reason
}

// recordSnapshotFallbackOnBoot discards any snapshot before a cold boot and
// marks the boot timeline with why the snapshot was not used.
func (m *Manager) recordSnapshotFallbackOnBoot(vmID string, tl *BootTimeline) {
	reason := m.takeSnapshotFallback(vmID)
	if m.discardSnapshotFiles(vmID) && reason == "" {
		reason = snapshotReasonColdBoot
	}
	if reason == "" {
		return
	}
	log.Printf("vmmanager: cold boot of VM %s after snapshot fallback (%s)", vmID, reason)
	if tl != nil {
		tl.markDetail("snapshot_fallback", "reason="+reason)
		tl.mu.Lock()
		tl.Snapshot = &SnapshotTimeline{Mode: m.hibernateMode(), Outcome: "fallback_cold", Reason: reason}
		tl.mu.Unlock()
	}
}

func (m *Manager) persistHibernateTimeline(tl *BootTimeline) {
	if err := tl.persistAs(m.cfg.StateDir, hibernateTimelineName); err != nil {
		log.Printf("vmmanager: warning: could not persist hibernate timeline for VM %s: %v", tl.VMID, err)
	}
}

// HibernateTimelineForVM returns the VM's most recent hibernate record.
func (m *Manager) HibernateTimelineForVM(vmID string) (*BootTimeline, error) {
	data, err := os.ReadFile(filepath.Join(m.cfg.StateDir, vmID, hibernateTimelineName))
	if err != nil {
		return nil, err
	}
	var t BootTimeline
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	return &t, nil
}
