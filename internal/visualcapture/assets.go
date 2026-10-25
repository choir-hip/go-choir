package visualcapture

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strings"
	"sync"
)

// Failed asset answers (fix 3, trace review round 4,
// docs/evidence/selfdev-trace-reviews-2026-10-25.md D1). A page whose module
// imports a chunk the server does not have renders nothing, and Chromium
// prints nothing for it: only the capture server sees the 404. The server is
// wrapped to record such answers, and the capture names them as advice in the
// job's summary, Freeze's visual_warnings and the verifier's inspection. The
// manifest never carries them, so its format and digest do not change.

const (
	// MaxAssetFailures bounds the distinct failed assets kept, so the job's
	// summary stays well inside its 4 KiB output tail with the manifest
	// digest in it (Freeze looks for the digest there, V17).
	MaxAssetFailures = 6
	// maxAssetPathLen bounds one recorded path.
	maxAssetPathLen = 96
)

// assetExtensions are the subresources a page needs to render: scripts,
// styles, wasm, documents the app opens, fonts and images.
var assetExtensions = map[string]bool{
	".js": true, ".mjs": true, ".css": true, ".wasm": true, ".pdf": true, ".epub": true,
	".woff": true, ".woff2": true, ".ttf": true, ".otf": true, ".eot": true,
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".svg": true, ".webp": true, ".avif": true, ".ico": true, ".bmp": true,
}

// AssetFailure is one asset path the capture server answered with a
// non-2xx status, and how often.
type AssetFailure struct {
	Path   string `json:"path"`
	Status int    `json:"status"`
	Count  int    `json:"count"`
}

// Validate checks a failure's shape, as ParseSummaryAssetFailures admits it.
func (f AssetFailure) Validate() error {
	if !isAssetPath(f.Path) || len(f.Path) > maxAssetPathLen || f.Status < 100 || f.Status > 599 || (f.Status >= 200 && f.Status < 300) || f.Count < 1 {
		return fmt.Errorf("asset failure %q (status %d, count %d) is not a failed asset answer", f.Path, f.Status, f.Count)
	}
	for _, c := range f.Path {
		if c <= ' ' || c == 0x7f {
			return fmt.Errorf("asset failure path %q has a space or control character", f.Path)
		}
	}
	return nil
}

// isAssetPath reports a request path for an asset, outside the intended
// signed-out backend (/api/ answers 503 and /auth/ 401 on purpose).
func isAssetPath(p string) bool {
	if !strings.HasPrefix(p, "/") || p == "/api" || strings.HasPrefix(p, "/api/") || p == "/auth" || strings.HasPrefix(p, "/auth/") {
		return false
	}
	return assetExtensions[strings.ToLower(path.Ext(p))]
}

// AssetRecorder records the failed asset answers of the handler it wraps.
type AssetRecorder struct {
	mu       sync.Mutex
	failures []AssetFailure
}

// NewAssetRecorder returns an empty recorder.
func NewAssetRecorder() *AssetRecorder { return &AssetRecorder{} }

// Wrap records every non-2xx answer to an asset request (a 304 is a cache
// answer, not a failure). Past MaxAssetFailures distinct paths, new ones are
// not kept.
func (a *AssetRecorder) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		status := sw.status
		if status == 0 {
			status = http.StatusOK
		}
		if (status >= 200 && status < 300) || status == http.StatusNotModified || !isAssetPath(r.URL.Path) {
			return
		}
		a.record(r.URL.Path, status)
	})
}

func (a *AssetRecorder) record(p string, status int) {
	if len(p) > maxAssetPathLen {
		p = p[:maxAssetPathLen]
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := range a.failures {
		if a.failures[i].Path == p && a.failures[i].Status == status {
			a.failures[i].Count++
			return
		}
	}
	if len(a.failures) < MaxAssetFailures {
		a.failures = append(a.failures, AssetFailure{Path: p, Status: status, Count: 1})
	}
}

// Failures is what the recorder kept, in first-seen order.
func (a *AssetRecorder) Failures() []AssetFailure {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.failures) == 0 {
		return nil
	}
	return append([]AssetFailure(nil), a.failures...)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusWriter) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.ResponseWriter.Write(b)
}

// AssetFailureWarnings is the advice for failed asset answers: one warning
// naming each, or none.
func AssetFailureWarnings(failures []AssetFailure) []string {
	if len(failures) == 0 {
		return nil
	}
	named := make([]string, 0, len(failures))
	for _, f := range failures {
		row := fmt.Sprintf("%d %s", f.Status, f.Path)
		if f.Count > 1 {
			row += fmt.Sprintf(" (x%d)", f.Count)
		}
		named = append(named, row)
	}
	more := ""
	if len(failures) >= MaxAssetFailures {
		more = fmt.Sprintf(" (the first %d kept)", MaxAssetFailures)
	}
	return []string{fmt.Sprintf("the capture server could not serve %d asset request(s)%s: %s. A script or module that failed to load never ran, so the screenshot may show a page that did not render; a relative import inlined into an .html page resolves against that page's directory, not /assets/", len(failures), more, strings.Join(named, ", "))}
}

// ParseSummaryAssetFailures reads the asset failures from a capture job's
// output tail: the summary line whose manifest digest is manifestSHA256.
// Anything malformed is skipped, never an error; the result is bounded and
// every entry passes Validate.
func ParseSummaryAssetFailures(tail, manifestSHA256 string) []AssetFailure {
	lines := strings.Split(tail, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "{") || !strings.Contains(line, manifestSHA256) {
			continue
		}
		var summary struct {
			ManifestSHA256 string            `json:"manifest_sha256"`
			AssetFailures  []json.RawMessage `json:"asset_failures"`
		}
		if json.Unmarshal([]byte(line), &summary) != nil || summary.ManifestSHA256 != manifestSHA256 {
			continue
		}
		var out []AssetFailure
		for _, raw := range summary.AssetFailures {
			var f AssetFailure
			if json.Unmarshal(raw, &f) != nil {
				continue
			}
			f.Count = max(f.Count, 1)
			if f.Validate() != nil {
				continue
			}
			out = append(out, f)
			if len(out) == MaxAssetFailures {
				break
			}
		}
		return out
	}
	return nil
}
