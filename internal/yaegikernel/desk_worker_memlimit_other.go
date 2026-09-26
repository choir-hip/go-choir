//go:build !linux

package yaegikernel

import (
	"errors"
	"fmt"
	"syscall"
)

// ApplyWorkerMemoryLimit sets RLIMIT_AS on the calling worker process.
// Portable fallback (Darwin/*BSD): syscall.Setrlimit is deprecated but
// still functional where the kernel honors RLIMIT_AS. Darwin defines the
// rlimit but does not honor it — Setrlimit(RLIMIT_AS) fails EINVAL — so an
// unsupported-kernel answer degrades to uncapped rather than killing the
// worker: the subprocess+package boundary (primary containment) still
// holds, the worker simply lacks the extra address cap. Real setrlimit
// errors still propagate.
func ApplyWorkerMemoryLimit(bytes uint64) error {
	if bytes == 0 {
		return nil
	}
	if err := syscall.Setrlimit(syscall.RLIMIT_AS, &syscall.Rlimit{Cur: bytes, Max: bytes}); err != nil {
		if errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.ENOSYS) {
			// Kernel does not support RLIMIT_AS here — degrade to uncapped.
			return nil
		}
		return fmt.Errorf("setrlimit RLIMIT_AS %d: %w", bytes, err)
	}
	return nil
}
