package vmmanager

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// D2 S5 item 2, the CPU fingerprint (design 1.2 and S5; review 306 DeepSeek
// finding 5). Nothing guarded a CPUID change under a resumed guest (a
// microcode or BIOS change without a kernel change). The fingerprint is a
// sha256 over the sorted, de-duplicated (vendor_id, model name, flags,
// microcode) of EVERY processor, so a change on any core moves it.
//
// Failure modes, written first:
//   - only the first core is read, so a mixed-microcode host is missed;
//   - an unreadable cpuinfo at hibernate or wake lets a release-bound snapshot
//     load on an unknown CPU (fail-open);
//   - a legacy snapshot (no fingerprint) is voided by the upgrade;
//   - two identical cores make the fingerprint depend on the core count.

const cpuBlock = "processor\t: %d\nvendor_id\t: GenuineIntel\nmodel name\t: Xeon Test\nmicrocode\t: %s\nflags\t\t: fpu vme sse4_2 avx2\ncache size\t: 1024 KB\n\n"

func writeCPUInfo(t *testing.T, microcodes ...string) string {
	t.Helper()
	var b strings.Builder
	for i, mc := range microcodes {
		b.WriteString(strings.Replace(strings.Replace(cpuBlock, "%d", string(rune('0'+i)), 1), "%s", mc, 1))
	}
	path := filepath.Join(t.TempDir(), "cpuinfo")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func fingerprintOf(t *testing.T, path string) string {
	t.Helper()
	prev := cpuInfoPath
	cpuInfoPath = path
	defer func() { cpuInfoPath = prev }()
	fp, err := hostCPUFingerprint()
	if err != nil {
		t.Fatalf("hostCPUFingerprint(%s): %v", path, err)
	}
	return fp
}

func TestHostCPUFingerprint(t *testing.T) {
	same4 := fingerprintOf(t, writeCPUInfo(t, "0xa", "0xa", "0xa", "0xa"))
	same2 := fingerprintOf(t, writeCPUInfo(t, "0xa", "0xa"))
	if same4 != same2 {
		t.Fatal("identical cores must de-duplicate: the fingerprint depends on the core count")
	}
	lastCore := fingerprintOf(t, writeCPUInfo(t, "0xa", "0xa", "0xa", "0xb"))
	if lastCore == same4 {
		t.Fatal("a microcode change on the last core did not move the fingerprint")
	}
	again := fingerprintOf(t, writeCPUInfo(t, "0xa", "0xa", "0xa", "0xa"))
	if again != same4 || len(same4) != 64 {
		t.Fatalf("fingerprint is not stable or not a sha256 hex: %q", same4)
	}
	prev := cpuInfoPath
	defer func() { cpuInfoPath = prev }()
	cpuInfoPath = filepath.Join(t.TempDir(), "nope")
	if _, err := hostCPUFingerprint(); err == nil {
		t.Fatal("an unreadable cpuinfo must be an error")
	}
	empty := filepath.Join(t.TempDir(), "empty")
	_ = os.WriteFile(empty, []byte("nothing useful\n"), 0o644)
	cpuInfoPath = empty
	if _, err := hostCPUFingerprint(); err == nil {
		t.Fatal("a cpuinfo with no processor is an error, never an empty fingerprint")
	}
}

// stubHostCPU replaces the reader for the test.
func stubHostCPU(t *testing.T, fp *string, err *error) {
	t.Helper()
	prev := readHostCPU
	readHostCPU = func() (string, error) {
		if err != nil && *err != nil {
			return "", *err
		}
		return *fp, nil
	}
	t.Cleanup(func() { readHostCPU = prev })
}

func TestSnapshotRefusesAChangedOrUnknownHostCPU(t *testing.T) {
	cases := []struct {
		name        string
		atHiber     string
		hiberErr    error
		atWake      string
		wakeErr     error
		releaseMeta bool
		want        string
	}{
		{"same cpu", "cpu-a", nil, "cpu-a", nil, true, ""},
		{"changed, release-bound", "cpu-a", nil, "cpu-b", nil, true, snapshotReasonHostCPUMismatch},
		{"changed, legacy", "cpu-a", nil, "cpu-b", nil, false, snapshotReasonHostCPUMismatch},
		{"unreadable at hibernate, release-bound", "", os.ErrNotExist, "cpu-a", nil, true, snapshotReasonHostCPUUnknown},
		{"unreadable at wake, release-bound", "cpu-a", nil, "", os.ErrNotExist, true, snapshotReasonHostCPUUnknown},
		{"unreadable at hibernate, legacy (as today)", "", os.ErrNotExist, "cpu-a", nil, false, ""},
		{"unreadable at wake, legacy (as today)", "cpu-a", nil, "", os.ErrNotExist, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var fp string
			var ferr error
			var dir string
			var meta *SnapshotMeta
			var validate func() string
			if tc.releaseMeta {
				r := newReleaseFixture(t)
				stubHostCPU(t, &fp, &ferr) // after the fixture's own stub
				fp, ferr = tc.atHiber, tc.hiberErr
				d, m := r.hibernate(t)
				dir, meta = d, m
				validate = func() string { return r.validate(dir, meta) }
			} else {
				f := newSnapshotFixture(t)
				stubHostCPU(t, &fp, &ferr)
				fp, ferr = tc.atHiber, tc.hiberErr
				f.serveLaunchAPI(t)
				if err := f.mgr.HibernateVM(f.vmID); err != nil {
					t.Fatal(err)
				}
				dir = f.mgr.snapshotDir(f.vmID)
				meta, _ = readSnapshotMeta(dir)
				validate = func() string {
					v := snapshotValidation{Now: time.Now(), FirecrackerVersion: meta.FirecrackerVersion, HostKernelRelease: meta.HostKernelRelease, Expect: f.expect}
					return f.mgr.validateSnapshot(dir, meta, v)
				}
			}
			fp, ferr = tc.atWake, tc.wakeErr
			if got := validate(); got != tc.want {
				t.Fatalf("validateSnapshot = %q, want %q (meta.HostCPU=%q)", got, tc.want, meta.HostCPU)
			}
		})
	}
}
