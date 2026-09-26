package yaegikernel

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// R4: the acting desk's commitment pack rides the cell frame like R3d's
// Doc snapshot — the scope binds it on Begin, choir.Pack() reads it, and
// End clears it with the rest of the cell state.

func TestChoirPackBindsFromCellFrame(t *testing.T) {
	_, _, scope, _ := testChoirFixture(t)
	pack := &types.ActingPack{
		AgentID: "desk-a",
		Items: []types.ActingPackItem{
			{RecordID: "act-1", Claim: "alpha wins", Discrepancy: types.DiscrepancyContradicted,
				ObservationExcerpt: "alpha lost", ResolvedAt: "2026-09-26T00:00:00Z"},
			{RecordID: "act-2", Claim: "beta holds", Discrepancy: types.DiscrepancyUnresolved},
		},
	}
	hooks := scope.BindCell()
	hooks.Begin(SessionFrame{ID: "f1", Source: "cell", Pack: pack})
	got := scope.Pack()
	if got.AgentID != "desk-a" || len(got.Items) != 2 {
		t.Fatalf("pack = %+v", got)
	}
	if got.Items[0].ObservationExcerpt != "alpha lost" || got.Items[1].Discrepancy != types.DiscrepancyUnresolved {
		t.Fatalf("pack items = %+v", got.Items)
	}
	hooks.End()
	// After the cell, the pack clears with inbox/doc.
	if cleared := scope.Pack(); len(cleared.Items) != 0 {
		t.Fatalf("pack leaked past cell end: %+v", cleared)
	}
}

func TestChoirPackUnboundScopeIsEmpty(t *testing.T) {
	_, _, scope, _ := testChoirFixture(t)
	got := scope.Pack()
	if got.AgentID != "" || len(got.Items) != 0 {
		t.Fatalf("unbound pack = %+v", got)
	}
}

func TestChoirPackIsExportedOnTheObservationTier(t *testing.T) {
	_, issuer, scope, _ := testChoirFixture(t)
	exports := scope.ChoirExports()["choir/choir"]
	if _, ok := exports["Pack"]; !ok {
		t.Fatal("Pack not exported on the observation tier")
	}
	// A research desk sees the same verb — the pack boundary is structural,
	// not per-desk policy.
	research, err := NewChoirScope(scope.broker, issuer, "computer-choir", "activation-r", 1, "research", "")
	if err != nil {
		t.Fatalf("research scope: %v", err)
	}
	if _, ok := research.ChoirExports()["choir/choir"]["Pack"]; !ok {
		t.Error("research scope missing Pack export")
	}
}
