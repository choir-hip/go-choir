package platform

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

// PackageReleaseKind is the Store B object a published package release is
// (Library S2, docs/design/library-package-propagation-2026-10-26.md 3.1):
// immutable, insert-if-absent, filed under the publisher's owner and the
// release id.
const PackageReleaseKind objectgraph.ObjectKind = "choir.package_release"

// ReleaseListingKind is one version of a record's listing (design 4.2):
// its scopes, its state and its sequence number on the track. Each version
// is its own object, written insert-if-absent, naming the digest of the
// version before it.
const ReleaseListingKind objectgraph.ObjectKind = "choir.release_listing"

const packagesRoute = "/internal/platform/packages"

// PackageBaseStatus is the package base store in corpusd's health detail:
// FP0 reads the shallow flag without SSH before S2's proof (design 3.2,
// check 3).
type PackageBaseStatus struct {
	Store   string `json:"store,omitempty"`
	Shallow *bool  `json:"shallow,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ConfigurePackageBase names the host's object store for base-parent
// checks and reads its shallow flag once. An empty dir leaves base parents
// unverifiable (every one refuses).
func (h *Handler) ConfigurePackageBase(ctx context.Context, dir string) {}

// ConfigurePackageOwnerActs installs the owner-act policy package publish
// verifies its receipt under.
func (h *Handler) ConfigurePackageOwnerActs(authority *OwnerActAuthority) {}

// PackageReleaseView is one published package release with its current
// listing.
type PackageReleaseView struct {
	ReleaseID   string             `json:"release_id"`
	CanonicalID string             `json:"canonical_id"`
	PublishedAt string             `json:"published_at"`
	Published   bool               `json:"published,omitempty"`
	Record      json.RawMessage    `json:"record"`
	Listing     pkgrelease.Listing `json:"listing"`
}

// HandlePackages serves the package release routes. Every route is host
// only.
func (h *Handler) HandlePackages(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotImplemented, apiError{Error: "package releases are not built"})
}

// PackagePublishSubject is the owner-act subject of a package publish: the
// release id and the one scope S2 lists at.
func PackagePublishSubject(releaseID string) map[string]any { return nil }

// listingObject files one listing version as a Store B object.
func listingObject(ownerID, computerID string, listing pkgrelease.Listing, now time.Time) (objectgraph.Object, error) {
	return objectgraph.Object{}, errors.New("not built")
}
