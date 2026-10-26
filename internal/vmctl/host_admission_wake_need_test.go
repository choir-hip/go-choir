package vmctl

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

// Density station D4: admission charges what a wake settles at, not the
// last peak (docs/problems/density-station-d-failure-modes-2026-10-26.md,
// D4). 2026-10-26 11:49: b@c's 7847 MiB peak made a start 448 MiB short
// (13650 MiB required against 13202 MiB available) that a charge near its
// settled size would not have had. Failure modes M1-M10 of that section are
// pinned here; M1 (overcommit) is staging evidence, not a unit test.

func wakeNeedAdmission() HostAdmissionConfig {
	cfg := measuredAdmission()
	cfg.NeedMode = HostAdmissionNeedWake
	return cfg
}

func owned(userID, vmID string, peak, settled uint64) *VMOwnership {
	own := hibernated(userID, vmID, peak)
	own.MemorySettledRSSBytes = settled
	return own
}

// the 11:49 numbers: peak 7847 MiB, settled 1.2 GiB, 13202 MiB available,
// the 4779 MiB floor. Refused under peak, admitted under wake.
func TestWakeNeedAdmitsTheCaseThePeakRefused(t *testing.T) {
	for _, tc := range []struct {
		mode   string
		admit  bool
		source string
		need   uint64
	}{
		{HostAdmissionNeedPeak, false, "", 0},
		{HostAdmissionNeedWake, true, "settled", 1229 << 20},
	} {
		cfg := measuredAdmission()
		cfg.NeedMode = tc.mode
		f := newAdmissionFixture(t, cfg)
		f.reg.SetPressureReclaimConfig(PressureReclaimConfig{
			Mode: PressureReclaimModeActive, Signal: PressureReclaimSignalSustained,
			MinMemoryAvailableBytes: 4779 << 20,
		})
		f.setAvail(13202 << 20)
		ticket, err := f.reg.admitHostMemory(owned("u", "vm-b", 7847<<20, 1229<<20), WarmnessClassPrimary)
		if tc.admit {
			if err != nil || ticket == nil || ticket.need != tc.need || ticket.source != tc.source {
				t.Fatalf("%s: err=%v ticket=%+v, want admit at %d MiB (%s)", tc.mode, err, ticket, tc.need>>20, tc.source)
			}
			ticket.finish(false)
			continue
		}
		if RecoveryRefusalFrom(err) == nil {
			t.Fatalf("%s: admitted at 13202 MiB with a 7847 MiB peak", tc.mode)
		}
	}
}

// M3, M10, M8: the estimate under each mode and the bounds.
func TestWakeNeedEstimate(t *testing.T) {
	const guest = 8 * gib // newAdmissionFixture sets Node B's 8 GiB guest
	for name, tc := range map[string]struct {
		mode    string
		peak    uint64
		settled uint64
		need    uint64
		source  string
	}{
		"wake unmeasured is the default, not the peak": {HostAdmissionNeedWake, 7847 << 20, 0, 2 * gib, "default"},
		"wake settled 1.2 GiB":                         {HostAdmissionNeedWake, 7847 << 20, 1229 << 20, 1229 << 20, "settled"},
		"wake settled 300 MiB charges the minimum":     {HostAdmissionNeedWake, 7847 << 20, 300 << 20, 1 * gib, "settled"},
		"wake settled over the guest is the guest":     {HostAdmissionNeedWake, 7847 << 20, 64 * gib, guest, "settled"},
		"peak mode ignores the settled value":          {HostAdmissionNeedPeak, 7847 << 20, 300 << 20, 7847 << 20, "measured"},
		"peak mode unmeasured is the default":          {HostAdmissionNeedPeak, 0, 300 << 20, 2 * gib, "default"},
		"an empty mode is peak":                        {"", 3 * gib, 300 << 20, 3 * gib, "measured"},
		"an unknown mode is peak":                      {"settled-ish", 3 * gib, 300 << 20, 3 * gib, "measured"},
	} {
		cfg := measuredAdmission()
		cfg.NeedMode = tc.mode
		f := newAdmissionFixture(t, cfg)
		need, source := f.reg.estimateAdmissionNeed(owned("u", "vm", tc.peak, tc.settled))
		if need != tc.need || source != tc.source {
			t.Fatalf("%s: need=%d source=%s, want %d %s", name, need, source, tc.need, tc.source)
		}
	}
}

// M3: an ownership file written before the field existed reads as
// unmeasured (the default need), never as zero need.
func TestWakeNeedOldOwnershipJSONReadsUnmeasured(t *testing.T) {
	var own VMOwnership
	old := `{"vm_id":"vm-old","user_id":"u","desktop_id":"primary","memory_peak_rss_bytes":8227188736,"memory_measured_at":"2026-10-26T11:00:00Z"}`
	if err := json.Unmarshal([]byte(old), &own); err != nil {
		t.Fatal(err)
	}
	if own.MemorySettledRSSBytes != 0 {
		t.Fatalf("settled = %d from a file without the field", own.MemorySettledRSSBytes)
	}
	f := newAdmissionFixture(t, wakeNeedAdmission())
	if need, source := f.reg.estimateAdmissionNeed(&own); need != 2*gib || source != "default" {
		t.Fatalf("old ownership under wake: need=%d source=%s, want the 2 GiB default", need, source)
	}
	out, err := json.Marshal(&VMOwnership{VMID: "vm-new"})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal(out, &m)
	if _, present := m["memory_settled_rss_bytes"]; present {
		t.Fatalf("an unmeasured ownership writes the field: %s", out)
	}
}

// M6: a protected start is admitted without a check, and reserves what the
// mode charges. M7: guest-size mode is unchanged by the need mode.
func TestWakeNeedProtectedAndGuestSizeUnchanged(t *testing.T) {
	f := newAdmissionFixture(t, wakeNeedAdmission())
	f.setAvail(1 * gib)
	ticket, err := f.reg.admitHostMemory(owned("u", "vm-paper", 7847<<20, 1229<<20), WarmnessClassCriticalProtected)
	if err != nil || ticket == nil || ticket.need != 1229<<20 {
		t.Fatalf("protected start under wake: err=%v ticket=%+v", err, ticket)
	}
	if s := f.reg.HostAdmissionSummary(); len(s.Recent) != 1 || s.Recent[0].Decision != "admit_protected" || s.Recent[0].NeedSource != "settled" {
		t.Fatalf("protected decision = %+v", s.Recent)
	}

	cfg := HostAdmissionConfig{Mode: HostAdmissionModeGuestSize, NeedMode: HostAdmissionNeedWake}
	legacy := newAdmissionFixture(t, cfg)
	legacy.setAvail(8 * gib)
	if _, err := legacy.reg.admitHostMemory(owned("u", "vm-1", gib, gib), WarmnessClassPrimary); RecoveryRefusalFrom(err) == nil {
		t.Fatalf("guest-size mode admitted at 8 GiB because the need mode is wake")
	}
	if s := legacy.reg.HostAdmissionSummary(); len(s.Recent) != 1 || s.Recent[0].NeedSource != "guest_size" {
		t.Fatalf("guest-size decision = %+v", s.Recent)
	}
}

// settleFixture is a registry with one active computer and a started
// reservation; the test drives RSS readings and the clock.
func settleFixture(t *testing.T, vmID string) (*admissionFixture, *admissionTicket) {
	t.Helper()
	f := newAdmissionFixture(t, wakeNeedAdmission())
	own := &VMOwnership{VMID: vmID, UserID: "u-" + vmID, DesktopID: PrimaryDesktopID, Kind: VMKindInteractive,
		State: VMStateActive, MemoryPeakRSSBytes: 7847 << 20}
	addOwnership(f.reg, own)
	ticket, err := f.reg.admitHostMemory(own, WarmnessClassPrimary)
	if err != nil {
		t.Fatal(err)
	}
	return f, ticket
}

func (f *admissionFixture) advanceAndSample(d time.Duration) {
	*f.now = f.now.Add(d)
	f.reg.sampleVMMemory()
}

func (f *admissionFixture) settledOf(vmID string) uint64 {
	return f.reg.GetOwnershipByVMID(vmID).MemorySettledRSSBytes
}

// M4: the settled RSS is the VM's resident memory at the window's end, taken
// only then; the peak stays what it was.
func TestSettledRSSCapturedAtWindowEndOnly(t *testing.T) {
	f, ticket := settleFixture(t, "vm-1")
	ticket.finish(true)
	f.mgr.setRSS("vm-1", 3*gib)
	f.advanceAndSample(15 * time.Second)
	f.mgr.setRSS("vm-1", 1229<<20)
	f.advanceAndSample(2 * time.Minute)
	if got := f.settledOf("vm-1"); got != 0 {
		t.Fatalf("settled = %d inside the window", got>>20)
	}
	f.advanceAndSample(3 * time.Minute) // 5m17s after the finish: the window ended
	if got := f.settledOf("vm-1"); got != 1229<<20 {
		t.Fatalf("settled = %d MiB at the window end, want 1229", got>>20)
	}
	if got := f.reg.GetOwnershipByVMID("vm-1").MemoryPeakRSSBytes; got != 3*gib {
		t.Fatalf("peak = %d MiB, want the 3 GiB peak kept", got>>20)
	}
	// Not again: a later reading does not move it.
	f.mgr.setRSS("vm-1", 5*gib)
	f.advanceAndSample(time.Minute)
	if got := f.settledOf("vm-1"); got != 1229<<20 {
		t.Fatalf("settled moved to %d MiB after the window", got>>20)
	}
}

// M4: an in-flight reservation (the start has not reported) captures
// nothing, however long it has run.
func TestSettledRSSNotCapturedForInFlightStart(t *testing.T) {
	f, _ := settleFixture(t, "vm-1")
	f.mgr.setRSS("vm-1", 900<<20)
	f.advanceAndSample(15 * time.Second)
	f.advanceAndSample(8 * time.Minute) // past the settle window, inside the in-flight TTL
	if got := f.settledOf("vm-1"); got != 0 {
		t.Fatalf("settled = %d MiB for a start that never finished", got>>20)
	}
}

// M2: a start that failed (finish(false)) captures nothing.
func TestSettledRSSNotCapturedForFailedStart(t *testing.T) {
	f, ticket := settleFixture(t, "vm-1")
	ticket.finish(false)
	f.mgr.setRSS("vm-1", 900<<20)
	f.advanceAndSample(15 * time.Second)
	f.advanceAndSample(6 * time.Minute)
	if got := f.settledOf("vm-1"); got != 0 {
		t.Fatalf("settled = %d MiB for a failed start", got>>20)
	}
}

// M2: a VM killed inside the window has no reading at its end.
func TestSettledRSSNotCapturedForVMKilledInsideWindow(t *testing.T) {
	f, ticket := settleFixture(t, "vm-1")
	ticket.finish(true)
	f.mgr.setRSS("vm-1", 900<<20)
	f.advanceAndSample(15 * time.Second)
	f.mgr.rmu.Lock()
	delete(f.mgr.rss, "vm-1")
	f.mgr.rmu.Unlock()
	f.advanceAndSample(time.Minute)
	f.advanceAndSample(5 * time.Minute)
	if got := f.settledOf("vm-1"); got != 0 {
		t.Fatalf("settled = %d MiB for a VM that was gone at the window end", got>>20)
	}
}

// Review 305 F1: a VM killed in the last sampler interval of its window,
// when an admission or health read (not the sampler) is the one that ends the
// window, still has its last reading in the ledger. Capture waits for the
// sampler's liveness pass, so the stale reading is not taken.
func TestSettledRSSNotCapturedForVMKilledInTheLastSamplerInterval(t *testing.T) {
	f, ticket := settleFixture(t, "vm-1")
	ticket.finish(true)
	f.mgr.setRSS("vm-1", 1229<<20)
	f.advanceAndSample(15 * time.Second)
	f.advanceAndSample(4*time.Minute + 30*time.Second) // 4m45s in, inside the window
	// Killed now; the sampler has not run since.
	f.mgr.rmu.Lock()
	delete(f.mgr.rss, "vm-1")
	f.mgr.rmu.Unlock()
	*f.now = f.now.Add(25 * time.Second) // 5m10s: the window is over
	f.reg.HostAdmissionSummary()         // a health read ends the window
	f.advanceAndSample(10 * time.Second) // the sampler finds the VM gone
	if got := f.settledOf("vm-1"); got != 0 {
		t.Fatalf("settled = %d MiB captured from a VM that was dead at the window end", got>>20)
	}
}

// Review 305 F1, the other side: a live VM whose window is ended by a health
// read is still captured, at the sampler's next pass.
func TestSettledRSSCapturedAfterAHealthReadEndsTheWindow(t *testing.T) {
	f, ticket := settleFixture(t, "vm-1")
	ticket.finish(true)
	f.mgr.setRSS("vm-1", 1229<<20)
	f.advanceAndSample(15 * time.Second)
	f.advanceAndSample(4*time.Minute + 30*time.Second)
	*f.now = f.now.Add(25 * time.Second)
	f.reg.HostAdmissionSummary()
	if s := f.reg.HostAdmissionSummary(); len(s.Reservations) != 0 {
		t.Fatalf("an ended window still holds %d reservations", len(s.Reservations))
	}
	f.advanceAndSample(10 * time.Second)
	if got := f.settledOf("vm-1"); got != 1229<<20 {
		t.Fatalf("settled = %d MiB, want 1229", got>>20)
	}
}

// M5: the first start's window ends while the VM's second start is still
// settling; the reading is the second start's early one and must not be
// taken for the first's.
func TestSettledRSSNotTakenFromANewerStartsEarlyReading(t *testing.T) {
	f, first := settleFixture(t, "vm-1")
	first.finish(true)
	f.mgr.setRSS("vm-1", 1229<<20)
	f.advanceAndSample(15 * time.Second)
	// The VM hibernates two minutes in, and wakes again.
	f.mgr.rmu.Lock()
	delete(f.mgr.rss, "vm-1")
	f.mgr.rmu.Unlock()
	f.advanceAndSample(2 * time.Minute)
	second, err := f.reg.admitHostMemory(f.reg.GetOwnershipByVMID("vm-1"), WarmnessClassPrimary)
	if err != nil {
		t.Fatal(err)
	}
	second.finish(true)
	f.mgr.setRSS("vm-1", 300<<20)
	f.advanceAndSample(30 * time.Second)
	f.advanceAndSample(3 * time.Minute) // the first window is over; the second is not
	if got := f.settledOf("vm-1"); got != 0 {
		t.Fatalf("settled = %d MiB taken while the newer start was settling", got>>20)
	}
	f.advanceAndSample(3 * time.Minute) // the second window ends
	if got := f.settledOf("vm-1"); got != 300<<20 {
		t.Fatalf("settled = %d MiB, want the second start's 300", got>>20)
	}
}

// M9: the settled value survives a vmctl restart in the ownership file, and
// a restart invents none: a window open at the restart captures nothing.
func TestSettledRSSAcrossRestart(t *testing.T) {
	f, ticket := settleFixture(t, "vm-1")
	path := filepath.Join(t.TempDir(), "ownerships.json")
	if err := f.reg.SetPersistencePath(path); err != nil {
		t.Fatal(err)
	}
	ticket.finish(true)
	f.mgr.setRSS("vm-1", 1229<<20)
	f.advanceAndSample(15 * time.Second)
	f.advanceAndSample(6 * time.Minute)
	if got := f.settledOf("vm-1"); got != 1229<<20 {
		t.Fatalf("settled = %d MiB, want 1229", got>>20)
	}
	f.reg.mu.Lock()
	f.reg.saveLocked()
	f.reg.mu.Unlock()
	reloaded := NewOwnershipRegistry("http://127.0.0.1:8085")
	if err := reloaded.SetPersistencePath(path); err != nil {
		t.Fatal(err)
	}
	if got := reloaded.GetOwnershipByVMID("vm-1"); got == nil || got.MemorySettledRSSBytes != 1229<<20 {
		t.Fatalf("settled value lost across restart: %+v", got)
	}

	// A second computer whose window is open at the restart: nothing after.
	g, ticket2 := settleFixture(t, "vm-2")
	path2 := filepath.Join(t.TempDir(), "ownerships.json")
	if err := g.reg.SetPersistencePath(path2); err != nil {
		t.Fatal(err)
	}
	ticket2.finish(true)
	g.reg.mu.Lock()
	g.reg.saveLocked()
	g.reg.mu.Unlock()
	fresh := NewOwnershipRegistry("http://127.0.0.1:8085")
	if err := fresh.SetPersistencePath(path2); err != nil {
		t.Fatal(err)
	}
	fresh.SetHostAdmissionConfig(wakeNeedAdmission())
	fresh.sampleVMMemory()
	if got := fresh.GetOwnershipByVMID("vm-2"); got != nil && got.MemorySettledRSSBytes != 0 {
		t.Fatalf("a restart invented a settled value: %d MiB", got.MemorySettledRSSBytes>>20)
	}
}

// M8: the capture runs in peak mode too (only the charge is switched), and
// the health summary names the need mode.
func TestSettledRSSCapturedInPeakModeAndShownInHealth(t *testing.T) {
	cfg := measuredAdmission() // peak
	f := newAdmissionFixture(t, cfg)
	own := &VMOwnership{VMID: "vm-1", UserID: "u", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive}
	addOwnership(f.reg, own)
	ticket, err := f.reg.admitHostMemory(own, WarmnessClassPrimary)
	if err != nil {
		t.Fatal(err)
	}
	ticket.finish(true)
	f.mgr.setRSS("vm-1", 700<<20)
	f.advanceAndSample(15 * time.Second)
	f.advanceAndSample(6 * time.Minute)
	if got := f.settledOf("vm-1"); got != 700<<20 {
		t.Fatalf("peak mode did not record the settled value: %d MiB", got>>20)
	}
	if s := f.reg.HostAdmissionSummary(); s.NeedMode != HostAdmissionNeedPeak {
		t.Fatalf("health need mode = %q, want peak", s.NeedMode)
	}
	f.reg.SetHostAdmissionConfig(wakeNeedAdmission())
	if s := f.reg.HostAdmissionSummary(); s.NeedMode != HostAdmissionNeedWake {
		t.Fatalf("health need mode = %q, want wake", s.NeedMode)
	}
}
