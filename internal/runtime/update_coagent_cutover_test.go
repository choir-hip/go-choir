package runtime

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestUpdateCoagentPendingUpdateSurvivesRestartAndDeliversOnce(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID := "user-alice"
	trajectoryID := "traj-update-restart"
	superAgent, err := rt.EnsurePersistentSuperAgent(ctx, ownerID)
	if err != nil {
		t.Fatalf("ensure super agent: %v", err)
	}
	if _, err := s.CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
		TrajectoryID:   trajectoryID,
		OwnerID:        ownerID,
		Kind:           types.TrajectoryKindTask,
		Status:         types.TrajectoryLive,
		SettlementRule: types.SettlementRule{RequireNoOpenWorkItems: true},
	}); err != nil {
		t.Fatalf("create trajectory: %v", err)
	}

	update := types.WorkerUpdateRecord{
		UpdateID:      "update-restart-1",
		OwnerID:       ownerID,
		AgentID:       "co-super:impl",
		TargetAgentID: superAgent.AgentID,
		ChannelID:     superAgent.ChannelID,
		TrajectoryID:  trajectoryID,
		Role:          AgentProfileCoSuper,
		Kind:          "status",
		Summary:       "implementation evidence is ready",
		Content:       "implementation evidence is ready",
		CreatedAt:     time.Now().UTC(),
	}
	message := types.ChannelMessage{
		ChannelID:    update.ChannelID,
		FromAgentID:  update.AgentID,
		ToAgentID:    update.TargetAgentID,
		TrajectoryID: update.TrajectoryID,
		Role:         update.Role,
		Content:      update.Content,
		Timestamp:    update.CreatedAt,
	}
	if _, created, err := s.DispatchWorkerUpdate(ctx, update, &message); err != nil {
		t.Fatalf("dispatch update: %v", err)
	} else if !created {
		t.Fatal("first dispatch returned existing update")
	}
	if _, created, err := s.DispatchWorkerUpdate(ctx, update, &message); err != nil {
		t.Fatalf("repeat dispatch: %v", err)
	} else if created {
		t.Fatal("repeat dispatch created duplicate update")
	}
	pending, err := s.ListPendingWorkerUpdates(ctx, ownerID, superAgent.AgentID, 10)
	if err != nil {
		t.Fatalf("list pending updates: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("pending updates = %+v, want exactly one", pending)
	}

	rt.Stop()
	rt2 := New(rt.cfg, s, events.NewEventBus(), NewStubProvider(0))
	t.Cleanup(rt2.Stop)
	run, err := rt2.reconcilePersistentSuperActor(ctx, ownerID, superAgent.AgentID)
	if err != nil {
		t.Fatalf("reconcile after restart: %v", err)
	}
	if run == nil {
		t.Fatal("reconcile after restart did not wake persistent super")
	}
	waitForRuntimeRunTerminal(t, rt2, run.RunID, ownerID, 5*time.Second)
	updates, err := s.ListWorkerUpdatesByTrajectory(ctx, ownerID, trajectoryID, 10)
	if err != nil {
		t.Fatalf("list updates by trajectory: %v", err)
	}
	if len(updates) != 1 || updates[0].DeliveredToRunID != run.RunID || updates[0].DeliveredAt == nil {
		t.Fatalf("delivered update = %+v, want exactly-once delivery to %s", updates, run.RunID)
	}
	pending, err = s.ListPendingWorkerUpdates(ctx, ownerID, superAgent.AgentID, 10)
	if err != nil {
		t.Fatalf("list pending after completion: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("pending after completion = %+v, want none", pending)
	}
}

func waitForRuntimeRunTerminal(t *testing.T, rt *Runtime, runID, ownerID string, timeout time.Duration) types.RunRecord {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last types.RunRecord
	for time.Now().Before(deadline) {
		rec, err := rt.GetRun(context.Background(), runID, ownerID)
		if err == nil {
			last = *rec
			if rec.State == types.RunCompleted || rec.State == types.RunFailed || rec.State == types.RunCancelled || rec.State == types.RunBlocked {
				return *rec
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("run %s did not reach terminal state; last=%+v", runID, last)
	return last
}

func TestTrajectoryObligationsReportPendingUpdateCoagent(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID := "user-alice"
	trajectoryID := "traj-update-stall"
	superAgent, err := rt.EnsurePersistentSuperAgent(ctx, ownerID)
	if err != nil {
		t.Fatalf("ensure super agent: %v", err)
	}
	if _, err := s.CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
		TrajectoryID:   trajectoryID,
		OwnerID:        ownerID,
		Kind:           types.TrajectoryKindTask,
		Status:         types.TrajectoryLive,
		SettlementRule: types.SettlementRule{RequireNoOpenWorkItems: true},
	}); err != nil {
		t.Fatalf("create trajectory: %v", err)
	}
	update := types.WorkerUpdateRecord{
		UpdateID:      "update-stall-1",
		OwnerID:       ownerID,
		AgentID:       "co-super:verifier",
		TargetAgentID: superAgent.AgentID,
		ChannelID:     superAgent.ChannelID,
		TrajectoryID:  trajectoryID,
		Role:          AgentProfileCoSuper,
		Kind:          "verification",
		Summary:       "verification result pending",
		Content:       "verification result pending",
		CreatedAt:     time.Now().UTC(),
	}
	message := types.ChannelMessage{
		ChannelID:    update.ChannelID,
		FromAgentID:  update.AgentID,
		ToAgentID:    update.TargetAgentID,
		TrajectoryID: update.TrajectoryID,
		Role:         update.Role,
		Content:      update.Content,
		Timestamp:    update.CreatedAt,
	}
	if _, _, err := s.DispatchWorkerUpdate(ctx, update, &message); err != nil {
		t.Fatalf("dispatch update: %v", err)
	}
	obligations, err := rt.TrajectoryObligations(ctx, ownerID, trajectoryID)
	if err != nil {
		t.Fatalf("trajectory obligations: %v", err)
	}
	if obligations.PendingUpdates != 1 || obligations.SettlementReady {
		t.Fatalf("obligations = %+v, want one pending update and not ready", obligations)
	}
	if len(obligations.WaitingOn) == 0 || !strings.Contains(obligations.WaitingOn[0], "pending update_coagent") {
		t.Fatalf("waiting_on = %+v, want pending update_coagent reason", obligations.WaitingOn)
	}
}

func TestVSuperCoSuperSlotReusedByTrajectorySlot(t *testing.T) {
	rt, _ := testRuntime(t)
	ctx := context.Background()
	parent, err := rt.StartRunWithMetadata(ctx, "coordinate candidate", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileVSuper,
		runMetadataAgentRole:    AgentProfileVSuper,
		runMetadataAgentID:      "vsuper:traj-slot",
		runMetadataTrajectoryID: "traj-slot",
	})
	if err != nil {
		t.Fatalf("start parent: %v", err)
	}
	first, err := rt.StartChildRun(ctx, parent.RunID, "implement once", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileCoSuper,
		runMetadataAgentRole:    AgentProfileCoSuper,
		runMetadataCoSuperSlot:  "implementation",
	})
	if err != nil {
		t.Fatalf("start first co-super: %v", err)
	}
	second, err := rt.StartChildRun(ctx, parent.RunID, "implement duplicate", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileCoSuper,
		runMetadataAgentRole:    AgentProfileCoSuper,
		runMetadataCoSuperSlot:  "implementation",
	})
	if err != nil {
		t.Fatalf("start duplicate co-super: %v", err)
	}
	if second.RunID != first.RunID {
		t.Fatalf("duplicate slot run = %s, want existing %s", second.RunID, first.RunID)
	}
	if second.Metadata[runMetadataSpawnReused] != true {
		t.Fatalf("duplicate slot metadata = %+v, want spawn_reused", second.Metadata)
	}
}
