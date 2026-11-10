package textureowner

import (
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
)

const (
	textureAPIPathPrefix       = "/api/texture/"
	textureDocumentsPathPrefix = "/api/texture/documents/"
	textureRevisionsPathPrefix = "/api/texture/revisions/"
)

// HandleTextureRouter dispatches all document and revision Texture routes.
func (h *Handler) HandleTextureRouter(w http.ResponseWriter, r *http.Request) {
	r = asOwnerRoute(r)
	path := r.URL.Path
	switch path {
	case "/api/texture/files/open":
		h.HandleTextureOpenFile(w, r)
		return
	case "/api/texture/markdown-lineage/import":
		h.HandleTextureImportMarkdownLineage(w, r)
		return
	case "/api/texture/algorithm2/ledger", "/api/texture/algorithm2/proposals", "/api/texture/algorithm2/round":
		h.HandleAlgorithm2(w, r)
		return
	case "/api/texture/fork":
		h.HandleTextureFork(w, r)
		return
	case "/api/texture/lifecycle-documents":
		h.HandleTextureLifecycleCreate(w, r)
		return
	case relatedReviewPath:
		h.HandleTextureRelatedReview(w, r)
		return
	}
	if strings.HasPrefix(path, algorithm2ProposalsPrefix) {
		h.HandleAlgorithm2(w, r)
		return
	}
	if strings.HasPrefix(path, agentcore.TextureLifecyclePathPrefix) {
		// The lifecycle and engineering-evidence projections the Texture UI
		// reads (MA3.10): Texture serves them; the UI no longer calls the
		// owner root /api/trajectories.
		agentcore.NewAPIHandler(h.Core).ServeTextureLifecycleProjection(w, r)
		return
	}
	if strings.HasPrefix(path, textureRevisionsPathPrefix) {
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
		case strings.HasSuffix(rest, "/events"):
			h.HandleTextureDocumentEvents(w, r)
		case strings.HasSuffix(rest, "/source-open"):
			h.HandleTextureSourceOpen(w, r)
		case strings.HasSuffix(rest, "/research") || strings.Contains(rest, "/research/"):
			h.HandleTextureStoryResearch(w, r)
		case strings.HasSuffix(rest, "/revise"):
			h.HandleTextureAgentRevision(w, r)
		case strings.HasSuffix(rest, "/restore"):
			h.HandleTextureRestoreRevision(w, r)
		case strings.HasSuffix(rest, "/diagnosis"):
			h.HandleTextureDiagnosis(w, r)
		case strings.HasSuffix(rest, "/models"):
			h.HandleTextureModelProvenance(w, r)
		case strings.HasSuffix(rest, "/export"):
			h.HandleTextureExportDocument(w, r)
		case strings.HasSuffix(rest, "/history"):
			h.HandleTextureHistory(w, r)
		case strings.Contains(strings.TrimSuffix(rest, "/"), "/"):
			// An unlisted sub-route under a document is not the document.
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "texture endpoint not found"})
		default:
			h.HandleTextureDocument(w, r)
		}
		return
	}
	writeAPIJSON(w, http.StatusNotFound, apiError{Error: "texture endpoint not found"})
}

// HandleTextureDocumentsRoot routes create and list requests.
func (h *Handler) HandleTextureDocumentsRoot(w http.ResponseWriter, r *http.Request) {
	r = asOwnerRoute(r)
	switch r.Method {
	case http.MethodPost:
		h.HandleTextureCreateDocument(w, r)
	case http.MethodGet:
		h.HandleTextureListDocuments(w, r)
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}
