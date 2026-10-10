package vmmanager

import (
	"testing"
)

// vmctl reconciles every Firecracker process on the host against its
// registry (docs/problems/node-b-oom-from-retained-qa-computers-2026-10-10.md).
// Failure modes pinned: a non-Firecracker process or one without --id is
// reported as a VM; either --id spelling is missed; reaping touches a VM
// the manager is running.
func TestFirecrackerVMIDsFromEntries(t *testing.T) {
	entries := []procCmdlineEntry{
		{pid: 10, data: []byte("/nix/store/x-firecracker-1.15.1/bin/firecracker\x00--api-sock\x00/run/a.sock\x00--id\x00vm-aaa\x00")},
		{pid: 11, data: []byte("/nix/store/x-firecracker-1.15.1/bin/firecracker\x00--id=candidate-fleet-bbb\x00")},
		{pid: 12, data: []byte("/usr/bin/grep\x00--id\x00vm-aaa\x00")},
		{pid: 13, data: []byte("/nix/store/x-firecracker/bin/firecracker\x00--api-sock\x00/run/c.sock\x00")},
		{pid: 14, data: []byte("/nix/store/x-firecracker/bin/firecracker\x00--id\x00vm-aaa\x00")},
	}
	got := firecrackerVMIDsFromEntries(entries)
	if len(got) != 2 || len(got["vm-aaa"]) != 2 || got["vm-aaa"][0] != 10 || got["vm-aaa"][1] != 14 || len(got["candidate-fleet-bbb"]) != 1 {
		t.Fatalf("vm ids = %v", got)
	}
}

func TestReapUnmanagedVMRefusesTrackedRunningVM(t *testing.T) {
	m := NewManager(ManagerConfig{StateDir: t.TempDir()})
	m.vms["vm-tracked"] = &VMInstance{Config: VMConfig{VMID: "vm-tracked"}, State: StateRunning}
	if err := m.ReapUnmanagedVM("vm-tracked"); err == nil {
		t.Fatal("reap of a tracked running VM succeeded")
	}
}
