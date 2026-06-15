package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestHandlePromptBarVTextRouteCompletesConductorSynchronously(t *testing.T) {
	rt, handler := testAPISetup(t)

	req := authenticatedRequest(http.MethodPost, "/api/prompt-bar", `{"text":"write one short sentence that says VText wrapper cleanup works"}`, "user-alice")
	w := httptest.NewRecorder()
	handler.HandlePromptBar(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusAccepted, w.Body.String())
	}

	var resp promptBarSubmitResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	rec, err := rt.GetRun(context.Background(), resp.SubmissionID, "user-alice")
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if rec.State != types.RunCompleted {
		t.Fatalf("conductor state = %q, want %q", rec.State, types.RunCompleted)
	}

	statusReq := authenticatedRequest(http.MethodGet, "/api/prompt-bar/submissions/"+resp.SubmissionID, "", "user-alice")
	statusW := httptest.NewRecorder()
	handler.HandlePromptBarSubmission(statusW, statusReq)
	if statusW.Code != http.StatusOK {
		t.Fatalf("status endpoint = %d, want 200; body=%s", statusW.Code, statusW.Body.String())
	}
	var status promptBarSubmissionStatusResponse
	if err := json.NewDecoder(statusW.Body).Decode(&status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.State != types.RunCompleted {
		t.Fatalf("submission state = %q, want %q", status.State, types.RunCompleted)
	}
	if status.Decision == nil || status.Decision.DocID == "" || status.Decision.InitialLoopID == "" {
		t.Fatalf("status decision missing materialized vtext route: %+v", status.Decision)
	}
}

func TestHandlePromptBarOperationalProofInitialRunRequestsPersistentSuper(t *testing.T) {
	rt, handler := testAPISetup(t)

	req := authenticatedRequest(http.MethodPost, "/api/prompt-bar", `{"text":"Universal Wire staging proof request: using product paths only, run the existing Universal Wire source-refresh/research/projection/publication flow, create or approve an Article VText, update universal-wire/Wire.vtext, then leave evidence ids and verifier proof. Do not use test-only routes."}`, "user-alice")
	w := httptest.NewRecorder()
	handler.HandlePromptBar(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusAccepted, w.Body.String())
	}

	var resp promptBarSubmitResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	conductor, err := rt.GetRun(context.Background(), resp.SubmissionID, "user-alice")
	if err != nil {
		t.Fatalf("get conductor: %v", err)
	}
	var decision conductorDecision
	if err := json.Unmarshal([]byte(conductor.Result), &decision); err != nil {
		t.Fatalf("decode conductor decision: %v\n%s", err, conductor.Result)
	}
	if decision.InitialLoopID == "" {
		t.Fatalf("conductor decision missing initial loop: %+v", decision)
	}
	initialRun, err := rt.GetRun(context.Background(), decision.InitialLoopID, "user-alice")
	if err != nil {
		t.Fatalf("get initial loop run: %v", err)
	}
	if initialRun.AgentProfile != AgentProfileSuper || initialRun.AgentRole != AgentProfileSuper {
		t.Fatalf("initial loop profile = %q/%q, want super", initialRun.AgentProfile, initialRun.AgentRole)
	}
	if initialRun.ChannelID != decision.DocID {
		t.Fatalf("initial super channel = %q, want vtext doc %q", initialRun.ChannelID, decision.DocID)
	}
	if got := metadataStringValue(conductor.Metadata, "initial_handoff"); got != "persistent_super" {
		t.Fatalf("initial_handoff = %q, want persistent_super", got)
	}
}

func TestHandlePromptBarExplicitNoWorkerDecisionStartsWithVText(t *testing.T) {
	rt, handler := testAPISetup(t)

	prompt := "Create a short VText document titled M32_VTEXT_DECISION_ROUTE_TEST. The body should say this marker is a deployed acceptance probe. Keep the document reader-facing only. Because this task is fully supplied and requires no research or execution worker, record an off-document VText decision note with decision_kind no_worker_needed, exact reason M3.2 staging proof: user supplied the needed content and requested no research or execution worker., evidence ref staging-marker:M32_VTEXT_DECISION_ROUTE_TEST, next action Write the concise reader-facing VText revision. Then write the concise reader-facing VText revision."
	body, err := json.Marshal(map[string]string{"text": prompt})
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	req := authenticatedRequest(http.MethodPost, "/api/prompt-bar", string(body), "user-alice")
	w := httptest.NewRecorder()
	handler.HandlePromptBar(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusAccepted, w.Body.String())
	}

	var resp promptBarSubmitResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	conductor, err := rt.GetRun(context.Background(), resp.SubmissionID, "user-alice")
	if err != nil {
		t.Fatalf("get conductor: %v", err)
	}
	var decision conductorDecision
	if err := json.Unmarshal([]byte(conductor.Result), &decision); err != nil {
		t.Fatalf("decode conductor decision: %v\n%s", err, conductor.Result)
	}
	if decision.InitialLoopID == "" {
		t.Fatalf("conductor decision missing initial loop: %+v", decision)
	}
	initialRun, err := rt.GetRun(context.Background(), decision.InitialLoopID, "user-alice")
	if err != nil {
		t.Fatalf("get initial loop run: %v", err)
	}
	if initialRun.AgentProfile != AgentProfileVText || initialRun.AgentRole != AgentProfileVText {
		t.Fatalf("initial loop profile = %q/%q, want vtext", initialRun.AgentProfile, initialRun.AgentRole)
	}
	if got := metadataStringValue(conductor.Metadata, "initial_handoff"); got == "persistent_super" {
		t.Fatalf("initial_handoff = %q, want no initial super handoff", got)
	}
	if !metadataBoolValue(conductor.Metadata, "prompt_bar_no_worker_decision_route") {
		t.Fatalf("conductor missing prompt-bar no-worker route flag: %+v", conductor.Metadata)
	}
	if !metadataBoolValue(initialRun.Metadata, "vtext_initial_decision_required") {
		t.Fatalf("initial run missing deterministic decision metadata: %+v", initialRun.Metadata)
	}
	if got := initialVTextToolChoice(initialRun); got != exactRequiredToolChoice("edit_vtext") {
		t.Fatalf("initial tool choice = %q, want edit_vtext after deterministic decision record", got)
	}
	done := waitForPromptBarUnitRunTerminal(t, rt, decision.InitialLoopID, "user-alice", 5*time.Second)
	if done.State != types.RunCompleted {
		t.Fatalf("initial vtext state = %q, want completed", done.State)
	}
	decisions, err := rt.Store().ListVTextDecisionsByDocument(context.Background(), "user-alice", decision.DocID, 10)
	if err != nil {
		t.Fatalf("list decisions: %v", err)
	}
	if len(decisions) != 1 {
		t.Fatalf("decision count = %d, want 1: %+v", len(decisions), decisions)
	}
	if decisions[0].RunID != decision.InitialLoopID ||
		decisions[0].DecisionKind != "no_worker_needed" ||
		decisions[0].Reason != "M3.2 staging proof: user supplied the needed content and requested no research or execution worker." {
		t.Fatalf("decision record = %+v", decisions[0])
	}
}

func TestConductorVTextRouteDerivesNoWorkerDecisionFromStoredPrompt(t *testing.T) {
	rt, _ := testAPISetup(t)

	prompt := "Create a short VText document titled M32_VTEXT_ROUTE_DIAGNOSTIC. The body should say this marker is a deployed route diagnostic. Keep the document reader-facing only. Because this task is fully supplied and requires no research or execution worker, record an off-document VText decision note with decision_kind no_worker_needed, exact reason M3.2 staging proof: user supplied the needed content and requested no research or execution worker., evidence ref staging-marker:M32_VTEXT_ROUTE_DIAGNOSTIC, next action Write the concise reader-facing VText revision. Then write the concise reader-facing VText revision."
	rec, err := rt.completePromptBarDecisionRun(context.Background(), prompt, "user-alice", map[string]any{
		runMetadataAgentProfile:  AgentProfileConductor,
		runMetadataAgentRole:     AgentProfileConductor,
		"input_source":           "prompt_bar",
		"requested_app":          AgentProfileVText,
		"initial_document_title": "M32_VTEXT_ROUTE_DIAGNOSTIC",
		"submission_surface":     "prompt_bar",
	}, conductorDecision{
		Action: "open_app",
		App:    AgentProfileVText,
		Title:  "M32_VTEXT_ROUTE_DIAGNOSTIC",
	})
	if err != nil {
		t.Fatalf("complete conductor decision: %v", err)
	}

	decision, err := rt.ensureConductorVTextRoute(context.Background(), rec, "", "")
	if err != nil {
		t.Fatalf("ensure conductor vtext route: %v", err)
	}
	if got := metadataStringValue(rec.Metadata, "initial_handoff"); got == "persistent_super" {
		t.Fatalf("initial_handoff = %q, want no initial super handoff", got)
	}
	if !metadataBoolValue(rec.Metadata, "prompt_bar_no_worker_decision_route") {
		t.Fatalf("route should persist no-worker flag derived from stored prompt: %+v", rec.Metadata)
	}
	initialRun, err := rt.GetRun(context.Background(), decision.InitialLoopID, "user-alice")
	if err != nil {
		t.Fatalf("get initial loop run: %v", err)
	}
	if initialRun.AgentProfile != AgentProfileVText || initialRun.AgentRole != AgentProfileVText {
		t.Fatalf("initial loop profile = %q/%q, want vtext", initialRun.AgentProfile, initialRun.AgentRole)
	}
	if !metadataBoolValue(initialRun.Metadata, "vtext_initial_decision_required") {
		t.Fatalf("initial run missing deterministic decision metadata: %+v", initialRun.Metadata)
	}
}

func waitForPromptBarUnitRunTerminal(t *testing.T, rt *Runtime, runID, ownerID string, timeout time.Duration) types.RunRecord {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		rec, err := rt.GetRun(context.Background(), runID, ownerID)
		if err != nil {
			t.Fatalf("get run %s: %v", runID, err)
		}
		if rec.State.Terminal() {
			return *rec
		}
		time.Sleep(20 * time.Millisecond)
	}
	rec, err := rt.GetRun(context.Background(), runID, ownerID)
	if err != nil {
		t.Fatalf("get run %s after timeout: %v", runID, err)
	}
	t.Fatalf("timeout waiting for run %s (state=%s)", runID, rec.State)
	return types.RunRecord{}
}

func TestHandlePromptBarResearcherMentionDoesNotSetRoutingFlag(t *testing.T) {
	rt, handler := testAPISetup(t)

	req := authenticatedRequest(http.MethodPost, "/api/prompt-bar", `{"text":"Create a vtext document for M3. Ask researcher for a concise finding. Ask super to create a tiny verification note."}`, "user-alice")
	w := httptest.NewRecorder()
	handler.HandlePromptBar(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusAccepted, w.Body.String())
	}

	var resp promptBarSubmitResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	conductor, err := rt.GetRun(context.Background(), resp.SubmissionID, "user-alice")
	if err != nil {
		t.Fatalf("get conductor: %v", err)
	}
	var decision conductorDecision
	if err := json.Unmarshal([]byte(conductor.Result), &decision); err != nil {
		t.Fatalf("decode conductor decision: %v\n%s", err, conductor.Result)
	}
	if metadataBoolValue(conductor.Metadata, runMetadataExplicitResearcher) {
		t.Fatalf("conductor metadata must not set %s from prompt text: %+v", runMetadataExplicitResearcher, conductor.Metadata)
	}
	if decision.InitialLoopID == "" {
		t.Fatalf("conductor decision missing initial loop: %+v", decision)
	}
	initialRun, err := rt.GetRun(context.Background(), decision.InitialLoopID, "user-alice")
	if err != nil {
		t.Fatalf("get initial loop run: %v", err)
	}
	if metadataBoolValue(initialRun.Metadata, runMetadataExplicitResearcher) {
		t.Fatalf("initial run metadata must not set %s from prompt text: %+v", runMetadataExplicitResearcher, initialRun.Metadata)
	}
	if initialRun.AgentProfile != AgentProfileVText || initialRun.AgentRole != AgentProfileVText {
		t.Fatalf("initial loop profile = %q/%q, want ordinary vtext route", initialRun.AgentProfile, initialRun.AgentRole)
	}
	if got := metadataStringValue(conductor.Metadata, "initial_handoff"); got == "persistent_super" {
		t.Fatalf("initial_handoff = %q, want no researcher-driven route override", got)
	}
}
