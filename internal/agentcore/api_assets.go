package agentcore

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/imageblob"
)

// assetPathPrefix is the content-addressed asset route
// (docs/reports/texture-media-and-autoradio-design-2026-10-20.md section 4):
//
//	GET /api/assets/sha256/<64 lowercase hex>
//
// Owner-authenticated, HTTP range requests, Content-Type from the stored
// kind, nosniff, immutable caching, and a re-hash of the bytes on every read.
// The proxy forwards /api/* to the computer as a reverse proxy, so ranges and
// validators pass through untouched. Screenshots (visual evidence) are the
// first asset; audio and video kinds are typed in imageblob and refused until
// they land (texture media design T1, T4).
const assetPathPrefix = "/api/assets/sha256/"

// HandleAssets serves GET and HEAD for /api/assets/sha256/<hex>.
func (h *APIHandler) HandleAssets(w http.ResponseWriter, r *http.Request) {
	if _, err := authenticateUser(r); err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	digest, ok := strings.CutPrefix(r.URL.Path, assetPathPrefix)
	if !ok || h == nil || h.rt == nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "asset not found"})
		return
	}
	asset, err := h.rt.imageStore().Asset(digest)
	switch {
	case errors.Is(err, imageblob.ErrBadDigest), errors.Is(err, imageblob.ErrNotFound):
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "asset not found"})
		return
	case errors.Is(err, imageblob.ErrKindNotSupported):
		writeAPIJSON(w, http.StatusUnsupportedMediaType, apiError{Error: "this kind of asset is not served yet"})
		return
	case err != nil:
		// Corrupt or unreadable: the detail (a path, a digest of the swapped
		// bytes) stays in the log, not the response.
		log.Printf("runtime api: asset %s: %v", digest, err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "asset could not be read intact"})
		return
	}
	header := w.Header()
	header.Set("Content-Type", asset.ContentType)
	header.Set("X-Content-Type-Options", "nosniff")
	// The bytes never change under their digest; private keeps shared caches
	// (the proxy's, a CDN) from holding an owner's screenshot.
	header.Set("Cache-Control", "private, max-age=31536000, immutable")
	header.Set("ETag", `"sha256-`+asset.Digest+`"`)
	// An image opened as a document runs no script and loads nothing.
	header.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(asset.Data))
}
