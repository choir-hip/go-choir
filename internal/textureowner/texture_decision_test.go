package textureowner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestRecordTextureDecisionToolPersistsAndEmitsReadableEvent(t *testing.T) {
	ctx := context.Background()
	s, _, registry := textureToolCommitRuntime(t)
	docID := seedTextureDecisionDocument(t, s)
	run := seedTextureDecisionRun(t, s, docID)

	raw, err := registry.Execute(toolregistry.WithExecutionContext(ctx, textureToolExecutionContext(run)), "record_texture_decision", json.RawMessage(`{
		"decision_kind":"delegation_skipped",
		"reason":"The owner supplied the source excerpt, so this revision can proceed without researcher.",
		"evidence_refs":["rev-owner-source","source:owner-excerpt"],
		"next_action":"Use patch_texture for the reader-facing revision."
	}`))
	if err != nil {
		t.Fatalf("record_texture_decision: %v", err)
	}
	var resp map[string]any
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "recorded" || resp["doc_id"] != docID || resp["decision_kind"] != "delegation_skipped" {
		t.Fatalf("unexpected response: %+v", resp)
	}

	decisions, err := s.ListTextureDecisionsByDocument(ctx, "user-1", docID, 10)
	if err != nil {
		t.Fatalf("list decisions: %v", err)
	}
	if len(decisions) != 1 {
		t.Fatalf("decisions len = %d, want 1", len(decisions))
	}
	if decisions[0].RunID != run.RunID || decisions[0].ActorID != run.AgentID || decisions[0].TrajectoryID != trajectoryIDForRun(run) {
		t.Fatalf("decision linkage = %+v, run = %+v", decisions[0], run)
	}
	if len(decisions[0].EvidenceRefs) != 2 || decisions[0].EvidenceRefs[1] != "source:owner-excerpt" {
		t.Fatalf("evidence refs = %#v", decisions[0].EvidenceRefs)
	}

	events, err := s.ListEventsByOwner(ctx, "user-1", 20)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	found := false
	for _, ev := range events {
		if ev.Kind != types.EventTextureDecisionRecorded {
			continue
		}
		found = true
		if !strings.Contains(string(ev.Payload), "delegation_skipped") || !strings.Contains(string(ev.Payload), "owner supplied the source excerpt") {
			t.Fatalf("decision event payload not readable: %s", ev.Payload)
		}
	}
	if !found {
		t.Fatal("missing Texture decision event")
	}
}

func TestTextureDiagnosisAndTraceLogsIncludeDecisionRecords(t *testing.T) {
	ctx := context.Background()
	s, h, _ := textureToolCommitRuntime(t)
	docID := seedTextureDecisionDocument(t, s)
	run := seedTextureDecisionRun(t, s, docID)
	decision := types.TextureDecisionRecord{
		DecisionID:   "decision-trace-1",
		OwnerID:      "user-1",
		DocID:        docID,
		RunID:        run.RunID,
		TrajectoryID: trajectoryIDForRun(run),
		ActorID:      run.AgentID,
		DecisionKind: "wait_for_evidence",
		Reason:       "Researcher has not delivered source evidence yet.",
		EvidenceRefs: []string{"run:" + run.RunID},
		NextAction:   "Wait for the addressed worker update.",
		CreatedAt:    run.CreatedAt,
	}
	if err := s.CreateTextureDecision(ctx, decision); err != nil {
		t.Fatalf("create decision: %v", err)
	}
	h.emitTextureDecisionRecordedEvent(ctx, run, decision)

	diagReq := textureRequest(t, http.MethodGet, "/api/texture/documents/"+docID+"/diagnosis?limit=10", nil)
	diagW := httptest.NewRecorder()
	h.HandleTextureDiagnosis(diagW, diagReq)
	if diagW.Code != http.StatusOK {
		t.Fatalf("diagnosis status = %d, body: %s", diagW.Code, diagW.Body.String())
	}
	var diag textureDiagnosisResponse
	if err := json.NewDecoder(diagW.Body).Decode(&diag); err != nil {
		t.Fatalf("decode diagnosis: %v", err)
	}
	if len(diag.Decisions) != 1 || diag.Decisions[0].DecisionID != decision.DecisionID || diag.Decisions[0].Reason != decision.Reason {
		t.Fatalf("diagnosis decisions = %+v", diag.Decisions)
	}
}

func seedTextureDecisionDocument(t *testing.T, s interface {
	CreateDocument(context.Context, types.Document) error
	CreateRevision(context.Context, types.Revision) error
}) string {
	t.Helper()
	docID := "doc-texture-decision"
	now := time.Date(2026, 6, 18, 15, 0, 0, 0, time.UTC)
	doc := types.Document{
		DocID:     docID,
		OwnerID:   "user-1",
		Title:     "Decision doc",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.CreateDocument(context.Background(), doc); err != nil {
		t.Fatalf("create document: %v", err)
	}
	rev := types.Revision{
		RevisionID:       "rev-texture-decision-base",
		DocID:            docID,
		OwnerID:          "user-1",
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      "owner",
		Content:          "# Decision doc\n\nOwner supplied source material.",
		Citations:        json.RawMessage("[]"),
		Metadata:         json.RawMessage("{}"),
		ParentRevisionID: "",
		CreatedAt:        now,
	}
	if err := s.CreateRevision(context.Background(), rev); err != nil {
		t.Fatalf("create revision: %v", err)
	}
	return docID
}

func seedTextureDecisionRun(t *testing.T, s interface {
	CreateRun(context.Context, types.RunRecord) error
}, docID string) *types.RunRecord {
	t.Helper()
	now := time.Date(2026, 6, 18, 15, 1, 0, 0, time.UTC)
	run := &types.RunRecord{
		RunID:        "run-texture-decision",
		AgentID:      agentprofile.Texture + ":" + docID,
		ChannelID:    docID,
		OwnerID:      "user-1",
		SandboxID:    "sandbox-texture-test",
		TrajectoryID: "run-texture-decision",
		State:        types.RunRunning,
		Prompt:       "revise with owner-provided evidence",
		AgentProfile: agentprofile.Texture,
		AgentRole:    agentprofile.Texture,
		Metadata: map[string]any{
			"agent_profile": agentprofile.Texture,
			"agent_role":    agentprofile.Texture,
			"channel_id":    docID,
			"type":          "texture_agent_revision",
			"doc_id":        docID,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.CreateRun(context.Background(), *run); err != nil {
		t.Fatalf("create texture run: %v", err)
	}
	return run
}
