package claudedelegate

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsulejob"
)

// Run states.
const (
	StateRunning = "running"
	StateDone    = "done"    // claude exited 0
	StateFailed  = "failed"  // claude exited non-zero or could not start
	StateTimeout = "timeout" // the wall clock killed it
	StateStopped = "stopped" // stop was asked
)

// Files in a run directory. Cells may read all of them; none carries the
// token (every line is redacted before it is written).
const (
	SpecFile   = "spec.json"
	PromptFile = "prompt.txt"
	EventsFile = "events.jsonl"
	StderrFile = "stderr.log"
	StatusFile = "status.json"
)

const (
	maxEventsFileBytes = 256 << 20
	maxStderrFileBytes = 1 << 20
	maxResultTextBytes = 8 << 10
	killGrace          = 10 * time.Second
)

// Result is the stream-json result record, as Claude Code reports it.
type Result struct {
	Subtype           string          `json:"subtype,omitempty"`
	IsError           bool            `json:"is_error"`
	NumTurns          int             `json:"num_turns,omitempty"`
	SessionID         string          `json:"session_id,omitempty"`
	TotalCostUSD      *float64        `json:"total_cost_usd,omitempty"`
	DurationMS        int64           `json:"duration_ms,omitempty"`
	StopReason        string          `json:"stop_reason,omitempty"`
	Usage             json.RawMessage `json:"usage,omitempty"`
	ModelUsage        json.RawMessage `json:"modelUsage,omitempty"`
	PermissionDenials json.RawMessage `json:"permission_denials,omitempty"`
	Text              string          `json:"result,omitempty"`
}

// Progress counts what the stream showed so far.
type Progress struct {
	AssistantMessages int      `json:"assistant_messages"`
	ToolUses          int      `json:"tool_uses"`
	LastTools         []string `json:"last_tools,omitempty"`
	DroppedLines      int      `json:"dropped_lines,omitempty"`
}

// Status is status.json: what `choir-delegate wait` prints.
type Status struct {
	ID           string    `json:"id"`
	State        string    `json:"state"`
	Dir          string    `json:"dir"`
	StartedAt    time.Time `json:"started_at"`
	FinishedAt   time.Time `json:"finished_at,omitzero"`
	ElapsedSec   int64     `json:"elapsed_sec"`
	WallClockMin int       `json:"wall_clock_min"`
	MaxTurns     int       `json:"max_turns"`
	ExitCode     *int      `json:"exit_code,omitempty"`
	Reason       string    `json:"reason,omitempty"`
	Progress     Progress  `json:"progress"`
	EventsBytes  int64     `json:"events_bytes"`
	EventsSHA256 string    `json:"events_sha256,omitempty"`
	StderrTail   string    `json:"stderr_tail,omitempty"`
	Result       *Result   `json:"result,omitempty"`
}

// Launch builds the (unstarted) command that becomes the claude process for
// a run directory. The server sets its environment, stdio and process group.
type Launch func(runDir string) *exec.Cmd

// Server is the broker side: it holds the token and runs one delegate at a
// time. It never logs, prints or writes the token.
type Server struct {
	Root    string   // run directories live here
	BaseEnv []string // the claude environment before Env adds the token
	Launch  Launch
	Now     func() time.Time

	token []byte
	// wallClock overrides the spec's wall clock (tests only).
	wallClock time.Duration

	mu      sync.Mutex
	current *run
}

// NewServer keeps the token in memory only.
func NewServer(root string, token string, baseEnv []string, launch Launch) *Server {
	return &Server{Root: root, BaseEnv: baseEnv, Launch: launch, Now: time.Now, token: []byte(strings.TrimSpace(token))}
}

type run struct {
	mu       sync.Mutex
	status   Status
	cmd      *exec.Cmd
	stopping string // StateTimeout or StateStopped once a kill is asked
	done     chan struct{}
}

// Request is one line on the delegate socket.
type Request struct {
	Op   string          `json:"op"` // start | stop | status
	ID   string          `json:"id,omitempty"`
	Spec json.RawMessage `json:"spec,omitempty"`
}

// Response answers a Request.
type Response struct {
	OK        bool    `json:"ok"`
	Error     string  `json:"error,omitempty"`
	Available bool    `json:"available"`
	Model     string  `json:"model,omitempty"`
	Status    *Status `json:"status,omitempty"`
}

func (s *Server) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Handle answers one request.
func (s *Server) Handle(req Request) Response {
	switch req.Op {
	case "status":
		resp := Response{OK: true, Available: len(s.token) > 0, Model: Model}
		s.mu.Lock()
		if s.current != nil {
			st := s.current.snapshot(s.now())
			resp.Status = &st
		}
		s.mu.Unlock()
		return resp
	case "start":
		spec, err := DecodeSpec(req.Spec)
		if err != nil {
			return Response{Error: err.Error(), Available: len(s.token) > 0}
		}
		st, err := s.Start(spec)
		if err != nil {
			return Response{Error: err.Error(), Available: len(s.token) > 0}
		}
		return Response{OK: true, Available: true, Model: Model, Status: &st}
	case "stop":
		st, err := s.Stop(req.ID)
		if err != nil {
			return Response{Error: err.Error(), Available: len(s.token) > 0}
		}
		return Response{OK: true, Available: true, Status: &st}
	default:
		return Response{Error: fmt.Sprintf("unknown op %q (start, stop, status)", req.Op), Available: len(s.token) > 0}
	}
}

// Serve answers JSON lines on l until it closes.
func (s *Server) Serve(l net.Listener) error {
	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		go func(c net.Conn) {
			defer c.Close()
			_ = c.SetDeadline(time.Now().Add(30 * time.Second))
			var req Request
			if err := json.NewDecoder(io.LimitReader(c, 1<<20)).Decode(&req); err != nil {
				_ = json.NewEncoder(c).Encode(Response{Error: "request is not JSON"})
				return
			}
			_ = json.NewEncoder(c).Encode(s.Handle(req))
		}(conn)
	}
}

func newRunID(now time.Time) string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("d%s-%s", now.UTC().Format("20060102T150405"), hex.EncodeToString(b[:]))
}

// Start begins one run. One run at a time: a second start while one runs
// is refused.
func (s *Server) Start(spec Spec) (Status, error) {
	if len(s.token) == 0 {
		return Status{}, fmt.Errorf("the Claude delegate is unavailable in this capsule (no token)")
	}
	if s.Launch == nil {
		return Status{}, fmt.Errorf("the Claude delegate has no launcher")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current != nil {
		select {
		case <-s.current.done:
		default:
			return Status{}, fmt.Errorf("delegate run %s is still running; wait for it or stop it first", s.current.status.ID)
		}
	}
	now := s.now()
	id := newRunID(now)
	dir := filepath.Join(s.Root, id)
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(home, 0o700); err != nil {
		return Status{}, fmt.Errorf("create run directory: %w", err)
	}
	specBytes, _ := json.MarshalIndent(spec, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, SpecFile), specBytes, 0o600); err != nil {
		return Status{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, PromptFile), []byte(Prompt(spec)), 0o600); err != nil {
		return Status{}, err
	}
	env, err := Env(s.BaseEnv, home, string(s.token))
	if err != nil {
		return Status{}, err
	}
	prompt, err := os.Open(filepath.Join(dir, PromptFile))
	if err != nil {
		return Status{}, err
	}
	events, err := os.OpenFile(filepath.Join(dir, EventsFile), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		_ = prompt.Close()
		return Status{}, err
	}
	stderrFile, err := os.OpenFile(filepath.Join(dir, StderrFile), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		_ = prompt.Close()
		_ = events.Close()
		return Status{}, err
	}
	cmd := s.Launch(dir)
	cmd.Env = env
	cmd.Stdin = prompt
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = prompt.Close()
		_ = events.Close()
		_ = stderrFile.Close()
		return Status{}, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = prompt.Close()
		_ = events.Close()
		_ = stderrFile.Close()
		return Status{}, err
	}
	r := &run{done: make(chan struct{}), cmd: cmd, status: Status{
		ID: id, State: StateRunning, Dir: dir, StartedAt: now.UTC(),
		WallClockMin: spec.WallClockMin, MaxTurns: spec.MaxTurns,
	}}
	if err := cmd.Start(); err != nil {
		_ = prompt.Close()
		_ = events.Close()
		_ = stderrFile.Close()
		r.status.State, r.status.Reason, r.status.FinishedAt = StateFailed, "start claude: "+redactString(err.Error(), s.token), now.UTC()
		close(r.done)
		s.current = r
		r.write(s.now())
		return r.snapshot(s.now()), nil
	}
	_ = prompt.Close()
	s.current = r
	r.write(now)
	go s.supervise(r, spec, stdout, stderr, events, stderrFile)
	return r.snapshot(now), nil
}

func redactString(s string, secret []byte) string {
	return string(redactLine([]byte(s), secret))
}

// supervise pumps output, enforces the wall clock and records the end.
func (s *Server) supervise(r *run, spec Spec, stdout, stderr io.Reader, events, stderrFile *os.File) {
	defer close(r.done)
	wall := time.Duration(spec.WallClockMin) * time.Minute
	if s.wallClock > 0 {
		wall = s.wallClock
	}
	timer := time.AfterFunc(wall, func() { r.kill(StateTimeout) })
	defer timer.Stop()

	hash := sha256.New()
	var pumps sync.WaitGroup
	var lastResult []byte
	var resultMu sync.Mutex
	pumps.Add(2)
	go func() {
		defer pumps.Done()
		var written int64
		lastWrite := time.Time{}
		pumpLines(stdout, s.token, func(line []byte, dropped int) {
			if line == nil {
				line = []byte(fmt.Sprintf("{\"type\":\"choir_delegate\",\"dropped_line_bytes\":%d}\n", dropped))
				r.mu.Lock()
				r.status.Progress.DroppedLines++
				r.mu.Unlock()
			}
			if written+int64(len(line)) <= maxEventsFileBytes {
				n, _ := events.Write(line)
				written += int64(n)
				hash.Write(line[:n])
			}
			r.observe(line, &resultMu, &lastResult)
			r.mu.Lock()
			r.status.EventsBytes = written
			r.mu.Unlock()
			if now := s.now(); now.Sub(lastWrite) > 2*time.Second {
				lastWrite = now
				r.write(now)
			}
		})
	}()
	tail := capsulejob.NewTail(2048)
	go func() {
		defer pumps.Done()
		var written int64
		pumpLines(stderr, s.token, func(line []byte, dropped int) {
			if line == nil {
				line = []byte(fmt.Sprintf("[choir-delegate: dropped a %d-byte line]\n", dropped))
			}
			if written+int64(len(line)) <= maxStderrFileBytes {
				n, _ := stderrFile.Write(line)
				written += int64(n)
			}
			_, _ = tail.Write(line)
		})
	}()
	pumps.Wait()
	waitErr := r.cmd.Wait()
	_ = events.Close()
	_ = stderrFile.Close()

	now := s.now()
	r.mu.Lock()
	code := 0
	if r.cmd.ProcessState != nil {
		code = r.cmd.ProcessState.ExitCode()
	}
	r.status.ExitCode = &code
	r.status.FinishedAt = now.UTC()
	r.status.EventsSHA256 = "sha256:" + hex.EncodeToString(hash.Sum(nil))
	r.status.StderrTail = tail.String()
	resultMu.Lock()
	if len(lastResult) > 0 {
		var res Result
		if json.Unmarshal(lastResult, &res) == nil {
			if len(res.Text) > maxResultTextBytes {
				res.Text = res.Text[:maxResultTextBytes] + "…[truncated]"
			}
			r.status.Result = &res
		}
	}
	resultMu.Unlock()
	switch {
	case r.stopping != "":
		r.status.State = r.stopping
		if r.stopping == StateTimeout {
			r.status.Reason = fmt.Sprintf("wall clock of %d min reached; the run's process group was killed", spec.WallClockMin)
		} else {
			r.status.Reason = "stopped on request"
		}
	case waitErr == nil && code == 0:
		r.status.State = StateDone
	default:
		r.status.State = StateFailed
		if waitErr != nil {
			r.status.Reason = redactString(waitErr.Error(), s.token)
		}
	}
	r.mu.Unlock()
	r.write(now)
}

// observe counts progress and keeps the last result line.
func (r *run) observe(line []byte, resultMu *sync.Mutex, lastResult *[]byte) {
	var head struct {
		Type    string `json:"type"`
		Message struct {
			Content []struct {
				Type string `json:"type"`
				Name string `json:"name"`
			} `json:"content"`
		} `json:"message"`
	}
	if json.Unmarshal(bytes.TrimSpace(line), &head) != nil {
		return
	}
	switch head.Type {
	case "assistant":
		r.mu.Lock()
		r.status.Progress.AssistantMessages++
		for _, c := range head.Message.Content {
			if c.Type == "tool_use" {
				r.status.Progress.ToolUses++
				r.status.Progress.LastTools = append(r.status.Progress.LastTools, c.Name)
				if n := len(r.status.Progress.LastTools); n > 8 {
					r.status.Progress.LastTools = r.status.Progress.LastTools[n-8:]
				}
			}
		}
		r.mu.Unlock()
	case "result":
		resultMu.Lock()
		*lastResult = append([]byte(nil), line...)
		resultMu.Unlock()
	}
}

// kill ends the run's whole process group: SIGTERM, then SIGKILL after a
// grace period (capsulejob.KillGroup). The group's leader is the supervisor
// stage, which forwards the stop to claude's own group and reaps every
// descendant (docs/design/capsule-long-exec-2026-10-15.md, F2/F3).
func (r *run) kill(reason string) {
	r.mu.Lock()
	if r.stopping != "" || r.cmd == nil || r.cmd.Process == nil {
		r.mu.Unlock()
		return
	}
	select {
	case <-r.done:
		r.mu.Unlock()
		return
	default:
	}
	r.stopping = reason
	pid := r.cmd.Process.Pid
	r.mu.Unlock()
	capsulejob.KillGroup(pid, killGrace, r.done)
}

// Stop asks the current run to stop.
func (s *Server) Stop(id string) (Status, error) {
	s.mu.Lock()
	r := s.current
	s.mu.Unlock()
	if r == nil || (id != "" && r.status.ID != id) {
		return Status{}, fmt.Errorf("no delegate run %q", id)
	}
	r.kill(StateStopped)
	return r.snapshot(s.now()), nil
}

// Wait blocks until the current run ends or d passes (tests and shutdown).
func (s *Server) Wait(d time.Duration) (Status, error) {
	s.mu.Lock()
	r := s.current
	s.mu.Unlock()
	if r == nil {
		return Status{}, errors.New("no run")
	}
	select {
	case <-r.done:
	case <-time.After(d):
	}
	return r.snapshot(s.now()), nil
}

func (r *run) snapshot(now time.Time) Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	st := r.status
	end := now
	if !st.FinishedAt.IsZero() {
		end = st.FinishedAt
	}
	st.ElapsedSec = int64(end.Sub(st.StartedAt) / time.Second)
	st.Progress.LastTools = append([]string(nil), st.Progress.LastTools...)
	return st
}

// write replaces status.json atomically.
func (r *run) write(now time.Time) {
	st := r.snapshot(now)
	_ = capsulejob.WriteJSONAtomic(st.Dir, StatusFile, st)
}

// ReadStatus reads a run's status.json.
func ReadStatus(root, id string) (Status, error) {
	if !capsulejob.ValidID(id) {
		return Status{}, fmt.Errorf("invalid run id %q", id)
	}
	data, err := os.ReadFile(filepath.Join(root, id, StatusFile))
	if err != nil {
		return Status{}, err
	}
	var st Status
	if err := json.Unmarshal(data, &st); err != nil {
		return Status{}, err
	}
	return st, nil
}

// Call sends one request to the delegate socket.
func Call(socket string, req Request, timeout time.Duration) (Response, error) {
	conn, err := net.DialTimeout("unix", socket, timeout)
	if err != nil {
		return Response{}, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return Response{}, err
	}
	var resp Response
	if err := json.NewDecoder(bufio.NewReader(conn)).Decode(&resp); err != nil {
		return Response{}, err
	}
	return resp, nil
}
