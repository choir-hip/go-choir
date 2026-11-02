//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package updater

import (
	"errors"
	"os"
	"syscall"
)

// lockFile takes an exclusive advisory lock on f, waiting when wait is set.
// acquired=false with a nil error means another holder has it (only when not
// waiting).
func lockFile(f *os.File, wait bool) (func(), bool, error) {
	how := syscall.LOCK_EX
	if !wait {
		how |= syscall.LOCK_NB
	}
	for {
		err := syscall.Flock(int(f.Fd()), how)
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, err
		}
		return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }, true, nil
	}
}
