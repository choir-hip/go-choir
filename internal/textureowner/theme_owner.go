package textureowner

import (
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/themedoc"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The theme read surface (Autoradio lane S, item 3;
// docs/design/autoradio-app-ui-2026-11-10.md section 5, server gap 3). A
// theme is a Texture holding one fenced choir-theme JSON block. Like the
// style route, Texture is the only reader of its revisions: this route
// parses the revision into the closed, validated theme in internal/themedoc
// and serves only that, never the document's prose, and it is the owner's
// (an in-process desk call is refused). Ways it can fail, written before the
// code (the parser's own list is in internal/themedoc):
//
//	R1 Another owner's document is served, or its existence is told apart
//	   from a missing one.
//	R2 An invalid theme is served as a theme (it answers 422 with the reason).
//	R3 ?rev= names a revision of a different document and is served as this one.
//	R4 A desk or an anonymous caller reads it; any method but GET.
//	R5 The answer is cached.

const themePath = "/api/texture/theme"

type themeAnswer struct {
	DocID      string         `json:"doc_id"`
	RevisionID string         `json:"revision_id"`
	Theme      themedoc.Theme `json:"theme"`
	Ignored    []string       `json:"ignored,omitempty"`
}

// HandleTheme serves GET /api/texture/theme?doc=<doc_id>[&rev=<revision_id>].
func (h *Handler) HandleTheme(w http.ResponseWriter, r *http.Request) {
	r = asOwnerRoute(r)
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if strings.TrimSpace(r.RemoteAddr) == "" {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "a theme is read by the owner's reader; a desk does not read it"})
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
	docID := strings.TrimSpace(r.URL.Query().Get("doc"))
	if docID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "doc is required"})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	ctx := r.Context()
	doc, err := h.getTextureDocument(asOwner(ctx), ownerID, docID)
	if err != nil || doc.ArchivedAt != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "no such theme document"}) // R1
		return
	}
	var rev types.Revision
	if want := strings.TrimSpace(r.URL.Query().Get("rev")); want != "" {
		rev, err = h.getTextureRevision(ctx, ownerID, want)
		if err != nil || rev.DocID != doc.DocID { // R3
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "no such theme revision"})
			return
		}
	} else {
		rev, err = h.getTextureRevision(ctx, ownerID, doc.CurrentRevisionID)
		if err != nil {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "the document has no readable revision"})
			return
		}
	}
	theme, ignored, err := themedoc.Parse(rev.Content)
	if err != nil { // R2
		writeAPIJSON(w, http.StatusUnprocessableEntity, apiError{Error: "not a valid theme: " + err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusOK, themeAnswer{DocID: doc.DocID, RevisionID: rev.RevisionID, Theme: theme, Ignored: ignored})
}
