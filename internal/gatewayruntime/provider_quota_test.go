package gatewayruntime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

// docs/problems/provider-quota-kills-desk-loops-2026-11-04.md: the gateway's
// typed provider quota answer (429, kind provider_quota) is not the
// per-computer limiter. Failure modes pinned: the call waits out the
// quota's Retry-After in-call (up to ten minutes, invisible); the answer
// reaches the tool loop untyped; the reset time or model is lost.
func TestProviderQuotaAnswerIsTypedAndNotWaitedInCall(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "1913")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"synthetic: quota exhausted: status 429 Too Many Requests (sanitized); resets at 2026-11-04T03:07:16Z","kind":"provider_quota","provider":"synthetic","model":"hf:deepseek-ai/DeepSeek-V4.1-Flash-1017","reason":"provider quota exhausted (synthetic)","retry_at":"2026-11-04T03:07:16Z"}`))
	}))
	defer server.Close()

	start := time.Now()
	_, err := New(server.URL, "token").CallWithTools(context.Background(), provideriface.ToolLoopRequest{Model: "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017"})
	q, ok := provideriface.AsProviderQuota(err)
	if !ok {
		t.Fatalf("err = %v, want the typed provider quota", err)
	}
	if time.Since(start) > 2*time.Second || hits.Load() != 1 {
		t.Fatalf("the quota was retried in-call (%d hits, %s)", hits.Load(), time.Since(start))
	}
	want := time.Date(2026, 11, 4, 3, 7, 16, 0, time.UTC)
	if q.Provider != "synthetic" || q.Model != "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017" || !q.RetryAt.Equal(want) || q.Reason != "provider quota exhausted (synthetic)" {
		t.Fatalf("quota = %+v", q)
	}
}
