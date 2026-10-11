package searchplane

import (
	"context"
	"testing"
	"time"
)

// docs/problems/search-plane-empty-result-benches-healthy-providers-2026-10-11.md.
// Failure modes: one narrow query that finds nothing cools down every
// provider it reached, so the next caller's broad query gets an outage; a
// provider that returns nothing on every query is never benched.
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
	router := NewRouter([]Provider{answers("a"), answers("b"), answers("c")}, store, Config{
		ProvidersPerQuery: 3, MinMergedResults: 1, MaxWaves: 1, RequestTimeout: time.Second,
	})
	if _, err := router.Search(context.Background(), "site:example.invalid nothing here", 5); err == nil {
		t.Fatal("a query nobody can answer returned results")
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
	if _, err := router.Search(context.Background(), "ai news", 5); err != nil {
		t.Fatalf("the next query after an empty one got an outage: %v", err)
	}
}

func TestRouter_ProviderEmptyOnEveryQueryIsBenched(t *testing.T) {
	store := NewMemoryHealthStore()
	router := NewRouter([]Provider{&stubProvider{name: "hollow", searchFn: func(ctx context.Context, query string, maxResults int) ([]Result, error) {
		return nil, nil
	}}}, store, Config{ProvidersPerQuery: 1, MinMergedResults: 1, MaxWaves: 1, RequestTimeout: time.Second})
	for i := 0; i < EmptyStreakBeforeCooldown; i++ {
		_, _ = router.Search(context.Background(), "q", 5)
	}
	rec, err := store.Get("hollow")
	if err != nil {
		t.Fatal(err)
	}
	if rec.State != StateCoolingDown {
		t.Fatalf("a provider empty on %d queries in a row was not benched: %+v", EmptyStreakBeforeCooldown, rec)
	}
}
