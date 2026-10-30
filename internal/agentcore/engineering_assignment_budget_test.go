package agentcore

import "testing"

// The engineering capsule's memory must hold the cmd/autoputer link and fit
// the default guest's capsule admission budget
// (docs/archive/problems/selfdev-drill-capsule-cannot-build-autoputer-2026-10-14.md,
// B5). Measured 2026-10-14 in a linux/arm64 container: the relink's largest
// process peaked at 1,891,288 KiB (1.80 GiB) with DWARF, 1,571,536 KiB
// (1.50 GiB) with -ldflags=-s -w; GOGC=50 did not lower it (1.75 GiB). At
// 1 GiB the drill's link ran 14+ minutes at 400-465 MB RSS (file pages
// reclaimed and re-faulted). The admission budget is 3/4 of the guest's
// MemTotal (internal/autoputer/capsule_executor_linux.go); the default guest
// is VM_MEM_MIB=4096 (nix/node-b.nix), and one engineering capsule is live at
// a time. Ways it could fail: the budget is below the link peak and the link
// thrashes or is killed; the budget exceeds what a default guest admits and
// every engineering spawn is refused.
func TestEngineeringCapsuleMemoryHoldsTheLinkAndFitsTheDefaultGuest(t *testing.T) {
	const strippedLinkPeak = int64(1571536) << 10
	if engineeringAssignmentMemoryMax < strippedLinkPeak {
		t.Fatalf("engineering capsule memory %d is below the measured stripped link peak %d", engineeringAssignmentMemoryMax, strippedLinkPeak)
	}
	// MemTotal of a 4096 MiB guest is a little below 4096 MiB (kernel
	// reservations); 3900 MiB is a conservative reading.
	const defaultGuestAdmission = int64(3900) << 20 * 3 / 4
	if engineeringAssignmentMemoryMax > defaultGuestAdmission {
		t.Fatalf("engineering capsule memory %d exceeds the default guest's admission budget %d", engineeringAssignmentMemoryMax, defaultGuestAdmission)
	}
}

// RES-04: under the doctrine capsule memory policy the request becomes the
// capsule's memory.high, and with no swap a memory.high below the link's
// anonymous peak throttles the build for minutes (the 1 GiB drill). The
// request must hold the DWARF link peak and admit what admission counted
// before RES-04, so the policy changes the ceiling, never what fits.
func TestEngineeringCapsuleMemoryRequestHoldsTheDWARFLink(t *testing.T) {
	const dwarfLinkPeak = int64(1891288) << 10
	if engineeringAssignmentMemoryRequest < dwarfLinkPeak {
		t.Fatalf("engineering capsule memory request %d is below the measured DWARF link peak %d", engineeringAssignmentMemoryRequest, dwarfLinkPeak)
	}
	if engineeringAssignmentMemoryRequest != engineeringAssignmentMemoryMax {
		t.Fatalf("engineering capsule memory request %d differs from the admission before RES-04 (%d)", engineeringAssignmentMemoryRequest, engineeringAssignmentMemoryMax)
	}
}
