package gatewayruntime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// docs/archive/problems/research-fanout-overruns-computer-model-budget-2026-10-11.md:
// the gateway's per-computer limiter answered 429 with Retry-After, and the
// guest retried after 0.5 s and 1 s and failed, so callers died on rejected
// calls. Failure modes pinned: a call rejected by the limiter fails instead
// of waiting out Retry-After; it waits past its caller's deadline; a call
// that is rejected for another reason waits.
func TestGatewayCallWaitsOutTheRateLimitWindow(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) <= 3 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limit exceeded for autoputer c (limit 60 requests per 1m0s)"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	start := time.Now()
	if _, err := New(server.URL, "token").call(context.Background(), llmRequest{Model: "m"}); err != nil {
		t.Fatalf("a call rejected three times by the rate limiter failed: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 2500*time.Millisecond {
		t.Fatalf("retried before Retry-After elapsed (%s for three 1 s waits)", elapsed)
	}

	hits.Store(-100)
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	start = time.Now()
	if _, err := New(server.URL, "token").call(ctx, llmRequest{Model: "m"}); err == nil {
		t.Fatal("a call still rate limited at its deadline succeeded")
	}
	if elapsed := time.Since(start); elapsed > 2500*time.Millisecond {
		t.Fatalf("waited %s past a 1.5 s deadline", elapsed)
	}

	var bad atomic.Int32
	rejects := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bad.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer rejects.Close()
	if _, err := New(rejects.URL, "token").call(context.Background(), llmRequest{Model: "m"}); err == nil || bad.Load() != 1 {
		t.Fatalf("a 400 was retried or succeeded: %v (%d calls)", err, bad.Load())
	}
}
