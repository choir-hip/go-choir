// Package capsulejob runs long commands inside an engineering capsule:
// builds and tests that outlast choir.Exec's 30 s bound
// (docs/design/capsule-long-exec-2026-10-15.md).
//
// A job runs under a supervisor process (Supervise) that is a child
// subreaper: it starts the command in its own process group, forwards a
// stop, and kills and reaps every descendant when the command ends. The
// broker side (Manager) owns the job table, the wall clock, the log and its
// digest. The same mechanics serve the Claude delegate
// (internal/claudedelegate): process-group kill, atomic status files, the
// bounded tail and the wait cap live here once.
package capsulejob

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Job states.
const (
	StateRunning = "running"
	StateDone    = "done"    // the command exited 0
	StateFailed  = "failed"  // it exited non-zero, died on a signal, or could not start
	StateTimeout = "timeout" // the wall clock stopped it
	StateStopped = "stopped" // a stop was asked, or the capsule ended
	// StateInterrupted closes a job a guest restart found open (CLAUDE.md
	// restart rule: a crash never resumes work).
	StateInterrupted = "interrupted"
)

// Terminal reports whether state ends a job.
func Terminal(state string) bool {
	switch state {
	case StateDone, StateFailed, StateTimeout, StateStopped, StateInterrupted:
		return true
	}
	return false
}

// Bounds and defaults.
const (
	// DefaultCwd is where a job runs unless it names another directory:
	// the capsule's source tree (choir.Exec runs at /, which misses it).
	DefaultCwd = "/workspace/platform"
	// Root holds one directory per job inside the capsule: the capsule's
	// scratch /tmp, which the classifier ignores, so nothing here is part
	// of the frozen candidate, and it ends with the capsule.
	Root = "/tmp/choir-jobs"
	// LogFile and StatusFile are a job directory's files. Cells may read
	// both; neither is authority (the broker's memory is).
	LogFile    = "output.log"
	StatusFile = "status.json"

	DefaultWallClock = 20 * time.Minute
	MaxWallClock     = 45 * time.Minute
	MinWallClock     = 5 * time.Second

	// MaxLogBytes caps the stored log; bytes past it are counted, not kept.
	MaxLogBytes = 64 << 20
	// TailBytes bounds the output tail any context ever sees.
	TailBytes = 4 << 10
	// KillGrace is how long a stopped command has between SIGTERM and
	// SIGKILL.
	KillGrace = 5 * time.Second
	// ReapDeadline bounds the supervisor's sweep of orphans after the
	// command ends.
	ReapDeadline = 10 * time.Second
	// WaitMaxSeconds keeps one wait under choir.Exec's 30 s bound (the
	// Claude delegate's `choir-delegate wait`).
	WaitMaxSeconds = 25
	// MaxMemoryMiB bounds a job's declared memory need: the default hotplug
	// region (docs/archive/reports/memory-hotplug-controller-design-2026-10-14.md
	// section 9).
	MaxMemoryMiB = 4096

	maxArgs      = 256
	maxArgvBytes = 16 << 10
	maxIDLen     = 64
)

// Spec is what a job runs. ID is chosen by the caller (the cell mints it, so
// the cell holds it before the job starts); Owner binds the job to the run
// that started it and is never model input.
type Spec struct {
	ID          string   `json:"id"`
	Owner       string   `json:"owner,omitempty"`
	Argv        []string `json:"argv"`
	Cwd         string   `json:"cwd,omitempty"`
	WallClockMS int64    `json:"wall_clock_ms,omitempty"`
	// MemoryMiB is the memory the job needs beyond its capsule's own size
	// (a browser capture, a large Go link); 0 is none. The executor admits
	// it against the guest's live capsule budget and raises the capsule's
	// cgroup by it; on a hotplug computer a memory lease raises that budget
	// first.
	MemoryMiB int `json:"memory_mib,omitempty"`
}

// WallClock is the spec's wall clock.
func (s Spec) WallClock() time.Duration { return time.Duration(s.WallClockMS) * time.Millisecond }

// Normalize fills the defaults (cwd, wall clock) and validates the spec.
func (s Spec) Normalize() (Spec, error) {
	s.ID = strings.TrimSpace(s.ID)
	if !ValidID(s.ID) {
		return s, fmt.Errorf("job id %q is invalid (letters, digits, - and _, at most %d)", s.ID, maxIDLen)
	}
	if len(s.Argv) == 0 || strings.TrimSpace(s.Argv[0]) == "" {
		return s, fmt.Errorf("job %s needs a command (argv[0])", s.ID)
	}
	if len(s.Argv) > maxArgs {
		return s, fmt.Errorf("job %s has %d arguments, more than %d", s.ID, len(s.Argv), maxArgs)
	}
	size := 0
	for _, a := range s.Argv {
		if strings.ContainsRune(a, 0) {
			return s, fmt.Errorf("job %s has an argument with a NUL byte", s.ID)
		}
		size += len(a)
	}
	if size > maxArgvBytes {
		return s, fmt.Errorf("job %s argv is %d bytes, more than %d", s.ID, size, maxArgvBytes)
	}
	cwd := strings.TrimSpace(s.Cwd)
	if cwd == "" {
		cwd = DefaultCwd
	}
	if !filepath.IsAbs(cwd) || filepath.Clean(cwd) != cwd || strings.ContainsRune(cwd, 0) {
		return s, fmt.Errorf("job %s cwd %q must be an absolute, clean path", s.ID, s.Cwd)
	}
	s.Cwd = cwd
	if s.MemoryMiB < 0 || s.MemoryMiB > MaxMemoryMiB {
		return s, fmt.Errorf("job %s memory_mib %d is outside [0, %d]", s.ID, s.MemoryMiB, MaxMemoryMiB)
	}
	switch wall := s.WallClock(); {
	case s.WallClockMS == 0:
		s.WallClockMS = DefaultWallClock.Milliseconds()
	case wall < MinWallClock || wall > MaxWallClock:
		return s, fmt.Errorf("job %s wall clock %s is outside [%s, %s]", s.ID, wall, MinWallClock, MaxWallClock)
	}
	return s, nil
}

// ValidID reports whether id can name a job (and its directory): letters,
// digits, '-' and '_', at most 64 bytes. It also serves the delegate's run
// ids.
func ValidID(id string) bool {
	if id == "" || len(id) > maxIDLen {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

// NewID mints a job id: "job-" and 12 random hex digits.
func NewID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return "job-" + hex.EncodeToString(b[:])
}

// EnvDigest is the sha256 (hex) of the job's environment, sorted, one
// KEY=VALUE per line: the receipt binds which allowlisted environment ran.
func EnvDigest(env []string) string {
	sorted := append([]string(nil), env...)
	sort.Strings(sorted)
	sum := sha256.Sum256([]byte(strings.Join(sorted, "\n")))
	return hex.EncodeToString(sum[:])
}

// Status is a job as the broker knows it: the authority the executor mints
// a receipt from.
type Status struct {
	ID          string    `json:"id"`
	State       string    `json:"state"`
	Argv        []string  `json:"argv"`
	Cwd         string    `json:"cwd"`
	EnvDigest   string    `json:"env_digest,omitempty"`
	WallClockMS int64     `json:"wall_clock_ms"`
	StartedAt   time.Time `json:"started_at"`
	FinishedAt  time.Time `json:"finished_at,omitzero"`
	ElapsedMS   int64     `json:"elapsed_ms"`
	ExitCode    *int      `json:"exit_code,omitempty"`
	Signal      string    `json:"signal,omitempty"`
	Reason      string    `json:"reason,omitempty"`
	// OutputSHA256 is the hex sha256 of the bytes stored in the log (the
	// first MaxLogBytes of the combined stdout and stderr); OutputBytes
	// counts every byte the command wrote, Truncated says some were not
	// stored.
	OutputSHA256 string `json:"output_sha256,omitempty"`
	OutputBytes  int64  `json:"output_bytes"`
	Truncated    bool   `json:"truncated,omitempty"`
	// Orphans counts descendants the supervisor could not reap in time.
	Orphans int    `json:"orphans,omitempty"`
	Tail    string `json:"tail,omitempty"`
	LogPath string `json:"log_path,omitempty"`
}

// Terminal reports whether the job has ended.
func (s Status) Terminal() bool { return Terminal(s.State) }

// Outcome is what the supervisor reports about the command it ran.
type Outcome struct {
	State    string `json:"state"`
	ExitCode int    `json:"exit_code"`
	Signal   string `json:"signal,omitempty"`
	SigNum   int    `json:"signal_number,omitempty"`
	Reason   string `json:"reason,omitempty"`
	ChildPID int    `json:"child_pid,omitempty"`
	Orphans  int    `json:"orphans,omitempty"`
}

// ExitStatus is the process exit status a supervisor stage ends with: the
// command's own code, 128+signal for a signal death, 127 when it could not
// start.
func (o Outcome) ExitStatus() int {
	switch {
	case o.ChildPID == 0:
		return 127
	case o.SigNum > 0:
		return 128 + o.SigNum
	case o.ExitCode < 0 || o.ExitCode > 255:
		return 1
	default:
		return o.ExitCode
	}
}
