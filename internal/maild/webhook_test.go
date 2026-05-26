package maild

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func signedResendRequest(t *testing.T, secret, msgID, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/email/resend/webhook", strings.NewReader(body))
	key, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_"))
	if err != nil {
		t.Fatalf("decode secret: %v", err)
	}
	ts := time.Now().UTC()
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

func TestHandleResendWebhookStoresVerifiedEventIdempotently(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.WebhookSecret = "whsec_" + "dGVzdC1zZWNyZXQ="
	h := NewHandler(cfg, store)
	body := `{"id":"evt-1","type":"email.received","data":{"email_id":"email-1"}}`

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

type ioNopCloser struct {
	*strings.Reader
}

func (c ioNopCloser) Close() error { return nil }
