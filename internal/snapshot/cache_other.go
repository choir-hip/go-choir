//go:build !linux

package snapshot

// adviseDropFileCache is a no-op off linux (the guest is linux).
func adviseDropFileCache(string, int64, int64) {}
