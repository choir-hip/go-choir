package gomemlimit

import (
	"errors"
	"math"
	"reflect"
	"testing"
)

// Failure modes these tests guard (docs/archive/reports/lr-guest-memory-return-2026-10-14.md):
//   - a limit near the working set (GC thrash): fractions and floors are pinned;
//   - an operator's GOMEMLIMIT overridden by the computed one;
//   - a malformed GOMEMLIMIT handed to a worker (the Go runtime refuses to
//     start on one);

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

type limitRecorder struct {
	current int64
	calls   []int64
}

func (r *limitRecorder) set(v int64) int64 {
	prev := r.current
	r.calls = append(r.calls, v)
	if v >= 0 {
		r.current = v
	}
	return prev
}

func TestApplyDaemon_SetsHalfOfGuest(t *testing.T) {
	rec := &limitRecorder{current: math.MaxInt64}
	d := apply(RoleDaemon, fakeEnv(nil), fakeReadFile(meminfo8GiB, nil), rec.set)
	want := int64(8131820*1024) / 2
	if d.LimitBytes != want || rec.current != want {
		t.Fatalf("limit = %d (runtime %d), want %d; decision %+v", d.LimitBytes, rec.current, want, d)
	}
	if d.WorkerLimitBytes != LimitFor(RoleDeskWorker, int64(8131820*1024)) {
		t.Fatalf("worker limit = %d", d.WorkerLimitBytes)
	}
}

func TestApplyDaemon_EnvGOMEMLIMITWins(t *testing.T) {
	rec := &limitRecorder{current: 3 << 30} // the runtime already applied GOMEMLIMIT
	d := apply(RoleDaemon, fakeEnv(map[string]string{"GOMEMLIMIT": "3GiB"}), fakeReadFile(meminfo8GiB, nil), rec.set)
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
	d := apply(RoleDeskWorker, fakeEnv(nil), fakeReadFile("", errors.New("no such file")), rec.set)
	if d.LimitBytes != 0 || d.Skipped == "" || rec.current != math.MaxInt64 {
		t.Fatalf("decision %+v, runtime %d; want no limit", d, rec.current)
	}
}

func TestApplyWorker_SixteenthOfGuest(t *testing.T) {
	rec := &limitRecorder{current: math.MaxInt64}
	d := apply(RoleDeskWorker, fakeEnv(nil), fakeReadFile(meminfo8GiB, nil), rec.set)
	if want := int64(8131820*1024) / 16; d.LimitBytes != want || rec.current != want {
		t.Fatalf("worker limit = %d (runtime %d), want %d", d.LimitBytes, rec.current, want)
	}
}

// Failure modes for CHOIR_GOMEMLIMIT_FRACTION:
//   - a bad value (0, negative, >1, NaN, text) silently producing a tiny or
//     no limit: it must fall back to 0.5 and say so in the decision;
//   - the default changing: unset must equal the old RAM/2;
//   - a small fraction pushing the limit under the 512 MiB floor (GC thrash);
//   - the fraction affecting desk workers or overriding GOMEMLIMIT.

func TestParseFraction(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want float64
		note bool
	}{
		{"", 0.5, false}, {"0.25", 0.25, false}, {" 1 ", 1, false}, {"1.0", 1, false},
		{"0", 0.5, true}, {"-0.3", 0.5, true}, {"1.5", 0.5, true}, {"NaN", 0.5, true},
		{"Inf", 0.5, true}, {"half", 0.5, true},
	} {
		got, note := ParseFraction(tc.in)
		if got != tc.want || (note != "") != tc.note {
			t.Fatalf("ParseFraction(%q) = %v, %q; want %v note=%v", tc.in, got, note, tc.want, tc.note)
		}
	}
}

func TestDaemonLimitForFloorAndCap(t *testing.T) {
	total := int64(8131820 * 1024)
	if got := DaemonLimitFor(total, 0.5); got != total/2 {
		t.Fatalf("default = %d", got)
	}
	if got := DaemonLimitFor(total, 0.25); got != int64(float64(total)*0.25) {
		t.Fatalf("quarter = %d", got)
	}
	if got := DaemonLimitFor(total, 0.01); got != daemonFloor {
		t.Fatalf("floor = %d, want %d", got, int64(daemonFloor))
	}
	if got := DaemonLimitFor(total, 1); got != total {
		t.Fatalf("full = %d", got)
	}
	if got := DaemonLimitFor(256*mib, 0.5); got != 256*mib {
		t.Fatalf("floor capped to machine = %d", got)
	}
}

func TestApplyDaemonFractionEnv(t *testing.T) {
	total := int64(8131820 * 1024)
	for _, tc := range []struct {
		env  string
		want int64
		note bool
	}{{"", total / 2, false}, {"0.25", int64(float64(total) * 0.25), false}, {"2", total / 2, true}} {
		rec := &limitRecorder{current: math.MaxInt64}
		d := apply(RoleDaemon, fakeEnv(map[string]string{FractionEnv: tc.env}), fakeReadFile(meminfo8GiB, nil), rec.set)
		if d.LimitBytes != tc.want || rec.current != tc.want || (d.FractionNote != "") != tc.note {
			t.Fatalf("env %q: %+v runtime %d, want %d", tc.env, d, rec.current, tc.want)
		}
	}
	rec := &limitRecorder{current: math.MaxInt64}
	d := apply(RoleDeskWorker, fakeEnv(map[string]string{FractionEnv: "0.1"}), fakeReadFile(meminfo8GiB, nil), rec.set)
	if d.LimitBytes != LimitFor(RoleDeskWorker, total) {
		t.Fatalf("worker affected: %+v", d)
	}
	rec = &limitRecorder{current: math.MaxInt64}
	apply(RoleDaemon, fakeEnv(map[string]string{FractionEnv: "0.1", "GOMEMLIMIT": "3GiB"}), fakeReadFile(meminfo8GiB, nil), rec.set)
	if rec.current != math.MaxInt64 {
		t.Fatalf("GOMEMLIMIT overridden")
	}
}
