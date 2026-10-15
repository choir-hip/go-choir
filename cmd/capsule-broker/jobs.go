//go:build linux

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsulejob"
)

// Capsule jobs, broker side (docs/design/capsule-long-exec-2026-10-15.md).
// guest core's executor starts, reads and stops a job through the
// capability-verified verbs below; the broker's capsulejob.Manager holds the
// authority (exit code, output digest, times). A job runs as
//
//	broker --isolation-stage supervise --supervise-result-fd 3 --
//	    broker --isolation-stage job-exec --job-inet=<egress> -- argv...
//
// The supervisor is a child subreaper with the broker's own confinement; the
// job-exec stage takes the session worker's (workload Landlock, capability
// drop, L2 seccomp, inet only in an egress capsule) and then becomes the
// command, so a job never has more authority than a cell.

const (
	supervisorStage = "supervise"
	jobExecStage    = "job-exec"
)

// supervisedCommand wraps argv in the supervisor stage. resultFD names the
// supervisor's outcome pipe (-1: none).
func supervisedCommand(brokerBin string, resultFD int, argv ...string) *exec.Cmd {
	args := []string{"--isolation-stage", supervisorStage, fmt.Sprintf("--supervise-result-fd=%d", resultFD), "--"}
	cmd := exec.Command(brokerBin, append(args, argv...)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	return cmd
}

// newJobManager builds the capsule's job table. The job environment is the
// direct-exec allowlist (PATH, HOME, TMPDIR, LANG, the C toolchain and ICU
// flags) plus, in an egress capsule, the proxy settings: never a caller's
// variables.
func newJobManager(brokerBin string, egress bool) *capsulejob.Manager {
	env := directExecEnvFrom(os.LookupEnv, nil)
	if egress {
		env = append(env, capsule.EgressProxyEnv()...)
	}
	return &capsulejob.Manager{
		Root: capsulejob.Root,
		Env:  env,
		Launch: func(spec capsulejob.Spec) *exec.Cmd {
			inner := append([]string{brokerBin, "--isolation-stage", jobExecStage, fmt.Sprintf("--job-inet=%t", egress), "--"}, spec.Argv...)
			return supervisedCommand(brokerBin, 3, inner...)
		},
	}
}

// runJobExecStage hardens this process the way a session worker is
// hardened and becomes the job's command. It returns only on failure.
func runJobExecStage(argv []string, inet bool) error {
	if len(argv) == 0 {
		return fmt.Errorf("job-exec: no command")
	}
	if err := capsule.NewWorkloadLandlock("/").Apply(); err != nil {
		return fmt.Errorf("job landlock: %w", err)
	}
	if err := capsule.DropWorkloadCapabilities(); err != nil {
		return fmt.Errorf("job capability drop: %w", err)
	}
	loadFilter := capsule.LoadWorkloadFilter
	if inet {
		loadFilter = capsule.LoadWorkloadInetFilter
	}
	if err := loadFilter(); err != nil {
		return fmt.Errorf("job seccomp: %w", err)
	}
	path := argv[0]
	if !strings.Contains(path, "/") {
		resolved, err := exec.LookPath(path)
		if err != nil {
			return fmt.Errorf("job command %q: %w", path, err)
		}
		path = resolved
	}
	return syscall.Exec(path, argv, os.Environ())
}

// jobParams are the job verbs' parameters. Owner is never a parameter: it
// is the verified capability's run.
type jobParams struct {
	ID          string   `json:"id"`
	Argv        []string `json:"argv,omitempty"`
	Cwd         string   `json:"cwd,omitempty"`
	WallClockMS int64    `json:"wall_clock_ms,omitempty"`
}

func (b *Broker) handleJob(_ context.Context, cap *capsule.Capability, verb string, params json.RawMessage) BrokerRPCResponse {
	if b.jobs == nil {
		return BrokerRPCResponse{Error: "jobs are unavailable in this capsule"}
	}
	var p jobParams
	if err := json.Unmarshal(params, &p); err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("failed to parse %s params: %v", verb, err)}
	}
	var st capsulejob.Status
	var err error
	switch verb {
	case "job_start":
		st, err = b.jobs.Start(capsulejob.Spec{ID: p.ID, Owner: cap.AgentRunID, Argv: p.Argv, Cwd: p.Cwd, WallClockMS: p.WallClockMS})
	case "job_status":
		st, err = b.jobs.Status(p.ID, cap.AgentRunID)
	case "job_stop":
		st, err = b.jobs.Stop(p.ID, cap.AgentRunID)
	default:
		err = fmt.Errorf("unknown job verb %s", verb)
	}
	if err != nil {
		return BrokerRPCResponse{Error: err.Error()}
	}
	raw, err := json.Marshal(st)
	if err != nil {
		return BrokerRPCResponse{Error: err.Error()}
	}
	return BrokerRPCResponse{Result: raw}
}
