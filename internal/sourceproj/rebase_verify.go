package sourceproj

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	// RebaseHunkMissing: a path of P that `git apply` cannot apply to B' as a
	// whole is checked by a 3-way merge of (B', B, B+P); the candidate lacks a
	// hunk the merge fixes (a clean hunk dropped or altered, a text edited
	// inside the path, or the file deleted). Review 321, F1.
	RebaseHunkMissing = "hunk_missing"
)

// RebaseCheck is one rebase candidate against the applied change it rebases.
type RebaseCheck struct {
	// StackBase is B, the base P was built on.
	StackBase string
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
	// Unresolved the conflict paths the candidate left as B' has them; Dropped
	// the conflict paths whose candidate lacks a hunk of P (hunk granularity).
	Differing, Unresolved, Dropped []string
}

// VerifyRebase checks, in scratch indexes of repo (the guest repository holds
// B and B'), that a candidate built on B' contains P (docs/design/
// u4-stack-rebase-build-2026-10-26.md, question 7):
//   - the candidate's base is B';
//   - T_C is B' with the candidate applied, T_P is B' with P applied on the
//     paths that apply cleanly, and every such clean path is byte-equal in T_C
//     and T_P;
//   - every path of P that conflicts with B' is checked per hunk: the 3-way
//     merge of (B', B, B+P) is clean, and the candidate equals it; or it
//     conflicts, and the candidate holds every non-conflicting stretch of the
//     merge in order and differs from the merge with the conflicts resolved to
//     B' (so it was resolved, not dropped or kept as the new base has it);
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
	if len(conflict) > 0 {
		if strings.TrimSpace(c.StackBase) == "" {
			return RebaseVerdict{}, errors.New("sourceproj: the stack base is needed to verify a conflicting path")
		}
		stackCommit, ok := resolveCommit(ctx, repo, c.StackBase)
		if !ok {
			return RebaseVerdict{}, fmt.Errorf("sourceproj: the stack base %q is not in the source repository", c.StackBase)
		}
		stackTree, err := git(ctx, repo, nil, nil, "rev-parse", stackCommit+"^{tree}")
		if err != nil {
			return RebaseVerdict{}, fmt.Errorf("sourceproj: tree of %s: %w", stackCommit, err)
		}
		postTree, err := patchedTree(ctx, repo, stackCommit, c.Patch, false)
		if err != nil {
			return RebaseVerdict{}, fmt.Errorf("sourceproj: the applied change does not apply to its own base %s: %w", stackCommit, err)
		}
		for _, path := range conflict {
			outcome, err := checkConflictPath(ctx, repo, path, bootedTree, stackTree, postTree, candTree)
			if err != nil {
				return RebaseVerdict{}, err
			}
			switch outcome {
			case conflictResolved:
				verdict.Resolved = append(verdict.Resolved, path)
			case conflictKeptBase:
				verdict.Unresolved = append(verdict.Unresolved, path)
			default:
				verdict.Dropped = append(verdict.Dropped, path)
			}
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
	for _, list := range [][]string{verdict.Clean, verdict.Resolved, verdict.Extra, verdict.Differing, verdict.Unresolved, verdict.Dropped} {
		sort.Strings(list)
	}
	switch {
	case len(verdict.Differing) > 0:
		verdict.Reason = RebaseCleanPathDiffers
	case len(verdict.Dropped) > 0:
		verdict.Reason = RebaseHunkMissing
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

type conflictOutcome int

const (
	conflictResolved conflictOutcome = iota
	conflictKeptBase
	conflictDropped
)

// conflictMarker is long enough that file text cannot collide with it.
const conflictMarker = 40

// checkConflictPath decides one path of P that does not apply to B' as it is.
func checkConflictPath(ctx context.Context, repo, path, bootedTree, stackTree, postTree, candTree string) (conflictOutcome, error) {
	ours, err := blobOf(ctx, repo, bootedTree, path)
	if err != nil {
		return 0, err
	}
	base, err := blobOf(ctx, repo, stackTree, path)
	if err != nil {
		return 0, err
	}
	theirs, err := blobOf(ctx, repo, postTree, path)
	if err != nil {
		return 0, err
	}
	cand, err := blobOf(ctx, repo, candTree, path)
	if err != nil {
		return 0, err
	}
	// The new base already made P's change in a whole-file shape. P deleted the
	// file and so did B': the candidate must leave it deleted (restoring it
	// would undo P). P added a file and B' has exactly that file: the candidate
	// must leave it as it is.
	if !ours.exists && !theirs.exists {
		if cand.exists {
			return conflictDropped, nil
		}
		return conflictResolved, nil
	}
	if ours.exists && theirs.exists && ours.entry == theirs.entry {
		if cand.entry == ours.entry {
			return conflictResolved, nil
		}
		return conflictDropped, nil
	}
	// P leaves the file in place, so a candidate without it dropped the change.
	if theirs.exists && !cand.exists {
		return conflictDropped, nil
	}
	// P modifies a file the new base deleted: nothing to merge against, so the
	// candidate must hold P's exact post-image. Any other text (B's original
	// included) drops P's hunks (re-review 337).
	if !ours.exists && theirs.exists {
		if cand.entry == theirs.entry {
			return conflictResolved, nil
		}
		return conflictDropped, nil
	}
	if !(ours.regular() && base.regular() && theirs.regular()) {
		// Added, deleted, binary-mode or special entries cannot be merged by
		// text: the whole-path rule (the candidate differs from B').
		if cand.entry == ours.entry {
			return conflictKeptBase, nil
		}
		return conflictResolved, nil
	}
	// The mode the candidate must have: P's when P changed it, else B''s.
	wantMode := ours.mode
	if theirs.mode != base.mode {
		wantMode = theirs.mode
	}
	oursText, baseText, theirsText, err := catBlobs(ctx, repo, ours.sha, base.sha, theirs.sha)
	if err != nil {
		return 0, err
	}
	candText, err := catBlob(ctx, repo, cand.sha)
	if err != nil {
		return 0, err
	}
	merged, conflicts, err := mergeFile(ctx, oursText, baseText, theirsText, "")
	if err != nil {
		return 0, err
	}
	if conflicts == 0 {
		if bytes.Equal(candText, merged) && cand.mode == wantMode {
			return conflictResolved, nil
		}
		return conflictDropped, nil
	}
	keptBase, _, err := mergeFile(ctx, oursText, baseText, theirsText, "--ours")
	if err != nil {
		return 0, err
	}
	if bytes.Equal(candText, keptBase) {
		return conflictKeptBase, nil
	}
	// The raw merge, conflict markers left in, is not a resolution; neither is
	// any text with the verifier's long markers that no input side had.
	if bytes.Equal(candText, merged) || (hasConflictMarker(candText) && !hasConflictMarker(oursText) && !hasConflictMarker(baseText) && !hasConflictMarker(theirsText)) {
		return conflictDropped, nil
	}
	if containsInOrder(candText, stableSegments(merged)) && cand.mode == wantMode {
		return conflictResolved, nil
	}
	return conflictDropped, nil
}

type blobEntry struct {
	entry, sha string
	exists     bool
	mode       string
}

func (b blobEntry) regular() bool {
	return b.exists && (b.mode == "100644" || b.mode == "100755")
}

func blobOf(ctx context.Context, repo, tree, path string) (blobEntry, error) {
	entry, err := rebaseTreeEntry(ctx, repo, tree, path)
	if err != nil || entry == "" {
		return blobEntry{}, err
	}
	fields := strings.Fields(entry)
	if len(fields) != 3 || fields[1] != "blob" {
		return blobEntry{entry: entry, exists: true, mode: "other"}, nil
	}
	return blobEntry{entry: entry, sha: fields[2], exists: true, mode: fields[0]}, nil
}

func catBlob(ctx context.Context, repo, sha string) ([]byte, error) {
	return gitRaw(ctx, repo, nil, nil, "cat-file", "blob", sha)
}

func catBlobs(ctx context.Context, repo string, shas ...string) (a, b, c []byte, err error) {
	out := make([][]byte, len(shas))
	for i, sha := range shas {
		if out[i], err = catBlob(ctx, repo, sha); err != nil {
			return nil, nil, nil, err
		}
	}
	return out[0], out[1], out[2], nil
}

// mergeFile runs `git merge-file -p` on three texts and returns the merged
// text and the number of conflicts (0 for a clean merge). extra is one more
// flag ("--ours"), or "".
func mergeFile(ctx context.Context, ours, base, theirs []byte, extra string) ([]byte, int, error) {
	dir, err := os.MkdirTemp("", "choir-sourceproj-merge-")
	if err != nil {
		return nil, 0, err
	}
	defer os.RemoveAll(dir)
	paths := []string{filepath.Join(dir, "ours"), filepath.Join(dir, "base"), filepath.Join(dir, "theirs")}
	for i, text := range [][]byte{ours, base, theirs} {
		if err := os.WriteFile(paths[i], text, 0o600); err != nil {
			return nil, 0, err
		}
	}
	args := []string{"merge-file", "-p", fmt.Sprintf("--marker-size=%d", conflictMarker), "-L", "ours", "-L", "base", "-L", "theirs"}
	if extra != "" {
		args = append(args, extra)
	}
	ctx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append(args, paths...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	runErr := cmd.Run()
	var exit *exec.ExitError
	switch {
	case runErr == nil:
		return stdout.Bytes(), 0, nil
	case errors.As(runErr, &exit) && exit.ExitCode() > 0 && exit.ExitCode() < 128:
		return stdout.Bytes(), exit.ExitCode(), nil
	}
	return nil, 0, fmt.Errorf("sourceproj: git merge-file: %w", runErr)
}

func hasConflictMarker(text []byte) bool {
	open := bytes.Repeat([]byte("<"), conflictMarker)
	close := bytes.Repeat([]byte(">"), conflictMarker)
	for _, line := range bytes.Split(text, []byte("\n")) {
		if bytes.HasPrefix(line, open) || bytes.HasPrefix(line, close) {
			return true
		}
	}
	return false
}

// stableSegments splits merged text at its conflict blocks and returns the
// text between them (the first and last may be empty).
func stableSegments(merged []byte) [][]byte {
	open := bytes.Repeat([]byte("<"), conflictMarker)
	close := bytes.Repeat([]byte(">"), conflictMarker)
	var segments [][]byte
	var current []byte
	inBlock := false
	for _, line := range bytes.SplitAfter(merged, []byte("\n")) {
		switch {
		case !inBlock && bytes.HasPrefix(line, open):
			segments = append(segments, current)
			current, inBlock = nil, true
		case inBlock && bytes.HasPrefix(line, close):
			inBlock = false
		case inBlock:
		default:
			current = append(current, line...)
		}
	}
	return append(segments, current)
}

// containsInOrder reports whether text starts with the first segment, ends with
// the last and holds the ones between in order (anything may fill the gaps
// where conflict blocks were).
func containsInOrder(text []byte, segments [][]byte) bool {
	pos := 0
	for i, seg := range segments {
		if len(seg) == 0 {
			continue
		}
		switch {
		case i == 0:
			if !bytes.HasPrefix(text, seg) {
				return false
			}
			pos = len(seg)
		case i == len(segments)-1:
			if len(text) < pos+len(seg) || !bytes.HasSuffix(text[pos:], seg) {
				return false
			}
			pos = len(text)
		default:
			idx := bytes.Index(text[pos:], seg)
			if idx < 0 {
				return false
			}
			pos += idx + len(seg)
		}
	}
	return true
}
