package toolregistry

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Model-pool autoroute (docs/design/model-pool-autoroute-2026-11-02.md),
// the guest half. Ways it could fail, written before the code:
//   - a quota 429 on a member with a healthy member left still waits the
//     85 s same-model backoff (2026-11-01: every Synthetic thread did);
//   - a Synthetic concurrency 429 fails the run over instead of retrying
//     the same model (one burst would move every thread);
//   - a pool that is down everywhere fails faster than before (the last
//     member must owe the skipped members their backoff);
//   - the gateway's own per-computer limit moves the run, though every
//     member is refused alike;
//   - a ChatGPT usage limit does not move the run to Synthetic.

type poolProvider struct {
	provideriface.Provider
	mu       sync.Mutex
	errs     map[string][]error // per provider, consumed in order; the last repeats
	requests []provideriface.ToolLoopRequest
}

func (p *poolProvider) CallWithTools(_ context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requests = append(p.requests, req)
	if errs := p.errs[req.Provider]; len(errs) > 0 {
		err := errs[0]
		if len(errs) > 1 {
			p.errs[req.Provider] = errs[1:]
		}
		if err != nil {
			return nil, err
		}
	}
	return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "done by " + req.Provider, Model: req.Model}, nil
}

func (p *poolProvider) count(name string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for _, r := range p.requests {
		if r.Provider == name {
			n++
		}
	}
	return n
}

var (
	synthMember = provideriface.LLMSelection{Provider: "synthetic", Model: lunaSyntheticModel}
	lunaMember  = provideriface.LLMSelection{Provider: "chatgpt", Model: "gpt-6-luna"}
)

func runPool(ctx context.Context, p *poolProvider, primary provideriface.LLMSelection, fallbacks ...provideriface.LLMSelection) (string, error, map[string]int) {
	phases := map[string]int{}
	var mu sync.Mutex
	emit := func(_ types.EventKind, name string, _ json.RawMessage) {
		mu.Lock()
		phases[name]++
		mu.Unlock()
	}
	text, _, err := RunToolLoop(ctx, p, nil, []json.RawMessage{json.RawMessage(`{"role":"user","content":"hello"}`)}, "system", 0, emit, nil,
		WithToolLoopLLMConfig(primary), WithToolLoopFallbackLLMConfigs(fallbacks), WithToolLoopConversationID("run-1"))
	return text, err, phases
}

// The production delays stay in place: a same-model wait would show as 5 s.
func TestSyntheticQuotaRoutesToChatGPTWithNoRetryWait(t *testing.T) {
	quota := &gatewayLikeError{"gateway call failed: gateway client: synthetic: quota exhausted: status 429 Too Many Requests (sanitized)", true}
	p := &poolProvider{errs: map[string][]error{"synthetic": {quota}}}
	start := time.Now()
	text, err, phases := runPool(context.Background(), p, synthMember, lunaMember)
	if err != nil || text != "done by chatgpt" {
		t.Fatalf("run = %q, %v", text, err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("failover took %s, want no same-model backoff", elapsed)
	}
	if p.count("synthetic") != 1 || phases["provider_rate_limit"] != 0 || phases["model_failover"] != 1 {
		t.Fatalf("synthetic calls %d, phases %v", p.count("synthetic"), phases)
	}
}

func TestChatGPTQuotaRoutesToSyntheticWithNoRetryWait(t *testing.T) {
	for name, lunaErr := range map[string]error{
		"typed quota":  &gatewayLikeError{"gateway call failed: gateway client: chatgpt: quota exhausted: status 429 Too Many Requests (sanitized)", true},
		"plain 429":    &gatewayLikeError{"gateway call failed: gateway client: chatgpt: status 429 Too Many Requests (sanitized)", true},
		"circuit open": &gatewayLikeError{"gateway call failed: gateway client: provider chatgpt model gpt-6-luna: circuit open (model unhealthy)", true},
	} {
		t.Run(name, func(t *testing.T) {
			p := &poolProvider{errs: map[string][]error{"chatgpt": {lunaErr}}}
			start := time.Now()
			text, err, phases := runPool(context.Background(), p, lunaMember, synthMember)
			if err != nil || text != "done by synthetic" {
				t.Fatalf("run = %q, %v", text, err)
			}
			if elapsed := time.Since(start); elapsed > 2*time.Second || phases["provider_rate_limit"] != 0 {
				t.Fatalf("failover took %s with phases %v, want no same-model backoff", elapsed, phases)
			}
		})
	}
}

func TestConcurrency429RetriesTheSameModelInAPool(t *testing.T) {
	fastRateLimitRetries(t)
	conc := &gatewayLikeError{"gateway call failed: gateway client: synthetic: status 429 Too Many Requests (concurrency limit, sanitized)", true}
	p := &poolProvider{errs: map[string][]error{"synthetic": {conc, nil}}}
	text, err, phases := runPool(context.Background(), p, synthMember, lunaMember)
	if err != nil || text != "done by synthetic" {
		t.Fatalf("run = %q, %v: a concurrency 429 must not move the run", text, err)
	}
	if p.count("chatgpt") != 0 || phases["provider_rate_limit"] != 1 || phases["model_failover"] != 0 {
		t.Fatalf("chatgpt calls %d, phases %v", p.count("chatgpt"), phases)
	}
}

func TestGatewayComputerLimitRetriesTheSameModel(t *testing.T) {
	fastRateLimitRetries(t)
	limit := &gatewayLikeError{"gateway call failed: gateway client: rate limit exceeded for autoputer c-1 (limit 60 requests per 1m0s)", true}
	p := &poolProvider{errs: map[string][]error{"synthetic": {limit, nil}}}
	text, err, _ := runPool(context.Background(), p, synthMember, lunaMember)
	if err != nil || text != "done by synthetic" || p.count("chatgpt") != 0 {
		t.Fatalf("run = %q, %v, chatgpt calls %d: the computer's own limit refuses every member", text, err, p.count("chatgpt"))
	}
}

// Both members down: the first fails over at once, and the last pays the
// backoff for both, so the pool waits at least as long as before
// (2 x len(delays) same-model retries, where before each member had one set).
func TestAllMembersDownKeepsTheBackoff(t *testing.T) {
	original := providerRateLimitRetryDelays
	providerRateLimitRetryDelays = []time.Duration{0, 0, 0}
	t.Cleanup(func() { providerRateLimitRetryDelays = original })
	synthQuota := &gatewayLikeError{"gateway call failed: gateway client: synthetic: quota exhausted: status 429 Too Many Requests (sanitized)", true}
	lunaQuota := &gatewayLikeError{"gateway call failed: gateway client: chatgpt: quota exhausted: status 429 Too Many Requests (sanitized)", true}
	lunaOpen := &gatewayLikeError{"gateway call failed: gateway client: provider chatgpt model gpt-6-luna: circuit open (model unhealthy)", true}
	// Luna answers its quota once, then its open circuit, then (cooldown
	// over, top-up landed) a success on the fifth call.
	p := &poolProvider{errs: map[string][]error{
		"synthetic": {synthQuota},
		"chatgpt":   {lunaQuota, lunaOpen, lunaOpen, lunaOpen, nil},
	}}
	text, err, phases := runPool(context.Background(), p, synthMember, lunaMember)
	if err != nil || text != "done by chatgpt" {
		t.Fatalf("run = %q, %v: the last member must keep backing off through its open circuit", text, err)
	}
	if p.count("synthetic") != 1 || p.count("chatgpt") != 5 || phases["provider_rate_limit"] != 4 {
		t.Fatalf("synthetic %d chatgpt %d phases %v", p.count("synthetic"), p.count("chatgpt"), phases)
	}

	// Never recovering: the last member's retries end after two rounds and
	// the call returns the provider's error (the run's transient loop then
	// retries as before).
	p = &poolProvider{errs: map[string][]error{"chatgpt": {lunaQuota}}}
	_, err = callToolLoopProviderWithPoolRetries(context.Background(), p, provideriface.ToolLoopRequest{Provider: "chatgpt"}, nil, false, 2)
	if err == nil || !strings.Contains(err.Error(), "quota exhausted") {
		t.Fatalf("err = %v, want the last member's quota", err)
	}
	if got, want := p.count("chatgpt"), 1+2*len(providerRateLimitRetryDelays); got != want {
		t.Fatalf("last-member calls = %d, want %d (today's two members' worth of retries)", got, want)
	}
	// With a member left, the same error returns after one call.
	p = &poolProvider{errs: map[string][]error{"chatgpt": {lunaQuota}}}
	_, _ = callToolLoopProviderWithPoolRetries(context.Background(), p, provideriface.ToolLoopRequest{Provider: "chatgpt"}, nil, true, 1)
	if got := p.count("chatgpt"); got != 1 {
		t.Fatalf("calls with a member left = %d, want 1", got)
	}
}
