//go:build !linux

package yaegikernel

import (
	"syscall"

	"golang.org/x/sys/unix"
)

// socketpairFDs creates the connected socket pair the desk worker inherits.
func socketpairFDs() ([2]int, error) {
	return unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
}

// killProcessGroup SIGKILLs the worker's whole process group. On non-Linux
// there is no Pdeathsig, so a parent exit could orphan a live worker — the
// broker reaps/reaps-on-drain instead; staging (Linux) is the containment
// proof surface.
func killProcessGroup(pid int) {
	if pid > 0 {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}
}

// workerSysProcAttr isolates the worker in its own process group. No
// Pdeathsig on this platform — the parent reaps explicitly.
func workerSysProcAttr(group bool) *syscall.SysProcAttr {
	if !group {
		return nil
	}
	return &syscall.SysProcAttr{Setpgid: true}
}
