package vmctl

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// S3 snapshot hibernate, vmctl side
// (docs/archive/reports/s3-snapshot-hibernate-design-2026-10-14.md).
//
// Ways this could fail, written first:
//   - vmctl never asks the manager for a snapshot resume, so a hibernated
//     computer always cold boots (the feature is dead code);
//   - a snapshot resume is followed by a cold boot anyway (two
//     realizations), or advances the epoch so the woken guest's credentials
//     no longer match vmctl's record;
//   - a failed snapshot resume leaves the computer down instead of cold
//     booting it;
//   - a held computer is woken from its snapshot, bypassing the maintenance
//     hold the snapshot was not taken under;
//   - the seconds-long snapshot write runs under the registry lock, so every
//     other computer's lookup and resolve stalls behind one hibernate;
//   - the operator endpoint is reachable by a guest, or its receipt is not
//     readable, so staging cannot prove which path a hibernate took.
//
// Re-credential on resume (design note items 26-28):
//   - the resume credential is minted for a new realization or epoch, so the
//     resumed guest (which keeps its realization) refuses it;
//   - it reuses the boot idempotency key, which corpusd replays as an
//     expired issuance and refuses;
//   - two resume attempts share one key, so the second replays the first
//     (already consumed) envelope;
//   - an explicit refusal is retried, or reported as success;
//   - an issuer is offered when vmctl has no corpusd to ask.

type snapshotMockVMManager struct {
	*mockVMManager
	snapMu          sync.Mutex
	snapshotCalls   []VMManagerConfig
	snapshotResp    *VMInstanceInfo
	snapshotErr     error
	hibernateHook   func(vmID string)
	hibernateRecord json.RawMessage
	// callIssuer makes the mock mint the resume credential the way the
	// manager does after validating a snapshot.
	callIssuer bool
	issuers    []func() (string, error)
	issued     []string
	issueErrs  []error
}

func (m *snapshotMockVMManager) HibernateVM(vmID string) error {
	if m.hibernateHook != nil {
		m.hibernateHook(vmID)
	}
	return m.mockVMManager.HibernateVM(vmID)
}

func (m *snapshotMockVMManager) ResumeVMFromSnapshot(vmID string, cfg VMManagerConfig, issue func() (string, error)) (*VMInstanceInfo, error) {
	m.snapMu.Lock()
	m.snapshotCalls = append(m.snapshotCalls, cfg)
	m.issuers = append(m.issuers, issue)
	m.snapMu.Unlock()
	if m.callIssuer && issue != nil {
		envelope, err := issue()
		m.snapMu.Lock()
		m.issued = append(m.issued, envelope)
		m.issueErrs = append(m.issueErrs, err)
		m.snapMu.Unlock()
	}
	if m.snapshotErr != nil {
		return nil, m.snapshotErr
	}
	return m.snapshotResp, nil
}

func (m *snapshotMockVMManager) HibernateTimeline(vmID string) (json.RawMessage, error) {
	if m.hibernateRecord == nil {
		return nil, errors.New("no hibernate timeline")
	}
	return m.hibernateRecord, nil
}

func newSnapshotTestRegistry(t *testing.T, mock *snapshotMockVMManager, userID string) (*OwnershipRegistry, *VMOwnership) {
	t.Helper()
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetVMManager(mock)
	initial, err := reg.ResolveOrAssign(userID)
	if err != nil {
		t.Fatalf("ResolveOrAssign: %v", err)
	}
	mock.getVMs[initial.VMID] = &VMInstanceInfo{HostURL: initial.ComputerURL, Epoch: initial.Epoch, Healthy: true, State: "running"}
	if err := reg.HibernateVM(userID); err != nil {
		t.Fatalf("HibernateVM: %v", err)
	}
	mock.getVMs[initial.VMID].State = "hibernated"
	return reg, initial
}

func TestSnapshotResumeWakesSameRealizationWithoutColdBoot(t *testing.T) {
	mock := &snapshotMockVMManager{mockVMManager: &mockVMManager{
		recoverResponse: &VMInstanceInfo{HostURL: "http://127.0.0.1:9043", Epoch: 99, Healthy: true, State: "running"},
		getVMs:          make(map[string]*VMInstanceInfo),
	}}
	reg, initial := newSnapshotTestRegistry(t, mock, "user-snap")
	mock.snapshotResp = &VMInstanceInfo{HostURL: initial.ComputerURL, Epoch: initial.Epoch, Healthy: true, State: "running"}
	bootsBefore := len(mock.boots)

	own, err := reg.ResumeVM("user-snap")
	if err != nil {
		t.Fatalf("ResumeVM: %v", err)
	}
	if len(mock.snapshotCalls) != 1 {
		t.Fatalf("snapshot resume calls = %d, want 1", len(mock.snapshotCalls))
	}
	if len(mock.recovers) != 0 || len(mock.boots) != bootsBefore {
		t.Fatalf("snapshot resume was followed by a cold start (recovers=%d boots=%d)", len(mock.recovers), len(mock.boots)-bootsBefore)
	}
	if own.State != VMStateActive || own.Epoch != initial.Epoch {
		t.Fatalf("resumed ownership state=%s epoch=%d, want active epoch %d (same realization)", own.State, own.Epoch, initial.Epoch)
	}
	cfg := mock.snapshotCalls[0]
	if cfg.VMID != initial.VMID || cfg.Epoch != initial.Epoch || cfg.ComputerID == "" || cfg.MaintenanceHold || cfg.GatewayToken != "" {
		t.Fatalf("snapshot resume expectation = %+v", cfg)
	}
}

func TestSnapshotResumeFailureColdBootsWithFreshEpoch(t *testing.T) {
	mock := &snapshotMockVMManager{mockVMManager: &mockVMManager{
		refreshResponse: &VMInstanceInfo{HostURL: "http://127.0.0.1:9043", Epoch: 7, Healthy: true, State: "running"},
		getVMs:          make(map[string]*VMInstanceInfo),
	}}
	reg, _ := newSnapshotTestRegistry(t, mock, "user-snap-fail")
	mock.snapshotErr = errors.New("snapshot fallback: disk changed")

	own, err := reg.ResumeVM("user-snap-fail")
	if err != nil {
		t.Fatalf("ResumeVM after a failed snapshot resume: %v", err)
	}
	// The cold boot of the tracked hibernated instance is on the current
	// deploy (RefreshVM), not its stored config.
	if len(mock.snapshotCalls) != 1 || len(mock.refreshes) != 1 || len(mock.recovers) != 0 {
		t.Fatalf("snapshot calls=%d refreshes=%d recovers=%d, want 1, 1, 0", len(mock.snapshotCalls), len(mock.refreshes), len(mock.recovers))
	}
	if own.State != VMStateActive || own.Epoch != 7 {
		t.Fatalf("cold boot ownership state=%s epoch=%d, want active epoch 7", own.State, own.Epoch)
	}
}

func TestSnapshotResumeSkipsHeldComputer(t *testing.T) {
	mock := &snapshotMockVMManager{mockVMManager: &mockVMManager{getVMs: make(map[string]*VMInstanceInfo)}}
	reg, initial := newSnapshotTestRegistry(t, mock, "user-snap-held")
	held := cloneOwnership(reg.GetOwnershipForDesktop("user-snap-held", PrimaryDesktopID))
	held.HoldStatus = &MaintenanceHold{}
	if _, ok := reg.resumeFromSnapshot(held, mock); ok || len(mock.snapshotCalls) != 0 {
		t.Fatalf("held computer %s was offered to the snapshot resume", initial.VMID)
	}
	active := cloneOwnership(held)
	active.HoldStatus = nil
	active.State = VMStateActive
	if _, ok := reg.resumeFromSnapshot(active, mock); ok || len(mock.snapshotCalls) != 0 {
		t.Fatal("a computer vmctl did not hibernate was offered to the snapshot resume")
	}
}

func TestSnapshotHibernateRunsOutsideRegistryLock(t *testing.T) {
	mock := &snapshotMockVMManager{mockVMManager: &mockVMManager{getVMs: make(map[string]*VMInstanceInfo)}}
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetVMManager(mock)
	if _, err := reg.ResolveOrAssign("user-hib-slow"); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.ResolveOrAssign("user-other"); err != nil {
		t.Fatal(err)
	}
	var sawOther, sawRefreshing bool
	mock.hibernateHook = func(string) {
		done := make(chan struct{})
		go func() {
			sawOther = reg.GetOwnershipForDesktop("user-other", PrimaryDesktopID) != nil
			if err := reg.hibernateVMForDesktopWithReason("user-hib-slow", PrimaryDesktopID, "idle"); err != nil {
				sawRefreshing = true
			}
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("registry lock held across the manager hibernate")
		}
	}
	if err := reg.HibernateVM("user-hib-slow"); err != nil {
		t.Fatalf("HibernateVM: %v", err)
	}
	if !sawOther {
		t.Fatal("other computer's lookup failed during a hibernate")
	}
	if !sawRefreshing {
		t.Fatal("a second hibernate of the same computer was admitted while the first ran")
	}
	if got := reg.GetOwnershipForDesktop("user-hib-slow", PrimaryDesktopID); got == nil || got.State != VMStateHibernated || got.StoppedBy != "idle" {
		t.Fatalf("after hibernate: %+v", got)
	}
}

func TestHibernateEndpointAndHibernateTimeline(t *testing.T) {
	mock := &snapshotMockVMManager{
		mockVMManager:   &mockVMManager{getVMs: make(map[string]*VMInstanceInfo)},
		hibernateRecord: json.RawMessage(`{"kind":"vm_boot_timeline","boot_kind":"hibernate","snapshot":{"outcome":"snapshot"}}`),
	}
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetVMManager(mock)
	own, err := reg.ResolveOrAssign("user-hib-endpoint")
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(reg)

	post := func(remote, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/internal/vmctl/hibernate", bytes.NewBufferString(body))
		req.RemoteAddr = remote
		rec := httptest.NewRecorder()
		h.HandleHibernate(rec, req)
		return rec
	}
	if rec := post("203.0.113.9:4000", `{"computer_id":"`+own.ComputerID+`"}`); rec.Code != http.StatusForbidden {
		t.Fatalf("non-host caller: %d, want 403", rec.Code)
	}
	if rec := post("192.0.2.1:4000", `{"computer_id":"computer-missing"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown computer: %d, want 404", rec.Code)
	}
	rec := post("192.0.2.1:4000", `{"computer_id":"`+own.ComputerID+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("hibernate: %d %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Status    string          `json:"status"`
		VMID      string          `json:"vm_id"`
		Timeline  json.RawMessage `json:"hibernate_timeline"`
		ComputerI string          `json:"computer_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Status != "hibernated" || resp.VMID != own.VMID || len(resp.Timeline) == 0 {
		t.Fatalf("hibernate response = %s", rec.Body.String())
	}
	if got := reg.GetOwnershipByComputerID(own.ComputerID); got == nil || got.State != VMStateHibernated || got.StoppedBy != "operator" {
		t.Fatalf("after operator hibernate: %+v", got)
	}

	get := func(query string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/internal/vmctl/boot-timeline?"+query, nil)
		rec := httptest.NewRecorder()
		h.HandleBootTimeline(rec, req)
		return rec
	}
	if rec := get("computer_id=" + own.ComputerID + "&kind=hibernate"); rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"outcome":"snapshot"`)) {
		t.Fatalf("kind=hibernate: %d %s", rec.Code, rec.Body.String())
	}
	if rec := get("computer_id=" + own.ComputerID + "&kind=bogus"); rec.Code != http.StatusBadRequest {
		t.Fatalf("kind=bogus: %d, want 400", rec.Code)
	}
}
