package gateway

import (
	"context"
	"fmt"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/health"
	"github.com/yusefmosiah/go-choir/internal/provider"
)

// CircuitBreakingProvider wraps a provider.Provider with a per-provider circuit
// breaker. When the breaker is open, Call and Stream return a sanitized
// ErrCircuitOpen without contacting the upstream. This prevents the gateway
// from retrying a failing LLM provider endlessly (production-readiness
// checklist: "LLM provider failures circuit-break (not retry endlessly)").
//
// The decorator is transparent when the upstream is healthy: a closed breaker
// forwards every call and records the outcome. It only changes behavior after
// repeated failures, which is strictly an improvement over the prior behavior
// of forwarding into a failing upstream on every request.
type CircuitBreakingProvider struct {
	inner   provider.Provider
	breaker *health.CircuitBreaker
}

// NewCircuitBreakingProvider wraps p with a circuit breaker using cfg.
func NewCircuitBreakingProvider(p provider.Provider, cfg health.BreakerConfig) *CircuitBreakingProvider {
	return &CircuitBreakingProvider{
		inner:   p,
		breaker: health.NewCircuitBreaker(cfg),
	}
}

// Breaker returns the underlying circuit breaker for observability and admin
// reset endpoints.
func (c *CircuitBreakingProvider) Breaker() *health.CircuitBreaker { return c.breaker }

// Name returns the wrapped provider's name.
func (c *CircuitBreakingProvider) Name() string { return c.inner.Name() }

// IsReal returns the wrapped provider's realness.
func (c *CircuitBreakingProvider) IsReal() bool { return c.inner.IsReal() }

// Call executes the LLM request through the circuit breaker.
func (c *CircuitBreakingProvider) Call(ctx context.Context, req provider.LLMRequest) (*provider.LLMResponse, error) {
	var resp *provider.LLMResponse
	err := c.breaker.Execute(func() error {
		var callErr error
		resp, callErr = c.inner.Call(ctx, req)
		return callErr
	})
	if err == health.ErrCircuitOpen {
		return nil, fmt.Errorf("provider %s: circuit open (upstream unhealthy)", c.inner.Name())
	}
	return resp, err
}

// Stream executes the streaming LLM request through the circuit breaker. The
// breaker records the outcome based on whether the stream completed without
// error; per-chunk failures are surfaced to onChunk callers but the breaker
// only transitions on the terminal Stream result.
func (c *CircuitBreakingProvider) Stream(ctx context.Context, req provider.LLMRequest, onChunk func(provider.StreamChunk)) (*provider.LLMResponse, error) {
	var resp *provider.LLMResponse
	err := c.breaker.Execute(func() error {
		var streamErr error
		resp, streamErr = c.inner.Stream(ctx, req, onChunk)
		return streamErr
	})
	if err == health.ErrCircuitOpen {
		return nil, fmt.Errorf("provider %s: circuit open (upstream unhealthy)", c.inner.Name())
	}
	return resp, err
}

// WrapMultiProvider returns a new MultiProvider where every registered provider
// is wrapped with a CircuitBreakingProvider using cfg. Providers registered
// after wrapping are NOT wrapped; call this once after ResolveAll.
func WrapMultiProvider(mp *provider.MultiProvider, cfg health.BreakerConfig) *provider.MultiProvider {
	out := provider.NewMultiProvider()
	for _, name := range mp.Names() {
		p := mp.Get(name)
		if p == nil {
			continue
		}
		out.Register(name, NewCircuitBreakingProvider(p, cfg))
	}
	return out
}

// BreakerRegistry tracks the circuit breakers for each wrapped provider so the
// gateway can expose their state via health/ops endpoints and reset them.
type BreakerRegistry struct {
	mu       sync.Mutex
	breakers map[string]*health.CircuitBreaker
}

// NewBreakerRegistry returns an empty registry.
func NewBreakerRegistry() *BreakerRegistry {
	return &BreakerRegistry{breakers: make(map[string]*health.CircuitBreaker)}
}

// Register associates a breaker with a provider name.
func (r *BreakerRegistry) Register(name string, b *health.CircuitBreaker) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.breakers[name] = b
}

// Snapshot returns the current breaker state for every registered provider.
func (r *BreakerRegistry) Snapshot() map[string]health.Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]health.Snapshot, len(r.breakers))
	for name, b := range r.breakers {
		out[name] = b.Snapshot()
	}
	return out
}

// Reset clears the breaker for the named provider, returning it to closed.
func (r *BreakerRegistry) Reset(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.breakers[name]
	if !ok {
		return false
	}
	b.Reset()
	return true
}

// Names returns the registered provider names.
func (r *BreakerRegistry) Names() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	names := make([]string, 0, len(r.breakers))
	for n := range r.breakers {
		names = append(names, n)
	}
	return names
}

// breakerFor returns the circuit breaker registered for the named provider, or
// nil when no breaker is registered. It is used by the per-service health
// endpoint to surface the breaker state alongside the dependency probe
// (M22b / C20). The lookup is case-sensitive; provider names are registered
// by the gateway entry point.
func (r *BreakerRegistry) breakerFor(name string) *health.CircuitBreaker {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.breakers[name]
}
