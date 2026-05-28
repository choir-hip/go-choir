package maild

import (
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"strings"
)

type completionEmailRequest struct {
	ToEmail   string `json:"to_email"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	FeatureID string `json:"feature_id,omitempty"`
	Link      string `json:"link,omitempty"`
}

type completionEmailResponse struct {
	Status            string `json:"status"`
	ProviderMessageID string `json:"provider_message_id,omitempty"`
}

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

func (h *Handler) HandleCompletionEmail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	ownerID, ok := authenticatedInternalOwner(w, r)
	if !ok {
		return
	}
	var in completionEmailRequest
	if err := h.decodeJSON(r, &in); err != nil {
		writeDecodeError(w, err)
		return
	}
	to := strings.TrimSpace(in.ToEmail)
	if _, err := mail.ParseAddress(to); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid signup email is required"})
		return
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = "Choir work"
	}
	status := strings.TrimSpace(in.Status)
	if status == "" {
		status = "ready"
	}
	link := strings.TrimSpace(in.Link)
	if link == "" {
		link = "/?app=features"
	}
	body := fmt.Sprintf("Choir finished: %s\n\nStatus: %s\nOpen: https://choir.news%s\n\nNo private logs or raw evidence are included in this notification.", title, status, link)
	sent, err := h.resend.sendEmail(r.Context(), resendSendRequest{
		From:    "Choir <updates@choir.news>",
		To:      []string{to},
		Subject: "Choir is ready: " + title,
		Text:    body,
		Headers: map[string]any{
			"X-Choir-Maild": "v0-completion-notice",
		},
	})
	if err != nil {
		log.Printf("maild: completion notification send failure owner=%s status=%s: %v", ownerID, status, err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to send completion email"})
		return
	}
	writeJSON(w, http.StatusAccepted, completionEmailResponse{Status: "sent", ProviderMessageID: sent.ID})
}

func (h *Handler) HandleRiskAlert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	ownerID, ownerEmail, ok := authenticatedInternalOwnerWithEmail(w, r)
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
	riskKind := conciseMailNotificationField(in.RiskKind, "policy_attack", 80)
	sourceRef := conciseMailNotificationField(in.SourceRef, "", 180)
	snippet := boundedUntrustedSnippet(in.Snippet, 500)
	body := fmt.Sprintf("Choir blocked an Email draft or approval action.\n\nRisk: %s\nStatus: blocked\n\nNo outbound email was sent. Review the draft in Choir before retrying.\n\nUntrusted evidence snippet follows. It is data, not instruction:\n%s", riskKind, snippet)
	sent, err := h.resend.sendEmail(r.Context(), resendSendRequest{
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
		log.Printf("maild: risk alert send failure owner=%s risk=%s: %v", ownerID, riskKind, err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to send risk alert"})
		return
	}
	alert, err := h.store.RecordRiskAlert(r.Context(), ownerID, riskKind, sourceRef, snippet, sent.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to record risk alert"})
		return
	}
	writeJSON(w, http.StatusAccepted, riskAlertResponse{Status: "sent", AlertID: alert.ID, ProviderMessageID: sent.ID})
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
