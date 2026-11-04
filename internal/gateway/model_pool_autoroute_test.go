package gateway

import (
	"context"
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

// Model-pool autoroute (docs/design/model-pool-autoroute-2026-11-02.md):
// the gateway's per-model breaker is each pool member's health, as the
// search plane's health rows are each search provider's. Ways it could
// fail, written before the code:
//   - a Synthetic quota 429 never opens the model (2026-11-01: 11,917 of
//     them, every run paying an 85 s retry first);
//   - a concurrency burst under the streak opens it (the onboarding rule);
//   - a streak that a success interrupted still opens it;
//   - the cooldown never grows, so a dead member is probed every 30 s
//     forever, or never resets, so a recovered member stays benched;
//   - in-flight failures that land after the open stack trips;
//   - a ChatGPT usage limit is treated like a one-off 429;
//   - one model's quota cools a sibling model on the same provider.

type quotaFake struct {
	*httptest.Server
	mode  atomic.Value // "quota", "concurrent", "ok"
	calls atomic.Int32
}

func newQuotaFake(t *testing.T, mode string) *quotaFake {
	t.Helper()
	f := &quotaFake{}
	f.mode.Store(mode)
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		f.calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		switch f.mode.Load().(string) {
		case "quota":
			// No reset time: the doubling quota cooldown applies (a known
			// reset time is provider_quota_wait_test.go's).
			w.Header().Set("x-synthetic-quotas", `{"subscription":{"limit":135,"requests":135},"planWeeklyLimit":{"percentRemaining":0}}`)
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = fmt.Fprint(w, `{"error":"You've exceeded your subscription rate limits. Upgrade, or try again later"}`)
		case "concurrent":
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = fmt.Fprint(w, `{"error":"Too many concurrent requests"}`)
		default:
			_, _ = fmt.Fprint(w, `{"id":"c","choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"ok"}}]}`)
		}
	}))
	t.Cleanup(f.Close)
	return f
}

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

func syntheticOver(t *testing.T, f *quotaFake, cfg health.BreakerConfig) *CircuitBreakingProvider {
	t.Helper()
	t.Setenv("SYNTHETIC_MAX_CONCURRENT", "8")
	t.Setenv("SYNTHETIC_MAX_WAIT", "30s")
	syn, err := provider.NewSyntheticProvider(provider.SyntheticConfig{APIKey: "syn-test-key", ModelID: syntheticGateModel, BaseURL: f.URL})
	if err != nil {
		t.Fatal(err)
	}
	return NewCircuitBreakingProvider(syn, cfg)
}

func TestSyntheticQuota429CoolsTheModelOnTheFirstOne(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 11, 2, 1, 16, 0, 0, time.UTC)}
	f := newQuotaFake(t, "quota")
	cbp := syntheticOver(t, f, health.BreakerConfig{FailureThreshold: 5, OpenTimeout: 30 * time.Second, Now: clock.Now})

	err := synthCall(cbp)
	if err == nil || !strings.Contains(err.Error(), "quota exhausted") {
		t.Fatalf("first call: err = %v, want the quota named", err)
	}
	err = synthCall(cbp)
	if err == nil || !strings.Contains(err.Error(), "circuit open") || strings.Contains(err.Error(), "429") {
		t.Fatalf("second call: err = %v, want a circuit open with no 429 text (the guest fails over at once on it)", err)
	}
	if got := f.calls.Load(); got != 1 {
		t.Fatalf("upstream calls = %d, want 1: a cooling member gets no calls", got)
	}
	snap := cbp.ModelBreaker(syntheticGateModel).Snapshot()
	if snap.State != health.StateOpen || snap.Trips != 1 || snap.OpenUntil == nil || !snap.OpenUntil.Equal(clock.Now().Add(defaultQuotaCooldownBase)) {
		t.Fatalf("snapshot = %+v, want open one trip for the quota base", snap)
	}

	// The probe at the end of the cooldown still finds the quota spent:
	// the cooldown doubles.
	clock.Advance(defaultQuotaCooldownBase)
	if err := synthCall(cbp); err == nil || !strings.Contains(err.Error(), "quota exhausted") {
		t.Fatalf("probe: err = %v, want the quota", err)
	}
	snap = cbp.ModelBreaker(syntheticGateModel).Snapshot()
	if snap.Trips != 2 || !snap.OpenUntil.Equal(clock.Now().Add(2*defaultQuotaCooldownBase)) {
		t.Fatalf("after a failed probe: %+v, want trip 2 for twice the base", snap)
	}

	// The owner tops up: the next probe succeeds and the member is back.
	f.mode.Store("ok")
	clock.Advance(2 * defaultQuotaCooldownBase)
	if err := synthCall(cbp); err != nil {
		t.Fatalf("probe after top-up: %v", err)
	}
	if snap := cbp.ModelBreaker(syntheticGateModel).Snapshot(); snap.State != health.StateClosed || snap.Trips != 0 {
		t.Fatalf("after a good probe: %+v, want closed with no trips", snap)
	}
}

func TestConcurrency429BurstUnderTheStreakDoesNotDemote(t *testing.T) {
	f := newQuotaFake(t, "concurrent")
	cbp := syntheticOver(t, f, health.BreakerConfig{FailureThreshold: 2, OpenTimeout: time.Hour})
	burst := func(n int) {
		t.Helper()
		for i := 0; i < n; i++ {
			err := synthCall(cbp)
			if err == nil || !strings.Contains(err.Error(), "concurrency limit") {
				t.Fatalf("burst call %d: err = %v, want the concurrency 429", i, err)
			}
		}
	}
	burst(ConcurrencyStreakBeforeCooldown - 1)
	if got := cbp.ModelBreaker(syntheticGateModel).State(); got != health.StateClosed {
		t.Fatalf("after %d concurrency 429s: %v, want closed", ConcurrencyStreakBeforeCooldown-1, got)
	}
	// A success ends the streak.
	f.mode.Store("ok")
	if err := synthCall(cbp); err != nil {
		t.Fatal(err)
	}
	f.mode.Store("concurrent")
	burst(ConcurrencyStreakBeforeCooldown - 1)
	if got := cbp.ModelBreaker(syntheticGateModel).State(); got != health.StateClosed {
		t.Fatalf("two bursts split by a success: %v, want closed", got)
	}
	// A streak with nothing getting through demotes.
	burst(1)
	if got := cbp.ModelBreaker(syntheticGateModel).State(); got != health.StateOpen {
		t.Fatalf("after %d in a row: %v, want open", ConcurrencyStreakBeforeCooldown, got)
	}
	if err := synthCall(cbp); err == nil || !strings.Contains(err.Error(), "circuit open") {
		t.Fatalf("after the streak: err = %v, want circuit open", err)
	}
}

func TestSuccessiveFailuresGrowTheCooldownAndASuccessRestores(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 11, 2, 2, 0, 0, 0, time.UTC)}
	stub := newModelStub(map[string]error{"m": fmt.Errorf("opencode-go: status 503 Service Unavailable (sanitized)")})
	cbp := NewCircuitBreakingProvider(stub, health.BreakerConfig{FailureThreshold: 2, OpenTimeout: 30 * time.Second, Now: clock.Now})
	b := cbp.ModelBreaker("m")

	_ = callModel(cbp, "m")
	_ = callModel(cbp, "m")
	if s := b.Snapshot(); s.State != health.StateOpen || !s.OpenUntil.Equal(clock.Now().Add(30*time.Second)) {
		t.Fatalf("after two 5xx: %+v, want open 30 s", s)
	}
	clock.Advance(30 * time.Second)
	_ = callModel(cbp, "m") // the probe fails
	if s := b.Snapshot(); s.Trips != 2 || !s.OpenUntil.Equal(clock.Now().Add(60*time.Second)) {
		t.Fatalf("after a failed probe: %+v, want trip 2 for 60 s", s)
	}
	clock.Advance(30 * time.Second)
	calls := stub.count("m")
	if err := callModel(cbp, "m"); err == nil || !strings.Contains(err.Error(), "circuit open") || stub.count("m") != calls {
		t.Fatalf("mid-cooldown: err = %v, upstream calls %d -> %d, want refused unsent", err, calls, stub.count("m"))
	}
	clock.Advance(30 * time.Second)
	stub.mu.Lock()
	delete(stub.errs, "m")
	stub.mu.Unlock()
	if err := callModel(cbp, "m"); err != nil {
		t.Fatalf("probe after the cooldown: %v", err)
	}
	if s := b.Snapshot(); s.State != health.StateClosed || s.Trips != 0 {
		t.Fatalf("after a good probe: %+v, want closed, trips reset", s)
	}
	// Reset means the next demotion starts from the base again.
	stub.mu.Lock()
	stub.errs["m"] = fmt.Errorf("opencode-go: status 503 Service Unavailable (sanitized)")
	stub.mu.Unlock()
	_ = callModel(cbp, "m")
	_ = callModel(cbp, "m")
	if s := b.Snapshot(); s.Trips != 1 || !s.OpenUntil.Equal(clock.Now().Add(30*time.Second)) {
		t.Fatalf("a new demotion: %+v, want trip 1 for 30 s", s)
	}
}

type chatgptStub struct{ *modelStub }

func (chatgptStub) Name() string { return "chatgpt" }

func TestChatGPTQuotaCoolsOnlyThatModel(t *testing.T) {
	quota := fmt.Errorf("chatgpt: quota exhausted: status 429 Too Many Requests (sanitized): %w", provider.ErrUpstreamQuotaExhausted)
	stub := chatgptStub{newModelStub(map[string]error{"gpt-6-luna": quota})}
	cbp := NewCircuitBreakingProvider(stub, health.BreakerConfig{FailureThreshold: 5, OpenTimeout: 30 * time.Second})
	if err := callModel(cbp, "gpt-6-luna"); err == nil || !strings.Contains(err.Error(), "quota exhausted") {
		t.Fatalf("first: %v", err)
	}
	// The refusal names the quota, not "model unhealthy"
	// (docs/problems/provider-quota-kills-desk-loops-2026-11-04.md).
	if err := callModel(cbp, "gpt-6-luna"); err == nil || !strings.Contains(err.Error(), "provider chatgpt model gpt-6-luna: provider quota exhausted (chatgpt)") ||
		!strings.Contains(err.Error(), "circuit open") || strings.Contains(err.Error(), "unhealthy") {
		t.Fatalf("second: %v, want the model's quota circuit open", err)
	}
	if stub.count("gpt-6-luna") != 1 {
		t.Fatalf("upstream calls = %d, want 1", stub.count("gpt-6-luna"))
	}
	if err := callModel(cbp, "gpt-6-sol"); err != nil {
		t.Fatalf("a sibling model was cooled too: %v", err)
	}
}

// Calls in flight when the model opens land their quota errors after it:
// they must not stack trips (8 slots would push a fresh cooldown to the cap).
func TestInFlightQuotaErrorsDoNotStackTrips(t *testing.T) {
	quota := fmt.Errorf("synthetic: quota exhausted: %w", provider.ErrUpstreamQuotaExhausted)
	stub := newModelStub(map[string]error{"m": quota})
	cbp := NewCircuitBreakingProvider(stub, health.BreakerConfig{FailureThreshold: 5, OpenTimeout: 30 * time.Second})
	b := cbp.ModelBreaker("m")
	for i := 0; i < 8; i++ {
		cbp.observeModelOutcome(context.Background(), "m", b, quota)
	}
	if s := b.Snapshot(); s.Trips != 1 {
		t.Fatalf("trips = %d after eight in-flight quota errors, want 1", s.Trips)
	}
}
