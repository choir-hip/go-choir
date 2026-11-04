package sourceproj

import (
	"context"
	"strings"
	"testing"
)

// Clean re-application (owner ruling 2026-11-04). Ways it can fail, written
// before the code:
//   1. a patch that conflicts with the new base is called clean
//      -> ...ConflictIsAVerdictNotClean
//   2. a patch that applies only at an offset is refused (the host builder's
//      `git apply` takes it) -> ...CleanAtAnOffset
//   3. the check moves HEAD or the work tree -> ...LeavesTheRepositoryAlone
//   4. a patch the new base already holds is "clean" -> ...AlreadyInTheBaseIsNotClean
//   5. a missing booted base reads as a conflict instead of an error
//      -> ...MissingBaseIsAnError

func TestReapplyCleanAtAnOffsetAndLeavesTheRepositoryAlone(t *testing.T) {
	r := newRebaseRepo(t)
	// P edits only a.txt; the platform inserts lines above it, so P's hunk
	// lands at an offset and touches nothing the platform changed.
	p := patchFrom(t, r.repo, r.base, func(root string) {
		editLine(t, root, "frontend/src/a.txt", 20, "a line 20 CHANGED BY P")
	})
	scratch := t.TempDir()
	tgit(t, scratch, "clone", "-q", "--no-checkout", r.repo, ".")
	tgit(t, scratch, "checkout", "-q", r.base)
	write(t, scratch, "frontend/src/a.txt", "platform 1\nplatform 2\n"+numbered("a", 40))
	tgit(t, scratch, "add", "-A")
	tgit(t, scratch, "commit", "-q", "-m", "platform")
	booted := tgit(t, scratch, "rev-parse", "HEAD")
	tgit(t, r.repo, "fetch", "-q", scratch, "HEAD:refs/platform/offset")

	head := tgit(t, r.repo, "rev-parse", "HEAD")
	v, err := Reapply(context.Background(), r.repo, r.base, booted, p)
	if err != nil || !v.Clean || v.Tree == "" {
		t.Fatalf("offset patch = %+v, %v", v, err)
	}
	if got := tgit(t, r.repo, "rev-parse", "HEAD"); got != head {
		t.Fatalf("Reapply moved HEAD %s -> %s", head, got)
	}
	if out := tgit(t, r.repo, "status", "--porcelain"); out != "" {
		t.Fatalf("Reapply touched the work tree: %s", out)
	}
	// The tree is the booted base with the change.
	blob := tgit(t, r.repo, "cat-file", "-p", v.Tree+":frontend/src/a.txt")
	if !strings.Contains(blob, "platform 1") || !strings.Contains(blob, "a line 20 CHANGED BY P") {
		t.Fatalf("re-applied tree a.txt = %q", blob)
	}
}

func TestReapplyConflictIsAVerdictNotClean(t *testing.T) {
	r := newRebaseRepo(t) // B' edits b.txt line 5, which P edits too
	v, err := Reapply(context.Background(), r.repo, r.base, r.booted, r.p)
	if err != nil || v.Clean || !strings.Contains(v.Reason, "does not apply") {
		t.Fatalf("conflict = %+v, %v", v, err)
	}
}

func TestReapplyAlreadyInTheBaseIsNotClean(t *testing.T) {
	r := newRebaseRepo(t)
	// The platform shipped P itself: applying P again is not a clean change.
	scratch := t.TempDir()
	tgit(t, scratch, "clone", "-q", "--no-checkout", r.repo, ".")
	tgit(t, scratch, "checkout", "-q", r.base)
	editLine(t, scratch, "frontend/src/a.txt", 5, "a line 05 CHANGED BY P")
	editLine(t, scratch, "frontend/src/b.txt", 5, "b line 05 CHANGED BY P")
	tgit(t, scratch, "add", "-A")
	tgit(t, scratch, "commit", "-q", "-m", "upstreamed")
	booted := tgit(t, scratch, "rev-parse", "HEAD")
	tgit(t, r.repo, "fetch", "-q", scratch, "HEAD:refs/platform/up")
	v, err := Reapply(context.Background(), r.repo, r.base, booted, r.p)
	if err != nil || v.Clean {
		t.Fatalf("upstreamed = %+v, %v", v, err)
	}
}

func TestReapplyMissingBaseIsAnError(t *testing.T) {
	r := newRebaseRepo(t)
	if _, err := Reapply(context.Background(), r.repo, r.base, strings.Repeat("e", 40), r.p); err == nil {
		t.Fatal("a missing booted base must be an error, not a verdict")
	}
	if v, err := Reapply(context.Background(), r.repo, r.base, r.base, r.p); err != nil || v.Clean {
		t.Fatalf("same base = %+v, %v", v, err)
	}
}
