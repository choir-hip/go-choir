package runtime

import (
	"bytes"
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
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
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

func TestRunToolLoopExactInitialToolChoiceAcceptsDuplicateSameTool(t *testing.T) {
	registry := toolregistry.NewToolRegistry()
	var edited int
	if err := registry.Register(toolregistry.Tool{
		Name:        "patch_texture",
		Description: "Edit the Texture document.",
		Parameters:  map[string]any{"type": "object"},
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			edited++
			return `{"status":"stored","revision_id":"rev-2"}`, nil
		},
	}); err != nil {
		t.Fatalf("register patch_texture: %v", err)
	}

	var choices []string
	provider := &capturingToolChoiceProvider{responses: []*provideriface.ToolLoopResponse{{
		StopReason: "tool_use",
		ToolCalls: []types.ToolCall{
			{ID: "call-edit-1", Name: "patch_texture", Arguments: json.RawMessage(`{"doc_id":"doc-1","content":"first"}`)},
			{ID: "call-edit-2", Name: "patch_texture", Arguments: json.RawMessage(`{"doc_id":"doc-1","content":"second"}`)},
		},
		Usage: provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
		Model: "test-model",
	}}, choices: &choices}

	var retrySeen bool
	var duplicateNoticeSeen bool
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		if kind == types.EventRunRetry && phase == "initial_tool_choice" {
			retrySeen = true
		}
		if kind != types.EventToolResult {
			return
		}
		var decoded map[string]any
		if err := json.Unmarshal(payload, &decoded); err != nil {
			t.Fatalf("decode tool result: %v", err)
		}
		if decoded["call_id"] == "call-edit-2" && strings.Contains(fmt.Sprint(decoded["output"]), "duplicate Texture write tool patch_texture") {
			duplicateNoticeSeen = true
		}
	}

	run := &types.RunRecord{
		RunID:        "run-texture",
		OwnerID:      "owner-1",
		AgentProfile: agentprofile.Texture,
		AgentRole:    agentprofile.Texture,
	}
	text, _, err := toolregistry.RunToolLoop(toolregistry.WithExecutionContext(context.Background(), toolExecutionContextForRun(run)), provider, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":"write v1"}`)},
		"You are a Texture appagent.",
		0,
		emit,
		nil,
		toolregistry.WithInitialToolChoice("function:patch_texture"),
		toolregistry.WithTerminalToolSuccesses("patch_texture"))
	if err != nil {
		t.Fatalf("run tool loop: %v", err)
	}
	if text != "" {
		t.Fatalf("text = %q, want empty terminal tool result", text)
	}
	if edited != 1 {
		t.Fatalf("patch_texture executed %d times, want 1", edited)
	}
	if retrySeen {
		t.Fatal("same-tool duplicate response must not trigger initial tool-choice retry")
	}
	if !duplicateNoticeSeen {
		t.Fatal("missing duplicate patch_texture notice for second call")
	}
	if len(choices) != 1 || choices[0] != "function:patch_texture" {
		t.Fatalf("tool choices = %#v, want one exact initial patch_texture choice", choices)
	}
}

// --- Integration: Runtime with Tool Registry ---

func TestRuntimeWithToolRegistryUsesToolLoop(t *testing.T) {
	// When a tool registry is configured, the runtime should use the
	// tool-calling loop path instead of the simple Provider.Execute path.
	registry := toolregistry.NewToolRegistry()
	if err := registry.Register(toolregistry.Tool{
		Name: "test_tool",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "tool result", nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	// Create a provider that supports provideriface.ToolLoopProvider.
	provider := newMockToolLoopProvider(
		&provideriface.ToolLoopResponse{
			StopReason: "end_turn",
			Text:       "Final answer from tool loop",
			Usage:      provideriface.TokenUsage{InputTokens: 10, OutputTokens: 5},
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
	registry := toolregistry.NewToolRegistry()
	if err := registry.Register(toolregistry.Tool{
		Name: "read_file",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "file contents here", nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	provider := newMockToolLoopProvider(
		// First: tool use.
		&provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{
				{ID: "call-1", Name: "read_file", Arguments: json.RawMessage(`{"path":"/tmp/test.txt"}`)},
			},
			Usage: provideriface.TokenUsage{InputTokens: 15, OutputTokens: 10},
		},
		// Second: final answer.
		&provideriface.ToolLoopResponse{
			StopReason: "end_turn",
			Text:       "The file contains: file contents here",
			Usage:      provideriface.TokenUsage{InputTokens: 25, OutputTokens: 5},
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

	// Also check persisted events. Poll since the bus events may arrive
	// before the store has persisted them (common under -race).
	waitForEvents(t, rt.Store(), rec.RunID, []types.EventKind{
		types.EventToolInvoked,
		types.EventToolResult,
	}, 3*time.Second)
}

// --- testRuntimeWithProviderAndRegistry ---

func testRuntimeWithProviderAndRegistry(t *testing.T, provider provideriface.Provider, registry *toolregistry.ToolRegistry) (*Runtime, *store.Store) {
	t.Helper()

	dir := filepath.Join(os.TempDir(), "go-choir-m3-toolloop-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, t.Name()+".db")
	_ = os.Remove(dbPath)

	s, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	bus := events.NewEventBus()
	cfg := provideriface.Config{
		SandboxID:           "sandbox-test",
		StorePath:           dbPath,
		ProviderTimeout:     5 * time.Second,
		SupervisionInterval: 1 * time.Hour,
	}

	rt := New(cfg, s, bus, provider)
	rt.toolRegistry = registry
	setTestDispatch(rt, s)

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

func rawMessagesForTest(messages []json.RawMessage) string {
	parts := make([][]byte, 0, len(messages))
	for _, msg := range messages {
		parts = append(parts, []byte(msg))
	}
	return string(bytes.Join(parts, []byte("\n")))
}

func extractLastUserMessage(messages []json.RawMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		var msg struct {
			Role    string `json:"role"`
			Content any    `json:"content"`
		}
		if err := json.Unmarshal(messages[i], &msg); err != nil {
			continue
		}
		if msg.Role == "user" {
			return extractTextFromContent(msg.Content)
		}
	}
	return ""
}

func extractTextFromContent(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		var text string
		for _, item := range v {
			if block, ok := item.(map[string]any); ok {
				if blockType, _ := block["type"].(string); blockType == "text" {
					if value, _ := block["text"].(string); value != "" {
						text += value
					}
				}
			}
		}
		return text
	default:
		return ""
	}
}
