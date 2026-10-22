package server

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/unix"
)

// pageCacheChunk bounds each mapping (and its mincore vector) so a large
// file never needs one huge mapping.
const pageCacheChunk = 256 << 20

type fileResidencyResult struct {
	dev, ino       uint64
	uid            uint32
	size, resident int64
}

func pageCacheSupported() error { return nil }

// pageCacheMayMeasure says whether mincore tells this process the truth
// about a file owned by uid. The kernel's rule (can_do_mincore) also admits
// files the caller could open for writing, which a read-only store never is,
// so ownership or CAP_FOWNER is the test; anything else is unmeasured.
var pageCacheMayMeasure = func(uid uint32) bool {
	return int(uid) == os.Geteuid() || hasCapFowner()
}

var hasCapFowner = sync.OnceValue(func() bool {
	status, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(status), "\n") {
		if hex, ok := strings.CutPrefix(line, "CapEff:"); ok {
			caps, err := strconv.ParseUint(strings.TrimSpace(hex), 16, 64)
			return err == nil && caps&(1<<unix.CAP_FOWNER) != 0
		}
	}
	return false
})

func fileDevice(path string) (uint64, bool) {
	var st unix.Stat_t
	if err := unix.Stat(path, &st); err != nil {
		return 0, false
	}
	return uint64(st.Dev), true
}

// fileResidency maps the file read-only and asks mincore which pages are in
// the page cache. mmap and mincore never fault pages in, so the read leaves
// residency as it found it. O_NOATIME keeps atime unchanged where allowed.
func fileResidency(path string) (fileResidencyResult, bool) {
	f, err := os.OpenFile(path, os.O_RDONLY|unix.O_NOATIME|unix.O_NOFOLLOW, 0)
	if err != nil {
		f, err = os.OpenFile(path, os.O_RDONLY|unix.O_NOFOLLOW, 0)
		if err != nil {
			return fileResidencyResult{}, false
		}
	}
	defer f.Close()
	var st unix.Stat_t
	if err := unix.Fstat(int(f.Fd()), &st); err != nil || st.Mode&unix.S_IFMT != unix.S_IFREG {
		return fileResidencyResult{}, false
	}
	out := fileResidencyResult{dev: uint64(st.Dev), ino: st.Ino, uid: st.Uid, size: st.Size}
	page := int64(os.Getpagesize())
	for off := int64(0); off < st.Size; off += pageCacheChunk {
		n := st.Size - off
		if n > pageCacheChunk {
			n = pageCacheChunk
		}
		mem, err := unix.Mmap(int(f.Fd()), off, int(n), unix.PROT_READ, unix.MAP_SHARED)
		if err != nil {
			return out, false
		}
		vec := make([]byte, (n+page-1)/page)
		_, _, errno := unix.Syscall(unix.SYS_MINCORE, uintptr(unsafe.Pointer(&mem[0])), uintptr(len(mem)), uintptr(unsafe.Pointer(&vec[0])))
		_ = unix.Munmap(mem)
		if errno != 0 {
			return out, false
		}
		for i, v := range vec {
			if v&1 == 0 {
				continue
			}
			// The last page counts only the bytes inside the file.
			out.resident += min(page, n-int64(i)*page)
		}
	}
	return out, true
}
