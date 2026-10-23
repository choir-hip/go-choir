package toolregistry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Pool luna-synthetic (owner 2026-10-23: "chatgpt luna 100% with synthetic
// only as fallback for now"): the run is on chatgpt/gpt-6-luna and
// llm_fallbacks is the one Synthetic member. Ways it could fail, written
// before the code: a luna error class that should move the run does not (the
// run fails with Synthetic idle); a class that should not move it does (our
// own bad request goes to Synthetic); the move goes to the wrong provider
// or loses the reasoning effort; the run goes back to luna after the move;
// a Synthetic 429 after a luna failure bounces the run or never ends.

const (
	lunaSyntheticModel = "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017"
)

// gatewayLikeError mimics the guest's gatewayHTTPStatusError: the gateway
// answers a provider failure with a 502 whose body is the sanitized provider
// text, and the guest's error is transient for 429 and 5xx.
type gatewayLikeError struct {
	msg       string
	transient bool
}

func (e *gatewayLikeError) Error() string   { return e.msg }
func (e *gatewayLikeError) Transient() bool { return e.transient }

type lunaSyntheticProvider struct {
	provideriface.Provider
	mu       sync.Mutex
	lunaErr  error
	synthErr error
	// toolFirst makes Synthetic's first answer a tool call.
	toolFirst bool
	requests  []provideriface.ToolLoopRequest
}

func (p *lunaSyntheticProvider) CallWithTools(_ context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requests = append(p.requests, req)
	if req.Provider == "chatgpt" && p.lunaErr != nil {
		return nil, p.lunaErr
	}
	if req.Provider == "synthetic" && p.synthErr != nil {
		return nil, p.synthErr
	}
	if p.toolFirst && req.Provider == "synthetic" {
		served := 0
		for _, r := range p.requests {
			if r.Provider == "synthetic" {
				served++
			}
		}
		if served == 1 {
			return &provideriface.ToolLoopResponse{StopReason: "tool_use", Model: req.Model,
				ToolCalls: []types.ToolCall{{ID: "c1", Name: "ping", Arguments: json.RawMessage(`{}`)}}}, nil
		}
	}
	return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "done by " + req.Provider + "/" + req.Model, Model: req.Model}, nil
}

func (p *lunaSyntheticProvider) providers() []string {
	out := make([]string, 0, len(p.requests))
	for _, r := range p.requests {
		out = append(out, r.Provider)
	}
	return out
}

func runLunaSynthetic(ctx context.Context, p *lunaSyntheticProvider, registry *ToolRegistry) (string, error, []string) {
	var events []string
	emit := func(_ types.EventKind, name string, payload json.RawMessage) {
		if name == "model_failover" {
			events = append(events, string(payload))
		}
	}
	text, _, err := RunToolLoop(ctx, p, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":"hello"}`)}, "system", 0, emit, nil,
		WithToolLoopLLMConfig(provideriface.LLMSelection{Provider: "chatgpt", Model: "gpt-6-luna", ReasoningEffort: "high"}),
		WithToolLoopFallbackLLMConfigs([]provideriface.LLMSelection{{Provider: "synthetic", Model: lunaSyntheticModel}}),
		WithToolLoopConversationID("run-1"))
	return text, err, events
}

func fastRateLimitRetries(t *testing.T) {
	t.Helper()
	original := providerRateLimitRetryDelays
	providerRateLimitRetryDelays = []time.Duration{0}
	t.Cleanup(func() { providerRateLimitRetryDelays = original })
}

// Every luna error class that should move a thread to Synthetic does, with
// the error text the guest actually sees: the gateway answers a provider
// failure 502 with the sanitized provider text (so a ChatGPT 429 or usage
// limit reads "status 429"), an open breaker reads "circuit open", and a
// slow call reads as a deadline.
func TestRunToolLoopMovesLunaFailuresToSynthetic(t *testing.T) {
	fastRateLimitRetries(t)
	for name, lunaErr := range map[string]error{
		"429 rate limit":        &gatewayLikeError{"gateway call failed: gateway client: chatgpt: status 429 Too Many Requests (sanitized)", true},
		"429 usage limit":       &gatewayLikeError{"gateway call failed: gateway client: chatgpt: status 429 (sanitized)", true},
		"usage limit text":      fmt.Errorf("gateway call failed: You've hit your usage limit"),
		"500":                   &gatewayLikeError{"gateway call failed: gateway client: chatgpt: status 500 Internal Server Error (sanitized)", true},
		"502":                   &gatewayLikeError{"gateway call failed: gateway client: status 502 Bad Gateway (sanitized)", true},
		"503":                   &gatewayLikeError{"gateway call failed: gateway client: chatgpt: status 503 Service Unavailable (sanitized)", true},
		"504":                   &gatewayLikeError{"gateway call failed: gateway client: status 504 Gateway Timeout (sanitized)", true},
		"timeout in a 502 body": &gatewayLikeError{"gateway call failed: gateway client: chatgpt: http call: context deadline exceeded", true},
		"deadline exceeded":     fmt.Errorf("gateway call failed: %w", context.DeadlineExceeded),
		"open circuit":          &gatewayLikeError{"gateway call failed: gateway client: provider chatgpt model gpt-6-luna: circuit open (model unhealthy)", true},
		"model not served":      fmt.Errorf("gateway call failed: model \"gpt-6-luna\" is not in the catalog for provider chatgpt"),
	} {
		t.Run(name, func(t *testing.T) {
			p := &lunaSyntheticProvider{lunaErr: lunaErr}
			text, err, events := runLunaSynthetic(context.Background(), p, nil)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if text != "done by synthetic/"+lunaSyntheticModel {
				t.Fatalf("text = %q, providers %v", text, p.providers())
			}
			last := p.requests[len(p.requests)-1]
			if last.Provider != "synthetic" || last.Model != lunaSyntheticModel || last.ReasoningEffort != "high" || last.ConversationID != "run-1#failover" {
				t.Fatalf("fallback request = %+v", last)
			}
			if len(events) != 1 || !strings.Contains(events[0], `"to_provider":"synthetic"`) || !strings.Contains(events[0], `"from_provider":"chatgpt"`) {
				t.Fatalf("failover events = %v", events)
			}
		})
	}
}

// A request error of ours does not move a thread to Synthetic: it fails on
// luna, once.
func TestRunToolLoopKeepsOwnRequestErrorsOnLuna(t *testing.T) {
	for name, lunaErr := range map[string]error{
		"bad request":   &gatewayLikeError{"gateway call failed: gateway client: invalid request body", false},
		"image refused": &gatewayLikeError{"gateway call failed: gateway client: image too large", false},
		"canceled":      fmt.Errorf("gateway call failed: %w", context.Canceled),
	} {
		t.Run(name, func(t *testing.T) {
			p := &lunaSyntheticProvider{lunaErr: lunaErr}
			_, err, events := runLunaSynthetic(context.Background(), p, nil)
			if err == nil || len(events) != 0 || len(p.requests) != 1 || p.requests[0].Provider != "chatgpt" {
				t.Fatalf("err %v, events %v, providers %v; want a luna failure and no move", err, events, p.providers())
			}
		})
	}
}

// The move is sticky for the rest of the run: after luna fails once, the
// next iteration (after a tool call) goes straight to Synthetic and never
// retries luna.
func TestRunToolLoopStaysOnSyntheticAfterLunaFails(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.Register(Tool{Name: "ping", Func: func(context.Context, json.RawMessage) (string, error) { return "pong", nil }}); err != nil {
		t.Fatal(err)
	}
	p := &lunaSyntheticProvider{lunaErr: &gatewayLikeError{"gateway client: status 503", true}, toolFirst: true}
	text, err, events := runLunaSynthetic(context.Background(), p, registry)
	if err != nil || text != "done by synthetic/"+lunaSyntheticModel {
		t.Fatalf("text %q err %v", text, err)
	}
	if got := strings.Join(p.providers(), ","); got != "chatgpt,synthetic,synthetic" || len(events) != 1 {
		t.Fatalf("providers %s, %d failover events; want chatgpt,synthetic,synthetic and one event", got, len(events))
	}
}

// A Synthetic concurrency 429 after a luna failure: Synthetic is the last
// member, so the run does not bounce back to luna. The rate-limit retries run
// against Synthetic alone, then the transient wait, and the run ends with
// the Synthetic error when its context ends.
func TestRunToolLoopSyntheticConcurrency429AfterLunaFailureDoesNotBounce(t *testing.T) {
	fastRateLimitRetries(t)
	p := &lunaSyntheticProvider{
		lunaErr:  &gatewayLikeError{"gateway client: chatgpt: status 503 (sanitized)", true},
		synthErr: &gatewayLikeError{"gateway call failed: gateway client: synthetic: status 429 Too Many Requests (sanitized)", true},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err, events := runLunaSynthetic(ctx, p, nil)
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want the run to end at its deadline in the transient wait", err)
	}
	if got := strings.Join(p.providers(), ","); got != "chatgpt,synthetic,synthetic" || len(events) != 1 {
		t.Fatalf("providers %s, %d events; want luna once then Synthetic with its rate-limit retry, never luna again", got, len(events))
	}
}
