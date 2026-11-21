package yaegikernel

import (
	"fmt"
	"strings"
	"testing"
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
