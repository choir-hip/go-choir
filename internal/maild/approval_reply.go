package maild

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

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
		return fmt.Errorf("approval token is not active")
	}
	if expiresAt, err := time.Parse(time.RFC3339Nano, token.ExpiresAt); err == nil && time.Now().UTC().After(expiresAt) {
		return fmt.Errorf("approval token expired")
	}
	fromAddress, _ := parseSender(email.From, email.Headers["from"])
	if !strings.EqualFold(strings.TrimSpace(fromAddress), token.ApprovalEmail) {
		return fmt.Errorf("approval reply sender mismatch")
	}
	command, editText := parseApprovalReplyCommand(email.Text)
	draft, err := h.store.GetDraft(ctx, token.OwnerID, token.DraftID)
	if err != nil {
		return err
	}
	if draft.VersionHash != token.VersionHash || draft.Version != token.Version {
		_ = h.store.UseDraftApprovalToken(ctx, token.ID, "superseded")
		return fmt.Errorf("approval token draft version mismatch")
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
		if _, err := h.store.UpdateDraftTextFromApprovalEdit(ctx, draft, editText); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("unsupported approval reply command")
	}
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
