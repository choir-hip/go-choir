//go:build !linux

package agentcore

import "time"

// stepSystemClock is unsupported off Linux; guests only run on Linux.
func stepSystemClock(time.Time) error { return errHibernateClockUnsupported }

// syncAllFilesystems is a no-op off Linux.
func syncAllFilesystems() {}
