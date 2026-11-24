package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
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
	if closed, err := h.registrationClosed("abandoned", false); err != nil || !closed {
		t.Fatalf("registrationClosed = %v, %v; want closed", closed, err)
	}
	if closed, err := h.registrationClosed("u1", false); err != nil || closed {
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
	if closed, err := h.registrationClosed("", false); err == nil || !closed {
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

// What binds is computers, and a computer is made for each account that
// holds a passkey. The cap counts those accounts, except the operator's own
// (CI and drills), and the operator's mark is a secret the server checks, not
// an address a stranger can type (review of 2026-11-24, B1):
//  8. an address on a test domain (example.com) counting as exempt: it is a
//     person's like any other;
//  9. a wrong, empty or absent operator secret exempting anyone, and an empty
//     configured secret making an empty header an operator;
//  10. an operator account filling the room for people;
//  11. count and insert racing: many finishes at cap-1 all landing;
//  12. the same address with spaces or a different case in the domain
//      slipping past or splitting the count.

const testOperatorSecret = "operator-secret-0123456789-0123456789-abcdef"

func registerBeginAs(t *testing.T, h *Handler, email, secret string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/auth/register/begin", bytes.NewBufferString(`{"email":"`+email+`"}`))
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		req.Header.Set(OperatorHeader, secret)
	}
	rec := httptest.NewRecorder()
	h.HandleRegisterBegin(rec, req)
	return rec
}

func TestATestAddressIsAPersonAtTheCap(t *testing.T) {
	h, _ := testHandlerEnv(t)
	h.config.RegistrationCap = 2
	h.config.OperatorSecret = testOperatorSecret
	addPasskey(t, h, "p1", "one@mosiah.org")
	addPasskey(t, h, "p2", "ci1@example.com")
	for _, email := range []string{"x@example.com", "x@example.test", "x@EXAMPLE.COM", " x@example.com "} {
		if rec := registerBegin(t, h, email); rec.Code != http.StatusForbidden {
			t.Fatalf("%q at the cap: %d, want 403 (a typed test address must not be exempt)", email, rec.Code)
		}
	}
}

func TestOnlyTheOperatorSecretExemptsAnAccount(t *testing.T) {
	h, _ := testHandlerEnv(t)
	h.config.RegistrationCap = 1
	h.config.OperatorSecret = testOperatorSecret
	addPasskey(t, h, "p1", "one@mosiah.org")
	for _, secret := range []string{"wrong", testOperatorSecret + "x", testOperatorSecret[:10]} {
		if rec := registerBeginAs(t, h, "ci@example.com", secret); rec.Code != http.StatusForbidden {
			t.Fatalf("secret %q: %d, want 403", secret, rec.Code)
		}
	}
	if rec := registerBeginAs(t, h, "ci@example.com", testOperatorSecret); rec.Code != http.StatusOK {
		t.Fatalf("the operator secret: %d %s", rec.Code, rec.Body.String())
	}
	u, err := h.store.GetUserByEmail("ci@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if marked, err := h.store.IsOperatorAccount(u.ID); err != nil || !marked {
		t.Fatalf("operator account marked = %v, %v", marked, err)
	}
	// With no secret configured, the header means nothing, even empty.
	h.config.OperatorSecret = ""
	if rec := registerBeginAs(t, h, "ci2@example.com", "anything"); rec.Code != http.StatusForbidden {
		t.Fatalf("no secret configured: %d, want 403", rec.Code)
	}
}

func TestOperatorAccountsDoNotFillTheRoomForPeople(t *testing.T) {
	h, _ := testHandlerEnv(t)
	h.config.RegistrationCap = 2
	h.config.OperatorSecret = testOperatorSecret
	for i, id := range []string{"o1", "o2", "o3"} {
		addPasskey(t, h, id, id+"@example.com")
		if err := h.store.MarkOperatorAccount(id); err != nil {
			t.Fatal(err, i)
		}
	}
	addPasskey(t, h, "p1", "one@mosiah.org")
	if rec := registerBegin(t, h, "two@mosiah.org"); rec.Code != http.StatusOK {
		t.Fatalf("a person with three operator accounts present: %d %s", rec.Code, rec.Body.String())
	}
	n, err := h.store.CountRegisteredUsers()
	if err != nil || n != 1 {
		t.Fatalf("counted = %d, %v; want 1", n, err)
	}
}

func TestFinishAtTheCapIsAtomic(t *testing.T) {
	h, _ := testHandlerEnv(t)
	const cap = 5
	for i := 0; i < cap-1; i++ {
		addPasskey(t, h, "p"+string(rune('a'+i)), "p"+string(rune('a'+i))+"@mosiah.org")
	}
	const racers = 12
	for i := 0; i < racers; i++ {
		if _, err := h.store.CreateUser("r"+string(rune('a'+i)), "r"+string(rune('a'+i))+"@mosiah.org"); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	var landed, refused int32
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			err := h.store.CreateCredentialWithinCap(&Credential{
				ID: "cred-" + id, UserID: id, PublicKey: []byte("k"), AttestationType: "none",
				Transport: `["internal"]`, AAGUID: make([]byte, 16), Flags: "{}", CreatedAt: time.Now().UTC(),
			}, cap)
			switch {
			case err == nil:
				atomic.AddInt32(&landed, 1)
			case errors.Is(err, ErrRegistrationClosed):
				atomic.AddInt32(&refused, 1)
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}("r" + string(rune('a'+i)))
	}
	wg.Wait()
	if landed != 1 || refused != racers-1 {
		t.Fatalf("landed %d, refused %d; want 1 and %d", landed, refused, racers-1)
	}
	if n, _ := h.store.CountRegisteredUsers(); n != cap {
		t.Fatalf("count = %d, want %d", n, cap)
	}
}

func TestAddressesAreNormalisedOnce(t *testing.T) {
	h, _ := testHandlerEnv(t)
	h.config.RegistrationCap = 3
	if rec := registerBegin(t, h, "  Pat@Mosiah.ORG "); rec.Code != http.StatusOK {
		t.Fatalf("spaced address: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := h.store.GetUserByEmail("Pat@mosiah.org"); err != nil {
		t.Fatalf("the stored address is not the normalised one: %v", err)
	}
	// The same address typed another way resumes the one row; it is not a second user.
	if rec := registerBegin(t, h, "Pat@mosiah.org  "); rec.Code != http.StatusOK {
		t.Fatalf("second spelling: %d %s", rec.Code, rec.Body.String())
	}
	var n int
	if err := h.store.DB().QueryRow("SELECT COUNT(*) FROM users WHERE lower(email) = 'pat@mosiah.org'").Scan(&n); err != nil || n != 1 {
		t.Fatalf("rows for one address = %d, %v", n, err)
	}
	if got := normalizeEmail("  A@B.Com "); got != "A@b.com" {
		t.Fatalf("normalizeEmail = %q", got)
	}
}

func TestOperatorSecretFromEnvFailsClosed(t *testing.T) {
	dir := t.TempDir()
	short := filepath.Join(dir, "short")
	good := filepath.Join(dir, "good")
	if err := os.WriteFile(short, []byte("tiny\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(good, []byte(testOperatorSecret+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AUTH_OPERATOR_SECRET_FILE", good)
	cfg, err := LoadConfig()
	if err != nil || cfg.OperatorSecret != testOperatorSecret {
		t.Fatalf("good file: %q, %v", cfg.OperatorSecret, err)
	}
	for _, bad := range []string{short, filepath.Join(dir, "missing")} {
		t.Setenv("AUTH_OPERATOR_SECRET_FILE", bad)
		if _, err := LoadConfig(); err == nil {
			t.Fatalf("AUTH_OPERATOR_SECRET_FILE=%s must refuse to start", bad)
		}
	}
	t.Setenv("AUTH_OPERATOR_SECRET_FILE", "")
	if cfg, err := LoadConfig(); err != nil || cfg.OperatorSecret != "" {
		t.Fatalf("unset: %q, %v", cfg.OperatorSecret, err)
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
