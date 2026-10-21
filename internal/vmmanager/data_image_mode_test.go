package vmmanager

import (
	"os"
	"path/filepath"
	"testing"
)

// F12 (docs/problems/data-at-rest-snapshots-images-escrow-key-plaintext-
// 2026-10-21.md): a data image was created 0644 and never re-moded. The
// parent directory is the barrier today; the file itself must not depend on it.
//
// Failure modes pinned here, written before the code (goal file FP9, "Mode
// change"): a new image is 0644; an existing 0644 image stays 0644 after a
// boot; a quarantined or staged image keeps a loose mode; the tightening
// follows a symlink and changes another file's mode; the tightening changes
// the disk identity a snapshot is bound to.

func modeOf(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat %s: %v", path, err)
	}
	return info.Mode().Perm()
}

func stubMkfsAndResize(t *testing.T) string {
	t.Helper()
	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"mkfs.ext4", "resize2fs"} {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", binDir)
	return filepath.Join(binDir, "mkfs.ext4")
}

func TestCreateDataImageIsOwnerOnly(t *testing.T) {
	stubMkfsAndResize(t)
	mgr := NewManager(DefaultManagerConfig())
	dataImg := filepath.Join(t.TempDir(), "vm", "data.img")
	if err := mgr.createDataImage(dataImg, 8); err != nil {
		t.Fatalf("createDataImage: %v", err)
	}
	if got := modeOf(t, dataImg); got != 0o600 {
		t.Fatalf("new data image mode = %o, want 600", got)
	}
}

func TestStageSparseImageIsOwnerOnly(t *testing.T) {
	root := t.TempDir()
	vmID := "vm-staging-mode"
	if err := os.Mkdir(filepath.Join(root, vmID), 0o750); err != nil {
		t.Fatal(err)
	}
	mkfs := stubMkfsAndResize(t)
	mgr := NewManager(ManagerConfig{MkfsExt4Path: mkfs})
	staging, err := mgr.StageSparseImage(root, vmID, 3, "opmode", 8)
	if err != nil {
		t.Fatalf("StageSparseImage: %v", err)
	}
	if got := modeOf(t, staging); got != 0o600 {
		t.Fatalf("staged image mode = %o, want 600", got)
	}
}

func TestQuarantineDataImageTightensTheMode(t *testing.T) {
	root := t.TempDir()
	vmID := "vm-quarantine-mode"
	vmDir := filepath.Join(root, vmID)
	if err := os.Mkdir(vmDir, 0o750); err != nil {
		t.Fatal(err)
	}
	image := filepath.Join(vmDir, "data.img")
	if err := os.WriteFile(image, []byte("image"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(image, 0o644); err != nil {
		t.Fatal(err)
	}
	mgr := NewManager(DefaultManagerConfig())
	quarantine, err := mgr.QuarantineDataImage(root, vmID, 1, "opone", 0)
	if err != nil {
		t.Fatalf("QuarantineDataImage: %v", err)
	}
	if got := modeOf(t, quarantine); got != 0o600 {
		t.Fatalf("quarantined image mode = %o, want 600", got)
	}
}

// bootWithExistingImage runs BootVM against nonexistent Firecracker inputs, so
// it fails after the data image stage (as TestBootVMExpandsExistingSmall...
// does), and returns the image path.
func bootWithExistingImage(t *testing.T, vmID string, imageMode os.FileMode) (stateDir, image string) {
	t.Helper()
	stubMkfsAndResize(t)
	stateDir = t.TempDir()
	cfg := DefaultManagerConfig()
	cfg.StateDir = stateDir
	cfg.StoreDiskPath = "/nonexistent/store.erofs"
	cfg.KernelImagePath = "/nonexistent/kernel"
	cfg.RootfsPath = "/nonexistent/rootfs"
	mgr := NewManager(cfg)
	image = filepath.Join(stateDir, vmID, "data.img")
	if err := os.MkdirAll(filepath.Dir(image), 0o750); err != nil {
		t.Fatal(err)
	}
	// Full size so the boot's resize step does not touch the file.
	if err := os.WriteFile(image, nil, imageMode); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(image, int64(dataImageSizeMB)*1024*1024); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(image, imageMode); err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.BootVM(VMConfig{VMID: vmID}); err == nil {
		t.Fatal("expected launch to fail with nonexistent Firecracker inputs")
	}
	return stateDir, image
}

func TestBootVMTightensAnExistingLooseDataImage(t *testing.T) {
	_, image := bootWithExistingImage(t, "old-vm-mode", 0o644)
	if got := modeOf(t, image); got != 0o600 {
		t.Fatalf("existing data image mode after boot = %o, want 600", got)
	}
}

// A tightened mode must not move the identity a memory snapshot is bound to
// (size, mtime, inode, device): chmod changes ctime only.
func TestTighteningTheModeKeepsTheSnapshotDiskIdentity(t *testing.T) {
	stateDir := t.TempDir()
	image := filepath.Join(stateDir, "vm-identity", "data.img")
	if err := os.MkdirAll(filepath.Dir(image), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(image, []byte("image"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(image, 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := diskIdentity(image)
	if err != nil {
		t.Fatal(err)
	}
	if err := tightenDataImageMode(image); err != nil {
		t.Fatalf("tightenDataImageMode: %v", err)
	}
	after, err := diskIdentity(image)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("disk identity moved: before %+v after %+v", before, after)
	}
	if got := modeOf(t, image); got != 0o600 {
		t.Fatalf("mode = %o, want 600", got)
	}
}

// The tightening must not follow a symlink: a data.img that points at another
// file would otherwise change that file's mode.
func TestBootVMDoesNotChmodThroughASymlinkedDataImage(t *testing.T) {
	stubMkfsAndResize(t)
	stateDir := t.TempDir()
	cfg := DefaultManagerConfig()
	cfg.StateDir = stateDir
	cfg.StoreDiskPath = "/nonexistent/store.erofs"
	cfg.KernelImagePath = "/nonexistent/kernel"
	cfg.RootfsPath = "/nonexistent/rootfs"
	mgr := NewManager(cfg)

	target := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.WriteFile(target, make([]byte, 16), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(target, 0o644); err != nil {
		t.Fatal(err)
	}
	vmDir := filepath.Join(stateDir, "vm-symlink")
	if err := os.MkdirAll(vmDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(vmDir, "data.img")); err != nil {
		t.Fatal(err)
	}
	_, _ = mgr.BootVM(VMConfig{VMID: "vm-symlink"})
	if got := modeOf(t, target); got != 0o644 {
		t.Fatalf("a symlink target's mode changed to %o; the tightening followed the link", got)
	}
}

// The helper itself never chmods through a link (the boot path already refuses
// a symlinked image before it gets here, so this pins the helper on its own).
func TestTightenDataImageModeDoesNotFollowASymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, make([]byte, 16), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(target, 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "data.img")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := tightenDataImageMode(link); err == nil {
		t.Fatal("a symlink was accepted as a data image")
	}
	if got := modeOf(t, target); got != 0o644 {
		t.Fatalf("the link target's mode changed to %o", got)
	}
}

// ensure the helper rejects a directory or device rather than chmod-ing it.
func TestTightenDataImageModeRefusesNonRegularFiles(t *testing.T) {
	dir := t.TempDir()
	if err := tightenDataImageMode(dir); err == nil {
		t.Fatal("a directory was accepted as a data image")
	}
	if err := tightenDataImageMode(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("a missing path was accepted as a data image")
	}
}
