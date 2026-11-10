package textureowner

import (
	"net/http"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/styleguide"
)

// The style read surface (lane G). A style guide is a Texture document titled
// "Style Guide" on the owner's computer. Texture is the only reader of its
// revisions: this route parses the current revision into the closed design in
// internal/styleguide and serves only that, never the guide's text. Pass 1
// reads at read time only; no desk prompt reads the guide, and the route is
// the owner's (an in-process desk call is refused).

const stylePath = "/api/texture/style"

type styleAnswer struct {
	// Source is "document" when a guide's current revision was read, else
	// "default" (no guide, or one that could not be used; Reason says which).
	Source     string            `json:"source"`
	Reason     string            `json:"reason,omitempty"`
	DocID      string            `json:"doc_id,omitempty"`
	RevisionID string            `json:"revision_id,omitempty"`
	Design     styleguide.Design `json:"design"`
	Ignored    []string          `json:"ignored,omitempty"`
}

// HandleStyle serves GET /api/texture/style.
func (h *Handler) HandleStyle(w http.ResponseWriter, r *http.Request) {
	r = asOwnerRoute(r)
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if strings.TrimSpace(r.RemoteAddr) == "" {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "the style guide is read by the owner's reader; a desk does not read it"})
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
	writeAPIJSON(w, http.StatusOK, h.readStyle(r, ownerID))
}

// readStyle never fails: any problem is the default with a reason.
func (h *Handler) readStyle(r *http.Request, ownerID string) styleAnswer {
	def := styleAnswer{Source: "default", Design: styleguide.Default()}
	ctx := r.Context()
	computerID := strings.TrimSpace(h.Core.TextureComputerID())
	docs, err := h.Store.ListDocumentsByScope(asTextureReader(ctx), ownerID, computerID, algorithm2DocLimit)
	if err != nil {
		def.Reason = "the documents could not be listed"
		return def
	}
	sort.SliceStable(docs, func(i, j int) bool { return docs[i].UpdatedAt.After(docs[j].UpdatedAt) })
	for _, d := range docs {
		if d.ArchivedAt != nil || !styleguide.IsGuideTitle(d.Title) {
			continue
		}
		revID, text, err := h.headText(ctx, ownerID, d)
		if err != nil {
			def.Reason = "the guide's current revision could not be read"
			return def
		}
		if len(text) > styleguide.MaxGuideBytes {
			def.Reason = "the guide is larger than the limit"
			return def
		}
		design, ignored := styleguide.Parse(text)
		return styleAnswer{Source: "document", DocID: d.DocID, RevisionID: revID, Design: design, Ignored: ignored}
	}
	def.Reason = "no style guide document"
	return def
}
