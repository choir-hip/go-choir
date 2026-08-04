package textureowner

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

func TestRecordTextureDecisionToolAppendsCanonicalOwnerMessage(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	docID := "doc-texture-decision"
	run := startDurableTextureParent(t, rt, "user-alice", docID, "Revise with owner-provided evidence.", nil)
	registry := rt.ToolRegistryForProfile(agentprofile.Texture)

	args := json.RawMessage(`{
		"decision_kind":"delegation_skipped",
		"reason":"The owner supplied the source excerpt, so this revision can proceed without researcher.",
		"evidence_refs":["rev-owner-source","source:owner-excerpt"],
		"next_action":"Use patch_texture for the reader-facing revision."
	}`)
	raw, err := registry.Execute(toolregistry.WithExecutionContext(ctx, textureToolExecutionContext(run)), "record_texture_decision", args)
	if err != nil {
		t.Fatalf("record_texture_decision: %v", err)
	}
	replayed, err := registry.Execute(toolregistry.WithExecutionContext(ctx, textureToolExecutionContext(run)), "record_texture_decision", args)
	if err != nil || replayed != raw {
		t.Fatalf("replay changed canonical decision: raw=%s replay=%s err=%v", raw, replayed, err)
	}
	var resp map[string]any
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "recorded" || resp["doc_id"] != docID || resp["decision_kind"] != "delegation_skipped" {
		t.Fatalf("unexpected response: %+v", resp)
	}

	snapshot, err := s.GetSupervisionProjectionSnapshot(ctx, run.OwnerID, run.SandboxID, trajectoryIDForRun(run))
	if err != nil {
		t.Fatalf("load canonical supervision projection: %v", err)
	}
	if len(snapshot.Control.Messages) != 1 || snapshot.Control.Messages[0].ID != resp["decision_id"] ||
		len(snapshot.Control.Messages[0].ArtifactRefs) != 1 || snapshot.Control.AttentionCount != 1 {
		t.Fatalf("canonical decision message = %+v", snapshot.Control)
	}
}
