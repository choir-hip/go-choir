package yaegikernel

import (
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
		`1 "w1 alpha": sent s1 (40 new in first 10`,
		`2 "w2 beta": not sent (this question already holds 40 pages new to this run that you have not read`,
		`5 "w5 omega": not sent (this question already holds 40 pages`,
		"read the ones that matter, then search again for what is still missing, naming it",
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
	for _, want := range []string{"40 pages", "new to this run", "not read", "search again for what is still missing, naming it"} {
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
