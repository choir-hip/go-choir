//go:build linux

package vmmanager

import (
	"encoding/binary"
	"math"
	"os"
	"syscall"
	"unsafe"
)

const (
	btrfsSuperMagic = 0x9123683E
	// btrfsIocTreeSearch is _IOWR(0x94, 17, struct btrfs_ioctl_search_args),
	// a 4096-byte struct: a 104-byte search key and a 3992-byte buffer.
	btrfsIocTreeSearch  = 0xD0009411
	btrfsSearchArgsSize = 4096
	btrfsSearchKeySize  = 104
	btrfsSearchMaxItems = 4096
	btrfsSearchMaxPages = 1 << 20
)

// measureFileDiskBytes reads a btrfs file's extent items (compression-aware;
// needs CAP_SYS_ADMIN, which vmctl has as root) and falls back to
// st_blocks*512 on any other filesystem or on any error.
func measureFileDiskBytes(path string) (int64, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return 0, "", err
	}
	fallback := stBlocksBytes(info)
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() {
		return fallback, diskBytesMethodStBlocks, nil
	}
	var sfs syscall.Statfs_t
	if err := syscall.Fstatfs(int(f.Fd()), &sfs); err != nil || uint32(sfs.Type) != btrfsSuperMagic {
		return fallback, diskBytesMethodStBlocks, nil
	}
	n, err := btrfsFileDiskBytes(int(f.Fd()), uint64(st.Ino))
	if err != nil || (n == 0 && info.Size() > 0) {
		// No extents read for a non-empty file: trust st_blocks (zero for a
		// fully sparse file, the uncompressed size otherwise).
		return fallback, diskBytesMethodStBlocks, nil
	}
	return n, diskBytesMethodBtrfs, nil
}

// btrfsFileDiskBytes pages through the file's EXTENT_DATA items in its own
// subvolume (tree_id 0) and sums their on-disk bytes.
func btrfsFileDiskBytes(fd int, ino uint64) (int64, error) {
	var args [btrfsSearchArgsSize]byte
	ne := binary.NativeEndian
	seen := make(map[uint64]struct{})
	var total int64
	minOffset := uint64(0)
	for page := 0; page < btrfsSearchMaxPages; page++ {
		clear(args[:])
		key := args[:btrfsSearchKeySize]
		ne.PutUint64(key[0:], 0)               // tree_id: the fd's subvolume
		ne.PutUint64(key[8:], ino)             // min_objectid
		ne.PutUint64(key[16:], ino)            // max_objectid
		ne.PutUint64(key[24:], minOffset)      // min_offset
		ne.PutUint64(key[32:], math.MaxUint64) // max_offset
		ne.PutUint64(key[40:], 0)              // min_transid
		ne.PutUint64(key[48:], math.MaxUint64) // max_transid
		ne.PutUint32(key[56:], btrfsExtentDataKey)
		ne.PutUint32(key[60:], btrfsExtentDataKey)
		ne.PutUint32(key[64:], btrfsSearchMaxItems)
		if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), btrfsIocTreeSearch, uintptr(unsafe.Pointer(&args[0]))); errno != 0 {
			return 0, errno
		}
		nr := ne.Uint32(key[64:])
		if nr == 0 {
			return total, nil
		}
		sum, last, err := parseBtrfsExtentItems(args[btrfsSearchKeySize:], nr, ino, seen)
		if err != nil {
			return 0, err
		}
		total += sum
		if last == math.MaxUint64 {
			return total, nil
		}
		minOffset = last + 1
	}
	return total, nil
}
