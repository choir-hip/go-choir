package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/auth"
)

// Autopaper daily digest (docs/problems/autopaper-daily-digest-phase1-2026-
// 10-20.md, M12). Subscribing, unsubscribing and send-now are owner acts that
// cause mail, so a headless API key (which an agent inside the computer may
// hold) never reaches them; the owner's browser session does. Reading the
// digest status keeps API-key access. Every spelling maild could treat as the
// same route is caught (empty path segments, trailing slash).
func TestEmailDigestWriteRoutesRefuseAPIKeyButForwardOwnerSession(t *testing.T) {
	var maildPaths []string
	maild := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		maildPaths = append(maildPaths, r.Method+" "+r.URL.Path)
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "ok"})
	}))
	defer maild.Close()
	autoputer := httptest.NewServer(http.NewServeMux())
	defer autoputer.Close()
	h, priv := newEmailTestHandler(t, maild.URL, autoputer.URL)
	h.SetAPIKeyValidator(stubEmailAPIKeys{})

	withKey := func(method, path string) int {
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("Authorization", "Bearer "+auth.APIKeyPrefix+"test-secret")
		w := httptest.NewRecorder()
		h.HandleAPI(w, req)
		return w.Code
	}
	for _, c := range []struct{ method, path string }{
		{http.MethodPut, "/api/email/digest"},
		{http.MethodPut, "/api/email/digest/"},
		{http.MethodPut, "/api/email//digest"},
		{http.MethodDelete, "/api/email/digest"},
		{http.MethodPost, "/api/email/digest/send-now"},
		{http.MethodPost, "/api/email/digest/send-now/"},
		{http.MethodPost, "/api/email/digest//send-now"},
	} {
		if code := withKey(c.method, c.path); code != http.StatusForbidden {
			t.Fatalf("API key %s %s status = %d, want 403", c.method, c.path, code)
		}
	}
	if len(maildPaths) != 0 {
		t.Fatalf("API-key digest write reached maild: %v", maildPaths)
	}
	if code := withKey(http.MethodGet, "/api/email/digest"); code != http.StatusAccepted {
		t.Fatalf("API key digest read status = %d, want forwarded", code)
	}

	for _, c := range []struct{ method, path string }{
		{http.MethodPut, "/api/email/digest"},
		{http.MethodPost, "/api/email/digest/send-now?preview=1"},
	} {
		req := httptest.NewRequest(c.method, c.path, nil)
		req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWTWithEmail(priv, "user-real", "owner@example.com")})
		w := httptest.NewRecorder()
		h.HandleAPI(w, req)
		if w.Code != http.StatusAccepted {
			t.Fatalf("owner session %s %s status = %d, want forwarded; body=%s", c.method, c.path, w.Code, w.Body.String())
		}
	}
}
