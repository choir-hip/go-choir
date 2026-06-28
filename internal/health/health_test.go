package health

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestCheckerFunc(t *testing.T) {
	c := CheckerFunc{NameStr: "dep", Fn: func(ctx context.Context) error { return nil }}
	if c.Name() != "dep" {
		t.Fatalf("Name = %q, want %q", c.Name(), "dep")
	}
	if err := c.Check(context.Background()); err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
}

func TestHTTPChecker_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	c := HTTPChecker{NameStr: "upstream", URL: srv.URL, Timeout: time.Second}
	if err := c.Check(context.Background()); err != nil {
		t.Fatalf("Check error: %v", err)
	}
}

func TestHTTPChecker_Non2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	c := HTTPChecker{NameStr: "upstream", URL: srv.URL, Timeout: time.Second}
	if err := c.Check(context.Background()); err == nil {
		t.Fatal("expected error for non-2xx, got nil")
	}
}

func TestHTTPChecker_Unreachable(t *testing.T) {
	c := HTTPChecker{NameStr: "dead", URL: "http://127.0.0.1:1", Timeout: 100 * time.Millisecond}
	if err := c.Check(context.Background()); err == nil {
		t.Fatal("expected error for unreachable host, got nil")
	}
}

func TestTCPChecker_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	c := TCPChecker{NameStr: "tcp", Address: srv.Listener.Addr().String(), Timeout: time.Second}
	if err := c.Check(context.Background()); err != nil {
		t.Fatalf("Check error: %v", err)
	}
}

func TestTCPChecker_Unreachable(t *testing.T) {
	c := TCPChecker{NameStr: "dead-tcp", Address: "127.0.0.1:1", Timeout: 100 * time.Millisecond}
	if err := c.Check(context.Background()); err == nil {
		t.Fatal("expected error for unreachable tcp, got nil")
	}
}

func TestAggregator_AllHealthy(t *testing.T) {
	agg := NewAggregator("svc", 0,
		CheckerFunc{NameStr: "a", Fn: func(ctx context.Context) error { return nil }},
		CheckerFunc{NameStr: "b", Fn: func(ctx context.Context) error { return nil }},
	)
	resp := agg.RefreshIfStale()
	if resp.Status != string(StatusOK) {
		t.Fatalf("Status = %q, want ok", resp.Status)
	}
	if len(resp.Dependencies) != 2 {
		t.Fatalf("Dependencies len = %d, want 2", len(resp.Dependencies))
	}
	if resp.Dependencies["a"].Status != StatusOK {
		t.Errorf("dep a status = %q, want ok", resp.Dependencies["a"].Status)
	}
}

func TestAggregator_PartialDegraded(t *testing.T) {
	agg := NewAggregator("svc", 0,
		CheckerFunc{NameStr: "a", Fn: func(ctx context.Context) error { return nil }},
		CheckerFunc{NameStr: "b", Fn: func(ctx context.Context) error { return errors.New("down") }},
	)
	resp := agg.RefreshIfStale()
	if resp.Status != string(StatusDegraded) {
		t.Fatalf("Status = %q, want degraded", resp.Status)
	}
	if resp.Dependencies["b"].Status != StatusUnhealthy {
		t.Errorf("dep b status = %q, want unhealthy", resp.Dependencies["b"].Status)
	}
	if resp.Dependencies["b"].Error == "" {
		t.Error("dep b error message empty")
	}
}

func TestAggregator_AllUnhealthy(t *testing.T) {
	agg := NewAggregator("svc", 0,
		CheckerFunc{NameStr: "a", Fn: func(ctx context.Context) error { return errors.New("down") }},
		CheckerFunc{NameStr: "b", Fn: func(ctx context.Context) error { return errors.New("down") }},
	)
	resp := agg.RefreshIfStale()
	if resp.Status != string(StatusUnhealthy) {
		t.Fatalf("Status = %q, want unhealthy", resp.Status)
	}
}

func TestAggregator_NoCheckers(t *testing.T) {
	agg := NewAggregator("svc", 0)
	resp := agg.RefreshIfStale()
	if resp.Status != string(StatusOK) {
		t.Fatalf("Status = %q, want ok with no deps", resp.Status)
	}
}

func TestAggregator_ErrorTruncation(t *testing.T) {
	long := make([]byte, 300)
	for i := range long {
		long[i] = 'x'
	}
	agg := NewAggregator("svc", 0,
		CheckerFunc{NameStr: "a", Fn: func(ctx context.Context) error { return errors.New(string(long)) }},
	)
	resp := agg.RefreshIfStale()
	msg := resp.Dependencies["a"].Error
	if len(msg) > 203 {
		t.Fatalf("error message not truncated: len=%d", len(msg))
	}
	if msg[len(msg)-3:] != "..." {
		t.Fatalf("truncated message should end with ..., got %q", msg[len(msg)-3:])
	}
}

func TestAggregator_Caching(t *testing.T) {
	var calls int32
	probe := CheckerFunc{NameStr: "dep", Fn: func(ctx context.Context) error {
		atomic.AddInt32(&calls, 1)
		return nil
	}}
	agg := NewAggregator("svc", 50*time.Millisecond, probe)

	// First call runs synchronously (cold cache).
	r1 := agg.RefreshIfStale()
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("after first call, probe calls = %d, want 1", atomic.LoadInt32(&calls))
	}
	if r1.Cached {
		t.Error("first response should not be marked cached")
	}

	// Second call within TTL should be served from cache without re-running.
	r2 := agg.RefreshIfStale()
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("cached call re-ran probe: calls = %d, want 1", atomic.LoadInt32(&calls))
	}
	if !r2.Cached {
		t.Error("second response should be marked cached")
	}

	// After TTL, a background refresh is triggered; wait for it.
	time.Sleep(60 * time.Millisecond)
	agg.RefreshIfStale()
	// Allow the goroutine to complete.
	time.Sleep(20 * time.Millisecond)
	if atomic.LoadInt32(&calls) < 2 {
		t.Fatalf("after TTL, probe calls = %d, want >= 2", atomic.LoadInt32(&calls))
	}
}

func TestLivenessHandler(t *testing.T) {
	h := LivenessHandler("svc")
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp LivenessResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != string(StatusOK) || resp.Service != "svc" {
		t.Fatalf("response = %+v", resp)
	}
}

func TestLivenessHandler_MethodNotAllowed(t *testing.T) {
	h := LivenessHandler("svc")
	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", w.Code)
	}
}

func TestReadinessHandler_OK(t *testing.T) {
	agg := NewAggregator("svc", 0,
		CheckerFunc{NameStr: "dep", Fn: func(ctx context.Context) error { return nil }},
	)
	h := ReadinessHandler("svc", agg)
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp ReadinessResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != string(StatusOK) {
		t.Fatalf("status = %q, want ok", resp.Status)
	}
}

func TestReadinessHandler_UnhealthyReturns503(t *testing.T) {
	agg := NewAggregator("svc", 0,
		CheckerFunc{NameStr: "dep", Fn: func(ctx context.Context) error { return errors.New("down") }},
	)
	h := ReadinessHandler("svc", agg)
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
}

func TestReadinessHandler_DegradedReturns200(t *testing.T) {
	agg := NewAggregator("svc", 0,
		CheckerFunc{NameStr: "a", Fn: func(ctx context.Context) error { return nil }},
		CheckerFunc{NameStr: "b", Fn: func(ctx context.Context) error { return errors.New("down") }},
	)
	h := ReadinessHandler("svc", agg)
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for degraded", w.Code)
	}
}

func TestReadinessHandler_MethodNotAllowed(t *testing.T) {
	agg := NewAggregator("svc", 0)
	h := ReadinessHandler("svc", agg)
	req := httptest.NewRequest(http.MethodPost, "/health/ready", nil)
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", w.Code)
	}
}

func TestAggregator_PerCheckTimeout(t *testing.T) {
	slow := CheckerFunc{NameStr: "slow", Fn: func(ctx context.Context) error {
		select {
		case <-time.After(2 * time.Second):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}}
	agg := NewAggregator("svc", 0, slow)
	agg.SetCheckTimeout(200 * time.Millisecond)
	start := time.Now()
	resp := agg.RefreshIfStale()
	elapsed := time.Since(start)
	if elapsed > 1500*time.Millisecond {
		t.Fatalf("per-check timeout did not bound the call: elapsed=%s", elapsed)
	}
	if resp.Dependencies["slow"].Status != StatusUnhealthy {
		t.Fatalf("slow dep status = %q, want unhealthy", resp.Dependencies["slow"].Status)
	}
}

func TestStateString(t *testing.T) {
	cases := []struct {
		s    State
		want string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{State(99), "unknown"},
	}
	for _, c := range cases {
		if got := c.s.String(); got != c.want {
			t.Errorf("State(%d).String() = %q, want %q", c.s, got, c.want)
		}
	}
}

// fmt is imported to keep the linter happy when extending tests later.
var _ = fmt.Sprintf
