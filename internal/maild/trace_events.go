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
	"time"
)

const (
	emailTraceEventApprovalRecorded = "email.draft.approval_recorded"
	emailTraceEventSent             = "email.draft.sent"
	primaryDesktopID                = "primary"
)

type emailDraftSourceRef struct {
	EmailAppagentRunID string `json:"email_appagent_run_id"`
}

type vmctlResolveRequest struct {
	UserID    string `json:"user_id"`
	DesktopID string `json:"desktop_id,omitempty"`
}

type vmctlResolveResponse struct {
	SandboxURL string `json:"sandbox_url"`
}

func (h *Handler) emitApprovedDraftTraceEvents(ctx context.Context, draft EmailDraft, approvalEventID, approvalEventType, approvalProviderMessageID, sentMessageID, providerMessageID string) {
	runID := emailAppagentRunIDFromSourceRef(draft.SourceRef)
	if runID == "" {
		return
	}
	runtimeURL, err := h.resolveTraceRuntimeURL(ctx, draft.OwnerID)
	if err != nil {
		log.Printf("maild: resolve email trace runtime draft=%s run=%s owner=%s: %v", draft.ID, runID, draft.OwnerID, err)
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
	if err := h.postRuntimeTraceEvent(ctx, runtimeURL, draft.OwnerID, runID, emailTraceEventApprovalRecorded, base); err != nil {
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
	if err := h.postRuntimeTraceEvent(ctx, runtimeURL, draft.OwnerID, runID, emailTraceEventSent, sent); err != nil {
		log.Printf("maild: append email sent trace event draft=%s run=%s: %v", draft.ID, runID, err)
	}
}

func (h *Handler) resolveTraceRuntimeURL(ctx context.Context, ownerID string) (string, error) {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return "", fmt.Errorf("owner_id is required")
	}
	sandboxURL, err := resolveOwnerSandboxURL(ctx, h.cfg.VmctlURL, ownerID)
	if err != nil {
		return "", err
	}
	return sandboxURL, nil
}

func resolveOwnerSandboxURL(ctx context.Context, vmctlURL, ownerID string) (string, error) {
	body, err := json.Marshal(vmctlResolveRequest{
		UserID:    ownerID,
		DesktopID: primaryDesktopID,
	})
	if err != nil {
		return "", err
	}
	endpoint := strings.TrimRight(strings.TrimSpace(vmctlURL), "/") + "/internal/vmctl/resolve"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("vmctl resolve status %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	var resolved vmctlResolveResponse
	if err := json.Unmarshal(data, &resolved); err != nil {
		return "", err
	}
	if strings.TrimSpace(resolved.SandboxURL) == "" {
		return "", fmt.Errorf("vmctl resolved empty sandbox_url")
	}
	return strings.TrimSpace(resolved.SandboxURL), nil
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

func (h *Handler) postRuntimeTraceEvent(ctx context.Context, runtimeURL, ownerID, runID, kind string, payload map[string]any) error {
	if payload == nil {
		payload = map[string]any{}
	}
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return fmt.Errorf("owner_id is required")
	}
	endpoint := strings.TrimRight(strings.TrimSpace(runtimeURL), "/") +
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
