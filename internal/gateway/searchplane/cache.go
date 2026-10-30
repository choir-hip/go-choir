package searchplane

import (
	"container/list"
	"context"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Result cache and in-flight join
// (docs/archive/problems/search-is-duplicated-and-discarded-2026-10-14.md,
// docs/archive/reports/search-efficiency-and-reuse-2026-10-14.md §1).
//
// An identical search (same scope, same query up to whitespace, same
// max_results) answered within the TTL is replayed from memory without
// calling any provider; one already in flight is joined. Only arrivals that
// answered with results are kept, so a failure or an empty answer never
// hides a later success. A hit or a join records nothing in the health
// store: no provider was called on its behalf.

// Served says how a search was answered; the gateway logs it so a cached
// answer is never counted as a live search.
const (
	ServedMiss   = "miss"   // providers were started for this search
	ServedHit    = "hit"    // replayed from the cache
	ServedJoined = "joined" // joined an identical search in flight
	ServedOff    = "off"    // the cache is disabled
)

// CacheConfig bounds the cache. TTL <= 0 disables it (NewCache returns nil).
type CacheConfig struct {
	// TTL is how long an answer is served; FreshTTL replaces it for
	// time-sensitive queries (IsTimeSensitive) when shorter.
	TTL      time.Duration
	FreshTTL time.Duration
	// MaxBytes caps the result text held (titles, URLs, snippets);
	// the oldest entries go first.
	MaxBytes int64
	// Bound limits a shared search run on a context detached from its
	// first caller: the provider timeout plus the pace wait, plus the
	// escalation window under budget routing.
	Bound time.Duration
}

// StartFunc starts providers for a search (Router.Start).
type StartFunc func(ctx context.Context) (*Stream, error)

// Cache is safe for concurrent use. A nil *Cache is the uncached path.
type Cache struct {
	cfg CacheConfig
	now func() time.Time

	mu      sync.Mutex
	entries map[string]*list.Element // of *cacheEntry
	order   *list.List               // oldest first
	bytes   int64
	flights map[string]*flight
	running sync.WaitGroup // flights' pump goroutines (tests wait on it)
}

type cacheEntry struct {
	key       string
	cachedAt  time.Time
	expires   time.Time
	providers []string
	arrivals  []Arrival
	bytes     int64
}

// flight is one search in progress that later identical searches join.
type flight struct {
	ready chan struct{} // closed once providers (or err) are known
	err   error

	mu        sync.Mutex
	providers []string
	cooling   []Cooling
	// route is the shared search's routing record (escalated providers
	// join it after the start); capacity bounds its arrivals.
	route    *Route
	capacity int
	arrivals []Arrival
	done     bool
	wake     chan struct{} // closed and replaced on every change
}

// NewCache returns nil when cfg.TTL <= 0.
func NewCache(cfg CacheConfig) *Cache {
	if cfg.TTL <= 0 {
		return nil
	}
	if cfg.FreshTTL <= 0 || cfg.FreshTTL > cfg.TTL {
		cfg.FreshTTL = cfg.TTL
	}
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = 64 << 20
	}
	if cfg.Bound <= 0 {
		cfg.Bound = 45 * time.Second
	}
	return &Cache{
		cfg:     cfg,
		now:     time.Now,
		entries: map[string]*list.Element{},
		order:   list.New(),
		flights: map[string]*flight{},
	}
}

// NormalizeQuery collapses whitespace and nothing else: case, quotes and
// operators (OR, site:, phrases) can change what a provider returns.
func NormalizeQuery(q string) string {
	return strings.Join(strings.Fields(q), " ")
}

var timeSensitive = regexp.MustCompile(`(?i)\b(today|tonight|yesterday|this (week|morning|evening|month)|latest|breaking|live|now|just|hours? ago|minutes? ago|january|february|march|april|may|june|july|august|september|october|november|december)\b`)

// IsTimeSensitive reports whether a query asks about the news of the
// moment, which gets the shorter FreshTTL.
func IsTimeSensitive(q string) bool { return timeSensitive.MatchString(q) }

func cacheKey(scope, query string, maxResults int) string {
	return scope + "\x00" + NormalizeQuery(query) + "\x00" + strconv.Itoa(maxResults)
}

// Start answers from the cache, joins an identical search in flight, or
// calls start on a context detached from ctx (so a joiner is not cancelled
// when the first caller leaves) and shares its arrivals.
func (c *Cache) Start(ctx context.Context, scope, query string, maxResults int, start StartFunc) (*Stream, error) {
	if c == nil {
		s, err := start(ctx)
		if s != nil {
			s.Served = ServedOff
		}
		return s, err
	}
	key := cacheKey(scope, query, maxResults)
	c.mu.Lock()
	if el, found := c.entries[key]; found {
		e := el.Value.(*cacheEntry)
		if c.now().Before(e.expires) {
			c.mu.Unlock()
			return replay(query, e), nil
		}
		c.removeLocked(el)
	}
	if f, found := c.flights[key]; found {
		c.mu.Unlock()
		return f.join(ctx, query)
	}
	f := &flight{ready: make(chan struct{}), wake: make(chan struct{})}
	c.flights[key] = f
	c.mu.Unlock()

	shared, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.cfg.Bound)
	stream, err := start(shared)
	if err != nil {
		cancel()
		f.err = err
		close(f.ready)
		c.mu.Lock()
		delete(c.flights, key)
		c.mu.Unlock()
		return nil, err
	}
	f.providers = append([]string{}, stream.Providers...)
	f.cooling = append([]Cooling{}, stream.Cooling...)
	f.route = stream.Route
	f.capacity = max(cap(stream.Arrivals), len(stream.Providers))
	close(f.ready)
	c.running.Add(1)
	go c.pump(key, query, f, stream, cancel)
	s, err := f.join(ctx, query)
	if s != nil {
		s.Served = ServedMiss
	}
	return s, err
}

// pump copies the shared search's arrivals into the flight, then caches
// the arrivals that answered with results.
func (c *Cache) pump(key, query string, f *flight, stream *Stream, cancel context.CancelFunc) {
	defer c.running.Done()
	defer cancel()
	for a := range stream.Arrivals {
		f.mu.Lock()
		f.arrivals = append(f.arrivals, copyArrival(a))
		close(f.wake)
		f.wake = make(chan struct{})
		f.mu.Unlock()
	}
	f.mu.Lock()
	f.done = true
	close(f.wake)
	f.wake = make(chan struct{})
	var keep []Arrival
	var names []string
	var size int64
	for _, a := range f.arrivals {
		if a.Answered() && len(a.Results) > 0 {
			keep = append(keep, copyArrival(a))
			names = append(names, a.Provider)
			size += arrivalBytes(a)
		}
	}
	f.mu.Unlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.flights, key)
	if len(keep) == 0 || size > c.cfg.MaxBytes {
		return
	}
	ttl := c.cfg.TTL
	if IsTimeSensitive(query) {
		ttl = c.cfg.FreshTTL
	}
	now := c.now()
	if old, found := c.entries[key]; found {
		c.removeLocked(old)
	}
	e := &cacheEntry{key: key, cachedAt: now, expires: now.Add(ttl), providers: names, arrivals: keep, bytes: size}
	c.entries[key] = c.order.PushBack(e)
	c.bytes += size
	for c.bytes > c.cfg.MaxBytes {
		c.removeLocked(c.order.Front())
	}
}

func (c *Cache) removeLocked(el *list.Element) {
	e := el.Value.(*cacheEntry)
	c.order.Remove(el)
	delete(c.entries, e.key)
	c.bytes -= e.bytes
}

func (c *Cache) bytesHeld() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bytes
}

// waitIdle waits for every shared search's pump to finish (tests).
func (c *Cache) waitIdle() { c.running.Wait() }

// WaitIdleForTest waits until every shared search has been cached or
// dropped, so a test's next identical search is deterministically a hit.
func (c *Cache) WaitIdleForTest() {
	if c != nil {
		c.running.Wait()
	}
}

// join returns a stream of the flight's arrivals: those already landed,
// then the rest. Its buffer holds one arrival per provider, so the copier
// never blocks on a caller that stopped reading; it stops when ctx ends.
func (f *flight) join(ctx context.Context, query string) (*Stream, error) {
	select {
	case <-f.ready:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if f.err != nil {
		return nil, f.err
	}
	out := make(chan Arrival, f.capacity)
	s := &Stream{
		Query:     query,
		Providers: append([]string{}, f.providers...),
		Cooling:   append([]Cooling{}, f.cooling...),
		Route:     f.route,
		Arrivals:  out,
		Served:    ServedJoined,
	}
	go func() {
		defer close(out)
		sent := 0
		for {
			f.mu.Lock()
			batch := make([]Arrival, 0, len(f.arrivals)-sent)
			for _, a := range f.arrivals[sent:] {
				batch = append(batch, copyArrival(a))
			}
			sent = len(f.arrivals)
			done, wake := f.done, f.wake
			f.mu.Unlock()
			for _, a := range batch {
				out <- a
			}
			if done {
				return
			}
			select {
			case <-wake:
			case <-ctx.Done():
				return
			}
		}
	}()
	return s, nil
}

func replay(query string, e *cacheEntry) *Stream {
	out := make(chan Arrival, len(e.arrivals))
	for _, a := range e.arrivals {
		a = copyArrival(a)
		a.LatencyMs = 0
		out <- a
	}
	close(out)
	return &Stream{
		Query:     query,
		Providers: append([]string{}, e.providers...),
		Arrivals:  out,
		Served:    ServedHit,
		CachedAt:  e.cachedAt,
	}
}

func copyArrival(a Arrival) Arrival {
	a.Results = append([]Result(nil), a.Results...)
	return a
}

func arrivalBytes(a Arrival) int64 {
	n := int64(64)
	for _, r := range a.Results {
		n += int64(len(r.Title)+len(r.URL)+len(r.Snippet)+len(r.PublishedAt)+len(r.Provider)) + 64
	}
	return n
}
