package yaegikernel

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

// A hard memory budget per desk worker (SDC RES-02) inside a desks cgroup
// with an aggregate limit (RES-01).
//
// Before it, a worker's bound was a virtual address cap (RLIMIT_AS 8 GiB) in
// an 8 GiB guest and a soft Go limit: two runaway cells filled the guest and
// the kernel killed the daemon, the largest process
// (docs/archive/problems/desk-cell-init-only-for-loop-never-ends-guest-oom-2026-10-16.md).
// Now each worker is created inside its own cgroup (<root>/w-<id>) with
// memory.max and memory.high from its desk's manifest row, no swap, a process
// cap, and oom.group: the kernel kills the worker at its budget, never the
// daemon. The worker is placed in the cgroup at clone (CLONE_INTO_CGROUP), so
// there is no instant at which it runs outside the budget.
//
// The root is the desks cgroup, created and limited by EnsureDeskCgroupRoot at
// daemon start. A guest image sets DeskCgroupRootEnv; required confinement
// mode refuses to start a worker without a budget.

// DeskCgroupRootEnv names the desks cgroup directory (the guest image sets
// /sys/fs/cgroup/choir-desks). Empty means no cgroup budget: allowed only
// outside required mode (dev and CI), where RLIMIT_AS and GOMEMLIMIT remain.
const DeskCgroupRootEnv = "CHOIR_DESK_CGROUP_ROOT"

const (
	workerCgroupPrefix = "w-"
	// workerPidsMax caps threads and processes in one worker's cgroup: the
	// seccomp filter admits clone with CLONE_THREAD only, so this bounds a
	// thread bomb the memory limit would meet late.
	workerPidsMax = 1024
)

// workerCgroup is one worker's cgroup directory.
type workerCgroup struct {
	path      string
	maxBytes  uint64
	highBytes uint64
	dir       *os.File
}

var writeCgroupFile = func(dir, name, value string) error {
	return os.WriteFile(filepath.Join(dir, name), []byte(value), 0o644)
}

// EnsureDeskCgroupRoot creates the desks cgroup, enables the memory and pids
// controllers for its children, sets its aggregate limits, and removes the
// cgroups of workers a crashed daemon left behind (an empty directory per
// dead worker: workers die with the daemon, nothing resumes). It lists one
// directory; it touches nothing else.
func EnsureDeskCgroupRoot(root string, maxBytes, highBytes uint64) error {
	if root == "" {
		return fmt.Errorf("desk cgroup root is not set")
	}
	if highBytes >= maxBytes && maxBytes != 0 {
		return fmt.Errorf("desks cgroup: memory.high %d must be below memory.max %d", highBytes, maxBytes)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("desks cgroup %s: %w", root, err)
	}
	// Children can use a controller only if their parent's subtree_control
	// lists it, and the parent's parent must list it for the parent.
	if err := writeCgroupFile(filepath.Dir(root), "cgroup.subtree_control", "+memory +pids"); err != nil {
		// Already enabled is fine; anything else shows in the next write.
		if !strings.Contains(err.Error(), "device or resource busy") {
			return fmt.Errorf("desks cgroup: enable controllers in %s: %w", filepath.Dir(root), err)
		}
	}
	// Orphans first, while the root has no controllers to constrain the sweep.
	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("desks cgroup: list %s: %w", root, err)
	}
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), workerCgroupPrefix) {
			removeCgroupDir(filepath.Join(root, e.Name()))
		}
	}
	if err := writeCgroupFile(root, "cgroup.subtree_control", "+memory +pids"); err != nil && !strings.Contains(err.Error(), "device or resource busy") {
		return fmt.Errorf("desks cgroup: enable controllers in %s: %w", root, err)
	}
	if maxBytes > 0 {
		if err := writeCgroupFile(root, "memory.max", strconv.FormatUint(maxBytes, 10)); err != nil {
			return fmt.Errorf("desks cgroup: memory.max: %w", err)
		}
		if highBytes > 0 {
			if err := writeCgroupFile(root, "memory.high", strconv.FormatUint(highBytes, 10)); err != nil {
				return fmt.Errorf("desks cgroup: memory.high: %w", err)
			}
		}
	}
	return nil
}

// planWorkerCgroup creates the worker's cgroup from its desk's manifest row,
// or returns nil when no budget is configured and the mode allows that. It
// refuses rather than weakens: in required mode a missing root, a role with no
// budget, or a cgroup that cannot be made is an error and no worker starts.
func planWorkerCgroup(cfg DeskSessionWorkerConfig) (*workerCgroup, error) {
	root := cfg.CgroupRoot
	if root == "" {
		root = os.Getenv(DeskCgroupRootEnv)
	}
	required := cfg.Confinement.Mode == CellConfinementRequired
	if root == "" {
		if required {
			return nil, fmt.Errorf("desk worker memory budget: required mode needs %s (no desks cgroup)", DeskCgroupRootEnv)
		}
		return nil, nil
	}
	d, known := deskmanifest.Lookup(cfg.Session.Role)
	if !known || d.Carrier != deskmanifest.CarrierDeskWorker || d.Memory.WorkerMaxBytes == 0 {
		return nil, fmt.Errorf("desk worker memory budget: the manifest gives desk role %q no worker budget", cfg.Session.Role)
	}
	id := make([]byte, 6)
	if _, err := rand.Read(id); err != nil {
		return nil, fmt.Errorf("desk worker cgroup name: %w", err)
	}
	return createWorkerCgroup(root, workerCgroupPrefix+hex.EncodeToString(id), d.Memory.WorkerMaxBytes, d.Memory.WorkerHighBytes)
}

func createWorkerCgroup(root, name string, maxBytes, highBytes uint64) (*workerCgroup, error) {
	path := filepath.Join(root, name)
	if err := os.Mkdir(path, 0o755); err != nil {
		return nil, fmt.Errorf("desk worker cgroup %s: %w", path, err)
	}
	cg := &workerCgroup{path: path, maxBytes: maxBytes, highBytes: highBytes}
	settings := []struct{ file, value string }{
		{"memory.swap.max", "0"},
		{"memory.max", strconv.FormatUint(maxBytes, 10)},
		{"memory.oom.group", "1"},
		{"pids.max", strconv.Itoa(workerPidsMax)},
	}
	if highBytes > 0 {
		settings = append(settings, struct{ file, value string }{"memory.high", strconv.FormatUint(highBytes, 10)})
	}
	for _, s := range settings {
		err := writeCgroupFile(path, s.file, s.value)
		if err != nil && s.file == "memory.swap.max" && errors.Is(err, os.ErrNotExist) {
			// No swap controller means no swap to bound.
			continue
		}
		if err != nil {
			_ = os.Remove(path)
			return nil, fmt.Errorf("desk worker cgroup %s: %s: %w", path, s.file, err)
		}
	}
	dir, err := os.Open(path)
	if err != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("desk worker cgroup %s: open: %w", path, err)
	}
	cg.dir = dir
	return cg, nil
}

// softHeapLimit is the Go soft limit a worker with this budget gets: three
// quarters of memory.max, so the collector works before the kernel does.
func (c *workerCgroup) softHeapLimit() uint64 { return c.maxBytes / 4 * 3 }

// memoryEvents reads memory.events: how often the worker hit memory.high
// (throttled) and memory.max (oom_kill: the kernel killed it).
func (c *workerCgroup) memoryEvents() (high, oomKill uint64) {
	raw, err := os.ReadFile(filepath.Join(c.path, "memory.events"))
	if err != nil {
		return 0, 0
	}
	for _, line := range strings.Split(string(raw), "\n") {
		f := strings.Fields(line)
		if len(f) != 2 {
			continue
		}
		n, _ := strconv.ParseUint(f[1], 10, 64)
		switch f[0] {
		case "high":
			high = n
		case "oom_kill":
			oomKill = n
		}
	}
	return high, oomKill
}

// note describes how the budget figured in the worker's end, for the exit
// cause the desk and the trace read.
func (c *workerCgroup) note() string {
	high, oomKill := c.memoryEvents()
	switch {
	case oomKill > 0:
		return fmt.Sprintf("; killed by its memory budget (memory.max %d MiB, oom_kill %d)", c.maxBytes>>20, oomKill)
	case high > 0:
		return fmt.Sprintf("; throttled at its memory budget %d times (memory.high %d MiB)", high, c.highBytes>>20)
	}
	return ""
}

// release ends whatever is left in the cgroup and removes it.
func (c *workerCgroup) release() {
	if c == nil {
		return
	}
	if c.dir != nil {
		_ = c.dir.Close()
		c.dir = nil
	}
	removeCgroupDir(c.path)
}

// cgroupRmdir removes an empty cgroup directory (rmdir; the kernel's
// pseudo-files go with it). A test standing a plain directory in for the
// cgroup filesystem replaces it.
var cgroupRmdir = os.Remove

// removeCgroupDir kills any task left in a cgroup, waits briefly for it to
// empty, and removes it. A cgroup with tasks cannot be removed.
func removeCgroupDir(path string) {
	_ = writeCgroupFile(path, "cgroup.kill", "1")
	deadline := time.Now().Add(2 * time.Second)
	for {
		if err := cgroupRmdir(path); err == nil || os.IsNotExist(err) {
			return
		}
		if time.Now().After(deadline) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}
