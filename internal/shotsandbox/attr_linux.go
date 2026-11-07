//go:build linux

package shotsandbox

import (
	"os/exec"
	"syscall"
)

// setChildAttr puts a child in its own process group and dies with its parent.
func setChildAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
}

// killGroup stops the child and everything it started.
func killGroup(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
