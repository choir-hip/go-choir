package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseDesktopBackendRequiresHTTPSOrLoopback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		backend string
		wantErr bool
	}{
		{name: "deployed HTTPS", backend: "https://choir.news"},
		{name: "loopback localhost", backend: "http://localhost:8080"},
		{name: "loopback IPv4", backend: "http://127.0.0.1:8080"},
		{name: "loopback IPv6", backend: "http://[::1]:8080"},
		{name: "remote HTTP", backend: "http://choir.news", wantErr: true},
		{name: "credentials", backend: "https://user:pass@choir.news", wantErr: true},
		{name: "backend path", backend: "https://choir.news/base", wantErr: true},
		{name: "backend query", backend: "https://choir.news?mode=desktop", wantErr: true},
		{name: "unsupported scheme", backend: "file:///tmp/choir", wantErr: true},
		{name: "relative", backend: "choir.news", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseDesktopBackend(tt.backend)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseDesktopBackend(%q) error = %v, wantErr %v", tt.backend, err, tt.wantErr)
			}
		})
	}
}

func TestDesktopAuthUsesOneBridgeAndKeepsTokensNative(t *testing.T) {
	const (
		email        = "person@example.com"
		exchangeCode = "one-time-secret-code"
		accessToken  = "native-access-secret"
		refreshToken = "native-refresh-secret"
	)

	var redeemCalls atomic.Int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/desktop/redeem" {
			t.Errorf("unexpected backend path %q", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		redeemCalls.Add(1)
		var request map[string]string
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode redemption request: %v", err)
		}
		if request["code"] != exchangeCode {
			t.Errorf("redemption code = %q, want %q", request["code"], exchangeCode)
		}
		writeJSON(w, http.StatusOK, desktopTokenResponse{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
		})
	}))
	defer backend.Close()

	session := mustDesktopSession(t, backend.URL)
	var bridgeCalls atomic.Int32
	session.openAuthSession = func(authURL, callbackScheme string) (string, error) {
		bridgeCalls.Add(1)
		parsed, err := url.Parse(authURL)
		if err != nil {
			t.Fatalf("parse bridge URL: %v", err)
		}
		if parsed.Path != "/desktop-bridge.html" {
			t.Errorf("opened path = %q, want desktop bridge", parsed.Path)
		}
		if parsed.Query().Get("email") != email {
			t.Errorf("bridge email = %q, want %q", parsed.Query().Get("email"), email)
		}
		if callbackScheme != "choir" {
			t.Errorf("callback scheme = %q, want choir", callbackScheme)
		}
		return "choir://auth-complete?code=" + exchangeCode, nil
	}

	request := httptest.NewRequest(http.MethodPost, "/desktop-auth/start-session", strings.NewReader(`{"email":"`+email+`"}`))
	response := httptest.NewRecorder()
	session.handleStart(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if bridgeCalls.Load() != 1 {
		t.Fatalf("bridge calls = %d, want exactly 1", bridgeCalls.Load())
	}
	if redeemCalls.Load() != 1 {
		t.Fatalf("redeem calls = %d, want 1", redeemCalls.Load())
	}
	for _, secret := range []string{exchangeCode, accessToken, refreshToken, "access_token", "refresh_token"} {
		if strings.Contains(response.Body.String(), secret) {
			t.Errorf("renderer response exposed %q: %s", secret, response.Body.String())
		}
	}
	var result map[string]bool
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode renderer response: %v", err)
	}
	if !result["authenticated"] {
		t.Fatalf("renderer response = %v, want authenticated", result)
	}

	assertCookieValues(t, session.jar.Cookies(session.endpoint("/api/trajectories")), map[string]string{
		accessCookieName: accessToken,
	})
	assertCookieValues(t, session.jar.Cookies(session.endpoint("/auth/session")), map[string]string{
		accessCookieName:  accessToken,
		refreshCookieName: refreshToken,
	})
}

func TestDesktopSessionSeedsHttpOnlyScopedCookies(t *testing.T) {
	t.Parallel()

	backend, err := parseDesktopBackend("https://choir.news")
	if err != nil {
		t.Fatal(err)
	}
	jar := &recordingCookieJar{}
	session := &desktopSession{backend: backend, jar: jar}
	if err := session.seedSession(desktopTokenResponse{AccessToken: "access", RefreshToken: "refresh"}); err != nil {
		t.Fatal(err)
	}
	if jar.setURL == nil || jar.setURL.String() != "https://choir.news/" {
		t.Fatalf("cookie URL = %v, want backend root", jar.setURL)
	}
	if len(jar.setCookies) != 2 {
		t.Fatalf("seeded cookies = %d, want 2", len(jar.setCookies))
	}

	byName := make(map[string]*http.Cookie, len(jar.setCookies))
	for _, cookie := range jar.setCookies {
		byName[cookie.Name] = cookie
	}
	access := byName[accessCookieName]
	refresh := byName[refreshCookieName]
	if access == nil || access.Path != "/" || !access.HttpOnly || !access.Secure || access.SameSite != http.SameSiteLaxMode {
		t.Errorf("access cookie attributes = %#v", access)
	}
	if refresh == nil || refresh.Path != "/auth" || !refresh.HttpOnly || !refresh.Secure || refresh.SameSite != http.SameSiteLaxMode {
		t.Errorf("refresh cookie attributes = %#v", refresh)
	}
}

func TestDesktopProxyOwnsCookiesAndAbsorbsSetCookie(t *testing.T) {
	const (
		accessToken  = "native-access"
		refreshToken = "native-refresh"
	)

	requestCookies := make(chan string, 1)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCookies <- r.Header.Get("Cookie")
		http.SetCookie(w, &http.Cookie{Name: accessCookieName, Value: "rotated-access", Path: "/", HttpOnly: true})
		http.SetCookie(w, &http.Cookie{Name: refreshCookieName, Value: "rotated-refresh", Path: "/auth", HttpOnly: true})
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}))
	defer backend.Close()

	session := mustDesktopSession(t, backend.URL)
	if err := session.seedSession(desktopTokenResponse{AccessToken: accessToken, RefreshToken: refreshToken}); err != nil {
		t.Fatal(err)
	}
	handler := assetHandler(session)
	request := httptest.NewRequest(http.MethodGet, "/api/trajectories", nil)
	request.Header.Set("Cookie", "renderer_cookie=attacker; "+accessCookieName+"=renderer-access")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	upstreamCookies := <-requestCookies
	if strings.Contains(upstreamCookies, "attacker") || strings.Contains(upstreamCookies, "renderer-access") || strings.Contains(upstreamCookies, "renderer_cookie") {
		t.Fatalf("proxy forwarded renderer cookies: %q", upstreamCookies)
	}
	if !strings.Contains(upstreamCookies, accessCookieName+"="+accessToken) {
		t.Fatalf("proxy did not inject native access cookie: %q", upstreamCookies)
	}
	if strings.Contains(upstreamCookies, refreshCookieName+"=") {
		t.Fatalf("proxy sent /auth-scoped refresh cookie to /api: %q", upstreamCookies)
	}
	if got := response.Header().Values("Set-Cookie"); len(got) != 0 {
		t.Fatalf("renderer received Set-Cookie: %v", got)
	}

	assertCookieValues(t, session.jar.Cookies(session.endpoint("/api/trajectories")), map[string]string{
		accessCookieName: "rotated-access",
	})
	assertCookieValues(t, session.jar.Cookies(session.endpoint("/auth/session")), map[string]string{
		accessCookieName:  "rotated-access",
		refreshCookieName: "rotated-refresh",
	})
}

func TestDesktopProxyLogoutClearsNativeJar(t *testing.T) {
	requestCookies := make(chan string, 1)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/logout" {
			t.Errorf("logout path = %q", r.URL.Path)
		}
		requestCookies <- r.Header.Get("Cookie")
		http.SetCookie(w, &http.Cookie{Name: accessCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
		http.SetCookie(w, &http.Cookie{Name: refreshCookieName, Value: "", Path: "/auth", MaxAge: -1, HttpOnly: true})
		w.WriteHeader(http.StatusNoContent)
	}))
	defer backend.Close()

	session := mustDesktopSession(t, backend.URL)
	if err := session.seedSession(desktopTokenResponse{AccessToken: "native-access", RefreshToken: "native-refresh"}); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	request.Header.Set("Cookie", accessCookieName+"=renderer-access; "+refreshCookieName+"=renderer-refresh")
	response := httptest.NewRecorder()
	assetHandler(session).ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want 204", response.Code)
	}
	upstreamCookies := <-requestCookies
	if !strings.Contains(upstreamCookies, accessCookieName+"=native-access") || !strings.Contains(upstreamCookies, refreshCookieName+"=native-refresh") {
		t.Fatalf("logout did not use native session cookies: %q", upstreamCookies)
	}
	if strings.Contains(upstreamCookies, "renderer-access") || strings.Contains(upstreamCookies, "renderer-refresh") {
		t.Fatalf("logout forwarded renderer cookies: %q", upstreamCookies)
	}
	if got := response.Header().Values("Set-Cookie"); len(got) != 0 {
		t.Fatalf("renderer received logout Set-Cookie: %v", got)
	}
	if cookies := session.jar.Cookies(session.endpoint("/auth/session")); len(cookies) != 0 {
		t.Fatalf("native jar retained cookies after logout: %v", cookies)
	}
}

func TestDesktopProxyRefreshRotatesNativeJar(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/refresh" {
			t.Errorf("refresh path = %q", r.URL.Path)
		}
		cookies := r.Header.Get("Cookie")
		if !strings.Contains(cookies, accessCookieName+"=old-access") || !strings.Contains(cookies, refreshCookieName+"=old-refresh") {
			t.Errorf("refresh request cookies = %q", cookies)
		}
		http.SetCookie(w, &http.Cookie{Name: accessCookieName, Value: "new-access", Path: "/", HttpOnly: true})
		http.SetCookie(w, &http.Cookie{Name: refreshCookieName, Value: "new-refresh", Path: "/auth", HttpOnly: true})
		w.WriteHeader(http.StatusNoContent)
	}))
	defer backend.Close()

	session := mustDesktopSession(t, backend.URL)
	if err := session.seedSession(desktopTokenResponse{AccessToken: "old-access", RefreshToken: "old-refresh"}); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	response := httptest.NewRecorder()
	assetHandler(session).ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("refresh status = %d, want 204", response.Code)
	}
	assertCookieValues(t, session.jar.Cookies(session.endpoint("/auth/session")), map[string]string{
		accessCookieName:  "new-access",
		refreshCookieName: "new-refresh",
	})
}

func TestDesktopProxyBlocksRendererExchangeRoutes(t *testing.T) {
	t.Parallel()

	var upstreamCalls atomic.Int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamCalls.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer backend.Close()

	handler := assetHandler(mustDesktopSession(t, backend.URL))
	for _, path := range []string{
		"/auth/desktop/exchange",
		"/auth/desktop/exchange/",
		"/auth/desktop/exchange-redirect",
		"/auth/desktop/exchange-redirect/extra",
		"/auth/desktop/redeem",
		"/auth/desktop/redeem/extra",
	} {
		request := httptest.NewRequest(http.MethodPost, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Errorf("%s status = %d, want 404", path, response.Code)
		}
	}
	if upstreamCalls.Load() != 0 {
		t.Fatalf("blocked routes reached backend %d times", upstreamCalls.Load())
	}
}

func TestDesktopAuthErrorsAreBoundedAndDoNotLeak(t *testing.T) {
	const sensitive = "person@example.com?code=secret-token"

	session := mustDesktopSession(t, "http://127.0.0.1:1")
	session.openAuthSession = func(_, _ string) (string, error) {
		return "", errors.New(sensitive)
	}

	var logs bytes.Buffer
	previousOutput := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previousOutput) })

	request := httptest.NewRequest(http.MethodPost, "/desktop-auth/start-session", strings.NewReader(`{"email":"person@example.com"}`))
	response := httptest.NewRecorder()
	session.handleStart(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("auth error status = %d, want 401", response.Code)
	}
	if strings.Contains(response.Body.String(), sensitive) || strings.Contains(logs.String(), sensitive) || strings.Contains(logs.String(), "person@example.com") {
		t.Fatalf("sensitive auth detail leaked: response=%q logs=%q", response.Body.String(), logs.String())
	}

	oversized := httptest.NewRequest(http.MethodPost, "/desktop-auth/start-session", strings.NewReader(`{"email":"`+strings.Repeat("a", desktopAuthRequestLimit)+`"}`))
	oversizedResponse := httptest.NewRecorder()
	session.handleStart(oversizedResponse, oversized)
	if oversizedResponse.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized status = %d, want 413", oversizedResponse.Code)
	}
}

func TestDesktopAuthRejectsConcurrentNativeSession(t *testing.T) {
	session := mustDesktopSession(t, "http://127.0.0.1:1")
	entered := make(chan struct{})
	release := make(chan struct{})
	session.openAuthSession = func(_, _ string) (string, error) {
		close(entered)
		<-release
		return "", errors.New("cancelled")
	}

	firstDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		request := httptest.NewRequest(http.MethodPost, "/desktop-auth/start-session", strings.NewReader(`{"email":"first@example.com"}`))
		response := httptest.NewRecorder()
		session.handleStart(response, request)
		firstDone <- response
	}()
	<-entered

	secondRequest := httptest.NewRequest(http.MethodPost, "/desktop-auth/start-session", strings.NewReader(`{"email":"second@example.com"}`))
	secondResponse := httptest.NewRecorder()
	session.handleStart(secondResponse, secondRequest)
	if secondResponse.Code != http.StatusConflict {
		t.Fatalf("concurrent auth status = %d, want 409", secondResponse.Code)
	}

	close(release)
	if firstResponse := <-firstDone; firstResponse.Code != http.StatusUnauthorized {
		t.Fatalf("first auth status = %d, want 401", firstResponse.Code)
	}
}

func TestDesktopAuthRedemptionUsesNetworkTimeout(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		writeJSON(w, http.StatusOK, desktopTokenResponse{AccessToken: "access", RefreshToken: "refresh"})
	}))
	defer backend.Close()

	session := mustDesktopSession(t, backend.URL)
	session.client.Timeout = 10 * time.Millisecond
	session.openAuthSession = func(_, _ string) (string, error) {
		return "choir://auth-complete?code=code", nil
	}
	request := httptest.NewRequest(http.MethodPost, "/desktop-auth/start-session", strings.NewReader(`{"email":"person@example.com"}`))
	response := httptest.NewRecorder()
	started := time.Now()
	session.handleStart(response, request)
	if response.Code != http.StatusBadGateway {
		t.Fatalf("timeout status = %d, want 502", response.Code)
	}
	if elapsed := time.Since(started); elapsed > 80*time.Millisecond {
		t.Fatalf("redemption timeout took %s", elapsed)
	}
}

func TestDesktopExchangeCodeValidatesCallbackAuthority(t *testing.T) {
	t.Parallel()

	if code, err := desktopExchangeCode("choir://auth-complete?code=valid"); err != nil || code != "valid" {
		t.Fatalf("valid callback = %q, %v", code, err)
	}
	for _, callback := range []string{
		"https://auth-complete?code=valid",
		"choir://attacker?code=valid",
		"choir://auth-complete",
		":not-a-url",
	} {
		if _, err := desktopExchangeCode(callback); err == nil {
			t.Errorf("desktopExchangeCode(%q) succeeded", callback)
		}
	}
}

func TestDesktopRendererSourceCannotHandleSessionTokens(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("../../frontend/src/lib/auth.js")
	if err != nil {
		t.Fatalf("read renderer auth source: %v", err)
	}
	for _, forbidden := range []string{"document.cookie", "access_token", "refresh_token", "choir_access", "choir_refresh"} {
		if strings.Contains(string(source), forbidden) {
			t.Errorf("renderer auth source contains forbidden session authority %q", forbidden)
		}
	}
}

func TestDesktopDoesNotRegisterSupersededBaseSyncService(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read desktop main source: %v", err)
	}
	for _, forbidden := range []string{
		"application.NewService(newSyncService",
		"func newSyncService(",
	} {
		if strings.Contains(string(source), forbidden) {
			t.Errorf("desktop still exposes superseded Base sync authority %q", forbidden)
		}
	}
}

func mustDesktopSession(t *testing.T, backend string) *desktopSession {
	t.Helper()
	session, err := newDesktopSession(backend)
	if err != nil {
		t.Fatalf("newDesktopSession(%q): %v", backend, err)
	}
	return session
}

func assertCookieValues(t *testing.T, cookies []*http.Cookie, want map[string]string) {
	t.Helper()
	got := make(map[string]string, len(cookies))
	for _, cookie := range cookies {
		got[cookie.Name] = cookie.Value
	}
	if len(got) != len(want) {
		t.Fatalf("cookies = %v, want %v", got, want)
	}
	for name, value := range want {
		if got[name] != value {
			t.Errorf("cookie %s = %q, want %q", name, got[name], value)
		}
	}
}

type recordingCookieJar struct {
	setURL     *url.URL
	setCookies []*http.Cookie
}

func (j *recordingCookieJar) SetCookies(target *url.URL, cookies []*http.Cookie) {
	copyURL := *target
	j.setURL = &copyURL
	j.setCookies = make([]*http.Cookie, 0, len(cookies))
	for _, cookie := range cookies {
		copyCookie := *cookie
		j.setCookies = append(j.setCookies, &copyCookie)
	}
}

func (j *recordingCookieJar) Cookies(*url.URL) []*http.Cookie {
	return nil
}
