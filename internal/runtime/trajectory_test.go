package runtime

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestSpawnMintsTrajectoryAndChildJoinsIt(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)

	root, err := rt.StartRunWithMetadata(ctx, "build a document", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileConductor,
		runMetadataAgentRole:    AgentProfileConductor,
	})
	if err != nil {
		t.Fatalf("start root run: %v", err)
	}
	if root.TrajectoryID == "" {
		t.Fatalf("root run has no trajectory_id column value: %+v", root)
	}
	if root.TrajectoryID != trajectoryIDForRun(root) {
		t.Fatalf("column %q != metadata %q", root.TrajectoryID, trajectoryIDForRun(root))
	}

	trajectory, err := s.GetTrajectory(ctx, "user-alice", root.TrajectoryID)
	if err != nil {
		t.Fatalf("trajectory record not minted: %v", err)
	}
	if trajectory.Kind != types.TrajectoryKindDocument || trajectory.Status != types.TrajectoryLive {
		t.Fatalf("unexpected trajectory: %+v", trajectory)
	}
	if !trajectory.SettlementRule.RequireNoOpenWorkItems {
		t.Fatalf("settlement rule not stored as data: %+v", trajectory.SettlementRule)
	}
	if trajectory.SubjectRefs["root_loop_id"] != root.RunID {
		t.Fatalf("subject refs missing root run: %+v", trajectory.SubjectRefs)
	}

	// The stored run row carries the trajectory_id column.
	stored, err := s.GetRun(ctx, root.RunID)
	if err != nil {
		t.Fatalf("get stored run: %v", err)
	}
	if stored.TrajectoryID != root.TrajectoryID {
		t.Fatalf("stored trajectory_id = %q, want %q", stored.TrajectoryID, root.TrajectoryID)
	}

	// A spawned run joins the same trajectory: same ID, no second record,
	// and the original kind survives even though the spawned profile
	// differs. (StartChildRun is the pre-M3 spawn API; the provenance edge
	// it records is spawned_by, not a control relationship.)
	spawned, err := rt.StartChildRun(ctx, root.RunID, "research the topic", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileResearcher,
		runMetadataAgentRole:    AgentProfileResearcher,
	})
	if err != nil {
		t.Fatalf("start spawned run: %v", err)
	}
	if spawned.TrajectoryID != root.TrajectoryID {
		t.Fatalf("spawned-run trajectory %q != root trajectory %q", spawned.TrajectoryID, root.TrajectoryID)
	}
	listed, err := s.ListTrajectoriesByOwner(ctx, "user-alice", 10)
	if err != nil {
		t.Fatalf("list trajectories: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("trajectories = %d, want 1 (a spawned run must not mint a second)", len(listed))
	}
	if listed[0].Kind != types.TrajectoryKindDocument {
		t.Fatalf("spawned run changed trajectory kind: %+v", listed[0])
	}
}

func TestProcessorSpawnMintsPublicationTrajectory(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)

	run, err := rt.StartRunWithMetadata(ctx, "ingest source handoff", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileProcessor,
		runMetadataAgentRole:    AgentProfileProcessor,
		runMetadataProcessorKey: "processor:global_firehose:global:gdelt",
	})
	if err != nil {
		t.Fatalf("start processor run: %v", err)
	}
	trajectory, err := s.GetTrajectory(ctx, "user-alice", run.TrajectoryID)
	if err != nil {
		t.Fatalf("trajectory record not minted: %v", err)
	}
	if trajectory.Kind != types.TrajectoryKindPublication {
		t.Fatalf("processor trajectory kind = %s, want publication", trajectory.Kind)
	}
	if trajectory.SubjectRefs["processor_key"] != "processor:global_firehose:global:gdelt" {
		t.Fatalf("subject refs missing processor key: %+v", trajectory.SubjectRefs)
	}
	if len(trajectory.SettlementRule.RequiredSubjectRefs) == 0 {
		t.Fatalf("publication settlement rule missing required refs: %+v", trajectory.SettlementRule)
	}
}

func TestTrajectoryObligationsAnswersWaitingOn(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)

	run, err := rt.StartRunWithMetadata(ctx, "publish the cycle", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileProcessor,
		runMetadataAgentRole:    AgentProfileProcessor,
	})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	item, err := s.CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:              "user-alice",
		TrajectoryID:         run.TrajectoryID,
		Objective:            "select and verify the candidate story",
		ObjectiveFingerprint: "fp-obligation",
		CreatedByRunID:       run.RunID,
	})
	if err != nil {
		t.Fatalf("create work item: %v", err)
	}

	obligations, err := rt.TrajectoryObligations(ctx, "user-alice", run.TrajectoryID)
	if err != nil {
		t.Fatalf("trajectory obligations: %v", err)
	}
	if obligations.SettlementReady {
		t.Fatalf("trajectory with open work item reports settlement ready: %+v", obligations)
	}
	if len(obligations.OpenWorkItems) != 1 || obligations.OpenWorkItems[0].WorkItemID != item.WorkItemID {
		t.Fatalf("open work items = %+v, want the created item", obligations.OpenWorkItems)
	}
	// Publication kind also waits on its required subject ref.
	if len(obligations.WaitingOn) != 2 {
		t.Fatalf("waiting_on = %+v, want open-item + missing publish_ref", obligations.WaitingOn)
	}

	if _, err := s.UpdateWorkItemStatus(ctx, "user-alice", item.WorkItemID, types.WorkItemCompleted); err != nil {
		t.Fatalf("complete work item: %v", err)
	}
	obligations, err = rt.TrajectoryObligations(ctx, "user-alice", run.TrajectoryID)
	if err != nil {
		t.Fatalf("trajectory obligations after completion: %v", err)
	}
	if len(obligations.OpenWorkItems) != 0 {
		t.Fatalf("open work items after completion = %+v", obligations.OpenWorkItems)
	}
	// Still not ready: the publish_ref subject ref is missing — the rule
	// is evaluated as data, not satisfied by run state.
	if obligations.SettlementReady {
		t.Fatalf("publication trajectory settled without publish_ref: %+v", obligations)
	}
}

func TestEvaluateTrajectorySettlementIsPureDataEvaluation(t *testing.T) {
	rec := types.TrajectoryRecord{
		Status:         types.TrajectoryLive,
		SubjectRefs:    map[string]string{"publish_ref": "refs/publications/p-1"},
		SettlementRule: types.SettlementRule{RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"publish_ref"}},
	}
	if ready, waiting := EvaluateTrajectorySettlement(rec, 0); !ready || len(waiting) != 0 {
		t.Fatalf("satisfied rule not ready: ready=%v waiting=%v", ready, waiting)
	}
	if ready, _ := EvaluateTrajectorySettlement(rec, 3); ready {
		t.Fatalf("open work items did not block settlement")
	}
	rec.SubjectRefs = nil
	if ready, waiting := EvaluateTrajectorySettlement(rec, 0); ready || len(waiting) != 1 {
		t.Fatalf("missing required ref did not block settlement: waiting=%v", waiting)
	}
	rec.Status = types.TrajectorySettled
	if ready, _ := EvaluateTrajectorySettlement(rec, 0); ready {
		t.Fatalf("non-live trajectory reported ready to settle")
	}
}
