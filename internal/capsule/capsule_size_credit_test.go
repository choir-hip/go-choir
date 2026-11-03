package capsule

import (
	"strings"
	"testing"
)

// A build-class job spends its own capsule's admitted host reservation
// (docs/problems/capsule-reservation-idle-baseline-and-job-double-gate-2026-11-03.md,
// problem 2). Failure modes pinned:
//   - C1 a job whose need fits in its capsule's unrealised reservation
//     still waits for the whole need in host headroom (the double gate);
//   - C2 the credit counts what the capsule already uses, or more than the
//     host still holds for this computer's builds, or anything when the
//     capsule's usage cannot be read (an older vmctl sends no own figure:
//     no credit, as before);
//   - C3 the credit lifts the pressure gate.

func TestUnrealisedReservation(t *testing.T) {
	const gib = int64(1 << 30)
	for _, tc := range []struct {
		name          string
		usage         int64
		usageOK       bool
		hostOwn, want int64
	}{
		{"fresh capsule", 200 << 20, true, 3 * gib, 3*gib - 200<<20},
		{"grown capsule", 2560 << 20, true, 3 * gib, 512 << 20},
		{"past its reservation", 4 * gib, true, 3 * gib, 0},
		{"host holds less", 0, true, gib, gib},
		{"older vmctl", 0, true, 0, 0},
		{"usage unreadable", 0, false, 3 * gib, 0},
	} {
		if got := UnrealisedReservation(3*gib, tc.usage, tc.usageOK, tc.hostOwn); got != tc.want {
			t.Errorf("C2 %s: credit %d MiB, want %d", tc.name, got>>20, tc.want>>20)
		}
	}
}

func TestHostMemoryWaitReasonCredited(t *testing.T) {
	const gib = int64(1 << 30)
	facts := HostMemoryFacts{HeadroomBytes: gib}
	if got := HostMemoryWaitReasonCredited(facts, true, 3*gib, 2560<<20); got != "" {
		t.Fatalf("C1: 512 MiB beyond the reservation in 1024 MiB of headroom waits: %q", got)
	}
	got := HostMemoryWaitReasonCredited(facts, true, 3*gib, 512<<20)
	if !strings.Contains(got, "2560 MiB") || !strings.Contains(got, "512 MiB") {
		t.Fatalf("C1: the wait does not say what is beyond the capsule's reservation: %q", got)
	}
	if got := HostMemoryWaitReasonCredited(HostMemoryFacts{PressureHigh: true, HeadroomBytes: 8 * gib}, true, 3*gib, 3*gib); got == "" {
		t.Fatalf("C3: a credited job started under host memory pressure")
	}
	if HostMemoryWaitReasonCredited(facts, true, 3*gib, 0) != HostMemoryWaitReasonFor(facts, true, 3*gib) {
		t.Fatalf("no credit must read as before")
	}
}
