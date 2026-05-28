package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type requestEmailDraftArgs struct {
	DocID             string   `json:"doc_id"`
	RevisionID        string   `json:"revision_id"`
	SourceContentHash string   `json:"source_content_hash"`
	FromAlias         string   `json:"from_alias,omitempty"`
	ToAddresses       []string `json:"to_addresses"`
	CCAddresses       []string `json:"cc_addresses,omitempty"`
	BCCAddresses      []string `json:"bcc_addresses,omitempty"`
	Subject           string   `json:"subject"`
	BodyText          string   `json:"body_text"`
	SourceRefs        []string `json:"source_refs,omitempty"`
	ApprovalMode      string   `json:"approval_mode,omitempty"`
}

type maildEmailDraftResponse struct {
	ID          string   `json:"id"`
	Status      string   `json:"status"`
	Version     int      `json:"version"`
	VersionHash string   `json:"version_hash"`
	FromAddress string   `json:"from_address"`
	ToAddresses []string `json:"to_addresses"`
	Subject     string   `json:"subject"`
}

func newRequestEmailDraftTool(rt *Runtime) Tool {
	return Tool{
		Name:        "request_email_draft",
		Description: "VText-only handoff to the Email appagent. Creates a Trace-visible versioned email draft request; it never sends mail.",
		Parameters: jsonSchemaObject(map[string]any{
			"doc_id":              map[string]any{"type": "string", "description": "Canonical VText document id that owns the email content."},
			"revision_id":         map[string]any{"type": "string", "description": "Exact VText revision id containing the email artifact."},
			"source_content_hash": map[string]any{"type": "string", "description": "Hash of the exact VText source artifact/version being handed to Email."},
			"from_alias":          map[string]any{"type": "string", "description": "Owned numeric Choir email alias. Empty means Email appagent must choose the owner default."},
			"to_addresses":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"cc_addresses":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"bcc_addresses":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"subject":             map[string]any{"type": "string"},
			"body_text":           map[string]any{"type": "string"},
			"source_refs":         map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"approval_mode":       map[string]any{"type": "string", "enum": []string{"owner_click", "owner_click_or_email_reply"}, "description": "How the owner may approve this exact draft version."},
		}, []string{"doc_id", "revision_id", "source_content_hash", "to_addresses", "subject", "body_text"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			if canonicalAgentProfile(stringFromToolContext(ctx, toolCtxProfile)) != AgentProfileVText {
				return "", fmt.Errorf("request_email_draft is only available to vtext agents")
			}
			rec := ctxRunRecord(ctx)
			if rec == nil {
				return "", fmt.Errorf("request_email_draft requires a run context")
			}
			var in requestEmailDraftArgs
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode request_email_draft args: %w", err)
			}
			result, err := rt.recordEmailDraftRequest(ctx, rec, in)
			if err != nil {
				return "", err
			}
			return toolResultJSON(result)
		},
	}
}

func (rt *Runtime) recordEmailDraftRequest(ctx context.Context, parent *types.RunRecord, in requestEmailDraftArgs) (map[string]any, error) {
	if rt == nil || rt.store == nil {
		return nil, fmt.Errorf("runtime store unavailable")
	}
	ownerID := strings.TrimSpace(parent.OwnerID)
	if ownerID == "" {
		return nil, fmt.Errorf("owner_id is required")
	}
	docID := strings.TrimSpace(in.DocID)
	revisionID := strings.TrimSpace(in.RevisionID)
	sourceHash := strings.TrimSpace(in.SourceContentHash)
	if docID == "" || revisionID == "" || sourceHash == "" {
		return nil, fmt.Errorf("doc_id, revision_id, and source_content_hash are required")
	}
	toAddresses := normalizeEmailAddressList(in.ToAddresses)
	if len(toAddresses) == 0 {
		return nil, fmt.Errorf("at least one to_address is required")
	}
	subject := strings.TrimSpace(in.Subject)
	body := strings.TrimSpace(in.BodyText)
	if subject == "" || body == "" {
		return nil, fmt.Errorf("subject and body_text are required")
	}
	approvalMode := strings.TrimSpace(in.ApprovalMode)
	if approvalMode == "" {
		approvalMode = "owner_click"
	}
	if approvalMode != "owner_click" && approvalMode != "owner_click_or_email_reply" {
		return nil, fmt.Errorf("approval_mode must be owner_click or owner_click_or_email_reply")
	}

	agentID := persistentEmailAgentID(ownerID)
	now := time.Now().UTC()
	runID := uuid.NewString()
	if err := rt.store.UpsertAgent(ctx, types.AgentRecord{
		AgentID:   agentID,
		OwnerID:   ownerID,
		SandboxID: rt.cfg.SandboxID,
		Profile:   AgentProfileEmail,
		Role:      AgentProfileEmail,
		ChannelID: agentID,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		return nil, fmt.Errorf("persist email appagent: %w", err)
	}

	draftID := "email-draft-request-" + uuid.NewString()
	versionID := "email-draft-version-" + uuid.NewString()
	draftVersionHash := emailDraftVersionHash(in.FromAlias, toAddresses, in.CCAddresses, in.BCCAddresses, subject, body, docID, revisionID, sourceHash)
	risk := detectEmailDraftPolicyRisk(subject, body, toAddresses, in.CCAddresses, in.BCCAddresses)
	status := "draft_pending_owner_approval"
	if risk != "" {
		status = "blocked_risk_alert_required"
	}

	result := map[string]any{
		"status":                status,
		"draft_id":              draftID,
		"draft_version_id":      versionID,
		"draft_version_hash":    draftVersionHash,
		"doc_id":                docID,
		"revision_id":           revisionID,
		"source_content_hash":   sourceHash,
		"source_kind":           "vtext_email_artifact",
		"approval_mode":         approvalMode,
		"from_alias":            strings.TrimSpace(in.FromAlias),
		"to_addresses":          toAddresses,
		"cc_addresses":          normalizeEmailAddressList(in.CCAddresses),
		"bcc_addresses":         normalizeEmailAddressList(in.BCCAddresses),
		"subject":               subject,
		"send_authorized":       false,
		"maild_send_attempted":  false,
		"maild_draft_persisted": false,
	}
	if risk != "" {
		result["risk_code"] = risk
		result["risk_alert_subject"] = "[Choir Risk Alert] Email draft blocked"
	} else if strings.TrimSpace(rt.cfg.MaildURL) == "" {
		result["maild_persistence_status"] = "runtime_maild_url_not_configured"
	} else {
		persisted, err := rt.persistEmailDraftToMaild(ctx, ownerID, runID, in, toAddresses, subject, body)
		if err != nil {
			status = "draft_persistence_failed"
			result["status"] = status
			result["maild_persistence_status"] = "failed"
			result["maild_persistence_error"] = err.Error()
		} else {
			draftID = persisted.ID
			versionID = fmt.Sprintf("%s-v%d", persisted.ID, persisted.Version)
			draftVersionHash = persisted.VersionHash
			result["status"] = persisted.Status
			result["draft_id"] = draftID
			result["draft_version_id"] = versionID
			result["draft_version_hash"] = draftVersionHash
			result["from_alias"] = persisted.FromAddress
			result["to_addresses"] = persisted.ToAddresses
			result["subject"] = persisted.Subject
			result["maild_draft_persisted"] = true
			result["maild_persistence_status"] = "persisted"
			result["maild_draft_id"] = persisted.ID
			result["maild_draft_version_hash"] = persisted.VersionHash
		}
	}

	metadata := map[string]any{
		runMetadataAgentProfile:  AgentProfileEmail,
		runMetadataAgentRole:     AgentProfileEmail,
		runMetadataAgentID:       agentID,
		runMetadataChannelID:     agentID,
		runMetadataDesktopID:     desktopIDForRun(parent),
		"parent_id":              parent.RunID,
		"source_agent_profile":   AgentProfileVText,
		"email_action":           "draft_request",
		"email_draft_id":         draftID,
		"email_draft_version_id": versionID,
		"email_draft_hash":       draftVersionHash,
		"email_policy_status":    status,
		"maild_draft_persisted":  result["maild_draft_persisted"],
		"doc_id":                 docID,
		"revision_id":            revisionID,
	}
	metadata = ensureTrajectoryID(metadata, parent, runID)
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshal email appagent result: %w", err)
	}
	run := &types.RunRecord{
		RunID:        runID,
		AgentID:      agentID,
		ChannelID:    agentID,
		ParentRunID:  parent.RunID,
		AgentProfile: AgentProfileEmail,
		AgentRole:    AgentProfileEmail,
		OwnerID:      ownerID,
		SandboxID:    rt.cfg.SandboxID,
		State:        types.RunCompleted,
		Prompt:       "Create an Email appagent draft request from VText artifact " + revisionID,
		Result:       string(resultBytes),
		CreatedAt:    now,
		UpdatedAt:    now,
		FinishedAt:   &now,
		Metadata:     metadata,
	}
	if err := rt.store.CreateRun(ctx, *run); err != nil {
		return nil, fmt.Errorf("persist email appagent run: %w", err)
	}
	rt.emitEvent(ctx, run, types.EventRunSubmitted, events.CauseTaskLifecycle, emailEventJSON(map[string]any{
		"prompt_length": len(run.Prompt),
		"parent_id":     parent.RunID,
	}))
	rt.emitEvent(ctx, run, types.EventRunStarted, events.CauseTaskLifecycle, emailEventJSON(map[string]any{
		"authority": "email_appagent",
		"action":    "draft_request",
	}))
	rt.emitEvent(ctx, run, types.EventRunCompleted, events.CauseTaskLifecycle, emailEventJSON(map[string]any{
		"authority":  "email_appagent",
		"action":     "draft_request",
		"status":     status,
		"draft_id":   draftID,
		"version_id": versionID,
	}))
	result["agent_id"] = agentID
	result["loop_id"] = runID
	result["channel_id"] = agentID
	result["profile"] = AgentProfileEmail
	return result, nil
}

func (rt *Runtime) persistEmailDraftToMaild(ctx context.Context, ownerID, emailRunID string, in requestEmailDraftArgs, toAddresses []string, subject, body string) (maildEmailDraftResponse, error) {
	maildURL := strings.TrimRight(strings.TrimSpace(rt.cfg.MaildURL), "/")
	if maildURL == "" {
		return maildEmailDraftResponse{}, fmt.Errorf("runtime maild url is not configured")
	}
	sourceRef, _ := json.Marshal(map[string]string{
		"doc_id":                strings.TrimSpace(in.DocID),
		"revision_id":           strings.TrimSpace(in.RevisionID),
		"source_content_hash":   strings.TrimSpace(in.SourceContentHash),
		"email_appagent_run_id": strings.TrimSpace(emailRunID),
	})
	payload := map[string]any{
		"from_address":  strings.TrimSpace(in.FromAlias),
		"to_addresses":  toAddresses,
		"cc_addresses":  normalizeEmailAddressList(in.CCAddresses),
		"bcc_addresses": normalizeEmailAddressList(in.BCCAddresses),
		"subject":       subject,
		"text_body":     body,
		"source_kind":   "vtext_email_artifact",
		"source_ref":    string(sourceRef),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return maildEmailDraftResponse{}, fmt.Errorf("marshal maild draft payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, maildURL+"/api/email/drafts", bytes.NewReader(data))
	if err != nil {
		return maildEmailDraftResponse{}, fmt.Errorf("create maild draft request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Authenticated-User", ownerID)
	req.Header.Set("X-Internal-Caller", "true")
	req.Header.Set("X-Choir-Email-Appagent-Run", emailRunID)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return maildEmailDraftResponse{}, fmt.Errorf("maild draft request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return maildEmailDraftResponse{}, fmt.Errorf("read maild draft response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return maildEmailDraftResponse{}, fmt.Errorf("maild draft status %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}
	var draft maildEmailDraftResponse
	if err := json.Unmarshal(bodyBytes, &draft); err != nil {
		return maildEmailDraftResponse{}, fmt.Errorf("decode maild draft response: %w", err)
	}
	if strings.TrimSpace(draft.ID) == "" || strings.TrimSpace(draft.VersionHash) == "" {
		return maildEmailDraftResponse{}, fmt.Errorf("maild draft response missing id or version hash")
	}
	return draft, nil
}

func persistentEmailAgentID(ownerID string) string {
	return "email:" + strings.TrimSpace(ownerID)
}

func normalizeEmailAddressList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || strings.ContainsAny(value, "\r\n") {
			continue
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	return out
}

func emailDraftVersionHash(from string, to, cc, bcc []string, subject, body, docID, revisionID, sourceHash string) string {
	payload, _ := json.Marshal(map[string]any{
		"from":                strings.TrimSpace(from),
		"to":                  normalizeEmailAddressList(to),
		"cc":                  normalizeEmailAddressList(cc),
		"bcc":                 normalizeEmailAddressList(bcc),
		"subject":             strings.TrimSpace(subject),
		"body_text":           strings.TrimSpace(body),
		"doc_id":              strings.TrimSpace(docID),
		"revision_id":         strings.TrimSpace(revisionID),
		"source_content_hash": strings.TrimSpace(sourceHash),
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func detectEmailDraftPolicyRisk(subject, body string, to, cc, bcc []string) string {
	for _, addr := range append(append(append([]string{}, to...), cc...), bcc...) {
		if strings.ContainsAny(addr, "\r\n") {
			return "recipient_header_injection"
		}
	}
	text := strings.ToLower(subject + "\n" + body)
	for _, marker := range []string{
		"ignore previous instructions",
		"ignore all previous instructions",
		"approve this email",
		"reply approve",
		"send this without approval",
		"hidden recipient",
		"bcc:",
		"system prompt",
	} {
		if strings.Contains(text, marker) {
			return "suspected_prompt_injection"
		}
	}
	return ""
}

func emailEventJSON(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return data
}
