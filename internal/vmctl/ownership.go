// Package vmctl implements the VM ownership registry and lifecycle control
// for Mission 3. The registry maps authenticated users to VM-backed sandbox
// workloads and ensures concurrent first requests for the same user collapse
// onto a single VM assignment (VAL-VM-004).
//
// Key invariants:
//   - Each authenticated user/desktop pair receives exactly one active
//     interactive VM at a time.
//   - Different users receive distinct VMs with isolated state (VAL-VM-005).
//   - Concurrent first requests for one user converge on one assignment (VAL-VM-004).
//   - VM control endpoints are internal-only (VAL-VM-012).
//   - Invalid auth is denied before VM or gateway side effects (VAL-CROSS-110).
//   - Idle/logout lifecycle transitions only the current user's VM (VAL-VM-008).
//   - vmctl detects unhealthy guests and recovers safely (VAL-VM-009).
//   - Guest VM files/env/process args remain free of provider credentials (VAL-VM-011).
//   - Crash recovery does not duplicate canonical effects (VAL-CROSS-117).
//   - Idle stop or hibernate resumes the same user's state (VAL-CROSS-116).
package vmctl

import (
	"context"
	"crypto/rand"
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
	"unicode"
)

// VMState represents the lifecycle state of a VM.
type VMState string

const (
	// VMStateBooting means the VM is being created or started.
	VMStateBooting VMState = "booting"

	// VMStateActive means the VM is running and healthy.
	VMStateActive VMState = "active"

	// VMStateDegraded means the VM is running but unhealthy.
	VMStateDegraded VMState = "degraded"

	// VMStateStopping means the VM is being shut down.
	VMStateStopping VMState = "stopping"

	// VMStateStopped means the VM has been stopped and is not running.
	// The VM can be resumed, restoring the same user's persisted state
	// (VAL-CROSS-116, VAL-VM-008).
	VMStateStopped VMState = "stopped"

	// VMStateHibernated means the VM's persistent state has been preserved
	// and the VM is not running. Resume restores the same user state.
	// The epoch does NOT increment on resume, so callers can distinguish
	// fresh boot from resume (VAL-CROSS-117).
	VMStateHibernated VMState = "hibernated"

	// VMStateFailed means the VM failed to start or has crashed.
	VMStateFailed VMState = "failed"

	// PrimaryDesktopID is the default desktop/workspace selector used when the
	// caller does not explicitly target a branch desktop.
	PrimaryDesktopID = "primary"
)

// VMKind distinguishes interactive desktop VMs from headless worker VMs.
type VMKind string

const (
	VMKindInteractive VMKind = "interactive"
	VMKindWorker      VMKind = "worker"
)

// VMOwnership represents the assignment of a user to a specific VM.
type VMOwnership struct {
	// VMID is the unique identifier for the VM.
	VMID string `json:"vm_id"`

	// UserID is the authenticated user who owns this VM.
	UserID string `json:"user_id"`

	// DesktopID is the desktop/workspace selector this interactive VM belongs to.
	// For worker VMs, this is the parent desktop selector the worker belongs to.
	DesktopID string `json:"desktop_id"`

	// Kind distinguishes interactive desktops from headless worker VMs.
	Kind VMKind `json:"kind,omitempty"`

	// ParentDesktopID records the source desktop when this desktop was forked
	// from another interactive desktop.
	ParentDesktopID string `json:"parent_desktop_id,omitempty"`

	// ParentVMID records the source VM whose persistent data image was used to
	// create this VM. It is empty for primary desktops and host-process fallback
	// forks that cannot materialize a separate disk image.
	ParentVMID string `json:"parent_vm_id,omitempty"`

	// SnapshotKind describes the fork materialization semantics. This makes
	// metadata-only copies visibly different from data-disk snapshots.
	SnapshotKind string `json:"snapshot_kind,omitempty"`

	// WorkerID is the typed handle identifier for worker VMs. Empty for
	// interactive desktop VMs.
	WorkerID string `json:"worker_id,omitempty"`

	// ParentAgentID is the durable super/agent identity that requested a worker.
	ParentAgentID string `json:"parent_agent_id,omitempty"`

	// TrajectoryID ties a worker request back to the user-visible workflow.
	TrajectoryID string `json:"trajectory_id,omitempty"`

	// Purpose is the caller-provided reason for this worker VM.
	Purpose string `json:"purpose,omitempty"`

	// ObjectiveFingerprint is a normalized objective identity used to collapse
	// accidental duplicate worker requests without hiding explicit portfolios.
	ObjectiveFingerprint string `json:"objective_fingerprint,omitempty"`

	// MachineClass is the requested resource envelope for this VM.
	MachineClass string `json:"machine_class,omitempty"`

	// WarmnessClass is the typed lifecycle policy class for keepalive and
	// reclaim decisions. Public health exposes only aggregate counts.
	WarmnessClass WarmnessClass `json:"warmness_class,omitempty"`

	// Published indicates whether this desktop is user-switchable through the
	// normal browser/proxy routing path. Background candidate desktops stay
	// unpublished until explicitly published by the control plane.
	Published bool `json:"published"`

	// SandboxURL is the URL where this VM's sandbox runtime is reachable.
	SandboxURL string `json:"sandbox_url"`

	// State is the current lifecycle state of the VM.
	State VMState `json:"state"`

	// CreatedAt is when the VM was first created.
	CreatedAt time.Time `json:"created_at"`

	// LastActiveAt is when the VM was last used.
	LastActiveAt time.Time `json:"last_active_at"`

	// SandboxCredential is the credential issued by the gateway for this VM.
	// It is used to authenticate sandbox-to-gateway provider requests.
	SandboxCredential string `json:"-"`

	// Epoch is the monotonically increasing boot counter for this VM.
	// On fresh boot or recovery, the epoch increments. On resume from
	// hibernate, the epoch stays the same. Callers can use epoch to
	// detect whether a VM went through a fresh boot vs. a resume,
	// which prevents duplicate canonical effects (VAL-CROSS-117).
	Epoch int64 `json:"epoch"`

	// StoppedBy indicates why the VM was stopped. Empty if running.
	// Valid values: "idle", "logout", "recovery", "manual".
	StoppedBy string `json:"stopped_by,omitempty"`
}

// WorkerRequest is the typed internal vmctl request for a background worker VM.
type WorkerRequest struct {
	UserID               string `json:"user_id"`
	DesktopID            string `json:"desktop_id,omitempty"`
	ParentAgentID        string `json:"parent_agent_id"`
	TrajectoryID         string `json:"trajectory_id,omitempty"`
	Purpose              string `json:"purpose"`
	ObjectiveFingerprint string `json:"objective_fingerprint,omitempty"`
	MachineClass         string `json:"machine_class,omitempty"`
	AllowParallel        bool   `json:"allow_parallel,omitempty"`
}

// WorkerVMHandle is the typed result returned when vmctl provisions a worker VM.
type WorkerVMHandle struct {
	Kind                 VMKind  `json:"kind"`
	WorkerID             string  `json:"worker_id"`
	VMID                 string  `json:"vm_id"`
	UserID               string  `json:"user_id"`
	DesktopID            string  `json:"desktop_id"`
	ParentAgentID        string  `json:"parent_agent_id,omitempty"`
	TrajectoryID         string  `json:"trajectory_id,omitempty"`
	Purpose              string  `json:"purpose"`
	ObjectiveFingerprint string  `json:"objective_fingerprint,omitempty"`
	MachineClass         string  `json:"machine_class"`
	SandboxURL           string  `json:"sandbox_url"`
	State                VMState `json:"state"`
}

// IsReady returns true if the VM is in a state that can serve routed requests.
func (o *VMOwnership) IsReady() bool {
	return o.State == VMStateActive
}

// VMManager is the interface the OwnershipRegistry uses to manage real
// Firecracker VM lifecycles. When Firecracker is available on the host,
// the registry delegates VM boot/stop/resume/recover operations to the
// concrete vmmanager.Manager. When Firecracker is not available, the
// registry runs in host-process mode with no-op VM lifecycle calls.
type VMManager interface {
	// BootVM launches a new Firecracker VM and returns its instance info.
	BootVM(cfg VMManagerConfig) (*VMInstanceInfo, error)

	// StopVM cleanly stops a running VM.
	StopVM(vmID string) error

	// HibernateVM saves VM state and stops it (persistent data preserved).
	HibernateVM(vmID string) error

	// ResumeVM resumes a stopped or hibernated VM (same epoch, same state).
	ResumeVM(vmID string) (*VMInstanceInfo, error)

	// ReattachVM adopts a VM process that survived vmctl restart.
	ReattachVM(vmID, hostURL string, epoch int64) (*VMInstanceInfo, error)

	// RecoverVM force-kills and reboots a failed VM (new epoch), merging any
	// current ownership-derived config into persisted VM launch config.
	RecoverVM(vmID string, cfg VMManagerConfig) (*VMInstanceInfo, error)

	// RefreshVM force-kills and reboots a VM onto the current deploy's
	// default boot artifacts while preserving mutable state.
	RefreshVM(vmID string, cfg VMManagerConfig) (*VMInstanceInfo, error)

	// DestroyVMState deletes stopped terminal VM state. The registry calls this
	// only after policy checks exclude primary/published/active computers.
	DestroyVMState(vmID string) error

	// GetVM returns the VM instance info, or nil if not found.
	GetVM(vmID string) *VMInstanceInfo

	// CheckHealth probes the VM's guest health endpoint.
	CheckHealth(vmID string) (bool, error)
}

type vmGatewayTokenReader interface {
	ReadGatewayToken(vmID string) (string, error)
}

// VMManagerConfig holds the configuration for launching a single VM,
// mirroring the vmmanager.VMConfig fields that the registry controls.
type VMManagerConfig struct {
	VMID              string
	KernelImagePath   string
	InitrdPath        string
	RootfsPath        string
	StoreDiskPath     string
	KernelParams      string
	GuestPort         int
	MachineCPUCount   int
	MachineMemSizeMib int
	PersistentDir     string
	SourceVMID        string
	// GatewayToken is the credential token for the sandbox to authenticate
	// to the host-side gateway. Written to the persistent directory so the
	// guest init script can read it and set RUNTIME_GATEWAY_TOKEN.
	GatewayToken string
	ComputerKind string
	OwnerID      string
	DesktopID    string
	WorkerID     string
	CandidateID  string
}

// VMImageProfile points a VM boot at a non-default guest image. Ordinary
// workers use the manager defaults; evidence/verifier classes such as
// worker-playwright use explicit profile paths so their heavy browser closure
// does not leak into every user/candidate VM.
type VMImageProfile struct {
	KernelImagePath string
	InitrdPath      string
	RootfsPath      string
	StoreDiskPath   string
	KernelParams    string
}

// MissingRequiredFields returns required boot artifact fields that are absent
// from a non-default image profile.
func (p VMImageProfile) MissingRequiredFields() []string {
	var missing []string
	if strings.TrimSpace(p.KernelImagePath) == "" {
		missing = append(missing, "kernel_image")
	}
	if strings.TrimSpace(p.InitrdPath) == "" {
		missing = append(missing, "initrd")
	}
	if strings.TrimSpace(p.RootfsPath) == "" {
		missing = append(missing, "rootfs")
	}
	if strings.TrimSpace(p.StoreDiskPath) == "" {
		missing = append(missing, "store_disk")
	}
	if strings.TrimSpace(p.KernelParams) == "" {
		missing = append(missing, "kernel_params")
	}
	return missing
}

// VMInstanceInfo holds the information returned by the VM manager
// after a VM lifecycle operation.
type VMInstanceInfo struct {
	HostURL         string
	Epoch           int64
	Healthy         bool
	State           string
	StartedAt       time.Time
	LastHealthCheck time.Time
	LastHealthyAt   time.Time
}

// OwnershipRegistry manages the mapping of users to VMs. It provides
// thread-safe VM assignment with singleflight semantics so that concurrent
// first requests for the same user collapse onto one VM assignment
// (VAL-VM-004).
//
// The registry also manages lifecycle transitions:
//   - Idle timeout: VMs idle beyond a configurable threshold transition
//     to stopped/hibernated state (VAL-VM-008, VAL-CROSS-116).
//   - Logout teardown: removing ownership on logout (VAL-VM-008).
//   - Unhealthy recovery: detecting and recovering failed VMs (VAL-VM-009).
//   - Crash dedup: epoch tracking prevents duplicate canonical effects
//     across recovery (VAL-CROSS-117).
type OwnershipRegistry struct {
	mu sync.RWMutex

	// ownerships maps user/desktop composite keys to their active VM ownership.
	ownerships map[string]*VMOwnership

	// workerVMs maps typed worker handles to their active headless child VMs.
	workerVMs map[string]*VMOwnership

	// vmByID maps VM ID to ownership for reverse lookup.
	vmByID map[string]*VMOwnership

	// pendingWaiters maps user/desktop composite keys to channels that concurrent
	// callers wait on when a VM assignment is already in progress. This
	// collapses concurrent first requests (VAL-VM-004).
	pendingWaiters map[string][]chan *VMOwnership

	// gatewayCredentialNextCheck throttles host-side token reconciliation for
	// already-running VMs. It avoids an internal gateway round trip on every
	// proxied browser request while still retrying quickly after transient
	// failures.
	gatewayCredentialNextCheck map[string]time.Time

	// sandboxURLBase is the base URL pattern for sandbox runtimes.
	// The VM ID is appended as a path component: base + "/" + vmID
	sandboxURLBase string

	// idleTimeout is the duration after which a VM with no activity
	// is eligible for stop/hibernate. Zero means no idle timeout.
	idleTimeout time.Duration

	// pressureReclaim controls pressure-aware dry-run lifecycle observation.
	// It ranks reclaim candidates from measured host pressure without changing
	// VM state until a later mission explicitly enables active reclaim.
	pressureReclaim PressureReclaimConfig
	pressureSampler hostPressureSampler

	// retentionPrune controls durable VM-state deletion for explicitly
	// disposable computers such as staging Playwright accounts.
	retentionPrune      RetentionPruneConfig
	retentionUserEmails map[string]string

	// warmnessPolicy controls under-capacity primary keepalive and future
	// always-on tier modeling.
	warmnessPolicy WarmnessPolicyConfig

	// epochCounter tracks the global epoch counter for VM boot tracking.
	// Each fresh boot or recovery increments this counter, providing a
	// mechanism to prevent duplicate canonical effects (VAL-CROSS-117).
	epochCounter int64

	// vmManager is the optional Firecracker VM lifecycle manager.
	// When nil, the registry operates in host-process sandbox mode where
	// all VMs share the same sandbox URL. When set, the registry delegates
	// VM lifecycle operations to this manager for real Firecracker VMs.
	vmManager VMManager

	// workerImageProfiles maps worker machine classes to alternate guest image
	// artifacts. This keeps heavyweight evidence workers (for example
	// worker-playwright) out of the default VM image.
	workerImageProfiles map[string]VMImageProfile

	// gatewayURL is the URL of the host-side gateway service. When set,
	// the registry issues gateway tokens for VM sandboxes before booting
	// so the guest sandbox can authenticate to the gateway.
	gatewayURL string

	// persistencePath stores ownership metadata across vmctl restarts. The
	// Firecracker data disks live under the VM manager state dir; this file is
	// the durable routing index that lets vmctl reattach to those disks.
	persistencePath string
}

// NewOwnershipRegistry creates a new ownership registry.
// The idleTimeout parameter configures automatic VM stop after inactivity.
// Zero means no idle timeout (VMs stay active indefinitely).
func NewOwnershipRegistry(sandboxURLBase string) *OwnershipRegistry {
	if sandboxURLBase == "" {
		sandboxURLBase = "http://127.0.0.1:8085"
	}
	return &OwnershipRegistry{
		ownerships:                 make(map[string]*VMOwnership),
		workerVMs:                  make(map[string]*VMOwnership),
		vmByID:                     make(map[string]*VMOwnership),
		pendingWaiters:             make(map[string][]chan *VMOwnership),
		gatewayCredentialNextCheck: make(map[string]time.Time),
		sandboxURLBase:             sandboxURLBase,
		idleTimeout:                0, // no idle timeout by default
		pressureReclaim:            DefaultPressureReclaimConfig(),
		pressureSampler:            sampleHostPressure,
		retentionPrune:             DefaultRetentionPruneConfig(),
		retentionUserEmails:        make(map[string]string),
		warmnessPolicy:             DefaultWarmnessPolicyConfig(),
		workerImageProfiles:        make(map[string]VMImageProfile),
		epochCounter:               1,
	}
}

type persistedOwnershipState struct {
	SavedAt      time.Time      `json:"saved_at"`
	EpochCounter int64          `json:"epoch_counter"`
	Ownerships   []*VMOwnership `json:"ownerships"`
}

// SetPersistencePath enables durable ownership metadata. Existing metadata is
// loaded immediately. Running/booting states from a previous vmctl process are
// loaded as stopped so the next resolve performs a controlled fresh boot of the
// same VM ID and data disk instead of returning a stale sandbox URL.
func (r *OwnershipRegistry) SetPersistencePath(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.persistencePath = path
	if err := r.loadLocked(); err != nil {
		return err
	}
	return nil
}

func (r *OwnershipRegistry) loadLocked() error {
	if r.persistencePath == "" {
		return nil
	}
	data, err := os.ReadFile(r.persistencePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("load ownership registry %s: %w", r.persistencePath, err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil
	}
	var state persistedOwnershipState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("decode ownership registry %s: %w", r.persistencePath, err)
	}

	r.ownerships = make(map[string]*VMOwnership)
	r.workerVMs = make(map[string]*VMOwnership)
	r.vmByID = make(map[string]*VMOwnership)
	r.pendingWaiters = make(map[string][]chan *VMOwnership)
	maxEpoch := r.epochCounter
	for _, loaded := range state.Ownerships {
		if loaded == nil || strings.TrimSpace(loaded.VMID) == "" || strings.TrimSpace(loaded.UserID) == "" {
			continue
		}
		own := *loaded
		own.UserID = strings.TrimSpace(own.UserID)
		own.VMID = strings.TrimSpace(own.VMID)
		own.DesktopID = normalizeDesktopID(own.DesktopID)
		if own.Kind == "" {
			own.Kind = VMKindInteractive
		}
		if own.State == "" || own.State == VMStateBooting || own.State == VMStateActive || own.State == VMStateDegraded || own.State == VMStateStopping {
			own.State = VMStateStopped
			if own.StoppedBy == "" {
				own.StoppedBy = "vmctl-restart"
			}
		}
		if own.CreatedAt.IsZero() {
			own.CreatedAt = time.Now()
		}
		if own.LastActiveAt.IsZero() {
			own.LastActiveAt = own.CreatedAt
		}
		if own.Epoch > maxEpoch {
			maxEpoch = own.Epoch
		}
		ptr := &own
		if own.Kind == VMKindWorker && strings.TrimSpace(own.WorkerID) != "" {
			r.workerVMs[own.WorkerID] = ptr
		} else {
			r.ownerships[ownershipKey(own.UserID, own.DesktopID)] = ptr
		}
		r.vmByID[own.VMID] = ptr
	}
	if state.EpochCounter > maxEpoch {
		maxEpoch = state.EpochCounter
	}
	r.epochCounter = maxEpoch
	log.Printf("vmctl: loaded %d persisted ownership(s) from %s", len(r.vmByID), r.persistencePath)
	return nil
}

func (r *OwnershipRegistry) saveLocked() {
	if r.persistencePath == "" {
		return
	}
	ownerships := make([]*VMOwnership, 0, len(r.ownerships)+len(r.workerVMs))
	for _, own := range r.ownerships {
		cp := *own
		ownerships = append(ownerships, &cp)
	}
	for _, own := range r.workerVMs {
		cp := *own
		ownerships = append(ownerships, &cp)
	}
	sort.Slice(ownerships, func(i, j int) bool {
		a, b := ownerships[i], ownerships[j]
		ak := string(a.Kind) + "|" + a.UserID + "|" + a.DesktopID + "|" + a.WorkerID + "|" + a.VMID
		bk := string(b.Kind) + "|" + b.UserID + "|" + b.DesktopID + "|" + b.WorkerID + "|" + b.VMID
		return ak < bk
	})
	state := persistedOwnershipState{
		SavedAt:      time.Now().UTC(),
		EpochCounter: r.epochCounter,
		Ownerships:   ownerships,
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		log.Printf("vmctl: persist ownership registry: marshal: %v", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(r.persistencePath), 0o750); err != nil {
		log.Printf("vmctl: persist ownership registry: mkdir: %v", err)
		return
	}
	tmp := r.persistencePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o640); err != nil {
		log.Printf("vmctl: persist ownership registry: write: %v", err)
		return
	}
	if err := os.Rename(tmp, r.persistencePath); err != nil {
		log.Printf("vmctl: persist ownership registry: rename: %v", err)
	}
}

// SetVMManager sets the Firecracker VM lifecycle manager. When set, the
// registry delegates VM lifecycle operations to the manager instead of
// running in host-process sandbox mode. This activates real Firecracker
// VM lifecycle on Node B.
func (r *OwnershipRegistry) SetVMManager(mgr VMManager) {
	r.mu.Lock()
	r.vmManager = mgr
	candidates := make([]*VMOwnership, 0)
	if mgr != nil {
		for _, own := range r.ownerships {
			if own.State == VMStateStopped && own.StoppedBy == "vmctl-restart" && strings.TrimSpace(own.SandboxURL) != "" {
				candidates = append(candidates, own)
			}
		}
		for _, own := range r.workerVMs {
			if own.State == VMStateStopped && own.StoppedBy == "vmctl-restart" && strings.TrimSpace(own.SandboxURL) != "" {
				candidates = append(candidates, own)
			}
		}
	}
	r.mu.Unlock()

	reattached := false
	for _, own := range candidates {
		vmID := own.VMID
		hostURL := own.SandboxURL
		epoch := own.Epoch
		info, err := mgr.ReattachVM(vmID, hostURL, epoch)
		if err != nil {
			log.Printf("vmctl: reattach skipped for VM %s: %v", vmID, err)
			continue
		}
		r.mu.Lock()
		if cur, ok := r.vmByID[vmID]; ok {
			cur.State = VMStateActive
			cur.SandboxURL = info.HostURL
			cur.Epoch = info.Epoch
			if cur.LastActiveAt.IsZero() {
				cur.LastActiveAt = time.Now()
			}
			cur.StoppedBy = ""
			r.saveLocked()
			reattached = true
		}
		r.mu.Unlock()
	}
	if reattached {
		go r.ReconcileReadyGatewayCredentials()
	}
}

// SetWorkerImageProfile registers an alternate guest image for a worker
// machine class. It is intended for bounded evidence/verifier classes such as
// worker-playwright, not for ordinary user or candidate computers.
func (r *OwnershipRegistry) SetWorkerImageProfile(machineClass string, profile VMImageProfile) {
	machineClass = strings.ToLower(strings.TrimSpace(machineClass))
	if machineClass == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if strings.TrimSpace(profile.KernelImagePath) == "" &&
		strings.TrimSpace(profile.InitrdPath) == "" &&
		strings.TrimSpace(profile.RootfsPath) == "" &&
		strings.TrimSpace(profile.StoreDiskPath) == "" &&
		strings.TrimSpace(profile.KernelParams) == "" {
		delete(r.workerImageProfiles, machineClass)
		return
	}
	r.workerImageProfiles[machineClass] = profile
}

// SetGatewayURL configures the gateway URL for issuing sandbox tokens.
// When set, the registry will issue a gateway token for each VM before
// booting so the guest sandbox can authenticate to the gateway.
func (r *OwnershipRegistry) SetGatewayURL(url string) {
	r.mu.Lock()
	r.gatewayURL = url
	vmIDs := r.readyVMIDsLocked()
	r.mu.Unlock()
	if len(vmIDs) > 0 {
		go r.reconcileGatewayCredentialsForVMs(vmIDs)
	}
}

// SetIdleTimeout configures the idle timeout for automatic VM lifecycle
// management. After this duration of inactivity, VMs are eligible for
// stop/hibernate (VAL-VM-008, VAL-CROSS-116).
func (r *OwnershipRegistry) SetIdleTimeout(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.idleTimeout = d
}

// SetPressureReclaimConfig configures pressure-aware lifecycle behavior.
// Dry-run mode only observes and ranks candidates. Active mode hibernates a
// bounded number of eligible idle candidates when the host is under pressure.
func (r *OwnershipRegistry) SetPressureReclaimConfig(cfg PressureReclaimConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pressureReclaim = normalizePressureReclaimConfig(cfg)
}

// SetRetentionPruneConfig configures deletion of explicitly disposable VM
// state. Real user primary computers remain protected unless the configured
// classifier marks the owner as ephemeral.
func (r *OwnershipRegistry) SetRetentionPruneConfig(cfg RetentionPruneConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.retentionPrune = normalizeRetentionPruneConfig(cfg)
}

func (r *OwnershipRegistry) setRetentionUserEmailsForTest(emails map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.retentionUserEmails = make(map[string]string, len(emails))
	for userID, email := range emails {
		r.retentionUserEmails[strings.TrimSpace(userID)] = strings.TrimSpace(email)
	}
}

// SetWarmnessPolicyConfig configures adaptive keepalive policy. It currently
// controls whether primary computers stay warm while the host is under
// configured pressure thresholds, and models a future always-on class.
func (r *OwnershipRegistry) SetWarmnessPolicyConfig(cfg WarmnessPolicyConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.warmnessPolicy = normalizeWarmnessPolicyConfig(cfg)
}

func (r *OwnershipRegistry) setPressureSamplerForTest(sampler hostPressureSampler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pressureSampler = sampler
}

// StartIdleSweeper periodically hibernates idle active VMs. It schedules an
// immediate background sweep so vmctl can bind its health/control port before
// potentially slow retention pruning walks old VM state directories.
func (r *OwnershipRegistry) StartIdleSweeper(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	var sweepMu sync.Mutex
	sweep := func() {
		sweepMu.Lock()
		defer sweepMu.Unlock()
		if warmed := r.WarmAlwaysOnDesktops(); warmed > 0 {
			log.Printf("vmctl: warmness policy resumed %d always-on desktop VM(s)", warmed)
		}
		if warmed := r.WarmCommunityWirePlatformComputer(); warmed > 0 {
			log.Printf("vmctl: warmness policy resumed %d community wire platform computer(s)", warmed)
		}
		if plan := r.PressureReclaimPlan(); plan.Mode == PressureReclaimModeDryRun {
			log.Printf("vmctl: pressure reclaim dry-run decision=%s reason=%q active=%d eligible=%d protected=%d pressure=%v",
				plan.Decision, plan.Reason, plan.Inventory.Active, plan.Inventory.Eligible, plan.Inventory.Protected, plan.Pressure.Pressure)
		} else if plan.Mode == PressureReclaimModeActive {
			log.Printf("vmctl: pressure reclaim active decision=%s reason=%q active=%d eligible=%d protected=%d pressure=%v",
				plan.Decision, plan.Reason, plan.Inventory.Active, plan.Inventory.Eligible, plan.Inventory.Protected, plan.Pressure.Pressure)
			if reclaimed := r.ReclaimPressureVMs(); reclaimed > 0 {
				log.Printf("vmctl: pressure reclaim hibernated %d VM(s)", reclaimed)
			}
			if destroyed := r.ReclaimStaleVMState(); destroyed > 0 {
				log.Printf("vmctl: pressure reclaim destroyed %d stale worker/candidate VM state directories", destroyed)
			}
		}
		if result := r.PruneRetention(); result.Deleted > 0 {
			log.Printf("vmctl: retention prune deleted %d VM state directorie(s), reclaimed %.1f MiB", result.Deleted, float64(result.BytesDeleted)/(1024*1024))
		}
		if stopped := r.StopIdleVMs(); stopped > 0 {
			log.Printf("vmctl: idle sweeper hibernated %d VM(s)", stopped)
		}
	}

	go func() {
		sweep()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sweep()
			}
		}
	}()
}

// nextEpoch returns the next epoch value and increments the counter.
// This is used to track VM boot/recovery generations for crash dedup
// (VAL-CROSS-117).
func (r *OwnershipRegistry) nextEpoch() int64 {
	r.epochCounter++
	return r.epochCounter
}

func normalizeDesktopID(desktopID string) string {
	desktopID = strings.TrimSpace(desktopID)
	if desktopID == "" {
		return PrimaryDesktopID
	}
	return desktopID
}

func ownershipKey(userID, desktopID string) string {
	return strings.TrimSpace(userID) + "|" + normalizeDesktopID(desktopID)
}

func workerHandleFromOwnership(own *VMOwnership) *WorkerVMHandle {
	if own == nil {
		return nil
	}
	return &WorkerVMHandle{
		Kind:                 VMKindWorker,
		WorkerID:             own.WorkerID,
		VMID:                 own.VMID,
		UserID:               own.UserID,
		DesktopID:            own.DesktopID,
		ParentAgentID:        own.ParentAgentID,
		TrajectoryID:         own.TrajectoryID,
		Purpose:              own.Purpose,
		ObjectiveFingerprint: workerObjectiveFingerprintForOwnership(own),
		MachineClass:         own.MachineClass,
		SandboxURL:           own.SandboxURL,
		State:                own.State,
	}
}

func workerObjectiveFingerprintForOwnership(own *VMOwnership) string {
	if own == nil {
		return ""
	}
	if strings.TrimSpace(own.ObjectiveFingerprint) != "" {
		return strings.TrimSpace(own.ObjectiveFingerprint)
	}
	return workerObjectiveFingerprint(own.UserID, own.DesktopID, own.ParentAgentID, own.TrajectoryID, own.Purpose)
}

func workerObjectiveFingerprint(userID, desktopID, parentAgentID, trajectoryID, purpose string) string {
	parts := []string{
		strings.TrimSpace(userID),
		normalizeDesktopID(desktopID),
		strings.TrimSpace(parentAgentID),
		strings.TrimSpace(trajectoryID),
		normalizeObjectiveText(purpose),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

func normalizeObjectiveText(raw string) string {
	var b strings.Builder
	lastSpace := false
	for _, r := range strings.ToLower(strings.TrimSpace(raw)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace && b.Len() > 0 {
			b.WriteByte(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

func normalizeWorkerMachineClass(raw string) (string, int, int, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "default", "standard", "worker", "worker-standard", "worker-default", "worker-small", "small":
		return "worker-small", 1, 1024, nil
	case "worker-medium", "medium":
		return "worker-medium", 2, 4096, nil
	case "worker-large", "large":
		return "worker-large", 4, 8192, nil
	case "worker-playwright", "playwright", "evidence", "evidence-browser", "verifier-browser":
		return "worker-playwright", 4, 8192, nil
	default:
		return "", 0, 0, fmt.Errorf("unsupported machine_class %q", strings.TrimSpace(raw))
	}
}

func workerMachineClassRequiresImageProfile(machineClass string) bool {
	return strings.TrimSpace(machineClass) == "worker-playwright"
}

func machineShapeForOwnership(own *VMOwnership) (int, int) {
	if own != nil && own.Kind == VMKindWorker {
		_, cpu, mem, err := normalizeWorkerMachineClass(own.MachineClass)
		if err == nil {
			return cpu, mem
		}
	}
	return interactiveVMCPUCount, interactiveVMMemSizeMib
}

func computerKindForOwnership(own *VMOwnership) string {
	if own == nil {
		return "active"
	}
	if own.Kind == VMKindWorker {
		return "worker"
	}
	if own.WarmnessClass == WarmnessClassPublicPlatform ||
		(own.UserID == CommunityWirePlatformOwnerID && normalizeDesktopID(own.DesktopID) == CommunityWirePlatformDesktopID) {
		return "platform"
	}
	if own.ParentVMID != "" || own.ParentDesktopID != "" || normalizeDesktopID(own.DesktopID) != PrimaryDesktopID || own.WarmnessClass == WarmnessClassCandidate {
		return "candidate"
	}
	return "active"
}

func candidateIDForOwnership(own *VMOwnership) string {
	if own == nil {
		return ""
	}
	if own.Kind == VMKindWorker {
		return strings.TrimSpace(own.WorkerID)
	}
	if computerKindForOwnership(own) == "candidate" {
		return normalizeDesktopID(own.DesktopID)
	}
	return ""
}

func activeOwnershipNeedsReadinessCheck(own *VMOwnership, mgr VMManager) bool {
	if own == nil || mgr == nil {
		return false
	}
	if !vmInstanceInfoReady(mgr.GetVM(own.VMID)) {
		return true
	}
	if own.LastActiveAt.IsZero() {
		return true
	}
	return time.Since(own.LastActiveAt) >= activeResolveReadinessCheckInterval
}

func vmInstanceInfoReady(info *VMInstanceInfo) bool {
	if info == nil || strings.TrimSpace(info.HostURL) == "" || !info.Healthy {
		return false
	}
	state := strings.ToLower(strings.TrimSpace(info.State))
	return state == "" || state == "running"
}

func activeVMCanRouteDuringHealthGrace(info *VMInstanceInfo, now time.Time) bool {
	if info == nil || strings.TrimSpace(info.HostURL) == "" {
		return false
	}
	state := strings.ToLower(strings.TrimSpace(info.State))
	switch state {
	case "pending":
		started := info.StartedAt
		return !started.IsZero() && now.Sub(started) <= activeResolvePendingRouteGrace
	case "", "running":
		healthyAt := info.LastHealthyAt
		if healthyAt.IsZero() {
			healthyAt = info.StartedAt
		}
		return !healthyAt.IsZero() && now.Sub(healthyAt) <= activeResolveUnhealthyRouteGrace
	default:
		return false
	}
}

func (r *OwnershipRegistry) ensureActiveVMReady(own *VMOwnership, mgr VMManager) (*VMInstanceInfo, error) {
	if own == nil {
		return nil, fmt.Errorf("ownership is required")
	}
	if mgr == nil {
		return &VMInstanceInfo{
			HostURL: own.SandboxURL,
			Epoch:   own.Epoch,
			Healthy: true,
			State:   "running",
		}, nil
	}

	info := mgr.GetVM(own.VMID)
	if info == nil {
		log.Printf("vmctl: active ownership for VM %s has no manager instance; starting existing VM", own.VMID)
		return r.startExistingVM(own, mgr)
	}

	state := strings.ToLower(strings.TrimSpace(info.State))
	switch state {
	case "stopped", "hibernated":
		log.Printf("vmctl: active ownership for VM %s found manager state=%s; resuming", own.VMID, state)
		return r.startExistingVM(own, mgr)
	case "pending":
		if activeVMCanRouteDuringHealthGrace(info, time.Now()) {
			log.Printf("vmctl: active ownership for VM %s found manager state=pending; preserving in-flight boot", own.VMID)
			return info, nil
		}
		log.Printf("vmctl: active ownership for VM %s found stale manager state=pending; recovering before routing", own.VMID)
		return r.recoverOrRestartActiveVM(own, mgr)
	case "", "running":
		healthy, err := mgr.CheckHealth(own.VMID)
		if err != nil {
			log.Printf("vmctl: active VM %s health probe errored; recovering before routing: %v", own.VMID, err)
			return r.recoverOrRestartActiveVM(own, mgr)
		}
		if healthy {
			if refreshed := mgr.GetVM(own.VMID); refreshed != nil {
				return refreshed, nil
			}
			return info, nil
		}
		refreshed := mgr.GetVM(own.VMID)
		if activeVMCanRouteDuringHealthGrace(refreshed, time.Now()) {
			log.Printf("vmctl: active VM %s health check failed; preserving route within transient health grace", own.VMID)
			return refreshed, nil
		}
		log.Printf("vmctl: active VM %s is unhealthy on resolve; recovering before routing", own.VMID)
		return r.recoverOrRestartActiveVM(own, mgr)
	default:
		log.Printf("vmctl: active ownership for VM %s found manager state=%s; recovering before routing", own.VMID, state)
		return r.recoverOrRestartActiveVM(own, mgr)
	}
}

func (r *OwnershipRegistry) recoverOrRestartActiveVM(own *VMOwnership, mgr VMManager) (*VMInstanceInfo, error) {
	if mgr.GetVM(own.VMID) == nil {
		return r.startExistingVM(own, mgr)
	}
	recovered, err := mgr.RecoverVM(own.VMID, vmManagerConfigForOwnership(own, ""))
	if err != nil {
		if mgr.GetVM(own.VMID) == nil {
			return r.startExistingVM(own, mgr)
		}
		return nil, err
	}
	return recovered, nil
}

func (r *OwnershipRegistry) startExistingVM(own *VMOwnership, mgr VMManager) (*VMInstanceInfo, error) {
	if own == nil || mgr == nil {
		return nil, nil
	}
	if info, err := mgr.ResumeVM(own.VMID); err == nil {
		return info, nil
	} else if existing := mgr.GetVM(own.VMID); existing != nil {
		state := strings.ToLower(strings.TrimSpace(existing.State))
		if state == "failed" || state == "pending" {
			recovered, recoverErr := mgr.RecoverVM(own.VMID, vmManagerConfigForOwnership(own, ""))
			if recoverErr == nil {
				return recovered, nil
			}
			return nil, fmt.Errorf("resume existing VM %s failed: %w; recovery also failed: %v", own.VMID, err, recoverErr)
		}
		return nil, err
	}
	return mgr.BootVM(vmManagerConfigForOwnership(own, r.issueGatewayToken(own.VMID)))
}

func vmManagerConfigForOwnership(own *VMOwnership, gatewayToken string) VMManagerConfig {
	if own == nil {
		return VMManagerConfig{}
	}
	cpu, mem := machineShapeForOwnership(own)
	return VMManagerConfig{
		VMID:              own.VMID,
		GuestPort:         8085,
		MachineCPUCount:   cpu,
		MachineMemSizeMib: mem,
		GatewayToken:      gatewayToken,
		ComputerKind:      computerKindForOwnership(own),
		OwnerID:           own.UserID,
		DesktopID:         own.DesktopID,
		WorkerID:          own.WorkerID,
		CandidateID:       candidateIDForOwnership(own),
	}
}

// issueGatewayToken requests a gateway credential token for the given
// sandbox ID by calling the gateway's credential issuance endpoint.
// Returns the raw token string or an empty string on failure.
// Failures are logged but not fatal — the VM will still boot but
// won't be able to authenticate to the gateway until a token is provided.
func (r *OwnershipRegistry) issueGatewayToken(sandboxID string) string {
	r.mu.RLock()
	gwURL := r.gatewayURL
	r.mu.RUnlock()
	return issueGatewayTokenAt(gwURL, sandboxID)
}

func issueGatewayTokenAt(gwURL, sandboxID string) string {
	if gwURL == "" {
		return ""
	}

	// Call the gateway's credential issuance endpoint.
	// This is the same endpoint used by the host sandbox's ExecStartPre.
	body := fmt.Sprintf(`{"sandbox_id":"%s"}`, sandboxID)
	url := strings.TrimRight(gwURL, "/") + "/provider/v1/credentials/issue"

	ctx, cancel := context.WithTimeout(context.Background(), gatewayCredentialRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		log.Printf("vmctl: gateway token request creation failed for %s: %v", sandboxID, err)
		return ""
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("vmctl: gateway token request failed for %s: %v", sandboxID, err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		log.Printf("vmctl: gateway token issue returned %d for %s", resp.StatusCode, sandboxID)
		return ""
	}

	var result struct {
		SandboxID       string `json:"sandbox_id"`
		SandboxIDCompat string `json:"SandboxID"`
		RawToken        string `json:"raw_token"`
		RawTokenCompat  string `json:"RawToken"`
		ExpiresAt       string `json:"expires_at"`
		ExpiresAtCompat string `json:"ExpiresAt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("vmctl: gateway token response decode failed: %v", err)
		return ""
	}
	if result.RawToken == "" {
		result.RawToken = result.RawTokenCompat
	}

	return result.RawToken
}

const (
	gatewayCredentialRequestTimeout        = 5 * time.Second
	gatewayCredentialEnsureSuccessInterval = 10 * time.Minute
	gatewayCredentialEnsureFailureInterval = 30 * time.Second
	activeResolveReadinessCheckInterval    = 10 * time.Second
	activeResolveUnhealthyRouteGrace       = 45 * time.Second
	activeResolvePendingRouteGrace         = 3 * time.Minute
	interactiveVMCPUCount                  = 2
	interactiveVMMemSizeMib                = 2048
)

func (r *OwnershipRegistry) ensureExistingGatewayCredential(vmID string) {
	vmID = strings.TrimSpace(vmID)
	if vmID == "" {
		return
	}
	r.mu.Lock()
	now := time.Now()
	gwURL := strings.TrimSpace(r.gatewayURL)
	if gwURL == "" {
		r.mu.Unlock()
		return
	}
	mgr := r.vmManager
	reader, ok := mgr.(vmGatewayTokenReader)
	if !ok {
		r.mu.Unlock()
		return
	}
	if nextCheck, ok := r.gatewayCredentialNextCheck[vmID]; ok && now.Before(nextCheck) {
		r.mu.Unlock()
		return
	}
	// Suppress request stampedes while still allowing quick retry if the
	// gateway is temporarily unavailable or starting during deploy.
	r.gatewayCredentialNextCheck[vmID] = now.Add(gatewayCredentialEnsureFailureInterval)
	r.mu.Unlock()

	rawToken, err := reader.ReadGatewayToken(vmID)
	if err != nil {
		log.Printf("vmctl: gateway credential ensure skipped for VM %s: %v", vmID, err)
		return
	}
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		log.Printf("vmctl: gateway credential ensure skipped for VM %s: empty token", vmID)
		return
	}
	bodyBytes, err := json.Marshal(map[string]string{"raw_token": rawToken})
	if err != nil {
		log.Printf("vmctl: gateway credential ensure marshal failed for VM %s: %v", vmID, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), gatewayCredentialRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(gwURL, "/")+"/provider/v1/credentials/ensure", strings.NewReader(string(bodyBytes)))
	if err != nil {
		log.Printf("vmctl: gateway credential ensure request creation failed for VM %s: %v", vmID, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("vmctl: gateway credential ensure failed for VM %s: %v", vmID, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		log.Printf("vmctl: gateway credential ensure returned %d for VM %s: %s", resp.StatusCode, vmID, strings.TrimSpace(string(detail)))
		return
	}
	r.mu.Lock()
	r.gatewayCredentialNextCheck[vmID] = time.Now().Add(gatewayCredentialEnsureSuccessInterval)
	r.mu.Unlock()
}

// ReconcileReadyGatewayCredentials imports host-held gateway credentials for
// all currently active VMs. This is the deploy/restart safety net: vmctl can
// reattach Firecracker processes that kept running while the gateway restarted
// with an empty in-memory credential registry.
func (r *OwnershipRegistry) ReconcileReadyGatewayCredentials() int {
	r.mu.RLock()
	vmIDs := r.readyVMIDsLocked()
	r.mu.RUnlock()
	r.reconcileGatewayCredentialsForVMs(vmIDs)
	return len(vmIDs)
}

func (r *OwnershipRegistry) readyVMIDsLocked() []string {
	vmIDs := make([]string, 0, len(r.ownerships)+len(r.workerVMs))
	for _, own := range r.ownerships {
		if own != nil && own.IsReady() {
			vmIDs = append(vmIDs, own.VMID)
		}
	}
	for _, own := range r.workerVMs {
		if own != nil && own.IsReady() {
			vmIDs = append(vmIDs, own.VMID)
		}
	}
	return vmIDs
}

func (r *OwnershipRegistry) reconcileGatewayCredentialsForVMs(vmIDs []string) {
	seen := make(map[string]struct{}, len(vmIDs))
	for _, vmID := range vmIDs {
		vmID = strings.TrimSpace(vmID)
		if vmID == "" {
			continue
		}
		if _, ok := seen[vmID]; ok {
			continue
		}
		seen[vmID] = struct{}{}
		r.ensureExistingGatewayCredential(vmID)
	}
}

// ResolveOrAssign resolves the VM ownership for the primary desktop of the
// given user.
func (r *OwnershipRegistry) ResolveOrAssign(userID string) (*VMOwnership, error) {
	return r.ResolveOrAssignDesktopContext(context.Background(), userID, PrimaryDesktopID)
}

// ResolveOrAssignDesktop resolves the VM ownership for the given user/desktop
// pair. If the desktop already has an active VM, it is returned. If the VM is
// still booting, concurrent callers wait for that same boot to finish instead
// of routing to a placeholder sandbox URL. If no VM exists, a new VM is
// assigned. Concurrent first requests for the same
// user/desktop pair collapse onto one assignment.
func (r *OwnershipRegistry) ResolveOrAssignDesktop(userID, desktopID string) (*VMOwnership, error) {
	return r.ResolveOrAssignDesktopContext(context.Background(), userID, desktopID)
}

// ResolveOrAssignDesktopContext resolves the VM ownership for the given
// user/desktop pair and lets callers abandon pending boot waits when their
// request context is canceled. The first boot caller is allowed to keep
// booting the VM so a later retry can reuse the completed assignment.
func (r *OwnershipRegistry) ResolveOrAssignDesktopContext(ctx context.Context, userID, desktopID string) (*VMOwnership, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	desktopID = normalizeDesktopID(desktopID)
	key := ownershipKey(userID, desktopID)
	r.mu.Lock()

	// Check if the desktop already has an active ownership.
	if own, ok := r.ownerships[key]; ok {
		if own.IsReady() {
			mgr := r.vmManager
			snapshot := *own
			if activeOwnershipNeedsReadinessCheck(&snapshot, mgr) {
				r.mu.Unlock()
				info, err := r.ensureActiveVMReady(&snapshot, mgr)
				if err != nil {
					log.Printf("vmctl: active VM %s readiness check failed: %v", snapshot.VMID, err)
					return nil, fmt.Errorf("failed to verify active VM %s: %w", snapshot.VMID, err)
				}

				r.mu.Lock()
				current := r.ownerships[key]
				if current == nil || current.VMID != snapshot.VMID || !current.IsReady() {
					r.mu.Unlock()
					return r.ResolveOrAssignDesktopContext(ctx, userID, desktopID)
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
		}

		// VM exists but is stopped or hibernated. Resume it instead
		// of creating a new VM, preserving the user's state and epoch
		// (VAL-CROSS-116, VAL-CROSS-117).
		if own.State == VMStateStopped || own.State == VMStateHibernated {
			mgr := r.vmManager
			r.mu.Unlock()

			if mgr != nil {
				info, err := r.startExistingVM(own, mgr)
				if err != nil {
					log.Printf("vmctl: start existing VM %s failed: %v", own.VMID, err)
					return nil, fmt.Errorf("failed to start existing VM %s: %w", own.VMID, err)
				}
				r.mu.Lock()
				own.SandboxURL = info.HostURL
				own.Epoch = info.Epoch
				r.mu.Unlock()
			}

			r.mu.Lock()
			own.State = VMStateActive
			own.LastActiveAt = time.Now()
			own.StoppedBy = ""
			r.saveLocked()
			r.mu.Unlock()
			r.ensureExistingGatewayCredential(own.VMID)
			log.Printf("vmctl: resumed VM %s for user %s desktop %s on resolve (epoch=%d)", own.VMID, userID, desktopID, own.Epoch)
			return own, nil
		}

		if own.State == VMStateBooting {
			if waiters, ok := r.pendingWaiters[key]; ok {
				return r.waitForPendingAssignmentLocked(ctx, key, userID, desktopID, waiters)
			}
			// No pending waiter means this is a stale booting ownership, for
			// example after a process restart. Fall through and recover it the
			// same way as a failed/degraded ownership.
		}

		// VM exists but failed or is degraded. Create a new one
		// with a fresh epoch. Clean up the old mapping.
		delete(r.vmByID, own.VMID)
	}

	// Check if a VM assignment is already in progress for this user/desktop.
	// The zero-waiter case still means a first caller is actively booting the VM,
	// so later callers must join that in-flight boot rather than minting a second
	// VM or routing to the placeholder sandbox URL.
	if waiters, ok := r.pendingWaiters[key]; ok {
		return r.waitForPendingAssignmentLocked(ctx, key, userID, desktopID, waiters)
	}

	// We are the first caller for this user/desktop pair. Create a new VM.
	vmID := generateVMID()
	epoch := r.nextEpoch()

	own := &VMOwnership{
		VMID:      vmID,
		UserID:    userID,
		DesktopID: desktopID,
		Kind:      VMKindInteractive,
		WarmnessClass: warmnessClassForOwnership(&VMOwnership{
			UserID:    userID,
			DesktopID: desktopID,
			Kind:      VMKindInteractive,
		}, r.warmnessPolicy),
		SandboxURL:   r.sandboxURLForVM(vmID),
		State:        VMStateBooting,
		CreatedAt:    time.Now(),
		LastActiveAt: time.Now(),
		Epoch:        epoch,
		Published:    true,
	}

	// Register pending waiters map before unlocking so other callers can find it.
	r.pendingWaiters[key] = nil

	// Store the ownership immediately in booting state.
	r.ownerships[key] = own
	r.vmByID[vmID] = own
	r.saveLocked()

	// Check if we have a real Firecracker VM manager.
	mgr := r.vmManager

	r.mu.Unlock()

	// Boot the real Firecracker VM if a manager is configured.
	if mgr != nil {
		// Issue a gateway token for the VM sandbox before booting.
		// The token is written to the persistent directory by the vmmanager
		// and read by the guest init script to authenticate to the gateway.
		gwToken := r.issueGatewayToken(vmID)

		info, err := mgr.BootVM(VMManagerConfig{
			VMID:              vmID,
			GuestPort:         8085,
			MachineCPUCount:   interactiveVMCPUCount,
			MachineMemSizeMib: interactiveVMMemSizeMib,
			GatewayToken:      gwToken,
			ComputerKind:      "active",
			OwnerID:           userID,
			DesktopID:         desktopID,
		})
		if err != nil {
			log.Printf("vmctl: Firecracker boot failed for VM %s: %v", vmID, err)
			r.mu.Lock()
			own.State = VMStateFailed
			r.saveLocked()
			waiters := r.pendingWaiters[key]
			delete(r.pendingWaiters, key)
			r.mu.Unlock()
			for _, ch := range waiters {
				ch <- nil
			}
			return nil, fmt.Errorf("failed to boot VM %s: %w", vmID, err)
		}
		r.mu.Lock()
		own.SandboxURL = info.HostURL
		own.Epoch = info.Epoch
		r.mu.Unlock()
		log.Printf("vmctl: booted Firecracker VM %s for user %s at %s (epoch=%d)", vmID, userID, info.HostURL, info.Epoch)
	}

	// Transition to active.
	r.transitionVM(vmID, VMStateActive)

	// Notify any waiters.
	r.mu.Lock()
	waiters := r.pendingWaiters[key]
	delete(r.pendingWaiters, key)
	r.mu.Unlock()

	for _, ch := range waiters {
		ch <- own
	}

	log.Printf("vmctl: assigned VM %s to user %s desktop %s", vmID, userID, desktopID)

	return own, nil
}

func (r *OwnershipRegistry) waitForPendingAssignmentLocked(ctx context.Context, key, userID, desktopID string, waiters []chan *VMOwnership) (*VMOwnership, error) {
	ch := make(chan *VMOwnership, 1)
	r.pendingWaiters[key] = append(waiters, ch)
	r.mu.Unlock()

	select {
	case own := <-ch:
		if own == nil {
			return nil, fmt.Errorf("vm assignment failed for user %s desktop %s", userID, desktopID)
		}
		return own, nil
	case <-ctx.Done():
		r.removePendingWaiter(key, ch)
		return nil, fmt.Errorf("vm assignment canceled for user %s desktop %s: %w", userID, desktopID, ctx.Err())
	}
}

func (r *OwnershipRegistry) removePendingWaiter(key string, target chan *VMOwnership) {
	r.mu.Lock()
	defer r.mu.Unlock()

	waiters, ok := r.pendingWaiters[key]
	if !ok {
		return
	}
	for i, ch := range waiters {
		if ch == target {
			waiters = append(waiters[:i], waiters[i+1:]...)
			break
		}
	}
	r.pendingWaiters[key] = waiters
}

// ForkDesktop creates or resumes a distinct interactive VM for a target desktop
// derived from an existing source desktop. The target desktop must differ from
// the source desktop and the source desktop must already exist.
func (r *OwnershipRegistry) ForkDesktop(userID, sourceDesktopID, targetDesktopID string) (*VMOwnership, error) {
	sourceDesktopID = normalizeDesktopID(sourceDesktopID)
	targetDesktopID = normalizeDesktopID(targetDesktopID)
	if sourceDesktopID == targetDesktopID {
		return nil, fmt.Errorf("target desktop must differ from source desktop")
	}

	sourceKey := ownershipKey(userID, sourceDesktopID)
	targetKey := ownershipKey(userID, targetDesktopID)

	r.mu.Lock()
	source := r.ownerships[sourceKey]
	if source == nil {
		r.mu.Unlock()
		return nil, fmt.Errorf("no source VM found for user %s desktop %s", userID, sourceDesktopID)
	}
	if source.State != VMStateActive && source.State != VMStateStopped && source.State != VMStateHibernated {
		r.mu.Unlock()
		return nil, fmt.Errorf("source VM %s is not forkable while state=%s", source.VMID, source.State)
	}

	if own, ok := r.ownerships[targetKey]; ok {
		own.ParentDesktopID = sourceDesktopID
		if own.ParentVMID == "" {
			own.ParentVMID = source.VMID
		}
		if own.SnapshotKind == "" {
			own.SnapshotKind = "existing_target"
		}
		own.LastActiveAt = time.Now()
		own.Published = false
		r.saveLocked()
		r.mu.Unlock()
		log.Printf("vmctl: fork target desktop %s already exists for user %s on VM %s", targetDesktopID, userID, own.VMID)
		return own, nil
	}

	if waiters, ok := r.pendingWaiters[targetKey]; ok {
		ch := make(chan *VMOwnership, 1)
		r.pendingWaiters[targetKey] = append(waiters, ch)
		r.mu.Unlock()

		own := <-ch
		if own == nil {
			return nil, fmt.Errorf("fork VM assignment failed for user %s desktop %s", userID, targetDesktopID)
		}
		return own, nil
	}

	mgr := r.vmManager
	if mgr != nil && source.State == VMStateActive {
		r.mu.Unlock()
		return nil, fmt.Errorf("source VM %s is active; refusing unsafe live data image fork", source.VMID)
	}

	sourceVMID := source.VMID
	now := time.Now()
	vmID := generateVMID()
	snapshotKind := "metadata_only"
	if mgr != nil {
		snapshotKind = "data_img_copy"
	}
	own := &VMOwnership{
		VMID:            vmID,
		UserID:          userID,
		DesktopID:       targetDesktopID,
		Kind:            VMKindInteractive,
		WarmnessClass:   WarmnessClassCandidate,
		ParentDesktopID: sourceDesktopID,
		ParentVMID:      sourceVMID,
		SnapshotKind:    snapshotKind,
		SandboxURL:      r.sandboxURLForVM(vmID),
		State:           VMStateBooting,
		CreatedAt:       now,
		LastActiveAt:    now,
		Epoch:           r.nextEpoch(),
		Published:       false,
	}
	r.pendingWaiters[targetKey] = nil
	r.ownerships[targetKey] = own
	r.vmByID[vmID] = own
	r.saveLocked()
	r.mu.Unlock()

	if mgr != nil {
		info, err := mgr.BootVM(VMManagerConfig{
			VMID:              vmID,
			GuestPort:         8085,
			MachineCPUCount:   interactiveVMCPUCount,
			MachineMemSizeMib: interactiveVMMemSizeMib,
			SourceVMID:        sourceVMID,
			GatewayToken:      r.issueGatewayToken(vmID),
			ComputerKind:      "candidate",
			OwnerID:           userID,
			DesktopID:         targetDesktopID,
			CandidateID:       targetDesktopID,
		})
		if err != nil {
			log.Printf("vmctl: Firecracker fork boot failed for VM %s from %s: %v", vmID, sourceVMID, err)
			r.mu.Lock()
			own.State = VMStateFailed
			r.saveLocked()
			waiters := r.pendingWaiters[targetKey]
			delete(r.pendingWaiters, targetKey)
			r.mu.Unlock()
			for _, ch := range waiters {
				ch <- nil
			}
			return nil, fmt.Errorf("failed to boot fork VM %s from %s: %w", vmID, sourceVMID, err)
		}
		r.mu.Lock()
		own.SandboxURL = info.HostURL
		own.Epoch = info.Epoch
		r.mu.Unlock()
	}

	r.transitionVM(vmID, VMStateActive)

	r.mu.Lock()
	waiters := r.pendingWaiters[targetKey]
	delete(r.pendingWaiters, targetKey)
	r.mu.Unlock()
	for _, ch := range waiters {
		ch <- own
	}

	log.Printf("vmctl: forked desktop %s from %s for user %s onto VM %s", targetDesktopID, sourceDesktopID, userID, own.VMID)
	return own, nil
}

// RequestWorker provisions a headless child VM under an existing desktop and
// returns a typed worker handle. Workers are keyed by worker_id, not by desktop
// routing state, because multiple workers may belong to one desktop.
func (r *OwnershipRegistry) RequestWorker(req WorkerRequest) (*VMOwnership, error) {
	req.UserID = strings.TrimSpace(req.UserID)
	req.DesktopID = normalizeDesktopID(req.DesktopID)
	req.ParentAgentID = strings.TrimSpace(req.ParentAgentID)
	req.TrajectoryID = strings.TrimSpace(req.TrajectoryID)
	req.Purpose = strings.TrimSpace(req.Purpose)
	req.ObjectiveFingerprint = strings.TrimSpace(req.ObjectiveFingerprint)
	if req.UserID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	if req.ParentAgentID == "" {
		return nil, fmt.Errorf("parent_agent_id is required")
	}
	if req.Purpose == "" {
		return nil, fmt.Errorf("purpose is required")
	}
	machineClass, cpuCount, memSizeMib, err := normalizeWorkerMachineClass(req.MachineClass)
	if err != nil {
		return nil, err
	}
	if req.ObjectiveFingerprint == "" {
		req.ObjectiveFingerprint = workerObjectiveFingerprint(req.UserID, req.DesktopID, req.ParentAgentID, req.TrajectoryID, req.Purpose)
	}

	r.mu.RLock()
	parent := r.ownerships[ownershipKey(req.UserID, req.DesktopID)]
	if parent != nil && !req.AllowParallel {
		for _, worker := range r.workerVMs {
			if reusableWorkerLease(worker, req, machineClass) {
				r.mu.RUnlock()
				log.Printf("vmctl: reused worker VM %s for user %s desktop %s (worker_id=%s purpose=%q)", worker.VMID, req.UserID, req.DesktopID, worker.WorkerID, req.Purpose)
				return worker, nil
			}
		}
	}
	r.mu.RUnlock()
	if parent == nil {
		return nil, fmt.Errorf("no parent desktop VM found for user %s desktop %s", req.UserID, req.DesktopID)
	}
	var imageProfile VMImageProfile
	hasImageProfile := false
	r.mu.RLock()
	mgrConfigured := r.vmManager != nil
	if mgrConfigured {
		imageProfile, hasImageProfile = r.workerImageProfiles[machineClass]
	}
	r.mu.RUnlock()
	if mgrConfigured {
		if workerMachineClassRequiresImageProfile(machineClass) && !hasImageProfile {
			return nil, fmt.Errorf("%s requires a configured worker image profile", machineClass)
		}
		if hasImageProfile {
			if missing := imageProfile.MissingRequiredFields(); len(missing) > 0 {
				return nil, fmt.Errorf("%s worker image profile is incomplete: missing %s", machineClass, strings.Join(missing, ", "))
			}
		}
	}

	now := time.Now()
	vmID := generateVMID()
	workerID := generateWorkerID()
	own := &VMOwnership{
		VMID:                 vmID,
		UserID:               req.UserID,
		DesktopID:            req.DesktopID,
		Kind:                 VMKindWorker,
		WorkerID:             workerID,
		ParentAgentID:        req.ParentAgentID,
		TrajectoryID:         req.TrajectoryID,
		Purpose:              req.Purpose,
		ObjectiveFingerprint: req.ObjectiveFingerprint,
		MachineClass:         machineClass,
		WarmnessClass: warmnessClassForOwnership(&VMOwnership{
			UserID:               req.UserID,
			DesktopID:            req.DesktopID,
			Kind:                 VMKindWorker,
			ParentAgentID:        req.ParentAgentID,
			TrajectoryID:         req.TrajectoryID,
			Purpose:              req.Purpose,
			ObjectiveFingerprint: req.ObjectiveFingerprint,
			MachineClass:         machineClass,
		}, r.warmnessPolicy),
		SandboxURL:      r.sandboxURLForVM(vmID),
		State:           VMStateBooting,
		CreatedAt:       now,
		LastActiveAt:    now,
		Published:       false,
		ParentDesktopID: "",
	}

	r.mu.Lock()
	own.Epoch = r.nextEpoch()
	r.workerVMs[workerID] = own
	r.vmByID[vmID] = own
	mgr := r.vmManager
	r.saveLocked()
	r.mu.Unlock()

	if mgr != nil {
		gwToken := r.issueGatewayToken(vmID)
		bootCfg := VMManagerConfig{
			VMID:              vmID,
			KernelImagePath:   imageProfile.KernelImagePath,
			InitrdPath:        imageProfile.InitrdPath,
			RootfsPath:        imageProfile.RootfsPath,
			StoreDiskPath:     imageProfile.StoreDiskPath,
			KernelParams:      imageProfile.KernelParams,
			GuestPort:         8085,
			MachineCPUCount:   cpuCount,
			MachineMemSizeMib: memSizeMib,
			GatewayToken:      gwToken,
			ComputerKind:      "worker",
			OwnerID:           req.UserID,
			DesktopID:         req.DesktopID,
			WorkerID:          workerID,
			CandidateID:       workerID,
		}
		info, err := mgr.BootVM(bootCfg)
		if err != nil {
			log.Printf("vmctl: Firecracker boot failed for worker VM %s: %v", vmID, err)
			r.mu.Lock()
			own.State = VMStateFailed
			r.saveLocked()
			r.mu.Unlock()
			return nil, fmt.Errorf("failed to boot worker VM %s: %w", vmID, err)
		}
		r.mu.Lock()
		own.SandboxURL = info.HostURL
		own.Epoch = info.Epoch
		r.mu.Unlock()
		log.Printf("vmctl: booted worker VM %s for user %s desktop %s (worker_id=%s class=%s epoch=%d)", vmID, req.UserID, req.DesktopID, workerID, machineClass, own.Epoch)
	}

	r.transitionVM(vmID, VMStateActive)
	log.Printf("vmctl: assigned worker VM %s for user %s desktop %s (worker_id=%s purpose=%q)", vmID, req.UserID, req.DesktopID, workerID, req.Purpose)
	return own, nil
}

func reusableWorkerLease(worker *VMOwnership, req WorkerRequest, machineClass string) bool {
	if worker == nil {
		return false
	}
	switch worker.State {
	case VMStateBooting, VMStateActive, VMStateDegraded:
	default:
		return false
	}
	return worker.UserID == req.UserID &&
		worker.DesktopID == req.DesktopID &&
		worker.ParentAgentID == req.ParentAgentID &&
		worker.TrajectoryID == req.TrajectoryID &&
		workerObjectiveFingerprintForOwnership(worker) == req.ObjectiveFingerprint &&
		worker.MachineClass == machineClass
}

// PublishDesktop marks a background candidate desktop as user-switchable.
func (r *OwnershipRegistry) PublishDesktop(userID, desktopID string) (*VMOwnership, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	own, ok := r.ownerships[ownershipKey(userID, desktopID)]
	if !ok {
		return nil, fmt.Errorf("no VM found for user %s desktop %s", userID, normalizeDesktopID(desktopID))
	}
	own.Published = true
	own.LastActiveAt = time.Now()
	r.saveLocked()
	log.Printf("vmctl: published desktop %s for user %s on VM %s", own.DesktopID, userID, own.VMID)
	return own, nil
}

// GetOwnership returns the current ownership for a user's primary desktop, or
// nil if none exists.
func (r *OwnershipRegistry) GetOwnership(userID string) *VMOwnership {
	return r.GetOwnershipForDesktop(userID, PrimaryDesktopID)
}

// GetOwnershipForDesktop returns the current ownership for a specific
// user/desktop pair, or nil if none exists.
func (r *OwnershipRegistry) GetOwnershipForDesktop(userID, desktopID string) *VMOwnership {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.ownerships[ownershipKey(userID, desktopID)]
}

// GetOwnershipByVMID returns the ownership for a specific VM ID, or nil.
func (r *OwnershipRegistry) GetOwnershipByVMID(vmID string) *VMOwnership {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.vmByID[vmID]
}

// ListOwnerships returns all current ownerships.
func (r *OwnershipRegistry) ListOwnerships() []*VMOwnership {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*VMOwnership, 0, len(r.ownerships)+len(r.workerVMs))
	for _, own := range r.ownerships {
		result = append(result, own)
	}
	for _, own := range r.workerVMs {
		result = append(result, own)
	}
	return result
}

// WarmnessSummary returns redacted aggregate lifecycle policy state. It never
// includes user IDs, VM IDs, desktop IDs, or credentials.
func (r *OwnershipRegistry) WarmnessSummary(idleEligible []*VMOwnership) WarmnessHealthSummary {
	r.mu.RLock()
	cfg := r.warmnessPolicy
	ownerships := make([]*VMOwnership, 0, len(r.ownerships)+len(r.workerVMs))
	for _, own := range r.ownerships {
		ownerships = append(ownerships, cloneOwnership(own))
	}
	for _, own := range r.workerVMs {
		ownerships = append(ownerships, cloneOwnership(own))
	}
	idle := make([]*VMOwnership, 0, len(idleEligible))
	for _, own := range idleEligible {
		idle = append(idle, cloneOwnership(own))
	}
	r.mu.RUnlock()
	return warmnessSummary(cfg, ownerships, idle)
}

// WarmnessClassForOwnership returns the policy class for an ownership using
// the registry's current warmness configuration.
func (r *OwnershipRegistry) WarmnessClassForOwnership(own *VMOwnership) WarmnessClass {
	r.mu.RLock()
	cfg := r.warmnessPolicy
	r.mu.RUnlock()
	return warmnessClassForOwnership(own, cfg)
}

// StopVM stops the VM for the given user's primary desktop.
func (r *OwnershipRegistry) StopVM(userID string) error {
	return r.StopVMForDesktop(userID, PrimaryDesktopID)
}

// StopVMForDesktop stops the VM for the given user/desktop pair,
// transitioning it to stopped state.
func (r *OwnershipRegistry) StopVMForDesktop(userID, desktopID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := ownershipKey(userID, desktopID)
	own, ok := r.ownerships[key]
	if !ok {
		return fmt.Errorf("no VM found for user %s desktop %s", userID, normalizeDesktopID(desktopID))
	}

	// Delegate to the real VM manager if available.
	if r.vmManager != nil && (own.State == VMStateActive || own.State == VMStateDegraded) {
		_ = r.vmManager.StopVM(own.VMID)
	}

	own.State = VMStateStopped
	own.LastActiveAt = time.Now()
	r.saveLocked()
	log.Printf("vmctl: stopped VM %s for user %s desktop %s", own.VMID, userID, own.DesktopID)
	return nil
}

// RemoveOwnership removes the ownership for a user's primary desktop.
func (r *OwnershipRegistry) RemoveOwnership(userID string) error {
	return r.RemoveOwnershipForDesktop(userID, PrimaryDesktopID)
}

// RemoveOwnershipForDesktop removes the ownership for a specific user/desktop
// pair entirely (e.g. after logout). The VM is stopped and the mappings are
// cleaned up.
func (r *OwnershipRegistry) RemoveOwnershipForDesktop(userID, desktopID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := ownershipKey(userID, desktopID)
	own, ok := r.ownerships[key]
	if !ok {
		return nil // already gone, idempotent
	}

	// Delegate to the real VM manager if available.
	if r.vmManager != nil && (own.State == VMStateActive || own.State == VMStateDegraded) {
		_ = r.vmManager.StopVM(own.VMID)
	}

	own.State = VMStateStopped
	delete(r.ownerships, key)
	delete(r.vmByID, own.VMID)
	for workerID, worker := range r.workerVMs {
		if worker.UserID != userID || worker.DesktopID != normalizeDesktopID(desktopID) {
			continue
		}
		if r.vmManager != nil && (worker.State == VMStateActive || worker.State == VMStateDegraded) {
			_ = r.vmManager.StopVM(worker.VMID)
		}
		worker.State = VMStateStopped
		delete(r.workerVMs, workerID)
		delete(r.vmByID, worker.VMID)
	}
	r.saveLocked()
	log.Printf("vmctl: removed VM %s ownership for user %s desktop %s", own.VMID, userID, own.DesktopID)
	return nil
}

// MarkUnhealthy marks the VM for the given user's primary desktop as degraded.
func (r *OwnershipRegistry) MarkUnhealthy(userID string) error {
	return r.MarkUnhealthyForDesktop(userID, PrimaryDesktopID)
}

// MarkUnhealthyForDesktop marks the VM for the given user/desktop pair as
// degraded.
func (r *OwnershipRegistry) MarkUnhealthyForDesktop(userID, desktopID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	own, ok := r.ownerships[ownershipKey(userID, desktopID)]
	if !ok {
		return fmt.Errorf("no VM found for user %s desktop %s", userID, normalizeDesktopID(desktopID))
	}

	own.State = VMStateDegraded
	own.LastActiveAt = time.Now()
	r.saveLocked()
	log.Printf("vmctl: marked VM %s unhealthy for user %s desktop %s", own.VMID, userID, own.DesktopID)
	return nil
}

// HibernateVM transitions the VM for the given user to hibernated state.
// The VM can be resumed later with ResumeVM, restoring the same user's
// persisted state (VAL-CROSS-116, VAL-VM-008).
//
// The epoch does NOT change on hibernate; it will stay the same on resume,
// allowing callers to distinguish fresh boot from resume (VAL-CROSS-117).
func (r *OwnershipRegistry) HibernateVM(userID string) error {
	return r.HibernateVMForDesktop(userID, PrimaryDesktopID)
}

func (r *OwnershipRegistry) HibernateVMForDesktop(userID, desktopID string) error {
	return r.hibernateVMForDesktopWithReason(userID, desktopID, "idle")
}

func (r *OwnershipRegistry) hibernateVMForDesktopWithReason(userID, desktopID, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	own, ok := r.ownerships[ownershipKey(userID, desktopID)]
	if !ok {
		return fmt.Errorf("no VM found for user %s desktop %s", userID, normalizeDesktopID(desktopID))
	}

	if own.State != VMStateActive && own.State != VMStateDegraded {
		return fmt.Errorf("VM %s cannot be hibernated (state=%s)", own.VMID, own.State)
	}

	// Delegate to the real VM manager if available.
	if r.vmManager != nil {
		_ = r.vmManager.HibernateVM(own.VMID)
	}

	own.State = VMStateHibernated
	own.LastActiveAt = time.Now()
	own.StoppedBy = normalizeStopReason(reason)
	r.saveLocked()
	log.Printf("vmctl: hibernated VM %s for user %s desktop %s reason=%s (epoch=%d)", own.VMID, userID, own.DesktopID, own.StoppedBy, own.Epoch)
	return nil
}

// HibernateWorker transitions the worker VM with the given typed handle to
// hibernated state.
func (r *OwnershipRegistry) HibernateWorker(workerID string) error {
	return r.hibernateWorkerWithReason(workerID, "idle")
}

func (r *OwnershipRegistry) hibernateWorkerWithReason(workerID, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	own, ok := r.workerVMs[strings.TrimSpace(workerID)]
	if !ok {
		return fmt.Errorf("no worker VM found for worker_id %s", strings.TrimSpace(workerID))
	}
	if own.State != VMStateActive && own.State != VMStateDegraded {
		return fmt.Errorf("worker VM %s cannot be hibernated (state=%s)", own.VMID, own.State)
	}
	if r.vmManager != nil {
		_ = r.vmManager.HibernateVM(own.VMID)
	}
	own.State = VMStateHibernated
	own.LastActiveAt = time.Now()
	own.StoppedBy = normalizeStopReason(reason)
	r.saveLocked()
	log.Printf("vmctl: hibernated worker VM %s for user %s desktop %s worker_id %s reason=%s", own.VMID, own.UserID, own.DesktopID, own.WorkerID, own.StoppedBy)
	return nil
}

func normalizeStopReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "idle"
	}
	return reason
}

// ReclaimPressureVMs hibernates the top ranked pressure-reclaim candidates
// when active pressure reclaim is enabled and the host is currently under
// pressure. Candidate selection is bounded by MaxCandidates and excludes
// protected computers such as premium always-on and critical verifier workers.
func (r *OwnershipRegistry) ReclaimPressureVMs() int {
	candidates := r.pressureReclaimActionCandidates()
	reclaimed := 0
	for _, candidate := range candidates {
		if candidate.own == nil || candidate.public.Protected {
			continue
		}
		var err error
		if candidate.own.Kind == VMKindWorker {
			err = r.hibernateWorkerWithReason(candidate.own.WorkerID, "pressure")
		} else {
			err = r.hibernateVMForDesktopWithReason(candidate.own.UserID, candidate.own.DesktopID, "pressure")
		}
		if err == nil {
			reclaimed++
		}
	}
	return reclaimed
}

// ReclaimStaleVMState deletes terminal worker/candidate VM state only when
// state-dir storage pressure is present. It intentionally excludes active,
// primary, published, premium, and recent work; package/source evidence must
// survive outside these disposable producer machines before their VM state is
// eligible for deletion.
func (r *OwnershipRegistry) ReclaimStaleVMState() int {
	candidates := r.staleStateReclaimCandidates()
	destroyed := 0
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		if r.destroyStaleVMState(candidate) {
			destroyed++
		}
	}
	return destroyed
}

func (r *OwnershipRegistry) staleStateReclaimCandidates() []*VMOwnership {
	r.mu.RLock()
	cfg := normalizePressureReclaimConfig(r.pressureReclaim)
	warmnessPolicy := r.warmnessPolicy
	sampler := r.pressureSampler
	ownerships := make([]*VMOwnership, 0, len(r.ownerships)+len(r.workerVMs))
	for _, own := range r.ownerships {
		ownerships = append(ownerships, cloneOwnership(own))
	}
	for _, own := range r.workerVMs {
		ownerships = append(ownerships, cloneOwnership(own))
	}
	r.mu.RUnlock()

	if cfg.Mode != PressureReclaimModeActive || cfg.MaxStateDeletes <= 0 {
		return nil
	}
	if sampler == nil {
		sampler = sampleHostPressure
	}
	sample := sampler(cfg)
	annotatePressure(&sample, cfg)
	if !sample.StateDirPressure {
		return nil
	}

	now := time.Now()
	candidates := make([]*VMOwnership, 0, len(ownerships))
	for _, own := range ownerships {
		if staleVMStateReclaimable(own, cfg, warmnessPolicy, now) {
			candidates = append(candidates, own)
		}
	}
	stateSizes := make(map[string]int64, len(candidates))
	for _, own := range candidates {
		stateSizes[own.VMID] = vmStateDirUsageBytes(cfg.StateDir, own.VMID)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		leftSize := stateSizes[left.VMID]
		rightSize := stateSizes[right.VMID]
		if leftSize != rightSize {
			return leftSize > rightSize
		}
		if left.Kind != right.Kind {
			return left.Kind == VMKindWorker
		}
		leftIdle := now.Sub(left.LastActiveAt)
		rightIdle := now.Sub(right.LastActiveAt)
		if leftIdle != rightIdle {
			return leftIdle > rightIdle
		}
		return left.VMID < right.VMID
	})
	if len(candidates) > cfg.MaxStateDeletes {
		candidates = candidates[:cfg.MaxStateDeletes]
	}
	return candidates
}

func vmStateDirUsageBytes(stateDir, vmID string) int64 {
	vmID = strings.TrimSpace(vmID)
	if vmID == "" {
		return 0
	}
	root := filepath.Clean(stateDir)
	if root == "." || root == string(os.PathSeparator) {
		return 0
	}
	dir := filepath.Clean(filepath.Join(root, vmID))
	if dir == root || !strings.HasPrefix(dir, root+string(os.PathSeparator)) {
		return 0
	}
	var total int64
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if stat, ok := info.Sys().(*syscall.Stat_t); ok && stat.Blocks > 0 {
			total += stat.Blocks * 512
			return nil
		}
		if info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	return total
}

func staleVMStateReclaimable(own *VMOwnership, cfg PressureReclaimConfig, warmnessPolicy WarmnessPolicyConfig, now time.Time) bool {
	if own == nil || strings.TrimSpace(own.VMID) == "" || own.LastActiveAt.IsZero() {
		return false
	}
	switch own.State {
	case VMStateStopped, VMStateHibernated, VMStateFailed:
	default:
		return false
	}
	if now.Sub(own.LastActiveAt) < cfg.StaleStateMinAge {
		return false
	}
	switch warmnessClassForOwnership(own, warmnessPolicy) {
	case WarmnessClassPremiumAlwaysOn:
		return false
	case WarmnessClassCriticalProtected:
		if !staleCriticalWorkerIdle(now.Sub(own.LastActiveAt)) {
			return false
		}
	}
	if own.Kind == VMKindWorker {
		return true
	}
	if own.Kind != VMKindInteractive {
		return false
	}
	if own.DesktopID == PrimaryDesktopID || own.Published {
		return false
	}
	return true
}

func (r *OwnershipRegistry) destroyStaleVMState(candidate *VMOwnership) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	cfg := normalizePressureReclaimConfig(r.pressureReclaim)
	warmnessPolicy := r.warmnessPolicy
	now := time.Now()
	current, key := r.currentOwnershipForCandidateLocked(candidate)
	if !staleVMStateReclaimable(current, cfg, warmnessPolicy, now) {
		return false
	}
	if r.vmManager == nil {
		return false
	}
	if err := r.vmManager.DestroyVMState(current.VMID); err != nil {
		log.Printf("vmctl: stale VM state destroy skipped for %s: %v", current.VMID, err)
		return false
	}
	if current.Kind == VMKindWorker {
		delete(r.workerVMs, strings.TrimSpace(current.WorkerID))
	} else if key != "" {
		delete(r.ownerships, key)
	}
	delete(r.vmByID, current.VMID)
	r.saveLocked()
	log.Printf("vmctl: destroyed stale %s VM state %s for desktop %s", current.Kind, current.VMID, current.DesktopID)
	return true
}

func (r *OwnershipRegistry) currentOwnershipForCandidateLocked(candidate *VMOwnership) (*VMOwnership, string) {
	if candidate == nil {
		return nil, ""
	}
	if candidate.Kind == VMKindWorker {
		own := r.workerVMs[strings.TrimSpace(candidate.WorkerID)]
		if own != nil && own.VMID == candidate.VMID {
			return own, ""
		}
		return nil, ""
	}
	key := ownershipKey(candidate.UserID, candidate.DesktopID)
	own := r.ownerships[key]
	if own != nil && own.VMID == candidate.VMID {
		return own, key
	}
	return nil, ""
}

// ResumeVM resumes a stopped or hibernated VM for the given user,
// restoring the same user's persisted state (VAL-CROSS-116).
//
// The epoch does NOT increment on resume, so callers can detect that
// this is a resume rather than a fresh boot. This prevents duplicate
// canonical effects (VAL-CROSS-117).
func (r *OwnershipRegistry) ResumeVM(userID string) (*VMOwnership, error) {
	return r.ResumeVMForDesktop(userID, PrimaryDesktopID)
}

func (r *OwnershipRegistry) ResumeVMForDesktop(userID, desktopID string) (*VMOwnership, error) {
	var ensureVMID string
	defer func() {
		if ensureVMID != "" {
			r.ensureExistingGatewayCredential(ensureVMID)
		}
	}()
	r.mu.Lock()

	key := ownershipKey(userID, desktopID)
	own, ok := r.ownerships[key]
	if !ok {
		r.mu.Unlock()
		return nil, fmt.Errorf("no VM found for user %s desktop %s", userID, normalizeDesktopID(desktopID))
	}

	if own.State != VMStateStopped && own.State != VMStateHibernated {
		if own.State == VMStateActive || own.State == VMStateBooting {
			own.LastActiveAt = time.Now()
			if own.IsReady() {
				ensureVMID = own.VMID
			}
			r.mu.Unlock()
			return own, nil
		}
		r.mu.Unlock()
		return nil, fmt.Errorf("VM %s cannot be resumed (state=%s)", own.VMID, own.State)
	}

	// Delegate to the real VM manager if available.
	mgr := r.vmManager
	vmID := own.VMID
	if mgr != nil {
		snapshot := *own
		r.mu.Unlock()
		info, err := r.startExistingVM(&snapshot, mgr)
		if err != nil {
			return nil, fmt.Errorf("failed to resume VM %s: %w", vmID, err)
		}
		r.mu.Lock()
		own = r.ownerships[key]
		if own == nil || own.VMID != vmID {
			r.mu.Unlock()
			return nil, fmt.Errorf("VM %s ownership changed during resume", vmID)
		}
		if info != nil {
			own.SandboxURL = info.HostURL
			own.Epoch = info.Epoch
		}
	}

	// Transition to active. Epoch stays the same for resume (VAL-CROSS-117).
	// A fresh boot would increment the epoch.
	own.State = VMStateActive
	own.LastActiveAt = time.Now()
	own.StoppedBy = ""
	r.saveLocked()
	log.Printf("vmctl: resumed VM %s for user %s desktop %s (epoch=%d, same-epoch=resume)", own.VMID, userID, own.DesktopID, own.Epoch)
	ensureVMID = own.VMID
	r.mu.Unlock()
	return own, nil
}

// RecoverVM recovers an unhealthy or failed VM for the given user.
// Unlike ResumeVM, RecoverVM creates a fresh boot by incrementing the
// epoch counter. This signals to callers that any in-flight work from
// the previous boot should not be retried (VAL-CROSS-117, VAL-VM-009).
//
// The persistent user data is preserved across recovery so the user's
// state survives the crash (VAL-CROSS-116).
func (r *OwnershipRegistry) RecoverVM(userID string) (*VMOwnership, error) {
	return r.RecoverVMForDesktop(userID, PrimaryDesktopID)
}

func (r *OwnershipRegistry) RecoverVMForDesktop(userID, desktopID string) (*VMOwnership, error) {
	var ensureVMID string
	defer func() {
		if ensureVMID != "" {
			r.ensureExistingGatewayCredential(ensureVMID)
		}
	}()
	r.mu.Lock()
	defer r.mu.Unlock()

	own, ok := r.ownerships[ownershipKey(userID, desktopID)]
	if !ok {
		return nil, fmt.Errorf("no VM found for user %s desktop %s", userID, normalizeDesktopID(desktopID))
	}

	if own.State != VMStateDegraded && own.State != VMStateFailed {
		return nil, fmt.Errorf("VM %s is not in a recoverable state (state=%s)", own.VMID, own.State)
	}

	// Delegate to the real VM manager if available.
	if r.vmManager != nil {
		info, err := r.vmManager.RecoverVM(own.VMID, vmManagerConfigForOwnership(own, ""))
		if err != nil {
			return nil, fmt.Errorf("failed to recover VM %s: %w", own.VMID, err)
		}
		own.SandboxURL = info.HostURL
		own.Epoch = info.Epoch
	} else {
		// Increment epoch on recovery — this is a fresh boot, not a resume.
		// The epoch change prevents duplicate canonical effects (VAL-CROSS-117).
		own.Epoch = r.nextEpoch()
	}

	own.State = VMStateActive
	own.LastActiveAt = time.Now()
	own.StoppedBy = ""
	r.saveLocked()
	log.Printf("vmctl: recovered VM %s for user %s desktop %s (new_epoch=%d, fresh-boot)", own.VMID, userID, own.DesktopID, own.Epoch)
	ensureVMID = own.VMID
	return own, nil
}

// RefreshVMForDesktop force-reboots a computer onto the current guest image
// while preserving persistent user data. This is for deploy-time image refresh
// and owner-scoped recovery from stale boot artifacts, not crash recovery.
func (r *OwnershipRegistry) RefreshVMForDesktop(userID, desktopID string) (*VMOwnership, error) {
	var ensureVMID string
	defer func() {
		if ensureVMID != "" {
			r.ensureExistingGatewayCredential(ensureVMID)
		}
	}()
	r.mu.Lock()
	defer r.mu.Unlock()

	own, ok := r.ownerships[ownershipKey(userID, desktopID)]
	if !ok {
		return nil, fmt.Errorf("no VM found for user %s desktop %s", userID, normalizeDesktopID(desktopID))
	}

	if own.State != VMStateActive &&
		own.State != VMStateBooting &&
		own.State != VMStateStopped &&
		own.State != VMStateHibernated &&
		own.State != VMStateDegraded &&
		own.State != VMStateFailed {
		return nil, fmt.Errorf("VM %s is not refreshable (state=%s)", own.VMID, own.State)
	}

	if r.vmManager != nil {
		var info *VMInstanceInfo
		var err error
		missingStoppedInstance := r.vmManager.GetVM(own.VMID) == nil &&
			(own.State == VMStateStopped || own.State == VMStateHibernated)
		if missingStoppedInstance {
			info, err = r.vmManager.BootVM(vmManagerConfigForOwnership(own, issueGatewayTokenAt(r.gatewayURL, own.VMID)))
		} else {
			info, err = r.vmManager.RefreshVM(own.VMID, vmManagerConfigForOwnership(own, ""))
		}
		if err != nil {
			return nil, fmt.Errorf("failed to refresh VM %s: %w", own.VMID, err)
		}
		own.SandboxURL = info.HostURL
		own.Epoch = info.Epoch
	} else {
		own.Epoch = r.nextEpoch()
	}

	own.State = VMStateActive
	own.LastActiveAt = time.Now()
	own.StoppedBy = ""
	r.saveLocked()
	log.Printf("vmctl: refreshed VM %s for user %s desktop %s (new_epoch=%d, deploy-image-refresh)", own.VMID, userID, own.DesktopID, own.Epoch)
	ensureVMID = own.VMID
	return own, nil
}

// LogoutVM handles VM lifecycle transition on user logout. It transitions
// only the current user's VM to stopped state (VAL-VM-008).
// Other users' VMs are not affected.
func (r *OwnershipRegistry) LogoutVM(userID string) error {
	return r.LogoutVMForDesktop(userID, PrimaryDesktopID)
}

func (r *OwnershipRegistry) LogoutVMForDesktop(userID, desktopID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	own, ok := r.ownerships[ownershipKey(userID, desktopID)]
	if !ok {
		return nil // no VM for this user, idempotent
	}

	// Delegate to the real VM manager if available.
	if r.vmManager != nil && (own.State == VMStateActive || own.State == VMStateDegraded) {
		_ = r.vmManager.StopVM(own.VMID)
	}

	own.State = VMStateStopped
	own.LastActiveAt = time.Now()
	own.StoppedBy = "logout"
	r.saveLocked()
	log.Printf("vmctl: stopped VM %s for user %s desktop %s (reason=logout)", own.VMID, userID, own.DesktopID)
	return nil
}

// CheckIdleVMs returns legacy user IDs for idle primary desktops only.
// Multi-desktop callers should use CheckIdleOwnerships.
func (r *OwnershipRegistry) CheckIdleVMs() []string {
	owns := r.CheckIdleOwnerships()
	idle := make([]string, 0, len(owns))
	for _, own := range owns {
		if own != nil && own.Kind == VMKindInteractive && own.DesktopID == PrimaryDesktopID {
			idle = append(idle, own.UserID)
		}
	}
	return idle
}

// CheckIdleOwnerships returns idle ownership records whose VMs have exceeded
// the idle timeout and should be stopped or hibernated.
func (r *OwnershipRegistry) CheckIdleOwnerships() []*VMOwnership {
	r.mu.RLock()
	if r.idleTimeout <= 0 {
		r.mu.RUnlock()
		return nil
	}

	idleTimeout := r.idleTimeout
	warmnessPolicy := r.warmnessPolicy
	pressureCfg := r.pressureReclaim
	sampler := r.pressureSampler
	ownerships := make([]*VMOwnership, 0, len(r.ownerships)+len(r.workerVMs))
	for _, own := range r.ownerships {
		ownerships = append(ownerships, cloneOwnership(own))
	}
	for _, own := range r.workerVMs {
		ownerships = append(ownerships, cloneOwnership(own))
	}
	r.mu.RUnlock()

	warmnessPolicy = normalizeWarmnessPolicyConfig(warmnessPolicy)
	pressureCfg = normalizePressureReclaimConfig(pressureCfg)
	var pressure HostPressureSample
	if warmnessPolicy.PrimaryKeepaliveMode == PrimaryKeepaliveModeUnderCapacity {
		if sampler == nil {
			sampler = sampleHostPressure
		}
		pressure = sampler(pressureCfg)
		annotatePressure(&pressure, pressureCfg)
	}

	now := time.Now()
	candidates := idleOwnershipCandidates(ownerships, warmnessPolicy, pressure, idleTimeout, now)
	idle := make([]*VMOwnership, 0, len(candidates))
	for _, candidate := range candidates {
		idle = append(idle, candidate.own)
	}
	return idle
}

// StopIdleVMs transitions all idle VMs to hibernated state.
// Returns the number of VMs that were stopped (VAL-VM-008).
func (r *OwnershipRegistry) StopIdleVMs() int {
	idleOwnerships := r.CheckIdleOwnerships()
	stopped := 0
	for _, own := range idleOwnerships {
		if own == nil {
			continue
		}
		var err error
		if own.Kind == VMKindWorker {
			err = r.HibernateWorker(own.WorkerID)
		} else {
			err = r.HibernateVMForDesktop(own.UserID, own.DesktopID)
		}
		if err == nil {
			stopped++
		}
	}
	return stopped
}

// WarmAlwaysOnDesktops resumes explicitly configured always-on primary
// desktops that already have an ownership record. It intentionally does not
// create new ownerships for configured users and does not warm candidate
// desktops or worker VMs.
func (r *OwnershipRegistry) WarmAlwaysOnDesktops() int {
	r.mu.RLock()
	cfg := normalizeWarmnessPolicyConfig(r.warmnessPolicy)
	if len(cfg.AlwaysOnUserIDs) == 0 {
		r.mu.RUnlock()
		return 0
	}
	type warmTarget struct {
		userID    string
		desktopID string
		vmID      string
	}
	targets := make([]warmTarget, 0)
	for _, own := range r.ownerships {
		if own == nil {
			continue
		}
		if own.Kind == VMKindWorker || own.DesktopID != PrimaryDesktopID || !own.Published {
			continue
		}
		if !cfg.AlwaysOnUserIDs[strings.TrimSpace(own.UserID)] {
			continue
		}
		if own.State != VMStateStopped && own.State != VMStateHibernated {
			continue
		}
		targets = append(targets, warmTarget{
			userID:    own.UserID,
			desktopID: own.DesktopID,
			vmID:      own.VMID,
		})
	}
	r.mu.RUnlock()

	warmed := 0
	for _, target := range targets {
		if _, err := r.ResumeVMForDesktop(target.userID, target.desktopID); err != nil {
			log.Printf("vmctl: warmness policy failed to resume always-on desktop vm=%s user=%s desktop=%s: %v", target.vmID, target.userID, target.desktopID, err)
			continue
		}
		warmed++
	}
	return warmed
}

// SetSandboxCredential stores the gateway credential for a VM's sandbox.
func (r *OwnershipRegistry) SetSandboxCredential(vmID, credential string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	own, ok := r.vmByID[vmID]
	if !ok {
		return fmt.Errorf("no VM found with ID %s", vmID)
	}

	own.SandboxCredential = credential
	return nil
}

// ActiveCount returns the number of active (booting or active) VMs.
func (r *OwnershipRegistry) ActiveCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, own := range r.ownerships {
		if own.IsReady() {
			count++
		}
	}
	for _, own := range r.workerVMs {
		if own.IsReady() {
			count++
		}
	}
	return count
}

// transitionVM transitions a VM to a new state.
func (r *OwnershipRegistry) transitionVM(vmID string, state VMState) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if own, ok := r.vmByID[vmID]; ok {
		own.State = state
		own.LastActiveAt = time.Now()
		r.saveLocked()
	}
}

// sandboxURLForVM generates the sandbox URL for a given VM ID.
// In production, this would resolve to the actual VM's network address.
// For host-process mode during development, all VMs route to the same
// host sandbox at the configured base URL.
func (r *OwnershipRegistry) sandboxURLForVM(vmID string) string {
	// In the current host-process mode, all VMs share the same sandbox
	// URL. When Firecracker is integrated, this will return per-VM URLs
	// based on the VM's assigned network address.
	return r.sandboxURLBase
}

// generateVMID creates a unique VM identifier.
func generateVMID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "vm-" + hex.EncodeToString(b)
}

func generateWorkerID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "worker-" + hex.EncodeToString(b)
}
