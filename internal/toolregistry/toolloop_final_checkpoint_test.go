package toolregistry

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// A model that always ends its turn with no tool call, and an injector that
// always has a pending packet, must not loop: the final-checkpoint
// re-injection is bounded (paper-token-budget-2026-11-09, section 1.3).
func TestRunToolLoopFinalCheckpointInjectionIsBounded(t *testing.T) {
	provider := &modelFailingProvider{}
	finalCalls := 0
	inject := func(final bool) ([]json.RawMessage, error) {
		if !final {
			return nil, nil
		}
		finalCalls++
		return []json.RawMessage{json.RawMessage(`{"role":"user","content":"final checkpoint packet"}`)}, nil
	}
	_, _, err := RunToolLoop(context.Background(), provider, nil,
		[]json.RawMessage{json.RawMessage(`{"role":"user","content":"hello"}`)}, "system", 0, func(types.EventKind, string, json.RawMessage) {}, inject)
	if err != nil {
		t.Fatal(err)
	}
	// One injection, one model call that ends without tools, then the loop ends.
	if finalCalls != 1 {
		t.Fatalf("final checkpoint injector called %d times, want 1", finalCalls)
	}
	if got := len(provider.requests); got != 2 {
		t.Fatalf("model calls = %d, want 2", got)
	}
}

// A park waiter that reports ready while the injector has nothing left to
// inject (the packet was already delivered this activation) completes the loop
// instead of failing it.
func TestRunToolLoopParkContinueWithEmptyInjectionCompletes(t *testing.T) {
	provider := &modelFailingProvider{}
	inject := func(bool) ([]json.RawMessage, error) { return nil, nil }
	waiter := func(context.Context, ToolLoopParkState) (ToolLoopParkResult, error) {
		return ToolLoopParkResult{Continue: true, Reason: "update_coagent_signal"}, nil
	}
	_, _, err := RunToolLoop(context.Background(), provider, nil,
		[]json.RawMessage{json.RawMessage(`{"role":"user","content":"hello"}`)}, "system", 0, func(types.EventKind, string, json.RawMessage) {}, inject,
		WithParkWaiter(waiter))
	if err != nil {
		t.Fatalf("loop failed on a phantom ready: %v", err)
	}
	if got := len(provider.requests); got != 1 {
		t.Fatalf("model calls = %d, want 1", got)
	}
}

// A waiter that is always ready and an injector that always has a packet are
// bounded by the same final-checkpoint cap.
func TestRunToolLoopParkContinueInjectionIsBounded(t *testing.T) {
	provider := &modelFailingProvider{}
	calls := 0
	inject := func(bool) ([]json.RawMessage, error) {
		calls++
		return []json.RawMessage{json.RawMessage(`{"role":"user","content":"packet"}`)}, nil
	}
	waiter := func(context.Context, ToolLoopParkState) (ToolLoopParkResult, error) {
		return ToolLoopParkResult{Continue: true, Reason: "update_coagent_signal"}, nil
	}
	_, _, err := RunToolLoop(context.Background(), provider, nil,
		[]json.RawMessage{json.RawMessage(`{"role":"user","content":"hello"}`)}, "system", 0, func(types.EventKind, string, json.RawMessage) {}, inject,
		WithParkWaiter(waiter))
	if err != nil {
		t.Fatal(err)
	}
	if calls > maxFinalCheckpointInjections {
		t.Fatalf("injector called %d times, want at most %d", calls, maxFinalCheckpointInjections)
	}
}
