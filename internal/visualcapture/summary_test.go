package visualcapture

import (
	"encoding/json"
	"strings"
	"testing"
)

// V17: the manifest digest Freeze looks for in the job's output is the one
// the summary prints, and it is the digest of the manifest bytes on disk.
func TestSummaryNamesTheManifestDigest(t *testing.T) {
	raw := []byte("{\"kind\":\"visual_capture\"}\n")
	m := Manifest{Subject: Subject{FrontendSHA256: strings.Repeat("b", 64)}, Browser: Browser{Version: "155"},
		Captures: []Capture{{Route: "/", Artifact: ArtifactRef(strings.Repeat("c", 64)), Bytes: 12}}}
	out := Summary(m, raw, nil, 0)
	if !strings.Contains(string(out), DigestHex(raw)) {
		t.Fatalf("summary does not carry the manifest digest: %s", out)
	}
	var back struct {
		ManifestSHA256 string `json:"manifest_sha256"`
		FrontendSHA256 string `json:"frontend_sha256"`
		Captures       []struct {
			Route string `json:"route"`
		} `json:"captures"`
	}
	if err := json.Unmarshal(out, &back); err != nil || back.ManifestSHA256 != DigestHex(raw) || back.FrontendSHA256 != m.Subject.FrontendSHA256 || len(back.Captures) != 1 {
		t.Fatalf("summary is not the expected one-line JSON: %v %s", err, out)
	}
	if strings.Contains(string(out), "\n") {
		t.Fatal("the summary must be one line")
	}
}

// Problem 7 (docs/evidence/selfdev-trace-reviews-2026-10-25.md): the server
// answers an extensionless route with index.html, so a route that is not a
// file captures the root page. Two routes with one artifact are named in the
// summary the engineer reads, and nothing is said when every route differs.
func TestSummaryWarnsWhenRoutesShareOneScreenshot(t *testing.T) {
	same := ArtifactRef(strings.Repeat("f", 64))
	m := Manifest{Subject: Subject{FrontendSHA256: strings.Repeat("b", 64)}, Captures: []Capture{
		{Route: "/", Artifact: same, Bytes: 12},
		{Route: "/read", Artifact: same, Bytes: 12},
		{Route: "/x.html", Artifact: ArtifactRef(strings.Repeat("e", 64)), Bytes: 12},
		{Route: "/pdf", Artifact: same, Bytes: 12},
	}}
	out := string(Summary(m, []byte("{}\n"), nil, 0))
	var back struct {
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal([]byte(out), &back); err != nil || len(back.Warnings) != 1 {
		t.Fatalf("summary warnings = %v (%v): %s", back.Warnings, err, out)
	}
	w := back.Warnings[0]
	for _, want := range []string{"/, /read and /pdf", "same screenshot", "index.html", ".html"} {
		if !strings.Contains(w, want) {
			t.Fatalf("warning %q lacks %q", w, want)
		}
	}
	if strings.Contains(w, "/x.html") || strings.Contains(out, "\n") {
		t.Fatalf("warning names a distinct route or the summary is not one line: %s", out)
	}
	m.Captures = m.Captures[2:3]
	if out := string(Summary(m, []byte("{}\n"), nil, 0)); strings.Contains(out, "warnings") {
		t.Fatalf("a capture with distinct routes warns: %s", out)
	}
}
