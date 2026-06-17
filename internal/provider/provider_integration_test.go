//go:build integration

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/runtime"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// TestIntegrationAllModelsLive calls each supported model with real
// credentials from the environment. It is skipped unless the env var
// RUN_INTEGRATION_TESTS is set.
func TestIntegrationAllModelsLive(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") == "" {
		t.Skip("skipping integration test (set RUN_INTEGRATION_TESTS=1)")
	}

	cfg := ProviderConfig{
		BedrockModels: []string{
			"us.anthropic.claude-haiku-4-5-20251001-v1:0",
			"us.anthropic.claude-sonnet-4-6",
			"us.anthropic.claude-opus-4-6-v1",
		},
		ZAIModels: []string{"glm-5.2", "glm-5.1", "glm-5-turbo"},
		FireworksModels: []string{
			"accounts/fireworks/models/deepseek-v4-flash",
			"accounts/fireworks/models/deepseek-v4-pro",
			"accounts/fireworks/models/kimi-k2p6",
		},
	}

	mp := ResolveAll(cfg)
	names := mp.Names()
	if len(names) == 0 {
		t.Fatal("expected at least one provider to resolve from env credentials")
	}
	t.Logf("resolved providers: %v", names)

	req := LLMRequest{
		System:    "Respond with exactly: hello",
		Messages:  []Message{{Role: "user", Content: []Block{{Type: "text", Text: "Say hello"}}}},
		MaxTokens: 64,
	}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			p := mp.Get(name)
			if p == nil {
				t.Fatalf("provider %q not found", name)
			}

			resp, err := p.Call(context.Background(), req)
			if err != nil {
				// For debugging: make a raw request to see the full error.
				t.Fatalf("call failed: %v", err)
			}

			if resp.Text == "" {
				t.Error("expected non-empty response text")
			}
			if resp.StopReason != "end_turn" && resp.StopReason != "stop" {
				t.Errorf("unexpected stop_reason: %s", resp.StopReason)
			}

			t.Logf("provider=%s model=%s stop=%s tokens=%d+%d text=%q",
				name, resp.Model, resp.StopReason,
				resp.Usage.InputTokens, resp.Usage.OutputTokens, resp.Text)
		})
	}
}

func TestFireworksRuntimeToolLoopPreservesReasoningContentThroughAdapter(t *testing.T) {
	var requests []openAIChatCompletionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body openAIChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		requests = append(requests, body)
		w.Header().Set("Content-Type", "application/json")
		switch len(requests) {
		case 1:
			_, _ = fmt.Fprint(w, `{
				"id":"chatcmpl_tool",
				"model":"accounts/fireworks/models/deepseek-v4-flash",
				"choices":[{
					"finish_reason":"tool_calls",
					"message":{
						"role":"assistant",
						"reasoning_content":"hidden plan before tool",
						"content":"",
						"tool_calls":[{
							"id":"call_1",
							"type":"function",
							"function":{"name":"record_status","arguments":"{\"status\":\"ok\"}"}
						}]
					}
				}],
				"usage":{"prompt_tokens":12,"completion_tokens":7,"total_tokens":19}
			}`)
		default:
			_, _ = fmt.Fprint(w, `{
				"id":"chatcmpl_final",
				"model":"accounts/fireworks/models/deepseek-v4-flash",
				"choices":[{
					"finish_reason":"stop",
					"message":{"role":"assistant","content":"DONE"}
				}],
				"usage":{"prompt_tokens":20,"completion_tokens":2,"total_tokens":22}
			}`)
		}
	}))
	defer server.Close()

	provider := &FireworksProvider{
		apiKey:     "fw-test-key",
		modelID:    "accounts/fireworks/models/deepseek-v4-flash",
		httpClient: server.Client(),
		baseURL:    server.URL,
	}
	registry := runtime.NewToolRegistry()
	if err := registry.Register(runtime.Tool{
		Name:        "record_status",
		Description: "Record status.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"status": map[string]any{"type": "string"}},
			"required":   []string{"status"},
		},
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"recorded":true}`, nil
		},
	}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	text, _, err := runtime.RunToolLoop(
		context.Background(),
		NewBridgeProvider(provider),
		registry,
		[]json.RawMessage{json.RawMessage(`{"role":"user","content":[{"type":"text","text":"Record ok, then finish with DONE."}]}`)},
		"You are a harness proof agent.",
		0,
		func(types.EventKind, string, json.RawMessage) {},
		nil,
		runtime.WithToolLoopLLMConfig(runtime.LLMSelection{
			Provider:        "fireworks",
			Model:           "accounts/fireworks/models/deepseek-v4-flash",
			ReasoningEffort: "none",
		}),
	)
	if err != nil {
		t.Fatalf("run tool loop: %v", err)
	}
	if text != "DONE" {
		t.Fatalf("text = %q, want DONE", text)
	}
	if len(requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(requests))
	}
	second := requests[1]
	var assistant *openAIChatMessage
	for i := range second.Messages {
		if second.Messages[i].Role == "assistant" {
			assistant = &second.Messages[i]
			break
		}
	}
	if assistant == nil {
		t.Fatalf("second request did not include assistant turn: %#v", second.Messages)
	}
	if assistant.ReasoningContent != "hidden plan before tool" {
		t.Fatalf("reasoning_content = %q, want hidden plan before tool", assistant.ReasoningContent)
	}
	if len(assistant.ToolCalls) != 1 || assistant.ToolCalls[0].Function.Name != "record_status" {
		t.Fatalf("assistant tool calls = %#v", assistant.ToolCalls)
	}
	foundToolResult := false
	for _, msg := range second.Messages {
		if msg.Role == "tool" && msg.ToolCallID == "call_1" && strings.Contains(fmt.Sprint(msg.Content), "recorded") {
			foundToolResult = true
		}
	}
	if !foundToolResult {
		t.Fatalf("second request did not include tool result for call_1: %#v", second.Messages)
	}
}

func TestIntegrationFireworksRuntimeToolLoopLive(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") == "" {
		t.Skip("skipping integration test (set RUN_INTEGRATION_TESTS=1)")
	}
	models := []string{
		"accounts/fireworks/models/deepseek-v4-flash",
		"accounts/fireworks/models/deepseek-v4-pro",
		"accounts/fireworks/models/kimi-k2p6",
	}
	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			p, err := NewFireworksProviderFromEnv(model)
			if err != nil {
				t.Fatalf("fireworks provider: %v", err)
			}
			registry := runtime.NewToolRegistry()
			if err := registry.Register(runtime.Tool{
				Name:        "record_status",
				Description: "Record status.",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{"status": map[string]any{"type": "string"}},
					"required":   []string{"status"},
				},
				Func: func(ctx context.Context, args json.RawMessage) (string, error) {
					return `{"recorded":true,"status":"ok"}`, nil
				},
			}); err != nil {
				t.Fatalf("register tool: %v", err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			text, usage, err := runtime.RunToolLoop(
				ctx,
				NewBridgeProvider(p),
				registry,
				[]json.RawMessage{json.RawMessage(`{"role":"user","content":[{"type":"text","text":"Call record_status with status OK, then respond with FIREWORKS_RUNTIME_TOOL_LOOP_OK."}]}`)},
				"You are a Choir runtime harness proof. Use the tool once when requested.",
				0,
				func(types.EventKind, string, json.RawMessage) {},
				nil,
				runtime.WithToolLoopLLMConfig(runtime.LLMSelection{
					Provider:        "fireworks",
					Model:           model,
					ReasoningEffort: "none",
				}),
				runtime.WithInitialToolChoice("function:record_status"),
			)
			if err != nil {
				t.Fatalf("runtime tool loop: %v", err)
			}
			if !strings.Contains(text, "FIREWORKS_RUNTIME_TOOL_LOOP_OK") {
				t.Fatalf("text = %q, want marker; usage=%+v", text, usage)
			}
			t.Logf("model=%s usage=%d+%d text=%q", model, usage.InputTokens, usage.OutputTokens, text)
		})
	}
}

func TestIntegrationFireworksRuntimeToolLoopTextureShapedLive(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") == "" {
		t.Skip("skipping integration test (set RUN_INTEGRATION_TESTS=1)")
	}
	model := "accounts/fireworks/models/deepseek-v4-flash"
	p, err := NewFireworksProviderFromEnv(model)
	if err != nil {
		t.Fatalf("fireworks provider: %v", err)
	}
	registry := runtime.NewToolRegistry()
	if err := registry.Register(runtime.Tool{
		Name:        "record_status",
		Description: strings.Repeat("Record status for a Texture-shaped harness proof. ", 20),
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"status": map[string]any{"type": "string"}},
			"required":   []string{"status"},
		},
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"recorded":true,"status":"ok"}`, nil
		},
	}); err != nil {
		t.Fatalf("register record_status: %v", err)
	}
	for i := 1; i < 12; i++ {
		name := fmt.Sprintf("unused_texture_like_tool_%02d", i)
		if err := registry.Register(runtime.Tool{
			Name:        name,
			Description: strings.Repeat("Unused Texture-like tool definition to simulate a broad appagent catalog. ", 16),
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{"note": map[string]any{"type": "string"}},
			},
			Func: func(ctx context.Context, args json.RawMessage) (string, error) {
				return `{"unused":true}`, nil
			},
		}); err != nil {
			t.Fatalf("register %s: %v", name, err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	text, usage, err := runtime.RunToolLoop(
		ctx,
		NewBridgeProvider(p),
		registry,
		[]json.RawMessage{json.RawMessage(`{"role":"user","content":[{"type":"text","text":"Call record_status with status OK, then respond with FIREWORKS_TEXTURE_SHAPED_TOOL_LOOP_OK."}]}`)},
		strings.Repeat("You are a Texture appagent. Use the exact tool requested when the user asks for a tool proof. ", 160),
		0,
		func(types.EventKind, string, json.RawMessage) {},
		nil,
		runtime.WithToolLoopLLMConfig(runtime.LLMSelection{
			Provider:        "fireworks",
			Model:           model,
			ReasoningEffort: "none",
		}),
		runtime.WithInitialToolChoice("required"),
	)
	if err != nil {
		t.Fatalf("runtime texture-shaped tool loop: %v", err)
	}
	if !strings.Contains(text, "FIREWORKS_TEXTURE_SHAPED_TOOL_LOOP_OK") {
		t.Fatalf("text = %q, want marker; usage=%+v", text, usage)
	}
	t.Logf("model=%s usage=%d+%d text=%q", model, usage.InputTokens, usage.OutputTokens, text)
}
