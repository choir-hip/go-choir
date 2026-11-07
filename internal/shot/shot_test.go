package shot

import (
	"context"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
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
	plan.Video = &capsule.ShotVideo{View: "02-after-home", Seconds: 3}
	out := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	res, err := Run(ctx, Options{BaseURL: srv.URL, Plan: plan, OutDir: out, Browser: browser, Ffmpeg: ffmpeg, Timeouts: Timeouts{Plan: 100 * time.Second}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 3 {
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
	raw, err := os.ReadFile(res[2].Path)
	if err != nil {
		t.Fatal(err)
	}
	if res[2].Kind != "webm" || !capsule.IsWebM(raw) || len(raw) > capsule.MaxVideoBytes {
		t.Errorf("video invalid: %+v", res[2])
	}
	// The phone shot is after the click: the page must show it (a dark theme
	// or the click colour); the desktop shot did not click.
	if _, err := exec.LookPath("ffprobe"); err == nil {
		o, _ := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", res[2].Path).Output()
		t.Logf("video duration %s", strings.TrimSpace(string(o)))
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
