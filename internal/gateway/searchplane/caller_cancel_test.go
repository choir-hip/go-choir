package searchplane

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"
)

// docs/archive/problems/search-plane-cooldown-on-caller-cancel-2026-10-09.md.
// Failure modes: a caller that goes away (cancel or its own shorter
// deadline) strikes every provider it started and cools the plane down; a
// provider that really overruns the request timeout is no longer struck; a
// provider error carrying its request URL stores or returns the API key.

func blockingProvider(name string) *stubProvider {
	return &stubProvider{name: name, searchFn: func(ctx context.Context, query string, maxResults int) ([]Result, error) {
		<-ctx.Done()
		return nil, fmt.Errorf("http request: %w", ctx.Err())
	}}
}

func assertNoStrike(t *testing.T, store HealthStore, names ...string) {
	t.Helper()
	health, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		rec := health[name]
		if rec.StrikeCount != 0 || rec.State == StateCoolingDown {
			t.Fatalf("provider %s struck by the caller going away: %+v", name, rec)
		}
	}
}

func TestRouter_CallerCancelDoesNotStrikeProviders(t *testing.T) {
	store := NewMemoryHealthStore()
	router := NewRouter([]Provider{blockingProvider("a"), blockingProvider("b"), blockingProvider("c")}, store, Config{RequestTimeout: 10 * time.Second})
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	stream, err := router.Start(ctx, "q", 5)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range drain(t, stream) {
		if a.Status != string(OutcomeCallerGone) {
			t.Fatalf("arrival after the caller cancelled = %+v, want caller_gone", a)
		}
	}
	assertNoStrike(t, store, "a", "b", "c")
}

func TestRouter_CallerDeadlineShorterThanRequestTimeoutDoesNotStrike(t *testing.T) {
	store := NewMemoryHealthStore()
	router := NewRouter([]Provider{blockingProvider("a")}, store, Config{RequestTimeout: 10 * time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	stream, err := router.Start(ctx, "q", 5)
	if err != nil {
		t.Fatal(err)
	}
	drain(t, stream)
	assertNoStrike(t, store, "a")
}

func TestRouter_ProviderOverrunningRequestTimeoutIsStillStruck(t *testing.T) {
	store := NewMemoryHealthStore()
	router := NewRouter([]Provider{blockingProvider("slow")}, store, Config{RequestTimeout: 50 * time.Millisecond})
	stream, err := router.Start(context.Background(), "q", 5)
	if err != nil {
		t.Fatal(err)
	}
	if arrivals := drain(t, stream); len(arrivals) != 1 || arrivals[0].Status != string(OutcomeTimeout) {
		t.Fatalf("arrivals = %+v, want one timeout", arrivals)
	}
	health, _ := store.Snapshot()
	if rec := health["slow"]; rec.StrikeCount != 1 || rec.LastFailureClass != string(OutcomeTimeout) {
		t.Fatalf("a provider that overran the request timeout must be struck as timeout: %+v", rec)
	}
}

const testSecret = "sk-secret-0123456789"

func urlError(key string) error {
	return fmt.Errorf("http request: %w", &url.Error{
		Op:  "Get",
		URL: "https://serpapi.com/search.json?engine=google&q=private+query&api_key=" + key,
		Err: context.DeadlineExceeded,
	})
}

func TestProviderErrorSummaryNeverHoldsKeyOrQuery(t *testing.T) {
	store := NewMemoryHealthStore()
	router := NewRouter([]Provider{&stubProvider{name: "serpapi", searchFn: func(ctx context.Context, query string, maxResults int) ([]Result, error) {
		return nil, urlError(testSecret)
	}}}, store, Config{RequestTimeout: time.Second})
	stream, err := router.Start(context.Background(), "private query", 5)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range drain(t, stream) {
		if a.Error == "" || strings.Contains(a.Error, testSecret) || strings.Contains(a.Error, "private") {
			t.Fatalf("arrival error leaks the request query (or is empty): %q", a.Error)
		}
	}
	health, _ := store.Snapshot()
	if s := health["serpapi"].LastErrorSummary; s == "" || strings.Contains(s, testSecret) || strings.Contains(s, "private") {
		t.Fatalf("stored summary leaks the request query (or is empty): %q", s)
	}
}

func TestRedactSummaryCleansLegacyStoredSummaries(t *testing.T) {
	cases := []string{
		`timeout: http request: Get "https://serpapi.com/search.json?api_key=` + testSecret + `&q=x": context canceled`,
		`status 401: {"error":"Invalid API key ` + testSecret + `"} key=` + testSecret,
		"token=" + testSecret + " apikey=" + testSecret,
	}
	for _, in := range cases {
		out := RedactSummary(in)
		if strings.Contains(out, testSecret) {
			t.Fatalf("RedactSummary(%q) = %q, still holds the secret", in, out)
		}
	}
	if got := RedactSummary("status 422: Count should be less than or equal to 20"); !strings.Contains(got, "Count should be") {
		t.Fatalf("plain provider message must survive: %q", got)
	}
}
