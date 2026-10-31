package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// F13: the native begin/verify put the attempt cookie and the session in the
// app's own cookie jar and nowhere else: the renderer's response carries no
// cookie, no token, no code.
func TestDesktopEmailSignInKeepsCookiesInTheNativeJar(t *testing.T) {
	const (
		attemptCookie = "attempt-cookie-secret"
		accessToken   = "email-access-secret"
		refreshToken  = "email-refresh-secret"
		code          = "12345678"
	)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" || !strings.HasPrefix(r.Header.Get("User-Agent"), "Choir-Desktop/") {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		switch r.URL.Path {
		case "/auth/email/begin":
			http.SetCookie(w, &http.Cookie{Name: "choir_signin_attempt", Value: attemptCookie, Path: "/auth/email", HttpOnly: true, SameSite: http.SameSiteStrictMode})
			_ = json.NewEncoder(w).Encode(map[string]string{"attempt_id": "attempt-1"})
		case "/auth/email/verify":
			var in struct {
				AttemptID string `json:"attempt_id"`
				Code      string `json:"code"`
			}
			_ = json.NewDecoder(r.Body).Decode(&in)
			c, err := r.Cookie("choir_signin_attempt")
			if err != nil || c.Value != attemptCookie || in.AttemptID != "attempt-1" || in.Code != code {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid or expired code"})
				return
			}
			http.SetCookie(w, &http.Cookie{Name: testAccessCookieName, Value: accessToken, Path: "/", HttpOnly: true})
			http.SetCookie(w, &http.Cookie{Name: testRefreshCookieName, Value: refreshToken, Path: "/auth", HttpOnly: true})
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "user": map[string]string{"email": "person@example.com"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer backend.Close()

	session := mustDesktopSession(t, backend.URL)
	handler := assetHandler(session)

	post := func(path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	// Wrong code first: the jar already holds the attempt cookie from begin.
	rec := post(desktopEmailBeginPath, `{"email":"person@example.com"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "attempt-1") {
		t.Fatalf("begin = %d %s", rec.Code, rec.Body.String())
	}
	if len(rec.Result().Cookies()) != 0 || rec.Header().Get("Set-Cookie") != "" || strings.Contains(rec.Body.String(), attemptCookie) {
		t.Fatalf("begin leaked a cookie to the renderer: %v %s", rec.Header(), rec.Body.String())
	}
	if rec := post(desktopEmailVerifyPath, `{"attempt_id":"attempt-1","code":"00000000"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("wrong code = %d", rec.Code)
	}

	rec = post(desktopEmailVerifyPath, `{"attempt_id":"attempt-1","code":"`+code+`"}`)
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"authenticated":true}` {
		t.Fatalf("verify = %d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Set-Cookie") != "" {
		t.Fatalf("verify leaked Set-Cookie to the renderer: %v", rec.Header())
	}
	for _, secret := range []string{accessToken, refreshToken, attemptCookie, code} {
		if strings.Contains(rec.Body.String(), secret) {
			t.Fatalf("verify response carries %q", secret)
		}
	}

	// The session is in the native jar.
	assertCookieValues(t, session.jar.Cookies(session.endpoint("/auth/")), map[string]string{
		testAccessCookieName:  accessToken,
		testRefreshCookieName: refreshToken,
	})
}

func TestDesktopEmailSignInRejectsMalformedRequests(t *testing.T) {
	session := mustDesktopSession(t, "https://choir.news")
	handler := assetHandler(session)
	for name, tc := range map[string]struct{ path, body string }{
		"empty email":        {desktopEmailBeginPath, `{"email":""}`},
		"unknown field body": {desktopEmailBeginPath, `{"email":"a@example.com"} {}`},
		"missing code":       {desktopEmailVerifyPath, `{"attempt_id":"x"}`},
		"not json":           {desktopEmailVerifyPath, `nope`},
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
		})
	}
	req := httptest.NewRequest(http.MethodGet, desktopEmailBeginPath, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET = %d, want 405", rec.Code)
	}
}
