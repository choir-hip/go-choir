package textureowner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
)


func TestTextureDiagnosisAndTraceLogsIncludeDecisionRecords(t *testing.T) {
	ctx := context.Background()
	_, h := testAPISetup(t)
	s := h.Store
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
		Reason:       "Research has not delivered source evidence yet.",
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
		ComputerID:   "autoputer-texture-test",
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
