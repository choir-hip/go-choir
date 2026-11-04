package autoputer

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// The guest reporter. Failure modes pinned:
//   - G1 a host refusal does not refuse the spawn or the job, loses its
//     wait data, or leaves the entry in the live set;
//   - G2 a vmctl that cannot be asked refuses spawns or jobs (the guest
//     must run as before S4);
//   - G3 a released capsule or job lease stays in the reported set; a
//     released capsule keeps its jobs' leases;
//   - G4 a thinking capsule is reported at a stale baseline, or a leased
//     capsule's baseline moves under its lease.

type fakeHostReporter struct {
	mu      sync.Mutex
	reports []vmctl.CapsuleReservationReport
	refuse  bool
	err     error
}

func (f *fakeHostReporter) ReportCapsuleReservations(_ context.Context, r vmctl.CapsuleReservationReport) (vmctl.CapsuleReservationResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reports = append(f.reports, r)
	if f.err != nil {
		return vmctl.CapsuleReservationResult{}, f.err
	}
	res := vmctl.CapsuleReservationResult{}
	if r.Admit != nil {
		ok := !f.refuse
		res.Admitted = &ok
		if !ok {
			res.Reason = "host memory available 5120 MiB"
			res.Wait = &vmctl.CapsuleMemoryWait{NeedBytes: 3 << 30, Holders: []vmctl.CapsuleMemoryHolder{{Kind: "job", Class: "test", Bytes: 3 << 30}}}
		}
	}
	return res, nil
}

func (f *fakeHostReporter) last() vmctl.CapsuleReservationReport {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reports[len(f.reports)-1]
}

func entries(r vmctl.CapsuleReservationReport) map[string]uint64 {
	out := map[string]uint64{}
	for _, c := range r.Reservations {
		out[c.CapsuleID+"/"+c.JobID] = c.Bytes
	}
	return out
}

func TestCapsuleHostMemoryAdmitRefuseRelease(t *testing.T) {
	client := &fakeHostReporter{}
	h := newCapsuleHostMemory(client, "computer-a")

	if err := h.AdmitBuild(context.Background(), "cap-a", capsule.BuildBaselineBytes); err != nil {
		t.Fatalf("admitted build: %v", err)
	}
	client.refuse = true
	err := h.AdmitBuild(context.Background(), "cap-b", capsule.BuildBaselineBytes)
	var refusal *capsule.HostMemoryRefusal
	if !errors.As(err, &refusal) || refusal.Reason == "" {
		t.Fatalf("G1: refusal not returned: %v", err)
	}
	if got := client.last(); len(got.Reservations) != 1 || got.Reservations[0].CapsuleID != "cap-a" || got.Admit.CapsuleID != "cap-b" {
		t.Fatalf("G1: report %+v", got)
	}
	// A refused job lease carries the host's wait data and is not live.
	err = h.AdmitJob(context.Background(), "cap-a", "job-1", 300<<20, 3<<30)
	if !errors.As(err, &refusal) || refusal.Wait == nil || len(refusal.Wait.Holders) != 1 || refusal.Wait.Holders[0].Kind != "job" {
		t.Fatalf("G1: job refusal %v", err)
	}
	if got := client.last(); got.Admit.JobID != "job-1" || entries(got)["cap-a/"] != 300<<20 {
		t.Fatalf("G1: job admit report %+v", got)
	}
	client.refuse = false
	if err := h.AdmitJob(context.Background(), "cap-a", "job-1", 300<<20, 3<<30); err != nil {
		t.Fatalf("admitted job: %v", err)
	}
	if !h.reportOnce(context.Background()) || entries(client.last())["cap-a/job-1"] != 3<<30 {
		t.Fatalf("the lease is not reported: %+v", client.last())
	}
	h.ReleaseJob("cap-a", "job-1")
	if !h.reportOnce(context.Background()) || len(entries(client.last())) != 1 {
		t.Fatalf("G3: the released lease is reported: %+v", client.last())
	}
	h.AdmitJob(context.Background(), "cap-a", "job-2", 0, gib3)
	h.ReleaseBuild("cap-a")
	if !h.reportOnce(context.Background()) || len(client.last().Reservations) != 0 {
		t.Fatalf("G3: the empty set was not reported: %+v", client.last())
	}
	if h.reportOnce(context.Background()) {
		t.Fatalf("an idle reporter keeps posting")
	}
}

const gib3 = int64(3 << 30)

func TestCapsuleHostMemoryRemeasuresOnlyCapsulesWithoutALease(t *testing.T) {
	client := &fakeHostReporter{}
	h := newCapsuleHostMemory(client, "computer-a")
	usage := map[string]int64{"cap-a": 700 << 20, "cap-b": 900 << 20}
	h.SetBaselineReader(func(id string) (int64, bool) { b, ok := usage[id]; return b, ok })
	h.AdmitBuild(context.Background(), "cap-a", capsule.BuildBaselineBytes)
	h.AdmitBuild(context.Background(), "cap-b", capsule.BuildBaselineBytes)
	h.AdmitJob(context.Background(), "cap-b", "job-b", 250<<20, gib3)
	h.reportOnce(context.Background())
	got := entries(client.last())
	if got["cap-a/"] != 700<<20 {
		t.Fatalf("G4: thinking capsule reported at %d MiB, want its usage", got["cap-a/"]>>20)
	}
	if got["cap-b/"] != 250<<20 || got["cap-b/job-b"] != uint64(gib3) {
		t.Fatalf("G4: leased capsule moved: %+v", got)
	}
}

func TestCapsuleHostMemoryUnreachableVMCTLDoesNotRefuse(t *testing.T) {
	client := &fakeHostReporter{err: vmctl.ErrCapsuleReservationsUnsupported}
	h := newCapsuleHostMemory(client, "computer-a")
	err := h.AdmitBuild(context.Background(), "cap-a", capsule.BuildBaselineBytes)
	var refusal *capsule.HostMemoryRefusal
	if err == nil || errors.As(err, &refusal) {
		t.Fatalf("G2: want a plain error the spawn logs, got %v", err)
	}
	if err := h.AdmitJob(context.Background(), "cap-a", "job-1", 0, gib3); err == nil || errors.As(err, &refusal) {
		t.Fatalf("G2: want a plain error the job logs, got %v", err)
	}
	// The capsule and its lease are live and re-reported at the next tick.
	if !h.reportOnce(context.Background()) || len(client.last().Reservations) != 2 {
		t.Fatalf("G2: live entries not re-reported: %+v", client.last())
	}
}
