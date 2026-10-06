package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

// SR egress-refusal regression (kept per close-panel dissent): a desk_go_eval
// cell whose second choir.WebSearch exceeds a 1-call activation ledger must
// see the budget error returned inside the cell body — not as a tool-call
// error — and the ledger Usage must key to the activation RunID, not a
// shared bucket.
func TestDeskEgressRefusalIntoCell(t *testing.T) {
	deskTestWorkerBin(t)
	stub := &stubSearchClient{}
	ledger := researchtools.NewEgressBudgetLedger(1, 32<<20)
	rt := &Runtime{
		researchDeps: &researchtools.Dependencies{
			Search: stub,
			Egress: ledger,
		},
	}
	workers := newDeskSessionWorkers()
	tool := newDeskGoEvalTool(rt, workers, agentprofile.Research)
	execCtx := deskEvalExecCtx(t)
	execCtx.Profile = agentprofile.Research
	execCtx.Role = agentprofile.Research
	execCtx.RunID = "sr-probe-activation"
	ctx := toolregistry.WithExecutionContext(context.Background(), execCtx)

	src := `import "choir"
r1, e1 := choir.WebSearch("first", 1)
print("call1 err=", e1, " ok=", len(r1) > 0)
r2, e2 := choir.WebSearch("second", 1)
print("call2 err=", e2)`
	raw, _ := json.Marshal(src)
	out, err := tool.Func(ctx, json.RawMessage(`{"source":`+string(raw)+`,"timeout_ms":30000}`))
	if err != nil {
		t.Fatalf("desk_go_eval returned error: %v", err)
	}
	body := string(out)
	t.Logf("cell output:\n%s", body)
	if !strings.Contains(body, "activation egress budget exhausted") {
		t.Fatalf("budget error not returned into the cell: %s", body)
	}
	// The first call must still have succeeded: refusal is at the cap, not
	// before the first charge.
	if !strings.Contains(body, `call1 err= \u003cnil\u003e`) {
		t.Fatalf("first in-cell search should be admitted under a 1-call cap: %s", body)
	}
	calls, _ := ledger.Usage(toolregistry.ExecutionContext{RunID: "sr-probe-activation"})
	if calls != 1 {
		t.Fatalf("ledger Usage under the activation key = %d, want 1 (keying lost if this reads 0)", calls)
	}
	unknown, _ := ledger.Usage(toolregistry.ExecutionContext{})
	if unknown != 0 {
		t.Fatalf("ledger charged a non-activation bucket: unknown=%d", unknown)
	}
}