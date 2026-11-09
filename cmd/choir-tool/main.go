// Command choir-tool keeps a guest tool layer's memory proportional to its use
// (docs/design/guest-tools-live-load-2026-11-09.md, phase 1). The layers are
// read-only mounts at /nix/.ro-tool-<layer>; their pages stay in the guest page
// cache after a tool ran. evict-idle drops the pages of every layer that no
// process maps and whose exec-log stamp is old.
//
// Failure modes, written before the code: a layer mount is absent (single
// image: exit 0 with nothing to do); a file vanishes or is unreadable mid-walk
// (skip it); a symlink points out of the mount (never followed); an empty file
// cannot be mmapped (zero resident); a process starts using the layer between
// the busy check and the fadvise (harmless: DONTNEED drops only clean cache and
// the process faults pages back); fadvise unsupported on the filesystem
// (resident stays, reported as is); a stamp is garbled (treated as absent); the
// state directory is unwritable (the decision still stands, the error is
// reported with exit 1).
package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const idleGrace = 10 * time.Minute

type env struct {
	mountRoot string // holds .ro-tool-<layer> directories; /nix in the guest
	procRoot  string
	stampDir  string
	stateDir  string
	now       func() time.Time
}

func defaultEnv() env {
	return env{mountRoot: "/nix", procRoot: "/proc", stampDir: "/run/choir-tool-exec", stateDir: "/run/choir-tool-evict", now: time.Now}
}

// The exec-log wrappers (nix/guest-tools.nix `logged`): the toolchain's are
// go, gcc, cc, g++ and c++; the office layer wraps every binary of
// libreoffice, so any other stamp is office's. Browser and delegate are called
// by absolute path and have no stamp.
var toolchainStamps = map[string]bool{"go": true, "gcc": true, "cc": true, "g++": true, "c++": true}

func (e env) layerDir(layer string) string { return filepath.Join(e.mountRoot, ".ro-tool-"+layer) }

func (e env) layers() []string {
	ents, _ := os.ReadDir(e.mountRoot)
	var out []string
	for _, d := range ents {
		if n, ok := strings.CutPrefix(d.Name(), ".ro-tool-"); ok && n != "" {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// walkFiles calls fn for each regular file under dir; symlinks are skipped.
func walkFiles(dir string, fn func(path string, size int64)) {
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
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
}

func (e env) resident(layer string) (res, total int64) {
	walkFiles(e.layerDir(layer), func(p string, size int64) {
		total += size
		res += fileResident(p, size)
	})
	return
}

func (e env) evict(layer string) {
	walkFiles(e.layerDir(layer), func(p string, _ int64) { dropFile(p) })
}

// busy returns the pids that map or execute a file of the layer. The layer's
// store paths are the top-level entries of its mount, seen by processes as
// /nix/store/<entry>.
func (e env) busy(layer string) []int {
	ents, err := os.ReadDir(e.layerDir(layer))
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
	procs, _ := os.ReadDir(e.procRoot)
	var pids []int
	for _, d := range procs {
		pid, err := strconv.Atoi(d.Name())
		if err != nil {
			continue
		}
		base := filepath.Join(e.procRoot, d.Name())
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

func readStamp(path string) (time.Time, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}, false
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	if err != nil || n <= 0 {
		return time.Time{}, false
	}
	return time.Unix(n, 0), true
}

// lastUse is the newest evidence that the layer was used: its tools' stamps
// and the last time busy saw it in use (the state file).
func (e env) lastUse(layer string) (time.Time, bool) {
	var newest time.Time
	found := false
	note := func(t time.Time, ok bool) {
		if ok && (!found || t.After(newest)) {
			newest, found = t, true
		}
	}
	ents, _ := os.ReadDir(e.stampDir)
	for _, d := range ents {
		name := d.Name()
		switch layer {
		case "toolchain":
			if !toolchainStamps[name] {
				continue
			}
		case "office":
			if toolchainStamps[name] {
				continue
			}
		default:
			continue
		}
		note(readStamp(filepath.Join(e.stampDir, name)))
	}
	note(readStamp(filepath.Join(e.stateDir, layer)))
	return newest, found
}

type result struct {
	Layer          string `json:"layer"`
	ResidentBytes  int64  `json:"resident_bytes"`
	TotalBytes     int64  `json:"total_bytes"`
	Evicted        bool   `json:"evicted,omitempty"`
	ResidentBefore *int64 `json:"resident_before,omitempty"`
	ResidentAfter  *int64 `json:"resident_after,omitempty"`
	Busy           []int  `json:"busy,omitempty"`
	Reason         string `json:"reason,omitempty"`
}

func (e env) evictIdle(w func(any)) error {
	var firstErr error
	fail := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	for _, l := range e.layers() {
		r := result{Layer: l}
		r.ResidentBytes, r.TotalBytes = e.resident(l)
		if pids := e.busy(l); len(pids) > 0 {
			r.Busy, r.Reason = pids, "busy"
			err := os.MkdirAll(e.stateDir, 0o755)
			if err == nil {
				err = os.WriteFile(filepath.Join(e.stateDir, l), []byte(strconv.FormatInt(e.now().Unix(), 10)+"\n"), 0o644)
			}
			fail(err)
			w(r)
			continue
		}
		if t, ok := e.lastUse(l); ok && e.now().Sub(t) < idleGrace {
			r.Reason = "used within grace"
			w(r)
			continue
		}
		before := r.ResidentBytes
		e.evict(l)
		after, _ := e.resident(l)
		r.Evicted, r.ResidentBefore, r.ResidentAfter, r.ResidentBytes, r.Reason = true, &before, &after, after, "idle"
		w(r)
	}
	return firstErr
}

func run(e env, args []string, out func(any)) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: choir-tool resident|busy|evict <layer> | evict-idle")
		return 64
	}
	layerArg := func() (string, bool) {
		if len(args) != 2 || args[1] == "" || strings.ContainsAny(args[1], "/.") {
			fmt.Fprintf(os.Stderr, "choir-tool %s: want one layer name\n", args[0])
			return "", false
		}
		if _, err := os.Stat(e.layerDir(args[1])); err != nil {
			fmt.Fprintf(os.Stderr, "choir-tool: layer %q is not mounted\n", args[1])
			return "", false
		}
		return args[1], true
	}
	switch args[0] {
	case "resident":
		l, ok := layerArg()
		if !ok {
			return 64
		}
		r := result{Layer: l}
		r.ResidentBytes, r.TotalBytes = e.resident(l)
		out(r)
	case "busy":
		l, ok := layerArg()
		if !ok {
			return 64
		}
		out(struct {
			Layer string `json:"layer"`
			Pids  []int  `json:"pids"`
		}{l, append([]int{}, e.busy(l)...)})
	case "evict":
		l, ok := layerArg()
		if !ok {
			return 64
		}
		if pids := e.busy(l); len(pids) > 0 {
			fmt.Fprintf(os.Stderr, "choir-tool: layer %s is in use by pids %v; not evicting\n", l, pids)
			return 2
		}
		before, total := e.resident(l)
		e.evict(l)
		after, _ := e.resident(l)
		out(result{Layer: l, TotalBytes: total, ResidentBytes: after, Evicted: true, ResidentBefore: &before, ResidentAfter: &after})
	case "evict-idle":
		if err := e.evictIdle(out); err != nil {
			fmt.Fprintln(os.Stderr, "choir-tool: evict-idle:", err)
			return 1
		}
	default:
		fmt.Fprintf(os.Stderr, "choir-tool: unknown command %q\n", args[0])
		return 64
	}
	return 0
}

func main() {
	enc := json.NewEncoder(os.Stdout)
	os.Exit(run(defaultEnv(), os.Args[1:], func(v any) { _ = enc.Encode(v) }))
}
