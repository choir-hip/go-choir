package visualcapture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The test binary doubles as a fake Chromium: with FAKE_CHROMIUM set it
// reads the argv a capture passes, fetches the page over loopback, and
// writes a PNG of the window size (or misbehaves as FAKE_CHROMIUM_MODE says).
func TestMain(m *testing.M) {
	if os.Getenv("FAKE_CHROMIUM") == "1" {
		os.Exit(fakeChromium(os.Args[1:]))
	}
	os.Exit(m.Run())
}

func fakeChromium(args []string) int {
	if len(args) == 1 && args[0] == "--version" {
		fmt.Println("FakeChromium 150.0")
		return 0
	}
	mode := os.Getenv("FAKE_CHROMIUM_MODE")
	if mode == "hang" {
		time.Sleep(time.Hour)
	}
	var out, url string
	w, h := 0, 0
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--screenshot="):
			out = strings.TrimPrefix(a, "--screenshot=")
		case strings.HasPrefix(a, "--window-size="):
			dims := strings.Split(strings.TrimPrefix(a, "--window-size="), ",")
			w, _ = strconv.Atoi(dims[0])
			h, _ = strconv.Atoi(dims[1])
		case strings.HasPrefix(a, "http://"):
			url = a
		}
	}
	if logPath := os.Getenv("FAKE_CHROMIUM_ARGV"); logPath != "" {
		f, _ := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		fmt.Fprintln(f, strings.Join(args, "\x1f"))
		fmt.Fprintln(f, "ENV "+strings.Join(os.Environ(), "\x1f"))
		_ = f.Close()
	}
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fetch:", err)
		return 1
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != 200 || !bytes.Contains(body, []byte("<html")) {
		fmt.Fprintf(os.Stderr, "page answered %d\n", resp.StatusCode)
		return 1
	}
	// Like a browser, ask for the page's scripts and module imports, so the
	// server sees a missing chunk (fix 3, trace review round 4).
	for _, ref := range subresourceRefs(body) {
		if u, err := neturl.Parse(url); err == nil {
			if target, err := u.Parse(ref); err == nil {
				if r, err := http.Get(target.String()); err == nil {
					_ = r.Body.Close()
				}
			}
		}
	}
	if mode == "wrongsize" {
		h--
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.RGBA{255, 255, 255, 255}
			if mode != "blank" && x < len(body) && y < 40 {
				c = color.RGBA{0, 0, 0, 255}
			}
			img.Set(x, y, c)
		}
	}
	f, err := os.Create(out)
	if err != nil {
		return 1
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return 1
	}
	return 0
}

var subresourcePattern = regexp.MustCompile(`(?:src=|from|import)\s*"?([./][^"\s>]*)`)

// subresourceRefs lists the src= attributes and module import specifiers a
// fake page names.
func subresourceRefs(body []byte) []string {
	var out []string
	for _, m := range subresourcePattern.FindAllSubmatch(body, -1) {
		out = append(out, string(m[1]))
	}
	return out
}

// setFakeMode passes the fake Chromium its switches through the test hook
// (Chromium's environment is otherwise rebuilt from scratch).
func setFakeMode(t *testing.T, mode string) {
	t.Helper()
	prev := testBrowserEnv
	testBrowserEnv = []string{"FAKE_CHROMIUM=1", "FAKE_CHROMIUM_MODE=" + mode, "FAKE_CHROMIUM_ARGV=" + filepath.Join(fakeLogDir, "argv.log")}
	t.Cleanup(func() { testBrowserEnv = prev })
}

var fakeLogDir = os.TempDir()

type fixture struct {
	serve, out, argvLog string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	dir := t.TempDir()
	fakeLogDir = dir
	fx := fixture{serve: filepath.Join(dir, "frontend"), out: filepath.Join(dir, "evidence"), argvLog: filepath.Join(dir, "argv.log")}
	for name, body := range map[string]string{
		"index.html":                  "<html><body><h1>Choir</h1><script src=/assets/app.js></script></body></html>",
		"assets/app.js":               "console.log('app')",
		"choir-drill/visual-1.html":   "<html><body><h1>visual-1</h1></body></html>",
		"choir-drill/visual-1.txt":    "visual-1\n",
		"legal/terms/index.html.keep": "",
	} {
		p := filepath.Join(fx.serve, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HTTP_PROXY", "http://proxy.invalid:3128")
	setFakeMode(t, "")
	return fx
}

func (fx fixture) opts(t *testing.T, routes ...string) Options {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return Options{Routes: routes, Browser: exe, Fontconfig: "/nix/store/x-fonts.conf", ServeRoot: fx.serve, OutputDir: fx.out,
		Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) }}
}

func evidenceFiles(t *testing.T, dir string) []EvidenceFile {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []EvidenceFile
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256Hex(data)
		out = append(out, EvidenceFile{Name: e.Name(), Size: int64(len(data)), SHA256: sum})
	}
	return out
}

func TestRunCapturesEveryRouteAndWritesABoundManifest(t *testing.T) {
	fx := newFixture(t)
	// A stale capture from an earlier run must not survive (Freeze refuses
	// files the manifest does not name).
	if err := os.MkdirAll(fx.out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fx.out, "stale.png"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := Run(context.Background(), fx.opts(t, "/", "/choir-drill/visual-1.html"))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if m.Kind != ManifestKind || len(m.Captures) != 2 || m.Browser.Version != "FakeChromium 150.0" || m.WaitMS != DefaultWaitMS {
		t.Fatalf("manifest = %+v", m)
	}
	tree, err := digestTree(fx.serve)
	if err != nil {
		t.Fatal(err)
	}
	if m.Subject.FrontendSHA256 != FrontendDigest(tree) || m.Subject.FrontendFiles != len(tree) {
		t.Fatalf("subject %+v does not bind the served tree", m.Subject)
	}
	for i, c := range m.Captures {
		if c.Width != DefaultWidth || c.Height != DefaultHeight || c.FullPage || !c.ReducedMotion || c.Blank || c.TakenAt != "2026-10-15T12:00:00Z" {
			t.Fatalf("capture %d = %+v", i, c)
		}
		data, err := os.ReadFile(filepath.Join(fx.out, c.File))
		if err != nil {
			t.Fatal(err)
		}
		if ArtifactRef(sha256Hex(data)) != c.Artifact || int64(len(data)) != c.Bytes {
			t.Fatalf("capture %d is not named by its bytes", i)
		}
	}
	raw, err := os.ReadFile(filepath.Join(fx.out, ManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateEvidence(raw, evidenceFiles(t, fx.out), FrontendDigest(tree)); err != nil {
		t.Fatalf("the run's own evidence does not validate: %v", err)
	}
	if _, err := os.Stat(filepath.Join(fx.out, "stale.png")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale capture survived: %v", err)
	}
	// V7: Chromium is told to stay off the network and gets no proxy.
	log, _ := os.ReadFile(fx.argvLog)
	for _, want := range []string{"--no-proxy-server", "--headless=new", "--force-prefers-reduced-motion", "--window-size=1280,800", "--virtual-time-budget=5000", "--host-resolver-rules=MAP * ~NOTFOUND , EXCLUDE 127.0.0.1"} {
		if !strings.Contains(string(log), want) {
			t.Fatalf("Chromium argv lacks %q", want)
		}
	}
	if strings.Contains(string(log), "proxy.invalid") {
		t.Fatal("Chromium inherited a proxy variable")
	}
	if !strings.Contains(string(log), "FONTCONFIG_FILE=/nix/store/x-fonts.conf") {
		t.Fatal("Chromium did not get the fontconfig file")
	}
}

func TestRunRefusesWithoutTheGrant(t *testing.T) {
	fx := newFixture(t)
	opts := fx.opts(t)
	opts.Browser = ""
	if _, err := Run(context.Background(), opts); !errors.Is(err, ErrNotGranted) {
		t.Fatalf("err = %v, want ErrNotGranted", err)
	}
	if _, err := os.Stat(fx.out); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("an ungranted run wrote output")
	}
}

func TestRunRefusesWithoutABuiltFrontend(t *testing.T) {
	fx := newFixture(t)
	opts := fx.opts(t)
	opts.ServeRoot = t.TempDir()
	if _, err := Run(context.Background(), opts); err == nil || !strings.Contains(err.Error(), "no built frontend") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunKillsAHungBrowserAtTheDeadline(t *testing.T) {
	fx := newFixture(t)
	setFakeMode(t, "hang")
	opts := fx.opts(t)
	opts.ShotDeadline = time.Second
	start := time.Now()
	_, err := Run(context.Background(), opts)
	if err == nil || !strings.Contains(err.Error(), "did not finish") {
		t.Fatalf("err = %v", err)
	}
	if elapsed := time.Since(start); elapsed > 30*time.Second {
		t.Fatalf("hung browser held the run for %s", elapsed)
	}
	if _, err := os.Stat(filepath.Join(fx.out, ManifestFile)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a failed run left a manifest")
	}
}

func TestRunRefusesAWrongSizeCapture(t *testing.T) {
	fx := newFixture(t)
	setFakeMode(t, "wrongsize")
	if _, err := Run(context.Background(), fx.opts(t)); err == nil || !strings.Contains(err.Error(), "not the 1280x800 viewport") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunMarksABlankCapture(t *testing.T) {
	fx := newFixture(t)
	setFakeMode(t, "blank")
	m, err := Run(context.Background(), fx.opts(t))
	if err != nil {
		t.Fatal(err)
	}
	if !m.Captures[0].Blank {
		t.Fatal("a uniform capture is not marked blank")
	}
}

func TestOptionsBounds(t *testing.T) {
	base := Options{Browser: "/bin/chromium"}
	cases := []struct {
		name string
		edit func(*Options)
		want string
	}{
		{"too many routes", func(o *Options) { o.Routes = []string{"/a", "/b", "/c", "/d", "/e", "/f", "/g"} }, "at most 6"},
		{"duplicate route", func(o *Options) { o.Routes = []string{"/a", "/a"} }, "named twice"},
		{"too wide", func(o *Options) { o.Width = 4000 }, "outside"},
		{"too tall", func(o *Options) { o.Height = 34888 }, "no full page"},
		{"long wait", func(o *Options) { o.WaitMS = 60000 }, "wait"},
		{"other origin", func(o *Options) { o.Routes = []string{"https://example.com/"} }, "path on this origin"},
	}
	for _, tc := range cases {
		o := base
		tc.edit(&o)
		if _, err := o.Normalize(); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: err = %v, want %q", tc.name, err, tc.want)
		}
	}
	o, err := base.Normalize()
	if err != nil || len(o.Routes) != 1 || o.Routes[0] != "/" || o.Width != 1280 || o.Height != 800 || o.ServeRoot != ServeRoot || o.OutputDir != OutputDir {
		t.Fatalf("defaults = %+v, %v", o, err)
	}
}

func TestNormalizeRoute(t *testing.T) {
	for _, ok := range []string{"/", "/desk", "/choir-drill/x.html", "/?view=files"} {
		if _, err := NormalizeRoute(ok); err != nil {
			t.Fatalf("%q refused: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "desk", "//evil.example/x", "/a/../../etc/passwd", "/x#frag", "/a b", "http://127.0.0.1/", `/a\b`} {
		if _, err := NormalizeRoute(bad); err == nil {
			t.Fatalf("%q admitted", bad)
		}
	}
}

func TestServerServesTheTreeWithAnSPAFallbackAndASignedOutBackend(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(root, "index.html"), []byte("<html>index</html>"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "assets", "app.js"), []byte("app"), 0o644)
	outside := filepath.Join(t.TempDir(), "secret.txt")
	_ = os.WriteFile(outside, []byte("secret"), 0o644)
	_ = os.Symlink(outside, filepath.Join(root, "leak.txt"))
	srv := httptest.NewServer(Server(root))
	defer srv.Close()
	get := func(p string) (int, string) {
		resp, err := http.Get(srv.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	for p, want := range map[string]struct {
		code int
		body string
	}{
		"/":                   {200, "index"},
		"/desk/files":         {200, "index"},
		"/assets/app.js":      {200, "app"},
		"/assets/missing.js":  {404, ""},
		"/api/runtime/health": {503, "no backend"},
		// A signed-out session, as the auth service answers one with no
		// cookie: the app shell leaves Loading for the signed-out desktop.
		"/auth/session":        {200, `{"authenticated":false}`},
		"/auth/session?x=1":    {200, `{"authenticated":false}`},
		"/auth/api-keys":       {401, "not signed in"},
		"/leak.txt":            {404, ""},
		"/../../etc/passwd":    {400, ""},
		"/assets/../index.htm": {404, ""},
	} {
		code, body := get(p)
		if code != want.code || !strings.Contains(body, want.body) {
			t.Fatalf("GET %s = %d %q, want %d containing %q", p, code, body, want.code, want.body)
		}
	}
}

func validManifest(t *testing.T, frontend string, captures ...[]byte) ([]byte, []EvidenceFile) {
	t.Helper()
	m := Manifest{Kind: ManifestKind, Subject: Subject{FrontendSHA256: frontend, FrontendFiles: 3}, Browser: Browser{Name: "chromium"}, WaitMS: 5000}
	var files []EvidenceFile
	for i, data := range captures {
		sum := sha256Hex(data)
		m.Captures = append(m.Captures, Capture{Artifact: ArtifactRef(sum), File: sum + ".png", MediaType: MediaTypePNG, Bytes: int64(len(data)),
			Route: fmt.Sprintf("/r%d", i), Width: 1280, Height: 800, ReducedMotion: true, TakenAt: "2026-10-15T12:00:00Z"})
		files = append(files, EvidenceFile{Name: sum + ".png", Size: int64(len(data)), SHA256: sum})
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, EvidenceFile{Name: ManifestFile, Size: int64(len(raw)), SHA256: sha256Hex(raw)})
	return raw, files
}

func TestValidateEvidenceRefusesWhatDoesNotBind(t *testing.T) {
	frontend := strings.Repeat("a", 64)
	raw, files := validManifest(t, frontend, []byte("png-1"), []byte("png-2"))
	if m, err := ValidateEvidence(raw, files, frontend); err != nil || len(m.Routes()) != 2 {
		t.Fatalf("valid evidence refused: %v", err)
	}
	if _, err := ValidateEvidence(raw, files, strings.Repeat("b", 64)); err == nil || !strings.Contains(err.Error(), "different frontend build") {
		t.Fatalf("V8 stale capture admitted: %v", err)
	}
	extra := append(append([]EvidenceFile(nil), files...), EvidenceFile{Name: "hand-made.png", Size: 3, SHA256: sha256Hex([]byte("abc"))})
	if _, err := ValidateEvidence(raw, extra, frontend); err == nil || !strings.Contains(err.Error(), "does not name") {
		t.Fatalf("extra file admitted: %v", err)
	}
	tampered := append([]EvidenceFile(nil), files...)
	tampered[0].SHA256 = strings.Repeat("c", 64)
	if _, err := ValidateEvidence(raw, tampered, frontend); err == nil {
		t.Fatal("hash mismatch admitted")
	}
	if _, err := ValidateEvidence(raw, files[1:], frontend); err == nil || !strings.Contains(err.Error(), "not in the evidence directory") {
		t.Fatalf("V21 missing capture admitted: %v", err)
	}
	if _, err := ValidateEvidence(nil, files, frontend); err == nil {
		t.Fatal("missing manifest admitted")
	}
	// Two routes with the same pixels share one file (measured: a live run
	// of two routes serving one page); one route named twice is refused.
	same, sameFiles := validManifest(t, frontend, []byte("png-1"), []byte("png-1"))
	if _, err := ValidateEvidence(same, sameFiles[1:], frontend); err != nil {
		t.Fatalf("two routes sharing one capture refused: %v", err)
	}
	var twice Manifest
	_ = json.Unmarshal(same, &twice)
	twice.Captures[1].Route = twice.Captures[0].Route
	twiceRaw, _ := json.Marshal(twice)
	if _, err := ValidateEvidence(twiceRaw, sameFiles[1:], frontend); err == nil || !strings.Contains(err.Error(), "twice") {
		t.Fatalf("one route named twice admitted: %v", err)
	}
	var m Manifest
	_ = json.Unmarshal(raw, &m)
	m.Captures[0].FullPage = true
	full, _ := json.Marshal(m)
	if _, err := ValidateEvidence(full, files, frontend); err == nil {
		t.Fatal("V10 full-page capture admitted")
	}
	var unknown map[string]any
	_ = json.Unmarshal(raw, &unknown)
	unknown["extra"] = true
	withUnknown, _ := json.Marshal(unknown)
	if _, err := ValidateEvidence(withUnknown, files, frontend); err == nil {
		t.Fatal("manifest with an unknown field admitted")
	}
}

func TestCheckPNG(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 300, 200))
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	if blank, err := CheckPNG(buf.Bytes(), 300, 200); err != nil || !blank {
		t.Fatalf("uniform = %v, %v", blank, err)
	}
	img.Set(10, 10, color.RGBA{1, 2, 3, 255})
	buf.Reset()
	_ = png.Encode(&buf, img)
	if blank, err := CheckPNG(buf.Bytes(), 300, 200); err != nil || blank {
		t.Fatalf("drawn = %v, %v", blank, err)
	}
	if _, err := CheckPNG(buf.Bytes(), 300, 201); err == nil {
		t.Fatal("wrong size admitted")
	}
	if _, err := CheckPNG([]byte("GIF89a"), 300, 200); err == nil {
		t.Fatal("non-PNG admitted")
	}
}

func TestIsCaptureJob(t *testing.T) {
	if !IsCaptureJob([]string{"choir-capture", "--route", "/"}) || !IsCaptureJob([]string{"/run/current-system/sw/bin/choir-capture"}) {
		t.Fatal("capture job not recognized")
	}
	if IsCaptureJob([]string{"go", "build"}) || IsCaptureJob(nil) || IsCaptureJob([]string{"sh", "-c", "choir-capture"}) {
		t.Fatal("non-capture job recognized")
	}
}

func TestFrontendDigestIsOrderIndependent(t *testing.T) {
	a := []FileDigest{{"index.html", strings.Repeat("1", 64)}, {"assets/app.js", strings.Repeat("2", 64)}}
	b := []FileDigest{a[1], a[0]}
	if FrontendDigest(a) != FrontendDigest(b) {
		t.Fatal("digest depends on order")
	}
	b[0].SHA256 = strings.Repeat("3", 64)
	if FrontendDigest(a) == FrontendDigest(b) {
		t.Fatal("digest ignores content")
	}
}
