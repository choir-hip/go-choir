//go:build linux

package vmmanager

import (
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// The FIEMAP ioctl itself, on whatever filesystem the test runs on (CI:
// ext4 or overlay). Failure modes: wrong ioctl number or struct layout
// (EINVAL, or garbage extents), a walk that misses the data after a hole,
// and a bound above st_blocks for plain (uncompressed) extents.
func TestFiemapFileExtentsOnThisFilesystem(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mem")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	data := make([]byte, 256<<10)
	if _, err := rand.Read(data); err != nil {
		t.Fatal(err)
	}
	// Data, a 4 MiB hole, data.
	if _, err := f.WriteAt(data, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt(data, int64(len(data))+4<<20); err != nil {
		t.Fatal(err)
	}
	if err := f.Sync(); err != nil {
		t.Fatal(err)
	}
	exts, err := fiemapFileExtents(int(f.Fd()))
	if errors.Is(err, syscall.EOPNOTSUPP) || errors.Is(err, syscall.ENOTTY) {
		t.Skipf("filesystem has no FIEMAP: %v", err)
	}
	if err != nil {
		t.Fatalf("fiemap: %v", err)
	}
	var mapped uint64
	end := uint64(0)
	for _, e := range exts {
		mapped += e.Length
		if e.Logical+e.Length > end {
			end = e.Logical + e.Length
		}
	}
	if mapped < uint64(2*len(data)) || end < uint64(2*len(data))+4<<20 {
		t.Fatalf("extents map %d bytes ending at %d; want both data ranges: %+v", mapped, end, exts)
	}
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	stBlocks := stBlocksBytes(info)
	bound := fiemapBoundBytes(exts)
	if bound < int64(2*len(data)) || clampFiemapBound(bound, stBlocks) > stBlocks {
		t.Fatalf("bound %d for %d bytes of random data (st_blocks %d)", bound, 2*len(data), stBlocks)
	}
}
