package capsule

import (
	"context"
	"fmt"
)

// Size classes at spawn (station S4,
// docs/design/capsule-heavy-work-scheduling-2026-11-03.md, point 1).
//
// CPU is compressible and memory is not, so heavy work is reserved once, at
// placement, against a declared class, and the capsule's jobs spend that
// reservation locally: no job asks anyone. A light capsule's editing
// footprint is already in its computer's settled resident memory, so it
// reserves nothing on the host. A build capsule reserves BuildPeakBytes on
// the host for its life (HostMemory.AdmitBuild), and a build spawn the host
// cannot cover is refused at spawn with the host's reason.

// SizeClass is a capsule's heavy-work class.
type SizeClass string

const (
	// SizeClassLight is editing: no host reservation. The zero value.
	SizeClassLight SizeClass = "light"
	// SizeClassBuild is a capsule that runs frontend builds, Go links and
	// Playwright: it reserves one build's peak on the host.
	SizeClassBuild SizeClass = "build"
)

// BuildPeakBytes is the guest growth one build-class capsule's heavy work
// was measured to add: a frontend build plus Playwright used about 3 GB of
// a 7.9 GB guest (docs/problems/capsule-frontend-build-hangs-b-c-runtime-2026-10-27.md),
// and four concurrent arm builds added about 12 GB on Node B
// (2026-11-03 00:46Z, docs/design/capsule-heavy-work-scheduling-2026-11-03.md).
const BuildPeakBytes = int64(3 << 30)

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

// HostReservationBytes is what a capsule of this class reserves on the host.
func (c SizeClass) HostReservationBytes() int64 {
	if c.Normalized() == SizeClassBuild {
		return BuildPeakBytes
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

// HostMemory is the host's side of capsule heavy work (S4): the guest's
// report of its live build reservations to vmctl, and the host facts that
// come back. A nil HostMemory (no vmctl, a test) runs as before: nothing is
// reserved and no job waits.
type HostMemory interface {
	// AdmitBuild asks the host to reserve bytes for a build capsule about
	// to spawn. A *HostMemoryRefusal is the host's refusal, which refuses
	// the spawn; any other error is logged and the spawn goes on.
	AdmitBuild(ctx context.Context, capsuleID string, bytes int64) error
	// ReleaseBuild drops a capsule's reservation (its spawn failed or it
	// ended). It never blocks.
	ReleaseBuild(capsuleID string)
	// Facts is the host's latest facts and whether they are fresh enough
	// to act on.
	Facts() (HostMemoryFacts, bool)
}

// HostMemoryRefusal is the host refusing a build spawn, with its reason.
type HostMemoryRefusal struct{ Reason string }

func (r *HostMemoryRefusal) Error() string {
	return "host memory refused the build capsule: " + r.Reason
}

// HostMemoryFacts is what the host publishes for heavy work: MemAvailable
// over the floor and the burst pool less what is promised to others (the
// room a build spawn is admitted against), and PSI memory pressure.
type HostMemoryFacts struct {
	HeadroomBytes   int64
	MemorySomeAvg10 float64
	PressureHigh    bool
	// Protected is the paper or the owner's computer: its jobs never wait.
	Protected bool
}

// HostMemoryWaitReason says why a build-class job should wait now, or ""
// to start it. Facts that are not fresh start it, as before S4; the host
// floor stays the hard stop.
func HostMemoryWaitReason(facts HostMemoryFacts, fresh bool) string {
	return HostMemoryWaitReasonFor(facts, fresh, BuildPeakBytes)
}

// JobHostNeedBytes is the host headroom a build-class job waits for: what it
// declared in memory_mib, never more than one build's peak. A job that
// declares nothing keeps the BuildPeakBytes claim, so a build stays safe; a
// job that declares its size (a screenshot run, about 1.5 GiB) does not queue
// behind other computers' builds for memory it does not use.
func JobHostNeedBytes(memoryMiB int) int64 {
	if memoryMiB <= 0 {
		return BuildPeakBytes
	}
	if need := int64(memoryMiB) << 20; need < BuildPeakBytes {
		return need
	}
	return BuildPeakBytes
}

// HostMemoryWaitReasonFor is HostMemoryWaitReason for a job that needs need
// bytes of host headroom.
func HostMemoryWaitReasonFor(facts HostMemoryFacts, fresh bool, need int64) string {
	if !fresh || facts.Protected {
		return ""
	}
	if facts.PressureHigh {
		return fmt.Sprintf("waiting for host memory: the host is under memory pressure (PSI some avg10 %.1f%%)", facts.MemorySomeAvg10)
	}
	if facts.HeadroomBytes < need {
		return fmt.Sprintf("waiting for host memory: %d MiB over the host floor and burst pool, under this job's %d MiB", facts.HeadroomBytes>>20, need>>20)
	}
	return ""
}
