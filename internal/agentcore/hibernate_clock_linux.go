//go:build linux

package agentcore

import (
	"syscall"
	"time"
)

// stepSystemClock sets CLOCK_REALTIME. The guest service runs as root, so a
// resumed guest can step its own clock to the host's wall time.
func stepSystemClock(t time.Time) error {
	tv := syscall.NsecToTimeval(t.UnixNano())
	return hibernateClockError(syscall.Settimeofday(&tv))
}

// syncAllFilesystems flushes dirty pages to the disks the snapshot leaves
// behind.
func syncAllFilesystems() { syscall.Sync() }
