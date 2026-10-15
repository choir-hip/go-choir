//go:build linux

package capsulejob

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// SuperviseConfig is one supervised command. Stdio is inherited: the
// supervisor's fds 0, 1 and 2 become the command's.
type SuperviseConfig struct {
	Argv []string
	Env  []string
	Dir  string
	// KillGrace and ReapDeadline default to the package bounds.
	KillGrace    time.Duration
	ReapDeadline time.Duration
	// OnStart is called with the command's pid (its process group).
	OnStart func(pid int)
	// Signals, when set, replaces the SIGTERM/SIGINT subscription (tests).
	Signals <-chan os.Signal
}

// Supervise runs one command as this process's child and returns when the
// command and every descendant are gone (F2, F3 of the design). This
// process becomes a child subreaper, so a descendant whose parent dies is
// re-parented here rather than to the capsule's PID 1. The command gets its
// own process group and dies with this process (Pdeathsig). SIGTERM or
// SIGINT here is a stop: it is forwarded to the command's group, which gets
// SIGKILL after the grace. When the command exits, what is left of its
// group is killed, every re-parented child is killed, and all are reaped,
// for at most ReapDeadline.
//
// This process reaps with wait4(-1) alone, never through os/exec, so no
// second waiter can race it for the command's status.
func Supervise(cfg SuperviseConfig) Outcome {
	grace := cfg.KillGrace
	if grace <= 0 {
		grace = KillGrace
	}
	reapFor := cfg.ReapDeadline
	if reapFor <= 0 {
		reapFor = ReapDeadline
	}
	if len(cfg.Argv) == 0 {
		return Outcome{State: StateFailed, ExitCode: -1, Reason: "no command"}
	}
	if err := unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0); err != nil {
		return Outcome{State: StateFailed, ExitCode: -1, Reason: fmt.Sprintf("become subreaper: %v", err)}
	}
	// F8: the command must not read this process's memory or fds.
	_ = unix.Prctl(unix.PR_SET_DUMPABLE, 0, 0, 0, 0)

	sigs := cfg.Signals
	if sigs == nil {
		ch := make(chan os.Signal, 2)
		signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
		defer signal.Stop(ch)
		sigs = ch
	}

	path := cfg.Argv[0]
	if !strings.Contains(path, "/") {
		resolved, err := lookPath(path, cfg.Env)
		if err != nil {
			return Outcome{State: StateFailed, ExitCode: -1, Reason: fmt.Sprintf("start: %v", err)}
		}
		path = resolved
	}
	pid, err := syscall.ForkExec(path, cfg.Argv, &syscall.ProcAttr{
		Dir: cfg.Dir, Env: cfg.Env, Files: []uintptr{0, 1, 2},
		Sys: &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL},
	})
	if err != nil {
		return Outcome{State: StateFailed, ExitCode: -1, Reason: fmt.Sprintf("start: %v", err)}
	}
	if cfg.OnStart != nil {
		cfg.OnStart(pid)
	}

	mainExit := make(chan syscall.WaitStatus, 1)
	gone := make(chan struct{})
	go func() {
		defer close(gone)
		for {
			var ws syscall.WaitStatus
			wpid, err := syscall.Wait4(-1, &ws, 0, nil)
			if errors.Is(err, syscall.EINTR) {
				continue
			}
			if err != nil {
				return // ECHILD: no child is left
			}
			if wpid == pid {
				mainExit <- ws
			}
		}
	}()

	out := Outcome{ChildPID: pid}
	stopped := false
	var escalate <-chan time.Time
	var ws syscall.WaitStatus
wait:
	for {
		select {
		case ws = <-mainExit:
			break wait
		case <-sigs:
			if !stopped {
				stopped = true
				_ = syscall.Kill(-pid, syscall.SIGTERM)
				escalate = time.After(grace)
			}
		case <-escalate:
			_ = syscall.Kill(-pid, syscall.SIGKILL)
			killChildren()
		}
	}

	switch {
	case ws.Exited():
		out.ExitCode = ws.ExitStatus()
	case ws.Signaled():
		out.ExitCode = -1
		out.Signal = ws.Signal().String()
		out.SigNum = int(ws.Signal())
	default:
		out.ExitCode = -1
	}
	switch {
	case stopped:
		out.State, out.Reason = StateStopped, "stopped on request"
	case out.ExitCode == 0:
		out.State = StateDone
	default:
		out.State = StateFailed
	}

	// Whatever the command left behind dies with it.
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	deadline := time.After(reapFor)
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		killChildren()
		select {
		case <-gone:
			return out
		case <-deadline:
			out.Orphans = len(childPIDs())
			return out
		case <-tick.C:
		}
	}
}

// RunSupervisorStage is the body of a supervisor process: it supervises
// argv with this process's environment, working directory and stdio, writes
// {"event":"started","pid":N} and then {"event":"outcome",...} as JSON lines
// to resultFD when resultFD >= 0, and returns the exit status the process
// should end with.
func RunSupervisorStage(argv []string, resultFD int) int {
	var result *os.File
	if resultFD >= 0 {
		result = os.NewFile(uintptr(resultFD), "supervisor-result")
	}
	emit := func(v any) {
		if result == nil {
			return
		}
		data, err := json.Marshal(v)
		if err == nil {
			_, _ = result.Write(append(data, '\n'))
		}
	}
	dir, _ := os.Getwd()
	out := Supervise(SuperviseConfig{
		Argv: argv, Env: os.Environ(), Dir: dir,
		OnStart: func(pid int) { emit(map[string]any{"event": "started", "pid": pid}) },
	})
	emit(struct {
		Event string `json:"event"`
		Outcome
	}{"outcome", out})
	if result != nil {
		_ = result.Close()
	}
	if out.ChildPID == 0 {
		fmt.Fprintf(os.Stderr, "supervisor: %s\n", out.Reason)
	}
	return out.ExitStatus()
}

// lookPath resolves a bare command name against the PATH in env.
func lookPath(name string, env []string) (string, error) {
	path := ""
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, "PATH="); ok {
			path = v
		}
	}
	for _, dir := range strings.Split(path, ":") {
		if dir == "" {
			continue
		}
		candidate := dir + "/" + name
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return candidate, nil
		}
	}
	return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
}

// childPIDs lists this process's live children through /proc (a re-parented
// orphan is a child of the subreaper).
func childPIDs() []int {
	self := os.Getpid()
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	var out []int
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == self {
			continue
		}
		raw, err := os.ReadFile("/proc/" + e.Name() + "/stat")
		if err != nil {
			continue
		}
		// pid (comm) state ppid ...; comm may hold spaces and parentheses.
		s := string(raw)
		i := strings.LastIndexByte(s, ')')
		if i < 0 {
			continue
		}
		fields := strings.Fields(s[i+1:])
		if len(fields) < 2 || fields[0] == "Z" {
			continue
		}
		if ppid, err := strconv.Atoi(fields[1]); err == nil && ppid == self {
			out = append(out, pid)
		}
	}
	return out
}

// killChildren SIGKILLs every live child and the process group it leads.
func killChildren() {
	for _, pid := range childPIDs() {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
}
