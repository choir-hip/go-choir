package agentcore

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Cut 0 trace export: every desk cell leaves a desk.cell run event and every
// choir.* call it makes leaves a desk.egress event carrying the request id
// the worker minted and the id the host answered with, so a bundle shows
// which caller got which answer
// (docs/reports/reeng-trace-bundle-format-2026-10-12.md).
//
// Ways this could fail, written first: the events land on no run (the cell
// has a run record but the event is keyed elsewhere); the egress event lacks
// the request id because the frame id never reaches the handler; parallel
// calls record one event; the cell event omits the worker or session that
// I5 checks.
func TestDeskCellAndEgressRecordRunEvents(t *testing.T) {
	deskTestWorkerBin(t)
	rt, s := testRuntime(t)
	rt.researchDeps = &researchtools.Dependencies{Search: stubSearchClient{}}
	workers := newDeskSessionWorkers()
	t.Cleanup(workers.closeAll)
	tool := newDeskGoEvalTool(rt, workers, agentprofile.Research)
	execCtx := deskEvalExecCtx(t)
	execCtx.Profile = agentprofile.Research
	execCtx.Role = agentprofile.Research
	execCtx.RunID = "run-trace-events"
	execCtx.RunRecord = &types.RunRecord{RunID: execCtx.RunID, AgentID: "research:trace", OwnerID: execCtx.OwnerID, TrajectoryID: "traj-trace"}
	ctx := toolregistry.WithExecutionContext(context.Background(), execCtx)

	src, _ := json.Marshal(`done := make(chan error, 3)
for _, q := range []string{"q1", "q2", "q3"} {
	q := q
	go func() {
		s, err := choir.SearchStart(q)
		if err == nil {
			_, err = choir.SearchWait(s.ID, 5000)
		}
		done <- err
	}()
}
for i := 0; i < 3; i++ {
	if err := <-done; err != nil {
		panic(err)
	}
}`)
	out, err := tool.Func(ctx, json.RawMessage(`{"source":`+string(src)+`,"timeout_ms":30000}`))
	if err != nil {
		t.Fatalf("desk_go_eval: %v", err)
	}
	var cell deskCellOut
	if err := json.Unmarshal([]byte(out), &cell); err != nil || cell.Error != "" {
		t.Fatalf("cell failed: %s (%v)", out, err)
	}

	evs, err := s.ListEvents(context.Background(), execCtx.RunID, 100)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	var cells, egress []map[string]any
	for _, ev := range evs {
		var p map[string]any
		_ = json.Unmarshal(ev.Payload, &p)
		switch ev.Kind {
		case types.EventDeskCell:
			cells = append(cells, p)
		case types.EventDeskEgress:
			egress = append(egress, p)
		}
		if ev.TrajectoryID != "traj-trace" && (ev.Kind == types.EventDeskCell || ev.Kind == types.EventDeskEgress) {
			t.Fatalf("event %s not keyed to the run's trajectory: %q", ev.Kind, ev.TrajectoryID)
		}
	}
	if len(cells) != 1 {
		t.Fatalf("want one desk.cell event, got %d", len(cells))
	}
	if cells[0]["desk_session"] == "" || cells[0]["worker_pid"] == float64(0) || cells[0]["started_at"] == nil {
		t.Fatalf("desk.cell lacks session, worker or timing: %v", cells[0])
	}
	// Journal cut R4: a cell's per-call egress rows ride its desk.cell event
	// (egress[]), one durable record per cell. Only the async end of a search
	// (search_done, which outlives the cell) and refusals stay standalone.
	var starts, dones []map[string]any
	rows, _ := cells[0]["egress"].([]any)
	for _, r := range rows {
		e, _ := r.(map[string]any)
		if e["verb"] != "search_start" {
			t.Fatalf("folded egress row with unexpected verb: %v", e)
		}
		starts = append(starts, e)
	}
	for _, e := range egress {
		if e["verb"] != "search_done" {
			t.Fatalf("a per-call egress row was appended standalone: %v", e)
		}
		dones = append(dones, e)
	}
	if len(starts) != 3 || len(dones) != 3 || len(egress) != 3 {
		t.Fatalf("want three search_start rows on the desk.cell event and three standalone search_done events, got %d and %d of %d", len(starts), len(dones), len(egress))
	}
	seen := map[any]bool{}
	for _, e := range starts {
		id := e["request_id"]
		if id == "" || id == nil || id != e["response_request_id"] || seen[id] {
			t.Fatalf("egress row needs a distinct request id echoed on its result: %v", e)
		}
		seen[id] = true
		if e["worker_pid"] != cells[0]["worker_pid"] || e["desk_session"] != cells[0]["desk_session"] || e["run_id"] != cells[0]["run_id"] {
			t.Fatalf("egress row session, worker or run mismatch: %v", e)
		}
		if e["started_at"] == nil || e["finished_at"] == nil || e["is_error"] != false {
			t.Fatalf("egress row lacks timing or outcome: %v", e)
		}
	}
	searchIDs := map[any]bool{}
	for _, e := range dones {
		if e["answered"] != float64(1) || e["total"] != float64(1) || searchIDs[e["search_id"]] {
			t.Fatalf("search_done event needs its own search id and counts: %v", e)
		}
		searchIDs[e["search_id"]] = true
	}
}

// A choir.Research child runs in a pool of its own, keyed on the root run.
// Its desk.cell events must name the child's own session, or two sibling
// children read as two workers on one desk session in the trace (C0
// breakage B2; one REPL per desk does not cover sub-RLMs).
func TestResearchChildCellsNameTheirOwnSession(t *testing.T) {
	deskTestWorkerBin(t)
	rt, s := testRuntime(t)
	execCtx := deskEvalExecCtx(t)
	execCtx.Profile = agentprofile.Research
	execCtx.Role = agentprofile.Research
	execCtx.RunID = "run-root"
	parent := types.RunRecord{RunID: "run-root", AgentID: "research:doc", OwnerID: execCtx.OwnerID, ComputerID: execCtx.ComputerID}
	if err := s.CreateRun(context.Background(), parent); err != nil {
		t.Fatalf("create run: %v", err)
	}
	for _, child := range []string{"child-a", "child-b"} {
		workers := newResearchChildDeskWorkers(child)
		t.Cleanup(workers.closeAll)
		tool := newDeskGoEvalTool(rt, workers, agentprofile.Research)
		ctx := toolregistry.WithExecutionContext(context.Background(), execCtx)
		if out, err := tool.Func(ctx, json.RawMessage(`{"source":"x := 1; _ = x","timeout_ms":15000}`)); err != nil {
			t.Fatalf("child cell: %v (%s)", err, out)
		}
	}
	evs, err := s.ListEvents(context.Background(), "run-root", 100)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	sessions := map[any]bool{}
	for _, ev := range evs {
		if ev.Kind != types.EventDeskCell {
			continue
		}
		var p map[string]any
		_ = json.Unmarshal(ev.Payload, &p)
		sessions[p["desk_session"]] = true
	}
	if len(sessions) != 2 {
		t.Fatalf("two children must name two sessions, got %v", sessions)
	}
}
