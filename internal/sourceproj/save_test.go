package sourceproj

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The S4 failure modes, numbered as in
// docs/design/capsule-overlay-s4-source-failure-modes-2026-11-03.md.

type saveLog struct{ calls [][]Saved }

func (l *saveLog) hook(_ context.Context, saved []Saved) error {
	l.calls = append(l.calls, append([]Saved(nil), saved...))
	return nil
}

func savedOf(t *testing.T, res Result, kind string) Saved {
	t.Helper()
	for _, s := range res.Saved {
		if s.Kind == kind {
			return s
		}
	}
	t.Fatalf("no %s save in %+v", kind, res.Saved)
	return Saved{}
}

// 1, 2, 5, 6: the dirty tree, staged new files and untracked files in the way
// are saved, on the tape (the hook) before the reset, and once.
func TestDirtyTreeIsSavedBeforeTheReset(t *testing.T) {
	repo, base := baseRepo(t)
	log := &saveLog{}
	in := Input{Repo: repo, BootedBase: base, OnSave: log.hook}
	reconcile(t, in) // the first projection: HEAD was the creation commit
	moved := platformMoved(t, repo, base, func(r string) {
		editLine(t, r, "frontend/src/b.txt", 3, "platform edit")
		write(t, r, "frontend/src/new.txt", "the platform's new file\n")
	})
	log.calls = nil

	write(t, repo, "frontend/src/a.txt", "owner edit\n")                    // modified
	if err := os.Remove(filepath.Join(repo, "internal/x.go")); err != nil { // deleted
		t.Fatal(err)
	}
	write(t, repo, "notes/staged.txt", "staged new\n") // staged new
	tgit(t, repo, "add", "notes/staged.txt")
	write(t, repo, "frontend/src/new.txt", "owner's untracked file in the way\n")
	write(t, repo, "scratch.txt", "untracked, not in the way\n")

	res := reconcile(t, Input{Repo: repo, BootedBase: moved, OnSave: log.hook})
	if res.State != StateBase || headOf(t, repo) != moved || !res.Changed || !res.Repaired {
		t.Fatalf("%+v head %s", res, headOf(t, repo))
	}
	mustClean(t, repo)
	saved := savedOf(t, res, SavedDirty)
	if !saved.Created || saved.Ref != RefSavedPrefix+saved.Commit || saved.From != base || saved.To != moved {
		t.Fatalf("1: %+v", saved)
	}
	if got := tgit(t, repo, "rev-parse", saved.Ref); got != saved.Commit {
		t.Fatalf("1: the ref holds %s, want %s", got, saved.Commit)
	}
	if got := tgit(t, repo, "rev-parse", saved.Commit+"^"); got != base {
		t.Fatalf("1: the saved commit's parent is %s, want the HEAD it was on %s", got, base)
	}
	for path, want := range map[string]string{
		"frontend/src/a.txt":   "owner edit",
		"notes/staged.txt":     "staged new",
		"frontend/src/new.txt": "owner's untracked file in the way",
	} {
		if got := show(t, repo, saved.Commit, path); got != want {
			t.Fatalf("1/2: %s in the saved commit = %q, want %q", path, got, want)
		}
	}
	if out := tgit(t, repo, "ls-tree", "--name-only", saved.Commit, "internal/x.go"); out != "" {
		t.Fatalf("1: the deletion must be saved: %q", out)
	}
	if out := tgit(t, repo, "ls-tree", "--name-only", saved.Commit, "scratch.txt"); out != "" {
		t.Fatalf("2: an untracked file no reset touches is not saved: %q", out)
	}
	if raw, err := os.ReadFile(filepath.Join(repo, "scratch.txt")); err != nil || !strings.Contains(string(raw), "not in the way") {
		t.Fatalf("2: the untracked file must stay: %v", err)
	}
	if len(log.calls) != 1 || len(log.calls[0]) != len(res.Saved) {
		t.Fatalf("5: the hook must see the saves once: %+v", log.calls)
	}
	// 6: a clean tree at the expected HEAD saves nothing more.
	log.calls = nil
	if res := reconcile(t, Input{Repo: repo, BootedBase: moved, OnSave: log.hook}); res.Changed || len(res.Saved) != 0 || len(log.calls) != 0 {
		t.Fatalf("6: %+v %+v", res, log.calls)
	}
}

// 3: a HEAD nothing under refs/choir/ names is kept, the creation commit too.
func TestAForeignHeadIsSaved(t *testing.T) {
	repo, base := baseRepo(t)
	res := reconcile(t, Input{Repo: repo, BootedBase: base})
	if res.Changed && len(res.Saved) != 0 {
		// HEAD was already the base: nothing moved, so nothing is saved.
		t.Fatalf("%+v", res)
	}
	patch := patchFrom(t, repo, base, func(r string) { editLine(t, r, "frontend/src/a.txt", 5, "A") })
	in := Input{Repo: repo, BootedBase: base, Top: opFor("op-a", base, patch)}
	reconcile(t, in)
	write(t, repo, "frontend/src/b.txt", "stranger\n")
	tgit(t, repo, "add", "-A")
	tgit(t, repo, "commit", "-q", "-m", "stranger")
	stranger := headOf(t, repo)
	res = reconcile(t, in)
	saved := savedOf(t, res, SavedHead)
	if saved.Commit != stranger || tgit(t, repo, "rev-parse", RefSavedPrefix+stranger) != stranger || !res.Repaired {
		t.Fatalf("3: %+v", res)
	}
	if len(res.Saved) != 1 {
		t.Fatalf("3: a clean foreign HEAD has no dirty save: %+v", res.Saved)
	}
}

// The first projection moves off the creation commit and keeps it.
func TestTheCreationCommitIsSavedOnTheFirstProjection(t *testing.T) {
	repo, base := baseRepo(t)
	moved := platformMoved(t, repo, base, func(r string) { editLine(t, r, "frontend/src/b.txt", 3, "platform edit") })
	res := reconcile(t, Input{Repo: repo, BootedBase: moved})
	saved := savedOf(t, res, SavedHead)
	if saved.Commit != base || res.From != base || res.Repaired {
		t.Fatalf("%+v", res)
	}
}

// 4: the same dirt saves to the same ref; an existing ref holding another
// commit is never moved.
func TestSavedRefsAreCreatedNeverMoved(t *testing.T) {
	repo, base := baseRepo(t)
	ctx := context.Background()
	first, err := createSavedRef(ctx, repo, base)
	if err != nil || !first {
		t.Fatalf("create: %v %v", first, err)
	}
	again, err := createSavedRef(ctx, repo, base)
	if err != nil || again {
		t.Fatalf("4: an existing ref with the same commit is success, not a creation: %v %v", again, err)
	}
	// A ref that somehow holds another commit is refused, never overwritten.
	other := tgit(t, repo, "commit-tree", base+"^{tree}", "-m", "other")
	tgit(t, repo, "update-ref", RefSavedPrefix+other, base)
	if _, err := createSavedRef(ctx, repo, other); err == nil {
		t.Fatal("4: a saved ref holding another commit must not be overwritten")
	}
	if got := tgit(t, repo, "rev-parse", RefSavedPrefix+other); got != base {
		t.Fatalf("4: the ref moved to %s", got)
	}

	// Deterministic: the same dirt on the same HEAD is the same commit.
	write(t, repo, "frontend/src/a.txt", "dirt\n")
	c1, err := saveWorkTree(ctx, repo, base, base)
	if err != nil || c1 == "" {
		t.Fatalf("save: %q %v", c1, err)
	}
	c2, err := saveWorkTree(ctx, repo, base, base)
	if err != nil || c2 != c1 {
		t.Fatalf("6: the same dirt saved to %s then %s (%v)", c1, c2, err)
	}
	// A clean tree has nothing to save.
	tgit(t, repo, "checkout", "--", ".")
	if c, err := saveWorkTree(ctx, repo, base, base); err != nil || c != "" {
		t.Fatalf("clean tree saved %q (%v)", c, err)
	}
}

// 5: a hook that fails leaves the work tree and HEAD as they were.
func TestAFailedRecordStopsTheReset(t *testing.T) {
	repo, base := baseRepo(t)
	reconcile(t, Input{Repo: repo, BootedBase: base})
	write(t, repo, "frontend/src/a.txt", "owner edit\n")
	failing := func(context.Context, []Saved) error { return errors.New("the tape is down") }
	moved := platformMoved(t, repo, base, func(r string) { editLine(t, r, "frontend/src/b.txt", 3, "platform edit") })
	_, err := Reconcile(context.Background(), Input{Repo: repo, BootedBase: moved, OnSave: failing})
	if err == nil || !strings.Contains(err.Error(), "the tape is down") || !strings.Contains(err.Error(), "nothing was reset") {
		t.Fatalf("5: %v", err)
	}
	if headOf(t, repo) != base {
		t.Fatalf("5: HEAD moved to %s", headOf(t, repo))
	}
	if raw, _ := os.ReadFile(filepath.Join(repo, "frontend/src/a.txt")); string(raw) != "owner edit\n" {
		t.Fatalf("5: the edit was reset: %q", raw)
	}
	// The next attempt, with the tape back, saves the same ref and resets.
	log := &saveLog{}
	res := reconcile(t, Input{Repo: repo, BootedBase: moved, OnSave: log.hook})
	saved := savedOf(t, res, SavedDirty)
	if saved.Created || len(log.calls) != 1 || headOf(t, repo) != moved {
		t.Fatalf("5: the retry must hand the hook the ref the failed attempt made: %+v %+v", saved, log.calls)
	}
}

// 7, 9, 10: the graft when the booted base left the patch's paths alone.
func TestAStackOnAMovedBaseIsGraftedWhenItsPathsAreUnchanged(t *testing.T) {
	repo, base := baseRepo(t)
	ctx := context.Background()
	patch := patchFrom(t, repo, base, func(r string) {
		editLine(t, r, "frontend/src/a.txt", 5, "the stack's change")
		write(t, r, "frontend/src/added.txt", "added by the stack\n")
		if err := os.Remove(filepath.Join(r, "frontend/src/b.txt")); err != nil {
			t.Fatal(err)
		}
	})
	op := opFor("op-a", base, patch)
	reconcile(t, Input{Repo: repo, BootedBase: base, Top: op})
	served := headOf(t, repo)
	moved := platformMoved(t, repo, base, func(r string) {
		editLine(t, r, "internal/x.go", 3, "func X() int { return 2 }")
		write(t, r, "docs/new.md", "a platform doc\n")
	})
	res := reconcile(t, Input{Repo: repo, BootedBase: moved, Top: op})
	if res.State != StateRematerialized || res.Refusal() != "" || !res.Diverged || res.Base != moved || res.ServedBase != base {
		t.Fatalf("7: %+v", res)
	}
	head := headOf(t, repo)
	mustClean(t, repo)
	if parent := tgit(t, repo, "rev-parse", head+"^"); parent != moved {
		t.Fatalf("10: the grafted commit's parent is %s, want %s", parent, moved)
	}
	if got, ok := StackBase(ctx, repo, head); !ok || got != moved {
		t.Fatalf("10: Choir-Stack-Base = %s %v, want %s", got, ok, moved)
	}
	// Byte-identical: the patch's paths hold its after-image, the rest is B'.
	if show(t, repo, head, "frontend/src/a.txt") != show(t, repo, served, "frontend/src/a.txt") ||
		show(t, repo, head, "frontend/src/added.txt") != "added by the stack" ||
		tgit(t, repo, "ls-tree", "--name-only", head, "frontend/src/b.txt") != "" {
		t.Fatal("7: the graft is not the patch's after-image")
	}
	if show(t, repo, head, "internal/x.go") != show(t, repo, moved, "internal/x.go") || show(t, repo, head, "docs/new.md") != "a platform doc" {
		t.Fatal("7: the graft must keep the booted base's other paths")
	}
	// 9: deterministic; a second reconcile changes nothing.
	again := reconcile(t, Input{Repo: repo, BootedBase: moved, Top: op})
	if again.Changed || headOf(t, repo) != head || again.State != StateRematerialized {
		t.Fatalf("9: %+v", again)
	}
	// The served projection commit stays named by the applied ref.
	if got := tgit(t, repo, "rev-parse", RefAppliedPrefix+op.AppliedEventDigest); got != served {
		t.Fatalf("the applied ref names %s, want the served projection %s", got, served)
	}
	// 10: a capsule cast from the graft freezes a cumulative patch on B'.
	capsulePatch := patchFrom(t, repo, head, func(r string) { editLine(t, r, "frontend/src/a.txt", 9, "the capsule's change") })
	cumulative, gotBase, err := Cumulative(ctx, repo, head, capsulePatch)
	if err != nil || gotBase != moved {
		t.Fatalf("10: Cumulative base %s err %v, want %s", gotBase, err, moved)
	}
	tree, err := patchedTree(ctx, repo, moved, cumulative, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"frontend/src/added.txt", "internal/x.go"} {
		if tgit(t, repo, "show", tree+":"+p) != show(t, repo, head, p) {
			t.Fatalf("10: the cumulative patch on B' lost %s", p)
		}
	}
}

// 7: a touched path the booted base changed (a different line of the same
// file) is not grafted; 8: a directory/file clash at an ancestor is not.
func TestAGraftIsRefusedWhenTheBaseTouchedThePatchsPaths(t *testing.T) {
	repo, base := baseRepo(t)
	patch := patchFrom(t, repo, base, func(r string) { editLine(t, r, "frontend/src/a.txt", 5, "the stack's change") })
	op := opFor("op-a", base, patch)
	moved := platformMoved(t, repo, base, func(r string) { editLine(t, r, "frontend/src/a.txt", 35, "far away, same file") })
	if res := reconcile(t, Input{Repo: repo, BootedBase: moved, Top: op}); res.State != StateBaseMoved || !strings.HasPrefix(res.Refusal(), "stack_base_moved: ") {
		t.Fatalf("7: %+v", res)
	}

	// 8: the patch adds pkg/a/b; the booted base adds a file at pkg/a.
	repo, base = baseRepo(t)
	patch = patchFrom(t, repo, base, func(r string) { write(t, r, "pkg/a/b", "the stack's file\n") })
	op = opFor("op-df", base, patch)
	moved = platformMoved(t, repo, base, func(r string) { write(t, r, "pkg/a", "the platform's file\n") })
	res := reconcile(t, Input{Repo: repo, BootedBase: moved, Top: op})
	if res.State != StateBaseMoved || !strings.Contains(res.Reason, "does not fit") {
		t.Fatalf("8: %+v", res)
	}

	// The patch adds a file where the booted base put a directory.
	repo, base = baseRepo(t)
	patch = patchFrom(t, repo, base, func(r string) { write(t, r, "pkg/c", "the stack's file\n") })
	op = opFor("op-fd", base, patch)
	moved = platformMoved(t, repo, base, func(r string) { write(t, r, "pkg/c/inner", "the platform's file\n") })
	if res := reconcile(t, Input{Repo: repo, BootedBase: moved, Top: op}); res.State != StateBaseMoved {
		t.Fatalf("7 (dir at a file's path): %+v", res)
	}
}

// 11: a graft that equals the booted base is upstreamed. Contained answers
// this first in practice; the graft must agree when asked directly.
func TestAGraftEqualToTheBootedBaseIsUpstreamed(t *testing.T) {
	repo, base := baseRepo(t)
	ctx := context.Background()
	moved := platformMoved(t, repo, base, func(r string) { editLine(t, r, "internal/x.go", 3, "func X() int { return 2 }") })
	graft, ok, why, err := graftTree(ctx, repo, base, moved, tgit(t, repo, "rev-parse", base+"^{tree}"))
	if err != nil || !ok || graft != tgit(t, repo, "rev-parse", moved+"^{tree}") {
		t.Fatalf("11: %s %v %q %v", graft, ok, why, err)
	}
}
