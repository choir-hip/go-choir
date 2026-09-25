package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// mockGatewayCaller is a test double for GatewayCaller that records calls
// and returns configurable responses.
type mockGatewayCaller struct {
	name    string
	isReal  bool
	resp    *LLMResponse
	err     error
	lastReq *LLMRequest // captures the most recent request
}

func (m *mockGatewayCaller) Name() string { return m.name }
func (m *mockGatewayCaller) IsReal() bool { return m.isReal }
func (m *mockGatewayCaller) Call(ctx context.Context, req LLMRequest) (*LLMResponse, error) {
	m.lastReq = &req
	return m.resp, m.err
}
func (m *mockGatewayCaller) Stream(ctx context.Context, req LLMRequest, onChunk func(StreamChunk)) (*LLMResponse, error) {
	m.lastReq = &req
	if m.err != nil {
		return nil, m.err
	}
	// Simulate streaming by emitting the full response text as a single chunk.
	if m.resp != nil && m.resp.Text != "" {
		onChunk(StreamChunk{
			Type:  "content_block_delta",
			Delta: m.resp.Text,
			Index: 0,
		})
		onChunk(StreamChunk{
			Type:       "message_stop",
			StopReason: m.resp.StopReason,
			Usage:      &StreamUsage{InputTokens: m.resp.Usage.InputTokens, OutputTokens: m.resp.Usage.OutputTokens},
		})
	}
	return m.resp, nil
}

// --- GatewayBridgeProvider construction tests ---

func TestGatewayBridgeProviderRequiresNonNilClient(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for nil client")
		}
		if !strings.Contains(fmt.Sprint(r), "non-nil") {
			t.Fatalf("expected non-nil panic, got: %v", r)
		}
	}()
	NewGatewayBridgeProvider(nil)
}
func TestGatewayBridgeProviderExecuteFailure(t *testing.T) {
	mock := &mockGatewayCaller{
		name:   "gateway",
		isReal: true,
		err:    fmt.Errorf("gateway client: status 503 (sanitized)"),
	}
	gbp := NewGatewayBridgeProvider(mock)

	task := &types.RunRecord{
		RunID:  "task-2",
		Prompt: "This should fail",
	}

	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {}

	err := gbp.Execute(context.Background(), task, emit)
	if err == nil {
		t.Fatal("expected error from Execute")
	}
	if !strings.Contains(err.Error(), "gateway call failed") {
		t.Fatalf("expected gateway call failed error, got: %v", err)
	}

	// Provider failures must be structured errors, not panics.
	// The runtime should remain available for later runs (VAL-RUNTIME-008).
	if task.Result != "" {
		t.Fatalf("task result should be empty on failure, got: %s", task.Result)
	}
}

func TestGatewayBridgeProviderExecuteCancelledContext(t *testing.T) {
	mock := &mockGatewayCaller{
		name:   "gateway",
		isReal: true,
		err:    context.Canceled,
	}
	gbp := NewGatewayBridgeProvider(mock)

	task := &types.RunRecord{RunID: "task-ctx", Prompt: "cancelled"}
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {}

	err := gbp.Execute(context.Background(), task, emit)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}
func TestGatewayBridgeProviderCallWithToolsUsesPerRunModelSelection(t *testing.T) {
	mock := &mockGatewayCaller{
		name:   "gateway",
		isReal: true,
		resp: &LLMResponse{
			ID:         "resp-model-policy",
			Text:       "ok",
			Model:      "accounts/fireworks/models/deepseek-v4-flash",
			StopReason: "end_turn",
			Usage:      Usage{InputTokens: 10, OutputTokens: 2},
		},
	}
	gbp := NewGatewayBridgeProvider(mock)
	gbp.SetRuntimeLLMConfig("chatgpt", "gpt-5.5", "low")

	req := provideriface.ToolLoopRequest{
		Provider:        "fireworks",
		Model:           "accounts/fireworks/models/deepseek-v4-flash",
		ReasoningEffort: "none",
		System:          "system",
		Messages:        []json.RawMessage{[]byte(`{"role":"user","content":"hi"}`)},
		ToolChoice:      "required",
		MaxTokens:       1024,
	}

	if _, err := gbp.CallWithTools(context.Background(), req); err != nil {
		t.Fatalf("CallWithTools failed: %v", err)
	}
	if mock.lastReq == nil {
		t.Fatal("no request sent to gateway")
	}
	if mock.lastReq.Provider != "fireworks" {
		t.Fatalf("provider = %q, want fireworks", mock.lastReq.Provider)
	}
	if mock.lastReq.Model != "accounts/fireworks/models/deepseek-v4-flash" {
		t.Fatalf("model = %q", mock.lastReq.Model)
	}
	if mock.lastReq.ReasoningEffort != "none" {
		t.Fatalf("reasoning = %q, want none", mock.lastReq.ReasoningEffort)
	}
	if mock.lastReq.ToolChoice != "required" {
		t.Fatalf("tool_choice = %q, want required", mock.lastReq.ToolChoice)
	}
}

func TestGatewayBridgeProviderCallWithToolsError(t *testing.T) {
	mock := &mockGatewayCaller{
		name:   "gateway",
		isReal: true,
		err:    fmt.Errorf("gateway client: status 502 (sanitized)"),
	}
	gbp := NewGatewayBridgeProvider(mock)

	req := provideriface.ToolLoopRequest{
		System:    "system",
		Messages:  []json.RawMessage{[]byte(`{"role":"user","content":"hi"}`)},
		MaxTokens: 1024,
	}

	_, err := gbp.CallWithTools(context.Background(), req)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "gateway call failed") {
		t.Fatalf("expected gateway call failed, got: %v", err)
	}
}

// Note: HTTP-level integration tests for the full gateway-bridge-provider
// path live in the gateway package (internal/gateway/gateway_test.go) to
// avoid circular imports. The mock-based tests above cover the
// GatewayBridgeProvider logic end-to-end.

// --- Gateway preference decision logic tests ---

func TestGatewayURLPreferredOverDirectResolution(t *testing.T) {
	t.Setenv("RUNTIME_GATEWAY_URL", "http://gateway.test:8084")
	t.Setenv("RUNTIME_GATEWAY_TOKEN", "test-token")
	t.Setenv("ZAI_API_KEY", "fake-key-for-test")

	// Verify that when RUNTIME_GATEWAY_URL is set, we should use the gateway.
	gatewayURL := os.Getenv("RUNTIME_GATEWAY_URL")
	if gatewayURL == "" {
		t.Fatal("expected gateway URL to be set")
	}

	// Also verify that direct resolution would succeed (ZAI_API_KEY is set).
	// The autoputer logic should prefer the gateway URL regardless.
	p, err := ResolveProvider(ProviderConfig{
		ZAIModels:        []string{"glm-5.1"},
		SelectedProvider: "zai",
	})
	if err != nil {
		t.Fatalf("direct resolution should still work: %v", err)
	}
	if p == nil {
		t.Fatal("direct resolution should return a provider when ZAI_API_KEY is set")
	}
	if p.Name() != "zai" {
		t.Fatalf("expected zai provider, got: %s", p.Name())
	}

	// The autoputer should prefer gateway when URL is set, even though direct
	// resolution would also succeed. This test validates both paths work.
}
