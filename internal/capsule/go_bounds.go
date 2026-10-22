//go:build linux

package capsule

import (
	"strconv"

	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Go bounds for capsule workloads
// (docs/problems/capsule-release-build-runs-at-the-2gib-ceiling-2026-10-22.md).
// The capsule masks /sys, so Go cannot read its cgroup's cpu.max and sizes
// GOMAXPROCS, and with it go build -p, to the guest's CPUs: four compiles
// resident at once inside one CPU of quota and 2 GiB with no swap. The
// executor names the quota as GOMAXPROCS and sets GOMEMLIMIT below the
// cgroup limit; the broker carries both to cells, jobs and direct exec
// (yaegikernel.CapsuleGoMaxProcsEnv), where a cell may override them.

// goMemLimitFor is the GOMEMLIMIT for a capsule whose throttling limit is
// limit: 5/8 of it, or 0 (no bound) when that is under
// yaegikernel.MinCarriedGoMemLimit, 64 MiB. The rest (768 MiB of 2 GiB) is for what the Go heap
// limit does not cover: the broker and session worker (about 150 MiB), the
// go command beside the one tool -p 1 runs, cgo's C compilers and the
// external linker, and page cache. GOMEMLIMIT is soft and per process: a
// live heap above it is slower, never killed for it.
func goMemLimitFor(limit int64) int64 {
	if bound := limit / 8 * 5; bound >= yaegikernel.MinCarriedGoMemLimit {
		return bound
	}
	return 0
}

// capsuleGoMemoryBase is the limit the bound sits under: memory.high when the
// doctrine policy set one (the limit that throttles), else memory.max.
func capsuleGoMemoryBase(caps *Capsule) int64 {
	if caps.MemoryHigh > 0 {
		return caps.MemoryHigh
	}
	return caps.MemoryMax
}

// goBoundsBrokerEnv is the carry for a capsule with CPU quota/period and a
// throttling memory limit: GOMAXPROCS is the quota in CPUs, rounded up, and
// GOMEMLIMIT goMemLimitFor(memory). A quota or memory of 0 or less sets no
// bound for it. A period of 0 is the cgroup default, 100000 (namespace.go).
func goBoundsBrokerEnv(quota, period, memory int64) []string {
	var env []string
	if period <= 0 {
		period = 100000
	}
	if quota > 0 {
		env = append(env, yaegikernel.CapsuleGoMaxProcsEnv+"="+strconv.FormatInt((quota+period-1)/period, 10))
	}
	if limit := goMemLimitFor(memory); limit > 0 {
		env = append(env, yaegikernel.CapsuleGoMemLimitEnv+"="+strconv.FormatInt(limit, 10))
	}
	return env
}

// capsuleBrokerGoBoundsEnv is what the executor adds to a new broker's
// environment for the spawned capsule.
func capsuleBrokerGoBoundsEnv(caps *Capsule) []string {
	return goBoundsBrokerEnv(caps.Spec.CpuQuota, caps.Spec.CpuPeriod, capsuleGoMemoryBase(caps))
}
