package vmctl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestComputerKindForOwnershipPlatform(t *testing.T) {
	own := &VMOwnership{
		UserID:        UniversalWirePlatformOwnerID,
		DesktopID:     UniversalWirePlatformDesktopID,
		WarmnessClass: WarmnessClassPublicPlatform,
	}
	if got := computerKindForOwnership(own); got != "platform" {
		t.Fatalf("computerKindForOwnership() = %q, want platform", got)
	}
}

func TestWarmnessClassProtectedIncludesPublicPlatform(t *testing.T) {
	if !warmnessClassProtected(WarmnessClassPublicPlatform) {
		t.Fatal("expected public_platform warmness class to be protected from idle reclaim")
	}
}

func TestWarmUniversalWirePlatformComputerPersistsResumeFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ownership.json")
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	if err := reg.SetPersistencePath(path); err != nil {
		t.Fatalf("SetPersistencePath: %v", err)
	}
	reg.SetVMManager(&mockVMManager{resumeError: errors.New("missing managed instance")})
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

func TestHandleResolveEnsuresUniversalWirePlatformComputer(t *testing.T) {
	mgr := &mockVMManager{
		bootResponse: &VMInstanceInfo{
			HostURL: "http://10.203.141.2:8085",
			Epoch:   4,
			Healthy: true,
			State:   "running",
		},
	}
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetVMManager(mgr)
	handler := NewHandler(reg)

	body := bytes.NewBufferString(`{"user_id":"universal-wire-platform","desktop_id":"platform"}`)
	req := httptest.NewRequest(http.MethodPost, "/internal/vmctl/resolve", body)
	req.Header.Set("X-Internal-Caller", "true")
	rec := httptest.NewRecorder()
	handler.HandleResolve(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("resolve status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp resolveResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode resolve response: %v", err)
	}
	if resp.UserID != UniversalWirePlatformOwnerID || resp.DesktopID != UniversalWirePlatformDesktopID {
		t.Fatalf("resolve identity = (%q, %q), want platform computer", resp.UserID, resp.DesktopID)
	}
	if resp.SandboxURL != "http://10.203.141.2:8085" || resp.State != string(VMStateActive) {
		t.Fatalf("resolve response = %+v, want active platform sandbox", resp)
	}
	if len(mgr.boots) != 1 || mgr.boots[0].VMID != UniversalWirePlatformVMID {
		t.Fatalf("platform boot calls = %#v, want stable platform VM", mgr.boots)
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
				State:   "pending",
			},
		},
		recoverResponse: &VMInstanceInfo{
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
		WarmnessClass: WarmnessClassPublicPlatform, SandboxURL: "http://10.200.17.2:8085",
		State: VMStateBooting,
		Epoch: 58}
	reg.vmByID[UniversalWirePlatformVMID] = reg.ownerships[key]

	err := reg.EnsureUniversalWirePlatformComputer(t.Context())
	if err != nil {
		t.Fatalf("EnsureUniversalWirePlatformComputer: %v", err)
	}
	if len(mgr.recovers) != 1 || mgr.recovers[0] != UniversalWirePlatformVMID {
		t.Fatalf("expected recovery for %s, got %#v", UniversalWirePlatformVMID, mgr.recovers)
	}
	if len(mgr.recoverCfgs) != 1 || mgr.recoverCfgs[0].ComputerKind != "platform" {
		t.Fatalf("expected platform recovery config, got %#v", mgr.recoverCfgs)
	}
	own := reg.ownerships[key]
	if own.State != VMStateActive {
		t.Fatalf("state = %s, want active", own.State)
	}
	if own.SandboxURL != "http://10.200.99.2:8085" {
		t.Fatalf("sandbox URL = %q, want recovered URL", own.SandboxURL)
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
			State:   "pending",
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
		WarmnessClass: WarmnessClassPublicPlatform, SandboxURL: "http://10.200.17.2:8085",
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
	if own.State != VMStateActive || own.SandboxURL != "http://10.200.99.2:8085" {
		t.Fatalf("ownership after coalesced recovery = state %s url %q", own.State, own.SandboxURL)
	}
}

func TestSandboxProxyEnsuresUniversalWirePlatformBeforeProxying(t *testing.T) {
	runtime := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/runtime/runs" {
			t.Fatalf("proxied path = %q, want /internal/runtime/runs", r.URL.Path)
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"run_id":"run-platform"}`))
	}))
	defer runtime.Close()

	mgr := &mockVMManager{
		bootResponse: &VMInstanceInfo{
			HostURL: runtime.URL,
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
		WarmnessClass: WarmnessClassPublicPlatform, SandboxURL: "http://10.200.17.2:8085",
		State: VMStateBooting,
		Epoch: 58}
	reg.vmByID[UniversalWirePlatformVMID] = reg.ownerships[key]

	req := httptest.NewRequest(
		http.MethodPost,
		"/internal/vmctl/sandbox-proxy/universal-wire-platform/internal/runtime/runs",
		strings.NewReader(`{"objective":"process source"}`),
	)
	req.Header.Set("X-Internal-Caller", "true")
	rec := httptest.NewRecorder()

	NewHandler(reg).HandleSandboxProxy(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if len(mgr.boots) != 1 || mgr.boots[0].VMID != UniversalWirePlatformVMID {
		t.Fatalf("expected fresh platform boot before proxy, got %#v", mgr.boots)
	}
	own := reg.ownerships[key]
	if own.State != VMStateActive || own.SandboxURL != runtime.URL {
		t.Fatalf("ownership after proxy = state %s url %q", own.State, own.SandboxURL)
	}
}

func TestSandboxProxyForwardsInternalRuntimeStatusGET(t *testing.T) {
	runtime := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("proxied method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/internal/runtime/runs/run-status" {
			t.Fatalf("proxied path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("owner_id"); got != UniversalWirePlatformOwnerID {
			t.Fatalf("owner_id = %q, want %q", got, UniversalWirePlatformOwnerID)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"run_id":"run-status","state":"completed"}`))
	}))
	defer runtime.Close()

	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	key := ownershipKey(UniversalWirePlatformOwnerID, UniversalWirePlatformDesktopID)
	reg.ownerships[key] = &VMOwnership{VMID: UniversalWirePlatformVMID,
		UserID:    UniversalWirePlatformOwnerID,
		DesktopID: UniversalWirePlatformDesktopID,
		Kind:      VMKindInteractive, SandboxURL: runtime.URL,
		State: VMStateActive,
		Epoch: 60}
	reg.vmByID[UniversalWirePlatformVMID] = reg.ownerships[key]

	req := httptest.NewRequest(
		http.MethodGet,
		"/internal/vmctl/sandbox-proxy/universal-wire-platform/internal/runtime/runs/run-status?owner_id=universal-wire-platform",
		nil,
	)
	req.Header.Set("X-Internal-Caller", "true")
	rec := httptest.NewRecorder()

	NewHandler(reg).HandleSandboxProxy(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestSandboxProxyPlatformEnsureFailureReturnsBoundedError(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	key := ownershipKey(UniversalWirePlatformOwnerID, UniversalWirePlatformDesktopID)
	reg.ownerships[key] = &VMOwnership{VMID: UniversalWirePlatformVMID,
		UserID:        UniversalWirePlatformOwnerID,
		DesktopID:     UniversalWirePlatformDesktopID,
		Kind:          VMKindInteractive,
		WarmnessClass: WarmnessClassPublicPlatform, SandboxURL: "http://10.200.17.2:8085",
		State: VMStateBooting,
		Epoch: 58}
	reg.vmByID[UniversalWirePlatformVMID] = reg.ownerships[key]

	req := httptest.NewRequest(
		http.MethodPost,
		"/internal/vmctl/sandbox-proxy/universal-wire-platform/internal/runtime/runs",
		strings.NewReader(`{"objective":"process source"}`),
	)
	req.Header.Set("X-Internal-Caller", "true")
	rec := httptest.NewRecorder()

	NewHandler(reg).HandleSandboxProxy(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); !strings.Contains(got, "platform sandbox is not ready") {
		t.Fatalf("response body = %s, want bounded platform-not-ready error", got)
	}
	if got := rec.Body.String(); strings.Contains(got, UniversalWirePlatformVMID) || strings.Contains(got, UniversalWirePlatformOwnerID) || strings.Contains(got, "10.200.17.2") {
		t.Fatalf("response body leaked platform details: %s", got)
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
	<-m.release
	m.mu.Lock()
	m.info = m.recovered
	m.mu.Unlock()
	return m.recovered, nil
}

func (m *blockingPlatformRecoverManager) RefreshVM(string, VMManagerConfig) (*VMInstanceInfo, error) {
	return nil, errors.New("refresh should not be called")
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
