package searchplane

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// Async search (Cut 2 unit 2a,
// docs/archive/definitions/choir-reeng-cut2-evidence-to-publication-2026-10-12.md).
// The router starts every eligible provider at once and hands each one's
// completion to the caller as it lands. Ways it can fail:
//  1. Providers start in waves, so a later provider waits on an earlier one.
//  2. Arrivals come back in registration order, or all at once at the end,
//     so a slow provider delays the lines of faster ones.
//  3. A cooling provider is called anyway, or counted among the providers
//     the search started with (N), so it reads as a failure.
//  4. The arrival channel never closes, or closes before the last provider.

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

func manyResults(prefix string, n int) []Result {
	out := make([]Result, n)
	for i := range out {
		out[i] = Result{Title: fmt.Sprintf("%s %d", prefix, i), URL: fmt.Sprintf("https://%s.example/%d", prefix, i)}
	}
	return out
}

func delayedProvider(name string, delay time.Duration, n int) *stubProvider {
	return &stubProvider{name: name, searchFn: func(ctx context.Context, q string, max int) ([]Result, error) {
		select {
		case <-time.After(delay):
			return manyResults(name, n), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}
}

func drain(t *testing.T, stream *Stream) []Arrival {
	t.Helper()
	var out []Arrival
	timeout := time.After(10 * time.Second)
	for {
		select {
		case a, ok := <-stream.Arrivals:
			if !ok {
				return out
			}
			out = append(out, a)
		case <-timeout:
			t.Fatalf("arrival channel did not close; got %d arrivals", len(out))
		}
	}
}

func TestRouter_StartsEveryEligibleProviderAtOnce(t *testing.T) {
	release := make(chan struct{})
	var started atomic.Int32
	gated := func(name string) *stubProvider {
		return &stubProvider{name: name, searchFn: func(ctx context.Context, q string, max int) ([]Result, error) {
			started.Add(1)
			<-release
			return manyResults(name, 1), nil
		}}
	}
	router := NewRouter([]Provider{gated("a"), gated("b"), gated("c")}, NewMemoryHealthStore(), Config{RequestTimeout: 5 * time.Second})
	stream, err := router.Start(context.Background(), "q", 5)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(stream.Providers); got != "[a b c]" {
		t.Fatalf("providers = %s, want every eligible provider", got)
	}
	deadline := time.Now().Add(2 * time.Second)
	for started.Load() < 3 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if started.Load() != 3 {
		t.Fatalf("%d of 3 providers started before any finished; want all at once", started.Load())
	}
	close(release)
	if arrivals := drain(t, stream); len(arrivals) != 3 {
		t.Fatalf("arrivals = %d, want one per provider", len(arrivals))
	}
}

func TestRouter_ArrivalsInCompletionOrder(t *testing.T) {
	providers := []Provider{
		delayedProvider("slowest", 300*time.Millisecond, 1),
		delayedProvider("middle", 150*time.Millisecond, 1),
		delayedProvider("fastest", 0, 1),
	}
	router := NewRouter(providers, NewMemoryHealthStore(), Config{RequestTimeout: 5 * time.Second})
	stream, err := router.Start(context.Background(), "q", 5)
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, a := range drain(t, stream) {
		order = append(order, a.Provider)
		if a.Status != string(OutcomeSuccess) || len(a.Results) != 1 || a.Results[0].Provider != a.Provider {
			t.Fatalf("arrival %+v: want a success carrying its provider's results", a)
		}
	}
	if got := fmt.Sprint(order); got != "[fastest middle slowest]" {
		t.Fatalf("arrival order = %s, want completion order", got)
	}
}

func TestRouter_SlowProviderDoesNotDelayEarlierArrivals(t *testing.T) {
	slow := blockingProvider("slow")
	router := NewRouter([]Provider{slow, delayedProvider("fast", 0, 3)}, NewMemoryHealthStore(), Config{RequestTimeout: 5 * time.Second})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, err := router.Start(ctx, "q", 5)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case a := <-stream.Arrivals:
		if a.Provider != "fast" || len(a.Results) != 3 {
			t.Fatalf("first arrival = %+v, want the fast provider's results", a)
		}
	case <-time.After(time.Second):
		t.Fatal("the fast provider's arrival waited on the slow provider")
	}
	cancel()
	rest := drain(t, stream)
	if len(rest) != 1 || rest[0].Provider != "slow" || rest[0].Status != string(OutcomeCallerGone) {
		t.Fatalf("after cancel = %+v, want the slow provider closed as caller_gone", rest)
	}
}

func TestRouter_CoolingProvidersListedSeparately(t *testing.T) {
	store := NewMemoryHealthStore()
	until := time.Now().Add(time.Hour)
	store.records["cool"] = ProviderHealth{Provider: "cool", State: StateCoolingDown, CooldownUntil: &until, StrikeCount: 1, UpdatedAt: time.Now()}
	cool := delayedProvider("cool", 0, 1)
	router := NewRouter([]Provider{delayedProvider("a", 0, 1), cool, delayedProvider("b", 0, 1)}, store, Config{RequestTimeout: time.Second})
	stream, err := router.Start(context.Background(), "q", 5)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(stream.Providers); got != "[a b]" {
		t.Fatalf("providers = %s, want only the eligible ones (N excludes cooling)", got)
	}
	if len(stream.Cooling) != 1 || stream.Cooling[0].Provider != "cool" || stream.Cooling[0].Until.IsZero() {
		t.Fatalf("cooling = %+v, want the cooling provider listed with its until", stream.Cooling)
	}
	arrivals := drain(t, stream)
	if len(arrivals) != 2 || cool.searchCnt.Load() != 0 {
		t.Fatalf("arrivals = %+v, cooling calls = %d; a cooling provider is never called", arrivals, cool.searchCnt.Load())
	}
}

func TestRouter_AllCoolingStartsNoProvider(t *testing.T) {
	store := NewMemoryHealthStore()
	until := time.Now().Add(time.Hour)
	store.records["only"] = ProviderHealth{Provider: "only", State: StateCoolingDown, CooldownUntil: &until, StrikeCount: 1, UpdatedAt: time.Now()}
	router := NewRouter([]Provider{&stubProvider{name: "only"}}, store, Config{RequestTimeout: time.Second})
	stream, err := router.Start(context.Background(), "q", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(stream.Providers) != 0 || len(stream.Cooling) != 1 {
		t.Fatalf("stream = %+v, want no providers and one cooling", stream)
	}
	if arrivals := drain(t, stream); len(arrivals) != 0 {
		t.Fatalf("arrivals = %+v, want none", arrivals)
	}
}

func TestRouter_FailedProviderArrivesWithItsError(t *testing.T) {
	router := NewRouter([]Provider{&stubProvider{name: "broken"}, delayedProvider("ok", 0, 2)}, NewMemoryHealthStore(), Config{RequestTimeout: time.Second})
	stream, err := router.Start(context.Background(), "q", 5)
	if err != nil {
		t.Fatal(err)
	}
	answered := 0
	for _, a := range drain(t, stream) {
		if a.Answered() {
			answered++
			continue
		}
		if a.Provider != "broken" || a.Error == "" || a.Status != string(OutcomeError) {
			t.Fatalf("failed arrival = %+v, want the provider's error", a)
		}
	}
	if answered != 1 {
		t.Fatalf("answered = %d, want 1", answered)
	}
}
