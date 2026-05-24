package gatewayruntime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/runtime"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestCallWithToolsRoutesThroughGatewayWireContract(t *testing.T) {
	var gotAuth string
	var gotReq llmRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/provider/v1/inference" {
			t.Fatalf("path = %q, want /provider/v1/inference", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeJSON(t, w, llmResponse{
			ID:         "resp-1",
			Text:       "using a tool",
			Model:      gotReq.Model,
			StopReason: "tool_use",
			Usage:      tokenUsage{InputTokens: 11, OutputTokens: 7},
			ToolCalls: []contentToolCall{{
				ID:        "tool-1",
				Name:      "lookup",
				Arguments: json.RawMessage(`{"query":"choir"}`),
			}},
			ProviderName: "fireworks",
		})
	}))
	defer server.Close()

	provider := New(server.URL, "sandbox-token")
	provider.SetRuntimeLLMConfig("fireworks", "accounts/fireworks/models/deepseek-v4-flash", "none")

	resp, err := provider.CallWithTools(context.Background(), runtime.ToolLoopRequest{
		System:     "system",
		Messages:   []json.RawMessage{json.RawMessage(`{"role":"user","content":[{"type":"text","text":"hi"}]}`)},
		ToolChoice: "required",
		MaxTokens:  2048,
		ToolDefinitions: []runtime.ToolDefinition{{
			Name:        "lookup",
			Description: "look something up",
			Parameters:  map[string]any{"type": "object"},
		}},
	})
	if err != nil {
		t.Fatalf("CallWithTools returned error: %v", err)
	}
	if gotAuth != "Bearer sandbox-token" {
		t.Fatalf("authorization = %q, want bearer token", gotAuth)
	}
	if gotReq.Provider != "fireworks" || gotReq.Model != "accounts/fireworks/models/deepseek-v4-flash" || gotReq.ReasoningEffort != "none" {
		t.Fatalf("gateway request policy = provider %q model %q reasoning %q", gotReq.Provider, gotReq.Model, gotReq.ReasoningEffort)
	}
	if gotReq.Stream {
		t.Fatal("CallWithTools should use non-streaming gateway request")
	}
	if gotReq.ToolChoice != "required" {
		t.Fatalf("tool_choice = %q, want required", gotReq.ToolChoice)
	}
	if len(gotReq.Tools) != 1 || gotReq.Tools[0].Name != "lookup" {
		t.Fatalf("tools = %+v, want lookup tool", gotReq.Tools)
	}
	if resp.StopReason != "tool_use" || len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Name != "lookup" {
		t.Fatalf("tool response = %+v", resp)
	}
}

func TestExecuteStreamsGatewayDeltas(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"type":"message_start","id":"resp-2","model":"m"}` + "\n\n"))
		_, _ = w.Write([]byte(`data: {"type":"content_block_delta","delta":"hello "}` + "\n\n"))
		_, _ = w.Write([]byte(`data: {"type":"content_block_delta","delta":"world"}` + "\n\n"))
		_, _ = w.Write([]byte(`data: {"type":"message_delta","stop_reason":"end_turn","usage":{"input_tokens":3,"output_tokens":2}}` + "\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	provider := New(server.URL, "sandbox-token")
	provider.SetRuntimeLLMConfig("fireworks", "accounts/fireworks/models/deepseek-v4-flash", "none")
	run := &types.RunRecord{RunID: "run-1", Prompt: "tell me a story"}
	var deltas []string
	err := provider.Execute(context.Background(), run, func(kind types.EventKind, phase string, payload json.RawMessage) {
		if kind != types.EventRunDelta {
			return
		}
		var body map[string]string
		if err := json.Unmarshal(payload, &body); err == nil {
			deltas = append(deltas, body["text"])
		}
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if run.Result != "hello world" {
		t.Fatalf("run result = %q, want streamed text", run.Result)
	}
	if got := len(deltas); got != 2 {
		t.Fatalf("delta count = %d, want 2 (%v)", got, deltas)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}
