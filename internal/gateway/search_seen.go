package gateway

import (
	"fmt"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"hash/fnv"
	"net/http"
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

// searchURLHashes lists the hashes (hex, comma-joined) of the first n result
// URLs, in order, as the served log line carries them.
func searchURLHashes(urls []string, n int) string {
	if len(urls) > n {
		urls = urls[:n]
	}
	parts := make([]string, 0, len(urls))
	for _, u := range urls {
		parts = append(parts, fmt.Sprintf("%x", searchURLKey(u)))
	}
	return strings.Join(parts, ",")
}

// modelCaller is the desk and run a model call's guest named, cleaned for the
// log line: desk=unknown and run=- when it sent none (an old guest, or a
// caller no desk or run owns), never an empty field. Logged only.
func modelCaller(r *http.Request) (desk, run string) {
	desk = searchAttributionValue(r, "X-Choir-Desk")
	if desk == "-" {
		desk = "unknown"
	}
	return desk, searchAttributionValue(r, "X-Choir-Run")
}

// searchAttributionValue is one attribution header cleaned for the log: no
// space or control character, at most 64 bytes, "-" when absent. A header
// cannot forge a second log line.
func searchAttributionValue(r *http.Request, name string) string {
	return logField(r.Header.Get(name), 64)
}

// logField cleans a guest-supplied string for a log line's key=value field: no
// space, control character or non-ASCII byte, at most max bytes, "-" when
// nothing is left. A guest's model name, tool_choice or reasoning effort can
// carry spaces, "=" and newlines; raw, they forge fields ("desk=fake") and
// whole lines that the money and status scripts parse
// (docs/problems/gateway-log-lines-carry-raw-guest-strings-2026-11-22.md).
func logField(s string, max int) string { return provider.LogField(s, max) }

// logModelField is a guest-named model, tool_choice or effort as the model-call
// lines carry it.
func logModelField(s string) string { return logField(s, 96) }

// logText is free text from a guest or an upstream (a provider's error, a
// reason) as one bounded, Go-quoted log token: a newline or a fake line stays
// inside the quotes.
func logText(s string) string { return provider.LogText(s, 300) }
