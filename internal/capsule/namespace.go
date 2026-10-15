//go:build linux

package capsule

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/cgroups/v3/cgroup2"
)

// CgroupManager manages cgroup v2 resources for a capsule.
type CgroupManager struct {
	manager *cgroup2.Manager
	path    string
}

// CreateCgroup creates a cgroup v2 hierarchy for a capsule with the
// specified resource limits.
func CreateCgroup(capsuleID string, spec SpawnSpec) (*CgroupManager, error) {
	cgPath := filepath.Join("/capsule", capsuleID)

	// Build CPU max string: "quota period" (e.g. "100000 100000" = 1 CPU).
	cpuMax := cgroup2.CPUMax(fmt.Sprintf("%d %d", spec.CpuQuota, spec.CpuPeriod))
	if spec.CpuPeriod == 0 {
		cpuMax = cgroup2.CPUMax(fmt.Sprintf("%d 100000", spec.CpuQuota))
	}

	// Create the cgroup with resource limits.
	resources := &cgroup2.Resources{
		Memory: &cgroup2.Memory{
			Max:  &spec.MemoryMax,
			Swap: &[]int64{0}[0], // no swap
		},
		CPU: &cgroup2.CPU{
			Max: cpuMax,
		},
		Pids: &cgroup2.Pids{
			Max: spec.PidsMax,
		},
	}

	mgr, err := cgroup2.NewManager("/sys/fs/cgroup", cgPath, resources)
	if err != nil {
		return nil, fmt.Errorf("failed to create cgroup %s: %w", cgPath, err)
	}

	return &CgroupManager{
		manager: mgr,
		path:    cgPath,
	}, nil
}

func (c *CgroupManager) Open() (*os.File, error) {
	if c == nil || c.path == "" {
		return nil, fmt.Errorf("capsule cgroup is unavailable")
	}
	file, err := os.Open(filepath.Join("/sys/fs/cgroup", strings.TrimPrefix(c.path, "/")))
	if err != nil {
		return nil, fmt.Errorf("open capsule cgroup: %w", err)
	}
	return file, nil
}

// AddPID adds a process to the cgroup.
func (c *CgroupManager) AddPID(pid int) error {
	return c.manager.AddProc(uint64(pid))
}

func (c *CgroupManager) Freeze(ctx context.Context) error {
	return c.setFrozen(ctx, true)
}

func (c *CgroupManager) Thaw(ctx context.Context) error {
	return c.setFrozen(ctx, false)
}

func (c *CgroupManager) setFrozen(ctx context.Context, frozen bool) error {
	if c == nil || c.path == "" {
		return fmt.Errorf("capsule cgroup is unavailable")
	}
	value := "0"
	if frozen {
		value = "1"
	}
	root := filepath.Join("/sys/fs/cgroup", strings.TrimPrefix(c.path, "/"))
	if err := os.WriteFile(filepath.Join(root, "cgroup.freeze"), []byte(value), 0o600); err != nil {
		return fmt.Errorf("set capsule cgroup freeze=%s: %w", value, err)
	}
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		events, err := os.ReadFile(filepath.Join(root, "cgroup.events"))
		if err != nil {
			return fmt.Errorf("read capsule cgroup events: %w", err)
		}
		for _, line := range strings.Split(string(events), "\n") {
			fields := strings.Fields(line)
			if len(fields) == 2 && fields[0] == "frozen" && fields[1] == value {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// Delete removes the cgroup.
func (c *CgroupManager) Delete() error {
	return c.manager.Delete()
}

// Kill atomically SIGKILLs every task in the cgroup via cgroup.kill.
func (c *CgroupManager) Kill() error {
	return c.manager.Kill()
}

// WaitEmpty blocks until the cgroup reports populated=0 (no remaining tasks)
// or ctx is done. Required between Kill and Delete: cgroup.kill signals tasks
// but they take a moment to exit, and Delete fails EBUSY on a non-empty cgroup.
func (c *CgroupManager) WaitEmpty(ctx context.Context) error {
	if c == nil || c.path == "" {
		return fmt.Errorf("capsule cgroup is unavailable")
	}
	root := filepath.Join("/sys/fs/cgroup", strings.TrimPrefix(c.path, "/"))
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		events, err := os.ReadFile(filepath.Join(root, "cgroup.events"))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return fmt.Errorf("read capsule cgroup events: %w", err)
		}
		for _, line := range strings.Split(string(events), "\n") {
			fields := strings.Fields(line)
			if len(fields) == 2 && fields[0] == "populated" && fields[1] == "0" {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// TeardownReclaim returns the page cache an emptied capsule cgroup still
// holds before it is removed: removal would reparent it to the parent cgroup
// instead of freeing it, and the guest never frees clean cache on its own,
// so free page reporting would never return it to the host
// (docs/reports/playwright-in-capsules-memory-2026-10-15.md §1.1, measured:
// 0.19-0.43 GiB left by one browser run; docs/design/visual-evidence-slice-2026-10-15.md V27).
// It writes amount bytes to memory.reclaim (EAGAIN, a partial reclaim, is
// not an error) and reports memory.peak and the file and anon charge before
// and after, for the log.
func (c *CgroupManager) TeardownReclaim(amount int64) string {
	if c == nil || c.path == "" || amount <= 0 {
		return ""
	}
	root := filepath.Join("/sys/fs/cgroup", strings.TrimPrefix(c.path, "/"))
	peak := readCgroupInt(filepath.Join(root, "memory.peak"))
	beforeFile, beforeAnon := readMemoryStatFileAnon(filepath.Join(root, "memory.stat"))
	start := time.Now()
	err := os.WriteFile(filepath.Join(root, "memory.reclaim"), []byte(fmt.Sprint(amount)), 0o200)
	took := time.Since(start)
	afterFile, afterAnon := readMemoryStatFileAnon(filepath.Join(root, "memory.stat"))
	result := "ok"
	if err != nil {
		if errors.Is(err, syscall.EAGAIN) {
			result = "partial"
		} else {
			result = "error: " + err.Error()
		}
	}
	return fmt.Sprintf("peak=%dMiB file=%dMiB->%dMiB anon=%dMiB->%dMiB reclaim=%s in %s",
		peak>>20, beforeFile>>20, afterFile>>20, beforeAnon>>20, afterAnon>>20, result, took.Round(time.Millisecond))
}

func readCgroupInt(path string) int64 {
	raw, err := os.ReadFile(path)
	if err != nil {
		return -1
	}
	var v int64
	_, _ = fmt.Sscan(strings.TrimSpace(string(raw)), &v)
	return v
}

func readMemoryStatFileAnon(path string) (file, anon int64) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return -1, -1
	}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		var v int64
		_, _ = fmt.Sscan(fields[1], &v)
		switch fields[0] {
		case "file":
			file = v
		case "anon":
			anon = v
		}
	}
	return file, anon
}

// Path returns the cgroup path.
func (c *CgroupManager) Path() string {
	return c.path
}

// cleanupOrphanedCapsuleCgroup kills every process that survived in the exact
// capsule cgroup and deletes the hierarchy before restart reconciliation can
// acknowledge executor absence.
func cleanupOrphanedCapsuleCgroup(capsuleID string) error {
	path := filepath.Join("/sys/fs/cgroup/capsule", capsuleID)
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect orphan capsule cgroup: %w", err)
	}
	manager, err := cgroup2.Load(filepath.Join("/capsule", capsuleID))
	if err == nil {
		_ = manager.Kill()
		_ = manager.Delete()
	} else {
		_ = os.WriteFile(filepath.Join(path, "cgroup.kill"), []byte("1"), 0o644)
		_ = os.RemoveAll(path)
	}
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("verify orphan capsule cgroup absence: %w", err)
	}
	return fmt.Errorf("orphan capsule cgroup remained after delete")
}

func capsuleCgroupResidueExists(capsuleID string) (bool, error) {
	_, err := os.Lstat(filepath.Join("/sys/fs/cgroup/capsule", capsuleID))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

// MountOverlayFS mounts an overlayfs filesystem with the given layers.
// The lower layer is the shared EROFS base; the upper layer is the
// capsule's writable directory.
func MountOverlayFS(mergedDir, upperDir, workDir, lowerDir string) error {
	// Ensure directories exist. The writable upper/work dirs must be owned by
	// the capsule namespace host UID, otherwise the broker (mapped to that host
	// UID) cannot write into the overlay.
	for _, dir := range []string{mergedDir, upperDir, workDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create overlay dir %s: %w", dir, err)
		}
	}
	for _, dir := range []string{upperDir, workDir} {
		if err := os.Chown(dir, capsuleNamespaceHostID, capsuleNamespaceHostID); err != nil {
			return fmt.Errorf("failed to make overlay dir writable %s: %w", dir, err)
		}
	}

	// Mount overlayfs with userxattr (required for user namespaces).
	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s,userxattr", lowerDir, upperDir, workDir)
	if err := syscall.Mount("overlay", mergedDir, "overlay", 0, opts); err != nil {
		return fmt.Errorf("failed to mount overlayfs at %s: %w", mergedDir, err)
	}

	return nil
}

// UnmountOverlayFS unmounts the overlayfs mount.
func UnmountOverlayFS(mergedDir string) error {
	if err := syscall.Unmount(mergedDir, 0); err != nil {
		if err != syscall.EINVAL {
			return fmt.Errorf("failed to unmount overlayfs at %s: %w", mergedDir, err)
		}
	}
	return nil
}
