package capsule

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Job memory, platform-neutral parts (docs/reports/memory-hotplug-
// controller-design-2026-10-14.md section 9.4): the live budget (F18) and
// lowering a capsule's cgroup without an OOM kill (J7).

type fakeCgroupMemory struct {
	current   int64
	reclaimTo int64 // the usage memory.reclaim can reach
	max       int64
	calls     []string
	setErr    error
}

func (f *fakeCgroupMemory) Current() (int64, error) {
	f.calls = append(f.calls, "current")
	return f.current, nil
}

func (f *fakeCgroupMemory) Reclaim(n int64) error {
	f.calls = append(f.calls, "reclaim")
	f.current = max(f.reclaimTo, f.current-n)
	return nil
}

func (f *fakeCgroupMemory) SetMax(n int64) error {
	f.calls = append(f.calls, "max")
	if f.setErr != nil {
		return f.setErr
	}
	f.max = n
	return nil
}

func TestLowerCgroupMemory(t *testing.T) {
	const gib = int64(1 << 30)
	// Usage already under the base: lowered at once, no reclaim.
	cg := &fakeCgroupMemory{current: gib, max: 3 * gib}
	if ok, err := lowerCgroupMemory(cg, 2*gib); !ok || err != nil || cg.max != 2*gib || strings.Join(cg.calls, ",") != "current,max" {
		t.Fatalf("under the base: ok=%t err=%v max=%d calls=%v", ok, err, cg.max, cg.calls)
	}
	// Reclaimable page cache over the base: reclaimed, then lowered.
	cg = &fakeCgroupMemory{current: 2*gib + gib/2, reclaimTo: gib, max: 3 * gib}
	if ok, err := lowerCgroupMemory(cg, 2*gib); !ok || err != nil || cg.max != 2*gib || strings.Join(cg.calls, ",") != "current,reclaim,current,max" {
		t.Fatalf("reclaimable: ok=%t err=%v max=%d calls=%v", ok, err, cg.max, cg.calls)
	}
	// Unreclaimable usage over the base: never lowered (that would OOM-kill
	// inside the capsule); the caller retains the boost.
	cg = &fakeCgroupMemory{current: 2*gib + gib/2, reclaimTo: 2*gib + gib/4, max: 3 * gib}
	if ok, err := lowerCgroupMemory(cg, 2*gib); ok || err != nil || cg.max != 3*gib {
		t.Fatalf("unreclaimable: ok=%t err=%v max=%d", ok, err, cg.max)
	}
	// A write that fails is reported, not lowered.
	cg = &fakeCgroupMemory{current: gib, max: 3 * gib, setErr: errors.New("gone")}
	if ok, err := lowerCgroupMemory(cg, 2*gib); ok || err == nil {
		t.Fatalf("failed write: ok=%t err=%v", ok, err)
	}
}

func TestCgroupMemoryDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "memory.current"), []byte("1048576\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Control files exist in a real cgroup; the writer opens, never creates.
	for _, name := range []string{"memory.max", "memory.reclaim"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cg := cgroupMemoryDir(dir)
	if n, err := cg.Current(); err != nil || n != 1<<20 {
		t.Fatalf("current = %d %v", n, err)
	}
	if err := cg.SetMax(3 << 30); err != nil {
		t.Fatal(err)
	}
	if raw, _ := os.ReadFile(filepath.Join(dir, "memory.max")); string(raw) != "3221225472" {
		t.Fatalf("memory.max = %q", raw)
	}
	if err := cg.Reclaim(1 << 20); err != nil {
		t.Fatal(err)
	}
	if raw, _ := os.ReadFile(filepath.Join(dir, "memory.reclaim")); string(raw) != "1048576" {
		t.Fatalf("memory.reclaim = %q", raw)
	}
}

// F18: the budget is 3/4 of MemTotal as it is now, so a plugged lease
// raises it and an unplug lowers it; an unreadable meminfo reads 0 (the
// executor then keeps its boot budget).
func TestLiveMemoryBudget(t *testing.T) {
	meminfo := "MemTotal:        4000000 kB\nMemFree:  100 kB\n"
	read := func(string) ([]byte, error) { return []byte(meminfo), nil }
	budget := LiveMemoryBudget(read)
	if got, want := budget(), int64(4000000*1024*3/4); got != want {
		t.Fatalf("budget = %d, want %d", got, want)
	}
	meminfo = "MemTotal:        5048576 kB\n"
	if got, want := budget(), int64(5048576*1024*3/4); got != want {
		t.Fatalf("budget after a plug = %d, want %d", got, want)
	}
	if got := LiveMemoryBudget(func(string) ([]byte, error) { return nil, os.ErrNotExist })(); got != 0 {
		t.Fatalf("unreadable = %d", got)
	}
}
