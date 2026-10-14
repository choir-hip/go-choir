package vmmanager

// On-disk size of snapshot files (design note items 39-43). Node B's state
// filesystem is btrfs with compress=zstd:3: an 8 GiB guest memory file uses
// about 6% of that on disk. st_blocks on btrfs counts the uncompressed size
// of allocated extents, and FIEMAP reports logical extent lengths, so
// neither sees compression. On btrfs the file's extent items are read with
// BTRFS_IOC_TREE_SEARCH (snapshot_disk_linux.go), as compsize does: each
// distinct extent's disk_num_bytes counts once, inline extents count their
// inline length, holes count zero. Any failure falls back to st_blocks*512,
// which overcounts on btrfs (the safe direction for a budget).

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

const (
	// diskBytesMethodBtrfs: summed from the file's btrfs extent items.
	diskBytesMethodBtrfs = "btrfs_extents"
	// diskBytesMethodStBlocks: st_blocks*512 (allocated, not compressed).
	diskBytesMethodStBlocks = "st_blocks"
	// diskBytesMethodMixed: the files of one snapshot were measured by
	// different methods.
	diskBytesMethodMixed = "mixed"

	btrfsExtentDataKey = 108
	// btrfsSearchHeaderSize: transid, objectid, offset (u64); type, len (u32).
	btrfsSearchHeaderSize = 32
	// btrfsFileExtentInlineHeaderSize: generation, ram_bytes (u64);
	// compression, encryption (u8); other_encoding (u16); type (u8).
	btrfsFileExtentInlineHeaderSize = 21
	// btrfsFileExtentRegularSize: the header plus disk_bytenr,
	// disk_num_bytes, offset, num_bytes (u64).
	btrfsFileExtentRegularSize = 53
	btrfsFileExtentInline      = 0
	btrfsFileExtentReg         = 1
	btrfsFileExtentPrealloc    = 2
)

// fileDiskBytes reports the bytes a regular file occupies on disk and the
// method used. Tests replace it.
var fileDiskBytes = measureFileDiskBytes

func stBlocksBytes(info os.FileInfo) int64 {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return int64(st.Blocks) * 512
	}
	return info.Size()
}

// dirDiskBytes sums the on-disk bytes of the regular files in dir.
func dirDiskBytes(dir string) (int64, string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, ""
	}
	var total int64
	method := ""
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		n, m, err := fileDiskBytes(filepath.Join(dir, entry.Name()))
		if err != nil {
			n, m = stBlocksBytes(info), diskBytesMethodStBlocks
		}
		total += n
		switch {
		case method == "":
			method = m
		case method != m:
			method = diskBytesMethodMixed
		}
	}
	return total, method
}

// parseBtrfsExtentItems sums the on-disk bytes of the EXTENT_DATA items of
// inode ino in one BTRFS_IOC_TREE_SEARCH result buffer holding nr items.
// seen carries the extents already counted (an extent referenced twice is
// stored once). last is the key offset of the final item, for paging.
// Search headers are in host byte order; item bodies are on-disk
// little-endian.
func parseBtrfsExtentItems(buf []byte, nr uint32, ino uint64, seen map[uint64]struct{}) (sum int64, last uint64, err error) {
	off := 0
	for i := uint32(0); i < nr; i++ {
		if off+btrfsSearchHeaderSize > len(buf) {
			return 0, 0, fmt.Errorf("btrfs search buffer truncated at item %d header", i)
		}
		objectid := binary.NativeEndian.Uint64(buf[off+8:])
		offset := binary.NativeEndian.Uint64(buf[off+16:])
		typ := binary.NativeEndian.Uint32(buf[off+24:])
		length := int(binary.NativeEndian.Uint32(buf[off+28:]))
		off += btrfsSearchHeaderSize
		if off+length > len(buf) {
			return 0, 0, fmt.Errorf("btrfs search buffer truncated at item %d body", i)
		}
		item := buf[off : off+length]
		off += length
		last = offset
		if objectid != ino || typ != btrfsExtentDataKey {
			continue
		}
		if len(item) < btrfsFileExtentInlineHeaderSize {
			return 0, 0, fmt.Errorf("btrfs file extent item %d is %d bytes", i, len(item))
		}
		switch item[20] {
		case btrfsFileExtentInline:
			sum += int64(len(item) - btrfsFileExtentInlineHeaderSize)
		case btrfsFileExtentReg, btrfsFileExtentPrealloc:
			if len(item) < btrfsFileExtentRegularSize {
				return 0, 0, fmt.Errorf("btrfs file extent item %d is %d bytes", i, len(item))
			}
			bytenr := binary.LittleEndian.Uint64(item[21:])
			if bytenr == 0 {
				continue // hole
			}
			if _, dup := seen[bytenr]; dup {
				continue
			}
			seen[bytenr] = struct{}{}
			sum += int64(binary.LittleEndian.Uint64(item[29:]))
		}
	}
	return sum, last, nil
}
