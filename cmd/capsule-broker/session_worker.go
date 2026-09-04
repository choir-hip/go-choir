//go:build linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// workerSessionConfig carries everything a session worker needs: identity for
// the framed loop, fencing for handle verification, and the package surface.
type workerSessionConfig struct {
	computerID  string
	epoch       uint64
	activation  string
	allowedRoot string
	allowed     []string
	timeout     time.Duration
	role        string
}

// sessionWorker owns one persistent worker process serving framed eval cells
// for a single activation. At most one in-flight Eval runs at a time; a
// timeout, poisoned cell, or transport error kills the whole process group.
// Transport is the multiplexed Unix domain socketpair (Step 2): the broker
// keeps one end, the worker inherits the other as fd 3. Raw stdio piping is
// gone; worker stdin stays closed and stdout carries only crash diagnostics.
type sessionWorker struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	framed *yaegikernel.FramedConn
	pid    int
	dead   bool
	config workerSessionConfig
}

var sessionFrameID uint64
var sessionFrameMu sync.Mutex

func nextSessionFrameID() string {
	sessionFrameMu.Lock()
	defer sessionFrameMu.Unlock()
	sessionFrameID++
	return fmt.Sprintf("cell-%d", sessionFrameID)
}

// sessionReadyTimeout bounds the worker's post-prebind ready handshake. A
// process that starts but never proves readiness (missing binary entry,
// rejected identity, failed prebind) is killed, never trusted.
const sessionReadyTimeout = 10 * time.Second

// spawnSessionWorker starts a session worker process in its own process group
// with a sanitized environment, mirroring the one-shot worker hardening. The
// role rides a trusted flag from the verified outer capability and bounds the
func spawnSessionWorker(bin string, cfg workerSessionConfig) (*sessionWorker, error) {
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		return nil, fmt.Errorf("session socketpair: %w", err)
	}
	parentFile := os.NewFile(uintptr(fds[0]), "session-parent")
	childFile := os.NewFile(uintptr(fds[1]), "session-child")
	parentConn, err := net.FileConn(parentFile)
	_ = parentFile.Close()
	if err != nil {
		_ = childFile.Close()
		_ = unix.Close(fds[0])
		return nil, fmt.Errorf("session parent socket: %w", err)
	}
	args := []string{
		"--isolation-stage", "exec-go-session",
		"--session-computer-id", cfg.computerID,
		"--session-activation", cfg.activation,
		"--session-allowed-root", cfg.allowedRoot,
		"--session-epoch", fmt.Sprintf("%d", cfg.epoch),
		"--session-role", cfg.role,
		"--session-sock-fd", "3",
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = cfg.allowedRoot
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	cmd.Env = []string{"PATH=/run/current-system/sw/bin:/bin:/usr/bin", "TMPDIR=/tmp"}
	cmd.ExtraFiles = []*os.File{childFile}
	cmd.Stderr = &bytes.Buffer{}
	if err := cmd.Start(); err != nil {
		_ = childFile.Close()
		_ = parentConn.Close()
		return nil, fmt.Errorf("start session worker: %w", err)
	}
	_ = childFile.Close()
	w := &sessionWorker{
		cmd:    cmd,
		framed: yaegikernel.NewFramedConn(parentConn),
		pid:    cmd.Process.Pid,
		config: cfg,
	}
	if err := w.awaitReady(sessionReadyTimeout); err != nil {
		w.killLocked()
		return nil, err
	}
	return w, nil
}

// awaitReady consumes the worker's post-prebind ready result frame from the
// session socket, so a process that starts but never proves readiness is
// killed, never trusted.
func (w *sessionWorker) awaitReady(timeout time.Duration) error {
	type readyFrame struct {
		ID string `json:"id"`
	}
	type result struct {
		stream  byte
		payload []byte
		err     error
	}
	ch := make(chan result, 1)
	go func() {
		stream, payload, err := w.framed.ReadFrame()
		ch <- result{stream: stream, payload: payload, err: err}
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case r := <-ch:
		if r.err != nil {
			return fmt.Errorf("session worker ready: %w", r.err)
		}
		if r.stream != yaegikernel.StreamCell {
			return fmt.Errorf("session worker ready: unexpected stream %d", r.stream)
		}
		var f readyFrame
		if err := json.Unmarshal(r.payload, &f); err != nil {
			return fmt.Errorf("session worker ready: %w", err)
		}
		if f.ID != "ready" {
			return fmt.Errorf("session worker ready: unexpected frame %q", f.ID)
		}
		return nil
	case <-timer.C:
		return fmt.Errorf("session worker ready: handshake timed out")
	}
}

// eval sends one cell over the session socket and waits for its result. The
// inbox snapshot rides the frame for cell-start injection. A timeout kills
// the worker; a poisoned cell (worker exits) is surfaced so the caller
// respawns. Reserved output streams are tolerated and skipped.
func (w *sessionWorker) eval(source string, inbox []yaegikernel.IncomingMessage, timeout time.Duration) (yaegikernel.SessionResult, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.dead {
		return yaegikernel.SessionResult{}, fmt.Errorf("session worker dead, respawn required")
	}
	frame, err := json.Marshal(yaegikernel.SessionFrame{ID: nextSessionFrameID(), Source: source, Inbox: inbox})
	if err != nil {
		return yaegikernel.SessionResult{}, err
	}
	if err := w.framed.WriteFrame(yaegikernel.StreamCell, frame); err != nil {
		w.killLocked()
		return yaegikernel.SessionResult{}, fmt.Errorf("session worker write: %w", err)
	}
	type result struct {
		res yaegikernel.SessionResult
		err error
	}
	ch := make(chan result, 1)
	go func() {
		for {
			stream, payload, rerr := w.framed.ReadFrame()
			if rerr != nil {
				ch <- result{err: fmt.Errorf("session worker read: %w", rerr)}
				return
			}
			if stream != yaegikernel.StreamCell {
				continue
			}
			var res yaegikernel.SessionResult
			if uerr := json.Unmarshal(payload, &res); uerr != nil {
				ch <- result{err: fmt.Errorf("session worker decode: %w", uerr)}
				return
			}
			ch <- result{res: res}
			return
		}
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case r := <-ch:
		if r.err != nil {
			w.killLocked()
			return yaegikernel.SessionResult{}, r.err
		}
		return r.res, nil
	case <-timer.C:
		w.killLocked()
		return yaegikernel.SessionResult{}, fmt.Errorf("session worker eval timed out")
	}
}

// close ends the session cleanly, then ensures the process group is dead.
func (w *sessionWorker) close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.dead {
		return
	}
	if frame, err := json.Marshal(yaegikernel.SessionFrame{ID: "close", Close: true}); err == nil {
		_ = w.framed.WriteFrame(yaegikernel.StreamCell, frame)
	}
	w.killLocked()
}

// sessionKillReapGrace bounds process-group reaping after SIGKILL (Def 2
// containment: kill-and-reaped within 500ms).
const sessionKillReapGrace = 500 * time.Millisecond

// killProcessGroup SIGKILLs the process group, kills the leader, and waits
// for reaping bounded by grace. It reports whether the group reaped in time;
// a late reap is logged by the caller, never fatal (init reaps orphans).
func killProcessGroup(cmd *exec.Cmd, grace time.Duration) bool {
	if cmd == nil || cmd.Process == nil {
		return true
	}
	pid := cmd.Process.Pid
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	_ = cmd.Process.Kill()
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	select {
	case <-done:
		return true
	case <-time.After(grace):
		log.Printf("capsule-broker: pid %d not reaped after SIGKILL", pid)
		return false
	}
}

func (w *sessionWorker) killLocked() {
	if w.dead {
		return
	}
	w.dead = true
	if w.cmd != nil && w.cmd.Process != nil {
		w.pid = w.cmd.Process.Pid
		killProcessGroup(w.cmd, sessionKillReapGrace)
	}
	if w.framed != nil {
		_ = w.framed.Close()
		w.framed = nil
	}
}

// sessionConfigFor builds the worker config from broker state. The worker is
// fenced by its own allowed root (the broker merged dir); the computer
// identity is the broker's capsule (never empty: an empty id would make the
// worker reject itself at startup). The role comes from the verified outer
// capability and bounds the prebound choir surface. The allowed set is the
// server-owned stdlib surface plus the prebound choir package: never
// model-controlled.
func (b *Broker) sessionConfigFor(activationID, role string) workerSessionConfig {
	allowed := yaegikernel.DefaultSafeStdlibPackagesList()
	computerID := b.capsuleID
	if computerID == "" {
		computerID = "worker-local"
	}
	return workerSessionConfig{
		computerID:  computerID,
		epoch:       1,
		activation:  activationID,
		allowedRoot: b.mergedDir,
		allowed:     allowed,
		timeout:     60 * time.Second,
		role:        role,
	}
}

// sessionFor returns the live worker for an activation, spawning it on first
// use. A dead worker is replaced, never resurrected: post-poison state is
// never trusted.
func (b *Broker) sessionFor(agentRunID, role string) (*sessionWorker, error) {
	if b == nil {
		return nil, fmt.Errorf("session worker: broker unavailable")
	}
	b.sessionMu.Lock()
	defer b.sessionMu.Unlock()
	if w, ok := b.sessionWorkers[agentRunID]; ok && w != nil && !w.dead {
		return w, nil
	}
	if b.brokerBin == "" {
		return nil, fmt.Errorf("session worker: broker binary path unavailable")
	}
	w, err := spawnSessionWorker(b.brokerBin, b.sessionConfigFor(agentRunID, role))
	if err != nil {
		return nil, err
	}
	b.sessionWorkers[agentRunID] = w
	return w, nil
}

// dropSession kills and forgets an activation worker. It is idempotent.
func (b *Broker) dropSession(agentRunID string) {
	if b == nil {
		return
	}
	b.sessionMu.Lock()
	defer b.sessionMu.Unlock()
	if w, ok := b.sessionWorkers[agentRunID]; ok {
		delete(b.sessionWorkers, agentRunID)
		if w != nil {
			w.close()
		}
	}
}

// handleInitSession creates (or recreates) the session worker for the calling
// activation. It is capability-gated like every verb by handleRPC.
func (b *Broker) handleInitSession(_ context.Context, cap *capsule.Capability, _ json.RawMessage) BrokerRPCResponse {
	if b == nil || cap.AgentRunID == "" {
		return BrokerRPCResponse{Error: "init_session: activation required"}
	}
	b.dropSession(cap.AgentRunID)
	w, err := b.sessionFor(cap.AgentRunID, string(cap.AgentRole))
	if err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("init_session: %v", err)}
	}
	raw, err := json.Marshal(map[string]any{"route": string(b.effectiveRoute()), "session": true, "pid": w.pid})
	if err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("init_session response: %v", err)}
	}
	return BrokerRPCResponse{Result: raw}
}

// handleCloseSession destroys the calling activation's worker.
func (b *Broker) handleCloseSession(_ context.Context, cap *capsule.Capability, _ json.RawMessage) BrokerRPCResponse {
	if b == nil || cap.AgentRunID == "" {
		return BrokerRPCResponse{Error: "close_session: activation required"}
	}
	b.dropSession(cap.AgentRunID)
	raw, err := json.Marshal(map[string]any{"session": false})
	if err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("close_session response: %v", err)}
	}
	return BrokerRPCResponse{Result: raw}
}

// handleGoEvalSession evaluates one cell on the activation's persistent
// worker. A dead or poisoned worker is dropped and reported (never silently
// retried: the cell may have partially executed). When no session worker can
// start, the cell falls back to the one-shot tools worker and the result is
// marked Fallback: the Def 2 fallback is per-call behavior with a receipt,
// not a flag. The cell deadline never exceeds the parent RPC deadline: a
// model-supplied TimeoutMS cannot extend the activation budget.
func (b *Broker) handleGoEvalSession(ctx context.Context, cap *capsule.Capability, params json.RawMessage) BrokerRPCResponse {
	var p capsule.GoEvalRequest
	if err := json.Unmarshal(params, &p); err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("failed to parse go_eval params: %v", err)}
	}
	timeout := 60 * time.Second
	if p.TimeoutMS > 0 {
		timeout = time.Duration(p.TimeoutMS) * time.Millisecond
	}
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining < timeout {
			timeout = remaining
		}
	}
	if timeout <= 0 {
		return BrokerRPCResponse{Error: "go_eval: parent deadline already exceeded"}
	}
	start := time.Now()
	w, err := b.sessionFor(cap.AgentRunID, string(cap.AgentRole))
	if err != nil {
		return b.fallbackGoEval(ctx, cap, params, fmt.Sprintf("session worker unavailable: %v", err))
	}
	res, err := w.eval(p.Source, p.Inbox, timeout)
	if err != nil {
		b.dropSession(cap.AgentRunID)
		result := capsule.GoEvalResult{ExitCode: 1, Error: fmt.Sprintf("session eval: %v", err), Duration: time.Since(start)}
		resultBytes, _ := json.Marshal(result)
		return BrokerRPCResponse{Result: resultBytes}
	}
	result := capsule.GoEvalResult{
		Stdout:   res.Stdout,
		Stderr:   res.Stderr,
		Error:    res.Error,
		Duration: time.Since(start),
		Receipts: res.Receipts,
	}
	if res.Error != "" {
		// Poisoned cells drop their tray: no intents ship, the inbox cursor
		// cannot advance, and the worker is discarded, never reused.
		result.ExitCode = 1
		b.dropSession(cap.AgentRunID)
	} else {
		result.Intents = res.Intents
	}
	resultBytes, _ := json.Marshal(result)
	return BrokerRPCResponse{Result: resultBytes}
}

// fallbackGoEval runs the cell on the one-shot tools worker after a session
// spawn failure and marks the result Fallback with the session error that
// caused it. When the one-shot path also fails, both errors are reported so
// the fallback attempt is visible, never silent.
func (b *Broker) fallbackGoEval(ctx context.Context, cap *capsule.Capability, params json.RawMessage, sessionErr string) BrokerRPCResponse {
	resp := b.handleGoEvalOneShot(ctx, cap, params)
	if resp.Error != "" {
		return BrokerRPCResponse{Error: fmt.Sprintf("%s; tools fallback also failed: %v", sessionErr, resp.Error)}
	}
	var result capsule.GoEvalResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("%s; tools fallback result undecodable: %v", sessionErr, err)}
	}
	result.Fallback = true
	result.Stderr += "\n[rlm fallback: " + sessionErr + "]"
	resultBytes, _ := json.Marshal(result)
	return BrokerRPCResponse{Result: resultBytes}
}
