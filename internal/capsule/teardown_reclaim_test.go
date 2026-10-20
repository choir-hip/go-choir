package capsule

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// Teardown reclaim (docs/design/visual-evidence-slice-2026-10-15.md V27,
// V28): an emptied capsule cgroup's page cache is dropped before the cgroup
// is removed, a partial or failed reclaim never blocks teardown, and the
// control-run switch turns it off.

type reclaimFake struct {
	fakeCgroupMemory
	reclaimErr error
	wrote      int64
}

func (f *reclaimFake) Reclaim(n int64) error {
	f.wrote = n
	f.calls = append(f.calls, "reclaim")
	f.current = max(f.reclaimTo, f.current-n)
	return f.reclaimErr
}

func TestTeardownReclaimAsksForEverythingTheCgroupIsChargedFor(t *testing.T) {
	const mib = int64(1 << 20)
	fake := &reclaimFake{fakeCgroupMemory: fakeCgroupMemory{current: 400 * mib, reclaimTo: 0}}
	report := teardownReclaim(fake)
	if fake.wrote != 400*mib {
		t.Fatalf("asked to reclaim %d, want the whole charge", fake.wrote)
	}
	for _, want := range []string{"current=400MiB->0MiB", "reclaim=ok"} {
		if !strings.Contains(report, want) {
			t.Errorf("report %q lacks %q", report, want)
		}
	}
}

func TestTeardownReclaimToleratesPartialAndFailedReclaim(t *testing.T) {
	const mib = int64(1 << 20)
	partial := &reclaimFake{fakeCgroupMemory: fakeCgroupMemory{current: 300 * mib, reclaimTo: 100 * mib}, reclaimErr: syscall.EAGAIN}
	if report := teardownReclaim(partial); !strings.Contains(report, "reclaim=partial") || !strings.Contains(report, "300MiB->100MiB") {
		t.Fatalf("EAGAIN is a partial reclaim, not a failure: %q", report)
	}
	denied := &reclaimFake{fakeCgroupMemory: fakeCgroupMemory{current: 300 * mib, reclaimTo: 300 * mib}, reclaimErr: os.ErrPermission}
	if report := teardownReclaim(denied); !strings.Contains(report, "reclaim=error:") {
		t.Fatalf("a failed reclaim must be reported and not panic: %q", report)
	}
}

func TestTeardownReclaimHasNothingToDoWithoutACharge(t *testing.T) {
	if report := teardownReclaim(nil); report != "" {
		t.Fatalf("no cgroup: %q", report)
	}
	empty := &reclaimFake{}
	if report := teardownReclaim(empty); !strings.Contains(report, "nothing") || empty.wrote != 0 {
		t.Fatalf("an uncharged cgroup was reclaimed: %q wrote=%d", report, empty.wrote)
	}
}

// A cgroup directory reports memory.peak and the file and anon charge, and
// reclaim goes to memory.reclaim.
func TestTeardownReclaimOverACgroupDirectory(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("memory.current", "524288000\n") // 500 MiB
	write("memory.peak", "629145600\n")    // 600 MiB
	write("memory.stat", "anon 1048576\nfile 419430400\nkernel 4096\nbogus\n")
	write("memory.reclaim", "")
	report := teardownReclaim(cgroupMemoryDir(dir))
	for _, want := range []string{"current=500MiB", "peak=600MiB", "file=400MiB", "anon=1MiB", "reclaim=ok"} {
		if !strings.Contains(report, want) {
			t.Errorf("report %q lacks %q", report, want)
		}
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "memory.reclaim")); strings.TrimSpace(string(got)) != "524288000" {
		t.Fatalf("memory.reclaim got %q", got)
	}
	// A missing memory.current is reported, never a failure of the caller.
	if report := teardownReclaim(cgroupMemoryDir(filepath.Join(dir, "gone"))); !strings.Contains(report, "unreadable") {
		t.Fatalf("missing cgroup: %q", report)
	}
}

func TestParseMemoryStatFileAnon(t *testing.T) {
	file, anon := parseMemoryStatFileAnon("anon 5\nfile 7\nfile_mapped 9\n")
	if file != 7 || anon != 5 {
		t.Fatalf("file=%d anon=%d", file, anon)
	}
	file, anon = parseMemoryStatFileAnon("")
	if file != -1 || anon != -1 {
		t.Fatalf("empty stat: file=%d anon=%d", file, anon)
	}
}

func TestTeardownReclaimSwitch(t *testing.T) {
	t.Setenv(teardownReclaimEnv, "")
	if !teardownReclaimEnabled() {
		t.Fatal("reclaim must be on by default")
	}
	for _, off := range []string{"off", " OFF "} {
		t.Setenv(teardownReclaimEnv, off)
		if teardownReclaimEnabled() {
			t.Fatalf("%q must switch reclaim off", off)
		}
	}
	t.Setenv(teardownReclaimEnv, "on")
	if !teardownReclaimEnabled() {
		t.Fatal("only off switches it off")
	}
}
