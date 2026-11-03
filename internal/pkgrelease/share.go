package pkgrelease

import (
	"context"
	"fmt"
	"sort"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// Who sees a listing, and what publishing shows the owner before a release
// becomes public. The owner's ruling of 2026-11-03: "published releases mean
// published, for everyone to see. thats the whole point of publishing to the
// library. no such thing as published only to the computer that made it."
// Publishing is the one act that makes a release public; there is no share.

// Refusal reasons of the listing and publish acts.
const (
	// ReasonScopeNotBuilt: a scope this build cannot enforce (platform
	// before S4; named computers before a computer reads packages, S5).
	ReasonScopeNotBuilt = "scope_not_built"
	// ReasonConfirmPathsRequired: publishing makes the release public, so
	// it needs the owner's confirmation of the paths that become public
	// (--confirm-paths).
	ReasonConfirmPathsRequired = "confirm_paths_required"
	// ReasonConfirmPathsMismatch: the confirmation names other paths than
	// the release's.
	ReasonConfirmPathsMismatch = "confirm_paths_mismatch"
	// ReasonListingState: the act is not a transition from the listing's
	// state (a yanked release takes no owner act).
	ReasonListingState = "listing_state_refused"
	// ReasonListingMoved: another act wrote the listing's next version first.
	ReasonListingMoved = "listing_moved"
	// ReasonOwnerActReplayed: the act's receipt already wrote a version of
	// this listing; a receipt acts once (review 291 F4).
	ReasonOwnerActReplayed = "owner_act_replayed"
	// ReasonShareRetired: share is no longer an act. A published release is
	// public; the owner takes it down by withdrawing it.
	ReasonShareRetired = "share_retired"
)

// ShareRetiredDetail is what a share answers.
const ShareRetiredDetail = "share is retired: a published release is public, visible to and installable by every signed-in owner. " +
	"Publishing is the act that makes it public; to take a release down, withdraw it (choir package withdraw)"

// PublicNotice is what a publish says before anything becomes public.
func PublicNotice(pathsDigest string) string {
	return "publishing makes this release public: every signed-in owner can see it in the Library and install it. " +
		"What becomes public: every path listed in the preview and its content, the review diff, and the release record " +
		"(your owner id, this computer's id, the operation's receipts, the precommitment questions and the test command shown in the preview). " +
		"Confirm these paths with --confirm-paths " + pathsDigest
}

// VisibleTo reports whether reader may see the listing: its publisher
// always, in every state; every other signed-in owner while it is listed. A
// withdrawn, suspended or yanked release is its publisher's alone. The
// scope set does not narrow this (owner, 2026-11-03): an owner-only or
// shared listing written before the ruling is public too, a read-time rule
// with no migration. An empty reader sees nothing.
func (l Listing) VisibleTo(reader string) bool {
	if reader == "" {
		return false
	}
	return l.Track == reader || l.State == StateListed
}

// Preview is what becomes public in a publish (design 4.4): every path of
// the release's content, and the size of the review diff. PathsDigest is what
// --confirm-paths names; it binds the release id and the paths, so a
// confirmation for one release never confirms another.
type Preview struct {
	Paths            []string `json:"paths"`
	PathsDigest      string   `json:"paths_digest"`
	ReviewPatchBytes int      `json:"review_patch_bytes"`
	// The record's free text, public with it: the precommitment questions
	// and the declared test command. The digest binds them through the
	// release id, which hashes the whole record.
	Questions   []string `json:"precommitment_questions"`
	TestCommand string   `json:"test_command,omitempty"`
}

// PublishPreview is computed from the bytes the host stores: tree_patch
// applied to the empty tree, confined again to the root, every file path
// listed, and the secret scan run again over the lines it adds.
func PublishPreview(ctx context.Context, record Record, treePatch, reviewPatch []byte) (Preview, error) {
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
		return Preview{}, refuse(ReasonSecretFound, "the stored tree_patch adds secret content (%v); nothing is published", kinds)
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
	questions := []string{}
	for _, p := range record.Precommitments {
		questions = append(questions, p.Question)
	}
	return Preview{Paths: paths, PathsDigest: computerevent.DigestBytes(raw), ReviewPatchBytes: len(reviewPatch),
		Questions: questions, TestCommand: record.Declarations.TestCommand}, nil
}

// ValidTrack reports whether id has the shape of a track (an owner id).
func ValidTrack(id string) bool { return safeIdentity.MatchString(id) }
