package visualcapture

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Written before the code (docs/problems/
// selfdev-visual-capture-misses-the-changed-app-2026-11-02.md). The ways the
// package capture could fail, each checked below:
//  1. a patch that touches a package selects no package (or the wrong one:
//     a sibling file, a nested directory, a deleted file, a path outside);
//  2. a package name escapes the packages directory;
//  3. the plan misses a preset, a width or an opened kind, or names a route
//     twice (Freeze refuses a manifest that does);
//  4. the plan grows past its cap without a note saying what it dropped;
//  5. a -next package that declares no opens gets no documents;
//  6. a scene's backend answers more of /api than the shell needs to open
//     the window, or answers it outside a scene;
//  7. one failed scene fails the run (it must be a note for the judge);
//  8. the time budget runs past its bound;
//  9. the evidence validator refuses package captures, or admits a scene
//     whose route, viewport or theme does not match it.

func TestTouchedPackagesSelectsEveryPackageAPatchChanges(t *testing.T) {
	paths := []string{
		"frontend/src/packages/slides-next/App.svelte",
		"frontend/src/packages/slides-next/deck.js",
		"workspace/platform/frontend/src/packages/reader/reader.css",
		"/workspace/platform/frontend/src/packages/image/assets/icon.svg",
		"frontend/src/lib/TextureEditor.svelte",
		"frontend/src/packages/README.md",
		"frontend/src/packages/../lib/x.js",
		"frontend/src/packages/Bad_Name/App.svelte",
		"docs/frontend/src/packages/nope/App.svelte",
		"internal/agentcore/x.go",
	}
	got := TouchedPackages(paths)
	want := []string{"image", "reader", "slides-next"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("TouchedPackages = %v, want %v", got, want)
	}
	if got := TouchedPackages([]string{"frontend/src/lib/App.svelte"}); len(got) != 0 {
		t.Fatalf("a patch outside the packages touched %v", got)
	}
}

func TestPatchPathsReadsAUnifiedDiff(t *testing.T) {
	patch := []byte(`diff --git a/frontend/src/packages/slides-next/App.svelte b/frontend/src/packages/slides-next/App.svelte
new file mode 100644
--- /dev/null
+++ b/frontend/src/packages/slides-next/App.svelte
@@ -0,0 +1 @@
+<p>x</p>
diff --git a/frontend/src/packages/reader/old.js b/frontend/src/packages/reader/old.js
deleted file mode 100644
--- a/frontend/src/packages/reader/old.js
+++ /dev/null
@@ -1 +0,0 @@
-x
diff --git a/frontend/src/lib/a.js b/frontend/src/lib/a.js
--- a/frontend/src/lib/a.js
+++ b/frontend/src/lib/a.js
@@ -1 +1 @@
-+++ b/frontend/src/packages/fake/App.svelte
++y
`)
	got := TouchedPackages(PatchPaths(patch))
	if !reflect.DeepEqual(got, []string{"reader", "slides-next"}) {
		t.Fatalf("packages from the patch = %v", got)
	}
}

func TestValidPackageName(t *testing.T) {
	for _, ok := range []string{"slides-next", "reader", "a", "x2"} {
		if !ValidPackageName(ok) {
			t.Fatalf("%q refused", ok)
		}
	}
	for _, bad := range []string{"", "..", "a/b", "Slides", "a_b", "-x", strings.Repeat("a", 65), "a b"} {
		if ValidPackageName(bad) {
			t.Fatalf("%q admitted", bad)
		}
	}
}

func writeManifest(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(PackagesDir), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, PackageManifestFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPackageKindsFollowTheManifestOpens(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "reader", `{"name":"reader","opens":[{"media_type":"application/pdf","extensions":["pdf"]},{"media_type":"application/epub+zip","extensions":["epub"]}]}`)
	writeManifest(t, root, "image", `{"name":"image","opens":[{"media_type":"image/png","extensions":["png","jpg","webp","svg"]}]}`)
	writeManifest(t, root, "odd", `{"name":"odd","opens":[{"media_type":"text/calendar","extensions":["ics"]}]}`)
	writeManifest(t, root, "slides-next", `{"name":"slides-next","opens":[]}`)
	writeManifest(t, root, "many", `{"name":"many","opens":[{"media_type":"x","extensions":["pdf","pptx","html","epub","png"]}]}`)

	cases := map[string]struct {
		kinds []string
		note  string
	}{
		"reader":      {kinds: []string{"pdf", "epub"}},
		"image":       {kinds: []string{"image"}},
		"odd":         {kinds: nil, note: "no capture fixture for .ics"},
		"slides-next": {kinds: []string{"pdf", "pptx", "html"}, note: "declares no opens"},
		"many":        {kinds: []string{"pdf", "pptx", "html"}, note: "at most 3"},
	}
	for name, want := range cases {
		target, notes := LoadPackageTarget(root, name)
		var got []string
		for _, f := range target.Kinds {
			got = append(got, f.Kind)
		}
		if !reflect.DeepEqual(got, want.kinds) {
			t.Fatalf("%s kinds = %v, want %v", name, got, want.kinds)
		}
		if want.note != "" && !strings.Contains(strings.Join(notes, "\n"), want.note) {
			t.Fatalf("%s notes %q lack %q", name, notes, want.note)
		}
	}
	if target, notes := LoadPackageTarget(root, "missing"); target.Name != "" || len(notes) != 1 || !strings.Contains(notes[0], "no "+PackageManifestFile) {
		t.Fatalf("a missing package = %+v %q", target, notes)
	}
}

func TestPlanCoversEveryPresetWidthAndKind(t *testing.T) {
	slides := PackageTarget{Name: "slides-next", Kinds: []Fixture{fixtureFor("pdf"), fixtureFor("pptx"), fixtureFor("html")}}
	scenes, notes := PlanPackages([]PackageTarget{slides})
	if len(notes) != 0 {
		t.Fatalf("notes %q", notes)
	}
	// 3 presets x 2 widths x (3 kinds + empty) + 3 presets x (maximized + error) on desktop.
	if len(scenes) != 30 {
		t.Fatalf("%d scenes, want 30", len(scenes))
	}
	seen := map[string]bool{}
	cells := map[string]bool{}
	for _, s := range scenes {
		if err := s.Validate(); err != nil {
			t.Fatalf("scene %+v: %v", s, err)
		}
		r := s.Route()
		if seen[r] {
			t.Fatalf("route %s planned twice", r)
		}
		seen[r] = true
		if _, err := NormalizeRoute(r); err != nil {
			t.Fatalf("route %s: %v", r, err)
		}
		cells[s.Theme+"/"+s.Viewport+"/"+s.State] = true
	}
	for _, theme := range PackageThemes {
		for _, vp := range []string{ViewportDesktop, ViewportPhone} {
			for _, st := range []string{"open-pdf", "open-pptx", "open-html", "empty"} {
				if !cells[theme+"/"+vp+"/"+st] {
					t.Fatalf("plan lacks %s/%s/%s", theme, vp, st)
				}
			}
		}
		for _, st := range []string{"maximized-pdf", "error"} {
			if !cells[theme+"/"+ViewportDesktop+"/"+st] {
				t.Fatalf("plan lacks %s/desktop/%s", theme, st)
			}
		}
	}
	w, h := scenes[0].Size()
	if scenes[0].Viewport == ViewportDesktop && (w != DesktopViewportWidth || h != DesktopViewportHeight) {
		t.Fatalf("desktop size %dx%d", w, h)
	}
	for _, s := range scenes {
		if s.Viewport == ViewportPhone {
			if w, h := s.Size(); w != PhoneWidth || h != PhoneHeight {
				t.Fatalf("phone size %dx%d", w, h)
			}
		}
	}
}

func TestPlanCapsItsCapturesAndSaysWhatItDropped(t *testing.T) {
	kinds := []Fixture{fixtureFor("pdf"), fixtureFor("pptx"), fixtureFor("html")}
	scenes, notes := PlanPackages([]PackageTarget{{Name: "slides-next", Kinds: kinds}, {Name: "reader", Kinds: kinds[:1]}, {Name: "image"}})
	if len(scenes) != MaxPackageCaptures {
		t.Fatalf("%d scenes, want the cap %d", len(scenes), MaxPackageCaptures)
	}
	joined := strings.Join(notes, "\n")
	if !strings.Contains(joined, "at most 2 packages") || !strings.Contains(joined, "image") {
		t.Fatalf("no note on the third package: %q", notes)
	}
	if !strings.Contains(joined, "dropped") {
		t.Fatalf("no note on the dropped scenes: %q", notes)
	}
	// The first opened kind of every package, at every preset and width,
	// survives the cap.
	have := map[string]bool{}
	for _, s := range scenes {
		have[s.Package+"/"+s.Theme+"/"+s.Viewport+"/"+s.State] = true
	}
	for _, pkg := range []string{"slides-next", "reader"} {
		for _, theme := range PackageThemes {
			for _, vp := range []string{ViewportDesktop, ViewportPhone} {
				if !have[pkg+"/"+theme+"/"+vp+"/open-pdf"] {
					t.Fatalf("the cap dropped %s/%s/%s/open-pdf", pkg, theme, vp)
				}
			}
		}
	}
	for _, n := range notes {
		if len(n) > maxNoteLen {
			t.Fatalf("note longer than %d: %q", maxNoteLen, n)
		}
	}
}

func TestFixturesAreDeterministicDocuments(t *testing.T) {
	for _, kind := range []string{"pdf", "pptx", "html", "epub", "image"} {
		f := fixtureFor(kind)
		a, b := fixtureBytes(f.Kind), fixtureBytes(f.Kind)
		if len(a) == 0 || !bytes.Equal(a, b) {
			t.Fatalf("%s fixture is empty or differs between runs", kind)
		}
		switch kind {
		case "pdf":
			if !bytes.HasPrefix(a, []byte("%PDF-")) {
				t.Fatal("pdf fixture is not a PDF")
			}
		case "pptx", "epub":
			zr, err := zip.NewReader(bytes.NewReader(a), int64(len(a)))
			if err != nil {
				t.Fatalf("%s fixture is not a zip: %v", kind, err)
			}
			want := "ppt/slides/slide1.xml"
			if kind == "epub" {
				want = "mimetype"
				if zr.File[0].Name != "mimetype" || zr.File[0].Method != zip.Store {
					t.Fatal("epub mimetype must be the first, stored entry")
				}
			}
			found := false
			for _, zf := range zr.File {
				found = found || zf.Name == want
			}
			if !found {
				t.Fatalf("%s fixture lacks %s", kind, want)
			}
		}
	}
}

func get(t *testing.T, srv *httptest.Server, method, path string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(`{"x":1}`))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func TestSceneServerAnswersOnlyTheRoutesTheShellNeeds(t *testing.T) {
	fx := newFixture(t)
	cs := newCaptureServer(fx.serve)
	srv := httptest.NewServer(cs)
	defer srv.Close()

	// Outside a scene nothing changes: signed out, /api 503.
	if code, body := get(t, srv, "GET", "/auth/session"); code != 200 || !strings.Contains(body, `"authenticated":false`) {
		t.Fatalf("no scene: /auth/session %d %s", code, body)
	}
	if code, _ := get(t, srv, "GET", "/api/desktop/state"); code != http.StatusServiceUnavailable {
		t.Fatalf("no scene: /api/desktop/state %d", code)
	}
	if code, _ := get(t, srv, "GET", FixturePrefix+"deck.pdf"); code != http.StatusNotFound {
		t.Fatalf("no scene: fixture %d", code)
	}

	scene := Scene{Package: "slides-next", Theme: "futuristic-noir", Viewport: ViewportDesktop, State: "open-pdf", Fixture: "pdf"}
	cs.setScene(&scene, []Fixture{fixtureFor("pdf"), fixtureFor("html")})
	if code, body := get(t, srv, "GET", "/auth/session"); code != 200 || !strings.Contains(body, `"authenticated":true`) {
		t.Fatalf("scene: /auth/session %d %s", code, body)
	}
	code, body := get(t, srv, "GET", "/api/desktop/state")
	if code != 200 {
		t.Fatalf("scene: desktop state %d", code)
	}
	var state struct {
		Windows []struct {
			AppID      string         `json:"app_id"`
			Mode       string         `json:"mode"`
			AppContext map[string]any `json:"app_context"`
			Geometry   map[string]int `json:"geometry"`
		} `json:"windows"`
	}
	if err := json.Unmarshal([]byte(body), &state); err != nil || len(state.Windows) != 1 {
		t.Fatalf("desktop state %s: %v", body, err)
	}
	w := state.Windows[0]
	if w.AppID != "slides-next" || w.Mode != "normal" || w.AppContext["sourceUrl"] != FixturePrefix+"deck.pdf" || w.Geometry["width"] != DesktopWindowWidth {
		t.Fatalf("window %+v", w)
	}
	if code, body := get(t, srv, "GET", "/api/preferences/theme"); code != 200 || !strings.Contains(body, `"futuristic-noir"`) {
		t.Fatalf("theme %d %s", code, body)
	}
	if code, body := get(t, srv, "GET", "/api/media/recents"); code != 200 || !strings.Contains(body, FixturePrefix+"deck.html") {
		t.Fatalf("recents %d %s", code, body)
	}
	if code, _ := get(t, srv, "PUT", "/api/desktop/state"); code != 200 {
		t.Fatalf("desktop state save %d", code)
	}
	if code, body := get(t, srv, "GET", FixturePrefix+"deck.pdf"); code != 200 || !strings.HasPrefix(body, "%PDF-") {
		t.Fatalf("fixture %d", code)
	}
	if code, _ := get(t, srv, "GET", FixturePrefix+"../index.html"); code == 200 {
		t.Fatal("a fixture path left the fixture set")
	}
	// Everything else under /api stays closed, in a scene too.
	for _, p := range []string{"/api/files/secret.txt", "/api/agent/runs", "/api/model-policy/verify", "/api/assets/sha256/00"} {
		if code, _ := get(t, srv, "GET", p); code != http.StatusServiceUnavailable {
			t.Fatalf("scene: %s answered %d", p, code)
		}
	}
	if code, _ := get(t, srv, "POST", "/api/agent/runs"); code != http.StatusMethodNotAllowed {
		t.Fatalf("scene: POST elsewhere answered %d", code)
	}
	// Phone: the window fills the viewport; maximized: the shell's own mode.
	phone := Scene{Package: "slides-next", Theme: "london-salmon", Viewport: ViewportPhone, State: "maximized-pdf", Fixture: "pdf"}
	cs.setScene(&phone, nil)
	_, body = get(t, srv, "GET", "/api/desktop/state")
	if !strings.Contains(body, `"mode":"maximized"`) || !strings.Contains(body, `"width":390`) || !strings.Contains(body, `"present":true`) {
		t.Fatalf("phone maximized state %s", body)
	}
	cs.setScene(nil, nil)
	if code, _ := get(t, srv, "GET", "/api/desktop/state"); code != http.StatusServiceUnavailable {
		t.Fatal("the scene outlived setScene(nil)")
	}
}

func TestRunCapturesPackageScenesAsEvidence(t *testing.T) {
	fx := newFixture(t)
	src := t.TempDir()
	writeManifest(t, src, "slides-next", `{"name":"slides-next","opens":[{"media_type":"application/pdf","extensions":["pdf"]},{"media_type":"text/html","extensions":["html"]}]}`)
	opts := fx.opts(t, "/")
	opts.Packages = []string{"slides-next"}
	opts.SourceRoot = src
	opts.WaitMS = 20 // the scene page holds its load event this long
	setFakeMode(t, "fail:futuristic-noir/phone/open-html")
	m, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// "/" + 3 presets x 2 widths x (2 kinds + empty) + 3 x (maximized + error) - 1 failed.
	if len(m.Captures) != 1+18+6-1 {
		t.Fatalf("%d captures", len(m.Captures))
	}
	if m.Captures[0].Scene != nil || m.Captures[0].Route != "/" {
		t.Fatalf("first capture %+v", m.Captures[0])
	}
	for _, c := range m.Captures[1:] {
		if c.Scene == nil || c.Scene.Package != "slides-next" || c.Route != c.Scene.Route() {
			t.Fatalf("package capture %+v", c)
		}
		if w, h := c.Scene.Size(); c.Width != w || c.Height != h {
			t.Fatalf("capture %s is %dx%d", c.Route, c.Width, c.Height)
		}
	}
	if !strings.Contains(strings.Join(m.Notes, "\n"), "futuristic-noir/phone/open-html") {
		t.Fatalf("the failed scene is not a note: %q", m.Notes)
	}
	raw, err := os.ReadFile(filepath.Join(fx.out, ManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	tree, _ := digestTree(fx.serve)
	if _, err := ValidateEvidence(raw, evidenceFiles(t, fx.out), FrontendDigest(tree)); err != nil {
		t.Fatalf("package evidence does not validate: %v", err)
	}
}

func TestRunStopsPackageScenesAtTheTimeBudget(t *testing.T) {
	fx := newFixture(t)
	src := t.TempDir()
	writeManifest(t, src, "reader", `{"name":"reader","opens":[{"media_type":"application/pdf","extensions":["pdf"]}]}`)
	opts := fx.opts(t, "/")
	opts.Packages = []string{"reader"}
	opts.SourceRoot = src
	opts.WaitMS = 20 // the scene page holds its load event this long
	opts.PackageBudget = time.Nanosecond
	m, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(m.Captures) > 2 || !strings.Contains(strings.Join(m.Notes, "\n"), "time budget") {
		t.Fatalf("captures %d, notes %q", len(m.Captures), m.Notes)
	}
}

func TestOptionsRefuseABadPackageName(t *testing.T) {
	fx := newFixture(t)
	opts := fx.opts(t, "/")
	opts.Packages = []string{"../lib"}
	if _, err := opts.Normalize(); err == nil {
		t.Fatal("a package name that leaves the packages directory was admitted")
	}
	opts.Packages = []string{"a", "a"}
	if _, err := opts.Normalize(); err == nil {
		t.Fatal("a package named twice was admitted")
	}
}

func TestValidateEvidenceChecksPackageScenes(t *testing.T) {
	fx := newFixture(t)
	src := t.TempDir()
	writeManifest(t, src, "image", `{"name":"image","opens":[{"media_type":"image/png","extensions":["png"]}]}`)
	opts := fx.opts(t, "/")
	opts.Packages = []string{"image"}
	opts.SourceRoot = src
	opts.WaitMS = 20 // the scene page holds its load event this long
	if _, err := Run(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(fx.out, ManifestFile))
	tree, _ := digestTree(fx.serve)
	files := evidenceFiles(t, fx.out)
	for name, mutate := range map[string]func(m *Manifest){
		"route not the scene's": func(m *Manifest) { m.Captures[1].Route = "/?choir-capture=other" },
		"unknown theme": func(m *Manifest) {
			m.Captures[1].Scene.Theme = "vector-heart"
			m.Captures[1].Route = m.Captures[1].Scene.Route()
		},
		"wrong viewport": func(m *Manifest) { m.Captures[1].Width = 1280 },
		"bad package":    func(m *Manifest) { m.Captures[1].Scene.Package = "../x" },
		"long note":      func(m *Manifest) { m.Notes = []string{strings.Repeat("n", maxNoteLen+1)} },
	} {
		var m Manifest
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		mutate(&m)
		bad, _ := json.Marshal(m)
		if _, err := ValidateEvidence(bad, files, FrontendDigest(tree)); err == nil {
			t.Fatalf("%s: admitted", name)
		}
	}
}
