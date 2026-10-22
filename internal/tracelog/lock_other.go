//go:build !(linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package tracelog

import "os"

// openNoFollow is unavailable here; the Lstat checks are the only guard.
const openNoFollow = 0

// tryLockDir has no advisory lock on this platform. The guest is Linux; this
// build exists so the package compiles elsewhere. Without the lock a second
// writer is not refused, so nothing else may write the directory.
func tryLockDir(*os.File) (bool, error) { return true, nil }
