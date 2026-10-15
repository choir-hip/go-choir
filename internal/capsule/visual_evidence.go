package capsule

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

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
	// ended done in this run, the latest one; empty when no such job ran
	// (hand-written evidence, V17), which visual_captured does not accept.
	CaptureReceipt string                  `json:"capture_receipt,omitempty"`
	Captures       []VisualEvidenceCapture `json:"captures"`
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

// buildVisualEvidence validates a staged evidence directory (its manifest
// and files) against the staged release and summarizes it for the bundle.
func buildVisualEvidence(manifestRaw []byte, files []visualcapture.EvidenceFile, release []FrozenReleaseFile, jobs []yaegikernel.JobView) (*VisualEvidence, error) {
	m, err := visualcapture.ValidateEvidence(manifestRaw, files, ReleaseFrontendDigest(release))
	if err != nil {
		return nil, fmt.Errorf("capsule freeze: %w", err)
	}
	sum := sha256.Sum256(manifestRaw)
	out := &VisualEvidence{ManifestSHA256: hex.EncodeToString(sum[:]), FrontendSHA256: m.Subject.FrontendSHA256, CaptureReceipt: latestCaptureReceipt(jobs)}
	for _, c := range m.Captures {
		out.Captures = append(out.Captures, VisualEvidenceCapture{
			Artifact: c.Artifact, Route: c.Route, MediaType: c.MediaType, Bytes: c.Bytes,
			Width: c.Width, Height: c.Height, Blank: c.Blank,
		})
	}
	return out, out.Validate()
}

// latestCaptureReceipt is the receipt of the newest capture job that ended
// done (jobs are oldest first).
func latestCaptureReceipt(jobs []yaegikernel.JobView) string {
	for i := len(jobs) - 1; i >= 0; i-- {
		j := jobs[i]
		if visualcapture.IsCaptureJob(j.Argv) && j.State == "done" && j.ReceiptRef != "" {
			return j.ReceiptRef
		}
	}
	return ""
}

func isHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil && strings.ToLower(s) == s
}
