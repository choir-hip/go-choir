package sourceproj

import (
	"context"
	"strings"
	"testing"
)

// A superseded rebase (docs/problems/noop-rebase-never-clears-base-moved-2026-11-04.md):
// the rebase round concludes the new base B' supersedes the applied change P
// and freezes no source change. Ways it can fail, written first:
//   - an empty candidate passes without the Superseded flag (an ordinary
//     empty candidate, failure 15's shape);
//   - an empty candidate drops a path of P that applies cleanly to B' (a
//     change B' never touched) and passes;
//   - an empty candidate drops P's change to a file B' deleted and passes;
//   - the library-next shape (P and B' each add the same path with different
//     bytes) is refused, so the round can never end;
//   - the candidate's base is not B' and passes.

// newCollisionRepo: P adds frontend/src/packages/lib/App.svelte and lib.css;
// B' adds the same two paths with other bytes (an export of another version).
func newCollisionRepo(t *testing.T, alsoEditA bool) *rebaseRepo {
	t.Helper()
	repo, base := baseRepo(t)
	p := patchFrom(t, repo, base, func(root string) {
		write(t, root, "frontend/src/packages/lib/App.svelte", "<script>arm version</script>\n")
		write(t, root, "frontend/src/packages/lib/lib.css", ".arm {}\n")
		if alsoEditA {
			editLine(t, root, "frontend/src/a.txt", 5, "a line 05 CHANGED BY P")
		}
	})
	scratch := t.TempDir()
	tgit(t, scratch, "clone", "-q", "--no-checkout", repo, ".")
	tgit(t, scratch, "checkout", "-q", base)
	write(t, scratch, "frontend/src/packages/lib/App.svelte", "<script>platform export</script>\n<p>more</p>\n")
	write(t, scratch, "frontend/src/packages/lib/lib.css", ".platform {}\n")
	write(t, scratch, "frontend/src/packages/lib/Icon.svelte", "<svg/>\n")
	tgit(t, scratch, "add", "-A")
	tgit(t, scratch, "commit", "-q", "-m", "export")
	booted := tgit(t, scratch, "rev-parse", "HEAD")
	tgit(t, repo, "fetch", "-q", scratch, "HEAD:refs/platform/export")
	return &rebaseRepo{repo: repo, base: base, booted: booted, p: p}
}

func (r *rebaseRepo) verifySuperseded(t *testing.T, candidateBase string, flag bool) RebaseVerdict {
	t.Helper()
	v, err := VerifyRebase(context.Background(), r.repo, RebaseCheck{StackBase: r.base, BootedBase: r.booted, Patch: r.p, CandidateBase: candidateBase, Superseded: flag})
	if err != nil {
		t.Fatalf("VerifyRebase: %v", err)
	}
	return v
}

func TestVerifyRebaseSupersededPassesWhenTheBaseTookEveryPathOver(t *testing.T) {
	r := newCollisionRepo(t, false)
	v := r.verifySuperseded(t, r.booted, true)
	if !v.Pass || !v.Superseded || v.Reason != "" {
		t.Fatalf("the export collision must pass as superseded: %+v", v)
	}
	if got := strings.Join(v.Unresolved, ","); got != "frontend/src/packages/lib/App.svelte,frontend/src/packages/lib/lib.css" {
		t.Fatalf("superseded paths = %q", got)
	}
	if len(v.Extra) != 0 {
		t.Fatalf("an empty candidate has no extra paths: %+v", v.Extra)
	}
}

func TestVerifyRebaseEmptyCandidateWithoutTheFlagStaysRefused(t *testing.T) {
	r := newCollisionRepo(t, false)
	if v := r.verifySuperseded(t, r.booted, false); v.Pass || v.Reason != RebaseCandidateDoesNotApply {
		t.Fatalf("an empty candidate without the superseded claim passed: %+v", v)
	}
}

func TestVerifyRebaseSupersededRefusesACleanPathTheBaseNeverTouched(t *testing.T) {
	r := newCollisionRepo(t, true)
	v := r.verifySuperseded(t, r.booted, true)
	if v.Pass || v.Superseded || v.Reason != RebaseCleanPathDiffers || strings.Join(v.Differing, ",") != "frontend/src/a.txt" {
		t.Fatalf("dropping a change the base never took over passed: %+v", v)
	}
}

func TestVerifyRebaseSupersededRefusesAChangeToAFileTheBaseDeleted(t *testing.T) {
	repo, base := baseRepo(t)
	p := patchFrom(t, repo, base, func(root string) { editLine(t, root, "frontend/src/a.txt", 5, "a line 05 CHANGED BY P") })
	scratch := t.TempDir()
	tgit(t, scratch, "clone", "-q", "--no-checkout", repo, ".")
	tgit(t, scratch, "checkout", "-q", base)
	tgit(t, scratch, "rm", "-q", "frontend/src/a.txt")
	tgit(t, scratch, "commit", "-q", "-m", "delete")
	booted := tgit(t, scratch, "rev-parse", "HEAD")
	tgit(t, repo, "fetch", "-q", scratch, "HEAD:refs/platform/del")
	r := &rebaseRepo{repo: repo, base: base, booted: booted, p: p}
	if v := r.verifySuperseded(t, booted, true); v.Pass || v.Reason != RebaseHunkMissing {
		t.Fatalf("dropping P's change to a file the base deleted passed: %+v", v)
	}
}

func TestVerifyRebaseSupersededRefusesAnotherBase(t *testing.T) {
	r := newCollisionRepo(t, false)
	if v := r.verifySuperseded(t, r.base, true); v.Pass || v.Reason != RebaseBaseMismatch {
		t.Fatalf("a superseded candidate on the old base passed: %+v", v)
	}
}
