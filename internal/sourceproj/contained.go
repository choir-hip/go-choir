package sourceproj

import (
	"context"
	"errors"
)

// Contained reports whether patch (authored against base, producing
// patchedTreeID) is already in the booted base's tree. See the containment
// test and its failure modes in the U1 build notes (C1 to C9):
//
//  1. every path the patch touches has, in booted, exactly the entry the
//     patch leaves (same blob and mode, or absent where the patch deletes);
//  2. failing that, the patch reverse-applies cleanly to booted and the
//     forward patch then reproduces booted's touched entries.
//
// An error means git failed, not "no". A patch that does not apply the
// reverse way is simply not contained.
func Contained(ctx context.Context, repo, booted, base string, patch []byte, patchedTreeID string) (bool, error) {
	touched, err := treeDiffPaths(ctx, repo, base, patchedTreeID, nil)
	if err != nil {
		return false, err
	}
	if len(touched) == 0 {
		return true, nil // a patch that changes nothing is in every base
	}
	differing, err := treeDiffPaths(ctx, repo, patchedTreeID, booted, touched)
	if err != nil {
		return false, err
	}
	if len(differing) == 0 {
		return true, nil
	}
	reversed, err := patchedTree(ctx, repo, booted, patch, true)
	if err != nil {
		if errors.Is(err, ErrPatchDoesNotApply) {
			return false, nil
		}
		return false, err
	}
	forward, err := patchedTree(ctx, repo, reversed, patch, false)
	if err != nil {
		if errors.Is(err, ErrPatchDoesNotApply) {
			return false, nil
		}
		return false, err
	}
	differing, err = treeDiffPaths(ctx, repo, forward, booted, touched)
	if err != nil {
		return false, err
	}
	return len(differing) == 0, nil
}
