package platform

import (
	"fmt"

	"github.com/yusefmosiah/go-choir/internal/persistentdisk"
)

// fileCASVolumeUsage reads the free space of the volume that holds the
// artifacts root; a seam for tests.
var fileCASVolumeUsage = persistentdisk.Statfs

// fileCASFreeFloorBytes is the least free space a chunk write leaves on the
// host volume: 5% of it, and never under 8 GiB. The host volume also holds
// PostgreSQL (the tape) and every computer's data image; a snapshot stream
// is 4 MiB chunks without end, and one computer's snapshots must not fill
// the disk the tape is on (the paper's first snapshot sent 31 GB, 2026-10-30).
func fileCASFreeFloorBytes(total uint64) uint64 {
	return max(total/20, 8<<30)
}

// fileCASSpaceRefusal says why a chunk write must wait for space, or "" when
// it may go ahead. It fails closed: a volume it cannot read refuses.
func (s *Service) fileCASSpaceRefusal() string {
	usage, err := fileCASVolumeUsage(s.artifactsRoot)
	if err != nil {
		return "cannot read the host volume's free space (fails closed): " + err.Error()
	}
	if floor := fileCASFreeFloorBytes(usage.TotalBytes); usage.AvailBytes < floor {
		return fmt.Sprintf("the host volume has %d bytes free, under the %d-byte floor that keeps space for the tape; chunk refused until space is freed", usage.AvailBytes, floor)
	}
	return ""
}
