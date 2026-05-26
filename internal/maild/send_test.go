package maild

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleSendRequiresOwnedFromAliasAndStoresSentMessage(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.ResendAPIKey = "re_test"
	resend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/emails" {
			t.Fatalf("%s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer re_test" {
			t.Fatalf("Authorization = %q", got)
		}
		var payload resendSendRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.From != "000@choir.news" || len(payload.To) != 1 || payload.To[0] != "friend@example.com" {
			t.Fatalf("payload = %+v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"sent-1"}`))
	}))
	defer resend.Close()
	cfg.ResendBaseURL = resend.URL
	h := NewHandler(cfg, store)
	h.resend = newResendClient(cfg, resend.Client())

	req := httptest.NewRequest(http.MethodPost, "/api/email/send", strings.NewReader(`{
		"from_address":"000@choir.news",
		"to_addresses":["friend@example.com"],
		"subject":"Re: project",
		"text_body":"Received."
	}`))
	req.Header.Set("X-Authenticated-User", "user-root")
	w := httptest.NewRecorder()
	h.HandleSend(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusAccepted, w.Body.String())
	}
	messages, err := store.ListMessages(req.Context(), "user-root", "sent", 10)
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("sent messages = %+v", messages)
	}
	if messages[0].Direction != "outbound" || messages[0].Subject != "Re: project" || messages[0].TrustStatus != "trusted" {
		t.Fatalf("sent message = %+v", messages[0])
	}
}

func TestHandleSendRejectsUnownedFromAlias(t *testing.T) {
	store, cfg := newTestStore(t)
	h := NewHandler(cfg, store)
	req := httptest.NewRequest(http.MethodPost, "/api/email/send", strings.NewReader(`{
		"from_address":"000@choir.news",
		"to_addresses":["friend@example.com"],
		"subject":"Nope",
		"text_body":"Nope."
	}`))
	req.Header.Set("X-Authenticated-User", "other-user")
	w := httptest.NewRecorder()
	h.HandleSend(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusForbidden, w.Body.String())
	}
}

func TestResendSendEmailReturnsBoundedProviderError(t *testing.T) {
	cfg := &Config{
		ResendAPIKey:  "re_test",
		ResendBaseURL: "http://unused",
	}
	longDetail := strings.Repeat("x", maxProviderErrorDetail+50)
	resend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, longDetail, http.StatusForbidden)
	}))
	defer resend.Close()
	cfg.ResendBaseURL = resend.URL
	client := newResendClient(cfg, resend.Client())

	_, err := client.sendEmail(context.Background(), resendSendRequest{
		From:    "000@choir.news",
		To:      []string{"delivered@resend.dev"},
		Subject: "test",
		Text:    "test",
	})
	if err == nil {
		t.Fatalf("sendEmail error = nil")
	}
	var providerErr *resendHTTPError
	if !errors.As(err, &providerErr) {
		t.Fatalf("error type = %T, want *resendHTTPError", err)
	}
	if providerErr.StatusCode != http.StatusForbidden {
		t.Fatalf("StatusCode = %d, want %d", providerErr.StatusCode, http.StatusForbidden)
	}
	if len(providerErr.Detail) <= maxProviderErrorDetail {
		t.Fatalf("Detail length = %d, want bounded detail with ellipsis", len(providerErr.Detail))
	}
	if len(providerErr.Detail) > maxProviderErrorDetail+3 {
		t.Fatalf("Detail length = %d, want <= %d", len(providerErr.Detail), maxProviderErrorDetail+3)
	}
}
