package capsule

import "fmt"

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
