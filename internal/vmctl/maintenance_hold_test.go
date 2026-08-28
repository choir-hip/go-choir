package vmctl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMaintenanceHoldSetClearIdempotent(t *testing.T) {
	reg := NewOwnershipRegistry("")
	key := ownershipKey("user-hold", PrimaryDesktopID)
	own := &VMOwnership{
		VMID:       "vm-hold-1",
		ComputerID: "computer-hold-1",
		UserID:     "user-hold",
		DesktopID:  PrimaryDesktopID,
		State:      VMStateStopped,
	}
	reg.ownerships[key] = own

	if own.IsHeld() {
		t.Fatal("fresh ownership should not be held")
	}
	if err := reg.SetHold("computer-hold-1", "maintenance", "recovery"); err != nil {
		t.Fatalf("SetHold: %v", err)
	}
	if !own.IsHeld() {
		t.Fatal("ownership should be held after SetHold")
	}
	if own.HoldStatus.Reason != "maintenance" || own.HoldStatus.HeldBy != "recovery" {
		t.Fatalf("unexpected hold: %+v", own.HoldStatus)
	}
	if err := reg.SetHold("computer-hold-1", "maintenance", "recovery"); err != nil {
		t.Fatalf("SetHold idempotent: %v", err)
	}
	if err := reg.SetHold("computer-missing", "maintenance", "recovery"); err == nil {
		t.Fatal("SetHold on unknown computer should error")
	}
	if err := reg.ClearHold("computer-hold-1"); err != nil {
		t.Fatalf("ClearHold: %v", err)
	}
	if own.IsHeld() {
		t.Fatal("ownership should not be held after ClearHold")
	}
}

func TestMaintenanceHoldRefreshRefused(t *testing.T) {
	reg := NewOwnershipRegistry("")
	key := ownershipKey("user-hold", PrimaryDesktopID)
	reg.ownerships[key] = &VMOwnership{
		VMID:       "vm-hold-1",
		ComputerID: "computer-hold-1",
		UserID:     "user-hold",
		DesktopID:  PrimaryDesktopID,
		State:      VMStateActive,
		HoldStatus: &MaintenanceHold{Reason: "maintenance", HeldBy: "recovery"},
	}
	reg.SetVMManager(&mockVMManager{})

	_, err := reg.RefreshVMForDesktop("user-hold", PrimaryDesktopID)
	if err == nil || !strings.Contains(err.Error(), "maintenance hold") {
		t.Fatalf("Refresh should refuse a held computer, got err=%v", err)
	}
}

func TestMaintenanceHoldEnsureReadyReturnsHeld(t *testing.T) {
	reg := NewOwnershipRegistry("")
	mgr := &mockVMManager{}
	own := &VMOwnership{
		VMID:       "vm-hold-1",
		ComputerID: "computer-hold-1",
		UserID:     "user-hold",
		DesktopID:  PrimaryDesktopID,
		State:      VMStateActive,
		HoldStatus: &MaintenanceHold{Reason: "maintenance", HeldBy: "recovery"},
	}

	info, err := reg.ensureActiveVMReady(own, mgr)
	if err != nil {
		t.Fatalf("ensureActiveVMReady held: %v", err)
	}
	if info == nil || info.State != "held" {
		t.Fatalf("expected held state, got %+v", info)
	}
	if len(mgr.recovers) != 0 {
		t.Fatalf("held computer must not recover on readiness check, recovers=%v", mgr.recovers)
	}
}

func TestMaintenanceHoldResolveDoesNotAutoStart(t *testing.T) {
	reg := NewOwnershipRegistry("")
	key := ownershipKey("user-hold", PrimaryDesktopID)
	mgr := &mockVMManager{}
	reg.SetVMManager(mgr)
	reg.ownerships[key] = &VMOwnership{
		VMID:       "vm-hold-1",
		ComputerID: "computer-hold-1",
		UserID:     "user-hold",
		DesktopID:  PrimaryDesktopID,
		State:      VMStateStopped,
		HoldStatus: &MaintenanceHold{Reason: "maintenance", HeldBy: "recovery"},
	}

	result, err := reg.resolveDesktopContext(context.Background(), "user-hold", PrimaryDesktopID, true, "")
	if err != nil {
		t.Fatalf("resolve held: %v", err)
	}
	if result == nil {
		t.Fatal("resolve held returned nil ownership")
	}
	if len(mgr.boots) != 0 || len(mgr.recovers) != 0 {
		t.Fatalf("held computer must not boot/recover on resolve, boots=%v recovers=%v", mgr.boots, mgr.recovers)
	}
}

func TestHeldServingOwnershipSkipsResolveReadinessCheck(t *testing.T) {
	reg := NewOwnershipRegistry("")
	mgr := &mockVMManager{}
	reg.SetVMManager(mgr)
	own := &VMOwnership{
		VMID:        "vm-hold-1",
		ComputerID:  "computer-hold-1",
		UserID:      "user-hold",
		DesktopID:   PrimaryDesktopID,
		State:       VMStateActive,
		ComputerURL: "http://10.0.0.1:8085",
		HoldStatus:  &MaintenanceHold{Reason: "maintenance", HeldBy: "recovery"},
	}
	// The mock VM is not ready (GetVM nil), so without the held short-circuit
	// this would return true and register a pendingWaiter that a concurrent
	// browser probe blocks on until the 15s abort. A held, serving computer
	// must skip the readiness wait and resolve instantly.
	if activeOwnershipNeedsReadinessCheck(own, mgr) {
		t.Fatal("held, serving computer must skip the resolve readiness wait")
	}
}

func TestMaintenanceHoldRefusesPlainRecover(t *testing.T) {
	reg := NewOwnershipRegistry("")
	key := ownershipKey("user-hold", PrimaryDesktopID)
	reg.ownerships[key] = &VMOwnership{
		VMID:       "vm-hold-1",
		ComputerID: "computer-hold-1",
		UserID:     "user-hold",
		DesktopID:  PrimaryDesktopID,
		State:      VMStateStopped,
		HoldStatus: &MaintenanceHold{Reason: "maintenance", HeldBy: "recovery"},
	}
	reg.SetVMManager(&mockVMManager{})

	_, err := reg.RecoverVMForDesktop("user-hold", PrimaryDesktopID)
	if err == nil || !strings.Contains(err.Error(), "maintenance hold") {
		t.Fatalf("plain recover must refuse a held computer, got %v", err)
	}
}

func TestMaintenanceHoldAuthorizesReplayOnlyRecoveryBoot(t *testing.T) {
	reg := NewOwnershipRegistry("")
	key := ownershipKey("user-hold", PrimaryDesktopID)
	reg.ownerships[key] = &VMOwnership{
		VMID:       "vm-hold-1",
		ComputerID: "computer-hold-1",
		UserID:     "user-hold",
		DesktopID:  PrimaryDesktopID,
		State:      VMStateStopped,
		HoldStatus: &MaintenanceHold{Reason: "maintenance", HeldBy: "recovery"},
	}
	mock := &mockVMManager{}
	reg.SetVMManager(mock)

	own, err := reg.RecoverVMForDesktopMaintenance("user-hold", PrimaryDesktopID, true)
	if err != nil {
		t.Fatalf("authorized maintenance recovery: %v", err)
	}
	if len(mock.boots) != 1 {
		t.Fatalf("expected exactly one boot, got %d", len(mock.boots))
	}
	boot := mock.boots[0]
	if !boot.RecoveryReplayOnly || !boot.MaintenanceHold {
		t.Fatalf("recovery boot must carry replay-only + maintenance hold, got %+v", boot)
	}
	if own.State != VMStateActive {
		t.Fatalf("recovered ownership should be active, got %s", own.State)
	}
}

func TestMaintenanceServeBootsHeldComputerWithGuestHold(t *testing.T) {
	reg := NewOwnershipRegistry("")
	own := &VMOwnership{
		VMID: "vm-hold-serve", ComputerID: "computer-hold-serve",
		UserID: "user-hold-serve", DesktopID: PrimaryDesktopID,
		State: VMStateStopped, HoldStatus: &MaintenanceHold{Reason: "maintenance", HeldBy: "recovery"},
	}
	reg.ownerships[ownershipKey(own.UserID, own.DesktopID)] = own
	mock := &mockVMManager{}
	reg.SetVMManager(mock)
	handler := NewHandler(reg)

	request := httptest.NewRequest(http.MethodPost, "/internal/vmctl/maintenance-serve", strings.NewReader(
		`{"user_id":"user-hold-serve","desktop_id":"primary"}`,
	))
	request.Header.Set("X-Internal-Caller", "true")
	response := httptest.NewRecorder()
	handler.HandleMaintenanceServe(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("maintenance serve status=%d body=%s", response.Code, response.Body.String())
	}
	if len(mock.boots) != 1 {
		t.Fatalf("expected one maintenance boot, got %d", len(mock.boots))
	}
	if !mock.boots[0].MaintenanceHold || mock.boots[0].RecoveryReplayOnly {
		t.Fatalf("maintenance serve must retain guest hold without replay-only mode: %+v", mock.boots[0])
	}
	if own.State != VMStateActive || !own.IsHeld() {
		t.Fatalf("maintenance serve must leave ownership active and held: %+v", own)
	}
}
