// Package vmctl implements the VM ownership registry and lifecycle control
// for Mission 3. The registry maps authenticated users to VM-backed autoputer
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
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/hostauth"
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

	// ComputerURL is the URL where this VM's autoputer runtime is reachable.
	ComputerURL string `json:"computer_url"`

	// State is the current lifecycle state of the VM.
	State VMState `json:"state"`

	// CreatedAt is when the VM was first created.
	CreatedAt time.Time `json:"created_at"`

	// LastActiveAt is when the VM was last used.
	LastActiveAt time.Time `json:"last_active_at"`

	// AutoputerCredential is the credential issued by the gateway for this VM.
	// It is used to authenticate autoputer-to-gateway provider requests.
	AutoputerCredential string `json:"-"`

	// Epoch is the monotonically increasing boot counter for this VM.
	// On fresh boot or recovery, the epoch increments. On resume from
	// hibernate, the epoch stays the same. Callers can use epoch to
	// detect whether a VM went through a fresh boot vs. a resume,
	// which prevents duplicate canonical effects (VAL-CROSS-117).
	Epoch int64 `json:"epoch"`

	// StoppedBy indicates why the VM was stopped. Empty if running.
	// Values include "idle", "logout", "recovery", "manual", "wedged",
	// "recovery_failed", "vmctl-restart" (running at a vmctl restart) and
	// "interrupted-by-vmctl-restart" (a start in flight at one).
	StoppedBy string `json:"stopped_by,omitempty"`
	// BootClaim is the durable record of a start whose result is not yet
	// published, written before the manager spawns (boot_claim.go). A vmctl
	// that loads a row with an open claim closes the start; it never adopts
	// it.
	BootClaim *BootClaim `json:"boot_claim,omitempty"`
	// StartInterruptions counts the consecutive starts of the computer a
	// vmctl restart interrupted; a publish resets it. The second is
	// stoppedByStartInterruptedRepeatedly (boot_claim.go).
	StartInterruptions int `json:"start_interruptions,omitempty"`
	// FailureWarmedAt is when the always-on sweep last spent its automatic
	// retry on this computer after a wedged or recovery_failed stop. Interim
	// bound until the boot ledger (restart station L1b).
	FailureWarmedAt string `json:"failure_warmed_at,omitempty"`
	// DeployGrants are the deploy targets that already re-armed this
	// computer's automatic failure retry (GrantDeployRetry), oldest first,
	// each kept for at least failureWarmWindow. A deploy of a different build
	// is an external event with a fresh allowance of one retry; a target in
	// the set never grants again, so A, B, ..., A re-arms A once per window.
	DeployGrants []deployGrant `json:"deploy_grants,omitempty"`
	// failureWarmGen counts the changes to FailureWarmedAt (spend, grant) in
	// this process, so a refund only restores the spend it belongs to.
	failureWarmGen uint64
	// HoldStatus, when non-nil, places the computer under a durable
	// host-authoritative maintenance hold. vmctl refuses any automatic
	// lifecycle action (boot/recover/refresh/reattach/resume/reclaim/
	// retention) while held, and the deploy active-VM refresh skips it.
	// It is set and cleared only by an authorized owner/recovery operation.
	HoldStatus *MaintenanceHold `json:"hold_status,omitempty"`

	// DivergenceStatus classifies the computer's divergence from platform baseline:
	// "tracking" (clean baseline, can auto-fast-forward non-breaking platform updates)
	// "divergent" (has diverged ledgers/packages; updates require rebase/proposal)
	// "canary" (derived display label for a constructed computer; it does not
	// gate deploy refresh — docs/problems/owner-computer-pinned-as-canary-2026-10-13.md)
	DivergenceStatus string `json:"divergence_status,omitempty"`

	// PlatformBaseRef is the base platform release commit or version this computer
	// was provisioned from or last cleanly rebased against.
	PlatformBaseRef string `json:"platform_base_ref,omitempty"`

	// MemoryPeakRSSBytes is the highest Firecracker resident memory vmctl
	// measured during this computer's most recent start (boot or wake).
	// Admission estimates the next start from it (host_admission.go).
	MemoryPeakRSSBytes uint64 `json:"memory_peak_rss_bytes,omitempty"`
	// MemoryMeasuredAt is when MemoryPeakRSSBytes was last sampled.
	MemoryMeasuredAt string `json:"memory_measured_at,omitempty"`
	// MemorySettledRSSBytes is the Firecracker resident memory the computer's
	// most recent started boot or wake had when its admission settle window
	// ended (density D4). Zero is unmeasured, never zero need; admission with
	// VMCTL_ADMISSION_NEED=wake charges it (host_admission.go).
	MemorySettledRSSBytes uint64 `json:"memory_settled_rss_bytes,omitempty"`
	// MemorySettledAt is when MemorySettledRSSBytes was captured.
	MemorySettledAt string `json:"memory_settled_at,omitempty"`
	// ReportedRelease is what the computer last reported it runs (P6):
	// a report, never vmctl authority (reported_release.go).
	ReportedRelease *ReportedRelease `json:"reported_release,omitempty"`
}

// IsReady returns true if the VM is in a state that can serve routed requests.
func (o *VMOwnership) IsReady() bool {
	return o.State == VMStateActive
}

// MaintenanceHold is the host-authoritative maintenance hold for a computer.
// While set, vmctl refuses automatic lifecycle action for the VM.
type MaintenanceHold struct {
	// Reason is why the computer is held (e.g. maintenance, recovery).
	Reason string `json:"reason,omitempty"`
	// HeldBy is the authorization authority that set the hold.
	HeldBy string `json:"held_by,omitempty"`
	// HeldAt is when the hold was set.
	HeldAt time.Time `json:"held_at,omitempty"`
}

// IsHeld returns true when the computer is under a maintenance hold.
func (o *VMOwnership) IsHeld() bool {
	return o != nil && o.HoldStatus != nil
}

// byComputerIDLocked returns the ownership for a stable computer ID, or nil.
// The caller must hold r.mu.
func (r *OwnershipRegistry) byComputerIDLocked(computerID string) *VMOwnership {
	for _, own := range r.ownerships {
		if own.ComputerID == computerID {
			return own
		}
	}
	return nil
}

// SetHold places a computer under a durable host-authoritative maintenance
// hold. While held, vmctl refuses any automatic lifecycle action
// (boot/recover/refresh/reattach/resume/reclaim/retention) for the VM and the
// deploy active-VM refresh skips it. This is an authorized owner/recovery
// operation; it is NOT called by the deployment refresh loop or the self-drive
// reconciler. Idempotent for the same reason/heldBy.
func (r *OwnershipRegistry) SetHold(computerID, reason, heldBy string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	own := r.byComputerIDLocked(computerID)
	if own == nil {
		return fmt.Errorf("vmctl: no ownership for computer %s", computerID)
	}
	now := time.Now().UTC()
	if own.HoldStatus == nil {
		own.HoldStatus = &MaintenanceHold{}
	} else if own.HoldStatus.HeldBy == heldBy && own.HoldStatus.Reason == reason {
		return nil // idempotent
	}
	own.HoldStatus.Reason = reason
	own.HoldStatus.HeldBy = heldBy
	own.HoldStatus.HeldAt = now
	return r.writePersistenceLocked()
}

// keyTouchMinInterval is the floor between two registry writes a touch makes
// for one computer: a client polling every second costs one write a minute.
const keyTouchMinInterval = 60 * time.Second

// TouchResult is what TouchComputerActivity found and did.
type TouchResult struct {
	Found   bool
	Touched bool
	State   VMState
}

// TouchComputerActivity moves the idle clock (LastActiveAt) of the computer a
// (user, computer) pair names, and nothing else. It is the write the
// authenticated API-key path lacks: that path joins a key to an ownership
// without resolving it, so the idle sweep never saw its traffic
// (docs/problems/idle-sweep-hibernates-a-computer-just-after-its-apply-restart-2026-10-24.md).
//
// It never resolves, wakes, starts or boots. Only an Active ownership is
// touched; any other state is reported and left as it was. A stored value
// younger than keyTouchMinInterval is left alone.
func (r *OwnershipRegistry) TouchComputerActivity(userID, computerID string, now time.Time) TouchResult {
	userID, computerID = strings.TrimSpace(userID), strings.TrimSpace(computerID)
	if userID == "" || computerID == "" {
		return TouchResult{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, own := range r.ownerships {
		if own == nil || own.UserID != userID || stableComputerID(own.UserID, own.DesktopID, own.ComputerID) != computerID {
			continue
		}
		result := TouchResult{Found: true, State: own.State}
		if own.State != VMStateActive {
			return result
		}
		if !own.LastActiveAt.IsZero() && now.Sub(own.LastActiveAt) < keyTouchMinInterval {
			return result
		}
		own.LastActiveAt = now
		r.saveLocked()
		result.Touched = true
		return result
	}
	return TouchResult{}
}

// ClearHold removes a maintenance hold from a computer. This is an authorized
// owner/recovery operation; it is NOT called by any automatic reconciler.
func (r *OwnershipRegistry) ClearHold(computerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	own := r.byComputerIDLocked(computerID)
	if own == nil {
		return fmt.Errorf("vmctl: no ownership for computer %s", computerID)
	}
	own.HoldStatus = nil
	return r.writePersistenceLocked()
}

// SetDivergenceStatus records the divergence classification ("tracking", "divergent", "canary")
// and the platform base reference for a computer.
func (r *OwnershipRegistry) SetDivergenceStatus(computerID, status, platformBaseRef string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	own := r.byComputerIDLocked(computerID)
	if own == nil {
		return fmt.Errorf("vmctl: no ownership for computer %s", computerID)
	}
	own.DivergenceStatus = status
	if platformBaseRef != "" {
		own.PlatformBaseRef = platformBaseRef
	}
	return r.writePersistenceLocked()
}

// VMManager is the interface the OwnershipRegistry uses to manage real
// Firecracker VM lifecycles. When Firecracker is available on the host,
// the registry delegates VM boot/stop/resume/recover operations to the
// concrete vmmanager.Manager. When Firecracker is not available, the
// registry runs in host-process mode with no-op VM lifecycle calls.

type VMManager interface {
	// ReserveBootEpoch durably allocates the realization epoch before vmctl
	// mints any credential bound to that boot attempt.
	ReserveBootEpoch(vmID string, minimum int64) (int64, error)

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

// snapshotResumer is the optional S3 capability of a manager that hibernates
// computers into a Firecracker memory snapshot
// (docs/reports/s3-snapshot-hibernate-design-2026-10-14.md). A successful
// resume returns the snapshot's own realization: same epoch, no restart.
// issueCredential (nil when vmctl has no corpusd) mints a fresh envelope for
// that same realization; the manager calls it only for a snapshot it will
// load and delivers it to the guest in the host-only resumed call. Any error
// means the snapshot was not used (and is gone); the caller cold boots,
// which is a crash boot.
type snapshotResumer interface {
	ResumeVMFromSnapshot(vmID string, cfg VMManagerConfig, issueCredential func() (string, error)) (*VMInstanceInfo, error)
}

// snapshotDiscarder is the optional S3 capability that drops memory
// snapshots a computer can no longer use (design note items 44-47). A
// stopped computer cold boots, so its snapshot is dead weight that also
// holds a host-port reservation. Neither call blocks on a VM mid-operation.
type snapshotDiscarder interface {
	// DiscardSnapshot removes vmID's snapshot; true when files were removed.
	DiscardSnapshot(vmID, reason string) bool
	// SweepOrphanSnapshots removes every snapshot whose VM keep reports not
	// hibernated, plus interrupted captures, and returns the removed paths.
	SweepOrphanSnapshots(keep func(vmID string) bool) []string
}

// discardStoppedSnapshotLocked drops the memory snapshot of a computer that
// is now stopped. It must not rely on the manager's StopVM: that is not
// called for a hibernated computer the manager no longer tracks (after a
// vmctl restart) and returns early for one it does. Callers hold r.mu; the
// discard never waits on the manager's per-VM operation lock.
func (r *OwnershipRegistry) discardStoppedSnapshotLocked(own *VMOwnership, reason string) {
	if own == nil || r.vmManager == nil {
		return
	}
	if discarder, ok := r.vmManager.(snapshotDiscarder); ok && discarder != nil {
		if discarder.DiscardSnapshot(own.VMID, reason) {
			log.Printf("vmctl: discarded memory snapshot of stopped computer %s (VM %s, %s)", own.ComputerID, own.VMID, reason)
		}
	}
}

// SweepOrphanSnapshots runs once at vmctl start, after the ownership
// registry is loaded and the manager is wired: it removes the memory
// snapshot of every VM whose ownership is not hibernated (held or not), and
// interrupted captures. It refuses to run unless ownerships were read from
// the durable file (design note item 46).
func (r *OwnershipRegistry) SweepOrphanSnapshots() []string {
	r.mu.RLock()
	mgr := r.vmManager
	loaded := r.persistenceLoaded
	hibernated := make(map[string]bool, len(r.ownerships))
	for _, own := range r.ownerships {
		if own != nil && own.State == VMStateHibernated {
			hibernated[own.VMID] = true
		}
	}
	r.mu.RUnlock()
	discarder, ok := mgr.(snapshotDiscarder)
	if !ok || discarder == nil {
		return nil
	}
	if !loaded {
		log.Printf("vmctl: snapshot sweep skipped: ownerships were not loaded from a durable registry file")
		return nil
	}
	removed := discarder.SweepOrphanSnapshots(func(vmID string) bool { return hibernated[vmID] })
	log.Printf("vmctl: snapshot sweep removed %d orphaned snapshot path(s); kept %d hibernated computer(s)", len(removed), len(hibernated))
	return removed
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
	// GatewayToken is the credential token for the autoputer to authenticate
	// to the host-side gateway. Written to the persistent directory so the
	// guest init script can read it and set RUNTIME_GATEWAY_TOKEN.
	GatewayToken               string
	ComputerCredentialEnvelope string
	ComputerKind               string
	OwnerID                    string
	DesktopID                  string
	// MaintenanceHold boots the guest under the guest-visible maintenance
	// hold (RUNTIME_MAINTENANCE_HOLD=1): run admission refused, no rewake.
	MaintenanceHold bool
	// RecoveryReplayOnly boots the guest into the B14 one-shot replay drive
	// (RUNTIME_RECOVERY_REPLAY_ONLY=1): materialize to head, then exit
	// without starting the runtime or appending.
	RecoveryReplayOnly bool

	// BootKind labels the boot attempt for the per-boot timeline receipt
	// (cold | refresh | recover | resume). Optional; the manager defaults
	// empty values to "cold".
	BootKind string

	// NewComputer marks a computer's first creation (the mint path in
	// resolveDesktopContext). Only it may take the data image template
	// (vmmanager data_image_template.go, VMCTL_DATA_IMAGE_TEMPLATE).
	NewComputer bool

	// OfflineMaintenance runs on a refresh between the kill and the boot,
	// with the data image path (store_compaction.go). Refresh only.
	OfflineMaintenance func(dataImagePath string) error

	// PostMaintenanceCredential re-mints the boot envelope after
	// OfflineMaintenance returns, for the same realization; refresh only
	// (maintenanceCredentialIssuer).
	PostMaintenanceCredential func() (string, error)

	// WarmnessClass is the computer's class at this boot ("test" for a
	// test account's computer): the visual capture grant reads it
	// (docs/design/visual-evidence-slice-2026-10-15.md).
	WarmnessClass string
}

// ErrOfflineMaintenanceUnbootable marks an OfflineMaintenance failure that
// left no bootable image; the refresh then fails instead of booting.
var ErrOfflineMaintenanceUnbootable = errors.New("offline maintenance left no bootable image")

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
	// GuestRelease is the guest release the VM was launched from, "" for a
	// legacy launch; Behind is the receipt of a snapshot resume onto a release
	// older than the current one (D2). Health recomputes it live.
	GuestRelease string
	Behind       bool
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

	// deployRefreshInFlight reports whether a deploy refresh of the computer is
	// running (the handler's tracker; SetDeployRefreshInFlight). Hibernate
	// skips such a computer: the refresh holds the guest's planned-restart
	// marker window, and a snapshot inside it would capture the marker (D2 S5).
	deployRefreshInFlight func(userID, desktopID string) bool

	// hostStopHealthProbe reads what a planned host stop needs from a guest's
	// /health (S5, host_stop_hibernate.go); nil is the HTTP probe.
	hostStopHealthProbe func(*VMOwnership) hostStopHealth
	// hostStopping is set when a planned host stop's pass begins; it stays set
	// (the process is exiting) and refuses every resume.
	hostStopping atomic.Bool

	// resumeEnroller is called after a snapshot resume onto a guest release
	// (SetResumeEnroller).
	resumeEnroller func(own VMOwnership, release string)

	// clock overrides time.Now for host-capacity tests.
	clock func() time.Time
	// unmanagedSince records when reconcile first saw a live VM process the
	// manager does not track
	// (host_capacity.go).
	unmanagedSince map[string]time.Time
	// lastProcessReconcile is the latest reconcile pass, for health.
	lastProcessReconcile VMProcessReconcileResult
	// networkDeadSince records when a live guest first refused a connection
	// to its service port (wedge.go); guestNetworkProbe overrides the probe.
	networkDeadSince  map[string]time.Time
	guestNetworkProbe func(computerURL string) bool

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
	// gatewayCredentialPeer is the guest address each VM's last successful
	// reconciliation named. A boot onto a new address is not throttled: the
	// gateway's Jev pin must follow the computer at once
	// (docs/problems/l2a-merge-recall-on-the-paper-2026-10-14.md).
	gatewayCredentialPeer map[string]string

	// autoputerURLBase is the base URL pattern for autoputer runtimes.
	// The VM ID is appended as a path component: base + "/" + vmID
	autoputerURLBase string

	// idleTimeout is the duration after which a VM with no activity
	// is eligible for stop/hibernate. Zero means no idle timeout.
	idleTimeout time.Duration

	// guestBusyProbe answers whether the guest behind an ownership has live
	// work (autoputer /health reports running_runs>0). Nil falls back to the
	// HTTP probe; tests inject a deterministic stub.
	guestBusyProbe func(own *VMOwnership) bool
	// guestHealthProbe is guestBusyProbe with what else the answer showed
	// (density D3a): whether the guest reports live_capsules. Nil falls back
	// to guestBusyProbe (reporting) and then the HTTP probe.
	guestHealthProbe func(own *VMOwnership) guestHealthReading
	// guestReportsCapsules is each guest's last answer to "does your /health
	// carry live_capsules", from a live probe.
	guestReportsCapsules map[string]bool
	// guestLastBusyAt is when a probe last found each guest busy (density
	// D3, PR-09). Work is activity: LastActiveAt tracks ownership
	// resolutions only, so a computer that built for 14 minutes with no
	// traffic reads idle the moment its apply restart boots a guest with no
	// work. Idle is measured from the later of the two. Not persisted: a
	// vmctl restart forgets it, and the guest's own busy fields still fence.
	guestLastBusyAt map[string]time.Time
	// legacyGuestIdleTimeout is the idle timeout of a guest whose /health
	// does not report live_capsules: such a guest cannot be told from one
	// holding a parked capsule, so the shorter idleTimeout does not apply
	// to it. Zero means no such rule (every guest uses idleTimeout).
	legacyGuestIdleTimeout time.Duration
	// guestBusySeen is each guest's last busy answer from a live probe.
	// vmctl health reads it instead of probing, so a guest that never
	// answers cannot make vmctl itself look down
	// (docs/problems/vmctl-health-fate-shares-a-hung-guest-2026-10-10.md).
	guestBusySeen map[string]bool

	// pressureReclaim controls pressure-aware dry-run lifecycle observation.
	// It ranks reclaim candidates from measured host pressure without changing
	// VM state until a later mission explicitly enables active reclaim.
	pressureReclaim PressureReclaimConfig
	pressureSampler hostPressureSampler
	// pressureWin is the pressure monitor's sustained-signal window and
	// the hibernate in-flight/cool-down bookkeeping (pressure_sustained.go).
	pressureWin pressureWindow

	// hostAdmission and admissionLedger admit starts by measured use, with
	// reservations for starts in flight (host_admission.go).
	hostAdmission   HostAdmissionConfig
	admissionLedger hostMemoryLedger
	// cacheReturnCfg and cacheReturn are the guest page cache return
	// trigger (guest_cache_return.go): the switch, and its memory-only
	// state. Off by default.
	cacheReturnCfg GuestCacheReturnConfig
	cacheReturn    guestCacheReturnState
	// makeRoomMu is the one make-room decider (host_admission_make_room.go):
	// a refused start that waits on it re-admits before choosing a victim.
	// Never taken while holding mu.
	makeRoomMu sync.Mutex
	// makeRoomGuard authorizes make-room's hibernates as the idle sweep's
	// guard authorizes its own; nil means make-room hibernates nothing.
	makeRoomGuard ComputerVersionRouteGuard

	// retentionPrune controls durable VM-state deletion for explicitly
	// disposable computers such as staging Playwright accounts.
	retentionPrune      RetentionPruneConfig
	retentionUserEmails map[string]string
	// retentionClock is time.Now outside tests (the test-class idle rule
	// and the quarantine age-out compare ages against it).
	retentionClock func() time.Time

	// warmnessPolicy controls under-capacity primary keepalive and future
	// always-on tier modeling.
	warmnessPolicy WarmnessPolicyConfig

	// epochCounter tracks the global epoch counter for VM boot tracking.
	// Each fresh boot or recovery increments this counter, providing a
	// mechanism to prevent duplicate canonical effects (VAL-CROSS-117).
	epochCounter int64

	// lifecycle is the one lifecycle claim per stable ownership: every path
	// that boots an existing computer (a visit's resolve, resume, recover,
	// the always-on sweep, a deploy refresh or rescue) and hibernate take it
	// under r.mu before they unlock for the manager, and hold it until the
	// resulting state is published. A second path finds it held and refuses
	// (errLifecycleBusy) or, for a visit, waits on pendingWaiters
	// (docs/problems/vmctl-failed-protected-computer-waits-a-day-and-deploy-cannot-reach-it-2026-10-21.md,
	// "Review round 3").
	lifecycle map[string]lifecycleHolder
	// interruptedStarts holds the claim of each start a vmctl restart
	// interrupted, from load until its process is gone (boot_claim.go).
	interruptedStarts map[string]*BootClaim

	// vmManager is the optional Firecracker VM lifecycle manager.
	// When nil, the registry operates in host-process autoputer mode where
	// all VMs share the same autoputer URL. When set, the registry delegates
	// VM lifecycle operations to this manager for real Firecracker VMs.
	vmManager VMManager

	// gatewayURL is the URL of the host-side gateway service. When set,
	// the registry issues gateway tokens for VM autoputeres before booting
	// so the guest autoputer can authenticate to the gateway.
	gatewayURL string
	corpusdURL string

	// persistencePath stores ownership metadata across vmctl restarts. The
	// Firecracker data disks live under the VM manager state dir; this file is
	// the durable routing index that lets vmctl reattach to those disks.
	persistencePath string
	// persistenceLoaded records that ownerships were read from the durable
	// file. The snapshot start sweep runs only then: an empty in-memory
	// registry would make every hibernated computer's snapshot look orphaned.
	persistenceLoaded bool

	// recoveryAdmission is the durable typed recovery breaker. It is separate
	// from lifecycle state: conditions survive restarts and gate every
	// realization start before resource allocation. Nil disables the gate.
	recoveryAdmission *RecoveryAdmission
}

// NewOwnershipRegistry creates a new ownership registry.
// The idleTimeout parameter configures automatic VM stop after inactivity.
// Zero means no idle timeout (VMs stay active indefinitely).
func NewOwnershipRegistry(autoputerURLBase string) *OwnershipRegistry {
	if autoputerURLBase == "" {
		autoputerURLBase = "http://127.0.0.1:8085"
	}
	return &OwnershipRegistry{
		ownerships:                 make(map[string]*VMOwnership),
		unmanagedSince:             make(map[string]time.Time),
		networkDeadSince:           make(map[string]time.Time),
		guestBusySeen:              make(map[string]bool),
		vmByID:                     make(map[string]*VMOwnership),
		pendingWaiters:             make(map[string][]chan *VMOwnership),
		gatewayCredentialNextCheck: make(map[string]time.Time),
		gatewayCredentialPeer:      make(map[string]string),
		lifecycle:                  make(map[string]lifecycleHolder),
		interruptedStarts:          make(map[string]*BootClaim),
		autoputerURLBase:           autoputerURLBase,
		idleTimeout:                0, // no idle timeout by default
		pressureReclaim:            DefaultPressureReclaimConfig(),
		pressureSampler:            sampleHostPressure,
		hostAdmission:              DefaultHostAdmissionConfig(),
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
// same VM ID and data disk instead of returning a stale autoputer URL.
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
	r.interruptedStarts = make(map[string]*BootClaim)
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
		// A start in flight at the restart is closed, never adopted: its row
		// does not describe the process it spawned (boot_claim.go).
		interrupted := interruptStartAtLoadLocked(&own, time.Now())
		if !interrupted && (own.State == "" || own.State == VMStateActive || own.State == VMStateDegraded || own.State == VMStateStopping) {
			own.State = VMStateStopped
			if own.StoppedBy == "" {
				own.StoppedBy = stoppedByVMCTLRestart
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
		if interrupted {
			r.interruptedStarts[own.VMID] = own.BootClaim
			log.Printf("vmctl: VM %s (user %s): its %s of epoch %d was in flight at the vmctl restart; closing it, stopped_by=%s (%d interrupted starts in a row)", own.VMID, own.UserID, own.BootClaim.Kind, own.BootClaim.Epoch, own.StoppedBy, own.StartInterruptions)
		}
	}
	if state.EpochCounter > maxEpoch {
		maxEpoch = state.EpochCounter
	}
	r.epochCounter = maxEpoch
	r.persistenceLoaded = true
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
// running in host-process autoputer mode. This activates real Firecracker
// VM lifecycle on Node B.
func (r *OwnershipRegistry) SetVMManager(mgr VMManager) {
	r.mu.Lock()
	r.vmManager = mgr
	r.mu.Unlock()
}

// SetDeployRefreshInFlight registers the in-flight check hibernate consults.
func (r *OwnershipRegistry) SetDeployRefreshInFlight(fn func(userID, desktopID string) bool) {
	r.mu.Lock()
	r.deployRefreshInFlight = fn
	r.mu.Unlock()
}

// SetResumeEnroller registers the callback a release-bound snapshot resume
// calls (D2 S4: the handler's enrollResumed).
func (r *OwnershipRegistry) SetResumeEnroller(fn func(own VMOwnership, release string)) {
	r.mu.Lock()
	r.resumeEnroller = fn
	r.mu.Unlock()
}

// VMManagerHandle returns the configured VM lifecycle manager, if any. The
// boot-timeline endpoint uses it to read per-VM receipts.
func (r *OwnershipRegistry) VMManagerHandle() VMManager {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.vmManager
}

// ReattachManagedVMs first closes the starts the restart interrupted
// (boot_claim.go), then adopts VM processes of computers that were running,
// only after their owner/computer D-ROUTE has been independently authorized.
func (r *OwnershipRegistry) ReattachManagedVMs(ctx context.Context, guard ComputerVersionRouteGuard) int {
	// The starts the restart interrupted are closed before anything is
	// adopted or served.
	r.closeInterruptedStarts()
	r.mu.RLock()
	mgr := r.vmManager
	candidates := make([]VMOwnership, 0)
	if mgr != nil {
		for _, own := range r.ownerships {
			if own.IsHeld() {
				// A held computer is never reattached to a stale route after a
				// vmctl restart; it stays in maintenance hold.
				continue
			}
			if own.State == VMStateStopped && own.StoppedBy == stoppedByVMCTLRestart && strings.TrimSpace(own.ComputerURL) != "" {
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
		if r.reattachOwnership(own, mgr) {
			reattached++
		}
	}
	if reattached > 0 {
		go r.ReconcileReadyGatewayCredentials()
	}
	return reattached
}

// reattachOwnership adopts one surviving VM process. A failed reattach
// leaves the process running outside the manager; ReconcileVMProcesses
// retries it and powers it off after a grace period (host_capacity.go).
func (r *OwnershipRegistry) reattachOwnership(own VMOwnership, mgr VMManager) bool {
	_, ok := r.reattachOwnershipInfo(own, mgr)
	return ok
}

func (r *OwnershipRegistry) reattachOwnershipInfo(own VMOwnership, mgr VMManager) (*VMInstanceInfo, bool) {
	if r.startInterrupted(own.VMID) {
		// The process belongs to a start no row describes: its address and
		// readiness are unknown. Only the close ends it.
		log.Printf("vmctl: reattach refused for VM %s: its start was interrupted by a vmctl restart and is being closed", own.VMID)
		return nil, false
	}
	info, err := mgr.ReattachVM(own.VMID, own.ComputerURL, own.Epoch)
	if err != nil {
		log.Printf("vmctl: reattach skipped for VM %s: %v", own.VMID, err)
		return nil, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cur, ok := r.vmByID[own.VMID]
	if !ok {
		return nil, false
	}
	cur.State = VMStateActive
	cur.ComputerURL = info.HostURL
	cur.Epoch = info.Epoch
	if cur.LastActiveAt.IsZero() {
		cur.LastActiveAt = time.Now()
	}
	cur.StoppedBy = ""
	delete(r.unmanagedSince, own.VMID)
	r.saveLocked()
	return info, true
}

// SetGatewayURL configures the gateway URL for issuing autoputer tokens.
// When set, the registry will issue a gateway token for each VM before
// booting so the guest autoputer can authenticate to the gateway.
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

// ConfigureRecoveryAdmission attaches the durable typed recovery breaker. It
// loads existing conditions immediately so a vmctl restart keeps refusing
// deterministically blocked starts without a guest boot.
func (r *OwnershipRegistry) ConfigureRecoveryAdmission(cfg RecoveryAdmissionConfig) error {
	admission, err := NewRecoveryAdmission(cfg)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.recoveryAdmission = admission
	r.mu.Unlock()
	return nil
}

// RecoveryConditionFor returns the durable recovery condition for a computer,
// if one is recorded.
func (r *OwnershipRegistry) RecoveryConditionFor(computerID string) (*RecoveryCondition, bool) {
	admission := r.recoveryAdmissionHandle()
	if admission == nil {
		return nil, false
	}
	return admission.Condition(computerID)
}

func (r *OwnershipRegistry) recoveryAdmissionHandle() *RecoveryAdmission {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.recoveryAdmission
}

// admitRecoveryStart is the retained-aware fail-fast gate consulted before any
// realization start. It returns a typed *RecoveryRefusal when the computer is
// durably blocked; a nil admission (feature off), an unknown local state, or
// unverifiable metadata leave the decision to the guest planner.
func (r *OwnershipRegistry) admitRecoveryStart(own *VMOwnership, emptyStore bool) error {
	if own == nil {
		return nil
	}
	admission := r.recoveryAdmissionHandle()
	if admission == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), recoveryAdmissionTimeout)
	defer cancel()
	if refusal := admission.Admit(ctx, stableComputerID(own.UserID, own.DesktopID, own.ComputerID), own.UserID, emptyStore); refusal != nil {
		return refusal
	}
	return nil
}

// noteRecoveryStartFailure converts a VM-manager boot failure carrying the
// guest planner's typed refusal into a durable recovery condition and returns
// a typed refusal for prompt propagation. Other failures pass through.
func (r *OwnershipRegistry) noteRecoveryStartFailure(own *VMOwnership, err error) error {
	if err == nil || own == nil {
		return err
	}
	if liveUnmanagedBootRefusal(err) {
		// vmmanager refused to boot over a live process it does not track
		// (ErrLiveUnmanaged): the computer is running, so callers wait.
		return &RecoveryRefusal{
			Kind:              RecoveryRefusalGuestReattachPending,
			Reason:            "computer is still running but did not answer reattach; retry shortly",
			RetryAfterSeconds: guestReattachRetryAfterSeconds,
		}
	}
	admission := r.recoveryAdmissionHandle()
	if admission == nil {
		return err
	}
	var guest guestBootRefusal
	if !errors.As(err, &guest) {
		return err
	}
	kind, reason, local, watermark, target, empty, chain := guest.GuestBootRefusal()
	computerID := stableComputerID(own.UserID, own.DesktopID, own.ComputerID)
	refusal := admission.RecordGuestRefusal(GuestBootRefusalInput{
		ComputerID:        computerID,
		OwnerID:           own.UserID,
		Kind:              kind,
		Reason:            reason,
		LocalSequence:     local,
		WatermarkSequence: watermark,
		TargetSequence:    target,
		EmptyStore:        empty,
		ChainExists:       chain,
	})
	if refusal == nil {
		return err
	}
	log.Printf("vmctl: computer %s recovery refusal recorded (kind=%s kind_guest=%s): %s", computerID, refusal.Kind, kind, refusal.Reason)
	return refusal
}

// clearRecoveryCondition drops a stale durable condition after a realization
// started successfully: a healthy retained store is the authority again.
func (r *OwnershipRegistry) clearRecoveryCondition(own *VMOwnership) {
	if own == nil {
		return
	}
	admission := r.recoveryAdmissionHandle()
	if admission == nil {
		return
	}
	admission.Clear(stableComputerID(own.UserID, own.DesktopID, own.ComputerID))
}

// SetIdleTimeout configures the idle timeout for automatic VM lifecycle
// management. After this duration of inactivity, VMs are eligible for
// stop/hibernate (VAL-VM-008, VAL-CROSS-116).
func (r *OwnershipRegistry) SetIdleTimeout(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.idleTimeout = d
}

// SetLegacyGuestIdleTimeout sets the idle timeout for a guest that does not
// report live_capsules in /health (density D3a). The idle sweep, pressure
// reclaim and make-room hibernate such a guest only after it has been idle
// this long, however short idleTimeout and the reclaim idle are. Zero turns
// the rule off.
func (r *OwnershipRegistry) SetLegacyGuestIdleTimeout(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.legacyGuestIdleTimeout = d
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

func (r *OwnershipRegistry) warmnessPolicySnapshot() WarmnessPolicyConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return normalizeWarmnessPolicyConfig(r.warmnessPolicy)
}

// refreshTestAccounts reloads the test tier from the auth DB: the accounts
// retention treats as ephemeral (by email domain or user-id prefix). A
// failed read keeps the last good set, so a missing DB never promotes test
// computers or drops anyone's tier.
func (r *OwnershipRegistry) refreshTestAccounts() {
	r.mu.RLock()
	retention := r.retentionPrune
	r.mu.RUnlock()
	if strings.TrimSpace(retention.AuthDBPath) == "" {
		return
	}
	emails, errs := loadRetentionEmailsFromAuthDB(retention.AuthDBPath)
	if len(errs) > 0 {
		log.Printf("vmctl: test tier refresh kept the previous set: %s", strings.Join(errs, "; "))
		return
	}
	test := make(map[string]bool)
	for userID, email := range emails {
		if retentionOwnershipEphemeral(&VMOwnership{UserID: userID}, email, retention) {
			test[userID] = true
		}
	}
	r.mu.Lock()
	r.warmnessPolicy.TestUserIDs = test
	r.mu.Unlock()
}

// bootWarmnessClass is a computer's class for a boot's inputs (the visual
// capture grant reads "test"; docs/design/visual-evidence-slice-2026-10-15.md
// V2). The test tier is refreshed by the idle sweep, so a test account
// minted moments ago may not be in it yet: on a primary answer for an
// account the tier does not know, the tier is reloaded once from the auth
// DB. A failed reload keeps the previous set, so the answer fails closed.
// Callers must not hold r.mu.
func (r *OwnershipRegistry) bootWarmnessClass(own *VMOwnership) WarmnessClass {
	if own == nil {
		return ""
	}
	class := warmnessClassForOwnership(own, r.warmnessPolicySnapshot())
	if class != WarmnessClassPrimary {
		return class
	}
	r.refreshTestAccounts()
	return warmnessClassForOwnership(own, r.warmnessPolicySnapshot())
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
		r.refreshTestAccounts()
		r.ReconcileVMProcesses(ctx, guard)
		if warmed := r.WarmAlwaysOnDesktops(ctx, guard); warmed > 0 {
			log.Printf("vmctl: warmness policy resumed %d always-on desktop VM(s)", warmed)
		}
		if warmed := r.WarmUniversalWirePlatformComputer(ctx, guard); warmed > 0 {
			log.Printf("vmctl: warmness policy resumed %d universal wire platform computer(s)", warmed)
		}
		if plan := r.PressureReclaimPlan(); plan.Mode == PressureReclaimModeDryRun {
			log.Printf("vmctl: pressure reclaim dry-run decision=%s reason=%q active=%d eligible=%d protected=%d pressure=%v reclaim_pressure=%v signals=%s",
				plan.Decision, plan.Reason, plan.Inventory.Active, plan.Inventory.Eligible, plan.Inventory.Protected, plan.Pressure.Pressure, plan.Pressure.ReclaimPressure, strings.Join(plan.Pressure.Signals, ","))
		} else if plan.Mode == PressureReclaimModeActive {
			log.Printf("vmctl: pressure reclaim active decision=%s reason=%q active=%d eligible=%d protected=%d pressure=%v reclaim_pressure=%v signals=%s",
				plan.Decision, plan.Reason, plan.Inventory.Active, plan.Inventory.Eligible, plan.Inventory.Protected, plan.Pressure.Pressure, plan.Pressure.ReclaimPressure, strings.Join(plan.Pressure.Signals, ","))
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

// lifecycleHolder names the path that holds a computer's lifecycle claim.
type lifecycleHolder string

const (
	// lifecycleVisit: a visit's resolve starts the computer or checks an
	// active one's readiness; other resolves wait on pendingWaiters.
	lifecycleVisit     lifecycleHolder = "visit start"
	lifecycleResume    lifecycleHolder = "resume"
	lifecycleRecover   lifecycleHolder = "recovery"
	lifecycleRefresh   lifecycleHolder = "refresh"
	lifecycleHibernate lifecycleHolder = "hibernate"
	lifecycleRescue    lifecycleHolder = "deploy rescue"
	// lifecyclePlatformWarm: the sweep's warm of the platform computer
	// (platform_computer.go).
	lifecyclePlatformWarm lifecycleHolder = "platform warm"
)

// errLifecycleBusy marks a refusal made because another path holds the
// computer's lifecycle claim; it spawned nothing and clears by itself.
var errLifecycleBusy = errors.New("is already in progress")

// lifecycleConflictLocked refuses when any path holds key's claim.
func (r *OwnershipRegistry) lifecycleConflictLocked(key string) error {
	holder, held := r.lifecycle[key]
	if !held {
		return nil
	}
	vmID := "unknown"
	if own := r.ownerships[key]; own != nil && own.VMID != "" {
		vmID = own.VMID
	}
	return fmt.Errorf("VM %s %s %w", vmID, holder, errLifecycleBusy)
}

// releaseLifecycleLocked ends holder's claim; another holder's stands.
func (r *OwnershipRegistry) releaseLifecycleLocked(key string, holder lifecycleHolder) {
	if r.lifecycle[key] == holder {
		delete(r.lifecycle, key)
	}
}

func (r *OwnershipRegistry) releaseLifecycle(key string, holder lifecycleHolder) {
	r.mu.Lock()
	r.releaseLifecycleLocked(key, holder)
	r.mu.Unlock()
}

// lifecycleRebootingLocked reports a claim under which the realization is
// being replaced or suspended; lookups report the computer as booting. A
// visit's readiness check of an active computer is not one.
func (r *OwnershipRegistry) lifecycleRebootingLocked(key string) bool {
	holder, held := r.lifecycle[key]
	return held && holder != lifecycleVisit
}

func machineShapeForOwnership(own *VMOwnership) (int, int) {
	if own != nil && isPlatformOwnership(own) {
		return platformVMCPUCount, platformVMMemSizeMib
	}
	return interactiveMachineShape()
}

// interactiveMachineShape returns the interactive guest CPU/memory shape,
// honoring host overrides. Malformed override values fall back to the
// compiled defaults rather than refusing to boot.
func interactiveMachineShape() (int, int) {
	return envPositiveOrDefault(interactiveVMCPUCountEnv, interactiveVMCPUCount),
		envPositiveOrDefault(interactiveVMMemSizeMibEnv, interactiveVMMemSizeMib)
}

func envPositiveOrDefault(key string, def int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
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
	if own.IsHeld() {
		// A held, serving computer resolves from its persisted ComputerURL
		// without a readiness/assignment wait. Registering a pendingWaiter here
		// serializes concurrent browser probes such that a second probe blocks
		// on a nil-channel wait that nobody signals until the browser aborts
		// (15s) — the resolve-race in the held-computer receipt. Short-circuit so
		// the held computer resolves instantly and deterministically.
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

// unreadyActiveVM is the answer for a live guest that has not answered.
func unreadyActiveVM(info *VMInstanceInfo) *VMInstanceInfo {
	unready := *info
	unready.Healthy = false
	return &unready
}

func (r *OwnershipRegistry) ensureActiveVMReady(own *VMOwnership, mgr VMManager) (*VMInstanceInfo, error) {
	if own == nil {
		return nil, fmt.Errorf("ownership is required")
	}
	if own.IsHeld() {
		// A held computer is never automatically booted/recovered/restarted on
		// a resolve/readiness check. It stays in its current (held) state; the
		// epoch is not incremented and no new realization is started. Only an
		// authorized maintenance recovery may move it.
		return &VMInstanceInfo{
			HostURL: own.ComputerURL,
			Epoch:   own.Epoch,
			Healthy: false,
			State:   "held",
		}, nil
	}
	if mgr == nil {
		return &VMInstanceInfo{
			HostURL: own.ComputerURL,
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
		// A live process that has not answered is pending, not broken: a
		// resolve never destroys it (docs/vmctl-360-review-2026-10-10.md,
		// Phase 0 step 2). The owner restarts it explicitly.
		log.Printf("vmctl: active ownership for VM %s is still pending past its grace; routing as unready, not recovering", own.VMID)
		return unreadyActiveVM(info), nil
	case "", "running":
		healthy, err := mgr.CheckHealth(own.VMID)
		if err != nil {
			log.Printf("vmctl: active VM %s health probe errored; routing as unready, not recovering: %v", own.VMID, err)
			return unreadyActiveVM(info), nil
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
		log.Printf("vmctl: active VM %s is unhealthy on resolve; routing as unready, not recovering", own.VMID)
		return unreadyActiveVM(info), nil
	default:
		// failed: the process crashed or boot's readiness timeout already
		// killed it, so starting it again destroys nothing alive.
		log.Printf("vmctl: active ownership for VM %s found manager state=%s; recovering before routing", own.VMID, state)
		return r.recoverOrRestartActiveVM(own, mgr)
	}
}

func (r *OwnershipRegistry) reserveFreshVMConfig(own *VMOwnership, gatewayToken string, mgr VMManager) (VMManagerConfig, error) {
	if own == nil || mgr == nil {
		return VMManagerConfig{}, fmt.Errorf("VM realization authority is unavailable")
	}
	// bootWarmnessClass refreshes the test tier for an account minted since
	// the sweep.
	warmness := r.bootWarmnessClass(own)
	epoch, err := mgr.ReserveBootEpoch(own.VMID, own.Epoch+1)
	if err != nil {
		return VMManagerConfig{}, err
	}
	r.mu.Lock()
	if r.epochCounter < epoch {
		r.epochCounter = epoch
	}
	if err := r.writePersistenceLocked(); err != nil {
		r.mu.Unlock()
		return VMManagerConfig{}, fmt.Errorf("persist reserved VM realization %d: %w", epoch, err)
	}
	cfg := vmManagerConfigForOwnership(own, gatewayToken)
	cfg.Epoch = epoch
	cfg.RealizationID = realizationIDFor(own.VMID, epoch)
	corpusdURL := r.corpusdURL
	r.mu.Unlock()

	cfg.WarmnessClass = string(warmness)
	cfg.ComputerCredentialEnvelope = issueComputerCredentialEnvelope(corpusdURL, cfg.ComputerID, cfg.RealizationID, cfg.Epoch)
	if cfg.ComputerCredentialEnvelope == "" && strings.TrimSpace(corpusdURL) != "" {
		return VMManagerConfig{}, fmt.Errorf("realization credential unavailable")
	}
	return cfg, nil
}

func (r *OwnershipRegistry) recoverOrRestartActiveVM(own *VMOwnership, mgr VMManager) (*VMInstanceInfo, error) {
	if mgr.GetVM(own.VMID) == nil {
		return r.startExistingVM(own, mgr)
	}
	if err := r.admitRecoveryStart(own, false); err != nil {
		return nil, err
	}
	cfg, refresh, err := r.trackedBootConfig(own, mgr)
	if err != nil {
		return nil, err
	}
	recovered, err := r.spawnClaimed(own.VMID, bootClaimBoot, cfg.Epoch, func() (*VMInstanceInfo, error) {
		return bootTracked(mgr, own.VMID, cfg, refresh)
	})
	if err != nil {
		if mgr.GetVM(own.VMID) == nil {
			return r.startExistingVM(own, mgr)
		}
		return nil, r.noteRecoveryStartFailure(own, err)
	}
	r.clearRecoveryCondition(own)
	return recovered, nil
}

func (r *OwnershipRegistry) startExistingVM(own *VMOwnership, mgr VMManager) (*VMInstanceInfo, error) {
	return r.startExistingVMCtx(context.Background(), own, mgr)
}

// startExistingVMCtx is startExistingVM for a start with a request context,
// which a cancelled caller uses to stop make-room.
func (r *OwnershipRegistry) startExistingVMCtx(ctx context.Context, own *VMOwnership, mgr VMManager) (*VMInstanceInfo, error) {
	if own == nil || mgr == nil {
		return nil, nil
	}
	if mgr.GetVM(own.VMID) == nil && r.liveUnmanagedProcess(own.VMID, mgr) {
		// The guest is still running but vmctl lost track of it (a reattach
		// after vmctl's restart found it too busy to answer). Booting would
		// kill it mid-work: retry the reattach, else refuse with Retry-After.
		// Only an explicit stop or the wedge watchdog (wedge.go) may end it
		// (vmctl-restart-reboots-busy-computer-2026-10-10).
		if info, ok := r.reattachOwnershipInfo(*own, mgr); ok {
			return info, nil
		}
		return nil, &RecoveryRefusal{
			Kind:              RecoveryRefusalGuestReattachPending,
			Reason:            "computer is still running but did not answer reattach; retry shortly",
			RetryAfterSeconds: guestReattachRetryAfterSeconds,
		}
	}
	var ticket *admissionTicket
	if tracked := mgr.GetVM(own.VMID); tracked == nil || !managedGuestMayLive(tracked.State) {
		// Booting adds a VM, and so does starting a tracked instance whose
		// guest is not running; recovering a live one replaces it in place.
		r.mu.RLock()
		class := warmnessClassForOwnership(own, r.warmnessPolicy)
		r.mu.RUnlock()
		admitted, err := r.admitHostMemoryCtx(ctx, own, class)
		if err != nil {
			return nil, err
		}
		ticket = admitted
	}
	info, err := r.startAdmittedVM(own, mgr)
	// The reservation ends with the start: released on failure, settling
	// against measured memory on success (host_admission.go).
	ticket.finish(err == nil && info != nil)
	return info, err
}

// startAdmittedVM is startExistingVM after host memory admission.
func (r *OwnershipRegistry) startAdmittedVM(own *VMOwnership, mgr VMManager) (*VMInstanceInfo, error) {
	if err := r.admitRecoveryStart(own, false); err != nil {
		return nil, err
	}
	if info, ok := r.resumeFromSnapshot(own, mgr); ok {
		r.clearRecoveryCondition(own)
		return info, nil
	}
	if mgr.GetVM(own.VMID) != nil {
		cfg, refresh, err := r.trackedBootConfig(own, mgr)
		if err != nil {
			return nil, err
		}
		info, err := r.spawnClaimed(own.VMID, bootClaimBoot, cfg.Epoch, func() (*VMInstanceInfo, error) {
			return bootTracked(mgr, own.VMID, cfg, refresh)
		})
		if err != nil {
			return nil, r.noteRecoveryStartFailure(own, err)
		}
		r.clearRecoveryCondition(own)
		return info, nil
	}
	cfg, err := r.reserveFreshVMConfig(own, r.issueGatewayToken(own.VMID), mgr)
	if err != nil {
		return nil, err
	}
	info, err := r.spawnClaimed(own.VMID, bootClaimBoot, cfg.Epoch, func() (*VMInstanceInfo, error) { return mgr.BootVM(cfg) })
	if err != nil {
		return nil, r.noteRecoveryStartFailure(own, err)
	}
	r.clearRecoveryCondition(own)
	return info, nil
}

// trackedBootConfig prepares the start of an instance the manager still
// tracks. A guest that is not live (stopped by the wedge detector, failed,
// hibernated without a usable snapshot) boots onto the current deploy with a
// freshly issued gateway token (refresh true: RefreshVM, refreshBootConfig),
// never on the stored config of the boot that hung; a live guest is
// recovered in place (RecoverVM), as crash recovery always did
// (docs/problems/vmctl-failed-protected-computer-waits-a-day-and-deploy-cannot-reach-it-2026-10-21.md,
// Argon 1).
func (r *OwnershipRegistry) trackedBootConfig(own *VMOwnership, mgr VMManager) (VMManagerConfig, bool, error) {
	info := mgr.GetVM(own.VMID)
	refresh := info != nil && !managedGuestMayLive(info.State)
	token := ""
	if refresh {
		token = r.issueGatewayToken(own.VMID)
	}
	cfg, err := r.reserveFreshVMConfig(own, token, mgr)
	return cfg, refresh, err
}

func bootTracked(mgr VMManager, vmID string, cfg VMManagerConfig, refresh bool) (*VMInstanceInfo, error) {
	if refresh {
		return mgr.RefreshVM(vmID, cfg)
	}
	return mgr.RecoverVM(vmID, cfg)
}

// resumeFromSnapshot wakes a hibernated, unheld computer from its memory
// snapshot when the manager kept one. It reports false whenever the caller
// must start the computer the ordinary way (fresh epoch, cold boot).
func (r *OwnershipRegistry) resumeFromSnapshot(own *VMOwnership, mgr VMManager) (*VMInstanceInfo, bool) {
	if own == nil || own.State != VMStateHibernated || own.IsHeld() {
		return nil, false
	}
	resumer, ok := mgr.(snapshotResumer)
	if !ok || resumer == nil {
		return nil, false
	}
	resumeCfg := vmManagerConfigForOwnership(own, "")
	info, err := r.spawnClaimed(own.VMID, bootClaimResume, own.Epoch, func() (*VMInstanceInfo, error) {
		return resumer.ResumeVMFromSnapshot(own.VMID, resumeCfg, r.resumeCredentialIssuer(own))
	})
	if err != nil {
		log.Printf("vmctl: computer %s (VM %s) not resumed from snapshot; cold boot follows: %v", own.ComputerID, own.VMID, err)
		return nil, false
	}
	if info == nil {
		return nil, false
	}
	log.Printf("vmctl: computer %s (VM %s) resumed from snapshot (epoch=%d)", own.ComputerID, own.VMID, info.Epoch)
	// A resume onto a guest release is followed by an update at the computer's
	// next idle moment (D2 S4); the registry does not know the handler.
	if info.GuestRelease != "" {
		r.mu.RLock()
		enroll := r.resumeEnroller
		r.mu.RUnlock()
		if enroll != nil {
			enroll(*own, info.GuestRelease)
		}
	}
	return info, true
}

// resumeCredentialIssuer returns the S3 re-credential issuer for one resume
// attempt of own's snapshot, or nil when vmctl has no corpusd. The envelope
// binds the snapshot's own realization (same VM, same epoch): a resume is
// not a new realization. corpusd replays a same-key request from its
// recorded issue time, so the boot key would be refused hours later; each
// attempt gets a fresh key (a random nonce), which retries inside the attempt
// reuse. Consuming the envelope advances the computer's credential
// revocation epoch exactly as a boot does
// (docs/reports/s3-snapshot-hibernate-design-2026-10-14.md).
func (r *OwnershipRegistry) resumeCredentialIssuer(own *VMOwnership) func() (string, error) {
	if own == nil {
		return nil
	}
	r.mu.RLock()
	corpusdURL := r.corpusdURL
	r.mu.RUnlock()
	if strings.TrimSpace(corpusdURL) == "" || own.ComputerID == "" {
		return nil
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil
	}
	computerID, epoch := own.ComputerID, own.Epoch
	realizationID := realizationIDFor(own.VMID, epoch)
	key := fmt.Sprintf("guest-credential-resume:%s:%d:%s", realizationID, epoch, hex.EncodeToString(nonce[:]))
	return func() (string, error) {
		envelope := issueComputerCredentialEnvelopeWithKey(corpusdURL, computerID, realizationID, key)
		if envelope == "" {
			return "", fmt.Errorf("resume credential for %s unavailable", realizationID)
		}
		return envelope, nil
	}
}

// maintenanceCredentialIssuer returns the re-mint vmmanager calls after a
// refresh's OfflineMaintenance hook, or nil when vmctl has no corpusd. The
// boot envelope in cfg was minted before the kill; corpusd gives it 10
// minutes, and a long hook outlives it (docs/problems/offline-maintenance-
// outlives-the-boot-credential-2026-10-24.md). The re-mint binds the same
// reserved realization under its own key, so the lifecycle receipt names it
// and corpusd does not replay the stale envelope; the key is fixed per
// realization, so a second call replays the post-hook envelope instead of
// issuing another.
func (r *OwnershipRegistry) maintenanceCredentialIssuer(cfg VMManagerConfig) func() (string, error) {
	r.mu.RLock()
	corpusdURL := r.corpusdURL
	r.mu.RUnlock()
	if strings.TrimSpace(corpusdURL) == "" || cfg.ComputerID == "" || cfg.RealizationID == "" {
		return nil
	}
	computerID, realizationID := cfg.ComputerID, cfg.RealizationID
	key := fmt.Sprintf("guest-credential-maintained:%s:%d", realizationID, cfg.Epoch)
	return func() (string, error) {
		envelope := issueComputerCredentialEnvelopeWithKey(corpusdURL, computerID, realizationID, key)
		if envelope == "" {
			return "", fmt.Errorf("post-maintenance credential for %s unavailable", realizationID)
		}
		return envelope, nil
	}
}

func vmManagerConfigForOwnership(own *VMOwnership, gatewayToken string) VMManagerConfig {
	if own == nil {
		return VMManagerConfig{}
	}
	cpu, mem := machineShapeForOwnership(own)
	// MaintenanceHold is derived from the ownership hold record: the boot
	// configuration is never the authority for fenced boots, so a stale
	// manager instance config cannot re-fence a computer after unhold
	// (2026-09-03). Callers needing a different mode set the flags explicitly
	// afterwards (recoverVMForDesktop does for maintenance recovery).
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
		MaintenanceHold:   own.IsHeld(),
	}
	return cfg
}

// issueGatewayToken requests a gateway credential token for the given
// autoputer ID by calling the gateway's credential issuance endpoint.
// Returns the raw token string or an empty string on failure.
// Failures are logged but not fatal — the VM will still boot but
// won't be able to authenticate to the gateway until a token is provided.
func (r *OwnershipRegistry) issueGatewayToken(computerID string) string {
	r.mu.RLock()
	gwURL := r.gatewayURL
	r.mu.RUnlock()
	return issueGatewayTokenAt(gwURL, computerID)
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
	hostauth.MarkInternal(request)
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

// credentialIssueAttempts bounds issuance retries inside one realization;
// the issuer replays a same-key retry, so a timed-out first attempt that did
// commit is safe to repeat.
const credentialIssueAttempts = 3

var credentialIssueRetryBackoff = func(attempt int) time.Duration {
	return time.Duration(attempt) * time.Second
}

func issueComputerCredentialEnvelope(corpusdURL, computerID, realizationID string, epoch int64) string {
	return issueComputerCredentialEnvelopeWithKey(corpusdURL, computerID, realizationID, fmt.Sprintf("guest-credential:%s:%d", realizationID, epoch))
}

// issueComputerCredentialEnvelopeWithKey requests an envelope under an
// explicit idempotency key, retrying transport errors and 5xx with that key.
func issueComputerCredentialEnvelopeWithKey(corpusdURL, computerID, realizationID, idempotencyKey string) string {
	if corpusdURL == "" || computerID == "" || realizationID == "" || idempotencyKey == "" {
		return ""
	}
	for attempt := 1; ; attempt++ {
		envelope, retryable := issueComputerCredentialEnvelopeOnce(corpusdURL, computerID, realizationID, idempotencyKey)
		if envelope != "" || !retryable || attempt >= credentialIssueAttempts {
			return envelope
		}
		log.Printf("vmctl: retrying computer credential request for %s (attempt %d of %d)", realizationID, attempt+1, credentialIssueAttempts)
		time.Sleep(credentialIssueRetryBackoff(attempt))
	}
}

// issueComputerCredentialEnvelopeOnce makes one issuance request. retryable
// is true for transport errors and 5xx; an explicit refusal is final.
func issueComputerCredentialEnvelopeOnce(corpusdURL, computerID, realizationID, idempotencyKey string) (string, bool) {
	body, _ := json.Marshal(map[string]string{
		"computer_id":     computerID,
		"realization_id":  realizationID,
		"idempotency_key": idempotencyKey,
	})
	ctx, cancel := context.WithTimeout(context.Background(), gatewayCredentialRequestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(corpusdURL, "/")+"/internal/computers/credentials/issue", strings.NewReader(string(body)))
	if err != nil {
		return "", false
	}
	request.Header.Set("Content-Type", "application/json")
	hostauth.MarkInternal(request)
	hostauth.MarkIssuer(request)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		log.Printf("vmctl: computer credential request failed for %s: %v", realizationID, err)
		return "", true
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusInternalServerError {
		log.Printf("vmctl: computer credential request unavailable for %s: status %d", realizationID, response.StatusCode)
		return "", true
	}
	var result struct {
		Envelope json.RawMessage `json:"envelope"`
	}
	if response.StatusCode != http.StatusCreated || json.NewDecoder(io.LimitReader(response.Body, 128<<10)).Decode(&result) != nil || len(result.Envelope) == 0 {
		log.Printf("vmctl: computer credential request refused for %s", realizationID)
		return "", false
	}
	var envelope any
	if json.Unmarshal(result.Envelope, &envelope) != nil {
		return "", false
	}
	canonical, err := computerevent.CanonicalJSON(envelope)
	if err != nil {
		return "", false
	}
	return base64.RawURLEncoding.EncodeToString(canonical), false
}

func issueGatewayTokenAt(gwURL, computerID string) string {
	if gwURL == "" {
		return ""
	}

	// Call the gateway's credential issuance endpoint.
	// This is the same endpoint used by the host autoputer's ExecStartPre.
	body := fmt.Sprintf(`{"computer_id":"%s"}`, computerID)
	url := strings.TrimRight(gwURL, "/") + "/provider/v1/credentials/issue"

	ctx, cancel := context.WithTimeout(context.Background(), gatewayCredentialRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		log.Printf("vmctl: gateway token request creation failed for %s: %v", computerID, err)
		return ""
	}
	req.Header.Set("Content-Type", "application/json")
	hostauth.MarkInternal(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("vmctl: gateway token request failed for %s: %v", computerID, err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		log.Printf("vmctl: gateway token issue returned %d for %s", resp.StatusCode, computerID)
		return ""
	}

	var result struct {
		ComputerID       string `json:"computer_id"`
		ComputerIDCompat string `json:"ComputerID"`
		RawToken         string `json:"raw_token"`
		RawTokenCompat   string `json:"RawToken"`
		ExpiresAt        string `json:"expires_at"`
		ExpiresAtCompat  string `json:"ExpiresAt"`
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
	// Host overrides for the interactive guest shape. The retained staging
	// computer outgrew the 2 CPU / 2 GiB default (11 GiB texture/object-graph
	// store OOMs the 2 GiB guest on boot replay and snapshot scans,
	// 2026-09-03). Set VM_INTERACTIVE_CPU_COUNT / VM_INTERACTIVE_MEM_MIB on
	// the vmctl service to resize guests without a code change.
	interactiveVMCPUCountEnv   = "VM_INTERACTIVE_CPU_COUNT"
	interactiveVMMemSizeMibEnv = "VM_INTERACTIVE_MEM_MIB"
	platformVMCPUCount         = 2
	platformVMMemSizeMib       = 4096
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
	peer := ""
	if own := r.vmByID[vmID]; own != nil {
		peer = guestAddressOf(own.ComputerURL)
	}
	if nextCheck, ok := r.gatewayCredentialNextCheck[vmID]; ok && now.Before(nextCheck) && r.gatewayCredentialPeer[vmID] == peer {
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
	// The guest address moves the gateway's Jev pin to this boot's address;
	// the token outlives boots, the address does not.
	ensure := map[string]string{"raw_token": rawToken}
	if peer != "" {
		ensure["peer_ip"] = peer
	}
	bodyBytes, err := json.Marshal(ensure)
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
	hostauth.MarkInternal(req)
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
	r.gatewayCredentialPeer[vmID] = peer
	r.mu.Unlock()
}

// guestAddressOf is the IP address in a VM's runtime URL, or "" when the
// URL names none (a hostname, an empty or unparseable URL).
func guestAddressOf(computerURL string) string {
	u, err := url.Parse(strings.TrimSpace(computerURL))
	if err != nil {
		return ""
	}
	ip := net.ParseIP(u.Hostname())
	if ip == nil {
		return ""
	}
	return ip.String()
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
// of routing to a placeholder autoputer URL. If no VM exists, a new VM is
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
	return r.resolveDesktopContext(ctx, userID, desktopID, true, "")
}

func (r *OwnershipRegistry) resolveExistingDesktopContext(ctx context.Context, userID, desktopID, expectedVMID string) (*VMOwnership, error) {
	return r.resolveDesktopContext(ctx, userID, desktopID, false, expectedVMID)
}

func (r *OwnershipRegistry) resolveDesktopContext(ctx context.Context, userID, desktopID string, allowAssignment bool, expectedVMID string) (*VMOwnership, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	desktopID = normalizeDesktopID(desktopID)
	key := ownershipKey(userID, desktopID)
	r.mu.Lock()

	// Check if the desktop already has an active ownership.
	if own, ok := r.ownerships[key]; ok {
		if !allowAssignment && strings.TrimSpace(own.VMID) != strings.TrimSpace(expectedVMID) {
			r.mu.Unlock()
			return nil, fmt.Errorf("realized VM ownership changed for routed computer %s/%s", userID, desktopID)
		}
		// Another visit's claim is joined through pendingWaiters below; any
		// other path's claim refuses.
		if holder := r.lifecycle[key]; holder != "" && holder != lifecycleVisit {
			err := r.lifecycleConflictLocked(key)
			r.mu.Unlock()
			return nil, err
		}
		if own.IsReady() {
			mgr := r.vmManager
			snapshot := *own
			if activeOwnershipNeedsReadinessCheck(&snapshot, mgr) {
				if waiters, ok := r.pendingWaiters[key]; ok {
					return r.waitForPendingAssignmentLocked(ctx, key, userID, desktopID, waiters)
				}
				r.pendingWaiters[key] = nil
				r.lifecycle[key] = lifecycleVisit
				r.mu.Unlock()
				info, err := r.ensureActiveVMReady(&snapshot, mgr)
				if err != nil {
					log.Printf("vmctl: active VM %s readiness check failed: %v", snapshot.VMID, err)
					r.mu.Lock()
					waiters := r.pendingWaiters[key]
					delete(r.pendingWaiters, key)
					r.releaseLifecycleLocked(key, lifecycleVisit)
					r.mu.Unlock()
					for _, ch := range waiters {
						ch <- nil
					}
					return nil, fmt.Errorf("failed to verify active VM %s: %w", snapshot.VMID, err)
				}

				r.mu.Lock()
				waiters := r.pendingWaiters[key]
				delete(r.pendingWaiters, key)
				r.releaseLifecycleLocked(key, lifecycleVisit)
				current := r.ownerships[key]
				if current == nil || current.VMID != snapshot.VMID || !current.IsReady() {
					r.mu.Unlock()
					for _, ch := range waiters {
						ch <- nil
					}
					return r.resolveDesktopContext(ctx, userID, desktopID, allowAssignment, expectedVMID)
				}
				if info != nil {
					current.ComputerURL = info.HostURL
					current.Epoch = info.Epoch
				}
				current.State = VMStateActive
				current.LastActiveAt = time.Now()
				current.StoppedBy = ""
				publishStartLocked(current)
				r.saveLocked()
				vmID := current.VMID
				result := cloneOwnership(current)
				r.mu.Unlock()
				r.ensureExistingGatewayCredential(vmID)
				for _, ch := range waiters {
					ch <- cloneOwnership(result)
				}
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

		if own.State == VMStateBooting {
			if waiters, ok := r.pendingWaiters[key]; ok {
				return r.waitForPendingAssignmentLocked(ctx, key, userID, desktopID, waiters)
			}
		}

		// A retained VM that is stopped, unhealthy, failed, or stranded in a
		// stale boot is started again under the same VMID and data root. Failed
		// realizations receive a fresh epoch; no replacement computer is minted.
		if own.State == VMStateStopped || own.State == VMStateHibernated ||
			own.State == VMStateDegraded || own.State == VMStateFailed ||
			own.State == VMStateBooting {
			if waiters, ok := r.pendingWaiters[key]; ok {
				return r.waitForPendingAssignmentLocked(ctx, key, userID, desktopID, waiters)
			}
			mgr := r.vmManager
			pending := cloneOwnership(own)
			if pending.IsHeld() {
				// A held computer is never auto-started on resolve. It stays
				// stopped (or is transitioned to stopped); the epoch is not
				// incremented and no realization is booted. Only an authorized
				// maintenance recovery may move it.
				log.Printf("vmctl: computer %s is under maintenance hold; refusing auto-start on resolve", pending.ComputerID)
				if own.State != VMStateStopped {
					own.State = VMStateStopped
					r.discardStoppedSnapshotLocked(own, "held_stopped")
				}
				r.pendingWaiters[key] = nil
				r.saveLocked()
				result := cloneOwnership(own)
				r.mu.Unlock()
				return result, nil
			}
			recovering := pending.State == VMStateDegraded || pending.State == VMStateFailed || pending.State == VMStateBooting
			r.pendingWaiters[key] = nil
			r.lifecycle[key] = lifecycleVisit
			r.mu.Unlock()

			var info *VMInstanceInfo
			var err error
			if mgr != nil {
				info, err = r.startExistingVMCtx(ctx, pending, mgr)
				if err != nil {
					log.Printf("vmctl: start existing VM %s failed: %v", pending.VMID, err)
					r.mu.Lock()
					waiters := r.pendingWaiters[key]
					delete(r.pendingWaiters, key)
					r.releaseLifecycleLocked(key, lifecycleVisit)
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
				r.releaseLifecycleLocked(key, lifecycleVisit)
				r.mu.Unlock()
				for _, ch := range waiters {
					ch <- nil
				}
				return r.resolveDesktopContext(ctx, userID, desktopID, allowAssignment, expectedVMID)
			}
			if info != nil {
				current.ComputerURL = info.HostURL
				current.Epoch = info.Epoch
			} else if recovering {
				current.Epoch = r.nextEpoch()
			}
			current.State = VMStateActive
			current.LastActiveAt = time.Now()
			current.StoppedBy = ""
			publishStartLocked(current)
			r.saveLocked()
			waiters := r.pendingWaiters[key]
			delete(r.pendingWaiters, key)
			r.releaseLifecycleLocked(key, lifecycleVisit)
			result := cloneOwnership(current)
			r.mu.Unlock()
			r.ensureExistingGatewayCredential(result.VMID)
			for _, ch := range waiters {
				ch <- cloneOwnership(result)
			}
			log.Printf("vmctl: started retained VM %s for user %s desktop %s on resolve (epoch=%d)", result.VMID, userID, desktopID, result.Epoch)
			return result, nil
		}

	}
	if !allowAssignment {
		r.mu.Unlock()
		return nil, fmt.Errorf("no realized VM ownership for routed computer %s/%s", userID, desktopID)
	}

	// Check if a VM assignment is already in progress for this user/desktop.
	// The zero-waiter case still means a first caller is actively booting the VM,
	// so later callers must join that in-flight boot rather than minting a second
	// VM or routing to the placeholder autoputer URL.
	if waiters, ok := r.pendingWaiters[key]; ok {
		return r.waitForPendingAssignmentLocked(ctx, key, userID, desktopID, waiters)
	}

	// We are the first caller for this user/desktop pair. Create a new VM.
	// The boot-epoch reservation is deliberately deferred past the recovery
	// admission gate below: a deterministically refused start must not allocate
	// any realization resource, epoch included.
	vmID := generateVMID()

	// A new computer adds a VM: refuse it before minting anything when the
	// host would fall below its memory floor, counting the starts already in
	// flight (host_admission.go). The reservation ends with this boot.
	booted := false
	if r.vmManager != nil {
		candidate := &VMOwnership{VMID: vmID, UserID: userID, DesktopID: desktopID, Kind: VMKindInteractive,
			ComputerID: stableComputerID(userID, desktopID, "")}
		class := warmnessClassForOwnership(candidate, r.warmnessPolicy)
		ticket, err := r.admitHostMemoryLocked(candidate, class)
		if err != nil && makeRoomRefusal(err) && r.makeRoomEnabledLocked() {
			// Make room without holding r.mu across a hibernate (density
			// D1, M10). Nothing for this computer is registered yet, so the
			// lock may go; another request that registers it meanwhile wins
			// and this one starts over.
			stillNeeded := func() bool {
				r.mu.RLock()
				defer r.mu.RUnlock()
				return stillNeededLocked(r, key)
			}
			r.mu.Unlock()
			ticket, err = r.makeRoomAndAdmit(ctx, candidate, class, stillNeeded)
			if errors.Is(err, errMakeRoomSuperseded) {
				return r.resolveDesktopContext(ctx, userID, desktopID, allowAssignment, expectedVMID)
			}
			r.mu.Lock()
			if err == nil && !stillNeededLocked(r, key) {
				r.mu.Unlock()
				ticket.finish(false)
				return r.resolveDesktopContext(ctx, userID, desktopID, allowAssignment, expectedVMID)
			}
		}
		if err != nil {
			r.mu.Unlock()
			return nil, err
		}
		defer func() { ticket.finish(booted) }()
	}
	// A first boot is a spawn too (spawnClaimed gates the others).
	if r.hostStopping.Load() {
		r.mu.Unlock()
		return nil, errHostStopping
	}
	mgr := r.vmManager
	epoch := r.nextEpoch()

	own := &VMOwnership{
		VMID:         vmID,
		UserID:       userID,
		DesktopID:    desktopID,
		ComputerID:   stableComputerID(userID, desktopID, ""),
		Kind:         VMKindInteractive,
		ComputerURL:  r.autoputerURLForVM(vmID),
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
	if err := r.writePersistenceLocked(); err != nil {
		delete(r.ownerships, key)
		delete(r.vmByID, vmID)
		delete(r.pendingWaiters, key)
		r.mu.Unlock()
		return nil, fmt.Errorf("persist initial realization for VM %s: %w", vmID, err)
	}

	corpusdConfigured := strings.TrimSpace(r.corpusdURL) != ""
	r.mu.Unlock()

	// Retained-aware fail-fast before any gateway/credential/boot resource:
	// a fresh realization has a provably empty store, so the shared planner can
	// refuse deterministically when the existing chain demands a base whose
	// remaining tail exceeds the recovery bound.
	if mgr != nil {
		if refusal := r.admitRecoveryStart(own, true); refusal != nil {
			r.mu.Lock()
			own.State = VMStateFailed
			r.saveLocked()
			waiters := r.pendingWaiters[key]
			delete(r.pendingWaiters, key)
			r.mu.Unlock()
			for _, ch := range waiters {
				ch <- nil
			}
			return nil, refusal
		}
	}

	// Boot the real Firecracker VM if a manager is configured.
	if mgr != nil {
		reservedEpoch, reserveErr := mgr.ReserveBootEpoch(vmID, epoch)
		if reserveErr != nil {
			r.mu.Lock()
			own.State = VMStateFailed
			r.saveLocked()
			waiters := r.pendingWaiters[key]
			delete(r.pendingWaiters, key)
			r.mu.Unlock()
			for _, ch := range waiters {
				ch <- nil
			}
			return nil, fmt.Errorf("reserve initial realization for VM %s: %w", vmID, reserveErr)
		}
		epoch = reservedEpoch
		r.mu.Lock()
		if r.epochCounter < epoch {
			r.epochCounter = epoch
		}
		own.Epoch = epoch
		r.saveLocked()
		r.mu.Unlock()

		// Issue a gateway token for the VM autoputer before booting.
		// The token is written to the persistent directory by the vmmanager
		// and read by the guest init script to authenticate to the gateway.
		gwToken := r.issueGatewayToken(vmID)

		realizationID := realizationIDFor(vmID, epoch)
		credentialEnvelope := r.issueComputerCredentialEnvelope(own.ComputerID, realizationID, epoch)
		var info *VMInstanceInfo
		var err error
		if credentialEnvelope == "" && corpusdConfigured {
			err = fmt.Errorf("realization credential unavailable")
		} else {
			shapeCPU, shapeMem := interactiveMachineShape()
			warmness := r.bootWarmnessClass(own)
			info, err = mgr.BootVM(VMManagerConfig{
				VMID:                       vmID,
				ComputerID:                 own.ComputerID,
				RealizationID:              realizationID,
				Epoch:                      epoch,
				GuestPort:                  8085,
				MachineCPUCount:            shapeCPU,
				MachineMemSizeMib:          shapeMem,
				GatewayToken:               gwToken,
				ComputerCredentialEnvelope: credentialEnvelope,
				ComputerKind:               "active",
				OwnerID:                    userID,
				DesktopID:                  desktopID,
				WarmnessClass:              string(warmness),
				NewComputer:                true,
			})
		}
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
			return nil, fmt.Errorf("failed to boot VM %s: %w", vmID, r.noteRecoveryStartFailure(own, err))
		}
		r.mu.Lock()
		own.ComputerURL = info.HostURL
		own.Epoch = info.Epoch
		r.mu.Unlock()
		booted = true
		r.clearRecoveryCondition(own)
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

// guestBindTier ranks how far an ownership's computer URL names the guest
// calling from that address now: 2 for a running VM (its URL was set when
// its boot completed), 1 for a booting VM (its URL is a placeholder or the
// one from its last boot until this boot completes), 0 for a stopped,
// hibernated or failed computer, whose old URL may already belong to a
// newer computer.
func guestBindTier(state VMState) int {
	switch state {
	case VMStateActive, VMStateDegraded, VMStateStopping:
		return 2
	case VMStateBooting:
		return 1
	}
	return 0
}

// GuestOwnershipForAddress is the one rule that binds a guest's source
// address to its computer: the single live ownership whose computer URL
// host is that address, a running VM's claim before a booting one's. It
// returns nil when no live ownership claims the address, or when more than
// one claims it in the deciding tier (fail closed rather than guess).
// Stale ownerships never bind: walking every ownership and taking the first
// match bound a live guest to a stopped computer that once had its address,
// at random (B12, docs/problems/reeng-cut2-breakage-2026-10-13.md).
func (r *OwnershipRegistry) GuestOwnershipForAddress(host string) *VMOwnership {
	host = strings.TrimSpace(host)
	if r == nil || host == "" {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	claims := map[int][]*VMOwnership{}
	for _, own := range r.ownerships {
		if own == nil {
			continue
		}
		tier := guestBindTier(own.State)
		if tier == 0 {
			continue
		}
		u, err := url.Parse(strings.TrimSpace(own.ComputerURL))
		if err != nil || !strings.EqualFold(u.Hostname(), host) {
			continue
		}
		claims[tier] = append(claims[tier], own)
	}
	for tier := 2; tier >= 1; tier-- {
		switch len(claims[tier]) {
		case 0:
			continue
		case 1:
			snapshot := *claims[tier][0]
			return &snapshot
		default:
			log.Printf("vmctl: guest address %s is claimed by %d live computers; binding none", host, len(claims[tier]))
			return nil
		}
	}
	return nil
}

// GetOwnershipByComputerID returns the unique ownership for a stable
// ComputerID without applying a user-ID filter. Callers must authorize the
// ComputerID before invoking this internal control-plane lookup.
func (r *OwnershipRegistry) GetOwnershipByComputerID(computerID string) *VMOwnership {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var match *VMOwnership
	for _, own := range r.ownerships {
		if own == nil || stableComputerID(own.UserID, own.DesktopID, own.ComputerID) != computerID {
			continue
		}
		if match != nil {
			return nil
		}
		match = own
	}
	if match == nil {
		return nil
	}
	snapshot := *match
	return &snapshot
}

// reconcileLookupReadiness prevents a cached active ownership from outliving
// the guest health evidence that made it routable. It never starts or replaces
// a VM: lookup remains non-constructive, and lifecycle recovery stays explicit.
func (r *OwnershipRegistry) reconcileLookupReadiness(own *VMOwnership) *VMOwnership {
	if own == nil {
		return own
	}
	if own.State == VMStateDegraded {
		// Degraded is not terminal: the realization may have recovered after
		// the health check that latched it. Re-probe on lookup and re-promote
		// the same realization — no start, replace, or assignment. A held
		// computer still promotes: holds gate auto-start, not routing, and a
		// held active ownership already serves.
		return r.reconcileDegradedOwnership(own)
	}
	if !own.IsReady() {
		return own
	}

	key := ownershipKey(own.UserID, own.DesktopID)
	r.mu.RLock()
	mgr := r.vmManager
	refreshing := r.lifecycleRebootingLocked(key)
	r.mu.RUnlock()
	if mgr == nil {
		return own
	}
	if refreshing {
		snapshot := *own
		snapshot.State = VMStateBooting
		return &snapshot
	}

	healthy, healthErr := mgr.CheckHealth(own.VMID)
	if healthErr == nil && healthy {
		return own
	}
	if activeVMCanRouteDuringHealthGrace(mgr.GetVM(own.VMID), time.Now()) {
		if healthErr != nil {
			log.Printf("vmctl: active VM %s lookup health check failed during unhealthy-route grace; keeping active: %v", own.VMID, healthErr)
		} else {
			log.Printf("vmctl: active VM %s lookup health check returned unhealthy during unhealthy-route grace; keeping active", own.VMID)
		}
		return own
	}

	r.mu.Lock()
	current := r.ownerships[key]
	if current == nil {
		r.mu.Unlock()
		return nil
	}
	if current.VMID != own.VMID || current.Epoch != own.Epoch || current.State != VMStateActive {
		result := cloneOwnership(current)
		r.mu.Unlock()
		return result
	}
	if refreshing = r.lifecycleRebootingLocked(key); refreshing {
		result := cloneOwnership(current)
		result.State = VMStateBooting
		r.mu.Unlock()
		return result
	}
	current.State = VMStateDegraded
	r.saveLocked()
	result := cloneOwnership(current)
	r.mu.Unlock()

	if healthErr != nil {
		log.Printf("vmctl: active VM %s lookup health check failed; marked degraded: %v", own.VMID, healthErr)
	} else {
		log.Printf("vmctl: active VM %s lookup health check returned unhealthy; marked degraded", own.VMID)
	}
	return result
}

// reconcileDegradedOwnership re-probes a degraded realization on lookup and
// re-promotes it to active when the guest health check passes. It is
// non-constructive: it never starts, replaces, or assigns a VM, and it only
// promotes the same VMID/epoch that latched degraded. Without this branch a
// degraded latch is terminal — one transient health failure would refuse
// routed ownership forever even though the guest recovered.
func (r *OwnershipRegistry) reconcileDegradedOwnership(own *VMOwnership) *VMOwnership {
	key := ownershipKey(own.UserID, own.DesktopID)
	r.mu.RLock()
	mgr := r.vmManager
	refreshing := r.lifecycleRebootingLocked(key)
	r.mu.RUnlock()
	if mgr == nil {
		return own
	}
	if refreshing {
		snapshot := *own
		snapshot.State = VMStateBooting
		return &snapshot
	}

	healthy, healthErr := mgr.CheckHealth(own.VMID)
	if healthErr != nil || !healthy {
		if healthErr != nil {
			log.Printf("vmctl: degraded VM %s lookup re-probe failed; staying degraded: %v", own.VMID, healthErr)
		}
		return own
	}

	r.mu.Lock()
	current := r.ownerships[key]
	if current == nil {
		r.mu.Unlock()
		return nil
	}
	if current.VMID != own.VMID || current.Epoch != own.Epoch || current.State != VMStateDegraded {
		result := cloneOwnership(current)
		r.mu.Unlock()
		return result
	}
	if refreshing = r.lifecycleRebootingLocked(key); refreshing {
		result := cloneOwnership(current)
		result.State = VMStateBooting
		r.mu.Unlock()
		return result
	}
	current.State = VMStateActive
	current.LastActiveAt = time.Now()
	current.StoppedBy = ""
	r.saveLocked()
	result := cloneOwnership(current)
	r.mu.Unlock()

	log.Printf("vmctl: degraded VM %s lookup health re-probe healthy; re-promoted to active", own.VMID)
	return result
}

// LiveComputerURL returns the live autoputer URL for the given user/desktop pair.
// It prefers the VM manager's live HostURL over the cached ownership record.
// Returns an error if the ownership does not exist and no live VM is found.
func (r *OwnershipRegistry) LiveComputerURL(userID, desktopID string) (string, error) {
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
		if strings.TrimSpace(snap.ComputerURL) != "" {
			return snap.ComputerURL, nil
		}
	}
	return "", fmt.Errorf("no live autoputer URL for %s/%s", userID, desktopID)
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
	if err := r.lifecycleConflictLocked(key); err != nil {
		return err
	}

	// Every product stop is a fresh realization boundary. Propagate manager
	// failures whenever it still tracks an instance, even if durable ownership
	// state is stale and says stopped.
	if r.vmManager != nil && (own.State == VMStateActive || own.State == VMStateDegraded || r.vmManager.GetVM(own.VMID) != nil) {
		if err := r.vmManager.StopVM(own.VMID); err != nil {
			return fmt.Errorf("stop VM %s: %w", own.VMID, err)
		}
	}
	// A process the manager no longer tracks (a failed reattach) is still a
	// running VM: stop is only true once it is gone.
	if err := r.reapUnmanagedLocked(own); err != nil {
		return err
	}

	own.State = VMStateStopped
	own.LastActiveAt = time.Now()
	r.discardStoppedSnapshotLocked(own, "stopped")
	r.saveLocked()
	log.Printf("vmctl: stopped VM %s for user %s desktop %s", own.VMID, userID, own.DesktopID)
	return nil
}

// MarkRecoveryInProgress fences premium warmness while recover_current owns
// the stopped-to-fresh-boot transition.
func (r *OwnershipRegistry) MarkRecoveryInProgress(userID, desktopID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	own, ok := r.ownerships[ownershipKey(userID, desktopID)]
	if !ok {
		return fmt.Errorf("no VM found for user %s desktop %s", userID, normalizeDesktopID(desktopID))
	}
	if own.State != VMStateStopped {
		return fmt.Errorf("VM %s is not stopped for recovery", own.VMID)
	}
	own.StoppedBy = "recover_current"
	r.saveLocked()
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
	if err := r.lifecycleConflictLocked(key); err != nil {
		return err
	}

	// Delegate to the real VM manager if available.
	if r.vmManager != nil && (own.State == VMStateActive || own.State == VMStateDegraded) {
		_ = r.vmManager.StopVM(own.VMID)
	}

	own.State = VMStateStopped
	r.discardStoppedSnapshotLocked(own, "ownership_removed")
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

	key := ownershipKey(userID, desktopID)
	own, ok := r.ownerships[key]
	if !ok {
		return fmt.Errorf("no VM found for user %s desktop %s", userID, normalizeDesktopID(desktopID))
	}
	if err := r.lifecycleConflictLocked(key); err != nil {
		return err
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
	return r.hibernateVMForDesktop(userID, desktopID, reason, nil)
}

// hibernateVMForDesktop is the hibernate. hostStop asks a manager that has
// the call for a host-stop capture bounded by the stop budget, which with
// allowBusy also takes a guest busy only with runs (S5); only
// HibernateForHostStop sets it.
func (r *OwnershipRegistry) hibernateVMForDesktop(userID, desktopID, reason string, hostStop *hostStopArg) error {
	r.mu.Lock()
	key := ownershipKey(userID, desktopID)
	own, ok := r.ownerships[key]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("no VM found for user %s desktop %s", userID, normalizeDesktopID(desktopID))
	}
	if err := r.lifecycleConflictLocked(key); err != nil {
		r.mu.Unlock()
		return err
	}

	if own.State != VMStateActive && own.State != VMStateDegraded {
		r.mu.Unlock()
		return fmt.Errorf("VM %s cannot be hibernated (state=%s)", own.VMID, own.State)
	}
	// A deploy refresh of this computer is in flight: it holds the guest's
	// planned-restart marker window, which a snapshot must never capture.
	// Skipped like a busy guest (never the kill path); the idle sweep, make-room
	// and the operator all arrive here.
	if r.deployRefreshInFlight != nil && r.deployRefreshInFlight(userID, desktopID) {
		vmID := own.VMID
		r.mu.Unlock()
		log.Printf("vmctl: hibernate of VM %s skipped: a deploy refresh of this computer is in flight", vmID)
		return fmt.Errorf("VM %s: %w (a deploy refresh is in flight)", vmID, errHibernateGuestBusy)
	}

	// Delegate to the real VM manager if available. A snapshot hibernate
	// (S3) fences the guest and writes its memory to disk, which takes
	// seconds: it runs outside r.mu under the per-computer refreshing guard,
	// so other computers' lookups and resolves are not stalled and this
	// computer's read as booting meanwhile.
	if mgr := r.vmManager; mgr != nil {
		vmID, epoch := own.VMID, own.Epoch
		// Density D6 S3: a hibernate of a selected computer returns its cold
		// page cache between the fence and the snapshot. Decided here, on
		// the ownership, never by a field of the VM's configuration.
		cacheReturn := r.hibernateCacheReturnForLocked(own)
		r.lifecycle[key] = lifecycleHibernate
		r.noteHibernateStartLocked()
		r.mu.Unlock()
		var mgrErr error
		if hs, ok := mgr.(hostStopHibernateManager); ok && hostStop != nil {
			mgrErr = hs.HibernateVMForHostStop(hostStop.ctx, vmID, hostStop.allowBusy)
		} else if withOpt, ok := mgr.(hibernateCacheReturnManager); ok && cacheReturn != nil {
			mgrErr = withOpt.HibernateVMWithCacheReturn(vmID, *cacheReturn)
		} else {
			mgrErr = mgr.HibernateVM(vmID)
		}
		busy := hibernateGuestBusy(mgrErr)
		if mgrErr != nil && !busy {
			log.Printf("vmctl: manager hibernate of VM %s: %v", vmID, mgrErr)
		}
		r.mu.Lock()
		r.noteHibernateEndLocked()
		r.releaseLifecycleLocked(key, lifecycleHibernate)
		if busy {
			// The guest's fence said it is working and the manager left it
			// running. Nothing changes: it is not hibernated, not touched,
			// and the caller counts a skip (never the kill path; density D1,
			// docs/problems/busy-guest-hibernate-takes-kill-path-2026-10-26.md).
			r.mu.Unlock()
			log.Printf("vmctl: hibernate of VM %s skipped: the guest is working and stays running", vmID)
			return fmt.Errorf("VM %s: %w", vmID, errHibernateGuestBusy)
		}
		own, ok = r.ownerships[key]
		if !ok || own.VMID != vmID || own.Epoch != epoch {
			r.mu.Unlock()
			return fmt.Errorf("VM %s ownership changed during hibernate", vmID)
		}
	}
	defer r.mu.Unlock()

	own.State = VMStateHibernated
	own.LastActiveAt = time.Now()
	own.StoppedBy = normalizeStopReason(reason)
	r.saveLocked()
	log.Printf("vmctl: hibernated VM %s for user %s desktop %s reason=%s (epoch=%d)", own.VMID, userID, own.DesktopID, own.StoppedBy, own.Epoch)
	return nil
}

// errHibernateGuestBusy is hibernateVMForDesktopWithReason's answer when the
// guest's own fence said it is working: the computer was left running and the
// caller counts a skip.
var errHibernateGuestBusy = errors.New("hibernate skipped: the guest is busy")

// hibernateGuestBusy reports whether the manager's hibernate error is a busy
// guest's refusal (vmmanager.HibernateGuestBusyError), found by method so
// vmctl need not import the manager.
func hibernateGuestBusy(err error) bool {
	var busy interface{ HibernateGuestBusy() bool }
	return err != nil && errors.As(err, &busy) && busy.HibernateGuestBusy()
}

func normalizeStopReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "idle"
	}
	return reason
}

// ReclaimPressureVMs hibernates bounded eligible computers when active
// pressure reclaim is enabled and the host is under pressure. Under the
// sustained signal (pressure_sustained.go) that is one computer, one reclaim
// at a time, after a sustained shortage and outside the hibernate
// cool-down; under the instant signal (the rollback) one sample decides and
// up to MaxCandidates go per pass.
func (r *OwnershipRegistry) ReclaimPressureVMs(ctx context.Context, guard ComputerVersionRouteGuard) int {
	r.mu.RLock()
	cfg := normalizePressureReclaimConfig(r.pressureReclaim)
	r.mu.RUnlock()
	if cfg.Signal == PressureReclaimSignalSustained {
		return r.reclaimSustained(ctx, guard, cfg)
	}
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
	own, _, err := r.resumeVMForDesktop(userID, desktopID, false)
	return own, err
}

// errNotAlwaysOnStart refuses an always-on sweep start whose computer is no
// longer one the tiers keep on, or whose automatic retry is spent; the
// registry was re-read under the lock, so the sweep's list was stale.
var errNotAlwaysOnStart = errors.New("not an always-on start now")

// errHostStopping refuses a resume once a planned host stop's pass began.
var errHostStopping = errors.New("the host is stopping; the computer is not resumed")

// resumeVMForDesktop starts a stopped or hibernated computer under its
// lifecycle claim. With alwaysOn (the always-on sweep) the eligibility is
// re-read, a failure stop is charged against the automatic retry, and the
// claim is taken in one locked step, so a stale sweep list can never
// authorize an uncharged boot; spent reports the charge.
func (r *OwnershipRegistry) resumeVMForDesktop(userID, desktopID string, alwaysOn bool) (*VMOwnership, bool, error) {
	var ensureVMID string
	defer func() {
		if ensureVMID != "" {
			r.ensureExistingGatewayCredential(ensureVMID)
		}
	}()
	if r.hostStopping.Load() {
		return nil, false, errHostStopping
	}
	r.mu.Lock()

	key := ownershipKey(userID, desktopID)
	own, ok := r.ownerships[key]
	if !ok {
		r.mu.Unlock()
		return nil, false, fmt.Errorf("no VM found for user %s desktop %s", userID, normalizeDesktopID(desktopID))
	}
	if err := r.lifecycleConflictLocked(key); err != nil {
		r.mu.Unlock()
		return nil, false, err
	}

	if own.State != VMStateStopped && own.State != VMStateHibernated {
		if own.State == VMStateActive || own.State == VMStateBooting {
			own.LastActiveAt = time.Now()
			if own.IsReady() {
				ensureVMID = own.VMID
			}
			r.mu.Unlock()
			return own, false, nil
		}
		r.mu.Unlock()
		return nil, false, fmt.Errorf("VM %s cannot be resumed (state=%s)", own.VMID, own.State)
	}
	var receipt failureWarmReceipt
	spent := false
	if alwaysOn {
		if !alwaysOnWarmEligibleLocked(own, normalizeWarmnessPolicyConfig(r.warmnessPolicy)) {
			r.mu.Unlock()
			return nil, false, fmt.Errorf("VM %s (stopped by %q) %w", own.VMID, own.StoppedBy, errNotAlwaysOnStart)
		}
		if stoppedByFailure(own.StoppedBy) {
			rec, ok := r.spendRetryLocked(own, false)
			if !ok {
				r.mu.Unlock()
				return nil, false, fmt.Errorf("VM %s automatic retry is spent: %w", own.VMID, errNotAlwaysOnStart)
			}
			receipt, spent = rec, true
		}
	}

	// Delegate to the real VM manager if available.
	mgr := r.vmManager
	vmID := own.VMID
	if mgr != nil {
		snapshot := *own
		// The start runs outside r.mu for as long as the manager takes; the
		// claim, taken in the same step as the charge, keeps every other
		// start out until the result is published.
		r.lifecycle[key] = lifecycleResume
		r.mu.Unlock()
		info, err := r.startExistingVM(&snapshot, mgr)
		r.mu.Lock()
		if err != nil {
			if spent && transientStartRefusal(err) {
				r.refundFailureWarmLocked(vmID, receipt)
				spent = false
			}
			r.releaseLifecycleLocked(key, lifecycleResume)
			r.mu.Unlock()
			return nil, spent, fmt.Errorf("failed to resume VM %s: %w", vmID, err)
		}
		r.releaseLifecycleLocked(key, lifecycleResume)
		own = r.ownerships[key]
		if own == nil || own.VMID != vmID {
			r.mu.Unlock()
			return nil, spent, fmt.Errorf("VM %s ownership changed during resume", vmID)
		}
		if info != nil {
			own.ComputerURL = info.HostURL
			own.Epoch = info.Epoch
		}
	}

	// Transition the stable computer to its newly booted disposable realization.
	own.State = VMStateActive
	own.LastActiveAt = time.Now()
	own.StoppedBy = ""
	publishStartLocked(own)
	r.saveLocked()
	log.Printf("vmctl: started VM %s for user %s desktop %s (epoch=%d)", own.VMID, userID, own.DesktopID, own.Epoch)
	ensureVMID = own.VMID
	r.mu.Unlock()
	return own, spent, nil
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

// RecoverVMForDesktop recovers a degraded/failed/stopped VM. It refuses a held
// computer: automatic recovery is exactly what the maintenance hold fences.
// The authorized maintenance path is RecoverVMForDesktopMaintenance.
func (r *OwnershipRegistry) RecoverVMForDesktop(userID, desktopID string) (*VMOwnership, error) {
	return r.recoverVMForDesktop(userID, desktopID, false, false)
}

// RecoverVMForDesktopMaintenance is the authorized maintenance exception: it
// recovers a held computer (the B14 cold-recover replay-only drive). It still
// boots the guest under the hold (RUNTIME_MAINTENANCE_HOLD=1) and, with
// replayOnly, into the one-shot replay drive that exits without starting the
// runtime or appending.
func (r *OwnershipRegistry) RecoverVMForDesktopMaintenance(userID, desktopID string, replayOnly bool) (*VMOwnership, error) {
	return r.recoverVMForDesktop(userID, desktopID, true, replayOnly)
}

func (r *OwnershipRegistry) recoverVMForDesktop(userID, desktopID string, maintenance, replayOnly bool) (*VMOwnership, error) {
	var ensureVMID string
	defer func() {
		if ensureVMID != "" {
			r.ensureExistingGatewayCredential(ensureVMID)
		}
	}()
	key := ownershipKey(userID, desktopID)
	r.mu.Lock()
	own, ok := r.ownerships[key]
	if !ok {
		r.mu.Unlock()
		return nil, fmt.Errorf("no VM found for user %s desktop %s", userID, normalizeDesktopID(desktopID))
	}
	held := own.IsHeld()
	if err := r.lifecycleConflictLocked(key); err != nil {
		r.mu.Unlock()
		return nil, err
	}

	if own.State != VMStateDegraded && own.State != VMStateFailed && own.State != VMStateStopped {
		r.mu.Unlock()
		return nil, fmt.Errorf("VM %s is not in a recoverable state (state=%s)", own.VMID, own.State)
	}
	if held && !maintenance {
		r.mu.Unlock()
		return nil, fmt.Errorf("VM %s (computer %s) is under maintenance hold: recovery refused", own.VMID, own.ComputerID)
	}

	mgr := r.vmManager
	var wasStopped bool
	var vmid string
	var snapshot VMOwnership
	if mgr != nil {
		snapshot = *own
		wasStopped = own.State == VMStateStopped && mgr.GetVM(own.VMID) == nil
		vmid = own.VMID
		// Claimed until this function returns, after the publish (the boot
		// below runs outside r.mu); every exit after this point has unlocked.
		r.lifecycle[key] = lifecycleRecover
		defer r.releaseLifecycle(key, lifecycleRecover)
	}
	r.mu.Unlock()

	var info *VMInstanceInfo
	if mgr != nil {
		// No recovery spawns over the process of an interrupted start whose
		// close is open, or over any process the manager does not track:
		// both are ended only by the close or an explicit stop
		// (boot_claim.go, review round 2, F2). The maintenance path too.
		if err := r.refuseWhileCloseOpen(vmid, mgr); err != nil {
			return nil, fmt.Errorf("recover VM %s: %w", vmid, err)
		}
		// Authorized maintenance recovery (the B14 replay-only drive) is an
		// explicit operator path and stays exempt from the recovery-bound
		// admission below: refusing it would remove the evidence-gathering
		// repair boot. It is not exempt from the open-close refusal above,
		// which spawned nothing and clears with the close. Automatic recovery
		// is gated by both.
		if !maintenance {
			if refusal := r.admitRecoveryStart(&snapshot, false); refusal != nil {
				return nil, refusal
			}
		}
		cfg, err := r.reserveFreshVMConfig(&snapshot, "", mgr)
		if err != nil {
			return nil, fmt.Errorf("reserve recovery realization for VM %s: %w", vmid, err)
		}
		cfg.MaintenanceHold = held
		cfg.RecoveryReplayOnly = replayOnly
		if wasStopped {
			info, err = r.spawnClaimed(vmid, bootClaimBoot, cfg.Epoch, func() (*VMInstanceInfo, error) { return mgr.BootVM(cfg) })
			if err != nil {
				return nil, fmt.Errorf("failed to boot recovered VM %s: %w", vmid, r.noteRecoveryStartFailure(&snapshot, err))
			}
		} else {
			info, err = r.spawnClaimed(vmid, bootClaimBoot, cfg.Epoch, func() (*VMInstanceInfo, error) { return mgr.RecoverVM(vmid, cfg) })
			if err != nil {
				return nil, fmt.Errorf("failed to recover VM %s: %w", vmid, r.noteRecoveryStartFailure(&snapshot, err))
			}
		}
		r.clearRecoveryCondition(&snapshot)
	}

	r.mu.Lock()
	own = r.ownerships[key]
	if own == nil {
		r.mu.Unlock()
		return nil, fmt.Errorf("ownership vanished during recovery for user %s desktop %s", userID, normalizeDesktopID(desktopID))
	}
	if mgr != nil && info != nil {
		own.ComputerURL = info.HostURL
		own.Epoch = info.Epoch
	} else {
		own.Epoch = r.nextEpoch()
	}
	own.State = VMStateActive
	own.LastActiveAt = time.Now()
	own.StoppedBy = ""
	publishStartLocked(own)
	r.saveLocked()
	log.Printf("vmctl: recovered VM %s for user %s desktop %s (new_epoch=%d, fresh-boot)", own.VMID, userID, own.DesktopID, own.Epoch)
	ensureVMID = own.VMID
	r.mu.Unlock()
	return own, nil
}

// RefreshVMForDesktop force-reboots a computer onto the current guest image
// while preserving persistent user data. This is for deploy-time image refresh
// and owner-scoped recovery from stale boot artifacts, not crash recovery.
func (r *OwnershipRegistry) RefreshVMForDesktop(userID, desktopID string) (*VMOwnership, error) {
	return r.refreshVMForDesktop(userID, desktopID, nil)
}

// RefreshVMForDesktopWithMaintenance refreshes a running computer and runs
// maintenance on its data image between the kill and the boot
// (store_compaction.go). It refuses a computer that is not running.
func (r *OwnershipRegistry) RefreshVMForDesktopWithMaintenance(userID, desktopID string, maintenance func(dataImagePath string) error) (*VMOwnership, error) {
	if maintenance == nil {
		return nil, fmt.Errorf("refresh with maintenance: no maintenance given")
	}
	return r.refreshVMForDesktop(userID, desktopID, maintenance)
}

func (r *OwnershipRegistry) refreshVMForDesktop(userID, desktopID string, maintenance func(dataImagePath string) error) (*VMOwnership, error) {
	var ensureVMID string
	defer func() {
		if ensureVMID != "" {
			r.ensureExistingGatewayCredential(ensureVMID)
		}
	}()

	key := ownershipKey(userID, desktopID)
	r.mu.Lock()
	// Every refresh (deploy, retry sweep, compaction) ends here; a host stop
	// is capturing computers and a refresh would kill the one it captures.
	if r.hostStopping.Load() {
		r.mu.Unlock()
		return nil, errHostStopping
	}
	own, ok := r.ownerships[key]
	if !ok {
		r.mu.Unlock()
		return nil, fmt.Errorf("no VM found for user %s desktop %s", userID, normalizeDesktopID(desktopID))
	}
	if own.IsHeld() {
		r.mu.Unlock()
		return nil, fmt.Errorf("VM %s (computer %s) is under maintenance hold; refusing refresh", own.VMID, own.ComputerID)
	}
	if own.State != VMStateActive &&
		own.State != VMStateBooting &&
		own.State != VMStateStopped &&
		own.State != VMStateHibernated &&
		own.State != VMStateDegraded &&
		own.State != VMStateFailed {
		r.mu.Unlock()
		return nil, fmt.Errorf("VM %s is not refreshable (state=%s)", own.VMID, own.State)
	}
	if err := r.lifecycleConflictLocked(key); err != nil {
		r.mu.Unlock()
		return nil, err
	}

	mgr := r.vmManager
	snapshot := *own
	missingManagerInstance := mgr != nil && mgr.GetVM(own.VMID) == nil &&
		(own.State == VMStateStopped || own.State == VMStateHibernated || own.State == VMStateFailed)
	if maintenance != nil && (mgr == nil || own.State != VMStateActive || missingManagerInstance) {
		r.mu.Unlock()
		return nil, fmt.Errorf("VM %s refresh with maintenance needs a running VM (state=%s)", own.VMID, own.State)
	}
	// Released after the publish below.
	r.lifecycle[key] = lifecycleRefresh
	r.mu.Unlock()
	defer r.releaseLifecycle(key, lifecycleRefresh)

	var info *VMInstanceInfo
	if mgr != nil {
		// A durable refusal gates refresh only when the computer is not
		// currently serving: a healthy running realization is never stopped by
		// stale recovery evidence.
		if snapshot.State != VMStateActive {
			if refusal := r.admitRecoveryStart(&snapshot, false); refusal != nil {
				return nil, refusal
			}
		}
		cfg, err := r.reserveFreshVMConfig(&snapshot, "", mgr)
		if err != nil {
			return nil, fmt.Errorf("reserve refresh realization for VM %s: %w", snapshot.VMID, err)
		}
		if !missingManagerInstance && maintenance != nil {
			cfg.OfflineMaintenance = maintenance
			cfg.PostMaintenanceCredential = r.maintenanceCredentialIssuer(cfg)
		}
		info, err = r.spawnClaimed(snapshot.VMID, bootClaimBoot, cfg.Epoch, func() (*VMInstanceInfo, error) {
			if missingManagerInstance {
				return mgr.BootVM(cfg)
			}
			return mgr.RefreshVM(snapshot.VMID, cfg)
		})
		if err != nil {
			r.mu.Lock()
			current := r.ownerships[key]
			if current != nil && current.VMID == snapshot.VMID && current.Epoch == snapshot.Epoch && current.State == snapshot.State {
				current.State = VMStateFailed
				current.LastActiveAt = time.Now()
				r.saveLocked()
			}
			r.mu.Unlock()
			return nil, fmt.Errorf("failed to refresh VM %s: %w", snapshot.VMID, r.noteRecoveryStartFailure(&snapshot, err))
		}
		r.clearRecoveryCondition(&snapshot)
	}

	r.mu.Lock()
	own = r.ownerships[key]
	if own == nil || own.VMID != snapshot.VMID {
		r.mu.Unlock()
		if mgr != nil {
			_ = mgr.StopVM(snapshot.VMID)
		}
		return nil, fmt.Errorf("VM %s ownership changed during refresh", snapshot.VMID)
	}
	if own.Epoch != snapshot.Epoch || own.State != snapshot.State {
		currentEpoch, currentState := own.Epoch, own.State
		r.mu.Unlock()
		return nil, fmt.Errorf("VM %s lifecycle changed during refresh (epoch=%d state=%s, started epoch=%d state=%s)", snapshot.VMID, currentEpoch, currentState, snapshot.Epoch, snapshot.State)
	}
	if info != nil {
		own.ComputerURL = info.HostURL
		own.Epoch = info.Epoch
	} else {
		own.Epoch = r.nextEpoch()
	}
	own.State = VMStateActive
	own.LastActiveAt = time.Now()
	own.StoppedBy = ""
	publishStartLocked(own)
	r.saveLocked()
	log.Printf("vmctl: refreshed VM %s for user %s desktop %s (new_epoch=%d, deploy-image-refresh)", own.VMID, userID, own.DesktopID, own.Epoch)
	ensureVMID = own.VMID
	result := cloneOwnership(own)
	r.mu.Unlock()
	return result, nil
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

	key := ownershipKey(userID, desktopID)
	own, ok := r.ownerships[key]
	if !ok {
		return nil // no VM for this user, idempotent
	}
	if err := r.lifecycleConflictLocked(key); err != nil {
		return err
	}

	// Delegate to the real VM manager if available.
	if r.vmManager != nil && (own.State == VMStateActive || own.State == VMStateDegraded) {
		_ = r.vmManager.StopVM(own.VMID)
	}
	if err := r.reapUnmanagedLocked(own); err != nil {
		log.Printf("vmctl: logout for user %s: %v", userID, err)
	}

	own.State = VMStateStopped
	own.LastActiveAt = time.Now()
	own.StoppedBy = "logout"
	r.discardStoppedSnapshotLocked(own, "logout")
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
	return r.checkIdleOwnerships(r.hibernationHold(r.guestBusy(), time.Now))
}

// IdleOwnershipsSeen is CheckIdleOwnerships from the guests' last recorded
// busy answers, without probing any guest. For health only.
func (r *OwnershipRegistry) IdleOwnershipsSeen() []*VMOwnership {
	return r.checkIdleOwnerships(r.hibernationHold(r.guestBusySeenAnswer(), time.Now))
}

func (r *OwnershipRegistry) checkIdleOwnerships(busy func(*VMOwnership) bool) []*VMOwnership {
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
		ownerships = append(ownerships, r.cloneWithActivityLocked(own))
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
		if pressureCfg.Signal == PressureReclaimSignalSustained {
			// The keepalive drops only on a sustained shortage, never on
			// one burst during a sweep.
			pressure.ReclaimPressure = r.SustainedPressureState().Sustained
		}
	}

	now := time.Now()
	candidates := idleOwnershipCandidates(ownerships, warmnessPolicy, pressure, idleTimeout, now, busy)
	idle := make([]*VMOwnership, 0, len(candidates))
	for _, candidate := range candidates {
		idle = append(idle, candidate.own)
	}
	return idle
}

// guestHealthReading is what one /health answer says about a guest.
type guestHealthReading struct {
	// Busy: live work, or an answer that cannot be taken as idle.
	Busy bool
	// ReportsCapsules: the answer carried live_capsules (density D3a). A
	// live capsule is Busy; a guest that lacks the field is not known to be
	// free of one.
	ReportsCapsules bool
}

// guestBusy returns the busy-check closure used by the idle sweep and
// pressure reclaim. It fails closed (Phase 0 step 3): only an explicit 200
// /health reporting no work is idle. An unreachable, non-200 (a replaying
// guest answers 503) or unreadable answer is busy — a stopped computer loses
// whatever it was doing, and a truly dead guest is the wedge watchdog's.
func (r *OwnershipRegistry) guestBusy() func(*VMOwnership) bool {
	probe := r.guestHealthProbe
	if probe == nil {
		if busyProbe := r.guestBusyProbe; busyProbe != nil {
			probe = func(own *VMOwnership) guestHealthReading {
				return guestHealthReading{Busy: busyProbe(own), ReportsCapsules: true}
			}
		} else {
			probe = httpGuestHealth()
		}
	}
	return func(own *VMOwnership) bool {
		reading := probe(own)
		if own != nil {
			r.mu.Lock()
			if r.guestBusySeen == nil {
				r.guestBusySeen = make(map[string]bool)
			}
			r.guestBusySeen[own.VMID] = reading.Busy
			if reading.Busy {
				if r.guestLastBusyAt == nil {
					r.guestLastBusyAt = make(map[string]time.Time)
				}
				r.guestLastBusyAt[own.VMID] = r.now()
			}
			if r.guestReportsCapsules == nil {
				r.guestReportsCapsules = make(map[string]bool)
			}
			r.guestReportsCapsules[own.VMID] = reading.ReportsCapsules
			r.mu.Unlock()
		}
		return reading.Busy
	}
}

// cloneWithActivityLocked is cloneOwnership with LastActiveAt moved to the
// later of it and the last time a probe found the guest busy, for the
// hibernation deciders' idle arithmetic (density D3). The caller holds r.mu.
func (r *OwnershipRegistry) cloneWithActivityLocked(own *VMOwnership) *VMOwnership {
	c := cloneOwnership(own)
	if c != nil {
		if busyAt := r.guestLastBusyAt[c.VMID]; busyAt.After(c.LastActiveAt) {
			c.LastActiveAt = busyAt
		}
	}
	return c
}

// holdReasonLegacyGuest is why a hibernation decider leaves a guest alone
// that is idle past the short timer but not past the legacy one.
const holdReasonLegacyGuest = "guest_does_not_report_live_capsules"

// hibernationHoldReason answers, for a decider that hibernates (the idle
// sweep, pressure reclaim, make-room), why a guest must be left alone: "busy",
// holdReasonLegacyGuest, or "" when it may go. The legacy rule (density D3a)
// holds a guest whose /health lacks live_capsules until it has been idle
// legacyGuestIdleTimeout; it is not busy forever. Deploy refresh uses
// guestBusy directly, so it still reaches old guests.
func (r *OwnershipRegistry) hibernationHoldReason(busy func(*VMOwnership) bool, now func() time.Time) func(*VMOwnership) string {
	return func(own *VMOwnership) string {
		if busy(own) {
			return "busy"
		}
		r.mu.RLock()
		legacy := r.legacyGuestIdleTimeout
		reports := r.guestReportsCapsules[own.VMID]
		// Idle is measured from the later of LastActiveAt and the last time
		// a probe found the guest busy, as the plan, the sweep and reclaim
		// measure it, whatever clone the caller holds (review 312).
		lastActive := own.LastActiveAt
		if busyAt := r.guestLastBusyAt[own.VMID]; busyAt.After(lastActive) {
			lastActive = busyAt
		}
		r.mu.RUnlock()
		if legacy <= 0 || reports {
			return ""
		}
		if !lastActive.IsZero() && now().Sub(lastActive) > legacy {
			return ""
		}
		return holdReasonLegacyGuest
	}
}

// hibernationHold is hibernationHoldReason as a busy check.
func (r *OwnershipRegistry) hibernationHold(busy func(*VMOwnership) bool, now func() time.Time) func(*VMOwnership) bool {
	reason := r.hibernationHoldReason(busy, now)
	return func(own *VMOwnership) bool { return reason(own) != "" }
}

// guestBusySeenAnswer answers from the last recorded probe. A guest never
// probed counts as busy, as an unanswered probe does.
func (r *OwnershipRegistry) guestBusySeenAnswer() func(*VMOwnership) bool {
	return func(own *VMOwnership) bool {
		if own == nil {
			return true
		}
		r.mu.RLock()
		defer r.mu.RUnlock()
		busy, seen := r.guestBusySeen[own.VMID]
		return !seen || busy
	}
}

func httpGuestBusy() func(*VMOwnership) bool {
	probe := httpGuestHealth()
	return func(own *VMOwnership) bool { return probe(own).Busy }
}

func httpGuestHealth() func(*VMOwnership) guestHealthReading {
	client := &http.Client{Timeout: 4 * time.Second}
	busy := guestHealthReading{Busy: true}
	return func(own *VMOwnership) guestHealthReading {
		base := strings.TrimRight(strings.TrimSpace(own.ComputerURL), "/")
		if base == "" {
			// No address: the guest cannot be asked, so it cannot be shown
			// idle. Fail closed like every other unanswered probe.
			return busy
		}
		req, err := http.NewRequest(http.MethodGet, base+"/health", nil)
		if err != nil {
			return busy
		}
		hostauth.MarkInternal(req)
		resp, err := client.Do(req)
		if err != nil {
			return busy
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return busy
		}
		var health struct {
			RunningRuns          int `json:"running_runs"`
			DeskPendingMutations int `json:"desk_pending_mutations"`
			SelfdevActiveOps     int `json:"selfdev_active_operations"`
			// LiveCapsules is a pointer: absent is a guest that predates
			// the field, not zero live capsules.
			LiveCapsules *int `json:"live_capsules"`
		}
		if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&health); err != nil {
			return busy
		}
		// running_runs covers in-flight activations; desk_pending_mutations
		// covers the gap between them — a controller that owes the document
		// another desk run is not idle even when running_runs==0.
		// selfdev_active_operations covers the mid-transaction states
		// (executing/accepted/materializing/rollback_pending) whose loss is
		// not restart-repairable; parked states are excluded so a wedged op
		// cannot pin the guest busy forever. live_capsules (density D3a)
		// covers the capsules those three miss: a parked run's, one before
		// its first Freeze, a frozen candidate awaiting approval.
		// Receipts: docs/problems/vmctl-pressure-reclaim-mid-respawn-gap-2026-09-28.md,
		// docs/problems/capsule-subject-artifact-ephemeral-2026-09-28.md,
		// docs/problems/idle-hibernate-blind-to-live-capsules-2026-10-26.md
		reading := guestHealthReading{ReportsCapsules: health.LiveCapsules != nil}
		reading.Busy = health.RunningRuns > 0 || health.DeskPendingMutations > 0 || health.SelfdevActiveOps > 0 ||
			(health.LiveCapsules != nil && *health.LiveCapsules > 0)
		return reading
	}
}

// setGuestBusyProbeForTest pins the busy signal deterministically.
func (r *OwnershipRegistry) setGuestBusyProbeForTest(probe func(*VMOwnership) bool) {
	r.guestBusyProbe = probe
}

// setGuestHealthProbeForTest pins the busy signal and whether the guest
// reports live_capsules.
func (r *OwnershipRegistry) setGuestHealthProbeForTest(probe func(*VMOwnership) guestHealthReading) {
	r.guestHealthProbe = probe
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

type alwaysOnWarmTarget struct {
	userID    string
	desktopID string
	vmID      string
}

// failureWarmWindow bounds automatic warms of a computer that stopped by
// failing: at most one per window, unless a deploy of a different build
// re-arms the retry (GrantDeployRetry). The window is the time backstop; the
// deploy is the external event.
const failureWarmWindow = 24 * time.Hour

func stoppedByFailure(stoppedBy string) bool {
	return stoppedBy == stoppedByWedged || stoppedBy == stoppedByRecoveryFailed || stoppedBy == stoppedByStartInterruptedRepeatedly
}

func failureWarmAllowed(lastWarmedAt string, now time.Time) bool {
	if strings.TrimSpace(lastWarmedAt) == "" {
		return true
	}
	last, err := time.Parse(time.RFC3339Nano, lastWarmedAt)
	if err != nil {
		// An unreadable record fails closed: no automatic warm.
		return false
	}
	return !now.Before(last.Add(failureWarmWindow))
}

// deployGrantsCap bounds the consumed deploy targets remembered inside one
// retry window. A grant past it is refused rather than forgetting a target
// the window still needs (fail closed: the time backstop still applies).
const deployGrantsCap = 64

// deployGrant is one consumed deploy target and when it re-armed the retry.
type deployGrant struct {
	Target    string `json:"target"`
	GrantedAt string `json:"granted_at"`
}

// expired reports a grant older than the retry window; an unreadable time
// never expires (fail closed: the target stays consumed).
func (g deployGrant) expired(now time.Time) bool {
	at, err := time.Parse(time.RFC3339Nano, g.GrantedAt)
	return err == nil && !now.Before(at.Add(failureWarmWindow))
}

// recordFailureWarm durably spends the automatic retry before the boot, so
// a crash between the record and the boot loses the retry rather than
// repeating it. It refuses if the retry was already spent.
func (r *OwnershipRegistry) recordFailureWarm(vmID string) bool {
	_, ok := r.spendFailureWarm(vmID)
	return ok
}

// failureWarmReceipt identifies one spend of the automatic retry: the value
// it replaced and the generation it wrote. A refund restores the old value
// only while the generation is still current.
type failureWarmReceipt struct {
	previous string
	gen      uint64
}

// spendFailureWarm is a standalone spend of the always-on retry: only a
// computer that is stopped by a failure now, and that no path holds, is
// charged. The sweep itself charges inside resumeVMForDesktop's claim step.
func (r *OwnershipRegistry) spendFailureWarm(vmID string) (failureWarmReceipt, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	own, found := r.vmByID[vmID]
	if !found || own == nil {
		return failureWarmReceipt{}, false
	}
	if _, held := r.lifecycle[ownershipKey(own.UserID, own.DesktopID)]; held {
		return failureWarmReceipt{}, false
	}
	return r.spendRetryLocked(own, false)
}

// spendDeployRetry is a deploy rescue's spend: every rescue that spawns a
// process is charged against the target's allowance whether the computer was
// stopped by a failure or a vmctl restart, or left failed. It charges only
// while the caller holds the rescue claim, so no other path's spend or start
// can fall between the rescue's eligibility and its charge.
func (r *OwnershipRegistry) spendDeployRetry(vmID string) (failureWarmReceipt, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	own, found := r.vmByID[vmID]
	if !found || own == nil || r.lifecycle[ownershipKey(own.UserID, own.DesktopID)] != lifecycleRescue {
		return failureWarmReceipt{}, false
	}
	return r.spendRetryLocked(own, true)
}

// spendRetryLocked judges the registry's entry now, under the lock, never a
// caller's snapshot: a computer that is running again, or stopped for another
// reason, is not charged and nothing is written for a refund to undo. anyStop
// is the rescue's reach (a vmctl-restart stop, or a failed computer).
func (r *OwnershipRegistry) spendRetryLocked(own *VMOwnership, anyStop bool) (failureWarmReceipt, bool) {
	switch {
	case own.State == VMStateFailed && anyStop:
	case own.State != VMStateStopped:
		return failureWarmReceipt{}, false
	case !stoppedByFailure(own.StoppedBy) && !(anyStop && stoppedByRestart(own.StoppedBy)):
		return failureWarmReceipt{}, false
	}
	now := r.now()
	if !failureWarmAllowed(own.FailureWarmedAt, now) {
		return failureWarmReceipt{previous: own.FailureWarmedAt, gen: own.failureWarmGen}, false
	}
	rec := failureWarmReceipt{previous: own.FailureWarmedAt}
	own.FailureWarmedAt = now.UTC().Format(time.RFC3339Nano)
	own.failureWarmGen++
	rec.gen = own.failureWarmGen
	r.saveLocked()
	return rec, true
}

// refundFailureWarm gives back a retry spent by a start that was refused
// before anything spawned (host memory pressure, a live guest still
// reattaching). It is a compare-and-restore: if a later grant or spend has
// touched FailureWarmedAt since this receipt, the newer state stands.
func (r *OwnershipRegistry) refundFailureWarm(vmID string, rec failureWarmReceipt) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refundFailureWarmLocked(vmID, rec)
}

func (r *OwnershipRegistry) refundFailureWarmLocked(vmID string, rec failureWarmReceipt) {
	own, ok := r.vmByID[vmID]
	if !ok || own == nil || rec.gen == 0 || own.failureWarmGen != rec.gen {
		return
	}
	own.FailureWarmedAt = rec.previous
	own.failureWarmGen++
	r.saveLocked()
}

// rescueClaim is the outcome of claiming a computer for a deploy rescue.
type rescueClaim int

const (
	// rescueClaimed: the computer is stopped by a failure or a vmctl restart,
	// or failed, and the caller holds its lifecycle claim (releaseRescue).
	rescueClaimed rescueClaim = iota + 1
	// rescueAlreadyActive: the computer is running again (a reattach landed
	// after the caller's snapshot); the ordinary planned refresh applies.
	rescueAlreadyActive
	// rescueBusy: a refresh, a resume or a recovery owns the computer now.
	rescueBusy
	// rescueNotNeeded: held, gone, or stopped for a reason a deploy does not
	// reach.
	rescueNotNeeded
)

// claimRescue re-reads the computer under the lock and, if it is still a
// rescue, takes its lifecycle claim before any boot starts, so every other
// start path sees "in progress" and none of them boots over the rescue (or
// the rescue over them); the charge follows under the claim
// (spendDeployRetry). The returned snapshot is the registry's, not the
// caller's.
func (r *OwnershipRegistry) claimRescue(userID, desktopID string) (VMOwnership, VMManager, rescueClaim) {
	key := ownershipKey(userID, desktopID)
	r.mu.Lock()
	defer r.mu.Unlock()
	own := r.ownerships[key]
	if own == nil || own.IsHeld() {
		return VMOwnership{}, nil, rescueNotNeeded
	}
	switch own.State {
	case VMStateActive:
		return *own, nil, rescueAlreadyActive
	case VMStateBooting:
		return *own, nil, rescueBusy
	}
	if r.lifecycleConflictLocked(key) != nil {
		return *own, nil, rescueBusy
	}
	if !deployRescueStopped(own.State, own.StoppedBy, warmnessClassForOwnership(own, r.warmnessPolicy), own.DesktopID) {
		return *own, nil, rescueNotNeeded
	}
	r.lifecycle[key] = lifecycleRescue
	return *own, r.vmManager, rescueClaimed
}

func (r *OwnershipRegistry) releaseRescue(userID, desktopID string) {
	r.releaseLifecycle(ownershipKey(userID, desktopID), lifecycleRescue)
}

// rescueBoot boots a claimed, stopped computer onto the current deploy. It is
// the deploy path, not the crash-recovery path: a stopped instance the manager
// still tracks (the wedge detector's StopVM keeps it) is refreshed
// (RefreshVM: the current kernel, initrd, store and parameters), never
// recovered with the config the hung boot used; an untracked one is booted.
// Both get a freshly issued gateway token. The caller holds the claim.
func (r *OwnershipRegistry) rescueBoot(own VMOwnership, mgr VMManager) (*VMOwnership, error) {
	if mgr == nil {
		return nil, fmt.Errorf("VM realization authority is unavailable")
	}
	tracked := mgr.GetVM(own.VMID) != nil
	// A rescue never starts over a live guest (deployRescue defers), so the
	// boot always adds a VM, tracked or not: admit it and reserve its memory.
	r.mu.RLock()
	class := warmnessClassForOwnership(&own, r.warmnessPolicy)
	r.mu.RUnlock()
	ticket, err := r.admitHostMemory(&own, class)
	if err != nil {
		return nil, err
	}
	info, err := r.rescueLaunch(&own, mgr, tracked)
	ticket.finish(err == nil && info != nil)
	if err != nil {
		return nil, err
	}
	key := ownershipKey(own.UserID, own.DesktopID)
	r.mu.Lock()
	cur := r.ownerships[key]
	if cur == nil || cur.VMID != own.VMID {
		r.mu.Unlock()
		_ = mgr.StopVM(own.VMID)
		return nil, fmt.Errorf("VM %s ownership changed during the rescue boot", own.VMID)
	}
	if info != nil {
		cur.ComputerURL = info.HostURL
		cur.Epoch = info.Epoch
	} else {
		cur.Epoch = r.nextEpoch()
	}
	cur.State = VMStateActive
	cur.LastActiveAt = time.Now()
	cur.StoppedBy = ""
	publishStartLocked(cur)
	r.saveLocked()
	log.Printf("vmctl: rescued VM %s for user %s desktop %s onto the current deploy (epoch=%d, crash boot)", cur.VMID, cur.UserID, cur.DesktopID, cur.Epoch)
	result := cloneOwnership(cur)
	r.mu.Unlock()
	r.ensureExistingGatewayCredential(result.VMID)
	return result, nil
}

func (r *OwnershipRegistry) rescueLaunch(own *VMOwnership, mgr VMManager, tracked bool) (*VMInstanceInfo, error) {
	if err := r.admitRecoveryStart(own, false); err != nil {
		return nil, err
	}
	cfg, err := r.reserveFreshVMConfig(own, r.issueGatewayToken(own.VMID), mgr)
	if err != nil {
		return nil, err
	}
	info, err := r.spawnClaimed(own.VMID, bootClaimBoot, cfg.Epoch, func() (*VMInstanceInfo, error) {
		if tracked {
			return mgr.RefreshVM(own.VMID, cfg)
		}
		return mgr.BootVM(cfg)
	})
	if err != nil {
		return nil, r.noteRecoveryStartFailure(own, err)
	}
	r.clearRecoveryCondition(own)
	return info, nil
}

// markRecoveryFailed turns a stop by a vmctl restart into a recovery failure
// after a rescue boot failed hard, so the always-on sweep's bound applies to
// it instead of retrying the boot every pass.
func (r *OwnershipRegistry) markRecoveryFailed(vmID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if own, ok := r.vmByID[vmID]; ok && own != nil && own.State == VMStateStopped && stoppedByRestart(own.StoppedBy) {
		own.StoppedBy = stoppedByRecoveryFailed
		r.saveLocked()
	}
}

// transientStartRefusal reports whether err is an admission refusal that
// spawned nothing and clears by itself.
// IsTransientStartRefusal reports a start refused before anything spawned
// for a reason that clears by itself (host memory pressure, a live guest
// still reattaching, an interrupted start's close still open, another start
// in progress): the caller may retry.
func IsTransientStartRefusal(err error) bool { return transientStartRefusal(err) }

func transientStartRefusal(err error) bool {
	if errors.Is(err, errLifecycleBusy) || errors.Is(err, errHostStopping) {
		return true
	}
	refusal := RecoveryRefusalFrom(err)
	if refusal == nil {
		return false
	}
	switch refusal.Kind {
	case RecoveryRefusalHostMemoryPressure, RecoveryRefusalGuestReattachPending:
		return true
	}
	return false
}

// GrantDeployRetry is the effect of a real deploy on a failure-stopped
// always-on computer: a deploy of a build the computer has not been re-armed
// for is an external event (docs/restart-subsystem-design-2026-10-13.md
// section 1), so it opens a new episode with a fresh allowance of one
// automatic retry. It clears FailureWarmedAt once per distinct target and
// remembers every target it consumed for at least failureWarmWindow, so the
// same target posted again (a CI re-run, the retry sweep, a duplicate
// request) and A, B, ..., A never re-arm inside the window; past
// deployGrantsCap targets in one window it refuses instead of forgetting
// one. The bound is one automatic boot per (computer, deploy target) after a
// failure stop, plus the failureWarmWindow backstop. It
// applies only to protected, unheld computers. The store compaction is a
// maintenance refresh, not a deploy, and never calls it. It reports whether
// it re-armed.
func (r *OwnershipRegistry) GrantDeployRetry(vmID, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	own, ok := r.vmByID[vmID]
	if !ok || own == nil || own.IsHeld() {
		return false
	}
	if !protectedWarmnessClass(warmnessClassForOwnership(own, r.warmnessPolicy)) {
		return false
	}
	now := r.now()
	kept := make([]deployGrant, 0, len(own.DeployGrants)+1)
	for _, g := range own.DeployGrants {
		if g.expired(now) {
			continue
		}
		if g.Target == target {
			return false
		}
		kept = append(kept, g)
	}
	if len(kept) >= deployGrantsCap {
		log.Printf("vmctl: deploy target %s not granted to vm=%s: %d targets already consumed inside the retry window", target, vmID, len(kept))
		return false
	}
	own.DeployGrants = append(kept, deployGrant{Target: target, GrantedAt: now.UTC().Format(time.RFC3339Nano)})
	own.FailureWarmedAt = ""
	own.failureWarmGen++
	r.saveLocked()
	log.Printf("vmctl: deploy target %s re-arms the automatic retry of protected computer vm=%s", target, vmID)
	return true
}

// alwaysOnBootClass reports whether the tiers keep a computer of this class
// on by booting it: a platform computer that is critical_protected, and a
// paid account's primary computer. (The public platform computer has its own
// warmer.)
func alwaysOnBootClass(class WarmnessClass, desktopID string) bool {
	switch class {
	case WarmnessClassCriticalProtected:
		return true
	case WarmnessClassPremiumAlwaysOn:
		return normalizeDesktopID(desktopID) == PrimaryDesktopID
	}
	return false
}

// alwaysOnWarmTargets lists the stopped or hibernated computers the tiers
// keep on: a paid account's primary computer, and every platform computer.
// A held computer, or one the owner stopped to recover, is left alone.
func (r *OwnershipRegistry) alwaysOnWarmTargets() []alwaysOnWarmTarget {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cfg := normalizeWarmnessPolicyConfig(r.warmnessPolicy)
	now := r.now()
	targets := make([]alwaysOnWarmTarget, 0)
	for _, own := range r.ownerships {
		if own == nil || !alwaysOnWarmEligibleLocked(own, cfg) {
			continue
		}
		// A computer stopped because it wedged or failed recovery gets one
		// automatic warm per failureWarmWindow, and one more for each deploy
		// of a different build (GrantDeployRetry), so a transient fault heals
		// without the owner (ruling 2026-10-10), a rollback can land, and a
		// repeating fault cannot loop. Interim bound until the boot ledger
		// (restart station L1b) replaces it; delete FailureWarmedAt and
		// DeployGrants then. This list is advisory: the start re-reads and
		// charges under the claim (resumeVMForDesktop).
		if stoppedByFailure(own.StoppedBy) && !failureWarmAllowed(own.FailureWarmedAt, now) {
			continue
		}
		targets = append(targets, alwaysOnWarmTarget{userID: own.UserID, desktopID: own.DesktopID, vmID: own.VMID})
	}
	return targets
}

// alwaysOnWarmEligibleLocked reports a stopped or hibernated, unheld
// computer the tiers keep on that the owner did not stop to recover.
func alwaysOnWarmEligibleLocked(own *VMOwnership, cfg WarmnessPolicyConfig) bool {
	if own.IsHeld() || own.StoppedBy == "recover_current" {
		return false
	}
	if own.State != VMStateStopped && own.State != VMStateHibernated {
		return false
	}
	return alwaysOnBootClass(warmnessClassForOwnership(own, cfg), own.DesktopID)
}

// WarmAlwaysOnDesktops resumes the computers the tiers keep on that already
// have an ownership record; it never creates ownerships.
func (r *OwnershipRegistry) WarmAlwaysOnDesktops(ctx context.Context, guard ComputerVersionRouteGuard) int {
	targets := r.alwaysOnWarmTargets()
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
		// Eligibility, the charge of a failure stop and the claim are one
		// locked step; a refusal that spawned nothing is refunded inside it.
		_, spent, err := r.resumeVMForDesktop(target.userID, target.desktopID, true)
		if spent {
			log.Printf("vmctl: warmness policy spent the automatic retry for always-on desktop vm=%s after a failure stop; the next is allowed after %s or for the next deploy target", target.vmID, failureWarmWindow)
		}
		if err != nil {
			log.Printf("vmctl: warmness policy failed to resume always-on desktop vm=%s user=%s desktop=%s: %v", target.vmID, target.userID, target.desktopID, err)
			continue
		}
		warmed++
	}
	return warmed
}

// SetAutoputerCredential stores the gateway credential for a VM's autoputer.
func (r *OwnershipRegistry) SetAutoputerCredential(vmID, credential string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	own, ok := r.vmByID[vmID]
	if !ok {
		return fmt.Errorf("no VM found with ID %s", vmID)
	}

	own.AutoputerCredential = credential
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

// autoputerURLForVM generates the autoputer URL for a given VM ID.
// In production, this would resolve to the actual VM's network address.
// For host-process mode during development, all VMs route to the same
// host autoputer at the configured base URL.
func (r *OwnershipRegistry) autoputerURLForVM(vmID string) string {
	// In the current host-process mode, all VMs share the same autoputer
	// URL. When Firecracker is integrated, this will return per-VM URLs
	// based on the VM's assigned network address.
	return r.autoputerURLBase
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
