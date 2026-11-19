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
//  7. A burst whose first search is still running is judged before its pages
//     arrive (the rest must wait for them).
//  8. Searches started from several goroutines at once are all sent.
//  9. SearchWait of a refused stub's empty id fails with "no search": it must
//     say the search was not sent.
// 10. The SearchStart and Search surface docs still say the unread pages are
//     "a note only; the search is sent".
//
// The burst rule: one search is sent at a time per cell scope. Before each
// search is judged, the run's searches still running are waited on (at most
// searchWaitMs in all, as Search waits), so the second search of a burst
// is judged against the pages the first returned; a burst of five whose first
// returns 15 or more unread pages sends one.

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

func TestABurstWaitsForTheRunningFirstSearchBeforeJudgingTheRest(t *testing.T) {
	f := newResearchFixture(t)
	f.runningFor = map[string]bool{"burst one": true}
	cell, _ := searchCell(t, f)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for { // the start frame of s1 arrives with the first call
			if _, err := f.scope.searches.Get("s1"); err == nil {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		time.Sleep(150 * time.Millisecond)
		f.scope.searches.apply(providerFrame("s1", "keenable", true, pagesN("r", 1, 40)...))
		f.scope.searches.apply(SearchFrame{ID: "s1", Kind: SearchFrameDone})
	}()
	out := cell(`choir.SearchStart("burst one")
s, _ := choir.SearchStart("burst two")
fmt.Println(s.ID == "", s.Refused != "")`)
	<-done
	if f.searchCalls.Load() != 1 || !strings.Contains(out.Stdout, "true true") {
		t.Fatalf("the second search must wait for the first's pages and be refused (calls %d):\n%s", f.searchCalls.Load(), out.Stdout)
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
