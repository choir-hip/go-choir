package auth

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The reviewer sign-in: a user name and a password for one configured account.
// Every test uses a temporary store and a temporary credential file. The
// password here is a test value; it is not a credential for anything.

const (
	demoTestUser = "reviewer-test"
	demoTestPass = "correct horse battery staple 0123456789 test-only"
)

type demoEnvT struct {
	h    *Handler
	uid  string
	file string
	now  *time.Time
}

func demoEnv(t *testing.T) *demoEnvT {
	t.Helper()
	h := appEnv(t)
	u, err := h.store.CreateUser("demo-user-id", "throwaway@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "demo-credential")
	line, err := FormatDemoCredential(demoTestUser, demoTestPass)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := *h.config
	cfg.DemoAccountID = u.ID
	cfg.DemoCredentialFile = file
	h.config = &cfg
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	e := &demoEnvT{h: h, uid: u.ID, file: file, now: &now}
	h.demoGate = newDemoGate(func() time.Time { return *e.now })
	return e
}

func (e *demoEnvT) post(t *testing.T, ip string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	r := httptest.NewRequest(http.MethodPost, "/auth/app/demo/login", bytes.NewReader(raw))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Forwarded-For", ip)
	w := httptest.NewRecorder()
	e.h.HandleAppDemoLogin(w, r)
	return w
}

func good() map[string]any {
	return map[string]any{"app": testApp, "username": demoTestUser, "password": demoTestPass}
}

func TestDemoNotAvailableWithoutConfiguration(t *testing.T) {
	cases := map[string]func(e *demoEnvT){
		"no account id": func(e *demoEnvT) { e.h.config.DemoAccountID = ""; e.h.config.DemoAccountIDFile = "" },
		"no file path":  func(e *demoEnvT) { e.h.config.DemoCredentialFile = "" },
		"file removed":  func(e *demoEnvT) { _ = os.Remove(e.file) },
		"file garbage":  func(e *demoEnvT) { _ = os.WriteFile(e.file, []byte("not a credential\n"), 0o600) },
		"no such user":  func(e *demoEnvT) { e.h.config.DemoAccountID = "someone-else-entirely" },
		"hash too heavy": func(e *demoEnvT) {
			_ = os.WriteFile(e.file, []byte(demoTestUser+"\n$argon2id$v=19$m=4194304,t=3,p=2$c2FsdHNhbHRzYWx0c2FsdA$aGFzaGhhc2hoYXNoaGFzaGhhc2hoYXNoaGFzaGhhc2g\n"), 0o600)
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			e := demoEnv(t)
			mutate(e)
			w := e.post(t, "198.51.100.1", good())
			if w.Code != http.StatusNotFound {
				t.Fatalf("want 404 not available, got %d %s", w.Code, w.Body)
			}
			keys, _ := e.h.store.ListAPIKeys(context.Background(), e.uid)
			if len(keys) != 0 {
				t.Fatalf("a key was made while not available")
			}
		})
	}
}

func TestDemoRightPasswordGivesTheNarrowKeyForTheConfiguredAccountOnly(t *testing.T) {
	e := demoEnv(t)
	other, _ := e.h.store.CreateUser("other-user-id", "other@example.invalid")
	body := good()
	// A request cannot name an account: these fields are refused, not obeyed.
	body["user_id"] = other.ID
	if w := e.post(t, "198.51.100.1", body); w.Code != http.StatusBadRequest {
		t.Fatalf("a body naming an account must be refused, got %d", w.Code)
	}
	w := e.post(t, "198.51.100.1", good())
	if w.Code != http.StatusOK {
		t.Fatalf("got %d %s", w.Code, w.Body)
	}
	var k appKeyResponse
	if err := json.Unmarshal(w.Body.Bytes(), &k); err != nil || !strings.HasPrefix(k.APIKey, APIKeyPrefix) {
		t.Fatalf("no key in %s", w.Body)
	}
	keys, _ := e.h.store.ListAPIKeys(context.Background(), e.uid)
	if len(keys) != 1 || keys[0].ID != k.KeyID {
		t.Fatalf("the key must belong to the configured account, got %v", keys)
	}
	if k2, _ := e.h.store.ListAPIKeys(context.Background(), other.ID); len(k2) != 0 {
		t.Fatal("another account got a key")
	}
	if keys[0].ExpiresAt == nil || keys[0].ExpiresAt.After(e.now.Add(DemoKeyTTL+time.Hour)) {
		t.Fatalf("the key must expire within %s, got %v", DemoKeyTTL, keys[0].ExpiresAt)
	}
	for _, withheld := range []string{"owner:acts", "admin", "manage:keys", "computer:lifecycle",
		"computer:self_development:read", "computer:self_development:genesis", "computer:self_development:propose",
		"computer:self_development:approve", "computer:self_development:rollback", "computer:self_development:mode",
		"precommit:read", "precommit:write", "acceptance:read"} {
		if apiKeyHasScope(k.Scopes, withheld) || apiKeyHasScope(keys[0].Scopes, withheld) {
			t.Errorf("the reviewer key must not carry %s", withheld)
		}
	}
	for _, need := range []string{"read:runtime", "write:runtime", "read:texture", "write:texture", "read:base", "write:base"} {
		if !apiKeyHasScope(keys[0].Scopes, need) {
			t.Errorf("the app needs %s", need)
		}
	}
	if !e.h.IsDemoAccount(e.uid) || e.h.IsDemoAccount(other.ID) || e.h.IsDemoAccount("") {
		t.Fatal("IsDemoAccount must be true for the configured account only")
	}
}

func TestDemoWrongCredentialsAreAlikeAndLogNoSecret(t *testing.T) {
	e := demoEnv(t)
	var logs bytes.Buffer
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	badPass := good()
	badPass["password"] = "wrong-password-value-xyz"
	badUser := good()
	badUser["username"] = "not-the-reviewer-abc"
	a := e.post(t, "198.51.100.1", badPass)
	b := e.post(t, "198.51.100.2", badUser)
	if a.Code != http.StatusUnauthorized || b.Code != http.StatusUnauthorized || a.Body.String() != b.Body.String() {
		t.Fatalf("wrong user and wrong password must answer alike: %d %q / %d %q", a.Code, a.Body, b.Code, b.Body)
	}
	if keys, _ := e.h.store.ListAPIKeys(context.Background(), e.uid); len(keys) != 0 {
		t.Fatal("a key was made for a wrong credential")
	}
	e.post(t, "198.51.100.3", good())
	out := logs.String()
	for _, secret := range []string{"wrong-password-value-xyz", "not-the-reviewer-abc", demoTestPass, demoTestUser} {
		if strings.Contains(out, secret) {
			t.Fatalf("the log carries %q", secret)
		}
	}
	if !strings.Contains(out, "operation=app_demo_login") || !strings.Contains(out, "result=denied") || !strings.Contains(out, "result=success") {
		t.Fatalf("every attempt must be logged: %s", out)
	}
}

func TestDemoUnknownAppRefused(t *testing.T) {
	e := demoEnv(t)
	b := good()
	b["app"] = "evil.example"
	if w := e.post(t, "198.51.100.1", b); w.Code != http.StatusBadRequest {
		t.Fatalf("got %d", w.Code)
	}
}

func TestDemoPerAddressBoundExpiresAndNeverLocksTheAccount(t *testing.T) {
	e := demoEnv(t)
	bad := good()
	bad["password"] = "nope-nope-nope-nope"
	for i := 0; i < DemoAttemptsPerAddress; i++ {
		if w := e.post(t, "198.51.100.9", bad); w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: %d", i, w.Code)
		}
	}
	// This address is now out, even with the right password: the bound counts attempts, not failures.
	w := e.post(t, "198.51.100.9", good())
	if w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") == "" {
		t.Fatalf("want 429 with Retry-After, got %d", w.Code)
	}
	// The account is not locked: another address, right password, signs in.
	if w := e.post(t, "198.51.100.10", good()); w.Code != http.StatusOK {
		t.Fatalf("another address must still sign in, got %d", w.Code)
	}
	// And the bound expires on its own.
	*e.now = e.now.Add(DemoAddressWindow + time.Second)
	if w := e.post(t, "198.51.100.9", good()); w.Code != http.StatusOK {
		t.Fatalf("after the window the address must sign in, got %d", w.Code)
	}
}

func TestDemoGlobalBoundRecoversAndIsNotAccountLockout(t *testing.T) {
	e := demoEnv(t)
	bad := good()
	bad["password"] = "nope-nope-nope-nope"
	for i := 0; i < DemoAttemptsGlobal; i++ {
		ip := "203.0.113." + itoa(i%250+1)
		if i >= 250 {
			ip = "192.0.2." + itoa(i-249)
		}
		if w := e.post(t, ip, bad); w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: %d", i, w.Code)
		}
	}
	w := e.post(t, "198.18.0.1", good())
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("the global bound must refuse, got %d", w.Code)
	}
	if secs := w.Header().Get("Retry-After"); secs == "" {
		t.Fatal("Retry-After missing")
	}
	*e.now = e.now.Add(DemoGlobalWindow + time.Second)
	if w := e.post(t, "198.18.0.1", good()); w.Code != http.StatusOK {
		t.Fatalf("the global bound must lift on its own, got %d", w.Code)
	}
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

func TestDemoReadsTheFileEachTimeSoRemovingItTurnsTheRouteOff(t *testing.T) {
	e := demoEnv(t)
	if w := e.post(t, "198.51.100.1", good()); w.Code != http.StatusOK {
		t.Fatalf("got %d", w.Code)
	}
	_ = os.Remove(e.file)
	if w := e.post(t, "198.51.100.2", good()); w.Code != http.StatusNotFound {
		t.Fatalf("got %d", w.Code)
	}
	if e.h.IsDemoAccount(e.uid) != true {
		t.Fatal("the account stays protected from deletion even when sign-in is off")
	}
}

func TestDemoAccountIDComesFromAHostFile(t *testing.T) {
	e := demoEnv(t)
	idFile := filepath.Join(t.TempDir(), "demo-id")
	_ = os.WriteFile(idFile, []byte(e.uid+"\n"), 0o600)
	e.h.config.DemoAccountID = ""
	e.h.config.DemoAccountIDFile = idFile
	if w := e.post(t, "198.51.100.1", good()); w.Code != http.StatusOK || !e.h.IsDemoAccount(e.uid) {
		t.Fatalf("got %d", w.Code)
	}
	_ = os.Remove(idFile)
	if w := e.post(t, "198.51.100.2", good()); w.Code != http.StatusNotFound {
		t.Fatalf("got %d", w.Code)
	}
}

func TestDemoPasswordFormatRoundTripsAndRefusesHeavyParameters(t *testing.T) {
	line, err := FormatDemoCredential("u", "p-long-enough-0123456789-abcdef")
	if err != nil {
		t.Fatal(err)
	}
	user, hash, err := parseDemoCredential([]byte(line))
	if err != nil || user != "u" || !verifyDemoPassword(hash, "p-long-enough-0123456789-abcdef") || verifyDemoPassword(hash, "other") {
		t.Fatalf("round trip failed: %v", err)
	}
	if _, err := FormatDemoCredential("u", "short"); err == nil {
		t.Fatal("a short password must be refused at generation")
	}
	if _, _, err := parseDemoCredential([]byte("u\n$argon2id$v=19$m=1,t=1,p=1$YQ$YQ\n")); err == nil {
		t.Fatal("weak parameters must be refused")
	}
}

func TestWhoamiSaysReviewerOnlyForTheReviewerAccount(t *testing.T) {
	e := demoEnv(t)
	w := e.post(t, "198.51.100.1", good())
	var k appKeyResponse
	_ = json.Unmarshal(w.Body.Bytes(), &k)
	other, _ := e.h.store.CreateUser("other-user-id", "other@example.invalid")
	_, otherSecret, _ := e.h.store.CreateAPIKey(context.Background(), other.ID, "x", []string{"read:runtime"}, nil)
	for secret, want := range map[string]bool{k.APIKey: true, otherSecret: false} {
		r := httptest.NewRequest(http.MethodGet, "/auth/whoami", nil)
		r.Header.Set("Authorization", "Bearer "+secret)
		rec := httptest.NewRecorder()
		e.h.HandleWhoami(rec, r)
		var got struct{ Reviewer bool }
		_ = json.Unmarshal(rec.Body.Bytes(), &got)
		if rec.Code != 200 || got.Reviewer != want {
			t.Fatalf("want reviewer=%v got %d %s", want, rec.Code, rec.Body)
		}
	}
}

// Main's proof rule: an account whose address was never proved mints nothing and signs in to
// nothing. The reviewer route is no way round it: the account must be one whose address is proved.
func TestDemoAccountWithAnUnprovedAddressGetsNoKey(t *testing.T) {
	e := demoEnv(t)
	if _, err := e.h.store.db.Exec(`UPDATE users SET email_verified_at = NULL WHERE id = ?`, e.uid); err != nil {
		t.Fatal(err)
	}
	if w := e.post(t, "198.51.100.7", good()); w.Code != http.StatusNotFound {
		t.Fatalf("an unproved reviewer account must be 'not available', got %d %s", w.Code, w.Body)
	}
	if keys, _ := e.h.store.ListAPIKeys(context.Background(), e.uid); len(keys) != 0 {
		t.Fatalf("no key may be minted for an unproved account, got %d", len(keys))
	}
}

// off must leave nothing usable for the account (review, 2026-11-24): the pending
// grants go with the keys and sessions.
func TestResetDemoAccessDeletesEveryPendingGrantOfTheAccountOnly(t *testing.T) {
	e := demoEnv(t)
	other, _ := e.h.store.CreateUser("other-user-id", "other@example.invalid")
	db := e.h.store.db
	now := time.Now().UTC()
	exp := now.Add(10 * time.Minute)
	for _, uid := range []string{e.uid, other.ID} {
		mustExec(t, db, `INSERT INTO device_authorizations (id, device_code_hash, user_code_hash, ip_hash, created_at, expires_at, status, user_id) VALUES (?, ?, ?, 'ip', ?, ?, 'approved', ?)`, "da-"+uid, "dc-"+uid, "uc-"+uid, now, exp, uid)
		mustExec(t, db, `INSERT INTO desktop_exchange_codes (code, user_id, access_token, refresh_token, created_at, expires_at) VALUES (?, ?, 'a', 'r', ?, ?)`, "code-"+uid, uid, now, exp)
		mustExec(t, db, `INSERT INTO recovery_tokens (id, user_id, email, email_hash, ip_hash, token_hash, created_at, expires_at) VALUES (?, ?, 'x@example.invalid', 'eh', 'ih', 'th', ?, ?)`, "rt-"+uid, uid, now, exp)
		mustExec(t, db, `INSERT INTO email_signin_attempts (id, user_id, email_hash, code_hash, link_token_hash, cookie_hash, ip_hash, created_at, expires_at) VALUES (?, ?, 'eh', 'ch', 'lh', 'kh', 'ih', ?, ?)`, "es-"+uid, uid, now, exp)
		mustExec(t, db, `INSERT INTO challenge_state (id, user_id, challenge, type, created_at, expires_at) VALUES (?, ?, 'c', 'login', ?, ?)`, "cs-"+uid, uid, now, exp)
	}
	if _, _, err := ResetDemoAccess(context.Background(), e.h.store, e.uid); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"device_authorizations", "desktop_exchange_codes", "recovery_tokens", "email_signin_attempts", "challenge_state"} {
		var mine, theirs int
		_ = db.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE user_id = ?`, e.uid).Scan(&mine)
		_ = db.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE user_id = ?`, other.ID).Scan(&theirs)
		if mine != 0 || theirs != 1 {
			t.Fatalf("%s: the account kept %d rows (want 0), another account has %d (want 1)", table, mine, theirs)
		}
	}
}

func mustExec(t *testing.T, db interface {
	Exec(string, ...any) (sql.Result, error)
}, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

// While the account is off (the credential file is gone) no sign-in of any other kind
// reaches it either: no session token, no app key, and a token made before is not read.
func TestAnOffDemoAccountMintsAndAuthenticatesNothing(t *testing.T) {
	e := demoEnv(t)
	user, _ := e.h.store.GetUserByID(e.uid)
	tok, err := e.h.issueAccessJWT(user, "passkey")
	if err != nil {
		t.Fatalf("the account is on: %v", err)
	}
	if _, err := e.h.mintAppKey(context.Background(), user, testApp); err != nil {
		t.Fatalf("the account is on: %v", err)
	}
	if err := os.Remove(e.file); err != nil {
		t.Fatal(err)
	}
	if _, err := e.h.issueAccessJWT(user, "passkey"); err == nil {
		t.Error("a session token was made for an off account")
	}
	if _, err := e.h.mintAppKey(context.Background(), user, testApp); err == nil {
		t.Error("an app key was made for an off account")
	}
	r := httptest.NewRequest(http.MethodGet, "/auth/api-keys", nil)
	r.AddCookie(&http.Cookie{Name: AccessTokenCookieName, Value: tok})
	w := httptest.NewRecorder()
	if u, _ := e.h.requireAuthUserAny(w, r); u != nil {
		t.Error("a session token made before was accepted for an off account")
	}
	// Another account is untouched.
	other, _ := e.h.store.CreateUser("other-user-id", "other@example.invalid")
	if _, err := e.h.issueAccessJWT(other, "passkey"); err != nil {
		t.Errorf("another account: %v", err)
	}
}

func TestDemoLoginNeverServesTheBootstrapAccountAndNeverCaches(t *testing.T) {
	e := demoEnv(t)
	if w := e.post(t, "198.51.100.8", good()); w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("a key reply must be no-store: %d %q", w.Code, w.Header().Get("Cache-Control"))
	}
	e.h.config.DemoAccountID = BootstrapAdminAPIKeyUserID
	if _, err := e.h.store.db.Exec(`INSERT OR IGNORE INTO users (id, email, created_at) VALUES (?, 'bootstrap@example.invalid', ?)`, BootstrapAdminAPIKeyUserID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if w := e.post(t, "198.51.100.9", good()); w.Code != http.StatusNotFound {
		t.Fatalf("the bootstrap account must never be the reviewer's: %d %s", w.Code, w.Body)
	}
}

// A session token made before `off` is read by no route of this service while the account is off.
func TestAnOffDemoAccountIsReadByNoCookieRoute(t *testing.T) {
	e := demoEnv(t)
	user, _ := e.h.store.GetUserByID(e.uid)
	tok, err := e.h.issueAccessJWT(user, "passkey")
	if err != nil {
		t.Fatal(err)
	}
	cookieReq := func() *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/auth/device/approve", nil)
		r.AddCookie(&http.Cookie{Name: AccessTokenCookieName, Value: tok})
		return r
	}
	if u := e.h.requirePasskeySession(httptest.NewRecorder(), cookieReq()); u == nil {
		t.Fatal("the account is on: the token must work")
	}
	if err := os.Remove(e.file); err != nil {
		t.Fatal(err)
	}
	if u := e.h.requirePasskeySession(httptest.NewRecorder(), cookieReq()); u != nil {
		t.Error("requirePasskeySession accepted a token made before off")
	}
	if u := e.h.requireAuthUser(httptest.NewRecorder(), cookieReq()); u != nil {
		t.Error("requireAuthUser accepted a token made before off")
	}
	if _, err := e.h.ValidateAccessToken(tok); err == nil {
		t.Error("ValidateAccessToken accepted a token made before off")
	}
}

// `off` removes the credential and then proves the keys revoked. A login that passed the
// credential check just before must not leave a live key after: the mint is checked again.
func TestADemoLoginThatRacesOffLeavesNoLiveKey(t *testing.T) {
	e := demoEnv(t)
	e.h.demoMintHook = func() { _ = os.Remove(e.file) } // off lands between the check and the insert
	w := e.post(t, "198.51.100.20", good())
	if w.Code != http.StatusNotFound {
		t.Fatalf("a login that raced off must answer not available, got %d %s", w.Code, w.Body)
	}
	keys, _ := e.h.store.ListAPIKeys(context.Background(), e.uid)
	for _, k := range keys {
		if k.RevokedAt == nil {
			t.Fatalf("a key minted across off is live: %s", k.ID)
		}
	}
	if strings.Contains(w.Body.String(), APIKeyPrefix) {
		t.Fatal("the key was sent")
	}
}

func TestAnAppKeyMintThatRacesOffLeavesNoLiveKey(t *testing.T) {
	e := demoEnv(t)
	user, _ := e.h.store.GetUserByID(e.uid)
	e.h.demoMintHook = func() { _ = os.Remove(e.file) }
	if _, err := e.h.mintAppKey(context.Background(), user, testApp); err == nil {
		t.Fatal("a key was handed out across off")
	}
	keys, _ := e.h.store.ListAPIKeys(context.Background(), e.uid)
	for _, k := range keys {
		if k.RevokedAt == nil {
			t.Fatalf("a key minted across off is live: %s", k.ID)
		}
	}
}

func demoDeviceEnv(t *testing.T) (*Handler, ed25519.PrivateKey, *User, string) {
	t.Helper()
	h, priv := testHandlerEnv(t)
	h.config.RPOrigins = []string{"https://choir.news"}
	u, _ := h.store.CreateUser("device-demo-user", "device-demo@example.invalid")
	file := filepath.Join(t.TempDir(), "demo-credential")
	line, _ := FormatDemoCredential(demoTestUser, demoTestPass)
	if err := os.WriteFile(file, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := *h.config
	cfg.DemoAccountID = u.ID
	cfg.DemoCredentialFile = file
	h.config = &cfg
	return h, priv, u, file
}

func liveKeys(h *Handler, uid string) (n int) {
	keys, _ := h.store.ListAPIKeys(context.Background(), uid)
	for _, k := range keys {
		if k.RevokedAt == nil {
			n++
		}
	}
	return
}

// An approval that is in flight when `off` runs (its row had no user yet, so off's delete
// missed it) must not end in a key.
func TestADeviceApprovalInFlightDuringOffMintsNoKey(t *testing.T) {
	h, priv, u, file := demoDeviceEnv(t)
	b := deviceBegin(t, h, "laptop", "203.0.113.7")
	h.demoMintHook = func() { _ = os.Remove(file) } // off lands after the session was accepted
	rec := deviceDecide(h, passkeyCookieReq(http.MethodPost, "/auth/device/approve", map[string]string{"user_code": b.UserCode, "decision": "approve"}, priv, u.ID))
	if rec.Code == http.StatusOK {
		t.Fatalf("an approval that raced off was accepted: %s", rec.Body)
	}
	rewindPoll(t, h)
	if _, m := devicePoll(h, b.DeviceCode); m["api_key"] != nil {
		t.Fatalf("a key came out of an approval that raced off: %v", m)
	}
	if liveKeys(h, u.ID) != 0 {
		t.Fatal("a live key exists for the off account")
	}
}

// An approved row that is collected after `off` yields no key either.
func TestACollectionAfterOffMintsNoKey(t *testing.T) {
	h, priv, u, file := demoDeviceEnv(t)
	b := deviceBegin(t, h, "laptop", "203.0.113.8")
	if rec := deviceDecide(h, passkeyCookieReq(http.MethodPost, "/auth/device/approve", map[string]string{"user_code": b.UserCode, "decision": "approve"}, priv, u.ID)); rec.Code != http.StatusOK {
		t.Fatalf("approve while on: %d %s", rec.Code, rec.Body)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	rewindPoll(t, h)
	if _, m := devicePoll(h, b.DeviceCode); m["api_key"] != nil {
		t.Fatalf("a key was collected after off: %v", m)
	}
	if liveKeys(h, u.ID) != 0 {
		t.Fatal("a live key exists for the off account")
	}
}
