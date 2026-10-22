//go:build linux

package capsule

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The executor turns a capsule's CPU quota and memory into the Go bounds its
// broker carries to workloads
// (docs/problems/capsule-release-build-runs-at-the-2gib-ceiling-2026-10-22.md).
// Ways it could fail, written before the code:
//
//	H1. GOMAXPROCS rounds a fractional quota down (1.5 CPUs to 1) or to 0;
//	H2. GOMEMLIMIT leaves no headroom below the cgroup limit for the
//	    broker, the session worker, the go command, cgo's C compilers and
//	    page cache, or is computed from the doctrine ceiling (2 x request)
//	    instead of memory.high, the limit that throttles;
//	H3. a capsule with no quota or no memory gets a bound of 0, which Go
//	    reads as "no limit" (GOMEMLIMIT=0 is a 0-byte limit: GC forever);
//	H4. the broker process gets GOMAXPROCS or GOMEMLIMIT itself;
//	H5. a small capsule gets a GOMEMLIMIT under the 64 MiB floor.
func TestCapsuleGoBoundsEnv(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		quota, period, memory   int64
		wantProcs, wantMemLimit string
	}{
		{"engineering: one CPU, 2 GiB", 100000, 100000, 2 << 30, "1", "1342177280"},
		{"claude: two CPUs, 3 GiB", 200000, 100000, 3 << 30, "2", "2013265920"},
		{"H1: 1.5 CPUs rounds up", 150000, 100000, 2 << 30, "2", "1342177280"},
		{"H1: a sliver of a CPU is one", 10000, 100000, 2 << 30, "1", "1342177280"},
		{"period 0 is the cgroup default 100000", 100000, 0, 2 << 30, "1", "1342177280"},
		{"H3: no quota, no GOMAXPROCS", 0, 100000, 2 << 30, "", "1342177280"},
		{"H3: no memory, no GOMEMLIMIT", 100000, 100000, 0, "1", ""},
		{"H5: 5/8 of 64 MiB is under the floor", 100000, 100000, 64 << 20, "1", ""},
		{"H5: 5/8 of 128 MiB is above it", 100000, 100000, 128 << 20, "1", "83886080"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := goBoundsBrokerEnv(tc.quota, tc.period, tc.memory)
			got := map[string]string{}
			for _, kv := range env {
				k, v, _ := strings.Cut(kv, "=")
				got[k] = v
			}
			if got[yaegikernel.CapsuleGoMaxProcsEnv] != tc.wantProcs || got[yaegikernel.CapsuleGoMemLimitEnv] != tc.wantMemLimit {
				t.Fatalf("env = %q, want GOMAXPROCS %q GOMEMLIMIT %q", env, tc.wantProcs, tc.wantMemLimit)
			}
			// H4: carry names only.
			for k := range got {
				if k == "GOMAXPROCS" || k == "GOMEMLIMIT" {
					t.Fatalf("the broker itself is bounded: %q", env)
				}
			}
		})
	}
}

// H2: the bound is 5/8 of the limit that throttles: memory.high when the
// doctrine policy set one, else memory.max.
func TestCapsuleGoMemoryBase(t *testing.T) {
	observe := &Capsule{MemoryMax: 2 << 30}
	if got := capsuleGoMemoryBase(observe); got != 2<<30 {
		t.Fatalf("observe base = %d", got)
	}
	doctrine := &Capsule{MemoryMax: 4 << 30, MemoryHigh: 2 << 30}
	if got := capsuleGoMemoryBase(doctrine); got != 2<<30 {
		t.Fatalf("doctrine base = %d, want memory.high", got)
	}
	limit := goMemLimitFor(2 << 30)
	if headroom := int64(2<<30) - limit; headroom < 512<<20 {
		t.Fatalf("GOMEMLIMIT %d leaves %d MiB headroom under 2 GiB", limit, headroom>>20)
	}
}

// Hop executor to broker: the broker launch reads the spawned capsule's own
// quota and memory shape.
func TestCapsuleBrokerGoBoundsEnvReadsTheCapsule(t *testing.T) {
	caps := &Capsule{MemoryMax: 4 << 30, MemoryHigh: 2 << 30, Spec: SpawnSpec{CpuQuota: 100000, CpuPeriod: 100000}}
	got := strings.Join(capsuleBrokerGoBoundsEnv(caps), ",")
	want := yaegikernel.CapsuleGoMaxProcsEnv + "=1," + yaegikernel.CapsuleGoMemLimitEnv + "=1342177280"
	if got != want {
		t.Fatalf("broker bounds = %q, want %q", got, want)
	}
}
