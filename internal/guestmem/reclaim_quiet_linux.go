//go:build linux

package guestmem

import (
	"golang.org/x/sys/unix"
)

// quietNice is the CPU priority of the act's thread.
const quietNice = 19

// quietThread sets the calling OS thread to nice 19. The caller has locked
// its goroutine to the thread and lets the goroutine end with it, so the
// priority is never restored. Nice affects CPU only; the memory-side cost of
// the act is bounded by its target and step size. A failure is ignored: the
// act is then merely not quiet.
func quietThread() {
	_ = unix.Setpriority(unix.PRIO_PROCESS, unix.Gettid(), quietNice)
}
