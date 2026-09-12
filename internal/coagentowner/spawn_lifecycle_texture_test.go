package coagentowner

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestLifecycleTextureSpawnAgentFailsClosedBeforeGenericMutation(t *testing.T) {
	registry := toolregistry.NewToolRegistry()
	if err := RegisterSpawnTool(registry, nil, nil, agentprofile.PolicyFor(agentprofile.Texture)); err != nil {
		t.Fatal(err)
	}
	run := &types.RunRecord{
		RunID: "run-lifecycle-texture", OwnerID: "owner", ComputerID: "computer", AgentID: "texture:doc", ChannelID: "doc",
		TrajectoryID: "trajectory", AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture,
		Metadata: map[string]any{"lifecycle_work_item_id": "work-texture"},
	}
	exec := toolregistry.ExecutionContext{RunID: run.RunID, OwnerID: run.OwnerID, ComputerID: run.ComputerID, AgentID: run.AgentID, Profile: agentprofile.Texture, Role: agentprofile.Texture, ChannelID: run.ChannelID, RunRecord: run}
	_, err := registry.Execute(toolregistry.WithExecutionContext(context.Background(), exec), "spawn_agent", json.RawMessage(`{"role":"researcher","objective":"research exact gap"}`))
	if err == nil || !strings.Contains(err.Error(), "open_researcher") || !strings.Contains(err.Error(), "atomically") {
		t.Fatalf("lifecycle Texture generic spawn refusal = %v", err)
	}
}
