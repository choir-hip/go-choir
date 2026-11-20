package yaegikernel

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// Every web search a run sends passes the need test
// (docs/problems/web-searches-are-sent-without-asking-what-is-held-2026-11-19.md,
// fix 1). How the verbs differed before this change (read from the code):
//   - SearchAll(question, wordings) judged each wording against the pages
//     held for ITS QUESTION (unreadNew(qk) >= 15), refused it as holds_unread
//     with the count and "Next" text, and stopped when wordings stopped
//     yielding. It sent through SearchStart.
//   - SearchStart(query, fresh...) had no test: it answered a same-terms query
//     from a held search (reuseSearch), else added a once-per-cell NOTE when
//     the run held 15 or more unread pages (noteUnreadPages) and SENT. It
//     returned a SearchStub at once, without waiting.
//   - Search(query, fresh...) was SearchStart + SearchWait(15 s) + Get, so the
//     same: no test.
//   - fresh skipped only the same-terms reuse; it was never logged.
//   - The run line counted holds_unread only for SearchAll, and unread_notes
//     for the note.
//
// Failure list, written before the code:
//  1. SearchStart with 15 or more unread pages held sends the search.
//  2. Its refusal, if any, names no reason, no count, no held pages, or is an
//     error, or is not counted as holds_unread in the run's counts.
//  3. Search (the waiting verb) sends it too, or returns an error, or returns
//     a Search that does not carry the held pages and the reason.
//  4. With fewer than 15 unread pages the search is not sent.
//  5. fresh does not send it, or is not counted when the gate would have
//     refused (fresh_over_unread).
//  6. A burst of five SearchStart calls in one cell sends all five: each is
//     judged before the earlier one's pages have arrived.
//  7. A search started while another of the run is still running waits for it
//     (blocking the cell up to 15 s, and the cell's own timeout) or is sent
//     unjudged: it must come back at once, not sent, naming the running search
//     (rule of 2026-11-19, replacing the wait: a cell cannot time out on it).
//  8. Searches started from several goroutines at once are all sent.
//  9. SearchWait of a refused stub's empty id fails with "no search": it must
//     say the search was not sent.
// 10. The SearchStart and Search surface docs still say the unread pages are
//     "a note only; the search is sent".
//
// The burst rule: one search is judged and sent at a time per cell scope. A
// search started while another of the run is still running cannot be judged by
// the pages that one will return, so it is refused at once (no waiting), naming
// the running search; a burst of five sends the first. fresh sends anyway.

func seedUnread(t *testing.T, f *researchFixture, n int) func(string) SessionResult {
	t.Helper()
	f.urlsFor = map[string][]string{"w1 alpha": pagesN("a", 1, n), "plain two": pagesN("p", 1, 5), "plain three": pagesN("q", 1, 5)}
	cell, _ := searchCell(t, f)
	cell(`choir.SearchAll("the harbor strike", []string{"w1 alpha"})`)
	if f.searchCalls.Load() != 1 {
		t.Fatalf("seed sent %d searches", f.searchCalls.Load())
	}
	return cell
}

func TestSearchStartRefusedWhenTheRunHoldsEnoughUnreadPages(t *testing.T) {
	f := newResearchFixture(t)
	cell := seedUnread(t, f, 40)
	out := cell(`s, err := choir.SearchStart("plain two")
fmt.Println("err:", err)
fmt.Println("id:", s.ID)
fmt.Println(s)`)
	if f.searchCalls.Load() != 1 {
		t.Fatalf("the provider was asked %d times, want 1 (the seed only):\n%s", f.searchCalls.Load(), out.Stdout)
	}
	for _, want := range []string{"err: <nil>", "id: \n", "not sent", "40 pages new to this run", "have not tried to read", "a.example (40)", "https://a.example/p1", "fresh"} {
		if !strings.Contains(out.Stdout, want) {
			t.Fatalf("stdout lacks %q:\n%s", want, out.Stdout)
		}
	}
	if st := out.SearchStats; st == nil || st.NotSent["holds_unread"] != 1 || st.Started != 0 {
		t.Fatalf("stats = %+v, want 1 holds_unread and nothing started", st)
	}
}

func TestSearchRefusedReturnsTheHeldPages(t *testing.T) {
	f := newResearchFixture(t)
	cell := seedUnread(t, f, 40)
	out := cell(`s, err := choir.Search("plain two")
fmt.Println(err, s.ID == "", s.Done, len(s.Results), s.Refused != "")
fmt.Println(s.Refused)`)
	if f.searchCalls.Load() != 1 {
		t.Fatalf("the provider was asked %d times, want 1:\n%s", f.searchCalls.Load(), out.Stdout)
	}
	if !strings.Contains(out.Stdout, "<nil> true true 40 true") || !strings.Contains(out.Stdout, "40 pages new to this run") {
		t.Fatalf("stdout:\n%s", out.Stdout)
	}
}

func TestSearchStartIsSentBelowTheLine(t *testing.T) {
	f := newResearchFixture(t)
	cell := seedUnread(t, f, 14)
	cell(`choir.SearchStart("plain two")`)
	if f.searchCalls.Load() != 2 {
		t.Fatalf("14 unread pages are below the line; provider asked %d times, want 2", f.searchCalls.Load())
	}
}

func TestFreshSendsAndIsCountedWhenTheGateWouldRefuse(t *testing.T) {
	f := newResearchFixture(t)
	cell := seedUnread(t, f, 40)
	out := cell(`s, err := choir.SearchStart("plain two", true)
fmt.Println(s.ID, err)`)
	if f.searchCalls.Load() != 2 || !strings.Contains(out.Stdout, "s2 <nil>") {
		t.Fatalf("fresh must send: calls %d\n%s", f.searchCalls.Load(), out.Stdout)
	}
	if st := out.SearchStats; st == nil || st.FreshOverUnread != 1 || st.NotSent["holds_unread"] != 0 {
		t.Fatalf("stats = %+v, want fresh_over_unread 1", st)
	}
}

func TestABurstOfFiveInOneCellSendsOne(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{}
	for _, q := range []string{"burst one", "burst two", "burst three", "burst four", "burst five"} {
		f.urlsFor[q] = pagesN(strings.ReplaceAll(q, " ", ""), 1, 40)
	}
	cell, _ := searchCell(t, f)
	out := cell(`for _, q := range []string{"burst one", "burst two", "burst three", "burst four", "burst five"} {
	s, _ := choir.SearchStart(q)
	fmt.Println(q, "->", s.ID)
}`)
	if f.searchCalls.Load() != 1 {
		t.Fatalf("a burst of five sent %d, want 1:\n%s", f.searchCalls.Load(), out.Stdout)
	}
	if st := out.SearchStats; st == nil || st.NotSent["holds_unread"] != 4 {
		t.Fatalf("stats = %+v, want 4 holds_unread", st)
	}
}

func TestASearchStartedWhileAnotherRunsIsRefusedAtOnce(t *testing.T) {
	f := newResearchFixture(t)
	f.runningFor = map[string]bool{"burst one": true}
	cell, _ := searchCell(t, f)
	out := cell(`import "time"
choir.SearchStart("burst one")
t0 := time.Now()
s, err := choir.SearchStart("burst two")
fmt.Println(err, s.ID == "s1", time.Since(t0) < time.Second)
fmt.Println(s.Refused)`)
	if f.searchCalls.Load() != 1 {
		t.Fatalf("the second search was sent (calls %d):\n%s", f.searchCalls.Load(), out.Stdout)
	}
	for _, want := range []string{"<nil> true true", "a search of this run is still running (s1)", `choir.SearchWait("s1", 15000)`, "fresh"} {
		if !strings.Contains(out.Stdout, want) {
			t.Fatalf("stdout lacks %q:\n%s", want, out.Stdout)
		}
	}
	if st := out.SearchStats; st == nil || st.NotSent["search_running"] != 1 {
		t.Fatalf("stats = %+v, want 1 search_running", st)
	}
	if notes := strings.Join(out.Searches, "\n"); !strings.Contains(notes, "not sent: burst two") {
		t.Fatalf("cell result lacks the note:\n%s", notes)
	}
	// The first search ends with few pages: the next search is judged and sent.
	f.scope.searches.apply(providerFrame("s1", "keenable", true, pagesN("r", 1, 5)...))
	f.scope.searches.apply(SearchFrame{ID: "s1", Kind: SearchFrameDone})
	cell(`choir.SearchStart("burst two")`)
	if f.searchCalls.Load() != 2 {
		t.Fatalf("after the first ended under the unread line, the search must be sent (calls %d)", f.searchCalls.Load())
	}
}

func TestAfterTheRunningSearchEndsWithManyPagesTheNextIsRefusedForThem(t *testing.T) {
	f := newResearchFixture(t)
	f.runningFor = map[string]bool{"burst one": true}
	cell, _ := searchCell(t, f)
	cell(`choir.SearchStart("burst one")`)
	f.scope.searches.apply(providerFrame("s1", "keenable", true, pagesN("r", 1, 40)...))
	f.scope.searches.apply(SearchFrame{ID: "s1", Kind: SearchFrameDone})
	out := cell(`s, _ := choir.SearchStart("burst two")
fmt.Println(s.Refused)`)
	if f.searchCalls.Load() != 1 || !strings.Contains(out.Stdout, "already holds 40 pages") {
		t.Fatalf("calls %d:\n%s", f.searchCalls.Load(), out.Stdout)
	}
}

func TestFreshSendsWhileAnotherSearchRuns(t *testing.T) {
	f := newResearchFixture(t)
	f.runningFor = map[string]bool{"burst one": true}
	cell, _ := searchCell(t, f)
	cell(`choir.SearchStart("burst one")
choir.SearchStart("burst two", true)`)
	if f.searchCalls.Load() != 2 {
		t.Fatalf("fresh is the run's own statement that it wants the search: calls %d", f.searchCalls.Load())
	}
}

func TestSearchesFromManyGoroutinesAreJudgedOneAtATime(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{}
	for _, q := range []string{"par one", "par two", "par three", "par four", "par five"} {
		f.urlsFor[q] = pagesN(strings.ReplaceAll(q, " ", ""), 1, 40)
	}
	f.scope.BindCell().Begin(SessionFrame{RunID: "run-1"})
	var wg sync.WaitGroup
	for q := range f.urlsFor {
		wg.Add(1)
		go func(q string) {
			defer wg.Done()
			if _, err := f.scope.SearchStart(q); err != nil {
				t.Error(err)
			}
		}(q)
	}
	wg.Wait()
	if f.searchCalls.Load() != 1 {
		t.Fatalf("five searches at once sent %d, want 1", f.searchCalls.Load())
	}
}

func TestSearchWaitOfARefusedStubSaysItWasNotSent(t *testing.T) {
	f := newResearchFixture(t)
	cell := seedUnread(t, f, 40)
	out := cell(`s, _ := choir.SearchStart("plain two")
_, err := choir.SearchWait(s.ID, 10)
fmt.Println(err)`)
	if !strings.Contains(out.Stdout, "was not sent") {
		t.Fatalf("stdout:\n%s", out.Stdout)
	}
}

func TestTheSearchDocsNoLongerCallTheUnreadPagesANoteOnly(t *testing.T) {
	for _, verb := range []string{"SearchStart", "Search"} {
		doc := choirFunctionDocs[verb].Summary
		if strings.Contains(doc, "a note only") {
			t.Errorf("%s doc still says a note only: %s", verb, doc)
		}
		if !strings.Contains(doc, "not sent") || !strings.Contains(doc, "15") {
			t.Errorf("%s doc does not say a search is not sent while 15 or more unread pages are held", verb)
		}
	}
}

// From the review of the gate (seat 2, and the coordinator). Failure list,
// written before the code:
//  11. SearchAll for a question whose words are all small (an empty term key)
//      skips the need test and sends every wording.
//  12. A refused SearchStart called as a bare statement shows the cell nothing:
//      no line in the cell result, although a reuse refusal has one.
//  13. The refusal text does not say the held pages may be from any thread of
//      the run, not only this query's.
//  14. fresh_over_unread counts a search the host then refused or failed to
//      start: it must count only a search that was sent.

func TestSearchAllOfAQuestionWithNoTermsFallsBackToTheRunWideCount(t *testing.T) {
	if k := questionKey("!!! ???"); k != "" {
		t.Fatalf("the test needs a question with an empty key, got %q", k)
	}
	f := newResearchFixture(t)
	cell := seedUnread(t, f, 40)
	out := cell(`r, _ := choir.SearchAll("!!! ???", []string{"plain two", "plain three"})
fmt.Println(len(r.Results))
fmt.Println(r)`)
	if f.searchCalls.Load() != 1 {
		t.Fatalf("an empty question key must use the run-wide count; provider asked %d times:\n%s", f.searchCalls.Load(), out.Stdout)
	}
	// Run-wide, so the text says the run, as the plain verbs' refusal does; and
	// the held pages are in Results, which Next tells the run to read.
	for _, want := range []string{"not sent (this run already holds 40 pages new to this run", "40\n"} {
		if !strings.Contains(out.Stdout, want) {
			t.Fatalf("stdout lacks %q:\n%s", want, out.Stdout)
		}
	}
	if strings.Contains(out.Stdout, "this question already holds") {
		t.Fatalf("a run-wide count must not be called the question's:\n%s", out.Stdout)
	}
}

func TestARefusedBareSearchStartLeavesANoteInTheCellResult(t *testing.T) {
	f := newResearchFixture(t)
	cell := seedUnread(t, f, 40)
	out := cell(`choir.SearchStart("plain two")`)
	notes := strings.Join(out.Searches, "\n")
	for _, want := range []string{"not sent: plain two", "40 pages new to this run", "fresh"} {
		if !strings.Contains(notes, want) {
			t.Fatalf("cell result lacks %q:\n%s", want, notes)
		}
	}
}

func TestTheRefusalSaysTheHeldPagesMayBeFromAnyThread(t *testing.T) {
	f := newResearchFixture(t)
	cell := seedUnread(t, f, 40)
	out := cell(`s, _ := choir.SearchStart("plain two")
fmt.Println(s.Refused)`)
	if !strings.Contains(out.Stdout, "any thread of this run") {
		t.Fatalf("stdout:\n%s", out.Stdout)
	}
}

func TestFreshOverUnreadCountsOnlyASearchThatWasSent(t *testing.T) {
	f := newResearchFixture(t)
	f.hostErr = map[string]string{"plain two": "provider refused"}
	cell := seedUnread(t, f, 40)
	out := cell(`_, err := choir.SearchStart("plain two", true)
fmt.Println(err != nil)`)
	if !strings.Contains(out.Stdout, "true") {
		t.Fatalf("the host refusal must reach the cell:\n%s", out.Stdout)
	}
	if st := out.SearchStats; st != nil {
		t.Fatalf("stats = %+v, a search the host refused counts nothing (not started, not fresh_over_unread)", st)
	}
	// The same cell's next fresh search is sent, and it alone is counted.
	out = cell(`_, err := choir.SearchStart("plain three", true)
fmt.Println(err)`)
	if st := out.SearchStats; st == nil || st.FreshOverUnread != 1 || st.Started != 1 {
		t.Fatalf("stats = %+v, want exactly 1 fresh_over_unread and 1 started", st)
	}
}

// Follow-ups to the review of the running rule. Failure list, written first:
//  15. SearchAll sends a wording while another search of the run is running.
//  16. The refusal's example call is not valid Go (SearchWait(id, ms)).
//  17. A running search that has run two minutes still holds the next back.
//  18. A search of an earlier run still holds the next run's search back.

func TestSearchAllAppliesTheRunningRuleToEachWording(t *testing.T) {
	f := newResearchFixture(t)
	f.runningFor = map[string]bool{"burst one": true}
	cell, _ := searchCell(t, f)
	cell(`choir.SearchStart("burst one")`)
	out := cell(`r, err := choir.SearchAll("another question", []string{"burst two", "burst three"})
fmt.Println("err:", err)
for _, w := range r.Wordings { fmt.Println("wording", w.Wording, "sent:", w.Sent, "why:", w.Why) }
fmt.Println("next:", r.Next)
fmt.Println(r)`)
	if f.searchCalls.Load() != 1 {
		t.Fatalf("SearchAll sent a wording while a search of the run ran (calls %d):\n%s", f.searchCalls.Load(), out.Stdout)
	}
	// Not a failure: no error, no FAILED line, each wording named not sent with
	// the running search's id.
	for _, want := range []string{"err: <nil>", "wording burst two sent: false why: a search of this run is still running (s1)", "wording burst three sent: false why: a search of this run is still running (s1)", `choir.SearchWait("s1", 15000)`} {
		if !strings.Contains(out.Stdout, want) {
			t.Fatalf("stdout lacks %q:\n%s", want, out.Stdout)
		}
	}
	if strings.Contains(out.Stdout, "FAILED") {
		t.Fatalf("a running refusal is not a failure:\n%s", out.Stdout)
	}
	// One clear message, not the long refusal once per wording.
	if n := strings.Count(out.Stdout, "cannot be judged by the pages it will return"); n > 1 {
		t.Fatalf("the instruction is repeated %d times:\n%s", n, out.Stdout)
	}
	st := out.SearchStats
	if st == nil || st.NotSent["search_running"] != 2 {
		t.Fatalf("stats = %+v, want 2 search_running (one per wording)", st)
	}
	// Running refusals are counted apart: they are not wordings the call was
	// free to send, so the sent share stays honest.
	if st.AllOffered != 0 || st.AllRunning != 2 || st.AllSent != 0 {
		t.Fatalf("stats = %+v, want 0 offered, 2 running, 0 sent", st)
	}
}

// A refusal because another search runs carries that search's id in the value,
// so the instructed next step is reachable from it.
func TestARunningRefusalCarriesTheRunningSearchID(t *testing.T) {
	f := newResearchFixture(t)
	f.runningFor = map[string]bool{"burst one": true}
	cell, _ := searchCell(t, f)
	cell(`choir.SearchStart("burst one")`)
	out := cell(`s, _ := choir.SearchStart("burst two")
fmt.Println(s.ID, s.Refused != "")
c, err := choir.SearchWait(s.ID, 10)
fmt.Println(c.ID, err)
q, err := choir.Search("burst two")
fmt.Println(q.ID, q.Done, len(q.Results), q.Refused != "", err)`)
	if want := "s1 true\ns1 <nil>\ns1 false 0 true <nil>"; strings.TrimSpace(out.Stdout) != want {
		t.Fatalf("stdout:\n%s\nwant:\n%s", out.Stdout, want)
	}
	if f.searchCalls.Load() != 1 {
		t.Fatalf("calls %d", f.searchCalls.Load())
	}
}

// SearchWait of an id no search has says what a refused search's id is.
func TestSearchWaitOfNoIDSaysWhatToDo(t *testing.T) {
	f := newResearchFixture(t)
	cell, _ := searchCell(t, f)
	out := cell(`_, err := choir.SearchWait("", 10)
fmt.Println(err)`)
	if !strings.Contains(out.Stdout, "was not sent") || !strings.Contains(out.Stdout, "unread") || !strings.Contains(out.Stdout, "still running") {
		t.Fatalf("the error must name both refusals:\n%s", out.Stdout)
	}
}

// A finished identical search is answered although an unrelated search runs.
func TestAFinishedMatchIsReusedWhileAnUnrelatedSearchRuns(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"harbor strike": pagesN("h", 1, 5)}
	f.runningFor = map[string]bool{"other thing": true}
	cell, _ := searchCell(t, f)
	cell(`choir.SearchStart("harbor strike")`)
	cell(`choir.SearchStart("other thing", true)`)
	out := cell(`b, _ := choir.SearchStart("strike harbor")
fmt.Println(b.ID, b.Reused != "", b.Refused == "")`)
	if strings.TrimSpace(out.Stdout) != "s1 true true" || f.searchCalls.Load() != 2 {
		t.Fatalf("a finished match is reused even while another search runs (calls %d): %s", f.searchCalls.Load(), out.Stdout)
	}
}

func TestTheRunningRefusalExampleIsValidGo(t *testing.T) {
	why := runningRefusal("s7")
	if !strings.Contains(why, `choir.SearchWait("s7", 15000)`) {
		t.Fatalf("the example call must be valid Go with a number for ms: %s", why)
	}
	if strings.Contains(why, "ms)") {
		t.Fatalf("the refusal still shows a placeholder argument: %s", why)
	}
}

func TestARunningSearchOlderThanTwoMinutesNoLongerHoldsTheNextBack(t *testing.T) {
	f := newResearchFixture(t)
	clk := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	f.scope.searches.now = func() time.Time { return clk }
	f.runningFor = map[string]bool{"burst one": true}
	cell, _ := searchCell(t, f)
	cell(`choir.SearchStart("burst one")`)
	clk = clk.Add(searchWaitMax - time.Second)
	cell(`choir.SearchStart("burst two")`)
	if f.searchCalls.Load() != 1 {
		t.Fatalf("just under two minutes the running search still holds the next back (calls %d)", f.searchCalls.Load())
	}
	clk = clk.Add(2 * time.Second)
	cell(`choir.SearchStart("burst two")`)
	if f.searchCalls.Load() != 2 {
		t.Fatalf("after two minutes the host's search is not waited on; the next is sent (calls %d)", f.searchCalls.Load())
	}
}

func TestARunningSearchOfAnEarlierRunDoesNotHoldTheNextRunBack(t *testing.T) {
	f := newResearchFixture(t)
	f.runningFor = map[string]bool{"burst one": true}
	cell, hooks := searchCell(t, f)
	cell(`choir.SearchStart("burst one")`)
	hooks.Begin(SessionFrame{RunID: "run-2"})
	cell(`choir.SearchStart("burst two")`)
	if f.searchCalls.Load() != 2 {
		t.Fatalf("a search left running by the earlier run must not hold the new run back (calls %d)", f.searchCalls.Load())
	}
}
