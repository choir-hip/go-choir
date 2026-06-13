package runtime

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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

func TestStartSweepsAssignedOpenWorkItemsAfterPassivation(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "go-choir-m3-runtime-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, t.Name()+".db")
	_ = os.Remove(dbPath)

	ctx := context.Background()
	ownerID := "user-alice"
	agentID := "cosuper:work-sweep"
	trajectoryID := "traj-work-sweep"
	channelID := "channel-work-sweep"

	s1, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open store 1: %v", err)
	}
	now := time.Now().UTC()
	if err := s1.UpsertAgent(ctx, types.AgentRecord{
		AgentID:   agentID,
		OwnerID:   ownerID,
		SandboxID: "sandbox-test",
		Profile:   AgentProfileCoSuper,
		Role:      AgentProfileCoSuper,
		ChannelID: channelID,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert agent: %v", err)
	}
	if _, err := s1.CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
		OwnerID:        ownerID,
		TrajectoryID:   trajectoryID,
		Kind:           types.TrajectoryKindTask,
		Status:         types.TrajectoryLive,
		SettlementRule: types.SettlementRule{RequireNoOpenWorkItems: true},
	}); err != nil {
		t.Fatalf("create trajectory: %v", err)
	}
	interrupted := types.RunRecord{
		RunID:        "interrupted-work-sweep",
		AgentID:      agentID,
		ChannelID:    channelID,
		TrajectoryID: trajectoryID,
		AgentProfile: AgentProfileCoSuper,
		AgentRole:    AgentProfileCoSuper,
		OwnerID:      ownerID,
		SandboxID:    "sandbox-test",
		State:        types.RunRunning,
		Prompt:       "interrupted assigned work",
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileCoSuper,
			runMetadataAgentRole:    AgentProfileCoSuper,
			runMetadataAgentID:      agentID,
			runMetadataChannelID:    channelID,
			runMetadataTrajectoryID: trajectoryID,
		},
	}
	if err := s1.CreateRun(ctx, interrupted); err != nil {
		t.Fatalf("create interrupted run: %v", err)
	}
	item, err := s1.CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:          ownerID,
		TrajectoryID:     trajectoryID,
		Objective:        "finish assigned open obligation",
		Reason:           "restart recovery should not require a pending update_coagent row",
		AuthorityProfile: AgentProfileCoSuper,
		AssignedAgentID:  agentID,
		CreatedByRunID:   interrupted.RunID,
	})
	if err != nil {
		t.Fatalf("create work item: %v", err)
	}
	if pending, err := s1.CountPendingWorkerUpdatesByTrajectory(ctx, ownerID, trajectoryID); err != nil {
		t.Fatalf("count pending worker updates: %v", err)
	} else if pending != 0 {
		t.Fatalf("pending updates = %d, want 0", pending)
	}
	_ = s1.Close()

	s2, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open store 2: %v", err)
	}
	rt := New(Config{
		SandboxID:           "sandbox-test",
		StorePath:           dbPath,
		ProviderTimeout:     time.Second,
		SupervisionInterval: time.Hour,
	}, s2, events.NewEventBus(), NewStubProvider(2*time.Second))
	t.Cleanup(func() {
		rt.Stop()
		_ = s2.Close()
		_ = os.Remove(dbPath)
	})

	rt.Start(ctx)

	passivated, err := s2.GetRun(ctx, interrupted.RunID)
	if err != nil {
		t.Fatalf("get interrupted run: %v", err)
	}
	if passivated.State != types.RunPassivated {
		t.Fatalf("interrupted state = %q, want %q", passivated.State, types.RunPassivated)
	}

	var active types.RunRecord
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		active, err = s2.GetLatestActiveRunByAgent(ctx, ownerID, agentID)
		if err == nil && active.RunID != interrupted.RunID {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("get active replacement run: %v", err)
	}
	if active.RunID == "" || active.RunID == interrupted.RunID {
		t.Fatalf("replacement run = %+v, want new active run", active)
	}
	if got := metadataStringValue(active.Metadata, "request_source"); got != "trajectory_work_item_sweep" {
		t.Fatalf("request_source = %q, want trajectory_work_item_sweep", got)
	}
	if got := metadataStringValue(active.Metadata, runMetadataTrajectoryID); got != trajectoryID {
		t.Fatalf("trajectory metadata = %q, want %q", got, trajectoryID)
	}
	if ids := metadataStringSlice(active.Metadata["work_item_ids"]); !containsString(ids, item.WorkItemID) {
		t.Fatalf("work_item_ids = %+v, want %s", ids, item.WorkItemID)
	}
	if !strings.Contains(active.Prompt, "finish assigned open obligation") {
		t.Fatalf("replacement prompt did not include work item objective: %q", active.Prompt)
	}
}

func TestStartRewarmsCoagentWithPendingUpdatesAndAssignedWork(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "go-choir-m3-runtime-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, t.Name()+".db")
	_ = os.Remove(dbPath)

	ctx := context.Background()
	ownerID := "user-alice"
	agentID := "cosuper:combined-rewarm"
	trajectoryID := "traj-combined-rewarm"
	otherTrajectoryID := "traj-combined-rewarm-other"
	channelID := "channel-combined-rewarm"
	now := time.Now().UTC()

	s1, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open store 1: %v", err)
	}
	if err := s1.UpsertAgent(ctx, types.AgentRecord{
		AgentID:   agentID,
		OwnerID:   ownerID,
		SandboxID: "sandbox-test",
		Profile:   AgentProfileCoSuper,
		Role:      AgentProfileCoSuper,
		ChannelID: channelID,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert agent: %v", err)
	}
	if _, err := s1.CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
		OwnerID:        ownerID,
		TrajectoryID:   trajectoryID,
		Kind:           types.TrajectoryKindTask,
		Status:         types.TrajectoryLive,
		SettlementRule: types.SettlementRule{RequireNoOpenWorkItems: true},
	}); err != nil {
		t.Fatalf("create trajectory: %v", err)
	}
	if _, err := s1.CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
		OwnerID:        ownerID,
		TrajectoryID:   otherTrajectoryID,
		Kind:           types.TrajectoryKindTask,
		Status:         types.TrajectoryLive,
		SettlementRule: types.SettlementRule{RequireNoOpenWorkItems: true},
	}); err != nil {
		t.Fatalf("create second trajectory: %v", err)
	}
	interrupted := types.RunRecord{
		RunID:        "interrupted-combined-rewarm",
		AgentID:      agentID,
		ChannelID:    channelID,
		TrajectoryID: trajectoryID,
		AgentProfile: AgentProfileCoSuper,
		AgentRole:    AgentProfileCoSuper,
		OwnerID:      ownerID,
		SandboxID:    "sandbox-test",
		State:        types.RunRunning,
		Prompt:       "interrupted combined restart backlog",
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileCoSuper,
			runMetadataAgentRole:    AgentProfileCoSuper,
			runMetadataAgentID:      agentID,
			runMetadataChannelID:    channelID,
			runMetadataTrajectoryID: trajectoryID,
		},
	}
	if err := s1.CreateRun(ctx, interrupted); err != nil {
		t.Fatalf("create interrupted run: %v", err)
	}
	item, err := s1.CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:          ownerID,
		TrajectoryID:     trajectoryID,
		Objective:        "finish combined assigned obligation",
		Reason:           "restart recovery must include assigned work with pending updates",
		AuthorityProfile: AgentProfileCoSuper,
		AssignedAgentID:  agentID,
		CreatedByRunID:   interrupted.RunID,
	})
	if err != nil {
		t.Fatalf("create work item: %v", err)
	}
	otherItem, err := s1.CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:          ownerID,
		TrajectoryID:     otherTrajectoryID,
		Objective:        "finish second trajectory assigned obligation",
		Reason:           "restart recovery must include every pending-update trajectory",
		AuthorityProfile: AgentProfileCoSuper,
		AssignedAgentID:  agentID,
		CreatedByRunID:   interrupted.RunID,
	})
	if err != nil {
		t.Fatalf("create second work item: %v", err)
	}
	update := types.WorkerUpdateRecord{
		UpdateID:      "update-combined-rewarm",
		OwnerID:       ownerID,
		AgentID:       "co-super:verifier",
		TargetAgentID: agentID,
		ChannelID:     channelID,
		TrajectoryID:  trajectoryID,
		Role:          AgentProfileCoSuper,
		Kind:          "verification",
		Summary:       "combined restart update",
		Content:       "pending update content for combined restart",
		CreatedAt:     now.Add(time.Millisecond),
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
	if _, _, err := s1.DispatchWorkerUpdate(ctx, update, &message); err != nil {
		t.Fatalf("dispatch update: %v", err)
	}
	otherUpdate := types.WorkerUpdateRecord{
		UpdateID:      "update-combined-rewarm-other",
		OwnerID:       ownerID,
		AgentID:       "co-super:reviewer",
		TargetAgentID: agentID,
		ChannelID:     channelID,
		TrajectoryID:  otherTrajectoryID,
		Role:          AgentProfileCoSuper,
		Kind:          "status",
		Summary:       "second trajectory restart update",
		Content:       "pending update content for second trajectory",
		CreatedAt:     now.Add(2 * time.Millisecond),
	}
	otherMessage := types.ChannelMessage{
		ChannelID:    otherUpdate.ChannelID,
		FromAgentID:  otherUpdate.AgentID,
		ToAgentID:    otherUpdate.TargetAgentID,
		TrajectoryID: otherUpdate.TrajectoryID,
		Role:         otherUpdate.Role,
		Content:      otherUpdate.Content,
		Timestamp:    otherUpdate.CreatedAt,
	}
	if _, _, err := s1.DispatchWorkerUpdate(ctx, otherUpdate, &otherMessage); err != nil {
		t.Fatalf("dispatch second update: %v", err)
	}
	_ = s1.Close()

	s2, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open store 2: %v", err)
	}
	rt := New(Config{
		SandboxID:           "sandbox-test",
		StorePath:           dbPath,
		ProviderTimeout:     time.Second,
		SupervisionInterval: time.Hour,
	}, s2, events.NewEventBus(), NewStubProvider(2*time.Second))
	t.Cleanup(func() {
		rt.Stop()
		_ = s2.Close()
		_ = os.Remove(dbPath)
	})

	rt.Start(ctx)

	passivated, err := s2.GetRun(ctx, interrupted.RunID)
	if err != nil {
		t.Fatalf("get interrupted run: %v", err)
	}
	if passivated.State != types.RunPassivated {
		t.Fatalf("interrupted state = %q, want %q", passivated.State, types.RunPassivated)
	}

	var active types.RunRecord
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		active, err = s2.GetLatestActiveRunByAgent(ctx, ownerID, agentID)
		if err == nil && active.RunID != interrupted.RunID {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("get active replacement run: %v", err)
	}
	if active.RunID == "" || active.RunID == interrupted.RunID {
		t.Fatalf("replacement run = %+v, want new active run", active)
	}
	if ids := metadataStringSlice(active.Metadata["worker_update_ids"]); !containsString(ids, update.UpdateID) {
		t.Fatalf("worker_update_ids = %+v, want %s", ids, update.UpdateID)
	} else if !containsString(ids, otherUpdate.UpdateID) {
		t.Fatalf("worker_update_ids = %+v, want %s", ids, otherUpdate.UpdateID)
	}
	if ids := metadataStringSlice(active.Metadata["work_item_ids"]); !containsString(ids, item.WorkItemID) {
		t.Fatalf("work_item_ids = %+v, want %s", ids, item.WorkItemID)
	} else if !containsString(ids, otherItem.WorkItemID) {
		t.Fatalf("work_item_ids = %+v, want %s", ids, otherItem.WorkItemID)
	}
	if !strings.Contains(active.Prompt, update.Content) {
		t.Fatalf("replacement prompt missing update content: %q", active.Prompt)
	}
	if !strings.Contains(active.Prompt, otherUpdate.Content) {
		t.Fatalf("replacement prompt missing second update content: %q", active.Prompt)
	}
	if !strings.Contains(active.Prompt, item.Objective) {
		t.Fatalf("replacement prompt missing work item objective: %q", active.Prompt)
	}
	if !strings.Contains(active.Prompt, otherItem.Objective) {
		t.Fatalf("replacement prompt missing second work item objective: %q", active.Prompt)
	}

	obligations, err := rt.TrajectoryObligations(ctx, ownerID, trajectoryID)
	if err != nil {
		t.Fatalf("trajectory obligations: %v", err)
	}
	if len(obligations.OpenWorkItems) != 1 || obligations.OpenWorkItems[0].WorkItemID != item.WorkItemID {
		t.Fatalf("open work items = %+v, want %s still open", obligations.OpenWorkItems, item.WorkItemID)
	}
	if obligations.PendingUpdates != 1 || obligations.SettlementReady {
		t.Fatalf("obligations = %+v, want pending update and unsettled open work", obligations)
	}
	otherObligations, err := rt.TrajectoryObligations(ctx, ownerID, otherTrajectoryID)
	if err != nil {
		t.Fatalf("second trajectory obligations: %v", err)
	}
	if len(otherObligations.OpenWorkItems) != 1 || otherObligations.OpenWorkItems[0].WorkItemID != otherItem.WorkItemID {
		t.Fatalf("second open work items = %+v, want %s still open", otherObligations.OpenWorkItems, otherItem.WorkItemID)
	}
	if otherObligations.PendingUpdates != 1 || otherObligations.SettlementReady {
		t.Fatalf("second obligations = %+v, want pending update and unsettled open work", otherObligations)
	}
}

func TestCoagentRewarmUsesResidentActivationNotActiveRunProxy(t *testing.T) {
	rt, s := testRuntimeWithProviderAndRegistry(t, NewStubProvider(2*time.Second), nil)
	ctx := context.Background()
	ownerID := "user-alice"
	agentID := "coagent:resident-reuse"
	trajectoryID := "traj-resident-reuse"

	active, err := rt.StartRunWithMetadata(ctx, "continue active work", ownerID, map[string]any{
		runMetadataAgentProfile: AgentProfileCoSuper,
		runMetadataAgentRole:    AgentProfileCoSuper,
		runMetadataAgentID:      agentID,
		runMetadataChannelID:    "chan-resident-reuse",
		runMetadataTrajectoryID: trajectoryID,
	})
	if err != nil {
		t.Fatalf("start resident run: %v", err)
	}
	if resident, found, err := rt.residentRunByAgent(ctx, ownerID, agentID); err != nil {
		t.Fatalf("resident lookup: %v", err)
	} else if !found || resident.RunID != active.RunID {
		t.Fatalf("resident lookup = (%+v, %v), want %s", resident, found, active.RunID)
	}

	update := types.WorkerUpdateRecord{
		UpdateID:      "update-resident-reuse",
		OwnerID:       ownerID,
		AgentID:       "co-super:impl",
		TargetAgentID: agentID,
		ChannelID:     active.ChannelID,
		TrajectoryID:  trajectoryID,
		Role:          AgentProfileCoSuper,
		Kind:          "status",
		Content:       "new steering input",
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

	got, err := rt.reconcileUpdatedCoagentActor(ctx, ownerID, agentID)
	if err != nil {
		t.Fatalf("reconcile resident coagent: %v", err)
	}
	if got == nil || got.RunID != active.RunID {
		t.Fatalf("reconcile returned %+v, want resident run %s", got, active.RunID)
	}
}

func TestCoagentRewarmIgnoresBlockedHistoricalActivation(t *testing.T) {
	rt, s := testRuntimeWithProviderAndRegistry(t, NewStubProvider(2*time.Second), nil)
	ctx := context.Background()
	ownerID := "user-alice"
	agentID := "coagent:blocked-history"
	trajectoryID := "traj-blocked-history"
	now := time.Now().UTC()
	if err := s.UpsertAgent(ctx, types.AgentRecord{
		AgentID:   agentID,
		OwnerID:   ownerID,
		SandboxID: "sandbox-test",
		Profile:   AgentProfileCoSuper,
		Role:      AgentProfileCoSuper,
		ChannelID: "chan-blocked-history",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert agent: %v", err)
	}
	blocked := types.RunRecord{
		RunID:        "run-blocked-history",
		AgentID:      agentID,
		ChannelID:    "chan-blocked-history",
		TrajectoryID: trajectoryID,
		AgentProfile: AgentProfileCoSuper,
		AgentRole:    AgentProfileCoSuper,
		OwnerID:      ownerID,
		SandboxID:    "sandbox-test",
		State:        types.RunBlocked,
		Prompt:       "historical blocked activation",
		Error:        "historical provider failure",
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileCoSuper,
			runMetadataAgentRole:    AgentProfileCoSuper,
			runMetadataAgentID:      agentID,
			runMetadataChannelID:    "chan-blocked-history",
			runMetadataTrajectoryID: trajectoryID,
		},
	}
	if err := s.CreateRun(ctx, blocked); err != nil {
		t.Fatalf("create blocked historical run: %v", err)
	}
	update := types.WorkerUpdateRecord{
		UpdateID:      "update-blocked-history",
		OwnerID:       ownerID,
		AgentID:       "co-super:impl",
		TargetAgentID: agentID,
		ChannelID:     blocked.ChannelID,
		TrajectoryID:  trajectoryID,
		Role:          AgentProfileCoSuper,
		Kind:          "status",
		Content:       "durable backlog should start a fresh activation",
		CreatedAt:     now.Add(time.Millisecond),
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

	rewarmed, err := rt.reconcileUpdatedCoagentActor(ctx, ownerID, agentID)
	if err != nil {
		t.Fatalf("reconcile blocked historical coagent: %v", err)
	}
	if rewarmed == nil {
		t.Fatal("reconcile did not start a replacement activation")
	}
	if rewarmed.RunID == blocked.RunID {
		t.Fatalf("reconcile reused blocked historical run %s", blocked.RunID)
	}
	if got := metadataStringValue(rewarmed.Metadata, "request_source"); got != "update_coagent" {
		t.Fatalf("request_source = %q, want update_coagent", got)
	}
	if ids := metadataStringSlice(rewarmed.Metadata["worker_update_ids"]); !containsString(ids, update.UpdateID) {
		t.Fatalf("worker_update_ids = %+v, want %s", ids, update.UpdateID)
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

func TestUpdateCoagentDeliveryRequiresSuccessfulActivation(t *testing.T) {
	cases := []struct {
		name          string
		state         types.RunState
		wantDelivered bool
	}{
		{name: "completed", state: types.RunCompleted, wantDelivered: true},
		{name: "failed", state: types.RunFailed, wantDelivered: false},
		{name: "cancelled", state: types.RunCancelled, wantDelivered: false},
		{name: "blocked", state: types.RunBlocked, wantDelivered: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rt, s := testRuntime(t)
			ctx := context.Background()
			ownerID := "user-alice"
			targetAgentID := "coagent:" + tc.name
			updateID := "update-delivery-" + tc.name
			now := time.Now().UTC()

			update := types.WorkerUpdateRecord{
				UpdateID:      updateID,
				OwnerID:       ownerID,
				AgentID:       "co-super:impl",
				TargetAgentID: targetAgentID,
				ChannelID:     "chan-delivery-" + tc.name,
				TrajectoryID:  "traj-delivery-" + tc.name,
				Role:          AgentProfileCoSuper,
				Kind:          "status",
				Summary:       "delivery rule evidence",
				Content:       "delivery rule evidence",
				CreatedAt:     now,
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

			rec := types.RunRecord{
				RunID:        "run-delivery-" + tc.name,
				AgentID:      targetAgentID,
				ChannelID:    update.ChannelID,
				TrajectoryID: update.TrajectoryID,
				AgentProfile: AgentProfileCoSuper,
				AgentRole:    AgentProfileCoSuper,
				OwnerID:      ownerID,
				SandboxID:    "sandbox-test",
				State:        types.RunRunning,
				Prompt:       "process update",
				CreatedAt:    now,
				UpdatedAt:    now,
				Metadata: map[string]any{
					runMetadataAgentProfile: AgentProfileCoSuper,
					runMetadataAgentRole:    AgentProfileCoSuper,
					runMetadataAgentID:      targetAgentID,
					runMetadataChannelID:    update.ChannelID,
					runMetadataTrajectoryID: update.TrajectoryID,
					"request_source":        "update_coagent",
					"worker_update_ids":     []string{updateID},
				},
			}
			if err := s.CreateRun(ctx, rec); err != nil {
				t.Fatalf("create run: %v", err)
			}
			finishedAt := now.Add(time.Second)
			rec.State = tc.state
			rec.UpdatedAt = finishedAt
			if tc.state.Terminal() {
				rec.FinishedAt = &finishedAt
			}
			if tc.state == types.RunCompleted {
				rec.Result = "processed update"
			} else if tc.state == types.RunFailed || tc.state == types.RunBlocked {
				rec.Error = "activation did not incorporate update"
			}
			if err := rt.updateRunAndMarkSuccessfulCoagentActivationDelivered(ctx, &rec); err != nil {
				t.Fatalf("update activation outcome: %v", err)
			}

			stored, err := s.GetWorkerUpdate(ctx, ownerID, updateID)
			if err != nil {
				t.Fatalf("get worker update: %v", err)
			}
			if tc.wantDelivered {
				if stored.DeliveredAt == nil || stored.DeliveredToRunID != rec.RunID {
					t.Fatalf("delivered update = %+v, want delivered to %s", stored, rec.RunID)
				}
				pending, err := s.ListPendingWorkerUpdates(ctx, ownerID, targetAgentID, 10)
				if err != nil {
					t.Fatalf("list pending updates: %v", err)
				}
				if len(pending) != 0 {
					t.Fatalf("pending updates after success = %+v, want none", pending)
				}
				return
			}
			if stored.DeliveredAt != nil || stored.DeliveredToRunID != "" {
				t.Fatalf("failed activation delivered update unexpectedly: %+v", stored)
			}
			pending, err := s.ListPendingWorkerUpdates(ctx, ownerID, targetAgentID, 10)
			if err != nil {
				t.Fatalf("list pending updates: %v", err)
			}
			if len(pending) != 1 || pending[0].UpdateID != updateID {
				t.Fatalf("pending updates = %+v, want %s still pending", pending, updateID)
			}
		})
	}
}

func TestUpdateCoagentDeliveryIgnoresStrayWorkerUpdateMetadata(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID := "user-alice"
	now := time.Now().UTC()
	update := types.WorkerUpdateRecord{
		UpdateID:      "update-stray-1",
		OwnerID:       ownerID,
		AgentID:       "co-super:impl",
		TargetAgentID: "coagent:right",
		ChannelID:     "chan-stray",
		TrajectoryID:  "traj-stray",
		Role:          AgentProfileCoSuper,
		Kind:          "status",
		Summary:       "stray metadata must not consume this",
		Content:       "stray metadata must not consume this",
		CreatedAt:     now,
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

	finishRun := func(t *testing.T, rec types.RunRecord) {
		t.Helper()
		if err := s.CreateRun(ctx, rec); err != nil {
			t.Fatalf("create run %s: %v", rec.RunID, err)
		}
		finishedAt := now.Add(time.Second)
		rec.State = types.RunCompleted
		rec.Result = "done"
		rec.UpdatedAt = finishedAt
		rec.FinishedAt = &finishedAt
		if err := rt.updateRunAndMarkSuccessfulCoagentActivationDelivered(ctx, &rec); err != nil {
			t.Fatalf("update run %s: %v", rec.RunID, err)
		}
		stored, err := s.GetWorkerUpdate(ctx, ownerID, update.UpdateID)
		if err != nil {
			t.Fatalf("get update after %s: %v", rec.RunID, err)
		}
		if stored.DeliveredAt != nil || stored.DeliveredToRunID != "" {
			t.Fatalf("stray run %s delivered update unexpectedly: %+v", rec.RunID, stored)
		}
	}

	finishRun(t, types.RunRecord{
		RunID:        "run-stray-no-source",
		AgentID:      update.TargetAgentID,
		ChannelID:    update.ChannelID,
		TrajectoryID: update.TrajectoryID,
		AgentProfile: AgentProfileCoSuper,
		AgentRole:    AgentProfileCoSuper,
		OwnerID:      ownerID,
		SandboxID:    "sandbox-test",
		State:        types.RunRunning,
		Prompt:       "unrelated completed run",
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileCoSuper,
			runMetadataAgentRole:    AgentProfileCoSuper,
			runMetadataAgentID:      update.TargetAgentID,
			"worker_update_ids":     []string{update.UpdateID},
		},
	})

	finishRun(t, types.RunRecord{
		RunID:        "run-stray-wrong-target",
		AgentID:      "coagent:wrong",
		ChannelID:    update.ChannelID,
		TrajectoryID: update.TrajectoryID,
		AgentProfile: AgentProfileCoSuper,
		AgentRole:    AgentProfileCoSuper,
		OwnerID:      ownerID,
		SandboxID:    "sandbox-test",
		State:        types.RunRunning,
		Prompt:       "wrong target completed run",
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile:          AgentProfileCoSuper,
			runMetadataAgentRole:             AgentProfileCoSuper,
			runMetadataAgentID:               "coagent:wrong",
			runMetadataWorkerUpdatesInjected: true,
			"worker_update_ids":              []string{update.UpdateID},
		},
	})

	pending, err := s.ListPendingWorkerUpdates(ctx, ownerID, update.TargetAgentID, 10)
	if err != nil {
		t.Fatalf("list pending updates: %v", err)
	}
	if len(pending) != 1 || pending[0].UpdateID != update.UpdateID {
		t.Fatalf("pending updates = %+v, want %s still pending", pending, update.UpdateID)
	}
}

func TestUpdateCoagentWarmActivationInjectsPendingTurn(t *testing.T) {
	provider := &warmUpdateInjectionProvider{StubProvider: NewStubProvider(0)}
	rt, s := testRuntimeWithProviderAndRegistry(t, provider, nil)
	ctx := context.Background()
	ownerID := "user-alice"
	targetAgentID := "coagent:warm"
	trajectoryID := "traj-warm-update"
	now := time.Now().UTC()

	rec := types.RunRecord{
		RunID:        "run-warm-update",
		AgentID:      targetAgentID,
		ChannelID:    "chan-warm-update",
		TrajectoryID: trajectoryID,
		AgentProfile: AgentProfileCoSuper,
		AgentRole:    AgentProfileCoSuper,
		OwnerID:      ownerID,
		SandboxID:    "sandbox-test",
		State:        types.RunRunning,
		Prompt:       "continue current activation",
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileCoSuper,
			runMetadataAgentRole:    AgentProfileCoSuper,
			runMetadataAgentID:      targetAgentID,
			runMetadataChannelID:    "chan-warm-update",
			runMetadataTrajectoryID: trajectoryID,
		},
	}
	if err := s.CreateRun(ctx, rec); err != nil {
		t.Fatalf("create warm run: %v", err)
	}

	update := types.WorkerUpdateRecord{
		UpdateID:      "update-warm-1",
		OwnerID:       ownerID,
		AgentID:       "co-super:impl",
		TargetAgentID: targetAgentID,
		ChannelID:     rec.ChannelID,
		TrajectoryID:  trajectoryID,
		Role:          AgentProfileCoSuper,
		Kind:          "status",
		Summary:       "warm steering evidence",
		Content:       "WARM_UPDATE_CONTENT: incorporate this before finishing.",
		CreatedAt:     now.Add(time.Millisecond),
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
		t.Fatalf("dispatch warm update: %v", err)
	}

	rt.executeWithToolLoop(ctx, &rec, nil, func(types.EventKind, string, json.RawMessage) {})

	if len(provider.requests) < 2 {
		t.Fatalf("provider calls = %d, want second call after injected update", len(provider.requests))
	}
	if !toolLoopRequestContains(provider.requests[1], "WARM_UPDATE_CONTENT") {
		t.Fatalf("second provider request did not contain injected update: %+v", provider.requests[1].Messages)
	}
	storedRun, err := s.GetRun(ctx, rec.RunID)
	if err != nil {
		t.Fatalf("get warm run: %v", err)
	}
	if storedRun.State != types.RunCompleted {
		t.Fatalf("warm run state = %q error=%q", storedRun.State, storedRun.Error)
	}
	if storedRun.Result != "processed warm update" {
		t.Fatalf("warm run result = %q", storedRun.Result)
	}
	if ids := metadataStringSlice(storedRun.Metadata["worker_update_ids"]); len(ids) != 1 || ids[0] != update.UpdateID {
		t.Fatalf("worker_update_ids metadata = %+v, want %s", ids, update.UpdateID)
	}
	storedUpdate, err := s.GetWorkerUpdate(ctx, ownerID, update.UpdateID)
	if err != nil {
		t.Fatalf("get warm update: %v", err)
	}
	if storedUpdate.DeliveredAt == nil || storedUpdate.DeliveredToRunID != rec.RunID {
		t.Fatalf("warm update delivery = %+v, want delivered to %s", storedUpdate, rec.RunID)
	}
}

type warmUpdateInjectionProvider struct {
	*StubProvider
	requests []ToolLoopRequest
}

func (p *warmUpdateInjectionProvider) CallWithTools(ctx context.Context, req ToolLoopRequest) (*ToolLoopResponse, error) {
	p.requests = append(p.requests, req)
	if len(p.requests) == 1 {
		return &ToolLoopResponse{
			StopReason: "end_turn",
			Text:       "initial response before warm update",
			Usage:      TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model:      "test-model",
		}, nil
	}
	text := "processed warm update"
	if !toolLoopRequestContains(req, "WARM_UPDATE_CONTENT") {
		text = "missing warm update"
	}
	return &ToolLoopResponse{
		StopReason: "end_turn",
		Text:       text,
		Usage:      TokenUsage{InputTokens: 1, OutputTokens: 1},
		Model:      "test-model",
	}, nil
}

func toolLoopRequestContains(req ToolLoopRequest, needle string) bool {
	for _, msg := range req.Messages {
		if strings.Contains(string(msg), needle) {
			return true
		}
	}
	return false
}

func TestVSuperCoSuperSlotReusedByTrajectorySlot(t *testing.T) {
	rt, s := testRuntime(t)
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

	passivated, err := s.GetRun(ctx, first.RunID)
	if err != nil {
		t.Fatalf("get first co-super: %v", err)
	}
	passivated.State = types.RunPassivated
	passivated.Error = ""
	passivated.FinishedAt = nil
	passivated.UpdatedAt = time.Now().UTC()
	if err := s.UpdateRun(ctx, passivated); err != nil {
		t.Fatalf("passivate first co-super: %v", err)
	}
	if active, found, err := s.ActiveCoSuperSlotRun(ctx, "user-alice", "traj-slot", "implementation"); err != nil {
		t.Fatalf("active slot lookup: %v", err)
	} else if found {
		t.Fatalf("passivated slot owner still active: %+v", active)
	}

	third, err := rt.StartChildRun(ctx, parent.RunID, "implement after passivation", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileCoSuper,
		runMetadataAgentRole:    AgentProfileCoSuper,
		runMetadataCoSuperSlot:  "implementation",
	})
	if err != nil {
		t.Fatalf("start replacement co-super: %v", err)
	}
	if third.RunID == first.RunID {
		t.Fatalf("replacement slot run reused passivated run %s", first.RunID)
	}
	if third.Metadata[runMetadataSpawnReused] == true {
		t.Fatalf("replacement slot metadata = %+v, want fresh spawn", third.Metadata)
	}
}
