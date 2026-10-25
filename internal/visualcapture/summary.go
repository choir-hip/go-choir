package visualcapture

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Summary is the one-line JSON a capture job prints, which becomes the job's
// output tail. It names the manifest's digest: Freeze accepts the job's
// receipt as the evidence's receipt only when that digest is in the output,
// so a manifest written or rewritten after the job does not carry one (V17).
func Summary(m Manifest, manifestRaw []byte) []byte {
	type row struct {
		Route    string `json:"route"`
		Artifact string `json:"artifact"`
		Bytes    int64  `json:"bytes"`
		Blank    bool   `json:"blank,omitempty"`
	}
	summary := struct {
		Manifest       string   `json:"manifest"`
		ManifestSHA256 string   `json:"manifest_sha256"`
		FrontendSHA256 string   `json:"frontend_sha256"`
		Browser        string   `json:"browser,omitempty"`
		Captures       []row    `json:"captures"`
		Warnings       []string `json:"warnings,omitempty"`
	}{
		Manifest:       OutputDir + "/" + ManifestFile,
		ManifestSHA256: DigestHex(manifestRaw),
		FrontendSHA256: m.Subject.FrontendSHA256,
		Browser:        m.Browser.Version,
		Captures:       []row{},
	}
	shots := make([]RouteShot, 0, len(m.Captures))
	for _, c := range m.Captures {
		summary.Captures = append(summary.Captures, row{Route: c.Route, Artifact: c.Artifact, Bytes: c.Bytes, Blank: c.Blank})
		shots = append(shots, RouteShot{Route: c.Route, Artifact: c.Artifact})
	}
	summary.Warnings = SharedShotWarnings(shots)
	out, _ := json.Marshal(summary)
	return out
}

// RouteShot is one capture's route and artifact ref.
type RouteShot struct {
	Route    string
	Artifact string
}

// SharedShotWarnings names each artifact that more than one route produced,
// in capture order. The capture server answers a path with no extension that
// is not a file with index.html, so routes with one screenshot most often
// all show the root page (problem 7, docs/evidence/selfdev-trace-reviews-
// 2026-10-25.md). The engineer reads it in the job's summary and Freeze's
// result; the verifier in the bundle inspection. It is advice, not a check.
func SharedShotWarnings(shots []RouteShot) []string {
	routes := map[string][]string{}
	var order []string
	for _, s := range shots {
		if _, seen := routes[s.Artifact]; !seen {
			order = append(order, s.Artifact)
		}
		routes[s.Artifact] = append(routes[s.Artifact], s.Route)
	}
	var out []string
	for _, artifact := range order {
		rs := routes[artifact]
		if len(rs) < 2 {
			continue
		}
		named := strings.Join(rs[:len(rs)-1], ", ") + " and " + rs[len(rs)-1]
		out = append(out, fmt.Sprintf("routes %s produced the same screenshot (%s): the capture server answers a path that is not a file with index.html, so these may all show the root page; capture a page by its file, such as /read.html", named, short(strings.TrimPrefix(artifact, "artifact:sha256:"))))
	}
	return out
}
