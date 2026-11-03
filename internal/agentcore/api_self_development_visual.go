package agentcore

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
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
func (rt *Runtime) CandidateVisualEvidence(ctx context.Context, computerID, operationID string) (VisualEvidenceListing, error) {
	out, _, err := rt.candidateVisualEvidence(ctx, computerID, operationID)
	return out, err
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
	visual := bundle.VisualEvidence
	if visual == nil || len(visual.Captures) == 0 {
		return VisualEvidenceListing{}, http.StatusNotFound, errors.New("the candidate carries no visual evidence")
	}
	store := rt.imageStore()
	out := VisualEvidenceListing{
		OperationID: operation.OperationID, TrajectoryID: operation.TrajectoryID, ComputerID: operation.ComputerID, State: operation.State,
		BundleDigest: operation.BundleDigest, ManifestSHA256: visual.ManifestSHA256, FrontendSHA256: visual.FrontendSHA256,
		CaptureReceipt: visual.CaptureReceipt, Captures: make([]VisualEvidenceCapture, 0, len(visual.Captures)),
	}
	for _, capture := range visual.Captures {
		digest := strings.TrimPrefix(capture.Artifact, "artifact:sha256:")
		out.Captures = append(out.Captures, VisualEvidenceCapture{
			Artifact: capture.Artifact, Digest: digest, Route: capture.Route, Width: capture.Width, Height: capture.Height,
			Bytes: capture.Bytes, Blank: capture.Blank, Resolvable: store.Has(digest), AssetURL: assetPathPrefix + digest,
		})
	}
	return out, http.StatusOK, nil
}
