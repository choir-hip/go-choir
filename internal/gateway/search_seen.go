package gateway

import (
	"hash/fnv"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/gateway/searchplane"
)

// searchSeenPerComputer bounds the URLs remembered per computer (8 bytes
// of hash and a time each).
const searchSeenPerComputer = 50000

// searchURLSeen remembers, per computer, the result URLs returned to it in
// the last ttl, so the per-search log line can say how many of a search's
// URLs the computer had already received
// (docs/archive/problems/search-is-duplicated-and-discarded-2026-10-14.md: result
// duplication was not measurable). Measurement only: it changes no answer.
type searchURLSeen struct {
	ttl time.Duration
	max int
	now func() time.Time

	mu         sync.Mutex
	byComputer map[string]map[uint64]time.Time
}

func newSearchURLSeen(ttl time.Duration, max int) *searchURLSeen {
	return &searchURLSeen{ttl: ttl, max: max, now: time.Now, byComputer: map[string]map[uint64]time.Time{}}
}

// observe records one search's result URLs for a computer and returns how
// many distinct ones it had already received within ttl.
func (s *searchURLSeen) observe(computerID string, urls []string) (seen int) {
	if s == nil {
		return 0
	}
	keys := distinctURLKeys(urls)
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.byComputer[computerID]
	if m == nil {
		m = map[uint64]time.Time{}
		s.byComputer[computerID] = m
	}
	for _, k := range keys {
		if at, ok := m[k]; ok && now.Sub(at) < s.ttl {
			seen++
		}
		m[k] = now
	}
	if len(m) > s.max {
		for k, at := range m {
			if now.Sub(at) >= s.ttl {
				delete(m, k)
			}
		}
	}
	if len(m) > s.max {
		// Still full of live URLs: keep only this search's, newest first.
		m = map[uint64]time.Time{}
		for i := len(keys) - 1; i >= 0 && len(m) < s.max; i-- {
			m[keys[i]] = now
		}
		s.byComputer[computerID] = m
	}
	return seen
}

func (s *searchURLSeen) size(computerID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.byComputer[computerID])
}

func distinctURLKeys(urls []string) []uint64 {
	seen := map[uint64]bool{}
	var out []uint64
	for _, raw := range urls {
		k := searchURLKey(raw)
		if raw == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	return out
}

// searchURLKey hashes a URL with the fragment dropped, the host lowercased
// and a trailing slash trimmed.
func searchURLKey(raw string) uint64 {
	raw = strings.TrimSpace(raw)
	if u, err := url.Parse(raw); err == nil {
		u.Fragment = ""
		u.Host = strings.ToLower(u.Host)
		raw = u.String()
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(strings.TrimRight(raw, "/")))
	return h.Sum64()
}

// observeResultURLs returns the distinct URLs among a search's results and
// how many of them the computer already received within a day.
func (c *SearchClient) observeResultURLs(computerID string, urls []string) (unique, seen int) {
	unique = len(distinctURLKeys(urls))
	if c == nil {
		return unique, 0
	}
	return unique, c.seen.observe(computerID, urls)
}

// searchServed is the log's served= value; "off" when the cache is off.
func searchServed(stream *searchplane.Stream) string {
	if stream.Served == "" {
		return searchplane.ServedOff
	}
	return stream.Served
}
