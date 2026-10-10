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
	return reg, &now
}

func addOwnership(reg *OwnershipRegistry, own *VMOwnership) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	key := ownershipKey(own.UserID, own.DesktopID)
	reg.ownerships[key] = own
	reg.vmByID[own.VMID] = own
}

func TestReconcileReapsUnmanagedProcessOnlyAfterGrace(t *testing.T) {
	mgr := newProcessControlVMManager()
	mgr.reattachError = errors.New("guest health check failed")
	reg, now := hostCapacityRegistry(t, mgr)
	addOwnership(reg, &VMOwnership{VMID: "vm-qa", UserID: "qa-user", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive,
		State: VMStateStopped, StoppedBy: "vmctl-restart", ComputerURL: "http://10.0.0.2:8085"})
	mgr.live["vm-qa"] = []int{4242}

	res := reg.ReconcileVMProcesses(context.Background(), allowAllRoutes)
	if len(mgr.reaped) != 0 || res.Reaped != 0 {
		t.Fatalf("reaped on first sight: %v", mgr.reaped)
	}
	if len(mgr.reattaches) != 1 {
		t.Fatalf("reattach retries = %d, want 1 before any reap", len(mgr.reattaches))
	}

	*now = now.Add(unmanagedReapGraceDefault - time.Second)
	reg.ReconcileVMProcesses(context.Background(), allowAllRoutes)
	if len(mgr.reaped) != 0 {
		t.Fatalf("reaped inside the grace period: %v", mgr.reaped)
	}

	*now = now.Add(2 * time.Second)
	res = reg.ReconcileVMProcesses(context.Background(), allowAllRoutes)
	if res.Reaped != 1 || len(mgr.reaped) != 1 || mgr.reaped[0] != "vm-qa" {
		t.Fatalf("not reaped after grace: result=%+v reaped=%v", res, mgr.reaped)
	}
	own := reg.GetOwnershipByVMID("vm-qa")
	if own == nil || own.State != VMStateStopped || own.StoppedBy != stoppedByUnmanagedReap {
		t.Fatalf("ownership after reap = %+v", own)
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

func TestPressureReclaimBusyProtectionExpiresForProofComputers(t *testing.T) {
	reg, now := hostCapacityRegistry(t, newProcessControlVMManager())
	reg.SetWarmnessPolicyConfig(WarmnessPolicyConfig{AlwaysOnUserIDs: map[string]bool{"owner": true}})
	reg.SetPressureReclaimConfig(PressureReclaimConfig{
		Mode:                    PressureReclaimModeActive,
		MinIdle:                 time.Minute,
		MinMemoryAvailableBytes: 4 << 30,
		MaxBusyProtect:          2 * time.Hour,
		MaxCandidates:           5,
	})
	reg.setPressureSamplerForTest(func(cfg PressureReclaimConfig) HostPressureSample {
		return HostPressureSample{MemoryTotalBytes: 32 << 30, MemoryAvailableBytes: 1 << 30, MemoryAvailablePercent: 3}
	})
	reg.setGuestBusyProbeForTest(func(*VMOwnership) bool { return true })
	reg.SetRetentionPruneConfig(RetentionPruneConfig{EphemeralEmailDomains: []string{"example.com"}})
	reg.setRetentionUserEmailsForTest(map[string]string{"qa": "m11-selfdev@example.com", "real": "someone@realmail.org"})
	idle := now.Add(-time.Hour)
	addOwnership(reg, &VMOwnership{VMID: "vm-qa", UserID: "qa", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive, LastActiveAt: idle})
	addOwnership(reg, &VMOwnership{VMID: "vm-owner", UserID: "owner", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive, LastActiveAt: idle})
	addOwnership(reg, &VMOwnership{VMID: "vm-real", UserID: "real", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive, LastActiveAt: idle})

	if got := reg.pressureReclaimActionCandidates(); len(got) != 0 {
		t.Fatalf("busy computers eligible before the cap: %d", len(got))
	}
	*now = now.Add(2*time.Hour + time.Minute)
	got := reg.pressureReclaimActionCandidates()
	if len(got) != 1 || got[0].own.VMID != "vm-qa" {
		t.Fatalf("after the cap, want only the proof-account computer eligible (premium and real users keep busy protection); got %d", len(got))
	}
}

var allowAllRoutes ComputerVersionRouteGuard = func(context.Context, string, string) error { return nil }
