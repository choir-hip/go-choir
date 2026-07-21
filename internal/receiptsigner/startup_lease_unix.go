//go:build unix

package receiptsigner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

type StartupLease struct {
	file *os.File
}

func AcquireStartupLease(path string) (*StartupLease, error) {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) || path == string(os.PathSeparator) {
		return nil, fmt.Errorf("receipt signer: absolute startup lease path is required")
	}
	directory := filepath.Dir(path)
	info, err := os.Lstat(directory)
	if err != nil {
		return nil, fmt.Errorf("receipt signer: startup lease directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("receipt signer: startup lease directory must be a directory")
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("receipt signer: open startup lease: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return nil, fmt.Errorf("receipt signer: startup lease permissions: %w", err)
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		file.Close()
		return nil, fmt.Errorf("receipt signer: acquire startup lease: %w", err)
	}
	return &StartupLease{file: file}, nil
}

func (l *StartupLease) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	file := l.file
	l.file = nil
	unlockErr := unix.Flock(int(file.Fd()), unix.LOCK_UN)
	closeErr := file.Close()
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
}

func StartupLeaseHeld(path string) (bool, error) {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) || path == string(os.PathSeparator) {
		return false, fmt.Errorf("receipt signer: absolute startup lease path is required")
	}
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return true, nil
		}
		return false, err
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_UN); err != nil {
		return false, err
	}
	return false, nil
}
