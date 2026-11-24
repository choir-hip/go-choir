package datavolume

import "syscall"

// changeTimeNanos is the inode change time in Unix nanoseconds.
func changeTimeNanos(st *syscall.Stat_t) int64 {
	return int64(st.Ctim.Sec)*1e9 + int64(st.Ctim.Nsec)
}
