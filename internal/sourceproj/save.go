package sourceproj

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Capsule overlay S4 (docs/design/capsule-overlay-computer-state-2026-11-03.md
// section 5, failure modes in
// docs/design/capsule-overlay-s4-source-failure-modes-2026-11-03.md): reconcile
// never resets work it has not saved, and a stack whose paths the new base left
// alone is grafted onto it byte for byte.

// saveWorkTree commits what `git reset --hard <expected>` would lose from the
// work tree and index on top of head, in a scratch index so the real one is
// untouched: every tracked change (modified, deleted, staged new), and every
// untracked file (ignored ones too) at a path expected tracks, or at an
// ancestor of one. It returns "" when there is nothing to lose. The commit is
// a function of its tree and head (fixed identity, head's committer date), so
// the same dirt saves to the same commit.
func saveWorkTree(ctx context.Context, repo, head, expected string) (string, error) {
	// An unborn HEAD (a repository with no commit yet) has nothing tracked to
	// lose, but a reset still overwrites the files at the paths expected
	// tracks: they are saved in a commit with no parent.
	base := head
	if base == "" {
		empty, err := git(ctx, repo, nil, []byte{}, "hash-object", "-t", "tree", "--stdin")
		if err != nil {
			return "", err
		}
		base = empty
	}
	headTree, err := git(ctx, repo, nil, nil, "rev-parse", base+"^{tree}")
	if err != nil {
		return "", err
	}
	var tree string
	err = scratchIndex(ctx, repo, func(env []string) error {
		index := strings.TrimPrefix(env[0], "GIT_INDEX_FILE=")
		real := filepath.Join(repo, ".git", "index")
		if err := copyFile(real, index); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			if _, err := git(ctx, repo, env, nil, "read-tree", base); err != nil {
				return err
			}
		}
		// An assume-unchanged entry hides its edit from add -u, and a reset
		// still writes the path: clear the bit in the scratch index only.
		// (skip-worktree entries are left alone: a reset does not write them.)
		if err := clearAssumeUnchanged(ctx, repo, env); err != nil {
			return err
		}
		if _, err := git(ctx, repo, env, nil, "add", "-u"); err != nil { // the whole tree (git 2 and later)
			return err
		}
		clobbered, err := untrackedInTheWay(ctx, repo, env, expected)
		if err != nil {
			return err
		}
		for i := 0; i < len(clobbered); i += 100 {
			end := i + 100
			if end > len(clobbered) {
				end = len(clobbered)
			}
			args := []string{"add", "-f", "--"}
			for _, p := range clobbered[i:end] {
				args = append(args, ":(literal)"+p)
			}
			if _, err := git(ctx, repo, env, nil, args...); err != nil {
				return err
			}
		}
		out, err := git(ctx, repo, env, nil, "write-tree")
		if err != nil {
			return err
		}
		if !commitID.MatchString(out) {
			return fmt.Errorf("git wrote a malformed tree id %q", out)
		}
		tree = out
		return nil
	})
	if err != nil || tree == headTree {
		return "", err
	}
	when := "1000000000" // a fixed date git parses as raw (a small number is not)
	if head != "" {
		if when, err = git(ctx, repo, nil, nil, "log", "-1", "--format=%ct", head); err != nil {
			return "", err
		}
	}
	date := strings.TrimSpace(when) + " +0000"
	env := []string{
		"GIT_AUTHOR_NAME=Choir Projection", "GIT_AUTHOR_EMAIL=projection@choir.local", "GIT_AUTHOR_DATE=" + date,
		"GIT_COMMITTER_NAME=Choir Projection", "GIT_COMMITTER_EMAIL=projection@choir.local", "GIT_COMMITTER_DATE=" + date,
	}
	msg := "choir: saved before the projection reset\n\nChoir-Saved-From: " + firstNonEmptyString(head, "(unborn)") + "\n"
	args := []string{"-c", "commit.gpgsign=false", "commit-tree", tree}
	if head != "" {
		args = append(args, "-p", head)
	}
	out, err := git(ctx, repo, env, []byte(msg), append(args, "-F", "-")...)
	if err != nil {
		return "", err
	}
	if !commitID.MatchString(out) {
		return "", fmt.Errorf("git wrote a malformed commit id %q", out)
	}
	return out, nil
}

// untrackedInTheWay lists the paths a reset to expected would overwrite that
// the scratch index does not track: a path expected tracks that the index
// does not (a file added on the way, or one the owner untracked with
// `git rm --cached`) and that exists in the work tree, or an ancestor of one
// that is a file there. Ignored files count: a reset overwrites them too.
func untrackedInTheWay(ctx context.Context, repo string, env []string, expected string) ([]string, error) {
	raw, err := gitRaw(ctx, repo, nil, nil, "ls-tree", "-r", "-z", "--name-only", "--full-tree", expected)
	if err != nil {
		return nil, err
	}
	indexed, err := gitRaw(ctx, repo, env, nil, "ls-files", "-z")
	if err != nil {
		return nil, err
	}
	tracked := map[string]bool{}
	for _, p := range strings.Split(string(indexed), "\x00") {
		if p != "" {
			tracked[p] = true
		}
	}
	seen := map[string]bool{}
	var candidates []string
	for _, p := range strings.Split(string(raw), "\x00") {
		if p == "" || tracked[p] {
			continue
		}
		for q := p; q != "." && q != "/" && q != ""; q = path.Dir(q) {
			info, err := os.Lstat(filepath.Join(repo, filepath.FromSlash(q)))
			if err != nil {
				continue
			}
			if q != p && info.IsDir() {
				continue // an ancestor that is a directory is in nobody's way
			}
			if !seen[q] {
				seen[q] = true
				candidates = append(candidates, q)
			}
		}
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	var out []string
	for i := 0; i < len(candidates); i += 100 {
		end := i + 100
		if end > len(candidates) {
			end = len(candidates)
		}
		args := []string{"ls-files", "-z", "--others", "--"}
		for _, p := range candidates[i:end] {
			args = append(args, ":(literal)"+p)
		}
		listed, err := gitRaw(ctx, repo, env, nil, args...)
		if err != nil {
			return nil, err
		}
		for _, p := range strings.Split(string(listed), "\x00") {
			if p != "" {
				out = append(out, p)
			}
		}
	}
	return out, nil
}

// clearAssumeUnchanged clears the assume-unchanged bit of every entry in the
// scratch index that has it (ls-files -v marks those with a lower-case tag).
func clearAssumeUnchanged(ctx context.Context, repo string, env []string) error {
	raw, err := gitRaw(ctx, repo, env, nil, "ls-files", "-v", "-z")
	if err != nil {
		return err
	}
	var marked []string
	for _, rec := range strings.Split(string(raw), "\x00") {
		if len(rec) < 3 || rec[1] != ' ' {
			continue
		}
		if tag := rec[0]; tag >= 'a' && tag <= 'z' {
			marked = append(marked, rec[2:])
		}
	}
	for i := 0; i < len(marked); i += 100 {
		end := i + 100
		if end > len(marked) {
			end = len(marked)
		}
		if _, err := git(ctx, repo, env, nil, append([]string{"update-index", "--no-assume-unchanged", "--"}, marked[i:end]...)...); err != nil {
			return err
		}
	}
	return nil
}

func firstNonEmptyString(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func copyFile(from, to string) error {
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(to, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return err
	}
	if err := dst.Close(); err != nil {
		return err
	}
	// Keep the index's own mtime: git's racy-entry test compares each entry's
	// mtime with the index file's, and a fresh mtime would make a same-size
	// edit made in the same second as the last index write look clean.
	info, err := src.Stat()
	if err != nil {
		return err
	}
	return os.Chtimes(to, info.ModTime(), info.ModTime())
}

// createSavedRef creates refs/choir/saved/<commit> at commit, never moving an
// existing ref: the empty old value makes git refuse a ref that exists. One
// that already holds commit is success (an earlier attempt made it), and
// created says which.
func createSavedRef(ctx context.Context, repo, commit string) (created bool, err error) {
	if !commitID.MatchString(commit) {
		return false, fmt.Errorf("saved ref: malformed commit id %q", commit)
	}
	ref := RefSavedPrefix + commit
	if _, err := git(ctx, repo, nil, nil, "update-ref", ref, commit, ""); err == nil {
		return true, nil
	} else if got, ok := resolveCommit(ctx, repo, ref); ok && got == commit {
		return false, nil
	} else {
		return false, fmt.Errorf("saved ref %s: %w", ref, err)
	}
}

// updateIndexRemove removes paths from the index env names.
func updateIndexRemove(ctx context.Context, repo string, env, paths []string) error {
	for i := 0; i < len(paths); i += 100 {
		end := i + 100
		if end > len(paths) {
			end = len(paths)
		}
		if _, err := git(ctx, repo, env, nil, append([]string{"update-index", "--force-remove", "--"}, paths[i:end]...)...); err != nil {
			return err
		}
	}
	return nil
}

// placeAfterImage writes tree's entry (base to tree) at each of paths into the
// index env names: a path the patch deletes is removed, any other gets the
// after-image's mode and blob. why is set, with nothing returned as an error,
// when an entry does not fit the index (a file and a directory at one path).
func placeAfterImage(ctx context.Context, repo string, env []string, base, tree string, paths []string) (why string, err error) {
	if len(paths) == 0 {
		return "", nil
	}
	entries, err := treeDiffRaw(ctx, repo, base, tree, paths)
	if err != nil {
		return "", err
	}
	var removes, adds []string
	for _, e := range entries {
		if strings.Trim(e.b.mode, "0") == "" {
			removes = append(removes, e.path)
			continue
		}
		adds = append(adds, e.b.mode+","+e.b.oid+","+e.path)
	}
	if err := updateIndexRemove(ctx, repo, env, removes); err != nil {
		return "", err
	}
	for i := 0; i < len(adds); i += 100 {
		end := i + 100
		if end > len(adds) {
			end = len(adds)
		}
		args := []string{"update-index", "--add"}
		for _, a := range adds[i:end] {
			args = append(args, "--cacheinfo", a)
		}
		if _, err := git(ctx, repo, env, nil, args...); err != nil {
			// No --replace: a file and a directory at one path is refused,
			// and that is "not graftable", not a git failure.
			return err.Error(), nil
		}
	}
	return "", nil
}
