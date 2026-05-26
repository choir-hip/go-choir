package maild

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

type sendEmailRequest struct {
	FromAddress      string   `json:"from_address"`
	ToAddresses      []string `json:"to_addresses"`
	CcAddresses      []string `json:"cc_addresses,omitempty"`
	BccAddresses     []string `json:"bcc_addresses,omitempty"`
	Subject          string   `json:"subject"`
	TextBody         string   `json:"text_body"`
	HTMLBody         string   `json:"html_body,omitempty"`
	ReplyToMessageID string   `json:"reply_to_message_id,omitempty"`
}

type sendEmailResponse struct {
	ID                string `json:"id"`
	ProviderMessageID string `json:"provider_message_id"`
	Status            string `json:"status"`
}

// HandleSend sends owner-authored outbound mail through Resend and records the
// sent row in maild. It does not accept agent-originated sends.
func (h *Handler) HandleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	ownerID := strings.TrimSpace(r.Header.Get("X-Authenticated-User"))
	if ownerID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}
	var in sendEmailRequest
	if err := decodeJSON(r, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
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
	payload, err := buildResendSendRequest(in, alias)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := h.applyReplyHeaders(r.Context(), ownerID, in.ReplyToMessageID, &payload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "reply target is not owned by current user"})
			return
		}
		if errors.Is(err, errMissingReplyMessageID) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "reply target is missing message id"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load reply target"})
		return
	}
	sent, err := h.resend.sendEmail(r.Context(), payload)
	if err != nil {
		var providerErr *resendHTTPError
		if errors.As(err, &providerErr) {
			log.Printf("maild: outbound send provider failure owner=%s alias=%s status=%d detail=%q", ownerID, alias.ID, providerErr.StatusCode, providerErr.Detail)
		} else {
			log.Printf("maild: outbound send failure owner=%s alias=%s: %v", ownerID, alias.ID, err)
		}
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to send email"})
		return
	}
	msg, err := h.store.StoreOutboundMessage(r.Context(), ownerID, alias, sent.ID, in)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to store sent email"})
		return
	}
	writeJSON(w, http.StatusAccepted, sendEmailResponse{ID: msg.ID, ProviderMessageID: sent.ID, Status: "sent"})
}

func (h *Handler) resolveOwnedFromAlias(ctx context.Context, ownerID, fromAddress string) (EmailAlias, error) {
	localPart, domain, ok := splitEmailAddress(fromAddress)
	if !ok {
		return EmailAlias{}, sql.ErrNoRows
	}
	alias, err := h.store.ResolveAlias(ctx, domain, localPart)
	if err != nil {
		return EmailAlias{}, err
	}
	if alias.TargetID != ownerID {
		return EmailAlias{}, sql.ErrNoRows
	}
	return alias, nil
}

var errMissingReplyMessageID = errors.New("reply target is missing message id")

func (h *Handler) applyReplyHeaders(ctx context.Context, ownerID, replyToMessageID string, payload *resendSendRequest) error {
	replyToMessageID = strings.TrimSpace(replyToMessageID)
	if replyToMessageID == "" {
		return nil
	}
	msg, err := h.store.GetMessage(ctx, ownerID, replyToMessageID)
	if err != nil {
		return err
	}
	rfcMessageID := extractRFCMessageID(msg.RawHeadersJSON)
	if rfcMessageID == "" {
		return errMissingReplyMessageID
	}
	if payload.Headers == nil {
		payload.Headers = make(map[string]any)
	}
	payload.Headers["In-Reply-To"] = rfcMessageID
	payload.Headers["References"] = rfcMessageID
	return nil
}

func extractRFCMessageID(rawHeadersJSON string) string {
	if strings.TrimSpace(rawHeadersJSON) == "" {
		return ""
	}
	var headers map[string]any
	if err := json.Unmarshal([]byte(rawHeadersJSON), &headers); err != nil {
		return ""
	}
	for _, key := range []string{"message_id", "message-id", "Message-ID", "Message-Id"} {
		if value, ok := headers[key]; ok {
			if id := strings.TrimSpace(fmt.Sprint(value)); id != "" {
				return id
			}
		}
	}
	for key, value := range headers {
		if strings.EqualFold(key, "message-id") || strings.EqualFold(key, "message_id") {
			if id := strings.TrimSpace(fmt.Sprint(value)); id != "" {
				return id
			}
		}
	}
	return ""
}

func buildResendSendRequest(in sendEmailRequest, alias EmailAlias) (resendSendRequest, error) {
	from := strings.TrimSpace(in.FromAddress)
	if from == "" {
		from = alias.LocalPart + "@" + alias.Domain
	}
	to := cleanAddresses(in.ToAddresses)
	if len(to) == 0 {
		return resendSendRequest{}, fmt.Errorf("at least one recipient is required")
	}
	subject := strings.TrimSpace(in.Subject)
	if subject == "" {
		subject = "(no subject)"
	}
	text := strings.TrimSpace(in.TextBody)
	html := strings.TrimSpace(in.HTMLBody)
	if text == "" && html == "" {
		return resendSendRequest{}, fmt.Errorf("message body is required")
	}
	return resendSendRequest{
		From:    from,
		To:      to,
		Cc:      cleanAddresses(in.CcAddresses),
		Bcc:     cleanAddresses(in.BccAddresses),
		Subject: subject,
		Text:    text,
		HTML:    html,
		Headers: map[string]any{
			"X-Choir-Maild": "v0-owner-send",
		},
	}, nil
}

func cleanAddresses(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

// StoreOutboundMessage records a user-composed sent message.
func (s *Store) StoreOutboundMessage(ctx context.Context, ownerID string, alias EmailAlias, providerMessageID string, in sendEmailRequest) (EmailMessage, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	messageID := messageRowID("outbound:" + providerMessageID)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return EmailMessage{}, fmt.Errorf("begin outbound message tx: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()
	if _, err := tx.ExecContext(ctx, `INSERT INTO email_messages (
		id, provider, provider_message_id, direction, mailbox_owner_id, alias_id,
		from_address, subject, text_body, html_body, trust_status, sent_at, created_at
	) VALUES (?, ?, ?, 'outbound', ?, ?, ?, ?, ?, ?, 'trusted', ?, ?)`,
		messageID,
		providerResend,
		providerMessageID,
		ownerID,
		alias.ID,
		strings.TrimSpace(in.FromAddress),
		strings.TrimSpace(in.Subject),
		nullString(in.TextBody),
		nullString(in.HTMLBody),
		now,
		now,
	); err != nil {
		return EmailMessage{}, fmt.Errorf("insert outbound message: %w", err)
	}
	if err := insertRecipients(ctx, tx, messageID, "to", in.ToAddresses); err != nil {
		return EmailMessage{}, err
	}
	if err := insertRecipients(ctx, tx, messageID, "cc", in.CcAddresses); err != nil {
		return EmailMessage{}, err
	}
	if err := insertRecipients(ctx, tx, messageID, "bcc", in.BccAddresses); err != nil {
		return EmailMessage{}, err
	}
	if err := tx.Commit(); err != nil {
		return EmailMessage{}, fmt.Errorf("commit outbound message tx: %w", err)
	}
	tx = nil
	return s.GetMessage(ctx, ownerID, messageID)
}
