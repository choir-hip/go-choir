package toolregistry

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// threadCallProvider fails deepseek, answers luna's first turn with a tool
// call, and records every request.
type threadCallProvider struct {
	provideriface.Provider
	mu       sync.Mutex
	requests []provideriface.ToolLoopRequest
	lunaN    int
}

func (p *threadCallProvider) CallWithTools(_ context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requests = append(p.requests, req)
	if req.Model == "deepseek-v4.1-flash" {
		return nil, fmt.Errorf("gateway: status 503 service unavailable")
	}
	if strings.Contains(string(req.Messages[len(req.Messages)-1]), "INSTRUCTION") {
		return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "checkpoint", Model: req.Model}, nil
	}
	p.lunaN++
	if p.lunaN == 1 {
		return &provideriface.ToolLoopResponse{StopReason: "tool_use", Model: req.Model,
			ToolCalls: []types.ToolCall{{ID: "c1", Name: "echo", Arguments: json.RawMessage(`{}`)}}}, nil
	}
	return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "reply " + req.Model, Model: req.Model}, nil
}

// A thread call made after a failover is the loop's own next request for the
// member now answering, with its system prompt, tools, messages, conversation
// id and cache key, and one more user message.
func TestThreadCallAfterFailoverIsTheThreadsNextTurn(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.Register(Tool{Name: "echo", Description: "echo", Func: func(context.Context, json.RawMessage) (string, error) { return `{}`, nil }}); err != nil {
		t.Fatal(err)
	}
	provider := &threadCallProvider{}
	var asked *provideriface.ToolLoopResponse
	var askErr error
	calls := 0
	hooks := ToolLoopMemoryHooks{BeforeProviderCall: func(ctx context.Context, msgs []json.RawMessage) ([]json.RawMessage, error) {
		calls++
		if calls == 3 { // after the failover retry and the tool turn
			ask, ok := ThreadCall(ctx)
			if !ok {
				t.Fatal("no thread call offered to the hook")
			}
			asked, askErr = ask(ctx, "INSTRUCTION")
		}
		return msgs, nil
	}}
	_, _, err := RunToolLoop(context.Background(), provider, registry,
		[]json.RawMessage{json.RawMessage(`{"role":"user","content":"hello"}`)}, "system", 0, func(types.EventKind, string, json.RawMessage) {}, nil,
		WithToolLoopLLMConfig(provideriface.LLMSelection{Provider: "opencode-go", Model: "deepseek-v4.1-flash", ReasoningEffort: "high"}),
		WithToolLoopFallbackLLMConfigs([]provideriface.LLMSelection{{Provider: "chatgpt", Model: "gpt-6-luna"}}),
		WithToolLoopConversationID("run-1"),
		WithToolLoopPromptCache("texture", "agent-1"),
		WithToolLoopMemoryHooks(hooks))
	if err != nil || askErr != nil || asked == nil {
		t.Fatalf("run err=%v ask err=%v asked=%v", err, askErr, asked)
	}
	var thread, next, first provideriface.ToolLoopRequest
	for _, r := range provider.requests {
		last := string(r.Messages[len(r.Messages)-1])
		switch {
		case strings.Contains(last, "INSTRUCTION"):
			thread = r
		case r.Model == "gpt-6-luna" && len(r.Messages) > 1:
			next = r
		case r.Model == "gpt-6-luna":
			first = r
		}
	}
	if thread.Model != "gpt-6-luna" || thread.Provider != "chatgpt" || thread.ConversationID != "run-1#failover" || thread.ReasoningEffort != "high" {
		t.Fatalf("thread call went to %s/%s conv %q effort %q, want the failed-over member", thread.Provider, thread.Model, thread.ConversationID, thread.ReasoningEffort)
	}
	if thread.CacheKey == "" || thread.CacheKey != next.CacheKey || thread.CacheKey != first.CacheKey {
		t.Fatalf("cache keys thread=%q next=%q first=%q", thread.CacheKey, next.CacheKey, first.CacheKey)
	}
	if thread.System != next.System || len(thread.ToolDefinitions) != 1 || len(thread.ToolDefinitions) != len(next.ToolDefinitions) || thread.ToolChoice != next.ToolChoice {
		t.Fatalf("thread system/tools differ from the next turn's: %q tools %d", thread.System, len(thread.ToolDefinitions))
	}
	if len(thread.Messages) != len(next.Messages)+1 {
		t.Fatalf("thread messages %d, next turn %d, want one more", len(thread.Messages), len(next.Messages))
	}
	for i := range next.Messages {
		if string(thread.Messages[i]) != string(next.Messages[i]) {
			t.Fatalf("message %d differs from the next turn's prefix", i)
		}
	}
}

func TestThreadCallAbsentOutsideTheLoop(t *testing.T) {
	if _, ok := ThreadCall(context.Background()); ok {
		t.Fatal("thread call offered outside a loop")
	}
}
