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
	"context"
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
	// Truncated names the sections the request budget cut short ("processes",
	// "cgroups", "page_cache:<label>"); their numbers are a floor.
	Truncated []string `json:"truncated,omitempty"`
	// Busy says another detail read was already running, so this one did no
	// work; ask again once it has finished.
	Busy bool `json:"busy,omitempty"`
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

// Limits bound a Read. Budget, MaxEntries and MaxProbeBytes are the whole
// request's, shared by every section and tree; TreeMaxFiles is per tree.
type Limits struct {
	TopProcesses int
	CgroupDepth  int
	TopCgroups   int
	// Budget is the request-wide deadline.
	Budget time.Duration
	// MaxEntries caps directory entries examined across the whole read
	// (process table, cgroup walk and file trees together).
	MaxEntries   int
	TreeMaxFiles int
	// ProbeChunkBytes is the size of one mincore mapping; MaxProbeBytes caps
	// the file bytes the residency probe maps across the whole read.
	ProbeChunkBytes int64
	MaxProbeBytes   int64
}

// DefaultLimits keeps one Read to a few seconds and a bounded amount of
// work on a guest with a full nix store.
var DefaultLimits = Limits{
	TopProcesses: 16, CgroupDepth: 2, TopCgroups: 24,
	Budget: 3 * time.Second, MaxEntries: 400000, TreeMaxFiles: 200000,
	ProbeChunkBytes: 64 << 20, MaxProbeBytes: 16 << 30,
}

// readGate admits one detail read at a time: the read exists to diagnose a
// guest under memory pressure and must not multiply its own load.
var readGate = make(chan struct{}, 1)

// scan is one request's budget: a deadline, an entry count, a probe-byte
// allowance and the sections that ran out. It is used from one goroutine.
type scan struct {
	ctx        context.Context
	deadline   time.Time
	entries    int
	maxEntries int
	probeLeft  int64
	chunk      int64
	truncated  map[string]bool
}

func newScan(ctx context.Context, lim Limits) *scan {
	chunk := lim.ProbeChunkBytes
	if page := int64(os.Getpagesize()); chunk < page {
		chunk = page
	} else {
		chunk -= chunk % page
	}
	return &scan{
		ctx: ctx, deadline: time.Now().Add(lim.Budget),
		maxEntries: lim.MaxEntries, probeLeft: lim.MaxProbeBytes, chunk: chunk,
		truncated: map[string]bool{},
	}
}

// done says the deadline passed or the request went away.
func (s *scan) done() bool {
	return s.ctx.Err() != nil || !time.Now().Before(s.deadline)
}

// spend takes one entry from the budget; false means stop.
func (s *scan) spend() bool {
	if s.entries >= s.maxEntries || s.done() {
		return false
	}
	s.entries++
	return true
}

// readDir lists dir in small batches, calling fn per entry until fn says
// stop or the budget is spent; running out marks section truncated. It
// returns the open error, if any.
func (s *scan) readDir(dir, section string, fn func(os.DirEntry) bool) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	for {
		batch, err := f.ReadDir(256)
		for _, e := range batch {
			if !s.spend() {
				s.truncated[section] = true
				return nil
			}
			if !fn(e) {
				return nil
			}
		}
		if err != nil {
			// io.EOF ends the listing; any other error ends it as far as it got.
			return nil
		}
	}
}

// Read takes one breakdown from the live kernel under one request-wide
// budget. It stops when ctx is done and refuses to run beside another Read.
func Read(ctx context.Context, trees []Tree, lim Limits) *Detail {
	select {
	case readGate <- struct{}{}:
		defer func() { <-readGate }()
	default:
		return &Detail{Busy: true}
	}
	start := time.Now()
	s := newScan(ctx, lim)
	out := &Detail{}
	if raw, err := os.ReadFile("/proc/meminfo"); err == nil {
		out.Meminfo = Meminfo(string(raw))
	}
	out.Processes = processGroups(s, "/proc", lim.TopProcesses)
	out.Cgroups = cgroups(s, "/sys/fs/cgroup", lim.CgroupDepth, lim.TopCgroups)
	for _, tree := range trees {
		if tree.Root == "" {
			continue
		}
		split := pageCache(s, tree.Label, tree.Root, lim.TreeMaxFiles, residentBytes)
		out.PageCache = append(out.PageCache, split)
	}
	for _, name := range []string{"processes", "cgroups"} {
		if s.truncated[name] {
			out.Truncated = append(out.Truncated, name)
		}
	}
	for _, split := range out.PageCache {
		if split.Truncated {
			out.Truncated = append(out.Truncated, "page_cache:"+split.Label)
		}
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

// processGroups sums VmRSS by process name under a /proc root, largest
// first, keeping top groups and folding the rest into OtherGroup. A process
// that exits mid-walk or has no VmRSS (a kernel thread) is skipped. It
// examines at most the request's entry budget and marks "processes" when it
// stops early.
func processGroups(s *scan, procRoot string, top int) []ProcessGroup {
	byName := map[string]*ProcessGroup{}
	if err := s.readDir(procRoot, "processes", func(entry os.DirEntry) bool {
		if _, err := strconv.Atoi(entry.Name()); err != nil {
			return true
		}
		dir := filepath.Join(procRoot, entry.Name())
		rss, ok := vmRSS(filepath.Join(dir, "status"))
		if !ok {
			return true
		}
		comm, err := os.ReadFile(filepath.Join(dir, "comm"))
		if err != nil {
			return true
		}
		name := strings.TrimRight(string(comm), "\n")
		g := byName[name]
		if g == nil {
			g = &ProcessGroup{Name: name}
			byName[name] = g
		}
		g.Processes++
		g.RSSBytes += rss
		return true
	}); err != nil {
		return nil
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

// cgroups reads memory.current and memory.stat for the cgroups at most depth
// levels under a cgroup v2 root, largest first, keeping top. A cgroup
// without memory.current (no memory controller there) is skipped. It marks
// "cgroups" when the request budget stops the walk.
func cgroups(s *scan, root string, depth, top int) []Cgroup {
	var out []Cgroup
	var walk func(dir, rel string, level int)
	walk = func(dir, rel string, level int) {
		if level > depth {
			return
		}
		_ = s.readDir(dir, "cgroups", func(entry os.DirEntry) bool {
			if !entry.IsDir() {
				return true
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
			return !s.truncated["cgroups"]
		})
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
// and asks probe how many of its bytes are cached. Directories are read in
// batches against the request's shared budget. It stops at maxFiles, the
// shared entry or probe budget, the deadline or a cancelled request, and
// says so; a probe that did not cover the whole file also says so.
func pageCache(s *scan, label, root string, maxFiles int, probe func(string, int64, *scan) (int64, bool, error)) PageCacheSplit {
	out := PageCacheSplit{Label: label, Root: root}
	seen := map[fileKey]bool{}
	section := "page_cache:" + label
	stopped := false
	var walk func(dir string) bool
	// A root that is a regular file (a SQLite state file) counts that file
	// and its sidecars in the same directory (<name>-wal, <name>-shm).
	fileRoot := ""
	if info, err := os.Lstat(root); err == nil && info.Mode().IsRegular() {
		fileRoot = filepath.Base(root)
	}
	walk = func(dir string) bool {
		err := s.readDir(dir, section, func(d os.DirEntry) bool {
			path := filepath.Join(dir, d.Name())
			if fileRoot != "" && d.Name() != fileRoot && !strings.HasPrefix(d.Name(), fileRoot+"-") {
				return true
			}
			switch {
			case fileRoot != "" && d.IsDir():
				return true
			case d.IsDir():
				return walk(path)
			case !d.Type().IsRegular():
				return true
			}
			if out.Files >= maxFiles {
				stopped = true
				return false
			}
			info, err := d.Info()
			if err != nil {
				out.Errors++
				return true
			}
			if st, ok := info.Sys().(*syscall.Stat_t); ok {
				key := fileKey{dev: uint64(st.Dev), ino: uint64(st.Ino)}
				if seen[key] {
					return true
				}
				seen[key] = true
			}
			resident, complete, err := probe(path, info.Size(), s)
			if err != nil {
				out.Errors++
				return true
			}
			out.Files++
			out.Bytes += info.Size()
			out.ResidentBytes += resident
			if !complete {
				stopped = true
				return false
			}
			return true
		})
		if err != nil {
			out.Errors++
		}
		return !stopped && !s.truncated[section]
	}
	if s.done() {
		out.Truncated = true
		return out
	}
	if fileRoot != "" {
		walk(filepath.Dir(root))
	} else {
		walk(root)
	}
	out.Truncated = stopped || s.truncated[section]
	return out
}
