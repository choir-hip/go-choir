package yaegikernel

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// A run reads what it holds before it searches again (owner, 2026-11-21: "on
// search, we should be reading much more and searching less";
// docs/design/semantic-layer-2026-11-20.md section 4). The paper sends about
// 2,700 web searches a day against 1,000 and a research run sends 3.9; 44% of
// the later searches of a run went out with no page read since the one before.
//
// Failure list, written before the code (read from the code before the change):
//  1. A second search goes out although the first search's top five pages new
//     to the run are untried: SearchAll counted unread pages per question key,
//     so a question worded differently started at zero, and the plain verbs
//     tolerated 14 unread pages.
//  2. Its refusal does not list the untried pages, or the run cannot pass it
//     by trying them (a dead link counts once tried), or it needs more than
//     five pages from a search, or fewer than five pages are all it needs when
//     the search returned fewer.
//  3. The count is not run-wide: a page tried for one question does not
//     release a search for another, or an untried page of another question's
//     search does not hold this one.
//  4. The pages the paper already held (the corpus brief) are not search zero:
//     a first web search goes out with those pages untried, or a run with no
//     held pages is held back.
//  5. A site: hunt for an outlet the run holds an untried page from is sent.
//  6. choir.Search(q, true) still sends; SearchFresh accepts a reason under 20
//     characters, or sends without recording it, or is refused when a reason
//     is given.
//  7. The host is told nothing of what the run held and had tried when it
//     asked.
//  8. The refusal is a quota in disguise: a run that reads between searches
//     is stopped after N searches. It must not be: eight searches, each after
//     the reads, are all sent.
// Follow-ups (seat 1, 2026-11-21), failure list written before the code:
// 10. A brief page is shown without its capture id, or only ReadAll(url) clears
//     it (a live web fetch of a page the corpus holds), or ReadCapture(id) of a
//     page whose capture is gone leaves it untried.
// 11. A site: hunt is refused for untried pages of the outlet beyond the top-5
//     window the rest of the rule uses.
// 12. site:sports.nytimes.com is refused for any nytimes.com page; a page of an
//     aggregator host counts as the outlet.
// 13. A Read that hangs until the cell dies leaves the page untried.
// 14. The SearchFresh pointer comes before the pages to read, or reads as the
//     rule instead of the exception.
// 15. A finished run's brief pages stay in the store.
//  9. A refusal is not counted by its reason (read_first, held_first,
//     site_held), or a SearchFresh send is not counted (fresh_sent).

const readEverything = `for _, s := range choir.Searches.List() { for _, h := range s.Results { choir.Read(h.URL) } }
`

func readTop(n int, host string) string {
	return fmt.Sprintf(`for i := 1; i <= %d; i++ { choir.Read(fmt.Sprintf("https://%s.example/p%%d", i)) }
`, n, host)
}

// readTopF is readTop for a source that goes through fmt.Sprintf.
func readTopF(n int, host string) string { return strings.ReplaceAll(readTop(n, host), "%", "%%") }

func TestASecondSearchWaitsForTheFirstSearchsTopFivePages(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"one": pagesN("a", 1, 40), "two": pagesN("b", 1, 40)}
	cell, _ := searchCell(t, f)
	cell(`choir.Search("one")`)
	out := cell(`s, err := choir.Search("two")
fmt.Println(err, s.ID == "", len(s.Results), s.Refused)
for _, h := range s.Results { fmt.Println(h.URL) }`)
	if f.searchCalls.Load() != 1 {
		t.Fatalf("the second search was sent with the first's pages untried:\n%s", out.Stdout)
	}
	for _, want := range []string{"<nil> true 5 ", "https://a.example/p1", "https://a.example/p5", "SearchFresh"} {
		if !strings.Contains(out.Stdout, want) {
			t.Fatalf("refusal lacks %q:\n%s", want, out.Stdout)
		}
	}
	if strings.Contains(out.Stdout, "https://a.example/p6") {
		t.Fatalf("the refusal names more than the top five:\n%s", out.Stdout)
	}
	if st := out.SearchStats; st == nil || st.NotSent["read_first"] != 1 || st.Started != 0 {
		t.Fatalf("stats = %+v, want one read_first", st)
	}
	// Four of five tried is not enough; the fifth releases it.
	cell(readTop(4, "a"))
	if cell(`s, _ := choir.Search("two"); fmt.Println(s.ID == "")`); f.searchCalls.Load() != 1 {
		t.Fatalf("four of five pages tried must not release the search")
	}
	cell(`choir.Read("https://a.example/p5")`)
	out = cell(`s, err := choir.Search("two")
fmt.Println(err, s.ID, s.Refused == "")`)
	if f.searchCalls.Load() != 2 || !strings.Contains(out.Stdout, "<nil> s2 true") {
		t.Fatalf("the five pages are tried and the search was not sent:\n%s", out.Stdout)
	}
}

func TestADeadLinkCountsOnceTriedAndAShortSearchNeedsOnlyItsPages(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"one": pagesN("a", 1, 3), "two": pagesN("b", 1, 10)}
	f.failing["https://a.example/p2"] = "HTTP 403"
	cell, _ := searchCell(t, f)
	cell(`choir.Search("one")`)
	cell(`choir.Read("https://a.example/p1")
choir.Read("https://a.example/p2")`)
	cell(`choir.Search("two")`)
	if f.searchCalls.Load() != 1 {
		t.Fatalf("one of three pages untried: the search must wait")
	}
	cell(`choir.Read("https://a.example/p3")`)
	cell(`choir.Search("two")`)
	if f.searchCalls.Load() != 2 {
		t.Fatalf("three pages tried (one dead) and the search returned only three: it must be sent")
	}
}

func TestTheCountIsRunWideForEveryVerb(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"w1": pagesN("a", 1, 40), "w2": pagesN("b", 1, 40), "plain": pagesN("c", 1, 40)}
	cell, _ := searchCell(t, f)
	cell(`choir.SearchAll("the harbor strike", []string{"w1"})`)
	// A different question, worded differently: the old per-question count started at zero.
	out := cell(`r, err := choir.SearchAll("the cabinet changes", []string{"w2"})
fmt.Println(err, len(r.Wordings), r.Wordings[0].Why)
s, _ := choir.SearchStart("plain")
fmt.Println(s.Refused != "")`)
	if f.searchCalls.Load() != 1 {
		t.Fatalf("a second question was sent with the first's pages untried:\n%s", out.Stdout)
	}
	if !strings.Contains(out.Stdout, "true\n") {
		t.Fatalf("SearchStart was not refused:\n%s", out.Stdout)
	}
	if st := out.SearchStats; st == nil || st.NotSent["read_first"] < 1 {
		t.Fatalf("stats = %+v", st)
	}
	// Trying them releases every verb.
	cell(readTop(5, "a"))
	cell(`choir.SearchAll("the cabinet changes", []string{"w2"})`)
	if f.searchCalls.Load() != 2 {
		t.Fatalf("pages tried but SearchAll was not sent")
	}
}

func TestThePagesThePaperHeldAreSearchZero(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"qq": pagesN("n", 1, 10)}
	cell, hooks := searchCell(t, f)
	var held []HeldPage
	for i := 1; i <= 8; i++ {
		held = append(held, HeldPage{URL: fmt.Sprintf("https://h.example/p%d", i), CaptureID: fmt.Sprintf("cap-h-%d", i)})
	}
	hooks.Begin(SessionFrame{RunID: "run-1", Held: held})
	out := cell(`s, _ := choir.Search("qq")
fmt.Println(s.ID == "", len(s.Results), s.Refused)
for _, h := range s.Results { fmt.Println(h.URL) }`)
	if f.searchCalls.Load() != 0 {
		t.Fatalf("a first search was sent with the held pages untried:\n%s", out.Stdout)
	}
	for _, want := range []string{"true 5 ", "https://h.example/p1", "https://h.example/p5"} {
		if !strings.Contains(out.Stdout, want) {
			t.Fatalf("refusal lacks %q:\n%s", want, out.Stdout)
		}
	}
	if st := out.SearchStats; st == nil || st.NotSent["held_first"] != 1 {
		t.Fatalf("stats = %+v, want one held_first", st)
	}
	// A held page is tried by ReadCapture of its capture as much as by Read of its url.
	cell(`for i := 1; i <= 5; i++ { choir.ReadCapture(fmt.Sprintf("cap:https://h.example/p%d", i), 0, 0) }`)
	cell(`choir.Search("qq")`)
	if f.searchCalls.Load() != 1 {
		t.Fatalf("the held pages are tried and the first search was not sent")
	}
	// A run holding nothing is never held back.
	g := newResearchFixture(t)
	gc, _ := searchCell(t, g)
	gc(`choir.Search("qq")`)
	if g.searchCalls.Load() != 1 {
		t.Fatalf("a run with no held pages was held back")
	}
}

func TestASiteHuntForAnOutletTheRunHoldsIsRefused(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"one": append(pagesN("a", 1, 3), "https://news.other.example/story-1"), "site:other.example one": pagesN("z", 1, 3), "site:nowhere.example one": pagesN("y", 1, 3)}
	cell, _ := searchCell(t, f)
	cell(`choir.Search("one")`)
	cell(`choir.Read("https://a.example/p1")
choir.Read("https://a.example/p2")
choir.Read("https://a.example/p3")`)
	// The first search returned a page of other.example the run has not tried:
	// a hunt for that outlet is answered by reading it.
	out := cell(`s, _ := choir.Search("site:other.example one")
fmt.Println(s.ID == "", len(s.Results))
for _, h := range s.Results { fmt.Println(h.URL) }`)
	if f.searchCalls.Load() != 1 {
		t.Fatalf("a site: hunt for a held outlet was sent:\n%s", out.Stdout)
	}
	if !strings.Contains(out.Stdout, "https://news.other.example/story-1") {
		t.Fatalf("the refusal does not list the outlet's untried page:\n%s", out.Stdout)
	}
	if st := out.SearchStats; st == nil || st.NotSent["site_held"] != 1 {
		t.Fatalf("stats = %+v, want one site_held", st)
	}
	// An outlet the run holds nothing from is hunted, once the page is read.
	cell(`choir.Read("https://news.other.example/story-1")`)
	cell(`choir.Search("site:nowhere.example one")`)
	if f.searchCalls.Load() != 2 {
		t.Fatalf("a site: hunt for an outlet the run does not hold was refused")
	}
}

func TestSearchFreshNeedsAReasonAndIsTheOneWayPast(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"one": pagesN("a", 1, 40), "two": pagesN("b", 1, 40), "three": pagesN("c", 1, 40)}
	cell, _ := searchCell(t, f)
	cell(`choir.Search("one")`)
	out := cell(`s, err := choir.SearchFresh("two", "too short")
fmt.Println(err, s.ID == "", s.Refused)`)
	if f.searchCalls.Load() != 1 || !strings.Contains(out.Stdout, "20") {
		t.Fatalf("a reason under 20 characters sent or was not explained:\n%s", out.Stdout)
	}
	out = cell(`s, err := choir.Search("two", true)
fmt.Println(err, s.ID == "", s.Refused)`)
	if f.searchCalls.Load() != 1 || !strings.Contains(out.Stdout, "SearchFresh") {
		t.Fatalf("Search(q, true) must be refused with the pointer to SearchFresh:\n%s", out.Stdout)
	}
	out = cell(`s, err := choir.SearchStart("two", true)
fmt.Println(err, s.ID == "", s.Refused)`)
	if f.searchCalls.Load() != 1 || !strings.Contains(out.Stdout, "SearchFresh") {
		t.Fatalf("SearchStart(q, true) must be refused with the pointer to SearchFresh:\n%s", out.Stdout)
	}
	const why = "the first search's pages are about the wrong court case"
	out = cell(fmt.Sprintf(`s, err := choir.SearchFresh("two", %q)
fmt.Println(err, s.ID, s.Refused == "")`, why))
	if f.searchCalls.Load() != 2 || !strings.Contains(out.Stdout, "<nil> s2 true") {
		t.Fatalf("SearchFresh with a reason must send:\n%s", out.Stdout)
	}
	if st := out.SearchStats; st == nil || st.FreshSent != 1 || st.Started != 1 {
		t.Fatalf("stats = %+v, want fresh_sent 1", st)
	}
	f.searchMu.Lock()
	last := f.payloads[len(f.payloads)-1]
	f.searchMu.Unlock()
	if last.FreshWhy != why || last.Verb != "search_fresh" {
		t.Fatalf("the host was not told the reason: %+v", last)
	}
}

func TestTheHostIsToldWhatTheRunHeldAndHadTried(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"one": pagesN("a", 1, 12), "two": pagesN("b", 1, 12)}
	cell, hooks := searchCell(t, f)
	hooks.Begin(SessionFrame{RunID: "run-1", Held: []HeldPage{{URL: "https://h.example/p1"}, {URL: "https://h.example/p2"}}})
	cell(`choir.Read("https://h.example/p1")
choir.Read("https://h.example/p2")
choir.Search("one")`)
	cell(readTop(7, "a"))
	cell(`choir.SearchAll("q", []string{"two"})`)
	f.searchMu.Lock()
	defer f.searchMu.Unlock()
	if len(f.payloads) != 2 {
		t.Fatalf("payloads = %d", len(f.payloads))
	}
	p1, p2 := f.payloads[0], f.payloads[1]
	if p1.Verb != "search" || p1.Earlier != 0 || p1.HeldBrief != 2 || p1.HeldTried != 2 {
		t.Fatalf("first payload %+v", p1)
	}
	if p2.Verb != "search_all" || p2.Earlier != 1 || p2.Returned != 12 || p2.Tried != 7 || p2.Read != 9 || p2.HeldBrief != 2 || p2.HeldTried != 2 {
		t.Fatalf("second payload %+v", p2)
	}
	if p2.Outlets < 1 {
		t.Fatalf("outlets read not reported: %+v", p2)
	}
}

func TestAnyNumberOfSearchesIsSentWhenTheRunReadsBetween(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{}
	cell, _ := searchCell(t, f)
	for i := 1; i <= 9; i++ {
		host := fmt.Sprintf("h%d", i)
		q := fmt.Sprintf("question %d", i)
		f.urlsFor[q] = pagesN(host, 1, 20)
		cell(fmt.Sprintf(`choir.Search(%q)`, q))
		cell(readTop(5, host))
	}
	if f.searchCalls.Load() != 9 {
		t.Fatalf("a run that reads between searches was stopped at %d of 9: the refusal must not be a quota", f.searchCalls.Load())
	}
}

func heldPages(n int) []HeldPage {
	var held []HeldPage
	for i := 1; i <= n; i++ {
		held = append(held, HeldPage{URL: fmt.Sprintf("https://h.example/p%d", i), CaptureID: fmt.Sprintf("cap-h-%d", i)})
	}
	return held
}

func TestABriefPageIsShownWithItsCaptureIDAndClearedByReadCaptureWithNoWebFetch(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"qq": pagesN("n", 1, 10)}
	f.captureErr = map[string]string{"cap-h-3": "capture not found"}
	cell, hooks := searchCell(t, f)
	hooks.Begin(SessionFrame{RunID: "run-1", Held: heldPages(8)})
	out := cell(`s, _ := choir.Search("qq")
fmt.Println(s.Refused)`)
	for i := 1; i <= 5; i++ {
		if want := fmt.Sprintf("cap-h-%d  https://h.example/p%d", i, i); !strings.Contains(out.Stdout, want) {
			t.Fatalf("the refusal does not list %q:\n%s", want, out.Stdout)
		}
	}
	// The pointer is the last line, after the pages, and reads as the exception.
	at, last := strings.Index(out.Stdout, "Only if none of these pages can bear on what you lack"), strings.LastIndex(out.Stdout, "https://h.example/p5")
	if at < 0 || at < last || !strings.Contains(out.Stdout[at:], "choir.SearchFresh(query, why)") {
		t.Fatalf("SearchFresh must be the exception after the pages:\n%s", out.Stdout)
	}
	if strings.Contains(out.Stdout[:last], "SearchFresh") {
		t.Fatalf("SearchFresh named before the list of pages:\n%s", out.Stdout)
	}
	// ReadCapture by id clears a page with no web fetch; a capture that is gone
	// is released by the attempt.
	cell(`for i := 1; i <= 5; i++ { choir.ReadCapture(fmt.Sprintf("cap-h-%d", i), 0, 0) }`)
	if f.reads.Load() != 0 || f.captureReads.Load() != 5 {
		t.Fatalf("web fetches %d, capture reads %d: want 0 and 5", f.reads.Load(), f.captureReads.Load())
	}
	cell(`choir.Search("qq")`)
	if f.searchCalls.Load() != 1 {
		t.Fatalf("the five held pages were attempted (one capture gone) and the search was not sent")
	}
}

func TestASiteHuntLooksOnlyAtTheTopFivePagesOfASearch(t *testing.T) {
	f := newResearchFixture(t)
	busy := []string{"https://a.example/1", "https://a.example/2", "https://a.example/3"}
	for i := 1; i <= 8; i++ {
		busy = append(busy, fmt.Sprintf("https://busy.example/s%d", i))
	}
	f.urlsFor = map[string][]string{"one": busy, "site:busy.example one": pagesN("z", 1, 3)}
	cell, _ := searchCell(t, f)
	cell(`choir.Search("one")`)
	out := cell(`s, _ := choir.Search("site:busy.example one")
fmt.Println(len(s.Results), s.Refused != "")`)
	if f.searchCalls.Load() != 1 || !strings.Contains(out.Stdout, "2 true") {
		t.Fatalf("the hunt must list the 2 busy.example pages of the top five, not all 8:\n%s", out.Stdout)
	}
	// The top five tried: the other six busy.example pages do not hold the hunt.
	cell(`choir.Read("https://a.example/1"); choir.Read("https://a.example/2"); choir.Read("https://a.example/3")
choir.Read("https://busy.example/s1"); choir.Read("https://busy.example/s2")`)
	cell(`choir.Search("site:busy.example one")`)
	if f.searchCalls.Load() != 2 {
		t.Fatalf("pages of the outlet beyond the top five held the hunt")
	}
}

func TestSiteTermsMatchTheHostTheyNameAndNeverAnAggregator(t *testing.T) {
	cases := []struct {
		site, page string
		want       bool
	}{
		{"nytimes.com", "https://www.nytimes.com/a", true},
		{"nytimes.com", "https://sports.nytimes.com/a", true},
		{"sports.nytimes.com", "https://sports.nytimes.com/a", true},
		{"sports.nytimes.com", "https://www.nytimes.com/a", false},
		{"sports.nytimes.com", "https://news.nytimes.com/a", false},
		{"sports.nytimes.com", "https://x.sports.nytimes.com/a", true},
		{"google.com", "https://news.google.com/rss/a", false},
		{"msn.com", "https://www.msn.com/en-us/news/a", false},
		{"news.google.com", "https://news.google.com/rss/a", false},
		{"bbc.co.uk", "https://www.bbc.co.uk/news/a", true},
	}
	for _, c := range cases {
		if got := siteMatcher(c.site)(c.page); got != c.want {
			t.Errorf("site:%s against %s = %v, want %v", c.site, c.page, got, c.want)
		}
	}
}

func TestAReadThatHangsIsAlreadyAnAttempt(t *testing.T) {
	f := newResearchFixture(t)
	f.scope.BindCell().Begin(SessionFrame{RunID: "run-1"})
	const u = "https://hang.example/p1"
	f.hang[u] = true
	go func() { _, _ = f.scope.Read(u) }()
	deadline := time.Now().Add(1500 * time.Millisecond)
	for !f.scope.triedKeys()[searchHitKey(u)] {
		if time.Now().After(deadline) {
			t.Fatal("a Read in flight is not yet an attempt: a read that hangs until the cell dies leaves the page untried")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestAFinishedRunsBriefPagesAreNotKept(t *testing.T) {
	f := newResearchFixture(t)
	f.scope.BindCell().Begin(SessionFrame{RunID: "run-1", Held: heldPages(3)})
	f.scope.BindCell().Begin(SessionFrame{RunID: "run-2", Held: heldPages(2)})
	st := f.scope.searches
	st.mu.Lock()
	defer st.mu.Unlock()
	if _, ok := st.heldRun["run-1"]; ok || len(st.heldRun["run-2"]) != 2 {
		t.Fatalf("heldRun = %v: run-1's pages must be dropped when run-2 begins", st.heldRun)
	}
}

// The research tree shares what has been TRIED, not what has been returned (the lead,
// 2026-11-21, after seat 1's read of the per-tree window): each choir.Research child
// is a session of its own with its own store, and its window is its own searches'
// pages. A sibling's search never holds it; a page anyone in the tree tried is never
// demanded of anyone again. Five first searches, one per session, are correct: the
// saving is no second search without reading, per branch.
func TestParentPlusFourChildrenWithNothingReadEachMaySendItsFirstSearchAndNoSecond(t *testing.T) {
	for c := 0; c < 5; c++ {
		f := newResearchFixture(t)
		f.urlsFor = map[string][]string{"first": pagesN(fmt.Sprintf("s%d", c), 1, 10)}
		cc, hooks := f.session(t)
		hooks.Begin(SessionFrame{RunID: "run-1", Tree: &TreeState{}})
		cc(`choir.Search("first")`)
		if f.searchCalls.Load() != 1 {
			t.Fatalf("session %d was held on its first search by the others' pages", c)
		}
		cc(`choir.Search("second")`)
		if f.searchCalls.Load() != 1 {
			t.Fatalf("session %d sent a second search with its own first five untried", c)
		}
	}
}

func TestEachOfTwoSessionsWithDifferentObjectivesSendsItsFirstSearch(t *testing.T) {
	a, b := newResearchFixture(t), newResearchFixture(t)
	a.urlsFor = map[string][]string{"harbor strike": pagesN("a", 1, 10)}
	b.urlsFor = map[string][]string{"cabinet changes": pagesN("b", 1, 10)}
	ca, ha := a.session(t)
	cb, hb := b.session(t)
	ha.Begin(SessionFrame{RunID: "run-1"})
	hb.Begin(SessionFrame{RunID: "run-1", Tree: treeOf()})
	ca(`choir.Search("harbor strike")`)
	cb(`choir.Search("cabinet changes")`)
	if a.searchCalls.Load() != 1 || b.searchCalls.Load() != 1 {
		t.Fatalf("both children must send: %d and %d", a.searchCalls.Load(), b.searchCalls.Load())
	}
}

func TestAPageASiblingTriedIsNotDemandedAndAPageItReturnedIsInThisSessionsOwnWindow(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"one": pagesN("a", 1, 10)}
	var urls []string
	for i := 1; i <= 4; i++ {
		urls = append(urls, fmt.Sprintf("https://a.example/p%d", i))
	}
	cc, hooks := f.session(t)
	// A sibling already tried four of the five pages this session's own search will return.
	hooks.Begin(SessionFrame{RunID: "run-1", Tree: treeOf(urls...)})
	cc(`choir.Search("one")`)
	cc(`choir.Search("two")`)
	if f.searchCalls.Load() != 1 {
		t.Fatalf("the fifth page, returned by this session's own search and tried by no one, must hold it")
	}
	cc(`choir.Read("https://a.example/p5")`)
	cc(`choir.Search("two")`)
	if f.searchCalls.Load() != 2 {
		t.Fatalf("four pages tried by a sibling and one by this session: the search must be sent")
	}
}

func TestOnePageWhateverTheSchemeOrWWW(t *testing.T) {
	if a, b := searchHitKey("http://www.A.example/x/#f"), searchHitKey("https://a.example/x"); a != b {
		t.Fatalf("keys differ: %q %q", a, b)
	}
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"one": {"https://www.a.example/1", "http://a.example/2"}, "two": pagesN("b", 1, 2)}
	cell, _ := searchCell(t, f)
	cell(`choir.Search("one")`)
	cell(`choir.Read("https://a.example/1"); choir.Read("https://www.a.example/2")`)
	cell(`choir.Search("two")`)
	if f.searchCalls.Load() != 2 {
		t.Fatalf("a page read under another scheme or www form must count as tried")
	}
}

// Follow-ups (seat 1, 2026-11-21, second read), failure list written before the code:
// 16. A sibling's read is not seen within a cell: the tree snapshot is taken when
//     the cell begins, so four children fanned out after one search are each
//     refused and each read the same five pages. The guest asks the host for the
//     live tree before it refuses.
// 17. The worked cell, with brief pages held, is refused by SearchAll and then
//     web-reads the brief pages instead of reading their captures.
// 18. Two registrable domains under one public suffix (bbc.co.uk, theguardian.co.uk,
//     and the .com.au / .co.jp forms) count as one outlet, or a site: hunt for
//     one is refused for another's untried page.

func treeOf(tried ...string) *TreeState {
	t := &TreeState{}
	for _, u := range tried {
		t.Tried = append(t.Tried, PageKey(u))
	}
	return t
}

func ownFirstFive(t *testing.T, tree *TreeState) (*researchFixture, func(string) SessionResult, []string) {
	t.Helper()
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"child question": pagesN("a", 1, 10)}
	cc, hooks := f.session(t)
	hooks.Begin(SessionFrame{RunID: "run-1", Tree: tree})
	cc(`choir.Search("child question")`)
	var urls []string
	for i := 1; i <= 5; i++ {
		urls = append(urls, fmt.Sprintf("https://a.example/p%d", i))
	}
	return f, cc, urls
}

func TestAPageASiblingTriedAfterTheCellBeganIsNotDemandedAgain(t *testing.T) {
	f, cc, urls := ownFirstFive(t, treeOf())
	// A sibling reads the five pages while this cell is running: the host's live
	// tree has them tried, the snapshot this cell began with does not.
	f.liveTree = treeOf(urls...)
	out := cc(`s, err := choir.Search("second question")
fmt.Println(err, s.Refused == "", s.ID)`)
	if f.searchCalls.Load() != 2 || !strings.Contains(out.Stdout, "<nil> true s2") {
		t.Fatalf("the sibling's reads were not seen at the gate (sent %d):\n%s", f.searchCalls.Load(), out.Stdout)
	}
	if f.probes.Load() != 1 {
		t.Fatalf("%d probes, want 1", f.probes.Load())
	}
}

func TestTheRefusalCarriesTheCurrentUntriedPagesAfterTheHostWasAsked(t *testing.T) {
	f, cc, urls := ownFirstFive(t, treeOf())
	// The sibling read two of the five: the refusal names the other three.
	f.liveTree = treeOf(urls[0], urls[1])
	out := cc(`s, _ := choir.Search("second question")
fmt.Println(s.ID == "", len(s.Results))
for _, h := range s.Results { fmt.Println(h.URL) }`)
	if f.searchCalls.Load() != 1 || !strings.Contains(out.Stdout, "true 3") || strings.Contains(out.Stdout, "a.example/p1\n") || !strings.Contains(out.Stdout, "a.example/p5") {
		t.Fatalf("want a refusal naming the three untried pages (sent %d):\n%s", f.searchCalls.Load(), out.Stdout)
	}
}

func TestACellWithNoRunAsksTheHostNothing(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"one": pagesN("a", 1, 10)}
	cell, hooks := f.session(t)
	hooks.Begin(SessionFrame{})
	cell(`choir.Search("one")`)
	cell(`choir.Search("two")`)
	if f.probes.Load() != 0 {
		t.Fatalf("a cell with no run probed the host")
	}
}

func TestChoirOutletIsTheFunctionTheGateUses(t *testing.T) {
	f := newResearchFixture(t)
	cell, _ := searchCell(t, f)
	out := cell(`fmt.Println(choir.Outlet("https://www.bbc.co.uk/news/x"), choir.Outlet("https://www.theguardian.co.uk/a"), choir.Outlet("https://x.example.com.au/p"))`)
	if !strings.Contains(out.Stdout, "bbc.co.uk theguardian.co.uk example.com.au") {
		t.Fatalf("choir.Outlet: %q", out.Stdout)
	}
}

func TestAHuntForOneCoUKOutletIsNotRefusedForAnothersUntriedPage(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"one": {"https://www.bbc.co.uk/1", "https://www.bbc.co.uk/2"}, "site:theguardian.co.uk one": pagesN("z", 1, 2)}
	cell, _ := searchCell(t, f)
	cell(`choir.Search("one")`)
	// With the bbc pages untried a hunt for bbc.co.uk is
	// refused, a hunt for theguardian.co.uk is not refused by site_held (the
	// read-first rule still holds it, for the bbc pages are untried).
	out := cell(`choir.Search("site:theguardian.co.uk one")`)
	if st := out.SearchStats; st == nil || st.NotSent[reasonSiteHeld] != 0 {
		t.Fatalf("a hunt for theguardian.co.uk was held for a bbc.co.uk page: %+v", st)
	}
	out = cell(`choir.Search("site:bbc.co.uk one")`)
	if st := out.SearchStats; st == nil || st.NotSent[reasonSiteHeld] != 1 {
		t.Fatalf("a hunt for bbc.co.uk was not held for its untried pages: %+v", st)
	}
}

func TestTheWorkedCellReadsHeldBriefPagesByCaptureID(t *testing.T) {
	f := newResearchFixture(t)
	cell, hooks := f.session(t)
	hooks.Begin(SessionFrame{RunID: "run-1", Held: heldPages(8)})
	out := cell(workedCell(t))
	if f.reads.Load() != 0 || f.captureReads.Load() != 5 || f.searchCalls.Load() != 0 {
		t.Fatalf("web fetches %d, capture reads %d, searches %d: want 0, 5 and 0:\n%s", f.reads.Load(), f.captureReads.Load(), f.searchCalls.Load(), out.Stdout)
	}
	if !strings.Contains(out.Stdout, "interim packet: 5 sources") || strings.Contains(out.Stdout, "<error") {
		t.Fatalf("the worked cell did not report from the held pages:\n%s", out.Stdout)
	}
}

// Seat 2's second read (2026-11-21), failure list written before the code:
// 19. The worked cell counts bbc.co.uk and itv.co.uk as one outlet, and
//     news.google.com and msn.com as two.
// 20. The worked cell reads the first five merged results, not the first five
//     new pages: a Held page in front takes a slot.
// 21. A run that searched keeps its brief pages after another run begins.
// 22. SearchAll names the first wording's outlet to a later wording of another.

func workedCellStdout(t *testing.T, urls []string) string {
	t.Helper()
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"ministry cabinet changes": urls}
	cell, hooks := f.session(t)
	hooks.Begin(SessionFrame{RunID: "run-1"})
	return cell(workedCell(t)).Stdout
}

func TestTheWorkedCellCountsOutletsByTheKernelsRule(t *testing.T) {
	if out := workedCellStdout(t, []string{"https://www.bbc.co.uk/a", "https://www.itv.co.uk/b"}); !strings.Contains(out, "outlets read: 2 ") {
		t.Fatalf("bbc.co.uk and itv.co.uk are two outlets:\n%s", out)
	}
	if out := workedCellStdout(t, []string{"https://news.google.com/a", "https://www.msn.com/b"}); !strings.Contains(out, "outlets read: 0 ") {
		t.Fatalf("two aggregators are no outlet at all:\n%s", out)
	}
	if out := workedCellStdout(t, []string{"https://www.bbc.co.uk/a", "https://news.bbc.co.uk/b"}); !strings.Contains(out, "outlets read: 1 ") {
		t.Fatalf("www.bbc.co.uk and news.bbc.co.uk are one outlet:\n%s", out)
	}
	f := newResearchFixture(t)
	cell, _ := searchCell(t, f)
	out := cell(`fmt.Printf("%q %q\n", choir.Outlet("https://news.google.com/x"), choir.Outlet("https://www.msn.com/y"))`)
	if !strings.Contains(out.Stdout, `"" ""`) {
		t.Fatalf("an aggregator's page is no outlet: %q", out.Stdout)
	}
}

func TestTheWorkedCellReadsTheFirstFiveNewPagesNotTheFirstFiveMerged(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"ministry cabinet changes": pagesN("a", 1, 3)}
	cell, hooks := f.session(t)
	hooks.Begin(SessionFrame{RunID: "run-1"})
	// The same question was searched before, and its pages read: the worked cell's
	// SearchAll is answered from that search (its pages come back Held, in front).
	cell(`choir.Search("ministry cabinet changes")
for _, h := range choir.Searches.List()[0].Results { choir.Read(h.URL) }`)
	before := f.reads.Load()
	out := cell(workedCell(t))
	if got := f.reads.Load(); got != before {
		t.Fatalf("the cell read %d held page(s) again:\n%s", got-before, out.Stdout)
	}
	if strings.Contains(out.Stdout, "interim packet") || strings.Contains(out.Stdout, "<error") {
		t.Fatalf("no new page was read, so no packet:\n%s", out.Stdout)
	}
}

func TestABriefPageOfARunThatSearchedIsDroppedWhenAnotherRunBegins(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"qq": pagesN("n", 1, 3)}
	cell, hooks := f.session(t)
	hooks.Begin(SessionFrame{RunID: "run-1", Held: heldPages(3)})
	cell(`choir.Search("qq")`)
	f.scope.BindCell().Begin(SessionFrame{RunID: "run-2"})
	st := f.scope.searches
	st.mu.Lock()
	defer st.mu.Unlock()
	if _, ok := st.heldRun["run-1"]; ok {
		t.Fatalf("heldRun = %v: run-1 searched, and its brief pages are kept after run-2 began", st.heldRun)
	}
}

func TestSearchAllNamesTheRunsUntriedPagesToALaterWordingNotTheFirstOnesOutlet(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"one": {"https://bbc.example/1", "https://bbc.example/2"}}
	cell, _ := searchCell(t, f)
	cell(`choir.Search("one")`)
	out := cell(`r, _ := choir.SearchAll("hunt", []string{"site:bbc.example q", "site:reuters.example q"})
for _, w := range r.Wordings { fmt.Println("W:", w.Why) }`)
	lines := []string{}
	for _, l := range strings.Split(out.Stdout, "\n") {
		if strings.HasPrefix(l, "W:") {
			lines = append(lines, l)
		}
	}
	if len(lines) != 2 || !strings.Contains(lines[0], "bbc.example") {
		t.Fatalf("wordings: %v", lines)
	}
	if strings.Contains(lines[1], "page(s) of bbc.example") || !strings.Contains(lines[1], "the top pages of this run's earlier searches are not read yet") {
		t.Fatalf("the second wording is told the first's outlet, not the pages the run holds: %s", lines[1])
	}
}

// A Held page (returned by an earlier search, no capture id) in front of five new
// pages takes no slot: the five new pages are read and the next search is sent.
func TestAHeldPageInFrontOfFiveNewPagesTakesNoSlotInTheWorkedCell(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{
		"earlier":                  {"https://old.example/x"},
		"ministry cabinet changes": {"https://old.example/x", "https://n1.example/1", "https://n2.example/2", "https://n3.example/3", "https://n4.example/4", "https://n5.example/5", "https://n6.example/6"},
	}
	cell, hooks := f.session(t)
	hooks.Begin(SessionFrame{RunID: "run-1"})
	cell(`choir.Search("earlier")
choir.Read("https://old.example/x")`)
	before := f.reads.Load()
	out := cell(workedCell(t))
	if got := f.reads.Load() - before; got != 5 {
		t.Fatalf("the cell read %d pages, want the 5 new ones:\n%s", got, out.Stdout)
	}
	if strings.Contains(out.Stdout, "https://old.example/x capture") {
		t.Fatalf("the held page was read again:\n%s", out.Stdout)
	}
	cell(`choir.Search("another question")`)
	if f.searchCalls.Load() != 3 {
		t.Fatalf("the five new pages are tried: the next search must be sent (searches %d)", f.searchCalls.Load())
	}
}

// The capture id in a refusal is SearchHit.CaptureID, never text a provider put in a
// snippet.
func TestARefusalPrintsTheCaptureIDFieldNotASnippetThatLooksLikeOne(t *testing.T) {
	pages := []SearchHit{
		{URL: "https://p.example/1", Snippet: "capture_id=evil-id"},
		{URL: "https://h.example/2", CaptureID: "cap-real", Held: true},
	}
	got := pageLines(pages)
	if strings.Contains(got, "evil-id") || !strings.Contains(got, "cap-real  https://h.example/2") || !strings.Contains(got, "\n  https://p.example/1") {
		t.Fatalf("pageLines = %q", got)
	}
}

// An aggregator's page is no outlet in any count: the stop rule's, the decision's
// and choir.Outlet's are one function.
func TestAggregatorPagesAreNoOutletInTheRunsCounts(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"qq": {"https://news.google.com/a", "https://www.msn.com/b", "https://www.bbc.co.uk/c", "https://news.bbc.co.uk/d"}, "two": pagesN("z", 1, 2)}
	cell, _ := searchCell(t, f)
	cell(`choir.Search("qq")`)
	cell(`for _, h := range choir.Searches.List()[0].Results { choir.Read(h.URL) }`)
	out := cell(`choir.SearchStart("two")`)
	if st := out.SearchStats; st == nil || st.OutletsLast != 1 {
		t.Fatalf("outlets held at the send: %+v, want 1 (bbc.co.uk; the two aggregators are none)", st)
	}
	if ns := f.scope.searches.needState(); ns.outlets != 1 {
		t.Fatalf("the decision counts %d outlets read, want 1", ns.outlets)
	}
}
