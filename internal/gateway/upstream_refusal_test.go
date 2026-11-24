package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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

// docs/problems/sub-call-spacer-outlived-its-bucket-2026-11-24.md (review
// round). With the guest's spacer gone, a burst meets the provider's
// refusals, and what the gateway says about a refusal decides whether the
// burst becomes a wall of blind retries. Ways it can fail, written first:
//   - an ordinary upstream 429 or a concurrency refusal is answered 502
//     with no Retry-After, so the guest retries it blind;
//   - one passing probe resets the cooldown and the burst flaps;
//   - a half-open probe of an ordinary failure has no deadline;
//   - the narrowing line is logged once per late failure, not once per open.

func postRefusalInference(t *testing.T, h *Handler, token, providerName, model string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(ProviderRequest{Provider: providerName, Model: model,
		Messages: []provider.Message{{Role: "user", Content: []provider.Block{{Type: "text", Text: "Hi"}}}}})
	req := httptest.NewRequest(http.MethodPost, "/provider/v1/inference", strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleInference(w, req)
	return w
}

func TestOrdinaryUpstreamRefusalIsAnswered429WithRetryAfter(t *testing.T) {
	for name, refusal := range map[string]error{
		"ordinary 429":      errors.New("opencode-go: status 429 Too Many Requests (sanitized)"),
		"concurrency limit": fmt.Errorf("synthetic: status 429 (concurrency limit, sanitized): %w", provider.ErrUpstreamConcurrencyLimit),
	} {
		t.Run(name, func(t *testing.T) {
			stub := &quotaStub{name: "synthetic"}
			stub.set(refusal)
			mp := provider.NewMultiProvider()
			mp.Register("synthetic", stub)
			mp = WrapMultiProvider(mp, health.BreakerConfig{FailureThreshold: 5, OpenTimeout: 30 * time.Second})
			reg := NewIdentityRegistry(time.Hour)
			h := NewMultiHandlerWithRateLimit(reg, mp, nil)
			cred, _ := reg.IssueCredential("autoputer-refusal")

			w := postRefusalInference(t, h, cred.RawToken, "synthetic", syntheticTestModel)
			var got ErrorResponse
			_ = json.Unmarshal(w.Body.Bytes(), &got)
			secs, _ := strconv.Atoi(w.Header().Get("Retry-After"))
			if w.Code != http.StatusTooManyRequests || got.Kind != ErrorKindUpstreamRateLimit || secs < 1 {
				t.Fatalf("%d %s Retry-After %q: want 429, kind %s, a positive Retry-After", w.Code, w.Body.String(), w.Header().Get("Retry-After"), ErrorKindUpstreamRateLimit)
			}
		})
	}

	// A model that is down stays an untyped 502.
	stub := &quotaStub{name: "synthetic"}
	stub.set(errors.New("opencode-go: status 503 Service Unavailable (sanitized)"))
	mp := provider.NewMultiProvider()
	mp.Register("synthetic", stub)
	reg := NewIdentityRegistry(time.Hour)
	h := NewMultiHandlerWithRateLimit(reg, WrapMultiProvider(mp, health.BreakerConfig{FailureThreshold: 5, OpenTimeout: 30 * time.Second}), nil)
	cred, _ := reg.IssueCredential("autoputer-down")
	if w := postRefusalInference(t, h, cred.RawToken, "synthetic", syntheticTestModel); w.Code != http.StatusBadGateway || w.Header().Get("Retry-After") != "" {
		t.Fatalf("a 503 answered %d with Retry-After %q, want an untyped 502", w.Code, w.Header().Get("Retry-After"))
	}
}

// syntheticTestModel is a catalogued Synthetic model.
const syntheticTestModel = "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017"

// gatedStub blocks every call until release is closed, then answers err.
type gatedStub struct {
	release chan struct{}
	started atomic.Int32
	mu      sync.Mutex
	err     error
	block   bool
}

func (s *gatedStub) Name() string { return "opencode-go" }
func (s *gatedStub) IsReal() bool { return true }
func (s *gatedStub) Call(ctx context.Context, req provider.LLMRequest) (*provider.LLMResponse, error) {
	s.started.Add(1)
	s.mu.Lock()
	block, err := s.block, s.err
	s.mu.Unlock()
	if block {
		select {
		case <-s.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if err != nil {
		return nil, err
	}
	return &provider.LLMResponse{Text: "ok"}, nil
}
func (s *gatedStub) Stream(ctx context.Context, req provider.LLMRequest, _ func(provider.StreamChunk)) (*provider.LLMResponse, error) {
	return s.Call(ctx, req)
}
func (s *gatedStub) set(block bool, err error) {
	s.mu.Lock()
	s.block, s.err = block, err
	s.mu.Unlock()
}

// 16 calls in flight together, the provider refuses them all: the breaker
// opens once, from inside the burst, one narrowing line says so, and a second
// burst is refused at the gateway without reaching the provider.
func TestBurstOpensTheBreakerOnceFromInsideTheBurst(t *testing.T) {
	const burst = 16
	var logs bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(prev)

	stub := &gatedStub{release: make(chan struct{})}
	stub.set(true, errors.New("opencode-go: status 429 Too Many Requests (sanitized)"))
	cbp := NewCircuitBreakingProvider(stub, health.BreakerConfig{FailureThreshold: 5, OpenTimeout: time.Hour})
	var wg sync.WaitGroup
	for i := 0; i < burst; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _ = callModel(cbp, "m") }()
	}
	for stub.started.Load() < burst {
		time.Sleep(time.Millisecond)
	}
	close(stub.release)
	wg.Wait()
	if n := strings.Count(logs.String(), "narrowed by a provider failure"); n != 1 {
		t.Fatalf("%d narrowing lines for one open, want 1:\n%s", n, logs.String())
	}
	if s := cbp.ModelBreaker("m").Snapshot(); s.Opens != 1 {
		t.Fatalf("the breaker opened %d times, want 1", s.Opens)
	}
	before := stub.started.Load()
	for i := 0; i < burst; i++ {
		if err := callModel(cbp, "m"); err == nil || !strings.Contains(err.Error(), "circuit open") {
			t.Fatalf("second burst call %d: %v, want circuit open", i, err)
		}
	}
	if stub.started.Load() != before {
		t.Fatal("the second burst reached the provider")
	}
}

// Every half-open probe has a deadline, not only a quota's: a probe that
// stalls must not hold the model shut for every computer until the
// inference timeout.
func TestEveryHalfOpenProbeHasADeadline(t *testing.T) {
	stub := &gatedStub{release: make(chan struct{})}
	stub.set(false, errors.New("opencode-go: status 429 Too Many Requests (sanitized)"))
	cbp := NewCircuitBreakingProvider(stub, health.BreakerConfig{FailureThreshold: 1, OpenTimeout: 30 * time.Millisecond})
	cbp.probeDeadline = 100 * time.Millisecond
	_ = callModel(cbp, "m") // opens
	time.Sleep(60 * time.Millisecond)
	stub.set(true, nil) // the probe stalls
	done := make(chan error, 1)
	go func() { done <- callModel(cbp, "m") }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("a stalled probe succeeded")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a stalled half-open probe of an ordinary failure ran with no deadline")
	}
}

// The gateway never promises a wait the guest would clamp: the computed
// Retry-After is capped at the guest's one-minute clamp, however long the
// model's breaker stays open.
func TestComputedRetryAfterIsCappedAtTheGuestsClamp(t *testing.T) {
	clock := &fakeClock{now: time.Unix(1000, 0)}
	stub := &quotaStub{name: "synthetic"}
	stub.set(errors.New("synthetic: status 429 Too Many Requests (sanitized)"))
	cbp := NewCircuitBreakingProvider(stub, health.BreakerConfig{FailureThreshold: 1, OpenTimeout: 10 * time.Minute, Now: clock.Now})
	_ = callModel(cbp, "m") // opens for ten minutes
	wait := cbp.refusalRetryAfter("m")
	if wait > time.Minute || wait < time.Second {
		t.Fatalf("computed Retry-After = %s, want between 1s and the guest's 1m clamp", wait)
	}
}

// A quota open and an ordinary 429 from a call that was already in flight are
// one open and one log line: the quota's.
func TestOneOpenIsOneLineAcrossQuotaAndOrdinaryFailures(t *testing.T) {
	var logs bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(prev)

	stub := &twoFacedStub{release: make(chan struct{}), inFlight: make(chan struct{}, 1)}
	cbp := NewCircuitBreakingProvider(stub, health.BreakerConfig{FailureThreshold: 5, OpenTimeout: time.Hour})
	done := make(chan struct{})
	go func() { _ = callModel(cbp, "m"); close(done) }() // call 1: held in flight, will fail 429
	<-stub.inFlight
	_ = callModel(cbp, "m") // call 2: a quota, opens the breaker and logs its line
	close(stub.release)
	<-done
	if n := strings.Count(logs.String(), "provider quota exhausted"); n != 1 {
		t.Fatalf("quota lines = %d, want 1:\n%s", n, logs.String())
	}
	if n := strings.Count(logs.String(), "narrowed by a provider failure"); n != 0 {
		t.Fatalf("a late ordinary 429 of the same open logged %d narrowing lines, want 0:\n%s", n, logs.String())
	}
}

// twoFacedStub's first call waits and then fails 429; its later calls are a
// spent quota.
type twoFacedStub struct {
	release  chan struct{}
	inFlight chan struct{}
	calls    atomic.Int32
}

func (s *twoFacedStub) Name() string { return "synthetic" }
func (s *twoFacedStub) IsReal() bool { return true }
func (s *twoFacedStub) Call(ctx context.Context, req provider.LLMRequest) (*provider.LLMResponse, error) {
	if s.calls.Add(1) == 1 {
		s.inFlight <- struct{}{}
		select {
		case <-s.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(30 * time.Second):
			return nil, errors.New("twoFacedStub: release never came")
		}
		return nil, errors.New("synthetic: status 429 Too Many Requests (sanitized)")
	}
	return nil, &provider.QuotaExhaustedError{Provider: "synthetic", Status: "429 Too Many Requests"}
}
func (s *twoFacedStub) Stream(ctx context.Context, req provider.LLMRequest, _ func(provider.StreamChunk)) (*provider.LLMResponse, error) {
	return s.Call(ctx, req)
}
