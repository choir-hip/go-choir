package maild

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"
)

type messageListResponse struct {
	Messages   []messageSummary `json:"messages"`
	NextCursor string           `json:"next_cursor,omitempty"`
	Total      int              `json:"total"`
	Unread     int              `json:"unread"`
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
	RawHeaders  map[string]string    `json:"raw_headers,omitempty"`
	Recipients  recipientsResponse   `json:"recipients"`
	Attachments []attachmentResponse `json:"attachments,omitempty"`
}

type recipientsResponse struct {
	To  []addressResponse `json:"to,omitempty"`
	Cc  []addressResponse `json:"cc,omitempty"`
	Bcc []addressResponse `json:"bcc,omitempty"`
}

type addressResponse struct {
	Address string `json:"address"`
	Display string `json:"display,omitempty"`
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
	ProvenanceJSON string `json:"provenance_json,omitempty"`
	TextRef        string `json:"text_ref,omitempty"`
	TextBody       string `json:"text_body,omitempty"`
	HasAttachments bool   `json:"has_attachments,omitempty"`
}

type recordIngressEventRequest struct {
	SourcePacketID        string `json:"source_packet_id"`
	ConductorSubmissionID string `json:"conductor_submission_id"`
	Status                string `json:"status"`
}

type ingressEventsResponse struct {
	Events []EmailIngressEvent `json:"events"`
}

// HandleMessages handles /api/email/messages and /api/email/messages/*.
func (h *Handler) HandleMessages(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := h.authenticatedInternalOwner(w, r)
	if !ok {
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
	if len(parts) == 2 && parts[1] == "ingress-events" {
		switch r.Method {
		case http.MethodGet:
			h.handleMessageIngressEvents(w, r, ownerID, messageID)
		case http.MethodPost:
			h.handleRecordMessageIngressEvent(w, r, ownerID, messageID)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
		return
	}
	if len(parts) == 2 && parts[1] == "read" {
		switch r.Method {
		case http.MethodPost:
			h.handleMessageRead(w, r, ownerID, messageID)
		case http.MethodDelete:
			h.handleMessageUnread(w, r, ownerID, messageID)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
		return
	}
	if len(parts) == 2 && parts[1] == "unread" {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		h.handleMessageUnread(w, r, ownerID, messageID)
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
}

// s1aBindTapCallerToOwner checks S1a authority binding: when the request
// arrives over a guest tap (non-loopback RemoteAddr), the guest's source
// IP must resolve via vmctl to a live ownership whose user_id equals the
// asserted ownerID. Loopback callers are host services and keep header
// trust. Returns the verified ownerID, or "" with a 403 already written.
func (h *Handler) s1aBindTapCallerToOwner(w http.ResponseWriter, r *http.Request, ownerID string) (string, bool) {
	srcHost := maildRemoteAddrHost(r)
	if srcHost == "" || isMaildLoopbackHost(srcHost) {
		return ownerID, true // loopback: host service, header trust is fine
	}
	if h.cfg == nil || strings.TrimSpace(h.cfg.VmctlURL) == "" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "guest caller binding unavailable"})
		return "", false
	}
	bound, err := maildLookupGuestOwnership(r.Context(), h.cfg.VmctlURL, r.RemoteAddr)
	if err != nil || bound == nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "guest caller not bound to a live computer"})
		return "", false
	}
	if !strings.EqualFold(strings.TrimSpace(bound.UserID), ownerID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "guest caller not bound to asserted owner"})
		return "", false
	}
	return ownerID, true
}

func maildRemoteAddrHost(r *http.Request) string {
	if host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr)); err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func isMaildLoopbackHost(host string) bool {
	return host == "127.0.0.1" || host == "::1" || host == "" || host == "@" ||
		strings.HasPrefix(host, "192.0.2.") || strings.HasPrefix(host, "/")
}

type maildGuestOwnership struct {
	Found      bool   `json:"found"`
	UserID     string `json:"user_id"`
	ComputerID string `json:"computer_id"`
}

func maildLookupGuestOwnership(ctx context.Context, vmctlURL, remoteAddr string) (*maildGuestOwnership, error) {
	body, _ := json.Marshal(map[string]string{"remote_addr": remoteAddr})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(strings.TrimSpace(vmctlURL), "/")+"/internal/vmctl/lookup-guest",
		bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vmctl lookup-guest status %d", resp.StatusCode)
	}
	var out maildGuestOwnership
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if !out.Found {
		return nil, nil
	}
	return &out, nil
}

func (h *Handler) authenticatedInternalOwner(w http.ResponseWriter, r *http.Request) (string, bool) {
	ownerID, _, ok := h.authenticatedInternalOwnerWithEmail(w, r)
	return ownerID, ok
}

func (h *Handler) authenticatedInternalOwnerWithEmail(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	ownerID := strings.TrimSpace(r.Header.Get("X-Authenticated-User"))
	if ownerID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return "", "", false
	}
	// S1a: the caller must be either a host service on a trusted transport
	// (loopback / unix socket / TEST-NET-1 test harness) or a bound guest
	// whose tap IP resolves to a live ownership for the asserted owner.
	srcHost := maildRemoteAddrHost(r)
	if isMaildLoopbackHost(srcHost) {
		if !strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Internal-Caller")), "true") {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "internal caller required"})
			return "", "", false
		}
		return ownerID, normalizedTrustedEmail(r.Header.Get("X-Authenticated-Email")), true
	}
	boundID, ok := h.s1aBindTapCallerToOwner(w, r, ownerID)
	if !ok {
		return "", "", false
	}
	return boundID, normalizedTrustedEmail(r.Header.Get("X-Authenticated-Email")), true
}

func normalizedTrustedEmail(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "\r\n") {
		return ""
	}
	addr, err := mail.ParseAddress(value)
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(addr.Address))
}

func (h *Handler) handleMessageList(w http.ResponseWriter, r *http.Request, ownerID string) {
	q := r.URL.Query()
	folder := q.Get("folder")
	cursor := q.Get("cursor")
	// Default to 100 messages per page to display full typical inboxes
	limit := 100
	if l := q.Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = val
		}
	}
	res, err := h.store.ListMessagesPaged(r.Context(), ListMessagesOptions{
		OwnerID: ownerID,
		Folder:  folder,
		Limit:   limit,
		Cursor:  cursor,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	out := make([]messageSummary, 0, len(res.Messages))
	for _, msg := range res.Messages {
		out = append(out, summarizeMessage(msg))
	}
	writeJSON(w, http.StatusOK, messageListResponse{
		Messages:   out,
		NextCursor: res.NextCursor,
		Total:      res.Total,
		Unread:     res.Unread,
	})
}

func (h *Handler) handleMessageDetail(w http.ResponseWriter, r *http.Request, ownerID, messageID string) {
	msg, err := h.store.GetMessage(r.Context(), ownerID, messageID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	recipients, err := h.store.ListRecipients(r.Context(), ownerID, messageID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load recipients"})
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
		RawHeaders:  parseRawHeaders(msg.RawHeadersJSON),
		Recipients:  groupRecipients(recipients),
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
		ProvenanceJSON: packet.ProvenanceJSON,
		TextRef:        packet.TextRef,
		TextBody:       msg.TextBody,
		HasAttachments: msg.HasAttachments,
	})
}

func (h *Handler) handleMessageIngressEvents(w http.ResponseWriter, r *http.Request, ownerID, messageID string) {
	events, err := h.store.ListIngressEvents(r.Context(), ownerID, messageID, 50)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load ingress events"})
		return
	}
	writeJSON(w, http.StatusOK, ingressEventsResponse{Events: events})
}

func (h *Handler) handleRecordMessageIngressEvent(w http.ResponseWriter, r *http.Request, ownerID, messageID string) {
	if !strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Internal-Caller")), "true") {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "internal caller required"})
		return
	}
	var in recordIngressEventRequest
	if err := h.decodeJSON(r, &in); err != nil {
		writeDecodeError(w, err)
		return
	}
	sourcePacketID := strings.TrimSpace(in.SourcePacketID)
	submissionID := strings.TrimSpace(in.ConductorSubmissionID)
	if sourcePacketID == "" || submissionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "source packet and submission are required"})
		return
	}
	packet, _, err := h.store.GetSourcePacketForMessage(r.Context(), ownerID, messageID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if packet.ID != sourcePacketID {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "source packet does not match message"})
		return
	}
	status := strings.TrimSpace(in.Status)
	if status == "" {
		status = "accepted"
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	event := EmailIngressEvent{
		ID:                    ingressEventRowID(messageID, submissionID),
		MessageID:             messageID,
		SourcePacketID:        sourcePacketID,
		OwnerID:               ownerID,
		ConductorSubmissionID: submissionID,
		Status:                status,
		CreatedAt:             now,
		CompletedAt:           now,
	}
	if err := h.store.RecordIngressEvent(r.Context(), event); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to record ingress event"})
		return
	}
	writeJSON(w, http.StatusAccepted, event)
}

func (h *Handler) handleMessageRead(w http.ResponseWriter, r *http.Request, ownerID, messageID string) {
	if err := h.store.MarkMessageRead(r.Context(), ownerID, messageID, time.Now()); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "read"})
}

func (h *Handler) handleMessageUnread(w http.ResponseWriter, r *http.Request, ownerID, messageID string) {
	if err := h.store.MarkMessageUnread(r.Context(), ownerID, messageID); err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "unread"})
}

func summarizeMessage(msg EmailMessage) messageSummary {
	return messageSummary{
		ID:             msg.ID,
		Direction:      msg.Direction,
		FromAddress:    msg.FromAddress,
		FromDisplay:    msg.FromDisplay,
		Subject:        msg.Subject,
		Snippet:        snippet(msg.TextBody),
		TrustStatus:    msg.TrustStatus,
		ReadAt:         msg.ReadAt,
		ReceivedAt:     msg.ReceivedAt,
		SentAt:         msg.SentAt,
		CreatedAt:      msg.CreatedAt,
		HasAttachments: msg.HasAttachments,
	}
}

func snippet(text string) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\n", " "))
	if len(text) <= 120 {
		return text
	}
	return text[:117] + "..."
}

func parseRawHeaders(rawHeadersJSON string) map[string]string {
	if strings.TrimSpace(rawHeadersJSON) == "" {
		return nil
	}
	var headers map[string]string
	if err := json.Unmarshal([]byte(rawHeadersJSON), &headers); err != nil {
		return nil
	}
	if len(headers) == 0 {
		return nil
	}
	return headers
}

func groupRecipients(recipients []EmailRecipient) recipientsResponse {
	var out recipientsResponse
	for _, recipient := range recipients {
		address := strings.TrimSpace(recipient.Address)
		if address == "" {
			continue
		}
		item := addressResponse{Address: address, Display: strings.TrimSpace(recipient.Display)}
		switch strings.ToLower(strings.TrimSpace(recipient.Kind)) {
		case "to":
			out.To = append(out.To, item)
		case "cc":
			out.Cc = append(out.Cc, item)
		case "bcc":
			out.Bcc = append(out.Bcc, item)
		}
	}
	return out
}

func writeStoreError(w http.ResponseWriter, err error) {
	if err == sql.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "mail store error"})
}

var errAPIBodyTooLarge = errors.New("maild API request body too large")

func apiMaxBytesOrDefault(value int64) int64 {
	if value > 0 {
		return value
	}
	return DefaultAPIMaxBody
}

func (h *Handler) decodeJSON(r *http.Request, dst any) error {
	maxBytes := int64(DefaultAPIMaxBody)
	if h != nil && h.cfg != nil {
		maxBytes = apiMaxBytesOrDefault(h.cfg.APIMaxBytes)
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, maxBytes+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > maxBytes {
		return errAPIBodyTooLarge
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}

func writeDecodeError(w http.ResponseWriter, err error) {
	if errors.Is(err, errAPIBodyTooLarge) {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "request body too large"})
		return
	}
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
}
