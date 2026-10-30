package agentcore

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Cut 1 landing L1 (docs/archive/definitions/choir-reeng-cut1-authority-2026-10-12.md,
// "Restart"): a crash restart never resumes work. At a markerless boot every
// pre-boot run that is pending, running, parked (passivated) or blocked
// becomes terminal with the fate interrupted_by_restart, and boot creates no
// work. The Cut 0 tracer saw the opposite on staging: the in-flight run was
// passivated, and boot opened a spawned work item for a pre-boot Texture run
// one second after the boot (c0-accept-1, invariant I3).
//
// Ways this could fail, written first: only pending and running are scanned,
// so a parked or blocked run stays open; the run is closed but with no named
// fate, so I2 cannot tell it from a lost run; boot still creates a spawned
// work item for the closed run; a planned boot is also closed, so the one
// resume the restart rule allows is lost; the owner-scoped list (production
// computers) is not the path exercised.
func TestCrashBootClosesPreBootRunsWithFate(t *testing.T) {
	rt, s := testRuntime(t)
	ownerID := "user-crash-boot"
	t.Setenv("CHOIR_OWNER_ID", ownerID)
	runs := seedPreBootRuns(t, rt, ownerID)
	before := countWorkItems(t, rt, ownerID)

	rt.Start(context.Background())

	for _, want := range runs {
		got, err := s.GetRun(context.Background(), want.RunID)
		if err != nil {
			t.Fatalf("get run %s: %v", want.RunID, err)
		}
		if got.State != types.RunCancelled || got.Error != runFateInterruptedByRestart {
			t.Fatalf("pre-boot %s run %s after a crash boot: state=%q error=%q, want cancelled with %q",
				want.State, want.RunID, got.State, got.Error, runFateInterruptedByRestart)
		}
		if got.FinishedAt == nil {
			t.Fatalf("closed run %s has no finished_at", want.RunID)
		}
	}
	if after := countWorkItems(t, rt, ownerID); after != before {
		t.Fatalf("a crash boot created work: %d work items before, %d after", before, after)
	}
}

// An old open run behind more than one header page of newer finished runs is
// still closed: the owner-scoped list filters states after its limit, so a
// limited scan misses it (agentic-consensus 2026-10-13, codex).
func TestCrashBootClosesOpenRunBehindManyFinishedRuns(t *testing.T) {
	rt, s := testRuntime(t)
	ownerID := "user-crash-boot-deep"
	t.Setenv("CHOIR_OWNER_ID", ownerID)
	ctx := context.Background()
	base := time.Now().UTC().Add(-time.Hour)
	old := types.RunRecord{RunID: "crash-boot-deep-open", AgentID: "research:deep", AgentProfile: agentprofile.Research,
		AgentRole: agentprofile.Research, OwnerID: ownerID, ComputerID: rt.cfg.ComputerID, State: types.RunRunning,
		Prompt: "old open work", CreatedAt: base, UpdatedAt: base}
	if err := s.CreateRun(ctx, old); err != nil {
		t.Fatalf("create old run: %v", err)
	}
	for i := 0; i < 1030; i++ {
		at := base.Add(time.Duration(i+1) * time.Second)
		done := types.RunRecord{RunID: fmt.Sprintf("crash-boot-deep-done-%04d", i), AgentID: "research:deep", AgentProfile: agentprofile.Research,
			AgentRole: agentprofile.Research, OwnerID: ownerID, ComputerID: rt.cfg.ComputerID, State: types.RunCompleted,
			Prompt: "finished", CreatedAt: at, UpdatedAt: at, FinishedAt: &at}
		if err := s.CreateRun(ctx, done); err != nil {
			t.Fatalf("create finished run %d: %v", i, err)
		}
	}

	rt.Start(ctx)

	got, err := s.GetRun(ctx, old.RunID)
	if err != nil {
		t.Fatalf("get old run: %v", err)
	}
	if got.State != types.RunCancelled || got.Error != runFateInterruptedByRestart {
		t.Fatalf("old open run behind 1030 finished runs: state=%q error=%q, want cancelled with %q", got.State, got.Error, runFateInterruptedByRestart)
	}
}

// A planned restart (marker present) keeps today's one resume: the run is
// parked, not closed.
func TestPlannedBootDoesNotCloseRuns(t *testing.T) {
	rt, s := testRuntime(t)
	ownerID := "user-planned-boot"
	t.Setenv("CHOIR_OWNER_ID", ownerID)
	WithBootRestart(PlannedRestart{Reason: "platform_update", Target: "test"}, true)(rt)
	runs := seedPreBootRuns(t, rt, ownerID)

	rt.Start(context.Background())

	got, err := s.GetRun(context.Background(), runs[0].RunID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if got.State.Terminal() {
		t.Fatalf("a planned boot closed run %s: state=%q error=%q", got.RunID, got.State, got.Error)
	}
}

func seedPreBootRuns(t *testing.T, rt *Runtime, ownerID string) []types.RunRecord {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC().Add(-time.Minute)
	trajectoryID := "traj-crash-boot"
	if _, err := rt.store.CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
		OwnerID:        ownerID,
		TrajectoryID:   trajectoryID,
		Kind:           types.TrajectoryKindTask,
		Status:         types.TrajectoryLive,
		SettlementRule: types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true},
	}); err != nil {
		t.Fatalf("create trajectory: %v", err)
	}
	var out []types.RunRecord
	for i, c := range []struct {
		state   types.RunState
		profile string
	}{
		{types.RunRunning, agentprofile.Texture},
		{types.RunPending, agentprofile.Research},
		{types.RunPassivated, agentprofile.Research},
		{types.RunBlocked, agentprofile.Management},
	} {
		agentID := c.profile + ":crash-boot"
		rec := types.RunRecord{
			RunID:        "crash-boot-" + string(c.state),
			AgentID:      agentID,
			TrajectoryID: trajectoryID,
			AgentProfile: c.profile,
			AgentRole:    c.profile,
			OwnerID:      ownerID,
			ComputerID:   rt.cfg.ComputerID,
			State:        c.state,
			Prompt:       "pre-boot work",
			CreatedAt:    now.Add(time.Duration(i) * time.Second),
			UpdatedAt:    now.Add(time.Duration(i) * time.Second),
			Metadata: map[string]any{
				runMetadataAgentProfile: c.profile,
				runMetadataAgentRole:    c.profile,
				runMetadataAgentID:      agentID,
				runMetadataTrajectoryID: trajectoryID,
			},
		}
		if c.state == types.RunPassivated {
			rec.Metadata["passivated_reason"] = "idle_actor_passivate"
		}
		if err := rt.store.CreateRun(ctx, rec); err != nil {
			t.Fatalf("create %s run: %v", c.state, err)
		}
		out = append(out, rec)
	}
	return out
}

func countWorkItems(t *testing.T, rt *Runtime, ownerID string) int {
	t.Helper()
	items, err := rt.store.ListWorkItemsByTrajectory(context.Background(), ownerID, "traj-crash-boot", false)
	if err != nil {
		t.Fatalf("list work items: %v", err)
	}
	return len(items)
}
