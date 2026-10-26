package platform

import (
	"net/http"

	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

// Library S3 at corpusd (docs/design/library-package-propagation-2026-10-26.md
// 4.1, 4.2, 4.4, 15.1): share, withdraw and reinstate as listing versions.
// Not built: the tests pin the behavior first.

const listingsRoute = "/internal/platform/listings"

// trackPageSize is one page of a track read; reads page to the end.
var trackPageSize = 500

// ListingChange is one owner act on a listing: a scope change, or a state.
type ListingChange struct {
	Scopes       []string               `json:"scopes,omitempty"`
	SharedWith   *pkgrelease.SharedWith `json:"shared_with,omitempty"`
	State        string                 `json:"state,omitempty"`
	Reason       string                 `json:"reason,omitempty"`
	ConfirmPaths string                 `json:"confirm_paths,omitempty"`
}

// PackageListingRequest is the proxy's listing act.
type PackageListingRequest struct {
	OwnerID    string `json:"owner_id"`
	ComputerID string `json:"computer_id"`
	ListingChange
}

// PackageListingResult answers a listing act.
type PackageListingResult struct {
	ReleaseID string              `json:"release_id"`
	Changed   bool                `json:"changed"`
	Listing   pkgrelease.Listing  `json:"listing"`
	Preview   *pkgrelease.Preview `json:"preview,omitempty"`
}

// PackageListingRefusal is a refused listing act, with the widening's
// preview when it needs a confirmation.
type PackageListingRefusal struct {
	Error   string              `json:"error"`
	Reason  string              `json:"reason,omitempty"`
	Preview *pkgrelease.Preview `json:"preview,omitempty"`
}

// PackageListingSubject is the owner-act subject of a listing act.
func PackageListingSubject(releaseID string, change ListingChange) map[string]any {
	return map[string]any{"release_id": releaseID}
}

// PackageShareReadsMode is CHOIR_PACKAGE_SHARE_READS.
type PackageShareReadsMode string

// ParsePackageShareReads reads the switch value.
func ParsePackageShareReads(raw string) PackageShareReadsMode { return PackageShareReadsMode(raw) }

// SetPackageShareReads sets the mode once at startup.
func (h *Handler) SetPackageShareReads(mode PackageShareReadsMode) {}

// HandleListings serves PUT /internal/platform/listings/<record id>.
func (h *Handler) HandleListings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotImplemented, apiError{Error: "not built"})
}
