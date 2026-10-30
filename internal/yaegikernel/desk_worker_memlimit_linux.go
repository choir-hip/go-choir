//go:build linux

package yaegikernel

import (
	"fmt"
	"os"

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

// VolunteerForOOMKill raises the calling worker's oom_score_adj to the
// maximum, so when a guest runs out of memory the kernel kills a desk worker
// before the daemon. The address-space cap above cannot prevent that on its
// own: it is 8 GiB, the size of the guest. On the paper two runaway cells
// filled the guest and the kernel killed the daemon, the largest process,
// closing every run
// (docs/archive/problems/desk-cell-init-only-for-loop-never-ends-guest-oom-2026-10-16.md).
// Raising one's own score needs no privilege.
func VolunteerForOOMKill() error {
	return os.WriteFile("/proc/self/oom_score_adj", []byte("1000"), 0)
}
