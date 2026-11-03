package ownerfiles

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// A hardlink taken before a rewrite keeps the old bytes: the helper replaces
// by rename and never writes the linked inode (failure mode 2).
func TestWriteFileReplacesByRenameSoALinkKeepsTheOldBytes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "snap")
	if err := os.Link(path, link); err != nil {
		t.Skipf("hardlink unsupported here: %v", err)
	}
	if err := WriteFile(path, []byte("new"), 0o640); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(link); string(got) != "old" {
		t.Fatalf("linked inode changed: %q", got)
	}
	if got, _ := os.ReadFile(path); string(got) != "new" {
		t.Fatalf("path = %q", got)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o640 {
		t.Fatalf("mode = %v, %v", info.Mode().Perm(), err)
	}
	assertNoTemps(t, dir)
}

// The helper syncs the file before the rename and the directory after it,
// and a failed sync leaves the old file and no temporary (failure mode 3).
func TestWriteFileSyncsFileThenDirectoryAndCleansUpOnFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	restore := Sync
	t.Cleanup(func() { Sync = restore })
	var synced []string
	Sync = func(f *os.File) error {
		info, _ := f.Stat()
		kind := "file"
		if info != nil && info.IsDir() {
			kind = "dir"
		}
		synced = append(synced, kind)
		return nil
	}
	if err := WriteFile(path, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if strings.Join(synced, ",") != "file,dir" {
		t.Fatalf("sync order = %v", synced)
	}
	Sync = func(*os.File) error { return errors.New("disk said no") }
	if err := WriteFile(path, []byte("two"), 0o644); err == nil {
		t.Fatal("a failed sync was acknowledged")
	}
	if got, _ := os.ReadFile(path); string(got) != "one" {
		t.Fatalf("old file lost: %q", got)
	}
	assertNoTemps(t, dir)
}

// A fill error leaves the old file and no temporary.
func TestWriteFromFillErrorKeepsTheOldFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "b.bin")
	if err := WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := WriteFrom(path, 0o600, func(w io.Writer) error {
		_, _ = w.Write([]byte("partial"))
		return errors.New("chunk fetch failed")
	})
	if err == nil {
		t.Fatal("fill error was swallowed")
	}
	if got, _ := os.ReadFile(path); string(got) != "keep" {
		t.Fatalf("old file lost: %q", got)
	}
	assertNoTemps(t, dir)
}

// Exclusive waits for a held Tx, and a Tx's own methods never take the read
// side again, so a Tx doing many writes while Exclusive waits finishes
// (failure mode 1).
func TestExclusiveWaitsForATxAndATxNeverRelocks(t *testing.T) {
	dir := t.TempDir()
	tx := Begin()
	var exclusiveRan atomic.Bool
	done := make(chan struct{})
	go func() {
		_ = Exclusive(func() error { exclusiveRan.Store(true); return nil })
		close(done)
	}()
	// Give Exclusive time to queue for the write side; a recursive RLock
	// would now deadlock.
	time.Sleep(50 * time.Millisecond)
	for i := 0; i < 20; i++ {
		name := filepath.Join(dir, "f"+string(rune('a'+i)))
		if err := tx.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := tx.Rename(name, name+".moved"); err != nil {
			t.Fatal(err)
		}
	}
	if exclusiveRan.Load() {
		t.Fatal("Exclusive ran while a Tx held the read side")
	}
	tx.End()
	tx.End() // idempotent
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Exclusive never ran after the Tx ended")
	}
	if !exclusiveRan.Load() {
		t.Fatal("Exclusive did not run")
	}
}

// A package-level write waits while Exclusive holds the write side.
func TestWritesWaitForExclusive(t *testing.T) {
	dir := t.TempDir()
	release := make(chan struct{})
	held := make(chan struct{})
	go func() {
		_ = Exclusive(func() error { close(held); <-release; return nil })
	}()
	<-held
	wrote := make(chan error, 1)
	go func() { wrote <- WriteFile(filepath.Join(dir, "w"), []byte("x"), 0o644) }()
	select {
	case <-wrote:
		t.Fatal("a write ran while Exclusive held the lock")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	select {
	case err := <-wrote:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the write never ran")
	}
}

func TestRenameNoReplaceRefusesAnExistingTarget(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	if err := WriteFile(a, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(b, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RenameNoReplace(a, b); !errors.Is(err, os.ErrExist) {
		t.Fatalf("err = %v, want ErrExist", err)
	}
	if got, _ := os.ReadFile(b); string(got) != "b" {
		t.Fatalf("target replaced: %q", got)
	}
}

func assertNoTemps(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), TempInfix) {
			t.Fatalf("temporary left behind: %s", e.Name())
		}
	}
}
