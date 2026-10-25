//go:build linux

package capsule

import (
	"errors"
	"fmt"
	"io"

	"golang.org/x/sys/unix"
)

// readSourceBeneath reads rel (slash-separated) beneath root without
// following a symbolic link in any component, the way release staging reads
// a capsule's files (openBeneath: openat2 RESOLVE_BENEATH|RESOLVE_NO_SYMLINKS)
// and then fstats what it opened. It returns isDir for a directory, the
// bytes for a regular file, an error wrapping errSourcePatchSymlink for a
// symlink anywhere on the path, and the open's error (ENOENT, ENOTDIR) for
// a path that is not there. Anything else is refused.
func readSourceBeneath(root, rel string) (data []byte, isDir bool, err error) {
	rootFD, err := unix.Open(root, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, false, fmt.Errorf("source root is unavailable: %w", err)
	}
	defer unix.Close(rootFD)
	in, err := openBeneath(rootFD, rel)
	if errors.Is(err, unix.ELOOP) {
		return nil, false, errSourcePatchSymlink
	}
	if err != nil {
		return nil, false, err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return nil, false, err
	}
	switch {
	case info.IsDir():
		return nil, true, nil
	case !info.Mode().IsRegular():
		return nil, false, fmt.Errorf("not a regular file (mode %s)", info.Mode())
	}
	data, err = io.ReadAll(in)
	return data, false, err
}
