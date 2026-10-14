package maild

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/server"
)

const testRecoveryLink = "https://choir.news/recover#token=choir_rt_AbCdEfGhIjKlMnOpQrStUvWxYz0123456789-_abcde"

type fakeResend struct {
	mu       sync.Mutex
	payloads []resendSendRequest
	status   int
}

func (f *fakeResend) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.payloads)
}

func newAuthMailTestHandler(t *testing.T) (*Handler, *fakeResend) {
	t.Helper()
	store, cfg := newTestStore(t)
	cfg.ResendAPIKey = "re_test"
	fake := &fakeResend{status: http.StatusOK}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload resendSendRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		fake.mu.Lock()
		fake.payloads = append(fake.payloads, payload)
		status := fake.status
		fake.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status == http.StatusOK {
			_, _ = w.Write([]byte(`{"id":"recovery-mail-1"}`))
			return
		}
		_, _ = w.Write([]byte(`{"message":"provider down"}`))
	}))
	t.Cleanup(srv.Close)
	cfg.ResendBaseURL = srv.URL
	h := NewHandler(cfg, store)
	h.resend = newResendClient(cfg, srv.Client())
	return h, fake
}

func authRecoveryRequest(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, AuthRecoveryLinkPath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")
	return req
}

func recoveryBody(to, link string, minutes int) string {
	b, _ := json.Marshal(map[string]any{"to_address": to, "link": link, "expires_minutes": minutes})
	return string(b)
}

func TestAuthRecoveryLinkSendsOneFixedTemplateMail(t *testing.T) {
	h, fake := newAuthMailTestHandler(t)
	w := httptest.NewRecorder()
	h.HandleAuthRecoveryLink(w, authRecoveryRequest(recoveryBody("owner@example.com", testRecoveryLink, 15)))
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", w.Code, w.Body.String())
	}
	if fake.count() != 1 {
		t.Fatalf("resend sends = %d, want 1", fake.count())
	}
	p := fake.payloads[0]
	if len(p.To) != 1 || p.To[0] != "owner@example.com" {
		t.Fatalf("to = %v", p.To)
	}
	if p.From != "Choir <updates@choir.news>" || p.Subject != "Recover your Choir account" {
		t.Fatalf("from/subject = %q / %q", p.From, p.Subject)
	}
	if !strings.Contains(p.Text, testRecoveryLink) || !strings.Contains(p.Text, "expires in 15 minutes") {
		t.Fatalf("text body missing link or expiry: %q", p.Text)
	}
	if !strings.Contains(p.HTML, `href="`+testRecoveryLink+`"`) {
		t.Fatalf("html body missing link: %q", p.HTML)
	}
	if p.Headers["X-Choir-Maild"] != "v0-auth-recovery-link" {
		t.Fatalf("headers = %v", p.Headers)
	}
	if strings.Contains(w.Body.String(), "choir_rt_") {
		t.Fatalf("response leaked token: %s", w.Body.String())
	}
}

func TestAuthRecoveryLinkRefusesNonHostCallers(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*http.Request)
	}{
		{"guest tap caller", func(r *http.Request) { r.RemoteAddr = "10.200.0.7:40000" }},
		{"missing internal caller", func(r *http.Request) { r.Header.Del("X-Internal-Caller") }},
		{"proxy-forwarded user request", func(r *http.Request) { r.Header.Set("X-Authenticated-User", "user-1") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, fake := newAuthMailTestHandler(t)
			req := authRecoveryRequest(recoveryBody("owner@example.com", testRecoveryLink, 15))
			tc.mutate(req)
			w := httptest.NewRecorder()
			h.HandleAuthRecoveryLink(w, req)
			if w.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", w.Code)
			}
			if fake.count() != 0 {
				t.Fatalf("resend sends = %d, want 0", fake.count())
			}
		})
	}
}

func TestAuthRecoveryLinkRefusesForeignOrMalformedLinks(t *testing.T) {
	const tok = "choir_rt_AbCdEfGhIjKlMnOpQrStUvWxYz0123456789"
	cases := map[string]string{
		"foreign host":        "https://evil.example/recover#token=" + tok,
		"lookalike subdomain": "https://choir.news.evil.example/recover#token=" + tok,
		"plain http primary":  "http://choir.news/recover#token=" + tok,
		"wrong path":          "https://choir.news/login#token=" + tok,
		"query string":        "https://choir.news/recover?next=https://evil.example#token=" + tok,
		"userinfo":            "https://choir.news@evil.example/recover#token=" + tok,
		"no token":            "https://choir.news/recover",
		"wrong token prefix":  "https://choir.news/recover#token=abc_AbCdEfGhIjKlMnOpQrSt",
		"token with markup":   "https://choir.news/recover#token=" + tok + "<script>",
		"javascript scheme":   "javascript:alert(1)//choir.news/recover#token=" + tok,
	}
	for name, link := range cases {
		t.Run(name, func(t *testing.T) {
			h, fake := newAuthMailTestHandler(t)
			w := httptest.NewRecorder()
			h.HandleAuthRecoveryLink(w, authRecoveryRequest(recoveryBody("owner@example.com", link, 15)))
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
			}
			if fake.count() != 0 {
				t.Fatalf("resend sends = %d, want 0", fake.count())
			}
			if strings.Contains(w.Body.String(), "choir_rt_") {
				t.Fatalf("response echoed the link: %s", w.Body.String())
			}
		})
	}
}

func TestAuthRecoveryLinkAllowsLocalhostForDevelopment(t *testing.T) {
	h, fake := newAuthMailTestHandler(t)
	w := httptest.NewRecorder()
	link := "http://localhost:4173/recover#token=choir_rt_AbCdEfGhIjKlMnOpQrStUvWxYz0123456789"
	h.HandleAuthRecoveryLink(w, authRecoveryRequest(recoveryBody("owner@example.com", link, 15)))
	if w.Code != http.StatusAccepted || fake.count() != 1 {
		t.Fatalf("status = %d sends = %d", w.Code, fake.count())
	}
}

func TestAuthRecoveryLinkRejectsBadRecipientAndExpiry(t *testing.T) {
	cases := map[string]string{
		"empty recipient":    recoveryBody("", testRecoveryLink, 15),
		"header injection":   recoveryBody("owner@example.com\r\nBcc: x@evil.example", testRecoveryLink, 15),
		"zero expiry":        recoveryBody("owner@example.com", testRecoveryLink, 0),
		"too long expiry":    recoveryBody("owner@example.com", testRecoveryLink, 600),
		"unknown body field": `{"to_address":"owner@example.com","link":"` + testRecoveryLink + `","expires_minutes":15,"html":"<b>x</b>"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			h, fake := newAuthMailTestHandler(t)
			w := httptest.NewRecorder()
			h.HandleAuthRecoveryLink(w, authRecoveryRequest(body))
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", w.Code)
			}
			if fake.count() != 0 {
				t.Fatalf("resend sends = %d, want 0", fake.count())
			}
		})
	}
}

func TestAuthRecoveryLinkProviderFailureIsReportedWithoutToken(t *testing.T) {
	h, fake := newAuthMailTestHandler(t)
	fake.status = http.StatusInternalServerError
	var logs bytes.Buffer
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	w := httptest.NewRecorder()
	h.HandleAuthRecoveryLink(w, authRecoveryRequest(recoveryBody("owner@example.com", testRecoveryLink, 15)))
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", w.Code)
	}
	if strings.Contains(logs.String(), "choir_rt_") || strings.Contains(w.Body.String(), "choir_rt_") {
		t.Fatalf("token leaked: logs=%q body=%q", logs.String(), w.Body.String())
	}
	if strings.Contains(logs.String(), "owner@example.com") {
		t.Fatalf("recipient address logged in clear: %q", logs.String())
	}
}

func TestAuthRecoveryLinkRouteIsRegistered(t *testing.T) {
	h, fake := newAuthMailTestHandler(t)
	s := server.NewServer("maild", "0")
	RegisterRoutes(s, h)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, authRecoveryRequest(recoveryBody("owner@example.com", testRecoveryLink, 15)))
	if w.Code != http.StatusAccepted || fake.count() != 1 {
		t.Fatalf("status = %d sends = %d", w.Code, fake.count())
	}
}
