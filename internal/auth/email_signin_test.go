package auth

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Failure modes F1-F24 of docs/design/email-sign-in-2026-10-31.md (v3), as Go
// tests. Addresses are example.com / example.test; no real address appears.

const testEmailKey = "test-email-code-key-0123456789abcdef0123456789"

type fakeSignInMailer struct {
	mu    sync.Mutex
	sent  []SignInMail
	err   error
	calls int
}

func (f *fakeSignInMailer) SendSignInCode(_ context.Context, m SignInMail) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, m)
	return nil
}

func (f *fakeSignInMailer) mails() []SignInMail {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]SignInMail(nil), f.sent...)
}

func (f *fakeSignInMailer) last(t *testing.T) SignInMail {
	t.Helper()
	m := f.mails()
	if len(m) == 0 {
		t.Fatal("no sign-in mail was sent")
	}
	return m[len(m)-1]
}

func emailEnv(t *testing.T) (*Handler, *fakeSignInMailer) {
	t.Helper()
	h, _ := testHandlerEnv(t)
	h.config.EmailSignIn = true
	h.config.EmailCodeKey = []byte(testEmailKey)
	h.config.RPOrigins = []string{"https://choir.news"}
	h.initEmailSignIn()
	m := &fakeSignInMailer{}
	h.signInMailer = m
	h.signInMailDispatch = func(f func()) { f() }
	return h, m
}

// addEmailUser creates a user; verified and withPasskey control eligibility.
func addEmailUser(t *testing.T, h *Handler, id, email string, verified, withPasskey bool) *User {
	t.Helper()
	u, err := h.store.CreateUser(id, email)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if withPasskey {
		if err := h.store.CreateCredential(&Credential{
			ID: "cred-" + id, UserID: id, PublicKey: []byte("pk"), AttestationType: "none",
			Transport: "[]", AAGUID: make([]byte, 16), Flags: "{}", CreatedAt: time.Now().UTC(),
		}); err != nil {
			t.Fatalf("CreateCredential: %v", err)
		}
	}
	if verified {
		if err := h.store.SetEmailVerified(id); err != nil {
			t.Fatalf("SetEmailVerified: %v", err)
		}
	}
	u, _ = h.store.GetUserByID(id)
	return u
}

func jsonReq(method, path string, body any, ip string) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if ip != "" {
		req.RemoteAddr = ip + ":1234"
	}
	return req
}

func liveCookies(rec *httptest.ResponseRecorder) []*http.Cookie {
	var out []*http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Value != "" && c.MaxAge >= 0 {
			out = append(out, c)
		}
	}
	return out
}

func withCookies(req *http.Request, cs []*http.Cookie) *http.Request {
	for _, c := range cs {
		req.AddCookie(c)
	}
	return req
}

type beginResult struct {
	code      int
	attemptID string
	cookie    *http.Cookie
	body      string
}

func emailBegin(t *testing.T, h *Handler, email, ip string) beginResult {
	t.Helper()
	rec := httptest.NewRecorder()
	h.HandleEmailBegin(rec, jsonReq(http.MethodPost, "/auth/email/begin", map[string]string{"email": email}, ip))
	res := beginResult{code: rec.Code, body: rec.Body.String()}
	var resp emailBeginResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	res.attemptID = resp.AttemptID
	for _, c := range rec.Result().Cookies() {
		if c.Name == EmailAttemptCookieName {
			res.cookie = c
		}
	}
	return res
}

func emailVerifyCode(h *Handler, attemptID, code string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := jsonReq(http.MethodPost, "/auth/email/verify", map[string]string{"attempt_id": attemptID, "code": code}, "")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.HandleEmailVerify(rec, req)
	return rec
}

func emailVerifyToken(h *Handler, token string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.HandleEmailVerify(rec, jsonReq(http.MethodPost, "/auth/email/verify", map[string]string{"token": token}, ""))
	return rec
}

func tokenFromLink(t *testing.T, link string) string {
	t.Helper()
	i := strings.Index(link, "#token=")
	if i < 0 {
		t.Fatalf("link has no fragment token: %q", link)
	}
	return link[i+len("#token="):]
}

// signInByEmail runs begin and verify and returns the live session cookies.
func signInByEmail(t *testing.T, h *Handler, m *fakeSignInMailer, email string) []*http.Cookie {
	t.Helper()
	b := emailBegin(t, h, email, "")
	if b.code != http.StatusOK || b.cookie == nil {
		t.Fatalf("begin = %d", b.code)
	}
	rec := emailVerifyCode(h, b.attemptID, m.last(t).Code, b.cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("verify = %d %s", rec.Code, rec.Body.String())
	}
	return liveCookies(rec)
}

func passkeySession(t *testing.T, h *Handler, user *User) []*http.Cookie {
	t.Helper()
	rec := httptest.NewRecorder()
	if _, err := h.issueSession(rec, httptest.NewRequest(http.MethodGet, "/", nil), user, AuthMethodPasskey); err != nil {
		t.Fatalf("issueSession: %v", err)
	}
	return liveCookies(rec)
}

func refreshRowFor(t *testing.T, h *Handler, cs []*http.Cookie) *RefreshSession {
	t.Helper()
	for _, c := range cs {
		if c.Name == RefreshTokenCookieName {
			rs, err := h.store.GetRefreshSessionByTokenHash(sha256SumHex(c.Value))
			if err != nil {
				t.Fatalf("refresh row: %v", err)
			}
			return rs
		}
	}
	t.Fatal("no refresh cookie")
	return nil
}

func cookieByName(cs []*http.Cookie, name string) *http.Cookie {
	for _, c := range cs {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// --- F1-F3, F5, F16, F18, F22 ---

func TestF1WrongCodeFiveTimesKillsAttempt(t *testing.T) {
	h, m := emailEnv(t)
	addEmailUser(t, h, "u1", "a@example.com", true, true)
	b := emailBegin(t, h, "a@example.com", "")
	code := m.last(t).Code
	wrong := "00000000"
	if code == wrong {
		wrong = "11111111"
	}
	for i := 0; i < EmailMaxWrongCodes; i++ {
		if rec := emailVerifyCode(h, b.attemptID, wrong, b.cookie); rec.Code != http.StatusBadRequest {
			t.Fatalf("wrong code %d = %d", i, rec.Code)
		}
	}
	if rec := emailVerifyCode(h, b.attemptID, code, b.cookie); rec.Code != http.StatusBadRequest {
		t.Fatalf("right code after 5 wrong = %d, want 400", rec.Code)
	}
}

func TestF2ExpiredCodeFails(t *testing.T) {
	h, m := emailEnv(t)
	addEmailUser(t, h, "u1", "a@example.com", true, true)
	b := emailBegin(t, h, "a@example.com", "")
	if _, err := h.store.DB().Exec("UPDATE email_signin_attempts SET expires_at = ?", time.Now().UTC().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if rec := emailVerifyCode(h, b.attemptID, m.last(t).Code, b.cookie); rec.Code != http.StatusBadRequest {
		t.Fatalf("expired code = %d", rec.Code)
	}
}

func TestF3ConsumedCodeAndLinkFailOnSecondUse(t *testing.T) {
	h, m := emailEnv(t)
	addEmailUser(t, h, "u1", "a@example.com", true, true)
	b := emailBegin(t, h, "a@example.com", "")
	code := m.last(t).Code
	if rec := emailVerifyCode(h, b.attemptID, code, b.cookie); rec.Code != http.StatusOK {
		t.Fatalf("first use = %d", rec.Code)
	}
	if rec := emailVerifyCode(h, b.attemptID, code, b.cookie); rec.Code != http.StatusBadRequest {
		t.Fatalf("second use = %d", rec.Code)
	}
	emailBegin(t, h, "a@example.com", "")
	tok := tokenFromLink(t, m.last(t).Link)
	if rec := emailVerifyToken(h, tok); rec.Code != http.StatusOK {
		t.Fatalf("link first use = %d", rec.Code)
	}
	if rec := emailVerifyToken(h, tok); rec.Code != http.StatusBadRequest {
		t.Fatalf("link second use = %d", rec.Code)
	}
}

func TestF3ConcurrentVerifiesExactlyOneWins(t *testing.T) {
	h, m := emailEnv(t)
	addEmailUser(t, h, "u1", "a@example.com", true, true)
	b := emailBegin(t, h, "a@example.com", "")
	code := m.last(t).Code
	tok := tokenFromLink(t, m.last(t).Link)
	run := func(name string, f func() *httptest.ResponseRecorder) {
		var wg sync.WaitGroup
		var mu sync.Mutex
		wins := 0
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if f().Code == http.StatusOK {
					mu.Lock()
					wins++
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
		if wins != 1 {
			t.Fatalf("%s: winners = %d, want 1", name, wins)
		}
	}
	// Distinct source IPs are not needed: verify limit is 30/h per IP.
	run("code", func() *httptest.ResponseRecorder { return emailVerifyCode(h, b.attemptID, code, b.cookie) })
	b2 := emailBegin(t, h, "a@example.com", "")
	_ = b2
	tok = tokenFromLink(t, m.last(t).Link)
	run("link", func() *httptest.ResponseRecorder { return emailVerifyToken(h, tok) })
	_ = tok
}

func TestF5CodeWorksOnlyWithItsAttemptID(t *testing.T) {
	h, m := emailEnv(t)
	addEmailUser(t, h, "u1", "a@example.com", true, true)
	addEmailUser(t, h, "u2", "b@example.com", true, true)
	b1 := emailBegin(t, h, "a@example.com", "")
	code1 := m.last(t).Code
	b2 := emailBegin(t, h, "b@example.com", "")
	// u1's code against u2's attempt (with u2's cookie), and against a made-up id.
	if rec := emailVerifyCode(h, b2.attemptID, code1, b2.cookie); rec.Code != http.StatusBadRequest {
		t.Fatalf("code against another attempt = %d", rec.Code)
	}
	if rec := emailVerifyCode(h, strings.Repeat("a", 32), code1, b1.cookie); rec.Code != http.StatusBadRequest {
		t.Fatalf("code against unknown attempt = %d", rec.Code)
	}
	// u1's cookie against u2's attempt id with u2's real code.
	if rec := emailVerifyCode(h, b2.attemptID, m.last(t).Code, b1.cookie); rec.Code != http.StatusBadRequest {
		t.Fatalf("another client's cookie = %d", rec.Code)
	}
}

func TestF16CodeVerifyWithoutAttemptCookieFails(t *testing.T) {
	h, m := emailEnv(t)
	addEmailUser(t, h, "u1", "a@example.com", true, true)
	b := emailBegin(t, h, "a@example.com", "")
	if rec := emailVerifyCode(h, b.attemptID, m.last(t).Code, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("no cookie = %d", rec.Code)
	}
	// The failure takes no guess: the right code still works with the cookie.
	if rec := emailVerifyCode(h, b.attemptID, m.last(t).Code, b.cookie); rec.Code != http.StatusOK {
		t.Fatalf("with cookie = %d", rec.Code)
	}
}

func TestAttemptCookieAttributes(t *testing.T) {
	h, _ := emailEnv(t)
	h.config.CookieSecure = true
	b := emailBegin(t, h, "nobody@example.com", "")
	c := b.cookie
	if c == nil || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode || c.Path != "/auth/email" || c.MaxAge != 600 {
		t.Fatalf("attempt cookie = %+v", c)
	}
	h.config.CookieSecure = false
	if c := emailBegin(t, h, "nobody2@example.com", "").cookie; c == nil || c.Secure {
		t.Fatalf("Secure must follow CookieSecure: %+v", c)
	}
}

func TestLinkTokenWorksWithoutAttemptCookie(t *testing.T) {
	h, m := emailEnv(t)
	addEmailUser(t, h, "u1", "a@example.com", true, true)
	emailBegin(t, h, "a@example.com", "")
	if rec := emailVerifyToken(h, tokenFromLink(t, m.last(t).Link)); rec.Code != http.StatusOK {
		t.Fatalf("link without cookie = %d", rec.Code)
	}
}

func TestF18SecondBeginKillsFirstCodeAndLink(t *testing.T) {
	h, m := emailEnv(t)
	addEmailUser(t, h, "u1", "a@example.com", true, true)
	b1 := emailBegin(t, h, "a@example.com", "")
	first := m.last(t)
	b2 := emailBegin(t, h, "a@example.com", "")
	second := m.last(t)
	if rec := emailVerifyCode(h, b1.attemptID, first.Code, b1.cookie); rec.Code != http.StatusBadRequest {
		t.Fatalf("first code after second begin = %d", rec.Code)
	}
	if rec := emailVerifyToken(h, tokenFromLink(t, first.Link)); rec.Code != http.StatusBadRequest {
		t.Fatalf("first link after second begin = %d", rec.Code)
	}
	if rec := emailVerifyCode(h, b2.attemptID, second.Code, b2.cookie); rec.Code != http.StatusOK {
		t.Fatalf("second code = %d", rec.Code)
	}
}

func TestF22VerifyRequiresJSONContentType(t *testing.T) {
	h, m := emailEnv(t)
	addEmailUser(t, h, "u1", "a@example.com", true, true)
	b := emailBegin(t, h, "a@example.com", "")
	body := fmt.Sprintf(`{"attempt_id":%q,"code":%q}`, b.attemptID, m.last(t).Code)
	for _, ct := range []string{"text/plain", "application/x-www-form-urlencoded", "multipart/form-data", ""} {
		req := httptest.NewRequest(http.MethodPost, "/auth/email/verify", strings.NewReader(body))
		if ct != "" {
			req.Header.Set("Content-Type", ct)
		}
		req.AddCookie(b.cookie)
		rec := httptest.NewRecorder()
		h.HandleEmailVerify(rec, req)
		if rec.Code != http.StatusUnsupportedMediaType {
			t.Fatalf("content type %q = %d, want 415", ct, rec.Code)
		}
	}
	// A charset parameter is fine.
	req := httptest.NewRequest(http.MethodPost, "/auth/email/verify", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.AddCookie(b.cookie)
	rec := httptest.NewRecorder()
	h.HandleEmailVerify(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("json+charset = %d", rec.Code)
	}
}

// --- F4, F9, F10, F11, F17, F21, item 9, item 12 ---

func TestF4UnknownAddressSameShapeWritesRowSendsNothing(t *testing.T) {
	h, m := emailEnv(t)
	addEmailUser(t, h, "u1", "a@example.com", true, true)
	known := emailBegin(t, h, "a@example.com", "")
	unknown := emailBegin(t, h, "nobody@example.test", "")
	if known.code != unknown.code || known.code != http.StatusOK {
		t.Fatalf("status known=%d unknown=%d", known.code, unknown.code)
	}
	if len(known.attemptID) != len(unknown.attemptID) || known.cookie == nil || unknown.cookie == nil {
		t.Fatalf("shape differs: %q vs %q", known.body, unknown.body)
	}
	if len(m.mails()) != 1 {
		t.Fatalf("mails = %d, want 1 (known only)", len(m.mails()))
	}
	a, err := h.store.GetEmailAttempt(context.Background(), unknown.attemptID)
	if err != nil || a.UserID != "" {
		t.Fatalf("unknown row: %+v err=%v", a, err)
	}
	// Verify on it fails exactly like a wrong code.
	if rec := emailVerifyCode(h, unknown.attemptID, "12345678", unknown.cookie); rec.Code != http.StatusBadRequest {
		t.Fatalf("verify on unknown row = %d", rec.Code)
	}
}

func TestF6LimitsHoldAcrossConcurrentRequests(t *testing.T) {
	h, _ := emailEnv(t)
	addEmailUser(t, h, "u1", "a@example.com", true, true)
	count := func(email string, ipFor func(i int) string, n int) (ok, limited int) {
		var wg sync.WaitGroup
		var mu sync.Mutex
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				r := emailBegin(t, h, email, ipFor(i))
				mu.Lock()
				defer mu.Unlock()
				switch r.code {
				case http.StatusOK:
					ok++
				case http.StatusTooManyRequests:
					limited++
				default:
					t.Errorf("status %d", r.code)
				}
			}(i)
		}
		wg.Wait()
		return
	}
	// Per email: 5 per hour whether or not the address has an account.
	if ok, limited := count("a@example.com", func(i int) string { return fmt.Sprintf("198.51.100.%d", i+1) }, 20); ok != EmailMaxBeginPerEmail || limited != 15 {
		t.Fatalf("per email: ok=%d limited=%d", ok, limited)
	}
	if ok, _ := count("nobody@example.test", func(i int) string { return fmt.Sprintf("198.51.101.%d", i+1) }, 12); ok != EmailMaxBeginPerEmail {
		t.Fatalf("unknown address per email: ok=%d", ok)
	}
	// Per IP: 10 per hour across different addresses.
	if ok, limited := count("x@example.test", func(int) string { return "203.0.113.9" }, 1); ok != 1 || limited != 0 {
		t.Fatalf("seed: %d %d", ok, limited)
	}
	okTotal := 1
	for i := 0; i < 20; i++ {
		r := emailBegin(t, h, fmt.Sprintf("u%d@example.test", i), "203.0.113.9")
		if r.code == http.StatusOK {
			okTotal++
		}
	}
	if okTotal != EmailMaxBeginPerIP {
		t.Fatalf("per ip: ok=%d, want %d", okTotal, EmailMaxBeginPerIP)
	}
}

func TestVerifyPerIPLimit(t *testing.T) {
	h, _ := emailEnv(t)
	for i := 0; i < EmailMaxVerifyPerIP; i++ {
		if rec := emailVerifyCode(h, strings.Repeat("b", 32), "12345678", nil); rec.Code != http.StatusBadRequest {
			t.Fatalf("verify %d = %d", i, rec.Code)
		}
	}
	if rec := emailVerifyCode(h, strings.Repeat("b", 32), "12345678", nil); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("31st verify = %d, want 429", rec.Code)
	}
}

func TestF9MailerDownBeginStillAnswersAttemptUnsentAndLogged(t *testing.T) {
	h, m := emailEnv(t)
	addEmailUser(t, h, "u1", "a@example.com", true, true)
	m.err = errors.New("maild down")
	var b beginResult
	logs := captureLogs(t, func() { b = emailBegin(t, h, "a@example.com", "") })
	if b.code != http.StatusOK || b.attemptID == "" {
		t.Fatalf("begin with mailer down = %d", b.code)
	}
	a, err := h.store.GetEmailAttempt(context.Background(), b.attemptID)
	if err != nil || a.Sent {
		t.Fatalf("attempt should be unsent: %+v err=%v", a, err)
	}
	if !strings.Contains(logs, "sent=false") || !strings.Contains(logs, "email_signin_mail") {
		t.Fatalf("log does not name the unsent attempt: %q", logs)
	}
	// And the sent flag flips on success.
	m.err = nil
	b = emailBegin(t, h, "a@example.com", "")
	if a, _ := h.store.GetEmailAttempt(context.Background(), b.attemptID); !a.Sent {
		t.Fatal("attempt should be marked sent")
	}
}

func TestF10MailTextHasNeitherAttemptIDNorTokenHash(t *testing.T) {
	h, m := emailEnv(t)
	addEmailUser(t, h, "u1", "a@example.com", true, true)
	b := emailBegin(t, h, "a@example.com", "")
	a, _ := h.store.GetEmailAttempt(context.Background(), b.attemptID)
	mail := m.last(t)
	blob := fmt.Sprintf("%+v", mail)
	for _, secret := range []string{b.attemptID, a.LinkTokenHash, a.CodeHash, a.CookieHash, b.cookie.Value} {
		if strings.Contains(blob, secret) {
			t.Fatalf("mail carries a secret handle %q", secret[:8])
		}
	}
	if !strings.HasPrefix(mail.Link, "https://choir.news/signin/email#token=choir_st_") {
		t.Fatalf("link = %q", mail.Link)
	}
	if mail.Device == "" || mail.Place == "" {
		t.Fatalf("device/place missing: %+v", mail)
	}
}

func TestF11AuthNeverLogsCodeTokenOrAddress(t *testing.T) {
	h, m := emailEnv(t)
	addEmailUser(t, h, "u1", "secret.person@example.com", true, true)
	var code, tok string
	logs := captureLogs(t, func() {
		b := emailBegin(t, h, "secret.person@example.com", "")
		code, tok = m.last(t).Code, tokenFromLink(t, m.last(t).Link)
		emailVerifyCode(h, b.attemptID, "00000000", b.cookie)
		emailVerifyCode(h, b.attemptID, code, b.cookie)
		emailBegin(t, h, "stranger@example.test", "")
		emailVerifyToken(h, tok)
		emailVerifyToken(h, "choir_st_notatoken")
	})
	for _, bad := range []string{code, tok, "secret.person", "stranger@", "choir_st_notatoken"} {
		if strings.Contains(logs, bad) {
			t.Fatalf("log leaks %q:\n%s", bad, logs)
		}
	}
}

func TestF17NoPasskeyNoMail(t *testing.T) {
	h, m := emailEnv(t)
	addEmailUser(t, h, "u1", "nokey@example.com", true, false)
	b := emailBegin(t, h, "nokey@example.com", "")
	if b.code != http.StatusOK || len(m.mails()) != 0 {
		t.Fatalf("status=%d mails=%d", b.code, len(m.mails()))
	}
}

func TestF21UnverifiedAddressNoMail(t *testing.T) {
	h, m := emailEnv(t)
	addEmailUser(t, h, "u1", "unverified@example.com", false, true)
	b := emailBegin(t, h, "unverified@example.com", "")
	if b.code != http.StatusOK || len(m.mails()) != 0 {
		t.Fatalf("status=%d mails=%d", b.code, len(m.mails()))
	}
	if rec := emailVerifyCode(h, b.attemptID, "12345678", b.cookie); rec.Code != http.StatusBadRequest {
		t.Fatalf("verify = %d", rec.Code)
	}
}

func TestLookupIsExactAsStored(t *testing.T) {
	h, m := emailEnv(t)
	addEmailUser(t, h, "u1", "Mixed.Case@example.com", true, true)
	emailBegin(t, h, "mixed.case@example.com", "")
	if len(m.mails()) != 0 {
		t.Fatal("a differently cased address must not match (exact lookup, as recovery does)")
	}
	emailBegin(t, h, "  Mixed.Case@example.com ", "")
	if len(m.mails()) != 1 {
		t.Fatalf("exact (trimmed) address should be mailed, mails=%d", len(m.mails()))
	}
	// The rate-limit hash is normalized: differently cased requests share a budget.
	a1 := h.emailAddressHash(normalizeEmailAddress("Mixed.Case@example.com"))
	a2 := h.emailAddressHash(normalizeEmailAddress("mixed.case@EXAMPLE.com"))
	if a1 != a2 {
		t.Fatal("email_hash must normalize case")
	}
}

func TestMailIsSentOffTheRequestPath(t *testing.T) {
	h, m := emailEnv(t)
	addEmailUser(t, h, "u1", "a@example.com", true, true)
	var queued []func()
	h.signInMailDispatch = func(f func()) { queued = append(queued, f) }
	emailBegin(t, h, "a@example.com", "")
	if len(queued) != 1 || len(m.mails()) != 0 {
		t.Fatalf("known: queued=%d sent-in-request=%d", len(queued), len(m.mails()))
	}
	emailBegin(t, h, "nobody@example.test", "")
	if len(queued) != 1 {
		t.Fatalf("unknown address must not queue mail, queued=%d", len(queued))
	}
	queued[0]()
	if len(m.mails()) != 1 {
		t.Fatal("queued mail did not send")
	}
}

func TestEmailHashAndCodeHashAreKeyed(t *testing.T) {
	h, m := emailEnv(t)
	addEmailUser(t, h, "u1", "a@example.com", true, true)
	b := emailBegin(t, h, "a@example.com", "")
	a, _ := h.store.GetEmailAttempt(context.Background(), b.attemptID)
	if a.EmailHash == sha256SumHex("a@example.com") || a.CodeHash == sha256SumHex(m.last(t).Code) {
		t.Fatal("hashes must be HMACs under the key, not bare SHA-256")
	}
	if strings.Contains(a.CodeHash, m.last(t).Code) {
		t.Fatal("code hash leaks code")
	}
}

// --- config: fail closed ---

func TestConfigFailsClosedWithSwitchOnAndNoKey(t *testing.T) {
	t.Setenv("AUTH_EMAIL_SIGN_IN", "on")
	t.Setenv("AUTH_EMAIL_CODE_KEY", "")
	t.Setenv("AUTH_EMAIL_CODE_KEY_FILE", "")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("LoadConfig must fail with the switch on and no key")
	}
	t.Setenv("AUTH_EMAIL_CODE_KEY", "short")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("LoadConfig must reject a short key")
	}
	t.Setenv("AUTH_EMAIL_CODE_KEY", testEmailKey)
	cfg, err := LoadConfig()
	if err != nil || !cfg.EmailSignIn {
		t.Fatalf("LoadConfig with key: %v", err)
	}
	keyFile := filepath.Join(t.TempDir(), "k")
	if err := os.WriteFile(keyFile, []byte(testEmailKey+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AUTH_EMAIL_CODE_KEY", "")
	t.Setenv("AUTH_EMAIL_CODE_KEY_FILE", keyFile)
	if cfg, err := LoadConfig(); err != nil || string(cfg.EmailCodeKey) != testEmailKey {
		t.Fatalf("key file: %v", err)
	}
	t.Setenv("AUTH_EMAIL_SIGN_IN", "")
	t.Setenv("AUTH_EMAIL_CODE_KEY_FILE", "")
	if cfg, err := LoadConfig(); err != nil || cfg.EmailSignIn {
		t.Fatalf("switch off needs no key: %v", err)
	}
}

// --- sessions: method carried, F14, F15, F19, item 1 ---

func TestEmailSessionCarriesMethodInJWTAndRow(t *testing.T) {
	h, m := emailEnv(t)
	addEmailUser(t, h, "u1", "a@example.com", true, true)
	cs := signInByEmail(t, h, m, "a@example.com")
	rs := refreshRowFor(t, h, cs)
	if rs.AuthMethod != AuthMethodEmailCode {
		t.Fatalf("row method = %q", rs.AuthMethod)
	}
	_, am, err := h.validateAccessTokenMethod(cookieByName(cs, AccessTokenCookieName).Value)
	if err != nil || am != AuthMethodEmailCode {
		t.Fatalf("am = %q err=%v", am, err)
	}
	// 7 days absolute, not 30.
	if d := time.Until(rs.ExpiresAt); d > 7*24*time.Hour+time.Minute || d < 7*24*time.Hour-time.Minute {
		t.Fatalf("email session expiry in %v", d)
	}
	// The link variant records email_link.
	emailBegin(t, h, "a@example.com", "")
	rec := emailVerifyToken(h, tokenFromLink(t, m.last(t).Link))
	if rs := refreshRowFor(t, h, liveCookies(rec)); rs.AuthMethod != AuthMethodEmailLink {
		t.Fatalf("link method = %q", rs.AuthMethod)
	}
	// A passkey session keeps 30 days and the passkey claim.
	pk := passkeySession(t, h, mustUser(t, h, "u1"))
	if rs := refreshRowFor(t, h, pk); rs.AuthMethod != AuthMethodPasskey || time.Until(rs.ExpiresAt) < 29*24*time.Hour {
		t.Fatalf("passkey row: %+v", rs)
	}
}

func mustUser(t *testing.T, h *Handler, id string) *User {
	t.Helper()
	u, err := h.store.GetUserByID(id)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestAccessJWTWithoutAMReadsAsPasskey(t *testing.T) {
	h, _ := emailEnv(t)
	u := addEmailUser(t, h, "u1", "a@example.com", true, true)
	tok, err := h.issueAccessJWT(u, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, am, err := h.validateAccessTokenMethod(tok); err != nil || am != AuthMethodPasskey {
		t.Fatalf("am = %q err=%v", am, err)
	}
}

func TestF14MethodSurvivesTenRotationsWithoutExtendingExpiry(t *testing.T) {
	h, m := emailEnv(t)
	addEmailUser(t, h, "u1", "a@example.com", true, true)
	cs := signInByEmail(t, h, m, "a@example.com")
	origin := refreshRowFor(t, h, cs).ExpiresAt
	for i := 0; i < 10; i++ {
		req := withCookies(httptest.NewRequest(http.MethodGet, "/auth/session", nil), cs)
		// Force the refresh path by dropping the access cookie.
		req.Header.Del("Cookie")
		req.AddCookie(cookieByName(cs, RefreshTokenCookieName))
		rec := httptest.NewRecorder()
		h.HandleSession(rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"authenticated":true`) {
			t.Fatalf("rotation %d: %d %s", i, rec.Code, rec.Body.String())
		}
		cs = liveCookies(rec)
		rs := refreshRowFor(t, h, cs)
		if rs.AuthMethod != AuthMethodEmailCode {
			t.Fatalf("rotation %d: method = %q", i, rs.AuthMethod)
		}
		if !rs.ExpiresAt.Equal(origin) {
			t.Fatalf("rotation %d moved the expiry: %v -> %v", i, origin, rs.ExpiresAt)
		}
		if _, am, err := h.validateAccessTokenMethod(cookieByName(cs, AccessTokenCookieName).Value); err != nil || am != AuthMethodEmailCode {
			t.Fatalf("rotation %d: am = %q err=%v", i, am, err)
		}
	}
}

func TestF19EmailSessionPastSevenDaysCannotRefresh(t *testing.T) {
	h, m := emailEnv(t)
	addEmailUser(t, h, "u1", "a@example.com", true, true)
	cs := signInByEmail(t, h, m, "a@example.com")
	if _, err := h.store.DB().Exec("UPDATE refresh_sessions SET expires_at = ?", time.Now().UTC().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/auth/session", nil)
	req.AddCookie(cookieByName(cs, RefreshTokenCookieName))
	rec := httptest.NewRecorder()
	h.HandleSession(rec, req)
	if !strings.Contains(rec.Body.String(), `"authenticated":false`) {
		t.Fatalf("expired email session refreshed: %s", rec.Body.String())
	}
}

func exchangeToApp(t *testing.T, h *Handler, cs []*http.Cookie) (*httptest.ResponseRecorder, string) {
	t.Helper()
	req := withCookies(httptest.NewRequest(http.MethodGet, "/auth/desktop/exchange-redirect", nil), cs)
	rec := httptest.NewRecorder()
	h.HandleDesktopExchangeRedirect(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("exchange-redirect = %d %s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	i := strings.Index(loc, "code=")
	if i < 0 {
		t.Fatalf("location %q", loc)
	}
	return rec, loc[i+5:]
}

func redeem(t *testing.T, h *Handler, code string) []*http.Cookie {
	t.Helper()
	rec := httptest.NewRecorder()
	h.HandleDesktopRedeem(rec, jsonReq(http.MethodPost, "/auth/desktop/redeem", map[string]string{"code": code}, ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("redeem = %d %s", rec.Code, rec.Body.String())
	}
	return liveCookies(rec)
}

func TestF15ExchangedEmailSessionStaysEmailAndNeverExtends(t *testing.T) {
	h, m := emailEnv(t)
	addEmailUser(t, h, "u1", "a@example.com", true, true)
	cs := signInByEmail(t, h, m, "a@example.com")
	origin := refreshRowFor(t, h, cs).ExpiresAt

	// Age the source session: it was issued six days ago and expires in one.
	soon := time.Now().UTC().Add(24 * time.Hour)
	if _, err := h.store.DB().Exec("UPDATE refresh_sessions SET expires_at = ?", soon); err != nil {
		t.Fatal(err)
	}
	_ = origin
	for i := 0; i < 3; i++ {
		_, code := exchangeToApp(t, h, cs)
		cs = redeem(t, h, code)
		rs := refreshRowFor(t, h, cs)
		if rs.AuthMethod != AuthMethodEmailCode {
			t.Fatalf("exchange %d: method = %q", i, rs.AuthMethod)
		}
		if rs.ExpiresAt.After(soon.Add(time.Second)) {
			t.Fatalf("exchange %d re-based the session past its original expiry: %v > %v", i, rs.ExpiresAt, soon)
		}
		if _, am, _ := h.validateAccessTokenMethod(cookieByName(cs, AccessTokenCookieName).Value); am != AuthMethodEmailCode {
			t.Fatalf("exchange %d: am = %q", i, am)
		}
	}
	// A passkey session exchanged stays passkey with the full term.
	pk := passkeySession(t, h, mustUser(t, h, "u1"))
	_, code := exchangeToApp(t, h, pk)
	app := redeem(t, h, code)
	if rs := refreshRowFor(t, h, app); rs.AuthMethod != AuthMethodPasskey || time.Until(rs.ExpiresAt) < 29*24*time.Hour {
		t.Fatalf("passkey exchange: %+v", rs)
	}
}

// --- gate: F7, F8, F20, items 4, 5 ---

type gatedRoute struct {
	name   string
	method string
	path   string
	body   any
}

func gatedRoutes() []gatedRoute {
	return []gatedRoute{
		{"rename credential", http.MethodPost, "/auth/credentials/rename", map[string]string{"id": "cred-u1", "name": "x"}},
		{"delete credential", http.MethodDelete, "/auth/credentials/cred-u1", nil},
		{"create api key", http.MethodPost, "/auth/api-keys", map[string]any{"label": "k", "scopes": []string{"read:texture"}}},
		{"revoke api key", http.MethodDelete, "/auth/api-keys/some-key", nil},
		{"revoke session", http.MethodDelete, "/auth/sessions/some-session", nil},
		{"logout all", http.MethodPost, "/auth/logout?all=true", nil},
		{"logout all_devices", http.MethodPost, "/auth/logout?all_devices=true", nil},
		{"root status", http.MethodGet, "/api/auth/root/status", nil},
	}
}

func (h *Handler) gatedHandler(path string) http.HandlerFunc {
	switch {
	case strings.HasPrefix(path, "/auth/credentials/rename"):
		return h.HandleRenameCredential
	case strings.HasPrefix(path, "/auth/credentials/"):
		return h.HandleDeleteCredential
	case path == "/auth/api-keys":
		return h.HandleCreateAPIKey
	case strings.HasPrefix(path, "/auth/api-keys/"):
		return h.HandleRevokeAPIKey
	case strings.HasPrefix(path, "/auth/sessions/"):
		return h.HandleRevokeSession
	case strings.HasPrefix(path, "/auth/logout"):
		return h.HandleLogout
	default:
		return h.HandleRootStatus
	}
}

func callGated(h *Handler, g gatedRoute, cs []*http.Cookie) *httptest.ResponseRecorder {
	req := withCookies(jsonReq(g.method, g.path, g.body, ""), cs)
	rec := httptest.NewRecorder()
	h.gatedHandler(strings.SplitN(g.path, "?", 2)[0])(rec, req)
	return rec
}

func TestF7F20GatedRoutesRefuseEmailSessionAcceptPasskeySession(t *testing.T) {
	h, m := emailEnv(t)
	addEmailUser(t, h, "u1", "a@example.com", true, true)
	emailCS := signInByEmail(t, h, m, "a@example.com")
	pkCS := passkeySession(t, h, mustUser(t, h, "u1"))
	for _, g := range gatedRoutes() {
		t.Run("email/"+g.name, func(t *testing.T) {
			rec := callGated(h, g, emailCS)
			if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "Sign in with a passkey to do this.") {
				t.Fatalf("email session: %d %s", rec.Code, rec.Body.String())
			}
			// A refused call changes nothing (logout-all deleted no rows).
			if _, err := h.store.GetRefreshSessionByTokenHash(sha256SumHex(cookieByName(emailCS, RefreshTokenCookieName).Value)); err != nil {
				t.Fatalf("email session row gone after a refused call: %v", err)
			}
		})
	}
	for _, g := range gatedRoutes() {
		t.Run("passkey/"+g.name, func(t *testing.T) {
			rec := callGated(h, g, pkCS)
			if rec.Code == http.StatusForbidden || rec.Code == http.StatusUnauthorized {
				t.Fatalf("passkey session refused: %d %s", rec.Code, rec.Body.String())
			}
			if strings.HasPrefix(g.name, "logout") { // the passkey logout-all ended its session
				pkCS = passkeySession(t, h, mustUser(t, h, "u1"))
			}
		})
	}
}

func TestGateLogoutAllUsesRefreshRowWhenAccessJWTUnreadable(t *testing.T) {
	h, m := emailEnv(t)
	addEmailUser(t, h, "u1", "a@example.com", true, true)
	cs := signInByEmail(t, h, m, "a@example.com")
	req := httptest.NewRequest(http.MethodPost, "/auth/logout?all_devices=true", nil)
	req.AddCookie(&http.Cookie{Name: AccessTokenCookieName, Value: "garbage.token.value"})
	req.AddCookie(cookieByName(cs, RefreshTokenCookieName))
	rec := httptest.NewRecorder()
	h.HandleLogout(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("logout-all with unreadable access JWT and an email refresh row = %d, want 403", rec.Code)
	}
	// A passkey refresh row with the same unreadable access JWT is allowed.
	pk := passkeySession(t, h, mustUser(t, h, "u1"))
	req = httptest.NewRequest(http.MethodPost, "/auth/logout?all=true", nil)
	req.AddCookie(&http.Cookie{Name: AccessTokenCookieName, Value: "garbage.token.value"})
	req.AddCookie(cookieByName(pk, RefreshTokenCookieName))
	rec = httptest.NewRecorder()
	h.HandleLogout(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("passkey logout-all = %d", rec.Code)
	}
}

func TestSelfLogoutAndReadsStayOpenToEmailSession(t *testing.T) {
	h, m := emailEnv(t)
	addEmailUser(t, h, "u1", "a@example.com", true, true)
	cs := signInByEmail(t, h, m, "a@example.com")
	for name, call := range map[string]func(*httptest.ResponseRecorder, *http.Request){
		"list credentials": func(w *httptest.ResponseRecorder, r *http.Request) { h.HandleListCredentials(w, r) },
		"list sessions":    func(w *httptest.ResponseRecorder, r *http.Request) { h.HandleListSessions(w, r) },
		"list api keys":    func(w *httptest.ResponseRecorder, r *http.Request) { h.HandleListAPIKeys(w, r) },
	} {
		req := withCookies(httptest.NewRequest(http.MethodGet, "/", nil), cs)
		rec := httptest.NewRecorder()
		call(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s = %d", name, rec.Code)
		}
	}
	req := withCookies(httptest.NewRequest(http.MethodPost, "/auth/logout", nil), cs)
	rec := httptest.NewRecorder()
	h.HandleLogout(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("self logout = %d", rec.Code)
	}
}

func TestGateBearerKeyKeepsItsScopeRules(t *testing.T) {
	h, _ := emailEnv(t)
	addEmailUser(t, h, "u1", "a@example.com", true, true)
	_, secret, err := h.store.CreateAPIKey(context.Background(), "u1", "admin", []string{"admin"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	req := jsonReq(http.MethodPost, "/auth/api-keys", map[string]any{"label": "sub", "scopes": []string{"read:texture"}}, "")
	req.Header.Set("Authorization", "Bearer "+secret)
	rec := httptest.NewRecorder()
	h.HandleCreateAPIKey(rec, req)
	if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
		t.Fatalf("Bearer key create = %d %s", rec.Code, rec.Body.String())
	}
	// A Bearer key presented with an email-session cookie that is expired or
	// absent is still a key (no cookie method applies).
	req = jsonReq(http.MethodDelete, "/auth/api-keys/nonexistent", nil, "")
	req.Header.Set("Authorization", "Bearer "+secret)
	rec = httptest.NewRecorder()
	h.HandleRevokeAPIKey(rec, req)
	if rec.Code == http.StatusForbidden && strings.Contains(rec.Body.String(), "passkey") {
		t.Fatalf("Bearer key refused by the passkey gate")
	}
}

// --- purposes, verify-address: item 3 ---

func TestVerifyAddressFlowAndPurposeIsolation(t *testing.T) {
	h, m := emailEnv(t)
	u := addEmailUser(t, h, "u1", "a@example.com", false, true)
	addEmailUser(t, h, "u2", "b@example.com", true, true)
	pk := passkeySession(t, h, u)

	// begin (passkey session) mails a code and no link.
	req := withCookies(jsonReq(http.MethodPost, "/auth/email/verify-address/begin", nil, ""), pk)
	rec := httptest.NewRecorder()
	h.HandleVerifyAddressBegin(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("verify-address begin = %d %s", rec.Code, rec.Body.String())
	}
	var br emailBeginResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &br)
	var attemptCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == EmailAttemptCookieName {
			attemptCookie = c
		}
	}
	mail := m.last(t)
	if mail.Purpose != EmailAttemptPurposeVerifyAddress || mail.Link != "" || mail.To != "a@example.com" {
		t.Fatalf("mail = %+v", mail)
	}

	// A verify_address code cannot sign in.
	if rec := emailVerifyCode(h, br.AttemptID, mail.Code, attemptCookie); rec.Code != http.StatusBadRequest {
		t.Fatalf("verify_address code at /auth/email/verify = %d", rec.Code)
	}
	// (that failed attempt took one guess; the code still stands)
	finish := func(cs []*http.Cookie, id, code string, ck *http.Cookie) *httptest.ResponseRecorder {
		req := withCookies(jsonReq(http.MethodPost, "/auth/email/verify-address/finish", map[string]string{"attempt_id": id, "code": code}, ""), cs)
		if ck != nil {
			req.AddCookie(ck)
		}
		rec := httptest.NewRecorder()
		h.HandleVerifyAddressFinish(rec, req)
		return rec
	}
	// An email session cannot finish (passkey session required).
	other := addEmailUser(t, h, "u3", "c@example.com", true, true)
	_ = other
	emailCS := signInByEmail(t, h, m, "c@example.com")
	if rec := finish(emailCS, br.AttemptID, mail.Code, attemptCookie); rec.Code != http.StatusForbidden {
		t.Fatalf("finish with an email session = %d", rec.Code)
	}
	// Another user's passkey session cannot consume this attempt.
	if rec := finish(passkeySession(t, h, mustUser(t, h, "u2")), br.AttemptID, mail.Code, attemptCookie); rec.Code != http.StatusBadRequest {
		t.Fatalf("finish by a different user = %d", rec.Code)
	}
	if mustUser(t, h, "u1").EmailVerifiedAt != nil {
		t.Fatal("address verified by the wrong session")
	}
	// The owner finishes.
	if rec := finish(pk, br.AttemptID, mail.Code, attemptCookie); rec.Code != http.StatusOK {
		t.Fatalf("finish = %d %s", rec.Code, rec.Body.String())
	}
	if mustUser(t, h, "u1").EmailVerifiedAt == nil {
		t.Fatal("address not verified after finish")
	}

	// A sign_in code cannot finish a verify-address.
	h2, m2 := emailEnv(t)
	u2 := addEmailUser(t, h2, "u1", "a@example.com", true, true)
	b := emailBegin(t, h2, "a@example.com", "")
	req2 := withCookies(jsonReq(http.MethodPost, "/auth/email/verify-address/finish", map[string]string{"attempt_id": b.attemptID, "code": m2.last(t).Code}, ""), passkeySession(t, h2, u2))
	req2.AddCookie(b.cookie)
	rec2 := httptest.NewRecorder()
	h2.HandleVerifyAddressFinish(rec2, req2)
	if rec2.Code != http.StatusBadRequest {
		t.Fatalf("sign_in code at verify-address/finish = %d", rec2.Code)
	}
}

func TestVerifyAddressBeginRequiresPasskeySession(t *testing.T) {
	h, m := emailEnv(t)
	addEmailUser(t, h, "u1", "a@example.com", true, true)
	cs := signInByEmail(t, h, m, "a@example.com")
	rec := httptest.NewRecorder()
	h.HandleVerifyAddressBegin(rec, withCookies(jsonReq(http.MethodPost, "/auth/email/verify-address/begin", nil, ""), cs))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("email session verify-address/begin = %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.HandleVerifyAddressBegin(rec, jsonReq(http.MethodPost, "/auth/email/verify-address/begin", nil, ""))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous verify-address/begin = %d", rec.Code)
	}
}

func TestRecoveryVerifySetsEmailVerified(t *testing.T) {
	h, _ := recoveryMailTestEnv(t)
	addEmailUser(t, h, "u1", "owner@example.com", false, true)
	token, err := h.store.CreateRecoveryToken(context.Background(), "u1", "owner@example.com", hashEmail("owner@example.com"), hashIP("203.0.113.5"))
	if err != nil {
		t.Fatal(err)
	}
	postRecoveryVerify(t, h, token)
	if mustUser(t, h, "u1").EmailVerifiedAt == nil {
		t.Fatal("consumed recovery link must mark the address verified")
	}
}

// --- F23 and item 7: the switch ---

func TestF23SwitchOffRefusesVerifyAndDeletesAttemptsAndEmailSessions(t *testing.T) {
	h, m := emailEnv(t)
	addEmailUser(t, h, "u1", "a@example.com", true, true)
	emailCS := signInByEmail(t, h, m, "a@example.com")
	pkCS := passkeySession(t, h, mustUser(t, h, "u1"))
	b := emailBegin(t, h, "a@example.com", "")
	code := m.last(t).Code
	tok := tokenFromLink(t, m.last(t).Link)

	if err := h.SetEmailSignIn(false); err != nil {
		t.Fatal(err)
	}
	if rec := emailVerifyCode(h, b.attemptID, code, b.cookie); rec.Code != http.StatusForbidden {
		t.Fatalf("code verify with the switch off = %d", rec.Code)
	}
	if rec := emailVerifyToken(h, tok); rec.Code != http.StatusForbidden {
		t.Fatalf("link verify with the switch off = %d", rec.Code)
	}
	var n int
	_ = h.store.DB().QueryRow("SELECT COUNT(*) FROM email_signin_attempts WHERE consumed_at IS NULL").Scan(&n)
	if n != 0 {
		t.Fatalf("unconsumed attempts remain: %d", n)
	}
	// Even if the switch comes back on, the old code and link are gone.
	if err := h.SetEmailSignIn(true); err != nil {
		t.Fatal(err)
	}
	if rec := emailVerifyCode(h, b.attemptID, code, b.cookie); rec.Code != http.StatusBadRequest {
		t.Fatalf("old code after switch cycle = %d", rec.Code)
	}
	if rec := emailVerifyToken(h, tok); rec.Code != http.StatusBadRequest {
		t.Fatalf("old link after switch cycle = %d", rec.Code)
	}
	// The email session died with the switch; the passkey session did not.
	if _, err := h.store.GetRefreshSessionByTokenHash(sha256SumHex(cookieByName(emailCS, RefreshTokenCookieName).Value)); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("email refresh session survived the switch: %v", err)
	}
	if _, err := h.store.GetRefreshSessionByTokenHash(sha256SumHex(cookieByName(pkCS, RefreshTokenCookieName).Value)); err != nil {
		t.Fatalf("passkey session lost: %v", err)
	}
	// And the email session cannot refresh.
	req := httptest.NewRequest(http.MethodGet, "/auth/session", nil)
	req.AddCookie(cookieByName(emailCS, RefreshTokenCookieName))
	rec := httptest.NewRecorder()
	h.HandleSession(rec, req)
	if !strings.Contains(rec.Body.String(), `"authenticated":false`) {
		t.Fatalf("email session refreshed after the switch: %s", rec.Body.String())
	}
}

func TestSwitchOffBeginAnswersAsUnknownAndWritesNothing(t *testing.T) {
	h, m := emailEnv(t)
	addEmailUser(t, h, "u1", "a@example.com", true, true)
	if err := h.SetEmailSignIn(false); err != nil {
		t.Fatal(err)
	}
	b := emailBegin(t, h, "a@example.com", "")
	if b.code != http.StatusOK || b.attemptID == "" || b.cookie == nil || len(m.mails()) != 0 {
		t.Fatalf("begin with switch off: %+v mails=%d", b, len(m.mails()))
	}
	var n int
	_ = h.store.DB().QueryRow("SELECT COUNT(*) FROM email_signin_attempts").Scan(&n)
	if n != 0 {
		t.Fatalf("rows written with the switch off: %d", n)
	}
}

func TestApplyEmailSignInSwitchAtBootWithSwitchOff(t *testing.T) {
	h, m := emailEnv(t)
	addEmailUser(t, h, "u1", "a@example.com", true, true)
	signInByEmail(t, h, m, "a@example.com")
	emailBegin(t, h, "a@example.com", "")
	// A restart with AUTH_EMAIL_SIGN_IN unset: a fresh handler on the same store.
	h2 := NewHandler(h.store, h.webauthn, &Config{
		AccessTokenTTL: h.config.AccessTokenTTL, RefreshTokenTTL: h.config.RefreshTokenTTL, RPOrigins: h.config.RPOrigins,
	}, h.signer)
	if err := h2.ApplyEmailSignInSwitch(); err != nil {
		t.Fatal(err)
	}
	var attempts, sessions int
	_ = h.store.DB().QueryRow("SELECT COUNT(*) FROM email_signin_attempts WHERE consumed_at IS NULL").Scan(&attempts)
	_ = h.store.DB().QueryRow("SELECT COUNT(*) FROM refresh_sessions WHERE auth_method LIKE 'email%'").Scan(&sessions)
	if attempts != 0 || sessions != 0 {
		t.Fatalf("boot with the switch off left attempts=%d email sessions=%d", attempts, sessions)
	}
}

func TestSweeperDeletesOldRowsOnly(t *testing.T) {
	h, _ := emailEnv(t)
	emailBegin(t, h, "a@example.test", "")
	old := emailBegin(t, h, "b@example.test", "")
	if _, err := h.store.DB().Exec("UPDATE email_signin_attempts SET created_at = ? WHERE id = ?", time.Now().UTC().Add(-25*time.Hour), old.attemptID); err != nil {
		t.Fatal(err)
	}
	n, err := h.store.SweepEmailAttempts(context.Background(), time.Now().UTC().Add(-EmailAttemptRetention))
	if err != nil || n != 1 {
		t.Fatalf("swept %d err=%v", n, err)
	}
}

// --- migration: item 2 ---

func TestBackfillMarksOnlyUsersWithACredentialAndRunsOnce(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "old.db")
	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	// The pre-email-sign-in shape: users without email_verified_at.
	for _, stmt := range []string{
		`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT UNIQUE NOT NULL, created_at DATETIME NOT NULL)`,
		`CREATE TABLE credentials (id TEXT PRIMARY KEY, user_id TEXT NOT NULL, public_key BLOB NOT NULL, attestation_type TEXT NOT NULL, transport TEXT NOT NULL, sign_count INTEGER NOT NULL DEFAULT 0, aaguid BLOB NOT NULL, flags TEXT NOT NULL DEFAULT '{}', name TEXT NOT NULL DEFAULT '', prf_capable INTEGER NOT NULL DEFAULT 0, prf_salt BLOB, created_at DATETIME NOT NULL, last_used_at DATETIME)`,
		`INSERT INTO users VALUES ('with-key','k@example.com','2026-01-01 00:00:00')`,
		`INSERT INTO users VALUES ('no-key','n@example.com','2026-01-01 00:00:00')`,
		`INSERT INTO users VALUES ('choir-bootstrap-admin','bootstrap-admin@choir.local','2026-01-01 00:00:00')`,
		`INSERT INTO credentials (id,user_id,public_key,attestation_type,transport,aaguid,created_at) VALUES ('c1','with-key','pk','none','[]','a','2026-01-01 00:00:00')`,
	} {
		if _, err := raw.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	_ = raw.Close()

	s, err := OpenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	check := func(id string, want bool) {
		t.Helper()
		u, err := s.GetUserByID(id)
		if err != nil {
			t.Fatal(err)
		}
		if (u.EmailVerifiedAt != nil) != want {
			t.Fatalf("%s verified=%v, want %v", id, u.EmailVerifiedAt != nil, want)
		}
	}
	check("with-key", true)
	check("no-key", false)
	check("choir-bootstrap-admin", false)

	// A user created later stays unverified across reopen (backfill ran once).
	if _, err := s.CreateUser("late", "late@example.com"); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	s, err = OpenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	check("late", false)
	// Existing sessions read as passkey.
	if _, err := s.DB().Exec(`INSERT INTO refresh_sessions (id,user_id,token_hash,created_at,expires_at) VALUES ('r','with-key','h','2026-01-01 00:00:00','2027-01-01 00:00:00')`); err != nil {
		t.Fatal(err)
	}
	rs, err := s.GetRefreshSessionByID("r")
	if err != nil || rs.AuthMethod != AuthMethodPasskey {
		t.Fatalf("legacy row method = %q err=%v", rs.AuthMethod, err)
	}
}
