package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

func TestHandlePromptBarTextureRouteCompletesConductorSynchronously(t *testing.T) {
	rt, handler := testAPISetup(t)

	req := authenticatedRequest(http.MethodPost, "/api/prompt-bar", `{"text":"write one short sentence that says Texture wrapper cleanup works"}`, "user-alice")
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
		t.Fatalf("status decision missing materialized texture route: %+v", status.Decision)
	}
}

func TestHandlePromptBarOperationalProofInitialRunStartsWithTexture(t *testing.T) {
	rt, handler := testAPISetup(t)

	req := authenticatedRequest(http.MethodPost, "/api/prompt-bar", `{"text":"Universal Wire staging proof request: using product paths only, run the existing Universal Wire source-refresh/research/projection/publication flow, create or approve an Article Texture, update universal-wire/Wire.texture, then leave evidence ids and verifier proof. Do not use test-only routes."}`, "user-alice")
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
	if initialRun.AgentProfile != AgentProfileTexture || initialRun.AgentRole != AgentProfileTexture {
		t.Fatalf("initial loop profile = %q/%q, want texture", initialRun.AgentProfile, initialRun.AgentRole)
	}
	if initialRun.ChannelID != decision.DocID {
		t.Fatalf("initial texture channel = %q, want texture doc %q", initialRun.ChannelID, decision.DocID)
	}
	if got := metadataStringValue(conductor.Metadata, "initial_handoff"); got != "" {
		t.Fatalf("initial_handoff = %q, want no conductor-level super handoff", got)
	}
	runs, err := rt.Store().ListRunsByOwner(context.Background(), "user-alice", 100)
	if err != nil {
		t.Fatalf("list runs before texture super request: %v", err)
	}
	for _, run := range runs {
		if trajectoryIDForRun(&run) == resp.SubmissionID && run.AgentProfile == AgentProfileSuper {
			t.Fatalf("super run appeared before Texture request on prompt-bar trajectory: %+v", run)
		}
	}

	requestCtx := toolregistry.WithExecutionContext(context.Background(), toolExecutionContextForRun(initialRun))
	superResult, err := rt.requestPersistentSuperExecution(requestCtx, "user-alice", decision.DocID, initialRun.RunID, initialRun.AgentID, "Run the Universal Wire verification steps and report evidence back to Texture.", "")
	if err != nil {
		t.Fatalf("texture request super execution: %v", err)
	}
	if got := superResult["profile"]; got != AgentProfileSuper {
		t.Fatalf("super profile = %v, want %s: %+v", got, AgentProfileSuper, superResult)
	}
	if got := superResult["requested_by"]; got != initialRun.AgentID {
		t.Fatalf("requested_by = %v, want %s", got, initialRun.AgentID)
	}
}

func TestHandlePromptBarExplicitNoWorkerDecisionStartsWithTexture(t *testing.T) {
	rt, handler := testAPISetup(t)

	prompt := "Create a short Texture document titled M32_TEXTURE_DECISION_ROUTE_TEST. The body should say this marker is a deployed acceptance probe. Keep the document reader-facing only. Because this task is fully supplied and requires no research or execution worker, record an off-document Texture decision note with decision_kind no_worker_needed, exact reason M3.2 staging proof: user supplied the needed content and requested no research or execution worker., evidence ref staging-marker:M32_TEXTURE_DECISION_ROUTE_TEST, next action Write the concise reader-facing Texture revision. Then write the concise reader-facing Texture revision."
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
	if initialRun.AgentProfile != AgentProfileTexture || initialRun.AgentRole != AgentProfileTexture {
		t.Fatalf("initial loop profile = %q/%q, want texture", initialRun.AgentProfile, initialRun.AgentRole)
	}
	if got := metadataStringValue(conductor.Metadata, "initial_handoff"); got == "persistent_super" {
		t.Fatalf("initial_handoff = %q, want no initial super handoff", got)
	}
	if !metadataBoolValue(initialRun.Metadata, "texture_initial_decision_required") {
		t.Fatalf("initial run missing deterministic decision metadata: %+v", initialRun.Metadata)
	}
	if got := initialTextureToolChoice(initialRun); got != "" {
		t.Fatalf("initial tool choice = %q, want unconstrained first Texture turn", got)
	}
	done := waitForPromptBarUnitRunTerminal(t, rt, decision.InitialLoopID, "user-alice", 5*time.Second)
	if done.State != types.RunCompleted {
		t.Fatalf("initial texture state = %q, want completed", done.State)
	}
	decisions, err := rt.Store().ListTextureDecisionsByDocument(context.Background(), "user-alice", decision.DocID, 10)
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
	seedRev, err := rt.Store().GetRevision(context.Background(), decision.UserRevisionID, "user-alice")
	if err != nil {
		t.Fatalf("get seed revision: %v", err)
	}
	// The prompt-bar V0 is the exact owner prompt as canonical Texture content,
	// not a blank intake revision or a separate prompt band. The owner literally
	// typed the decision rationale into the prompt, so V0 contains it verbatim.
	if seedRev.Content != prompt {
		t.Fatalf("prompt-bar V0 content = %q, want exact owner prompt as canonical V0", seedRev.Content)
	}
	if metadataBoolValue(decodeRevisionMetadata(seedRev.Metadata), "prompt_bar_instruction_revision") {
		t.Fatalf("prompt-bar V0 must not carry the deleted prompt_bar_instruction_revision marker")
	}
	doc, err := rt.Store().GetDocument(context.Background(), decision.DocID, "user-alice")
	if err != nil {
		t.Fatalf("get document: %v", err)
	}
	if doc.CurrentRevisionID != seedRev.RevisionID {
		t.Fatalf("document head = %q, want V0 user revision %q before any appagent write under the stub provider", doc.CurrentRevisionID, seedRev.RevisionID)
	}
}

func TestHandlePromptBarExplicitSuperExecutionStartsWithTextureWithoutAutomaticSuper(t *testing.T) {
	rt, handler := testAPISetup(t)

	prompt := "Create a Texture document for M32_CONTROL_PLANE_EXEC_TEST. Include this execution-shaped sentence verbatim: the document should ask downstream super execution to create a tiny file artifacts/m32_control_plane_exec_test.txt containing the marker, then report the requested execution handle. Do not request super execution in this proof; only write the note after Texture owns the artifact context."
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
	initialRun, err := rt.GetRun(context.Background(), decision.InitialLoopID, "user-alice")
	if err != nil {
		t.Fatalf("get initial loop run: %v", err)
	}
	if initialRun.AgentProfile != AgentProfileTexture || initialRun.AgentRole != AgentProfileTexture {
		t.Fatalf("initial loop profile = %q/%q, want texture", initialRun.AgentProfile, initialRun.AgentRole)
	}
	if metadataBoolValue(initialRun.Metadata, "texture_initial_super_request_required") {
		t.Fatalf("initial run carried deleted Texture super request metadata: %+v", initialRun.Metadata)
	}
	if got := metadataStringValue(initialRun.Metadata, "texture_initial_super_request_objective"); got != "" {
		t.Fatalf("initial run carried deleted Texture super request objective = %q", got)
	}
	if got := metadataStringValue(conductor.Metadata, "initial_handoff"); got == "persistent_super" {
		t.Fatalf("conductor opened a super handoff before Texture acted: %+v", conductor.Metadata)
	}

	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		runs, err := rt.Store().ListRunsByOwner(context.Background(), "user-alice", 100)
		if err != nil {
			t.Fatalf("list runs: %v", err)
		}
		for _, run := range runs {
			if trajectoryIDForRun(&run) == resp.SubmissionID && run.AgentProfile == AgentProfileSuper {
				t.Fatalf("automatic super run appeared before Texture requested one: %+v", run)
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	decisions, err := rt.Store().ListTextureDecisionsByDocument(context.Background(), "user-alice", decision.DocID, 10)
	if err != nil {
		t.Fatalf("list decisions: %v", err)
	}
	for _, decision := range decisions {
		if decision.RunID == initialRun.RunID && decision.DecisionKind == "delegation_opened" {
			t.Fatalf("automatic delegation decision recorded before Texture requested super: %+v", decisions)
		}
	}

	requestCtx := toolregistry.WithExecutionContext(context.Background(), toolExecutionContextForRun(initialRun))
	superResult, err := rt.requestPersistentSuperExecution(requestCtx, "user-alice", decision.DocID, initialRun.RunID, initialRun.AgentID, "Create artifacts/m32_control_plane_exec_test.txt and report evidence back to Texture.", "")
	if err != nil {
		t.Fatalf("texture request super execution: %v", err)
	}
	if got := superResult["profile"]; got != AgentProfileSuper {
		t.Fatalf("super profile = %v, want %s: %+v", got, AgentProfileSuper, superResult)
	}
	if got := superResult["requested_by"]; got != initialRun.AgentID {
		t.Fatalf("requested_by = %v, want %s", got, initialRun.AgentID)
	}
	if got := superResult["requested_by_run_id"]; got != initialRun.RunID {
		t.Fatalf("requested_by_run_id = %v, want %s", got, initialRun.RunID)
	}
}

func TestConductorTextureRouteRecordsExplicitDecisionFromStoredPrompt(t *testing.T) {
	rt, _ := testAPISetup(t)

	prompt := "Create a short Texture document titled M32_TEXTURE_ROUTE_DIAGNOSTIC. The body should say this marker is a deployed route diagnostic. Keep the document reader-facing only. Because this task is fully supplied and requires no research or execution worker, record an off-document Texture decision note with decision_kind no_worker_needed, exact reason M3.2 staging proof: user supplied the needed content and requested no research or execution worker., evidence ref staging-marker:M32_TEXTURE_ROUTE_DIAGNOSTIC, next action Write the concise reader-facing Texture revision. Then write the concise reader-facing Texture revision."
	rec, err := rt.completePromptBarDecisionRun(context.Background(), prompt, "user-alice", map[string]any{
		runMetadataAgentProfile:  AgentProfileConductor,
		runMetadataAgentRole:     AgentProfileConductor,
		"input_source":           "prompt_bar",
		"requested_app":          AgentProfileTexture,
		"initial_document_title": "M32_TEXTURE_ROUTE_DIAGNOSTIC",
		"submission_surface":     "prompt_bar",
	}, conductorDecision{
		Action: "open_app",
		App:    AgentProfileTexture,
		Title:  "M32_TEXTURE_ROUTE_DIAGNOSTIC",
	})
	if err != nil {
		t.Fatalf("complete conductor decision: %v", err)
	}

	decision, err := rt.ensureConductorTextureRoute(context.Background(), rec, "", "")
	if err != nil {
		t.Fatalf("ensure conductor texture route: %v", err)
	}
	if got := metadataStringValue(rec.Metadata, "initial_handoff"); got == "persistent_super" {
		t.Fatalf("initial_handoff = %q, want no initial super handoff", got)
	}
	initialRun, err := rt.GetRun(context.Background(), decision.InitialLoopID, "user-alice")
	if err != nil {
		t.Fatalf("get initial loop run: %v", err)
	}
	if initialRun.AgentProfile != AgentProfileTexture || initialRun.AgentRole != AgentProfileTexture {
		t.Fatalf("initial loop profile = %q/%q, want texture", initialRun.AgentProfile, initialRun.AgentRole)
	}
	if !metadataBoolValue(initialRun.Metadata, "texture_initial_decision_required") {
		t.Fatalf("initial run missing deterministic decision metadata: %+v", initialRun.Metadata)
	}
	done := waitForPromptBarUnitRunTerminal(t, rt, decision.InitialLoopID, "user-alice", 5*time.Second)
	if done.State != types.RunCompleted {
		t.Fatalf("initial texture state = %q, want completed", done.State)
	}
	decisions, err := rt.Store().ListTextureDecisionsByDocument(context.Background(), "user-alice", decision.DocID, 10)
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

	req := authenticatedRequest(http.MethodPost, "/api/prompt-bar", `{"text":"Create a texture document for M3. Ask researcher for a concise finding. Ask super to create a tiny verification note."}`, "user-alice")
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
	if initialRun.AgentProfile != AgentProfileTexture || initialRun.AgentRole != AgentProfileTexture {
		t.Fatalf("initial loop profile = %q/%q, want ordinary texture route", initialRun.AgentProfile, initialRun.AgentRole)
	}
	if got := metadataStringValue(conductor.Metadata, "initial_handoff"); got == "persistent_super" {
		t.Fatalf("initial_handoff = %q, want no researcher-driven route override", got)
	}
}
