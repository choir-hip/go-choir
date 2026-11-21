package agentcore

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/search"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The read-first rule's record and its search zero
// (docs/design/semantic-layer-2026-11-20.md section 4.3). Failure list,
// written before the code:
//  1. The host learns nothing of what the run held and had tried when it asked:
//     no search_decision event, or one that names the query, or one written
//     after the stream opened (so a search could count its own results as held).
//  2. A search's end carries no pages_returned.
//  3. SearchFresh's reason is not on the tape with the decision.
//  4. The pages the corpus brief delivered do not reach the run's cells: a first
//     web search goes out with the held pages untried.
//  5. A brief that delivered nothing hands the run a search zero.

// logSearchClient wraps a client and notes, in one ordered log, when each
// stream is opened.
type logSearchClient struct {
	inner search.Client
	log   func(string)
}

func (c logSearchClient) Stream(ctx context.Context, query string, maxResults int) (*search.Stream, error) {
	c.log("stream:" + query)
	return c.inner.Stream(ctx, query, maxResults)
}

type orderedLog struct {
	mu      sync.Mutex
	entries []string
	events  []map[string]any
}

func (l *orderedLog) add(s string) {
	l.mu.Lock()
	l.entries = append(l.entries, s)
	l.mu.Unlock()
}

func (l *orderedLog) tap(kind types.EventKind, payload map[string]any) {
	verb, _ := payload["verb"].(string)
	if kind != types.EventDeskEgress || (verb != "search_decision" && verb != "search_done") {
		return
	}
	l.mu.Lock()
	l.events = append(l.events, payload)
	l.entries = append(l.entries, "event:"+verb)
	l.mu.Unlock()
}

func (l *orderedLog) snapshot() ([]string, []map[string]any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.entries...), append([]map[string]any(nil), l.events...)
}

func TestSearchDecisionIsOnTheTapBeforeTheStreamAndTheResultAfter(t *testing.T) {
	var lg orderedLog
	client := logSearchClient{inner: manyPagesClient{pages: 6}, log: lg.add}
	tool, ctx, _, _ := researchSearchToolWith(t, client, func(rt *Runtime) { rt.deskEventTap = lg.tap })
	out := evalDeskCell(t, tool, ctx, `import "time"
s, _ := choir.Search("alpha query")
for i := 0; i < 5 && i < len(s.Results); i++ { choir.Read(s.Results[i].URL) }
t, _ := choir.Search("beta query")
fmt.Println(s.ID, t.ID, len(s.Results), t.Refused == "")
const why = "the first answer is about another harbor"
u, _ := choir.SearchFresh("gamma query", why)
fmt.Println(u.ID, u.Refused == "")
time.Sleep(300 * time.Millisecond)`)
	if out.Error != "" {
		t.Fatalf("cell error: %s", out.Error)
	}
	if !strings.Contains(out.Stdout, "s1 s2 6 true") || !strings.Contains(out.Stdout, "s3 true") {
		t.Fatalf("stdout:\n%s", out.Stdout)
	}
	entries, events := lg.snapshot()
	pos := func(s string, from int) int {
		for i := from; i < len(entries); i++ {
			if entries[i] == s {
				return i
			}
		}
		return -1
	}
	d1 := pos("event:search_decision", 0)
	s1 := pos("stream:alpha query", 0)
	e1 := pos("event:search_done", 0)
	if d1 < 0 || s1 < 0 || e1 < 0 || !(d1 < s1 && s1 < e1) {
		t.Fatalf("the first decision must be written before its stream opens and its result after: %v", entries)
	}
	d2 := pos("event:search_decision", d1+1)
	s2 := pos("stream:beta query", 0)
	if d2 < 0 || s2 < 0 || !(d2 < s2) {
		t.Fatalf("the second decision must be written before its stream opens: %v", entries)
	}
	var decisions []map[string]any
	for _, ev := range events {
		if ev["verb"] == "search_decision" {
			decisions = append(decisions, ev)
		}
	}
	if len(decisions) != 3 {
		t.Fatalf("%d decisions, want 3: %v", len(decisions), events)
	}
	num := func(m map[string]any, k string) int {
		switch v := m[k].(type) {
		case int:
			return v
		case float64:
			return int(v)
		}
		return -1
	}
	// The first search asked with nothing held; the second with the first's
	// 6 pages returned and the 5 read; a search never counts its own results.
	if d := decisions[0]; num(d, "earlier_searches") != 0 || num(d, "pages_returned") != 0 || num(d, "pages_tried") != 0 || d["search_verb"] != "search" {
		t.Fatalf("first decision counted something held: %v", d)
	}
	if d := decisions[1]; num(d, "earlier_searches") != 1 || num(d, "pages_returned") != 6 || num(d, "pages_tried") != 5 {
		t.Fatalf("second decision: %v", d)
	}
	d3 := decisions[2]
	if d3["fresh_why"] != "the first answer is about another harbor" || d3["search_verb"] != "search_fresh" {
		t.Fatalf("SearchFresh's reason is not on the tape: %v", d3)
	}
	for _, d := range decisions {
		for k, v := range d {
			if s, ok := v.(string); ok && strings.Contains(s, "alpha query") || k == "query" {
				t.Fatalf("a decision names the query: %v", d)
			}
		}
	}
	// The end of a search carries what it returned.
	var done map[string]any
	for _, ev := range events {
		if ev["verb"] == "search_done" {
			done = ev
			break
		}
	}
	if done == nil || num(done, "pages_returned") != 6 {
		t.Fatalf("search_done lacks pages_returned: %v", done)
	}
}

func TestThePagesTheBriefDeliveredAreSearchZeroForTheRunsCells(t *testing.T) {
	var held []yaegikernel.HeldPage
	for i := 1; i <= 6; i++ {
		held = append(held, yaegikernel.HeldPage{URL: fmt.Sprintf("https://held.example/p%d", i), CaptureID: fmt.Sprintf("cap-%d", i)})
	}
	var lg orderedLog
	var rtp *Runtime
	client := logSearchClient{inner: manyPagesClient{pages: 3}, log: lg.add}
	tool, ctx, _, _ := researchSearchToolWith(t, client, func(rt *Runtime) {
		rtp = rt
		rt.deskEventTap = lg.tap
		rt.noteBriefHeld("run-desk-eval", held)
	})
	out := evalDeskCell(t, tool, ctx, `s, _ := choir.Search("alpha query")
fmt.Println(s.ID == "", s.Refused != "", len(s.Results))
for _, h := range s.Results { fmt.Println(h.URL) }`)
	if out.Error != "" {
		t.Fatalf("cell error: %s", out.Error)
	}
	if !strings.Contains(out.Stdout, "true true 5") || !strings.Contains(out.Stdout, "https://held.example/p1") || strings.Contains(out.Stdout, "held.example/p6") {
		t.Fatalf("a first search was not held back for the brief's top five pages:\n%s", out.Stdout)
	}
	if entries, _ := lg.snapshot(); len(entries) != 0 {
		t.Fatalf("a refused search reached the host: %v", entries)
	}
	if line, ok := rtp.takeRunSearchSummary("run-desk-eval"); !ok || !strings.Contains(line, "held_first=1") {
		t.Fatalf("the run line does not count the refusal by its reason: %q", line)
	}
	out = evalDeskCell(t, tool, ctx, `for i := 1; i <= 5; i++ { choir.Read(fmt.Sprintf("https://held.example/p%d", i)) }
s, _ := choir.Search("alpha query")
fmt.Println(s.ID, s.Refused == "")`)
	if !strings.Contains(out.Stdout, "s1 true") {
		t.Fatalf("with the held pages tried the search must be sent:\n%s", out.Stdout)
	}
	time.Sleep(50 * time.Millisecond)
	_, events := lg.snapshot()
	for _, ev := range events {
		if ev["verb"] == "search_decision" {
			if h, _ := ev["held_brief"].(int); h != 6 {
				if f, _ := ev["held_brief"].(float64); int(f) != 6 {
					t.Fatalf("the decision does not report the 6 held pages: %v", ev)
				}
			}
		}
	}
}

func TestARunWhoseBriefDeliveredNothingHasNoSearchZero(t *testing.T) {
	rt := &Runtime{}
	rt.noteBriefHeld("run-1", nil)
	if got := rt.briefHeldFor("run-1"); got != nil {
		t.Fatalf("an empty brief handed over %v", got)
	}
	rt.noteBriefHeld("run-1", []yaegikernel.HeldPage{{URL: "https://a.example/x"}})
	if len(rt.briefHeldFor("run-1")) != 1 {
		t.Fatal("the delivered pages did not reach the run")
	}
	if rt.briefHeldFor("run-2") != nil || rt.briefHeldFor("") != nil {
		t.Fatal("another run, or none, got the pages")
	}
}
