package vmctl

import (
	"bytes"
	"context"
	"errors"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWarmUniversalWirePlatformComputerPersistsResumeFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ownership.json")
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	if err := reg.SetPersistencePath(path); err != nil {
		t.Fatalf("SetPersistencePath: %v", err)
	}
	// The manager tracks the stopped VM, so the warm resumes it (an untracked
	// VM is booted like the ensure does: interrupted_start_test.go).
	reg.SetVMManager(&mockVMManager{resumeError: errors.New("missing managed instance"),
		getVMs: map[string]*VMInstanceInfo{UniversalWirePlatformVMID: {State: "stopped", Epoch: 1146}}})
	key := ownershipKey(UniversalWirePlatformOwnerID, UniversalWirePlatformDesktopID)
	reg.mu.Lock()
	reg.ownerships[key] = &VMOwnership{
		VMID:          UniversalWirePlatformVMID,
		UserID:        UniversalWirePlatformOwnerID,
		DesktopID:     UniversalWirePlatformDesktopID,
		ComputerID:    stableComputerID(UniversalWirePlatformOwnerID, UniversalWirePlatformDesktopID, ""),
		Kind:          VMKindInteractive,
		WarmnessClass: WarmnessClassPublicPlatform,
		State:         VMStateStopped,
		Epoch:         1146,
	}
	reg.vmByID[UniversalWirePlatformVMID] = reg.ownerships[key]
	reg.saveLocked()
	reg.mu.Unlock()

	allowRoute := func(context.Context, string, string) error { return nil }
	if warmed := reg.WarmUniversalWirePlatformComputer(t.Context(), allowRoute); warmed != 0 {
		t.Fatalf("warmed = %d, want 0 after resume failure", warmed)
	}
	own := reg.GetOwnershipForDesktop(UniversalWirePlatformOwnerID, UniversalWirePlatformDesktopID)
	if own == nil || own.State != VMStateFailed || own.StoppedBy != "recovery_failed" {
		t.Fatalf("ownership after resume failure = %#v, want durable recovery_failed state", own)
	}
	byVMID := reg.GetOwnershipByVMID(UniversalWirePlatformVMID)
	if byVMID == nil || byVMID.State != VMStateFailed || byVMID.StoppedBy != "recovery_failed" ||
		byVMID.ComputerID != own.ComputerID || byVMID.UserID != own.UserID || byVMID.DesktopID != own.DesktopID {
		t.Fatalf("VMID ownership after resume failure = %#v, want coherent retained failed ownership %#v", byVMID, own)
	}

	restarted := NewOwnershipRegistry("http://127.0.0.1:8085")
	if err := restarted.SetPersistencePath(path); err != nil {
		t.Fatalf("restart SetPersistencePath: %v", err)
	}
	loaded := restarted.GetOwnershipForDesktop(UniversalWirePlatformOwnerID, UniversalWirePlatformDesktopID)
	if loaded == nil || loaded.State != VMStateFailed || loaded.StoppedBy != "recovery_failed" {
		t.Fatalf("persisted ownership after resume failure = %#v, want durable recovery_failed state", loaded)
	}
}

func TestEnsureUniversalWirePlatformComputerBootsStableVM(t *testing.T) {
	mgr := &mockVMManager{
		bootResponse: &VMInstanceInfo{
			HostURL: "http://10.203.140.2:8085",
			Epoch:   3,
			Healthy: true,
			State:   "running",
		},
	}
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetVMManager(mgr)

	err := reg.EnsureUniversalWirePlatformComputer(t.Context())
	if err != nil {
		t.Fatalf("EnsureUniversalWirePlatformComputer: %v", err)
	}
	if len(mgr.boots) != 1 {
		t.Fatalf("expected one boot, got %d", len(mgr.boots))
	}
	if mgr.boots[0].VMID != UniversalWirePlatformVMID {
		t.Fatalf("boot VMID = %q, want %q", mgr.boots[0].VMID, UniversalWirePlatformVMID)
	}
	if mgr.boots[0].ComputerKind != "platform" {
		t.Fatalf("ComputerKind = %q, want platform", mgr.boots[0].ComputerKind)
	}
	key := ownershipKey(UniversalWirePlatformOwnerID, UniversalWirePlatformDesktopID)
	own := reg.ownerships[key]
	if own == nil || own.WarmnessClass != WarmnessClassPublicPlatform {
		t.Fatalf("expected public_platform ownership, got %#v", own)
	}
}

func TestEnsureUniversalWirePlatformComputerHeldSkipsBoot(t *testing.T) {
	var logs bytes.Buffer
	previousLogWriter := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(previousLogWriter)

	mgr := &mockVMManager{}
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetVMManager(mgr)
	key := ownershipKey(UniversalWirePlatformOwnerID, UniversalWirePlatformDesktopID)
	reg.ownerships[key] = &VMOwnership{
		VMID:          UniversalWirePlatformVMID,
		ComputerID:    UniversalWirePlatformComputerID,
		UserID:        UniversalWirePlatformOwnerID,
		DesktopID:     UniversalWirePlatformDesktopID,
		Kind:          VMKindInteractive,
		WarmnessClass: WarmnessClassPublicPlatform,
		State:         VMStateStopped,
		StoppedBy:     "recovery_failed",
		HoldStatus:    &MaintenanceHold{Reason: "maintenance", HeldBy: "test"},
	}
	reg.vmByID[UniversalWirePlatformVMID] = reg.ownerships[key]

	if err := reg.EnsureUniversalWirePlatformComputer(t.Context()); err != nil {
		t.Fatalf("EnsureUniversalWirePlatformComputer held: %v", err)
	}
	if warmed := reg.WarmUniversalWirePlatformComputer(t.Context(), func(context.Context, string, string) error { return nil }); warmed != 0 {
		t.Fatalf("held platform warm result = %d, want 0", warmed)
	}
	if len(mgr.boots) != 0 || len(mgr.recovers) != 0 || len(mgr.resumes) != 0 {
		t.Fatalf("held platform computer must not launch: boots=%v recovers=%v resumes=%v", mgr.boots, mgr.recovers, mgr.resumes)
	}
	own := reg.GetOwnershipForDesktop(UniversalWirePlatformOwnerID, UniversalWirePlatformDesktopID)
	if own == nil || !own.IsHeld() || own.State != VMStateStopped || own.StoppedBy != "recovery_failed" {
		t.Fatalf("held platform ownership changed: %#v", own)
	}
	if !strings.Contains(logs.String(), "platform computer vm="+UniversalWirePlatformVMID+" refused: held") {
		t.Fatalf("held platform refusal was not logged: %q", logs.String())
	}
}

func TestEnsureUniversalWirePlatformComputerRecoversPersistedBootingWithoutWaiter(t *testing.T) {
	mgr := &mockVMManager{
		resumeError: errors.New("pending vm cannot resume"),
		getVMs: map[string]*VMInstanceInfo{
			UniversalWirePlatformVMID: {
				HostURL: "http://10.200.17.2:8085",
				Epoch:   58,
				Healthy: false,
				// failed: the process is gone, so recovery destroys nothing
				// alive. A live pending boot is never recovered.
				State: "failed",
			},
		},
		// A dead tracked instance starts on the current deploy (RefreshVM).
		refreshResponse: &VMInstanceInfo{
			HostURL: "http://10.200.99.2:8085",
			Epoch:   59,
			Healthy: true,
			State:   "running",
		},
	}
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetVMManager(mgr)
	key := ownershipKey(UniversalWirePlatformOwnerID, UniversalWirePlatformDesktopID)
	reg.ownerships[key] = &VMOwnership{VMID: UniversalWirePlatformVMID,
		UserID:        UniversalWirePlatformOwnerID,
		DesktopID:     UniversalWirePlatformDesktopID,
		Kind:          VMKindInteractive,
		WarmnessClass: WarmnessClassPublicPlatform, ComputerURL: "http://10.200.17.2:8085",
		State: VMStateBooting,
		Epoch: 58}
	reg.vmByID[UniversalWirePlatformVMID] = reg.ownerships[key]

	err := reg.EnsureUniversalWirePlatformComputer(t.Context())
	if err != nil {
		t.Fatalf("EnsureUniversalWirePlatformComputer: %v", err)
	}
	if len(mgr.refreshes) != 1 || mgr.refreshes[0] != UniversalWirePlatformVMID || len(mgr.recovers) != 0 {
		t.Fatalf("expected a fresh realization of %s on the current deploy, got refreshes %#v recovers %#v", UniversalWirePlatformVMID, mgr.refreshes, mgr.recovers)
	}
	if len(mgr.refreshCfgs) != 1 || mgr.refreshCfgs[0].ComputerKind != "platform" {
		t.Fatalf("expected platform recovery config, got %#v", mgr.refreshCfgs)
	}
	own := reg.ownerships[key]
	if own.State != VMStateActive {
		t.Fatalf("state = %s, want active", own.State)
	}
	if own.ComputerURL != "http://10.200.99.2:8085" {
		t.Fatalf("autoputer URL = %q, want recovered URL", own.ComputerURL)
	}
	if own.Epoch != 59 {
		t.Fatalf("epoch = %d, want 59", own.Epoch)
	}
}

func TestEnsureUniversalWirePlatformComputerCoalescesPersistedBootingRecovery(t *testing.T) {
	mgr := &blockingPlatformRecoverManager{
		started: make(chan struct{}),
		release: make(chan struct{}),
		info: &VMInstanceInfo{
			HostURL: "http://10.200.17.2:8085",
			Epoch:   58,
			Healthy: false,
			State:   "failed",
		},
		recovered: &VMInstanceInfo{
			HostURL: "http://10.200.99.2:8085",
			Epoch:   59,
			Healthy: true,
			State:   "running",
		},
	}
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetVMManager(mgr)
	key := ownershipKey(UniversalWirePlatformOwnerID, UniversalWirePlatformDesktopID)
	reg.ownerships[key] = &VMOwnership{VMID: UniversalWirePlatformVMID,
		UserID:        UniversalWirePlatformOwnerID,
		DesktopID:     UniversalWirePlatformDesktopID,
		Kind:          VMKindInteractive,
		WarmnessClass: WarmnessClassPublicPlatform, ComputerURL: "http://10.200.17.2:8085",
		State: VMStateBooting,
		Epoch: 58}
	reg.vmByID[UniversalWirePlatformVMID] = reg.ownerships[key]

	errs := make(chan error, 2)
	go func() {
		errs <- reg.EnsureUniversalWirePlatformComputer(t.Context())
	}()

	select {
	case <-mgr.started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first platform recovery to start")
	}

	go func() {
		errs <- reg.EnsureUniversalWirePlatformComputer(t.Context())
	}()
	waitForPlatformWaiter(t, reg, key, 1)

	close(mgr.release)
	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("EnsureUniversalWirePlatformComputer call %d: %v", i+1, err)
		}
	}
	if calls := mgr.recoverCallCount(); calls != 1 {
		t.Fatalf("recover calls = %d, want 1", calls)
	}
	own := reg.ownerships[key]
	if own.State != VMStateActive || own.ComputerURL != "http://10.200.99.2:8085" {
		t.Fatalf("ownership after coalesced recovery = state %s url %q", own.State, own.ComputerURL)
	}
}

type blockingPlatformRecoverManager struct {
	started   chan struct{}
	release   chan struct{}
	info      *VMInstanceInfo
	recovered *VMInstanceInfo

	mu           sync.Mutex
	recoverCalls int
	startedOnce  sync.Once
}

func (m *blockingPlatformRecoverManager) ReserveBootEpoch(_ string, minimum int64) (int64, error) {
	return minimum, nil
}

func (m *blockingPlatformRecoverManager) BootVM(VMManagerConfig) (*VMInstanceInfo, error) {
	return nil, errors.New("boot should not be called")
}

func (m *blockingPlatformRecoverManager) StopVM(string) error {
	return nil
}

func (m *blockingPlatformRecoverManager) HibernateVM(string) error {
	return nil
}

func (m *blockingPlatformRecoverManager) ResumeVM(string) (*VMInstanceInfo, error) {
	return nil, errors.New("resume should not be called")
}

func (m *blockingPlatformRecoverManager) ReattachVM(string, string, int64) (*VMInstanceInfo, error) {
	return nil, errors.New("reattach should not be called")
}

func (m *blockingPlatformRecoverManager) RecoverVM(string, VMManagerConfig) (*VMInstanceInfo, error) {
	m.mu.Lock()
	m.recoverCalls++
	m.mu.Unlock()
	m.startedOnce.Do(func() { close(m.started) })
	<-m.release // hang-guard: fixture release; the test unblocks it
	m.mu.Lock()
	m.info = m.recovered
	m.mu.Unlock()
	return m.recovered, nil
}

// RefreshVM is how a dead tracked instance starts (on the current deploy);
// it is counted with RecoverVM as one fresh realization.
func (m *blockingPlatformRecoverManager) RefreshVM(vmID string, cfg VMManagerConfig) (*VMInstanceInfo, error) {
	return m.RecoverVM(vmID, cfg)
}

func (m *blockingPlatformRecoverManager) DestroyVMState(string) error {
	return nil
}

func (m *blockingPlatformRecoverManager) GetVM(string) *VMInstanceInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.info
}

func (m *blockingPlatformRecoverManager) CheckHealth(string) (bool, error) {
	return false, nil
}

func (m *blockingPlatformRecoverManager) recoverCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.recoverCalls
}

func waitForPlatformWaiter(t *testing.T, reg *OwnershipRegistry, key string, want int) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			reg.mu.RLock()
			got := len(reg.pendingWaiters[key])
			reg.mu.RUnlock()
			t.Fatalf("timed out waiting for %d platform waiters; got %d", want, got)
		case <-ticker.C:
			reg.mu.RLock()
			got := len(reg.pendingWaiters[key])
			reg.mu.RUnlock()
			if got == want {
				return
			}
		}
	}
}
