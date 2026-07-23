package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
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

func TestTextureRunProjectsLifecycleEvents(t *testing.T) {
	rt, _ := testAPISetup(t)
	trajectoryID := seedDurableTextureSubject(t, rt.Store(), "user-texture-events", "doc-event-projection")
	rec, err := rt.StartRunWithMetadata(context.Background(), "revise the document", "user-texture-events", map[string]any{
		runMetadataAgentProfile: agentprofile.Texture,
		runMetadataAgentRole:    agentprofile.Texture,
		runMetadataAgentID:      "texture:doc-event-projection",
		runMetadataChannelID:    "doc-event-projection",
		"doc_id":                "doc-event-projection",
		"trajectory_id":         trajectoryID,
	})
	if err != nil {
		t.Fatalf("start Texture run: %v", err)
	}
	progress := waitForTextureLifecycleEvent(t, rt, "user-texture-events", rec.RunID, types.EventTextureAgentRevisionProgress)
	var progressPayload map[string]string
	if err := json.Unmarshal(progress.Payload, &progressPayload); err != nil {
		t.Fatalf("decode Texture progress: %v", err)
	}
	if progressPayload["doc_id"] != "doc-event-projection" ||
		progressPayload["loop_id"] != rec.RunID ||
		progressPayload["phase"] == "" {
		t.Fatalf("Texture progress payload = %#v", progressPayload)
	}

	now := time.Now().UTC()
	failed := types.RunRecord{
		RunID:     "run-texture-failure-projection",
		OwnerID:   "user-texture-events",
		AgentID:   "texture:doc-failure-projection",
		ChannelID: "doc-failure-projection",
		State:     types.RunRunning,
		Prompt:    "fail the Texture revision",
		CreatedAt: now,
		UpdatedAt: now,
		Metadata: map[string]any{
			runMetadataAgentProfile: agentprofile.Texture,
			runMetadataAgentRole:    agentprofile.Texture,
			runMetadataAgentID:      "texture:doc-failure-projection",
			runMetadataChannelID:    "doc-failure-projection",
			"doc_id":                "doc-failure-projection",
		},
	}
	if err := rt.Store().CreateRun(context.Background(), failed); err != nil {
		t.Fatalf("create failing Texture run: %v", err)
	}
	rt.handleExecutionError(context.Background(), &failed, errors.New("provider failed"))
	failure := waitForTextureLifecycleEvent(t, rt, "user-texture-events", failed.RunID, types.EventTextureAgentRevisionFailed)
	var failurePayload map[string]string
	if err := json.Unmarshal(failure.Payload, &failurePayload); err != nil {
		t.Fatalf("decode Texture failure: %v", err)
	}
	if failurePayload["doc_id"] != "doc-failure-projection" ||
		failurePayload["loop_id"] != failed.RunID ||
		failurePayload["error"] != "provider failed" {
		t.Fatalf("Texture failure payload = %#v", failurePayload)
	}
}

func waitForTextureLifecycleEvent(t *testing.T, rt *Runtime, ownerID, runID string, kind types.EventKind) types.EventRecord {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		records, err := rt.Store().ListEvents(context.Background(), runID, 100)
		if err != nil {
			t.Fatalf("list Texture lifecycle events: %v", err)
		}
		for _, record := range records {
			if record.Kind == kind {
				return record
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	run, runErr := rt.GetRun(context.Background(), runID, ownerID)
	t.Fatalf("timed out waiting for Texture lifecycle event %q; run=%+v err=%v", kind, run, runErr)
	return types.EventRecord{}
}
