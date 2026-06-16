package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestRecordVTextDecisionToolPersistsAndEmitsReadableEvent(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	docID := seedVTextDecisionDocument(t, s)
	run, err := rt.createRunWithMetadata(ctx, "revise with owner-provided evidence", "user-1", map[string]any{
		runMetadataAgentProfile: AgentProfileVText,
		runMetadataAgentRole:    AgentProfileVText,
		runMetadataChannelID:    docID,
		"type":                  "vtext_agent_revision",
		"doc_id":                docID,
	})
	if err != nil {
		t.Fatalf("create vtext run: %v", err)
	}

	registry := NewToolRegistry()
	if err := RegisterVTextTools(registry, rt); err != nil {
		t.Fatalf("register vtext tools: %v", err)
	}
	raw, err := registry.Execute(WithToolExecutionContext(ctx, run), "record_texture_decision", json.RawMessage(`{
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

	decisions, err := s.ListVTextDecisionsByDocument(ctx, "user-1", docID, 10)
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
		if ev.Kind != types.EventVTextDecisionRecorded {
			continue
		}
		found = true
		if !strings.Contains(string(ev.Payload), "delegation_skipped") || !strings.Contains(string(ev.Payload), "owner supplied the source excerpt") {
			t.Fatalf("decision event payload not readable: %s", ev.Payload)
		}
	}
	if !found {
		t.Fatal("missing vtext decision event")
	}
}

func TestVTextDiagnosisAndTraceLogsIncludeDecisionRecords(t *testing.T) {
	ctx := context.Background()
	rt, h := testAPISetup(t)
	s := rt.Store()
	docID := seedVTextDecisionDocument(t, s)
	run, err := rt.createRunWithMetadata(ctx, "revise with owner-provided evidence", "user-1", map[string]any{
		runMetadataAgentProfile: AgentProfileVText,
		runMetadataAgentRole:    AgentProfileVText,
		runMetadataChannelID:    docID,
		"type":                  "vtext_agent_revision",
		"doc_id":                docID,
	})
	if err != nil {
		t.Fatalf("create vtext run: %v", err)
	}
	decision := types.VTextDecisionRecord{
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
	if err := s.CreateVTextDecision(ctx, decision); err != nil {
		t.Fatalf("create decision: %v", err)
	}
	rt.emitVTextDecisionRecordedEvent(ctx, run, decision)

	diagReq := vtextRequest(t, http.MethodGet, "/api/texture/documents/"+docID+"/diagnosis?limit=10", nil)
	diagW := httptest.NewRecorder()
	h.HandleVTextDiagnosis(diagW, diagReq)
	if diagW.Code != http.StatusOK {
		t.Fatalf("diagnosis status = %d, body: %s", diagW.Code, diagW.Body.String())
	}
	var diag vtextDiagnosisResponse
	if err := json.NewDecoder(diagW.Body).Decode(&diag); err != nil {
		t.Fatalf("decode diagnosis: %v", err)
	}
	if len(diag.Decisions) != 1 || diag.Decisions[0].DecisionID != decision.DecisionID || diag.Decisions[0].Reason != decision.Reason {
		t.Fatalf("diagnosis decisions = %+v", diag.Decisions)
	}

	traceReq := authenticatedRequest(http.MethodGet, "/api/trace/trajectories/"+trajectoryIDForRun(run)+"/logs", "", "user-1")
	traceW := httptest.NewRecorder()
	h.HandleTraceTrajectories(traceW, traceReq)
	if traceW.Code != http.StatusOK {
		t.Fatalf("trace logs status = %d, body: %s", traceW.Code, traceW.Body.String())
	}
	body := traceW.Body.String()
	if !strings.Contains(body, "vtext decision wait_for_evidence: Researcher has not delivered source evidence yet.") ||
		!strings.Contains(body, `"decision_id":"decision-trace-1"`) {
		t.Fatalf("trace logs missing readable decision: %s", body)
	}
}

func seedVTextDecisionDocument(t *testing.T, s interface {
	CreateDocument(context.Context, types.Document) error
	CreateRevision(context.Context, types.Revision) error
}) string {
	t.Helper()
	docID := "doc-vtext-decision"
	now := testNow()
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
		RevisionID:       "rev-vtext-decision-base",
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
