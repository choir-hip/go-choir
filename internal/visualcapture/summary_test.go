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
	out := Summary(m, raw)
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
