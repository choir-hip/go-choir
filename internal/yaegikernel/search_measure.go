package yaegikernel

import (
	"golang.org/x/net/publicsuffix"
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
		if o := pageOutlet(key); o != "" {
			seen[o] = true
		}
	}
	return len(seen)
}

// outletOf is the registrable domain of a page URL, by the public suffix list
// (golang.org/x/net/publicsuffix): bbc.co.uk, theguardian.co.uk, abc.net.au and
// asahi.co.jp are each their own outlet, and www.bbc.co.uk and news.bbc.co.uk
// are one. An IP, a single-label host or a host that is itself a public suffix
// is itself. Empty for a URL with no host. This is the one function the stop
// rule, the site: rule and the research worked cell (choir.Outlet) share.
func outletOf(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	host := strings.Trim(strings.ToLower(u.Hostname()), ".")
	if host == "" {
		return ""
	}
	if net.ParseIP(host) != nil {
		return host
	}
	if d, err := publicsuffix.EffectiveTLDPlusOne(host); err == nil {
		return d
	}
	return host
}

// Outlet is the registrable domain of a page URL (choir.Outlet): the outlet a
// page belongs to, by the same rule the stop rule and the site: rule use. A page
// of an aggregator (news.google.com, msn.com: hosts that republish other
// outlets' stories) belongs to no outlet: it is "".
func (s *ChoirScope) Outlet(url string) string { return pageOutlet(url) }

// PageOutlet is pageOutlet for the host: the one outlet rule (the registrable domain by the
// public suffix list, "" for an aggregator's page) the stop rule, the decision's counts,
// choir.Outlet and the research digest's one-per-outlet choice share.
func PageOutlet(raw string) string { return pageOutlet(raw) }

// pageOutlet is the outlet a page belongs to: outletOf, or "" for an aggregator's
// page. The stop rule's counts, the decision's counts and choir.Outlet use it.
func pageOutlet(raw string) string {
	if aggregatorHosts[hostOf(raw)] {
		return ""
	}
	return outletOf(raw)
}
