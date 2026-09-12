package maild

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func seedMessage(t *testing.T, store *Store, ownerID, messageID, trustStatus string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	db, err := store.mailboxForOwner(ownerID)
	if err != nil {
		t.Fatalf("open mailbox for seed: %v", err)
	}
	_, err = db.Exec(`INSERT INTO email_messages (
		id, provider, provider_message_id, provider_event_id, direction,
		mailbox_owner_id, alias_id, from_address, subject, text_body,
		trust_status, received_at, created_at
	) VALUES (?, 'resend', ?, ?, 'inbound', ?, ?, 'sender@example.com', 'Project update', ?, ?, ?, ?)`,
		messageID, "provider-"+messageID, "event-"+messageID, ownerID, DefaultRootAliasID,
		"Please review this update. It is external content.", trustStatus, now, now)
	if err != nil {
		t.Fatalf("seed message: %v", err)
	}
	_, err = db.Exec(`INSERT INTO email_source_packets (
		id, message_id, trust_label, provenance_json, text_ref, created_at
	) VALUES (?, ?, 'UNTRUSTED_EXTERNAL_EMAIL', '{"provider":"resend"}', ?, ?)`,
		"source-"+messageID, messageID, "message:"+messageID, now)
	if err != nil {
		t.Fatalf("seed source packet: %v", err)
	}
}

func setInternalOwner(req *http.Request, ownerID string) {
	req.Header.Set("X-Authenticated-User", ownerID)
	req.Header.Set("X-Internal-Caller", "true")
}

func TestHandleMessagesRequiresTrustedUser(t *testing.T) {
	store, cfg := newTestStore(t)
	h := NewHandler(cfg, store)
	req := httptest.NewRequest(http.MethodGet, "/api/email/messages", nil)
	w := httptest.NewRecorder()
	h.HandleMessages(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleMessagesRequiresInternalCaller(t *testing.T) {
	store, cfg := newTestStore(t)
	h := NewHandler(cfg, store)
	req := httptest.NewRequest(http.MethodGet, "/api/email/messages", nil)
	req.Header.Set("X-Authenticated-User", "user-1")
	w := httptest.NewRecorder()
	h.HandleMessages(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusForbidden, w.Body.String())
	}
}

func TestHandleMessagesListsOwnerInbox(t *testing.T) {
	store, cfg := newTestStore(t)
	seedMessage(t, store, "user-1", "msg-1", "untrusted")
	seedMessage(t, store, "user-2", "msg-2", "untrusted")
	h := NewHandler(cfg, store)

	req := httptest.NewRequest(http.MethodGet, "/api/email/messages?folder=inbox", nil)
	setInternalOwner(req, "user-1")
	w := httptest.NewRecorder()
	h.HandleMessages(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp messageListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Messages) != 1 || resp.Messages[0].ID != "msg-1" {
		t.Fatalf("messages = %+v, want only msg-1", resp.Messages)
	}
	msg, err := store.GetMessage(req.Context(), "user-1", "msg-1")
	if err != nil {
		t.Fatalf("GetMessage: %v", err)
	}
	if msg.Provider != "resend" || msg.ProviderMessageID != "provider-msg-1" || msg.ProviderEventID != "event-msg-1" {
		t.Fatalf("provider ids = provider=%q message=%q event=%q", msg.Provider, msg.ProviderMessageID, msg.ProviderEventID)
	}
}

func TestHandleMessagesListsAttachmentIndicator(t *testing.T) {
	store, cfg := newTestStore(t)
	seedMessage(t, store, "user-1", "msg-1", "quarantined")
	mbDB, err := store.mailboxForOwner("user-1")
	if err != nil {
		t.Fatalf("open mailbox: %v", err)
	}
	if _, err := mbDB.Exec(`INSERT INTO email_attachments (
		id, message_id, filename, content_type, size_bytes, status, created_at
	) VALUES ('att-1', 'msg-1', 'brief.pdf', 'application/pdf', 1024, 'quarantined', ?)`,
		time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatalf("insert attachment: %v", err)
	}
	h := NewHandler(cfg, store)

	req := httptest.NewRequest(http.MethodGet, "/api/email/messages?folder=quarantine", nil)
	setInternalOwner(req, "user-1")
	w := httptest.NewRecorder()
	h.HandleMessages(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp messageListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Messages) != 1 || !resp.Messages[0].HasAttachments {
		t.Fatalf("messages = %+v, want attachment indicator", resp.Messages)
	}
}

func TestHandleMessageDetailIncludesRawHeaders(t *testing.T) {
	store, cfg := newTestStore(t)
	seedMessage(t, store, "user-1", "msg-1", "untrusted")
	mbDB, err := store.mailboxForOwner("user-1")
	if err != nil {
		t.Fatalf("open mailbox: %v", err)
	}
	if _, err := mbDB.Exec(`UPDATE email_messages SET raw_headers_json = ? WHERE id = ?`,
		`{"message_id":"<provider@example.com>","authentication-results":"spf=pass"}`, "msg-1"); err != nil {
		t.Fatalf("update raw headers: %v", err)
	}
	h := NewHandler(cfg, store)

	req := httptest.NewRequest(http.MethodGet, "/api/email/messages/msg-1", nil)
	setInternalOwner(req, "user-1")
	w := httptest.NewRecorder()
	h.HandleMessages(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp messageDetailResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.RawHeaders["message_id"] != "<provider@example.com>" {
		t.Fatalf("raw headers = %+v", resp.RawHeaders)
	}
	if resp.RawHeaders["authentication-results"] != "spf=pass" {
		t.Fatalf("raw headers = %+v", resp.RawHeaders)
	}
}

func TestHandleMessageDetailIncludesStoredRecipients(t *testing.T) {
	store, cfg := newTestStore(t)
	seedMessage(t, store, "user-1", "msg-1", "untrusted")
	mbDB, err := store.mailboxForOwner("user-1")
	if err != nil {
		t.Fatalf("open mailbox: %v", err)
	}
	_, err = mbDB.Exec(`INSERT INTO email_message_recipients (id, message_id, kind, address, display)
		VALUES
		('recipient-to-1', 'msg-1', 'to', '000+read@choir.news', ''),
		('recipient-cc-1', 'msg-1', 'cc', 'copy@example.com', 'Copy Person')`)
	if err != nil {
		t.Fatalf("insert recipients: %v", err)
	}
	h := NewHandler(cfg, store)

	req := httptest.NewRequest(http.MethodGet, "/api/email/messages/msg-1", nil)
	setInternalOwner(req, "user-1")
	w := httptest.NewRecorder()
	h.HandleMessages(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp messageDetailResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Recipients.To) != 1 || resp.Recipients.To[0].Address != "000+read@choir.news" {
		t.Fatalf("to recipients = %+v", resp.Recipients.To)
	}
	if len(resp.Recipients.Cc) != 1 || resp.Recipients.Cc[0].Address != "copy@example.com" || resp.Recipients.Cc[0].Display != "Copy Person" {
		t.Fatalf("cc recipients = %+v", resp.Recipients.Cc)
	}
}

func TestHandleMessageSourcePacketEnforcesOwnership(t *testing.T) {
	store, cfg := newTestStore(t)
	seedMessage(t, store, "user-1", "msg-1", "untrusted")
	h := NewHandler(cfg, store)

	req := httptest.NewRequest(http.MethodGet, "/api/email/messages/msg-1/source-packet", nil)
	setInternalOwner(req, "user-2")
	w := httptest.NewRecorder()
	h.HandleMessages(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusNotFound, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/email/messages/msg-1/source-packet", nil)
	setInternalOwner(req, "user-1")
	w = httptest.NewRecorder()
	h.HandleMessages(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp sourcePacketResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.SourcePacketID != "source-msg-1" || resp.TrustLabel != "UNTRUSTED_EXTERNAL_EMAIL" {
		t.Fatalf("source response = %+v", resp)
	}
	if resp.TextRef != "message:msg-1" {
		t.Fatalf("text ref = %q, want message:msg-1", resp.TextRef)
	}
	if !strings.Contains(resp.TextBody, "external content") {
		t.Fatalf("text body = %q, want stored message body", resp.TextBody)
	}
	if resp.ProvenanceJSON != `{"provider":"resend"}` {
		t.Fatalf("provenance = %q", resp.ProvenanceJSON)
	}
}

func TestHandleMessageIngressEventsRequiresInternalCaller(t *testing.T) {
	store, cfg := newTestStore(t)
	seedMessage(t, store, "user-1", "msg-1", "untrusted")
	h := NewHandler(cfg, store)

	body := `{"source_packet_id":"source-msg-1","conductor_submission_id":"submission-1","status":"accepted"}`
	req := httptest.NewRequest(http.MethodPost, "/api/email/messages/msg-1/ingress-events", strings.NewReader(body))
	setInternalOwner(req, "user-1")
	req.Header.Del("X-Internal-Caller")
	w := httptest.NewRecorder()
	h.HandleMessages(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusForbidden, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/email/messages/msg-1/ingress-events", strings.NewReader(body))
	setInternalOwner(req, "user-1")
	w = httptest.NewRecorder()
	h.HandleMessages(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusAccepted, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/email/messages/msg-1/ingress-events", nil)
	setInternalOwner(req, "user-1")
	w = httptest.NewRecorder()
	h.HandleMessages(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp ingressEventsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode ingress response: %v", err)
	}
	if len(resp.Events) != 1 || resp.Events[0].ConductorSubmissionID != "submission-1" {
		t.Fatalf("events = %+v", resp.Events)
	}
}

func TestHandleMessageIngressEventsIsIdempotentForSameSubmission(t *testing.T) {
	store, cfg := newTestStore(t)
	seedMessage(t, store, "user-1", "msg-1", "untrusted")
	h := NewHandler(cfg, store)

	body := `{"source_packet_id":"source-msg-1","conductor_submission_id":"submission-1","status":"accepted"}`
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/email/messages/msg-1/ingress-events", strings.NewReader(body))
		setInternalOwner(req, "user-1")
		w := httptest.NewRecorder()
		h.HandleMessages(w, req)
		if w.Code != http.StatusAccepted {
			t.Fatalf("post %d status = %d, want %d; body=%s", i+1, w.Code, http.StatusAccepted, w.Body.String())
		}
	}

	events, err := store.ListIngressEvents(t.Context(), "user-1", "msg-1", 10)
	if err != nil {
		t.Fatalf("list ingress events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1: %+v", len(events), events)
	}
	if events[0].ConductorSubmissionID != "submission-1" || events[0].SourcePacketID != "source-msg-1" {
		t.Fatalf("event = %+v", events[0])
	}
}

func TestHandleMessageIngressEventsRejectsOversizedRequestBody(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.APIMaxBytes = 128
	seedMessage(t, store, "user-1", "msg-1", "untrusted")
	h := NewHandler(cfg, store)

	body := `{"source_packet_id":"source-msg-1","conductor_submission_id":"submission-1","status":"` + strings.Repeat("x", 256) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/email/messages/msg-1/ingress-events", strings.NewReader(body))
	setInternalOwner(req, "user-1")
	w := httptest.NewRecorder()
	h.HandleMessages(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusRequestEntityTooLarge, w.Body.String())
	}
	events, err := store.ListIngressEvents(req.Context(), "user-1", "msg-1", 10)
	if err != nil {
		t.Fatalf("list ingress events: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("events = %+v, want none after oversized request", events)
	}
}

func TestHandleMessageReadMarksOwnerMessage(t *testing.T) {
	store, cfg := newTestStore(t)
	seedMessage(t, store, "user-1", "msg-1", "untrusted")
	h := NewHandler(cfg, store)

	req := httptest.NewRequest(http.MethodPost, "/api/email/messages/msg-1/read", nil)
	setInternalOwner(req, "user-1")
	w := httptest.NewRecorder()
	h.HandleMessages(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	msg, err := store.GetMessage(req.Context(), "user-1", "msg-1")
	if err != nil {
		t.Fatalf("GetMessage: %v", err)
	}
	if msg.ReadAt == "" {
		t.Fatalf("ReadAt not set")
	}
}

func TestHandleMessagesPaginationAndCounts(t *testing.T) {
	store, cfg := newTestStore(t)
	h := NewHandler(cfg, store)

	// Seed 3 messages with descending timestamps
	base := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	db, err := store.mailboxForOwner("user-paged")
	if err != nil {
		t.Fatalf("open mailbox: %v", err)
	}
	for i := 1; i <= 3; i++ {
		ts := base.Add(time.Duration(i) * time.Hour).Format(time.RFC3339Nano)
		msgID := fmt.Sprintf("msg-%d", i)
		readAt := ""
		if i == 1 {
			readAt = ts // msg-1 is read, msg-2 and msg-3 are unread
		}
		_, err = db.Exec(`INSERT INTO email_messages (
			id, provider, provider_message_id, provider_event_id, direction,
			mailbox_owner_id, alias_id, from_address, subject, text_body,
			trust_status, received_at, created_at, read_at
		) VALUES (?, 'resend', ?, ?, 'inbound', 'user-paged', 'alias-1', 'sender@example.com', 'Update', 'Body', 'untrusted', ?, ?, ?)`,
			msgID, "p-"+msgID, "e-"+msgID, ts, ts, readAt)
		if err != nil {
			t.Fatalf("insert msg %d: %v", i, err)
		}
	}

	// Page 1: limit 2
	req1 := httptest.NewRequest(http.MethodGet, "/api/email/messages?folder=inbox&limit=2", nil)
	setInternalOwner(req1, "user-paged")
	w1 := httptest.NewRecorder()
	h.HandleMessages(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("page 1 status = %d, want 200", w1.Code)
	}
	var resp1 messageListResponse
	if err := json.NewDecoder(w1.Body).Decode(&resp1); err != nil {
		t.Fatalf("decode page 1: %v", err)
	}
	if len(resp1.Messages) != 2 {
		t.Fatalf("page 1 got %d messages, want 2", len(resp1.Messages))
	}
	if resp1.Total != 3 || resp1.Unread != 2 {
		t.Fatalf("page 1 total=%d unread=%d, want total=3 unread=2", resp1.Total, resp1.Unread)
	}
	if resp1.NextCursor == "" {
		t.Fatalf("page 1 expected non-empty next_cursor")
	}
	// Newest first: msg-3 then msg-2
	if resp1.Messages[0].ID != "msg-3" || resp1.Messages[1].ID != "msg-2" {
		t.Fatalf("page 1 unexpected order: %+v", resp1.Messages)
	}

	// Page 2: with cursor
	req2 := httptest.NewRequest(http.MethodGet, "/api/email/messages?folder=inbox&limit=2&cursor="+resp1.NextCursor, nil)
	setInternalOwner(req2, "user-paged")
	w2 := httptest.NewRecorder()
	h.HandleMessages(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("page 2 status = %d, want 200", w2.Code)
	}
	var resp2 messageListResponse
	if err := json.NewDecoder(w2.Body).Decode(&resp2); err != nil {
		t.Fatalf("decode page 2: %v", err)
	}
	if len(resp2.Messages) != 1 || resp2.Messages[0].ID != "msg-1" {
		t.Fatalf("page 2 got %+v, want only msg-1", resp2.Messages)
	}
	if resp2.NextCursor != "" {
		t.Fatalf("page 2 expected empty next_cursor at end, got %q", resp2.NextCursor)
	}
	if resp2.Total != 3 || resp2.Unread != 2 {
		t.Fatalf("page 2 total=%d unread=%d, want total=3 unread=2", resp2.Total, resp2.Unread)
	}

	// Malformed cursor: returns 400
	reqBad := httptest.NewRequest(http.MethodGet, "/api/email/messages?folder=inbox&cursor=invalid-not-base64", nil)
	setInternalOwner(reqBad, "user-paged")
	wBad := httptest.NewRecorder()
	h.HandleMessages(wBad, reqBad)
	if wBad.Code != http.StatusBadRequest {
		t.Fatalf("bad cursor status = %d, want 400", wBad.Code)
	}
}

func TestHandleMessageMarkReadAndUnread(t *testing.T) {
	store, cfg := newTestStore(t)
	h := NewHandler(cfg, store)

	db, err := store.mailboxForOwner("user-read-test")
	if err != nil {
		t.Fatalf("open mailbox: %v", err)
	}
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = db.Exec(`INSERT INTO email_messages (
		id, provider, provider_message_id, provider_event_id, direction,
		mailbox_owner_id, alias_id, from_address, subject, text_body,
		trust_status, received_at, created_at, read_at
	) VALUES ('msg-read-1', 'resend', 'p-1', 'e-1', 'inbound', 'user-read-test', 'alias-1', 'sender@example.com', 'Test Read', 'Body', 'untrusted', ?, ?, NULL)`, ts, ts)
	if err != nil {
		t.Fatalf("insert message: %v", err)
	}

	// 1. Initially unread: unread = 1
	reqList1 := httptest.NewRequest(http.MethodGet, "/api/email/messages?folder=inbox", nil)
	setInternalOwner(reqList1, "user-read-test")
	wList1 := httptest.NewRecorder()
	h.HandleMessages(wList1, reqList1)
	if wList1.Code != http.StatusOK {
		t.Fatalf("list 1 status = %d, want 200", wList1.Code)
	}
	var respList1 messageListResponse
	if err := json.NewDecoder(wList1.Body).Decode(&respList1); err != nil {
		t.Fatalf("decode list 1: %v", err)
	}
	if respList1.Total != 1 || respList1.Unread != 1 {
		t.Fatalf("initial unread = %d, want 1", respList1.Unread)
	}
	if respList1.Messages[0].ReadAt != "" {
		t.Fatalf("initial read_at = %q, want empty", respList1.Messages[0].ReadAt)
	}

	// 2. Mark read: POST /api/email/messages/msg-read-1/read
	reqRead := httptest.NewRequest(http.MethodPost, "/api/email/messages/msg-read-1/read", nil)
	setInternalOwner(reqRead, "user-read-test")
	wRead := httptest.NewRecorder()
	h.HandleMessages(wRead, reqRead)
	if wRead.Code != http.StatusOK {
		t.Fatalf("mark read status = %d, want 200", wRead.Code)
	}

	// 3. Verify unread = 0 and read_at is non-empty
	reqList2 := httptest.NewRequest(http.MethodGet, "/api/email/messages?folder=inbox", nil)
	setInternalOwner(reqList2, "user-read-test")
	wList2 := httptest.NewRecorder()
	h.HandleMessages(wList2, reqList2)
	var respList2 messageListResponse
	if err := json.NewDecoder(wList2.Body).Decode(&respList2); err != nil {
		t.Fatalf("decode list 2: %v", err)
	}
	if respList2.Unread != 0 {
		t.Fatalf("post-read unread = %d, want 0", respList2.Unread)
	}
	if respList2.Messages[0].ReadAt == "" {
		t.Fatalf("post-read read_at is empty, expected timestamp")
	}

	// 4. Mark unread: POST /api/email/messages/msg-read-1/unread
	reqUnread := httptest.NewRequest(http.MethodPost, "/api/email/messages/msg-read-1/unread", nil)
	setInternalOwner(reqUnread, "user-read-test")
	wUnread := httptest.NewRecorder()
	h.HandleMessages(wUnread, reqUnread)
	if wUnread.Code != http.StatusOK {
		t.Fatalf("mark unread status = %d, want 200", wUnread.Code)
	}

	// 5. Verify unread = 1 and read_at is cleared
	reqList3 := httptest.NewRequest(http.MethodGet, "/api/email/messages?folder=inbox", nil)
	setInternalOwner(reqList3, "user-read-test")
	wList3 := httptest.NewRecorder()
	h.HandleMessages(wList3, reqList3)
	var respList3 messageListResponse
	if err := json.NewDecoder(wList3.Body).Decode(&respList3); err != nil {
		t.Fatalf("decode list 3: %v", err)
	}
	if respList3.Unread != 1 {
		t.Fatalf("post-unread unread = %d, want 1", respList3.Unread)
	}
	if respList3.Messages[0].ReadAt != "" {
		t.Fatalf("post-unread read_at = %q, want empty", respList3.Messages[0].ReadAt)
	}

	// 6. Mark unread via DELETE /api/email/messages/msg-read-1/read
	reqDeleteRead := httptest.NewRequest(http.MethodDelete, "/api/email/messages/msg-read-1/read", nil)
	setInternalOwner(reqDeleteRead, "user-read-test")
	wDeleteRead := httptest.NewRecorder()
	h.HandleMessages(wDeleteRead, reqDeleteRead)
	if wDeleteRead.Code != http.StatusOK {
		t.Fatalf("delete read status = %d, want 200", wDeleteRead.Code)
	}
}
