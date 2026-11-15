//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package updater

import (
	"io/fs"
	"syscall"
)

// allocatedBytes is what a file occupies on the volume (st_blocks), the unit
// the data_volume observability section reports.
func allocatedBytes(info fs.FileInfo) int64 {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return int64(stat.Blocks) * 512
	}
	return info.Size()
}

// syncFilesystems asks the kernel to commit what it holds: ext4 returns freed
// blocks to the allocator only when the transaction that freed them commits.
func syncFilesystems() { syscall.Sync() }
