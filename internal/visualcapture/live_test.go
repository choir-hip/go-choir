package visualcapture

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
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
