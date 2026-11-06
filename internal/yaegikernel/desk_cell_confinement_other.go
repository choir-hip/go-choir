//go:build !linux

package yaegikernel

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

type cellSpawnPlan struct {
	attr    *syscall.SysProcAttr
	env     []string
	dir     string
	tmpDir  string
	uid     uint32
	release func()
	gvisor  *gvisorCell
	wrap    func(*exec.Cmd) error
	// budgetCap and queued: see the Linux plan.
	budgetCap uint64
	queued    time.Duration
}

// planCellConfinement: confinement is a Linux mechanism (the guest is
// Linux). A dev host runs workers as before; required mode refuses. gVisor
// is Linux-only too: only local unit tests run a desk worker natively on a
// non-Linux host (R40), so no desk is refused here for lacking it.
func planCellConfinement(cfg DeskSessionWorkerConfig) (cellSpawnPlan, error) {
	if cfg.Confinement.Mode == CellConfinementRequired {
		return cellSpawnPlan{}, fmt.Errorf("desk cell confinement: required mode needs Linux")
	}
	return cellSpawnPlan{attr: workerSysProcAttr(cfg.ProcessGroup), dir: cfg.Session.AllowedRoot}, nil
}

// ConfineDeskSessionWorker refuses a required sandbox it cannot provide.
func ConfineDeskSessionWorker(getenv func(string) string, prefix string) error {
	if getenv == nil {
		getenv = os.Getenv
	}
	if cellSandboxLevel(getenv(sessionEnvPrefix(prefix)+cellSandboxEnv)) == cellSandboxRequired {
		return fmt.Errorf("desk cell worker: required sandbox needs Linux")
	}
	return nil
}

// discard undoes a plan whose worker did not start: its tmp dir and uid.
func (p cellSpawnPlan) discard() {
	removeCellTmpDir(p.tmpDir)
	if p.release != nil {
		p.release()
	}
}

// CloseInheritedFDs: descriptor hygiene is a Linux worker step.
func CloseInheritedFDs(keep ...int) {}
