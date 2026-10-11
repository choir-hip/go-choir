package searchplane

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// docs/problems/search-wave-waits-for-its-slowest-provider-2026-10-11.md.
// Failure modes: a wave whose fast provider meets the target still waits
// for the slow one; the cancelled slow provider is struck or marks the
// result degraded; a slow provider's results are dropped when the fast ones
// fall short.
func manyResults(prefix string, n int) []Result {
	out := make([]Result, n)
	for i := range out {
		out[i] = Result{Title: fmt.Sprintf("%s %d", prefix, i), URL: fmt.Sprintf("https://%s.example/%d", prefix, i)}
	}
	return out
}

func TestRouter_WaveDoesNotWaitForSlowProviderOnceTargetIsMet(t *testing.T) {
	fast := &stubProvider{name: "fast", searchFn: func(ctx context.Context, q string, n int) ([]Result, error) {
		return manyResults("fast", 40), nil
	}}
	slow := blockingProvider("slow")
	store := NewMemoryHealthStore()
	router := NewRouter([]Provider{fast, slow}, store, Config{
		ProvidersPerQuery: 2, MinMergedResults: 40, MaxWaves: 1,
		RequestTimeout: 5 * time.Second, WaveGrace: 50 * time.Millisecond,
	})
	started := time.Now()
	resp, err := router.Search(context.Background(), "q", 40)
	if err != nil {
		t.Fatal(err)
	}
	if took := time.Since(started); took > time.Second {
		t.Fatalf("wave took %s, want it to return once the fast provider met the target", took)
	}
	if len(resp.Results) != 40 || resp.Degraded {
		t.Fatalf("results=%d degraded=%v", len(resp.Results), resp.Degraded)
	}
	assertNoStrike(t, store, "slow")
}

func TestRouter_WaveWaitsForSlowProviderWhenFastOnesFallShort(t *testing.T) {
	fast := &stubProvider{name: "fast", searchFn: func(ctx context.Context, q string, n int) ([]Result, error) {
		return manyResults("fast", 3), nil
	}}
	slow := &stubProvider{name: "slow", searchFn: func(ctx context.Context, q string, n int) ([]Result, error) {
		select {
		case <-time.After(300 * time.Millisecond):
			return manyResults("slow", 10), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}
	router := NewRouter([]Provider{fast, slow}, NewMemoryHealthStore(), Config{
		ProvidersPerQuery: 2, MinMergedResults: 40, MaxWaves: 1,
		RequestTimeout: 5 * time.Second, WaveGrace: 50 * time.Millisecond,
	})
	resp, err := router.Search(context.Background(), "q", 40)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 13 {
		t.Fatalf("results = %d, want the slow provider's results included", len(resp.Results))
	}
}
