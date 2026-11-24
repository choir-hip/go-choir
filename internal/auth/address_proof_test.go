package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// The invariant (docs/problems/passkey-claims-an-unproved-address-2026-11-24.md):
// no account is ever bound to an address that its holder has not proved, in the
// same ceremony; an account whose address is not proved mints nothing.

var proofSeq atomic.Int64

// nextTestIP is a fresh client address, so the per-address limits of the
// mailed code (10 begins and 30 verifies an hour) never join separate tests.
func nextTestIP() string {
	n := proofSeq.Add(1)
	return fmt.Sprintf("198.51.%d.%d", n>>8&255, n&255)
}

func testMailer(h *Handler) *fakeSignInMailer { return h.signInMailer.(*fakeSignInMailer) }

func newBinding(t *testing.T) string {
	t.Helper()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

// proofFields asks for the code and reads it from the mail. When the address
// was not mailed a code (an account exists: it got the recovery path) the code
// field is a wrong one, so the server's own refusal answers.
func proofFields(t *testing.T, h *Handler, email string) (map[string]string, *httptest.ResponseRecorder) {
	t.Helper()
	binding := newBinding(t)
	before := len(testMailer(h).mails())
	rec := postJSONFrom(h.HandleRegisterMail, nextTestIP(), map[string]string{"email": email, "binding": binding})
	if rec.Code != http.StatusOK {
		return nil, rec
	}
	id, _ := decodeMap(t, rec)["attempt_id"].(string)
	code := "00000000"
	if m := testMailer(h).mails(); len(m) > before {
		code = m[len(m)-1].Code
	}
	return map[string]string{"email": email, "attempt_id": id, "code": code, "binding": binding}, rec
}

// registerBeginFor runs the two steps and answers the second; when the first
// is refused (the cap, a bad address) it answers that refusal.
func registerBeginFor(t *testing.T, h *Handler, email string) *httptest.ResponseRecorder {
	t.Helper()
	fields, rec := proofFields(t, h, email)
	if fields == nil {
		return rec
	}
	return postJSONFrom(h.HandleRegisterBegin, nextTestIP(), fields)
}

// withProof adds a proof to a register/begin body whose address is new; a body
// that cannot get one (a bad address, a refused sign-up) is returned as it is.
func withProof(t *testing.T, h *Handler, body string) string {
	t.Helper()
	var in map[string]string
	if err := json.Unmarshal([]byte(body), &in); err != nil || in["email"] == "" {
		return body
	}
	fields, _ := proofFields(t, h, in["email"])
	if fields == nil {
		return body
	}
	out, _ := json.Marshal(fields)
	return string(out)
}

// A registration that names an address and carries no proof of the inbox is
// refused, and nothing exists for the address afterwards.
func TestRegisterBeginWithoutProofIsRefusedAndLeavesNothing(t *testing.T) {
	h := appEnv(t)
	rec := postJSON(h.HandleRegisterBegin, map[string]string{"email": "stranger@example.com"})
	if rec.Code != http.StatusBadRequest || decodeMap(t, rec)["code"] != registerProofRequiredCode {
		t.Fatalf("register/begin with no proof = %d %s", rec.Code, rec.Body.String())
	}
	// A wrong code, a code for another address and another client's binding are refused too.
	fields, _ := proofFields(t, h, "owner@example.com")
	for name, mut := range map[string]func(map[string]string){
		"wrong code":    func(f map[string]string) { f["code"] = "00000000" },
		"other address": func(f map[string]string) { f["email"] = "stranger@example.com" },
		"other binding": func(f map[string]string) { f["binding"] = newBindingNoT() },
	} {
		f := map[string]string{}
		for k, v := range fields {
			f[k] = v
		}
		mut(f)
		if rec := postJSONFrom(h.HandleRegisterBegin, nextTestIP(), f); rec.Code == http.StatusOK {
			t.Fatalf("%s: register/begin answered 200: %s", name, rec.Body.String())
		}
	}
	for _, e := range []string{"stranger@example.com", "owner@example.com"} {
		if _, err := h.store.GetUserByEmail(e); err == nil {
			t.Fatalf("a user row exists for %s before any passkey finished", e)
		}
	}
	var pending int
	_ = h.store.DB().QueryRow(`SELECT COUNT(*) FROM pending_registrations`).Scan(&pending)
	if pending != 0 {
		t.Fatalf("%d pending registrations from refused begins", pending)
	}
}

func newBindingNoT() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// The proof is single use, ten minutes, and works once: a reused or an expired
// code is refused.
func TestRegistrationProofIsSingleUseAndExpires(t *testing.T) {
	h := appEnv(t)
	fields, _ := proofFields(t, h, "once@example.com")
	if rec := postJSONFrom(h.HandleRegisterBegin, nextTestIP(), fields); rec.Code != http.StatusOK {
		t.Fatalf("first begin = %d %s", rec.Code, rec.Body.String())
	}
	if rec := postJSONFrom(h.HandleRegisterBegin, nextTestIP(), fields); rec.Code == http.StatusOK {
		t.Fatalf("a spent code was accepted again: %s", rec.Body.String())
	}
	old, _ := proofFields(t, h, "late@example.com")
	if _, err := h.store.DB().Exec(`UPDATE email_signin_attempts SET expires_at = ? WHERE id = ?`, time.Now().UTC().Add(-time.Minute), old["attempt_id"]); err != nil {
		t.Fatal(err)
	}
	if rec := postJSONFrom(h.HandleRegisterBegin, nextTestIP(), old); rec.Code == http.StatusOK {
		t.Fatalf("an expired code was accepted: %s", rec.Body.String())
	}
	// Five wrong tries kill the attempt, the right code included.
	live, _ := proofFields(t, h, "tries@example.com")
	for i := 0; i < EmailMaxWrongCodes; i++ {
		bad := map[string]string{"email": live["email"], "attempt_id": live["attempt_id"], "code": "00000000", "binding": live["binding"]}
		postJSONFrom(h.HandleRegisterBegin, nextTestIP(), bad)
	}
	if rec := postJSONFrom(h.HandleRegisterBegin, nextTestIP(), live); rec.Code == http.StatusOK {
		t.Fatal("an attempt that took five wrong codes still accepted the right one")
	}
}

// Of several finishes made from separate proofs for one address exactly one
// account exists; of one challenge finished twice exactly one finish wins.
func TestConcurrentFinishesForOneAddressMakeOneAccount(t *testing.T) {
	h := appEnv(t)
	for round := 0; round < 15; round++ {
		email := fmt.Sprintf("one%d@example.com", round)
		var bodies []any
		for i := 0; i < 4; i++ {
			begin := decodeMap(t, registerBeginFor(t, h, email))
			bodies = append(bodies, appFinishBody(testApp, newSoftPasskey(t).create(t, begin, testOrigin)))
		}
		codes := finishTogether(h.HandleAppRegisterFinish, bodies)
		var users, creds, keys int
		_ = h.store.DB().QueryRow(`SELECT COUNT(*) FROM users WHERE email = ?`, email).Scan(&users)
		_ = h.store.DB().QueryRow(`SELECT COUNT(*) FROM credentials WHERE user_id IN (SELECT id FROM users WHERE email = ?)`, email).Scan(&creds)
		_ = h.store.DB().QueryRow(`SELECT COUNT(*) FROM api_keys WHERE user_id IN (SELECT id FROM users WHERE email = ?)`, email).Scan(&keys)
		if countOf(codes, 200) != 1 || users != 1 || creds != 1 || keys != 1 {
			t.Fatalf("round %d: codes %v, users %d, credentials %d, keys %d; want one of each", round, codes, users, creds, keys)
		}
	}
	// One challenge, finished three times at once.
	begin := decodeMap(t, registerBeginFor(t, h, "same@example.com"))
	codes := finishTogether(h.HandleAppRegisterFinish, []any{
		appFinishBody(testApp, newSoftPasskey(t).create(t, begin, testOrigin)),
		appFinishBody(testApp, newSoftPasskey(t).create(t, begin, testOrigin)),
		appFinishBody(testApp, newSoftPasskey(t).create(t, begin, testOrigin)),
	})
	if countOf(codes, 200) != 1 {
		t.Fatalf("one challenge finished three times: %v, want exactly one 200", codes)
	}
}

// A finish whose pending row expired or was never made creates nothing.
func TestFinishWithoutAPendingRegistrationCreatesNothing(t *testing.T) {
	h := appEnv(t)
	begin := decodeMap(t, registerBeginFor(t, h, "expired@example.com"))
	if _, err := h.store.DB().Exec(`UPDATE pending_registrations SET expires_at = ?`, time.Now().UTC().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	rec := postJSON(h.HandleAppRegisterFinish, appFinishBody(testApp, newSoftPasskey(t).create(t, begin, testOrigin)))
	if rec.Code == http.StatusOK {
		t.Fatalf("an expired registration finished: %s", rec.Body.String())
	}
	if _, err := h.store.GetUserByEmail("expired@example.com"); err == nil {
		t.Fatal("an account exists for an expired registration")
	}
}

// A read error while checking the pending row fails closed.
func TestRegisterFinishFailsClosedWhenThePendingRowCannotBeRead(t *testing.T) {
	h := appEnv(t)
	begin := decodeMap(t, registerBeginFor(t, h, "closed@example.com"))
	body := appFinishBody(testApp, newSoftPasskey(t).create(t, begin, testOrigin))
	if _, err := h.store.DB().Exec(`DROP TABLE pending_registrations`); err != nil {
		t.Fatal(err)
	}
	if rec := postJSON(h.HandleAppRegisterFinish, body); rec.Code == http.StatusOK {
		t.Fatalf("finish succeeded with an unreadable pending table: %s", rec.Body.String())
	}
	if _, err := h.store.GetUserByEmail("closed@example.com"); err == nil {
		t.Fatal("an account was made")
	}
}

// An address that has an account is never registered again: asking for a code
// mails the recovery link, answers like any address, and a guessed code or a
// proof for another address binds nothing.
func TestAnAddressWithAnAccountGoesToRecoveryNotRegistration(t *testing.T) {
	h := appEnv(t)
	rm := &fakeRecoveryMailer{}
	h.recoveryMailer = rm
	h.recoveryMailDispatch = func(f func()) { f() }
	if rec := createAccount(t, h, "held@example.com", newSoftPasskey(t), testOrigin); rec.Code != 200 {
		t.Fatalf("setup: %s", rec.Body.String())
	}
	user, _ := h.store.GetUserByEmail("held@example.com")
	mailsBefore := len(testMailer(h).mails())
	fields, rec := proofFields(t, h, "held@example.com")
	if rec.Code != http.StatusOK || fields == nil {
		t.Fatalf("register/mail for an existing address = %d (it must answer like any address)", rec.Code)
	}
	if len(testMailer(h).mails()) != mailsBefore {
		t.Fatal("a registration code was mailed for an address that has an account")
	}
	if len(rm.mails()) != 1 {
		t.Fatalf("recovery mails = %d, want 1", len(rm.mails()))
	}
	if rec := postJSONFrom(h.HandleRegisterBegin, nextTestIP(), fields); rec.Code == http.StatusOK {
		t.Fatal("register/begin for an address with an account answered 200")
	}
	creds, _ := h.store.GetCredentialsByUserID(user.ID)
	if len(creds) != 1 {
		t.Fatalf("credentials = %d, want 1", len(creds))
	}
}

// A proved address with no passkey (an interrupted recovery) cannot be claimed
// by a stranger either: it is an account, so registration is refused and only
// the mail can add a passkey.
func TestAProvedAddressWithNoPasskeyCannotBeClaimed(t *testing.T) {
	h := appEnv(t)
	h.recoveryMailer = &fakeRecoveryMailer{}
	h.recoveryMailDispatch = func(f func()) { f() }
	if _, err := h.store.CreateUser("u-proved", "proved@example.com"); err != nil { // proved, no credential
		t.Fatal(err)
	}
	fields, rec := proofFields(t, h, "proved@example.com")
	if rec.Code != http.StatusOK {
		t.Fatalf("register/mail = %d", rec.Code)
	}
	if rec := postJSONFrom(h.HandleRegisterBegin, nextTestIP(), fields); rec.Code == http.StatusOK {
		t.Fatalf("a stranger began a registration for a proved address with no passkey: %s", rec.Body.String())
	}
	if creds, _ := h.store.GetCredentialsByUserID("u-proved"); len(creds) != 0 {
		t.Fatal("a passkey was attached without the mail")
	}
}

// An account whose address was never proved (a row from before this rule)
// cannot sign in on the app route or the web route, or mint a key; the
// recovery that proves the address replaces its passkeys, keys and sessions.
func TestAccountWithAnUnprovedAddressMintsNothingUntilMailRecoveryProvesIt(t *testing.T) {
	h := appEnv(t)
	h.recoveryMailer = &fakeRecoveryMailer{}
	h.recoveryMailDispatch = func(f func()) { f() }
	squatter := newSoftPasskey(t)
	if rec := createAccount(t, h, "legacy@example.com", squatter, testOrigin); rec.Code != 200 {
		t.Fatalf("setup: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := h.store.DB().Exec(`UPDATE users SET email_verified_at = NULL WHERE email = ?`, "legacy@example.com"); err != nil {
		t.Fatal(err)
	}
	user, _ := h.store.GetUserByEmail("legacy@example.com")
	keys := countKeys(t, h, user.ID)
	begin := decodeMap(t, postJSON(h.HandleAppLoginBegin, map[string]string{}))
	rec := postJSON(h.HandleAppLoginFinish, appFinishBody(testApp, squatter.assert(t, begin, testOrigin)))
	if rec.Code != http.StatusForbidden || decodeMap(t, rec)["code"] != addressUnprovedCode {
		t.Fatalf("an unproved account on the app route = %d %s", rec.Code, rec.Body.String())
	}
	if countKeys(t, h, user.ID) != keys {
		t.Fatal("a key was minted for an unproved account")
	}
	if _, err := h.mintAppKey(context.Background(), user, testApp); err == nil {
		t.Fatal("mintAppKey minted for an unproved account")
	}
	// The owner's mail proves the address; the recovery replaces the squatter's passkey.
	if _, err := h.store.RevokeUnprovedAccess(BootstrapAdminAPIKeyUserID); err != nil {
		t.Fatal(err)
	}
	var live int
	_ = h.store.DB().QueryRow(`SELECT COUNT(*) FROM api_keys WHERE user_id = ? AND revoked_at IS NULL`, user.ID).Scan(&live)
	if live != 0 {
		t.Fatalf("%d keys of an unproved account survive the start-up revoke", live)
	}
	owner := newSoftPasskey(t)
	token, err := h.store.CreateRecoveryToken(context.Background(), user.ID, "legacy@example.com", hashEmail("legacy@example.com"), hashIP("127.0.0.1"))
	if err != nil {
		t.Fatal(err)
	}
	vr := postJSON(h.HandleRecoveryVerify, map[string]string{"token": token})
	if vr.Code != 200 {
		t.Fatalf("recovery verify = %d %s", vr.Code, vr.Body.String())
	}
	// Verify alone proves nothing: an interrupted recovery leaves the account unproved and the squatter's passkey in place.
	if u, _ := h.store.GetUserByEmail("legacy@example.com"); u.EmailVerifiedAt != nil {
		t.Fatal("recovery/verify marked the address proved before any passkey was made")
	}
	if fr := postJSON(h.HandleRegisterFinish, owner.create(t, decodeMap(t, vr), testOrigin)); fr.Code != 200 {
		t.Fatalf("recovery finish = %d %s", fr.Code, fr.Body.String())
	}
	creds, _ := h.store.GetCredentialsByUserID(user.ID)
	if len(creds) != 1 {
		t.Fatalf("after recovery %d passkeys, want only the owner's", len(creds))
	}
	if u, _ := h.store.GetUserByEmail("legacy@example.com"); u.EmailVerifiedAt == nil {
		t.Fatal("the address is still unproved after the recovery finished")
	}
	// The squatter's passkey is gone; the owner's signs in.
	begin = decodeMap(t, postJSON(h.HandleAppLoginBegin, map[string]string{}))
	if rec := postJSON(h.HandleAppLoginFinish, appFinishBody(testApp, squatter.assert(t, begin, testOrigin))); rec.Code == 200 {
		t.Fatal("the squatter's passkey still signs in")
	}
	begin = decodeMap(t, postJSON(h.HandleAppLoginBegin, map[string]string{}))
	if rec := postJSON(h.HandleAppLoginFinish, appFinishBody(testApp, owner.assert(t, begin, testOrigin))); rec.Code != 200 {
		t.Fatalf("the owner cannot sign in after recovery: %d %s", rec.Code, rec.Body.String())
	}
}

// An interrupted recovery (the challenge is never finished) changes nothing: a
// second recovery still finds the account unproved and replaces the squatter.
func TestInterruptedRecoveryDoesNotDisableTheReplacement(t *testing.T) {
	h := appEnv(t)
	squatter := newSoftPasskey(t)
	if rec := createAccount(t, h, "retry@example.com", squatter, testOrigin); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	_, _ = h.store.DB().Exec(`UPDATE users SET email_verified_at = NULL WHERE email = ?`, "retry@example.com")
	user, _ := h.store.GetUserByEmail("retry@example.com")
	for i := 0; i < 2; i++ {
		token, _ := h.store.CreateRecoveryToken(context.Background(), user.ID, "retry@example.com", hashEmail("retry@example.com"), hashIP("127.0.0.1"))
		vr := postJSON(h.HandleRecoveryVerify, map[string]string{"token": token})
		if vr.Code != 200 {
			t.Fatalf("verify %d = %d", i, vr.Code)
		}
		if i == 0 {
			continue // the passkey sheet was cancelled
		}
		if fr := postJSON(h.HandleRegisterFinish, newSoftPasskey(t).create(t, decodeMap(t, vr), testOrigin)); fr.Code != 200 {
			t.Fatalf("finish = %d %s", fr.Code, fr.Body.String())
		}
	}
	if creds, _ := h.store.GetCredentialsByUserID(user.ID); len(creds) != 1 {
		t.Fatalf("%d passkeys, want only the owner's", len(creds))
	}
}

// An access token of an unproved account is refused by the auth handlers.
func TestAccessTokenOfAnUnprovedAccountIsRefused(t *testing.T) {
	h := appEnv(t)
	u, err := h.store.CreateUser("u-unproved", "unproved@example.com")
	if err != nil {
		t.Fatal(err)
	}
	tok, err := h.issueAccessJWT(u, AuthMethodPasskey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.store.DB().Exec(`UPDATE users SET email_verified_at = NULL WHERE id = ?`, u.ID); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/auth/api-keys", nil)
	req.AddCookie(&http.Cookie{Name: AccessTokenCookieName, Value: tok})
	rec := httptest.NewRecorder()
	h.HandleCreateAPIKey(rec, req)
	if rec.Code == http.StatusOK || rec.Code == http.StatusCreated {
		t.Fatalf("an unproved account's access token minted a key: %d", rec.Code)
	}
}

// One mailbox, one account, whatever case it was typed in.
func TestOneMailboxIsOneAccountWhateverTheCase(t *testing.T) {
	h := appEnv(t)
	h.recoveryMailer = &fakeRecoveryMailer{}
	h.recoveryMailDispatch = func(f func()) { f() }
	if rec := createAccount(t, h, "Case@Example.com", newSoftPasskey(t), testOrigin); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	before := len(testMailer(h).mails())
	fields, rec := proofFields(t, h, "case@example.com")
	if rec.Code != http.StatusOK || fields == nil {
		t.Fatalf("register/mail = %d", rec.Code)
	}
	if len(testMailer(h).mails()) != before {
		t.Fatal("a registration code was mailed for a mailbox that has an account (typed in another case)")
	}
	if rec := postJSONFrom(h.HandleRegisterBegin, nextTestIP(), fields); rec.Code == http.StatusOK {
		t.Fatal("register/begin worked for a mailbox that has an account")
	}
	// Even a pending registration that got past the mail cannot make a second account.
	err := h.store.CreateAccountWithCredential(&PendingRegistration{UserID: "dup", Email: "CASE@example.com"}, &Credential{ID: "dupc", PublicKey: []byte("k"), AttestationType: "none", Transport: "[]", AAGUID: make([]byte, 16), Flags: "{}", CreatedAt: time.Now().UTC()})
	if err != ErrAddressTaken {
		t.Fatalf("a second account for one mailbox: %v", err)
	}
}

// register/mail does not say whether the address has an account: the same
// statuses and the same words, request for request, past the limits and at the cap.
func TestRegisterMailAnswersAKnownAndANewAddressAlike(t *testing.T) {
	h := appEnv(t)
	h.recoveryMailer = &fakeRecoveryMailer{}
	h.recoveryMailDispatch = func(f func()) { f() }
	if rec := createAccount(t, h, "known@example.com", newSoftPasskey(t), testOrigin); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	run := func(email string) (codes []int, errs []string) {
		for i := 0; i < 8; i++ {
			rec := postJSONFrom(h.HandleRegisterMail, nextTestIP(), map[string]string{"email": email, "binding": newBinding(t)})
			codes = append(codes, rec.Code)
			var b map[string]any
			_ = json.Unmarshal(rec.Body.Bytes(), &b)
			s, _ := b["error"].(string)
			errs = append(errs, s)
		}
		return
	}
	// The known address has had one request already (its account was made through the proof).
	if _, rec := proofFields(t, h, "brand-new@example.com"); rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}
	kc, ke := run("known@example.com")
	nc, ne := run("brand-new@example.com")
	for i := range kc {
		if kc[i] != nc[i] || ke[i] != ne[i] {
			t.Fatalf("request %d: known %d %q, new %d %q", i, kc[i], ke[i], nc[i], ne[i])
		}
	}
	// At the cap both are refused the same way, and only the new one is waitlisted.
	h2 := appEnv(t)
	h2.config.RegistrationCap = 1
	addPasskey(t, h2, "u1", "held@example.com")
	a := postJSONFrom(h2.HandleRegisterMail, nextTestIP(), map[string]string{"email": "held@example.com", "binding": newBinding(t)})
	b := postJSONFrom(h2.HandleRegisterMail, nextTestIP(), map[string]string{"email": "fresh@example.com", "binding": newBinding(t)})
	if a.Code != http.StatusForbidden || a.Code != b.Code || a.Body.String() != b.Body.String() {
		t.Fatalf("at the cap: known %d %s, new %d %s", a.Code, a.Body.String(), b.Code, b.Body.String())
	}
	entries, _ := h2.store.ListRegistrationWaitlist()
	if len(entries) != 1 || entries[0].Email != "fresh@example.com" {
		t.Fatalf("waitlist = %+v, want only the new address", entries)
	}
}

func TestMailingRoutesWantJSON(t *testing.T) {
	h := appEnv(t)
	for name, f := range map[string]http.HandlerFunc{"register/mail": h.HandleRegisterMail, "register/begin": h.HandleRegisterBegin, "recovery/request": h.HandleRecoveryRequest} {
		req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"email":"a@example.com"}`))
		req.Header.Set("Content-Type", "text/plain")
		rec := httptest.NewRecorder()
		f(rec, req)
		if rec.Code != http.StatusUnsupportedMediaType {
			t.Fatalf("%s with text/plain = %d, want 415", name, rec.Code)
		}
	}
}

// Pending registrations nobody finished are swept, and never outlive five minutes in the table's growth.
func TestPendingRegistrationsAreSwept(t *testing.T) {
	h := appEnv(t)
	old := time.Now().UTC().Add(-time.Hour)
	if err := h.store.SavePendingRegistration(&PendingRegistration{Challenge: "old", Email: "a@example.com", UserID: "u", SessionData: "{}", CreatedAt: old, ExpiresAt: old.Add(5 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if n, err := h.store.CleanExpiredPendingRegistrations(); err != nil || n != 1 {
		t.Fatalf("sweep = %d, %v; want 1", n, err)
	}
	// A new begin also drops what has expired.
	_ = h.store.SavePendingRegistration(&PendingRegistration{Challenge: "old2", Email: "b@example.com", UserID: "u2", SessionData: "{}", CreatedAt: old, ExpiresAt: old.Add(5 * time.Minute)})
	_ = h.store.SavePendingRegistration(&PendingRegistration{Challenge: "new", Email: "c@example.com", UserID: "u3", SessionData: "{}", CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(5 * time.Minute)})
	var n int
	_ = h.store.DB().QueryRow(`SELECT COUNT(*) FROM pending_registrations`).Scan(&n)
	if n != 1 {
		t.Fatalf("pending rows = %d, want 1", n)
	}
}

// Recovery of an unproved account clears the old holder's data ROOT and browser
// sessions; recovery of a proved account only adds a passkey and takes nothing.
func TestRecoveryClearsTheOldHoldersRootOnlyWhenTheAddressWasNeverProved(t *testing.T) {
	h := appEnv(t)
	seed := func(id string) {
		t.Helper()
		if _, err := h.store.CreateUser(id, id+"@example.com"); err != nil {
			t.Fatal(err)
		}
		for _, q := range []string{
			`INSERT INTO owner_root_keys (owner_id, created_at) VALUES ('` + id + `', CURRENT_TIMESTAMP)`,
			`INSERT INTO owner_root_key_wraps (owner_id, credential_id, prf_salt, wrapped_root, created_at) VALUES ('` + id + `', x'01', x'02', x'03', CURRENT_TIMESTAMP)`,
			`INSERT INTO account_set (id, handle_hash, active_user_id, created_at, last_used_at) VALUES ('set-` + id + `', 'h-` + id + `', '` + id + `', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
			`INSERT INTO account_set_member (set_id, user_id, refresh_session_id, added_at) VALUES ('set-` + id + `', '` + id + `', 'rs', CURRENT_TIMESTAMP)`,
		} {
			if _, err := h.store.DB().Exec(q); err != nil {
				t.Fatal(err)
			}
		}
	}
	count := func(id string) (n int) {
		for _, q := range []string{`SELECT COUNT(*) FROM owner_root_keys WHERE owner_id = ?`, `SELECT COUNT(*) FROM owner_root_key_wraps WHERE owner_id = ?`, `SELECT COUNT(*) FROM account_set_member WHERE user_id = ?`} {
			var c int
			_ = h.store.DB().QueryRow(q, id).Scan(&c)
			n += c
		}
		return
	}
	cred := func(id string) *Credential {
		return &Credential{ID: "new-" + id, UserID: id, PublicKey: []byte("k"), AttestationType: "none", Transport: "[]", AAGUID: make([]byte, 16), Flags: "{}", CreatedAt: time.Now().UTC()}
	}
	seed("unproved")
	seed("proved")
	if _, err := h.store.DB().Exec(`UPDATE users SET email_verified_at = NULL WHERE id = 'unproved'`); err != nil {
		t.Fatal(err)
	}
	if err := h.store.AddRecoveryCredential(cred("unproved")); err != nil {
		t.Fatal(err)
	}
	if err := h.store.AddRecoveryCredential(cred("proved")); err != nil {
		t.Fatal(err)
	}
	if n := count("unproved"); n != 0 {
		t.Fatalf("an unproved account's root and account-set rows survive recovery: %d", n)
	}
	if n := count("proved"); n != 3 {
		t.Fatalf("recovery of a proved account took %d of its 3 root and account-set rows", 3-n)
	}
}

// A token made before the proof rule carries no proof claim and is refused, for a proved account too.
func TestAccessTokenWithoutTheProofClaimIsRefused(t *testing.T) {
	h := appEnv(t)
	u, _ := h.store.CreateUser("u-proved2", "proved2@example.com")
	good, err := h.issueAccessJWT(u, AuthMethodPasskey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.ValidateAccessToken(good); err != nil {
		t.Fatalf("a token the server just made is refused: %v", err)
	}
	legacy := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{"sub": u.ID, "iat": time.Now().Unix(), "exp": time.Now().Add(time.Minute).Unix(), "scope": "access"})
	tok, _ := legacy.SignedString(h.signer)
	if _, err := h.ValidateAccessToken(tok); err == nil {
		t.Fatal("a token with no proof claim was accepted")
	}
	if _, err := h.store.DB().Exec(`UPDATE users SET email_verified_at = NULL WHERE id = ?`, u.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.ValidateAccessToken(good); err == nil {
		t.Fatal("a token of an account that is no longer proved was accepted")
	}
	unproved, _ := h.store.GetUserByID(u.ID)
	if _, err := h.issueAccessJWT(unproved, AuthMethodPasskey); err == nil {
		t.Fatal("a token was made for an unproved account")
	}
}

// register/mail for an address that has an account makes a link of its own: it neither replaces the link
// /auth/recovery/request made nor spends that route's budget.
func TestRegisterMailForAKnownAddressLeavesTheRecoveryLinkAndBudgetAlone(t *testing.T) {
	h, rm := recoveryMailTestEnv(t)
	h.config.RPID, h.config.RPOrigins = testRP, []string{testOrigin}
	if _, err := h.store.CreateUser("holder", "holder@example.com"); err != nil {
		t.Fatal(err)
	}
	// A link the person asked for is in the inbox.
	if rec := postRecoveryRequest(t, h, "holder@example.com", "203.0.113.9:1"); rec.Code != 200 || len(rm.mails()) != 1 {
		t.Fatalf("setup: %d, %d mails", rec.Code, len(rm.mails()))
	}
	link := rm.mails()[0].link
	token := strings.TrimPrefix(link[strings.Index(link, "#")+1:], "token=")
	for i := 0; i < 4; i++ {
		if rec := postJSONFrom(h.HandleRegisterMail, nextTestIP(), map[string]string{"email": "holder@example.com", "binding": newBinding(t)}); rec.Code != 200 {
			t.Fatalf("register/mail %d = %d %s", i, rec.Code, rec.Body.String())
		}
	}
	if len(rm.mails()) != 2 {
		t.Fatalf("%d mails in all: want the direct link and one link of register/mail's own", len(rm.mails()))
	}
	if _, err := h.store.ConsumeRecoveryToken(context.Background(), token); err != nil {
		t.Fatalf("the link in the inbox was invalidated: %v", err)
	}
	for i := 0; i < RecoveryMaxPerEmail-1; i++ { // the setup request already spent one
		if rec := postRecoveryRequest(t, h, "holder@example.com", fmt.Sprintf("203.0.113.%d:1", 20+i)); rec.Code != 200 {
			t.Fatalf("recovery/request %d was refused (%d) after register/mail calls: its budget was spent", i, rec.Code)
		}
	}
}

// A recovery link that could not be mailed does not stand in for the retry: the next request mails again.
func TestAFailedRecoveryMailLeavesNoLiveLinkToSuppressTheRetry(t *testing.T) {
	h, rm := recoveryMailTestEnv(t)
	h.config.RPID, h.config.RPOrigins = testRP, []string{testOrigin}
	h.signInMailer = &fakeSignInMailer{}
	if _, err := h.store.CreateUser("held3", "held3@example.com"); err != nil {
		t.Fatal(err)
	}
	rm.err = errors.New("maild down")
	if rec := postJSONFrom(h.HandleRegisterMail, nextTestIP(), map[string]string{"email": "held3@example.com", "binding": newBinding(t)}); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("first try = %d, want 503", rec.Code)
	}
	rm.err = nil
	before := len(rm.mails())
	if rec := postJSONFrom(h.HandleRegisterMail, nextTestIP(), map[string]string{"email": "held3@example.com", "binding": newBinding(t)}); rec.Code != http.StatusOK || len(rm.mails()) != before+1 {
		t.Fatalf("retry = %d with %d new mails: a link nobody received suppressed it", rec.Code, len(rm.mails())-before)
	}
}

// A link register/mail made neither replaces nor is suppressed by the link /auth/recovery/request made.
func TestTheTwoRecoveryRoutesDoNotTouchEachOthersLinks(t *testing.T) {
	h, rm := recoveryMailTestEnv(t)
	h.config.RPID, h.config.RPOrigins = testRP, []string{testOrigin}
	if _, err := h.store.CreateUser("held4", "held4@example.com"); err != nil {
		t.Fatal(err)
	}
	if rec := postJSONFrom(h.HandleRegisterMail, nextTestIP(), map[string]string{"email": "held4@example.com", "binding": newBinding(t)}); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	regLink := rm.mails()[0].link
	regToken := strings.TrimPrefix(regLink[strings.Index(regLink, "#")+1:], "token=")
	if rec := postRecoveryRequest(t, h, "held4@example.com", "203.0.113.50:1"); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if len(rm.mails()) != 2 {
		t.Fatalf("the direct route was suppressed by the other's live link: %d mails", len(rm.mails()))
	}
	if _, err := h.store.ConsumeRecoveryToken(context.Background(), regToken); err != nil {
		t.Fatalf("the direct route replaced the link register/mail made: %v", err)
	}
}

// A mail that could not be sent is an error the page shows, for a new address and a known one.
func TestRegisterMailSendFailureIsAnErrorNotACodeThatNeverComes(t *testing.T) {
	h, rm := recoveryMailTestEnv(t)
	h.config.RPID, h.config.RPOrigins = testRP, []string{testOrigin}
	h.signInMailer = &fakeSignInMailer{err: errors.New("maild down")}
	if _, err := h.store.CreateUser("held2", "held2@example.com"); err != nil {
		t.Fatal(err)
	}
	rm.err = errors.New("maild down")
	for _, email := range []string{"fresh2@example.com", "held2@example.com"} {
		rec := postJSONFrom(h.HandleRegisterMail, nextTestIP(), map[string]string{"email": email, "binding": newBinding(t)})
		if rec.Code != http.StatusServiceUnavailable || strings.Contains(rec.Body.String(), "attempt_id") {
			t.Fatalf("%s: send failure answered %d %s", email, rec.Code, rec.Body.String())
		}
	}
}

// An address pasted with spaces finds its account on the recovery and login routes.
func TestRecoveryRequestAndLoginBeginTrimTheAddress(t *testing.T) {
	h, rm := recoveryMailTestEnv(t)
	if _, err := h.store.CreateUser("trim", "trim@example.com"); err != nil {
		t.Fatal(err)
	}
	if rec := postRecoveryRequest(t, h, "  trim@example.com ", "203.0.113.30:1"); rec.Code != 200 || len(rm.mails()) != 1 {
		t.Fatalf("recovery with a padded address: %d, %d mails", rec.Code, len(rm.mails()))
	}
	rec := postJSONFrom(h.HandleLoginBegin, nextTestIP(), map[string]string{"email": "  trim@example.com "})
	if rec.Code == http.StatusNotFound && strings.Contains(rec.Body.String(), "user not found") {
		t.Fatal("login/begin did not find the account of a padded address")
	}
}

// An account whose address is not proved cannot be switched to: the answer is "signed out", not an
// error, and nothing is issued.
func TestSwitchingToAnUnprovedAccountIsRefusedAsSignedOut(t *testing.T) {
	h := appEnv(t)
	u, err := h.store.CreateUser("u-switch", "switch@example.com")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := h.store.CreateRefreshSession(&RefreshSession{ID: "rs-switch", UserID: u.ID, TokenHash: sha256SumHex("switch-refresh"), CreatedAt: now, ExpiresAt: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	handle := "switch-handle-switch-handle-switch-handle"
	if _, _, err := h.store.AddAccountToSet("", sha256SumHex(handle), u.ID, "rs-switch"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.store.DB().Exec(`UPDATE users SET email_verified_at = NULL WHERE id = ?`, u.ID); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/auth/accounts/switch", strings.NewReader(`{"user_id":"u-switch"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: AccountsCookieName, Value: handle})
	rec := httptest.NewRecorder()
	h.HandleAccountSwitch(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("switch to an unproved account = %d %s, want 409 (signed out)", rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == AccessTokenCookieName && c.Value != "" {
			t.Fatal("an access token was issued for an unproved account")
		}
	}
}

// Three requests from a stranger do not lock the owner out: while a link is live a new request makes no new
// link (the same 200, no mail), so the owner's link in the inbox stays good and the budget is not spent.
func TestDirectRecoveryRequestsDoNotSupersedeALiveLink(t *testing.T) {
	h, rm := recoveryMailTestEnv(t)
	if _, err := h.store.CreateUser("held5", "held5@example.com"); err != nil {
		t.Fatal(err)
	}
	if rec := postRecoveryRequest(t, h, "held5@example.com", "203.0.113.60:1"); rec.Code != 200 || len(rm.mails()) != 1 {
		t.Fatalf("the owner's request: %d, %d mails", rec.Code, len(rm.mails()))
	}
	link := rm.mails()[0].link
	token := strings.TrimPrefix(link[strings.Index(link, "#")+1:], "token=")
	for i := 0; i < 5; i++ {
		if rec := postRecoveryRequest(t, h, "held5@example.com", fmt.Sprintf("203.0.113.%d:1", 61+i)); rec.Code != 200 {
			t.Fatalf("a stranger's request %d answered %d, want the same 200", i, rec.Code)
		}
	}
	if len(rm.mails()) != 1 {
		t.Fatalf("%d mails: a live link was replaced", len(rm.mails()))
	}
	if _, err := h.store.ConsumeRecoveryToken(context.Background(), token); err != nil {
		t.Fatalf("the owner's link was invalidated by strangers: %v", err)
	}
	var rows int
	if err := h.store.DB().QueryRow(`SELECT COUNT(*) FROM recovery_tokens WHERE user_id = 'held5'`).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("recovery rows = %d (%v), want the one link", rows, err)
	}
}

// A known and an unknown address are answered alike however fast the requests come: the live-link rule applies to both.
func TestRecoveryRequestAnswersAKnownAndAnUnknownAddressAlike(t *testing.T) {
	h, _ := recoveryMailTestEnv(t)
	if _, err := h.store.CreateUser("held6", "held6@example.com"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		k := postRecoveryRequest(t, h, "held6@example.com", fmt.Sprintf("203.0.113.%d:1", 80+i))
		u := postRecoveryRequest(t, h, "nobody6@example.com", fmt.Sprintf("203.0.113.%d:1", 120+i))
		if k.Code != u.Code || k.Body.String() != u.Body.String() {
			t.Fatalf("request %d: known %d %s, unknown %d %s", i, k.Code, k.Body.String(), u.Code, u.Body.String())
		}
	}
}
