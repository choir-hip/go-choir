package agentcore

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// TestCompleteIntentEnforcesExecutionReceiptOnPass proves the reducer's fate
// path keeps the retired tool's admission contract: a terminal completed pass
// without execution refs fails closed, and an rlm:* intent token is rejected
// as an execution receipt. Both checks fire before the fate saga, so a
// minimal run record and executor suffice.
func TestCompleteIntentEnforcesExecutionReceiptOnPass(t *testing.T) {
	newReduction := func() *rlmCallReduction {
		return &rlmCallReduction{
			active: true,
			rec:    &types.RunRecord{RunID: "run-test", OwnerID: "owner", ComputerID: "computer"},
			toolCtx: &CapsuleToolCtx{
				Executor: new(capsule.Executor),
			},
			scope: ReductionScope{RunID: "run-test"},
		}
	}

	// Completed + pass with empty execution_refs must fail closed.
	_, err := newReduction().commitCompleteIntent(context.Background(), yaegikernel.StagedIntent{
		Kind: yaegikernel.IntentComplete, Result: "completed", Verdict: "pass", Summary: "done",
	})
	if err == nil || !strings.Contains(err.Error(), "terminal completed pass requires at least one valid execution_ref") {
		t.Fatalf("empty refs err = %v, want terminal pass empty-refs reject", err)
	}

	// An rlm:* intent token in execution_refs must fail closed at receipt
	// resolution, never reaching the saga.
	_, err = newReduction().commitCompleteIntent(context.Background(), yaegikernel.StagedIntent{
		Kind: yaegikernel.IntentComplete, Result: "completed", Verdict: "pass", Summary: "done",
		ExecutionRefs: []string{"rlm:complete:1"},
	})
	if err == nil || !strings.Contains(err.Error(), "internal intent token, not an execution receipt") {
		t.Fatalf("intent token err = %v, want intent token rejection", err)
	}

	// An empty summary fails closed.
	_, err = newReduction().commitCompleteIntent(context.Background(), yaegikernel.StagedIntent{
		Kind: yaegikernel.IntentComplete, Result: "completed", Verdict: "pass",
	})
	if err == nil || !strings.Contains(err.Error(), "summary is required") {
		t.Fatalf("empty summary err = %v, want summary required", err)
	}
}
