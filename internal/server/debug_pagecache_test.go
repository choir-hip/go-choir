//go:build linux

package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Ways the page-cache residency read could fail (written before the code;
// docs/problems/base-os-copied-into-every-guest-ram-2026-10-22.md):
//  1. It changes what it measures: reading files pulls them into the page
//     cache, so the read must only mmap and mincore, never read or fault.
//  2. It double-counts: the nix store is hardlink-optimised, so one inode
//     under many names must count once.
//  3. It follows symlinks out of the root, or crosses into another
//     filesystem mounted below it, and attributes that cache to the root.
//  4. It leaks names: persistent-state paths can name owner content, so
//     only store roots list their largest files; others give totals.
//  5. It runs unbounded on a large tree: a file cap and a deadline stop it
//     and the result says it is partial.
//  6. A missing root (no persistent mount) fails the whole read instead of
//     reporting that root as absent.
//  7. It is served to a guest caller: it rides the pprof prefix and its
//     host-sourced guard.
// 10. A few huge files run the walk long under the file cap: a byte cap
//     (apparent bytes examined) also stops it and says partial.
// 11. Joining procmem makes every procmem read pay for a full store walk:
//     the residency field is opt-in (procmem?pagecache=1), absent by
//     default, and behind the same host-sourced guard.
//  9. The kernel lies to an unprivileged caller: since Linux 5.2 mincore
//     reports every page resident for a file the caller neither owns nor
//     could write (without CAP_FOWNER), which on a read-only store would
//     read as 100% resident. Such files count as unmeasured, never resident.
//  8. Units are misread: residency is counted in pages and capped at the
//     file size, reported in bytes with the unit in the field name.

func writeFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("x", size)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func rootByName(t *testing.T, got PageCache, name string) PageCacheRoot {
	t.Helper()
	for _, r := range got.Roots {
		if r.Name == name {
			return r
		}
	}
	t.Fatalf("root %q missing from %+v", name, got.Roots)
	return PageCacheRoot{}
}

func TestPageCacheCountsResidentBytesOncePerInode(t *testing.T) {
	store := t.TempDir()
	writeFile(t, filepath.Join(store, "aaa-libc", "lib", "libc.so"), 3*4096+100)
	writeFile(t, filepath.Join(store, "bbb-daemon", "bin", "autoputer"), 8192)
	if err := os.Link(filepath.Join(store, "aaa-libc", "lib", "libc.so"), filepath.Join(store, "bbb-daemon", "libc-link.so")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/etc", filepath.Join(store, "ccc-escape")); err != nil {
		t.Fatal(err)
	}
	got, err := readPageCache([]pageCacheRootSpec{{Name: "store", Path: store, ListTop: true}}, pageCacheLimits{MaxBytes: 1 << 30, MaxFiles: 1000, Deadline: 10 * time.Second, Top: 1})
	if err != nil {
		t.Fatal(err)
	}
	r := rootByName(t, got, "store")
	if !r.Present || r.Partial {
		t.Fatalf("root = %+v", r)
	}
	if r.Files != 2 {
		t.Fatalf("files = %d, want 2 (hardlink once, symlink not followed)", r.Files)
	}
	want := int64(3*4096 + 100 + 8192)
	if r.Bytes != want {
		t.Fatalf("bytes = %d, want %d", r.Bytes, want)
	}
	// Freshly written files sit in the page cache; residency is capped at
	// the file size, not rounded up to whole pages.
	if r.ResidentBytes != want {
		t.Fatalf("resident = %d, want %d", r.ResidentBytes, want)
	}
	if len(r.Top) != 1 || !strings.HasSuffix(r.Top[0].Path, "libc.so") || r.Top[0].ResidentBytes != 3*4096+100 {
		t.Fatalf("top = %+v, want the largest resident file only", r.Top)
	}
	if !strings.HasPrefix(r.Top[0].Path, "aaa-libc/") {
		t.Fatalf("top path %q should be relative to the root", r.Top[0].Path)
	}
}

func TestPageCacheNeverFaultsPagesIn(t *testing.T) {
	store := t.TempDir()
	// A sparse file: holes are not resident until something reads them.
	f, err := os.Create(filepath.Join(store, "sparse.img"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(64 << 20); err != nil {
		t.Fatal(err)
	}
	f.Close()
	spec := []pageCacheRootSpec{{Name: "store", Path: store, ListTop: true}}
	for i := 0; i < 2; i++ {
		got, err := readPageCache(spec, pageCacheLimits{MaxBytes: 1 << 30, MaxFiles: 10, Deadline: 10 * time.Second, Top: 5})
		if err != nil {
			t.Fatal(err)
		}
		r := rootByName(t, got, "store")
		if r.Bytes != 64<<20 {
			t.Fatalf("bytes = %d", r.Bytes)
		}
		if r.ResidentBytes != 0 {
			t.Fatalf("pass %d: resident = %d; the read pulled pages into the cache", i, r.ResidentBytes)
		}
	}
}

func TestPageCacheHidesNamesOutsideStoreRoots(t *testing.T) {
	persistent := t.TempDir()
	writeFile(t, filepath.Join(persistent, "owner-private-title.md"), 4096)
	got, err := readPageCache([]pageCacheRootSpec{{Name: "persistent", Path: persistent}}, pageCacheLimits{MaxBytes: 1 << 30, MaxFiles: 10, Deadline: 10 * time.Second, Top: 5})
	if err != nil {
		t.Fatal(err)
	}
	r := rootByName(t, got, "persistent")
	if r.Files != 1 || r.ResidentBytes != 4096 || len(r.Top) != 0 {
		t.Fatalf("root = %+v, want totals only", r)
	}
	raw, _ := json.Marshal(got)
	if strings.Contains(string(raw), "owner-private-title") {
		t.Fatal("a persistent file name leaked into the read")
	}
}

func TestPageCacheStopsAtFileCapAndSaysPartial(t *testing.T) {
	store := t.TempDir()
	for _, n := range []string{"a", "b", "c", "d"} {
		writeFile(t, filepath.Join(store, n), 10)
	}
	got, err := readPageCache([]pageCacheRootSpec{{Name: "store", Path: store}}, pageCacheLimits{MaxBytes: 1 << 30, MaxFiles: 2, Deadline: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if r := rootByName(t, got, "store"); r.Files != 2 || !r.Partial {
		t.Fatalf("root = %+v, want 2 files and partial", r)
	}
	got, err = readPageCache([]pageCacheRootSpec{{Name: "store", Path: store}}, pageCacheLimits{MaxBytes: 1 << 30, MaxFiles: 100, Deadline: -time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if r := rootByName(t, got, "store"); !r.Partial {
		t.Fatalf("root = %+v, want partial after the deadline", r)
	}
}

func TestPageCacheMissingRootIsAbsentNotAnError(t *testing.T) {
	got, err := readPageCache([]pageCacheRootSpec{{Name: "persistent", Path: filepath.Join(t.TempDir(), "absent")}}, pageCacheLimits{MaxBytes: 1 << 30, MaxFiles: 10, Deadline: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if r := rootByName(t, got, "persistent"); r.Present || r.Files != 0 {
		t.Fatalf("root = %+v, want absent", r)
	}
}

func TestPageCacheServedOnlyToHostCallers(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, DebugPprofPrefix+"pagecache", nil)
	req.RemoteAddr = "203.0.113.9:4000"
	rec := httptest.NewRecorder()
	serveDebugPprof(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("guest caller got %d", rec.Code)
	}
}

func TestPageCacheCountsUnownedFilesAsUnmeasured(t *testing.T) {
	store := t.TempDir()
	writeFile(t, filepath.Join(store, "root-owned"), 8192)
	prev := pageCacheMayMeasure
	pageCacheMayMeasure = func(uint32) bool { return false }
	defer func() { pageCacheMayMeasure = prev }()
	got, err := readPageCache([]pageCacheRootSpec{{Name: "store", Path: store, ListTop: true}}, pageCacheLimits{MaxBytes: 1 << 30, MaxFiles: 10, Deadline: 10 * time.Second, Top: 5})
	if err != nil {
		t.Fatal(err)
	}
	r := rootByName(t, got, "store")
	if r.ResidentBytes != 0 || r.Files != 0 || len(r.Top) != 0 {
		t.Fatalf("root = %+v; an unmeasurable file was counted", r)
	}
	if r.UnmeasuredFiles != 1 || r.UnmeasuredBytes != 8192 {
		t.Fatalf("root = %+v, want one unmeasured file of 8192 bytes", r)
	}
}

func TestPageCacheStopsAtByteCapAndSaysPartial(t *testing.T) {
	store := t.TempDir()
	for _, n := range []string{"a", "b", "c"} {
		writeFile(t, filepath.Join(store, n), 4096)
	}
	got, err := readPageCache([]pageCacheRootSpec{{Name: "store", Path: store}}, pageCacheLimits{MaxFiles: 100, MaxBytes: 8192, Deadline: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if r := rootByName(t, got, "store"); r.Files != 2 || r.Bytes != 8192 || !r.Partial {
		t.Fatalf("root = %+v, want 2 files, 8192 bytes and partial", r)
	}
}

func TestProcMemJoinsPageCacheOnlyWhenAsked(t *testing.T) {
	if _, err := os.Stat("/proc/meminfo"); err != nil {
		t.Skip("no /proc")
	}
	store := t.TempDir()
	writeFile(t, filepath.Join(store, "pkg", "bin", "tool"), 4096)
	prev := guestPageCacheRoots
	guestPageCacheRoots = []pageCacheRootSpec{{Name: "store", Path: store, ListTop: true}}
	defer func() { guestPageCacheRoots = prev }()
	get := func(remote, query string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, DebugPprofPrefix+"procmem"+query, nil)
		req.RemoteAddr = remote
		rec := httptest.NewRecorder()
		serveDebugPprof(rec, req)
		return rec
	}
	if rec := get("203.0.113.9:4000", "?pagecache=1"); rec.Code != http.StatusForbidden {
		t.Fatalf("guest caller got %d", rec.Code)
	}
	var plain map[string]json.RawMessage
	rec := get("127.0.0.1:4000", "")
	if err := json.Unmarshal(rec.Body.Bytes(), &plain); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("plain procmem %d %q", rec.Code, rec.Body.String())
	}
	if _, ok := plain["page_cache"]; ok {
		t.Fatal("plain procmem walked the page cache")
	}
	var joined struct {
		Meminfo   map[string]int64 `json:"meminfo"`
		PageCache *PageCache       `json:"page_cache"`
	}
	rec = get("127.0.0.1:4000", "?pagecache=1")
	if err := json.Unmarshal(rec.Body.Bytes(), &joined); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("joined procmem %d %q", rec.Code, rec.Body.String())
	}
	if joined.Meminfo["MemTotal_kib"] == 0 || joined.PageCache == nil {
		t.Fatalf("joined = %+v, want meminfo and page_cache", joined)
	}
	r := rootByName(t, *joined.PageCache, "store")
	if r.Files != 1 || r.ResidentBytes != 4096 || len(r.Top) != 1 || r.Top[0].Path != "pkg/bin/tool" {
		t.Fatalf("store root = %+v", r)
	}
}
