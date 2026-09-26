package yaegikernel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// DeskSessionWorkerConfig is the host-side analogue of the capsule-broker
// workerSessionConfig: everything a host session worker needs to serve
// framed eval cells — the desk's executable path, the SessionWorkerConfig
// the worker re-executes into, and the process hardening. Role bounds the
// prebound choir surface; it is a trusted host flag, never model input.
type DeskSessionWorkerConfig struct {
	// Bin is the host executable re-executed as the worker (the daemon's own
	// path or a desk-worker binary). When empty the current process's
	// executable is used (os.Executable).
	Bin string
	// WorkerArg selects the worker mode the binary re-execs into (e.g.
	// "desk-session" / "--isolation-stage exec-go-session"). Passed as the
	// first argv element.
	WorkerArg string
	// EnvPrefix prefixes the per-config environment variables the worker
	// reads (defaults to CHOIR_DESK_SESSION_). The worker side reads them via
	// SessionWorkerConfigFromEnv.
	EnvPrefix string
	// Session is the SessionWorkerConfig handed to the worker.
	Session SessionWorkerConfig
	// ReadyTimeout bounds the post-prebind ready handshake; a process that
	// starts but never proves readiness is killed, never trusted. Defaults
	// to sessionReadyTimeout.
	ReadyTimeout time.Duration
	// ProcessGroup isolates the worker into its own process group so a kill
	// reaches every descendant (mirrors the capsule spawn). Default true.
	ProcessGroup bool
	// ExtraEnv appends worker-readable environment (test markers, child
	// selectors) after the sanitized PATH/TMPDIR and session config vars.
	ExtraEnv []string
}

// sessionReadyTimeout bounds the worker's post-prebind ready handshake. A
// process that starts but never proves readiness (missing binary entry,
// rejected identity, failed prebind) is killed, never trusted.
const sessionReadyTimeout = 10 * time.Second

// DeskSessionWorker owns one persistent host worker process serving framed
// eval cells for a single desk activation. At most one in-flight Eval runs at
// a time; a timeout, poisoned cell, or transport error kills the whole
// process group. Transport is the multiplexed Unix socketpair (Step 2): the
// host keeps one end, the worker inherits the other as fd 3 — worker traffic
// never touches process stdio; stdout/stderr carry only crash diagnostics.
type DeskSessionWorker struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	framed *FramedConn
	pid    int
	dead   bool
	cfg    DeskSessionWorkerConfig
}

var deskSessionFrameID uint64
var deskSessionFrameMu sync.Mutex

func nextDeskSessionFrameID() string {
	deskSessionFrameMu.Lock()
	defer deskSessionFrameMu.Unlock()
	deskSessionFrameID++
	return fmt.Sprintf("cell-%d", deskSessionFrameID)
}

// SpawnDeskSessionWorker starts a host session worker process in its own
// process group with a sanitized environment, mirroring the capsule-broker
// spawn hardening. The worker binary is re-executed with cfg.WorkerArg; its
// SessionWorkerConfig rides CHOIR_DESK_SESSION_* env vars (never argv for
// secret-adjacent fields). The child end of the socketpair is passed as
// ExtraFiles fd 3; the host keeps the parent end wrapped in FramedConn.
func SpawnDeskSessionWorker(cfg DeskSessionWorkerConfig) (*DeskSessionWorker, error) {
	bin := cfg.Bin
	if bin == "" {
		var err error
		bin, err = os.Executable()
		if err != nil {
			return nil, fmt.Errorf("desk worker binary: %w", err)
		}
	}
	workerArg := cfg.WorkerArg
	if workerArg == "" {
		return nil, fmt.Errorf("desk worker arg required (worker mode entrypoint)")
	}
	fds, err := unixSocketpairFiles()
	if err != nil {
		return nil, fmt.Errorf("desk session socketpair: %w", err)
	}
	parentFile := fds[0]
	childFile := fds[1]
	parentConn, err := net.FileConn(parentFile)
	_ = parentFile.Close()
	if err != nil {
		_ = childFile.Close()
		return nil, fmt.Errorf("desk session parent socket: %w", err)
	}
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd := exec.Command(bin, workerArg)
	if cfg.Session.AllowedRoot != "" {
		cmd.Dir = cfg.Session.AllowedRoot
	}
	cmd.SysProcAttr = workerSysProcAttr(cfg.ProcessGroup)
	cmd.Env = append(append([]string{"PATH=/usr/bin:/bin:/usr/local/bin", "TMPDIR=/tmp"}, cfg.sessionEnv()...), cfg.ExtraEnv...)
	cmd.ExtraFiles = []*os.File{childFile}
	cmd.Stdin = nil
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	if err := cmd.Start(); err != nil {
		_ = childFile.Close()
		_ = parentConn.Close()
		return nil, fmt.Errorf("start desk session worker: %w", err)
	}
	_ = childFile.Close()
	w := &DeskSessionWorker{
		cmd:    cmd,
		framed: NewFramedConn(parentConn),
		pid:    cmd.Process.Pid,
		cfg:    cfg,
	}
	timeout := cfg.ReadyTimeout
	if timeout <= 0 {
		timeout = sessionReadyTimeout
	}
	if err := w.awaitReady(timeout); err != nil {
		w.killLocked()
		return nil, fmt.Errorf("%w (worker stdout=%q stderr=%q)", err, stdoutBuf.String(), stderrBuf.String())
	}
	return w, nil
}

// sessionEnv renders the SessionWorkerConfig as worker-readable env vars.
func (c DeskSessionWorkerConfig) sessionEnv() []string {
	prefix := c.EnvPrefix
	if prefix == "" {
		prefix = "CHOIR_DESK_SESSION_"
	}
	env := []string{
		prefix + "COMPUTER_ID=" + c.Session.ComputerID,
		prefix + "ACTIVATION=" + c.Session.ActivationID,
		prefix + "EPOCH=" + fmt.Sprintf("%d", c.Session.Epoch),
		prefix + "ALLOWED_ROOT=" + c.Session.AllowedRoot,
		prefix + "ROLE=" + c.Session.Role,
		prefix + "SLOT=" + c.Session.Slot,
		prefix + "SOCK_FD=3",
		prefix + "MEMORY_LIMIT_BYTES=" + fmt.Sprintf("%d", c.Session.MemoryLimitBytes),
	}
	if len(c.Session.AllowedPackages) > 0 {
		env = append(env, prefix+"ALLOWED_PACKAGES="+joinCSV(c.Session.AllowedPackages))
	}
	return env
}

// SessionWorkerConfigFromEnv reconstructs the config the host spawner wrote —
// the worker-side half of the spawn contract. prefix defaults to
// CHOIR_DESK_SESSION_; fd is the inherited session socket descriptor (3).
func SessionWorkerConfigFromEnv(getenv func(string) string, prefix string) SessionWorkerConfig {
	if getenv == nil {
		getenv = os.Getenv
	}
	if prefix == "" {
		prefix = "CHOIR_DESK_SESSION_"
	}
	var epoch uint64
	if v := getenv(prefix + "EPOCH"); v != "" {
		fmt.Sscanf(v, "%d", &epoch)
	}
	return SessionWorkerConfig{
		AllowedPackages:  splitCSV(getenv(prefix + "ALLOWED_PACKAGES")),
		ComputerID:       getenv(prefix + "COMPUTER_ID"),
		ActivationID:     getenv(prefix + "ACTIVATION"),
		Epoch:            epoch,
		AllowedRoot:      getenv(prefix + "ALLOWED_ROOT"),
		Role:             getenv(prefix + "ROLE"),
		Slot:             getenv(prefix + "SLOT"),
		MemoryLimitBytes: parseUint64Env(getenv(prefix + "MEMORY_LIMIT_BYTES")),
	}
}

// parseUint64Env parses a numeric env value; malformed input reads as 0
// (uncapped) rather than silently inheriting a bogus bound.
func parseUint64Env(v string) uint64 {
	var n uint64
	if v != "" {
		_, _ = fmt.Sscanf(v, "%d", &n)
	}
	return n
}

// SessionWorkerSockFD reads the inherited session socket fd (3) into a
// net.Conn the worker serves via ExecuteWorkerSessionConn. Returns the fd
// number and the wrapped conn; fd<0 means no inherited socket (stdio path).
func SessionWorkerSockFD(getenv func(string) string, prefix string) (int, net.Conn, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	if prefix == "" {
		prefix = "CHOIR_DESK_SESSION_"
	}
	fd := -1
	if v := getenv(prefix + "SOCK_FD"); v != "" {
		fmt.Sscanf(v, "%d", &fd)
	}
	if fd < 0 {
		return -1, nil, nil
	}
	f := os.NewFile(uintptr(fd), "desk-session-sock")
	if f == nil {
		return fd, nil, fmt.Errorf("desk session sock fd %d invalid", fd)
	}
	conn, err := net.FileConn(f)
	_ = f.Close()
	if err != nil {
		return fd, nil, fmt.Errorf("desk session sock fd %d: %w", fd, err)
	}
	return fd, conn, nil
}

// awaitReady consumes the worker's post-prebind ready result frame from the
// session socket, so a process that starts but never proves readiness is
// killed, never trusted.
func (w *DeskSessionWorker) awaitReady(timeout time.Duration) error {
	type result struct {
		stream  byte
		payload []byte
		err     error
	}
	ch := make(chan result, 1)
	go func() {
		stream, payload, err := w.framed.ReadFrame()
		ch <- result{stream, payload, err}
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case r := <-ch:
		if r.err != nil {
			return fmt.Errorf("desk session worker ready: %w", r.err)
		}
		if r.stream != StreamCell {
			return fmt.Errorf("desk session worker ready: unexpected stream %d", r.stream)
		}
		var res SessionResult
		if err := json.Unmarshal(r.payload, &res); err != nil || res.ID != "ready" {
			return fmt.Errorf("desk session worker ready: bad handshake")
		}
		return nil
	case <-timer.C:
		return fmt.Errorf("desk session worker ready: timed out")
	}
}

// Eval sends one cell to the worker and returns its SessionResult. One
// in-flight Eval at a time; concurrent Evals serialize on w.mu. A poisoned
// cell, transport error, or timeout marks the worker dead — the caller
// respawns rather than reusing poisoned state.
func (w *DeskSessionWorker) Eval(ctx context.Context, source string) (SessionResult, error) {
	return w.EvalInbox(ctx, source, nil)
}

// EvalInbox runs one cell and injects the cell-start inbox snapshot into the
// session frame so choir.Inbox() inside the cell reads delivered acts
// without a network roundtrip — the desk analogue of GoEvalRequest.Inbox.
func (w *DeskSessionWorker) EvalInbox(ctx context.Context, source string, inbox []IncomingMessage) (SessionResult, error) {
	return w.EvalCell(ctx, source, inbox, nil, nil)
}

// EvalCell runs one cell with the full frame payload: the inbox snapshot,
// the acting desk's score-free commitment pack (R4, choir.Pack()), and for
// a texture desk the bound document's head (R3d, choir.ReadDoc()).
func (w *DeskSessionWorker) EvalCell(ctx context.Context, source string, inbox []IncomingMessage, doc *DocSnapshot, pack *types.ActingPack) (SessionResult, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.dead {
		return SessionResult{}, fmt.Errorf("desk session worker dead")
	}
	frame := SessionFrame{ID: nextDeskSessionFrameID(), Source: source, Inbox: inbox, Doc: doc, Pack: pack}
	raw, err := json.Marshal(frame)
	if err != nil {
		return SessionResult{}, fmt.Errorf("desk eval marshal: %w", err)
	}
	if err := w.framed.WriteFrame(StreamCell, raw); err != nil {
		w.dead = true
		return SessionResult{}, fmt.Errorf("desk eval write: %w", err)
	}
	type result struct {
		res SessionResult
		err error
	}
	ch := make(chan result, 1)
	go func() {
		stream, payload, err := w.framed.ReadFrame()
		if err != nil {
			ch <- result{err: err}
			return
		}
		if stream != StreamCell {
			ch <- result{err: fmt.Errorf("desk eval: unexpected stream %d", stream)}
			return
		}
		var res SessionResult
		if uerr := json.Unmarshal(payload, &res); uerr != nil {
			ch <- result{err: fmt.Errorf("desk eval decode: %w", uerr)}
			return
		}
		ch <- result{res: res}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			w.dead = true
			return SessionResult{}, r.err
		}
		if r.res.ID != frame.ID {
			w.dead = true
			return SessionResult{}, fmt.Errorf("desk eval: id mismatch got %q want %q", r.res.ID, frame.ID)
		}
		if r.res.Error != "" && r.res.Reuse != ReusePreserve {
			// Cell poisoned the worker — dead, never reused.
			w.dead = true
		}
		return r.res, nil
	case <-ctx.Done():
		w.killLocked()
		return SessionResult{}, ctx.Err()
	}
}

// Kill terminates the worker's process group and marks it dead. Staged
// intents die with the process — the tray lives only in worker memory, so a
// kill before the Eval reply returns reduces nothing (the derivable wake the
// reducer re-fires is what makes a kill recoverable).
func (w *DeskSessionWorker) Kill() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.killLocked()
}

func (w *DeskSessionWorker) killLocked() {
	w.dead = true
	if w.cmd != nil && w.cmd.Process != nil {
		killProcessGroup(w.cmd.Process.Pid)
		_ = w.cmd.Process.Kill()
		_ = w.cmd.Wait()
	}
	if w.framed != nil {
		_ = w.framed.Close()
	}
}

// PID exposes the worker's process id for receipts/observability.
func (w *DeskSessionWorker) PID() int { return w.pid }

// Dead reports whether the worker is unusable (killed or poisoned).
func (w *DeskSessionWorker) Dead() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.dead
}

// Close sends a close frame then releases the worker (graceful shutdown).
func (w *DeskSessionWorker) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.framed != nil {
		raw, _ := json.Marshal(SessionFrame{ID: nextDeskSessionFrameID(), Close: true})
		_ = w.framed.WriteFrame(StreamCell, raw)
	}
	w.dead = true
	if w.cmd != nil && w.cmd.Process != nil {
		killProcessGroup(w.cmd.Process.Pid)
		_ = w.cmd.Wait()
	}
	if w.framed != nil {
		_ = w.framed.Close()
	}
}

// unixSocketpairFiles returns two connected Unix-domain sockets as *os.File
// so the child end can ride exec.Cmd.ExtraFiles (fd 3) — net.Conn alone
// cannot be inherited.
func unixSocketpairFiles() ([2]*os.File, error) {
	var out [2]*os.File
	fds, err := socketpairFDs()
	if err != nil {
		return out, err
	}
	out[0] = os.NewFile(uintptr(fds[0]), "desk-session-parent")
	out[1] = os.NewFile(uintptr(fds[1]), "desk-session-child")
	if out[0] == nil || out[1] == nil {
		return out, fmt.Errorf("desk session socketpair: nil file")
	}
	return out, nil
}
