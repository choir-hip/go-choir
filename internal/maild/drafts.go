package maild

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type aliasListResponse struct {
	Aliases []aliasResponse `json:"aliases"`
}

type aliasResponse struct {
	ID         string `json:"id"`
	Address    string `json:"address"`
	LocalPart  string `json:"local_part"`
	Domain     string `json:"domain"`
	Visibility string `json:"visibility"`
}

type createDraftRequest struct {
	FromAddress      string   `json:"from_address"`
	ToAddresses      []string `json:"to_addresses"`
	CcAddresses      []string `json:"cc_addresses,omitempty"`
	BccAddresses     []string `json:"bcc_addresses,omitempty"`
	Subject          string   `json:"subject"`
	TextBody         string   `json:"text_body"`
	HTMLBody         string   `json:"html_body,omitempty"`
	ReplyToMessageID string   `json:"reply_to_message_id,omitempty"`
	SourceKind       string   `json:"source_kind,omitempty"`
	SourceRef        string   `json:"source_ref,omitempty"`
}

type draftResponse struct {
	ID                string   `json:"id"`
	Status            string   `json:"status"`
	Version           int      `json:"version"`
	VersionHash       string   `json:"version_hash"`
	FromAddress       string   `json:"from_address"`
	ToAddresses       []string `json:"to_addresses"`
	CcAddresses       []string `json:"cc_addresses,omitempty"`
	BccAddresses      []string `json:"bcc_addresses,omitempty"`
	Subject           string   `json:"subject"`
	TextBody          string   `json:"text_body,omitempty"`
	HTMLBody          string   `json:"html_body,omitempty"`
	ReplyToMessageID  string   `json:"reply_to_message_id,omitempty"`
	SourceKind        string   `json:"source_kind,omitempty"`
	SourceRef         string   `json:"source_ref,omitempty"`
	SentMessageID     string   `json:"sent_message_id,omitempty"`
	ProviderMessageID string   `json:"provider_message_id,omitempty"`
	CreatedAt         string   `json:"created_at"`
	UpdatedAt         string   `json:"updated_at"`
}

type draftListResponse struct {
	Drafts []draftResponse `json:"drafts"`
}

type sendDraftResponse struct {
	Draft             draftResponse `json:"draft"`
	MessageID         string        `json:"message_id"`
	ProviderMessageID string        `json:"provider_message_id"`
	ApprovalEventID   string        `json:"approval_event_id"`
	Status            string        `json:"status"`
}

type sendDraftRequest struct {
	VersionHash string `json:"version_hash"`
}

const approvalEmailDraftBodyPreviewRunes = 4000

type approvalEmailResponse struct {
	Status            string `json:"status"`
	TokenID           string `json:"token_id"`
	ProviderMessageID string `json:"provider_message_id"`
	ReviewURL         string `json:"review_url"`
	ReplyAddress      string `json:"reply_address"`
}

func (h *Handler) HandleAliases(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	ownerID, ok := authenticatedInternalOwner(w, r)
	if !ok {
		return
	}
	aliases, err := h.store.ListAliasesForOwner(r.Context(), ownerID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list aliases"})
		return
	}
	out := make([]aliasResponse, 0, len(aliases))
	for _, alias := range aliases {
		out = append(out, aliasResponse{
			ID:         alias.ID,
			Address:    alias.Address,
			LocalPart:  alias.LocalPart,
			Domain:     alias.Domain,
			Visibility: alias.Visibility,
		})
	}
	writeJSON(w, http.StatusOK, aliasListResponse{Aliases: out})
}

func (h *Handler) HandleDrafts(w http.ResponseWriter, r *http.Request) {
	ownerID, ownerEmail, ok := authenticatedInternalOwnerWithEmail(w, r)
	if !ok {
		return
	}
	if r.URL.Path == "/api/email/drafts" {
		switch r.Method {
		case http.MethodGet:
			h.handleDraftList(w, r, ownerID)
		case http.MethodPost:
			h.handleDraftCreate(w, r, ownerID)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
		return
	}
	const prefix = "/api/email/drafts/"
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/")
	if rest == "" || rest == r.URL.Path {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	parts := strings.Split(rest, "/")
	draftID := parts[0]
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		h.handleDraftDetail(w, r, ownerID, draftID)
		return
	}
	if len(parts) == 2 && parts[1] == "send" {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		h.handleDraftSend(w, r, ownerID, draftID)
		return
	}
	if len(parts) == 2 && parts[1] == "approval-email" {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		h.handleDraftApprovalEmail(w, r, ownerID, ownerEmail, draftID)
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
}

func (h *Handler) handleDraftList(w http.ResponseWriter, r *http.Request, ownerID string) {
	drafts, err := h.store.ListDrafts(r.Context(), ownerID, 50)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list drafts"})
		return
	}
	out := make([]draftResponse, 0, len(drafts))
	for _, draft := range drafts {
		out = append(out, summarizeDraft(draft))
	}
	writeJSON(w, http.StatusOK, draftListResponse{Drafts: out})
}

func (h *Handler) handleDraftCreate(w http.ResponseWriter, r *http.Request, ownerID string) {
	var in createDraftRequest
	if err := h.decodeJSON(r, &in); err != nil {
		writeDecodeError(w, err)
		return
	}
	alias, err := h.resolveOwnedDraftFromAlias(r.Context(), ownerID, in.FromAddress)
	if err != nil {
		if err == sql.ErrNoRows {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "from address is not owned by current user"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to resolve from address"})
		return
	}
	draft, err := h.store.CreateDraft(r.Context(), ownerID, alias, in)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, summarizeDraft(draft))
}

func (h *Handler) resolveOwnedDraftFromAlias(ctx context.Context, ownerID, fromAddress string) (EmailAlias, error) {
	if strings.TrimSpace(fromAddress) == "" {
		return h.store.DefaultAliasForOwner(ctx, ownerID)
	}
	return h.resolveOwnedFromAlias(ctx, ownerID, fromAddress)
}

func (h *Handler) handleDraftDetail(w http.ResponseWriter, r *http.Request, ownerID, draftID string) {
	draft, err := h.store.GetDraft(r.Context(), ownerID, draftID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, summarizeDraft(draft))
}

func (h *Handler) handleDraftSend(w http.ResponseWriter, r *http.Request, ownerID, draftID string) {
	var in sendDraftRequest
	if err := h.decodeJSON(r, &in); err != nil {
		writeDecodeError(w, err)
		return
	}
	resp, err := h.sendApprovedDraft(r.Context(), ownerID, draftID, in.VersionHash, "owner_click_approved", "")
	if err != nil {
		writeDraftSendError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, resp)
}

func (h *Handler) handleDraftApprovalEmail(w http.ResponseWriter, r *http.Request, ownerID, ownerEmail, draftID string) {
	if ownerEmail == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "verified signup email is required"})
		return
	}
	draft, err := h.store.GetDraft(r.Context(), ownerID, draftID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if draft.Status == "sent" {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "draft already sent"})
		return
	}
	resp, err := h.sendDraftApprovalEmail(r.Context(), draft, ownerEmail)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to send approval email"})
		return
	}
	writeJSON(w, http.StatusAccepted, resp)
}

func (h *Handler) sendDraftApprovalEmail(ctx context.Context, draft EmailDraft, ownerEmail string) (approvalEmailResponse, error) {
	token, err := h.store.CreateDraftApprovalToken(ctx, draft, ownerEmail, 24*time.Hour)
	if err != nil {
		return approvalEmailResponse{}, fmt.Errorf("create approval token: %w", err)
	}
	reviewURL := "https://choir.news/?app=email&draft=" + draft.ID + "&approval=" + token.Token
	replyAddress := "approve+" + token.Token + "@" + strings.ToLower(strings.TrimSpace(h.cfg.PrimaryDomain))
	body := buildDraftApprovalEmailBody(draft, reviewURL)
	sent, err := h.resend.sendEmail(ctx, resendSendRequest{
		From:    "Choir <updates@choir.news>",
		To:      []string{ownerEmail},
		ReplyTo: []string{replyAddress},
		Subject: "Choir email draft needs approval: " + draft.Subject,
		Text:    body,
		Headers: map[string]any{
			"X-Choir-Maild":                     "v0-email-draft-approval",
			"X-Choir-Email-Draft-ID":            draft.ID,
			"X-Choir-Email-Draft-Version-Hash":  draft.VersionHash,
			"X-Choir-Email-Approval-Token-ID":   token.ID,
			"X-Choir-Email-Approval-Reply-Port": replyAddress,
		},
	})
	if err != nil {
		return approvalEmailResponse{}, err
	}
	token.ProviderMessageID = sent.ID
	if err := h.store.MarkDraftApprovalTokenSent(ctx, token.ID, sent.ID); err != nil {
		return approvalEmailResponse{}, fmt.Errorf("record approval email: %w", err)
	}
	return approvalEmailResponse{
		Status:            "sent",
		TokenID:           token.ID,
		ProviderMessageID: sent.ID,
		ReviewURL:         reviewURL,
		ReplyAddress:      replyAddress,
	}, nil
}

func buildDraftApprovalEmailBody(draft EmailDraft, reviewURL string) string {
	toLine := strings.Join(decodeAddressJSON(draft.ToJSON), ", ")
	if toLine == "" {
		toLine = "(no recipients)"
	}
	bodyPreview := approvalEmailDraftBodyPreview(draft.TextBody)
	return fmt.Sprintf("Choir email draft needs approval.\n\nFrom: %s\nTo: %s\nSubject: %s\n\nDraft message:\n%s\n\nOpen in Choir to review and send:\n%s\n\nOr reply to this email with one of:\napprove\nreject\nedit: <requested change>\n\nOpening the link does not send the draft.",
		draft.FromAddress, toLine, draft.Subject, bodyPreview, reviewURL)
}

func approvalEmailDraftBodyPreview(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return "(No plain text body.)"
	}
	runes := []rune(body)
	if len(runes) <= approvalEmailDraftBodyPreviewRunes {
		return body
	}
	return string(runes[:approvalEmailDraftBodyPreviewRunes]) + "\n\n[Draft message preview truncated. Open in Choir to review the full draft.]"
}

func (h *Handler) sendApprovedDraft(ctx context.Context, ownerID, draftID, versionHash, eventType, approvalProviderMessageID string) (sendDraftResponse, error) {
	draft, err := h.store.GetDraft(ctx, ownerID, draftID)
	if err != nil {
		return sendDraftResponse{}, err
	}
	if draft.Status == "sent" {
		return sendDraftResponse{}, errDraftAlreadySent
	}
	if strings.TrimSpace(versionHash) == "" || strings.TrimSpace(versionHash) != draft.VersionHash {
		return sendDraftResponse{}, errDraftVersionChanged
	}
	alias, err := h.resolveOwnedFromAlias(ctx, ownerID, draft.FromAddress)
	if err != nil {
		return sendDraftResponse{}, errDraftForbidden
	}
	approvalEventID, err := h.store.RecordDraftApprovalEvent(ctx, draft, eventType, approvalProviderMessageID)
	if err != nil {
		return sendDraftResponse{}, fmt.Errorf("record approval: %w", err)
	}
	sendReq := draft.toSendRequest()
	sendReq.TextBody = cleanApprovedDraftTextBody(sendReq.TextBody, draft.SourceKind)
	payload, err := buildResendSendRequest(sendReq, alias)
	if err != nil {
		return sendDraftResponse{}, err
	}
	sendReq.HTMLBody = payload.HTML
	payload.Headers["X-Choir-Maild"] = "v0-approved-draft-send"
	payload.Headers["X-Choir-Email-Draft-ID"] = draft.ID
	payload.Headers["X-Choir-Email-Draft-Version-Hash"] = draft.VersionHash
	if err := h.applyReplyHeaders(ctx, ownerID, draft.ReplyToMessageID, &payload); err != nil {
		return sendDraftResponse{}, errDraftInvalidReplyTarget
	}
	sent, err := h.resend.sendEmail(ctx, payload)
	if err != nil {
		return sendDraftResponse{}, err
	}
	msg, err := h.store.StoreOutboundMessage(ctx, ownerID, alias, sent.ID, sendReq)
	if err != nil {
		return sendDraftResponse{}, fmt.Errorf("store sent: %w", err)
	}
	updated, err := h.store.MarkDraftSent(ctx, ownerID, draftID, msg.ID, sent.ID)
	if err != nil {
		return sendDraftResponse{}, fmt.Errorf("mark sent: %w", err)
	}
	tracePayloadDraft := draft
	tracePayloadDraft.OwnerID = ownerID
	h.emitApprovedDraftTraceEvents(ctx, tracePayloadDraft, approvalEventID, eventType, approvalProviderMessageID, msg.ID, sent.ID)
	return sendDraftResponse{
		Draft:             summarizeDraft(updated),
		MessageID:         msg.ID,
		ProviderMessageID: sent.ID,
		ApprovalEventID:   approvalEventID,
		Status:            "sent",
	}, nil
}

func cleanApprovedDraftTextBody(body, sourceKind string) string {
	body = strings.TrimSpace(body)
	if body == "" || sourceKind != "vtext_email_artifact" {
		return body
	}
	lower := strings.ToLower(body)
	cut := len(body)
	for _, marker := range []string{
		"\n## workflow",
		"\n# workflow",
		"\n**workflow:**",
		"\n## source ref",
		"\n# source ref",
		"\n**source ref:**",
		"\n**source refs:**",
		"\n**source references:**",
		"\n## outbound send",
		"\n# outbound send",
		"\n**outbound send:**",
		"\n---",
	} {
		if idx := strings.Index(lower, marker); idx >= 0 && idx < cut {
			cut = idx
		}
	}
	return strings.TrimSpace(body[:cut])
}

var (
	errDraftAlreadySent        = fmt.Errorf("draft already sent")
	errDraftVersionChanged     = fmt.Errorf("draft version changed")
	errDraftForbidden          = fmt.Errorf("draft forbidden")
	errDraftInvalidReplyTarget = fmt.Errorf("draft reply target invalid")
)

func writeDraftSendError(w http.ResponseWriter, err error) {
	switch {
	case err == sql.ErrNoRows:
		writeStoreError(w, err)
	case err == errDraftAlreadySent:
		writeJSON(w, http.StatusConflict, map[string]string{"error": "draft already sent"})
	case err == errDraftVersionChanged:
		writeJSON(w, http.StatusConflict, map[string]string{"error": "draft version changed; reopen the draft before approving"})
	case err == errDraftForbidden:
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "from address is not owned by current user"})
	case err == errDraftInvalidReplyTarget:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "reply target is invalid"})
	default:
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to send email"})
	}
}

func (s *Store) ListAliasesForOwner(ctx context.Context, ownerID string) ([]EmailAliasSummary, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, domain, local_part, visibility
		FROM email_aliases
		WHERE target_id = ? AND target_type = 'user' AND disabled_at IS NULL
		ORDER BY canonical_number IS NULL, canonical_number, local_part`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EmailAliasSummary
	for rows.Next() {
		var alias EmailAliasSummary
		if err := rows.Scan(&alias.ID, &alias.Domain, &alias.LocalPart, &alias.Visibility); err != nil {
			return nil, err
		}
		alias.Address = strings.ToLower(alias.LocalPart) + "@" + strings.ToLower(alias.Domain)
		out = append(out, alias)
	}
	return out, rows.Err()
}

func (s *Store) DefaultAliasForOwner(ctx context.Context, ownerID string) (EmailAlias, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, domain, local_part, coalesce(canonical_number, -1),
		target_type, target_id, visibility, receive_policy_id
		FROM email_aliases
		WHERE target_id = ? AND target_type = 'user' AND disabled_at IS NULL
		ORDER BY canonical_number IS NULL, canonical_number, local_part
		LIMIT 1`, ownerID)
	var alias EmailAlias
	if err := row.Scan(&alias.ID, &alias.Domain, &alias.LocalPart, &alias.CanonicalNumber, &alias.TargetType, &alias.TargetID, &alias.Visibility, &alias.ReceivePolicyID); err != nil {
		return EmailAlias{}, err
	}
	return alias, nil
}

func (s *Store) CreateDraft(ctx context.Context, ownerID string, alias EmailAlias, in createDraftRequest) (EmailDraft, error) {
	to := cleanAddresses(in.ToAddresses)
	if len(to) == 0 {
		return EmailDraft{}, fmt.Errorf("at least one recipient is required")
	}
	subject := strings.TrimSpace(in.Subject)
	if subject == "" {
		subject = "(no subject)"
	}
	text := strings.TrimSpace(in.TextBody)
	html := strings.TrimSpace(in.HTMLBody)
	if text == "" && html == "" {
		return EmailDraft{}, fmt.Errorf("message body is required")
	}
	cc := cleanAddresses(in.CcAddresses)
	bcc := cleanAddresses(in.BccAddresses)
	toJSON, _ := json.Marshal(to)
	ccJSON, _ := json.Marshal(cc)
	bccJSON, _ := json.Marshal(bcc)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	draft := EmailDraft{
		ID:               "email-draft-" + uuid.NewString(),
		OwnerID:          ownerID,
		FromAliasID:      alias.ID,
		FromAddress:      canonicalAliasAddress(alias),
		ToJSON:           string(toJSON),
		CcJSON:           string(ccJSON),
		BccJSON:          string(bccJSON),
		Subject:          subject,
		TextBody:         text,
		HTMLBody:         html,
		ReplyToMessageID: strings.TrimSpace(in.ReplyToMessageID),
		SourceKind:       strings.TrimSpace(in.SourceKind),
		SourceRef:        strings.TrimSpace(in.SourceRef),
		Status:           "draft_pending_owner_approval",
		Version:          1,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	draft.VersionHash = draftVersionHash(draft)
	_, err := s.db.ExecContext(ctx, `INSERT INTO email_drafts (
		id, owner_id, from_alias_id, from_address, to_json, cc_json, bcc_json,
		subject, text_body, html_body, reply_to_message_id, source_kind, source_ref,
		status, version, version_hash, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		draft.ID, draft.OwnerID, draft.FromAliasID, draft.FromAddress, draft.ToJSON, draft.CcJSON, draft.BccJSON,
		draft.Subject, nullString(draft.TextBody), nullString(draft.HTMLBody), nullString(draft.ReplyToMessageID),
		nullString(draft.SourceKind), nullString(draft.SourceRef), draft.Status, draft.Version, draft.VersionHash,
		draft.CreatedAt, draft.UpdatedAt)
	if err != nil {
		return EmailDraft{}, fmt.Errorf("insert draft: %w", err)
	}
	return draft, nil
}

func (s *Store) ListDrafts(ctx context.Context, ownerID string, limit int) ([]EmailDraft, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, draftSelectSQL()+` WHERE owner_id = ? ORDER BY updated_at DESC LIMIT ?`, ownerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EmailDraft
	for rows.Next() {
		draft, err := scanDraft(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, draft)
	}
	return out, rows.Err()
}

func (s *Store) GetDraft(ctx context.Context, ownerID, draftID string) (EmailDraft, error) {
	row := s.db.QueryRowContext(ctx, draftSelectSQL()+` WHERE owner_id = ? AND id = ?`, ownerID, draftID)
	return scanDraft(row)
}

func (s *Store) MarkDraftSent(ctx context.Context, ownerID, draftID, messageID, providerMessageID string) (EmailDraft, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return EmailDraft{}, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()
	result, err := tx.ExecContext(ctx, `UPDATE email_drafts
		SET status = 'sent', sent_message_id = ?, provider_message_id = ?, updated_at = ?
		WHERE owner_id = ? AND id = ? AND status <> 'sent'`,
		messageID, providerMessageID, now, ownerID, draftID)
	if err != nil {
		return EmailDraft{}, fmt.Errorf("mark draft sent: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return EmailDraft{}, sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, `UPDATE email_draft_approval_tokens
		SET status = 'stale_sent', used_at = coalesce(used_at, ?)
		WHERE owner_id = ? AND draft_id = ? AND status = 'active'`,
		now, ownerID, draftID); err != nil {
		return EmailDraft{}, fmt.Errorf("stale sent draft approval tokens: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return EmailDraft{}, err
	}
	tx = nil
	return s.GetDraft(ctx, ownerID, draftID)
}

func (s *Store) RecordDraftApprovalEvent(ctx context.Context, draft EmailDraft, eventType, providerMessageID string) (string, error) {
	eventType = strings.TrimSpace(eventType)
	if eventType == "" {
		return "", fmt.Errorf("approval event type is required")
	}
	eventID := "email-approval-" + uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `INSERT INTO email_draft_approval_events (
		id, draft_id, owner_id, version, version_hash, event_type, provider_message_id, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		eventID,
		draft.ID,
		draft.OwnerID,
		draft.Version,
		draft.VersionHash,
		eventType,
		nullString(providerMessageID),
		now,
	)
	if err != nil {
		return "", fmt.Errorf("insert approval event: %w", err)
	}
	return eventID, nil
}

func (s *Store) CreateDraftApprovalToken(ctx context.Context, draft EmailDraft, approvalEmail string, ttl time.Duration) (EmailApprovalToken, error) {
	approvalEmail = normalizedTrustedEmail(approvalEmail)
	if approvalEmail == "" {
		return EmailApprovalToken{}, fmt.Errorf("approval email is required")
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	now := time.Now().UTC()
	token := EmailApprovalToken{
		ID:            "email-approval-token-" + uuid.NewString(),
		Token:         strings.ReplaceAll(uuid.NewString(), "-", ""),
		DraftID:       draft.ID,
		OwnerID:       draft.OwnerID,
		Version:       draft.Version,
		VersionHash:   draft.VersionHash,
		ApprovalEmail: approvalEmail,
		Status:        "active",
		CreatedAt:     now.Format(time.RFC3339Nano),
		ExpiresAt:     now.Add(ttl).Format(time.RFC3339Nano),
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE email_draft_approval_tokens
		SET status = 'superseded'
		WHERE draft_id = ? AND owner_id = ? AND status = 'active'`,
		draft.ID, draft.OwnerID); err != nil {
		return EmailApprovalToken{}, fmt.Errorf("supersede approval tokens: %w", err)
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO email_draft_approval_tokens (
		id, token, draft_id, owner_id, version, version_hash, approval_email,
		status, created_at, expires_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		token.ID, token.Token, token.DraftID, token.OwnerID, token.Version, token.VersionHash,
		token.ApprovalEmail, token.Status, token.CreatedAt, token.ExpiresAt)
	if err != nil {
		return EmailApprovalToken{}, fmt.Errorf("insert approval token: %w", err)
	}
	return token, nil
}

func (s *Store) MarkDraftApprovalTokenSent(ctx context.Context, tokenID, providerMessageID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE email_draft_approval_tokens
		SET provider_message_id = ?
		WHERE id = ?`, nullString(providerMessageID), tokenID)
	if err != nil {
		return fmt.Errorf("mark approval token sent: %w", err)
	}
	return nil
}

func (s *Store) GetDraftApprovalToken(ctx context.Context, token string) (EmailApprovalToken, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, token, draft_id, owner_id, version,
		version_hash, approval_email, status, coalesce(provider_message_id, ''),
		created_at, expires_at, coalesce(used_at, '')
		FROM email_draft_approval_tokens
		WHERE token = ?`, strings.TrimSpace(token))
	var out EmailApprovalToken
	if err := row.Scan(&out.ID, &out.Token, &out.DraftID, &out.OwnerID, &out.Version, &out.VersionHash, &out.ApprovalEmail, &out.Status, &out.ProviderMessageID, &out.CreatedAt, &out.ExpiresAt, &out.UsedAt); err != nil {
		return EmailApprovalToken{}, err
	}
	return out, nil
}

func (s *Store) UseDraftApprovalToken(ctx context.Context, tokenID, status string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx, `UPDATE email_draft_approval_tokens
		SET status = ?, used_at = ?
		WHERE id = ? AND status = 'active'`,
		status, now, tokenID)
	if err != nil {
		return fmt.Errorf("use approval token: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) UpdateDraftTextFromApprovalEdit(ctx context.Context, draft EmailDraft, editText string) (EmailDraft, error) {
	editText = strings.TrimSpace(editText)
	if editText == "" {
		return EmailDraft{}, fmt.Errorf("edit text is required")
	}
	updated := draft
	updated.TextBody = "Owner approval reply requested edits:\n\n" + editText
	updated.Version++
	updated.Status = "draft_pending_owner_approval"
	updated.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	updated.VersionHash = draftVersionHash(updated)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return EmailDraft{}, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()
	if _, err := tx.ExecContext(ctx, `UPDATE email_drafts
		SET text_body = ?, status = ?, version = ?, version_hash = ?, updated_at = ?
		WHERE id = ? AND owner_id = ? AND status <> 'sent'`,
		updated.TextBody, updated.Status, updated.Version, updated.VersionHash, updated.UpdatedAt, updated.ID, updated.OwnerID); err != nil {
		return EmailDraft{}, fmt.Errorf("update draft edit: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE email_draft_approval_tokens
		SET status = 'superseded'
		WHERE draft_id = ? AND owner_id = ? AND status = 'active'`,
		updated.ID, updated.OwnerID); err != nil {
		return EmailDraft{}, fmt.Errorf("supersede edited draft tokens: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return EmailDraft{}, err
	}
	tx = nil
	return s.GetDraft(ctx, updated.OwnerID, updated.ID)
}

func (s *Store) RecordRiskAlert(ctx context.Context, ownerID, riskKind, sourceRef, snippet, providerMessageID string) (EmailRiskAlert, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	alert := EmailRiskAlert{
		ID:                "email-risk-alert-" + uuid.NewString(),
		OwnerID:           strings.TrimSpace(ownerID),
		RiskKind:          strings.TrimSpace(riskKind),
		SourceRef:         strings.TrimSpace(sourceRef),
		Snippet:           strings.TrimSpace(snippet),
		ProviderMessageID: strings.TrimSpace(providerMessageID),
		CreatedAt:         now,
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO email_risk_alerts (
		id, owner_id, risk_kind, source_ref, snippet, provider_message_id, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		alert.ID, alert.OwnerID, alert.RiskKind, nullString(alert.SourceRef), nullString(alert.Snippet), nullString(alert.ProviderMessageID), alert.CreatedAt)
	if err != nil {
		return EmailRiskAlert{}, fmt.Errorf("insert risk alert: %w", err)
	}
	return alert, nil
}

func (s *Store) CountDraftApprovalEvents(ctx context.Context, draftID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM email_draft_approval_events WHERE draft_id = ?`, draftID).Scan(&count)
	return count, err
}

func draftSelectSQL() string {
	return `SELECT id, owner_id, from_alias_id, from_address, to_json, cc_json, bcc_json,
		subject, coalesce(text_body, ''), coalesce(html_body, ''), coalesce(reply_to_message_id, ''),
		coalesce(source_kind, ''), coalesce(source_ref, ''), status, version, version_hash,
		coalesce(sent_message_id, ''), coalesce(provider_message_id, ''), created_at, updated_at
		FROM email_drafts`
}

func scanDraft(row interface{ Scan(...any) error }) (EmailDraft, error) {
	var draft EmailDraft
	err := row.Scan(
		&draft.ID, &draft.OwnerID, &draft.FromAliasID, &draft.FromAddress, &draft.ToJSON, &draft.CcJSON, &draft.BccJSON,
		&draft.Subject, &draft.TextBody, &draft.HTMLBody, &draft.ReplyToMessageID, &draft.SourceKind, &draft.SourceRef,
		&draft.Status, &draft.Version, &draft.VersionHash, &draft.SentMessageID, &draft.ProviderMessageID,
		&draft.CreatedAt, &draft.UpdatedAt,
	)
	if err != nil {
		return EmailDraft{}, err
	}
	return draft, nil
}

func summarizeDraft(draft EmailDraft) draftResponse {
	return draftResponse{
		ID:                draft.ID,
		Status:            draft.Status,
		Version:           draft.Version,
		VersionHash:       draft.VersionHash,
		FromAddress:       draft.FromAddress,
		ToAddresses:       decodeAddressJSON(draft.ToJSON),
		CcAddresses:       decodeAddressJSON(draft.CcJSON),
		BccAddresses:      decodeAddressJSON(draft.BccJSON),
		Subject:           draft.Subject,
		TextBody:          draft.TextBody,
		HTMLBody:          draft.HTMLBody,
		ReplyToMessageID:  draft.ReplyToMessageID,
		SourceKind:        draft.SourceKind,
		SourceRef:         draft.SourceRef,
		SentMessageID:     draft.SentMessageID,
		ProviderMessageID: draft.ProviderMessageID,
		CreatedAt:         draft.CreatedAt,
		UpdatedAt:         draft.UpdatedAt,
	}
}

func (d EmailDraft) toSendRequest() sendEmailRequest {
	return sendEmailRequest{
		FromAddress:      d.FromAddress,
		ToAddresses:      decodeAddressJSON(d.ToJSON),
		CcAddresses:      decodeAddressJSON(d.CcJSON),
		BccAddresses:     decodeAddressJSON(d.BccJSON),
		Subject:          d.Subject,
		TextBody:         d.TextBody,
		HTMLBody:         d.HTMLBody,
		ReplyToMessageID: d.ReplyToMessageID,
	}
}

func decodeAddressJSON(raw string) []string {
	var out []string
	_ = json.Unmarshal([]byte(strings.TrimSpace(raw)), &out)
	if out == nil {
		return []string{}
	}
	return out
}

func draftVersionHash(draft EmailDraft) string {
	payload, _ := json.Marshal(map[string]any{
		"from":                draft.FromAddress,
		"to":                  decodeAddressJSON(draft.ToJSON),
		"cc":                  decodeAddressJSON(draft.CcJSON),
		"bcc":                 decodeAddressJSON(draft.BccJSON),
		"subject":             draft.Subject,
		"text_body":           draft.TextBody,
		"html_body":           draft.HTMLBody,
		"reply_to_message_id": draft.ReplyToMessageID,
		"source_kind":         draft.SourceKind,
		"source_ref":          draft.SourceRef,
		"version":             draft.Version,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}
