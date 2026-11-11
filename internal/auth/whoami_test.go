package auth

// Failure list: whoami.go (W1-W5).

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWhoamiAnswersTheBearersOwnRecordOnly(t *testing.T) {
	h, priv := testHandlerEnv(t)
	ctx := context.Background()
	user, err := h.store.CreateUser("whoami-owner", "whoami-owner@example.com")
	if err != nil {
		t.Fatal(err)
	}
	keyID, secret, err := h.store.CreateComputerScopedAPIKey(ctx, user.ID, "phone", []string{"read:base"}, "computer-a", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := h.store.CreateComputerScopedAPIKey(ctx, user.ID, "sibling-secret-label", []string{"admin"}, "computer-b", nil); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	h.HandleWhoami(rec, bearerReq(http.MethodGet, "/auth/whoami", nil, secret))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, leak := range []string{"example.com", "whoami-owner@", "sibling-secret-label", "computer-b", "admin"} { // W2
		if strings.Contains(body, leak) {
			t.Fatalf("W2: answer leaks %q: %s", leak, body)
		}
	}
	if strings.Contains(body, user.ID) || strings.Contains(body, "user_id") { // W7
		t.Fatalf("W7: user id in answer: %s", body)
	}
	for _, want := range []string{keyID, "computer-a", PublicHandle(user.ID), "phone"} {
		if !strings.Contains(body, want) {
			t.Fatalf("answer lacks %q: %s", want, body)
		}
	}
	if !strings.HasPrefix(PublicHandle(user.ID), "writer-") || PublicHandle(user.ID) != PublicHandle(" "+user.ID+" ") {
		t.Fatalf("handle = %q", PublicHandle(user.ID))
	}
	if rec.Header().Get("Cache-Control") != "no-store" { // W5
		t.Fatalf("cache-control = %q", rec.Header().Get("Cache-Control"))
	}

	// W1: a cookie session, no credential, a wrong key, a revoked key.
	rec = httptest.NewRecorder()
	h.HandleWhoami(rec, authedAPIKeyReq(http.MethodGet, "/auth/whoami", nil, priv, user.ID))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("W1: cookie session status=%d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.HandleWhoami(rec, httptest.NewRequest(http.MethodGet, "/auth/whoami", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("W1: anonymous status=%d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.HandleWhoami(rec, bearerReq(http.MethodGet, "/auth/whoami", nil, "choir_sk_nope"))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("W1: wrong key status=%d", rec.Code)
	}
	if err := h.store.RevokeAPIKey(ctx, user.ID, keyID); err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	h.HandleWhoami(rec, bearerReq(http.MethodGet, "/auth/whoami", nil, secret))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("W1: revoked key status=%d", rec.Code)
	}

	// An expired key.
	past := time.Now().Add(-time.Hour)
	_, expired, err := h.store.CreateAPIKey(ctx, user.ID, "old", []string{"read:base"}, &past)
	if err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	h.HandleWhoami(rec, bearerReq(http.MethodGet, "/auth/whoami", nil, expired))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("W1: expired key status=%d", rec.Code)
	}

	// W4: only GET.
	rec = httptest.NewRecorder()
	h.HandleWhoami(rec, bearerReq(http.MethodPost, "/auth/whoami", nil, secret))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("W4: POST status=%d", rec.Code)
	}
}

func TestWhoamiTakesTheBearerNotTheCookie(t *testing.T) { // W6
	h, priv := testHandlerEnv(t)
	user, err := h.store.CreateUser("whoami-cookie", "whoami-cookie@example.com")
	if err != nil {
		t.Fatal(err)
	}
	keyID, secret, err := h.store.CreateAPIKey(context.Background(), user.ID, "phone", []string{"read:base"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	req := authedAPIKeyReq(http.MethodGet, "/auth/whoami", nil, priv, user.ID)
	req.Header.Set("Authorization", "Bearer choir_sk_bogus")
	rec := httptest.NewRecorder()
	h.HandleWhoami(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("cookie + bogus bearer status=%d body=%s", rec.Code, rec.Body.String())
	}
	req = authedAPIKeyReq(http.MethodGet, "/auth/whoami", nil, priv, user.ID)
	req.Header.Set("Authorization", "Bearer "+secret)
	rec = httptest.NewRecorder()
	h.HandleWhoami(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), keyID) {
		t.Fatalf("cookie + valid bearer status=%d body=%s", rec.Code, rec.Body.String())
	}
}
