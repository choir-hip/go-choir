package agentcore

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Window A (docs/problems/paper-post-refresh-health-windows-2026-10-22.md):
// on a planned boot the paper's /health failed for 75 s while boot
// passivation ran one JSON scan of every work item, for the one parked run
// that had a requester. The scan re-ensured a spawned work item that the
// spawn had already created.
//
// Ways this could fail, written first: the phase still reads work items by
// a JSON path (cost grows with every spawn the computer ever made); it
// takes another JSON path over a kind instead; it mints a work item at
// boot; a marker boot closes the run instead of parking it; the park loses
// passivated_reason runtime_restarted, so the one resume
// (actorruntime handleInitialDispatch) no longer finds it.
func TestPlannedBootPassivationScansNoKind(t *testing.T) {
	rt, s := testRuntime(t)
	ownerID := "user-boot-passivation-cost"
	t.Setenv("CHOIR_OWNER_ID", ownerID)
	WithBootRestart(PlannedRestart{Reason: "platform_update", Target: "test"}, true)(rt)
	ctx := context.Background()
	at := time.Now().UTC().Add(-time.Hour)
	trajectoryID := "traj-boot-passivation-cost"
	if _, err := s.CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
		OwnerID: ownerID, TrajectoryID: trajectoryID, Kind: types.TrajectoryKindTask, Status: types.TrajectoryLive,
		SettlementRule: types.SettlementRule{Version: types.LifecycleReducerVersion},
	}); err != nil {
		t.Fatalf("create trajectory: %v", err)
	}
	run := func(id, profile string, state types.RunState, requester string) types.RunRecord {
		agentID := profile + ":" + id
		return types.RunRecord{RunID: id, AgentID: agentID, TrajectoryID: trajectoryID, AgentProfile: profile, AgentRole: profile,
			OwnerID: ownerID, ComputerID: rt.cfg.ComputerID, State: state, Prompt: "work " + id, RequestedByRunID: requester,
			CreatedAt: at, UpdatedAt: at, Metadata: map[string]any{runMetadataAgentProfile: profile, runMetadataAgentRole: profile,
				runMetadataAgentID: agentID, runMetadataTrajectoryID: trajectoryID}}
	}
	parent := run("boot-cost-parent", agentprofile.Management, types.RunCompleted, "")
	if err := s.CreateRun(ctx, parent); err != nil {
		t.Fatalf("create parent: %v", err)
	}
	// The accumulated input: work items from earlier spawns.
	for i := 0; i < 200; i++ {
		if _, err := s.CreateWorkItem(ctx, types.WorkItemRecord{OwnerID: ownerID, TrajectoryID: trajectoryID,
			Objective: fmt.Sprintf("earlier spawn %d", i), ObjectiveFingerprint: fmt.Sprintf("spawned_coagent:earlier-%d", i),
			Status: types.WorkItemCompleted}); err != nil {
			t.Fatalf("create earlier work item %d: %v", i, err)
		}
	}
	child := run("boot-cost-child", agentprofile.Research, types.RunRunning, parent.RunID)
	// The spawn created the child's work item, as production spawns do.
	if item, err := rt.ensureSpawnedCoagentWorkItem(ctx, &child, &parent, "spawned_work_item_id"); err != nil || item.WorkItemID == "" {
		t.Fatalf("spawn-time work item: %+v %v", item, err)
	}
	parked := []types.RunRecord{child,
		run("boot-cost-texture", agentprofile.Texture, types.RunRunning, ""),
		run("boot-cost-orphan", agentprofile.Research, types.RunPending, "")}
	for _, rec := range parked {
		if err := s.CreateRun(ctx, rec); err != nil {
			t.Fatalf("create %s: %v", rec.RunID, err)
		}
	}
	itemsBefore, err := s.ListWorkItemsByTrajectory(ctx, ownerID, trajectoryID, false)
	if err != nil {
		t.Fatalf("list work items: %v", err)
	}

	before := s.EngineMutexStats()
	rt.passivateInterruptedActivations(ctx)
	after := s.EngineMutexStats()

	for op, st := range after {
		if st.Calls == before[op].Calls {
			continue
		}
		if strings.HasSuffix(op, " json") || strings.Contains(op, "Snapshot") {
			t.Errorf("boot passivation scanned a kind: %q ran %d times", op, st.Calls-before[op].Calls)
		}
	}
	itemsAfter, err := s.ListWorkItemsByTrajectory(ctx, ownerID, trajectoryID, false)
	if err != nil {
		t.Fatalf("list work items: %v", err)
	}
	if len(itemsAfter) != len(itemsBefore) {
		t.Errorf("boot passivation minted work: %d work items before, %d after", len(itemsBefore), len(itemsAfter))
	}
	for _, want := range parked {
		got, err := s.GetRun(ctx, want.RunID)
		if err != nil {
			t.Fatalf("get %s: %v", want.RunID, err)
		}
		if got.State != types.RunPassivated || metadataStringValue(got.Metadata, "passivated_reason") != "runtime_restarted" {
			t.Errorf("run %s after a planned boot: state=%q reason=%q, want passivated with runtime_restarted",
				want.RunID, got.State, metadataStringValue(got.Metadata, "passivated_reason"))
		}
	}
}
