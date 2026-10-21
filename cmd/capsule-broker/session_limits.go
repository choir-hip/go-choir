//go:build linux

package main

import (
	"fmt"
	"runtime/debug"

	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

const (
	// sessionWorkerSoftLimit is the Go soft memory limit of the capsule's
	// exec-go-session worker: well under the 2 GiB an engineering capsule
	// asks for, so the collector works before the capsule's cgroup does. A
	// first value (the worker interprets small Go cells; builds run in their
	// own processes).
	sessionWorkerSoftLimit = 768 << 20
	// sessionWorkerAddressCap is its RLIMIT_AS: not a tight bound (Go reserves
	// address space far above what it uses) but a ceiling no cell can pass.
	sessionWorkerAddressCap uint64 = 4 << 30
)

// applySessionWorkerLimits bounds the exec-go-session worker (SDC RES-03). It
// runs before the worker serves any cell. The worker volunteers for the OOM
// kill so that when the capsule's cgroup is full the kernel ends the worker
// and not the broker or a build running beside it; a worker that cannot do
// that does not serve.
func applySessionWorkerLimits() error {
	if err := yaegikernel.VolunteerForOOMKill(); err != nil {
		return fmt.Errorf("oom_score_adj: %w", err)
	}
	if err := yaegikernel.ApplyWorkerMemoryLimit(sessionWorkerAddressCap); err != nil {
		return err
	}
	debug.SetMemoryLimit(sessionWorkerSoftLimit)
	return nil
}
