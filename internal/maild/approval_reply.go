package maild

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
)

var errApprovalReplyRejected = errors.New("approval reply rejected")

func approvalReplyToken(recipients []string, primaryDomain string) (string, bool) {
	wantDomain := strings.ToLower(strings.TrimSpace(primaryDomain))
	for _, recipient := range recipients {
		local, domain, ok := splitEmailAddress(recipient)
		if !ok || strings.ToLower(domain) != wantDomain {
			continue
		}
		local = strings.ToLower(strings.TrimSpace(local))
		if strings.HasPrefix(local, "approve+") && len(local) > len("approve+") {
			return strings.TrimSpace(local[len("approve+"):]), true
		}
	}
	return "", false
}

func (h *Handler) processApprovalReply(ctx context.Context, providerEventID string, email resendReceivedEmail, tokenValue string) error {
	token, err := h.store.GetDraftApprovalToken(ctx, tokenValue)
	if err != nil {
		return err
	}
	if token.Status != "active" {
		return h.rejectApprovalReply(ctx, token, "approval_token_not_active", "approval token is not active", email)
	}
	if expiresAt, err := time.Parse(time.RFC3339Nano, token.ExpiresAt); err == nil && time.Now().UTC().After(expiresAt) {
		return h.rejectApprovalReply(ctx, token, "approval_token_expired", "approval token expired", email)
	}
	fromAddress, _ := parseSender(email.From, email.Headers["from"])
	if !strings.EqualFold(strings.TrimSpace(fromAddress), token.ApprovalEmail) {
		return h.rejectApprovalReply(ctx, token, "approval_sender_mismatch", "approval reply sender mismatch", email)
	}
	command, editText := parseApprovalReplyCommand(email.Text)
	draft, err := h.store.GetDraft(ctx, token.OwnerID, token.DraftID)
	if err != nil {
		return err
	}
	if draft.Status == "sent" {
		if err := h.sendApprovalReplyRiskAlert(ctx, token, "approval_draft_already_sent", "approval reply targeted an already-sent draft", email); err != nil {
			log.Printf("maild: approval reply risk alert failed owner=%s draft=%s risk=approval_draft_already_sent: %v", token.OwnerID, token.DraftID, err)
		}
		_ = h.store.UseDraftApprovalToken(ctx, token.ID, "stale_sent")
		return fmt.Errorf("%w: approval reply targeted an already-sent draft", errApprovalReplyRejected)
	}
	if draft.VersionHash != token.VersionHash || draft.Version != token.Version {
		if err := h.sendApprovalReplyRiskAlert(ctx, token, "approval_draft_version_mismatch", "approval token draft version mismatch", email); err != nil {
			log.Printf("maild: approval reply risk alert failed owner=%s draft=%s risk=approval_draft_version_mismatch: %v", token.OwnerID, token.DraftID, err)
		}
		_ = h.store.UseDraftApprovalToken(ctx, token.ID, "superseded")
		return fmt.Errorf("%w: approval token draft version mismatch", errApprovalReplyRejected)
	}
	switch command {
	case "approve":
		resp, err := h.sendApprovedDraft(ctx, token.OwnerID, token.DraftID, token.VersionHash, "email_reply_approved", email.ID)
		if err != nil {
			return err
		}
		if err := h.store.UseDraftApprovalToken(ctx, token.ID, "approved"); err != nil && err != sql.ErrNoRows {
			return err
		}
		_ = providerEventID
		_ = resp
		return nil
	case "reject":
		if _, err := h.store.RecordDraftApprovalEvent(ctx, draft, "email_reply_rejected", email.ID); err != nil {
			return err
		}
		return h.store.UseDraftApprovalToken(ctx, token.ID, "rejected")
	case "edit":
		if _, err := h.store.RecordDraftApprovalEvent(ctx, draft, "email_reply_edit_requested", email.ID); err != nil {
			return err
		}
		if err := h.store.UseDraftApprovalToken(ctx, token.ID, "edited"); err != nil {
			return err
		}
		updated, err := h.store.UpdateDraftTextFromApprovalEdit(ctx, draft, editText)
		if err != nil {
			return err
		}
		if _, err := h.sendDraftApprovalEmail(ctx, updated, token.ApprovalEmail); err != nil {
			log.Printf("maild: edited draft approval email failed owner=%s draft=%s: %v", token.OwnerID, token.DraftID, err)
		}
		return nil
	default:
		return h.rejectApprovalReply(ctx, token, "approval_reply_unsupported_command", "unsupported approval reply command", email)
	}
}

func (h *Handler) rejectApprovalReply(ctx context.Context, token EmailApprovalToken, riskKind, reason string, email resendReceivedEmail) error {
	if err := h.sendApprovalReplyRiskAlert(ctx, token, riskKind, reason, email); err != nil {
		log.Printf("maild: approval reply risk alert failed owner=%s draft=%s risk=%s: %v", token.OwnerID, token.DraftID, riskKind, err)
	}
	return fmt.Errorf("%w: %s", errApprovalReplyRejected, reason)
}

func (h *Handler) sendApprovalReplyRiskAlert(ctx context.Context, token EmailApprovalToken, riskKind, reason string, email resendReceivedEmail) error {
	if strings.TrimSpace(token.ApprovalEmail) == "" {
		return fmt.Errorf("approval email is empty")
	}
	sourceRef := "approval_reply:" + token.DraftID
	snippet := approvalReplyRiskSnippet(reason, email)
	_, err := h.sendStructuredRiskAlert(ctx, token.OwnerID, token.ApprovalEmail, riskKind, sourceRef, snippet)
	return err
}

func approvalReplyRiskSnippet(reason string, email resendReceivedEmail) string {
	return fmt.Sprintf("Reason: %s\nFrom: %s\nTo: %s\nText: %s",
		strings.TrimSpace(reason),
		strings.TrimSpace(email.From),
		strings.Join(email.To, ", "),
		strings.TrimSpace(email.Text),
	)
}

func parseApprovalReplyCommand(text string) (string, string) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		switch {
		case lower == "approve" || strings.HasPrefix(lower, "approve "):
			return "approve", ""
		case lower == "reject" || strings.HasPrefix(lower, "reject "):
			return "reject", ""
		case lower == "deny" || strings.HasPrefix(lower, "deny "):
			return "reject", ""
		case strings.HasPrefix(lower, "edit:"):
			return "edit", strings.TrimSpace(trimmed[len("edit:"):])
		case strings.HasPrefix(lower, "edit "):
			return "edit", strings.TrimSpace(trimmed[len("edit "):])
		default:
			return "unknown", ""
		}
	}
	return "unknown", ""
}
