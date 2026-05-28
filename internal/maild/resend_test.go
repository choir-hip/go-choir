package maild

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResendSendEmailSetsStableIdempotencyKey(t *testing.T) {
	cfg := &Config{
		ResendAPIKey:  "re_test",
		ResendBaseURL: "http://unused",
	}
	keys := []string{}
	resend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"sent-1"}`))
	}))
	defer resend.Close()
	cfg.ResendBaseURL = resend.URL
	client := newResendClient(cfg, resend.Client())
	payload := resendSendRequest{
		From:    "000@choir.news",
		To:      []string{"delivered@resend.dev"},
		Subject: "test",
		Text:    "same payload",
		Headers: map[string]any{"X-Choir-Maild": "v0-approved-draft-send"},
	}

	if _, err := client.sendEmail(context.Background(), payload); err != nil {
		t.Fatalf("send 1: %v", err)
	}
	if _, err := client.sendEmail(context.Background(), payload); err != nil {
		t.Fatalf("send 2: %v", err)
	}

	if len(keys) != 2 {
		t.Fatalf("keys = %+v", keys)
	}
	if keys[0] == "" || keys[0] != keys[1] {
		t.Fatalf("idempotency keys = %+v, want stable non-empty key", keys)
	}
	if len(keys[0]) > 256 {
		t.Fatalf("idempotency key length = %d, want <= 256", len(keys[0]))
	}
}
