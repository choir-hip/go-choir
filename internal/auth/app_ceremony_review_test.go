package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// R2: two live register/begin challenges for one email cannot both finish; the
// second finish is refused and mints no key and stores no credential. A
// recovery-flow challenge (the address was proved by mail) may add a passkey to
// an account that has one.
func TestSecondRegistrationFinishForOneEmailIsRefused(t *testing.T) {
	h := appEnv(t)
	a, b := newSoftPasskey(t), newSoftPasskey(t)
	beginA := decodeMap(t, postJSON(h.HandleRegisterBegin, map[string]string{"email": "alice@example.com"}))
	beginB := decodeMap(t, postJSON(h.HandleRegisterBegin, map[string]string{"email": "alice@example.com"}))
	if rec := postJSON(h.HandleAppRegisterFinish, appFinishBody(testApp, a.create(t, beginA, testOrigin))); rec.Code != 200 {
		t.Fatalf("first finish = %d: %s", rec.Code, rec.Body.String())
	}
	rec := postJSON(h.HandleAppRegisterFinish, appFinishBody(testApp, b.create(t, beginB, testOrigin)))
	if rec.Code != http.StatusConflict {
		t.Fatalf("second finish = %d, want 409: %s", rec.Code, rec.Body.String())
	}
	user, _ := h.store.GetUserByEmail("alice@example.com")
	creds, _ := h.store.GetCredentialsByUserID(user.ID)
	if len(creds) != 1 || countKeys(t, h, user.ID) != 1 {
		t.Fatalf("credentials %d, keys %d; want 1 and 1", len(creds), countKeys(t, h, user.ID))
	}
	// The web finish is the same check.
	beginC := decodeMap(t, postJSON(h.HandleRegisterBegin, map[string]string{"email": "web@example.com"}))
	beginD := decodeMap(t, postJSON(h.HandleRegisterBegin, map[string]string{"email": "web@example.com"}))
	if rec := postJSON(h.HandleRegisterFinish, newSoftPasskey(t).create(t, beginC, testOrigin)); rec.Code != 200 {
		t.Fatalf("web first finish = %d: %s", rec.Code, rec.Body.String())
	}
	if rec := postJSON(h.HandleRegisterFinish, newSoftPasskey(t).create(t, beginD, testOrigin)); rec.Code != http.StatusConflict {
		t.Fatalf("web second finish = %d, want 409", rec.Code)
	}
}

func TestRecoveryChallengeMayAddAPasskeyToAnAccountThatHasOne(t *testing.T) {
	h := appEnv(t)
	if rec := createAccount(t, h, "rec@example.com", newSoftPasskey(t), testOrigin); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	user, _ := h.store.GetUserByEmail("rec@example.com")
	creds, _ := h.store.GetCredentialsByUserID(user.ID)
	waUser, err := newWebAuthnUser(user, creds)
	if err != nil {
		t.Fatal(err)
	}
	creation, session, err := h.webauthn.BeginRegistration(waUser)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := jsonMarshal(session)
	now := time.Now().UTC()
	if err := h.store.SaveChallengeState(&ChallengeState{
		ID: session.Challenge, UserID: user.ID, Challenge: session.Challenge, Type: "registration",
		Source: ChallengeSourceRecovery, WebAuthnSessionData: string(raw), CreatedAt: now, ExpiresAt: now.Add(SessionChallengeTTL),
	}); err != nil {
		t.Fatal(err)
	}
	opts := decodeMap(t, jsonRecorder(t, creation))
	if rec := postJSON(h.HandleRegisterFinish, newSoftPasskey(t).create(t, opts, testOrigin)); rec.Code != 200 {
		t.Fatalf("recovery finish = %d: %s", rec.Code, rec.Body.String())
	}
	creds, _ = h.store.GetCredentialsByUserID(user.ID)
	if len(creds) != 2 {
		t.Fatalf("credentials = %d, want 2", len(creds))
	}
}

// Probed in review, now pinned: a user-verification-clear assertion or creation
// is refused, and a credential asserted under another user's handle is refused.
func TestAppCeremonyRefusesUnverifiedUsersAndSwappedUserHandles(t *testing.T) {
	h := appEnv(t)
	weak := newSoftPasskey(t)
	weak.noUV = true
	if rec := createAccount(t, h, "weak@example.com", weak, testOrigin); rec.Code == 200 {
		t.Fatal("a creation without user verification was accepted")
	}
	p, other := newSoftPasskey(t), newSoftPasskey(t)
	if rec := createAccount(t, h, "p@example.com", p, testOrigin); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if rec := createAccount(t, h, "other@example.com", other, testOrigin); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	p.noUV = true
	opts := decodeMap(t, postJSON(h.HandleAppLoginBegin, map[string]string{}))
	if rec := postJSON(h.HandleAppLoginFinish, appFinishBody(testApp, p.assert(t, opts, testOrigin))); rec.Code == 200 {
		t.Fatal("an assertion without user verification signed in")
	}
	p.noUV = false
	p.handle = other.handle
	opts = decodeMap(t, postJSON(h.HandleAppLoginBegin, map[string]string{}))
	if rec := postJSON(h.HandleAppLoginFinish, appFinishBody(testApp, p.assert(t, opts, testOrigin))); rec.Code == 200 {
		t.Fatal("a credential asserted under another account's handle signed in")
	}
}

// A key that cannot be minted after the passkey is stored answers 500 and
// leaves a passkey the person can sign in with once the store works again.
func TestAppRegisterFinishWhenTheKeyCannotBeMinted(t *testing.T) {
	h := appEnv(t)
	if _, err := h.store.db.Exec("ALTER TABLE api_keys RENAME TO api_keys_away"); err != nil {
		t.Fatal(err)
	}
	p := newSoftPasskey(t)
	rec := createAccount(t, h, "mint@example.com", p, testOrigin)
	if rec.Code != 500 || strings.Contains(rec.Body.String(), APIKeyPrefix) {
		t.Fatalf("finish = %d: %s", rec.Code, rec.Body.String())
	}
	if _, err := h.store.db.Exec("ALTER TABLE api_keys_away RENAME TO api_keys"); err != nil {
		t.Fatal(err)
	}
	opts := decodeMap(t, postJSON(h.HandleAppLoginBegin, map[string]string{}))
	if rec := postJSON(h.HandleAppLoginFinish, appFinishBody(testApp, p.assert(t, opts, testOrigin))); rec.Code != 200 {
		t.Fatalf("sign-in after the store came back = %d: %s", rec.Code, rec.Body.String())
	}
}

// The answers that carry a bearer key are never cached.
func TestAppKeyAnswersAreNoStore(t *testing.T) {
	h := appEnv(t)
	p := newSoftPasskey(t)
	rec := createAccount(t, h, "ns@example.com", p, testOrigin)
	if rec.Code != 200 || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("register finish: %d, Cache-Control %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
	opts := decodeMap(t, postJSON(h.HandleAppLoginBegin, map[string]string{}))
	fin := postJSON(h.HandleAppLoginFinish, appFinishBody(testApp, p.assert(t, opts, testOrigin)))
	if fin.Code != 200 || fin.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("login finish: %d, Cache-Control %q", fin.Code, fin.Header().Get("Cache-Control"))
	}
}

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }

func jsonRecorder(t *testing.T, v any) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	writeJSON(rec, 200, v)
	return rec
}
