package textureowner

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/styleguide"
)

type styleResp struct {
	Source     string            `json:"source"`
	Reason     string            `json:"reason"`
	DocID      string            `json:"doc_id"`
	RevisionID string            `json:"revision_id"`
	Design     styleguide.Design `json:"design"`
	Ignored    []string          `json:"ignored"`
}

func getStyle(t *testing.T, f *algoFixture, user string, mutate ...func(*http.Request)) (*httptest.ResponseRecorder, styleResp) {
	t.Helper()
	w := f.call(http.MethodGet, "/api/texture/style", nil, append([]func(*http.Request){func(r *http.Request) { r.Header.Set("X-Authenticated-User", user) }}, mutate...)...)
	var out styleResp
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w, out
}

const guideBody = "# Style Guide\n\nprose\n\n## Design values\n\n- serif: modern\n- headline-weight: 800\n- dek-style: italic\n- nameplate-rule: none\n"

// The end-to-end artifact: a guide Texture, read through the Texture surface,
// answers closed names; no guide text is in the answer.
func TestStyleRouteReadsTheOwnersGuideTexture(t *testing.T) {
	f := newAlgoFixture(t, "x")
	_ = startTextDoc(t, f.core.Store(), "doc-style", "Style Guide", guideBody)
	w, got := getStyle(t, f, "user-1")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if got.Source != "document" || got.DocID != "doc-style" || got.RevisionID == "" {
		t.Fatalf("answer=%+v", got)
	}
	want := styleguide.Design{Serif: "modern", HeadlineWeight: "800", DekStyle: "italic", NameplateRule: "none"}
	if got.Design != want {
		t.Fatalf("design=%+v", got.Design)
	}
	if strings.Contains(w.Body.String(), "prose") {
		t.Fatalf("guide text leaked: %s", w.Body.String())
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("cache-control=%q", cc)
	}
	if dir := os.Getenv("STYLE_ARTIFACT_DIR"); dir != "" {
		_ = os.WriteFile(filepath.Join(dir, "style-answer.json"), w.Body.Bytes(), 0o644)
	}
}

func TestStyleRouteNoGuideIsTheDefault(t *testing.T) {
	f := newAlgoFixture(t, "x")
	w, got := getStyle(t, f, "user-1")
	if w.Code != http.StatusOK || got.Source != "default" || got.Design != styleguide.Default() {
		t.Fatalf("status=%d answer=%+v", w.Code, got)
	}
}

func TestStyleRouteHostileGuideIsClosedNames(t *testing.T) {
	f := newAlgoFixture(t, "x")
	_ = startTextDoc(t, f.core.Store(), "doc-style", "Style Guide", "## Design values\n- serif: red; } body{display:none}\n- headline-weight: url(//evil)\n")
	w, got := getStyle(t, f, "user-1")
	if w.Code != http.StatusOK || got.Design != styleguide.Default() || len(got.Ignored) != 2 {
		t.Fatalf("status=%d answer=%+v", w.Code, got)
	}
	if strings.Contains(w.Body.String(), "evil") && !strings.Contains(w.Body.String(), "ignored") {
		t.Fatalf("body=%s", w.Body.String())
	}
}

func TestStyleRouteOversizeGuideIsTheDefaultWithAReason(t *testing.T) {
	f := newAlgoFixture(t, "x")
	_ = startTextDoc(t, f.core.Store(), "doc-style", "Style Guide", guideBody+strings.Repeat("filler line\n", 2000))
	_, got := getStyle(t, f, "user-1")
	if got.Source != "default" || got.Reason == "" || got.Design != styleguide.Default() {
		t.Fatalf("answer=%+v", got)
	}
}

func TestStyleRouteAnotherOwnersGuideDoesNotLeak(t *testing.T) {
	f := newAlgoFixture(t, "x")
	_ = startTextDoc(t, f.core.Store(), "doc-style", "Style Guide", guideBody)
	_, got := getStyle(t, f, "user-2")
	if got.Source != "default" || got.DocID != "" || got.Design != styleguide.Default() {
		t.Fatalf("user-2 saw %+v", got)
	}
}

func TestStyleRouteIsOwnerOnly(t *testing.T) {
	f := newAlgoFixture(t, "x")
	_ = startTextDoc(t, f.core.Store(), "doc-style", "Style Guide", guideBody)
	if w, _ := getStyle(t, f, "user-1", func(r *http.Request) { r.RemoteAddr = "" }); w.Code != http.StatusForbidden {
		t.Fatalf("in-process call status=%d", w.Code)
	}
	if w, _ := getStyle(t, f, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status=%d", w.Code)
	}
	if w := f.call(http.MethodPost, "/api/texture/style", map[string]string{}); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("post status=%d", w.Code)
	}
}

// Style reaches no prompt in pass 1: nothing outside the read surface imports
// the package.
func TestStyleGuideReachesNoPrompt(t *testing.T) {
	root := filepath.Join("..")
	var offenders []string
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		if strings.Contains(p, "styleguide"+string(filepath.Separator)) || strings.HasSuffix(p, "style_owner.go") {
			return nil
		}
		b, _ := os.ReadFile(p)
		if strings.Contains(string(b), "internal/styleguide") {
			offenders = append(offenders, p)
		}
		return nil
	})
	if len(offenders) != 0 {
		t.Fatalf("styleguide imported outside the Texture read surface: %v", offenders)
	}
}
