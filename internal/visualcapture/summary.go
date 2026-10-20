package visualcapture

import "encoding/json"

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
		Manifest       string `json:"manifest"`
		ManifestSHA256 string `json:"manifest_sha256"`
		FrontendSHA256 string `json:"frontend_sha256"`
		Browser        string `json:"browser,omitempty"`
		Captures       []row  `json:"captures"`
	}{
		Manifest:       OutputDir + "/" + ManifestFile,
		ManifestSHA256: DigestHex(manifestRaw),
		FrontendSHA256: m.Subject.FrontendSHA256,
		Browser:        m.Browser.Version,
		Captures:       []row{},
	}
	for _, c := range m.Captures {
		summary.Captures = append(summary.Captures, row{Route: c.Route, Artifact: c.Artifact, Bytes: c.Bytes, Blank: c.Blank})
	}
	out, _ := json.Marshal(summary)
	return out
}
