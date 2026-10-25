package capsule

// A round stopped before it froze (the activation budget, the assignment
// deadline, the engineer's run failing without Complete) keeps its source
// changes as an unfinished patch, taken from the capsule just before it is
// destroyed (docs/evidence/selfdev-trace-reviews-2026-10-25.md, problem 1;
// owner: "if we get part of the work done, but not everything, we can
// iteratively improve on it rather than having to start over"). Ways the
// snapshot can fail, written before the code; each has a test below:
//
//  U1. A change outside workspace/platform (scratch /tmp, the release tree,
//      a home cache) is carried.
//  U2. A symbolic link the cell planted is followed, so bytes from outside
//      the source reach the patch; or a planted link is carried as a file.
//  U3. node_modules, .git, a cache, a secret-named path, a binary file or a
//      new empty file (a sandbox mask) is carried.
//  U4. A file above the bound is read whole or truncated, or a patch above
//      the bound is truncated instead of refused.
//  U5. The patch does not reproduce the capsule's tree from its base.
//  U6. A capsule that changed no source yields a patch (must be empty).
//  U7. A deleted base file is lost (the patch must carry the deletion), or
//      a file touched but left equal is carried.
//  U8. The same capsule gives different bytes on a retried snapshot.

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// unfinishedFixture is a capsule's two views: <root>/source-lower is the
// pinned base, <root>/merged the tree the cell sees.
type unfinishedFixture struct {
	root, merged, lower string
}

func newUnfinishedFixture(t *testing.T, base map[string]string) unfinishedFixture {
	t.Helper()
	root := t.TempDir()
	f := unfinishedFixture{root: root, merged: filepath.Join(root, "merged"), lower: filepath.Join(root, "source-lower")}
	prefixed := map[string]string{}
	for rel, content := range base {
		prefixed["workspace/platform/"+rel] = content
	}
	writeTree(t, f.lower, prefixed)
	writeTree(t, f.merged, prefixed)
	return f
}

func (f unfinishedFixture) write(t *testing.T, rel, content string) FileChange {
	t.Helper()
	p := filepath.Join(f.merged, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	kind := ChangeAdded
	if _, err := os.Stat(p); err == nil {
		kind = ChangeModified
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return FileChange{Path: "/" + rel, Kind: kind, Mode: 0o644}
}

func (f unfinishedFixture) remove(t *testing.T, rel string) FileChange {
	t.Helper()
	if err := os.Remove(filepath.Join(f.merged, filepath.FromSlash(rel))); err != nil {
		t.Fatal(err)
	}
	return FileChange{Path: "/" + rel, Kind: ChangeDeleted, Mode: 0o644}
}

// applyTo applies patch to a copy of the base platform tree and returns it.
func (f unfinishedFixture) applyTo(t *testing.T, patch []byte) string {
	t.Helper()
	tree := filepath.Join(t.TempDir(), "tree")
	if out, err := exec.Command("cp", "-R", filepath.Join(f.lower, "workspace", "platform"), tree).CombinedOutput(); err != nil {
		t.Fatalf("copy base: %v %s", err, out)
	}
	patchPath := filepath.Join(t.TempDir(), "source.patch")
	if err := os.WriteFile(patchPath, patch, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "apply", patchPath)
	cmd.Dir = tree
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git apply: %v %s", err, out)
	}
	return tree
}

// U5, U7, U8: the patch reproduces the capsule's tree, carries the deletion,
// skips a file left equal, and is stable.
func TestUnfinishedPatchReproducesTheCapsuleTree(t *testing.T) {
	requireGit(t)
	f := newUnfinishedFixture(t, map[string]string{"a.go": "package a\n", "b.go": "package b\n", "same.go": "package same\n"})
	changes := []FileChange{
		f.write(t, "workspace/platform/a.go", "package a\n\n// half done\nfunc A() {}\n"),
		f.write(t, "workspace/platform/reader/new.go", "package reader\n"),
		f.remove(t, "workspace/platform/b.go"),
		f.write(t, "workspace/platform/same.go", "package same\n"),
	}
	got, err := UnfinishedPatchFromChanges(context.Background(), f.merged, changes, CarriedPatchMaxBytes)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got.Files, ",") != "a.go,b.go,reader/new.go" {
		t.Fatalf("files = %v, want a.go, b.go (deleted), reader/new.go", got.Files)
	}
	if got.SHA256 == "" || len(got.Patch) == 0 {
		t.Fatalf("patch = %+v, want bytes and a digest", got)
	}
	tree := f.applyTo(t, got.Patch)
	if data, _ := os.ReadFile(filepath.Join(tree, "a.go")); string(data) != "package a\n\n// half done\nfunc A() {}\n" {
		t.Fatalf("a.go after apply = %q", data)
	}
	if _, err := os.Stat(filepath.Join(tree, "b.go")); !os.IsNotExist(err) {
		t.Fatalf("b.go survived its deletion: %v", err)
	}
	if data, _ := os.ReadFile(filepath.Join(tree, "reader", "new.go")); string(data) != "package reader\n" {
		t.Fatalf("reader/new.go after apply = %q", data)
	}
	again, err := UnfinishedPatchFromChanges(context.Background(), f.merged, changes, CarriedPatchMaxBytes)
	if err != nil || again.SHA256 != got.SHA256 {
		t.Fatalf("a retried snapshot gave %s (%v), first %s", again.SHA256, err, got.SHA256)
	}
}

// U1, U3: only source; everything else is left out and named.
func TestUnfinishedPatchCarriesSourceOnly(t *testing.T) {
	requireGit(t)
	f := newUnfinishedFixture(t, map[string]string{"a.go": "package a\n"})
	changes := []FileChange{
		f.write(t, "workspace/platform/a.go", "package a\n// edited\n"),
		f.write(t, "tmp/choir-jobs/j1/output.log", "job output\n"),
		f.write(t, "var/lib/artifact/release/frontend/index.html", "<html>\n"),
		f.write(t, "root/.cache/go-build/x", "cache\n"),
		f.write(t, "workspace/platform/frontend/node_modules/left-pad/i.js", "module.exports = 1\n"),
		f.write(t, "workspace/platform/.git/config", "[core]\n"),
		f.write(t, "workspace/platform/frontend/.cache/x", "cache\n"),
		f.write(t, "workspace/platform/.env", "TOKEN=secret-value\n"),
		f.write(t, "workspace/platform/bin/tool", "\x00\x01\x02binary"),
		f.write(t, "workspace/platform/package.json", ""),
	}
	got, err := UnfinishedPatchFromChanges(context.Background(), f.merged, changes, CarriedPatchMaxBytes)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got.Files, ",") != "a.go" {
		t.Fatalf("files = %v, want only a.go", got.Files)
	}
	for _, leak := range []string{"job output", "<html>", "cache", "left-pad", "[core]", "secret-value", "binary", "package.json"} {
		if strings.Contains(string(got.Patch), leak) {
			t.Fatalf("patch carries %q:\n%s", leak, got.Patch)
		}
	}
	omitted := strings.Join(got.Omitted, "\n")
	for _, want := range []string{"node_modules", ".git", ".cache", ".env", "bin/tool", "package.json"} {
		if !strings.Contains(omitted, want) {
			t.Fatalf("omitted list %q does not name %q", omitted, want)
		}
	}
	if strings.Contains(omitted, "choir-jobs") || strings.Contains(omitted, "release") {
		t.Fatalf("omitted names non-source paths, which are not the patch's business: %q", omitted)
	}
}

// U2: a planted link is never read through and never carried.
func TestUnfinishedPatchNeverFollowsAPlantedLink(t *testing.T) {
	requireGit(t)
	f := newUnfinishedFixture(t, map[string]string{"a.go": "package a\n"})
	outside := filepath.Join(f.root, "host-secret")
	if err := os.WriteFile(outside, []byte("HOST SECRET\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	edit := f.write(t, "workspace/platform/a.go", "package a\n// edited\n")
	link := filepath.Join(f.merged, "workspace", "platform", "leak.go")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	got, err := UnfinishedPatchFromChanges(context.Background(), f.merged,
		[]FileChange{edit, {Path: "/workspace/platform/leak.go", Kind: ChangeAdded, Mode: os.ModeSymlink | 0o777}}, CarriedPatchMaxBytes)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got.Patch), "HOST SECRET") || strings.Join(got.Files, ",") != "a.go" {
		t.Fatalf("a planted link was carried: files %v\n%s", got.Files, got.Patch)
	}
	if !strings.Contains(strings.Join(got.Omitted, "\n"), "leak.go (symlink)") {
		t.Fatalf("omitted = %v, want the link named", got.Omitted)
	}

	// A link above a changed file (a directory swapped for a link after the
	// walk recorded a regular file) refuses the snapshot rather than read
	// through it.
	dir := filepath.Join(f.merged, "workspace", "platform", "pkg")
	if err := os.Symlink(f.root, dir); err != nil {
		t.Fatal(err)
	}
	_, err = UnfinishedPatchFromChanges(context.Background(), f.merged,
		[]FileChange{{Path: "/workspace/platform/pkg/host-secret", Kind: ChangeAdded, Mode: 0o644}}, CarriedPatchMaxBytes)
	if !errors.Is(err, errSourcePatchSymlink) {
		t.Fatalf("a change through a link gave %v, want the symlink refusal", err)
	}
}

// U4: over the bound is refused, never truncated.
func TestUnfinishedPatchOversizeIsRefused(t *testing.T) {
	requireGit(t)
	f := newUnfinishedFixture(t, map[string]string{"a.go": "package a\n"})
	big := f.write(t, "workspace/platform/big.txt", strings.Repeat("line of text\n", 400))
	_, err := UnfinishedPatchFromChanges(context.Background(), f.merged, []FileChange{big}, 1024)
	if !errors.Is(err, ErrCarriedPatchTooLarge) {
		t.Fatalf("an oversize file gave %v, want ErrCarriedPatchTooLarge", err)
	}
	var many []FileChange
	for i := 0; i < 8; i++ {
		many = append(many, f.write(t, "workspace/platform/m"+string(rune('a'+i))+".txt", strings.Repeat("x\n", 100)))
	}
	_, err = UnfinishedPatchFromChanges(context.Background(), f.merged, many, 1024)
	if !errors.Is(err, ErrCarriedPatchTooLarge) {
		t.Fatalf("an oversize patch gave %v, want ErrCarriedPatchTooLarge", err)
	}
}

// U6: no source change is empty.
func TestUnfinishedPatchOfNoSourceChangeIsEmpty(t *testing.T) {
	f := newUnfinishedFixture(t, map[string]string{"a.go": "package a\n"})
	changes := []FileChange{
		f.write(t, "tmp/scratch.txt", "scratch\n"),
		f.write(t, "workspace/platform/a.go", "package a\n"),
		f.write(t, "workspace/platform/frontend/node_modules/x.js", "x\n"),
	}
	got, err := UnfinishedPatchFromChanges(context.Background(), f.merged, changes, CarriedPatchMaxBytes)
	if !errors.Is(err, ErrCarriedPatchEmpty) || len(got.Patch) != 0 {
		t.Fatalf("no source change gave %v, %d bytes; want ErrCarriedPatchEmpty", err, len(got.Patch))
	}
}
