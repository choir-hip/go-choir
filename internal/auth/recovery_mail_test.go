package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/maild"
	"github.com/yusefmosiah/go-choir/internal/server"
)

type sentRecoveryMail struct {
	to        string
	link      string
	expiresIn time.Duration
}

type fakeRecoveryMailer struct {
	mu   sync.Mutex
	sent []sentRecoveryMail
	err  error
}

func (f *fakeRecoveryMailer) SendRecoveryLink(_ context.Context, to, link string, expiresIn time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, sentRecoveryMail{to: to, link: link, expiresIn: expiresIn})
	return f.err
}

func (f *fakeRecoveryMailer) mails() []sentRecoveryMail {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sentRecoveryMail(nil), f.sent...)
}

// recoveryMailTestEnv returns a handler whose recovery mail is delivered
// synchronously to a fake mailer, so a test can count sends right after the
// request returns.
func recoveryMailTestEnv(t *testing.T) (*Handler, *fakeRecoveryMailer) {
	t.Helper()
	h, _ := testHandlerEnv(t)
	mailer := &fakeRecoveryMailer{}
	h.recoveryMailer = mailer
	h.recoveryMailDispatch = func(f func()) { f() }
	return h, mailer
}

func postRecoveryRequest(t *testing.T, h *Handler, email, remoteAddr string) *httptest.ResponseRecorder {
	t.Helper()
	body := fmt.Sprintf(`{"email":%q}`, email)
	req := httptest.NewRequest(http.MethodPost, "/auth/recovery/request", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if remoteAddr != "" {
		req.RemoteAddr = remoteAddr
	}
	rec := httptest.NewRecorder()
	h.HandleRecoveryRequest(rec, req)
	return rec
}

func postRecoveryVerify(t *testing.T, h *Handler, token string) *httptest.ResponseRecorder {
	t.Helper()
	body := fmt.Sprintf(`{"token":%q}`, token)
	req := httptest.NewRequest(http.MethodPost, "/auth/recovery/verify", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.HandleRecoveryVerify(rec, req)
	return rec
}

func tokenFromRecoveryLink(t *testing.T, link string) string {
	t.Helper()
	const marker = RecoveryLinkPath + "#token="
	idx := strings.Index(link, marker)
	if idx < 0 {
		t.Fatalf("link %q has no %q", link, marker)
	}
	return link[idx+len(marker):]
}

func TestRecoveryRequestSendsExactlyOneMailWithLink(t *testing.T) {
	h, mailer := recoveryMailTestEnv(t)
	user, err := h.store.CreateUser("mail-user-1", "owner@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	rec := postRecoveryRequest(t, h, "owner@example.com", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}
	mails := mailer.mails()
	if len(mails) != 1 {
		t.Fatalf("mails sent = %d, want 1", len(mails))
	}
	m := mails[0]
	if m.to != user.Email {
		t.Fatalf("mail to %q, want the stored address %q", m.to, user.Email)
	}
	if m.expiresIn != RecoveryTokenTTL {
		t.Fatalf("expiresIn = %v, want %v", m.expiresIn, RecoveryTokenTTL)
	}
	wantPrefix := "http://localhost:4173/recover#token=" + RecoveryTokenPrefix
	if !strings.HasPrefix(m.link, wantPrefix) {
		t.Fatalf("link = %q, want prefix %q", m.link, wantPrefix)
	}
	token := tokenFromRecoveryLink(t, m.link)

	// The link's token is the one whose hash was stored for this user.
	var owner string
	if err := h.store.DB().QueryRow("SELECT user_id FROM recovery_tokens WHERE token_hash = ?", sha256SumHex(token)).Scan(&owner); err != nil {
		t.Fatalf("token from mail has no stored hash: %v", err)
	}
	if owner != user.ID {
		t.Fatalf("stored token user = %q, want %q", owner, user.ID)
	}
	if strings.Contains(rec.Body.String(), RecoveryTokenPrefix) {
		t.Fatalf("response body leaked the token: %s", rec.Body.String())
	}

	// The mailed link opens a passkey registration bound to the account.
	verify := postRecoveryVerify(t, h, token)
	if verify.Code != http.StatusOK {
		t.Fatalf("verify status = %d; body=%s", verify.Code, verify.Body.String())
	}
	var count int
	if err := h.store.DB().QueryRow("SELECT COUNT(*) FROM challenge_state WHERE user_id = ? AND type = 'registration'", user.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("registration challenges for user = %d (err %v), want 1", count, err)
	}
}

func TestRecoveryRequestUnknownEmailSendsNoMailAndAnswersIdentically(t *testing.T) {
	h, mailer := recoveryMailTestEnv(t)
	if _, err := h.store.CreateUser("mail-user-2", "known@example.com"); err != nil {
		t.Fatalf("create user: %v", err)
	}

	known := postRecoveryRequest(t, h, "known@example.com", "198.51.100.1:1000")
	unknown := postRecoveryRequest(t, h, "unknown@example.com", "198.51.100.2:1000")

	if known.Code != unknown.Code {
		t.Fatalf("status differs: known %d, unknown %d", known.Code, unknown.Code)
	}
	if known.Body.String() != unknown.Body.String() {
		t.Fatalf("body differs: known %q, unknown %q", known.Body.String(), unknown.Body.String())
	}
	if known.Header().Get("Content-Type") != unknown.Header().Get("Content-Type") {
		t.Fatalf("content type differs")
	}
	mails := mailer.mails()
	if len(mails) != 1 || mails[0].to != "known@example.com" {
		t.Fatalf("mails = %+v, want exactly one to the known address", mails)
	}
}

func TestRecoveryLinkRefusedAfterUse(t *testing.T) {
	h, mailer := recoveryMailTestEnv(t)
	if _, err := h.store.CreateUser("mail-user-3", "used@example.com"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	postRecoveryRequest(t, h, "used@example.com", "")
	token := tokenFromRecoveryLink(t, mailer.mails()[0].link)

	if rec := postRecoveryVerify(t, h, token); rec.Code != http.StatusOK {
		t.Fatalf("first verify = %d; body=%s", rec.Code, rec.Body.String())
	}
	if rec := postRecoveryVerify(t, h, token); rec.Code != http.StatusBadRequest {
		t.Fatalf("second verify = %d, want 400", rec.Code)
	}
}

func TestRecoveryLinkRefusedAfterExpiry(t *testing.T) {
	h, mailer := recoveryMailTestEnv(t)
	if _, err := h.store.CreateUser("mail-user-4", "late@example.com"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	postRecoveryRequest(t, h, "late@example.com", "")
	token := tokenFromRecoveryLink(t, mailer.mails()[0].link)
	if _, err := h.store.DB().Exec("UPDATE recovery_tokens SET expires_at = ? WHERE token_hash = ?",
		time.Now().UTC().Add(-time.Second), sha256SumHex(token)); err != nil {
		t.Fatalf("expire token: %v", err)
	}
	if rec := postRecoveryVerify(t, h, token); rec.Code != http.StatusBadRequest {
		t.Fatalf("verify expired = %d, want 400", rec.Code)
	}
}

func TestRecoveryNewLinkSupersedesOlderLink(t *testing.T) {
	h, mailer := recoveryMailTestEnv(t)
	if _, err := h.store.CreateUser("mail-user-5", "twice@example.com"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	postRecoveryRequest(t, h, "twice@example.com", "")
	postRecoveryRequest(t, h, "twice@example.com", "")
	mails := mailer.mails()
	if len(mails) != 2 {
		t.Fatalf("mails = %d, want 2", len(mails))
	}
	older := tokenFromRecoveryLink(t, mails[0].link)
	newer := tokenFromRecoveryLink(t, mails[1].link)
	if rec := postRecoveryVerify(t, h, older); rec.Code != http.StatusBadRequest {
		t.Fatalf("older link verify = %d, want 400", rec.Code)
	}
	if rec := postRecoveryVerify(t, h, newer); rec.Code != http.StatusOK {
		t.Fatalf("newer link verify = %d; body=%s", rec.Code, rec.Body.String())
	}
}

func TestRecoveryRateLimitStopsMail(t *testing.T) {
	h, mailer := recoveryMailTestEnv(t)
	if _, err := h.store.CreateUser("mail-user-6", "flood@example.com"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	for i := 0; i < RecoveryMaxPerEmail; i++ {
		if rec := postRecoveryRequest(t, h, "flood@example.com", fmt.Sprintf("198.51.100.%d:1000", 10+i)); rec.Code != http.StatusOK {
			t.Fatalf("request %d = %d", i+1, rec.Code)
		}
	}
	if rec := postRecoveryRequest(t, h, "flood@example.com", "198.51.100.99:1000"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("over-limit request = %d, want 429", rec.Code)
	}
	if got := len(mailer.mails()); got != RecoveryMaxPerEmail {
		t.Fatalf("mails = %d, want %d (none for the limited request)", got, RecoveryMaxPerEmail)
	}

	// An unknown address is limited the same way, so 429 reveals nothing.
	for i := 0; i < RecoveryMaxPerEmail; i++ {
		postRecoveryRequest(t, h, "nobody@example.com", fmt.Sprintf("198.51.100.%d:1000", 50+i))
	}
	if rec := postRecoveryRequest(t, h, "nobody@example.com", "198.51.100.98:1000"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("unknown over-limit request = %d, want 429", rec.Code)
	}
}

func TestRecoveryMailFailureKeepsResponseAndLogsNoToken(t *testing.T) {
	h, mailer := recoveryMailTestEnv(t)
	mailer.err = errors.New("maild recovery mail status 502")
	if _, err := h.store.CreateUser("mail-user-7", "down@example.com"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	var rec *httptest.ResponseRecorder
	logs := captureLogs(t, func() {
		rec = postRecoveryRequest(t, h, "down@example.com", "")
	})
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"ok":true}` {
		t.Fatalf("response = %d %s, want 200 {\"ok\":true}", rec.Code, rec.Body.String())
	}
	if !strings.Contains(logs, "operation=recovery_mail") || !strings.Contains(logs, "result=error") {
		t.Fatalf("mail failure not logged: %s", logs)
	}
	if strings.Contains(logs, RecoveryTokenPrefix) || strings.Contains(logs, "/recover#") {
		t.Fatalf("logs leaked the token or link:\n%s", logs)
	}
}

func TestRecoveryMailNotConfiguredAnswersIdentically(t *testing.T) {
	h, _ := testHandlerEnv(t) // TestConfig has no MaildURL: no mailer.
	if h.recoveryMailer != nil {
		t.Fatal("expected no mailer without MaildURL")
	}
	if _, err := h.store.CreateUser("mail-user-8", "nomailer@example.com"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	var rec *httptest.ResponseRecorder
	logs := captureLogs(t, func() {
		rec = postRecoveryRequest(t, h, "nomailer@example.com", "")
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(logs, "reason=mailer_not_configured") {
		t.Fatalf("missing not-configured log: %s", logs)
	}
}

func TestRecoveryLinkOriginComesFromConfigNotRequest(t *testing.T) {
	h, mailer := recoveryMailTestEnv(t)
	if _, err := h.store.CreateUser("mail-user-9", "origin@example.com"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/auth/recovery/request", bytes.NewBufferString(`{"email":"origin@example.com"}`))
	req.Host = "evil.example"
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("X-Forwarded-Host", "evil.example")
	rec := httptest.NewRecorder()
	h.HandleRecoveryRequest(rec, req)
	mails := mailer.mails()
	if len(mails) != 1 || strings.Contains(mails[0].link, "evil.example") || !strings.HasPrefix(mails[0].link, "http://localhost:4173/recover#") {
		t.Fatalf("mails = %+v", mails)
	}
}

func TestConsumeRecoveryTokenConcurrentUseHasOneWinner(t *testing.T) {
	store := TestStore(t)
	user, err := store.CreateUser("race-user", "race@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	token, err := store.CreateRecoveryToken(context.Background(), user.ID, user.Email, hashEmail(user.Email), hashIP("127.0.0.1"))
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	const n = 8
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := store.ConsumeRecoveryToken(context.Background(), token); err == nil {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("concurrent consumers that succeeded = %d, want 1", wins)
	}
}

// testMaild starts a real maild handler (behind the shared server
// middleware, so hostauth applies) whose Resend endpoint is a fake that
// records each send. No real email is sent.
type testMaild struct {
	srv      *httptest.Server
	mu       sync.Mutex
	payloads []map[string]any
}

func (m *testMaild) sends() []map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]map[string]any(nil), m.payloads...)
}

func startTestMaild(t *testing.T) *testMaild {
	t.Helper()
	m := &testMaild{}
	resend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p map[string]any
		_ = json.NewDecoder(r.Body).Decode(&p)
		m.mu.Lock()
		m.payloads = append(m.payloads, p)
		m.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"fake-resend-1"}`))
	}))
	t.Cleanup(resend.Close)

	dir := t.TempDir()
	mcfg := &maild.Config{
		Port:             "0",
		DBPath:           filepath.Join(dir, "mail.db"),
		StorageRoot:      filepath.Join(dir, "mail"),
		PrimaryDomain:    "choir.news",
		RootOwnerID:      "user-root",
		ResendAPIKey:     "re_test",
		ResendBaseURL:    resend.URL,
		WebhookMaxBytes:  maild.DefaultWebhookMaxBody,
		APIMaxBytes:      maild.DefaultAPIMaxBody,
		ProviderMaxBytes: maild.DefaultProviderMaxBody,
	}
	if err := mcfg.EnsureDirs(); err != nil {
		t.Fatalf("maild dirs: %v", err)
	}
	mstore, err := maild.OpenStore(mcfg.DBPath, mcfg.StorageRoot)
	if err != nil {
		t.Fatalf("maild store: %v", err)
	}
	t.Cleanup(func() { _ = mstore.Close() })
	if err := mstore.EnsureSchema(mcfg); err != nil {
		t.Fatalf("maild schema: %v", err)
	}
	ms := server.NewServer("maild", "0")
	maild.RegisterRoutes(ms, maild.NewHandler(mcfg, mstore))
	m.srv = httptest.NewServer(ms) // listens on 127.0.0.1: a loopback host caller
	t.Cleanup(m.srv.Close)
	return m
}

// TestRecoveryRequestDeliversThroughMaild runs the real path: auth's maild
// mailer posts to maild's internal route, and maild sends one fixed-template
// email through a fake Resend endpoint. No real email is sent.
func TestRecoveryRequestDeliversThroughMaild(t *testing.T) {
	m := startTestMaild(t)

	h, _ := testHandlerEnv(t)
	h.config.RPOrigins = []string{"https://choir.news"}
	h.recoveryMailer = NewMaildRecoveryMailer(m.srv.URL, m.srv.Client())
	done := make(chan struct{}, 4)
	h.recoveryMailDispatch = func(f func()) { f(); done <- struct{}{} }
	if _, err := h.store.CreateUser("maild-user", "owner@example.com"); err != nil {
		t.Fatalf("create user: %v", err)
	}

	logs := captureLogs(t, func() {
		if rec := postRecoveryRequest(t, h, "owner@example.com", ""); rec.Code != http.StatusOK {
			t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
		}
		if rec := postRecoveryRequest(t, h, "stranger@example.com", "198.51.100.7:1000"); rec.Code != http.StatusOK {
			t.Fatalf("unknown status = %d", rec.Code)
		}
	})
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("recovery mail dispatch did not run")
	}

	payloads := m.sends()
	if len(payloads) != 1 {
		t.Fatalf("resend sends = %d, want 1", len(payloads))
	}
	p := payloads[0]
	to, _ := p["to"].([]any)
	if len(to) != 1 || to[0] != "owner@example.com" {
		t.Fatalf("resend to = %v", p["to"])
	}
	text, _ := p["text"].(string)
	if !strings.Contains(text, "https://choir.news/recover#token="+RecoveryTokenPrefix) {
		t.Fatalf("mail text has no recovery link: %q", text)
	}
	if !strings.Contains(logs, "operation=recovery_mail") || !strings.Contains(logs, "result=sent") {
		t.Fatalf("auth did not log the send: %s", logs)
	}
	if strings.Contains(logs, RecoveryTokenPrefix) {
		t.Fatalf("logs leaked the token:\n%s", logs)
	}
}

// TestRecoveryMailCarriesHostCredentialUnderEnforce: with hostauth in
// enforce, maild's recovery route needs the host-internal credential as
// well as loopback. Auth's mailer attaches it (hostauth.MarkInternal) only
// to a loopback internal-service port; when the port is not one, the
// credential is withheld, maild refuses, and no mail is sent
// (docs/problems/loopback-is-a-credential-2026-10-14.md).
func TestRecoveryMailCarriesHostCredentialUnderEnforce(t *testing.T) {
	const token = "auth-recovery-hostauth-test-0123456789abcdef0123"
	m := startTestMaild(t)
	u, err := url.Parse(m.srv.URL)
	if err != nil {
		t.Fatalf("parse maild url: %v", err)
	}
	link := "https://choir.news/recover#token=" + RecoveryTokenPrefix + "AbCdEfGhIjKlMnOpQrStUvWxYz0123456789"
	mailer := NewMaildRecoveryMailer(m.srv.URL, m.srv.Client())

	t.Run("credential withheld from a non-internal port", func(t *testing.T) {
		t.Setenv(hostauth.EnvPorts, "1")
		restore := hostauth.SetForTesting(hostauth.ModeEnforce, token)
		defer restore()
		before := len(m.sends())
		if err := mailer.SendRecoveryLink(context.Background(), "owner@example.com", link, RecoveryTokenTTL); err == nil {
			t.Fatal("send without the credential succeeded, want maild to refuse")
		}
		if got := len(m.sends()); got != before {
			t.Fatalf("resend sends = %d, want %d (refused before sending)", got, before)
		}
	})
	t.Run("credential attached to the maild port", func(t *testing.T) {
		t.Setenv(hostauth.EnvPorts, u.Port())
		restore := hostauth.SetForTesting(hostauth.ModeEnforce, token)
		defer restore()
		before := len(m.sends())
		if err := mailer.SendRecoveryLink(context.Background(), "owner@example.com", link, RecoveryTokenTTL); err != nil {
			t.Fatalf("send with the credential: %v", err)
		}
		if got := len(m.sends()); got != before+1 {
			t.Fatalf("resend sends = %d, want %d", got, before+1)
		}
	})
}
