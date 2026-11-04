package vmctl

import (
	"testing"
	"time"
)

// The idle baseline a build set is charged on top of
// (docs/problems/capsule-reservation-idle-baseline-and-job-double-gate-2026-11-03.md,
// problem 1). Failure modes pinned here:
//   - I1 a settled RSS from an earlier realization (a refresh onto a new
//     release) is trusted as idle, so the build is under-reserved by the
//     difference and the ledger overcommits;
//   - I2 a heartbeat overwrites the set's measured idle with the settled
//     RSS, undoing I1's fix one report later;
//   - I3 with no pre-capsule reading (vmctl restarted under a live build),
//     a stale settled RSS is trusted instead of the conservative
//     max(settled, rss); a fresh one is used as it is;
//   - I4 a settled RSS of this realization but older than the bound is
//     trusted;
//   - I5 an unadmitted boot (a deploy refresh) never re-measures the
//     settled RSS, so it stays the previous release's for the computer's
//     whole life; or it is re-measured before its settle window ends, or
//     while a build inflates it.

func idleFixture(t *testing.T, own *VMOwnership, rss uint64) *admissionFixture {
	t.Helper()
	f := newAdmissionFixture(t, measuredAdmission())
	own.UserID, own.DesktopID, own.Kind, own.State = "u-"+own.ComputerID, PrimaryDesktopID, VMKindInteractive, VMStateActive
	addOwnership(f.reg, own)
	f.mgr.setRSS(own.VMID, rss)
	f.reg.sampleVMMemory()
	f.setAvail(20 * gib)
	return f
}

func reservedNow(f *admissionFixture) uint64 { return f.reg.HostAdmissionSummary().ReservedBytes }

func TestBuildIdleIgnoresASettledRSSFromAnEarlierRealization(t *testing.T) {
	// The pool arm: 418 MiB settled on epoch 3, refreshed onto epoch 4,
	// idle at 1.8 GiB before its build.
	f := idleFixture(t, &VMOwnership{VMID: "vm-p", ComputerID: "computer-p", Epoch: 4,
		MemorySettledRSSBytes: 418 * mib, MemorySettledEpoch: 3,
		MemorySettledAt: time.Date(2026, 10, 10, 6, 0, 0, 0, time.UTC).Format(time.RFC3339)}, 1843*mib)
	admitBuild(t, f, "computer-p", "cap-p")
	if got := reservedNow(f); got != 3*gib {
		t.Fatalf("I1: reserved %d MiB, want 3072 (idle is the 1843 MiB pre-capsule reading)", got/mib)
	}
	// I2: the heartbeat keeps the measured idle.
	if _, err := f.reg.ReportCapsuleReservations(CapsuleReservationReport{ComputerID: "computer-p",
		Reservations: []CapsuleReservation{{CapsuleID: "cap-p", Bytes: 3 * gib}}}); err != nil {
		t.Fatal(err)
	}
	if got := reservedNow(f); got != 3*gib {
		t.Fatalf("I2: reserved %d MiB after a heartbeat, want 3072", got/mib)
	}
	views := f.reg.HostAdmissionSummary().Reservations
	if len(views) != 1 || views[0].IdleBytes != 1843*mib || views[0].IdleSource != "pre_capsule_rss" {
		t.Fatalf("I1: the view does not say what idle was charged: %+v", views)
	}
}

func TestBuildIdleWithoutAPreCapsuleReading(t *testing.T) {
	recent := time.Date(2026, 10, 10, 6, 30, 0, 0, time.UTC).Format(time.RFC3339)
	for _, tc := range []struct {
		name        string
		own         VMOwnership
		wantOutMiB  uint64
		wantSource  string
		failureMode string
	}{
		{"stale epoch", VMOwnership{Epoch: 4, MemorySettledRSSBytes: 418 * mib, MemorySettledEpoch: 3, MemorySettledAt: recent},
			3072, "max_settled_rss", "I3"},
		{"legacy unstamped", VMOwnership{Epoch: 4, MemorySettledRSSBytes: 418 * mib, MemorySettledAt: recent},
			3072, "max_settled_rss", "I3"},
		{"fresh", VMOwnership{Epoch: 4, MemorySettledRSSBytes: 1843 * mib, MemorySettledEpoch: 4, MemorySettledAt: recent},
			1843 + 3072 - 2560, "settled", "I3"},
		{"aged", VMOwnership{Epoch: 4, MemorySettledRSSBytes: 1843 * mib, MemorySettledEpoch: 4,
			MemorySettledAt: time.Date(2026, 10, 9, 7, 0, 0, 0, time.UTC).Format(time.RFC3339)},
			3072, "max_settled_rss", "I4"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			own := tc.own
			own.VMID, own.ComputerID = "vm-p", "computer-p"
			// vmctl restarted under a live build: the guest's heartbeat is
			// the first report, and the RSS already holds build growth.
			f := idleFixture(t, &own, 2560*mib)
			if _, err := f.reg.ReportCapsuleReservations(CapsuleReservationReport{ComputerID: "computer-p",
				Reservations: []CapsuleReservation{{CapsuleID: "cap-p", Bytes: 3 * gib}}}); err != nil {
				t.Fatal(err)
			}
			views := f.reg.HostAdmissionSummary().Reservations
			if len(views) != 1 || views[0].OutstandingBytes != tc.wantOutMiB*mib || views[0].IdleSource != tc.wantSource {
				t.Fatalf("%s: views %+v, want outstanding %d MiB from %s", tc.failureMode, views, tc.wantOutMiB, tc.wantSource)
			}
		})
	}
}

func TestBuildIdleTakesTheLargerOfAFreshSettleAndThePreCapsuleReading(t *testing.T) {
	recent := time.Date(2026, 10, 10, 6, 30, 0, 0, time.UTC).Format(time.RFC3339)
	f := idleFixture(t, &VMOwnership{VMID: "vm-p", ComputerID: "computer-p", Epoch: 4,
		MemorySettledRSSBytes: 1000 * mib, MemorySettledEpoch: 4, MemorySettledAt: recent}, 800*mib)
	admitBuild(t, f, "computer-p", "cap-p")
	if got := reservedNow(f); got != (1000+3072-800)*mib {
		t.Fatalf("reserved %d MiB, want %d", got/mib, 1000+3072-800)
	}
}

func TestUnadmittedBootReSettlesTheIdleRSS(t *testing.T) {
	f := idleFixture(t, &VMOwnership{VMID: "vm-p", ComputerID: "computer-p", Epoch: 3,
		MemorySettledRSSBytes: 418 * mib, MemorySettledEpoch: 3,
		MemorySettledAt: time.Date(2026, 10, 10, 6, 0, 0, 0, time.UTC).Format(time.RFC3339)}, 418*mib)
	settled := func() (uint64, int64) {
		f.reg.mu.RLock()
		defer f.reg.mu.RUnlock()
		own := f.reg.vmByID["vm-p"]
		return own.MemorySettledRSSBytes, own.MemorySettledEpoch
	}
	// A deploy refresh: a new realization, no admission.
	f.reg.mu.Lock()
	f.reg.vmByID["vm-p"].Epoch = 4
	f.reg.mu.Unlock()
	f.mgr.setRSS("vm-p", 1500*mib)
	f.reg.sampleVMMemory()
	*f.now = f.now.Add(4 * time.Minute)
	f.mgr.setRSS("vm-p", 1843*mib)
	f.reg.sampleVMMemory()
	if b, e := settled(); b != 418*mib || e != 3 {
		t.Fatalf("I5: re-settled inside the settle window: %d MiB epoch %d", b/mib, e)
	}
	// A build is live when the window ends: its growth is not idle.
	admitBuild(t, f, "computer-p", "cap-p")
	f.mgr.setRSS("vm-p", 3000*mib)
	*f.now = f.now.Add(2 * time.Minute)
	f.reg.sampleVMMemory()
	if b, e := settled(); b != 418*mib || e != 3 {
		t.Fatalf("I5: re-settled while a build was live: %d MiB epoch %d", b/mib, e)
	}
	// The build ends; the next pass re-settles at the reading then.
	if _, err := f.reg.ReportCapsuleReservations(CapsuleReservationReport{ComputerID: "computer-p"}); err != nil {
		t.Fatal(err)
	}
	f.mgr.setRSS("vm-p", 1900*mib)
	*f.now = f.now.Add(10 * time.Second)
	f.reg.sampleVMMemory()
	if b, e := settled(); b != 1900*mib || e != 4 {
		t.Fatalf("I5: not re-settled after the refresh: %d MiB epoch %d, want 1900 epoch 4", b/mib, e)
	}
	// Stamped: it stays put on later passes.
	f.mgr.setRSS("vm-p", 2500*mib)
	*f.now = f.now.Add(10 * time.Minute)
	f.reg.sampleVMMemory()
	if b, _ := settled(); b != 1900*mib {
		t.Fatalf("I5: a stamped settle was re-taken: %d MiB", b/mib)
	}
}

func TestAdmittedStartStampsItsSettleWithTheEpoch(t *testing.T) {
	f := newAdmissionFixture(t, measuredAdmission())
	own := hibernated("u-w", "vm-w", 2*gib)
	own.Epoch = 7
	addOwnership(f.reg, own)
	ticket, err := f.reg.admitHostMemory(own, WarmnessClassPrimary)
	if err != nil {
		t.Fatal(err)
	}
	f.mgr.setRSS("vm-w", 900*mib)
	f.reg.sampleVMMemory()
	ticket.finish(true)
	*f.now = f.now.Add(6 * time.Minute)
	f.reg.sampleVMMemory()
	f.reg.sampleVMMemory()
	f.reg.mu.RLock()
	defer f.reg.mu.RUnlock()
	if got := f.reg.vmByID["vm-w"]; got.MemorySettledRSSBytes != 900*mib || got.MemorySettledEpoch != 7 {
		t.Fatalf("D4 settle not stamped with its epoch: %d MiB epoch %d", got.MemorySettledRSSBytes/mib, got.MemorySettledEpoch)
	}
}

// Review r1 (devin): I6 a stale settled RSS installed before the sampler's
// first reading is never repaired; I7 a future-dated settle reads as fresh;
// I8 a set's baseline outlives its builds, so the next build is charged on
// an old idle.
func TestBuildIdleUnmeasuredFutureAndRecreated(t *testing.T) {
	f := newAdmissionFixture(t, measuredAdmission())
	own := &VMOwnership{VMID: "vm-p", ComputerID: "computer-p", UserID: "u-p", DesktopID: PrimaryDesktopID,
		Kind: VMKindInteractive, State: VMStateActive, Epoch: 4,
		MemorySettledRSSBytes: 418 * mib, MemorySettledEpoch: 3, MemorySettledAt: f.now.Format(time.RFC3339)}
	addOwnership(f.reg, own)
	f.setAvail(20 * gib)
	admitBuild(t, f, "computer-p", "cap-p") // no RSS reading yet
	views := f.reg.HostAdmissionSummary().Reservations
	if len(views) != 1 || views[0].IdleSource != "unmeasured" || views[0].OutstandingBytes != 3*gib {
		t.Fatalf("I6: before any reading: %+v", views)
	}
	f.mgr.setRSS("vm-p", 2000*mib)
	f.reg.sampleVMMemory()
	if _, err := f.reg.ReportCapsuleReservations(CapsuleReservationReport{ComputerID: "computer-p",
		Reservations: []CapsuleReservation{{CapsuleID: "cap-p", Bytes: 3 * gib}}}); err != nil {
		t.Fatal(err)
	}
	if v := f.reg.HostAdmissionSummary().Reservations; len(v) != 1 || v[0].IdleBytes != 2000*mib || v[0].IdleSource != "max_settled_rss" {
		t.Fatalf("I6: after the first reading: %+v", v)
	}
	// I8: the build ends; the next one is charged on the RSS before it.
	if _, err := f.reg.ReportCapsuleReservations(CapsuleReservationReport{ComputerID: "computer-p"}); err != nil {
		t.Fatal(err)
	}
	f.mgr.setRSS("vm-p", 2600*mib)
	f.reg.sampleVMMemory()
	admitBuild(t, f, "computer-p", "cap-q")
	if v := f.reg.HostAdmissionSummary().Reservations; len(v) != 1 || v[0].IdleBytes != 2600*mib || v[0].OutstandingBytes != 3*gib {
		t.Fatalf("I8: the next build's baseline: %+v", v)
	}
	// I7: a settle stamped in the future is not fresh.
	future := settledIdle{bytes: gib, epoch: 4, current: 4, at: f.now.Add(time.Hour).Format(time.RFC3339)}
	if future.fresh(*f.now) {
		t.Fatal("I7: a future-dated settle reads as fresh")
	}
}
