//go:build !linux

package vmctl

import (
	"os"
	"time"
)

// inodeChangeTime has no portable ctime off Linux: it returns fallback (the
// caller passes now), so nothing ages out there. The modification time would
// be the image's last write, older than the rename, and age out early.
func inodeChangeTime(_ os.FileInfo, fallback time.Time) time.Time { return fallback }

// allocatedBytes is the apparent size off Linux.
func allocatedBytes(info os.FileInfo) int64 { return info.Size() }
