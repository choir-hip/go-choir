package visualcapture

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SummaryMaxBytes bounds the summary line. The job's output tail is the last
// 4 KiB (capsulejob.TailBytes) and Freeze finds the receipt by the manifest
// digest near the line's start, so a longer line loses it (review bunch
// 238 D: six 512-byte routes, their shared-screenshot warning and six failed
// assets made about 7 KiB).
const SummaryMaxBytes = 3 << 10

// summaryRouteLen is how much of a route a shortened summary keeps.
const summaryRouteLen = 64

// Summary is the one-line JSON a capture job prints, which becomes the job's
// output tail. It names the manifest's digest: Freeze accepts the job's
// receipt as the evidence's receipt only when that digest is in the output,
// so a manifest written or rewritten after the job does not carry one (V17).
// failures are the run's failed asset answers (Report.AssetFailures): the
// summary lists them and warns, so Freeze can carry them to the verifier.
// dropped counts failed answers the recorder did not keep (Report.AssetDropped).
//
// The line is at most SummaryMaxBytes. Past it the summary sheds, in order:
// long routes (cut to 64 bytes with "..."), the warnings (Freeze and the
// verifier recompute them from the manifest and the failures), the per-route
// rows (the manifest has them), and last the failure list.
func Summary(m Manifest, manifestRaw []byte, failures []AssetFailure, dropped int) []byte {
	if len(failures) > MaxAssetFailures {
		failures = failures[:MaxAssetFailures]
	}
	var out []byte
	for level := 0; level <= 4; level++ {
		out = summaryAt(m, manifestRaw, failures, dropped, level)
		if len(out) <= SummaryMaxBytes {
			break
		}
	}
	return out
}

func summaryAt(m Manifest, manifestRaw []byte, failures []AssetFailure, dropped, level int) []byte {
	type row struct {
		Route    string `json:"route"`
		Artifact string `json:"artifact"`
		Bytes    int64  `json:"bytes"`
		Blank    bool   `json:"blank,omitempty"`
	}
	summary := struct {
		Manifest       string         `json:"manifest"`
		ManifestSHA256 string         `json:"manifest_sha256"`
		FrontendSHA256 string         `json:"frontend_sha256"`
		Browser        string         `json:"browser,omitempty"`
		Captures       []row          `json:"captures"`
		AssetFailures  []AssetFailure `json:"asset_failures,omitempty"`
		AssetDropped   int            `json:"asset_failures_dropped,omitempty"`
		Warnings       []string       `json:"warnings,omitempty"`
		Shortened      bool           `json:"shortened,omitempty"`
	}{
		Manifest:       OutputDir + "/" + ManifestFile,
		ManifestSHA256: DigestHex(manifestRaw),
		FrontendSHA256: m.Subject.FrontendSHA256,
		Browser:        m.Browser.Version,
		Captures:       []row{},
		Shortened:      level > 0,
	}
	route := func(r string) string {
		if level >= 1 && len(r) > summaryRouteLen {
			return r[:summaryRouteLen] + "..."
		}
		return r
	}
	shots := make([]RouteShot, 0, len(m.Captures))
	for _, c := range m.Captures {
		if level < 3 {
			summary.Captures = append(summary.Captures, row{Route: route(c.Route), Artifact: c.Artifact, Bytes: c.Bytes, Blank: c.Blank})
		}
		shots = append(shots, RouteShot{Route: route(c.Route), Artifact: c.Artifact})
	}
	if level < 4 {
		summary.AssetFailures = failures
	}
	summary.AssetDropped = dropped
	if level < 2 {
		summary.Warnings = append(SharedShotWarnings(shots), AssetFailureWarnings(failures, dropped)...)
	}
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
