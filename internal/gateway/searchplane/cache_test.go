package searchplane

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Result cache and in-flight join
// (docs/archive/problems/search-is-duplicated-and-discarded-2026-10-14.md). Ways it
// can fail, numbered as in that record:
//  1. A cached answer is served after its TTL (news goes stale).
//  2. A failed or empty search is cached and hides a later success.
//  3. One scope's search answers another scope's request.
//  4. A cached or joined answer records provider outcomes it did not cause
//     (covered at the router level below).
//  5. A join deadlocks, duplicates or loses arrivals, or the first caller
//     leaving cancels the joiner's search.
//  6. Memory grows without bound.
//  8. Shared result slices race between concurrent searches (-race).
//  9. The kill switch (TTL 0) does not restore the uncached path.
// 10. Normalization merges queries that differ in meaning.
// 11. A hit or join is indistinguishable from a live search (Served).

// fakeSearch is a search the test controls: Start returns a Stream whose
// arrivals the test sends, and counts how often providers were started.
type fakeSearch struct {
	starts   atomic.Int32
	mu       sync.Mutex
	ctxs     []context.Context
	chans    []chan Arrival
	provider []string
	err      error
}

func (f *fakeSearch) start(providers ...string) StartFunc {
	return func(ctx context.Context) (*Stream, error) {
		f.starts.Add(1)
		if f.err != nil {
			return nil, f.err
		}
		ch := make(chan Arrival, len(providers))
		f.mu.Lock()
		f.ctxs = append(f.ctxs, ctx)
		f.chans = append(f.chans, ch)
		f.mu.Unlock()
		return &Stream{Providers: append([]string{}, providers...), Arrivals: ch}, nil
	}
}

func (f *fakeSearch) send(i int, a Arrival) {
	f.mu.Lock()
	ch := f.chans[i]
	f.mu.Unlock()
	ch <- a
}

func (f *fakeSearch) close(i int) {
	f.mu.Lock()
	ch := f.chans[i]
	f.mu.Unlock()
	close(ch)
}

func hit(provider string, urls ...string) Arrival {
	a := Arrival{Provider: provider, Status: string(OutcomeSuccess), LatencyMs: 120}
	for _, u := range urls {
		a.Results = append(a.Results, Result{Title: "t " + u, URL: u, Snippet: strings.Repeat("x", 100), Provider: provider})
	}
	if len(urls) == 0 {
		a.Status = string(OutcomeSuccessEmpty)
	}
	return a
}

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func testCache(cfg CacheConfig) (*Cache, *clock) {
	if cfg.TTL == 0 {
		cfg.TTL = 30 * time.Minute
	}
	if cfg.FreshTTL == 0 {
		cfg.FreshTTL = 10 * time.Minute
	}
	if cfg.MaxBytes == 0 {
		cfg.MaxBytes = 1 << 20
	}
	if cfg.Bound == 0 {
		cfg.Bound = 5 * time.Second
	}
	c := NewCache(cfg)
	clk := &clock{t: time.Date(2026, 10, 14, 15, 0, 0, 0, time.UTC)}
	c.now = clk.now
	return c, clk
}

// answer runs one search to completion through the cache with the fake
// answering every provider at once.
func answer(t *testing.T, c *Cache, f *fakeSearch, scope, query string, arrivals ...Arrival) (*Stream, []Arrival) {
	t.Helper()
	names := make([]string, len(arrivals))
	for i, a := range arrivals {
		names[i] = a.Provider
	}
	before := int(f.starts.Load())
	s, err := c.Start(context.Background(), scope, query, 40, f.start(names...))
	if err != nil {
		t.Fatal(err)
	}
	if int(f.starts.Load()) > before {
		idx := len(f.chans) - 1
		for _, a := range arrivals {
			f.send(idx, a)
		}
		f.close(idx)
	}
	got := drain(t, s)
	c.waitIdle()
	return s, got
}

func TestCacheHitReplaysWithoutStartingProviders(t *testing.T) {
	c, _ := testCache(CacheConfig{})
	f := &fakeSearch{}
	first, _ := answer(t, c, f, "vm-a", "parlioSDR README", hit("keenable", "https://a.example/1", "https://a.example/2"), hit("brave", "https://b.example/1"))
	if first.Served != ServedMiss {
		t.Fatalf("first served = %q, want miss", first.Served)
	}
	second, got := answer(t, c, f, "vm-a", "parlioSDR   README ", hit("keenable", "https://other.example/"))
	if f.starts.Load() != 1 {
		t.Fatalf("providers started %d times, want 1 (second search is a hit)", f.starts.Load())
	}
	if second.Served != ServedHit || second.CachedAt.IsZero() {
		t.Fatalf("second served = %q cached_at=%v, want hit with cached_at", second.Served, second.CachedAt)
	}
	if strings.Join(second.Providers, ",") != "keenable,brave" || len(got) != 2 {
		t.Fatalf("replay providers = %v arrivals = %d, want keenable,brave and 2", second.Providers, len(got))
	}
	if got[0].Results[0].URL != "https://a.example/1" || got[0].LatencyMs != 0 {
		t.Fatalf("replayed arrival = %+v, want the cached results with latency 0", got[0])
	}
	if len(second.Cooling) != 0 {
		t.Fatalf("a hit skipped no provider; cooling = %v", second.Cooling)
	}
}

func TestCacheExpiresAfterTTL(t *testing.T) { // failure mode 1
	c, clk := testCache(CacheConfig{TTL: 30 * time.Minute})
	f := &fakeSearch{}
	answer(t, c, f, "vm-a", "Halo Trust annual report", hit("keenable", "https://h.example/"))
	clk.add(29 * time.Minute)
	if s, _ := answer(t, c, f, "vm-a", "Halo Trust annual report", hit("keenable", "https://h.example/")); s.Served != ServedHit {
		t.Fatalf("within TTL served = %q, want hit", s.Served)
	}
	clk.add(2 * time.Minute)
	if s, _ := answer(t, c, f, "vm-a", "Halo Trust annual report", hit("keenable", "https://h.example/")); s.Served != ServedMiss {
		t.Fatalf("after TTL served = %q, want miss", s.Served)
	}
	if f.starts.Load() != 2 {
		t.Fatalf("starts = %d, want 2", f.starts.Load())
	}
}

func TestCacheTimeSensitiveQueriesUseFreshTTL(t *testing.T) { // failure mode 1
	c, clk := testCache(CacheConfig{TTL: 30 * time.Minute, FreshTTL: 10 * time.Minute})
	f := &fakeSearch{}
	for _, q := range []string{"AI regulation news this week", "latest Mauritius riots", "Morgan Stanley results October 2026"} {
		answer(t, c, f, "vm-a", q, hit("keenable", "https://n.example/"))
	}
	clk.add(11 * time.Minute)
	for _, q := range []string{"AI regulation news this week", "latest Mauritius riots", "Morgan Stanley results October 2026"} {
		if s, _ := answer(t, c, f, "vm-a", q, hit("keenable", "https://n.example/")); s.Served != ServedMiss {
			t.Fatalf("%q after fresh TTL served = %q, want miss", q, s.Served)
		}
	}
}

func TestCacheDoesNotStoreSearchWithoutResults(t *testing.T) { // failure mode 2
	c, _ := testCache(CacheConfig{})
	f := &fakeSearch{}
	failed := Arrival{Provider: "keenable", Status: string(OutcomeServerError), Error: "status 502"}
	answer(t, c, f, "vm-a", "q", failed, hit("brave"))
	if s, _ := answer(t, c, f, "vm-a", "q", hit("keenable", "https://k.example/")); s.Served != ServedMiss {
		t.Fatalf("after a failed+empty search, served = %q, want miss", s.Served)
	}
	if s, _ := answer(t, c, f, "vm-a", "q"); s.Served != ServedHit {
		t.Fatalf("after a search with results, served = %q, want hit", s.Served)
	}
}

func TestCacheKeepsOnlyArrivalsWithResults(t *testing.T) { // failure mode 2
	c, _ := testCache(CacheConfig{})
	f := &fakeSearch{}
	failed := Arrival{Provider: "brave", Status: string(OutcomeRateLimited), Error: "429"}
	answer(t, c, f, "vm-a", "q", hit("keenable", "https://k.example/"), failed, hit("exa"))
	s, got := answer(t, c, f, "vm-a", "q")
	if s.Served != ServedHit || strings.Join(s.Providers, ",") != "keenable" || len(got) != 1 {
		t.Fatalf("hit providers = %v arrivals = %d, want keenable only", s.Providers, len(got))
	}
}

func TestCacheScopesDoNotShare(t *testing.T) { // failure mode 3
	c, _ := testCache(CacheConfig{})
	f := &fakeSearch{}
	answer(t, c, f, "vm-a", "q", hit("keenable", "https://k.example/"))
	if s, _ := answer(t, c, f, "vm-b", "q", hit("keenable", "https://k.example/")); s.Served != ServedMiss {
		t.Fatalf("other scope served = %q, want miss", s.Served)
	}
	if f.starts.Load() != 2 {
		t.Fatalf("starts = %d, want 2", f.starts.Load())
	}
}

func TestCacheJoinsInFlightSearch(t *testing.T) { // failure mode 5
	c, _ := testCache(CacheConfig{})
	f := &fakeSearch{}
	leader, err := c.Start(context.Background(), "vm-a", "q", 40, f.start("keenable", "brave"))
	if err != nil {
		t.Fatal(err)
	}
	f.send(0, hit("keenable", "https://k.example/"))
	joiner, err := c.Start(context.Background(), "vm-a", "q", 40, f.start("keenable", "brave"))
	if err != nil {
		t.Fatal(err)
	}
	if joiner.Served != ServedJoined || f.starts.Load() != 1 {
		t.Fatalf("joiner served = %q starts = %d, want joined and 1", joiner.Served, f.starts.Load())
	}
	if strings.Join(joiner.Providers, ",") != "keenable,brave" {
		t.Fatalf("joiner providers = %v", joiner.Providers)
	}
	f.send(0, hit("brave", "https://b.example/"))
	f.close(0)
	for name, s := range map[string]*Stream{"leader": leader, "joiner": joiner} {
		got := drain(t, s)
		if len(got) != 2 || got[0].Provider != "keenable" || got[1].Provider != "brave" {
			t.Fatalf("%s arrivals = %+v, want keenable then brave once each", name, got)
		}
	}
	c.waitIdle()
	if s, _ := answer(t, c, f, "vm-a", "q"); s.Served != ServedHit {
		t.Fatalf("after the flight, served = %q, want hit", s.Served)
	}
}

func TestCacheJoinSurvivesFirstCallerLeaving(t *testing.T) { // failure mode 5
	c, _ := testCache(CacheConfig{})
	f := &fakeSearch{}
	leaderCtx, leave := context.WithCancel(context.Background())
	leader, err := c.Start(leaderCtx, "vm-a", "q", 40, f.start("keenable"))
	if err != nil {
		t.Fatal(err)
	}
	joiner, err := c.Start(context.Background(), "vm-a", "q", 40, f.start("keenable"))
	if err != nil {
		t.Fatal(err)
	}
	leave()
	drain(t, leader) // the leaving caller's stream closes
	f.mu.Lock()
	searchCtx := f.ctxs[0]
	f.mu.Unlock()
	if searchCtx.Err() != nil {
		t.Fatalf("the shared search was cancelled with its first caller: %v", searchCtx.Err())
	}
	f.send(0, hit("keenable", "https://k.example/"))
	f.close(0)
	if got := drain(t, joiner); len(got) != 1 {
		t.Fatalf("joiner arrivals = %d, want 1", len(got))
	}
}

func TestCacheStartErrorReachesJoinersAndIsNotCached(t *testing.T) {
	c, _ := testCache(CacheConfig{})
	f := &fakeSearch{err: errors.New("no search providers available")}
	if _, err := c.Start(context.Background(), "vm-a", "q", 40, f.start("keenable")); err == nil {
		t.Fatal("start error was swallowed")
	}
	f.err = nil
	if s, _ := answer(t, c, f, "vm-a", "q", hit("keenable", "https://k.example/")); s.Served != ServedMiss {
		t.Fatalf("after a start error, served = %q, want a fresh miss", s.Served)
	}
}

func TestCacheEvictsOldestOverMaxBytes(t *testing.T) { // failure mode 6
	c, clk := testCache(CacheConfig{MaxBytes: 1000})
	f := &fakeSearch{}
	for i := 0; i < 10; i++ {
		answer(t, c, f, "vm-a", "q"+string(rune('a'+i)), hit("keenable", "https://k.example/"+string(rune('a'+i))))
		clk.add(time.Second)
	}
	if c.bytesHeld() > 1000 {
		t.Fatalf("cache holds %d bytes, cap 1000", c.bytesHeld())
	}
	if s, _ := answer(t, c, f, "vm-a", "qj", hit("keenable", "https://k.example/j")); s.Served != ServedHit {
		t.Fatalf("newest entry served = %q, want hit", s.Served)
	}
	if s, _ := answer(t, c, f, "vm-a", "qa", hit("keenable", "https://k.example/a")); s.Served != ServedMiss {
		t.Fatalf("oldest entry served = %q, want miss (evicted)", s.Served)
	}
}

func TestCacheHitsDoNotShareResultSlices(t *testing.T) { // failure mode 8
	c, _ := testCache(CacheConfig{})
	f := &fakeSearch{}
	answer(t, c, f, "vm-a", "q", hit("keenable", "https://k.example/"))
	_, got := answer(t, c, f, "vm-a", "q")
	got[0].Results[0].URL = "mutated"
	_, again := answer(t, c, f, "vm-a", "q")
	if again[0].Results[0].URL != "https://k.example/" {
		t.Fatalf("a caller's mutation reached the cache: %q", again[0].Results[0].URL)
	}
}

func TestCacheConcurrentIdenticalSearchesStartOnce(t *testing.T) { // failure modes 5, 8
	c, _ := testCache(CacheConfig{})
	f := &fakeSearch{}
	var wg sync.WaitGroup
	streams := make([]*Stream, 20)
	for i := range streams {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s, err := c.Start(context.Background(), "vm-a", "same", 40, f.start("keenable"))
			if err != nil {
				t.Error(err)
				return
			}
			streams[i] = s
		}(i)
	}
	wg.Wait()
	if f.starts.Load() != 1 {
		t.Fatalf("starts = %d, want 1", f.starts.Load())
	}
	f.send(0, hit("keenable", "https://k.example/"))
	f.close(0)
	for i, s := range streams {
		if got := drain(t, s); len(got) != 1 {
			t.Fatalf("stream %d got %d arrivals, want 1", i, len(got))
		}
	}
}

func TestNilCacheIsTheUncachedPath(t *testing.T) { // failure mode 9
	if NewCache(CacheConfig{TTL: 0}) != nil {
		t.Fatal("TTL 0 must disable the cache")
	}
	var c *Cache
	f := &fakeSearch{}
	for i := 0; i < 2; i++ {
		s, err := c.Start(context.Background(), "vm-a", "q", 40, f.start("keenable"))
		if err != nil {
			t.Fatal(err)
		}
		if s.Served != ServedOff {
			t.Fatalf("served = %q, want off", s.Served)
		}
		f.close(i)
	}
	if f.starts.Load() != 2 {
		t.Fatalf("starts = %d, want 2", f.starts.Load())
	}
}

func TestNormalizeQueryKeepsMeaning(t *testing.T) { // failure mode 10
	same := [][2]string{
		{"parlioSDR  README", " parlioSDR README "},
		{"a\tb\nc", "a b c"},
	}
	for _, p := range same {
		if NormalizeQuery(p[0]) != NormalizeQuery(p[1]) {
			t.Fatalf("%q and %q should share a key", p[0], p[1])
		}
	}
	differ := [][2]string{
		{`"raw 14-bit IQ"`, `raw 14-bit IQ`},
		{`smiSDR OR parlioSDR`, `smiSDR or parlioSDR`},
		{`site:github.com parlioSDR`, `parlioSDR`},
		{`Apple`, `apple`},
	}
	for _, p := range differ {
		if NormalizeQuery(p[0]) == NormalizeQuery(p[1]) {
			t.Fatalf("%q and %q must not share a key", p[0], p[1])
		}
	}
}

// Failure mode 4 at the router: a hit calls no provider and records
// nothing in the health store.
func TestCacheHitRecordsNoProviderOutcome(t *testing.T) {
	var calls atomic.Int32
	p := providerFunc{name: "keenable", fn: func(ctx context.Context, q string, n int) ([]Result, error) {
		calls.Add(1)
		return []Result{{Title: "t", URL: "https://k.example/"}}, nil
	}}
	store := NewMemoryHealthStore()
	r := NewRouter([]Provider{p}, store, Config{RequestTimeout: time.Second})
	c, _ := testCache(CacheConfig{})
	for i := 0; i < 3; i++ {
		s, err := c.Start(context.Background(), "vm-a", "q", 40, func(ctx context.Context) (*Stream, error) {
			return r.Start(ctx, "q", 40)
		})
		if err != nil {
			t.Fatal(err)
		}
		drain(t, s)
		c.waitIdle()
	}
	if calls.Load() != 1 {
		t.Fatalf("provider called %d times, want 1", calls.Load())
	}
	snap, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if got := snap["keenable"].WindowAttempts; got != 1 {
		t.Fatalf("health attempts = %d, want 1 (hits record nothing)", got)
	}
}

// Failure mode 9 through the environment: the default turns the cache on,
// CHOIR_SEARCH_CACHE_TTL_SECONDS=0 turns it (and the join) off.
func TestCacheEnvKillSwitch(t *testing.T) {
	t.Setenv("CHOIR_SEARCH_CACHE_TTL_SECONDS", "")
	t.Setenv("CHOIR_SEARCH_CACHE_SCOPE", "")
	cfg := ConfigFromEnv()
	if cfg.CacheTTL != DefaultCacheTTL || cfg.CacheFreshTTL != DefaultCacheFreshTTL || cfg.CacheShared {
		t.Fatalf("defaults = %+v", cfg)
	}
	t.Setenv("CHOIR_SEARCH_CACHE_TTL_SECONDS", "0")
	if c := NewCache(CacheConfig{TTL: ConfigFromEnv().CacheTTL}); c != nil {
		t.Fatal("TTL 0 from the environment must disable the cache")
	}
	t.Setenv("CHOIR_SEARCH_CACHE_SCOPE", "shared")
	if !ConfigFromEnv().CacheShared {
		t.Fatal("CHOIR_SEARCH_CACHE_SCOPE=shared not read")
	}
}

type providerFunc struct {
	name string
	fn   func(ctx context.Context, q string, n int) ([]Result, error)
}

func (p providerFunc) Name() string { return p.name }
func (p providerFunc) Search(ctx context.Context, q string, n int) ([]Result, error) {
	return p.fn(ctx, q, n)
}
