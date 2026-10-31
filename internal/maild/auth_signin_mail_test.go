package maild

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/server"
)

const (
	testSignInToken = "choir_st_AbCdEfGhIjKlMnOpQrStUvWxYz0123456789-_abcde"
	testSignInLink  = "https://choir.news/signin/email#token=" + testSignInToken
	signInGateToken = "maild-signin-gate-token-0123456789abcdef012345"
)

func signInBody(to, purpose, code, link string) string {
	b, _ := json.Marshal(map[string]any{
		"to_address": to, "purpose": purpose, "code": code, "link": link,
		"device": "Safari on macOS", "place": "network 203.0.113.x", "expires_minutes": 10,
	})
	return string(b)
}

func signInRequest(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, AuthSignInCodePath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")
	req.RemoteAddr = "127.0.0.1:8000"
	req.Header.Set(hostauth.Header, signInGateToken)
	return req
}

// serve runs the handler behind hostauth.Middleware in enforce mode.
func serveSignIn(t *testing.T, h *Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	hostauth.Middleware(http.HandlerFunc(h.HandleAuthSignInCode)).ServeHTTP(w, req)
	return w
}

func TestAuthSignInMailSendsFixedTemplate(t *testing.T) {
	defer hostauth.SetForTesting(hostauth.ModeEnforce, signInGateToken)()
	h, fake := newAuthMailTestHandler(t)
	w := serveSignIn(t, h, signInRequest(signInBody("owner@example.com", "sign_in", "01234567", testSignInLink)))
	if w.Code != http.StatusAccepted || fake.count() != 1 {
		t.Fatalf("status=%d sends=%d body=%s", w.Code, fake.count(), w.Body.String())
	}
	p := fake.payloads[0]
	for _, want := range []string{"01234567", testSignInLink, "Safari on macOS", "network 203.0.113.x", "Never give this code to anyone"} {
		if !strings.Contains(p.Text, want) {
			t.Errorf("text missing %q: %q", want, p.Text)
		}
	}
	if p.Subject != "Your Choir sign-in code" || p.Headers["X-Choir-Maild"] != "v0-auth-signin-code" {
		t.Fatalf("subject/headers = %q %v", p.Subject, p.Headers)
	}
	if strings.Contains(w.Body.String(), "choir_st_") {
		t.Fatalf("response leaked token: %s", w.Body.String())
	}
}

func TestAuthSignInMailVerifyAddressHasNoLink(t *testing.T) {
	defer hostauth.SetForTesting(hostauth.ModeEnforce, signInGateToken)()
	h, fake := newAuthMailTestHandler(t)
	w := serveSignIn(t, h, signInRequest(signInBody("owner@example.com", "verify_address", "01234567", "")))
	if w.Code != http.StatusAccepted || fake.count() != 1 {
		t.Fatalf("status=%d sends=%d", w.Code, fake.count())
	}
	if strings.Contains(fake.payloads[0].Text, "http") {
		t.Fatalf("verify-address mail carries a link: %q", fake.payloads[0].Text)
	}
	w = serveSignIn(t, h, signInRequest(signInBody("owner@example.com", "verify_address", "01234567", testSignInLink)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("verify_address with a link = %d, want 400", w.Code)
	}
}

// F12: a guest (tap) caller, a missing marker, a proxied user request and a
// missing credential all get refused with nothing sent.
func TestAuthSignInMailRefusesNonHostCallers(t *testing.T) {
	defer hostauth.SetForTesting(hostauth.ModeEnforce, signInGateToken)()
	cases := map[string]func(*http.Request){
		"guest tap caller":        func(r *http.Request) { r.RemoteAddr = "10.200.0.7:40000" },
		"missing internal caller": func(r *http.Request) { r.Header.Del("X-Internal-Caller") },
		"proxy-forwarded user":    func(r *http.Request) { r.Header.Set("X-Authenticated-User", "user-1") },
		"missing host credential": func(r *http.Request) { r.Header.Del(hostauth.Header) },
		"wrong host credential":   func(r *http.Request) { r.Header.Set(hostauth.Header, "wrong-credential-0123456789abcdef0123456789") },
		"non-POST":                func(r *http.Request) { r.Method = http.MethodGet },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			h, fake := newAuthMailTestHandler(t)
			req := signInRequest(signInBody("owner@example.com", "sign_in", "01234567", testSignInLink))
			mutate(req)
			w := serveSignIn(t, h, req)
			if w.Code != http.StatusForbidden && w.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want refusal", w.Code)
			}
			if fake.count() != 0 {
				t.Fatalf("sends = %d, want 0", fake.count())
			}
		})
	}
}

// F24: maild refuses the sign-in route when hostauth is not enforcing, even
// for a loopback caller with the marker (the recovery route still works).
func TestAuthSignInMailRefusesWhenHostauthNotEnforcing(t *testing.T) {
	for _, mode := range []hostauth.Mode{hostauth.ModeOff, hostauth.ModeObserve} {
		t.Run(string(mode), func(t *testing.T) {
			defer hostauth.SetForTesting(mode, signInGateToken)()
			h, fake := newAuthMailTestHandler(t)
			w := serveSignIn(t, h, signInRequest(signInBody("owner@example.com", "sign_in", "01234567", testSignInLink)))
			if w.Code != http.StatusForbidden || fake.count() != 0 {
				t.Fatalf("status=%d sends=%d, want 403 and 0", w.Code, fake.count())
			}
		})
	}
	// Enforce mode with no credential loaded is also not enforcing.
	defer hostauth.SetForTesting(hostauth.ModeEnforce, "")()
	h, fake := newAuthMailTestHandler(t)
	w := serveSignIn(t, h, signInRequest(signInBody("owner@example.com", "sign_in", "01234567", testSignInLink)))
	if w.Code != http.StatusForbidden || fake.count() != 0 {
		t.Fatalf("no-credential enforce: status=%d sends=%d", w.Code, fake.count())
	}
}

func TestAuthSignInMailLinkValidator(t *testing.T) {
	defer hostauth.SetForTesting(hostauth.ModeEnforce, signInGateToken)()
	const tok = testSignInToken
	cases := map[string]string{
		"foreign host":        "https://evil.example/signin/email#token=" + tok,
		"lookalike subdomain": "https://choir.news.evil.example/signin/email#token=" + tok,
		"plain http primary":  "http://choir.news/signin/email#token=" + tok,
		"wrong path":          "https://choir.news/recover#token=" + tok,
		"path suffix":         "https://choir.news/signin/email/x#token=" + tok,
		"query string":        "https://choir.news/signin/email?next=x#token=" + tok,
		"token in query":      "https://choir.news/signin/email?token=" + tok,
		"empty query":         "https://choir.news/signin/email?#token=" + tok,
		"userinfo":            "https://choir.news@evil.example/signin/email#token=" + tok,
		"no token":            "https://choir.news/signin/email",
		"recovery prefix":     "https://choir.news/signin/email#token=choir_rt_AbCdEfGhIjKlMnOpQrStUvWxYz0123456789",
		"token with markup":   "https://choir.news/signin/email#token=" + tok + "<script>",
		"extra fragment key":  "https://choir.news/signin/email#x=1&token=" + tok,
		"javascript scheme":   "javascript:alert(1)//choir.news/signin/email#token=" + tok,
	}
	for name, link := range cases {
		t.Run(name, func(t *testing.T) {
			h, fake := newAuthMailTestHandler(t)
			w := serveSignIn(t, h, signInRequest(signInBody("owner@example.com", "sign_in", "01234567", link)))
			if w.Code != http.StatusBadRequest || fake.count() != 0 {
				t.Fatalf("status=%d sends=%d, want 400 and 0", w.Code, fake.count())
			}
			if strings.Contains(w.Body.String(), "choir_st_") {
				t.Fatalf("response echoed the link: %s", w.Body.String())
			}
		})
	}
	h, fake := newAuthMailTestHandler(t)
	w := serveSignIn(t, h, signInRequest(signInBody("owner@example.com", "sign_in", "01234567", "http://localhost:4173/signin/email#token="+tok)))
	if w.Code != http.StatusAccepted || fake.count() != 1 {
		t.Fatalf("localhost: status=%d sends=%d", w.Code, fake.count())
	}
}

func TestAuthSignInMailRejectsBadFields(t *testing.T) {
	defer hostauth.SetForTesting(hostauth.ModeEnforce, signInGateToken)()
	cases := map[string]string{
		"short code":       signInBody("owner@example.com", "sign_in", "1234567", testSignInLink),
		"letter code":      signInBody("owner@example.com", "sign_in", "1234567a", testSignInLink),
		"bad purpose":      signInBody("owner@example.com", "recover", "01234567", testSignInLink),
		"header injection": signInBody("owner@example.com\r\nBcc: x@evil.example", "sign_in", "01234567", testSignInLink),
		"newline device":   `{"to_address":"owner@example.com","purpose":"sign_in","code":"01234567","link":"` + testSignInLink + `","device":"a\nb","place":"p","expires_minutes":10}`,
		"unknown field":    `{"to_address":"owner@example.com","purpose":"sign_in","code":"01234567","link":"` + testSignInLink + `","html":"x","expires_minutes":10}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			h, fake := newAuthMailTestHandler(t)
			w := serveSignIn(t, h, signInRequest(body))
			if w.Code != http.StatusBadRequest || fake.count() != 0 {
				t.Fatalf("status=%d sends=%d, want 400 and 0", w.Code, fake.count())
			}
		})
	}
}

func TestAuthSignInMailRouteIsRegistered(t *testing.T) {
	defer hostauth.SetForTesting(hostauth.ModeEnforce, signInGateToken)()
	h, fake := newAuthMailTestHandler(t)
	s := server.NewServer("maild", "0")
	RegisterRoutes(s, h)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, signInRequest(signInBody("owner@example.com", "sign_in", "01234567", testSignInLink)))
	if w.Code != http.StatusAccepted || fake.count() != 1 {
		t.Fatalf("status=%d sends=%d body=%s", w.Code, fake.count(), w.Body.String())
	}
}
