package vmmanager

import (
	"strconv"
	"strings"
)

// virtio-blk discard for the guest data drive
// (docs/problems/guest-data-image-never-trims-2026-10-20.md).
//
// data.img is a sparse file on btrfs. The host allocates a block when the
// guest first writes it and keeps it until the file is hole-punched, so a
// guest that never trims makes the host allocation track every block Dolt
// ever wrote. Firecracker 1.17.0 added opt-in VIRTIO_BLK_F_DISCARD for
// writable drives on the Sync io_engine (drive field "discard"; a regular
// backing file is hole-punched). The guest's fstrim timer
// (nix/autoputer-vm.nix, choir-data-fstrim) is what issues the trims.
//
// Firecracker 1.16.x, the host's pinned line (flake.nix
// choirFirecrackerRelease, kept below 1.17), has no such field and denies
// unknown fields, so the drive option
// is gated twice: VM_DATA_DISCARD must be on, and the Firecracker binary must
// report 1.17.0 or newer. Off, the data drive is byte-for-byte what it was.

// First Firecracker release with drive discard.
const (
	firecrackerDiscardMajor = 1
	firecrackerDiscardMinor = 17
)

// firecrackerSupportsBlockDiscard reports whether a `firecracker --version`
// banner ("Firecracker v1.17.0") names a release with virtio-blk discard.
// Anything it cannot parse is "no", so a doubt leaves the drive unchanged.
func firecrackerSupportsBlockDiscard(banner string) bool {
	fields := strings.Fields(banner)
	if len(fields) == 0 {
		return false
	}
	ver := strings.TrimPrefix(fields[len(fields)-1], "v")
	parts := strings.SplitN(ver, ".", 3)
	if len(parts) != 3 {
		return false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return false
	}
	// The patch component may carry a suffix ("0-dev"); it must start with a
	// digit but its value does not matter for the gate.
	if parts[2] == "" || parts[2][0] < '0' || parts[2][0] > '9' {
		return false
	}
	if major != firecrackerDiscardMajor {
		return major > firecrackerDiscardMajor
	}
	return minor >= firecrackerDiscardMinor
}

// dataDiscardEnabledFromEnv resolves the gate for LoadConfigFromEnv: the
// operator flag AND a Firecracker that knows the field.
func dataDiscardEnabledFromEnv(flagOn bool, firecrackerBin string) bool {
	if !flagOn {
		return false
	}
	return firecrackerSupportsBlockDiscard(firecrackerBuildVersion(firecrackerBin))
}

// applyDataDriveDiscard adds the discard options to the data drive only.
// io_engine is set to Sync explicitly: Async (io_uring) does not support
// discard, and a future default change must not silently break it.
func applyDataDriveDiscard(on bool, drive map[string]interface{}) {
	if !on {
		return
	}
	drive["io_engine"] = "Sync"
	drive["discard"] = true
}
