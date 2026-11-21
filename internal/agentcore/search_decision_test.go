package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/search"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
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
//  3. SearchFresh's reason is not on the tape with the decision.
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
	var logged strings.Builder
	prev := log.Writer()
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(prev) })
	tool, ctx, _, rows := decisionRig(t, manyPagesClient{pages: 6}, nil)
	out := evalDeskCell(t, tool, ctx, `import "time"
s, _ := choir.Search("alpha query")
for i := 0; i < 5 && i < len(s.Results); i++ { choir.Read(s.Results[i].URL) }
t, _ := choir.Search("beta query")
fmt.Println(s.ID, t.ID, len(s.Results), t.Refused == "")
u, _ := choir.SearchFresh("gamma query", "the first answer is about another harbor")
fmt.Println(u.ID, u.Refused == "")
time.Sleep(300 * time.Millisecond)`)
	if out.Error != "" {
		t.Fatalf("cell error: %s", out.Error)
	}
	if !strings.Contains(out.Stdout, "s1 s2 6 true") || !strings.Contains(out.Stdout, "s3 true") {
		t.Fatalf("stdout:\n%s", out.Stdout)
	}
	decisions, dones := rows()
	if len(decisions) != 3 {
		t.Fatalf("%d decisions, want 3: %v", len(decisions), decisions)
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
	if d := decisions[2]; d["fresh_why"] != "the first answer is about another harbor" || d["search_verb"] != "search_fresh" {
		t.Fatalf("SearchFresh's reason is not on the tape: %v", d)
	}
	for _, d := range decisions {
		for k, v := range d {
			if sv, ok := v.(string); ok && strings.Contains(sv, "alpha query") || k == "query" {
				t.Fatalf("a decision names the query: %v", d)
			}
		}
	}
	if !strings.Contains(logged.String(), `runtime: search fresh run=run-desk-eval why="the first answer is about another harbor"`) || strings.Count(logged.String(), "runtime: search fresh") != 1 {
		t.Fatalf("the SearchFresh reason is not on the console, once: %q", logged.String())
	}
	if len(dones) < 1 || decNum(dones[0], "pages_returned") != 6 {
		t.Fatalf("search_done lacks pages_returned: %v", dones)
	}
}

func TestThePagesTheBriefDeliveredAreSearchZeroForTheRunsCells(t *testing.T) {
	var held []yaegikernel.HeldPage
	for i := 1; i <= 6; i++ {
		held = append(held, yaegikernel.HeldPage{URL: fmt.Sprintf("https://held.example/p%d", i), CaptureID: fmt.Sprintf("cap-%d", i)})
	}
	var rtp *Runtime
	tool, ctx, _, rows := decisionRig(t, manyPagesClient{pages: 3}, func(rt *Runtime) {
		rtp = rt
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
	if decisions, _ := rows(); len(decisions) != 0 {
		t.Fatalf("a refused search reached the host: %v", decisions)
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
	decisions, _ := rows()
	if len(decisions) != 1 || decNum(decisions[0], "held_brief") != 6 || decNum(decisions[0], "held_tried") != 5 {
		t.Fatalf("the decision does not report the 6 held pages, 5 tried: %v", decisions)
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

func TestTheTreeKeepsWhatAnySessionTriedAndWasReturned(t *testing.T) {
	rt := &Runtime{}
	if rt.treeStateFor("run-1") != nil {
		t.Fatal("an untouched tree has a state")
	}
	var urls []string
	for i := 1; i <= 8; i++ {
		urls = append(urls, fmt.Sprintf("https://a.example/p%d", i))
	}
	rt.noteTreeSearch("run-1", urls)
	// A second search of another session returns 3 old and 7 new pages: the
	// window takes the first five new to the tree.
	var more []string
	for i := 6; i <= 15; i++ {
		more = append(more, fmt.Sprintf("https://a.example/p%d", i))
	}
	rt.noteTreeSearch("run-1", more)
	st := rt.treeStateFor("run-1")
	if st == nil || len(st.Window) != 10 || st.Window[0].URL != "https://a.example/p1" || st.Window[5].URL != "https://a.example/p9" {
		t.Fatalf("window = %+v", st)
	}
	// Reads of any session: a Read by url, a ReadCapture of a held page by id.
	rt.noteBriefHeld("run-1", []yaegikernel.HeldPage{{URL: "https://h.example/x", CaptureID: "cap-9"}})
	rt.noteTreeRead("run-1", yaegikernel.ActionRead, json.RawMessage(`{"url":"http://www.a.example/p1/"}`))
	rt.noteTreeRead("run-1", yaegikernel.ActionReadCapture, json.RawMessage(`{"capture_id":"cap-9"}`))
	rt.noteTreeRead("run-1", yaegikernel.ActionReadCapture, json.RawMessage(`{"capture_id":"cap-unknown"}`))
	st = rt.treeStateFor("run-1")
	got := strings.Join(st.Tried, " ")
	if got != "https://a.example/p1 https://h.example/x" {
		t.Fatalf("tried = %q", got)
	}
	if rt.treeStateFor("run-2") != nil || rt.treeStateFor("") != nil {
		t.Fatal("another run, or none, got the state")
	}
}
