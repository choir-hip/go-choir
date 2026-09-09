package projectionbase

import (
	"errors"
	"testing"
)

func TestPlanRecoveryStagingShapeRefusesLifetimeReplay(t *testing.T) {
	_, err := PlanRecovery(false, 20000, true, 1, 148333)
	if !errors.Is(err, ErrBaseRefused) {
		t.Fatalf("stale W=1 against H=148333 must refuse, got %v", err)
	}
	_, err = PlanRecovery(false, 20000, true, 13, 148333)
	if !errors.Is(err, ErrBaseRefused) {
		t.Fatalf("stale W=13 against H=148333 must refuse, got %v", err)
	}
}

func TestPlanRecoveryConsumesNearHeadWatermark(t *testing.T) {
	plan, err := PlanRecovery(false, 20000, true, 148000, 148333)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Action != RecoveryRebase || plan.TailEvents != 333 || plan.StartSequence != 148000 {
		t.Fatalf("want rebase of 333 from W, got %#v", plan)
	}
}

func TestPlanRecoveryResumeWhenLocalAlreadyPastW(t *testing.T) {
	plan, err := PlanRecovery(false, 148300, true, 148000, 148333)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Action != RecoveryResume || plan.TailEvents != 33 || plan.StartSequence != 148300 {
		t.Fatalf("want resume of 33 from local, got %#v", plan)
	}
}

func TestPlanRecoveryEmptyStoreInstalls(t *testing.T) {
	plan, err := PlanRecovery(true, 0, true, 148000, 148333)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Action != RecoveryInstall || plan.TailEvents != 333 {
		t.Fatalf("want install of 333, got %#v", plan)
	}
}

func TestPlanRecoveryGenesisOnlyForEmptyUnchained(t *testing.T) {
	plan, err := PlanRecovery(true, 0, false, 0, 0)
	if err != nil || plan.Action != RecoveryGenesis {
		t.Fatalf("empty+no chain: %#v %v", plan, err)
	}
	if _, err := PlanRecovery(false, 20, false, 0, 0); !errors.Is(err, ErrBaseRefused) {
		t.Fatalf("non-empty without chain must refuse, got %v", err)
	}
}

func TestPlanRecoveryMissingBaseRefuses(t *testing.T) {
	if _, err := PlanRecovery(true, 0, true, 0, 42); !errors.Is(err, ErrBaseRefused) {
		t.Fatalf("missing W must refuse, got %v", err)
	}
}

func TestPlanRecoveryWHIsZeroTailInstall(t *testing.T) {
	plan, err := PlanRecovery(true, 0, true, 148333, 148333)
	if err != nil || plan.Action != RecoveryInstall || plan.TailEvents != 0 {
		t.Fatalf("W=H install: %#v %v", plan, err)
	}
}

func TestPlanRecoveryGenesisForZeroSeqOpenedStore(t *testing.T) {
	plan, err := PlanRecovery(false, 0, false, 0, 0)
	if err != nil || plan.Action != RecoveryGenesis {
		t.Fatalf("opened empty store with no chain must genesis, got %#v %v", plan, err)
	}
}

func TestPlanRecoveryZeroSeqWithFreshWRebases(t *testing.T) {
	plan, err := PlanRecovery(false, 0, true, 148000, 148333)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Action != RecoveryRebase || plan.TailEvents != 333 {
		t.Fatalf("opened empty store behind W must rebase, got %#v", plan)
	}
}
