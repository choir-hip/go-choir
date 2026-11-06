package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// MA6.6 code mode (plan section 12; design
// docs/design/ma66-code-mode-import-sets-2026-11-06.md): a cell makes a
// hundred read-verb calls in one model round trip, and the trace shows one
// round trip. Failure modes, written first:
//
//	C1 the hundred calls cost more than one model call (the loop is driven
//	   by the model, a call per tool use);
//	C2 a call in the loop is refused or fails (a per-cell bound below a
//	   hundred, a frame the host does not serve);
//	C3 the trace does not show it: no desk.cell event, or its host-call count
//	   is not the hundred the cell made;
//	C4 the run does not complete on the turn after the cell.
type codeModeProvider struct {
	*provider.StubProvider
	mu     sync.Mutex
	calls  int
	output string
}

func (p *codeModeProvider) CallWithTools(_ context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	raw := rawMessagesForTest(req.Messages)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	if !strings.Contains(raw, "call-code-mode") {
		args, _ := json.Marshal(map[string]any{"source": `ok, docs := 0, 0
for i := 0; i < 100; i++ {
	page, err := choir.Stories(5, "")
	if err != nil {
		fmt.Println("call", i, err)
		continue
	}
	ok++
	docs = len(page.Stories)
}
fmt.Println("reads", ok, "stories", docs)`, "timeout_ms": 60000})
		return &provideriface.ToolLoopResponse{StopReason: "tool_use", Model: "test-model",
			ToolCalls: []types.ToolCall{{ID: "call-code-mode", Name: "desk_go_eval", Arguments: args}}}, nil
	}
	p.output = raw[strings.Index(raw, "call-code-mode"):]
	return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "done", Model: "test-model"}, nil
}

func TestMA66AHundredReadsInOneModelRoundTrip(t *testing.T) {
	deskTestWorkerBin(t)
	rt, s := testRuntime(t)
	model := &codeModeProvider{StubProvider: provider.NewStubProvider(0)}
	rt.provider = model
	if err := rt.InstallDefaultAgentTools(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	owner, computer := "owner-code-mode", rt.cfg.ComputerID
	management := persistentManagementAgentID(owner)
	rt.SetDispatchActor(func(dctx context.Context, ownerID, _, _, kind, content, _, _ string) error {
		if kind == "initial_dispatch" {
			go func() {
				if rec, err := s.GetRunByOwner(dctx, ownerID, strings.TrimSpace(content)); err == nil {
					rt.ExecuteActivationSync(dctx, &rec)
				}
			}()
		}
		return nil
	})
	rt.SetScheduleActor(func(context.Context, string, string, string, string, string, string, string, time.Time) error {
		return nil
	})
	if err := s.PutComputerSetting(ctx, store.ComputerSetting{OwnerID: owner, ComputerID: computer, Key: store.SettingIntakeLabels, Value: true}); err != nil {
		t.Fatal(err)
	}
	if err := rt.appendCaptureArrival(ctx, owner, computer, "rss:code-mode", []types.CaptureArrival{
		{CaptureID: "obj:choir.web_capture:test:code-mode", Title: "a"},
	}, false); err != nil {
		t.Fatal(err)
	}
	rec, err := rt.WakeRecordsInbox(ctx, owner, computer, management)
	if err != nil || rec == nil {
		t.Fatalf("wake management: %v, %v", rec, err)
	}
	run := waitForStoredRunTerminalState(t, s, rec.RunID, 60*time.Second)
	// C4.
	if run.State != types.RunCompleted {
		t.Fatalf("run %s: %s (%s)", run.RunID, run.State, run.Error)
	}
	model.mu.Lock()
	calls, output := model.calls, model.output
	model.mu.Unlock()
	// C2: every call answered.
	if !strings.Contains(output, `\"stdout\":\"reads 100 stories`) {
		t.Fatalf("the cell's hundred reads: %s", output)
	}

	evs, err := s.ListEvents(ctx, run.RunID, 500)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrips []map[string]any
	var cells []map[string]any
	for _, ev := range evs {
		var p map[string]any
		_ = json.Unmarshal(ev.Payload, &p)
		switch {
		case ev.Kind == types.EventRunProgress && p["phase"] == "tool_loop":
			roundTrips = append(roundTrips, p)
		case ev.Kind == types.EventDeskCell:
			cells = append(cells, p)
		}
	}
	// C1/C3: the trace shows two model calls, the one that wrote the cell
	// and the one that ended the turn; one desk.cell event carries the
	// hundred host calls.
	if calls != 2 || len(roundTrips) != 2 {
		t.Fatalf("model calls = %d, tool_loop events = %d; want 2 and 2", calls, len(roundTrips))
	}
	if roundTrips[0]["tool_calls"] != float64(1) || roundTrips[1]["tool_calls"] != float64(0) {
		t.Fatalf("round trips: %v / %v", roundTrips[0]["tool_call_names"], roundTrips[1]["tool_call_names"])
	}
	if len(cells) != 1 {
		t.Fatalf("desk.cell events = %d, want 1", len(cells))
	}
	rows, _ := cells[0]["egress"].([]any)
	if cells[0]["egress_calls"] != float64(100) || len(rows) != 100 || cells[0]["egress_omitted"] != nil {
		t.Fatalf("desk.cell host calls = %v, rows %d, omitted %v", cells[0]["egress_calls"], len(rows), cells[0]["egress_omitted"])
	}
	for _, r := range rows {
		if row, _ := r.(map[string]any); row["verb"] != "stories" || row["is_error"] != false {
			t.Fatalf("egress row: %v", row)
		}
	}
	t.Logf("one model round trip, %v host calls, cell %v ms", cells[0]["egress_calls"], cells[0]["duration_ms"])
}

// TestMA66MeasureHostCalls is the measurement the design records (section
// 4): N read-verb host calls in one cell through the real worker, the daemon's
// egress handler and the store. Run it with MA66_MEASURE=1.
func TestMA66MeasureHostCalls(t *testing.T) {
	if os.Getenv("MA66_MEASURE") == "" {
		t.Skip("a measurement: MA66_MEASURE=1")
	}
	deskTestWorkerBin(t)
	rt, s := testRuntime(t)
	workers := newDeskSessionWorkers()
	t.Cleanup(workers.closeAll)
	tool := newDeskGoEvalTool(rt, workers, agentprofile.Research)
	for _, n := range []int{100, 1000, 1100} {
		execCtx := deskEvalExecCtx(t)
		execCtx.Profile, execCtx.Role = agentprofile.Research, agentprofile.Research
		execCtx.RunID = fmt.Sprintf("run-measure-%d", n)
		execCtx.RunRecord = &types.RunRecord{RunID: execCtx.RunID, AgentID: "research:doc-1", OwnerID: execCtx.OwnerID}
		ctx := objectgraph.WithReader(toolregistry.WithExecutionContext(context.Background(), execCtx), "desk:research")
		src, _ := json.Marshal(fmt.Sprintf(`ok := 0
for i := 0; i < %d; i++ {
	if _, err := choir.ListEvidence("", 5); err == nil { ok++ }
}
fmt.Println("ok", ok)`, n))
		start := time.Now()
		out, err := tool.Func(ctx, json.RawMessage(`{"source":`+string(src)+`,"timeout_ms":600000}`))
		elapsed := time.Since(start)
		var c deskCellOut
		_ = json.Unmarshal([]byte(out), &c)
		first, _, _ := strings.Cut(c.Stdout, "\n")
		evs, _ := s.ListEvents(context.Background(), execCtx.RunID, 100)
		size := 0
		for _, ev := range evs {
			if ev.Kind == types.EventDeskCell {
				size += len(ev.Payload)
			}
		}
		t.Logf("n=%d elapsed=%v per-call=%v desk.cell bytes=%d out=%q err=%v", n, elapsed, elapsed/time.Duration(n), size, first, err)
	}
}
