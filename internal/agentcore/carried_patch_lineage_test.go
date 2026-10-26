package agentcore

// F1 (docs/problems/settled-document-refuses-owner-revision-2026-10-26.md):
// when a settled document is revised, the owner's revise starts a new
// trajectory that continues the settled one (subject ref
// continues_trajectory_id, store.LifecycleTrajectoryLineage). The carry reads
// a trajectory's earlier rounds; it must read the rounds of the trajectories
// the new one continues too, or the next round starts cold (T1a Reader round
// 2 -> round 3). Ways it could fail, written before the code:
//
//  L1. The next trajectory's history does not include the settled
//      trajectory's rounds (the carried work is lost).
//  L2. A trajectory that continues nothing gains rounds it does not own.
//  L3. A failed lineage read is swallowed into an empty history (the offer
//      would be decided "nothing to carry" and remembered).
//
// The lineage itself (one document only, bounded, cycle-safe) is
// internal/store's (TestLifecycleTrajectoryLineage).

import (
	"context"
	"errors"
	"testing"
)

func fixedLineage(chain map[string][]string) func(context.Context, string, string, string) ([]string, error) {
	return func(_ context.Context, _, _, trajectoryID string) ([]string, error) {
		if got, ok := chain[trajectoryID]; ok {
			return got, nil
		}
		return []string{trajectoryID}, nil
	}
}

// L1, L2.
func TestCarriedHistoryFollowsTheContinuedTrajectories(t *testing.T) {
	f := newUnfinishedRoundFixture(t, testCarriedPatch("packages/reader/reader.go"), true)
	f.open(0)
	first := f.bind(0)
	f.stopByBudget(t, first)
	settled := first.Binding.TrajectoryID
	if len(readUnfinishedRounds(f.root, settled)) != 1 {
		t.Fatal("fixture: the stopped round kept nothing")
	}
	owner, computer := first.Binding.OwnerID, first.Binding.ComputerID
	history := storeCarriedHistory{rt: f.rt, lineage: fixedLineage(map[string][]string{
		"trajectory-next":  {"trajectory-next", settled},
		"trajectory-after": {"trajectory-after", "trajectory-next", settled},
	})}
	for _, trajectoryID := range []string{"trajectory-next", "trajectory-after"} {
		entries, err := history.CarriedCandidates(context.Background(), owner, computer, trajectoryID)
		if err != nil {
			t.Fatalf("L1: history of %s: %v", trajectoryID, err)
		}
		found := 0
		for _, entry := range entries {
			if entry.AssignmentID == first.AssignmentID && entry.Unfinished {
				found++
			}
		}
		if found != 1 {
			t.Fatalf("L1: %s sees the settled trajectory's kept round %d times: %+v", trajectoryID, found, entries)
		}
	}
	entries, err := history.CarriedCandidates(context.Background(), owner, computer, "trajectory-alone")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("L2: a trajectory that continues nothing gained %+v", entries)
	}
}

// L3.
func TestCarriedHistoryReportsALineageReadFailure(t *testing.T) {
	f := newUnfinishedRoundFixture(t, testCarriedPatch("a.go"), true)
	broken := errors.New("lineage read failed")
	history := storeCarriedHistory{rt: f.rt, lineage: func(context.Context, string, string, string) ([]string, error) { return nil, broken }}
	if _, err := history.CarriedCandidates(context.Background(), f.seed.OwnerID, f.computer, f.seed.TrajectoryID); !errors.Is(err, broken) {
		t.Fatalf("L3: lineage failure err=%v, want it reported", err)
	}
}
