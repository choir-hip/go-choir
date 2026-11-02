package agentcore

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/visualcapture"
)

// VisualEvidenceListing is the read-only answer of
// GET /api/computers/<id>/self-development/operations/<op>/visual-evidence:
// which evidence digests belong to the operation's frozen bundle, with the
// capture receipt that vouches for them. It is the drill's instrument and the
// seam a Texture capsule_evidence embed resolves its operation id through;
// the bytes come from GET /api/assets/sha256/<hex>
// (docs/design/visual-evidence-slice-2026-10-15.md section 10.3).
type VisualEvidenceListing struct {
	OperationID string `json:"operation_id"`
	// TrajectoryID is the operation's trajectory, for the Texture evidence
	// embed (station TE); not part of the route's answer.
	TrajectoryID   string                  `json:"-"`
	ComputerID     string                  `json:"computer_id"`
	State          string                  `json:"state"`
	BundleDigest   string                  `json:"bundle_digest"`
	ManifestSHA256 string                  `json:"manifest_sha256"`
	FrontendSHA256 string                  `json:"frontend_sha256"`
	CaptureReceipt string                  `json:"capture_receipt,omitempty"`
	Captures       []VisualEvidenceCapture `json:"captures"`
	// PackagesTouched are the app packages the candidate's source patch
	// changes (frontend/src/packages/<name>/); PackagesUncaptured are those
	// with no package capture in the evidence, which the judge reads as
	// evidence against (docs/problems/
	// selfdev-visual-capture-misses-the-changed-app-2026-11-02.md).
	PackagesTouched    []string `json:"packages_touched,omitempty"`
	PackagesUncaptured []string `json:"packages_uncaptured,omitempty"`
	// Notes are the capture's own notes: scenes that failed, the cap, the
	// time budget.
	Notes []string `json:"notes,omitempty"`
}

// VisualEvidenceCapture is one capture row of the listing.
type VisualEvidenceCapture struct {
	Artifact string `json:"artifact"`
	Digest   string `json:"digest"`
	Route    string `json:"route"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Bytes    int64  `json:"bytes"`
	Blank    bool   `json:"blank"`
	// Resolvable says the computer's asset store holds the bytes; false after
	// a failed store write at Freeze (V22).
	Resolvable bool   `json:"resolvable"`
	AssetURL   string `json:"asset_url"`
	// Scene is a package capture's package, preset, width and state.
	Scene *visualcapture.Scene `json:"scene,omitempty"`
}

// listSelfDevelopmentVisualEvidence never mutates. It refuses before Freeze
// and whenever the bundle is not exactly the one the operation names (the
// source-patch export's binding rules), and it answers 404, not an empty
// list, for a candidate that carries no captures.
func (h *APIHandler) listSelfDevelopmentVisualEvidence(w http.ResponseWriter, r *http.Request, computerID, operationID string) {
	out, status, err := h.rt.candidateVisualEvidence(r.Context(), computerID, operationID)
	if err != nil {
		writeAPIJSON(w, status, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusOK, out)
}

// CandidateVisualEvidence is the listing as the Texture evidence embed reads
// it (station TE, docs/design/texture-evidence-embeds-te-2026-10-25.md): the
// digests come only from the operation's frozen bundle on this computer, so
// the embed's membership check is the listing's own binding.
// A Texture document embeds at most visualcapture.MaxRoutes captures, so
// the embed gets the route captures and then a few package captures, the
// opened document on desktop first (textureEmbedCaptures); the judge and
// the CLI read the whole listing.
func (rt *Runtime) CandidateVisualEvidence(ctx context.Context, computerID, operationID string) (VisualEvidenceListing, error) {
	out, _, err := rt.candidateVisualEvidence(ctx, computerID, operationID)
	if err == nil {
		out.Captures = textureEmbedCaptures(out.Captures, visualcapture.MaxRoutes)
	}
	return out, err
}

// textureEmbedCaptures keeps at most limit captures: every route capture,
// then package captures ranked desktop before phone and an opened document
// before other states, in capture order within a rank.
func textureEmbedCaptures(captures []VisualEvidenceCapture, limit int) []VisualEvidenceCapture {
	if len(captures) <= limit {
		return captures
	}
	rank := func(c VisualEvidenceCapture) int {
		if c.Scene == nil {
			return 0
		}
		r := 1
		if c.Scene.Viewport != visualcapture.ViewportDesktop {
			r += 2
		}
		if !strings.HasPrefix(c.Scene.State, "open-") {
			r++
		}
		return r
	}
	keep := make([]bool, len(captures))
	n := 0
	for r := 0; r <= 4 && n < limit; r++ {
		for i, c := range captures {
			if n < limit && !keep[i] && rank(c) == r {
				keep[i] = true
				n++
			}
		}
	}
	out := make([]VisualEvidenceCapture, 0, n)
	for i, c := range captures {
		if keep[i] {
			out = append(out, c)
		}
	}
	return out
}

func (rt *Runtime) candidateVisualEvidence(ctx context.Context, computerID, operationID string) (VisualEvidenceListing, int, error) {
	if rt == nil || rt.selfdevOperations == nil {
		return VisualEvidenceListing{}, http.StatusServiceUnavailable, errors.New("self-development operations unavailable")
	}
	operation, err := rt.selfdevOperations.Get(ctx, computerID, operationID)
	if err != nil {
		return VisualEvidenceListing{}, http.StatusNotFound, errors.New("self-development operation not found")
	}
	if operation.State == selfdev.StateRequested || operation.State == selfdev.StateExecuting || operation.BundleDigest == "" {
		return VisualEvidenceListing{}, http.StatusConflict, errors.New("operation is " + operation.State + "; visual evidence exists only once the candidate is frozen")
	}
	if strings.TrimSpace(rt.selfdevUpdaterRoot) == "" {
		return VisualEvidenceListing{}, http.StatusServiceUnavailable, errors.New("self-development bundle store unavailable")
	}
	if !computerevent.IsSHA256(operation.BundleDigest) {
		return VisualEvidenceListing{}, http.StatusConflict, errors.New("operation bundle digest is malformed")
	}
	bundle, status, err := readBoundSelfDevelopmentBundle(filepath.Join(rt.selfdevUpdaterRoot, "incoming", operation.BundleDigest), operation)
	if err != nil {
		return VisualEvidenceListing{}, status, err
	}
	touched := bundlePackagesTouched(filepath.Join(rt.selfdevUpdaterRoot, "incoming", operation.BundleDigest), bundle.SourcePatchSHA256)
	visual := bundle.VisualEvidence
	if visual == nil || len(visual.Captures) == 0 {
		msg := "the candidate carries no visual evidence"
		if len(touched) > 0 {
			msg += "; its patch changes the app package(s) " + strings.Join(touched, ", ") + ", which no capture shows"
		}
		return VisualEvidenceListing{}, http.StatusNotFound, errors.New(msg)
	}
	store := rt.imageStore()
	out := VisualEvidenceListing{
		OperationID: operation.OperationID, TrajectoryID: operation.TrajectoryID, ComputerID: operation.ComputerID, State: operation.State,
		BundleDigest: operation.BundleDigest, ManifestSHA256: visual.ManifestSHA256, FrontendSHA256: visual.FrontendSHA256,
		CaptureReceipt: visual.CaptureReceipt, Captures: make([]VisualEvidenceCapture, 0, len(visual.Captures)),
		PackagesTouched: touched, Notes: visual.Notes,
	}
	captured := map[string]bool{}
	for _, capture := range visual.Captures {
		digest := strings.TrimPrefix(capture.Artifact, "artifact:sha256:")
		out.Captures = append(out.Captures, VisualEvidenceCapture{
			Artifact: capture.Artifact, Digest: digest, Route: capture.Route, Width: capture.Width, Height: capture.Height,
			Bytes: capture.Bytes, Blank: capture.Blank, Resolvable: store.Has(digest), AssetURL: assetPathPrefix + digest,
			Scene: capture.Scene,
		})
		if capture.Scene != nil {
			captured[capture.Scene.Package] = true
		}
	}
	out.PackagesUncaptured = uncapturedPackages(touched, captured)
	return out, http.StatusOK, nil
}

// bundlePackagesTouched lists the app packages a frozen bundle's source
// patch changes; nothing when the bundle has no readable, bound patch.
func bundlePackagesTouched(bundleRoot, patchSHA string) []string {
	if strings.TrimSpace(patchSHA) == "" {
		return nil
	}
	patch, _, err := readBoundSourcePatch(filepath.Join(bundleRoot, "source.patch"), patchSHA)
	if err != nil {
		return nil
	}
	return visualcapture.TouchedPackages(visualcapture.PatchPaths(patch))
}

// uncapturedPackages are the touched packages no capture shows.
func uncapturedPackages(touched []string, captured map[string]bool) []string {
	var out []string
	for _, name := range touched {
		if !captured[name] {
			out = append(out, name)
		}
	}
	return out
}
