//go:build !(linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package updater

import "os"

// lockFile is a no-op where the updater does not run.
func lockFile(*os.File, bool) (func(), bool, error) { return func() {}, true, nil }
