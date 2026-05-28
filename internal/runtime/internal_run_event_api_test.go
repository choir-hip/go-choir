package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestInternalRunEventAppendAcceptsOnlyEmailEvidenceEvents(t *testing.T) {
	rt, handler := testAPISetup(t)
	now := time.Now().UTC()
	run := types.RunRecord{
		RunID:        "email-run-1",
		AgentID:      "email-appagent-1",
		ChannelID:    "email-channel-1",
		AgentProfile: "email_appagent",
		AgentRole:    "email_appagent",
		OwnerID:      "user-root",
		SandboxID:    "sandbox-test",
		State:        types.RunCompleted,
		Prompt:       "draft email evidence",
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataTrajectoryID: "trajectory-email-1",
		},
	}
	if err := rt.Store().CreateRun(context.Background(), run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/internal/runtime/runs/email-run-1/events?owner_id=user-root", strings.NewReader(`{
		"owner_id":"user-root",
		"kind":"email.draft.sent",
		"phase":"email_appagent_evidence",
		"payload":{
			"authority":"email_appagent",
			"maild_role":"transport_evidence",
			"draft_id":"email-draft-1",
			"send_authorized":true
		}
	}`))
	req.Header.Set("X-Internal-Caller", "true")
	w := httptest.NewRecorder()
	handler.HandleInternalRunEvents(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("append status = %d, want %d; body=%s", w.Code, http.StatusAccepted, w.Body.String())
	}
	var resp internalRunEventAppendResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode append response: %v", err)
	}
	if resp.Status != "appended" || resp.EventID == "" || resp.Kind != types.EventEmailDraftSent {
		t.Fatalf("append response = %+v", resp)
	}
	events, err := rt.Store().ListEvents(context.Background(), "email-run-1", 10)
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1: %+v", len(events), events)
	}
	if events[0].RunID != run.RunID || events[0].AgentID != run.AgentID ||
		events[0].OwnerID != run.OwnerID || events[0].TrajectoryID != "trajectory-email-1" ||
		events[0].Kind != types.EventEmailDraftSent || events[0].Phase != "email_appagent_evidence" {
		t.Fatalf("event = %+v", events[0])
	}
	var payload map[string]any
	if err := json.Unmarshal(events[0].Payload, &payload); err != nil {
		t.Fatalf("decode event payload: %v", err)
	}
	if _, ok := payload["phase"]; ok {
		t.Fatalf("payload should not duplicate event phase: %+v", payload)
	}

	req = httptest.NewRequest(http.MethodPost, "/internal/runtime/runs/email-run-1/events?owner_id=user-root", strings.NewReader(`{
		"owner_id":"user-root",
		"kind":"runtime.tool_call",
		"payload":{"attempt":"unsupported"}
	}`))
	req.Header.Set("X-Internal-Caller", "true")
	w = httptest.NewRecorder()
	handler.HandleInternalRunEvents(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unsupported append status = %d, want %d; body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}
