package agentcore

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestPersistActivationStateDoesNotOverwriteStoreTerminalizedAssignmentRun(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	now := time.Now().UTC()
	stored := types.RunRecord{RunID: "run-store-terminal-assignment", AgentID: "co-super:terminal-assignment", OwnerID: "owner-store-terminal-assignment", ComputerID: rt.TextureComputerID(),
		AgentProfile: agentprofile.CoSuper, AgentRole: agentprofile.CoSuper, State: types.RunRunning, Prompt: "assigned work", CreatedAt: now, UpdatedAt: now,
		Metadata: map[string]any{"assignment_id": "assignment-terminal", "assignment_attempt": 1}}
	if err := s.CreateRun(ctx, stored); err != nil {
		t.Fatal(err)
	}
	finished := now.Add(time.Second)
	stored.State, stored.Error, stored.UpdatedAt, stored.FinishedAt = types.RunFailed, "assignment report failed", finished, &finished
	if err := s.UpdateRun(ctx, stored); err != nil {
		t.Fatal(err)
	}
	local := stored
	local.State, local.Error, local.Result, local.FinishedAt = types.RunCompleted, "", "provider final text after terminal tool", &finished
	persisted, err := rt.persistActivationState(ctx, &local)
	if err != nil || persisted || local.State != types.RunFailed || local.Error != "assignment report failed" || local.Result != "" {
		t.Fatalf("terminal assignment overwrite guard: persisted=%t local=%+v err=%v", persisted, local, err)
	}
	reloaded, err := s.GetRunByOwner(ctx, stored.OwnerID, stored.RunID)
	if err != nil || reloaded.State != types.RunFailed || reloaded.Error != "assignment report failed" {
		t.Fatalf("stored terminal changed: %+v err=%v", reloaded, err)
	}
}
