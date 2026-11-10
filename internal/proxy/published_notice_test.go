package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

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
	var toldMu sync.Mutex
	tellStatus := http.StatusNoContent
	autoputer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/published"):
			toldMu.Lock()
			told = append(told, r.URL.Path+" as "+r.Header.Get("X-Authenticated-User"))
			toldMu.Unlock()
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
	waitTold := func(n int) []string {
		for i := 0; i < 200; i++ {
			toldMu.Lock()
			got := append([]string(nil), told...)
			toldMu.Unlock()
			if len(got) >= n {
				return got
			}
			time.Sleep(10 * time.Millisecond)
		}
		toldMu.Lock()
		defer toldMu.Unlock()
		return append([]string(nil), told...)
	}
	if got := waitTold(1); len(got) != 1 || got[0] != "/api/texture/documents/doc-1/published as user-1" {
		t.Fatalf("the computer was told %v", got)
	}
	// The computer being unable to take the notice does not fail the publish.
	tellStatus = http.StatusInternalServerError
	if w := post(); w.Code != http.StatusCreated {
		t.Fatalf("publish with a failing notice = %d", w.Code)
	}
	// A refused publish tells no one.
	waitTold(2)
	toldMu.Lock()
	told, status = nil, http.StatusConflict
	toldMu.Unlock()
	if w := post(); w.Code != http.StatusConflict {
		t.Fatalf("refused publish = %d", w.Code)
	}
	time.Sleep(100 * time.Millisecond)
	toldMu.Lock()
	defer toldMu.Unlock()
	if len(told) != 0 {
		t.Fatalf("a refused publish told the computer: %v", told)
	}
}

// A computer that hangs on the notice cannot delay the publish: the handler
// returns with the response while the notice is still in flight.
func TestPublishNoticeDoesNotBlockThePublishResponse(t *testing.T) {
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(platform.PublishTextureResponse{RoutePath: "/pub/texture/v-abc", PermalinkPath: "/pub/texture/v-abc/v1", Revision: 1, State: "published"})
	}))
	defer corpusd.Close()
	release := make(chan struct{})
	arrived := make(chan struct{}, 1)
	autoputer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/published"):
			arrived <- struct{}{}
			<-release // the computer hangs
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/api/texture/documents/doc-1":
			_ = json.NewEncoder(w).Encode(autoputerTextureDocument{DocID: "doc-1", OwnerID: "user-1", Title: "View", CurrentRevisionID: "rev-1"})
		case r.URL.Path == "/api/texture/revisions/rev-1":
			_ = json.NewEncoder(w).Encode(autoputerTextureRevision{RevisionID: "rev-1", DocID: "doc-1", OwnerID: "user-1", VersionNumber: 1, RevisionHash: "h", Content: "Words."})
		default:
			http.NotFound(w, r)
		}
	}))
	defer autoputer.Close()
	defer close(release)
	h, priv := opinionTestHandler(t, autoputer.URL, corpusd.URL)
	req := httptest.NewRequest(http.MethodPost, "https://choir.news/api/platform/texture/opinions", strings.NewReader(`{"doc_id":"doc-1"}`))
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWTWithEmail(priv, "user-1", "p@example.com")})
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { h.HandleAPI(rec, req); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("the publish waited for a hung computer")
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("publish = %d", rec.Code)
	}
	select {
	case <-arrived: // and the notice still goes out, on its own
	case <-time.After(3 * time.Second):
		t.Fatal("the computer was never told")
	}
}
