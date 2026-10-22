//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package tracelog

import (
	"os"
	"syscall"
)

// openNoFollow makes open fail on a symlink at the final path component.
const openNoFollow = syscall.O_NOFOLLOW

// tryLockDir takes an exclusive advisory lock on an open directory without
// waiting. It reports false when another descriptor, in this process or
// another, holds it. The lock lives as long as the descriptor stays open.
func tryLockDir(d *os.File) (bool, error) {
	for {
		err := syscall.Flock(int(d.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		switch err {
		case nil:
			return true, nil
		case syscall.EINTR:
			continue
		case syscall.EWOULDBLOCK:
			return false, nil
		default:
			return false, err
		}
	}
}
