package store

import (
	"context"
	"encoding/json"
	"errors"
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
		RunID:        "run-og-1",
		AgentID:      "agent-og-1",
		OwnerID:      "owner-og",
		SandboxID:    "sandbox-1",
		State:        types.RunRunning,
		Prompt:       "test prompt",
		CreatedAt:    now,
		UpdatedAt:    now,
		TrajectoryID: "traj-1",
		Metadata:     map[string]any{"key": "value"},
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

	ownerScoped, err := s.GetRunByOwnerOG(ctx, rec.OwnerID, rec.RunID)
	if err != nil {
		t.Fatalf("get owner-scoped run: %v", err)
	}
	if ownerScoped.RunID != rec.RunID || ownerScoped.OwnerID != rec.OwnerID {
		t.Fatalf("owner-scoped run = %+v, want run %q owned by %q", ownerScoped, rec.RunID, rec.OwnerID)
	}
	if _, err := s.GetRunByOwnerOG(ctx, "another-owner", rec.RunID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong-owner lookup error = %v, want ErrNotFound", err)
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

func TestListAllRunsByStateOGExhaustsKeysetPages(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	for i := range 3 {
		rec := types.RunRecord{
			RunID:     "run-og-state-page-" + string(rune('A'+i)),
			AgentID:   "researcher:page",
			OwnerID:   "owner-state-page",
			SandboxID: "sandbox-1",
			State:     types.RunCompleted,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := s.CreateRunOG(ctx, rec); err != nil {
			t.Fatalf("create paged terminal run %d: %v", i, err)
		}
	}
	runs, err := s.listAllRunsByStateOG(ctx, types.RunCompleted, 2)
	if err != nil {
		t.Fatalf("list all terminal runs across pages: %v", err)
	}
	if len(runs) != 3 {
		t.Fatalf("terminal runs across keyset pages = %d, want 3: %+v", len(runs), runs)
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

// =========================================================================
// Run Memory Entry tests
// =========================================================================

func TestOGAppendAndListRunMemoryEntries(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC()
	for i := range 3 {
		rec := types.RunMemoryEntry{
			EntryID:   "mem-og-" + string(rune('A'+i)),
			RunID:     "run-og-mem",
			OwnerID:   "owner-og",
			Seq:       int64(i + 1),
			Kind:      types.RunMemoryEntryMessage,
			Message:   json.RawMessage(`{"role":"user"}`),
			CreatedAt: now.Add(time.Duration(i) * time.Second),
		}
		if err := s.AppendRunMemoryEntryOG(ctx, rec); err != nil {
			t.Fatalf("append memory %d: %v", i, err)
		}
	}

	entries, err := s.ListRunMemoryEntriesOG(ctx, "owner-og", "run-og-mem", 100)
	if err != nil {
		t.Fatalf("list memory: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
}

// =========================================================================
// Run Acceptance tests
// =========================================================================

func TestOGCreateAndGetRunAcceptance(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	rec := types.RunAcceptanceRecord{
		AcceptanceID:    "acc-og-1",
		TargetMissionID: "mission-1",
		OwnerID:         "owner-og",
		TrajectoryID:    "traj-og-1",
		RunID:           "run-og-1",
		AcceptanceLevel: types.RunAcceptanceDocsLevel,
		State:           types.RunAcceptanceSynthesized,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}
	if err := s.CreateRunAcceptanceOG(ctx, rec); err != nil {
		t.Fatalf("create acceptance: %v", err)
	}

	got, err := s.GetRunAcceptanceOG(ctx, "owner-og", "acc-og-1")
	if err != nil {
		t.Fatalf("get acceptance: %v", err)
	}
	if got.AcceptanceID != rec.AcceptanceID {
		t.Errorf("acceptance_id: got %q", got.AcceptanceID)
	}
	if got.TrajectoryID != rec.TrajectoryID {
		t.Errorf("trajectory_id: got %q", got.TrajectoryID)
	}
}

func TestOGListRunAcceptancesByTrajectory(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	for i := range 2 {
		rec := types.RunAcceptanceRecord{
			AcceptanceID:    "acc-og-list-" + string(rune('A'+i)),
			TargetMissionID: "mission-1",
			OwnerID:         "owner-og",
			TrajectoryID:    "traj-og-list",
			AcceptanceLevel: types.RunAcceptanceDocsLevel,
			State:           types.RunAcceptanceSynthesized,
			CreatedAt:       time.Now().UTC(),
			UpdatedAt:       time.Now().UTC(),
		}
		if err := s.CreateRunAcceptanceOG(ctx, rec); err != nil {
			t.Fatalf("create acceptance %d: %v", i, err)
		}
	}

	accepts, err := s.ListRunAcceptancesByTrajectoryOG(ctx, "owner-og", "traj-og-list", 10)
	if err != nil {
		t.Fatalf("list acceptances: %v", err)
	}
	if len(accepts) != 2 {
		t.Fatalf("expected 2 acceptances, got %d", len(accepts))
	}
}

// =========================================================================
// Run Continuation tests
// =========================================================================

func TestOGCreateAndGetRunContinuation(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	rec := types.RunContinuationRecord{
		ContinuationID: "cont-og-1",
		OwnerID:        "owner-og",
		SourceRunID:    "run-og-1",
		Objective:      "next objective",
		Status:         types.RunContinuationSelected,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	if err := s.CreateRunContinuationOG(ctx, rec); err != nil {
		t.Fatalf("create continuation: %v", err)
	}

	got, err := s.GetRunContinuationOG(ctx, "owner-og", "cont-og-1")
	if err != nil {
		t.Fatalf("get continuation: %v", err)
	}
	if got.ContinuationID != rec.ContinuationID {
		t.Errorf("continuation_id: got %q", got.ContinuationID)
	}
	if got.SourceRunID != rec.SourceRunID {
		t.Errorf("source_run_id: got %q", got.SourceRunID)
	}
	if got.Objective != rec.Objective {
		t.Errorf("objective: got %q", got.Objective)
	}
}

func TestOGListRunContinuationsBySourceRun(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	for i := range 2 {
		rec := types.RunContinuationRecord{
			ContinuationID: "cont-og-list-" + string(rune('A'+i)),
			OwnerID:        "owner-og",
			SourceRunID:    "run-og-source",
			Objective:      "objective",
			Status:         types.RunContinuationSelected,
			CreatedAt:      time.Now().UTC(),
			UpdatedAt:      time.Now().UTC(),
		}
		if err := s.CreateRunContinuationOG(ctx, rec); err != nil {
			t.Fatalf("create continuation %d: %v", i, err)
		}
	}

	contins, err := s.ListRunContinuationsBySourceRunOG(ctx, "owner-og", "run-og-source", 10)
	if err != nil {
		t.Fatalf("list continuations: %v", err)
	}
	if len(contins) != 2 {
		t.Fatalf("expected 2 continuations, got %d", len(contins))
	}
}

// =========================================================================
// Texture Document tests
// =========================================================================

func TestOGCreateAndGetTextureDocument(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	rec := types.Document{
		DocID:     "doc-og-1",
		OwnerID:   "owner-og",
		Title:     "Test Document",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.CreateTextureDocumentOG(ctx, rec); err != nil {
		t.Fatalf("create document: %v", err)
	}

	got, err := s.GetTextureDocumentOG(ctx, "owner-og", "doc-og-1")
	if err != nil {
		t.Fatalf("get document: %v", err)
	}
	if got.DocID != rec.DocID {
		t.Errorf("doc_id: got %q", got.DocID)
	}
	if got.Title != rec.Title {
		t.Errorf("title: got %q", got.Title)
	}
}

func TestOGListTextureDocumentsByOwner(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	for i := range 3 {
		rec := types.Document{
			DocID:     "doc-og-list-" + string(rune('A'+i)),
			OwnerID:   "owner-list",
			Title:     "Test",
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if err := s.CreateTextureDocumentOG(ctx, rec); err != nil {
			t.Fatalf("create document %d: %v", i, err)
		}
	}

	docs, err := s.ListTextureDocumentsByOwnerOG(ctx, "owner-list", 10)
	if err != nil {
		t.Fatalf("list documents: %v", err)
	}
	if len(docs) != 3 {
		t.Fatalf("expected 3 documents, got %d", len(docs))
	}
}

func TestOGUpdateTextureDocument(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	rec := types.Document{
		DocID:     "doc-og-update",
		OwnerID:   "owner-og",
		Title:     "Original",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.CreateTextureDocumentOG(ctx, rec); err != nil {
		t.Fatalf("create document: %v", err)
	}

	rec.Title = "Updated"
	rec.CurrentRevisionID = "rev-1"
	rec.UpdatedAt = time.Now().UTC()
	if err := s.UpdateTextureDocumentOG(ctx, rec); err != nil {
		t.Fatalf("update document: %v", err)
	}

	got, err := s.GetTextureDocumentOG(ctx, "owner-og", "doc-og-update")
	if err != nil {
		t.Fatalf("get document: %v", err)
	}
	if got.Title != "Updated" {
		t.Errorf("title: got %q, want %q", got.Title, "Updated")
	}
	if got.CurrentRevisionID != "rev-1" {
		t.Errorf("current_revision_id: got %q", got.CurrentRevisionID)
	}
}

// =========================================================================
// Texture Revision tests
// =========================================================================

func TestOGCreateAndGetTextureRevision(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	// Create a document first.
	doc := types.Document{
		DocID:     "doc-og-rev",
		OwnerID:   "owner-og",
		Title:     "Test",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.CreateTextureDocumentOG(ctx, doc); err != nil {
		t.Fatalf("create document: %v", err)
	}

	rec := types.Revision{
		RevisionID:    "rev-og-1",
		DocID:         "doc-og-rev",
		OwnerID:       "owner-og",
		AuthorKind:    types.AuthorUser,
		AuthorLabel:   "user",
		VersionNumber: 0,
		Content:       "test content",
		CreatedAt:     time.Now().UTC(),
	}
	if err := s.CreateTextureRevisionOG(ctx, rec); err != nil {
		t.Fatalf("create revision: %v", err)
	}

	got, err := s.GetTextureRevisionOG(ctx, "owner-og", "rev-og-1")
	if err != nil {
		t.Fatalf("get revision: %v", err)
	}
	if got.RevisionID != rec.RevisionID {
		t.Errorf("revision_id: got %q", got.RevisionID)
	}
	if got.Content != rec.Content {
		t.Errorf("content: got %q", got.Content)
	}
}

func TestOGListTextureRevisionsByDoc(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	for i := range 3 {
		rec := types.Revision{
			RevisionID:    "rev-og-list-" + string(rune('A'+i)),
			DocID:         "doc-og-list",
			OwnerID:       "owner-og",
			AuthorKind:    types.AuthorUser,
			AuthorLabel:   "user",
			VersionNumber: i,
			Content:       "test",
			CreatedAt:     time.Now().UTC(),
		}
		if err := s.CreateTextureRevisionOG(ctx, rec); err != nil {
			t.Fatalf("create revision %d: %v", i, err)
		}
	}

	revisions, err := s.ListTextureRevisionsByDocOG(ctx, "owner-og", "doc-og-list", 100)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	if len(revisions) != 3 {
		t.Fatalf("expected 3 revisions, got %d", len(revisions))
	}
}

// =========================================================================
// Texture Decision tests
// =========================================================================

func TestOGCreateAndListTextureDecisions(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	for i := range 2 {
		rec := types.TextureDecisionRecord{
			DecisionID:   "dec-og-" + string(rune('A'+i)),
			OwnerID:      "owner-og",
			DocID:        "doc-og-dec",
			DecisionKind: "open",
			Reason:       "test reason",
			CreatedAt:    time.Now().UTC(),
		}
		if err := s.CreateTextureDecisionOG(ctx, rec); err != nil {
			t.Fatalf("create decision %d: %v", i, err)
		}
	}

	decisions, err := s.ListTextureDecisionsByDocOG(ctx, "owner-og", "doc-og-dec", 10)
	if err != nil {
		t.Fatalf("list decisions: %v", err)
	}
	if len(decisions) != 2 {
		t.Fatalf("expected 2 decisions, got %d", len(decisions))
	}
}

// =========================================================================
// Evidence tests
// =========================================================================

func TestOGCreateAndGetEvidence(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	rec := types.EvidenceRecord{
		EvidenceID: "ev-og-1",
		OwnerID:    "owner-og",
		AgentID:    "agent-og-1",
		Kind:       "observation",
		Content:    "test evidence",
		CreatedAt:  time.Now().UTC(),
	}
	if err := s.CreateEvidenceOG(ctx, rec); err != nil {
		t.Fatalf("create evidence: %v", err)
	}

	got, err := s.GetEvidenceOG(ctx, "owner-og", "ev-og-1")
	if err != nil {
		t.Fatalf("get evidence: %v", err)
	}
	if got.EvidenceID != rec.EvidenceID {
		t.Errorf("evidence_id: got %q", got.EvidenceID)
	}
	if got.Content != rec.Content {
		t.Errorf("content: got %q", got.Content)
	}
}

func TestOGListEvidenceByAgent(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	for i := range 3 {
		rec := types.EvidenceRecord{
			EvidenceID: "ev-og-list-" + string(rune('A'+i)),
			OwnerID:    "owner-og",
			AgentID:    "agent-og-list",
			Kind:       "observation",
			Content:    "test",
			CreatedAt:  time.Now().UTC(),
		}
		if err := s.CreateEvidenceOG(ctx, rec); err != nil {
			t.Fatalf("create evidence %d: %v", i, err)
		}
	}

	records, err := s.ListEvidenceByAgentOG(ctx, "owner-og", "agent-og-list", 10)
	if err != nil {
		t.Fatalf("list evidence: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}
}

// =========================================================================
// Content Item tests
// =========================================================================

func TestOGCreateAndGetContentItem(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	rec := types.ContentItem{
		ContentID:  "content-og-1",
		OwnerID:    "owner-og",
		SourceType: "web",
		MediaType:  "text/html",
		Title:      "Test Content",
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := s.CreateContentItemOG(ctx, rec); err != nil {
		t.Fatalf("create content item: %v", err)
	}

	got, err := s.GetContentItemOG(ctx, "owner-og", "content-og-1")
	if err != nil {
		t.Fatalf("get content item: %v", err)
	}
	if got.ContentID != rec.ContentID {
		t.Errorf("content_id: got %q", got.ContentID)
	}
	if got.Title != rec.Title {
		t.Errorf("title: got %q", got.Title)
	}
}

func TestOGListContentItemsByOwner(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	for i := range 3 {
		rec := types.ContentItem{
			ContentID:  "content-og-list-" + string(rune('A'+i)),
			OwnerID:    "owner-list",
			SourceType: "web",
			MediaType:  "text/html",
			CreatedAt:  time.Now().UTC(),
			UpdatedAt:  time.Now().UTC(),
		}
		if err := s.CreateContentItemOG(ctx, rec); err != nil {
			t.Fatalf("create content item %d: %v", i, err)
		}
	}

	items, err := s.ListContentItemsByOwnerOG(ctx, "owner-list", 10)
	if err != nil {
		t.Fatalf("list content items: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
}

// =========================================================================
// Podcast Subscription tests
// =========================================================================

func TestOGCreateAndGetPodcastSubscription(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	rec := types.PodcastSubscription{
		SubscriptionID: "sub-og-1",
		OwnerID:        "owner-og",
		FeedURL:        "https://example.com/feed.xml",
		Title:          "Test Podcast",
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	if err := s.CreatePodcastSubscriptionOG(ctx, rec); err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	got, err := s.GetPodcastSubscriptionOG(ctx, "owner-og", "sub-og-1")
	if err != nil {
		t.Fatalf("get subscription: %v", err)
	}
	if got.SubscriptionID != rec.SubscriptionID {
		t.Errorf("subscription_id: got %q", got.SubscriptionID)
	}
	if got.FeedURL != rec.FeedURL {
		t.Errorf("feed_url: got %q", got.FeedURL)
	}
}

func TestOGListPodcastSubscriptionsByOwner(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	for i := range 2 {
		rec := types.PodcastSubscription{
			SubscriptionID: "sub-og-list-" + string(rune('A'+i)),
			OwnerID:        "owner-list",
			FeedURL:        "https://example.com/feed.xml",
			CreatedAt:      time.Now().UTC(),
			UpdatedAt:      time.Now().UTC(),
		}
		if err := s.CreatePodcastSubscriptionOG(ctx, rec); err != nil {
			t.Fatalf("create subscription %d: %v", i, err)
		}
	}

	subs, err := s.ListPodcastSubscriptionsByOwnerOG(ctx, "owner-list", 10)
	if err != nil {
		t.Fatalf("list subscriptions: %v", err)
	}
	if len(subs) != 2 {
		t.Fatalf("expected 2 subscriptions, got %d", len(subs))
	}
}

// =========================================================================
// Browser Session tests
// =========================================================================

func TestOGCreateAndGetBrowserSession(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	rec := types.BrowserSessionRecord{
		SessionID: "bs-og-1",
		OwnerID:   "owner-og",
		Provider:  "playwright",
		Mode:      "headless",
		State:     "active",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.CreateBrowserSessionOG(ctx, rec); err != nil {
		t.Fatalf("create browser session: %v", err)
	}

	got, err := s.GetBrowserSessionOG(ctx, "owner-og", "bs-og-1")
	if err != nil {
		t.Fatalf("get browser session: %v", err)
	}
	if got.SessionID != rec.SessionID {
		t.Errorf("session_id: got %q", got.SessionID)
	}
	if got.State != rec.State {
		t.Errorf("state: got %q", got.State)
	}
}

func TestOGListBrowserSessionsByOwner(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	for i := range 2 {
		rec := types.BrowserSessionRecord{
			SessionID: "bs-og-list-" + string(rune('A'+i)),
			OwnerID:   "owner-list",
			Provider:  "playwright",
			Mode:      "headless",
			State:     "active",
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if err := s.CreateBrowserSessionOG(ctx, rec); err != nil {
			t.Fatalf("create session %d: %v", i, err)
		}
	}

	sessions, err := s.ListBrowserSessionsByOwnerOG(ctx, "owner-list", 10)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
}

// =========================================================================
// App Change Package tests
// =========================================================================

// =========================================================================
// App Adoption tests
// =========================================================================

// =========================================================================
// Desktop State tests
// =========================================================================

func TestOGSaveAndGetDesktopState(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	rec := types.DesktopState{
		OwnerID:   "owner-og",
		DesktopID: "desktop-og-1",
		Windows: []types.WindowState{
			{WindowID: "win-1", AppID: "texture", Title: "Test Window"},
		},
		ActiveWindowID: "win-1",
		UpdatedAt:      time.Now().UTC(),
	}
	if err := s.SaveDesktopStateOG(ctx, rec); err != nil {
		t.Fatalf("save desktop state: %v", err)
	}

	got, err := s.GetDesktopStateOG(ctx, "owner-og", "desktop-og-1")
	if err != nil {
		t.Fatalf("get desktop state: %v", err)
	}
	if got.DesktopID != rec.DesktopID {
		t.Errorf("desktop_id: got %q", got.DesktopID)
	}
	if got.ActiveWindowID != rec.ActiveWindowID {
		t.Errorf("active_window_id: got %q", got.ActiveWindowID)
	}
	if len(got.Windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(got.Windows))
	}
}
