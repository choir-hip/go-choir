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
	"path/filepath"
	"sort"
	"time"
)

// PageCacheFile is one file's resident bytes, its path relative to the root.
type PageCacheFile struct {
	Path          string `json:"path"`
	ResidentBytes int64  `json:"resident_bytes"`
}

// PageCacheRoot totals one walked root. Files and bytes count each inode
// once; Unmeasured files are counted apart; Partial says a bound stopped the walk; Skipped counts files that
// could not be opened or mapped.
type PageCacheRoot struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	Present       bool   `json:"present"`
	Files         int    `json:"files"`
	Bytes         int64  `json:"bytes"`
	ResidentBytes int64  `json:"resident_bytes"`
	Skipped       int    `json:"skipped,omitempty"`
	// Unmeasured files are ones the kernel would report as fully resident
	// whatever their state: since Linux 5.2, mincore answers truthfully only
	// for a file the caller owns or could write, or with CAP_FOWNER.
	UnmeasuredFiles int             `json:"unmeasured_files,omitempty"`
	UnmeasuredBytes int64           `json:"unmeasured_bytes,omitempty"`
	Partial         bool            `json:"partial,omitempty"`
	Top             []PageCacheFile `json:"top,omitempty"`
}

// PageCache is the whole read: meminfo, then each root.
type PageCache struct {
	Meminfo   map[string]int64 `json:"meminfo,omitempty"`
	Roots     []PageCacheRoot  `json:"roots"`
	ElapsedMS int64            `json:"elapsed_ms"`
}

type pageCacheRootSpec struct {
	Name    string
	Path    string
	ListTop bool // file names are public (the nix store)
}

type pageCacheLimits struct {
	MaxFiles int           // per root
	Deadline time.Duration // whole read
	Top      int           // largest resident files listed per ListTop root
}

// The guest's roots: the base-OS store and the per-computer state volume
// (nix/autoputer-vm.nix volumes, mountPoint /mnt/persistent).
var guestPageCacheRoots = []pageCacheRootSpec{
	{Name: "store", Path: "/nix/store", ListTop: true},
	{Name: "persistent", Path: "/mnt/persistent"},
}

var guestPageCacheLimits = pageCacheLimits{MaxFiles: 500_000, Deadline: 30 * time.Second, Top: 40}

type inodeKey struct{ dev, ino uint64 }

func readPageCache(roots []pageCacheRootSpec, lim pageCacheLimits) (PageCache, error) {
	if err := pageCacheSupported(); err != nil {
		return PageCache{}, err
	}
	start := time.Now()
	deadline := start.Add(lim.Deadline)
	out := PageCache{}
	for _, spec := range roots {
		r, err := walkPageCacheRoot(spec, lim, deadline)
		if err != nil {
			return out, err
		}
		out.Roots = append(out.Roots, r)
	}
	out.ElapsedMS = time.Since(start).Milliseconds()
	return out, nil
}

func walkPageCacheRoot(spec pageCacheRootSpec, lim pageCacheLimits, deadline time.Time) (PageCacheRoot, error) {
	r := PageCacheRoot{Name: spec.Name, Path: spec.Path}
	rootDev, ok := fileDevice(spec.Path)
	if !ok {
		return r, nil // absent root: reported, not an error
	}
	r.Present = true
	seen := map[inodeKey]bool{}
	var top []PageCacheFile
	_ = filepath.WalkDir(spec.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if d == nil || d.IsDir() {
				r.Skipped++
				return fs.SkipDir
			}
			r.Skipped++
			return nil
		}
		if d.IsDir() {
			// Do not cross into another filesystem mounted below the root.
			if dev, ok := fileDevice(path); !ok || dev != rootDev {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil // symlinks are never followed
		}
		if time.Now().After(deadline) || r.Files >= lim.MaxFiles {
			r.Partial = true
			return fs.SkipAll
		}
		res, ok := fileResidency(path)
		if !ok {
			r.Skipped++
			return nil
		}
		key := inodeKey{res.dev, res.ino}
		if seen[key] {
			return nil
		}
		seen[key] = true
		if !pageCacheMayMeasure(res.uid) {
			r.UnmeasuredFiles++
			r.UnmeasuredBytes += res.size
			return nil
		}
		r.Files++
		r.Bytes += res.size
		r.ResidentBytes += res.resident
		if spec.ListTop && lim.Top > 0 && res.resident > 0 {
			rel, _ := filepath.Rel(spec.Path, path)
			top = append(top, PageCacheFile{Path: rel, ResidentBytes: res.resident})
		}
		return nil
	})
	sort.Slice(top, func(i, j int) bool { return top[i].ResidentBytes > top[j].ResidentBytes })
	if len(top) > lim.Top {
		top = top[:lim.Top]
	}
	r.Top = top
	return r, nil
}

func servePageCache(w http.ResponseWriter) {
	out, err := readPageCache(guestPageCacheRoots, guestPageCacheLimits)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotImplemented)
		return
	}
	if info, err := readKiBFields("/proc/meminfo"); err == nil {
		out.Meminfo = map[string]int64{}
		for _, k := range []string{"MemTotal", "Cached", "Shmem", "Active(file)", "Inactive(file)", "Mapped"} {
			if v, ok := info[k]; ok {
				out.Meminfo[k+"_kib"] = v
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
