package vmmanager

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"strings"
	"syscall"
	"testing"
)

// Unprivileged snapshot disk measure
// (docs/problems/snapshot-budget-falls-back-to-logical-bytes-2026-10-14.md).
// vmctl cannot run BTRFS_IOC_TREE_SEARCH (its capability bounding set has no
// CAP_SYS_ADMIN), so every snapshot was counted at st_blocks: its full
// logical size. FIEMAP needs no privilege. It reports each extent's logical
// length and physical start, and flags compressed extents ENCODED, but not
// their compressed length. Extents never overlap on disk, so the distance
// from an encoded extent's start to the next higher start among the file's
// own extents is an upper bound of its on-disk size.
//
// Ways this could fail, written first:
//   - the bound undercounts (unsafe for a budget): a gap is taken to an
//     extent that starts inside the measured one (only possible for two
//     references to one extent, which must count once), or an inline,
//     delalloc or unknown extent (no real physical start) is used as a
//     successor;
//   - a compressed extent with no successor, or with a successor farther
//     away than its logical length, counts more than its logical length;
//   - extents arrive out of physical order (btrfs compresses in parallel
//     workers) and the "next" start is taken in file order, not disk order;
//   - FIEMAP paging loops forever or stops early: a batch without the LAST
//     flag must continue at the end of its final extent, a batch that does
//     not advance must fail, and an empty batch ends;
//   - the parser misreads the kernel layout (32-byte header, 56-byte
//     extents, native byte order);
//   - the measure falls back silently again: the reason (with the errno,
//     and the missing capability for EPERM) must be logged, once per reason,
//     not once per file;
//   - the bound exceeds st_blocks (an inconsistency): the result must never
//     be worse than the st_blocks fallback it replaces.

func TestFiemapBoundContiguousEncodedExtentsAreExact(t *testing.T) {
	// Three 128 KiB compressed extents packed back to back on disk at 4, 8
	// and 12 KiB each, then one uncompressed 64 KiB extent.
	exts := []fiemapExtent{
		{Logical: 0, Physical: 1 << 20, Length: 128 << 10, Flags: fiemapExtentEncoded},
		{Logical: 128 << 10, Physical: 1<<20 + 4<<10, Length: 128 << 10, Flags: fiemapExtentEncoded},
		{Logical: 256 << 10, Physical: 1<<20 + 12<<10, Length: 128 << 10, Flags: fiemapExtentEncoded},
		{Logical: 384 << 10, Physical: 1<<20 + 24<<10, Length: 64 << 10, Flags: fiemapExtentLast},
	}
	if got, want := fiemapBoundBytes(exts), int64(4<<10+8<<10+12<<10+64<<10); got != want {
		t.Fatalf("bound = %d, want %d", got, want)
	}
}

func TestFiemapBoundCapsAtLogicalLengthAndCountsLastExtentWhole(t *testing.T) {
	exts := []fiemapExtent{
		// Successor 10 MiB away (other files in between): capped at 128 KiB.
		{Logical: 0, Physical: 1 << 20, Length: 128 << 10, Flags: fiemapExtentEncoded},
		// Highest physical start, no successor: counted at its logical length.
		{Logical: 128 << 10, Physical: 11 << 20, Length: 96 << 10, Flags: fiemapExtentEncoded | fiemapExtentLast},
	}
	if got, want := fiemapBoundBytes(exts), int64(128<<10+96<<10); got != want {
		t.Fatalf("bound = %d, want %d", got, want)
	}
}

func TestFiemapBoundUsesDiskOrderNotFileOrder(t *testing.T) {
	// File order A, B, C; disk order C, A, B (parallel compression workers).
	exts := []fiemapExtent{
		{Logical: 0, Physical: 1<<20 + 8<<10, Length: 128 << 10, Flags: fiemapExtentEncoded},                      // A: next is B, 4 KiB
		{Logical: 128 << 10, Physical: 1<<20 + 12<<10, Length: 128 << 10, Flags: fiemapExtentEncoded},             // B: last on disk, 128 KiB
		{Logical: 256 << 10, Physical: 1 << 20, Length: 128 << 10, Flags: fiemapExtentEncoded | fiemapExtentLast}, // C: next is A, 8 KiB
	}
	if got, want := fiemapBoundBytes(exts), int64(4<<10+128<<10+8<<10); got != want {
		t.Fatalf("bound = %d, want %d", got, want)
	}
}

func TestFiemapBoundCountsSharedExtentOnce(t *testing.T) {
	// Two file ranges reference one compressed extent (same disk start); a
	// successor 8 KiB later bounds it.
	exts := []fiemapExtent{
		{Logical: 0, Physical: 1 << 20, Length: 64 << 10, Flags: fiemapExtentEncoded},
		{Logical: 64 << 10, Physical: 1 << 20, Length: 64 << 10, Flags: fiemapExtentEncoded | fiemapExtentShared},
		{Logical: 128 << 10, Physical: 1<<20 + 8<<10, Length: 4 << 10, Flags: fiemapExtentLast},
	}
	if got, want := fiemapBoundBytes(exts), int64(8<<10+4<<10); got != want {
		t.Fatalf("bound = %d, want %d", got, want)
	}
}

func TestFiemapBoundInlineAndUnflushedExtentsAreNotSuccessors(t *testing.T) {
	exts := []fiemapExtent{
		{Logical: 0, Physical: 0, Length: 2 << 10, Flags: fiemapExtentDataInline | fiemapExtentNotAligned},
		{Logical: 4 << 10, Physical: 1 << 20, Length: 128 << 10, Flags: fiemapExtentEncoded},
		// A delalloc extent reports no real start; were it a successor, the
		// encoded extent above would be bounded at 1 KiB.
		{Logical: 132 << 10, Physical: 1<<20 + 1<<10, Length: 16 << 10, Flags: fiemapExtentDelalloc | fiemapExtentUnknown | fiemapExtentLast},
	}
	if got, want := fiemapBoundBytes(exts), int64(2<<10+128<<10+16<<10); got != want {
		t.Fatalf("bound = %d, want %d", got, want)
	}
}

func TestFiemapBoundEmptyIsZero(t *testing.T) {
	if got := fiemapBoundBytes(nil); got != 0 {
		t.Fatalf("bound of no extents (a fully sparse file) = %d", got)
	}
}

// fiemapBuffer lays out a FIEMAP result as the kernel does.
func fiemapBuffer(exts []fiemapExtent) []byte {
	buf := make([]byte, fiemapHeaderSize+fiemapExtentSize*len(exts))
	binary.NativeEndian.PutUint32(buf[20:], uint32(len(exts)))
	for i, e := range exts {
		o := fiemapHeaderSize + fiemapExtentSize*i
		binary.NativeEndian.PutUint64(buf[o:], e.Logical)
		binary.NativeEndian.PutUint64(buf[o+8:], e.Physical)
		binary.NativeEndian.PutUint64(buf[o+16:], e.Length)
		binary.NativeEndian.PutUint32(buf[o+40:], e.Flags)
		binary.NativeEndian.PutUint32(buf[o+44:], 0xFFFFFFFF) // reserved: ignored
	}
	return buf
}

func TestParseFiemapExtentsReadsKernelLayout(t *testing.T) {
	want := []fiemapExtent{
		{Logical: 0, Physical: 7 << 30, Length: 128 << 10, Flags: fiemapExtentEncoded},
		{Logical: 128 << 10, Physical: 3 << 30, Length: 4 << 10, Flags: fiemapExtentLast},
	}
	buf := fiemapBuffer(want)
	got, err := parseFiemapExtents(buf)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("parsed %v, want %v", got, want)
	}
	// A count larger than the buffer is a truncated result, not a panic.
	binary.NativeEndian.PutUint32(buf[20:], 3)
	if _, err := parseFiemapExtents(buf); err == nil {
		t.Fatal("truncated buffer parsed without error")
	}
}

func TestCollectFiemapExtentsPages(t *testing.T) {
	pages := map[uint64][]fiemapExtent{
		0:        {{Logical: 0, Physical: 1 << 20, Length: 4 << 10}},
		4 << 10:  {{Logical: 8 << 10, Physical: 2 << 20, Length: 4 << 10}}, // a hole at 4-8 KiB
		12 << 10: {{Logical: 12 << 10, Physical: 3 << 20, Length: 4 << 10, Flags: fiemapExtentLast}},
	}
	var starts []uint64
	got, err := collectFiemapExtents(func(start uint64) ([]fiemapExtent, error) {
		starts = append(starts, start)
		return pages[start], nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || fmt.Sprint(starts) != fmt.Sprint([]uint64{0, 4 << 10, 12 << 10}) {
		t.Fatalf("got %d extents from starts %v", len(got), starts)
	}

	// An empty batch ends the walk.
	got, err = collectFiemapExtents(func(start uint64) ([]fiemapExtent, error) {
		if start == 0 {
			return []fiemapExtent{{Logical: 0, Physical: 1 << 20, Length: 4 << 10}}, nil
		}
		return nil, nil
	})
	if err != nil || len(got) != 1 {
		t.Fatalf("empty batch: %d extents, err %v", len(got), err)
	}

	// A batch that does not advance fails instead of looping.
	calls := 0
	_, err = collectFiemapExtents(func(start uint64) ([]fiemapExtent, error) {
		calls++
		return []fiemapExtent{{Logical: 0, Physical: 1 << 20, Length: 0}}, nil
	})
	if err == nil || calls > 2 {
		t.Fatalf("non-advancing batch: err %v after %d calls", err, calls)
	}

	// An ioctl error is returned.
	if _, err := collectFiemapExtents(func(uint64) ([]fiemapExtent, error) { return nil, syscall.EOPNOTSUPP }); !errors.Is(err, syscall.EOPNOTSUPP) {
		t.Fatalf("ioctl error = %v", err)
	}
}

func TestDiskMeasureFallbackIsLoggedOncePerReason(t *testing.T) {
	var out bytes.Buffer
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(&out)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
		resetDiskMeasureNotes()
	})
	resetDiskMeasureNotes()

	for i := 0; i < 3; i++ {
		noteDiskMeasureFallback(diskBytesMethodBtrfs, diskBytesMethodFiemapBound, syscall.EPERM)
	}
	noteDiskMeasureFallback(diskBytesMethodFiemapBound, diskBytesMethodStBlocks, syscall.EOPNOTSUPP)

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("logged %d lines, want 2:\n%s", len(lines), out.String())
	}
	if !strings.Contains(lines[0], "btrfs_extents") || !strings.Contains(lines[0], "fiemap_bound") ||
		!strings.Contains(lines[0], syscall.EPERM.Error()) || !strings.Contains(lines[0], "CAP_SYS_ADMIN") {
		t.Fatalf("first line does not name the measures, the errno and the capability: %q", lines[0])
	}
	if !strings.Contains(lines[1], "st_blocks") {
		t.Fatalf("second line does not name st_blocks: %q", lines[1])
	}
}

func TestFiemapBoundNeverExceedsStBlocks(t *testing.T) {
	exts := []fiemapExtent{{Logical: 0, Physical: 1 << 20, Length: 1 << 20, Flags: fiemapExtentLast}}
	if got := clampFiemapBound(fiemapBoundBytes(exts), 512<<10); got != 512<<10 {
		t.Fatalf("bound above st_blocks not clamped: %d", got)
	}
	if got := clampFiemapBound(4<<10, 512<<10); got != 4<<10 {
		t.Fatalf("bound below st_blocks changed: %d", got)
	}
}
