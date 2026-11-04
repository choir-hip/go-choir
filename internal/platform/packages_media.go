package platform

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

// Release media at the host (docs/problems/library-releases-have-no-screenshots-2026-11-04.md).
// A record names its round's screenshots and at most one video
// (pkgrelease.Media); the proxy uploads the bytes it fetched from the
// publisher and verified, and publish re-reads and re-verifies every item
// before it lists the release. The bytes are served under the release's own
// visibility, at the proxy's public route:
//
//	PUT /internal/platform/packages/media/<sha>                             upload one item
//	GET /internal/platform/packages/releases/<id>/media/<sha>?owner_id=      one item of a release the owner may see

// PackageMediaURL is the proxy's public route for one item of a release.
func PackageMediaURL(releaseID, digest string) string {
	return "/api/catalog/releases/" + releaseID + "/media/" + digest
}

// PackageMediaView is one item as the catalog serves it: the record's item
// and the URL any signed-in owner fetches it from.
type PackageMediaView struct {
	pkgrelease.Media
	URL string `json:"url"`
}

// mediaViews is the record's media with URLs; an empty array, never null,
// so a gallery shows its "no screenshots" state.
func mediaViews(releaseID string, record pkgrelease.Record) []PackageMediaView {
	out := make([]PackageMediaView, 0, len(record.Media))
	for _, m := range record.Media {
		out = append(out, PackageMediaView{Media: m, URL: PackageMediaURL(releaseID, m.SHA256)})
	}
	return out
}

func (h *Handler) putPackageMedia(w http.ResponseWriter, r *http.Request, digest string) {
	if !computerevent.IsSHA256(digest) {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "package media: sha256 digest required"})
		return
	}
	store := h.contentStore(pkgrelease.MediaNamespace)
	store.maxBytes = pkgrelease.MaxMediaBytes
	switch err := store.Put(digest, r.Body); {
	case err == nil:
		writeJSON(w, http.StatusCreated, map[string]string{"digest": digest})
	case errors.Is(err, errContentDigestMismatch):
		writeJSON(w, http.StatusBadRequest, apiError{Error: "package media: digest mismatch"})
	case errors.Is(err, errContentTooLarge):
		writeJSON(w, http.StatusRequestEntityTooLarge, packageAPIError{
			Error:  fmt.Sprintf("package media: more than %d bytes (8 MiB)", pkgrelease.MaxMediaBytes),
			Reason: pkgrelease.ReasonBlobTooLarge,
		})
	default:
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "package media: write"})
	}
}

// readPackageMedia is one stored item, verified against the record's item.
func (h *Handler) readPackageMedia(item pkgrelease.Media) ([]byte, error) {
	raw, err := h.contentStore(pkgrelease.MediaNamespace).ReadVerified(item.SHA256, pkgrelease.MaxMediaBytes)
	if err != nil {
		return nil, &pkgrelease.Refusal{Reason: pkgrelease.ReasonBlobMismatch, Detail: "media " + item.Name + " (" + item.SHA256 + ") is not in the host content store"}
	}
	if err := pkgrelease.VerifyMedia(item, raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// verifyPackageMedia requires every item the record names to be stored and
// to be that item.
func (h *Handler) verifyPackageMedia(record pkgrelease.Record) error {
	for _, item := range record.Media {
		if _, err := h.readPackageMedia(item); err != nil {
			return err
		}
	}
	return nil
}

// readPackageReleaseMedia serves one item of a release the reader may see;
// an unseen release, or a digest the record does not name, answers as
// missing.
func (h *Handler) readPackageReleaseMedia(w http.ResponseWriter, r *http.Request, releaseID, digest string) {
	reader, ok := packageReader(w, r)
	if !ok {
		return
	}
	notFound := apiError{Error: "package media not found"}
	if !computerevent.IsSHA256(releaseID) || !computerevent.IsSHA256(digest) {
		writeJSON(w, http.StatusNotFound, notFound)
		return
	}
	item, ok := h.visibleReleaseMedia(r.Context(), reader, releaseID, digest)
	if !ok {
		writeJSON(w, http.StatusNotFound, notFound)
		return
	}
	// The bytes were fully verified at publish; a read re-checks only that
	// the store still holds them (hash and length), never a full decode a
	// reader could repeat.
	raw, err := h.contentStore(pkgrelease.MediaNamespace).ReadVerified(item.SHA256, pkgrelease.MaxMediaBytes)
	if err != nil || len(raw) != item.Bytes {
		writeJSON(w, http.StatusNotFound, notFound)
		return
	}
	w.Header().Set("Content-Type", item.MediaType)
	w.Header().Set("Content-Length", strconv.Itoa(len(raw)))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}

// visibleReleaseMedia is the item with digest of a release reader may see.
func (h *Handler) visibleReleaseMedia(ctx context.Context, reader, releaseID, digest string) (pkgrelease.Media, bool) {
	_, record, err := h.findPackageRecord(ctx, reader, releaseID)
	if err != nil {
		return pkgrelease.Media{}, false
	}
	item, ok := record.MediaItem(digest)
	if !ok {
		return pkgrelease.Media{}, false
	}
	listings, _, err := h.trackListings(ctx, record.Publisher.OwnerID, record.Package.Name)
	if err != nil {
		return pkgrelease.Media{}, false
	}
	listing, ok := listings[releaseID]
	if !ok || !h.listingVisible(listing, reader) {
		return pkgrelease.Media{}, false
	}
	return item, true
}
