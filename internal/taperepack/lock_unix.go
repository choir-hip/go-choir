//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package taperepack

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// lockWorkDir takes a non-blocking exclusive lock on a file of the work
// directory, so two runs never share spill files. It does not lock the
// artifacts store; the appender's own per-computer lock does that.
func lockWorkDir(path string) (release func(), err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err != syscall.EINTR {
			break
		}
	}
	if err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, fmt.Errorf("taperepack: another run holds the work directory (%s)", path)
		}
		return nil, err
	}
	return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, nil
}
