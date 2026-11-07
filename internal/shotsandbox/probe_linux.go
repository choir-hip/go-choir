//go:build linux

package shotsandbox

import (
	"golang.org/x/sys/unix"
)

// platformProbe answers a test probe that needs Linux: "socket:<family>"
// reports whether the seccomp filter lets the stage open such a socket.
func platformProbe(kind, arg string) (Result, bool) {
	if kind != "socket" {
		return Result{}, false
	}
	family := map[string]int{"netlink": unix.AF_NETLINK, "packet": unix.AF_PACKET, "inet": unix.AF_INET, "unix": unix.AF_UNIX}[arg]
	fd, err := unix.Socket(family, unix.SOCK_DGRAM, 0)
	if err != nil {
		if family == unix.AF_NETLINK {
			// SOCK_DGRAM on netlink needs a protocol; a refusal is what is asked.
			fd, err = unix.Socket(family, unix.SOCK_RAW, unix.NETLINK_ROUTE)
		}
		if err != nil {
			return Result{Probe: "refused: " + err.Error()}, true
		}
	}
	_ = unix.Close(fd)
	return Result{Probe: "opened"}, true
}
