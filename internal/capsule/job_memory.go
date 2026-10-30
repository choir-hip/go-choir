package capsule

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// Job memory (docs/archive/reports/memory-hotplug-controller-design-2026-10-14.md
// section 9). A job may declare memory it needs beyond its capsule's size.
// The executor admits that need against the capsule budget, which is read
// live (3/4 of the guest's current MemTotal, so a memory lease that plugged
// memory raises it), and raises the capsule's cgroup memory.max by it for
// the job. At the job's end the cgroup is lowered before the need leaves
// the lease; a cgroup whose usage cannot come back under its base keeps the
// boost until the capsule ends, because lowering memory.max under usage
// makes the kernel OOM-kill inside the capsule (its broker included).

// LiveMemoryBudget returns the capsule budget reader: 3/4 of MemTotal from
// /proc/meminfo at each call, or 0 when it cannot be read (the executor
// then keeps its boot budget).
func LiveMemoryBudget(readFile func(string) ([]byte, error)) func() int64 {
	if readFile == nil {
		readFile = os.ReadFile
	}
	return func() int64 {
		raw, err := readFile("/proc/meminfo")
		if err != nil {
			return 0
		}
		return MemTotalBytes(string(raw)) * 3 / 4
	}
}

// MemTotalBytes reads MemTotal from /proc/meminfo text, or 0.
func MemTotalBytes(meminfo string) int64 {
	for _, line := range strings.Split(meminfo, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "MemTotal:" {
			kb, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil || kb <= 0 {
				return 0
			}
			return kb * 1024
		}
	}
	return 0
}

// cgroupMemory is a capsule cgroup's memory controls.
type cgroupMemory interface {
	Current() (int64, error)
	Reclaim(bytes int64) error
	SetMax(bytes int64) error
}

// cgroupMemoryDir is cgroupMemory over a cgroup v2 directory.
type cgroupMemoryDir string

func (d cgroupMemoryDir) Current() (int64, error) {
	raw, err := os.ReadFile(filepath.Join(string(d), "memory.current"))
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
}

// Reclaim asks the kernel to reclaim bytes from the cgroup (memory.reclaim,
// Linux 5.19+). It fails with EAGAIN when it could not reclaim all of it;
// the caller reads memory.current afterwards either way.
func (d cgroupMemoryDir) Reclaim(bytes int64) error {
	return writeCgroupFile(filepath.Join(string(d), "memory.reclaim"), strconv.FormatInt(bytes, 10))
}

func (d cgroupMemoryDir) SetMax(bytes int64) error {
	return writeCgroupFile(filepath.Join(string(d), "memory.max"), strconv.FormatInt(bytes, 10))
}

// writeCgroupFile writes one value to a cgroup control file with a raw
// descriptor, so an EAGAIN (memory.reclaim's partial reclaim) comes back as an
// error. os.WriteFile registers the pollable kernfs file with the runtime
// poller and parks on EAGAIN until the file is writable, which a control file
// never reports: a partial reclaim hung teardown, under the executor lock,
// forever (engineering-completion-path-cluster-2026-10-20.md, failure 3).
func writeCgroupFile(path, value string) error {
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

// lowerCgroupMemory brings memory.max down to limit only when usage is (or
// can be reclaimed to) at most limit, so the kernel never OOM-kills inside
// the capsule for it. lowered is false when usage stays above limit.
func lowerCgroupMemory(cg cgroupMemory, limit int64) (lowered bool, err error) {
	if cg == nil {
		return false, fmt.Errorf("capsule cgroup memory controls are unavailable")
	}
	current, err := cg.Current()
	if err != nil {
		return false, err
	}
	if current > limit {
		_ = cg.Reclaim(current - limit) // EAGAIN when short; re-read below
		if current, err = cg.Current(); err != nil {
			return false, err
		}
		if current > limit {
			return false, nil
		}
	}
	if err := cg.SetMax(limit); err != nil {
		return false, err
	}
	return true, nil
}

// jobMemoryBoost is one job's raise of its capsule's memory: counted in the
// executor's budget and the cgroup until returned; release ends the job's
// share of the memory lease.
type jobMemoryBoost struct {
	bytes   int64
	release func()
}
