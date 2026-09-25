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
