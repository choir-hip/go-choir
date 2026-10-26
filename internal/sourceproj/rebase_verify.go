package sourceproj

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Verdict reasons of VerifyRebase. The first word of the materializer's
// refusal is rebase_content_mismatch; the reason says which check failed.
const (
	RebaseBaseMismatch          = "base_mismatch"
	RebaseCandidateDoesNotApply = "candidate_does_not_apply"
	RebaseCleanPathDiffers      = "clean_path_differs"
	// RebaseConflictUnresolved: a path of P that conflicts with B' has, in the
	// candidate, the content B' has. Content cannot tell "resolved by keeping
	// B's version" from "dropped", so it is refused (named residual of the
	// design, re-review 307 finding 2).
	RebaseConflictUnresolved = "kept_base_unverifiable"
)

// RebaseCheck is one rebase candidate against the applied change it rebases.
type RebaseCheck struct {
	// BootedBase is B', the base the computer now runs.
	BootedBase string
	// Patch is P, the applied stack top's source patch (built on its stack base).
	Patch []byte
	// CandidateBase and Candidate are the bundle's SourcePatchBaseCommit and
	// source patch.
	CandidateBase string
	Candidate     []byte
}

// RebaseVerdict is what VerifyRebase found. Paths are sorted.
type RebaseVerdict struct {
	Pass   bool
	Reason string
	// Clean are the paths of P that apply to B' as they are; Resolved the
	// paths of P that conflict with B' and that the candidate changed from B';
	// Extra the paths the candidate changes that P does not.
	Clean, Resolved, Extra []string
	// Differing are the clean paths whose candidate content is not P's;
	// Unresolved the conflict paths the candidate left as B' has them.
	Differing, Unresolved []string
}

// VerifyRebase checks, in scratch indexes of repo (the guest repository holds
// B and B'), that a candidate built on B' contains P (docs/design/
// u4-stack-rebase-build-2026-10-26.md, question 7):
//   - the candidate's base is B';
//   - T_C is B' with the candidate applied, T_P is B' with P applied on the
//     paths that apply cleanly, and every such clean path is byte-equal in T_C
//     and T_P;
//   - every path of P that conflicts with B' differs in T_C from B', so it was
//     resolved and not dropped;
//   - paths the candidate changes outside P are allowed and listed.
//
// A content mismatch is a verdict, not an error; an error is git failing or an
// unusable input.
func VerifyRebase(ctx context.Context, repo string, c RebaseCheck) (RebaseVerdict, error) {
	if len(c.Patch) == 0 {
		return RebaseVerdict{}, errors.New("sourceproj: the applied change has no patch to verify a rebase against")
	}
	booted, ok := resolveCommit(ctx, repo, c.BootedBase)
	if !ok {
		return RebaseVerdict{}, fmt.Errorf("sourceproj: the booted base %q is not in the source repository", c.BootedBase)
	}
	candBase, ok := resolveCommit(ctx, repo, c.CandidateBase)
	if !ok || candBase != booted || strings.TrimSpace(c.CandidateBase) != candBase {
		return RebaseVerdict{Reason: RebaseBaseMismatch}, nil
	}
	if len(c.Candidate) == 0 {
		return RebaseVerdict{Reason: RebaseCandidateDoesNotApply}, nil
	}
	bootedTree, err := git(ctx, repo, nil, nil, "rev-parse", booted+"^{tree}")
	if err != nil {
		return RebaseVerdict{}, fmt.Errorf("sourceproj: tree of %s: %w", booted, err)
	}
	candTree, err := patchedTree(ctx, repo, booted, c.Candidate, false)
	if err != nil {
		if errors.Is(err, ErrPatchDoesNotApply) {
			return RebaseVerdict{Reason: RebaseCandidateDoesNotApply}, nil
		}
		return RebaseVerdict{}, err
	}
	paths, err := patchPaths(ctx, repo, c.Patch)
	if err != nil {
		return RebaseVerdict{}, err
	}
	clean, conflict, err := classifyApply(ctx, repo, booted, c.Patch, paths)
	if err != nil {
		return RebaseVerdict{}, err
	}
	verdict := RebaseVerdict{Clean: clean}
	if len(clean) > 0 {
		var pTree string
		var err error
		if len(conflict) == 0 {
			// The whole patch applies; no path needs naming to --include.
			pTree, err = patchedTree(ctx, repo, booted, c.Patch, false)
		} else {
			pTree, err = patchedTreeOnly(ctx, repo, booted, c.Patch, clean)
		}
		if err != nil {
			return RebaseVerdict{}, err
		}
		for _, path := range clean {
			want, err := rebaseTreeEntry(ctx, repo, pTree, path)
			if err != nil {
				return RebaseVerdict{}, err
			}
			got, err := rebaseTreeEntry(ctx, repo, candTree, path)
			if err != nil {
				return RebaseVerdict{}, err
			}
			if want != got {
				verdict.Differing = append(verdict.Differing, path)
			}
		}
	}
	for _, path := range conflict {
		was, err := rebaseTreeEntry(ctx, repo, bootedTree, path)
		if err != nil {
			return RebaseVerdict{}, err
		}
		now, err := rebaseTreeEntry(ctx, repo, candTree, path)
		if err != nil {
			return RebaseVerdict{}, err
		}
		if was == now {
			verdict.Unresolved = append(verdict.Unresolved, path)
		} else {
			verdict.Resolved = append(verdict.Resolved, path)
		}
	}
	inP := map[string]bool{}
	for _, path := range paths {
		inP[path] = true
	}
	changed, err := treeDiffPaths(ctx, repo, bootedTree, candTree, nil)
	if err != nil {
		return RebaseVerdict{}, err
	}
	for _, path := range changed {
		if !inP[path] {
			verdict.Extra = append(verdict.Extra, path)
		}
	}
	for _, list := range [][]string{verdict.Clean, verdict.Resolved, verdict.Extra, verdict.Differing, verdict.Unresolved} {
		sort.Strings(list)
	}
	switch {
	case len(verdict.Differing) > 0:
		verdict.Reason = RebaseCleanPathDiffers
	case len(verdict.Unresolved) > 0:
		verdict.Reason = RebaseConflictUnresolved
	default:
		verdict.Pass = true
	}
	return verdict, nil
}

// patchPaths lists the paths a patch touches (both sides of a rename).
func patchPaths(ctx context.Context, repo string, patch []byte) ([]string, error) {
	raw, err := gitRaw(ctx, repo, nil, patch, "apply", "--numstat", "-z", "-")
	if err != nil {
		return nil, fmt.Errorf("sourceproj: read the paths of the applied patch: %w", err)
	}
	tokens := strings.Split(string(raw), "\x00")
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		if p != "" && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for i := 0; i < len(tokens); i++ {
		fields := strings.SplitN(tokens[i], "\t", 3)
		if len(fields) != 3 {
			continue
		}
		if fields[2] != "" {
			add(fields[2])
			continue
		}
		// A rename: the next two tokens are the old and the new path.
		if i+2 < len(tokens) {
			add(tokens[i+1])
			add(tokens[i+2])
			i += 2
		}
	}
	sort.Strings(out)
	return out, nil
}

// globChars are the characters git apply's --include would read as a pattern.
const globChars = "*?[\\"

// classifyApply splits the paths of patch into those that apply to base as
// they are (clean) and those that do not (conflict). When the whole patch
// applies every path is clean; otherwise each path is tried alone with
// --include. A path with a glob character cannot be named to --include, so it
// is a conflict path (the stricter check) whenever the whole patch fails.
func classifyApply(ctx context.Context, repo, base string, patch []byte, paths []string) (clean, conflict []string, err error) {
	if _, whole := patchedTree(ctx, repo, base, patch, false); whole == nil {
		return append([]string(nil), paths...), nil, nil
	}
	for _, path := range paths {
		if strings.ContainsAny(path, globChars) {
			conflict = append(conflict, path)
			continue
		}
		if _, only := patchedTreeOnly(ctx, repo, base, patch, []string{path}); only == nil {
			clean = append(clean, path)
		} else if errors.Is(only, ErrPatchDoesNotApply) {
			conflict = append(conflict, path)
		} else {
			return nil, nil, only
		}
	}
	return clean, conflict, nil
}

// patchedTreeOnly applies only the named paths of patch to base's tree in a
// scratch index and returns the tree. Paths must not contain glob characters.
func patchedTreeOnly(ctx context.Context, repo, base string, patch []byte, only []string) (string, error) {
	var tree string
	err := scratchIndex(ctx, repo, func(env []string) error {
		if _, err := git(ctx, repo, env, nil, "read-tree", base); err != nil {
			return fmt.Errorf("read base tree: %w", err)
		}
		args := []string{"apply", "--cached", "--whitespace=nowarn"}
		for _, path := range only {
			args = append(args, "--include="+path)
		}
		if _, err := git(ctx, repo, env, patch, append(args, "-")...); err != nil {
			return fmt.Errorf("%w: %v", ErrPatchDoesNotApply, err)
		}
		out, err := git(ctx, repo, env, nil, "write-tree")
		if err != nil {
			return fmt.Errorf("write tree: %w", err)
		}
		tree = out
		return nil
	})
	return tree, err
}

// treeEntry is a path's "mode type object" in a tree, or "" when absent.
func rebaseTreeEntry(ctx context.Context, repo, tree, path string) (string, error) {
	raw, err := gitRaw(ctx, repo, nil, nil, "ls-tree", "-z", tree, "--", ":(literal)"+path)
	if err != nil {
		return "", fmt.Errorf("sourceproj: read %s in %s: %w", path, tree, err)
	}
	entry, _, _ := strings.Cut(string(raw), "\t")
	return entry, nil
}
