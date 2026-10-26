package yaegikernel

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
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
	// Emit services StreamBrokerEmit frames the worker sends mid-cell for
	// ActionEmit. It performs the durable channel write + recipient wake on
	// host authority. Nil refuses emits rather than recording an in-worker
	// receipt that a cell death would silently lose.
	Emit EmitHandler
	// Egress services StreamBrokerEgress frames the worker sends mid-cell
	// for host-mediated network verbs (search_start / read /
	// read_capture). The host resolves the call against its deps and
	// returns the bounded result. Nil refuses the verb.
	Egress func(ctx context.Context, action BrokerAction, payload json.RawMessage) (json.RawMessage, error)
	// Refused reports a frame the daemon refused (outside the desk's table,
	// a forged token, a stale epoch) so the host can record it on the trace.
	Refused func(ctx context.Context, r FrameRefusal)
	// CgroupRoot is the desks cgroup the worker's budget cgroup is made in.
	// Empty reads DeskCgroupRootEnv. Tests only; a daemon sets the env.
	CgroupRoot string
	// Confinement is the OS boundary the worker runs inside
	// (desk_cell_confinement.go). The zero value confines as far as the
	// spawner can; it is never weaker than that.
	Confinement CellConfinement
}

// FrameRefusal is one refused worker frame.
type FrameRefusal struct {
	Role      string
	Action    BrokerAction
	Stream    byte
	Reason    string
	RequestID string
	WorkerPID int
	// Forged marks a frame that proves tampering (wrong or missing slot
	// token): the worker is ended, not only refused.
	Forged bool
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
	// stderr carries only crash diagnostics; it is read once the process
	// is reaped, when the copy into it has finished.
	stderr *lockedBuffer
	// transportLost: the session socket failed before the host ended the
	// worker, so the process may have exited on its own.
	transportLost bool
	reaped        bool
	exitCause     string
	// slotToken is the secret the host generated for this worker at spawn
	// and handed it through its environment (ARCH-07).
	slotToken []byte
	// cellTmpDir is the worker's private tmp dir, removed at reap.
	cellTmpDir string
	// budget is the worker's memory cgroup (nil without one), released at
	// reap.
	budget *workerCgroup
}

// envHasKey reports whether env sets key.
func envHasKey(env []string, key string) bool {
	for _, kv := range env {
		if strings.HasPrefix(kv, key+"=") {
			return true
		}
	}
	return false
}

// workerExitGrace is how long the host waits, after the session socket
// fails, for the worker to exit on its own before it kills the corpse, so a
// crash's own status (an exit code, the kernel's OOM kill) is what gets
// recorded (management delegation S1, R2-19).
const workerExitGrace = 2 * time.Second

// stderrTailBytes bounds the crash diagnostics an exit cause carries.
const stderrTailBytes = 600

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
	plan, err := planCellConfinement(cfg)
	if err != nil {
		return nil, err
	}
	// The memory budget: a cgroup of the worker's own, from its desk's
	// manifest row (RES-02). Refused, not skipped, in required mode.
	budget, err := planWorkerCgroup(cfg)
	if err != nil {
		removeCellTmpDir(plan.tmpDir)
		return nil, err
	}
	if budget != nil {
		if err := attachCgroup(plan.attr, budget); err != nil {
			budget.release()
			removeCellTmpDir(plan.tmpDir)
			return nil, err
		}
	}
	fds, err := unixSocketpairFiles()
	if err != nil {
		budget.release()
		removeCellTmpDir(plan.tmpDir)
		return nil, fmt.Errorf("desk session socketpair: %w", err)
	}
	parentFile := fds[0]
	childFile := fds[1]
	parentConn, err := net.FileConn(parentFile)
	_ = parentFile.Close()
	if err != nil {
		_ = childFile.Close()
		budget.release()
		removeCellTmpDir(plan.tmpDir)
		return nil, fmt.Errorf("desk session parent socket: %w", err)
	}
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		_ = parentConn.Close()
		_ = childFile.Close()
		budget.release()
		removeCellTmpDir(plan.tmpDir)
		return nil, fmt.Errorf("desk session slot token: %w", err)
	}
	var stdoutBuf, stderrBuf lockedBuffer
	cmd := exec.Command(bin, workerArg)
	cmd.Dir = plan.dir
	cmd.SysProcAttr = plan.attr
	// The confinement env comes last: its TMPDIR (the private tmp dir)
	// wins over the default (exec.Cmd keeps the last duplicate).
	cmd.Env = append(append(append([]string{"PATH=/usr/bin:/bin:/usr/local/bin", "TMPDIR=/tmp"}, cfg.sessionEnv()...), cfg.ExtraEnv...), plan.env...)
	// The slot token rides the environment, never argv (argv is visible to
	// every process through /proc).
	cmd.Env = append(cmd.Env, sessionEnvPrefix(cfg.EnvPrefix)+"SLOT_TOKEN="+hex.EncodeToString(token))
	if budget != nil && !envHasKey(cmd.Env, "GOMEMLIMIT") {
		// The collector works before the kernel kills: a soft limit below
		// memory.max (an operator's override, if any, was passed above).
		cmd.Env = append(cmd.Env, "GOMEMLIMIT="+strconv.FormatUint(budget.softHeapLimit(), 10))
	}
	cmd.ExtraFiles = []*os.File{childFile}
	cmd.Stdin = nil
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	if err := cmd.Start(); err != nil {
		_ = childFile.Close()
		_ = parentConn.Close()
		budget.release()
		removeCellTmpDir(plan.tmpDir)
		return nil, fmt.Errorf("start desk session worker: %w", err)
	}
	_ = childFile.Close()
	w := &DeskSessionWorker{
		cmd:        cmd,
		framed:     NewFramedConn(parentConn),
		pid:        cmd.Process.Pid,
		cfg:        cfg,
		stderr:     &stderrBuf,
		cellTmpDir: plan.tmpDir,
		budget:     budget,
		// The worker returns the hex form; the host compares that.
		slotToken: []byte(hex.EncodeToString(token)),
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
	if c.Session.KeepCellOutput {
		env = append(env, prefix+"KEEP_CELL_OUTPUT=1")
	}
	if carry := appDeskCarry(prefix, c.Session.Role); carry != "" {
		env = append(env, carry)
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
	prefix = sessionEnvPrefix(prefix)
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
		KeepCellOutput:   getenv(prefix+"KEEP_CELL_OUTPUT") == "1",
		SlotToken:        getenv(prefix + "SLOT_TOKEN"),
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
		if subtle.ConstantTimeCompare([]byte(res.SlotToken), w.slotToken) != 1 || len(w.slotToken) == 0 {
			return fmt.Errorf("desk session worker ready: slot token does not match the process the host started")
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
	return w.EvalCell(ctx, source, inbox, nil, nil, nil, nil, nil)
}

// EvalCell runs one cell with the full frame payload: the inbox snapshot,
// the acting desk's score-free commitment pack (R4, choir.Pack()), the
// pending update_coagent records (choir.Updates()), and for a texture desk
// the bound document's head (R3d, choir.ReadDoc()).
func (w *DeskSessionWorker) EvalCell(ctx context.Context, source string, inbox []IncomingMessage, doc *DocSnapshot, pack *types.ActingPack, updates []PendingUpdate, emits []PendingEmit, records []InboxRecord) (SessionResult, error) {
	return w.EvalFrame(ctx, SessionFrame{Source: source, Inbox: inbox, Doc: doc, Pack: pack, Updates: updates, Emits: emits, Records: records})
}

// EvalFrame runs one cell with a frame the caller filled; the worker gives
// it its id.
func (w *DeskSessionWorker) EvalFrame(ctx context.Context, frame SessionFrame) (SessionResult, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.dead {
		return SessionResult{}, fmt.Errorf("desk session worker dead")
	}
	frame.ID = nextDeskSessionFrameID()
	raw, err := json.Marshal(frame)
	if err != nil {
		return SessionResult{}, fmt.Errorf("desk eval marshal: %w", err)
	}
	if err := w.framed.WriteFrame(StreamCell, raw); err != nil {
		w.dead = true
		w.transportLost = true
		return SessionResult{}, fmt.Errorf("desk eval write: %w", err)
	}
	type result struct {
		res SessionResult
		err error
	}
	ch := make(chan result, 1)
	go func() {
		for {
			stream, payload, err := w.framed.ReadFrame()
			if err != nil {
				ch <- result{err: err}
				return
			}
			// A mid-cell ActionEmit arrives on its own stream; the host performs
			// the durable channel write + recipient wake, then answers and keeps
			// waiting for the cell's StreamCell result.
			if stream == StreamBrokerEmit {
				if forged := w.serviceEmit(ctx, payload); forged {
					ch <- result{err: errForgedFrame}
					return
				}
				continue
			}
			// Mid-cell host-mediated network verbs (search_start / read /
			// read_capture) ride the shared egress stream; the host resolves
			// them and answers, then keeps waiting for the StreamCell result.
			if stream == StreamBrokerEgress {
				if forged := w.serviceEgress(ctx, payload); forged {
					ch <- result{err: errForgedFrame}
					return
				}
				continue
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
			return
		}
	}()
	select {
	case r := <-ch:
		if errors.Is(r.err, errForgedFrame) {
			// A conforming worker never sends a frame it cannot sign: the
			// process is not the one the host started, or it has been taken
			// over. End it now rather than grant it a grace period.
			w.killLocked()
			return SessionResult{}, r.err
		}
		if r.err != nil {
			if errors.Is(r.err, ErrFrameVersion) && w.cfg.Refused != nil {
				// Refused from the header alone: there is no decoded action or
				// request id, but the refusal is still traced before the
				// worker ends.
				w.cfg.Refused(ctx, FrameRefusal{Role: w.cfg.Session.Role, Action: ActionTransport,
					Reason: "refused: " + r.err.Error(), WorkerPID: w.pid})
			}
			w.dead = true
			w.transportLost = true
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

// serviceEmit answers one StreamBrokerEmit frame. The worker sent a
// BrokerRequest carrying an EmitPayload; the host performs the durable
// channel write + recipient wake via cfg.Emit (host authority, never the
// worker's in-memory tray), then replies on StreamBrokerEmitResult so the
// blocked cell resumes.
func (w *DeskSessionWorker) serviceEmit(ctx context.Context, payload []byte) (forged bool) {
	var req BrokerRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		w.writeEmitResult(req.RequestID, nil, fmt.Errorf("emit decode request: %w", err))
		return false
	}
	if reason, forged := w.admitFrame(ctx, StreamBrokerEmit, req); reason != "" {
		w.writeEmitResult(req.RequestID, nil, errors.New(reason))
		return forged
	}
	var emit EmitPayload
	if err := json.Unmarshal(req.Payload, &emit); err != nil {
		w.writeEmitResult(req.RequestID, nil, fmt.Errorf("emit decode payload: %w", err))
		return false
	}
	handler := w.cfg.Emit
	if handler == nil {
		w.writeEmitResult(req.RequestID, nil, fmt.Errorf("emit host unavailable"))
		return false
	}
	res, err := handler(ctx, emit)
	w.writeEmitResult(req.RequestID, &res, err)
	return false
}

func (w *DeskSessionWorker) writeEmitResult(requestID string, res *EmitResult, cause error) {
	resp := NewErrorResponse(requestID, "", 0)
	if cause != nil {
		resp.Error = cause.Error()
	} else if ok, serr := NewSuccessResponse(requestID, res, "", 0); serr == nil {
		resp = ok
	} else {
		resp.Error = serr.Error()
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		return
	}
	_ = w.framed.WriteFrame(StreamBrokerEmitResult, raw)
}

// serviceEgress answers one StreamBrokerEgress frame. The worker sent a
// BrokerRequest whose action names a host-mediated network verb; the host
// resolves it via cfg.Egress (the deps-bound handler in agentcore) and
// replies on StreamBrokerEgressResult so the blocked cell resumes.
func (w *DeskSessionWorker) serviceEgress(ctx context.Context, payload []byte) (forged bool) {
	var req BrokerRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		w.writeEgressResult(req.RequestID, nil, fmt.Errorf("egress decode request: %w", err))
		return false
	}
	if reason, forged := w.admitFrame(ctx, StreamBrokerEgress, req); reason != "" {
		w.writeEgressResult(req.RequestID, nil, errors.New(reason))
		return forged
	}
	handler := w.cfg.Egress
	if handler == nil {
		w.writeEgressResult(req.RequestID, nil, fmt.Errorf("%s host unavailable", req.Action))
		return false
	}
	res, err := handler(withEgressFrame(ctx, req.RequestID, w), req.Action, req.Payload)
	w.writeEgressResult(req.RequestID, res, err)
	return false
}

// ActionTransport names a refusal that happened below the broker protocol: a
// frame header the host will not read (ErrFrameVersion). It is never a wire
// action and no desk's frame table names it.
const ActionTransport BrokerAction = "transport"

// errForgedFrame ends an EvalCell whose worker sent a frame that proves it is
// not the process the host started.
var errForgedFrame = errors.New("desk worker sent a forged frame")

// admitFrame is the daemon's check on every host-served frame (SEC-02,
// ARCH-07). Who sent the frame is decided by the worker the host spawned,
// never by a field in the frame: the role is the one in cfg.Session (set by
// the host from the run), the epoch is the worker's, and the slot token is the
// secret the host generated at spawn. A frame is served only when the desk's
// manifest table names its action; an unknown desk is served nothing.
//
// It returns a non-empty reason to refuse. forged is true when the frame
// proves tampering (a wrong or missing slot token): the worker is ended.
func (w *DeskSessionWorker) admitFrame(ctx context.Context, stream byte, req BrokerRequest) (reason string, forged bool) {
	refuse := func(reason string, forged bool) (string, bool) {
		if w.cfg.Refused != nil {
			w.cfg.Refused(ctx, FrameRefusal{Role: w.cfg.Session.Role, Action: req.Action, Stream: stream,
				Reason: reason, RequestID: req.RequestID, WorkerPID: w.pid, Forged: forged})
		}
		return reason, forged
	}
	if len(w.slotToken) == 0 {
		return refuse("refused: the host holds no slot token for this worker", false)
	}
	if subtle.ConstantTimeCompare([]byte(req.SlotToken), w.slotToken) != 1 {
		return refuse("refused: slot token does not match the worker the host started", true)
	}
	if req.ProtocolVersion != ProtocolVersion {
		return refuse(fmt.Sprintf("refused: protocol version %d, the host speaks %d", req.ProtocolVersion, ProtocolVersion), false)
	}
	if req.Epoch != w.cfg.Session.Epoch {
		return refuse(fmt.Sprintf("refused: epoch %d is not this worker's epoch %d", req.Epoch, w.cfg.Session.Epoch), false)
	}
	if stream == StreamBrokerEmit && req.Action != ActionEmit {
		return refuse(fmt.Sprintf("refused: the emit stream carries only %s, not %s", ActionEmit, req.Action), false)
	}
	role := w.cfg.Session.Role
	if !deskmanifest.FrameAllowed(role, string(req.Action)) {
		return refuse(fmt.Sprintf("refused: desk %q may not call %s", role, req.Action), false)
	}
	return "", false
}

type egressFrameKey struct{}

type egressFrame struct {
	requestID string
	worker    *DeskSessionWorker
}

func withEgressFrame(ctx context.Context, requestID string, w *DeskSessionWorker) context.Context {
	return context.WithValue(ctx, egressFrameKey{}, egressFrame{requestID: requestID, worker: w})
}

// EgressFrameFromContext returns the request id of the broker frame an Egress
// handler is serving and the worker process that sent it. The host answers
// on a result frame carrying the same request id (writeEgressResult).
func EgressFrameFromContext(ctx context.Context) (requestID string, workerPID int) {
	f, _ := ctx.Value(egressFrameKey{}).(egressFrame)
	if f.worker != nil {
		workerPID = f.worker.pid
	}
	return f.requestID, workerPID
}

// EgressWorkerFromContext returns the worker whose cell sent the egress
// frame an Egress handler is serving, so the handler can push search
// arrivals to that worker after it answers.
func EgressWorkerFromContext(ctx context.Context) *DeskSessionWorker {
	f, _ := ctx.Value(egressFrameKey{}).(egressFrame)
	return f.worker
}

// PushSearch writes one search frame to the worker. It may be called at any
// time, from any goroutine, during a cell or between cells: the worker's
// reader loop applies it to the session's search store. It does not take
// the Eval lock, which a running cell holds; frame writes serialize on the
// socket. A dead worker's socket is closed and the write fails.
func (w *DeskSessionWorker) PushSearch(f SearchFrame) error {
	raw, err := json.Marshal(f)
	if err != nil {
		return fmt.Errorf("search frame marshal: %w", err)
	}
	return w.framed.WriteFrame(StreamSearchArrival, raw)
}

func (w *DeskSessionWorker) writeEgressResult(requestID string, res json.RawMessage, cause error) {
	resp := NewErrorResponse(requestID, "", 0)
	if cause != nil {
		resp.Error = cause.Error()
	} else {
		resp.Success = true
		resp.Result = res
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		return
	}
	_ = w.framed.WriteFrame(StreamBrokerEgressResult, raw)
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
	w.reapLocked("killed by the host", 0)
	if w.framed != nil {
		_ = w.framed.Close()
	}
}

// reapLocked ends the worker's process group, waits for the process once,
// and records how it ended. After a lost transport it first gives the
// worker workerExitGrace to exit on its own, so a crash is recorded as the
// crash, not as the host's kill of the corpse.
func (w *DeskSessionWorker) reapLocked(hostReason string, grace time.Duration) {
	if w.reaped || w.cmd == nil || w.cmd.Process == nil {
		return
	}
	w.reaped = true
	defer removeCellTmpDir(w.cellTmpDir)
	defer func() {
		if w.budget != nil {
			// Say whether the memory budget ended the worker: a bare "signal:
			// killed" told a desk nothing, and the daemon's own OOM kill looked
			// the same.
			w.exitCause += w.budget.note()
			w.budget.release()
			w.budget = nil
		}
	}()
	done := make(chan error, 1)
	go func() { done <- w.cmd.Wait() }()
	if w.transportLost {
		if grace <= 0 {
			grace = workerExitGrace
		}
		select {
		case err := <-done:
			w.exitCause = w.describeExit("exited on its own", err)
			return
		case <-time.After(grace):
			hostReason = "lost its session socket and was killed by the host after " + grace.String()
		}
	}
	killProcessGroup(w.cmd.Process.Pid)
	_ = w.cmd.Process.Kill()
	w.exitCause = w.describeExit(hostReason, <-done)
}

// describeExit renders the reaped process's status and the tail of its
// crash diagnostics.
func (w *DeskSessionWorker) describeExit(reason string, waitErr error) string {
	status := "exit status unknown"
	if w.cmd != nil && w.cmd.ProcessState != nil {
		status = w.cmd.ProcessState.String()
	} else if waitErr != nil {
		status = waitErr.Error()
	}
	out := reason + " (" + status + ")"
	if w.stderr != nil {
		tail := strings.TrimSpace(w.stderr.String())
		if len(tail) > stderrTailBytes {
			tail = "…" + tail[len(tail)-stderrTailBytes:]
		}
		if tail != "" {
			out += "; stderr: " + tail
		}
	}
	return out
}

// ExitCause reports how the worker process ended: who ended it (the host's
// kill, a graceful close, or the worker on its own), the process status,
// and the tail of its stderr. Empty while the process has not been reaped.
func (w *DeskSessionWorker) ExitCause() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.exitCause
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
	w.reapLocked("closed by the host", 0)
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

// removeCellTmpDir removes a worker's private tmp dir once its process is
// gone. Only directories the spawner created (choir-cell-*) are removed.
func removeCellTmpDir(dir string) {
	if dir == "" || !strings.Contains(dir, "choir-cell-") {
		return
	}
	_ = os.RemoveAll(dir)
}

// lockedBuffer is the worker's stdout or stderr sink. exec.Cmd copies the
// child's output into it from its own goroutine while the process runs, so
// a reader before the reap (the spawn error path, a test) must not race the
// copy.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
