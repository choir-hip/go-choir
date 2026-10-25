package capsule

// Failure modes are listed in manifest_whiteout_test.go. This file proves
// failure modes 6 and 7 without mounting anything: the lower layer is a
// plain directory tree standing for <state>/source-lower/workspace/platform,
// the upper a plain directory, and the whiteout / opaque facts arrive as
// the change list the manifest diff produces for them.

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const platformRoot = "workspace/platform/"

type overlayFixture struct {
	root, lower, upper, merged string
}

func newOverlayFixture(t *testing.T) *overlayFixture {
	t.Helper()
	root := t.TempDir()
	f := &overlayFixture{
		root:   root,
		lower:  filepath.Join(root, "source-lower", "workspace", "platform"),
		upper:  filepath.Join(root, "upper"),
		merged: filepath.Join(root, "merged"),
	}
	for _, d := range []string{f.lower, f.upper, filepath.Join(f.merged, "workspace", "platform")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func putFile(t *testing.T, root, rel, body string, mode os.FileMode) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

// lowerFile puts a base file in the pinned source snapshot (and, until the
// test deletes it, in the merged view).
func (f *overlayFixture) lowerFile(t *testing.T, rel, body string) {
	t.Helper()
	putFile(t, f.lower, rel, body, 0o644)
	putFile(t, filepath.Join(f.merged, "workspace", "platform"), rel, body, 0o644)
}

// upperFile is a file the capsule wrote (copy-up or new): upper and merged.
func (f *overlayFixture) upperFile(t *testing.T, rel, body string) {
	t.Helper()
	putFile(t, filepath.Join(f.upper, "workspace", "platform"), rel, body, 0o644)
	merged := filepath.Join(f.merged, "workspace", "platform", filepath.FromSlash(rel))
	_ = os.RemoveAll(merged)
	putFile(t, filepath.Join(f.merged, "workspace", "platform"), rel, body, 0o644)
}

// mergedRemove is the merged view after the capsule deleted rel.
func (f *overlayFixture) mergedRemove(t *testing.T, rel string) {
	t.Helper()
	if err := os.RemoveAll(filepath.Join(f.merged, "workspace", "platform", filepath.FromSlash(rel))); err != nil {
		t.Fatal(err)
	}
}

func whiteoutChange(rel string) FileChange {
	return FileChange{Path: platformRoot + rel, Kind: ChangeDeleted, Mode: os.ModeDevice | os.ModeCharDevice, Whiteout: true}
}

func deletedPaths(changes []FileChange) []string {
	var out []string
	for _, c := range changes {
		if c.Kind == ChangeDeleted {
			out = append(out, c.Path)
		}
	}
	sort.Strings(out)
	return out
}

func wantPaths(t *testing.T, got []FileChange, want ...string) {
	t.Helper()
	have := deletedPaths(got)
	for i := range want {
		want[i] = platformRoot + want[i]
	}
	sort.Strings(want)
	if strings.Join(have, "\n") != strings.Join(want, "\n") {
		t.Fatalf("deleted paths = %v, want %v", have, want)
	}
}

func (f *overlayFixture) expand(t *testing.T, changes []FileChange) []FileChange {
	t.Helper()
	out, err := expandOverlayDeletions(context.Background(), changes, f.upper, f.lower)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestExpandWhiteoutOfLowerDirectoryDeletesEveryFileUnderIt(t *testing.T) {
	f := newOverlayFixture(t)
	f.lowerFile(t, "gone/a.go", "a\n")
	f.lowerFile(t, "gone/deep/b.go", "b\n")
	f.lowerFile(t, "keep/c.go", "c\n")
	out := f.expand(t, []FileChange{whiteoutChange("gone")})
	wantPaths(t, out, "gone", "gone/a.go", "gone/deep/b.go")
}

func TestExpandWhiteoutOfLowerFileIsUnchanged(t *testing.T) {
	f := newOverlayFixture(t)
	f.lowerFile(t, "a.go", "a\n")
	out := f.expand(t, []FileChange{whiteoutChange("a.go")})
	wantPaths(t, out, "a.go")
}

func TestExpandOpaqueDirectoryDeletesLowerEntriesTheUpperLacks(t *testing.T) {
	f := newOverlayFixture(t)
	f.lowerFile(t, "d/old.go", "old\n")
	f.lowerFile(t, "d/kept.go", "kept\n")
	f.lowerFile(t, "d/sub/x.go", "x\n")
	f.lowerFile(t, "d/sub/y.go", "y\n")
	// The capsule removed and recreated d: only kept.go (new bytes) and
	// sub/y.go (same name, new content) exist again, plus a brand new file.
	f.mergedRemove(t, "d")
	f.upperFile(t, "d/kept.go", "kept, rewritten\n")
	f.upperFile(t, "d/sub/y.go", "y, rewritten\n")
	f.upperFile(t, "d/fresh.go", "fresh\n")
	opaque := FileChange{Path: platformRoot + "d", Kind: ChangeAdded, Mode: os.ModeDir | 0o755, Opaque: true}
	out := f.expand(t, []FileChange{
		opaque,
		{Path: platformRoot + "d/kept.go", Kind: ChangeAdded, Mode: 0o644},
		{Path: platformRoot + "d/sub/y.go", Kind: ChangeAdded, Mode: 0o644},
		{Path: platformRoot + "d/fresh.go", Kind: ChangeAdded, Mode: 0o644},
	})
	wantPaths(t, out, "d/old.go", "d/sub/x.go")
}

func TestExpandOpaqueDirectoryOverLowerFileDeletesTheFile(t *testing.T) {
	f := newOverlayFixture(t)
	f.lowerFile(t, "thing", "was a file\n")
	f.mergedRemove(t, "thing")
	f.upperFile(t, "thing/inner.go", "now a dir\n")
	out := f.expand(t, []FileChange{
		{Path: platformRoot + "thing", Kind: ChangeAdded, Mode: os.ModeDir | 0o755, Opaque: true},
		{Path: platformRoot + "thing/inner.go", Kind: ChangeAdded, Mode: 0o644},
	})
	wantPaths(t, out, "thing")
}

func TestExpandLowerDirectoryReplacedByFileDeletesTheFilesUnderIt(t *testing.T) {
	f := newOverlayFixture(t)
	f.lowerFile(t, "old/a.go", "a\n")
	f.lowerFile(t, "old/b.go", "b\n")
	f.mergedRemove(t, "old")
	f.upperFile(t, "old", "now a file\n")
	out := f.expand(t, []FileChange{{Path: platformRoot + "old", Kind: ChangeAdded, Mode: 0o644}})
	wantPaths(t, out, "old/a.go", "old/b.go")
}

func TestExpandIgnoresDirectoriesOutsideThePinnedSource(t *testing.T) {
	f := newOverlayFixture(t)
	f.lowerFile(t, "release/frontend/index.html", "lower\n")
	// The spawn's own opaque release dir lives outside workspace/platform
	// and must never turn into deletions (failure mode 8).
	out := f.expand(t, []FileChange{
		{Path: "var/lib/artifact/release", Kind: ChangeAdded, Mode: os.ModeDir | 0o755, Opaque: true},
		{Path: "etc/gone", Kind: ChangeDeleted, Mode: os.ModeDevice | os.ModeCharDevice},
	})
	if len(out) != 2 {
		t.Fatalf("expansion touched changes outside workspace/platform: %+v", out)
	}
}

func TestExpandPlainDirectoryAndUpperOnlyChangesAreUnchanged(t *testing.T) {
	f := newOverlayFixture(t)
	f.lowerFile(t, "docs/README.md", "r\n")
	f.upperFile(t, "docs/new.md", "n\n")
	in := []FileChange{
		{Path: platformRoot + "docs", Kind: ChangeModified, Mode: os.ModeDir | 0o755},
		{Path: platformRoot + "docs/new.md", Kind: ChangeAdded, Mode: 0o644},
	}
	if out := f.expand(t, in); len(out) != len(in) {
		t.Fatalf("a plain directory change expanded to %+v", out)
	}
}

// Failure mode 9: a marker on a strict ancestor of the pinned source
// (workspace) hides the whole platform tree. rm -rf /workspace leaves one
// whiteout at workspace and nothing under it; rm -rf and recreate leaves an
// opaque workspace with only the recreated files. Skipping the marker
// freezes a patch that does not reproduce the candidate tree.
func TestExpandWhiteoutAtWorkspaceDeletesThePlatformTree(t *testing.T) {
	f := newOverlayFixture(t)
	f.lowerFile(t, "a.go", "a\n")
	f.lowerFile(t, "d/deep/b.go", "b\n")
	out := f.expand(t, []FileChange{{Path: "workspace", Kind: ChangeDeleted, Mode: os.ModeDevice | os.ModeCharDevice, Whiteout: true}})
	assertDeleted(t, out, "workspace", platformRoot+"a.go", platformRoot+"d/deep/b.go")
}

func TestExpandOpaqueWorkspaceDeletesLowerFilesNotRecreated(t *testing.T) {
	f := newOverlayFixture(t)
	f.lowerFile(t, "kept.go", "kept\n")
	f.lowerFile(t, "old.go", "old\n")
	f.lowerFile(t, "sub/x.go", "x\n")
	f.mergedRemove(t, ".")
	f.upperFile(t, "kept.go", "kept, rewritten\n")
	f.upperFile(t, "brand-new.go", "new\n")
	out := f.expand(t, []FileChange{
		{Path: "workspace", Kind: ChangeModified, Mode: os.ModeDir | 0o755, Opaque: true},
		{Path: platformRoot[:len(platformRoot)-1], Kind: ChangeAdded, Mode: os.ModeDir | 0o755},
		{Path: platformRoot + "kept.go", Kind: ChangeAdded, Mode: 0o644},
		{Path: platformRoot + "brand-new.go", Kind: ChangeAdded, Mode: 0o644},
	})
	assertDeleted(t, out, platformRoot+"old.go", platformRoot+"sub/x.go")
}

// Failure mode 12 (bunch223 review, Devin F1): at workspace/platform or a
// strict ancestor, only a whiteout or a directory is representable. A
// symlink there serves whatever tree it points at (the walk never descends
// it, and a probe through it reads that tree as "held again"); a file or
// device there is refused too rather than guessed at.
func TestExpandRefusesNonDirectoryAtThePlatformBoundary(t *testing.T) {
	for _, path := range []string{"workspace", "workspace/platform"} {
		for _, kind := range []struct {
			name string
			mode os.FileMode
			make func(t *testing.T, f *overlayFixture, at string)
		}{
			{"symlink", os.ModeSymlink | 0o777, func(t *testing.T, f *overlayFixture, at string) {
				// The link points at a staged tree that serves a.go at a
				// platform path, so a probe through it reads a.go as held.
				staged := filepath.Join(f.upper, "var", "lib", "artifact", "fake")
				served := staged
				if at == "workspace" {
					served = filepath.Join(staged, "platform")
				}
				putFile(t, served, "a.go", "staged\n", 0o644)
				mustSymlink(t, staged, filepath.Join(f.upper, filepath.FromSlash(at)))
			}},
			{"file", 0o644, func(t *testing.T, f *overlayFixture, at string) {
				putFile(t, f.upper, at, "a file now\n", 0o644)
			}},
		} {
			t.Run(path+"/"+kind.name, func(t *testing.T) {
				f := newOverlayFixture(t)
				f.lowerFile(t, "a.go", "a\n")
				f.lowerFile(t, "b.go", "b\n")
				kind.make(t, f, path)
				out, err := expandOverlayDeletions(context.Background(), []FileChange{{Path: path, Kind: ChangeAdded, Mode: kind.mode}}, f.upper, f.lower)
				if !errors.Is(err, errOverlayBoundaryNotDirectory) {
					t.Fatalf("expansion of a %s at %s = %v, %+v; want errOverlayBoundaryNotDirectory", kind.name, path, err, out)
				}
			})
		}
	}
}

// Failure mode 13 (bunch223 review): the "held again" probe never follows a
// symlink in any component of the upper path.
func TestExpandHeldProbeDoesNotFollowUpperSymlinks(t *testing.T) {
	f := newOverlayFixture(t)
	f.lowerFile(t, "d/sub/x.go", "x\n")
	f.lowerFile(t, "d/y.go", "y\n")
	upperD := filepath.Join(f.upper, "workspace", "platform", "d")
	staged := filepath.Join(f.upper, "var", "lib", "artifact", "fake")
	putFile(t, staged, "x.go", "staged\n", 0o644)
	mustSymlink(t, staged, filepath.Join(upperD, "sub"))
	out, err := expandOverlayDeletions(context.Background(), []FileChange{
		{Path: platformRoot + "d", Kind: ChangeAdded, Mode: os.ModeDir | 0o755, Opaque: true},
	}, f.upper, f.lower)
	if !errors.Is(err, errOverlayUpperSymlink) {
		t.Fatalf("probe through an upper symlink = %v, %+v; want errOverlayUpperSymlink", err, out)
	}
}

// Failure mode 14 (bunch223 review, Devin F2): a dot-shaped name is either
// a legal file name ("..x"), handled like any other, or a path with a "."
// or ".." component, refused by name; never silently skipped.
func TestExpandHandlesDotShapedNames(t *testing.T) {
	f := newOverlayFixture(t)
	f.lowerFile(t, "..d/a.go", "a\n")
	wantPaths(t, f.expand(t, []FileChange{whiteoutChange("..d")}), "..d", "..d/a.go")
	_, err := expandOverlayDeletions(context.Background(), []FileChange{whiteoutChange("x/../..d")}, f.upper, f.lower)
	if !errors.Is(err, errUnsafeSourcePath) {
		t.Fatalf("a .. component = %v, want errUnsafeSourcePath", err)
	}
}

func TestExpandPlainWorkspaceDirectoryIsUnchanged(t *testing.T) {
	f := newOverlayFixture(t)
	f.lowerFile(t, "a.go", "a\n")
	in := []FileChange{{Path: "workspace", Kind: ChangeModified, Mode: os.ModeDir | 0o755}}
	if out := f.expand(t, in); len(out) != 1 {
		t.Fatalf("a plain workspace directory change expanded to %+v", out)
	}
}

// Failure mode 10: a whiteout is the manifest's fact, carried on the change,
// not re-derived from a character-device mode.
func TestExpandTriggersOnTheWhiteoutFactNotTheMode(t *testing.T) {
	f := newOverlayFixture(t)
	f.lowerFile(t, "gone/a.go", "a\n")
	notWhiteout := FileChange{Path: platformRoot + "gone", Kind: ChangeDeleted, Mode: os.ModeDevice | os.ModeCharDevice}
	if out := f.expand(t, []FileChange{notWhiteout}); len(out) != 1 {
		t.Fatalf("a char-device deletion without the whiteout fact expanded to %+v", out)
	}
	wantPaths(t, f.expand(t, []FileChange{whiteoutChange("gone")}), "gone", "gone/a.go")
}

func assertDeleted(t *testing.T, got []FileChange, want ...string) {
	t.Helper()
	have := deletedPaths(got)
	sort.Strings(want)
	if strings.Join(have, "\n") != strings.Join(want, "\n") {
		t.Fatalf("deleted paths = %v, want %v", have, want)
	}
}

func TestExpandHonoursCancellation(t *testing.T) {
	f := newOverlayFixture(t)
	f.lowerFile(t, "gone/a.go", "a\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := expandOverlayDeletions(ctx, []FileChange{whiteoutChange("gone")}, f.upper, f.lower); err == nil {
		t.Fatal("a cancelled expansion succeeded")
	}
}

// The patch for the expanded change list: deletions render as deletions,
// the builder's applier removes the files, and create-then-delete shows up
// nowhere.
func TestSourcePatchForDeletionsRoundTripsThroughGitApply(t *testing.T) {
	requireGit(t)
	f := newOverlayFixture(t)
	f.lowerFile(t, "frontend/src/lib/EpubApp.svelte", "<script></script>\n")
	f.lowerFile(t, "frontend/src/lib/Other.svelte", "other\n")
	putFile(t, f.lower, "tool.sh", "#!/bin/sh\n", 0o755)
	f.lowerFile(t, "dir/a.go", "a\n")
	f.lowerFile(t, "dir/b.go", "b\n")
	f.lowerFile(t, "rewritten.go", "old body\n")
	f.mergedRemove(t, "frontend/src/lib/EpubApp.svelte")
	f.mergedRemove(t, "tool.sh")
	f.mergedRemove(t, "dir")
	f.upperFile(t, "rewritten.go", "new body\n")
	// created and deleted again: upper-only, so nothing anywhere.
	changes := f.expand(t, []FileChange{
		whiteoutChange("frontend/src/lib/EpubApp.svelte"),
		whiteoutChange("tool.sh"),
		whiteoutChange("dir"),
		{Path: platformRoot + "rewritten.go", Kind: ChangeModified, Mode: 0o644},
	})
	files, err := collectSourcePatchFiles(f.merged, changes)
	if err != nil {
		t.Fatalf("collectSourcePatchFiles: %v", err)
	}
	patch, err := renderSourcePatch(files)
	if err != nil {
		t.Fatal(err)
	}
	text := string(patch)
	for _, want := range []string{
		"diff --git a/frontend/src/lib/EpubApp.svelte b/frontend/src/lib/EpubApp.svelte\ndeleted file mode 100644\n--- a/frontend/src/lib/EpubApp.svelte\n+++ /dev/null\n",
		"deleted file mode 100644\n--- a/dir/a.go\n+++ /dev/null\n",
		"deleted file mode 100644\n--- a/dir/b.go\n+++ /dev/null\n",
		"+new body",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("patch missing %q:\n%s", want, text)
		}
	}
	if err := verifySourcePatch(context.Background(), patch, files); err != nil {
		t.Fatalf("verifySourcePatch refused the deletion patch: %v", err)
	}
	// git apply on a real repository holding the executable base file.
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	for _, rel := range []string{"frontend/src/lib/EpubApp.svelte", "frontend/src/lib/Other.svelte", "tool.sh", "dir/a.go", "dir/b.go", "rewritten.go"} {
		body, err := os.ReadFile(filepath.Join(f.lower, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		mode := os.FileMode(0o644)
		if rel == "tool.sh" {
			mode = 0o755
		}
		putFile(t, repo, rel, string(body), mode)
	}
	if out, err := exec.Command("git", "-C", repo, "add", "-A").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, out)
	}
	patchPath := filepath.Join(t.TempDir(), "source.patch")
	if err := os.WriteFile(patchPath, patch, 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", repo, "apply", patchPath).CombinedOutput(); err != nil {
		t.Fatalf("git apply rejected the deletion patch: %v: %s\n%s", err, out, text)
	}
	for _, gone := range []string{"frontend/src/lib/EpubApp.svelte", "tool.sh", "dir/a.go", "dir/b.go"} {
		if _, err := os.Lstat(filepath.Join(repo, filepath.FromSlash(gone))); !os.IsNotExist(err) {
			t.Fatalf("git apply left %s in place (err=%v)", gone, err)
		}
	}
	if _, err := os.Stat(filepath.Join(repo, "frontend/src/lib/Other.svelte")); err != nil {
		t.Fatalf("git apply removed an untouched file: %v", err)
	}
}

func TestSourcePatchDeleteThenRecreateIsAModification(t *testing.T) {
	requireGit(t)
	f := newOverlayFixture(t)
	f.lowerFile(t, "a.go", "old\n")
	f.upperFile(t, "a.go", "recreated\n")
	// The whiteout was replaced by a real file: the manifest says modified
	// (or added), never deleted, and the patch is a plain modification.
	changes := f.expand(t, []FileChange{{Path: platformRoot + "a.go", Kind: ChangeModified, Mode: 0o644}})
	files, err := collectSourcePatchFiles(f.merged, changes)
	if err != nil {
		t.Fatal(err)
	}
	patch, err := renderSourcePatch(files)
	if err != nil {
		t.Fatal(err)
	}
	text := string(patch)
	if strings.Contains(text, "deleted file") || strings.Contains(text, "/dev/null") {
		t.Fatalf("delete-then-recreate rendered as a deletion:\n%s", text)
	}
	if !strings.Contains(text, "-old") || !strings.Contains(text, "+recreated") {
		t.Fatalf("modification missing from patch:\n%s", text)
	}
	if err := verifySourcePatch(context.Background(), patch, files); err != nil {
		t.Fatal(err)
	}
}

func TestSourcePatchCreateThenDeleteUpperOnlyFileIsNoSourceChange(t *testing.T) {
	f := newOverlayFixture(t)
	// The capsule created tmp.go and deleted it: no upper entry, no
	// whiteout, so the change list is empty and no patch is emitted.
	files, err := collectSourcePatchFiles(f.merged, f.expand(t, nil))
	if err != nil || len(files) != 0 {
		t.Fatalf("files=%v err=%v, want none", files, err)
	}
	// Even when a stale deletion for an upper-only path reaches the patch
	// builder (the last commit held the file), it renders as nothing.
	files, err = collectSourcePatchFiles(f.merged, []FileChange{{Path: platformRoot + "tmp.go", Kind: ChangeDeleted, Mode: 0o644}})
	if err != nil {
		t.Fatal(err)
	}
	patch, err := renderSourcePatch(files)
	if err != nil || len(patch) != 0 {
		t.Fatalf("patch=%q err=%v, want empty", patch, err)
	}
}

func TestSourcePatchRefusesToDeleteASymlink(t *testing.T) {
	f := newOverlayFixture(t)
	f.lowerFile(t, "real.txt", "real\n")
	if err := os.Symlink("real.txt", filepath.Join(f.lower, "link")); err != nil {
		t.Fatal(err)
	}
	_, err := collectSourcePatchFiles(f.merged, []FileChange{whiteoutChange("link")})
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("err = %v, want a named refusal for deleting a symbolic link (failure mode 7)", err)
	}
}

func TestSourcePatchMissingNewFileStillFailsClosed(t *testing.T) {
	f := newOverlayFixture(t)
	// An added/modified change whose merged file is absent is a real
	// inconsistency, not a deletion: it keeps failing the freeze.
	_, err := collectSourcePatchFiles(f.merged, []FileChange{{Path: platformRoot + "ghost.go", Kind: ChangeAdded, Mode: 0o644}})
	if err == nil {
		t.Fatal("an added change with no merged file produced a patch")
	}
}
