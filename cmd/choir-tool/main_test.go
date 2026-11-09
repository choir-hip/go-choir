package main

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func newEnv(t *testing.T) env {
	t.Helper()
	root := t.TempDir()
	return env{mountRoot: root, procRoot: filepath.Join(root, "proc"), stampDir: filepath.Join(root, "stamps"),
		stateDir: filepath.Join(root, "state"), now: time.Now}
}

// layerFile writes a 1 MiB file in the layer's store entry and faults it in.
func layerFile(t *testing.T, e env, layer, entry string) string {
	t.Helper()
	d := filepath.Join(e.layerDir(layer), entry, "bin")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(d, "tool")
	if err := os.WriteFile(p, make([]byte, 1<<20), 0o755); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(p); err != nil || len(b) != 1<<20 {
		t.Fatal(err)
	}
	return p
}

func TestResidentAndEvict(t *testing.T) {
	e := newEnv(t)
	layerFile(t, e, "toolchain", "aaa-go")
	_ = os.Symlink("/etc/hostname", filepath.Join(e.layerDir("toolchain"), "link")) // never followed
	res, total := e.resident("toolchain")
	if total != 1<<20 {
		t.Fatalf("total %d", total)
	}
	if res == 0 {
		t.Skip("no page-cache measurement on this platform or filesystem (mincore reports nothing)")
	}
	syncAll() // fadvise drops only clean pages
	e.evict("toolchain")
	after, _ := e.resident("toolchain")
	if after >= res {
		t.Skipf("filesystem ignores fadvise DONTNEED (resident %d -> %d)", res, after)
	}
}

func fakeProc(t *testing.T, e env, pid int, exe, maps string) {
	t.Helper()
	d := filepath.Join(e.procRoot, strconv.Itoa(pid))
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if exe != "" {
		if err := os.Symlink(exe, filepath.Join(d, "exe")); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(d, "maps"), []byte(maps), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBusyAndRefuse(t *testing.T) {
	e := newEnv(t)
	layerFile(t, e, "toolchain", "aaa-go")
	fakeProc(t, e, 10, "/nix/store/aaa-go/bin/go", "")
	fakeProc(t, e, 11, "/usr/bin/x", "7f00-7f01 r-xp 0 00:00 0   /nix/store/aaa-go/lib/libx.so\n")
	fakeProc(t, e, 12, "/usr/bin/y", "7f00-7f01 r-xp 0 00:00 0   /nix/store/aaa-gone/lib/libx.so\n")
	fakeProc(t, e, 13, "/nix/store/aaa-go-other/bin/z", "")
	fakeProc(t, e, 14, "/x/merged/nix/store/aaa-go/bin/go", "")
	fakeProc(t, e, 15, "/usr/bin/w", "7f00-7f01 r-xp 0 00:00 0   /var/capsule/c1/merged/nix/store/aaa-go/lib/libx.so\n")
	fakeProc(t, e, 16, "/x/merged/nix/store/aaa-go-other/bin/z", "")
	got := e.busy("toolchain")
	if len(got) != 4 || got[0] != 10 || got[1] != 11 || got[2] != 14 || got[3] != 15 {
		t.Fatalf("busy = %v", got)
	}
	n := 0
	if code := run(e, []string{"evict", "toolchain"}, func(any) { n++ }); code != 2 || n != 0 {
		t.Fatalf("evict busy code=%d outputs=%d", code, n)
	}
}

func TestEvictIdleGrace(t *testing.T) {
	e := newEnv(t)
	layerFile(t, e, "toolchain", "aaa-go")
	layerFile(t, e, "office", "bbb-lo")
	layerFile(t, e, "delegate", "ccc-claude")
	if err := os.MkdirAll(e.stampDir, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	stamp := func(name string, at time.Time) {
		if err := os.WriteFile(filepath.Join(e.stampDir, name), []byte(strconv.FormatInt(at.Unix(), 10)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	stamp("go", now.Add(-time.Minute))
	stamp("soffice", now.Add(-time.Hour))
	fakeProc(t, e, 20, "/nix/store/ccc-claude/bin/claude", "")
	reasons := func(at time.Time) map[string]string {
		e.now = func() time.Time { return at }
		m := map[string]string{}
		if err := e.evictIdle(func(v any) { r := v.(result); m[r.Layer] = r.Reason }); err != nil {
			t.Fatal(err)
		}
		return m
	}
	got := reasons(now)
	if got["toolchain"] != "used within grace" || got["office"] != "idle" || got["delegate"] != "busy" {
		t.Fatalf("first pass: %v", got)
	}
	// The process ends; the busy sighting still holds the delegate for the grace.
	if err := os.RemoveAll(filepath.Join(e.procRoot, "20")); err != nil {
		t.Fatal(err)
	}
	if got := reasons(now.Add(time.Minute)); got["delegate"] != "used within grace" {
		t.Fatalf("after busy: %v", got)
	}
	if got := reasons(now.Add(11 * time.Minute)); got["delegate"] != "idle" || got["toolchain"] != "idle" {
		t.Fatalf("late: %v", got)
	}
}

func TestNoLayersExitsZero(t *testing.T) {
	e := newEnv(t)
	if code := run(e, []string{"evict-idle"}, func(any) { t.Fatal("output") }); code != 0 {
		t.Fatal(code)
	}
	if code := run(e, []string{"resident", "browser"}, func(any) {}); code == 0 {
		t.Fatal("unmounted layer accepted")
	}
}
