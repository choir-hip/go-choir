package autoputer

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// The guest reporter (S4). Failure modes pinned:
//   - G1 a host refusal does not refuse the spawn, or leaves the capsule in
//     the live set;
//   - G2 a vmctl that cannot be asked refuses spawns (the guest must run as
//     before S4);
//   - G3 a released capsule stays in the reported set;
//   - G4 stale facts are acted on.

type fakeHostReporter struct {
	mu      sync.Mutex
	reports []vmctl.CapsuleReservationReport
	refuse  bool
	err     error
	facts   vmctl.HostMemoryFacts
}

func (f *fakeHostReporter) ReportCapsuleReservations(_ context.Context, r vmctl.CapsuleReservationReport) (vmctl.CapsuleReservationResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reports = append(f.reports, r)
	if f.err != nil {
		return vmctl.CapsuleReservationResult{}, f.err
	}
	res := vmctl.CapsuleReservationResult{Facts: f.facts}
	if r.Admit != nil {
		ok := !f.refuse
		res.Admitted = &ok
		if !ok {
			res.Reason = "host memory available 5120 MiB"
		}
	}
	return res, nil
}

func (f *fakeHostReporter) last() vmctl.CapsuleReservationReport {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reports[len(f.reports)-1]
}

func TestCapsuleHostMemoryAdmitRefuseRelease(t *testing.T) {
	client := &fakeHostReporter{facts: vmctl.HostMemoryFacts{HeadroomBytes: 7 << 30, Priority: vmctl.CapsulePriorityBatch}}
	h := newCapsuleHostMemory(client, "computer-a")
	now := time.Date(2026, 11, 3, 1, 0, 0, 0, time.UTC)
	h.now = func() time.Time { return now }

	if err := h.AdmitBuild(context.Background(), "cap-a", capsule.BuildPeakBytes); err != nil {
		t.Fatalf("admitted build: %v", err)
	}
	client.refuse = true
	err := h.AdmitBuild(context.Background(), "cap-b", capsule.BuildPeakBytes)
	var refusal *capsule.HostMemoryRefusal
	if !errors.As(err, &refusal) || refusal.Reason == "" {
		t.Fatalf("G1: refusal not returned: %v", err)
	}
	if got := client.last(); len(got.Reservations) != 1 || got.Reservations[0].CapsuleID != "cap-a" || got.Admit.CapsuleID != "cap-b" {
		t.Fatalf("G1: report %+v", got)
	}
	if facts, fresh := h.Facts(); !fresh || facts.HeadroomBytes != 7<<30 || facts.Protected {
		t.Fatalf("facts %+v fresh=%t", facts, fresh)
	}
	h.ReleaseBuild("cap-a")
	if !h.reportOnce(context.Background()) || len(client.last().Reservations) != 0 {
		t.Fatalf("G3: the empty set was not reported: %+v", client.last())
	}
	if h.reportOnce(context.Background()) {
		t.Fatalf("an idle reporter keeps posting")
	}
	now = now.Add(time.Minute)
	if _, fresh := h.Facts(); fresh {
		t.Fatalf("G4: minute-old facts read as fresh")
	}
}

func TestCapsuleHostMemoryUnreachableVMCTLDoesNotRefuse(t *testing.T) {
	client := &fakeHostReporter{err: vmctl.ErrCapsuleReservationsUnsupported}
	h := newCapsuleHostMemory(client, "computer-a")
	err := h.AdmitBuild(context.Background(), "cap-a", capsule.BuildPeakBytes)
	var refusal *capsule.HostMemoryRefusal
	if err == nil || errors.As(err, &refusal) {
		t.Fatalf("G2: want a plain error the spawn logs, got %v", err)
	}
	if _, fresh := h.Facts(); fresh {
		t.Fatalf("G2: facts fresh without a host answer")
	}
	// The capsule is live and re-reported at the next tick.
	if !h.reportOnce(context.Background()) || len(client.last().Reservations) != 1 {
		t.Fatalf("G2: live capsule not re-reported: %+v", client.last())
	}
}
