package capsulejob

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// Launch builds the (unstarted) supervisor command for a job; the Manager
// sets its environment, directory, stdio, result pipe and process group.
// In the capsule broker it is `broker --isolation-stage supervise
// --supervise-result-fd 3 -- broker --isolation-stage job-exec -- argv...`.
type Launch func(spec Spec) *exec.Cmd

// Manager is the broker side of jobs: the job table, the one-job rule, the
// wall clock, the log with its digest, and the tail. Its memory is the
// authority the executor reads; the files it writes are for reading only.
type Manager struct {
	Root   string   // one directory per job
	Env    []string // the allowlisted job environment
	Launch Launch
	Now    func() time.Time
	// KillGrace is the stop-to-SIGKILL grace the supervisor gets before the
	// backstop kills its group (it adds its own reap time).
	KillGrace time.Duration
	// MaxLog caps the stored log (MaxLogBytes when zero).
	MaxLog int64

	mu   sync.Mutex
	jobs map[string]*job
}

type job struct {
	mu       sync.Mutex
	spec     Spec
	status   Status
	cmd      *exec.Cmd
	childPID int
	stopping string // StateTimeout or StateStopped once a stop is asked
	tail     *Tail
	done     chan struct{}
}

func (m *Manager) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now()
}

func (m *Manager) grace() time.Duration {
	if m.KillGrace > 0 {
		return m.KillGrace
	}
	return KillGrace
}

// Start begins one job. One job runs at a time per Manager (one per
// capsule: two heavy builds exhaust its memory); a second start while one
// runs is refused, as is a reused id.
func (m *Manager) Start(spec Spec) (Status, error) {
	spec, err := spec.Normalize()
	if err != nil {
		return Status{}, err
	}
	if m.Launch == nil {
		return Status{}, fmt.Errorf("jobs are unavailable in this capsule (no launcher)")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.jobs == nil {
		m.jobs = map[string]*job{}
	}
	if _, taken := m.jobs[spec.ID]; taken {
		return Status{}, fmt.Errorf("job %s already exists", spec.ID)
	}
	for _, other := range m.jobs {
		select {
		case <-other.done:
		default:
			return Status{}, fmt.Errorf("job %s is still running; one job runs at a time in a capsule: wait for it or stop it", other.spec.ID)
		}
	}
	dir := filepath.Join(m.Root, spec.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Status{}, fmt.Errorf("create job directory: %w", err)
	}
	logPath := filepath.Join(dir, LogFile)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return Status{}, fmt.Errorf("create job log: %w", err)
	}
	now := m.now().UTC()
	j := &job{spec: spec, tail: NewTail(TailBytes), done: make(chan struct{}), status: Status{
		ID: spec.ID, State: StateRunning, Argv: append([]string(nil), spec.Argv...), Cwd: spec.Cwd,
		EnvDigest: EnvDigest(m.Env), WallClockMS: spec.WallClockMS, StartedAt: now, LogPath: logPath,
	}}
	m.jobs[spec.ID] = j

	fail := func(reason string) (Status, error) {
		_ = logFile.Close()
		code := -1
		j.status.State, j.status.Reason, j.status.FinishedAt, j.status.ExitCode = StateFailed, reason, now, &code
		j.status.OutputSHA256 = hex.EncodeToString(sha256.New().Sum(nil))
		close(j.done)
		_ = WriteJSONAtomic(dir, StatusFile, j.snapshot(now))
		return j.snapshot(now), nil
	}
	if info, err := os.Stat(spec.Cwd); err != nil || !info.IsDir() {
		return fail(fmt.Sprintf("cwd %s is not a directory", spec.Cwd))
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		return fail("output pipe: " + err.Error())
	}
	resR, resW, err := os.Pipe()
	if err != nil {
		_ = outR.Close()
		_ = outW.Close()
		return fail("result pipe: " + err.Error())
	}
	cmd := m.Launch(spec)
	cmd.Env = append([]string(nil), m.Env...)
	cmd.Dir = spec.Cwd
	cmd.Stdin = nil
	cmd.Stdout, cmd.Stderr = outW, outW
	cmd.ExtraFiles = []*os.File{resW}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	if err := cmd.Start(); err != nil {
		for _, f := range []*os.File{outR, outW, resR, resW} {
			_ = f.Close()
		}
		return fail("start: " + err.Error())
	}
	_ = outW.Close()
	_ = resW.Close()
	j.cmd = cmd
	_ = WriteJSONAtomic(dir, StatusFile, j.snapshot(now))
	go m.supervise(j, dir, logFile, outR, resR)
	return j.snapshot(now), nil
}

// supervise pumps output, reads the supervisor's report, enforces the wall
// clock and records the end.
func (m *Manager) supervise(j *job, dir string, logFile *os.File, outR, resR *os.File) {
	defer close(j.done)
	timer := time.AfterFunc(j.spec.WallClock(), func() { m.stop(j, StateTimeout) })
	defer timer.Stop()

	hash := sha256.New()
	pumped := make(chan struct{})
	go func() {
		defer close(pumped)
		m.pump(j, outR, logFile, hash)
	}()
	var outcome *Outcome
	reported := make(chan struct{})
	go func() {
		defer close(reported)
		scanner := bufio.NewScanner(resR)
		for scanner.Scan() {
			var line struct {
				Event string `json:"event"`
				PID   int    `json:"pid"`
				Outcome
			}
			if json.Unmarshal(scanner.Bytes(), &line) != nil {
				continue
			}
			switch line.Event {
			case "started":
				j.mu.Lock()
				j.childPID = line.PID
				j.mu.Unlock()
			case "outcome":
				o := line.Outcome
				outcome = &o
			}
		}
	}()
	waitErr := j.cmd.Wait()
	<-reported
	_ = resR.Close()
	// Every descendant is reaped before the supervisor exits, so the pipe
	// is at EOF; a supervisor killed by the backstop may leave a holder, so
	// the pump gets a bounded drain.
	select {
	case <-pumped:
	case <-time.After(5 * time.Second):
		_ = outR.Close()
		<-pumped
	}
	_ = outR.Close()
	_ = logFile.Close()

	now := m.now().UTC()
	j.mu.Lock()
	j.status.FinishedAt = now
	j.status.OutputSHA256 = hex.EncodeToString(hash.Sum(nil))
	j.status.Tail = j.tail.String()
	code := -1
	switch {
	case outcome != nil:
		code = outcome.ExitCode
		j.status.Signal = outcome.Signal
		j.status.Orphans = outcome.Orphans
		if outcome.ChildPID == 0 {
			j.status.Reason = outcome.Reason
		}
	case j.cmd.ProcessState != nil:
		code = j.cmd.ProcessState.ExitCode()
		j.status.Reason = "the supervisor reported no outcome"
		if waitErr != nil {
			j.status.Reason += ": " + waitErr.Error()
		}
	}
	j.status.ExitCode = &code
	switch {
	case j.stopping == StateTimeout:
		j.status.State = StateTimeout
		j.status.Reason = fmt.Sprintf("wall clock of %s reached; the job's processes were killed", j.spec.WallClock())
	case j.stopping == StateStopped:
		j.status.State = StateStopped
		if j.status.Reason == "" {
			j.status.Reason = "stopped on request"
		}
	case outcome != nil && outcome.State == StateDone && code == 0:
		j.status.State = StateDone
	default:
		j.status.State = StateFailed
	}
	j.mu.Unlock()
	_ = WriteJSONAtomic(dir, StatusFile, j.snapshot(now))
}

// pump copies the combined output into the log (up to MaxLogBytes), the
// digest of what it stored, and the tail.
func (m *Manager) pump(j *job, r io.Reader, logFile *os.File, digest hash.Hash) {
	buf := make([]byte, 32<<10)
	var stored int64
	limit := m.MaxLog
	if limit <= 0 {
		limit = MaxLogBytes
	}
	for {
		n, err := r.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			_, _ = j.tail.Write(chunk)
			keep := chunk
			if room := limit - stored; int64(len(keep)) > room {
				keep = keep[:max(room, 0)]
			}
			if len(keep) > 0 {
				w, _ := logFile.Write(keep)
				digest.Write(keep[:w])
				stored += int64(w)
			}
			j.mu.Lock()
			j.status.OutputBytes += int64(n)
			if int64(n) > int64(len(keep)) {
				j.status.Truncated = true
			}
			j.mu.Unlock()
		}
		if err != nil {
			return
		}
	}
}

// stop asks a running job to stop: SIGTERM to the supervisor, which forwards
// it to the command's group and escalates after its grace; a backstop kills
// the supervisor's group and the command's group if it has not exited.
func (m *Manager) stop(j *job, reason string) {
	j.mu.Lock()
	select {
	case <-j.done:
		j.mu.Unlock()
		return
	default:
	}
	if j.stopping != "" || j.cmd == nil || j.cmd.Process == nil {
		j.mu.Unlock()
		return
	}
	j.stopping = reason
	pid := j.cmd.Process.Pid
	j.mu.Unlock()
	_ = syscall.Kill(pid, syscall.SIGTERM)
	backstop := 2*m.grace() + ReapDeadline
	go func() {
		select {
		case <-j.done:
		case <-time.After(backstop):
			j.mu.Lock()
			child := j.childPID
			j.mu.Unlock()
			_ = syscall.Kill(-pid, syscall.SIGKILL)
			if child > 0 {
				_ = syscall.Kill(-child, syscall.SIGKILL)
			}
		}
	}()
}

// Stop asks job id to stop. It is a no-op for a job that has ended.
func (m *Manager) Stop(id, owner string) (Status, error) {
	j, err := m.get(id, owner)
	if err != nil {
		return Status{}, err
	}
	m.stop(j, StateStopped)
	return j.snapshot(m.now()), nil
}

// Status reads job id.
func (m *Manager) Status(id, owner string) (Status, error) {
	j, err := m.get(id, owner)
	if err != nil {
		return Status{}, err
	}
	return j.snapshot(m.now()), nil
}

// Wait blocks until job id ends or d passes, then reads it.
func (m *Manager) Wait(id, owner string, d time.Duration) (Status, error) {
	j, err := m.get(id, owner)
	if err != nil {
		return Status{}, err
	}
	select {
	case <-j.done:
	case <-time.After(d):
	}
	return j.snapshot(m.now()), nil
}

// get finds a job; a job belongs to the run that started it.
func (m *Manager) get(id, owner string) (*job, error) {
	m.mu.Lock()
	j := m.jobs[id]
	m.mu.Unlock()
	if j == nil || (j.spec.Owner != "" && j.spec.Owner != owner) {
		return nil, fmt.Errorf("no job %q", id)
	}
	return j, nil
}

func (j *job) snapshot(now time.Time) Status {
	j.mu.Lock()
	defer j.mu.Unlock()
	st := j.status
	st.Argv = append([]string(nil), st.Argv...)
	end := now
	if !st.FinishedAt.IsZero() {
		end = st.FinishedAt
	}
	st.ElapsedMS = end.Sub(st.StartedAt).Milliseconds()
	if !st.Terminal() {
		st.Tail = j.tail.String()
	}
	return st
}
