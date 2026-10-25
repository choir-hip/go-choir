package visualcapture

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ErrNotGranted is a capture on a computer whose grant is off (V1).
var ErrNotGranted = errors.New("visual capture is not granted on this computer (no " + BrowserEnv + " in the job environment; the host grants it per computer)")

// Options is one capture run. The roots are fixed in the capsule; tests
// point them elsewhere.
type Options struct {
	Routes     []string
	Width      int
	Height     int
	WaitMS     int
	Browser    string // the Chromium binary (BrowserEnv)
	Fontconfig string // FONTCONFIG_FILE for Chromium (FontconfigEnv)
	ServeRoot  string
	OutputDir  string
	// ShotDeadline bounds one Chromium run; zero means ShotDeadline.
	ShotDeadline time.Duration
	// Now stamps captures; nil means time.Now.
	Now func() time.Time
}

// Normalize fills the defaults and checks the bounds (V10).
func (o Options) Normalize() (Options, error) {
	if strings.TrimSpace(o.Browser) == "" {
		return o, ErrNotGranted
	}
	if len(o.Routes) == 0 {
		o.Routes = []string{"/"}
	}
	if len(o.Routes) > MaxRoutes {
		return o, fmt.Errorf("%d routes; at most %d per capture", len(o.Routes), MaxRoutes)
	}
	seen := map[string]bool{}
	for i, r := range o.Routes {
		clean, err := NormalizeRoute(r)
		if err != nil {
			return o, err
		}
		if seen[clean] {
			return o, fmt.Errorf("route %s is named twice", clean)
		}
		seen[clean] = true
		o.Routes[i] = clean
	}
	if o.Width == 0 {
		o.Width = DefaultWidth
	}
	if o.Height == 0 {
		o.Height = DefaultHeight
	}
	if o.Width < MinDimension || o.Width > MaxWidth || o.Height < MinDimension || o.Height > MaxHeight {
		return o, fmt.Errorf("viewport %dx%d is outside %dx%d..%dx%d (viewport captures only; no full page)", o.Width, o.Height, MinDimension, MinDimension, MaxWidth, MaxHeight)
	}
	if o.WaitMS == 0 {
		o.WaitMS = DefaultWaitMS
	}
	if o.WaitMS < 0 || o.WaitMS > MaxWaitMS {
		return o, fmt.Errorf("wait %d ms is outside 0..%d", o.WaitMS, MaxWaitMS)
	}
	if o.ServeRoot == "" {
		o.ServeRoot = ServeRoot
	}
	if o.OutputDir == "" {
		o.OutputDir = OutputDir
	}
	if o.ShotDeadline <= 0 {
		o.ShotDeadline = ShotDeadline
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return o, nil
}

// Run captures every route and writes the PNGs and the manifest into the
// output directory, replacing whatever an earlier run left there. The
// manifest is written last, by rename (V21).
func Run(ctx context.Context, opts Options) (Manifest, error) {
	res, err := RunReport(ctx, opts)
	return res.Manifest, err
}

// Report is a capture run: the manifest it wrote, and the asset requests
// its server could not serve, which are advice and never in the manifest.
type Report struct {
	Manifest      Manifest
	AssetFailures []AssetFailure
	// AssetDropped counts failed answers past the kept paths.
	AssetDropped int
}

// RunReport is Run with the run's failed asset answers (fix 3, trace review
// round 4).
func RunReport(ctx context.Context, opts Options) (Report, error) {
	// The server has shut down when captureRoutes returns, so the record
	// is complete.
	m, recorder, err := captureRoutes(ctx, opts)
	res := Report{Manifest: m}
	if recorder != nil {
		res.AssetFailures, res.AssetDropped = recorder.Failures(), recorder.Dropped()
	}
	return res, err
}

func captureRoutes(ctx context.Context, opts Options) (Manifest, *AssetRecorder, error) {
	opts, err := opts.Normalize()
	if err != nil {
		return Manifest{}, nil, err
	}
	info, err := os.Stat(filepath.Join(opts.ServeRoot, "index.html"))
	if err != nil || !info.Mode().IsRegular() {
		return Manifest{}, nil, fmt.Errorf("no built frontend at %s (build the frontend into the release tree first)", opts.ServeRoot)
	}
	files, err := digestTree(opts.ServeRoot)
	if err != nil {
		return Manifest{}, nil, err
	}
	if err := resetOutput(opts.OutputDir); err != nil {
		return Manifest{}, nil, err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return Manifest{}, nil, fmt.Errorf("serve the frontend on loopback: %w (capture needs the capsule's loopback network: a capsule with the ecosystem network)", err)
	}
	recorder := NewAssetRecorder()
	srv := &http.Server{Handler: recorder.Wrap(Server(opts.ServeRoot)), ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(listener) }()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	base := "http://" + listener.Addr().String()
	scratch, err := os.MkdirTemp("", "choir-capture-")
	if err != nil {
		return Manifest{}, recorder, err
	}
	defer os.RemoveAll(scratch)

	m := Manifest{
		Kind:    ManifestKind,
		Subject: Subject{FrontendSHA256: FrontendDigest(files), FrontendFiles: len(files)},
		Browser: Browser{Name: "chromium", Path: opts.Browser, Version: browserVersion(ctx, opts)},
		WaitMS:  opts.WaitMS,
	}
	var total int64
	for i, route := range opts.Routes {
		shot := filepath.Join(scratch, fmt.Sprintf("shot-%d.png", i))
		profile := filepath.Join(scratch, fmt.Sprintf("profile-%d", i))
		if err := os.MkdirAll(profile, 0o700); err != nil {
			return Manifest{}, recorder, err
		}
		args := BrowserArgs(base+route, shot, profile, opts.Width, opts.Height, opts.WaitMS)
		if err := runShot(ctx, opts, args, profile, shot); err != nil {
			return Manifest{}, recorder, fmt.Errorf("capture %s: %w", route, err)
		}
		data, err := os.ReadFile(shot)
		if err != nil {
			return Manifest{}, recorder, fmt.Errorf("capture %s: Chromium wrote no screenshot: %w", route, err)
		}
		blank, err := CheckPNG(data, opts.Width, opts.Height)
		if err != nil {
			return Manifest{}, recorder, fmt.Errorf("capture %s: %w", route, err)
		}
		total += int64(len(data))
		if total > MaxTotalBytes {
			return Manifest{}, recorder, fmt.Errorf("captures exceed %d bytes", MaxTotalBytes)
		}
		sum := sha256.Sum256(data)
		digest := hex.EncodeToString(sum[:])
		name := digest + ".png"
		if err := writeAtomic(filepath.Join(opts.OutputDir, name), data); err != nil {
			return Manifest{}, recorder, err
		}
		m.Captures = append(m.Captures, Capture{
			Artifact: ArtifactRef(digest), File: name, MediaType: MediaTypePNG, Bytes: int64(len(data)),
			Route: route, Width: opts.Width, Height: opts.Height, ReducedMotion: true, Blank: blank,
			TakenAt: opts.Now().UTC().Format(time.RFC3339),
		})
	}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return Manifest{}, recorder, err
	}
	if err := writeAtomic(filepath.Join(opts.OutputDir, ManifestFile), append(raw, '\n')); err != nil {
		return Manifest{}, recorder, err
	}
	return m, recorder, nil
}

// BrowserArgs is one headless Chromium screenshot run: new headless, no
// sandbox (the capsule is the sandbox), no GPU, no network beyond loopback
// (V7), reduced motion (V9), a fixed viewport, and a virtual time budget in
// place of a ready-element wait.
func BrowserArgs(url, out, profile string, width, height, waitMS int) []string {
	return []string{
		"--headless=new",
		"--no-sandbox",
		"--disable-gpu",
		"--disable-dev-shm-usage",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-extensions",
		"--disable-background-networking",
		"--disable-component-update",
		"--disable-sync",
		"--disable-default-apps",
		"--disable-breakpad",
		"--disable-crash-reporter",
		"--mute-audio",
		"--hide-scrollbars",
		"--force-prefers-reduced-motion",
		"--force-device-scale-factor=1",
		"--no-proxy-server",
		"--host-resolver-rules=MAP * ~NOTFOUND , EXCLUDE 127.0.0.1",
		"--user-data-dir=" + profile,
		fmt.Sprintf("--window-size=%d,%d", width, height),
		fmt.Sprintf("--virtual-time-budget=%d", waitMS),
		"--screenshot=" + out,
		url,
	}
}

// browserEnv is Chromium's environment: PATH and LANG from ours, a private
// HOME and XDG dirs in the shot's profile, the fontconfig file, and no proxy
// variables (V7).
func browserEnv(opts Options, profile string) []string {
	env := []string{
		"HOME=" + profile,
		"XDG_CONFIG_HOME=" + filepath.Join(profile, "config"),
		"XDG_CACHE_HOME=" + filepath.Join(profile, "cache"),
		"TMPDIR=" + os.TempDir(),
		"LANG=C.UTF-8",
	}
	if p := os.Getenv("PATH"); p != "" {
		env = append(env, "PATH="+p)
	}
	if opts.Fontconfig != "" {
		env = append(env, "FONTCONFIG_FILE="+opts.Fontconfig)
	}
	return append(env, testBrowserEnv...)
}

// testBrowserEnv lets a test's fake Chromium receive its switches.
var testBrowserEnv []string

func runShot(ctx context.Context, opts Options, args []string, profile, out string) error {
	shotCtx, cancel := context.WithTimeout(ctx, opts.ShotDeadline)
	defer cancel()
	cmd := exec.Command(opts.Browser, args...)
	cmd.Env = browserEnv(opts, profile)
	// Chromium's output goes to a file, not a pipe: a helper that outlives
	// the browser would hold a pipe open and Wait with it (measured with
	// Chrome's helpers on macOS: the shot finished, Wait hung to the
	// deadline).
	logFile, err := os.CreateTemp(profile, "chromium-*.log")
	if err != nil {
		return err
	}
	defer logFile.Close()
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	setProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start Chromium %s: %w", opts.Browser, err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	// The shot is done when its PNG decodes, not when Chromium exits:
	// teardown can outlast the shot by tens of seconds (measured: Chrome on
	// macOS wrote the PNG, then its teardown watchdog fired 20 s later).
	poll := time.NewTicker(200 * time.Millisecond)
	defer poll.Stop()
	for {
		select {
		case err := <-done:
			killProcessGroup(cmd) // V6: whatever Chromium left in its group
			if pngComplete(out) {
				return nil
			}
			if err != nil {
				return fmt.Errorf("Chromium exited: %v; output tail: %s", err, fileTail(logFile.Name()))
			}
			return nil // the caller reports the missing screenshot
		case <-poll.C:
			if pngComplete(out) {
				killProcessGroup(cmd)
				<-done
				return nil
			}
		case <-shotCtx.Done():
			killProcessGroup(cmd)
			<-done
			return fmt.Errorf("Chromium did not finish within %s and was killed; output tail: %s", opts.ShotDeadline, fileTail(logFile.Name()))
		}
	}
}

// pngComplete reports whether path holds a whole PNG (it decodes).
func pngComplete(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return false
	}
	_, err = png.Decode(bytes.NewReader(data))
	return err == nil
}

// fileTail is the last 1 KiB of a file, trimmed.
func fileTail(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if len(data) > 1024 {
		data = data[len(data)-1024:]
	}
	return strings.TrimSpace(string(data))
}

func browserVersion(ctx context.Context, opts Options) string {
	vctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(vctx, opts.Browser, "--version")
	cmd.Env = browserEnv(opts, os.TempDir())
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	v := strings.TrimSpace(string(out))
	if len(v) > 120 {
		v = v[:120]
	}
	return v
}

// Server serves a built frontend: files under root, index.html for any
// other path without an extension (an SPA route), 404 for a missing asset,
// and a signed-out backend at once (V16): /auth/session answers 200
// {"authenticated":false}, as the auth service does for a request with no
// session cookie, other /auth/ routes 401, and /api/ 503. The app shell
// reads a 5xx from /auth/session as a transient auth failure and stays on
// Loading, then Reconnecting (frontend/src/lib/auth.js requestSessionState,
// App.svelte checkSession), so only the signed-out answer lets a capture
// show the signed-out desktop and its public-preview apps (problem 6,
// docs/evidence/selfdev-trace-reviews-2026-10-25.md). Nothing outside root
// is served: paths are cleaned and symlinks are not followed out.
func Server(root string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		p := r.URL.Path
		if p == "/auth/session" {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			_, _ = io.WriteString(w, `{"authenticated":false}`)
			return
		}
		if p == "/auth" || strings.HasPrefix(p, "/auth/") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"error":"not signed in: this is a capture of the static frontend"}`)
			return
		}
		if p == "/api" || strings.HasPrefix(p, "/api/") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, `{"error":"no backend: this is a capture of the static frontend"}`)
			return
		}
		rel := cleanRel(p)
		if rel != "" {
			if full, ok := regularUnder(root, rel); ok {
				http.ServeFile(w, r, full)
				return
			}
			if filepath.Ext(rel) != "" {
				http.NotFound(w, r)
				return
			}
		}
		if full, ok := regularUnder(root, "index.html"); ok {
			http.ServeFile(w, r, full)
			return
		}
		http.NotFound(w, r)
	})
}

// regularUnder resolves rel under root and admits it only when the resolved
// path is a regular file still under root.
func regularUnder(root, rel string) (string, bool) {
	full := filepath.Join(root, filepath.FromSlash(rel))
	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		return "", false
	}
	rootResolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", false
	}
	if resolved != rootResolved && !strings.HasPrefix(resolved, rootResolved+string(filepath.Separator)) {
		return "", false
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	return resolved, true
}

// digestTree hashes every regular file under root (symlinks and other
// non-regular entries are skipped, as Freeze stages only regular files).
func digestTree(root string) ([]FileDigest, error) {
	var out []FileDigest
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		h := sha256.New()
		_, err = io.Copy(h, f)
		_ = f.Close()
		if err != nil {
			return err
		}
		out = append(out, FileDigest{Path: filepath.ToSlash(rel), SHA256: hex.EncodeToString(h.Sum(nil))})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("digest the frontend tree: %w", err)
	}
	return out, nil
}

// resetOutput empties the output directory: the tool owns it, and Freeze
// refuses files the manifest does not name.
func resetOutput(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
