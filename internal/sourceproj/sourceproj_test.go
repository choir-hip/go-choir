package sourceproj

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// These tests drive real git repositories. The failure modes they pin are
// written in docs/design/computer-update-architecture-rnd-2026-10-24.md,
// "U1 build notes": P1 to P9, B1 to B5, C1 to C9, F1 to F4.

func tgit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func numbered(prefix string, n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "%s line %02d\n", prefix, i)
	}
	return b.String()
}

// baseRepo has a frontend text file, a second frontend file and a Go file.
func baseRepo(t *testing.T) (repo, base string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	t.Setenv(EnvSwitch, "1")
	repo = t.TempDir()
	tgit(t, repo, "init", "-q")
	tgit(t, repo, "config", "core.filemode", "false")
	write(t, repo, "frontend/src/a.txt", numbered("a", 40))
	write(t, repo, "frontend/src/b.txt", numbered("b", 40))
	write(t, repo, "internal/x.go", "package x\n\nfunc X() int { return 1 }\n")
	tgit(t, repo, "add", "-A")
	tgit(t, repo, "commit", "-q", "-m", "base")
	return repo, tgit(t, repo, "rev-parse", "HEAD")
}

// patchFrom clones repo at rev, applies edit, and returns the staged patch.
func patchFrom(t *testing.T, repo, rev string, edit func(root string)) []byte {
	t.Helper()
	scratch := t.TempDir()
	tgit(t, scratch, "clone", "-q", "--no-checkout", repo, ".")
	tgit(t, scratch, "checkout", "-q", rev)
	edit(scratch)
	tgit(t, scratch, "add", "-A")
	return []byte(tgit(t, scratch, "diff", "--cached", "--no-color") + "\n")
}

func editLine(t *testing.T, root, rel string, line int, text string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	raw, err := os.ReadFile(full)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(raw), "\n")
	lines[line-1] = text
	if err := os.WriteFile(full, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
}

func opFor(id, base string, patch []byte) *Op {
	return &Op{
		OperationID: id, BundleDigest: strings.Repeat("b", 64), ReleaseDigest: strings.Repeat("c", 64), DecisionEvent: strings.Repeat("d", 64),
		AppliedEventDigest: fmt.Sprintf("%064x", len(id)*7919+int(id[len(id)-1])),
		CreatedAt:          time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC), Base: base, Patch: patch,
	}
}

func reconcile(t *testing.T, in Input) Result {
	t.Helper()
	res, err := Reconcile(context.Background(), in)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	return res
}

func headOf(t *testing.T, repo string) string { return tgit(t, repo, "rev-parse", "HEAD") }

func mustClean(t *testing.T, repo string) {
	t.Helper()
	if out := tgit(t, repo, "status", "--porcelain", "--untracked-files=no"); out != "" {
		t.Fatalf("work tree not clean:\n%s", out)
	}
}

func show(t *testing.T, repo, rev, path string) string {
	t.Helper()
	return tgit(t, repo, "show", rev+":"+path)
}

func TestSwitchOffTouchesNothing(t *testing.T) {
	repo, base := baseRepo(t)
	t.Setenv(EnvSwitch, "")
	patch := patchFrom(t, repo, base, func(r string) { editLine(t, r, "frontend/src/a.txt", 5, "changed") })
	res := reconcile(t, Input{Repo: repo, BootedBase: base, Top: opFor("op-a", base, patch)})
	if res.State != StateDisabled || res.Changed || headOf(t, repo) != base {
		t.Fatalf("P8: switch off must be inert: %+v head %s", res, headOf(t, repo))
	}
	if out, _, err := Cumulative(context.Background(), repo, base, patch); err != nil || string(out) != string(patch) {
		t.Fatalf("P8: Cumulative must pass the patch through: %v", err)
	}
}

func TestAppliesCompoundAndRestoreRewinds(t *testing.T) {
	repo, base := baseRepo(t)
	ctx := context.Background()

	// An empty stack on the booted base: HEAD is the base, attached.
	res := reconcile(t, Input{Repo: repo, BootedBase: base})
	if res.State != StateBase || res.Diverged || headOf(t, repo) != base {
		t.Fatalf("empty stack = %+v", res)
	}

	// Apply A.
	patchA := patchFrom(t, repo, base, func(r string) { editLine(t, r, "frontend/src/a.txt", 5, "A was here") })
	opA := opFor("op-a", base, patchA)
	res = reconcile(t, Input{Repo: repo, BootedBase: base, Top: opA})
	if res.State != StateProjected || !res.Diverged || !res.Changed || res.Repaired {
		t.Fatalf("apply A = %+v", res)
	}
	headA := headOf(t, repo)
	if res.Head != headA || headA == base {
		t.Fatalf("HEAD %s result %s", headA, res.Head)
	}
	mustClean(t, repo)
	if tgit(t, repo, "rev-parse", RefAppliedPrefix+opA.AppliedEventDigest) != headA {
		t.Fatalf("applied ref does not point at A's commit")
	}
	if tgit(t, repo, "rev-parse", RefBase) != base || tgit(t, repo, "symbolic-ref", "HEAD") != Branch {
		t.Fatalf("base ref / attached branch wrong")
	}
	if !strings.Contains(show(t, repo, "HEAD", "frontend/src/a.txt"), "A was here") {
		t.Fatalf("A is not in HEAD")
	}

	// A candidate B is authored against HEAD (A's commit); Freeze composes it.
	patchB := patchFrom(t, repo, headA, func(r string) { editLine(t, r, "frontend/src/b.txt", 7, "B was here") })
	cum, cumBase, err := Cumulative(ctx, repo, headA, patchB)
	if err != nil {
		t.Fatalf("Cumulative: %v", err)
	}
	if cumBase != base {
		t.Fatalf("cumulative base = %s, want the stack base %s", cumBase, base)
	}
	if string(cum) == string(patchB) || !strings.Contains(string(cum), "A was here") || !strings.Contains(string(cum), "B was here") {
		t.Fatalf("F1: the cumulative patch must carry A and B:\n%s", cum)
	}
	opB := opFor("op-b", cumBase, cum)
	res = reconcile(t, Input{Repo: repo, BootedBase: base, Top: opB})
	if res.State != StateProjected || !res.Changed || res.Repaired {
		t.Fatalf("apply B = %+v", res)
	}
	headB := headOf(t, repo)
	if !strings.Contains(show(t, repo, "HEAD", "frontend/src/a.txt"), "A was here") || !strings.Contains(show(t, repo, "HEAD", "frontend/src/b.txt"), "B was here") {
		t.Fatalf("the served tree must carry A and B")
	}
	mustClean(t, repo)

	// Reboot: reconcile is a no-op.
	res = reconcile(t, Input{Repo: repo, BootedBase: base, Top: opB})
	if res.Changed || res.Repaired || headOf(t, repo) != headB {
		t.Fatalf("reboot reconcile must be a no-op: %+v", res)
	}

	// Restore to the head after A: the projection rewinds, to A's own commit.
	res = reconcile(t, Input{Repo: repo, BootedBase: base, Top: opA})
	if !res.Changed || res.Repaired {
		t.Fatalf("P3: restore = %+v (rewinding is not damage)", res)
	}
	if headOf(t, repo) != headA || tgit(t, repo, "rev-parse", RefAppliedPrefix+opA.AppliedEventDigest) != headA {
		t.Fatalf("P3: restored HEAD %s, want A's commit %s", headOf(t, repo), headA)
	}
	if strings.Contains(show(t, repo, "HEAD", "frontend/src/b.txt"), "B was here") {
		t.Fatalf("P3: B must be gone after the restore")
	}
	mustClean(t, repo)

	// A new candidate C is built on A, not on B.
	patchC := patchFrom(t, repo, headA, func(r string) { editLine(t, r, "frontend/src/a.txt", 30, "C was here") })
	cumC, _, err := Cumulative(ctx, repo, headA, patchC)
	if err != nil || strings.Contains(string(cumC), "B was here") || !strings.Contains(string(cumC), "A was here") {
		t.Fatalf("C must be built on A: %v\n%s", err, cumC)
	}

	// Rebuilding the commit in a second clone gives the same id: the
	// bundle's pinned commits stay resolvable after a restore.
	other := t.TempDir()
	tgit(t, other, "clone", "-q", repo, ".")
	tgit(t, other, "checkout", "-q", "--detach", base)
	tgit(t, other, "update-ref", "-d", Branch)
	res = reconcile(t, Input{Repo: other, BootedBase: base, Top: opA})
	if headOf(t, other) != headA {
		t.Fatalf("the projection commit must be reproducible: %s vs %s (%+v)", headOf(t, other), headA, res)
	}
}

func TestEmptyStackAfterRollbackResetsToTheRecordedBase(t *testing.T) {
	repo, base := baseRepo(t)
	patch := patchFrom(t, repo, base, func(r string) { editLine(t, r, "frontend/src/a.txt", 5, "A") })
	reconcile(t, Input{Repo: repo, BootedBase: base, Top: opFor("op-a", base, patch)})
	// A rollback to genesis: no top. The booted base is unknown to the repo
	// (R3), so the recorded base is the target.
	res := reconcile(t, Input{Repo: repo, BootedBase: strings.Repeat("9", 40)})
	if res.State != StateBaseObjectsMissing || headOf(t, repo) != base || res.Repaired {
		t.Fatalf("rollback to genesis = %+v head %s", res, headOf(t, repo))
	}
	mustClean(t, repo)
}

func TestDamageIsRepairedNotStaged(t *testing.T) {
	repo, base := baseRepo(t)
	patch := patchFrom(t, repo, base, func(r string) { editLine(t, r, "frontend/src/a.txt", 5, "A") })
	in := Input{Repo: repo, BootedBase: base, Top: opFor("op-a", base, patch)}
	reconcile(t, in)
	head := headOf(t, repo)

	// An owner or agent edit to a tracked file.
	write(t, repo, "frontend/src/a.txt", "vandalized\n")
	res := reconcile(t, in)
	if !res.Changed || !res.Repaired {
		t.Fatalf("P4: dirty tree = %+v", res)
	}
	mustClean(t, repo)
	if headOf(t, repo) != head || strings.Contains(show(t, repo, "HEAD", "frontend/src/a.txt"), "vandalized") {
		t.Fatalf("P4: the edit must not become a commit")
	}

	// A staged edit.
	write(t, repo, "frontend/src/b.txt", "staged\n")
	tgit(t, repo, "add", "frontend/src/b.txt")
	if res := reconcile(t, in); !res.Repaired {
		t.Fatalf("P4: staged edit = %+v", res)
	}
	mustClean(t, repo)

	// A stranger moved HEAD to a commit of its own.
	write(t, repo, "frontend/src/b.txt", "stranger\n")
	tgit(t, repo, "add", "-A")
	tgit(t, repo, "commit", "-q", "-m", "stranger")
	res = reconcile(t, in)
	if !res.Repaired || headOf(t, repo) != head {
		t.Fatalf("P4: stray commit = %+v head %s", res, headOf(t, repo))
	}
	mustClean(t, repo)

	// Untracked files are not damage: the capsule copies commit objects.
	write(t, repo, "scratch.txt", "kept\n")
	if res := reconcile(t, in); res.Changed {
		t.Fatalf("an untracked file must not move anything: %+v", res)
	}
}

func TestPendingTouchesNothing(t *testing.T) {
	repo, base := baseRepo(t)
	patch := patchFrom(t, repo, base, func(r string) { editLine(t, r, "frontend/src/a.txt", 5, "A") })
	reconcile(t, Input{Repo: repo, BootedBase: base, Top: opFor("op-a", base, patch)})
	head := headOf(t, repo)
	// current names a selfdev release whose operation is not recorded yet.
	res := reconcile(t, Input{Repo: repo, BootedBase: base, Pending: true})
	if res.State != StatePending || res.Changed || headOf(t, repo) != head || !strings.HasPrefix(res.Refusal(), "source_projection_pending") {
		t.Fatalf("P2: pending = %+v", res)
	}
}

func TestUnrebuildableIsNamedAndNeverFallsBack(t *testing.T) {
	repo, base := baseRepo(t)
	patch := patchFrom(t, repo, base, func(r string) { editLine(t, r, "frontend/src/a.txt", 5, "A") })
	reconcile(t, Input{Repo: repo, BootedBase: base, Top: opFor("op-a", base, patch)})
	head := headOf(t, repo)

	cases := map[string]Input{
		"base commit missing":  {Repo: repo, BootedBase: base, Top: opFor("op-x", strings.Repeat("7", 40), patch)},
		"patch does not apply": {Repo: repo, BootedBase: base, Top: opFor("op-x", base, []byte("diff --git a/nope b/nope\n--- a/nope\n+++ b/nope\n@@ -1 +1 @@\n-x\n+y\n"))},
		"bundle unreadable":    {Repo: repo, BootedBase: base, Top: &Op{OperationID: "op-x"}, TopUnavailable: "the applied operation's bundle is not retained on this computer"},
	}
	for name, in := range cases {
		res := reconcile(t, in)
		if res.State != StateUnrebuildable || res.Reason == "" || !strings.HasPrefix(res.Refusal(), "source_projection_unrebuildable") {
			t.Fatalf("P5 %s: %+v", name, res)
		}
		if headOf(t, repo) != head {
			t.Fatalf("P5 %s: HEAD moved to %s; it must stay (never fall back to the base)", name, headOf(t, repo))
		}
	}
}

func TestRepoUnavailable(t *testing.T) {
	_, base := baseRepo(t)
	res := reconcile(t, Input{Repo: t.TempDir(), BootedBase: base})
	if res.State != StateRepoUnavailable || res.Refusal() == "" {
		t.Fatalf("P6: %+v", res)
	}
	if res := reconcile(t, Input{Repo: "", BootedBase: base}); res.State != StateRepoUnavailable {
		t.Fatalf("P6: %+v", res)
	}
}

func TestNoBootedBaseChangesNothing(t *testing.T) {
	repo, base := baseRepo(t)
	res := reconcile(t, Input{Repo: repo})
	if res.State != StateNoBootedBase || res.Refusal() != "" || headOf(t, repo) != base {
		t.Fatalf("P9: %+v", res)
	}
}

// platformMoved adds a commit on top of base that edits rel, in the same
// repository (the guest repo holds the booted base's objects).
func platformMoved(t *testing.T, repo, base string, edit func(root string)) string {
	t.Helper()
	scratch := t.TempDir()
	tgit(t, scratch, "clone", "-q", "--no-checkout", repo, ".")
	tgit(t, scratch, "checkout", "-q", "-b", "platform", base)
	edit(scratch)
	tgit(t, scratch, "add", "-A")
	tgit(t, scratch, "commit", "-q", "-m", "platform moved")
	commit := tgit(t, scratch, "rev-parse", "HEAD")
	tgit(t, repo, "fetch", "-q", scratch, "platform:refs/platform/moved-"+commit[:8])
	return commit
}

func TestEmptyStackMovesToTheBootedBase(t *testing.T) {
	repo, base := baseRepo(t)
	moved := platformMoved(t, repo, base, func(r string) { editLine(t, r, "frontend/src/b.txt", 3, "platform edit") })
	res := reconcile(t, Input{Repo: repo, BootedBase: moved})
	if res.State != StateBase || headOf(t, repo) != moved || res.Repaired {
		t.Fatalf("B1: %+v head %s", res, headOf(t, repo))
	}
	mustClean(t, repo)
}

func TestBaseObjectsMissing(t *testing.T) {
	repo, base := baseRepo(t)
	gone := strings.Repeat("a", 40)
	// Empty stack: nothing moves, the cast is not refused (status quo), the
	// state is named.
	res := reconcile(t, Input{Repo: repo, BootedBase: gone})
	if res.State != StateBaseObjectsMissing || res.Refusal() != "" || headOf(t, repo) != base {
		t.Fatalf("B2: %+v", res)
	}
	// A live stack on a base that is not the booted one, and the booted
	// base cannot be inspected: it cannot be shown upstreamed.
	patch := patchFrom(t, repo, base, func(r string) { editLine(t, r, "frontend/src/a.txt", 5, "A") })
	res = reconcile(t, Input{Repo: repo, BootedBase: gone, Top: opFor("op-a", base, patch)})
	if res.State != StateBaseMoved || !strings.Contains(res.Reason, "not in the guest source repository") || !res.Diverged {
		t.Fatalf("B4/C4: %+v", res)
	}
	// The same stack on the booted base the repo cannot resolve by object but
	// names identically is simply projected.
	res = reconcile(t, Input{Repo: repo, BootedBase: strings.ToUpper(base), Top: opFor("op-a", base, patch)})
	if res.State != StateProjected {
		t.Fatalf("B3: %+v", res)
	}
}

func TestStackBaseMovedRefusesAndKeepsTheProjection(t *testing.T) {
	repo, base := baseRepo(t)
	moved := platformMoved(t, repo, base, func(r string) { editLine(t, r, "frontend/src/b.txt", 3, "platform edit") })
	patch := patchFrom(t, repo, base, func(r string) { editLine(t, r, "frontend/src/a.txt", 5, "A") })
	res := reconcile(t, Input{Repo: repo, BootedBase: moved, Top: opFor("op-a", base, patch)})
	if res.State != StateBaseMoved || !res.Diverged || res.Base != base {
		t.Fatalf("B4: %+v", res)
	}
	want := "stack_base_moved: "
	if !strings.HasPrefix(res.Refusal(), want) || !strings.Contains(res.Refusal(), base) || !strings.Contains(res.Refusal(), moved) {
		t.Fatalf("B4: the refusal must name both bases: %q", res.Refusal())
	}
	if !strings.Contains(show(t, repo, "HEAD", "frontend/src/a.txt"), "A") || strings.Contains(show(t, repo, "HEAD", "frontend/src/b.txt"), "platform edit") {
		t.Fatalf("B4: the identity stays the stack on its own base")
	}
}

func TestUpstreamedStackIsRetiredNotDivergence(t *testing.T) {
	repo, base := baseRepo(t)
	edit := func(r string) { editLine(t, r, "frontend/src/a.txt", 5, "accepted change") }
	patch := patchFrom(t, repo, base, edit)
	opA := opFor("op-a", base, patch)
	reconcile(t, Input{Repo: repo, BootedBase: base, Top: opA})

	// B5/test 1: main took the change verbatim (and more elsewhere).
	moved := platformMoved(t, repo, base, func(r string) {
		edit(r)
		editLine(t, r, "frontend/src/b.txt", 3, "something else landed too")
	})
	res := reconcile(t, Input{Repo: repo, BootedBase: moved, Top: opA})
	if res.State != StateUpstreamed || res.Diverged || res.Refusal() != "" || headOf(t, repo) != moved {
		t.Fatalf("B5: %+v head %s", res, headOf(t, repo))
	}
	mustClean(t, repo)

	// The retirement is a pure function of (stack, booted base): back on the
	// old base the stack is live again (C7: nothing is remembered).
	res = reconcile(t, Input{Repo: repo, BootedBase: base, Top: opA})
	if res.State != StateProjected || !res.Diverged {
		t.Fatalf("C7: %+v", res)
	}
}

func TestUpstreamedWhenMainEditedTheSameFileElsewhere(t *testing.T) {
	repo, base := baseRepo(t)
	patch := patchFrom(t, repo, base, func(r string) { editLine(t, r, "frontend/src/a.txt", 5, "accepted change") })
	op := opFor("op-a", base, patch)
	// Main took the change and also edited a distant line of the same file:
	// the blobs differ, so only the reverse-apply test can see it (test 2).
	moved := platformMoved(t, repo, base, func(r string) {
		editLine(t, r, "frontend/src/a.txt", 5, "accepted change")
		editLine(t, r, "frontend/src/a.txt", 35, "later edit by someone else")
	})
	res := reconcile(t, Input{Repo: repo, BootedBase: moved, Top: op})
	if res.State != StateUpstreamed {
		t.Fatalf("C1/test 2: %+v", res)
	}
}

func TestNotContained(t *testing.T) {
	repo, base := baseRepo(t)
	two := patchFrom(t, repo, base, func(r string) {
		editLine(t, r, "frontend/src/a.txt", 5, "A change")
		editLine(t, r, "frontend/src/b.txt", 5, "B change")
	})
	op := opFor("op-two", base, two)

	cases := map[string]func(root string){
		"C3 partial: only one of two files": func(r string) { editLine(t, r, "frontend/src/a.txt", 5, "A change") },
		"C2 reviewer edited the change": func(r string) {
			editLine(t, r, "frontend/src/a.txt", 5, "A change, reworded")
			editLine(t, r, "frontend/src/b.txt", 5, "B change")
		},
		"unrelated platform change": func(r string) { editLine(t, r, "internal/x.go", 3, "func X() int { return 2 }") },
		"same line, different content": func(r string) {
			editLine(t, r, "frontend/src/a.txt", 5, "other")
			editLine(t, r, "frontend/src/b.txt", 5, "other")
		},
	}
	for name, edit := range cases {
		moved := platformMoved(t, repo, base, edit)
		res := reconcile(t, Input{Repo: repo, BootedBase: moved, Top: op})
		if res.State != StateBaseMoved || !res.Diverged {
			t.Fatalf("%s: %+v", name, res)
		}
	}
}

func TestContainedEdgeCases(t *testing.T) {
	repo, base := baseRepo(t)
	ctx := context.Background()

	// C5: a deletion is contained only when the path is gone.
	del := patchFrom(t, repo, base, func(r string) { _ = os.Remove(filepath.Join(r, "frontend/src/b.txt")) })
	tree, err := patchedTree(ctx, repo, base, del, false)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := Contained(ctx, repo, base, base, del, tree); err != nil || ok {
		t.Fatalf("C5: a deletion is not in a base that still has the file: %v %v", ok, err)
	}
	gone := platformMoved(t, repo, base, func(r string) { _ = os.Remove(filepath.Join(r, "frontend/src/b.txt")) })
	if ok, err := Contained(ctx, repo, gone, base, del, tree); err != nil || !ok {
		t.Fatalf("C5: a deletion is in a base that deleted the file: %v %v", ok, err)
	}

	// A new file: contained when identical, not when the content differs.
	add := patchFrom(t, repo, base, func(r string) { write(t, r, "frontend/src/new.txt", "hello\n") })
	addTree, _ := patchedTree(ctx, repo, base, add, false)
	same := platformMoved(t, repo, base, func(r string) { write(t, r, "frontend/src/new.txt", "hello\n") })
	diff := platformMoved(t, repo, base, func(r string) { write(t, r, "frontend/src/new.txt", "hello world\n") })
	if ok, _ := Contained(ctx, repo, same, base, add, addTree); !ok {
		t.Fatalf("an identical new file is contained")
	}
	if ok, _ := Contained(ctx, repo, diff, base, add, addTree); ok {
		t.Fatalf("a different new file is not contained")
	}

	// A patch that changes nothing is contained everywhere.
	if ok, err := Contained(ctx, repo, base, base, nil, mustTree(t, repo, base)); err != nil || !ok {
		t.Fatalf("an empty effect: %v %v", ok, err)
	}
}

func mustTree(t *testing.T, repo, commit string) string {
	return tgit(t, repo, "rev-parse", commit+"^{tree}")
}

func TestCumulativeFailureModes(t *testing.T) {
	repo, base := baseRepo(t)
	ctx := context.Background()

	// F2: an empty stack composes nothing: the bundle is today's bytes.
	patch := patchFrom(t, repo, base, func(r string) { editLine(t, r, "frontend/src/a.txt", 5, "A") })
	got, gotBase, err := Cumulative(ctx, repo, base, patch)
	if err != nil || string(got) != string(patch) || gotBase != base {
		t.Fatalf("F2: %v base %s", err, gotBase)
	}

	// F1/net-zero: a candidate that undoes the stack leaves nothing to release.
	reconcile(t, Input{Repo: repo, BootedBase: base, Top: opFor("op-a", base, patch)})
	head := headOf(t, repo)
	undo := patchFrom(t, repo, head, func(r string) { editLine(t, r, "frontend/src/a.txt", 5, "a line 05") })
	if _, _, err := Cumulative(ctx, repo, head, undo); err == nil || !strings.Contains(err.Error(), "nothing to release") {
		t.Fatalf("a candidate equal to the base must be refused, got %v", err)
	}

	// A patch that does not apply to its pinned source is refused by name.
	bad := []byte("diff --git a/nope b/nope\n--- a/nope\n+++ b/nope\n@@ -1 +1 @@\n-x\n+y\n")
	if _, _, err := Cumulative(ctx, repo, head, bad); err == nil || !strings.Contains(err.Error(), "does not apply") {
		t.Fatalf("got %v", err)
	}

	// The composed patch is what the host builder applies to the base alone.
	more := patchFrom(t, repo, head, func(r string) {
		editLine(t, r, "frontend/src/b.txt", 9, "B")
		write(t, r, "frontend/src/added.txt", "new\n")
	})
	cum, cbase, err := Cumulative(ctx, repo, head, more)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := patchedTree(ctx, repo, head, more, false)
	if got, err := patchedTree(ctx, repo, cbase, cum, false); err != nil || got != want {
		t.Fatalf("F1: %s vs %s (%v)", got, want, err)
	}
	if SHA256(cum) == "" || len(StackBaseMust(t, repo, head)) != 40 {
		t.Fatalf("trailer")
	}
}

func StackBaseMust(t *testing.T, repo, commit string) string {
	t.Helper()
	b, ok := StackBase(context.Background(), repo, commit)
	if !ok {
		t.Fatalf("no stack base trailer on %s", commit)
	}
	return b
}
