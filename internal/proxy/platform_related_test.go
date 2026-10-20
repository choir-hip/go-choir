package proxy

// Ways the public related-stories route can fail (related-stories design
// 5.7), written before the code:
//
//  1. It needs a session, or resolves or calls a computer: the list must
//     work for a signed-out reader while the paper computer is stopped.
//  2. It forwards without the internal-caller marker corpusd requires, or
//     drops the route/limit, or invents a query corpusd did not get.
//  3. A POST is served.
//  4. A missing route is forwarded instead of refused.
//  5. A success is not cacheable (the reader's repeat opens all reach
//     corpusd), or a partial answer / the kill-switch answer IS cacheable
//     (an empty list sticks for five minutes after the fault clears).
//  6. A corpusd failure reaches the reader as the corpusd error text, or as
//     a hang, instead of a plain 502 the page ignores.

import (
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/platform"
)

func TestPlatformRelatedStoriesIsPublicCacheableAndNeverCallsAComputer(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	guest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("related reached a computer: %s %s", r.Method, r.URL.Path)
		http.Error(w, "computer stopped", http.StatusServiceUnavailable)
	}))
	defer guest.Close()
	var gotInternal, gotQuery string
	var answer platform.RelatedStories
	corpusdFails := false
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/platform/publications/related" {
			t.Fatalf("corpusd path: got %s", r.URL.Path)
		}
		if corpusdFails {
			http.Error(w, `{"error":"dolt exploded: secret detail"}`, http.StatusInternalServerError)
			return
		}
		gotInternal = r.Header.Get("X-Internal-Caller")
		gotQuery = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(answer)
	}))
	defer corpusd.Close()
	h, err := NewHandler(&Config{AllowDirectAutoputerForTests: true, Port: "0",
		ComputerURL:       guest.URL,
		AuthPublicKeyPath: "/unused/in/test",
		CorpusdURL:        corpusd.URL}, pub)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	get := func(target string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.HandleAPI(w, httptest.NewRequest(http.MethodGet, target, nil))
		return w
	}

	answer = platform.RelatedStories{Route: "/pub/texture/harbor-abc", Revision: 3, Basis: platform.RelatedBasis{Captures: 2, Mode: "captures"},
		Links: []platform.RelatedLink{{FrontStory: platform.FrontStory{PublicationID: "pub-2", Title: "Ferries resume"}, Reason: "capture"}}}
	w := get("/api/platform/publications/related?route=/pub/texture/harbor-abc&limit=5")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	if gotInternal != "true" || gotQuery != "limit=5&route=%2Fpub%2Ftexture%2Fharbor-abc" {
		t.Fatalf("corpusd got internal=%q query=%q", gotInternal, gotQuery)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "public, max-age=300" {
		t.Fatalf("success Cache-Control = %q, want public, max-age=300", cc)
	}
	var got platform.RelatedStories
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil || len(got.Links) != 1 || got.Links[0].Reason != "capture" {
		t.Fatalf("body = %+v %v", got, err)
	}

	// An empty complete answer is still a success and cacheable; a partial
	// or switched-off answer is not.
	answer = platform.RelatedStories{Route: "/pub/texture/harbor-abc", Links: []platform.RelatedLink{}, Basis: platform.RelatedBasis{Mode: "captures"}}
	if cc := get("/api/platform/publications/related?route=/pub/texture/x").Header().Get("Cache-Control"); cc != "public, max-age=300" {
		t.Fatalf("empty answer Cache-Control = %q", cc)
	}
	answer = platform.RelatedStories{Route: "/pub/texture/x", Links: []platform.RelatedLink{}, Partial: true, Basis: platform.RelatedBasis{Mode: "captures"}}
	if cc := get("/api/platform/publications/related?route=/pub/texture/x").Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("partial answer Cache-Control = %q, want no-store", cc)
	}
	answer = platform.RelatedStories{Route: "/pub/texture/x", Links: []platform.RelatedLink{}, Basis: platform.RelatedBasis{Mode: "off"}}
	w = get("/api/platform/publications/related?route=/pub/texture/x")
	if cc := w.Header().Get("Cache-Control"); cc != "no-store" || w.Code != http.StatusOK {
		t.Fatalf("switched-off answer = %d Cache-Control %q, want 200 no-store", w.Code, cc)
	}

	if w := get("/api/platform/publications/related"); w.Code != http.StatusBadRequest {
		t.Fatalf("no route: %d", w.Code)
	}
	post := httptest.NewRecorder()
	h.HandleAPI(post, httptest.NewRequest(http.MethodPost, "/api/platform/publications/related?route=/pub/texture/x", nil))
	if post.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST: %d", post.Code)
	}

	corpusdFails = true
	fail := get("/api/platform/publications/related?route=/pub/texture/x")
	if fail.Code != http.StatusBadGateway || fail.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("corpusd failure: %d Cache-Control %q, want 502 no-store", fail.Code, fail.Header().Get("Cache-Control"))
	}
	if body := fail.Body.String(); body == "" || strings.Contains(body, "dolt") || strings.Contains(body, "secret") {
		t.Fatalf("corpusd failure leaked its text: %s", body)
	}
}
