package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// --- Mock ToolLoopProvider ---

// mockToolLoopProvider implements ToolLoopProvider for testing the
// tool-calling loop. It simulates LLM responses with configurable
// stop reasons and tool calls.
type mockToolLoopProvider struct {
	// Provider is the base Provider interface (for ProviderName etc).
	Provider

	// responses is a sequence of responses to return from CallWithTools.
	// Each response is consumed in order; if exhausted, the last response
	// is reused.
	responses []*ToolLoopResponse

	// callCount tracks how many times CallWithTools was invoked.
	callCount int32

	lastReq ToolLoopRequest
}

func (m *mockToolLoopProvider) CallWithTools(ctx context.Context, req ToolLoopRequest) (*ToolLoopResponse, error) {
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
func newMockToolLoopProvider(responses ...*ToolLoopResponse) *mockToolLoopProvider {
	return &mockToolLoopProvider{
		responses: responses,
	}
}

type capturingToolChoiceProvider struct {
	Provider
	responses []*ToolLoopResponse
	choices   *[]string
	maxTokens *[]int
	callCount int
}

func (p *capturingToolChoiceProvider) CallWithTools(ctx context.Context, req ToolLoopRequest) (*ToolLoopResponse, error) {
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

// --- Tool-Calling Loop Tests ---

func TestRunToolLoopEndTurn(t *testing.T) {
	// Simple case: LLM returns end_turn immediately.
	provider := newMockToolLoopProvider(
		&ToolLoopResponse{
			StopReason: "end_turn",
			Text:       "Hello! How can I help?",
			Usage:      TokenUsage{InputTokens: 10, OutputTokens: 20},
			Model:      "test-model",
		},
	)

	var emittedEvents []types.EventKind
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		emittedEvents = append(emittedEvents, kind)
	}

	text, usage, err := RunToolLoop(
		context.Background(),
		provider,
		nil, // no tool registry
		[]json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`)},
		"You are helpful.",
		4096,
		emit,
		nil,
	)

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

func TestRunToolLoopEmitsProviderCallProgressBeforeCall(t *testing.T) {
	provider := newMockToolLoopProvider(&ToolLoopResponse{
		StopReason: "end_turn",
		Text:       "done",
		Usage:      TokenUsage{InputTokens: 1, OutputTokens: 1},
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

	_, _, err := RunToolLoop(
		context.Background(),
		provider,
		nil,
		[]json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`)},
		"You are helpful.",
		0,
		emit,
		nil,
		WithToolLoopLLMConfig(LLMSelection{Provider: "fireworks", Model: "accounts/fireworks/models/deepseek-v4-flash", ReasoningEffort: "none"}),
	)
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
	provider := &capturingToolChoiceProvider{responses: []*ToolLoopResponse{
		{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:        "call-1",
				Name:      "record_status",
				Arguments: json.RawMessage(`{"status":"ok"}`),
			}},
			Usage: TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model: "test-model",
		},
		{
			StopReason: "end_turn",
			Text:       "done",
			Usage:      TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model:      "test-model",
		},
	}, choices: &choices}

	text, _, err := RunToolLoop(
		context.Background(),
		provider,
		registry,
		[]json.RawMessage{json.RawMessage(`{"role":"user","content":"record"}`)},
		"You are helpful.",
		0,
		func(types.EventKind, string, json.RawMessage) {},
		nil,
		WithInitialToolChoice("required"),
	)
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

func TestRunToolLoopRequiredToolTurnIsBoundedAndRetriesMissingTool(t *testing.T) {
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
	provider := &capturingToolChoiceProvider{responses: []*ToolLoopResponse{
		{
			StopReason: "max_tokens",
			Text:       strings.Repeat("runaway prose ", 100),
			Usage:      TokenUsage{InputTokens: 1, OutputTokens: requiredToolTurnMaxTokens},
			Model:      "test-model",
		},
		{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:        "call-1",
				Name:      "record_status",
				Arguments: json.RawMessage(`{"status":"ok"}`),
			}},
			Usage: TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model: "test-model",
		},
		{
			StopReason: "end_turn",
			Text:       "done",
			Usage:      TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model:      "test-model",
		},
	}, choices: &choices, maxTokens: &maxTokens}

	var retrySeen bool
	text, _, err := RunToolLoop(
		context.Background(),
		provider,
		registry,
		[]json.RawMessage{json.RawMessage(`{"role":"user","content":"record"}`)},
		"You are helpful.",
		131072,
		func(kind types.EventKind, phase string, payload json.RawMessage) {
			if kind == types.EventRunRetry && phase == "required_tool_call" {
				retrySeen = true
			}
		},
		nil,
		WithInitialToolChoice("required"),
	)
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
	if len(maxTokens) != 3 || maxTokens[0] != requiredToolTurnMaxTokens || maxTokens[1] != requiredToolTurnMaxTokens || maxTokens[2] != 131072 {
		t.Fatalf("maxTokens = %#v, want bounded required turns then full content budget", maxTokens)
	}
}

func TestRunToolLoopMemoryHookPersistsFinalAssistant(t *testing.T) {
	provider := newMockToolLoopProvider(&ToolLoopResponse{
		StopReason: "end_turn",
		Text:       "done",
		Usage:      TokenUsage{InputTokens: 1, OutputTokens: 1},
		Model:      "test-model",
	})

	var appended []string
	hooks := ToolLoopMemoryHooks{
		AfterAppendMessage: func(ctx context.Context, role string, msg json.RawMessage) error {
			appended = append(appended, role+":"+string(msg))
			return nil
		},
	}

	text, _, err := RunToolLoop(
		context.Background(),
		provider,
		nil,
		[]json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`)},
		"You are helpful.",
		4096,
		func(types.EventKind, string, json.RawMessage) {},
		nil,
		WithToolLoopMemoryHooks(hooks),
	)
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
	Provider
	calls int32
}

func (p *overflowThenSuccessProvider) CallWithTools(ctx context.Context, req ToolLoopRequest) (*ToolLoopResponse, error) {
	call := atomic.AddInt32(&p.calls, 1)
	if call == 1 {
		return nil, fmt.Errorf("maximum context length exceeded")
	}
	return &ToolLoopResponse{
		StopReason: "end_turn",
		Text:       fmt.Sprintf("recovered with %d messages", len(req.Messages)),
		Usage:      TokenUsage{InputTokens: 2, OutputTokens: 3},
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

	text, usage, err := RunToolLoop(
		context.Background(),
		provider,
		nil,
		[]json.RawMessage{json.RawMessage(`{"role":"user","content":"very long"}`)},
		"You are helpful.",
		4096,
		func(types.EventKind, string, json.RawMessage) {},
		nil,
		WithToolLoopMemoryHooks(hooks),
	)
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
	Provider
	calls int32
}

func (p *rateLimitThenSuccessProvider) CallWithTools(ctx context.Context, req ToolLoopRequest) (*ToolLoopResponse, error) {
	call := atomic.AddInt32(&p.calls, 1)
	if call == 1 {
		return nil, fmt.Errorf("gateway call failed: chatgpt: status 429 Too Many Requests (sanitized)")
	}
	return &ToolLoopResponse{
		StopReason: "end_turn",
		Text:       "recovered after rate limit",
		Usage:      TokenUsage{InputTokens: 4, OutputTokens: 5},
		Model:      "test-model",
	}, nil
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

	text, usage, err := RunToolLoop(
		context.Background(),
		provider,
		nil,
		[]json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`)},
		"You are helpful.",
		4096,
		emit,
		nil,
	)
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
		&ToolLoopResponse{
			StopReason: "tool_use",
			Text:       "",
			ToolCalls: []types.ToolCall{
				{ID: "call-1", Name: "calculator", Arguments: json.RawMessage(`{"expr":"2+2"}`)},
			},
			Usage: TokenUsage{InputTokens: 15, OutputTokens: 10},
			Model: "test-model",
		},
		// Second response: final answer after tool result.
		&ToolLoopResponse{
			StopReason: "end_turn",
			Text:       "The answer is 42.",
			Usage:      TokenUsage{InputTokens: 25, OutputTokens: 5},
			Model:      "test-model",
		},
	)

	var emittedEvents []types.EventKind
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		emittedEvents = append(emittedEvents, kind)
	}

	text, usage, err := RunToolLoop(
		context.Background(),
		provider,
		registry,
		[]json.RawMessage{json.RawMessage(`{"role":"user","content":"calculate 2+2"}`)},
		"You are helpful.",
		4096,
		emit,
		nil,
	)

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
		&ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{
				{ID: "call-1", Name: "search", Arguments: json.RawMessage(`{"query":"test"}`)},
			},
			Usage: TokenUsage{InputTokens: 20, OutputTokens: 10},
		},
		// Second response: read.
		&ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{
				{ID: "call-2", Name: "read", Arguments: json.RawMessage(`{"path":"/tmp/test"}`)},
			},
			Usage: TokenUsage{InputTokens: 30, OutputTokens: 10},
		},
		// Third response: final answer.
		&ToolLoopResponse{
			StopReason: "end_turn",
			Text:       "Based on my search and reading, here is the answer.",
			Usage:      TokenUsage{InputTokens: 40, OutputTokens: 15},
		},
	)

	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {}

	text, _, err := RunToolLoop(
		context.Background(),
		provider,
		registry,
		[]json.RawMessage{json.RawMessage(`{"role":"user","content":"research this"}`)},
		"You are helpful.",
		4096,
		emit,
		nil,
	)

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
		&ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{
				{ID: "call-loop", Name: "loop_tool", Arguments: json.RawMessage(`{}`)},
			},
			Usage: TokenUsage{InputTokens: 10, OutputTokens: 5},
		},
	)

	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {}

	_, _, err := RunToolLoop(
		context.Background(),
		provider,
		registry,
		[]json.RawMessage{json.RawMessage(`{"role":"user","content":"loop"}`)},
		"You are helpful.",
		4096,
		emit,
		nil,
	)

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

func TestRunToolLoopMaxTokens(t *testing.T) {
	provider := newMockToolLoopProvider(
		&ToolLoopResponse{
			StopReason: "max_tokens",
			Text:       "partial...",
			Usage:      TokenUsage{InputTokens: 10, OutputTokens: 4096},
		},
	)

	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {}

	_, _, err := RunToolLoop(
		context.Background(),
		provider,
		nil,
		[]json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`)},
		"You are helpful.",
		4096,
		emit,
		nil,
	)

	if err == nil {
		t.Fatal("expected error for max_tokens stop reason")
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

// contextBlockingProvider is a ToolLoopProvider that blocks until context
// cancellation, used for testing context-aware cancellation in the tool loop.
type contextBlockingProvider struct {
	Provider // embed nil stub; ProviderName not used in this test
}

func (p *contextBlockingProvider) CallWithTools(ctx context.Context, req ToolLoopRequest) (*ToolLoopResponse, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestRunToolLoopToolUseWithoutCalls(t *testing.T) {
	// Provider returns tool_use stop reason but no tool calls.
	provider := newMockToolLoopProvider(
		&ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls:  nil, // missing tool calls!
			Usage:      TokenUsage{},
		},
	)

	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {}

	_, _, err := RunToolLoop(
		context.Background(),
		provider,
		NewToolRegistry(),
		[]json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`)},
		"You are helpful.",
		4096,
		emit,
		nil,
	)

	if err == nil {
		t.Fatal("expected error for tool_use without tool calls")
	}
}

// --- ToolLoopProvider Adapter Tests ---

func TestToolLoopAdapter(t *testing.T) {
	// The toolLoopAdapter wraps a basic Provider to implement ToolLoopProvider.
	stub := NewStubProvider(10 * time.Millisecond)
	adapter := &toolLoopAdapter{Provider: stub}

	req := ToolLoopRequest{
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
	// When a provider already implements ToolLoopProvider, it should be returned directly.
	provider := newMockToolLoopProvider(
		&ToolLoopResponse{StopReason: "end_turn", Text: "direct"},
	)

	result := asToolLoopProvider(provider)
	if _, ok := result.(*mockToolLoopProvider); !ok {
		t.Error("expected direct cast to mockToolLoopProvider")
	}

	// When a provider doesn't implement ToolLoopProvider, it should be wrapped.
	stub := NewStubProvider(10 * time.Millisecond)
	result = asToolLoopProvider(stub)
	if _, ok := result.(*toolLoopAdapter); !ok {
		t.Error("expected toolLoopAdapter wrapper for stub provider")
	}
}

// --- Integration: Runtime with Tool Registry ---

func TestRuntimeWithToolRegistryUsesToolLoop(t *testing.T) {
	// When a tool registry is configured, the runtime should use the
	// tool-calling loop path instead of the simple Provider.Execute path.
	registry := NewToolRegistry()
	if err := registry.Register(Tool{
		Name: "test_tool",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "tool result", nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	// Create a provider that supports ToolLoopProvider.
	provider := newMockToolLoopProvider(
		&ToolLoopResponse{
			StopReason: "end_turn",
			Text:       "Final answer from tool loop",
			Usage:      TokenUsage{InputTokens: 10, OutputTokens: 5},
		},
	)

	rt, store := testRuntimeWithProviderAndRegistry(t, provider, registry)
	defer rt.Stop()

	rec, err := rt.StartRun(context.Background(), "test prompt", "user-alice")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	// Check task completed with tool-loop result.
	fetched := waitForStoredRunTerminalState(t, store, rec.RunID, 5*time.Second)
	if fetched.State != types.RunCompleted {
		t.Errorf("state: got %q, want completed", fetched.State)
	}
	if fetched.Result != "Final answer from tool loop" {
		t.Errorf("result: got %q, want Final answer from tool loop", fetched.Result)
	}

	// Token usage should be stored in metadata.
	if fetched.Metadata == nil {
		t.Error("metadata should not be nil")
	} else {
		if _, ok := fetched.Metadata["input_tokens"]; !ok {
			t.Error("metadata should contain input_tokens")
		}
		if _, ok := fetched.Metadata["output_tokens"]; !ok {
			t.Error("metadata should contain output_tokens")
		}
	}
}

func TestRuntimeWithToolRegistryEmitsToolEvents(t *testing.T) {
	// Runtime with tool registry should emit tool.invoked and tool.result
	// events when tools are used.
	registry := NewToolRegistry()
	if err := registry.Register(Tool{
		Name: "read_file",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "file contents here", nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	provider := newMockToolLoopProvider(
		// First: tool use.
		&ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{
				{ID: "call-1", Name: "read_file", Arguments: json.RawMessage(`{"path":"/tmp/test.txt"}`)},
			},
			Usage: TokenUsage{InputTokens: 15, OutputTokens: 10},
		},
		// Second: final answer.
		&ToolLoopResponse{
			StopReason: "end_turn",
			Text:       "The file contains: file contents here",
			Usage:      TokenUsage{InputTokens: 25, OutputTokens: 5},
		},
	)

	rt, _ := testRuntimeWithProviderAndRegistry(t, provider, registry)
	defer rt.Stop()

	// Subscribe to events.
	ch := rt.EventBus().SubscribeWithBuffer(256)
	defer rt.EventBus().Unsubscribe(ch)

	rec, err := rt.StartRun(context.Background(), "read the test file", "user-alice")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	// Wait for completion.
	time.Sleep(200 * time.Millisecond)

	// Collect events from the bus.
	var invokedFound, resultFound bool
	timeout := time.After(2 * time.Second)
	for !invokedFound || !resultFound {
		select {
		case ev := <-ch:
			if ev.Record.RunID != rec.RunID {
				continue
			}
			if ev.Record.Kind == types.EventToolInvoked {
				invokedFound = true
			}
			if ev.Record.Kind == types.EventToolResult {
				resultFound = true
			}
		case <-timeout:
			t.Fatalf("timed out waiting for tool events (invoked=%v result=%v)", invokedFound, resultFound)
		}
	}

	// Also check persisted events.
	events, err := rt.Store().ListEvents(context.Background(), rec.RunID, 100)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}

	var persistedInvoked, persistedResult bool
	for _, ev := range events {
		if ev.Kind == types.EventToolInvoked {
			persistedInvoked = true
		}
		if ev.Kind == types.EventToolResult {
			persistedResult = true
		}
	}
	if !persistedInvoked {
		t.Error("expected persisted tool.invoked event")
	}
	if !persistedResult {
		t.Error("expected persisted tool.result event")
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

// --- testRuntimeWithProviderAndRegistry ---

func testRuntimeWithProviderAndRegistry(t *testing.T, provider Provider, registry *ToolRegistry) (*Runtime, *store.Store) {
	t.Helper()

	dir := filepath.Join(os.TempDir(), "go-choir-m3-toolloop-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, t.Name()+".db")
	_ = os.Remove(dbPath)

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	bus := events.NewEventBus()
	cfg := Config{
		SandboxID:           "sandbox-test",
		StorePath:           dbPath,
		ProviderTimeout:     5 * time.Second,
		SupervisionInterval: 1 * time.Hour,
	}

	rt := New(cfg, s, bus, provider, WithToolRegistry(registry))

	t.Cleanup(func() {
		rt.Stop()
		_ = s.Close()
		_ = os.Remove(dbPath)
	})

	return rt, s
}

func waitForStoredRunTerminalState(t *testing.T, s *store.Store, runID string, timeout time.Duration) types.RunRecord {
	t.Helper()

	ctx := context.Background()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		rec, err := s.GetRun(ctx, runID)
		if err != nil {
			t.Fatalf("get task: %v", err)
		}
		if rec.State.Terminal() {
			return rec
		}
		time.Sleep(25 * time.Millisecond)
	}

	rec, err := s.GetRun(ctx, runID)
	if err != nil {
		t.Fatalf("get task after timeout: %v", err)
	}
	t.Fatalf("timeout waiting for task %s (state=%s)", runID[:8], rec.State)
	return types.RunRecord{}
}
