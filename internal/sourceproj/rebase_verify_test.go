package sourceproj

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// U4 slice 3 (docs/design/u4-stack-rebase-build-2026-10-26.md, question 7 and
// failure mode D11): VerifyRebase checks that a rebase candidate really
// contains the applied change P. Ways it can fail, written first:
//   - a clean hunk of P is dropped or altered and the candidate passes;
//   - a conflict path is left as the new base has it (dropped) and passes;
//   - the candidate is built on a base other than B' and passes;
//   - extra paths are refused, or unlisted;
//   - a path that merely looks like a glob classifies other paths;
//   - a patch that does not apply to B' is a git error instead of a verdict.

type rebaseRepo struct {
	repo, base, booted string
	p                  []byte
}

// newRebaseRepo: B has a.txt, b.txt; P (on B) edits a.txt line 5 and b.txt line
// 5; B' (a platform commit on B) edits b.txt line 5 differently (so P conflicts
// there) and adds c.txt.
func newRebaseRepo(t *testing.T) *rebaseRepo {
	t.Helper()
	repo, base := baseRepo(t)
	p := patchFrom(t, repo, base, func(root string) {
		editLine(t, root, "frontend/src/a.txt", 5, "a line 05 CHANGED BY P")
		editLine(t, root, "frontend/src/b.txt", 5, "b line 05 CHANGED BY P")
	})
	scratch := t.TempDir()
	tgit(t, scratch, "clone", "-q", "--no-checkout", repo, ".")
	tgit(t, scratch, "checkout", "-q", base)
	editLine(t, scratch, "frontend/src/b.txt", 5, "b line 05 CHANGED BY PLATFORM")
	write(t, scratch, "frontend/src/c.txt", "platform file\n")
	tgit(t, scratch, "add", "-A")
	tgit(t, scratch, "commit", "-q", "-m", "platform")
	booted := tgit(t, scratch, "rev-parse", "HEAD")
	tgit(t, repo, "fetch", "-q", scratch, "HEAD:refs/platform/b2")
	return &rebaseRepo{repo: repo, base: base, booted: booted, p: p}
}

// candidate is a patch on B' made by edit.
func (r *rebaseRepo) candidate(t *testing.T, edit func(root string)) []byte {
	return patchFrom(t, r.repo, r.booted, edit)
}

func (r *rebaseRepo) verify(t *testing.T, candidateBase string, candidate []byte) RebaseVerdict {
	t.Helper()
	v, err := VerifyRebase(context.Background(), r.repo, RebaseCheck{StackBase: r.base, BootedBase: r.booted, Patch: r.p, CandidateBase: candidateBase, Candidate: candidate})
	if err != nil {
		t.Fatalf("VerifyRebase: %v", err)
	}
	return v
}

func TestVerifyRebasePassesACorrectRebaseAndListsExtras(t *testing.T) {
	r := newRebaseRepo(t)
	good := r.candidate(t, func(root string) {
		editLine(t, root, "frontend/src/a.txt", 5, "a line 05 CHANGED BY P")
		editLine(t, root, "frontend/src/b.txt", 5, "b line 05 CHANGED BY PLATFORM AND P")
	})
	v := r.verify(t, r.booted, good)
	if !v.Pass || v.Reason != "" {
		t.Fatalf("verdict %+v", v)
	}
	if strings.Join(v.Clean, ",") != "frontend/src/a.txt" || strings.Join(v.Resolved, ",") != "frontend/src/b.txt" || len(v.Extra) != 0 {
		t.Fatalf("clean %v resolved %v extra %v", v.Clean, v.Resolved, v.Extra)
	}
	withExtra := r.candidate(t, func(root string) {
		editLine(t, root, "frontend/src/a.txt", 5, "a line 05 CHANGED BY P")
		editLine(t, root, "frontend/src/b.txt", 5, "b line 05 CHANGED BY PLATFORM AND P")
		write(t, root, "frontend/src/new.txt", "added while rebasing\n")
	})
	v = r.verify(t, r.booted, withExtra)
	if !v.Pass || strings.Join(v.Extra, ",") != "frontend/src/new.txt" {
		t.Fatalf("extra paths are allowed and listed: %+v", v)
	}
}

func TestVerifyRebaseRefusesADroppedOrAlteredCleanHunk(t *testing.T) {
	r := newRebaseRepo(t)
	dropped := r.candidate(t, func(root string) {
		editLine(t, root, "frontend/src/b.txt", 5, "b line 05 CHANGED BY PLATFORM AND P")
	})
	if v := r.verify(t, r.booted, dropped); v.Pass || v.Reason != RebaseCleanPathDiffers || strings.Join(v.Differing, ",") != "frontend/src/a.txt" {
		t.Fatalf("a dropped clean hunk: %+v", v)
	}
	altered := r.candidate(t, func(root string) {
		editLine(t, root, "frontend/src/a.txt", 5, "a line 05 CHANGED BY P")
		editLine(t, root, "frontend/src/a.txt", 6, "a line 06 SNUCK IN")
		editLine(t, root, "frontend/src/b.txt", 5, "b line 05 CHANGED BY PLATFORM AND P")
	})
	if v := r.verify(t, r.booted, altered); v.Pass || v.Reason != RebaseCleanPathDiffers {
		t.Fatalf("an altered clean path: %+v", v)
	}
}

func TestVerifyRebaseRefusesAnUnresolvedConflictPath(t *testing.T) {
	r := newRebaseRepo(t)
	unresolved := r.candidate(t, func(root string) {
		editLine(t, root, "frontend/src/a.txt", 5, "a line 05 CHANGED BY P")
	})
	v := r.verify(t, r.booted, unresolved)
	if v.Pass || v.Reason != RebaseConflictUnresolved || strings.Join(v.Unresolved, ",") != "frontend/src/b.txt" {
		t.Fatalf("a conflict path left as the new base has it counts as dropped: %+v", v)
	}
}

func TestVerifyRebaseRefusesACandidateOnAnotherBase(t *testing.T) {
	r := newRebaseRepo(t)
	good := patchFrom(t, r.repo, r.base, func(root string) {
		editLine(t, root, "frontend/src/a.txt", 5, "a line 05 CHANGED BY P")
		editLine(t, root, "frontend/src/b.txt", 5, "b line 05 CHANGED BY P")
	})
	// Built on the old base B: refused by its base, whatever its content.
	if v := r.verify(t, r.base, good); v.Pass || v.Reason != RebaseBaseMismatch {
		t.Fatalf("a candidate on the old base: %+v", v)
	}
	// Abbreviated or unresolvable bases are not B' either.
	if v := r.verify(t, "deadbeef", good); v.Pass || v.Reason != RebaseBaseMismatch {
		t.Fatalf("an unresolvable base: %+v", v)
	}
	if v := r.verify(t, r.booted, nil); v.Pass {
		t.Fatalf("an empty candidate passed: %+v", v)
	}
}

func TestVerifyRebaseAPatchThatDoesNotApplyIsAVerdict(t *testing.T) {
	r := newRebaseRepo(t)
	// Authored against B, so its b.txt hunk does not apply to B'.
	stale := patchFrom(t, r.repo, r.base, func(root string) {
		editLine(t, root, "frontend/src/b.txt", 5, "b line 05 CHANGED BY P")
	})
	if v := r.verify(t, r.booted, stale); v.Pass || v.Reason != RebaseCandidateDoesNotApply {
		t.Fatalf("a candidate that does not apply to B': %+v", v)
	}
}

func TestVerifyRebaseAllCleanWhenThePlatformMovedElsewhere(t *testing.T) {
	r := newRebaseRepo(t)
	// P touches only a.txt this time: nothing conflicts with B'.
	r.p = patchFrom(t, r.repo, r.base, func(root string) { editLine(t, root, "frontend/src/a.txt", 5, "a line 05 CHANGED BY P") })
	good := r.candidate(t, func(root string) { editLine(t, root, "frontend/src/a.txt", 5, "a line 05 CHANGED BY P") })
	v := r.verify(t, r.booted, good)
	if !v.Pass || len(v.Resolved) != 0 || len(v.Clean) != 1 {
		t.Fatalf("all-clean rebase: %+v", v)
	}
	// And an empty P is not a rebase.
	r.p = nil
	if _, err := VerifyRebase(context.Background(), r.repo, RebaseCheck{StackBase: r.base, BootedBase: r.booted, Patch: nil, CandidateBase: r.booted, Candidate: good}); err == nil {
		t.Fatal("an empty applied patch must be an error, not a pass")
	}
}

func TestVerifyRebaseAGlobLookingPathIsClassifiedByItself(t *testing.T) {
	repo, base := baseRepo(t)
	write(t, repo, "frontend/src/[x].txt", numbered("x", 10))
	tgit(t, repo, "add", "-A")
	tgit(t, repo, "commit", "-q", "-m", "glob")
	base = headOf(t, repo)
	p := patchFrom(t, repo, base, func(root string) {
		editLine(t, root, "frontend/src/a.txt", 5, "a line 05 CHANGED BY P")
		editLine(t, root, "frontend/src/[x].txt", 2, "x line 02 CHANGED BY P")
	})
	// Candidate drops the a.txt hunk. If [x].txt's glob matched a.txt-like paths
	// the classification would be wrong; the verdict must name exactly a.txt.
	cand := patchFrom(t, repo, base, func(root string) { editLine(t, root, "frontend/src/[x].txt", 2, "x line 02 CHANGED BY P") })
	v, err := VerifyRebase(context.Background(), repo, RebaseCheck{StackBase: base, BootedBase: base, Patch: p, CandidateBase: base, Candidate: cand})
	if err != nil {
		t.Fatal(err)
	}
	if v.Pass || strings.Join(v.Differing, ",") != "frontend/src/a.txt" {
		t.Fatalf("verdict %+v", v)
	}
}

// Review 321, F1 (DeepSeek, high): a path holding both a conflicting hunk and a
// clean one. Path granularity let a candidate that resolved the conflict and
// dropped the clean hunk pass. The check is per hunk: the 3-way merge of
// (B', B, B+P) says what each non-overlapping hunk must look like.
//
// Ways it can fail, written first (the first is the reviewer's reproduction):
//   - the clean hunk of a mixed path is absent and the candidate passes;
//   - a merge that is clean by itself but not by `git apply` (a neighbouring
//     line changed) accepts a candidate that differs from the merge;
//   - a change P made that B' already has fails as "kept base";
//   - a conflict path the candidate deletes counts as resolved;
//   - keeping B' at the conflicting hunk while applying the rest passes.

type mixedRepo struct {
	repo, base, booted string
	p                  []byte
}

// newMixedRepo: B has a.txt (40 lines). P edits line 5 and line 30 (two hunks).
// B' edits line 5 differently, so hunk 1 conflicts and hunk 2 would apply.
func newMixedRepo(t *testing.T, platform func(root string)) *mixedRepo {
	t.Helper()
	repo, base := baseRepo(t)
	p := patchFrom(t, repo, base, func(root string) {
		editLine(t, root, "frontend/src/a.txt", 5, "a line 05 P1")
		editLine(t, root, "frontend/src/a.txt", 30, "a line 30 P2")
	})
	scratch := t.TempDir()
	tgit(t, scratch, "clone", "-q", "--no-checkout", repo, ".")
	tgit(t, scratch, "checkout", "-q", base)
	platform(scratch)
	tgit(t, scratch, "add", "-A")
	tgit(t, scratch, "commit", "-q", "-m", "platform")
	booted := tgit(t, scratch, "rev-parse", "HEAD")
	tgit(t, repo, "fetch", "-q", scratch, "HEAD:refs/platform/mixed")
	return &mixedRepo{repo: repo, base: base, booted: booted, p: p}
}

func (m *mixedRepo) verify(t *testing.T, cand []byte) RebaseVerdict {
	t.Helper()
	v, err := VerifyRebase(context.Background(), m.repo, RebaseCheck{StackBase: m.base, BootedBase: m.booted, Patch: m.p, CandidateBase: m.booted, Candidate: cand})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func (m *mixedRepo) candidate(t *testing.T, edit func(root string)) []byte {
	return patchFrom(t, m.repo, m.booted, edit)
}

func TestVerifyRebaseAMixedPathCannotDropTheCleanHunk(t *testing.T) {
	m := newMixedRepo(t, func(root string) { editLine(t, root, "frontend/src/a.txt", 5, "a line 05 PLATFORM") })
	const a = "frontend/src/a.txt"
	// The reviewer's reproduction: the conflicting hunk resolved, P's second
	// hunk (line 30) absent.
	dropped := m.candidate(t, func(root string) { editLine(t, root, a, 5, "a line 05 PLATFORM+P1") })
	if v := m.verify(t, dropped); v.Pass || v.Reason != RebaseHunkMissing || strings.Join(v.Dropped, ",") != a {
		t.Fatalf("a mixed path that dropped P's second hunk passed: %+v", v)
	}
	// Both resolved: pass, the path is a resolved conflict.
	both := m.candidate(t, func(root string) {
		editLine(t, root, a, 5, "a line 05 PLATFORM+P1")
		editLine(t, root, a, 30, "a line 30 P2")
	})
	if v := m.verify(t, both); !v.Pass || strings.Join(v.Resolved, ",") != a {
		t.Fatalf("a correct mixed-path rebase: %+v", v)
	}
	// The second hunk altered is also a missing hunk.
	altered := m.candidate(t, func(root string) {
		editLine(t, root, a, 5, "a line 05 PLATFORM+P1")
		editLine(t, root, a, 30, "a line 30 P2 ALTERED")
	})
	if v := m.verify(t, altered); v.Pass || v.Reason != RebaseHunkMissing {
		t.Fatalf("an altered clean hunk passed: %+v", v)
	}
	// A path P touches is checked whole, like a clean path: an edit elsewhere in
	// it is not P. Extra edits belong in paths P does not touch.
	elsewhere := m.candidate(t, func(root string) {
		editLine(t, root, a, 5, "a line 05 PLATFORM+P1")
		editLine(t, root, a, 30, "a line 30 P2")
		editLine(t, root, a, 15, "a line 15 touched while rebasing")
	})
	if v := m.verify(t, elsewhere); v.Pass || v.Reason != RebaseHunkMissing {
		t.Fatalf("an extra edit inside a path P touches must be refused, as for a clean path: %+v", v)
	}
	// Keeping B' at the conflicting hunk while applying the rest: kept base.
	kept := m.candidate(t, func(root string) { editLine(t, root, a, 30, "a line 30 P2") })
	if v := m.verify(t, kept); v.Pass || v.Reason != RebaseConflictUnresolved {
		t.Fatalf("keeping the new base's version of the conflict: %+v", v)
	}
}

func TestVerifyRebaseACleanMergeThatGitApplyRefusesMustEqualTheMerge(t *testing.T) {
	// B' changes line 32, inside the context of P's second hunk: `git apply`
	// refuses that hunk, yet the 3-way merge is clean, so the candidate must be
	// exactly the merge.
	m := newMixedRepo(t, func(root string) { editLine(t, root, "frontend/src/a.txt", 32, "a line 32 PLATFORM") })
	const a = "frontend/src/a.txt"
	good := m.candidate(t, func(root string) {
		editLine(t, root, a, 5, "a line 05 P1")
		editLine(t, root, a, 30, "a line 30 P2")
	})
	if v := m.verify(t, good); !v.Pass {
		t.Fatalf("the exact merge: %+v", v)
	}
	missing := m.candidate(t, func(root string) { editLine(t, root, a, 5, "a line 05 P1") })
	if v := m.verify(t, missing); v.Pass || v.Reason != RebaseHunkMissing {
		t.Fatalf("a candidate lacking P's second hunk: %+v", v)
	}
}

func TestVerifyRebaseAChangeTheNewBaseAlreadyHasIsNotKeptBase(t *testing.T) {
	// B' made P's first change itself (upstreamed in part) and also edited
	// line 7, inside the first hunk's context, so `git apply` refuses the hunk:
	// the merge sees identical changes at line 5 and is clean. The candidate
	// that adds only P's second change contains P.
	m := newMixedRepo(t, func(root string) {
		editLine(t, root, "frontend/src/a.txt", 5, "a line 05 P1")
		editLine(t, root, "frontend/src/a.txt", 7, "a line 07 PLATFORM")
	})
	const a = "frontend/src/a.txt"
	good := m.candidate(t, func(root string) { editLine(t, root, a, 30, "a line 30 P2") })
	if v := m.verify(t, good); !v.Pass {
		t.Fatalf("P's first change is already in the base: %+v", v)
	}
	none := m.candidate(t, func(root string) { editLine(t, root, "frontend/src/b.txt", 3, "unrelated") })
	if v := m.verify(t, none); v.Pass {
		t.Fatalf("a candidate that never applies P's second hunk passed: %+v", v)
	}
}

func TestVerifyRebaseDeletingAConflictPathIsNotResolving(t *testing.T) {
	m := newMixedRepo(t, func(root string) { editLine(t, root, "frontend/src/a.txt", 5, "a line 05 PLATFORM") })
	deleted := m.candidate(t, func(root string) {
		if err := os.Remove(filepath.Join(root, "frontend/src/a.txt")); err != nil {
			t.Fatal(err)
		}
	})
	if v := m.verify(t, deleted); v.Pass || v.Reason != RebaseHunkMissing {
		t.Fatalf("deleting a file P modified passed: %+v", v)
	}
}

// Review 328 (Devin 2, 3 and DeepSeek F2): shapes around the hunk check.
//   - a candidate that is the raw conflicted merge, markers and all, passes;
//   - a candidate that drops P's mode change on a conflict path passes;
//   - delete/delete and add/add-identical (B' already made P's change) refuse
//     the correct candidate, forcing content P does not contain.

func TestVerifyRebaseRefusesTheRawConflictedMerge(t *testing.T) {
	m := newMixedRepo(t, func(root string) { editLine(t, root, "frontend/src/a.txt", 5, "a line 05 PLATFORM") })
	const a = "frontend/src/a.txt"
	// The text `git merge-file` writes with its conflict markers left in.
	oursT := numbered("a", 40)
	lines := strings.Split(oursT, "\n")
	lines[4] = "a line 05 PLATFORM"
	ours := strings.Join(lines, "\n")
	baseLines := strings.Split(oursT, "\n")
	baseText := strings.Join(baseLines, "\n")
	theirsLines := strings.Split(oursT, "\n")
	theirsLines[4], theirsLines[29] = "a line 05 P1", "a line 30 P2"
	theirs := strings.Join(theirsLines, "\n")
	merged, conflicts, err := mergeFile(context.Background(), []byte(ours), []byte(baseText), []byte(theirs), "")
	if err != nil || conflicts != 1 || !strings.Contains(string(merged), strings.Repeat("<", conflictMarker)) {
		t.Fatalf("fixture: merge conflicts=%d err=%v", conflicts, err)
	}
	raw := m.candidate(t, func(root string) { write(t, root, a, string(merged)) })
	if v := m.verify(t, raw); v.Pass || v.Reason != RebaseHunkMissing {
		t.Fatalf("a candidate with the conflict markers left in passed: %+v", v)
	}
}

func TestVerifyRebaseAConflictPathKeepsPsModeChange(t *testing.T) {
	repo, base := baseRepo(t)
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	tgit(t, repo, "config", "core.filemode", "true")
	// P edits line 5 and makes the file executable.
	scratch := t.TempDir()
	tgit(t, scratch, "clone", "-q", "--no-checkout", repo, ".")
	tgit(t, scratch, "checkout", "-q", base)
	tgit(t, scratch, "config", "core.filemode", "true")
	editLine(t, scratch, "frontend/src/a.txt", 5, "a line 05 P1")
	if err := os.Chmod(filepath.Join(scratch, "frontend/src/a.txt"), 0o755); err != nil {
		t.Fatal(err)
	}
	tgit(t, scratch, "add", "-A")
	p := []byte(tgit(t, scratch, "diff", "--cached", "--no-color") + "\n")
	if !strings.Contains(string(p), "new mode 100755") {
		t.Fatalf("fixture: P must change the mode:\n%s", p)
	}
	// B' edits the same line.
	pl := t.TempDir()
	tgit(t, pl, "clone", "-q", "--no-checkout", repo, ".")
	tgit(t, pl, "checkout", "-q", base)
	editLine(t, pl, "frontend/src/a.txt", 5, "a line 05 PLATFORM")
	tgit(t, pl, "add", "-A")
	tgit(t, pl, "commit", "-q", "-m", "platform")
	booted := tgit(t, pl, "rev-parse", "HEAD")
	tgit(t, repo, "fetch", "-q", pl, "HEAD:refs/platform/mode")
	check := func(candidate []byte) RebaseVerdict {
		v, err := VerifyRebase(context.Background(), repo, RebaseCheck{StackBase: base, BootedBase: booted, Patch: p, CandidateBase: booted, Candidate: candidate})
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	withMode := patchFromMode(t, repo, booted, true)
	without := patchFromMode(t, repo, booted, false)
	if v := check(withMode); !v.Pass {
		t.Fatalf("the content resolution with P's mode change: %+v", v)
	}
	if v := check(without); v.Pass || v.Reason != RebaseHunkMissing {
		t.Fatalf("a candidate that drops P's mode change passed: %+v", v)
	}
}

func patchFromMode(t *testing.T, repo, rev string, exec bool) []byte {
	t.Helper()
	scratch := t.TempDir()
	tgit(t, scratch, "clone", "-q", "--no-checkout", repo, ".")
	tgit(t, scratch, "checkout", "-q", rev)
	tgit(t, scratch, "config", "core.filemode", "true")
	editLine(t, scratch, "frontend/src/a.txt", 5, "a line 05 PLATFORM+P1")
	if exec {
		if err := os.Chmod(filepath.Join(scratch, "frontend/src/a.txt"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	tgit(t, scratch, "add", "-A")
	return []byte(tgit(t, scratch, "diff", "--cached", "--no-color") + "\n")
}

func TestVerifyRebaseACorrectRebaseWhereTheNewBaseAlreadyMadeTheChange(t *testing.T) {
	// delete/delete: P deletes b.txt, and so did B'. The correct candidate
	// leaves it deleted.
	repo, base := baseRepo(t)
	p := patchFrom(t, repo, base, func(root string) { _ = os.Remove(filepath.Join(root, "frontend/src/b.txt")) })
	pl := t.TempDir()
	tgit(t, pl, "clone", "-q", "--no-checkout", repo, ".")
	tgit(t, pl, "checkout", "-q", base)
	_ = os.Remove(filepath.Join(pl, "frontend/src/b.txt"))
	write(t, pl, "frontend/src/c.txt", "platform\n")
	tgit(t, pl, "add", "-A")
	tgit(t, pl, "commit", "-q", "-m", "platform deleted b")
	booted := tgit(t, pl, "rev-parse", "HEAD")
	tgit(t, repo, "fetch", "-q", pl, "HEAD:refs/platform/dd")
	verify := func(patch []byte, cand []byte) RebaseVerdict {
		v, err := VerifyRebase(context.Background(), repo, RebaseCheck{StackBase: base, BootedBase: booted, Patch: patch, CandidateBase: booted, Candidate: cand})
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	stays := patchFrom(t, repo, booted, func(root string) { write(t, root, "frontend/src/unrelated.txt", "x\n") })
	if v := verify(p, stays); !v.Pass {
		t.Fatalf("delete/delete with the file still deleted: %+v", v)
	}
	restored := patchFrom(t, repo, booted, func(root string) { write(t, root, "frontend/src/b.txt", numbered("b", 40)) })
	if v := verify(p, restored); v.Pass {
		t.Fatalf("restoring a file P deletes passed: %+v", v)
	}

	// add/add identical: P adds n.txt, and B' already added exactly it.
	repo2, base2 := baseRepo(t)
	p2 := patchFrom(t, repo2, base2, func(root string) { write(t, root, "frontend/src/n.txt", "same\n") })
	pl2 := t.TempDir()
	tgit(t, pl2, "clone", "-q", "--no-checkout", repo2, ".")
	tgit(t, pl2, "checkout", "-q", base2)
	write(t, pl2, "frontend/src/n.txt", "same\n")
	tgit(t, pl2, "add", "-A")
	tgit(t, pl2, "commit", "-q", "-m", "platform added n")
	booted2 := tgit(t, pl2, "rev-parse", "HEAD")
	tgit(t, repo2, "fetch", "-q", pl2, "HEAD:refs/platform/aa")
	v2 := func(cand []byte) RebaseVerdict {
		v, err := VerifyRebase(context.Background(), repo2, RebaseCheck{StackBase: base2, BootedBase: booted2, Patch: p2, CandidateBase: booted2, Candidate: cand})
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	if v := v2(patchFrom(t, repo2, booted2, func(root string) { write(t, root, "frontend/src/other.txt", "x\n") })); !v.Pass {
		t.Fatalf("add/add identical, the file left as it is: %+v", v)
	}
	if v := v2(patchFrom(t, repo2, booted2, func(root string) { write(t, root, "frontend/src/n.txt", "changed\n") })); v.Pass {
		t.Fatalf("altering a file P adds, which the base already has identically, passed: %+v", v)
	}
}
