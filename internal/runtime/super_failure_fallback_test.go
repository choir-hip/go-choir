package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
)

func TestRuntimeSynthesizesTextureBlockerWhenSuperFailsBeforeDelegation(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	ownerID := "user-super-fallback"
	docID := "doc-super-fallback"

	now := time.Now().UTC()
	superRun := &types.RunRecord{
		RunID:        "super-run-before-delegation",
		AgentID:      "super:" + ownerID,
		ChannelID:    docID,
		AgentProfile: agentprofile.Super,
		AgentRole:    agentprofile.Super,
		OwnerID:      ownerID,
		SandboxID:    "sandbox-test",
		State:        types.RunRunning,
		Prompt:       "Run MissionGradient continuation and delegate worker-medium",
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: agentprofile.Super,
			runMetadataAgentRole:    agentprofile.Super,
			runMetadataAgentID:      "super:" + ownerID,
			runMetadataChannelID:    docID,
			runMetadataTrajectoryID: "traj-super-before-delegation",
			"requested_by_agent_id": "texture:" + docID,
			"requested_by_profile":  agentprofile.Texture,
		},
	}
	if err := s.CreateRun(ctx, *superRun); err != nil {
		t.Fatalf("create super run: %v", err)
	}
	if err := s.UpsertAgent(ctx, types.AgentRecord{
		AgentID:   "super:" + ownerID,
		OwnerID:   ownerID,
		SandboxID: "sandbox-test",
		Profile:   agentprofile.Super,
		Role:      agentprofile.Super,
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
	if update.Packet.Kind != "blocker" || update.TargetAgentID != "texture:"+docID || update.ChannelID != docID {
		t.Fatalf("update kind/target/channel = %+v", update)
	}
	if !strings.Contains(coagentClaimsText(update.Packet.Claims), "No worker lease/delegation tool result was recorded") {
		t.Fatalf("claims missing no-worker blocker: %+v", update.Packet.Claims)
	}
	if !strings.Contains(strings.Join(update.Packet.Notes, "\n"), "successful tools: read_file") {
		t.Fatalf("notes missing successful tool summary: %+v", update.Packet.Notes)
	}
}

func TestRuntimeSynthesizesWorkerDelegationUpdateAfterStartWorkerDelegation(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	ownerID := "user-super-start-fallback"
	docID := "doc-super-start-fallback"

	now := time.Now().UTC()
	superRun := &types.RunRecord{
		RunID:        "super-run-after-start-delegation",
		AgentID:      "super:" + ownerID,
		ChannelID:    docID,
		AgentProfile: agentprofile.Super,
		AgentRole:    agentprofile.Super,
		OwnerID:      ownerID,
		SandboxID:    "sandbox-test",
		State:        types.RunRunning,
		Prompt:       "Run MissionGradient continuation and delegate worker-medium",
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: agentprofile.Super,
			runMetadataAgentRole:    agentprofile.Super,
			runMetadataAgentID:      "super:" + ownerID,
			runMetadataChannelID:    docID,
			runMetadataTrajectoryID: "traj-super-after-start-delegation",
			"requested_by_agent_id": "texture:" + docID,
			"requested_by_profile":  agentprofile.Texture,
		},
	}
	if err := s.CreateRun(ctx, *superRun); err != nil {
		t.Fatalf("create super run: %v", err)
	}
	if err := s.UpsertAgent(ctx, types.AgentRecord{
		AgentID:   "super:" + ownerID,
		OwnerID:   ownerID,
		SandboxID: "sandbox-test",
		Profile:   agentprofile.Super,
		Role:      agentprofile.Super,
		ChannelID: docID,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert super agent: %v", err)
	}

	startOutput, _ := json.Marshal(map[string]any{
		"status":              "worker_run_incomplete",
		"state":               "failed",
		"worker_run_id":       "worker-run-started",
		"worker_id":           "worker-1",
		"worker_vm_id":        "vm-1",
		"completion_blocker":  "worker_failed_before_package",
		"terminal_error":      "worker failed before AppChangePackage evidence",
		"app_change_packages": []map[string]any{},
	})
	payload, _ := json.Marshal(map[string]any{
		"tool":     "start_worker_delegation",
		"is_error": false,
		"output":   string(startOutput),
	})
	if err := s.AppendEvent(ctx, &types.EventRecord{
		EventID:      "event-start-worker-delegation",
		RunID:        superRun.RunID,
		AgentID:      agentIDForRun(superRun),
		ChannelID:    docID,
		OwnerID:      ownerID,
		TrajectoryID: "traj-super-after-start-delegation",
		Timestamp:    time.Now().UTC(),
		Kind:         types.EventToolResult,
		Payload:      payload,
	}); err != nil {
		t.Fatalf("append start_worker_delegation event: %v", err)
	}

	rt.handleExecutionError(ctx, superRun, fmt.Errorf("tool loop iteration 7: gateway call failed"))

	updates, err := s.ListWorkerUpdatesByTrajectory(ctx, ownerID, "traj-super-after-start-delegation", 10)
	if err != nil {
		t.Fatalf("list worker updates: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("worker updates = %d, want 1", len(updates))
	}
	update := updates[0]
	findings := coagentClaimsText(update.Packet.Claims)
	if !strings.Contains(findings, "worker delegation returned status") {
		t.Fatalf("claims missing worker delegation summary: %+v", update.Packet.Claims)
	}
	if strings.Contains(findings, "No worker lease/delegation tool result was recorded") {
		t.Fatalf("claims used stale before-delegation fallback: %+v", update.Packet.Claims)
	}
	if !strings.Contains(strings.Join(update.Packet.Notes, "\n"), "delegate_terminal_error=worker failed before AppChangePackage evidence") {
		t.Fatalf("notes missing worker terminal error: %+v", update.Packet.Notes)
	}
}
