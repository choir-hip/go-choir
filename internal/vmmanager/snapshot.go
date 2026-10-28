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

	"github.com/yusefmosiah/go-choir/internal/hostauth"
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

	// DefaultSnapshotBudgetGiB bounds the on-disk bytes of all committed
	// snapshots on the host (VM_SNAPSHOT_BUDGET_GIB). btrfs compresses guest
	// memory (about 6% on Node B), so committed snapshots count at their
	// measured on-disk bytes and a new one at its logical worst case
	// (snapshot_disk.go; design note items 39-43).
	DefaultSnapshotBudgetGiB = 48
	// DefaultSnapshotMaxAge bounds how long a snapshot stays resumable. Once
	// fresh credentials are delivered on resume it is the only age bound.
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
	snapshotReasonModeKill        = "mode_kill"
	snapshotReasonNoAPISocket     = "no_api_socket"
	snapshotReasonDiskBudget      = "disk_budget"
	snapshotReasonGuestBusy       = "guest_busy"
	snapshotReasonGuestNoEndpoint = "guest_prepare_unsupported"
	snapshotReasonGuestPrepare    = "guest_prepare_failed"
	snapshotReasonPauseFailed     = "pause_failed"
	snapshotReasonCreateFailed    = "snapshot_create_failed"
	snapshotReasonCommitFailed    = "snapshot_commit_failed"
	snapshotReasonNoSnapshot      = "no_snapshot"
	snapshotReasonUnreadable      = "metadata_unreadable"
	snapshotReasonConsumed        = "consumed"
	snapshotReasonVersion         = "metadata_version"
	snapshotReasonIdentity        = "vm_identity_mismatch"
	snapshotReasonFirecracker     = "firecracker_version_mismatch"
	snapshotReasonHostKernel      = "host_kernel_mismatch"
	snapshotReasonLaunchConfig    = "launch_config_mismatch"
	// snapshotReasonImageMoved: the guest link named another image when the
	// snapshot's guest was launched than it does now, or cannot be read (D2
	// S1b, H6). The guest's memory describes the launch-time store disk.
	snapshotReasonImageMoved = "image_moved_since_launch"
	// snapshotReasonLaunchRecord: the launch record could not be rewritten at
	// resume, so the guest would lose its H6 guard.
	snapshotReasonLaunchRecord     = "launch_record_failed"
	snapshotReasonDisk             = "disk_identity_mismatch"
	snapshotReasonFilesMissing     = "snapshot_files_missing"
	snapshotReasonCredentialWindow = "credential_window_passed"
	snapshotReasonCredentialIssue  = "credential_issue_failed"
	snapshotReasonMaxAge           = "max_age_passed"
	snapshotReasonSubnetTaken      = "subnet_taken"
	snapshotReasonConsumeFailed    = "consume_failed"
	snapshotReasonSpawnFailed      = "spawn_failed"
	snapshotReasonLoadFailed       = "load_failed"
	snapshotReasonGuestResumed     = "guest_resumed_failed"
	snapshotReasonHealthFailed     = "health_failed"
	snapshotReasonColdBoot         = "cold_boot_discarded"
	// snapshotReasonConfinement refuses a snapshot taken under another
	// confinement (jailer vs direct) or another VMM uid: no mixed-mode
	// resume (VMM jailer slice 1, failure item 15).
	snapshotReasonConfinement = "confinement_mismatch"
	// Snapshot vault (SDC NET-06): the target did not probe as an encrypted
	// mount; a jailed VMM would capture on the main file system first; a
	// plaintext snapshot was found for a computer that now uses the vault.
	snapshotReasonVaultUnavailable = "snapshot_vault_unavailable"
	snapshotReasonVaultJailed      = "snapshot_vault_jailed_unsupported"
	// snapshotReasonVaultUnknownComputer: the vault list is set and the
	// computer has no id, so its placement cannot be told (kill path).
	snapshotReasonVaultUnknownComputer = "snapshot_vault_unknown_computer"
	snapshotReasonNotInVault           = "snapshot_not_in_vault"
)

// Credential paths of a snapshot resume, recorded on the wake timeline.
const (
	// snapshotCredentialDelivered: the host minted a fresh envelope for the
	// snapshot's realization and the guest installed it.
	snapshotCredentialDelivered = "delivered"
	// snapshotCredentialRenewOnly: no envelope; the guest renewed its frozen
	// capability inside the renewal window.
	snapshotCredentialRenewOnly = "renew_only"
)

// ResumeCredentialIssuer mints a fresh bootstrap envelope (base64url
// canonical JSON) for the realization being resumed. vmctl supplies it; the
// manager calls it only for a snapshot that validated, and the envelope goes
// to the guest in the host-only resumed call, never to disk or logs.
type ResumeCredentialIssuer func() (string, error)

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
	// ResumeDeadline is the last moment the guest could still renew its
	// frozen capability. It bounds a resume only when no fresh credential
	// can be delivered (CredentialResume false, or no issuer).
	ResumeDeadline time.Time `json:"resume_deadline,omitempty"`
	// CredentialResume records that the guest advertised it installs a
	// fresh envelope delivered with resumed.
	CredentialResume bool `json:"credential_resume,omitempty"`
	// Bytes is the logical size of the snapshot files; DiskBytes what they
	// occupy on disk (compressed on btrfs), measured by DiskBytesMethod
	// after fsync. The budget counts DiskBytes.
	Bytes           int64     `json:"bytes"`
	DiskBytes       int64     `json:"disk_bytes,omitempty"`
	DiskBytesMethod string    `json:"disk_bytes_method,omitempty"`
	FenceMS         int64     `json:"fence_ms"`
	PauseMS         int64     `json:"pause_ms"`
	SnapshotMS      int64     `json:"snapshot_ms"`
	Consumed        bool      `json:"consumed"`
	ConsumedAt      time.Time `json:"consumed_at,omitempty"`
	// Confinement is the capturing VMM's confinement (direct or jailer);
	// empty in snapshots from before the jailer, which were direct. VMMUID is
	// the jailed VMM's uid, which owns the snapshot files.
	Confinement string `json:"confinement,omitempty"`
	VMMUID      int    `json:"vmm_uid,omitempty"`
	// Vault records that the snapshot was written into the snapshot vault.
	// Observability only: where a snapshot is found decides where it is read.
	Vault bool `json:"vault,omitempty"`
	// SnapshotType is Full or Diff (L3a, snapshot_diff.go); empty in metas
	// from before L3a, which are Full. ColdOrigin records that the captured
	// process was cold booted; a Diff loads only with it. Additive fields,
	// no version bump.
	SnapshotType string `json:"snapshot_type,omitempty"`
	ColdOrigin   bool   `json:"cold_origin,omitempty"`
	// LaunchImageTarget is the target of the guest link when the captured
	// guest was launched (the launch record, D2 S1b); empty for a guest
	// launched before the record existed, which validates as before.
	// Additive, no version bump.
	LaunchImageTarget string `json:"launch_image_target,omitempty"`
	// GuestRelease is the guest release the captured guest was launched from
	// (D2 S3); empty for a legacy launch. Additive, no version bump: an old
	// meta reads as legacy.
	GuestRelease string `json:"guest_release,omitempty"`
	// HostCPU is the capturing host's CPU fingerprint (host_cpu.go, D2 S5);
	// empty when it could not be read, and in metas from before S5.
	HostCPU string `json:"host_cpu,omitempty"`
	// BusyHostStop records a busy capture on a planned host stop (S5,
	// busy_hibernate.go): the guest was not fenced. It loads only onto its
	// own current release. Additive, no version bump: a reader that predates
	// it ignores the field, so a rollback must discard busy snapshots (the
	// commit message says how).
	BusyHostStop bool `json:"busy_host_stop,omitempty"`
}

// snapshotConfinement is the confinement a snapshot was captured under.
func (meta *SnapshotMeta) snapshotConfinement() string {
	if meta.Confinement == "" {
		return ConfinementDirect
	}
	return meta.Confinement
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
	DiskBytes       int64  `json:"disk_bytes,omitempty"`
	DiskBytesMethod string `json:"disk_bytes_method,omitempty"`
	CreatedAt       string `json:"created_at,omitempty"`
	ResumeDeadline  string `json:"resume_deadline,omitempty"`
	LoadMS          int64  `json:"load_ms,omitempty"`
	WakeToHealthyMS int64  `json:"wake_to_healthy_ms,omitempty"`
	ClockStepMS     int64  `json:"clock_step_ms,omitempty"`
	// Credential is snapshotCredentialDelivered or snapshotCredentialRenewOnly.
	Credential string `json:"credential,omitempty"`
	// Vault: the snapshot was written into the snapshot vault.
	Vault bool `json:"vault,omitempty"`
	// SnapshotType is what the hibernate committed (Full or Diff);
	// DiffRefused says why it was not a Diff (snapshot_diff.go).
	SnapshotType string `json:"snapshot_type,omitempty"`
	ColdOrigin   bool   `json:"cold_origin,omitempty"`
	DiffRefused  string `json:"diff_refused,omitempty"`
	// GuestRelease is the release a resumed guest runs; CurrentRelease the
	// release current when the wake was validated; Behind is the receipt of
	// that comparison (D2 S3). The update after a behind resume does not
	// depend on it (the sweep resolves the current release itself).
	GuestRelease   string `json:"guest_release,omitempty"`
	CurrentRelease string `json:"current_release,omitempty"`
	Behind         bool   `json:"behind,omitempty"`
	// CacheReturn is the cache return before this hibernate's snapshot
	// (density D6 S3); nil when the hibernate did not ask for one.
	CacheReturn *CacheReturnTimeline `json:"cache_return,omitempty"`
	// Busy: a host-stop capture of a guest that answered busy (S5).
	// BusyRefused: why a host stop left a busy guest running.
	Busy        bool   `json:"busy,omitempty"`
	BusyRefused string `json:"busy_refused,omitempty"`
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
	return m.firecrackerLaunchArgsWithAPI(vmID, configPath, m.hibernateMode() == HibernateModeSnapshot)
}

// firecrackerLaunchArgsWithAPI is the boot command line with the API socket
// on or off.
func (m *Manager) firecrackerLaunchArgsWithAPI(vmID, configPath string, apiSocket bool) []string {
	if apiSocket {
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
	// A release-bound snapshot is rebuilt against its OWN release's files. A
	// release that no longer loads leaves the artifacts blank (legacy
	// defaults): validateSnapshot refuses it first (release_missing).
	if meta.GuestRelease != "" && m.releaseRetainOn() {
		if rel, err := m.releaseByID(meta.GuestRelease); err == nil {
			setLaunchRelease(&cfg, rel)
		}
	}
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
	fcConfig := m.buildFirecrackerConfig(cfg, meta.HostPort)
	// A jailed launch hashes its jail-path config, so the hash matches only
	// a snapshot taken under the same confinement.
	if mode, _ := m.decideConfinement(cfg); mode == ConfinementJailer {
		if jailConfig, _, err := rewriteFirecrackerConfigForJail(fcConfig); err == nil {
			fcConfig = jailConfig
		}
	}
	return cfg, fcConfig
}

// snapshotConfinementMismatch refuses a snapshot whose confinement is not
// the one the restore would use, or whose files another uid owns.
func (m *Manager) snapshotConfinementMismatch(dir string, meta *SnapshotMeta, cfg VMConfig) bool {
	want, _ := m.decideConfinement(cfg)
	if want != meta.snapshotConfinement() {
		return true
	}
	if want != ConfinementJailer {
		return false
	}
	uid, ok := m.readVMMUID(meta.VMID)
	if !ok || uid != meta.VMMUID {
		return true
	}
	for _, name := range []string{snapshotStateName, snapshotMemName} {
		owner, ok := m.fileOwnerFn(filepath.Join(dir, name))
		if !ok || owner != uid {
			return true
		}
	}
	return false
}

// snapshotValidation carries the facts a validation compares against.
type snapshotValidation struct {
	Now                time.Time
	FirecrackerVersion string
	HostKernelRelease  string
	Expect             VMConfig
	// CredentialDelivery means a fresh envelope will be delivered on
	// resume, so the frozen capability's renewal window does not bound it.
	CredentialDelivery bool
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
	if reason := m.busySnapshotRefusal(meta, v.Now); reason != "" {
		return reason
	}
	if !v.CredentialDelivery && meta.resumeWindowPassed(v.Now) {
		return snapshotReasonCredentialWindow
	}
	if reason := m.releaseRefusal(meta, v.Now); reason != "" {
		return reason
	}
	if meta.FirecrackerVersion == "" || meta.FirecrackerVersion != v.FirecrackerVersion {
		return snapshotReasonFirecracker
	}
	if meta.HostKernelRelease == "" || meta.HostKernelRelease != v.HostKernelRelease {
		return snapshotReasonHostKernel
	}
	if m.imageMovedSinceLaunch(meta) {
		return snapshotReasonImageMoved
	}
	if reason := cpuRefusal(meta); reason != "" {
		return reason
	}
	expectCfg, fcConfig := m.expectedLaunchConfig(meta, v.Expect)
	if m.snapshotConfinementMismatch(dir, meta, expectCfg) {
		return snapshotReasonConfinement
	}
	hash, err := launchConfigHash(fcConfig)
	if err != nil || hash != meta.LaunchConfigSHA256 {
		return snapshotReasonLaunchConfig
	}
	for _, name := range []string{snapshotStateName, snapshotMemName} {
		if info, err := os.Stat(filepath.Join(dir, name)); err != nil || !info.Mode().IsRegular() {
			return snapshotReasonFilesMissing
		}
	}
	if reason := m.diffSnapshotRefusalReason(dir, meta); reason != "" {
		return reason
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

// resumeWindowPassed reports whether the guest can no longer renew its
// frozen capability by itself.
func (meta *SnapshotMeta) resumeWindowPassed(now time.Time) bool {
	return !meta.ResumeDeadline.IsZero() && !now.Before(meta.ResumeDeadline)
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
	for _, dir := range m.snapshotAnyDirs(vmID) {
		if _, err := os.Stat(dir); err == nil {
			if err := os.RemoveAll(dir); err != nil {
				log.Printf("vmmanager: remove snapshot dir %s: %v", dir, err)
				continue
			}
			removed = true
		}
	}
	m.removeVaultVMDir(vmID)
	m.removeJailSnapshotLinks(vmID)
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
	vmID string
	dir  string
	meta *SnapshotMeta
	// bytes is on disk: from metadata, or measured now for a snapshot
	// committed before metadata carried it.
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

// listSnapshots scans <root>/*/snapshot/meta.json in every root (the vault
// when configured, then the state directory).
func (m *Manager) listSnapshots() []snapshotEntry {
	out := make([]snapshotEntry, 0)
	for _, root := range m.snapshotRoots() {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			dir := snapshotDirIn(root, entry.Name())
			meta, err := readSnapshotMeta(dir)
			if err != nil {
				continue
			}
			bytes := meta.DiskBytes
			if bytes <= 0 {
				bytes, _ = dirDiskBytes(dir)
			}
			out = append(out, snapshotEntry{vmID: entry.Name(), dir: dir, meta: meta, bytes: bytes})
		}
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
// room; the caller then uses the kill path. Committed snapshots count at
// their on-disk bytes; need is the new snapshot's logical worst case, since
// its compressed size is unknown until written. The free-space floor guards
// that worst case absolutely.
func (m *Manager) ensureSnapshotRoom(vmID string, need int64) bool {
	return m.ensureSnapshotRoomAt(m.cfg.StateDir, vmID, need)
}

// ensureSnapshotRoomAt is ensureSnapshotRoom for a snapshot that will occupy
// root: the budget counts every snapshot on the host, and the free-space floor
// is measured on the file system the new snapshot lands on (a vault has its
// own, smaller floor: only snapshots live there).
func (m *Manager) ensureSnapshotRoomAt(root, vmID string, need int64) bool {
	return m.ensureSnapshotRoomFor(root, m.isVaultRoot(root), vmID, need)
}

// ensureSnapshotRoomFor measures free space at statPath (a vault capture
// passes the path through its held directory) with the vault's floor when
// vaultFloor.
func (m *Manager) ensureSnapshotRoomFor(statPath string, vaultFloor bool, vmID string, need int64) bool {
	root := statPath
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
		if locked, _ := m.tryDiscardSnapshot(entry.vmID, "budget_evicted"); !locked {
			continue
		}
		used -= entry.bytes
	}
	if used+need > budget {
		return false
	}
	avail, err := statfsAvailable(root)
	if err != nil {
		log.Printf("vmmanager: statfs %s: %v; refusing snapshot", root, err)
		return false
	}
	floor := snapshotDiskFloorBytes
	if vaultFloor {
		floor = snapshotVaultFloorBytes
	}
	return avail-need >= floor
}

// tryLockVMOperation takes the VM's operation lock only if it is free.
func (m *Manager) tryLockVMOperation(vmID string) (func(), bool) {
	m.opMu.Lock()
	lock := m.vmOps[vmID]
	if lock == nil {
		lock = &sync.Mutex{}
		m.vmOps[vmID] = lock
	}
	m.opMu.Unlock()
	if !lock.TryLock() {
		return nil, false
	}
	return lock.Unlock, true
}

// tryDiscardSnapshot discards a VM's snapshot only if that VM's operation
// lock is free, so a discard never races its resume, capture or boot (each
// of which removes the files itself). locked=false: the VM is mid-operation.
func (m *Manager) tryDiscardSnapshot(vmID, reason string) (locked, removed bool) {
	unlock, ok := m.tryLockVMOperation(vmID)
	if !ok {
		return false, false
	}
	defer unlock()
	removed = m.discardSnapshotFiles(vmID)
	if removed {
		log.Printf("vmmanager: discarded snapshot of VM %s (%s)", vmID, reason)
	}
	return true, removed
}

// hasSnapshotFiles reports whether a committed or partial snapshot exists.
func (m *Manager) hasSnapshotFiles(vmID string) bool {
	for _, dir := range m.snapshotAnyDirs(vmID) {
		if _, err := os.Lstat(dir); err == nil {
			return true
		}
	}
	return false
}

// DiscardSnapshot removes a stopped computer's snapshot and releases its
// host-port reservation: a stopped computer cold boots, so the files are
// dead weight (design note item 44). It never blocks: a VM mid-operation is
// skipped (item 45). It reports whether files were removed.
func (m *Manager) DiscardSnapshot(vmID, reason string) bool {
	if strings.TrimSpace(vmID) == "" || strings.ContainsRune(vmID, os.PathSeparator) {
		return false
	}
	locked, removed := m.tryDiscardSnapshot(vmID, reason)
	if !locked && m.hasSnapshotFiles(vmID) {
		log.Printf("vmmanager: snapshot of VM %s kept for now (%s): a manager operation holds the VM; it removes or replaces the files itself", vmID, reason)
	}
	return removed
}

// SweepOrphanSnapshots runs once at vmctl start. It removes every
// interrupted capture (snapshot.partial/) and every committed snapshot whose
// VM keep reports false (not hibernated), releasing its port, and logs each
// removal. A nil keep removes no committed snapshot for that reason. With the
// snapshot vault configured it also removes, from the state directory, every
// committed snapshot of a computer the vault now serves: that is plaintext
// guest memory the vault exists to keep off the main file system. Only real
// directories under a root are touched (symlinks are skipped), and a VM whose
// operation lock is held is left alone (design note items 46-47). It
// returns the removed paths.
func (m *Manager) SweepOrphanSnapshots(keep func(vmID string) bool) []string {
	removed := make([]string, 0)
	for _, root := range m.snapshotRoots() {
		removed = append(removed, m.sweepSnapshotRoot(root, keep)...)
	}
	// A recorded vault root that is no longer configured and now holds no
	// snapshot is dropped from the record (finding B2).
	m.dropEmptyVaultRoots()
	return removed
}

func (m *Manager) sweepSnapshotRoot(root string, keep func(vmID string) bool) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		log.Printf("vmmanager: snapshot sweep: read %s: %v", root, err)
		return nil
	}
	isDir := func(path string) bool {
		info, err := os.Lstat(path)
		return err == nil && info.IsDir()
	}
	inVault := m.isVaultRoot(root)
	removed := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue // files and symlinks
		}
		vmID := entry.Name()
		committed, partial := snapshotDirIn(root, vmID), snapshotPartialDirIn(root, vmID)
		hasCommitted, hasPartial := isDir(committed), isDir(partial)
		if !hasCommitted && !hasPartial {
			continue
		}
		dropCommitted := hasCommitted && keep != nil && !keep(vmID)
		plaintextOfVaultComputer := false
		if hasCommitted && !inVault && m.vaultConfigured() {
			computerID := ""
			if meta, err := readSnapshotMeta(committed); err == nil {
				computerID = meta.ComputerID
			}
			plaintextOfVaultComputer = m.vaultSelected(vmID, computerID)
			dropCommitted = dropCommitted || plaintextOfVaultComputer
		}
		if !hasPartial && !dropCommitted {
			continue
		}
		unlock, ok := m.tryLockVMOperation(vmID)
		if !ok {
			log.Printf("vmmanager: snapshot sweep: VM %s is mid-operation; skipped", vmID)
			continue
		}
		if hasPartial {
			if err := os.RemoveAll(partial); err != nil {
				log.Printf("vmmanager: snapshot sweep: remove %s: %v", partial, err)
			} else {
				log.Printf("vmmanager: snapshot sweep: removed interrupted capture %s", partial)
				removed = append(removed, partial)
			}
		}
		if dropCommitted {
			if err := os.RemoveAll(committed); err != nil {
				log.Printf("vmmanager: snapshot sweep: remove %s: %v", committed, err)
			} else {
				m.releaseSnapshotPort(vmID)
				if plaintextOfVaultComputer {
					log.Printf("vmmanager: snapshot sweep: removed plaintext snapshot %s (VM %s uses the snapshot vault)", committed, vmID)
				} else {
					log.Printf("vmmanager: snapshot sweep: removed orphaned snapshot %s (VM %s is not hibernated)", committed, vmID)
				}
				removed = append(removed, committed)
			}
		}
		if inVault {
			_ = os.Remove(filepath.Join(root, vmID))
		}
		unlock()
	}
	return removed
}

// --- guest fence client --------------------------------------------------

// guestHibernatePrepareResponse is the guest's answer to prepare.
type guestHibernatePrepareResponse struct {
	Status              string    `json:"status"`
	Outcome             string    `json:"outcome,omitempty"`
	Error               string    `json:"error,omitempty"`
	Reason              string    `json:"reason,omitempty"`
	CapabilityExpiresAt time.Time `json:"capability_expires_at,omitempty"`
	GuestBuild          string    `json:"guest_build,omitempty"`
	CredentialResume    bool      `json:"credential_resume,omitempty"`
	RunningRuns         int       `json:"running_runs,omitempty"`
	InflightWork        int       `json:"inflight_work,omitempty"`
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
	hostauth.MarkInternal(req)
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
		return resp, snapshotReasonGuestBusy, fmt.Errorf("guest busy: running_runs=%d selfdev_active=%d reason=%q", resp.RunningRuns, resp.SelfdevActiveOps, resp.Reason)
	case status == http.StatusNotFound || status == http.StatusMethodNotAllowed:
		return resp, snapshotReasonGuestNoEndpoint, fmt.Errorf("guest answered %d", status)
	default:
		return resp, snapshotReasonGuestPrepare, fmt.Errorf("guest answered %d: %s", status, resp.Error)
	}
}

// guestHibernateResumed tells a resumed guest the host wall time and, when
// envelope is set, hands it a fresh credential envelope; it steps its clock,
// installs the credential (or renews), and reopens admission. The envelope
// travels only in this host-only request body.
func guestHibernateResumed(ctx context.Context, hostURL string, now time.Time, envelope string) (guestHibernateResumedResponse, error) {
	var resp guestHibernateResumedResponse
	body := map[string]any{"host_unix_nano": now.UnixNano()}
	if envelope != "" {
		body["credential_envelope"] = envelope
	}
	status, err := postGuestJSON(ctx, hostURL+guestHibernateResumedPath, body, &resp)
	if err != nil {
		return resp, err
	}
	if status != http.StatusOK || resp.Status != "resumed" {
		return resp, fmt.Errorf("guest answered %d: %s", status, resp.Error)
	}
	return resp, nil
}
