package capsule

// Failure modes under test (docs/problems/engineering-completion-path-cluster-2026-10-20.md,
// Failure 21). The capsule overlay's lowest layer is the guest's own root, so
// a computer that has applied a release shows /var/lib/artifact/release
// through the capsule, owned by an unmapped uid. The executor therefore
// creates an empty, capsule-owned, opaque release directory in the overlay
// upper before the mount (prepareCapsuleReleaseDir). Each way that can go
// wrong has a test:
//
//  1. Lower release files owned by another uid make the build's
//     `cp -rf --no-preserve=... dist/. /var/lib/artifact/release/frontend/`
//     fail with "setting permissions: Operation not permitted" and no
//     frontend build can freeze (the bug; reproduced against a real overlay
//     in capsule_release_dir_linux_test.go, which needs root).
//  2. The release directory is not owned by the capsule's mapped uid/gid, so
//     capsule root cannot write into it.
//  3. The release directory is not opaque (user.overlay.opaque=y, the
//     userxattr form MountOverlayFS mounts with), so the lower release files
//     still show through.
//  4. The pre-created directory shows up in the capsule Diff, either as
//     added directories (a capsule that built nothing would then freeze with
//     a non-empty OrderedFileEffects and a changed bundle digest) or, through
//     the opaque marker, as deletions of lower files (the release staging
//     loop refuses any deleted path under the release prefix).
//  5. An existing upper directory (a re-spawn, a retried prepare) breaks the
//     helper, or the helper destroys what is already in the release dir:
//     it must be idempotent and never remove content.
//  6. A symlink at any component of var/lib/artifact/release in the upper
//     directory is followed, so the chown and the opaque marker land on a
//     directory outside the capsule's upper layer: the helper must refuse.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

const opaqueXattr = "user.overlay.opaque"

var releaseDirChain = []string{"var", "var/lib", "var/lib/artifact", "var/lib/artifact/release"}

// releaseOwner is the uid/gid the test asks the helper to chown to: the
// capsule's mapped host id when running as root, otherwise the caller's own
// id (an unprivileged chown to oneself succeeds, so the call is still made).
func releaseOwner() (int, int) {
	if os.Geteuid() == 0 {
		return 65534, 65534 // capsuleNamespaceHostID (linux-only constant)
	}
	return os.Getuid(), os.Getgid()
}

func requireXattr(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := unix.Setxattr(dir, opaqueXattr, []byte("y"), 0); err != nil {
		t.Skipf("filesystem does not support %s: %v", opaqueXattr, err)
	}
}

func opaqueValue(t *testing.T, path string) string {
	t.Helper()
	names := make([]byte, 4096)
	n, err := unix.Listxattr(path, names)
	if err != nil {
		t.Fatalf("listxattr %s: %v", path, err)
	}
	present := false
	for _, name := range strings.Split(string(names[:n]), "\x00") {
		if name == opaqueXattr {
			present = true
		}
	}
	if !present {
		return ""
	}
	buf := make([]byte, 16)
	n, err = unix.Getxattr(path, opaqueXattr, buf)
	if err != nil {
		t.Fatalf("getxattr %s: %v", path, err)
	}
	return string(buf[:n])
}

func TestPrepareCapsuleReleaseDirCreatesOwnedOpaqueEmptyDir(t *testing.T) {
	requireXattr(t)
	upper := filepath.Join(t.TempDir(), "upper")
	uid, gid := releaseOwner()
	if err := prepareCapsuleReleaseDir(upper, uid, gid); err != nil {
		t.Fatal(err)
	}
	release := filepath.Join(upper, "var", "lib", "artifact", "release")
	info, err := os.Lstat(release)
	if err != nil || !info.IsDir() {
		t.Fatalf("release dir: info=%v err=%v", info, err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("release dir mode = %v, want 0755", info.Mode().Perm())
	}
	stat := info.Sys().(*syscall.Stat_t)
	if int(stat.Uid) != uid || int(stat.Gid) != gid {
		t.Fatalf("release dir owner = %d:%d, want %d:%d (failure mode 2)", stat.Uid, stat.Gid, uid, gid)
	}
	if got := opaqueValue(t, release); got != "y" {
		t.Fatalf("release dir %s = %q, want y (failure mode 3)", opaqueXattr, got)
	}
	entries, err := os.ReadDir(release)
	if err != nil || len(entries) != 0 {
		t.Fatalf("release dir not empty: entries=%v err=%v", entries, err)
	}
	// Only the leaf is capsule-owned and opaque: the parents stay plain so
	// the capsule gets no write access to the rest of /var/lib/artifact.
	for _, parent := range releaseDirChain[:3] {
		path := filepath.Join(upper, parent)
		if got := opaqueValue(t, path); got != "" {
			t.Fatalf("parent %s is opaque (%q): it would hide the guest's /var/lib/artifact siblings", parent, got)
		}
	}
}

func TestPrepareCapsuleReleaseDirIsIdempotentAndKeepsContent(t *testing.T) {
	requireXattr(t)
	upper := t.TempDir()
	uid, gid := releaseOwner()
	if err := prepareCapsuleReleaseDir(upper, uid, gid); err != nil {
		t.Fatal(err)
	}
	built := filepath.Join(upper, "var", "lib", "artifact", "release", "frontend", "index.html")
	if err := os.MkdirAll(filepath.Dir(built), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(built, []byte("built"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := prepareCapsuleReleaseDir(upper, uid, gid); err != nil {
		t.Fatalf("second prepare on an existing upper (failure mode 5): %v", err)
	}
	if data, err := os.ReadFile(built); err != nil || string(data) != "built" {
		t.Fatalf("second prepare disturbed release content: data=%q err=%v", data, err)
	}
	if got := opaqueValue(t, filepath.Dir(filepath.Dir(built))); got != "y" {
		t.Fatalf("opaque marker lost on second prepare: %q", got)
	}
}

func TestPrepareCapsuleReleaseDirAcceptsPartiallyExistingUpper(t *testing.T) {
	requireXattr(t)
	upper := t.TempDir()
	if err := os.MkdirAll(filepath.Join(upper, "var", "lib", "other"), 0o755); err != nil {
		t.Fatal(err)
	}
	uid, gid := releaseOwner()
	if err := prepareCapsuleReleaseDir(upper, uid, gid); err != nil {
		t.Fatalf("prepare with existing parents (failure mode 5): %v", err)
	}
	if _, err := os.Stat(filepath.Join(upper, "var", "lib", "other")); err != nil {
		t.Fatalf("existing sibling disturbed: %v", err)
	}
}

func TestPrepareCapsuleReleaseDirRefusesSymlinkComponents(t *testing.T) {
	requireXattr(t)
	for _, component := range releaseDirChain {
		t.Run(component, func(t *testing.T) {
			upper := t.TempDir()
			outside := t.TempDir()
			if err := os.MkdirAll(filepath.Join(outside, "var", "lib", "artifact", "release"), 0o755); err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(upper, filepath.FromSlash(component))
			if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, link); err != nil {
				t.Fatal(err)
			}
			uid, gid := releaseOwner()
			if err := prepareCapsuleReleaseDir(upper, uid, gid); err == nil {
				t.Fatalf("symlink at %s was followed (failure mode 6)", component)
			}
			// Nothing outside the upper layer may have been marked or created.
			for _, probe := range []string{outside, filepath.Join(outside, "var", "lib", "artifact", "release")} {
				if got := opaqueValue(t, probe); got != "" {
					t.Fatalf("symlink target %s was marked opaque", probe)
				}
			}
			if entries, _ := os.ReadDir(filepath.Join(outside, "var", "lib", "artifact", "release")); len(entries) != 0 {
				t.Fatalf("symlink target gained entries: %v", entries)
			}
		})
	}
}

func TestPrepareCapsuleReleaseDirRefusesNonDirectoryComponent(t *testing.T) {
	upper := t.TempDir()
	if err := os.MkdirAll(filepath.Join(upper, "var", "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(upper, "var", "lib", "artifact"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	uid, gid := releaseOwner()
	if err := prepareCapsuleReleaseDir(upper, uid, gid); err == nil {
		t.Fatal("a regular file at var/lib/artifact was accepted")
	}
}

// diffUpper is the capsule Diff of an upper directory against an empty last
// manifest, without mounting anything.
func diffUpper(t *testing.T, upper string) []FileChange {
	t.Helper()
	current, err := walkUpperdir(context.Background(), upper)
	if err != nil {
		t.Fatal(err)
	}
	return diffManifests(nil, current)
}

func changePaths(changes []FileChange) []string {
	var out []string
	for _, change := range changes {
		out = append(out, change.Path)
	}
	return out
}

func TestPristineReleaseDirIsNotACapsuleChange(t *testing.T) {
	requireXattr(t)
	upper := t.TempDir()
	uid, gid := releaseOwner()
	if err := prepareCapsuleReleaseDir(upper, uid, gid); err != nil {
		t.Fatal(err)
	}
	if changes := diffUpper(t, upper); len(changes) != 0 {
		t.Fatalf("a capsule that built nothing diffs as %v (failure mode 4)", changePaths(changes))
	}
}

func TestBuiltReleaseFilesDiffAsBeforeThePrecreatedDir(t *testing.T) {
	requireXattr(t)
	write := func(upper string) {
		path := filepath.Join(upper, "var", "lib", "artifact", "release", "frontend", "index.html")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("built"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	prepared := t.TempDir()
	uid, gid := releaseOwner()
	if err := prepareCapsuleReleaseDir(prepared, uid, gid); err != nil {
		t.Fatal(err)
	}
	write(prepared)
	plain := t.TempDir()
	write(plain)
	got, want := changePaths(diffUpper(t, prepared)), changePaths(diffUpper(t, plain))
	if len(got) != len(want) {
		t.Fatalf("prepared upper diffs as %v, plain upper as %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("prepared upper diffs as %v, plain upper as %v", got, want)
		}
	}
	for _, change := range diffUpper(t, prepared) {
		if change.Kind == ChangeDeleted {
			t.Fatalf("opaque release dir produced a deletion: %v (failure mode 4)", change.Path)
		}
	}
}

func TestPristineReleaseDirKeepsOtherChangesUnderItsParents(t *testing.T) {
	requireXattr(t)
	upper := t.TempDir()
	uid, gid := releaseOwner()
	if err := prepareCapsuleReleaseDir(upper, uid, gid); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(upper, "var", "lib", "other", "note.txt")
	if err := os.MkdirAll(filepath.Dir(other), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, []byte("n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := changePaths(diffUpper(t, upper))
	want := []string{"var", "var/lib", "var/lib/other", "var/lib/other/note.txt"}
	if len(got) != len(want) {
		t.Fatalf("diff = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("diff = %v, want %v", got, want)
		}
	}
}
