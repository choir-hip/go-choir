package platform

import (
	"context"
	"net/http"
	"strconv"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

// Release blobs at the host: the patches a package release record names
// (source.tree_patch, source.review_patch, parent_patch), served under the
// release's own visibility so a reader can rebuild and verify the release
// tree (choir package export, design 10.2):
//
//	GET /internal/platform/packages/releases/<id>/blobs/<sha>?owner_id=   one patch of a release the owner may see
//
// Failure modes, written before the code:
//
//	B1  a digest the record does not name is served, even one in the store
//	    (another release's patch, a media item): it answers as missing;
//	B2  a release the reader may not see (withdrawn, to anyone but its
//	    publisher) or a missing one is served or told apart: both 404;
//	B3  a path trick reaches the store: a non-sha256 id or digest, an
//	    encoded slash or dot segment, is refused before any read; the store
//	    is only ever addressed by the record's own digest;
//	B4  a blob over the 2 MiB cap is read or served: ReadVerified caps the
//	    read at pkgrelease.MaxBlobBytes and the proxy caps its fetch;
//	B5  stored bytes that are not the record's blob (hash or length) are
//	    served: they are re-verified and answer as missing;
//	B6  a guest, or a read naming no owner, reaches it: host only, owner
//	    required (packageReader).

// PackageBlobURL is the proxy's public route for one patch of a release.
func PackageBlobURL(releaseID, digest string) string {
	return "/api/catalog/releases/" + releaseID + "/blobs/" + digest
}

// namedBlob is the record's patch with digest, if the record names it.
func namedBlob(record pkgrelease.Record, digest string) (pkgrelease.Blob, bool) {
	blobs := []pkgrelease.Blob{record.Source.TreePatch, record.Source.ReviewPatch}
	if record.ParentPatch != nil {
		blobs = append(blobs, *record.ParentPatch)
	}
	for _, b := range blobs {
		if b.SHA256 != "" && b.SHA256 == digest {
			return b, true
		}
	}
	return pkgrelease.Blob{}, false
}

func (h *Handler) readPackageReleaseBlob(w http.ResponseWriter, r *http.Request, releaseID, digest string) {
	reader, ok := packageReader(w, r)
	if !ok {
		return
	}
	notFound := apiError{Error: "package blob not found"}
	if !computerevent.IsSHA256(releaseID) || !computerevent.IsSHA256(digest) {
		writeJSON(w, http.StatusNotFound, notFound)
		return
	}
	blob, ok := h.visibleReleaseBlob(r.Context(), reader, releaseID, digest)
	if !ok {
		writeJSON(w, http.StatusNotFound, notFound)
		return
	}
	raw, err := h.readPackageBlob(blob)
	if err != nil {
		writeJSON(w, http.StatusNotFound, notFound)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(len(raw)))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}

// visibleReleaseBlob is the patch with digest of a release reader may see.
func (h *Handler) visibleReleaseBlob(ctx context.Context, reader, releaseID, digest string) (pkgrelease.Blob, bool) {
	_, record, err := h.findPackageRecord(ctx, reader, releaseID)
	if err != nil {
		return pkgrelease.Blob{}, false
	}
	blob, ok := namedBlob(record, digest)
	if !ok {
		return pkgrelease.Blob{}, false
	}
	listings, _, err := h.trackListings(ctx, record.Publisher.OwnerID, record.Package.Name)
	if err != nil {
		return pkgrelease.Blob{}, false
	}
	listing, ok := listings[releaseID]
	if !ok || !h.listingVisible(listing, reader) {
		return pkgrelease.Blob{}, false
	}
	return blob, true
}
