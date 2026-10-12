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

// modelFailingProvider fails every call to the models in fail with err and
// answers the rest, recording each request.
type modelFailingProvider struct {
	provideriface.Provider
	mu       sync.Mutex
	fail     map[string]error
	requests []provideriface.ToolLoopRequest
}

func (p *modelFailingProvider) CallWithTools(_ context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	p.mu.Lock()
	p.requests = append(p.requests, req)
	p.mu.Unlock()
	if err := p.fail[req.Model]; err != nil {
		return nil, err
	}
	return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "done by " + req.Model, Model: req.Model}, nil
}

func runFailoverLoop(t *testing.T, ctx context.Context, provider *modelFailingProvider, messages []json.RawMessage) (string, error, []string) {
	t.Helper()
	var events []string
	emit := func(kind types.EventKind, name string, payload json.RawMessage) {
		if name == "model_failover" {
			events = append(events, string(payload))
		}
	}
	text, _, err := RunToolLoop(ctx, provider, nil, messages, "system", 0, emit, nil,
		WithToolLoopLLMConfig(provideriface.LLMSelection{Provider: "opencode-go", Model: "deepseek-v4.1-flash", ReasoningEffort: "high"}),
		WithToolLoopFallbackLLMConfigs([]provideriface.LLMSelection{{Provider: "chatgpt", Model: "gpt-6-luna"}}),
		WithToolLoopConversationID("run-1"))
	return text, err, events
}

// A failing model hands the rest of the run to the next pool member, with a
// fresh conversation id, reasoning replay stripped and one recorded event.
func TestRunToolLoopFailsOverToNextPoolMember(t *testing.T) {
	provider := &modelFailingProvider{fail: map[string]error{"deepseek-v4.1-flash": fmt.Errorf("gateway: status 503 service unavailable")}}
	history := []json.RawMessage{
		json.RawMessage(`{"role":"user","content":"hello"}`),
		json.RawMessage(`{"role":"assistant","content":"thinking","reasoning_content":"deepseek private reasoning"}`),
	}
	text, err, events := runFailoverLoop(t, context.Background(), provider, history)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if text != "done by gpt-6-luna" {
		t.Fatalf("text = %q", text)
	}
	if len(events) != 1 || !strings.Contains(events[0], `"to_model":"gpt-6-luna"`) || !strings.Contains(events[0], `"from_model":"deepseek-v4.1-flash"`) {
		t.Fatalf("failover events = %v", events)
	}
	last := provider.requests[len(provider.requests)-1]
	if last.Provider != "chatgpt" || last.ConversationID != "run-1#failover" || last.ReasoningEffort != "high" {
		t.Fatalf("fallback request = provider %q conversation %q effort %q", last.Provider, last.ConversationID, last.ReasoningEffort)
	}
	for _, msg := range last.Messages {
		if strings.Contains(string(msg), "reasoning_content") {
			t.Fatalf("reasoning replay crossed providers: %s", msg)
		}
	}
}

// Failover happens once: when the fallback fails too, the run ends with the
// error instead of bouncing between models.
func TestRunToolLoopFailsOverAtMostOnce(t *testing.T) {
	provider := &modelFailingProvider{fail: map[string]error{
		"deepseek-v4.1-flash": fmt.Errorf("opencode provider: unsupported model"),
		"gpt-6-luna":          fmt.Errorf("model \"gpt-6-luna\" is not in the catalog for provider chatgpt"),
	}}
	_, err, events := runFailoverLoop(t, context.Background(), provider, []json.RawMessage{json.RawMessage(`{"role":"user","content":"hello"}`)})
	if err == nil || !strings.Contains(err.Error(), "not in the catalog") {
		t.Fatalf("err = %v, want the fallback's error", err)
	}
	if len(events) != 1 || len(provider.requests) != 2 {
		t.Fatalf("events %d, requests %d; want one failover and two calls", len(events), len(provider.requests))
	}
}

// Our own cancellation and our own malformed request never fail over.
func TestModelFailoverErrorClasses(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want bool
	}{
		{context.Canceled, false},
		{fmt.Errorf("wrap: %w", context.Canceled), false},
		{fmt.Errorf("invalid request body"), false},
		{fmt.Errorf("status 503"), true},
		{fmt.Errorf("You've hit your usage limit"), true},
		{fmt.Errorf("model \"gpt-5.5\" is not in the catalog for provider chatgpt"), true},
		{context.DeadlineExceeded, true},
	} {
		if got := IsModelFailoverError(tc.err); got != tc.want {
			t.Fatalf("IsModelFailoverError(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}

// A run cancelled by its caller does not fail over even if the provider's
// error looks transient.
func TestRunToolLoopDoesNotFailOverAfterCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	provider := &modelFailingProvider{fail: map[string]error{"deepseek-v4.1-flash": fmt.Errorf("status 503")}}
	_, err, events := runFailoverLoop(t, ctx, provider, []json.RawMessage{json.RawMessage(`{"role":"user","content":"hello"}`)})
	if err == nil || len(events) != 0 {
		t.Fatalf("err %v, failover events %v; want an error and no failover", err, events)
	}
}
