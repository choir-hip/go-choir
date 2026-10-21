//go:build !linux

package yaegikernel

import (
	"fmt"
	"syscall"
)

// attachCgroup: cgroups are Linux's. Planning a budget on another OS (a
// DeskCgroupRootEnv set on a dev host) refuses the spawn.
func attachCgroup(attr *syscall.SysProcAttr, cg *workerCgroup) error {
	return fmt.Errorf("desk worker memory budget needs Linux cgroups")
}
