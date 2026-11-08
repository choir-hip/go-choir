package searchplane

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// Keenable's rate limit and the failure log line. Keenable has no monthly
// pace or daily cap (owner, 2026-11-08); see keenable_unpaced_test.go.
// Ways it can fail:
//  6. Keenable is called faster than 8 a second (above its 10/s limit) by
//     concurrent searches from many computers.
//  9. The env knobs: defaults wrong, or pacing for one provider dropped
//     when another's rate is set.
//  10. The failure log line carries a key, a query or a response body, or
//      does not say why (status, stage).

func mustDay(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func countingKeenable() *stubProvider {
	return &stubProvider{name: "keenable", searchFn: func(ctx context.Context, q string, max int) ([]Result, error) {
		return manyResults("k", 12), nil
	}}
}

func TestKeenableRateLimit_EightPerSecondAcrossConcurrentSearches(t *testing.T) { // 6
	t.Setenv("CHOIR_SEARCH_PROVIDER_RPS", "")
	env := ConfigFromEnv()
	cfg := budgetConfig()
	cfg.MinInterval = env.MinInterval
	cfg.PaceWait = 5 * time.Second
	var mu sync.Mutex
	var starts []time.Time
	keenable := &stubProvider{name: "keenable", searchFn: func(ctx context.Context, q string, max int) ([]Result, error) {
		mu.Lock()
		starts = append(starts, time.Now())
		mu.Unlock()
		return manyResults("k", 12), nil
	}}
	router := NewRouter([]Provider{keenable}, NewMemoryHealthStore(), cfg)
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, _ := router.Start(context.Background(), "q", 40)
			drain(t, s)
		}()
	}
	wg.Wait()
	sort.Slice(starts, func(i, j int) bool { return starts[i].Before(starts[j]) })
	if len(starts) != 24 {
		t.Fatalf("calls = %d", len(starts))
	}
	for i := 0; i+8 < len(starts); i++ {
		// Nine calls span at least a second, less timer jitter: never more
		// than 8 in a second, far under Keenable's 10.
		if gap := starts[i+8].Sub(starts[i]); gap < 950*time.Millisecond {
			t.Fatalf("calls %d..%d within %v", i, i+8, gap)
		}
	}
}

func TestConfigFromEnv_KeenableRate(t *testing.T) { // 9
	t.Setenv("CHOIR_SEARCH_PROVIDER_RPS", "")
	cfg := ConfigFromEnv()
	if cfg.MinInterval["keenable"] != 125*time.Millisecond || cfg.MinInterval["brave"] != time.Second {
		t.Fatalf("default pacing = %v", cfg.MinInterval)
	}
	if _, ok := cfg.DailyCaps["keenable"]; ok {
		t.Fatal("keenable has a daily cap; it has none (owner, 2026-11-08)")
	}

	t.Setenv("CHOIR_SEARCH_PROVIDER_RPS", "brave=2")
	if cfg := ConfigFromEnv(); cfg.MinInterval["keenable"] != 125*time.Millisecond || cfg.MinInterval["brave"] != 500*time.Millisecond {
		t.Fatalf("override dropped keenable pacing: %v", cfg.MinInterval)
	}
	t.Setenv("CHOIR_SEARCH_PROVIDER_RPS", "keenable=0")
	if cfg := ConfigFromEnv(); cfg.MinInterval["brave"] != time.Second {
		t.Fatalf("keenable=0: %v", cfg.MinInterval)
	} else if _, ok := cfg.MinInterval["keenable"]; ok {
		t.Fatalf("keenable=0 kept pacing: %v", cfg.MinInterval)
	}
}

func TestFailureFields_SayWhyWithoutBodyOrKey(t *testing.T) { // 10
	cases := []struct {
		err           error
		status, stage string
	}{
		{errors.New(`status 401 Unauthorized: {"error":"invalid api key sk-SECRET123456"}`), "401", "http_status"},
		{errors.New(`status 402 Payment Required: Insufficient credit`), "402", "http_status"},
		{errors.New("parse response: unexpected end of JSON input"), "-", "parse_response"},
		{fmt.Errorf("http request: %w", context.DeadlineExceeded), "-", "timeout"},
		{errors.New(`http request: Post "https://api.example/search?api_key=SECRET": dial tcp: refused`), "-", "http_request"},
		{errors.New("status error: Your account has run out of searches"), "-", "status_error"},
		{errors.New("SECRET weird failure"), "-", "other"},
	}
	for _, c := range cases {
		status, stage := FailureFields(c.err)
		if status != c.status || stage != c.stage {
			t.Errorf("%q: status=%s stage=%s, want %s %s", c.err, status, stage, c.status, c.stage)
		}
	}

	// Through the router: the log line says provider, class, status and
	// stage, and nothing from the body or the query.
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)
	exa := &stubProvider{name: "exa", searchFn: func(ctx context.Context, q string, max int) ([]Result, error) {
		return nil, errors.New(`status 400 Bad Request: {"error":"key=SECRET bad numResults for query QUERYTEXT"}`)
	}}
	router := NewRouter([]Provider{exa}, NewMemoryHealthStore(), Config{RequestTimeout: time.Second})
	s, _ := router.Start(context.Background(), "QUERYTEXT", 40)
	drain(t, s)
	line := buf.String()
	if !strings.Contains(line, "searchplane: exa call failed class=error http_status=400 stage=http_status") {
		t.Fatalf("log = %q", line)
	}
	if strings.Contains(line, "SECRET") || strings.Contains(line, "QUERYTEXT") {
		t.Fatalf("log leaks: %q", line)
	}
}
