package vmctl

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

func TestOwnershipRegistry_ResolveOrAssignCreatesVM(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	own, err := reg.ResolveOrAssign("user-1")
	if err != nil {
		t.Fatalf("ResolveOrAssign: %v", err)
	}

	if own.UserID != "user-1" {
		t.Errorf("expected UserID user-1, got %s", own.UserID)
	}
	if own.VMID == "" {
		t.Error("expected non-empty VMID")
	}
	if !strings.HasPrefix(own.VMID, "vm-") {
		t.Errorf("expected VMID to start with vm-, got %s", own.VMID)
	}
	if own.State != VMStateActive {
		t.Errorf("expected state active, got %s", own.State)
	}
	if own.SandboxURL == "" {
		t.Error("expected non-empty SandboxURL")
	}
}

func TestOwnershipRegistry_ResolveOrAssignReturnsSameVM(t *testing.T) {
	// VAL-VM-003: Repeated requests from the same user stay pinned to
	// the same active VM.
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	own1, err := reg.ResolveOrAssign("user-1")
	if err != nil {
		t.Fatalf("first ResolveOrAssign: %v", err)
	}

	own2, err := reg.ResolveOrAssign("user-1")
	if err != nil {
		t.Fatalf("second ResolveOrAssign: %v", err)
	}

	if own1.VMID != own2.VMID {
		t.Errorf("expected same VMID for repeated requests, got %s and %s", own1.VMID, own2.VMID)
	}
}

func TestOwnershipRegistry_ResolveOrAssignReturnsSnapshot(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	own, err := reg.ResolveOrAssign("user-snapshot")
	if err != nil {
		t.Fatalf("ResolveOrAssign: %v", err)
	}
	own.SandboxURL = "http://caller-mutated"
	own.State = VMStateFailed

	got := reg.GetOwnership("user-snapshot")
	if got == nil {
		t.Fatal("expected registry ownership")
	}
	if got.SandboxURL == "http://caller-mutated" {
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
	mu        sync.Mutex
	boots     []VMManagerConfig
	started   chan struct{}
	release   chan struct{}
	hostURL   string
	startOnce sync.Once
}

func newBlockingBootVMManager(hostURL string) *blockingBootVMManager {
	return &blockingBootVMManager{
		started: make(chan struct{}),
		release: make(chan struct{}),
		hostURL: hostURL,
	}
}

func (m *blockingBootVMManager) BootVM(cfg VMManagerConfig) (*VMInstanceInfo, error) {
	m.mu.Lock()
	m.boots = append(m.boots, cfg)
	m.mu.Unlock()
	m.startOnce.Do(func() { close(m.started) })
	<-m.release
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
func (m *blockingBootVMManager) DestroyVMState(vmID string) error      { return nil }
func (m *blockingBootVMManager) GetVM(vmID string) *VMInstanceInfo     { return nil }
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
	if firstOwn.SandboxURL != "http://127.0.0.1:9009" {
		t.Fatalf("first resolve sandbox URL = %q, want VM URL", firstOwn.SandboxURL)
	}
	if secondOwn.SandboxURL != "http://127.0.0.1:9009" {
		t.Fatalf("second resolve sandbox URL = %q, want VM URL", secondOwn.SandboxURL)
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
	if retryOwn.SandboxURL != "http://127.0.0.1:9009" {
		t.Fatalf("retry sandbox URL = %q, want VM URL", retryOwn.SandboxURL)
	}
}

func TestOwnershipRegistry_ActiveCount(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	if count := reg.ActiveCount(); count != 0 {
		t.Errorf("expected 0 active VMs, got %d", count)
	}

	if _, err := reg.ResolveOrAssign("user-1"); err != nil {
		t.Fatalf("ResolveOrAssign user-1: %v", err)
	}
	if count := reg.ActiveCount(); count != 1 {
		t.Errorf("expected 1 active VM, got %d", count)
	}

	if _, err := reg.ResolveOrAssign("user-2"); err != nil {
		t.Fatalf("ResolveOrAssign user-2: %v", err)
	}
	if count := reg.ActiveCount(); count != 2 {
		t.Errorf("expected 2 active VMs, got %d", count)
	}
}

func TestOwnershipRegistry_StopVM(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	own, _ := reg.ResolveOrAssign("user-1")
	if own.State != VMStateActive {
		t.Fatal("expected active state after assign")
	}

	if err := reg.StopVM("user-1"); err != nil {
		t.Fatalf("StopVM: %v", err)
	}

	// After stopping, the ownership should reflect stopped state.
	updated := reg.GetOwnership("user-1")
	if updated.State != VMStateStopped {
		t.Errorf("expected stopped state, got %s", updated.State)
	}
}

func TestOwnershipRegistry_StopNonexistentUser(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	err := reg.StopVM("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent user")
	}
}

func TestOwnershipRegistry_RemoveOwnership(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	own, _ := reg.ResolveOrAssign("user-1")
	vmID := own.VMID

	if err := reg.RemoveOwnership("user-1"); err != nil {
		t.Fatalf("RemoveOwnership: %v", err)
	}

	// Ownership should be gone.
	if reg.GetOwnership("user-1") != nil {
		t.Error("expected nil ownership after remove")
	}
	if reg.GetOwnershipByVMID(vmID) != nil {
		t.Error("expected nil VM-by-ID after remove")
	}
}

func TestOwnershipRegistry_RemoveOwnershipIdempotent(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	// Removing nonexistent user should not error.
	if err := reg.RemoveOwnership("nonexistent"); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestOwnershipRegistry_MarkUnhealthy(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	if _, err := reg.ResolveOrAssign("user-1"); err != nil {
		t.Fatalf("ResolveOrAssign: %v", err)
	}
	if err := reg.MarkUnhealthy("user-1"); err != nil {
		t.Fatalf("MarkUnhealthy: %v", err)
	}

	own := reg.GetOwnership("user-1")
	if own.State != VMStateDegraded {
		t.Errorf("expected degraded state, got %s", own.State)
	}
}

func TestOwnershipRegistry_ListOwnerships(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	if _, err := reg.ResolveOrAssign("user-1"); err != nil {
		t.Fatalf("ResolveOrAssign user-1: %v", err)
	}
	if _, err := reg.ResolveOrAssign("user-2"); err != nil {
		t.Fatalf("ResolveOrAssign user-2: %v", err)
	}
	if _, err := reg.ResolveOrAssign("user-3"); err != nil {
		t.Fatalf("ResolveOrAssign user-3: %v", err)
	}

	list := reg.ListOwnerships()
	if len(list) != 3 {
		t.Errorf("expected 3 ownerships, got %d", len(list))
	}
}

func TestOwnershipRegistry_RequestWorkerIncludedInList(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	if _, err := reg.ResolveOrAssignDesktop("user-1", PrimaryDesktopID); err != nil {
		t.Fatalf("ResolveOrAssignDesktop: %v", err)
	}
	if _, err := reg.RequestWorker(WorkerRequest{
		UserID:        "user-1",
		DesktopID:     PrimaryDesktopID,
		ParentAgentID: "super:primary",
		TrajectoryID:  "traj-1",
		Purpose:       "Run tests",
		MachineClass:  "worker-small",
	}); err != nil {
		t.Fatalf("RequestWorker: %v", err)
	}

	list := reg.ListOwnerships()
	if len(list) != 2 {
		t.Fatalf("expected interactive + worker ownerships, got %d", len(list))
	}
	if reg.ActiveCount() != 2 {
		t.Fatalf("active count = %d, want 2", reg.ActiveCount())
	}
}

func TestOwnershipRegistry_RequestWorkerReusesActiveLeaseUnlessParallelAllowed(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	if _, err := reg.ResolveOrAssignDesktop("user-1", PrimaryDesktopID); err != nil {
		t.Fatalf("ResolveOrAssignDesktop: %v", err)
	}
	req := WorkerRequest{
		UserID:        "user-1",
		DesktopID:     PrimaryDesktopID,
		ParentAgentID: "super:primary",
		TrajectoryID:  "traj-1",
		Purpose:       "Run the launch/upload/theme patch",
		MachineClass:  "worker-small",
	}
	first, err := reg.RequestWorker(req)
	if err != nil {
		t.Fatalf("RequestWorker first: %v", err)
	}
	secondReq := req
	secondReq.Purpose = " run THE LAUNCH/upload/theme patch!! "
	second, err := reg.RequestWorker(secondReq)
	if err != nil {
		t.Fatalf("RequestWorker second: %v", err)
	}
	if second.WorkerID != first.WorkerID || second.VMID != first.VMID {
		t.Fatalf("second worker = %s/%s, want reused %s/%s", second.WorkerID, second.VMID, first.WorkerID, first.VMID)
	}
	if first.ObjectiveFingerprint == "" || second.ObjectiveFingerprint != first.ObjectiveFingerprint {
		t.Fatalf("objective fingerprints = first %q second %q, want same non-empty fingerprint", first.ObjectiveFingerprint, second.ObjectiveFingerprint)
	}

	req.AllowParallel = true
	parallel, err := reg.RequestWorker(req)
	if err != nil {
		t.Fatalf("RequestWorker parallel: %v", err)
	}
	if parallel.WorkerID == first.WorkerID || parallel.VMID == first.VMID {
		t.Fatalf("parallel worker reused %s/%s unexpectedly", parallel.WorkerID, parallel.VMID)
	}
}

func TestOwnershipRegistry_RequestWorkerReturnsSnapshot(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	if _, err := reg.ResolveOrAssignDesktop("user-worker-snapshot", PrimaryDesktopID); err != nil {
		t.Fatalf("ResolveOrAssignDesktop: %v", err)
	}
	req := WorkerRequest{
		UserID:        "user-worker-snapshot",
		DesktopID:     PrimaryDesktopID,
		ParentAgentID: "super:primary",
		TrajectoryID:  "traj-worker-snapshot",
		Purpose:       "Run a worker snapshot test",
		MachineClass:  "worker-small",
	}
	first, err := reg.RequestWorker(req)
	if err != nil {
		t.Fatalf("RequestWorker first: %v", err)
	}
	first.SandboxURL = "http://caller-mutated"
	first.State = VMStateFailed

	stored := reg.GetOwnershipByVMID(first.VMID)
	if stored == nil {
		t.Fatal("expected stored worker ownership")
	}
	if stored.SandboxURL == "http://caller-mutated" || stored.State != VMStateActive {
		t.Fatalf("stored worker was mutated through returned pointer: url=%q state=%s", stored.SandboxURL, stored.State)
	}

	reused, err := reg.RequestWorker(req)
	if err != nil {
		t.Fatalf("RequestWorker reused: %v", err)
	}
	reused.State = VMStateFailed

	stored = reg.GetOwnershipByVMID(reused.VMID)
	if stored == nil {
		t.Fatal("expected reused worker ownership")
	}
	if stored.State != VMStateActive {
		t.Fatalf("reused worker return was a live pointer: state=%s", stored.State)
	}
}

func TestNormalizeWorkerMachineClassResourceEnvelope(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		wantClass string
		wantCPU   int
		wantMem   int
	}{
		{name: "default", raw: "", wantClass: "worker-small", wantCPU: 1, wantMem: 1024},
		{name: "standard alias", raw: "standard", wantClass: "worker-small", wantCPU: 1, wantMem: 1024},
		{name: "small", raw: "worker-small", wantClass: "worker-small", wantCPU: 1, wantMem: 1024},
		{name: "medium", raw: "worker-medium", wantClass: "worker-medium", wantCPU: 2, wantMem: 4096},
		{name: "medium alias", raw: " medium ", wantClass: "worker-medium", wantCPU: 2, wantMem: 4096},
		{name: "large", raw: "worker-large", wantClass: "worker-large", wantCPU: 4, wantMem: 8192},
		{name: "playwright evidence worker", raw: "worker-playwright", wantClass: "worker-playwright", wantCPU: 4, wantMem: 8192},
		{name: "playwright alias", raw: "verifier-browser", wantClass: "worker-playwright", wantCPU: 4, wantMem: 8192},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotClass, gotCPU, gotMem, err := normalizeWorkerMachineClass(tt.raw)
			if err != nil {
				t.Fatalf("normalizeWorkerMachineClass(%q): %v", tt.raw, err)
			}
			if gotClass != tt.wantClass || gotCPU != tt.wantCPU || gotMem != tt.wantMem {
				t.Fatalf("normalizeWorkerMachineClass(%q) = (%q, %d, %d), want (%q, %d, %d)", tt.raw, gotClass, gotCPU, gotMem, tt.wantClass, tt.wantCPU, tt.wantMem)
			}
		})
	}
}

func TestOwnershipRegistry_RequestWorkerBootsWithNormalizedMachineShape(t *testing.T) {
	mock := &mockVMManager{}
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetVMManager(mock)

	if _, err := reg.ResolveOrAssignDesktop("user-1", PrimaryDesktopID); err != nil {
		t.Fatalf("ResolveOrAssignDesktop: %v", err)
	}
	mock.boots = nil

	worker, err := reg.RequestWorker(WorkerRequest{
		UserID:        "user-1",
		DesktopID:     PrimaryDesktopID,
		ParentAgentID: "super:primary",
		TrajectoryID:  "traj-1",
		Purpose:       "Run a repo mutation and export verifier task",
		MachineClass:  "medium",
	})
	if err != nil {
		t.Fatalf("RequestWorker: %v", err)
	}
	if worker.MachineClass != "worker-medium" {
		t.Fatalf("worker machine_class = %q, want worker-medium", worker.MachineClass)
	}
	if len(mock.boots) != 1 {
		t.Fatalf("BootVM calls = %d, want 1", len(mock.boots))
	}
	got := mock.boots[0]
	if got.MachineCPUCount != 2 || got.MachineMemSizeMib != 4096 {
		t.Fatalf("BootVM shape = %d cpu / %d MiB, want 2 cpu / 4096 MiB", got.MachineCPUCount, got.MachineMemSizeMib)
	}
	if got.ComputerKind != "worker" || got.OwnerID != "user-1" || got.DesktopID != PrimaryDesktopID || got.WorkerID != worker.WorkerID || got.CandidateID != worker.WorkerID {
		t.Fatalf("BootVM guest identity = %+v, want worker identity for %s", got, worker.WorkerID)
	}
}

func TestOwnershipRegistry_RequestPlaywrightWorkerUsesDedicatedImageProfile(t *testing.T) {
	mock := &mockVMManager{}
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetVMManager(mock)
	reg.SetWorkerImageProfile("worker-playwright", VMImageProfile{
		KernelImagePath: "/var/lib/go-choir/guest-playwright/vmlinux",
		InitrdPath:      "/var/lib/go-choir/guest-playwright/initrd",
		RootfsPath:      "/var/lib/go-choir/guest-playwright/rootfs.ext4",
		StoreDiskPath:   "/var/lib/go-choir/guest-playwright/storedisk.erofs",
		KernelParams:    "root=fstab init=/nix/store/playwright-init/init",
	})

	if _, err := reg.ResolveOrAssignDesktop("user-1", PrimaryDesktopID); err != nil {
		t.Fatalf("ResolveOrAssignDesktop: %v", err)
	}
	mock.boots = nil

	worker, err := reg.RequestWorker(WorkerRequest{
		UserID:        "user-1",
		DesktopID:     PrimaryDesktopID,
		ParentAgentID: "super:primary",
		TrajectoryID:  "traj-1",
		Purpose:       "Capture browser screenshots and video evidence",
		MachineClass:  "worker-playwright",
	})
	if err != nil {
		t.Fatalf("RequestWorker: %v", err)
	}
	if worker.MachineClass != "worker-playwright" {
		t.Fatalf("worker machine_class = %q, want worker-playwright", worker.MachineClass)
	}
	if len(mock.boots) != 1 {
		t.Fatalf("BootVM calls = %d, want 1", len(mock.boots))
	}
	got := mock.boots[0]
	if got.MachineCPUCount != 4 || got.MachineMemSizeMib != 8192 {
		t.Fatalf("BootVM shape = %d cpu / %d MiB, want 4 cpu / 8192 MiB", got.MachineCPUCount, got.MachineMemSizeMib)
	}
	if got.KernelImagePath != "/var/lib/go-choir/guest-playwright/vmlinux" ||
		got.InitrdPath != "/var/lib/go-choir/guest-playwright/initrd" ||
		got.RootfsPath != "/var/lib/go-choir/guest-playwright/rootfs.ext4" ||
		got.StoreDiskPath != "/var/lib/go-choir/guest-playwright/storedisk.erofs" ||
		!strings.Contains(got.KernelParams, "playwright-init") {
		t.Fatalf("BootVM did not use playwright image profile: %+v", got)
	}
}

func TestOwnershipRegistry_RequestPlaywrightWorkerRequiresDedicatedImageProfile(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetVMManager(&mockVMManager{})

	if _, err := reg.ResolveOrAssignDesktop("user-1", PrimaryDesktopID); err != nil {
		t.Fatalf("ResolveOrAssignDesktop: %v", err)
	}

	_, err := reg.RequestWorker(WorkerRequest{
		UserID:        "user-1",
		DesktopID:     PrimaryDesktopID,
		ParentAgentID: "super:primary",
		TrajectoryID:  "traj-1",
		Purpose:       "Capture browser screenshots and video evidence",
		MachineClass:  "worker-playwright",
	})
	if err == nil || !strings.Contains(err.Error(), "requires a configured worker image profile") {
		t.Fatalf("RequestWorker error = %v, want missing image profile error", err)
	}
}

func TestOwnershipRegistry_RequestPlaywrightWorkerRejectsIncompleteImageProfile(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetVMManager(&mockVMManager{})
	reg.SetWorkerImageProfile("worker-playwright", VMImageProfile{
		KernelImagePath: "/var/lib/go-choir/guest-playwright/vmlinux",
		RootfsPath:      "/var/lib/go-choir/guest-playwright/rootfs.ext4",
	})

	if _, err := reg.ResolveOrAssignDesktop("user-1", PrimaryDesktopID); err != nil {
		t.Fatalf("ResolveOrAssignDesktop: %v", err)
	}

	_, err := reg.RequestWorker(WorkerRequest{
		UserID:        "user-1",
		DesktopID:     PrimaryDesktopID,
		ParentAgentID: "super:primary",
		TrajectoryID:  "traj-1",
		Purpose:       "Capture browser screenshots and video evidence",
		MachineClass:  "worker-playwright",
	})
	if err == nil || !strings.Contains(err.Error(), "worker image profile is incomplete") {
		t.Fatalf("RequestWorker error = %v, want incomplete image profile error", err)
	}
}

func TestOwnershipRegistry_InteractiveVMUsesBuildCapableMemoryEnvelope(t *testing.T) {
	mock := &mockVMManager{}
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetVMManager(mock)

	if _, err := reg.ResolveOrAssignDesktop("user-1", PrimaryDesktopID); err != nil {
		t.Fatalf("ResolveOrAssignDesktop: %v", err)
	}
	if len(mock.boots) != 1 {
		t.Fatalf("BootVM calls = %d, want 1", len(mock.boots))
	}
	got := mock.boots[0]
	if got.MachineCPUCount != interactiveVMCPUCount || got.MachineMemSizeMib != interactiveVMMemSizeMib {
		t.Fatalf("interactive BootVM shape = %d cpu / %d MiB, want %d cpu / %d MiB",
			got.MachineCPUCount, got.MachineMemSizeMib, interactiveVMCPUCount, interactiveVMMemSizeMib)
	}
	if got.ComputerKind != "active" || got.OwnerID != "user-1" || got.DesktopID != PrimaryDesktopID {
		t.Fatalf("interactive BootVM guest identity = %+v", got)
	}
}

func TestOwnershipRegistry_SetSandboxCredential(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	own, _ := reg.ResolveOrAssign("user-1")
	if err := reg.SetSandboxCredential(own.VMID, "cred-123"); err != nil {
		t.Fatalf("SetSandboxCredential: %v", err)
	}

	updated := reg.GetOwnership("user-1")
	if updated.SandboxCredential != "cred-123" {
		t.Errorf("expected credential cred-123, got %s", updated.SandboxCredential)
	}
}

func TestOwnershipRegistry_IsReady(t *testing.T) {
	own := &VMOwnership{State: VMStateActive}
	if !own.IsReady() {
		t.Error("expected active VM to be ready")
	}

	own.State = VMStateBooting
	if own.IsReady() {
		t.Error("expected booting VM to wait for readiness")
	}

	own.State = VMStateStopped
	if own.IsReady() {
		t.Error("expected stopped VM to not be ready")
	}

	own.State = VMStateFailed
	if own.IsReady() {
		t.Error("expected failed VM to not be ready")
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
	mux.HandleFunc("/internal/vmctl/fork-desktop", handler.HandleForkDesktop)
	mux.HandleFunc("/internal/vmctl/publish-desktop", handler.HandlePublishDesktop)
	mux.HandleFunc("/internal/vmctl/request-worker", handler.HandleRequestWorker)
	mux.HandleFunc("/internal/vmctl/hibernate-worker", handler.HandleHibernateWorker)
	mux.HandleFunc("/internal/vmctl/lookup", handler.HandleLookup)
	mux.HandleFunc("/internal/vmctl/stop", handler.HandleStop)
	mux.HandleFunc("/internal/vmctl/remove", handler.HandleRemove)
	mux.HandleFunc("/internal/vmctl/list", handler.HandleList)
	mux.HandleFunc("/internal/vmctl/hibernate", handler.HandleHibernate)
	mux.HandleFunc("/internal/vmctl/resume", handler.HandleResume)
	mux.HandleFunc("/internal/vmctl/recover", handler.HandleRecover)
	mux.HandleFunc("/internal/vmctl/logout", handler.HandleLogout)
	mux.HandleFunc("/internal/vmctl/idle-check", handler.HandleIdleCheck)
	mux.HandleFunc("/internal/vmctl/runtime-package/sandbox", handler.HandleRuntimePackage)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, reg
}

func TestHandler_RuntimePackageStreamsSandboxPackage(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	handler := NewHandler(reg)
	pkgDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(pkgDir, "bin"), 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(pkgDir, "share", "go-choir", "skills"), 0o755); err != nil {
		t.Fatalf("mkdir skills: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "bin", "sandbox"), []byte("sandbox-binary"), 0o755); err != nil {
		t.Fatalf("write sandbox: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "share", "go-choir", "skills", "SKILL.md"), []byte("skill"), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	handler.SetSandboxRuntimePackageDir(pkgDir)

	req := httptest.NewRequest(http.MethodGet, "/internal/vmctl/runtime-package/sandbox", nil)
	req.Header.Set("X-Internal-Caller", "true")
	req.Host = "10.203.154.1:8083"
	rr := httptest.NewRecorder()
	handler.HandleRuntimePackage(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "application/x-tar" {
		t.Fatalf("content-type = %q", got)
	}

	tr := tar.NewReader(rr.Body)
	entries := make(map[string]string)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read tar: %v", err)
		}
		if hdr.FileInfo().Mode().IsRegular() {
			data, err := io.ReadAll(tr)
			if err != nil {
				t.Fatalf("read %s: %v", hdr.Name, err)
			}
			entries[hdr.Name] = string(data)
		}
	}
	if entries["bin/sandbox"] != "sandbox-binary" {
		t.Fatalf("bin/sandbox entry = %q", entries["bin/sandbox"])
	}
	if entries["share/go-choir/skills/SKILL.md"] != "skill" {
		t.Fatalf("skills entry = %q", entries["share/go-choir/skills/SKILL.md"])
	}
	if env := entries["choir-runtime.env"]; !strings.Contains(env, "RUNTIME_WORKER_REPO_BASE_SHA=") ||
		!strings.Contains(env, "CHOIR_DEPLOYED_COMMIT=") ||
		!strings.Contains(env, "RUNTIME_WIRE_PUBLISH_URL=http://10.203.154.1:8082") ||
		!strings.Contains(env, "RUNTIME_PLATFORMD_URL=http://10.203.154.1:8082") {
		t.Fatalf("runtime env missing deployment/service refs: %q", env)
	}
}

func TestHandler_RuntimePackageDeniesExternalCaller(t *testing.T) {
	handler := NewHandler(NewOwnershipRegistry("http://127.0.0.1:8085"))
	handler.SetSandboxRuntimePackageDir(t.TempDir())

	req := httptest.NewRequest(http.MethodGet, "/internal/vmctl/runtime-package/sandbox", nil)
	req.Host = "choir.news"
	req.RemoteAddr = "203.0.113.10:4444"
	rr := httptest.NewRecorder()
	handler.HandleRuntimePackage(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHandler_Health(t *testing.T) {
	srv, reg := newTestServer(t)
	reg.SetIdleTimeout(time.Millisecond)
	if _, err := reg.ResolveOrAssign("health-user"); err != nil {
		t.Fatalf("resolve health user: %v", err)
	}
	if _, err := reg.RequestWorker(WorkerRequest{
		UserID:        "health-user",
		DesktopID:     PrimaryDesktopID,
		ParentAgentID: "agent-health",
		TrajectoryID:  "trace-health",
		Purpose:       "health telemetry worker",
		MachineClass:  "worker-small",
	}); err != nil {
		t.Fatalf("request health worker: %v", err)
	}

	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("health request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result vmctlHealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode health response: %v", err)
	}

	if result.Status != "ok" {
		t.Errorf("expected ok status, got %s", result.Status)
	}
	if result.Service != "vmctl" {
		t.Errorf("expected vmctl service, got %s", result.Service)
	}
	if result.ActiveVMs != 2 || result.TotalOwnerships != 2 {
		t.Fatalf("health counts active=%d total=%d, want 2/2", result.ActiveVMs, result.TotalOwnerships)
	}
	if result.ByKind[string(VMKindInteractive)] != 1 || result.ByKind[string(VMKindWorker)] != 1 {
		t.Fatalf("health kind counts = %+v, want interactive and worker", result.ByKind)
	}
	if result.ByState[string(VMStateActive)] != 2 {
		t.Fatalf("health state counts = %+v, want two active", result.ByState)
	}
	if result.Reclaim.Mode != PressureReclaimModeOff {
		t.Fatalf("default reclaim mode = %s, want off", result.Reclaim.Mode)
	}
	if result.Warmness.Policy.PrimaryKeepaliveMode != PrimaryKeepaliveModeOff {
		t.Fatalf("default warmness mode = %s, want off", result.Warmness.Policy.PrimaryKeepaliveMode)
	}
	if result.Warmness.ByClass[string(WarmnessClassPrimary)] != 1 || result.Warmness.ByClass[string(WarmnessClassWorker)] != 1 {
		t.Fatalf("warmness class counts = %+v, want primary and worker", result.Warmness.ByClass)
	}
}

func TestOwnershipRegistry_PressureReclaimDryRunOrdersIdleWorkersBeforeInteractive(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetPressureReclaimConfig(PressureReclaimConfig{
		Mode:                      PressureReclaimModeDryRun,
		MinIdle:                   10 * time.Minute,
		MinMemoryAvailableBytes:   2 * 1024 * 1024 * 1024,
		MinMemoryAvailablePercent: 15,
		MaxMemorySomeAvg10:        1,
		MaxIOSomeAvg10:            5,
		StateDir:                  t.TempDir(),
		MaxCandidates:             5,
	})
	reg.setPressureSamplerForTest(func(cfg PressureReclaimConfig) HostPressureSample {
		return HostPressureSample{
			SampledAt:              "2026-05-14T12:00:00Z",
			MemoryTotalBytes:       8 * 1024 * 1024 * 1024,
			MemoryAvailableBytes:   512 * 1024 * 1024,
			MemoryAvailablePercent: 6.25,
			MemorySomeAvg10:        2.5,
		}
	})

	if _, err := reg.ResolveOrAssign("interactive-old"); err != nil {
		t.Fatalf("resolve interactive-old: %v", err)
	}
	if _, err := reg.ResolveOrAssign("interactive-recent"); err != nil {
		t.Fatalf("resolve interactive-recent: %v", err)
	}
	worker, err := reg.RequestWorker(WorkerRequest{
		UserID:        "interactive-old",
		DesktopID:     PrimaryDesktopID,
		ParentAgentID: "agent-1",
		Purpose:       "background indexing",
		MachineClass:  "worker-small",
	})
	if err != nil {
		t.Fatalf("request worker: %v", err)
	}

	reg.mu.Lock()
	reg.ownerships[ownershipKey("interactive-old", PrimaryDesktopID)].LastActiveAt = time.Now().Add(-2 * time.Hour)
	reg.ownerships[ownershipKey("interactive-recent", PrimaryDesktopID)].LastActiveAt = time.Now().Add(-time.Minute)
	reg.workerVMs[worker.WorkerID].LastActiveAt = time.Now().Add(-3 * time.Hour)
	reg.mu.Unlock()

	plan := reg.PressureReclaimPlan()
	if plan.Mode != PressureReclaimModeDryRun {
		t.Fatalf("mode = %s, want dry-run", plan.Mode)
	}
	if plan.Decision != "would_reclaim" {
		t.Fatalf("decision = %s, want would_reclaim (plan=%+v)", plan.Decision, plan)
	}
	if !plan.Pressure.Pressure || !plan.Pressure.MemoryPressure {
		t.Fatalf("expected memory pressure in plan: %+v", plan.Pressure)
	}
	if plan.Inventory.Eligible != 2 || plan.Inventory.Protected != 1 {
		t.Fatalf("inventory eligible/protected = %d/%d, want 2/1", plan.Inventory.Eligible, plan.Inventory.Protected)
	}
	if len(plan.Candidates) < 3 {
		t.Fatalf("expected at least 3 candidates, got %+v", plan.Candidates)
	}
	if plan.Candidates[0].Kind != VMKindWorker || plan.Candidates[0].Protected {
		t.Fatalf("first candidate = %+v, want eligible worker", plan.Candidates[0])
	}
	if plan.Candidates[1].Kind != VMKindInteractive || plan.Candidates[1].Protected {
		t.Fatalf("second candidate = %+v, want eligible interactive", plan.Candidates[1])
	}
	if !plan.Candidates[2].Protected || !containsString(plan.Candidates[2].ProtectedReasons, "recent_activity") {
		t.Fatalf("third candidate = %+v, want recent_activity protected", plan.Candidates[2])
	}
}

func TestOwnershipRegistry_PressureReclaimProtectsCriticalWorkerPurpose(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetPressureReclaimConfig(PressureReclaimConfig{
		Mode:          PressureReclaimModeDryRun,
		MinIdle:       10 * time.Minute,
		MaxCandidates: 5,
	})
	reg.setPressureSamplerForTest(func(cfg PressureReclaimConfig) HostPressureSample {
		return HostPressureSample{
			SampledAt:              "2026-05-14T12:00:00Z",
			MemoryTotalBytes:       8 * 1024 * 1024 * 1024,
			MemoryAvailableBytes:   512 * 1024 * 1024,
			MemoryAvailablePercent: 6.25,
		}
	})
	if _, err := reg.ResolveOrAssign("user-promote"); err != nil {
		t.Fatalf("resolve user-promote: %v", err)
	}
	worker, err := reg.RequestWorker(WorkerRequest{
		UserID:        "user-promote",
		DesktopID:     PrimaryDesktopID,
		ParentAgentID: "agent-promote",
		Purpose:       "promotion verifier with rollback evidence",
		MachineClass:  "worker-small",
	})
	if err != nil {
		t.Fatalf("request worker: %v", err)
	}
	reg.mu.Lock()
	reg.workerVMs[worker.WorkerID].LastActiveAt = time.Now().Add(-2 * time.Hour)
	reg.mu.Unlock()

	plan := reg.PressureReclaimPlan()
	if plan.Inventory.Protected != 2 || plan.Inventory.Eligible != 0 {
		t.Fatalf("inventory protected/eligible = %d/%d, want 2/0", plan.Inventory.Protected, plan.Inventory.Eligible)
	}
	if len(plan.Candidates) < 1 {
		t.Fatalf("expected candidates, got %+v", plan.Candidates)
	}
	foundCriticalWorker := false
	for _, candidate := range plan.Candidates {
		if candidate.Kind == VMKindWorker && containsString(candidate.ProtectedReasons, "critical_worker_purpose") {
			foundCriticalWorker = true
			break
		}
	}
	if !foundCriticalWorker {
		t.Fatalf("candidates = %+v, want protected critical worker", plan.Candidates)
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

func TestOwnershipRegistry_ReclaimStaleVMStateDestroysOnlyTerminalWorkersAndUnpublishedCandidates(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetPressureReclaimConfig(PressureReclaimConfig{
		Mode:                      PressureReclaimModeActive,
		MinStateDirAvailableBytes: 10 * 1024 * 1024 * 1024,
		StaleStateMinAge:          time.Hour,
		MaxStateDeletes:           5,
		MaxCandidates:             5,
	})
	reg.setPressureSamplerForTest(func(cfg PressureReclaimConfig) HostPressureSample {
		return HostPressureSample{
			SampledAt:                "2026-05-20T12:00:00Z",
			StateDirAvailableBytes:   512 * 1024 * 1024,
			StateDirAvailablePercent: 2,
		}
	})
	if _, err := reg.ResolveOrAssign("stale-user"); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	unpublished, err := reg.ForkDesktop("stale-user", PrimaryDesktopID, "candidate-old")
	if err != nil {
		t.Fatalf("fork unpublished: %v", err)
	}
	published, err := reg.ForkDesktop("stale-user", PrimaryDesktopID, "candidate-published")
	if err != nil {
		t.Fatalf("fork published: %v", err)
	}
	if _, err := reg.PublishDesktop("stale-user", "candidate-published"); err != nil {
		t.Fatalf("publish candidate: %v", err)
	}
	worker, err := reg.RequestWorker(WorkerRequest{
		UserID:        "stale-user",
		DesktopID:     PrimaryDesktopID,
		ParentAgentID: "agent-stale",
		Purpose:       "experiment cleanup worker",
		MachineClass:  "worker-small",
	})
	if err != nil {
		t.Fatalf("request worker: %v", err)
	}
	recentWorker, err := reg.RequestWorker(WorkerRequest{
		UserID:               "stale-user",
		DesktopID:            PrimaryDesktopID,
		ParentAgentID:        "agent-recent",
		Purpose:              "recent experiment worker",
		ObjectiveFingerprint: "recent",
		MachineClass:         "worker-small",
	})
	if err != nil {
		t.Fatalf("request recent worker: %v", err)
	}

	old := time.Now().Add(-3 * time.Hour)
	reg.mu.Lock()
	reg.ownerships[ownershipKey("stale-user", PrimaryDesktopID)].LastActiveAt = old
	reg.ownerships[ownershipKey("stale-user", "candidate-old")].State = VMStateHibernated
	reg.ownerships[ownershipKey("stale-user", "candidate-old")].LastActiveAt = old
	reg.ownerships[ownershipKey("stale-user", "candidate-published")].State = VMStateHibernated
	reg.ownerships[ownershipKey("stale-user", "candidate-published")].LastActiveAt = old
	reg.workerVMs[worker.WorkerID].State = VMStateHibernated
	reg.workerVMs[worker.WorkerID].LastActiveAt = old
	reg.workerVMs[recentWorker.WorkerID].State = VMStateHibernated
	reg.workerVMs[recentWorker.WorkerID].LastActiveAt = time.Now()
	reg.mu.Unlock()
	mgr := &mockVMManager{}
	reg.SetVMManager(mgr)

	destroyed := reg.ReclaimStaleVMState()
	if destroyed != 2 {
		t.Fatalf("destroyed = %d, want 2 (destroy calls=%v)", destroyed, mgr.destroys)
	}
	if !containsString(mgr.destroys, unpublished.VMID) || !containsString(mgr.destroys, worker.VMID) {
		t.Fatalf("destroyed VMs = %v, want unpublished candidate %s and worker %s", mgr.destroys, unpublished.VMID, worker.VMID)
	}
	if reg.GetOwnershipForDesktop("stale-user", "candidate-old") != nil {
		t.Fatalf("unpublished candidate ownership should be removed")
	}
	if reg.GetOwnershipByVMID(worker.VMID) != nil {
		t.Fatalf("worker ownership should be removed")
	}
	if reg.GetOwnershipForDesktop("stale-user", "candidate-published") == nil {
		t.Fatalf("published candidate should be protected (vm=%s)", published.VMID)
	}
	if reg.GetOwnershipByVMID(recentWorker.VMID) == nil {
		t.Fatalf("recent worker should be protected")
	}
	if reg.GetOwnership("stale-user") == nil {
		t.Fatalf("primary ownership should be protected")
	}
}

func TestOwnershipRegistry_RetentionPlanTargetsOnlyOrphansAndEphemeralPrimaries(t *testing.T) {
	stateDir := t.TempDir()
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetRetentionPruneConfig(RetentionPruneConfig{
		Mode:                    RetentionPruneModeDryRun,
		StateDir:                stateDir,
		EphemeralEmailDomains:   []string{"example.com", "example.test"},
		EphemeralUserIDPrefixes: []string{"diagnostic-", "sourcemaxx-proof-"},
		OrphanMinAge:            time.Hour,
		EphemeralMinAge:         time.Hour,
		MaxDeletes:              10,
		MaxBytes:                1024 * 1024 * 1024,
	})
	reg.setRetentionUserEmailsForTest(map[string]string{
		"test-user":         "playwright@example.com",
		"example-test-user": "load@example.test",
		"real-user":         "yusefnathanson@me.com",
		"owner-test-a":      "a@b.com",
		"owner-test-b":      "b@c.com",
		"active-user":       "active@example.com",
		"branch-user":       "branch@example.test",
	})
	testOwn, err := reg.ResolveOrAssign("test-user")
	if err != nil {
		t.Fatalf("resolve test user: %v", err)
	}
	exampleTestOwn, err := reg.ResolveOrAssign("example-test-user")
	if err != nil {
		t.Fatalf("resolve example.test user: %v", err)
	}
	syntheticOwn, err := reg.ResolveOrAssign("diagnostic-1778792614")
	if err != nil {
		t.Fatalf("resolve synthetic user: %v", err)
	}
	sourcemaxxOwn, err := reg.ResolveOrAssign("sourcemaxx-proof-85751dc5")
	if err != nil {
		t.Fatalf("resolve sourcemaxx synthetic user: %v", err)
	}
	realOwn, err := reg.ResolveOrAssign("real-user")
	if err != nil {
		t.Fatalf("resolve real user: %v", err)
	}
	ownerTestAOwn, err := reg.ResolveOrAssign("owner-test-a")
	if err != nil {
		t.Fatalf("resolve owner test a: %v", err)
	}
	ownerTestBOwn, err := reg.ResolveOrAssign("owner-test-b")
	if err != nil {
		t.Fatalf("resolve owner test b: %v", err)
	}
	activeOwn, err := reg.ResolveOrAssign("active-user")
	if err != nil {
		t.Fatalf("resolve active user: %v", err)
	}
	if _, err := reg.ResolveOrAssign("branch-user"); err != nil {
		t.Fatalf("resolve branch user primary: %v", err)
	}
	branchOwn, err := reg.ForkDesktop("branch-user", PrimaryDesktopID, "candidate-a")
	if err != nil {
		t.Fatalf("fork branch user: %v", err)
	}
	old := time.Now().Add(-3 * time.Hour)
	reg.mu.Lock()
	reg.ownerships[ownershipKey("test-user", PrimaryDesktopID)].State = VMStateHibernated
	reg.ownerships[ownershipKey("test-user", PrimaryDesktopID)].LastActiveAt = old
	reg.ownerships[ownershipKey("example-test-user", PrimaryDesktopID)].State = VMStateHibernated
	reg.ownerships[ownershipKey("example-test-user", PrimaryDesktopID)].LastActiveAt = old
	reg.ownerships[ownershipKey("diagnostic-1778792614", PrimaryDesktopID)].State = VMStateHibernated
	reg.ownerships[ownershipKey("diagnostic-1778792614", PrimaryDesktopID)].LastActiveAt = old
	reg.ownerships[ownershipKey("sourcemaxx-proof-85751dc5", PrimaryDesktopID)].State = VMStateHibernated
	reg.ownerships[ownershipKey("sourcemaxx-proof-85751dc5", PrimaryDesktopID)].LastActiveAt = old
	reg.ownerships[ownershipKey("real-user", PrimaryDesktopID)].State = VMStateHibernated
	reg.ownerships[ownershipKey("real-user", PrimaryDesktopID)].LastActiveAt = old
	reg.ownerships[ownershipKey("owner-test-a", PrimaryDesktopID)].State = VMStateHibernated
	reg.ownerships[ownershipKey("owner-test-a", PrimaryDesktopID)].LastActiveAt = old
	reg.ownerships[ownershipKey("owner-test-b", PrimaryDesktopID)].State = VMStateHibernated
	reg.ownerships[ownershipKey("owner-test-b", PrimaryDesktopID)].LastActiveAt = old
	reg.ownerships[ownershipKey("active-user", PrimaryDesktopID)].State = VMStateActive
	reg.ownerships[ownershipKey("active-user", PrimaryDesktopID)].LastActiveAt = old
	reg.ownerships[ownershipKey("branch-user", "candidate-a")].State = VMStateHibernated
	reg.ownerships[ownershipKey("branch-user", "candidate-a")].LastActiveAt = old
	reg.mu.Unlock()

	for _, vmID := range []string{
		testOwn.VMID,
		exampleTestOwn.VMID,
		syntheticOwn.VMID,
		sourcemaxxOwn.VMID,
		realOwn.VMID,
		ownerTestAOwn.VMID,
		ownerTestBOwn.VMID,
		activeOwn.VMID,
		branchOwn.VMID,
		"vm-orphan-old",
		"vm-orphan-recent",
	} {
		dir := filepath.Join(stateDir, vmID)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", vmID, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "data"), []byte("state"), 0o600); err != nil {
			t.Fatalf("write %s: %v", vmID, err)
		}
	}
	oldTime := time.Now().Add(-3 * time.Hour)
	if err := os.Chtimes(filepath.Join(stateDir, "vm-orphan-old"), oldTime, oldTime); err != nil {
		t.Fatalf("chtimes orphan old: %v", err)
	}

	plan := reg.RetentionPrunePlan()
	if plan.Decision != "would_prune" {
		t.Fatalf("decision = %s, want would_prune: %+v", plan.Decision, plan)
	}
	if !retentionPlanHasVM(plan, testOwn.VMID) {
		t.Fatalf("plan missing ephemeral test primary %s: %+v", testOwn.VMID, plan.Candidates)
	}
	if !retentionPlanHasVM(plan, exampleTestOwn.VMID) {
		t.Fatalf("plan missing example.test primary %s: %+v", exampleTestOwn.VMID, plan.Candidates)
	}
	if !retentionPlanHasVM(plan, syntheticOwn.VMID) {
		t.Fatalf("plan missing synthetic-prefix primary %s: %+v", syntheticOwn.VMID, plan.Candidates)
	}
	if !retentionPlanHasVM(plan, sourcemaxxOwn.VMID) {
		t.Fatalf("plan missing sourcemaxx synthetic-prefix primary %s: %+v", sourcemaxxOwn.VMID, plan.Candidates)
	}
	if !retentionPlanHasVM(plan, "vm-orphan-old") {
		t.Fatalf("plan missing old orphan: %+v", plan.Candidates)
	}
	if retentionPlanHasVM(plan, realOwn.VMID) {
		t.Fatalf("plan must not include real user primary %s: %+v", realOwn.VMID, plan.Candidates)
	}
	if retentionPlanHasVM(plan, ownerTestAOwn.VMID) {
		t.Fatalf("plan must not include owner test account a %s: %+v", ownerTestAOwn.VMID, plan.Candidates)
	}
	if retentionPlanHasVM(plan, ownerTestBOwn.VMID) {
		t.Fatalf("plan must not include owner test account b %s: %+v", ownerTestBOwn.VMID, plan.Candidates)
	}
	if retentionPlanHasVM(plan, activeOwn.VMID) {
		t.Fatalf("plan must not include active ephemeral primary %s: %+v", activeOwn.VMID, plan.Candidates)
	}
	if retentionPlanHasVM(plan, branchOwn.VMID) {
		t.Fatalf("plan must not include unpublished non-primary desktop %s: %+v", branchOwn.VMID, plan.Candidates)
	}
	if retentionPlanHasVM(plan, "vm-orphan-recent") {
		t.Fatalf("plan must not include recent orphan: %+v", plan.Candidates)
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
	result := reg.PruneRetention()
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

	result := reg.PruneRetention()
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

func TestOwnershipRegistry_ReclaimStaleVMStateRequiresStoragePressure(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetPressureReclaimConfig(PressureReclaimConfig{
		Mode:                      PressureReclaimModeActive,
		MinStateDirAvailableBytes: 10 * 1024 * 1024 * 1024,
		StaleStateMinAge:          time.Hour,
		MaxStateDeletes:           5,
		MaxCandidates:             5,
	})
	reg.setPressureSamplerForTest(func(cfg PressureReclaimConfig) HostPressureSample {
		return HostPressureSample{
			SampledAt:                "2026-05-20T12:00:00Z",
			StateDirAvailableBytes:   20 * 1024 * 1024 * 1024,
			StateDirAvailablePercent: 20,
		}
	})
	if _, err := reg.ResolveOrAssign("no-storage-pressure-user"); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	candidate, err := reg.ForkDesktop("no-storage-pressure-user", PrimaryDesktopID, "candidate-old")
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	reg.mu.Lock()
	reg.ownerships[ownershipKey("no-storage-pressure-user", "candidate-old")].State = VMStateHibernated
	reg.ownerships[ownershipKey("no-storage-pressure-user", "candidate-old")].LastActiveAt = time.Now().Add(-3 * time.Hour)
	reg.mu.Unlock()
	mgr := &mockVMManager{}
	reg.SetVMManager(mgr)

	if destroyed := reg.ReclaimStaleVMState(); destroyed != 0 {
		t.Fatalf("destroyed = %d, want 0", destroyed)
	}
	if len(mgr.destroys) != 0 {
		t.Fatalf("destroy calls = %v, want none", mgr.destroys)
	}
	if reg.GetOwnershipByVMID(candidate.VMID) == nil {
		t.Fatalf("candidate should remain when storage pressure is absent")
	}
}

func TestOwnershipRegistry_PressureReclaimExpiresStaleCriticalWorkerProtection(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetPressureReclaimConfig(PressureReclaimConfig{
		Mode:                      PressureReclaimModeActive,
		MinIdle:                   10 * time.Minute,
		MinMemoryAvailablePercent: 15,
		MaxCandidates:             1,
	})
	reg.setPressureSamplerForTest(func(cfg PressureReclaimConfig) HostPressureSample {
		return HostPressureSample{
			SampledAt:              "2026-05-18T18:30:00Z",
			MemoryTotalBytes:       8 * 1024 * 1024 * 1024,
			MemoryAvailableBytes:   512 * 1024 * 1024,
			MemoryAvailablePercent: 6.25,
		}
	})
	if _, err := reg.ResolveOrAssign("user-stale-promote"); err != nil {
		t.Fatalf("resolve user-stale-promote: %v", err)
	}
	worker, err := reg.RequestWorker(WorkerRequest{
		UserID:        "user-stale-promote",
		DesktopID:     PrimaryDesktopID,
		ParentAgentID: "agent-promote",
		Purpose:       "promotion verifier with rollback evidence",
		MachineClass:  "worker-small",
	})
	if err != nil {
		t.Fatalf("request worker: %v", err)
	}
	reg.mu.Lock()
	reg.ownerships[ownershipKey("user-stale-promote", PrimaryDesktopID)].LastActiveAt = time.Now().Add(-time.Minute)
	reg.workerVMs[worker.WorkerID].LastActiveAt = time.Now().Add(-(criticalWorkerProtectionMaxIdle + time.Hour))
	reg.mu.Unlock()

	plan := reg.PressureReclaimPlan()
	if plan.Inventory.Eligible != 1 {
		t.Fatalf("eligible = %d, want stale critical worker reclaimable (plan=%+v)", plan.Inventory.Eligible, plan)
	}
	if len(plan.Candidates) == 0 || plan.Candidates[0].Kind != VMKindWorker || plan.Candidates[0].Protected {
		t.Fatalf("first candidate = %+v, want unprotected stale critical worker", plan.Candidates)
	}
	if containsString(plan.Candidates[0].ProtectedReasons, "critical_worker_purpose") {
		t.Fatalf("stale critical worker should not keep critical protection: %+v", plan.Candidates[0])
	}
	if got := reg.ReclaimPressureVMs(); got != 1 {
		t.Fatalf("reclaimed = %d, want 1", got)
	}
	if workerOwn := reg.GetOwnershipByVMID(worker.VMID); workerOwn == nil || workerOwn.State != VMStateHibernated || workerOwn.StoppedBy != "pressure" {
		t.Fatalf("worker ownership after pressure reclaim = %+v, want hibernated by pressure", workerOwn)
	}
}

func TestOwnershipRegistry_ActivePressureReclaimHibernatesBoundedEligibleCandidates(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetPressureReclaimConfig(PressureReclaimConfig{
		Mode:                      PressureReclaimModeActive,
		MinIdle:                   10 * time.Minute,
		MinMemoryAvailableBytes:   2 * 1024 * 1024 * 1024,
		MinMemoryAvailablePercent: 15,
		MaxCandidates:             1,
	})
	reg.SetWarmnessPolicyConfig(WarmnessPolicyConfig{PrimaryKeepaliveMode: PrimaryKeepaliveModeUnderCapacity})
	reg.setPressureSamplerForTest(func(cfg PressureReclaimConfig) HostPressureSample {
		return HostPressureSample{
			SampledAt:              "2026-05-17T16:56:38Z",
			MemoryTotalBytes:       8 * 1024 * 1024 * 1024,
			MemoryAvailableBytes:   256 * 1024 * 1024,
			MemoryAvailablePercent: 3.125,
		}
	})

	if _, err := reg.ResolveOrAssign("pressure-user"); err != nil {
		t.Fatalf("resolve pressure-user: %v", err)
	}
	worker, err := reg.RequestWorker(WorkerRequest{
		UserID:        "pressure-user",
		DesktopID:     PrimaryDesktopID,
		ParentAgentID: "agent-worker",
		Purpose:       "background indexing",
		MachineClass:  "worker-small",
	})
	if err != nil {
		t.Fatalf("request worker: %v", err)
	}
	critical, err := reg.RequestWorker(WorkerRequest{
		UserID:        "pressure-user",
		DesktopID:     PrimaryDesktopID,
		ParentAgentID: "agent-verifier",
		Purpose:       "promotion verifier with rollback evidence",
		MachineClass:  "worker-small",
	})
	if err != nil {
		t.Fatalf("request critical worker: %v", err)
	}
	reg.mu.Lock()
	reg.ownerships[ownershipKey("pressure-user", PrimaryDesktopID)].LastActiveAt = time.Now().Add(-3 * time.Hour)
	reg.workerVMs[worker.WorkerID].LastActiveAt = time.Now().Add(-4 * time.Hour)
	reg.workerVMs[critical.WorkerID].LastActiveAt = time.Now().Add(-5 * time.Hour)
	reg.mu.Unlock()

	plan := reg.PressureReclaimPlan()
	if plan.Mode != PressureReclaimModeActive || plan.Decision != "reclaim" {
		t.Fatalf("plan mode/decision = %s/%s, want active/reclaim", plan.Mode, plan.Decision)
	}
	if got := reg.ReclaimPressureVMs(); got != 1 {
		t.Fatalf("reclaimed = %d, want 1", got)
	}
	if workerOwn := reg.GetOwnershipByVMID(worker.VMID); workerOwn == nil || workerOwn.State != VMStateHibernated || workerOwn.StoppedBy != "pressure" {
		t.Fatalf("worker ownership after pressure reclaim = %+v, want hibernated by pressure", workerOwn)
	}
	if primary := reg.GetOwnership("pressure-user"); primary == nil || primary.State != VMStateActive {
		t.Fatalf("primary should stay active while lower-priority worker was reclaimable, got %+v", primary)
	}
	if criticalOwn := reg.GetOwnershipByVMID(critical.VMID); criticalOwn == nil || criticalOwn.State != VMStateActive {
		t.Fatalf("critical worker should stay active, got %+v", criticalOwn)
	}
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
	if stopped := reg.StopIdleVMs(); stopped != 0 {
		t.Fatalf("stopped = %d, want 0", stopped)
	}
	if own := reg.GetOwnership("primary-warm"); own == nil || own.State != VMStateActive {
		t.Fatalf("primary should remain active, got %+v", own)
	}
}

func TestOwnershipRegistry_PrimaryKeepaliveReclaimsLowerPriorityFirstUnderPressure(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetIdleTimeout(10 * time.Millisecond)
	reg.SetPressureReclaimConfig(PressureReclaimConfig{
		Mode:                      PressureReclaimModeDryRun,
		MinIdle:                   time.Millisecond,
		MinMemoryAvailableBytes:   2 * 1024 * 1024 * 1024,
		MinMemoryAvailablePercent: 15,
	})
	reg.SetWarmnessPolicyConfig(WarmnessPolicyConfig{PrimaryKeepaliveMode: PrimaryKeepaliveModeUnderCapacity})
	reg.setPressureSamplerForTest(func(cfg PressureReclaimConfig) HostPressureSample {
		return HostPressureSample{
			SampledAt:              "2026-05-14T12:00:00Z",
			MemoryTotalBytes:       8 * 1024 * 1024 * 1024,
			MemoryAvailableBytes:   512 * 1024 * 1024,
			MemoryAvailablePercent: 6.25,
		}
	})

	if _, err := reg.ResolveOrAssign("pressure-user"); err != nil {
		t.Fatalf("resolve pressure-user: %v", err)
	}
	worker, err := reg.RequestWorker(WorkerRequest{
		UserID:        "pressure-user",
		DesktopID:     PrimaryDesktopID,
		ParentAgentID: "agent-pressure",
		Purpose:       "background indexing",
		MachineClass:  "worker-small",
	})
	if err != nil {
		t.Fatalf("request worker: %v", err)
	}
	reg.mu.Lock()
	reg.ownerships[ownershipKey("pressure-user", PrimaryDesktopID)].LastActiveAt = time.Now().Add(-time.Hour)
	reg.workerVMs[worker.WorkerID].LastActiveAt = time.Now().Add(-2 * time.Hour)
	reg.mu.Unlock()

	idle := reg.CheckIdleOwnerships()
	if len(idle) != 1 || idle[0].Kind != VMKindWorker {
		t.Fatalf("idle candidates = %+v, want only lower-priority worker", idle)
	}
	if stopped := reg.StopIdleVMs(); stopped != 1 {
		t.Fatalf("stopped = %d, want 1", stopped)
	}
	if own := reg.GetOwnership("pressure-user"); own == nil || own.State != VMStateActive {
		t.Fatalf("primary should remain active while worker was reclaimable, got %+v", own)
	}
	if workerOwn := reg.GetOwnershipByVMID(worker.VMID); workerOwn == nil || workerOwn.State != VMStateHibernated {
		t.Fatalf("worker should be hibernated, got %+v", workerOwn)
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

func TestOwnershipRegistry_WarmAlwaysOnResumesHibernatedPrimaryOnly(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	mock := &mockVMManager{}
	reg.SetVMManager(mock)
	reg.SetWarmnessPolicyConfig(WarmnessPolicyConfig{
		AlwaysOnUserIDs: map[string]bool{"premium-user": true},
	})

	premium, err := reg.ResolveOrAssign("premium-user")
	if err != nil {
		t.Fatalf("resolve premium-user: %v", err)
	}
	if err := reg.HibernateVM("premium-user"); err != nil {
		t.Fatalf("hibernate premium-user: %v", err)
	}
	if _, err := reg.ResolveOrAssign("ordinary-user"); err != nil {
		t.Fatalf("resolve ordinary-user: %v", err)
	}
	if err := reg.HibernateVM("ordinary-user"); err != nil {
		t.Fatalf("hibernate ordinary-user: %v", err)
	}
	if _, err := reg.ForkDesktop("premium-user", PrimaryDesktopID, "candidate-a"); err != nil {
		t.Fatalf("fork candidate: %v", err)
	}
	if err := reg.HibernateVMForDesktop("premium-user", "candidate-a"); err != nil {
		t.Fatalf("hibernate candidate: %v", err)
	}

	if warmed := reg.WarmAlwaysOnDesktops(); warmed != 1 {
		t.Fatalf("warmed = %d, want 1", warmed)
	}
	if len(mock.resumes) != 1 || mock.resumes[0] != premium.VMID {
		t.Fatalf("resumes = %+v, want only premium primary %s", mock.resumes, premium.VMID)
	}
	if got := reg.GetOwnership("premium-user"); got == nil || got.State != VMStateActive {
		t.Fatalf("premium primary = %+v, want active", got)
	}
	if got := reg.GetOwnership("ordinary-user"); got == nil || got.State != VMStateHibernated {
		t.Fatalf("ordinary primary = %+v, want hibernated", got)
	}
	if got := reg.GetOwnershipForDesktop("premium-user", "candidate-a"); got == nil || got.State != VMStateHibernated {
		t.Fatalf("premium candidate = %+v, want hibernated", got)
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

func TestHandler_ResolveCreatesVM(t *testing.T) {
	// VAL-VM-001: First protected request resolves through VM ownership.
	srv, _ := newTestServer(t)

	body := `{"user_id":"user-1"}`
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/internal/vmctl/resolve", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("resolve request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result resolveResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode resolve response: %v", err)
	}

	if result.UserID != "user-1" {
		t.Errorf("expected user-1, got %s", result.UserID)
	}
	if result.VMID == "" {
		t.Error("expected non-empty VMID")
	}
	if result.SandboxURL == "" {
		t.Error("expected non-empty SandboxURL")
	}
	if result.State != "active" {
		t.Errorf("expected active state, got %s", result.State)
	}
}

func TestHandler_ResolveReturnsExistingVM(t *testing.T) {
	// VAL-VM-003: Repeated requests stay pinned to the same VM.
	srv, _ := newTestServer(t)

	// First resolve.
	body := `{"user_id":"user-1"}`
	req1, _ := http.NewRequest(http.MethodPost, srv.URL+"/internal/vmctl/resolve", strings.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("X-Internal-Caller", "true")
	resp1, _ := http.DefaultClient.Do(req1)
	var result1 resolveResponse
	if err := json.NewDecoder(resp1.Body).Decode(&result1); err != nil {
		t.Fatalf("decode result1: %v", err)
	}
	_ = resp1.Body.Close()

	// Second resolve.
	req2, _ := http.NewRequest(http.MethodPost, srv.URL+"/internal/vmctl/resolve", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-Internal-Caller", "true")
	resp2, _ := http.DefaultClient.Do(req2)
	var result2 resolveResponse
	if err := json.NewDecoder(resp2.Body).Decode(&result2); err != nil {
		t.Fatalf("decode result2: %v", err)
	}
	_ = resp2.Body.Close()

	if result1.VMID != result2.VMID {
		t.Errorf("expected same VMID across resolves, got %s and %s", result1.VMID, result2.VMID)
	}
}

func TestHandler_ResolveDeniesExternalCallers(t *testing.T) {
	// VAL-VM-012: vmctl control endpoints are not publicly accessible.
	// Verify the isInternalCaller function properly rejects non-localhost callers.
	if !isInternalCaller(&http.Request{Host: "192.168.1.1:8083", RemoteAddr: "10.0.0.1:12345"}) {
		// Good, non-localhost is rejected
	} else {
		t.Error("expected non-localhost caller to be rejected")
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

func TestHandler_Lookup(t *testing.T) {
	srv, _ := newTestServer(t)

	// First create an ownership.
	body := `{"user_id":"user-1"}`
	req1, _ := http.NewRequest(http.MethodPost, srv.URL+"/internal/vmctl/resolve", strings.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("X-Internal-Caller", "true")
	resp1, _ := http.DefaultClient.Do(req1)
	_ = resp1.Body.Close()

	// Now lookup.
	req2, _ := http.NewRequest(http.MethodGet, srv.URL+"/internal/vmctl/lookup?user_id=user-1", nil)
	req2.Header.Set("X-Internal-Caller", "true")
	resp2, _ := http.DefaultClient.Do(req2)
	defer func() { _ = resp2.Body.Close() }()

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}

	var result ownershipResponse
	if err := json.NewDecoder(resp2.Body).Decode(&result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result.UserID != "user-1" {
		t.Errorf("expected user-1, got %s", result.UserID)
	}
}

func TestHandler_LookupNonexistent(t *testing.T) {
	srv, _ := newTestServer(t)

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/internal/vmctl/lookup?user_id=nonexistent", nil)
	req.Header.Set("X-Internal-Caller", "true")

	resp, _ := http.DefaultClient.Do(req)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestHandler_ForkDesktop(t *testing.T) {
	srv, reg := newTestServer(t)

	if _, err := reg.ResolveOrAssignDesktop("user-1", PrimaryDesktopID); err != nil {
		t.Fatalf("ResolveOrAssignDesktop primary: %v", err)
	}

	body := `{"user_id":"user-1","source_desktop_id":"primary","target_desktop_id":"branch-a"}`
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/internal/vmctl/fork-desktop", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("fork request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result resolveResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result.DesktopID != "branch-a" {
		t.Fatalf("desktop_id = %q, want branch-a", result.DesktopID)
	}
	if result.ParentDesktopID != PrimaryDesktopID {
		t.Fatalf("parent_desktop_id = %q, want %q", result.ParentDesktopID, PrimaryDesktopID)
	}
	if result.Published {
		t.Fatal("forked desktop should not be published yet")
	}
}

func TestOwnershipRegistry_ForkDesktopWithVMManagerUsesSourceDataImage(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	mock := &mockVMManager{
		bootResponse: &VMInstanceInfo{
			HostURL: "http://127.0.0.1:9090",
			Epoch:   2,
			Healthy: true,
			State:   "running",
		},
	}
	reg.SetVMManager(mock)

	source, err := reg.ResolveOrAssignDesktop("user-1", PrimaryDesktopID)
	if err != nil {
		t.Fatalf("ResolveOrAssignDesktop primary: %v", err)
	}
	if err := reg.StopVMForDesktop("user-1", PrimaryDesktopID); err != nil {
		t.Fatalf("StopVMForDesktop primary: %v", err)
	}

	branch, err := reg.ForkDesktop("user-1", PrimaryDesktopID, "branch-a")
	if err != nil {
		t.Fatalf("ForkDesktop: %v", err)
	}
	if branch.ParentVMID != source.VMID {
		t.Fatalf("ParentVMID = %q, want %q", branch.ParentVMID, source.VMID)
	}
	if branch.SnapshotKind != "data_img_copy" {
		t.Fatalf("SnapshotKind = %q, want data_img_copy", branch.SnapshotKind)
	}
	if len(mock.boots) != 2 {
		t.Fatalf("expected source boot plus fork boot, got %d", len(mock.boots))
	}
	forkBoot := mock.boots[1]
	if forkBoot.SourceVMID != source.VMID {
		t.Fatalf("fork SourceVMID = %q, want %q", forkBoot.SourceVMID, source.VMID)
	}
	if forkBoot.VMID != branch.VMID {
		t.Fatalf("fork boot VMID = %q, want branch VMID %q", forkBoot.VMID, branch.VMID)
	}
	if forkBoot.ComputerKind != "candidate" || forkBoot.OwnerID != "user-1" || forkBoot.DesktopID != "branch-a" || forkBoot.CandidateID != "branch-a" {
		t.Fatalf("fork boot guest identity = %+v", forkBoot)
	}
}

func TestOwnershipRegistry_ForkDesktopRejectsActiveSourceWithVMManager(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetVMManager(&mockVMManager{})

	source, err := reg.ResolveOrAssignDesktop("user-1", PrimaryDesktopID)
	if err != nil {
		t.Fatalf("ResolveOrAssignDesktop primary: %v", err)
	}
	if _, err := reg.ForkDesktop("user-1", PrimaryDesktopID, "branch-a"); err == nil {
		t.Fatal("expected active source fork to fail with VM manager")
	} else if !strings.Contains(err.Error(), source.VMID) || !strings.Contains(err.Error(), "unsafe live data image fork") {
		t.Fatalf("unexpected fork error: %v", err)
	}
}

func TestHandler_PublishDesktop(t *testing.T) {
	srv, reg := newTestServer(t)

	if _, err := reg.ResolveOrAssignDesktop("user-1", PrimaryDesktopID); err != nil {
		t.Fatalf("ResolveOrAssignDesktop primary: %v", err)
	}
	if _, err := reg.ForkDesktop("user-1", PrimaryDesktopID, "branch-a"); err != nil {
		t.Fatalf("ForkDesktop: %v", err)
	}

	body := `{"user_id":"user-1","desktop_id":"branch-a"}`
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/internal/vmctl/publish-desktop", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("publish request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result resolveResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if !result.Published {
		t.Fatal("published desktop should be marked published")
	}
}

func TestHandler_RequestWorker(t *testing.T) {
	srv, reg := newTestServer(t)

	if _, err := reg.ResolveOrAssignDesktop("user-1", PrimaryDesktopID); err != nil {
		t.Fatalf("ResolveOrAssignDesktop primary: %v", err)
	}

	body := `{"user_id":"user-1","desktop_id":"primary","parent_agent_id":"super:primary","trajectory_id":"traj-1","purpose":"Run background coding task","machine_class":"worker-medium"}`
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/internal/vmctl/request-worker", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request-worker request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result WorkerVMHandle
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result.Kind != VMKindWorker {
		t.Fatalf("kind = %q, want %q", result.Kind, VMKindWorker)
	}
	if result.WorkerID == "" || result.VMID == "" {
		t.Fatalf("expected non-empty worker identifiers: %+v", result)
	}
	if result.DesktopID != PrimaryDesktopID {
		t.Fatalf("desktop_id = %q, want %q", result.DesktopID, PrimaryDesktopID)
	}
	if result.ParentAgentID != "super:primary" {
		t.Fatalf("parent_agent_id = %q, want super:primary", result.ParentAgentID)
	}
	if result.TrajectoryID != "traj-1" {
		t.Fatalf("trajectory_id = %q, want traj-1", result.TrajectoryID)
	}
	if result.Purpose != "Run background coding task" {
		t.Fatalf("purpose = %q, want background coding task", result.Purpose)
	}
	if result.MachineClass != "worker-medium" {
		t.Fatalf("machine_class = %q, want worker-medium", result.MachineClass)
	}
}

func TestHandler_HibernateWorker(t *testing.T) {
	srv, reg := newTestServer(t)

	if _, err := reg.ResolveOrAssignDesktop("user-1", PrimaryDesktopID); err != nil {
		t.Fatalf("ResolveOrAssignDesktop primary: %v", err)
	}
	worker, err := reg.RequestWorker(WorkerRequest{
		UserID:        "user-1",
		DesktopID:     PrimaryDesktopID,
		ParentAgentID: "super:primary",
		Purpose:       "Run background proof task",
		MachineClass:  "worker-small",
		AllowParallel: true,
	})
	if err != nil {
		t.Fatalf("RequestWorker: %v", err)
	}

	body := fmt.Sprintf(`{"worker_id":%q}`, worker.WorkerID)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/internal/vmctl/hibernate-worker", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("hibernate-worker request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result["status"] != "hibernated" {
		t.Fatalf("status = %q, want hibernated", result["status"])
	}
	if result["worker_id"] != worker.WorkerID {
		t.Fatalf("worker_id = %q, want %q", result["worker_id"], worker.WorkerID)
	}
	own := reg.GetOwnershipByVMID(worker.VMID)
	if own == nil {
		t.Fatalf("worker ownership missing after hibernate")
	}
	if own.State != VMStateHibernated {
		t.Fatalf("worker state = %q, want %q", own.State, VMStateHibernated)
	}
	if own.StoppedBy != "idle" {
		t.Fatalf("stopped_by = %q, want idle", own.StoppedBy)
	}
}

func TestHandler_Stop(t *testing.T) {
	srv, _ := newTestServer(t)

	// First create.
	body := `{"user_id":"user-1"}`
	req1, _ := http.NewRequest(http.MethodPost, srv.URL+"/internal/vmctl/resolve", strings.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("X-Internal-Caller", "true")
	resp1, _ := http.DefaultClient.Do(req1)
	_ = resp1.Body.Close()

	// Now stop.
	req2, _ := http.NewRequest(http.MethodPost, srv.URL+"/internal/vmctl/stop", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-Internal-Caller", "true")
	resp2, _ := http.DefaultClient.Do(req2)
	defer func() { _ = resp2.Body.Close() }()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp2.StatusCode)
	}

	// Lookup should still find it but in stopped state.
	req3, _ := http.NewRequest(http.MethodGet, srv.URL+"/internal/vmctl/lookup?user_id=user-1", nil)
	req3.Header.Set("X-Internal-Caller", "true")
	resp3, _ := http.DefaultClient.Do(req3)
	defer func() { _ = resp3.Body.Close() }()

	var result ownershipResponse
	if err := json.NewDecoder(resp3.Body).Decode(&result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result.State != "stopped" {
		t.Errorf("expected stopped state, got %s", result.State)
	}
}

func TestHandler_Remove(t *testing.T) {
	srv, _ := newTestServer(t)

	// First create.
	body := `{"user_id":"user-1"}`
	req1, _ := http.NewRequest(http.MethodPost, srv.URL+"/internal/vmctl/resolve", strings.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("X-Internal-Caller", "true")
	resp1, _ := http.DefaultClient.Do(req1)
	_ = resp1.Body.Close()

	// Now remove.
	req2, _ := http.NewRequest(http.MethodPost, srv.URL+"/internal/vmctl/remove", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-Internal-Caller", "true")
	resp2, _ := http.DefaultClient.Do(req2)
	defer func() { _ = resp2.Body.Close() }()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp2.StatusCode)
	}

	// Lookup should return 404.
	req3, _ := http.NewRequest(http.MethodGet, srv.URL+"/internal/vmctl/lookup?user_id=user-1", nil)
	req3.Header.Set("X-Internal-Caller", "true")
	resp3, _ := http.DefaultClient.Do(req3)
	defer func() { _ = resp3.Body.Close() }()

	if resp3.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 after remove, got %d", resp3.StatusCode)
	}
}

func TestHandler_List(t *testing.T) {
	srv, _ := newTestServer(t)

	// Create two ownerships.
	for _, userID := range []string{"user-1", "user-2"} {
		body := fmt.Sprintf(`{"user_id":"%s"}`, userID)
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/internal/vmctl/resolve", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Internal-Caller", "true")
		resp, _ := http.DefaultClient.Do(req)
		_ = resp.Body.Close()
	}

	// List.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/internal/vmctl/list", nil)
	req.Header.Set("X-Internal-Caller", "true")
	resp, _ := http.DefaultClient.Do(req)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode result: %v", err)
	}

	count, _ := result["count"].(float64)
	if int(count) != 2 {
		t.Errorf("expected 2 ownerships, got %v", count)
	}
}

func TestHandler_MethodNotAllowed(t *testing.T) {
	srv, _ := newTestServer(t)

	// GET on a POST-only endpoint.
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/internal/vmctl/resolve", nil)
	req.Header.Set("X-Internal-Caller", "true")
	resp, _ := http.DefaultClient.Do(req)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", resp.StatusCode)
	}
}

// --- Client Tests ---

func TestClient_ResolveAndLookup(t *testing.T) {
	srv, _ := newTestServer(t)
	client := NewClient(srv.URL)

	// Resolve creates a VM.
	resp, err := client.Resolve("user-client-test")
	if err != nil {
		t.Fatalf("client resolve: %v", err)
	}
	if resp.UserID != "user-client-test" {
		t.Errorf("expected user-client-test, got %s", resp.UserID)
	}
	if resp.VMID == "" {
		t.Error("expected non-empty VMID")
	}

	// Lookup finds the existing VM.
	lookup, err := client.Lookup("user-client-test")
	if err != nil {
		t.Fatalf("client lookup: %v", err)
	}
	if lookup == nil {
		t.Fatal("expected non-nil lookup result")
	}
	if lookup.VMID != resp.VMID {
		t.Errorf("expected same VMID %s, got %s", resp.VMID, lookup.VMID)
	}
}

func TestClient_LookupNonexistent(t *testing.T) {
	srv, _ := newTestServer(t)
	client := NewClient(srv.URL)

	result, err := client.Lookup("nonexistent")
	if err != nil {
		t.Fatalf("client lookup nonexistent: %v", err)
	}
	if result != nil {
		t.Error("expected nil for nonexistent user")
	}
}

func TestClient_Stop(t *testing.T) {
	srv, _ := newTestServer(t)
	client := NewClient(srv.URL)

	if _, err := client.Resolve("user-stop-test"); err != nil {
		t.Fatalf("client resolve: %v", err)
	}

	if err := client.Stop("user-stop-test"); err != nil {
		t.Fatalf("client stop: %v", err)
	}
}

func TestClient_Remove(t *testing.T) {
	srv, _ := newTestServer(t)
	client := NewClient(srv.URL)

	if _, err := client.Resolve("user-remove-test"); err != nil {
		t.Fatalf("client resolve: %v", err)
	}

	if err := client.Remove("user-remove-test"); err != nil {
		t.Fatalf("client remove: %v", err)
	}

	// Lookup should return nil.
	result, _ := client.Lookup("user-remove-test")
	if result != nil {
		t.Error("expected nil after remove")
	}
}

func TestClient_DifferentUsersIsolatedVMs(t *testing.T) {
	// VAL-VM-005: Different users receive distinct VMs.
	srv, _ := newTestServer(t)
	client := NewClient(srv.URL)

	resp1, _ := client.Resolve("alice")
	resp2, _ := client.Resolve("bob")

	if resp1.VMID == resp2.VMID {
		t.Error("expected different VM IDs for different users")
	}
}

func TestClient_ResolveDesktopAndLookupDesktop(t *testing.T) {
	srv, _ := newTestServer(t)
	client := NewClient(srv.URL)

	primary, err := client.ResolveDesktop("user-desktop", PrimaryDesktopID)
	if err != nil {
		t.Fatalf("ResolveDesktop primary: %v", err)
	}
	branch, err := client.ForkDesktop("user-desktop", PrimaryDesktopID, "branch-a")
	if err != nil {
		t.Fatalf("ForkDesktop branch: %v", err)
	}
	branch, err = client.PublishDesktop("user-desktop", "branch-a")
	if err != nil {
		t.Fatalf("PublishDesktop branch: %v", err)
	}
	if primary.VMID == branch.VMID {
		t.Fatalf("expected different VM IDs per desktop, got %s", primary.VMID)
	}

	lookup, err := client.LookupDesktop("user-desktop", "branch-a")
	if err != nil {
		t.Fatalf("LookupDesktop branch: %v", err)
	}
	if lookup == nil || lookup.VMID != branch.VMID {
		t.Fatalf("branch lookup mismatch: %+v", lookup)
	}
	if lookup.DesktopID != "branch-a" {
		t.Errorf("lookup DesktopID = %q, want %q", lookup.DesktopID, "branch-a")
	}
}

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

func TestClient_ForkDesktop(t *testing.T) {
	srv, _ := newTestServer(t)
	client := NewClient(srv.URL)

	if _, err := client.ResolveDesktop("user-desktop", PrimaryDesktopID); err != nil {
		t.Fatalf("ResolveDesktop primary: %v", err)
	}
	branch, err := client.ForkDesktop("user-desktop", PrimaryDesktopID, "branch-a")
	if err != nil {
		t.Fatalf("ForkDesktop: %v", err)
	}
	if branch.DesktopID != "branch-a" {
		t.Fatalf("desktop_id = %q, want branch-a", branch.DesktopID)
	}
	if branch.ParentDesktopID != PrimaryDesktopID {
		t.Fatalf("parent_desktop_id = %q, want %q", branch.ParentDesktopID, PrimaryDesktopID)
	}
	if branch.Published {
		t.Fatal("forked desktop should not be published yet")
	}
}

func TestClient_PublishDesktop(t *testing.T) {
	srv, _ := newTestServer(t)
	client := NewClient(srv.URL)

	if _, err := client.ResolveDesktop("user-desktop", PrimaryDesktopID); err != nil {
		t.Fatalf("ResolveDesktop primary: %v", err)
	}
	if _, err := client.ForkDesktop("user-desktop", PrimaryDesktopID, "branch-a"); err != nil {
		t.Fatalf("ForkDesktop: %v", err)
	}
	published, err := client.PublishDesktop("user-desktop", "branch-a")
	if err != nil {
		t.Fatalf("PublishDesktop: %v", err)
	}
	if !published.Published {
		t.Fatal("published desktop should be marked published")
	}
}

func TestClient_RequestWorker(t *testing.T) {
	srv, _ := newTestServer(t)
	client := NewClient(srv.URL)

	if _, err := client.ResolveDesktop("user-desktop", PrimaryDesktopID); err != nil {
		t.Fatalf("ResolveDesktop primary: %v", err)
	}
	handle, err := client.RequestWorker(WorkerRequest{
		UserID:        "user-desktop",
		DesktopID:     PrimaryDesktopID,
		ParentAgentID: "super:primary",
		TrajectoryID:  "traj-1",
		Purpose:       "Run background coding task",
		MachineClass:  "worker-small",
	})
	if err != nil {
		t.Fatalf("RequestWorker: %v", err)
	}
	if handle.Kind != VMKindWorker {
		t.Fatalf("kind = %q, want %q", handle.Kind, VMKindWorker)
	}
	if handle.WorkerID == "" || handle.VMID == "" {
		t.Fatalf("expected non-empty worker identifiers: %+v", handle)
	}
	if handle.DesktopID != PrimaryDesktopID {
		t.Fatalf("desktop_id = %q, want %q", handle.DesktopID, PrimaryDesktopID)
	}
	if handle.ParentAgentID != "super:primary" {
		t.Fatalf("parent_agent_id = %q, want super:primary", handle.ParentAgentID)
	}
	if handle.Purpose != "Run background coding task" {
		t.Fatalf("purpose = %q, want background coding task", handle.Purpose)
	}
}

func TestClient_ConcurrentResolveSameUser(t *testing.T) {
	// VAL-VM-004: Concurrent first requests for one user collapse.
	srv, _ := newTestServer(t)
	client := NewClient(srv.URL)

	const concurrency = 10
	results := make(chan *resolveResponse, concurrency)
	errors := make(chan error, concurrency)

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := client.Resolve("user-concurrent")
			if err != nil {
				errors <- err
				return
			}
			results <- resp
		}()
	}
	wg.Wait()
	close(results)
	close(errors)

	for err := range errors {
		t.Errorf("concurrent client resolve: %v", err)
	}

	var vmIDs []string
	for resp := range results {
		vmIDs = append(vmIDs, resp.VMID)
	}

	if len(vmIDs) != concurrency {
		t.Fatalf("expected %d results, got %d", concurrency, len(vmIDs))
	}

	first := vmIDs[0]
	for _, id := range vmIDs[1:] {
		if id != first {
			t.Errorf("expected all concurrent callers to get VM %s, got %s", first, id)
		}
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

func TestOwnershipRegistry_LastActiveAtUpdated(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	own1, _ := reg.ResolveOrAssign("user-1")
	firstActive := own1.LastActiveAt

	// Wait a tiny bit and resolve again.
	time.Sleep(10 * time.Millisecond)

	if _, err := reg.ResolveOrAssign("user-1"); err != nil {
		t.Fatalf("ResolveOrAssign: %v", err)
	}
	updated := reg.GetOwnership("user-1")

	if !updated.LastActiveAt.After(firstActive) {
		t.Error("expected LastActiveAt to be updated on subsequent resolve")
	}
}

// --- Endpoint URL Tests ---

func TestEndpointURLs(t *testing.T) {
	base := "http://localhost:8083"

	if got := ResolveEndpoint(base); got != "http://localhost:8083/internal/vmctl/resolve" {
		t.Errorf("ResolveEndpoint = %s", got)
	}
	if got := LookupEndpoint(base); got != "http://localhost:8083/internal/vmctl/lookup" {
		t.Errorf("LookupEndpoint = %s", got)
	}
	if got := ForkDesktopEndpoint(base); got != "http://localhost:8083/internal/vmctl/fork-desktop" {
		t.Errorf("ForkDesktopEndpoint = %s", got)
	}
	if got := PublishDesktopEndpoint(base); got != "http://localhost:8083/internal/vmctl/publish-desktop" {
		t.Errorf("PublishDesktopEndpoint = %s", got)
	}
	if got := RequestWorkerEndpoint(base); got != "http://localhost:8083/internal/vmctl/request-worker" {
		t.Errorf("RequestWorkerEndpoint = %s", got)
	}
	if got := HibernateWorkerEndpoint(base); got != "http://localhost:8083/internal/vmctl/hibernate-worker" {
		t.Errorf("HibernateWorkerEndpoint = %s", got)
	}
	if got := StopEndpoint(base); got != "http://localhost:8083/internal/vmctl/stop" {
		t.Errorf("StopEndpoint = %s", got)
	}
	if got := RemoveEndpoint(base); got != "http://localhost:8083/internal/vmctl/remove" {
		t.Errorf("RemoveEndpoint = %s", got)
	}
	if got := HibernateEndpoint(base); got != "http://localhost:8083/internal/vmctl/hibernate" {
		t.Errorf("HibernateEndpoint = %s", got)
	}
	if got := ResumeEndpoint(base); got != "http://localhost:8083/internal/vmctl/resume" {
		t.Errorf("ResumeEndpoint = %s", got)
	}
	if got := RecoverEndpoint(base); got != "http://localhost:8083/internal/vmctl/recover" {
		t.Errorf("RecoverEndpoint = %s", got)
	}
	if got := LogoutEndpoint(base); got != "http://localhost:8083/internal/vmctl/logout" {
		t.Errorf("LogoutEndpoint = %s", got)
	}
	if got := IdleCheckEndpoint(base); got != "http://localhost:8083/internal/vmctl/idle-check" {
		t.Errorf("IdleCheckEndpoint = %s", got)
	}
	if got := ReclaimEndpoint(base); got != "http://localhost:8083/internal/vmctl/reclaim" {
		t.Errorf("ReclaimEndpoint = %s", got)
	}
	if got := RetentionPlanEndpoint(base); got != "http://localhost:8083/internal/vmctl/retention-plan" {
		t.Errorf("RetentionPlanEndpoint = %s", got)
	}
	if got := RetentionShadowPlanEndpoint(base); got != "http://localhost:8083/internal/vmctl/retention-shadow-plan" {
		t.Errorf("RetentionShadowPlanEndpoint = %s", got)
	}
	if got := PulseEndpoint(base); got != "http://localhost:8083/internal/vmctl/pulse" {
		t.Errorf("PulseEndpoint = %s", got)
	}
	if got := PruneEndpoint(base); got != "http://localhost:8083/internal/vmctl/prune" {
		t.Errorf("PruneEndpoint = %s", got)
	}
}

func TestPulseAccountClassifier(t *testing.T) {
	tests := []struct {
		email string
		want  string
	}{
		{"owner@choir.news", PulseAccountReal},
		{"YusefNathanson@me.com", PulseAccountReal},
		{"codex-proof@example.com", PulseAccountCodexAgenticTest},
		{"matrix@example.test", PulseAccountCodexAgenticTest},
		{"a@b.com", PulseAccountProtectedTest},
		{"b@c.com", PulseAccountProtectedTest},
		{"system@choir.local", PulseAccountInternal},
		{"", PulseAccountUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			if got := ClassifyPulseAccount(tt.email); got != tt.want {
				t.Fatalf("ClassifyPulseAccount(%q) = %q, want %q", tt.email, got, tt.want)
			}
		})
	}
}

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
		{VMID: "vm-real-1", UserID: "real-1", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, Published: true, State: VMStateActive, LastActiveAt: now.Add(-time.Hour)},
		{VMID: "vm-real-2", UserID: "real-2", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, Published: true, State: VMStateFailed, LastActiveAt: now.Add(-8 * 24 * time.Hour)},
		{VMID: "vm-codex-1", UserID: "codex-1", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, Published: true, State: VMStateHibernated, LastActiveAt: now.Add(-time.Hour)},
		{VMID: "vm-protected-a", UserID: "protected-a", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, Published: true, State: VMStateHibernated, LastActiveAt: now.Add(-time.Hour)},
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
	stopped := reg.StopIdleVMs()
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
	reg.StartIdleSweeper(ctx, 5*time.Millisecond)

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

func TestOwnershipRegistry_FailedVMGetsNewAssignment(t *testing.T) {
	// Failed VMs should get a new assignment (new VMID, new epoch).
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	own1, _ := reg.ResolveOrAssign("user-1")
	oldVMID := own1.VMID
	oldEpoch := own1.Epoch

	// Simulate a failure.
	reg.mu.Lock()
	reg.ownerships[ownershipKey("user-1", PrimaryDesktopID)].State = VMStateFailed
	reg.mu.Unlock()

	// ResolveOrAssign should create a new VM for the failed state.
	own2, _ := reg.ResolveOrAssign("user-1")
	if own2.VMID == oldVMID {
		t.Error("expected new VM ID for failed VM")
	}
	if own2.Epoch <= oldEpoch {
		t.Errorf("expected new epoch > %d for new VM, got %d", oldEpoch, own2.Epoch)
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

func TestHandler_HibernateAndResume(t *testing.T) {
	srv, _ := newTestServer(t)

	// Create a VM.
	body := `{"user_id":"user-1"}`
	req1, _ := http.NewRequest(http.MethodPost, srv.URL+"/internal/vmctl/resolve", strings.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("X-Internal-Caller", "true")
	resp1, _ := http.DefaultClient.Do(req1)
	var result1 resolveResponse
	if err := json.NewDecoder(resp1.Body).Decode(&result1); err != nil {
		t.Fatalf("decode result1: %v", err)
	}
	_ = resp1.Body.Close()
	vmID := result1.VMID

	// Hibernate.
	req2, _ := http.NewRequest(http.MethodPost, srv.URL+"/internal/vmctl/hibernate", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-Internal-Caller", "true")
	resp2, _ := http.DefaultClient.Do(req2)
	defer func() { _ = resp2.Body.Close() }()

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on hibernate, got %d", resp2.StatusCode)
	}

	var hibResult map[string]interface{}
	if err := json.NewDecoder(resp2.Body).Decode(&hibResult); err != nil {
		t.Fatalf("decode hibResult: %v", err)
	}
	if hibResult["status"] != "hibernated" {
		t.Errorf("expected status=hibernated, got %v", hibResult["status"])
	}
	if hibResult["vm_id"] != vmID {
		t.Errorf("expected vm_id=%s, got %v", vmID, hibResult["vm_id"])
	}

	// Resume.
	req3, _ := http.NewRequest(http.MethodPost, srv.URL+"/internal/vmctl/resume", strings.NewReader(body))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("X-Internal-Caller", "true")
	resp3, _ := http.DefaultClient.Do(req3)
	defer func() { _ = resp3.Body.Close() }()

	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on resume, got %d", resp3.StatusCode)
	}

	var result3 resolveResponse
	if err := json.NewDecoder(resp3.Body).Decode(&result3); err != nil {
		t.Fatalf("decode result3: %v", err)
	}
	if result3.VMID != vmID {
		t.Errorf("expected same VMID after resume, got %s", result3.VMID)
	}
	if result3.State != "active" {
		t.Errorf("expected active state after resume, got %s", result3.State)
	}
}

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
	if result3.SandboxURL != "http://127.0.0.1:9101" {
		t.Fatalf("SandboxURL = %s, want boot fallback URL", result3.SandboxURL)
	}
	if len(mock.resumes) != 1 {
		t.Fatalf("expected one resume attempt, got %d", len(mock.resumes))
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

func TestHandler_LogoutStopsVM(t *testing.T) {
	// VAL-VM-008: Logout stops only the current user's VM.
	srv, _ := newTestServer(t)

	// Create VMs for two users.
	for _, userID := range []string{"user-alice", "user-bob"} {
		body := fmt.Sprintf(`{"user_id":"%s"}`, userID)
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/internal/vmctl/resolve", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Internal-Caller", "true")
		resp, _ := http.DefaultClient.Do(req)
		_ = resp.Body.Close()
	}

	// Logout alice.
	body := `{"user_id":"user-alice"}`
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/internal/vmctl/logout", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")
	resp, _ := http.DefaultClient.Do(req)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on logout, got %d", resp.StatusCode)
	}

	// Lookup alice — should be stopped.
	req2, _ := http.NewRequest(http.MethodGet, srv.URL+"/internal/vmctl/lookup?user_id=user-alice", nil)
	req2.Header.Set("X-Internal-Caller", "true")
	resp2, _ := http.DefaultClient.Do(req2)
	var aliceResp ownershipResponse
	if err := json.NewDecoder(resp2.Body).Decode(&aliceResp); err != nil {
		t.Fatalf("decode aliceResp: %v", err)
	}
	_ = resp2.Body.Close()
	if aliceResp.State != "stopped" {
		t.Errorf("expected alice VM stopped after logout, got %s", aliceResp.State)
	}

	// Lookup bob — should still be active.
	req3, _ := http.NewRequest(http.MethodGet, srv.URL+"/internal/vmctl/lookup?user_id=user-bob", nil)
	req3.Header.Set("X-Internal-Caller", "true")
	resp3, _ := http.DefaultClient.Do(req3)
	var bobResp ownershipResponse
	if err := json.NewDecoder(resp3.Body).Decode(&bobResp); err != nil {
		t.Fatalf("decode bobResp: %v", err)
	}
	_ = resp3.Body.Close()
	if bobResp.State != "active" {
		t.Errorf("expected bob VM still active, got %s", bobResp.State)
	}
}

func TestHandler_IdleCheckEndpoint(t *testing.T) {
	srv, reg := newTestServer(t)

	// Set a very short idle timeout.
	reg.SetIdleTimeout(50 * time.Millisecond)

	// Create a VM.
	body := `{"user_id":"user-1"}`
	req1, _ := http.NewRequest(http.MethodPost, srv.URL+"/internal/vmctl/resolve", strings.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("X-Internal-Caller", "true")
	resp1, _ := http.DefaultClient.Do(req1)
	_ = resp1.Body.Close()

	// Backdate the VM.
	reg.mu.Lock()
	reg.ownerships[ownershipKey("user-1", PrimaryDesktopID)].LastActiveAt = time.Now().Add(-100 * time.Millisecond)
	reg.mu.Unlock()

	// Trigger idle check.
	req2, _ := http.NewRequest(http.MethodPost, srv.URL+"/internal/vmctl/idle-check", nil)
	req2.Header.Set("X-Internal-Caller", "true")
	resp2, _ := http.DefaultClient.Do(req2)
	defer func() { _ = resp2.Body.Close() }()

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on idle-check, got %d", resp2.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp2.Body).Decode(&result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if vmsStopped, _ := result["vms_stopped"].(float64); int(vmsStopped) != 1 {
		t.Errorf("expected 1 VM stopped, got %v", result["vms_stopped"])
	}
}

func TestHandler_LifecycleEndpointsDenyExternalCallers(t *testing.T) {
	// VAL-VM-012: All lifecycle endpoints require internal access.
	// The isInternalCaller function is tested separately above.
	// This test verifies that the handler endpoints exist and are
	// wired up correctly. The actual external caller denial is
	// tested via isInternalCaller unit tests and via the proxy's
	// HandleVMctlDeny which blocks /internal/vmctl/* at the proxy
	// level for browser callers.
	srv, _ := newTestServer(t)

	endpoints := []struct {
		path   string
		method string
		body   string
	}{
		{"/internal/vmctl/fork-desktop", "POST", `{"user_id":"user-1","source_desktop_id":"primary","target_desktop_id":"branch-a"}`},
		{"/internal/vmctl/publish-desktop", "POST", `{"user_id":"user-1","desktop_id":"branch-a"}`},
		{"/internal/vmctl/request-worker", "POST", `{"user_id":"user-1","desktop_id":"primary","parent_agent_id":"super:primary","purpose":"Run tests"}`},
		{"/internal/vmctl/hibernate", "POST", `{"user_id":"user-1"}`},
		{"/internal/vmctl/resume", "POST", `{"user_id":"user-1"}`},
		{"/internal/vmctl/recover", "POST", `{"user_id":"user-1"}`},
		{"/internal/vmctl/logout", "POST", `{"user_id":"user-1"}`},
		{"/internal/vmctl/idle-check", "POST", ""},
		{"/internal/vmctl/reclaim", "POST", ""},
		{"/internal/vmctl/retention-plan", "GET", ""},
		{"/internal/vmctl/retention-shadow-plan", "GET", ""},
		{"/internal/vmctl/pulse", "GET", ""},
		{"/internal/vmctl/prune", "POST", ""},
	}

	for _, ep := range endpoints {
		t.Run(ep.path, func(t *testing.T) {
			var body io.Reader
			if ep.body != "" {
				body = strings.NewReader(ep.body)
			}
			req, _ := http.NewRequest(ep.method, srv.URL+ep.path, body)
			if ep.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			req.Header.Set("X-Internal-Caller", "true")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode == http.StatusMethodNotAllowed {
				t.Errorf("endpoint %s not registered (405)", ep.path)
			}
		})
	}
}

// --- VMManager Wiring Tests ---

// mockVMManager is a test double for the VMManager interface.
// It records lifecycle calls so tests can verify that the OwnershipRegistry
// properly delegates to the VM manager when one is configured.
type mockVMManager struct {
	boots       []VMManagerConfig
	stops       []string
	hibernates  []string
	resumes     []string
	reattaches  []string
	recovers    []string
	refreshes   []string
	recoverCfgs []VMManagerConfig
	refreshCfgs []VMManagerConfig
	destroys    []string
	tokens      map[string]string
	// Configurable responses
	bootResponse     *VMInstanceInfo
	bootError        error
	resumeResponse   *VMInstanceInfo
	resumeError      error
	reattachResponse *VMInstanceInfo
	reattachError    error
	recoverResponse  *VMInstanceInfo
	recoverError     error
	refreshResponse  *VMInstanceInfo
	refreshError     error
	getVMs           map[string]*VMInstanceInfo
	checkHealthOK    *bool
	checkHealthError error
	checkHealthCalls []string
}

func (m *mockVMManager) BootVM(cfg VMManagerConfig) (*VMInstanceInfo, error) {
	m.boots = append(m.boots, cfg)
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
	return nil
}

func (m *mockVMManager) HibernateVM(vmID string) error {
	m.hibernates = append(m.hibernates, vmID)
	return nil
}

func (m *mockVMManager) ResumeVM(vmID string) (*VMInstanceInfo, error) {
	m.resumes = append(m.resumes, vmID)
	if m.resumeError != nil {
		return nil, m.resumeError
	}
	if m.resumeResponse != nil {
		return m.resumeResponse, nil
	}
	return &VMInstanceInfo{HostURL: "http://127.0.0.1:9002", Epoch: 1, Healthy: true, State: "running"}, nil
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

func (m *mockVMManager) RecoverVM(vmID string, cfg VMManagerConfig) (*VMInstanceInfo, error) {
	m.recovers = append(m.recovers, vmID)
	m.recoverCfgs = append(m.recoverCfgs, cfg)
	if m.recoverError != nil {
		return nil, m.recoverError
	}
	if m.recoverResponse != nil {
		return m.recoverResponse, nil
	}
	return &VMInstanceInfo{HostURL: "http://127.0.0.1:9003", Epoch: 2, Healthy: true, State: "running"}, nil
}

func (m *mockVMManager) RefreshVM(vmID string, cfg VMManagerConfig) (*VMInstanceInfo, error) {
	m.refreshes = append(m.refreshes, vmID)
	m.refreshCfgs = append(m.refreshCfgs, cfg)
	if m.refreshError != nil {
		return nil, m.refreshError
	}
	if m.refreshResponse != nil {
		return m.refreshResponse, nil
	}
	return &VMInstanceInfo{HostURL: "http://127.0.0.1:9004", Epoch: 3, Healthy: true, State: "running"}, nil
}

func (m *mockVMManager) DestroyVMState(vmID string) error {
	m.destroys = append(m.destroys, vmID)
	return nil
}

func (m *mockVMManager) GetVM(vmID string) *VMInstanceInfo {
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
		_, _ = w.Write([]byte(`{"sandbox_id":"vm-existing-old-account","status":"imported"}`))
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
	own := &VMOwnership{
		VMID:         "vm-existing-old-account",
		UserID:       "user-old-account",
		DesktopID:    PrimaryDesktopID,
		Kind:         VMKindInteractive,
		SandboxURL:   "http://127.0.0.1:9001",
		State:        VMStateActive,
		CreatedAt:    now,
		LastActiveAt: now,
		Epoch:        3,
		Published:    true,
	}
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
	own := &VMOwnership{
		VMID:         "vm-stale-active",
		UserID:       "user-old-account",
		DesktopID:    PrimaryDesktopID,
		Kind:         VMKindInteractive,
		SandboxURL:   "http://127.0.0.1:9001",
		State:        VMStateActive,
		CreatedAt:    now,
		LastActiveAt: now,
		Epoch:        3,
		Published:    true,
	}
	reg.mu.Lock()
	reg.ownerships[ownershipKey(own.UserID, own.DesktopID)] = own
	reg.vmByID[own.VMID] = own
	reg.mu.Unlock()

	mock := &mockVMManager{
		getVMs: map[string]*VMInstanceInfo{
			own.VMID: {
				HostURL:       own.SandboxURL,
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
	if resolved.SandboxURL != "http://127.0.0.1:9044" {
		t.Fatalf("SandboxURL = %q, want recovered host URL", resolved.SandboxURL)
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
	own := &VMOwnership{
		VMID:         "vm-busy-active",
		UserID:       "user-busy",
		DesktopID:    PrimaryDesktopID,
		Kind:         VMKindInteractive,
		SandboxURL:   "http://127.0.0.1:9001",
		State:        VMStateActive,
		CreatedAt:    now.Add(-time.Hour),
		LastActiveAt: now.Add(-time.Minute),
		Epoch:        3,
		Published:    true,
	}
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
	if resolved.SandboxURL != "http://127.0.0.1:9009" {
		t.Fatalf("SandboxURL = %q, want current manager host URL", resolved.SandboxURL)
	}
	if resolved.Epoch != own.Epoch {
		t.Fatalf("Epoch = %d, want %d", resolved.Epoch, own.Epoch)
	}
}

func TestOwnershipRegistry_ResolvePreservesPendingActiveBoot(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	now := time.Now()
	own := &VMOwnership{
		VMID:         "vm-pending-active",
		UserID:       "user-pending",
		DesktopID:    PrimaryDesktopID,
		Kind:         VMKindInteractive,
		SandboxURL:   "http://127.0.0.1:9001",
		State:        VMStateActive,
		CreatedAt:    now.Add(-time.Hour),
		LastActiveAt: now.Add(-time.Minute),
		Epoch:        3,
		Published:    true,
	}
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
	if resolved.SandboxURL != "http://127.0.0.1:9010" {
		t.Fatalf("SandboxURL = %q, want current pending host URL", resolved.SandboxURL)
	}
	if resolved.Epoch != own.Epoch {
		t.Fatalf("Epoch = %d, want %d", resolved.Epoch, own.Epoch)
	}
}

func TestOwnershipRegistry_ResolveStartsActiveOwnershipMissingFromManager(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	now := time.Now().Add(-time.Minute)
	own := &VMOwnership{
		VMID:         "vm-missing-manager-instance",
		UserID:       "user-old-account",
		DesktopID:    PrimaryDesktopID,
		Kind:         VMKindInteractive,
		SandboxURL:   "http://127.0.0.1:9001",
		State:        VMStateActive,
		CreatedAt:    now,
		LastActiveAt: now,
		Epoch:        3,
		Published:    true,
	}
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
	if resolved.SandboxURL != "http://127.0.0.1:9045" {
		t.Fatalf("SandboxURL = %q, want restarted host URL", resolved.SandboxURL)
	}
}

func TestOwnershipRegistry_ResolveRecoversFailedManagerInstanceForHibernatedDesktop(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	now := time.Now().Add(-time.Hour)
	own := &VMOwnership{
		VMID:         "vm-failed-cold-resume",
		UserID:       "user-existing-account",
		DesktopID:    PrimaryDesktopID,
		Kind:         VMKindInteractive,
		SandboxURL:   "http://127.0.0.1:9001",
		State:        VMStateHibernated,
		CreatedAt:    now,
		LastActiveAt: now,
		Epoch:        7,
		Published:    true,
		StoppedBy:    "idle",
	}
	reg.mu.Lock()
	reg.ownerships[ownershipKey(own.UserID, own.DesktopID)] = own
	reg.vmByID[own.VMID] = own
	reg.mu.Unlock()

	mock := &mockVMManager{
		resumeError: fmt.Errorf("vm vm-failed-cold-resume cannot be resumed (state=failed)"),
		getVMs: map[string]*VMInstanceInfo{
			own.VMID: {
				HostURL: own.SandboxURL,
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
	if len(mock.resumes) != 1 || mock.resumes[0] != own.VMID {
		t.Fatalf("resumes = %+v, want [%s]", mock.resumes, own.VMID)
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
	if resolved.SandboxURL != "http://127.0.0.1:9046" {
		t.Fatalf("SandboxURL = %q, want recovered host URL", resolved.SandboxURL)
	}
	if resolved.Epoch != 8 {
		t.Fatalf("Epoch = %d, want 8", resolved.Epoch)
	}
	if resolved.State != VMStateActive || resolved.StoppedBy != "" {
		t.Fatalf("resolved state=%s stopped_by=%q, want active/empty", resolved.State, resolved.StoppedBy)
	}
}

func TestOwnershipRegistry_DelegatesBootToVMManager(t *testing.T) {
	// When a VMManager is set, ResolveOrAssign should boot a real VM
	// and use the returned HostURL instead of the static sandbox URL base.
	mock := &mockVMManager{
		bootResponse: &VMInstanceInfo{
			HostURL: "http://127.0.0.1:9042",
			Epoch:   7,
			Healthy: true,
			State:   "running",
		},
	}

	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetVMManager(mock)

	own, err := reg.ResolveOrAssign("user-with-vm")
	if err != nil {
		t.Fatalf("ResolveOrAssign: %v", err)
	}

	// Verify the VM manager was called to boot.
	if len(mock.boots) != 1 {
		t.Fatalf("expected 1 BootVM call, got %d", len(mock.boots))
	}
	if mock.boots[0].VMID != own.VMID {
		t.Errorf("expected boot VMID %s, got %s", own.VMID, mock.boots[0].VMID)
	}

	// Verify the sandbox URL came from the VM manager response.
	if own.SandboxURL != "http://127.0.0.1:9042" {
		t.Errorf("expected sandbox URL from VM manager, got %s", own.SandboxURL)
	}

	// Verify epoch came from the VM manager response.
	if own.Epoch != 7 {
		t.Errorf("expected epoch 7 from VM manager, got %d", own.Epoch)
	}
}

func TestOwnershipRegistry_DelegatesStopToVMManager(t *testing.T) {
	mock := &mockVMManager{}
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetVMManager(mock)

	own, _ := reg.ResolveOrAssign("user-stop-vm")
	if err := reg.StopVM("user-stop-vm"); err != nil {
		t.Fatalf("StopVM: %v", err)
	}

	if len(mock.stops) != 1 {
		t.Fatalf("expected 1 StopVM call, got %d", len(mock.stops))
	}
	if mock.stops[0] != own.VMID {
		t.Errorf("expected stop VMID %s, got %s", own.VMID, mock.stops[0])
	}
}

func TestOwnershipRegistry_DelegatesHibernateToVMManager(t *testing.T) {
	mock := &mockVMManager{}
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetVMManager(mock)

	_, _ = reg.ResolveOrAssign("user-hibernate-vm")
	if err := reg.HibernateVM("user-hibernate-vm"); err != nil {
		t.Fatalf("HibernateVM: %v", err)
	}

	if len(mock.hibernates) != 1 {
		t.Fatalf("expected 1 HibernateVM call, got %d", len(mock.hibernates))
	}
}

func TestOwnershipRegistry_DelegatesResumeToVMManager(t *testing.T) {
	mock := &mockVMManager{
		resumeResponse: &VMInstanceInfo{
			HostURL: "http://127.0.0.1:9043",
			Epoch:   5,
			Healthy: true,
			State:   "running",
		},
	}
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetVMManager(mock)

	_, _ = reg.ResolveOrAssign("user-resume-vm")
	_ = reg.HibernateVM("user-resume-vm")

	own, err := reg.ResumeVM("user-resume-vm")
	if err != nil {
		t.Fatalf("ResumeVM: %v", err)
	}

	if len(mock.resumes) != 1 {
		t.Fatalf("expected 1 ResumeVM call, got %d", len(mock.resumes))
	}

	// Verify the sandbox URL was updated from the resume response.
	if own.SandboxURL != "http://127.0.0.1:9043" {
		t.Errorf("expected sandbox URL from resume response, got %s", own.SandboxURL)
	}
}

func TestOwnershipRegistry_DelegatesRecoverToVMManager(t *testing.T) {
	mock := &mockVMManager{
		recoverResponse: &VMInstanceInfo{
			HostURL: "http://127.0.0.1:9044",
			Epoch:   99,
			Healthy: true,
			State:   "running",
		},
	}
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetVMManager(mock)

	_, _ = reg.ResolveOrAssign("user-recover-vm")
	_ = reg.MarkUnhealthy("user-recover-vm")

	own, err := reg.RecoverVM("user-recover-vm")
	if err != nil {
		t.Fatalf("RecoverVM: %v", err)
	}

	if len(mock.recovers) != 1 {
		t.Fatalf("expected 1 RecoverVM call, got %d", len(mock.recovers))
	}
	if len(mock.recoverCfgs) != 1 {
		t.Fatalf("expected 1 RecoverVM config, got %d", len(mock.recoverCfgs))
	}
	recoverCfg := mock.recoverCfgs[0]
	if recoverCfg.ComputerKind != "active" || recoverCfg.OwnerID != "user-recover-vm" || recoverCfg.DesktopID != PrimaryDesktopID {
		t.Fatalf("recover config identity = %+v, want active ownership identity", recoverCfg)
	}

	// Verify the epoch and sandbox URL came from the recovery response.
	if own.Epoch != 99 {
		t.Errorf("expected epoch 99 from recover response, got %d", own.Epoch)
	}
	if own.SandboxURL != "http://127.0.0.1:9044" {
		t.Errorf("expected sandbox URL from recover response, got %s", own.SandboxURL)
	}
}

func TestOwnershipRegistry_RefreshActiveVMDelegatesToVMManager(t *testing.T) {
	mock := &mockVMManager{
		refreshResponse: &VMInstanceInfo{
			HostURL: "http://127.0.0.1:9045",
			Epoch:   100,
			Healthy: true,
			State:   "running",
		},
	}
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetVMManager(mock)

	_, _ = reg.ResolveOrAssign("user-refresh-vm")

	own, err := reg.RefreshVMForDesktop("user-refresh-vm", PrimaryDesktopID)
	if err != nil {
		t.Fatalf("RefreshVMForDesktop: %v", err)
	}
	if len(mock.refreshes) != 1 {
		t.Fatalf("expected 1 RefreshVM call, got %d", len(mock.refreshes))
	}
	if len(mock.refreshCfgs) != 1 {
		t.Fatalf("expected 1 RefreshVM config, got %d", len(mock.refreshCfgs))
	}
	refreshCfg := mock.refreshCfgs[0]
	if refreshCfg.ComputerKind != "active" || refreshCfg.OwnerID != "user-refresh-vm" || refreshCfg.DesktopID != PrimaryDesktopID {
		t.Fatalf("refresh config identity = %+v, want active ownership identity", refreshCfg)
	}
	if len(mock.recovers) != 0 {
		t.Fatalf("expected refresh to avoid crash-recovery path, got %d RecoverVM calls", len(mock.recovers))
	}
	if own.State != VMStateActive {
		t.Fatalf("state = %s, want active", own.State)
	}
	if own.Epoch != 100 {
		t.Fatalf("epoch = %d, want 100", own.Epoch)
	}
	if own.SandboxURL != "http://127.0.0.1:9045" {
		t.Fatalf("sandbox URL = %s", own.SandboxURL)
	}
}

func TestOwnershipRegistry_LiveSandboxURLSnapshotsDuringRefresh(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
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
			if _, err := reg.LiveSandboxURL("user-live-url-refresh", PrimaryDesktopID); err != nil {
				t.Errorf("LiveSandboxURL: %v", err)
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
			_ = own.SandboxURL
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
	reg.SetVMManager(mock)
	initial, err := reg.ResolveOrAssign("user-refresh-hibernated")
	if err != nil {
		t.Fatalf("ResolveOrAssign: %v", err)
	}
	mock.getVMs = map[string]*VMInstanceInfo{
		initial.VMID: {
			HostURL: initial.SandboxURL,
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
	if own.SandboxURL != "http://127.0.0.1:9047" {
		t.Fatalf("sandbox URL = %s, want boot response URL", own.SandboxURL)
	}
}

func TestOwnershipRegistry_DelegatesLogoutToVMManager(t *testing.T) {
	mock := &mockVMManager{}
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetVMManager(mock)

	_, _ = reg.ResolveOrAssign("user-logout-vm")
	_ = reg.LogoutVM("user-logout-vm")

	if len(mock.stops) != 1 {
		t.Fatalf("expected 1 StopVM call from logout, got %d", len(mock.stops))
	}
}

func TestOwnershipRegistry_DelegatesRemoveToVMManager(t *testing.T) {
	mock := &mockVMManager{}
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetVMManager(mock)

	_, _ = reg.ResolveOrAssign("user-remove-vm")
	_ = reg.RemoveOwnership("user-remove-vm")

	if len(mock.stops) != 1 {
		t.Fatalf("expected 1 StopVM call from remove, got %d", len(mock.stops))
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

func TestOwnershipRegistry_NoVMManagerUsesHostProcessMode(t *testing.T) {
	// Without a VMManager, ResolveOrAssign should use the static sandbox URL.
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")

	own, err := reg.ResolveOrAssign("user-no-vm")
	if err != nil {
		t.Fatalf("ResolveOrAssign: %v", err)
	}

	// Sandbox URL should be the static base URL.
	if own.SandboxURL != "http://127.0.0.1:8085" {
		t.Errorf("expected static sandbox URL in host-process mode, got %s", own.SandboxURL)
	}
}

func TestOwnershipRegistry_ResumeOnResolveWithVMManager(t *testing.T) {
	// When a user has a hibernated VM and resolves again, the VM should
	// be resumed through the manager with the per-VM sandbox URL.
	mock := &mockVMManager{
		resumeResponse: &VMInstanceInfo{
			HostURL: "http://127.0.0.1:9050",
			Epoch:   3,
			Healthy: true,
			State:   "running",
		},
	}
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetVMManager(mock)

	// First assign and hibernate.
	own1, _ := reg.ResolveOrAssign("user-resume-resolve")
	_ = reg.HibernateVM("user-resume-resolve")

	// Resolve again should resume the VM.
	own2, err := reg.ResolveOrAssign("user-resume-resolve")
	if err != nil {
		t.Fatalf("ResolveOrAssign after hibernate: %v", err)
	}

	// Same VM ID.
	if own1.VMID != own2.VMID {
		t.Errorf("expected same VM ID after resume, got %s and %s", own1.VMID, own2.VMID)
	}

	// Sandbox URL should be updated from the resume response.
	if own2.SandboxURL != "http://127.0.0.1:9050" {
		t.Errorf("expected sandbox URL from resume, got %s", own2.SandboxURL)
	}

	// Verify resume was called on the manager.
	if len(mock.resumes) != 1 {
		t.Fatalf("expected 1 ResumeVM call, got %d", len(mock.resumes))
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

	resolved, err := restarted.ResolveOrAssign("user-persist")
	if err != nil {
		t.Fatalf("ResolveOrAssign after restart: %v", err)
	}
	if resolved.VMID != own.VMID {
		t.Fatalf("resolved VMID = %s, want persisted %s", resolved.VMID, own.VMID)
	}
	if resolved.SandboxURL != "http://127.0.0.1:9099" {
		t.Fatalf("resolved SandboxURL = %s, want manager boot URL", resolved.SandboxURL)
	}
	if resolved.State != VMStateActive {
		t.Fatalf("resolved state = %s, want active", resolved.State)
	}
	if len(mock.resumes) != 1 {
		t.Fatalf("expected resume attempt before boot fallback, got %d", len(mock.resumes))
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
		_, _ = w.Write([]byte(`{"sandbox_id":"vm-reattach-old-token","status":"imported"}`))
	}))
	t.Cleanup(gateway.Close)

	rawToken := "vm-reattach-old-token:token-from-host-persistent-dir"
	now := time.Now()
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetGatewayURL(gateway.URL)
	own := &VMOwnership{
		VMID:         "vm-reattach-old-token",
		UserID:       "user-old-account",
		DesktopID:    PrimaryDesktopID,
		Kind:         VMKindInteractive,
		SandboxURL:   "http://127.0.0.1:9001",
		State:        VMStateStopped,
		CreatedAt:    now,
		LastActiveAt: now,
		Epoch:        3,
		Published:    true,
		StoppedBy:    "vmctl-restart",
	}
	reg.mu.Lock()
	reg.ownerships[ownershipKey(own.UserID, own.DesktopID)] = own
	reg.vmByID[own.VMID] = own
	reg.mu.Unlock()

	reg.SetVMManager(&mockVMManager{
		tokens: map[string]string{
			own.VMID: rawToken,
		},
	})

	select {
	case got := <-ensured:
		if got != rawToken {
			t.Fatalf("ensured raw token = %q, want %q", got, rawToken)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for reattached VM gateway credential reconciliation")
	}
}

// --- Gateway Token Issuance Tests ---

func TestIssueGatewayToken_Success(t *testing.T) {
	// Verify that issueGatewayToken calls the gateway's credential endpoint
	// and returns the credential value.
	credValue := "vm-test-123:changedplaceholder"
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/provider/v1/credentials/issue" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if got := r.Header.Get("X-Internal-Caller"); got != "true" {
			t.Errorf("expected X-Internal-Caller=true, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		// Mirror the real gateway CredentialResult JSON shape.
		resp := map[string]string{
			"SandboxID": "vm-test-123",
			"RawToken":  credValue,
			"ExpiresAt": "2025-01-01T00:00:00Z",
		}
		jsonData, _ := json.Marshal(resp)
		w.Write(jsonData)
	}))
	defer gateway.Close()

	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetGatewayURL(gateway.URL)

	token := reg.issueGatewayToken("vm-test-123")
	if token != credValue {
		t.Errorf("expected credential value %q, got %q", credValue, token)
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
			"sandbox_id": "vm-test-legacy",
			"raw_token":  credValue,
			"expires_at": "2025-01-01T00:00:00Z",
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

func TestSetGatewayURL(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetGatewayURL("http://gateway.test:8084")

	reg.mu.RLock()
	gwURL := reg.gatewayURL
	reg.mu.RUnlock()

	if gwURL != "http://gateway.test:8084" {
		t.Errorf("expected gateway URL http://gateway.test:8084, got %s", gwURL)
	}
}
