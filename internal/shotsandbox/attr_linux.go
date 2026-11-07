//go:build linux

package shotsandbox

import (
	"os/exec"
	"syscall"
)

// setChildAttr puts a child in its own process group and dies with its
// parent. When the stage has dropped to an id, the child is also started with
// that credential explicitly, so a thread of the stage that somehow kept
// another id cannot be the parent of a root child (D-B1).
func setChildAttr(cmd *exec.Cmd, spec Spec) {
	cmd.SysProcAttr = childAttr(spec)
}

func childAttr(spec Spec) *syscall.SysProcAttr {
	attr := &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	if spec.RunAsUID != 0 || spec.RunAsGID != 0 {
		attr.Credential = &syscall.Credential{Uid: uint32(spec.RunAsUID), Gid: uint32(spec.RunAsGID), NoSetGroups: true}
	}
	return attr
}

// killGroup stops the child and everything it started.
func killGroup(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
