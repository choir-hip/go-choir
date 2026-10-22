//go:build linux

package vmctl

import (
	"os"
	"syscall"
	"time"
)

// inodeChangeTime is the file's ctime: the rename into quarantine sets it,
// and any later metadata change only makes it younger. fallback is returned
// when the platform gives no ctime.
func inodeChangeTime(info os.FileInfo, fallback time.Time) time.Time {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return time.Unix(int64(stat.Ctim.Sec), int64(stat.Ctim.Nsec))
	}
	return fallback
}

// allocatedBytes is the file's allocated size (a sparse image's real cost).
func allocatedBytes(info os.FileInfo) int64 {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && stat.Blocks > 0 {
		return stat.Blocks * 512
	}
	return info.Size()
}
