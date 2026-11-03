package agentcore

import (
	"os"
	"path/filepath"
	"testing"
)

// Problem 17: a layered release runs the daemon from the updater's private
// store under a root-only 0700 directory, so a cell uid could not exec the
// desk worker beside it (fork/exec .../choir-updater/store/<name>/bin/desk-worker:
// permission denied). The same file is at /nix/store/<name>/... through the
// guest's store overlay, a path the cell uid can traverse.
func TestDeskWorkerBinaryForLayeredReleaseUsesStoreOverlayPath(t *testing.T) {
	updaterRoot := filepath.Join(t.TempDir(), "choir-updater")
	nixStore := filepath.Join(t.TempDir(), "nix", "store")
	t.Setenv("CHOIR_UPDATER_ROOT", updaterRoot)
	prev := deskWorkerNixStoreRoot
	deskWorkerNixStoreRoot = nixStore
	t.Cleanup(func() { deskWorkerNixStoreRoot = prev })

	const name = "a967jdk2bcvxlmyjf0lwnbaybixpb656-autoputer-0.1.0"
	writeExec := func(path string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	exe := filepath.Join(updaterRoot, "store", name, "bin", "autoputer")
	writeExec(exe)
	writeExec(filepath.Join(updaterRoot, "store", name, "bin", "desk-worker"))

	// Failure mode 2: the overlay is not mounted, so the alias does not
	// exist. Keep the private-store path (the loop bound closes the trigger).
	if got, want := deskWorkerBinaryFor(exe), filepath.Join(updaterRoot, "store", name, "bin", "desk-worker"); got != want {
		t.Fatalf("no overlay: got %q, want %q", got, want)
	}

	// Failure mode 1: the overlay presents the release at /nix/store/<name>.
	writeExec(filepath.Join(nixStore, name, "bin", "desk-worker"))
	if got, want := deskWorkerBinaryFor(exe), filepath.Join(nixStore, name, "bin", "desk-worker"); got != want {
		t.Fatalf("overlay: got %q, want %q", got, want)
	}

	// No dedicated worker: the daemon itself serves desk-session, also
	// through the overlay path.
	if err := os.Remove(filepath.Join(nixStore, name, "bin", "desk-worker")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(updaterRoot, "store", name, "bin", "desk-worker")); err != nil {
		t.Fatal(err)
	}
	writeExec(filepath.Join(nixStore, name, "bin", "autoputer"))
	if got, want := deskWorkerBinaryFor(exe), filepath.Join(nixStore, name, "bin", "autoputer"); got != want {
		t.Fatalf("overlay self: got %q, want %q", got, want)
	}

	// Failure mode 3: a daemon outside the updater store is unchanged.
	base := filepath.Join(t.TempDir(), "bin", "autoputer")
	writeExec(base)
	if got := deskWorkerBinaryFor(base); got != base {
		t.Fatalf("base daemon: got %q, want %q", got, base)
	}

	// Failure mode 4: names that are not one clean store entry never map.
	for _, bad := range []string{
		filepath.Join(updaterRoot, "store", ".staging-1", "bin", "autoputer"),
		filepath.Join(updaterRoot, "store", "autoputer"),
		filepath.Join(updaterRoot, "storex", name, "bin", "autoputer"),
	} {
		if alias := layeredStoreAlias(bad); alias != "" {
			t.Fatalf("layeredStoreAlias(%q) = %q, want none", bad, alias)
		}
	}
}
