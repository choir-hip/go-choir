package agentcore

import (
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// docs/problems/capsule-session-worker-dies-at-start-2026-10-09.md.
// Failure mode: an engineering run whose capsule worker never starts made
// ~400 model calls in 22 minutes; engineering runs had no tool-loop budget.
func TestEngineeringRunsHaveAToolLoopBudget(t *testing.T) {
	rec := &types.RunRecord{AgentID: "engineering:assignment-x", Metadata: map[string]any{}}
	budget := engineeringToolLoopBudget(rec)
	if budget.MaxProviderCalls != defaultEngineeringMaxProviderCalls || budget.MaxElapsed != defaultEngineeringMaxElapsed {
		t.Fatalf("engineering budget = %+v", budget)
	}
	if budget.MaxProviderCalls <= 0 || budget.MaxProviderCalls > 400 || budget.MaxElapsed <= 0 || budget.MaxElapsed > 2*time.Hour {
		t.Fatalf("engineering budget is unbounded or absurd: %+v", budget)
	}
	rec.Metadata["actor_budget_max_provider_calls"] = 500
	if got := engineeringToolLoopBudget(rec).MaxProviderCalls; got != 500 {
		t.Fatalf("run metadata override ignored: %d", got)
	}
}
