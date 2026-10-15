//go:build !linux

package capsulejob

import (
	"os"
	"time"
)

// SuperviseConfig is one supervised command (Linux only).
type SuperviseConfig struct {
	Argv         []string
	Env          []string
	Dir          string
	KillGrace    time.Duration
	ReapDeadline time.Duration
	OnStart      func(pid int)
	Signals      <-chan os.Signal
}

// Supervise needs a child subreaper, which only Linux has.
func Supervise(SuperviseConfig) Outcome {
	return Outcome{State: StateFailed, ExitCode: -1, Reason: "the job supervisor runs only on Linux"}
}

// RunSupervisorStage is Linux only.
func RunSupervisorStage([]string, int) int { return 127 }
