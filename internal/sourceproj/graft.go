package sourceproj

import (
	"context"
	"errors"
	"fmt"

	"github.com/yusefmosiah/go-choir/internal/packagepath"
)

// ErrPackagePath is the correctness refusal for a stack whose patch changes a
// package path (forced modularity M2, door 3,
// docs/design/forced-modularity-build-2026-11-04.md section 2). The stack
// carries platform operations only: an app exists on a computer as an
// installed module, never as source in the stack. It is a refusal like
// release skew, not a switch.
var ErrPackagePath = errors.New(packagepath.ReasonPackagePath)

// PackagePaths lists the paths a patch's tree change (base to tree) makes
// under a package root.
func PackagePaths(ctx context.Context, repo, base, tree string) ([]string, error) {
	touched, err := treeDiffPaths(ctx, repo, base, tree, nil)
	if err != nil {
		return nil, err
	}
	return packagepath.Classify(touched).Package, nil
}

// pathGraft is the re-materialization of a stack onto a moved base (S4):
// when no path the patch touches differs between base and booted, the graft
// is booted's tree with the patch's after-image at exactly those paths, byte
// for byte. A path the booted base also changed is not grafted (ok false, why
// names it), nor is an after-image that does not fit booted's tree (a
// directory and a file at one path). An error means git failed.
func pathGraft(ctx context.Context, repo, base, booted, tree string) (graft string, ok bool, why string, err error) {
	touched, err := treeDiffPaths(ctx, repo, base, tree, nil)
	if err != nil {
		return "", false, "", err
	}
	bootedTree, err := git(ctx, repo, nil, nil, "rev-parse", booted+"^{tree}")
	if err != nil {
		return "", false, "", err
	}
	if len(touched) > 0 {
		changed, err := treeDiffPaths(ctx, repo, base, booted, touched)
		if err != nil {
			return "", false, "", err
		}
		if len(changed) > 0 {
			return "", false, "the booted base also changed " + shownPaths(changed) + ", which the patch touches, so it is not grafted", nil
		}
	}
	refused := ""
	err = scratchIndex(ctx, repo, func(env []string) error {
		if _, err := git(ctx, repo, env, nil, "read-tree", booted); err != nil {
			return err
		}
		why, err := placeAfterImage(ctx, repo, env, base, tree, touched)
		if err != nil {
			return err
		}
		if why != "" {
			refused = "the patch's after-image does not fit the booted tree (" + why + "), so it is not grafted"
			return nil
		}
		out, err := git(ctx, repo, env, nil, "write-tree")
		if err != nil {
			return err
		}
		if !commitID.MatchString(out) {
			return fmt.Errorf("git wrote a malformed tree id %q", out)
		}
		graft = out
		return nil
	})
	if err != nil {
		return "", false, "", err
	}
	if refused != "" {
		return "", false, refused, nil
	}
	// Checked: the grafted paths hold the after-image, and nothing else
	// differs from booted.
	if len(touched) > 0 {
		// (An empty pathspec list would compare the whole trees.)
		if differ, err := treeDiffPaths(ctx, repo, tree, graft, touched); err != nil {
			return "", false, "", err
		} else if len(differ) > 0 {
			return "", false, "the graft does not hold the patch's after-image at " + differ[0], nil
		}
	}
	in := make(map[string]bool, len(touched))
	for _, p := range touched {
		in[p] = true
	}
	outside, err := treeDiffPaths(ctx, repo, bootedTree, graft, nil)
	if err != nil {
		return "", false, "", err
	}
	for _, p := range outside {
		if !in[p] {
			return "", false, "the graft changes " + p + ", which the patch does not touch", nil
		}
	}
	return graft, true, "", nil
}

// shownPaths names up to five paths.
func shownPaths(paths []string) string {
	return packagepath.Shown(paths, 5)
}
