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
	rt.researchDeps = &researchtools.Dependencies{Search: &stubSearchClient{}, Egress: researchtools.NewEgressBudgetLedger(10, 32<<20)}
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
	go func() { _, err := choir.WebSearch(q, 1); done <- err }()
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
	if len(egress) != 3 {
		t.Fatalf("want three desk.egress events, got %d", len(egress))
	}
	seen := map[any]bool{}
	for _, e := range egress {
		id := e["request_id"]
		if id == "" || id == nil || id != e["response_request_id"] || seen[id] {
			t.Fatalf("egress event needs a distinct request id echoed on its result: %v", e)
		}
		seen[id] = true
		if e["verb"] != "web_search" || e["worker_pid"] != cells[0]["worker_pid"] {
			t.Fatalf("egress event verb/worker mismatch: %v", e)
		}
	}
}
