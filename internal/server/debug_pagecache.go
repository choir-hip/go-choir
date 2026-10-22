// Page-cache residency by filesystem for host-sourced callers. An idle
// guest holds about 640 MiB of page cache, and whether that is the shared
// base OS (the nix store image, which every guest copies into its own RAM)
// or the computer's own Dolt data decides lever L10 / SDC RES-08
// (docs/problems/base-os-copied-into-every-guest-ram-2026-10-22.md).
// procmem shows only the meminfo total. This read walks each root and asks
// the kernel which pages of each file are resident (mmap + mincore): it
// never reads file contents and never faults a page in, so it does not
// change what it measures. Only store roots list file names; other roots
// give totals, because persistent-state names can carry owner content.

package server

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// PageCacheFile is one file's resident bytes, its path relative to the root.
type PageCacheFile struct {
	Path          string `json:"path"`
	ResidentBytes int64  `json:"resident_bytes"`
}

// PageCacheRoot totals one root. Files, bytes and residency count each
// measured inode once. ExaminedFiles counts every regular file the walk
// reached (hardlink repeats and unmeasured files included), which is what
// the file cap bounds. Partial says a bound stopped the walk. Skipped
// (unreadable directories and files) and SkippedMounts (filesystems mounted
// below the root, such as systemd InaccessiblePaths) make the totals a
// floor even when Partial is false; Excluded names subtrees left out on
// purpose because another root counts them.
type PageCacheRoot struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	Present       bool   `json:"present"`
	Files         int    `json:"files"`
	Bytes         int64  `json:"bytes"`
	ResidentBytes int64  `json:"resident_bytes"`
	ExaminedFiles int    `json:"examined_files"`
	Skipped       int    `json:"skipped,omitempty"`
	SkippedMounts int    `json:"skipped_mounts,omitempty"`
	// Unmeasured files are ones the kernel would report as fully resident
	// whatever their state: since Linux 5.2, mincore answers truthfully only
	// for a file the caller owns or could write, or with CAP_FOWNER.
	UnmeasuredFiles int             `json:"unmeasured_files,omitempty"`
	UnmeasuredBytes int64           `json:"unmeasured_bytes,omitempty"`
	Partial         bool            `json:"partial,omitempty"`
	Excluded        []string        `json:"excluded,omitempty"`
	Top             []PageCacheFile `json:"top,omitempty"`
}

// PageCacheLayering says how the reader sees /nix/store. When the release
// runs layered, /nix/store is an overlay whose upper is the updater's
// private store on /mnt/persistent (nix/autoputer-vm.nix layering): its
// files are the release delta, not the base image, so they are counted as
// store_upper and left out of persistent. Partitioned is false when the
// mount table could not be read; then the store and persistent totals may
// both include the upper and must not be quoted as the base image.
type PageCacheLayering struct {
	FSType      string `json:"fstype"`
	Upper       string `json:"upper,omitempty"`
	Work        string `json:"work,omitempty"`
	Partitioned bool   `json:"partitioned"`
}

// PageCache is the whole read: meminfo, the store's layering, then each root.
type PageCache struct {
	Meminfo       map[string]int64   `json:"meminfo,omitempty"`
	StoreLayering *PageCacheLayering `json:"store_layering,omitempty"`
	Roots         []PageCacheRoot    `json:"roots"`
	ElapsedMS     int64              `json:"elapsed_ms"`
}

type pageCacheRootSpec struct {
	Name    string
	Path    string
	ListTop bool // file names are public (the nix store)
	// UpperDir, when set, is an overlay upper: a file whose root-relative
	// path is a regular file there is counted in the root UpperName.
	UpperDir  string
	UpperName string
	Exclude   []string // subtrees another root counts
}

type pageCacheLimits struct {
	MaxFiles int           // regular files examined per walk
	MaxBytes int64         // apparent bytes of distinct inodes per walk; zero stops at once
	Deadline time.Duration // whole read, checked per directory, per file and per mapped chunk
	Top      int           // largest resident files listed per ListTop root
}

// The guest's roots: the base-OS store and the per-computer state volume
// (nix/autoputer-vm.nix volumes, mountPoint /mnt/persistent).
var (
	guestStorePath      = "/nix/store"
	guestPersistentPath = "/mnt/persistent"
	guestMountinfoPath  = "/proc/self/mountinfo"
)

var guestPageCacheLimits = pageCacheLimits{MaxFiles: 500_000, MaxBytes: 64 << 30, Deadline: 30 * time.Second, Top: 40}

type inodeKey struct{ dev, ino uint64 }

// residency outcomes of fileResidency.
const (
	residencyOK = iota
	residencyFailed
	residencyDeadline
)

func readPageCache(roots []pageCacheRootSpec, lim pageCacheLimits) (PageCache, error) {
	if err := pageCacheSupported(); err != nil {
		return PageCache{}, err
	}
	start := time.Now()
	deadline := start.Add(lim.Deadline)
	out := PageCache{}
	seen := map[inodeKey]bool{} // shared, so no inode counts in two roots
	for _, spec := range roots {
		out.Roots = append(out.Roots, walkPageCacheRoot(spec, lim, deadline, seen)...)
	}
	out.ElapsedMS = time.Since(start).Milliseconds()
	return out, nil
}

// pageCacheTally accumulates one root and its largest files.
type pageCacheTally struct {
	root PageCacheRoot
	top  []PageCacheFile
}

func (t *pageCacheTally) finish(limit int) PageCacheRoot {
	sort.Slice(t.top, func(i, j int) bool { return t.top[i].ResidentBytes > t.top[j].ResidentBytes })
	if len(t.top) > limit {
		t.top = t.top[:limit]
	}
	t.root.Top = t.top
	return t.root
}

func walkPageCacheRoot(spec pageCacheRootSpec, lim pageCacheLimits, deadline time.Time, seen map[inodeKey]bool) []PageCacheRoot {
	base := &pageCacheTally{root: PageCacheRoot{Name: spec.Name, Path: spec.Path}}
	var upper *pageCacheTally
	if spec.UpperDir != "" {
		upper = &pageCacheTally{root: PageCacheRoot{Name: spec.UpperName, Path: spec.UpperDir}}
		_, upper.root.Present = fileDevice(spec.UpperDir)
	}
	result := func() []PageCacheRoot {
		out := []PageCacheRoot{base.finish(lim.Top)}
		if upper != nil {
			out = append(out, upper.finish(lim.Top))
		}
		return out
	}
	rootDev, ok := fileDevice(spec.Path)
	if !ok {
		return result() // absent root: reported, not an error
	}
	base.root.Present = true
	excluded := map[string]bool{}
	for _, e := range spec.Exclude {
		if e != "" {
			excluded[filepath.Clean(e)] = true
		}
	}
	stop := func() error {
		base.root.Partial = true
		if upper != nil {
			upper.root.Partial = true
		}
		return fs.SkipAll
	}
	var files int
	var bytes int64
	_ = filepath.WalkDir(spec.Path, func(path string, d fs.DirEntry, err error) error {
		if time.Now().After(deadline) {
			return stop()
		}
		if err != nil {
			base.root.Skipped++
			if d == nil || d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if excluded[filepath.Clean(path)] {
				base.root.Excluded = append(base.root.Excluded, path)
				return fs.SkipDir
			}
			// Do not cross into another filesystem mounted below the root.
			if dev, ok := fileDevice(path); !ok || dev != rootDev {
				base.root.SkippedMounts++
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil // symlinks are never followed
		}
		t := base
		if upper != nil {
			rel, _ := filepath.Rel(spec.Path, path)
			if fi, err := os.Lstat(filepath.Join(spec.UpperDir, rel)); err == nil && fi.Mode().IsRegular() {
				t = upper
			}
		}
		if files >= lim.MaxFiles {
			return stop()
		}
		files++
		t.root.ExaminedFiles++
		id, ok := fileIdentity(d)
		if !ok {
			t.root.Skipped++
			return nil
		}
		key := inodeKey{id.dev, id.ino}
		if seen[key] {
			return nil
		}
		// The byte cap applies before anything is mapped.
		if bytes+id.size > lim.MaxBytes {
			return stop()
		}
		bytes += id.size
		seen[key] = true
		if !pageCacheMayMeasure(id.uid) {
			t.root.UnmeasuredFiles++
			t.root.UnmeasuredBytes += id.size
			return nil
		}
		resident, status := fileResidency(path, id, deadline)
		switch status {
		case residencyDeadline:
			return stop() // the unfinished file is not counted
		case residencyFailed:
			t.root.Skipped++
			return nil
		}
		t.root.Files++
		t.root.Bytes += id.size
		t.root.ResidentBytes += resident
		if spec.ListTop && lim.Top > 0 && resident > 0 {
			rel, _ := filepath.Rel(spec.Path, path)
			t.top = append(t.top, PageCacheFile{Path: rel, ResidentBytes: resident})
		}
		return nil
	})
	return result()
}

// parseStoreOverlay reads the topmost mount at mountpoint from a
// /proc/self/mountinfo body: its filesystem type and, for an overlay, its
// upper and work directories. ok is false when no mount is found.
func parseStoreOverlay(mountinfo, mountpoint string) (PageCacheLayering, bool) {
	var got PageCacheLayering
	found := false
	for _, line := range strings.Split(mountinfo, "\n") {
		f := strings.Fields(line)
		sep := -1
		for i, v := range f {
			if v == "-" && i >= 6 {
				sep = i
				break
			}
		}
		if sep < 0 || len(f) < sep+4 || unescapeMountinfo(f[4]) != mountpoint {
			continue
		}
		got, found = PageCacheLayering{FSType: f[sep+1]}, true
		if got.FSType != "overlay" {
			continue
		}
		for _, opt := range strings.Split(f[sep+3], ",") {
			if v, ok := strings.CutPrefix(opt, "upperdir="); ok {
				got.Upper = unescapeMountinfo(v)
			} else if v, ok := strings.CutPrefix(opt, "workdir="); ok {
				got.Work = unescapeMountinfo(v)
			}
		}
	}
	return got, found
}

// unescapeMountinfo undoes the kernel's octal escapes (\040 for a space).
func unescapeMountinfo(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			if n, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(n))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func servePageCache(w http.ResponseWriter) {
	out, err := readGuestPageCache()
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotImplemented)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// readGuestPageCache is the guest read served by pagecache and joined into
// procmem?pagecache=1 (owner notes §40).
func readGuestPageCache() (PageCache, error) {
	layering := PageCacheLayering{}
	if mi, err := os.ReadFile(guestMountinfoPath); err == nil {
		layering, layering.Partitioned = parseStoreOverlay(string(mi), guestStorePath)
	}
	roots := []pageCacheRootSpec{
		{Name: "store", Path: guestStorePath, ListTop: true, UpperDir: layering.Upper, UpperName: "store_upper"},
		{Name: "persistent", Path: guestPersistentPath, Exclude: []string{layering.Upper, layering.Work}},
	}
	out, err := readPageCache(roots, guestPageCacheLimits)
	if err != nil {
		return out, err
	}
	out.StoreLayering = &layering
	if info, err := readKiBFields("/proc/meminfo"); err == nil {
		out.Meminfo = map[string]int64{}
		for _, k := range []string{"MemTotal", "Cached", "Shmem", "Active(file)", "Inactive(file)", "Mapped"} {
			if v, ok := info[k]; ok {
				out.Meminfo[k+"_kib"] = v
			}
		}
	}
	return out, nil
}
