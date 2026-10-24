package yaegikernel

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsulejob"
)

// Capsule jobs, in-cell (docs/design/capsule-long-exec-2026-10-15.md): a
// long build or test is a job, not a choir.Exec. choir.Start stages the
// job's start in the tray, like Freeze, and returns its id at once; the
// capsule executor starts it after the cell returns and wakes the desk when
// it ends. choir.Poll reads the job as the cell-start snapshot showed it.
// No process, handle or fd ever enters the interpreter.

// IntentJob stages a job start or stop (JobOp) for the executor.
const IntentJob = "job"

// Job operations.
const (
	JobOpStart = "start"
	JobOpStop  = "stop"
)

// JobStateStarting is how Poll shows a job this cell staged: it starts after
// the cell returns.
const JobStateStarting = "starting"

// JobView is one capsule job as a cell sees it at cell start.
type JobView struct {
	ID           string   `json:"id"`
	State        string   `json:"state"`
	Argv         []string `json:"argv"`
	Cwd          string   `json:"cwd"`
	ExitCode     *int     `json:"exit_code,omitempty"`
	Reason       string   `json:"reason,omitempty"`
	ElapsedSec   int64    `json:"elapsed_sec"`
	WallClockSec int64    `json:"wall_clock_sec"`
	OutputBytes  int64    `json:"output_bytes"`
	// Tail is the last few KiB of the job's combined output.
	Tail string `json:"tail,omitempty"`
	// ReceiptRef is the job's execution receipt, once it has ended: cite it
	// in Freeze (a done job only) or Complete.
	ReceiptRef string `json:"receipt_ref,omitempty"`
	// LogRef names the job's whole log, stored by content address.
	LogRef string `json:"log_ref,omitempty"`
}

// String prints the job as JSON, so println(st) shows its fields.
func (j JobView) String() string {
	b, err := json.Marshal(j)
	if err != nil {
		return fmt.Sprintf("{id:%s state:%s}", j.ID, j.State)
	}
	return string(b)
}

// jobOptions reads choir.Start's options: cwd (absolute, default
// /workspace/platform), minutes (wall clock, default 20, at most 45) and
// memory_mib (memory the job needs beyond its capsule, whole MiB).
func jobOptions(opts map[string]any) (cwd string, wallMS int64, memoryMiB int, err error) {
	keys := make([]string, 0, len(opts))
	for k := range opts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := opts[k]
		switch k {
		case "cwd":
			s, ok := v.(string)
			if !ok {
				return "", 0, 0, fmt.Errorf("choir.Start option cwd must be a string, not %T", v)
			}
			cwd = s
		case "minutes":
			m, err := jobNumber("minutes", v)
			if err != nil {
				return "", 0, 0, err
			}
			wallMS = int64(m * float64(time.Minute/time.Millisecond))
		case "memory_mib":
			m, err := jobNumber("memory_mib", v)
			if err != nil {
				return "", 0, 0, err
			}
			if m != math.Trunc(m) {
				return "", 0, 0, fmt.Errorf("choir.Start option memory_mib must be a whole number of MiB")
			}
			if m > capsulejob.MaxMemoryMiB {
				return "", 0, 0, fmt.Errorf("choir.Start option memory_mib %v is above the %d MiB a job may ask for", m, capsulejob.MaxMemoryMiB)
			}
			memoryMiB = int(m)
		default:
			return "", 0, 0, fmt.Errorf("choir.Start has no option %q (cwd, minutes, memory_mib)", k)
		}
	}
	return cwd, wallMS, memoryMiB, nil
}

// jobNumber reads a positive number option.
func jobNumber(name string, v any) (float64, error) {
	var m float64
	switch n := v.(type) {
	case int:
		m = float64(n)
	case int64:
		m = float64(n)
	case float64:
		m = n
	default:
		return 0, fmt.Errorf("choir.Start option %s must be a number, not %T", name, v)
	}
	if m <= 0 || math.IsNaN(m) || math.IsInf(m, 0) {
		return 0, fmt.Errorf("choir.Start option %s must be positive", name)
	}
	return m, nil
}

// JobStart stages a job start. At most one per cell, and not in a cell
// that stages Freeze or Complete (the job would start after them).
func (t *Tray) JobStart(spec capsulejob.Spec) (string, error) {
	for _, in := range t.intents {
		switch {
		case in.Kind == IntentJob && in.JobOp == JobOpStart:
			return "", fmt.Errorf("tray: at most one job start per cell")
		case in.Kind == IntentFreeze || in.Kind == IntentComplete:
			return "", fmt.Errorf("tray: a job cannot start in a cell that stages %s", in.Kind)
		}
	}
	return t.stage(StagedIntent{Kind: IntentJob, JobOp: JobOpStart, JobID: spec.ID, Argv: append([]string(nil), spec.Argv...), Cwd: spec.Cwd, WallClockMS: spec.WallClockMS, MemoryMiB: spec.MemoryMiB})
}

// JobStop stages a job stop.
func (t *Tray) JobStop(id string) (string, error) {
	for _, in := range t.intents {
		if in.Kind == IntentFreeze || in.Kind == IntentComplete {
			return "", fmt.Errorf("tray: a job stop cannot share a cell with %s", in.Kind)
		}
	}
	return t.stage(StagedIntent{Kind: IntentJob, JobOp: JobOpStop, JobID: id})
}

// hasJobIntent reports whether the tray stages a job start or stop.
func (t *Tray) hasJobIntent() bool {
	for _, in := range t.intents {
		if in.Kind == IntentJob {
			return true
		}
	}
	return false
}

// Start stages a capsule job running argv (no shell: pass "sh", "-c", ...
// for one) and returns its id. The executor starts it after this cell
// returns; when it ends you are woken, its record arrives in
// choir.Records(), and choir.Poll(id) shows its exit code, output tail and
// receipt. opts: "cwd" (default /workspace/platform), "minutes" (wall
// clock, default 20, at most 45), "memory_mib" (memory the job needs
// beyond its capsule). One job runs at a time in a capsule.
func (s *ChoirScope) Start(argv []string, opts map[string]any) (string, error) {
	if err := s.mutateDenied("Exec"); err != nil {
		return "", err
	}
	if !s.deskModule("Start") {
		return "", fmt.Errorf("choir: jobs are engineering's")
	}
	if s.tray == nil {
		return "", fmt.Errorf("choir: Start requires a bound cell")
	}
	cwd, wallMS, memoryMiB, err := jobOptions(opts)
	if err != nil {
		return "", err
	}
	spec, err := capsulejob.Spec{ID: capsulejob.NewID(), Argv: argv, Cwd: cwd, WallClockMS: wallMS, MemoryMiB: memoryMiB}.Normalize()
	if err != nil {
		return "", fmt.Errorf("choir.Start: %w", err)
	}
	if _, err := s.tray.JobStart(spec); err != nil {
		return "", err
	}
	return spec.ID, nil
}

// Stop stages a stop of job id; the job ends with state stopped.
func (s *ChoirScope) Stop(id string) error {
	if !s.deskModule("Stop") {
		return fmt.Errorf("choir: jobs are engineering's")
	}
	if s.tray == nil {
		return fmt.Errorf("choir: Stop requires a bound cell")
	}
	id = strings.TrimSpace(id)
	if !capsulejob.ValidID(id) {
		return fmt.Errorf("choir.Stop: %q is not a job id", id)
	}
	_, err := s.tray.JobStop(id)
	return err
}

// Poll returns job id as this cell's start snapshot showed it, or as
// "starting" when this cell staged it. It never waits in the cell; a job it
// returns unfinished is recorded as awaited, and the host ends the turn
// after the cell and resumes it when the job ends
// (problems/engineering-busy-polls-its-own-job-2026-10-24.md).
func (s *ChoirScope) Poll(id string) (JobView, error) {
	id = strings.TrimSpace(id)
	for _, j := range s.jobs {
		if j.ID == id {
			j.Argv = append([]string(nil), j.Argv...)
			if !capsulejob.Terminal(j.State) {
				s.noteAwaited(id)
			}
			return j, nil
		}
	}
	if s.tray != nil {
		for _, in := range s.tray.intents {
			if in.Kind == IntentJob && in.JobOp == JobOpStart && in.JobID == id {
				s.noteAwaited(id)
				return JobView{ID: id, State: JobStateStarting, Argv: append([]string(nil), in.Argv...), Cwd: in.Cwd,
					WallClockSec: in.WallClockMS / 1000, Reason: "staged in this cell; it starts after the cell returns"}, nil
			}
		}
	}
	known := make([]string, 0, len(s.jobs))
	for _, j := range s.jobs {
		known = append(known, j.ID+" ("+j.State+")")
	}
	if len(known) == 0 {
		return JobView{}, fmt.Errorf("choir.Poll: no job %q; this capsule has no jobs", id)
	}
	return JobView{}, fmt.Errorf("choir.Poll: no job %q; jobs: %s", id, strings.Join(known, ", "))
}

// noteAwaited records a job a Poll in the bound cell returned unfinished,
// once.
func (s *ChoirScope) noteAwaited(id string) {
	s.awaitMu.Lock()
	defer s.awaitMu.Unlock()
	for _, a := range s.awaited {
		if a == id {
			return
		}
	}
	s.awaited = append(s.awaited, id)
}

// takeAwaited returns and clears the jobs the bound cell awaited.
func (s *ChoirScope) takeAwaited() []string {
	s.awaitMu.Lock()
	defer s.awaitMu.Unlock()
	out := s.awaited
	s.awaited = nil
	return out
}
