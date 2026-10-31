//go:build linux

package tapestore

import (
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"
)

// FIEMAP (linux/fiemap.h).
const (
	fsIocFiemap        = 0xC020660B // _IOWR('f', 11, struct fiemap)
	fiemapFlagSync     = 0x1
	fiemapExtentLast   = 0x1
	fiemapExtentShared = 0x2000
	fiemapBatch        = 256
)

type fiemapHeader struct {
	Start, Length                  uint64
	Flags, Mapped, Count, Reserved uint32
}

type fiemapExtent struct {
	Logical, Physical, Length uint64
	Reserved64                [2]uint64
	Flags                     uint32
	Reserved                  [3]uint32
}

type fiemapRequest struct {
	hdr fiemapHeader
	ext [fiemapBatch]fiemapExtent
}

// fileExtents counts the file's extents with FIEMAP and reports whether any
// is shared (a snapshot or reflink also references it). Memory: one 14 KB
// request, reused.
func fileExtents(f *os.File) (int, bool, error) {
	req := new(fiemapRequest)
	var start uint64
	n, shared := 0, false
	for {
		req.hdr = fiemapHeader{Start: start, Length: ^uint64(0) - start, Flags: fiemapFlagSync, Count: fiemapBatch}
		if _, _, e := unix.Syscall(unix.SYS_IOCTL, f.Fd(), fsIocFiemap, uintptr(unsafe.Pointer(req))); e != 0 {
			return -1, false, e
		}
		m := int(req.hdr.Mapped)
		if m == 0 {
			return n, shared, nil
		}
		last := false
		for i := 0; i < m; i++ {
			e := req.ext[i]
			n++
			if e.Flags&fiemapExtentShared != 0 {
				shared = true
			}
			if e.Flags&fiemapExtentLast != 0 {
				last = true
			}
		}
		if last {
			return n, shared, nil
		}
		end := req.ext[m-1].Logical + req.ext[m-1].Length
		if end <= start {
			return n, shared, nil
		}
		start = end
	}
}

// setCompressHint sets the btrfs compression property on a new file, so
// btrfs compresses every chunk that shrinks instead of giving up on the file
// after the first one that does not. Any error (not btrfs, no support) is
// ignored: it is a hint.
func setCompressHint(f *os.File) {
	_ = unix.Fsetxattr(int(f.Fd()), "btrfs.compression", []byte("zstd"), 0)
}

// dropCache tells the kernel the file's pages will not be read again soon,
// so a compaction does not push the paper's working set out of the page
// cache.
func dropCache(f *os.File) {
	_ = unix.Fadvise(int(f.Fd()), 0, 0, unix.FADV_DONTNEED)
}

// lockLowPriorityThread pins the calling goroutine to its OS thread for good
// (the thread exits with the goroutine) and lowers that thread to nice 19 and
// the idle IO class. Errors are ignored: priority is best effort, the pacing
// is the bound.
func lockLowPriorityThread() {
	runtime.LockOSThread()
	tid := unix.Gettid()
	_ = unix.Setpriority(unix.PRIO_PROCESS, tid, 19)
	const ioprioWhoProcess, ioprioClassIdle, ioprioClassShift = 1, 3, 13
	_, _, _ = unix.Syscall(unix.SYS_IOPRIO_SET, ioprioWhoProcess, uintptr(tid), ioprioClassIdle<<ioprioClassShift)
}
