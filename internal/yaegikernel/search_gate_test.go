package yaegikernel

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// What holds a web search now (owner, 2026-11-22: "we shouldnt need rules"): a search of
// the same terms as a finished one of the run is answered from it, one search of a run runs
// at a time, and a query with no real word is refused. NOTHING is refused for pages the run
// holds untried: the run is given the paper's graph and told to use it first
// (research_digest.go in agentcore). The tests below keep the running rule and the removal.

func TestNoSearchIsRefusedForUntriedPages(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"one": pagesN("a", 1, 40), "two": pagesN("b", 1, 40), "three": pagesN("c", 1, 40)}
	cell, _ := searchCell(t, f)
	// Send, read nothing, send a different query, again: all sent, none refused.
	out := cell(`a, _ := choir.Search("one")
b, _ := choir.Search("two")
c, _ := choir.SearchStart("three")
d, _ := choir.SearchAll("a fourth question", []string{"fourth wording alpha"})
fmt.Println(a.Refused == "", b.Refused == "", c.Refused == "", len(d.Wordings), d.Wordings[0].Sent)`)
	if f.searchCalls.Load() != 4 || !strings.Contains(out.Stdout, "true true true 1 true") {
		t.Fatalf("a search was refused for untried pages (sent %d):\n%s", f.searchCalls.Load(), out.Stdout)
	}
	if st := out.SearchStats; st == nil || len(st.NotSent) != 0 {
		t.Fatalf("stats = %+v, want nothing not sent", st)
	}
	// The same-terms answer and the running rule still hold.
	out = cell(`s, _ := choir.Search("one")
fmt.Println(s.ID, s.Reused != "")`)
	if f.searchCalls.Load() != 4 || !strings.Contains(out.Stdout, "s1 true") {
		t.Fatalf("a repeat of an earlier question must be answered from it (sent %d):\n%s", f.searchCalls.Load(), out.Stdout)
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
	for _, want := range []string{"<nil> true true", "a search of this run is still running (s1)", `choir.SearchWait("s1", 15000)`} {
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
	cell(readTop(5, "r"))
	cell(`choir.SearchStart("burst two")`)
	if f.searchCalls.Load() != 2 {
		t.Fatalf("after the first ended and its pages were read, the search must be sent (calls %d)", f.searchCalls.Load())
	}
}

func TestSearchesFromManyGoroutinesAreJudgedOneAtATime(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{}
	f.runningFor = map[string]bool{}
	for _, q := range []string{"par one", "par two", "par three", "par four", "par five"} {
		f.urlsFor[q] = pagesN(strings.ReplaceAll(q, " ", ""), 1, 40)
		f.runningFor[q] = true // the first stays running: the others are refused at once
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
	if !strings.Contains(out.Stdout, "was not sent") || !strings.Contains(out.Stdout, "still running") {
		t.Fatalf("the error must say a refused search has no id:\n%s", out.Stdout)
	}
}

// A finished identical search is answered although an unrelated search runs.
func TestAFinishedMatchIsReusedWhileAnUnrelatedSearchRuns(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"harbor strike": pagesN("h", 1, 5)}
	f.runningFor = map[string]bool{"other thing": true}
	cell, _ := searchCell(t, f)
	cell(`choir.SearchStart("harbor strike")`)
	cell(readTop(5, "h"))
	if _, err := f.scope.startSearch("other thing", searchOpts{verb: "search"}); err != nil {
		t.Fatal(err)
	}
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
