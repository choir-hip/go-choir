package main

// S5 busy hibernate: the decision at vmctl's exit
// (docs/problems/paper-never-checkpointed-2026-10-27.md, slice S5).
//
// vmctl's exit is a planned host stop in two cases:
//   - the host is shutting down (systemd's system state is "stopping"): the
//     unit's KillMode=process leaves the guests to die with the host, a crash
//     for every computer;
//   - VMCTL_STOP_MANAGED_ON_EXIT is true: the manager stops every guest.
//
// Any other exit is a vmctl restart (a deploy, an operator restart): guests
// stay running and the next vmctl reattaches them, as before. A system
// state that cannot be read is not taken as a shutdown. A crash runs no exit
// code. Owner, 2026-10-28: on by default, no off switch.

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	hostStopWhyShutdown   = "host_shutdown"
	hostStopWhyStopOnExit = "stop_managed_on_exit"

	// defaultHostStopBudget stays under systemd's default 90 s stop timeout:
	// after it no new capture starts.
	defaultHostStopBudget = 75 * time.Second
)

type hostStopDecision struct {
	Hibernate bool
	Why       string
}

func decideHostStop(stopManagedOnExit bool, systemState func() (string, error)) hostStopDecision {
	if stopManagedOnExit {
		return hostStopDecision{Hibernate: true, Why: hostStopWhyStopOnExit}
	}
	if systemState == nil {
		return hostStopDecision{}
	}
	state, err := systemState()
	if err != nil {
		return hostStopDecision{}
	}
	if strings.TrimSpace(state) == "stopping" {
		return hostStopDecision{Hibernate: true, Why: hostStopWhyShutdown}
	}
	return hostStopDecision{}
}

// systemdSystemState is `systemctl is-system-running`. It exits non-zero for
// every state but running, so the printed state is what counts.
func systemdSystemState() (string, error) {
	bin := "/run/current-system/sw/bin/systemctl"
	if _, err := os.Stat(bin); err != nil {
		if bin, err = exec.LookPath("systemctl"); err != nil {
			return "", err
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "is-system-running").Output()
	state := strings.TrimSpace(string(out))
	if state == "" {
		if err == nil {
			err = os.ErrNotExist
		}
		return "", err
	}
	return state, nil
}

func hostStopBudgetFromEnv() time.Duration {
	if v := strings.TrimSpace(os.Getenv("VMCTL_HOST_STOP_BUDGET")); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return defaultHostStopBudget
}
