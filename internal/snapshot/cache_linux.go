//go:build linux

package snapshot

import (
	"os"

	"golang.org/x/sys/unix"
)

// adviseDropFileCache asks the kernel to drop the clean pages of path's byte
// range [off, off+n) from the page cache (posix_fadvise DONTNEED; n 0 is to
// the end). Best effort: an error changes nothing.
func adviseDropFileCache(path string, off, n int64) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	_ = unix.Fadvise(int(f.Fd()), off, n, unix.FADV_DONTNEED)
}
