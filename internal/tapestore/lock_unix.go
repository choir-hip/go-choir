//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package tapestore

import (
	"os"
	"syscall"
)

// openNoFollow makes open fail on a symlink at the final path component.
const openNoFollow = syscall.O_NOFOLLOW

// lockDir takes an exclusive advisory lock on an open directory and returns
// its release. It is the cross-instance (and cross-process) half of the
// per-computer write lock; the in-process half is a mutex.
func lockDir(d *os.File) (func(), error) {
	for {
		err := syscall.Flock(int(d.Fd()), syscall.LOCK_EX)
		if err == syscall.EINTR {
			continue
		}
		if err != nil {
			return nil, err
		}
		break
	}
	return func() { _ = syscall.Flock(int(d.Fd()), syscall.LOCK_UN) }, nil
}

// statBlocks is st_blocks in bytes.
func statBlocks(fi os.FileInfo) int64 {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return int64(st.Blocks) * 512
	}
	return 0
}

// copyOwner gives dst the owner of src when they differ.
func copyOwner(src os.FileInfo, dst *os.File) error {
	st, ok := src.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	cur, err := dst.Stat()
	if err != nil {
		return err
	}
	if ct, ok := cur.Sys().(*syscall.Stat_t); ok && ct.Uid == st.Uid && ct.Gid == st.Gid {
		return nil
	}
	return dst.Chown(int(st.Uid), int(st.Gid))
}
