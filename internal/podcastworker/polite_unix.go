//go:build !windows

package podcastworker

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// MaxThreads is the most model threads the worker uses: the benchmark showed
// eight give nothing over four (docs/problems/podcast-clipping-work-was-never-written-down-2026-11-15.md).
const MaxThreads = 4

// BeAPoliteNeighbour lowers this process to the lowest CPU priority (nice 19)
// and, where `ionice` exists, the idle I/O class. ffmpeg and the model run as
// children and inherit both. These are manners on a host the worker shares,
// not limits on model calls. It returns what it could not do.
func BeAPoliteNeighbour() []error {
	var problems []error
	if err := syscall.Setpriority(syscall.PRIO_PROCESS, 0, 19); err != nil {
		problems = append(problems, fmt.Errorf("nice 19: %w", err))
	}
	if path, err := exec.LookPath("ionice"); err == nil {
		if out, err := exec.Command(path, "-c", "3", "-p", fmt.Sprint(os.Getpid())).CombinedOutput(); err != nil {
			problems = append(problems, fmt.Errorf("ionice idle class: %v: %s", err, tail(out)))
		}
	} else {
		problems = append(problems, fmt.Errorf("ionice not found; the I/O class stays as it was"))
	}
	return problems
}
