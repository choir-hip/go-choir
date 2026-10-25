package visualcapture

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Fix 3 of trace review round 4 (docs/evidence/selfdev-trace-reviews-
// 2026-10-25.md, D1): /read.html imported ./pdf-text-*.js, which resolved
// to a 404 outside /assets/, so the module never ran and the capture showed
// an empty page. Chromium prints nothing for a failed module load; only the
// capture server saw it. The server now records failed asset answers and the
// capture names them as advice.
//
// Ways this can fail, written before the code:
//  1. a missing chunk's 404 is not recorded (the status is not observed);
//  2. the intended /api/ 503 and /auth/ 401 are reported as failures;
//  3. a page route (the SPA fallback, an .html file) is reported;
//  4. an unbounded list pushes the manifest digest out of the job's 4 KiB
//     tail, so Freeze finds no capture receipt;
//  5. the manifest changes (format or digest) when an asset fails;
//  6. Freeze reads the failures of another job than the one whose receipt
//     vouches for the manifest;
//  7. a tail that does not parse fails Freeze instead of saying nothing;
//  8. the verifier's bundle mirror lacks the field, so inspection refuses;
//  9. concurrent requests race on the record;
//  10. a hostile tail (huge or malformed entries) makes the bundle invalid.

func TestServerRecordsFailedAssetAnswersOnly(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{"index.html": "<html></html>", "assets/app.js": "1", "read.html": "<html></html>"} {
		p := filepath.Join(root, filepath.FromSlash(name))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rec := NewAssetRecorder()
	srv := httptest.NewServer(rec.Wrap(Server(root)))
	defer srv.Close()
	for _, p := range []string{
		"/", "/read.html", "/assets/app.js", "/reader", // served or the SPA fallback
		"/pdf-text-BRoX2Cg_.js", "/pdf-text-BRoX2Cg_.js?v=1", // the D1 chunk, twice
		"/assets/missing.css", "/fonts/x.woff2", "/img/logo.svg", "/doc/sample.pdf", "/m.wasm",
		"/api/files/x.js", "/api/state", "/auth/me", "/auth/session", "/nothing.html", "/nothing.txt",
	} {
		resp, err := http.Get(srv.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
	}
	got := map[string]AssetFailure{}
	for _, f := range rec.Failures() {
		got[f.Path] = f
	}
	want := []string{"/pdf-text-BRoX2Cg_.js", "/assets/missing.css", "/fonts/x.woff2", "/img/logo.svg", "/doc/sample.pdf", "/m.wasm"}
	if len(want) != MaxAssetFailures || len(got) != len(want) {
		t.Fatalf("recorded %v, want %v", rec.Failures(), want)
	}
	for _, p := range want {
		if got[p].Status != http.StatusNotFound {
			t.Fatalf("%s: %+v", p, got[p])
		}
	}
	if got["/pdf-text-BRoX2Cg_.js"].Count != 2 {
		t.Fatalf("the chunk was asked for twice: %+v", got["/pdf-text-BRoX2Cg_.js"])
	}
	// Past the bound a new path is not kept; a kept one still counts.
	for _, p := range []string{"/x.mjs", "/b.epub", "/i.png", "/assets/missing.css"} {
		resp, err := http.Get(srv.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
	}
	if f := rec.Failures(); len(f) != MaxAssetFailures || f[1].Count != 2 {
		t.Fatalf("past the bound: %v", f)
	}
	if rec.Failures()[0].Path != "/pdf-text-BRoX2Cg_.js" {
		t.Fatalf("failures are not in first-seen order: %v", rec.Failures())
	}
}

func TestAssetRecorderIsBoundedAndConcurrencySafe(t *testing.T) {
	rec := NewAssetRecorder()
	h := rec.Wrap(http.NotFoundHandler())
	done := make(chan struct{})
	for g := 0; g < 8; g++ {
		go func(g int) {
			defer func() { done <- struct{}{} }()
			for i := 0; i < 50; i++ {
				r := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/chunk-%d-%d-%s.js", g, i, strings.Repeat("x", 400)), nil)
				h.ServeHTTP(httptest.NewRecorder(), r)
			}
		}(g)
	}
	for g := 0; g < 8; g++ {
		<-done
	}
	failures := rec.Failures()
	if len(failures) != MaxAssetFailures {
		t.Fatalf("%d failures kept, want %d", len(failures), MaxAssetFailures)
	}
	for _, f := range failures {
		if len(f.Path) > maxAssetPathLen {
			t.Fatalf("path not cut: %d bytes", len(f.Path))
		}
	}
}

// The summary names the failures, stays one line well inside the job's tail
// with the manifest digest still in it, and the manifest is untouched.
func TestSummaryWarnsOfFailedAssetsAndStaysInsideTheTail(t *testing.T) {
	raw := []byte("{\"kind\":\"visual_capture\"}\n")
	m := Manifest{Subject: Subject{FrontendSHA256: strings.Repeat("b", 64)},
		Captures: []Capture{{Route: "/read.html", Artifact: ArtifactRef(strings.Repeat("c", 64)), Bytes: 12}}}
	var failures []AssetFailure
	for i := 0; i < MaxAssetFailures; i++ {
		failures = append(failures, AssetFailure{Path: "/" + strings.Repeat("p", maxAssetPathLen-5) + fmt.Sprint(i) + ".js", Status: 404, Count: 3})
	}
	failures[0].Path = "/pdf-text-BRoX2Cg_.js"
	out := Summary(m, raw, failures)
	if strings.Contains(string(out), "\n") || len(out) > 2<<10 {
		t.Fatalf("summary is %d bytes or not one line", len(out))
	}
	var back struct {
		ManifestSHA256 string         `json:"manifest_sha256"`
		Warnings       []string       `json:"warnings"`
		AssetFailures  []AssetFailure `json:"asset_failures"`
	}
	if err := json.Unmarshal(out, &back); err != nil || back.ManifestSHA256 != DigestHex(raw) || len(back.AssetFailures) != MaxAssetFailures {
		t.Fatalf("summary: %v %s", err, out)
	}
	if len(back.Warnings) != 1 || !strings.Contains(back.Warnings[0], "404 /pdf-text-BRoX2Cg_.js") || !strings.Contains(back.Warnings[0], "never ran") {
		t.Fatalf("warnings = %v", back.Warnings)
	}
	parsed := ParseSummaryAssetFailures("npm noise\n"+string(out)+"\n", DigestHex(raw))
	if len(parsed) != MaxAssetFailures || parsed[0].Path != "/pdf-text-BRoX2Cg_.js" {
		t.Fatalf("parsed = %v", parsed)
	}
	if got := ParseSummaryAssetFailures(string(out), strings.Repeat("9", 64)); got != nil {
		t.Fatalf("another manifest's summary was read: %v", got)
	}
	if out := string(Summary(m, raw, nil)); strings.Contains(out, "asset_failures") || strings.Contains(out, "warnings") {
		t.Fatalf("no failures, yet: %s", out)
	}
}

// A tail the capture did not print as the parser expects yields nothing, or
// only sane entries, never an error.
func TestParseSummaryAssetFailuresSanitizes(t *testing.T) {
	sha := strings.Repeat("a", 64)
	line := func(failures string) string {
		return `{"manifest_sha256":"` + sha + `","asset_failures":` + failures + `}`
	}
	for _, tail := range []string{"", "not json", `{"manifest_sha256":"` + sha + `"`, line(`"x"`)} {
		if got := ParseSummaryAssetFailures(tail, sha); got != nil {
			t.Fatalf("%q parsed to %v", tail, got)
		}
	}
	many := make([]string, 0, 40)
	for i := 0; i < 40; i++ {
		many = append(many, fmt.Sprintf(`{"path":"/c%d.js","status":404,"count":1}`, i))
	}
	bad := `[{"path":"no-slash.js","status":404,"count":1},{"path":"/ok.js","status":200,"count":1},{"path":"/a b.js","status":404,"count":1},{"path":"/good.js","status":503,"count":0}]`
	got := ParseSummaryAssetFailures(line(bad), sha)
	if len(got) != 1 || got[0].Path != "/good.js" || got[0].Count != 1 {
		t.Fatalf("sanitized = %v", got)
	}
	if got := ParseSummaryAssetFailures(line("["+strings.Join(many, ",")+"]"), sha); len(got) != MaxAssetFailures {
		t.Fatalf("%d kept", len(got))
	}
	for _, f := range got {
		if err := f.Validate(); err != nil {
			t.Fatalf("a parsed failure does not validate: %v", err)
		}
	}
}

// End to end through Run: the page's module imports a chunk the build did
// not put where the import resolves; the browser asks for it and the run
// reports the 404, while the manifest is byte for byte what it would be
// without the failure.
func TestRunReportsAModuleWhoseImportIsMissing(t *testing.T) {
	fx := newFixture(t)
	page := `<html><body><div id=app></div><script type=module>import{l as M}from"./pdf-text-BRoX2Cg_.js";M()</script></body></html>`
	if err := os.WriteFile(filepath.Join(fx.serve, "read.html"), []byte(page), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fx.serve, "assets", "pdf-text-BRoX2Cg_.js"), []byte("export const l=()=>1"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := RunReport(context.Background(), fx.opts(t, "/read.html"))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.AssetFailures) != 1 || res.AssetFailures[0].Path != "/pdf-text-BRoX2Cg_.js" || res.AssetFailures[0].Status != 404 {
		t.Fatalf("asset failures = %+v", res.AssetFailures)
	}
	raw, err := os.ReadFile(filepath.Join(fx.out, ManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "pdf-text") || strings.Contains(string(raw), "asset") {
		t.Fatalf("the manifest carries the advice: %s", raw)
	}
	again, err := json.MarshalIndent(res.Manifest, "", "  ")
	if err != nil || string(append(again, '\n')) != string(raw) {
		t.Fatalf("the manifest is not the plain manifest: %v", err)
	}
	// The fixed build: the import resolves, nothing is reported.
	fixed := strings.Replace(page, `"./pdf-text-`, `"./assets/pdf-text-`, 1)
	if err := os.WriteFile(filepath.Join(fx.serve, "read.html"), []byte(fixed), 0o644); err != nil {
		t.Fatal(err)
	}
	if res, err := RunReport(context.Background(), fx.opts(t, "/read.html")); err != nil || len(res.AssetFailures) != 0 {
		t.Fatalf("a resolving import reported %+v (%v)", res.AssetFailures, err)
	}
}
