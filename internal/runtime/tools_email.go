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
	"regexp"
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

type maildApprovalEmailResponse struct {
	Status            string `json:"status"`
	TokenID           string `json:"token_id"`
	ProviderMessageID string `json:"provider_message_id"`
	ReviewURL         string `json:"review_url"`
	ReplyAddress      string `json:"reply_address"`
}

type maildRiskAlertResponse struct {
	Status            string `json:"status"`
	AlertID           string `json:"alert_id"`
	ProviderMessageID string `json:"provider_message_id"`
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
		}, []string{"doc_id", "revision_id", "to_addresses", "subject", "body_text"}, false),
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
	if docID == "" || revisionID == "" {
		return nil, fmt.Errorf("doc_id and revision_id are required")
	}
	toAddresses := normalizeEmailAddressList(in.ToAddresses)
	if len(toAddresses) == 0 {
		return nil, fmt.Errorf("at least one to_address is required")
	}
	subject := strings.TrimSpace(in.Subject)
	body := cleanEmailDraftBodyText(in.BodyText)
	if subject == "" || body == "" {
		return nil, fmt.Errorf("subject and body_text are required")
	}
	if sourceHash == "" {
		sourceHash = emailSourceContentHash(docID, revisionID, body)
	}
	ownerEmail := strings.TrimSpace(metadataString(parent.Metadata, runMetadataOwnerEmail))
	if ownerEmail == "" {
		ownerEmail = stringFromToolContext(ctx, toolCtxOwnerEmail)
	}
	approvalMode := strings.TrimSpace(in.ApprovalMode)
	if approvalMode == "" {
		approvalMode = "owner_click_or_email_reply"
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

	fromAlias := cleanEmailDraftFromAlias(in.FromAlias)
	draftID := "email-draft-request-" + uuid.NewString()
	versionID := "email-draft-version-" + uuid.NewString()
	draftVersionHash := emailDraftVersionHash(fromAlias, toAddresses, in.CCAddresses, in.BCCAddresses, subject, body, docID, revisionID, sourceHash)
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
		"from_alias":            fromAlias,
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
		if strings.TrimSpace(rt.cfg.MaildURL) == "" {
			result["risk_alert_status"] = "runtime_maild_url_not_configured"
		} else if ownerEmail == "" {
			result["risk_alert_status"] = "owner_email_not_available"
		} else if alert, err := rt.persistEmailRiskAlertToMaild(ctx, ownerID, ownerEmail, risk, docID+":"+revisionID, body); err != nil {
			result["risk_alert_status"] = "failed"
			result["risk_alert_error"] = err.Error()
		} else {
			result["risk_alert_status"] = alert.Status
			result["risk_alert_id"] = alert.AlertID
			result["risk_alert_provider_message_id"] = alert.ProviderMessageID
		}
	} else if strings.TrimSpace(rt.cfg.MaildURL) == "" {
		result["maild_persistence_status"] = "runtime_maild_url_not_configured"
	} else {
		persisted, err := rt.persistEmailDraftToMaild(ctx, ownerID, ownerEmail, runID, in, fromAlias, toAddresses, subject, body)
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
			if approvalMode == "owner_click_or_email_reply" {
				if ownerEmail == "" {
					result["approval_email_status"] = "owner_email_not_available"
				} else if approval, err := rt.persistEmailApprovalRequestToMaild(ctx, ownerID, ownerEmail, persisted.ID); err != nil {
					result["approval_email_status"] = "failed"
					result["approval_email_error"] = err.Error()
				} else {
					result["approval_email_status"] = approval.Status
					result["approval_email_provider_message_id"] = approval.ProviderMessageID
					result["approval_email_token_id"] = approval.TokenID
					result["approval_email_review_url"] = approval.ReviewURL
					result["approval_email_reply_address"] = approval.ReplyAddress
				}
			}
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
		runMetadataOwnerEmail:    ownerEmail,
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
	if risk != "" {
		rt.emitEvent(ctx, run, types.EventEmailDraftBlocked, events.CauseToolExecution,
			emailEventJSON(map[string]any{
				"phase":                          "email_appagent_evidence",
				"authority":                      "email_appagent",
				"action":                         "draft_blocked",
				"draft_id":                       draftID,
				"version_id":                     versionID,
				"draft_version_hash":             draftVersionHash,
				"risk_code":                      risk,
				"risk_alert_status":              result["risk_alert_status"],
				"risk_alert_id":                  result["risk_alert_id"],
				"risk_alert_provider_message_id": result["risk_alert_provider_message_id"],
				"send_authorized":                false,
				"maild_send_attempted":           false,
			}))
	}
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

func (rt *Runtime) persistEmailDraftToMaild(ctx context.Context, ownerID, ownerEmail, emailRunID string, in requestEmailDraftArgs, fromAlias string, toAddresses []string, subject, body string) (maildEmailDraftResponse, error) {
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
		"from_address":  fromAlias,
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
	if strings.TrimSpace(ownerEmail) != "" {
		req.Header.Set("X-Authenticated-Email", ownerEmail)
	}
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

func (rt *Runtime) persistEmailApprovalRequestToMaild(ctx context.Context, ownerID, ownerEmail, draftID string) (maildApprovalEmailResponse, error) {
	maildURL := strings.TrimRight(strings.TrimSpace(rt.cfg.MaildURL), "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, maildURL+"/api/email/drafts/"+draftID+"/approval-email", strings.NewReader(`{}`))
	if err != nil {
		return maildApprovalEmailResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Authenticated-User", ownerID)
	req.Header.Set("X-Authenticated-Email", ownerEmail)
	req.Header.Set("X-Internal-Caller", "true")
	return decodeMaildJSONResponse[maildApprovalEmailResponse](req)
}

func (rt *Runtime) persistEmailRiskAlertToMaild(ctx context.Context, ownerID, ownerEmail, riskKind, sourceRef, snippet string) (maildRiskAlertResponse, error) {
	maildURL := strings.TrimRight(strings.TrimSpace(rt.cfg.MaildURL), "/")
	payload := map[string]string{
		"risk_kind":  riskKind,
		"source_ref": sourceRef,
		"snippet":    snippet,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return maildRiskAlertResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, maildURL+"/api/notifications/email-risk-alert", bytes.NewReader(data))
	if err != nil {
		return maildRiskAlertResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Authenticated-User", ownerID)
	req.Header.Set("X-Authenticated-Email", ownerEmail)
	req.Header.Set("X-Internal-Caller", "true")
	return decodeMaildJSONResponse[maildRiskAlertResponse](req)
}

func decodeMaildJSONResponse[T any](req *http.Request) (T, error) {
	var zero T
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return zero, err
	}
	defer func() { _ = resp.Body.Close() }()
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return zero, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return zero, fmt.Errorf("maild status %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}
	var out T
	if err := json.Unmarshal(bodyBytes, &out); err != nil {
		return zero, err
	}
	return out, nil
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

func cleanEmailDraftFromAlias(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "\r\n\t <>") {
		return ""
	}
	loc := choirSenderAliasPattern.FindStringIndex(value)
	if loc == nil || loc[0] != 0 || loc[1] != len(value) {
		return ""
	}
	return value
}

func cleanEmailDraftBodyText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	cut := len(value)
	for _, marker := range []string{
		"</<parameter>",
		"<parameter name=",
		"<payload ",
		"</parameter>",
		"</payload>",
		"</invoke>",
	} {
		if idx := strings.Index(lower, marker); idx >= 0 && idx < cut {
			cut = idx
		}
	}
	cleaned := strings.TrimSpace(value[:cut])
	cleaned = truncateEmailIntentAtMarkers(cleaned, emailDraftArtifactTailMarkers())
	cleaned = strings.TrimSpace(cleaned)
	for {
		next := strings.TrimSpace(cleaned)
		next = trailingEmailDraftToolTagPattern.ReplaceAllString(next, "")
		next = strings.TrimSpace(next)
		next = strings.TrimSuffix(next, "</")
		next = strings.TrimSpace(next)
		if next == cleaned {
			return cleaned
		}
		cleaned = next
	}
}

func emailDraftArtifactTailMarkers() []string {
	return []string{
		"\n**instructions from user:",
		"\ninstructions from user:",
		"\n**instructions:**",
		"\n**instructions**\n",
		"\ninstructions:",
		"\ninstructions\n",
		"\n**source refs:**",
		"\nsource refs:",
		"\n**source ref:**",
		"\nsource ref:",
		"\n**source references:**",
		"\nsource references:",
		"\n## workflow",
		"\n# workflow",
		"\n**workflow:**",
		"\n**constraint:**",
		"\nconstraint:",
		"\n**constraints:**",
		"\nconstraints:",
		"\n**next step:**",
		"\nnext step:",
		"\n**notes:**",
		"\nnotes:",
		"\n---",
		"\ncreate the draft only",
		" create the draft only",
		"\ndo not send",
		" do not send",
	}
}

func emailSourceContentHash(docID, revisionID, content string) string {
	payload, _ := json.Marshal(map[string]string{
		"doc_id":      strings.TrimSpace(docID),
		"revision_id": strings.TrimSpace(revisionID),
		"content":     strings.TrimSpace(content),
	})
	sum := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(sum[:])
}

type emailDraftIntent struct {
	ToAddresses []string
	Subject     string
	BodyText    string
}

var emailAddressPattern = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)
var choirSenderAliasPattern = regexp.MustCompile(`(?i)^[0-9]+(?:\+[A-Z0-9][A-Z0-9._\-]{0,63})?@choir\.news$`)
var trailingEmailDraftToolTagPattern = regexp.MustCompile(`(?is)</[a-z][a-z0-9:_-]*>\s*$`)

func extractEmailDraftIntent(prompt, content string) (emailDraftIntent, bool) {
	combined := strings.TrimSpace(prompt + "\n" + content)
	if combined == "" {
		return emailDraftIntent{}, false
	}
	lower := strings.ToLower(combined)
	if !strings.Contains(lower, "email") && !strings.Contains(lower, "send") && !strings.Contains(lower, "draft") {
		return emailDraftIntent{}, false
	}
	to := normalizeEmailAddressList(emailAddressPattern.FindAllString(combined, -1))
	if len(to) == 0 {
		return emailDraftIntent{}, false
	}

	subject, subjectLabeled := extractEmailLabeledField(combined, "subject", []string{
		"\nbody exactly:",
		" body exactly:",
		"\n**body exactly:**",
		" **body exactly:**",
		"\nbody:",
		" body:",
		"\n**body:**",
		" **body:**",
		"\nbody\n",
		"\n**body**\n",
	})
	body, bodyLabeled := extractEmailLabeledField(combined, "body", append([]string{
		"\nstatus:",
		"\n**status:**",
	}, emailDraftArtifactTailMarkers()...))
	if !bodyLabeled {
		body, bodyLabeled = extractEmailLabeledField(combined, "body exactly", append([]string{
			"\nstatus:",
			"\n**status:**",
		}, emailDraftArtifactTailMarkers()...))
	}
	if !bodyLabeled {
		body = fallbackEmailBodyAfterAddress(combined, to[0])
	}
	if body == "" {
		return emailDraftIntent{}, false
	}
	if !bodyLabeled && looksLikeComplexEmailPlaceholder(lower) {
		return emailDraftIntent{}, false
	}
	if !subjectLabeled {
		subject = fallbackEmailSubject(body)
	} else {
		subject = strings.Trim(subject, " \t\r\n.,;:!?")
	}
	return emailDraftIntent{
		ToAddresses: to,
		Subject:     subject,
		BodyText:    body,
	}, true
}

func extractEmailLabeledField(text, label string, stopMarkers []string) (string, bool) {
	labelPattern := regexp.MustCompile(`(?i)(?:^|[\s*_` + "`" + `>-])(?:\*\*)?` + regexp.QuoteMeta(label) + `\s*:\s*(?:\*\*)?`)
	loc := labelPattern.FindStringIndex(text)
	if loc == nil {
		linePattern := regexp.MustCompile(`(?im)^[ \t>*_-]*(?:\*\*)?` + regexp.QuoteMeta(label) + `(?:\*\*)?[ \t]*$`)
		loc = linePattern.FindStringIndex(text)
		if loc == nil {
			return "", false
		}
	}
	start := loc[1]
	lower := strings.ToLower(text)
	end := len(text)
	afterLower := lower[start:]
	for _, stop := range stopMarkers {
		stop = strings.ToLower(stop)
		if stop == "" {
			continue
		}
		if stopIdx := strings.Index(afterLower, stop); stopIdx >= 0 && start+stopIdx < end {
			end = start + stopIdx
		}
	}
	return strings.TrimSpace(strings.Trim(text[start:end], " \t\r\n\"'`*")), true
}

func fallbackEmailBodyAfterAddress(text, address string) string {
	idx := strings.Index(strings.ToLower(text), strings.ToLower(address))
	if idx < 0 {
		return ""
	}
	body := strings.TrimSpace(text[idx+len(address):])
	body = strings.Trim(body, " \t\r\n:,-")
	for _, prefix := range []string{
		"with message ",
		"message ",
		"saying ",
		"that says ",
		"to say ",
	} {
		if strings.HasPrefix(strings.ToLower(body), prefix) {
			body = strings.TrimSpace(body[len(prefix):])
			break
		}
	}
	body = strings.TrimSpace(truncateEmailIntentAtMarkers(body, append([]string{
		"\nstatus:",
		"\n**status:**",
	}, emailDraftArtifactTailMarkers()...)))
	return body
}

func truncateEmailIntentAtMarkers(text string, markers []string) string {
	lower := strings.ToLower(text)
	end := len(text)
	for _, marker := range markers {
		if idx := strings.Index(lower, strings.ToLower(marker)); idx >= 0 && idx < end {
			end = idx
		}
	}
	return text[:end]
}

func fallbackEmailSubject(body string) string {
	words := strings.Fields(body)
	if len(words) == 0 {
		return "Message from Choir"
	}
	if len(words) > 8 {
		words = words[:8]
	}
	subject := strings.Join(words, " ")
	subject = strings.Trim(subject, " \t\r\n.,;:!?")
	if subject == "" {
		return "Message from Choir"
	}
	return subject
}

func looksLikeComplexEmailPlaceholder(lower string) bool {
	for _, marker := range []string{
		"figure out",
		"research",
		"investigate",
		"find out",
		"results",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
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
