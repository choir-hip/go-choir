package sourceproj

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// S6, the package lock and package-aware containment
// (docs/design/s6-package-lock-build-2026-11-04.md, failures K1 to K12;
// Library design L17). A package root under the stack is an entry with its
// own answer; the shell keeps S4.

const (
	imageRoot   = "frontend/src/packages/image"
	libraryRoot = "frontend/src/packages/library-next"
)

// pkgRepo has a shell (two files) and one first-party package, image.
func pkgRepo(t *testing.T) (repo, base string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	repo = t.TempDir()
	tgit(t, repo, "init", "-q")
	tgit(t, repo, "config", "core.filemode", "false")
	write(t, repo, "frontend/src/a.txt", numbered("a", 40))
	write(t, repo, "frontend/src/b.txt", numbered("b", 40))
	write(t, repo, imageRoot+"/App.svelte", numbered("image app", 40))
	write(t, repo, imageRoot+"/choir-package.json", `{"name":"image"}`+"\n")
	tgit(t, repo, "add", "-A")
	tgit(t, repo, "commit", "-q", "-m", "base")
	return repo, tgit(t, repo, "rev-parse", "HEAD")
}

func lockOp(id, base string, patch []byte) Input {
	return Input{Top: opFor(id, base, patch)}
}

func entryOf(t *testing.T, res Result, root string) PackageEntry {
	t.Helper()
	for _, e := range res.Packages {
		if e.Root == root {
			return e
		}
	}
	t.Fatalf("no package entry for %s in %+v", root, res.Packages)
	return PackageEntry{}
}

func subtree(t *testing.T, repo, rev, root string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", repo, "rev-parse", "--verify", "--quiet", rev+":"+root)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func addLocalLibrary(t *testing.T, r string) {
	write(t, r, libraryRoot+"/App.svelte", numbered("our library", 30))
	write(t, r, libraryRoot+"/choir-package.json", `{"name":"library-next","launcher_order":115}`+"\n")
}

func addExportedLibrary(t *testing.T, r string) {
	write(t, r, libraryRoot+"/App.svelte", numbered("platform library", 60))
	write(t, r, libraryRoot+"/Icon.svelte", "<svg/>\n")
	write(t, r, libraryRoot+"/choir-package.json", `{"name":"library-next","launcher_order":12}`+"\n")
}

// Today's case (2026-11-04, 8d78b7e78): an applied package-only change adds
// library-next; the base gains a different library-next. The stack is not
// stranded: the projection keeps this computer's version (the approved
// content), the entry is a conflict, and casting is admitted.
func TestPackageLockTodaysCaseAnotherLibraryOnTheBaseAdmitsTheCast(t *testing.T) {
	repo, base := pkgRepo(t)
	patch := patchFrom(t, repo, base, func(r string) { addLocalLibrary(t, r) })
	in := lockOp("op-local-library", base, patch)
	in.Repo, in.BootedBase = repo, base
	reconcile(t, in)
	ours := subtree(t, repo, "HEAD", libraryRoot)

	moved := platformMoved(t, repo, base, func(r string) {
		addExportedLibrary(t, r)
		editLine(t, r, "frontend/src/b.txt", 3, "platform shell edit")
	})
	in.BootedBase = moved
	res := reconcile(t, in)
	if res.State != StateRematerialized || res.Refusal() != "" || res.Base != moved || res.ServedBase != base {
		t.Fatalf("today's case must admit the Cast: %+v", res)
	}
	e := entryOf(t, res, libraryRoot)
	if e.State != PackageBehindBase || !e.Conflict || e.Name != "library-next" || e.Tree != ours || e.BaseTree != subtree(t, repo, moved, libraryRoot) || e.ParentTree != "" {
		t.Fatalf("K3: the entry: %+v", e)
	}
	if got := subtree(t, repo, "HEAD", libraryRoot); got != ours {
		t.Fatalf("the projection must hold this computer's library at the root: %s want %s", got, ours)
	}
	if _, err := os.Stat(filepath.Join(repo, libraryRoot, "Icon.svelte")); !os.IsNotExist(err) {
		t.Fatalf("R is placed whole: the base's other files in the root are not mixed in (%v)", err)
	}
	if !strings.Contains(show(t, repo, "HEAD", "frontend/src/b.txt"), "platform shell edit") {
		t.Fatal("the rest of the projection is the booted base")
	}
	if releaseParent := tgit(t, repo, "rev-parse", "HEAD^"); releaseParent != moved {
		t.Fatalf("the re-materialized commit sits on the booted base: parent %s", releaseParent)
	}
	if StackBaseMust(t, repo, headOf(t, repo)) != moved {
		t.Fatal("Freeze composes from the booted base")
	}
	mustClean(t, repo)
}

// L17: an adopter's change to a first-party package, and a deploy that
// edits the same file of that package, no longer strands self-development.
func TestPackageLockL17BaseMoveUnderAnAdoptedPackageKeepsCastingOpen(t *testing.T) {
	repo, base := pkgRepo(t)
	patch := patchFrom(t, repo, base, func(r string) { editLine(t, r, imageRoot+"/App.svelte", 5, "adopted line 05") })
	moved := platformMoved(t, repo, base, func(r string) { editLine(t, r, imageRoot+"/App.svelte", 6, "platform line 06") })
	in := lockOp("op-image", base, patch)
	in.Repo, in.BootedBase = repo, moved
	res := reconcile(t, in)
	if res.State != StateRematerialized || res.Refusal() != "" {
		t.Fatalf("L17: %+v", res)
	}
	e := entryOf(t, res, imageRoot)
	if e.State != PackageBehindBase || !e.Conflict || e.ParentTree != subtree(t, repo, base, imageRoot) {
		t.Fatalf("L17: %+v", e)
	}
	app := show(t, repo, "HEAD", imageRoot+"/App.svelte")
	if !strings.Contains(app, "adopted line 05") || strings.Contains(app, "platform line 06") {
		t.Fatal("L17: the approved package is placed whole")
	}
}

// package_base_moved: the root is still P on the new base, so R is placed
// byte-identically, and the commit is reproducible (K5, K6).
func TestPackageLockRematerializationIsByteIdentical(t *testing.T) {
	repo, base := pkgRepo(t)
	patch := patchFrom(t, repo, base, func(r string) {
		editLine(t, r, imageRoot+"/App.svelte", 5, "adopted line 05")
		write(t, r, imageRoot+"/extra.css", "a{}\n")
	})
	in := lockOp("op-image", base, patch)
	in.Repo, in.BootedBase = repo, base
	first := reconcile(t, in)
	if e := entryOf(t, first, imageRoot); first.State != StateProjected || e.State != PackageAdopted {
		t.Fatalf("on its own base the entry is adopted: %+v", first)
	}
	r := subtree(t, repo, "HEAD", imageRoot)
	moved := platformMoved(t, repo, base, func(root string) { editLine(t, root, "frontend/src/a.txt", 3, "platform") })
	in.BootedBase = moved
	res := reconcile(t, in)
	e := entryOf(t, res, imageRoot)
	if res.State != StateRematerialized || e.State != PackageBaseMoved || e.Conflict || e.Tree != r || e.BaseTree != e.ParentTree {
		t.Fatalf("base moved: %+v %+v", res, e)
	}
	if got := subtree(t, repo, "HEAD", imageRoot); got != r {
		t.Fatalf("K5: the root is not R byte for byte: %s want %s", got, r)
	}
	head := headOf(t, repo)
	tgit(t, repo, "reset", "-q", "--hard", base)
	again := reconcile(t, in)
	if again.Head != head || headOf(t, repo) != head {
		t.Fatalf("K6: the rebuild gave %s, want %s", again.Head, head)
	}
}

// package_behind_base where the base changed other files of the root than
// the stack did: the stack's paths are grafted (S4's rule), nothing is lost,
// and it is not a conflict.
func TestPackageLockBehindBaseOnDisjointPathsIsGraftedNotAConflict(t *testing.T) {
	repo, base := pkgRepo(t)
	patch := patchFrom(t, repo, base, func(r string) { editLine(t, r, imageRoot+"/App.svelte", 5, "adopted") })
	moved := platformMoved(t, repo, base, func(r string) { write(t, r, imageRoot+"/choir-package.json", `{"name":"image","v":2}`+"\n") })
	in := lockOp("op-image", base, patch)
	in.Repo, in.BootedBase = repo, moved
	res := reconcile(t, in)
	e := entryOf(t, res, imageRoot)
	if res.State != StateRematerialized || e.State != PackageBehindBase || e.Conflict {
		t.Fatalf("%+v %+v", res, e)
	}
	if !strings.Contains(show(t, repo, "HEAD", imageRoot+"/App.svelte"), "adopted") || !strings.Contains(show(t, repo, "HEAD", imageRoot+"/choir-package.json"), `"v":2`) {
		t.Fatal("both changes must be in the projection")
	}
}

// package_behind_base gives a conflict when the base changed a path the
// stack changed inside the root.
func TestPackageLockBehindBaseGivesAConflict(t *testing.T) {
	repo, base := pkgRepo(t)
	patch := patchFrom(t, repo, base, func(r string) { editLine(t, r, imageRoot+"/App.svelte", 5, "ours") })
	moved := platformMoved(t, repo, base, func(r string) { editLine(t, r, imageRoot+"/App.svelte", 5, "theirs") })
	in := lockOp("op-image", base, patch)
	in.Repo, in.BootedBase = repo, moved
	e := entryOf(t, reconcile(t, in), imageRoot)
	if e.State != PackageBehindBase || !e.Conflict {
		t.Fatalf("%+v", e)
	}
}

// Per-entry retirement: the base took package A verbatim (an export) and not
// package B. A is upstreamed and left as booted has it; B stays placed.
func TestPackageLockPerEntryRetirementWithAnotherUnpromotedEntry(t *testing.T) {
	repo, base := pkgRepo(t)
	patch := patchFrom(t, repo, base, func(r string) {
		addLocalLibrary(t, r)
		editLine(t, r, imageRoot+"/App.svelte", 5, "adopted")
	})
	moved := platformMoved(t, repo, base, func(r string) { addLocalLibrary(t, r) })
	in := lockOp("op-two", base, patch)
	in.Repo, in.BootedBase = repo, moved
	res := reconcile(t, in)
	if res.State != StateRematerialized {
		t.Fatalf("one entry is still live: %+v", res)
	}
	if e := entryOf(t, res, libraryRoot); e.State != PackageUpstreamed || e.Conflict {
		t.Fatalf("the exported entry retires: %+v", e)
	}
	if e := entryOf(t, res, imageRoot); e.State != PackageBaseMoved {
		t.Fatalf("the other entry stays: %+v", e)
	}
	if subtree(t, repo, "HEAD", libraryRoot) != subtree(t, repo, moved, libraryRoot) || !strings.Contains(show(t, repo, "HEAD", imageRoot+"/App.svelte"), "adopted") {
		t.Fatal("HEAD must hold the base's library and the stack's image")
	}
	// A candidate cast on this projection no longer carries the retired entry.
	cand := patchFrom(t, repo, headOf(t, repo), func(r string) { write(t, r, "frontend/src/c.txt", "c\n") })
	cumulative, cbase, err := Cumulative(context.Background(), repo, headOf(t, repo), cand)
	if err != nil || cbase != moved || strings.Contains(string(cumulative), "library-next") || !strings.Contains(string(cumulative), "image/App.svelte") {
		t.Fatalf("the next candidate's cumulative patch: base %s err %v\n%s", cbase, err, cumulative)
	}
	// Both taken: the whole stack is upstreamed (today's rule).
	both := platformMoved(t, repo, base, func(r string) {
		addLocalLibrary(t, r)
		editLine(t, r, imageRoot+"/App.svelte", 5, "adopted")
	})
	in.BootedBase = both
	if res := reconcile(t, in); res.State != StateUpstreamed || headOf(t, repo) != both {
		t.Fatalf("all retired: %+v", res)
	}
}

// K1: a shell path the base also changed keeps S4: stack_base_moved, with
// the package entries still reported.
func TestPackageLockAShellConflictIsStillStackBaseMoved(t *testing.T) {
	repo, base := pkgRepo(t)
	patch := patchFrom(t, repo, base, func(r string) {
		addLocalLibrary(t, r)
		editLine(t, r, "frontend/src/a.txt", 5, "stack")
	})
	moved := platformMoved(t, repo, base, func(r string) {
		addExportedLibrary(t, r)
		editLine(t, r, "frontend/src/a.txt", 30, "platform")
	})
	in := lockOp("op-mixed", base, patch)
	in.Repo, in.BootedBase = repo, moved
	res := reconcile(t, in)
	if res.State != StateBaseMoved || !strings.Contains(res.Reason, "frontend/src/a.txt") || strings.Contains(res.Reason, "library-next") {
		t.Fatalf("K1: %+v", res)
	}
	if e := entryOf(t, res, libraryRoot); e.State != PackageBehindBase {
		t.Fatalf("K1: entries are still reported: %+v", e)
	}
}

// K7: a crash between the apply and the projection leaves HEAD on the
// previous operation's projection with a dirty file. The next reconcile
// saves the dirty work and lands on the package-aware projection.
func TestPackageLockCrashBetweenApplyAndProjectionIsReconciled(t *testing.T) {
	repo, base := pkgRepo(t)
	first := patchFrom(t, repo, base, func(r string) { editLine(t, r, "frontend/src/b.txt", 9, "op-a") })
	in := lockOp("op-a", base, first)
	in.Repo, in.BootedBase = repo, base
	reconcile(t, in)
	old := headOf(t, repo)
	// op-b applied (cumulative: op-a's change and a local library), the
	// platform moved, and the computer crashed before reconciling.
	second := patchFrom(t, repo, base, func(r string) {
		editLine(t, r, "frontend/src/b.txt", 9, "op-a")
		addLocalLibrary(t, r)
	})
	moved := platformMoved(t, repo, base, func(r string) { addExportedLibrary(t, r) })
	write(t, repo, "frontend/src/a.txt", "half-written\n")
	var saved []Saved
	in = lockOp("op-b", base, second)
	in.Repo, in.BootedBase = repo, moved
	in.OnSave = func(_ context.Context, s []Saved) error { saved = append(saved, s...); return nil }
	res := reconcile(t, in)
	if res.State != StateRematerialized || res.OperationID != "op-b" || res.From != old || len(saved) != 1 || saved[0].Kind != SavedDirty {
		t.Fatalf("K7: %+v saved %+v", res, saved)
	}
	if e := entryOf(t, res, libraryRoot); e.State != PackageBehindBase || !e.Conflict {
		t.Fatalf("K7: %+v", e)
	}
	if !strings.Contains(show(t, repo, "HEAD", "frontend/src/b.txt"), "op-a") {
		t.Fatal("K7: the shell change is grafted")
	}
	mustClean(t, repo)
}

// K9: the stack deletes a package the base also changed: the root is gone in
// the projection, and that is a conflict.
func TestPackageLockADeletedPackageStaysDeleted(t *testing.T) {
	repo, base := pkgRepo(t)
	patch := patchFrom(t, repo, base, func(r string) {
		if err := os.RemoveAll(filepath.Join(r, filepath.FromSlash(imageRoot))); err != nil {
			t.Fatal(err)
		}
	})
	moved := platformMoved(t, repo, base, func(r string) { editLine(t, r, imageRoot+"/App.svelte", 2, "platform") })
	in := lockOp("op-del", base, patch)
	in.Repo, in.BootedBase = repo, moved
	res := reconcile(t, in)
	e := entryOf(t, res, imageRoot)
	if res.State != StateRematerialized || e.State != PackageBehindBase || !e.Conflict || e.Tree != "" {
		t.Fatalf("K9: %+v %+v", res, e)
	}
	if subtree(t, repo, "HEAD", imageRoot) != "" {
		t.Fatal("K9: the root must be absent")
	}
}

// K2: what counts as a package root.
func TestPackageRootOf(t *testing.T) {
	for path, want := range map[string]string{
		"frontend/src/packages/image/App.svelte":         imageRoot,
		"frontend/src/packages/image/deep/x.ts":          imageRoot,
		"frontend/src/packages/README.md":                "",
		"frontend/src/packages":                          "",
		"frontend/src/lib/packages/image/App.svelte":     "",
		"xfrontend/src/packages/image/App.svelte":        "",
		"frontend/src/packages//App.svelte":              "",
		"frontend/src/packages/library-next/Icon.svelte": libraryRoot,
	} {
		if got := PackageRootOf(path); got != want {
			t.Errorf("PackageRootOf(%q) = %q, want %q", path, got, want)
		}
	}
}
