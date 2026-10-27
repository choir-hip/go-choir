package sourceproj

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"unicode"
)

// upstreamedByPatchID is git's own answer to "is this change already
// upstream" (git cherry, and git rebase skipping an applied commit): a
// non-merge commit C in base..booted whose diff, on the patch's paths, has the
// patch's stable patch-id. A promotion to main is such a commit, and stays one
// when main later edits beside it, where the three-way merge conflicts
// (docs/problems/selfdev-blocked-behind-unperformed-rebase-2026-10-27.md).
//
// A patch-id has no line numbers, so a commit that made the same change at
// another place with the same context (a duplicated block) has it too. A match
// therefore counts only when the content test holds at C: the change
// base->after is exactly there, or merges into it cleanly to its bytes.
//
// Then main's changes after C (the tree diff C..booted, so a merge is read as
// its result) must leave the change alone (laterLeavesChange): no touched
// path deleted, re-moded or binary, no line the patch added removed, and no
// line it removed added back. A revert, exact or not, is caught there; an
// edit elsewhere in the file, or beside the change, is not (review 353, F1).
//
// Without history from base to booted (booted does not descend from base, or
// an object of the range is not in the repository) the answer is no, the
// content's answer. Only touched paths are diffed, so a promotion that also
// changed other paths still matches.
//
// An error means git failed, not "no".
func upstreamedByPatchID(ctx context.Context, repo, booted, base string, patchedTreeID string, touched []string) (bool, error) {
	if base == booted {
		return false, nil
	}
	if _, err := gitRaw(ctx, repo, nil, nil, "merge-base", "--is-ancestor", base, booted); err != nil {
		if exitCode1(err) || missingObject(err) {
			return false, nil
		}
		return false, err
	}
	// The wanted id comes from git's own diff of base and the patched tree,
	// so it is normalized exactly as the log's diffs are (review 353, F5).
	change, err := gitRaw(ctx, repo, nil, nil, pathArgs([]string{"diff", "-p"}, diffOpts, base, patchedTreeID, touched)...)
	if err != nil {
		if missingObject(err) {
			return false, nil
		}
		return false, err
	}
	want, err := patchID(ctx, repo, change)
	if err != nil || want == "" {
		return false, err
	}
	log, err := gitRaw(ctx, repo, nil, nil, pathArgs([]string{"log", "--no-merges", "--format=commit %H", "-p"}, diffOpts, base+".."+booted, "", touched)...)
	if err != nil {
		if missingObject(err) {
			return false, nil
		}
		return false, err
	}
	ids, err := gitRaw(ctx, repo, nil, log, "patch-id", "--stable")
	if err != nil {
		return false, err
	}
	// git log lists newest first: the first match whose content holds is
	// the last commit that made the change.
	for _, line := range strings.Split(strings.TrimSpace(string(ids)), "\n") {
		f := strings.Fields(line)
		if len(f) != 2 || f[0] != want || !commitID.MatchString(f[1]) {
			continue
		}
		made, err := containedByContent(ctx, repo, f[1], base, patchedTreeID, touched)
		if err != nil {
			return false, err
		}
		if made {
			return laterLeavesChange(ctx, repo, f[1], booted, change, touched)
		}
	}
	return false, nil
}

// diffOpts make a diff independent of the repository's diff configuration.
var diffOpts = []string{"--no-color", "--no-ext-diff", "--no-textconv", "--no-renames", "--src-prefix=a/", "--dst-prefix=b/"}

// pathArgs is cmd, opts, the revisions (b may be empty) and the touched paths
// as literal pathspecs.
func pathArgs(cmd, opts []string, a, b string, touched []string) []string {
	args := append(append([]string{}, cmd...), opts...)
	args = append(args, a)
	if b != "" {
		args = append(args, b)
	}
	args = append(args, "--")
	for _, p := range touched {
		args = append(args, ":(literal)"+p)
	}
	return args
}

// laterLeavesChange reports whether main's changes from made (the commit that
// made the change) to booted leave the change in place. change is the patch
// as git diffs it (base to the patched tree, on the touched paths).
func laterLeavesChange(ctx context.Context, repo, made, booted string, change []byte, touched []string) (bool, error) {
	if made == booted {
		return true, nil
	}
	raw, err := treeDiffRaw(ctx, repo, made, booted, touched)
	if err != nil {
		return false, err
	}
	if len(raw) == 0 {
		return true, nil
	}
	changed := make([]string, 0, len(raw))
	for _, d := range raw {
		// Deleted, re-moded, or not a regular file on either side: the line
		// test below cannot speak for it.
		if !d.a.regular() || !d.b.regular() || d.a.mode != d.b.mode {
			return false, nil
		}
		changed = append(changed, d.path)
	}
	later, err := gitRaw(ctx, repo, nil, nil, pathArgs([]string{"diff", "-U0"}, diffOpts, made, booted, changed)...)
	if err != nil {
		return false, err
	}
	if bytes.Contains(later, []byte("\nBinary files ")) || bytes.HasPrefix(later, []byte("Binary files ")) {
		return false, nil
	}
	pAdded, pRemoved := diffLines(change)
	lAdded, lRemoved := diffLines(later)
	for key := range lRemoved {
		if pAdded[key] {
			return false, nil // a line the patch added is gone
		}
	}
	for key := range lAdded {
		if pRemoved[key] {
			return false, nil // a line the patch removed is back
		}
	}
	return true, nil
}

// diffLines are a unified diff's added and removed lines, keyed by path and
// text. A line with no letter or digit (blank, or only "}") carries nothing
// and is left out: the evidence appended a blank line and a "}" beside a
// deletion that removed both. A real re-add brings back lettered lines.
func diffLines(diff []byte) (added, removed map[string]bool) {
	added, removed = map[string]bool{}, map[string]bool{}
	path, header := "", false
	for _, line := range strings.Split(string(diff), "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			path, header = "", true
		case header && strings.HasPrefix(line, "@@"):
			header = false
		case header && strings.HasPrefix(line, "+++ "):
			if name := strings.TrimPrefix(line, "+++ "); name != "/dev/null" {
				path = strings.TrimPrefix(name, "b/")
			}
		case header && strings.HasPrefix(line, "--- "):
			if name := strings.TrimPrefix(line, "--- "); name != "/dev/null" && path == "" {
				path = strings.TrimPrefix(name, "a/")
			}
		case header:
		case strings.HasPrefix(line, "+"):
			if text := line[1:]; meaningful(text) {
				added[path+"\x00"+text] = true
			}
		case strings.HasPrefix(line, "-"):
			if text := line[1:]; meaningful(text) {
				removed[path+"\x00"+text] = true
			}
		}
	}
	return added, removed
}

// meaningful reports whether a line has a letter or a digit.
func meaningful(text string) bool {
	return strings.IndexFunc(text, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) >= 0
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

// missingObject reports whether git failed because an object is not in the
// repository: no history to read, not a failure of git (review 353, F2).
func missingObject(err error) bool {
	var reason string
	var ge *gitError
	if errors.As(err, &ge) {
		reason = ge.reason
	} else if err != nil {
		reason = err.Error()
	}
	for _, s := range []string{"unable to read", "bad object", "not a valid object", "missing blob", "missing tree", "Not a valid commit name", "bad revision", "unknown revision"} {
		if strings.Contains(reason, s) {
			return true
		}
	}
	return false
}
