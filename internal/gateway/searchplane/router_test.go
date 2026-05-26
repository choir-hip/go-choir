package searchplane

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type stubProvider struct {
	name      string
	searchFn  func(ctx context.Context, query string, maxResults int) ([]Result, error)
	searchCnt atomic.Int32
}

func (s *stubProvider) Name() string { return s.name }
func (s *stubProvider) Search(ctx context.Context, query string, maxResults int) ([]Result, error) {
	s.searchCnt.Add(1)
	if s.searchFn != nil {
		return s.searchFn(ctx, query, maxResults)
	}
	return nil, errors.New("stub failure")
}

func TestRouter_ParallelFanout(t *testing.T) {
	p1 := &stubProvider{name: "a", searchFn: func(ctx context.Context, query string, maxResults int) ([]Result, error) {
		return []Result{{Title: "A", URL: "http://example.com/a"}}, nil
	}}
	p2 := &stubProvider{name: "b", searchFn: func(ctx context.Context, query string, maxResults int) ([]Result, error) {
		return []Result{{Title: "B", URL: "http://example.com/b"}}, nil
	}}
	router := NewRouter([]Provider{p1, p2}, NewMemoryHealthStore(), Config{
		ProvidersPerQuery: 2,
		MinMergedResults:  1,
		MaxWaves:          1,
		RequestTimeout:    time.Second,
	})
	resp, err := router.Search(context.Background(), "test", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if p1.searchCnt.Load() != 1 || p2.searchCnt.Load() != 1 {
		t.Fatalf("parallel counts a=%d b=%d, want 1 each", p1.searchCnt.Load(), p2.searchCnt.Load())
	}
	if len(resp.Results) != 2 {
		t.Fatalf("results = %d, want 2", len(resp.Results))
	}
}

func TestRouter_SearchOutageWhenAllCooling(t *testing.T) {
	store := NewMemoryHealthStore()
	until := time.Now().Add(time.Hour)
	store.records["only"] = ProviderHealth{
		Provider:      "only",
		State:         StateCoolingDown,
		CooldownUntil: &until,
		StrikeCount:   1,
		UpdatedAt:     time.Now(),
	}
	router := NewRouter([]Provider{&stubProvider{name: "only"}}, store, Config{ProvidersPerQuery: 1, MinMergedResults: 1, MaxWaves: 1, RequestTimeout: time.Second})
	_, err := router.Search(context.Background(), "test", 5)
	if err == nil {
		t.Fatal("expected outage")
	}
	var outage *OutageError
	if !errors.As(err, &outage) {
		t.Fatalf("expected OutageError, got %T (%v)", err, err)
	}
}

func TestRouter_NoEmptySuccess(t *testing.T) {
	router := NewRouter([]Provider{&stubProvider{
		name: "empty",
		searchFn: func(ctx context.Context, query string, maxResults int) ([]Result, error) {
			return nil, nil
		},
	}}, NewMemoryHealthStore(), Config{ProvidersPerQuery: 1, MinMergedResults: 1, MaxWaves: 1, RequestTimeout: time.Second})
	_, err := router.Search(context.Background(), "test", 5)
	if err == nil {
		t.Fatal("expected outage for empty merged results")
	}
}
