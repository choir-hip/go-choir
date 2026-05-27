package maild

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func signedResendRequest(t *testing.T, secret, msgID, body string) *http.Request {
	t.Helper()
	return signedResendRequestAt(t, secret, msgID, body, time.Now().UTC())
}

func signedResendRequestAt(t *testing.T, secret, msgID, body string, ts time.Time) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/email/resend/webhook", strings.NewReader(body))
	key, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_"))
	if err != nil {
		t.Fatalf("decode secret: %v", err)
	}
	sig := "v1," + signSvixPayload(key, msgID, ts.Unix(), []byte(body))
	req.Header.Set("svix-id", msgID)
	req.Header.Set("svix-timestamp", strconv.FormatInt(ts.Unix(), 10))
	req.Header.Set("svix-signature", sig)
	return req
}

func TestHandleResendWebhookRequiresSecret(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.WebhookSecret = ""
	h := NewHandler(cfg, store)

	req := httptest.NewRequest(http.MethodPost, "/api/email/resend/webhook", strings.NewReader(`{"id":"evt-1","type":"email.received"}`))
	w := httptest.NewRecorder()
	h.HandleResendWebhook(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusServiceUnavailable, w.Body.String())
	}
}

func TestHandleResendWebhookRejectsEmptyDecodedSecret(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.WebhookSecret = "whsec_"
	h := NewHandler(cfg, store)
	body := `{"id":"evt-empty-secret","type":"domain.updated"}`

	req := signedResendRequest(t, cfg.WebhookSecret, "msg-empty-secret", body)
	w := httptest.NewRecorder()
	h.HandleResendWebhook(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "invalid_signature") {
		t.Fatalf("body = %s, want invalid_signature", w.Body.String())
	}
	count, err := store.CountWebhookEvents(req.Context())
	if err != nil {
		t.Fatalf("CountWebhookEvents: %v", err)
	}
	if count != 0 {
		t.Fatalf("webhook count = %d, want 0", count)
	}
}

func TestHandleResendWebhookStoresVerifiedEventIdempotently(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.WebhookSecret = "whsec_" + "dGVzdC1zZWNyZXQ="
	h := NewHandler(cfg, store)
	body := `{"id":"evt-1","type":"domain.updated"}`

	req := signedResendRequest(t, cfg.WebhookSecret, "msg-1", body)
	w := httptest.NewRecorder()
	h.HandleResendWebhook(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("first status = %d, want %d; body=%s", w.Code, http.StatusAccepted, w.Body.String())
	}

	req = signedResendRequest(t, cfg.WebhookSecret, "msg-1", body)
	w = httptest.NewRecorder()
	h.HandleResendWebhook(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("duplicate status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	count, err := store.CountWebhookEvents(req.Context())
	if err != nil {
		t.Fatalf("CountWebhookEvents: %v", err)
	}
	if count != 1 {
		t.Fatalf("webhook count = %d, want 1", count)
	}
}

func TestHandleResendWebhookAcceptsAnyValidSvixSignature(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.WebhookSecret = "whsec_" + "dGVzdC1zZWNyZXQ="
	h := NewHandler(cfg, store)
	body := `{"id":"evt-multiple-signatures","type":"domain.updated"}`

	req := signedResendRequest(t, cfg.WebhookSecret, "msg-multiple-signatures", body)
	req.Header.Set("svix-signature", "v1,invalid-signature "+req.Header.Get("svix-signature"))
	w := httptest.NewRecorder()
	h.HandleResendWebhook(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusAccepted, w.Body.String())
	}
}

func TestHandleResendWebhookDuplicateRetriesMissingInboundMessage(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.WebhookSecret = "whsec_" + "dGVzdC1zZWNyZXQ="
	cfg.ResendAPIKey = "re_test"
	var calls int32
	resend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			http.Error(w, "temporary provider failure", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"email-retry-1",
			"to":["000@choir.news"],
			"from":"sender@example.com",
			"created_at":"2026-05-26T10:00:00Z",
			"subject":"Retry me",
			"text":"This should store after retry.",
			"headers":{"from":"Sender <sender@example.com>","authentication-results":"mx.example; spf=pass; dkim=pass"},
			"bcc":[],
			"cc":[],
			"reply_to":[],
			"message_id":"<email-retry-1@example.com>",
			"attachments":[]
		}`))
	}))
	defer resend.Close()
	cfg.ResendBaseURL = resend.URL
	h := NewHandler(cfg, store)
	h.resend = newResendClient(cfg, resend.Client())

	body := `{"id":"evt-retry-1","type":"email.received","data":{"email_id":"email-retry-1"}}`
	req := signedResendRequest(t, cfg.WebhookSecret, "msg-retry-1", body)
	w := httptest.NewRecorder()
	h.HandleResendWebhook(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("first status = %d, want %d; body=%s", w.Code, http.StatusServiceUnavailable, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "ingest_retry_requested") {
		t.Fatalf("first body = %s, want ingest_retry_requested", w.Body.String())
	}
	stats, err := store.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats after first delivery: %v", err)
	}
	if stats.Messages != 0 || stats.WebhookEvents != 1 {
		t.Fatalf("stats after first delivery = %+v, want one event and no message", stats)
	}

	req = signedResendRequest(t, cfg.WebhookSecret, "msg-retry-1", body)
	w = httptest.NewRecorder()
	h.HandleResendWebhook(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("retry status = %d, want %d; body=%s", w.Code, http.StatusAccepted, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "duplicate_ingested") {
		t.Fatalf("retry body = %s, want duplicate_ingested", w.Body.String())
	}
	messages, err := store.ListMessages(req.Context(), "user-root", "inbox", 10)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(messages) != 1 || messages[0].Subject != "Retry me" {
		t.Fatalf("messages = %+v, want retried message", messages)
	}
	count, err := store.CountWebhookEvents(context.Background())
	if err != nil {
		t.Fatalf("CountWebhookEvents: %v", err)
	}
	if count != 1 {
		t.Fatalf("webhook count = %d, want 1", count)
	}
}

func TestHandleResendWebhookDuplicateRetryFailureRequestsAnotherRetry(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.WebhookSecret = "whsec_" + "dGVzdC1zZWNyZXQ="
	cfg.ResendAPIKey = "re_test"
	resend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "temporary provider failure", http.StatusBadGateway)
	}))
	defer resend.Close()
	cfg.ResendBaseURL = resend.URL
	h := NewHandler(cfg, store)
	h.resend = newResendClient(cfg, resend.Client())

	body := `{"id":"evt-retry-still-failing","type":"email.received","data":{"email_id":"email-retry-still-failing"}}`
	req := signedResendRequest(t, cfg.WebhookSecret, "msg-retry-still-failing", body)
	w := httptest.NewRecorder()
	h.HandleResendWebhook(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("first status = %d, want %d; body=%s", w.Code, http.StatusServiceUnavailable, w.Body.String())
	}

	req = signedResendRequest(t, cfg.WebhookSecret, "msg-retry-still-failing", body)
	w = httptest.NewRecorder()
	h.HandleResendWebhook(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("retry status = %d, want %d; body=%s", w.Code, http.StatusServiceUnavailable, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "duplicate_ingest_retry_requested") {
		t.Fatalf("retry body = %s, want duplicate_ingest_retry_requested", w.Body.String())
	}
}

func TestHandleResendWebhookFetchesAndStoresInboundMessage(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.WebhookSecret = "whsec_" + "dGVzdC1zZWNyZXQ="
	cfg.ResendAPIKey = "re_test"
	resend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/emails/receiving/email-1" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer re_test" {
			t.Fatalf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"email-1",
			"to":["000@choir.news"],
			"from":"sender@example.com",
			"created_at":"2026-05-26T10:00:00Z",
			"subject":"Project files",
			"text":"Please review the attached files.",
			"html":"<p>Please review the attached files.</p>",
			"headers":{"from":"Sender Name <sender@example.com>","authentication-results":"mx.example; dkim=pass"},
			"bcc":[],
			"cc":[],
			"reply_to":[],
			"message_id":"<email-1@example.com>",
			"attachments":[{"id":"att-1","filename":"brief.pdf","content_type":"application/pdf","content_disposition":"attachment","size":1234}]
		}`))
	}))
	defer resend.Close()
	cfg.ResendBaseURL = resend.URL
	h := NewHandler(cfg, store)
	h.resend = newResendClient(cfg, resend.Client())

	body := `{"id":"evt-1","type":"email.received","data":{"email_id":"email-1"}}`
	req := signedResendRequest(t, cfg.WebhookSecret, "msg-1", body)
	w := httptest.NewRecorder()
	h.HandleResendWebhook(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusAccepted, w.Body.String())
	}

	messages, err := store.ListMessages(req.Context(), "user-root", "quarantine", 10)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("messages = %+v, want one quarantined message", messages)
	}
	msg := messages[0]
	if msg.Subject != "Project files" || msg.FromDisplay != "Sender Name" || msg.TrustStatus != "quarantined" {
		t.Fatalf("message = %+v", msg)
	}
	if msg.Provider != providerResend || msg.ProviderMessageID != "email-1" || msg.ProviderEventID != "evt-1" {
		t.Fatalf("provider ids = provider=%q message=%q event=%q", msg.Provider, msg.ProviderMessageID, msg.ProviderEventID)
	}
	var headers map[string]string
	if err := json.Unmarshal([]byte(msg.RawHeadersJSON), &headers); err != nil {
		t.Fatalf("unmarshal raw headers: %v", err)
	}
	if headers["message_id"] != "<email-1@example.com>" {
		t.Fatalf("stored message_id = %q", headers["message_id"])
	}
	var authResults map[string]string
	if err := json.Unmarshal([]byte(msg.AuthenticationResultsJSON), &authResults); err != nil {
		t.Fatalf("unmarshal authentication results: %v", err)
	}
	if authResults["authentication-results"] != "mx.example; dkim=pass" {
		t.Fatalf("authentication results = %+v", authResults)
	}
	attachments, err := store.ListAttachments(req.Context(), "user-root", msg.ID)
	if err != nil {
		t.Fatalf("ListAttachments: %v", err)
	}
	if len(attachments) != 1 || attachments[0].Status != "quarantined" || attachments[0].Filename != "brief.pdf" {
		t.Fatalf("attachments = %+v", attachments)
	}
	packet, _, err := store.GetSourcePacketForMessage(req.Context(), "user-root", msg.ID)
	if err != nil {
		t.Fatalf("GetSourcePacketForMessage: %v", err)
	}
	if packet.TrustLabel != "UNTRUSTED_EXTERNAL_EMAIL" {
		t.Fatalf("TrustLabel = %q", packet.TrustLabel)
	}
	if packet.TextRef != "message:"+msg.ID {
		t.Fatalf("TextRef = %q, want %q", packet.TextRef, "message:"+msg.ID)
	}
}

func TestHandleResendWebhookRejectsUnwhitelistedTrustedUploadAlias(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.WebhookSecret = "whsec_" + "dGVzdC1zZWNyZXQ="
	cfg.ResendAPIKey = "re_test"
	seedTrustedUploadAlias(t, store, false)
	resend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"email-upload-1",
			"to":["000+upload-secret@choir.news"],
			"from":"sender@example.com",
			"created_at":"2026-05-26T10:00:00Z",
			"subject":"Trusted upload",
			"text":"Please file this.",
			"headers":{"from":"Sender <sender@example.com>","authentication-results":"mx.example; spf=pass; dkim=pass"},
			"bcc":[],
			"cc":[],
			"reply_to":[],
			"message_id":"<email-upload-1@example.com>",
			"attachments":[]
		}`))
	}))
	defer resend.Close()
	cfg.ResendBaseURL = resend.URL
	h := NewHandler(cfg, store)
	h.resend = newResendClient(cfg, resend.Client())

	body := `{"id":"evt-upload-1","type":"email.received","data":{"email_id":"email-upload-1"}}`
	req := signedResendRequest(t, cfg.WebhookSecret, "msg-upload-1", body)
	w := httptest.NewRecorder()
	h.HandleResendWebhook(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusAccepted, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "accepted_ingest_failed") {
		t.Fatalf("body = %s, want accepted_ingest_failed", w.Body.String())
	}
	stats, err := store.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.Messages != 0 {
		t.Fatalf("messages = %d, want 0", stats.Messages)
	}
}

func TestHandleResendWebhookRejectsWhitelistedTrustedUploadAliasWithoutAuthenticationResults(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.WebhookSecret = "whsec_" + "dGVzdC1zZWNyZXQ="
	cfg.ResendAPIKey = "re_test"
	seedTrustedUploadAlias(t, store, true)
	resend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"email-upload-missing-auth-results",
			"to":["000+upload-secret@choir.news"],
			"from":"sender@example.com",
			"created_at":"2026-05-26T10:00:00Z",
			"subject":"Trusted upload missing auth results",
			"text":"Please file this.",
			"headers":{"from":"Sender <sender@example.com>"},
			"bcc":[],
			"cc":[],
			"reply_to":[],
			"message_id":"<email-upload-missing-auth-results@example.com>",
			"attachments":[]
		}`))
	}))
	defer resend.Close()
	cfg.ResendBaseURL = resend.URL
	h := NewHandler(cfg, store)
	h.resend = newResendClient(cfg, resend.Client())

	body := `{"id":"evt-upload-missing-auth-results","type":"email.received","data":{"email_id":"email-upload-missing-auth-results"}}`
	req := signedResendRequest(t, cfg.WebhookSecret, "msg-upload-missing-auth-results", body)
	w := httptest.NewRecorder()
	h.HandleResendWebhook(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusAccepted, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "accepted_ingest_failed") {
		t.Fatalf("body = %s, want accepted_ingest_failed", w.Body.String())
	}
	stats, err := store.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.Messages != 0 {
		t.Fatalf("messages = %d, want 0", stats.Messages)
	}
}

func TestHandleResendWebhookAcceptsWhitelistedTrustedUploadAlias(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.WebhookSecret = "whsec_" + "dGVzdC1zZWNyZXQ="
	cfg.ResendAPIKey = "re_test"
	seedTrustedUploadAlias(t, store, true)
	resend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"email-upload-2",
			"to":["000+upload-secret@choir.news"],
			"from":"sender@example.com",
			"created_at":"2026-05-26T10:00:00Z",
			"subject":"Trusted upload",
			"text":"Please file this.",
			"headers":{"from":"Sender <sender@example.com>","authentication-results":"mx.example; spf=pass; dkim=pass"},
			"bcc":[],
			"cc":[],
			"reply_to":[],
			"message_id":"<email-upload-2@example.com>",
			"attachments":[]
		}`))
	}))
	defer resend.Close()
	cfg.ResendBaseURL = resend.URL
	h := NewHandler(cfg, store)
	h.resend = newResendClient(cfg, resend.Client())

	body := `{"id":"evt-upload-2","type":"email.received","data":{"email_id":"email-upload-2"}}`
	req := signedResendRequest(t, cfg.WebhookSecret, "msg-upload-2", body)
	w := httptest.NewRecorder()
	h.HandleResendWebhook(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusAccepted, w.Body.String())
	}

	messages, err := store.ListMessages(req.Context(), "user-root", "inbox", 10)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(messages) != 1 || messages[0].Subject != "Trusted upload" {
		t.Fatalf("messages = %+v, want whitelisted trusted upload", messages)
	}
	if messages[0].TrustStatus != "trusted" {
		t.Fatalf("trust status = %q, want trusted", messages[0].TrustStatus)
	}
	var authResults map[string]string
	if err := json.Unmarshal([]byte(messages[0].AuthenticationResultsJSON), &authResults); err != nil {
		t.Fatalf("unmarshal authentication results: %v", err)
	}
	if authResults["authentication-results"] != "mx.example; spf=pass; dkim=pass" {
		t.Fatalf("authentication results = %+v", authResults)
	}
}

func TestHandleResendWebhookRejectsMutatedBody(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.WebhookSecret = "whsec_" + "dGVzdC1zZWNyZXQ="
	h := NewHandler(cfg, store)

	signedBody := `{"id":"evt-1","type":"email.received","data":{"email_id":"email-1"}}`
	req := signedResendRequest(t, cfg.WebhookSecret, "msg-1", signedBody)
	req.Body = ioNopCloser{strings.NewReader(`{"type":"email.received","id":"evt-1","data":{"email_id":"email-1"}}`)}

	w := httptest.NewRecorder()
	h.HandleResendWebhook(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestHandleResendWebhookRejectsMissingHeaders(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.WebhookSecret = "whsec_" + "dGVzdC1zZWNyZXQ="
	h := NewHandler(cfg, store)

	req := httptest.NewRequest(http.MethodPost, "/api/email/resend/webhook", strings.NewReader(`{"id":"evt-1","type":"email.received"}`))
	w := httptest.NewRecorder()
	h.HandleResendWebhook(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestHandleResendWebhookRejectsStaleTimestamp(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.WebhookSecret = "whsec_" + "dGVzdC1zZWNyZXQ="
	h := NewHandler(cfg, store)
	body := `{"id":"evt-stale","type":"domain.updated"}`

	req := signedResendRequestAt(t, cfg.WebhookSecret, "msg-stale", body, time.Now().UTC().Add(-10*time.Minute))
	w := httptest.NewRecorder()
	h.HandleResendWebhook(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "invalid_signature") {
		t.Fatalf("body = %s, want invalid_signature", w.Body.String())
	}
}

func seedTrustedUploadAlias(t *testing.T, store *Store, whitelist bool) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := store.db.Exec(`INSERT INTO email_receive_policies (
		id, name, allow_public_inbound, allow_attachments, require_sender_whitelist,
		require_secret_alias, allow_auto_agent_read, allow_auto_agent_write,
		allow_auto_outbound_send, quarantine_by_default, created_at
	) VALUES ('policy-trusted-upload-test', 'trusted upload test', 0, 1, 1, 1, 1, 0, 0, 1, ?)`, now); err != nil {
		t.Fatalf("insert trusted policy: %v", err)
	}
	if _, err := store.db.Exec(`INSERT INTO email_aliases (
		id, domain, local_part, canonical_number, target_type, target_id,
		visibility, receive_policy_id, created_at
	) VALUES ('alias-trusted-upload-test', 'choir.news', '000+upload-secret', 0, 'user', 'user-root', 'unlisted', 'policy-trusted-upload-test', ?)`, now); err != nil {
		t.Fatalf("insert trusted alias: %v", err)
	}
	if !whitelist {
		return
	}
	if _, err := store.db.Exec(`INSERT INTO email_sender_whitelist (
		id, owner_id, alias_id, sender_address, created_at
	) VALUES ('whitelist-trusted-upload-test', 'user-root', 'alias-trusted-upload-test', 'sender@example.com', ?)`, now); err != nil {
		t.Fatalf("insert sender whitelist: %v", err)
	}
}

type ioNopCloser struct {
	*strings.Reader
}

func (c ioNopCloser) Close() error { return nil }
