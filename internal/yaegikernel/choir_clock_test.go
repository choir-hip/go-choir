package yaegikernel

import (
	"testing"
	"time"
)

func TestContextCarriesEngineerClockOnlyWhileBound(t *testing.T) {
	s := &ChoirScope{computerID: "c", activationID: "a", slot: "implementation"}
	hooks := s.BindCell()
	if _, ok := s.Context()["elapsed_min"]; ok {
		t.Fatal("clock keys without a clock")
	}
	hooks.Begin(SessionFrame{Clock: &CellClock{ElapsedSec: 41*60 + 30, BudgetSec: 3600}})
	ctx := s.Context()
	if ctx["elapsed_min"] != "41" || ctx["budget_min"] != "60" || ctx["budget_left_min"] != "18" {
		t.Fatalf("context = %v", ctx)
	}
	if _, ok := ctx["revision"]; ok {
		t.Fatal("revision shown without one")
	}
	// The cell's own time since start is added (C2).
	s.clockAt = time.Now().Add(-90 * time.Second)
	if got := s.Context()["elapsed_min"]; got != "43" {
		t.Fatalf("elapsed_min after 90s in cell = %s, want 43", got)
	}
	// Queue and transit before the cell began count too: the anchor reading
	// wins when larger.
	hooks.Begin(SessionFrame{Clock: &CellClock{ElapsedSec: 60, BudgetSec: 3600, AnchorUnixMS: time.Now().Add(-10 * time.Minute).UnixMilli()}})
	if got := s.Context()["elapsed_min"]; got != "10" {
		t.Fatalf("anchor elapsed_min = %s, want 10", got)
	}
	hooks.Begin(SessionFrame{Clock: &CellClock{ElapsedSec: 4000, BudgetSec: 3600, Revision: 2}})
	ctx = s.Context()
	if ctx["budget_left_min"] != "0" || ctx["revision"] != "2" {
		t.Fatalf("over-budget context = %v", ctx)
	}
	hooks.End()
	if _, ok := s.Context()["elapsed_min"]; ok {
		t.Fatal("clock survived the cell")
	}
}
