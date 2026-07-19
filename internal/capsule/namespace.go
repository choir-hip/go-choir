//go:build linux

package capsule

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/containerd/cgroups/v3/cgroup2"
	gonso "github.com/cpuguy83/gonso"
)

// NamespaceSet creates and manages Linux namespaces for a capsule.
// Uses gonso for safe namespace manipulation in Go.
type NamespaceSet struct {
	set *gonso.Set
}

// CreateCapsuleNamespaces creates a new set of namespaces for a capsule:
// - CLONE_NEWNS: mount namespace (for overlayfs)
// - CLONE_NEWPID: PID namespace (process isolation)
// - CLONE_NEWNET: network namespace (air-gapped, no interfaces)
// - CLONE_NEWUTS: hostname namespace
// - CLONE_NEWIPC: IPC namespace
// - CLONE_NEWUSER: user namespace (for broker privilege separation)
// - CLONE_NEWCGROUP: cgroup namespace
//
// CLONE_NEWNET is the primary network isolation mechanism (v10 design).
// seccomp socket family filtering is defense-in-depth on top of this.
func CreateCapsuleNamespaces() (*NamespaceSet, error) {
	flags := gonso.NS_MNT | gonso.NS_PID | gonso.NS_NET |
		gonso.NS_UTS | gonso.NS_IPC | gonso.NS_USER | gonso.NS_CGROUP

	nsSet, err := gonso.Unshare(flags)
	if err != nil {
		return nil, fmt.Errorf("failed to unshare namespaces: %w", err)
	}

	return &NamespaceSet{set: &nsSet}, nil
}

// Do runs a function inside the namespace set.
func (n *NamespaceSet) Do(fn func() error) error {
	var execErr error
	err := n.set.Do(func() {
		execErr = fn()
	})
	if err != nil {
		return err
	}
	return execErr
}

// Close releases the namespace set resources.
func (n *NamespaceSet) Close() error {
	if n.set != nil {
		return n.set.Close()
	}
	return nil
}

// CgroupManager manages cgroup v2 resources for a capsule.
type CgroupManager struct {
	manager *cgroup2.Manager
	path    string
}

// CreateCgroup creates a cgroup v2 hierarchy for a capsule with the
// specified resource limits.
func CreateCgroup(capsuleID string, spec SpawnSpec) (*CgroupManager, error) {
	cgPath := filepath.Join("capsule", capsuleID)

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

// AddPID adds a process to the cgroup.
func (c *CgroupManager) AddPID(pid int) error {
	return c.manager.AddProc(uint64(pid))
}

// Delete removes the cgroup.
func (c *CgroupManager) Delete() error {
	return c.manager.Delete()
}

// Path returns the cgroup path.
func (c *CgroupManager) Path() string {
	return c.path
}

// MountOverlayFS mounts an overlayfs filesystem with the given layers.
// The lower layer is the shared EROFS base; the upper layer is the
// capsule's writable directory.
func MountOverlayFS(mergedDir, upperDir, workDir, lowerDir string) error {
	// Ensure directories exist.
	for _, dir := range []string{mergedDir, upperDir, workDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create overlay dir %s: %w", dir, err)
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

// SetupUserNamespaceMappings sets up UID/GID mappings for a user namespace.
// The root user inside the namespace maps to an unprivileged user outside.
func SetupUserNamespaceMappings(pid int, uidMap, gidMap string) error {
	uidMapPath := fmt.Sprintf("/proc/%d/uid_map", pid)
	gidMapPath := fmt.Sprintf("/proc/%d/gid_map", pid)

	if err := os.WriteFile(uidMapPath, []byte(uidMap), 0o644); err != nil {
		return fmt.Errorf("failed to write uid_map: %w", err)
	}
	if err := os.WriteFile(gidMapPath, []byte(gidMap), 0o644); err != nil {
		return fmt.Errorf("failed to write gid_map: %w", err)
	}

	return nil
}
