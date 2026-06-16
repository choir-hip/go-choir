package runtime

import (
	"context"
	"fmt"
	"testing"
	"time"

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
		runMetadataAgentProfile:        AgentProfileProcessor,
		runMetadataAgentRole:           AgentProfileProcessor,
		runMetadataProcessorKey:        "processor:global_firehose:global:gdelt",
		"ingestion_handoff_request_id": "processor-request-1",
		"source_network_request_id":    "processor-request-1",
		"source_item_ids":              []string{"srcitem-1", "srcitem-2"},
		"source_count":                 2,
		"continuity_ref":               "sourcecycled://processor/global/latest",
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
	if len(trajectory.SettlementRule.RequiredSubjectRefs) != 2 {
		t.Fatalf("publication settlement rule missing required refs: %+v", trajectory.SettlementRule)
	}
	workItems, err := s.ListWorkItemsByTrajectory(ctx, "user-alice", run.TrajectoryID, true)
	if err != nil {
		t.Fatalf("list processor work items: %v", err)
	}
	if len(workItems) != 3 {
		t.Fatalf("processor open work items = %+v, want request item + two source-item items", workItems)
	}
	sawRequest := false
	sawSourceItems := map[string]bool{}
	for _, item := range workItems {
		switch item.ObjectiveFingerprint {
		case wireProcessorDecisionWorkItemFingerprint(run.TrajectoryID):
			sawRequest = true
			if item.Details["kind"] != "wire_processor_request_resolution" || item.Details["request_id"] != "processor-request-1" {
				t.Fatalf("processor request decision details = %+v", item.Details)
			}
		case wireProcessorSourceItemDecisionWorkItemFingerprint(run.TrajectoryID, "srcitem-1"),
			wireProcessorSourceItemDecisionWorkItemFingerprint(run.TrajectoryID, "srcitem-2"):
			sourceItemID, _ := item.Details["source_item_id"].(string)
			sawSourceItems[sourceItemID] = true
			if item.Details["kind"] != "wire_source_item_resolution" || item.Details["request_id"] != "processor-request-1" {
				t.Fatalf("processor source-item decision details = %+v", item.Details)
			}
		default:
			t.Fatalf("unexpected processor work item = %+v", item)
		}
	}
	if !sawRequest || !sawSourceItems["srcitem-1"] || !sawSourceItems["srcitem-2"] {
		t.Fatalf("processor work items missing expected request/source items: %+v", workItems)
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
	autoItems, err := s.ListWorkItemsByTrajectory(ctx, "user-alice", run.TrajectoryID, true)
	if err != nil {
		t.Fatalf("list auto-opened work items: %v", err)
	}
	for _, item := range autoItems {
		if _, err := s.UpdateWorkItemStatus(ctx, "user-alice", item.WorkItemID, types.WorkItemCompleted); err != nil {
			t.Fatalf("complete auto-opened work item %s: %v", item.WorkItemID, err)
		}
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
	// Publication kind also waits on both required subject refs.
	if len(obligations.WaitingOn) != 3 {
		t.Fatalf("waiting_on = %+v, want open-item + missing publish_ref + missing edition_ref", obligations.WaitingOn)
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
	// Still not ready: publish_ref and edition_ref are missing — the rule
	// is evaluated as data, not satisfied by run state.
	if obligations.SettlementReady {
		t.Fatalf("publication trajectory settled without publish_ref: %+v", obligations)
	}
}

func TestCancelRunTrajectoryPersistsFallbackTrajectoryID(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	now := time.Now().UTC()
	trajectoryID := "traj-legacy-metadata-only"
	run := types.RunRecord{
		RunID:        "run-legacy-metadata-only",
		AgentID:      "agent-legacy",
		AgentProfile: AgentProfileTexture,
		AgentRole:    AgentProfileTexture,
		OwnerID:      "user-alice",
		SandboxID:    "sandbox-test",
		State:        types.RunPending,
		Prompt:       "legacy row with metadata trajectory",
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileTexture,
			runMetadataAgentRole:    AgentProfileTexture,
			runMetadataTrajectoryID: trajectoryID,
		},
	}
	if err := s.CreateRun(ctx, run); err != nil {
		t.Fatalf("create legacy run: %v", err)
	}

	cancelled, err := rt.CancelRunTrajectory(ctx, run.RunID, run.OwnerID)
	if err != nil {
		t.Fatalf("cancel trajectory: %v", err)
	}
	if len(cancelled) != 1 || cancelled[0] != run.RunID {
		t.Fatalf("cancelled = %+v, want only %s", cancelled, run.RunID)
	}
	stored, err := s.GetRun(ctx, run.RunID)
	if err != nil {
		t.Fatalf("get cancelled run: %v", err)
	}
	if stored.TrajectoryID != trajectoryID {
		t.Fatalf("stored trajectory_id = %q, want %q", stored.TrajectoryID, trajectoryID)
	}
	if stored.State != types.RunCancelled {
		t.Fatalf("state = %s, want cancelled", stored.State)
	}
	trajectory, err := s.GetTrajectory(ctx, run.OwnerID, trajectoryID)
	if err != nil {
		t.Fatalf("get trajectory: %v", err)
	}
	if trajectory.Status != types.TrajectoryCancelled {
		t.Fatalf("trajectory status = %s, want cancelled", trajectory.Status)
	}
}

func TestCancelRunTrajectoryDrainsMoreThanOneActivePage(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	now := time.Now().UTC()
	trajectoryID := "traj-cancel-many"
	if _, err := s.CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
		TrajectoryID:   trajectoryID,
		OwnerID:        "user-alice",
		Kind:           types.TrajectoryKindTask,
		Status:         types.TrajectoryLive,
		SettlementRule: types.SettlementRule{RequireNoOpenWorkItems: true},
	}); err != nil {
		t.Fatalf("create trajectory: %v", err)
	}
	const totalRuns = 1001
	for i := 0; i < totalRuns; i++ {
		runID := fmt.Sprintf("run-cancel-many-%04d", i)
		if err := s.CreateRun(ctx, types.RunRecord{
			RunID:        runID,
			AgentID:      fmt.Sprintf("agent-cancel-many-%04d", i),
			AgentProfile: AgentProfileCoSuper,
			AgentRole:    AgentProfileCoSuper,
			OwnerID:      "user-alice",
			SandboxID:    "sandbox-test",
			State:        types.RunPending,
			Prompt:       "pending trajectory activation",
			TrajectoryID: trajectoryID,
			CreatedAt:    now.Add(time.Duration(i) * time.Millisecond),
			UpdatedAt:    now.Add(time.Duration(i) * time.Millisecond),
			Metadata: map[string]any{
				runMetadataAgentProfile: AgentProfileCoSuper,
				runMetadataAgentRole:    AgentProfileCoSuper,
				runMetadataTrajectoryID: trajectoryID,
			},
		}); err != nil {
			t.Fatalf("create run %d: %v", i, err)
		}
	}

	cancelled, err := rt.CancelRunTrajectory(ctx, "run-cancel-many-0000", "user-alice")
	if err != nil {
		t.Fatalf("cancel trajectory: %v", err)
	}
	if len(cancelled) != totalRuns {
		t.Fatalf("cancelled count = %d, want %d", len(cancelled), totalRuns)
	}
	active, err := s.ListActiveRunsByTrajectory(ctx, "user-alice", trajectoryID, totalRuns+1)
	if err != nil {
		t.Fatalf("list active runs: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("active runs after cancellation = %d, want 0", len(active))
	}
}

func TestEvaluateTrajectorySettlementIsPureDataEvaluation(t *testing.T) {
	rec := types.TrajectoryRecord{
		Status:         types.TrajectoryLive,
		SubjectRefs:    map[string]string{"publish_ref": "refs/publications/p-1", "edition_ref": "refs/editions/e-1"},
		SettlementRule: types.SettlementRule{RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"publish_ref", "edition_ref"}},
	}
	if ready, waiting := EvaluateTrajectorySettlement(rec, 0); !ready || len(waiting) != 0 {
		t.Fatalf("satisfied rule not ready: ready=%v waiting=%v", ready, waiting)
	}
	if ready, _ := EvaluateTrajectorySettlement(rec, 3); ready {
		t.Fatalf("open work items did not block settlement")
	}
	rec.SubjectRefs = nil
	if ready, waiting := EvaluateTrajectorySettlement(rec, 0); ready || len(waiting) != 2 {
		t.Fatalf("missing required ref did not block settlement: waiting=%v", waiting)
	}
	rec.Status = types.TrajectorySettled
	if ready, _ := EvaluateTrajectorySettlement(rec, 0); ready {
		t.Fatalf("non-live trajectory reported ready to settle")
	}
}
