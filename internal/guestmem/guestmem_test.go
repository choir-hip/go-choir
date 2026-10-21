package guestmem

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// What can go wrong in a guest memory breakdown (SDC RES-07, launch
// checklist L1-02), each covered below:
//   - a meminfo value in kB reported as bytes, or an absent field reported
//     as zero memory;
//   - a process that exits mid-walk aborting the whole read;
//   - a kernel thread (no VmRSS line) counted as a zero-RSS group;
//   - a process name with spaces split, or its trailing newline kept;
//   - an unbounded process table returned whole;
//   - a nix-optimised hardlink counted once per name, doubling the store;
//   - a symlink followed out of the walked root;
//   - a walk that overruns its budget instead of stopping and saying so;
//   - a file the residency probe cannot read aborting the walk;
//   - a cgroup without memory files reported as using nothing.
// The kernel half (mincore over a mapping, never faulting pages in) is in
// resident_linux_test.go.

func TestMeminfoBytesKeepsOnlyPresentFields(t *testing.T) {
	got := Meminfo("MemTotal: 8000 kB\nCached:  300 kB\nShmem: 20 kB\nbogus\nHugePages_Total: 0\nMapped: x kB\n")
	if got["MemTotal"] != 8000<<10 || got["Cached"] != 300<<10 || got["Shmem"] != 20<<10 {
		t.Fatalf("meminfo = %v", got)
	}
	if _, ok := got["AnonPages"]; ok {
		t.Fatal("an absent field must be absent, not zero")
	}
	if _, ok := got["HugePages_Total"]; ok {
		t.Fatal("unselected fields must not be reported")
	}
	if _, ok := got["Mapped"]; ok {
		t.Fatal("an unparsable value must be absent")
	}
}

func writeProc(t *testing.T, root, pid, comm, status string) {
	t.Helper()
	dir := filepath.Join(root, pid)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if comm != "" {
		if err := os.WriteFile(filepath.Join(dir, "comm"), []byte(comm+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if status != "" {
		if err := os.WriteFile(filepath.Join(dir, "status"), []byte(status), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestProcessGroupsSumsRSSByNameAndBounds(t *testing.T) {
	root := t.TempDir()
	writeProc(t, root, "1", "autoputer", "Name:\tautoputer\nVmRSS:\t   2000 kB\n")
	writeProc(t, root, "20", "desk worker", "VmRSS:\t100 kB\n")
	writeProc(t, root, "21", "desk worker", "VmRSS:\t150 kB\n")
	writeProc(t, root, "2", "kthreadd", "Name:\tkthreadd\n") // kernel thread: no VmRSS
	writeProc(t, root, "33", "", "")                         // exited mid-walk
	writeProc(t, root, "40", "pdftotext", "VmRSS:\t50 kB\n")
	if err := os.MkdirAll(filepath.Join(root, "self"), 0o755); err != nil { // not a pid
		t.Fatal(err)
	}

	groups := ProcessGroups(root, 2)
	if len(groups) != 3 {
		t.Fatalf("groups = %+v, want two named and one other", groups)
	}
	if groups[0].Name != "autoputer" || groups[0].Processes != 1 || groups[0].RSSBytes != 2000<<10 {
		t.Fatalf("first = %+v", groups[0])
	}
	if groups[1].Name != "desk worker" || groups[1].Processes != 2 || groups[1].RSSBytes != 250<<10 {
		t.Fatalf("second = %+v", groups[1])
	}
	if groups[2].Name != OtherGroup || groups[2].Processes != 1 || groups[2].RSSBytes != 50<<10 {
		t.Fatalf("other = %+v", groups[2])
	}
	for _, g := range groups {
		if g.Name == "kthreadd" {
			t.Fatal("a kernel thread must not be a group")
		}
	}
}

func TestPageCacheDedupesHardlinksSkipsSymlinksAndProbeErrors(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	write := func(path string, n int) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, make([]byte, n), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "a", "lib.so"), 4096)
	if err := os.Link(filepath.Join(root, "a", "lib.so"), filepath.Join(root, "b-lib.so")); err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(root, "unreadable"), 10)
	write(filepath.Join(root, "empty"), 0)
	write(filepath.Join(outside, "big"), 1<<20)
	if err := os.Symlink(filepath.Join(outside, "big"), filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "dirlink")); err != nil {
		t.Fatal(err)
	}

	probe := func(path string, size int64) (int64, error) {
		if strings.HasSuffix(path, "unreadable") {
			return 0, errors.New("permission denied")
		}
		return size / 2, nil
	}
	got := pageCache("store", root, time.Second, 100, probe)
	if got.Files != 2 || got.Bytes != 4096 || got.ResidentBytes != 2048 {
		t.Fatalf("split = %+v, want one deduplicated lib.so and an empty file", got)
	}
	if got.Errors != 1 || got.Truncated {
		t.Fatalf("errors=%d truncated=%t", got.Errors, got.Truncated)
	}
}

func TestPageCacheStopsAtItsBoundsAndSaysSo(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 10; i++ {
		if err := os.WriteFile(filepath.Join(root, string(rune('a'+i))), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	probe := func(string, int64) (int64, error) { return 1, nil }
	if got := pageCache("nix", root, time.Second, 3, probe); !got.Truncated || got.Files != 3 {
		t.Fatalf("file cap: %+v", got)
	}
	slow := func(string, int64) (int64, error) { time.Sleep(20 * time.Millisecond); return 1, nil }
	if got := pageCache("nix", root, 30*time.Millisecond, 100, slow); !got.Truncated || got.Files >= 10 {
		t.Fatalf("time budget: %+v", got)
	}
	if got := pageCache("missing", filepath.Join(root, "nope"), time.Second, 100, probe); got.Files != 0 || got.Errors != 1 {
		t.Fatalf("missing root: %+v", got)
	}
}

func TestCgroupsReadsCurrentAndStatAndSkipsDirsWithoutMemory(t *testing.T) {
	root := t.TempDir()
	mk := func(rel, current, stat string) {
		t.Helper()
		dir := filepath.Join(root, rel)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if current != "" {
			if err := os.WriteFile(filepath.Join(dir, "memory.current"), []byte(current), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if stat != "" {
			if err := os.WriteFile(filepath.Join(dir, "memory.stat"), []byte(stat), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	mk("system.slice", "900\n", "anon 100\nfile 800\nshmem 7\n")
	mk("system.slice/autoputer.service", "700\n", "anon 90\nfile 610\n")
	mk("init.scope", "", "")
	mk("capsule.slice/a/b/too-deep", "5\n", "anon 5\n")

	got := Cgroups(root, 2, 10)
	if len(got) != 2 {
		t.Fatalf("cgroups = %+v", got)
	}
	if got[0].Path != "system.slice" || got[0].CurrentBytes != 900 || got[0].AnonBytes != 100 || got[0].FileBytes != 800 {
		t.Fatalf("first = %+v", got[0])
	}
	if got[1].Path != "system.slice/autoputer.service" || got[1].CurrentBytes != 700 {
		t.Fatalf("second = %+v", got[1])
	}
	if top := Cgroups(root, 2, 1); len(top) != 1 || top[0].Path != "system.slice" {
		t.Fatalf("bounded = %+v", top)
	}
}
