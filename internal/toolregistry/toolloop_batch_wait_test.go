package toolregistry

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// ctxBoundProvider fails a call once its context ends, so a loop that never
// stops calling the model ends at the test's deadline.
type ctxBoundProvider struct{ *mockToolLoopProvider }

func (p ctxBoundProvider) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return p.mockToolLoopProvider.CallWithTools(ctx, req)
}

func batchWaitFixture(t *testing.T, output string) (*ToolRegistry, *mockToolLoopProvider) {
	t.Helper()
	registry := NewToolRegistry()
	if err := registry.Register(Tool{Name: "capsule_go_eval", Func: func(context.Context, json.RawMessage) (string, error) { return output, nil }}); err != nil {
		t.Fatal(err)
	}
	provider := newMockToolLoopProvider(
		&provideriface.ToolLoopResponse{StopReason: "tool_use", ToolCalls: []types.ToolCall{{ID: "call-1", Name: "capsule_go_eval", Arguments: json.RawMessage(`{"source":"choir.Poll(id)"}`)}}, Model: "m"},
		&provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "done", Model: "m"},
	)
	return registry, provider
}

// A batch wait that waited (a cell looked at a running job) is followed by
// the model only after the wait, with its instruction as the next turn: no
// model turn sits between the cell and the wait
// (problems/engineering-busy-polls-its-own-job-2026-10-24.md).
func TestToolBatchWaitHoldsTheNextModelTurn(t *testing.T) {
	registry, provider := batchWaitFixture(t, `{"job_park":"job-1"}`)
	waits, callsAtWait := 0, -1
	text, _, err := RunToolLoop(context.Background(), provider, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":"build"}`)}, "system", 0, noEmit, nil,
		WithToolBatchWait(func(_ context.Context, calls []types.ToolCall, results []types.ToolResult) (ToolLoopCompletionGuardResult, bool, error) {
			waits++
			callsAtWait = provider.CallCount()
			if len(calls) != 1 || len(results) != 1 || !strings.Contains(results[0].Output, "job-1") {
				t.Errorf("wait saw calls %v results %v", calls, results)
			}
			return ToolLoopCompletionGuardResult{Continue: true, Waited: true, Reason: "capsule_job_ended", Instruction: "job-1 ended: state done"}, true, nil
		}))
	if err != nil || text != "done" {
		t.Fatalf("run: %q %v", text, err)
	}
	if waits != 1 || callsAtWait != 1 || provider.CallCount() != 2 {
		t.Fatalf("waits %d (at model call %d), model calls %d; want one wait after the first call, two calls", waits, callsAtWait, provider.CallCount())
	}
	last, _ := json.Marshal(provider.lastReq.Messages)
	if !strings.Contains(string(last), "job-1 ended: state done") {
		t.Fatalf("the resumed turn lacks the instruction: %s", last)
	}
}

// A batch wait that does not apply leaves the loop as it was; a terminal
// batch never reaches it.
func TestToolBatchWaitThatDoesNotApply(t *testing.T) {
	registry, provider := batchWaitFixture(t, `{"stdout":"ok"}`)
	waits := 0
	wait := WithToolBatchWait(func(context.Context, []types.ToolCall, []types.ToolResult) (ToolLoopCompletionGuardResult, bool, error) {
		waits++
		return ToolLoopCompletionGuardResult{}, false, nil
	})
	if _, _, err := RunToolLoop(context.Background(), provider, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":"x"}`)}, "system", 0, noEmit, nil, wait); err != nil {
		t.Fatal(err)
	}
	if waits != 1 || provider.CallCount() != 2 {
		t.Fatalf("waits %d, model calls %d; want 1 and 2", waits, provider.CallCount())
	}
	registry, provider = batchWaitFixture(t, `{"fate_terminal":true}`)
	waits = 0
	if _, _, err := RunToolLoop(context.Background(), provider, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":"x"}`)}, "system", 0, noEmit, nil, wait,
		WithTerminalToolResult("capsule_go_eval", func(out string) bool { return strings.Contains(out, "fate_terminal") })); err != nil {
		t.Fatal(err)
	}
	if waits != 0 || provider.CallCount() != 1 {
		t.Fatalf("terminal batch: waits %d, model calls %d; want 0 and 1", waits, provider.CallCount())
	}
}

// Batch waits share the completion guard's wait bound: a run cannot park
// forever, one job at a time.
func TestToolBatchWaitsAreBounded(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.Register(Tool{Name: "capsule_go_eval", Func: func(context.Context, json.RawMessage) (string, error) { return `{"job_park":"job-1"}`, nil }}); err != nil {
		t.Fatal(err)
	}
	provider := ctxBoundProvider{newMockToolLoopProvider(&provideriface.ToolLoopResponse{StopReason: "tool_use", ToolCalls: []types.ToolCall{{ID: "c", Name: "capsule_go_eval", Arguments: json.RawMessage(`{}`)}}, Model: "m"})}
	// Without the bound the loop would call the model forever; the deadline
	// turns that into a failure, not a hung test binary.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, _, err := RunToolLoop(ctx, provider, registry, []json.RawMessage{json.RawMessage(`{"role":"user","content":"x"}`)}, "system", 0, noEmit, nil,
		WithToolBatchWait(func(context.Context, []types.ToolCall, []types.ToolResult) (ToolLoopCompletionGuardResult, bool, error) {
			return ToolLoopCompletionGuardResult{Continue: true, Waited: true, Instruction: "again"}, true, nil
		}))
	if err == nil || !strings.Contains(err.Error(), "waited") {
		t.Fatalf("unbounded batch waits: %v", err)
	}
}
