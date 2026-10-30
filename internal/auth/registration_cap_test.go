package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// L4-03 (docs/archive/definitions/choir-launch-fp8-image-jailer-2026-10-21.md, P7):
// a first cap on registered users, with a waitlist. Failure modes guarded,
// written before the code:
//  1. the cap on by default (AUTH_REGISTRATION_CAP unset or 0 is off);
//  2. a refused sign-up still creating a user row (it would count later and
//     its computer could be minted);
//  3. abandoned ceremonies (users with no passkey) filling the cap;
//  4. an incomplete registration slipping past a full cap by resuming;
//  5. many ceremonies begun below the cap all finishing above it (the
//     finish re-checks);
//  6. a count that cannot be read admitting everyone (fail closed);
//  7. the refusal losing the person (the waitlist keeps the email; a
//     repeat attempt counts, it does not duplicate).

func registerBegin(t *testing.T, h *Handler, email string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/auth/register/begin", bytes.NewBufferString(`{"email":"`+email+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.HandleRegisterBegin(rec, req)
	return rec
}

func addPasskey(t *testing.T, h *Handler, userID, email string) {
	t.Helper()
	if _, err := h.store.CreateUser(userID, email); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := h.store.CreateCredential(&Credential{
		ID: "cred-" + userID, UserID: userID, PublicKey: []byte("k"), AttestationType: "none",
		Transport: `["internal"]`, AAGUID: make([]byte, 16), Flags: "{}", CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("CreateCredential: %v", err)
	}
}

func TestRegistrationCapOffByDefault(t *testing.T) {
	h, _ := testHandlerEnv(t)
	if h.config.RegistrationCap != 0 {
		t.Fatalf("default cap = %d, want 0 (off)", h.config.RegistrationCap)
	}
	for i := 0; i < 3; i++ {
		addPasskey(t, h, "u"+string(rune('a'+i)), "u"+string(rune('a'+i))+"@mosiah.org")
	}
	if rec := registerBegin(t, h, "new@mosiah.org"); rec.Code != http.StatusOK {
		t.Fatalf("cap off refused a sign-up: %d %s", rec.Code, rec.Body.String())
	}
}

func TestRegistrationCapRefusesAndWaitlists(t *testing.T) {
	h, _ := testHandlerEnv(t)
	h.config.RegistrationCap = 2
	addPasskey(t, h, "u1", "one@mosiah.org")

	// Abandoned ceremonies do not count.
	if _, err := h.store.CreateUser("abandoned", "abandoned@mosiah.org"); err != nil {
		t.Fatal(err)
	}
	if rec := registerBegin(t, h, "two@mosiah.org"); rec.Code != http.StatusOK {
		t.Fatalf("below the cap: %d %s", rec.Code, rec.Body.String())
	}
	addPasskey(t, h, "u2", "two-done@mosiah.org")

	// At the cap: refused, no user row, waitlisted.
	rec := registerBegin(t, h, "Three@Mosiah.org")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("at the cap: %d %s, want 403", rec.Code, rec.Body.String())
	}
	var body map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body["code"] != registrationWaitlistCode || body["error"] == "" {
		t.Fatalf("refusal body = %v", body)
	}
	if _, err := h.store.GetUserByEmail("Three@Mosiah.org"); err == nil {
		t.Fatal("a refused sign-up created a user")
	}
	registerBegin(t, h, "three@mosiah.org")
	entries, err := h.store.ListRegistrationWaitlist()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Email != "three@mosiah.org" || entries[0].Attempts != 2 {
		t.Fatalf("waitlist = %+v, want one entry with two attempts", entries)
	}

	// An incomplete registration cannot resume past a full cap.
	if rec := registerBegin(t, h, "abandoned@mosiah.org"); rec.Code != http.StatusForbidden {
		t.Fatalf("incomplete registration resumed past the cap: %d", rec.Code)
	}
	// An existing user with a passkey still gets the usual answer.
	if rec := registerBegin(t, h, "one@mosiah.org"); rec.Code != http.StatusConflict {
		t.Fatalf("existing user: %d, want 409", rec.Code)
	}
	// The finish re-checks: a ceremony begun below the cap cannot finish above it.
	if closed, err := h.registrationClosed("abandoned"); err != nil || !closed {
		t.Fatalf("registrationClosed = %v, %v; want closed", closed, err)
	}
	if closed, err := h.registrationClosed("u1"); err != nil || closed {
		t.Fatalf("a user who already has a passkey is never closed out: %v, %v", closed, err)
	}
	// Raising the cap reopens.
	h.config.RegistrationCap = 3
	if rec := registerBegin(t, h, "three@mosiah.org"); rec.Code != http.StatusOK {
		t.Fatalf("after raising the cap: %d %s", rec.Code, rec.Body.String())
	}
}

func TestRegistrationCapFailsClosed(t *testing.T) {
	h, _ := testHandlerEnv(t)
	h.config.RegistrationCap = 10
	if err := h.store.db.Close(); err != nil {
		t.Fatal(err)
	}
	if closed, err := h.registrationClosed(""); err == nil || !closed {
		t.Fatalf("an unreadable count must close registration: %v, %v", closed, err)
	}
}

func TestRegistrationCapFromEnv(t *testing.T) {
	t.Setenv("AUTH_REGISTRATION_CAP", "")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RegistrationCap != 0 {
		t.Fatalf("unset = %d", cfg.RegistrationCap)
	}
	t.Setenv("AUTH_REGISTRATION_CAP", "150")
	if cfg, _ := LoadConfig(); cfg.RegistrationCap != 150 {
		t.Fatalf("150 = %d", cfg.RegistrationCap)
	}
	for _, bad := range []string{"-1", "x"} {
		t.Setenv("AUTH_REGISTRATION_CAP", bad)
		if _, err := LoadConfig(); err == nil {
			t.Fatalf("AUTH_REGISTRATION_CAP=%q must be refused at load", bad)
		}
	}
}
