package proxy

// Ways the opinion publish route can fail (docs/problems/oped-not-connected-2026-10-20.md):
//
//  F2. An anonymous caller, an API key (a headless credential an agent may
//      hold) or a request body picks the opinion mode, the owner or the
//      access policy.
//  F9. The byline carries the signed-in user's email.

import (
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/platform"
)

func opinionTestHandler(t *testing.T, autoputerURL, corpusdURL string) (*Handler, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	h, err := NewHandler(&Config{AllowDirectAutoputerForTests: true, Port: "0", ComputerURL: autoputerURL,
		AuthPublicKeyPath: "/unused/in/test", CorpusdURL: corpusdURL}, pub)
	if err != nil {
		t.Fatal(err)
	}
	return h, priv
}

func TestOpinionPublishRequiresCookieSessionAndRefusesBodyPolicy(t *testing.T) {
	var corpusdBodies []platform.PublishTextureRequest
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req platform.PublishTextureRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		corpusdBodies = append(corpusdBodies, req)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(platform.PublishTextureResponse{RoutePath: "/pub/texture/my-view-abc123", PermalinkPath: "/pub/texture/my-view-abc123/v1", Revision: 1, State: "published"})
	}))
	defer corpusd.Close()
	autoputer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/texture/documents/doc-1":
			_ = json.NewEncoder(w).Encode(autoputerTextureDocument{DocID: "doc-1", OwnerID: "user-1", Title: "My view", CurrentRevisionID: "rev-1"})
		case "/api/texture/revisions/rev-1":
			_ = json.NewEncoder(w).Encode(autoputerTextureRevision{RevisionID: "rev-1", DocID: "doc-1", OwnerID: "user-1", VersionNumber: 0,
				RevisionHash: "h", Content: "I think the harbor matters.", Metadata: json.RawMessage(`{"access_policy":{"visibility":"private"}}`)})
		default:
			t.Errorf("autoputer path %s", r.URL.Path)
		}
	}))
	defer autoputer.Close()
	h, priv := opinionTestHandler(t, autoputer.URL, corpusd.URL)
	h.SetAPIKeyValidator(stubEmailAPIKeys{})

	post := func(body string, mutate func(*http.Request)) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "https://choir.news/api/platform/texture/opinions", strings.NewReader(body))
		if mutate != nil {
			mutate(req)
		}
		w := httptest.NewRecorder()
		h.HandleAPI(w, req)
		return w
	}
	cookie := func(req *http.Request) {
		req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWTWithEmail(priv, "user-1", "reader.person@example.com")})
	}

	if w := post(`{"doc_id":"doc-1"}`, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous opinion publish = %d, want 401", w.Code)
	}
	apiKey := func(req *http.Request) { req.Header.Set("Authorization", "Bearer "+auth.APIKeyPrefix+"test-secret") }
	if w := post(`{"doc_id":"doc-1"}`, apiKey); w.Code != http.StatusForbidden {
		t.Fatalf("API-key opinion publish = %d, want 403: %s", w.Code, w.Body)
	}
	for _, body := range []string{
		`{"doc_id":"doc-1","publish_mode":"auto"}`,
		`{"doc_id":"doc-1","owner_id":"user-2"}`,
		`{"doc_id":"doc-1","access_policy":{"visibility":"private"}}`,
		`{"doc_id":"doc-1","author_label":"The Editor"}`,
	} {
		if w := post(body, cookie); w.Code != http.StatusBadRequest {
			t.Fatalf("body %s = %d, want 400", body, w.Code)
		}
	}
	if len(corpusdBodies) != 0 {
		t.Fatalf("corpusd was reached by a refused call: %+v", corpusdBodies)
	}
	if req := httptest.NewRequest(http.MethodGet, "/api/platform/texture/opinions", nil); true {
		cookie(req)
		w := httptest.NewRecorder()
		h.HandleAPI(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("GET opinions = %d, want 405", w.Code)
		}
	}

	w := post(`{"doc_id":"doc-1","slug":"my-view"}`, cookie)
	if w.Code != http.StatusCreated {
		t.Fatalf("signed-in opinion publish = %d %s", w.Code, w.Body)
	}
	if len(corpusdBodies) != 1 {
		t.Fatalf("corpusd calls = %d", len(corpusdBodies))
	}
	got := corpusdBodies[0]
	if got.PublishMode != platform.PublishModeOpinion || got.OwnerID != "user-1" || got.RequestedBy != "user-1" {
		t.Fatalf("corpusd request = mode %q owner %q requester %q", got.PublishMode, got.OwnerID, got.RequestedBy)
	}
	// The access policy is the route's: public, whatever the revision's own
	// metadata says.
	if !strings.Contains(string(got.AccessPolicy), `"visibility":"public"`) {
		t.Fatalf("access policy = %s, want public", got.AccessPolicy)
	}
	// F9: a short handle, never the email or its parts.
	if !platform.ValidAuthorLabel(got.AuthorLabel) || !strings.HasPrefix(got.AuthorLabel, "writer-") ||
		strings.Contains(got.AuthorLabel, "@") || strings.Contains(strings.ToLower(got.AuthorLabel), "reader") || strings.Contains(got.AuthorLabel, "example") {
		t.Fatalf("author label = %q", got.AuthorLabel)
	}
	if again := opinionAuthorLabel("user-1"); again != got.AuthorLabel {
		t.Fatalf("the byline is not stable: %q then %q", got.AuthorLabel, again)
	}
	if opinionAuthorLabel("user-2") == got.AuthorLabel {
		t.Fatalf("two users share a byline")
	}
	var resp platform.PublishTextureResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || resp.PublicURL != "https://choir.news/pub/texture/my-view-abc123" {
		t.Fatalf("response = %s (%v)", w.Body, err)
	}
}

// corpusd's refusals (switch off 409, daily limit 429, mode conflict 409)
// reach the browser with their status and message, so the toolbar can say
// why.
func TestOpinionPublishPassesCorpusdRefusalsThrough(t *testing.T) {
	status := http.StatusTooManyRequests
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "daily opinion limit reached"})
	}))
	defer corpusd.Close()
	autoputer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/texture/documents/doc-1":
			_ = json.NewEncoder(w).Encode(autoputerTextureDocument{DocID: "doc-1", OwnerID: "user-1", Title: "T", CurrentRevisionID: "rev-1"})
		default:
			_ = json.NewEncoder(w).Encode(autoputerTextureRevision{RevisionID: "rev-1", DocID: "doc-1", OwnerID: "user-1", Content: "Words."})
		}
	}))
	defer autoputer.Close()
	h, priv := opinionTestHandler(t, autoputer.URL, corpusd.URL)
	for _, want := range []int{http.StatusTooManyRequests, http.StatusConflict} {
		status = want
		req := httptest.NewRequest(http.MethodPost, "https://choir.news/api/platform/texture/opinions", strings.NewReader(`{"doc_id":"doc-1"}`))
		req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(priv, "user-1")})
		w := httptest.NewRecorder()
		h.HandleAPI(w, req)
		if w.Code != want || !strings.Contains(w.Body.String(), "daily opinion limit") {
			t.Fatalf("corpusd %d -> browser %d %s", want, w.Code, w.Body)
		}
	}
}
