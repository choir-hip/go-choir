package pkgrelease

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// gitTimeout bounds every git call: a publish must not hang on git.
const gitTimeout = 60 * time.Second

// maxManifestBytes bounds the manifest read from a tree.
const maxManifestBytes = 256 << 10

// gitRun runs git in dir. The caller's GIT_* environment never redirects the
// repository, and neither the system nor the user's global config applies:
// a diff.noprefix or a color setting there would change the patch bytes.
// safe.directory names dir on the command line, the one protected scope
// that lets a service read a checkout another user owns, as the host
// builder does (internal/builder/util.go).
func gitRun(ctx context.Context, dir string, env []string, stdin []byte, args ...string) ([]byte, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("git directory is not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-c", "safe.directory=" + dir, "-C", dir}, args...)...)
	base := make([]string, 0, len(os.Environ())+2)
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GIT_") {
			base = append(base, kv)
		}
	}
	// GIT_NO_LAZY_FETCH: a missing object in a partial clone (the deploy
	// tree, should it ever be one) is a refusal, never a promisor fetch.
	cmd.Env = append(append(base, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_NO_LAZY_FETCH=1"), env...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		reason := strings.TrimSpace(stderr.String())
		if len(reason) > 600 {
			reason = reason[:600]
		}
		if reason == "" {
			reason = err.Error()
		}
		return nil, fmt.Errorf("git %s: %s", args[0], reason)
	}
	return stdout.Bytes(), nil
}

func gitLine(ctx context.Context, dir string, env []string, stdin []byte, args ...string) (string, error) {
	out, err := gitRun(ctx, dir, env, stdin, args...)
	return strings.TrimSpace(string(out)), err
}

// scratch is a throwaway place for indexes and new objects. Over a
// repository it borrows that repository's objects as a read-only alternate,
// so nothing is ever written there; alone (corpusd) it is its own empty
// repository.
type scratch struct {
	dir  string // the repository git runs in
	env  []string
	tmp  string
	next int
}

func newScratch(ctx context.Context, repo string) (*scratch, error) {
	tmp, err := os.MkdirTemp("", "choir-pkgrelease-")
	if err != nil {
		return nil, err
	}
	s := &scratch{tmp: tmp}
	if repo == "" {
		s.dir = filepath.Join(tmp, "repo")
		if _, err := gitRun(ctx, tmp, nil, nil, "init", "-q", "--object-format=sha1", s.dir); err != nil {
			s.close()
			return nil, err
		}
		return s, nil
	}
	common, err := gitLine(ctx, repo, nil, nil, "rev-parse", "--git-common-dir")
	if err != nil {
		s.close()
		return nil, fmt.Errorf("locate repository objects: %w", err)
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(repo, common)
	}
	objects := filepath.Join(tmp, "objects")
	if err := os.MkdirAll(objects, 0o700); err != nil {
		s.close()
		return nil, err
	}
	s.dir = repo
	s.env = []string{"GIT_OBJECT_DIRECTORY=" + objects, "GIT_ALTERNATE_OBJECT_DIRECTORIES=" + filepath.Join(common, "objects")}
	return s, nil
}

func (s *scratch) close() { _ = os.RemoveAll(s.tmp) }

// index returns the environment of a fresh index file.
func (s *scratch) index() []string {
	s.next++
	return append(append([]string{}, s.env...), fmt.Sprintf("GIT_INDEX_FILE=%s/index-%d", s.tmp, s.next))
}

func (s *scratch) git(ctx context.Context, env []string, stdin []byte, args ...string) (string, error) {
	if env == nil {
		env = s.env
	}
	return gitLine(ctx, s.dir, env, stdin, args...)
}

// apply applies patch onto tree (EmptyTree for nothing) in a fresh index
// and returns the tree it writes.
func (s *scratch) apply(ctx context.Context, tree string, patch []byte) (string, error) {
	env := s.index()
	readArgs := []string{"read-tree", tree}
	if tree == EmptyTree {
		readArgs = []string{"read-tree", "--empty"}
	}
	if _, err := s.git(ctx, env, nil, readArgs...); err != nil {
		return "", err
	}
	if _, err := s.git(ctx, env, patch, "apply", "--cached", "--whitespace=nowarn", "-"); err != nil {
		return "", err
	}
	out, err := s.git(ctx, env, nil, "write-tree")
	if err != nil {
		return "", err
	}
	if !gitObjectID.MatchString(out) {
		return "", fmt.Errorf("git wrote a malformed tree id %q", out)
	}
	return out, nil
}

// prefixed is the full-path tree that holds only sub at root.
func (s *scratch) prefixed(ctx context.Context, root, sub string) (string, error) {
	if sub == EmptyTree {
		return EmptyTree, nil
	}
	env := s.index()
	if _, err := s.git(ctx, env, nil, "read-tree", "--empty"); err != nil {
		return "", err
	}
	if _, err := s.git(ctx, env, nil, "read-tree", "--prefix="+root+"/", sub); err != nil {
		return "", err
	}
	return s.git(ctx, env, nil, "write-tree")
}

// diff is the binary diff between two trees, at full index, with no
// renames, so git apply rebuilds exactly b from a.
func (s *scratch) diff(ctx context.Context, a, b string) ([]byte, error) {
	out, err := gitRun(ctx, s.dir, s.env, nil, "diff-tree", "-p", "-r", "--binary", "--full-index", "--no-renames", "--no-color", "--no-ext-diff", a, b)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// subtree is the tree id at root in treeish; present is false when treeish
// holds no directory there.
func subtreeIn(ctx context.Context, dir string, env []string, treeish, root string) (string, bool, error) {
	if _, err := gitLine(ctx, dir, env, nil, "rev-parse", "--verify", "-q", treeish+"^{tree}"); err != nil {
		return "", false, fmt.Errorf("%s is not a tree here: %w", treeish, err)
	}
	// <rev>:<path> takes the rest as the path, so the object's type is asked
	// separately: a file at root is no package directory.
	out, err := gitLine(ctx, dir, env, nil, "rev-parse", "--verify", "-q", treeish+":"+root)
	if err != nil || out == "" {
		return "", false, nil
	}
	if !gitObjectID.MatchString(out) {
		return "", false, fmt.Errorf("git answered a malformed object id %q", out)
	}
	if kind, err := gitLine(ctx, dir, env, nil, "cat-file", "-t", out); err != nil || kind != "tree" {
		return "", false, nil
	}
	return out, true, nil
}

// treeEntry is one line of git ls-tree -r -t.
type treeEntry struct {
	mode, kind, path string
}

func (s *scratch) entries(ctx context.Context, tree string) ([]treeEntry, error) {
	out, err := gitRun(ctx, s.dir, s.env, nil, "ls-tree", "-r", "-t", "-z", "--full-tree", tree)
	if err != nil {
		return nil, err
	}
	var entries []treeEntry
	for _, line := range strings.Split(string(out), "\x00") {
		if line == "" {
			continue
		}
		meta, name, ok := strings.Cut(line, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 {
			return nil, fmt.Errorf("malformed ls-tree line %q", line)
		}
		entries = append(entries, treeEntry{mode: fields[0], kind: fields[1], path: name})
	}
	return entries, nil
}

// confined requires every entry of a full-path tree to lie under root (its
// ancestors are trees), every file under root to be a regular file of mode
// 100644 or 100755, and no .gitattributes or .gitmodules in any case: no
// symlink, no submodule, no attribute that changes how an adopter checks
// the bytes out (design 3.2, check 1; L2, L3).
func (s *scratch) confined(ctx context.Context, tree, root string) error {
	entries, err := s.entries(ctx, tree)
	if err != nil {
		return refuse(ReasonContentNotConfined, "the tree cannot be listed: %v", err)
	}
	var outside, tricks []string
	for _, e := range entries {
		under := e.path == root || strings.HasPrefix(e.path, root+"/")
		switch {
		case !under && e.kind == "tree" && strings.HasPrefix(root, e.path+"/"):
			// An ancestor directory of the root.
		case !under:
			outside = append(outside, e.path)
		case e.kind == "tree":
		case e.kind == "commit":
			tricks = append(tricks, e.path+" (a submodule)")
		case e.mode == "120000":
			tricks = append(tricks, e.path+" (a symlink)")
		case e.mode != "100644" && e.mode != "100755":
			tricks = append(tricks, e.path+" (mode "+e.mode+")")
		case strings.EqualFold(path.Base(e.path), ".gitattributes") || strings.EqualFold(path.Base(e.path), ".gitmodules"):
			// Case-insensitive: on a case-insensitive checkout .GITATTRIBUTES
			// is .gitattributes.
			tricks = append(tricks, e.path)
		}
	}
	if len(outside) > 0 {
		return refuse(ReasonContentNotConfined, "paths outside %s: %s", root, listSome(outside))
	}
	if len(tricks) > 0 {
		return refuse(ReasonContentNotConfined, "entries a package may not carry: %s", listSome(tricks))
	}
	return nil
}

func listSome(items []string) string {
	sort.Strings(items)
	if len(items) > 20 {
		return strings.Join(items[:20], ", ") + fmt.Sprintf(" and %d more", len(items)-20)
	}
	return strings.Join(items, ", ")
}
