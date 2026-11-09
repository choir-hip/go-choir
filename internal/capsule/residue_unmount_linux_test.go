//go:build linux

package capsule

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Residue teardown failure modes (Devin review note 1, sandbox pause build):
//   - an upper holding a symlink at selfdev, selfdev/bundle, selfdev/carried,
//     nix/store or dev, pointing outside the capsule root, redirects an
//     unmount by name off the capsule;
//   - a mount outside the root with a root-like prefix (a sibling) is treated
//     as the capsule's;
//   - the capsule's own mounts are left mounted, or unmounted root-first;
//   - the root is not detached last.
// These tests drive the selection over a mountinfo fixture and a real symlink
// tree through the capsuleUnmount seam (the test user cannot mount); they do
// not prove a real kernel unmount, which the d@e drill covers.

func mountinfoLine(id int, mp string) string {
	return fmt.Sprintf("%d 1 0:%d / %s rw - overlay overlay rw\n", id, id, mp)
}

func TestSelectCapsuleMountsDeepestFirstAndUnderRootOnly(t *testing.T) {
	root := "/state/cap1/root"
	info := mountinfoLine(1, root) +
		mountinfoLine(2, root+"/run/current-system") +
		mountinfoLine(3, root+"/selfdev/bundle") +
		mountinfoLine(4, root+"/nix/store") +
		mountinfoLine(5, root+"/nix/store/sub") +
		mountinfoLine(6, "/state/cap1/rootx/selfdev") + // sibling with the same prefix
		mountinfoLine(7, "/nix/store") +
		mountinfoLine(8, "/state/cap2/root/selfdev/bundle")
	got := selectCapsuleMounts([]byte(info), root)
	// Same-depth entries keep a stable (later-first) order; check by depth.
	if len(got) != 4 || got[0] != root+"/nix/store/sub" {
		t.Fatalf("selection = %v, want 4 mounts, deepest first", got)
	}
	for i := 1; i < len(got); i++ {
		if strings.Count(got[i], "/") > strings.Count(got[i-1], "/") {
			t.Fatalf("not deepest first: %v", got)
		}
	}
	stacked := mountinfoLine(1, root+"/dev/null") + mountinfoLine(2, root+"/dev/null")
	if g := selectCapsuleMounts([]byte(stacked), root); len(g) != 2 {
		t.Fatalf("stacked mounts must each be selected: %v", g)
	}
}

func TestUnmountCapsuleRootSkipsSymlinkRedirectedMounts(t *testing.T) {
	base := t.TempDir()
	outside := filepath.Join(base, "outside")
	root := filepath.Join(base, "state", "root")
	for _, d := range []string{outside + "/bundle", outside + "/carried", outside + "/store", outside + "/dev",
		root + "/run/current-system", root + "/real"} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// The sandbox-writable upper plants symlinks at the capsule's mount names.
	for name, target := range map[string]string{
		"selfdev": outside, "nix": outside, "dev": filepath.Join(outside, "dev"),
	} {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	// Mountinfo spells these under root, but each is reached via a symlink.
	info := mountinfoLine(1, root+"/run/current-system") +
		mountinfoLine(2, root+"/selfdev/bundle") +
		mountinfoLine(3, root+"/selfdev/carried") +
		mountinfoLine(4, root+"/nix/store") +
		mountinfoLine(5, root+"/dev/null") +
		mountinfoLine(6, root+"/real") +
		mountinfoLine(7, outside+"/store")
	oldRead, oldUnmount := capsuleMountinfoRead, capsuleUnmount
	t.Cleanup(func() { capsuleMountinfoRead, capsuleUnmount = oldRead, oldUnmount })
	capsuleMountinfoRead = func() ([]byte, error) { return []byte(info), nil }
	var calls []string
	capsuleUnmount = func(path string, flags int) error { calls = append(calls, path); return nil }

	if err := unmountCapsuleRoot(root); err != nil {
		t.Fatal(err)
	}
	if len(calls) == 0 || calls[len(calls)-1] != root {
		t.Fatalf("root must be detached last: %v", calls)
	}
	own := append([]string(nil), calls[:len(calls)-1]...)
	sort.Strings(own)
	if want := []string{root + "/real", root + "/run/current-system"}; !reflect.DeepEqual(own, want) {
		t.Fatalf("unmounted %v, want only the capsule's own real mounts %v", own, want)
	}
}

func TestPathUnderRootIsSymlinkFree(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	if err := os.MkdirAll(filepath.Join(root, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(base, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if !pathUnderRootIsSymlinkFree(root, filepath.Join(root, "a", "b")) {
		t.Fatal("plain path refused")
	}
	if pathUnderRootIsSymlinkFree(root, filepath.Join(root, "link", "root", "a")) {
		t.Fatal("path through a symlink accepted")
	}
	if pathUnderRootIsSymlinkFree(root, root) || pathUnderRootIsSymlinkFree(root, base) {
		t.Fatal("root or outside accepted")
	}
}
