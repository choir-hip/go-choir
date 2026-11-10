package main

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/server"
)

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(prevOut); log.SetFlags(prevFlags) })
	return &buf
}

func accessLines(buf *bytes.Buffer) []string {
	var out []string
	for _, l := range strings.Split(buf.String(), "\n") {
		if strings.Contains(l, "operation=http_access") {
			out = append(out, l)
		}
	}
	return out
}

func statusHandler(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }
}

func TestAccessLogLineForEachStatus(t *testing.T) {
	buf := captureLog(t)
	h := newRateTestHandler(t)
	rl := auth.NewRateLimitersWithLimits(smallLimits(1))
	cases := []struct {
		pattern string
		class   auth.RateClass
		code    int
		addr    string
		want    []string
	}{
		{"GET /auth/session", auth.RateClassRead, 200, "203.0.113.1:1", []string{`route="/auth/session"`, "status=200", "class=read", "count=1", "limit=1", "addr=203.0.x.x", "outcome=ok", "user=-", "sid=-", "method=GET"}},
		{"POST /auth/logout", auth.RateClassAct, 401, "203.0.113.2:1", []string{`route="/auth/logout"`, "status=401", "class=act", "outcome=denied"}},
		{"POST /auth/login/begin", auth.RateClassCeremony, 500, "203.0.113.3:1", []string{`route="/auth/login/begin"`, "status=500", "outcome=server_error"}},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.RemoteAddr = c.addr
		h.Instrument(rl, c.pattern, c.class, statusHandler(c.code))(httptest.NewRecorder(), req)
	}
	// 429: the second request from one address in a limit-1 class.
	var rec *httptest.ResponseRecorder
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.RemoteAddr = "203.0.113.4:1"
		rec = httptest.NewRecorder()
		h.Instrument(rl, "GET /auth/accounts", auth.RateClassRead, statusHandler(200))(rec, req)
	}
	if rec.Code != 429 {
		t.Fatalf("want 429, got %d", rec.Code)
	}

	t.Log("\n" + buf.String())
	lines := accessLines(buf)
	if len(lines) != 5 {
		t.Fatalf("want 5 access lines, got %d:\n%s", len(lines), buf.String())
	}
	for i, c := range cases {
		for _, w := range c.want {
			if !strings.Contains(lines[i], w) {
				t.Errorf("line %d missing %q: %s", i, w, lines[i])
			}
		}
		if !strings.Contains(lines[i], " ms=") {
			t.Errorf("line %d has no duration: %s", i, lines[i])
		}
	}
	for _, w := range []string{`route="/auth/accounts"`, "status=429", "count=2", "limit=1", "outcome=rate_limited"} {
		if !strings.Contains(lines[4], w) {
			t.Errorf("429 line missing %q: %s", w, lines[4])
		}
	}
}

func TestAccessLogPanicStillWritesALine(t *testing.T) {
	buf := captureLog(t)
	h := newRateTestHandler(t)
	func() {
		defer func() { _ = recover() }()
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.RemoteAddr = "203.0.113.5:1"
		h.Instrument(auth.NewRateLimiters(), "GET /auth/session", auth.RateClassRead, func(http.ResponseWriter, *http.Request) { panic("boom") })(httptest.NewRecorder(), req)
	}()
	if lines := accessLines(buf); len(lines) != 1 || !strings.Contains(lines[0], "status=500") {
		t.Fatalf("panic line: %v", lines)
	}
}

func TestAccessLogNeverCarriesRequestSecrets(t *testing.T) {
	buf := captureLog(t)
	h := newRateTestHandler(t)
	secrets := []string{
		"someone@example.com", "someone%40example.com", "198.51.100.77", "ACCESS-TOKEN-SECRET", "REFRESH-TOKEN-SECRET",
		"ATTEMPT-COOKIE-SECRET", "12345678-CODE", "choir_rt_TOKENSECRET0123456789", "BEARER-SECRET", "Mozilla/5.0 SecretAgent", "CHALLENGE-SECRET", "secret-query-value",
	}
	req := httptest.NewRequest(http.MethodPost, "/auth/email/verify?email=someone%40example.com&code=12345678-CODE&token=choir_rt_TOKENSECRET0123456789&x=secret-query-value", strings.NewReader(`{"email":"someone@example.com","challenge":"CHALLENGE-SECRET"}`))
	req.RemoteAddr = "10.0.0.1:1"
	req.Header.Set("X-Forwarded-For", "198.51.100.77")
	req.Header.Set("User-Agent", "Mozilla/5.0 SecretAgent")
	req.Header.Set("Authorization", "Bearer BEARER-SECRET")
	req.AddCookie(&http.Cookie{Name: auth.AccessTokenCookieName, Value: "ACCESS-TOKEN-SECRET"})
	req.AddCookie(&http.Cookie{Name: auth.RefreshTokenCookieName, Value: "REFRESH-TOKEN-SECRET"})
	req.AddCookie(&http.Cookie{Name: "choir_attempt", Value: "ATTEMPT-COOKIE-SECRET"})
	rec := httptest.NewRecorder()
	h.Instrument(auth.NewRateLimiters(), "POST /auth/email/verify", auth.RateClassSensitive, statusHandler(401))(rec, req)

	lines := accessLines(buf)
	if len(lines) != 1 {
		t.Fatalf("want one line, got %v", lines)
	}
	for _, s := range secrets {
		if strings.Contains(buf.String(), s) {
			t.Errorf("log output contains %q:\n%s", s, buf.String())
		}
	}
	if !strings.Contains(lines[0], "addr=198.51.x.x") {
		t.Errorf("address not masked to a prefix: %s", lines[0])
	}
	if strings.Contains(lines[0], "?") {
		t.Errorf("line carries a query string: %s", lines[0])
	}
}

// Every route is registered through Instrument: serve each registered route
// once and find its access line.
func TestEveryAuthRouteWritesAnAccessLine(t *testing.T) {
	buf := captureLog(t)
	h := newRateTestHandler(t)
	s := server.NewServer("auth", "0")
	registerRoutes(s, h, auth.NewRateLimiters())
	for i, rt := range authRoutes(h) {
		method, path := http.MethodGet, rt.pattern
		if m, p, ok := strings.Cut(rt.pattern, " "); ok {
			method, path = m, p
		}
		path = strings.ReplaceAll(path, "{id}", "x")
		before := len(accessLines(buf))
		req := httptest.NewRequest(method, path, nil)
		req.RemoteAddr = "203.0.113." + string(rune('0'+i/100)) + string(rune('0'+i/10%10)) + string(rune('0'+i%10)) + ":1"
		s.ServeHTTP(httptest.NewRecorder(), req)
		lines := accessLines(buf)
		if len(lines) != before+1 {
			t.Errorf("route %q wrote %d access lines, want 1", rt.pattern, len(lines)-before)
			continue
		}
		route := rt.pattern
		if _, p, ok := strings.Cut(rt.pattern, " "); ok {
			route = p
		}
		if !strings.Contains(lines[len(lines)-1], `route="`+route+`"`) {
			t.Errorf("route %q: line has the wrong route: %s", rt.pattern, lines[len(lines)-1])
		}
	}
}
