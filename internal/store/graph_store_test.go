package store

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestOGUpsertAndGetAgent(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	rec := types.AgentRecord{
		AgentID:   "agent-og-1",
		OwnerID:   "owner-og",
		SandboxID: "sandbox-1",
		Profile:   "researcher",
		Role:      "researcher",
		ChannelID: "ch-1",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.UpsertAgentOG(ctx, rec); err != nil {
		t.Fatalf("upsert agent: %v", err)
	}

	got, err := s.GetAgentOG(ctx, "agent-og-1")
	if err != nil {
		t.Fatalf("get agent: %v", err)
	}
	if got.AgentID != rec.AgentID {
		t.Errorf("agent_id: got %q, want %q", got.AgentID, rec.AgentID)
	}
	if got.OwnerID != rec.OwnerID {
		t.Errorf("owner_id: got %q, want %q", got.OwnerID, rec.OwnerID)
	}
	if got.Profile != rec.Profile {
		t.Errorf("profile: got %q, want %q", got.Profile, rec.Profile)
	}
	if got.Role != rec.Role {
		t.Errorf("role: got %q, want %q", got.Role, rec.Role)
	}
}

func TestOGUpsertAgentUpdate(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	rec := types.AgentRecord{
		AgentID:   "agent-og-2",
		OwnerID:   "owner-og",
		SandboxID: "sandbox-1",
		Profile:   "researcher",
		Role:      "researcher",
		ChannelID: "ch-1",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.UpsertAgentOG(ctx, rec); err != nil {
		t.Fatalf("upsert agent: %v", err)
	}

	// Update the role.
	rec.Role = "conductor"
	rec.UpdatedAt = time.Now().UTC()
	if err := s.UpsertAgentOG(ctx, rec); err != nil {
		t.Fatalf("upsert agent (update): %v", err)
	}

	got, err := s.GetAgentOG(ctx, "agent-og-2")
	if err != nil {
		t.Fatalf("get agent: %v", err)
	}
	if got.Role != "conductor" {
		t.Errorf("role after update: got %q, want %q", got.Role, "conductor")
	}
}

func TestOGCreateAndGetRun(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC()
	rec := types.RunRecord{
		RunID:       "run-og-1",
		AgentID:     "agent-og-1",
		OwnerID:     "owner-og",
		SandboxID:   "sandbox-1",
		State:       types.RunRunning,
		Prompt:      "test prompt",
		CreatedAt:   now,
		UpdatedAt:   now,
		TrajectoryID: "traj-1",
		Metadata:    map[string]any{"key": "value"},
	}
	if err := s.CreateRunOG(ctx, rec); err != nil {
		t.Fatalf("create run: %v", err)
	}

	got, err := s.GetRunOG(ctx, "run-og-1")
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if got.RunID != rec.RunID {
		t.Errorf("run_id: got %q, want %q", got.RunID, rec.RunID)
	}
	if got.AgentID != rec.AgentID {
		t.Errorf("agent_id: got %q, want %q", got.AgentID, rec.AgentID)
	}
	if got.State != rec.State {
		t.Errorf("state: got %q, want %q", got.State, rec.State)
	}
	if got.Prompt != rec.Prompt {
		t.Errorf("prompt: got %q, want %q", got.Prompt, rec.Prompt)
	}
	if got.TrajectoryID != rec.TrajectoryID {
		t.Errorf("trajectory_id: got %q, want %q", got.TrajectoryID, rec.TrajectoryID)
	}
}

func TestOGUpdateRun(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC()
	rec := types.RunRecord{
		RunID:     "run-og-2",
		AgentID:   "agent-og-2",
		OwnerID:   "owner-og",
		SandboxID: "sandbox-1",
		State:     types.RunRunning,
		Prompt:    "test prompt",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.CreateRunOG(ctx, rec); err != nil {
		t.Fatalf("create run: %v", err)
	}

	// Update to completed.
	finished := now.Add(5 * time.Second)
	rec.State = types.RunCompleted
	rec.Result = "test result"
	rec.UpdatedAt = finished
	rec.FinishedAt = &finished
	if err := s.UpdateRunOG(ctx, rec); err != nil {
		t.Fatalf("update run: %v", err)
	}

	got, err := s.GetRunOG(ctx, "run-og-2")
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if got.State != types.RunCompleted {
		t.Errorf("state: got %q, want %q", got.State, types.RunCompleted)
	}
	if got.Result != "test result" {
		t.Errorf("result: got %q, want %q", got.Result, "test result")
	}
	if got.FinishedAt == nil {
		t.Error("expected finished_at to be set")
	}
}

func TestOGListRunsByOwner(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC()
	for i := range 3 {
		rec := types.RunRecord{
			RunID:     "run-og-list-" + string(rune('A'+i)),
			AgentID:   "agent-1",
			OwnerID:   "owner-list",
			SandboxID: "sandbox-1",
			State:     types.RunRunning,
			Prompt:    "test",
			CreatedAt: now.Add(time.Duration(i) * time.Second),
			UpdatedAt: now.Add(time.Duration(i) * time.Second),
		}
		if err := s.CreateRunOG(ctx, rec); err != nil {
			t.Fatalf("create run %d: %v", i, err)
		}
	}

	// Add a different owner's run.
	other := types.RunRecord{
		RunID:     "run-og-other",
		AgentID:   "agent-1",
		OwnerID:   "other-owner",
		SandboxID: "sandbox-1",
		State:     types.RunRunning,
		Prompt:    "test",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.CreateRunOG(ctx, other); err != nil {
		t.Fatalf("create other run: %v", err)
	}

	runs, err := s.ListRunsByOwnerOG(ctx, "owner-list", 10)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 3 {
		t.Fatalf("expected 3 runs, got %d", len(runs))
	}
	for _, r := range runs {
		if r.OwnerID != "owner-list" {
			t.Errorf("unexpected owner_id %q", r.OwnerID)
		}
	}
}

func TestOGListRunsByState(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC()
	for i := range 2 {
		rec := types.RunRecord{
			RunID:     "run-og-state-running-" + string(rune('A'+i)),
			AgentID:   "agent-1",
			OwnerID:   "owner-state",
			SandboxID: "sandbox-1",
			State:     types.RunRunning,
			Prompt:    "test",
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := s.CreateRunOG(ctx, rec); err != nil {
			t.Fatalf("create running run %d: %v", i, err)
		}
	}
	completed := types.RunRecord{
		RunID:     "run-og-state-completed",
		AgentID:   "agent-1",
		OwnerID:   "owner-state",
		SandboxID: "sandbox-1",
		State:     types.RunCompleted,
		Prompt:    "test",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.CreateRunOG(ctx, completed); err != nil {
		t.Fatalf("create completed run: %v", err)
	}

	running, err := s.ListRunsByStateOG(ctx, types.RunRunning, 10)
	if err != nil {
		t.Fatalf("list running: %v", err)
	}
	if len(running) != 2 {
		t.Fatalf("expected 2 running runs, got %d", len(running))
	}
	for _, r := range running {
		if r.State != types.RunRunning {
			t.Errorf("state: got %q, want %q", r.State, types.RunRunning)
		}
	}
}

func TestOGAppendAndListEvents(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC()
	for i := range 3 {
		rec := &types.EventRecord{
			EventID:   "evt-og-" + string(rune('A'+i)),
			Seq:       int64(i + 1),
			RunID:     "run-og-evt",
			AgentID:   "agent-1",
			OwnerID:   "owner-evt",
			Kind:      "test_event",
			Phase:     "execution",
			Timestamp: now.Add(time.Duration(i) * time.Second),
			Payload:   []byte(`{}`),
		}
		if err := s.AppendEventOG(ctx, rec); err != nil {
			t.Fatalf("append event %d: %v", i, err)
		}
	}

	events, err := s.ListEventsOG(ctx, "run-og-evt", 10)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
}

func TestOGListEventsByOwner(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC()
	rec := &types.EventRecord{
		EventID:   "evt-og-owner",
		Seq:       1,
		RunID:     "run-og-evt",
		OwnerID:   "owner-evt-list",
		Kind:      "test_event",
		Timestamp: now,
		Payload:   []byte(`{}`),
	}
	if err := s.AppendEventOG(ctx, rec); err != nil {
		t.Fatalf("append event: %v", err)
	}

	events, err := s.ListEventsByOwnerOG(ctx, "owner-evt-list", 10)
	if err != nil {
		t.Fatalf("list events by owner: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].OwnerID != "owner-evt-list" {
		t.Errorf("owner_id: got %q", events[0].OwnerID)
	}
}

func TestOGGetAgentNotFound(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	_, err := s.GetAgentOG(ctx, "no-such-agent")
	if err == nil {
		t.Error("expected error for missing agent")
	}
}

func TestOGGetRunNotFound(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	_, err := s.GetRunOG(ctx, "no-such-run")
	if err == nil {
		t.Error("expected error for missing run")
	}
}

// =========================================================================
// Trajectory tests
// =========================================================================

func TestOGCreateTrajectoryIfAbsent(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	rec := types.TrajectoryRecord{
		TrajectoryID: "traj-og-1",
		OwnerID:      "owner-og",
		Kind:         types.TrajectoryKindTask,
		Status:       types.TrajectoryLive,
	}
	created, err := s.CreateTrajectoryIfAbsentOG(ctx, rec)
	if err != nil {
		t.Fatalf("create trajectory: %v", err)
	}
	if created.TrajectoryID != rec.TrajectoryID {
		t.Errorf("trajectory_id: got %q", created.TrajectoryID)
	}

	// Second call should return the existing record.
	rec.Status = types.TrajectorySettled
	existing, err := s.CreateTrajectoryIfAbsentOG(ctx, rec)
	if err != nil {
		t.Fatalf("create trajectory (second): %v", err)
	}
	if existing.Status != types.TrajectoryLive {
		t.Errorf("expected original status, got %q", existing.Status)
	}
}

func TestOGGetTrajectory(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	rec := types.TrajectoryRecord{
		TrajectoryID: "traj-og-2",
		OwnerID:      "owner-og",
		Kind:         types.TrajectoryKindTask,
	}
	_, err := s.CreateTrajectoryIfAbsentOG(ctx, rec)
	if err != nil {
		t.Fatalf("create trajectory: %v", err)
	}

	got, err := s.GetTrajectoryOG(ctx, "owner-og", "traj-og-2")
	if err != nil {
		t.Fatalf("get trajectory: %v", err)
	}
	if got.TrajectoryID != "traj-og-2" {
		t.Errorf("trajectory_id: got %q", got.TrajectoryID)
	}
}

func TestOGListTrajectoriesByOwner(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	for i := range 3 {
		rec := types.TrajectoryRecord{
			TrajectoryID: "traj-og-list-" + string(rune('A'+i)),
			OwnerID:      "owner-list",
			Kind:         types.TrajectoryKindTask,
		}
		if _, err := s.CreateTrajectoryIfAbsentOG(ctx, rec); err != nil {
			t.Fatalf("create trajectory %d: %v", i, err)
		}
	}

	trajs, err := s.ListTrajectoriesByOwnerOG(ctx, "owner-list", 10)
	if err != nil {
		t.Fatalf("list trajectories: %v", err)
	}
	if len(trajs) != 3 {
		t.Fatalf("expected 3 trajectories, got %d", len(trajs))
	}
}

func TestOGUpdateTrajectoryStatus(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	rec := types.TrajectoryRecord{
		TrajectoryID: "traj-og-status",
		OwnerID:      "owner-og",
		Kind:         types.TrajectoryKindTask,
	}
	_, err := s.CreateTrajectoryIfAbsentOG(ctx, rec)
	if err != nil {
		t.Fatalf("create trajectory: %v", err)
	}

	updated, err := s.UpdateTrajectoryStatusOG(ctx, "owner-og", "traj-og-status", types.TrajectorySettled)
	if err != nil {
		t.Fatalf("update status: %v", err)
	}
	if updated.Status != types.TrajectorySettled {
		t.Errorf("status: got %q, want %q", updated.Status, types.TrajectorySettled)
	}
	if updated.SettledAt == nil {
		t.Error("expected settled_at to be set")
	}
}

// =========================================================================
// Work Item tests
// =========================================================================

func TestOGCreateAndGetWorkItem(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	rec := types.WorkItemRecord{
		WorkItemID:   "wi-og-1",
		TrajectoryID: "traj-og-1",
		OwnerID:      "owner-og",
		Objective:    "test objective",
		Status:       types.WorkItemOpen,
	}
	created, err := s.CreateWorkItemOG(ctx, rec)
	if err != nil {
		t.Fatalf("create work item: %v", err)
	}
	if created.WorkItemID != rec.WorkItemID {
		t.Errorf("work_item_id: got %q", created.WorkItemID)
	}

	got, err := s.GetWorkItemOG(ctx, "owner-og", "wi-og-1")
	if err != nil {
		t.Fatalf("get work item: %v", err)
	}
	if got.Objective != rec.Objective {
		t.Errorf("objective: got %q, want %q", got.Objective, rec.Objective)
	}
}

func TestOGListWorkItemsByTrajectory(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	for i := range 3 {
		rec := types.WorkItemRecord{
			WorkItemID:   "wi-og-list-" + string(rune('A'+i)),
			TrajectoryID: "traj-og-list",
			OwnerID:      "owner-og",
			Objective:    "test",
			Status:       types.WorkItemOpen,
		}
		if _, err := s.CreateWorkItemOG(ctx, rec); err != nil {
			t.Fatalf("create work item %d: %v", i, err)
		}
	}
	// Add a completed item.
	completed := types.WorkItemRecord{
		WorkItemID:   "wi-og-completed",
		TrajectoryID: "traj-og-list",
		OwnerID:      "owner-og",
		Objective:    "done",
		Status:       types.WorkItemCompleted,
	}
	if _, err := s.CreateWorkItemOG(ctx, completed); err != nil {
		t.Fatalf("create completed work item: %v", err)
	}

	// List all.
	all, err := s.ListWorkItemsByTrajectoryOG(ctx, "owner-og", "traj-og-list", false)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("expected 4 work items, got %d", len(all))
	}

	// List open only.
	open, err := s.ListWorkItemsByTrajectoryOG(ctx, "owner-og", "traj-og-list", true)
	if err != nil {
		t.Fatalf("list open: %v", err)
	}
	if len(open) != 3 {
		t.Fatalf("expected 3 open work items, got %d", len(open))
	}
}

func TestOGUpdateWorkItemStatus(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	rec := types.WorkItemRecord{
		WorkItemID:   "wi-og-update",
		TrajectoryID: "traj-og-1",
		OwnerID:      "owner-og",
		Objective:    "test",
		Status:       types.WorkItemOpen,
	}
	if _, err := s.CreateWorkItemOG(ctx, rec); err != nil {
		t.Fatalf("create work item: %v", err)
	}

	updated, err := s.UpdateWorkItemStatusOG(ctx, "owner-og", "wi-og-update", types.WorkItemCompleted)
	if err != nil {
		t.Fatalf("update status: %v", err)
	}
	if updated.Status != types.WorkItemCompleted {
		t.Errorf("status: got %q, want %q", updated.Status, types.WorkItemCompleted)
	}

	// Verify by reading back.
	got, err := s.GetWorkItemOG(ctx, "owner-og", "wi-og-update")
	if err != nil {
		t.Fatalf("get work item: %v", err)
	}
	if got.Status != types.WorkItemCompleted {
		t.Errorf("status after read: got %q", got.Status)
	}
}

// =========================================================================
// Channel Message tests
// =========================================================================

func TestOGAppendAndListChannelMessages(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC()
	for i := range 3 {
		msg := &types.ChannelMessage{
			ChannelID: "ch-og-1",
			Seq:       int64(i + 1),
			From:      "agent-1",
			Role:      "worker",
			Content:   "test message",
			Timestamp: now.Add(time.Duration(i) * time.Second),
		}
		if err := s.AppendChannelMessageOG(ctx, msg, "owner-og"); err != nil {
			t.Fatalf("append message %d: %v", i, err)
		}
	}

	msgs, err := s.ListChannelMessagesOG(ctx, "owner-og", "ch-og-1", 0, 10)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
}

func TestOGListChannelMessagesAfterSeq(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC()
	for i := range 5 {
		msg := &types.ChannelMessage{
			ChannelID: "ch-og-2",
			Seq:       int64(i + 1),
			From:      "agent-1",
			Role:      "worker",
			Content:   "test",
			Timestamp: now.Add(time.Duration(i) * time.Second),
		}
		if err := s.AppendChannelMessageOG(ctx, msg, "owner-og"); err != nil {
			t.Fatalf("append message %d: %v", i, err)
		}
	}

	msgs, err := s.ListChannelMessagesOG(ctx, "owner-og", "ch-og-2", 2, 10)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages after seq 2, got %d", len(msgs))
	}
	for _, m := range msgs {
		if m.Seq <= 2 {
			t.Errorf("seq %d should be > 2", m.Seq)
		}
	}
}

// =========================================================================
// Inbox Delivery tests
// =========================================================================

func TestOGCreateAndGetInboxDelivery(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	rec := types.InboxDelivery{
		DeliveryID: "del-og-1",
		OwnerID:    "owner-og",
		ToAgentID:  "agent-1",
		Content:    "test delivery",
		CreatedAt:  time.Now().UTC(),
	}
	if err := s.CreateInboxDeliveryOG(ctx, rec); err != nil {
		t.Fatalf("create delivery: %v", err)
	}

	got, err := s.GetInboxDeliveryOG(ctx, "owner-og", "del-og-1")
	if err != nil {
		t.Fatalf("get delivery: %v", err)
	}
	if got.DeliveryID != rec.DeliveryID {
		t.Errorf("delivery_id: got %q", got.DeliveryID)
	}
	if got.Content != rec.Content {
		t.Errorf("content: got %q", got.Content)
	}
}
