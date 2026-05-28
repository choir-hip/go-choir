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
