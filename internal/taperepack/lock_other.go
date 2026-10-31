//go:build !(linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package taperepack

// lockWorkDir is a no-op where flock is not available.
func lockWorkDir(string) (func(), error) { return func() {}, nil }

func fsFree(string) int64 { return -1 }
