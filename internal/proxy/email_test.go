package proxy

import (
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
	req.Header.Set("X-Internal-Caller", "true")
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
	if resp["internal_caller"] != "" {
		t.Fatalf("client X-Internal-Caller leaked to maild: %q", resp["internal_caller"])
	}
}

func TestEmailSendToChoirFetchesSourcePacketAndSubmitsPromptBar(t *testing.T) {
	var maildUser, recordUser, recordInternalCaller string
	maild := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/email/messages/msg-1/source-packet":
			maildUser = r.Header.Get("X-Authenticated-User")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"source_packet_id": "src-email-1",
				"message_id":       "msg-1",
				"trust_label":      "UNTRUSTED_EXTERNAL_EMAIL",
				"from_address":     "sender@example.com",
				"subject":          "Project update",
				"snippet":          "Untrusted summary only",
			})
		case "/api/email/messages/msg-1/ingress-events":
			recordUser = r.Header.Get("X-Authenticated-User")
			recordInternalCaller = r.Header.Get("X-Internal-Caller")
			var payload map[string]string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode ingress body: %v", err)
			}
			if payload["source_packet_id"] != "src-email-1" || payload["conductor_submission_id"] != "run-email-1" {
				t.Fatalf("ingress payload = %+v", payload)
			}
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]string{"id": "ingress-1"})
		default:
			t.Fatalf("maild path = %s", r.URL.Path)
		}
	}))
	defer maild.Close()

	var promptUser, promptText string
	sandbox := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/prompt-bar" {
			t.Fatalf("sandbox path = %s", r.URL.Path)
		}
		promptUser = r.Header.Get("X-Authenticated-User")
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode prompt body: %v", err)
		}
		promptText = payload["text"]
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"submission_id": "run-email-1",
			"state":         "pending",
			"status_url":    "/api/prompt-bar/submissions/run-email-1",
		})
	}))
	defer sandbox.Close()

	h, priv := newEmailTestHandler(t, maild.URL, sandbox.URL)
	req := httptest.NewRequest(http.MethodPost, "/api/email/messages/msg-1/send-to-choir", nil)
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(priv, "user-real")})
	req.Header.Set("X-Authenticated-User", "spoofed")
	w := httptest.NewRecorder()

	h.HandleAPI(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", w.Code, w.Body.String())
	}
	if maildUser != "user-real" {
		t.Fatalf("maild user = %q, want user-real", maildUser)
	}
	if recordUser != "user-real" || recordInternalCaller != "true" {
		t.Fatalf("record headers user=%q internal=%q", recordUser, recordInternalCaller)
	}
	if promptUser != "user-real" {
		t.Fatalf("prompt user = %q, want user-real", promptUser)
	}
	for _, want := range []string{"UNTRUSTED_EXTERNAL_EMAIL", "src-email-1", "msg-1", "Project update"} {
		if !strings.Contains(promptText, want) {
			t.Fatalf("prompt text missing %q:\n%s", want, promptText)
		}
	}
	var resp emailSendToChoirResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.SubmissionID != "run-email-1" || resp.SourcePacketID != "src-email-1" {
		t.Fatalf("unexpected response: %+v", resp)
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
