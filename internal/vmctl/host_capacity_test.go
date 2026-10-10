package vmctl

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Node B global OOM 2026-10-10
// (docs/problems/node-b-oom-from-retained-qa-computers-2026-10-10.md):
// reattach skips left Firecracker processes running outside the registry,
// so pressure reclaim, the idle sweeper and stop could not see them, new
// computers kept booting onto a starved host, every VM inherited vmctl's
// OOM protection, and a QA computer stuck busy held memory all night.
//
// Failure modes pinned here:
//   - a live process behind a stopped ownership is never reaped (the leak);
//   - it is reaped before a reattach retry and a grace period (a transient
//     health failure at restart would kill a working computer);
//   - a premium or critical computer is ever reaped by reconcile;
//   - a process with no ownership at all is never reaped, or is reaped on
//     first sight (a boot in flight must not be raced);
//   - a manager-tracked running VM is touched by reconcile;
//   - stop reports "stopped" while the process still runs;
//   - a new computer boots onto a host below the memory floor;
//   - admission refuses a premium computer, or a recover of a running VM;
//   - VMs keep the inherited OOM priority (QA outlives the owner computer);
//   - busy protection never expires for a proof-account computer under
//     pressure, or expires for a premium or real user's computer.

type processControlVMManager struct {
	*mockVMManager
	live      map[string][]int
	reaped    []string
	reapFails bool
	oomAdj    map[string]int
}

func newProcessControlVMManager() *processControlVMManager {
	return &processControlVMManager{
		mockVMManager: &mockVMManager{getVMs: map[string]*VMInstanceInfo{}},
		live:          map[string][]int{},
		oomAdj:        map[string]int{},
	}
}

func (m *processControlVMManager) LiveFirecrackerVMs() map[string][]int {
	out := make(map[string][]int, len(m.live))
	for id, pids := range m.live {
		out[id] = append([]int(nil), pids...)
	}
	return out
}

func (m *processControlVMManager) ReapUnmanagedVM(vmID string) error {
	m.reaped = append(m.reaped, vmID)
	if m.reapFails {
		return errors.New("process still running")
	}
	delete(m.live, vmID)
	return nil
}

func (m *processControlVMManager) SetVMOOMScoreAdj(vmID string, adj int) error {
	m.oomAdj[vmID] = adj
	return nil
}

func hostCapacityRegistry(t *testing.T, mgr VMManager) (*OwnershipRegistry, *time.Time) {
	t.Helper()
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetVMManager(mgr)
	now := time.Date(2026, 10, 10, 7, 0, 0, 0, time.UTC)
	reg.clock = func() time.Time { return now }
	// Guests accept connections unless a test says otherwise (wedge.go).
	reg.guestNetworkProbe = func(string) bool { return true }
	return reg, &now
}

func addOwnership(reg *OwnershipRegistry, own *VMOwnership) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	key := ownershipKey(own.UserID, own.DesktopID)
	reg.ownerships[key] = own
	reg.vmByID[own.VMID] = own
}

// Phase 0 step 3: a live guest behind an ownership is a running computer.
// Reconcile keeps retrying its reattach and never powers it off on a timer;
// only wedge evidence (wedge_test.go) or an explicit stop ends it. Failure
// mode pinned: a busy proof-account computer (an M11 run) is killed ten
// minutes after a vmctl restart because its reattach health check was slow.
func TestReconcileNeverReapsALiveGuestWithAnOwnership(t *testing.T) {
	mgr := newProcessControlVMManager()
	mgr.reattachError = errors.New("guest health check failed")
	reg, now := hostCapacityRegistry(t, mgr)
	addOwnership(reg, &VMOwnership{VMID: "vm-qa", UserID: "qa-user", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive,
		State: VMStateStopped, StoppedBy: "vmctl-restart", ComputerURL: "http://10.0.0.2:8085"})
	mgr.live["vm-qa"] = []int{4242}

	for i := 0; i < 24; i++ {
		res := reg.ReconcileVMProcesses(context.Background(), allowAllRoutes)
		if res.Reaped != 0 || res.Wedged != 0 {
			t.Fatalf("pass %d reaped a live, connectable guest: %+v %v", i, res, mgr.reaped)
		}
		*now = now.Add(time.Hour)
	}
	if len(mgr.reaped) != 0 || len(mgr.reattaches) != 24 {
		t.Fatalf("reaped=%v reattach retries=%d, want none reaped and a retry every pass", mgr.reaped, len(mgr.reattaches))
	}
}

func TestReconcileNeverReapsProtectedComputer(t *testing.T) {
	mgr := newProcessControlVMManager()
	mgr.reattachError = errors.New("guest health check failed")
	reg, now := hostCapacityRegistry(t, mgr)
	reg.SetWarmnessPolicyConfig(WarmnessPolicyConfig{AlwaysOnUserIDs: map[string]bool{"owner": true}})
	addOwnership(reg, &VMOwnership{VMID: "vm-owner", UserID: "owner", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive,
		State: VMStateStopped, StoppedBy: "vmctl-restart", ComputerURL: "http://10.0.0.3:8085"})
	addOwnership(reg, &VMOwnership{VMID: "vm-critical", UserID: "crit", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive,
		WarmnessClass: WarmnessClassCriticalProtected, State: VMStateStopped, StoppedBy: "vmctl-restart", ComputerURL: "http://10.0.0.4:8085"})
	mgr.live["vm-owner"] = []int{1}
	mgr.live["vm-critical"] = []int{2}

	reg.ReconcileVMProcesses(context.Background(), allowAllRoutes)
	*now = now.Add(24 * time.Hour)
	res := reg.ReconcileVMProcesses(context.Background(), allowAllRoutes)
	if len(mgr.reaped) != 0 {
		t.Fatalf("protected computers reaped: %v", mgr.reaped)
	}
	if res.ProtectedUnmanaged != 2 {
		t.Fatalf("protected unmanaged = %d, want 2", res.ProtectedUnmanaged)
	}
}

func TestReconcileReattachesWhenGuestRecovers(t *testing.T) {
	mgr := newProcessControlVMManager()
	reg, _ := hostCapacityRegistry(t, mgr)
	addOwnership(reg, &VMOwnership{VMID: "vm-late", UserID: "late", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive,
		State: VMStateStopped, StoppedBy: "vmctl-restart", ComputerURL: "http://10.0.0.5:8085"})
	mgr.live["vm-late"] = []int{7}

	res := reg.ReconcileVMProcesses(context.Background(), allowAllRoutes)
	if res.Reattached != 1 || len(mgr.reaped) != 0 {
		t.Fatalf("result = %+v reaped = %v, want one reattach", res, mgr.reaped)
	}
	if own := reg.GetOwnershipByVMID("vm-late"); own == nil || own.State != VMStateActive {
		t.Fatalf("ownership after reattach = %+v", own)
	}
}

func TestReconcileReapsOrphanWithoutOwnershipOnSecondSight(t *testing.T) {
	mgr := newProcessControlVMManager()
	reg, _ := hostCapacityRegistry(t, mgr)
	mgr.live["vm-orphan"] = []int{1117847}
	mgr.live["vm-managed"] = []int{99}
	mgr.getVMs["vm-managed"] = &VMInstanceInfo{State: "running"}

	reg.ReconcileVMProcesses(context.Background(), allowAllRoutes)
	if len(mgr.reaped) != 0 {
		t.Fatalf("orphan reaped on first sight: %v", mgr.reaped)
	}
	res := reg.ReconcileVMProcesses(context.Background(), allowAllRoutes)
	if res.Reaped != 1 || len(mgr.reaped) != 1 || mgr.reaped[0] != "vm-orphan" {
		t.Fatalf("orphan not reaped on second sight (managed VM must be untouched): %+v %v", res, mgr.reaped)
	}
}

func TestStopReapsUnmanagedProcessAndFailsIfItSurvives(t *testing.T) {
	mgr := newProcessControlVMManager()
	reg, _ := hostCapacityRegistry(t, mgr)
	addOwnership(reg, &VMOwnership{VMID: "vm-stale", UserID: "stale", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive,
		State: VMStateStopped, StoppedBy: "vmctl-restart"})
	mgr.live["vm-stale"] = []int{11}

	if err := reg.StopVMForDesktop("stale", PrimaryDesktopID); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if len(mgr.reaped) != 1 {
		t.Fatalf("stop did not reap the unmanaged process: %v", mgr.reaped)
	}

	mgr.live["vm-stale"] = []int{12}
	mgr.reapFails = true
	if err := reg.StopVMForDesktop("stale", PrimaryDesktopID); err == nil {
		t.Fatal("stop reported success while the process survived")
	}
}

func TestHostMemoryAdmissionRefusesNewComputerBelowFloor(t *testing.T) {
	mgr := newProcessControlVMManager()
	reg, _ := hostCapacityRegistry(t, mgr)
	reg.SetPressureReclaimConfig(PressureReclaimConfig{
		Mode:                    PressureReclaimModeActive,
		MinMemoryAvailableBytes: 4 << 30,
	})
	available := uint64(5 << 30) // above the floor, but not after a new VM
	reg.setPressureSamplerForTest(func(cfg PressureReclaimConfig) HostPressureSample {
		return HostPressureSample{MemoryTotalBytes: 32 << 30, MemoryAvailableBytes: available, MemoryAvailablePercent: float64(available) / float64(32<<30) * 100}
	})

	_, err := reg.ResolveOrAssign("qa-new")
	refusal := RecoveryRefusalFrom(err)
	if refusal == nil || refusal.Kind != RecoveryRefusalHostMemoryPressure || refusal.RetryAfterSeconds <= 0 {
		t.Fatalf("new computer below floor: err=%v refusal=%+v", err, refusal)
	}
	if len(mgr.boots) != 0 {
		t.Fatalf("refused start booted a VM: %d boots", len(mgr.boots))
	}
	if reg.GetOwnershipForDesktop("qa-new", PrimaryDesktopID) != nil {
		t.Fatal("refused start left an ownership behind")
	}

	reg.SetWarmnessPolicyConfig(WarmnessPolicyConfig{AlwaysOnUserIDs: map[string]bool{"owner": true}})
	if _, err := reg.ResolveOrAssign("owner"); err != nil {
		t.Fatalf("premium computer refused under pressure: %v", err)
	}

	available = 24 << 30
	if _, err := reg.ResolveOrAssign("qa-new"); err != nil {
		t.Fatalf("new computer refused with ample memory: %v", err)
	}
}

func TestReconcileAppliesOOMPriorityByClass(t *testing.T) {
	mgr := newProcessControlVMManager()
	reg, _ := hostCapacityRegistry(t, mgr)
	reg.SetWarmnessPolicyConfig(WarmnessPolicyConfig{AlwaysOnUserIDs: map[string]bool{"owner": true}})
	addOwnership(reg, &VMOwnership{VMID: "vm-owner", UserID: "owner", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive})
	addOwnership(reg, &VMOwnership{VMID: "vm-qa", UserID: "qa", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive})
	mgr.live["vm-owner"] = []int{1}
	mgr.live["vm-qa"] = []int{2}
	mgr.getVMs["vm-owner"] = &VMInstanceInfo{State: "running"}
	mgr.getVMs["vm-qa"] = &VMInstanceInfo{State: "running"}

	reg.ReconcileVMProcesses(context.Background(), allowAllRoutes)
	if mgr.oomAdj["vm-owner"] != oomScoreAdjProtected || mgr.oomAdj["vm-qa"] != oomScoreAdjOrdinary {
		t.Fatalf("oom adj = %v, want owner %d qa %d", mgr.oomAdj, oomScoreAdjProtected, oomScoreAdjOrdinary)
	}
	if oomScoreAdjProtected >= oomScoreAdjOrdinary {
		t.Fatal("protected computers must be less likely OOM victims than ordinary ones")
	}
}

// Phase 0 step 3: busy protection never expires, for proof accounts too —
// a Gate 2 run on an example.com account can be busy for hours. Admission's
// memory floor, not reclaim of busy guests, keeps the host from OOM.
func TestPressureReclaimNeverTakesABusyComputer(t *testing.T) {
	reg, now := hostCapacityRegistry(t, newProcessControlVMManager())
	reg.SetPressureReclaimConfig(PressureReclaimConfig{
		Mode:                    PressureReclaimModeActive,
		MinIdle:                 time.Minute,
		MinMemoryAvailableBytes: 4 << 30,
		MaxCandidates:           5,
	})
	reg.setPressureSamplerForTest(func(cfg PressureReclaimConfig) HostPressureSample {
		return HostPressureSample{MemoryTotalBytes: 32 << 30, MemoryAvailableBytes: 1 << 30, MemoryAvailablePercent: 3}
	})
	busy := map[string]bool{"vm-qa": true}
	reg.setGuestBusyProbeForTest(func(own *VMOwnership) bool { return busy[own.VMID] })
	reg.SetRetentionPruneConfig(RetentionPruneConfig{EphemeralEmailDomains: []string{"example.com"}})
	reg.setRetentionUserEmailsForTest(map[string]string{"qa": "m11-selfdev@example.com", "idle": "idle@example.com"})
	idle := now.Add(-time.Hour)
	addOwnership(reg, &VMOwnership{VMID: "vm-qa", UserID: "qa", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive, LastActiveAt: idle})
	addOwnership(reg, &VMOwnership{VMID: "vm-idle", UserID: "idle", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive, LastActiveAt: idle})

	*now = now.Add(24 * time.Hour)
	got := reg.pressureReclaimActionCandidates()
	if len(got) != 1 || got[0].own.VMID != "vm-idle" {
		t.Fatalf("after a day busy, want only the idle computer eligible; got %d", len(got))
	}
}

var allowAllRoutes ComputerVersionRouteGuard = func(context.Context, string, string) error { return nil }

// docs/problems/vmctl-restart-reboots-busy-computer-2026-10-10.md: after a
// vmctl restart, a guest too busy to answer the reattach health check was
// killed as an "orphan" and rebooted by the next resolve (rerun 11, 08:57Z).
// Resolve must retry reattach and refuse with Retry-After, never boot over
// a live process; only an explicit stop or the wedge watchdog ends it.
func TestResolveNeverBootsOverLiveUnmanagedGuest(t *testing.T) {
	mgr := newProcessControlVMManager()
	reg, _ := hostCapacityRegistry(t, mgr)
	own := &VMOwnership{VMID: "vm-busy", UserID: "busy", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive,
		State: VMStateStopped, StoppedBy: "vmctl-restart", ComputerURL: "http://10.0.0.9:8085"}
	addOwnership(reg, own)
	mgr.live["vm-busy"] = []int{3949451}
	mgr.reattachError = errors.New("guest health check failed")

	_, err := reg.startExistingVM(own, mgr)
	var refusal *RecoveryRefusal
	if !errors.As(err, &refusal) || refusal.Kind != RecoveryRefusalGuestReattachPending || refusal.RetryAfterSeconds <= 0 {
		t.Fatalf("resolve over a live unmanaged guest = %v, want a guest_reattach_pending refusal", err)
	}
	if len(mgr.boots) != 0 || len(mgr.reaped) != 0 {
		t.Fatalf("resolve booted (%d) or reaped (%v) a live guest", len(mgr.boots), mgr.reaped)
	}

	mgr.reattachError = nil
	info, err := reg.startExistingVM(own, mgr)
	if err != nil || info == nil || len(mgr.boots) != 0 {
		t.Fatalf("resolve after the guest answers = %+v, %v (boots %d); want a reattach", info, err, len(mgr.boots))
	}
	if cur := reg.GetOwnershipByVMID("vm-busy"); cur == nil || cur.State != VMStateActive {
		t.Fatalf("ownership after reattach = %+v", cur)
	}
}
