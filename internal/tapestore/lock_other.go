//go:build !(linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package tapestore

import "os"

// openNoFollow is unavailable here; the Lstat checks are the only guard.
const openNoFollow = 0

// lockDir has no advisory lock on this platform; the in-process mutex is the
// only serialization, which is correct while the platform process is the
// only writer (appendix A.5).
func lockDir(*os.File) (func(), error) { return func() {}, nil }
