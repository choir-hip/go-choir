//go:build linux

package server

import (
	"encoding/json"
	"fmt"
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
//
// From review (DeepSeek v4.1 Flash, bunch 13):
// 12. A bound is checked only between files: one huge file is mapped whole
//     before the byte cap rejects it, a file started just before the
//     deadline runs past it, and a directory-only tree ignores the deadline.
//     The byte cap applies before mapping, the deadline inside the chunk
//     loop and on every directory.
// 13. The overlay upper (the updater's private store) is counted twice: once
//     through /nix/store and again under /mnt/persistent, and inside the
//     base-store total. It is its own root (store_upper), excluded from
//     persistent, and the read says whether it could partition.
// 14. Skipped subtrees vanish: an unreadable directory or a mount below the
//     root (systemd InaccessiblePaths are mounts) is not counted, so a floor
//     reads as a total. Both are counted.
// 15. The file cap bounds measured files only: unmeasured files and
//     hardlink repeats run free. It bounds every regular file examined.
// 16. Exact residency of fresh files can flake under memory pressure; the
//     tests assert bounds (never above the file size) instead.

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
	// Residency is capped at the file size, not rounded up to whole pages
	// (fresh files are usually all resident; memory pressure may evict some).
	if r.ResidentBytes > want {
		t.Fatalf("resident = %d, above the %d bytes of the files", r.ResidentBytes, want)
	}
	if len(r.Top) > 1 {
		t.Fatalf("top = %+v, want at most the largest resident file", r.Top)
	}
	if len(r.Top) == 1 {
		if top := r.Top[0]; !strings.HasPrefix(top.Path, "aaa-libc/") && !strings.HasPrefix(top.Path, "bbb-daemon/") || top.ResidentBytes > 3*4096+100 {
			t.Fatalf("top = %+v, want a root-relative path and residency within the file", r.Top)
		}
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
	if r.Files != 1 || r.ResidentBytes > 4096 || len(r.Top) != 0 {
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
	useGuestPaths(t, store, filepath.Join(t.TempDir(), "absent"), "")
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
	if r.Files != 1 || r.ResidentBytes > 4096 || len(r.Top) > 1 || len(r.Top) == 1 && r.Top[0].Path != "pkg/bin/tool" {
		t.Fatalf("store root = %+v", r)
	}
}

// useGuestPaths points the guest read at test trees. An empty mountinfo
// body writes none (an unreadable mountinfo).
func useGuestPaths(t *testing.T, store, persistent, mountinfo string) {
	t.Helper()
	mi := filepath.Join(t.TempDir(), "mountinfo")
	if mountinfo != "" {
		if err := os.WriteFile(mi, []byte(mountinfo), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ps, pp, pm := guestStorePath, guestPersistentPath, guestMountinfoPath
	guestStorePath, guestPersistentPath, guestMountinfoPath = store, persistent, mi
	t.Cleanup(func() { guestStorePath, guestPersistentPath, guestMountinfoPath = ps, pp, pm })
}

func sparseFile(t *testing.T, path string, size int64) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := f.Truncate(size); err != nil {
		t.Skipf("this filesystem cannot hold a %d-byte sparse file: %v", size, err)
	}
}

func TestPageCacheByteCapStopsBeforeMappingAHugeFile(t *testing.T) {
	store := t.TempDir()
	sparseFile(t, filepath.Join(store, "huge.img"), 1<<40)
	start := time.Now()
	got, err := readPageCache([]pageCacheRootSpec{{Name: "store", Path: store}}, pageCacheLimits{MaxFiles: 10, MaxBytes: 1 << 20, Deadline: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	r := rootByName(t, got, "store")
	if !r.Partial || r.Files != 0 || r.Bytes != 0 {
		t.Fatalf("root = %+v, want partial with nothing counted", r)
	}
	if el := time.Since(start); el > 250*time.Millisecond {
		t.Fatalf("took %v: the 1 TiB file was mapped before the byte cap applied", el)
	}
}

func TestPageCacheDeadlineStopsInsideAFile(t *testing.T) {
	store := t.TempDir()
	sparseFile(t, filepath.Join(store, "huge.img"), 1<<40)
	start := time.Now()
	got, err := readPageCache([]pageCacheRootSpec{{Name: "store", Path: store}}, pageCacheLimits{MaxFiles: 10, MaxBytes: 2 << 40, Deadline: 30 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if r := rootByName(t, got, "store"); !r.Partial || r.Files != 0 {
		t.Fatalf("root = %+v, want partial with the unfinished file not counted", r)
	}
	if el := time.Since(start); el > 500*time.Millisecond {
		t.Fatalf("took %v against a 30 ms deadline", el)
	}
}

func TestPageCacheDeadlineStopsADirectoryOnlyWalk(t *testing.T) {
	store := t.TempDir()
	for i := 0; i < 50; i++ {
		if err := os.MkdirAll(filepath.Join(store, fmt.Sprintf("d%02d", i), "sub"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got, err := readPageCache([]pageCacheRootSpec{{Name: "store", Path: store}}, pageCacheLimits{MaxFiles: 10, MaxBytes: 1 << 30, Deadline: -time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if r := rootByName(t, got, "store"); !r.Partial {
		t.Fatalf("root = %+v, want partial: the deadline passed with directories left", r)
	}
}

func TestPageCacheFileCapCountsEveryExaminedFile(t *testing.T) {
	store := t.TempDir()
	writeFile(t, filepath.Join(store, "a"), 10)
	for _, n := range []string{"b", "c", "d"} {
		if err := os.Link(filepath.Join(store, "a"), filepath.Join(store, n)); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(store, "e"), 10)
	prev := pageCacheMayMeasure
	pageCacheMayMeasure = func(uint32) bool { return false }
	defer func() { pageCacheMayMeasure = prev }()
	got, err := readPageCache([]pageCacheRootSpec{{Name: "store", Path: store}}, pageCacheLimits{MaxFiles: 3, MaxBytes: 1 << 30, Deadline: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	r := rootByName(t, got, "store")
	if !r.Partial || r.ExaminedFiles != 3 {
		t.Fatalf("root = %+v, want partial after 3 examined files (hardlink repeats and unmeasured files count)", r)
	}
}

func TestPageCacheCountsSkippedSubtrees(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 directory")
	}
	persistent := t.TempDir()
	writeFile(t, filepath.Join(persistent, "dolt", "x"), 100)
	locked := filepath.Join(persistent, "choir-signers")
	writeFile(t, filepath.Join(locked, "key"), 32)
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(locked, 0o755)
	got, err := readPageCache([]pageCacheRootSpec{{Name: "persistent", Path: persistent}}, pageCacheLimits{MaxFiles: 10, MaxBytes: 1 << 30, Deadline: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	r := rootByName(t, got, "persistent")
	if r.Files != 1 || r.Skipped != 1 || r.Partial {
		t.Fatalf("root = %+v, want one file, one skipped subtree, not partial (a floor, not a bound)", r)
	}
}

func TestParseStoreOverlayTakesTheTopMount(t *testing.T) {
	mi := "24 1 254:0 / /nix/store ro,relatime - erofs /dev/vda ro\n" +
		"30 24 254:16 / /mnt/persistent rw - ext4 /dev/vdb rw\n" +
		"41 24 0:44 / /nix/store rw - overlay overlay rw,lowerdir=/nix/store,upperdir=/mnt/persistent/choir-updater/store,workdir=/mnt/persistent/choir-updater/.overlay-work\n"
	got, ok := parseStoreOverlay(mi, "/nix/store")
	if !ok || got.FSType != "overlay" || got.Upper != "/mnt/persistent/choir-updater/store" || got.Work != "/mnt/persistent/choir-updater/.overlay-work" {
		t.Fatalf("got %+v ok=%v", got, ok)
	}
	got, ok = parseStoreOverlay(mi[:strings.Index(mi, "41 24")], "/nix/store")
	if !ok || got.FSType != "erofs" || got.Upper != "" {
		t.Fatalf("plain erofs store: got %+v ok=%v", got, ok)
	}
	if _, ok := parseStoreOverlay("30 1 254:16 / /mnt/persistent rw - ext4 /dev/vdb rw\n", "/nix/store"); ok {
		t.Fatal("no store mount must not parse as known")
	}
}

func TestPageCacheSplitsTheOverlayUpperFromTheBaseStore(t *testing.T) {
	store, persistent := t.TempDir(), t.TempDir()
	upper := filepath.Join(persistent, "choir-updater", "store")
	work := filepath.Join(persistent, "choir-updater", ".overlay-work")
	writeFile(t, filepath.Join(store, "aaa-base", "bin", "x"), 4096)
	writeFile(t, filepath.Join(upper, "bbb-release", "bin", "y"), 8192)
	writeFile(t, filepath.Join(work, "w"), 10)
	writeFile(t, filepath.Join(persistent, "dolt", "data"), 100)
	// The merged view shows the upper file under the store path.
	if err := os.MkdirAll(filepath.Join(store, "bbb-release", "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(filepath.Join(upper, "bbb-release", "bin", "y"), filepath.Join(store, "bbb-release", "bin", "y")); err != nil {
		t.Fatal(err)
	}
	useGuestPaths(t, store, persistent, fmt.Sprintf("41 24 0:44 / %s rw - overlay overlay rw,lowerdir=%s,upperdir=%s,workdir=%s\n", store, store, upper, work))
	got, err := readGuestPageCache()
	if err != nil {
		t.Fatal(err)
	}
	if l := got.StoreLayering; l == nil || !l.Partitioned || l.FSType != "overlay" || l.Upper != upper {
		t.Fatalf("store_layering = %+v", got.StoreLayering)
	}
	if r := rootByName(t, got, "store"); r.Files != 1 || r.Bytes != 4096 {
		t.Fatalf("store = %+v, want the base file only", r)
	}
	if r := rootByName(t, got, "store_upper"); r.Files != 1 || r.Bytes != 8192 || len(r.Top) > 1 {
		t.Fatalf("store_upper = %+v, want the release file only", r)
	}
	if r := rootByName(t, got, "persistent"); r.Files != 1 || r.Bytes != 100 {
		t.Fatalf("persistent = %+v, want the Dolt file only (upper and work excluded)", r)
	}
}

func TestPageCacheWithoutMountinfoSaysNotPartitioned(t *testing.T) {
	store := t.TempDir()
	writeFile(t, filepath.Join(store, "a"), 10)
	useGuestPaths(t, store, filepath.Join(t.TempDir(), "absent"), "")
	got, err := readGuestPageCache()
	if err != nil {
		t.Fatal(err)
	}
	if got.StoreLayering == nil || got.StoreLayering.Partitioned {
		t.Fatalf("store_layering = %+v, want present and not partitioned", got.StoreLayering)
	}
}

func TestPageCacheServedAloneToHostCallers(t *testing.T) {
	store := t.TempDir()
	writeFile(t, filepath.Join(store, "pkg", "bin", "tool"), 4096)
	useGuestPaths(t, store, filepath.Join(t.TempDir(), "absent"), fmt.Sprintf("24 1 254:0 / %s ro - erofs /dev/vda ro\n", store))
	req := httptest.NewRequest(http.MethodGet, DebugPprofPrefix+"pagecache", nil)
	req.RemoteAddr = "127.0.0.1:4000"
	rec := httptest.NewRecorder()
	serveDebugPprof(rec, req)
	var got PageCache
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil {
		t.Fatalf("host caller got %d %q", rec.Code, rec.Body.String())
	}
	if r := rootByName(t, got, "store"); r.Files != 1 || got.StoreLayering == nil || !got.StoreLayering.Partitioned {
		t.Fatalf("read = %+v", got)
	}
}
