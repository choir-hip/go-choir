// The guest half of memory return, joined on the host-only procmem read
// (docs/problems/guest-damon-reclaim-state-unreported-2026-10-22.md). Host
// RSS of a computer falls only when the guest frees pages (DAMON_RECLAIM for
// cold page cache, RES-06) and free page reporting hands whole free blocks
// to the host through the balloon. Without these fields a flat host RSS
// cannot say which step stopped. Read only: kernel parameter files, the
// balloon driver's negotiated features, selected /proc/vmstat counters and
// /proc/buddyinfo. Nothing here writes a parameter. A file the kernel does
// not have (an older kernel, or the option off) is absent, never an error.

package server

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// MemoryReturn is the guest's view of the path from freed pages to the host.
type MemoryReturn struct {
	DamonReclaim  DamonReclaim     `json:"damon_reclaim"`
	PageReporting PageReporting    `json:"page_reporting"`
	Balloon       BalloonDriver    `json:"balloon"`
	Vmstat        map[string]int64 `json:"vmstat,omitempty"`
	FreeBlocks    *FreeBlocks      `json:"free_blocks,omitempty"`
	// Errors names files that exist but could not be read or parsed.
	Errors []string `json:"errors,omitempty"`
}

// DamonReclaim is /sys/module/damon_reclaim/parameters. Present is false on
// a kernel without CONFIG_DAMON_RECLAIM.
type DamonReclaim struct {
	Present    bool   `json:"present"`
	Enabled    *bool  `json:"enabled,omitempty"`
	KdamondPID *int64 `json:"kdamond_pid,omitempty"` // -1: not running
	// KdamondCPUTicks is the kernel thread's utime+stime in USER_HZ ticks
	// (100 per second on Linux), read only when the pid's comm is kdamond.*.
	KdamondCPUTicks *int64 `json:"kdamond_cpu_ticks,omitempty"`
	// Stats are the nr_* and bytes_* counters this kernel has
	// (bytes_reclaimed_regions, nr_quota_exceeds, ...).
	Stats map[string]int64 `json:"stats,omitempty"`
	// Parameters are the remaining inputs, raw (min_age in microseconds,
	// wmarks_* in permille of free memory, skip_anon, quotas).
	Parameters map[string]string `json:"parameters,omitempty"`
}

// PageReporting is free page reporting's one parameter: free blocks of at
// least this order (in pages, as a power of two) are reported to the host.
// The kernel holds -1 until a reporting device registers and sets it, but
// the page_reporting.page_reporting_order boot argument sets it with no
// device, so a non-negative order proves nothing. Registered is true only
// when the order is set and the virtio balloon is bound with
// VIRTIO_BALLOON_F_REPORTING negotiated: virtio_balloon's probe registers
// page reporting when that feature is negotiated and fails (leaving no
// bound device) when registration fails. The balloon is the only reporting
// device a Firecracker guest has.
type PageReporting struct {
	Present    bool   `json:"present"`
	Order      *int64 `json:"order,omitempty"`
	Registered bool   `json:"registered"`
}

// BalloonDriver is the virtio balloon bound in the guest and the features it
// negotiated with the device. A feature is omitted when the features string
// is too short to hold its bit.
type BalloonDriver struct {
	Bound        bool   `json:"bound"`
	Device       string `json:"device,omitempty"`
	StatsVQ      *bool  `json:"stats_vq,omitempty"`       // bit 1
	DeflateOnOOM *bool  `json:"deflate_on_oom,omitempty"` // bit 2
	FreePageHint *bool  `json:"free_page_hint,omitempty"` // bit 3
	Reporting    *bool  `json:"reporting,omitempty"`      // bit 5, VIRTIO_BALLOON_F_REPORTING
}

// FreeBlocks sums /proc/buddyinfo over zones: free memory by block order, in
// KiB. ReportableKiB is the part in blocks of at least the reporting order:
// the only free memory free page reporting can hand back.
type FreeBlocks struct {
	KiBByOrder    []int64 `json:"kib_by_order"`
	ReportableKiB *int64  `json:"reportable_kib,omitempty"`
}

// memoryReturnVmstat are the counters that tell reclaim from refault: pages
// stolen by kswapd, direct and proactive reclaim, file refaults and major
// faults (DAMON evicting hot store pages shows here), and balloon moves.
var memoryReturnVmstat = []string{
	"pgmajfault",
	"pgsteal_kswapd", "pgsteal_direct", "pgsteal_proactive",
	"pgscan_kswapd", "pgscan_direct", "pgscan_proactive",
	"workingset_refault_file", "workingset_refault_anon",
	"balloon_inflate", "balloon_deflate", "balloon_migrate",
}

// ReadMemoryReturn is the guest's own memory-return state, read from the
// real /sys and /proc (the cache return act reports its free page reporting
// facts from it).
func ReadMemoryReturn() MemoryReturn {
	return readMemoryReturn("/sys", "/proc", int64(os.Getpagesize())/1024)
}

// readMemoryReturn reads under sysRoot (/sys) and procRoot (/proc).
// pageKiB is the guest's page size in KiB.
func readMemoryReturn(sysRoot, procRoot string, pageKiB int64) MemoryReturn {
	var out MemoryReturn
	note := func(err error) { out.Errors = append(out.Errors, err.Error()) }

	out.DamonReclaim = readDamonReclaim(sysRoot, procRoot, note)

	out.Balloon = readBalloonDriver(sysRoot, note)

	if v, ok, err := readSysInt(filepath.Join(sysRoot, "module/page_reporting/parameters/page_reporting_order")); err != nil {
		note(err)
	} else if ok {
		b := out.Balloon
		reporting := b.Bound && b.Reporting != nil && *b.Reporting
		out.PageReporting = PageReporting{Present: true, Order: &v, Registered: reporting && v >= 0 && v < 64}
	}

	if vm, err := readVmstat(filepath.Join(procRoot, "vmstat")); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			note(err)
		}
	} else {
		for _, k := range memoryReturnVmstat {
			if v, ok := vm[k]; ok {
				if out.Vmstat == nil {
					out.Vmstat = map[string]int64{}
				}
				out.Vmstat[k] = v
			}
		}
	}

	if fb, err := readBuddyinfo(filepath.Join(procRoot, "buddyinfo"), pageKiB); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			note(err)
		}
	} else {
		out.FreeBlocks = fb
		if out.PageReporting.Registered {
			var sum int64
			for order, kib := range fb.KiBByOrder {
				if int64(order) >= *out.PageReporting.Order {
					sum += kib
				}
			}
			fb.ReportableKiB = &sum
		}
	}
	return out
}

func readDamonReclaim(sysRoot, procRoot string, note func(error)) DamonReclaim {
	var d DamonReclaim
	dir := filepath.Join(sysRoot, "module/damon_reclaim/parameters")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			note(err)
		}
		return d
	}
	d.Present = true
	for _, e := range entries {
		name := e.Name()
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				note(err)
			}
			continue
		}
		val := strings.TrimSpace(string(raw))
		switch {
		case name == "enabled":
			b := val == "Y" || val == "1"
			d.Enabled = &b
		case name == "kdamond_pid" || strings.HasPrefix(name, "nr_") || strings.HasPrefix(name, "bytes_"):
			n, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				note(fmt.Errorf("damon_reclaim %s: not a number: %q", name, val))
				continue
			}
			if name == "kdamond_pid" {
				d.KdamondPID = &n
				continue
			}
			if d.Stats == nil {
				d.Stats = map[string]int64{}
			}
			d.Stats[name] = n
		default:
			if d.Parameters == nil {
				d.Parameters = map[string]string{}
			}
			d.Parameters[name] = val
		}
	}
	if d.KdamondPID != nil && *d.KdamondPID > 0 {
		d.KdamondCPUTicks = kernelThreadCPUTicks(procRoot, *d.KdamondPID, "kdamond")
	}
	return d
}

// kernelThreadCPUTicks is utime+stime of pid when its comm starts with
// prefix, so a pid reused by another process reports nothing.
func kernelThreadCPUTicks(procRoot string, pid int64, prefix string) *int64 {
	dir := filepath.Join(procRoot, strconv.FormatInt(pid, 10))
	comm, err := os.ReadFile(filepath.Join(dir, "comm"))
	if err != nil || !strings.HasPrefix(strings.TrimSpace(string(comm)), prefix) {
		return nil
	}
	stat, err := os.ReadFile(filepath.Join(dir, "stat"))
	if err != nil {
		return nil
	}
	// Fields after "(comm)": state is field 3, utime 14, stime 15.
	s := string(stat)
	i := strings.LastIndexByte(s, ')')
	if i < 0 {
		return nil
	}
	f := strings.Fields(s[i+1:])
	if len(f) < 13 {
		return nil
	}
	u, err1 := strconv.ParseInt(f[11], 10, 64)
	st, err2 := strconv.ParseInt(f[12], 10, 64)
	if err1 != nil || err2 != nil {
		return nil
	}
	n := u + st
	return &n
}

func readBalloonDriver(sysRoot string, note func(error)) BalloonDriver {
	var b BalloonDriver
	dir := filepath.Join(sysRoot, "bus/virtio/drivers/virtio_balloon")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			note(err)
		}
		return b
	}
	var devs []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "virtio") {
			devs = append(devs, e.Name())
		}
	}
	if len(devs) == 0 {
		return b
	}
	sort.Strings(devs)
	b.Bound, b.Device = true, devs[0]
	raw, err := os.ReadFile(filepath.Join(dir, devs[0], "features"))
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			note(err)
		}
		return b
	}
	bits := strings.TrimSpace(string(raw))
	bit := func(i int) *bool {
		if i >= len(bits) {
			return nil
		}
		v := bits[i] == '1'
		return &v
	}
	b.StatsVQ, b.DeflateOnOOM, b.FreePageHint, b.Reporting = bit(1), bit(2), bit(3), bit(5)
	return b
}

// readSysInt reads one integer file; ok is false when the file is absent.
func readSysInt(path string) (int64, bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, false, nil
		}
		return 0, false, err
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("%s: not a number: %q", filepath.Base(path), strings.TrimSpace(string(raw)))
	}
	return n, true, nil
}

// readVmstat reads "name value" lines.
func readVmstat(path string) (map[string]int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string]int64{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), " ")
		if !ok {
			continue
		}
		if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
			out[k] = n
		}
	}
	return out, sc.Err()
}

// readBuddyinfo sums "Node N, zone NAME c0 c1 ..." lines (free block counts
// per order) over zones and converts them to KiB.
func readBuddyinfo(path string, pageKiB int64) (*FreeBlocks, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fb := &FreeBlocks{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		// Node, 0,, zone, NAME, then counts.
		if len(fields) < 5 || fields[0] != "Node" || fields[2] != "zone" {
			continue
		}
		for order, c := range fields[4:] {
			n, err := strconv.ParseInt(c, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("buddyinfo: not a number: %q", c)
			}
			for len(fb.KiBByOrder) <= order {
				fb.KiBByOrder = append(fb.KiBByOrder, 0)
			}
			fb.KiBByOrder[order] += n * (pageKiB << order)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return fb, nil
}
