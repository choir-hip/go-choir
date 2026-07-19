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
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
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
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
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

// VMKind classifies persistent computer implementations.
type VMKind string

const (
	VMKindInteractive VMKind = "interactive"
)

// VMOwnership represents the assignment of a user to a specific VM.
type VMOwnership struct {
	// VMID is the unique identifier for the VM.
	VMID string `json:"vm_id"`
	// ComputerID is globally stable semantic identity. It survives realization
	// replacement and is never a VMID or a desktop selector.
	ComputerID string `json:"computer_id"`

	// UserID is the authenticated user who owns this VM.
	UserID string `json:"user_id"`

	// DesktopID is the stable workspace selector for this computer.
	DesktopID string `json:"desktop_id"`

	// Kind identifies the interactive computer implementation.
	Kind VMKind `json:"kind,omitempty"`

	// WarmnessClass is the typed lifecycle policy class for keepalive and
	// reclaim decisions. Public health exposes only aggregate counts.
	WarmnessClass WarmnessClass `json:"warmness_class,omitempty"`

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
	ComputerID        string
	RealizationID     string
	Epoch             int64
	KernelImagePath   string
	InitrdPath        string
	RootfsPath        string
	StoreDiskPath     string
	KernelParams      string
	GuestPort         int
	MachineCPUCount   int
	MachineMemSizeMib int
	PersistentDir     string
	// GatewayToken is the credential token for the sandbox to authenticate
	// to the host-side gateway. Written to the persistent directory so the
	// guest init script can read it and set RUNTIME_GATEWAY_TOKEN.
	GatewayToken               string
	ComputerCredentialEnvelope string
	ComputerKind               string
	OwnerID                    string
	DesktopID                  string
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
	retentionPrune              RetentionPruneConfig
	retentionShadowPrune        RetentionPruneConfig
	retentionShadowPruneEnabled bool
	retentionUserEmails         map[string]string

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

	// gatewayURL is the URL of the host-side gateway service. When set,
	// the registry issues gateway tokens for VM sandboxes before booting
	// so the guest sandbox can authenticate to the gateway.
	gatewayURL string
	corpusdURL string

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
		if own.ComputerID == own.VMID || own.ComputerID == own.DesktopID {
			own.ComputerID = ""
		}
		own.ComputerID = stableComputerID(own.UserID, own.DesktopID, own.ComputerID)
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
		r.ownerships[ownershipKey(own.UserID, own.DesktopID)] = ptr
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
	if err := r.writePersistenceLocked(); err != nil {
		log.Printf("vmctl: persist ownership registry: %v", err)
	}
}

func (r *OwnershipRegistry) writePersistenceLocked() error {
	if r.persistencePath == "" {
		return nil
	}
	ownerships := make([]*VMOwnership, 0, len(r.ownerships))
	for _, own := range r.ownerships {
		cp := *own
		ownerships = append(ownerships, &cp)
	}
	sort.Slice(ownerships, func(i, j int) bool {
		a, b := ownerships[i], ownerships[j]
		ak := string(a.Kind) + "|" + a.UserID + "|" + a.DesktopID + "|" + a.VMID
		bk := string(b.Kind) + "|" + b.UserID + "|" + b.DesktopID + "|" + b.VMID
		return ak < bk
	})
	state := persistedOwnershipState{SavedAt: time.Now().UTC(), EpochCounter: r.epochCounter, Ownerships: ownerships}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(r.persistencePath), 0o750); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	tmp := r.persistencePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o640); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	if err := os.Rename(tmp, r.persistencePath); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// SetVMManager sets the Firecracker VM lifecycle manager. When set, the
// registry delegates VM lifecycle operations to the manager instead of
// running in host-process sandbox mode. This activates real Firecracker
// VM lifecycle on Node B.
func (r *OwnershipRegistry) SetVMManager(mgr VMManager) {
	r.mu.Lock()
	r.vmManager = mgr
	r.mu.Unlock()
}

// ReattachManagedVMs adopts VM processes that survived vmctl restart only
// after their owner/computer D-ROUTE has been independently authorized.
func (r *OwnershipRegistry) ReattachManagedVMs(ctx context.Context, guard ComputerVersionRouteGuard) int {
	r.mu.RLock()
	mgr := r.vmManager
	candidates := make([]VMOwnership, 0)
	if mgr != nil {
		for _, own := range r.ownerships {
			if own.State == VMStateStopped && own.StoppedBy == "vmctl-restart" && strings.TrimSpace(own.SandboxURL) != "" {
				candidates = append(candidates, *own)
			}
		}
	}
	r.mu.RUnlock()

	reattached := 0
	for _, own := range candidates {
		if guard == nil {
			log.Printf("vmctl: reattach refused for VM %s: ComputerVersion route guard is unavailable", own.VMID)
			continue
		}
		if err := guard(ctx, own.UserID, normalizeDesktopID(own.DesktopID)); err != nil {
			log.Printf("vmctl: reattach refused for VM %s: %v", own.VMID, err)
			continue
		}
		info, err := mgr.ReattachVM(own.VMID, own.SandboxURL, own.Epoch)
		if err != nil {
			log.Printf("vmctl: reattach skipped for VM %s: %v", own.VMID, err)
			continue
		}
		r.mu.Lock()
		if cur, ok := r.vmByID[own.VMID]; ok {
			cur.State = VMStateActive
			cur.SandboxURL = info.HostURL
			cur.Epoch = info.Epoch
			if cur.LastActiveAt.IsZero() {
				cur.LastActiveAt = time.Now()
			}
			cur.StoppedBy = ""
			r.saveLocked()
			reattached++
		}
		r.mu.Unlock()
	}
	if reattached > 0 {
		go r.ReconcileReadyGatewayCredentials()
	}
	return reattached
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

// SetCorpusdURL configures the platform event service used to mint a
// realization-bound guest bootstrap envelope before each fresh primary boot.
func (r *OwnershipRegistry) SetCorpusdURL(url string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.corpusdURL = strings.TrimSpace(url)
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

// SetRetentionShadowPruneConfig configures a dry-run-only retention policy for
// operator visibility. It is intentionally separate from the active prune
// policy so broader candidate classes can be observed before they are allowed
// to delete VM state.
func (r *OwnershipRegistry) SetRetentionShadowPruneConfig(cfg RetentionPruneConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cfg = normalizeRetentionPruneConfig(cfg)
	if cfg.Mode != RetentionPruneModeOff {
		cfg.Mode = RetentionPruneModeDryRun
	}
	r.retentionShadowPrune = cfg
	r.retentionShadowPruneEnabled = true
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
type ComputerVersionRouteGuard func(context.Context, string, string) error

func authorizeLifecycleRoute(ctx context.Context, guard ComputerVersionRouteGuard, userID, desktopID string) bool {
	if guard == nil || strings.TrimSpace(userID) == "" || strings.TrimSpace(desktopID) == "" {
		return false
	}
	if err := guard(ctx, userID, desktopID); err != nil {
		log.Printf("vmctl: lifecycle mutation refused without D-ROUTE user=%s desktop=%s: %v", userID, desktopID, err)
		return false
	}
	return true
}

func (r *OwnershipRegistry) StartIdleSweeper(ctx context.Context, interval time.Duration, guard ComputerVersionRouteGuard) {
	if interval <= 0 {
		interval = time.Minute
	}
	var sweepMu sync.Mutex
	sweep := func() {
		sweepMu.Lock()
		defer sweepMu.Unlock()
		if warmed := r.WarmAlwaysOnDesktops(ctx, guard); warmed > 0 {
			log.Printf("vmctl: warmness policy resumed %d always-on desktop VM(s)", warmed)
		}
		if warmed := r.WarmUniversalWirePlatformComputer(ctx, guard); warmed > 0 {
			log.Printf("vmctl: warmness policy resumed %d universal wire platform computer(s)", warmed)
		}
		if plan := r.PressureReclaimPlan(); plan.Mode == PressureReclaimModeDryRun {
			log.Printf("vmctl: pressure reclaim dry-run decision=%s reason=%q active=%d eligible=%d protected=%d pressure=%v",
				plan.Decision, plan.Reason, plan.Inventory.Active, plan.Inventory.Eligible, plan.Inventory.Protected, plan.Pressure.Pressure)
		} else if plan.Mode == PressureReclaimModeActive {
			log.Printf("vmctl: pressure reclaim active decision=%s reason=%q active=%d eligible=%d protected=%d pressure=%v",
				plan.Decision, plan.Reason, plan.Inventory.Active, plan.Inventory.Eligible, plan.Inventory.Protected, plan.Pressure.Pressure)
			if reclaimed := r.ReclaimPressureVMs(ctx, guard); reclaimed > 0 {
				log.Printf("vmctl: pressure reclaim hibernated %d VM(s)", reclaimed)
			}
		}
		if result := r.PruneRetention(ctx, guard); result.Deleted > 0 {
			log.Printf("vmctl: retention prune deleted %d VM state directorie(s), reclaimed %.1f MiB", result.Deleted, float64(result.BytesDeleted)/(1024*1024))
		}
		if stopped := r.StopIdleVMs(ctx, guard); stopped > 0 {
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

func machineShapeForOwnership(own *VMOwnership) (int, int) {
	if own != nil && isPlatformOwnership(own) {
		return platformVMCPUCount, platformVMMemSizeMib
	}
	return interactiveVMCPUCount, interactiveVMMemSizeMib
}

func isPlatformOwnership(own *VMOwnership) bool {
	if own == nil {
		return false
	}
	return own.WarmnessClass == WarmnessClassPublicPlatform ||
		(own.UserID == UniversalWirePlatformOwnerID && normalizeDesktopID(own.DesktopID) == UniversalWirePlatformDesktopID)
}

func computerKindForOwnership(own *VMOwnership) string {
	if own == nil {
		return "active"
	}
	if isPlatformOwnership(own) {
		return "platform"
	}
	return "active"
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

func (r *OwnershipRegistry) freshVMConfig(own *VMOwnership, gatewayToken string) VMManagerConfig {
	cfg := vmManagerConfigForOwnership(own, gatewayToken)
	cfg.Epoch = own.Epoch + 1
	cfg.RealizationID = realizationIDFor(own.VMID, cfg.Epoch)
	cfg.ComputerCredentialEnvelope = r.issueComputerCredentialEnvelope(cfg.ComputerID, cfg.RealizationID, cfg.Epoch)
	return cfg
}
func freshVMConfigWithCredentialIssuer(own *VMOwnership, gatewayToken, corpusdURL string) VMManagerConfig {
	cfg := vmManagerConfigForOwnership(own, gatewayToken)
	cfg.Epoch = own.Epoch + 1
	cfg.RealizationID = realizationIDFor(own.VMID, cfg.Epoch)
	cfg.ComputerCredentialEnvelope = issueComputerCredentialEnvelope(corpusdURL, cfg.ComputerID, cfg.RealizationID, cfg.Epoch)
	return cfg
}

func (r *OwnershipRegistry) recoverOrRestartActiveVM(own *VMOwnership, mgr VMManager) (*VMInstanceInfo, error) {
	if mgr.GetVM(own.VMID) == nil {
		return r.startExistingVM(own, mgr)
	}
	recovered, err := mgr.RecoverVM(own.VMID, r.freshVMConfig(own, ""))
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
	if mgr.GetVM(own.VMID) != nil {
		return mgr.RecoverVM(own.VMID, r.freshVMConfig(own, ""))
	}
	cfg := r.freshVMConfig(own, r.issueGatewayToken(own.VMID))
	return mgr.BootVM(cfg)
}

func vmManagerConfigForOwnership(own *VMOwnership, gatewayToken string) VMManagerConfig {
	if own == nil {
		return VMManagerConfig{}
	}
	cpu, mem := machineShapeForOwnership(own)
	cfg := VMManagerConfig{
		VMID:              own.VMID,
		ComputerID:        stableComputerID(own.UserID, own.DesktopID, own.ComputerID),
		GuestPort:         8085,
		MachineCPUCount:   cpu,
		MachineMemSizeMib: mem,
		GatewayToken:      gatewayToken,
		ComputerKind:      computerKindForOwnership(own),
		OwnerID:           own.UserID,
		DesktopID:         own.DesktopID,
		RealizationID:     realizationIDFor(own.VMID, own.Epoch),
		Epoch:             own.Epoch,
	}
	return cfg
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

func (r *OwnershipRegistry) platformControlPublicKey(ctx context.Context, signer computerevent.SignerRef) (ed25519.PublicKey, error) {
	r.mu.RLock()
	corpusdURL := r.corpusdURL
	r.mu.RUnlock()
	if corpusdURL == "" || signer.SignerDomain != "platform-control" || signer.KeyID == "" {
		return nil, fmt.Errorf("vmctl: platform control key authority unavailable")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(corpusdURL, "/")+"/internal/platform/control-key", nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-Internal-Caller", "true")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var result struct {
		SignerDomain string `json:"signer_domain"`
		KeyID        string `json:"key_id"`
		PublicKey    string `json:"public_key"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&result) != nil || result.SignerDomain != signer.SignerDomain || result.KeyID != signer.KeyID {
		return nil, fmt.Errorf("vmctl: platform control key response refused")
	}
	publicKey, err := base64.RawStdEncoding.DecodeString(result.PublicKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("vmctl: platform control public key invalid")
	}
	return ed25519.PublicKey(publicKey), nil
}

func (r *OwnershipRegistry) issueComputerCredentialEnvelope(computerID, realizationID string, epoch int64) string {
	r.mu.RLock()
	corpusdURL := r.corpusdURL
	r.mu.RUnlock()
	return issueComputerCredentialEnvelope(corpusdURL, computerID, realizationID, epoch)
}

func issueComputerCredentialEnvelope(corpusdURL, computerID, realizationID string, epoch int64) string {
	if corpusdURL == "" || computerID == "" || realizationID == "" {
		return ""
	}
	body, _ := json.Marshal(map[string]string{
		"computer_id":     computerID,
		"realization_id":  realizationID,
		"idempotency_key": fmt.Sprintf("guest-credential:%s:%d", realizationID, epoch),
	})
	ctx, cancel := context.WithTimeout(context.Background(), gatewayCredentialRequestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(corpusdURL, "/")+"/internal/computers/credentials/issue", strings.NewReader(string(body)))
	if err != nil {
		return ""
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Internal-Caller", "true")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		log.Printf("vmctl: computer credential request failed for %s: %v", realizationID, err)
		return ""
	}
	defer response.Body.Close()
	var result struct {
		Envelope json.RawMessage `json:"envelope"`
	}
	if response.StatusCode != http.StatusCreated || json.NewDecoder(io.LimitReader(response.Body, 128<<10)).Decode(&result) != nil || len(result.Envelope) == 0 {
		log.Printf("vmctl: computer credential request refused for %s", realizationID)
		return ""
	}
	var envelope any
	if json.Unmarshal(result.Envelope, &envelope) != nil {
		return ""
	}
	canonical, err := computerevent.CanonicalJSON(envelope)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(canonical)
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
	platformVMCPUCount                     = 2
	platformVMMemSizeMib                   = 4096
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
	vmIDs := make([]string, 0, len(r.ownerships))
	for _, own := range r.ownerships {
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
				result := cloneOwnership(current)
				r.mu.Unlock()
				r.ensureExistingGatewayCredential(vmID)
				return result, nil
			}

			own.LastActiveAt = time.Now()
			r.saveLocked()
			vmID := own.VMID
			result := cloneOwnership(own)
			r.mu.Unlock()
			r.ensureExistingGatewayCredential(vmID)
			return result, nil
		}

		// VM exists but is stopped or hibernated. Resume it instead
		// of creating a new VM, preserving the user's state and epoch
		// (VAL-CROSS-116, VAL-CROSS-117).
		if own.State == VMStateStopped || own.State == VMStateHibernated {
			if waiters, ok := r.pendingWaiters[key]; ok {
				return r.waitForPendingAssignmentLocked(ctx, key, userID, desktopID, waiters)
			}
			mgr := r.vmManager
			pending := cloneOwnership(own)
			r.pendingWaiters[key] = nil
			r.mu.Unlock()

			var info *VMInstanceInfo
			var err error
			if mgr != nil {
				info, err = r.startExistingVM(pending, mgr)
				if err != nil {
					log.Printf("vmctl: start existing VM %s failed: %v", pending.VMID, err)
					r.mu.Lock()
					waiters := r.pendingWaiters[key]
					delete(r.pendingWaiters, key)
					r.mu.Unlock()
					for _, ch := range waiters {
						ch <- nil
					}
					return nil, fmt.Errorf("failed to start existing VM %s: %w", pending.VMID, err)
				}
			}

			r.mu.Lock()
			current := r.ownerships[key]
			if current == nil || current.VMID != pending.VMID {
				waiters := r.pendingWaiters[key]
				delete(r.pendingWaiters, key)
				r.mu.Unlock()
				for _, ch := range waiters {
					ch <- nil
				}
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
			waiters := r.pendingWaiters[key]
			delete(r.pendingWaiters, key)
			result := cloneOwnership(current)
			r.mu.Unlock()
			r.ensureExistingGatewayCredential(result.VMID)
			for _, ch := range waiters {
				ch <- cloneOwnership(result)
			}
			log.Printf("vmctl: resumed VM %s for user %s desktop %s on resolve (epoch=%d)", result.VMID, userID, desktopID, result.Epoch)
			return result, nil
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
		VMID:       vmID,
		UserID:     userID,
		DesktopID:  desktopID,
		ComputerID: stableComputerID(userID, desktopID, ""),
		Kind:       VMKindInteractive,
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
			VMID:                       vmID,
			ComputerID:                 own.ComputerID,
			RealizationID:              realizationIDFor(vmID, epoch),
			Epoch:                      epoch,
			GuestPort:                  8085,
			MachineCPUCount:            interactiveVMCPUCount,
			MachineMemSizeMib:          interactiveVMMemSizeMib,
			GatewayToken:               gwToken,
			ComputerCredentialEnvelope: r.issueComputerCredentialEnvelope(own.ComputerID, realizationIDFor(vmID, epoch), epoch),
			ComputerKind:               "active",
			OwnerID:                    userID,
			DesktopID:                  desktopID,
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
	result := cloneOwnership(own)
	r.mu.Unlock()

	for _, ch := range waiters {
		ch <- cloneOwnership(result)
	}

	log.Printf("vmctl: assigned VM %s to user %s desktop %s", vmID, userID, desktopID)

	return result, nil
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
		return cloneOwnership(own), nil
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

// GetOwnership returns the current ownership for a user's primary desktop, or
// nil if none exists.
func (r *OwnershipRegistry) GetOwnership(userID string) *VMOwnership {
	return r.GetOwnershipForDesktop(userID, PrimaryDesktopID)
}

// GetOwnershipForDesktop returns the current ownership for a specific
// user/desktop pair, or nil if none exists.
//
// It returns a snapshot copy of the ownership taken under the read lock.
// Returning the live map pointer would let callers read mutable fields (e.g.
// State) outside the lock while another goroutine mutates them under r.mu
// (such as the idle sweeper calling hibernateVMForDesktopWithReason), which
// is a data race. The copy is safe because VMOwnership holds only value
// fields, and no caller mutates the returned pointer.
func (r *OwnershipRegistry) GetOwnershipForDesktop(userID, desktopID string) *VMOwnership {
	r.mu.RLock()
	defer r.mu.RUnlock()
	own, ok := r.ownerships[ownershipKey(userID, desktopID)]
	if !ok || own == nil {
		return nil
	}
	snap := *own
	return &snap
}

func (r *OwnershipRegistry) GetOwnershipForComputer(userID, computerID string) *VMOwnership {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, own := range r.ownerships {
		if own != nil && own.UserID == userID && stableComputerID(own.UserID, own.DesktopID, own.ComputerID) == computerID {
			snapshot := *own
			return &snapshot
		}
	}
	return nil
}

// LiveSandboxURL returns the live sandbox URL for the given user/desktop pair.
// It prefers the VM manager's live HostURL over the cached ownership record.
// Returns an error if the ownership does not exist and no live VM is found.
func (r *OwnershipRegistry) LiveSandboxURL(userID, desktopID string) (string, error) {
	r.mu.RLock()
	own, ok := r.ownerships[ownershipKey(userID, desktopID)]
	mgr := r.vmManager
	var snap VMOwnership
	found := ok && own != nil
	if found {
		snap = *own
	}
	r.mu.RUnlock()
	if found {
		// Prefer live VM manager URL if available.
		if mgr != nil {
			if info := mgr.GetVM(snap.VMID); info != nil && strings.TrimSpace(info.HostURL) != "" {
				return info.HostURL, nil
			}
		}
		if strings.TrimSpace(snap.SandboxURL) != "" {
			return snap.SandboxURL, nil
		}
	}
	return "", fmt.Errorf("no live sandbox URL for %s/%s", userID, desktopID)
}

// GetOwnershipByVMID returns the ownership for a specific VM ID, or nil.
//
// It returns a snapshot copy taken under the read lock for the same
// concurrency-safety reason as GetOwnershipForDesktop (see its comment).
func (r *OwnershipRegistry) GetOwnershipByVMID(vmID string) *VMOwnership {
	r.mu.RLock()
	defer r.mu.RUnlock()
	own, ok := r.vmByID[vmID]
	if !ok || own == nil {
		return nil
	}
	snap := *own
	return &snap
}

// ListOwnerships returns all current ownerships.
//
// It returns snapshot copies taken under the read lock for the same
// concurrency-safety reason as GetOwnershipForDesktop (see its comment).
func (r *OwnershipRegistry) ListOwnerships() []*VMOwnership {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*VMOwnership, 0, len(r.ownerships))
	for _, own := range r.ownerships {
		snap := *own
		result = append(result, &snap)
	}
	return result
}

// WarmnessSummary returns redacted aggregate lifecycle policy state. It never
// includes user IDs, VM IDs, desktop IDs, or credentials.
func (r *OwnershipRegistry) WarmnessSummary(idleEligible []*VMOwnership) WarmnessHealthSummary {
	r.mu.RLock()
	cfg := r.warmnessPolicy
	ownerships := make([]*VMOwnership, 0, len(r.ownerships))
	for _, own := range r.ownerships {
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

	// Every product start is a fresh realization. Propagate actuator failure;
	// never project a stopped state that vmctl did not observe.
	if r.vmManager != nil && (own.State == VMStateActive || own.State == VMStateDegraded) {
		if err := r.vmManager.StopVM(own.VMID); err != nil {
			return fmt.Errorf("stop VM %s: %w", own.VMID, err)
		}
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

func normalizeStopReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "idle"
	}
	return reason
}

// ReclaimPressureVMs hibernates bounded eligible computers when active
// pressure reclaim is enabled and the host is currently under pressure.
func (r *OwnershipRegistry) ReclaimPressureVMs(ctx context.Context, guard ComputerVersionRouteGuard) int {
	candidates := r.pressureReclaimActionCandidates()
	reclaimed := 0
	for _, candidate := range candidates {
		if candidate.own == nil || candidate.public.Protected || !authorizeLifecycleRoute(ctx, guard, candidate.own.UserID, candidate.own.DesktopID) {
			continue
		}
		err := r.hibernateVMForDesktopWithReason(candidate.own.UserID, candidate.own.DesktopID, "pressure")
		if err == nil {
			reclaimed++
		}
	}
	return reclaimed
}

// ResumeVM starts a stopped or hibernated computer as a fresh disposable
// realization while preserving its persistent user state. The epoch advances
// and a new realization-bound credential envelope is issued.
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

	// Transition the stable computer to its newly booted disposable realization.
	own.State = VMStateActive
	own.LastActiveAt = time.Now()
	own.StoppedBy = ""
	r.saveLocked()
	log.Printf("vmctl: started VM %s for user %s desktop %s (epoch=%d)", own.VMID, userID, own.DesktopID, own.Epoch)
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
	r.mu.RLock()
	corpusdURL := r.corpusdURL
	r.mu.RUnlock()
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
		info, err := r.vmManager.RecoverVM(own.VMID, freshVMConfigWithCredentialIssuer(own, "", corpusdURL))
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
	ownerships := make([]*VMOwnership, 0, len(r.ownerships))
	for _, own := range r.ownerships {
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
func (r *OwnershipRegistry) StopIdleVMs(ctx context.Context, guard ComputerVersionRouteGuard) int {
	idleOwnerships := r.CheckIdleOwnerships()
	stopped := 0
	for _, own := range idleOwnerships {
		if own == nil || !authorizeLifecycleRoute(ctx, guard, own.UserID, own.DesktopID) {
			continue
		}
		err := r.HibernateVMForDesktop(own.UserID, own.DesktopID)
		if err == nil {
			stopped++
		}
	}
	return stopped
}

// WarmAlwaysOnDesktops resumes configured primary computers that already have
// an ownership record; it never creates ownerships.
func (r *OwnershipRegistry) WarmAlwaysOnDesktops(ctx context.Context, guard ComputerVersionRouteGuard) int {
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
		if own.DesktopID != PrimaryDesktopID {
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
		if guard == nil {
			log.Printf("vmctl: warmness policy refused always-on desktop vm=%s: ComputerVersion route guard is unavailable", target.vmID)
			continue
		}
		if err := guard(ctx, target.userID, target.desktopID); err != nil {
			log.Printf("vmctl: warmness policy refused always-on desktop vm=%s user=%s desktop=%s: %v", target.vmID, target.userID, target.desktopID, err)
			continue
		}
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

func stableComputerID(userID, desktopID, existing string) string {
	existing = strings.TrimSpace(existing)
	if strings.HasPrefix(existing, "computer-") && len(existing) >= len("computer-")+6 && len(existing) <= 80 &&
		!strings.ContainsAny(existing, "/\\ \t\r\n\x00") && existing != normalizeDesktopID(desktopID) {
		return existing
	}
	return "computer-" + computerevent.DigestBytes([]byte(strings.TrimSpace(userID) + "\x00" + normalizeDesktopID(desktopID)))[:32]
}

func realizationIDFor(vmID string, epoch int64) string {
	return fmt.Sprintf("%s-epoch-%d", strings.TrimSpace(vmID), epoch)
}
