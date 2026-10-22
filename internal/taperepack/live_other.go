//go:build !(linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package taperepack

// refuseLiveWriter has no advisory lock to try on this platform, as the
// appender has none (internal/tapestore lock_other.go).
func refuseLiveWriter(string, string) error { return nil }
