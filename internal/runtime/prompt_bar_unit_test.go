package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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

	req := authenticatedRequest(http.MethodPost, "/api/prompt-bar", `{"text":"Community Wire staging proof request: using product paths only, run the existing Global Wire source-refresh/research/projection/publication flow, create or approve an Article VText, update global-wire/Wire.vtext, then leave evidence ids and verifier proof. Do not use test-only routes."}`, "user-alice")
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
