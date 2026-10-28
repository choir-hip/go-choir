package vmctl

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
)

// S5 busy hibernate, vmctl's half: what a planned host stop does with each
// running computer (docs/problems/paper-never-checkpointed-2026-10-27.md,
// slice S5). Written before the code. Ways it can fail:
//   - H1 a computer with a live capsule, or a self-development apply, is
//     offered the busy capture (CLAUDE.md, always-busy note);
//   - H2 a guest whose /health cannot be read, or lacks live_capsules, is
//     offered the busy capture (fail closed: not known to be free of one);
//   - H3 a computer whose deploy refresh is in flight is captured (its
//     planned-restart marker window);
//   - H4 a captured computer is not recorded hibernated, so the next vmctl
//     start sweeps its snapshot as an orphan;
//   - H5 a busy guest that refused is recorded hibernated (it is still
//     running);
//   - H6 a manager without the host-stop call is asked anyway;
//   - H7 the protected computers are not first, so a stop deadline spends
//     itself on test computers;
//   - H8 a computer that is not running is touched;
//   - H9 a cancelled stop deadline still starts new captures.

type hostStopManager struct {
	*processControlVMManager
	mu       sync.Mutex
	order    []string
	plain    []string
	hostStop []string
	plainErr map[string]error
	stopErr  map[string]error
}

func (m *hostStopManager) HibernateVM(vmID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.plain = append(m.plain, vmID)
	m.order = append(m.order, vmID)
	return m.plainErr[vmID]
}

func (m *hostStopManager) HibernateVMForHostStop(_ context.Context, vmID string, allowBusy bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.order = append(m.order, vmID)
	if !allowBusy { // a stop-budgeted hibernate without the busy capture
		m.plain = append(m.plain, vmID)
		return m.plainErr[vmID]
	}
	m.hostStop = append(m.hostStop, vmID)
	return m.stopErr[vmID]
}

func newHostStopManager() *hostStopManager {
	return &hostStopManager{processControlVMManager: newProcessControlVMManager(), plainErr: map[string]error{}, stopErr: map[string]error{}}
}

func intp(n int) *int { return &n }

func hostStopRegistry(t *testing.T, mgr VMManager, health map[string]hostStopHealth) *OwnershipRegistry {
	t.Helper()
	reg, _ := hostCapacityRegistry(t, mgr)
	reg.setHostStopHealthProbeForTest(func(own *VMOwnership) hostStopHealth {
		return health[own.VMID]
	})
	return reg
}

var freeHealth = hostStopHealth{Answered: true, LiveCapsules: intp(0)}

func TestHostStopOffersTheBusyCaptureOnlyToAComputerFreeOfCapsulesAndApplies(t *testing.T) {
	mgr := newHostStopManager()
	reg := hostStopRegistry(t, mgr, map[string]hostStopHealth{
		"vm-free":    freeHealth,
		"vm-capsule": {Answered: true, LiveCapsules: intp(1)},                   // H1
		"vm-apply":   {Answered: true, LiveCapsules: intp(0), SelfdevActive: 1}, // H1
		"vm-legacy":  {Answered: true},                                          // H2: no live_capsules
		"vm-silent":  {},                                                        // H2: no answer
	})
	for _, id := range []string{"vm-free", "vm-capsule", "vm-apply", "vm-legacy", "vm-silent"} {
		activeComputer(reg, id)
	}
	report := reg.HibernateForHostStop(context.Background())
	if len(mgr.hostStop) != 1 || mgr.hostStop[0] != "vm-free" {
		t.Fatalf("host-stop captures = %v, want only vm-free", mgr.hostStop)
	}
	if len(mgr.plain) != 4 {
		t.Fatalf("plain hibernates = %v, want the other four (a fenced snapshot if idle)", mgr.plain)
	}
	for _, id := range []string{"vm-free", "vm-capsule", "vm-apply", "vm-legacy", "vm-silent"} {
		own := reg.GetOwnershipByVMID(id)
		if own == nil || own.State != VMStateHibernated || own.StoppedBy != StopReasonHostStop {
			t.Fatalf("%s: %+v, want hibernated by host_stop (H4)", id, own)
		}
	}
	if report.Hibernated != 5 || report.BusyOffered != 1 {
		t.Fatalf("report = %+v", report)
	}
}

func TestHostStopLeavesARefusingBusyGuestRunning(t *testing.T) {
	mgr := newHostStopManager()
	mgr.stopErr["vm-a"] = busyErr{}
	mgr.plainErr["vm-b"] = busyErr{}
	reg := hostStopRegistry(t, mgr, map[string]hostStopHealth{"vm-a": freeHealth, "vm-b": {Answered: true, LiveCapsules: intp(2)}})
	activeComputer(reg, "vm-a")
	activeComputer(reg, "vm-b")
	report := reg.HibernateForHostStop(context.Background())
	for _, id := range []string{"vm-a", "vm-b"} {
		if own := reg.GetOwnershipByVMID(id); own == nil || own.State != VMStateActive {
			t.Fatalf("%s: %+v, want still active (H5)", id, own)
		}
	}
	if report.Skipped != 2 || report.Hibernated != 0 {
		t.Fatalf("report = %+v", report)
	}
}

func TestHostStopSkipsADeployRefreshInFlight(t *testing.T) {
	mgr := newHostStopManager()
	reg := hostStopRegistry(t, mgr, map[string]hostStopHealth{"vm-a": freeHealth})
	own := activeComputer(reg, "vm-a")
	reg.SetDeployRefreshInFlight(func(userID, desktopID string) bool { return userID == own.UserID })
	reg.HibernateForHostStop(context.Background())
	if len(mgr.hostStop)+len(mgr.plain) != 0 {
		t.Fatalf("manager called %v %v, want nothing (H3)", mgr.hostStop, mgr.plain)
	}
	if got := reg.GetOwnershipByVMID("vm-a"); got.State != VMStateActive {
		t.Fatalf("state = %s", got.State)
	}
}

func TestHostStopWithAManagerWithoutTheCall(t *testing.T) {
	mgr := newOptsManager() // HibernateVM and the cache-return call only
	reg := hostStopRegistry(t, mgr, map[string]hostStopHealth{"vm-a": freeHealth})
	activeComputer(reg, "vm-a")
	report := reg.HibernateForHostStop(context.Background())
	if len(mgr.plain) != 1 || report.BusyOffered != 0 {
		t.Fatalf("plain=%v report=%+v, want one plain hibernate (H6)", mgr.plain, report)
	}
}

func TestHostStopOrdersProtectedFirstAndTouchesOnlyRunningComputers(t *testing.T) {
	mgr := newHostStopManager()
	reg := hostStopRegistry(t, mgr, map[string]hostStopHealth{"vm-a": freeHealth, "vm-paper": freeHealth, "vm-z": freeHealth})
	activeComputer(reg, "vm-a")
	activeComputer(reg, "vm-z")
	paper := activeComputer(reg, "vm-paper")
	reg.mu.Lock()
	paper.WarmnessClass = WarmnessClassCriticalProtected
	reg.mu.Unlock()
	stopped := activeComputer(reg, "vm-stopped")
	reg.mu.Lock()
	stopped.State = VMStateStopped
	reg.mu.Unlock()
	reg.HibernateForHostStop(context.Background())
	if len(mgr.order) != 3 || mgr.order[0] != "vm-paper" {
		t.Fatalf("order = %v, want vm-paper first and vm-stopped untouched (H7, H8)", mgr.order)
	}
}

func TestHostStopCancelledDeadlineStartsNothing(t *testing.T) {
	mgr := newHostStopManager()
	reg := hostStopRegistry(t, mgr, map[string]hostStopHealth{"vm-a": freeHealth})
	activeComputer(reg, "vm-a")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	report := reg.HibernateForHostStop(ctx)
	if len(mgr.order) != 0 || report.NotStarted != 1 {
		t.Fatalf("order=%v report=%+v, want nothing started (H9)", mgr.order, report)
	}
}

// S5 review (DeepSeek): the idle sweep's always-on warm resumed a computer
// the pass had just hibernated, consuming its one-shot snapshot before the
// host stop ended it. Once the pass begins nothing resumes a computer.
func TestHostStopRefusesEveryResumeOnceThePassBegins(t *testing.T) {
	mgr := newHostStopManager()
	reg := hostStopRegistry(t, mgr, map[string]hostStopHealth{"vm-a": freeHealth})
	own := activeComputer(reg, "vm-a")
	reg.HibernateForHostStop(context.Background())
	if got := reg.GetOwnershipByVMID("vm-a"); got.State != VMStateHibernated {
		t.Fatalf("state = %s, want hibernated", got.State)
	}
	if _, _, err := reg.resumeVMForDesktop(own.UserID, own.DesktopID, true); !errors.Is(err, errHostStopping) {
		t.Fatalf("always-on resume after the pass: %v, want errHostStopping", err)
	}
	if _, err := reg.ResumeVMForDesktop(own.UserID, own.DesktopID); err == nil {
		t.Fatal("a visit resume after the pass succeeded")
	}
	if got := reg.GetOwnershipByVMID("vm-a"); got.State != VMStateHibernated {
		t.Fatalf("state = %s, want still hibernated", got.State)
	}
}

// S5 review (both): a computer that is not offered the busy capture still
// hibernates within the stop budget, not with an unbounded fence.
func TestHostStopBoundsEveryHibernateByTheBudget(t *testing.T) {
	mgr := newHostStopManager()
	reg := hostStopRegistry(t, mgr, map[string]hostStopHealth{"vm-silent": {}})
	activeComputer(reg, "vm-silent")
	reg.HibernateForHostStop(context.Background())
	if len(mgr.order) != 1 || len(mgr.plain) != 1 || len(mgr.hostStop) != 0 {
		t.Fatalf("order=%v plain=%v hostStop=%v, want one budgeted hibernate without the busy capture", mgr.order, mgr.plain, mgr.hostStop)
	}
}

// S5 review 374 (Devin 1): the platform computer's own warm calls the
// manager without resumeVMForDesktop. Every spawn goes through spawnClaimed,
// so that is where a host stop refuses it, for this path and any other.
func TestHostStopRefusesThePlatformWarmAndEverySpawn(t *testing.T) {
	mgr := &mockVMManager{getVMs: map[string]*VMInstanceInfo{UniversalWirePlatformVMID: {State: "stopped", Epoch: 7}}}
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetVMManager(mgr)
	key := ownershipKey(UniversalWirePlatformOwnerID, UniversalWirePlatformDesktopID)
	reg.mu.Lock()
	own := &VMOwnership{VMID: UniversalWirePlatformVMID, UserID: UniversalWirePlatformOwnerID, DesktopID: UniversalWirePlatformDesktopID,
		ComputerID: stableComputerID(UniversalWirePlatformOwnerID, UniversalWirePlatformDesktopID, ""), Kind: VMKindInteractive,
		WarmnessClass: WarmnessClassPublicPlatform, State: VMStateHibernated, StoppedBy: StopReasonHostStop, Epoch: 7}
	reg.ownerships[key] = own
	reg.vmByID[UniversalWirePlatformVMID] = own
	reg.mu.Unlock()

	reg.hostStopping.Store(true)
	allow := func(context.Context, string, string) error { return nil }
	if warmed := reg.WarmUniversalWirePlatformComputer(context.Background(), allow); warmed != 0 {
		t.Fatalf("warmed = %d during a host stop", warmed)
	}
	if _, err := reg.spawnClaimed(UniversalWirePlatformVMID, bootClaimBoot, 7, func() (*VMInstanceInfo, error) {
		t.Fatal("a spawn ran during a host stop")
		return nil, nil
	}); !errors.Is(err, errHostStopping) {
		t.Fatalf("spawnClaimed err = %v, want errHostStopping", err)
	}
	if len(mgr.resumes)+len(mgr.boots) != 0 {
		t.Fatalf("manager resumed %v booted %d", mgr.resumes, len(mgr.boots))
	}
	reg.mu.RLock()
	state := reg.ownerships[key].State
	reg.mu.RUnlock()
	if state != VMStateHibernated {
		t.Fatalf("state = %s, want the refused warm to leave it hibernated", state)
	}
}

// S5 review 378 F1: a deploy refresh that reaches the handler after the pass
// began (the direct POST, a sweep iteration past its top check, a compaction
// refresh) must not write a planned-restart marker into a guest the pass is
// about to capture, nor kill it.
func TestHostStopRefusesADeployRefreshBeforeItMarksTheGuest(t *testing.T) {
	h, mgr, fake := newDeployRefreshFixture(t, "owner-stopping")
	fake.outcome = guestMarkWritten
	h.registry.hostStopping.Store(true)
	code, resp := postDeployRefresh(t, h, `{"user_id":"owner-stopping","desktop_id":"primary","target":"761d94c5"}`)
	if code != http.StatusConflict || resp.Outcome != deployRefreshDeferred {
		t.Fatalf("refresh during a host stop: %d %+v, want 409 deferred", code, resp)
	}
	if len(fake.marks) != 0 || len(mgr.refreshes) != 0 {
		t.Fatalf("guest marked %v or refreshed %v during a host stop", fake.marks, mgr.refreshes)
	}
	// The registry's own refresh is a gate too (compaction maintenance).
	own := h.registry.GetOwnershipForDesktop("owner-stopping", PrimaryDesktopID)
	if _, err := h.registry.RefreshVMForDesktop(own.UserID, own.DesktopID); !errors.Is(err, errHostStopping) {
		t.Fatalf("RefreshVMForDesktop err = %v, want errHostStopping", err)
	}
}

// S5 review 378 F2: first boots are spawns too.
func TestHostStopRefusesFirstBoots(t *testing.T) {
	mgr := &mockVMManager{}
	reg := NewOwnershipRegistry("")
	reg.SetCorpusdURL(testComputerCredentialIssuerURL(t))
	reg.SetVMManager(mgr)
	reg.hostStopping.Store(true)
	if _, err := reg.ResolveOrAssign("owner-new"); !errors.Is(err, errHostStopping) {
		t.Fatalf("ResolveOrAssign err = %v, want errHostStopping", err)
	}
	if _, err := reg.ensureUniversalWirePlatformOwnership(context.Background()); !errors.Is(err, errHostStopping) {
		t.Fatalf("platform first boot err = %v, want errHostStopping", err)
	}
	if len(mgr.boots) != 0 {
		t.Fatalf("booted %d VM(s) during a host stop", len(mgr.boots))
	}
}
