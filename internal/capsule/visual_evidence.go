package capsule

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/visualcapture"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Visual evidence in the frozen bundle
// (docs/design/visual-evidence-slice-2026-10-15.md part 3). A capture job
// (cmd/choir-capture) leaves PNGs and a manifest under
// /var/lib/artifact/evidence/visual/; Freeze stages them beside release/,
// checks them against the frozen frontend, and records this summary in the
// bundle, whose digest therefore covers every capture's bytes.

// VisualEvidence is the bundle's record of the captures it carries.
type VisualEvidence struct {
	ManifestSHA256 string `json:"manifest_sha256"`
	FrontendSHA256 string `json:"frontend_sha256"`
	// CaptureReceipt is the execution receipt of the choir-capture job that
	// ended done in this capsule and whose output names this manifest's
	// digest, the latest such job; empty when there is none (hand-written or
	// rewritten evidence, V17), which visual_captured does not accept.
	CaptureReceipt string                  `json:"capture_receipt,omitempty"`
	Captures       []VisualEvidenceCapture `json:"captures"`
	// AssetFailures are the asset requests the capture server could not
	// serve, as the receipt's capture job printed them: advice for the
	// engineer and the verifier, never a check, and never in the manifest
	// (fix 3, docs/evidence/selfdev-trace-reviews-2026-10-25.md round 4).
	AssetFailures []visualcapture.AssetFailure `json:"asset_failures,omitempty"`
}

// VisualEvidenceCapture is one capture: its artifact ref, route and size.
type VisualEvidenceCapture struct {
	Artifact  string `json:"artifact"`
	Route     string `json:"route"`
	MediaType string `json:"media_type"`
	Bytes     int64  `json:"bytes"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Blank     bool   `json:"blank,omitempty"`
}

// Validate checks the record's shape (refs, digests, bounds).
func (v *VisualEvidence) Validate() error {
	if v == nil {
		return nil
	}
	if !isHex64(v.ManifestSHA256) || !isHex64(v.FrontendSHA256) {
		return fmt.Errorf("visual evidence: manifest and frontend digests are required")
	}
	if v.CaptureReceipt != "" && !strings.HasPrefix(v.CaptureReceipt, "capsule-exec:sha256:") {
		return fmt.Errorf("visual evidence: capture receipt %q is not an execution receipt", v.CaptureReceipt)
	}
	if len(v.Captures) == 0 || len(v.Captures) > visualcapture.MaxRoutes {
		return fmt.Errorf("visual evidence: 1..%d captures are admitted", visualcapture.MaxRoutes)
	}
	for _, c := range v.Captures {
		sum, ok := strings.CutPrefix(c.Artifact, "artifact:sha256:")
		if !ok || !isHex64(sum) || c.MediaType != visualcapture.MediaTypePNG || c.Bytes <= 0 {
			return fmt.Errorf("visual evidence: capture %q is not a content-addressed PNG", c.Artifact)
		}
		if _, err := visualcapture.NormalizeRoute(c.Route); err != nil {
			return fmt.Errorf("visual evidence: %w", err)
		}
	}
	if len(v.AssetFailures) > visualcapture.MaxAssetFailures {
		return fmt.Errorf("visual evidence: at most %d asset failures are recorded", visualcapture.MaxAssetFailures)
	}
	for _, f := range v.AssetFailures {
		if err := f.Validate(); err != nil {
			return fmt.Errorf("visual evidence: %w", err)
		}
	}
	return nil
}

// Refs lists the capture artifact refs, in capture order.
func (v *VisualEvidence) Refs() []string {
	if v == nil {
		return nil
	}
	out := make([]string, 0, len(v.Captures))
	for _, c := range v.Captures {
		out = append(out, c.Artifact)
	}
	return out
}

// ReleaseFrontendDigest is visualcapture.FrontendDigest of a release's
// frontend/ files, as Freeze staged them.
func ReleaseFrontendDigest(files []FrozenReleaseFile) string {
	var tree []visualcapture.FileDigest
	for _, f := range files {
		if rel, ok := strings.CutPrefix(f.Path, "frontend/"); ok && rel != "" {
			tree = append(tree, visualcapture.FileDigest{Path: rel, SHA256: f.SHA256})
		}
	}
	return visualcapture.FrontendDigest(tree)
}

// visualEvidenceNames lists the files the capsule left directly in its
// evidence directory, from the capsule's diff. Deleted entries and
// directories are skipped; a nested path is refused (the tool writes a flat
// directory).
func visualEvidenceNames(changes []FileChange) ([]string, error) {
	var names []string
	for _, change := range changes {
		rel, ok := strings.CutPrefix(strings.TrimPrefix(change.Path, "/"), visualcapture.EvidencePrefix)
		if !ok || change.Kind == ChangeDeleted || change.Mode.IsDir() {
			continue
		}
		if rel == "" || strings.Contains(rel, "/") || rel == "." || rel == ".." {
			return nil, fmt.Errorf("capsule freeze: visual evidence path %q is not a file directly in %s", change.Path, visualcapture.OutputDir)
		}
		names = append(names, rel)
	}
	if len(names) > visualcapture.MaxRoutes+1 {
		return nil, fmt.Errorf("capsule freeze: the visual evidence directory holds %d files; at most %d captures and the manifest", len(names), visualcapture.MaxRoutes)
	}
	return names, nil
}

// evidenceInput is an opened evidence file; *os.File satisfies it.
type evidenceInput interface {
	io.Reader
	io.Closer
	Stat() (os.FileInfo, error)
}

// stageEvidenceFiles reads each named evidence file through open (the Linux
// executor opens beneath the capsule root without following symlinks),
// bounds it, scans it for secrets, checks the whole directory against the
// staged release, and only then writes it under
// <temporary>/evidence/visual/ (beside release/, never under it: V20). open
// is the only capsule-specific part, so every refusal is tested without a
// capsule. jobs are the capsule's own jobs, oldest first.
func stageEvidenceFiles(ctx context.Context, names []string, open func(name string) (evidenceInput, error), temporary string, release []FrozenReleaseFile, jobs []yaegikernel.JobView) (*VisualEvidence, error) {
	if len(names) == 0 {
		return nil, nil
	}
	var files []visualcapture.EvidenceFile
	contents := map[string][]byte{}
	var total int64
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		input, err := open(name)
		if err != nil {
			return nil, fmt.Errorf("capsule freeze: visual evidence %q is unavailable: %w", name, err)
		}
		info, err := input.Stat()
		if err != nil || !info.Mode().IsRegular() {
			_ = input.Close()
			return nil, fmt.Errorf("capsule freeze: visual evidence %q is not a regular file", name)
		}
		limit := int64(visualcapture.MaxCaptureBytes)
		if name == visualcapture.ManifestFile {
			limit = visualcapture.MaxManifestBytes
		}
		data, err := io.ReadAll(io.LimitReader(input, limit+1))
		_ = input.Close()
		if err != nil {
			return nil, err
		}
		if int64(len(data)) > limit {
			return nil, fmt.Errorf("capsule freeze: visual evidence %q is larger than %d bytes", name, limit)
		}
		total += int64(len(data))
		if total > visualcapture.MaxTotalBytes+visualcapture.MaxManifestBytes {
			return nil, fmt.Errorf("capsule freeze: visual evidence exceeds %d bytes", visualcapture.MaxTotalBytes)
		}
		if name == visualcapture.ManifestFile {
			for _, line := range bytes.Split(data, []byte("\n")) {
				if len(computerevent.DetectRefusalSecrets(line)) != 0 {
					return nil, fmt.Errorf("capsule freeze: visual evidence refuses secret content in %q", name)
				}
			}
		} else if err := refuseBinarySecrets(ctx, name, data); err != nil {
			return nil, err
		}
		sum := sha256.Sum256(data)
		files = append(files, visualcapture.EvidenceFile{Name: name, Size: int64(len(data)), SHA256: hex.EncodeToString(sum[:])})
		contents[name] = data
	}
	evidence, err := buildVisualEvidence(contents[visualcapture.ManifestFile], files, contents, release, jobs)
	if err != nil {
		return nil, err
	}
	// Only a validated directory is written out: a refused one leaves no
	// partial evidence in the bundle area.
	target := filepath.Join(temporary, filepath.FromSlash(visualcapture.BundleDir))
	if err := os.MkdirAll(target, 0o700); err != nil {
		return nil, err
	}
	for name, data := range contents {
		if err := os.WriteFile(filepath.Join(target, name), data, 0o444); err != nil {
			return nil, err
		}
	}
	return evidence, nil
}

// refuseBinarySecrets scans a capture's bytes in windows, carrying the tail
// of the previous window so a secret straddling a boundary is still found
// (the release scan's rule for binary payloads).
func refuseBinarySecrets(ctx context.Context, name string, data []byte) error {
	const window, carry = 64 << 10, 512
	for start := 0; start < len(data); start += window {
		if err := ctx.Err(); err != nil {
			return err
		}
		from := max(start-carry, 0)
		if len(computerevent.DetectBinarySecrets(data[from:min(start+window, len(data))])) != 0 {
			return fmt.Errorf("capsule freeze: visual evidence refuses secret content in %q", name)
		}
	}
	return nil
}

// buildVisualEvidence validates a staged evidence directory (its manifest,
// files and their bytes) against the staged release and summarizes it for the
// bundle. Each capture must decode as a PNG of the viewport its manifest
// entry names; blank is recomputed from the image, never taken from the
// manifest (V11).
func buildVisualEvidence(manifestRaw []byte, files []visualcapture.EvidenceFile, contents map[string][]byte, release []FrozenReleaseFile, jobs []yaegikernel.JobView) (*VisualEvidence, error) {
	m, err := visualcapture.ValidateEvidence(manifestRaw, files, ReleaseFrontendDigest(release))
	if err != nil {
		return nil, fmt.Errorf("capsule freeze: %w", err)
	}
	manifestSHA := visualcapture.DigestHex(manifestRaw)
	out := &VisualEvidence{ManifestSHA256: manifestSHA, FrontendSHA256: m.Subject.FrontendSHA256}
	if job, ok := latestCaptureJob(jobs, manifestSHA); ok {
		out.CaptureReceipt = job.ReceiptRef
		out.AssetFailures = captureAssetFailures(job.Tail, manifestSHA)
	}
	for _, c := range m.Captures {
		blank, err := visualcapture.CheckPNG(contents[c.File], c.Width, c.Height)
		if err != nil {
			return nil, fmt.Errorf("capsule freeze: visual evidence %s (route %s): %w", c.File, c.Route, err)
		}
		out.Captures = append(out.Captures, VisualEvidenceCapture{
			Artifact: c.Artifact, Route: c.Route, MediaType: c.MediaType, Bytes: c.Bytes,
			Width: c.Width, Height: c.Height, Blank: blank,
		})
	}
	return out, out.Validate()
}

// latestCaptureJob is the newest capture job that ended done with a receipt
// and whose output names this manifest's digest (jobs are oldest first). A
// manifest written or rewritten after the job, or by hand, is in no job's
// output, so it gets no receipt (V17).
func latestCaptureJob(jobs []yaegikernel.JobView, manifestSHA256 string) (yaegikernel.JobView, bool) {
	for i := len(jobs) - 1; i >= 0; i-- {
		j := jobs[i]
		if visualcapture.IsCaptureJob(j.Argv) && j.State == "done" && j.ReceiptRef != "" && strings.Contains(j.Tail, manifestSHA256) {
			return j, true
		}
	}
	return yaegikernel.JobView{}, false
}

// captureAssetFailures reads the receipt job's failed asset answers from its
// summary, dropping any path the refusal scan flags: the bundle record is
// scanned as the manifest is.
func captureAssetFailures(tail, manifestSHA256 string) []visualcapture.AssetFailure {
	var out []visualcapture.AssetFailure
	for _, f := range visualcapture.ParseSummaryAssetFailures(tail, manifestSHA256) {
		if len(computerevent.DetectRefusalSecrets([]byte(f.Path))) == 0 {
			out = append(out, f)
		}
	}
	return out
}

func isHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil && strings.ToLower(s) == s
}
