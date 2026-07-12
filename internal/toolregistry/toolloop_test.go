package toolregistry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// --- Mock provideriface.ToolLoopProvider ---

// mockToolLoopProvider implements provideriface.ToolLoopProvider for testing the
// tool-calling loop. It simulates LLM responses with configurable
// stop reasons and tool calls.
type mockToolLoopProvider struct {
	// Provider is the base Provider interface (for ProviderName etc).
	provideriface.Provider

	// responses is a sequence of responses to return from CallWithTools.
	// Each response is consumed in order; if exhausted, the last response
	// is reused.
	responses []*provideriface.ToolLoopResponse

	// callCount tracks how many times CallWithTools was invoked.
	callCount int32

	lastReq provideriface.ToolLoopRequest
}

func (m *mockToolLoopProvider) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	m.lastReq = req
	idx := int(atomic.AddInt32(&m.callCount, 1)) - 1
	if idx >= len(m.responses) {
		idx = len(m.responses) - 1
	}
	if idx < 0 {
		return nil, fmt.Errorf("no responses configured")
	}
	return m.responses[idx], nil
}

func (m *mockToolLoopProvider) CallCount() int {
	return int(atomic.LoadInt32(&m.callCount))
}

// newMockToolLoopProvider creates a mock that returns the given responses in sequence.
func newMockToolLoopProvider(responses ...*provideriface.ToolLoopResponse) *mockToolLoopProvider {
	return &mockToolLoopProvider{
		responses: responses,
	}
}

type capturingToolChoiceProvider struct {
	provideriface.Provider
	responses []*provideriface.ToolLoopResponse
	choices   *[]string
	maxTokens *[]int
	requests  *[]provideriface.ToolLoopRequest
	callCount int
}

func (p *capturingToolChoiceProvider) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	if p.requests != nil {
		*p.requests = append(*p.requests, req)
	}
	if p.choices != nil {
		*p.choices = append(*p.choices, req.ToolChoice)
	}
	if p.maxTokens != nil {
		*p.maxTokens = append(*p.maxTokens, req.MaxTokens)
	}
	idx := p.callCount
	p.callCount++
	if idx >= len(p.responses) {
		idx = len(p.responses) - 1
	}
	if idx < 0 {
		return nil, fmt.Errorf("no responses configured")
	}
	return p.responses[idx], nil
}

type basicProvider struct{}

func (basicProvider) Execute(_ context.Context, task *types.RunRecord, _ provideriface.EventEmitFunc) error { task.Result = "basic result"; return nil }
func (basicProvider) ProviderName() string { return "basic" }

// --- Tool-Calling Loop Tests ---

func TestRunToolLoopEndTurn(t *testing.T) {
	// Simple case: LLM returns end_turn immediately.
	provider := newMockToolLoopProvider(
		&provideriface.ToolLoopResponse{
			StopReason: "end_turn",
			Text:       "Hello! How can I help?",
			Usage:      provideriface.TokenUsage{InputTokens: 10, OutputTokens: 20},
			Model:      "test-model",
		},
	)

	var emittedEvents []types.EventKind
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		emittedEvents = append(emittedEvents, kind)
	}

	text, usage, err := RunToolLoop(context.Background(), provider, nil, // no tool registry
		[]json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`)},
		"You are helpful.",
		4096,
		emit,
		nil,)

	if err != nil {
		t.Fatalf("run tool loop: %v", err)
	}
	if text != "Hello! How can I help?" {
		t.Errorf("text: got %q, want Hello! How can I help?", text)
	}
	if usage.InputTokens != 10 || usage.OutputTokens != 20 {
		t.Errorf("usage: got in=%d out=%d, want in=10 out=20", usage.InputTokens, usage.OutputTokens)
	}

	// Should have emitted a progress event for the iteration.
	found := false
	for _, k := range emittedEvents {
		if k == types.EventRunProgress {
			found = true
		}
	}
	if !found {
		t.Error("expected loop.progress event from loop iteration")
	}
}

func TestRunToolLoopTerminalToolSuccessStopsWithoutExtraProviderTurn(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.Register(Tool{
		Name: "patch_texture",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"status":"stored","revision_id":"rev-2"}`, nil
		},
	}); err != nil {
		t.Fatalf("register patch_texture: %v", err)
	}

	provider := newMockToolLoopProvider(
		&provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:        "call-edit",
				Name:      "patch_texture",
				Arguments: json.RawMessage(`{"doc_id":"doc-1"}`),
			}},
			Usage: provideriface.TokenUsage{InputTokens: 10, OutputTokens: 4},
			Model: "test-model",
		},
		&provideriface.ToolLoopResponse{
			StopReason: "end_turn",
			Text:       "should not be requested",
			Usage:      provideriface.TokenUsage{InputTokens: 10, OutputTokens: 4},
			Model:      "test-model",
		},
	)

	var terminalProgress bool
	text, _, err := RunToolLoop(context.Background(), provider, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":"revise"}`)},
		"You are Texture.",
		0,
		func(kind types.EventKind, phase string, payload json.RawMessage) {
			if kind == types.EventRunProgress && phase == "terminal_tool_success" {
				terminalProgress = true
			}
		},
		nil,
		WithTerminalToolSuccesses("patch_texture"),)
	if err != nil {
		t.Fatalf("run tool loop: %v", err)
	}
	if text != "" {
		t.Fatalf("text = %q, want empty terminal tool result", text)
	}
	if provider.CallCount() != 1 {
		t.Fatalf("provider calls = %d, want 1", provider.CallCount())
	}
	if !terminalProgress {
		t.Fatal("missing terminal_tool_success progress event")
	}
}

func TestRunToolLoopRequiredNextToolSatisfiedInSameBatchDoesNotRetry(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.Register(Tool{
		Name: "request_worker_vm",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"status":"worker_requested","delegation_required":true,"next_tool":"start_worker_delegation","start_args":{"worker_sandbox_url":"http://worker","worker_id":"worker-1","profile":"vsuper"},"next_instruction":"Call start_worker_delegation next with start_args plus the full execution objective."}`, nil
		},
	}); err != nil {
		t.Fatalf("register request_worker_vm: %v", err)
	}
	if err := registry.Register(Tool{
		Name: "start_worker_delegation",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"status":"worker_run_started","worker_run_id":"run-worker"}`, nil
		},
	}); err != nil {
		t.Fatalf("register start_worker_delegation: %v", err)
	}

	provider := newMockToolLoopProvider(
		&provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{
				{ID: "call-request", Name: "request_worker_vm", Arguments: json.RawMessage(`{}`)},
				{ID: "call-start", Name: "start_worker_delegation", Arguments: json.RawMessage(`{}`)},
			},
			Usage: provideriface.TokenUsage{InputTokens: 10, OutputTokens: 4},
			Model: "test-model",
		},
		&provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{
				{ID: "call-duplicate-start", Name: "start_worker_delegation", Arguments: json.RawMessage(`{}`)},
			},
			Usage: provideriface.TokenUsage{InputTokens: 10, OutputTokens: 4},
			Model: "test-model",
		},
	)

	_, _, err := RunToolLoop(context.Background(), provider, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":"start worker"}`)},
		"You are Super.",
		0,
		func(kind types.EventKind, phase string, payload json.RawMessage) {},
		nil,
		WithTerminalToolSuccesses("request_worker_vm", "start_worker_delegation"),)
	if err != nil {
		t.Fatalf("run tool loop: %v", err)
	}
	if provider.CallCount() != 1 {
		t.Fatalf("provider calls = %d, want 1", provider.CallCount())
	}
}

func TestRunToolLoopEmitsProviderCallProgressBeforeCall(t *testing.T) {
	provider := newMockToolLoopProvider(&provideriface.ToolLoopResponse{
		StopReason: "end_turn",
		Text:       "done",
		Usage:      provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
		Model:      "test-model",
	})

	var providerCallPayload map[string]any
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		if kind != types.EventRunProgress || phase != "provider_call" {
			return
		}
		if err := json.Unmarshal(payload, &providerCallPayload); err != nil {
			t.Fatalf("unmarshal provider_call payload: %v", err)
		}
	}

	_, _, err := RunToolLoop(context.Background(), provider, nil, []json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`)},
		"You are helpful.",
		0,
		emit,
		nil,
		WithToolLoopLLMConfig(provideriface.LLMSelection{Provider: "fireworks", Model: "accounts/fireworks/models/deepseek-v4-flash", ReasoningEffort: "none"}),)
	if err != nil {
		t.Fatalf("run tool loop: %v", err)
	}
	if providerCallPayload == nil {
		t.Fatal("missing provider_call progress event")
	}
	if got := providerCallPayload["phase"]; got != "provider_call_started" {
		t.Fatalf("phase = %v, want provider_call_started", got)
	}
	if got := providerCallPayload["max_tokens_requested"]; got != false {
		t.Fatalf("max_tokens_requested = %v, want false", got)
	}
	if got := providerCallPayload["llm_provider"]; got != "fireworks" {
		t.Fatalf("llm_provider = %v, want fireworks", got)
	}
	if got := providerCallPayload["last_user_text"]; got != "hi" {
		t.Fatalf("last_user_text = %v, want hi", got)
	}
	if got := providerCallPayload["system_sha256"]; got == "" {
		t.Fatalf("system_sha256 should be present")
	}
	if got := providerCallPayload["system_preview"]; !strings.Contains(fmt.Sprint(got), "You are helpful") {
		t.Fatalf("system_preview = %v, want prompt excerpt", got)
	}
	if roles, ok := providerCallPayload["message_roles"].([]any); !ok || len(roles) != 1 || roles[0] != "user" {
		t.Fatalf("message_roles = %#v, want [user]", providerCallPayload["message_roles"])
	}
}

func TestRunToolLoopEmitsResponseTextAndToolCallNames(t *testing.T) {
	provider := newMockToolLoopProvider(
		&provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			Text:       "I found the first route.",
			ToolCalls: []types.ToolCall{{
				ID:        "call-1",
				Name:      "echo",
				Arguments: json.RawMessage(`{"message":"hello"}`),
			}},
			Model: "test-model",
		},
		&provideriface.ToolLoopResponse{
			StopReason: "end_turn",
			Text:       "done",
			Model:      "test-model",
		},
	)
	registry := NewToolRegistry()
	if err := registry.Register(Tool{
		Name:        "echo",
		Description: "Echo a message.",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return string(args), nil
		},
	}); err != nil {
		t.Fatalf("register echo: %v", err)
	}

	var toolLoopPayload map[string]any
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		if kind != types.EventRunProgress || phase != "tool_loop" || toolLoopPayload != nil {
			return
		}
		if err := json.Unmarshal(payload, &toolLoopPayload); err != nil {
			t.Fatalf("unmarshal tool_loop payload: %v", err)
		}
	}

	_, _, err := RunToolLoop(context.Background(), provider, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`)},
		"You are helpful.",
		4096,
		emit,
		nil,)
	if err != nil {
		t.Fatalf("run tool loop: %v", err)
	}
	if toolLoopPayload == nil {
		t.Fatal("missing tool_loop progress event")
	}
	if got := toolLoopPayload["response_text"]; got != "I found the first route." {
		t.Fatalf("response_text = %v", got)
	}
	if got := int(toolLoopPayload["response_text_chars"].(float64)); got != len("I found the first route.") {
		t.Fatalf("response_text_chars = %d", got)
	}
	names, ok := toolLoopPayload["tool_call_names"].([]any)
	if !ok || len(names) != 1 || names[0] != "echo" {
		t.Fatalf("tool_call_names = %#v, want [echo]", toolLoopPayload["tool_call_names"])
	}
}

func TestRunToolLoopInitialToolChoiceAppliesOnlyFirstCall(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.Register(Tool{
		Name:        "record_status",
		Description: "Record status.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"status": map[string]any{"type": "string"}},
			"required":   []string{"status"},
		},
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"ok":true}`, nil
		},
	}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	var choices []string
	provider := &capturingToolChoiceProvider{responses: []*provideriface.ToolLoopResponse{
		{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:        "call-1",
				Name:      "record_status",
				Arguments: json.RawMessage(`{"status":"ok"}`),
			}},
			Usage: provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model: "test-model",
		},
		{
			StopReason: "end_turn",
			Text:       "done",
			Usage:      provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model:      "test-model",
		},
	}, choices: &choices}

	text, _, err := RunToolLoop(context.Background(), provider, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":"record"}`)},
		"You are helpful.",
		0,
		func(types.EventKind, string, json.RawMessage) {},
		nil,
		WithInitialToolChoice("required"),)
	if err != nil {
		t.Fatalf("run tool loop: %v", err)
	}
	if text != "done" {
		t.Fatalf("text = %q, want done", text)
	}
	if len(choices) != 2 {
		t.Fatalf("choices = %#v, want two provider calls", choices)
	}
	if choices[0] != "required" {
		t.Fatalf("first tool choice = %q, want required", choices[0])
	}
	if choices[1] != "" {
		t.Fatalf("second tool choice = %q, want empty", choices[1])
	}
}

func TestRunToolLoopCompletionGuardRetriesEndTurn(t *testing.T) {
	registry := NewToolRegistry()
	var recorded int
	if err := registry.Register(Tool{
		Name:        "record_status",
		Description: "Record status.",
		Parameters:  map[string]any{"type": "object"},
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			recorded++
			return `{"status":"recorded"}`, nil
		},
	}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	var requests []provideriface.ToolLoopRequest
	provider := &capturingToolChoiceProvider{responses: []*provideriface.ToolLoopResponse{
		{
			StopReason: "end_turn",
			Text:       "premature",
			Usage:      provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model:      "test-model",
		},
		{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:        "call-record",
				Name:      "record_status",
				Arguments: json.RawMessage(`{"status":"ok"}`),
			}},
			Usage: provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model: "test-model",
		},
		{
			StopReason: "end_turn",
			Text:       "done",
			Usage:      provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model:      "test-model",
		},
	}, requests: &requests}

	var retries int
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		if kind == types.EventRunRetry && phase == "completion_guard" {
			retries++
		}
	}
	guard := func(ctx context.Context, state ToolLoopCompletionState) (ToolLoopCompletionGuardResult, error) {
		if state.Attempts == 0 {
			return ToolLoopCompletionGuardResult{
				Continue:    true,
				Reason:      "status_missing",
				Instruction: "Record status before ending.",
			}, nil
		}
		return ToolLoopCompletionGuardResult{}, nil
	}

	text, _, err := RunToolLoop(context.Background(), provider, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":"record status"}`)},
		"You are helpful.",
		0,
		emit,
		nil,
		func(opts *toolLoopOptions) {
			opts.completionGuard = guard
		},)
	if err != nil {
		t.Fatalf("run tool loop: %v", err)
	}
	if text != "done" {
		t.Fatalf("text = %q, want final text after guarded retry", text)
	}
	if recorded != 1 {
		t.Fatalf("recorded = %d, want 1", recorded)
	}
	if retries != 1 {
		t.Fatalf("completion guard retries = %d, want 1", retries)
	}
	if len(requests) < 2 || !strings.Contains(extractLastUserMessage(requests[1].Messages), "Record status before ending.") {
		t.Fatalf("guard reminder missing from second request; requests=%#v", requests)
	}
}

func TestRunToolLoopExactInitialToolChoiceRejectsDifferentReturnedTool(t *testing.T) {
	registry := NewToolRegistry()
	var edited, recorded int
	if err := registry.Register(Tool{
		Name:        "patch_texture",
		Description: "Edit the Texture document.",
		Parameters:  map[string]any{"type": "object"},
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			edited++
			return `{"status":"edited"}`, nil
		},
	}); err != nil {
		t.Fatalf("register patch_texture: %v", err)
	}
	if err := registry.Register(Tool{
		Name:        "record_texture_decision",
		Description: "Record a Texture decision.",
		Parameters:  map[string]any{"type": "object"},
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			recorded++
			return `{"status":"recorded"}`, nil
		},
	}); err != nil {
		t.Fatalf("register record_texture_decision: %v", err)
	}

	var choices []string
	provider := &capturingToolChoiceProvider{responses: []*provideriface.ToolLoopResponse{
		{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:        "call-edit",
				Name:      "patch_texture",
				Arguments: json.RawMessage(`{"content":"private reason leaked"}`),
			}},
			Usage: provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model: "test-model",
		},
		{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:        "call-decision",
				Name:      "record_texture_decision",
				Arguments: json.RawMessage(`{"decision_kind":"no_worker_needed"}`),
			}},
			Usage: provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model: "test-model",
		},
		{
			StopReason: "end_turn",
			Text:       "done",
			Usage:      provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model:      "test-model",
		},
	}, choices: &choices}

	var retrySeen bool
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		if kind == types.EventRunRetry && phase == "initial_tool_choice" {
			retrySeen = true
			var decoded map[string]any
			if err := json.Unmarshal(payload, &decoded); err != nil {
				t.Fatalf("decode retry payload: %v", err)
			}
			if decoded["required_tool"] != "record_texture_decision" {
				t.Fatalf("required_tool = %v, want record_texture_decision", decoded["required_tool"])
			}
		}
	}

	text, _, err := RunToolLoop(context.Background(), provider, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":"record first"}`)},
		"You are helpful.",
		0,
		emit,
		nil,
		WithInitialToolChoice("function:record_texture_decision"),)
	if err != nil {
		t.Fatalf("run tool loop: %v", err)
	}
	if text != "done" {
		t.Fatalf("text = %q, want done", text)
	}
	if edited != 0 {
		t.Fatalf("patch_texture executed %d times, want 0", edited)
	}
	if recorded != 1 {
		t.Fatalf("record_texture_decision executed %d times, want 1", recorded)
	}
	if len(choices) != 3 || choices[0] != "function:record_texture_decision" || choices[1] != "function:record_texture_decision" || choices[2] != "" {
		t.Fatalf("tool choices = %#v, want exact retry then unconstrained final", choices)
	}
	if !retrySeen {
		t.Fatal("missing initial_tool_choice retry event")
	}
}

func TestRunToolLoopExactInitialToolChoiceRetriesEndTurnWithoutTool(t *testing.T) {
	registry := NewToolRegistry()
	var edited int
	if err := registry.Register(Tool{
		Name:        "patch_texture",
		Description: "Edit the Texture document.",
		Parameters:  map[string]any{"type": "object"},
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			edited++
			return `{"status":"edited"}`, nil
		},
	}); err != nil {
		t.Fatalf("register patch_texture: %v", err)
	}
	if err := registry.Register(Tool{
		Name:        "spawn_agent",
		Description: "Delegate work.",
		Parameters:  map[string]any{"type": "object"},
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"status":"spawned"}`, nil
		},
	}); err != nil {
		t.Fatalf("register spawn_agent: %v", err)
	}

	var choices []string
	var requests []provideriface.ToolLoopRequest
	provider := &capturingToolChoiceProvider{
		responses: []*provideriface.ToolLoopResponse{
			{
				StopReason: "end_turn",
				Text:       "I will write later.",
				Usage:      provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
				Model:      "test-model",
			},
			{
				StopReason: "tool_use",
				ToolCalls: []types.ToolCall{{
					ID:        "call-edit",
					Name:      "patch_texture",
					Arguments: json.RawMessage(`{"content":"fast scaffold"}`),
				}},
				Usage: provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
				Model: "test-model",
			},
			{
				StopReason: "end_turn",
				Text:       "done",
				Usage:      provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
				Model:      "test-model",
			},
		},
		choices:  &choices,
		requests: &requests,
	}

	var retrySeen bool
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		if kind != types.EventRunRetry || phase != "initial_tool_choice" {
			return
		}
		retrySeen = true
		var decoded map[string]any
		if err := json.Unmarshal(payload, &decoded); err != nil {
			t.Fatalf("decode retry payload: %v", err)
		}
		if decoded["required_tool"] != "patch_texture" || decoded["reason"] != "model_ended_turn_without_required_initial_tool" {
			t.Fatalf("retry payload = %+v", decoded)
		}
	}

	text, _, err := RunToolLoop(context.Background(), provider, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":"write first"}`)},
		"You are a Texture appagent.",
		0,
		emit,
		nil,
		WithInitialToolChoice("function:patch_texture"),)
	if err != nil {
		t.Fatalf("run tool loop: %v", err)
	}
	if text != "done" {
		t.Fatalf("text = %q, want done", text)
	}
	if edited != 1 {
		t.Fatalf("patch_texture executed %d times, want 1", edited)
	}
	if len(choices) != 3 || choices[0] != "function:patch_texture" || choices[1] != "function:patch_texture" || choices[2] != "" {
		t.Fatalf("tool choices = %#v, want exact retry then unconstrained final", choices)
	}
	if len(requests) < 2 {
		t.Fatalf("requests = %d, want at least 2", len(requests))
	}
	for i := 0; i < 2; i++ {
		if len(requests[i].ToolDefinitions) != 1 || requests[i].ToolDefinitions[0].Name != "patch_texture" {
			t.Fatalf("request %d tool definitions = %+v, want only patch_texture", i, requests[i].ToolDefinitions)
		}
	}
	if !retrySeen {
		t.Fatal("missing initial_tool_choice retry event")
	}
}

func TestRunToolLoopExactInitialToolChoiceRetriesFailedRequiredTool(t *testing.T) {
	registry := NewToolRegistry()
	var edited int
	if err := registry.Register(Tool{
		Name:        "patch_texture",
		Description: "Edit the Texture document.",
		Parameters:  map[string]any{"type": "object"},
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			edited++
			if strings.Contains(string(args), "bad-find") {
				return "", fmt.Errorf("edit 0: find text not present")
			}
			return `{"status":"stored","revision_id":"rev-2"}`, nil
		},
	}); err != nil {
		t.Fatalf("register patch_texture: %v", err)
	}

	var choices []string
	var requests []provideriface.ToolLoopRequest
	provider := &capturingToolChoiceProvider{
		responses: []*provideriface.ToolLoopResponse{
			{
				StopReason: "tool_use",
				ToolCalls: []types.ToolCall{
					{ID: "call-bad-1", Name: "patch_texture", Arguments: json.RawMessage(`{"doc_id":"doc-1","find":"bad-find"}`)},
					{ID: "call-bad-2", Name: "patch_texture", Arguments: json.RawMessage(`{"doc_id":"doc-1","find":"bad-find"}`)},
				},
				Usage: provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
				Model: "test-model",
			},
			{
				StopReason: "tool_use",
				ToolCalls: []types.ToolCall{
					{ID: "call-good", Name: "patch_texture", Arguments: json.RawMessage(`{"doc_id":"doc-1","content":"good"}`)},
				},
				Usage: provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
				Model: "test-model",
			},
			{
				StopReason: "end_turn",
				Text:       "done",
				Usage:      provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
				Model:      "test-model",
			},
		},
		choices:  &choices,
		requests: &requests,
	}

	var retrySeen bool
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		if kind != types.EventRunRetry || phase != "initial_tool_choice" {
			return
		}
		var decoded map[string]any
		if err := json.Unmarshal(payload, &decoded); err != nil {
			t.Fatalf("decode retry payload: %v", err)
		}
		if decoded["reason"] == "required_initial_tool_failed" {
			retrySeen = true
		}
	}

	text, _, err := RunToolLoop(context.Background(), provider, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":"write first"}`)},
		"You are a Texture appagent.",
		0,
		emit,
		nil,
		WithInitialToolChoice("function:patch_texture"),)
	if err != nil {
		t.Fatalf("run tool loop: %v", err)
	}
	if text != "done" {
		t.Fatalf("text = %q, want done", text)
	}
	if edited != 3 {
		t.Fatalf("patch_texture executed %d times, want both failed attempts plus retry", edited)
	}
	if len(choices) != 3 || choices[0] != "function:patch_texture" || choices[1] != "function:patch_texture" || choices[2] != "" {
		t.Fatalf("tool choices = %#v, want exact retry then unconstrained final", choices)
	}
	if len(requests) < 2 {
		t.Fatalf("requests = %d, want at least 2", len(requests))
	}
	for i := 0; i < 2; i++ {
		if len(requests[i].ToolDefinitions) != 1 || requests[i].ToolDefinitions[0].Name != "patch_texture" {
			t.Fatalf("request %d tool definitions = %+v, want only patch_texture", i, requests[i].ToolDefinitions)
		}
	}
	if !retrySeen {
		t.Fatal("missing required_initial_tool_failed retry event")
	}
}


func TestRunToolLoopCarriesAssistantReasoningContent(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.Register(Tool{
		Name:        "record_status",
		Description: "Record status.",
		Parameters:  map[string]any{"type": "object"},
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"ok":true}`, nil
		},
	}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	var requests []provideriface.ToolLoopRequest
	provider := &capturingToolChoiceProvider{responses: []*provideriface.ToolLoopResponse{
		{
			StopReason:       "tool_use",
			ReasoningContent: "hidden plan before tool",
			ToolCalls: []types.ToolCall{{
				ID:        "call-1",
				Name:      "record_status",
				Arguments: json.RawMessage(`{"status":"ok"}`),
			}},
			Usage: provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model: "test-model",
		},
		{
			StopReason: "end_turn",
			Text:       "done",
			Usage:      provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model:      "test-model",
		},
	}, requests: &requests}

	text, _, err := RunToolLoop(context.Background(), provider, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":"record"}`)},
		"You are helpful.",
		0,
		func(types.EventKind, string, json.RawMessage) {},
		nil,)
	if err != nil {
		t.Fatalf("run tool loop: %v", err)
	}
	if text != "done" {
		t.Fatalf("text = %q, want done", text)
	}
	if len(requests) != 2 {
		t.Fatalf("provider calls = %d, want 2", len(requests))
	}
	var assistant struct {
		Role             string `json:"role"`
		ReasoningContent string `json:"reasoning_content"`
	}
	foundAssistant := false
	for _, raw := range requests[1].Messages {
		if err := json.Unmarshal(raw, &assistant); err != nil {
			continue
		}
		if assistant.Role == "assistant" {
			foundAssistant = true
			break
		}
	}
	if !foundAssistant {
		t.Fatalf("second request messages did not include assistant turn: %s", rawMessagesForTest(requests[1].Messages))
	}
	if assistant.ReasoningContent != "hidden plan before tool" {
		t.Fatalf("reasoning_content = %q, want hidden plan before tool", assistant.ReasoningContent)
	}
}

func rawMessagesForTest(messages []json.RawMessage) string {
	parts := make([][]byte, 0, len(messages))
	for _, msg := range messages {
		parts = append(parts, []byte(msg))
	}
	return string(bytes.Join(parts, []byte("\n")))
}

func TestRunToolLoopRequiredToolTurnRetriesMissingToolWithoutArtificialBudget(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.Register(Tool{
		Name:        "record_status",
		Description: "Record status.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"status": map[string]any{"type": "string"}},
			"required":   []string{"status"},
		},
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"ok":true}`, nil
		},
	}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	var choices []string
	var maxTokens []int
	provider := &capturingToolChoiceProvider{responses: []*provideriface.ToolLoopResponse{
		{
			StopReason: "max_tokens",
			Text:       strings.Repeat("runaway prose ", 100),
			Usage:      provideriface.TokenUsage{InputTokens: 1, OutputTokens: 131072},
			Model:      "test-model",
		},
		{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:        "call-1",
				Name:      "record_status",
				Arguments: json.RawMessage(`{"status":"ok"}`),
			}},
			Usage: provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model: "test-model",
		},
		{
			StopReason: "end_turn",
			Text:       "done",
			Usage:      provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model:      "test-model",
		},
	}, choices: &choices, maxTokens: &maxTokens}

	var retrySeen bool
	text, _, err := RunToolLoop(context.Background(), provider, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":"record"}`)},
		"You are helpful.",
		131072,
		func(kind types.EventKind, phase string, payload json.RawMessage) {
			if kind == types.EventRunRetry && phase == "required_tool_call" {
				retrySeen = true
			}
		},
		nil,
		WithInitialToolChoice("required"),)
	if err != nil {
		t.Fatalf("run tool loop: %v", err)
	}
	if text != "done" {
		t.Fatalf("text = %q, want done", text)
	}
	if !retrySeen {
		t.Fatal("missing required_tool_call retry event")
	}
	if len(choices) != 3 || choices[0] != "required" || choices[1] != "required" || choices[2] != "" {
		t.Fatalf("choices = %#v, want required retry then normal completion", choices)
	}
	if len(maxTokens) != 3 || maxTokens[0] != 131072 || maxTokens[1] != 131072 || maxTokens[2] != 131072 {
		t.Fatalf("maxTokens = %#v, want selected model budget preserved", maxTokens)
	}
}

func TestRunToolLoopRequiredNextToolUsesRequiredChoice(t *testing.T) {
	var calls []string
	registry := NewToolRegistry()
	if err := registry.Register(Tool{
		Name:        "request_worker_vm",
		Description: "Request worker.",
		Parameters:  map[string]any{"type": "object"},
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			calls = append(calls, "request_worker_vm")
			return `{"status":"worker_requested","delegation_required":true,"next_tool":"start_worker_delegation","start_args":{"worker_sandbox_url":"http://worker","worker_id":"worker-1","profile":"vsuper"},"next_instruction":"Call start_worker_delegation next with start_args plus the full execution objective."}`, nil
		},
	}); err != nil {
		t.Fatalf("register request_worker_vm: %v", err)
	}
	if err := registry.Register(Tool{
		Name:        "start_worker_delegation",
		Description: "Start worker.",
		Parameters:  map[string]any{"type": "object"},
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			calls = append(calls, "start_worker_delegation")
			return `{"status":"worker_run_started","worker_run_id":"run-1"}`, nil
		},
	}); err != nil {
		t.Fatalf("register start_worker_delegation: %v", err)
	}

	var choices []string
	var maxTokens []int
	var requests []provideriface.ToolLoopRequest
	provider := &capturingToolChoiceProvider{responses: []*provideriface.ToolLoopResponse{
		{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:        "call-request",
				Name:      "request_worker_vm",
				Arguments: json.RawMessage(`{"purpose":"build"}`),
			}},
			Usage: provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model: "test-model",
		},
		{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:        "call-start",
				Name:      "start_worker_delegation",
				Arguments: json.RawMessage(`{"worker_sandbox_url":"http://worker","objective":"build"}`),
			}},
			Usage: provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model: "test-model",
		},
		{
			StopReason: "end_turn",
			Text:       "started",
			Usage:      provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model:      "test-model",
		},
	}, choices: &choices, maxTokens: &maxTokens, requests: &requests}

	var retrySeen bool
	text, _, err := RunToolLoop(context.Background(), provider, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":"start worker"}`)},
		"You are helpful.",
		131072,
		func(kind types.EventKind, phase string, payload json.RawMessage) {
			if kind == types.EventRunRetry && phase == "required_next_tool" {
				retrySeen = true
			}
		},
		nil,)
	if err != nil {
		t.Fatalf("run tool loop: %v", err)
	}
	if text != "started" {
		t.Fatalf("text = %q, want started", text)
	}
	if !retrySeen {
		t.Fatal("missing required_next_tool event")
	}
	if diff := strings.Join(calls, ","); diff != "request_worker_vm,start_worker_delegation" {
		t.Fatalf("calls = %s", diff)
	}
	if len(choices) != 3 || choices[0] != "" || choices[1] != "function:start_worker_delegation" || choices[2] != "" {
		t.Fatalf("choices = %#v, want second call to require exact start_worker_delegation", choices)
	}
	if len(maxTokens) != 3 || maxTokens[0] != 131072 || maxTokens[1] != 131072 || maxTokens[2] != 131072 {
		t.Fatalf("maxTokens = %#v, want selected model budget preserved", maxTokens)
	}
	if len(requests) < 2 || !strings.Contains(rawMessagesForTest(requests[1].Messages), "Runtime-required continuation: call start_worker_delegation now") {
		t.Fatalf("second request missing required-next-tool reminder: %s", rawMessagesForTest(requests[1].Messages))
	}
}

func TestRunToolLoopRequiredNextToolGetsFiniteBudgetWhenPolicyOmitsMaxTokens(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.Register(Tool{
		Name:        "request_worker_vm",
		Description: "Request worker.",
		Parameters:  map[string]any{"type": "object"},
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"status":"worker_requested","delegation_required":true,"next_tool":"start_worker_delegation","start_args":{"worker_sandbox_url":"http://worker","worker_id":"worker-1","profile":"vsuper"},"next_instruction":"Call start_worker_delegation next with start_args plus the full execution objective."}`, nil
		},
	}); err != nil {
		t.Fatalf("register request_worker_vm: %v", err)
	}
	if err := registry.Register(Tool{
		Name:        "start_worker_delegation",
		Description: "Start worker.",
		Parameters:  map[string]any{"type": "object"},
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"status":"worker_run_started","worker_run_id":"run-1"}`, nil
		},
	}); err != nil {
		t.Fatalf("register start_worker_delegation: %v", err)
	}

	var choices []string
	var maxTokens []int
	provider := &capturingToolChoiceProvider{responses: []*provideriface.ToolLoopResponse{
		{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:        "call-request",
				Name:      "request_worker_vm",
				Arguments: json.RawMessage(`{"purpose":"build"}`),
			}},
			Usage: provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model: "test-model",
		},
		{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:        "call-start",
				Name:      "start_worker_delegation",
				Arguments: json.RawMessage(`{"worker_sandbox_url":"http://worker","objective":"build"}`),
			}},
			Usage: provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model: "test-model",
		},
		{
			StopReason: "end_turn",
			Text:       "started",
			Usage:      provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model:      "test-model",
		},
	}, choices: &choices, maxTokens: &maxTokens}

	text, _, err := RunToolLoop(context.Background(), provider, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":"start worker"}`)},
		"You are helpful.",
		0,
		func(kind types.EventKind, phase string, payload json.RawMessage) {},
		nil,)
	if err != nil {
		t.Fatalf("run tool loop: %v", err)
	}
	if text != "started" {
		t.Fatalf("text = %q, want started", text)
	}
	if len(choices) != 3 || choices[0] != "" || choices[1] != "function:start_worker_delegation" || choices[2] != "" {
		t.Fatalf("choices = %#v, want second call to require exact start_worker_delegation", choices)
	}
	if len(maxTokens) != 3 || maxTokens[0] != 0 || maxTokens[1] != requiredNextToolDefaultMaxTokens || maxTokens[2] != 0 {
		t.Fatalf("maxTokens = %#v, want omitted normal calls and finite required-next-tool budget", maxTokens)
	}
}

func TestRunToolLoopRequiredNextToolMaxTokensStopsAfterBoundedRetries(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.Register(Tool{
		Name:        "request_worker_vm",
		Description: "Request worker.",
		Parameters:  map[string]any{"type": "object"},
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"status":"worker_requested","delegation_required":true,"next_tool":"start_worker_delegation","start_args":{"worker_sandbox_url":"http://worker"}}`, nil
		},
	}); err != nil {
		t.Fatalf("register request_worker_vm: %v", err)
	}
	if err := registry.Register(Tool{
		Name:        "start_worker_delegation",
		Description: "Start worker.",
		Parameters:  map[string]any{"type": "object"},
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"status":"worker_run_started"}`, nil
		},
	}); err != nil {
		t.Fatalf("register start_worker_delegation: %v", err)
	}

	var choices []string
	var retryAttempts []int
	provider := &capturingToolChoiceProvider{responses: []*provideriface.ToolLoopResponse{
		{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:        "call-request",
				Name:      "request_worker_vm",
				Arguments: json.RawMessage(`{"purpose":"build"}`),
			}},
			Usage: provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model: "test-model",
		},
		{
			StopReason: "max_tokens",
			Text:       strings.Repeat("thinking ", 100),
			Usage:      provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model:      "test-model",
		},
	}, choices: &choices}

	_, _, err := RunToolLoop(context.Background(), provider, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":"start worker"}`)},
		"You are helpful.",
		131072,
		func(kind types.EventKind, phase string, payload json.RawMessage) {
			if kind != types.EventRunRetry || phase != "required_tool_call" {
				return
			}
			var decoded map[string]any
			if json.Unmarshal(payload, &decoded) == nil {
				retryAttempts = append(retryAttempts, intMapValue(decoded, "attempt"))
			}
		},
		nil,)
	if err == nil || !strings.Contains(err.Error(), `required next tool "start_worker_delegation" was not called after 2 retries`) {
		t.Fatalf("err = %v, want bounded required next tool retry error", err)
	}
	if len(choices) != 4 || choices[0] != "" || choices[1] != "function:start_worker_delegation" || choices[2] != "function:start_worker_delegation" || choices[3] != "function:start_worker_delegation" {
		t.Fatalf("choices = %#v, want exact required tool until bounded failure", choices)
	}
	if len(retryAttempts) != 2 || retryAttempts[0] != 1 || retryAttempts[1] != 2 {
		t.Fatalf("retryAttempts = %#v, want [1 2]", retryAttempts)
	}
}

func TestRunToolLoopRetriesEndTurnBeforeRequiredNextTool(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.Register(Tool{
		Name:        "request_worker_vm",
		Description: "Request worker.",
		Parameters:  map[string]any{"type": "object"},
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"status":"worker_requested","delegation_required":true,"next_tool":"start_worker_delegation","start_args":{"worker_sandbox_url":"http://worker"}}`, nil
		},
	}); err != nil {
		t.Fatalf("register request_worker_vm: %v", err)
	}
	if err := registry.Register(Tool{
		Name:        "start_worker_delegation",
		Description: "Start worker.",
		Parameters:  map[string]any{"type": "object"},
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"status":"worker_run_started"}`, nil
		},
	}); err != nil {
		t.Fatalf("register start_worker_delegation: %v", err)
	}

	var choices []string
	provider := &capturingToolChoiceProvider{responses: []*provideriface.ToolLoopResponse{
		{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:        "call-request",
				Name:      "request_worker_vm",
				Arguments: json.RawMessage(`{"purpose":"build"}`),
			}},
			Usage: provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model: "test-model",
		},
		{
			StopReason: "end_turn",
			Text:       "worker requested",
			Usage:      provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model:      "test-model",
		},
		{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:        "call-start",
				Name:      "start_worker_delegation",
				Arguments: json.RawMessage(`{"worker_sandbox_url":"http://worker","objective":"build"}`),
			}},
			Usage: provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model: "test-model",
		},
		{
			StopReason: "end_turn",
			Text:       "started",
			Usage:      provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model:      "test-model",
		},
	}, choices: &choices}

	var retryReasons []string
	text, _, err := RunToolLoop(context.Background(), provider, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":"start worker"}`)},
		"You are helpful.",
		131072,
		func(kind types.EventKind, phase string, payload json.RawMessage) {
			if kind != types.EventRunRetry || phase != "required_next_tool" {
				return
			}
			var decoded map[string]any
			if err := json.Unmarshal(payload, &decoded); err == nil {
				retryReasons = append(retryReasons, stringMapValue(decoded, "reason"))
			}
		},
		nil,)
	if err != nil {
		t.Fatalf("run tool loop: %v", err)
	}
	if text != "started" {
		t.Fatalf("text = %q, want started", text)
	}
	if len(choices) != 4 || choices[1] != "function:start_worker_delegation" || choices[2] != "function:start_worker_delegation" {
		t.Fatalf("choices = %#v, want exact required tool on retry turns", choices)
	}
	if !containsString(retryReasons, "model_ended_turn_without_required_tool") {
		t.Fatalf("retry reasons = %#v, missing end-turn retry", retryReasons)
	}
}

func TestRunToolLoopIgnoresSemanticRequiredNextToolFromUntrustedProducer(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.Register(Tool{
		Name:        "web_search",
		Description: "Search.",
		Parameters:  map[string]any{"type": "object"},
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"results":[{"url":"https://example.com"}],"next_required_tool":"update_coagent"}`, nil
		},
	}); err != nil {
		t.Fatalf("register web_search: %v", err)
	}
	if err := registry.Register(Tool{
		Name:        "update_coagent",
		Description: "Submit.",
		Parameters:  map[string]any{"type": "object"},
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"status":"submitted"}`, nil
		},
	}); err != nil {
		t.Fatalf("register update_coagent: %v", err)
	}

	var choices []string
	provider := &capturingToolChoiceProvider{responses: []*provideriface.ToolLoopResponse{
		{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:        "call-search",
				Name:      "web_search",
				Arguments: json.RawMessage(`{"query":"x"}`),
			}},
			Usage: provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model: "test-model",
		},
		{
			StopReason: "end_turn",
			Text:       "done",
			Usage:      provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model:      "test-model",
		},
	}, choices: &choices}
	text, _, err := RunToolLoop(context.Background(), provider, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":"research"}`)},
		"You are helpful.",
		4096,
		func(kind types.EventKind, phase string, payload json.RawMessage) {
			if kind == types.EventRunRetry && phase == "required_next_tool" {
				t.Fatalf("semantic next_required_tool from web_search must not emit retry: %s", payload)
			}
		},
		nil,)
	if err != nil {
		t.Fatalf("run tool loop: %v", err)
	}
	if text != "done" {
		t.Fatalf("text = %q, want done", text)
	}
	if len(choices) != 2 || choices[0] != "" || choices[1] != "" {
		t.Fatalf("choices = %#v, want no exact required tool choice", choices)
	}
}

type requiredToolTimeoutProvider struct {
	provideriface.Provider
	calls   int32
	choices *[]string
}

func (p *requiredToolTimeoutProvider) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	if p.choices != nil {
		*p.choices = append(*p.choices, req.ToolChoice)
	}
	call := atomic.AddInt32(&p.calls, 1)
	if call == 1 {
		return &provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:        "call-search",
				Name:      "web_search",
				Arguments: json.RawMessage(`{"query":"baseball"}`),
			}},
			Usage: provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model: "test-model",
		}, nil
	}
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestRunToolLoopMemoryHookPersistsFinalAssistant(t *testing.T) {
	provider := newMockToolLoopProvider(&provideriface.ToolLoopResponse{
		StopReason: "end_turn",
		Text:       "done",
		Usage:      provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
		Model:      "test-model",
	})

	var appended []string
	hooks := ToolLoopMemoryHooks{
		AfterAppendMessage: func(ctx context.Context, role string, msg json.RawMessage) error {
			appended = append(appended, role+":"+string(msg))
			return nil
		},
	}

	text, _, err := RunToolLoop(context.Background(), provider, nil, []json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`)},
		"You are helpful.",
		4096,
		func(types.EventKind, string, json.RawMessage) {},
		nil,
		WithToolLoopMemoryHooks(hooks),)
	if err != nil {
		t.Fatalf("run tool loop: %v", err)
	}
	if text != "done" {
		t.Fatalf("text: got %q, want done", text)
	}
	if len(appended) != 1 {
		t.Fatalf("appended messages: got %d, want 1", len(appended))
	}
	if !strings.HasPrefix(appended[0], "assistant:") {
		t.Fatalf("appended role: got %q, want assistant", appended[0])
	}
}

type overflowThenSuccessProvider struct {
	provideriface.Provider
	calls int32
}

func (p *overflowThenSuccessProvider) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	call := atomic.AddInt32(&p.calls, 1)
	if call == 1 {
		return nil, fmt.Errorf("maximum context length exceeded")
	}
	return &provideriface.ToolLoopResponse{
		StopReason: "end_turn",
		Text:       fmt.Sprintf("recovered with %d messages", len(req.Messages)),
		Usage:      provideriface.TokenUsage{InputTokens: 2, OutputTokens: 3},
		Model:      "test-model",
	}, nil
}

func TestRunToolLoopMemoryHookCanRetryProviderOverflow(t *testing.T) {
	provider := &overflowThenSuccessProvider{}
	var retried bool
	hooks := ToolLoopMemoryHooks{
		OnProviderError: func(ctx context.Context, messages []json.RawMessage, err error) ([]json.RawMessage, bool, error) {
			if !isContextOverflowError(err) {
				return nil, false, nil
			}
			retried = true
			return []json.RawMessage{json.RawMessage(`{"role":"user","content":"compacted"}`)}, true, nil
		},
	}

	text, usage, err := RunToolLoop(context.Background(), provider, nil, []json.RawMessage{json.RawMessage(`{"role":"user","content":"very long"}`)},
		"You are helpful.",
		4096,
		func(types.EventKind, string, json.RawMessage) {},
		nil,
		WithToolLoopMemoryHooks(hooks),)
	if err != nil {
		t.Fatalf("run tool loop: %v", err)
	}
	if !retried {
		t.Fatalf("expected retry hook")
	}
	if got := atomic.LoadInt32(&provider.calls); got != 2 {
		t.Fatalf("provider calls: got %d, want 2", got)
	}
	if text != "recovered with 1 messages" {
		t.Fatalf("text: got %q", text)
	}
	if usage.InputTokens != 2 || usage.OutputTokens != 3 {
		t.Fatalf("usage: got %+v", usage)
	}
}

type rateLimitThenSuccessProvider struct {
	provideriface.Provider
	calls int32
}

func (p *rateLimitThenSuccessProvider) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	call := atomic.AddInt32(&p.calls, 1)
	if call == 1 {
		return nil, fmt.Errorf("gateway call failed: chatgpt: status 429 Too Many Requests (sanitized)")
	}
	return &provideriface.ToolLoopResponse{
		StopReason: "end_turn",
		Text:       "recovered after rate limit",
		Usage:      provideriface.TokenUsage{InputTokens: 4, OutputTokens: 5},
		Model:      "test-model",
	}, nil
}

type exactToolChoicePreconditionThenToolProvider struct {
	provideriface.Provider
	calls    int32
	choices  []string
	requests []provideriface.ToolLoopRequest
}

func (p *exactToolChoicePreconditionThenToolProvider) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	p.requests = append(p.requests, req)
	p.choices = append(p.choices, req.ToolChoice)
	call := atomic.AddInt32(&p.calls, 1)
	if call == 1 {
		return nil, fmt.Errorf("gateway call failed: fireworks: status 412 Precondition Failed (sanitized)")
	}
	return &provideriface.ToolLoopResponse{
		StopReason: "tool_use",
		ToolCalls: []types.ToolCall{{
			ID:        "call-edit",
			Name:      "patch_texture",
			Arguments: json.RawMessage(`{"doc_id":"doc-1","content":"mission checkpoint"}`),
		}},
		Usage: provideriface.TokenUsage{InputTokens: 8, OutputTokens: 2},
		Model: "accounts/fireworks/models/deepseek-v4-flash",
	}, nil
}

type providerPreconditionThenToolProvider struct {
	provideriface.Provider
	failuresBeforeSuccess int32
	calls                 int32
	requests              []provideriface.ToolLoopRequest
}

func (p *providerPreconditionThenToolProvider) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	p.requests = append(p.requests, req)
	call := atomic.AddInt32(&p.calls, 1)
	if call <= p.failuresBeforeSuccess {
		return nil, fmt.Errorf("gateway call failed: fireworks: status 412 Precondition Failed (sanitized)")
	}
	return &provideriface.ToolLoopResponse{
		StopReason: "tool_use",
		ToolCalls: []types.ToolCall{{
			ID:        "call-edit",
			Name:      "patch_texture",
			Arguments: json.RawMessage(`{"doc_id":"doc-1","content":"mission checkpoint"}`),
		}},
		Usage: provideriface.TokenUsage{InputTokens: 9, OutputTokens: 3},
		Model: req.Model,
	}, nil
}

type providerErrorsThenToolProvider struct {
	provideriface.Provider
	errors   []error
	calls    int32
	requests []provideriface.ToolLoopRequest
}

func (p *providerErrorsThenToolProvider) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	p.requests = append(p.requests, req)
	call := int(atomic.AddInt32(&p.calls, 1))
	if call <= len(p.errors) {
		return nil, p.errors[call-1]
	}
	return &provideriface.ToolLoopResponse{
		StopReason: "tool_use",
		ToolCalls: []types.ToolCall{{
			ID:        "call-edit",
			Name:      "patch_texture",
			Arguments: json.RawMessage(`{"doc_id":"doc-1","content":"mission checkpoint"}`),
		}},
		Usage: provideriface.TokenUsage{InputTokens: 12, OutputTokens: 4},
		Model: req.Model,
	}, nil
}

func TestRunToolLoopRelaxesExactInitialToolChoiceAfterProviderPrecondition(t *testing.T) {
	provider := &exactToolChoicePreconditionThenToolProvider{}
	registry := NewToolRegistry()
	if err := registry.Register(Tool{
		Name:        "patch_texture",
		Description: "Edit the Texture document.",
		Parameters:  map[string]any{"type": "object"},
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"status":"ok","revision_id":"rev-1"}`, nil
		},
	}); err != nil {
		t.Fatalf("register patch_texture: %v", err)
	}
	if err := registry.Register(Tool{
		Name:        "request_super_execution",
		Description: "Ask Super to execute follow-on platform work.",
		Parameters:  map[string]any{"type": "object"},
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"status":"requested"}`, nil
		},
	}); err != nil {
		t.Fatalf("register request_super_execution: %v", err)
	}
	var retrySeen bool
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		if kind == types.EventRunRetry && phase == "provider_tool_choice" {
			retrySeen = true
			var decoded map[string]any
			if err := json.Unmarshal(payload, &decoded); err != nil {
				t.Fatalf("decode retry payload: %v", err)
			}
			if decoded["tool_choice"] != "function:patch_texture" || decoded["retry_tool_choice"] != "required" {
				t.Fatalf("retry payload = %+v", decoded)
			}
		}
	}

	text, usage, err := RunToolLoop(context.Background(), provider, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":[{"type":"text","text":"write the mission checkpoint"}]}`)},
		"You are a Texture appagent.",
		0,
		emit,
		nil,
		WithInitialToolChoice("function:patch_texture"),
		WithTerminalToolSuccesses("patch_texture"),)
	if err != nil {
		t.Fatalf("run tool loop: %v", err)
	}
	if text != "" {
		t.Fatalf("text = %q, want empty terminal tool result", text)
	}
	if usage.InputTokens != 8 || usage.OutputTokens != 2 {
		t.Fatalf("usage = %+v", usage)
	}
	if got := atomic.LoadInt32(&provider.calls); got != 2 {
		t.Fatalf("provider calls = %d, want 2", got)
	}
	if len(provider.choices) != 2 || provider.choices[0] != "function:patch_texture" || provider.choices[1] != "required" {
		t.Fatalf("tool choices = %#v, want exact then required", provider.choices)
	}
	if len(provider.requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(provider.requests))
	}
	for i, req := range provider.requests {
		if len(req.ToolDefinitions) != 1 || req.ToolDefinitions[0].Name != "patch_texture" {
			t.Fatalf("request %d tool definitions = %+v, want only patch_texture", i, req.ToolDefinitions)
		}
	}
	if !retrySeen {
		t.Fatal("missing provider_tool_choice retry event")
	}
}

type deepSeekToolChoicePreconditionThenToolProvider struct {
	provideriface.Provider
	choices []string
	calls   int32
}

func (p *deepSeekToolChoicePreconditionThenToolProvider) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	p.choices = append(p.choices, req.ToolChoice)
	call := atomic.AddInt32(&p.calls, 1)
	if call == 1 {
		return nil, fmt.Errorf("provider deepseek call failed: deepseek: Thinking mode does not support this tool_choice")
	}
	return &provideriface.ToolLoopResponse{
		StopReason: "tool_use",
		ToolCalls: []types.ToolCall{{
			ID:        "call-edit",
			Name:      "patch_texture",
			Arguments: json.RawMessage(`{"content":"ok"}`),
		}},
		Usage: provideriface.TokenUsage{InputTokens: 11, OutputTokens: 5},
		Model: req.Model,
	}, nil
}

func TestRunToolLoopRelaxesExactInitialToolChoiceAfterDeepSeekThinkingToolChoiceError(t *testing.T) {
	provider := &deepSeekToolChoicePreconditionThenToolProvider{}
	registry := NewToolRegistry()
	if err := registry.Register(Tool{
		Name:        "patch_texture",
		Description: "Edit the Texture document.",
		Parameters:  map[string]any{"type": "object"},
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"status":"ok","revision_id":"rev-1"}`, nil
		},
	}); err != nil {
		t.Fatalf("register patch_texture: %v", err)
	}
	var retrySeen bool
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		if kind == types.EventRunRetry && phase == "provider_tool_choice" {
			retrySeen = true
			var decoded map[string]any
			if err := json.Unmarshal(payload, &decoded); err != nil {
				t.Fatalf("decode retry payload: %v", err)
			}
			if decoded["tool_choice"] != "function:patch_texture" || decoded["retry_tool_choice"] != "required" {
				t.Fatalf("retry payload = %+v", decoded)
			}
		}
	}

	_, _, err := RunToolLoop(context.Background(), provider, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":[{"type":"text","text":"write"}]}`)},
		"You are a Texture appagent.",
		0,
		emit,
		nil,
		WithInitialToolChoice("function:patch_texture"),
		WithTerminalToolSuccesses("patch_texture"),)
	if err != nil {
		t.Fatalf("run tool loop: %v", err)
	}
	if len(provider.choices) != 2 || provider.choices[0] != "function:patch_texture" || provider.choices[1] != "required" {
		t.Fatalf("tool choices = %#v, want exact then required", provider.choices)
	}
	if !retrySeen {
		t.Fatal("missing provider_tool_choice retry event")
	}
}

func TestRunToolLoopFallsBackModelAfterRelaxedInitialToolChoicePrecondition(t *testing.T) {
	provider := &providerPreconditionThenToolProvider{failuresBeforeSuccess: 2}
	registry := NewToolRegistry()
	if err := registry.Register(Tool{
		Name:        "patch_texture",
		Description: "Edit the Texture document.",
		Parameters:  map[string]any{"type": "object"},
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"status":"ok","revision_id":"rev-1"}`, nil
		},
	}); err != nil {
		t.Fatalf("register patch_texture: %v", err)
	}
	if err := registry.Register(Tool{
		Name:        "request_super_execution",
		Description: "Ask Super to execute follow-on platform work.",
		Parameters:  map[string]any{"type": "object"},
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"status":"requested"}`, nil
		},
	}); err != nil {
		t.Fatalf("register request_super_execution: %v", err)
	}
	var modelFallbackSeen bool
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		if kind == types.EventRunRetry && phase == "provider_model_fallback" {
			modelFallbackSeen = true
			var decoded map[string]any
			if err := json.Unmarshal(payload, &decoded); err != nil {
				t.Fatalf("decode fallback payload: %v", err)
			}
			if decoded["from_model"] != "accounts/fireworks/models/deepseek-v4-flash" ||
				decoded["to_model"] != "accounts/fireworks/models/deepseek-v4-pro" {
				t.Fatalf("fallback payload = %+v", decoded)
			}
		}
	}

	text, usage, err := RunToolLoop(context.Background(), provider, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":[{"type":"text","text":"write the mission checkpoint"}]}`)},
		"You are a Texture appagent.",
		0,
		emit,
		nil,
		WithToolLoopLLMConfig(provideriface.LLMSelection{
			Provider:        "fireworks",
			Model:           "accounts/fireworks/models/deepseek-v4-flash",
			ReasoningEffort: "medium",
		}),
		WithProviderPreconditionFallbacks(provideriface.LLMSelection{
			Provider:        "fireworks",
			Model:           "accounts/fireworks/models/deepseek-v4-pro",
			ReasoningEffort: "medium",
			Source:          "test_fallback",
		}),
		WithInitialToolChoice("function:patch_texture"),
		WithTerminalToolSuccesses("patch_texture"),)
	if err != nil {
		t.Fatalf("run tool loop: %v", err)
	}
	if text != "" {
		t.Fatalf("text = %q, want empty terminal tool result", text)
	}
	if usage.InputTokens != 9 || usage.OutputTokens != 3 {
		t.Fatalf("usage = %+v", usage)
	}
	if got := atomic.LoadInt32(&provider.calls); got != 3 {
		t.Fatalf("provider calls = %d, want 3", got)
	}
	if !modelFallbackSeen {
		t.Fatal("missing provider_model_fallback retry event")
	}
	if len(provider.requests) != 3 {
		t.Fatalf("requests = %d, want 3", len(provider.requests))
	}
	wantChoices := []string{"function:patch_texture", "required", "required"}
	wantModels := []string{
		"accounts/fireworks/models/deepseek-v4-flash",
		"accounts/fireworks/models/deepseek-v4-flash",
		"accounts/fireworks/models/deepseek-v4-pro",
	}
	for i, req := range provider.requests {
		if req.ToolChoice != wantChoices[i] || req.Model != wantModels[i] {
			t.Fatalf("request %d choice/model = %q/%q, want %q/%q", i, req.ToolChoice, req.Model, wantChoices[i], wantModels[i])
		}
		if len(req.ToolDefinitions) != 1 || req.ToolDefinitions[0].Name != "patch_texture" {
			t.Fatalf("request %d tool definitions = %+v, want only patch_texture", i, req.ToolDefinitions)
		}
	}
}

func TestRunToolLoopTriesMultipleProviderPreconditionFallbacks(t *testing.T) {
	provider := &providerPreconditionThenToolProvider{failuresBeforeSuccess: 3}
	registry := NewToolRegistry()
	if err := registry.Register(Tool{
		Name:        "patch_texture",
		Description: "Edit the Texture document.",
		Parameters:  map[string]any{"type": "object"},
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"status":"ok","revision_id":"rev-1"}`, nil
		},
	}); err != nil {
		t.Fatalf("register patch_texture: %v", err)
	}
	var fallbackModels []string
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		if kind != types.EventRunRetry || phase != "provider_model_fallback" {
			return
		}
		var decoded map[string]any
		if err := json.Unmarshal(payload, &decoded); err != nil {
			t.Fatalf("decode fallback payload: %v", err)
		}
		fallbackModels = append(fallbackModels, fmt.Sprintf("%s/%s", decoded["to_provider"], decoded["to_model"]))
	}

	_, _, err := RunToolLoop(context.Background(), provider, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":[{"type":"text","text":"write the mission checkpoint"}]}`)},
		"You are a Texture appagent.",
		0,
		emit,
		nil,
		WithToolLoopLLMConfig(provideriface.LLMSelection{
			Provider:        "fireworks",
			Model:           "accounts/fireworks/models/deepseek-v4-flash",
			ReasoningEffort: "medium",
		}),
		WithProviderPreconditionFallbacks(
			provideriface.LLMSelection{
				Provider:        "fireworks",
				Model:           "accounts/fireworks/models/deepseek-v4-pro",
				ReasoningEffort: "medium",
				Source:          "test_fireworks_fallback",
			},
			provideriface.LLMSelection{
				Provider:        "chatgpt",
				Model:           "gpt-5.5",
				ReasoningEffort: "low",
				Source:          "test_platform_fallback",
			},
		),
		WithInitialToolChoice("function:patch_texture"),
		WithTerminalToolSuccesses("patch_texture"),)
	if err != nil {
		t.Fatalf("run tool loop: %v", err)
	}
	if got := atomic.LoadInt32(&provider.calls); got != 4 {
		t.Fatalf("provider calls = %d, want 4", got)
	}
	wantFallbacks := []string{
		"fireworks/accounts/fireworks/models/deepseek-v4-pro",
		"chatgpt/gpt-5.5",
	}
	if !reflect.DeepEqual(fallbackModels, wantFallbacks) {
		t.Fatalf("fallback models = %+v, want %+v", fallbackModels, wantFallbacks)
	}
	wantChoices := []string{"function:patch_texture", "required", "required", "required"}
	wantModels := []string{
		"accounts/fireworks/models/deepseek-v4-flash",
		"accounts/fireworks/models/deepseek-v4-flash",
		"accounts/fireworks/models/deepseek-v4-pro",
		"gpt-5.5",
	}
	for i, req := range provider.requests {
		if req.ToolChoice != wantChoices[i] || req.Model != wantModels[i] {
			t.Fatalf("request %d choice/model = %q/%q, want %q/%q", i, req.ToolChoice, req.Model, wantChoices[i], wantModels[i])
		}
		if len(req.ToolDefinitions) != 1 || req.ToolDefinitions[0].Name != "patch_texture" {
			t.Fatalf("request %d tool definitions = %+v, want only patch_texture", i, req.ToolDefinitions)
		}
	}
}

func TestRunToolLoopFallsBackAfterProviderAvailabilityError(t *testing.T) {
	provider := &providerErrorsThenToolProvider{errors: []error{
		fmt.Errorf("gateway call failed: fireworks: status 412 Precondition Failed (sanitized)"),
		fmt.Errorf("gateway call failed: deepseek: status 402 Payment Required (sanitized)"),
	}}
	registry := NewToolRegistry()
	if err := registry.Register(Tool{
		Name:        "patch_texture",
		Description: "Edit the Texture document.",
		Parameters:  map[string]any{"type": "object"},
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"status":"ok","revision_id":"rev-1"}`, nil
		},
	}); err != nil {
		t.Fatalf("register patch_texture: %v", err)
	}
	var fallbackReasons []string
	var fallbackModels []string
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		if kind != types.EventRunRetry || phase != "provider_model_fallback" {
			return
		}
		var decoded map[string]any
		if err := json.Unmarshal(payload, &decoded); err != nil {
			t.Fatalf("decode fallback payload: %v", err)
		}
		fallbackReasons = append(fallbackReasons, fmt.Sprint(decoded["reason"]))
		fallbackModels = append(fallbackModels, fmt.Sprintf("%s/%s", decoded["to_provider"], decoded["to_model"]))
	}

	text, usage, err := RunToolLoop(context.Background(), provider, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":[{"type":"text","text":"run the wire proof"}]}`)},
		"You are Super.",
		0,
		emit,
		nil,
		WithToolLoopLLMConfig(provideriface.LLMSelection{
			Provider:        "fireworks",
			Model:           "accounts/fireworks/models/deepseek-v4-pro",
			ReasoningEffort: "medium",
		}),
		WithProviderPreconditionFallbacks(
			provideriface.LLMSelection{
				Provider:        "deepseek",
				Model:           "deepseek-v4-pro",
				ReasoningEffort: "medium",
				Source:          "test_deepseek_fallback",
			},
			provideriface.LLMSelection{
				Provider:        "chatgpt",
				Model:           "gpt-5.5",
				ReasoningEffort: "medium",
				Source:          "test_platform_fallback",
			},
		),
		WithTerminalToolSuccesses("patch_texture"),)
	if err != nil {
		t.Fatalf("run tool loop: %v", err)
	}
	if text != "" {
		t.Fatalf("text = %q, want empty terminal tool result", text)
	}
	if usage.InputTokens != 12 || usage.OutputTokens != 4 {
		t.Fatalf("usage = %+v", usage)
	}
	if got := atomic.LoadInt32(&provider.calls); got != 3 {
		t.Fatalf("provider calls = %d, want 3", got)
	}
	if !reflect.DeepEqual(fallbackReasons, []string{"provider_precondition_fallback", "provider_availability_fallback"}) {
		t.Fatalf("fallback reasons = %+v", fallbackReasons)
	}
	if !reflect.DeepEqual(fallbackModels, []string{"deepseek/deepseek-v4-pro", "chatgpt/gpt-5.5"}) {
		t.Fatalf("fallback models = %+v", fallbackModels)
	}
	if len(provider.requests) != 3 {
		t.Fatalf("requests = %d, want 3", len(provider.requests))
	}
	wantModels := []string{
		"accounts/fireworks/models/deepseek-v4-pro",
		"deepseek-v4-pro",
		"gpt-5.5",
	}
	for i, req := range provider.requests {
		if req.ToolChoice != "" {
			t.Fatalf("request %d tool choice = %q, want empty", i, req.ToolChoice)
		}
		if req.Model != wantModels[i] {
			t.Fatalf("request %d model = %q, want %q", i, req.Model, wantModels[i])
		}
	}
}

func TestRunToolLoopTriesProviderPreconditionFallbackWithoutToolChoice(t *testing.T) {
	provider := &providerPreconditionThenToolProvider{failuresBeforeSuccess: 1}
	registry := NewToolRegistry()
	if err := registry.Register(Tool{
		Name:        "patch_texture",
		Description: "Edit the Texture document.",
		Parameters:  map[string]any{"type": "object"},
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"status":"ok","revision_id":"rev-1"}`, nil
		},
	}); err != nil {
		t.Fatalf("register patch_texture: %v", err)
	}
	var fallbackModels []string
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		if kind != types.EventRunRetry || phase != "provider_model_fallback" {
			return
		}
		var decoded map[string]any
		if err := json.Unmarshal(payload, &decoded); err != nil {
			t.Fatalf("decode fallback payload: %v", err)
		}
		fallbackModels = append(fallbackModels, fmt.Sprintf("%s/%s", decoded["to_provider"], decoded["to_model"]))
	}

	_, _, err := RunToolLoop(context.Background(), provider, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":[{"type":"text","text":"lease a worker"}]}`)},
		"You are Super.",
		0,
		emit,
		nil,
		WithToolLoopLLMConfig(provideriface.LLMSelection{
			Provider:        "fireworks",
			Model:           "accounts/fireworks/models/deepseek-v4-pro",
			ReasoningEffort: "medium",
		}),
		WithProviderPreconditionFallbacks(provideriface.LLMSelection{
			Provider:        "deepseek",
			Model:           "deepseek-v4-pro",
			ReasoningEffort: "medium",
			Source:          "test_deepseek_fallback",
		}),
		WithTerminalToolSuccesses("patch_texture"),)
	if err != nil {
		t.Fatalf("run tool loop: %v", err)
	}
	if got := atomic.LoadInt32(&provider.calls); got != 2 {
		t.Fatalf("provider calls = %d, want 2", got)
	}
	if !reflect.DeepEqual(fallbackModels, []string{"deepseek/deepseek-v4-pro"}) {
		t.Fatalf("fallback models = %+v", fallbackModels)
	}
	if len(provider.requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(provider.requests))
	}
	for i, req := range provider.requests {
		if req.ToolChoice != "" {
			t.Fatalf("request %d tool choice = %q, want empty", i, req.ToolChoice)
		}
	}
	if provider.requests[1].Provider != "deepseek" || provider.requests[1].Model != "deepseek-v4-pro" {
		t.Fatalf("fallback request provider/model = %q/%q", provider.requests[1].Provider, provider.requests[1].Model)
	}
}

func TestRunToolLoopRetriesProviderRateLimit(t *testing.T) {
	originalDelays := providerRateLimitRetryDelays
	providerRateLimitRetryDelays = []time.Duration{0}
	defer func() { providerRateLimitRetryDelays = originalDelays }()

	provider := &rateLimitThenSuccessProvider{}
	var retryEvents int
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		if kind == types.EventRunRetry && phase == "provider_rate_limit" {
			retryEvents++
		}
	}

	text, usage, err := RunToolLoop(context.Background(), provider, nil, []json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`)},
		"You are helpful.",
		4096,
		emit,
		nil,)
	if err != nil {
		t.Fatalf("run tool loop: %v", err)
	}
	if text != "recovered after rate limit" {
		t.Fatalf("text = %q", text)
	}
	if got := atomic.LoadInt32(&provider.calls); got != 2 {
		t.Fatalf("provider calls = %d, want 2", got)
	}
	if retryEvents != 1 {
		t.Fatalf("retry events = %d, want 1", retryEvents)
	}
	if usage.InputTokens != 4 || usage.OutputTokens != 5 {
		t.Fatalf("usage = %+v", usage)
	}
}

func TestRunToolLoopWithToolUse(t *testing.T) {
	// LLM first returns tool_use, then end_turn after seeing tool result.
	registry := NewToolRegistry()
	if err := registry.Register(Tool{
		Name: "calculator",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "42", nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	provider := newMockToolLoopProvider(
		// First response: requests tool use.
		&provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			Text:       "",
			ToolCalls: []types.ToolCall{
				{ID: "call-1", Name: "calculator", Arguments: json.RawMessage(`{"expr":"2+2"}`)},
			},
			Usage: provideriface.TokenUsage{InputTokens: 15, OutputTokens: 10},
			Model: "test-model",
		},
		// Second response: final answer after tool result.
		&provideriface.ToolLoopResponse{
			StopReason: "end_turn",
			Text:       "The answer is 42.",
			Usage:      provideriface.TokenUsage{InputTokens: 25, OutputTokens: 5},
			Model:      "test-model",
		},
	)

	var emittedEvents []types.EventKind
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		emittedEvents = append(emittedEvents, kind)
	}

	text, usage, err := RunToolLoop(context.Background(), provider, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":"calculate 2+2"}`)},
		"You are helpful.",
		4096,
		emit,
		nil,)

	if err != nil {
		t.Fatalf("run tool loop: %v", err)
	}
	if text != "The answer is 42." {
		t.Errorf("text: got %q, want The answer is 42.", text)
	}
	if provider.CallCount() != 2 {
		t.Errorf("call count: got %d, want 2 (one tool_use + one end_turn)", provider.CallCount())
	}

	// Should have tool.invoked and tool.result events.
	foundInvoked := false
	foundResult := false
	for _, k := range emittedEvents {
		if k == types.EventToolInvoked {
			foundInvoked = true
		}
		if k == types.EventToolResult {
			foundResult = true
		}
	}
	if !foundInvoked {
		t.Error("expected tool.invoked event")
	}
	if !foundResult {
		t.Error("expected tool.result event")
	}

	// Token usage should accumulate across iterations.
	if usage.InputTokens != 40 || usage.OutputTokens != 15 {
		t.Errorf("total usage: got in=%d out=%d, want in=40 out=15", usage.InputTokens, usage.OutputTokens)
	}
}

func TestRunToolLoopMultipleToolIterations(t *testing.T) {
	// LLM uses tools twice before returning end_turn.
	registry := NewToolRegistry()

	if err := registry.Register(Tool{
		Name: "search",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "search results", nil
		},
	}); err != nil {
		t.Fatalf("register search: %v", err)
	}
	if err := registry.Register(Tool{
		Name: "read",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "file contents", nil
		},
	}); err != nil {
		t.Fatalf("register read: %v", err)
	}

	provider := newMockToolLoopProvider(
		// First response: search.
		&provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{
				{ID: "call-1", Name: "search", Arguments: json.RawMessage(`{"query":"test"}`)},
			},
			Usage: provideriface.TokenUsage{InputTokens: 20, OutputTokens: 10},
		},
		// Second response: read.
		&provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{
				{ID: "call-2", Name: "read", Arguments: json.RawMessage(`{"path":"/tmp/test"}`)},
			},
			Usage: provideriface.TokenUsage{InputTokens: 30, OutputTokens: 10},
		},
		// Third response: final answer.
		&provideriface.ToolLoopResponse{
			StopReason: "end_turn",
			Text:       "Based on my search and reading, here is the answer.",
			Usage:      provideriface.TokenUsage{InputTokens: 40, OutputTokens: 15},
		},
	)

	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {}

	text, _, err := RunToolLoop(context.Background(), provider, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":"research this"}`)},
		"You are helpful.",
		4096,
		emit,
		nil,)

	if err != nil {
		t.Fatalf("run tool loop: %v", err)
	}
	if text != "Based on my search and reading, here is the answer." {
		t.Errorf("text: got %q", text)
	}
	if provider.CallCount() != 3 {
		t.Errorf("call count: got %d, want 3", provider.CallCount())
	}
}

func TestRunToolLoopMaxIterations(t *testing.T) {
	// LLM keeps requesting tool_use, hitting the iteration limit.
	registry := NewToolRegistry()
	if err := registry.Register(Tool{
		Name: "loop_tool",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "result", nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	// Always return tool_use.
	provider := newMockToolLoopProvider(
		&provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{
				{ID: "call-loop", Name: "loop_tool", Arguments: json.RawMessage(`{}`)},
			},
			Usage: provideriface.TokenUsage{InputTokens: 10, OutputTokens: 5},
		},
	)

	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {}

	_, _, err := RunToolLoop(context.Background(), provider, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":"loop"}`)},
		"You are helpful.",
		4096,
		emit,
		nil,)

	if err == nil {
		t.Fatal("expected error for exceeding max iterations")
	}
	if !strings.Contains(err.Error(), "exceeded 200 iterations") {
		t.Fatalf("max-iteration error = %q, want 200-iteration ceiling", err.Error())
	}
	if provider.CallCount() != maxToolLoopIterations {
		t.Fatalf("provider calls = %d, want %d", provider.CallCount(), maxToolLoopIterations)
	}
}

func TestRunToolLoopBudgetLimitsProviderCalls(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.Register(Tool{
		Name: "loop_tool",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "result", nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	provider := newMockToolLoopProvider(
		&provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{
				{ID: "call-loop", Name: "loop_tool", Arguments: json.RawMessage(`{}`)},
			},
			Usage: provideriface.TokenUsage{InputTokens: 10, OutputTokens: 5},
		},
	)
	var budgetEvents int
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		if kind == types.EventRunProgress && phase == "tool_loop_budget" {
			budgetEvents++
			var fields map[string]any
			if err := json.Unmarshal(payload, &fields); err != nil {
				t.Fatalf("budget payload: %v", err)
			}
			if fields["provider_calls"] != float64(2) {
				t.Fatalf("provider_calls = %v, want 2", fields["provider_calls"])
			}
		}
	}

	_, usage, err := RunToolLoop(context.Background(), provider, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":"loop"}`)},
		"You are helpful.",
		4096,
		emit,
		nil,
		WithToolLoopBudget(ToolLoopBudget{Label: "test-budget", MaxProviderCalls: 2}),)
	if err == nil || !strings.Contains(err.Error(), `tool loop budget "test-budget" exhausted`) {
		t.Fatalf("error = %v, want budget exhaustion", err)
	}
	if provider.CallCount() != 2 {
		t.Fatalf("provider calls = %d, want 2", provider.CallCount())
	}
	if usage.InputTokens != 20 || usage.OutputTokens != 10 {
		t.Fatalf("usage = %+v, want two accumulated responses", usage)
	}
	if budgetEvents != 1 {
		t.Fatalf("budget events = %d, want 1", budgetEvents)
	}
}

func TestRunToolLoopBudgetCountsPriorProviderCalls(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.Register(Tool{
		Name: "loop_tool",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "result", nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	provider := newMockToolLoopProvider(
		&provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{
				{ID: "call-loop", Name: "loop_tool", Arguments: json.RawMessage(`{}`)},
			},
			Usage: provideriface.TokenUsage{InputTokens: 10, OutputTokens: 5},
		},
	)

	var usageEvents int
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		if kind == types.EventRunProgress && phase == "tool_loop_budget_usage" {
			usageEvents++
			var fields map[string]any
			if err := json.Unmarshal(payload, &fields); err != nil {
				t.Fatalf("usage payload: %v", err)
			}
			if fields["provider_calls"] != float64(3) {
				t.Fatalf("provider_calls = %v, want prior 2 + activation 1", fields["provider_calls"])
			}
			if fields["input_tokens"] != float64(110) || fields["output_tokens"] != float64(205) {
				t.Fatalf("tokens = %v/%v, want prior + activation", fields["input_tokens"], fields["output_tokens"])
			}
		}
	}

	_, _, err := RunToolLoop(context.Background(), provider, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":"loop"}`)},
		"You are helpful.",
		4096,
		emit,
		nil,
		WithToolLoopBudget(ToolLoopBudget{
			Label:              "rewarm-budget",
			SpentProviderCalls: 2,
			SpentInputTokens:   100,
			SpentOutputTokens:  200,
			MaxProviderCalls:   3,
		}),)
	if err == nil || !strings.Contains(err.Error(), `tool loop budget "rewarm-budget" exhausted`) {
		t.Fatalf("error = %v, want cumulative provider-call exhaustion", err)
	}
	if provider.CallCount() != 1 {
		t.Fatalf("provider calls = %d, want one activation call before cumulative exhaustion", provider.CallCount())
	}
	if usageEvents != 1 {
		t.Fatalf("usage events = %d, want 1", usageEvents)
	}
}

func TestRunToolLoopBudgetLimitsCumulativeTokens(t *testing.T) {
	provider := newMockToolLoopProvider(
		&provideriface.ToolLoopResponse{
			StopReason: "end_turn",
			Text:       "done",
			Usage:      provideriface.TokenUsage{InputTokens: 40, OutputTokens: 20},
			Model:      "test-model",
		},
	)
	var budgetEvents int
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		if kind == types.EventRunProgress && phase == "tool_loop_budget" {
			budgetEvents++
		}
	}

	text, usage, err := RunToolLoop(context.Background(), provider, nil, []json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`)},
		"You are helpful.",
		4096,
		emit,
		nil,
		WithToolLoopBudget(ToolLoopBudget{Label: "token-budget", MaxTotalTokens: 50}),)
	if err == nil || !strings.Contains(err.Error(), `total tokens 60 exceeded max 50`) {
		t.Fatalf("error = %v, want token budget exhaustion", err)
	}
	if text != "" {
		t.Fatalf("text = %q, want empty result on budget exhaustion", text)
	}
	if usage.InputTokens != 40 || usage.OutputTokens != 20 {
		t.Fatalf("usage = %+v, want accumulated response usage", usage)
	}
	if budgetEvents != 1 {
		t.Fatalf("budget events = %d, want 1", budgetEvents)
	}
}

func TestRunToolLoopParkWaiterBlocksWithoutProviderCallsUntilInjectedTurn(t *testing.T) {
	provider := newMockToolLoopProvider(
		&provideriface.ToolLoopResponse{
			StopReason: "end_turn",
			Text:       "idle for now",
			Usage:      provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model:      "test-model",
		},
		&provideriface.ToolLoopResponse{
			StopReason: "end_turn",
			Text:       "processed update",
			Usage:      provideriface.TokenUsage{InputTokens: 2, OutputTokens: 2},
			Model:      "test-model",
		},
	)
	parkEntered := make(chan struct{})
	releasePark := make(chan struct{})
	done := make(chan struct {
		text  string
		usage provideriface.TokenUsage
		err   error
	}, 1)
	var injectCount int32
	var updateReady int32
	var parkEnteredClosed int32
	var parkStarted, parkFinished int32
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		if kind != types.EventRunProgress {
			return
		}
		switch phase {
		case "park_wait_started":
			atomic.AddInt32(&parkStarted, 1)
		case "park_wait_finished":
			atomic.AddInt32(&parkFinished, 1)
		}
	}
	injector := func(finalCheckpoint bool) ([]json.RawMessage, error) {
		if atomic.LoadInt32(&updateReady) == 0 {
			return nil, nil
		}
		if atomic.AddInt32(&injectCount, 1) != 1 {
			return nil, nil
		}
		msg, _ := json.Marshal(map[string]any{
			"role": "user",
			"content": []map[string]string{{
				"type": "text",
				"text": "New durable update arrived.",
			}},
		})
		return []json.RawMessage{msg}, nil
	}
	waiter := func(ctx context.Context, state ToolLoopParkState) (ToolLoopParkResult, error) {
		if state.Attempts > 0 {
			return ToolLoopParkResult{Continue: false, Reason: "test_idle_deadline"}, nil
		}
		if atomic.CompareAndSwapInt32(&parkEnteredClosed, 0, 1) {
			close(parkEntered)
		}
		select {
		case <-ctx.Done():
			return ToolLoopParkResult{}, ctx.Err()
		case <-releasePark:
			return ToolLoopParkResult{Continue: true, Reason: "test_signal"}, nil
		}
	}

	go func() {
		text, usage, err := RunToolLoop(context.Background(), provider, nil, []json.RawMessage{json.RawMessage(`{"role":"user","content":"wait for updates"}`)},
			"You are helpful.",
			4096,
			emit,
			injector,
			WithParkWaiter(waiter),)
		done <- struct {
			text  string
			usage provideriface.TokenUsage
			err   error
		}{text: text, usage: usage, err: err}
	}()

	select {
	case <-parkEntered:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for park")
	}
	if provider.CallCount() != 1 {
		t.Fatalf("provider calls while parked = %d, want 1", provider.CallCount())
	}
	select {
	case result := <-done:
		t.Fatalf("tool loop completed while parked: text=%q err=%v", result.text, result.err)
	case <-time.After(30 * time.Millisecond):
	}
	atomic.StoreInt32(&updateReady, 1)
	close(releasePark)
	var result struct {
		text  string
		usage provideriface.TokenUsage
		err   error
	}
	select {
	case result = <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for parked loop to resume")
	}
	if result.err != nil {
		t.Fatalf("run tool loop: %v", result.err)
	}
	if result.text != "processed update" {
		t.Fatalf("text = %q, want processed update", result.text)
	}
	if provider.CallCount() != 2 {
		t.Fatalf("provider calls after signal = %d, want 2", provider.CallCount())
	}
	if result.usage.InputTokens != 3 || result.usage.OutputTokens != 3 {
		t.Fatalf("usage = %+v, want accumulated usage after resume", result.usage)
	}
	if atomic.LoadInt32(&parkStarted) != 2 || atomic.LoadInt32(&parkFinished) != 2 {
		t.Fatalf("park events started=%d finished=%d, want 2/2", parkStarted, parkFinished)
	}
}

func TestRunToolLoopContinuesAfterMaxTokensPartialText(t *testing.T) {
	provider := newMockToolLoopProvider(
		&provideriface.ToolLoopResponse{
			StopReason: "max_tokens",
			Text:       "partial...",
			Usage:      provideriface.TokenUsage{InputTokens: 10, OutputTokens: 4096},
			Model:      "test-model",
		},
		&provideriface.ToolLoopResponse{
			StopReason: "end_turn",
			Text:       "finished.",
			Usage:      provideriface.TokenUsage{InputTokens: 11, OutputTokens: 12},
			Model:      "test-model",
		},
	)

	var retryPhases []string
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		if kind == types.EventRunRetry {
			retryPhases = append(retryPhases, phase)
		}
	}

	text, usage, err := RunToolLoop(context.Background(), provider, nil, []json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`)},
		"You are helpful.",
		0,
		emit,
		nil,)

	if err != nil {
		t.Fatalf("run tool loop: %v", err)
	}
	if text != "partial...\nfinished." {
		t.Fatalf("text = %q, want partial plus continuation", text)
	}
	if usage.InputTokens != 21 || usage.OutputTokens != 4108 {
		t.Fatalf("usage = %+v, want accumulated usage", usage)
	}
	if provider.CallCount() != 2 {
		t.Fatalf("provider calls = %d, want 2", provider.CallCount())
	}
	if len(retryPhases) != 1 || retryPhases[0] != "max_tokens_continuation" {
		t.Fatalf("retry phases = %+v, want max_tokens_continuation", retryPhases)
	}
	if provider.lastReq.MaxTokens != 0 {
		t.Fatalf("continuation request max_tokens = %d, want omitted/0", provider.lastReq.MaxTokens)
	}
	if lastUser := extractLastUserMessage(provider.lastReq.Messages); !strings.Contains(lastUser, "Continue from that partial response") {
		t.Fatalf("last user message = %q, want continuation instruction", lastUser)
	}
}

func TestRunToolLoopMaxTokensWithoutTextStillFails(t *testing.T) {
	provider := newMockToolLoopProvider(
		&provideriface.ToolLoopResponse{
			StopReason: "max_tokens",
			Usage:      provideriface.TokenUsage{InputTokens: 10, OutputTokens: 4096},
			Model:      "test-model",
		},
	)

	_, _, err := RunToolLoop(context.Background(), provider, nil, []json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`)},
		"You are helpful.",
		0,
		func(kind types.EventKind, phase string, payload json.RawMessage) {},
		nil,)

	if err == nil || !strings.Contains(err.Error(), "max_tokens without text") {
		t.Fatalf("error = %v, want max_tokens without text", err)
	}
}

func TestRunToolLoopContextCancelled(t *testing.T) {
	// Use a provider that blocks until context is done.
	provider := &contextBlockingProvider{}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {}

	_, _, err := RunToolLoop(ctx, provider, nil, nil, "", 4096, emit, nil)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

// contextBlockingProvider is a provideriface.ToolLoopProvider that blocks until context
// cancellation, used for testing context-aware cancellation in the tool loop.
type contextBlockingProvider struct {
	provideriface.Provider // embed nil stub; ProviderName not used in this test
}

func (p *contextBlockingProvider) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestRunToolLoopToolUseWithoutCalls(t *testing.T) {
	// Provider returns tool_use stop reason but no tool calls.
	provider := newMockToolLoopProvider(
		&provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls:  nil, // missing tool calls!
			Usage:      provideriface.TokenUsage{},
		},
	)

	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {}

	_, _, err := RunToolLoop(context.Background(), provider, NewToolRegistry(), []json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`)},
		"You are helpful.",
		4096,
		emit,
		nil,)

	if err == nil {
		t.Fatal("expected error for tool_use without tool calls")
	}
}

// --- provideriface.ToolLoopProvider Adapter Tests ---

func TestToolLoopAdapter(t *testing.T) {
	// The toolLoopAdapter wraps a basic Provider to implement provideriface.ToolLoopProvider.
	stub := basicProvider{}
	adapter := &toolLoopAdapter{Provider: stub}

	req := provideriface.ToolLoopRequest{
		System:    "You are helpful.",
		Messages:  []json.RawMessage{json.RawMessage(`{"role":"user","content":[{"type":"text","text":"hello"}]}`)},
		MaxTokens: 4096,
	}

	resp, err := adapter.CallWithTools(context.Background(), req)
	if err != nil {
		t.Fatalf("call with tools: %v", err)
	}
	if resp.StopReason != "end_turn" {
		t.Errorf("stop reason: got %q, want end_turn", resp.StopReason)
	}
}

func TestAsToolLoopProvider(t *testing.T) {
	// When a provider already implements provideriface.ToolLoopProvider, it should be returned directly.
	provider := newMockToolLoopProvider(
		&provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "direct"},
	)

	result := AsToolLoopProvider(provider)
	if _, ok := result.(*mockToolLoopProvider); !ok {
		t.Error("expected direct cast to mockToolLoopProvider")
	}

	// When a provider doesn't implement provideriface.ToolLoopProvider, it should be wrapped.
	stub := basicProvider{}
	result = AsToolLoopProvider(stub)
	if _, ok := result.(*toolLoopAdapter); !ok {
		t.Error("expected toolLoopAdapter wrapper for stub provider")
	}
}
// --- Helper content builders ---

func TestBuildAssistantContent(t *testing.T) {
	calls := []types.ToolCall{
		{ID: "call-1", Name: "test", Arguments: json.RawMessage(`{"key":"val"}`)},
	}

	content := buildAssistantContent("Some text", calls)
	if len(content) != 2 {
		t.Fatalf("content blocks: got %d, want 2", len(content))
	}

	// First block should be text.
	textBlock, ok := content[0].(map[string]string)
	if !ok {
		t.Fatalf("first block: expected map[string]string")
	}
	if textBlock["type"] != "text" {
		t.Errorf("first block type: got %q, want text", textBlock["type"])
	}

	// Second block should be tool_use.
	toolBlock, ok := content[1].(map[string]any)
	if !ok {
		t.Fatalf("second block: expected map[string]any")
	}
	if toolBlock["type"] != "tool_use" {
		t.Errorf("second block type: got %v, want tool_use", toolBlock["type"])
	}
}

func TestBuildToolResultContent(t *testing.T) {
	results := []types.ToolResult{
		{CallID: "call-1", Output: "result text", IsError: false},
		{CallID: "call-2", Output: "error text", IsError: true},
	}

	content := buildToolResultContent(results)
	if len(content) != 2 {
		t.Fatalf("content blocks: got %d, want 2", len(content))
	}

	// First result: normal.
	block1, ok := content[0].(map[string]any)
	if !ok {
		t.Fatalf("first block: expected map[string]any")
	}
	if block1["tool_use_id"] != "call-1" {
		t.Errorf("first block tool_use_id: got %v, want call-1", block1["tool_use_id"])
	}

	// Second result: error.
	block2, ok := content[1].(map[string]any)
	if !ok {
		t.Fatalf("second block: expected map[string]any")
	}
	if block2["is_error"] != true {
		t.Errorf("second block is_error: got %v, want true", block2["is_error"])
	}
}

func intMapValue(m map[string]any, key string) int {
	switch value := m[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func isContextOverflowError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "context") &&
		(strings.Contains(text, "overflow") ||
			strings.Contains(text, "too long") ||
			strings.Contains(text, "exceed") ||
			strings.Contains(text, "length") ||
			strings.Contains(text, "window"))
}


func TestExecuteToolBatchPreservesExecutionContext(t *testing.T) {
	type contextKey string
	const key contextKey = "caller-context"
	registry := NewToolRegistry()
	if err := registry.Register(Tool{
		Name: "read_context",
		Func: func(ctx context.Context, _ json.RawMessage) (string, error) {
			value, _ := ctx.Value(key).(string)
			return value, nil
		},
	}); err != nil {
		t.Fatalf("register read_context: %v", err)
	}

	results := ExecuteToolBatch(
		context.WithValue(context.Background(), key, "preserved"),
		registry,
		[]types.ToolCall{{ID: "call-context", Name: "read_context"}},
		func(types.EventKind, string, json.RawMessage) {},
	)
	if len(results) != 1 || results[0].IsError || results[0].Output != "preserved" {
		t.Fatalf("results = %+v, want preserved caller context", results)
	}
}
