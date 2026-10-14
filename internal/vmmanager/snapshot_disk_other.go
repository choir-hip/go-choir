//go:build !linux

package vmmanager

import "os"

// measureFileDiskBytes is st_blocks*512 off Linux (no btrfs).
func measureFileDiskBytes(path string) (int64, string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, "", err
	}
	return stBlocksBytes(info), diskBytesMethodStBlocks, nil
}
