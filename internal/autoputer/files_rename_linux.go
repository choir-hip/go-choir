//go:build linux

package autoputer

import (
	"errors"

	"golang.org/x/sys/unix"
)

// renameNoReplace moves src to dst atomically and fails with an error that
// satisfies errors.Is(err, fs.ErrExist) if dst exists, however it came to
// exist after the caller's check (rename(2) alone replaces a regular file).
func renameNoReplace(src, dst string) error {
	err := unix.Renameat2(unix.AT_FDCWD, src, unix.AT_FDCWD, dst, unix.RENAME_NOREPLACE)
	if errors.Is(err, unix.EINVAL) || errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.ENOTSUP) {
		// The filesystem or kernel lacks RENAME_NOREPLACE.
		return renameLinkFallback(src, dst)
	}
	return err
}
