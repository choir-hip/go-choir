package agentcore

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

// visualEvidenceListing is the read-only answer of
// GET /api/computers/<id>/self-development/operations/<op>/visual-evidence:
// which evidence digests belong to the operation's frozen bundle, with the
// capture receipt that vouches for them. It is the drill's instrument and the
// seam a Texture capsule_evidence embed resolves its operation id through;
// the bytes come from GET /api/assets/sha256/<hex>
// (docs/design/visual-evidence-slice-2026-10-15.md section 10.3).
type visualEvidenceListing struct {
	OperationID    string                  `json:"operation_id"`
	ComputerID     string                  `json:"computer_id"`
	State          string                  `json:"state"`
	BundleDigest   string                  `json:"bundle_digest"`
	ManifestSHA256 string                  `json:"manifest_sha256"`
	FrontendSHA256 string                  `json:"frontend_sha256"`
	CaptureReceipt string                  `json:"capture_receipt,omitempty"`
	Captures       []visualEvidenceListRow `json:"captures"`
}

type visualEvidenceListRow struct {
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
	operation, err := h.rt.selfdevOperations.Get(r.Context(), computerID, operationID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "self-development operation not found"})
		return
	}
	if operation.State == selfdev.StateRequested || operation.State == selfdev.StateExecuting || operation.BundleDigest == "" {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "operation is " + operation.State + "; visual evidence exists only once the candidate is frozen"})
		return
	}
	if strings.TrimSpace(h.rt.selfdevUpdaterRoot) == "" {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "self-development bundle store unavailable"})
		return
	}
	if !computerevent.IsSHA256(operation.BundleDigest) {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "operation bundle digest is malformed"})
		return
	}
	bundle, status, err := readBoundSelfDevelopmentBundle(filepath.Join(h.rt.selfdevUpdaterRoot, "incoming", operation.BundleDigest), operation)
	if err != nil {
		writeAPIJSON(w, status, apiError{Error: err.Error()})
		return
	}
	visual := bundle.VisualEvidence
	if visual == nil || len(visual.Captures) == 0 {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "the candidate carries no visual evidence"})
		return
	}
	store := h.rt.imageStore()
	out := visualEvidenceListing{
		OperationID: operation.OperationID, ComputerID: operation.ComputerID, State: operation.State,
		BundleDigest: operation.BundleDigest, ManifestSHA256: visual.ManifestSHA256, FrontendSHA256: visual.FrontendSHA256,
		CaptureReceipt: visual.CaptureReceipt, Captures: make([]visualEvidenceListRow, 0, len(visual.Captures)),
	}
	for _, capture := range visual.Captures {
		digest := strings.TrimPrefix(capture.Artifact, "artifact:sha256:")
		out.Captures = append(out.Captures, visualEvidenceListRow{
			Artifact: capture.Artifact, Digest: digest, Route: capture.Route, Width: capture.Width, Height: capture.Height,
			Bytes: capture.Bytes, Blank: capture.Blank, Resolvable: store.Has(digest), AssetURL: assetPathPrefix + digest,
		})
	}
	writeAPIJSON(w, http.StatusOK, out)
}
