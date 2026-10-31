//go:build !linux

package store

// dropFileCache is a no-op off linux (the guest is linux).
func dropFileCache(string) {}
