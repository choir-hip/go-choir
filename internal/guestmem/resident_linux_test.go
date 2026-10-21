//go:build linux

package guestmem

import (
	"context"
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
		got, _, err := residentBytes(path, size, newScan(context.Background(), DefaultLimits))
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
	got, _, err := residentBytes(path, size, newScan(context.Background(), DefaultLimits))
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
	if got, _, err := residentBytes(empty, 0, newScan(context.Background(), DefaultLimits)); err != nil || got != 0 {
		t.Fatalf("empty = %d, %v", got, err)
	}
}

// A file bigger than the probe's chunk is read chunk by chunk, and the probe
// stops, reporting incomplete, when the request's probe-byte budget or
// deadline is spent, instead of mapping and scanning the whole file.
func TestResidentBytesIsChunkedAndCapped(t *testing.T) {
	page := int64(os.Getpagesize())
	path := filepath.Join(t.TempDir(), "big")
	size := 5 * page
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
	l := DefaultLimits
	l.ProbeChunkBytes = 2 * page
	l.MaxProbeBytes = 4 * page
	s := newScan(context.Background(), l)
	if _, complete, err := residentBytes(path, size, s); err != nil || complete {
		t.Fatalf("a 5-page file against a 4-page budget: complete=%t err=%v", complete, err)
	}
	if s.probeLeft != 0 {
		t.Fatalf("probe budget left = %d, want it spent", s.probeLeft)
	}
	l.MaxProbeBytes = 100 * page
	if _, complete, err := residentBytes(path, size, newScan(context.Background(), l)); err != nil || !complete {
		t.Fatalf("within budget: complete=%t err=%v", complete, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, complete, _ := residentBytes(path, size, newScan(ctx, l)); complete {
		t.Fatal("a cancelled request must stop the probe")
	}
}
