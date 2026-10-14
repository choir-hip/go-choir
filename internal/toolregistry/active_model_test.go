package toolregistry

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The tool loop puts the answering model on the tool context, so a desk
// record names the model that produced it
// (docs/problems/desk-records-lack-model-id-2026-10-14.md M3).
func TestRunToolLoopPutsAnsweringModelOnToolContext(t *testing.T) {
	var seen ActiveModel
	var ok bool
	registry := NewToolRegistry()
	if err := registry.Register(Tool{
		Name: "desk_go_eval",
		Func: func(ctx context.Context, _ json.RawMessage) (string, error) {
			seen, ok = ActiveModelFrom(ctx)
			return `{"ok":true}`, nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	provider := newMockToolLoopProvider(
		&provideriface.ToolLoopResponse{StopReason: "tool_use", ToolCalls: []types.ToolCall{{ID: "c1", Name: "desk_go_eval", Arguments: json.RawMessage(`{}`)}}},
		&provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "done"},
	)
	_, _, err := RunToolLoop(context.Background(), provider, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`)},
		"sys", 0, func(types.EventKind, string, json.RawMessage) {}, nil,
		WithToolLoopLLMConfig(provideriface.LLMSelection{Provider: "opencode-go", Model: "deepseek-v4.1-flash"}))
	if err != nil {
		t.Fatalf("run tool loop: %v", err)
	}
	if !ok || seen.ID() != "opencode-go/deepseek-v4.1-flash" {
		t.Fatalf("tool context model = %+v (%v), want opencode-go/deepseek-v4.1-flash", seen, ok)
	}
}

func TestActiveModelID(t *testing.T) {
	cases := []struct {
		m    ActiveModel
		want string
	}{
		{ActiveModel{Provider: "chatgpt", Model: "gpt-6-luna"}, "chatgpt/gpt-6-luna"},
		{ActiveModel{Model: "gpt-6-luna"}, "gpt-6-luna"},
		{ActiveModel{Provider: "openrouter", Model: "deepseek/deepseek-v4"}, "openrouter/deepseek/deepseek-v4"},
		{ActiveModel{Provider: "chatgpt"}, ""},
	}
	for _, c := range cases {
		if got := c.m.ID(); got != c.want {
			t.Errorf("%+v.ID() = %q, want %q", c.m, got, c.want)
		}
	}
	if _, ok := ActiveModelFrom(WithActiveModel(context.Background(), "p", "")); ok {
		t.Fatal("an empty model must not install")
	}
}
