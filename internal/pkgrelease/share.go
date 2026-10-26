package pkgrelease

import (
	"context"
	"errors"
)

// Library S3 (docs/design/library-package-propagation-2026-10-26.md 4.1,
// 4.4): sharing scopes and the widening preview. Not built: the tests pin
// the behavior first.

var errShareNotBuilt = errors.New("pkgrelease: S3 sharing is not built")

// S3 refusal reasons.
const (
	// ReasonScopeNotBuilt: a scope this build cannot enforce (platform
	// before S4; named computers before a computer reads packages, S5).
	ReasonScopeNotBuilt = "scope_not_built"
	// ReasonConfirmPathsRequired: a widening needs the owner's confirmation
	// of the paths that leave (--confirm-paths).
	ReasonConfirmPathsRequired = "confirm_paths_required"
	// ReasonConfirmPathsMismatch: the confirmation names other paths than
	// the release's.
	ReasonConfirmPathsMismatch = "confirm_paths_mismatch"
	// ReasonListingState: the act is not a transition from the listing's
	// state (a yanked release takes no owner act; only a listed release is
	// widened).
	ReasonListingState = "listing_state_refused"
	// ReasonListingMoved: another act wrote the listing's next version first.
	ReasonListingMoved = "listing_moved"
)

// VisibleTo reports whether reader may see the listing under its scope set.
func (l Listing) VisibleTo(reader string) bool { return false }

// Widens reports whether to admits a reader from does not.
func Widens(from, to Listing) bool { return false }

// Preview is what leaves in a widening (design 4.4).
type Preview struct {
	Paths            []string `json:"paths"`
	PathsDigest      string   `json:"paths_digest"`
	ReviewPatchBytes int      `json:"review_patch_bytes"`
}

// WideningPreview lists every path of the stored release and runs the
// secret scan again.
func WideningPreview(ctx context.Context, record Record, treePatch, reviewPatch []byte) (Preview, error) {
	return Preview{}, errShareNotBuilt
}
