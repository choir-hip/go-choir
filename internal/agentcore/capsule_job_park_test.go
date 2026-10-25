package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsulejob"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The busy-poll fix (docs/problems/engineering-busy-polls-its-own-job-2026-10-24.md):
// a cell whose Poll saw the run's own job still running parks the turn on
// the job's end. These tests are the failure modes written before the code.

// parkJobTable is a run's jobs as the park reads them: the running job and
// its wall-clock deadline, and every job's view.
type parkJobTable struct {
	mu       sync.Mutex
	running  bool
	view     yaegikernel.JobView
	deadline time.Time
}

func (f *parkJobTable) RunningJob(string) (yaegikernel.JobView, time.Time, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.view, f.deadline, f.running
}

func (f *parkJobTable) JobViews(string) []yaegikernel.JobView {
	f.mu.Lock()
	defer f.mu.Unlock()
	return []yaegikernel.JobView{f.view}
}

func (f *parkJobTable) end(state string, exit int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.running = false
	f.view.State, f.view.ExitCode, f.view.ElapsedSec = state, &exit, 420
	f.view.ReceiptRef = "capsule-exec:sha256:built"
}

type parkEvents struct {
	mu     sync.Mutex
	phases []string
	bodies []map[string]any
}

func (e *parkEvents) emit(kind types.EventKind, phase string, payload json.RawMessage) {
	if kind != types.EventRunProgress || !strings.HasPrefix(phase, "capsule_job_park") {
		return
	}
	var body map[string]any
	_ = json.Unmarshal(payload, &body)
	e.mu.Lock()
	defer e.mu.Unlock()
	e.phases = append(e.phases, phase)
	e.bodies = append(e.bodies, body)
}

func (e *parkEvents) snapshot() ([]string, []map[string]any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.phases...), append([]map[string]any(nil), e.bodies...)
}

func runningBuild(deadline time.Time) *parkJobTable {
	return &parkJobTable{running: true, deadline: deadline,
		view: yaegikernel.JobView{ID: "job-build", State: capsulejob.StateRunning, Argv: []string{"go", "build", "./cmd/autoputer"}}}
}

func evalCall(output string, isError bool) ([]types.ToolCall, []types.ToolResult) {
	return []types.ToolCall{{ID: "call-1", Name: "capsule_go_eval"}}, []types.ToolResult{{CallID: "call-1", Output: output, IsError: isError}}
}

type parkOutcome struct {
	res toolregistry.ToolLoopCompletionGuardResult
	ok  bool
	err error
}

func parkAsync(rt *Runtime, ctx context.Context, events *parkEvents, calls []types.ToolCall, results []types.ToolResult) chan parkOutcome {
	done := make(chan parkOutcome, 1)
	go func() {
		res, ok, err := rt.engineeringJobPark(ctx, &types.RunRecord{RunID: "run-1"}, newJobDeliveries(nil), events.emit, calls, results)
		done <- parkOutcome{res, ok, err}
	}()
	return done
}

func awaitPark(t *testing.T, done chan parkOutcome, within time.Duration) parkOutcome {
	t.Helper()
	select {
	case got := <-done:
		return got
	case <-time.After(within):
		t.Fatalf("the park did not end within %s", within)
	}
	return parkOutcome{}
}

// The host parks on the first job the cell's Poll returned unfinished,
// unless the cell ended the assignment or stopped a job (failure modes 2
// and 3). A failed cell that polled still waited.
func TestCellJobPark(t *testing.T) {
	stop := yaegikernel.StagedIntent{Kind: yaegikernel.IntentJob, JobOp: yaegikernel.JobOpStop, JobID: "job-build"}
	note := yaegikernel.StagedIntent{Kind: yaegikernel.IntentNote}
	for _, tc := range []struct {
		name   string
		result capsule.GoEvalResult
		want   string
	}{
		{"polled a running job", capsule.GoEvalResult{JobsAwaited: []string{"job-build", "job-test"}, Intents: []yaegikernel.StagedIntent{note}}, "job-build"},
		{"polled then failed", capsule.GoEvalResult{JobsAwaited: []string{"job-build"}, Error: "panic: build not done", ExitCode: 1}, "job-build"},
		{"no poll", capsule.GoEvalResult{}, ""},
		{"a terminal fate", capsule.GoEvalResult{JobsAwaited: []string{"job-build"}, FateTerminal: true}, ""},
		{"a stop in the cell", capsule.GoEvalResult{JobsAwaited: []string{"job-build"}, Intents: []yaegikernel.StagedIntent{stop}}, ""},
	} {
		if got := cellJobPark(tc.result); got != tc.want {
			t.Errorf("%s: park = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// Failure modes 1 and 6: the park holds until the polled job ends (a stale
// signal does not release it), resumes with the job's outcome as a wait,
// and records its start and end in the run's events.
func TestEngineeringJobParkWaitsForThePolledJob(t *testing.T) {
	rt, _ := testRuntime(t)
	table := runningBuild(time.Now().Add(20 * time.Minute))
	rt.jobTable = table
	events := &parkEvents{}
	calls, results := evalCall(`{"stdout":"running","job_park":"job-build"}`, false)
	done := parkAsync(rt, context.Background(), events, calls, results)
	time.Sleep(50 * time.Millisecond)
	rt.signalJobWake("run-1") // stale: the job still runs
	select {
	case got := <-done:
		t.Fatalf("released while the job runs: %+v", got)
	case <-time.After(150 * time.Millisecond):
	}
	phases, bodies := events.snapshot()
	if len(phases) != 1 || phases[0] != "capsule_job_park" || bodies[0]["job_id"] != "job-build" || bodies[0]["cause"] != "poll" || bodies[0]["deadline"] == nil {
		t.Fatalf("park start not recorded: %v %v", phases, bodies)
	}
	table.end(capsulejob.StateDone, 0)
	rt.signalJobWake("run-1")
	got := awaitPark(t, done, 5*time.Second)
	if got.err != nil || !got.ok || !got.res.Continue || !got.res.Waited ||
		!strings.Contains(got.res.Instruction, "job-build") || !strings.Contains(got.res.Instruction, "state done, exit code 0") {
		t.Fatalf("resume: %+v", got)
	}
	phases, bodies = events.snapshot()
	if len(phases) != 2 || phases[1] != "capsule_job_park_ended" || bodies[1]["job_id"] != "job-build" || bodies[1]["reason"] != "capsule_job_ended" || bodies[1]["state"] != capsulejob.StateDone {
		t.Fatalf("park end not recorded: %v %v", phases, bodies)
	}
}

// Failure modes 4 and 5: a cell that did not poll a running job, another
// tool, or a park on a job the run does not have, waits on nothing.
func TestEngineeringJobParkDoesNotApply(t *testing.T) {
	rt, _ := testRuntime(t)
	rt.jobTable = runningBuild(time.Now().Add(20 * time.Minute))
	events := &parkEvents{}
	type batch struct {
		name    string
		calls   []types.ToolCall
		results []types.ToolResult
	}
	evalBatch := func(name, output string, isError bool) batch {
		calls, results := evalCall(output, isError)
		return batch{name, calls, results}
	}
	for _, tc := range []batch{
		evalBatch("no park", `{"stdout":"ok"}`, false),
		evalBatch("unknown job", `{"job_park":"job-other"}`, false),
		evalBatch("a cell error", `tool_error: reduce: persist tray-1: boom`, true),
		{"another tool", []types.ToolCall{{ID: "c", Name: "desk_go_eval"}}, []types.ToolResult{{CallID: "c", Output: `{"job_park":"job-build"}`}}},
	} {
		got := awaitPark(t, parkAsync(rt, context.Background(), events, tc.calls, tc.results), 2*time.Second)
		if got.ok || got.err != nil {
			t.Errorf("%s: parked: %+v", tc.name, got)
		}
	}
	if phases, _ := events.snapshot(); len(phases) != 0 {
		t.Fatalf("events for parks that did not happen: %v", phases)
	}
	rt.jobTable = nil // a runtime with no capsule executor
	calls, results := evalCall(`{"job_park":"job-build"}`, false)
	if got := awaitPark(t, parkAsync(rt, context.Background(), events, calls, results), 2*time.Second); got.ok {
		t.Fatalf("parked with no job table: %+v", got)
	}
}

// Failure mode 2: the job ended between the cell and the park: the turn
// resumes at once with the outcome, without waiting for a signal.
func TestEngineeringJobParkOnAJobThatAlreadyEnded(t *testing.T) {
	rt, _ := testRuntime(t)
	table := runningBuild(time.Now().Add(20 * time.Minute))
	table.end(capsulejob.StateFailed, 2)
	rt.jobTable = table
	events := &parkEvents{}
	calls, results := evalCall(`{"job_park":"job-build"}`, false)
	got := awaitPark(t, parkAsync(rt, context.Background(), events, calls, results), 2*time.Second)
	if got.err != nil || !got.ok || !got.res.Waited || !strings.Contains(got.res.Instruction, "state failed, exit code 2") {
		t.Fatalf("resume: %+v", got)
	}
	phases, bodies := events.snapshot()
	if len(phases) != 2 || phases[1] != "capsule_job_park_ended" || bodies[1]["reason"] != "already_ended" {
		t.Fatalf("events: %v %v", phases, bodies)
	}
}

// Failure mode 3: the Complete refused because the job runs (failure 23)
// parks too, so the cell that tried to finish early is followed by the
// job's outcome, not by a fresh chance to Stop it.
func TestEngineeringJobParkAfterARefusedComplete(t *testing.T) {
	rt, _ := testRuntime(t)
	table := runningBuild(time.Now().Add(20 * time.Minute))
	rt.jobTable = table
	refusal := refuseCompleteWhileJobRuns(table, "run-1", []yaegikernel.StagedIntent{{LocalID: "tray-1", Kind: yaegikernel.IntentComplete}})
	if refusal == nil {
		t.Fatal("no refusal")
	}
	events := &parkEvents{}
	calls, results := evalCall("tool_error: "+refusal.Error(), true)
	done := parkAsync(rt, context.Background(), events, calls, results)
	select {
	case got := <-done:
		t.Fatalf("released while the job runs: %+v", got)
	case <-time.After(150 * time.Millisecond):
	}
	table.end(capsulejob.StateDone, 0)
	rt.signalJobWake("run-1")
	got := awaitPark(t, done, 5*time.Second)
	if !got.ok || !got.res.Waited || !strings.Contains(got.res.Instruction, "state done") {
		t.Fatalf("resume: %+v", got)
	}
	if _, bodies := events.snapshot(); len(bodies) == 0 || bodies[0]["cause"] != "complete_refused" {
		t.Fatalf("cause not recorded: %v", bodies)
	}
}

// Failure mode 1: a job that outlives its wall clock (plus slack) does not
// hold the turn: it resumes saying so; a cancelled run ends the park with
// the run's error.
func TestEngineeringJobParkIsBounded(t *testing.T) {
	rt, _ := testRuntime(t)
	rt.jobTable = runningBuild(time.Now().Add(-2 * jobWaitSlack))
	events := &parkEvents{}
	calls, results := evalCall(`{"job_park":"job-build"}`, false)
	got := awaitPark(t, parkAsync(rt, context.Background(), events, calls, results), 2*time.Second)
	if got.err != nil || !got.ok || !got.res.Waited || !strings.Contains(got.res.Instruction, "wall clock") {
		t.Fatalf("deadline: %+v", got)
	}
	if phases, bodies := events.snapshot(); len(phases) != 2 || bodies[1]["reason"] != "deadline" {
		t.Fatalf("events: %v %v", phases, bodies)
	}
	rt.jobTable = runningBuild(time.Now().Add(20 * time.Minute))
	ctx, cancel := context.WithCancel(context.Background())
	done := parkAsync(rt, ctx, events, calls, results)
	time.Sleep(50 * time.Millisecond)
	cancel()
	if got := awaitPark(t, done, 2*time.Second); got.err == nil {
		t.Fatalf("a cancelled run kept its park: %+v", got)
	}
}

// Problems 4 and 6 of docs/evidence/selfdev-trace-reviews-2026-10-25.md:
// round 1 was cancelled by its activation budget while parked, never having
// seen the budget, and followed most park ends with a Poll cell only to read
// the output. The park-ended resume carries the budget left and the job's
// output tail.
func TestEngineeringJobParkResumeShowsTheBudgetAndTheOutputTail(t *testing.T) {
	rt, _ := testRuntime(t)
	table := runningBuild(time.Now().Add(20 * time.Minute))
	table.view.Tail = strings.Repeat("x", 4000) + "\nFAIL reader-route chromium exit 0 bytes=0\n"
	rt.jobTable = table
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(23*time.Minute+30*time.Second))
	defer cancel()
	calls, results := evalCall(`{"job_park":"job-build"}`, false)
	done := parkAsync(rt, ctx, &parkEvents{}, calls, results)
	table.end(capsulejob.StateFailed, 1)
	rt.signalJobWake("run-1")
	got := awaitPark(t, done, 5*time.Second)
	text := got.res.Instruction
	if !strings.Contains(text, "Activation budget: 24 min left") || !strings.Contains(text, "Freeze before it ends") {
		t.Fatalf("no budget line: %s", text)
	}
	if !strings.Contains(text, "FAIL reader-route chromium exit 0 bytes=0") {
		t.Fatalf("no output tail: %s", text)
	}
	if strings.Count(text, "x") > 1600 {
		t.Fatalf("the tail is not bounded: %d bytes", len(text))
	}

	// Under ten minutes the line says to freeze now.
	table = runningBuild(time.Now().Add(20 * time.Minute))
	table.end(capsulejob.StateDone, 0)
	rt.jobTable = table
	ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(7*time.Minute))
	defer cancel()
	got = awaitPark(t, parkAsync(rt, ctx, &parkEvents{}, calls, results), 2*time.Second)
	if !strings.Contains(got.res.Instruction, "Activation budget: 7 min left") || !strings.Contains(got.res.Instruction, "Freeze what you have now") {
		t.Fatalf("no urgent budget line: %s", got.res.Instruction)
	}

	// A run with no deadline says nothing about a budget.
	got = awaitPark(t, parkAsync(rt, context.Background(), &parkEvents{}, calls, results), 2*time.Second)
	if strings.Contains(got.res.Instruction, "Activation budget") {
		t.Fatalf("a budget line without a deadline: %s", got.res.Instruction)
	}
}
