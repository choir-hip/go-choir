//go:build unix

package agentcore

import "syscall"

// freeBytesOnVolume is the space an unprivileged writer can use on the volume
// holding dir.
func freeBytesOnVolume(dir string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil
}
