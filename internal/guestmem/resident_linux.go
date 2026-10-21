//go:build linux

package guestmem

import (
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// residentBytes maps the file read-only, one chunk at a time, and asks
// mincore which of each chunk's pages are in the page cache. mincore reads
// residency without touching a page, so the probe faults nothing in; each
// mapping is released before the next. A chunk is taken from the request's
// probe-byte allowance, and the probe stops, reporting complete=false, when
// the allowance or the deadline is spent or the request is cancelled.
func residentBytes(path string, size int64, s *scan) (int64, bool, error) {
	if size <= 0 {
		return 0, true, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, false, err
	}
	defer f.Close()
	page := int64(os.Getpagesize())
	var resident int64
	for off := int64(0); off < size; off += s.chunk {
		length := min(s.chunk, size-off)
		if s.probeLeft < length || s.done() {
			s.probeLeft = 0
			return resident, false, nil
		}
		s.probeLeft -= length
		n, err := residentChunk(f, off, length, page)
		if err != nil {
			return resident, false, err
		}
		resident += n
	}
	return min(resident, (size+page-1)/page*page), true, nil
}

func residentChunk(f *os.File, off, length, page int64) (int64, error) {
	data, err := unix.Mmap(int(f.Fd()), off, int(length), unix.PROT_READ, unix.MAP_SHARED)
	if err != nil {
		return 0, err
	}
	defer unix.Munmap(data)
	vec := make([]byte, (length+page-1)/page)
	// x/sys has no mincore wrapper on every Linux arch, so call it directly.
	if _, _, errno := unix.Syscall(unix.SYS_MINCORE, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)), uintptr(unsafe.Pointer(&vec[0]))); errno != 0 {
		return 0, errno
	}
	var resident int64
	for _, v := range vec {
		if v&1 != 0 {
			resident++
		}
	}
	return resident * page, nil
}
