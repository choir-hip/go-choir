package agentcore

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/search"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Attribution and the per-run summary line (search-reuse commit 1,
// docs/problems/search-volume-outruns-keenable-credits-2026-11-18.md). Failure
// list, written before the code:
//  1. The gateway client is handed a context with no run, desk or
//     conversation, so the gateway cannot log whose search it is.
//  2. The desk's per-cell counts are dropped at the host, so the end of a
//     research run has nothing to summarise.
//  3. The summary line is logged twice for one run, or for a run that
//     searched nothing, or carries text (a query, a URL) and not counts.
//  4. A run's counts are kept after its line, leaking one entry per run.

type attributionClient struct {
	*pipeSearchClient
	got chan search.Attribution
}

func (c attributionClient) Stream(ctx context.Context, query string, maxResults int) (*search.Stream, error) {
	c.got <- search.AttributionFrom(ctx)
	return c.pipeSearchClient.Stream(ctx, query, maxResults)
}

func TestDeskSearchStartCarriesTheRunDeskAndConversation(t *testing.T) {
	client := attributionClient{newPipeSearchClient("fast"), make(chan search.Attribution, 4)}
	var rt *Runtime
	tool, ctx, _, _ := researchSearchToolWith(t, client, func(r *Runtime) { rt = r })
	exec := toolregistry.ExecutionContextFrom(ctx)
	exec.ChannelID = "chan-7"
	ctx = toolregistry.WithExecutionContext(ctx, exec)
	go func() {
		q := <-client.opened
		client.provider(t, q, "fast")
		client.done(t, q, 1)
	}()
	out := evalDeskCell(t, tool, ctx, `s, _ := choir.SearchStart("alpha"); choir.SearchWait(s.ID, 5000); fmt.Println("ok")`)
	if out.Error != "" {
		t.Fatalf("cell error: %s", out.Error)
	}
	a := <-client.got
	if a.RunID != exec.RunID || a.Desk != "research" || a.Conversation != "chan-7" {
		t.Fatalf("attribution = %+v, want run %q desk research conversation chan-7", a, exec.RunID)
	}
	// The cell's counts reached the host for this run.
	line, ok := rt.takeRunSearchSummary(exec.RunID)
	if !ok || !strings.Contains(line, "run="+exec.RunID+" desk=research started=1 ") || !strings.Contains(line, "pages_returned=1 pages_new=1") {
		t.Fatalf("run summary = %q (%v)", line, ok)
	}
	if _, again := rt.takeRunSearchSummary(exec.RunID); again {
		t.Fatal("a run's summary is logged once, then forgotten")
	}
}

func TestRunSearchSummaryLineIsCountsOnly(t *testing.T) {
	rt := &Runtime{}
	if _, ok := rt.takeRunSearchSummary("run-none"); ok {
		t.Fatal("a run that searched nothing must log nothing")
	}
	rt.noteRunSearchStats("run-9", "research", yaegikernel.SearchStats{Started: 3, FromHeld: 1, NotSent: map[string]int{"covered": 2}, PagesReturned: 100, PagesNew: 40, PagesRead: 6, AllCalls: 2, AllOffered: 9, AllSent: 4, Untagged: 2, SeenJudged: 2, SeenMostly: 1, OutletsMax: 5, OutletsLast: 5, OutletsSends: 1, ReuseDeclined: map[string]int{"out_of_window": 2}})
	rt.noteRunSearchStats("run-9", "research", yaegikernel.SearchStats{Started: 1, NotSent: map[string]int{"covered": 1, "holds_unread": 4, "stopped_yielding": 2}, PagesReturned: 20, PagesNew: 2, PagesRead: 1, AllCalls: 1, AllOffered: 3, AllSent: 1, Untagged: 1, SeenJudged: 1, OutletsMax: 4, OutletsLast: 4, OutletsSends: 1, ReuseDeclined: map[string]int{"out_of_window": 1}})
	// Two children of the run's tree, one of which searched on the research_web desk.
	rt.noteRunChild("run-9")
	rt.noteRunChild("run-9")
	rt.noteRunSearchStats("run-9", "research_web", yaegikernel.SearchStats{Started: 2, PagesReturned: 5, PagesNew: 5})
	line, ok := rt.takeRunSearchSummary("run-9")
	want := "search run=run-9 desk=research started=6 from_held=1 not_sent=9 (covered=3 holds_unread=4 stopped_yielding=2) children=2 started_research=4 started_research_web=2 search_all_calls=3 wordings_offered=12 wordings_sent=5 untagged=3 wordings_running=0 pages_returned=125 pages_new=47 pages_read=7 searches_judged=3 searches_mostly_seen=1 outlets_held_max=5 outlets_held_last=4 reuse_declined_out_of_window=3"
	if !ok || line != want {
		t.Fatalf("line = %q, want %q", line, want)
	}
}

// A run a planned restart resumed says its counts are this segment's only.
func TestResumedRunSearchSummaryIsFlagged(t *testing.T) {
	rt := &Runtime{}
	rt.runSearch.Store("run-r", &runSearchTotals{desk: "research", resumed: true, stats: yaegikernel.SearchStats{Started: 14}})
	line, ok := rt.takeRunSearchSummary("run-r")
	if !ok || !strings.HasSuffix(line, " resumed_segment=1") {
		t.Fatalf("line = %q (%v), want a resumed_segment=1 flag", line, ok)
	}
}
