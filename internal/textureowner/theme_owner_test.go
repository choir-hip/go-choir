package textureowner

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// Failure list: theme_owner.go (R1-R5).

const themeDocBody = "# Newsprint\n\nA theme.\n\n```choir-theme\n" + `{"schema_version":3,"id":"newsprint","name":"Newsprint","schemes":["light"],
"colors":{"paper":"#F7F5F0","fg":"#111111","muted":"#555555","accent":"#B3261E","onAccent":"#FFFFFF"},
"radii":{"card":"0px"}}` + "\n```\n"

func getTheme(f *algoFixture, query, user string, mutate ...func(*http.Request)) (int, map[string]any, string) {
	w := f.call(http.MethodGet, "/api/texture/theme"+query, nil, append([]func(*http.Request){func(r *http.Request) { r.Header.Set("X-Authenticated-User", user) }}, mutate...)...)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out, w.Header().Get("Cache-Control")
}

func TestThemeRouteServesAValidatedThemeFromATexture(t *testing.T) {
	f := newAlgoFixture(t, "x")
	_ = startTextDoc(t, f.core.Store(), "doc-theme", "Newsprint", themeDocBody)
	code, got, cc := getTheme(f, "?doc=doc-theme", "user-1")
	if code != http.StatusOK || got["doc_id"] != "doc-theme" || got["revision_id"] == "" {
		t.Fatalf("status=%d answer=%v", code, got)
	}
	th, _ := got["theme"].(map[string]any)
	if th["id"] != "newsprint" || th["schema_version"] != float64(3) {
		t.Fatalf("theme=%v", th)
	}
	if cc != "no-store" { // R5
		t.Fatalf("cache-control=%q", cc)
	}
	rev, _ := got["revision_id"].(string)
	if code, _, _ := getTheme(f, "?doc=doc-theme&rev="+rev, "user-1"); code != http.StatusOK {
		t.Fatalf("by revision status=%d", code)
	}
	// R3: a revision of another document.
	_ = startTextDoc(t, f.core.Store(), "doc-other", "Other", "plain")
	if code, _, _ := getTheme(f, "?doc=doc-other&rev="+rev, "user-1"); code != http.StatusNotFound {
		t.Fatalf("R3: foreign revision status=%d", code)
	}
}

// R6: a store failure is a 5xx; 404 stays for missing and foreign documents.
func TestThemeRouteStoreFailureIsNot404(t *testing.T) {
	f := newAlgoFixture(t, "x")
	_ = startTextDoc(t, f.core.Store(), "doc-theme", "Newsprint", themeDocBody)
	if code, _, _ := getTheme(f, "?doc=nope", "user-1"); code != http.StatusNotFound {
		t.Fatalf("missing doc status=%d", code)
	}
	_ = f.core.Store().Close()
	if code, _, _ := getTheme(f, "?doc=doc-theme", "user-1"); code < 500 {
		t.Fatalf("R6: closed store answered %d, want 5xx", code)
	}
}

func TestThemeRouteRefusals(t *testing.T) {
	f := newAlgoFixture(t, "x")
	_ = startTextDoc(t, f.core.Store(), "doc-theme", "Newsprint", themeDocBody)
	_ = startTextDoc(t, f.core.Store(), "doc-bad", "Bad", strings.Replace(themeDocBody, "#111111", "#EEEEEE", 1))
	if code, _, _ := getTheme(f, "?doc=doc-bad", "user-1"); code != http.StatusUnprocessableEntity { // R2
		t.Fatalf("R2: invalid theme status=%d", code)
	}
	code1, body1, _ := getTheme(f, "?doc=doc-theme", "user-2") // R1
	code2, body2, _ := getTheme(f, "?doc=nope", "user-2")
	if code1 != http.StatusNotFound || code2 != code1 || body1["error"] != body2["error"] {
		t.Fatalf("R1: another owner's doc: %d %v vs missing %d %v", code1, body1, code2, body2)
	}
	if code, _, _ := getTheme(f, "?doc=doc-theme", "user-1", func(r *http.Request) { r.RemoteAddr = "" }); code != http.StatusForbidden { // R4
		t.Fatalf("R4: in-process status=%d", code)
	}
	if code, _, _ := getTheme(f, "?doc=doc-theme", ""); code != http.StatusUnauthorized {
		t.Fatalf("R4: anonymous status=%d", code)
	}
	if code, _, _ := getTheme(f, "", "user-1"); code != http.StatusBadRequest {
		t.Fatalf("missing doc status=%d", code)
	}
	if w := f.call(http.MethodPost, "/api/texture/theme?doc=doc-theme", map[string]string{}); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("R4: post status=%d", w.Code)
	}
}
