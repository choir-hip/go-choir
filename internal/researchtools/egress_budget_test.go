package researchtools

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// R3r — the D2 cap boundary. Every host-mediated network call a research
// activation makes charges the per-activation egress ledger; desk workers
// carry an address-space cap. These tests pin both budget dimensions and
// the activation-key scoping.

func execCtxForRun(runID string) toolregistry.ExecutionContext {
	return toolregistry.ExecutionContext{
		RunID:     runID,
		RunRecord: &types.RunRecord{RunID: runID},
	}
}

func TestEgressBudgetLedgerTripsCallCap(t *testing.T) {
	ledger := NewEgressBudgetLedger(3, 0)
	ctx := execCtxForRun("run-a")
	for i := 0; i < 3; i++ {
		if err := ledger.chargeCall(ctx, "web_search"); err != nil {
			t.Fatalf("call %d under budget refused: %v", i+1, err)
		}
	}
	err := ledger.chargeCall(ctx, "web_search")
	if err == nil || !strings.Contains(err.Error(), "egress budget exhausted") {
		t.Fatalf("4th call must refuse at cap, got %v", err)
	}
	calls, _ := ledger.Usage(ctx)
	if calls != 3 {
		t.Fatalf("usage calls = %d, want 3 (refusal must not charge)", calls)
	}
}

func TestEgressBudgetLedgerTripsByteCap(t *testing.T) {
	ledger := NewEgressBudgetLedger(0, 1000)
	ctx := execCtxForRun("run-a")
	if err := ledger.chargeBytes(ctx, "fetch_url", 600); err != nil {
		t.Fatalf("600 bytes under 1000 budget refused: %v", err)
	}
	err := ledger.chargeBytes(ctx, "fetch_url", 600)
	if err == nil || !strings.Contains(err.Error(), "byte budget exceeded") {
		t.Fatalf("second 600B must refuse at cap, got %v", err)
	}
	_, fetched := ledger.Usage(ctx)
	if fetched != 600 {
		t.Fatalf("usage bytes = %d, want 600 (refusal must not charge)", fetched)
	}
}

func TestEgressBudgetLedgerKeysByActivation(t *testing.T) {
	ledger := NewEgressBudgetLedger(1, 0)
	a, b := execCtxForRun("run-a"), execCtxForRun("run-b")
	if err := ledger.chargeCall(a, "web_search"); err != nil {
		t.Fatal(err)
	}
	if err := ledger.chargeCall(a, "web_search"); err == nil {
		t.Fatal("run-a second call must refuse at cap 1")
	}
	// A distinct activation keeps its own budget — the cap is per
	// activation, not global.
	if err := ledger.chargeCall(b, "web_search"); err != nil {
		t.Fatalf("run-b call must be admitted under its own budget: %v", err)
	}
}

func TestEgressBudgetNilLedgerIsOpen(t *testing.T) {
	// deps.Egress == nil (tests only) must never refuse — production wires
	// a ledger always, so nil only opens unit-test paths.
	var d Dependencies
	if err := d.chargeEgressCall(context.Background(), "web_search"); err != nil {
		t.Fatalf("nil ledger refused: %v", err)
	}
	if err := d.chargeEgressBytes(context.Background(), "fetch_url", 1<<20); err != nil {
		t.Fatalf("nil ledger refused bytes: %v", err)
	}
}

// chargeEgressCall keys on the execution-context activation: two runs
// through the same ledger are independent; a nil RunRecord still keys on
// execCtx.RunID.
func TestEgressBudgetKeyFallsBackToRunID(t *testing.T) {
	ledger := NewEgressBudgetLedger(1, 0)
	ctx := context.Background()
	deps := Dependencies{Egress: ledger}
	runCtx := toolregistry.WithExecutionContext(ctx, toolregistry.ExecutionContext{RunID: "run-9"})
	if err := deps.chargeEgressCall(runCtx, "web_search"); err != nil {
		t.Fatal(err)
	}
	if err := deps.chargeEgressCall(runCtx, "web_search"); err == nil {
		t.Fatal("second call on run-9 must refuse at cap 1")
	}
	calls, _ := ledger.Usage(toolregistry.ExecutionContext{RunID: "run-9"})
	if calls != 1 {
		t.Fatalf("usage = %d, want 1", calls)
	}
}
