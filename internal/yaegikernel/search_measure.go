package yaegikernel

import (
	"net"
	"net/url"
	"strings"
)

// What the next measurement reads from journals (Landing 0 of
// docs/definitions/corpus-first-search-one-verb-2026-11-19.md): why in-run
// reuse declined a prior identical search, how many of a run's searches came
// back mostly already held, how many distinct outlets the run held at its
// sends. Counts and fixed tokens only: no query text, no URL.

// seenMostlyPercent is the share of a search's pages already held at which the
// search is counted as "mostly seen" in the run's line. A reading threshold for
// the measurement, not a gate.
const seenMostlyPercent = 75

// noteDeclined counts why reuse declined a prior identical search. An empty
// reason (nothing prior was identical) counts nothing.
func (s *ChoirScope) noteDeclined(reason string) {
	if s == nil || reason == "" {
		return
	}
	s.statsMu.Lock()
	if s.declineStat == nil {
		s.declineStat = map[string]int{}
	}
	s.declineStat[reason]++
	s.statsMu.Unlock()
}

// noteFreshOverMatch counts a search sent with fresh although a finished
// search of this run would have answered it.
func (s *ChoirScope) noteFreshOverMatch(query string) {
	if s == nil || s.searches == nil {
		return
	}
	if c, _, _ := s.searches.reuseVerdict(query); c.ID != "" {
		s.noteDeclined("fresh")
	}
}

// noteOutlets records how many distinct outlets the run held as a search that
// the host accepted was asked.
func (s *ChoirScope) noteOutlets(n int) {
	if s == nil {
		return
	}
	s.statsMu.Lock()
	s.outletsLast = n
	s.outletsSends++
	if n > s.outletsMax {
		s.outletsMax = n
	}
	s.statsMu.Unlock()
}

// outletsHeld is the number of distinct outlets (registrable domains) among
// the pages this run's searches returned.
func (st *SearchStore) outletsHeld() int {
	if st == nil {
		return 0
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	seen := map[string]bool{}
	for key := range st.first {
		if o := outletOf(key); o != "" {
			seen[o] = true
		}
	}
	return len(seen)
}

// secondLevel are the labels that make a two-letter country domain a public
// suffix of three labels (bbc.co.uk, data.gov.uk). A small list, not the
// public suffix list (not vendored): outlet counts are a measurement, and a
// miss counts one outlet too few or too many by a registry, never a decision.
var secondLevel = map[string]bool{"co": true, "com": true, "org": true, "net": true, "gov": true, "ac": true, "edu": true, "or": true, "ne": true, "go": true}

// outletOf is the registrable domain of a page URL: the last two host labels,
// or three when the last is a two-letter country code and the one before is a
// second-level label (co, com, gov ...). An IP or a single-label host is
// itself. Empty for a URL with no host.
func outletOf(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return ""
	}
	if net.ParseIP(host) != nil {
		return host
	}
	labels := strings.Split(strings.Trim(host, "."), ".")
	n := len(labels)
	switch {
	case n <= 2:
		return strings.Join(labels, ".")
	case len(labels[n-1]) == 2 && secondLevel[labels[n-2]]:
		return strings.Join(labels[n-3:], ".")
	default:
		return strings.Join(labels[n-2:], ".")
	}
}
