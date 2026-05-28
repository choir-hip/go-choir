package maild

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleCompletionEmailSendsConciseNotification(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.ResendAPIKey = "re_test"
	var payload resendSendRequest
	resend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/emails" {
			t.Fatalf("%s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"notice-1"}`))
	}))
	defer resend.Close()
	cfg.ResendBaseURL = resend.URL
	h := NewHandler(cfg, store)
	h.resend = newResendClient(cfg, resend.Client())

	req := httptest.NewRequest(http.MethodPost, "/api/notifications/completion-email", strings.NewReader(`{
		"to_email":"owner@example.com",
		"title":"Inbox assistant",
		"status":"verified",
		"feature_id":"package-secret-id",
		"link":"/?app=features"
	}`))
	setInternalOwner(req, "user-root")
	w := httptest.NewRecorder()
	h.HandleCompletionEmail(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusAccepted, w.Body.String())
	}
	if payload.From != "Choir <updates@choir.news>" || len(payload.To) != 1 || payload.To[0] != "owner@example.com" {
		t.Fatalf("payload address fields = %+v", payload)
	}
	if !strings.Contains(payload.Subject, "Inbox assistant") || !strings.Contains(payload.Text, "Status: verified") {
		t.Fatalf("payload missing concise title/status: %+v", payload)
	}
	if strings.Contains(payload.Text, "package-secret-id") || strings.Contains(payload.Text, "user-root") {
		t.Fatalf("payload leaked raw ids: %q", payload.Text)
	}
}
