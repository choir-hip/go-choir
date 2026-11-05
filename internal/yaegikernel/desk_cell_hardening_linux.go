//go:build linux

package yaegikernel

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
	"kernel.org/pub/linux/libs/security/libcap/psx"
)

// MA6.2 hardening of the desk cell worker (docs/design/ma62-ma63-hardening-2026-11-05.md).
//
//   - a mount namespace of its own whose root holds only what the worker
//     needs (the allowed root read-only, its own tmp dir, /dev/null,
//     /dev/urandom, the time zone files, and a /proc reduced to the pid
//     directories the worker may see): no /sys, no /dev tree, no /etc. The
//     spawner starts the worker with CLONE_NEWNS and CAP_SYS_ADMIN as an
//     ambient capability; the worker builds the root, pivots into it and
//     drops every capability on every thread before Landlock and seccomp.
//
//     /proc stays, reduced (hidepid=2, subset=pid: no /proc/sys, meminfo or
//     other process's directory), because libpsx, which go-landlock and the
//     capability drop use to act on every thread in a cgo build, lists
//     /proc/<pid>/task after each call and fails the call when it cannot
//     (go-landlock bug 39). The cell's Landlock domain grants it nothing
//     but that directory. Removing /proc outright needs a build without cgo
//     (psx then uses syscall.AllThreadsSyscall) or a two-stage start; the
//     design note names it as the residual;
//   - inherited descriptors other than 0, 1, 2 and the session socket are
//     closed on entry;
//   - resource limits beyond the address-space cap.

// cellMountNSEnv tells the worker its spawner gave it a mount namespace and
// the capability to build the root (after the session prefix).
const cellMountNSEnv = "MOUNTNS"

const (
	capSysAdmin = 21 // CAP_SYS_ADMIN: mount, pivot_root (the spawner grants it ambient)
	// _LINUX_CAPABILITY_VERSION_3 and its two-word data block.
	linuxCapabilityVersion3 = 0x20080522
)

type capHeader struct {
	version uint32
	pid     int32
}

type capData struct {
	effective, permitted, inheritable uint32
}

// isolateCellMounts replaces the worker's mount tree with a minimal one. It
// must run in a private mount namespace (CLONE_NEWNS by the spawner) with
// CAP_SYS_ADMIN. allowedRoot is bound read-only at its own path, tmp
// read-write at its own path, so every path the worker was handed still
// resolves. Anything it cannot build is an error: the worker does not serve
// with a half-built root.
func isolateCellMounts(allowedRoot, tmp string) error {
	if err := syscall.Mount("", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, ""); err != nil {
		return fmt.Errorf("mount namespace: make / private: %w", err)
	}
	newroot := filepath.Join(tmp, ".root")
	if err := os.MkdirAll(newroot, 0o700); err != nil {
		return fmt.Errorf("mount namespace: root dir: %w", err)
	}
	if err := syscall.Mount("choir-cell-root", newroot, "tmpfs", syscall.MS_NOSUID|syscall.MS_NODEV, "size=1m,mode=0755"); err != nil {
		return fmt.Errorf("mount namespace: root tmpfs: %w", err)
	}
	inRoot := func(p string) string { return filepath.Join(newroot, p) }
	bindDir := func(src string, readOnly bool) error {
		dst := inRoot(src)
		if err := os.MkdirAll(dst, 0o755); err != nil {
			return err
		}
		if err := syscall.Mount(src, dst, "", syscall.MS_BIND, ""); err != nil {
			return err
		}
		flags := keptMountFlags(dst) | syscall.MS_REMOUNT | syscall.MS_BIND | syscall.MS_NOSUID | syscall.MS_NODEV | syscall.MS_NOEXEC
		if readOnly {
			flags |= syscall.MS_RDONLY
		}
		return syscall.Mount("", dst, "", flags, "")
	}
	bindFile := func(src string) error {
		if _, err := os.Stat(src); err != nil {
			return nil // optional: Landlock rules for it are IgnoreIfMissing
		}
		dst := inRoot(src)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		_ = f.Close()
		return syscall.Mount(src, dst, "", syscall.MS_BIND, "")
	}
	if allowedRoot != "" && cellCanOpen(allowedRoot) {
		if err := bindDir(allowedRoot, true); err != nil {
			return fmt.Errorf("mount namespace: allowed root: %w", err)
		}
	}
	// The tmp dir holds the new root's mount point; a non-recursive bind
	// does not carry that mount along.
	if err := bindDir(tmp, false); err != nil {
		return fmt.Errorf("mount namespace: tmp dir: %w", err)
	}
	for _, f := range []string{"/dev/null", "/dev/urandom", "/etc/localtime"} {
		if err := bindFile(f); err != nil {
			return fmt.Errorf("mount namespace: %s: %w", f, err)
		}
	}
	if st, err := os.Stat("/etc/zoneinfo"); err == nil && st.IsDir() {
		if err := bindDir("/etc/zoneinfo", true); err != nil {
			return fmt.Errorf("mount namespace: /etc/zoneinfo: %w", err)
		}
	}
	// A reduced /proc (see the header).
	procDst := inRoot("/proc")
	if err := os.MkdirAll(procDst, 0o555); err != nil {
		return fmt.Errorf("mount namespace: /proc dir: %w", err)
	}
	if err := syscall.Mount("proc", procDst, "proc", syscall.MS_NOSUID|syscall.MS_NODEV|syscall.MS_NOEXEC, "hidepid=2,subset=pid"); err != nil {
		return fmt.Errorf("mount namespace: /proc: %w", err)
	}
	old := inRoot(".old")
	if err := os.MkdirAll(old, 0o700); err != nil {
		return fmt.Errorf("mount namespace: old root dir: %w", err)
	}
	// The old root's directory is created on the tmpfs, so it is not part
	// of the bound tmp dir.
	if err := syscall.PivotRoot(newroot, old); err != nil {
		return fmt.Errorf("mount namespace: pivot_root: %w", err)
	}
	if err := syscall.Chdir("/"); err != nil {
		return fmt.Errorf("mount namespace: chdir /: %w", err)
	}
	if err := syscall.Unmount("/.old", syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("mount namespace: unmount old root: %w", err)
	}
	if err := syscall.Rmdir("/.old"); err != nil {
		return fmt.Errorf("mount namespace: remove old root dir: %w", err)
	}
	// The root holds only mount points: seal it.
	if err := syscall.Mount("", "/", "", keptMountFlags("/")|syscall.MS_REMOUNT|syscall.MS_BIND|syscall.MS_RDONLY|syscall.MS_NOSUID|syscall.MS_NODEV|syscall.MS_NOEXEC, ""); err != nil {
		return fmt.Errorf("mount namespace: seal root: %w", err)
	}
	if err := syscall.Chdir(tmp); err != nil {
		return fmt.Errorf("mount namespace: chdir to tmp dir: %w", err)
	}
	return nil
}

// keptMountFlags are the atime flags and the like a remount must repeat: a
// remount that changes them is refused for a mount a user namespace owns, and
// is pointless for one it does not.
func keptMountFlags(path string) uintptr {
	var st unix.Statfs_t
	if unix.Statfs(path, &st) != nil {
		return 0
	}
	var f uintptr
	for _, m := range []struct{ st, ms uintptr }{
		{unix.ST_NOATIME, unix.MS_NOATIME},
		{unix.ST_NODIRATIME, unix.MS_NODIRATIME},
		{unix.ST_RELATIME, unix.MS_RELATIME},
		{unix.ST_NODEV, unix.MS_NODEV},
		{unix.ST_NOSUID, unix.MS_NOSUID},
		{unix.ST_NOEXEC, unix.MS_NOEXEC},
	} {
		if uintptr(st.Flags)&m.st != 0 {
			f |= m.ms
		}
	}
	return f
}

// dropAllCapabilities clears the ambient set and the effective, permitted
// and inheritable sets on every thread (capabilities are per thread), then
// reads them back on this thread. With no_new_privs and no exec the worker
// cannot regain any.
func dropAllCapabilities() error {
	if _, _, e := psx.Syscall6(unix.SYS_PRCTL, unix.PR_CAP_AMBIENT, unix.PR_CAP_AMBIENT_CLEAR_ALL, 0, 0, 0, 0); e != 0 {
		return fmt.Errorf("clear ambient capabilities: %w", e)
	}
	hdr := capHeader{version: linuxCapabilityVersion3}
	var data [2]capData
	if _, _, e := psx.Syscall3(unix.SYS_CAPSET, uintptr(unsafe.Pointer(&hdr)), uintptr(unsafe.Pointer(&data[0])), 0); e != 0 {
		return fmt.Errorf("capset: %w", e)
	}
	hdr = capHeader{version: linuxCapabilityVersion3}
	if _, _, e := syscall.RawSyscall(unix.SYS_CAPGET, uintptr(unsafe.Pointer(&hdr)), uintptr(unsafe.Pointer(&data[0])), 0); e != 0 {
		return fmt.Errorf("capget: %w", e)
	}
	for _, d := range data {
		if d.effective|d.permitted|d.inheritable != 0 {
			return fmt.Errorf("capabilities remain after the drop: %+v", d)
		}
	}
	return nil
}

// CloseInheritedFDs closes every descriptor above the session socket that
// survived execve. The Go runtime opens its own descriptors close-on-exec,
// so one without FD_CLOEXEC came from the spawner's process (or its parent)
// and is not the worker's to hold. Called first thing in the worker, before
// anything of its own is open at those numbers.
func CloseInheritedFDs(keep ...int) {
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return
	}
	keepSet := map[int]bool{0: true, 1: true, 2: true, 3: true}
	for _, k := range keep {
		keepSet[k] = true
	}
	for _, e := range entries {
		fd, err := strconv.Atoi(e.Name())
		if err != nil || keepSet[fd] {
			continue
		}
		flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0)
		if err != nil || flags&unix.FD_CLOEXEC != 0 {
			continue // the directory read's own fd, or the runtime's
		}
		_ = unix.Close(fd)
	}
}

// Worker resource limits (beyond RLIMIT_AS, ApplyWorkerMemoryLimit): no core
// dumps, a small descriptor table, a bounded file size, no locked memory and
// no message queues. Thread counts are bounded by the cgroup's pids.max.
const (
	cellMaxOpenFiles = 256
	cellMaxFileBytes = 256 << 20
)

func applyCellRlimits() error {
	for _, l := range []struct {
		res      int
		cur, max uint64
	}{
		{unix.RLIMIT_CORE, 0, 0},
		{unix.RLIMIT_NOFILE, cellMaxOpenFiles, cellMaxOpenFiles},
		{unix.RLIMIT_FSIZE, cellMaxFileBytes, cellMaxFileBytes},
		{unix.RLIMIT_MEMLOCK, 0, 0},
		{unix.RLIMIT_MSGQUEUE, 0, 0},
	} {
		if err := unix.Setrlimit(l.res, &unix.Rlimit{Cur: l.cur, Max: l.max}); err != nil {
			return fmt.Errorf("setrlimit %d: %w", l.res, err)
		}
	}
	return nil
}

// cellMountNSWanted reports whether the spawner asked for the mount
// namespace step.
func cellMountNSWanted(getenv func(string) string, prefix string) bool {
	return strings.TrimSpace(getenv(prefix+cellMountNSEnv)) == "1"
}
