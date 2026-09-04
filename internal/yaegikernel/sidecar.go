package yaegikernel

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// SidecarConfig configures the process-per-activation runner.
type SidecarConfig struct {
	Timeout          time.Duration `json:"timeout"`
	MaxOutputBytes   int64         `json:"max_output_bytes"`
	AllowedPackages  []string      `json:"allowed_packages"`
	WorkerBinaryPath string        `json:"worker_binary_path,omitempty"`
}

// SidecarRequest is the JSON payload sent to a standalone activation worker.
type SidecarRequest struct {
	Source          string   `json:"source"`
	AllowedPackages []string `json:"allowed_packages"`
}

// SidecarResponse is the JSON response from an activation worker.
type SidecarResponse struct {
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

// SidecarRunner executes model-authored Go code inside a separate OS process
// with strict timeout and process-group SIGKILL termination on exit.
type SidecarRunner struct {
	cfg SidecarConfig
}

// NewSidecarRunner creates a new process-per-activation runner.
func NewSidecarRunner(cfg SidecarConfig) *SidecarRunner {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	if cfg.MaxOutputBytes <= 0 {
		cfg.MaxOutputBytes = 2 * 1024 * 1024 // 2 MiB default output limit
	}
	if len(cfg.AllowedPackages) == 0 {
		for p := range DefaultSafeStdlibPackages {
			cfg.AllowedPackages = append(cfg.AllowedPackages, p)
		}
	}
	return &SidecarRunner{cfg: cfg}
}

// RunInProcess evaluates code in the current process using an isolated Evaluator.
func (r *SidecarRunner) RunInProcess(ctx context.Context, src string) (EvalResult, error) {
	evalCtx, cancel := context.WithTimeout(ctx, r.cfg.Timeout)
	defer cancel()

	allowlist := NewAllowlist(r.cfg.AllowedPackages...)
	evaluator := NewEvaluator(allowlist, nil)
	return evaluator.Eval(evalCtx, src)
}

// RunSubprocess executes code in a dedicated child process with process-group kill on timeout.
func (r *SidecarRunner) RunSubprocess(ctx context.Context, src string) (SidecarResponse, error) {
	evalCtx, cancel := context.WithTimeout(ctx, r.cfg.Timeout)
	defer cancel()

	start := time.Now()
	req := SidecarRequest{
		Source:          src,
		AllowedPackages: r.cfg.AllowedPackages,
	}
	reqData, err := json.Marshal(req)
	if err != nil {
		return SidecarResponse{}, fmt.Errorf("sidecar: marshal request: %w", err)
	}

	// Process-per-activation isolation is the contract. Without a configured
	// worker binary there is no process boundary, so fail closed rather than
	// silently falling back to an in-process interpreter that cannot be
	// process-group killed on timeout.
	bin := r.cfg.WorkerBinaryPath
	if bin == "" {
		return SidecarResponse{}, fmt.Errorf("sidecar: WorkerBinaryPath is required for subprocess isolation (fail closed)")
	}
	cmd := exec.CommandContext(evalCtx, bin)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdin = bytes.NewReader(reqData)
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // New process group for clean kill

	err = cmd.Start()
	if err != nil {
		return SidecarResponse{}, fmt.Errorf("sidecar: start worker: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-evalCtx.Done():
		// Kill the entire process group with SIGKILL
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			_ = cmd.Process.Kill()
		}
		return SidecarResponse{
			Stdout:     stdoutBuf.String(),
			Stderr:     stderrBuf.String(),
			Error:      "activation execution timed out",
			DurationMs: time.Since(start).Milliseconds(),
		}, evalCtx.Err()

	case waitErr := <-done:
		resp := SidecarResponse{
			Stdout:     stdoutBuf.String(),
			Stderr:     stderrBuf.String(),
			DurationMs: time.Since(start).Milliseconds(),
		}
		if waitErr != nil {
			resp.Error = waitErr.Error()
			return resp, waitErr
		}
		return resp, nil
	}
}

// ExecuteWorkerStdin handles stdin/stdout for a standalone activation worker binary.
func ExecuteWorkerStdin() {
	var req SidecarRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		resp := SidecarResponse{Error: fmt.Sprintf("decode request: %v", err)}
		_ = json.NewEncoder(os.Stdout).Encode(resp)
		os.Exit(1)
	}

	allowlist := NewAllowlist(req.AllowedPackages...)
	evaluator := NewEvaluator(allowlist, nil)
	res, err := evaluator.Eval(context.Background(), req.Source)

	resp := SidecarResponse{
		Stdout: res.Stdout,
		Stderr: res.Stderr,
	}
	if err != nil {
		resp.Error = err.Error()
	}
	_ = json.NewEncoder(os.Stdout).Encode(resp)
	if err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

// SessionWorkerConfig carries the per-activation identity a session worker
// needs before serving frames: package allowlist, computer/epoch fencing for
// handle verification, and the activation the choir scope reports.
type SessionWorkerConfig struct {
	AllowedPackages []string
	ComputerID      string
	ActivationID    string
	Epoch           uint64
	AllowedRoot     string
}

// ExecuteWorkerSessionStdin serves framed eval cells on stdin/stdout with one
// persistent Session (see RunSessionLoop). It exits zero on a close frame or
// EOF and non-zero when a cell poisons the session, so the broker respawns a
// clean worker. The process group boundary (set by the spawner) provides
// SIGKILL containment exactly like the one-shot worker.
func ExecuteWorkerSessionStdin(cfg SessionWorkerConfig) {
	allowlist := NewAllowlist(cfg.AllowedPackages...)
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		os.Exit(2)
	}
	issuer, err := NewHandleIssuer(secret)
	if err != nil {
		os.Exit(2)
	}
	root := cfg.AllowedRoot
	if root == "" {
		root = "/tmp"
	}
	broker, err := NewBroker(BrokerConfig{ComputerID: cfg.ComputerID, CurrentEpoch: cfg.Epoch, AllowedRoot: root}, issuer)
	if err != nil {
		os.Exit(2)
	}
	computerID := cfg.ComputerID
	if computerID == "" {
		computerID = "worker-local"
	}
	scope, err := NewChoirScope(broker, issuer, computerID, cfg.ActivationID, cfg.Epoch)
	if err != nil {
		os.Exit(2)
	}
	if err := RunSessionLoop(os.Stdin, os.Stdout, func() (*Session, error) {
		return NewSession(allowlist, scope.ChoirExports())
	}); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
