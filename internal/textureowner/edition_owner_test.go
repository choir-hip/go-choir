package textureowner

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// Failure list: edition_owner.go (D1-D4).

func getEdition(f *algoFixture, user string, mutate ...func(*http.Request)) (int, map[string]any, string) {
	w := f.call(http.MethodGet, "/api/texture/edition", nil, append([]func(*http.Request){func(r *http.Request) { r.Header.Set("X-Authenticated-User", user) }}, mutate...)...)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out, w.Header().Get("Cache-Control")
}

func TestEditionRouteFindsTheOwnersEdition(t *testing.T) {
	f := newAlgoFixture(t, "x")
	if code, got, _ := getEdition(f, "user-1"); code != http.StatusOK || got["found"] == true { // D3
		t.Fatalf("no edition: status=%d answer=%v", code, got)
	}
	_ = startTextDoc(t, f.core.Store(), "doc-edition", "  my   EDITION ", "# What I follow\n\n- Ukraine\n")
	_ = startTextDoc(t, f.core.Store(), "doc-other", "Interview, 11 Nov", "talk")
	code, got, cc := getEdition(f, "user-1")
	if code != http.StatusOK || got["found"] != true || got["doc_id"] != "doc-edition" || got["revision_id"] == "" {
		t.Fatalf("status=%d answer=%v", code, got)
	}
	if cc != "no-store" {
		t.Fatalf("cache-control=%q", cc)
	}
	raw, _ := json.Marshal(got)
	if strings.Contains(string(raw), "Ukraine") { // D4
		t.Fatalf("D4: text leaked: %s", raw)
	}
	if _, got, _ := getEdition(f, "user-2"); got["found"] == true { // D1
		t.Fatalf("D1: user-2 saw %v", got)
	}
}

func TestEditionRouteIsOwnerOnly(t *testing.T) {
	f := newAlgoFixture(t, "x")
	_ = startTextDoc(t, f.core.Store(), "doc-edition", "My edition", "x")
	if code, _, _ := getEdition(f, "user-1", func(r *http.Request) { r.RemoteAddr = "" }); code != http.StatusForbidden {
		t.Fatalf("in-process status=%d", code)
	}
	if code, _, _ := getEdition(f, ""); code != http.StatusUnauthorized {
		t.Fatalf("anonymous status=%d", code)
	}
	if w := f.call(http.MethodPost, "/api/texture/edition", map[string]string{}); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("post status=%d", w.Code)
	}
}
