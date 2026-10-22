package server

import (
	"io/fs"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// pageCacheChunk bounds each mapping (and its mincore vector) so a large
// file never needs one huge mapping.
const pageCacheChunk = 256 << 20

// fileID is a file's identity from lstat, taken before anything is opened
// or mapped, so dedup and the byte cap apply first.
type fileID struct {
	dev, ino uint64
	uid      uint32
	size     int64
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

func fileIdentity(d fs.DirEntry) (fileID, bool) {
	info, err := d.Info() // lstat
	if err != nil {
		return fileID{}, false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() {
		return fileID{}, false
	}
	return fileID{dev: uint64(st.Dev), ino: st.Ino, uid: st.Uid, size: st.Size}, true
}

// fileResidency maps the file read-only and asks mincore which pages are in
// the page cache. mmap and mincore never fault pages in, so the read leaves
// residency as it found it. O_NOATIME keeps atime unchanged where allowed.
// The opened file must still be the inode the walk saw (a swapped path is
// a failure), and the deadline is checked before every chunk.
func fileResidency(path string, want fileID, deadline time.Time) (int64, int) {
	f, err := os.OpenFile(path, os.O_RDONLY|unix.O_NOATIME|unix.O_NOFOLLOW, 0)
	if err != nil {
		f, err = os.OpenFile(path, os.O_RDONLY|unix.O_NOFOLLOW, 0)
		if err != nil {
			return 0, residencyFailed
		}
	}
	defer f.Close()
	var st unix.Stat_t
	if err := unix.Fstat(int(f.Fd()), &st); err != nil || st.Mode&unix.S_IFMT != unix.S_IFREG ||
		uint64(st.Dev) != want.dev || st.Ino != want.ino || st.Size != want.size {
		return 0, residencyFailed
	}
	page := int64(os.Getpagesize())
	var resident int64
	for off := int64(0); off < st.Size; off += pageCacheChunk {
		if time.Now().After(deadline) {
			return 0, residencyDeadline
		}
		n := st.Size - off
		if n > pageCacheChunk {
			n = pageCacheChunk
		}
		mem, err := unix.Mmap(int(f.Fd()), off, int(n), unix.PROT_READ, unix.MAP_SHARED)
		if err != nil {
			return 0, residencyFailed
		}
		vec := make([]byte, (n+page-1)/page)
		_, _, errno := unix.Syscall(unix.SYS_MINCORE, uintptr(unsafe.Pointer(&mem[0])), uintptr(len(mem)), uintptr(unsafe.Pointer(&vec[0])))
		_ = unix.Munmap(mem)
		if errno != 0 {
			return 0, residencyFailed
		}
		for i, v := range vec {
			if v&1 == 0 {
				continue
			}
			// The last page counts only the bytes inside the file.
			resident += min(page, n-int64(i)*page)
		}
	}
	return resident, residencyOK
}
