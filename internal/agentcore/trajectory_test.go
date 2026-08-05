package agentcore

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/workitem"
)

func TestSpawnMintsTrajectoryAndChildJoinsIt(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)

	root, err := rt.StartRunWithMetadata(ctx, "build a document", "user-alice", map[string]any{
		runMetadataAgentProfile: agentprofile.Conductor,
		runMetadataAgentRole:    agentprofile.Conductor,
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
	// differs. (StartCoagentRun is the pre-M3 spawn API; the provenance edge
	// it records is spawned_by, not a control relationship.)
	spawned, err := rt.StartCoagentRun(ctx, root.RunID, "research the topic", "user-alice", map[string]any{
		runMetadataAgentProfile: agentprofile.Researcher,
		runMetadataAgentRole:    agentprofile.Researcher,
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
		runMetadataAgentProfile:        agentprofile.Processor,
		runMetadataAgentRole:           agentprofile.Processor,
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
		case workitem.ProcessorDecisionFingerprint(run.TrajectoryID):
			sawRequest = true
			if item.Details["kind"] != "wire_processor_request_resolution" || item.Details["request_id"] != "processor-request-1" {
				t.Fatalf("processor request decision details = %+v", item.Details)
			}
		case workitem.SourceItemDecisionFingerprint(run.TrajectoryID, "srcitem-1"),
			workitem.SourceItemDecisionFingerprint(run.TrajectoryID, "srcitem-2"):
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
		runMetadataAgentProfile: agentprofile.Processor,
		runMetadataAgentRole:    agentprofile.Processor,
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
		AgentProfile: agentprofile.Texture,
		AgentRole:    agentprofile.Texture,
		OwnerID:      "user-alice",
		SandboxID:    "sandbox-test",
		State:        types.RunPending,
		Prompt:       "legacy row with metadata trajectory",
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: agentprofile.Texture,
			runMetadataAgentRole:    agentprofile.Texture,
			runMetadataTrajectoryID: trajectoryID,
		},
	}
	sibling := run
	sibling.RunID = "run-legacy-metadata-sibling"
	sibling.AgentID = "agent-legacy-sibling"
	sibling.TrajectoryID = trajectoryID
	sibling.Prompt = "current-schema sibling on legacy-addressed trajectory"
	if err := s.CreateRunOG(ctx, run); err != nil {
		t.Fatalf("seed legacy run %s: %v", run.RunID, err)
	}
	if err := s.CreateRun(ctx, sibling); err != nil {
		t.Fatalf("create sibling run %s: %v", sibling.RunID, err)
	}

	cancelled, err := rt.CancelRunTrajectory(ctx, run.RunID, run.OwnerID)
	if err != nil {
		t.Fatalf("cancel trajectory: %v", err)
	}
	if len(cancelled) != 2 {
		t.Fatalf("cancelled = %+v, want both legacy trajectory runs", cancelled)
	}
	cancelledSet := map[string]bool{}
	for _, runID := range cancelled {
		cancelledSet[runID] = true
	}
	if !cancelledSet[run.RunID] || !cancelledSet[sibling.RunID] {
		t.Fatalf("cancelled = %+v, want %s and %s", cancelled, run.RunID, sibling.RunID)
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
	storedSibling, err := s.GetRun(ctx, sibling.RunID)
	if err != nil {
		t.Fatalf("get cancelled legacy sibling: %v", err)
	}
	if storedSibling.TrajectoryID != trajectoryID || storedSibling.State != types.RunCancelled {
		t.Fatalf("legacy sibling after cancel = %+v, want trajectory %s and cancelled", storedSibling, trajectoryID)
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
	if raceDetectorEnabled {
		t.Skip("scale regression exceeds the production drain deadline under race instrumentation")
	}

	ctx := context.Background()
	rt, s := testRuntime(t)
	now := time.Now().UTC()
	trajectoryID := "traj-cancel-many"
	if _, err := s.CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
		TrajectoryID:   trajectoryID,
		OwnerID:        "user-alice",
		Kind:           types.TrajectoryKindTask,
		Status:         types.TrajectoryLive,
		SettlementRule: types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true},
	}); err != nil {
		t.Fatalf("create trajectory: %v", err)
	}
	const totalRuns = 1001
	for i := 0; i < totalRuns; i++ {
		runID := fmt.Sprintf("run-cancel-many-%04d", i)
		if err := s.CreateRun(ctx, types.RunRecord{
			RunID:        runID,
			AgentID:      fmt.Sprintf("agent-cancel-many-%04d", i),
			AgentProfile: agentprofile.CoSuper,
			AgentRole:    agentprofile.CoSuper,
			OwnerID:      "user-alice",
			SandboxID:    "sandbox-test",
			State:        types.RunPending,
			Prompt:       "pending trajectory activation",
			TrajectoryID: trajectoryID,
			CreatedAt:    now.Add(time.Duration(i) * time.Millisecond),
			UpdatedAt:    now.Add(time.Duration(i) * time.Millisecond),
			Metadata: map[string]any{
				runMetadataAgentProfile: agentprofile.CoSuper,
				runMetadataAgentRole:    agentprofile.CoSuper,
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

func TestCancelTrajectoryIsOwnerScopedTerminalizesAuthorityAndActiveRuns(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	const (
		ownerID      = "user-alice"
		trajectoryID = "traj-owner-cancel"
		activeRunID  = "run-owner-cancel-active"
	)
	if _, err := s.CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
		TrajectoryID:   trajectoryID,
		OwnerID:        ownerID,
		Kind:           types.TrajectoryKindTask,
		Status:         types.TrajectoryLive,
		SettlementRule: types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true},
	}); err != nil {
		t.Fatalf("create trajectory: %v", err)
	}
	openItem, err := s.CreateWorkItem(ctx, types.WorkItemRecord{
		TrajectoryID: trajectoryID,
		OwnerID:      ownerID,
		Objective:    "cancel this open obligation",
	})
	if err != nil {
		t.Fatalf("create open work item: %v", err)
	}
	completedItem, err := s.CreateWorkItem(ctx, types.WorkItemRecord{
		TrajectoryID: trajectoryID,
		OwnerID:      ownerID,
		Objective:    "preserve this completed obligation",
	})
	if err != nil {
		t.Fatalf("create completed work item: %v", err)
	}
	if _, err := s.UpdateWorkItemStatus(ctx, ownerID, completedItem.WorkItemID, types.WorkItemCompleted); err != nil {
		t.Fatalf("complete work item: %v", err)
	}
	now := time.Now().UTC()
	if err := s.CreateRun(ctx, types.RunRecord{
		RunID:        activeRunID,
		AgentID:      "agent-owner-cancel-active",
		AgentProfile: agentprofile.CoSuper,
		AgentRole:    agentprofile.CoSuper,
		OwnerID:      ownerID,
		SandboxID:    "sandbox-test",
		State:        types.RunPending,
		Prompt:       "active trajectory activation",
		TrajectoryID: trajectoryID,
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: agentprofile.CoSuper,
			runMetadataAgentRole:    agentprofile.CoSuper,
			runMetadataTrajectoryID: trajectoryID,
		},
	}); err != nil {
		t.Fatalf("create active run: %v", err)
	}

	if _, _, err := rt.CancelTrajectory(ctx, trajectoryID, "user-bob"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("cross-owner cancel error = %v, want not found", err)
	}
	trajectory, err := s.GetTrajectory(ctx, ownerID, trajectoryID)
	if err != nil {
		t.Fatalf("get trajectory after rejected cancel: %v", err)
	}
	if trajectory.Status != types.TrajectoryLive {
		t.Fatalf("cross-owner cancel changed trajectory status to %s", trajectory.Status)
	}
	storedOpen, err := s.GetWorkItem(ctx, ownerID, openItem.WorkItemID)
	if err != nil {
		t.Fatalf("get open work item after rejected cancel: %v", err)
	}
	if storedOpen.Status != types.WorkItemOpen {
		t.Fatalf("cross-owner cancel changed open item status to %s", storedOpen.Status)
	}

	returnedTrajectory, cancelled, err := rt.CancelTrajectory(ctx, trajectoryID, ownerID)
	if err != nil {
		t.Fatalf("cancel trajectory: %v", err)
	}
	if len(cancelled) != 1 || cancelled[0] != activeRunID {
		t.Fatalf("cancelled run ids = %+v, want [%s]", cancelled, activeRunID)
	}
	if returnedTrajectory.Status != types.TrajectoryCancelled {
		t.Fatalf("returned trajectory status = %s, want cancelled", returnedTrajectory.Status)
	}
	trajectory, err = s.GetTrajectory(ctx, ownerID, trajectoryID)
	if err != nil {
		t.Fatalf("get cancelled trajectory: %v", err)
	}
	if trajectory.Status != types.TrajectoryCancelled {
		t.Fatalf("trajectory status = %s, want cancelled", trajectory.Status)
	}
	storedOpen, err = s.GetWorkItem(ctx, ownerID, openItem.WorkItemID)
	if err != nil {
		t.Fatalf("get cancelled work item: %v", err)
	}
	if storedOpen.Status != types.WorkItemCancelled {
		t.Fatalf("open work item status = %s, want cancelled", storedOpen.Status)
	}
	storedCompleted, err := s.GetWorkItem(ctx, ownerID, completedItem.WorkItemID)
	if err != nil {
		t.Fatalf("get completed work item: %v", err)
	}
	if storedCompleted.Status != types.WorkItemCompleted {
		t.Fatalf("completed work item status = %s, want completed", storedCompleted.Status)
	}
	storedRun, err := s.GetRun(ctx, activeRunID)
	if err != nil {
		t.Fatalf("get cancelled run: %v", err)
	}
	if storedRun.State != types.RunCancelled {
		t.Fatalf("active run state = %s, want cancelled", storedRun.State)
	}

	returnedTrajectory, cancelled, err = rt.CancelTrajectory(ctx, trajectoryID, ownerID)
	if err != nil {
		t.Fatalf("repeat trajectory cancellation: %v", err)
	}
	if len(cancelled) != 0 {
		t.Fatalf("repeat trajectory cancellation cancelled runs = %+v, want none", cancelled)
	}
	if returnedTrajectory.Status != types.TrajectoryCancelled {
		t.Fatalf("repeat returned trajectory status = %s, want cancelled", returnedTrajectory.Status)
	}
}

func TestCancelTrajectoryRetriesActivationDrainForAlreadyCancelledAuthority(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	const (
		ownerID      = "user-alice"
		trajectoryID = "traj-cancel-drain-retry"
		runID        = "run-cancel-drain-retry"
	)
	if _, err := s.CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
		TrajectoryID:   trajectoryID,
		OwnerID:        ownerID,
		Kind:           types.TrajectoryKindTask,
		Status:         types.TrajectoryLive,
		SettlementRule: types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true},
	}); err != nil {
		t.Fatalf("create trajectory: %v", err)
	}
	now := time.Now().UTC()
	if err := s.CreateRun(ctx, types.RunRecord{
		RunID:        runID,
		AgentID:      "agent-cancel-drain-retry",
		AgentProfile: agentprofile.CoSuper,
		AgentRole:    agentprofile.CoSuper,
		OwnerID:      ownerID,
		SandboxID:    "sandbox-test",
		State:        types.RunPending,
		Prompt:       "activation left behind after durable cancellation",
		TrajectoryID: trajectoryID,
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: agentprofile.CoSuper,
			runMetadataAgentRole:    agentprofile.CoSuper,
			runMetadataTrajectoryID: trajectoryID,
		},
	}); err != nil {
		t.Fatalf("create active run: %v", err)
	}
	if trajectory, err := s.CancelTrajectoryAuthority(ctx, ownerID, trajectoryID); err != nil {
		t.Fatalf("commit cancellation authority: %v", err)
	} else if trajectory.Status != types.TrajectoryCancelled {
		t.Fatalf("authority status = %s, want cancelled", trajectory.Status)
	}

	trajectory, cancelled, err := rt.CancelTrajectory(ctx, trajectoryID, ownerID)
	if err != nil {
		t.Fatalf("retry trajectory cancellation: %v", err)
	}
	if trajectory.Status != types.TrajectoryCancelled {
		t.Fatalf("returned trajectory status = %s, want cancelled", trajectory.Status)
	}
	if len(cancelled) != 1 || cancelled[0] != runID {
		t.Fatalf("cancelled run ids = %+v, want [%s]", cancelled, runID)
	}
	stored, err := s.GetRun(ctx, runID)
	if err != nil {
		t.Fatalf("get drained run: %v", err)
	}
	if stored.State != types.RunCancelled {
		t.Fatalf("drained run state = %s, want cancelled", stored.State)
	}
}

func TestCancelTrajectoryReturnsSettledTruthWithoutCancellingActivations(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	const (
		ownerID      = "user-alice"
		trajectoryID = "traj-settled-cancel-truth"
		runID        = "run-settled-cancel-truth"
	)
	if _, err := s.CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
		TrajectoryID:   trajectoryID,
		OwnerID:        ownerID,
		Kind:           types.TrajectoryKindTask,
		Status:         types.TrajectoryLive,
		SettlementRule: types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true},
	}); err != nil {
		t.Fatalf("create trajectory: %v", err)
	}
	if _, err := s.UpdateTrajectoryStatus(ctx, ownerID, trajectoryID, types.TrajectorySettled); err != nil {
		t.Fatalf("settle trajectory: %v", err)
	}
	// Seed a legacy activation that predates terminal-trajectory admission guards.
	now := time.Now().UTC()
	if err := s.CreateRunOG(ctx, types.RunRecord{
		RunID:        runID,
		AgentID:      "agent-settled-cancel-truth",
		AgentProfile: agentprofile.CoSuper,
		AgentRole:    agentprofile.CoSuper,
		OwnerID:      ownerID,
		SandboxID:    "sandbox-test",
		State:        types.RunPending,
		Prompt:       "activation associated with settled authority",
		TrajectoryID: trajectoryID,
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: agentprofile.CoSuper,
			runMetadataAgentRole:    agentprofile.CoSuper,
			runMetadataTrajectoryID: trajectoryID,
		},
	}); err != nil {
		t.Fatalf("create active run: %v", err)
	}

	trajectory, cancelled, err := rt.CancelTrajectory(ctx, trajectoryID, ownerID)
	if err != nil {
		t.Fatalf("cancel settled trajectory: %v", err)
	}
	if trajectory.Status != types.TrajectorySettled {
		t.Fatalf("returned trajectory status = %s, want settled", trajectory.Status)
	}
	if len(cancelled) != 0 {
		t.Fatalf("cancelled settled trajectory activations = %+v, want none", cancelled)
	}
	stored, err := s.GetRun(ctx, runID)
	if err != nil {
		t.Fatalf("get settled trajectory run: %v", err)
	}
	if stored.State != types.RunPending {
		t.Fatalf("settled trajectory run state = %s, want pending", stored.State)
	}
}

func TestCancelledRequestDoesNotInterruptPostAuthorityActivationDrain(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	rt, s := testRuntime(t)
	const (
		ownerID      = "user-alice"
		trajectoryID = "traj-detached-cancel-drain"
		runID        = "run-detached-cancel-drain"
	)
	if _, err := s.CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
		TrajectoryID:   trajectoryID,
		OwnerID:        ownerID,
		Kind:           types.TrajectoryKindTask,
		Status:         types.TrajectoryLive,
		SettlementRule: types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true},
	}); err != nil {
		t.Fatalf("create trajectory: %v", err)
	}
	now := time.Now().UTC()
	if err := s.CreateRun(ctx, types.RunRecord{
		RunID:        runID,
		AgentID:      "agent-detached-cancel-drain",
		AgentProfile: agentprofile.CoSuper,
		AgentRole:    agentprofile.CoSuper,
		OwnerID:      ownerID,
		SandboxID:    "sandbox-test",
		State:        types.RunPending,
		Prompt:       "activation drained after request cancellation",
		TrajectoryID: trajectoryID,
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: agentprofile.CoSuper,
			runMetadataAgentRole:    agentprofile.CoSuper,
			runMetadataTrajectoryID: trajectoryID,
		},
	}); err != nil {
		t.Fatalf("create active run: %v", err)
	}
	trajectory, err := rt.cancelTrajectoryAuthority(ctx, ownerID, trajectoryID)
	if err != nil {
		t.Fatalf("commit cancellation authority: %v", err)
	}
	if trajectory.Status != types.TrajectoryCancelled {
		t.Fatalf("authority status = %s, want cancelled", trajectory.Status)
	}
	cancel()

	cancelled, err := rt.drainCancelledTrajectoryActivations(ctx, ownerID, "sandbox-test", trajectoryID)
	if err != nil {
		t.Fatalf("drain after request cancellation: %v", err)
	}
	if len(cancelled) != 1 || cancelled[0] != runID {
		t.Fatalf("cancelled run ids = %+v, want [%s]", cancelled, runID)
	}
	stored, err := s.GetRun(context.Background(), runID)
	if err != nil {
		t.Fatalf("get drained run: %v", err)
	}
	if stored.State != types.RunCancelled {
		t.Fatalf("drained run state = %s, want cancelled", stored.State)
	}
}

func TestHandleTrajectoryDetailPreservesGETAndRoutesOwnerScopedCancellation(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	h := NewAPIHandler(rt)
	server := httptest.NewServer(http.HandlerFunc(h.HandleTrajectoryDetail))
	defer server.Close()

	doRequest := func(method, escapedPath, ownerID string) (int, []byte) {
		t.Helper()
		req, err := http.NewRequest(method, server.URL+escapedPath, nil)
		if err != nil {
			t.Fatalf("create %s %s request: %v", method, escapedPath, err)
		}
		req.Header.Set("X-Authenticated-User", ownerID)
		resp, err := server.Client().Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", method, escapedPath, err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read %s %s response: %v", method, escapedPath, err)
		}
		return resp.StatusCode, body
	}

	doRawRequest := func(requestTarget string) (int, []byte) {
		t.Helper()
		conn, err := net.Dial("tcp", server.Listener.Addr().String())
		if err != nil {
			t.Fatalf("dial test server: %v", err)
		}
		defer conn.Close()
		rawRequest := "POST " + requestTarget + " HTTP/1.1\r\nHost: example.test\r\nX-Authenticated-User: user-alice\r\nConnection: close\r\n\r\n"
		if _, err := io.WriteString(conn, rawRequest); err != nil {
			t.Fatalf("write raw request %q: %v", requestTarget, err)
		}
		resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodPost})
		if err != nil {
			t.Fatalf("read raw response for %q: %v", requestTarget, err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read raw response body for %q: %v", requestTarget, err)
		}
		return resp.StatusCode, body
	}

	const (
		ownerID      = "user-alice"
		trajectoryID = "traj/with space"
		activeRunID  = "run-api-cancel-active"
	)
	if _, err := s.CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
		TrajectoryID:   trajectoryID,
		OwnerID:        ownerID,
		Kind:           types.TrajectoryKindTask,
		Status:         types.TrajectoryLive,
		SettlementRule: types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true},
	}); err != nil {
		t.Fatalf("create trajectory: %v", err)
	}
	item, err := s.CreateWorkItem(ctx, types.WorkItemRecord{
		TrajectoryID: trajectoryID,
		OwnerID:      ownerID,
		Objective:    "owner-visible cancellation obligation",
	})
	if err != nil {
		t.Fatalf("create work item: %v", err)
	}
	now := time.Now().UTC()
	if err := s.CreateRun(ctx, types.RunRecord{
		RunID:        activeRunID,
		AgentID:      "agent-api-cancel-active",
		AgentProfile: agentprofile.CoSuper,
		AgentRole:    agentprofile.CoSuper,
		OwnerID:      ownerID,
		SandboxID:    "sandbox-test",
		State:        types.RunPending,
		Prompt:       "active API trajectory activation",
		TrajectoryID: trajectoryID,
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: agentprofile.CoSuper,
			runMetadataAgentRole:    agentprofile.CoSuper,
			runMetadataTrajectoryID: trajectoryID,
		},
	}); err != nil {
		t.Fatalf("create active run: %v", err)
	}

	escapedTrajectoryPath := "/api/trajectories/" + url.PathEscape(trajectoryID)
	status, body := doRequest(http.MethodGet, escapedTrajectoryPath, ownerID)
	if status != http.StatusOK {
		t.Fatalf("GET detail status = %d, body=%s", status, body)
	}
	var before TrajectoryObligations
	if err := json.Unmarshal(body, &before); err != nil {
		t.Fatalf("decode GET detail: %v", err)
	}
	if before.Trajectory.TrajectoryID != trajectoryID || before.Trajectory.Status != types.TrajectoryLive {
		t.Fatalf("GET trajectory = %+v, want live %q", before.Trajectory, trajectoryID)
	}
	if len(before.OpenWorkItems) != 1 || before.OpenWorkItems[0].WorkItemID != item.WorkItemID {
		t.Fatalf("GET open work items = %+v, want %s", before.OpenWorkItems, item.WorkItemID)
	}

	cancelPath := escapedTrajectoryPath + "/cancel"
	status, body = doRequest(http.MethodPost, cancelPath, "user-bob")
	if status != http.StatusBadRequest {
		t.Fatalf("bodyless cross-owner cancel status = %d, body=%s", status, body)
	}
	trajectory, err := s.GetTrajectory(ctx, ownerID, trajectoryID)
	if err != nil {
		t.Fatalf("get trajectory after rejected HTTP cancel: %v", err)
	}
	if trajectory.Status != types.TrajectoryLive {
		t.Fatalf("cross-owner HTTP cancel changed trajectory status to %s", trajectory.Status)
	}

	for _, extraPath := range []string{
		cancelPath + "/extra",
		escapedTrajectoryPath + "/extra/cancel",
		escapedTrajectoryPath + "//cancel",
	} {
		status, body = doRequest(http.MethodPost, extraPath, ownerID)
		if status != http.StatusNotFound {
			t.Errorf("POST extra path %q status = %d, body=%s; want 404", extraPath, status, body)
		}
	}
	trajectory, err = s.GetTrajectory(ctx, ownerID, trajectoryID)
	if err != nil {
		t.Fatalf("get trajectory after invalid HTTP paths: %v", err)
	}
	if trajectory.Status != types.TrajectoryLive {
		t.Fatalf("invalid HTTP path changed trajectory status to %s", trajectory.Status)
	}

	for _, malformedPath := range []string{
		"/api/trajectories/%/cancel",
		"/api/trajectories/%2/cancel",
		"/api/trajectories/%GG/cancel",
	} {
		if id, ok := trajectoryCancelIDFromPath(malformedPath); ok {
			t.Errorf("malformed escaped path %q parsed as trajectory %q", malformedPath, id)
		}
		status, body := doRawRequest(malformedPath)
		if status != http.StatusBadRequest {
			t.Errorf("raw malformed path %q status = %d, body=%s; want 400", malformedPath, status, body)
		}
	}
	const percentBearingID = "traj%2Fliteral"
	percentBearingPath := "/api/trajectories/" + url.PathEscape(percentBearingID) + "/cancel"
	if id, ok := trajectoryCancelIDFromPath(percentBearingPath); !ok || id != percentBearingID {
		t.Fatalf("single unescape parsed %q as (%q, %t), want (%q, true)", percentBearingPath, id, ok, percentBearingID)
	}
}

func TestTrajectoryCancelPublicCommandReplaysAndConflicts(t *testing.T) {
	rt, s := testRuntime(t)
	h := NewAPIHandler(rt)
	const ownerID = "user-public-cancel"
	trajectoryID := seedDurableTextureSubject(t, s, ownerID, "doc-public-cancel")
	path := "/api/trajectories/" + url.PathEscape(trajectoryID) + "/cancel"
	snapshot, err := s.GetLifecycleSnapshot(context.Background(), ownerID, rt.TextureSandboxID(), trajectoryID)
	if err != nil {
		t.Fatalf("snapshot before cancel: %v", err)
	}
	cancelBody := func(reason string) string {
		body, _ := json.Marshal(trajectoryCancelRequest{
			IdempotencyKey: "cancel-command-1", ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion,
			ExpectedHeadRevisionID: snapshot.HeadRevision.RevisionID, Reason: reason,
		})
		return string(body)
	}

	callAs := func(requestOwner, body string) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
		request.Header.Set("X-Authenticated-User", requestOwner)
		response := httptest.NewRecorder()
		h.HandleTrajectoryDetail(response, request)
		return response
	}
	call := func(body string) *httptest.ResponseRecorder {
		return callAs(ownerID, body)
	}
	if response := callAs("user-other", cancelBody("owner requested")); response.Code != http.StatusNotFound {
		t.Fatalf("cross-owner cancel status=%d body=%s", response.Code, response.Body.String())
	}
	first := call(cancelBody("owner requested"))
	if first.Code != http.StatusOK {
		t.Fatalf("first cancel status=%d body=%s", first.Code, first.Body.String())
	}
	var firstResponse trajectoryCancelResponse
	if err := json.Unmarshal(first.Body.Bytes(), &firstResponse); err != nil {
		t.Fatal(err)
	}
	if firstResponse.Schema != types.DurableWorkSchemaV1 || firstResponse.Receipt.CommandID != "public-cancel:cancel-command-1" {
		t.Fatalf("first cancellation response = %+v", firstResponse)
	}
	replay := call(cancelBody("owner requested"))
	if replay.Code != http.StatusOK {
		t.Fatalf("replay status=%d body=%s", replay.Code, replay.Body.String())
	}
	var replayResponse trajectoryCancelResponse
	if err := json.Unmarshal(replay.Body.Bytes(), &replayResponse); err != nil {
		t.Fatal(err)
	}
	if replayResponse.Receipt.CommandDigest != firstResponse.Receipt.CommandDigest ||
		replayResponse.Receipt.ReducerSeq != firstResponse.Receipt.ReducerSeq {
		t.Fatalf("replay receipt = %+v, want original %+v", replayResponse.Receipt, firstResponse.Receipt)
	}
	conflict := call(cancelBody("different request"))
	if conflict.Code != http.StatusConflict {
		t.Fatalf("conflicting replay status=%d body=%s", conflict.Code, conflict.Body.String())
	}
}

func TestEvaluateTrajectorySettlementIsPureDataEvaluation(t *testing.T) {
	rec := types.TrajectoryRecord{
		Status:         types.TrajectoryLive,
		SubjectRefs:    map[string]string{"publish_ref": "refs/publications/p-1", "edition_ref": "refs/editions/e-1"},
		SettlementRule: types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"publish_ref", "edition_ref"}},
	}
	if ready, waiting := EvaluateTrajectorySettlement(rec, 0); !ready || len(waiting) != 0 {
		t.Fatalf("satisfied rule not ready: ready=%v waiting=%v", ready, waiting)
	}
	if ready, _ := EvaluateTrajectorySettlement(rec, 3); ready {
		t.Fatalf("open work items did not block settlement")
	}
	for name, rule := range map[string]types.SettlementRule{
		"missing version":   {RequireNoOpenWorkItems: true},
		"unknown version":   {Version: "durable-work/v2", RequireNoOpenWorkItems: true},
		"missing predicate": {Version: types.LifecycleReducerVersion},
		"missing refs":      {Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true},
		"duplicate ref":     {Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"publish_ref", "publish_ref"}},
		"empty ref":         {Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{""}},
	} {
		t.Run(name, func(t *testing.T) {
			malformed := rec
			malformed.SettlementRule = rule
			if ready, _ := EvaluateTrajectorySettlement(malformed, 0); ready {
				t.Fatalf("malformed settlement rule reported ready: %+v", rule)
			}
		})
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
