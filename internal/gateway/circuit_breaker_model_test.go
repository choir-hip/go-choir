package gateway

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/health"
	"github.com/yusefmosiah/go-choir/internal/provider"
)

// A free preview model's upstream rate limit opened the whole opencode-go
// circuit, and deepseek-v4.1-flash (the paid Go plan, same provider)
// failed with it (docs/problems/free-model-429-opens-provider-circuit-
// stub-stories-2026-10-14.md). Ways the fix can fail, written before it:
//   - a 429 (or 5xx, or a timeout) from one model still blocks another
//     model on the same provider;
//   - a model's half-open probe still decides for its siblings;
//   - a 429 opens nothing, so every run keeps hitting the rate-limited
//     model;
//   - a connection failure (the provider itself unreachable) no longer
//     opens anything shared, so each model rediscovers it alone;
//   - a slow model's deadline is read as a provider outage and blocks its
//     siblings;
//   - the open model's error no longer says "circuit open", so the
//     runtime does not fail over on it;
//   - the ops endpoints lose sight of the model breakers.

type modelStub struct {
	mu    sync.Mutex
	errs  map[string]error
	calls map[string]int
}

func newModelStub(errs map[string]error) *modelStub {
	return &modelStub{errs: errs, calls: map[string]int{}}
}

func (s *modelStub) Name() string { return "opencode-go" }
func (s *modelStub) IsReal() bool { return true }
func (s *modelStub) Call(_ context.Context, req provider.LLMRequest) (*provider.LLMResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls[req.Model]++
	if err := s.errs[req.Model]; err != nil {
		return nil, err
	}
	return &provider.LLMResponse{Text: "ok", Model: req.Model, ProviderName: "opencode-go"}, nil
}
func (s *modelStub) Stream(ctx context.Context, req provider.LLMRequest, _ func(provider.StreamChunk)) (*provider.LLMResponse, error) {
	return s.Call(ctx, req)
}
func (s *modelStub) count(model string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls[model]
}

func callModel(cbp *CircuitBreakingProvider, model string) error {
	_, err := cbp.Call(context.Background(), provider.LLMRequest{Model: model})
	return err
}

func TestOneModelsRateLimitDoesNotOpenTheCircuitForAnother(t *testing.T) {
	rateLimited := errors.New("opencode-go: status 429 Too Many Requests (sanitized)")
	stub := newModelStub(map[string]error{"cat-jam-free": rateLimited})
	cbp := NewCircuitBreakingProvider(stub, health.BreakerConfig{FailureThreshold: 2, OpenTimeout: time.Hour})

	for i := 0; i < 2; i++ {
		if err := callModel(cbp, "cat-jam-free"); err == nil || !strings.Contains(err.Error(), "429") {
			t.Fatalf("cat-jam call %d: err = %v, want the upstream 429", i, err)
		}
	}
	// cat-jam's own circuit is open: the third call fails fast, says
	// "circuit open" (the runtime fails over on it) and names the model.
	err := callModel(cbp, "cat-jam-free")
	if err == nil || !strings.Contains(err.Error(), "circuit open") || !strings.Contains(err.Error(), "cat-jam-free") {
		t.Fatalf("cat-jam after two 429s: err = %v, want its model circuit open", err)
	}
	if strings.Contains(err.Error(), "429") {
		t.Fatalf("circuit-open error %q carries 429; the runtime would back off on it instead of failing over", err)
	}
	if n := stub.count("cat-jam-free"); n != 2 {
		t.Fatalf("upstream saw %d cat-jam calls; an open circuit must not forward", n)
	}
	// deepseek on the same provider is untouched.
	if err := callModel(cbp, "deepseek-v4.1-flash"); err != nil {
		t.Fatalf("deepseek after cat-jam's circuit opened: %v", err)
	}
	if got := cbp.Breaker().State(); got != health.StateClosed {
		t.Fatalf("provider breaker = %v after a model's 429s, want closed", got)
	}
	if got := cbp.ModelBreaker("deepseek-v4.1-flash").State(); got != health.StateClosed {
		t.Fatalf("deepseek breaker = %v, want closed", got)
	}
}

// A model's half-open probe decides for that model only.
func TestHalfOpenProbeOfOneModelDoesNotDecideForAnother(t *testing.T) {
	now := time.Now()
	clock := func() time.Time { return now }
	stub := newModelStub(map[string]error{"cat-jam-free": errors.New("opencode-go: status 503 Service Unavailable (sanitized)")})
	cbp := NewCircuitBreakingProvider(stub, health.BreakerConfig{FailureThreshold: 1, OpenTimeout: 30 * time.Second, Now: clock})
	_ = callModel(cbp, "cat-jam-free")
	now = now.Add(31 * time.Second)
	_ = callModel(cbp, "cat-jam-free") // the probe fails and reopens cat-jam
	for i := 0; i < 3; i++ {
		if err := callModel(cbp, "deepseek-v4.1-flash"); err != nil {
			t.Fatalf("deepseek call %d while cat-jam probes: %v", i, err)
		}
	}
	if got := cbp.ModelBreaker("cat-jam-free").State(); got != health.StateOpen {
		t.Fatalf("cat-jam breaker = %v after a failed probe, want open", got)
	}
}

// A provider that cannot be reached is down for every model: a connection
// failure opens the provider-wide breaker.
func TestUnreachableProviderOpensForEveryModel(t *testing.T) {
	refused := fmt.Errorf("opencode-go: http call: %w", errors.New("dial tcp 10.0.0.1:443: connect: connection refused"))
	stub := newModelStub(map[string]error{"cat-jam-free": refused})
	cbp := NewCircuitBreakingProvider(stub, health.BreakerConfig{FailureThreshold: 2, OpenTimeout: time.Hour})
	_ = callModel(cbp, "cat-jam-free")
	_ = callModel(cbp, "cat-jam-free")
	if got := cbp.Breaker().State(); got != health.StateOpen {
		t.Fatalf("provider breaker = %v after connection failures, want open", got)
	}
	err := callModel(cbp, "deepseek-v4.1-flash")
	if err == nil || !strings.Contains(err.Error(), "provider opencode-go: circuit open") {
		t.Fatalf("deepseek on an unreachable provider: err = %v, want the provider circuit open", err)
	}
	if n := stub.count("deepseek-v4.1-flash"); n != 0 {
		t.Fatalf("upstream saw %d deepseek calls through an open provider circuit", n)
	}
}

// A slow model is that model's problem: its deadline does not open the
// provider for its siblings.
func TestModelDeadlineIsModelScoped(t *testing.T) {
	slow := fmt.Errorf("opencode-go: http call: %w", context.DeadlineExceeded)
	stub := newModelStub(map[string]error{"longcat-2.5-preview-free": slow})
	cbp := NewCircuitBreakingProvider(stub, health.BreakerConfig{FailureThreshold: 2, OpenTimeout: time.Hour})
	_ = callModel(cbp, "longcat-2.5-preview-free")
	_ = callModel(cbp, "longcat-2.5-preview-free")
	if got := cbp.ModelBreaker("longcat-2.5-preview-free").State(); got != health.StateOpen {
		t.Fatalf("slow model's breaker = %v, want open", got)
	}
	if got := cbp.Breaker().State(); got != health.StateClosed {
		t.Fatalf("provider breaker = %v after one model's deadlines, want closed", got)
	}
	if err := callModel(cbp, "deepseek-v4.1-flash"); err != nil {
		t.Fatalf("deepseek after a sibling's deadlines: %v", err)
	}
}

// Operators see and reset model breakers by provider/model.
func TestBreakerRegistryReportsAndResetsModelBreakers(t *testing.T) {
	stub := newModelStub(map[string]error{"cat-jam-free": errors.New("opencode-go: status 429 Too Many Requests (sanitized)")})
	cbp := NewCircuitBreakingProvider(stub, health.BreakerConfig{FailureThreshold: 1, OpenTimeout: time.Hour})
	reg := NewBreakerRegistry()
	reg.RegisterProvider("opencode-go", cbp)
	_ = callModel(cbp, "cat-jam-free")
	_ = callModel(cbp, "deepseek-v4.1-flash")
	snap := reg.Snapshot()
	if s, ok := snap["opencode-go/cat-jam-free"]; !ok || s.State != health.StateOpen {
		t.Fatalf("snapshot = %+v; want opencode-go/cat-jam-free open", snap)
	}
	if s, ok := snap["opencode-go"]; !ok || s.State != health.StateClosed {
		t.Fatalf("snapshot = %+v; want the provider breaker closed", snap)
	}
	if !reg.Reset("opencode-go/cat-jam-free") {
		t.Fatal("reset of a model breaker was refused")
	}
	if got := cbp.ModelBreaker("cat-jam-free").State(); got != health.StateClosed {
		t.Fatalf("cat-jam breaker after reset = %v", got)
	}
	_ = callModel(cbp, "cat-jam-free")
	if !reg.Reset("opencode-go") || cbp.ModelBreaker("cat-jam-free").State() != health.StateClosed {
		t.Fatal("resetting the provider must reset its models too")
	}
	if reg.Reset("opencode-go/never-called") {
		t.Fatal("reset of an unknown model breaker reported success")
	}
}
