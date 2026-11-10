package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provider"
)

func waitUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

// gateProvider blocks each call until released, so a test can hold slots.
type gateProvider struct {
	name    string
	gate    chan struct{}
	calls   atomic.Int64
	panicOn bool
	stream  bool
}

func (g *gateProvider) Name() string { return g.name }
func (g *gateProvider) IsReal() bool { return true }
func (g *gateProvider) Call(ctx context.Context, req provider.LLMRequest) (*provider.LLMResponse, error) {
	g.calls.Add(1)
	if g.panicOn {
		panic("provider exploded")
	}
	select {
	case <-g.gate:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return &provider.LLMResponse{ID: "r", Text: "ok", Model: "m", StopReason: "end_turn", ProviderName: g.name}, nil
}
func (g *gateProvider) Stream(ctx context.Context, req provider.LLMRequest, onChunk func(provider.StreamChunk)) (*provider.LLMResponse, error) {
	resp, err := g.Call(ctx, req)
	if err == nil {
		onChunk(provider.StreamChunk{Type: "content_block_delta", Delta: "ok"})
	}
	return resp, err
}

// releaseOne lets one blocked provider call finish.
func releaseOne(t *testing.T, g *gateProvider) {
	t.Helper()
	select {
	case g.gate <- struct{}{}:
	case <-time.After(3 * time.Second):
		t.Fatal("no provider call was waiting to be released")
	}
}

func modelCallHandler(t *testing.T, p provider.Provider, limiter *PerAutoputerRateLimiter) (*Handler, *IdentityRegistry) {
	t.Helper()
	reg := NewIdentityRegistry(time.Hour)
	return NewHandlerWithRateLimit(reg, p, limiter), reg
}

func inferAs(h *Handler, raw string, stream bool) *httptest.ResponseRecorder {
	body := `{"messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}],"stream":` + map[bool]string{true: "true", false: "false"}[stream] + `}`
	req := httptest.NewRequest(http.MethodPost, "/provider/v1/inference", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+raw)
	w := httptest.NewRecorder()
	h.HandleInference(w, req)
	return w
}
