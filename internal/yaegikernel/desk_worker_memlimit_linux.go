//go:build linux

package yaegikernel

import (
	"fmt"
	"syscall"

	"golang.org/x/sys/unix"
)

// ApplyWorkerMemoryLimit sets RLIMIT_AS on the calling worker process —
// the D2 memory cap: a model-authored cell cannot grow its address space
// past bytes and OOM the daemon (R3r).
func ApplyWorkerMemoryLimit(bytes uint64) error {
	if bytes == 0 {
		return nil
	}
	if err := unix.Setrlimit(unix.RLIMIT_AS, &unix.Rlimit{Cur: bytes, Max: bytes}); err != nil {
		return fmt.Errorf("setrlimit RLIMIT_AS %d: %w", bytes, err)
	}
	return nil
}
