//go:build linux

package guestmem

import (
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// residentBytes maps the file read-only and asks mincore which of its pages
// are in the page cache. mincore reads residency without touching a page,
// so the probe faults nothing in; the mapping is always released.
func residentBytes(path string, size int64) (int64, error) {
	if size <= 0 {
		return 0, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	data, err := unix.Mmap(int(f.Fd()), 0, int(size), unix.PROT_READ, unix.MAP_SHARED)
	if err != nil {
		return 0, err
	}
	defer unix.Munmap(data)
	page := int64(os.Getpagesize())
	vec := make([]byte, (size+page-1)/page)
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
