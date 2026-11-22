package yaegikernel

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// The same question again (Landing 1 of docs/definitions/corpus-first-search-one-verb-2026-11-19.md):
// a wording that is the run's earlier wording with only word order, stop words, a weekday
// that is today's or yesterday's, today/tonight, or today's or yesterday's date changed is
// answered from the earlier search, for SearchStart, Search and each SearchAll wording; a
// different site: restriction, a quoted phrase, another date, a different entity, number,
// negation or bare year is a different question and is sent. That is correctness, not a
// need rule, and it stays. (The need rules, mostly_seen and the read-first refusals, were
// removed 2026-11-22.) Ten different questions are never held.

func fixedNow(f *researchFixture) {
	clk := time.Date(2026, 11, 20, 14, 0, 0, 0, time.UTC)
	f.scope.searches.now = func() time.Time { return clk }
}

func TestANearRepeatIsAnsweredFromTheEarlierSearch(t *testing.T) {
	f := newResearchFixture(t)
	fixedNow(f)
	f.urlsFor = map[string][]string{"Kharkiv missile attack": pagesN("k", 1, 5)}
	cell, _ := searchCell(t, f)
	cell(`choir.SearchStart("Kharkiv missile attack")`)
	for _, q := range []string{
		"Kharkiv missile attack 20 November 2026", // today's date
		"Kharkiv missile attack 19 Nov",           // yesterday's
		"Kharkiv missile attack Friday",           // today (the fixed clock is Friday 2026-11-20)
		"the attack, missile Kharkiv, today",
	} {
		out := cell(fmt.Sprintf(`b, _ := choir.SearchStart(%q)
fmt.Println(b.ID, b.Reused != "")`, q))
		if strings.TrimSpace(out.Stdout) != "s1 true" || f.searchCalls.Load() != 1 {
			t.Fatalf("%q must be answered from s1 (calls %d): %s", q, f.searchCalls.Load(), out.Stdout)
		}
	}
	// The waiting verb returns the earlier search and says it is not a new answer.
	out := cell(`s, _ := choir.Search("Kharkiv missile attack Friday")
fmt.Println(s.ID, len(s.Results), s.Reused != "", s.ReusedFrom, s.Query)`)
	if strings.TrimSpace(out.Stdout) != "s1 5 true s1 Kharkiv missile attack" || f.searchCalls.Load() != 1 {
		t.Fatalf("Search must return s1 marked Reused (calls %d): %s", f.searchCalls.Load(), out.Stdout)
	}
}

// A search that changes where to look is a different question, however it is
// worded: the second-outlet hunt of a story that publishes on two outlets.
func TestAChangedSiteRestrictionQuoteOrDateIsSent(t *testing.T) {
	for _, q := range []string{
		"site:apnews.com Kharkiv missile attack",
		"site:reuters.com Kharkiv missile attack",
		`"Kharkiv missile" attack`,
		"Kharkiv missile attack 3 November 2026",
		"Kharkiv missile attack 2025",
	} {
		f := newResearchFixture(t)
		fixedNow(f)
		cell, _ := searchCell(t, f)
		cell(`choir.SearchStart("Kharkiv missile attack")`)
		cell(readEverything)
		out := cell(fmt.Sprintf(`b, _ := choir.SearchStart(%q)
fmt.Println(b.Reused == "", b.Refused == "")`, q))
		if strings.TrimSpace(out.Stdout) != "true true" || f.searchCalls.Load() != 2 {
			t.Errorf("%q is a different question and must be sent (calls %d): %s", q, f.searchCalls.Load(), out.Stdout)
		}
	}
	// The near and site rules alone: a claim searched freely (a small result, so
	// the pre-existing 15-unread gate does not apply), then the same words
	// restricted to one outlet and to another: three searches. With a normal
	// 40-page result the unread gate applies first (next test).
	f := newResearchFixture(t)
	fixedNow(f)
	cell, _ := searchCell(t, f)
	cell(`choir.SearchStart("Kharkiv missile attack")`)
	cell(readEverything)
	cell(`choir.SearchStart("site:reuters.com Kharkiv missile attack")`)
	cell(readEverything)
	cell(`choir.SearchStart("site:apnews.com Kharkiv missile attack")`)
	cell(`choir.SearchStart("Kharkiv missile attack site:reuters.com")`) // the same restriction again: reused
	if f.searchCalls.Load() != 3 {
		t.Fatalf("three questions, three sends; a repeat of one is reused (calls %d)", f.searchCalls.Load())
	}
}

func TestANearRepeatWordingOfSearchAllIsAnsweredFromHeld(t *testing.T) {
	f := newResearchFixture(t)
	fixedNow(f)
	f.urlsFor = map[string][]string{"Kharkiv missile attack": pagesN("k", 1, 5)}
	cell, _ := searchCell(t, f)
	out := cell(`r, _ := choir.SearchAll("Kharkiv strike", []string{"Kharkiv missile attack"})
` + readEverything + `
r, _ = choir.SearchAll("Kharkiv strike", []string{"Kharkiv missile attack 20 November 2026", "site:bbc.com Kharkiv missile attack"})
fmt.Println(r)`)
	if f.searchCalls.Load() != 2 || !strings.Contains(out.Stdout, `1 "Kharkiv missile attack 20 November 2026": not sent (answered from your search s1`) || !strings.Contains(out.Stdout, `2 "site:bbc.com Kharkiv missile attack": sent s2`) {
		t.Fatalf("a dated wording is answered from s1, the bbc-restricted one is sent (calls %d):\n%s", f.searchCalls.Load(), out.Stdout)
	}
}

func TestDifferentQuestionsAreNotMerged(t *testing.T) {
	pairs := [][2]string{
		{"Kharkiv missile attack", "Kyiv missile attack"},
		{"7 killed in Kharkiv attack", "9 killed in Kharkiv attack"},
		{"Kharkiv attack", "Kharkiv attack not confirmed"},
		{"Kharkiv attack", "Kharkiv attack -casualties"},
		{"2025 budget vote", "2026 budget vote"},
		{"protest march Paris", "protest Paris"},
		{"earnings 2026-10-01", "earnings 2026-11-01"},
	}
	for _, p := range pairs {
		f := newResearchFixture(t)
		f.urlsFor = map[string][]string{p[0]: pagesN("x", 1, 5), p[1]: pagesN("y", 1, 5)}
		cell, _ := searchCell(t, f)
		cell(fmt.Sprintf(`choir.SearchStart(%q)`, p[0]))
		cell(readEverything)
		cell(fmt.Sprintf(`choir.SearchStart(%q)`, p[1]))
		if f.searchCalls.Load() != 2 {
			t.Errorf("%q and %q are different questions: both must be sent (calls %d)", p[0], p[1], f.searchCalls.Load())
		}
	}
}

// The hold is a need test on one question, not a count: ten different
// questions whose pages are all already held are all sent.
func TestTenDistinctQuestionsAreAllSentWhateverTheyReturn(t *testing.T) {
	f := newResearchFixture(t)
	shared := pagesN("s", 1, 10) // every question returns the same 10 pages
	f.urlsFor = map[string][]string{}
	topics := []string{"harbor strike", "wheat harvest", "central bank rates", "volcano eruption", "chess championship", "vaccine trial", "bridge collapse", "film festival", "fuel prices", "rail merger"}
	for _, q := range topics {
		f.urlsFor[q] = shared
	}
	cell, _ := searchCell(t, f)
	cell(readTop(10, "s")) // the run has read the pages every question returns
	for _, q := range topics {
		out := cell(fmt.Sprintf(`s, _ := choir.Search(%q)
fmt.Println(s.Refused == "", len(s.Results))`, q))
		if strings.TrimSpace(out.Stdout) != "true 10" {
			t.Fatalf("question %q was held: %s", q, out.Stdout)
		}
	}
	if f.searchCalls.Load() != 10 {
		t.Fatalf("ten distinct questions: %d sent, want 10", f.searchCalls.Load())
	}
}

// One desk session serves two runs in turn (acquire waits for the session and
// then serves the other run in the same worker): a run's searches are its own,
// and the other run's cell does not wipe them.
func TestInterleavedRunsKeepTheirOwnSearches(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"harbor strike": pagesN("h", 1, 5), "wheat harvest": pagesN("w", 1, 5)}
	cell, hooks := f.session(t)
	hooks.Begin(SessionFrame{RunID: "run-A"})
	cell(`choir.Search("harbor strike")`)
	hooks.Begin(SessionFrame{RunID: "run-B"})
	cell(`choir.Search("wheat harvest")`)
	hooks.Begin(SessionFrame{RunID: "run-A"})
	out := cell(`s, _ := choir.SearchStart("harbor strike")
fmt.Println(s.ID, s.Reused != "")`)
	cell(readTop(5, "h"))
	if strings.TrimSpace(out.Stdout) != "s1 true" || f.searchCalls.Load() != 2 {
		t.Fatalf("run A's own search must answer run A after run B's cell (calls %d): %s", f.searchCalls.Load(), out.Stdout)
	}
	// And run B's pages are not run A's: B's search is not reused by A.
	out = cell(`s, _ := choir.SearchStart("wheat harvest")
fmt.Println(s.Reused != "")`)
	if strings.TrimSpace(out.Stdout) != "false" || f.searchCalls.Load() != 3 {
		t.Fatalf("run A must not be answered from run B's search (calls %d): %s", f.searchCalls.Load(), out.Stdout)
	}
}
