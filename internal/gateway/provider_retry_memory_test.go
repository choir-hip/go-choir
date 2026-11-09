package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/health"
)

// The gateway does not keep asking an upstream that is down (owner,
// 2026-11-09). Ways it could fail, written down before the change:
//  1. a model that keeps failing is probed on a fixed beat: the open interval
//     does not double on each failed probe;
//  2. a refused key (401) or a model that is gone (model_not_found) counts as a
//     client error and opens nothing, so every call keeps hitting it;
//  3. a hard failure is held only on the transient schedule (10 min cap);
//  4. a hard failure of one model opens its siblings;
//  5. a caller that does not go through a model breaker (the Jev judgment
//     endpoint) calls an upstream that just failed, on every request;
//  6. a Jev hold never ends on a success or grows without bound.

type fakeBreakerClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeBreakerClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeBreakerClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

func newFakeBreakerClock() *fakeBreakerClock {
	return &fakeBreakerClock{now: time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)}
}

func TestModelBreakerOpenIntervalDoublesOnEachFailedProbe(t *testing.T) {
	clock := newFakeBreakerClock()
	stub := newModelStub(map[string]error{"m": errors.New("opencode-go: status 503 Service Unavailable (sanitized)")})
	cbp := NewCircuitBreakingProvider(stub, health.BreakerConfig{FailureThreshold: 1, OpenTimeout: 30 * time.Second, Now: clock.Now})
	_ = callModel(cbp, "m") // opens for 30s
	calls := stub.count("m")
	for i, open := range []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute} {
		// Just before the interval ends the circuit refuses the call.
		clock.Advance(open - time.Second)
		if err := callModel(cbp, "m"); err == nil || stub.count("m") != calls {
			t.Fatalf("probe %d: the circuit let a call through %s into a %s interval (%v)", i+1, open-time.Second, open, err)
		}
		// Just after it, one probe goes through and fails, which doubles the interval.
		clock.Advance(2 * time.Second)
		_ = callModel(cbp, "m")
		calls++
		if stub.count("m") != calls {
			t.Fatalf("probe %d: no probe after the interval", i+1)
		}
	}
}

func TestHardFailureOpensTheModelAtOnceForALongDoublingHold(t *testing.T) {
	clock := newFakeBreakerClock()
	stub := newModelStub(map[string]error{"gone": errors.New("opencode-go: status 404 model_not_found (sanitized)")})
	cbp := NewCircuitBreakingProvider(stub, health.BreakerConfig{FailureThreshold: 5, OpenTimeout: 30 * time.Second, Now: clock.Now})
	if err := callModel(cbp, "gone"); err == nil {
		t.Fatal("want the upstream error")
	}
	// One failure is enough, and a sibling model is untouched.
	if err := callModel(cbp, "gone"); err == nil || stub.count("gone") != 1 {
		t.Fatalf("a second call reached a model that was gone (calls %d, err %v)", stub.count("gone"), err)
	}
	if err := callModel(cbp, "other"); err != nil {
		t.Fatalf("a sibling model was held: %v", err)
	}
	// Held 15 minutes, not the transient schedule's 30 s.
	clock.Advance(14 * time.Minute)
	_ = callModel(cbp, "gone")
	if stub.count("gone") != 1 {
		t.Fatal("probed inside the 15 minute hard hold")
	}
	clock.Advance(2 * time.Minute)
	_ = callModel(cbp, "gone") // the probe fails again: the hold doubles to 30 minutes
	if stub.count("gone") != 2 {
		t.Fatalf("no probe after the hold (calls %d)", stub.count("gone"))
	}
	clock.Advance(29 * time.Minute)
	_ = callModel(cbp, "gone")
	if stub.count("gone") != 2 {
		t.Fatal("the second hard hold was not longer than the first")
	}
	// A bad key is the same.
	stub.errs["badkey"] = errors.New("opencode-go: status 401 Unauthorized (sanitized)")
	_ = callModel(cbp, "badkey")
	_ = callModel(cbp, "badkey")
	if stub.count("badkey") != 1 {
		t.Fatalf("a refused key was retried at once (%d calls)", stub.count("badkey"))
	}
}

func TestOrdinaryBadRequestStillOpensNothing(t *testing.T) {
	stub := newModelStub(map[string]error{"m": errors.New("opencode-go: status 400 Bad Request (sanitized)")})
	cbp := NewCircuitBreakingProvider(stub, health.BreakerConfig{FailureThreshold: 1, OpenTimeout: time.Hour})
	for i := 0; i < 3; i++ {
		_ = callModel(cbp, "m")
	}
	if stub.count("m") != 3 {
		t.Fatalf("a 400 held the model (%d calls)", stub.count("m"))
	}
}

func TestJevDoesNotCallAnUpstreamThatJustFailed(t *testing.T) {
	var calls int
	var mu sync.Mutex
	status := http.StatusBadGateway
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		s := status
		mu.Unlock()
		if s != http.StatusOK {
			w.WriteHeader(s)
			return
		}
		_, _ = w.Write([]byte(`{"id":"x","model":"m"}`))
	}))
	defer upstream.Close()
	clock := newFakeBreakerClock()
	tr := NewJevTransportFromEnv(true)
	tr.apiKey, tr.url, tr.now = "k", upstream.URL, clock.Now
	tr.receipts = newJevReceiptWriter(filepath.Join(t.TempDir(), "r.jsonl"))
	req := JudgmentRequest{State: json.RawMessage(`{}`), Questions: map[string]json.RawMessage{"q": json.RawMessage(`{"type":"choice","choices":["a","b"]}`)}}
	ask := func() error { _, err := tr.Decide(context.Background(), "c1", req); return err }
	count := func() int { mu.Lock(); defer mu.Unlock(); return calls }

	if err := ask(); err == nil {
		t.Fatal("want the upstream failure")
	}
	for i := 0; i < 5; i++ {
		if err := ask(); err == nil {
			t.Fatal("a held upstream answered")
		}
	}
	if count() != 1 {
		t.Fatalf("upstream asked %d times inside its hold, want 1", count())
	}
	// The hold ends and doubles: a second failure holds longer.
	clock.Advance(2 * time.Minute)
	_ = ask()
	if count() != 2 {
		t.Fatalf("no probe after the hold (%d)", count())
	}
	clock.Advance(90 * time.Second) // inside the doubled hold (2 min of 1m*2)
	_ = ask()
	if count() != 2 {
		t.Fatalf("the doubled hold was not kept (%d)", count())
	}
	// A success ends the hold and its strikes.
	mu.Lock()
	status = http.StatusOK
	mu.Unlock()
	clock.Advance(5 * time.Minute)
	if err := ask(); err != nil {
		t.Fatalf("probe after the hold: %v", err)
	}
	if err := ask(); err != nil {
		t.Fatalf("a recovered upstream stayed held: %v", err)
	}
}

func TestJevAuthFailureIsHeldLong(t *testing.T) {
	var calls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer upstream.Close()
	clock := newFakeBreakerClock()
	tr := NewJevTransportFromEnv(true)
	tr.apiKey, tr.url, tr.now = "k", upstream.URL, clock.Now
	tr.receipts = newJevReceiptWriter(filepath.Join(t.TempDir(), "r.jsonl"))
	req := JudgmentRequest{State: json.RawMessage(`{}`), Questions: map[string]json.RawMessage{"q": json.RawMessage(`{"type":"choice","choices":["a","b"]}`)}}
	_, _ = tr.Decide(context.Background(), "c1", req)
	clock.Advance(14 * time.Minute)
	_, _ = tr.Decide(context.Background(), "c1", req)
	if calls != 1 {
		t.Fatalf("a refused key was retried inside 15 minutes (%d calls)", calls)
	}
}

// TestBareForbiddenOrNotFoundDoesNotHoldTheSharedModel (Devin r1): a 403 or
// 404 can be request-shaped, so it must not close a model for every computer.
func TestBareForbiddenOrNotFoundDoesNotHoldTheSharedModel(t *testing.T) {
	clock := newFakeBreakerClock()
	stub := newModelStub(map[string]error{
		"policy": errors.New("opencode-go: status 403 Forbidden (sanitized)"),
		"path":   errors.New("opencode-go: status 404 Not Found (sanitized)"),
	})
	cbp := NewCircuitBreakingProvider(stub, health.BreakerConfig{FailureThreshold: 5, OpenTimeout: 30 * time.Second, Now: clock.Now})
	for _, m := range []string{"policy", "path"} {
		_ = callModel(cbp, m)
		_ = callModel(cbp, m)
		if stub.count(m) != 2 {
			t.Fatalf("a bare %s held the shared model (calls %d)", m, stub.count(m))
		}
	}
}
