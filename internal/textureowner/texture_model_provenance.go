package textureowner

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// HandleTextureModelProvenance serves GET /api/texture/documents/{id}/models:
// the models that actually served the document's runs, by desk role
// (docs/problems/model-provenance-unwired-2026-11-05.md). The source panel
// shows it, and the publish path snapshots it into the published version.
func (h *Handler) HandleTextureModelProvenance(w http.ResponseWriter, r *http.Request) {
	r = asOwnerRoute(r)
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	docID := extractDocID(r.URL.Path)
	if docID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "doc_id is required"})
		return
	}
	provenance, err := h.documentModelProvenance(r.Context(), ownerID, docID)
	if errors.Is(err, store.ErrNotFound) {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
		return
	}
	if err != nil {
		log.Printf("texture api: model provenance for %s: %v", docID, err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "model provenance unavailable"})
		return
	}
	writeAPIJSON(w, http.StatusOK, provenance)
}

// documentModelProvenance reads the document's trajectories (its own and
// every revision's) and asks the runtime for the served models of their
// runs.
func (h *Handler) documentModelProvenance(ctx context.Context, ownerID, docID string) (types.ModelProvenance, error) {
	if h == nil || h.Core == nil {
		return types.ModelProvenance{}, errors.New("runtime unavailable")
	}
	doc, err := h.getTextureDocument(ctx, ownerID, docID)
	if err != nil {
		return types.ModelProvenance{}, err
	}
	trajectories := []string{doc.TrajectoryID}
	if revs, err := h.listTextureRevisions(ctx, ownerID, docID, 500); err == nil {
		for _, rev := range revs {
			if id := strings.TrimSpace(rev.TrajectoryID); id != "" {
				trajectories = append(trajectories, id)
			}
		}
	}
	return h.Core.DocumentModelProvenance(ctx, ownerID, docID, trajectories)
}
