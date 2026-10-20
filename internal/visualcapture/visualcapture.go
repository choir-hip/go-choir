// Package visualcapture takes viewport screenshots of a computer's candidate
// frontend inside its engineering capsule, and checks the evidence they leave
// (docs/design/visual-evidence-slice-2026-10-15.md).
//
// The capture tool (cmd/choir-capture) runs as a capsule job. It serves the
// built release frontend on the capsule's own loopback, runs headless
// Chromium once per route, and writes each PNG, named by its sha256, and one
// manifest into OutputDir. Freeze stages that directory beside release/ in
// the frozen bundle and checks it with ValidateEvidence. This package never
// imports the capsule, so both sides share one definition of the evidence.
package visualcapture

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// ToolName is the capture tool's command name on the capsule PATH.
	ToolName = "choir-capture"
	// BrowserEnv names the Chromium binary in a capture job's environment.
	// The capsule broker sets it only on a computer whose grant is on (V1).
	BrowserEnv = "CHOIR_CAPTURE_BROWSER"
	// FontconfigEnv names the fontconfig file Chromium renders with (V12).
	FontconfigEnv = "CHOIR_CAPTURE_FONTCONFIG"

	// ServeRoot is the built release frontend the tool serves.
	ServeRoot = "/var/lib/artifact/release/frontend"
	// OutputDir is where captures and the manifest land: beside the release
	// tree, outside /workspace/platform (V15), so the release never carries
	// them and the classifier sees them as artifacts.
	OutputDir = "/var/lib/artifact/evidence/visual"
	// EvidencePrefix is OutputDir as a capsule diff path.
	EvidencePrefix = "var/lib/artifact/evidence/visual/"
	// BundleDir is where Freeze stages the evidence inside a bundle directory.
	BundleDir = "evidence/visual"
	// ManifestFile is the manifest's name in OutputDir.
	ManifestFile = "visual-evidence.json"
	// ManifestKind is the manifest's kind.
	ManifestKind = "visual_capture"
	// MediaTypePNG is the only capture media type in this slice.
	MediaTypePNG = "image/png"

	DefaultWidth  = 1280
	DefaultHeight = 800
	MaxWidth      = 1920
	MaxHeight     = 1200
	MinDimension  = 200
	MaxRoutes     = 6
	DefaultWaitMS = 5000
	MaxWaitMS     = 20000
	maxRouteLen   = 512

	// ShotDeadline bounds one Chromium run; on expiry its process group is
	// killed (V5).
	ShotDeadline = 60 * time.Second
	// MaxCaptureBytes is the gateway's image limit; MaxTotalBytes bounds
	// one bundle's evidence (V19).
	MaxCaptureBytes = 20 << 20
	MaxTotalBytes   = 24 << 20
	// MaxManifestBytes bounds the manifest a Freeze will read.
	MaxManifestBytes = 256 << 10
)

// Subject binds a manifest to the frontend build it was taken of (V8).
type Subject struct {
	FrontendSHA256 string `json:"frontend_sha256"`
	FrontendFiles  int    `json:"frontend_files"`
}

// Capture is one screenshot.
type Capture struct {
	Artifact      string `json:"artifact"`
	File          string `json:"file"`
	MediaType     string `json:"media_type"`
	Bytes         int64  `json:"bytes"`
	Route         string `json:"route"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	FullPage      bool   `json:"full_page"`
	ReducedMotion bool   `json:"reduced_motion"`
	Blank         bool   `json:"blank,omitempty"`
	TakenAt       string `json:"taken_at"`
}

// Browser names what took the captures.
type Browser struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Version string `json:"version,omitempty"`
}

// Manifest is visual-evidence.json.
type Manifest struct {
	Kind     string    `json:"kind"`
	Subject  Subject   `json:"subject"`
	Captures []Capture `json:"captures"`
	Browser  Browser   `json:"browser"`
	WaitMS   int       `json:"wait_ms"`
}

// FileDigest is one file of a frontend tree, by its path under the tree.
type FileDigest struct {
	Path   string
	SHA256 string
}

// FrontendDigest is the digest of a frontend tree: sha256 over its files'
// sorted "path NUL sha256 LF" lines. Paths are slash-separated and relative
// to the frontend root. Freeze computes the same value from the staged
// release's frontend/ files.
func FrontendDigest(files []FileDigest) string {
	sorted := append([]FileDigest(nil), files...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	h := sha256.New()
	for _, f := range sorted {
		h.Write([]byte(f.Path))
		h.Write([]byte{0})
		h.Write([]byte(strings.ToLower(f.SHA256)))
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ArtifactRef is the artifact:sha256 ref of a capture's bytes.
func ArtifactRef(sha256Hex string) string { return "artifact:sha256:" + sha256Hex }

// IsCaptureJob reports whether a job's argv runs the capture tool: the seam
// a memory-lease admission hook uses to size a browser lease differently
// from a build (docs/design/capsule-long-exec-2026-10-15.md R3).
func IsCaptureJob(argv []string) bool {
	return len(argv) > 0 && filepath.Base(strings.TrimSpace(argv[0])) == ToolName
}

// NormalizeRoute checks a route: an absolute path with an optional query,
// on this origin only.
func NormalizeRoute(raw string) (string, error) {
	r := strings.TrimSpace(raw)
	switch {
	case r == "":
		return "", fmt.Errorf("route is empty")
	case len(r) > maxRouteLen:
		return "", fmt.Errorf("route %q is longer than %d bytes", r, maxRouteLen)
	case !strings.HasPrefix(r, "/") || strings.HasPrefix(r, "//") || strings.Contains(r, "://") || strings.ContainsAny(r, "\\#"):
		return "", fmt.Errorf("route %q must be a path on this origin, such as / or /choir-drill/x.html", raw)
	}
	for _, c := range r {
		if c <= ' ' || c == 0x7f {
			return "", fmt.Errorf("route %q has a space or control character", raw)
		}
	}
	p, _, _ := strings.Cut(r, "?")
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return "", fmt.Errorf("route %q leaves the frontend root", raw)
		}
	}
	return r, nil
}

// CheckPNG decodes a capture and checks its size is the viewport asked for
// (V11). blank reports a uniform image (a page that rendered nothing).
func CheckPNG(data []byte, width, height int) (blank bool, err error) {
	if len(data) == 0 {
		return false, fmt.Errorf("capture is empty")
	}
	if len(data) > MaxCaptureBytes {
		return false, fmt.Errorf("capture is %d bytes; the limit is %d", len(data), MaxCaptureBytes)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return false, fmt.Errorf("capture is not a PNG: %w", err)
	}
	if cfg.Width != width || cfg.Height != height {
		return false, fmt.Errorf("capture is %dx%d, not the %dx%d viewport asked for", cfg.Width, cfg.Height, width, height)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return false, fmt.Errorf("capture does not decode: %w", err)
	}
	return uniform(img), nil
}

func uniform(img image.Image) bool {
	b := img.Bounds()
	r0, g0, b0, a0 := img.At(b.Min.X, b.Min.Y).RGBA()
	for y := b.Min.Y; y < b.Max.Y; y += 2 {
		for x := b.Min.X; x < b.Max.X; x += 2 {
			r, g, bl, a := img.At(x, y).RGBA()
			if r != r0 || g != g0 || bl != b0 || a != a0 {
				return false
			}
		}
	}
	return true
}

// EvidenceFile is one file Freeze found in the capsule's evidence directory:
// its name in that directory, size and sha256.
type EvidenceFile struct {
	Name   string
	Size   int64
	SHA256 string
}

// ValidateEvidence checks a capsule's evidence directory as Freeze staged
// it: the manifest, every capture's file and hash, nothing extra, the size
// bounds, and the binding to the frozen frontend (V8, V17-V21).
// frontendSHA256 is FrontendDigest of the staged release's frontend/ files.
func ValidateEvidence(manifestRaw []byte, files []EvidenceFile, frontendSHA256 string) (Manifest, error) {
	var m Manifest
	if len(manifestRaw) == 0 {
		return m, fmt.Errorf("visual evidence has captures but no %s", ManifestFile)
	}
	if len(manifestRaw) > MaxManifestBytes {
		return m, fmt.Errorf("%s is larger than %d bytes", ManifestFile, MaxManifestBytes)
	}
	dec := json.NewDecoder(bytes.NewReader(manifestRaw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return m, fmt.Errorf("%s is malformed: %w", ManifestFile, err)
	}
	if m.Kind != ManifestKind {
		return m, fmt.Errorf("%s kind is %q, not %q", ManifestFile, m.Kind, ManifestKind)
	}
	if len(m.Captures) == 0 || len(m.Captures) > MaxRoutes {
		return m, fmt.Errorf("%s names %d captures; 1..%d are admitted", ManifestFile, len(m.Captures), MaxRoutes)
	}
	if !isHex64(m.Subject.FrontendSHA256) {
		return m, fmt.Errorf("%s has no frontend digest", ManifestFile)
	}
	if m.Subject.FrontendSHA256 != frontendSHA256 {
		return m, fmt.Errorf("visual evidence was captured from a different frontend build than the release (manifest %s, release %s); capture again after the last build", short(m.Subject.FrontendSHA256), short(frontendSHA256))
	}
	byName := map[string]EvidenceFile{}
	for _, f := range files {
		byName[f.Name] = f
	}
	var total int64
	// Two routes that render the same pixels share one file; a route is
	// named once.
	seen := map[string]bool{}
	routes := map[string]bool{}
	for i, c := range m.Captures {
		sum := strings.TrimSuffix(c.File, ".png")
		if !isHex64(sum) || c.File != sum+".png" || c.Artifact != ArtifactRef(sum) || c.MediaType != MediaTypePNG {
			return m, fmt.Errorf("%s capture %d is not named by its sha256 (file %q, artifact %q)", ManifestFile, i, c.File, c.Artifact)
		}
		route, err := NormalizeRoute(c.Route)
		if err != nil {
			return m, fmt.Errorf("%s capture %d: %w", ManifestFile, i, err)
		}
		if routes[route] {
			return m, fmt.Errorf("%s names route %s twice", ManifestFile, route)
		}
		routes[route] = true
		firstUse := !seen[c.File]
		seen[c.File] = true
		if c.FullPage || c.Width < MinDimension || c.Width > MaxWidth || c.Height < MinDimension || c.Height > MaxHeight {
			return m, fmt.Errorf("%s capture %d is not an admitted viewport capture", ManifestFile, i)
		}
		f, ok := byName[c.File]
		if !ok {
			return m, fmt.Errorf("%s names %s, which is not in the evidence directory", ManifestFile, c.File)
		}
		if f.SHA256 != sum || f.Size != c.Bytes {
			return m, fmt.Errorf("%s does not match its manifest entry (sha256 or size)", c.File)
		}
		if f.Size > MaxCaptureBytes {
			return m, fmt.Errorf("%s is %d bytes; the limit is %d", c.File, f.Size, MaxCaptureBytes)
		}
		if firstUse {
			total += f.Size
		}
	}
	if total > MaxTotalBytes {
		return m, fmt.Errorf("visual evidence is %d bytes; the limit is %d", total, MaxTotalBytes)
	}
	for _, f := range files {
		if f.Name != ManifestFile && !seen[f.Name] {
			return m, fmt.Errorf("the evidence directory holds %s, which the manifest does not name", f.Name)
		}
	}
	return m, nil
}

// Routes lists the manifest's routes in capture order.
func (m Manifest) Routes() []string {
	out := make([]string, 0, len(m.Captures))
	for _, c := range m.Captures {
		out = append(out, c.Route)
	}
	return out
}

// cleanRel turns a request path into a clean path under the served root,
// or "" when it would leave it.
func cleanRel(p string) string {
	c := path.Clean("/" + p)
	if c == "/" {
		return ""
	}
	return strings.TrimPrefix(c, "/")
}

// DigestHex is the lowercase hex sha256 of data: the digest Freeze records
// for the manifest.
func DigestHex(data []byte) string { return sha256Hex(data) }

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func isHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil && strings.ToLower(s) == s
}

func short(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}
