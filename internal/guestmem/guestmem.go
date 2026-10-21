// Package guestmem reads where a guest's memory goes: the kernel's meminfo
// split, resident memory by process name, memory by cgroup, and how much of
// the page cache belongs to which tree of files (the computer's store versus
// the shared nix store). It is the in-guest half of SDC RES-07 and launch
// checklist L1-02, read through the owner's observability route instead of a
// shell. Every read is bounded and read-only; the residency probe never
// faults a page in.
package guestmem

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// OtherGroup names the sum of the processes past the top groups.
const OtherGroup = "(other)"

// meminfoFields are the /proc/meminfo lines that split used memory into
// anonymous, page cache, shared memory and reclaimable slab.
var meminfoFields = map[string]bool{
	"MemTotal": true, "MemFree": true, "MemAvailable": true,
	"Buffers": true, "Cached": true, "Shmem": true, "AnonPages": true,
	"Mapped": true, "Dirty": true, "SReclaimable": true, "SUnreclaim": true,
	"Active(file)": true, "Inactive(file)": true,
	"Active(anon)": true, "Inactive(anon)": true,
}

// Detail is one breakdown of a guest's memory.
type Detail struct {
	// Meminfo holds the selected /proc/meminfo values in bytes; a field the
	// kernel did not report is absent, never zero.
	Meminfo   map[string]uint64 `json:"meminfo,omitempty"`
	Processes []ProcessGroup    `json:"processes,omitempty"`
	Cgroups   []Cgroup          `json:"cgroups,omitempty"`
	PageCache []PageCacheSplit  `json:"page_cache,omitempty"`
	// ElapsedMillis is how long the whole read took.
	ElapsedMillis int64 `json:"elapsed_ms"`
}

// ProcessGroup is the resident memory of the processes sharing one name.
type ProcessGroup struct {
	Name      string `json:"name"`
	Processes int    `json:"processes"`
	RSSBytes  uint64 `json:"rss_bytes"`
}

// Cgroup is one cgroup's charged memory and its anonymous and file parts.
type Cgroup struct {
	Path         string `json:"path"`
	CurrentBytes uint64 `json:"current_bytes"`
	AnonBytes    uint64 `json:"anon_bytes"`
	FileBytes    uint64 `json:"file_bytes"`
	ShmemBytes   uint64 `json:"shmem_bytes"`
}

// PageCacheSplit is how much of one tree of files is in the page cache.
type PageCacheSplit struct {
	Label         string `json:"label"`
	Root          string `json:"root"`
	Files         int    `json:"files"`
	Bytes         int64  `json:"bytes"`
	ResidentBytes int64  `json:"resident_bytes"`
	// Errors counts files the probe could not read; Truncated says the walk
	// stopped at its file cap or time budget, so the counts are a floor.
	Errors    int  `json:"errors,omitempty"`
	Truncated bool `json:"truncated,omitempty"`
}

// Tree names one tree of files to split the page cache by.
type Tree struct {
	Label string
	Root  string
}

// Limits bound a Read.
type Limits struct {
	TopProcesses int
	CgroupDepth  int
	TopCgroups   int
	TreeBudget   time.Duration
	TreeMaxFiles int
}

// DefaultLimits keeps one Read to a few seconds on a guest with a full nix
// store.
var DefaultLimits = Limits{TopProcesses: 16, CgroupDepth: 2, TopCgroups: 24, TreeBudget: 3 * time.Second, TreeMaxFiles: 200000}

// Read takes one breakdown from the live kernel.
func Read(trees []Tree, lim Limits) *Detail {
	start := time.Now()
	out := &Detail{}
	if raw, err := os.ReadFile("/proc/meminfo"); err == nil {
		out.Meminfo = Meminfo(string(raw))
	}
	out.Processes = ProcessGroups("/proc", lim.TopProcesses)
	out.Cgroups = Cgroups("/sys/fs/cgroup", lim.CgroupDepth, lim.TopCgroups)
	for _, tree := range trees {
		if tree.Root == "" {
			continue
		}
		out.PageCache = append(out.PageCache, pageCache(tree.Label, tree.Root, lim.TreeBudget, lim.TreeMaxFiles, residentBytes))
	}
	out.ElapsedMillis = time.Since(start).Milliseconds()
	return out
}

// Meminfo parses the selected /proc/meminfo fields to bytes.
func Meminfo(data string) map[string]uint64 {
	out := map[string]uint64{}
	for _, line := range strings.Split(data, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimSuffix(fields[0], ":")
		if !meminfoFields[name] {
			continue
		}
		kib, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		out[name] = kib << 10
	}
	return out
}

// ProcessGroups sums VmRSS by process name under a /proc root, largest
// first, keeping top groups and folding the rest into OtherGroup. A process
// that exits mid-walk or has no VmRSS (a kernel thread) is skipped.
func ProcessGroups(procRoot string, top int) []ProcessGroup {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil
	}
	byName := map[string]*ProcessGroup{}
	for _, entry := range entries {
		if _, err := strconv.Atoi(entry.Name()); err != nil {
			continue
		}
		dir := filepath.Join(procRoot, entry.Name())
		rss, ok := vmRSS(filepath.Join(dir, "status"))
		if !ok {
			continue
		}
		comm, err := os.ReadFile(filepath.Join(dir, "comm"))
		if err != nil {
			continue
		}
		name := strings.TrimRight(string(comm), "\n")
		g := byName[name]
		if g == nil {
			g = &ProcessGroup{Name: name}
			byName[name] = g
		}
		g.Processes++
		g.RSSBytes += rss
	}
	groups := make([]ProcessGroup, 0, len(byName))
	for _, g := range byName {
		groups = append(groups, *g)
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].RSSBytes != groups[j].RSSBytes {
			return groups[i].RSSBytes > groups[j].RSSBytes
		}
		return groups[i].Name < groups[j].Name
	})
	if top <= 0 || len(groups) <= top {
		return groups
	}
	other := ProcessGroup{Name: OtherGroup}
	for _, g := range groups[top:] {
		other.Processes += g.Processes
		other.RSSBytes += g.RSSBytes
	}
	return append(groups[:top:top], other)
}

func vmRSS(statusPath string) (uint64, bool) {
	f, err := os.Open(statusPath)
	if err != nil {
		return 0, false
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[0] == "VmRSS:" {
			kib, err := strconv.ParseUint(fields[1], 10, 64)
			return kib << 10, err == nil
		}
	}
	return 0, false
}

// Cgroups reads memory.current and memory.stat for the cgroups at most depth
// levels under a cgroup v2 root, largest first, keeping top. A cgroup
// without memory.current (no memory controller there) is skipped.
func Cgroups(root string, depth, top int) []Cgroup {
	var out []Cgroup
	var walk func(dir, rel string, level int)
	walk = func(dir, rel string, level int) {
		if level > depth {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			child := filepath.Join(dir, entry.Name())
			childRel := entry.Name()
			if rel != "" {
				childRel = rel + "/" + entry.Name()
			}
			if cg, ok := readCgroup(child, childRel); ok {
				out = append(out, cg)
			}
			walk(child, childRel, level+1)
		}
	}
	walk(root, "", 1)
	sort.Slice(out, func(i, j int) bool {
		if out[i].CurrentBytes != out[j].CurrentBytes {
			return out[i].CurrentBytes > out[j].CurrentBytes
		}
		return out[i].Path < out[j].Path
	})
	if top > 0 && len(out) > top {
		out = out[:top]
	}
	return out
}

func readCgroup(dir, rel string) (Cgroup, bool) {
	raw, err := os.ReadFile(filepath.Join(dir, "memory.current"))
	if err != nil {
		return Cgroup{}, false
	}
	current, err := strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil {
		return Cgroup{}, false
	}
	cg := Cgroup{Path: rel, CurrentBytes: current}
	if stat, err := os.ReadFile(filepath.Join(dir, "memory.stat")); err == nil {
		for _, line := range strings.Split(string(stat), "\n") {
			fields := strings.Fields(line)
			if len(fields) != 2 {
				continue
			}
			v, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				continue
			}
			switch fields[0] {
			case "anon":
				cg.AnonBytes = v
			case "file":
				cg.FileBytes = v
			case "shmem":
				cg.ShmemBytes = v
			}
		}
	}
	return cg, true
}

type fileKey struct{ dev, ino uint64 }

// pageCache walks one tree without following symlinks, counting each
// regular file once by device and inode (nix-optimised hardlinks share one),
// and asks probe how many of its bytes are cached. It stops at maxFiles or
// budget and says so.
func pageCache(label, root string, budget time.Duration, maxFiles int, probe func(string, int64) (int64, error)) PageCacheSplit {
	out := PageCacheSplit{Label: label, Root: root}
	deadline := time.Now().Add(budget)
	seen := map[fileKey]bool{}
	errStop := fs.SkipAll
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			out.Errors++
			if path == root {
				return errStop
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if out.Files >= maxFiles || time.Now().After(deadline) {
			out.Truncated = true
			return errStop
		}
		info, err := d.Info()
		if err != nil {
			out.Errors++
			return nil
		}
		if st, ok := info.Sys().(*syscall.Stat_t); ok {
			key := fileKey{dev: uint64(st.Dev), ino: uint64(st.Ino)}
			if seen[key] {
				return nil
			}
			seen[key] = true
		}
		resident, err := probe(path, info.Size())
		if err != nil {
			out.Errors++
			return nil
		}
		out.Files++
		out.Bytes += info.Size()
		out.ResidentBytes += resident
		return nil
	})
	if err != nil && err != errStop {
		out.Errors++
	}
	return out
}
