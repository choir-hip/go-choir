// Per-process memory for host-sourced callers. A guest's host RSS is far
// larger than its main process's Go heap (the paper computer: 5.6 GiB host
// RSS, 1.46 GB Sys); the rest is desk-session workers, other processes and
// the page cache, which no profile shows
// (docs/problems/autopaper-intake-defects-2026-10-14.md, memory findings).
// This read names that split. It reads /proc/<pid>/comm and status only:
// command lines carry flags and tokens and are never read.

package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ProcMemProcess is one process's memory, in KiB as the kernel reports it.
type ProcMemProcess struct {
	PID     int    `json:"pid"`
	PPID    int    `json:"ppid"`
	Name    string `json:"name"`
	RSSKiB  int64  `json:"rss_kib"`
	HWMKiB  int64  `json:"hwm_kib"`
	AnonKiB int64  `json:"anon_kib,omitempty"`
	FileKiB int64  `json:"file_kib,omitempty"`
}

// ProcMemGroup totals the processes that share a name.
type ProcMemGroup struct {
	Count  int   `json:"count"`
	RSSKiB int64 `json:"rss_kib"`
}

// ProcMem is the whole read: the guest's meminfo and its processes,
// largest RSS first.
type ProcMem struct {
	Meminfo     map[string]int64        `json:"meminfo"`
	TotalRSSKiB int64                   `json:"total_rss_kib"`
	ByName      map[string]ProcMemGroup `json:"by_name"`
	Processes   []ProcMemProcess        `json:"processes"`
	// PageCache is joined only on procmem?pagecache=1: the walk costs a
	// second or two of one core (debug_pagecache.go).
	PageCache      *PageCache `json:"page_cache,omitempty"`
	PageCacheError string     `json:"page_cache_error,omitempty"`
	// MemoryReturn is DAMON reclaim, free page reporting and the balloon as
	// the guest sees them (debug_memreturn.go). A few dozen small reads.
	MemoryReturn *MemoryReturn `json:"memory_return,omitempty"`
}

var procMeminfoKeys = []string{"MemTotal", "MemFree", "MemAvailable", "Buffers", "Cached", "Shmem", "Slab", "AnonPages"}

func readProcMem(root string) (ProcMem, error) {
	out := ProcMem{Meminfo: map[string]int64{}, ByName: map[string]ProcMemGroup{}}
	info, err := readKiBFields(filepath.Join(root, "meminfo"))
	if err != nil {
		return out, fmt.Errorf("procmem: %w", err)
	}
	for _, k := range procMeminfoKeys {
		if v, ok := info[k]; ok {
			out.Meminfo[k+"_kib"] = v
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return out, fmt.Errorf("procmem: %w", err)
	}
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		dir := filepath.Join(root, e.Name())
		status, err := readKiBFields(filepath.Join(dir, "status"))
		if err != nil {
			continue // exited mid-scan, or unreadable
		}
		rss, ok := status["VmRSS"]
		if !ok {
			continue // kernel thread
		}
		comm, _ := os.ReadFile(filepath.Join(dir, "comm"))
		p := ProcMemProcess{
			PID:     pid,
			PPID:    int(status["PPid"]),
			Name:    strings.TrimSpace(string(comm)),
			RSSKiB:  rss,
			HWMKiB:  status["VmHWM"],
			AnonKiB: status["RssAnon"],
			FileKiB: status["RssFile"],
		}
		out.Processes = append(out.Processes, p)
		g := out.ByName[p.Name]
		g.Count++
		g.RSSKiB += rss
		out.ByName[p.Name] = g
		out.TotalRSSKiB += rss
	}
	sort.Slice(out.Processes, func(i, j int) bool { return out.Processes[i].RSSKiB > out.Processes[j].RSSKiB })
	return out, nil
}

// readKiBFields reads "Key: value [kB]" lines. PPid has no unit and is read
// the same way.
func readKiBFields(path string) (map[string]int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fields := map[string]int64{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		key, rest, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		parts := strings.Fields(rest)
		if len(parts) == 0 {
			continue
		}
		if v, err := strconv.ParseInt(parts[0], 10, 64); err == nil {
			fields[key] = v
		}
	}
	return fields, sc.Err()
}

func serveProcMem(w http.ResponseWriter, r *http.Request) {
	mem, err := readProcMem("/proc")
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotImplemented)
		return
	}
	mr := readMemoryReturn("/sys", "/proc", int64(os.Getpagesize())/1024)
	mem.MemoryReturn = &mr
	if r.URL.Query().Get("pagecache") == "1" {
		if pc, err := readGuestPageCache(); err != nil {
			mem.PageCacheError = err.Error()
		} else {
			mem.PageCache = &pc
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(mem)
}
