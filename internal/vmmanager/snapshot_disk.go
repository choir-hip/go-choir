package vmmanager

// On-disk size of snapshot files (design note items 39-43). Node B's state
// filesystem is btrfs with compress=zstd:3: an 8 GiB guest memory file uses
// about 6% of that on disk. st_blocks on btrfs counts the uncompressed size
// of allocated extents, so it does not see compression.
//
// Three measures, best first (snapshot_disk_linux.go):
//   - btrfs_extents: the file's extent items read with
//     BTRFS_IOC_TREE_SEARCH, as compsize does: each distinct extent's
//     disk_num_bytes counts once, inline extents their inline length, holes
//     zero. Exact, but the ioctl needs CAP_SYS_ADMIN, which vmctl's unit
//     does not have (its bounding set is CAP_NET_ADMIN CAP_SYS_PTRACE;
//     docs/problems/snapshot-budget-falls-back-to-logical-bytes-2026-10-14.md).
//   - fiemap_bound: FIEMAP, which needs no privilege. It reports logical
//     lengths and flags compressed extents ENCODED without their compressed
//     length. Extents never overlap on disk, so an encoded extent occupies at
//     most the distance from its start to the next higher start among the
//     file's own extents, and at most its logical length. Exact when the
//     file's extents are packed together; an overcount (the safe direction)
//     when other files' extents or free space lie between them. Read-only
//     comparison on Node B, 2026-10-14: vmlinux st_blocks 397 MB, bound
//     323 MB, truth 212 MB; storedisk.erofs 2,881 / 2,856 / 2,799 MB.
//   - st_blocks*512: off btrfs, or when both fail.
//
// Like st_blocks, neither unprivileged measure sees the unreferenced tail of
// a partially overwritten extent; snapshot files are written once.

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
)

const (
	// diskBytesMethodBtrfs: summed from the file's btrfs extent items.
	diskBytesMethodBtrfs = "btrfs_extents"
	// diskBytesMethodFiemapBound: allocated extents from FIEMAP, compressed
	// extents bounded by their physical spacing (no privilege needed).
	diskBytesMethodFiemapBound = "fiemap_bound"
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

// FIEMAP layout and flags (include/uapi/linux/fiemap.h).
const (
	// fm_start, fm_length (u64); fm_flags, fm_mapped_extents,
	// fm_extent_count, fm_reserved (u32).
	fiemapHeaderSize = 32
	// fe_logical, fe_physical, fe_length, fe_reserved64[2] (u64); fe_flags,
	// fe_reserved[3] (u32).
	fiemapExtentSize = 56

	fiemapExtentLast       = 0x00000001
	fiemapExtentUnknown    = 0x00000002
	fiemapExtentDelalloc   = 0x00000004
	fiemapExtentEncoded    = 0x00000008
	fiemapExtentNotAligned = 0x00000100
	fiemapExtentDataInline = 0x00000200
	fiemapExtentShared     = 0x00002000

	// fiemapNoStart marks extents whose physical start is not a disk
	// position (inline in metadata, not yet allocated, or unknown).
	fiemapNoStart = fiemapExtentDataInline | fiemapExtentDelalloc | fiemapExtentUnknown

	// fiemapMaxBatches bounds the walk; one batch holds thousands of
	// extents, and a 16 GiB memory file has 131,072 compressed ones.
	fiemapMaxBatches = 1 << 16
)

// fiemapExtent is one FIEMAP extent record.
type fiemapExtent struct {
	Logical  uint64
	Physical uint64
	Length   uint64
	Flags    uint32
}

// parseFiemapExtents reads the extents of one FS_IOC_FIEMAP result buffer
// (the header, then fm_mapped_extents records, native byte order).
func parseFiemapExtents(buf []byte) ([]fiemapExtent, error) {
	if len(buf) < fiemapHeaderSize {
		return nil, fmt.Errorf("fiemap buffer is %d bytes", len(buf))
	}
	mapped := int(binary.NativeEndian.Uint32(buf[20:]))
	if fiemapHeaderSize+mapped*fiemapExtentSize > len(buf) {
		return nil, fmt.Errorf("fiemap reports %d extents in a %d-byte buffer", mapped, len(buf))
	}
	exts := make([]fiemapExtent, 0, mapped)
	for i := 0; i < mapped; i++ {
		o := fiemapHeaderSize + i*fiemapExtentSize
		exts = append(exts, fiemapExtent{
			Logical:  binary.NativeEndian.Uint64(buf[o:]),
			Physical: binary.NativeEndian.Uint64(buf[o+8:]),
			Length:   binary.NativeEndian.Uint64(buf[o+16:]),
			Flags:    binary.NativeEndian.Uint32(buf[o+40:]),
		})
	}
	return exts, nil
}

// collectFiemapExtents walks a file's extents in batches. fetch returns the
// extents at or after a logical offset. The walk ends at an extent flagged
// LAST or at an empty batch, and fails if a batch does not advance.
func collectFiemapExtents(fetch func(start uint64) ([]fiemapExtent, error)) ([]fiemapExtent, error) {
	var all []fiemapExtent
	start := uint64(0)
	for batch := 0; batch < fiemapMaxBatches; batch++ {
		exts, err := fetch(start)
		if err != nil {
			return nil, err
		}
		if len(exts) == 0 {
			return all, nil
		}
		all = append(all, exts...)
		last := exts[len(exts)-1]
		if last.Flags&fiemapExtentLast != 0 {
			return all, nil
		}
		next := last.Logical + last.Length
		if next <= start {
			return nil, fmt.Errorf("fiemap walk does not advance at offset %d", start)
		}
		start = next
	}
	return nil, fmt.Errorf("fiemap walk exceeded %d batches", fiemapMaxBatches)
}

// fiemapBoundBytes is an upper bound of the disk bytes of a file's extents.
// Uncompressed, inline and unallocated extents count their length. Each
// distinct compressed (ENCODED) extent counts the distance from its physical
// start to the next higher start among the file's extents, and never more
// than its logical length: extents do not overlap on disk, so that distance
// holds the whole extent.
func fiemapBoundBytes(exts []fiemapExtent) int64 {
	starts := make([]uint64, 0, len(exts))
	for _, e := range exts {
		if e.Flags&fiemapNoStart == 0 && e.Physical != 0 {
			starts = append(starts, e.Physical)
		}
	}
	sort.Slice(starts, func(i, j int) bool { return starts[i] < starts[j] })
	seen := make(map[uint64]struct{}, len(starts))
	var total uint64
	for _, e := range exts {
		if e.Flags&fiemapNoStart != 0 || e.Physical == 0 {
			total += e.Length
			continue
		}
		if _, dup := seen[e.Physical]; dup {
			continue
		}
		seen[e.Physical] = struct{}{}
		size := e.Length
		if e.Flags&fiemapExtentEncoded != 0 {
			i := sort.Search(len(starts), func(i int) bool { return starts[i] > e.Physical })
			if i < len(starts) && starts[i]-e.Physical < size {
				size = starts[i] - e.Physical
			}
		}
		total += size
	}
	return int64(total)
}

// clampFiemapBound keeps the bound at or below st_blocks, the measure it
// replaces: a bound above it means the extent map and the inode disagree,
// and the old figure stands.
func clampFiemapBound(bound, stBlocks int64) int64 {
	if bound > stBlocks {
		return stBlocks
	}
	return bound
}

var diskMeasureNotes sync.Map

func resetDiskMeasureNotes() { diskMeasureNotes.Clear() }

// noteDiskMeasureFallback logs, once per process and reason, that a measure
// could not run and which one is used instead. Every snapshot hibernate also
// logs the method it recorded (disk_bytes_method).
func noteDiskMeasureFallback(failed, using string, err error) {
	reason := "no extents reported for a non-empty file"
	if err != nil {
		reason = err.Error()
	}
	key := failed + "|" + using + "|" + reason
	if _, loaded := diskMeasureNotes.LoadOrStore(key, struct{}{}); loaded {
		return
	}
	hint := ""
	if failed == diskBytesMethodBtrfs && errors.Is(err, syscall.EPERM) {
		hint = " (BTRFS_IOC_TREE_SEARCH needs CAP_SYS_ADMIN, which is outside vmctl's capability bounding set)"
	}
	log.Printf("vmmanager: snapshot disk measure %s unavailable: %s%s; using %s", failed, reason, hint, using)
}
