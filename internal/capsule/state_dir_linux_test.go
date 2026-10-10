//go:build linux

package capsule

import (
	"os"
	"path/filepath"
	"testing"
)

// Ways this can fail: an earlier boot's tree survives (read-only overlay
// work dirs block removal), the current boot's live capsules are removed,
// unrelated entries in the root are removed, or a mount escape in the
// mount table hides a live mount.
func TestBootScopedStateDirRemovesOnlyEarlierBoots(t *testing.T) {
	root := t.TempDir()
	stale := filepath.Join(root, "boot-old", "capsule-a", "work", "work")
	if err := os.MkdirAll(stale, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stale, "f"), []byte("x"), 0o400); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(stale, 0o000); err != nil {
		t.Fatal(err)
	}
	live := filepath.Join(root, "boot-now", "capsule-b", "upper")
	if err := os.MkdirAll(live, 0o700); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(root, "keep-me")
	if err := os.MkdirAll(other, 0o700); err != nil {
		t.Fatal(err)
	}

	got, err := bootScopedStateDir(root, "now")
	if err != nil {
		t.Fatalf("bootScopedStateDir: %v", err)
	}
	if got != filepath.Join(root, "boot-now") {
		t.Fatalf("dir = %s", got)
	}
	if _, err := os.Stat(filepath.Join(root, "boot-old")); !os.IsNotExist(err) {
		t.Fatalf("earlier boot survived: %v", err)
	}
	for _, path := range []string{live, other} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s removed: %v", path, err)
		}
	}
	if _, err := bootScopedStateDir(root, "../x"); err == nil {
		t.Fatal("a boot id with a path separator was accepted")
	}
}

func TestMountTableEscapesAndPrefixes(t *testing.T) {
	if got := unescapeMountPath(`/a\040b/c\134d`); got != `/a b/c\d` {
		t.Fatalf("unescape = %q", got)
	}
	mounts := []string{"/s/boot-old/capsule-a/root"}
	if !hasMountUnder(mounts, "/s/boot-old") {
		t.Fatal("nested mount not seen")
	}
	if hasMountUnder(mounts, "/s/boot-ol") {
		t.Fatal("prefix without a separator matched")
	}
}
