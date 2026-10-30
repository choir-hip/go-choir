package proxy

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/auth"
)

// stubEmailAPIKeys accepts any choir_sk_ key as an admin key of user-real.
type stubEmailAPIKeys struct{}

func (stubEmailAPIKeys) GetAPIKeyByHash(context.Context, string) (*auth.APIKey, error) {
	return &auth.APIKey{ID: "key-1", UserID: "user-real", Scopes: []string{"admin"}}, nil
}
func (stubEmailAPIKeys) TouchAPIKeyLastUsed(context.Context, string) error { return nil }
func (stubEmailAPIKeys) GetUserByID(id string) (*auth.User, error) {
	if id != "user-real" {
		return nil, sql.ErrNoRows
	}
	return &auth.User{ID: id, Email: "owner@example.com"}, nil
}

// FM 18 (docs/archive/problems/maild-send-gate-defects-2026-10-14.md): approving a
// send is an owner act, so a headless API key (which an agent inside the
// computer may hold) never reaches maild's send route; the owner's browser
// session does. Other mail routes keep API-key access.
func TestEmailSendRouteRefusesAPIKeyButForwardsOwnerSession(t *testing.T) {
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
	for _, path := range []string{
		"/api/email/drafts/draft-1/send",
		"/api/email/drafts/draft-1/send/",
		"/api/email/drafts//draft-1//send",
	} {
		if code := withKey(http.MethodPost, path); code != http.StatusForbidden {
			t.Fatalf("API key POST %s status = %d, want 403", path, code)
		}
	}
	if len(maildPaths) != 0 {
		t.Fatalf("API-key send reached maild: %v", maildPaths)
	}
	if code := withKey(http.MethodPost, "/api/email/drafts"); code != http.StatusAccepted {
		t.Fatalf("API key draft create status = %d, want forwarded", code)
	}
	if code := withKey(http.MethodGet, "/api/email/drafts/draft-1"); code != http.StatusAccepted {
		t.Fatalf("API key draft read status = %d, want forwarded", code)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/email/drafts/draft-1/send", nil)
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWTWithEmail(priv, "user-real", "owner@example.com")})
	w := httptest.NewRecorder()
	h.HandleAPI(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("owner session send status = %d, want forwarded; body=%s", w.Code, w.Body.String())
	}
	if last := maildPaths[len(maildPaths)-1]; last != "POST /api/email/drafts/draft-1/send" {
		t.Fatalf("maild saw %q", last)
	}
}
