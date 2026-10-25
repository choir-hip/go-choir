//go:build linux

package capsule

// Linux half of manifest_whiteout_test.go / overlay_deletions_test.go: the
// same failure modes against what the kernel actually writes.
//
//   - TestWalkUpperdirRecordsMknodWhiteout needs permission to mknod a 0:0
//     character device (root, or CAP_MKNOD); it skips otherwise.
//   - TestCapsuleDeletes* mount a real overlay with the capsule's own mount
//     options (MountOverlayFS: userxattr) and need root; they skip when the
//     overlay or user.overlay.opaque is unavailable. The layout is the
//     capsule's: <base>/source-lower/workspace/platform is the pinned
//     source, <base>/merged the merged root, <base>/upper the upper.
//
// The test the incident needs is TestCapsuleDeletesLowerFileFreezes: a lower
// file removed through the merged view must reach the source patch as a
// deletion and the freeze's patch step must succeed.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestWalkUpperdirRecordsMknodWhiteout(t *testing.T) {
	upper := t.TempDir()
	if err := os.MkdirAll(filepath.Join(upper, "workspace", "platform"), 0o755); err != nil {
		t.Fatal(err)
	}
	whiteout := filepath.Join(upper, "workspace", "platform", "gone.go")
	if err := unix.Mknod(whiteout, unix.S_IFCHR|0o000, 0); err != nil {
		t.Skipf("cannot mknod a whiteout here: %v", err)
	}
	// A real character device that is not 0:0 is not a whiteout (/dev/null is 1:3).
	null := filepath.Join(upper, "workspace", "platform", "null")
	if err := unix.Mknod(null, unix.S_IFCHR|0o000, int(unix.Mkdev(1, 3))); err != nil {
		t.Skipf("cannot mknod a device here: %v", err)
	}
	manifests, err := walkUpperdir(context.Background(), upper)
	if err != nil {
		t.Fatal(err)
	}
	types := map[string]string{}
	for _, m := range manifests {
		types[m.Path] = m.Type
	}
	if got := types["workspace/platform/gone.go"]; got != "whiteout" {
		t.Fatalf("0:0 char device recorded as %q, want whiteout", got)
	}
	if got := types["workspace/platform/null"]; got != "device" {
		t.Fatalf("1:3 char device recorded as %q, want device", got)
	}
	kinds := kindsByPath(diffManifests(nil, manifests))
	if kinds["workspace/platform/gone.go"] != ChangeDeleted {
		t.Fatalf("whiteout diffs as %v, want deleted", kinds["workspace/platform/gone.go"])
	}
}

func TestWalkUpperdirRecordsOpaqueDirectories(t *testing.T) {
	upper := t.TempDir()
	opaque := filepath.Join(upper, "workspace", "platform", "d")
	plain := filepath.Join(upper, "workspace", "platform", "p")
	for _, d := range []string{opaque, plain} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := unix.Setxattr(opaque, overlayOpaqueXattr, []byte("y"), 0); err != nil {
		t.Skipf("filesystem does not support %s: %v", overlayOpaqueXattr, err)
	}
	manifests, err := walkUpperdir(context.Background(), upper)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, m := range manifests {
		got[m.Path] = m.Opaque
	}
	if !got["workspace/platform/d"] || got["workspace/platform/p"] || got["workspace/platform"] {
		t.Fatalf("opaque marks = %v, want only workspace/platform/d", got)
	}
}

type realOverlay struct {
	base, upper, merged, platform string
	caps                          *Capsule
}

// newRealOverlay mounts an overlay over a pinned-source lower layer holding
// files (path under workspace/platform -> body).
func newRealOverlay(t *testing.T, files map[string]string, beforeMount ...func(upper string) error) *realOverlay {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("real overlay deletion tests need root")
	}
	base, err := os.MkdirTemp("", "capsule-deletes-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(base) })
	if err := os.Chmod(base, 0o755); err != nil {
		t.Fatal(err)
	}
	lower := filepath.Join(base, "source-lower")
	if err := os.MkdirAll(filepath.Join(lower, "workspace", "platform"), 0o755); err != nil {
		t.Fatal(err)
	}
	for rel, body := range files {
		putFile(t, filepath.Join(lower, "workspace", "platform"), rel, body, 0o644)
	}
	o := &realOverlay{
		base:     base,
		upper:    filepath.Join(base, "upper"),
		merged:   filepath.Join(base, "merged"),
		platform: filepath.Join(base, "merged", "workspace", "platform"),
	}
	for _, prepare := range beforeMount {
		if err := prepare(o.upper); err != nil {
			t.Skipf("upper preparation unavailable: %v", err)
		}
	}
	if err := MountOverlayFS(o.merged, o.upper, filepath.Join(base, "work"), lower); err != nil {
		t.Skipf("overlay mount unavailable: %v", err)
	}
	t.Cleanup(func() { _ = UnmountOverlayFS(o.merged) })
	o.caps = &Capsule{ID: "deletes-test", State: StateActive, MergedDir: o.merged, UpperDir: o.upper}
	return o
}

func (o *realOverlay) patchOf(t *testing.T) (string, []FileChange) {
	t.Helper()
	changes, err := o.caps.Diff(context.Background())
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	patchPath, _, _, err := (&Executor{}).emitSourcePatch(o.caps, changes, t.TempDir())
	if err != nil {
		t.Fatalf("emitSourcePatch (the freeze's patch step): %v", err)
	}
	if patchPath == "" {
		return "", changes
	}
	body, err := os.ReadFile(patchPath)
	if err != nil {
		t.Fatal(err)
	}
	return string(body), changes
}

func TestCapsuleDeletesLowerFileFreezes(t *testing.T) {
	o := newRealOverlay(t, map[string]string{
		"frontend/src/lib/EpubApp.svelte": "<script></script>\n",
		"frontend/src/lib/Other.svelte":   "other\n",
	})
	if err := os.Remove(filepath.Join(o.platform, "frontend/src/lib/EpubApp.svelte")); err != nil {
		t.Fatal(err)
	}
	patch, changes := o.patchOf(t)
	for _, c := range changes {
		if c.Path == "workspace/platform/frontend/src/lib/EpubApp.svelte" && c.Kind != ChangeDeleted {
			t.Fatalf("deleted lower file diffs as %v", c.Kind)
		}
	}
	for _, want := range []string{"deleted file mode 100644", "--- a/frontend/src/lib/EpubApp.svelte", "+++ /dev/null"} {
		if !strings.Contains(patch, want) {
			t.Fatalf("patch missing %q:\n%s", want, patch)
		}
	}
	if strings.Contains(patch, "Other.svelte") {
		t.Fatalf("patch touches an untouched file:\n%s", patch)
	}
}

func TestCapsuleDeletesLowerFileThenRecreatesIt(t *testing.T) {
	o := newRealOverlay(t, map[string]string{"a.go": "old\n"})
	p := filepath.Join(o.platform, "a.go")
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("recreated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	patch, _ := o.patchOf(t)
	if strings.Contains(patch, "deleted file") || !strings.Contains(patch, "-old") || !strings.Contains(patch, "+recreated") {
		t.Fatalf("delete-then-recreate is not a modification:\n%s", patch)
	}
}

func TestCapsuleCreatesThenDeletesUpperOnlyFile(t *testing.T) {
	o := newRealOverlay(t, map[string]string{"a.go": "a\n"})
	p := filepath.Join(o.platform, "scratch.go")
	if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	patch, changes := o.patchOf(t)
	if patch != "" {
		t.Fatalf("create-then-delete produced a patch:\n%s", patch)
	}
	for _, c := range changes {
		if strings.HasSuffix(c.Path, "scratch.go") {
			t.Fatalf("create-then-delete left a change: %+v", c)
		}
	}
}

func TestCapsuleDeletesLowerDirectoryTree(t *testing.T) {
	o := newRealOverlay(t, map[string]string{"gone/a.go": "a\n", "gone/deep/b.go": "b\n", "keep.go": "k\n"})
	if err := os.RemoveAll(filepath.Join(o.platform, "gone")); err != nil {
		t.Fatal(err)
	}
	patch, _ := o.patchOf(t)
	for _, want := range []string{"--- a/gone/a.go", "--- a/gone/deep/b.go"} {
		if !strings.Contains(patch, want) {
			t.Fatalf("patch missing %q:\n%s", want, patch)
		}
	}
}

func TestCapsuleRemovesAndRecreatesLowerDirectoryOpaque(t *testing.T) {
	o := newRealOverlay(t, map[string]string{"d/old.go": "old\n", "d/kept.go": "kept\n", "d/sub/x.go": "x\n"})
	d := filepath.Join(o.platform, "d")
	if err := os.RemoveAll(d); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "kept.go"), []byte("kept, rewritten\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf [1]byte
	if n, err := unix.Lgetxattr(filepath.Join(o.upper, "workspace", "platform", "d"), overlayOpaqueXattr, buf[:]); err != nil || n != 1 {
		t.Skipf("kernel did not mark the recreated directory opaque (n=%d err=%v)", n, err)
	}
	patch, _ := o.patchOf(t)
	for _, want := range []string{"--- a/d/old.go", "--- a/d/sub/x.go", "-kept", "+kept, rewritten"} {
		if !strings.Contains(patch, want) {
			t.Fatalf("patch missing %q:\n%s", want, patch)
		}
	}
}

func TestCapsuleReleaseDirStaysOutOfDeletions(t *testing.T) {
	// The spawn's own opaque release dir must never become deletions.
	o := newRealOverlay(t, nil, func(upper string) error {
		return prepareCapsuleReleaseDir(upper, capsuleNamespaceHostID, capsuleNamespaceHostID)
	})
	changes, err := o.caps.Diff(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range changes {
		if c.Kind == ChangeDeleted {
			t.Fatalf("release dir produced a deletion: %+v", c)
		}
	}
}

// Failure mode 11: an empty regular file carrying user.overlay.whiteout is
// a whiteout; a non-empty one, or one without the xattr, is a file.
func TestWalkUpperdirRecordsXattrWhiteout(t *testing.T) {
	upper := t.TempDir()
	platform := filepath.Join(upper, "workspace", "platform")
	putFile(t, platform, "gone.go", "", 0o644)
	putFile(t, platform, "full.go", "body\n", 0o644)
	putFile(t, platform, "empty.go", "", 0o644)
	for _, name := range []string{"gone.go", "full.go"} {
		if err := unix.Setxattr(filepath.Join(platform, name), overlayWhiteoutXattr, nil, 0); err != nil {
			t.Skipf("filesystem does not support %s: %v", overlayWhiteoutXattr, err)
		}
	}
	manifests, err := walkUpperdir(context.Background(), upper)
	if err != nil {
		t.Fatal(err)
	}
	types := map[string]string{}
	for _, m := range manifests {
		types[m.Path] = m.Type
	}
	if got := types["workspace/platform/gone.go"]; got != "whiteout" {
		t.Fatalf("empty file with %s recorded as %q, want whiteout", overlayWhiteoutXattr, got)
	}
	for _, name := range []string{"full.go", "empty.go"} {
		if got := types["workspace/platform/"+name]; got != "file" {
			t.Fatalf("%s recorded as %q, want file", name, got)
		}
	}
	kinds := kindsByPath(diffManifests(nil, manifests))
	if kinds["workspace/platform/gone.go"] != ChangeDeleted {
		t.Fatalf("xattr whiteout diffs as %v, want deleted", kinds["workspace/platform/gone.go"])
	}
}

// Failure mode 9 against the kernel: removing workspace itself.
func TestCapsuleDeletesWorkspaceAncestor(t *testing.T) {
	o := newRealOverlay(t, map[string]string{"a.go": "a\n", "d/b.go": "b\n"})
	if err := os.RemoveAll(filepath.Join(o.merged, "workspace")); err != nil {
		t.Fatal(err)
	}
	patch, _ := o.patchOf(t)
	for _, want := range []string{"--- a/a.go\n+++ /dev/null", "--- a/d/b.go\n+++ /dev/null"} {
		if !strings.Contains(patch, want) {
			t.Fatalf("patch missing %q:\n%s", want, patch)
		}
	}
}

func TestCapsuleRemovesAndRecreatesWorkspaceAncestor(t *testing.T) {
	o := newRealOverlay(t, map[string]string{"kept.go": "kept\n", "old.go": "old\n"})
	if err := os.RemoveAll(filepath.Join(o.merged, "workspace")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(o.platform, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(o.platform, "kept.go"), []byte("kept, rewritten\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf [1]byte
	if n, err := unix.Lgetxattr(filepath.Join(o.upper, "workspace"), overlayOpaqueXattr, buf[:]); err != nil || n != 1 {
		t.Skipf("kernel did not mark the recreated workspace opaque (n=%d err=%v)", n, err)
	}
	patch, _ := o.patchOf(t)
	for _, want := range []string{"--- a/old.go\n+++ /dev/null", "-kept", "+kept, rewritten"} {
		if !strings.Contains(patch, want) {
			t.Fatalf("patch missing %q:\n%s", want, patch)
		}
	}
}
