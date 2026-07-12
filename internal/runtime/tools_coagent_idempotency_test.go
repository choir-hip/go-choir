package runtime

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

func TestReconcilerTextureHandoffIsIdempotentPerParentAndDocument(t *testing.T) {
	rt, s := testRuntime(t)
	if err := rt.InstallDefaultAgentTools(t.TempDir()); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}

	ctx := context.Background()
	now := time.Now().UTC()
	doc := types.Document{
		DocID:     "doc-reconciler-idempotent",
		OwnerID:   "user-alice",
		Title:     "Existing canonical article",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("create document: %v", err)
	}
	parent := &types.RunRecord{
		RunID:        "reconciler-idempotent-parent",
		AgentID:      "reconciler:story-corpus",
		ChannelID:    "reconciler:story-corpus",
		AgentProfile: AgentProfileReconciler,
		AgentRole:    AgentProfileReconciler,
		OwnerID:      doc.OwnerID,
		SandboxID:    "sandbox-test",
		State:        types.RunRunning,
		Prompt:       "Reconcile the story corpus.",
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile:          AgentProfileReconciler,
			runMetadataAgentRole:             AgentProfileReconciler,
			runMetadataReconcilerScope:       "story-corpus",
			"ingestion_handoff_cycle_id":     "cycle-idempotent",
			"ingestion_handoff_request_id":   "reconciler_publish_idempotent",
			"ingestion_handoff_request_kind": "reconciler",
			"required_texture_revisions":     1,
		},
	}
	if err := s.CreateRun(ctx, *parent); err != nil {
		t.Fatalf("create reconciler run: %v", err)
	}

	registry := rt.ToolRegistryForProfile(AgentProfileReconciler)
	args := json.RawMessage(`{
		"objective":"Revise the existing canonical article from this reconciliation.",
		"role":"texture",
		"channel_id":"doc-reconciler-idempotent"
	}`)
	firstRaw, err := registry.Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(parent)), "spawn_agent", args)
	if err != nil {
		t.Fatalf("first reconciler Texture handoff: %v", err)
	}
	secondRaw, err := registry.Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(parent)), "spawn_agent", args)
	if err != nil {
		t.Fatalf("second reconciler Texture handoff: %v", err)
	}
	var first, second struct {
		LoopID string `json:"loop_id"`
	}
	if err := json.Unmarshal([]byte(firstRaw), &first); err != nil {
		t.Fatalf("decode first handoff: %v", err)
	}
	if err := json.Unmarshal([]byte(secondRaw), &second); err != nil {
		t.Fatalf("decode second handoff: %v", err)
	}
	if second.LoopID != first.LoopID {
		t.Fatalf("second loop = %q, want idempotent first loop %q", second.LoopID, first.LoopID)
	}
	child, err := s.GetRun(ctx, first.LoopID)
	if err != nil {
		t.Fatalf("get Texture child: %v", err)
	}
	if child.RequestedByRunID != parent.RunID {
		t.Fatalf("Texture child requester = %q, want reconciler %q", child.RequestedByRunID, parent.RunID)
	}
	if err := rt.verifyRequiredTextureRevisions(ctx, parent); err == nil {
		t.Fatal("missing reconciler-owned canonical revision should fail completion verification")
	}
	revisionMetadata, err := json.Marshal(map[string]any{
		"loop_id":       child.RunID,
		"revision_role": textureRevisionRoleCanonical,
		"input_origin":  textureInputOriginReconcilerHandoff,
	})
	if err != nil {
		t.Fatalf("marshal revision metadata: %v", err)
	}
	bodyDoc, sourceEntities, projectedContent, err := markdownLineageStructuredRevision(
		doc.DocID,
		"revision-reconciler-idempotent",
		"# Reconciler-owned canonical update",
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("build structured revision: %v", err)
	}
	if err := s.CreateRevision(ctx, types.Revision{
		RevisionID:     "revision-reconciler-idempotent",
		DocID:          doc.DocID,
		OwnerID:        doc.OwnerID,
		AuthorKind:     types.AuthorAppAgent,
		AuthorLabel:    child.AgentID,
		Content:        projectedContent,
		BodyDoc:        bodyDoc,
		SourceEntities: sourceEntities,
		Citations:      json.RawMessage("[]"),
		Metadata:       revisionMetadata,
		CreatedAt:      time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create reconciler-owned revision: %v", err)
	}
	if err := rt.verifyRequiredTextureRevisions(ctx, parent); err != nil {
		t.Fatalf("verify reconciler-owned canonical revision: %v", err)
	}
}
