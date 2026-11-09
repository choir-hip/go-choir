//go:build linux

package toolresidency

import (
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// FileResident returns the bytes of the file in the page cache (mincore).
func FileResident(path string, size int64) int64 {
	if size <= 0 {
		return 0
	}
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	m, err := unix.Mmap(int(f.Fd()), 0, int(size), unix.PROT_READ, unix.MAP_SHARED)
	if err != nil {
		return 0
	}
	defer unix.Munmap(m)
	page := int64(os.Getpagesize())
	vec := make([]byte, (size+page-1)/page)
	if _, _, errno := unix.Syscall(unix.SYS_MINCORE, uintptr(unsafe.Pointer(&m[0])), uintptr(len(m)), uintptr(unsafe.Pointer(&vec[0]))); errno != 0 {
		return 0
	}
	var res int64
	for i, b := range vec {
		if b&1 == 0 {
			continue
		}
		n := page
		if rest := size - int64(i)*page; rest < n {
			n = rest
		}
		res += n
	}
	return res
}

// DropFile asks the kernel to drop the file's clean cached pages.
func DropFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	_ = unix.Fadvise(int(f.Fd()), 0, 0, unix.FADV_DONTNEED)
	f.Close()
}

func SyncAll() { unix.Sync() }
