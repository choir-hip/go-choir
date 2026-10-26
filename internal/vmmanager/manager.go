// Package vmmanager provides the Firecracker VM lifecycle manager for Node B.
//
// This package manages the full lifecycle of Firecracker microVMs:
// boot, health checking, stop, hibernate, resume, and force-kill.
// It is the concrete runtime that vmctl delegates to when managing
// VM-backed autoputer workloads (VAL-VM-010, VAL-VM-011).
//
// Build: 2026-04-12-01 - Force rebuild after Nix cache issue
//
// The VMManager is designed to run only on Linux with KVM available.
// On non-Linux platforms, the stub implementations return graceful
// errors so that local development and testing remain possible.
//
// Key invariants:
//   - Guest images are repo-built artifacts; the manager never generates
//     or modifies guest images at runtime (VAL-VM-010).
//   - Provider credentials are never injected into guest environment,
//     config files, mounted secrets, or process arguments (VAL-VM-011).
//   - VM IDs are stable identifiers that survive stop/hibernate/resume
//     cycles for the same user, preserving user state (VAL-CROSS-116).
//   - Crash recovery does not duplicate canonical effects; each VM
//     carries a monotonically increasing epoch counter that increments
//     on fresh boot but stays the same on resume (VAL-CROSS-117).
package vmmanager

import (
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/buildinfo"
)

// VMState represents the lifecycle state of a Firecracker VM.
type VMState string

const (
	// StatePending means the VM is being prepared but not yet launched.
	StatePending VMState = "pending"

	// StateRunning means the VM is actively running and healthy.
	StateRunning VMState = "running"

	// StateStopped means the VM has been cleanly shut down.
	StateStopped VMState = "stopped"

	// StateHibernated means the VM's memory and state have been saved to disk
	// and can be resumed. This is a future capability; for now the manager
	// uses stop/resume semantics with persistent state.
	StateHibernated VMState = "hibernated"

	// StateFailed means the VM has crashed or failed to start.
	StateFailed VMState = "failed"
)

// dataImageSizeMB is the minimum mutable data disk size for each VM (32 GB).
// Worker self-development runs and long-lived user computers need room for
// Texture/Dolt state, source checkouts, Go module/build caches, verifier
// artifacts, exported packages, and ordinary user growth without wedging boot.
const dataImageSizeMB = 32768

const (
	consoleLogName        = "console.log"
	consoleLogMaxBytes    = 1 << 20
	consoleLogGenerations = 4
)

// VMConfig holds the configuration for launching a single Firecracker VM.
type VMConfig struct {
	// VMID is the unique identifier for this VM instance.
	VMID string

	// KernelImagePath is the path to the repo-built guest kernel image.
	KernelImagePath string

	// InitrdPath is the path to the guest initramfs (optional).
	// If set, the initrd is passed to Firecracker for loading ext4
	// and other modules before mounting the root filesystem.
	InitrdPath string

	// RootfsPath is the path to the repo-built guest root filesystem image.
	// With the upstream microvm.nix approach, this is the boot disk used
	// as the guest root filesystem.
	RootfsPath string

	// StoreDiskPath is the path to the erofs store disk image built by
	// microvm.nix. When set, the Firecracker config includes this as a
	// read-only virtio-blk drive that the NixOS init mounts at /nix/store.
	StoreDiskPath string

	// Empty means use the manager default.
	KernelParams string

	// GuestRelease is the id of the guest release this launch was made from
	// (guest_release.go); empty for a legacy launch from the manager's
	// stable paths. It is never carried into a refresh.
	GuestRelease string `json:"guest_release,omitempty"`

	// GuestPort is the port the guest autoputer runtime listens on inside
	// the VM. The host-side vsock or tap networking maps this to a
	// host-accessible URL.
	GuestPort int

	// VsockPort is the vsock port for host-guest communication.
	VsockPort uint32

	// MachineCPUCount is the number of vCPUs for the guest.
	MachineCPUCount int

	// MachineMemSizeMib is the memory size in MiB for the guest.
	MachineMemSizeMib int

	// MemoryHotplugMiB is the virtio-mem hotplug region vmctl selected for
	// this computer (0: no device). It is a boot-time machine shape like
	// MachineMemSizeMib; the region boots unplugged (memory_hotplug.go).
	MemoryHotplugMiB int

	// PersistentDir is the host directory where per-VM persistent state
	// (user data, task state) is stored. This directory is mounted into
	// the guest and survives stop/resume cycles (VAL-CROSS-116).
	PersistentDir string

	// GatewayToken is the autoputer credential token for authenticating to
	// the host-side gateway. Written to a file in the persistent directory
	// so the guest init script can read and set RUNTIME_GATEWAY_TOKEN.
	// This is NOT a provider credential — it's a autoputer identity token
	// that allows the guest autoputer to call the gateway (VAL-VM-011
	// still holds: no provider credentials in the guest).
	GatewayToken string

	// ComputerKind and related identity fields are product-level metadata for
	// the guest's source-lineage projection. They are not secrets.
	ComputerKind  string
	OwnerID       string
	DesktopID     string
	ComputerID    string
	RealizationID string
	// ComputerCredentialEnvelope is a short-lived signed bootstrap envelope
	// for guest-to-corpusd event capability exchange. It is scoped to VMID and
	// contains no platform signing key.
	ComputerCredentialEnvelope string
	credentialDiskPath         string

	// Epoch is the monotonically increasing boot counter for this VM.
	// On fresh boot, the epoch increments. On resume from hibernate,
	// the epoch stays the same. Callers can use epoch to detect whether
	// a VM went through a fresh boot vs. a resume (VAL-CROSS-117).
	Epoch int64

	// MaintenanceHold boots the guest with RUNTIME_MAINTENANCE_HOLD=1 so the
	// runtime refuses run admission and skips every rewake sweep while the
	// host holds the computer (audited maintenance/recovery boot path).
	MaintenanceHold bool

	// RecoveryReplayOnly boots the guest into the B14 replay-only drive: the
	// runtime materializes the canonical tape to the platform head and exits
	// without starting the runtime, reconciling, or appending (authorized
	// recovery boot only).
	RecoveryReplayOnly bool

	// BootKind labels the boot attempt for the per-boot timeline receipt
	// (cold | refresh | recover | resume). Callers set it before bootVM;
	// bootVM defaults empty values to "cold".
	BootKind string

	// OfflineMaintenance, when set on a refresh, runs after the Firecracker
	// process is killed and before the boot, with the VM's data image path,
	// under the per-VM operation lock (docs/problems/guest-store-compaction-
	// scheduled-2026-10-20.md). It must leave a bootable image even when it
	// fails; the boot happens either way. Refresh takes it from the overrides
	// only and never stores it in an instance config.
	OfflineMaintenance func(dataImagePath string) error `json:"-"`

	// PostMaintenanceCredential, when set on a refresh with
	// OfflineMaintenance, re-mints the boot envelope after the hook returns
	// and before the boot writes the credential disk: a long hook outlives
	// the envelope minted before the kill (docs/problems/offline-maintenance-
	// outlives-the-boot-credential-2026-10-24.md). It is not called without a
	// hook, nor after an unbootable one; when it fails, the boot keeps the
	// envelope it was given. Like the hook, it is never stored.
	PostMaintenanceCredential func() (string, error) `json:"-"`

	// WarmnessClass is the class vmctl booted the computer under ("test"
	// for a test account's computer). It is a boot input only: the visual
	// capture grant reads it (visual_capture.go).
	WarmnessClass string

	// NewComputer marks the first creation of a computer (vmctl's mint
	// path). Only such a boot may clone its data image from the template
	// (data_image_template.go); every other boot keeps today's path.
	NewComputer bool
}

// ErrOfflineMaintenanceUnbootable is returned (wrapped) by an
// OfflineMaintenance hook that could not leave a bootable image, for example
// when restoring its backup failed. Refresh then does not boot the VM.
var ErrOfflineMaintenanceUnbootable = errors.New("offline maintenance left no bootable image")

// VMInstance represents a running or stopped Firecracker VM.
type VMInstance struct {
	// Config is the configuration used to launch this VM.
	Config VMConfig

	// State is the current lifecycle state.
	State VMState

	// HostURL is the URL where this VM's autoputer runtime is reachable
	// from the host. On the Firecracker tap path this is the guest IP on the
	// per-VM /30 subnet (for example "http://10.200.X.2:8085").
	HostURL string

	// PID is the Firecracker process ID (0 if not running).
	PID int

	// StartedAt is when the VM was last started.
	StartedAt time.Time

	// LastHealthCheck is when the VM was last health-checked.
	LastHealthCheck time.Time

	// LastHealthyAt is the last time the guest responded healthy.
	LastHealthyAt time.Time

	// Healthy is the result of the last health check.
	Healthy bool

	// Behind records that this guest was resumed from a snapshot onto a guest
	// release older than the current one (D2). A receipt of what validation
	// saw; health recomputes it from the live current release.
	Behind bool

	// cmd is the running Firecracker process (nil when stopped).
	cmd *exec.Cmd

	// cancel is a function to clean up the process (nil when stopped).
	done chan struct{}
	// BootTimeline is the per-boot receipt for the in-flight or most recent
	// boot attempt (S0a). Persisted under the VM state dir on terminal
	// observation; nil for instances reattached without a boot.
	BootTimeline *BootTimeline

	// confinement is how the running Firecracker was confined (jailer or
	// direct); read from confinement.json on reattach.
	confinement *ConfinementReceipt

	// coldOrigin is true when this vmctl launched the running Firecracker
	// from a config (a cold boot), false for a snapshot restore and for a
	// reattached process, whose origin it did not see. Only a cold-origin
	// guest may take a Diff snapshot (snapshot_diff.go).
	coldOrigin bool
}

// ManagerConfig holds the global configuration for the VM manager.
type ManagerConfig struct {
	// FirecrackerBinPath is the path to the Firecracker binary.
	// If empty, the manager searches $PATH.
	FirecrackerBinPath string
	// MkfsExt4Path formats the tiny per-realization appender credential disk.
	MkfsExt4Path string

	// KernelImagePath is the default kernel image for all VMs.
	KernelImagePath string

	// InitrdPath is the path to the guest initramfs (optional).
	// If set, the initrd is passed to Firecracker for module loading.
	InitrdPath string

	// RootfsPath is the default rootfs image for all VMs. With the
	// upstream microvm.nix approach, this is the boot disk used as /.
	RootfsPath string

	// StoreDiskPath is the path to the erofs store disk image built by
	// microvm.nix. This disk contains the shared read-only nix store
	// closure. All VMs reference the same store disk file; guests share its
	// host page cache, not guest memory (KSM is off on the host, SEC-09:
	// merged guest pages are a cross-tenant side channel).
	// When set, the Firecracker config includes this as a virtio-blk drive
	// mounted read-only at /nix/store by the guest init.
	StoreDiskPath string

	// KernelParams is the upstream microvm.nix kernel parameter set
	// (for example root=fstab, init=/nix/store/.../init, regInfo=...).
	// vmmanager appends its per-VM runtime parameters to this base cmdline.
	KernelParams string

	// KernelParamsFile is VM_KERNEL_PARAMS_FILE. When set and readable, a
	// launch reads the kernel parameters from it at that moment instead of
	// using the value cached at vmctl start (D2 S1b, H1).
	KernelParamsFile string

	// SnapshotReleaseRetain (VM_SNAPSHOT_RELEASE_RETAIN=on) is the D2 switch:
	// cold boots launch from the current guest release directory and
	// snapshots may resume onto their own release after a deploy
	// (guest_release.go). Off, launches are byte-identical to before and every
	// release-bound snapshot refuses (release_retain_off).
	SnapshotReleaseRetain bool

	// GuestReleasesDir is VM_GUEST_RELEASES_DIR, the release directories.
	GuestReleasesDir string

	// GuestReleaseSupportWindow is VM_GUEST_RELEASE_SUPPORT_WINDOW: how long a
	// superseded release's snapshots may still resume (default 168h).
	GuestReleaseSupportWindow time.Duration

	// GuestReleaseLink is the stable symlink the guest image is launched
	// from (VM_GUEST_RELEASE_LINK, /var/lib/go-choir/guest on Node B). Each
	// launch records its target; a snapshot whose launch target is not the
	// link's target now is refused (D2 S1b, H6). Empty disables the guard.
	GuestReleaseLink string

	// GuestPort is the port the guest autoputer listens on.
	GuestPort int

	// BaseVsockPort is the starting vsock port for VM assignment.
	BaseVsockPort uint32

	// MachineCPUCount is the default number of vCPUs per VM.
	MachineCPUCount int

	// MachineMemSizeMib is the default memory per VM.
	MachineMemSizeMib int

	// HostBasePort is the starting host port for VM autoputer listeners.
	// Each VM gets a unique port starting from this value.
	HostBasePort int

	// StateDir is the directory where VM state (pid files, persistent
	// data, epoch counters) is stored.
	StateDir string

	// HealthCheckInterval is how often the manager checks VM health.
	HealthCheckInterval time.Duration

	// HealthCheckTimeout is the per-check HTTP timeout.
	HealthCheckTimeout time.Duration

	// BootReadyTimeout is how long BootVM/ResumeVM wait for the guest
	// to answer /health before treating the launch as failed. Replay progress
	// (503 ReplayInProgress with an advancing sequence) extends this timeout
	// indefinitely; only a STALL (no sequence advance for ReplayStallTimeout)
	// fails the boot.
	BootReadyTimeout time.Duration

	// ReplayStallTimeout is how long a replaying guest may go without advancing
	// its replay sequence before waitForGuestReady treats it as stuck and fails
	// the boot (B5/B10). It must comfortably exceed a payload page fetch + a
	// Dolt checkpoint commit plus a guest process restart window.
	ReplayStallTimeout time.Duration

	// SourceSeedRepoPath seeds files/Source/platform in a newly created
	// per-VM data image. The engineering desk's assignment preflight
	// (capsule.Executor.PreflightSourceSnapshot) requires that path to be a
	// clean git checkout; without it the desk hot-loops ErrDeferUnprocessed
	// on the cast occurrence and self-development operations can never reach
	// awaiting_approval. Set to the host's deployed source checkout
	// (/opt/go-choir on Node B); empty disables seeding (legacy behavior —
	// the guest then must obtain Source some other way).
	SourceSeedRepoPath string

	// BalloonFreePageReporting adds a never-inflated virtio balloon with
	// free page reporting to every cold boot (VM_BALLOON_FREE_PAGE_REPORTING;
	// off by default). See balloon.go.
	BalloonFreePageReporting bool

	// DataDiscard adds virtio-blk discard (and the Sync io_engine it needs)
	// to the data drive (VM_DATA_DISCARD; off by default, and honored only
	// when the Firecracker binary is 1.17.0 or newer). See data_discard.go.
	// It is the pinned build's gate; FirecrackerNext.DataDiscard is the next
	// build's.
	DataDiscard bool

	// DataDiscardOnly narrows discard to the computers it names
	// (VM_DATA_DISCARD_ONLY; computer_select.go). Empty: every computer whose
	// build allows it.
	DataDiscardOnly []string

	// GuestReclaimOnly turns on the guest's DAMON_RECLAIM for the computers
	// it names (VM_GUEST_RECLAIM_ONLY; guest_reclaim.go); empty: off.
	// GuestReclaimMinAgeS overrides its 300 s coldness floor.
	GuestReclaimOnly    []string
	GuestReclaimMinAgeS int

	// CapsuleTunnelEnforceOnly runs the capsule egress tunnel-identity check
	// in enforce on the computers it names (VM_CAPSULE_TUNNEL_ENFORCE_ONLY;
	// capsule_tunnel_identity.go); everyone else keeps the image's observe.
	CapsuleTunnelEnforceOnly []string

	// CheckpointCadenceOnly runs the automatic checkpoint cadence on the
	// computers it names (VM_CHECKPOINT_CADENCE_ONLY; checkpoint_cadence.go),
	// with these values; empty: off. The guest compiles the cadence off.
	CheckpointCadenceOnly        []string
	CheckpointCadenceEveryEvents int
	CheckpointCadenceEvery       string

	// FirecrackerNext is a second VMM build launched only for the computers
	// it selects (VM_FIRECRACKER_NEXT_*; vmm_next.go). Empty BinPath: off.
	FirecrackerNext FirecrackerNextConfig

	// BalloonStatsOnly turns balloon statistics on for the computers it
	// names (VM_BALLOON_STATS_ONLY; computer_select.go); empty: off for
	// every computer. BalloonStatsPollingIntervalS is their interval in
	// seconds (VM_BALLOON_STATS_POLLING_INTERVAL_S; 0 means the default, 5).
	// Only meaningful with the balloon on. See balloon_stats.go.
	BalloonStatsOnly             []string
	BalloonStatsPollingIntervalS int

	// HibernateMode selects hibernate behavior (VM_HIBERNATE_MODE):
	// "kill" (default) stops Firecracker and cold boots on wake; "snapshot"
	// launches with an API socket and hibernates by memory snapshot, falling
	// back to kill (snapshot.go; S3).
	HibernateMode string
	// SnapshotBudgetGiB bounds the logical bytes of all committed snapshots
	// (VM_SNAPSHOT_BUDGET_GIB, default DefaultSnapshotBudgetGiB).
	SnapshotBudgetGiB int
	// SnapshotMaxAge bounds how long a snapshot stays resumable
	// (VM_SNAPSHOT_MAX_AGE, default DefaultSnapshotMaxAge).
	SnapshotMaxAge time.Duration
	// SnapshotType (VM_SNAPSHOT_TYPE) is full (default) or diff; diff
	// applies only to the computers SnapshotDiffOnly (VM_SNAPSHOT_DIFF_ONLY)
	// selects, and only when every L3a condition holds (snapshot_diff.go).
	SnapshotType     string
	SnapshotDiffOnly []string
	// SnapshotVaultDir (VM_SNAPSHOT_VAULT_DIR) is the mount of an
	// ephemeral-key dm-crypt volume. When set, snapshots of the selected
	// computers are written there and only there, and refused when the path
	// does not probe as such a mount. Empty (default): snapshots go to the
	// state directory as before. See snapshot_vault.go.
	SnapshotVaultDir string
	// SnapshotVaultOnly (VM_SNAPSHOT_VAULT_ONLY) limits the vault to these VM
	// ids or computer ids; empty means every computer when the dir is set.
	SnapshotVaultOnly []string
	// SnapshotVaultAcceptAnyCrypt (VM_SNAPSHOT_VAULT_ACCEPT_ANY_CRYPT) also
	// accepts a LUKS mapping, whose key slot is on disk. Weaker; off by default.
	SnapshotVaultAcceptAnyCrypt bool

	// Jailer runs each Firecracker under the Firecracker jailer (VMCTL_JAILER;
	// off by default). See confinement.go.
	Jailer JailerConfig

	// ClaudeDelegate is the Claude delegate allowlist and window
	// (CHOIR_CLAUDE_DELEGATE_*; empty by default). See claude_delegate.go.
	ClaudeDelegate ClaudeDelegateConfig

	// MemoryHotplug lets a computer vmctl selected boot with a virtio-mem
	// device (VM_MEMORY_HOTPLUG; off by default). See memory_hotplug.go.
	MemoryHotplug bool
	// MemoryHotplugBlockMiB and MemoryHotplugSlotMiB are Firecracker's block
	// and slot sizes (VM_MEMORY_HOTPLUG_BLOCK_MIB/_SLOT_MIB; 0 = 2 and 128).
	MemoryHotplugBlockMiB int
	MemoryHotplugSlotMiB  int

	// GuestDNSPort is the host resolver's loopback port (VMCTL_GUEST_DNS_PORT;
	// 0, the default, means none). With it set, a guest's DNS to its tap host
	// address port 53 is DNAT'd there (SEC-10, hostfirewall.go).
	GuestDNSPort int
	// GuestDNSOnly names the computers that boot pointed at that resolver
	// (VMCTL_GUEST_DNS_ONLY; empty selects none). See guest_dns.go.
	GuestDNSOnly []string

	// VisualCapture is the visual capture grant switch
	// (CHOIR_VISUAL_CAPTURE_COMPUTERS; empty by default). See
	// visual_capture.go.
	VisualCapture VisualCaptureConfig

	// CapsuleMirrors is the capsule package mirror grant switch
	// (CHOIR_CAPSULE_MIRROR_*; empty by default). See capsule_mirrors.go.
	CapsuleMirrors CapsuleMirrorConfig

	// HostFetch is the host fetch route grant switch
	// (CHOIR_HOST_FETCH_VMS; empty by default). See host_fetch.go.
	HostFetch HostFetchConfig

	// EventTable is the store.event_table grant switch
	// (CHOIR_EVENT_TABLE_COMPUTERS; empty by default). See
	// event_table_grant.go.
	EventTable EventTableConfig

	// DataImageTemplate is the new-computer data image template switch
	// (VMCTL_DATA_IMAGE_TEMPLATE; off by default). See
	// data_image_template.go.
	DataImageTemplate DataImageTemplateConfig
}

// DefaultManagerConfig returns a sensible default configuration.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		FirecrackerBinPath:  "firecracker",
		MkfsExt4Path:        "mkfs.ext4",
		GuestPort:           8085,
		BaseVsockPort:       6000,
		MachineCPUCount:     2,
		MachineMemSizeMib:   512,
		HostBasePort:        9000,
		StateDir:            "/var/lib/go-choir/vm-state",
		HealthCheckInterval: 15 * time.Second,
		HealthCheckTimeout:  3 * time.Second,
		BootReadyTimeout:    20 * time.Second,
		ReplayStallTimeout:  120 * time.Second,
		Jailer:              defaultJailerConfig(),
	}
}

// Manager manages Firecracker VM lifecycles on Node B.
// It provides thread-safe VM boot, stop, resume, and health-check
// operations that vmctl delegates to.
type Manager struct {
	cfg      ManagerConfig
	mu       sync.RWMutex
	vms      map[string]*VMInstance // vmID → instance
	nextPort int                    // next host port to assign
	opMu     sync.Mutex
	vmOps    map[string]*sync.Mutex // serializes lifecycle operations per VM identity

	healthCancel chan struct{}
	healthDone   chan struct{}

	// receiptMu serializes destruction receipt appends (receipts.go);
	// receiptMaxBytes bounds the receipt file (0 = default).
	receiptMu       sync.Mutex
	receiptMaxBytes int64

	// snapshotPortMu guards snapshot host-port reservations and the pending
	// snapshot-fallback reasons recorded for the next cold boot (S3).
	snapshotPortMu   sync.Mutex
	snapshotPorts    map[int]string
	snapshotFallback map[string]string

	// firecrackerStarter replaces the process start in tests; nil runs the
	// real binary. A jailed launch passes the jailer's arguments.
	firecrackerStarter func(vmID string, args []string) error

	// Test seams for the cache return before a snapshot (snapshot_cache_return.go):
	// the clock the fence budget reads, the settle's sleep, the call's
	// bound, a VM's RSS reader, and a hook at the Full or Diff decision.
	cacheReturnClock     func() time.Time
	cacheReturnSleep     func(time.Duration)
	cacheReturnTimeout   time.Duration
	cacheReturnRSSReader func(vmID string) func() (uint64, bool)
	onDiffDecision       func()

	// hostNet runs ip/iptables for guest taps (hostfirewall.go); tests
	// replace it. Reached through m.net().
	hostNetOnce sync.Once
	hostNet     *hostNet

	// Jailer state (confinement.go). jailerHostErr is nil once Start has
	// prepared the host for jailed launches; while it is set, every launch
	// is direct and says why.
	jailerMu      sync.RWMutex
	jailerHostErr error
	uidMu         sync.Mutex
	// jailSharedMu serializes building, linking and sweeping the shared
	// read-only artifacts under <jail base>/.shared (jail_shared.go).
	jailSharedMu sync.Mutex
	// launchedVMMs is the VMM build each VM last launched with, mirrored in
	// its state dir (vmm_next.go).
	launchedMu   sync.Mutex
	launchedVMMs map[string]vmmBuild
	cgroupRoot   string
	// procRoot is /proc unless a test replaces it (snapshot_diff.go).
	procRoot string
	// chownFn and fileOwnerFn are chown(2) and the owner lookup; tests that
	// cannot change owners replace them.
	chownFn     func(path string, uid, gid int) error
	fileOwnerFn func(path string) (int, bool)

	// vaultProbe checks that the snapshot vault is an encrypted mount
	// (snapshot_vault.go); tests replace it.
	vaultProbe func(dir string, acceptAnyCrypt bool) error
	// vaultHandleCheck verifies the vault directory a capture holds open
	// (snapshot_vault.go); nil is the production check. Tests replace it.
	vaultHandleCheck func(h *os.File, dir string, acceptAnyCrypt bool) error
	// vaultRootsMu serializes changes to the vault root record.
	vaultRootsMu sync.Mutex

	// Data image template (data_image_template.go). templateMu guards the
	// builds in flight; templateBuildMu serializes the builds themselves.
	// None of it is m.mu: a template is never built under the global lock.
	templateMu      sync.Mutex
	templateFlights map[string]*dataImageTemplateFlight
	templateBuildMu sync.Mutex
	// templateWait bounds a creation's wait for a build (0 = default);
	// templateStagingHook and templateCloneFn are test seams (nil = none,
	// FICLONE).
	templateWait        time.Duration
	templateStagingHook func(staging string) error
	templateCloneFn     func(dst, src *os.File) error
}

func (m *Manager) StateDir() string {
	if m == nil {
		return ""
	}
	return m.cfg.StateDir
}

const (
	vmSubnetSecondOctetBase  = 200
	vmSubnetSecondOctetCount = 16
	vmSubnetCapacity         = vmSubnetSecondOctetCount * 256
)

// NewManager creates a new VM manager with the given configuration.
func NewManager(cfg ManagerConfig) *Manager {
	if cfg.HostBasePort <= 0 {
		cfg.HostBasePort = 9000
	}
	m := &Manager{
		cfg:         cfg,
		vms:         make(map[string]*VMInstance),
		vmOps:       make(map[string]*sync.Mutex),
		nextPort:    cfg.HostBasePort,
		cgroupRoot:  defaultCgroupRoot,
		chownFn:     os.Lchown,
		fileOwnerFn: defaultFileOwner,
		vaultProbe:  probeSnapshotVault,
	}
	if cfg.Jailer.Enabled {
		m.jailerHostErr = errJailerHostNotPrepared
	}
	return m
}

func (m *Manager) lockVMOperation(vmID string) func() {
	m.opMu.Lock()
	lock := m.vmOps[vmID]
	if lock == nil {
		lock = &sync.Mutex{}
		m.vmOps[vmID] = lock
	}
	m.opMu.Unlock()

	lock.Lock()
	return lock.Unlock
}

func (m *Manager) guestAndHostIP(hostPort int) (guestIP, hostIP string) {
	ordinal := hostPort - m.cfg.HostBasePort
	if ordinal < 0 {
		ordinal = 0
	}
	ordinal %= vmSubnetCapacity
	second := vmSubnetSecondOctetBase + ordinal/256
	third := ordinal % 256
	return fmt.Sprintf("10.%d.%d.2", second, third), fmt.Sprintf("10.%d.%d.1", second, third)
}

func (m *Manager) reserveHostURLLocked(hostURL string) {
	hostPort, ok := m.hostPortFromHostURL(hostURL)
	if !ok {
		return
	}
	if m.nextPort <= hostPort {
		m.nextPort = hostPort + 1
	}
}

func (m *Manager) hostPortFromHostURL(hostURL string) (int, bool) {
	u, err := url.Parse(hostURL)
	if err != nil {
		return 0, false
	}
	host := u.Hostname()
	parts := strings.Split(host, ".")
	if len(parts) == 4 && parts[0] == "10" && parts[3] == "2" {
		second, err := strconv.Atoi(parts[1])
		if err != nil || second < vmSubnetSecondOctetBase || second >= vmSubnetSecondOctetBase+vmSubnetSecondOctetCount {
			return 0, false
		}
		third, err := strconv.Atoi(parts[2])
		if err != nil || third < 0 || third > 255 {
			return 0, false
		}
		ordinal := (second-vmSubnetSecondOctetBase)*256 + third
		return m.cfg.HostBasePort + ordinal, true
	}
	if len(parts) == 4 && parts[0] == "172" && parts[2] == "0" && parts[3] == "2" {
		subnetIndex, err := strconv.Atoi(parts[1])
		if err != nil || subnetIndex <= 0 {
			return 0, false
		}
		return m.cfg.HostBasePort + subnetIndex - 1, true
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < m.cfg.HostBasePort {
		return 0, false
	}
	return port, true
}

func (m *Manager) allocateHostPortLocked(vmID string) (int, error) {
	const maxAttempts = vmSubnetCapacity
	for i := 0; i < maxAttempts; i++ {
		hostPort := m.nextPort
		m.nextPort++
		// A hibernated VM's snapshot holds this subnet's IP and MAC in guest
		// memory; never hand it to another VM (S3).
		if m.snapshotPortHeldByOther(vmID, hostPort) {
			continue
		}
		if m.reclaimHostPortIfStale(vmID, hostPort) {
			return hostPort, nil
		}
	}
	return 0, fmt.Errorf("no free VM host subnet found after %d attempts", maxAttempts)
}

func (m *Manager) reclaimHostPortIfStale(vmID string, hostPort int) bool {
	_, hostIP := m.guestAndHostIP(hostPort)
	conflicts := m.interfacesForIPv4(hostIP)
	if len(conflicts) == 0 {
		return true
	}
	for _, iface := range conflicts {
		if !strings.HasPrefix(iface, "vm-") {
			log.Printf("vmmanager: host subnet %s is held by non-VM interface %s; skipping", hostIP, iface)
			return false
		}
		if m.interfaceHasCarrier(iface) {
			return false
		}
		log.Printf("vmmanager: deleting stale tap %s holding %s before assigning VM %s", iface, hostIP, vmID)
		m.deleteTapDevice(iface)
	}
	return len(m.interfacesForIPv4(hostIP)) == 0
}

func (m *Manager) interfacesForIPv4(ip string) []string {
	ipBin := findBinary("ip", "/run/current-system/sw/bin/ip")
	out, err := exec.Command(ipBin, "-o", "-4", "addr", "show", "to", ip+"/32").Output()
	if err != nil {
		return nil
	}
	return parseIPAddrShowInterfaces(out)
}

func parseIPAddrShowInterfaces(out []byte) []string {
	lines := strings.Split(string(out), "\n")
	ifaces := make([]string, 0, len(lines))
	seen := make(map[string]struct{}, len(lines))
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		iface := strings.TrimSpace(fields[1])
		if iface == "" {
			continue
		}
		if _, ok := seen[iface]; ok {
			continue
		}
		seen[iface] = struct{}{}
		ifaces = append(ifaces, iface)
	}
	return ifaces
}

func (m *Manager) interfaceHasCarrier(iface string) bool {
	ipBin := findBinary("ip", "/run/current-system/sw/bin/ip")
	out, err := exec.Command(ipBin, "-o", "link", "show", "dev", iface).Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "LOWER_UP")
}

// Start begins the manager's background health checking loop.
func (m *Manager) Start() {
	m.mu.Lock()
	m.healthCancel = make(chan struct{})
	m.healthDone = make(chan struct{})
	m.mu.Unlock()

	// Before any VM is reattached: drop rules of vanished taps and install
	// the guest forward default-deny (hostfirewall.go).
	m.net().reconcileAtStart()
	m.loadSnapshotReservations()
	if m.cfg.Jailer.Enabled {
		err := m.prepareJailerHost()
		m.jailerMu.Lock()
		m.jailerHostErr = err
		m.jailerMu.Unlock()
		if err != nil {
			log.Printf("vmmanager: JAILER UNAVAILABLE: %v; every VM launches direct (unconfined) until this is fixed", err)
		} else {
			log.Printf("vmmanager: jailer ready: bin=%s exec_file=%s chroot_base=%s uids=%d+%d cgroup_parent=%s only=%v",
				m.cfg.Jailer.BinPath, m.jailedExec(), m.jailChrootBase(), m.cfg.Jailer.UIDBase, m.cfg.Jailer.UIDCount, m.cfg.Jailer.CgroupParent, m.cfg.Jailer.Only)
		}
	}
	if m.cfg.DataImageTemplate.Enabled && strings.TrimSpace(m.cfg.StoreDiskPath) != "" {
		// A cache, not owner work: build the current template off every
		// lock so the first creation after a deploy finds it ready.
		go m.warmDataImageTemplate()
	}
	go m.healthCheckLoop()
	log.Printf("vmmanager: started with state_dir=%s base_port=%d hibernate_mode=%s jailer=%t", m.cfg.StateDir, m.cfg.HostBasePort, m.hibernateMode(), m.cfg.Jailer.Enabled)
}

// Stop shuts down all managed VMs and stops the health checker.
func (m *Manager) Stop() {
	m.stop(true)
}

// StopHealthChecks stops only the manager's background goroutines. It leaves
// Firecracker child processes running so a replacement vmctl process can
// reattach to them from durable ownership metadata.
func (m *Manager) StopHealthChecks() {
	m.stop(false)
}

// ReadGatewayToken returns the host-side bootstrap token written for a VM.
// This is a autoputer identity credential, not a provider credential. Callers use
// it only to reconcile the gateway's token hash store after gateway restart.
func (m *Manager) ReadGatewayToken(vmID string) (string, error) {
	tokenPath := filepath.Join(m.cfg.StateDir, vmID, "persist", "gateway-token")
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return "", fmt.Errorf("read gateway token for VM %s: %w", vmID, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func (m *Manager) resolveGatewayToken(cfg VMConfig) string {
	if token := strings.TrimSpace(cfg.GatewayToken); token != "" {
		return token
	}
	if strings.TrimSpace(cfg.VMID) == "" {
		return ""
	}
	persistDir := strings.TrimSpace(cfg.PersistentDir)
	if persistDir == "" {
		persistDir = filepath.Join(m.cfg.StateDir, cfg.VMID, "persist")
	}
	data, err := os.ReadFile(filepath.Join(persistDir, "gateway-token"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func (m *Manager) stop(stopVMs bool) {
	m.mu.Lock()
	cancel := m.healthCancel
	done := m.healthDone
	m.mu.Unlock()

	// Stop the health checker.
	if cancel != nil {
		close(cancel)
	}
	if done != nil {
		<-done
	}

	if !stopVMs {
		log.Printf("vmmanager: stopped health checker; managed VMs left running for reattach")
		return
	}

	// Stop all VMs.
	m.mu.Lock()
	var vmIDs []string
	for id := range m.vms {
		vmIDs = append(vmIDs, id)
	}
	m.mu.Unlock()

	for _, id := range vmIDs {
		if err := m.StopVM(id); err != nil {
			log.Printf("vmmanager: stop VM %s during shutdown: %v", id, err)
		}
	}
	log.Printf("vmmanager: stopped")
}

// BootVM launches a new Firecracker VM with the given configuration.
// Returns the VM instance with its host-accessible URL populated.
//
// The guest receives only the minimum bootstrap material:
//   - The repo-built kernel and rootfs (read-only overlay)
//   - A persistent state directory for user data (read-write)
//   - The guest port to listen on
//
// Provider credentials are never included in the guest environment,
// config files, or process arguments (VAL-VM-011).
func (m *Manager) BootVM(cfg VMConfig) (*VMInstance, error) {
	unlock := m.lockVMOperation(cfg.VMID)
	defer unlock()
	return m.bootVM(cfg)
}

// ReserveBootEpoch durably allocates the next realization epoch for a VM.
// Callers that bind credentials before boot use this as the sole epoch
// authority; BootVM then verifies and consumes the reserved value.
func (m *Manager) ReserveBootEpoch(vmID string, minimum int64) (int64, error) {
	if strings.TrimSpace(vmID) == "" {
		return 0, fmt.Errorf("vmmanager: VM ID is required to reserve boot epoch")
	}
	unlock := m.lockVMOperation(vmID)
	defer unlock()

	current, err := m.loadEpoch(vmID)
	if err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("vmmanager: load boot epoch for %s: %w", vmID, err)
	}
	next := current + 1
	if next < minimum {
		next = minimum
	}
	if next < 1 {
		next = 1
	}
	if err := m.saveEpoch(vmID, next); err != nil {
		return 0, fmt.Errorf("vmmanager: persist boot epoch for %s: %w", vmID, err)
	}
	return next, nil
}

func (m *Manager) consumeBootEpoch(vmID string, supplied int64) (int64, error) {
	stored, err := m.loadEpoch(vmID)
	switch {
	case supplied > 0 && err == nil && supplied < stored:
		return 0, fmt.Errorf("vmmanager: reserved boot epoch %d is stale; current epoch is %d", supplied, stored)
	case supplied > 0 && err != nil && !os.IsNotExist(err):
		return 0, fmt.Errorf("vmmanager: load boot epoch for %s: %w", vmID, err)
	case supplied > 0 && (err != nil || supplied > stored):
		if err := m.saveEpoch(vmID, supplied); err != nil {
			return 0, fmt.Errorf("vmmanager: persist supplied boot epoch for %s: %w", vmID, err)
		}
		return supplied, nil
	case supplied > 0:
		return supplied, nil
	case err != nil && !os.IsNotExist(err):
		return 0, fmt.Errorf("vmmanager: load boot epoch for %s: %w", vmID, err)
	default:
		next := stored + 1
		if next < 1 {
			next = 1
		}
		if err := m.saveEpoch(vmID, next); err != nil {
			return 0, fmt.Errorf("vmmanager: persist boot epoch for %s: %w", vmID, err)
		}
		return next, nil
	}
}

func (m *Manager) bootVM(cfg VMConfig) (*VMInstance, error) {
	// A new computer's data image template is found, or built, before the
	// global lock: only the clone below runs under it.
	templatePick := m.dataImageTemplateForBoot(cfg, dataImageSizeMB)

	m.mu.Lock()
	locked := true
	defer func() {
		if locked {
			m.mu.Unlock()
		}
	}()

	// Check if the VM already exists.
	if inst, ok := m.vms[cfg.VMID]; ok {
		if inst.State == StateRunning {
			return inst, nil
		}
		// VM exists but is not running; clean up before relaunching.
		m.forceCleanup(cfg.VMID, "boot-replace-tracked")
	} else {
		// A live Firecracker process for this VM that the manager does not
		// track is a running computer (a reattach that failed after vmctl's
		// restart), not debris. Booting never kills it: explicit stop or the
		// wedge watchdog are the only paths that end one
		// (docs/vmctl-360-review-2026-10-10.md, Phase 0 step 3).
		if m.liveFirecrackerForVMLocked(cfg.VMID) {
			return nil, fmt.Errorf("%w: vm %s", ErrLiveUnmanaged, cfg.VMID)
		}
		// Only stale metadata remains: a dead PID file or a leftover tap.
		m.cleanupOrphanedFirecrackerLocked(cfg.VMID, "boot")
	}

	// Assign a host port/subnet for this VM. A vmctl restart can leave stale
	// linkdown tap devices from failed boots holding old /30 routes; reclaim
	// those before reusing a subnet or the guest can boot but never pass host
	// readiness probes.
	hostPort, err := m.allocateHostPortLocked(cfg.VMID)
	if err != nil {
		return nil, err
	}

	bootKind := cfg.BootKind
	if bootKind == "" {
		bootKind = "cold"
	}
	tl := newBootTimeline(cfg.VMID, bootKind)
	tl.mark("boot_begin")
	// Any cold boot moves the disks past a memory snapshot: discard it first,
	// so it can never be loaded over them (S3). A wake that tried the
	// snapshot and fell back names why.
	m.recordSnapshotFallbackOnBoot(cfg.VMID, tl)
	vmm := m.vmmFor(cfg)
	m.recordLaunchedVMM(cfg.VMID, vmm)
	tl.FirecrackerBin = vmm.Firecracker
	tl.FirecrackerVer = firecrackerBuildVersion(vmm.Firecracker)
	tl.HostKernelRelease = hostKernelRelease()
	tl.markDetail("host_port_allocated", fmt.Sprintf("port=%d", hostPort))

	// Ensure VM state directory exists before creating data image.
	vmStateDir := filepath.Join(m.cfg.StateDir, cfg.VMID)
	if err := os.MkdirAll(vmStateDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create VM state directory: %w", err)
	}

	// Ensure the persistent directory exists.
	if cfg.PersistentDir == "" {
		cfg.PersistentDir = filepath.Join(m.cfg.StateDir, cfg.VMID, "persist")
	}
	if err := os.MkdirAll(cfg.PersistentDir, 0o755); err != nil {
		return nil, fmt.Errorf("create persistent dir %s: %w", cfg.PersistentDir, err)
	}

	// Write the gateway token to the persistent directory if provided.
	// The guest init script reads this file and sets RUNTIME_GATEWAY_TOKEN.
	// This is a autoputer identity token (not a provider credential) so
	// VAL-VM-011 is not violated — the token only authenticates the
	// autoputer to the host-side gateway, which holds the actual provider
	// credentials (VAL-GATEWAY-004).
	if cfg.GatewayToken != "" {
		tokenPath := filepath.Join(cfg.PersistentDir, "gateway-token")
		if err := os.WriteFile(tokenPath, []byte(cfg.GatewayToken), 0o600); err != nil {
			log.Printf("vmmanager: warning: could not write gateway token for VM %s: %v", cfg.VMID, err)
		}
	}
	cfg.GatewayToken = m.resolveGatewayToken(cfg)

	// Consume the durable reservation without overwriting the epoch already
	// bound into the realization credential and guest identity.
	epoch, err := m.consumeBootEpoch(cfg.VMID, cfg.Epoch)
	if err != nil {
		return nil, err
	}
	cfg.Epoch = epoch
	tl.markDetail("boot_epoch", fmt.Sprintf("epoch=%d", cfg.Epoch))

	// With the D2 switch on, a cold boot launches from the current release.
	m.bindCurrentRelease(&cfg)
	// Apply defaults from manager config.
	m.applyLaunchDefaults(&cfg)

	// Prepare the per-VM mutable data image.
	if cfg.StoreDiskPath != "" {
		vmDataDir := filepath.Join(m.cfg.StateDir, cfg.VMID)
		if err := os.MkdirAll(vmDataDir, 0o755); err != nil {
			return nil, fmt.Errorf("create VM data dir %s: %w", vmDataDir, err)
		}
		dataImg := filepath.Join(vmDataDir, "data.img")
		if _, err := os.Stat(dataImg); os.IsNotExist(err) {
			if cfg.NewComputer && m.cfg.DataImageTemplate.Enabled {
				source, err := m.prepareNewDataImage(dataImg, dataImageSizeMB, templatePick)
				if err != nil {
					return nil, fmt.Errorf("create data image for VM %s: %w", cfg.VMID, err)
				}
				tl.markDetail("data_image_source", source)
				log.Printf("vmmanager: data image for new VM %s: %s", cfg.VMID, source)
			} else if err := m.createDataImage(dataImg, dataImageSizeMB); err != nil {
				return nil, fmt.Errorf("create data image for VM %s: %w", cfg.VMID, err)
			}
		} else if err == nil {
			if err := m.ensureDataImageMinSize(dataImg, dataImageSizeMB); err != nil {
				return nil, fmt.Errorf("resize data image for VM %s: %w", cfg.VMID, err)
			}
		} else {
			return nil, fmt.Errorf("stat data image for VM %s: %w", cfg.VMID, err)
		}
		// An image from before the mode change is 0644. Tighten it; a failure
		// is logged, not fatal: the state directory stays the barrier and a
		// boot is worth more than the mode on one file.
		if err := tightenDataImageMode(dataImg); err != nil {
			log.Printf("vmmanager: data image mode for VM %s not tightened: %v", cfg.VMID, err)
		}
	} else if cfg.RootfsPath != "" {
		// Legacy approach: create a per-VM writable copy of the rootfs.
		// The base rootfs image is read-only (from the Nix store or
		// /var/lib/go-choir/guest/), but Firecracker needs write access.
		// Each VM gets its own copy so state is isolated per-user (VAL-VM-005).
		vmRootfs := filepath.Join(m.cfg.StateDir, cfg.VMID, "rootfs.ext4")
		if err := m.copyFile(cfg.RootfsPath, vmRootfs); err != nil {
			return nil, fmt.Errorf("copy rootfs for VM %s: %w", cfg.VMID, err)
		}
		cfg.RootfsPath = vmRootfs
	}

	tl.mark("data_image_ready")
	if info, statErr := os.Stat(filepath.Join(vmStateDir, "data.img")); statErr == nil {
		tl.DataImagePath = filepath.Join(vmStateDir, "data.img")
		tl.DataImageBytes = info.Size()
		if st, ok := info.Sys().(*syscall.Stat_t); ok {
			// Blocks are 512-byte units; allocated bytes show reflink extent.
			tl.DataImageAlloc = st.Blocks * 512
		}
	}

	if cfg.ComputerCredentialEnvelope != "" || strings.TrimSpace(cfg.GatewayToken) != "" {
		// Claude delegate: an allowlisted computer's disk also carries the
		// grant, and the owner's token while the window is open.
		delegateFiles, delegateStatus := m.cfg.ClaudeDelegate.claudeDelegateMaterial(cfg.ComputerID, time.Now())
		if delegateStatus != "" {
			log.Printf("vmmanager: claude delegate for computer %s (VM %s): %s", cfg.ComputerID, cfg.VMID, delegateStatus)
		}
		// Visual capture: a granted computer's disk carries the launch
		// grant (docs/design/visual-evidence-slice-2026-10-15.md).
		captureFiles, captureStatus := m.cfg.VisualCapture.visualCaptureMaterial(cfg.ComputerID, cfg.WarmnessClass, time.Now())
		if captureStatus != "" {
			log.Printf("vmmanager: visual capture for computer %s (VM %s): %s", cfg.ComputerID, cfg.VMID, captureStatus)
		}
		for name, content := range captureFiles {
			if delegateFiles == nil {
				delegateFiles = map[string][]byte{}
			}
			delegateFiles[name] = content
		}
		// Capsule package mirrors: a granted computer's disk carries the
		// ecosystems its L2 capsules use the host mirror for (station FP10).
		mirrorFiles, mirrorStatus := m.cfg.CapsuleMirrors.capsuleMirrorMaterial(cfg.ComputerID, cfg.WarmnessClass, time.Now())
		if mirrorStatus != "" {
			log.Printf("vmmanager: capsule mirrors for computer %s (VM %s): %s", cfg.ComputerID, cfg.VMID, mirrorStatus)
		}
		for name, content := range mirrorFiles {
			if delegateFiles == nil {
				delegateFiles = map[string][]byte{}
			}
			delegateFiles[name] = content
		}
		// Host fetch route (SDC NET-01): a granted computer's disk carries
		// the grant; its guest then fetches through the gateway.
		fetchFiles, fetchStatus := m.cfg.HostFetch.hostFetchMaterial(cfg.ComputerID, cfg.VMID, time.Now())
		if fetchStatus != "" {
			log.Printf("vmmanager: host fetch route for computer %s (VM %s): %s", cfg.ComputerID, cfg.VMID, fetchStatus)
		}
		for name, content := range fetchFiles {
			if delegateFiles == nil {
				delegateFiles = map[string][]byte{}
			}
			delegateFiles[name] = content
		}
		// Event table (F-c deploy 2): a named computer's disk carries the
		// grant; its writer then stamps telemetry for choir_events when the
		// owner's store.event_table is on.
		eventTableFiles, eventTableStatus := m.cfg.EventTable.eventTableMaterial(cfg.ComputerID, cfg.WarmnessClass, time.Now())
		if eventTableStatus != "" {
			log.Printf("vmmanager: event table for computer %s (VM %s): %s", cfg.ComputerID, cfg.VMID, eventTableStatus)
		}
		for name, content := range eventTableFiles {
			if delegateFiles == nil {
				delegateFiles = map[string][]byte{}
			}
			delegateFiles[name] = content
		}
		credentialDiskPath, err := m.createCredentialDisk(vmStateDir, cfg.ComputerCredentialEnvelope, cfg.GatewayToken, delegateFiles)
		if err != nil {
			return nil, fmt.Errorf("prepare appender credential disk for VM %s: %w", cfg.VMID, err)
		}
		cfg.credentialDiskPath = credentialDiskPath
		cfg.ComputerCredentialEnvelope = ""
	}
	tl.mark("credential_disk_ready")

	// Build the Firecracker configuration.
	// Provider credentials are explicitly NOT included here (VAL-VM-011).
	fcConfig := m.buildFirecrackerConfig(cfg, hostPort)

	guestIP, _ := m.guestAndHostIP(hostPort)
	hostURL := fmt.Sprintf("http://%s:%d", guestIP, cfg.GuestPort)

	// NewComputer is a boot input for this call only: the tracked config is
	// what a later recover or refresh boots from, and those never take the
	// template (a lost data.img keeps today's path).
	trackedCfg := cfg
	trackedCfg.NewComputer = false
	inst := &VMInstance{
		Config:    trackedCfg,
		State:     StatePending,
		HostURL:   hostURL,
		StartedAt: time.Now(),
		done:      make(chan struct{}),
	}
	inst.coldOrigin = true // launched from a config, not a snapshot load

	inst.BootTimeline = tl
	tl.ComputerID = cfg.ComputerID
	tl.Epoch = cfg.Epoch
	tl.KernelImagePath = cfg.KernelImagePath
	tl.InitrdPath = cfg.InitrdPath
	tl.StoreDiskPath = cfg.StoreDiskPath
	tl.MemSizeMiB = cfg.MachineMemSizeMib
	tl.VCPUCount = cfg.MachineCPUCount
	tl.GuestIP = guestIP
	tl.HostURL = hostURL
	tl.mark("firecracker_launch_begin")

	m.vms[cfg.VMID] = inst

	// Launch Firecracker.
	if err := m.launchFirecracker(cfg, fcConfig, hostPort); err != nil {
		inst.State = StateFailed
		return nil, fmt.Errorf("launch firecracker for VM %s: %w", cfg.VMID, err)
	}
	tl.mark("firecracker_spawned")

	// Do not hold the global manager lock while waiting for guest readiness.
	// Fresh boots should be allowed to overlap; only the shared state mutation
	// needs serialization.
	m.mu.Unlock()
	locked = false
	// A Firecracker child that exits before readiness (crash or typed boot
	// refusal followed by exit) must fail the boot promptly.
	alive := func() bool {
		m.mu.Lock()
		pid := inst.PID
		m.mu.Unlock()
		if pid <= 0 {
			return true
		}
		return processExists(pid)
	}
	if err := m.waitForGuestReady(hostURL, tl, alive); err != nil {
		m.mu.Lock()
		locked = true
		inst.State = StateFailed
		inst.Healthy = false
		m.killFirecrackerProcess(inst, "boot-readiness-failed")
		waitErr := fmt.Errorf("wait for guest ready for VM %s: %w", cfg.VMID, err)
		recordGuestRefusalOnTimeline(tl, err)
		tl.finish("failed", waitErr)
		if perr := tl.persist(m.cfg.StateDir); perr != nil {
			log.Printf("vmmanager: warning: could not persist boot timeline for VM %s: %v", cfg.VMID, perr)
		}
		return nil, waitErr
	}
	m.mu.Lock()
	locked = true

	inst.State = StateRunning
	inst.Healthy = true
	inst.LastHealthCheck = time.Now()
	inst.LastHealthyAt = inst.LastHealthCheck
	tl.firstHealthy()
	m.mu.Unlock()
	locked = false

	// The guest-side receipt is fetched once the guest is healthy: the merged
	// record carries both halves of the boot in one file. The fetch is
	// observation, not shared-state mutation — like waitForGuestReady it must
	// not run under m.mu, where a slow runtime_started would serialize every
	// boot (docs/problems/s0-fetch-guest-timeline-deadlock-2026-10-01.md).
	tl.fetchGuestBootTimeline(hostURL)
	tl.finish("healthy", nil)
	if perr := tl.persist(m.cfg.StateDir); perr != nil {
		log.Printf("vmmanager: warning: could not persist boot timeline for VM %s: %v", cfg.VMID, perr)
	}

	log.Printf("vmmanager: booted VM %s (host=%s epoch=%d)", cfg.VMID, hostURL, cfg.Epoch)

	return inst, nil
}

// applyLaunchDefaults fills empty launch fields from the manager config. A
// snapshot's expected launch config is rebuilt with the same rules.
func (m *Manager) applyLaunchDefaults(cfg *VMConfig) {
	if cfg.KernelImagePath == "" {
		cfg.KernelImagePath = m.cfg.KernelImagePath
	}
	if cfg.InitrdPath == "" {
		cfg.InitrdPath = m.cfg.InitrdPath
	}
	if cfg.RootfsPath == "" {
		cfg.RootfsPath = m.cfg.RootfsPath
	}
	if cfg.StoreDiskPath == "" {
		cfg.StoreDiskPath = m.cfg.StoreDiskPath
	}
	if cfg.KernelParams == "" {
		cfg.KernelParams = m.kernelParams()
	}
	if cfg.GuestPort == 0 {
		cfg.GuestPort = m.cfg.GuestPort
	}
	if cfg.MachineCPUCount == 0 {
		cfg.MachineCPUCount = m.cfg.MachineCPUCount
	}
	if cfg.MachineMemSizeMib == 0 {
		cfg.MachineMemSizeMib = m.cfg.MachineMemSizeMib
	}
}

// StopVM cleanly stops a running VM.
func (m *Manager) StopVM(vmID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	inst, ok := m.vms[vmID]
	if !ok {
		return fmt.Errorf("vm %s not found", vmID)
	}

	if inst.State != StateRunning && inst.State != StateFailed {
		return nil // already stopped or hibernated
	}

	m.killFirecrackerProcess(inst, "stop")
	inst.State = StateStopped
	inst.Healthy = false

	log.Printf("vmmanager: stopped VM %s", vmID)
	return nil
}

// HibernateVM saves the VM state and stops it. The VM can be resumed
// later, restoring the same user state from the persistent directory
// (VAL-CROSS-116).
//
// In snapshot mode (VM_HIBERNATE_MODE=snapshot) the guest is fenced and its
// memory is captured as a Firecracker snapshot that a later wake loads once
// (snapshot_lifecycle.go). Every failure there, and kill mode, take the path
// below: Firecracker is killed and the next wake cold boots from the
// persistent directory, which is a crash boot under the restart rule.
func (m *Manager) HibernateVM(vmID string) error {
	return m.HibernateVMWithOptions(vmID, HibernateOptions{})
}

// HibernateVMWithOptions is HibernateVM with per-call options (density D6
// S3). The options never reach the VM's configuration or its launch: they
// only add a step to a snapshot hibernate, and kill mode ignores them.
func (m *Manager) HibernateVMWithOptions(vmID string, opts HibernateOptions) error {
	if m.hibernateMode() == HibernateModeSnapshot {
		return m.hibernateVMSnapshot(vmID, opts)
	}
	return m.hibernateVMKill(vmID)
}

func (m *Manager) hibernateVMKill(vmID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	inst, ok := m.vms[vmID]
	if !ok {
		return fmt.Errorf("vm %s not found", vmID)
	}

	if inst.State != StateRunning {
		return fmt.Errorf("vm %s is not running (state=%s)", vmID, inst.State)
	}

	m.killFirecrackerProcess(inst, "hibernate")
	inst.State = StateHibernated
	inst.Healthy = false

	log.Printf("vmmanager: hibernated VM %s (persistent data preserved at %s)", vmID, inst.Config.PersistentDir)
	return nil
}

// ResumeVM resumes a stopped or hibernated VM, restoring the user's
// persisted state from the VM's persistent directory (VAL-CROSS-116).
// The epoch counter is NOT incremented on resume, so callers can
// distinguish fresh boot from resume (VAL-CROSS-117).
func (m *Manager) ResumeVM(vmID string) (*VMInstance, error) {
	m.mu.Lock()
	locked := true
	defer func() {
		if locked {
			m.mu.Unlock()
		}
	}()

	inst, ok := m.vms[vmID]
	if !ok {
		return nil, fmt.Errorf("vm %s not found", vmID)
	}

	if inst.State != StateStopped && inst.State != StateHibernated {
		if inst.State == StateRunning {
			return inst, nil
		}
		return nil, fmt.Errorf("vm %s cannot be resumed (state=%s)", vmID, inst.State)
	}

	// A resume is a cold boot from the stored config: with the D2 switch on it
	// launches from the current release, not the one the config last named.
	m.refreshLaunchArtifacts(&inst.Config, VMConfig{})

	// Clean up the old process.
	m.forceCleanup(vmID, "resume")
	// This path cold boots: a memory snapshot can no longer be loaded (S3).
	m.recordSnapshotFallbackOnBoot(vmID, nil)

	// Assign a new host port/subnet (old one may be reused if it is actually
	// free, but stale linkdown taps from failed boots must not capture routing).
	hostPort, err := m.allocateHostPortLocked(vmID)
	if err != nil {
		return nil, err
	}

	// The resume path duplicates bootVM's launch sequence; record the same
	// receipt under boot_kind=resume so the S3 wake timeline lands on the
	// same observer as cold boots.
	tl := newBootTimeline(vmID, "resume")
	tl.mark("boot_begin")
	vmm := m.vmmFor(inst.Config)
	m.recordLaunchedVMM(vmID, vmm)
	tl.FirecrackerBin = vmm.Firecracker
	tl.FirecrackerVer = firecrackerBuildVersion(vmm.Firecracker)
	tl.HostKernelRelease = hostKernelRelease()
	tl.markDetail("host_port_allocated", fmt.Sprintf("port=%d", hostPort))
	tl.ComputerID = inst.Config.ComputerID
	tl.Epoch = inst.Config.Epoch
	tl.KernelImagePath = inst.Config.KernelImagePath
	tl.InitrdPath = inst.Config.InitrdPath
	tl.StoreDiskPath = inst.Config.StoreDiskPath
	tl.MemSizeMiB = inst.Config.MachineMemSizeMib
	tl.VCPUCount = inst.Config.MachineCPUCount
	guestIP, _ := m.guestAndHostIP(hostPort)
	hostURL := fmt.Sprintf("http://%s:%d", guestIP, inst.Config.GuestPort)
	inst.HostURL = hostURL
	tl.GuestIP = guestIP
	tl.HostURL = hostURL
	if info, statErr := os.Stat(filepath.Join(m.cfg.StateDir, vmID, "data.img")); statErr == nil {
		tl.DataImagePath = filepath.Join(m.cfg.StateDir, vmID, "data.img")
		tl.DataImageBytes = info.Size()
		if st, ok := info.Sys().(*syscall.Stat_t); ok {
			tl.DataImageAlloc = st.Blocks * 512
		}
	}
	inst.BootTimeline = tl
	tl.mark("data_image_ready")

	// Reuse the existing epoch (no increment on resume).
	// This preserves the VM identity across stop/resume so callers
	// can detect duplicate canonical effects (VAL-CROSS-117).
	epoch := inst.Config.Epoch
	inst.Config.GatewayToken = m.resolveGatewayToken(inst.Config)

	fcConfig := m.buildFirecrackerConfig(inst.Config, hostPort)

	if err := m.launchFirecracker(inst.Config, fcConfig, hostPort); err != nil {
		inst.State = StateFailed
		return nil, fmt.Errorf("resume firecracker for VM %s: %w", vmID, err)
	}
	// A launch from config, not a snapshot load: cold origin.
	inst.coldOrigin = true
	tl.mark("firecracker_spawned")

	m.mu.Unlock()
	locked = false
	alive := func() bool {
		m.mu.Lock()
		pid := inst.PID
		m.mu.Unlock()
		if pid <= 0 {
			return true
		}
		return processExists(pid)
	}
	if err := m.waitForGuestReady(hostURL, tl, alive); err != nil {
		m.mu.Lock()
		locked = true
		inst.State = StateFailed
		inst.Healthy = false
		m.killFirecrackerProcess(inst, "resume-readiness-failed")
		waitErr := fmt.Errorf("wait for resumed guest ready for VM %s: %w", vmID, err)
		recordGuestRefusalOnTimeline(tl, err)
		tl.finish("failed", waitErr)
		if perr := tl.persist(m.cfg.StateDir); perr != nil {
			log.Printf("vmmanager: warning: could not persist boot timeline for VM %s: %v", vmID, perr)
		}
		return nil, waitErr
	}
	m.mu.Lock()
	locked = true

	inst.State = StateRunning
	inst.Healthy = true
	inst.StartedAt = time.Now()
	inst.LastHealthCheck = time.Now()
	inst.LastHealthyAt = inst.LastHealthCheck
	tl.firstHealthy()
	m.mu.Unlock()
	locked = false

	// Same contract as bootVM: the guest-receipt fetch and final persist are
	// observation work and must not hold m.mu
	// (docs/problems/s0-fetch-guest-timeline-deadlock-2026-10-01.md).
	tl.fetchGuestBootTimeline(hostURL)
	tl.finish("healthy", nil)
	if perr := tl.persist(m.cfg.StateDir); perr != nil {
		log.Printf("vmmanager: warning: could not persist boot timeline for VM %s: %v", vmID, perr)
	}

	log.Printf("vmmanager: resumed VM %s (host=%s epoch=%d)", vmID, hostURL, epoch)
	return inst, nil
}

// BootTimelineForVM returns the in-flight or persisted boot receipt for a VM.
// The in-flight record wins while a boot is running; the persisted file is
// the fallback after a vmctl restart reattaches without a live record.
func (m *Manager) BootTimelineForVM(vmID string) (*BootTimeline, error) {
	m.mu.RLock()
	inst, ok := m.vms[vmID]
	var live *BootTimeline
	if ok {
		live = inst.BootTimeline
	}
	m.mu.RUnlock()
	if live != nil {
		return live, nil
	}
	return readBootTimeline(m.cfg.StateDir, vmID)
}

// ReattachVM attaches a new manager process to a Firecracker process that
// survived vmctl restart. It trusts the durable routing metadata only after
// both the saved PID exists and the guest health endpoint responds.
func (m *Manager) ReattachVM(vmID, hostURL string, epoch int64) (*VMInstance, error) {
	return m.ReattachVMWithConfig(vmID, hostURL, epoch, VMConfig{})
}

func (m *Manager) ReattachVMWithConfig(vmID, hostURL string, epoch int64, overrides VMConfig) (*VMInstance, error) {
	if strings.TrimSpace(vmID) == "" {
		return nil, fmt.Errorf("vm_id is required")
	}
	if strings.TrimSpace(hostURL) == "" {
		return nil, fmt.Errorf("host_url is required")
	}
	pid, err := m.loadPID(vmID)
	if err != nil {
		return nil, err
	}
	if !processExists(pid) {
		return nil, fmt.Errorf("firecracker pid %d for VM %s is not running", pid, vmID)
	}
	// Adopt by identity (vmctl 360 review Phase 0 step 5): a live process
	// whose cmdline names this VM is this computer, answering or not. A busy
	// guest that misses the health probe is adopted unready, never left
	// outside the registry; routing waits for its health, and the wedge
	// watchdog ends it only on evidence.
	if !firecrackerCmdlineMatchesVM(pid, vmID) {
		return nil, fmt.Errorf("pid %d does not identify VM %s; refusing to adopt", pid, vmID)
	}
	healthy := m.probeGuestHealth(hostURL)
	if !healthy {
		log.Printf("vmmanager: adopting VM %s (pid=%d) unready: alive, health probe unanswered", vmID, pid)
	}

	m.mu.Lock()
	if inst, ok := m.vms[vmID]; ok {
		if inst.State == StateRunning {
			m.mu.Unlock()
			return inst, nil
		}
	}
	if epoch <= 0 {
		if loaded, err := m.loadEpoch(vmID); err == nil {
			epoch = loaded
		}
	}
	cfg := m.reattachConfig(vmID, epoch, overrides)
	cfg.VMID = vmID
	cfg.Epoch = epoch
	cfg.GatewayToken = m.resolveGatewayToken(cfg)
	now := time.Now()
	inst := &VMInstance{
		Config:          cfg,
		State:           StateRunning,
		HostURL:         hostURL,
		PID:             pid,
		StartedAt:       now,
		LastHealthCheck: now,
		Healthy:         healthy,
	}
	if healthy {
		inst.LastHealthyAt = now
	}
	inst.confinement = m.loadConfinement(vmID)
	m.vms[vmID] = inst
	m.reserveHostURLLocked(hostURL)
	m.mu.Unlock()

	m.reconcileTapIsolation(vmID, hostURL)
	m.ensureGuestEgressPolicy()
	m.adoptConsole(vmID, pid, inst.confinement.jailed())
	log.Printf("vmmanager: reattached VM %s (host=%s pid=%d epoch=%d)", vmID, hostURL, pid, epoch)
	return inst, nil
}

// GetVM returns the VM instance for the given ID, or nil if not found.
func (m *Manager) GetVM(vmID string) *VMInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.vms[vmID]
}

// ListVMs returns all managed VM instances.
func (m *Manager) ListVMs() []*VMInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*VMInstance, 0, len(m.vms))
	for _, inst := range m.vms {
		result = append(result, inst)
	}
	return result
}

// RemoveVM removes a VM from management. The VM must be stopped first.
// The persistent directory is preserved so the user's data survives
// across stop/remove/recreate cycles (VAL-CROSS-116).
func (m *Manager) RemoveVM(vmID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	inst, ok := m.vms[vmID]
	if !ok {
		return nil // idempotent
	}

	if inst.State == StateRunning {
		return fmt.Errorf("vm %s is still running; stop it first", vmID)
	}

	delete(m.vms, vmID)
	log.Printf("vmmanager: removed VM %s from management (persistent data preserved)", vmID)
	return nil
}

// DestroyVMState removes a stopped VM from management and deletes its state
// directory. This is intentionally stronger than RemoveVM and is used only by
// vmctl's stale worker/candidate cleanup policy after primary/published/active
// computers have already been excluded.
func (m *Manager) DestroyVMState(vmID string) error {
	vmID = strings.TrimSpace(vmID)
	if vmID == "" || strings.Contains(vmID, string(os.PathSeparator)) || vmID == "." || vmID == ".." {
		return fmt.Errorf("invalid VM ID %q", vmID)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if inst, ok := m.vms[vmID]; ok {
		if inst.State == StateRunning || inst.State == StatePending {
			return fmt.Errorf("vm %s is still running; refuse to destroy state", vmID)
		}
		if inst.PID > 0 && processExists(inst.PID) {
			return fmt.Errorf("vm %s has live process %d; refuse to destroy state", vmID, inst.PID)
		}
		delete(m.vms, vmID)
	} else {
		if m.liveFirecrackerForVMLocked(vmID) {
			return fmt.Errorf("%w: refuse to destroy state of vm %s", ErrLiveUnmanaged, vmID)
		}
		m.cleanupOrphanedFirecrackerLocked(vmID, "destroy-state")
	}
	if pids := firecrackerPIDsForVM(vmID); len(pids) > 0 {
		return fmt.Errorf("vm %s still has Firecracker process(es); refuse to destroy state", vmID)
	}
	if pid, err := m.loadPID(vmID); err == nil {
		if processExists(pid) && firecrackerCmdlineMatchesVM(pid, vmID) {
			return fmt.Errorf("vm %s still has Firecracker pid %d; refuse to destroy state", vmID, pid)
		}
		_ = os.Remove(m.pidPath(vmID))
	}

	if len(vmID) >= 8 {
		m.deleteTapDevice(tapNameForVMID(vmID))
	}
	if validJailerID(vmID) {
		if err := os.RemoveAll(m.jailDir(vmID)); err != nil {
			return fmt.Errorf("destroy jail of VM %s: %w", vmID, err)
		}
		m.removeCgroupLeaf(vmID)
	}
	stateRoot := filepath.Clean(m.cfg.StateDir)
	stateDir := filepath.Clean(filepath.Join(stateRoot, vmID))
	if stateDir == stateRoot || !strings.HasPrefix(stateDir, stateRoot+string(os.PathSeparator)) {
		return fmt.Errorf("refuse to delete VM state outside state dir: %s", stateDir)
	}
	if err := os.RemoveAll(stateDir); err != nil {
		return fmt.Errorf("destroy VM state %s: %w", vmID, err)
	}
	m.releaseSnapshotPort(vmID)
	log.Printf("vmmanager: destroyed stale VM state %s (%s)", vmID, stateDir)
	return nil
}

// CheckHealth probes the VM's guest health endpoint.
func (m *Manager) CheckHealth(vmID string) (bool, error) {
	m.mu.RLock()
	inst, ok := m.vms[vmID]
	m.mu.RUnlock()

	if !ok {
		return false, fmt.Errorf("vm %s not found", vmID)
	}

	if inst.State != StateRunning {
		return false, nil
	}

	// Use HTTP client to probe the guest's /health endpoint.
	healthy := m.probeGuestHealth(inst.HostURL)

	m.mu.Lock()
	inst.Healthy = healthy
	inst.LastHealthCheck = time.Now()
	if healthy {
		inst.LastHealthyAt = inst.LastHealthCheck
	}
	if !healthy {
		log.Printf("vmmanager: health check failed for VM %s at %s", vmID, inst.HostURL)
	}
	m.mu.Unlock()

	return healthy, nil
}

// releaseExitedVMNetwork removes the tap and its firewall rules after a
// Firecracker exit that vmctl did not cause (crash, guest poweroff). It acts
// only while the exited process is still the instance's current one: a vmctl
// kill clears inst.cmd and tears down itself, and a relaunch or replacement
// instance owns the tap now.
func (m *Manager) releaseExitedVMNetwork(vmID string, expected *VMInstance, cmd *exec.Cmd) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.vms[vmID]
	if !ok || current != expected || current.cmd != cmd || cmd == nil {
		return
	}
	log.Printf("vmmanager: Firecracker for VM %s exited; removing its tap network", vmID)
	m.deleteTapDevice(tapNameForVMID(vmID))
}

func (m *Manager) markInstanceFailed(vmID string, expected *VMInstance) {
	m.mu.Lock()
	defer m.mu.Unlock()

	current, ok := m.vms[vmID]
	if !ok || current != expected {
		log.Printf("vmmanager: ignored stale failure for VM %s", vmID)
		return
	}
	current.State = StateFailed
	current.Healthy = false
	log.Printf("vmmanager: marked VM %s as failed", vmID)
}

// RecoverVM attempts to recover a failed VM by force-killing and
// rebooting it. The persistent data is preserved so the user's state
// survives recovery (VAL-VM-009, VAL-CROSS-116).
//
// The epoch IS incremented on recovery because this is a fresh boot,
// not a resume. This prevents duplicate canonical effects (VAL-CROSS-117).
func (m *Manager) RecoverVM(vmID string) (*VMInstance, error) {
	return m.RecoverVMWithConfig(vmID, VMConfig{})
}

func (m *Manager) RecoverVMWithConfig(vmID string, overrides VMConfig) (*VMInstance, error) {
	unlock := m.lockVMOperation(vmID)
	defer unlock()

	m.mu.Lock()
	inst, ok := m.vms[vmID]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("vm %s not found", vmID)
	}

	// Force kill the old process.
	m.killFirecrackerProcess(inst, "recover")
	inst.State = StateFailed
	m.mu.Unlock()

	m.mu.Lock()
	cfg := m.recoverConfig(inst.Config, overrides, vmID)
	m.mu.Unlock()

	// Boot with the same config but new epoch.
	return m.bootVM(cfg)
}

// recoverConfig is the config a recovery boots: the stored one with the
// caller's overrides and a new epoch. The stored artifacts and kernel
// parameters date from the original launch; a recovery boots the current image
// (H1, review 318 DeepSeek 2).
func (m *Manager) recoverConfig(stored, overrides VMConfig, vmID string) VMConfig {
	cfg := mergeVMConfigOverrides(stored, overrides)
	if overrides.Epoch == 0 {
		cfg.Epoch = 0
	}
	cfg.VMID = vmID
	m.refreshLaunchArtifacts(&cfg, overrides)
	return cfg
}

// RefreshVM force-kills and reboots a VM onto the current manager boot
// artifacts while preserving the mutable data image. Deploy-time refresh is not
// crash recovery: active VMs must pick up the current guest/store/runtime
// closure instead of replaying stale boot paths captured when the VM was first
// created.
func (m *Manager) RefreshVM(vmID string) (*VMInstance, error) {
	return m.RefreshVMWithConfig(vmID, VMConfig{})
}

func (m *Manager) RefreshVMWithConfig(vmID string, overrides VMConfig) (*VMInstance, error) {
	unlock := m.lockVMOperation(vmID)
	defer unlock()

	m.mu.Lock()
	inst, ok := m.vms[vmID]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("vm %s not found", vmID)
	}

	m.killFirecrackerProcess(inst, "refresh")
	inst.State = StateFailed
	cfg := refreshBootConfig(inst.Config, overrides, m.cfg)
	cfg.OfflineMaintenance = nil
	if overrides.BootKind == "" {
		cfg.BootKind = "refresh"
	}
	if overrides.Epoch == 0 {
		cfg.Epoch = 0
	}
	cfg.VMID = vmID
	m.mu.Unlock()

	if err := m.runOfflineMaintenance(vmID, &cfg, overrides); err != nil {
		return nil, err
	}
	return m.bootVM(cfg)
}

// runOfflineMaintenance runs a refresh's OfflineMaintenance hook, then
// re-mints the boot envelope into cfg for any outcome that boots. It returns
// an error only when the hook left no bootable image. A failed re-mint keeps
// the envelope minted before the kill: a short hook still boots on it.
func (m *Manager) runOfflineMaintenance(vmID string, cfg *VMConfig, overrides VMConfig) error {
	if overrides.OfflineMaintenance == nil {
		return nil
	}
	dataImage := filepath.Join(m.cfg.StateDir, vmID, "data.img")
	started := time.Now()
	if err := overrides.OfflineMaintenance(dataImage); err != nil {
		if errors.Is(err, ErrOfflineMaintenanceUnbootable) {
			return fmt.Errorf("vm %s not booted after offline maintenance: %w", vmID, err)
		}
		log.Printf("vmmanager: offline maintenance for %s failed (booting the image it left): %v", vmID, err)
	}
	if overrides.PostMaintenanceCredential == nil {
		return nil
	}
	envelope, err := overrides.PostMaintenanceCredential()
	if err != nil || envelope == "" {
		log.Printf("vmmanager: boot credential re-mint for %s (%s) after offline maintenance failed; booting the envelope minted before the hook (hook took %s): %v", vmID, cfg.RealizationID, time.Since(started).Round(time.Second), err)
		return nil
	}
	cfg.ComputerCredentialEnvelope = envelope
	log.Printf("vmmanager: boot credential for %s (%s) re-minted after offline maintenance (hook took %s)", vmID, cfg.RealizationID, time.Since(started).Round(time.Second))
	return nil
}

func mergeVMConfigOverrides(cfg VMConfig, overrides VMConfig) VMConfig {
	if overrides.VMID != "" {
		cfg.VMID = overrides.VMID
	}
	if overrides.ComputerID != "" {
		cfg.ComputerID = overrides.ComputerID
	}
	if overrides.RealizationID != "" {
		cfg.RealizationID = overrides.RealizationID
	}
	if overrides.Epoch != 0 {
		cfg.Epoch = overrides.Epoch
	}
	if overrides.KernelImagePath != "" {
		cfg.KernelImagePath = overrides.KernelImagePath
	}
	if overrides.InitrdPath != "" {
		cfg.InitrdPath = overrides.InitrdPath
	}
	if overrides.RootfsPath != "" {
		cfg.RootfsPath = overrides.RootfsPath
	}
	if overrides.StoreDiskPath != "" {
		cfg.StoreDiskPath = overrides.StoreDiskPath
	}
	if overrides.KernelParams != "" {
		cfg.KernelParams = overrides.KernelParams
	}
	if overrides.GuestPort != 0 {
		cfg.GuestPort = overrides.GuestPort
	}
	if overrides.MachineCPUCount != 0 {
		cfg.MachineCPUCount = overrides.MachineCPUCount
	}
	if overrides.MachineMemSizeMib != 0 {
		cfg.MachineMemSizeMib = overrides.MachineMemSizeMib
	}
	// The hotplug selection is authoritative per call, like the boot-mode
	// flags below: vmctl decides it for every boot, and 0 must be able to
	// clear a stale selection.
	cfg.MemoryHotplugMiB = overrides.MemoryHotplugMiB
	if overrides.PersistentDir != "" {
		cfg.PersistentDir = overrides.PersistentDir
	}
	if overrides.GatewayToken != "" {
		cfg.GatewayToken = overrides.GatewayToken
	}
	if overrides.ComputerCredentialEnvelope != "" {
		cfg.ComputerCredentialEnvelope = overrides.ComputerCredentialEnvelope
	}
	if overrides.ComputerKind != "" {
		cfg.ComputerKind = overrides.ComputerKind
	}
	if overrides.OwnerID != "" {
		cfg.OwnerID = overrides.OwnerID
	}
	if overrides.DesktopID != "" {
		cfg.DesktopID = overrides.DesktopID
	}
	// Boot-mode flags are authoritative per call: a plain bool merge cannot
	// express "clear", so a stale MaintenanceHold/RecoveryReplayOnly retained
	// in the instance config would fence every future recover/refresh boot
	// after an unhold (2026-09-03: two successful unholds still booted
	// RUNTIME_MAINTENANCE_HOLD=1). Every caller sets these explicitly.
	cfg.MaintenanceHold = overrides.MaintenanceHold
	cfg.RecoveryReplayOnly = overrides.RecoveryReplayOnly
	// NewComputer is the same: only the mint's own call names it, and a
	// stored config must never carry it into a recover or refresh.
	cfg.NewComputer = overrides.NewComputer
	// BootKind merges only when the caller names it: an empty override must
	// not erase a label the adapter already set (recover/refresh).
	if overrides.BootKind != "" {
		cfg.BootKind = overrides.BootKind
	}
	if overrides.WarmnessClass != "" {
		cfg.WarmnessClass = overrides.WarmnessClass
	}
	return cfg
}

// refreshBootConfig resets the stored config to the current deploy first, then
// applies the caller's overrides. The order matters: resetting after the merge
// erased the machine shape vmctl asked for, so every refresh booted an
// interactive computer at the manager default (docs/problems/vm-refresh-drops-
// interactive-machine-shape-2026-10-09.md).
func refreshBootConfig(stored, overrides VMConfig, defaults ManagerConfig) VMConfig {
	return mergeVMConfigOverrides(refreshConfigForCurrentDeploy(stored, defaults), overrides)
}

func refreshConfigForCurrentDeploy(cfg VMConfig, defaults ManagerConfig) VMConfig {
	// A refresh or a wake rebuild never carries a release forward: the
	// current deploy (or the snapshot's own release) is chosen afresh.
	cfg.GuestRelease = ""
	if defaults.StoreDiskPath == "" {
		return cfg
	}

	cfg.KernelImagePath = ""
	cfg.InitrdPath = ""
	cfg.RootfsPath = ""
	cfg.StoreDiskPath = ""
	cfg.KernelParams = ""
	cfg.MachineMemSizeMib = 0
	cfg.MachineCPUCount = 0
	return cfg
}

// buildFirecrackerConfig generates the JSON configuration for a
// Firecracker VM launch. This config is written to a temp file and
// passed to the Firecracker binary via --config-file.
//
// IMPORTANT: Provider credentials are explicitly NOT included in
// this configuration. The guest environment, boot_args, and drives
// contain only the minimum bootstrap material (VAL-VM-011).
func (m *Manager) buildFirecrackerConfig(cfg VMConfig, hostPort int) map[string]interface{} {
	cfg.GatewayToken = m.resolveGatewayToken(cfg)
	hotplug := m.firecrackerMemoryHotplugConfig(cfg)

	// Calculate the /30 subnet for this VM based on host port. The ordinal
	// wraps through a bounded private 10.200.0.0-10.215.255.255 pool so
	// long-lived staging allocators cannot generate invalid IPv4 octets.
	guestIP, hostIP := m.guestAndHostIP(hostPort)

	// Build drives list.
	// With the upstream microvm.nix approach:
	//   - Shared nix-store disk (erofs) as a read-only virtio-blk drive
	//   - Per-VM data volume for mutable autoputer state
	// With legacy rootfs (old approach):
	//   - Rootfs ext4 as writable virtio-blk at /dev/vda
	var drives []map[string]interface{}

	if cfg.StoreDiskPath != "" {
		// Shared nix-store disk mounted read-only by the guest init.
		drives = append(drives, map[string]interface{}{
			"drive_id":       "store",
			"path_on_host":   cfg.StoreDiskPath,
			"is_root_device": false,
			"is_read_only":   true,
		})
	} else if cfg.RootfsPath != "" {
		// Legacy approach: ext4 rootfs as the writable root device.
		// This is the old custom init script approach with init=/bin/init.
		drives = append(drives, map[string]interface{}{
			"drive_id":       "rootfs",
			"path_on_host":   cfg.RootfsPath,
			"is_root_device": true,
			"is_read_only":   false,
		})
	}

	// Per-VM data volume for mutable autoputer state (always present).
	// vmmanager creates a data.img per-VM in the state directory.
	dataImgPath := filepath.Join(m.cfg.StateDir, cfg.VMID, "data.img")
	dataDrive := map[string]interface{}{
		"drive_id":       "data",
		"path_on_host":   dataImgPath,
		"is_root_device": false,
		"is_read_only":   false,
	}
	applyDataDriveDiscard(m.dataDiscardFor(cfg), dataDrive)
	drives = append(drives, dataDrive)
	if cfg.credentialDiskPath != "" {
		drives = append(drives, map[string]interface{}{
			"drive_id":       "credential",
			"path_on_host":   cfg.credentialDiskPath,
			"is_root_device": false,
			"is_read_only":   false,
		})
	}

	// Build the guest kernel boot arguments.
	// These contain NO provider credentials (VAL-VM-011).
	// Network config is passed via ip= kernel parameter so the guest
	// can configure its network interface at boot time.
	//
	// The autoputer gateway token is a per-VM autoputer identity credential, not a
	// provider secret. It rides the root-only credential disk (gateway-token,
	// mode 0400) instead of kernel argv so guest children cannot read it from
	// world-readable /proc/cmdline (S1 security-floor remainder).
	//
	// With the upstream microvm.nix approach:
	//   - start from the microvm-provided kernel params (`root=fstab`,
	//     `/nix/store/.../init`, `regInfo=...`)
	//   - append per-VM runtime parameters used by go-choir
	//
	// With legacy rootfs:
	//   - init=/bin/init tells the kernel to run our custom init script
	//   - root=/dev/vda points to the ext4 root drive
	var bootArgs string
	kernelParams := strings.TrimSpace(cfg.KernelParams)
	if kernelParams == "" {
		kernelParams = strings.TrimSpace(m.kernelParams())
	}
	if cfg.StoreDiskPath != "" && kernelParams != "" {
		runtimeArgs := []string{
			"console=ttyS0,115200",
			"reboot=k",
			"panic=1",
			"i8042.noaux",
			"i8042.nomux",
			"i8042.nopnp",
			"i8042.dumbkbd",
			fmt.Sprintf("guest_port=%d", cfg.GuestPort),
			"persistent=/mnt/persistent",
			fmt.Sprintf("vm_id=%s", cfg.VMID),
			fmt.Sprintf("epoch=%d", cfg.Epoch),
			fmt.Sprintf("choir.gateway_url=http://%s:8084", hostIP),
			fmt.Sprintf("choir.vmctl_url=http://%s:8083", hostIP),
			fmt.Sprintf("choir.maild_url=http://%s:8087", hostIP),
			fmt.Sprintf("choir.platform_url=http://%s:8086", hostIP),
			// Story versions publish through the proxy, which reads the
			// revision back from this computer; corpusd has no such route.
			fmt.Sprintf("choir.proxy_url=http://%s:8082", hostIP),
			fmt.Sprintf("ip=%s::%s:255.255.255.252::eth0:off", guestIP, hostIP),
		}
		runtimeArgs = append(runtimeArgs, guestIdentityKernelParams(cfg)...)
		if cfg.RecoveryReplayOnly {
			runtimeArgs = append(runtimeArgs, "choir.runtime_recovery_replay_only=1")
		}
		if cfg.MaintenanceHold {
			runtimeArgs = append(runtimeArgs, "choir.runtime_maintenance_hold=1")
		}
		if hotplug != nil {
			runtimeArgs = append(runtimeArgs, memoryHotplugKernelParam)
		}
		runtimeArgs = append(runtimeArgs, m.guestReclaimKernelParams(cfg)...)
		runtimeArgs = append(runtimeArgs, m.capsuleTunnelIdentityKernelParams(cfg)...)
		runtimeArgs = append(runtimeArgs, m.checkpointCadenceKernelParams(cfg)...)
		runtimeArgs = append(runtimeArgs, m.guestDNSKernelParams(cfg, hostIP)...)
		bootArgs = strings.Join(append([]string{kernelParams}, runtimeArgs...), " ")
	} else {
		// Legacy approach with custom init script. Keep the same runtime service
		// URLs as the microvm.nix path so guest env extraction can configure
		// gateway, vmctl, maild, and platform uniformly.
		bootArgs = fmt.Sprintf(
			"console=ttyS0 reboot=k panic=1 pci=off "+
				"root=/dev/vda rw init=/bin/init "+
				"guest_port=%d persistent=/mnt/persistent "+
				"vm_id=%s epoch=%d "+
				"ip=%s::%s:255.255.255.252::eth0:off",
			cfg.GuestPort, cfg.VMID, cfg.Epoch,
			guestIP, hostIP,
		)
		legacyRuntimeArgs := []string{
			fmt.Sprintf("choir.gateway_url=http://%s:8084", hostIP),
			fmt.Sprintf("choir.vmctl_url=http://%s:8083", hostIP),
			fmt.Sprintf("choir.maild_url=http://%s:8087", hostIP),
			fmt.Sprintf("choir.platform_url=http://%s:8086", hostIP),
			fmt.Sprintf("choir.proxy_url=http://%s:8082", hostIP),
		}
		for _, arg := range append(legacyRuntimeArgs, guestIdentityKernelParams(cfg)...) {
			bootArgs += " " + arg
		}
		if hotplug != nil {
			bootArgs += " " + memoryHotplugKernelParam
		}
		for _, arg := range m.guestReclaimKernelParams(cfg) {
			bootArgs += " " + arg
		}
		for _, arg := range m.capsuleTunnelIdentityKernelParams(cfg) {
			bootArgs += " " + arg
		}
		for _, arg := range m.checkpointCadenceKernelParams(cfg) {
			bootArgs += " " + arg
		}
		for _, arg := range m.guestDNSKernelParams(cfg, hostIP) {
			bootArgs += " " + arg
		}
	}

	// Build boot-source config. If an initrd is available, include it
	// so the kernel can load ext4/erofs/virtio modules before mounting.
	// With microvm.nix, the initrd is essential for systemd boot.
	bootSource := map[string]interface{}{
		"kernel_image_path": cfg.KernelImagePath,
		"boot_args":         bootArgs,
	}
	if cfg.InitrdPath != "" {
		bootSource["initrd_path"] = cfg.InitrdPath
	}

	// Firecracker VM configuration.
	// No secrets, no provider credentials, no host paths (VAL-VM-011).
	fcConfig := map[string]interface{}{
		"boot-source": bootSource,
		"drives":      drives,
		"machine-config": map[string]interface{}{
			"vcpu_count":   cfg.MachineCPUCount,
			"mem_size_mib": cfg.MachineMemSizeMib,
		},
		"network-interfaces": []map[string]interface{}{
			{
				"iface_id":      "eth0",
				"guest_mac":     fmt.Sprintf("AA:FC:00:00:00:%02X", hostPort%256),
				"host_dev_name": tapNameForVMID(cfg.VMID),
			},
		},
	}
	if balloon := m.firecrackerBalloonConfig(cfg); balloon != nil {
		fcConfig["balloon"] = balloon
	}
	if hotplug != nil {
		fcConfig["memory-hotplug"] = hotplug
	}

	return fcConfig
}

func guestIdentityKernelParams(cfg VMConfig) []string {
	params := make([]string, 0, 5)
	if value := kernelParamValue(cfg.ComputerID); value != "" {
		params = append(params, "choir.computer_id="+value)
	}
	if value := kernelParamValue(cfg.RealizationID); value != "" {
		params = append(params, "choir.realization_id="+value)
	}
	if value := kernelParamValue(cfg.OwnerID); value != "" {
		params = append(params, "choir.owner_id="+value)
	}
	if value := kernelParamValue(cfg.DesktopID); value != "" {
		params = append(params, "choir.desktop_id="+value)
	}
	return params
}

func kernelParamValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		switch r {
		case ' ', '\t', '\n', '\r', '\x00':
			b.WriteByte('_')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// rotatingConsoleWriter retains Firecracker's guest serial stdout in a fixed
// set of files. Firecracker v1.15.1 accepts serial_out_path only through its
// API; its --no-api JSON config deliberately skips that field. With
// console=ttyS0, its stdout is therefore the supported serial capture point.
type rotatingConsoleWriter struct {
	mu          sync.Mutex
	path        string
	maxBytes    int64
	generations int
	file        *os.File
	size        int64
}

func newRotatingConsoleWriter(path string, maxBytes int64, generations int) (*rotatingConsoleWriter, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("console log size limit must be positive")
	}
	if generations < 0 {
		return nil, fmt.Errorf("console log generations must not be negative")
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open console log %s: %w", path, err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("stat console log %s: %w", path, err)
	}

	writer := &rotatingConsoleWriter{
		path:        path,
		maxBytes:    maxBytes,
		generations: generations,
		file:        file,
		size:        info.Size(),
	}
	if writer.size >= writer.maxBytes {
		if err := writer.rotate(); err != nil {
			_ = file.Close()
			return nil, err
		}
	}
	return writer, nil
}

func (w *rotatingConsoleWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	written := 0
	for len(p) > 0 {
		if w.size >= w.maxBytes {
			if err := w.rotate(); err != nil {
				return written, err
			}
		}

		available := int(w.maxBytes - w.size)
		if available > len(p) {
			available = len(p)
		}
		n, err := w.file.Write(p[:available])
		written += n
		w.size += int64(n)
		p = p[n:]
		if err != nil {
			return written, err
		}
		if n != available {
			return written, io.ErrShortWrite
		}
	}
	return written, nil
}

func (w *rotatingConsoleWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

func (w *rotatingConsoleWriter) rotate() error {
	if err := w.file.Close(); err != nil {
		return fmt.Errorf("close console log %s for rotation: %w", w.path, err)
	}

	for generation := w.generations; generation >= 1; generation-- {
		source := w.path
		if generation > 1 {
			source = fmt.Sprintf("%s.%d", w.path, generation-1)
		}
		destination := fmt.Sprintf("%s.%d", w.path, generation)
		if generation == w.generations {
			if err := os.Remove(destination); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove expired console log %s: %w", destination, err)
			}
		}
		if err := os.Rename(source, destination); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("rotate console log %s: %w", w.path, err)
		}
	}

	file, err := os.OpenFile(w.path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open rotated console log %s: %w", w.path, err)
	}
	w.file = file
	w.size = 0
	return nil
}

func consoleLogPath(stateDir, vmID string) string {
	return filepath.Join(stateDir, vmID, consoleLogName)
}

// launchFirecracker starts a Firecracker process for the VM cfg describes.
// hostPort is the host port assigned for this VM, used for setting up
// networking. The confinement (jailer or direct) is decided here, per launch.
func (m *Manager) launchFirecracker(cfg VMConfig, fcConfig map[string]interface{}, hostPort int) (err error) {
	vmID := cfg.VMID
	mode, reason := m.decideConfinement(cfg)
	receipt := &ConfinementReceipt{Mode: mode, Reason: reason}
	var files []jailFile
	if mode == ConfinementJailer {
		jailConfig, jailFiles, err := rewriteFirecrackerConfigForJail(fcConfig)
		if err != nil {
			return fmt.Errorf("jail launch config: %w", err)
		}
		fcConfig, files = jailConfig, jailFiles
		uid, err := m.allocateVMMUID(vmID)
		if err != nil {
			return err
		}
		receipt.UID, receipt.GID = uid, uid
		receipt.JailRoot = m.jailRoot(vmID)
		receipt.Cgroup = m.cgroupLeafRel(vmID)
	}

	// Write the config. A jailed launch writes the same bytes into the jail;
	// this host copy is what the snapshot launch-config hash reads.
	configData, err := json.Marshal(fcConfig)
	if err != nil {
		return fmt.Errorf("marshal firecracker config: %w", err)
	}

	configDir := filepath.Join(m.cfg.StateDir, vmID)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("create config dir %s: %w", configDir, err)
	}

	configPath := filepath.Join(configDir, "fc-config.json")
	if err := os.WriteFile(configPath, configData, 0o644); err != nil {
		return fmt.Errorf("write firecracker config: %w", err)
	}
	// The launch record pins the image the link names now, before any spawn,
	// for direct and jailed launches alike (D2 S1b, H6).
	if err := m.writeLaunchRecord(cfg); err != nil {
		return fmt.Errorf("launch record for VM %s: %w", vmID, err)
	}
	m.recordConfinementLocked(vmID, receipt)

	networkReady := false
	defer func() {
		if err != nil && networkReady {
			// The network was prepared for a process that never started.
			m.deleteTapDevice(tapNameForVMID(vmID))
		}
	}()

	if receipt.jailed() {
		notes, jailErr := m.prepareJail(vmID, receipt.UID, files, configData)
		if jailErr != nil {
			return fmt.Errorf("prepare jail for VM %s: %w", vmID, jailErr)
		}
		receipt.JailFiles = notes
		// Torn down by the deferred cleanup if any part of it failed.
		networkReady = true
		if err = m.prepareVMNetwork(vmID, hostPort, cfg.GuestPort, receipt.UID); err != nil {
			return err
		}
		err = m.startJailedFirecracker(vmID, receipt, jailFirecrackerLaunchArgs(m.wantsAPISocket(cfg)))
		return err
	}

	m.reclaimDirectOwnership(vmID)
	networkReady = true
	if err = m.prepareVMNetwork(vmID, hostPort, cfg.GuestPort, -1); err != nil {
		return err
	}

	// Build the Firecracker command.
	// --no-api disables the Firecracker API socket (kill hibernate mode);
	// snapshot mode serves the API on <state>/<vm>/api/fc.sock instead
	// (firecrackerLaunchArgs, S3).
	// --enable-pci matches the upstream microvm runner so the guest sees the
	// expected Firecracker device model.
	// --id sets the VM identifier.
	// --config-file provides the VM configuration.
	err = m.startFirecrackerProcess(vmID, m.firecrackerLaunchArgsWithAPI(vmID, configPath, m.wantsAPISocket(cfg)))
	return err
}

// prepareVMNetwork creates the VM's tap and host-side routing for its /30
// subnet. Boot and snapshot restore share it: a restored guest keeps the IP
// and MAC it had, so the tap must be exactly the one it was launched with.
// tapOwner >= 0 creates the tap owned by that uid/gid, so a jailed VMM with
// no capabilities can attach to it.
func (m *Manager) prepareVMNetwork(vmID string, hostPort int, guestPort int, tapOwner int) error {
	// Create the tap device for VM networking.
	// Firecracker requires the tap device to exist before launching.
	// The tap name comes from the config's host_dev_name field.
	tapName := tapNameForVMID(vmID)
	if err := m.createTapDevice(tapName, tapOwner); err != nil {
		log.Printf("vmmanager: warning: could not create tap device %s: %v (may already exist)", tapName, err)
	}

	// Configure host-side networking for the tap device.
	// Assign the host IP in the /30 subnet and set up DNAT
	// so traffic to the host port reaches the guest.
	guestIP, hostIP := m.guestAndHostIP(hostPort)
	if err := m.setupHostNetworking(tapName, hostIP, hostPort, guestIP, guestPort); err != nil {
		log.Printf("vmmanager: host networking for %s failed; VM %s not started: %v", tapName, vmID, err)
		return fmt.Errorf("host networking for VM %s: %w", vmID, err)
	}
	return nil
}

// startFirecrackerProcess starts Firecracker with args, retains its serial
// console, records its PID on the tracked instance, and watches its exit.
// Boot and snapshot restore share it.
func (m *Manager) startFirecrackerProcess(vmID string, args []string) error {
	if m.firecrackerStarter != nil {
		return m.firecrackerStarter(vmID, args)
	}
	bin := m.launchedVMM(vmID).Firecracker
	if bin == "" {
		bin = "firecracker"
	}

	// Firecracker v1.15.1 does not deserialize serial_out_path from its
	// --no-api config file. The kernel's ttyS0 output is instead written to
	// Firecracker stdout, which we retain per VM rather than mixing it into
	// vmctl's shared journal.
	// The console is a file firecracker writes directly, never a pipe that
	// needs this process to read it (console_file.go).
	consolePath := consoleLogPath(m.cfg.StateDir, vmID)
	console, err := openConsoleSink(consolePath, consoleLogMaxBytes, consoleLogGenerations)
	if err != nil {
		return fmt.Errorf("open console sink for VM %s: %w", vmID, err)
	}

	cmd := exec.Command(bin, args...)
	cmd.Stdout = console
	cmd.Stderr = os.Stderr
	// Nothing of vmctl's environment (it holds VMCTL_ROUTE_DSN) reaches the
	// VMM.
	cmd.Env = []string{}

	// The launch thread cuts the bounding set to CAP_NET_ADMIN (the VMM
	// attaches to an ownerless tap) before the fork: vmctl's own set is wider
	// because it runs the jailer.
	report, err := startWithThreadPolicy(cmd, threadPolicy{
		keepCaps:     directKeepCaps,
		tolerateCaps: legacyCaps,
		speculation:  m.cfg.Jailer.SpeculationDirect,
	})
	if inst, ok := m.vms[vmID]; ok && inst.confinement != nil && !inst.confinement.jailed() {
		inst.confinement.LaunchThread = &report
	}
	if err != nil {
		_ = console.Close()
		return fmt.Errorf("start firecracker: %w", err)
	}
	// The child holds its own descriptor for the console file.
	_ = console.Close()
	applyLaunchOOMScoreAdj(vmID, cmd.Process.Pid)
	exited := make(chan struct{})
	go watchConsoleLog(vmID, cmd.Process.Pid, consolePath, exited)

	// Store the process info.
	var expected *VMInstance
	if inst, ok := m.vms[vmID]; ok {
		expected = inst
		inst.PID = cmd.Process.Pid
		inst.cmd = cmd
		inst.done = make(chan struct{})
		if err := m.savePID(vmID, inst.PID); err != nil {
			log.Printf("vmmanager: warning: could not save pid for VM %s: %v", vmID, err)
		}
		if inst.confinement != nil {
			inst.confinement.PID = inst.PID
			m.recordConfinementLocked(vmID, inst.confinement)
		}
	}

	// Monitor the process in the background and close the host-side sink once
	// os/exec has stopped forwarding its stdout.
	go func() {
		err := cmd.Wait()
		close(exited)
		if expected != nil {
			close(expected.done)
		}
		if err != nil {
			log.Printf("vmmanager: firecracker process for VM %s exited with error: %v", vmID, err)
			if expected != nil {
				m.markInstanceFailed(vmID, expected)
			}
		}
		if expected != nil {
			m.releaseExitedVMNetwork(vmID, expected, cmd)
		}
	}()

	return nil
}

// killFirecrackerProcess forcefully terminates a Firecracker process
// and cleans up the associated tap device.
//
// cause names the kill path for its destruction receipt (receipts.go).
func (m *Manager) killFirecrackerProcess(inst *VMInstance, cause string) {
	vmID := strings.TrimSpace(inst.Config.VMID)
	receipt := DestructionReceipt{VMID: vmID, Cause: cause, Epoch: inst.Config.Epoch, HostURL: inst.HostURL}
	killedPrimaryPID := 0
	if inst.cmd != nil && inst.cmd.Process != nil {
		killedPrimaryPID = inst.cmd.Process.Pid
		m.killProcessWithReceipt(inst.cmd.Process, receipt)
		// Wait for the process to exit.
		if inst.done != nil {
			select {
			case <-inst.done:
			case <-time.After(5 * time.Second):
				log.Printf("vmmanager: timeout waiting for VM process %d to exit", inst.PID)
			}
		}
	} else if inst.PID > 0 {
		killedPrimaryPID = inst.PID
		if proc, err := os.FindProcess(inst.PID); err == nil {
			m.killProcessWithReceipt(proc, receipt)
		}
		// A jailed Firecracker is not vmctl's child; its pidfd watcher closes
		// done when it is gone. Reattached instances have no done channel.
		if inst.done != nil {
			select {
			case <-inst.done:
			case <-time.After(5 * time.Second):
				log.Printf("vmmanager: timeout waiting for VM process %d to exit", inst.PID)
			}
		}
	}
	inst.PID = 0
	inst.cmd = nil
	if vmID != "" {
		for _, pid := range firecrackerPIDsForVM(vmID) {
			if pid == killedPrimaryPID {
				continue
			}
			if proc, err := os.FindProcess(pid); err == nil {
				log.Printf("vmmanager: killing duplicate Firecracker process for VM %s (pid=%d)", vmID, pid)
				duplicate := receipt
				duplicate.Cause = cause + ":duplicate"
				m.killProcessWithReceipt(proc, duplicate)
			}
		}
		_ = os.Remove(m.pidPath(vmID))
	}

	// Clean up the tap device.
	if vmID != "" && len(vmID) >= 8 {
		m.deleteTapDevice(tapNameForVMID(vmID))
	}
}

// forceCleanup removes any leftover state for a VM before relaunching.
func (m *Manager) forceCleanup(vmID, cause string) {
	if inst, ok := m.vms[vmID]; ok {
		m.killFirecrackerProcess(inst, cause)
	}
}

// ErrLiveUnmanaged refuses a boot over a live Firecracker process for the same
// VM that the manager does not track.
var ErrLiveUnmanaged = errors.New("vm has a live untracked Firecracker process; refusing to boot over it")

// liveFirecrackerForVMLocked reports a live Firecracker process proven to be
// this VM's, by cmdline scan or by a PID file whose process still matches.
func (m *Manager) liveFirecrackerForVMLocked(vmID string) bool {
	if len(firecrackerPIDsForVM(vmID)) > 0 {
		return true
	}
	pid, err := m.loadPID(vmID)
	return err == nil && processExists(pid) && firecrackerCmdlineMatchesVM(pid, vmID)
}

// cause names the path that asked for the cleanup, for its destruction receipt.
func (m *Manager) cleanupOrphanedFirecrackerLocked(vmID, cause string) {
	pids := firecrackerPIDsForVM(vmID)
	tapName := tapNameForVMID(vmID)
	if len(pids) > 0 {
		for _, pid := range pids {
			log.Printf("vmmanager: killing orphaned Firecracker process for VM %s (pid=%d) before restart", vmID, pid)
			if proc, err := os.FindProcess(pid); err == nil {
				m.killProcessWithReceipt(proc, DestructionReceipt{VMID: vmID, Cause: cause + ":orphan"})
			}
		}
		_ = os.Remove(m.pidPath(vmID))
		// Firecracker does not own the tap lifetime. Remove it explicitly
		// after reclaiming an orphan, otherwise the next launch can fail with
		// TapOpen(IfreqExecuteError) even though no VM uses the interface.
		m.deleteTapDevice(tapName)
		return
	}

	pid, err := m.loadPID(vmID)
	if err != nil {
		// A stale tap can remain even when the PID file was already removed.
		m.deleteTapDevice(tapName)
		return
	}
	if !processExists(pid) {
		_ = os.Remove(m.pidPath(vmID))
		m.deleteTapDevice(tapName)
		return
	}
	if !firecrackerCmdlineMatchesVM(pid, vmID) {
		log.Printf("vmmanager: ignoring stale pid file for VM %s; pid %d does not identify this Firecracker VM", vmID, pid)
		_ = os.Remove(m.pidPath(vmID))
		m.deleteTapDevice(tapName)
		return
	}
	log.Printf("vmmanager: killing orphaned Firecracker process for VM %s (pid=%d) before restart", vmID, pid)
	inst := &VMInstance{
		Config: VMConfig{VMID: vmID},
		PID:    pid,
	}
	m.killFirecrackerProcess(inst, cause+":orphan-pidfile")
}

func (m *Manager) pidPath(vmID string) string {
	return filepath.Join(m.cfg.StateDir, vmID, "firecracker.pid")
}

func (m *Manager) savePID(vmID string, pid int) error {
	path := m.pidPath(vmID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(fmt.Sprintf("%d", pid)), 0o644)
}

func (m *Manager) loadPID(vmID string) (int, error) {
	data, err := os.ReadFile(m.pidPath(vmID))
	if err != nil {
		return 0, fmt.Errorf("load firecracker pid for VM %s: %w", vmID, err)
	}
	var pid int
	for _, b := range strings.TrimSpace(string(data)) {
		if b < '0' || b > '9' {
			return 0, fmt.Errorf("invalid firecracker pid for VM %s", vmID)
		}
		pid = pid*10 + int(b-'0')
	}
	if pid <= 0 {
		return 0, fmt.Errorf("invalid firecracker pid for VM %s", vmID)
	}
	return pid, nil
}

func processExists(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func firecrackerCmdlineMatchesVM(pid int, vmID string) bool {
	if pid <= 0 || strings.TrimSpace(vmID) == "" {
		return false
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return false
	}
	return firecrackerCmdlineBytesMatchVM(data, vmID)
}

type procCmdlineEntry struct {
	pid  int
	data []byte
}

func firecrackerPIDsForVM(vmID string) []int {
	vmID = strings.TrimSpace(vmID)
	if vmID == "" {
		return nil
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	cmdlines := make([]procCmdlineEntry, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		data, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err != nil {
			continue
		}
		cmdlines = append(cmdlines, procCmdlineEntry{pid: pid, data: data})
	}
	return firecrackerPIDsForVMFromEntries(cmdlines, vmID)
}

func firecrackerPIDsForVMFromEntries(entries []procCmdlineEntry, vmID string) []int {
	vmID = strings.TrimSpace(vmID)
	if vmID == "" {
		return nil
	}
	pids := make([]int, 0)
	for _, entry := range entries {
		if entry.pid <= 0 {
			continue
		}
		if firecrackerCmdlineBytesMatchVM(entry.data, vmID) {
			pids = append(pids, entry.pid)
		}
	}
	sort.Ints(pids)
	return pids
}

func firecrackerCmdlineBytesMatchVM(data []byte, vmID string) bool {
	vmID = strings.TrimSpace(vmID)
	if vmID == "" {
		return false
	}
	args := processCmdlineArgs(data)
	if len(args) == 0 || isJailerCmdline(args) {
		return false
	}
	hasFirecracker := false
	for _, arg := range args {
		if strings.Contains(filepath.Base(arg), "firecracker") {
			hasFirecracker = true
			break
		}
	}
	if !hasFirecracker {
		return false
	}
	for i, arg := range args {
		if arg == "--id" && i+1 < len(args) && args[i+1] == vmID {
			return true
		}
		if strings.TrimPrefix(arg, "--id=") != arg && strings.TrimPrefix(arg, "--id=") == vmID {
			return true
		}
	}
	return false
}

// isJailerCmdline reports the jailer's own process: its command line names
// firecracker (--exec-file) and carries --id, but it is not the VM's
// Firecracker and must never be adopted, reaped or counted as it.
func isJailerCmdline(args []string) bool {
	if len(args) > 0 && strings.Contains(filepath.Base(args[0]), "jailer") {
		return true
	}
	for _, arg := range args {
		if arg == "--exec-file" || strings.HasPrefix(arg, "--exec-file=") {
			return true
		}
	}
	return false
}

func processCmdlineArgs(data []byte) []string {
	raw := string(data)
	if strings.Contains(raw, "\x00") {
		parts := strings.Split(raw, "\x00")
		args := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				args = append(args, part)
			}
		}
		return args
	}
	return strings.Fields(raw)
}

type guestHealthProbeResult struct {
	Healthy bool
	Status  int
	Body    string
	Err     error
	// Replay state parsed from a 503 ReplayInProgress body. While set, the guest
	// is alive and making replay progress; it is NOT product-ready.
	ReplayInProgress        bool
	ReplaySequence          uint64
	ReplayCommittedSequence uint64
	// ReplayProgress ticks during post-replay work (vocabulary migration,
	// fence verification) while the applied sequence is stationary.
	ReplayProgress uint64
	// BootRefusal is the guest planner's typed refusal parsed from a 503
	// "refused" /health body. It carries the recovery witness the host persists
	// without reading guest logs.
	BootRefusal *GuestBootRefusal
}

// GuestBootRefusal is the guest planner refusal body served by a refused
// guest's /health listener (autoputer boot refusal surface).
type GuestBootRefusal struct {
	Kind              string `json:"kind"`
	Reason            string `json:"reason"`
	ComputerID        string `json:"computer_id"`
	LocalSequence     uint64 `json:"local_sequence"`
	WatermarkSequence uint64 `json:"watermark_sequence"`
	TargetSequence    uint64 `json:"target_sequence"`
	EmptyStore        bool   `json:"empty_store"`
	ChainExists       bool   `json:"chain_exists"`
}

// GuestBootRefusedError is the typed boot failure raised when the guest
// planner refused to materialize. vmctl reads it structurally (via the
// GuestBootRefusal method) to persist a durable recovery condition; the
// structural coupling keeps vmctl free of a vmmanager import.
type GuestBootRefusedError struct {
	VMID              string
	Kind              string
	Reason            string
	ComputerID        string
	LocalSequence     uint64
	WatermarkSequence uint64
	TargetSequence    uint64
	EmptyStore        bool
	ChainExists       bool
}

func (e *GuestBootRefusedError) Error() string {
	if e == nil {
		return "guest boot refused"
	}
	return fmt.Sprintf("guest boot refused (%s): %s (local=%d W=%d H=%d empty=%t chain=%t)",
		e.Kind, e.Reason, e.LocalSequence, e.WatermarkSequence, e.TargetSequence, e.EmptyStore, e.ChainExists)
}

// GuestBootRefusal mirrors vmctl's structural guestBootRefusal interface: the
// method set must match exactly so errors.As on the vmctl side finds it
// without importing this package.
func (e *GuestBootRefusedError) GuestBootRefusal() (kind, reason string, localSequence, watermarkSequence, targetSequence uint64, emptyStore, chainExists bool) {
	return e.Kind, e.Reason, e.LocalSequence, e.WatermarkSequence, e.TargetSequence, e.EmptyStore, e.ChainExists
}

// GuestProcessExitedError is the typed boot failure raised when the
// Firecracker child exited before the guest became ready. It bounds a dead
// boot promptly instead of waiting out the full readiness deadline.
type GuestProcessExitedError struct {
	VMID      string
	HostURL   string
	LastProbe string
}

func (e *GuestProcessExitedError) Error() string {
	if e == nil {
		return "guest process exited before readiness"
	}
	return fmt.Sprintf("guest process exited before readiness at %s (vm=%s, last probe: %s)", e.HostURL, e.VMID, e.LastProbe)
}

func (r guestHealthProbeResult) String() string {
	if r.Err != nil {
		return r.Err.Error()
	}
	if r.Status != 0 {
		body := strings.TrimSpace(r.Body)
		if body != "" {
			return fmt.Sprintf("HTTP %d: %s", r.Status, body)
		}
		return fmt.Sprintf("HTTP %d", r.Status)
	}
	return "no probe result"
}

func (m *Manager) probeGuestHealthDetailed(hostURL string) guestHealthProbeResult {
	client := &http.Client{
		Timeout: m.cfg.HealthCheckTimeout,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: m.cfg.HealthCheckTimeout,
			}).DialContext,
		},
	}
	resp, err := client.Get(hostURL + "/health")
	if err != nil {
		return guestHealthProbeResult{Err: err}
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	result := guestHealthProbeResult{
		Healthy: resp.StatusCode == http.StatusOK,
		Status:  resp.StatusCode,
		Body:    string(body),
	}
	// A 503 ReplayInProgress body signals a live guest mid-replay: liveness, not
	// readiness. Parse the applied and durably committed sequence so the wait
	// loop can gate the kill on replay progress instead of wall clock (B5/B6).
	// A 503 "refused" body is the guest planner's typed refusal: the boot is
	// deterministically dead for these inputs, and the witness must reach vmctl.
	if resp.StatusCode == http.StatusServiceUnavailable {
		var refusalBody struct {
			Status string `json:"status"`
			GuestBootRefusal
		}
		if json.Unmarshal(body, &refusalBody) == nil && refusalBody.Status == "refused" && strings.TrimSpace(refusalBody.Kind) != "" {
			refusal := refusalBody.GuestBootRefusal
			result.BootRefusal = &refusal
		}
		var replayBody struct {
			Status            string `json:"status"`
			Sequence          uint64 `json:"sequence"`
			CommittedSequence uint64 `json:"committed_sequence"`
			Progress          uint64 `json:"progress"`
		}
		if result.BootRefusal == nil && json.Unmarshal(body, &replayBody) == nil && replayBody.Status == "replaying" {
			result.ReplayInProgress = true
			result.ReplaySequence = replayBody.Sequence
			result.ReplayCommittedSequence = replayBody.CommittedSequence
			result.ReplayProgress = replayBody.Progress
		}
	}
	return result
}

// probeGuestHealth attempts to reach the guest's /health endpoint.
// On Linux/Node B with real Firecracker VMs, this uses HTTP to probe
// the guest autoputer runtime. On macOS or environments without Firecracker,
// this returns true by default (the VM isn't actually running).
func (m *Manager) probeGuestHealth(hostURL string) bool {
	return m.probeGuestHealthDetailed(hostURL).Healthy
}

// waitForGuestReady polls the guest /health until ready. A typed guest boot
// refusal and an exited Firecracker child both fail the boot immediately: a
// deterministic refusal must reach vmctl promptly instead of waiting out the
// readiness deadline. alive may be nil (callers without a process handle).
func (m *Manager) waitForGuestReady(hostURL string, tl *BootTimeline, alive func() bool) error {
	deadline := time.Now().Add(m.cfg.BootReadyTimeout)
	stall := m.cfg.ReplayStallTimeout
	if stall <= 0 {
		stall = 120 * time.Second
	}
	var lastProbe guestHealthProbeResult
	var lastReplaySeq uint64
	var lastReplayProgress uint64
	var lastSequenceAdvance time.Time
	firstReplay := true
	sawHTTP := false
	sawHealthy := false
	for {
		if alive != nil && !alive() {
			tl.markDetail("guest_process_exited", fmt.Sprintf("last probe: %s", lastProbe.String()))
			return &GuestProcessExitedError{VMID: bootTimelineVMID(tl), HostURL: hostURL, LastProbe: lastProbe.String()}
		}
		lastProbe = m.probeGuestHealthDetailed(hostURL)
		if !sawHTTP && lastProbe.Err == nil {
			// First HTTP-level response (any status): the guest's listener is
			// bound and answering. Distinct from healthy — replay gate may
			// still be 503.
			sawHTTP = true
			tl.markDetail("first_http_response", fmt.Sprintf("status=%d", lastProbe.Status))
		}
		if lastProbe.BootRefusal != nil {
			refusal := lastProbe.BootRefusal
			tl.markDetail("boot_refused", fmt.Sprintf("kind=%s local=%d W=%d H=%d", refusal.Kind, refusal.LocalSequence, refusal.WatermarkSequence, refusal.TargetSequence))
			return &GuestBootRefusedError{
				VMID:              bootTimelineVMID(tl),
				Kind:              refusal.Kind,
				Reason:            refusal.Reason,
				ComputerID:        refusal.ComputerID,
				LocalSequence:     refusal.LocalSequence,
				WatermarkSequence: refusal.WatermarkSequence,
				TargetSequence:    refusal.TargetSequence,
				EmptyStore:        refusal.EmptyStore,
				ChainExists:       refusal.ChainExists,
			}
		}
		if lastProbe.Healthy {
			if !sawHealthy {
				sawHealthy = true
				tl.mark("first_healthy")
			}
			return nil
		}
		if lastProbe.ReplayInProgress {
			if firstReplay {
				tl.markDetail("replay_first_seen", fmt.Sprintf("seq=%d committed=%d", lastProbe.ReplaySequence, lastProbe.ReplayCommittedSequence))
			}
			if firstReplay || lastProbe.ReplaySequence != lastReplaySeq || lastProbe.ReplayProgress != lastReplayProgress {
				firstReplay = false
				lastReplaySeq = lastProbe.ReplaySequence
				lastReplayProgress = lastProbe.ReplayProgress
				lastSequenceAdvance = time.Now()
			}
			// A replaying guest is alive and progressing: extend the boot window.
			// The deadline only applies to a guest that has never reported replay
			// progress, and the stall window kills a genuinely stuck replay.
			if !lastSequenceAdvance.IsZero() && time.Since(lastSequenceAdvance) > stall {
				tl.markDetail("replay_stalled", fmt.Sprintf("seq=%d stall=%s", lastProbe.ReplaySequence, stall))
				return fmt.Errorf("guest replay stalled at %s (no sequence advance for %s, seq=%d): %s", hostURL, stall, lastProbe.ReplaySequence, lastProbe.String())
			}
			time.Sleep(250 * time.Millisecond)
			continue
		}
		if time.Now().After(deadline) {
			tl.markDetail("deadline_exceeded", fmt.Sprintf("timeout=%s", m.cfg.BootReadyTimeout))
			return fmt.Errorf("guest did not become healthy at %s within %s (last probe: %s)", hostURL, m.cfg.BootReadyTimeout, lastProbe.String())
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// bootTimelineVMID returns the receipt's VMID, tolerating a nil receipt (the
// readiness wait is exercised without one in focused tests).
func bootTimelineVMID(tl *BootTimeline) string {
	if tl == nil {
		return ""
	}
	return tl.VMID
}

// recordGuestRefusalOnTimeline copies a typed guest refusal into the persisted
// boot receipt so the witness survives independently of the error string.
func recordGuestRefusalOnTimeline(tl *BootTimeline, err error) {
	if tl == nil || err == nil {
		return
	}
	var refused *GuestBootRefusedError
	if !errors.As(err, &refused) {
		return
	}
	tl.mu.Lock()
	defer tl.mu.Unlock()
	refusal := GuestBootRefusal{
		Kind:              refused.Kind,
		Reason:            refused.Reason,
		ComputerID:        refused.ComputerID,
		LocalSequence:     refused.LocalSequence,
		WatermarkSequence: refused.WatermarkSequence,
		TargetSequence:    refused.TargetSequence,
		EmptyStore:        refused.EmptyStore,
		ChainExists:       refused.ChainExists,
	}
	tl.Refusal = &refusal
}

// loadEpoch loads the epoch counter for a VM from persistent storage.
func (m *Manager) loadEpoch(vmID string) (int64, error) {
	path := filepath.Join(m.cfg.StateDir, vmID, "epoch")
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var epoch int64
	for _, b := range data {
		if b >= '0' && b <= '9' {
			epoch = epoch*10 + int64(b-'0')
		}
	}
	if epoch == 0 {
		return 0, fmt.Errorf("invalid epoch data")
	}
	return epoch, nil
}

// saveEpoch persists the epoch counter for a VM.
func (m *Manager) saveEpoch(vmID string, epoch int64) error {
	path := filepath.Join(m.cfg.StateDir, vmID, "epoch")
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(fmt.Sprintf("%d", epoch)), 0o644)
}

// copyFile copies a file from src to dst. It creates the destination
// directory if needed and sets the output file to be writable by the
// owner. This is used to create per-VM writable copies of the rootfs
// image so each VM has its own isolated filesystem (VAL-VM-005).
func (m *Manager) copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create dir %s: %w", filepath.Dir(dst), err)
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open src %s: %w", src, err)
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("create dst %s: %w", dst, err)
	}
	defer out.Close()

	if _, err := out.ReadFrom(in); err != nil {
		return fmt.Errorf("copy %s to %s: %w", src, dst, err)
	}
	return nil
}

func (m *Manager) createCredentialDisk(vmStateDir, encodedEnvelope, gatewayToken string, extraFiles map[string][]byte) (string, error) {
	if strings.TrimSpace(encodedEnvelope) == "" && strings.TrimSpace(gatewayToken) == "" {
		return "", fmt.Errorf("credential disk requires an envelope or gateway token")
	}
	if os.Geteuid() != 0 || os.Getegid() != 0 {
		return "", fmt.Errorf("credential disk construction requires root uid/gid")
	}
	stagingRoot, err := os.MkdirTemp(vmStateDir, ".credential-populate-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stagingRoot)
	if env := strings.TrimSpace(encodedEnvelope); env != "" {
		if err := os.WriteFile(filepath.Join(stagingRoot, "computer-event-envelope"), []byte(env+"\n"), 0o400); err != nil {
			return "", err
		}
	}
	// The autoputer gateway token rides the same root-only credential disk
	// (read inside the guest via RUNTIME_GATEWAY_TOKEN_FILE), replacing the
	// world-readable kernel-cmdline channel (S1 security-floor remainder).
	if token := strings.TrimSpace(gatewayToken); token != "" {
		if err := os.WriteFile(filepath.Join(stagingRoot, "gateway-token"), []byte(token+"\n"), 0o400); err != nil {
			return "", err
		}
	}
	// Extra root-only files (the Claude delegate grant and token) ride the
	// same disk under the same mode and owner checks.
	names := []string{"computer-event-envelope", "gateway-token"}
	for name, content := range extraFiles {
		if name == "" || strings.ContainsAny(name, `/\`) || name == "." || name == ".." || slices.Contains(names, name) {
			return "", fmt.Errorf("credential disk extra file name %q is invalid", name)
		}
		if err := os.WriteFile(filepath.Join(stagingRoot, name), content, 0o400); err != nil {
			return "", err
		}
		names = append(names, name)
	}
	for _, name := range names {
		path := filepath.Join(stagingRoot, name)
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o400 {
			return "", fmt.Errorf("credential staging inode %s must be a regular mode-0400 file", name)
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || stat.Uid != 0 || stat.Gid != 0 {
			return "", fmt.Errorf("credential staging inode %s must be root-owned", name)
		}
	}
	temporaryImage, err := os.CreateTemp(vmStateDir, ".credential-*.img")
	if err != nil {
		return "", err
	}
	temporaryPath := temporaryImage.Name()
	keep := false
	defer func() {
		_ = temporaryImage.Close()
		if !keep {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporaryImage.Truncate(8 << 20); err != nil {
		return "", err
	}
	if err := temporaryImage.Close(); err != nil {
		return "", err
	}
	mkfs := strings.TrimSpace(m.cfg.MkfsExt4Path)
	if mkfs == "" {
		mkfs = "mkfs.ext4"
	}
	output, err := exec.Command(mkfs, "-F", "-q", "-b", "4096", "-m", "0", "-d", stagingRoot, "-L", "CHOIR_CRED", temporaryPath).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("format credential disk: %w: %s", err, strings.TrimSpace(string(output)))
	}
	finalPath := filepath.Join(vmStateDir, "credential.img")
	if err := os.Rename(temporaryPath, finalPath); err != nil {
		return "", err
	}
	if err := os.Chmod(finalPath, 0o600); err != nil {
		_ = os.Remove(finalPath)
		return "", err
	}
	keep = true
	return finalPath, nil
}

func (m *Manager) createDataImage(path string, sizeMB int) error {
	mkfsBin := strings.TrimSpace(m.cfg.MkfsExt4Path)
	if mkfsBin == "" || mkfsBin == "mkfs.ext4" {
		mkfsBin = findBinary("mkfs.ext4", "/run/current-system/sw/bin/mkfs.ext4")
	}
	staging, cleanup, seedErr := m.sourceSeedStagingRoot()
	if seedErr != nil {
		return fmt.Errorf("seed data image source checkout: %w", seedErr)
	}
	if cleanup != nil {
		defer cleanup()
	}
	return createSparseDataImageSeeded(path, sizeMB, mkfsBin, staging)
}

// sourceSeedStagingRoot returns a staging directory containing
// files/Source/platform as a clean git checkout at the commit this vmctl
// binary was built from (the deployed source identity). The capsule
// executor's PreflightSourceSnapshot requires a clean git tree with a HEAD;
// without a seed, the engineering desk's cast reconcile fails forever on a
// virgin computer. Returns ("", nil, nil) when seeding is disabled or the
// configured repo path is absent — the image then ships an empty Source dir.
func (m *Manager) sourceSeedStagingRoot() (staging string, cleanup func(), err error) {
	repo := m.sourceSeedRepo()
	if repo == "" {
		return "", nil, nil
	}
	dir, mkErr := os.MkdirTemp("", "choir-source-seed-*")
	if mkErr != nil {
		return "", nil, mkErr
	}
	cleanup = func() { _ = os.RemoveAll(dir) }
	target := filepath.Join(dir, "files", "Source", "platform")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", cleanup, err
	}
	commit := strings.TrimSpace(buildinfo.Commit)
	args := []string{"clone", "--quiet", repo, target}
	if out, cloneErr := exec.Command("git", args...).CombinedOutput(); cloneErr != nil {
		return "", cleanup, fmt.Errorf("git clone %s: %w (%s)", repo, cloneErr, strings.TrimSpace(string(out)))
	}
	if seedCheckoutCommit(commit) {
		if out, coErr := exec.Command("git", "-C", target, "checkout", "--quiet", commit).CombinedOutput(); coErr != nil {
			return "", cleanup, fmt.Errorf("git checkout %s in seeded source: %w (%s)", commit, coErr, strings.TrimSpace(string(out)))
		}
	}
	// The capsule preflight rejects a dirty tree. filemode/ctime noise across
	// mkfs seeding must not read as dirt; untracked preexisting host files
	// (.envrc.local et al.) stay untracked and invisible to `git diff --quiet`.
	if out, cfgErr := exec.Command("git", "-C", target, "config", "core.filemode", "false").CombinedOutput(); cfgErr != nil {
		return "", cleanup, fmt.Errorf("seeded source config: %w (%s)", cfgErr, strings.TrimSpace(string(out)))
	}
	return dir, cleanup, nil
}

func createSparseDataImageSeeded(path string, sizeMB int, mkfsBin string, stagingDir string) (err error) {
	if sizeMB <= 0 {
		return fmt.Errorf("data image size must be positive")
	}
	path = filepath.Clean(path)
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create data image dir %s: %w", parent, err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("create data image %s: %w", path, err)
	}
	defer func() {
		if err != nil {
			_ = os.Remove(path)
		}
	}()
	if err = file.Truncate(int64(sizeMB) * 1024 * 1024); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("size data image %s: %w", path, err)
	}
	args := []string{"-F", "-L", "go-choir-data"}
	if strings.TrimSpace(stagingDir) != "" {
		args = append(args, "-d", stagingDir)
	}
	args = append(args, path)
	cmd := exec.Command(mkfsBin, args...)
	if output, commandErr := cmd.CombinedOutput(); commandErr != nil {
		return fmt.Errorf("mkfs.ext4 data image %s: %w (%s)", path, commandErr, string(output))
	}
	if err := syncRegularFileAndParent(path); err != nil {
		return fmt.Errorf("sync data image %s: %w", path, err)
	}
	return nil
}

func (m *Manager) ensureDataImageMinSize(path string, sizeMB int) error {
	if sizeMB <= 0 {
		return fmt.Errorf("data image size must be positive")
	}
	want := int64(sizeMB) * 1024 * 1024
	info, err := regularFileInfo(path)
	if err != nil {
		return err
	}
	if info.Size() >= want {
		return nil
	}
	if err := os.Truncate(path, want); err != nil {
		return fmt.Errorf("truncate data image %s: %w", path, err)
	}
	resizeBin := findBinary("resize2fs", "/run/current-system/sw/bin/resize2fs")
	cmd := exec.Command(resizeBin, path)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("resize2fs data image %s: %w (%s)", path, err, string(output))
	}
	if err := syncRegularFileAndParent(path); err != nil {
		return fmt.Errorf("sync resized data image %s: %w", path, err)
	}
	return nil
}

// QuarantineDataImage atomically moves a VM's data image into the
// deterministic recovery quarantine. If maxRetained > 0, it retains prior
// completed recoveries only up to maxRetained and never removes an in-progress
// quarantine. If maxRetained <= 0, retention pruning is disabled.
func (m *Manager) QuarantineDataImage(stateRoot, vmID string, recoveryGeneration uint64, operationID string, maxRetained int) (string, error) {
	vmDir, err := recoveryVMStateDir(stateRoot, vmID)
	if err != nil {
		return "", err
	}
	if recoveryGeneration == 0 || !validRecoveryOperationID(operationID) {
		return "", fmt.Errorf("invalid recovery identity")
	}
	dataImage := filepath.Join(vmDir, "data.img")
	quarantine := filepath.Join(vmDir, fmt.Sprintf("data.img.quarantine-%d-%s", recoveryGeneration, operationID))
	if maxRetained > 0 {
		if _, err := pruneCompletedQuarantines(vmDir, maxRetained); err != nil {
			return "", err
		}
	}
	info, err := regularFileInfo(dataImage)
	if err != nil {
		return "", fmt.Errorf("validate data image: %w", err)
	}
	if err := ensureQuarantineCapacity(vmDir, info); err != nil {
		return "", err
	}
	if _, err := os.Lstat(quarantine); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			return "", fmt.Errorf("recovery quarantine already exists")
		}
		return "", fmt.Errorf("inspect recovery quarantine: %w", err)
	}
	if err := syncRegularFile(dataImage); err != nil {
		return "", fmt.Errorf("sync data image before quarantine: %w", err)
	}
	if err := os.Rename(dataImage, quarantine); err != nil {
		return "", fmt.Errorf("quarantine data image: %w", err)
	}
	if err := syncRegularFileAndParent(quarantine); err != nil {
		return "", fmt.Errorf("durably quarantine data image: %w", err)
	}
	// The rename kept the image's mode; a pre-change image is 0644.
	if err := tightenDataImageMode(quarantine); err != nil {
		log.Printf("vmmanager: quarantined image %s mode not tightened: %v", quarantine, err)
	}
	return quarantine, nil
}

// StageSparseImage creates the deterministic, default-empty successor image
// for a recovery. The source quarantine remains untouched.
func (m *Manager) StageSparseImage(stateRoot, vmID string, recoveryGeneration uint64, operationID string, sizeMB int) (string, error) {
	vmDir, err := recoveryVMStateDir(stateRoot, vmID)
	if err != nil {
		return "", err
	}
	if recoveryGeneration == 0 || !validRecoveryOperationID(operationID) || sizeMB <= 0 {
		return "", fmt.Errorf("invalid recovery staging identity")
	}
	staging := filepath.Join(vmDir, fmt.Sprintf("data.img.staging-%d-%s", recoveryGeneration, operationID))
	if _, err := os.Lstat(staging); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			return "", fmt.Errorf("recovery staging image already exists")
		}
		return "", fmt.Errorf("inspect recovery staging image: %w", err)
	}
	if err := m.createDataImage(staging, sizeMB); err != nil {
		return "", err
	}
	return staging, nil
}

type recoveryQuarantine struct {
	path       string
	generation uint64
	operation  string
	modTime    time.Time
}

func recoveryVMStateDir(stateRoot, vmID string) (string, error) {
	root := filepath.Clean(strings.TrimSpace(stateRoot))
	if !filepath.IsAbs(root) || vmID == "" || filepath.Base(vmID) != vmID {
		return "", fmt.Errorf("invalid VM state path")
	}
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return "", err
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return "", fmt.Errorf("VM state root is not a directory")
	}
	vmDir := filepath.Join(root, vmID)
	relative, err := filepath.Rel(root, vmDir)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("VM state path escapes root")
	}
	info, err := os.Lstat(vmDir)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", fmt.Errorf("VM state directory is not a directory")
	}
	return vmDir, nil
}

func validRecoveryOperationID(operationID string) bool {
	if operationID == "" || filepath.Base(operationID) != operationID {
		return false
	}
	for _, character := range operationID {
		if !(character >= 'a' && character <= 'z') && !(character >= 'A' && character <= 'Z') &&
			!(character >= '0' && character <= '9') && character != '-' && character != '_' {
			return false
		}
	}
	return true
}

func regularFileInfo(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("path is not a regular file")
	}
	return info, nil
}

func syncRegularFile(path string) error {
	if _, err := regularFileInfo(path); err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}

func syncRegularFileAndParent(path string) error {
	if err := syncRegularFile(path); err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func ensureQuarantineCapacity(vmDir string, info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Blocks <= 0 {
		return nil
	}
	var filesystem syscall.Statfs_t
	if err := syscall.Statfs(vmDir, &filesystem); err != nil {
		return fmt.Errorf("inspect recovery filesystem capacity: %w", err)
	}
	available := uint64(filesystem.Bavail) * uint64(filesystem.Bsize)
	retainedBytes := uint64(stat.Blocks) * 512
	if available < retainedBytes {
		return fmt.Errorf("insufficient capacity to retain quarantined data image")
	}
	return nil
}

func pruneCompletedQuarantines(vmDir string, maxRetained int) (int, error) {
	entries, err := os.ReadDir(vmDir)
	if err != nil {
		return 0, err
	}
	const prefix = "data.img.quarantine-"
	var quarantines []recoveryQuarantine
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		suffix := strings.TrimPrefix(name, prefix)
		separator := strings.IndexByte(suffix, '-')
		if separator <= 0 {
			continue
		}
		generation, parseErr := strconv.ParseUint(suffix[:separator], 10, 64)
		operation := suffix[separator+1:]
		if parseErr != nil || generation == 0 || !validRecoveryOperationID(operation) {
			continue
		}
		path := filepath.Join(vmDir, name)
		info, err := regularFileInfo(path)
		if err != nil {
			return 0, fmt.Errorf("validate retained quarantine %s: %w", path, err)
		}
		quarantines = append(quarantines, recoveryQuarantine{path: path, generation: generation, operation: operation, modTime: info.ModTime()})
	}
	// Oldest first: generation is monotonic per VM, mtime breaks ties within a
	// generation (operation IDs are random hex, not ordered).
	sort.Slice(quarantines, func(left, right int) bool {
		if quarantines[left].generation != quarantines[right].generation {
			return quarantines[left].generation < quarantines[right].generation
		}
		return quarantines[left].modTime.Before(quarantines[right].modTime)
	})
	prunedCount := 0
	for len(quarantines) >= maxRetained {
		pruned := false
		for index, quarantine := range quarantines {
			if !recoveryJournalPrunable(vmDir, quarantine.generation, quarantine.operation) {
				continue
			}
			if err := os.Remove(quarantine.path); err != nil {
				return prunedCount, fmt.Errorf("prune completed recovery quarantine: %w", err)
			}
			if err := syncDirectory(vmDir); err != nil {
				return prunedCount, fmt.Errorf("sync recovery quarantine pruning: %w", err)
			}
			quarantines = append(quarantines[:index], quarantines[index+1:]...)
			pruned = true
			prunedCount++
			break
		}
		if !pruned {
			return prunedCount, fmt.Errorf("recovery quarantine retention capacity reached")
		}
	}
	return prunedCount, nil
}

// PruneRecoveryQuarantines sweeps every VM dir under stateRoot and removes
// post-swap quarantine images beyond maxRetained per VM. It is the periodic
// counterpart to the per-recovery prune in QuarantineDataImage: without it,
// snapshots accumulate forever on VMs that never recover again.
func (m *Manager) PruneRecoveryQuarantines(stateRoot string, maxRetained int) (int, error) {
	if maxRetained < 1 {
		maxRetained = 1
	}
	entries, err := os.ReadDir(stateRoot)
	if err != nil {
		return 0, err
	}
	pruned := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		vmDir := filepath.Join(stateRoot, entry.Name())
		// Keep maxRetained post-swap quarantines: prune down to maxRetained.
		n, err := pruneCompletedQuarantines(vmDir, maxRetained+1)
		pruned += n
		if err != nil {
			return pruned, fmt.Errorf("prune %s: %w", vmDir, err)
		}
	}
	return pruned, nil
}

// recoveryJournalPrunable reports whether a quarantined image may be deleted.
// Only post-swap phases qualify: once the journal records "swapped" (or any
// later phase) the VM runs the staged image and the quarantine is a rollback
// copy, not the live store. Pre-swap phases (fenced, stopped, staging, and
// key_copied in journals written before key delivery moved to escrow) mean the quarantine may hold the only copy of the original image.
func recoveryJournalPrunable(vmDir string, generation uint64, operation string) bool {
	journal, err := os.ReadFile(filepath.Join(vmDir, fmt.Sprintf("rec-%d-%s.journal", generation, operation)))
	if err != nil {
		return false
	}
	var record struct {
		Phase string `json:"phase"`
	}
	if json.Unmarshal(journal, &record) != nil {
		return false
	}
	switch record.Phase {
	case "swapped", "booted", "verified", "route_published", "done":
		return true
	default:
		return false
	}
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

// createTapDevice creates a TAP network device for Firecracker VM
// networking. Firecracker requires the TAP device to pre-exist on the
// host before the VM can use it. The vmctl service needs CAP_NET_ADMIN
// to create TAP devices, which is configured in node-b.nix.
//
// The tap device is given a host-side IP in the 10.200.X.1/30 range,
// and the guest is expected to configure 10.200.X.2 as its IP. NAT
// masquerading is set up so the guest can reach the host's localhost
// services (gateway, etc.).
//
// owner >= 0 creates the tap owned by that uid/gid: a jailed VMM holds no
// capability and can attach only to a tap its uid owns.
func (m *Manager) createTapDevice(name string, owner int) error {
	h := m.net()

	// Check if the tap device already exists.
	if _, err := h.ip("link", "show", name); err == nil {
		if owner < 0 {
			return nil // already exists
		}
		// The owner of an existing tap cannot be changed: recreate it.
		m.deleteTapDevice(name)
	}

	// Create the tap device.
	createArgs := []string{"tuntap", "add", "dev", name, "mode", "tap"}
	if owner >= 0 {
		createArgs = append(createArgs, "user", strconv.Itoa(owner), "group", strconv.Itoa(owner))
	}
	if _, err := h.ip(createArgs...); err != nil {
		return fmt.Errorf("create tap device %s: %w", name, err)
	}

	// Bring the interface up.
	if _, err := h.ip("link", "set", name, "up"); err != nil {
		return fmt.Errorf("bring up tap device %s: %w", name, err)
	}

	log.Printf("vmmanager: created tap device %s", name)
	return nil
}

// deleteTapDevice removes every firewall rule the tap owns (filter and nat,
// by its go-choir-vm-<tap> comment or, for untagged older rules, its
// interface name) and then the TAP device itself. Every VM teardown path ends
// here: stop, kill, crash exit, failed launch, orphan and stale-tap cleanup.
func (m *Manager) deleteTapDevice(name string) {
	h := m.net()
	h.removeTap(name)
	if _, err := h.ip("link", "show", name); err != nil {
		return // already gone
	}
	if _, err := h.ip("link", "del", name); err != nil {
		log.Printf("vmmanager: warning: could not delete tap device %s: %v", name, err)
	}
}

func iptablesDeleteArgsForListedRule(table, chain, line, match string) ([]string, bool) {
	if strings.TrimSpace(match) == "" || !strings.Contains(line, match) {
		return nil, false
	}
	fields := iptablesRuleFields(line)
	if len(fields) < 2 || fields[0] != "-A" || fields[1] != chain {
		return nil, false
	}
	args := make([]string, 0, len(fields)+2)
	if table != "" {
		args = append(args, "-t", table)
	}
	args = append(args, "-D", chain)
	args = append(args, fields[2:]...)
	return args, true
}

func iptablesRuleFields(line string) []string {
	fields := make([]string, 0)
	var current strings.Builder
	var quote rune
	escaped := false
	flush := func() {
		if current.Len() == 0 {
			return
		}
		fields = append(fields, current.String())
		current.Reset()
	}
	for _, r := range line {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
				continue
			}
			current.WriteRune(r)
			continue
		}
		if r == '"' || r == '\'' {
			quote = r
			continue
		}
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			flush()
			continue
		}
		current.WriteRune(r)
	}
	flush()
	return fields
}

// setupHostNetworking configures the host-side networking for a VM's
// tap device. It assigns the host IP address to the tap interface and
// installs (hostfirewall.go):
//   - the guest's isolation: egress ACCEPT bound to its own address inside
//     the default-deny guest forward chain, which drops every destination in
//     the guest pool; INPUT anti-spoofing; source-bound host service ACCEPTs
//   - DNAT from 127.0.0.1:<hostPort> to the guest, and from the tap's host
//     IP to the intended host-local services
//   - MASQUERADE so host replies route back through the tap
//
// Without masquerading, the guest can send packets to the host but the
// host's reply packets don't route back to the guest's /30 subnet.
// The MASQUERADE rule makes host replies go back through the tap device.
func (m *Manager) setupHostNetworking(tapName, hostIP string, hostPort int, guestIP string, guestPort int) error {
	h := m.net()

	// Assign the host IP to the tap device.
	if _, err := h.ip("addr", "add", hostIP+"/30", "dev", tapName); err != nil {
		log.Printf("vmmanager: note: ip addr add %s/30 on %s: %v (may already be set)", hostIP, tapName, err)
	}

	// Enable IP forwarding globally.
	_ = h.writeProc("/proc/sys/net/ipv4/ip_forward", "1")

	// Enable route_localnet on the tap device. This allows the host to
	// route packets between the guest's subnet and 127.0.0.1 (localhost).
	// Without this, the DNAT rules that redirect guest→gateway traffic
	// (10.200.X.1:8084 → 127.0.0.1:8084) would be silently dropped because
	// the kernel treats 127.0.0.0/8 as a martian source on non-loopback
	// interfaces. This is critical for the guest to reach the gateway.
	routeLocalnet := fmt.Sprintf("/proc/sys/net/ipv4/conf/%s/route_localnet", tapName)
	if err := h.writeProc(routeLocalnet, "1"); err != nil {
		log.Printf("vmmanager: warning: could not enable route_localnet on %s: %v", tapName, err)
	}

	// Also enable accept_local so the host accepts packets from local
	// subnets on the tap interface.
	_ = h.writeProc(fmt.Sprintf("/proc/sys/net/ipv4/conf/%s/accept_local", tapName), "1")

	// SEC-03: every step below is part of the floor; a failure is returned
	// and the VM does not start (docs/problems/host-egress-floor-fails-open-2026-10-21.md).
	// F6 first: the guest egress policy (guest_egress.go) narrows what any
	// tap may reach before the per-tap rules see the packet, so it is in
	// place before this tap's egress ACCEPT.
	if err := h.ensureGuestEgress(); err != nil {
		return fmt.Errorf("guest egress policy: %w", err)
	}
	// S1a: isolation, anti-spoofing and host-service access. The "-o lo"
	// MASQUERADE rule was removed — the guest-originated packets it claimed
	// to catch do not exist (post-DNAT they carry the real tap IP, not
	// 127.0.0.1; 0-packet counters observed live).
	if err := h.ensureTap(tapName, guestIP); err != nil {
		return fmt.Errorf("tap firewall for %s: %w", tapName, err)
	}
	if err := h.ensureNAT(tapName, hostIP, guestIP, hostPort, guestPort); err != nil {
		return fmt.Errorf("tap nat for %s: %w", tapName, err)
	}
	return nil
}

// reconcileTapIsolation brings an already-live tap under the current rule
// set without a VM restart: for a reattached VM it derives the guest subnet
// from the bound host port and installs what setupHostNetworking installs on
// a fresh boot, removing the tap's older rule shapes (per-tap FORWARD DROPs
// and untagged ACCEPTs, service ACCEPTs without a source match) once the new
// rules are in. Called from ReattachVMWithConfig.
func (m *Manager) reconcileTapIsolation(vmID, hostURL string) {
	hostPort, ok := m.hostPortFromHostURL(hostURL)
	if !ok {
		log.Printf("vmmanager: warning: cannot derive guest subnet for VM %s from %q; tap firewall not reconciled", vmID, hostURL)
		return
	}
	guestIP, _ := m.guestAndHostIP(hostPort)
	tapName := tapNameForVMID(vmID)
	h := m.net()
	if err := h.ensureTap(tapName, guestIP); err != nil {
		log.Printf("vmmanager: warning: tap firewall for %s incomplete: %v", tapName, err)
	}
	// S1a: remove the retired guest->8085 DNAT rule from this tap. The port
	// no longer exists, but a stale DNAT widens the tap surface until boot.
	// Match the rendered text with the comment value unquoted (as iptables
	// -S prints it) or quoted.
	comment := tapComment(tapName)
	h.deleteMatching("nat", "PREROUTING", fmt.Sprintf("--dport 8085 -m comment --comment %s ", comment))
	h.deleteMatching("nat", "PREROUTING", fmt.Sprintf("--dport 8085 -m comment --comment %q", comment))
}

func tapReachableHostServicePorts() []string {
	return []string{
		"8082", // proxy / platform publish path
		"8083", // vmctl
		"8084", // gateway
		// :8085 removed 2026-10-04 (S1a): no production host autoputer serves
		// it; the rule only let guests reach a dead port.
		"8086", // corpusd durable Texture verification
		"8087", // maild draft persistence
		// Go module proxy for L2 capsules' goproxy.choir.internal route
		// (docs/reports/capsule-go-module-proxy-design-2026-10-14.md).
		"8088",
		// Package mirror for L2 capsules' mirror.choir.internal route
		// (station FP10, docs/definitions/choir-launch-fp10-net02-mirrors-2026-10-21.md).
		"8089",
	}
}

func tapNameForVMID(vmID string) string {
	// Linux interface names are limited to 15 visible bytes. Human-readable
	// prefixes collapse every long constructed ID (for example,
	// "candidate-control" and "candidate-fleet") onto the same TAP. Bind the
	// full stable VM identity into 60 bits while retaining the vm- namespace
	// used by stale-subnet reclamation and firewall cleanup.
	digest := sha256.Sum256([]byte(vmID))
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(digest[:])
	return "vm-" + strings.ToLower(encoded[:12])
}

// findBinary locates a binary by name, falling back to common NixOS paths.
func findBinary(name string, fallback string) string {
	// Try PATH lookup first.
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	// Try the fallback path.
	if _, err := os.Stat(fallback); err == nil {
		return fallback
	}
	// Try common NixOS system paths.
	for _, dir := range []string{"/run/current-system/sw/bin", "/usr/bin", "/bin"} {
		candidate := dir + "/" + name
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	// Last resort: just use the name and hope for the best.
	return name
}

// healthCheckLoop periodically checks the health of all running VMs.
func (m *Manager) healthCheckLoop() {
	defer close(m.healthDone)

	ticker := time.NewTicker(m.cfg.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.healthCancel:
			return
		case <-ticker.C:
			m.checkAllHealth()
		}
	}
}

// checkAllHealth checks the health of all running VMs.
func (m *Manager) checkAllHealth() {
	m.mu.RLock()
	var vmIDs []string
	for id, inst := range m.vms {
		if inst.State == StateRunning {
			vmIDs = append(vmIDs, id)
		}
	}
	m.mu.RUnlock()

	for _, id := range vmIDs {
		healthy, _ := m.CheckHealth(id)
		if !healthy {
			log.Printf("vmmanager: VM %s is unhealthy", id)
		}
	}
}

// tightenDataImageMode makes an existing data image owner-only (0600). A data
// image was created 0644 and never re-moded; the 0750 state directory above it
// is a barrier that a later group or ACL change would silently remove (review
// F12). It refuses anything that is not a regular file and never follows a
// link: the file is opened O_NOFOLLOW and the mode is set on the descriptor,
// so a swapped path cannot redirect it. Mode bits change ctime only, so the
// disk identity a memory snapshot is bound to (size, mtime, inode, device) is
// unchanged.
func tightenDataImageMode(path string) error {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return fmt.Errorf("open data image %s: %w", path, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("data image %s is not a regular file", path)
	}
	if info.Mode().Perm()&0o077 == 0 {
		return nil
	}
	if err := file.Chmod(0o600); err != nil {
		return fmt.Errorf("chmod data image %s: %w", path, err)
	}
	return nil
}
