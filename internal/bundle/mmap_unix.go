//go:build unix

package bundle

import (
	"os"

	"golang.org/x/sys/unix"
)

// mapPack maps the pack read-only: one pread an object costs a syscall per
// event, payload and pin receipt (about 1 ms each on the measuring Mac), a
// mapping costs a page fault per page. The mapping is page cache, not heap.
func mapPack(f *os.File, size int64) ([]byte, func() error) {
	if size <= 0 || int64(int(size)) != size {
		return nil, nil
	}
	data, err := unix.Mmap(int(f.Fd()), 0, int(size), unix.PROT_READ, unix.MAP_SHARED)
	if err != nil {
		return nil, nil
	}
	return data, func() error { return unix.Munmap(data) }
}
