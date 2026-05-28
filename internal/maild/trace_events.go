package maild

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
)

const (
	emailTraceEventApprovalRecorded = "email.draft.approval_recorded"
	emailTraceEventSent             = "email.draft.sent"
)

type emailDraftSourceRef struct {
	EmailAppagentRunID string `json:"email_appagent_run_id"`
}

func (h *Handler) emitApprovedDraftTraceEvents(ctx context.Context, draft EmailDraft, approvalEventID, approvalEventType, approvalProviderMessageID, sentMessageID, providerMessageID string) {
	runID := emailAppagentRunIDFromSourceRef(draft.SourceRef)
	if strings.TrimSpace(h.cfg.RuntimeURL) == "" || runID == "" {
		return
	}
	base := map[string]any{
		"authority":                    "email_appagent",
		"maild_role":                   "transport_evidence",
		"draft_id":                     draft.ID,
		"draft_version":                draft.Version,
		"draft_version_hash":           draft.VersionHash,
		"approval_event_id":            approvalEventID,
		"approval_event_type":          approvalEventType,
		"approval_provider_message_id": approvalProviderMessageID,
	}
	if err := h.postRuntimeTraceEvent(ctx, draft.OwnerID, runID, emailTraceEventApprovalRecorded, base); err != nil {
		log.Printf("maild: append email approval trace event draft=%s run=%s: %v", draft.ID, runID, err)
	}
	sent := map[string]any{
		"authority":                    "email_appagent",
		"maild_role":                   "transport_evidence",
		"draft_id":                     draft.ID,
		"draft_version":                draft.Version,
		"draft_version_hash":           draft.VersionHash,
		"approval_event_id":            approvalEventID,
		"approval_event_type":          approvalEventType,
		"approval_provider_message_id": approvalProviderMessageID,
		"sent_message_id":              sentMessageID,
		"provider_message_id":          providerMessageID,
		"send_authorized":              true,
	}
	if err := h.postRuntimeTraceEvent(ctx, draft.OwnerID, runID, emailTraceEventSent, sent); err != nil {
		log.Printf("maild: append email sent trace event draft=%s run=%s: %v", draft.ID, runID, err)
	}
}

func emailAppagentRunIDFromSourceRef(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.HasPrefix(raw, "{") {
		return ""
	}
	var ref emailDraftSourceRef
	if err := json.Unmarshal([]byte(raw), &ref); err != nil {
		return ""
	}
	return strings.TrimSpace(ref.EmailAppagentRunID)
}

func (h *Handler) postRuntimeTraceEvent(ctx context.Context, ownerID, runID, kind string, payload map[string]any) error {
	if payload == nil {
		payload = map[string]any{}
	}
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return fmt.Errorf("owner_id is required")
	}
	endpoint := strings.TrimRight(strings.TrimSpace(h.cfg.RuntimeURL), "/") +
		"/internal/runtime/runs/" + url.PathEscape(runID) +
		"/events?owner_id=" + url.QueryEscape(ownerID)
	body, err := json.Marshal(map[string]any{
		"owner_id": ownerID,
		"kind":     kind,
		"phase":    "email_appagent_evidence",
		"payload":  payload,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("runtime trace event status %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	return nil
}
