package yaegikernel

import (
	"fmt"
	"strings"
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

// TreeState is the host's view of a research tree's reading (SessionFrame.Tree):
// Tried is the PageKey of every page any session of the tree has tried to read
// (attempted, whether or not it loaded); Window is the first pages new to the
// tree that each search of the tree returned, in order.
type TreeState struct {
	Tried  []string   `json:"tried,omitempty"`
	Window []HeldPage `json:"window,omitempty"`
}

// PageKey is the key two URLs of one page share (the host's tree state uses it).
func PageKey(raw string) string { return searchHitKey(raw) }

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
		st.heldRun = map[string][]HeldPage{}
	}
	if _, ok := st.heldRun[runID]; ok {
		return
	}
	// A run this store no longer holds searches for is gone: its pages are not
	// kept (the session serves runs in turn; the guest never sees a run end).
	live := map[string]bool{runID: true}
	for _, sr := range st.searches {
		live[sr.run] = true
	}
	for k := range st.heldRun {
		if !live[k] {
			delete(st.heldRun, k)
		}
	}
	seen := map[string]bool{}
	var kept []HeldPage
	for _, p := range pages {
		key := searchHitKey(p.URL)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		kept = append(kept, p)
	}
	st.heldRun[runID] = kept
}

// heldHit is a held page as a search hit: the capture id rides the snippet, so
// the surface of SearchHit does not move.
func heldHit(p HeldPage) SearchHit {
	h := SearchHit{Title: p.Title, URL: p.URL, Held: true}
	if p.CaptureID != "" {
		h.Snippet = "capture_id=" + p.CaptureID
	}
	return h
}

// heldURLForCapture is the URL of the held page of this run with that capture
// id, or "": a ReadCapture of it is an attempt on that page.
func (st *SearchStore) heldURLForCapture(id string) string {
	if st == nil || id == "" {
		return ""
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	for _, p := range st.heldRun[st.run] {
		if p.CaptureID == id {
			return p.URL
		}
	}
	return ""
}

// windowLocked lists the pages the rule looks at, in order: the first
// readFirstPages of search zero, then, for each earlier search of the run, the
// first readFirstPages pages new to the run. zero marks search zero's.
type windowHit struct {
	hit  SearchHit
	zero bool
}

func (st *SearchStore) windowLocked(read map[string]bool) []windowHit {
	var out []windowHit
	for i, p := range st.heldRun[st.run] {
		if i == readFirstPages {
			break
		}
		out = append(out, windowHit{heldHit(p), true})
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
			out = append(out, windowHit{h, false})
		}
	}
	// The pages other sessions of the research tree were returned.
	for _, p := range st.tree.Window {
		out = append(out, windowHit{SearchHit{Title: p.Title, URL: p.URL}, false})
	}
	return out
}

// setTree installs the host's view of the research tree for this cell.
func (st *SearchStore) setTree(t *TreeState) {
	if st == nil {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	st.tree = TreeState{}
	st.treeTried = nil
	if t == nil {
		return
	}
	st.tree = *t
	st.treeTried = make(map[string]bool, len(t.Tried))
	for _, k := range t.Tried {
		st.treeTried[k] = true
	}
}

// triedAll is the pages this session tried and the pages any session of the
// research tree tried. The caller holds st.mu.
func (st *SearchStore) triedAll() map[string]bool {
	tried := map[string]bool{}
	if st.triedKeys != nil {
		for k := range st.triedKeys() {
			tried[k] = true
		}
	}
	for k := range st.treeTried {
		tried[k] = true
	}
	return tried
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
	tried := st.triedAll()
	seen := map[string]bool{}
	var zero, rest []SearchHit
	for _, w := range st.windowLocked(st.readSet()) {
		key := searchHitKey(w.hit.URL)
		if tried[key] || seen[key] {
			continue
		}
		seen[key] = true
		if w.zero {
			zero = append(zero, w.hit)
		} else {
			rest = append(rest, w.hit)
		}
	}
	if len(zero) > 0 {
		return readNeed{held: true, pages: zero}
	}
	return readNeed{pages: rest}
}

// untriedOfOutlet lists the pages in the rule's window (search zero's first
// five and each search's first five new pages) that match the site: term and
// that the run has not tried to read.
func (st *SearchStore) untriedOfOutlet(match func(pageURL string) bool) []SearchHit {
	if st == nil || match == nil {
		return nil
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	tried := st.triedAll()
	seen := map[string]bool{}
	var out []SearchHit
	for _, w := range st.windowLocked(st.readSet()) {
		key := searchHitKey(w.hit.URL)
		if tried[key] || seen[key] || !match(key) {
			continue
		}
		seen[key] = true
		out = append(out, w.hit)
	}
	return out
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

// siteMatcher returns the test a page URL must pass to be "of" the outlet a
// site: term names. A term naming a registrable domain (nytimes.com) matches
// every host of that domain; a term naming a subdomain (sports.nytimes.com)
// matches that host and its subdomains only. An aggregator's page never
// matches.
func siteMatcher(host string) func(pageURL string) bool {
	host = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(host)), "www.")
	registrable := outletOf("https://"+host) == host
	return func(pageURL string) bool {
		ph := hostOf(pageURL)
		if ph == "" || aggregatorHosts[ph] {
			return false
		}
		if registrable {
			return outletOf(pageURL) == host
		}
		return ph == host || strings.HasSuffix(ph, "."+host)
	}
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
	tried := st.triedAll()
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

// refusalListMax is how many pages a refusal lists by name; the rest are in the
// stub's Unread.
const refusalListMax = 10

// pageLines lists pages one to a line, a held page with its capture id.
func pageLines(pages []SearchHit) string {
	var b strings.Builder
	for i, h := range pages {
		if i == refusalListMax {
			fmt.Fprintf(&b, "\n  ... and %d more (they are in the Unread of this value)", len(pages)-i)
			break
		}
		if id := strings.TrimPrefix(h.Snippet, "capture_id="); id != h.Snippet && id != "" {
			fmt.Fprintf(&b, "\n  %s  %s", id, h.URL)
		} else {
			fmt.Fprintf(&b, "\n  %s", h.URL)
		}
	}
	return b.String()
}

// freshException is the last line of every read-first refusal: the exception,
// after the pages to read.
func freshException() string {
	return fmt.Sprintf("\nOnly if none of these pages can bear on what you lack: choir.SearchFresh(query, why), with a reason of at least %d characters, which is recorded.", freshWhyMin)
}

// readFirstRefusal is what the desk is told when a search is not sent for the
// pages the run holds: why, the pages (a held page with its capture id), and
// last the one way past. The pages ride on the stub too (Unread).
func readFirstRefusal(n readNeed) string {
	if n.held {
		return fmt.Sprintf("the paper already holds %d of the pages for this run's objective (the corpus brief) that you have not tried to read. Read them from the corpus, with no web fetch: choir.ReadCapture(capture_id, 0, 0). A page that will not load, or a capture that is gone, counts once you have tried it. Then search the web for what is still missing, naming it.%s%s", len(n.pages), pageLines(n.pages), freshException())
	}
	return fmt.Sprintf("the top pages of this run's earlier searches are not read yet: %d of them. Read them first (choir.ReadAll on their URLs; ReadAll takes at most 24 URLs a call, and a URL past the 24th is not read and stays on this list), then search again for what is still missing, naming it: a search that has read what the last one returned asks a sharper question. A page that will not load counts once you have tried it.%s%s", len(n.pages), pageLines(n.pages), freshException())
}

// siteHeldRefusal is what the desk is told when a site: hunt names an outlet
// the run already holds an untried page of.
func siteHeldRefusal(outlet string, pages []SearchHit) string {
	return fmt.Sprintf("this run already holds %d page(s) of %s among the top of its searches that you have not tried to read. Read them (choir.ReadAll on their URLs, or choir.ReadCapture for a held page): a search restricted to that outlet would return pages you hold.%s%s", len(pages), outlet, pageLines(pages), freshException())
}

// runningRefusal is what the desk is told when a search is started while another
// of its run is still running: that one's pages are not yet known, so this one
// cannot be judged by them. It returns at once; the cell is never held.
func runningRefusal(id string) string {
	return fmt.Sprintf("a search of this run is still running (%s), so this one cannot be judged by the pages it will return. Wait for it (choir.SearchWait(%q, 15000)), read what it returns, then search again for what is still missing, naming it. To send this search now anyway, say why: choir.SearchFresh(query, why)", id, id)
}
