package agentcore

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestRequiredTextureRevisionRejectsCompletedChildWithoutCanonicalWrite(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	now := time.Now().UTC()

	doc := types.Document{
		DocID:     "doc-required-texture-revision",
		OwnerID:   "user-alice",
		Title:     "Existing canonical article",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("create document: %v", err)
	}
	parent := &types.RunRecord{
		RunID:        "reconciler-required-texture-parent",
		AgentID:      "reconciler:story-corpus",
		ChannelID:    "reconciler:story-corpus",
		AgentProfile: agentprofile.Reconciler,
		AgentRole:    agentprofile.Reconciler,
		OwnerID:      doc.OwnerID,
		SandboxID:    "sandbox-test",
		State:        types.RunRunning,
		Prompt:       "Reconcile the story corpus.",
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile:      agentprofile.Reconciler,
			runMetadataAgentRole:         agentprofile.Reconciler,
			runMetadataReconcilerScope:   "story-corpus",
			"required_texture_revisions": 1,
		},
	}
	if err := s.CreateRun(ctx, *parent); err != nil {
		t.Fatalf("create reconciler run: %v", err)
	}
	child := types.RunRecord{
		RunID:            "texture-required-revision-child",
		AgentID:          "texture:" + doc.DocID,
		ChannelID:        doc.DocID,
		AgentProfile:     agentprofile.Texture,
		AgentRole:        agentprofile.Texture,
		OwnerID:          doc.OwnerID,
		SandboxID:        "sandbox-test",
		RequestedByRunID: parent.RunID,
		State:            types.RunCompleted,
		Prompt:           "Write the reconciler revision.",
		CreatedAt:        now,
		UpdatedAt:        now,
		FinishedAt:       &now,
		Metadata: map[string]any{
			runMetadataAgentProfile: agentprofile.Texture,
			runMetadataAgentRole:    agentprofile.Texture,
			"doc_id":                doc.DocID,
			"request_intent":        "universal_wire_reconciler_article_revision",
		},
	}
	if err := s.CreateRun(ctx, child); err != nil {
		t.Fatalf("create completed Texture child: %v", err)
	}

	const missingWriteError = "required reconciler Texture revisions missing: wrote 0, require 1"
	if err := rt.verifyRequiredTextureRevisions(ctx, parent); err == nil || err.Error() != missingWriteError {
		t.Fatalf("verification error = %v, want %q", err, missingWriteError)
	}
	const terminatedWithoutWriteError = "required reconciler Texture revisions missing: all 1 Texture handoff(s) terminated without the required canonical write"
	if err := rt.awaitRequiredChildRuns(ctx, parent, time.Millisecond); err == nil || err.Error() != terminatedWithoutWriteError {
		t.Fatalf("completion gate error = %v, want %q", err, terminatedWithoutWriteError)
	}

	decoys := []struct {
		revisionID string
		metadata   map[string]any
	}{
		{
			revisionID: "revision-wrong-loop",
			metadata: map[string]any{
				"loop_id":       "other-texture-loop",
				"revision_role": textureRevisionRoleCanonical,
				"input_origin":  textureInputOriginReconcilerHandoff,
			},
		},
		{
			revisionID: "revision-wrong-role",
			metadata: map[string]any{
				"loop_id":       child.RunID,
				"revision_role": textureRevisionRoleInput,
				"input_origin":  textureInputOriginReconcilerHandoff,
			},
		},
		{
			revisionID: "revision-wrong-origin",
			metadata: map[string]any{
				"loop_id":       child.RunID,
				"revision_role": textureRevisionRoleCanonical,
				"input_origin":  textureInputOriginProcessorHandoff,
			},
		},
	}
	headRevisionID := ""
	for i, decoy := range decoys {
		metadata, err := json.Marshal(decoy.metadata)
		if err != nil {
			t.Fatalf("marshal decoy revision metadata: %v", err)
		}
		content := "# Non-matching canonical evidence"
		if err := s.CreateRevision(ctx, types.Revision{
			RevisionID:       decoy.revisionID,
			DocID:            doc.DocID,
			OwnerID:          doc.OwnerID,
			AuthorKind:       types.AuthorAppAgent,
			AuthorLabel:      child.AgentID,
			Content:          content,
			BodyDoc:          runtimeTestTextureBodyDoc(t, doc.DocID, decoy.revisionID, content),
			Citations:        json.RawMessage("[]"),
			Metadata:         metadata,
			ParentRevisionID: headRevisionID,
			CreatedAt:        now.Add(time.Duration(i+1) * time.Second),
		}); err != nil {
			t.Fatalf("create decoy revision %s: %v", decoy.revisionID, err)
		}
		headRevisionID = decoy.revisionID
	}
	if err := rt.verifyRequiredTextureRevisions(ctx, parent); err == nil || err.Error() != missingWriteError {
		t.Fatalf("verification with non-matching metadata error = %v, want %q", err, missingWriteError)
	}

	revisionMetadata, err := json.Marshal(map[string]any{
		"loop_id":       child.RunID,
		"revision_role": textureRevisionRoleCanonical,
		"input_origin":  textureInputOriginReconcilerHandoff,
	})
	if err != nil {
		t.Fatalf("marshal revision metadata: %v", err)
	}
	if err := s.CreateRevision(ctx, types.Revision{
		RevisionID:       "revision-required-texture-canonical",
		DocID:            doc.DocID,
		OwnerID:          doc.OwnerID,
		AuthorKind:       types.AuthorAppAgent,
		AuthorLabel:      child.AgentID,
		Content:          "# Reconciler-owned canonical update",
		BodyDoc:          runtimeTestTextureBodyDoc(t, doc.DocID, "revision-required-texture-canonical", "# Reconciler-owned canonical update"),
		Citations:        json.RawMessage("[]"),
		Metadata:         revisionMetadata,
		ParentRevisionID: headRevisionID,
		CreatedAt:        now.Add(time.Duration(len(decoys)+1) * time.Second),
	}); err != nil {
		t.Fatalf("create matching canonical revision: %v", err)
	}
	if err := rt.verifyRequiredTextureRevisions(ctx, parent); err != nil {
		t.Fatalf("verify matching canonical revision: %v", err)
	}
	if err := rt.awaitRequiredChildRuns(ctx, parent, time.Millisecond); err != nil {
		t.Fatalf("completion gate with matching canonical revision: %v", err)
	}
}

func TestRequiredGenericChildCompletionRemainsIndependent(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	now := time.Now().UTC()
	parent := &types.RunRecord{
		RunID:     "generic-required-child-parent",
		AgentID:   "parent-agent",
		OwnerID:   "user-alice",
		SandboxID: "sandbox-test",
		State:     types.RunRunning,
		CreatedAt: now,
		UpdatedAt: now,
		Metadata: map[string]any{
			"required_child_runs": 1,
		},
	}
	if err := s.CreateRun(ctx, *parent); err != nil {
		t.Fatalf("create generic parent: %v", err)
	}
	if err := s.CreateRun(ctx, types.RunRecord{
		RunID:            "generic-required-child",
		AgentID:          "child-agent",
		OwnerID:          parent.OwnerID,
		SandboxID:        "sandbox-test",
		RequestedByRunID: parent.RunID,
		State:            types.RunCompleted,
		CreatedAt:        now,
		UpdatedAt:        now,
		FinishedAt:       &now,
	}); err != nil {
		t.Fatalf("create completed generic child: %v", err)
	}
	if err := rt.awaitRequiredChildRuns(ctx, parent, time.Millisecond); err != nil {
		t.Fatalf("generic child completion gate: %v", err)
	}
}
