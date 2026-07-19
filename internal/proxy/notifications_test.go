package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNotificationAPIForwardsToMaildWithTrustedUser(t *testing.T) {
	maild := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"path":            r.URL.Path,
			"user":            r.Header.Get("X-Authenticated-User"),
			"internal_caller": r.Header.Get("X-Internal-Caller"),
			"cookie":          r.Header.Get("Cookie"),
			"authorization":   r.Header.Get("Authorization"),
		})
	}))
	defer maild.Close()
	sandbox := httptest.NewServer(http.NewServeMux())
	defer sandbox.Close()
	h, priv := newEmailTestHandler(t, maild.URL, sandbox.URL)

	req := httptest.NewRequest(http.MethodPost, "/api/notifications/completion-email", strings.NewReader(`{"to_email":"owner@example.com","title":"Ready","status":"verified"}`))
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(priv, "user-real")})
	req.Header.Set("Authorization", "Bearer client-token")
	req.Header.Set("X-Authenticated-User", "spoofed")
	w := httptest.NewRecorder()

	h.HandleAPI(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["path"] != "/api/notifications/completion-email" {
		t.Fatalf("maild path = %q", resp["path"])
	}
	if resp["user"] != "user-real" || resp["internal_caller"] != "true" {
		t.Fatalf("forwarded auth = %+v", resp)
	}
	if resp["cookie"] != "" || resp["authorization"] != "" {
		t.Fatalf("client credentials leaked: %+v", resp)
	}
}
