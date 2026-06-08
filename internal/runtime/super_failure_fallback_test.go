package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestRuntimeSynthesizesVTextBlockerWhenSuperFailsBeforeDelegation(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	ownerID := "user-super-fallback"
	docID := "doc-super-fallback"

	now := time.Now().UTC()
	superRun := &types.RunRecord{
		RunID:        "super-run-before-delegation",
		AgentID:      "super:" + ownerID,
		ChannelID:    docID,
		AgentProfile: AgentProfileSuper,
		AgentRole:    AgentProfileSuper,
		OwnerID:      ownerID,
		SandboxID:    "sandbox-test",
		State:        types.RunRunning,
		Prompt:       "Run MissionGradient continuation and delegate worker-medium",
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileSuper,
			runMetadataAgentRole:    AgentProfileSuper,
			runMetadataAgentID:      "super:" + ownerID,
			runMetadataChannelID:    docID,
			runMetadataTrajectoryID: "traj-super-before-delegation",
			"requested_by_agent_id": "vtext:" + docID,
			"requested_by_profile":  AgentProfileVText,
		},
	}
	if err := s.CreateRun(ctx, *superRun); err != nil {
		t.Fatalf("create super run: %v", err)
	}
	if err := s.UpsertAgent(ctx, types.AgentRecord{
		AgentID:   "super:" + ownerID,
		OwnerID:   ownerID,
		SandboxID: "sandbox-test",
		Profile:   AgentProfileSuper,
		Role:      AgentProfileSuper,
		ChannelID: docID,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert super agent: %v", err)
	}

	readOutput, _ := json.Marshal(map[string]any{"path": "docs/mission.md", "bytes": 1234})
	payload, _ := json.Marshal(map[string]any{
		"tool":     "read_file",
		"is_error": false,
		"output":   string(readOutput),
	})
	if err := s.AppendEvent(ctx, &types.EventRecord{
		EventID:      "event-super-read-before-delegation",
		RunID:        superRun.RunID,
		AgentID:      agentIDForRun(superRun),
		ChannelID:    docID,
		OwnerID:      ownerID,
		TrajectoryID: "traj-super-before-delegation",
		Timestamp:    time.Now().UTC(),
		Kind:         types.EventToolResult,
		Payload:      payload,
	}); err != nil {
		t.Fatalf("append read_file event: %v", err)
	}

	rt.handleExecutionError(ctx, superRun, fmt.Errorf("tool loop: model stopped at max_tokens (iteration 30)"))

	updates, err := s.ListWorkerUpdatesByTrajectory(ctx, ownerID, "traj-super-before-delegation", 10)
	if err != nil {
		t.Fatalf("list worker updates: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("worker updates = %d, want 1", len(updates))
	}
	update := updates[0]
	if update.Kind != "blocker" || update.TargetAgentID != "vtext:"+docID || update.ChannelID != docID {
		t.Fatalf("update kind/target/channel = %+v", update)
	}
	if !strings.Contains(strings.Join(update.Findings, "\n"), "No worker lease/delegation tool result was recorded") {
		t.Fatalf("findings missing no-worker blocker: %+v", update.Findings)
	}
	if !strings.Contains(strings.Join(update.Notes, "\n"), "successful tools: read_file") {
		t.Fatalf("notes missing successful tool summary: %+v", update.Notes)
	}
}
