//go:build unix

package visualcapture

import (
	"os/exec"
	"syscall"
)

// setProcessGroup starts Chromium in its own process group, so a kill
// reaches its zygote and helpers (V5, V6).
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
