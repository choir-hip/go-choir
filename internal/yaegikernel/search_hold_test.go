package yaegikernel

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// The hold on re-asks (Landing 1 of docs/definitions/
// corpus-first-search-one-verb-2026-11-19.md): the paper sent about 3,900 web
// searches a day against 1,000; 8 runs with 8 or more searches sent 59% of a
// window's, and 47% of those were exact or near repeats of the run's own
// wording. The holds below are need tests (a correctness refusal on a QUESTION,
// with the fresh escape), not counts of searches: a run asking ten different
// questions is never held by them. Failure list, written before the code (and rewritten after the two reviews:
// a site: restriction, a quoted phrase and any date but today's or yesterday's
// are part of the question, not decoration):
//  1. A wording that is the run's earlier wording with only word order, stop
//     words, a weekday that is today's or yesterday's, today/tonight, or
//     today's or yesterday's date changed goes to the web again, for
//     SearchStart, Search and each SearchAll wording.
//  2. A different site: restriction, a quoted phrase, another date, a different
//     entity, number, negation or bare year is answered from the other's pages.
//  3. After a search whose pages were mostly held (under a quarter new, 8 or
//     more pages), another wording of the same question is sent although the
//     run holds pages it has not tried to read.
//  4. The hold refuses without the pages the run holds, without saying what to
//     do and what ends the hold, without fresh, or is not counted as
//     mostly_seen; SearchAll ignores it, or stops judging its later wordings,
//     so a site: wording to an outlet the run lacks is not sent.
//  5. The hold touches a question that is not the same: ten distinct questions
//     whose pages are all already held are not all sent.
//  6. Pages the run tried to read and could not read keep the hold while untried
//     pages remain, or keep it after every page was tried (a paywall).
//  7. Pages the run read lose the hold, though it holds nothing it has not read.
//  8. Two interleaved runs of one desk session wipe each other's searches: run
//     A's identical search after run B's cell goes to the web.

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
	// fresh sends it.
	cell(`choir.SearchStart("Kharkiv missile attack 20 November 2026", true)`)
	if f.searchCalls.Load() != 2 {
		t.Fatalf("fresh must send (calls %d)", f.searchCalls.Load())
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
	cell(`choir.SearchStart("site:reuters.com Kharkiv missile attack")`)
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
	out := cell(`r, _ := choir.SearchAll("Kharkiv strike", []string{"Kharkiv missile attack", "Kharkiv missile attack 20 November 2026", "site:bbc.com Kharkiv missile attack"})
fmt.Println(r)`)
	if f.searchCalls.Load() != 2 || !strings.Contains(out.Stdout, `2 "Kharkiv missile attack 20 November 2026": not sent (answered from your search s1`) || !strings.Contains(out.Stdout, `3 "site:bbc.com Kharkiv missile attack": sent s2`) {
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
		cell(fmt.Sprintf(`choir.SearchStart(%q)`, p[1]))
		if f.searchCalls.Load() != 2 {
			t.Errorf("%q and %q are different questions: both must be sent (calls %d)", p[0], p[1], f.searchCalls.Load())
		}
	}
}

// related returns n wordings of one question: they share the terms "harbor
// strike mauritius union" and add one term each.
func relatedWordings(n int) []string {
	var out []string
	for i := 1; i <= n; i++ {
		out = append(out, fmt.Sprintf("harbor strike mauritius union wording%d", i))
	}
	return out
}

func TestAWordingAfterAMostlySeenSearchIsHeldWithThePagesItHolds(t *testing.T) {
	f := newResearchFixture(t)
	pages := pagesN("h", 1, 10)
	ws := relatedWordings(3)
	f.urlsFor = map[string][]string{ws[0]: pages, ws[1]: pages, ws[2]: pagesN("z", 1, 10)}
	cell, _ := searchCell(t, f)
	cell(fmt.Sprintf(`choir.SearchStart(%q)`, ws[0])) // 10 new pages
	out := cell(fmt.Sprintf(`s, _ := choir.SearchStart(%q)
fmt.Println(s.Refused == "")`, ws[1])) // judged by s1: all new, so it is sent
	if f.searchCalls.Load() != 2 || strings.TrimSpace(out.Stdout) != "true" {
		t.Fatalf("a wording after a search with new pages is sent (calls %d): %s", f.searchCalls.Load(), out.Stdout)
	}
	// s2 returned the same 10 pages: 0 new. The next wording of the question is held.
	out = cell(fmt.Sprintf(`s, err := choir.SearchStart(%q)
fmt.Println(err, s.ID == "", len(s.Unread))
fmt.Println(s.Refused)`, ws[2]))
	if f.searchCalls.Load() != 2 {
		t.Fatalf("the third wording must be held (calls %d):\n%s", f.searchCalls.Load(), out.Stdout)
	}
	for _, want := range []string{"<nil> true 10", "the web has shown you what it has for this question", "what you hold or conclude not found", "fresh"} {
		if !strings.Contains(out.Stdout, want) {
			t.Fatalf("refusal lacks %q:\n%s", want, out.Stdout)
		}
	}
	if st := out.SearchStats; st == nil || st.NotSent["mostly_seen"] != 1 {
		t.Fatalf("stats = %+v, want 1 mostly_seen", st)
	}
	// The waiting verb hands back the pages and Refused.
	out = cell(fmt.Sprintf(`s, _ := choir.Search(%q)
fmt.Println(len(s.Results), s.Refused != "", s.Done)`, ws[2]))
	if strings.TrimSpace(out.Stdout) != "10 true true" || f.searchCalls.Load() != 2 {
		t.Fatalf("Search holds too (calls %d): %s", f.searchCalls.Load(), out.Stdout)
	}
	// fresh is the run's statement that it wants the search.
	cell(fmt.Sprintf(`choir.SearchStart(%q, true)`, ws[2]))
	if f.searchCalls.Load() != 3 {
		t.Fatalf("fresh must send (calls %d)", f.searchCalls.Load())
	}
}

func TestSearchAllHoldsTheQuestionAfterAMostlySeenSearch(t *testing.T) {
	f := newResearchFixture(t)
	pages := pagesN("h", 1, 10)
	ws := relatedWordings(4)
	f.urlsFor = map[string][]string{ws[0]: pages, ws[1]: pages, ws[2]: pagesN("z", 1, 10), ws[3]: pagesN("y", 1, 10)}
	cell, _ := searchCell(t, f)
	cell(fmt.Sprintf(`choir.Search(%q)
choir.Search(%q)`, ws[0], ws[1])) // the second returns the same 10 pages: 0 new
	out := cell(fmt.Sprintf(`r, err := choir.SearchAll("harbor strike", []string{%q, %q})
fmt.Println(err)
fmt.Println(r)`, ws[2], ws[3]))
	if f.searchCalls.Load() != 2 {
		t.Fatalf("SearchAll sent into a held question (calls %d):\n%s", f.searchCalls.Load(), out.Stdout)
	}
	if !strings.Contains(out.Stdout, "the web has shown you what it has for this question") || !strings.Contains(out.Stdout, "h.example") {
		t.Fatalf("SearchAll names the hold and carries the held pages:\n%s", out.Stdout)
	}
	if st := out.SearchStats; st == nil || st.NotSent["mostly_seen"] < 1 {
		t.Fatalf("stats = %+v, want mostly_seen counted", st)
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

func TestAFewPagesAreNotEvidenceThatTheWebHasShownItsAll(t *testing.T) {
	f := newResearchFixture(t)
	ws := relatedWordings(3)
	f.urlsFor = map[string][]string{ws[0]: pagesN("h", 1, 4), ws[1]: pagesN("h", 1, 4), ws[2]: pagesN("z", 1, 4)}
	cell, _ := searchCell(t, f)
	cell(fmt.Sprintf(`choir.Search(%q)`, ws[0]))
	cell(fmt.Sprintf(`choir.Search(%q)`, ws[1]))
	cell(fmt.Sprintf(`choir.Search(%q)`, ws[2]))
	if f.searchCalls.Load() != 3 {
		t.Fatalf("4 pages are too few to judge (calls %d)", f.searchCalls.Load())
	}
}

func TestTheHoldLiftsWhenTheRunHasReadWhatItHolds(t *testing.T) {
	f := newResearchFixture(t)
	pages := pagesN("h", 1, 10)
	ws := relatedWordings(3)
	f.urlsFor = map[string][]string{ws[0]: pages, ws[1]: pages, ws[2]: pagesN("z", 1, 10)}
	cell, _ := searchCell(t, f)
	cell(fmt.Sprintf(`choir.Search(%q)
choir.Search(%q, true)`, ws[0], ws[1]))
	cell(`for _, u := range choir.Searches.List()[0].Results { choir.Read(u.URL) }`)
	cell(fmt.Sprintf(`choir.Search(%q)`, ws[2]))
	if f.searchCalls.Load() != 3 {
		t.Fatalf("a run that read everything it holds may search again (calls %d)", f.searchCalls.Load())
	}
}

// A page the run tried and could not read is not "read": while pages remain
// that it has not tried, the question stays held. When every page held for the
// question has been tried and failed (a paywall, dead links), nothing readable
// is held and the next wording is SENT.
func TestDeadReadsKeepTheHoldWhileUntriedPagesRemainAndReleaseItWhenAllFailed(t *testing.T) {
	setup := func(failing int) (*researchFixture, func(string) SessionResult, []string) {
		f := newResearchFixture(t)
		pages := pagesN("h", 1, 10)
		for _, u := range pages[:failing] {
			f.failing[u] = "HTTP 403"
		}
		ws := relatedWordings(3)
		f.urlsFor = map[string][]string{ws[0]: pages, ws[1]: pages, ws[2]: pagesN("z", 1, 10)}
		cell, _ := searchCell(t, f)
		cell(fmt.Sprintf(`choir.Search(%q)
choir.Search(%q, true)`, ws[0], ws[1]))
		return f, cell, ws
	}
	// 5 of 10 fail and are tried; 5 are untried: held.
	f, cell, ws := setup(5)
	cell(`for _, u := range choir.Searches.List()[0].Results[:5] { choir.Read(u.URL) }`)
	out := cell(fmt.Sprintf(`s, _ := choir.Search(%q)
fmt.Println(s.Refused != "", len(s.Results))`, ws[2]))
	if f.searchCalls.Load() != 2 || strings.TrimSpace(out.Stdout) != "true 5" {
		t.Fatalf("5 untried pages hold the question (calls %d): %s", f.searchCalls.Load(), out.Stdout)
	}
	// Every page tried and every one failed: nothing readable is held: sent.
	f, cell, ws = setup(10)
	cell(`for _, u := range choir.Searches.List()[0].Results { choir.Read(u.URL) }`)
	out = cell(fmt.Sprintf(`s, _ := choir.Search(%q)
fmt.Println(s.Refused == "")`, ws[2]))
	if f.searchCalls.Load() != 3 || strings.TrimSpace(out.Stdout) != "true" {
		t.Fatalf("10 dead pages hold nothing readable: the next wording is sent (calls %d): %s", f.searchCalls.Load(), out.Stdout)
	}
	// The refusal says what releases it.
	if !strings.Contains(mostlySeenRefusal(5), "tried every page you hold") || !strings.Contains(mostlySeenRefusal(5), "next wording is sent") {
		t.Fatal("the refusal must say what ends the hold")
	}
}

// A wording that restricts the search to an outlet the run holds no page from
// asks for new outlets by construction: never "mostly seen". A restriction to
// an outlet the run already holds may be held.
func TestMostlySeenNeverHoldsASearchForAnOutletTheRunLacks(t *testing.T) {
	f := newResearchFixture(t)
	pages := pagesN("h", 1, 10)
	lw := func(i int) string { return fmt.Sprintf("harbor strike mauritius union port workers wording%d", i) }
	f.urlsFor = map[string][]string{lw(0): pages, lw(1): pages}
	cell, _ := searchCell(t, f)
	cell(fmt.Sprintf(`choir.Search(%q)
choir.Search(%q, true)`, lw(0), lw(1))) // the question is now mostly seen
	// A restriction to an outlet the run already holds may be held.
	out := cell(fmt.Sprintf(`s, _ := choir.Search("site:h.example " + %q)
fmt.Println(s.Refused != "")`, lw(2)))
	if f.searchCalls.Load() != 2 || strings.TrimSpace(out.Stdout) != "true" {
		t.Fatalf("a restriction to an outlet the run holds may be held (calls %d): %s", f.searchCalls.Load(), out.Stdout)
	}
	// A restriction to an outlet the run holds no page from asks for new outlets
	// by construction: sent.
	out = cell(fmt.Sprintf(`s, _ := choir.Search("site:z.example " + %q)
fmt.Println(s.Refused == "")`, lw(3)))
	if f.searchCalls.Load() != 3 || strings.TrimSpace(out.Stdout) != "true" {
		t.Fatalf("a restriction to an outlet the run lacks must be sent (calls %d): %s", f.searchCalls.Load(), out.Stdout)
	}
}

func TestRelatedWordingsNeedSharedTermsAndOverlap(t *testing.T) {
	if relatedTerms(strings.Fields("kharkiv missile attack death toll"), strings.Fields("kharkiv missile attack hospital")) {
		t.Error("three shared terms: two claims about one event are not one question")
	}
	if !relatedTerms(strings.Fields("harbor strike mauritius union w1"), strings.Fields("harbor strike mauritius union w2")) {
		t.Error("four shared terms at 66%: one question")
	}
	if relatedTerms(strings.Fields("a b c d e f g h"), strings.Fields("a b c d x y z w")) {
		t.Error("four shared terms but 33% overlap: not one question")
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

// With a normal 40-page free search, the run-wide unread gate (15 or more pages
// the run has not tried to read) refuses the second-outlet search first: the run
// must read, or pass fresh. This is the pre-existing read-first rule, not the
// near key: what the run must do is shown here.
func TestASecondOutletSearchAfterA40PageResultNeedsReadingOrFresh(t *testing.T) {
	f := newResearchFixture(t)
	fixedNow(f)
	f.urlsFor = map[string][]string{"Kharkiv missile attack": pagesN("k", 1, 40)}
	cell, _ := searchCell(t, f)
	cell(`choir.SearchStart("Kharkiv missile attack")`)
	out := cell(`s, _ := choir.SearchStart("site:apnews.com Kharkiv missile attack")
fmt.Println(s.Refused != "", len(s.Unread))`)
	if f.searchCalls.Load() != 1 || strings.TrimSpace(out.Stdout) != "true 40" {
		t.Fatalf("40 unread pages hold the next search (calls %d): %s", f.searchCalls.Load(), out.Stdout)
	}
	cell(`for i := 1; i <= 26; i++ { choir.Read(fmt.Sprintf("https://k.example/p%d", i)) }`)
	cell(`choir.SearchStart("site:apnews.com Kharkiv missile attack")`)
	if f.searchCalls.Load() != 2 {
		t.Fatalf("after reading all but 14, the second-outlet search is sent (calls %d)", f.searchCalls.Load())
	}
	cell(`choir.SearchStart("site:bbc.com Kharkiv missile attack", true)`)
	if f.searchCalls.Load() != 3 {
		t.Fatalf("fresh sends (calls %d)", f.searchCalls.Load())
	}
}

// SearchAll judges each wording on its own: after one is held as mostly seen, a
// later wording that restricts to an outlet the run lacks is SENT.
func TestSearchAllSendsASiteWordingToAnUnheldOutletAfterAHeldWording(t *testing.T) {
	f := newResearchFixture(t)
	pages := pagesN("h", 1, 10)
	lw := func(i int) string { return fmt.Sprintf("harbor strike mauritius union port workers wording%d", i) }
	f.urlsFor = map[string][]string{lw(0): pages, lw(1): pages}
	cell, _ := searchCell(t, f)
	cell(fmt.Sprintf(`choir.Search(%q)
choir.Search(%q, true)`, lw(0), lw(1))) // the question is mostly seen
	out := cell(fmt.Sprintf(`r, err := choir.SearchAll("harbor strike", []string{%q, %q, "site:z.example " + %q})
fmt.Println(err)
fmt.Println(r)
fmt.Println(r.Next)`, lw(2), lw(3), lw(4)))
	if f.searchCalls.Load() != 3 {
		t.Fatalf("the site: wording to an outlet the run lacks must be sent (calls %d):\n%s", f.searchCalls.Load(), out.Stdout)
	}
	for _, want := range []string{"2 \"" + lw(3) + "\": not sent", "3 \"site:z.example " + lw(4) + "\": sent s3", "tried every page you hold", "next wording is sent"} {
		if !strings.Contains(out.Stdout, want) {
			t.Fatalf("stdout lacks %q:\n%s", want, out.Stdout)
		}
	}
	if strings.Contains(out.Stdout, "the web has no more") {
		t.Fatalf("SearchAll must not claim the web has no more:\n%s", out.Stdout)
	}
	// Held wordings are counted apart from those the call could have sent.
	if st := out.SearchStats; st == nil || st.NotSent["mostly_seen"] != 2 || st.AllOffered != 1 || st.AllSent != 1 {
		t.Fatalf("stats = %+v, want 2 mostly_seen, 1 offered, 1 sent", st)
	}
}

func TestChoirSearchDescriptionNamesTheMostlySeenHoldAndItsRelease(t *testing.T) {
	d := choirFunctionDocs["Search"].Summary
	for _, want := range []string{"mostly pages you already held", "tried every page", "Reused"} {
		if !strings.Contains(d, want) {
			t.Errorf("the Search description lacks %q", want)
		}
	}
}
