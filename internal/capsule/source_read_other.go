//go:build !linux

package capsule

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// readSourceBeneath is the portable stand-in for the Linux openat2 reader
// (source_read_linux.go), with the same results: it Lstats every component
// beneath root and refuses a symbolic link in any of them. It is not
// race-free against a concurrent writer; production capsules run on Linux.
func readSourceBeneath(root, rel string) (data []byte, isDir bool, err error) {
	parts := strings.Split(rel, "/")
	p := root
	for i, part := range parts {
		if part == "" || part == "." || part == ".." {
			return nil, false, fmt.Errorf("unsafe source path %q", rel)
		}
		p = filepath.Join(p, part)
		info, err := os.Lstat(p)
		if err != nil {
			return nil, false, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, false, errSourcePatchSymlink
		}
		if i < len(parts)-1 {
			if !info.IsDir() {
				return nil, false, &fs.PathError{Op: "open", Path: p, Err: syscall.ENOTDIR}
			}
			continue
		}
		switch {
		case info.IsDir():
			return nil, true, nil
		case !info.Mode().IsRegular():
			return nil, false, fmt.Errorf("not a regular file (mode %s)", info.Mode())
		}
		data, err = os.ReadFile(p)
		return data, false, err
	}
	return nil, false, fmt.Errorf("unsafe source path %q", rel)
}
