package auth

import (
	"bytes"
	"context"
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
		"no account id": func(e *demoEnvT) { e.h.config.DemoAccountID = "" },
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
