package vmmanager

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

// Per-guest write bandwidth bound (guest isolation, "a write storm halves a
// co-reader"; docs/definitions/choir-guest-isolation-2026-11-01.ledger.md).
//
// The measured cause is the disk, not the weights. A guest flooding its data
// drive keeps both NVMe members of the state mirror busy with writes, and an
// O_DIRECT reader in another cgroup then waits behind flash program time at
// the device: with iocost off, the reader fell 1757 -> 1150 MB/s (p99 0.7 ->
// 2.3 ms); with iocost on (rlat/wlat anywhere from 3/10 ms to 20/100 ms) it
// fell to about 1000 MB/s, vrate sagging to 40-70 percent. io.weight cannot
// help a latency-bound reader that is not itself saturating a share, and a
// looser or tighter qos only moves which side pays. What is left is the
// write rate itself: io.max wbps on the guest's leaf bounds how fast a guest
// can keep the shared disk in the write state. Writes only: a guest's read
// bursts stay free, and buffered writes still burst into the page cache up to
// the dirty limits before the bound shows.
//
// The bound is written by vmctl into the leaf after the jailer has made it
// (the jailer's --cgroup takes file=value and io.max's value holds an '=').

// DefaultJailerIOWriteMiBps is the sustained write rate one guest's leaf may
// push at the state disk, per device (md mirrors it: a write of X is X on
// each member). 0 omits the bound.
const DefaultJailerIOWriteMiBps = 400

const maxJailerIOWriteMiBps = 1 << 20

var ioMaxDevicePattern = regexp.MustCompile(`^[0-9]{1,4}:[0-9]{1,7}$`)

// ioMaxLine is the io.max line bounding writes on dev ("MAJ:MIN") to mibps.
func ioMaxLine(dev string, mibps int) string {
	return dev + " wbps=" + strconv.FormatInt(int64(mibps)<<20, 10)
}

// blockDevOfMount returns "MAJ:MIN" of the block device backing the mount
// that holds dir, from mountinfo content (the longest mount point that is a
// prefix of dir wins). statRdev resolves a device node to its rdev. "" when
// the mount has no device node (tmpfs, overlay) or cannot be resolved.
func blockDevOfMount(mountinfo *bufio.Scanner, dir string, statRdev func(string) (uint64, bool)) string {
	dir = filepath.Clean(dir)
	best, bestSrc := -1, ""
	for mountinfo.Scan() {
		f := strings.Fields(mountinfo.Text())
		// id parent maj:min root mountpoint opts [optional...] - fstype source superopts
		sep := -1
		for i, s := range f {
			if s == "-" {
				sep = i
				break
			}
		}
		if sep < 6 || len(f) < sep+3 {
			continue
		}
		mp := unescapeMountinfo(f[4])
		if mp != "/" && dir != mp && !strings.HasPrefix(dir, mp+"/") {
			continue
		}
		if len(mp) > best {
			best, bestSrc = len(mp), unescapeMountinfo(f[sep+2])
		}
	}
	if best < 0 || !strings.HasPrefix(bestSrc, "/dev/") {
		return ""
	}
	rdev, ok := statRdev(bestSrc)
	if !ok {
		return ""
	}
	// Linux dev_t split (glibc major()/minor()).
	major := (rdev>>8)&0xfff | (rdev>>32)&^uint64(0xfff)
	minor := rdev&0xff | (rdev>>12)&^uint64(0xff)
	return fmt.Sprintf("%d:%d", major, minor)
}

func statRdevOS(p string) (uint64, bool) {
	var st syscall.Stat_t
	if err := syscall.Stat(p, &st); err != nil || st.Mode&syscall.S_IFMT != syscall.S_IFBLK {
		return 0, false
	}
	return uint64(st.Rdev), true
}

// capDevices turns the state dir's backing device ("MAJ:MIN") into the
// devices io.max must name. A stacked device is not bounded at its own
// queue: a cap on md127 let the bursts through (observer p99 4.5 ms,
// unchanged), a cap on each NVMe member held it to 1.7 ms. So a stacked
// device is replaced by the whole disks under it, resolved through sysfs; a
// plain device (or a partition's whole disk) stands for itself.
func capDevices(sysRoot, dev string) []string {
	var out []string
	seen := map[string]bool{}
	var walk func(name string, depth int)
	walk = func(name string, depth int) {
		dir := filepath.Join(sysRoot, "class", "block", name)
		if depth < 4 {
			if slaves, err := os.ReadDir(filepath.Join(dir, "slaves")); err == nil && len(slaves) > 0 {
				for _, s := range slaves {
					walk(s.Name(), depth+1)
				}
				return
			}
		}
		// A partition is bounded as its whole disk: the queue is the disk's.
		if _, err := os.Stat(filepath.Join(dir, "partition")); err == nil {
			if real, err := filepath.EvalSymlinks(dir); err == nil {
				dir = filepath.Dir(real)
			}
		}
		raw, err := os.ReadFile(filepath.Join(dir, "dev"))
		if err != nil {
			return
		}
		if d := strings.TrimSpace(string(raw)); ioMaxDevicePattern.MatchString(d) && !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	if raw, err := os.ReadFile(filepath.Join(sysRoot, "dev", "block", dev, "uevent")); err == nil {
		for _, l := range strings.Split(string(raw), "\n") {
			if n, ok := strings.CutPrefix(l, "DEVNAME="); ok {
				walk(n, 0)
			}
		}
	}
	return out
}

// stateBlockDevs is the devices the write bound names, resolved once.
func (m *Manager) stateBlockDevs() []string {
	m.ioMaxOnce.Do(func() {
		dev := strings.TrimSpace(m.cfg.Jailer.IOMaxDevice)
		if dev == "" {
			f, err := os.Open("/proc/self/mountinfo")
			if err != nil {
				return
			}
			defer f.Close()
			sc := bufio.NewScanner(f)
			sc.Buffer(make([]byte, 64<<10), 1<<20)
			dev = blockDevOfMount(sc, m.cfg.StateDir, statRdevOS)
		}
		if dev == "" {
			return
		}
		m.ioMaxDevs = capDevices("/sys", dev)
		if len(m.ioMaxDevs) == 0 {
			m.ioMaxDevs = []string{dev}
		}
	})
	return m.ioMaxDevs
}

// applyLeafWriteCap writes the write bound into the guest's leaf. A failure
// is logged and leaves the guest unbounded: a missing bound must not fail a
// launch (the bound is a backstop; the receipt does not depend on it).
func (m *Manager) applyLeafWriteCap(vmID string) {
	j := m.cfg.Jailer
	if j.IOWriteMiBps <= 0 {
		return
	}
	rel := m.cgroupLeafRel(vmID)
	root := m.cgroupRoot
	if rel == "" || root == "" {
		return
	}
	devs := m.stateBlockDevs()
	if len(devs) == 0 {
		log.Printf("vmmanager: no write bound for VM %s: state dir %s has no block device", vmID, m.cfg.StateDir)
		return
	}
	file := filepath.Join(root, filepath.FromSlash(rel), "io.max")
	for _, dev := range devs {
		// One device per write: io.max takes one line at a time.
		if err := os.WriteFile(file, []byte(ioMaxLine(dev, j.IOWriteMiBps)), 0o644); err != nil {
			log.Printf("vmmanager: write bound for VM %s (%s): %v", vmID, file, err)
		}
	}
}
