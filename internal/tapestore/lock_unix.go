//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package tapestore

import (
	"os"
	"syscall"
)

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
