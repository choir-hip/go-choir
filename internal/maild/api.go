package maild

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type messageListResponse struct {
	Messages []messageSummary `json:"messages"`
}

type messageSummary struct {
	ID             string `json:"id"`
	Direction      string `json:"direction"`
	FromAddress    string `json:"from_address"`
	FromDisplay    string `json:"from_display,omitempty"`
	Subject        string `json:"subject"`
	Snippet        string `json:"snippet,omitempty"`
	TrustStatus    string `json:"trust_status"`
	ReadAt         string `json:"read_at,omitempty"`
	ReceivedAt     string `json:"received_at,omitempty"`
	SentAt         string `json:"sent_at,omitempty"`
	CreatedAt      string `json:"created_at"`
	HasAttachments bool   `json:"has_attachments,omitempty"`
}

type messageDetailResponse struct {
	Message     messageSummary       `json:"message"`
	TextBody    string               `json:"text_body,omitempty"`
	HTMLBody    string               `json:"html_body,omitempty"`
	Attachments []attachmentResponse `json:"attachments,omitempty"`
}

type attachmentResponse struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	Status      string `json:"status"`
}

type sourcePacketResponse struct {
	SourcePacketID string `json:"source_packet_id"`
	MessageID      string `json:"message_id"`
	TrustLabel     string `json:"trust_label"`
	FromAddress    string `json:"from_address,omitempty"`
	Subject        string `json:"subject,omitempty"`
	Snippet        string `json:"snippet,omitempty"`
}

// HandleMessages handles /api/email/messages and /api/email/messages/*.
func (h *Handler) HandleMessages(w http.ResponseWriter, r *http.Request) {
	ownerID := strings.TrimSpace(r.Header.Get("X-Authenticated-User"))
	if ownerID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}
	if r.URL.Path == "/api/email/messages" {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		h.handleMessageList(w, r, ownerID)
		return
	}

	const prefix = "/api/email/messages/"
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/")
	if rest == "" || rest == r.URL.Path {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	parts := strings.Split(rest, "/")
	messageID := parts[0]
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		h.handleMessageDetail(w, r, ownerID, messageID)
		return
	}
	if len(parts) == 2 && parts[1] == "source-packet" {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		h.handleMessageSourcePacket(w, r, ownerID, messageID)
		return
	}
	if len(parts) == 2 && parts[1] == "read" {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		h.handleMessageRead(w, r, ownerID, messageID)
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
}

func (h *Handler) handleMessageList(w http.ResponseWriter, r *http.Request, ownerID string) {
	messages, err := h.store.ListMessages(r.Context(), ownerID, r.URL.Query().Get("folder"), 50)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	out := make([]messageSummary, 0, len(messages))
	for _, msg := range messages {
		out = append(out, summarizeMessage(msg))
	}
	writeJSON(w, http.StatusOK, messageListResponse{Messages: out})
}

func (h *Handler) handleMessageDetail(w http.ResponseWriter, r *http.Request, ownerID, messageID string) {
	msg, err := h.store.GetMessage(r.Context(), ownerID, messageID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	attachments, err := h.store.ListAttachments(r.Context(), ownerID, messageID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load attachments"})
		return
	}
	outAttachments := make([]attachmentResponse, 0, len(attachments))
	for _, attachment := range attachments {
		outAttachments = append(outAttachments, attachmentResponse{
			ID:          attachment.ID,
			Filename:    attachment.Filename,
			ContentType: attachment.ContentType,
			SizeBytes:   attachment.SizeBytes,
			Status:      attachment.Status,
		})
	}
	summary := summarizeMessage(msg)
	summary.HasAttachments = len(outAttachments) > 0
	writeJSON(w, http.StatusOK, messageDetailResponse{
		Message:     summary,
		TextBody:    msg.TextBody,
		HTMLBody:    msg.HTMLBody,
		Attachments: outAttachments,
	})
}

func (h *Handler) handleMessageSourcePacket(w http.ResponseWriter, r *http.Request, ownerID, messageID string) {
	packet, msg, err := h.store.GetSourcePacketForMessage(r.Context(), ownerID, messageID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sourcePacketResponse{
		SourcePacketID: packet.ID,
		MessageID:      msg.ID,
		TrustLabel:     packet.TrustLabel,
		FromAddress:    msg.FromAddress,
		Subject:        msg.Subject,
		Snippet:        snippet(msg.TextBody),
	})
}

func (h *Handler) handleMessageRead(w http.ResponseWriter, r *http.Request, ownerID, messageID string) {
	if err := h.store.MarkMessageRead(r.Context(), ownerID, messageID, time.Now()); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "read"})
}

func summarizeMessage(msg EmailMessage) messageSummary {
	return messageSummary{
		ID:          msg.ID,
		Direction:   msg.Direction,
		FromAddress: msg.FromAddress,
		FromDisplay: msg.FromDisplay,
		Subject:     msg.Subject,
		Snippet:     snippet(msg.TextBody),
		TrustStatus: msg.TrustStatus,
		ReadAt:      msg.ReadAt,
		ReceivedAt:  msg.ReceivedAt,
		SentAt:      msg.SentAt,
		CreatedAt:   msg.CreatedAt,
	}
}

func snippet(text string) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\n", " "))
	if len(text) <= 120 {
		return text
	}
	return text[:117] + "..."
}

func writeStoreError(w http.ResponseWriter, err error) {
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "mail store error"})
}

func decodeJSON(r *http.Request, dst any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}
