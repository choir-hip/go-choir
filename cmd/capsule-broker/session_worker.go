//go:build linux

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

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
}

// sessionWorker owns one persistent worker process serving framed eval cells
// for a single activation. At most one in-flight Eval runs at a time; a
// timeout, poisoned cell, or transport error kills the whole process group
// and marks the worker dead so the next call respawns clean.
type sessionWorker struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	stdin  *os.File
	dec    *json.Decoder
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

// spawnSessionWorker starts a session worker process in its own process group
// with a sanitized environment, mirroring the one-shot worker hardening.
func spawnSessionWorker(bin string, cfg workerSessionConfig) (*sessionWorker, error) {
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	args := []string{
		"--isolation-stage", "exec-go-session",
		"--session-computer-id", cfg.computerID,
		"--session-activation", cfg.activation,
		"--session-allowed-root", cfg.allowedRoot,
		"--session-epoch", fmt.Sprintf("%d", cfg.epoch),
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = cfg.allowedRoot
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	cmd.Env = []string{"PATH=/run/current-system/sw/bin:/bin:/usr/bin", "TMPDIR=/tmp"}
	cmd.Stdin = stdinR
	cmd.Stdout = stdoutW
	cmd.Stderr = &bytes.Buffer{}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start session worker: %w", err)
	}
	_ = stdinR.Close()
	_ = stdoutW.Close()
	return &sessionWorker{
		cmd:    cmd,
		stdin:  stdinW,
		dec:    json.NewDecoder(bufio.NewReader(stdoutR)),
		pid:    cmd.Process.Pid,
		config: cfg,
	}, nil
}

// eval sends one cell and waits for its result. A timeout kills the worker;
// a poisoned cell (worker exits) is surfaced so the caller respawns.
func (w *sessionWorker) eval(source string, timeout time.Duration) (yaegikernel.SessionResult, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.dead {
		return yaegikernel.SessionResult{}, fmt.Errorf("session worker dead, respawn required")
	}
	frame, err := json.Marshal(yaegikernel.SessionFrame{ID: nextSessionFrameID(), Source: source})
	if err != nil {
		return yaegikernel.SessionResult{}, err
	}
	if _, err := w.stdin.Write(append(frame, '\n')); err != nil {
		w.killLocked()
		return yaegikernel.SessionResult{}, fmt.Errorf("session worker write: %w", err)
	}
	type result struct {
		res yaegikernel.SessionResult
		err error
	}
	ch := make(chan result, 1)
	go func() {
		var res yaegikernel.SessionResult
		if derr := w.dec.Decode(&res); derr != nil {
			ch <- result{err: fmt.Errorf("session worker read: %w", derr)}
			return
		}
		ch <- result{res: res}
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
		_, _ = w.stdin.Write(append(frame, '\n'))
	}
	w.killLocked()
}

func (w *sessionWorker) killLocked() {
	if w.dead {
		return
	}
	w.dead = true
	if w.cmd != nil && w.cmd.Process != nil {
		_ = syscall.Kill(-w.cmd.Process.Pid, syscall.SIGKILL)
		_ = w.cmd.Process.Kill()
		done := make(chan struct{})
		go func() { _ = w.cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			log.Printf("capsule-broker: session worker pid %d not reaped after SIGKILL", w.pid)
		}
	}
}

// sessionConfigFor builds the worker config from broker state. The worker is
// fenced by its own allowed root (the broker merged dir); computer and epoch
// ride the worker flags for handle verification fenced to this session. The
// allowed set is the server-owned stdlib surface plus the prebound choir
// package: never model-controlled.
func (b *Broker) sessionConfigFor(activationID string) workerSessionConfig {
	allowed := yaegikernel.DefaultSafeStdlibPackagesList()
	return workerSessionConfig{
		computerID:  "",
		epoch:       1,
		activation:  activationID,
		allowedRoot: b.mergedDir,
		allowed:     allowed,
		timeout:     60 * time.Second,
	}
}

// sessionFor returns the live worker for an activation, spawning it on first
// use. A dead worker is replaced, never resurrected: post-poison state is
// never trusted.
func (b *Broker) sessionFor(agentRunID string) (*sessionWorker, error) {
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
	w, err := spawnSessionWorker(b.brokerBin, b.sessionConfigFor(agentRunID))
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
	w, err := b.sessionFor(cap.AgentRunID)
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
// retried: the cell may have partially executed).
func (b *Broker) handleGoEvalSession(ctx context.Context, cap *capsule.Capability, params json.RawMessage) BrokerRPCResponse {
	var p capsule.GoEvalRequest
	if err := json.Unmarshal(params, &p); err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("failed to parse go_eval params: %v", err)}
	}
	timeout := 60 * time.Second
	if p.TimeoutMS > 0 {
		timeout = time.Duration(p.TimeoutMS) * time.Millisecond
	}
	_ = ctx
	start := time.Now()
	w, err := b.sessionFor(cap.AgentRunID)
	if err != nil {
		return BrokerRPCResponse{Error: fmt.Sprintf("session worker unavailable: %v", err)}
	}
	res, err := w.eval(p.Source, timeout)
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
	}
	if res.Error != "" {
		result.ExitCode = 1
		b.dropSession(cap.AgentRunID)
	}
	resultBytes, _ := json.Marshal(result)
	return BrokerRPCResponse{Result: resultBytes}
}
