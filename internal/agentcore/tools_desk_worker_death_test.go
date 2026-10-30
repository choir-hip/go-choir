package agentcore

import (
	"context"
	"encoding/json"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// On the paper (2026-10-16) runaway cells filled the guest: the OOM killer
// took a Texture worker at 12:46 and the daemon at 18:55, closing every run
// (docs/archive/problems/desk-cell-init-only-for-loop-never-ends-guest-oom-2026-10-16.md).
// Each dead worker reached the model as "frame: read header: EOF", and the
// model re-ran the same loop.
//
// Ways this could fail, written first:
//  1. a worker that dies mid-cell, with no deadline, still reaches the model
//     as a bare transport error, with no cause and nothing to change;
//  2. the death is reported as a deadline, or a deadline's message changes;
//  3. the worker's stderr tail (a goroutine dump) floods the result;
//  4. the dead session is reused;
//  5. on Linux the worker does not volunteer for the OOM killer, so the
//     guest kills the daemon, the largest process, first;
//  6. volunteering fails the worker's start where /proc is not writable.

func deskCellWorkerPID(t *testing.T, s *store.Store, runID string) int {
	t.Helper()
	evs, err := s.ListEvents(context.Background(), runID, 100)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	pid := 0
	for _, ev := range evs {
		if ev.Kind != types.EventDeskCell {
			continue
		}
		var p struct {
			WorkerPID int `json:"worker_pid"`
		}
		_ = json.Unmarshal(ev.Payload, &p)
		if p.WorkerPID > 0 {
			pid = p.WorkerPID
		}
	}
	if pid == 0 {
		t.Fatalf("no desk.cell event names a worker pid: %d events", len(evs))
	}
	return pid
}

// Failure modes 1, 2, 3 and 4.
func TestDeskWorkerKilledMidCellTellsTheModelWhy(t *testing.T) {
	deskTestWorkerBin(t)
	rt, s := testRuntime(t)
	workers := newDeskSessionWorkers()
	t.Cleanup(workers.closeAll)
	tool := newDeskGoEvalTool(rt, workers, agentprofile.Research)
	execCtx := deskEvalExecCtx(t)
	execCtx.Profile = agentprofile.Research
	execCtx.RunID = "run-worker-death"
	execCtx.RunRecord = &types.RunRecord{RunID: execCtx.RunID, AgentID: "research:" + execCtx.OwnerID, OwnerID: execCtx.OwnerID, TrajectoryID: "traj-worker-death"}
	ctx := toolregistry.WithExecutionContext(context.Background(), execCtx)

	if out, err := tool.Func(ctx, json.RawMessage(`{"source":"kept := 7"}`)); err != nil {
		t.Fatalf("seed cell: %v (%s)", err, out)
	}
	pid := deskCellWorkerPID(t, s, execCtx.RunID)

	busyExec := execCtx
	busyExec.ToolCallID = "call-busy"
	busyCtx := toolregistry.WithExecutionContext(context.Background(), busyExec)
	go func() {
		time.Sleep(500 * time.Millisecond)
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}()
	out, err := tool.Func(busyCtx, json.RawMessage(`{"source":`+cellSource(`n := 0
for {
	n++
}`)+`}`))
	if err != nil {
		t.Fatalf("desk_go_eval returned error: %v", err)
	}
	var got struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("decode %s: %v", out, err)
	}
	for _, want := range []string{"worker died", "signal: killed", "session was killed", "out of memory", "every loop ends"} {
		if !strings.Contains(got.Error, want) {
			t.Fatalf("worker death error %q does not name %q", got.Error, want)
		}
	}
	for _, not := range []string{"deadline", "timeout_ms=", "stderr", "goroutine"} {
		if strings.Contains(got.Error, not) {
			t.Fatalf("worker death error %q mentions %q", got.Error, not)
		}
	}

	out, err = tool.Func(ctx, json.RawMessage(`{"source":"fmt.Println(kept)"}`))
	if err != nil {
		t.Fatalf("cell after death: %v", err)
	}
	if !strings.Contains(out, "undefined") {
		t.Fatalf("value from the dead session survived: %s", out)
	}
}

// Failure modes 5 and 6.
func TestDeskWorkerVolunteersForTheOOMKiller(t *testing.T) {
	deskTestWorkerBin(t)
	rt, s := testRuntime(t)
	workers := newDeskSessionWorkers()
	t.Cleanup(workers.closeAll)
	tool := newDeskGoEvalTool(rt, workers, agentprofile.Research)
	execCtx := deskEvalExecCtx(t)
	execCtx.Profile = agentprofile.Research
	execCtx.RunID = "run-worker-oom-adj"
	execCtx.RunRecord = &types.RunRecord{RunID: execCtx.RunID, AgentID: "research:" + execCtx.OwnerID, OwnerID: execCtx.OwnerID, TrajectoryID: "traj-worker-oom-adj"}
	ctx := toolregistry.WithExecutionContext(context.Background(), execCtx)

	// The worker starts and serves a cell on every platform.
	out, err := tool.Func(ctx, json.RawMessage(`{"source":"fmt.Println(\"up\")"}`))
	if err != nil || !strings.Contains(out, "up") {
		t.Fatalf("cell: %v (%s)", err, out)
	}
	if runtime.GOOS != "linux" {
		t.Skip("oom_score_adj is Linux only")
	}
	pid := deskCellWorkerPID(t, s, execCtx.RunID)
	raw, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/oom_score_adj")
	if err != nil {
		t.Fatalf("read worker oom_score_adj: %v", err)
	}
	if got := strings.TrimSpace(string(raw)); got != "1000" {
		t.Fatalf("worker oom_score_adj = %s, want 1000", got)
	}
	self, err := os.ReadFile("/proc/self/oom_score_adj")
	if err != nil {
		t.Fatalf("read own oom_score_adj: %v", err)
	}
	if strings.TrimSpace(string(self)) == "1000" {
		t.Fatal("the daemon side volunteered too; only the worker should")
	}
}
