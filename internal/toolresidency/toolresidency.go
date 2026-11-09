// Package toolresidency measures and drops the page cache of the guest's tool
// layers (docs/design/guest-tools-live-load-2026-11-09.md, phase 1). The layers
// are read-only erofs mounts at /nix/.ro-tool-<layer>. cmd/choir-tool evicts
// with it and the autoputer's observability reports with it, so the L3.5 proof
// reads tool memory from the API instead of over SSH.
//
// Failure modes: a file vanishes or is unreadable mid-walk (skipped); a symlink
// points out of the mount (never followed); an empty file cannot be mmapped
// (zero resident); the walk outlives its context (partial counts and the
// context's error); a status file is garbled or absent (treated as never
// evicted).
package toolresidency

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// IdleGrace is how long a layer must be unused before evict-idle drops it, and
// TimerInterval is how often nix/autoputer-vm.nix runs evict-idle
// (choir-tool-evict-idle.timer, OnUnitInactiveSec). Keep both in step with it.
const (
	IdleGrace     = 10 * time.Minute
	TimerInterval = 5 * time.Minute
)

// LayerPrefix names a layer's mount directory under the mount root.
const LayerPrefix = ".ro-tool-"

// LayerDir is the mount directory of a layer.
func LayerDir(mountRoot, layer string) string {
	return filepath.Join(mountRoot, LayerPrefix+layer)
}

// Layers lists the layer names mounted under mountRoot, sorted.
func Layers(mountRoot string) []string {
	ents, _ := os.ReadDir(mountRoot)
	var out []string
	for _, d := range ents {
		if n, ok := strings.CutPrefix(d.Name(), LayerPrefix); ok && n != "" {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// walkFiles calls fn for each regular file under dir; symlinks are skipped. It
// stops with the context's error once ctx is done.
func walkFiles(ctx context.Context, dir string, fn func(path string, size int64)) error {
	var stop error
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if e := ctx.Err(); e != nil {
			stop = e
			return e
		}
		if err != nil {
			return nil
		}
		if d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				fn(p, info.Size())
			}
		}
		return nil
	})
	return stop
}

// Resident returns the bytes of the layer directory's files in the page cache
// and their total size. When ctx ends first the counts are partial and err is
// the context's.
func Resident(ctx context.Context, dir string) (res, total int64, err error) {
	err = walkFiles(ctx, dir, func(p string, size int64) {
		total += size
		res += FileResident(p, size)
	})
	return
}

// Evict asks the kernel to drop the clean cached pages of every file under dir.
func Evict(ctx context.Context, dir string) error {
	return walkFiles(ctx, dir, func(p string, _ int64) { DropFile(p) })
}

// Busy returns the pids whose exe or /proc/<pid>/maps lie under a store path of
// the layer. The layer's store paths are the top-level entries of its mount,
// seen by processes as /nix/store/<entry>.
func Busy(ctx context.Context, procRoot, layerDir string) []int {
	ents, err := os.ReadDir(layerDir)
	if err != nil {
		return nil
	}
	var prefixes []string
	for _, d := range ents {
		prefixes = append(prefixes, "/nix/store/"+d.Name())
	}
	match := func(s string) bool {
		s = strings.TrimSuffix(s, " (deleted)")
		// Anywhere in the path: a capsule's chroot or merged root renders
		// the store path behind its own prefix.
		for _, p := range prefixes {
			if strings.HasSuffix(s, p) || strings.Contains(s, p+"/") {
				return true
			}
		}
		return false
	}
	procs, _ := os.ReadDir(procRoot)
	var pids []int
	for _, d := range procs {
		if ctx.Err() != nil {
			break
		}
		pid, err := strconv.Atoi(d.Name())
		if err != nil {
			continue
		}
		base := filepath.Join(procRoot, d.Name())
		hit := false
		if exe, err := os.Readlink(filepath.Join(base, "exe")); err == nil && match(exe) {
			hit = true
		}
		if !hit {
			if b, err := os.ReadFile(filepath.Join(base, "maps")); err == nil {
				for _, line := range strings.Split(string(b), "\n") {
					if i := strings.IndexByte(line, '/'); i >= 0 && match(line[i:]) {
						hit = true
						break
					}
				}
			}
		}
		if hit {
			pids = append(pids, pid)
		}
	}
	sort.Ints(pids)
	return pids
}

// Status is what choir-tool evict-idle records per layer in
// <stateDir>/<layer>.json, for the observability route to read.
type Status struct {
	LastRunAt     time.Time `json:"last_run_at"`
	LastEvictedAt time.Time `json:"last_evicted_at,omitzero"`
}

// StatusPath is the layer's status file.
func StatusPath(stateDir, layer string) string { return filepath.Join(stateDir, layer+".json") }

// ReadStatus reads a layer's status; the zero Status when absent or garbled.
func ReadStatus(stateDir, layer string) Status {
	var s Status
	if b, err := os.ReadFile(StatusPath(stateDir, layer)); err == nil {
		_ = json.Unmarshal(b, &s)
	}
	return s
}

// RecordRun notes that evict-idle looked at the layer at now, and evicted it
// when evicted is true; the last eviction time is kept across runs that did not
// evict.
func RecordRun(stateDir, layer string, now time.Time, evicted bool) error {
	s := ReadStatus(stateDir, layer)
	s.LastRunAt = now.UTC()
	if evicted {
		s.LastEvictedAt = now.UTC()
	}
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(StatusPath(stateDir, layer), append(b, '\n'), 0o644)
}
