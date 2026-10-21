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

// measureFileDiskBytes measures a btrfs file by its extent items
// (compression-aware; needs CAP_SYS_ADMIN, which vmctl's unit has held
// since the jailer change 10e9f18e1, so this exact method is expected on
// Node B: RES-14 is the staging reading of disk_bytes_method that confirms
// it), else by the unprivileged FIEMAP bound, and falls back to
// st_blocks*512 on any other filesystem or when both fail. Each fallback
// reason is logged once (snapshot_disk.go).
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
	if err == nil && !(n == 0 && info.Size() > 0) {
		return n, diskBytesMethodBtrfs, nil
	}
	// The extent search failed (EPERM without CAP_SYS_ADMIN), or read no
	// extents for a non-empty file.
	noteDiskMeasureFallback(diskBytesMethodBtrfs, diskBytesMethodFiemapBound, err)
	exts, ferr := fiemapFileExtents(int(f.Fd()))
	if ferr == nil {
		bound := fiemapBoundBytes(exts)
		if !(bound == 0 && fallback > 0) {
			return clampFiemapBound(bound, fallback), diskBytesMethodFiemapBound, nil
		}
	}
	// FIEMAP failed, or reported no extents for allocated data: trust
	// st_blocks (zero for a fully sparse file, the uncompressed size
	// otherwise).
	noteDiskMeasureFallback(diskBytesMethodFiemapBound, diskBytesMethodStBlocks, ferr)
	return fallback, diskBytesMethodStBlocks, nil
}

// fsIocFiemap is _IOWR('f', 11, struct fiemap); struct fiemap is 32 bytes.
const (
	fsIocFiemap        = 0xC020660B
	fiemapFlagSync     = 0x00000001
	fiemapBatchExtents = 4096
)

// fiemapFileExtents reads every extent of the open file with FS_IOC_FIEMAP
// (no privilege needed), syncing delayed allocation first.
func fiemapFileExtents(fd int) ([]fiemapExtent, error) {
	buf := make([]byte, fiemapHeaderSize+fiemapBatchExtents*fiemapExtentSize)
	ne := binary.NativeEndian
	return collectFiemapExtents(func(start uint64) ([]fiemapExtent, error) {
		clear(buf)
		ne.PutUint64(buf[0:], start)          // fm_start
		ne.PutUint64(buf[8:], math.MaxUint64) // fm_length: to the end
		ne.PutUint32(buf[16:], fiemapFlagSync)
		ne.PutUint32(buf[24:], fiemapBatchExtents) // fm_extent_count
		if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), fsIocFiemap, uintptr(unsafe.Pointer(&buf[0]))); errno != 0 {
			return nil, errno
		}
		return parseFiemapExtents(buf)
	})
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
