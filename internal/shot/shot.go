package shot

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"image/png"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"

	"github.com/yusefmosiah/go-choir/internal/capsule"
)

// Timeouts bounds a run. Zero fields take the defaults.
type Timeouts struct {
	View   time.Duration // one view, steps and capture (default 60s)
	Plan   time.Duration // the whole run (default 5m)
	Encode time.Duration // ffmpeg (default 60s)
}

func (t Timeouts) withDefaults() Timeouts {
	if t.View <= 0 {
		t.View = 60 * time.Second
	}
	if t.Plan <= 0 {
		t.Plan = 5 * time.Minute
	}
	if t.Encode <= 0 {
		t.Encode = 60 * time.Second
	}
	return t
}

// Options configure Run.
type Options struct {
	// BaseURL is the origin the plan's paths join to (http://127.0.0.1:port).
	BaseURL string
	Plan    capsule.ShotPlan
	// OutDir receives <Name>.png and the video; it is created.
	OutDir string
	// Browser is the Chromium binary; empty reads CHOIR_BROWSER.
	Browser string
	// Ffmpeg is the ffmpeg binary; empty reads CHOIR_FFMPEG. Needed only for video.
	Ffmpeg string
	// Fontconfig is FONTCONFIG_FILE for the browser; empty keeps the process's own.
	Fontconfig string
	Timeouts   Timeouts
}

// Result is one written file.
type Result struct {
	Name   string // file stem
	Kind   string // "png" or "webm"
	Path   string
	SHA256 string
	Bytes  int64
	Width  int // 0 for the video
	Height int
}

type viewport struct {
	w, h int64
	dpr  float64
}

func viewportFor(name string) viewport {
	if name == capsule.ShotViewportPhone {
		return viewport{390, 844, 3}
	}
	return viewport{1440, 900, 1}
}

var pressKeys = map[string]string{
	"Enter": kb.Enter, "Tab": kb.Tab, "Escape": kb.Escape, "Backspace": kb.Backspace,
	"ArrowDown": kb.ArrowDown, "ArrowUp": kb.ArrowUp, "ArrowLeft": kb.ArrowLeft, "ArrowRight": kb.ArrowRight,
	"PageDown": kb.PageDown, "PageUp": kb.PageUp, "Home": kb.Home, "End": kb.End,
}

func keyFor(name string) (string, bool) { k, ok := pressKeys[name]; return k, ok }

// Run shoots the plan and returns the files it wrote, in plan order (the
// video follows its view). On any error it removes what it wrote.
func Run(ctx context.Context, opts Options) (results []Result, err error) {
	if err := opts.Plan.Validate(); err != nil {
		return nil, err
	}
	base, err := url.Parse(opts.BaseURL)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return nil, fmt.Errorf("shot: base url %q is not an http origin", opts.BaseURL)
	}
	browser := firstNonEmpty(opts.Browser, os.Getenv(capsule.BrowserEnv))
	if browser == "" {
		return nil, fmt.Errorf("shot: no browser (set %s or Options.Browser)", capsule.BrowserEnv)
	}
	ffmpeg := ""
	if opts.Plan.Video != nil {
		if ffmpeg = firstNonEmpty(opts.Ffmpeg, os.Getenv(capsule.FfmpegEnv)); ffmpeg == "" {
			return nil, fmt.Errorf("shot: the plan has a video and no ffmpeg (set %s or Options.Ffmpeg)", capsule.FfmpegEnv)
		}
	}
	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return nil, err
	}
	to := opts.Timeouts.withDefaults()
	ctx, cancel := context.WithTimeout(ctx, to.Plan)
	defer cancel()
	defer func() {
		if err != nil {
			for _, r := range results {
				_ = os.Remove(r.Path)
			}
			results = nil
		}
	}()

	env := os.Environ()
	if opts.Fontconfig != "" {
		env = append(env, "FONTCONFIG_FILE="+opts.Fontconfig)
	}
	allocOpts := []chromedp.ExecAllocatorOption{
		chromedp.ExecPath(browser),
		chromedp.Flag("headless", "new"),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("hide-scrollbars", true),
		chromedp.Flag("force-color-profile", "srgb"),
		chromedp.Flag("font-render-hinting", "none"),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
		chromedp.ModifyCmdFunc(func(c *exec.Cmd) { c.Env = env }),
	}
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, allocOpts...)
	defer allocCancel()
	// The browser starts with the first context; keep it alive for the run.
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()
	if err := chromedp.Run(browserCtx); err != nil {
		return nil, fmt.Errorf("shot: browser did not start: %w", err)
	}

	for _, v := range opts.Plan.Views {
		var rec *recorder
		if vd := opts.Plan.Video; vd != nil && vd.View == v.Name {
			rec = &recorder{minSeconds: vd.Seconds}
		}
		res, verr := shootView(browserCtx, base, v, opts.OutDir, rec, to.View)
		if verr != nil {
			if rec != nil && rec.dir != "" {
				_ = os.RemoveAll(rec.dir)
			}
			return results, fmt.Errorf("shot: view %s: %w", v.Name, verr)
		}
		results = append(results, res)
		if rec != nil {
			vres, verr := encodeVideo(ctx, ffmpeg, rec, opts.Plan.Video.FileName(), opts.OutDir, to.Encode)
			if verr != nil {
				return results, fmt.Errorf("shot: video %s: %w", v.Name, verr)
			}
			results = append(results, vres)
		}
	}
	return results, nil
}

func firstNonEmpty(a ...string) string {
	for _, s := range a {
		if s = strings.TrimSpace(s); s != "" {
			return s
		}
	}
	return ""
}

// frame is one screencast JPEG on disk with its arrival time.
type frame struct {
	path string
	at   time.Time
}

type recorder struct {
	minSeconds int
	dir        string
	mu         sync.Mutex
	seq        int
	frames     []frame
	start      time.Time
}

func (r *recorder) add(data []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p := filepath.Join(r.dir, fmt.Sprintf("f%05d.jpg", r.seq))
	r.seq++
	if os.WriteFile(p, data, 0o600) == nil {
		r.frames = append(r.frames, frame{path: p, at: time.Now()})
	}
}

func shootView(parent context.Context, base *url.URL, v capsule.ShotView, outDir string, rec *recorder, timeout time.Duration) (Result, error) {
	// One fresh page (target) per view.
	vctx, cancel := chromedp.NewContext(parent)
	defer cancel()
	vctx, cancelT := context.WithTimeout(vctx, timeout)
	defer cancelT()

	vp := viewportFor(v.Viewport)
	setup := chromedp.Tasks{
		emulation.SetDeviceMetricsOverride(vp.w, vp.h, vp.dpr, v.Viewport == capsule.ShotViewportPhone),
	}
	if v.Theme != "" {
		setup = append(setup, emulation.SetEmulatedMedia().WithFeatures([]*emulation.MediaFeature{{Name: "prefers-color-scheme", Value: v.Theme}}))
	}
	if err := chromedp.Run(vctx, setup); err != nil {
		return Result{}, fmt.Errorf("setup: %w", err)
	}

	if rec != nil {
		dir, err := os.MkdirTemp("", "shot-frames-")
		if err != nil {
			return Result{}, err
		}
		rec.dir = dir
		chromedp.ListenTarget(vctx, func(ev any) {
			fe, ok := ev.(*page.EventScreencastFrame)
			if !ok {
				return
			}
			raw, id := fe.Data, fe.SessionID
			go func() { // never block the event loop
				if data, err := base64.StdEncoding.DecodeString(raw); err == nil {
					rec.add(data)
				}
				_ = chromedp.Run(vctx, page.ScreencastFrameAck(id))
			}()
		})
		err = chromedp.Run(vctx, page.StartScreencast().WithFormat(page.ScreencastFormatJpeg).WithQuality(80).WithEveryNthFrame(2).WithMaxWidth(1280))
		if err != nil {
			return Result{}, fmt.Errorf("start screencast: %w", err)
		}
		rec.start = time.Now()
	}

	// Validate guarantees Path starts with a single "/", so the host is the base's.
	target := base.Scheme + "://" + base.Host + v.Path
	if err := chromedp.Run(vctx, chromedp.Navigate(target)); err != nil {
		return Result{}, fmt.Errorf("navigate %s: %w", target, err)
	}
	for i, s := range v.Steps {
		if err := runStep(vctx, s); err != nil {
			return Result{}, fmt.Errorf("step %d (%s): %w", i, s.Op, err)
		}
	}

	var buf []byte
	if err := chromedp.Run(vctx, chromedp.ActionFunc(func(c context.Context) (err error) {
		buf, err = page.CaptureScreenshot().WithFormat(page.CaptureScreenshotFormatPng).Do(c)
		return err
	})); err != nil {
		return Result{}, fmt.Errorf("capture: %w", err)
	}

	if rec != nil {
		if hold := time.Duration(rec.minSeconds)*time.Second - time.Since(rec.start); hold > 0 {
			if err := chromedp.Run(vctx, chromedp.Sleep(hold)); err != nil {
				return Result{}, fmt.Errorf("record hold: %w", err)
			}
		}
		if err := chromedp.Run(vctx, page.StopScreencast()); err != nil {
			return Result{}, fmt.Errorf("stop screencast: %w", err)
		}
		_ = chromedp.Run(vctx, chromedp.Sleep(200*time.Millisecond)) // frames in flight
	}

	cfg, err := png.DecodeConfig(bytes.NewReader(buf))
	if err != nil {
		return Result{}, fmt.Errorf("capture is not a PNG: %w", err)
	}
	if len(buf) > capsule.MaxScreenBytes || cfg.Width*cfg.Height > capsule.MaxScreenPixels {
		return Result{}, fmt.Errorf("capture %dx%d, %d bytes exceeds the screen bounds", cfg.Width, cfg.Height, len(buf))
	}
	p := filepath.Join(outDir, v.Name+".png")
	if err := os.WriteFile(p, buf, 0o644); err != nil {
		return Result{}, err
	}
	sum := sha256.Sum256(buf)
	return Result{Name: v.Name, Kind: "png", Path: p, SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(buf)), Width: cfg.Width, Height: cfg.Height}, nil
}

func runStep(ctx context.Context, s capsule.ShotStep) error {
	if s.Op == capsule.ShotOpWaitMs {
		return chromedp.Run(ctx, chromedp.Sleep(time.Duration(s.Ms)*time.Millisecond))
	}
	sctx, cancel := context.WithTimeout(ctx, s.Timeout())
	defer cancel()
	var a chromedp.Action
	switch s.Op {
	case capsule.ShotOpClick:
		a = chromedp.Tasks{chromedp.WaitVisible(s.Selector, chromedp.ByQuery), chromedp.Click(s.Selector, chromedp.ByQuery)}
	case capsule.ShotOpFill:
		a = chromedp.Tasks{chromedp.WaitVisible(s.Selector, chromedp.ByQuery), chromedp.Clear(s.Selector, chromedp.ByQuery), chromedp.SendKeys(s.Selector, s.Text, chromedp.ByQuery)}
	case capsule.ShotOpPress:
		k, _ := keyFor(s.Text)
		a = chromedp.KeyEvent(k)
	case capsule.ShotOpWaitFor:
		a = chromedp.WaitVisible(s.Selector, chromedp.ByQuery)
	case capsule.ShotOpScroll:
		switch {
		case s.Selector != "":
			a = chromedp.ScrollIntoView(s.Selector, chromedp.ByQuery)
		case s.Text == "top":
			a = chromedp.Evaluate(`window.scrollTo(0,0)`, nil)
		default:
			a = chromedp.Evaluate(`window.scrollTo(0,document.documentElement.scrollHeight)`, nil)
		}
	default:
		return fmt.Errorf("unknown op %q", s.Op)
	}
	if err := chromedp.Run(sctx, a); err != nil {
		if errors.Is(err, context.DeadlineExceeded) || sctx.Err() != nil {
			return fmt.Errorf("%s %q timed out after %v", s.Op, s.Selector, s.Timeout())
		}
		return fmt.Errorf("%s %q: %w", s.Op, s.Selector, err)
	}
	return nil
}

// encodeVideo turns the recorded JPEG frames into a VP8 WebM at 10 fps, each
// frame held for its real display time, and checks it with the capsule's
// video checks. The scratch frames are removed.
func encodeVideo(ctx context.Context, ffmpeg string, rec *recorder, fileName, outDir string, timeout time.Duration) (Result, error) {
	defer os.RemoveAll(rec.dir)
	rec.mu.Lock()
	frames := append([]frame(nil), rec.frames...)
	rec.mu.Unlock()
	if len(frames) == 0 {
		return Result{}, fmt.Errorf("the screencast produced no frames")
	}
	end := rec.start.Add(time.Duration(rec.minSeconds) * time.Second)
	if last := frames[len(frames)-1].at; last.After(end) {
		end = last.Add(100 * time.Millisecond)
	}
	var list strings.Builder
	for i, f := range frames {
		next := end
		if i+1 < len(frames) {
			next = frames[i+1].at
		}
		d := next.Sub(f.at)
		if d < time.Millisecond {
			d = time.Millisecond
		}
		fmt.Fprintf(&list, "file '%s'\nduration %.3f\n", f.path, d.Seconds())
	}
	fmt.Fprintf(&list, "file '%s'\n", frames[len(frames)-1].path) // concat ignores the last duration
	listPath := filepath.Join(rec.dir, "frames.txt")
	if err := os.WriteFile(listPath, []byte(list.String()), 0o600); err != nil {
		return Result{}, err
	}
	out := filepath.Join(outDir, fileName)
	ectx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ectx, ffmpeg, "-hide_banner", "-loglevel", "error", "-y",
		"-f", "concat", "-safe", "0", "-i", listPath,
		"-vf", "fps=10,scale=trunc(iw/2)*2:trunc(ih/2)*2",
		"-c:v", "libvpx", "-b:v", "800k", "-crf", "32", "-an", "-t", fmt.Sprint(capsule.MaxShotVideoSeconds), out)
	if msg, err := cmd.CombinedOutput(); err != nil {
		_ = os.Remove(out)
		return Result{}, fmt.Errorf("ffmpeg: %w: %s", err, tail(string(msg), 1500))
	}
	data, err := os.ReadFile(out)
	if err != nil {
		return Result{}, err
	}
	if len(data) == 0 || len(data) > capsule.MaxVideoBytes {
		_ = os.Remove(out)
		return Result{}, fmt.Errorf("video is %d bytes; admitted 1..%d", len(data), capsule.MaxVideoBytes)
	}
	if !capsule.IsWebM(data) {
		_ = os.Remove(out)
		return Result{}, fmt.Errorf("ffmpeg output is not a WebM")
	}
	sum := sha256.Sum256(data)
	return Result{Name: strings.TrimSuffix(fileName, ".webm"), Kind: "webm", Path: out, SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(data))}, nil
}

func tail(s string, n int) string {
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}
