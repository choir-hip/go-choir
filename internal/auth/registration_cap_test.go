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
	if closed, err := h.registrationClosed("abandoned", "abandoned@mosiah.org"); err != nil || !closed {
		t.Fatalf("registrationClosed = %v, %v; want closed", closed, err)
	}
	if closed, err := h.registrationClosed("u1", "one@mosiah.org"); err != nil || closed {
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
	if closed, err := h.registrationClosed("", "x@mosiah.org"); err == nil || !closed {
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

// Test addresses (example.com, example.test: the CI and drill accounts, 908
// of the 925 passkey holders on Node B on 2026-11-23) are counted apart, so
// they neither fill nor bypass the owner's cap for strangers:
//  8. test addresses counting against the cap for people (CI would close
//     sign-up within days);
//  9. a test address bypassing the cap for people (an address nobody can
//     verify, so a stranger could use it): it has its own, separate cap;
//  10. no test domains configured leaving every address a person.

func TestRegistrationCapCountsTestAddressesApart(t *testing.T) {
	h, _ := testHandlerEnv(t)
	h.config.RegistrationCap = 2
	h.config.RegistrationTestCap = 3
	h.config.RegistrationTestDomains = []string{"example.com", "example.test"}
	addPasskey(t, h, "p1", "person@mosiah.org")
	addPasskey(t, h, "t1", "ci1@example.com")
	addPasskey(t, h, "t2", "ci2@Example.Test")

	// Two test accounts do not use the people's room.
	if rec := registerBegin(t, h, "second@mosiah.org"); rec.Code != http.StatusOK {
		t.Fatalf("a person below the cap: %d %s", rec.Code, rec.Body.String())
	}
	addPasskey(t, h, "p2", "second-done@mosiah.org")
	if rec := registerBegin(t, h, "third@mosiah.org"); rec.Code != http.StatusForbidden {
		t.Fatalf("a person at the cap: %d, want 403", rec.Code)
	}
	// A test address is bounded by its own cap, not by the people's.
	if rec := registerBegin(t, h, "ci3@example.com"); rec.Code != http.StatusOK {
		t.Fatalf("a test address below its cap: %d %s", rec.Code, rec.Body.String())
	}
	addPasskey(t, h, "t3", "ci3-done@example.com")
	rec := registerBegin(t, h, "ci4@example.com")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("a test address at its cap: %d, want 403", rec.Code)
	}
	if _, err := h.store.GetUserByEmail("ci4@example.com"); err == nil {
		t.Fatal("a refused test address created a user")
	}

	// No test domains: every address is a person.
	h.config.RegistrationTestDomains = nil
	h.config.RegistrationCap = 3
	if rec := registerBegin(t, h, "ci5@example.com"); rec.Code != http.StatusForbidden {
		t.Fatalf("with no test domains example.com counts as a person: %d, want 403", rec.Code)
	}
}

func TestRegistrationTestCapFromEnv(t *testing.T) {
	t.Setenv("AUTH_REGISTRATION_CAP", "40")
	t.Setenv("AUTH_REGISTRATION_TEST_CAP", "1100")
	t.Setenv("AUTH_REGISTRATION_TEST_DOMAINS", " Example.com, example.test ,")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RegistrationTestCap != 1100 || len(cfg.RegistrationTestDomains) != 2 ||
		cfg.RegistrationTestDomains[0] != "example.com" || cfg.RegistrationTestDomains[1] != "example.test" {
		t.Fatalf("config = %d %v", cfg.RegistrationTestCap, cfg.RegistrationTestDomains)
	}
	t.Setenv("AUTH_REGISTRATION_TEST_CAP", "-2")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("AUTH_REGISTRATION_TEST_CAP=-2 must be refused at load")
	}
}

// 11. the refusal at a full cap being unreadable by a person: the words are
// the owner's, and the body carries the code the site and the app read.
func TestFullSignUpIsSaidPlainly(t *testing.T) {
	h, _ := testHandlerEnv(t)
	h.config.RegistrationCap = 1
	addPasskey(t, h, "u1", "one@mosiah.org")
	rec := registerBegin(t, h, "late@mosiah.org")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("at the cap: %d, want 403", rec.Code)
	}
	if got, want := rec.Body.String(), `{"error":"Sign-up is full for now. You are on the list.","code":"registration_waitlist"}`; got != want+"\n" && got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}
