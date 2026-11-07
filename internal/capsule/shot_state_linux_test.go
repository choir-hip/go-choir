//go:build linux

package capsule

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// The candidate shot's state helpers (shot_state_linux.go, design 3.3).
// Failure modes, decided before the code:
//
//   - a shot copy pushes the state volume under its floor: the same charge a
//     capsule copy pays refuses it;
//   - the files layer shows the owner's mail or the capsule's own source, or
//     a file the owner kept unreadable: it leaves out what the capsule's
//     layer leaves out;
//   - the layer is not the owner's files but a path into them (FM4): a
//     hardlink the sandbox cannot write, directories it can, and a layer
//     inside the tree it copies is refused.

func TestCheckStateCopyFitsChargesTheCapsuleCopyAndKeepsTheFloor(t *testing.T) {
	old := computerStoreFreeBytes
	t.Cleanup(func() { computerStoreFreeBytes = old })
	const estimate = int64(100 << 20)
	need := int64(computerStoreCopyFactor*float64(estimate)) + computerStoreAllowance
	computerStoreFreeBytes = func(string) (int64, error) { return StateFloorBytes + need - 1, nil }
	if err := CheckStateCopyFits(t.TempDir(), estimate); err == nil || !strings.Contains(err.Error(), "does not fit") {
		t.Fatalf("one byte under the floor = %v, want refusal", err)
	}
	computerStoreFreeBytes = func(string) (int64, error) { return StateFloorBytes + need, nil }
	if err := CheckStateCopyFits(t.TempDir(), estimate); err != nil {
		t.Fatalf("exactly the floor = %v, want fits", err)
	}
	computerStoreFreeBytes = func(string) (int64, error) { return 0, errors.New("statfs failed") }
	if err := CheckStateCopyFits(t.TempDir(), estimate); err == nil {
		t.Fatal("an unreadable volume was treated as free")
	}
}

func TestStateVolumeAboveFloorRefusesAVolumeUnderIt(t *testing.T) {
	old := computerStoreFreeBytes
	t.Cleanup(func() { computerStoreFreeBytes = old })
	computerStoreFreeBytes = func(string) (int64, error) { return StateFloorBytes - 1, nil }
	if err := StateVolumeAboveFloor(t.TempDir()); err == nil {
		t.Fatal("a volume under the floor passed")
	}
	computerStoreFreeBytes = func(string) (int64, error) { return StateFloorBytes, nil }
	if err := StateVolumeAboveFloor(t.TempDir()); err != nil {
		t.Fatal(err)
	}
}

func TestShotSandboxIDsAreTheCapsulesUnprivilegedIDOnlyForRoot(t *testing.T) {
	uid, gid := ShotSandboxIDs()
	if os.Geteuid() == 0 {
		if uid != capsuleNamespaceHostID || gid != capsuleNamespaceHostID {
			t.Fatalf("root's sandbox ids = %d:%d, want the capsule's %d", uid, gid, capsuleNamespaceHostID)
		}
		return
	}
	if uid != os.Geteuid() || gid != os.Getegid() {
		t.Fatalf("unprivileged ids = %d:%d, want the caller's", uid, gid)
	}
}

func TestBuildShotFilesLayerLinksWhatTheCapsuleLayerLinksAndNothingElse(t *testing.T) {
	root := filepath.Join(t.TempDir(), "files")
	writeLayerTree(t, root, map[string]os.FileMode{
		"notes.txt": 0o644, "docs/a.md": 0o644, "mail/new/1": 0o644, "Source/platform/main.go": 0o644, "secret.key": 0o600,
	})
	dest := filepath.Join(t.TempDir(), "layer")
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	uid, gid := ShotSandboxIDs()
	receipt, err := BuildShotFilesLayer(context.Background(), root, filepath.Join(root, "Source", "platform"), dest, uid, gid)
	if err != nil || !receipt.Files {
		t.Fatalf("receipt = %+v, %v", receipt, err)
	}
	for _, kept := range []string{"notes.txt", "docs/a.md"} {
		live, err := os.Stat(filepath.Join(root, kept))
		if err != nil {
			t.Fatal(err)
		}
		linked, err := os.Stat(filepath.Join(dest, kept))
		if err != nil {
			t.Fatalf("%s is not in the layer: %v", kept, err)
		}
		if !os.SameFile(live, linked) {
			t.Fatalf("%s is a copy, not a link to the live file", kept)
		}
	}
	for _, left := range []string{"mail", "Source/platform", "secret.key"} {
		if _, err := os.Lstat(filepath.Join(dest, left)); err == nil {
			t.Fatalf("%s reached the shot layer", left)
		}
	}
	// A file made in the layer is the layer's alone, and the live file keeps
	// its bytes, mode and link count semantics.
	if err := os.WriteFile(filepath.Join(dest, "docs", "new.md"), []byte("shot"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "new.md")); err == nil {
		t.Fatal("a file made in the layer appeared in the live tree")
	}
	if st, err := os.Stat(filepath.Join(root, "notes.txt")); err != nil || st.Mode().Perm() != 0o644 || st.Sys().(*syscall.Stat_t).Nlink != 2 {
		t.Fatalf("live notes.txt = %v %v", st, err)
	}
}

func TestBuildShotFilesLayerRefusesALayerInsideTheTreeItCopies(t *testing.T) {
	root := filepath.Join(t.TempDir(), "files")
	writeLayerTree(t, root, map[string]os.FileMode{"a.txt": 0o644})
	inside := filepath.Join(root, "layer")
	if err := os.Mkdir(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildShotFilesLayer(context.Background(), root, "", inside, os.Geteuid(), os.Getegid()); err == nil || !strings.Contains(err.Error(), "overlap") {
		t.Fatalf("a layer inside the files root = %v, want refusal", err)
	}
	if _, err := BuildShotFilesLayer(context.Background(), "", "", t.TempDir(), os.Geteuid(), os.Getegid()); err == nil {
		t.Fatal("no files root was accepted")
	}
}

// N2: the shot only reads the owner's files, so a file that anyone but its
// owner could write is not linked into the layer (a linked inode is the live
// one); the same walk for a capsule layer still links it.
func TestBuildShotFilesLayerLeavesOutFilesOthersCanWrite(t *testing.T) {
	root := filepath.Join(t.TempDir(), "files")
	writeLayerTree(t, root, map[string]os.FileMode{"ok.txt": 0o644, "world.txt": 0o666, "group.txt": 0o664})
	dest := filepath.Join(t.TempDir(), "layer")
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	uid, gid := ShotSandboxIDs()
	receipt, err := BuildShotFilesLayer(context.Background(), root, "", dest, uid, gid)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "ok.txt")); err != nil {
		t.Fatalf("the read-only file was not linked: %v", err)
	}
	for _, left := range []string{"world.txt", "group.txt"} {
		if _, err := os.Lstat(filepath.Join(dest, left)); err == nil {
			t.Fatalf("%s is writable by others and reached the shot layer", left)
		}
	}
	if receipt.LeftOutCounts[LeaveOutWritable] != 2 {
		t.Fatalf("left out = %v", receipt.LeftOutCounts)
	}
	capsuleDest := t.TempDir()
	if err := snapshotFilesTree(context.Background(), root, capsuleDest, nil, &ComputerStateReceipt{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(capsuleDest, "world.txt")); err != nil {
		t.Fatalf("the capsule layer must keep linking it: %v", err)
	}
}
