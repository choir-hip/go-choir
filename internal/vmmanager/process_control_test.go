package vmmanager

import (
	"os"
	"testing"
)

// vmctl reconciles every Firecracker process on the host against its
// registry (docs/archive/problems/node-b-oom-from-retained-qa-computers-2026-10-10.md).
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

// VMResidentBytes feeds vmctl's measured admission
// (docs/reports/vm-density-limits-2026-10-14.md, lever L1). Ways it could
// fail, written before the code: VmRSS is read in the wrong unit (kB, not
// bytes); a status without VmRSS (a kernel thread, a zombie) reports a
// bogus value instead of nothing; a process that exited between listing and
// reading fails the whole reading; a VM with two Firecracker pids reports
// one of them.
func TestVMResidentBytesFromStatus(t *testing.T) {
	status := map[int]string{
		10: "Name:\tfirecracker\nVmPeak:\t 9000000 kB\nVmRSS:\t 1236992 kB\nRssAnon:\t 1200000 kB\n",
		11: "Name:\tfirecracker\nVmRSS:\t    1024 kB\n",
		12: "Name:\tfirecracker\nState:\tZ (zombie)\n",
	}
	read := func(pid int) (string, error) {
		s, ok := status[pid]
		if !ok {
			return "", os.ErrNotExist
		}
		return s, nil
	}
	got := vmResidentBytesFrom(map[string][]int{
		"vm-a":    {10, 11},
		"vm-z":    {12},
		"vm-gone": {13},
	}, read)
	if got["vm-a"] != (1236992+1024)*1024 {
		t.Fatalf("vm-a = %d, want the sum of both pids in bytes", got["vm-a"])
	}
	if _, ok := got["vm-z"]; ok {
		t.Fatalf("a status without VmRSS reported %d", got["vm-z"])
	}
	if _, ok := got["vm-gone"]; ok {
		t.Fatalf("an exited process reported %d", got["vm-gone"])
	}
}
