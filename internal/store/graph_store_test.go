package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestGetAgentByScopeSeparatesComputers(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	for _, rec := range []types.AgentRecord{
		{AgentID: "processor:doc-1", OwnerID: "owner-og", ComputerID: "computer-a", Profile: "processor", Role: "processor", ChannelID: "channel-a", CreatedAt: now, UpdatedAt: now},
		{AgentID: "processor:doc-1", OwnerID: "owner-og", ComputerID: "computer-b", Profile: "processor", Role: "processor", ChannelID: "channel-b", CreatedAt: now, UpdatedAt: now},
	} {
		if err := s.UpsertAgentOG(ctx, rec); err != nil {
			t.Fatalf("upsert scoped agent: %v", err)
		}
	}
	got, err := s.GetAgentByScopeOG(ctx, "owner-og", "computer-b", "processor:doc-1")
	if err != nil {
		t.Fatalf("get scoped agent: %v", err)
	}
	if got.ChannelID != "channel-b" {
		t.Fatalf("channel_id = %q, want channel-b", got.ChannelID)
	}
	if _, err := s.GetAgentByScopeOG(ctx, "owner-og", "computer-c", "processor:doc-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing scope error = %v, want ErrNotFound", err)
	}
}

func TestResolveLegacyAgentScopeRequiresExactlyOneOwner(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	base := types.AgentRecord{
		AgentID: "legacy-agent", OwnerID: "owner-a", ComputerID: "computer-a",
		Profile: "processor", Role: "processor", ChannelID: "channel-a", CreatedAt: now, UpdatedAt: now,
	}
	if err := s.UpsertAgentOG(ctx, base); err != nil {
		t.Fatalf("upsert unique legacy agent: %v", err)
	}
	got, err := s.ResolveLegacyAgentScopeOG(ctx, base.ComputerID, base.AgentID)
	if err != nil || got.OwnerID != base.OwnerID {
		t.Fatalf("resolve unique legacy agent: %+v, %v", got, err)
	}
	other := base
	other.OwnerID = "owner-b"
	other.ChannelID = "channel-b"
	if err := s.UpsertAgentOG(ctx, other); err != nil {
		t.Fatalf("upsert ambiguous legacy agent: %v", err)
	}
	if _, err := s.ResolveLegacyAgentScopeOG(ctx, base.ComputerID, base.AgentID); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("ambiguous scope error = %v", err)
	}
	if _, err := s.ResolveLegacyAgentScopeOG(ctx, base.ComputerID, "missing-agent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing scope error = %v, want ErrNotFound", err)
	}
}

func TestResolveLegacyAgentScopeUsesUniqueRunWitness(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	const (
		agentID    = "legacy-agent-without-record"
		computerID = "computer-a"
	)
	for _, runID := range []string{"run-a", "run-b"} {
		if err := s.CreateRunOG(ctx, types.RunRecord{
			RunID: runID, AgentID: agentID, OwnerID: "owner-a", ComputerID: computerID,
			State: types.RunCompleted, CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatalf("create run witness %s: %v", runID, err)
		}
	}
	got, err := s.ResolveLegacyAgentScopeOG(ctx, computerID, agentID)
	if err != nil || got.OwnerID != "owner-a" || got.ComputerID != computerID || got.AgentID != agentID {
		t.Fatalf("resolve run witness: %+v, %v", got, err)
	}
	if _, err := s.ResolveLegacyAgentScopeOG(ctx, "computer-b", agentID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong-computer run witness error = %v, want ErrNotFound", err)
	}
	if err := s.CreateRunOG(ctx, types.RunRecord{
		RunID: "run-other-owner", AgentID: agentID, OwnerID: "owner-b", ComputerID: computerID,
		State: types.RunCompleted, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create ambiguous run witness: %v", err)
	}
	if _, err := s.ResolveLegacyAgentScopeOG(ctx, computerID, agentID); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("ambiguous run witness error = %v", err)
	}
}

func TestListAllRunsByStateOGExhaustsKeysetPages(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	for i := range 3 {
		rec := types.RunRecord{
			RunID:      "run-og-state-page-" + string(rune('A'+i)),
			AgentID:    "research:page",
			OwnerID:    "owner-state-page",
			ComputerID: "autoputer-1",
			State:      types.RunCompleted,
			CreatedAt:  now,
			UpdatedAt:  now,
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

func TestForEachRunsByStateExhaustsKeysetPages(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	for i := range 3 {
		rec := types.RunRecord{
			RunID:      "run-og-foreach-page-" + string(rune('A'+i)),
			AgentID:    "research:foreach-page",
			OwnerID:    "owner-foreach-page",
			ComputerID: "autoputer-1",
			State:      types.RunCompleted,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if err := s.CreateRunOG(ctx, rec); err != nil {
			t.Fatalf("create foreach terminal run %d: %v", i, err)
		}
	}
	var seen []string
	if err := s.forEachRunsByStatePageSize(ctx, types.RunCompleted, 2, func(rec types.RunRecord) error {
		seen = append(seen, rec.RunID)
		return nil
	}); err != nil {
		t.Fatalf("foreach terminal runs across pages: %v", err)
	}
	if len(seen) != 3 {
		t.Fatalf("foreach terminal runs across keyset pages = %d, want 3: %v", len(seen), seen)
	}
	limited, err := s.ListRunsByStateOG(ctx, types.RunCompleted, 2)
	if err != nil {
		t.Fatalf("limited list by state: %v", err)
	}
	if len(limited) != 2 {
		t.Fatalf("limited list by state = %d, want 2: %+v", len(limited), limited)
	}
}

func TestListRecentRunsByOwnerLoadsRequestedChildrenOnly(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	ownerID := "owner-recent-window"
	child := types.RunRecord{
		RunID:            "run-recent-child",
		AgentID:          "engineering:recent-child",
		RequestedByRunID: "parent-recent",
		OwnerID:          ownerID,
		ComputerID:       "autoputer-1",
		State:            types.RunCompleted,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	root := types.RunRecord{
		RunID:      "run-recent-root",
		AgentID:    "research:recent-root",
		OwnerID:    ownerID,
		ComputerID: "autoputer-1",
		State:      types.RunCompleted,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	foreign := types.RunRecord{
		RunID:            "run-recent-foreign",
		AgentID:          "engineering:recent-foreign",
		RequestedByRunID: "parent-recent-foreign",
		OwnerID:          "owner-recent-foreign",
		ComputerID:       "autoputer-1",
		State:            types.RunCompleted,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	for _, rec := range []types.RunRecord{child, root, foreign} {
		if err := s.CreateRunOG(ctx, rec); err != nil {
			t.Fatalf("create %s: %v", rec.RunID, err)
		}
	}
	got, err := s.ListRecentRunsByOwner(ctx, ownerID, "autoputer-1", 16)
	if err != nil {
		t.Fatalf("list recent runs: %v", err)
	}
	if len(got) != 1 || got[0].RunID != child.RunID {
		t.Fatalf("recent delegated children = %+v, want [%s]", got, child.RunID)
	}
}

func TestListPassivatedPersistentManagementControlRunsByOwnerLoadsManagementOnly(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	ownerID := "owner-passivated-super"
	managementRun := types.RunRecord{
		RunID: "run-passivated-super", AgentID: "management:" + ownerID, OwnerID: ownerID,
		ComputerID: "autoputer-1", AgentProfile: "management", AgentRole: "management",
		State: types.RunPassivated, CreatedAt: now, UpdatedAt: now,
		Metadata: map[string]any{"request_source": "lifecycle_texture_control", "passivated_reason": "runtime_restarted"},
	}
	research := types.RunRecord{
		RunID: "run-passivated-researcher", AgentID: "research:passivated", OwnerID: ownerID,
		ComputerID: "autoputer-1", AgentProfile: "research", AgentRole: "research",
		State: types.RunPassivated, CreatedAt: now, UpdatedAt: now,
	}
	foreign := types.RunRecord{
		RunID: "run-passivated-super-foreign", AgentID: "management:owner-passivated-super-foreign",
		OwnerID: "owner-passivated-super-foreign", ComputerID: "autoputer-1",
		AgentProfile: "management", AgentRole: "management", State: types.RunPassivated,
		CreatedAt: now, UpdatedAt: now,
	}
	completed := types.RunRecord{
		RunID: "run-completed-super", AgentID: "management:" + ownerID, OwnerID: ownerID,
		ComputerID: "autoputer-1", AgentProfile: "management", AgentRole: "management",
		State: types.RunCompleted, CreatedAt: now, UpdatedAt: now,
	}
	for _, rec := range []types.RunRecord{managementRun, research, foreign, completed} {
		if err := s.CreateRunOG(ctx, rec); err != nil {
			t.Fatalf("create %s: %v", rec.RunID, err)
		}
	}
	got, err := s.ListPassivatedPersistentManagementControlRunsByOwner(ctx, ownerID, "autoputer-1", "", 16)
	if err != nil {
		t.Fatalf("list passivated super runs: %v", err)
	}
	if len(got) != 1 || got[0].RunID != managementRun.RunID {
		t.Fatalf("passivated super runs = %+v, want [%s]", got, managementRun.RunID)
	}
	byAgent, err := s.ListPassivatedPersistentManagementControlRunsByOwner(ctx, ownerID, "autoputer-1", "management:"+ownerID, 16)
	if err != nil {
		t.Fatalf("list passivated super runs by agent: %v", err)
	}
	if len(byAgent) != 1 || byAgent[0].RunID != managementRun.RunID {
		t.Fatalf("passivated super runs by agent = %+v, want [%s]", byAgent, managementRun.RunID)
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

func TestListRunsByOwnerStatesLoadsMatchingStatesOnly(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	ownerID := "owner-run-states"
	pending := types.RunRecord{
		RunID: "run-pending-owner", AgentID: "agent-pending", OwnerID: ownerID,
		ComputerID: "autoputer-1", AgentProfile: "management", AgentRole: "management",
		State: types.RunPending, CreatedAt: now, UpdatedAt: now,
	}
	running := types.RunRecord{
		RunID: "run-running-owner", AgentID: "agent-running", OwnerID: ownerID,
		ComputerID: "autoputer-1", AgentProfile: "research", AgentRole: "research",
		State: types.RunRunning, CreatedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second),
	}
	completed := types.RunRecord{
		RunID: "run-completed-owner", AgentID: "agent-completed", OwnerID: ownerID,
		ComputerID: "autoputer-1", AgentProfile: "management", AgentRole: "management",
		State: types.RunCompleted, CreatedAt: now, UpdatedAt: now,
	}
	foreign := types.RunRecord{
		RunID: "run-pending-foreign", AgentID: "agent-foreign", OwnerID: "owner-run-states-foreign",
		ComputerID: "autoputer-1", AgentProfile: "management", AgentRole: "management",
		State: types.RunPending, CreatedAt: now, UpdatedAt: now,
	}
	for _, rec := range []types.RunRecord{pending, running, completed, foreign} {
		if err := s.CreateRunOG(ctx, rec); err != nil {
			t.Fatalf("create %s: %v", rec.RunID, err)
		}
	}
	got, err := s.ListRunsByOwnerStates(ctx, ownerID, "autoputer-1", []types.RunState{types.RunPending, types.RunRunning}, 16)
	if err != nil {
		t.Fatalf("list runs by owner states: %v", err)
	}
	ids := map[string]bool{}
	for _, rec := range got {
		ids[rec.RunID] = true
	}
	if !ids[pending.RunID] || !ids[running.RunID] || ids[completed.RunID] || ids[foreign.RunID] || len(got) != 2 {
		t.Fatalf("owner states = %+v", got)
	}
}

func TestAppendChannelMessageIdempotentReplay(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	msg := &types.ChannelMessage{
		ChannelID:      "ch-idem",
		From:           "engineering",
		FromAgentID:    "engineering:impl",
		ToAgentID:      "management",
		Role:           "engineering",
		Content:        "hello",
		IdempotencyKey: "rlm:cell-1:tray-1",
	}
	if err := s.AppendChannelMessage(ctx, msg, "owner-idem"); err != nil {
		t.Fatal(err)
	}
	first := msg.Seq
	replay := &types.ChannelMessage{
		ChannelID:      "ch-idem",
		From:           "engineering",
		FromAgentID:    "engineering:impl",
		ToAgentID:      "management",
		Role:           "engineering",
		Content:        "hello",
		IdempotencyKey: "rlm:cell-1:tray-1",
	}
	if err := s.AppendChannelMessage(ctx, replay, "owner-idem"); err != nil {
		t.Fatal(err)
	}
	if replay.Seq != first {
		t.Fatalf("replay seq = %d, want %d", replay.Seq, first)
	}
	if !replay.Replayed {
		t.Fatal("replay must set ChannelMessage.Replayed")
	}
	conflict := &types.ChannelMessage{
		ChannelID:      "ch-idem",
		From:           "engineering",
		FromAgentID:    "engineering:impl",
		ToAgentID:      "management",
		Role:           "engineering",
		Content:        "changed",
		IdempotencyKey: "rlm:cell-1:tray-1",
	}
	if err := s.AppendChannelMessage(ctx, conflict, "owner-idem"); err == nil {
		t.Fatal("changed content under the same key must conflict")
	}
}
