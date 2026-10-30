package yaegikernel

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// p5-go-served-0825 (docs/archive/problems/p5-go-served-0825-engineering-never-freezes-2026-10-15.md):
// choir.WriteFile on an existing source file failed with "permission denied"
// because the capsule's source files are 0444 in an immutable lower layer and
// directories are 0777. These tests reproduce that file-mode shape as an
// unprivileged user. Failure modes pinned: W1 an existing 0444 file cannot be
// edited; W2 a writable file stops being written in place; W3 a symlink is
// replaced instead of refused; W4 a directory target is touched; W5 an
// unwritable parent leaves a partial result; W6 the replace loses the exec
// bit or leaves the file unwritable; W7 an explicit mode is ignored; W8 a
// temp file is left behind.

func skipIfRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file modes; the 0444 shape cannot be reproduced")
	}
}

// lowerShape makes a directory like a lower layer's: 0777, holding a file with
// the given mode.
func lowerShape(t *testing.T, name, content string, mode os.FileMode) (dir, path string) {
	t.Helper()
	dir = t.TempDir()
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	return dir, path
}

func inode(t *testing.T, path string) uint64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Sys().(*syscall.Stat_t).Ino
}

func assertNoTemp(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".choir-write-") {
			t.Fatalf("W8: temp file %q left behind", e.Name())
		}
	}
}

func TestWriteFileReplacingEditsAReadOnlyExistingFile(t *testing.T) {
	skipIfRoot(t)
	dir, path := lowerShape(t, "routes.go", "old", 0o444)
	// Premise: the plain write is the failure the drill saw.
	if err := os.WriteFile(path, []byte("x"), 0o644); err == nil {
		t.Fatal("premise: os.WriteFile over a 0444 file succeeded")
	}
	if err := WriteFileReplacing(path, []byte("new content"), 0o644, false); err != nil {
		t.Fatalf("W1: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "new content" {
		t.Fatalf("content = %q, %v", got, err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o644 { // old 0444 plus owner-write
		t.Fatalf("W6: mode = %v, want 0644", info.Mode().Perm())
	}
	// The file is now plain-writable, so a second edit takes the plain path.
	before := inode(t, path)
	if err := WriteFileReplacing(path, []byte("again"), 0o644, false); err != nil {
		t.Fatal(err)
	}
	if inode(t, path) != before {
		t.Fatal("W2: a writable file was replaced instead of written in place")
	}
	assertNoTemp(t, dir)
}

func TestWriteFileReplacingKeepsTheExecBitAndHonoursAnExplicitMode(t *testing.T) {
	skipIfRoot(t)
	_, script := lowerShape(t, "run.sh", "old", 0o555)
	if err := WriteFileReplacing(script, []byte("#!/bin/sh\n"), 0o644, false); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(script); info.Mode().Perm() != 0o755 {
		t.Fatalf("W6: exec file mode = %v, want 0755", info.Mode().Perm())
	}
	_, explicit := lowerShape(t, "tool", "old", 0o444)
	if err := WriteFileReplacing(explicit, []byte("x"), 0o600, true); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(explicit); info.Mode().Perm() != 0o600 {
		t.Fatalf("W7: explicit mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestWriteFileReplacingWritesAWritableFileInPlace(t *testing.T) {
	_, path := lowerShape(t, "plain.go", "old", 0o644)
	before := inode(t, path)
	if err := WriteFileReplacing(path, []byte("new"), 0o644, false); err != nil {
		t.Fatal(err)
	}
	if inode(t, path) != before {
		t.Fatal("W2: a writable file was replaced")
	}
	// A new file is created as before.
	fresh := filepath.Join(filepath.Dir(path), "new.go")
	if err := WriteFileReplacing(fresh, []byte("n"), 0o644, false); err != nil {
		t.Fatal(err)
	}
}

func TestWriteFileReplacingRefusesSymlinksAndDirectories(t *testing.T) {
	skipIfRoot(t)
	dir, target := lowerShape(t, "real.go", "old", 0o444)
	link := filepath.Join(dir, "link.go")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileReplacing(link, []byte("new"), 0o644, false); err == nil {
		t.Fatal("W3: write through a symlink to a read-only file succeeded")
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("W3: the symlink was replaced: %v %v", info, err)
	}
	if got, _ := os.ReadFile(target); string(got) != "old" {
		t.Fatalf("W3: symlink target changed to %q", got)
	}
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileReplacing(sub, []byte("x"), 0o644, false); err == nil {
		t.Fatal("W4: wrote over a directory")
	}
	if info, err := os.Stat(sub); err != nil || !info.IsDir() {
		t.Fatalf("W4: directory replaced: %v %v", info, err)
	}
	assertNoTemp(t, dir)
}

func TestWriteFileReplacingLeavesTheTargetWhenTheParentIsNotWritable(t *testing.T) {
	skipIfRoot(t)
	dir, path := lowerShape(t, "locked.go", "old", 0o444)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	err := WriteFileReplacing(path, []byte("new"), 0o644, false)
	if err == nil {
		t.Fatal("W5: replaced a file in an unwritable directory")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("W5: error lost the original cause: %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "old" {
		t.Fatalf("W5: target changed to %q", got)
	}
	assertNoTemp(t, dir)
}
