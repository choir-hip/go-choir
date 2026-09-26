//go:build linux

package yaegikernel

import (
	"syscall"

	"golang.org/x/sys/unix"
)

// socketpairFDs creates the connected socket pair the desk worker inherits.
func socketpairFDs() ([2]int, error) {
	return unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
}

// killProcessGroup SIGKILLs the worker's whole process group (the process
// itself is killed by the caller's Process.Kill for the reap to bind to).
func killProcessGroup(pid int) {
	if pid > 0 {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}
}

// workerSysProcAttr isolates the worker in its own process group and binds
// its death to the parent's (Pdeathsig: a parent exit can never orphan a
// live worker). Linux-only hardening — see desk_worker_spawn_other.go.
func workerSysProcAttr(group bool) *syscall.SysProcAttr {
	if !group {
		return nil
	}
	return &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
}
