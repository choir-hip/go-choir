package yaegikernel

import (
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/searchquery"
)

// The hold on re-asks (Landing 1 of docs/definitions/
// corpus-first-search-one-verb-2026-11-19.md). Both rules are need tests on a
// QUESTION, in the style of the unread test (search_need.go): a correctness
// refusal that hands back what the run already holds and says what to do
// instead, with the fresh escape. Neither counts searches: a run that asks ten
// different questions is never held by them, however many it sends.
//
//   - the same question again (searchquery.NearKeyAt): answered from the earlier
//     search's pages by reuseSearch, not sent;
//   - the web has shown what it has (mostlySeen): the latest search for this
//     question added under a quarter new pages and the run holds pages for it it
//     has not tried to read.
//
// Neither rule holds a search that changes WHERE to look. A story publishes
// only on two distinct outlets and a run hunts the second with a site:
// restriction, so a different site: restriction is a different question (it is
// in the near key), and mostlySeen never holds a wording whose site: restriction
// names an outlet the run holds no page from.
//
// Threshold sources. Every number below is a HYPOTHESIS unless it names its
// source; Landing 0's fields settle them: not_sent mostly_seen and from_held on
// the run line, against pages the run later read and cited (searches_judged,
// searches_mostly_seen give the base rate), with resumed_segment lines left out.

const (
	// holdMinPages: a search that returned fewer pages is not judged "mostly
	// held". HYPOTHESIS (no measurement behind 8; the searches in the
	// 2026-11-20 window returned 40 to 60 results, so 8 is far below a typical
	// search and only keeps a 3-page answer from being read as "all held").
	holdMinPages = 8
	// holdNewPercent: under this share of new pages, a search showed the run
	// what it already had. 25 is the lead's rule ("under a quarter new"); the
	// replay measured 85% already seen by the 4th search of the worst run
	// (eb7e) and 47% of the heavy runs' searches near-repeated. HYPOTHESIS for
	// the exact line; searches_mostly_seen (75% held) is the matching reading.
	holdNewPercent = 25
	// holdRelatedPercent is the word overlap (Jaccard over near-key terms) at
	// which two wordings are the same question. SOURCE: the 2026-11-20
	// measurement used word-set Jaccard 0.6 for "near-duplicate wording" (what-
	// starts-research-runs-and-what-is-new, section 3).
	holdRelatedPercent = 60
	// holdRelatedShared is a floor on the terms two wordings share: a short
	// query like {kharkiv missile attack death toll} against {kharkiv missile
	// attack hospital} reaches 60% with two claims about one event. HYPOTHESIS:
	// 4 shared terms keeps two narrow questions on one event apart; settled by
	// mostly_seen refusals whose next send (fresh) returned pages read and cited.
	holdRelatedShared = 4
)

// nearOf is a query's near key and its terms at now; both empty when nothing
// is left.
func nearOf(query string, now time.Time) (string, []string) {
	k := searchquery.NearKeyAt(query, now)
	if k == "" {
		return "", nil
	}
	return k, strings.Fields(k)
}

// relatedTerms reports whether two term sets are the same question's
// wordings: Jaccard overlap of holdRelatedPercent or more AND at least
// holdRelatedShared shared terms.
func relatedTerms(a, b []string) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	in := map[string]bool{}
	for _, t := range a {
		in[t] = true
	}
	common := 0
	for _, t := range b {
		if in[t] {
			common++
		}
	}
	union := len(a) + len(b) - common
	return common >= holdRelatedShared && common*100 >= holdRelatedPercent*union
}

// holdsOutlet reports whether any page the run's searches returned is from the
// outlet (registrable domain) of host.
func (st *SearchStore) holdsOutletLocked(host string) bool {
	want := outletOf("https://" + host)
	for key := range st.first {
		if outletOf(key) == want {
			return true
		}
	}
	return false
}

// mostlySeen judges a new wording against what the run's searches for the same
// question returned. The question is the SearchAll question (qk) the earlier
// searches were sent for, or the earlier searches whose wording is related to
// this one AND to the first such search (the question's own wording, so a
// chain of drifting wordings is not one question). It reports the pages the
// run holds for it that it has not tried to read, and whether the wording is
// held: the latest finished search for the question returned holdMinPages or
// more pages, under holdNewPercent of them new to the run, and at least one
// page the run holds for it is untried. When every page held for the question
// has been tried (read, or failed: a paywall, a dead link) nothing readable is
// left to read, and the wording is sent.
func (st *SearchStore) mostlySeen(query, qk string, now time.Time) (pages []SearchHit, held bool) {
	if st == nil {
		return nil, false
	}
	_, terms := nearOf(query, now)
	st.mu.Lock()
	defer st.mu.Unlock()
	// A wording restricted to an outlet the run holds no page from asks for
	// pages that are new outlets by construction: never held.
	for _, host := range searchquery.SiteRestrictions(query) {
		if !st.holdsOutletLocked(host) {
			return nil, false
		}
	}
	var tried map[string]bool
	if st.triedKeys != nil {
		tried = st.triedKeys()
	}
	read := st.readSet()
	var group []int
	var anchor []string
	for i := range st.order {
		if !st.inRun(i) {
			continue
		}
		c := st.searches[st.order[i]]
		if !c.Done {
			continue
		}
		switch {
		case qk != "" && c.question == qk:
			group = append(group, i)
		case relatedTerms(terms, c.nearTerms) && (anchor == nil || relatedTerms(anchor, c.nearTerms)):
			if anchor == nil {
				anchor = c.nearTerms
			}
			group = append(group, i)
		}
	}
	if len(group) == 0 {
		return nil, false
	}
	m := st.markOneLocked(group[len(group)-1], read)
	if len(m.Results) < holdMinPages || m.NewPages*100 >= holdNewPercent*len(m.Results) {
		return nil, false
	}
	seen := map[string]bool{}
	for _, i := range group {
		for _, h := range st.markOneLocked(i, read).Results {
			if key := searchHitKey(h.URL); !tried[key] && !h.Read && !seen[key] {
				seen[key] = true
				pages = append(pages, h)
			}
		}
	}
	return pages, len(pages) > 0
}

// mostlySeenRefusal is what the desk is told when the web has shown what it has
// for the question: why, what the run holds, what to do instead, and what
// releases the hold.
func mostlySeenRefusal(n int) string {
	return fmt.Sprintf("the web has shown you what it has for this question: its latest search returned mostly pages you already held, and you hold %d pages for it that you have not tried to read. Read what you hold or conclude not found; do not ask the web again for the same thing. The hold ends once you have tried every page you hold for it (a page that will not load counts once tried: if all of them fail, nothing readable is held and the next wording is sent). To send a search for something those pages cannot hold, say why: choir.SearchFresh(query, why)", n)
}
