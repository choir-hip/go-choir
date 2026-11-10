package textureowner

import (
	"net/http"
)

// HandleTextureDocumentPublished serves POST /api/texture/documents/{id}/published:
// the proxy telling this computer that a document of its owner was published
// (any mode). Publishing is done by the proxy and corpusd, not by the
// computer, so this is how a publisher's computer learns it is one. It arms
// the computer's polls of its corpusd feeds (notice_intake, oped_intake),
// idempotently. It writes no document, wakes no desk and resumes nothing.
func (h *Handler) HandleTextureDocumentPublished(w http.ResponseWriter, r *http.Request) {
	r = asOwnerRoute(r)
	if r.Method != http.MethodPost {
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
	if _, err := h.getTextureDocument(asOwner(r.Context()), ownerID, docID); err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
		return
	}
	h.ensurePlatformPolls(r.Context(), ownerID)
	w.WriteHeader(http.StatusNoContent)
}
