package sourceproj

import (
	"bytes"
	"context"
	"strings"
)

// upstreamedByPatchID is git's own answer to "is this change already
// upstream" (git cherry, and git rebase skipping an applied commit): a
// non-merge commit in base..booted whose diff, on the patch's paths, has the
// patch's stable patch-id. A promotion to main is such a commit, and stays one
// when main later edits beside it, where the three-way merge conflicts
// (docs/problems/selfdev-blocked-behind-unperformed-rebase-2026-10-27.md).
//
// A patch-id has no line numbers, so a commit that made the same change at
// another place with the same context (a duplicated block) has it too. A match
// therefore counts only when the content test holds at that commit: the change
// base->after is exactly there, or merges into it cleanly to its bytes. Main's
// later edits beside the change are what this lets through; they are after
// the commit, not in it.
//
// A later commit in the range with the reverse patch-id undoes it: main
// reverted the promotion, so the change is not in booted. Without history from
// base to booted (booted does not descend from base, or the commits are not in
// the repository) the answer is no, the content's answer. Only touched paths
// are diffed, so a promotion that also changed other paths still matches.
//
// An error means git failed, not "no".
func upstreamedByPatchID(ctx context.Context, repo, booted, base string, patch []byte, patchedTreeID string, touched []string) (bool, error) {
	if base == booted {
		return false, nil
	}
	want, err := patchID(ctx, repo, patch)
	if err != nil || want == "" {
		return false, err
	}
	if _, err := gitRaw(ctx, repo, nil, nil, "merge-base", "--is-ancestor", base, booted); err != nil {
		if exitCode1(err) {
			return false, nil
		}
		return false, nil // a commit of the range is missing: no history, no answer
	}
	args := []string{"log", "--no-merges", "--reverse", "--format=commit %H", "-p", "--no-color", "--no-ext-diff",
		"--no-textconv", "--no-renames", "--src-prefix=a/", "--dst-prefix=b/", base + ".." + booted, "--"}
	for _, p := range touched {
		args = append(args, ":(literal)"+p)
	}
	log, err := gitRaw(ctx, repo, nil, nil, args...)
	if err != nil {
		// An object of the range is not in the repository (the walk reads every
		// blob it diffs): no history, no answer, as for a missing commit.
		return false, nil
	}
	ids, err := gitRaw(ctx, repo, nil, log, "patch-id", "--stable")
	if err != nil {
		return false, err
	}
	reverse, err := reversePatchID(ctx, repo, ids, want, touched)
	if err != nil {
		return false, err
	}
	// Oldest first: the last word on the change wins.
	in := false
	for _, line := range strings.Split(strings.TrimSpace(string(ids)), "\n") {
		f := strings.Fields(line)
		if len(f) != 2 {
			continue
		}
		switch f[0] {
		case want:
			if !commitID.MatchString(f[1]) {
				continue
			}
			made, err := containedByContent(ctx, repo, f[1], base, patchedTreeID, touched)
			if err != nil {
				return false, err
			}
			if made {
				in = true
			}
		case reverse:
			in = false
		}
	}
	return in, nil
}

// patchID is the stable patch-id of a patch, empty when it has no diff.
func patchID(ctx context.Context, repo string, patch []byte) (string, error) {
	out, err := gitRaw(ctx, repo, nil, patch, "patch-id", "--stable")
	if err != nil {
		return "", err
	}
	f := strings.Fields(string(out))
	if len(f) == 0 {
		return "", nil
	}
	return f[0], nil
}

// reversePatchID is the stable patch-id of the reverse of the change whose
// patch-id is want, read from the first commit of ids that made it (ids is
// patch-id output, "<patch-id> <commit>" per line). Empty when no commit
// made it.
func reversePatchID(ctx context.Context, repo string, ids []byte, want string, touched []string) (string, error) {
	for _, line := range strings.Split(strings.TrimSpace(string(ids)), "\n") {
		f := strings.Fields(line)
		if len(f) != 2 || f[0] != want || !commitID.MatchString(f[1]) {
			continue
		}
		args := []string{"diff", "-p", "--no-color", "--no-ext-diff", "--no-textconv", "--no-renames",
			"--src-prefix=a/", "--dst-prefix=b/", f[1], f[1] + "^", "--"}
		for _, p := range touched {
			args = append(args, ":(literal)"+p)
		}
		diff, err := gitRaw(ctx, repo, nil, nil, args...)
		if err != nil {
			return "", err
		}
		if len(bytes.TrimSpace(diff)) == 0 {
			return "", nil
		}
		return patchID(ctx, repo, diff)
	}
	return "", nil
}
