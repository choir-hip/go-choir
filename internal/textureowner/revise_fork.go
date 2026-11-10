package textureowner

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// ForkRequest makes a new document from one published version. Text, when
// present, is the owner's direction for the fork (a merge whose only source is
// the version); without it the fork is a verbatim copy and nothing wakes.
type ForkRequest struct {
	OwnerID         string
	Text            string
	Source          ReviseSource
	ClientRequestID string
}

// forkRequestWire is the JSON body of POST /api/texture/fork.
type forkRequestWire struct {
	Text            string       `json:"text,omitempty"`
	Source          ReviseSource `json:"source"`
	ClientRequestID string       `json:"client_request_id"`
}

// HandleTextureFork handles POST /api/texture/fork: a new document from a
// published version, the client's answer to a comment on a document it does
// not own.
func (h *Handler) HandleTextureFork(w http.ResponseWriter, r *http.Request) {
	r = asOwnerRoute(r)
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	// An in-process caller is a desk (server.GuestHostCaller marks it with an
	// empty RemoteAddr): only the Texture desk writes a Texture, and an
	// owner-authored revision must not be a door for another desk (contract 1, R11).
	if strings.TrimSpace(r.RemoteAddr) == "" {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "a revise is an owner act; only the Texture desk writes a Texture, so address the Texture desk"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	var wire forkRequestWire
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, reviseMaxBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	receipt, err := h.forkDocument(r.Context(), ForkRequest{
		OwnerID: ownerID, Text: wire.Text, Source: wire.Source,
		ClientRequestID: firstNonEmpty(wire.ClientRequestID, r.Header.Get("Idempotency-Key")),
	})
	if err != nil {
		writeOwnerRevisionRefusal(w, err)
		return
	}
	writeAPIJSON(w, http.StatusAccepted, receipt.ownerRevisionResponse())
}

// forkDocument creates a document from a published version and commits its
// first revision through the same commit and the same validation as the
// revise call: with no text a take, with text a merge. Everything about the
// source is validated before the document exists, so a refused source creates
// nothing. A retry replays: the document and the revision both derive from the
// request id.
func (h *Handler) forkDocument(ctx context.Context, req ForkRequest) (ReviseReceipt, error) {
	ownerID := strings.TrimSpace(req.OwnerID)
	if ownerID == "" {
		return ReviseReceipt{}, refuseOwnerRevision(http.StatusUnauthorized, "authentication required")
	}
	clientRequestID := strings.TrimSpace(req.ClientRequestID)
	if clientRequestID == "" || len(clientRequestID) > 256 || strings.HasPrefix(clientRequestID, textureRootClientRequestPrefix) {
		return ReviseReceipt{}, refuseOwnerRevision(http.StatusBadRequest, "a valid client_request_id is required")
	}
	revise := ReviseRequest{OwnerID: ownerID, Text: req.Text, Sources: []ReviseSource{req.Source}, ClientRequestID: clientRequestID}
	revise.forkedFrom = true
	if _, err := classifyRevise(revise); err != nil {
		return ReviseReceipt{}, err
	}
	if strings.TrimSpace(req.Source.PublicationVersionID) == "" || strings.TrimSpace(req.Source.ManifestHash) == "" {
		return ReviseReceipt{}, refuseOwnerRevision(http.StatusBadRequest, "the source needs a publication_version_id and a manifest_hash")
	}
	resolved, err := h.resolveReviseSources(ctx, revise.Sources)
	if err != nil {
		return ReviseReceipt{}, err
	}
	if h.Core == nil || strings.TrimSpace(h.Core.TextureComputerID()) == "" {
		return ReviseReceipt{}, refuseOwnerRevision(http.StatusServiceUnavailable, "computer identity unavailable")
	}
	v := resolved[0].Version
	title := firstNonEmpty(strings.TrimSpace(v.Title), "Fork")
	created, err := h.startLifecycleTextureDocument(asOwner(ctx), ownerID, strings.TrimSpace(h.Core.TextureComputerID()), clientRequestID, title, strings.TrimSpace(v.Content))
	if err != nil {
		if errors.Is(err, store.ErrLifecycleCommandConflict) {
			return ReviseReceipt{}, refuseOwnerRevision(http.StatusConflict, "client_request_id was used for another request")
		}
		log.Printf("texture api: fork document: %v", err)
		return ReviseReceipt{}, refuseOwnerRevision(http.StatusBadRequest, "fork rejected")
	}
	revise.DocID = created.DocID
	receipt, err := h.reviseDocument(ctx, revise)
	if err != nil || strings.TrimSpace(req.Text) != "" {
		return receipt, err
	}
	// A verbatim fork wakes no desk, so nobody will serve the new document's
	// initial work item: the take that answers it closes it. A fork with text
	// leaves it open for the desk. The command id is the work item's, so a
	// retry settles once.
	settle := types.SettleLifecycleWorkRequest{
		OwnerID: ownerID, ComputerID: strings.TrimSpace(h.Core.TextureComputerID()),
		CommandID: "fork-settle:" + created.TargetWorkItemID, TrajectoryID: created.TrajectoryID,
		WorkItemID: created.TargetWorkItemID, ActingAgentID: created.TargetAgentID, ResultRef: receipt.RevisionID,
	}
	if settle.CommandDigest, err = store.ComputeSettleLifecycleWorkDigest(settle); err != nil {
		return ReviseReceipt{}, refuseOwnerRevision(http.StatusInternalServerError, "failed to close the fork's initial work")
	}
	if _, err := h.Store.SettleLifecycleWork(asOwner(ctx), settle); err != nil {
		log.Printf("texture api: close fork work %s: %v", created.TargetWorkItemID, err)
		return ReviseReceipt{}, refuseOwnerRevision(http.StatusInternalServerError, "failed to close the fork's initial work")
	}
	return receipt, nil
}
