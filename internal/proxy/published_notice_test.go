package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/platform"
)

// A successful publish tells the publisher's computer, so it arms its polls of
// the notice and op-ed feeds; a failed publish does not, and a computer that
// cannot be told does not fail the publish.
func TestPublishTellsTheComputerItWasPublished(t *testing.T) {
	status := http.StatusCreated
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		if status == http.StatusCreated {
			_ = json.NewEncoder(w).Encode(platform.PublishTextureResponse{RoutePath: "/pub/texture/v-abc", PermalinkPath: "/pub/texture/v-abc/v1", Revision: 1, State: "published"})
		}
	}))
	defer corpusd.Close()
	var told []string
	tellStatus := http.StatusNoContent
	autoputer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/published"):
			told = append(told, r.URL.Path+" as "+r.Header.Get("X-Authenticated-User"))
			w.WriteHeader(tellStatus)
		case r.URL.Path == "/api/texture/documents/doc-1":
			_ = json.NewEncoder(w).Encode(autoputerTextureDocument{DocID: "doc-1", OwnerID: "user-1", Title: "View", CurrentRevisionID: "rev-1"})
		case r.URL.Path == "/api/texture/revisions/rev-1":
			_ = json.NewEncoder(w).Encode(autoputerTextureRevision{RevisionID: "rev-1", DocID: "doc-1", OwnerID: "user-1", VersionNumber: 1, RevisionHash: "h", Content: "Words."})
		default:
			http.NotFound(w, r)
		}
	}))
	defer autoputer.Close()
	h, priv := opinionTestHandler(t, autoputer.URL, corpusd.URL)
	post := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "https://choir.news/api/platform/texture/opinions", strings.NewReader(`{"doc_id":"doc-1"}`))
		req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWTWithEmail(priv, "user-1", "p@example.com")})
		w := httptest.NewRecorder()
		h.HandleAPI(w, req)
		return w
	}
	if w := post(); w.Code != http.StatusCreated {
		t.Fatalf("publish = %d %s", w.Code, w.Body)
	}
	if len(told) != 1 || told[0] != "/api/texture/documents/doc-1/published as user-1" {
		t.Fatalf("the computer was told %v", told)
	}
	// The computer being unable to take the notice does not fail the publish.
	tellStatus = http.StatusInternalServerError
	if w := post(); w.Code != http.StatusCreated {
		t.Fatalf("publish with a failing notice = %d", w.Code)
	}
	// A refused publish tells no one.
	told, status = nil, http.StatusConflict
	if w := post(); w.Code != http.StatusConflict || len(told) != 0 {
		t.Fatalf("refused publish = %d, told %v", w.Code, told)
	}
}
