package guestmem

import (
	"os"

	"golang.org/x/sys/unix"
)

// writeControlFile writes one value to a kernel control file with a raw
// descriptor, so an EAGAIN (memory.reclaim's partial reclaim) comes back as
// an error. os.WriteFile registers the pollable kernfs file with the runtime
// poller and parks on EAGAIN until the file is writable, which a control file
// never reports: a partial reclaim hung teardown, under the executor lock,
// forever (engineering-completion-path-cluster-2026-10-20.md, failure 3).
// This is a copy of internal/capsule's writeCgroupFile, kept apart so the
// capsule package is not touched.
func writeControlFile(path, value string) error {
	fd, err := unix.Open(path, unix.O_WRONLY|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if err != nil {
		return &os.PathError{Op: "open", Path: path, Err: err}
	}
	defer unix.Close(fd)
	for {
		_, err = unix.Write(fd, []byte(value))
		if err != unix.EINTR {
			break
		}
	}
	if err != nil {
		return &os.PathError{Op: "write", Path: path, Err: err}
	}
	return nil
}
