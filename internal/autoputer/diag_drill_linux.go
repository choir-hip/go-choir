//go:build linux

package autoputer

import (
	"os"

	"golang.org/x/sys/unix"
)

// dropFromCache drops the file's pages from the guest page cache, so the next
// read of them reaches the disk.
func dropFromCache(f *os.File, size int64) {
	_ = unix.Fadvise(int(f.Fd()), 0, size, unix.FADV_DONTNEED)
}

func fdatasync(f *os.File) error { return unix.Fdatasync(int(f.Fd())) }
