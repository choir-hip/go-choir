package provider

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// Live smoke test against server.codeium.com; runs only when DEVIN_API_KEY
// is set (the Devin CLI session credential).
func TestDevinLiveSmoke(t *testing.T) {
	key := os.Getenv("DEVIN_API_KEY")
	if key == "" {
		t.Skip("DEVIN_API_KEY not set")
	}
	p, err := NewDevinProvider(key, "", "swe-2-flash")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	resp, err := p.Call(ctx, LLMRequest{
		Model:     "swe-2-flash",
		System:    "You are terse.",
		Messages:  []Message{{Role: "user", Content: []Block{{Type: "text", Text: "Reply with exactly: pong"}}}},
		MaxTokens: 2048,
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if !strings.Contains(strings.ToLower(resp.Text), "pong") {
		t.Fatalf("text = %q", resp.Text)
	}
	t.Logf("ok model=%s text=%q usage=%+v", resp.Model, resp.Text, resp.Usage)
}

// TestDevinLiveToolCall exercises the tool_use → tool_result round trip that
// desk runs depend on.
func TestDevinLiveToolCall(t *testing.T) {
	key := os.Getenv("DEVIN_API_KEY")
	if key == "" {
		t.Skip("DEVIN_API_KEY not set")
	}
	p, err := NewDevinProvider(key, "", "swe-2-flash")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	tools := []ToolDef{{
		Name:        "get_weather",
		Description: "Get the current weather for a city.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"city": map[string]any{"type": "string"},
			},
			"required": []string{"city"},
		},
	}}

	// Turn 1: model must call the tool.
	resp, err := p.Call(ctx, LLMRequest{
		Model:      "swe-2-flash",
		System:     "Use the get_weather tool when asked about weather.",
		Messages:   []Message{{Role: "user", Content: []Block{{Type: "text", Text: "What is the weather in Paris? Use the tool."}}}},
		Tools:      tools,
		ToolChoice: "auto",
		MaxTokens:  4096,
	})
	if err != nil {
		t.Fatalf("turn1: %v", err)
	}
	if len(resp.ToolCalls) == 0 {
		t.Fatalf("expected tool_use, got stop=%s text=%q", resp.StopReason, resp.Text)
	}
	tc := resp.ToolCalls[0]
	if tc.Name != "get_weather" {
		t.Fatalf("expected get_weather tool_use, got %+v", tc)
	}
	callID, callName, callArgs := tc.ID, tc.Name, tc.Arguments
	if callID == "" {
		t.Fatal("tool_use block missing id")
	}
	t.Logf("turn1 tool_use id=%s name=%s args=%s", callID, callName, string(callArgs))

	// Turn 2: feed the tool result back; model should answer with it.
	resultJSON, _ := json.Marshal(map[string]any{"city": "Paris", "temp_c": 18, "condition": "cloudy"})
	resp2, err := p.Call(ctx, LLMRequest{
		Model:  "swe-2-flash",
		System: "You are terse.",
		Messages: []Message{
			{Role: "user", Content: []Block{{Type: "text", Text: "What is the weather in Paris? Use the tool."}}},
			{Role: "assistant", Content: []Block{{Type: "tool_use", ID: callID, Name: callName, Input: callArgs}}},
			{Role: "user", Content: []Block{{Type: "tool_result", ToolUseID: callID, Text: string(resultJSON)}}},
		},
		Tools:      tools,
		ToolChoice: "auto",
		MaxTokens:  4096,
	})
	if err != nil {
		t.Fatalf("turn2: %v", err)
	}
	if !strings.Contains(strings.ToLower(resp2.Text), "paris") && !strings.Contains(resp2.Text, "18") {
		t.Fatalf("turn2 text = %q", resp2.Text)
	}
	t.Logf("turn2 text=%q", resp2.Text)
}
