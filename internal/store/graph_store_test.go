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
