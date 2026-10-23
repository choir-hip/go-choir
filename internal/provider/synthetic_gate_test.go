package provider

import (
	"context"
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
)

// Synthetic's account ceiling is about 10 concurrent calls; above it the
// upstream answers 429 "Too many concurrent requests"
// (docs/problems/synthetic-provider-onboarding-2026-10-22.md, "Staging:
// concurrency 429s and the circuit breaker"). The provider gates its own calls
// so the excess waits for a slot instead of being sent and rejected. These
// tests build the provider through its public constructor with the gate sized
// by the environment, against a fake upstream that 429s above its own limit.

// capUpstream is a fake Synthetic: it holds each call for hold, answers 429
// when more than limit calls are in flight, and records the peak in flight.
type capUpstream struct {
	*httptest.Server
	inflight, peak, rejected, served atomic.Int32
}

func newCapUpstream(t *testing.T, limit int32, hold time.Duration) *capUpstream {
	t.Helper()
	u := &capUpstream{}
	u.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		n := u.inflight.Add(1)
		defer u.inflight.Add(-1)
		for {
			p := u.peak.Load()
			if n <= p || u.peak.CompareAndSwap(p, n) {
				break
			}
		}
		if n > limit {
			u.rejected.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = fmt.Fprint(w, `{"error":"Too many concurrent requests"}`)
			return
		}
		time.Sleep(hold)
		u.served.Add(1)
		stream := strings.Contains(r.Header.Get("Accept"), "event-stream")
		if stream {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = fmt.Fprint(w, "data: {\"id\":\"c\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"ok\"}}]}\n\n"+
				"data: {\"id\":\"c\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id":"c","choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"ok"}}]}`)
	}))
	t.Cleanup(u.Close)
	return u
}

func newGatedSynthetic(t *testing.T, u *capUpstream, max, wait string) *SyntheticProvider {
	t.Helper()
	t.Setenv("SYNTHETIC_MAX_CONCURRENT", max)
	t.Setenv("SYNTHETIC_MAX_WAIT", wait)
	p, err := NewSyntheticProvider(SyntheticConfig{APIKey: "syn-test-key", ModelID: syntheticTestModel, BaseURL: u.URL})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// More callers than the upstream allows all succeed, in both call shapes: the
// excess waits for a slot. Without the gate the upstream sees all of them at
// once and 429s the excess.
func TestSyntheticGateKeepsConcurrencyUnderTheUpstreamCeiling(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprintf("stream=%t", stream), func(t *testing.T) {
			u := newCapUpstream(t, 3, 40*time.Millisecond)
			p := newGatedSynthetic(t, u, "3", "30s")
			const callers = 12
			errs := make(chan error, callers)
			var wg sync.WaitGroup
			for i := 0; i < callers; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					var err error
					if stream {
						_, err = p.Stream(context.Background(), syntheticUserRequest(), func(StreamChunk) {})
					} else {
						_, err = p.Call(context.Background(), syntheticUserRequest())
					}
					errs <- err
				}()
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				if err != nil {
					t.Fatalf("call failed: %v (upstream rejected %d, peak in flight %d)", err, u.rejected.Load(), u.peak.Load())
				}
			}
			if u.rejected.Load() != 0 || u.peak.Load() > 3 || u.served.Load() != callers {
				t.Fatalf("rejected=%d peak=%d served=%d, want 0, <=3, %d", u.rejected.Load(), u.peak.Load(), u.served.Load(), callers)
			}
		})
	}
}

// A call that cannot get a slot inside the wait bound fails without being sent,
// with an error that is not a status 429 (the run reads "429" as a rate limit
// to back off from), and the slot it never held is not leaked: once the
// holder finishes, the next call goes through.
func TestSyntheticGateWaitTimesOutWithoutSendingAndLeaksNothing(t *testing.T) {
	u := newCapUpstream(t, 5, 300*time.Millisecond)
	p := newGatedSynthetic(t, u, "1", "60ms")

	holder := make(chan error, 1)
	go func() {
		_, err := p.Call(context.Background(), syntheticUserRequest())
		holder <- err
	}()
	deadline := time.Now().Add(5 * time.Second)
	for u.inflight.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("holder never reached the upstream")
		}
		time.Sleep(2 * time.Millisecond)
	}

	_, err := p.Call(context.Background(), syntheticUserRequest())
	if err == nil {
		t.Fatal("second call got a slot while the only slot was held")
	}
	if !errors.Is(err, ErrConcurrencyGateWait) {
		t.Fatalf("wait timeout error = %v, want ErrConcurrencyGateWait", err)
	}
	if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "status") {
		t.Fatalf("wait timeout error %q reads like an upstream status", err)
	}
	if u.rejected.Load() != 0 || u.peak.Load() != 1 {
		t.Fatalf("the waiter reached the upstream: rejected=%d peak=%d", u.rejected.Load(), u.peak.Load())
	}
	if err := <-holder; err != nil {
		t.Fatalf("holder: %v", err)
	}
	if _, err := p.Call(context.Background(), syntheticUserRequest()); err != nil {
		t.Fatalf("call after the holder finished: %v (slot leaked?)", err)
	}
}

// A caller that goes away while waiting gives up at once and holds nothing.
func TestSyntheticGateWaiterHonoursItsContext(t *testing.T) {
	u := newCapUpstream(t, 5, 300*time.Millisecond)
	p := newGatedSynthetic(t, u, "1", "30s")
	go func() { _, _ = p.Call(context.Background(), syntheticUserRequest()) }()
	deadline := time.Now().Add(5 * time.Second)
	for u.inflight.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("holder never reached the upstream")
		}
		time.Sleep(2 * time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := p.Call(ctx, syntheticUserRequest())
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancelled waiter error = %v, want context deadline", err)
	}
	if time.Since(started) > 2*time.Second {
		t.Fatal("waiter ignored its context")
	}
}

// The upstream's own 429 is typed, so the breaker can tell a concurrency
// limit from model ill-health, and its text keeps the sanitized form.
func TestSyntheticUpstream429IsTypedAsAConcurrencyLimit(t *testing.T) {
	u := newCapUpstream(t, 0, 0) // rejects every call
	p := newGatedSynthetic(t, u, "8", "30s")
	for _, stream := range []bool{false, true} {
		var err error
		if stream {
			_, err = p.Stream(context.Background(), syntheticUserRequest(), func(StreamChunk) {})
		} else {
			_, err = p.Call(context.Background(), syntheticUserRequest())
		}
		if !errors.Is(err, ErrUpstreamConcurrencyLimit) {
			t.Fatalf("stream=%t: 429 error = %v, want ErrUpstreamConcurrencyLimit", stream, err)
		}
		if !strings.Contains(err.Error(), "synthetic: status 429") || strings.Contains(err.Error(), "concurrent") {
			t.Fatalf("stream=%t: 429 text = %q, want the sanitized status 429", stream, err)
		}
	}
}

// The gate's counters are readable for /health.
func TestSyntheticGateStatusCountsWaitsAndTimeouts(t *testing.T) {
	u := newCapUpstream(t, 5, 150*time.Millisecond)
	p := newGatedSynthetic(t, u, "1", "40ms")
	st, ok := SyntheticGateStatus()
	if !ok || st.Limit != 1 || st.InFlight != 0 {
		t.Fatalf("idle status = %+v ok=%t, want limit 1 and nothing in flight", st, ok)
	}
	go func() { _, _ = p.Call(context.Background(), syntheticUserRequest()) }()
	deadline := time.Now().Add(5 * time.Second)
	for u.inflight.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("holder never reached the upstream")
		}
		time.Sleep(2 * time.Millisecond)
	}
	if st, _ := SyntheticGateStatus(); st.InFlight != 1 {
		t.Fatalf("status during a call: %+v, want 1 in flight", st)
	}
	_, _ = p.Call(context.Background(), syntheticUserRequest())
	st, _ = SyntheticGateStatus()
	if st.Waited < 1 || st.WaitTimeouts != 1 || st.Peak != 1 {
		t.Fatalf("status after a wait timeout: %+v, want waited>=1, wait_timeouts=1, peak=1", st)
	}
}
