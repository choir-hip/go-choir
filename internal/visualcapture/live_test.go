package visualcapture

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestLiveChromiumCapture runs a real Chromium (CHOIR_CAPTURE_LIVE_BROWSER)
// over a fixture page and keeps the evidence in CHOIR_CAPTURE_LIVE_OUT when
// set. It proves the flags produce an exact-viewport PNG; it does not prove
// the capsule (seccomp, Landlock, mounts), which only a staging run can.
func TestLiveChromiumCapture(t *testing.T) {
	browser := os.Getenv("CHOIR_CAPTURE_LIVE_BROWSER")
	if browser == "" {
		t.Skip("set CHOIR_CAPTURE_LIVE_BROWSER to a Chromium binary")
	}
	root := t.TempDir()
	serve := filepath.Join(root, "frontend")
	if err := os.MkdirAll(filepath.Join(serve, "choir-drill"), 0o755); err != nil {
		t.Fatal(err)
	}
	page := `<!doctype html><html><head><style>body{font-family:sans-serif;margin:40px}h1{font-size:64px}
@media (prefers-reduced-motion: no-preference){h1{animation:spin 2s infinite}}@keyframes spin{to{transform:rotate(360deg)}}</style></head>
<body><h1 id="drill">choir-drill-live</h1><p>visual evidence slice</p></body></html>`
	_ = os.WriteFile(filepath.Join(serve, "index.html"), []byte(page), 0o644)
	_ = os.WriteFile(filepath.Join(serve, "choir-drill", "live.html"), []byte(page), 0o644)
	if runtime.GOOS == "darwin" {
		// Chrome on macOS stalls before the shot with a private HOME (its
		// keychain and updater live under the real one); Linux Chromium
		// takes the private HOME the capsule gives it.
		prev := testBrowserEnv
		testBrowserEnv = []string{"HOME=" + os.Getenv("HOME")}
		t.Cleanup(func() { testBrowserEnv = prev })
	}
	out := os.Getenv("CHOIR_CAPTURE_LIVE_OUT")
	if out == "" {
		out = filepath.Join(root, "evidence")
	}
	m, err := Run(context.Background(), Options{
		Routes: []string{"/", "/choir-drill/live.html"}, Browser: browser,
		Fontconfig: os.Getenv("CHOIR_CAPTURE_LIVE_FONTCONFIG"), ServeRoot: serve, OutputDir: out,
	})
	if err != nil {
		t.Fatalf("live capture: %v", err)
	}
	for _, c := range m.Captures {
		if c.Blank || c.Width != DefaultWidth || c.Height != DefaultHeight {
			t.Fatalf("capture %+v", c)
		}
		t.Logf("%s %s %d bytes (%s)", c.Route, c.Artifact, c.Bytes, m.Browser.Version)
	}
	raw, err := os.ReadFile(filepath.Join(out, ManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	tree, err := digestTree(serve)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateEvidence(raw, evidenceFiles(t, out), FrontendDigest(tree)); err != nil {
		t.Fatalf("live evidence does not validate: %v", err)
	}
}

// TestLiveCaptureOfTheBuiltFrontendRendersPastLoading serves a built Choir
// frontend (CHOIR_CAPTURE_LIVE_FRONTEND, a vite dist directory) through the
// capture server and reads the DOM a real Chromium (CHOIR_CAPTURE_LIVE_BROWSER)
// renders under the same virtual time budget a capture gets. The app shell
// checks /auth/session first: the capture must answer it as a signed-out
// session does, so the page leaves Loading for the signed-out desktop, where
// public-preview apps render (problem 6, docs/evidence/selfdev-trace-reviews-
// 2026-10-25.md). A 503 there is a transient auth error and leaves the app on
// Loading, then Reconnecting.
func TestLiveCaptureOfTheBuiltFrontendRendersPastLoading(t *testing.T) {
	browser := os.Getenv("CHOIR_CAPTURE_LIVE_BROWSER")
	frontend := os.Getenv("CHOIR_CAPTURE_LIVE_FRONTEND")
	if browser == "" || frontend == "" {
		t.Skip("set CHOIR_CAPTURE_LIVE_BROWSER to a Chromium binary and CHOIR_CAPTURE_LIVE_FRONTEND to a built frontend")
	}
	srv := httptest.NewServer(Server(frontend))
	defer srv.Close()
	profile := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, browser, "--headless=new", "--no-sandbox", "--disable-gpu", "--no-first-run",
		"--disable-extensions", "--user-data-dir="+profile, fmt.Sprintf("--virtual-time-budget=%d", DefaultWaitMS), "--dump-dom", srv.URL+"/")
	if runtime.GOOS == "darwin" {
		cmd.Env = append(os.Environ(), "HOME="+os.Getenv("HOME"))
	}
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("chromium --dump-dom: %v", err)
	}
	dom := string(out)
	state := ""
	if i := strings.Index(dom, `data-auth-state="`); i >= 0 {
		rest := dom[i+len(`data-auth-state="`):]
		state, _, _ = strings.Cut(rest, `"`)
	}
	if state != "signed_out" {
		t.Fatalf("the captured app shell is in auth state %q, not signed_out (Loading/Reconnecting means the capture's /auth/session answer reads as a transient auth failure)", state)
	}
	if !strings.Contains(dom, `data-authenticated="false"`) {
		t.Fatalf("the signed-out desktop did not render: %.400s", dom)
	}
}
