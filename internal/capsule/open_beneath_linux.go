//go:build linux

package capsule

import (
	"os"

	"golang.org/x/sys/unix"
)

// openBeneath opens rel for reading beneath the directory rootFD without
// following a symbolic link in any component (RESOLVE_NO_SYMLINKS, magic
// links included) and without leaving the root (RESOLVE_BENEATH). The trees
// these roots name are ones a cell can write, so a path is never resolved
// through a link the cell planted. O_NONBLOCK keeps a FIFO from hanging the
// open; the caller fstats the result before trusting what it is.
func openBeneath(rootFD int, rel string) (*os.File, error) {
	fd, err := unix.Openat2(rootFD, rel, &unix.OpenHow{
		Flags:   uint64(unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NONBLOCK),
		Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_MAGICLINKS,
	})
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), rel), nil
}
