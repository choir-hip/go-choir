package vmbackend

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Registry holds the backends a host can run, and is the one place process
// discovery asks "whose VMM is this?". Today's discovery
// (vmmanager.firecrackerPIDsForVM, LiveFirecrackerVMs) only recognises
// Firecracker; a Cloud Hypervisor computer would be invisible to boot's
// live-unmanaged guard, DestroyVMState's live-process refusal and vmctl's
// host-capacity reconcile. No second backend may launch a computer until
// those callers ask the Registry instead.
type Registry struct {
	order    []Name
	backends map[Name]Backend
}

// NewRegistry registers backends in order. Duplicate names are an error.
func NewRegistry(backends ...Backend) (*Registry, error) {
	r := &Registry{backends: map[Name]Backend{}}
	for _, b := range backends {
		if b == nil {
			return nil, fmt.Errorf("vmbackend: nil backend")
		}
		n := b.Name()
		if _, dup := r.backends[n]; dup {
			return nil, fmt.Errorf("vmbackend: backend %q registered twice", n)
		}
		r.backends[n] = b
		r.order = append(r.order, n)
	}
	return r, nil
}

// Get returns the named backend.
func (r *Registry) Get(n Name) (Backend, bool) {
	b, ok := r.backends[n]
	return b, ok
}

// Names lists registered backends in registration order.
func (r *Registry) Names() []Name { return append([]Name(nil), r.order...) }

// Identify asks every backend whether args are its VMM. A process two
// backends both claim is refused (ok=false): ambiguity must never let a
// reaper act on the wrong VM.
func (r *Registry) Identify(args []string) (Name, string, bool) {
	var (
		found Name
		vmID  string
		hits  int
	)
	for _, n := range r.order {
		if id, ok := r.backends[n].VMIDFromCmdline(args); ok && id != "" {
			found, vmID = n, id
			hits++
		}
	}
	if hits != 1 {
		return "", "", false
	}
	return found, vmID, true
}

// ProcCmdline is one /proc/<pid>/cmdline sample.
type ProcCmdline struct {
	PID  int
	Data []byte
}

// LiveProcess is one VMM process of one VM.
type LiveProcess struct {
	Backend Name
	PID     int
}

// LiveVMs maps VM id to every VMM process running it, across all backends.
// More than one process for a VM is a duplicate the caller must resolve.
func (r *Registry) LiveVMs(entries []ProcCmdline) map[string][]LiveProcess {
	out := map[string][]LiveProcess{}
	for _, e := range entries {
		if e.PID <= 0 {
			continue
		}
		n, vmID, ok := r.Identify(SplitCmdline(e.Data))
		if !ok {
			continue
		}
		out[vmID] = append(out[vmID], LiveProcess{Backend: n, PID: e.PID})
	}
	for id := range out {
		sort.Slice(out[id], func(i, j int) bool { return out[id][i].PID < out[id][j].PID })
	}
	return out
}

// PIDsForVM lists the pids of every VMM running vmID, across backends.
func (r *Registry) PIDsForVM(entries []ProcCmdline, vmID string) []int {
	vmID = strings.TrimSpace(vmID)
	if vmID == "" {
		return nil
	}
	var pids []int
	for _, p := range r.LiveVMs(entries)[vmID] {
		pids = append(pids, p.PID)
	}
	return pids
}

// ScanProc reads every /proc/<pid>/cmdline it can. Processes that exit
// mid-scan are skipped. On hosts without /proc it returns nil.
func ScanProc() []ProcCmdline {
	return scanProcAt("/proc")
}

func scanProcAt(root string) []ProcCmdline {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	out := make([]ProcCmdline, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, entry.Name(), "cmdline"))
		if err != nil || len(data) == 0 {
			continue
		}
		out = append(out, ProcCmdline{PID: pid, Data: data})
	}
	return out
}

// SplitCmdline splits /proc/<pid>/cmdline on NUL, keeping each argument
// whole (Cloud Hypervisor's --cmdline value contains spaces). Data with no
// NUL falls back to whitespace splitting, matching vmmanager's
// processCmdlineArgs.
func SplitCmdline(data []byte) []string {
	raw := string(data)
	if !strings.Contains(raw, "\x00") {
		return strings.Fields(raw)
	}
	parts := strings.Split(raw, "\x00")
	args := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			args = append(args, p)
		}
	}
	return args
}

// binaryBaseContains reports whether argv[0]'s base name contains want.
func binaryBaseContains(args []string, want string) bool {
	return len(args) > 0 && strings.Contains(filepath.Base(args[0]), want)
}
