package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/search"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The read-first rule's record and its search zero
// (docs/design/semantic-layer-2026-11-20.md section 4.3). Failure list,
// written before the code:
//  1. The host learns nothing of what the run held and had tried when it asked:
//     no decision on the search's row, or one that names the query, or one that
//     counts the search's own results as held. (Design 4.3 asked for a
//     standalone search.decision event before the stream; the journal cut R4
//     keeps one durable record per cell, so the decision rides the search_start
//     row, taken from the payload before the stream opens.)
//  2. A search's end carries no pages_returned.
//  3. (SearchFresh and its reason were removed 2026-11-22.)
//  4. The pages the corpus brief delivered do not reach the run's cells: a first
//     web search goes out with the held pages untried.
//  5. A brief that delivered nothing hands the run a search zero.

// decisionRig is a research desk tool over a real store, so the events a run
// writes can be read back.
func decisionRig(t *testing.T, client search.Client, configure func(rt *Runtime)) (toolregistry.Tool, context.Context, string, func() ([]map[string]any, []map[string]any)) {
	t.Helper()
	deskTestWorkerBin(t)
	rt, s := testRuntime(t)
	rt.researchDeps = &researchtools.Dependencies{Search: client}
	if configure != nil {
		configure(rt)
	}
	workers := newDeskSessionWorkers()
	t.Cleanup(workers.closeAll)
	tool := newDeskGoEvalTool(rt, workers, agentprofile.Research)
	execCtx := deskEvalExecCtx(t)
	execCtx.Profile = agentprofile.Research
	execCtx.Role = agentprofile.Research
	execCtx.RunID = "run-desk-eval"
	execCtx.RunRecord = &types.RunRecord{RunID: execCtx.RunID, AgentID: "research:decision", OwnerID: execCtx.OwnerID, TrajectoryID: "traj-decision"}
	ctx := toolregistry.WithExecutionContext(context.Background(), execCtx)
	rows := func() (decisions, dones []map[string]any) {
		evs, err := s.ListEvents(context.Background(), execCtx.RunID, 200)
		if err != nil {
			t.Fatalf("list events: %v", err)
		}
		for _, ev := range evs {
			var p map[string]any
			_ = json.Unmarshal(ev.Payload, &p)
			switch ev.Kind {
			case types.EventDeskCell:
				cellRows, _ := p["egress"].([]any)
				for _, r := range cellRows {
					row, _ := r.(map[string]any)
					if d, ok := row["decision"].(map[string]any); ok {
						decisions = append(decisions, d)
					}
				}
			case types.EventDeskEgress:
				if p["verb"] == "search_done" {
					dones = append(dones, p)
				}
			}
		}
		return decisions, dones
	}
	return tool, ctx, execCtx.RunID, rows
}

func decNum(m map[string]any, k string) int {
	switch v := m[k].(type) {
	case int:
		return v
	case float64:
		return int(v)
	}
	return -1
}

func TestSearchDecisionRidesTheSearchRowAndNeverCountsItsOwnResults(t *testing.T) {
	tool, ctx, _, rows := decisionRig(t, manyPagesClient{pages: 6}, nil)
	out := evalDeskCell(t, tool, ctx, `import "time"
s, _ := choir.Search("alpha query")
for i := 0; i < 5 && i < len(s.Results); i++ { choir.Read(s.Results[i].URL) }
t, _ := choir.Search("beta query")
fmt.Println(s.ID, t.ID, len(s.Results), t.Refused == "")
time.Sleep(300 * time.Millisecond)`)
	if out.Error != "" {
		t.Fatalf("cell error: %s", out.Error)
	}
	if !strings.Contains(out.Stdout, "s1 s2 6 true") {
		t.Fatalf("stdout:\n%s", out.Stdout)
	}
	decisions, dones := rows()
	if len(decisions) != 2 {
		t.Fatalf("%d decisions, want 2: %v", len(decisions), decisions)
	}
	// The first search asked with nothing held although it returned 6 pages: a
	// search never counts its own results. The second asked with the first's 6
	// pages returned and the 5 read.
	if d := decisions[0]; decNum(d, "earlier_searches") != 0 || decNum(d, "earlier_pages_new") != 0 || decNum(d, "pages_tried") != 0 || d["search_verb"] != "search" {
		t.Fatalf("first decision counted something held: %v", d)
	}
	if d := decisions[1]; decNum(d, "earlier_searches") != 1 || decNum(d, "earlier_pages_new") != 6 || decNum(d, "pages_tried") != 5 {
		t.Fatalf("second decision: %v", d)
	}
	for _, d := range decisions {
		for k, v := range d {
			if sv, ok := v.(string); ok && strings.Contains(sv, "alpha query") || k == "query" {
				t.Fatalf("a decision names the query: %v", d)
			}
		}
	}
	if len(dones) < 1 || decNum(dones[0], "pages_returned") != 6 {
		t.Fatalf("search_done lacks pages_returned: %v", dones)
	}
}

type hookClient struct {
	inner search.Client
	after func(query string)
}

func (c hookClient) Stream(ctx context.Context, query string, maxResults int) (*search.Stream, error) {
	if c.after != nil {
		c.after(query)
	}
	return c.inner.Stream(ctx, query, maxResults)
}

func TestPagesReturnedCountsAPageTwoProvidersReturnedOnce(t *testing.T) {
	d := searchDone{
		providers: []types.SearchServedUsage{{Provider: "a", Results: 3}, {Provider: "b", Results: 3}},
		urls:      []string{"https://x.example/1", "https://x.example/2", "https://x.example/3", "http://www.x.example/1/", "https://x.example/2", "https://y.example/9"},
	}
	if got := d.pagesReturned(); got != 4 {
		t.Fatalf("pagesReturned = %d, want 4 distinct pages", got)
	}
}

// The read-first refusals are gone (owner, 2026-11-22): a run that holds untried pages is
// not refused its next web search; it is told, by the prompt and the digest, to use the
// graph first.
func TestAWebSearchWithUntriedPagesIsNotRefused(t *testing.T) {
	tool, ctx, _, rows := decisionRig(t, manyPagesClient{pages: 40}, nil)
	out := evalDeskCell(t, tool, ctx, `import "time"
s, _ := choir.Search("alpha query")
t, _ := choir.Search("beta query")
u, _ := choir.SearchStart("gamma query")
fmt.Println(s.ID, t.ID, u.ID, s.Refused == "", t.Refused == "", u.Refused == "", len(s.Results))
time.Sleep(300 * time.Millisecond)`)
	if out.Error != "" || !strings.Contains(out.Stdout, "s1 s2 s3 true true true 40") {
		t.Fatalf("a search with 40 untried pages held was refused: %q %q", out.Error, out.Stdout)
	}
	if decisions, _ := rows(); len(decisions) != 3 {
		t.Fatalf("%d decisions, want 3", len(decisions))
	}
}

// A research run that holds the story document's text carries owner_private:document
// (the digest's label). What the egress gate then does to its web search is the existing
// rule, not weakened here: the paper's own story documents are public-bound, and the
// run's search goes out as any run's that was shown its story document does.
func TestAWebSearchOfARunHoldingTheStoryDocumentsLabelIsNotRefused(t *testing.T) {
	tool, ctx, _, _ := decisionRig(t, manyPagesClient{pages: 6}, func(r *Runtime) { r.taintRun("run-desk-eval", documentReadLabels) })
	out := evalDeskCell(t, tool, ctx, `s, err := choir.Search("alpha query")
fmt.Println(err, s.ID, s.Refused == "", len(s.Results))`)
	t.Logf("search under owner_private:document: error=%q stdout=%q", out.Error, out.Stdout)
	if out.Error != "" || !strings.Contains(out.Stdout, "true 6") {
		t.Fatalf("the web search of a run tainted with the story document's label did not go out: %q %q", out.Error, out.Stdout)
	}
}
