package textureowner

import (
	"net/http"
	"strings"
)

const (
	textureAPIPathPrefix       = "/api/texture/"
	textureDocumentsPathPrefix = "/api/texture/documents/"
	textureRevisionsPathPrefix = "/api/texture/revisions/"
)

// HandleTextureRouter dispatches all document and revision Texture routes.
func (h *Handler) HandleTextureRouter(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	switch path {
	case "/api/texture/diff":
		h.HandleTextureDiff(w, r)
		return
	case "/api/texture/files/open":
		h.HandleTextureOpenFile(w, r)
		return
	case "/api/texture/markdown-lineage/import":
		h.HandleTextureImportMarkdownLineage(w, r)
		return
	}
	if strings.HasPrefix(path, textureRevisionsPathPrefix) {
		if strings.HasSuffix(path, "/blame") {
			h.HandleTextureBlame(w, r)
			return
		}
		h.HandleTextureRevision(w, r)
		return
	}
	if strings.HasPrefix(path, textureDocumentsPathPrefix) {
		rest := strings.TrimPrefix(path, textureDocumentsPathPrefix)
		switch {
		case strings.HasSuffix(rest, "/revisions"):
			h.HandleTextureRevisions(w, r)
		case strings.HasSuffix(rest, "/manifest"):
			h.HandleTextureEnsureManifest(w, r)
		case strings.HasSuffix(rest, "/stream"):
			h.HandleTextureDocumentStream(w, r)
		case strings.HasSuffix(rest, "/revise"):
			h.HandleTextureAgentRevision(w, r)
		case strings.HasSuffix(rest, "/cancel"):
			h.HandleTextureCancelAgentRevision(w, r)
		case strings.HasSuffix(rest, "/compare"):
			h.HandleTextureSemanticCompare(w, r)
		case strings.HasSuffix(rest, "/merge-preview"):
			h.HandleTextureMergePreview(w, r)
		case strings.HasSuffix(rest, "/accept-merge"):
			h.HandleTextureAcceptMerge(w, r)
		case strings.HasSuffix(rest, "/restore"):
			h.HandleTextureRestoreRevision(w, r)
		case strings.HasSuffix(rest, "/diagnosis"):
			h.HandleTextureDiagnosis(w, r)
		case strings.HasSuffix(rest, "/export"):
			h.HandleTextureExportDocument(w, r)
		case strings.HasSuffix(rest, "/agent-revision"):
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "texture endpoint not found"})
		case strings.HasSuffix(rest, "/history"):
			h.HandleTextureHistory(w, r)
		default:
			h.HandleTextureDocument(w, r)
		}
		return
	}
	writeAPIJSON(w, http.StatusNotFound, apiError{Error: "texture endpoint not found"})
}

// HandleTextureDocumentsRoot routes create and list requests.
func (h *Handler) HandleTextureDocumentsRoot(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.HandleTextureCreateDocument(w, r)
	case http.MethodGet:
		h.HandleTextureListDocuments(w, r)
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}
