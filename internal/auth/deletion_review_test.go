package auth

// The findings of the two review seats on lane account-delete (seat 1 #2 #3,
// seat 2 B2 S1 S2 S3 S7 S8 S12 S14 and the reviewer account). Failure list,
// written before the fixes; each test names its line.
//
//  V1 /auth/session answers "authenticated" for a deleted account while its
//     access token lives (seat 1 #2, seat 2 S3).
//  V2 account recovery, a registration finish or a recovery challenge adds a
//     passkey to a pending account, and the new passkey can then cancel the
//     deletion (seat 1 #3). Mail-proved recovery must never cancel.
//  V3 a purging account is offered a cancel that does not exist (S1).
//  V4 a cancel is accepted after purge_after (S7); an unknown app after a
//     successful cancel fails the call but not the cancel (S8); a double
//     cancel panics (S2).
//  V5 a cancel leaves the record where the runner can still suspend takes
//     after the cancel restored them (B2): the cancel must leave a tombstone
//     the runner finishes.
//  V6 a deletion leaves the switcher's active user pointing at the deleted
//     account (S12).
//  V7 a session, key or credential is inserted for a pending account by a
//     writer that checked a moment before (S14).
//  V8 the reviewer account can be deleted (Handler.IsDemoAccount).

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/yusefmosiah/go-choir/internal/purgeguard"
)

func accessCookieOf(cookies []*http.Cookie) *http.Cookie {
	for _, c := range cookies {
		if c.Name == AccessTokenCookieName {
			return c
		}
	}
	return nil
}

func (e *delEnv) sessionBody(t *testing.T, cookies []*http.Cookie) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/auth/session", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	e.h.HandleSession(rec, req)
	var m map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &m)
	return rec.Code, m
}

func TestSessionEndpointRefusesADeletedAccount(t *testing.T) { // V1
	e := newDelEnv(t, "v1")
	cookies := e.cookies(t, AuthMethodPasskey)
	if _, m := e.sessionBody(t, cookies); m["authenticated"] != true {
		t.Fatalf("control: %v", m)
	}
	if _, _, err := e.h.store.RequestAccountDeletion(e.user.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, m := e.sessionBody(t, cookies); m["authenticated"] == true {
		t.Fatalf("/auth/session says a deleted account is signed in: %v", m)
	}
}

// A recovery challenge begun after the mail was proved, finished while the
// deletion is pending, adds nothing.
func TestRecoveryCannotAddAPasskeyToAPendingAccount(t *testing.T) { // V2
	h := appEnv(t)
	p := newSoftPasskey(t)
	created := decodeMap(t, createAccount(t, h, "rec@example.com", p, testOrigin))
	if created["api_key"] == nil {
		t.Fatalf("setup: %v", created)
	}
	user, _ := h.store.GetUserByEmail("rec@example.com")
	// The mail was proved earlier: a recovery challenge exists.
	creds, _ := h.store.GetCredentialsByUserID(user.ID)
	waUser, _ := newWebAuthnUser(user, creds)
	creation, session, err := h.webauthn.BeginRegistration(waUser, webauthn.WithConveyancePreference("none"),
		webauthn.WithAuthenticatorSelection(webauthn.SelectAuthenticator("platform", nil, "required")))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(session)
	saveRecovery := func() {
		if err := h.store.SaveChallengeState(&ChallengeState{ID: session.Challenge, UserID: user.ID, Challenge: session.Challenge, Type: "registration",
			Source: ChallengeSourceRecovery, WebAuthnSessionData: string(raw), CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(SessionChallengeTTL)}); err != nil {
			t.Fatal(err)
		}
	}
	// Control: before the request, a recovery registration adds a passkey.
	saveRecovery()
	attacker := newSoftPasskey(t)
	opts := map[string]any{}
	b, _ := json.Marshal(creation)
	_ = json.Unmarshal(b, &opts)
	rec := postJSON(h.HandleRegisterFinish, attacker.create(t, opts, testOrigin))
	if rec.Code != http.StatusOK {
		t.Fatalf("control: recovery registration = %d %s", rec.Code, rec.Body)
	}
	before, _ := h.store.GetCredentialsByUserID(user.ID)

	// Now the deletion is requested; the challenge begun before is gone, and a
	// recovery challenge written by anything is refused before it writes.
	if _, _, err := h.store.RequestAccountDeletion(user.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	creation2, session2, err := h.webauthn.BeginRegistration(waUser, webauthn.WithConveyancePreference("none"),
		webauthn.WithAuthenticatorSelection(webauthn.SelectAuthenticator("platform", nil, "required")))
	if err != nil {
		t.Fatal(err)
	}
	raw2, _ := json.Marshal(session2)
	if err := h.store.SaveChallengeState(&ChallengeState{ID: session2.Challenge, UserID: user.ID, Challenge: session2.Challenge, Type: "registration",
		Source: ChallengeSourceRecovery, WebAuthnSessionData: string(raw2), CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(SessionChallengeTTL)}); err != nil {
		t.Fatal(err)
	}
	attacker2 := newSoftPasskey(t)
	opts2 := map[string]any{}
	b2, _ := json.Marshal(creation2)
	_ = json.Unmarshal(b2, &opts2)
	rec2 := postJSON(h.HandleRegisterFinish, attacker2.create(t, opts2, testOrigin))
	if rec2.Code == http.StatusOK {
		t.Fatalf("a registration finished for a pending account: %s", rec2.Body)
	}
	after, _ := h.store.GetCredentialsByUserID(user.ID)
	if len(after) != len(before) {
		t.Fatalf("a passkey was written for a pending account: %d -> %d", len(before), len(after))
	}
	// The store itself refuses, whatever writer asks.
	if err := h.store.CreateCredential(&Credential{ID: "late", UserID: user.ID, PublicKey: []byte{1}, AttestationType: "none", Transport: "[]", AAGUID: make([]byte, 16), Flags: "{}", CreatedAt: time.Now().UTC()}); !errors.Is(err, ErrDeletionPending) {
		t.Fatalf("store.CreateCredential for a pending account: %v", err)
	}
}

func TestRecoveryRequestAndVerifyRefuseAPendingAccount(t *testing.T) { // V2
	e := newDelEnv(t, "v2b")
	// A token minted before the request, redeemed after it, makes no challenge.
	token, err := e.h.store.CreateRecoveryToken(context.Background(), e.user.ID, e.user.Email, hashEmail(e.user.Email), hashIP("203.0.113.9"))
	if err != nil {
		t.Fatal(err)
	}
	deleteAccount(t, e)
	rec := e.call(e.h.HandleRecoveryVerify, "/auth/recovery/verify", []byte(`{"token":"`+token+`"}`), nil, "")
	if rec.Code == http.StatusOK {
		t.Fatalf("recovery verify of a pending account answered a registration challenge: %s", rec.Body)
	}
	var n int
	if err := e.h.store.db.QueryRow(`SELECT COUNT(*) FROM challenge_state WHERE user_id = ?`, e.user.ID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("recovery left %d challenges (%v)", n, err)
	}
	// A new request mails nothing (the answer stays the same to a stranger).
	mailer := &fakeRecoveryMailer{}
	e.h.recoveryMailer = mailer
	e.h.recoveryMailDispatch = func(f func()) { f() }
	req := e.call(e.h.HandleRecoveryRequest, "/auth/recovery/request", []byte(`{"email":"`+e.user.Email+`"}`), nil, "")
	if req.Code != http.StatusOK {
		t.Fatalf("request answer must not reveal the state: %d", req.Code)
	}
	if len(mailer.mails()) != 0 {
		t.Fatal("a recovery link was mailed for a pending account")
	}
}

func TestPurgingAccountIsToldPurgingAndOfferedNothing(t *testing.T) { // V3
	e := newDelEnv(t, "v3")
	deleteAccount(t, e)
	if _, err := e.h.store.db.Exec(`UPDATE account_deletions SET requested_at = ?, purge_after = ?`, time.Now().UTC().Add(-100*time.Hour), time.Now().UTC().Add(-28*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := e.h.store.BeginPurge(e.user.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	rec := e.passkeySignIn(t)
	var m map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &m)
	if rec.Code != http.StatusForbidden || m["error"] != "account_deletion_purging" || m["cancel_token"] != nil {
		t.Fatalf("a purging account's sign-in: %d %s", rec.Code, rec.Body)
	}
}

// Round 3, B1: the state is the boundary, not the clock. A record past its
// purge time that the runner has not begun (a preflight that cannot pass, a
// busy runner) removed nothing, so a passkey sign-in is offered the cancel and
// the cancel works; only a record the runner moved to purging is closed.
func TestAPendingRecordPastItsPurgeTimeIsStillOfferedTheCancel(t *testing.T) { // V3, V4, round 3 B1
	e := newDelEnv(t, "v3b")
	deleteAccount(t, e)
	if _, err := e.h.store.db.Exec(`UPDATE account_deletions SET requested_at = ?, purge_after = ?`, time.Now().UTC().Add(-73*time.Hour), time.Now().UTC().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	rec := e.passkeySignIn(t)
	var offer deletionOffer
	_ = json.Unmarshal(rec.Body.Bytes(), &offer)
	if rec.Code != http.StatusForbidden || offer.Error != "account_deletion_pending" || offer.CancelToken == "" {
		t.Fatalf("a due but unbegun record's sign-in must offer the cancel: %d %s", rec.Code, rec.Body)
	}
	body, _ := json.Marshal(cancelRequest{CancelToken: offer.CancelToken})
	if c := e.call(e.h.HandleDeleteCancel, "/auth/account/delete/cancel", body, nil, ""); c.Code != http.StatusOK {
		t.Fatalf("the cancel of a due but unbegun record: %d %s", c.Code, c.Body)
	}
}

func TestCancelFollowsTheStateNotTheClock(t *testing.T) { // V4, S7, round 3 B1
	e := newDelEnv(t, "v4")
	deleteAccount(t, e)
	// After purge_after and before the runner begins: still offered and cancellable.
	late := time.Now().UTC().Add(purgeguard.GracePeriod + time.Minute)
	tok, err := e.h.store.OfferDeletionCancel(e.user.ID, late)
	if err != nil {
		t.Fatalf("no offer for a pending record past purge_after: %v", err)
	}
	if _, err := e.h.store.PeekCancelOffer(tok, late); err != nil {
		t.Fatalf("peek refused a pending record past purge_after: %v", err)
	}
	// The runner begins: from now on nothing can be offered, peeked or cancelled.
	if _, err := e.h.store.BeginPurge(e.user.ID, late); err != nil {
		t.Fatal(err)
	}
	if _, err := e.h.store.PeekCancelOffer(tok, late); err == nil {
		t.Fatal("peek accepted an offer for a purging record")
	}
	if _, err := e.h.store.CancelAccountDeletion(tok, late); err == nil {
		t.Fatal("cancel accepted an offer for a purging record")
	}
	if _, err := e.h.store.OfferDeletionCancel(e.user.ID, late); !errors.Is(err, ErrDeletionNotCancellable) {
		t.Fatalf("an offer for a purging record: %v", err)
	}
	if d, _ := e.h.store.AccountDeletionFor(e.user.ID); d == nil || d.State != DeletionPurging {
		t.Fatal("the refused cancel changed the record")
	}
}

func TestCancelWithAnUnknownAppChangesNothing(t *testing.T) { // V4, S8
	e := newDelEnv(t, "v4b")
	deleteAccount(t, e)
	var offer deletionOffer
	_ = json.Unmarshal(e.passkeySignIn(t).Body.Bytes(), &offer)
	body, _ := json.Marshal(cancelRequest{CancelToken: offer.CancelToken, App: "evil.app"})
	if rec := e.call(e.h.HandleDeleteCancel, "/auth/account/delete/cancel", body, nil, ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown app: %d", rec.Code)
	}
	if d, _ := e.h.store.AccountDeletionFor(e.user.ID); d == nil || d.State != DeletionPending {
		t.Fatal("a refused request cancelled the deletion")
	}
	good, _ := json.Marshal(cancelRequest{CancelToken: offer.CancelToken})
	if rec := e.call(e.h.HandleDeleteCancel, "/auth/account/delete/cancel", good, nil, ""); rec.Code != http.StatusOK {
		t.Fatalf("the token must still be good after a refused request: %d %s", rec.Code, rec.Body)
	}
}

func TestDoubleCancelIsAnInvalidOfferNotAFailure(t *testing.T) { // V4, S2
	e := newDelEnv(t, "v4c")
	deleteAccount(t, e)
	var offer deletionOffer
	_ = json.Unmarshal(e.passkeySignIn(t).Body.Bytes(), &offer)
	body, _ := json.Marshal(cancelRequest{CancelToken: offer.CancelToken})
	first := e.call(e.h.HandleDeleteCancel, "/auth/account/delete/cancel", body, nil, "")
	second := e.call(e.h.HandleDeleteCancel, "/auth/account/delete/cancel", body, nil, "")
	if first.Code != http.StatusOK || second.Code != http.StatusBadRequest {
		t.Fatalf("first %d second %d (%s)", first.Code, second.Code, second.Body)
	}
}

// B2: the cancel is one durable act, after which the takes are the runner's to
// serve again; a pass that was already working cannot undo it.
func TestCancelLeavesATombstoneTheRunnerFinishes(t *testing.T) { // V5
	e := newDelEnv(t, "v5")
	// Requested long ago so that "eligible by age" would be true if a
	// cancelled record counted.
	if _, _, err := e.h.store.RequestAccountDeletion(e.user.ID, time.Now().UTC().Add(-100*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := e.h.store.db.Exec(`UPDATE account_deletions SET purge_after = ?`, time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	tok, err := e.h.store.OfferDeletionCancel(e.user.ID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.h.store.CancelAccountDeletion(tok, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if d, _ := e.h.store.AccountDeletionFor(e.user.ID); d != nil {
		t.Fatalf("a cancelled record still answers as live: %+v", d)
	}
	if e.h.store.AccountAccessRevoked(e.user.ID) {
		t.Fatal("a cancelled account is still revoked")
	}
	if err := e.h.store.PurgeEligible(e.user.ID, time.Now().UTC()); !errors.Is(err, purgeguard.ErrNoRecord) {
		t.Fatalf("a cancelled record made a purge eligible: %v", err)
	}
	open, _ := e.h.store.OpenAccountDeletions(0)
	if len(open) != 1 || open[0].State != DeletionCancelled {
		t.Fatalf("the runner cannot see the tombstone: %+v", open)
	}
	if err := e.h.store.FinishCancelledDeletion(e.user.ID); err != nil {
		t.Fatal(err)
	}
	if open, _ := e.h.store.OpenAccountDeletions(0); len(open) != 0 {
		t.Fatalf("the tombstone survives its finish: %+v", open)
	}
	// A second deletion request after a cancel starts clean.
	if _, _, err := e.h.store.RequestAccountDeletion(e.user.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	tok, _ = e.h.store.OfferDeletionCancel(e.user.ID, time.Now().UTC())
	_, _ = e.h.store.CancelAccountDeletion(tok, time.Now().UTC())
	if d, _, err := e.h.store.RequestAccountDeletion(e.user.ID, time.Now().UTC()); err != nil || d.State != DeletionPending {
		t.Fatalf("a request over an unfinished tombstone: %+v %v", d, err)
	}
}

func TestDeletionRepairsTheAccountSwitcher(t *testing.T) { // V6
	e := newDelEnv(t, "v6a")
	other := addDelUser(t, e.h, e.host, "v6b")
	// One browser with both accounts, the first one active.
	_, rs1, err := e.h.createRefreshSessionRow(e.user, "ua", AuthMethodPasskey, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, rs2, err := other.h.createRefreshSessionRow(other.user, "ua", AuthMethodPasskey, nil)
	if err != nil {
		t.Fatal(err)
	}
	set, _, err := e.h.store.AddAccountToSet("", "hash-v6", other.user.ID, rs2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := e.h.store.AddAccountToSet("hash-v6", "hash-v6b", e.user.ID, rs1.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.h.store.SetActiveAccount(set, e.user.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := e.h.store.RequestAccountDeletion(e.user.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	var active string
	if err := e.h.store.db.QueryRow(`SELECT active_user_id FROM account_set WHERE id = ?`, set).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active == e.user.ID {
		t.Fatal("the switcher still names the deleted account as active")
	}
}

func TestNoSessionKeyOrCredentialIsWrittenForAPendingAccount(t *testing.T) { // V7
	e := newDelEnv(t, "v7")
	if _, _, err := e.h.store.RequestAccountDeletion(e.user.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := e.h.store.CreateRefreshSession(&RefreshSession{ID: "rs-late", UserID: e.user.ID, TokenHash: "h", CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(time.Hour), AuthMethod: AuthMethodPasskey}); !errors.Is(err, ErrDeletionPending) {
		t.Fatalf("session: %v", err)
	}
	if _, _, err := e.h.store.CreateAPIKey(context.Background(), e.user.ID, "late", []string{"read:runtime"}, nil); !errors.Is(err, ErrDeletionPending) {
		t.Fatalf("key: %v", err)
	}
	var n int
	_ = e.h.store.db.QueryRow(`SELECT (SELECT COUNT(*) FROM refresh_sessions WHERE user_id = ?) + (SELECT COUNT(*) FROM api_keys WHERE user_id = ? AND revoked_at IS NULL)`, e.user.ID, e.user.ID).Scan(&n)
	if n != 0 {
		t.Fatalf("%d live rows for a pending account", n)
	}
	// An account with no record still gets both.
	free := addDelUser(t, e.h, e.host, "v7free")
	if _, _, err := e.h.store.CreateAPIKey(context.Background(), free.user.ID, "ok", []string{"read:runtime"}, nil); err != nil {
		t.Fatalf("control: %v", err)
	}
}

func TestTheReviewerAccountCannotBeDeleted(t *testing.T) { // V8
	e := newDelEnv(t, "v8")
	cookies := e.cookies(t, AuthMethodPasskey)
	// An assertion begun before the account was named the reviewer account.
	opts := e.begin(t, cookies, "")
	// The reviewer account with its sign-in on (a valid credential file).
	file := filepath.Join(t.TempDir(), "demo-credential")
	line, err := FormatDemoCredential(demoTestUser, demoTestPass)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := *e.h.config
	cfg.DemoAccountID, cfg.DemoCredentialFile = e.user.ID, file
	e.h.config = &cfg
	if rec := e.call(e.h.HandleDeleteBegin, "/auth/account/delete/begin", nil, cookies, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("begin for the reviewer account: %d", rec.Code)
	}
	if rec := e.call(e.h.HandleDeleteFinish, "/auth/account/delete/finish", e.assertion(t, opts), cookies, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("finish for the reviewer account: %d %s", rec.Code, rec.Body)
	}
	e.noRecord(t)
	// Any other account is unaffected.
	other := addDelUser(t, e.h, e.host, "v8b")
	deleteAccount(t, other)
}

var _ = strings.Contains
var _ = protocol.VerificationRequired

// Round 2 (seat 2 F4, seat 1 #6 and #10): the routes a copied access cookie
// could still reach inside its five minutes, the email sign-in that told a
// pending account's state, and the two inserts that did not check the record.
func TestAStaleAccessCookieOfADeletedAccountReachesNoPasskeySessionRoute(t *testing.T) { // F4
	e := newDelEnv(t, "r2a")
	cookies := e.cookies(t, AuthMethodPasskey)
	req := httptest.NewRequest(http.MethodDelete, "/auth/credentials/x", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	if u := e.h.requirePasskeySession(httptest.NewRecorder(), req); u == nil {
		t.Fatal("control: a live passkey session is refused")
	}
	if _, _, err := e.h.store.RequestAccountDeletion(e.user.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	if u := e.h.requirePasskeySession(rec, req); u != nil || rec.Code != http.StatusUnauthorized {
		t.Fatalf("a cookie copied before the request still passes the passkey-session gate: %d", rec.Code)
	}
}

func TestEmailSignInIsNotOfferedToAPendingAccount(t *testing.T) { // seat 1 #6
	e := newDelEnv(t, "r2b")
	if err := e.h.store.SetEmailVerified(e.user.ID); err != nil {
		t.Fatal(err)
	}
	if e.h.lookupEligibleSignInUser(e.user.Email) == nil {
		t.Fatal("control: an eligible account is not eligible")
	}
	if _, _, err := e.h.store.RequestAccountDeletion(e.user.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if e.h.lookupEligibleSignInUser(e.user.Email) != nil {
		t.Fatal("a pending account is sent an email sign-in code, and the verify answer would tell its state")
	}
}

func TestNoExchangeCodeOrDeviceKeyIsWrittenForAPendingAccount(t *testing.T) { // seat 1 #10
	e := newDelEnv(t, "r2c")
	if _, _, err := e.h.store.RequestAccountDeletion(e.user.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := e.h.store.CreateDesktopExchangeCode(&DesktopExchangeCode{Code: "code-late", UserID: e.user.ID, CreatedAt: now, ExpiresAt: now.Add(time.Minute), AuthMethod: AuthMethodPasskey}); !errors.Is(err, ErrDeletionPending) {
		t.Fatalf("exchange code: %v", err)
	}
	// A device authorization approved for the account after the request (a
	// late writer): the poll collects no key.
	if _, err := e.h.store.db.Exec(`INSERT INTO device_authorizations (id, device_code_hash, user_code_hash, label, user_agent_summary, place_summary, ip_hash, created_at, expires_at, status, user_id, decided_at) VALUES ('dev1','dch','uch','late','','','iph',?,?,?,?,?)`,
		now, now.Add(time.Hour), DeviceStatusApproved, e.user.ID, now); err != nil {
		t.Fatal(err)
	}
	res, err := e.h.store.PollDeviceAuthorization(context.Background(), "dch", []string{"read:runtime"}, 0, now)
	if err != nil {
		t.Fatal(err)
	}
	if res.Secret != "" || res.KeyID != "" {
		t.Fatalf("a key was handed to a pending account: %+v", res)
	}
	var n int
	_ = e.h.store.db.QueryRow(`SELECT COUNT(*) FROM api_keys WHERE user_id = ? AND revoked_at IS NULL`, e.user.ID).Scan(&n)
	if n != 0 {
		t.Fatalf("%d live keys for a pending account", n)
	}
}
