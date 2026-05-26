package maild

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func seedMessage(t *testing.T, store *Store, ownerID, messageID, trustStatus string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := store.db.Exec(`INSERT INTO email_messages (
		id, provider, provider_message_id, provider_event_id, direction,
		mailbox_owner_id, alias_id, from_address, subject, text_body,
		trust_status, received_at, created_at
	) VALUES (?, 'resend', ?, ?, 'inbound', ?, ?, 'sender@example.com', 'Project update', ?, ?, ?, ?)`,
		messageID, "provider-"+messageID, "event-"+messageID, ownerID, DefaultRootAliasID,
		"Please review this update. It is external content.", trustStatus, now, now)
	if err != nil {
		t.Fatalf("seed message: %v", err)
	}
	_, err = store.db.Exec(`INSERT INTO email_source_packets (
		id, message_id, trust_label, provenance_json, text_ref, created_at
	) VALUES (?, ?, 'UNTRUSTED_EXTERNAL_EMAIL', '{"provider":"resend"}', ?, ?)`,
		"source-"+messageID, messageID, "message:"+messageID, now)
	if err != nil {
		t.Fatalf("seed source packet: %v", err)
	}
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

func TestHandleMessagesListsOwnerInbox(t *testing.T) {
	store, cfg := newTestStore(t)
	seedMessage(t, store, "user-1", "msg-1", "untrusted")
	seedMessage(t, store, "user-2", "msg-2", "untrusted")
	h := NewHandler(cfg, store)

	req := httptest.NewRequest(http.MethodGet, "/api/email/messages?folder=inbox", nil)
	req.Header.Set("X-Authenticated-User", "user-1")
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
}

func TestHandleMessageDetailIncludesRawHeaders(t *testing.T) {
	store, cfg := newTestStore(t)
	seedMessage(t, store, "user-1", "msg-1", "untrusted")
	if _, err := store.db.Exec(`UPDATE email_messages SET raw_headers_json = ? WHERE id = ?`,
		`{"message_id":"<provider@example.com>","authentication-results":"spf=pass"}`, "msg-1"); err != nil {
		t.Fatalf("update raw headers: %v", err)
	}
	h := NewHandler(cfg, store)

	req := httptest.NewRequest(http.MethodGet, "/api/email/messages/msg-1", nil)
	req.Header.Set("X-Authenticated-User", "user-1")
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
	_, err := store.db.Exec(`INSERT INTO email_message_recipients (id, message_id, kind, address, display)
		VALUES
		('recipient-to-1', 'msg-1', 'to', '000+read@choir.news', ''),
		('recipient-cc-1', 'msg-1', 'cc', 'copy@example.com', 'Copy Person')`)
	if err != nil {
		t.Fatalf("insert recipients: %v", err)
	}
	h := NewHandler(cfg, store)

	req := httptest.NewRequest(http.MethodGet, "/api/email/messages/msg-1", nil)
	req.Header.Set("X-Authenticated-User", "user-1")
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
	req.Header.Set("X-Authenticated-User", "user-2")
	w := httptest.NewRecorder()
	h.HandleMessages(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusNotFound, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/email/messages/msg-1/source-packet", nil)
	req.Header.Set("X-Authenticated-User", "user-1")
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
}

func TestHandleMessageIngressEventsRequiresInternalCaller(t *testing.T) {
	store, cfg := newTestStore(t)
	seedMessage(t, store, "user-1", "msg-1", "untrusted")
	h := NewHandler(cfg, store)

	body := `{"source_packet_id":"source-msg-1","conductor_submission_id":"submission-1","status":"accepted"}`
	req := httptest.NewRequest(http.MethodPost, "/api/email/messages/msg-1/ingress-events", strings.NewReader(body))
	req.Header.Set("X-Authenticated-User", "user-1")
	w := httptest.NewRecorder()
	h.HandleMessages(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusForbidden, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/email/messages/msg-1/ingress-events", strings.NewReader(body))
	req.Header.Set("X-Authenticated-User", "user-1")
	req.Header.Set("X-Internal-Caller", "true")
	w = httptest.NewRecorder()
	h.HandleMessages(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusAccepted, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/email/messages/msg-1/ingress-events", nil)
	req.Header.Set("X-Authenticated-User", "user-1")
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

func TestHandleMessageReadMarksOwnerMessage(t *testing.T) {
	store, cfg := newTestStore(t)
	seedMessage(t, store, "user-1", "msg-1", "untrusted")
	h := NewHandler(cfg, store)

	req := httptest.NewRequest(http.MethodPost, "/api/email/messages/msg-1/read", nil)
	req.Header.Set("X-Authenticated-User", "user-1")
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
