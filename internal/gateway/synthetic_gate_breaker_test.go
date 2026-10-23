package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/health"
	"github.com/yusefmosiah/go-choir/internal/provider"
)

// Staging 2026-10-23 (docs/problems/synthetic-provider-onboarding-2026-10-22.md,
// "Staging: concurrency 429s and the circuit breaker"): Synthetic answered 429
// "Too many concurrent requests" to 20 of 93 calls in 15 minutes, and those
// 429s opened the per-model breaker so later calls failed unsent. A
// concurrency 429 is not model ill-health, and the gateway should not send
// more than the account allows in the first place. Fake upstream below; the
// real SyntheticProvider and CircuitBreakingProvider are used unchanged.

const syntheticGateModel = "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017"

type syntheticFake struct {
	*httptest.Server
	limit                  int32
	inflight, peak         atomic.Int32
	calls, rejected, fives atomic.Int32
	fail5xx                atomic.Bool
}

func newSyntheticFake(t *testing.T, limit int32, hold time.Duration) *syntheticFake {
	t.Helper()
	f := &syntheticFake{limit: limit}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		f.calls.Add(1)
		n := f.inflight.Add(1)
		defer f.inflight.Add(-1)
		for {
			p := f.peak.Load()
			if n <= p || f.peak.CompareAndSwap(p, n) {
				break
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if f.fail5xx.Load() {
			f.fives.Add(1)
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		if n > f.limit {
			f.rejected.Add(1)
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = fmt.Fprint(w, `{"error":"Too many concurrent requests"}`)
			return
		}
		time.Sleep(hold)
		_, _ = fmt.Fprint(w, `{"id":"c","choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"ok"}}]}`)
	}))
	t.Cleanup(f.Close)
	return f
}

func breakerOverSynthetic(t *testing.T, f *syntheticFake, max string, threshold int) *CircuitBreakingProvider {
	t.Helper()
	t.Setenv("SYNTHETIC_MAX_CONCURRENT", max)
	t.Setenv("SYNTHETIC_MAX_WAIT", "30s")
	syn, err := provider.NewSyntheticProvider(provider.SyntheticConfig{APIKey: "syn-test-key", ModelID: syntheticGateModel, BaseURL: f.URL})
	if err != nil {
		t.Fatal(err)
	}
	return NewCircuitBreakingProvider(syn, health.BreakerConfig{FailureThreshold: threshold, OpenTimeout: time.Hour})
}

func synthCall(cbp *CircuitBreakingProvider) error {
	_, err := cbp.Call(context.Background(), provider.LLMRequest{
		Model:    syntheticGateModel,
		Messages: []provider.Message{{Role: "user", Content: []provider.Block{{Type: "text", Text: "hi"}}}},
	})
	return err
}

// M callers above the upstream's N all succeed through the gateway's provider
// stack: the gate holds the excess, the upstream never rejects, the breaker
// stays closed.
func TestSyntheticBurstAboveTheCeilingAllSucceeds(t *testing.T) {
	f := newSyntheticFake(t, 4, 30*time.Millisecond)
	cbp := breakerOverSynthetic(t, f, "4", 3)
	const callers = 20
	var wg sync.WaitGroup
	errs := make(chan error, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- synthCall(cbp) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("burst call failed: %v (rejected %d, peak %d)", err, f.rejected.Load(), f.peak.Load())
		}
	}
	if f.rejected.Load() != 0 || f.peak.Load() > 4 {
		t.Fatalf("upstream rejected %d, peak %d; the gate must hold the excess", f.rejected.Load(), f.peak.Load())
	}
	if got := cbp.ModelBreaker(syntheticGateModel).State(); got != health.StateClosed {
		t.Fatalf("model breaker = %v after a burst, want closed", got)
	}
}

// Even when 429s reach the gateway (the account ceiling is lower than the
// gate, as when another consumer shares the key), they do not open the
// breaker: a burst of them, then a good call is still sent.
func TestSyntheticConcurrency429DoesNotOpenTheBreaker(t *testing.T) {
	f := newSyntheticFake(t, 0, 0) // every call is 429
	cbp := breakerOverSynthetic(t, f, "8", 2)
	for i := 0; i < 6; i++ {
		err := synthCall(cbp)
		if err == nil || !strings.Contains(err.Error(), "status 429") {
			t.Fatalf("call %d: err = %v, want the sanitized 429", i, err)
		}
		if strings.Contains(err.Error(), "circuit open") {
			t.Fatalf("call %d: breaker opened on a concurrency 429 after %d upstream calls", i, f.calls.Load())
		}
	}
	if f.calls.Load() != 6 {
		t.Fatalf("upstream saw %d calls, want 6: an open breaker swallowed some", f.calls.Load())
	}
	if got := cbp.ModelBreaker(syntheticGateModel).State(); got != health.StateClosed {
		t.Fatalf("model breaker = %v after 429s, want closed", got)
	}
}

// What still counts: a 5xx from Synthetic opens the model breaker.
func TestSyntheticServerErrorsStillOpenTheBreaker(t *testing.T) {
	f := newSyntheticFake(t, 8, 0)
	f.fail5xx.Store(true)
	cbp := breakerOverSynthetic(t, f, "8", 2)
	for i := 0; i < 2; i++ {
		if err := synthCall(cbp); err == nil {
			t.Fatal("5xx call succeeded")
		}
	}
	if err := synthCall(cbp); err == nil || !strings.Contains(err.Error(), "circuit open") {
		t.Fatalf("after two 5xx: err = %v, want circuit open", err)
	}
	if f.calls.Load() != 2 {
		t.Fatalf("upstream calls = %d, want 2", f.calls.Load())
	}
}

// Other providers keep the designed behavior: an opencode-go style 429 still
// counts (circuit_breaker_client_error_test.go pins the table; this pins that
// the Synthetic exemption is typed, not a string match on "429").
func TestUntypedRateLimit429StillCounts(t *testing.T) {
	if !providerFailureCounts(context.Background(), errors.New("opencode-go: status 429 Too Many Requests (sanitized)")) {
		t.Fatal("an opencode-go 429 must still count toward the model breaker")
	}
	if providerFailureCounts(context.Background(), fmt.Errorf("synthetic: status 429 Too Many Requests (sanitized): %w", provider.ErrUpstreamConcurrencyLimit)) {
		t.Fatal("a typed concurrency limit must not count")
	}
	if providerFailureCounts(context.Background(), fmt.Errorf("synthetic: %w", provider.ErrConcurrencyGateWait)) {
		t.Fatal("a gate wait that never reached the upstream must not count")
	}
}

// /health reports the gate beside image_media.
func TestHealthReportsTheSyntheticGate(t *testing.T) {
	f := newSyntheticFake(t, 8, 0)
	cbp := breakerOverSynthetic(t, f, "5", 3)
	if err := synthCall(cbp); err != nil {
		t.Fatal(err)
	}
	h, _, _ := setupHandler(t)
	rec := httptest.NewRecorder()
	h.HandleHealth(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	gate, ok := out["synthetic_gate"].(map[string]any)
	if !ok {
		t.Fatalf("/health has no synthetic_gate: %s", rec.Body.String())
	}
	if got, _ := gate["limit"].(float64); got != 5 {
		t.Fatalf("synthetic_gate.limit = %v, want 5", gate["limit"])
	}
	for _, key := range []string{"in_flight", "peak", "granted", "waited", "wait_timeouts"} {
		if _, ok := gate[key].(float64); !ok {
			t.Fatalf("synthetic_gate.%s missing: %v", key, gate)
		}
	}
}
