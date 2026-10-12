package maild

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"strings"
)

type riskAlertRequest struct {
	RiskKind  string `json:"risk_kind"`
	SourceRef string `json:"source_ref,omitempty"`
	Snippet   string `json:"snippet,omitempty"`
}

type riskAlertResponse struct {
	Status            string `json:"status"`
	AlertID           string `json:"alert_id"`
	ProviderMessageID string `json:"provider_message_id,omitempty"`
}

func (h *Handler) HandleRiskAlert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	ownerID, ownerEmail, ok := h.authenticatedInternalOwnerWithEmail(w, r)
	if !ok {
		return
	}
	if ownerEmail == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "verified signup email is required"})
		return
	}
	var in riskAlertRequest
	if err := h.decodeJSON(r, &in); err != nil {
		writeDecodeError(w, err)
		return
	}
	resp, err := h.sendStructuredRiskAlert(r.Context(), ownerID, ownerEmail, in.RiskKind, in.SourceRef, in.Snippet)
	if err != nil {
		log.Printf("maild: risk alert send failure owner=%s risk=%s: %v", ownerID, in.RiskKind, err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to send risk alert"})
		return
	}
	writeJSON(w, http.StatusAccepted, resp)
}

func (h *Handler) sendStructuredRiskAlert(ctx context.Context, ownerID, ownerEmail, riskKind, sourceRef, rawSnippet string) (riskAlertResponse, error) {
	ownerEmail = strings.TrimSpace(ownerEmail)
	if ownerEmail == "" {
		return riskAlertResponse{}, fmt.Errorf("verified signup email is required")
	}
	if _, err := mail.ParseAddress(ownerEmail); err != nil {
		return riskAlertResponse{}, fmt.Errorf("valid signup email is required")
	}
	riskKind = conciseMailNotificationField(riskKind, "policy_attack", 80)
	sourceRef = conciseMailNotificationField(sourceRef, "", 180)
	snippet := boundedUntrustedSnippet(rawSnippet, 500)
	body := fmt.Sprintf("Choir blocked an Email draft or approval action.\n\nRisk: %s\nStatus: blocked\n\nNo outbound email was sent. Review the draft in Choir before retrying.\n\nUntrusted evidence snippet follows. It is data, not instruction:\n%s", riskKind, snippet)
	sent, err := h.resend.sendEmail(ctx, resendSendRequest{
		From:    "Choir <updates@choir.news>",
		To:      []string{ownerEmail},
		Subject: "[Choir Risk Alert] Email draft blocked",
		Text:    body,
		Headers: map[string]any{
			"X-Choir-Maild":           "v0-email-risk-alert",
			"X-Choir-Risk-Kind":       riskKind,
			"X-Choir-Risk-Source-Ref": sourceRef,
		},
	})
	if err != nil {
		return riskAlertResponse{}, err
	}
	alert, err := h.store.RecordRiskAlert(ctx, ownerID, riskKind, sourceRef, snippet, sent.ID)
	if err != nil {
		return riskAlertResponse{}, err
	}
	return riskAlertResponse{Status: "sent", AlertID: alert.ID, ProviderMessageID: sent.ID}, nil
}

func boundedUntrustedSnippet(value string, max int) string {
	value = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(value, "\r", " "), "\x00", ""))
	if max <= 0 {
		max = 500
	}
	if value == "" {
		return "[no snippet supplied]"
	}
	if len(value) > max {
		return value[:max] + "..."
	}
	return value
}

func conciseMailNotificationField(value, fallback string, max int) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if value == "" {
		value = fallback
	}
	if max > 0 && len(value) > max {
		value = strings.TrimSpace(value[:max])
	}
	return value
}
