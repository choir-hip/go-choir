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
//  6. A plain SearchStart or Search of a run holding many unread pages carries
//     no note, or the note stops the search.
//  7. The run counts do not say how many SearchAll calls were made, how many
//     wordings they were offered and how many they sent.

func TestSearchAllSendsOneWordingWhenTheFirstFillsTheQuestion(t *testing.T) {
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
		`2 "w2 beta": not sent (this question already holds 40 pages new to this run that you have not tried to read`,
		`5 "w5 omega": not sent (this question already holds 40 pages`,
		"then search again for what is still missing, naming it",
	} {
		if !strings.Contains(out.Stdout, want) {
			t.Fatalf("stdout lacks %q:\n%s", want, out.Stdout)
		}
	}
	st := out.SearchStats
	if st == nil || st.Started != 1 || st.NotSent["holds_unread"] != 4 || st.AllCalls != 1 || st.AllOffered != 5 || st.AllSent != 1 {
		t.Fatalf("stats = %+v, want 1 started, 4 holds_unread, 1 SearchAll call, 5 offered, 1 sent", st)
	}
}

func TestSearchAllSendsTheNextWordingWhenTheFirstReturnsLittle(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"w1 alpha": pagesN("a", 1, 3), "w2 beta": pagesN("b", 1, 40), "w3 gamma": pagesN("c", 1, 40)}
	cell, _ := searchCell(t, f)
	out := cell(`r, _ := choir.SearchAll("the harbor strike", []string{"w1 alpha", "w2 beta", "w3 gamma"})
fmt.Println(r)`)
	if f.searchCalls.Load() != 2 {
		t.Fatalf("host asked to start %d searches, want 2 (3 pages is not enough; 43 is):\n%s", f.searchCalls.Load(), out.Stdout)
	}
	if !strings.Contains(out.Stdout, `3 "w3 gamma": not sent (this question already holds 43 pages`) {
		t.Fatalf("stdout:\n%s", out.Stdout)
	}
}

func TestSearchAllSendsAgainOnceTheRunHasReadPagesOfTheQuestion(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"w1 alpha": pagesN("a", 1, 40), "w2 beta": pagesN("b", 1, 40), "w3 gamma": pagesN("c", 1, 40)}
	cell, _ := searchCell(t, f)
	cell(`r, _ := choir.SearchAll("the harbor strike", []string{"w1 alpha"})
fmt.Println(len(r.Results))`)
	// Unread pages remain: a later call for the same question (reworded
	// question, same terms) sends nothing.
	out := cell(`r, _ := choir.SearchAll("Harbor strike, the", []string{"w2 beta"})
fmt.Println(r)`)
	if f.searchCalls.Load() != 1 || !strings.Contains(out.Stdout, `1 "w2 beta": not sent (this question already holds 40 pages`) {
		t.Fatalf("calls = %d, stdout:\n%s", f.searchCalls.Load(), out.Stdout)
	}
	// The run reads 30 of the 40: 10 unread is below the line, so it sends.
	cell(`for i := 1; i <= 30; i++ { choir.Read(fmt.Sprintf("https://a.example/p%d", i)) }`)
	out = cell(`r, _ := choir.SearchAll("the harbor strike", []string{"w2 beta"})
fmt.Println(r)`)
	if f.searchCalls.Load() != 2 || !strings.Contains(out.Stdout, `1 "w2 beta": sent s2`) {
		t.Fatalf("calls = %d, stdout:\n%s", f.searchCalls.Load(), out.Stdout)
	}
}

func TestSearchAllUnreadIsPerQuestion(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"w1 alpha": pagesN("a", 1, 40), "other one": pagesN("o", 1, 40)}
	cell, _ := searchCell(t, f)
	cell(`choir.SearchAll("the harbor strike", []string{"w1 alpha"})`)
	cell(`r, _ := choir.SearchAll("a different question", []string{"other one"})
fmt.Println(r)`)
	if f.searchCalls.Load() != 2 {
		t.Fatalf("host asked to start %d searches, want 2: unread pages of one question do not withhold another", f.searchCalls.Load())
	}
}

func TestFreshSendsASearchTheQuestionHoldsUnreadPagesFor(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"w1 alpha": pagesN("a", 1, 40), "w2 beta": pagesN("b", 1, 40)}
	cell, _ := searchCell(t, f)
	out := cell(`choir.SearchAll("the harbor strike", []string{"w1 alpha", "w2 beta"})
s, err := choir.Search("w2 beta", true)
fmt.Println(s.ID, len(s.Results), err)`)
	if f.searchCalls.Load() != 2 || !strings.Contains(out.Stdout, "s2 40 <nil>") {
		t.Fatalf("calls = %d, stdout:\n%s", f.searchCalls.Load(), out.Stdout)
	}
}

func TestAPlainSearchOfARunHoldingUnreadPagesCarriesANoteAndIsSent(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"w1 alpha": pagesN("a", 1, 40), "plain two": pagesN("p", 1, 5)}
	cell, _ := searchCell(t, f)
	cell(`choir.SearchAll("the harbor strike", []string{"w1 alpha"})`)
	out := cell(`s, _ := choir.Search("plain two")
fmt.Println(len(s.Results))`)
	if f.searchCalls.Load() != 2 {
		t.Fatalf("the plain search must still be sent; host searches = %d", f.searchCalls.Load())
	}
	notes := strings.Join(out.Searches, "\n")
	for _, want := range []string{"40 pages", "new to this run", "not tried to read", "search again for what is still missing, naming it"} {
		if !strings.Contains(notes, want) {
			t.Fatalf("notes lack %q:\n%s", want, notes)
		}
	}
	if st := out.SearchStats; st == nil || st.UnreadNotes != 1 {
		t.Fatalf("stats = %+v, want 1 unread note", st)
	}
	// One note per cell, not one per search of a burst.
	out = cell(`choir.SearchStart("burst one")
choir.SearchStart("burst two")`)
	if n := strings.Count(strings.Join(out.Searches, "\n"), "new to this run"); n != 1 {
		t.Fatalf("%d unread notes for a burst of two, want 1:\n%s", n, strings.Join(out.Searches, "\n"))
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

// From the review of 56c96bf09 (seat 2). The unread count stuck: a page was
// unread until a Read of it succeeded, so a dead link, a 403, a page read only
// with ReadCapture or under another form of its URL never lowered it, and a
// question whose 15 unread pages could not be loaded was withheld for the whole
// run. The rule is "pages returned that the run has not yet tried". Failure
// list, written before the code:
//  8. 40 hits whose reads all fail leave the question withheld.
//  9. A hit read through ReadCapture (a capture made from the page) leaves it
//     counted unread.
//  10. A hit read under another form of its URL (host case, fragment, trailing
//     slash: the one form searchHitKey folds) leaves it counted unread.
//  11. A run that tried every returned page is still refused its next wording.
//  12. The withheld text and the plain-search note do not name fresh, nor say
//     that a page that will not load counts once it was tried.
//  13. tagQuestion on a search the store does not hold says nothing, so the
//     gate fails open unseen.

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
	calls, out := searchAllTwice(t, f, `for i := 1; i <= 30; i++ { choir.Read(fmt.Sprintf("https://a.example/p%d", i)) }`)
	if calls != 2 || !strings.Contains(out.Stdout, `1 "w2 beta": sent s2`) {
		t.Fatalf("a page that will not load counts once tried: calls = %d, stdout:\n%s", calls, out.Stdout)
	}
}

func TestUnreadCountFallsWhenAHitIsReadWithReadCapture(t *testing.T) {
	f := newResearchFixture(t)
	calls, out := searchAllTwice(t, f, `for i := 1; i <= 30; i++ { choir.ReadCapture(fmt.Sprintf("cap:https://a.example/p%d", i), 0, 0) }`)
	if calls != 2 || !strings.Contains(out.Stdout, `1 "w2 beta": sent s2`) {
		t.Fatalf("calls = %d, stdout:\n%s", calls, out.Stdout)
	}
}

func TestUnreadCountFallsWhenAHitIsReadUnderAnotherURLForm(t *testing.T) {
	f := newResearchFixture(t)
	calls, out := searchAllTwice(t, f, `for i := 1; i <= 30; i++ { choir.Read(fmt.Sprintf("https://A.example/p%d/#top", i)) }`)
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

func TestTheWithheldTextAndTheNoteNameFreshAndTried(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"w1 alpha": pagesN("a", 1, 40), "w2 beta": pagesN("b", 1, 40), "plain two": pagesN("p", 1, 5)}
	cell, _ := searchCell(t, f)
	cell(`choir.SearchAll("the harbor strike", []string{"w1 alpha"})`)
	out := cell(`r, _ := choir.SearchAll("the harbor strike", []string{"w2 beta"})
fmt.Println(r)
choir.Search("plain two")`)
	both := out.Stdout + "\n" + strings.Join(out.Searches, "\n")
	if n := strings.Count(both, "fresh"); n < 2 {
		t.Fatalf("fresh named %d times, want in the withheld text and in the note:\n%s", n, both)
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
