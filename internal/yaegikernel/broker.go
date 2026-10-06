package yaegikernel

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// EmitHandler performs the host-side durable write for ActionEmit.
type EmitHandler func(context.Context, EmitPayload) (EmitResult, error)

// BrokerConfig configures the trusted guest broker.
type BrokerConfig struct {
	ComputerID     string
	CurrentEpoch   uint64
	AllowedRoot    string
	MaxOutputBytes int64
	DefaultTimeout time.Duration
	// EmitRefused is the rollback gate. It refuses new emissions but never
	// affects records already durably delivered by the host.
	EmitRefused bool
	// Emit performs the host-side durable channel write and recipient wake.
	// A nil handler refuses emits rather than retaining an in-worker receipt.
	Emit EmitHandler
	// Egress is the generic host-mediated research boundary (research phase 1):
	// the worker binds it to a StreamBrokerEgress frame round-trip so the host
	// resolves any egress action (web_search/fetch_url/source_search/import_*/
	// read_*/list_*/search_wire_corpus) against its deps under the shared
	// activation egress budget. nil refuses the verb.
	Egress func(ctx context.Context, action BrokerAction, payload json.RawMessage) (json.RawMessage, error)
}

// Broker executes authorized operations requested by untrusted Yaegi activations.
type Broker struct {
	mu          sync.Mutex
	cfg         BrokerConfig
	handles     *HandleIssuer
	assignments map[string]AssignPayload
	messages    map[string]MessagePayload
}

// NewBroker creates a new trusted broker instance.
func NewBroker(cfg BrokerConfig, handles *HandleIssuer) (*Broker, error) {
	if strings.TrimSpace(cfg.ComputerID) == "" {
		return nil, fmt.Errorf("broker: computer_id is required")
	}
	if cfg.CurrentEpoch == 0 {
		cfg.CurrentEpoch = 1
	}
	if cfg.MaxOutputBytes <= 0 {
		cfg.MaxOutputBytes = 2 * 1024 * 1024 // 2 MiB
	}
	if cfg.DefaultTimeout <= 0 {
		cfg.DefaultTimeout = 30 * time.Second
	}
	if cfg.AllowedRoot == "" {
		cfg.AllowedRoot = "/tmp"
	}
	if handles == nil {
		return nil, fmt.Errorf("broker: handle issuer is required")
	}

	return &Broker{
		cfg:         cfg,
		handles:     handles,
		assignments: make(map[string]AssignPayload),
		messages:    make(map[string]MessagePayload),
	}, nil
}

// SetEpoch updates the current activation epoch for fencing.
func (b *Broker) SetEpoch(epoch uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cfg.CurrentEpoch = epoch
}

// SetEmit installs the durable-write handler for ActionEmit. The session
// socket is only available after the worker's framed conn exists, so the
// handler is bound post-construction rather than in BrokerConfig.
func (b *Broker) SetEmit(h EmitHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cfg.Emit = h
}

// SetEgress installs the generic host-mediated research boundary. Bound
// post-construction for the same reason as Emit: the session socket only
// exists after the worker's framed conn is up.
func (b *Broker) SetEgress(h func(context.Context, BrokerAction, json.RawMessage) (json.RawMessage, error)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cfg.Egress = h
}

// SetEmitRefused toggles the rollback gate for new emissions.
func (b *Broker) SetEmitRefused(refused bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cfg.EmitRefused = refused
}

// DrainReceipts returns worker-local assign/message entries recorded since
// the last drain and clears them, so the session loop surfaces each receipt
// to the host exactly once in SessionResult. Entries are host-reconciled,
// never trusted as durable; undrained entries die with the worker.
func (b *Broker) DrainReceipts() []string {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []string
	for id, p := range b.assignments {
		out = append(out, "assign/"+id+"/"+p.TaskID)
		delete(b.assignments, id)
	}
	for id, p := range b.messages {
		out = append(out, "message/"+id+"/"+p.Kind)
		delete(b.messages, id)
	}
	return out
}

// HandleRequest authenticates the handle and executes the requested action.
func (b *Broker) HandleRequest(ctx context.Context, req *BrokerRequest) *BrokerResponse {
	start := time.Now()
	if b == nil {
		return NewErrorResponse(req.RequestID, "broker uninitialized", time.Since(start))
	}
	if err := req.Validate(); err != nil {
		return NewErrorResponse(req.RequestID, err.Error(), time.Since(start))
	}

	b.mu.Lock()
	currentEpoch := b.cfg.CurrentEpoch
	b.mu.Unlock()

	// 1. Verify capability handle (signature, expiration, computer binding, epoch, and action scope)
	_, err := b.handles.Verify(req.HandleRef, b.cfg.ComputerID, currentEpoch, req.Action)
	if err != nil {
		return NewErrorResponse(req.RequestID, fmt.Sprintf("handle verification failed: %v", err), time.Since(start))
	}

	// 2. Dispatch action
	receiptID := newReceiptID()
	switch req.Action {
	case ActionExec:
		return b.handleExec(ctx, req, receiptID, start)
	case ActionReadFile:
		return b.handleReadFile(ctx, req, receiptID, start)
	case ActionWriteFile:
		return b.handleWriteFile(ctx, req, receiptID, start)
	case ActionListDir:
		return b.handleListDir(ctx, req, receiptID, start)
	case ActionAssign:
		return b.handleAssign(ctx, req, receiptID, start)
	case ActionMessage:
		return b.handleMessage(ctx, req, receiptID, start)
	case ActionEmit:
		return b.handleEmit(ctx, req, receiptID, start)
	case ActionWebSearch, ActionFetchURL, ActionSourceSearch, ActionImportDocument, ActionImportURL,
		ActionReadContentItem, ActionListContentSelectors, ActionReadContentSelector, ActionSearchWireCorpus,
		ActionSaveEvidence, ActionReadEvidence, ActionListEvidence, ActionGetRunMemoryEntry,
		ActionProductAPI:
		return b.handleEgress(ctx, req, receiptID, start)
	default:
		return NewErrorResponse(req.RequestID, fmt.Sprintf("unsupported action %q", req.Action), time.Since(start))
	}
}

func (b *Broker) handleExec(ctx context.Context, req *BrokerRequest, receiptID string, start time.Time) *BrokerResponse {
	var payload ExecPayload
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		return NewErrorResponse(req.RequestID, fmt.Sprintf("unmarshal exec payload: %v", err), time.Since(start))
	}

	timeout := b.cfg.DefaultTimeout
	if req.TimeoutMs > 0 {
		timeout = time.Duration(req.TimeoutMs) * time.Millisecond
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, payload.Command, payload.Args...)
	if payload.Dir != "" {
		cmd.Dir = payload.Dir
	} else {
		cmd.Dir = b.cfg.AllowedRoot
	}

	// Clean environment: inherit only safe baseline, never pass credentials
	cmd.Env = []string{
		"PATH=/run/current-system/sw/bin:/usr/bin:/bin",
		"HOME=/tmp",
		"TMPDIR=/tmp",
	}
	for k, v := range payload.Env {
		if !strings.HasPrefix(strings.ToUpper(k), "CHOIR_") && !strings.Contains(strings.ToUpper(k), "KEY") && !strings.Contains(strings.ToUpper(k), "TOKEN") {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	execStart := time.Now()
	err := cmd.Start()
	if err != nil {
		return NewErrorResponse(req.RequestID, fmt.Sprintf("exec start failed: %v", err), time.Since(start))
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	var waitErr error
	select {
	case <-execCtx.Done():
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			_ = cmd.Process.Kill()
		}
		waitErr = execCtx.Err()
	case waitErr = <-done:
	}

	exitCode := 0
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	result := ExecResult{
		ExitCode:   exitCode,
		Stdout:     truncateOutput(stdoutBuf.String(), b.cfg.MaxOutputBytes),
		Stderr:     truncateOutput(stderrBuf.String(), b.cfg.MaxOutputBytes),
		DurationMs: time.Since(execStart).Milliseconds(),
	}

	resp, err := NewSuccessResponse(req.RequestID, result, receiptID, time.Since(start))
	if err != nil {
		return NewErrorResponse(req.RequestID, err.Error(), time.Since(start))
	}
	return resp
}

func (b *Broker) handleReadFile(ctx context.Context, req *BrokerRequest, receiptID string, start time.Time) *BrokerResponse {
	var payload ReadFilePayload
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		return NewErrorResponse(req.RequestID, fmt.Sprintf("unmarshal read_file payload: %v", err), time.Since(start))
	}

	cleanPath, err := b.resolveSafePath(payload.Path)
	if err != nil {
		return NewErrorResponse(req.RequestID, err.Error(), time.Since(start))
	}

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return NewErrorResponse(req.RequestID, fmt.Sprintf("read file failed: %v", err), time.Since(start))
	}

	result := ReadFileResult{
		Content: string(data),
		Size:    int64(len(data)),
	}

	resp, err := NewSuccessResponse(req.RequestID, result, receiptID, time.Since(start))
	if err != nil {
		return NewErrorResponse(req.RequestID, err.Error(), time.Since(start))
	}
	return resp
}

func (b *Broker) handleWriteFile(ctx context.Context, req *BrokerRequest, receiptID string, start time.Time) *BrokerResponse {
	var payload WriteFilePayload
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		return NewErrorResponse(req.RequestID, fmt.Sprintf("unmarshal write_file payload: %v", err), time.Since(start))
	}

	cleanPath, err := b.resolveSafePath(payload.Path)
	if err != nil {
		return NewErrorResponse(req.RequestID, err.Error(), time.Since(start))
	}

	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o755); err != nil {
		return NewErrorResponse(req.RequestID, fmt.Sprintf("create parent dir: %v", err), time.Since(start))
	}

	mode := os.FileMode(0o644)
	if payload.Mode > 0 {
		mode = os.FileMode(payload.Mode)
	}

	if err := os.WriteFile(cleanPath, []byte(payload.Content), mode); err != nil {
		return NewErrorResponse(req.RequestID, fmt.Sprintf("write file failed: %v", err), time.Since(start))
	}

	result := WriteFileResult{
		BytesWritten: len(payload.Content),
		ModTime:      time.Now().Unix(),
	}

	resp, err := NewSuccessResponse(req.RequestID, result, receiptID, time.Since(start))
	if err != nil {
		return NewErrorResponse(req.RequestID, err.Error(), time.Since(start))
	}
	return resp
}
func (b *Broker) handleListDir(ctx context.Context, req *BrokerRequest, receiptID string, start time.Time) *BrokerResponse {
	var payload ListDirPayload
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		return NewErrorResponse(req.RequestID, fmt.Sprintf("unmarshal list_dir payload: %v", err), time.Since(start))
	}

	cleanPath, err := b.resolveSafePath(payload.Path)
	if err != nil {
		return NewErrorResponse(req.RequestID, err.Error(), time.Since(start))
	}

	entries, err := os.ReadDir(cleanPath)
	if err != nil {
		return NewErrorResponse(req.RequestID, fmt.Sprintf("list dir failed: %v", err), time.Since(start))
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	resp, err := NewSuccessResponse(req.RequestID, ListDirResult{Entries: names}, receiptID, time.Since(start))
	if err != nil {
		return NewErrorResponse(req.RequestID, err.Error(), time.Since(start))
	}
	return resp
}

func (b *Broker) handleAssign(ctx context.Context, req *BrokerRequest, receiptID string, start time.Time) *BrokerResponse {
	var payload AssignPayload
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		return NewErrorResponse(req.RequestID, fmt.Sprintf("unmarshal assign payload: %v", err), time.Since(start))
	}

	assignID := newReceiptID()
	b.mu.Lock()
	b.assignments[assignID] = payload
	b.mu.Unlock()

	result := AssignResult{
		AssignmentID: assignID,
		Status:       "dispatched",
	}

	resp, err := NewSuccessResponse(req.RequestID, result, receiptID, time.Since(start))
	if err != nil {
		return NewErrorResponse(req.RequestID, err.Error(), time.Since(start))
	}
	return resp
}

func (b *Broker) handleMessage(ctx context.Context, req *BrokerRequest, receiptID string, start time.Time) *BrokerResponse {
	var payload MessagePayload
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		return NewErrorResponse(req.RequestID, fmt.Sprintf("unmarshal message payload: %v", err), time.Since(start))
	}

	msgID := newReceiptID()
	b.mu.Lock()
	b.messages[msgID] = payload
	b.mu.Unlock()

	result := MessageResult{
		MessageID:   msgID,
		DeliveredAt: time.Now().UTC().Format(time.RFC3339),
	}

	resp, err := NewSuccessResponse(req.RequestID, result, receiptID, time.Since(start))
	if err != nil {
		return NewErrorResponse(req.RequestID, err.Error(), time.Since(start))
	}
	return resp
}

func (b *Broker) handleEmit(ctx context.Context, req *BrokerRequest, receiptID string, start time.Time) *BrokerResponse {
	var payload EmitPayload
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		return NewErrorResponse(req.RequestID, fmt.Sprintf("unmarshal emit payload: %v", err), time.Since(start))
	}
	if strings.TrimSpace(payload.ToDesk) == "" {
		return NewErrorResponse(req.RequestID, "emit requires a destination desk", time.Since(start))
	}
	if strings.TrimSpace(payload.Kind) == "" {
		return NewErrorResponse(req.RequestID, "emit requires a kind", time.Since(start))
	}
	if len(payload.Body) > MaxIntentBody {
		return NewErrorResponse(req.RequestID, fmt.Sprintf("emit body %d bytes exceeds egress budget %d", len(payload.Body), MaxIntentBody), time.Since(start))
	}
	b.mu.Lock()
	refused, emit := b.cfg.EmitRefused, b.cfg.Emit
	b.mu.Unlock()
	if refused {
		return NewErrorResponse(req.RequestID, "emit refused by gate", time.Since(start))
	}
	if emit == nil {
		return NewErrorResponse(req.RequestID, "emit host unavailable", time.Since(start))
	}
	result, err := emit(ctx, payload)
	if err != nil {
		return NewErrorResponse(req.RequestID, fmt.Sprintf("emit: %v", err), time.Since(start))
	}
	resp, err := NewSuccessResponse(req.RequestID, result, receiptID, time.Since(start))
	if err != nil {
		return NewErrorResponse(req.RequestID, err.Error(), time.Since(start))
	}
	return resp
}

// handleEgress resolves any host-mediated research verb through cfg.Egress.
// The raw payload passes through untouched — the host's deps-bound ToolFunc
// decodes and validates it — and the raw JSON result passes back to the cell.
func (b *Broker) handleEgress(ctx context.Context, req *BrokerRequest, receiptID string, start time.Time) *BrokerResponse {
	b.mu.Lock()
	h := b.cfg.Egress
	b.mu.Unlock()
	if h == nil {
		return NewErrorResponse(req.RequestID, fmt.Sprintf("%s host unavailable", req.Action), time.Since(start))
	}
	result, err := h(ctx, req.Action, req.Payload)
	if err != nil {
		return NewErrorResponse(req.RequestID, fmt.Sprintf("%s: %v", req.Action, err), time.Since(start))
	}
	resp, err := NewSuccessResponse(req.RequestID, result, receiptID, time.Since(start))
	if err != nil {
		return NewErrorResponse(req.RequestID, err.Error(), time.Since(start))
	}
	return resp
}

func (b *Broker) resolveSafePath(p string) (string, error) {
	clean := filepath.Clean(p)
	if !filepath.IsAbs(clean) {
		clean = filepath.Join(b.cfg.AllowedRoot, clean)
	}
	rel, err := filepath.Rel(b.cfg.AllowedRoot, clean)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path %q escapes allowed root %q", p, b.cfg.AllowedRoot)
	}
	return clean, nil
}

func truncateOutput(s string, maxBytes int64) string {
	if int64(len(s)) <= maxBytes {
		return s
	}
	return s[:maxBytes] + "\n...[output truncated by broker]"
}

func newReceiptID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "rcpt_" + hex.EncodeToString(b)
}
