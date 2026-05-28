package proxy

import (
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newEmailTestHandler(t *testing.T, maildURL, sandboxURL string) (*Handler, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	h, err := NewHandler(&Config{
		Port:              "0",
		SandboxURL:        sandboxURL,
		AuthPublicKeyPath: "/unused",
		PlatformdURL:      DefaultPlatformdURL,
		MaildURL:          maildURL,
	}, pub)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	return h, priv
}

func TestEmailAPIForwardsToMaildWithTrustedUser(t *testing.T) {
	maild := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"path":            r.URL.Path,
			"user":            r.Header.Get("X-Authenticated-User"),
			"x_user_id":       r.Header.Get("X-User-Id"),
			"authorization":   r.Header.Get("Authorization"),
			"cookie":          r.Header.Get("Cookie"),
			"internal_caller": r.Header.Get("X-Internal-Caller"),
		})
	}))
	defer maild.Close()
	sandbox := httptest.NewServer(http.NewServeMux())
	defer sandbox.Close()
	h, priv := newEmailTestHandler(t, maild.URL, sandbox.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/email/messages?folder=inbox", nil)
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(priv, "user-real")})
	req.Header.Set("Authorization", "Bearer client-token")
	req.Header.Set("X-Authenticated-User", "spoofed")
	req.Header.Set("X-User-Id", "spoofed-user-id")
	req.Header.Set("X-Internal-Caller", "client-spoof")
	w := httptest.NewRecorder()

	h.HandleAPI(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["path"] != "/api/email/messages" {
		t.Fatalf("maild path = %q", resp["path"])
	}
	if resp["user"] != "user-real" {
		t.Fatalf("forwarded user = %q, want user-real", resp["user"])
	}
	if resp["x_user_id"] != "" {
		t.Fatalf("spoofed X-User-Id leaked to maild: %q", resp["x_user_id"])
	}
	if resp["authorization"] != "" || resp["cookie"] != "" {
		t.Fatalf("client credential header leaked to maild: authorization=%q cookie=%q", resp["authorization"], resp["cookie"])
	}
	if resp["internal_caller"] != "true" {
		t.Fatalf("internal caller marker = %q, want proxy-injected true", resp["internal_caller"])
	}
}

func TestEmailSendToChoirPathIsOnlyForwardedToMaild(t *testing.T) {
	maildCalled := false
	maild := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		maildCalled = true
		if r.URL.Path != "/api/email/messages/msg-1/send-to-choir" {
			t.Fatalf("maild path = %s", r.URL.Path)
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}))
	defer maild.Close()
	sandboxCalled := false
	sandbox := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sandboxCalled = true
		t.Fatalf("proxy must not submit email to prompt-bar")
	}))
	defer sandbox.Close()
	h, priv := newEmailTestHandler(t, maild.URL, sandbox.URL)

	req := httptest.NewRequest(http.MethodPost, "/api/email/messages/msg-1/send-to-choir", nil)
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(priv, "user-real")})
	w := httptest.NewRecorder()

	h.HandleAPI(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", w.Code, w.Body.String())
	}
	if !maildCalled {
		t.Fatal("maild was not called")
	}
	if sandboxCalled {
		t.Fatal("sandbox prompt bar was called")
	}
}

func TestEmailWebhookPathIsNotProxiedThroughAuthenticatedAPI(t *testing.T) {
	maild := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("webhook path should not be proxied through generic proxy")
	}))
	defer maild.Close()
	sandbox := httptest.NewServer(http.NewServeMux())
	defer sandbox.Close()
	h, priv := newEmailTestHandler(t, maild.URL, sandbox.URL)

	req := httptest.NewRequest(http.MethodPost, "/api/email/resend/webhook", nil)
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(priv, "user-real")})
	w := httptest.NewRecorder()
	h.HandleAPI(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}
