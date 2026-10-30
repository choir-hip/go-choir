package searchplane

import (
	"context"
	"testing"
	"time"
)

// docs/archive/problems/search-plane-empty-result-benches-healthy-providers-2026-10-11.md.
// Failure modes: one narrow query that finds nothing cools down every
// provider it reached, so the next caller's broad query starts no provider;
// a provider that returns nothing on every query is never benched.
func TestRouter_EmptyResultDoesNotBenchHealthyProviders(t *testing.T) {
	answers := func(name string) *stubProvider {
		return &stubProvider{name: name, searchFn: func(ctx context.Context, query string, maxResults int) ([]Result, error) {
			if query == "site:example.invalid nothing here" {
				return nil, nil
			}
			return []Result{{Title: name, URL: "https://" + name + ".example/" + query}}, nil
		}}
	}
	store := NewMemoryHealthStore()
	router := NewRouter([]Provider{answers("a"), answers("b"), answers("c")}, store, Config{RequestTimeout: time.Second})
	stream, err := router.Start(context.Background(), "site:example.invalid nothing here", 5)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range drain(t, stream) {
		if a.Status != string(OutcomeSuccessEmpty) || !a.Answered() {
			t.Fatalf("empty arrival = %+v, want an answered success_empty", a)
		}
	}
	health, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a", "b", "c"} {
		if health[name].State == StateCoolingDown {
			t.Fatalf("provider %s benched by one empty result: %+v", name, health[name])
		}
	}
	next, err := router.Start(context.Background(), "ai news", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Providers) != 3 {
		t.Fatalf("the next query after an empty one started %v, want every provider", next.Providers)
	}
	drain(t, next)
}

func TestRouter_ProviderEmptyOnEveryQueryIsBenched(t *testing.T) {
	store := NewMemoryHealthStore()
	router := NewRouter([]Provider{&stubProvider{name: "hollow", searchFn: func(ctx context.Context, query string, maxResults int) ([]Result, error) {
		return nil, nil
	}}}, store, Config{RequestTimeout: time.Second})
	for i := 0; i < EmptyStreakBeforeCooldown; i++ {
		stream, err := router.Start(context.Background(), "q", 5)
		if err != nil {
			t.Fatal(err)
		}
		drain(t, stream)
	}
	rec, err := store.Get("hollow")
	if err != nil {
		t.Fatal(err)
	}
	if rec.State != StateCoolingDown {
		t.Fatalf("a provider empty on %d queries in a row was not benched: %+v", EmptyStreakBeforeCooldown, rec)
	}
}
