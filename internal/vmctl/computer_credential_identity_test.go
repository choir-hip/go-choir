package vmctl

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
	if cfg.Epoch != initialEpoch+1 || cfg.RealizationID != realizationIDFor(initial.VMID, initialEpoch+1) {
		t.Fatalf("refresh identity = epoch %d realization %q, want epoch %d realization %q", cfg.Epoch, cfg.RealizationID, initialEpoch+1, realizationIDFor(initial.VMID, initialEpoch+1))
	}
	if cfg.ComputerCredentialEnvelope == "" {
		t.Fatal("refresh omitted the per-realization computer credential envelope")
	}
}
