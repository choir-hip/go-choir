package pkgrelease

import (
	"context"
	"fmt"
	"sort"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// Library S3 (docs/design/library-package-propagation-2026-10-26.md 4.1,
// 4.4): who a listing's scope set admits, what counts as a widening, and
// what a widening shows the owner before anything leaves.

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

func (l Listing) hasScope(scope string) bool {
	for _, s := range l.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// VisibleTo reports whether reader may see the listing under its scope
// set (design 4.1): the track's owner always; every reader of a public
// listing; a named owner of a shared one. Nobody else, a platform admin
// included: the platform scope is S4's and is never admitted here.
func (l Listing) VisibleTo(reader string) bool {
	if reader == "" {
		return false
	}
	if l.Track == reader || l.hasScope(ScopePublic) {
		return true
	}
	if l.hasScope(ScopeShared) {
		for _, owner := range l.SharedWith.Owners {
			if owner == reader {
				return true
			}
		}
	}
	return false
}

// Widens reports whether to admits a reader from does not: a widening
// shows its paths, needs their confirmation and runs the secret scan again
// (design 4.4). Taking readers away, or keeping the same ones, never does.
func Widens(from, to Listing) bool {
	if to.hasScope(ScopePublic) && !from.hasScope(ScopePublic) {
		return true
	}
	if to.hasScope(ScopeShared) {
		for _, owner := range to.SharedWith.Owners {
			if !from.VisibleTo(owner) {
				return true
			}
		}
	}
	return false
}

// SameAudience reports whether two listings admit the same readers by the
// same scope set.
func SameAudience(a, b Listing) bool {
	a, b = a.Normalize(), b.Normalize()
	return fmt.Sprint(a.Scopes) == fmt.Sprint(b.Scopes) && fmt.Sprint(a.SharedWith.Owners) == fmt.Sprint(b.SharedWith.Owners) &&
		fmt.Sprint(a.SharedWith.Computers) == fmt.Sprint(b.SharedWith.Computers)
}

// Preview is what leaves in a widening (design 4.4): every path of the
// release's content, and the size of the review diff. PathsDigest is what
// --confirm-paths names; it binds the release id and the paths, so a
// confirmation for one release never confirms another.
type Preview struct {
	Paths            []string `json:"paths"`
	PathsDigest      string   `json:"paths_digest"`
	ReviewPatchBytes int      `json:"review_patch_bytes"`
}

// WideningPreview is computed from the bytes the host stores: tree_patch
// applied to the empty tree, confined again to the root, every file path
// listed, and the secret scan run again over the lines it adds.
func WideningPreview(ctx context.Context, record Record, treePatch, reviewPatch []byte) (Preview, error) {
	record = record.Normalize()
	releaseID, err := record.ReleaseID()
	if err != nil {
		return Preview{}, err
	}
	if err := VerifyBlob(record.Source.TreePatch, treePatch); err != nil {
		return Preview{}, err
	}
	if err := VerifyBlob(record.Source.ReviewPatch, reviewPatch); err != nil {
		return Preview{}, err
	}
	if kinds := AddedSecrets(treePatch); len(kinds) > 0 {
		return Preview{}, refuse(ReasonSecretFound, "the stored tree_patch adds secret content (%v); nothing is widened", kinds)
	}
	s, err := newScratch(ctx, "")
	if err != nil {
		return Preview{}, fmt.Errorf("pkgrelease: scratch repository: %w", err)
	}
	defer s.close()
	full, err := s.apply(ctx, EmptyTree, treePatch)
	if err != nil {
		return Preview{}, refuse(ReasonInvalid, "the stored tree_patch does not apply to the empty tree: %v", err)
	}
	if err := s.confined(ctx, full, record.Source.Root); err != nil {
		return Preview{}, err
	}
	entries, err := s.entries(ctx, full)
	if err != nil {
		return Preview{}, fmt.Errorf("pkgrelease: list the release: %w", err)
	}
	paths := []string{}
	for _, e := range entries {
		if e.kind == "blob" {
			paths = append(paths, e.path)
		}
	}
	sort.Strings(paths)
	raw, err := computerevent.CanonicalJSON(map[string]any{"release_id": releaseID, "paths": paths})
	if err != nil {
		return Preview{}, err
	}
	return Preview{Paths: paths, PathsDigest: computerevent.DigestBytes(raw), ReviewPatchBytes: len(reviewPatch)}, nil
}

// ValidTrack reports whether id has the shape of a track (an owner id).
func ValidTrack(id string) bool { return safeIdentity.MatchString(id) }

// Widened reports whether the listing admits any reader beyond the
// track's owner.
func (l Listing) Widened() bool {
	return l.hasScope(ScopeShared) || l.hasScope(ScopePublic) || l.hasScope(ScopePlatform)
}
