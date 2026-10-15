//go:build linux

package yaegikernel

import (
	"fmt"
	"os"
	"path/filepath"
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
	return out
}

// landlockABI is the kernel's Landlock ABI version, 0 when it has none.
func landlockABI() int {
	v, err := ll.LandlockGetABIVersion()
	if err != nil || v < 0 {
		return 0
	}
	return v
}
