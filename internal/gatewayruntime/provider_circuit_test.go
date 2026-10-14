package gatewayruntime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// An open circuit answers the same way for its whole open window (30 s), so
// retrying it 0.5 s and 1 s later only delays the run's failover to its
// next pool member (docs/problems/free-model-429-opens-provider-circuit-
// stub-stories-2026-10-14.md, cause 2). Ways it can fail: the client still
// retries an open circuit; it stops retrying other 502s, which a restart
// or a dropped upstream connection can clear.
func TestGatewayCallDoesNotRetryAnOpenCircuit(t *testing.T) {
	var hits atomic.Int32
	open := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"provider opencode-go model cat-jam-free: circuit open (model unhealthy)"}`))
	}))
	defer open.Close()
	start := time.Now()
	_, err := New(open.URL, "token").call(context.Background(), llmRequest{Model: "cat-jam-free"})
	if err == nil || !strings.Contains(err.Error(), "circuit open") {
		t.Fatalf("err = %v, want the open circuit", err)
	}
	if hits.Load() != 1 || time.Since(start) > 400*time.Millisecond {
		t.Fatalf("an open circuit was retried: %d calls in %s", hits.Load(), time.Since(start))
	}

	var other atomic.Int32
	flaky := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		other.Add(1)
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"opencode-go: status 503 Service Unavailable (sanitized)"}`))
	}))
	defer flaky.Close()
	if _, err := New(flaky.URL, "token").call(context.Background(), llmRequest{Model: "m"}); err == nil || other.Load() != 3 {
		t.Fatalf("a plain 502 was not retried: %v (%d calls)", err, other.Load())
	}
}
