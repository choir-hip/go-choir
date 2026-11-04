//go:build !linux

// Package childthread starts a child whose Pdeathsig must mean "the parent
// process died"; off Linux there is no Pdeathsig, and Start is cmd.Start.
package childthread

import "os/exec"

// Start is cmd.Start.
func Start(cmd *exec.Cmd) error { return cmd.Start() }

// Run is cmd.Run.
func Run(cmd *exec.Cmd) error { return cmd.Run() }
