//go:build linux

package vmbackend

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// OpenTap opens an existing tap device the way Firecracker opens it
// (IFF_TAP|IFF_NO_PI|IFF_VNET_HDR) and returns the handle to pass to a
// backend with Caps().TapFD. vmmanager creates the tap (ip tuntap add, up,
// /30, iptables) first; opening it here needs the same privilege vmctl
// already has, and the VMM then needs none for networking.
//
// IFF_VNET_HDR is required: Cloud Hypervisor sets the vnet header size on the
// fd it is given, and a tap opened without the flag would deliver frames
// without the header the device expects. IFF_MULTI_QUEUE is not set because
// vmmanager creates single-queue taps; the flag must match creation.
func OpenTap(name string) (*os.File, error) {
	if name == "" || len(name) >= unix.IFNAMSIZ {
		return nil, fmt.Errorf("vmbackend: invalid tap name %q", name)
	}
	fd, err := unix.Open("/dev/net/tun", unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("vmbackend: open /dev/net/tun: %w", err)
	}
	ifr, err := unix.NewIfreq(name)
	if err != nil {
		_ = unix.Close(fd)
		return nil, err
	}
	ifr.SetUint16(unix.IFF_TAP | unix.IFF_NO_PI | unix.IFF_VNET_HDR)
	if err := unix.IoctlIfreq(fd, unix.TUNSETIFF, ifr); err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("vmbackend: attach tap %s: %w", name, err)
	}
	return os.NewFile(uintptr(fd), "tap:"+name), nil
}
