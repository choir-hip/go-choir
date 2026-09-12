//go:build linux

package capsule

import (
	"fmt"
	"os"

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
// The V5 contract is mandatory; unsupported or boot-disabled Landlock fails closed.
func (r *LandlockRestrictor) Apply() error {
	if len(r.paths) == 0 {
		return fmt.Errorf("no paths configured for Landlock restriction")
	}

	directoryAccess := landlock.AccessFSSet(ll.AccessFSWriteFile |
		ll.AccessFSReadFile |
		ll.AccessFSReadDir |
		ll.AccessFSMakeDir |
		ll.AccessFSRemoveFile |
		ll.AccessFSRemoveDir |
		ll.AccessFSMakeSym |
		ll.AccessFSTruncate |
		ll.AccessFSExecute)
	fileAccess := landlock.AccessFSSet(ll.AccessFSWriteFile |
		ll.AccessFSReadFile |
		ll.AccessFSTruncate |
		ll.AccessFSExecute)
	directories := make([]string, 0, len(r.paths))
	files := make([]string, 0, len(r.paths))
	for _, path := range r.paths {
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("inspect Landlock allow path %q: %w", path, err)
		}
		if info.IsDir() {
			directories = append(directories, path)
		} else {
			files = append(files, path)
		}
	}
	rules := make([]landlock.Rule, 0, 2)
	if len(directories) > 0 {
		rules = append(rules, landlock.PathAccess(directoryAccess, directories...))
	}
	if len(files) > 0 {
		rules = append(rules, landlock.PathAccess(fileAccess, files...))
	}

	// Require the complete V5 contract. Downgrading would make isolation depend
	// on the host kernel instead of the frozen capsule policy.
	err := landlock.V5.RestrictPaths(rules...)
	if err != nil {
		return fmt.Errorf("failed to apply Landlock restrictions: %w", err)
	}

	return nil
}
