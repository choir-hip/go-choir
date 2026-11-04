package capsule

import (
	"context"
	"fmt"

	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Size classes at spawn (station S4,
// docs/design/capsule-heavy-work-scheduling-2026-11-03.md, point 1), held
// per job (docs/problems/build-capsule-holds-peak-while-agent-thinks-2026-11-04.md).
//
// CPU is compressible and memory is not, so heavy work is reserved on the
// host. It is reserved while it can be used: a build capsule's spawn
// reserves its baseline (what its idle processes use, measured from its
// cgroup while no job runs), and each of its jobs leases its need from
// vmctl when it starts and gives it back when it ends. A job's capsule
// cgroup is held to its usage at the start plus the lease while it runs,
// so a job cannot grow past what the host admitted. A light capsule's
// editing footprint is already in its computer's settled resident memory,
// so it reserves nothing and its jobs lease nothing.

// SizeClass is a capsule's heavy-work class.
type SizeClass string

const (
	// SizeClassLight is editing: no host reservation. The zero value.
	SizeClassLight SizeClass = "light"
	// SizeClassBuild is a capsule that runs frontend builds, Go links and
	// Playwright: it reserves one build's peak on the host.
	SizeClassBuild SizeClass = "build"
)

// BuildPeakBytes is the guest growth one build-class job was measured to
// add, the lease a job that declares no memory_mib takes: a frontend build
// plus Playwright used about 3 GB of a 7.9 GB guest
// (docs/problems/capsule-frontend-build-hangs-b-c-runtime-2026-10-27.md),
// and four concurrent arm builds added about 12 GB on Node B
// (2026-11-03 00:46Z, docs/design/capsule-heavy-work-scheduling-2026-11-03.md).
const BuildPeakBytes = int64(3 << 30)

// BuildBaselineBytes is what a build capsule reserves at its spawn, before
// it has processes to measure: a capsule cgroup at rest, its broker and a
// shell, held 43 to 95 MiB (docs/reports/playwright-in-capsules-memory-2026-10-15.md).
// From its first report on, the capsule's measured usage replaces it.
const BuildBaselineBytes = int64(128 << 20)

// Normalized is the class with the empty value read as light.
func (c SizeClass) Normalized() SizeClass {
	if c == "" {
		return SizeClassLight
	}
	return c
}

// Validate refuses a class it does not know, so a typo never silently
// spawns a capsule without its reservation.
func (c SizeClass) Validate() error {
	switch c.Normalized() {
	case SizeClassLight, SizeClassBuild:
		return nil
	}
	return fmt.Errorf("capsule size class %q is not light or build", string(c))
}

// HostReservationBytes is what a capsule of this class reserves on the host
// at its spawn: a build capsule's baseline, nothing for a light one.
func (c SizeClass) HostReservationBytes() int64 {
	if c.Normalized() == SizeClassBuild {
		return BuildBaselineBytes
	}
	return 0
}

// CPU by weight (S4 point 2): CPU is compressible, so there is no CPU
// admission anywhere, only proportional weights. A build capsule's cgroup
// (and so its jobs) weighs below a light one, and the capsules aggregate
// weighs below the guest's own services, so a build slows under contention
// and the computer's interactive work does not.
const (
	lightCPUWeight    = uint64(100) // the cgroup v2 default
	buildCPUWeight    = uint64(50)
	capsulesCPUWeight = uint64(50)
)

// capsulesCPUWeightFile is the capsules aggregate's cpu.weight; a var so a
// test can point it elsewhere.
var capsulesCPUWeightFile = "/sys/fs/cgroup/capsule/cpu.weight"

// CPUWeight is the cgroup cpu.weight of a capsule of this class.
func (c SizeClass) CPUWeight() uint64 {
	if c.Normalized() == SizeClassBuild {
		return buildCPUWeight
	}
	return lightCPUWeight
}

// HostMemory is the host's side of capsule heavy work: the guest's report
// of its live build capsules' baselines and their jobs' leases to vmctl. A
// nil HostMemory (no vmctl, a test) runs as before: nothing is reserved and
// no job waits.
type HostMemory interface {
	// AdmitBuild asks the host to reserve a build capsule's baseline before
	// it spawns. A *HostMemoryRefusal is the host's refusal, which refuses
	// the spawn; any other error is logged and the spawn goes on.
	AdmitBuild(ctx context.Context, capsuleID string, baseline int64) error
	// ReleaseBuild drops a capsule's baseline and every lease of its jobs
	// (its spawn failed or it ended). It never blocks.
	ReleaseBuild(capsuleID string)
	// AdmitJob asks the host for a job's lease, synchronously, with its
	// capsule's baseline measured now (0: keep the last). A
	// *HostMemoryRefusal is the host's refusal: the job waits. Any other
	// error is logged and the job starts.
	AdmitJob(ctx context.Context, capsuleID, jobID string, baseline, lease int64) error
	// ReleaseJob gives a job's lease back. It never blocks.
	ReleaseJob(capsuleID, jobID string)
	// SetBaselineReader installs how the reporter measures a capsule's
	// baseline (its cgroup usage) while none of its jobs holds a lease.
	SetBaselineReader(func(capsuleID string) (int64, bool))
}

// HostMemoryRefusal is the host refusing a build spawn or a job lease, with
// its reason, and, for the waits a desk is shown, the refusal as data.
type HostMemoryRefusal struct {
	Reason string
	Wait   *yaegikernel.MemoryWait
}

func (r *HostMemoryRefusal) Error() string {
	return "host memory refused the build capsule: " + r.Reason
}

// JobHostNeedBytes is what a build-class job leases on the host: what it
// declared in memory_mib, or one build's peak when it declares nothing, so
// a build stays safe and a job that declares its size (a screenshot run,
// about 1.5 GiB) does not wait for memory it does not use.
func JobHostNeedBytes(memoryMiB int) int64 {
	if memoryMiB <= 0 {
		return BuildPeakBytes
	}
	return int64(memoryMiB) << 20
}
