package agentcore

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Management delegation S1 (design round 2, R2-19): a host deadline on
// management cells, independent of delegation. Run 305a582d's cell 6 ran
// 860,565 ms with no bound until its worker died; a cell had a deadline only
// when the model set timeout_ms. The default is max(300 s, 2 x the p99 cell
// duration S0 measured on the paper); S0 measured p99 4.6 s, so 300 s.
//
// Ways this could fail, written first:
//  1. a management cell that never ends runs unbounded (the 860 s cell);
//  2. the kill reaches the model as a bare "context deadline exceeded", so
//     it cannot tell its own timeout_ms from the host's bound;
//  3. the kill leaves no operator trace: no log line naming the run and the
//     time the cell ran, and nothing on the desk.cell event;
//  4. the bound leaks to other desks, where long cells are legitimate
//     (research waits on searches with choir.SearchWait);
//  5. the model's own timeout_ms stops meaning what it meant;
//  6. the killed worker is reused, or its staged acts commit;
//  7. the run's own cancellation is reported as the host deadline;
//  8. a bad CHOIR_MANAGEMENT_CELL_DEADLINE silently removes the bound;
//  9. the default is so low that it kills legitimate cells.

type lockedLogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func captureLog(t *testing.T) *lockedLogBuffer {
	t.Helper()
	buf := &lockedLogBuffer{}
	prev := log.Writer()
	log.SetOutput(buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	return buf
}

// Failure modes 8 and 9.
func TestManagementCellDeadlineDefaultAndEnv(t *testing.T) {
	cases := []struct {
		env  string
		role string
		want time.Duration
	}{
		{"", agentprofile.Management, 300 * time.Second},
		{"90s", agentprofile.Management, 90 * time.Second},
		{"not-a-duration", agentprofile.Management, 300 * time.Second},
		{"-5s", agentprofile.Management, 300 * time.Second},
		{"0", agentprofile.Management, 0},
		{"off", agentprofile.Management, 0},
		{"", agentprofile.Research, 0},
		{"90s", agentprofile.Texture, 0},
		{"90s", agentprofile.Engineering, 0},
	}
	for _, tc := range cases {
		t.Setenv(managementCellDeadlineEnv, tc.env)
		if got := deskCellHostDeadline(tc.role); got != tc.want {
			t.Fatalf("deadline(role=%s, env=%q) = %s, want %s", tc.role, tc.env, got, tc.want)
		}
	}
}

// Failure modes 1, 2, 3 and 6.
func TestManagementCellOverHostDeadlineIsKilledAndReported(t *testing.T) {
	deskTestWorkerBin(t)
	t.Setenv(managementCellDeadlineEnv, "1s")
	logs := captureLog(t)
	rt, s := testRuntime(t)
	workers := newDeskSessionWorkers()
	t.Cleanup(workers.closeAll)
	tool := newDeskGoEvalTool(rt, workers, agentprofile.Management)
	execCtx := deskEvalExecCtx(t)
	execCtx.RunID = "run-mgmt-deadline"
	execCtx.RunRecord = &types.RunRecord{RunID: execCtx.RunID, AgentID: "management:" + execCtx.OwnerID, OwnerID: execCtx.OwnerID, TrajectoryID: "traj-mgmt-deadline"}
	ctx := toolregistry.WithExecutionContext(context.Background(), execCtx)

	// A value from an earlier cell, to show the session dies with the kill.
	if out, err := tool.Func(ctx, json.RawMessage(`{"source":"kept := 7"}`)); err != nil {
		t.Fatalf("seed cell: %v (%s)", err, out)
	}
	busyExec := execCtx
	busyExec.ToolCallID = "call-busy"
	busyCtx := toolregistry.WithExecutionContext(context.Background(), busyExec)
	started := time.Now()
	out, err := tool.Func(busyCtx, json.RawMessage(`{"source":`+cellSource(`n := 0
for {
	n++
}`)+`}`))
	if err != nil {
		t.Fatalf("desk_go_eval returned error: %v", err)
	}
	if elapsed := time.Since(started); elapsed > 20*time.Second {
		t.Fatalf("host deadline of 1s took %s", elapsed)
	}
	var got struct {
		Error    string `json:"error"`
		Duration int64  `json:"duration"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("decode %s: %v", out, err)
	}
	for _, want := range []string{"management cell deadline", "1s", "session was killed", managementCellDeadlineEnv} {
		if !strings.Contains(got.Error, want) {
			t.Fatalf("deadline error %q does not name %q", got.Error, want)
		}
	}
	if strings.Contains(got.Error, "timeout_ms=") {
		t.Fatalf("host deadline reported as the cell's own timeout_ms: %q", got.Error)
	}
	// The bound starts as the cell is prepared, just before evaluation.
	if got.Duration < 900 {
		t.Fatalf("duration = %d ms, want the time the cell ran", got.Duration)
	}

	// The log names the run, the deadline, the time and the worker's exit.
	line := logs.String()
	for _, want := range []string{"management cell deadline", execCtx.RunID, "killed by the host"} {
		if !strings.Contains(line, want) {
			t.Fatalf("kill log %q does not name %q", line, want)
		}
	}

	// The desk.cell event carries the same.
	evs, err := s.ListEvents(context.Background(), execCtx.RunID, 100)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	var killed map[string]any
	for _, ev := range evs {
		if ev.Kind != types.EventDeskCell {
			continue
		}
		var p map[string]any
		_ = json.Unmarshal(ev.Payload, &p)
		if p["killed_by"] != nil {
			killed = p
		}
	}
	if killed == nil {
		t.Fatalf("no desk.cell event records the kill: %d events", len(evs))
	}
	if killed["killed_by"] != "host_deadline" || killed["host_deadline_ms"] != float64(1000) {
		t.Fatalf("desk.cell kill fields = %v", killed)
	}
	if exit, _ := killed["worker_exit"].(string); !strings.Contains(exit, "killed by the host") {
		t.Fatalf("desk.cell worker_exit = %q", exit)
	}

	// The cell's fate is a timeout: it reduced nothing (its tray died with
	// the worker) and the run's ledger says why.
	fate, found, err := CellFate(context.Background(), rt.store, execCtx.OwnerID, execCtx.RunID, cellIDForExecution(busyExec, 0))
	if err != nil || !found || fate != cellFateTimeout {
		t.Fatalf("killed cell fate = %q, found=%t, err=%v; want %q", fate, found, err, cellFateTimeout)
	}

	// The killed session is not reused: the next cell runs in a new one.
	out, err = tool.Func(ctx, json.RawMessage(`{"source":"fmt.Println(kept)"}`))
	if err != nil {
		t.Fatalf("cell after kill: %v", err)
	}
	if !strings.Contains(out, "undefined") {
		t.Fatalf("value from the killed session survived: %s", out)
	}
}

// Failure mode 4: other desks keep running cells past the management bound.
func TestCellHostDeadlineLeavesOtherDesksAlone(t *testing.T) {
	deskTestWorkerBin(t)
	t.Setenv(managementCellDeadlineEnv, "600ms")
	for _, role := range []string{agentprofile.Research, agentprofile.Texture} {
		workers := newDeskSessionWorkers()
		tool := newDeskGoEvalTool(&Runtime{}, workers, role)
		execCtx := deskEvalExecCtx(t)
		execCtx.Profile = role
		execCtx.Role = role
		ctx := toolregistry.WithExecutionContext(context.Background(), execCtx)
		out, err := tool.Func(ctx, json.RawMessage(`{"source":`+cellSource(`import "time"
time.Sleep(1200 * time.Millisecond)
fmt.Println("finished")`)+`}`))
		workers.closeAll()
		if err != nil {
			t.Fatalf("%s cell: %v", role, err)
		}
		if !strings.Contains(out, "finished") || strings.Contains(out, "deadline") {
			t.Fatalf("%s cell was bounded by the management deadline: %s", role, out)
		}
	}
}

// Failure mode 5: the model's own timeout_ms keeps its meaning on management.
func TestManagementCellOwnTimeoutStillApplies(t *testing.T) {
	deskTestWorkerBin(t)
	t.Setenv(managementCellDeadlineEnv, "600ms")
	workers := newDeskSessionWorkers()
	t.Cleanup(workers.closeAll)
	tool := newDeskGoEvalTool(&Runtime{}, workers, agentprofile.Management)
	ctx := toolregistry.WithExecutionContext(context.Background(), deskEvalExecCtx(t))

	out, err := tool.Func(ctx, json.RawMessage(`{"source":`+cellSource(`import "time"
time.Sleep(1500 * time.Millisecond)`)+`,"timeout_ms":300}`))
	if err != nil {
		t.Fatalf("desk_go_eval: %v", err)
	}
	if !strings.Contains(out, "timeout_ms=300") || strings.Contains(out, "management cell deadline") {
		t.Fatalf("own timeout not reported as itself: %s", out)
	}

	out, err = tool.Func(ctx, json.RawMessage(`{"source":`+cellSource(`import "time"
time.Sleep(1200 * time.Millisecond)
fmt.Println("finished")`)+`,"timeout_ms":5000}`))
	if err != nil {
		t.Fatalf("desk_go_eval: %v", err)
	}
	if !strings.Contains(out, "finished") {
		t.Fatalf("a cell within its own timeout_ms was cut by the host deadline: %s", out)
	}
}

// Failure mode 7: the run's own cancellation is not the host deadline.
func TestManagementCellRunCancelIsNotTheHostDeadline(t *testing.T) {
	deskTestWorkerBin(t)
	t.Setenv(managementCellDeadlineEnv, "30s")
	workers := newDeskSessionWorkers()
	t.Cleanup(workers.closeAll)
	tool := newDeskGoEvalTool(&Runtime{}, workers, agentprofile.Management)
	base, cancel := context.WithCancel(context.Background())
	ctx := toolregistry.WithExecutionContext(base, deskEvalExecCtx(t))
	time.AfterFunc(500*time.Millisecond, cancel)
	out, _ := tool.Func(ctx, json.RawMessage(`{"source":`+cellSource(`import "time"
time.Sleep(10 * time.Second)`)+`}`))
	if strings.Contains(out, "management cell deadline") {
		t.Fatalf("run cancellation reported as the host deadline: %s", out)
	}
}
