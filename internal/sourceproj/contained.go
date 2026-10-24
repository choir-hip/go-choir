package sourceproj

import "context"

// Contained reports whether patch (authored against base, producing
// patchedTreeID) is already in the booted base's tree. The test is exact: every
// path the patch touches has, in booted, the entry the patch leaves (same blob
// and mode, or absent where the patch deletes). See the containment test and its
// failure modes in the U1 build notes (C1 to C9).
//
// There is deliberately no looser second test. A reverse-apply, even with a
// forward round trip, only shows that the patch's after-image with its context
// appears somewhere in the file, which a duplicated block satisfies; it would
// retire a stack that is not upstreamed. Exactness gives up the case where main
// has since edited the same file elsewhere, and that false negative lands safely
// as stack_base_moved (an owner act or U4 rebases).
//
// An error means git failed, not "no".
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
	return len(differing) == 0, nil
}
