//go:build linux

package store

import (
	"os"

	"golang.org/x/sys/unix"
)

// dropFileCache asks the kernel to drop path's clean pages from the page
// cache. A 20 GB copy would otherwise leave its pages cached against the
// guest's memory. Best effort: an error changes nothing.
func dropFileCache(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	_ = unix.Fadvise(int(f.Fd()), 0, 0, unix.FADV_DONTNEED)
}
