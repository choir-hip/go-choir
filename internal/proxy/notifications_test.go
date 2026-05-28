package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

func TestFeatureAdoptionCompletionWatchSendsMailAfterTerminalStatus(t *testing.T) {
	origPoll := featureAdoptionWatchPollInterval
	origTimeout := featureAdoptionWatchTimeout
	featureAdoptionWatchPollInterval = 5 * time.Millisecond
	featureAdoptionWatchTimeout = time.Second
	t.Cleanup(func() {
		featureAdoptionWatchPollInterval = origPoll
		featureAdoptionWatchTimeout = origTimeout
	})

	mailSeen := make(chan map[string]any, 1)
	maild := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode mail body: %v", err)
		}
		body["path"] = r.URL.Path
		body["user"] = r.Header.Get("X-Authenticated-User")
		body["internal_caller"] = r.Header.Get("X-Internal-Caller")
		mailSeen <- body
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "sent", "provider_message_id": "msg-test"})
	}))
	defer maild.Close()

	polls := 0
	sandbox := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/adoptions/adopt-1" {
			t.Fatalf("sandbox path = %s", r.URL.Path)
		}
		if r.Header.Get("X-Authenticated-User") != "user-real" {
			t.Fatalf("sandbox user = %q", r.Header.Get("X-Authenticated-User"))
		}
		polls++
		status := "verifying"
		if polls >= 2 {
			status = "verified"
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"adoption_id": "adopt-1",
			"package_id":  "pkg-1",
			"app_id":      "Feature One",
			"status":      status,
		})
	}))
	defer sandbox.Close()

	h, priv := newEmailTestHandler(t, maild.URL, sandbox.URL)
	req := httptest.NewRequest(http.MethodPost, "/api/notifications/watch-adoption-completion", strings.NewReader(`{
		"adoption_id":"adopt-1",
		"to_email":"owner@example.com",
		"title":"Feature One",
		"feature_id":"pkg-1",
		"link":"/?app=features"
	}`))
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(priv, "user-real")})
	w := httptest.NewRecorder()

	h.HandleAPI(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", w.Code, w.Body.String())
	}

	select {
	case got := <-mailSeen:
		if got["path"] != "/api/notifications/completion-email" {
			t.Fatalf("mail path = %v", got["path"])
		}
		if got["user"] != "user-real" || got["internal_caller"] != "true" {
			t.Fatalf("mail auth = %+v", got)
		}
		if got["to_email"] != "owner@example.com" || got["status"] != "verified" || got["title"] != "Feature One" {
			t.Fatalf("mail payload = %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for completion email")
	}
}
