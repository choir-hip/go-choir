package maild

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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
		From:    canonicalAliasAddress(alias),
		To:      to,
		Cc:      cleanAddresses(in.CcAddresses),
		Bcc:     cleanAddresses(in.BccAddresses),
		Subject: subject,
		Text:    text,
		HTML:    html,
		Headers: map[string]any{
			"X-Choir-Maild": "v0-approved-draft-send",
		},
	}, nil
}

func canonicalAliasAddress(alias EmailAlias) string {
	return strings.ToLower(strings.TrimSpace(alias.LocalPart)) + "@" + strings.ToLower(strings.TrimSpace(alias.Domain))
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
		canonicalAliasAddress(alias),
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
