//go:build !linux

package shotsandbox

import "os/exec"

func setChildAttr(cmd *exec.Cmd, spec Spec) {}

func killGroup(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
