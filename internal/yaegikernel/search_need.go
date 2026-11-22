package yaegikernel

import (
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/searchquery"
)

// What the run holds and had read when it searched: a measurement, never a gate.
// The owner's ruling (2026-11-22): "we shouldnt need rules. we should be able with
// prompts to encourage models to search the object graph first, and then the web".
// The read-first refusals (the first five pages of every search, search zero, site:
// holds, SearchFresh, the research tree's tried set) were removed with that ruling
// (docs/problems/read-first-reads-more-but-searches-as-much-2026-11-22.md): a run is
// GIVEN what the graph holds (the digest, research_digest.go) and TOLD to use it; the
// web stays open to it. What remains here is correctness (a search of the same terms
// is answered from the run's own earlier one; one search runs at a time) and the
// counts the proof reads.

// allCounts are a cell's SearchAll counts (SearchStats.AllCalls and the rest).
type allCounts struct {
	calls, offered, sent, untagged, running int
}

// tagQuestion records that search id was sent for the question with this key.
// It reports whether it did: false when the store does not hold the search
// (its start frame had not arrived), and then the gate cannot see that search.
func (st *SearchStore) tagQuestion(id, qk string) bool {
	if st == nil || qk == "" {
		return false
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	s := st.searches[id]
	if s == nil {
		return false
	}
	s.question = qk
	return true
}

// triedAll is the pages this session tried to read, whether or not the read worked.
// The caller holds st.mu.
func (st *SearchStore) triedAll() map[string]bool {
	tried := map[string]bool{}
	if st.triedKeys != nil {
		for k := range st.triedKeys() {
			tried[k] = true
		}
	}
	return tried
}

// aggregatorHosts are hosts that republish other outlets' stories: a page from
// one is not an outlet's own page, so it never makes a site: hunt "held" (and
// google.com or msn.com is not the outlet of news.google.com or msn.com's
// stories). There was no such list in the search code before; it is a short
// list of the hosts the paper's searches return, not a registry.
var aggregatorHosts = map[string]bool{
	"news.google.com": true, "msn.com": true, "news.yahoo.com": true, "flipboard.com": true,
	"newsbreak.com": true, "smartnews.com": true, "ground.news": true, "allsides.com": true,
	"news.apple.com": true, "bing.com": true, "dailyhunt.in": true, "newsnow.co.uk": true,
}

// needState is what the run held and had read when it asked, for the host's
// search.decision record: counts only.
type needState struct {
	earlier, returned, tried, read, outlets int
}

func (st *SearchStore) needState() needState {
	var n needState
	if st == nil {
		return n
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	tried := st.triedAll()
	read := st.readSet()
	outlets := map[string]bool{}
	for key := range read {
		if o := pageOutlet(key); o != "" {
			outlets[o] = true
		}
	}
	n.read, n.outlets = len(read), len(outlets)
	for i := range st.order {
		if !st.inRun(i) {
			continue
		}
		n.earlier++
		for _, h := range st.markOneLocked(i, read).Results {
			if h.Held {
				continue
			}
			n.returned++
			if tried[searchHitKey(h.URL)] {
				n.tried++
			}
		}
	}
	return n
}

// runningSearch returns the id of the latest search of this run still running
// (no done frame yet, started less than searchWaitMax ago), or "". A search that
// has run longer is the host's to end; it does not hold the next one back.
func (st *SearchStore) runningSearch() string {
	if st == nil {
		return ""
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	for i := len(st.order) - 1; i >= 0; i-- {
		if !st.inRun(i) {
			continue
		}
		if c := st.searches[st.order[i]]; !c.Done && st.nowTime().Sub(c.startedAt) < searchWaitMax {
			return c.ID
		}
	}
	return ""
}

// knows waits up to d until the store holds search id (its start frame has
// arrived), and reports whether it does.
func (st *SearchStore) knows(id string, d time.Duration) bool {
	if st == nil {
		return false
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	for {
		st.mu.Lock()
		_, ok := st.searches[id]
		changed := st.changed
		st.mu.Unlock()
		if ok {
			return true
		}
		select {
		case <-changed:
		case <-timer.C:
			return false
		}
	}
}

// questionKey is the key of a SearchAll question: the terms of its words, as
// every in-run key is (searchquery.Key). Empty when the question has none.
func questionKey(question string) string {
	return searchquery.Key(question)
}

// runningRefusal is what the desk is told when a search is started while another
// of its run is still running: that one's pages are not yet known, so this one
// cannot be judged by them. It returns at once; the cell is never held.
func runningRefusal(id string) string {
	return fmt.Sprintf("a search of this run is still running (%s). Wait for it (choir.SearchWait(%q, 15000)), read what it returns, then search again for what is still missing, naming it.", id, id)
}

// PageKey is the key two URLs of one page share.
func PageKey(raw string) string { return searchHitKey(raw) }

var _ = strings.TrimSpace
var _ = time.Second
