//go:build unix

package tapefollow

import (
	"os"
	"syscall"
)

// lockDir takes an exclusive, non-blocking flock on path: one syncer per
// store (the app and a CLI follow on the same directory refuse each other).
// The kernel drops it when the process dies, so a crash never leaves it held.
func lockDir(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o640)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}
