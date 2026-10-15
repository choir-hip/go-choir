package toolregistry

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

// A completion guard that waited for an outside event (a capsule job's end,
// docs/design/capsule-long-exec-2026-10-15.md F18) resumes the turn with its
// instruction and is not counted against the guard's two retries; its waits
// have their own bound.
func TestCompletionGuardWaitsAreNotRetries(t *testing.T) {
	provider := &modelFailingProvider{fail: map[string]error{}}
	waits := 0
	_, _, err := RunToolLoop(context.Background(), provider, nil,
		[]json.RawMessage{json.RawMessage(`{"role":"user","content":"hello"}`)}, "system", 0, noEmit, nil,
		WithToolLoopLLMConfig(provideriface.LLMSelection{Provider: "opencode-go", Model: "deepseek-v4.1-flash"}),
		WithCompletionGuard(func(context.Context, ToolLoopCompletionState) (ToolLoopCompletionGuardResult, error) {
			waits++
			if waits <= 5 {
				return ToolLoopCompletionGuardResult{Continue: true, Waited: true, Reason: "capsule_job_ended", Instruction: "job ended"}, nil
			}
			return ToolLoopCompletionGuardResult{}, nil
		}))
	if err != nil {
		t.Fatalf("five waits failed the run: %v", err)
	}
	if len(provider.requests) != 6 {
		t.Fatalf("requests = %d, want 6 (one per resumed turn)", len(provider.requests))
	}
	last, _ := json.Marshal(provider.requests[5].Messages)
	if !strings.Contains(string(last), "job ended") {
		t.Fatalf("the resumed turn does not carry the instruction: %s", last)
	}

	provider = &modelFailingProvider{fail: map[string]error{}}
	_, _, err = RunToolLoop(context.Background(), provider, nil,
		[]json.RawMessage{json.RawMessage(`{"role":"user","content":"hello"}`)}, "system", 0, noEmit, nil,
		WithToolLoopLLMConfig(provideriface.LLMSelection{Provider: "opencode-go", Model: "deepseek-v4.1-flash"}),
		WithCompletionGuard(func(context.Context, ToolLoopCompletionState) (ToolLoopCompletionGuardResult, error) {
			return ToolLoopCompletionGuardResult{Continue: true, Waited: true, Instruction: "again"}, nil
		}))
	if err == nil || !strings.Contains(err.Error(), "waited") {
		t.Fatalf("unbounded waits: %v", err)
	}
}
