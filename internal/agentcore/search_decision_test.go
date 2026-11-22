package agentcore

import (
	"context"
	"crypto/sha256"
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
	sum := sha256.Sum256([]byte("the first answer is about another harbor"))
	if want := fmt.Sprintf("runtime: search fresh run=run-desk-eval why_len=40 why_hash=%x", sum[:4]); !strings.Contains(logged.String(), want) || strings.Count(logged.String(), "runtime: search fresh") != 1 {
		t.Fatalf("the SearchFresh reason's length and hash are not on the console, once: %q", logged.String())
	}
	if strings.Contains(logged.String(), "another harbor") {
		t.Fatalf("the reason's text is on the console: %q", logged.String())
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

func TestTheTreeKeepsWhatAnySessionTried(t *testing.T) {
	rt := &Runtime{}
	rt.openTree("run-1")
	if rt.treeStateFor("run-1") != nil {
		t.Fatal("an untouched tree has a state")
	}
	// Reads of any session: a Read by url, a ReadCapture of a held page by id.
	rt.noteBriefHeld("run-1", []yaegikernel.HeldPage{{URL: "https://h.example/x", CaptureID: "cap-9"}})
	rt.noteTreeRead("run-1", yaegikernel.ActionRead, json.RawMessage(`{"url":"http://www.a.example/p1/"}`))
	rt.noteTreeRead("run-1", yaegikernel.ActionReadCapture, json.RawMessage(`{"capture_id":"cap-9"}`))
	rt.noteTreeRead("run-1", yaegikernel.ActionReadCapture, json.RawMessage(`{"capture_id":"cap-unknown"}`))
	st := rt.treeStateFor("run-1")
	got := strings.Join(st.Tried, " ")
	if got != "https://a.example/p1 https://h.example/x" {
		t.Fatalf("tried = %q", got)
	}
	if rt.treeStateFor("run-2") != nil || rt.treeStateFor("") != nil {
		t.Fatal("another run, or none, got the state")
	}
}

// A note that lands after the run's terminal observer closed the tree (a
// search_done of a stream that outlived the run) is dropped: the tree is not
// recreated, so it is not held for ever.
func TestANoteAfterTheRunEndedDoesNotRecreateTheTree(t *testing.T) {
	rt := &Runtime{}
	rt.noteTreeRead("run-1", yaegikernel.ActionRead, json.RawMessage(`{"url":"https://a.example/p1"}`))
	if _, ok := rt.treeRead.Load("run-1"); ok {
		t.Fatal("a note before any cell opened a tree")
	}
	rt.openTree("run-1")
	rt.noteTreeTried("run-1", "https://a.example/p1")
	if rt.treeStateFor("run-1") == nil {
		t.Fatal("an open tree dropped a note")
	}
	rt.closeTree("run-1")
	rt.noteTreeTried("run-1", "https://a.example/p2")
	if _, ok := rt.treeRead.Load("run-1"); ok {
		t.Fatal("a late note recreated the closed tree")
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

// A sibling's read is seen within a cell: the five pages the first search
// returned are tried by a sibling while the cell runs, and the second search is
// sent with no read by this cell, because the guest asked the host for the live
// tree before it refused.
func TestASiblingsReadWhileTheCellRunsReleasesTheNextSearch(t *testing.T) {
	var rt *Runtime
	client := hookClient{inner: manyPagesClient{pages: 6}, after: func(q string) {
		if q != "alpha query" || rt == nil {
			return
		}
		for i := 1; i <= 5; i++ {
			rt.noteTreeTried("run-desk-eval", fmt.Sprintf("https://seam.example/alpha-query/%d", i))
		}
	}}
	tool, ctx, _, rows := decisionRig(t, client, func(r *Runtime) { rt = r })
	out := evalDeskCell(t, tool, ctx, `import "time"
s, _ := choir.Search("alpha query")
time.Sleep(300 * time.Millisecond)
u, _ := choir.Search("beta query")
fmt.Println(s.ID, u.ID, u.Refused == "")`)
	if out.Error != "" || !strings.Contains(out.Stdout, "s1 s2 true") {
		t.Fatalf("the second search was refused for pages a sibling tried (err %q):\n%s", out.Error, out.Stdout)
	}
	decisions, _ := rows()
	if len(decisions) != 2 {
		t.Fatalf("%d decisions, want 2 (a probe is no search): %v", len(decisions), decisions)
	}
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

// The per-tree gate end to end. The tree shares what has been TRIED, not what has
// been returned: a child (a session of its own, with no RunRecord) is not held by
// the parent's search pages, so its first search is sent; its second search is
// held by its OWN first five until it, or anyone in the tree, tries them.
func TestAChildIsNotHeldByTheParentsPagesButByItsOwnAndASiblingsReadReleasesThem(t *testing.T) {
	var rt *Runtime
	tool, ctx, _, _ := decisionRig(t, manyPagesClient{pages: 8}, func(r *Runtime) { rt = r })
	parent := evalDeskCell(t, tool, ctx, `s, _ := choir.Search("alpha query")
fmt.Println(s.ID, len(s.Results))`)
	if parent.Error != "" || !strings.Contains(parent.Stdout, "s1 8") {
		t.Fatalf("parent cell: %q %q", parent.Error, parent.Stdout)
	}
	childExec := toolregistry.ExecutionContextFrom(ctx)
	// A session of its own that keeps its worker across cells (a real child does, under
	// its child key); the brief is not in play here.
	rec := *childExec.RunRecord
	rec.AgentID = "research:child"
	childExec.RunRecord = &rec
	childCtx := toolregistry.WithExecutionContext(ctx, childExec)
	child := evalDeskCell(t, tool, childCtx, `s, _ := choir.Search("child question")
fmt.Println(s.ID, s.Refused == "")`)
	if child.Error != "" || !strings.Contains(child.Stdout, "s1 true") {
		t.Fatalf("the child was held by the parent's pages: %q %q", child.Error, child.Stdout)
	}
	// Its second search is held by its own first five.
	child = evalDeskCell(t, tool, childCtx, `s, _ := choir.Search("child second question")
fmt.Println(s.ID == "", len(s.Results), s.Refused != "")`)
	if child.Error != "" || !strings.Contains(child.Stdout, "true 5 true") {
		t.Fatalf("the child's second search was not held by its own first five: %q %q", child.Error, child.Stdout)
	}
	// A sibling (anyone in the tree) tries them: the next search is sent.
	for i := 1; i <= 5; i++ {
		rt.noteTreeTried("run-desk-eval", fmt.Sprintf("https://seam.example/child-question/%d", i))
	}
	child = evalDeskCell(t, tool, childCtx, `s, _ := choir.Search("child second question")
fmt.Println(s.ID == "", s.Refused == "")`)
	if child.Error != "" || !strings.Contains(child.Stdout, "false true") {
		t.Fatalf("pages the tree tried did not release the child's search: %q %q", child.Error, child.Stdout)
	}
}

// The brief's pages hold the root session only: its text is delivered to the root
// run, a child is handed an objective and nothing of it.
func TestTheBriefsPagesHoldTheRootSessionNotAChild(t *testing.T) {
	var held []yaegikernel.HeldPage
	for i := 1; i <= 6; i++ {
		held = append(held, yaegikernel.HeldPage{URL: fmt.Sprintf("https://h.example/p%d", i), CaptureID: fmt.Sprintf("cap-%d", i)})
	}
	tool, ctx, _, _ := decisionRig(t, manyPagesClient{pages: 3}, func(r *Runtime) { r.noteBriefHeld("run-desk-eval", held) })
	root := evalDeskCell(t, tool, ctx, `s, _ := choir.Search("root question")
fmt.Println(s.ID == "", len(s.Results))`)
	if root.Error != "" || !strings.Contains(root.Stdout, "true 5") {
		t.Fatalf("the root session was not held by the brief's pages: %q %q", root.Error, root.Stdout)
	}
	childExec := toolregistry.ExecutionContextFrom(ctx)
	childExec.RunRecord = nil
	child := evalDeskCell(t, tool, toolregistry.WithExecutionContext(ctx, childExec), `s, _ := choir.Search("child question")
fmt.Println(s.ID, s.Refused == "")`)
	if child.Error != "" || !strings.Contains(child.Stdout, "s1 true") {
		t.Fatalf("a child was held by pages it was never handed: %q %q", child.Error, child.Stdout)
	}
}

func TestTheProbesAnswerAndTheFrameShareOneBound(t *testing.T) {
	rt := &Runtime{}
	rt.openTree("run-1")
	for i := 0; i < 2500; i++ {
		rt.noteTreeTried("run-1", fmt.Sprintf("https://a.example/p%d", i))
	}
	raw, err := rt.searchProbe("run-1")
	if err != nil {
		t.Fatal(err)
	}
	var res yaegikernel.SearchStartResult
	if err := json.Unmarshal(raw, &res); err != nil || res.Tree == nil {
		t.Fatalf("probe answer %s: %v", raw, err)
	}
	if len(res.Tree.Tried) != 2000 || res.Tree.Tried[1999] != "https://a.example/p2499" {
		t.Fatalf("the probe's answer is %d pages, last %q: want the most recent 2000", len(res.Tree.Tried), res.Tree.Tried[len(res.Tree.Tried)-1])
	}
	if st := rt.treeStateFor("run-1"); len(st.Tried) != 2000 {
		t.Fatalf("the frame's state is %d pages", len(st.Tried))
	}
}

func TestAClosedRunsTreeIsNotReopenedByALateCell(t *testing.T) {
	rt := &Runtime{}
	rt.openTree("run-1")
	rt.closeTree("run-1")
	rt.openTree("run-1")
	if _, ok := rt.treeRead.Load("run-1"); ok {
		t.Fatal("a cell that began after the run ended reopened its tree")
	}
}

func TestAResumedRunsLostGateStateIsLoggedForTheProofScript(t *testing.T) {
	var logged strings.Builder
	prev := log.Writer()
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(prev) })
	rt := &Runtime{}
	rt.noteGateStateLostIfResumed("run-fresh", false)
	if logged.Len() != 0 {
		t.Fatalf("a run that began in this process logged %q", logged.String())
	}
	rt.noteGateStateLostIfResumed("run-resumed", true)
	if !strings.Contains(logged.String(), "runtime: search gate state lost at resume run=run-resumed") {
		t.Fatalf("log = %q", logged.String())
	}
}
