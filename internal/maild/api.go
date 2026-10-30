package maild

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
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
		// loopback: a host service, which must also hold the host-internal
		// credential (docs/problems/loopback-is-a-credential-2026-10-14.md).
		if !hostauth.Authorize(r) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "internal credential required"})
			return "", false
		}
		return ownerID, true
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
		hostauth.TestNetworkAllowed(host) || strings.HasPrefix(host, "/")
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
	hostauth.MarkInternal(req)
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
	caller, ok := h.authenticateCaller(w, r)
	return caller.OwnerID, ok
}

// callerClass says who reached maild, which decides what it may do
// (docs/archive/problems/maild-send-gate-defects-2026-10-14.md). It comes from the
// transport, never from a header the caller sets.
type callerClass string

const (
	// callerOwnerSession is a host service on a trusted transport: in
	// production, the proxy forwarding the owner's authenticated session.
	callerOwnerSession callerClass = "owner_session"
	// callerGuest is the owner's computer over its tap, bound by S1a. It
	// may draft, read and propose; it may never approve or send.
	callerGuest callerClass = "guest"
)

type maildCaller struct {
	OwnerID string
	Class   callerClass
	// SessionEmail is the owner's signup address as the proxy read it from
	// the session (X-Authenticated-Email). Empty for a guest: a guest's
	// header is never believed.
	SessionEmail string
}

func (h *Handler) authenticateCaller(w http.ResponseWriter, r *http.Request) (maildCaller, bool) {
	ownerID := strings.TrimSpace(r.Header.Get("X-Authenticated-User"))
	if ownerID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return maildCaller{}, false
	}
	// S1a: the caller must be either a host service on a trusted transport
	// (loopback / unix socket / TEST-NET-1 test harness) or a bound guest
	// whose tap IP resolves to a live ownership for the asserted owner.
	srcHost := maildRemoteAddrHost(r)
	if isMaildLoopbackHost(srcHost) {
		if !strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Internal-Caller")), "true") {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "internal caller required"})
			return maildCaller{}, false
		}
		if !hostauth.Authorize(r) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "internal credential required"})
			return maildCaller{}, false
		}
		caller := maildCaller{
			OwnerID:      ownerID,
			Class:        callerOwnerSession,
			SessionEmail: normalizedTrustedEmail(r.Header.Get("X-Authenticated-Email")),
		}
		h.rememberOwnerAddress(r.Context(), caller)
		return caller, true
	}
	boundID, ok := h.s1aBindTapCallerToOwner(w, r, ownerID)
	if !ok {
		return maildCaller{}, false
	}
	// The guest's X-Authenticated-Email is never read, in either send gate
	// mode: owner-facing mail goes to the address of record (G6).
	return maildCaller{OwnerID: boundID, Class: callerGuest}, true
}

// requireOwnerSession refuses an owner act (approve, send) from anything
// but the owner's session. A guest bound to the owner is the party the
// approval gates, so it cannot open the gate (G1).
func (h *Handler) requireOwnerSession(w http.ResponseWriter, r *http.Request, caller maildCaller, act string) bool {
	if caller.Class == callerOwnerSession || h.cfg.legacySendGate() {
		return true
	}
	log.Printf("maild: send gate: refused %s from %s caller owner=%s remote=%s: owner session required",
		act, caller.Class, caller.OwnerID, r.RemoteAddr)
	writeJSON(w, http.StatusForbidden, map[string]string{
		"error": "owner session required: " + act + " is an owner act and cannot come from the computer",
	})
	return false
}

// rememberOwnerAddress keeps the owner's address of record: the signup
// address the proxy forwards from an authenticated owner session (the JWT
// email claim, or the auth registry for an API key). The auth service stays
// the authority; this is a derived copy, refreshed by every owner request,
// so guest-originated mail (approval requests, risk alerts) has a recipient
// the guest cannot choose (G6).
func (h *Handler) rememberOwnerAddress(ctx context.Context, caller maildCaller) {
	if h == nil || h.store == nil || caller.Class != callerOwnerSession || caller.SessionEmail == "" {
		return
	}
	h.addressMu.Lock()
	known := h.knownAddresses[caller.OwnerID] == caller.SessionEmail
	h.addressMu.Unlock()
	if known {
		return
	}
	if err := h.store.RecordOwnerAddress(ctx, caller.OwnerID, caller.SessionEmail); err != nil {
		log.Printf("maild: record owner address of record owner=%s: %v", caller.OwnerID, err)
		return
	}
	h.addressMu.Lock()
	if h.knownAddresses == nil {
		h.knownAddresses = map[string]string{}
	}
	h.knownAddresses[caller.OwnerID] = caller.SessionEmail
	h.addressMu.Unlock()
}

// ownerAddressOfRecord is where owner-facing mail goes: the live session's
// address for the owner's own requests, and the recorded address for a
// guest's. Never a header a guest set. Empty when there is none.
func (h *Handler) ownerAddressOfRecord(ctx context.Context, caller maildCaller) string {
	if caller.Class == callerOwnerSession {
		return caller.SessionEmail
	}
	if h == nil || h.store == nil {
		return ""
	}
	address, err := h.store.OwnerAddressOfRecord(ctx, caller.OwnerID)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("maild: load owner address of record owner=%s: %v", caller.OwnerID, err)
		}
		return ""
	}
	return address
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
