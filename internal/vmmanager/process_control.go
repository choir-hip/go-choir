package vmmanager

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Process control for vmctl's host-capacity reconcile
// (docs/archive/problems/node-b-oom-from-retained-qa-computers-2026-10-10.md):
// vmctl must see every Firecracker process on the host, power off the ones
// it does not track, and rank VMs for the kernel OOM killer.

// launchOOMScoreAdj is every VM's OOM priority at launch. Firecracker
// otherwise inherits vmctl's strong protection; vmctl lowers it for
// protected computers once it knows their class.
const launchOOMScoreAdj = 500

// LiveFirecrackerVMs maps VM id to the Firecracker pids running it.
func (m *Manager) LiveFirecrackerVMs() map[string][]int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	cmdlines := make([]procCmdlineEntry, 0)
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 || !entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err != nil {
			continue
		}
		cmdlines = append(cmdlines, procCmdlineEntry{pid: pid, data: data})
	}
	return firecrackerVMIDsFromEntries(cmdlines)
}

func firecrackerVMIDsFromEntries(entries []procCmdlineEntry) map[string][]int {
	out := map[string][]int{}
	for _, entry := range entries {
		if entry.pid <= 0 {
			continue
		}
		vmID := firecrackerVMIDFromCmdline(entry.data)
		if vmID == "" {
			continue
		}
		out[vmID] = append(out[vmID], entry.pid)
	}
	for vmID := range out {
		sort.Ints(out[vmID])
	}
	return out
}

func firecrackerVMIDFromCmdline(data []byte) string {
	args := processCmdlineArgs(data)
	if len(args) == 0 || !strings.Contains(filepath.Base(args[0]), "firecracker") {
		return ""
	}
	for i, arg := range args {
		if arg == "--id" && i+1 < len(args) {
			return strings.TrimSpace(args[i+1])
		}
		if v, ok := strings.CutPrefix(arg, "--id="); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// ReapUnmanagedVM powers off the Firecracker process of a VM this manager
// does not run, keeping its state directory. It refuses a VM the manager
// runs and fails if the process survives.
func (m *Manager) ReapUnmanagedVM(vmID string) error {
	vmID = strings.TrimSpace(vmID)
	if vmID == "" {
		return fmt.Errorf("vm_id is required")
	}
	unlock := m.lockVMOperation(vmID)
	defer unlock()
	m.mu.Lock()
	if inst, ok := m.vms[vmID]; ok && inst.State == StateRunning {
		m.mu.Unlock()
		return fmt.Errorf("vm %s is managed and running", vmID)
	}
	m.cleanupOrphanedFirecrackerLocked(vmID, "reap-unmanaged")
	m.mu.Unlock()
	deadline := time.Now().Add(5 * time.Second)
	for {
		pids := firecrackerPIDsForVM(vmID)
		if len(pids) == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("vm %s firecracker pids %v still running", vmID, pids)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// SetVMOOMScoreAdj sets the kernel OOM priority of the VM's processes.
func (m *Manager) SetVMOOMScoreAdj(vmID string, adj int) error {
	var firstErr error
	for _, pid := range firecrackerPIDsForVM(vmID) {
		if err := setOOMScoreAdj(pid, adj); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func setOOMScoreAdj(pid, adj int) error {
	return os.WriteFile(fmt.Sprintf("/proc/%d/oom_score_adj", pid), []byte(strconv.Itoa(adj)), 0o644)
}

// VMResidentBytes maps each live VM to the resident memory (VmRSS) of its
// Firecracker processes, in bytes. vmctl measures each computer's use with
// it, for admission by measured use (internal/vmctl/host_admission.go).
func (m *Manager) VMResidentBytes() map[string]uint64 {
	return vmResidentBytesFrom(m.LiveFirecrackerVMs(), func(pid int) (string, error) {
		data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "status"))
		return string(data), err
	})
}

// vmResidentBytesFrom sums VmRSS over each VM's pids. A pid that exited or
// reports no VmRSS contributes nothing; a VM with no reading is left out.
func vmResidentBytesFrom(live map[string][]int, readStatus func(pid int) (string, error)) map[string]uint64 {
	out := make(map[string]uint64, len(live))
	for vmID, pids := range live {
		var total uint64
		found := false
		for _, pid := range pids {
			status, err := readStatus(pid)
			if err != nil {
				continue
			}
			if kib, ok := statusVmRSSKiB(status); ok {
				total += kib * 1024
				found = true
			}
		}
		if found {
			out[vmID] = total
		}
	}
	return out
}

func statusVmRSSKiB(status string) (uint64, bool) {
	for _, line := range strings.Split(status, "\n") {
		rest, ok := strings.CutPrefix(line, "VmRSS:")
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			return 0, false
		}
		kib, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return 0, false
		}
		return kib, true
	}
	return 0, false
}

func applyLaunchOOMScoreAdj(vmID string, pid int) {
	if err := setOOMScoreAdj(pid, launchOOMScoreAdj); err != nil {
		log.Printf("vmmanager: set launch oom_score_adj for VM %s pid %d: %v", vmID, pid, err)
	}
}
