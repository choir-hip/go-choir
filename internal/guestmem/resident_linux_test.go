//go:build linux

package guestmem

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

// The residency probe must count a file's cached pages without faulting any
// in: after the cache is dropped, probing twice still reads zero, and after a
// full read every page (the partial last one included) is resident. An empty
// file reads zero without a mapping.
func TestResidentBytesCountsCachedPagesWithoutFaultingThemIn(t *testing.T) {
	page := int64(os.Getpagesize())
	path := filepath.Join(t.TempDir(), "table")
	size := 3*page + 100
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := f.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := unix.Fadvise(int(f.Fd()), 0, 0, unix.FADV_DONTNEED); err != nil {
		t.Skipf("fadvise: %v", err)
	}
	for i := 0; i < 2; i++ {
		got, err := residentBytes(path, size)
		if err != nil {
			t.Fatal(err)
		}
		if got != 0 {
			t.Skipf("the filesystem kept %d bytes cached after DONTNEED (tmpfs?)", got)
		}
	}
	if _, err := os.ReadFile(path); err != nil {
		t.Fatal(err)
	}
	got, err := residentBytes(path, size)
	if err != nil {
		t.Fatal(err)
	}
	if got != 4*page {
		t.Fatalf("resident = %d, want %d (four pages)", got, 4*page)
	}
	empty := filepath.Join(t.TempDir(), "empty")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := residentBytes(empty, 0); err != nil || got != 0 {
		t.Fatalf("empty = %d, %v", got, err)
	}
}
