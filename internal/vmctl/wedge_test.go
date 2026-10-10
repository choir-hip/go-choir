package vmctl

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Owner ruling 2026-10-10: a stranded computer recovers by itself
// (docs/problems/owner-computer-stranded-after-vmctl-restart-2026-10-10.md).
//
// Failure modes pinned here:
//   - a protected guest that refuses every connection is never stopped (the
//     owner had to click restart);
//   - a guest is stopped before the wedge window has passed;
//   - a busy guest that accepts connections is stopped;
//   - one accepted connection does not reset the window;
//   - a tracked running guest that refuses connections is never stopped;
//   - a held guest is stopped.

func wedgeRegistry(t *testing.T) (*OwnershipRegistry, *processControlVMManager, *time.Time, map[string]bool) {
	t.Helper()
	mgr := newProcessControlVMManager()
	mgr.reattachError = errors.New("guest health check failed")
	reg, now := hostCapacityRegistry(t, mgr)
	reg.SetWarmnessPolicyConfig(WarmnessPolicyConfig{AlwaysOnUserIDs: map[string]bool{"owner": true}})
	refusing := map[string]bool{}
	reg.guestNetworkProbe = func(url string) bool { return !refusing[url] }
	return reg, mgr, now, refusing
}

func TestReconcileStopsProtectedGuestRefusingConnectionsForWedgeWindow(t *testing.T) {
	reg, mgr, now, refusing := wedgeRegistry(t)
	const url = "http://10.0.0.3:8085"
	addOwnership(reg, &VMOwnership{VMID: "vm-owner", UserID: "owner", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive,
		State: VMStateStopped, StoppedBy: "vmctl-restart", ComputerURL: url})
	mgr.live["vm-owner"] = []int{1}
	refusing[url] = true

	reg.ReconcileVMProcesses(context.Background(), allowAllRoutes)
	*now = now.Add(wedgedAfterDefault - time.Second)
	res := reg.ReconcileVMProcesses(context.Background(), allowAllRoutes)
	if len(mgr.reaped) != 0 || res.Wedged != 0 {
		t.Fatalf("stopped inside the wedge window: %v", mgr.reaped)
	}

	*now = now.Add(2 * time.Second)
	res = reg.ReconcileVMProcesses(context.Background(), allowAllRoutes)
	if res.Wedged != 1 || len(mgr.reaped) != 1 || mgr.reaped[0] != "vm-owner" {
		t.Fatalf("wedged protected guest not stopped: result=%+v reaped=%v", res, mgr.reaped)
	}
	if own := reg.GetOwnershipByVMID("vm-owner"); own == nil || own.State != VMStateStopped || own.StoppedBy != stoppedByWedged {
		t.Fatalf("ownership after wedge stop = %+v", own)
	}
}

func TestReconcileKeepsBusyGuestAndResetsWindowOnAnyConnection(t *testing.T) {
	reg, mgr, now, refusing := wedgeRegistry(t)
	const url = "http://10.0.0.3:8085"
	addOwnership(reg, &VMOwnership{VMID: "vm-owner", UserID: "owner", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive,
		State: VMStateStopped, StoppedBy: "vmctl-restart", ComputerURL: url})
	mgr.live["vm-owner"] = []int{1}

	// Busy: accepts connections, fails reattach, for a whole day.
	for i := 0; i < 10; i++ {
		reg.ReconcileVMProcesses(context.Background(), allowAllRoutes)
		*now = now.Add(3 * time.Hour)
	}
	if len(mgr.reaped) != 0 {
		t.Fatalf("busy guest that accepts connections was stopped: %v", mgr.reaped)
	}

	// Refuses, then accepts once, then refuses again: the window restarts.
	refusing[url] = true
	reg.ReconcileVMProcesses(context.Background(), allowAllRoutes)
	*now = now.Add(4 * time.Minute)
	refusing[url] = false
	reg.ReconcileVMProcesses(context.Background(), allowAllRoutes)
	refusing[url] = true
	*now = now.Add(time.Minute)
	reg.ReconcileVMProcesses(context.Background(), allowAllRoutes)
	*now = now.Add(4 * time.Minute)
	reg.ReconcileVMProcesses(context.Background(), allowAllRoutes)
	if len(mgr.reaped) != 0 {
		t.Fatalf("an accepted connection did not reset the wedge window: %v", mgr.reaped)
	}
}

func TestReconcileStopsTrackedRunningGuestRefusingConnections(t *testing.T) {
	reg, mgr, now, refusing := wedgeRegistry(t)
	const url = "http://10.0.0.6:8085"
	addOwnership(reg, &VMOwnership{VMID: "vm-tracked", UserID: "someone", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive,
		State: VMStateActive, ComputerURL: url})
	mgr.live["vm-tracked"] = []int{5}
	mgr.getVMs["vm-tracked"] = &VMInstanceInfo{HostURL: url, State: "running"}
	refusing[url] = true

	reg.ReconcileVMProcesses(context.Background(), allowAllRoutes)
	*now = now.Add(wedgedAfterDefault)
	res := reg.ReconcileVMProcesses(context.Background(), allowAllRoutes)
	if res.Wedged != 1 || len(mgr.stops) != 1 || mgr.stops[0] != "vm-tracked" {
		t.Fatalf("wedged tracked guest not stopped: result=%+v stops=%v", res, mgr.stops)
	}
	if own := reg.GetOwnershipByVMID("vm-tracked"); own == nil || own.State != VMStateStopped || own.StoppedBy != stoppedByWedged {
		t.Fatalf("ownership after wedge stop = %+v", own)
	}
}

func TestReconcileNeverStopsHeldGuest(t *testing.T) {
	reg, mgr, now, refusing := wedgeRegistry(t)
	const url = "http://10.0.0.7:8085"
	addOwnership(reg, &VMOwnership{VMID: "vm-held", UserID: "owner", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive,
		State: VMStateStopped, StoppedBy: "vmctl-restart", ComputerURL: url, HoldStatus: &MaintenanceHold{}})
	mgr.live["vm-held"] = []int{9}
	refusing[url] = true

	reg.ReconcileVMProcesses(context.Background(), allowAllRoutes)
	*now = now.Add(time.Hour)
	reg.ReconcileVMProcesses(context.Background(), allowAllRoutes)
	if len(mgr.reaped) != 0 {
		t.Fatalf("held guest stopped: %v", mgr.reaped)
	}
}
