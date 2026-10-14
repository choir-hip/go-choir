package gomemlimit

import (
	"errors"
	"math"
	"reflect"
	"testing"

	doltmemlimit "github.com/dolthub/dolt/go/libraries/doltcore/memlimit"
)

// Failure modes these tests guard (docs/reports/lr-guest-memory-return-2026-10-14.md):
//   - a limit near the working set (GC thrash): fractions and floors are pinned;
//   - an operator's GOMEMLIMIT overridden by the computed one;
//   - a malformed GOMEMLIMIT handed to a worker (the Go runtime refuses to
//     start on one);
//   - the soft limit silently resizing Dolt's caches (Dolt reads the limit
//     once, at first use);
//   - the Dolt knob taking a budget other than the one asked for.

const meminfo8GiB = `MemTotal:        8131820 kB
MemFree:         6000000 kB
MemAvailable:    7000000 kB
`

func fakeEnv(kv map[string]string) func(string) string {
	return func(k string) string { return kv[k] }
}

func fakeReadFile(data string, err error) func(string) ([]byte, error) {
	return func(path string) ([]byte, error) {
		if path != meminfoPath {
			return nil, errors.New("unexpected path " + path)
		}
		if err != nil {
			return nil, err
		}
		return []byte(data), nil
	}
}

func TestParseMemTotal(t *testing.T) {
	got, err := parseMemTotal([]byte(meminfo8GiB))
	if err != nil || got != 8131820*1024 {
		t.Fatalf("parseMemTotal = %d, %v; want %d", got, err, int64(8131820*1024))
	}
	for _, bad := range []string{"", "MemFree: 10 kB\n", "MemTotal: x kB\n", "MemTotal: 0 kB\n", "MemTotal: 10 MB\n"} {
		if v, err := parseMemTotal([]byte(bad)); err == nil {
			t.Fatalf("parseMemTotal(%q) = %d, want error", bad, v)
		}
	}
}

func TestLimitFor(t *testing.T) {
	const gib = int64(1) << 30
	cases := []struct {
		role  Role
		total int64
		want  int64
	}{
		{RoleDaemon, 8 * gib, 4 * gib},
		{RoleDaemon, 4 * gib, 2 * gib},
		{RoleDaemon, 768 * mib, 512 * mib}, // floor
		{RoleDaemon, 256 * mib, 256 * mib}, // floor never exceeds the machine
		{RoleDeskWorker, 8 * gib, 512 * mib},
		{RoleDeskWorker, 16 * gib, 1 * gib},
		{RoleDeskWorker, 2 * gib, 256 * mib}, // floor
		{RoleDaemon, 0, 0},
		{RoleDeskWorker, -1, 0},
	}
	for _, tc := range cases {
		if got := LimitFor(tc.role, tc.total); got != tc.want {
			t.Fatalf("LimitFor(%s, %d) = %d, want %d", tc.role, tc.total, got, tc.want)
		}
	}
}

func TestValidGOMEMLIMIT(t *testing.T) {
	for _, ok := range []string{"off", "512MiB", "1GiB", "1073741824", "1B", "2TiB", "64KiB"} {
		if !ValidGOMEMLIMIT(ok) {
			t.Fatalf("ValidGOMEMLIMIT(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"", " ", "abc", "512MB", "-1", "1.5GiB", "GiB", "OFF", "512 MiB", "99999999999999999999"} {
		if ValidGOMEMLIMIT(bad) {
			t.Fatalf("ValidGOMEMLIMIT(%q) = true, want false", bad)
		}
	}
}

func TestWorkerEnv(t *testing.T) {
	if got := WorkerEnv(fakeEnv(nil)); got != nil {
		t.Fatalf("WorkerEnv(unset) = %v, want nil", got)
	}
	if got := WorkerEnv(fakeEnv(map[string]string{WorkerOverrideEnv: "off"})); !reflect.DeepEqual(got, []string{"GOMEMLIMIT=off"}) {
		t.Fatalf("WorkerEnv(off) = %v", got)
	}
	if got := WorkerEnv(fakeEnv(map[string]string{WorkerOverrideEnv: " 768MiB "})); !reflect.DeepEqual(got, []string{"GOMEMLIMIT=768MiB"}) {
		t.Fatalf("WorkerEnv(768MiB) = %v", got)
	}
	for _, refused := range []string{"768MB", "0", "1B", "32MiB"} {
		// Malformed values abort the worker's runtime; tiny ones make its GC
		// run without pause. Both are refused, and the worker computes its own.
		if got := WorkerEnv(fakeEnv(map[string]string{WorkerOverrideEnv: refused})); got != nil {
			t.Fatalf("WorkerEnv(%q) = %v, want nil", refused, got)
		}
	}
}

func TestDoltNodeCacheMiB(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"", 0, false},
		{"0", 0, false},
		{"x", 0, false},
		{"-5", 0, false},
		{"64", 64, true},
		{"8", 16, true},
		{"1000", 256, true},
	}
	for _, tc := range cases {
		got, ok := doltNodeCacheMiB(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Fatalf("doltNodeCacheMiB(%q) = %d,%v; want %d,%v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

type limitRecorder struct {
	current int64
	calls   []int64
	inits   []int64 // the limit Dolt saw at each init
}

func (r *limitRecorder) set(v int64) int64 {
	prev := r.current
	r.calls = append(r.calls, v)
	if v >= 0 {
		r.current = v
	}
	return prev
}

func (r *limitRecorder) doltInit() { r.inits = append(r.inits, r.current) }

func TestApplyDaemon_SetsHalfOfGuestAndPinsDoltDefaults(t *testing.T) {
	rec := &limitRecorder{current: math.MaxInt64}
	d := apply(RoleDaemon, fakeEnv(nil), fakeReadFile(meminfo8GiB, nil), rec.set, rec.doltInit, func() uint64 { return 256 * mib })
	want := int64(8131820*1024) / 2
	if d.LimitBytes != want || rec.current != want {
		t.Fatalf("limit = %d (runtime %d), want %d; decision %+v", d.LimitBytes, rec.current, want, d)
	}
	// Dolt saw no limit, exactly as before this lever: its caches keep their defaults.
	if !reflect.DeepEqual(rec.inits, []int64{math.MaxInt64}) {
		t.Fatalf("dolt init saw %v, want [MaxInt64]", rec.inits)
	}
	if d.DoltBudget != "default" || d.DoltNodeCacheBytes != 256*mib {
		t.Fatalf("dolt budget = %q %d", d.DoltBudget, d.DoltNodeCacheBytes)
	}
	if d.WorkerLimitBytes != LimitFor(RoleDeskWorker, int64(8131820*1024)) {
		t.Fatalf("worker limit = %d", d.WorkerLimitBytes)
	}
}

func TestApplyDaemon_EnvGOMEMLIMITWins(t *testing.T) {
	rec := &limitRecorder{current: 3 << 30} // the runtime already applied GOMEMLIMIT
	d := apply(RoleDaemon, fakeEnv(map[string]string{"GOMEMLIMIT": "3GiB"}), fakeReadFile(meminfo8GiB, nil), rec.set, rec.doltInit, func() uint64 { return 0 })
	if d.LimitBytes != 0 || d.Skipped == "" || rec.current != 3<<30 {
		t.Fatalf("decision %+v, runtime %d; want skipped with the env limit kept", d, rec.current)
	}
	for _, c := range rec.calls {
		if c >= 0 {
			t.Fatalf("SetMemoryLimit(%d) called with GOMEMLIMIT set", c)
		}
	}
}

func TestApply_NoMeminfo(t *testing.T) {
	rec := &limitRecorder{current: math.MaxInt64}
	d := apply(RoleDeskWorker, fakeEnv(nil), fakeReadFile("", errors.New("no such file")), rec.set, nil, nil)
	if d.LimitBytes != 0 || d.Skipped == "" || rec.current != math.MaxInt64 {
		t.Fatalf("decision %+v, runtime %d; want no limit", d, rec.current)
	}
}

func TestApplyWorker_SixteenthOfGuestNoDolt(t *testing.T) {
	rec := &limitRecorder{current: math.MaxInt64}
	d := apply(RoleDeskWorker, fakeEnv(nil), fakeReadFile(meminfo8GiB, nil), rec.set, rec.doltInit, nil)
	if want := int64(8131820*1024) / 16; d.LimitBytes != want || rec.current != want {
		t.Fatalf("worker limit = %d (runtime %d), want %d", d.LimitBytes, rec.current, want)
	}
	if len(rec.inits) != 0 {
		t.Fatalf("worker touched Dolt's budget")
	}
}

func TestApplyDaemon_DoltKnobOrder(t *testing.T) {
	rec := &limitRecorder{current: math.MaxInt64}
	d := apply(RoleDaemon, fakeEnv(map[string]string{DoltNodeCacheEnv: "64"}), fakeReadFile(meminfo8GiB, nil), rec.set, rec.doltInit, func() uint64 { return 64 * mib })
	transient := doltTransientLimit(64)
	if !reflect.DeepEqual(rec.inits, []int64{transient}) {
		t.Fatalf("dolt init saw %v, want [%d]", rec.inits, transient)
	}
	// transient, restore the previous (no limit), then the daemon's own.
	want := []int64{transient, math.MaxInt64, d.LimitBytes}
	if !reflect.DeepEqual(rec.calls, want) {
		t.Fatalf("SetMemoryLimit calls = %v, want %v", rec.calls, want)
	}
	if d.DoltBudget != DoltNodeCacheEnv+"=64" {
		t.Fatalf("dolt budget = %q", d.DoltBudget)
	}
}

// TestDoltTransientLimitAgainstRealDolt runs Dolt's own partition once (its
// Init is once per process, so this is the only test that calls it): the
// transient limit must give the node cache the asked-for MiB.
func TestDoltTransientLimitAgainstRealDolt(t *testing.T) {
	before := setMemoryLimit(-1)
	d := apply(RoleDaemon, fakeEnv(map[string]string{DoltNodeCacheEnv: "64"}), fakeReadFile("", errors.New("none")), setMemoryLimit, doltmemlimit.Init, doltmemlimit.NodeCacheSize)
	got := doltmemlimit.NodeCacheSize()
	if got < 64*mib || got > 64*mib+mib {
		t.Fatalf("dolt node cache = %d bytes, want 64 MiB (decision %+v)", got, d)
	}
	if mt := doltmemlimit.MemtableSize(); mt < 64*mib || mt > 64*mib+mib {
		t.Fatalf("dolt memtable = %d bytes, want about 64 MiB", mt)
	}
	if after := setMemoryLimit(-1); after != before {
		t.Fatalf("transient limit leaked into the process: %d, was %d", after, before)
	}
}
