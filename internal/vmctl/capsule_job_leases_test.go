package vmctl

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// Host memory is held per job, not per capsule
// (docs/problems/build-capsule-holds-peak-while-agent-thinks-2026-11-04.md).
// Failure modes pinned here:
//   - L1 a capsule whose agent thinks (no job) holds more than its baseline,
//     so a spawn waits for memory nobody is using;
//   - L2 a job lease the host can cover is refused;
//   - L3 two leases that cannot both fit are both admitted, or the one
//     refused waits without naming who holds the memory;
//   - L4 another computer's holder leaks its ids;
//   - L5 a lease outlives its job, its guest's boot, its VM or its reports;
//   - L6 a job starts under host memory pressure, or a baseline is refused
//     for pressure alone.

const baseline = 128 << 20

func leaseReport(t *testing.T, f *admissionFixture, computer string, held []CapsuleReservation, admit *CapsuleReservation) CapsuleReservationResult {
	t.Helper()
	res, err := f.reg.ReportCapsuleReservations(CapsuleReservationReport{ComputerID: computer, Reservations: held, Admit: admit})
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	return res
}

func capsuleOf(id string) CapsuleReservation {
	return CapsuleReservation{CapsuleID: id, Bytes: baseline}
}

func jobLease(capsule, job string, bytes uint64) *CapsuleReservation {
	return &CapsuleReservation{CapsuleID: capsule, JobID: job, Bytes: bytes}
}

func TestThinkingCapsulesReserveOnlyTheirBaselines(t *testing.T) {
	f := buildFixture(t)
	// 9 GiB available, 4 GiB floor, 1 GiB pool: under per-capsule 3 GiB
	// reservations one arm fit; baselines fit every arm.
	f.setAvail(9 * gib)
	for _, c := range []string{"a", "b", "c"} {
		c1 := CapsuleReservation{CapsuleID: "cap-" + c + "1", Bytes: baseline}
		res := leaseReport(t, f, "computer-"+c, nil, &c1)
		if res.Admitted == nil || !*res.Admitted {
			t.Fatalf("L1: capsule spawn on computer-%s refused: %s", c, res.Reason)
		}
		c2 := CapsuleReservation{CapsuleID: "cap-" + c + "2", Bytes: baseline}
		if res := leaseReport(t, f, "computer-"+c, []CapsuleReservation{c1}, &c2); !*res.Admitted {
			t.Fatalf("L1: second capsule on computer-%s refused: %s", c, res.Reason)
		}
	}
	if got := f.reg.HostAdmissionSummary().ReservedBytes; got != 6*baseline {
		t.Fatalf("L1: six thinking capsules reserve %d MiB, want %d", got>>20, 6*baseline>>20)
	}
	// Once resident, a baseline holds nothing unrealised.
	f.mgr.setRSS("vm-a", 800<<20+2*baseline)
	f.reg.sampleVMMemory()
	if got := f.reg.HostAdmissionSummary().ReservedBytes; got != 4*baseline {
		t.Fatalf("L1: resident baselines still reserved: %d MiB", got>>20)
	}
}

func TestJobLeaseAdmittedWhenTheHostCoversIt(t *testing.T) {
	f := buildFixture(t)
	f.setAvail(9 * gib)
	held := []CapsuleReservation{capsuleOf("cap-a")}
	leaseReport(t, f, "computer-a", nil, &held[0])
	res := leaseReport(t, f, "computer-a", held, jobLease("cap-a", "job-1", 3*gib))
	if res.Admitted == nil || !*res.Admitted || res.Wait != nil {
		t.Fatalf("L2: a 3 GiB lease refused at 9 GiB available: %+v", res)
	}
	if got := f.reg.HostAdmissionSummary().ReservedBytes; got != baseline+3*gib {
		t.Fatalf("L2: reserved %d MiB, want baseline + 3072", got>>20)
	}
	recent := f.reg.HostAdmissionSummary().Recent
	if last := recent[len(recent)-1]; last.Decision != "admit_job" || last.NeedSource != "capsule_job" {
		t.Fatalf("L2: decision %+v", last)
	}
	// A retried admit of a held lease (the heartbeat carries it) stays
	// admitted after the host fills.
	f.setAvail(5 * gib)
	held = append(held, *jobLease("cap-a", "job-1", 3*gib))
	if again := leaseReport(t, f, "computer-a", held, jobLease("cap-a", "job-1", 3*gib)); !*again.Admitted {
		t.Fatalf("L2: a held lease re-refused")
	}
}

func TestTwoLeasesThatCannotBothFitOneWaitsNamingTheHolder(t *testing.T) {
	f := buildFixture(t)
	// 9 GiB - 4 floor - 1 pool = 4 GiB of room: one 3 GiB lease fits.
	f.setAvail(9 * gib)
	for _, c := range []string{"a", "b"} {
		cap := capsuleOf("cap-" + c)
		leaseReport(t, f, "computer-"+c, nil, &cap)
	}
	first := leaseReport(t, f, "computer-a", []CapsuleReservation{capsuleOf("cap-a")}, jobLease("cap-a", "job-a", 3*gib))
	second := leaseReport(t, f, "computer-b", []CapsuleReservation{capsuleOf("cap-b")}, jobLease("cap-b", "job-b", 3*gib))
	if !*first.Admitted || *second.Admitted {
		t.Fatalf("L3: first admitted=%t second admitted=%t; one 3 GiB lease fits in 4 GiB", *first.Admitted, *second.Admitted)
	}
	w := second.Wait
	if w == nil || w.NeedBytes != 3*gib || w.AvailableBytes != 9*gib || w.FloorBytes != 4*gib || w.PoolBytes != gib || len(w.Holders) == 0 {
		t.Fatalf("L3: wait data %+v", w)
	}
	// Reserved: a's lease and b's baseline (both unrealised); room is what
	// is left over the floor and the pool.
	if w.ReservedBytes != 3*gib+2*baseline || w.RoomBytes != 4*gib-3*gib-2*baseline {
		t.Fatalf("L3: reserved %d MiB room %d MiB", w.ReservedBytes>>20, w.RoomBytes>>20)
	}
	top := w.Holders[0]
	if top.Kind != "job" || top.Own || top.Bytes != 3*gib || top.Class == "" {
		t.Fatalf("L3: largest holder %+v", top)
	}
	// L4: another computer's holder carries no ids.
	for _, h := range w.Holders {
		if !h.Own && (h.CapsuleID != "" || h.JobID != "") {
			t.Fatalf("L4: another computer's ids leaked: %+v", h)
		}
	}
	if !strings.Contains(second.Reason, "this job needs 3072 MiB") || !strings.Contains(second.Reason, "the largest holder is a running job on another") {
		t.Fatalf("L3: reason %q", second.Reason)
	}
	recent := f.reg.HostAdmissionSummary().Recent
	if last := recent[len(recent)-1]; last.Decision != "refuse_job" {
		t.Fatalf("L3: decision %+v", last)
	}

	// Its own job is named: a second capsule on computer-a waits behind
	// job-a by id.
	held := []CapsuleReservation{capsuleOf("cap-a"), *jobLease("cap-a", "job-a", 3*gib), capsuleOf("cap-a2")}
	mine := leaseReport(t, f, "computer-a", held, jobLease("cap-a2", "job-c", 3*gib))
	if *mine.Admitted || mine.Wait == nil {
		t.Fatalf("L3: own second lease admitted: %+v", mine)
	}
	if top := mine.Wait.Holders[0]; !top.Own || top.CapsuleID != "cap-a" || top.JobID != "job-a" || !strings.Contains(mine.Reason, "job job-a in capsule cap-a on this computer") {
		t.Fatalf("L3: own holder %+v reason %q", top, mine.Reason)
	}
}

func TestLeaseEndsWithItsJobItsBootItsVMAndItsReports(t *testing.T) {
	f := buildFixture(t)
	f.setAvail(20 * gib)
	cap := capsuleOf("cap-a")
	leaseReport(t, f, "computer-a", nil, &cap)
	leaseReport(t, f, "computer-a", []CapsuleReservation{cap}, jobLease("cap-a", "job-1", 3*gib))
	// Job end: the guest's next report carries the capsule alone.
	leaseReport(t, f, "computer-a", []CapsuleReservation{cap}, nil)
	if got := f.reg.HostAdmissionSummary().ReservedBytes; got != baseline {
		t.Fatalf("L5: reserved %d MiB after the job ended, want the baseline", got>>20)
	}
	leaseReport(t, f, "computer-a", []CapsuleReservation{cap}, jobLease("cap-a", "job-2", 3*gib))

	// Guest death: the VM reboots (a new epoch) and a report from the old
	// realization's set is never inherited, even before the new guest
	// reports.
	f.reg.mu.Lock()
	f.reg.vmByID["vm-a"].Epoch++
	f.reg.mu.Unlock()
	f.reg.sampleVMMemory() // no report from the new boot yet
	if got := f.reg.HostAdmissionSummary().ReservedBytes; got != 0 {
		t.Fatalf("L5: reserved %d MiB after the VM rebooted, before its guest reported", got>>20)
	}
	if res := leaseReport(t, f, "computer-a", nil, nil); res.Admitted != nil {
		t.Fatalf("an empty report answered an admit")
	}
	if got := f.reg.HostAdmissionSummary().ReservedBytes; got != 0 {
		t.Fatalf("L5: reserved %d MiB after the guest's boot changed", got>>20)
	}
	// A set from the old epoch is dropped when the new boot first reports
	// a capsule: only the new boot's entries count.
	leaseReport(t, f, "computer-b", nil, &CapsuleReservation{CapsuleID: "cap-b", Bytes: baseline})
	leaseReport(t, f, "computer-b", []CapsuleReservation{capsuleOf("cap-b")}, jobLease("cap-b", "job-b", 3*gib))
	f.reg.mu.Lock()
	f.reg.vmByID["vm-b"].Epoch++
	f.reg.mu.Unlock()
	fresh := capsuleOf("cap-b-new")
	leaseReport(t, f, "computer-b", nil, &fresh)
	if got := f.reg.HostAdmissionSummary().ReservedBytes; got != baseline {
		t.Fatalf("L5: reserved %d MiB after vm-b's new boot spawned one capsule, want the baseline", got>>20)
	}
	// The VM stops: the sampler drops its set.
	f.mgr.rmu.Lock()
	delete(f.mgr.rss, "vm-b")
	f.mgr.rmu.Unlock()
	f.reg.sampleVMMemory()
	if got := f.reg.HostAdmissionSummary().ReservedBytes; got != 0 {
		t.Fatalf("L5: reserved %d MiB after vm-b stopped", got>>20)
	}
	// A guest that stops reporting expires after the in-flight TTL.
	capC := capsuleOf("cap-c")
	leaseReport(t, f, "computer-c", nil, &capC)
	leaseReport(t, f, "computer-c", []CapsuleReservation{capC}, jobLease("cap-c", "job-c", 3*gib))
	*f.now = f.now.Add(16 * time.Minute)
	if got := f.reg.HostAdmissionSummary().ReservedBytes; got != 0 {
		t.Fatalf("L5: reserved %d MiB 16 min after the last report", got>>20)
	}
}

func TestJobLeaseWaitsUnderHostPressureButABaselineDoesNot(t *testing.T) {
	f := buildFixture(t)
	f.reg.setPressureSamplerForTest(func(PressureReclaimConfig) HostPressureSample {
		return HostPressureSample{MemoryTotalBytes: 32 * gib, MemoryAvailableBytes: 20 * gib, MemorySomeAvg10: 40}
	})
	cap := capsuleOf("cap-a")
	if res := leaseReport(t, f, "computer-a", nil, &cap); !*res.Admitted {
		t.Fatalf("L6: a baseline refused for pressure: %s", res.Reason)
	}
	res := leaseReport(t, f, "computer-a", []CapsuleReservation{cap}, jobLease("cap-a", "job-1", gib))
	if *res.Admitted || res.Wait == nil || !res.Wait.PressureHigh || !strings.Contains(res.Reason, "memory pressure") {
		t.Fatalf("L6: a job admitted under pressure: %+v", res)
	}
	// A protected computer's job never waits.
	f.reg.mu.Lock()
	f.reg.vmByID["vm-b"].WarmnessClass = WarmnessClassPublicPlatform
	f.reg.mu.Unlock()
	capB := capsuleOf("cap-b")
	leaseReport(t, f, "computer-b", nil, &capB)
	if res := leaseReport(t, f, "computer-b", []CapsuleReservation{capB}, jobLease("cap-b", "job-b", gib)); !*res.Admitted {
		t.Fatalf("L6: a protected job waited: %s", res.Reason)
	}
}

// A job id is checked like a capsule id.
func TestJobLeaseIDsAreValidated(t *testing.T) {
	f := buildFixture(t)
	if _, err := f.reg.ReportCapsuleReservations(CapsuleReservationReport{ComputerID: "computer-a",
		Admit: jobLease("cap-a", " job\n", gib)}); err == nil {
		t.Fatal("a job id with a newline was accepted")
	}
}

// L3 under concurrency: two guests ask in the same instant for leases that
// cannot both fit; the ledger decides them one after the other, so exactly
// one is admitted.
func TestSimultaneousLeasesAreDecidedOneAfterTheOther(t *testing.T) {
	for i := 0; i < 50; i++ {
		f := buildFixture(t)
		f.setAvail(9 * gib)
		for _, c := range []string{"a", "b"} {
			cap := capsuleOf("cap-" + c)
			leaseReport(t, f, "computer-"+c, nil, &cap)
		}
		var wg sync.WaitGroup
		admitted := make([]bool, 2)
		start := make(chan struct{})
		for n, c := range []string{"a", "b"} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				res, err := f.reg.ReportCapsuleReservations(CapsuleReservationReport{ComputerID: "computer-" + c,
					Reservations: []CapsuleReservation{capsuleOf("cap-" + c)}, Admit: jobLease("cap-"+c, "job-"+c, 3*gib)})
				admitted[n] = err == nil && res.Admitted != nil && *res.Admitted
			}()
		}
		close(start)
		wg.Wait()
		if admitted[0] == admitted[1] {
			t.Fatalf("L3 run %d: admitted %v; exactly one 3 GiB lease fits in 4 GiB of room", i, admitted)
		}
	}
}

// A guest from before per-job leases still reads own_reserved_bytes for its
// job credit: its 3 GiB jobless capsule is reported back as its own, and
// not also as promised to others.
func TestLegacyGuestReadsItsOwnReservation(t *testing.T) {
	f := buildFixture(t)
	f.setAvail(14 * gib)
	old := CapsuleReservation{CapsuleID: "cap-a", Bytes: 3 * gib}
	res := leaseReport(t, f, "computer-a", nil, &old)
	if res.Facts.OwnReservedBytes != 3*gib || res.Facts.ReservedOthersBytes != 0 || res.Facts.HeadroomBytes != 9*gib {
		t.Fatalf("legacy facts %+v", res.Facts)
	}
}
