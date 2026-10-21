//go:build linux

package yaegikernel

import "syscall"

// attachCgroup makes the child start inside the worker's cgroup
// (CLONE_INTO_CGROUP, Linux 5.7).
func attachCgroup(attr *syscall.SysProcAttr, cg *workerCgroup) error {
	attr.UseCgroupFD = true
	attr.CgroupFD = int(cg.dir.Fd())
	return nil
}
