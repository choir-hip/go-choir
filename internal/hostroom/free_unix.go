//go:build unix

package hostroom

import "syscall"

// FreeBytes is the space an unprivileged writer can use on the volume that
// holds path.
func FreeBytes(path string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil
}
