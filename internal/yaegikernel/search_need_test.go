package yaegikernel

import (
	"fmt"
	"strings"
	"testing"
)

// Search for what is lacking, not for what is new (docs/problems/
// search-volume-outruns-keenable-credits-2026-11-18.md, "Why a run searches
// again"). Measured after search reuse deployed: 83% of returned pages were new
// to the run, so "stop when wordings stop finding new pages" almost never
// stopped, and runs read under 5% of the pages they were handed. Failure list,
// written before the code:
// 2026-11-21: the 15-unread line, counted per question, became the run-wide
// read-first rule (search_need.go; search_read_first_test.go). What these tests
// kept: a SearchAll call sends the first wording and names the rest; a search
// that returned little needs only its own pages tried; every way of trying a
// page (a failed Read, ReadCapture, another form of the URL) releases it.
//  1. SearchAll sends every wording of a question although the first already
//     returned 40 pages new to the run that the run has not read.
//  2. A first wording that returns 3 pages stops the question (it must not:
//     the question still lacks pages).
//  3. A later SearchAll for the same question is refused or ignored after the
//     run has read the pages, or sends nothing while pages remain unread
//     (the question key is its terms, so a reworded question is the same one).
//  4. The wordings not sent are not reported with the reason, the count of
//     unread pages and what to do next; or not counted under a reason of
//     their own in the run's counts.
//  5. fresh stops sending a held or withheld search.
//  6. (Superseded 2026-11-19: a plain search of such a run is refused, not
//     noted; see search_gate_test.go.)
//  7. The run counts do not say how many SearchAll calls were made, how many
//     wordings they were offered and how many they sent.

func TestSearchAllSendsOneWordingAndNamesTheRest(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"w1 alpha": pagesN("a", 1, 40), "w2 beta": pagesN("b", 1, 40), "w3 gamma": pagesN("c", 1, 40), "w4 delta": pagesN("d", 1, 40), "w5 omega": pagesN("e", 1, 40)}
	cell, _ := searchCell(t, f)
	out := cell(`r, err := choir.SearchAll("the harbor strike", []string{"w1 alpha", "w2 beta", "w3 gamma", "w4 delta", "w5 omega"})
fmt.Println(err)
fmt.Println(r)`)
	if f.searchCalls.Load() != 1 {
		t.Fatalf("host asked to start %d searches, want 1:\n%s", f.searchCalls.Load(), out.Stdout)
	}
	for _, want := range []string{
		`1 "w1 alpha": sent s1 (10 new in first 10, 40 of 40 pages new)`,
		`2 "w2 beta": not sent (the top pages of this run's earlier searches are not read yet`,
		`5 "w5 omega": not sent (the top pages`,
		"then call SearchAll again for what is still missing, naming it",
	} {
		if !strings.Contains(out.Stdout, want) {
			t.Fatalf("stdout lacks %q:\n%s", want, out.Stdout)
		}
	}
	// One count for the refusal, however many wordings it covered.
	st := out.SearchStats
	if st == nil || st.Started != 1 || st.NotSent["read_first"] != 1 || st.AllCalls != 1 || st.AllOffered != 5 || st.AllSent != 1 {
		t.Fatalf("stats = %+v, want 1 started, 1 read_first, 1 SearchAll call, 5 offered, 1 sent", st)
	}
}

func TestSearchAllNeedsOnlyTheFewPagesAShortSearchReturned(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"w1 alpha": pagesN("a", 1, 3), "w2 beta": pagesN("b", 1, 40), "w3 gamma": pagesN("c", 1, 40)}
	cell, _ := searchCell(t, f)
	cell(`choir.SearchAll("the harbor strike", []string{"w1 alpha"})`)
	cell(`for i := 1; i <= 3; i++ { choir.Read(fmt.Sprintf("https://a.example/p%d", i)) }`)
	out := cell(`r, _ := choir.SearchAll("the harbor strike", []string{"w2 beta", "w3 gamma"})
fmt.Println(r)`)
	if f.searchCalls.Load() != 2 {
		t.Fatalf("host asked to start %d searches, want 2 (three pages tried; w3 waits on w2's):\n%s", f.searchCalls.Load(), out.Stdout)
	}
	if !strings.Contains(out.Stdout, `2 "w3 gamma": not sent`) {
		t.Fatalf("stdout:\n%s", out.Stdout)
	}
}

func TestSearchAllSendsAgainOnceTheRunHasReadTheTopPages(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"w1 alpha": pagesN("a", 1, 40), "w2 beta": pagesN("b", 1, 40)}
	cell, _ := searchCell(t, f)
	cell(`choir.SearchAll("the harbor strike", []string{"w1 alpha"})`)
	// A reworded question (same terms) is judged the same.
	out := cell(`r, _ := choir.SearchAll("Harbor strike, the", []string{"w2 beta"})
fmt.Println(r)`)
	if f.searchCalls.Load() != 1 || !strings.Contains(out.Stdout, `1 "w2 beta": not sent (the top pages`) {
		t.Fatalf("calls = %d, stdout:\n%s", f.searchCalls.Load(), out.Stdout)
	}
	cell(`for i := 1; i <= 5; i++ { choir.Read(fmt.Sprintf("https://a.example/p%d", i)) }`)
	out = cell(`r, _ := choir.SearchAll("the harbor strike", []string{"w2 beta"})
fmt.Println(r)`)
	if f.searchCalls.Load() != 2 || !strings.Contains(out.Stdout, `1 "w2 beta": sent s2`) {
		t.Fatalf("calls = %d, stdout:\n%s", f.searchCalls.Load(), out.Stdout)
	}
}

func TestSearchAllUnreadIsRunWideNotPerQuestion(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"w1 alpha": pagesN("a", 1, 40), "other one": pagesN("o", 1, 40)}
	cell, _ := searchCell(t, f)
	cell(`choir.SearchAll("the harbor strike", []string{"w1 alpha"})`)
	cell(`r, _ := choir.SearchAll("a different question", []string{"other one"})
fmt.Println(r)`)
	if f.searchCalls.Load() != 1 {
		t.Fatalf("host asked to start %d searches, want 1: another question's untried pages hold this one", f.searchCalls.Load())
	}
}

func TestSearchFreshSendsASearchTheRunHoldsUntriedPagesFor(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"w1 alpha": pagesN("a", 1, 40), "w2 beta": pagesN("b", 1, 40)}
	cell, _ := searchCell(t, f)
	out := cell(`choir.SearchAll("the harbor strike", []string{"w1 alpha", "w2 beta"})
s, err := choir.SearchFresh("w2 beta", "the first wording found the wrong strike")
fmt.Println(s.ID, len(s.Results), err)`)
	if f.searchCalls.Load() != 2 || !strings.Contains(out.Stdout, "s2 40 <nil>") {
		t.Fatalf("calls = %d, stdout:\n%s", f.searchCalls.Load(), out.Stdout)
	}
}

func TestAPlainSearchOfAFreshRunHasNoUnreadNote(t *testing.T) {
	f := newResearchFixture(t)
	cell, _ := searchCell(t, f)
	out := cell(`choir.SearchStart("plain one")`)
	if strings.Contains(strings.Join(out.Searches, "\n"), "not read") {
		t.Fatalf("a run with no unread pages is told nothing: %v", out.Searches)
	}
}

func searchAllTwice(t *testing.T, f *researchFixture, between string) (calls int32, out SessionResult) {
	t.Helper()
	f.urlsFor = map[string][]string{"w1 alpha": pagesN("a", 1, 40), "w2 beta": pagesN("b", 1, 40)}
	cell, _ := searchCell(t, f)
	cell(`choir.SearchAll("the harbor strike", []string{"w1 alpha"})`)
	cell(between)
	out = cell(`r, _ := choir.SearchAll("the harbor strike", []string{"w2 beta"})
fmt.Println(r)`)
	return f.searchCalls.Load(), out
}

func TestUnreadCountFallsWhenReadsFail(t *testing.T) {
	f := newResearchFixture(t)
	f.failing = map[string]string{}
	for i := 1; i <= 40; i++ {
		f.failing[fmt.Sprintf("https://a.example/p%d", i)] = "HTTP 403"
	}
	calls, out := searchAllTwice(t, f, `for i := 1; i <= 40; i++ { choir.Read(fmt.Sprintf("https://a.example/p%d", i)) }`)
	if calls != 2 || !strings.Contains(out.Stdout, `1 "w2 beta": sent s2`) {
		t.Fatalf("a page that will not load counts once tried: calls = %d, stdout:\n%s", calls, out.Stdout)
	}
}

func TestUnreadCountFallsWhenAHitIsReadWithReadCapture(t *testing.T) {
	f := newResearchFixture(t)
	calls, out := searchAllTwice(t, f, `for i := 1; i <= 40; i++ { choir.ReadCapture(fmt.Sprintf("cap:https://a.example/p%d", i), 0, 0) }`)
	if calls != 2 || !strings.Contains(out.Stdout, `1 "w2 beta": sent s2`) {
		t.Fatalf("calls = %d, stdout:\n%s", calls, out.Stdout)
	}
}

func TestUnreadCountFallsWhenAHitIsReadUnderAnotherURLForm(t *testing.T) {
	f := newResearchFixture(t)
	calls, out := searchAllTwice(t, f, `for i := 1; i <= 40; i++ { choir.Read(fmt.Sprintf("https://A.example/p%d/#top", i)) }`)
	if calls != 2 || !strings.Contains(out.Stdout, `1 "w2 beta": sent s2`) {
		t.Fatalf("calls = %d, stdout:\n%s", calls, out.Stdout)
	}
}

func TestARunThatTriedEveryReturnedPageIsNeverWithheld(t *testing.T) {
	f := newResearchFixture(t)
	f.failing = map[string]string{}
	for i := 1; i <= 40; i += 3 {
		f.failing[fmt.Sprintf("https://a.example/p%d", i)] = "HTTP 403"
	}
	calls, out := searchAllTwice(t, f, `for i := 1; i <= 40; i++ {
	switch i % 3 {
	case 0:
		choir.Read(fmt.Sprintf("https://a.example/p%d", i))
	case 1:
		choir.Read(fmt.Sprintf("https://a.example/p%d", i))
	default:
		choir.ReadCapture(fmt.Sprintf("cap:https://a.example/p%d/", i), 0, 0)
	}
}`)
	if calls != 2 || !strings.Contains(out.Stdout, `1 "w2 beta": sent s2`) {
		t.Fatalf("calls = %d, stdout:\n%s", calls, out.Stdout)
	}
}

func TestTheWithheldTextAndTheRefusalNameSearchFreshAndTried(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"w1 alpha": pagesN("a", 1, 40), "w2 beta": pagesN("b", 1, 40), "plain two": pagesN("p", 1, 5)}
	cell, _ := searchCell(t, f)
	cell(`choir.SearchAll("the harbor strike", []string{"w1 alpha"})`)
	out := cell(`r, _ := choir.SearchAll("the harbor strike", []string{"w2 beta"})
fmt.Println(r)
s, _ := choir.Search("plain two")
fmt.Println(s.Refused)`)
	both := out.Stdout + "\n" + strings.Join(out.Searches, "\n")
	if n := strings.Count(both, "SearchFresh"); n < 2 {
		t.Fatalf("SearchFresh named %d times, want in the withheld text and in the refusal:\n%s", n, both)
	}
	if n := strings.Count(both, "counts once you have tried it"); n < 2 {
		t.Fatalf("'a page that will not load counts once you have tried it' named %d times, want 2:\n%s", n, both)
	}
}

func TestTagQuestionSaysWhenItCannotTag(t *testing.T) {
	st := newSearchStore()
	if st.tagQuestion("s99", "harbor strike") {
		t.Fatal("tagQuestion of a search the store does not hold must say it did not tag")
	}
	st.apply(SearchFrame{ID: "s1", Kind: SearchFrameStart, Query: "x one"})
	if !st.tagQuestion("s1", "harbor strike") {
		t.Fatal("tagQuestion of a held search must say it tagged")
	}
	var a SearchStats
	a.Add(SearchStats{Untagged: 2})
	a.Add(SearchStats{Untagged: 1})
	if a.Untagged != 3 || a.IsZero() {
		t.Fatalf("stats = %+v, want untagged summed", a)
	}
}
