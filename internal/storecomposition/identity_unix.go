//go:build unix

package storecomposition

import (
	"os"
	"syscall"
)

// sysIdentity is the device and inode of a stat result.
func sysIdentity(fi os.FileInfo) (dev, ino uint64) {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return uint64(st.Dev), uint64(st.Ino) //nolint:unconvert // Dev's width differs by platform
	}
	return 0, 0
}
