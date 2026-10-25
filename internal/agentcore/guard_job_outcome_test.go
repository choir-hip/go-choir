package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsulejob"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// N1 of docs/evidence/selfdev-trace-reviews-2026-10-25.md (Round 3): the
// completion guard cancelled engineers that were still working. Each
// engineer started a short job (a 2-3 s probe or capture), ended its turn,
// and the job ended before the guard looked: engineeringJobWait saw no
// running job, so the generic reminder came 0.14-0.22 s after end_turn and
// was charged as a retry; the third such end_turn cancelled the run (faf32,
// 9d5d). The ways this fix can fail, written before the code:
//
//  1. A job that ended during the model call that ended the turn is still
//     charged as a guard retry (the bug).
//  2. A job outcome already given to the model is given again, so the
//     guard never charges and a run that just stops talking never stops.
//  3. A turn that ends with no job and no fate is no longer charged.
//  4. A model that ends every turn while fresh jobs keep ending loops
//     without a bound.
//  5. A job that had ended before the loop began is resumed as new.
//  6. Two undelivered jobs take two resumes, or one of them is lost.

// guardJobTable is a run's jobs in start order; at most one runs.
type guardJobTable struct {
	mu   sync.Mutex
	jobs []yaegikernel.JobView
}

func (g *guardJobTable) RunningJob(string) (yaegikernel.JobView, time.Time, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, v := range g.jobs {
		if v.State == capsulejob.StateRunning {
			return v, time.Now().Add(time.Minute), true
		}
	}
	return yaegikernel.JobView{}, time.Time{}, false
}

func (g *guardJobTable) JobViews(string) []yaegikernel.JobView {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]yaegikernel.JobView(nil), g.jobs...)
}

// ran records a job that started and ended (done, exit 0) between two
// looks of the guard, as a probe or a capture does in 2-3 s.
func (g *guardJobTable) ran(id string) {
	exit := 0
	g.mu.Lock()
	defer g.mu.Unlock()
	g.jobs = append(g.jobs, yaegikernel.JobView{ID: id, State: capsulejob.StateDone, ExitCode: &exit,
		Argv: []string{"choir-capture", "--route", "/"}, ElapsedSec: 2, ReceiptRef: "capsule-exec:sha256:" + id, Tail: "captured " + id})
}

// endTurnProvider ends every turn with text and no tool call (the end_turn
// the b@c engineers sent after a Start cell); before it answers call n it
// runs during(n), which is what happened while the model was thinking.
type endTurnProvider struct {
	provideriface.Provider
	mu       sync.Mutex
	during   func(call int)
	requests []provideriface.ToolLoopRequest
}

func (p *endTurnProvider) CallWithTools(_ context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	p.mu.Lock()
	p.requests = append(p.requests, req)
	call := len(p.requests)
	p.mu.Unlock()
	if p.during != nil {
		p.during(call)
	}
	return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "Waiting for the job.", Model: "test-model"}, nil
}

func (p *endTurnProvider) lastMessage(call int) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	msgs := p.requests[call-1].Messages
	return string(msgs[len(msgs)-1])
}

type guardRetry struct {
	Attempt int    `json:"attempt"`
	Reason  string `json:"reason"`
}

// runEngineeringGuardLoop runs the tool loop with the engineering desk's
// completion guard over table and returns the loop's error and the guard's
// loop.retry events.
func runEngineeringGuardLoop(t *testing.T, rt *Runtime, table *guardJobTable, provider *endTurnProvider) (error, []guardRetry) {
	t.Helper()
	rt.jobTable = table
	rec := &types.RunRecord{RunID: "run-1"}
	var mu sync.Mutex
	var retries []guardRetry
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		if kind != types.EventRunRetry || phase != "completion_guard" {
			return
		}
		var r guardRetry
		_ = json.Unmarshal(payload, &r)
		mu.Lock()
		retries = append(retries, r)
		mu.Unlock()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, _, err := toolregistry.RunToolLoop(ctx, provider, toolregistry.NewToolRegistry(),
		[]json.RawMessage{json.RawMessage(`{"role":"user","content":"build, capture, Freeze, Complete"}`)}, "system", 1024, emit, nil,
		toolregistry.WithToolLoopLLMConfig(provideriface.LLMSelection{Provider: "opencode-go", Model: "deepseek-v4.1-flash"}),
		toolregistry.WithCompletionGuard(rt.engineeringCompletionGuard(rec, rt.newRunJobDeliveries(rec))))
	mu.Lock()
	defer mu.Unlock()
	return err, append([]guardRetry(nil), retries...)
}

// Failure modes 1 and 3, the b@c pattern: three end_turns each after a job
// that ended before the guard looked are three job-outcome resumes, not
// retries; the guard's two retries are then charged only by end_turns with
// no job, and the third such end_turn ends the run as today.
func TestGuardResumesAJobThatEndedBeforeItLooked(t *testing.T) {
	rt, _ := testRuntime(t)
	table := &guardJobTable{}
	provider := &endTurnProvider{during: func(call int) {
		if call <= 3 {
			table.ran(fmt.Sprintf("job-%d", call))
		}
	}}
	err, retries := runEngineeringGuardLoop(t, rt, table, provider)
	if err == nil || !strings.Contains(err.Error(), "not satisfied after 2 retries") {
		t.Fatalf("loop error = %v, want the guard's exhaustion after the job resumes", err)
	}
	if got := len(provider.requests); got != 6 {
		t.Fatalf("model calls = %d, want 6 (three job resumes, then two reminders, then the cancel)", got)
	}
	for call := 2; call <= 4; call++ {
		job := fmt.Sprintf("job-%d", call-1)
		msg := provider.lastMessage(call)
		for _, want := range []string{"Your capsule job " + job, "ended: state done, exit code 0", "capsule-exec:sha256:" + job, "captured " + job} {
			if !strings.Contains(msg, want) {
				t.Fatalf("call %d resumes with %s, lacking %q", call, msg, want)
			}
		}
	}
	for call := 5; call <= 6; call++ {
		if msg := provider.lastMessage(call); !strings.Contains(msg, "without a committed terminal fate") {
			t.Fatalf("call %d: %s, want the reminder", call, msg)
		}
	}
	want := []guardRetry{{0, jobOutcomeUndelivered}, {0, jobOutcomeUndelivered}, {0, jobOutcomeUndelivered},
		{1, engineeringTerminalFateRequired}, {2, engineeringTerminalFateRequired}}
	if fmt.Sprint(retries) != fmt.Sprint(want) {
		t.Fatalf("guard events = %v, want %v", retries, want)
	}
}

// Failure mode 3: a turn that ends with no job and no fate is charged, and
// the third ends the run, as before the fix.
func TestGuardChargesATurnWithNoJob(t *testing.T) {
	rt, _ := testRuntime(t)
	provider := &endTurnProvider{}
	err, retries := runEngineeringGuardLoop(t, rt, &guardJobTable{}, provider)
	if err == nil || !strings.Contains(err.Error(), "not satisfied after 2 retries") {
		t.Fatalf("loop error = %v", err)
	}
	if len(provider.requests) != 3 || len(retries) != 2 || retries[0].Attempt != 1 || retries[1].Attempt != 2 {
		t.Fatalf("calls %d, retries %v; want 3 calls and two charged retries", len(provider.requests), retries)
	}
}

// Failure mode 4: a model that ends every turn while a fresh job ends each
// time is stopped by the guard's wait bound, not let loop.
func TestGuardJobOutcomeResumesAreBounded(t *testing.T) {
	rt, _ := testRuntime(t)
	table := &guardJobTable{}
	provider := &endTurnProvider{during: func(call int) { table.ran(fmt.Sprintf("job-%d", call)) }}
	err, retries := runEngineeringGuardLoop(t, rt, table, provider)
	if err == nil || !strings.Contains(err.Error(), "waited") || strings.Contains(err.Error(), "not satisfied") {
		t.Fatalf("loop error = %v, want the wait bound", err)
	}
	if len(provider.requests) <= 3 {
		t.Fatalf("model calls = %d: job resumes were charged as retries", len(provider.requests))
	}
	for _, r := range retries {
		if r.Attempt != 0 {
			t.Fatalf("a job resume was charged: %v", retries)
		}
	}
}

// Failure modes 2, 5 and 6, at the guard's wait: two undelivered jobs make
// one resume naming both; a job a park resume already delivered, or one
// that had ended before the loop began, is not resumed again, so the guard
// falls through to its charged reminder.
func TestGuardDeliversEachJobOutcomeOnce(t *testing.T) {
	rt, _ := testRuntime(t)
	table := &guardJobTable{}
	table.ran("job-old")
	rt.jobTable = table
	rec := &types.RunRecord{RunID: "run-1"}
	deliveries := rt.newRunJobDeliveries(rec)
	if _, ok := rt.engineeringJobWait(context.Background(), rec, deliveries); ok {
		t.Fatal("resumed a job that had ended before the loop began")
	}
	table.ran("job-a")
	table.ran("job-b")
	got, ok := rt.engineeringJobWait(context.Background(), rec, deliveries)
	if !ok || !got.Continue || !got.Waited || got.Reason != jobOutcomeUndelivered ||
		!strings.Contains(got.Instruction, "job-a") || !strings.Contains(got.Instruction, "job-b") || strings.Contains(got.Instruction, "job-old") {
		t.Fatalf("resume = %+v %v, want one resume naming job-a and job-b", got, ok)
	}
	if _, ok := rt.engineeringJobWait(context.Background(), rec, deliveries); ok {
		t.Fatal("resumed the same outcomes twice")
	}
	// A park resume delivers its job: the guard does not resume it again.
	table.ran("job-parked")
	calls, results := evalCall(`{"stdout":"running","job_park":"job-parked"}`, false)
	if res, ok, err := rt.engineeringJobPark(context.Background(), rec, deliveries, nil, calls, results); err != nil || !ok || !strings.Contains(res.Instruction, "job-parked") {
		t.Fatalf("park = %+v %v %v", res, ok, err)
	}
	if _, ok := rt.engineeringJobWait(context.Background(), rec, deliveries); ok {
		t.Fatal("the guard resumed a job the park had delivered")
	}
}
