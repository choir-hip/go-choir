//go:build linux

package capsule

import (
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// overlayMarks reads the two overlay facts about an upper-layer entry that
// a stat alone does not name as such:
//
//   - whiteout: a character device numbered 0:0 is how overlayfs records
//     that a lower-layer entry was deleted;
//   - opaque: a directory carrying user.overlay.opaque=y (the xattr form
//     MountOverlayFS's userxattr option uses) hides every lower entry under
//     it, i.e. a lower directory was removed and recreated.
//
// An xattr that cannot be read (unsupported filesystem) reads as not opaque:
// the same state the release dir's ENOTSUP fallback already accepts.
func overlayMarks(path string, info os.FileInfo) (whiteout, opaque bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false, false
	}
	switch {
	case info.Mode()&os.ModeCharDevice != 0:
		rdev := uint64(st.Rdev)
		return unix.Major(rdev) == 0 && unix.Minor(rdev) == 0, false
	case info.IsDir():
		var buf [4]byte
		n, err := unix.Lgetxattr(path, overlayOpaqueXattr, buf[:])
		return false, err == nil && n == 1 && buf[0] == 'y'
	}
	return false, false
}
