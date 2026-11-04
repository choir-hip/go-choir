package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/health"
	"github.com/yusefmosiah/go-choir/internal/provider"
)

// Provider quota as contention, gateway side
// (docs/problems/provider-quota-kills-desk-loops-2026-11-04.md). Ways it
// could fail, written down for these tests:
//   - the reset time is dropped, so the circuit half-opens at its own
//     backoff and the guest cannot say when the quota clears;
//   - a call the quota's circuit refuses reads "model unhealthy", and the
//     guest treats the quota as an outage;
//   - a far reset time keeps the model shut past an owner top-up;
//   - a model failure is typed as quota, and a dying model parks loops;
//   - the HTTP answer is untyped (502), so the guest cannot park on it;
//   - the stale quota hold outlives a later model failure.

type quotaStub struct {
	name  string
	mu    sync.Mutex
	err   error
	calls atomic.Int32
}

func (s *quotaStub) Call(ctx context.Context, req provider.LLMRequest) (*provider.LLMResponse, error) {
	s.calls.Add(1)
	s.mu.Lock()
	e := s.err
	s.mu.Unlock()
	if e != nil {
		return nil, e
	}
	return &provider.LLMResponse{Text: "ok"}, nil
}

func (s *quotaStub) Stream(ctx context.Context, req provider.LLMRequest, onChunk func(provider.StreamChunk)) (*provider.LLMResponse, error) {
	return s.Call(ctx, req)
}
func (s *quotaStub) Name() string { return s.name }
func (s *quotaStub) IsReal() bool { return true }

func (s *quotaStub) set(err error) {
	s.mu.Lock()
	s.err = err
	s.mu.Unlock()
}

func TestQuotaCircuitHalfOpensAtRetryAfter(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 11, 4, 2, 35, 23, 0, time.UTC)}
	stub := &quotaStub{name: "synthetic"}
	reset := clock.Now().Add(2 * time.Minute)
	stub.set(&provider.QuotaExhaustedError{Provider: "synthetic", Status: "429 Too Many Requests", RetryAt: reset})
	cbp := NewCircuitBreakingProvider(stub, health.BreakerConfig{FailureThreshold: 5, OpenTimeout: 30 * time.Second, Now: clock.Now})
	const model = "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017"

	_, err := cbp.Call(context.Background(), provider.LLMRequest{Model: model})
	q, ok := provider.AsQuotaExhausted(err)
	if !ok || q.Model != model || !q.RetryAt.Equal(reset) || q.RetryAtEstimated {
		t.Fatalf("first call: %v (%+v), want the typed quota with its reset time and model", err, q)
	}
	snap := cbp.ModelBreaker(model).Snapshot()
	if snap.State != health.StateOpen || snap.OpenUntil == nil || !snap.OpenUntil.Equal(reset) {
		t.Fatalf("snapshot = %+v, want open until the Retry-After time %s", snap, reset)
	}

	clock.Advance(time.Minute)
	_, err = cbp.Call(context.Background(), provider.LLMRequest{Model: model})
	q, ok = provider.AsQuotaExhausted(err)
	if !ok || !q.CircuitOpen || !q.RetryAt.Equal(reset) || strings.Contains(err.Error(), "unhealthy") || strings.Contains(err.Error(), "429") {
		t.Fatalf("refused call: %v, want the quota (circuit open, reset %s), never model unhealthy", err, reset)
	}
	if got := stub.calls.Load(); got != 1 {
		t.Fatalf("upstream calls = %d, want 1 while the quota circuit is open", got)
	}

	// At the reset time the circuit half-opens: the probe reaches the
	// upstream, succeeds, and the model is closed with no hold left.
	clock.Advance(time.Minute)
	stub.set(nil)
	if _, err := cbp.Call(context.Background(), provider.LLMRequest{Model: model}); err != nil {
		t.Fatalf("probe at the reset time: %v", err)
	}
	if snap := cbp.ModelBreaker(model).Snapshot(); snap.State != health.StateClosed {
		t.Fatalf("after the probe: %+v, want closed", snap)
	}
	if cbp.quotaRefusal(model) != nil {
		t.Fatal("the quota hold outlived the success")
	}
}

func TestQuotaFarResetIsCappedButReported(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 11, 4, 2, 35, 23, 0, time.UTC)}
	stub := &quotaStub{name: "synthetic"}
	reset := clock.Now().Add(3 * time.Hour)
	stub.set(&provider.QuotaExhaustedError{Provider: "synthetic", Status: "429 Too Many Requests", RetryAt: reset})
	cbp := NewCircuitBreakingProvider(stub, health.BreakerConfig{FailureThreshold: 5, OpenTimeout: 30 * time.Second, Now: clock.Now})
	_, _ = cbp.Call(context.Background(), provider.LLMRequest{Model: "m"})
	snap := cbp.ModelBreaker("m").Snapshot()
	if snap.OpenUntil == nil || !snap.OpenUntil.Equal(clock.Now().Add(defaultQuotaCooldownMax)) {
		t.Fatalf("snapshot = %+v, want open for the quota cooldown's maximum (a top-up is seen within it)", snap)
	}
	_, err := cbp.Call(context.Background(), provider.LLMRequest{Model: "m"})
	if q, ok := provider.AsQuotaExhausted(err); !ok || !q.RetryAt.Equal(reset) {
		t.Fatalf("refused call: %v, want the provider's own reset time %s", err, reset)
	}
}

func TestQuotaWithoutResetTimeIsEstimated(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 11, 4, 2, 35, 23, 0, time.UTC)}
	stub := &quotaStub{name: "synthetic"}
	stub.set(&provider.QuotaExhaustedError{Provider: "synthetic", Status: "429 Too Many Requests"})
	cbp := NewCircuitBreakingProvider(stub, health.BreakerConfig{FailureThreshold: 5, OpenTimeout: 30 * time.Second, Now: clock.Now})
	_, err := cbp.Call(context.Background(), provider.LLMRequest{Model: "m"})
	q, ok := provider.AsQuotaExhausted(err)
	want := clock.Now().Add(defaultQuotaCooldownBase)
	if !ok || !q.RetryAtEstimated || !q.RetryAt.Equal(want) {
		t.Fatalf("first call: %v (%+v), want the cooldown's end %s as an estimate", err, q, want)
	}
	// Past the estimate the half-open probe is in flight elsewhere: a
	// refused call is told to ask again shortly, never a past time.
	clock.Advance(defaultQuotaCooldownBase)
	b := cbp.ModelBreaker("m")
	allowed, done := b.Allow() // another caller holds the probe
	if !allowed {
		t.Fatal("no probe slot at the end of the cooldown")
	}
	_, err = cbp.Call(context.Background(), provider.LLMRequest{Model: "m"})
	done(nil)
	if q, ok := provider.AsQuotaExhausted(err); !ok || !q.RetryAt.After(clock.Now()) {
		t.Fatalf("refused during the probe: %v, want the quota with a future time", err)
	}
}

func TestModelFailureKeepsTodaysBehaviour(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 11, 4, 2, 35, 23, 0, time.UTC)}
	stub := &quotaStub{name: "synthetic"}
	cbp := NewCircuitBreakingProvider(stub, health.BreakerConfig{FailureThreshold: 2, OpenTimeout: 30 * time.Second, Now: clock.Now})
	// A quota first, then the model fails for real after the reset: the
	// open breaker is the failure's, not the quota's.
	stub.set(&provider.QuotaExhaustedError{Provider: "synthetic", Status: "429 Too Many Requests", RetryAt: clock.Now().Add(time.Minute)})
	_, _ = cbp.Call(context.Background(), provider.LLMRequest{Model: "m"})
	clock.Advance(time.Minute)
	stub.set(errors.New("synthetic: status 503 Service Unavailable (sanitized)"))
	for i := 0; i < 3; i++ {
		_, _ = cbp.Call(context.Background(), provider.LLMRequest{Model: "m"})
	}
	_, err := cbp.Call(context.Background(), provider.LLMRequest{Model: "m"})
	if err == nil || !strings.Contains(err.Error(), "circuit open (model unhealthy)") || provider.IsQuotaExhausted(err) {
		t.Fatalf("model failure: %v, want the untyped model-unhealthy circuit", err)
	}
}

// The HTTP answer: 429, Retry-After, and the typed body the guest parks on.
// A model failure stays a 502 with no kind.
func TestInferenceAnswersQuotaTyped(t *testing.T) {
	const model = "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017"
	reset := time.Now().Add(32 * time.Minute).UTC().Truncate(time.Second)
	mode := "quota"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if mode == "quota" {
			w.Header().Set("x-synthetic-quotas", fmt.Sprintf(`{"subscription":{"limit":500,"requests":0},"planWeeklyLimit":{"percentRemaining":0,"nextRegenAt":%q}}`, reset.Format(time.RFC3339)))
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = fmt.Fprint(w, `{"error":"You've exceeded your subscription rate limits. Upgrade, or try again later."}`)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprint(w, `{"error":"down"}`)
	}))
	defer server.Close()
	syn, err := provider.NewSyntheticProvider(provider.SyntheticConfig{APIKey: "syn-test-key", ModelID: model, BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	mp := provider.NewMultiProvider()
	mp.Register("synthetic", syn)
	mp = WrapMultiProvider(mp, health.BreakerConfig{FailureThreshold: 5, OpenTimeout: 30 * time.Second})
	reg := NewIdentityRegistry(time.Hour)
	h := NewMultiHandlerWithRateLimit(reg, mp, nil)
	cred, _ := reg.IssueCredential("autoputer-quota")
	post := func(m string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(ProviderRequest{Provider: "synthetic", Model: m,
			Messages: []provider.Message{{Role: "user", Content: []provider.Block{{Type: "text", Text: "Hi"}}}}})
		req := httptest.NewRequest(http.MethodPost, "/provider/v1/inference", strings.NewReader(string(body)))
		req.Header.Set("Authorization", "Bearer "+cred.RawToken)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.HandleInference(w, req)
		return w
	}
	for _, call := range []string{"upstream quota", "refused by the quota circuit"} {
		w := post(model)
		var got ErrorResponse
		_ = json.Unmarshal(w.Body.Bytes(), &got)
		if w.Code != http.StatusTooManyRequests || got.Kind != ErrorKindProviderQuota || got.Provider != "synthetic" || got.Model != model ||
			got.Reason != "provider quota exhausted (synthetic)" || got.RetryAt != reset.Format(time.RFC3339) || got.RetryAtEstimated {
			t.Fatalf("%s: %d %s, want 429 provider_quota with the reset time", call, w.Code, w.Body.String())
		}
		secs, _ := strconv.Atoi(w.Header().Get("Retry-After"))
		if secs < 30*60 || secs > 33*60 {
			t.Fatalf("%s: Retry-After = %q, want about 32 min", call, w.Header().Get("Retry-After"))
		}
		if strings.Contains(w.Body.String(), "unhealthy") {
			t.Fatalf("%s: %s names the quota model ill-health", call, w.Body.String())
		}
	}

	// A model failure is not a quota: 502, untyped.
	mode = "down"
	mp2 := provider.NewMultiProvider()
	mp2.Register("synthetic", syn)
	h = NewMultiHandlerWithRateLimit(reg, WrapMultiProvider(mp2, health.BreakerConfig{FailureThreshold: 5, OpenTimeout: 30 * time.Second}), nil)
	w := post(model)
	var got ErrorResponse
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if w.Code != http.StatusBadGateway || got.Kind != "" || w.Header().Get("Retry-After") != "" {
		t.Fatalf("model failure: %d %s, want an untyped 502", w.Code, w.Body.String())
	}
}

// Review r1 (DeepSeek B1): a success or a model failure from a call that
// was in flight when the quota tripped must not drop the hold while the
// breaker stays open, or every refusal until the reset reads "model
// unhealthy" and a parked loop dies on it.
func TestInFlightOutcomesKeepTheQuotaHold(t *testing.T) {
	for name, inflight := range map[string]error{
		"success": nil,
		"failure": errors.New("synthetic: status 503 Service Unavailable (sanitized)"),
	} {
		t.Run(name, func(t *testing.T) {
			clock := &fakeClock{now: time.Date(2026, 11, 4, 2, 35, 23, 0, time.UTC)}
			stub := &quotaStub{name: "synthetic"}
			reset := clock.Now().Add(32 * time.Minute)
			stub.set(&provider.QuotaExhaustedError{Provider: "synthetic", Status: "429 Too Many Requests", RetryAt: reset})
			cbp := NewCircuitBreakingProvider(stub, health.BreakerConfig{FailureThreshold: 5, OpenTimeout: 30 * time.Second, Now: clock.Now})
			b := cbp.ModelBreaker("m")
			allowed, done := b.Allow() // admitted before the trip
			if !allowed {
				t.Fatal("not admitted")
			}
			_, _ = cbp.Call(context.Background(), provider.LLMRequest{Model: "m"})
			done(inflight)
			cbp.observeModelOutcome(context.Background(), "m", b, inflight)
			clock.Advance(30 * time.Minute)
			_, err := cbp.Call(context.Background(), provider.LLMRequest{Model: "m"})
			if q, ok := provider.AsQuotaExhausted(err); !ok || !q.CircuitOpen || strings.Contains(err.Error(), "unhealthy") {
				t.Fatalf("refusal after an in-flight %s: %v, want the typed quota", name, err)
			}
		})
	}
}

// Review r1 (DeepSeek B4): a quota with no reset time from a call in flight
// when a model failure opened the breaker does not relabel the failure.
func TestInFlightQuotaDoesNotRelabelAModelFailure(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 11, 4, 2, 35, 23, 0, time.UTC)}
	stub := &quotaStub{name: "synthetic"}
	stub.set(errors.New("synthetic: status 503 Service Unavailable (sanitized)"))
	cbp := NewCircuitBreakingProvider(stub, health.BreakerConfig{FailureThreshold: 1, OpenTimeout: 30 * time.Second, Now: clock.Now})
	_, _ = cbp.Call(context.Background(), provider.LLMRequest{Model: "m"})
	b := cbp.ModelBreaker("m")
	cbp.observeModelOutcome(context.Background(), "m", b, &provider.QuotaExhaustedError{Provider: "synthetic", Status: "429 Too Many Requests"})
	_, err := cbp.Call(context.Background(), provider.LLMRequest{Model: "m"})
	if provider.IsQuotaExhausted(err) || !strings.Contains(err.Error(), "model unhealthy") {
		t.Fatalf("refusal: %v, want the model failure's circuit", err)
	}
}
