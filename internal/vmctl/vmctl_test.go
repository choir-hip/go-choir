package vmctl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// --- Ownership Registry Tests ---

func TestOwnershipRegistry_ResolveOrAssignReturnsSnapshot(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	own, err := reg.ResolveOrAssign("user-snapshot")
	if err != nil {
		t.Fatalf("ResolveOrAssign: %v", err)
	}
	own.ComputerURL = "http://caller-mutated"
	own.State = VMStateFailed

	got := reg.GetOwnership("user-snapshot")
	if got == nil {
		t.Fatal("expected registry ownership")
	}
	if got.ComputerURL == "http://caller-mutated" {
		t.Fatal("ResolveOrAssign returned a live ownership pointer")
	}
	if got.State != VMStateActive {
		t.Fatalf("registry state = %s, want active", got.State)
	}
}

func TestOwnershipRegistry_DifferentUsersGetDifferentVMs(t *testing.T) {
	// VAL-VM-005: Different users receive distinct VMs and isolated state.
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	own1, _ := reg.ResolveOrAssign("user-alice")
	own2, _ := reg.ResolveOrAssign("user-bob")

	if own1.VMID == own2.VMID {
		t.Error("expected different VM IDs for different users")
	}
	if own1.UserID == own2.UserID {
		t.Error("expected different user IDs")
	}
}

func TestOwnershipRegistry_SameUserDifferentDesktopsGetDifferentVMs(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	primary, err := reg.ResolveOrAssignDesktop("user-1", PrimaryDesktopID)
	if err != nil {
		t.Fatalf("ResolveOrAssignDesktop primary: %v", err)
	}
	branch, err := reg.ResolveOrAssignDesktop("user-1", "branch-a")
	if err != nil {
		t.Fatalf("ResolveOrAssignDesktop branch: %v", err)
	}

	if primary.VMID == branch.VMID {
		t.Fatalf("expected different VM IDs for different desktops, got %s", primary.VMID)
	}
	if primary.DesktopID != PrimaryDesktopID {
		t.Errorf("primary DesktopID = %q, want %q", primary.DesktopID, PrimaryDesktopID)
	}
	if branch.DesktopID != "branch-a" {
		t.Errorf("branch DesktopID = %q, want %q", branch.DesktopID, "branch-a")
	}
}

func TestOwnershipRegistry_ConcurrentRequestsCollapseToOneVM(t *testing.T) {
	// VAL-VM-004: Concurrent first requests for one user collapse onto one
	// VM assignment.
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	const concurrency = 20
	results := make(chan *VMOwnership, concurrency)
	errors := make(chan error, concurrency)

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			own, err := reg.ResolveOrAssign("user-concurrent")
			if err != nil {
				errors <- err
				return
			}
			results <- own
		}()
	}
	wg.Wait()
	close(results)
	close(errors)

	for err := range errors {
		t.Errorf("concurrent ResolveOrAssign: %v", err)
	}

	var vmIDs []string
	for own := range results {
		vmIDs = append(vmIDs, own.VMID)
	}

	if len(vmIDs) != concurrency {
		t.Fatalf("expected %d results, got %d", concurrency, len(vmIDs))
	}

	// All concurrent callers should receive the same VM ID.
	first := vmIDs[0]
	for _, id := range vmIDs[1:] {
		if id != first {
			t.Errorf("expected all concurrent callers to get VM %s, got %s", first, id)
		}
	}
}

type blockingBootVMManager struct {
	mu         sync.Mutex
	boots      []VMManagerConfig
	runningVMs map[string]*VMInstanceInfo
	started    chan struct{}
	release    chan struct{}
	hostURL    string
	startOnce  sync.Once
}

func newBlockingBootVMManager(hostURL string) *blockingBootVMManager {
	return &blockingBootVMManager{
		runningVMs: make(map[string]*VMInstanceInfo),
		started:    make(chan struct{}),
		release:    make(chan struct{}),
		hostURL:    hostURL,
	}
}

func (m *blockingBootVMManager) ReserveBootEpoch(_ string, minimum int64) (int64, error) {
	return minimum, nil
}

func (m *blockingBootVMManager) BootVM(cfg VMManagerConfig) (*VMInstanceInfo, error) {
	m.mu.Lock()
	m.boots = append(m.boots, cfg)
	m.mu.Unlock()
	m.startOnce.Do(func() { close(m.started) })
	<-m.release // hang-guard: fixture release; the test unblocks it
	return &VMInstanceInfo{HostURL: m.hostURL, Epoch: 1, Healthy: true, State: "running"}, nil
}

func (m *blockingBootVMManager) StopVM(vmID string) error      { return nil }
func (m *blockingBootVMManager) HibernateVM(vmID string) error { return nil }
func (m *blockingBootVMManager) ResumeVM(vmID string) (*VMInstanceInfo, error) {
	return &VMInstanceInfo{HostURL: m.hostURL, Epoch: 1, Healthy: true, State: "running"}, nil
}
func (m *blockingBootVMManager) ReattachVM(vmID, hostURL string, epoch int64) (*VMInstanceInfo, error) {
	return &VMInstanceInfo{HostURL: hostURL, Epoch: epoch, Healthy: true, State: "running"}, nil
}
func (m *blockingBootVMManager) RecoverVM(vmID string, cfg VMManagerConfig) (*VMInstanceInfo, error) {
	return &VMInstanceInfo{HostURL: m.hostURL, Epoch: 2, Healthy: true, State: "running"}, nil
}
func (m *blockingBootVMManager) RefreshVM(vmID string, cfg VMManagerConfig) (*VMInstanceInfo, error) {
	return &VMInstanceInfo{HostURL: m.hostURL, Epoch: 3, Healthy: true, State: "running"}, nil
}
func (m *blockingBootVMManager) DestroyVMState(vmID string) error { return nil }
func (m *blockingBootVMManager) GetVM(vmID string) *VMInstanceInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.runningVMs[vmID]
}
func (m *blockingBootVMManager) CheckHealth(vmID string) (bool, error) { return true, nil }

func TestOwnershipRegistry_BootingRequestsWaitForReadyVM(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	manager := newBlockingBootVMManager("http://127.0.0.1:9009")
	reg.SetVMManager(manager)

	firstDone := make(chan struct{})
	var firstOwn *VMOwnership
	var firstErr error
	go func() {
		firstOwn, firstErr = reg.ResolveOrAssign("user-booting")
		close(firstDone)
	}()

	select {
	case <-manager.started:
	case <-time.After(time.Second):
		t.Fatal("expected first resolve to start VM boot")
	}

	secondDone := make(chan struct{})
	var secondOwn *VMOwnership
	var secondErr error
	go func() {
		secondOwn, secondErr = reg.ResolveOrAssign("user-booting")
		close(secondDone)
	}()

	select {
	case <-secondDone:
		t.Fatal("second resolve returned before the VM finished booting")
	case <-time.After(150 * time.Millisecond):
	}

	close(manager.release)

	select {
	case <-firstDone:
	case <-time.After(time.Second):
		t.Fatal("first resolve did not finish after boot release")
	}
	select {
	case <-secondDone:
	case <-time.After(time.Second):
		t.Fatal("second resolve did not finish after boot release")
	}

	if firstErr != nil {
		t.Fatalf("first resolve: %v", firstErr)
	}
	if secondErr != nil {
		t.Fatalf("second resolve: %v", secondErr)
	}
	if firstOwn.VMID != secondOwn.VMID {
		t.Fatalf("expected both resolves to share one VM, got %s and %s", firstOwn.VMID, secondOwn.VMID)
	}
	if firstOwn.ComputerURL != "http://127.0.0.1:9009" {
		t.Fatalf("first resolve autoputer URL = %q, want VM URL", firstOwn.ComputerURL)
	}
	if secondOwn.ComputerURL != "http://127.0.0.1:9009" {
		t.Fatalf("second resolve autoputer URL = %q, want VM URL", secondOwn.ComputerURL)
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if len(manager.boots) != 1 {
		t.Fatalf("expected exactly one VM boot, got %d", len(manager.boots))
	}
}

func TestOwnershipRegistry_BootingWaitRespectsContextCancellation(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	manager := newBlockingBootVMManager("http://127.0.0.1:9009")
	reg.SetVMManager(manager)

	firstDone := make(chan struct{})
	go func() {
		_, _ = reg.ResolveOrAssign("user-cancel-wait")
		close(firstDone)
	}()

	select {
	case <-manager.started:
	case <-time.After(time.Second):
		t.Fatal("expected first resolve to start VM boot")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	own, err := reg.ResolveOrAssignDesktopContext(ctx, "user-cancel-wait", PrimaryDesktopID)
	if err == nil {
		t.Fatalf("expected canceled waiter error, got ownership %+v", own)
	}
	if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("expected context deadline error, got %v", err)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("canceled waiter returned too slowly: %s", elapsed)
	}

	select {
	case <-firstDone:
		t.Fatal("first resolve should keep booting for the next retry")
	default:
	}

	close(manager.release)
	select {
	case <-firstDone:
	case <-time.After(time.Second):
		t.Fatal("first resolve did not finish after boot release")
	}

	retryOwn, err := reg.ResolveOrAssign("user-cancel-wait")
	if err != nil {
		t.Fatalf("retry resolve: %v", err)
	}
	if retryOwn.ComputerURL != "http://127.0.0.1:9009" {
		t.Fatalf("retry autoputer URL = %q, want VM URL", retryOwn.ComputerURL)
	}
}

func TestOwnershipRegistry_StoppedVMGetsResumed(t *testing.T) {
	// When a user's VM is stopped, a new ResolveOrAssign should resume it
	// with the same VMID, preserving user state (VAL-CROSS-116).
	// The epoch stays the same on resume (VAL-CROSS-117).
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	own1, _ := reg.ResolveOrAssign("user-1")
	oldVMID := own1.VMID
	oldEpoch := own1.Epoch

	if err := reg.StopVM("user-1"); err != nil {
		t.Fatalf("StopVM: %v", err)
	}

	own2, _ := reg.ResolveOrAssign("user-1")
	if own2.VMID != oldVMID {
		t.Errorf("expected same VM ID after stop+resolve (resume), got %s vs %s", oldVMID, own2.VMID)
	}
	if own2.Epoch != oldEpoch {
		t.Errorf("expected same epoch after resume, got %d vs %d", oldEpoch, own2.Epoch)
	}
	if own2.State != VMStateActive {
		t.Errorf("expected active state after resume, got %s", own2.State)
	}
}

// --- Handler Tests ---

func newTestServer(t *testing.T) (*httptest.Server, *OwnershipRegistry) {
	t.Helper()
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	handler := NewHandler(reg)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handler.HandleHealth)
	mux.HandleFunc("/internal/vmctl/resolve", handler.HandleResolve)
	mux.HandleFunc("/internal/vmctl/lookup", handler.HandleLookup)
	mux.HandleFunc("/internal/vmctl/stop", handler.HandleStop)
	mux.HandleFunc("/internal/vmctl/remove", handler.HandleRemove)
	mux.HandleFunc("/internal/vmctl/list", handler.HandleList)
	mux.HandleFunc("/internal/vmctl/hibernate", handler.HandleHibernate)
	mux.HandleFunc("/internal/vmctl/resume", handler.HandleResume)
	mux.HandleFunc("/internal/vmctl/recover", handler.HandleRecover)
	mux.HandleFunc("/internal/vmctl/logout", handler.HandleLogout)
	mux.HandleFunc("/internal/vmctl/idle-check", handler.HandleIdleCheck)
	mux.HandleFunc("/internal/vmctl/runtime-package/autoputer", handler.HandleRuntimePackage)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, reg
}

func TestHandler_RuntimePackageRejectsMissingAutoputerBuildManifest(t *testing.T) {
	handler := NewHandler(NewOwnershipRegistry("http://127.0.0.1:8085"))
	pkgDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(pkgDir, "bin"), 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "bin", "autoputer"), []byte("autoputer-binary"), 0o755); err != nil {
		t.Fatalf("write autoputer: %v", err)
	}
	handler.SetAutoputerRuntimePackageDir(pkgDir)

	req := httptest.NewRequest(http.MethodGet, "/internal/vmctl/runtime-package/autoputer", nil)
	req.Header.Set("X-Internal-Caller", "true")
	rr := httptest.NewRecorder()
	handler.HandleRuntimePackage(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusServiceUnavailable, rr.Body.String())
	}
}

func TestHandler_RuntimePackageDeniesExternalCaller(t *testing.T) {
	handler := NewHandler(NewOwnershipRegistry("http://127.0.0.1:8085"))
	handler.SetAutoputerRuntimePackageDir(t.TempDir())

	req := httptest.NewRequest(http.MethodGet, "/internal/vmctl/runtime-package/autoputer", nil)
	req.Host = "choir.news"
	req.RemoteAddr = "203.0.113.10:4444"
	rr := httptest.NewRecorder()
	handler.HandleRuntimePackage(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHandler_ResolveConcurrentUsersDoNotBlockOnSlowBoot(t *testing.T) {
	srv, reg := newTestServer(t)
	manager := newBlockingBootVMManager("http://127.0.0.1:9009")
	manager.runningVMs["vm-ready-1"] = &VMInstanceInfo{
		HostURL: "http://127.0.0.1:9010",
		Epoch:   1,
		Healthy: true,
		State:   "running",
	}
	reg.SetVMManager(manager)
	// Pre-populate an active ready ownership for user-ready.
	ownReady := &VMOwnership{
		VMID:         "vm-ready-1",
		UserID:       "user-ready",
		DesktopID:    PrimaryDesktopID,
		Kind:         VMKindInteractive,
		ComputerURL:  "http://127.0.0.1:9010",
		State:        VMStateActive,
		LastActiveAt: time.Now(),
		Epoch:        1,
	}
	reg.mu.Lock()
	reg.ownerships[ownershipKey("user-ready", PrimaryDesktopID)] = ownReady
	reg.vmByID["vm-ready-1"] = ownReady
	reg.mu.Unlock()

	// Start a resolve request for user-slow that blocks in BootVM.
	slowDone := make(chan int, 1)
	go func() {
		reqBody := strings.NewReader(`{"user_id":"user-slow","desktop_id":"primary"}`)
		req, err := http.NewRequest(http.MethodPost, srv.URL+"/internal/vmctl/resolve", reqBody)
		if err != nil {
			slowDone <- 0
			return
		}
		req.Header.Set("X-Internal-Caller", "true")
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			slowDone <- 0
			return
		}
		defer resp.Body.Close()
		slowDone <- resp.StatusCode
	}()

	// Wait until user-slow enters BootVM.
	select {
	case <-manager.started:
	case <-time.After(2 * time.Second):
		t.Fatal("expected user-slow resolve to start BootVM")
	}

	// Now, while user-slow is blocked in BootVM, user-ready resolves.
	// This must complete immediately and NOT block on user-slow's boot.
	readyReqBody := strings.NewReader(`{"user_id":"user-ready","desktop_id":"primary"}`)
	readyReq, err := http.NewRequest(http.MethodPost, srv.URL+"/internal/vmctl/resolve", readyReqBody)
	if err != nil {
		t.Fatalf("create ready request: %v", err)
	}
	readyReq.Header.Set("X-Internal-Caller", "true")
	readyReq.Header.Set("Content-Type", "application/json")

	readyStart := time.Now()
	readyResp, err := http.DefaultClient.Do(readyReq)
	readyElapsed := time.Since(readyStart)
	if err != nil {
		t.Fatalf("user-ready resolve failed: %v", err)
	}
	defer readyResp.Body.Close()

	if readyResp.StatusCode != http.StatusOK {
		t.Fatalf("user-ready resolve status = %d, want %d", readyResp.StatusCode, http.StatusOK)
	}
	if readyElapsed > 500*time.Millisecond {
		t.Fatalf("user-ready resolve took %s; expected immediate non-blocking resolution (<500ms)", readyElapsed)
	}

	// Release user-slow's boot.
	close(manager.release)
	select {
	case status := <-slowDone:
		if status != http.StatusOK {
			t.Fatalf("user-slow resolve status = %d, want %d", status, http.StatusOK)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("user-slow resolve did not complete after release")
	}
}
func TestHandler_ReserveFreshVMConfigDoesNotDeadlockDuringSlowBoot(t *testing.T) {
	srv, reg := newTestServer(t)
	manager := newBlockingBootVMManager("http://127.0.0.1:9009")
	reg.SetVMManager(manager)

	// Pre-populate stopped ownership for slow user.
	ownSlow := &VMOwnership{
		VMID:         "vm-slow-1",
		UserID:       "user-slow",
		DesktopID:    PrimaryDesktopID,
		Kind:         VMKindInteractive,
		ComputerURL:  "http://127.0.0.1:9009",
		State:        VMStateStopped,
		LastActiveAt: time.Now(),
		Epoch:        1,
	}
	ownReady := &VMOwnership{
		VMID:         "vm-ready-1",
		UserID:       "user-ready",
		DesktopID:    PrimaryDesktopID,
		Kind:         VMKindInteractive,
		ComputerURL:  "http://127.0.0.1:9010",
		State:        VMStateActive,
		LastActiveAt: time.Now(),
		Epoch:        1,
	}
	reg.mu.Lock()
	reg.ownerships[ownershipKey("user-slow", PrimaryDesktopID)] = ownSlow
	reg.vmByID["vm-slow-1"] = ownSlow
	reg.ownerships[ownershipKey("user-ready", PrimaryDesktopID)] = ownReady
	reg.vmByID["vm-ready-1"] = ownReady
	reg.mu.Unlock()

	// Start a resume for user-slow that enters BootVM and blocks.
	slowDone := make(chan error, 1)
	go func() {
		_, err := reg.ResumeVMForDesktop("user-slow", PrimaryDesktopID)
		slowDone <- err
	}()

	// Wait until BootVM is reached.
	select {
	case <-manager.started:
	case <-time.After(2 * time.Second):
		t.Fatal("user-slow boot did not start within 2s")
	}

	// Concurrently call GET /health which requires r.mu.RLock.
	healthReq, err := http.NewRequest(http.MethodGet, srv.URL+"/health", nil)
	if err != nil {
		t.Fatal(err)
	}
	healthStart := time.Now()
	healthResp, err := http.DefaultClient.Do(healthReq)
	healthElapsed := time.Since(healthStart)
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	defer healthResp.Body.Close()
	if healthResp.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d, want 200", healthResp.StatusCode)
	}
	if healthElapsed > 500*time.Millisecond {
		t.Fatalf("health check took %s; expected non-blocking execution (<500ms)", healthElapsed)
	}

	// Release slow boot.
	close(manager.release)
	select {
	case err := <-slowDone:
		if err != nil {
			t.Fatalf("slow resume returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("slow resume did not complete after release")
	}
}

func TestOwnershipRegistry_StateDirPressureTriggersReclaimPlan(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetPressureReclaimConfig(PressureReclaimConfig{
		Mode:                      PressureReclaimModeDryRun,
		MinIdle:                   10 * time.Minute,
		MinStateDirAvailableBytes: 10 * 1024 * 1024 * 1024,
		MaxCandidates:             5,
	})
	reg.setPressureSamplerForTest(func(cfg PressureReclaimConfig) HostPressureSample {
		return HostPressureSample{
			SampledAt:                "2026-05-20T12:00:00Z",
			StateDirAvailableBytes:   512 * 1024 * 1024,
			StateDirAvailablePercent: 2,
		}
	})
	if _, err := reg.ResolveOrAssign("storage-pressure-user"); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	reg.mu.Lock()
	reg.ownerships[ownershipKey("storage-pressure-user", PrimaryDesktopID)].LastActiveAt = time.Now().Add(-2 * time.Hour)
	reg.mu.Unlock()

	plan := reg.PressureReclaimPlan()
	if !plan.Pressure.Pressure || !plan.Pressure.StateDirPressure {
		t.Fatalf("expected state-dir pressure in plan: %+v", plan.Pressure)
	}
	if plan.Decision != "would_reclaim" {
		t.Fatalf("decision = %s, want would_reclaim", plan.Decision)
	}
}

func TestOwnershipRegistry_RetentionShadowPlanDoesNotExpandActivePrune(t *testing.T) {
	stateDir := t.TempDir()
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetRetentionPruneConfig(RetentionPruneConfig{
		Mode:                  RetentionPruneModeActive,
		StateDir:              stateDir,
		EphemeralEmailDomains: []string{"example.com"},
		EphemeralMinAge:       time.Hour,
		MaxDeletes:            10,
		MaxBytes:              1024 * 1024 * 1024,
	})
	reg.SetRetentionShadowPruneConfig(RetentionPruneConfig{
		Mode:                    RetentionPruneModeActive,
		StateDir:                stateDir,
		EphemeralEmailDomains:   []string{"example.com", "example.test"},
		EphemeralUserIDPrefixes: []string{"diagnostic-"},
		EphemeralMinAge:         time.Hour,
		MaxDeletes:              10,
		MaxBytes:                1024 * 1024 * 1024,
	})
	reg.setRetentionUserEmailsForTest(map[string]string{
		"active-policy-user": "playwright@example.com",
		"shadow-email-user":  "load@example.test",
		"real-user":          "yusefnathanson@me.com",
	})

	activeOwn, err := reg.ResolveOrAssign("active-policy-user")
	if err != nil {
		t.Fatalf("resolve active policy user: %v", err)
	}
	shadowEmailOwn, err := reg.ResolveOrAssign("shadow-email-user")
	if err != nil {
		t.Fatalf("resolve shadow email user: %v", err)
	}
	shadowSyntheticOwn, err := reg.ResolveOrAssign("diagnostic-1778792614")
	if err != nil {
		t.Fatalf("resolve shadow synthetic user: %v", err)
	}
	realOwn, err := reg.ResolveOrAssign("real-user")
	if err != nil {
		t.Fatalf("resolve real user: %v", err)
	}

	old := time.Now().Add(-3 * time.Hour)
	reg.mu.Lock()
	for _, userID := range []string{"active-policy-user", "shadow-email-user", "diagnostic-1778792614", "real-user"} {
		own := reg.ownerships[ownershipKey(userID, PrimaryDesktopID)]
		own.State = VMStateHibernated
		own.LastActiveAt = old
	}
	reg.mu.Unlock()

	for _, vmID := range []string{activeOwn.VMID, shadowEmailOwn.VMID, shadowSyntheticOwn.VMID, realOwn.VMID} {
		dir := filepath.Join(stateDir, vmID)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", vmID, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "data"), []byte("state"), 0o600); err != nil {
			t.Fatalf("write %s: %v", vmID, err)
		}
	}

	activePlan := reg.RetentionPrunePlan()
	if activePlan.Mode != RetentionPruneModeActive {
		t.Fatalf("active plan mode = %s, want active", activePlan.Mode)
	}
	if !retentionPlanHasVM(activePlan, activeOwn.VMID) {
		t.Fatalf("active plan missing active-policy VM %s: %+v", activeOwn.VMID, activePlan.Candidates)
	}
	if retentionPlanHasVM(activePlan, shadowEmailOwn.VMID) {
		t.Fatalf("active plan must not include shadow example.test VM %s: %+v", shadowEmailOwn.VMID, activePlan.Candidates)
	}
	if retentionPlanHasVM(activePlan, shadowSyntheticOwn.VMID) {
		t.Fatalf("active plan must not include shadow synthetic VM %s: %+v", shadowSyntheticOwn.VMID, activePlan.Candidates)
	}
	if retentionPlanHasVM(activePlan, realOwn.VMID) {
		t.Fatalf("active plan must not include protected real-user VM %s: %+v", realOwn.VMID, activePlan.Candidates)
	}

	shadowPlan := reg.RetentionShadowPlan()
	if shadowPlan.Mode != RetentionPruneModeDryRun {
		t.Fatalf("shadow plan mode = %s, want dry-run", shadowPlan.Mode)
	}
	if shadowPlan.Decision != "would_prune" {
		t.Fatalf("shadow plan decision = %s, want would_prune: %+v", shadowPlan.Decision, shadowPlan)
	}
	for _, vmID := range []string{activeOwn.VMID, shadowEmailOwn.VMID, shadowSyntheticOwn.VMID} {
		if !retentionPlanHasVM(shadowPlan, vmID) {
			t.Fatalf("shadow plan missing VM %s: %+v", vmID, shadowPlan.Candidates)
		}
	}
	if retentionPlanHasVM(shadowPlan, realOwn.VMID) {
		t.Fatalf("shadow plan must not include protected real-user VM %s: %+v", realOwn.VMID, shadowPlan.Candidates)
	}

	mgr := &mockVMManager{}
	reg.SetVMManager(mgr)
	denyRoute := func(context.Context, string, string) error { return errors.New("route missing") }
	if denied := reg.PruneRetention(context.Background(), denyRoute); denied.Deleted != 0 {
		t.Fatalf("route-refused retention deleted = %d, want 0", denied.Deleted)
	}
	result := reg.PruneRetention(context.Background(), allowLifecycleRoute)
	if result.Deleted != 1 {
		t.Fatalf("deleted = %d, want 1: %+v", result.Deleted, result)
	}
	if !containsString(mgr.destroys, activeOwn.VMID) {
		t.Fatalf("destroyed VMs = %v, want active policy VM %s", mgr.destroys, activeOwn.VMID)
	}
	for _, vmID := range []string{shadowEmailOwn.VMID, shadowSyntheticOwn.VMID, realOwn.VMID} {
		if containsString(mgr.destroys, vmID) {
			t.Fatalf("active prune must not destroy shadow/protected VM %s: %v", vmID, mgr.destroys)
		}
	}
}

func TestOwnershipRegistry_PruneRetentionRemovesEphemeralPrimaryOwnership(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetRetentionPruneConfig(RetentionPruneConfig{
		Mode:                  RetentionPruneModeActive,
		StateDir:              t.TempDir(),
		EphemeralEmailDomains: []string{"example.com"},
		EphemeralMinAge:       time.Hour,
		MaxDeletes:            10,
		MaxBytes:              1024 * 1024 * 1024,
	})
	reg.setRetentionUserEmailsForTest(map[string]string{
		"test-user": "playwright@example.com",
		"real-user": "owner@choir-ip.com",
	})
	testOwn, err := reg.ResolveOrAssign("test-user")
	if err != nil {
		t.Fatalf("resolve test user: %v", err)
	}
	realOwn, err := reg.ResolveOrAssign("real-user")
	if err != nil {
		t.Fatalf("resolve real user: %v", err)
	}
	old := time.Now().Add(-3 * time.Hour)
	reg.mu.Lock()
	reg.ownerships[ownershipKey("test-user", PrimaryDesktopID)].State = VMStateHibernated
	reg.ownerships[ownershipKey("test-user", PrimaryDesktopID)].LastActiveAt = old
	reg.ownerships[ownershipKey("real-user", PrimaryDesktopID)].State = VMStateHibernated
	reg.ownerships[ownershipKey("real-user", PrimaryDesktopID)].LastActiveAt = old
	reg.mu.Unlock()
	mgr := &mockVMManager{}
	reg.SetVMManager(mgr)

	result := reg.PruneRetention(context.Background(), allowLifecycleRoute)
	if result.Deleted != 1 {
		t.Fatalf("deleted = %d, want 1: %+v", result.Deleted, result)
	}
	if !containsString(mgr.destroys, testOwn.VMID) {
		t.Fatalf("destroyed VMs = %v, want %s", mgr.destroys, testOwn.VMID)
	}
	if reg.GetOwnership("test-user") != nil {
		t.Fatalf("ephemeral test primary ownership should be removed")
	}
	if reg.GetOwnership("real-user") == nil {
		t.Fatalf("real user primary ownership should remain")
	}
	if containsString(mgr.destroys, realOwn.VMID) {
		t.Fatalf("real user VM %s must not be destroyed: %v", realOwn.VMID, mgr.destroys)
	}
}

func TestOwnershipRegistry_RetentionPlanPrefersLargeSafeCandidates(t *testing.T) {
	stateDir := t.TempDir()
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetRetentionPruneConfig(RetentionPruneConfig{
		Mode:                  RetentionPruneModeDryRun,
		StateDir:              stateDir,
		EphemeralEmailDomains: []string{"example.com"},
		OrphanMinAge:          time.Hour,
		EphemeralMinAge:       time.Hour,
		MaxDeletes:            1,
		MaxBytes:              1024 * 1024 * 1024,
	})
	reg.setRetentionUserEmailsForTest(map[string]string{
		"test-user": "playwright@example.com",
	})
	testOwn, err := reg.ResolveOrAssign("test-user")
	if err != nil {
		t.Fatalf("resolve test user: %v", err)
	}
	old := time.Now().Add(-3 * time.Hour)
	reg.mu.Lock()
	reg.ownerships[ownershipKey("test-user", PrimaryDesktopID)].State = VMStateHibernated
	reg.ownerships[ownershipKey("test-user", PrimaryDesktopID)].LastActiveAt = old
	reg.mu.Unlock()

	smallOrphan := filepath.Join(stateDir, "vm-orphan-small")
	largeEphemeral := filepath.Join(stateDir, testOwn.VMID)
	for _, dir := range []string{smallOrphan, largeEphemeral} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(smallOrphan, "data"), []byte("small"), 0o600); err != nil {
		t.Fatalf("write small orphan: %v", err)
	}
	if err := os.WriteFile(filepath.Join(largeEphemeral, "data"), bytes.Repeat([]byte("x"), 128*1024), 0o600); err != nil {
		t.Fatalf("write large ephemeral: %v", err)
	}
	if err := os.Chtimes(smallOrphan, old, old); err != nil {
		t.Fatalf("chtimes small orphan: %v", err)
	}

	plan := reg.RetentionPrunePlan()
	if len(plan.Candidates) != 1 {
		t.Fatalf("limited candidates = %d, want 1: %+v", len(plan.Candidates), plan)
	}
	if plan.Candidates[0].VMID != testOwn.VMID {
		t.Fatalf("first candidate = %+v, want large ephemeral %s", plan.Candidates[0], testOwn.VMID)
	}
	if plan.Inventory.ProjectedDeleteBytes <= 8192 {
		t.Fatalf("projected bytes = %d, want meaningful large candidate", plan.Inventory.ProjectedDeleteBytes)
	}
}

func retentionPlanHasVM(plan RetentionPrunePlan, vmID string) bool {
	for _, candidate := range plan.Candidates {
		if candidate.VMID == vmID {
			return true
		}
	}
	return false
}

func TestOwnershipRegistry_PressureReclaimNoPressureObservesOnly(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetPressureReclaimConfig(PressureReclaimConfig{
		Mode:                      PressureReclaimModeDryRun,
		MinIdle:                   10 * time.Minute,
		MinMemoryAvailableBytes:   1024,
		MinMemoryAvailablePercent: 10,
		MaxMemorySomeAvg10:        1,
		MaxIOSomeAvg10:            5,
	})
	reg.setPressureSamplerForTest(func(cfg PressureReclaimConfig) HostPressureSample {
		return HostPressureSample{
			SampledAt:              "2026-05-14T12:00:00Z",
			MemoryTotalBytes:       8 * 1024 * 1024 * 1024,
			MemoryAvailableBytes:   6 * 1024 * 1024 * 1024,
			MemoryAvailablePercent: 75,
			MemorySomeAvg10:        0,
			IOSomeAvg10:            0,
		}
	})
	if _, err := reg.ResolveOrAssign("idle-user"); err != nil {
		t.Fatalf("resolve idle-user: %v", err)
	}
	reg.mu.Lock()
	reg.ownerships[ownershipKey("idle-user", PrimaryDesktopID)].LastActiveAt = time.Now().Add(-2 * time.Hour)
	reg.mu.Unlock()

	plan := reg.PressureReclaimPlan()
	if plan.Decision != "observe" {
		t.Fatalf("decision = %s, want observe", plan.Decision)
	}
	if plan.Pressure.Pressure {
		t.Fatalf("expected no pressure, got %+v", plan.Pressure)
	}
	if plan.Inventory.Eligible != 1 {
		t.Fatalf("eligible = %d, want 1", plan.Inventory.Eligible)
	}
}

func TestOwnershipRegistry_PrimaryKeepaliveSkipsIdlePrimaryUnderCapacity(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetIdleTimeout(10 * time.Millisecond)
	reg.SetPressureReclaimConfig(PressureReclaimConfig{
		Mode:                      PressureReclaimModeDryRun,
		MinIdle:                   time.Millisecond,
		MinMemoryAvailableBytes:   1024,
		MinMemoryAvailablePercent: 10,
	})
	reg.SetWarmnessPolicyConfig(WarmnessPolicyConfig{PrimaryKeepaliveMode: PrimaryKeepaliveModeUnderCapacity})
	reg.setPressureSamplerForTest(func(cfg PressureReclaimConfig) HostPressureSample {
		return HostPressureSample{
			SampledAt:              "2026-05-14T12:00:00Z",
			MemoryTotalBytes:       8 * 1024 * 1024 * 1024,
			MemoryAvailableBytes:   6 * 1024 * 1024 * 1024,
			MemoryAvailablePercent: 75,
		}
	})

	if _, err := reg.ResolveOrAssign("primary-warm"); err != nil {
		t.Fatalf("resolve primary-warm: %v", err)
	}
	reg.mu.Lock()
	reg.ownerships[ownershipKey("primary-warm", PrimaryDesktopID)].LastActiveAt = time.Now().Add(-time.Hour)
	reg.mu.Unlock()

	idle := reg.CheckIdleOwnerships()
	if len(idle) != 0 {
		t.Fatalf("expected primary keepalive under capacity, got idle candidates: %+v", idle)
	}
	if stopped := reg.StopIdleVMs(context.Background(), allowLifecycleRoute); stopped != 0 {
		t.Fatalf("stopped = %d, want 0", stopped)
	}
	if own := reg.GetOwnership("primary-warm"); own == nil || own.State != VMStateActive {
		t.Fatalf("primary should remain active, got %+v", own)
	}
}

func TestOwnershipRegistry_PremiumAlwaysOnIsModeledAndProtected(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetIdleTimeout(10 * time.Millisecond)
	reg.SetPressureReclaimConfig(PressureReclaimConfig{
		Mode:                      PressureReclaimModeDryRun,
		MinIdle:                   time.Millisecond,
		MinMemoryAvailableBytes:   2 * 1024 * 1024 * 1024,
		MinMemoryAvailablePercent: 15,
	})
	reg.SetWarmnessPolicyConfig(WarmnessPolicyConfig{
		PrimaryKeepaliveMode: PrimaryKeepaliveModeUnderCapacity,
		AlwaysOnUserIDs:      map[string]bool{"premium-user": true},
	})
	reg.setPressureSamplerForTest(func(cfg PressureReclaimConfig) HostPressureSample {
		return HostPressureSample{
			SampledAt:              "2026-05-14T12:00:00Z",
			MemoryTotalBytes:       8 * 1024 * 1024 * 1024,
			MemoryAvailableBytes:   512 * 1024 * 1024,
			MemoryAvailablePercent: 6.25,
		}
	})

	if _, err := reg.ResolveOrAssign("premium-user"); err != nil {
		t.Fatalf("resolve premium-user: %v", err)
	}
	reg.mu.Lock()
	reg.ownerships[ownershipKey("premium-user", PrimaryDesktopID)].LastActiveAt = time.Now().Add(-time.Hour)
	reg.mu.Unlock()

	if idle := reg.CheckIdleOwnerships(); len(idle) != 0 {
		t.Fatalf("premium always-on should not be idle-eligible, got %+v", idle)
	}
	plan := reg.PressureReclaimPlan()
	if plan.Inventory.Protected != 1 || plan.Inventory.Eligible != 0 {
		t.Fatalf("inventory protected/eligible = %d/%d, want 1/0", plan.Inventory.Protected, plan.Inventory.Eligible)
	}
	if len(plan.Candidates) == 0 || plan.Candidates[0].WarmnessClass != string(WarmnessClassPremiumAlwaysOn) {
		t.Fatalf("candidate warmness = %+v, want premium always-on", plan.Candidates)
	}
	if !containsString(plan.Candidates[0].ProtectedReasons, "premium_always_on") {
		t.Fatalf("candidate reasons = %+v, want premium_always_on", plan.Candidates[0].ProtectedReasons)
	}
	summary := reg.WarmnessSummary(nil)
	if summary.Policy.PrimaryKeepaliveMode != PrimaryKeepaliveModeUnderCapacity {
		t.Fatalf("policy mode = %s, want under-capacity", summary.Policy.PrimaryKeepaliveMode)
	}
	if summary.Policy.AlwaysOnUserCount != 1 {
		t.Fatalf("always-on user count = %d, want 1", summary.Policy.AlwaysOnUserCount)
	}
	if summary.ByClass[string(WarmnessClassPremiumAlwaysOn)] != 1 {
		t.Fatalf("warmness summary = %+v, want one premium class", summary)
	}
}

func TestHandler_IdleCheckIncludesPressureReclaimPlan(t *testing.T) {
	srv, reg := newTestServer(t)
	reg.SetPressureReclaimConfig(PressureReclaimConfig{
		Mode:          PressureReclaimModeDryRun,
		MinIdle:       10 * time.Minute,
		MaxCandidates: 5,
	})
	reg.setPressureSamplerForTest(func(cfg PressureReclaimConfig) HostPressureSample {
		return HostPressureSample{SampledAt: "2026-05-14T12:00:00Z"}
	})

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/internal/vmctl/idle-check", nil)
	req.Header.Set("X-Internal-Caller", "true")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("idle-check request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result struct {
		Status            string              `json:"status"`
		StaleStateDeleted int                 `json:"stale_state_deleted"`
		Reclaim           PressureReclaimPlan `json:"reclaim"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode idle-check: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("status = %s, want ok", result.Status)
	}
	if result.Reclaim.Mode != PressureReclaimModeDryRun {
		t.Fatalf("reclaim mode = %s, want dry-run", result.Reclaim.Mode)
	}
	if result.StaleStateDeleted != 0 {
		t.Fatalf("stale_state_deleted = %d, want 0 in dry-run mode", result.StaleStateDeleted)
	}
}

func TestHandler_ResolveRequiresUserID(t *testing.T) {
	srv, _ := newTestServer(t)

	body := `{}`
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/internal/vmctl/resolve", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")

	resp, _ := http.DefaultClient.Do(req)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestHandler_LookupDemotesUnhealthyActiveOwnership(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	persistencePath := filepath.Join(t.TempDir(), "ownership.json")
	if err := reg.SetPersistencePath(persistencePath); err != nil {
		t.Fatalf("SetPersistencePath: %v", err)
	}
	own := &VMOwnership{
		VMID:        "vm-unhealthy",
		ComputerID:  "computer-unhealthy",
		UserID:      "user-unhealthy",
		DesktopID:   PrimaryDesktopID,
		ComputerURL: "http://127.0.0.1:9001",
		State:       VMStateActive,
		Epoch:       7,
	}
	initialEpoch := own.Epoch
	key := ownershipKey(own.UserID, own.DesktopID)
	reg.ownerships[key] = own
	reg.vmByID[own.VMID] = own
	unhealthy := false
	manager := &mockVMManager{
		getVMs: map[string]*VMInstanceInfo{
			own.VMID: {
				HostURL: own.ComputerURL,
				Epoch:   own.Epoch,
				Healthy: false,
				State:   "running",
			},
		},
		checkHealthOK: &unhealthy,
		recoverResponse: &VMInstanceInfo{
			HostURL: "http://127.0.0.1:9002",
			Epoch:   initialEpoch + 1,
			Healthy: true,
			State:   "running",
		},
	}
	reg.SetVMManager(manager)
	handler := NewHandler(reg)

	request := httptest.NewRequest(http.MethodGet, "/internal/vmctl/lookup?computer_id="+own.ComputerID, nil)
	request.Header.Set("X-Internal-Caller", "true")
	response := httptest.NewRecorder()
	handler.HandleLookup(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("lookup status = %d, want 200; body=%s", response.Code, response.Body.String())
	}
	var result ownershipResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode lookup response: %v", err)
	}
	if result.State != string(VMStateDegraded) {
		t.Fatalf("lookup state = %q, want %q", result.State, VMStateDegraded)
	}
	stored := reg.GetOwnershipForDesktop(own.UserID, own.DesktopID)
	if stored == nil || stored.State != VMStateDegraded {
		t.Fatalf("stored ownership = %+v, want durable degraded state", stored)
	}
	persistedBytes, err := os.ReadFile(persistencePath)
	if err != nil {
		t.Fatalf("read persisted ownership: %v", err)
	}
	var persisted persistedOwnershipState
	if err := json.Unmarshal(persistedBytes, &persisted); err != nil {
		t.Fatalf("decode persisted ownership: %v", err)
	}
	if len(persisted.Ownerships) != 1 || persisted.Ownerships[0].State != VMStateDegraded {
		t.Fatalf("persisted ownerships = %+v, want degraded ownership", persisted.Ownerships)
	}
	restarted := NewOwnershipRegistry("http://127.0.0.1:8085")
	if err := restarted.SetPersistencePath(persistencePath); err != nil {
		t.Fatalf("reload ownership: %v", err)
	}
	reloaded := restarted.GetOwnershipForDesktop(own.UserID, own.DesktopID)
	if reloaded == nil || reloaded.State != VMStateStopped {
		t.Fatalf("reloaded ownership = %+v, want stopped recovery state", reloaded)
	}
	recovered, err := reg.ResolveOrAssignDesktop(own.UserID, own.DesktopID)
	if err != nil {
		t.Fatalf("resolve degraded ownership: %v", err)
	}
	if recovered.VMID != own.VMID || recovered.ComputerID != own.ComputerID {
		t.Fatalf("recovered identity = %+v, want retained VMID %s ComputerID %s", recovered, own.VMID, own.ComputerID)
	}
	if recovered.Epoch != initialEpoch+1 {
		t.Fatalf("recovered epoch = %d, want %d", recovered.Epoch, initialEpoch+1)
	}
	if len(manager.recovers) != 1 || manager.recovers[0] != own.VMID {
		t.Fatalf("manager recovers = %v, want [%s]", manager.recovers, own.VMID)
	}
	if len(manager.checkHealthCalls) != 1 || manager.checkHealthCalls[0] != own.VMID {
		t.Fatalf("health checks = %v, want [%s]", manager.checkHealthCalls, own.VMID)
	}
}

func TestHandler_LookupKeepsActiveDuringUnhealthyRouteGrace(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	persistencePath := filepath.Join(t.TempDir(), "ownership.json")
	if err := reg.SetPersistencePath(persistencePath); err != nil {
		t.Fatalf("SetPersistencePath: %v", err)
	}
	own := &VMOwnership{
		VMID:        "vm-grace",
		ComputerID:  "computer-grace",
		UserID:      "user-grace",
		DesktopID:   PrimaryDesktopID,
		ComputerURL: "http://127.0.0.1:9001",
		State:       VMStateActive,
		Epoch:       8,
	}
	key := ownershipKey(own.UserID, own.DesktopID)
	reg.ownerships[key] = own
	reg.vmByID[own.VMID] = own
	unhealthy := false
	recentHealthy := time.Now().Add(-10 * time.Second)
	manager := &mockVMManager{
		getVMs: map[string]*VMInstanceInfo{
			own.VMID: {
				HostURL:       own.ComputerURL,
				Epoch:         own.Epoch,
				Healthy:       false,
				State:         "running",
				LastHealthyAt: recentHealthy,
			},
		},
		checkHealthOK: &unhealthy,
	}
	reg.SetVMManager(manager)
	handler := NewHandler(reg)

	request := httptest.NewRequest(http.MethodGet, "/internal/vmctl/lookup?user_id="+own.UserID, nil)
	request.Header.Set("X-Internal-Caller", "true")
	response := httptest.NewRecorder()
	handler.HandleLookup(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("lookup status = %d body=%s", response.Code, response.Body.String())
	}
	var result ownershipResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.State != string(VMStateActive) {
		t.Fatalf("lookup state = %q, want active during unhealthy-route grace", result.State)
	}
	stored := reg.GetOwnershipForDesktop(own.UserID, own.DesktopID)
	if stored == nil || stored.State != VMStateActive {
		t.Fatalf("stored ownership = %+v, want active during grace", stored)
	}
}

func TestHandler_LookupAuthorizesRouteBeforeHealthMutation(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	own := &VMOwnership{
		VMID:        "vm-route-refused",
		ComputerID:  "computer-route-refused",
		UserID:      "user-route-refused",
		DesktopID:   PrimaryDesktopID,
		ComputerURL: "http://127.0.0.1:9001",
		State:       VMStateActive,
		Epoch:       7,
	}
	key := ownershipKey(own.UserID, own.DesktopID)
	reg.ownerships[key] = own
	reg.vmByID[own.VMID] = own
	unhealthy := false
	manager := &mockVMManager{checkHealthOK: &unhealthy}
	reg.SetVMManager(manager)
	handler := NewHandler(reg)
	handler.routeAuthorityRequired = true

	request := httptest.NewRequest(http.MethodGet, "/internal/vmctl/lookup?computer_id="+own.ComputerID, nil)
	request.Header.Set("X-Internal-Caller", "true")
	response := httptest.NewRecorder()
	handler.HandleLookup(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("lookup status = %d, want 409; body=%s", response.Code, response.Body.String())
	}
	stored := reg.GetOwnershipForDesktop(own.UserID, own.DesktopID)
	if stored == nil || stored.State != VMStateActive || stored.Epoch != own.Epoch {
		t.Fatalf("route-refused lookup mutated ownership: %+v", stored)
	}
	if len(manager.checkHealthCalls) != 0 {
		t.Fatalf("route-refused lookup health checks = %v, want none", manager.checkHealthCalls)
	}
}

func TestHandler_LookupProjectsBootingWithoutRacingRefresh(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	own := &VMOwnership{
		VMID:        "vm-refreshing",
		ComputerID:  "computer-refreshing",
		UserID:      "user-refreshing",
		DesktopID:   PrimaryDesktopID,
		ComputerURL: "http://127.0.0.1:9001",
		State:       VMStateActive,
		Epoch:       7,
	}
	initialEpoch := own.Epoch
	key := ownershipKey(own.UserID, own.DesktopID)
	reg.ownerships[key] = own
	reg.vmByID[own.VMID] = own
	manager := &mockVMManager{
		getVMs: map[string]*VMInstanceInfo{
			own.VMID: {
				HostURL: own.ComputerURL,
				Epoch:   own.Epoch,
				Healthy: true,
				State:   "running",
			},
		},
		refreshResponse: &VMInstanceInfo{
			HostURL: "http://127.0.0.1:9002",
			Epoch:   initialEpoch + 1,
			Healthy: true,
			State:   "running",
		},
		refreshStarted: make(chan struct{}),
		refreshRelease: make(chan struct{}),
	}
	reg.SetVMManager(manager)
	refreshDone := make(chan error, 1)
	go func() {
		_, err := reg.RefreshVMForDesktop(own.UserID, own.DesktopID)
		refreshDone <- err
	}()
	<-manager.refreshStarted

	request := httptest.NewRequest(http.MethodGet, "/internal/vmctl/lookup?computer_id="+own.ComputerID, nil)
	request.Header.Set("X-Internal-Caller", "true")
	response := httptest.NewRecorder()
	NewHandler(reg).HandleLookup(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("lookup status = %d, want 200; body=%s", response.Code, response.Body.String())
	}
	var result ownershipResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode lookup response: %v", err)
	}
	if result.State != string(VMStateBooting) {
		t.Fatalf("lookup state = %q, want %q during refresh", result.State, VMStateBooting)
	}
	stored := reg.GetOwnershipForDesktop(own.UserID, own.DesktopID)
	if stored == nil || stored.State != VMStateActive || stored.Epoch != initialEpoch {
		t.Fatalf("lookup mutated refresh transaction: %+v", stored)
	}
	if len(manager.checkHealthCalls) != 0 {
		t.Fatalf("lookup health checks during refresh = %v, want none", manager.checkHealthCalls)
	}

	close(manager.refreshRelease)
	if err := <-refreshDone; err != nil {
		t.Fatalf("refresh completion: %v", err)
	}
	refreshed := reg.GetOwnershipForDesktop(own.UserID, own.DesktopID)
	if refreshed == nil || refreshed.VMID != own.VMID || refreshed.ComputerID != own.ComputerID ||
		refreshed.State != VMStateActive || refreshed.Epoch != initialEpoch+1 {
		t.Fatalf("refreshed ownership = %+v, want retained active identity at epoch %d", refreshed, initialEpoch+1)
	}
}

func TestOwnershipRegistry_DegradedRecoveryFailureDoesNotPublishActive(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	own := &VMOwnership{
		VMID:        "vm-recovery-fails",
		ComputerID:  "computer-recovery-fails",
		UserID:      "user-recovery-fails",
		DesktopID:   PrimaryDesktopID,
		ComputerURL: "http://127.0.0.1:9001",
		State:       VMStateDegraded,
		Epoch:       7,
	}
	key := ownershipKey(own.UserID, own.DesktopID)
	reg.ownerships[key] = own
	reg.vmByID[own.VMID] = own
	manager := &mockVMManager{
		getVMs: map[string]*VMInstanceInfo{
			own.VMID: {
				HostURL: own.ComputerURL,
				Epoch:   own.Epoch,
				Healthy: false,
				State:   "running",
			},
		},
		recoverError: errors.New("guest recovery failed"),
	}
	reg.SetVMManager(manager)

	if recovered, err := reg.ResolveOrAssignDesktop(own.UserID, own.DesktopID); err == nil {
		t.Fatalf("resolve returned %+v, want recovery error", recovered)
	}
	stored := reg.GetOwnershipForDesktop(own.UserID, own.DesktopID)
	if stored == nil || stored.VMID != own.VMID || stored.ComputerID != own.ComputerID ||
		stored.Epoch != own.Epoch || stored.State != VMStateDegraded {
		t.Fatalf("failed recovery published or replaced ownership: %+v", stored)
	}
	if len(manager.recovers) != 1 || manager.recovers[0] != own.VMID {
		t.Fatalf("manager recovers = %v, want [%s]", manager.recovers, own.VMID)
	}
}

func TestOwnershipRegistry_ComputerIDLookupFailsClosedOnAmbiguity(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.ownerships[ownershipKey("owner-a", "primary")] = &VMOwnership{
		UserID: "owner-a", DesktopID: "primary", VMID: "vm-a", ComputerID: "computer-duplicate",
	}
	reg.ownerships[ownershipKey("owner-b", "primary")] = &VMOwnership{
		UserID: "owner-b", DesktopID: "primary", VMID: "vm-b", ComputerID: "computer-duplicate",
	}
	if got := reg.GetOwnershipByComputerID("computer-duplicate"); got != nil {
		t.Fatalf("ambiguous ComputerID resolved to %+v", got)
	}
}

// --- Client Tests ---

func TestClient_ResolveDesktopContextCancelsRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	client := NewClientWithTimeout(srv.URL, time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := client.ResolveDesktopContext(ctx, "user-cancel", PrimaryDesktopID); err == nil {
		t.Fatal("expected canceled resolve request to fail")
	}
}

// --- isInternalCaller Tests ---

func TestIsInternalCaller(t *testing.T) {
	tests := []struct {
		name       string
		host       string
		remoteAddr string
		header     string
		want       bool
	}{
		{"localhost host", "localhost:8083", "127.0.0.1:12345", "", true},
		{"127.0.0.1 host", "127.0.0.1:8083", "127.0.0.1:12345", "", true},
		{"::1 host", "[::1]:8083", "[::1]:12345", "", true},
		{"external host", "192.168.1.1:8083", "10.0.0.1:12345", "", false},
		{"internal header", "external:8083", "10.0.0.1:12345", "true", true},
		{"empty header", "external:8083", "10.0.0.1:12345", "false", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &http.Request{
				Host:       tt.host,
				RemoteAddr: tt.remoteAddr,
				Header:     http.Header{"X-Internal-Caller": {tt.header}},
			}
			if got := isInternalCaller(r); got != tt.want {
				t.Errorf("isInternalCaller(%+v) = %v, want %v", tt, got, tt.want)
			}
		})
	}
}

// --- Timing Tests ---

// --- Endpoint URL Tests ---

func TestPulseSummaryAggregatesWithoutIdentityOutput(t *testing.T) {
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	stateDir := t.TempDir()
	users := []pulseAccountRecord{
		{UserID: "real-1", Email: "alpha@choir.news", CreatedAt: now.Add(-2 * time.Hour)},
		{UserID: "real-2", Email: "beta@me.com", CreatedAt: now.Add(-10 * 24 * time.Hour)},
		{UserID: "codex-1", Email: "proof@example.com", CreatedAt: now.Add(-time.Hour)},
		{UserID: "protected-a", Email: "a@b.com", CreatedAt: now.Add(-time.Hour)},
	}
	userByID := map[string]pulseAccountRecord{}
	for _, user := range users {
		userByID[user.UserID] = user
	}
	ownerships := []*VMOwnership{
		{VMID: "vm-real-1", UserID: "real-1", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive, LastActiveAt: now.Add(-time.Hour)},
		{VMID: "vm-real-2", UserID: "real-2", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateFailed, LastActiveAt: now.Add(-8 * 24 * time.Hour)},
		{VMID: "vm-codex-1", UserID: "codex-1", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateHibernated, LastActiveAt: now.Add(-time.Hour)},
		{VMID: "vm-protected-a", UserID: "protected-a", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateHibernated, LastActiveAt: now.Add(-time.Hour)},
	}
	for _, own := range ownerships {
		dir := filepath.Join(stateDir, own.VMID)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", own.VMID, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "data"), []byte("state"), 0o600); err != nil {
			t.Fatalf("write %s: %v", own.VMID, err)
		}
	}

	summary := pulseSummaryFromSnapshot(now, stateDir, users, userByID, ownerships, true, nil)
	if summary.Accounts.ByClass[PulseAccountReal] != 2 {
		t.Fatalf("real account count = %d, want 2", summary.Accounts.ByClass[PulseAccountReal])
	}
	if summary.Accounts.ByClass[PulseAccountCodexAgenticTest] != 1 {
		t.Fatalf("codex account count = %d, want 1", summary.Accounts.ByClass[PulseAccountCodexAgenticTest])
	}
	if summary.Accounts.ByClass[PulseAccountProtectedTest] != 1 {
		t.Fatalf("protected account count = %d, want 1", summary.Accounts.ByClass[PulseAccountProtectedTest])
	}
	if summary.Accounts.NewRealLast24h != 1 || summary.Accounts.NewRealLast7d != 1 || summary.Accounts.NewRealLast30d != 2 {
		t.Fatalf("new real buckets = %d/%d/%d, want 1/1/2", summary.Accounts.NewRealLast24h, summary.Accounts.NewRealLast7d, summary.Accounts.NewRealLast30d)
	}
	if summary.Activity.RealActiveLast24h != 1 || summary.Activity.RealActiveLast7d != 1 || summary.Activity.RealActiveLast30d != 2 {
		t.Fatalf("active real buckets = %d/%d/%d, want 1/1/2", summary.Activity.RealActiveLast24h, summary.Activity.RealActiveLast7d, summary.Activity.RealActiveLast30d)
	}
	if summary.Reliability.RealPrimaryFailed != 1 {
		t.Fatalf("failed real primary = %d, want 1", summary.Reliability.RealPrimaryFailed)
	}
	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal summary: %v", err)
	}
	for _, forbidden := range []string{"alpha@choir.news", "beta@me.com", "proof@example.com", "a@b.com", "real-1", "codex-1"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("Pulse summary leaked identity %q in %s", forbidden, string(data))
		}
	}
}

// --- Lifecycle Tests (VAL-VM-008, VAL-VM-009, VAL-CROSS-116, VAL-CROSS-117) ---

func TestOwnershipRegistry_HibernateAndResume(t *testing.T) {
	// VAL-CROSS-116: Idle stop or hibernate resumes the same user's state.
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	own1, _ := reg.ResolveOrAssign("user-1")
	vmID := own1.VMID
	epoch := own1.Epoch

	// Hibernate the VM.
	if err := reg.HibernateVM("user-1"); err != nil {
		t.Fatalf("HibernateVM: %v", err)
	}

	own := reg.GetOwnership("user-1")
	if own.State != VMStateHibernated {
		t.Errorf("expected hibernated state, got %s", own.State)
	}
	if own.VMID != vmID {
		t.Errorf("expected same VMID after hibernate, got %s", own.VMID)
	}

	// Resume the VM — epoch should NOT change (VAL-CROSS-117).
	resumed, err := reg.ResumeVM("user-1")
	if err != nil {
		t.Fatalf("ResumeVM: %v", err)
	}
	if resumed.State != VMStateActive {
		t.Errorf("expected active state after resume, got %s", resumed.State)
	}
	if resumed.VMID != vmID {
		t.Errorf("expected same VMID after resume, got %s", resumed.VMID)
	}
	if resumed.Epoch != epoch {
		t.Errorf("expected epoch %d after resume (no increment), got %d", epoch, resumed.Epoch)
	}
}

func TestOwnershipRegistry_RecoverIncrementsEpoch(t *testing.T) {
	// VAL-CROSS-117: Crash recovery does not duplicate canonical effects.
	// Recovery increments epoch to signal fresh boot.
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	own1, _ := reg.ResolveOrAssign("user-1")
	vmID := own1.VMID
	epoch := own1.Epoch

	// Mark unhealthy and recover.
	if err := reg.MarkUnhealthy("user-1"); err != nil {
		t.Fatalf("MarkUnhealthy: %v", err)
	}

	recovered, err := reg.RecoverVM("user-1")
	if err != nil {
		t.Fatalf("RecoverVM: %v", err)
	}

	if recovered.State != VMStateActive {
		t.Errorf("expected active state after recovery, got %s", recovered.State)
	}
	if recovered.VMID != vmID {
		t.Errorf("expected same VMID after recovery, got %s", recovered.VMID)
	}
	if recovered.Epoch <= epoch {
		t.Errorf("expected epoch > %d after recovery (fresh boot), got %d", epoch, recovered.Epoch)
	}
}

func TestOwnershipRegistry_LogoutStopsOnlyCurrentUser(t *testing.T) {
	// VAL-VM-008: Logout or idle transitions only the current user's VM.
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	if _, err := reg.ResolveOrAssign("user-alice"); err != nil {
		t.Fatalf("ResolveOrAssign alice: %v", err)
	}
	if _, err := reg.ResolveOrAssign("user-bob"); err != nil {
		t.Fatalf("ResolveOrAssign bob: %v", err)
	}

	// Logout user-alice.
	if err := reg.LogoutVM("user-alice"); err != nil {
		t.Fatalf("LogoutVM: %v", err)
	}

	// Alice's VM should be stopped.
	aliceOwn := reg.GetOwnership("user-alice")
	if aliceOwn.State != VMStateStopped {
		t.Errorf("expected alice VM stopped after logout, got %s", aliceOwn.State)
	}
	if aliceOwn.StoppedBy != "logout" {
		t.Errorf("expected stopped_by=logout, got %s", aliceOwn.StoppedBy)
	}

	// Bob's VM should still be active.
	bobOwn := reg.GetOwnership("user-bob")
	if bobOwn.State != VMStateActive {
		t.Errorf("expected bob VM still active after alice logout, got %s", bobOwn.State)
	}
}

func TestOwnershipRegistry_IdleTimeoutChecks(t *testing.T) {
	// VAL-VM-008: Idle timeout transitions inactive VMs.
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetIdleTimeout(50 * time.Millisecond)

	if _, err := reg.ResolveOrAssign("user-active"); err != nil {
		t.Fatalf("ResolveOrAssign user-active: %v", err)
	}
	if _, err := reg.ResolveOrAssign("user-idle"); err != nil {
		t.Fatalf("ResolveOrAssign user-idle: %v", err)
	}

	// Simulate user-idle being idle by backdating its LastActiveAt.
	reg.mu.Lock()
	idleOwn := reg.ownerships[ownershipKey("user-idle", PrimaryDesktopID)]
	idleOwn.LastActiveAt = time.Now().Add(-100 * time.Millisecond)
	reg.mu.Unlock()

	// Check idle VMs — should only find user-idle.
	idleUsers := reg.CheckIdleVMs()
	if len(idleUsers) != 1 {
		t.Fatalf("expected 1 idle user, got %d: %v", len(idleUsers), idleUsers)
	}
	if idleUsers[0] != "user-idle" {
		t.Errorf("expected user-idle to be idle, got %s", idleUsers[0])
	}

	// Stop idle VMs.
	stopped := reg.StopIdleVMs(context.Background(), allowLifecycleRoute)
	if stopped != 1 {
		t.Errorf("expected 1 VM stopped, got %d", stopped)
	}

	// Verify user-idle is now hibernated.
	idleOwn = reg.GetOwnership("user-idle")
	if idleOwn.State != VMStateHibernated {
		t.Errorf("expected hibernated after idle stop, got %s", idleOwn.State)
	}

	// Verify user-active is still active.
	activeOwn := reg.GetOwnership("user-active")
	if activeOwn.State != VMStateActive {
		t.Errorf("expected active VM still running, got %s", activeOwn.State)
	}
}

func TestOwnershipRegistry_IdleSweeperHibernatesIdleVM(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetIdleTimeout(10 * time.Millisecond)

	if _, err := reg.ResolveOrAssign("user-idle-sweeper"); err != nil {
		t.Fatalf("ResolveOrAssign: %v", err)
	}

	reg.mu.Lock()
	reg.ownerships[ownershipKey("user-idle-sweeper", PrimaryDesktopID)].LastActiveAt = time.Now().Add(-time.Minute)
	reg.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reg.StartIdleSweeper(ctx, 5*time.Millisecond, func(context.Context, string, string) error { return nil })

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		own := reg.GetOwnership("user-idle-sweeper")
		if own != nil && own.State == VMStateHibernated {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	own := reg.GetOwnership("user-idle-sweeper")
	if own == nil {
		t.Fatal("expected ownership after idle sweep")
	}
	t.Fatalf("expected hibernated after idle sweep, got %s", own.State)
}

func TestOwnershipRegistry_HibernateRequiresRunningVM(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	// No VM at all.
	if err := reg.HibernateVM("nonexistent"); err == nil {
		t.Error("expected error for nonexistent user")
	}

	// Already stopped VM.
	if _, err := reg.ResolveOrAssign("user-1"); err != nil {
		t.Fatalf("ResolveOrAssign: %v", err)
	}
	if err := reg.StopVM("user-1"); err != nil {
		t.Fatalf("StopVM: %v", err)
	}
	if err := reg.HibernateVM("user-1"); err == nil {
		t.Error("expected error for stopped VM")
	}
}

func TestOwnershipRegistry_ResumeNonResumableState(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	// No VM at all.
	_, err := reg.ResumeVM("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent user")
	}

	// Active VM — resume returns it as-is.
	if _, err := reg.ResolveOrAssign("user-1"); err != nil {
		t.Fatalf("ResolveOrAssign: %v", err)
	}
	own, err := reg.ResumeVM("user-1")
	if err != nil {
		t.Fatalf("ResumeVM on active: %v", err)
	}
	if own.State != VMStateActive {
		t.Errorf("expected active, got %s", own.State)
	}
}

func TestOwnershipRegistry_RecoverRequiresFailedState(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	// No VM at all.
	_, err := reg.RecoverVM("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent user")
	}

	// Active VM — cannot recover.
	if _, err := reg.ResolveOrAssign("user-1"); err != nil {
		t.Fatalf("ResolveOrAssign: %v", err)
	}
	_, err = reg.RecoverVM("user-1")
	if err == nil {
		t.Error("expected error for active VM")
	}
}

func TestOwnershipRegistry_EpochTracksBootGeneration(t *testing.T) {
	// VAL-CROSS-117: Epoch tracking prevents duplicate canonical effects.
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	// First assignment gets an epoch.
	own1, _ := reg.ResolveOrAssign("user-1")
	epoch1 := own1.Epoch
	if epoch1 == 0 {
		t.Error("expected non-zero epoch")
	}

	// Resolve again (same active VM) — epoch stays the same.
	own2, _ := reg.ResolveOrAssign("user-1")
	if own2.Epoch != epoch1 {
		t.Errorf("expected same epoch on re-resolve, got %d vs %d", epoch1, own2.Epoch)
	}

	// Stop and resolve (resume) — epoch stays the same.
	if err := reg.StopVM("user-1"); err != nil {
		t.Fatalf("StopVM: %v", err)
	}
	own3, _ := reg.ResolveOrAssign("user-1")
	if own3.Epoch != epoch1 {
		t.Errorf("expected same epoch on resume, got %d vs %d", epoch1, own3.Epoch)
	}

	// Mark unhealthy and recover — epoch increments.
	if err := reg.MarkUnhealthy("user-1"); err != nil {
		t.Fatalf("MarkUnhealthy: %v", err)
	}
	own4, _ := reg.RecoverVM("user-1")
	if own4.Epoch <= epoch1 {
		t.Errorf("expected epoch > %d after recovery, got %d", epoch1, own4.Epoch)
	}
}

func TestOwnershipRegistry_FailedVMPreservesIdentityOnRecovery(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	own1, err := reg.ResolveOrAssign("user-1")
	if err != nil {
		t.Fatalf("initial resolve: %v", err)
	}
	oldVMID := own1.VMID
	oldComputerID := own1.ComputerID
	oldEpoch := own1.Epoch

	reg.mu.Lock()
	reg.ownerships[ownershipKey("user-1", PrimaryDesktopID)].State = VMStateFailed
	reg.mu.Unlock()

	own2, err := reg.ResolveOrAssign("user-1")
	if err != nil {
		t.Fatalf("recover failed ownership: %v", err)
	}
	if own2.VMID != oldVMID || own2.ComputerID != oldComputerID {
		t.Fatalf("recovered identity = %+v, want VMID %s ComputerID %s", own2, oldVMID, oldComputerID)
	}
	if own2.Epoch <= oldEpoch {
		t.Fatalf("recovered epoch = %d, want greater than %d", own2.Epoch, oldEpoch)
	}
}

func TestOwnershipRegistry_NoIdleTimeoutWhenZero(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	// Default idle timeout is 0 — no idle checking.

	if _, err := reg.ResolveOrAssign("user-1"); err != nil {
		t.Fatalf("ResolveOrAssign: %v", err)
	}

	// Backdate the last active time.
	reg.mu.Lock()
	reg.ownerships[ownershipKey("user-1", PrimaryDesktopID)].LastActiveAt = time.Now().Add(-24 * time.Hour)
	reg.mu.Unlock()

	// Should find no idle VMs.
	idle := reg.CheckIdleVMs()
	if len(idle) != 0 {
		t.Errorf("expected no idle VMs with zero timeout, got %d", len(idle))
	}
}

func TestOwnershipRegistry_ResolveAfterLogout(t *testing.T) {
	// VAL-VM-008: After logout, the next request wakes or recreates the VM.
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	own1, _ := reg.ResolveOrAssign("user-1")
	vmID := own1.VMID

	if err := reg.LogoutVM("user-1"); err != nil {
		t.Fatalf("LogoutVM: %v", err)
	}

	// Resolving after logout should resume the same VM (VAL-CROSS-116).
	own2, _ := reg.ResolveOrAssign("user-1")
	if own2.VMID != vmID {
		t.Errorf("expected same VMID after logout+resolve (resume), got %s vs %s", vmID, own2.VMID)
	}
	if own2.State != VMStateActive {
		t.Errorf("expected active state after logout+resolve, got %s", own2.State)
	}
}

// --- Handler Lifecycle Tests ---

func TestHandler_ResumeBootsPersistedVMWhenManagerLostInstance(t *testing.T) {
	srv, reg := newTestServer(t)
	mock := &mockVMManager{
		bootResponse: &VMInstanceInfo{
			HostURL: "http://127.0.0.1:9100",
			Epoch:   1,
			Healthy: true,
			State:   "running",
		},
	}
	reg.SetVMManager(mock)

	body := `{"user_id":"user-resume-orphan"}`
	req1, _ := http.NewRequest(http.MethodPost, srv.URL+"/internal/vmctl/resolve", strings.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("X-Internal-Caller", "true")
	resp1, _ := http.DefaultClient.Do(req1)
	var result1 resolveResponse
	if err := json.NewDecoder(resp1.Body).Decode(&result1); err != nil {
		t.Fatalf("decode result1: %v", err)
	}
	_ = resp1.Body.Close()

	req2, _ := http.NewRequest(http.MethodPost, srv.URL+"/internal/vmctl/hibernate", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-Internal-Caller", "true")
	resp2, _ := http.DefaultClient.Do(req2)
	_ = resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on hibernate, got %d", resp2.StatusCode)
	}

	mock.boots = nil
	mock.resumes = nil
	mock.resumeError = fmt.Errorf("vm not found after vmctl restart")
	mock.bootResponse = &VMInstanceInfo{
		HostURL: "http://127.0.0.1:9101",
		Epoch:   2,
		Healthy: true,
		State:   "running",
	}

	req3, _ := http.NewRequest(http.MethodPost, srv.URL+"/internal/vmctl/resume", strings.NewReader(body))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("X-Internal-Caller", "true")
	resp3, _ := http.DefaultClient.Do(req3)
	defer func() { _ = resp3.Body.Close() }()
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on resume boot fallback, got %d", resp3.StatusCode)
	}

	var result3 resolveResponse
	if err := json.NewDecoder(resp3.Body).Decode(&result3); err != nil {
		t.Fatalf("decode result3: %v", err)
	}
	if result3.VMID != result1.VMID {
		t.Fatalf("resumed VMID = %s, want %s", result3.VMID, result1.VMID)
	}
	if result3.ComputerURL != "http://127.0.0.1:9101" {
		t.Fatalf("ComputerURL = %s, want boot fallback URL", result3.ComputerURL)
	}
	if len(mock.boots) != 1 || mock.boots[0].VMID != result1.VMID {
		t.Fatalf("boot fallback = %+v, want one boot for %s", mock.boots, result1.VMID)
	}
}

func TestHandler_RecoverRequiresUnhealthyState(t *testing.T) {
	srv, reg := newTestServer(t)

	// Create a VM.
	body := `{"user_id":"user-1"}`
	req1, _ := http.NewRequest(http.MethodPost, srv.URL+"/internal/vmctl/resolve", strings.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("X-Internal-Caller", "true")
	resp1, _ := http.DefaultClient.Do(req1)
	_ = resp1.Body.Close()

	// Try to recover a healthy VM — should fail.
	req2, _ := http.NewRequest(http.MethodPost, srv.URL+"/internal/vmctl/recover", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-Internal-Caller", "true")
	resp2, _ := http.DefaultClient.Do(req2)
	defer func() { _ = resp2.Body.Close() }()

	if resp2.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for healthy VM recovery, got %d", resp2.StatusCode)
	}

	// Mark unhealthy.
	if err := reg.MarkUnhealthy("user-1"); err != nil {
		t.Fatalf("MarkUnhealthy: %v", err)
	}

	// Now recover should succeed with a new epoch.
	req3, _ := http.NewRequest(http.MethodPost, srv.URL+"/internal/vmctl/recover", strings.NewReader(body))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("X-Internal-Caller", "true")
	resp3, _ := http.DefaultClient.Do(req3)
	defer func() { _ = resp3.Body.Close() }()

	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on recovery of unhealthy VM, got %d", resp3.StatusCode)
	}

	var result resolveResponse
	if err := json.NewDecoder(resp3.Body).Decode(&result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result.State != "active" {
		t.Errorf("expected active state after recovery, got %s", result.State)
	}
}

func TestIssueGatewayToken_GatewayFailure(t *testing.T) {
	// When the gateway returns an error, issueGatewayToken returns empty string.
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer gateway.Close()

	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetGatewayURL(gateway.URL)

	token := reg.issueGatewayToken("vm-test-123")
	if token != "" {
		t.Errorf("expected empty token on gateway failure, got %q", token)
	}
}

// mockVMManager is a test double for the VMManager interface.
// It records lifecycle calls so tests can verify that the OwnershipRegistry
// properly delegates to the VM manager when one is configured.
type mockVMManager struct {
	mu             sync.Mutex
	boots          []VMManagerConfig
	stops          []string
	hibernates     []string
	resumes        []string
	reattaches     []string
	reattachCfgs   []VMManagerConfig
	recovers       []string
	refreshes      []string
	recoverCfgs    []VMManagerConfig
	refreshCfgs    []VMManagerConfig
	destroys       []string
	tokens         map[string]string
	reservedEpochs map[string]int64
	// Configurable responses
	bootResponse       *VMInstanceInfo
	bootError          error
	bootHook           func(VMManagerConfig)
	stopError          error
	destroyError       error
	resumeResponse     *VMInstanceInfo
	resumeError        error
	reattachResponse   *VMInstanceInfo
	reattachError      error
	recoverResponse    *VMInstanceInfo
	recoverError       error
	refreshResponse    *VMInstanceInfo
	refreshError       error
	getVMs             map[string]*VMInstanceInfo
	checkHealthOK      *bool
	checkHealthError   error
	checkHealthCalls   []string
	resumeStarted      chan struct{}
	resumeRelease      chan struct{}
	resumeStartedOnce  sync.Once
	recoverStarted     chan struct{}
	recoverRelease     chan struct{}
	recoverStartedOnce sync.Once
	refreshStarted     chan struct{}
	refreshRelease     chan struct{}
	refreshStartedOnce sync.Once
}

func (m *mockVMManager) ReserveBootEpoch(vmID string, minimum int64) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.reservedEpochs == nil {
		m.reservedEpochs = make(map[string]int64)
	}
	next := m.reservedEpochs[vmID] + 1
	if next < minimum {
		next = minimum
	}
	m.reservedEpochs[vmID] = next
	return next, nil
}

func (m *mockVMManager) BootVM(cfg VMManagerConfig) (*VMInstanceInfo, error) {
	m.boots = append(m.boots, cfg)
	if m.bootHook != nil {
		m.bootHook(cfg)
	}
	if m.bootError != nil {
		return nil, m.bootError
	}
	if m.bootResponse != nil {
		return m.bootResponse, nil
	}
	return &VMInstanceInfo{HostURL: "http://127.0.0.1:9001", Epoch: 1, Healthy: true, State: "running"}, nil
}

func (m *mockVMManager) StopVM(vmID string) error {
	m.stops = append(m.stops, vmID)
	return m.stopError
}

func (m *mockVMManager) HibernateVM(vmID string) error {
	m.hibernates = append(m.hibernates, vmID)
	return nil
}

func (m *mockVMManager) ResumeVM(vmID string) (*VMInstanceInfo, error) {
	m.mu.Lock()
	m.resumes = append(m.resumes, vmID)
	m.resumeStartedOnce.Do(func() {
		if m.resumeStarted != nil {
			close(m.resumeStarted)
		}
	})
	release := m.resumeRelease
	m.mu.Unlock()
	if release != nil {
		<-release
	}
	if m.resumeError != nil {
		return nil, m.resumeError
	}
	result := m.resumeResponse
	if result == nil {
		result = &VMInstanceInfo{HostURL: "http://127.0.0.1:9002", Epoch: 1, Healthy: true, State: "running"}
	}
	m.mu.Lock()
	if m.getVMs != nil {
		m.getVMs[vmID] = result
	}
	m.mu.Unlock()
	return result, nil
}

func (m *mockVMManager) ReattachVM(vmID, hostURL string, epoch int64) (*VMInstanceInfo, error) {
	m.reattaches = append(m.reattaches, vmID)
	if m.reattachError != nil {
		return nil, m.reattachError
	}
	if m.reattachResponse != nil {
		return m.reattachResponse, nil
	}
	return &VMInstanceInfo{HostURL: hostURL, Epoch: epoch, Healthy: true, State: "running"}, nil
}

func (m *mockVMManager) ReattachVMWithConfig(vmID, hostURL string, epoch int64, cfg VMManagerConfig) (*VMInstanceInfo, error) {
	m.reattachCfgs = append(m.reattachCfgs, cfg)
	return m.ReattachVM(vmID, hostURL, epoch)
}

func (m *mockVMManager) RecoverVM(vmID string, cfg VMManagerConfig) (*VMInstanceInfo, error) {
	m.mu.Lock()
	m.recovers = append(m.recovers, vmID)
	m.recoverCfgs = append(m.recoverCfgs, cfg)
	m.recoverStartedOnce.Do(func() {
		if m.recoverStarted != nil {
			close(m.recoverStarted)
		}
	})
	release := m.recoverRelease
	m.mu.Unlock()
	if release != nil {
		<-release
	}
	if m.recoverError != nil {
		return nil, m.recoverError
	}
	if m.recoverResponse != nil {
		return m.recoverResponse, nil
	}
	return &VMInstanceInfo{HostURL: "http://127.0.0.1:9003", Epoch: 2, Healthy: true, State: "running"}, nil
}

func (m *mockVMManager) RefreshVM(vmID string, cfg VMManagerConfig) (*VMInstanceInfo, error) {
	m.mu.Lock()
	m.refreshes = append(m.refreshes, vmID)
	m.refreshCfgs = append(m.refreshCfgs, cfg)
	m.refreshStartedOnce.Do(func() {
		if m.refreshStarted != nil {
			close(m.refreshStarted)
		}
	})
	release := m.refreshRelease
	refreshError := m.refreshError
	response := m.refreshResponse
	m.mu.Unlock()
	if release != nil {
		<-release
	}
	if refreshError != nil {
		return nil, refreshError
	}
	if response != nil {
		return response, nil
	}
	return &VMInstanceInfo{HostURL: "http://127.0.0.1:9004", Epoch: 3, Healthy: true, State: "running"}, nil
}

func (m *mockVMManager) DestroyVMState(vmID string) error {
	m.destroys = append(m.destroys, vmID)
	return m.destroyError
}

func (m *mockVMManager) GetVM(vmID string) *VMInstanceInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getVMs != nil {
		return m.getVMs[vmID]
	}
	return nil
}

func (m *mockVMManager) CheckHealth(vmID string) (bool, error) {
	m.checkHealthCalls = append(m.checkHealthCalls, vmID)
	if m.checkHealthError != nil {
		return false, m.checkHealthError
	}
	if m.checkHealthOK != nil {
		return *m.checkHealthOK, nil
	}
	return true, nil
}

func (m *mockVMManager) ReadGatewayToken(vmID string) (string, error) {
	if m.tokens == nil {
		return "", os.ErrNotExist
	}
	token, ok := m.tokens[vmID]
	if !ok {
		return "", os.ErrNotExist
	}
	return token, nil
}

func TestOwnershipRegistry_ResolveReconcilesExistingGatewayCredential(t *testing.T) {
	var ensuredRawToken string
	gatewayMux := http.NewServeMux()
	gatewayMux.HandleFunc("/provider/v1/credentials/ensure", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("ensure method = %s, want POST", r.Method)
		}
		if r.Header.Get("X-Internal-Caller") != "true" {
			t.Errorf("missing internal caller header")
		}
		var req struct {
			RawToken string `json:"raw_token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode ensure request: %v", err)
		}
		ensuredRawToken = req.RawToken
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"computer_id":"vm-existing-old-account","status":"imported"}`))
	})
	gatewayServer := httptest.NewServer(gatewayMux)
	t.Cleanup(gatewayServer.Close)

	rawToken := "vm-existing-old-account:token-from-host-persistent-dir"

	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetGatewayURL(gatewayServer.URL)
	reg.SetVMManager(&mockVMManager{
		tokens: map[string]string{
			"vm-existing-old-account": rawToken,
		},
	})

	now := time.Now()
	own := &VMOwnership{VMID: "vm-existing-old-account",
		UserID:       "user-old-account",
		DesktopID:    PrimaryDesktopID,
		Kind:         VMKindInteractive,
		ComputerURL:  "http://127.0.0.1:9001",
		State:        VMStateActive,
		CreatedAt:    now,
		LastActiveAt: now,
		Epoch:        3}
	reg.mu.Lock()
	reg.ownerships[ownershipKey(own.UserID, own.DesktopID)] = own
	reg.vmByID[own.VMID] = own
	reg.mu.Unlock()

	resolved, err := reg.ResolveOrAssignDesktop("user-old-account", PrimaryDesktopID)
	if err != nil {
		t.Fatalf("ResolveOrAssignDesktop: %v", err)
	}
	if resolved.VMID != own.VMID {
		t.Fatalf("resolved VMID = %q, want %q", resolved.VMID, own.VMID)
	}
	if ensuredRawToken != rawToken {
		t.Fatalf("ensured raw token = %q, want %q", ensuredRawToken, rawToken)
	}
}

func TestOwnershipRegistry_ResolveRecoversUnhealthyActiveVMBeforeRouting(t *testing.T) {
	healthy := false
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	now := time.Now().Add(-time.Hour)
	own := &VMOwnership{VMID: "vm-stale-active",
		UserID:       "user-old-account",
		DesktopID:    PrimaryDesktopID,
		Kind:         VMKindInteractive,
		ComputerURL:  "http://127.0.0.1:9001",
		State:        VMStateActive,
		CreatedAt:    now,
		LastActiveAt: now,
		Epoch:        3}
	reg.mu.Lock()
	reg.ownerships[ownershipKey(own.UserID, own.DesktopID)] = own
	reg.vmByID[own.VMID] = own
	reg.mu.Unlock()

	mock := &mockVMManager{
		getVMs: map[string]*VMInstanceInfo{
			own.VMID: {
				HostURL:       own.ComputerURL,
				Epoch:         own.Epoch,
				Healthy:       false,
				State:         "running",
				StartedAt:     now,
				LastHealthyAt: now,
			},
		},
		checkHealthOK: &healthy,
		recoverResponse: &VMInstanceInfo{
			HostURL: "http://127.0.0.1:9044",
			Epoch:   4,
			Healthy: true,
			State:   "running",
		},
	}
	reg.SetVMManager(mock)

	resolved, err := reg.ResolveOrAssignDesktop("user-old-account", PrimaryDesktopID)
	if err != nil {
		t.Fatalf("ResolveOrAssignDesktop: %v", err)
	}
	if resolved.VMID != own.VMID {
		t.Fatalf("resolved VMID = %q, want %q", resolved.VMID, own.VMID)
	}
	if len(mock.checkHealthCalls) != 1 || mock.checkHealthCalls[0] != own.VMID {
		t.Fatalf("health checks = %+v, want [%s]", mock.checkHealthCalls, own.VMID)
	}
	if len(mock.recovers) != 1 || mock.recovers[0] != own.VMID {
		t.Fatalf("recovers = %+v, want [%s]", mock.recovers, own.VMID)
	}
	if resolved.ComputerURL != "http://127.0.0.1:9044" {
		t.Fatalf("ComputerURL = %q, want recovered host URL", resolved.ComputerURL)
	}
	if resolved.Epoch != 4 {
		t.Fatalf("Epoch = %d, want 4", resolved.Epoch)
	}
	if resolved.State != VMStateActive {
		t.Fatalf("State = %s, want %s", resolved.State, VMStateActive)
	}
}

func TestOwnershipRegistry_ResolvePreservesRecentlyHealthyActiveVM(t *testing.T) {
	healthy := false
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	now := time.Now()
	own := &VMOwnership{VMID: "vm-busy-active",
		UserID:       "user-busy",
		DesktopID:    PrimaryDesktopID,
		Kind:         VMKindInteractive,
		ComputerURL:  "http://127.0.0.1:9001",
		State:        VMStateActive,
		CreatedAt:    now.Add(-time.Hour),
		LastActiveAt: now.Add(-time.Minute),
		Epoch:        3}
	reg.mu.Lock()
	reg.ownerships[ownershipKey(own.UserID, own.DesktopID)] = own
	reg.vmByID[own.VMID] = own
	reg.mu.Unlock()

	mock := &mockVMManager{
		getVMs: map[string]*VMInstanceInfo{
			own.VMID: {
				HostURL:       "http://127.0.0.1:9009",
				Epoch:         own.Epoch,
				Healthy:       false,
				State:         "running",
				StartedAt:     now.Add(-time.Hour),
				LastHealthyAt: now.Add(-10 * time.Second),
			},
		},
		checkHealthOK: &healthy,
	}
	reg.SetVMManager(mock)

	resolved, err := reg.ResolveOrAssignDesktop("user-busy", PrimaryDesktopID)
	if err != nil {
		t.Fatalf("ResolveOrAssignDesktop: %v", err)
	}
	if len(mock.checkHealthCalls) != 1 || mock.checkHealthCalls[0] != own.VMID {
		t.Fatalf("health checks = %+v, want [%s]", mock.checkHealthCalls, own.VMID)
	}
	if len(mock.recovers) != 0 {
		t.Fatalf("recovers = %+v, want none for transient health failure", mock.recovers)
	}
	if resolved.ComputerURL != "http://127.0.0.1:9009" {
		t.Fatalf("ComputerURL = %q, want current manager host URL", resolved.ComputerURL)
	}
	if resolved.Epoch != own.Epoch {
		t.Fatalf("Epoch = %d, want %d", resolved.Epoch, own.Epoch)
	}
}

func TestOwnershipRegistry_ResolvePreservesPendingActiveBoot(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	now := time.Now()
	own := &VMOwnership{VMID: "vm-pending-active",
		UserID:       "user-pending",
		DesktopID:    PrimaryDesktopID,
		Kind:         VMKindInteractive,
		ComputerURL:  "http://127.0.0.1:9001",
		State:        VMStateActive,
		CreatedAt:    now.Add(-time.Hour),
		LastActiveAt: now.Add(-time.Minute),
		Epoch:        3}
	reg.mu.Lock()
	reg.ownerships[ownershipKey(own.UserID, own.DesktopID)] = own
	reg.vmByID[own.VMID] = own
	reg.mu.Unlock()

	mock := &mockVMManager{
		getVMs: map[string]*VMInstanceInfo{
			own.VMID: {
				HostURL:   "http://127.0.0.1:9010",
				Epoch:     own.Epoch,
				Healthy:   false,
				State:     "pending",
				StartedAt: now.Add(-30 * time.Second),
			},
		},
	}
	reg.SetVMManager(mock)

	resolved, err := reg.ResolveOrAssignDesktop("user-pending", PrimaryDesktopID)
	if err != nil {
		t.Fatalf("ResolveOrAssignDesktop: %v", err)
	}
	if len(mock.recovers) != 0 {
		t.Fatalf("recovers = %+v, want none for in-flight boot", mock.recovers)
	}
	if resolved.ComputerURL != "http://127.0.0.1:9010" {
		t.Fatalf("ComputerURL = %q, want current pending host URL", resolved.ComputerURL)
	}
	if resolved.Epoch != own.Epoch {
		t.Fatalf("Epoch = %d, want %d", resolved.Epoch, own.Epoch)
	}
}

func TestOwnershipRegistry_ResolveStartsActiveOwnershipMissingFromManager(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	now := time.Now().Add(-time.Minute)
	own := &VMOwnership{VMID: "vm-missing-manager-instance",
		UserID:       "user-old-account",
		DesktopID:    PrimaryDesktopID,
		Kind:         VMKindInteractive,
		ComputerURL:  "http://127.0.0.1:9001",
		State:        VMStateActive,
		CreatedAt:    now,
		LastActiveAt: now,
		Epoch:        3}
	reg.mu.Lock()
	reg.ownerships[ownershipKey(own.UserID, own.DesktopID)] = own
	reg.vmByID[own.VMID] = own
	reg.mu.Unlock()

	mock := &mockVMManager{
		resumeError: fmt.Errorf("vm not managed"),
		bootResponse: &VMInstanceInfo{
			HostURL: "http://127.0.0.1:9045",
			Epoch:   4,
			Healthy: true,
			State:   "running",
		},
	}
	reg.SetVMManager(mock)

	resolved, err := reg.ResolveOrAssignDesktop("user-old-account", PrimaryDesktopID)
	if err != nil {
		t.Fatalf("ResolveOrAssignDesktop: %v", err)
	}
	if resolved.VMID != own.VMID {
		t.Fatalf("resolved VMID = %q, want %q", resolved.VMID, own.VMID)
	}
	if len(mock.boots) != 1 {
		t.Fatalf("BootVM calls = %d, want 1", len(mock.boots))
	}
	if mock.boots[0].VMID != own.VMID {
		t.Fatalf("BootVM VMID = %q, want %q", mock.boots[0].VMID, own.VMID)
	}
	if resolved.ComputerURL != "http://127.0.0.1:9045" {
		t.Fatalf("ComputerURL = %q, want restarted host URL", resolved.ComputerURL)
	}
}

func TestOwnershipRegistry_ResolveRecoversFailedManagerInstanceForHibernatedDesktop(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	now := time.Now().Add(-time.Hour)
	own := &VMOwnership{VMID: "vm-failed-cold-resume",
		UserID:       "user-existing-account",
		DesktopID:    PrimaryDesktopID,
		Kind:         VMKindInteractive,
		ComputerURL:  "http://127.0.0.1:9001",
		State:        VMStateHibernated,
		CreatedAt:    now,
		LastActiveAt: now,
		Epoch:        7, StoppedBy: "idle"}
	reg.mu.Lock()
	reg.ownerships[ownershipKey(own.UserID, own.DesktopID)] = own
	reg.vmByID[own.VMID] = own
	reg.mu.Unlock()

	mock := &mockVMManager{
		resumeError: fmt.Errorf("vm vm-failed-cold-resume cannot be resumed (state=failed)"),
		getVMs: map[string]*VMInstanceInfo{
			own.VMID: {
				HostURL: own.ComputerURL,
				Epoch:   own.Epoch,
				Healthy: false,
				State:   "failed",
			},
		},
		recoverResponse: &VMInstanceInfo{
			HostURL: "http://127.0.0.1:9046",
			Epoch:   8,
			Healthy: true,
			State:   "running",
		},
	}
	reg.SetVMManager(mock)

	resolved, err := reg.ResolveOrAssignDesktop(own.UserID, PrimaryDesktopID)
	if err != nil {
		t.Fatalf("ResolveOrAssignDesktop: %v", err)
	}
	if len(mock.recovers) != 1 || mock.recovers[0] != own.VMID {
		t.Fatalf("recovers = %+v, want [%s]", mock.recovers, own.VMID)
	}
	if len(mock.boots) != 0 {
		t.Fatalf("expected recovery of failed manager instance, got %d boot calls", len(mock.boots))
	}
	if resolved.VMID != own.VMID {
		t.Fatalf("resolved VMID = %q, want %q", resolved.VMID, own.VMID)
	}
	if resolved.ComputerURL != "http://127.0.0.1:9046" {
		t.Fatalf("ComputerURL = %q, want recovered host URL", resolved.ComputerURL)
	}
	if resolved.Epoch != 8 {
		t.Fatalf("Epoch = %d, want 8", resolved.Epoch)
	}
	if resolved.State != VMStateActive || resolved.StoppedBy != "" {
		t.Fatalf("resolved state=%s stopped_by=%q, want active/empty", resolved.State, resolved.StoppedBy)
	}
}

func TestOwnershipRegistry_StartsFreshRealizationThroughVMManager(t *testing.T) {
	mock := &mockVMManager{
		recoverResponse: &VMInstanceInfo{
			HostURL: "http://127.0.0.1:9043",
			Epoch:   5,
			Healthy: true,
			State:   "running",
		},
		getVMs: make(map[string]*VMInstanceInfo),
	}
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetVMManager(mock)

	initial, _ := reg.ResolveOrAssign("user-resume-vm")
	mock.getVMs[initial.VMID] = &VMInstanceInfo{HostURL: initial.ComputerURL, Epoch: initial.Epoch, Healthy: true, State: "stopped"}
	_ = reg.HibernateVM("user-resume-vm")

	own, err := reg.ResumeVM("user-resume-vm")
	if err != nil {
		t.Fatalf("ResumeVM: %v", err)
	}
	if len(mock.recovers) != 1 {
		t.Fatalf("expected 1 fresh realization recovery call, got %d", len(mock.recovers))
	}
	if own.ComputerURL != "http://127.0.0.1:9043" || own.Epoch != 5 {
		t.Errorf("fresh realization = %+v", own)
	}
}

func TestOwnershipRegistry_LiveComputerURLSnapshotsDuringRefresh(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetCorpusdURL(testComputerCredentialIssuerURL(t))
	reg.SetVMManager(&mockVMManager{
		refreshResponse: &VMInstanceInfo{
			HostURL: "http://127.0.0.1:9045",
			Epoch:   100,
			Healthy: true,
			State:   "running",
		},
	})

	_, _ = reg.ResolveOrAssign("user-live-url-refresh")

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 200; i++ {
			if _, err := reg.LiveComputerURL("user-live-url-refresh", PrimaryDesktopID); err != nil {
				t.Errorf("LiveComputerURL: %v", err)
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 200; i++ {
			if _, err := reg.RefreshVMForDesktop("user-live-url-refresh", PrimaryDesktopID); err != nil {
				t.Errorf("RefreshVMForDesktop: %v", err)
				return
			}
		}
	}()

	close(start)
	wg.Wait()
}

func TestOwnershipRegistry_ResolveReturnSnapshotDuringRefresh(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetCorpusdURL(testComputerCredentialIssuerURL(t))
	reg.SetVMManager(&mockVMManager{
		refreshResponse: &VMInstanceInfo{
			HostURL: "http://127.0.0.1:9047",
			Epoch:   102,
			Healthy: true,
			State:   "running",
		},
	})

	own, err := reg.ResolveOrAssign("user-resolve-refresh")
	if err != nil {
		t.Fatalf("ResolveOrAssign: %v", err)
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 200; i++ {
			_ = own.ComputerURL
			_ = own.State
			_ = own.Epoch
		}
	}()

	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 200; i++ {
			if _, err := reg.RefreshVMForDesktop("user-resolve-refresh", PrimaryDesktopID); err != nil {
				t.Errorf("RefreshVMForDesktop: %v", err)
				return
			}
		}
	}()

	close(start)
	wg.Wait()
}

func TestOwnershipRegistry_RefreshAllowsHibernatedVM(t *testing.T) {
	mock := &mockVMManager{
		refreshResponse: &VMInstanceInfo{
			HostURL: "http://127.0.0.1:9046",
			Epoch:   101,
			Healthy: true,
			State:   "running",
		},
	}
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetCorpusdURL(testComputerCredentialIssuerURL(t))
	reg.SetVMManager(mock)
	initial, err := reg.ResolveOrAssign("user-refresh-hibernated")
	if err != nil {
		t.Fatalf("ResolveOrAssign: %v", err)
	}
	mock.getVMs = map[string]*VMInstanceInfo{
		initial.VMID: {
			HostURL: initial.ComputerURL,
			Epoch:   initial.Epoch,
			Healthy: true,
			State:   "running",
		},
	}
	if err := reg.HibernateVM("user-refresh-hibernated"); err != nil {
		t.Fatalf("HibernateVM: %v", err)
	}
	own, err := reg.RefreshVMForDesktop("user-refresh-hibernated", PrimaryDesktopID)
	if err != nil {
		t.Fatalf("RefreshVMForDesktop: %v", err)
	}
	if len(mock.refreshes) != 1 {
		t.Fatalf("expected 1 RefreshVM call, got %d", len(mock.refreshes))
	}
	if own.State != VMStateActive || own.StoppedBy != "" {
		t.Fatalf("refreshed ownership state=%s stopped_by=%q, want active/empty", own.State, own.StoppedBy)
	}
}

func TestOwnershipRegistry_ResolveCoalescesStoppedComputerStart(t *testing.T) {
	mgr := &mockVMManager{
		recoverResponse: &VMInstanceInfo{
			HostURL: "http://127.0.0.1:9048",
			Epoch:   103,
			Healthy: true,
			State:   "running",
		},
		recoverStarted: make(chan struct{}),
		recoverRelease: make(chan struct{}),
		getVMs:         make(map[string]*VMInstanceInfo),
	}
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetVMManager(mgr)
	initial, err := reg.ResolveOrAssign("user-coalesce-stopped")
	if err != nil {
		t.Fatalf("ResolveOrAssign: %v", err)
	}
	mgr.getVMs[initial.VMID] = &VMInstanceInfo{HostURL: initial.ComputerURL, Epoch: initial.Epoch, Healthy: true, State: "running"}
	if err := reg.StopVM("user-coalesce-stopped"); err != nil {
		t.Fatalf("StopVM: %v", err)
	}

	const callers = 8
	results := make(chan *VMOwnership, callers)
	errs := make(chan error, callers)
	var wg sync.WaitGroup
	wg.Add(callers)
	for range callers {
		go func() {
			defer wg.Done()
			own, err := reg.ResolveOrAssign("user-coalesce-stopped")
			if err != nil {
				errs <- err
				return
			}
			results <- own
		}()
	}

	select {
	case <-mgr.recoverStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first fresh realization start")
	}
	close(mgr.recoverRelease)
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		t.Fatalf("ResolveOrAssign concurrent stopped resume: %v", err)
	}
	for own := range results {
		if own.VMID != initial.VMID {
			t.Fatalf("VMID = %s, want existing %s", own.VMID, initial.VMID)
		}
		if own.State != VMStateActive {
			t.Fatalf("state = %s, want active", own.State)
		}
		if own.ComputerURL != "http://127.0.0.1:9048" {
			t.Fatalf("autoputer URL = %q, want resumed URL", own.ComputerURL)
		}
	}
	mgr.mu.Lock()
	recoverCount := len(mgr.recovers)
	mgr.mu.Unlock()
	if recoverCount != 1 {
		t.Fatalf("recover count = %d, want 1", recoverCount)
	}
}

func TestOwnershipRegistry_RefreshStoppedVMWithoutManagerInstanceBootsFromOwnership(t *testing.T) {
	mock := &mockVMManager{
		bootResponse: &VMInstanceInfo{
			HostURL: "http://127.0.0.1:9047",
			Epoch:   102,
			Healthy: true,
			State:   "running",
		},
	}
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetCorpusdURL(testComputerCredentialIssuerURL(t))
	reg.SetVMManager(mock)
	if _, err := reg.ResolveOrAssign("user-refresh-stopped-missing"); err != nil {
		t.Fatalf("ResolveOrAssign: %v", err)
	}
	if err := reg.LogoutVM("user-refresh-stopped-missing"); err != nil {
		t.Fatalf("LogoutVM: %v", err)
	}
	bootsBeforeRefresh := len(mock.boots)

	own, err := reg.RefreshVMForDesktop("user-refresh-stopped-missing", PrimaryDesktopID)
	if err != nil {
		t.Fatalf("RefreshVMForDesktop: %v", err)
	}
	if got := len(mock.boots) - bootsBeforeRefresh; got != 1 {
		t.Fatalf("expected 1 BootVM call for missing manager instance, got %d", got)
	}
	if len(mock.refreshes) != 0 {
		t.Fatalf("expected missing manager instance to avoid RefreshVM, got %d calls", len(mock.refreshes))
	}
	bootCfg := mock.boots[len(mock.boots)-1]
	if bootCfg.ComputerKind != "active" || bootCfg.OwnerID != "user-refresh-stopped-missing" || bootCfg.DesktopID != PrimaryDesktopID {
		t.Fatalf("boot config identity = %+v, want active ownership identity", bootCfg)
	}
	if own.State != VMStateActive || own.StoppedBy != "" {
		t.Fatalf("booted ownership state=%s stopped_by=%q, want active/empty", own.State, own.StoppedBy)
	}
	if own.ComputerURL != "http://127.0.0.1:9047" {
		t.Fatalf("autoputer URL = %s, want boot response URL", own.ComputerURL)
	}
}

func TestOwnershipRegistry_RefreshFailedPersistedVMWithoutManagerInstanceBootsRetainedComputer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ownership.json")
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	if err := reg.SetPersistencePath(path); err != nil {
		t.Fatalf("SetPersistencePath: %v", err)
	}
	reg.SetCorpusdURL(testComputerCredentialIssuerURL(t))
	reg.SetVMManager(&mockVMManager{})
	initial, err := reg.ResolveOrAssign("user-refresh-failed-missing")
	if err != nil {
		t.Fatalf("ResolveOrAssign: %v", err)
	}
	retainedVMID := initial.VMID
	reg.mu.Lock()
	reg.ownerships[ownershipKey(initial.UserID, initial.DesktopID)].State = VMStateFailed
	reg.saveLocked()
	reg.mu.Unlock()

	restarted := NewOwnershipRegistry("http://127.0.0.1:8085")
	if err := restarted.SetPersistencePath(path); err != nil {
		t.Fatalf("restart SetPersistencePath: %v", err)
	}
	loaded := restarted.GetOwnershipForDesktop(initial.UserID, initial.DesktopID)
	if loaded == nil || loaded.State != VMStateFailed || loaded.VMID != retainedVMID {
		t.Fatalf("reloaded ownership = %+v, want failed retained VM %s", loaded, retainedVMID)
	}
	mock := &mockVMManager{bootResponse: &VMInstanceInfo{
		HostURL: "http://127.0.0.1:9049", Epoch: loaded.Epoch + 1, Healthy: true, State: "running",
	}}
	restarted.SetCorpusdURL(testComputerCredentialIssuerURL(t))
	restarted.SetVMManager(mock)

	refreshed, err := restarted.RefreshVMForDesktop(initial.UserID, initial.DesktopID)
	if err != nil {
		t.Fatalf("RefreshVMForDesktop: %v", err)
	}
	if len(mock.boots) != 1 || len(mock.refreshes) != 0 {
		t.Fatalf("missing failed instance used boots=%d refreshes=%d, want 1/0", len(mock.boots), len(mock.refreshes))
	}
	if mock.boots[0].VMID != retainedVMID || refreshed.VMID != retainedVMID {
		t.Fatalf("refresh replaced retained VMID: boot=%s ownership=%s want=%s", mock.boots[0].VMID, refreshed.VMID, retainedVMID)
	}
	if mock.boots[0].ComputerID != loaded.ComputerID || mock.boots[0].ComputerCredentialEnvelope == "" {
		t.Fatalf("boot config lost retained identity or fresh credential: %+v", mock.boots[0])
	}
	if refreshed.State != VMStateActive || refreshed.ComputerURL != "http://127.0.0.1:9049" {
		t.Fatalf("refreshed ownership = %+v", refreshed)
	}
}

func TestOwnershipRegistry_BootFailureReturnsError(t *testing.T) {
	mock := &mockVMManager{
		bootError: fmt.Errorf("Firecracker process failed: KVM not available"),
	}
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetVMManager(mock)

	_, err := reg.ResolveOrAssign("user-boot-fail")
	if err == nil {
		t.Fatal("expected error when VM boot fails")
	}
	if !strings.Contains(err.Error(), "failed to boot VM") {
		t.Errorf("unexpected error message: %v", err)
	}

	// Verify ownership is marked as failed.
	own := reg.GetOwnership("user-boot-fail")
	if own == nil {
		t.Fatal("expected ownership to exist even after boot failure")
	}
	if own.State != VMStateFailed {
		t.Errorf("expected failed state, got %s", own.State)
	}
}

func TestOwnershipRegistry_StartOnResolveUsesFreshRealization(t *testing.T) {
	mock := &mockVMManager{
		recoverResponse: &VMInstanceInfo{
			HostURL: "http://127.0.0.1:9050",
			Epoch:   3,
			Healthy: true,
			State:   "running",
		},
		getVMs: make(map[string]*VMInstanceInfo),
	}
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetVMManager(mock)
	own1, _ := reg.ResolveOrAssign("user-resume-resolve")
	mock.getVMs[own1.VMID] = &VMInstanceInfo{HostURL: own1.ComputerURL, Epoch: own1.Epoch, Healthy: true, State: "running"}
	_ = reg.HibernateVM("user-resume-resolve")

	own2, err := reg.ResolveOrAssign("user-resume-resolve")
	if err != nil {
		t.Fatalf("ResolveOrAssign after hibernate: %v", err)
	}
	if own1.VMID != own2.VMID {
		t.Errorf("expected stable VM slot, got %s and %s", own1.VMID, own2.VMID)
	}
	if own2.ComputerURL != "http://127.0.0.1:9050" || own2.Epoch != 3 {
		t.Errorf("fresh realization = %+v", own2)
	}
	if len(mock.recovers) != 1 {
		t.Fatalf("expected 1 fresh realization call, got %d", len(mock.recovers))
	}
}

func TestOwnershipRegistry_PersistsOwnershipAndRebootsSameVMIDAfterRestart(t *testing.T) {
	path := t.TempDir() + "/ownerships.json"

	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	if err := reg.SetPersistencePath(path); err != nil {
		t.Fatalf("SetPersistencePath: %v", err)
	}

	own, err := reg.ResolveOrAssign("user-persist")
	if err != nil {
		t.Fatalf("ResolveOrAssign: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected ownership persistence file: %v", err)
	}

	restarted := NewOwnershipRegistry("http://127.0.0.1:8085")
	if err := restarted.SetPersistencePath(path); err != nil {
		t.Fatalf("restart SetPersistencePath: %v", err)
	}

	loaded := restarted.GetOwnership("user-persist")
	if loaded == nil {
		t.Fatal("expected persisted ownership after restart")
	}
	if loaded.VMID != own.VMID {
		t.Fatalf("loaded VMID = %s, want %s", loaded.VMID, own.VMID)
	}
	if loaded.State != VMStateStopped {
		t.Fatalf("loaded state = %s, want %s", loaded.State, VMStateStopped)
	}
	if loaded.StoppedBy != "vmctl-restart" {
		t.Fatalf("loaded StoppedBy = %q, want vmctl-restart", loaded.StoppedBy)
	}

	mock := &mockVMManager{
		reattachError: fmt.Errorf("vm process not available after process restart"),
		resumeError:   fmt.Errorf("vm not managed after process restart"),
		bootResponse: &VMInstanceInfo{
			HostURL: "http://127.0.0.1:9099",
			Epoch:   loaded.Epoch + 1,
			Healthy: true,
			State:   "running",
		},
	}
	restarted.SetVMManager(mock)
	restarted.ReattachManagedVMs(context.Background(), func(context.Context, string, string) error { return nil })

	resolved, err := restarted.ResolveOrAssign("user-persist")
	if err != nil {
		t.Fatalf("ResolveOrAssign after restart: %v", err)
	}
	if resolved.VMID != own.VMID {
		t.Fatalf("resolved VMID = %s, want persisted %s", resolved.VMID, own.VMID)
	}
	if resolved.ComputerURL != "http://127.0.0.1:9099" {
		t.Fatalf("resolved ComputerURL = %s, want manager boot URL", resolved.ComputerURL)
	}
	if resolved.State != VMStateActive {
		t.Fatalf("resolved state = %s, want active", resolved.State)
	}
	if len(mock.boots) != 1 {
		t.Fatalf("expected one boot fallback, got %d", len(mock.boots))
	}
	if mock.boots[0].VMID != own.VMID {
		t.Fatalf("boot VMID = %s, want persisted %s", mock.boots[0].VMID, own.VMID)
	}
}

func TestOwnershipRegistry_ReattachesPersistedVMWhenManagerCanAdopt(t *testing.T) {
	path := t.TempDir() + "/ownerships.json"

	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	if err := reg.SetPersistencePath(path); err != nil {
		t.Fatalf("SetPersistencePath: %v", err)
	}
	own, err := reg.ResolveOrAssign("user-reattach")
	if err != nil {
		t.Fatalf("ResolveOrAssign: %v", err)
	}

	restarted := NewOwnershipRegistry("http://127.0.0.1:8085")
	if err := restarted.SetPersistencePath(path); err != nil {
		t.Fatalf("restart SetPersistencePath: %v", err)
	}
	loaded := restarted.GetOwnership("user-reattach")
	if loaded == nil {
		t.Fatal("expected persisted ownership after restart")
	}
	if loaded.State != VMStateStopped || loaded.StoppedBy != "vmctl-restart" {
		t.Fatalf("loaded state = %s stopped_by=%q, want stopped/vmctl-restart", loaded.State, loaded.StoppedBy)
	}

	mock := &mockVMManager{}
	restarted.SetVMManager(mock)
	if got := restarted.ReattachManagedVMs(context.Background(), func(context.Context, string, string) error { return fmt.Errorf("route missing") }); got != 0 {
		t.Fatalf("reattached without D-ROUTE = %d, want 0", got)
	}
	if len(mock.reattaches) != 0 {
		t.Fatalf("route-refused reattach calls = %+v", mock.reattaches)
	}
	if got := restarted.ReattachManagedVMs(context.Background(), func(context.Context, string, string) error { return nil }); got != 1 {
		t.Fatalf("reattached count = %d, want 1", got)
	}

	reattached := restarted.GetOwnership("user-reattach")
	if reattached == nil {
		t.Fatal("expected ownership after reattach")
	}
	if reattached.VMID != own.VMID {
		t.Fatalf("reattached VMID = %s, want %s", reattached.VMID, own.VMID)
	}
	if reattached.State != VMStateActive {
		t.Fatalf("reattached state = %s, want active", reattached.State)
	}
	if reattached.StoppedBy != "" {
		t.Fatalf("reattached stopped_by = %q, want empty", reattached.StoppedBy)
	}
	if !reattached.LastActiveAt.Equal(own.LastActiveAt) {
		t.Fatalf("reattach changed LastActiveAt = %s, want preserved %s", reattached.LastActiveAt, own.LastActiveAt)
	}
	if len(mock.reattaches) != 1 || mock.reattaches[0] != own.VMID {
		t.Fatalf("reattaches = %+v, want [%s]", mock.reattaches, own.VMID)
	}
	if len(mock.boots) != 0 {
		t.Fatalf("reattach should not boot, got %d boot calls", len(mock.boots))
	}
}

func TestOwnershipRegistry_ReattachReconcilesGatewayCredential(t *testing.T) {
	ensured := make(chan string, 1)
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/provider/v1/credentials/ensure" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("ensure method = %s, want POST", r.Method)
		}
		if r.Header.Get("X-Internal-Caller") != "true" {
			t.Errorf("missing internal caller header")
		}
		var req struct {
			RawToken string `json:"raw_token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode ensure request: %v", err)
		}
		ensured <- req.RawToken
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"computer_id":"vm-reattach-old-token","status":"imported"}`))
	}))
	t.Cleanup(gateway.Close)

	rawToken := "vm-reattach-old-token:token-from-host-persistent-dir"
	now := time.Now()
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetGatewayURL(gateway.URL)
	own := &VMOwnership{VMID: "vm-reattach-old-token",
		UserID:       "user-old-account",
		DesktopID:    PrimaryDesktopID,
		Kind:         VMKindInteractive,
		ComputerURL:  "http://127.0.0.1:9001",
		State:        VMStateStopped,
		CreatedAt:    now,
		LastActiveAt: now,
		Epoch:        3, StoppedBy: "vmctl-restart"}
	reg.mu.Lock()
	reg.ownerships[ownershipKey(own.UserID, own.DesktopID)] = own
	reg.vmByID[own.VMID] = own
	reg.mu.Unlock()

	reg.SetVMManager(&mockVMManager{
		tokens: map[string]string{
			own.VMID: rawToken,
		},
	})
	reg.ReattachManagedVMs(context.Background(), func(context.Context, string, string) error { return nil })

	select {
	case got := <-ensured:
		if got != rawToken {
			t.Fatalf("ensured raw token = %q, want %q", got, rawToken)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for reattached VM gateway credential reconciliation")
	}
}

func TestIssueGatewayToken_LegacyJSONShapeStillWorks(t *testing.T) {
	credValue := "vm-test-legacy:changedplaceholder"
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Internal-Caller"); got != "true" {
			t.Errorf("expected X-Internal-Caller=true, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]string{
			"computer_id": "vm-test-legacy",
			"raw_token":   credValue,
			"expires_at":  "2025-01-01T00:00:00Z",
		}
		jsonData, _ := json.Marshal(resp)
		w.Write(jsonData)
	}))
	defer gateway.Close()

	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetGatewayURL(gateway.URL)

	token := reg.issueGatewayToken("vm-test-legacy")
	if token != credValue {
		t.Errorf("expected credential value %q, got %q", credValue, token)
	}
}

func TestIssueGatewayToken_NoGatewayURL(t *testing.T) {
	// When no gateway URL is configured, issueGatewayToken returns empty string.
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	// Don't call SetGatewayURL

	token := reg.issueGatewayToken("vm-test-123")
	if token != "" {
		t.Errorf("expected empty token when no gateway URL, got %q", token)
	}
}

func allowLifecycleRoute(context.Context, string, string) error { return nil }

func TestOwnershipRegistry_ActuatorPersistsAcrossUnflaggedRefresh(t *testing.T) {
	mock := &mockVMManager{
		refreshResponse: &VMInstanceInfo{HostURL: "http://127.0.0.1:9045", Epoch: 100, Healthy: true, State: "running"},
	}
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetCorpusdURL(testComputerCredentialIssuerURL(t))
	reg.SetVMManager(mock)
	_, _ = reg.ResolveOrAssign("user-actuator")
	if err := reg.SetActuatorForDesktop("user-actuator", PrimaryDesktopID, "rlm"); err != nil {
		t.Fatal(err)
	}
	own, err := reg.RefreshVMForDesktop("user-actuator", PrimaryDesktopID)
	if err != nil {
		t.Fatal(err)
	}
	if own.Actuator != "rlm" {
		t.Fatalf("ownership actuator = %q, want rlm", own.Actuator)
	}
	if len(mock.refreshCfgs) != 1 {
		t.Fatalf("expected 1 refresh cfg, got %d", len(mock.refreshCfgs))
	}
	if mock.refreshCfgs[0].Actuator != "rlm" {
		t.Fatalf("refresh cfg actuator = %q, want rlm after omit", mock.refreshCfgs[0].Actuator)
	}
	if err := reg.SetActuatorForDesktop("user-actuator", PrimaryDesktopID, "tools"); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.RefreshVMForDesktop("user-actuator", PrimaryDesktopID); err != nil {
		t.Fatal(err)
	}
	if mock.refreshCfgs[1].Actuator != "tools" {
		t.Fatalf("explicit tools refresh cfg = %q", mock.refreshCfgs[1].Actuator)
	}
}
