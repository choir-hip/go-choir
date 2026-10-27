package sourceproj

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Contained reports whether patch (authored against base, producing
// patchedTreeID) is already in the booted base's tree. It is decided per path
// the patch touches, with three blobs: the base's (B), the patch's after-image
// (P) and booted's (B'):
//
//   - an exact match is contained: B' has P's blob and mode, or the path is
//     absent in both where the patch deletes it (the U1 test, C1 to C9 in the
//     U1 build notes);
//   - a path the patch adds or deletes is contained only by that exact match;
//   - a path the patch modifies is also contained when a three-way merge of the
//     change B to P into B' (git merge-file, no whitespace or line-ending
//     options) is clean and its output is byte-identical to B', and B' has P's
//     mode;
//   - binary content (git's own detection), anything but a regular file, and a
//     path whose B or B' blob is not in the repository get the exact test only.
//
// This is what a promotion leaves when main also edited the same file
// elsewhere (docs/problems/promoted-stack-stays-base-moved-2026-10-26.md): diff3
// sees the identical change on both sides, takes it once, and the result is B'.
//
// It resists the duplicated block, where a reverse-apply does not. A
// reverse-apply only shows the after-image with its context appears somewhere
// in B', which a copy of the block elsewhere satisfies. The merge aligns B
// against P and B against B': if B' made the change somewhere else, the
// patch's change is still unmade at the place B and P align, so the merge
// applies it, the output differs from B', and the answer is "not contained".
// A partly upstreamed patch is the same: the missing hunks are applied and the
// output differs. An edit on main adjacent to a hunk conflicts and is "not
// contained", the safe false negative (it lands as stack_base_moved).
//
// When the content says no, patch identity is asked (upstreamedByPatchID):
// a verbatim promotion to main followed by an edit beside it is a conflict to
// the merge but the same commit to git.
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
	in, err := containedByContent(ctx, repo, booted, base, patchedTreeID, touched)
	if err != nil || in {
		return in, err
	}
	return upstreamedByPatchID(ctx, repo, booted, base, patchedTreeID, touched)
}

// containedByContent is the exact test, then the three-way test per path.
func containedByContent(ctx context.Context, repo, booted, base, patchedTreeID string, touched []string) (bool, error) {
	differing, err := treeDiffRaw(ctx, repo, patchedTreeID, booted, touched)
	if err != nil {
		return false, err
	}
	if len(differing) == 0 {
		return true, nil // exact
	}
	paths := make([]string, len(differing))
	for i, d := range differing {
		paths[i] = d.path
	}
	change, err := treeDiffRaw(ctx, repo, base, patchedTreeID, paths)
	if err != nil {
		return false, err
	}
	baseOf := make(map[string]treeEntry, len(change))
	for _, c := range change {
		baseOf[c.path] = c.a
	}
	for _, d := range differing {
		b, ok := baseOf[d.path]
		if !ok {
			// Every touched path is in the base-to-after diff; a path that is
			// not cannot be judged, so it gets the exact answer.
			return false, nil
		}
		in, err := mergeContained(ctx, repo, b, d.a, d.b)
		if err != nil {
			return false, fmt.Errorf("%s: %w", d.path, err)
		}
		if !in {
			return false, nil
		}
	}
	return true, nil
}

// mergeContained is the three-way test for one path whose booted entry
// differs from the after-image: b is the base's entry, p the after-image's,
// t booted's.
func mergeContained(ctx context.Context, repo string, b, p, t treeEntry) (bool, error) {
	if !b.regular() || !p.regular() || !t.regular() {
		return false, nil // added, deleted, a symlink or a submodule: exact only
	}
	if t.mode != p.mode {
		return false, nil
	}
	if t.oid == b.oid {
		// Booted still has the base's bytes and the patch changed them, so the
		// merge would print the after-image, which differs from booted: no,
		// without spawning git (this runs on every cast of a live stack).
		return false, nil
	}
	for _, oid := range []string{b.oid, p.oid, t.oid} {
		have, err := haveObject(ctx, repo, oid)
		if err != nil {
			return false, err
		}
		if !have {
			return false, nil // no merge base or no side: exact only
		}
	}
	blobs := map[string][]byte{}
	for name, oid := range map[string]string{"base": b.oid, "after": p.oid, "booted": t.oid} {
		raw, err := gitRaw(ctx, repo, nil, nil, "cat-file", "blob", oid)
		if err != nil {
			return false, err
		}
		if binary(raw) {
			return false, nil
		}
		blobs[name] = raw
	}
	dir, err := os.MkdirTemp("", "choir-contained-")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(dir)
	for name, raw := range blobs {
		if err := os.WriteFile(filepath.Join(dir, name), raw, 0o600); err != nil {
			return false, err
		}
	}
	booted := blobs["booted"]
	// merge-file <current> <base> <other>: the change base->after merged into
	// booted, printed (-p) rather than written over booted. Exit 0 is clean, 1
	// to 127 counts conflicts, anything else is an error.
	out, err := gitRaw(ctx, repo, nil, nil, "merge-file", "-p",
		filepath.Join(dir, "booted"), filepath.Join(dir, "base"), filepath.Join(dir, "after"))
	if err != nil {
		if code, ok := gitExitCode(err); ok && code >= 1 && code <= 127 {
			return false, nil
		}
		return false, err
	}
	return bytes.Equal(out, booted), nil
}

// treeEntry is one side of a raw tree diff; an absent side has mode 000000.
type treeEntry struct{ mode, oid string }

func (e treeEntry) regular() bool { return e.mode == "100644" || e.mode == "100755" }

type rawDiff struct {
	path string
	a, b treeEntry
}

// treeDiffRaw is treeDiffPaths with both sides' mode and blob id.
func treeDiffRaw(ctx context.Context, repo, a, b string, only []string) ([]rawDiff, error) {
	var out []rawDiff
	const chunk = 100
	for i := 0; i < len(only); i += chunk {
		end := i + chunk
		if end > len(only) {
			end = len(only)
		}
		args := []string{"diff-tree", "-r", "--no-renames", "--raw", "-z", a, b, "--"}
		for _, p := range only[i:end] {
			args = append(args, ":(literal)"+p)
		}
		raw, err := gitRaw(ctx, repo, nil, nil, args...)
		if err != nil {
			return nil, err
		}
		// -z raw: ":<mode a> <mode b> <oid a> <oid b> <status>\0<path>\0".
		fields := strings.Split(string(raw), "\x00")
		for j := 0; j+1 < len(fields); j += 2 {
			meta := strings.Fields(strings.TrimPrefix(fields[j], ":"))
			if len(meta) != 5 || fields[j+1] == "" {
				return nil, fmt.Errorf("git diff-tree --raw: unexpected record %q", fields[j])
			}
			if !commitID.MatchString(meta[2]) || !commitID.MatchString(meta[3]) {
				return nil, fmt.Errorf("git diff-tree --raw: malformed object id in %q", fields[j])
			}
			out = append(out, rawDiff{path: fields[j+1], a: treeEntry{meta[0], meta[2]}, b: treeEntry{meta[1], meta[3]}})
		}
	}
	return out, nil
}

// haveObject reports whether oid is in repo's object store.
func haveObject(ctx context.Context, repo, oid string) (bool, error) {
	_, err := gitRaw(ctx, repo, nil, nil, "cat-file", "-e", oid)
	if err == nil {
		return true, nil
	}
	if exitCode1(err) {
		return false, nil
	}
	return false, err
}

// binary is git's own content heuristic, a NUL in the first 8000 bytes, read
// from the bytes alone: repository attributes (a "-diff" or "binary" mark)
// cannot route a binary file through the line merge, or keep a text file out.
func binary(raw []byte) bool {
	if len(raw) > 8000 {
		raw = raw[:8000]
	}
	return bytes.IndexByte(raw, 0) >= 0
}

// gitExitCode returns the exit status of a git command that ran and failed.
func gitExitCode(err error) (int, bool) {
	var ge *gitError
	if !errors.As(err, &ge) {
		return 0, false
	}
	var ee *exec.ExitError
	if !errors.As(ge.cause, &ee) {
		return 0, false
	}
	return ee.ExitCode(), true
}
