package vmmanager

// S3 snapshot hibernate (docs/reports/s3-snapshot-hibernate-design-2026-10-14.md).
//
// In snapshot mode a hibernate asks the guest to fence (close run admission,
// confirm nothing is running, sync), pauses the VM, writes a full Firecracker
// snapshot under <state>/<vm>/snapshot/, and stops Firecracker. A wake first
// tries that snapshot: validate it against the current platform and the
// writable disks, mark it consumed durably, load it into a fresh Firecracker
// process, and tell the guest it resumed. Every refusal or failure falls back
// to today's cold boot.
//
// Restart rule (AGENTS.md "Restarts End Work (Crash) Or Resume It"): a
// snapshot resume is not a boot. The guest process never stopped, so no
// restart-closing pass runs, and the fence guaranteed nothing was in flight.
// A fallback cold boot is a crash boot: hibernate writes no planned-restart
// marker, so that boot closes open work exactly as a kill-hibernate does.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	// HibernateModeKill keeps today's hibernate: kill Firecracker, cold boot
	// on wake. It is the code default; launch is unchanged (--no-api).
	HibernateModeKill = "kill"
	// HibernateModeSnapshot launches with an API socket and hibernates by
	// memory snapshot, falling back to the kill path.
	HibernateModeSnapshot = "snapshot"

	snapshotDirName        = "snapshot"
	snapshotPartialDirName = "snapshot.partial"
	snapshotMetaName       = "meta.json"
	snapshotStateName      = "vmstate"
	snapshotMemName        = "mem"
	apiSockDirName         = "api"
	apiSockName            = "fc.sock"
	hibernateTimelineName  = "hibernate-timeline.json"

	snapshotMetaVersion = 1

	// DefaultSnapshotBudgetGiB bounds the logical bytes of all committed
	// snapshots on the host (VM_SNAPSHOT_BUDGET_GIB).
	DefaultSnapshotBudgetGiB = 48
	// DefaultSnapshotMaxAge bounds how long a snapshot stays resumable even
	// when the guest reported no credential deadline.
	DefaultSnapshotMaxAge = 12 * time.Hour
	// snapshotDiskFloorBytes is the free space the state filesystem must
	// still have after a snapshot is written. It also covers memory files a
	// resumed VM still maps after their names were deleted.
	snapshotDiskFloorBytes = int64(16) << 30
	// snapshotCredentialRenewalGrace mirrors the platform's
	// capabilityRenewalGrace (internal/platform/event_capability.go): an
	// expired event capability is still renewable for this long.
	snapshotCredentialRenewalGrace = 15 * time.Minute
	// snapshotCredentialMargin leaves the resumed guest time to renew.
	snapshotCredentialMargin = 3 * time.Minute
	// maxUnixSocketPathBytes is sun_path's usable length on Linux.
	maxUnixSocketPathBytes = 107

	guestHibernatePreparePath = "/internal/runtime/hibernate/prepare"
	guestHibernateResumedPath = "/internal/runtime/hibernate/resumed"
)

// Fallback reasons: the snapshot was not used and the caller cold boots, or
// the hibernate used the kill path. They are recorded on the timelines.
const (
	snapshotReasonModeKill         = "mode_kill"
	snapshotReasonNoAPISocket      = "no_api_socket"
	snapshotReasonDiskBudget       = "disk_budget"
	snapshotReasonGuestBusy        = "guest_busy"
	snapshotReasonGuestNoEndpoint  = "guest_prepare_unsupported"
	snapshotReasonGuestPrepare     = "guest_prepare_failed"
	snapshotReasonPauseFailed      = "pause_failed"
	snapshotReasonCreateFailed     = "snapshot_create_failed"
	snapshotReasonCommitFailed     = "snapshot_commit_failed"
	snapshotReasonNoSnapshot       = "no_snapshot"
	snapshotReasonUnreadable       = "metadata_unreadable"
	snapshotReasonConsumed         = "consumed"
	snapshotReasonVersion          = "metadata_version"
	snapshotReasonIdentity         = "vm_identity_mismatch"
	snapshotReasonFirecracker      = "firecracker_version_mismatch"
	snapshotReasonHostKernel       = "host_kernel_mismatch"
	snapshotReasonLaunchConfig     = "launch_config_mismatch"
	snapshotReasonDisk             = "disk_identity_mismatch"
	snapshotReasonFilesMissing     = "snapshot_files_missing"
	snapshotReasonCredentialWindow = "credential_window_passed"
	snapshotReasonMaxAge           = "max_age_passed"
	snapshotReasonSubnetTaken      = "subnet_taken"
	snapshotReasonConsumeFailed    = "consume_failed"
	snapshotReasonSpawnFailed      = "spawn_failed"
	snapshotReasonLoadFailed       = "load_failed"
	snapshotReasonGuestResumed     = "guest_resumed_failed"
	snapshotReasonHealthFailed     = "health_failed"
	snapshotReasonColdBoot         = "cold_boot_discarded"
)

// SnapshotFallbackError reports that a wake did not use a snapshot. The
// caller cold boots. Reason is one of the snapshotReason* values.
type SnapshotFallbackError struct {
	VMID   string
	Reason string
	Err    error
}

func (e *SnapshotFallbackError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("vm %s snapshot resume not used (%s): %v", e.VMID, e.Reason, e.Err)
	}
	return fmt.Sprintf("vm %s snapshot resume not used (%s)", e.VMID, e.Reason)
}

func (e *SnapshotFallbackError) Unwrap() error { return e.Err }

// SnapshotDiskIdentity pins one writable drive to the bytes the snapshot's
// memory was captured against. Taken after Firecracker stopped.
type SnapshotDiskIdentity struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	MtimeNS int64  `json:"mtime_ns"`
	Inode   uint64 `json:"inode"`
	Dev     uint64 `json:"dev"`
}

// SnapshotMeta is the durable record next to a snapshot. A snapshot is
// resumable only when this file exists in snapshot/ (not snapshot.partial/)
// and Consumed is false.
type SnapshotMeta struct {
	Version            int                    `json:"version"`
	VMID               string                 `json:"vm_id"`
	ComputerID         string                 `json:"computer_id,omitempty"`
	Epoch              int64                  `json:"epoch"`
	HostPort           int                    `json:"host_port"`
	HostURL            string                 `json:"host_url"`
	FirecrackerVersion string                 `json:"firecracker_version"`
	HostKernelRelease  string                 `json:"host_kernel_release"`
	LaunchConfigSHA256 string                 `json:"launch_config_sha256"`
	Config             VMConfig               `json:"config"`
	CredentialDiskPath string                 `json:"credential_disk_path,omitempty"`
	Disks              []SnapshotDiskIdentity `json:"disks"`
	MemSizeMiB         int                    `json:"mem_size_mib"`
	VCPUCount          int                    `json:"vcpu_count"`
	GuestBuild         string                 `json:"guest_build,omitempty"`
	CreatedAt          time.Time              `json:"created_at"`
	ResumeDeadline     time.Time              `json:"resume_deadline,omitempty"`
	Bytes              int64                  `json:"bytes"`
	FenceMS            int64                  `json:"fence_ms"`
	PauseMS            int64                  `json:"pause_ms"`
	SnapshotMS         int64                  `json:"snapshot_ms"`
	Consumed           bool                   `json:"consumed"`
	ConsumedAt         time.Time              `json:"consumed_at,omitempty"`
}

// SnapshotTimeline is the S3 section of a hibernate or wake timeline.
type SnapshotTimeline struct {
	Mode            string `json:"mode,omitempty"`
	Outcome         string `json:"outcome,omitempty"` // snapshot | kill | fallback_cold
	Reason          string `json:"reason,omitempty"`
	FenceMS         int64  `json:"fence_ms,omitempty"`
	PauseMS         int64  `json:"pause_ms,omitempty"`
	SnapshotMS      int64  `json:"snapshot_ms,omitempty"`
	Bytes           int64  `json:"bytes,omitempty"`
	CreatedAt       string `json:"created_at,omitempty"`
	ResumeDeadline  string `json:"resume_deadline,omitempty"`
	LoadMS          int64  `json:"load_ms,omitempty"`
	WakeToHealthyMS int64  `json:"wake_to_healthy_ms,omitempty"`
	ClockStepMS     int64  `json:"clock_step_ms,omitempty"`
}

// --- paths ---------------------------------------------------------------

func (m *Manager) vmStateDir(vmID string) string {
	return filepath.Join(m.cfg.StateDir, vmID)
}

func (m *Manager) snapshotDir(vmID string) string {
	return filepath.Join(m.vmStateDir(vmID), snapshotDirName)
}

func (m *Manager) snapshotPartialDir(vmID string) string {
	return filepath.Join(m.vmStateDir(vmID), snapshotPartialDirName)
}

func (m *Manager) apiSocketPath(vmID string) string {
	return filepath.Join(m.vmStateDir(vmID), apiSockDirName, apiSockName)
}

// hibernateMode returns the configured mode; anything but "snapshot" is kill.
func (m *Manager) hibernateMode() string {
	if strings.EqualFold(strings.TrimSpace(m.cfg.HibernateMode), HibernateModeSnapshot) {
		return HibernateModeSnapshot
	}
	return HibernateModeKill
}

func (m *Manager) snapshotBudgetBytes() int64 {
	gib := m.cfg.SnapshotBudgetGiB
	if gib <= 0 {
		gib = DefaultSnapshotBudgetGiB
	}
	return int64(gib) << 30
}

func (m *Manager) snapshotMaxAge() time.Duration {
	if m.cfg.SnapshotMaxAge > 0 {
		return m.cfg.SnapshotMaxAge
	}
	return DefaultSnapshotMaxAge
}

// prepareAPISocket readies <state>/<vm>/api/fc.sock for a snapshot-mode
// launch: a root-only directory and no stale socket file (Firecracker
// refuses to bind over one). ok=false keeps the VM on --no-api.
func (m *Manager) prepareAPISocket(vmID string) (string, bool) {
	sock := m.apiSocketPath(vmID)
	if len(sock) > maxUnixSocketPathBytes {
		log.Printf("vmmanager: API socket path for VM %s is %d bytes (max %d); launching without API", vmID, len(sock), maxUnixSocketPathBytes)
		return "", false
	}
	dir := filepath.Dir(sock)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		log.Printf("vmmanager: create API socket dir for VM %s: %v; launching without API", vmID, err)
		return "", false
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		log.Printf("vmmanager: chmod API socket dir for VM %s: %v; launching without API", vmID, err)
		return "", false
	}
	if err := os.Remove(sock); err != nil && !os.IsNotExist(err) {
		log.Printf("vmmanager: remove stale API socket for VM %s: %v; launching without API", vmID, err)
		return "", false
	}
	return sock, true
}

// firecrackerLaunchArgs is the boot command line. Kill mode keeps today's
// --no-api launch exactly; snapshot mode serves the API on the VM's socket
// beside the same --config-file (Firecracker boots from the file and keeps
// serving the API).
func (m *Manager) firecrackerLaunchArgs(vmID, configPath string) []string {
	if m.hibernateMode() == HibernateModeSnapshot {
		if sock, ok := m.prepareAPISocket(vmID); ok {
			return []string{"--api-sock", sock, "--enable-pci", "--id", vmID, "--config-file", configPath}
		}
	}
	return []string{"--no-api", "--enable-pci", "--id", vmID, "--config-file", configPath}
}

// firecrackerRestoreArgs is the snapshot-load command line: the API socket
// only. --enable-pci matches the launch; restore takes the PCI transport from
// the saved device state (pci_mngr.rs restore), so it is harmless.
func firecrackerRestoreArgs(vmID, sock string) []string {
	return []string{"--api-sock", sock, "--enable-pci", "--id", vmID}
}

// --- metadata ------------------------------------------------------------

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// launchConfigHash is the sha256 of the Firecracker JSON config as written to
// fc-config.json by launchFirecracker.
func launchConfigHash(fcConfig map[string]interface{}) (string, error) {
	data, err := json.Marshal(fcConfig)
	if err != nil {
		return "", err
	}
	return sha256Hex(data), nil
}

// snapshotStoredConfig strips bootstrap secrets from a launch config before
// it is written to metadata. The gateway token and envelope never leave the
// credential disk and persist dir.
func snapshotStoredConfig(cfg VMConfig) VMConfig {
	cfg.GatewayToken = ""
	cfg.ComputerCredentialEnvelope = ""
	cfg.credentialDiskPath = ""
	cfg.BootKind = ""
	return cfg
}

func readSnapshotMeta(dir string) (*SnapshotMeta, error) {
	data, err := os.ReadFile(filepath.Join(dir, snapshotMetaName))
	if err != nil {
		return nil, err
	}
	var meta SnapshotMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// writeSnapshotMeta writes meta.json atomically and durably: temp file,
// fsync, rename, fsync the directory. Consumption relies on this ordering.
func writeSnapshotMeta(dir string, meta *SnapshotMeta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	final := filepath.Join(dir, snapshotMetaName)
	tmp := final + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, final); err != nil {
		return err
	}
	return syncDirectory(dir)
}

// consumeSnapshot marks the snapshot used and makes that durable BEFORE any
// load. A crash after this point leads to a cold boot, never a second load.
func consumeSnapshot(dir string, meta *SnapshotMeta, now time.Time) error {
	meta.Consumed = true
	meta.ConsumedAt = now.UTC()
	return writeSnapshotMeta(dir, meta)
}

func diskIdentity(path string) (SnapshotDiskIdentity, error) {
	info, err := os.Stat(path)
	if err != nil {
		return SnapshotDiskIdentity{}, err
	}
	id := SnapshotDiskIdentity{Path: path, Size: info.Size(), MtimeNS: info.ModTime().UnixNano()}
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		id.Inode = uint64(st.Ino)
		id.Dev = uint64(st.Dev)
	}
	return id, nil
}

// snapshotBoundPaths lists the files whose bytes must still be the ones the
// memory snapshot was taken against: the drives the guest can write, and the
// boot artifacts and read-only drives. Node B deploys guest artifacts to
// stable paths (/var/lib/go-choir/guest/...), so an unchanged launch config
// hash does not prove unchanged software; the resumed guest's page cache and
// filesystem state describe the old read-only store image, so a store image
// replaced under the same path must refuse the resume.
func (m *Manager) snapshotBoundPaths(vmID string, cfg VMConfig) []string {
	paths := []string{filepath.Join(m.vmStateDir(vmID), "data.img")}
	seen := map[string]bool{paths[0]: true}
	for _, path := range []string{cfg.credentialDiskPath, cfg.RootfsPath, cfg.StoreDiskPath, cfg.KernelImagePath, cfg.InitrdPath} {
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		paths = append(paths, path)
	}
	return paths
}

// expectedLaunchConfig rebuilds the Firecracker config a cold boot of the
// snapshot's realization would use under the current deploy: stored identity
// (epoch, ids, port, credential disk) with today's platform artifacts and
// vmctl's current shape and hold. Its hash equals the launch hash only when
// nothing the snapshot depends on moved.
func (m *Manager) expectedLaunchConfig(meta *SnapshotMeta, expect VMConfig) (VMConfig, map[string]interface{}) {
	cfg := meta.Config
	cfg.VMID = meta.VMID
	cfg.credentialDiskPath = meta.CredentialDiskPath
	cfg = refreshConfigForCurrentDeploy(cfg, m.cfg)
	if expect.MachineCPUCount != 0 {
		cfg.MachineCPUCount = expect.MachineCPUCount
	}
	if expect.MachineMemSizeMib != 0 {
		cfg.MachineMemSizeMib = expect.MachineMemSizeMib
	}
	cfg.MaintenanceHold = expect.MaintenanceHold
	cfg.RecoveryReplayOnly = false
	m.applyLaunchDefaults(&cfg)
	cfg.GatewayToken = m.resolveGatewayToken(cfg)
	return cfg, m.buildFirecrackerConfig(cfg, meta.HostPort)
}

// snapshotValidation carries the facts a validation compares against.
type snapshotValidation struct {
	Now                time.Time
	FirecrackerVersion string
	HostKernelRelease  string
	Expect             VMConfig
}

// validateSnapshot returns "" when the snapshot may be loaded, else the
// fallback reason. It reads files but never changes them.
func (m *Manager) validateSnapshot(dir string, meta *SnapshotMeta, v snapshotValidation) string {
	if meta == nil {
		return snapshotReasonUnreadable
	}
	if meta.Consumed {
		return snapshotReasonConsumed
	}
	if meta.Version != snapshotMetaVersion {
		return snapshotReasonVersion
	}
	if meta.VMID == "" || meta.VMID != meta.Config.VMID || filepath.Base(filepath.Dir(dir)) != meta.VMID {
		return snapshotReasonIdentity
	}
	if v.Expect.ComputerID != "" && meta.ComputerID != "" && v.Expect.ComputerID != meta.ComputerID {
		return snapshotReasonIdentity
	}
	// The snapshot belongs to one realization: a caller expecting another
	// epoch has moved past it.
	if v.Expect.Epoch > 0 && meta.Epoch != v.Expect.Epoch {
		return snapshotReasonIdentity
	}
	if !v.Now.Before(meta.CreatedAt.Add(m.snapshotMaxAge())) {
		return snapshotReasonMaxAge
	}
	if !meta.ResumeDeadline.IsZero() && !v.Now.Before(meta.ResumeDeadline) {
		return snapshotReasonCredentialWindow
	}
	if meta.FirecrackerVersion == "" || meta.FirecrackerVersion != v.FirecrackerVersion {
		return snapshotReasonFirecracker
	}
	if meta.HostKernelRelease == "" || meta.HostKernelRelease != v.HostKernelRelease {
		return snapshotReasonHostKernel
	}
	_, fcConfig := m.expectedLaunchConfig(meta, v.Expect)
	hash, err := launchConfigHash(fcConfig)
	if err != nil || hash != meta.LaunchConfigSHA256 {
		return snapshotReasonLaunchConfig
	}
	for _, name := range []string{snapshotStateName, snapshotMemName} {
		if info, err := os.Stat(filepath.Join(dir, name)); err != nil || !info.Mode().IsRegular() {
			return snapshotReasonFilesMissing
		}
	}
	if len(meta.Disks) == 0 {
		return snapshotReasonDisk
	}
	for _, want := range meta.Disks {
		got, err := diskIdentity(want.Path)
		if err != nil || got != want {
			return snapshotReasonDisk
		}
	}
	return ""
}

// resumeDeadlineFrom turns the guest's reported event-capability expiry into
// the last moment a resumed guest can still renew it. Zero expiry means the
// guest holds no such capability.
func resumeDeadlineFrom(capabilityExpiresAt time.Time) time.Time {
	if capabilityExpiresAt.IsZero() {
		return time.Time{}
	}
	return capabilityExpiresAt.Add(snapshotCredentialRenewalGrace - snapshotCredentialMargin)
}

// discardSnapshotFiles removes a VM's snapshot (committed and partial) and
// releases its host-port reservation. Callers hold the VM's operation lock.
func (m *Manager) discardSnapshotFiles(vmID string) bool {
	removed := false
	for _, dir := range []string{m.snapshotDir(vmID), m.snapshotPartialDir(vmID)} {
		if _, err := os.Stat(dir); err == nil {
			if err := os.RemoveAll(dir); err != nil {
				log.Printf("vmmanager: remove snapshot dir %s: %v", dir, err)
				continue
			}
			removed = true
		}
	}
	m.releaseSnapshotPort(vmID)
	return removed
}

// --- host port reservations ---------------------------------------------

// reserveSnapshotPort keeps a hibernated VM's subnet from being handed to
// another boot: the snapshot's guest memory holds that IP and MAC.
func (m *Manager) reserveSnapshotPort(vmID string, hostPort int) {
	m.snapshotPortMu.Lock()
	defer m.snapshotPortMu.Unlock()
	if m.snapshotPorts == nil {
		m.snapshotPorts = map[int]string{}
	}
	for port, owner := range m.snapshotPorts {
		if owner == vmID {
			delete(m.snapshotPorts, port)
		}
	}
	m.snapshotPorts[hostPort] = vmID
}

func (m *Manager) releaseSnapshotPort(vmID string) {
	m.snapshotPortMu.Lock()
	defer m.snapshotPortMu.Unlock()
	for port, owner := range m.snapshotPorts {
		if owner == vmID {
			delete(m.snapshotPorts, port)
		}
	}
}

// snapshotPortHeldByOther reports whether hostPort is reserved for another
// VM's snapshot.
func (m *Manager) snapshotPortHeldByOther(vmID string, hostPort int) bool {
	m.snapshotPortMu.Lock()
	defer m.snapshotPortMu.Unlock()
	owner, ok := m.snapshotPorts[hostPort]
	return ok && owner != vmID
}

// loadSnapshotReservations re-reserves the ports of committed, unconsumed
// snapshots after a vmctl restart.
func (m *Manager) loadSnapshotReservations() {
	for _, entry := range m.listSnapshots() {
		if !entry.meta.Consumed && entry.meta.HostPort > 0 {
			m.reserveSnapshotPort(entry.vmID, entry.meta.HostPort)
		}
	}
}

// --- budget --------------------------------------------------------------

type snapshotEntry struct {
	vmID  string
	dir   string
	meta  *SnapshotMeta
	bytes int64
}

func dirLogicalBytes(dir string) int64 {
	var total int64
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	for _, entry := range entries {
		if info, err := entry.Info(); err == nil && info.Mode().IsRegular() {
			total += info.Size()
		}
	}
	return total
}

// listSnapshots scans <state>/*/snapshot/meta.json.
func (m *Manager) listSnapshots() []snapshotEntry {
	root := m.cfg.StateDir
	if root == "" {
		return nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	out := make([]snapshotEntry, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name(), snapshotDirName)
		meta, err := readSnapshotMeta(dir)
		if err != nil {
			continue
		}
		out = append(out, snapshotEntry{vmID: entry.Name(), dir: dir, meta: meta, bytes: dirLogicalBytes(dir)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].meta.CreatedAt.Before(out[j].meta.CreatedAt) })
	return out
}

// statfsAvailable reports free bytes on the filesystem holding path.
var statfsAvailable = func(path string) (int64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	return int64(st.Bavail) * int64(st.Bsize), nil
}

// ensureSnapshotRoom makes room for a snapshot of need bytes for vmID within
// the budget, discarding the least recently created snapshots of other VMs
// (their computers cold-wake later). It returns false when there is still no
// room; the caller then uses the kill path.
func (m *Manager) ensureSnapshotRoom(vmID string, need int64) bool {
	budget := m.snapshotBudgetBytes()
	if need > budget {
		return false
	}
	entries := m.listSnapshots()
	var used int64
	for _, entry := range entries {
		if entry.vmID == vmID {
			continue // replaced by this capture
		}
		used += entry.bytes
	}
	for _, entry := range entries {
		if used+need <= budget {
			break
		}
		if entry.vmID == vmID {
			continue
		}
		if !m.tryDiscardOtherSnapshot(entry.vmID, "budget_evicted") {
			continue
		}
		used -= entry.bytes
	}
	if used+need > budget {
		return false
	}
	avail, err := statfsAvailable(m.cfg.StateDir)
	if err != nil {
		log.Printf("vmmanager: statfs %s: %v; refusing snapshot", m.cfg.StateDir, err)
		return false
	}
	return avail-need >= snapshotDiskFloorBytes
}

// tryDiscardOtherSnapshot discards another VM's snapshot only if that VM's
// operation lock is free, so an eviction never races its resume.
func (m *Manager) tryDiscardOtherSnapshot(vmID, reason string) bool {
	m.opMu.Lock()
	lock := m.vmOps[vmID]
	if lock == nil {
		lock = &sync.Mutex{}
		m.vmOps[vmID] = lock
	}
	m.opMu.Unlock()
	if !lock.TryLock() {
		return false
	}
	defer lock.Unlock()
	if m.discardSnapshotFiles(vmID) {
		log.Printf("vmmanager: discarded snapshot of VM %s (%s)", vmID, reason)
	}
	return true
}

// --- guest fence client --------------------------------------------------

// guestHibernatePrepareResponse is the guest's answer to prepare.
type guestHibernatePrepareResponse struct {
	Status              string    `json:"status"`
	Outcome             string    `json:"outcome,omitempty"`
	Error               string    `json:"error,omitempty"`
	CapabilityExpiresAt time.Time `json:"capability_expires_at,omitempty"`
	GuestBuild          string    `json:"guest_build,omitempty"`
	RunningRuns         int       `json:"running_runs,omitempty"`
	SelfdevActiveOps    int       `json:"selfdev_active_operations,omitempty"`
}

// guestHibernateResumedResponse is the guest's answer to resumed.
type guestHibernateResumedResponse struct {
	Status      string `json:"status"`
	ClockStepMS int64  `json:"clock_step_ms"`
	Error       string `json:"error,omitempty"`
}

var guestHibernateClient = &http.Client{Timeout: 60 * time.Second}

func postGuestJSON(ctx context.Context, url string, body any, out any) (int, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")
	resp, err := guestHibernateClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if out != nil && len(data) > 0 {
		_ = json.Unmarshal(data, out)
	}
	return resp.StatusCode, nil
}

// guestHibernatePrepare asks the guest to fence. It returns the response on
// 200, or the kill-path reason.
func guestHibernatePrepare(ctx context.Context, hostURL string) (guestHibernatePrepareResponse, string, error) {
	var resp guestHibernatePrepareResponse
	status, err := postGuestJSON(ctx, hostURL+guestHibernatePreparePath, map[string]any{}, &resp)
	switch {
	case err != nil:
		return resp, snapshotReasonGuestPrepare, err
	case status == http.StatusOK && resp.Status == "prepared":
		return resp, "", nil
	case status == http.StatusConflict:
		return resp, snapshotReasonGuestBusy, fmt.Errorf("guest busy: running_runs=%d selfdev_active=%d", resp.RunningRuns, resp.SelfdevActiveOps)
	case status == http.StatusNotFound || status == http.StatusMethodNotAllowed:
		return resp, snapshotReasonGuestNoEndpoint, fmt.Errorf("guest answered %d", status)
	default:
		return resp, snapshotReasonGuestPrepare, fmt.Errorf("guest answered %d: %s", status, resp.Error)
	}
}

// guestHibernateResumed tells a resumed guest the host wall time; it steps
// its clock, refreshes process-local state and reopens admission.
func guestHibernateResumed(ctx context.Context, hostURL string, now time.Time) (guestHibernateResumedResponse, error) {
	var resp guestHibernateResumedResponse
	status, err := postGuestJSON(ctx, hostURL+guestHibernateResumedPath, map[string]any{
		"host_unix_nano": now.UnixNano(),
	}, &resp)
	if err != nil {
		return resp, err
	}
	if status != http.StatusOK || resp.Status != "resumed" {
		return resp, fmt.Errorf("guest answered %d: %s", status, resp.Error)
	}
	return resp, nil
}
