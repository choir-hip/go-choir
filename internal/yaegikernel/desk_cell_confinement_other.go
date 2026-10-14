//go:build !linux

package yaegikernel

import (
	"fmt"
	"os"
	"syscall"
)

type cellSpawnPlan struct {
	attr   *syscall.SysProcAttr
	env    []string
	dir    string
	tmpDir string
}

// planCellConfinement: confinement is a Linux mechanism (the guest is
// Linux). A dev host runs workers as before; required mode refuses.
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
