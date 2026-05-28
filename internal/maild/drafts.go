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
	ownerID, ok := authenticatedInternalOwner(w, r)
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
	alias, err := h.resolveOwnedFromAlias(r.Context(), ownerID, in.FromAddress)
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
	draft, err := h.store.GetDraft(r.Context(), ownerID, draftID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if draft.Status == "sent" {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "draft already sent"})
		return
	}
	if strings.TrimSpace(in.VersionHash) == "" || strings.TrimSpace(in.VersionHash) != draft.VersionHash {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "draft version changed; reopen the draft before approving"})
		return
	}
	alias, err := h.resolveOwnedFromAlias(r.Context(), ownerID, draft.FromAddress)
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "from address is not owned by current user"})
		return
	}
	approvalEventID, err := h.store.RecordDraftApprovalEvent(r.Context(), draft, "owner_click_approved", "")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to record approval"})
		return
	}
	sendReq := draft.toSendRequest()
	payload, err := buildResendSendRequest(sendReq, alias)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	payload.Headers["X-Choir-Maild"] = "v0-approved-draft-send"
	payload.Headers["X-Choir-Email-Draft-ID"] = draft.ID
	payload.Headers["X-Choir-Email-Draft-Version-Hash"] = draft.VersionHash
	if err := h.applyReplyHeaders(r.Context(), ownerID, draft.ReplyToMessageID, &payload); err != nil {
		if err == sql.ErrNoRows {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "reply target is not owned by current user"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "reply target is invalid"})
		return
	}
	sent, err := h.resend.sendEmail(r.Context(), payload)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to send email"})
		return
	}
	msg, err := h.store.StoreOutboundMessage(r.Context(), ownerID, alias, sent.ID, sendReq)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to store sent email"})
		return
	}
	updated, err := h.store.MarkDraftSent(r.Context(), ownerID, draftID, msg.ID, sent.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to mark draft sent"})
		return
	}
	writeJSON(w, http.StatusAccepted, sendDraftResponse{
		Draft:             summarizeDraft(updated),
		MessageID:         msg.ID,
		ProviderMessageID: sent.ID,
		ApprovalEventID:   approvalEventID,
		Status:            "sent",
	})
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
	result, err := s.db.ExecContext(ctx, `UPDATE email_drafts
		SET status = 'sent', sent_message_id = ?, provider_message_id = ?, updated_at = ?
		WHERE owner_id = ? AND id = ? AND status <> 'sent'`,
		messageID, providerMessageID, now, ownerID, draftID)
	if err != nil {
		return EmailDraft{}, fmt.Errorf("mark draft sent: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return EmailDraft{}, sql.ErrNoRows
	}
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
