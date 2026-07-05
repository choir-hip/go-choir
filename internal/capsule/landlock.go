//go:build linux


package capsule

import (
	"fmt"

	"github.com/landlock-lsm/go-landlock/landlock"
	ll "github.com/landlock-lsm/go-landlock/landlock/syscall"
)

// LandlockRestrictor applies Landlock path restrictions to the current
// process. Landlock is a Linux security module (5.13+) that provides
// unprivileged filesystem access control.
//
// For the broker: restricts access to the capsule's merged dir + broker
// store + /dev/pts (for PTY).
//
// For the workload: restricts access to the capsule's merged dir only.
type LandlockRestrictor struct {
	paths []string // allowed paths
}

// NewBrokerLandlock creates a Landlock restrictor for the broker process.
// The broker needs access to: capsule merged dir, broker store, /dev/pts,
// /dev/null, /dev/zero, /dev/urandom, /tmp (for session temp files).
func NewBrokerLandlock(mergedDir, brokerStore string) *LandlockRestrictor {
	return &LandlockRestrictor{
		paths: []string{
			mergedDir,
			brokerStore,
			"/dev/pts",
			"/dev/null",
			"/dev/zero",
			"/dev/urandom",
			"/dev/random",
			"/tmp",
			"/proc/self",
		},
	}
}

// NewWorkloadLandlock creates a Landlock restrictor for the workload process.
// The workload only gets access to the capsule's merged dir + /dev/pts +
// /dev/null + /dev/zero + /dev/urandom.
func NewWorkloadLandlock(mergedDir string) *LandlockRestrictor {
	return &LandlockRestrictor{
		paths: []string{
			mergedDir,
			"/dev/pts",
			"/dev/null",
			"/dev/zero",
			"/dev/urandom",
			"/dev/random",
			"/tmp",
			"/proc/self",
		},
	}
}

// Apply applies the Landlock restrictions to the current process.
// Must be called after fork, before exec. Requires Linux 5.13+.
// Uses the best available Landlock ABI with best-effort fallback.
func (r *LandlockRestrictor) Apply() error {
	if len(r.paths) == 0 {
		return fmt.Errorf("no paths configured for Landlock restriction")
	}

	// Use PathAccess with the full read+write access set.
	// The landlock library handles ABI version negotiation.
	rules := []landlock.Rule{
		landlock.PathAccess(
			landlock.AccessFSSet(ll.AccessFSWriteFile|
				ll.AccessFSReadFile|
				ll.AccessFSReadDir|
				ll.AccessFSMakeDir|
				ll.AccessFSRemoveFile|
				ll.AccessFSRemoveDir|
				ll.AccessFSMakeSym|
				ll.AccessFSTruncate|
				ll.AccessFSExecute),
			r.paths...,
		),
	}

	// Restrict to the best available Landlock ABI (best-effort).
	err := landlock.V5.BestEffort().RestrictPaths(rules...)
	if err != nil {
		return fmt.Errorf("failed to apply Landlock restrictions: %w", err)
	}

	return nil
}
