package vmctl

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testComputerCredentialIssuerURL(t *testing.T) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]string
		if r.URL.Path != "/internal/computers/credentials/issue" || r.Header.Get("X-Internal-Caller") != "true" {
			t.Errorf("credential request = %s %s headers=%v", r.Method, r.URL.Path, r.Header)
			http.Error(w, "unexpected credential request", http.StatusBadRequest)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode credential request: %v", err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"envelope": map[string]any{
				"schema_version": 1,
				"computer_id":    request["computer_id"],
				"realization_id": request["realization_id"],
			},
		})
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func TestStartExistingVMBindsCredentialToStableComputerAndRealization(t *testing.T) {
	var issued map[string]string
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/computers/credentials/issue" || r.Header.Get("X-Internal-Caller") != "true" {
			t.Fatalf("credential request = %s %s headers=%v", r.Method, r.URL.Path, r.Header)
		}
		if err := json.NewDecoder(r.Body).Decode(&issued); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"envelope":{"schema_version":1,"computer_id":"computer-stable","realization_id":"vm-realization"}}`))
	}))
	defer corpusd.Close()

	registry := NewOwnershipRegistry("")
	registry.SetCorpusdURL(corpusd.URL)
	manager := &mockVMManager{resumeError: errors.New("not running")}
	ownership := &VMOwnership{VMID: "vm-realization", ComputerID: "computer-stable", DesktopID: "primary", UserID: "owner-1", Epoch: 7}
	expectedComputerID := ownership.ComputerID
	if got := stableComputerID(ownership.UserID, ownership.DesktopID, ownership.ComputerID); got != expectedComputerID {
		t.Fatalf("stable computer identity = %q, want %q", got, expectedComputerID)
	}
	if _, err := registry.startExistingVM(ownership, manager); err != nil {
		t.Fatal(err)
	}
	if issued["computer_id"] != expectedComputerID || issued["realization_id"] != "vm-realization-epoch-8" || issued["idempotency_key"] != "guest-credential:vm-realization-epoch-8:8" {
		t.Fatalf("issued credential identity = %#v", issued)
	}
	if len(manager.boots) != 1 || manager.boots[0].ComputerCredentialEnvelope == "" || manager.boots[0].DesktopID != ownership.DesktopID {
		t.Fatalf("boot config = %#v", manager.boots)
	}
}

func TestRefreshBindsFreshCredentialWithoutBlockingRegistry(t *testing.T) {
	manager := &mockVMManager{
		refreshStarted: make(chan struct{}),
		refreshRelease: make(chan struct{}),
	}
	registry := NewOwnershipRegistry("")
	registry.SetCorpusdURL(testComputerCredentialIssuerURL(t))
	registry.SetVMManager(manager)
	initial, err := registry.ResolveOrAssign("owner-refresh")
	if err != nil {
		t.Fatal(err)
	}
	initialEpoch := initial.Epoch

	refreshDone := make(chan error, 1)
	go func() {
		_, refreshErr := registry.RefreshVMForDesktop("owner-refresh", PrimaryDesktopID)
		refreshDone <- refreshErr
	}()
	select {
	case <-manager.refreshStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for refresh to reach VM manager")
	}

	listDone := make(chan struct{})
	go func() {
		_ = registry.ListOwnerships()
		close(listDone)
	}()
	select {
	case <-listDone:
	case <-time.After(time.Second):
		t.Fatal("ownership registry remained locked during VM refresh")
	}
	if _, err := registry.RefreshVMForDesktop("owner-refresh", PrimaryDesktopID); err == nil || !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("concurrent refresh error = %v, want already in progress", err)
	}
	if err := registry.StopVM("owner-refresh"); err == nil || !strings.Contains(err.Error(), "refresh is already in progress") {
		t.Fatalf("concurrent stop error = %v, want refresh conflict", err)
	}

	close(manager.refreshRelease)
	if err := <-refreshDone; err != nil {
		t.Fatal(err)
	}
	manager.mu.Lock()
	if len(manager.refreshCfgs) != 1 {
		manager.mu.Unlock()
		t.Fatalf("refresh configs = %d, want 1", len(manager.refreshCfgs))
	}
	cfg := manager.refreshCfgs[0]
	manager.mu.Unlock()
	if cfg.Epoch <= initialEpoch || cfg.RealizationID != realizationIDFor(initial.VMID, cfg.Epoch) {
		t.Fatalf("refresh identity = epoch %d realization %q, want a later reserved epoch bound to the realization", cfg.Epoch, cfg.RealizationID)
	}
	if cfg.ComputerID != initial.ComputerID {
		t.Fatalf("refresh computer identity = %q, want stable %q", cfg.ComputerID, initial.ComputerID)
	}
	if cfg.ComputerCredentialEnvelope == "" {
		t.Fatal("refresh omitted the per-realization computer credential envelope")
	}
}

func TestRefreshManagerFailureProjectsFailedOwnershipAndAdvancesRetryIdentity(t *testing.T) {
	var issuedRealizations []string
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]string
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		issuedRealizations = append(issuedRealizations, request["realization_id"])
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"envelope":{"schema_version":1}}`))
	}))
	defer corpusd.Close()

	bootFailure := errors.New("boot failed after process replacement")
	manager := &mockVMManager{refreshError: bootFailure}
	registry := NewOwnershipRegistry("")
	registry.SetPersistencePath(filepath.Join(t.TempDir(), "ownership.json"))
	registry.SetCorpusdURL(corpusd.URL)
	registry.SetVMManager(manager)
	initial, err := registry.ResolveOrAssign("owner-refresh-failure")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := registry.RefreshVMForDesktop("owner-refresh-failure", PrimaryDesktopID); err == nil || !strings.Contains(err.Error(), bootFailure.Error()) {
		t.Fatalf("refresh error = %v, want manager failure", err)
	}
	current := registry.GetOwnership("owner-refresh-failure")
	if current == nil || current.VMID != initial.VMID || current.State != VMStateFailed {
		t.Fatalf("ownership after refresh failure = %+v, want same VM in failed state", current)
	}

	restarted := NewOwnershipRegistry("")
	restarted.SetCorpusdURL(corpusd.URL)
	restarted.SetVMManager(&mockVMManager{bootError: bootFailure})
	if err := restarted.SetPersistencePath(registry.persistencePath); err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.RefreshVMForDesktop("owner-refresh-failure", PrimaryDesktopID); err == nil || !strings.Contains(err.Error(), bootFailure.Error()) {
		t.Fatalf("restart retry error = %v, want manager failure", err)
	}
	if len(issuedRealizations) < 2 {
		t.Fatalf("credential issuances = %v, want failed attempt and retry", issuedRealizations)
	}
	previous, retry := issuedRealizations[len(issuedRealizations)-2], issuedRealizations[len(issuedRealizations)-1]
	if retry == previous {
		t.Fatalf("retry reused credential realization %q after failed boot", retry)
	}
}

func TestRefreshRefusesCredentialIssuanceWhenEpochReservationCannotPersist(t *testing.T) {
	issuanceCalls := 0
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		issuanceCalls++
		w.WriteHeader(http.StatusCreated)
	}))
	defer corpusd.Close()

	registry := NewOwnershipRegistry("")
	registry.SetCorpusdURL(corpusd.URL)
	registry.SetVMManager(&mockVMManager{})
	own := &VMOwnership{
		UserID:     "owner-persist-failure",
		DesktopID:  PrimaryDesktopID,
		VMID:       "vm-persist-failure",
		ComputerID: "computer-persist-failure",
		State:      VMStateFailed,
		Epoch:      804,
	}
	registry.mu.Lock()
	registry.ownerships[ownershipKey(own.UserID, own.DesktopID)] = own
	registry.vmByID[own.VMID] = own
	registry.persistencePath = t.TempDir()
	registry.mu.Unlock()

	_, err := registry.RefreshVMForDesktop(own.UserID, own.DesktopID)
	if err == nil || !strings.Contains(err.Error(), "persist reserved VM realization") {
		t.Fatalf("refresh error = %v, want durable reservation refusal", err)
	}
	if issuanceCalls != 0 {
		t.Fatalf("credential issuance calls = %d, want 0 before durable reservation", issuanceCalls)
	}
}
