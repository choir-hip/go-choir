package yaegikernel

import (
	"fmt"
	"time"

	"github.com/yusefmosiah/go-choir/internal/searchquery"
)

// Read what the run holds before it searches again (owner, 2026-11-21: "on
// search, we should be reading much more and searching less";
// docs/design/semantic-layer-2026-11-20.md section 4). The rule replaces the
// old line at 15 unread pages, counted per question: a question worded
// differently started at zero, so 44% of a run's later searches went out with
// no page read since the one before, and the runs read 9% of the pages they
// were handed.
//
// A web search is sent only when, for every earlier search of the run, the run
// has tried the first readFirstPages pages new to the run that search returned
// (all of them when it returned fewer). Tried means read or attempted: a dead
// link counts once tried. The pages the paper already held for the run (the
// corpus brief) are search zero and count the same, first. A query that names
// site:<outlet> is not sent while an untried page of that outlet is held. The
// count is run-wide and the same for every verb. It is a correctness refusal
// that names the pages to read, never a count of sends: a run that reads and
// still lacks something sends the search, as many times as it needs.
// choir.SearchFresh(query, why) sends over it with a recorded reason.

// readFirstPages is how many pages of one search the run must have tried
// before it searches again. A search returns up to 40 and the runs of
// 2026-11-18 read about 19 pages a run in all; five is a page a hit of the
// top, not a reading budget. HYPOTHESIS (the design's number); the run line's
// read_first, held_first, site_held and fresh_sent settle it.
const readFirstPages = 5

// freshWhyMin is the shortest reason SearchFresh takes: a sentence, not a flag.
const freshWhyMin = 20

// The fixed tokens a refusal is counted by.
const (
	reasonReadFirst = "read_first"
	reasonHeldFirst = "held_first"
	reasonSiteHeld  = "site_held"
)

// allCounts are a cell's SearchAll counts (SearchStats.AllCalls and the rest).
type allCounts struct {
	calls, offered, sent, freshSent, untagged, running int
}

// HeldPage is a page the paper already held for the run when it began: a
// capture of the corpus brief. The host hands them over with the run's cells
// (SessionFrame.Held); they are search zero.
type HeldPage struct {
	URL       string `json:"url"`
	Title     string `json:"title,omitempty"`
	CaptureID string `json:"capture_id,omitempty"`
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

// seedHeld makes the pages the paper held for runID the run's search zero
// (the brief's captures, in the brief's order, each URL once). It is called
// with every cell of the run and keeps the first set it is given.
func (st *SearchStore) seedHeld(runID string, pages []HeldPage) {
	if st == nil || runID == "" || len(pages) == 0 {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.heldRun == nil {
		st.heldRun = map[string][]SearchHit{}
	}
	if _, ok := st.heldRun[runID]; ok {
		return
	}
	seen := map[string]bool{}
	var hits []SearchHit
	for _, p := range pages {
		key := searchHitKey(p.URL)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		hits = append(hits, SearchHit{Title: p.Title, URL: p.URL, Held: true})
	}
	st.heldRun[runID] = hits
}

// readNeed is what the run holds and has not tried: the first readFirstPages
// pages of search zero and of every earlier search of the run (pages new to the
// run at that search), each page once. held is true when search zero has any,
// and then pages are search zero's only.
type readNeed struct {
	held  bool
	pages []SearchHit
}

func (st *SearchStore) readNeed() readNeed {
	if st == nil {
		return readNeed{}
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	var tried map[string]bool
	if st.triedKeys != nil {
		tried = st.triedKeys()
	}
	read := st.readSet()
	seen := map[string]bool{}
	var zero, rest []SearchHit
	for i, h := range st.heldRun[st.run] {
		if i == readFirstPages {
			break
		}
		if key := searchHitKey(h.URL); !tried[key] && !seen[key] {
			seen[key] = true
			zero = append(zero, h)
		}
	}
	if len(zero) > 0 {
		return readNeed{held: true, pages: zero}
	}
	for i := range st.order {
		if !st.inRun(i) {
			continue
		}
		top := 0
		for _, h := range st.markOneLocked(i, read).Results {
			if h.Held {
				continue
			}
			if top == readFirstPages {
				break
			}
			top++
			if key := searchHitKey(h.URL); !tried[key] && !seen[key] {
				seen[key] = true
				rest = append(rest, h)
			}
		}
	}
	return readNeed{pages: rest}
}

// untriedOfOutlet lists the pages of this run (search zero and every search's
// results) from the outlet that the run has not tried to read.
func (st *SearchStore) untriedOfOutlet(outlet string) []SearchHit {
	if st == nil || outlet == "" {
		return nil
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	var tried map[string]bool
	if st.triedKeys != nil {
		tried = st.triedKeys()
	}
	seen := map[string]bool{}
	var out []SearchHit
	add := func(h SearchHit) {
		key := searchHitKey(h.URL)
		if tried[key] || seen[key] || outletOf(key) != outlet {
			return
		}
		seen[key] = true
		out = append(out, h)
	}
	for _, h := range st.heldRun[st.run] {
		add(h)
	}
	for i := range st.order {
		if !st.inRun(i) {
			continue
		}
		for _, a := range st.searches[st.order[i]].Arrivals {
			for _, h := range a.Results {
				add(h)
			}
		}
	}
	return out
}

// needState is what the run held and had read when it asked, for the host's
// search.decision record: counts only.
type needState struct {
	earlier, returned, tried, read, heldBrief, heldTried, outlets int
}

func (st *SearchStore) needState() needState {
	var n needState
	if st == nil {
		return n
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	var tried map[string]bool
	if st.triedKeys != nil {
		tried = st.triedKeys()
	}
	read := st.readSet()
	outlets := map[string]bool{}
	for key := range read {
		if o := outletOf(key); o != "" {
			outlets[o] = true
		}
	}
	n.read, n.outlets = len(read), len(outlets)
	for _, h := range st.heldRun[st.run] {
		n.heldBrief++
		if tried[searchHitKey(h.URL)] {
			n.heldTried++
		}
	}
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

// readFirstRefusal is what the desk is told when a search is not sent for the
// pages the run holds: why, which pages, and the one way past. The pages
// themselves ride on the stub (SearchStub and Search carry them).
func readFirstRefusal(n readNeed) string {
	if n.held {
		return fmt.Sprintf("the paper already holds %d of the pages for this run's objective (the corpus brief) that you have not tried to read. Read the ones that bear on it (choir.ReadAll on their URLs, or choir.ReadCapture(capture_id)), then search the web for what is still missing, naming it. A page that will not load counts once you have tried it. If these pages cannot bear on what you lack, say why: choir.SearchFresh(query, why) sends the search with a reason of at least %d characters, which is recorded", len(n.pages), freshWhyMin)
	}
	return fmt.Sprintf("the top pages of this run's earlier searches are not read yet: %d of them, listed here. Read them first (choir.ReadAll on their URLs), then search again for what is still missing, naming it: a search that has read what the last one returned asks a sharper question. A page that will not load counts once you have tried it. If these pages cannot bear on what you lack, say why: choir.SearchFresh(query, why) sends the search with a reason of at least %d characters, which is recorded", len(n.pages), freshWhyMin)
}

// siteHeldRefusal is what the desk is told when a site: hunt names an outlet
// the run already holds an untried page of.
func siteHeldRefusal(outlet string, n int) string {
	return fmt.Sprintf("this run already holds %d page(s) of %s that you have not tried to read, listed here. Read them (choir.ReadAll on their URLs): a search restricted to that outlet would return pages you hold. If they cannot bear on what you lack, say why: choir.SearchFresh(query, why) sends the search with a reason of at least %d characters, which is recorded", n, outlet, freshWhyMin)
}

// runningRefusal is what the desk is told when a search is started while another
// of its run is still running: that one's pages are not yet known, so this one
// cannot be judged by them. It returns at once; the cell is never held.
func runningRefusal(id string) string {
	return fmt.Sprintf("a search of this run is still running (%s), so this one cannot be judged by the pages it will return. Wait for it (choir.SearchWait(%q, 15000)), read what it returns, then search again for what is still missing, naming it. To send this search now anyway, say why: choir.SearchFresh(query, why)", id, id)
}
