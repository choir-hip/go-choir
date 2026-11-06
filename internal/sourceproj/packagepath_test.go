package sourceproj

import (
	"context"
	"strings"
	"testing"
)

// Forced modularity M2, door 3 (docs/design/forced-modularity-build-2026-11-04.md
// section 2). Failure modes: a stack holding a package path is projected (app
// source rides the stack and the served build); the refusal resets HEAD and
// loses the computer's source; a stack whose package change main already
// carries is refused instead of retired; a clean rebase carries a package path.
func TestStackWithAPackagePathIsRefusedAndNothingMoves(t *testing.T) {
	repo, base := baseRepo(t)
	before := headOf(t, repo)
	patch := patchFrom(t, repo, base, func(root string) {
		write(t, root, "frontend/src/packages/image/App.svelte", "<p>x</p>\n")
		editLine(t, root, "frontend/src/a.txt", 3, "changed")
	})
	res := reconcile(t, Input{Repo: repo, BootedBase: base, Top: opFor("op-1", base, patch)})
	if res.State != StatePackagePath || !strings.HasPrefix(res.Refusal(), "stack_package_path: package_path_refused") {
		t.Fatalf("state %s refusal %q", res.State, res.Refusal())
	}
	if len(res.PackagePaths) != 1 || res.PackagePaths[0] != "frontend/src/packages/image/App.svelte" {
		t.Fatalf("package paths %v", res.PackagePaths)
	}
	if headOf(t, repo) != before || res.Changed {
		t.Fatal("a refused stack moved HEAD")
	}
}

func TestPackageChangeMainCarriesIsRetiredNotRefused(t *testing.T) {
	repo, base := baseRepo(t)
	patch := patchFrom(t, repo, base, func(root string) {
		write(t, root, "frontend/src/packages/image/App.svelte", "<p>x</p>\n")
	})
	booted := platformMoved(t, repo, base, func(root string) {
		write(t, root, "frontend/src/packages/image/App.svelte", "<p>x</p>\n")
	})
	res := reconcile(t, Input{Repo: repo, BootedBase: booted, Top: opFor("op-1", base, patch)})
	if res.State != StateUpstreamed {
		t.Fatalf("state %s: %s", res.State, res.Reason)
	}
}

func TestReapplyRefusesAPackagePath(t *testing.T) {
	repo, base := baseRepo(t)
	patch := patchFrom(t, repo, base, func(root string) {
		write(t, root, "frontend/src/packages/image/App.svelte", "<p>x</p>\n")
	})
	booted := platformMoved(t, repo, base, func(root string) {
		editLine(t, root, "internal/x.go", 3, "func X() int { return 2 }")
	})
	verdict, err := Reapply(context.Background(), repo, base, booted, patch)
	if err != nil || verdict.Clean || !strings.HasPrefix(verdict.Reason, ErrPackagePath.Error()) {
		t.Fatalf("verdict %+v err %v", verdict, err)
	}
}
