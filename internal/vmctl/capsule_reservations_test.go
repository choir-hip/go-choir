package vmctl

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Live build reservations (S4, capsule_reservations.go). Failure modes
// pinned here:
//   - B1 a live build is not part of the reserved term, so a start (or a
//     second build) is admitted into memory a build is about to take;
//   - B2 a build the host cannot cover is admitted anyway, or refused
//     without a reason;
//   - B3 a reservation outlives its capsule: a release, a stopped VM or a
//     guest that stops reporting leaves it charged for ever;
//   - B4 a guest charges more than its own ceiling can grow;
//   - B5 a protected computer's build is refused, or reserves nothing;
//   - B6 the facts count this computer's own reservation against it, or
//     miss the reservations promised to other computers;
//   - B7 a heartbeat of an already admitted capsule is re-admitted (and
//     refused) after the host filled up;
//   - B8 the facts a job waits on promise room the spawn gate refuses: the
//     gate keeps the burst pool above the floor, so the facts must too.

func buildFixture(t *testing.T) *admissionFixture {
	t.Helper()
	f := newAdmissionFixture(t, measuredAdmission())
	for _, id := range []string{"a", "b", "c"} {
		addOwnership(f.reg, &VMOwnership{VMID: "vm-" + id, ComputerID: "computer-" + id, UserID: "u-" + id,
			DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive,
			MemorySettledRSSBytes: 800 << 20})
		f.mgr.setRSS("vm-"+id, 800<<20)
	}
	f.reg.sampleVMMemory()
	return f
}

func admitBuild(t *testing.T, f *admissionFixture, computer, capsule string, held ...CapsuleReservation) CapsuleReservationResult {
	t.Helper()
	res, err := f.reg.ReportCapsuleReservations(CapsuleReservationReport{ComputerID: computer, Reservations: held,
		Admit: &CapsuleReservation{CapsuleID: capsule, Bytes: 3 * gib}})
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if res.Admitted == nil {
		t.Fatalf("no admission answer")
	}
	return res
}

func TestCapsuleBuildReservationsJoinReservedAndRefuseWhatTheHostCannotCover(t *testing.T) {
	f := buildFixture(t)
	// 12 GiB available, 4 GiB floor, 1 GiB headroom: two 3 GiB builds fit.
	f.setAvail(12 * gib)
	if res := admitBuild(t, f, "computer-a", "cap-a"); !*res.Admitted {
		t.Fatalf("first build refused: %s", res.Reason)
	}
	if got := f.reg.HostAdmissionSummary().ReservedBytes; got != 3*gib {
		t.Fatalf("B1: reserved = %d MiB, want 3072 (idle 800 + build 3072 - rss 800)", got>>20)
	}
	if res := admitBuild(t, f, "computer-b", "cap-b"); !*res.Admitted {
		t.Fatalf("second build refused: %s", res.Reason)
	}
	res := admitBuild(t, f, "computer-c", "cap-c")
	if *res.Admitted {
		t.Fatalf("B2: a third 3 GiB build admitted at 12 GiB available with 6 GiB reserved")
	}
	if !strings.Contains(res.Reason, "this build needs 3072 MiB") || !strings.Contains(res.Reason, "6144 MiB reserved") {
		t.Fatalf("B2: refusal reason %q", res.Reason)
	}
	// A start is charged against the same term (B1).
	if _, err := f.reg.admitHostMemory(hibernated("u-d", "vm-d", 2*gib), WarmnessClassPrimary); err == nil {
		t.Fatalf("B1: a 2 GiB start admitted beside 6 GiB of live builds at 12 GiB available")
	}
	recent := f.reg.HostAdmissionSummary().Recent
	if len(recent) == 0 || recent[len(recent)-2].Decision != "refuse_build" {
		t.Fatalf("B2: the refusal is not in admission.recent: %+v", recent)
	}
}

func TestCapsuleBuildReservationEndsWithItsCapsuleItsVMAndItsReports(t *testing.T) {
	f := buildFixture(t)
	f.setAvail(20 * gib)
	admitBuild(t, f, "computer-a", "cap-a")
	admitBuild(t, f, "computer-b", "cap-b")
	// Release: the guest reports the empty set.
	if _, err := f.reg.ReportCapsuleReservations(CapsuleReservationReport{ComputerID: "computer-a"}); err != nil {
		t.Fatal(err)
	}
	if got := f.reg.HostAdmissionSummary().ReservedBytes; got != 3*gib {
		t.Fatalf("B3: reserved %d MiB after cap-a ended, want 3072", got>>20)
	}
	// The VM stops: the sampler drops its set.
	f.mgr.rmu.Lock()
	delete(f.mgr.rss, "vm-b")
	f.mgr.rmu.Unlock()
	f.reg.sampleVMMemory()
	if got := f.reg.HostAdmissionSummary().ReservedBytes; got != 0 {
		t.Fatalf("B3: reserved %d MiB after vm-b stopped", got>>20)
	}
	// A guest that stops reporting expires after the in-flight TTL.
	admitBuild(t, f, "computer-c", "cap-c")
	*f.now = f.now.Add(16 * time.Minute)
	if got := f.reg.HostAdmissionSummary().ReservedBytes; got != 0 {
		t.Fatalf("B3: reserved %d MiB 16 min after the last report", got>>20)
	}
}

func TestCapsuleBuildReservationGrowsIntoItsChargeAndNeverPastTheCeiling(t *testing.T) {
	f := buildFixture(t)
	f.setAvail(20 * gib)
	admitBuild(t, f, "computer-a", "cap-a")
	// The build grows: 2.8 GiB of the 3.8 GiB charge is now resident.
	f.mgr.setRSS("vm-a", 2800<<20)
	f.reg.sampleVMMemory()
	if got := f.reg.HostAdmissionSummary().ReservedBytes; got != (800+3072-2800)<<20 {
		t.Fatalf("reserved %d MiB, want %d", got>>20, 800+3072-2800)
	}
	// B4: an 8 GiB guest at 7.5 GiB resident can grow 0.5 GiB at most.
	f.mgr.setRSS("vm-a", 7680<<20)
	f.reg.sampleVMMemory()
	if _, err := f.reg.ReportCapsuleReservations(CapsuleReservationReport{ComputerID: "computer-a",
		Reservations: []CapsuleReservation{{CapsuleID: "cap-a", Bytes: 3 * gib}, {CapsuleID: "cap-x", Bytes: 64 * gib}}}); err != nil {
		t.Fatal(err)
	}
	if got := f.reg.HostAdmissionSummary().ReservedBytes; got != 512<<20 {
		t.Fatalf("B4: reserved %d MiB past the ceiling, want 512", got>>20)
	}
}

func TestCapsuleBuildReservationProtectedAlwaysAdmittedAndCounted(t *testing.T) {
	f := buildFixture(t)
	f.reg.mu.Lock()
	f.reg.vmByID["vm-a"].WarmnessClass = WarmnessClassPublicPlatform
	f.reg.mu.Unlock()
	f.setAvail(5 * gib)
	res := admitBuild(t, f, "computer-a", "cap-a")
	if !*res.Admitted || res.Facts.Priority != CapsulePriorityProtected {
		t.Fatalf("B5: protected build refused or not protected: %+v", res)
	}
	if got := f.reg.HostAdmissionSummary().ReservedBytes; got != 3*gib {
		t.Fatalf("B5: protected build reserved %d MiB", got>>20)
	}
	if res := admitBuild(t, f, "computer-b", "cap-b"); *res.Admitted || res.Facts.Priority != CapsulePriorityBatch {
		t.Fatalf("a batch build admitted beside the protected one at 5 GiB: %+v", res)
	}
}

func TestCapsuleBuildFactsCountOthersNotSelf(t *testing.T) {
	f := buildFixture(t)
	f.setAvail(14 * gib)
	admitBuild(t, f, "computer-a", "cap-a")
	res := admitBuild(t, f, "computer-b", "cap-b")
	// For b: 14 - 4 floor - 1 pool - 3 (a's build) = 6 GiB; b's own 3 GiB
	// not taken.
	if res.Facts.ReservedOthersBytes != 3*gib || res.Facts.BurstPoolBytes != gib || res.Facts.HeadroomBytes != 6*gib {
		t.Fatalf("B6: facts %+v", res.Facts)
	}
	// B7: the host fills up; b's heartbeat still carries b, and a retried
	// admit of a held capsule stays admitted.
	f.setAvail(5 * gib)
	again := admitBuild(t, f, "computer-b", "cap-b", CapsuleReservation{CapsuleID: "cap-b", Bytes: 3 * gib})
	if !*again.Admitted {
		t.Fatalf("B7: a held capsule's admit was refused on a heartbeat")
	}
}

func TestCapsuleBuildFactsAgreeWithTheSpawnGate(t *testing.T) {
	// B8: with a's 3 GiB build live, c reads the facts and then asks to
	// spawn a 3 GiB build. At each availability the facts' headroom covers
	// the build exactly when the gate admits it.
	for _, avail := range []uint64{10 * gib, 11 * gib, 11*gib - 1<<20, 12 * gib} {
		f := buildFixture(t)
		f.setAvail(avail)
		admitBuild(t, f, "computer-a", "cap-a")
		facts, err := f.reg.ReportCapsuleReservations(CapsuleReservationReport{ComputerID: "computer-c"})
		if err != nil {
			t.Fatal(err)
		}
		fits := facts.Facts.HeadroomBytes >= 3*gib
		res := admitBuild(t, f, "computer-c", "cap-c")
		if fits != *res.Admitted {
			t.Fatalf("B8 at %d MiB available: facts headroom %d MiB (fits %t) but the gate admitted=%t (%s)",
				avail>>20, facts.Facts.HeadroomBytes>>20, fits, *res.Admitted, res.Reason)
		}
	}
}

func TestCapsuleReservationsHandlerBindsTheGuest(t *testing.T) {
	f := buildFixture(t)
	h := NewHandler(f.reg)
	body, _ := json.Marshal(CapsuleReservationReport{ComputerID: "computer-a", Reservations: []CapsuleReservation{}})
	req := httptest.NewRequest(http.MethodPost, capsuleReservationsPath, bytes.NewReader(body))
	req.RemoteAddr = "10.200.0.99:4000" // a tap address bound to no computer
	rec := httptest.NewRecorder()
	h.HandleCapsuleReservations(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("an unbound guest got %d", rec.Code)
	}
	req = httptest.NewRequest(http.MethodPost, capsuleReservationsPath, bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:4000"
	rec = httptest.NewRecorder()
	h.HandleCapsuleReservations(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"facts"`) {
		t.Fatalf("host caller: %d %s", rec.Code, rec.Body.String())
	}
}

// B9: the facts carry what the ledger still holds for this computer's own
// builds, so a job in a build capsule needs host headroom only beyond its
// capsule's unrealised reservation (problem 2 of
// docs/problems/capsule-reservation-idle-baseline-and-job-double-gate-2026-11-03.md).
func TestCapsuleBuildFactsCarryOwnOutstanding(t *testing.T) {
	f := buildFixture(t)
	f.setAvail(14 * gib)
	res := admitBuild(t, f, "computer-a", "cap-a")
	if res.Facts.OwnReservedBytes != 3*gib {
		t.Fatalf("B9: own reserved %d MiB, want 3072", res.Facts.OwnReservedBytes>>20)
	}
	// The build grows 2 GiB: the ledger holds 1 GiB more for it.
	f.mgr.setRSS("vm-a", 2848<<20)
	f.reg.sampleVMMemory()
	beat, err := f.reg.ReportCapsuleReservations(CapsuleReservationReport{ComputerID: "computer-a",
		Reservations: []CapsuleReservation{{CapsuleID: "cap-a", Bytes: 3 * gib}}})
	if err != nil {
		t.Fatal(err)
	}
	if beat.Facts.OwnReservedBytes != gib {
		t.Fatalf("B9: own reserved %d MiB after 2 GiB of growth, want 1024", beat.Facts.OwnReservedBytes>>20)
	}
	other, err := f.reg.ReportCapsuleReservations(CapsuleReservationReport{ComputerID: "computer-b"})
	if err != nil {
		t.Fatal(err)
	}
	if other.Facts.OwnReservedBytes != 0 {
		t.Fatalf("B9: a computer with no build is told it holds %d MiB", other.Facts.OwnReservedBytes>>20)
	}
}
