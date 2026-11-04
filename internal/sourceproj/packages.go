package sourceproj

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// S6, the package lock (docs/design/s6-package-lock-build-2026-11-04.md;
// Library design 10.3, L17). A package root the applied stack touches is an
// entry with its own answer when the base moves, so a deploy that changes a
// package (an export to main) never strands self-development on other paths.
// Entries are derived from the cumulative patch and three subtree ids; there
// is no lock record to keep.

// PackagesRoot is the directory whose children are package roots: the same
// root as pkgrelease.PackagesRoot, which imports this package, so the
// constant is repeated here.
const PackagesRoot = "frontend/src/packages"

// PackageState is one entry's answer.
type PackageState string

const (
	// PackageAdopted: the stack is on the booted base; the root holds R.
	PackageAdopted PackageState = "adopted"
	// PackageUpstreamed: the booted base's subtree is R; the entry is retired
	// and the root is left as booted has it.
	PackageUpstreamed PackageState = "upstreamed"
	// PackageBaseMoved: the base moved and its subtree is still P, so R is
	// placed byte-identically.
	PackageBaseMoved PackageState = "package_base_moved"
	// PackageBehindBase: the base's subtree is neither P nor R. When the base
	// changed none of the paths the stack changed in the root, those paths
	// are grafted (S4's rule); otherwise R is placed whole and Conflict is set.
	PackageBehindBase PackageState = "package_behind_base"
)

// PackageEntry is one package root under the applied stack.
type PackageEntry struct {
	Root  string       `json:"root"`
	Name  string       `json:"name"`
	State PackageState `json:"state"`
	// Tree is R, ParentTree P (the stack base's) and BaseTree B' (the booted
	// base's) subtree ids at the root; empty means absent.
	Tree       string `json:"tree,omitempty"`
	ParentTree string `json:"parent_tree,omitempty"`
	BaseTree   string `json:"base_tree,omitempty"`
	// Conflict: the projection keeps this computer's R over a base change at
	// a path the stack also changed. Approving a candidate cast on it keeps R.
	Conflict bool `json:"conflict,omitempty"`
	// ReleaseID names the package install whose release tree is R, when one
	// is recorded (set by the runtime, not by reconcile).
	ReleaseID string `json:"release_id,omitempty"`
}

// PackageRootOf returns the package root a path lies under
// ("frontend/src/packages/<name>"), or "" for a shell path: a file directly
// in PackagesRoot is shell, and only a path below a named directory counts.
func PackageRootOf(path string) string {
	rest, ok := strings.CutPrefix(path, PackagesRoot+"/")
	if !ok {
		return ""
	}
	name, more, found := strings.Cut(rest, "/")
	if !found || name == "" || more == "" {
		return ""
	}
	return PackagesRoot + "/" + name
}

// subtreeID is the object id at root in a tree-ish, or "" when absent.
func subtreeID(ctx context.Context, repo, treeish, root string) string {
	out, err := git(ctx, repo, nil, nil, "rev-parse", "--verify", "--quiet", treeish+":"+root)
	if err != nil || !commitID.MatchString(out) {
		return ""
	}
	return out
}

// splitPackagePaths sorts touched paths into shell paths and package roots.
func splitPackagePaths(touched []string) (shell []string, roots []string, byRoot map[string][]string) {
	byRoot = map[string][]string{}
	for _, p := range touched {
		if root := PackageRootOf(p); root != "" {
			if _, seen := byRoot[root]; !seen {
				roots = append(roots, root)
			}
			byRoot[root] = append(byRoot[root], p)
			continue
		}
		shell = append(shell, p)
	}
	sort.Strings(roots)
	return shell, roots, byRoot
}

// packageEntries lists the entries of a stack on its own base (adopted).
func packageEntries(ctx context.Context, repo, base, tree string) ([]PackageEntry, error) {
	touched, err := treeDiffPaths(ctx, repo, base, tree, nil)
	if err != nil {
		return nil, err
	}
	_, roots, _ := splitPackagePaths(touched)
	var entries []PackageEntry
	for _, root := range roots {
		p := subtreeID(ctx, repo, base, root)
		entries = append(entries, PackageEntry{Root: root, Name: strings.TrimPrefix(root, PackagesRoot+"/"), State: PackageAdopted,
			Tree: subtreeID(ctx, repo, tree, root), ParentTree: p, BaseTree: p})
	}
	return entries, nil
}

// packageGraft is the re-materialization of a stack onto a moved base, with
// package roots as units. Shell paths keep S4: when no shell path the patch
// touches differs between base and booted, the graft holds the patch's
// after-image at exactly those paths, byte for byte; a shell path the booted
// base also changed is not grafted (ok false, why names it), nor is an
// after-image that does not fit booted's tree (a directory and a file at one
// path). Each package root gets its entry: upstreamed roots are left
// as booted has them; a root whose touched paths booted left alone is grafted
// path by path (byte-identical to R when booted's subtree is P); a root where
// booted changed a touched path gets R whole. The entries are returned also
// when ok is false. An error means git failed.
func packageGraft(ctx context.Context, repo, base, booted, tree string) (graft string, entries []PackageEntry, ok bool, why string, err error) {
	touched, err := treeDiffPaths(ctx, repo, base, tree, nil)
	if err != nil {
		return "", nil, false, "", err
	}
	bootedTree, err := git(ctx, repo, nil, nil, "rev-parse", booted+"^{tree}")
	if err != nil {
		return "", nil, false, "", err
	}
	shell, roots, byRoot := splitPackagePaths(touched)
	graftPaths := append([]string(nil), shell...)
	var whole []PackageEntry
	for _, root := range roots {
		e := PackageEntry{Root: root, Name: strings.TrimPrefix(root, PackagesRoot+"/"),
			Tree: subtreeID(ctx, repo, tree, root), ParentTree: subtreeID(ctx, repo, base, root), BaseTree: subtreeID(ctx, repo, booted, root)}
		switch {
		case e.BaseTree == e.Tree:
			e.State = PackageUpstreamed
		case e.BaseTree == e.ParentTree:
			e.State = PackageBaseMoved
			graftPaths = append(graftPaths, byRoot[root]...)
		default:
			e.State = PackageBehindBase
			changed, err := treeDiffPaths(ctx, repo, base, booted, byRoot[root])
			if err != nil {
				return "", entries, false, "", err
			}
			if len(changed) > 0 {
				e.Conflict = true
				whole = append(whole, e)
			} else {
				graftPaths = append(graftPaths, byRoot[root]...)
			}
		}
		entries = append(entries, e)
	}
	if len(shell) > 0 {
		changed, err := treeDiffPaths(ctx, repo, base, booted, shell)
		if err != nil {
			return "", entries, false, "", err
		}
		if len(changed) > 0 {
			return "", entries, false, "the booted base also changed " + shownPaths(changed) + ", which the patch touches, so it is not grafted", nil
		}
	}
	entriesOut := entries
	refused := ""
	err = scratchIndex(ctx, repo, func(env []string) error {
		if _, err := git(ctx, repo, env, nil, "read-tree", booted); err != nil {
			return err
		}
		for _, e := range whole {
			listed, err := gitRaw(ctx, repo, env, nil, "ls-files", "-z", "--", ":(literal)"+e.Root)
			if err != nil {
				return err
			}
			var paths []string
			for _, p := range strings.Split(string(listed), "\x00") {
				if p != "" {
					paths = append(paths, p)
				}
			}
			if err := updateIndexRemove(ctx, repo, env, paths); err != nil {
				return err
			}
			if e.Tree == "" {
				continue
			}
			if _, err := git(ctx, repo, env, nil, "read-tree", "--prefix="+e.Root+"/", e.Tree); err != nil {
				refused = "the stack's package at " + e.Root + " does not fit the booted tree (" + err.Error() + "), so it is not placed"
				return nil
			}
		}
		why, err := placeAfterImage(ctx, repo, env, base, tree, graftPaths)
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
		return "", entriesOut, false, "", err
	}
	if refused != "" {
		return "", entriesOut, false, refused, nil
	}
	// Checked: the grafted paths hold the after-image, each whole root holds
	// R, and nothing else differs from booted.
	if len(graftPaths) > 0 {
		// (An empty pathspec list would compare the whole trees.)
		if differ, err := treeDiffPaths(ctx, repo, tree, graft, graftPaths); err != nil {
			return "", entriesOut, false, "", err
		} else if len(differ) > 0 {
			return "", entriesOut, false, "the graft does not hold the patch's after-image at " + differ[0], nil
		}
	}
	in := make(map[string]bool, len(graftPaths))
	for _, p := range graftPaths {
		in[p] = true
	}
	wholeRoot := map[string]bool{}
	for _, e := range whole {
		if got := subtreeID(ctx, repo, graft, e.Root); got != e.Tree {
			return "", entriesOut, false, "the graft does not hold the stack's package at " + e.Root, nil
		}
		wholeRoot[e.Root] = true
	}
	outside, err := treeDiffPaths(ctx, repo, bootedTree, graft, nil)
	if err != nil {
		return "", entriesOut, false, "", err
	}
	for _, p := range outside {
		// A whole root's own path differs too when booted had a file there.
		if !in[p] && !wholeRoot[PackageRootOf(p)] && !wholeRoot[p] {
			return "", entriesOut, false, "the graft changes " + p + ", which the patch does not touch", nil
		}
	}
	return graft, entriesOut, true, "", nil
}

// shownPaths names up to five paths.
func shownPaths(paths []string) string {
	shown := paths
	if len(shown) > 5 {
		shown = shown[:5]
	}
	more := ""
	if len(paths) > len(shown) {
		more = fmt.Sprintf(" and %d more", len(paths)-len(shown))
	}
	return strings.Join(shown, ", ") + more
}

// packageConflicts lists the roots whose entry is a conflict.
func packageConflicts(entries []PackageEntry) []string {
	var out []string
	for _, e := range entries {
		if e.Conflict {
			out = append(out, e.Root)
		}
	}
	return out
}
