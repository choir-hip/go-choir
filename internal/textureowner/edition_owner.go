package textureowner

import (
	"net/http"
	"strings"
	"time"
)

// The edition read surface (Autoradio lane S, item 4). The listener's edition
// is the Texture titled "My edition" (internal/editiondoc). This route finds
// it for the owner's reader, the way the style route finds the Style Guide;
// it answers ids and times, never the text (the owner reads the document
// through the document routes, and Texture stays the one custodian of its
// revisions). Ways it can fail, written before the code:
//
//	D1 It finds another owner's edition, or an archived one.
//	D2 Two documents with the title answer unstably: the most recently updated
//	   one wins, every time.
//	D3 No edition answers an error, so a first-run app cannot tell "not made
//	   yet" from a failure: it answers 200 with found false.
//	D4 It serves the document's text, or answers a desk, an anonymous caller,
//	   any method but GET, or a cached copy.

const editionPath = "/api/texture/edition"

type editionAnswer struct {
	Found      bool       `json:"found"`
	DocID      string     `json:"doc_id,omitempty"`
	RevisionID string     `json:"revision_id,omitempty"`
	UpdatedAt  *time.Time `json:"updated_at,omitempty"`
}

// HandleEdition serves GET /api/texture/edition.
func (h *Handler) HandleEdition(w http.ResponseWriter, r *http.Request) {
	r = asOwnerRoute(r)
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if strings.TrimSpace(r.RemoteAddr) == "" {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "the edition is found by the owner's reader; a desk does not"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	if h == nil || h.Core == nil || h.Store == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "texture store unavailable"})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	computerID := strings.TrimSpace(h.Core.TextureComputerID())
	// The same function the Autoradio desk finds the notebook with, so the
	// phone and the DJ cannot be given two documents.
	d, found, _, err := h.notebookDoc(asTextureReader(r.Context()), ownerID, computerID)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "the documents could not be listed"})
		return
	}
	if found {
		writeAPIJSON(w, http.StatusOK, editionAnswer{Found: true, DocID: d.DocID, RevisionID: d.CurrentRevisionID, UpdatedAt: utcPtr(d.UpdatedAt)})
		return
	}
	writeAPIJSON(w, http.StatusOK, editionAnswer{})
}

func utcPtr(t time.Time) *time.Time { u := t.UTC(); return &u }
