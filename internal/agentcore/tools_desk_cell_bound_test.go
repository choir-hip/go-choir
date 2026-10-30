package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// On the paper (2026-10-16) a management cell with timeout_ms=1000 was
// killed after running 3 ms: the bound started before the host read the
// cell's pending updates and emits, and that second went to the cell
// (docs/archive/problems/desk-cell-timeout-counts-host-prep-2026-10-16.md).
//
// Ways this could fail, written first:
//  1. host preparation slower than the bound kills a cell that has not run;
//  2. the reported run time excludes time the bound counted, so the model
//     reads "after 3 ms" for a 1000 ms bound;
//  3. a cell that does run past its bound is no longer stopped.
func TestDeskCellTimeoutCountsOnlyTheCell(t *testing.T) {
	deskTestWorkerBin(t)
	rt, _ := testRuntime(t)
	workers := newDeskSessionWorkers()
	t.Cleanup(workers.closeAll)
	tool := newDeskGoEvalTool(rt, workers, agentprofile.Research)
	execCtx := deskEvalExecCtx(t)
	execCtx.Profile = agentprofile.Research
	execCtx.RunID = "run-cell-bound"
	execCtx.RunRecord = &types.RunRecord{RunID: execCtx.RunID, AgentID: "research:" + execCtx.OwnerID, OwnerID: execCtx.OwnerID, TrajectoryID: "traj-cell-bound"}
	ctx := toolregistry.WithExecutionContext(context.Background(), execCtx)

	// Start the worker first, so the slow step below is the host's own.
	if out, err := tool.Func(ctx, json.RawMessage(`{"source":"fmt.Println(\"warm\")"}`)); err != nil || !strings.Contains(out, "warm") {
		t.Fatalf("warm cell: %v (%s)", err, out)
	}

	prev := deskCellFramePrepared
	deskCellFramePrepared = func() { time.Sleep(700 * time.Millisecond) }
	t.Cleanup(func() { deskCellFramePrepared = prev })

	// Failure modes 1 and 2.
	out, err := tool.Func(ctx, json.RawMessage(`{"source":"fmt.Println(\"ran\")","timeout_ms":500}`))
	if err != nil {
		t.Fatalf("desk_go_eval: %v", err)
	}
	if strings.Contains(out, "timeout_ms") || !strings.Contains(out, "ran") {
		t.Fatalf("a quick cell behind slow host preparation did not run: %s", out)
	}

	// Failure mode 3.
	out, err = tool.Func(ctx, json.RawMessage(`{"source":`+cellSource(`n := 0
for {
	n++
}`)+`,"timeout_ms":500}`))
	if err != nil {
		t.Fatalf("desk_go_eval: %v", err)
	}
	var got struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("decode %s: %v", out, err)
	}
	if !strings.Contains(got.Error, "timeout_ms=500 after") {
		t.Fatalf("a cell past its bound was not stopped by it: %q", got.Error)
	}
}
