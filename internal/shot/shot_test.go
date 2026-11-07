package shot

import (
	"context"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
)

func goodPlan() capsule.ShotPlan {
	return capsule.ShotPlan{Views: []capsule.ShotView{
		{Name: "01-before-home", Path: "/", Viewport: "desktop"},
		{Name: "02-after-home", Path: "/", Viewport: "phone", Theme: "dark", Steps: []capsule.ShotStep{{Op: "click", Selector: "#b"}}},
	}}
}

func findBrowser() string {
	if b := os.Getenv(capsule.BrowserEnv); b != "" {
		return b
	}
	// On CI the PATH browser is the runner's own Chrome, not the pinned
	// Chromium a guest runs; it failed to start or to answer its debug socket
	// on loaded runners (three runs on 2026-11-07). CI names a browser through
	// the env var or skips.
	if os.Getenv("CI") != "" {
		return ""
	}
	for _, n := range []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable"} {
		if p, err := exec.LookPath(n); err == nil {
			return p
		}
	}
	mac := "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	if _, err := os.Stat(mac); err == nil {
		return mac
	}
	return ""
}

func findFfmpeg() string {
	if f := os.Getenv(capsule.FfmpegEnv); f != "" {
		return f
	}
	p, _ := exec.LookPath("ffmpeg")
	return p
}

const testPage = `<!doctype html><meta charset=utf-8><title>t</title>
<style>body{font:32px sans-serif;margin:0;padding:24px}
@media (prefers-color-scheme: dark){body{background:#123;color:#fff}}
#b{font-size:32px;padding:12px}</style>
<button id=b onclick="this.textContent='clicked';document.body.style.background='#fa0'">press me</button>
<p>shot driver test page</p>`

func TestRunEndToEnd(t *testing.T) {
	browser, ffmpeg := findBrowser(), findFfmpeg()
	if browser == "" || ffmpeg == "" {
		t.Skipf("no chromium/ffmpeg (browser=%q ffmpeg=%q); set CHOIR_BROWSER and CHOIR_FFMPEG", browser, ffmpeg)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(testPage))
	}))
	defer srv.Close()

	plan := goodPlan()
	plan.Views[1].Steps = []capsule.ShotStep{{Op: "wait_for", Selector: "#b"}, {Op: "click", Selector: "#b"}, {Op: "wait_ms", Ms: 200}}
	plan.Views = append(plan.Views, capsule.ShotView{Name: "03-click-home", Path: "/", Viewport: "desktop",
		Steps: []capsule.ShotStep{{Op: "click", Selector: "#b"}, {Op: "wait_ms", Ms: 200}}})
	plan.Video = &capsule.ShotVideo{View: "02-after-home", Seconds: 3}
	out := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	res, err := Run(ctx, Options{BaseURL: srv.URL, Plan: plan, OutDir: out, Browser: browser, Ffmpeg: ffmpeg, Timeouts: Timeouts{Plan: 100 * time.Second}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 4 {
		t.Fatalf("results: %+v", res)
	}
	for _, r := range res {
		t.Logf("artifact %s %s %d bytes %dx%d sha %s", r.Kind, r.Path, r.Bytes, r.Width, r.Height, r.SHA256[:12])
	}
	if res[0].Width != 1440 || res[0].Height != 900 {
		t.Errorf("desktop is %dx%d", res[0].Width, res[0].Height)
	}
	if res[1].Width != 390*3 || res[1].Height != 844*3 {
		t.Errorf("phone is %dx%d", res[1].Width, res[1].Height)
	}
	if res[0].SHA256 == res[1].SHA256 {
		t.Error("the two screens have one digest")
	}
	for _, r := range res[:2] {
		f, err := os.Open(r.Path)
		if err != nil {
			t.Fatal(err)
		}
		_, derr := png.Decode(f)
		f.Close()
		if derr != nil {
			t.Errorf("%s: %v", r.Path, derr)
		}
	}
	// The click changes the page: same viewport, distinct digest.
	if res[3].Name != "03-click-home" || res[3].SHA256 == res[0].SHA256 {
		t.Errorf("the click did not change the desktop shot: %+v", res[3])
	}
	raw, err := os.ReadFile(res[2].Path)
	if err != nil {
		t.Fatal(err)
	}
	if res[2].Kind != "webm" || !capsule.IsWebM(raw) || len(raw) > capsule.MaxVideoBytes {
		t.Errorf("video invalid: %+v", res[2])
	}
	// The 3 s video is trimmed to at most Seconds (+1 s of container slack).
	if _, err := exec.LookPath("ffprobe"); err == nil {
		pctx, pcancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer pcancel()
		o, _ := exec.CommandContext(pctx, "ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", res[2].Path).Output()
		d, perr := strconv.ParseFloat(strings.TrimSpace(string(o)), 64)
		if perr != nil || d <= 0 || d > 4 {
			t.Errorf("video duration %q (%v); want 0 < d <= 4", o, perr)
		}
		t.Logf("video duration %v s", d)
	}
}

func TestPressKeysCoverShotKeys(t *testing.T) {
	for _, k := range capsule.ShotKeys {
		if _, err := keyFor(k); err != nil {
			t.Errorf("%s: %v", k, err)
		}
	}
	if _, err := keyFor("F13"); err == nil {
		t.Error("unknown key accepted")
	}
}

func TestRunFailedStepNamesViewAndLeavesNothing(t *testing.T) {
	browser := findBrowser()
	if browser == "" {
		t.Skip("no chromium; set CHOIR_BROWSER")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(testPage))
	}))
	defer srv.Close()
	plan := goodPlan()
	plan.Views[1].Steps = []capsule.ShotStep{{Op: "click", Selector: "#missing", Ms: 500}}
	out := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	_, err := Run(ctx, Options{BaseURL: srv.URL, Plan: plan, OutDir: out, Browser: browser})
	if err == nil || !strings.Contains(err.Error(), "02-after-home") || !strings.Contains(err.Error(), "step 0") {
		t.Fatalf("want a view/step error, got %v", err)
	}
	if ents, _ := os.ReadDir(out); len(ents) != 0 {
		t.Errorf("partial output left: %v", ents)
	}
}
