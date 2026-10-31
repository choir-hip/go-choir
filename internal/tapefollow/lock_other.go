//go:build !unix

package tapefollow

// lockDir is a no-op where flock does not exist; the follower ships on
// macOS first.
func lockDir(string) (func(), error) { return func() {}, nil }
