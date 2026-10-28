package toolregistry

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

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

// Failover never goes back: when the only fallback fails too, the run ends
// with the error instead of bouncing between models.
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
		// A pool member whose provider the gateway never registered (its
		// credential is missing) is "unavailable": fail over to the other.
		{fmt.Errorf("gateway: status 400: unsupported provider: synthetic"), true},
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

// The paper's research runs ended on "circuit open" while gpt-6-luna was
// healthy (docs/problems/free-model-429-opens-provider-circuit-stub-
// stories-2026-10-14.md, cause 2): a deepseek thread's fallbacks are
// [cat-jam, luna], and the loop failed over once, to cat-jam, behind the
// same open circuit. Ways the fix can fail: the loop still stops at the
// first fallback; it revisits a member or bounces between members; it
// fails the run while a member is left untried; it waits out transient
// retries on a member whose circuit is open before moving on; it reuses a
// conversation id across members.
func runPoolLoop(t *testing.T, provider *modelFailingProvider) (string, error, []string) {
	t.Helper()
	var events []string
	emit := func(kind types.EventKind, name string, payload json.RawMessage) {
		if name == "model_failover" {
			events = append(events, string(payload))
		}
	}
	text, _, err := RunToolLoop(context.Background(), provider, nil, []json.RawMessage{json.RawMessage(`{"role":"user","content":"hello"}`)}, "system", 0, emit, nil,
		WithToolLoopLLMConfig(provideriface.LLMSelection{Provider: "opencode-go", Model: "deepseek-v4.1-flash", ReasoningEffort: "high"}),
		WithToolLoopFallbackLLMConfigs([]provideriface.LLMSelection{
			{Provider: "opencode-go", Model: "cat-jam-free"},
			{Provider: "chatgpt", Model: "gpt-6-luna"},
		}),
		WithToolLoopConversationID("run-1"))
	return text, err, events
}

func requestedModels(p *modelFailingProvider) string {
	var models []string
	for _, req := range p.requests {
		models = append(models, req.Model)
	}
	return strings.Join(models, ",")
}

func TestRunToolLoopFailoverWalksThePoolPastAnOpenCircuit(t *testing.T) {
	providerOpen := fmt.Errorf("gateway call failed: gateway client: provider opencode-go: circuit open (upstream unhealthy)")
	provider := &modelFailingProvider{fail: map[string]error{
		"deepseek-v4.1-flash": providerOpen,
		"cat-jam-free":        providerOpen,
	}}
	start := time.Now()
	text, err, events := runPoolLoop(t, provider)
	if err != nil {
		t.Fatalf("run failed with a healthy member left: %v", err)
	}
	if text != "done by gpt-6-luna" {
		t.Fatalf("text = %q, want luna's answer", text)
	}
	if len(events) != 2 || !strings.Contains(events[1], `"to_model":"gpt-6-luna"`) {
		t.Fatalf("failover events = %v; want deepseek, then cat-jam, then luna", events)
	}
	if got := requestedModels(provider); got != "deepseek-v4.1-flash,cat-jam-free,gpt-6-luna" {
		t.Fatalf("calls went to %s; want each member once, in pool order", got)
	}
	ids := map[string]bool{}
	for _, req := range provider.requests {
		ids[req.ConversationID] = true
	}
	if len(ids) != 3 {
		t.Fatalf("conversation ids %v; each member needs a fresh one", ids)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("failover took %s; a member with a fallback left must not wait out transient retries", elapsed)
	}
}

// A model-scoped open circuit (cat-jam's, after its 429s) is skipped the
// same way, and deepseek's 5xx moves on too.
func TestRunToolLoopFailoverSkipsAModelCircuitAndA5xx(t *testing.T) {
	provider := &modelFailingProvider{fail: map[string]error{
		"deepseek-v4.1-flash": fmt.Errorf("gateway call failed: gateway client: provider opencode-go: status 503 Service Unavailable (sanitized)"),
		"cat-jam-free":        fmt.Errorf("gateway call failed: gateway client: provider opencode-go model cat-jam-free: circuit open (model unhealthy)"),
	}}
	text, err, _ := runPoolLoop(t, provider)
	if err != nil || text != "done by gpt-6-luna" {
		t.Fatalf("text %q err %v; want luna's answer", text, err)
	}
}

// The run fails only when every member has failed, each tried once.
func TestRunToolLoopFailsOnlyWhenEveryMemberFailed(t *testing.T) {
	provider := &modelFailingProvider{fail: map[string]error{
		"deepseek-v4.1-flash": fmt.Errorf("opencode provider: unsupported model"),
		"cat-jam-free":        fmt.Errorf("opencode provider: unsupported model"),
		"gpt-6-luna":          fmt.Errorf("model \"gpt-6-luna\" is not in the catalog for provider chatgpt"),
	}}
	_, err, events := runPoolLoop(t, provider)
	if err == nil || !strings.Contains(err.Error(), "gpt-6-luna") {
		t.Fatalf("err = %v, want the last member's error", err)
	}
	if got := requestedModels(provider); got != "deepseek-v4.1-flash,cat-jam-free,gpt-6-luna" || len(events) != 2 {
		t.Fatalf("calls %s, %d failovers; want each member once and two failovers", got, len(events))
	}
}
