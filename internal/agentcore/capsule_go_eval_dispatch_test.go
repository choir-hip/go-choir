package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
)

// TestCapsuleGoEvalToolDispatchesToExecutorGoEval verifies that the
// capsule_go_eval tool's Func routes through toolCtx.Executor.GoEval (the same
// single-broker path as capsule_exec). On non-linux hosts the stub executor's
// GoEval returns an error, so this test asserts the tool reaches
// Executor.GoEval rather than short-circuiting or dispatching elsewhere.
func TestCapsuleGoEvalToolDispatchesToExecutorGoEval(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping dispatch test in short mode")
	}
	tool := newCapsuleGoEvalTool(nil)

	// CoSuper role with a passing obligation validator; the stub executor is a
	// zero-value *capsule.Executor on non-linux, whose GoEval returns an error.
	// We assert the error is the executor's go_eval error (proving dispatch).
	toolCtx := &CapsuleToolCtx{
		Executor:                  new(capsule.Executor),
		AgentRunID:                "run-dispatch",
		ComputerID:                "computer",
		Role:                      capsule.RoleCoSuper,
		CapsuleHandle:             "handle",
		ValidateCurrentObligation: func(ctx context.Context) error { return nil },
	}
	ctx := WithCapsuleCtx(context.Background(), toolCtx)

	raw, _ := json.Marshal(map[string]any{
		"source": `package main; import "fmt"; func main(){ fmt.Println("x") }`,
	})
	_, err := tool.Func(ctx, raw)
	if err == nil {
		t.Fatalf("expected capsule_go_eval to dispatch to Executor.GoEval and return an error, got nil")
	}
	// The tool must dispatch into the capsule executor path. On linux the real
	// Executor.GoEval resolves the capability and refuses (no capsules bound);
	// on darwin the stub returns the kernel-required error. Both prove the tool
	// reached Executor.GoEval rather than short-circuiting or exiting early.
	if !strings.Contains(err.Error(), "capsule operation refused") && !strings.Contains(err.Error(), "go_eval") {
		t.Fatalf("expected the executor's refusal/error (proving dispatch to Executor.GoEval), got: %v", err)
	}
}

// TestRecordAssignmentResultRejectsUnknownFields (settlement gate item 3, v1):
// unknown submitted fields and extensions fail closed at admission, before any
// identity, fate, or effect.
func TestRecordAssignmentResultRejectsUnknownFields(t *testing.T) {
	tool := newRecordAssignedCoSuperReportTool(nil)
	toolCtx := &CapsuleToolCtx{
		Executor:                  new(capsule.Executor),
		AgentRunID:                "run-unknown-fields",
		Role:                      capsule.RoleCoSuper,
		ValidateCurrentObligation: func(ctx context.Context) error { return nil },
	}
	ctx := WithCapsuleCtx(context.Background(), toolCtx)
	raw, _ := json.Marshal(map[string]any{
		"result": "completed", "verdict": "none", "summary": "done",
		"evidence_refs": []string{}, "execution_refs": []string{},
		"provider_call_id": "call-smuggled",
	})
	if _, err := tool.Func(ctx, raw); err == nil || !strings.Contains(err.Error(), "unknown or malformed field") {
		t.Fatalf("unknown field = %v, want fail-closed admission reject", err)
	}
}

func TestRecordAssignmentResultRejectsTrailingData(t *testing.T) {
	tool := newRecordAssignedCoSuperReportTool(nil)
	toolCtx := &CapsuleToolCtx{
		Executor:                  new(capsule.Executor),
		AgentRunID:                "run-trailing-data",
		Role:                      capsule.RoleCoSuper,
		ValidateCurrentObligation: func(ctx context.Context) error { return nil },
	}
	ctx := WithCapsuleCtx(context.Background(), toolCtx)
	raw := []byte(`{"result":"completed","verdict":"none","summary":"done","evidence_refs":[],"execution_refs":[]} {"smuggled":1}`)
	if _, err := tool.Func(ctx, raw); err == nil || !strings.Contains(err.Error(), "unexpected trailing data") {
		t.Fatalf("trailing data err = %v, want unexpected trailing data reject", err)
	}
}
