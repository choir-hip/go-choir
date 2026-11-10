package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Ways the shared Jev limiter can fail (docs/evidence/jev-measurement-2026-11-10.md,
// "Build: failure list"), each with its test below:
//   - a bulk burst starves the interactive class;
//   - a provider 429 opens a long hold instead of one shared backoff;
//   - a cancelled waiter takes or keeps a slot;
//   - a slot leaks on an error path;
//   - one computer takes the whole share, or many computers together exceed
//     the one budget;
//   - the queue wait is unbounded;
//   - the receipt fsync serialises completions;
//   - a switch turns the limiter off.

func testLimiter(maxInFlight, reserve int, burst float64) *jevLimiter {
	l := newJevLimiter()
	l.maxInFlight, l.reserve, l.burst, l.rate = maxInFlight, reserve, burst, 0.0001
	l.maxWait = 60 * time.Millisecond
	return l
}

func TestJevLimiterInteractiveNotStarvedByBulk(t *testing.T) {
	l := testLimiter(4, 2, 100)
	var releases []func()
	for i := 0; i < 2; i++ {
		rel, _, err := l.acquire(context.Background(), fmt.Sprintf("c%d", i), jevBulk)
		if err != nil {
			t.Fatalf("bulk %d: %v", i, err)
		}
		releases = append(releases, rel)
	}
	if _, _, err := l.acquire(context.Background(), "c9", jevBulk); err == nil {
		t.Fatal("a third bulk call took the interactive reserve")
	}
	for i := 0; i < 2; i++ {
		rel, waited, err := l.acquire(context.Background(), "router", jevInteractive)
		if err != nil || waited > 20*time.Millisecond {
			t.Fatalf("interactive %d waited %s behind bulk: %v", i, waited, err)
		}
		releases = append(releases, rel)
	}
	for _, r := range releases {
		r()
	}
	// Tokens: bulk cannot spend the last tokenReserve.
	l = testLimiter(32, 8, 7)
	for i := 0; i < 2; i++ { // 7 tokens, reserve 5: bulk gets 2
		if _, _, err := l.acquire(context.Background(), fmt.Sprintf("b%d", i), jevBulk); err != nil {
			t.Fatalf("bulk token %d: %v", i, err)
		}
	}
	if _, _, err := l.acquire(context.Background(), "b9", jevBulk); err == nil {
		t.Fatal("bulk spent the interactive token reserve")
	}
	if _, _, err := l.acquire(context.Background(), "r", jevInteractive); err != nil {
		t.Fatalf("interactive refused with reserved tokens left: %v", err)
	}
}

func TestJevLimiterSharedBackoffHonoursRetryAfter(t *testing.T) {
	l := testLimiter(32, 8, 100)
	var skew time.Duration
	l.now = func() time.Time { return time.Now().Add(skew) }
	l.backoff(5 * time.Second)
	_, _, err := l.acquire(context.Background(), "c", jevInteractive)
	var limited *jevLimitedError
	if !errors.As(err, &limited) {
		t.Fatalf("during backoff = %v; want a limited error", err)
	}
	skew = 6 * time.Second
	now := l.now()
	if _, _, err := l.acquire(context.Background(), "other", jevInteractive); err != nil {
		t.Fatalf("after the backoff ended: %v", err)
	}
	if got := parseRetryAfter(http.Header{"Retry-After": []string{"7"}}, now); got != 7*time.Second {
		t.Fatalf("Retry-After 7 parsed as %s", got)
	}
	if got := parseRetryAfter(http.Header{}, now); got != jevBackoffDefault {
		t.Fatalf("missing Retry-After parsed as %s", got)
	}
}

func TestJevLimiterCancelledWaiterLeavesNoSlot(t *testing.T) {
	l := testLimiter(2, 0, 100)
	var held []func()
	for i := 0; i < 2; i++ {
		rel, _, _ := l.acquire(context.Background(), "c", jevInteractive)
		held = append(held, rel)
	}
	l.maxWait = time.Minute
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, _, err := l.acquire(ctx, "c", jevInteractive); done <- err }()
	time.Sleep(30 * time.Millisecond)
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled waiter = %v", err)
	}
	for _, r := range held {
		r()
		r() // a double release must not free a second slot
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.inFlight != 0 || l.computers["c"].inFlight != 0 {
		t.Fatalf("in flight after all releases = %d/%d", l.inFlight, l.computers["c"].inFlight)
	}
}

func TestJevLimiterPerComputerCap(t *testing.T) {
	l := testLimiter(100, 0, 1000)
	l.rate = 1000
	for i := 0; i < jevComputerInFlight; i++ {
		if _, _, err := l.acquire(context.Background(), "paper", jevBulk); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if _, _, err := l.acquire(context.Background(), "paper", jevBulk); err == nil {
		t.Fatal("one computer exceeded its in-flight cap")
	}
	if _, _, err := l.acquire(context.Background(), "other", jevBulk); err != nil {
		t.Fatalf("another computer starved by the first: %v", err)
	}
}

func TestJevLimiterRateIsProcessWideAndQueueBounded(t *testing.T) {
	l := testLimiter(100, 0, 3)
	l.tokenReserve = 0
	for i := 0; i < 3; i++ {
		if _, _, err := l.acquire(context.Background(), fmt.Sprintf("vm-%d", i), jevBulk); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	start := time.Now()
	_, _, err := l.acquire(context.Background(), "vm-new", jevBulk)
	var limited *jevLimitedError
	if !errors.As(err, &limited) {
		t.Fatalf("a fourth computer past the one budget = %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("the queue wait was not bounded: %s", time.Since(start))
	}
}

// A 429 from the provider is one shared backoff, never a long hold; a slot is
// released on every path.
func TestJevTransport429SetsBackoffNotHoldAndReleases(t *testing.T) {
	upstream := &statusRoundTripper{status: http.StatusTooManyRequests, retryAfter: "7"}
	tr := &JevTransport{enabled: true, apiKey: "k", httpClient: &http.Client{Transport: upstream},
		receipts: newJevReceiptWriter(filepath.Join(t.TempDir(), "r.jsonl"))}
	req := JudgmentRequest{State: json.RawMessage(`{"a":1}`), Questions: map[string]json.RawMessage{"q": json.RawMessage(`{"type":"choice","choices":["a","b"]}`)}}
	if _, err := tr.Decide(context.Background(), "c", req); err == nil {
		t.Fatal("429 should fail the call")
	}
	if !tr.hold.heldUntil(time.Now()).IsZero() {
		t.Fatal("a 429 opened the long hold")
	}
	l := tr.limit()
	l.mu.Lock()
	remaining := time.Until(l.backoffUntil)
	inFlight := l.inFlight
	l.mu.Unlock()
	if remaining < 5*time.Second || remaining > 8*time.Second {
		t.Fatalf("shared backoff = %s; want about the 7 s Retry-After", remaining)
	}
	if inFlight != 0 {
		t.Fatalf("slot leaked on the error path: %d", inFlight)
	}
}

type statusRoundTripper struct {
	status     int
	retryAfter string
}

func (s *statusRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	h := http.Header{}
	if s.retryAfter != "" {
		h.Set("Retry-After", s.retryAfter)
	}
	return &http.Response{StatusCode: s.status, Header: h, Body: io.NopCloser(strings.NewReader("{}")), Request: r}, nil
}

func TestJevReceiptWriterConcurrentWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "r.jsonl")
	w := newJevReceiptWriter(path)
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := w.write(jevTransportReceipt{Schema: "s", Request: jevReceiptRequest{ComputerID: fmt.Sprint(i)},
				Response: jevReceiptResponse{Status: 200, Distribution: json.RawMessage(`{"x":1}`)}})
			if err != nil {
				t.Errorf("write %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 40 {
		t.Fatalf("receipt lines = %d, want 40", len(lines))
	}
	for _, line := range lines {
		if !json.Valid([]byte(line)) {
			t.Fatalf("interleaved receipt line: %q", line)
		}
	}
}

func TestJevTransportIdleConnsPerHost(t *testing.T) {
	tr, ok := newJevHTTPClient().Transport.(*http.Transport)
	if !ok || tr.MaxIdleConnsPerHost < 64 {
		t.Fatalf("Jev transport keeps %v idle connections per host", tr)
	}
}

func TestJevStatsSummary(t *testing.T) {
	var s jevStats
	now := time.Now()
	for i := 1; i <= 10; i++ {
		if line := s.observe(now, time.Duration(i)*10*time.Millisecond, time.Millisecond, 200, true); line != "" {
			t.Fatalf("summary before the hour: %s", line)
		}
	}
	s.observe(now, time.Second, 0, http.StatusTooManyRequests, false)
	line := s.observe(now.Add(jevStatsInterval), 20*time.Millisecond, 0, 200, true)
	for _, want := range []string{"calls=12", "failures=1", "429s=1", "p50=", "p90="} {
		if !strings.Contains(line, want) {
			t.Fatalf("summary %q lacks %q", line, want)
		}
	}
	if again := s.observe(now.Add(jevStatsInterval), time.Millisecond, 0, 200, true); again != "" {
		t.Fatalf("summary repeated: %s", again)
	}
}

func TestJevLimiterHasNoSwitch(t *testing.T) {
	for _, name := range []string{"jev_limiter.go", "jev.go"} {
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.Contains(line, "Getenv") && !strings.Contains(line, "OPENROUTER_API_KEY") && !strings.Contains(line, "GATEWAY_JEV_RECEIPT_PATH") {
				t.Fatalf("%s reads an environment variable: %s", name, line)
			}
		}
	}
}
