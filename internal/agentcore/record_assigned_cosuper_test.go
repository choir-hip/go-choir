package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
)

func TestRecordAssignedCoSuperReportEnforcesExecutionReceiptOnPass(t *testing.T) {
	tool := newRecordAssignedCoSuperReportTool(nil)

	toolCtx := &CapsuleToolCtx{
		Executor:                  new(capsule.Executor),
		AgentRunID:                "run-test",
		ComputerID:                "computer-test",
		Role:                      capsule.RoleCoSuper,
		CapsuleHandle:             "handle-test",
		ValidateCurrentObligation: func(ctx context.Context) error { return nil },
	}
	ctx := WithCapsuleCtx(context.Background(), toolCtx)

	// Test 1: Completed + Pass with empty execution_refs must fail closed
	emptyRefsInput, _ := json.Marshal(map[string]any{
		"result":         "completed",
		"verdict":        "pass",
		"summary":        "completed without execution receipts",
		"evidence_refs":  []string{},
		"execution_refs": []string{},
	})
	_, err := tool.Func(ctx, emptyRefsInput)
	if err == nil {
		t.Fatal("expected error for completed pass with empty execution_refs, got nil")
	}
	if !strings.Contains(err.Error(), "terminal completed pass requires at least one valid execution_ref") {
		t.Fatalf("expected terminal pass empty-refs error, got: %v", err)
	}

	// Test 2: Passing rlm:* intent token in execution_refs must fail closed with typed error
	intentRefInput, _ := json.Marshal(map[string]any{
		"result":         "completed",
		"verdict":        "pass",
		"summary":        "completed with intent token",
		"evidence_refs":  []string{},
		"execution_refs": []string{"rlm:complete:1"},
	})
	_, err = tool.Func(ctx, intentRefInput)
	if err == nil {
		t.Fatal("expected error for execution_refs with rlm:complete:1, got nil")
	}
	if !strings.Contains(err.Error(), "internal intent token, not an execution receipt") {
		t.Fatalf("expected intent token rejection error, got: %v", err)
	}
}
