package vmctl

import (
	"path/filepath"
	"sync"
	"testing"
)

// Dead snapshots, vmctl side (design note items 44-47).
//
// Ways this could fail, written first:
//   - a stop leaves the hibernated computer's snapshot on disk (observed on
//     staging), because the manager's StopVM is a no-op for a hibernated
//     VM, or is not called at all when the manager lost track of it after a
//     vmctl restart;
//   - logout, ownership removal or a hold turning a hibernated computer
//     stopped leave the snapshot behind;
//   - a hibernate itself discards the snapshot it just wrote;
//   - the start sweep deletes the snapshot of a hibernated computer (held
//     or not), or runs over an empty in-memory registry and deletes every
//     snapshot.

type discardMockVMManager struct {
	*snapshotMockVMManager
	discardMu sync.Mutex
	discards  []string
	sweeps    int
	sweepKeep func(vmID string) bool
}

func (m *discardMockVMManager) DiscardSnapshot(vmID, reason string) bool {
	m.discardMu.Lock()
	defer m.discardMu.Unlock()
	m.discards = append(m.discards, vmID)
	return true
}

func (m *discardMockVMManager) SweepOrphanSnapshots(keep func(vmID string) bool) []string {
	m.discardMu.Lock()
	defer m.discardMu.Unlock()
	m.sweeps++
	m.sweepKeep = keep
	return nil
}

func newDiscardMock() *discardMockVMManager {
	return &discardMockVMManager{snapshotMockVMManager: &snapshotMockVMManager{mockVMManager: &mockVMManager{getVMs: make(map[string]*VMInstanceInfo)}}}
}

func hibernatedRegistry(t *testing.T, mock *discardMockVMManager, userID string) (*OwnershipRegistry, *VMOwnership) {
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
	if len(mock.discards) != 0 {
		t.Fatalf("a hibernate discarded the snapshot it wrote: %v", mock.discards)
	}
	return reg, initial
}

func TestStopDiscardsHibernatedComputersSnapshot(t *testing.T) {
	t.Run("manager tracks the hibernated VM", func(t *testing.T) {
		mock := newDiscardMock()
		reg, initial := hibernatedRegistry(t, mock, "user-stop-tracked")
		if err := reg.StopVMForDesktop("user-stop-tracked", PrimaryDesktopID); err != nil {
			t.Fatalf("StopVMForDesktop: %v", err)
		}
		if len(mock.discards) != 1 || mock.discards[0] != initial.VMID {
			t.Fatalf("discards = %v, want [%s]", mock.discards, initial.VMID)
		}
	})
	t.Run("manager lost the VM (vmctl restart)", func(t *testing.T) {
		mock := newDiscardMock()
		reg, initial := hibernatedRegistry(t, mock, "user-stop-untracked")
		delete(mock.getVMs, initial.VMID)
		stopsBefore := len(mock.stops)
		if err := reg.StopVMForDesktop("user-stop-untracked", PrimaryDesktopID); err != nil {
			t.Fatalf("StopVMForDesktop: %v", err)
		}
		if len(mock.stops) != stopsBefore {
			t.Fatalf("manager StopVM called for an untracked hibernated VM")
		}
		if len(mock.discards) != 1 || mock.discards[0] != initial.VMID {
			t.Fatalf("discards = %v, want [%s] even though the manager's StopVM was not called", mock.discards, initial.VMID)
		}
	})
	t.Run("logout", func(t *testing.T) {
		mock := newDiscardMock()
		reg, initial := hibernatedRegistry(t, mock, "user-logout")
		if err := reg.LogoutVM("user-logout"); err != nil {
			t.Fatalf("LogoutVM: %v", err)
		}
		if len(mock.discards) != 1 || mock.discards[0] != initial.VMID {
			t.Fatalf("discards = %v, want [%s]", mock.discards, initial.VMID)
		}
	})
	t.Run("ownership removed", func(t *testing.T) {
		mock := newDiscardMock()
		reg, initial := hibernatedRegistry(t, mock, "user-remove")
		if err := reg.RemoveOwnership("user-remove"); err != nil {
			t.Fatalf("RemoveOwnership: %v", err)
		}
		if len(mock.discards) != 1 || mock.discards[0] != initial.VMID {
			t.Fatalf("discards = %v, want [%s]", mock.discards, initial.VMID)
		}
	})
	t.Run("hold turns a hibernated computer stopped on resolve", func(t *testing.T) {
		mock := newDiscardMock()
		reg, initial := hibernatedRegistry(t, mock, "user-held")
		reg.mu.Lock()
		reg.vmByID[initial.VMID].HoldStatus = &MaintenanceHold{}
		reg.mu.Unlock()
		own, err := reg.ResolveOrAssign("user-held")
		if err != nil {
			t.Fatalf("ResolveOrAssign: %v", err)
		}
		if own.State != VMStateStopped {
			t.Fatalf("held computer state = %s, want stopped", own.State)
		}
		if len(mock.discards) != 1 || mock.discards[0] != initial.VMID {
			t.Fatalf("discards = %v, want [%s]", mock.discards, initial.VMID)
		}
	})
}

func TestSnapshotSweepKeepsOnlyHibernatedComputers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ownerships.json")
	mock := newDiscardMock()
	first := NewOwnershipRegistry("http://127.0.0.1:8085")
	if err := first.SetPersistencePath(path); err != nil {
		t.Fatal(err)
	}
	first.SetVMManager(mock)
	if got := first.SweepOrphanSnapshots(); got != nil || mock.sweeps != 0 {
		t.Fatalf("sweep ran without a loaded registry file (sweeps=%d)", mock.sweeps)
	}
	ids := map[string]string{}
	for _, user := range []string{"user-hib", "user-hib-held", "user-stopped", "user-active"} {
		own, err := first.ResolveOrAssign(user)
		if err != nil {
			t.Fatalf("ResolveOrAssign %s: %v", user, err)
		}
		ids[user] = own.VMID
		mock.getVMs[own.VMID] = &VMInstanceInfo{HostURL: own.ComputerURL, Epoch: own.Epoch, Healthy: true, State: "running"}
	}
	for _, user := range []string{"user-hib", "user-hib-held"} {
		if err := first.HibernateVM(user); err != nil {
			t.Fatalf("HibernateVM %s: %v", user, err)
		}
	}
	first.mu.Lock()
	first.vmByID[ids["user-hib-held"]].HoldStatus = &MaintenanceHold{}
	first.saveLocked()
	first.mu.Unlock()
	if err := first.StopVM("user-stopped"); err != nil {
		t.Fatal(err)
	}

	// vmctl restart: a new registry loads the durable file.
	restarted := NewOwnershipRegistry("http://127.0.0.1:8085")
	if err := restarted.SetPersistencePath(path); err != nil {
		t.Fatal(err)
	}
	sweeper := newDiscardMock()
	restarted.SetVMManager(sweeper)
	restarted.SweepOrphanSnapshots()
	if sweeper.sweeps != 1 || sweeper.sweepKeep == nil {
		t.Fatalf("sweeps = %d, want one with a keep set", sweeper.sweeps)
	}
	want := map[string]bool{
		ids["user-hib"]:      true,
		ids["user-hib-held"]: true,
		ids["user-stopped"]:  false,
		ids["user-active"]:   false, // running at restart: no committed snapshot to keep
		"vm-unknown":         false,
	}
	for vmID, keep := range want {
		if got := sweeper.sweepKeep(vmID); got != keep {
			t.Fatalf("keep(%s) = %v, want %v", vmID, got, keep)
		}
	}
}

func TestSnapshotSweepSkippedWithoutPersistence(t *testing.T) {
	mock := newDiscardMock()
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetVMManager(mock)
	if _, err := reg.ResolveOrAssign("user-mem"); err != nil {
		t.Fatal(err)
	}
	reg.SweepOrphanSnapshots()
	if mock.sweeps != 0 {
		t.Fatal("an in-memory registry must never drive the sweep")
	}
}
