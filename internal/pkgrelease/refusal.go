package pkgrelease

import (
	"errors"
	"fmt"
)

// Refusal reasons. Each is a stable code a caller maps to a status without
// parsing prose (design sections 3.2, 3.4 and 16).
const (
	ReasonInvalid            = "package_release_invalid"
	ReasonBlobMismatch       = "blob_mismatch"
	ReasonBlobTooLarge       = "package_blob_too_large"
	ReasonContentNotConfined = "content_not_confined"
	ReasonTreeMismatch       = "package_tree_mismatch"
	ReasonReviewMismatch     = "review_patch_mismatch"
	ReasonParentMismatch     = "package_parent_tree_mismatch"
	ReasonParentUnverifiable = "package_parent_unverifiable"
	ReasonSecretFound        = "secret_found"
	ReasonManifestInvalid    = "package_manifest_invalid"
	ReasonKindsNeedPlatform  = "package_kinds_need_platform"
	ReasonUnchanged          = "package_unchanged"
	ReasonTrackMoved         = "track_moved"
	// ReasonRecordUnlisted: the record was stored and its listing was not
	// (a crash between the two writes), and the track has since moved, so
	// it can no longer be listed where it would have been.
	ReasonRecordUnlisted = "package_record_unlisted"
	// ReasonListingConflict: the listing's canonical id is already held by
	// an object that is not this listing.
	ReasonListingConflict = "package_listing_conflict"
	// Publisher-side (guest) refusals.
	ReasonChangeNotConfined = "package_change_not_confined"
	ReasonParentUnpublished = "package_parent_unpublished"
)

// Refusal is a typed refusal: Reason is the code, Detail the prose.
type Refusal struct {
	Reason string
	Detail string
}

func (r *Refusal) Error() string { return r.Reason + ": " + r.Detail }

func refuse(reason, format string, args ...any) *Refusal {
	return &Refusal{Reason: reason, Detail: fmt.Sprintf(format, args...)}
}

// ReasonOf names an error's refusal code, or "" when it is not a Refusal.
func ReasonOf(err error) string {
	var refusal *Refusal
	if errors.As(err, &refusal) {
		return refusal.Reason
	}
	return ""
}
