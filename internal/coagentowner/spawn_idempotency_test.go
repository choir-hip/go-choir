package coagentowner

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/textureowner"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestReconcilerTextureHandoffIsIdempotentPerParentAndDocument(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(filepath.Join(t.TempDir(), "choir.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	core := agentcore.New(provideriface.Config{SandboxID: "sandbox-test"}, s, events.NewEventBus(), provider.NewStubProvider(0))
	core.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })
	if err := core.InstallDefaultAgentTools(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	texture := textureowner.NewHandler(core)
	registry := core.ToolRegistryForProfile(agentprofile.Reconciler)
	if err := RegisterSpawnTool(registry, core, texture, agentprofile.PolicyFor(agentprofile.Reconciler)); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	doc := types.Document{DocID: "doc-reconciler-idempotent", OwnerID: "user-alice", Title: "Existing canonical article", CreatedAt: now, UpdatedAt: now}
	agentID := "texture:" + doc.DocID
	start := types.StartLifecycleRequest{
		OwnerID: doc.OwnerID, ComputerID: "sandbox-test", CommandID: "start:" + doc.DocID, TrajectoryID: "trajectory:" + doc.DocID,
		Kind:            types.TrajectoryKindDocument,
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		SubjectRefs:     map[string]string{"artifact": "texture://documents/" + doc.DocID},
		InitialWork:     types.WorkItemRecord{WorkItemID: "work:" + doc.DocID, Objective: "Revise the existing canonical article.", AssignedAgentID: agentID},
		InitialDocument: doc,
		InitialRevision: types.Revision{RevisionID: "revision:" + doc.DocID, AuthorKind: types.AuthorUser, AuthorLabel: doc.OwnerID, Content: "Existing canonical article"},
		Agent: types.AgentRecord{
			AgentID: agentID, OwnerID: doc.OwnerID, ComputerID: "sandbox-test", SandboxID: "sandbox-test",
			Profile: agentprofile.Texture, Role: agentprofile.Texture, ChannelID: doc.DocID, CreatedAt: now, UpdatedAt: now,
		},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start durable lifecycle: %v", err)
	}
	parent := &types.RunRecord{
		RunID: "reconciler-idempotent-parent", AgentID: "reconciler:story-corpus", ChannelID: "reconciler:story-corpus",
		AgentProfile: agentprofile.Reconciler, AgentRole: agentprofile.Reconciler, OwnerID: doc.OwnerID,
		SandboxID: "sandbox-test", State: types.RunRunning, Prompt: "Reconcile the story corpus.", CreatedAt: now, UpdatedAt: now,
		Metadata: map[string]any{"agent_profile": agentprofile.Reconciler, "agent_role": agentprofile.Reconciler, "reconciler_scope": "story-corpus"},
	}
	if err := s.CreateRun(ctx, *parent); err != nil {
		t.Fatal(err)
	}
	toolCtx := toolregistry.WithExecutionContext(ctx, toolregistry.ExecutionContext{
		RunID: parent.RunID, AgentID: parent.AgentID, OwnerID: parent.OwnerID, Profile: parent.AgentProfile,
		Role: parent.AgentRole, ChannelID: parent.ChannelID, RunRecord: parent,
	})
	args := json.RawMessage(`{"objective":"Revise the existing canonical article from this reconciliation.","role":"texture","channel_id":"doc-reconciler-idempotent"}`)
	firstRaw, err := registry.Execute(toolCtx, "spawn_agent", args)
	if err != nil {
		t.Fatalf("first handoff: %v", err)
	}
	secondRaw, err := registry.Execute(toolCtx, "spawn_agent", args)
	if err != nil {
		t.Fatalf("second handoff: %v", err)
	}
	var first, second struct {
		LoopID string `json:"loop_id"`
	}
	if err := json.Unmarshal([]byte(firstRaw), &first); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(secondRaw), &second); err != nil {
		t.Fatal(err)
	}
	if first.LoopID == "" || second.LoopID != first.LoopID {
		t.Fatalf("handoff loops first=%q second=%q, want one reused child", first.LoopID, second.LoopID)
	}
	child, err := s.GetLifecycleRun(ctx, doc.OwnerID, "sandbox-test", first.LoopID)
	if err != nil {
		t.Fatal(err)
	}
	if child.RequestedByRunID != parent.RunID {
		t.Fatalf("child requester=%q, want %q", child.RequestedByRunID, parent.RunID)
	}
}

func TestSpawnAgentRejectsInvalidExplicitProfile(t *testing.T) {
	registry := toolregistry.NewToolRegistry()
	if err := RegisterSpawnTool(registry, nil, nil, agentprofile.PolicyFor(agentprofile.Super)); err != nil {
		t.Fatal(err)
	}
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{
		RunID: "parent-run", OwnerID: "user-alice", Profile: agentprofile.Super,
	})

	for _, profile := range []string{"texture", "texture researcher", "research", "research-agent", "Researcher", "coagent"} {
		_, err := registry.Execute(ctx, "spawn_agent", json.RawMessage(`{"objective":"Research the subject.","role":"researcher","profile":"`+profile+`"}`))
		if err == nil {
			t.Fatalf("spawn_agent accepted explicit profile %q outside the caller's allowed targets", profile)
		}
		if got := err.Error(); got != "profile must be one of researcher, co-super" {
			t.Fatalf("spawn_agent profile %q error = %q", profile, got)
		}
	}
}
