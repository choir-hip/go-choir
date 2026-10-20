package maild

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Failure mode (problem 2 in docs/problems/autopaper-daily-digest-phase1-
// 2026-10-20.md, design M1): the Resend idempotency key hashes the payload, so
// a retry whose body changed (the front list moved between attempts) gets a
// different key and the provider cannot deduplicate it. An explicit key names
// the send, not its content; sends without one keep the payload hash.
func TestResendSendEmailExplicitIdempotencyKeyOverridesPayloadHash(t *testing.T) {
	var keys []string
	resend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"sent-1"}`))
	}))
	defer resend.Close()
	client := newResendClient(&Config{ResendAPIKey: "re_test", ResendBaseURL: resend.URL}, resend.Client())
	payload := resendSendRequest{From: "a@choir.news", To: []string{"b@example.com"}, Subject: "s", Text: "one"}
	payload.idempotencyKey = "choir_digest_owner_2026-10-20"
	if _, err := client.sendEmail(context.Background(), payload); err != nil {
		t.Fatal(err)
	}
	payload.Text = "two, a different body"
	if _, err := client.sendEmail(context.Background(), payload); err != nil {
		t.Fatal(err)
	}
	payload.idempotencyKey = ""
	if _, err := client.sendEmail(context.Background(), payload); err != nil {
		t.Fatal(err)
	}
	if keys[0] != "choir_digest_owner_2026-10-20" || keys[1] != keys[0] {
		t.Fatalf("explicit keys = %q, %q, want the day key twice", keys[0], keys[1])
	}
	if !strings.HasPrefix(keys[2], "choir_maild_") {
		t.Fatalf("default key = %q, want the payload hash", keys[2])
	}
}
