package visualcapture

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Package captures (docs/problems/
// selfdev-visual-capture-misses-the-changed-app-2026-11-02.md). A route
// capture shows the signed-out frontend; an app package
// (frontend/src/packages/<name>/) opens only in a signed-in desktop with a
// restored window, a theme and a document. For each package named, the tool
// renders that package's window in the real shell of the built frontend, at
// each theme preset and two widths, with a fixture document for each kind
// the package's manifest opens: the screenshot set of the app design bar
// (docs/design/app-design-bar-2026-10-26.md check 1). The capture server
// plays the scene: the few routes the shell reads to restore one window, and
// the fixture documents, answered only while that scene's shot runs.

const (
	// SourceRoot is the candidate's source tree in the capsule; package
	// manifests are read from it.
	SourceRoot = "/workspace/platform"
	// PackagesDir is where app packages live under the source root.
	PackagesDir = "frontend/src/packages/"
	// PackageManifestFile is a package's manifest.
	PackageManifestFile = "choir-package.json"

	// MaxPackages bounds the packages one run renders.
	MaxPackages = 2
	// MaxPackageCaptures bounds one run's package captures.
	MaxPackageCaptures = 32
	// MaxCaptures bounds a manifest: the routes and the package captures.
	MaxCaptures = MaxRoutes + MaxPackageCaptures
	// MaxFixtureKinds bounds the document kinds one package opens in the plan.
	MaxFixtureKinds = 3
	// PackageBudget bounds the wall time of one run's package captures; the
	// scenes not reached are named in a note.
	PackageBudget = 10 * time.Minute
	// MaxNotes bounds the manifest's notes; maxNoteLen bounds each.
	MaxNotes   = 16
	maxNoteLen = 300

	// FixturePrefix is where the scene's fixture documents are served: not
	// under /api, and only during a scene.
	FixturePrefix = "/choir-capture-fixtures/"
	// SceneQueryKey names a package capture's scene in its route.
	SceneQueryKey = "choir-capture"

	ViewportDesktop = "desktop"
	ViewportPhone   = "phone"
	// The desktop scene is the bar's 1100x750 window (750 of content under
	// the 36px titlebar) in a viewport with room for the shell's prompt bar.
	DesktopViewportWidth  = 1180
	DesktopViewportHeight = 1000
	DesktopWindowWidth    = 1100
	DesktopWindowHeight   = 786
	PhoneWidth            = 390
	PhoneHeight           = 844

	// The states a scene renders.
	StateEmpty = "empty"
	StateError = "error"
	stateOpen  = "open-"
	stateMax   = "maximized-"
	maxPkgName = 64
)

// PackageThemes are the design bar's three presets (frontend/src/lib/theme.ts).
var PackageThemes = []string{"london-salmon", "futuristic-noir", "carbon-fiber-kintsugi"}

// nextOriginalKinds names the kinds an original app that is not a package
// opens, for a <name>-next rewrite that declares no opens (the side-by-side
// rule: a -next app leaves the file routes to its original). Slides opens
// PDF, PPTX and HTML decks (docs/definitions/choir-selfdev-apps-side-by-side-2026-11-02.md).
var nextOriginalKinds = map[string][]string{
	"slides": {"pdf", "pptx", "html"},
}

// ValidPackageName admits a package directory name: lowercase letters,
// digits and inner dashes, as check-packages names them.
func ValidPackageName(name string) bool {
	if name == "" || len(name) > maxPkgName || name[0] == '-' || name[len(name)-1] == '-' {
		return false
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}

// TouchedPackages lists, sorted, the packages whose directory a set of
// changed paths reaches into. Paths may be repo-relative or carry the
// capsule's workspace/platform prefix.
func TouchedPackages(paths []string) []string {
	seen := map[string]bool{}
	for _, p := range paths {
		p = strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(p)), "/")
		p = strings.TrimPrefix(p, strings.TrimPrefix(SourceRoot, "/")+"/")
		rest, ok := strings.CutPrefix(p, PackagesDir)
		if !ok {
			continue
		}
		name, file, ok := strings.Cut(rest, "/")
		if !ok || file == "" || !ValidPackageName(name) {
			continue
		}
		seen[name] = true
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// PatchPaths lists the paths a unified diff names on its diff --git lines.
// Hunk bodies are skipped, so a changed line that looks like a header names
// nothing.
func PatchPaths(patch []byte) []string {
	var out []string
	sc := bufio.NewScanner(bytes.NewReader(patch))
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		rest, ok := strings.CutPrefix(sc.Text(), "diff --git ")
		if !ok {
			continue
		}
		a, b, ok := strings.Cut(rest, " b/")
		if !ok {
			continue
		}
		out = append(out, strings.TrimPrefix(a, "a/"), b)
	}
	return out
}

// Fixture is one document kind a scene opens.
type Fixture struct {
	Kind      string `json:"kind"`
	MediaType string `json:"media_type"`
	File      string `json:"file"`
	Title     string `json:"title"`
}

var fixtures = map[string]Fixture{
	"pdf":   {Kind: "pdf", MediaType: "application/pdf", File: "deck.pdf", Title: "Quarterly review.pdf"},
	"pptx":  {Kind: "pptx", MediaType: "application/vnd.openxmlformats-officedocument.presentationml.presentation", File: "deck.pptx", Title: "Pitch.pptx"},
	"html":  {Kind: "html", MediaType: "text/html", File: "deck.html", Title: "Talk.html"},
	"epub":  {Kind: "epub", MediaType: "application/epub+zip", File: "book.epub", Title: "Sample book.epub"},
	"image": {Kind: "image", MediaType: "image/png", File: "picture.png", Title: "Picture.png"},
}

// extensionKinds maps an extension a manifest opens to its fixture kind.
var extensionKinds = map[string]string{
	"pdf": "pdf", "pptx": "pptx", "html": "html", "htm": "html", "epub": "epub",
	"png": "image", "jpg": "image", "jpeg": "image", "gif": "image", "webp": "image", "avif": "image", "bmp": "image", "svg": "image",
}

func fixtureFor(kind string) Fixture { return fixtures[kind] }

// PackageTarget is one package to render and the kinds it opens.
type PackageTarget struct {
	Name  string
	Kinds []Fixture
}

type packageManifest struct {
	Name  string `json:"name"`
	Opens []struct {
		MediaType  string   `json:"media_type"`
		Extensions []string `json:"extensions"`
	} `json:"opens"`
}

func readPackageManifest(sourceRoot, name string) (packageManifest, error) {
	var m packageManifest
	full := filepath.Join(sourceRoot, filepath.FromSlash(PackagesDir), name, PackageManifestFile)
	info, err := os.Lstat(full)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 64<<10 {
		return m, fmt.Errorf("no %s", PackageManifestFile)
	}
	raw, err := os.ReadFile(full)
	if err != nil {
		return m, fmt.Errorf("no %s", PackageManifestFile)
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return m, fmt.Errorf("%s does not parse", PackageManifestFile)
	}
	return m, nil
}

// LoadPackageTarget reads a package's manifest from the source tree and
// picks a fixture for each kind it opens. A package that is missing yields
// no target and a note; the notes say what the plan could not cover.
func LoadPackageTarget(sourceRoot, name string) (PackageTarget, []string) {
	m, err := readPackageManifest(sourceRoot, name)
	if err != nil {
		return PackageTarget{}, []string{note("package %s: %v under %s; nothing captured for it", name, err, PackagesDir+name)}
	}
	var exts []string
	for _, o := range m.Opens {
		exts = append(exts, o.Extensions...)
	}
	var notes []string
	if len(exts) == 0 {
		if base, ok := strings.CutSuffix(name, "-next"); ok && base != "" {
			if orig, err := readPackageManifest(sourceRoot, base); err == nil {
				for _, o := range orig.Opens {
					exts = append(exts, o.Extensions...)
				}
			} else {
				exts = append(exts, nextOriginalKinds[base]...)
			}
			if len(exts) > 0 {
				notes = append(notes, note("package %s declares no opens; its scenes open the kinds %s opens", name, base))
			}
		}
	}
	t := PackageTarget{Name: name}
	seen := map[string]bool{}
	for _, ext := range exts {
		ext = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(ext), "."))
		kind, ok := extensionKinds[ext]
		if !ok {
			notes = append(notes, note("package %s opens .%s; there is no capture fixture for .%s", name, ext, ext))
			continue
		}
		if seen[kind] {
			continue
		}
		seen[kind] = true
		if len(t.Kinds) == MaxFixtureKinds {
			notes = append(notes, note("package %s opens more kinds than a run captures (at most %d); %s is not shown", name, MaxFixtureKinds, kind))
			continue
		}
		t.Kinds = append(t.Kinds, fixtures[kind])
	}
	return t, notes
}

// Scene is one package capture: which package, preset, width and state.
type Scene struct {
	Package  string `json:"package"`
	Theme    string `json:"theme"`
	Viewport string `json:"viewport"`
	State    string `json:"state"`
	// Fixture is the kind of document the scene opens, if any.
	Fixture string `json:"fixture,omitempty"`
}

// Route is the scene's capture route: the shell's root, with the scene
// named in the query so every capture has its own route.
func (s Scene) Route() string {
	return "/?" + SceneQueryKey + "=" + s.Package + "/" + s.Theme + "/" + s.Viewport + "/" + s.State
}

// Size is the scene's viewport.
func (s Scene) Size() (int, int) {
	if s.Viewport == ViewportPhone {
		return PhoneWidth, PhoneHeight
	}
	return DesktopViewportWidth, DesktopViewportHeight
}

// Validate checks a scene names a valid package, a preset, a viewport and a
// state that matches its fixture.
func (s Scene) Validate() error {
	if !ValidPackageName(s.Package) {
		return fmt.Errorf("scene package %q is not a package name", s.Package)
	}
	if !contains(PackageThemes, s.Theme) {
		return fmt.Errorf("scene theme %q is not a capture preset", s.Theme)
	}
	if s.Viewport != ViewportDesktop && s.Viewport != ViewportPhone {
		return fmt.Errorf("scene viewport %q is not desktop or phone", s.Viewport)
	}
	switch {
	case s.State == StateEmpty && s.Fixture == "":
	case s.State == StateError && s.Fixture != "":
	case s.State == stateOpen+s.Fixture, s.State == stateMax+s.Fixture:
	case s.State == stateMax+"empty" && s.Fixture == "":
	default:
		return fmt.Errorf("scene state %q does not match fixture %q", s.State, s.Fixture)
	}
	if s.Fixture != "" {
		if _, ok := fixtures[s.Fixture]; !ok {
			return fmt.Errorf("scene fixture %q is not a capture fixture", s.Fixture)
		}
	}
	return nil
}

// PlanPackages is the run's package scenes. For each package, preset and
// width: each kind it opens and its empty state; on desktop also the
// maximized window (the shell's mode, with present set in the app context
// for a package that renders a present view) and the error state (a fixture
// that is not there). Past MaxPackageCaptures the lowest-priority scenes are
// dropped (the first opened kind first kept, then other kinds, empty,
// maximized, error), and a note says so.
func PlanPackages(targets []PackageTarget) ([]Scene, []string) {
	var notes []string
	if len(targets) > MaxPackages {
		var left []string
		for _, t := range targets[MaxPackages:] {
			left = append(left, t.Name)
		}
		notes = append(notes, note("a run renders at most %d packages; not captured: %s (capture them in another run)", MaxPackages, strings.Join(left, ", ")))
		targets = targets[:MaxPackages]
	}
	type ranked struct {
		Scene
		rank, order int
	}
	var all []ranked
	add := func(s Scene, rank int) { all = append(all, ranked{s, rank, len(all)}) }
	for _, t := range targets {
		for _, theme := range PackageThemes {
			for _, vp := range []string{ViewportDesktop, ViewportPhone} {
				for i, f := range t.Kinds {
					add(Scene{Package: t.Name, Theme: theme, Viewport: vp, State: stateOpen + f.Kind, Fixture: f.Kind}, min(i, 1))
				}
				add(Scene{Package: t.Name, Theme: theme, Viewport: vp, State: StateEmpty}, 2)
			}
			first := ""
			if len(t.Kinds) > 0 {
				first = t.Kinds[0].Kind
			}
			if first != "" {
				add(Scene{Package: t.Name, Theme: theme, Viewport: ViewportDesktop, State: stateMax + first, Fixture: first}, 3)
				add(Scene{Package: t.Name, Theme: theme, Viewport: ViewportDesktop, State: StateError, Fixture: first}, 4)
			} else {
				add(Scene{Package: t.Name, Theme: theme, Viewport: ViewportDesktop, State: stateMax + "empty"}, 3)
			}
		}
	}
	if len(all) > MaxPackageCaptures {
		sort.SliceStable(all, func(i, j int) bool { return all[i].rank < all[j].rank })
		dropped := map[string]int{}
		for _, r := range all[MaxPackageCaptures:] {
			dropped[r.State]++
		}
		all = all[:MaxPackageCaptures]
		sort.Slice(all, func(i, j int) bool { return all[i].order < all[j].order })
		var parts []string
		for st, n := range dropped {
			parts = append(parts, fmt.Sprintf("%d %s", n, st))
		}
		sort.Strings(parts)
		notes = append(notes, note("the plan is capped at %d package captures; dropped %s", MaxPackageCaptures, strings.Join(parts, ", ")))
	}
	out := make([]Scene, len(all))
	for i, r := range all {
		out[i] = r.Scene
	}
	return out, notes
}

// note formats a manifest note within maxNoteLen and without control
// characters.
func note(format string, args ...any) string {
	s := strings.Map(func(r rune) rune {
		if r < ' ' || r == 0x7f {
			return ' '
		}
		return r
	}, fmt.Sprintf(format, args...))
	if len(s) > maxNoteLen {
		s = s[:maxNoteLen-3] + "..."
	}
	return s
}

func validNote(s string) error {
	if s == "" || len(s) > maxNoteLen {
		return fmt.Errorf("a note is empty or longer than %d bytes", maxNoteLen)
	}
	for _, r := range s {
		if r < ' ' || r == 0x7f {
			return errors.New("a note carries a control character")
		}
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// fixtureFiles are the generated documents, by file name.
var fixtureFiles = func() map[string][]byte {
	out := map[string][]byte{}
	for _, f := range fixtures {
		out[f.File] = fixtureBytes(f.Kind)
	}
	return out
}()

// fixtureBytes builds a fixture document. Each is small, labelled as a
// sample, and the same on every run.
func fixtureBytes(kind string) []byte {
	titles := []string{"Quarterly review", "Revenue", "Costs", "Next steps"}
	switch kind {
	case "pdf":
		return fixturePDF(titles)
	case "pptx":
		return fixtureZip([][2]string{
			{"[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="xml" ContentType="application/xml"/></Types>`},
			{"ppt/slides/slide1.xml", pptxSlide("Pitch", "Why now")},
			{"ppt/slides/slide2.xml", pptxSlide("Market", "Big and growing")},
			{"ppt/slides/slide3.xml", pptxSlide("Team", "Small and fast")},
			{"ppt/slides/slide4.xml", pptxSlide("Ask", "A sample deck for the layout capture")},
		}, false)
	case "html":
		var b strings.Builder
		b.WriteString("<!doctype html><html><head><meta charset=\"utf-8\"><title>Sample talk</title></head><body>")
		for _, t := range titles {
			fmt.Fprintf(&b, "<section><h1>%s</h1><p>Body text for %s, with a second sentence so the line wraps in the capture.</p></section>", t, strings.ToLower(t))
		}
		b.WriteString("</body></html>")
		return []byte(b.String())
	case "epub":
		chapter := func(t string) string {
			return `<?xml version="1.0" encoding="UTF-8"?><html xmlns="http://www.w3.org/1999/xhtml"><head><title>` + t + `</title></head><body><h1>` + t +
				`</h1><p>A sample chapter for the layout capture. It has enough text to fill a few lines of the reading column, so the gutters, the measure and the type can be judged.</p><p>A second paragraph follows the first.</p></body></html>`
		}
		return fixtureZip([][2]string{
			{"mimetype", "application/epub+zip"},
			{"META-INF/container.xml", `<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`},
			{"OEBPS/content.opf", `<?xml version="1.0" encoding="UTF-8"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="id">choir-capture-fixture</dc:identifier><dc:title>Sample book</dc:title><dc:language>en</dc:language><meta property="dcterms:modified">2026-01-01T00:00:00Z</meta></metadata><manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="c1" href="c1.xhtml" media-type="application/xhtml+xml"/><item id="c2" href="c2.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="c1"/><itemref idref="c2"/></spine></package>`},
			{"OEBPS/nav.xhtml", `<?xml version="1.0" encoding="UTF-8"?><html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>Contents</title></head><body><nav epub:type="toc"><ol><li><a href="c1.xhtml">Chapter one</a></li><li><a href="c2.xhtml">Chapter two</a></li></ol></nav></body></html>`},
			{"OEBPS/c1.xhtml", chapter("Chapter one")},
			{"OEBPS/c2.xhtml", chapter("Chapter two")},
		}, true)
	case "image":
		img := image.NewRGBA(image.Rect(0, 0, 960, 540))
		for y := 0; y < 540; y++ {
			for x := 0; x < 960; x++ {
				img.Set(x, y, color.RGBA{uint8(40 + x/8), uint8(80 + y/6), 160, 255})
			}
		}
		var buf bytes.Buffer
		_ = png.Encode(&buf, img)
		return buf.Bytes()
	}
	return nil
}

func pptxSlide(title, body string) string {
	return `<?xml version="1.0" encoding="UTF-8"?><p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:sp><p:txBody><a:p><a:r><a:t>` +
		title + `</a:t></a:r></a:p></p:txBody></p:sp><p:sp><p:txBody><a:p><a:r><a:t>` + body + `</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`
}

func fixtureZip(entries [][2]string, storeFirst bool) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	stamp := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, e := range entries {
		method := zip.Deflate
		if storeFirst && i == 0 {
			method = zip.Store
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: e[0], Method: method, Modified: stamp})
		if err != nil {
			return nil
		}
		_, _ = w.Write([]byte(e[1]))
	}
	if err := zw.Close(); err != nil {
		return nil
	}
	return buf.Bytes()
}

func fixturePDF(pages []string) []byte {
	objs := []string{"<< /Type /Catalog /Pages 2 0 R >>"}
	var kids []string
	for i := range pages {
		kids = append(kids, fmt.Sprintf("%d 0 R", 3+i*2))
	}
	objs = append(objs, fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(pages)))
	for i, t := range pages {
		stream := fmt.Sprintf("BT /F1 48 Tf 60 300 Td (%s) Tj ET BT /F1 20 Tf 60 240 Td (Sample page %d for the layout capture) Tj ET", t, i+1)
		objs = append(objs,
			fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 960 540] /Resources << /Font << /F1 << /Type /Font /Subtype /Type1 /BaseFont /Helvetica >> >> >> /Contents %d 0 R >>", 4+i*2),
			fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream))
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objs))
	for i, o := range objs {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return b.Bytes()
}
