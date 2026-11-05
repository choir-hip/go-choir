//go:build linux

package yaegikernel

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"

	ll "github.com/landlock-lsm/go-landlock/landlock/syscall"
	"golang.org/x/sys/unix"
)

// runDeskCellProbes acts as an escaped cell would, from inside a confined
// worker (desk_cell_selftest.go). Every probe is harmless when refused and
// only reports when it is not: exec targets do not exist, signals are 0, a
// file written by mistake is removed.
func runDeskCellProbes(allowedRoot, tmp string, reads []string) DeskCellSelfTestProbes {
	out := DeskCellSelfTestProbes{}
	out["write_own_tmp"] = probeErr(os.WriteFile(filepath.Join(tmp, "probe"), []byte("x"), 0o600))
	if allowedRoot == "" {
		out["list_allowed_root"] = "ERROR: no allowed root"
	} else {
		out["list_allowed_root"] = probeErr(probeRead(allowedRoot))
		stray := filepath.Join(allowedRoot, fmt.Sprintf("choir-cell-selftest-%d", os.Getpid()))
		err := os.WriteFile(stray, []byte("x"), 0o600)
		if err == nil {
			_ = os.Remove(stray)
		}
		out["write_allowed_root"] = probeErr(err)
	}
	stray := "/etc/choir-cell-selftest"
	err := os.WriteFile(stray, []byte("x"), 0o600)
	if err == nil {
		_ = os.Remove(stray)
	}
	out["write_etc"] = probeErr(err)
	for _, p := range reads {
		out["read:"+p] = probeErr(probeRead(p))
	}
	socket := func(domain, typ, proto int) error {
		fd, err := unix.Socket(domain, typ, proto)
		if err == nil {
			_ = unix.Close(fd)
		}
		return err
	}
	out["socket_inet_stream"] = probeErr(socket(unix.AF_INET, unix.SOCK_STREAM, 0))
	out["socket_inet_dgram"] = probeErr(socket(unix.AF_INET, unix.SOCK_DGRAM, 0))
	out["socket_inet6_stream"] = probeErr(socket(unix.AF_INET6, unix.SOCK_STREAM, 0))
	out["socket_unix_stream"] = probeErr(socket(unix.AF_UNIX, unix.SOCK_STREAM, 0))
	out["socket_netlink"] = probeErr(socket(unix.AF_NETLINK, unix.SOCK_RAW, unix.NETLINK_ROUTE))
	out["socket_packet_raw"] = probeErr(socket(unix.AF_PACKET, unix.SOCK_RAW, 0))
	out["socket_inet_raw"] = probeErr(socket(unix.AF_INET, unix.SOCK_RAW, unix.IPPROTO_ICMP))
	// A path that does not exist: an unconfined worker fails with ENOENT; a
	// confined one never gets that far (EPERM from the filter).
	_, err = syscall.ForkExec("/nonexistent/choir-cell-selftest", []string{"probe"}, &syscall.ProcAttr{})
	out["fork_exec"] = probeErr(err)
	out["execve"] = probeErr(unix.Exec("/nonexistent/choir-cell-selftest", []string{"probe"}, nil))
	out["kill_pid1"] = probeErr(unix.Kill(1, 0))
	out["kill_parent"] = probeErr(unix.Kill(os.Getppid(), 0))
	out["setuid_root"] = probeErr(unix.Setresuid(0, 0, 0))
	out["unshare_user"] = probeErr(unix.Unshare(unix.CLONE_NEWUSER))
	out["unshare_net"] = probeErr(unix.Unshare(unix.CLONE_NEWNET))
	// MA6.2: the classes a red team named. Each is called with arguments
	// the kernel would refuse (EINVAL, EFAULT or EBADF) if the filter let it
	// through, so a worker that is wrongly unconfined learns nothing and
	// changes nothing; the filter answers EPERM first.
	for name, p := range deskCellClassProbes() {
		out[name] = probeErr(p())
	}
	// The worker's own mount namespace has no /sys or /dev tree, and a
	// /proc reduced to its own pid directories (hidepid=2, subset=pid).
	out["proc_meminfo"] = probeErr(probeRead("/proc/meminfo"))
	out["proc_pid1_environ"] = probeErr(probeRead("/proc/1/environ"))
	out["proc_parent_status"] = probeErr(probeRead(fmt.Sprintf("/proc/%d/status", os.Getppid())))
	out["sys_kernel"] = probeErr(probeRead("/sys/kernel"))
	out["dev_mem"] = probeErr(probeRead("/dev/mem"))
	out["dev_extra_entries"] = probeErr(devExtraEntries())
	return out
}

// deskCellClassProbes are the MA6.2 denied classes, each a call with
// arguments the kernel would refuse (EINVAL, EFAULT or EBADF) if the filter
// let it through, so a wrongly unconfined worker learns nothing and changes
// nothing; the filter answers EPERM first.
func deskCellClassProbes() map[string]func() error {
	sys := func(trap uintptr, a ...uintptr) func() error {
		return func() error {
			var x [6]uintptr
			copy(x[:], a)
			_, _, e := unix.Syscall6(trap, x[0], x[1], x[2], x[3], x[4], x[5])
			if e == 0 {
				return nil
			}
			return e
		}
	}
	return map[string]func() error{
		"io_uring_setup":  sys(unix.SYS_IO_URING_SETUP, 0, 0),
		"keyctl":          sys(unix.SYS_KEYCTL, 0xdead),
		"bpf":             sys(unix.SYS_BPF, 0xdead, 0, 0),
		"perf_event_open": sys(unix.SYS_PERF_EVENT_OPEN, 0, ^uintptr(0), ^uintptr(0), ^uintptr(0), 0),
		"userfaultfd":     sys(unix.SYS_USERFAULTFD, 0xdead0000),
		"ptrace":          sys(unix.SYS_PTRACE, 0xdead, 1),
		// CLONE_THREAD without CLONE_SIGHAND is EINVAL from the kernel; the
		// namespace bit makes the filter refuse it first.
		"clone_newns":   sys(unix.SYS_CLONE, unix.CLONE_THREAD|unix.CLONE_NEWNS),
		"clone_newuser": sys(unix.SYS_CLONE, unix.CLONE_THREAD|unix.CLONE_NEWUSER),
		"mount":         sys(unix.SYS_MOUNT, 0, 0, 0, 0, 0),
	}
}

// deskCellClassProbeNames lists them, sorted.
func deskCellClassProbeNames() []string {
	var names []string
	for n := range deskCellClassProbes() {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// devExtraEntries returns an error (the pass for its probe) unless /dev holds
// something besides the two device files the worker was given.
func devExtraEntries() error {
	entries, err := os.ReadDir("/dev")
	if err != nil {
		return err
	}
	var extra []string
	for _, e := range entries {
		if n := e.Name(); n != "null" && n != "urandom" {
			extra = append(extra, n)
		}
	}
	if len(extra) == 0 {
		return fmt.Errorf("no other entries in /dev")
	}
	return nil // extras are visible: the probe reports success, which fails the judge
}

// landlockABI is the kernel's Landlock ABI version, 0 when it has none.
func landlockABI() int {
	v, err := ll.LandlockGetABIVersion()
	if err != nil || v < 0 {
		return 0
	}
	return v
}
