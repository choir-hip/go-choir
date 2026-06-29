package gateway

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/health"
	"github.com/yusefmosiah/go-choir/internal/provider"
)

type stubProvider struct {
	name   string
	real   bool
	callFn func(ctx context.Context, req provider.LLMRequest) (*provider.LLMResponse, error)
}

func (s *stubProvider) Call(ctx context.Context, req provider.LLMRequest) (*provider.LLMResponse, error) {
	if s.callFn == nil {
		return &provider.LLMResponse{Text: "ok"}, nil
	}
	return s.callFn(ctx, req)
}

func (s *stubProvider) Stream(ctx context.Context, req provider.LLMRequest, onChunk func(provider.StreamChunk)) (*provider.LLMResponse, error) {
	return s.Call(ctx, req)
}

func (s *stubProvider) Name() string { return s.name }
func (s *stubProvider) IsReal() bool { return s.real }

func TestCircuitBreakingProvider_ForwardsWhenClosed(t *testing.T) {
	p := &stubProvider{name: "stub", real: true}
	cbp := NewCircuitBreakingProvider(p, health.BreakerConfig{FailureThreshold: 3, OpenTimeout: time.Hour})
	resp, err := cbp.Call(context.Background(), provider.LLMRequest{Model: "m"})
	if err != nil {
		t.Fatalf("Call error: %v", err)
	}
	if resp.Text != "ok" {
		t.Fatalf("Text = %q", resp.Text)
	}
	if cbp.Breaker().State() != health.StateClosed {
		t.Fatalf("state = %v, want closed", cbp.Breaker().State())
	}
}

func TestCircuitBreakingProvider_OpensOnFailures(t *testing.T) {
	p := &stubProvider{name: "stub", real: true, callFn: func(ctx context.Context, req provider.LLMRequest) (*provider.LLMResponse, error) {
		return nil, errors.New("upstream 503")
	}}
	cbp := NewCircuitBreakingProvider(p, health.BreakerConfig{FailureThreshold: 2, OpenTimeout: time.Hour})
	_, _ = cbp.Call(context.Background(), provider.LLMRequest{})
	_, _ = cbp.Call(context.Background(), provider.LLMRequest{})
	if cbp.Breaker().State() != health.StateOpen {
		t.Fatalf("state = %v, want open", cbp.Breaker().State())
	}
	_, err := cbp.Call(context.Background(), provider.LLMRequest{})
	if err == nil {
		t.Fatal("expected circuit-open error, got nil")
	}
}

func TestCircuitBreakingProvider_NameAndIsReal(t *testing.T) {
	p := &stubProvider{name: "stub", real: true}
	cbp := NewCircuitBreakingProvider(p, health.BreakerConfig{})
	if cbp.Name() != "stub" {
		t.Fatalf("Name = %q", cbp.Name())
	}
	if !cbp.IsReal() {
		t.Fatal("IsReal = false, want true")
	}
}

func TestWrapMultiProvider(t *testing.T) {
	mp := provider.NewMultiProvider()
	mp.Register("a", &stubProvider{name: "a"})
	mp.Register("b", &stubProvider{name: "b"})
	wrapped := WrapMultiProvider(mp, health.BreakerConfig{FailureThreshold: 1, OpenTimeout: time.Hour})
	for _, name := range wrapped.Names() {
		if _, ok := wrapped.Get(name).(*CircuitBreakingProvider); !ok {
			t.Fatalf("provider %q not wrapped", name)
		}
	}
}

func TestBreakerRegistry(t *testing.T) {
	r := NewBreakerRegistry()
	b := health.NewCircuitBreaker(health.BreakerConfig{FailureThreshold: 1, OpenTimeout: time.Hour})
	r.Register("p", b)
	if len(r.Names()) != 1 {
		t.Fatalf("Names len = %d, want 1", len(r.Names()))
	}
	snap := r.Snapshot()
	if _, ok := snap["p"]; !ok {
		t.Fatal("Snapshot missing p")
	}
	if !r.Reset("p") {
		t.Fatal("Reset returned false for known provider")
	}
	if r.Reset("unknown") {
		t.Fatal("Reset returned true for unknown provider")
	}
}
