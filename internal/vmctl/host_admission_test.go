package vmctl

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// Admission by measured use with in-flight reservations (density quick
// tier, lever L1). docs/reports/vm-density-limits-2026-10-14.md finding 6:
// admission compared one MemAvailable sample against floor + the full
// 8 GiB guest, and recorded nothing about boots already in flight.
//
// Failure modes pinned here (A1-A8 in the problem doc, written before the
// code):
//   - A1 concurrent admissions all pass the same MemAvailable reading;
//   - A2 a reservation never ends, or ends before its VM took the memory;
//   - A3 a computer with no measurement is under-estimated;
//   - A4 a start is admitted that leaves the host under floor + headroom;
//   - A5 a protected computer is refused, or reserves nothing;
//   - A7 a missing sample refuses everyone;
//   - A8 a decision is invisible in health;
//   - the guest-size mode (the rollback) stops behaving as before;
//   - restart semantics change: a vmctl restart resumes or boots anything.

const gib = uint64(1 << 30)

// rssVMManager reports per-VM resident memory like the Firecracker manager.
type rssVMManager struct {
	*processControlVMManager
	rmu sync.Mutex
	rss map[string]uint64
}

func (m *rssVMManager) VMResidentBytes() map[string]uint64 {
	m.rmu.Lock()
	defer m.rmu.Unlock()
	out := make(map[string]uint64, len(m.rss))
	for k, v := range m.rss {
		out[k] = v
	}
	return out
}

func (m *rssVMManager) setRSS(vmID string, bytes uint64) {
	m.rmu.Lock()
	m.rss[vmID] = bytes
	m.rmu.Unlock()
}

type admissionFixture struct {
	reg   *OwnershipRegistry
	mgr   *rssVMManager
	now   *time.Time
	mu    sync.Mutex
	avail uint64
}

func newAdmissionFixture(t *testing.T, admission HostAdmissionConfig) *admissionFixture {
	t.Helper()
	// Node B's guest shape (the compiled default is 2 GiB).
	t.Setenv(interactiveVMMemSizeMibEnv, "8192")
	mgr := &rssVMManager{processControlVMManager: newProcessControlVMManager(), rss: map[string]uint64{}}
	reg, now := hostCapacityRegistry(t, mgr)
	f := &admissionFixture{reg: reg, mgr: mgr, now: now, avail: 16 * gib}
	reg.SetPressureReclaimConfig(PressureReclaimConfig{
		Mode:                    PressureReclaimModeActive,
		Signal:                  PressureReclaimSignalSustained,
		MinMemoryAvailableBytes: 4 * gib,
	})
	reg.SetHostAdmissionConfig(admission)
	reg.setPressureSamplerForTest(func(PressureReclaimConfig) HostPressureSample {
		f.mu.Lock()
		defer f.mu.Unlock()
		return HostPressureSample{MemoryTotalBytes: 32 * gib, MemoryAvailableBytes: f.avail,
			MemoryAvailablePercent: float64(f.avail) / float64(32*gib) * 100}
	})
	return f
}

func measuredAdmission() HostAdmissionConfig {
	return HostAdmissionConfig{
		Mode:             HostAdmissionModeMeasured,
		DefaultNeedBytes: 2 * gib,
		MinNeedBytes:     1 * gib,
		HeadroomBytes:    1 * gib,
		Settle:           5 * time.Minute,
		InFlightTTL:      15 * time.Minute,
	}
}

func (f *admissionFixture) setAvail(b uint64) {
	f.mu.Lock()
	f.avail = b
	f.mu.Unlock()
}

func hibernated(userID, vmID string, peak uint64) *VMOwnership {
	return &VMOwnership{VMID: vmID, UserID: userID, DesktopID: PrimaryDesktopID, Kind: VMKindInteractive,
		State: VMStateHibernated, MemoryPeakRSSBytes: peak}
}

// The 13:xx refusal: 11.8 GiB available refused a wake under the 8 GiB
// reservation; a measured computer that used 1.5 GiB fits.
func TestMeasuredAdmissionUsesLastRSSNotGuestSize(t *testing.T) {
	f := newAdmissionFixture(t, measuredAdmission())
	f.setAvail(8 * gib)
	own := hibernated("u1", "vm-1", 1536<<20)
	ticket, err := f.reg.admitHostMemory(own, WarmnessClassPrimary)
	if err != nil {
		t.Fatalf("measured 1.5 GiB computer refused at 8 GiB available: %v", err)
	}
	if ticket == nil || ticket.need != 1536<<20 || ticket.source != "measured" {
		t.Fatalf("ticket = %+v, want need 1.5 GiB from the measurement", ticket)
	}
	ticket.finish(false)

	legacy := newAdmissionFixture(t, HostAdmissionConfig{Mode: HostAdmissionModeGuestSize})
	legacy.setAvail(8 * gib)
	if _, err := legacy.reg.admitHostMemory(own, WarmnessClassPrimary); RecoveryRefusalFrom(err) == nil {
		t.Fatalf("guest-size mode admitted at 8 GiB: today's rule needs floor + 8 GiB")
	}
	legacy.setAvail(12*gib + 1)
	ticket, err = legacy.reg.admitHostMemory(own, WarmnessClassPrimary)
	if err != nil {
		t.Fatalf("guest-size mode refused at floor + 8 GiB: %v", err)
	}
	ticket.finish(true)
	if s := legacy.reg.HostAdmissionSummary(); len(s.Reservations) != 0 {
		t.Fatalf("guest-size mode keeps a ledger: %+v", s.Reservations)
	}
}

// A3: unknown computers use the conservative default; a measured one is
// never estimated under the minimum or over the guest size.
func TestAdmissionEstimateBounds(t *testing.T) {
	f := newAdmissionFixture(t, measuredAdmission())
	_, guestMib := interactiveMachineShape()
	for name, tc := range map[string]struct {
		peak   uint64
		need   uint64
		source string
	}{
		"unknown":       {0, 2 * gib, "default"},
		"tiny measured": {200 << 20, 1 * gib, "measured"},
		"over guest":    {64 * gib, uint64(guestMib) << 20, "measured"},
	} {
		need, source := f.reg.estimateAdmissionNeed(hibernated("u", "vm", tc.peak))
		if need != tc.need || source != tc.source {
			t.Fatalf("%s: need=%d source=%s, want %d %s", name, need, source, tc.need, tc.source)
		}
	}
}

// A4: never admit below floor + headroom + need; the boundary admits.
func TestAdmissionHoldsTheFloor(t *testing.T) {
	f := newAdmissionFixture(t, measuredAdmission())
	own := hibernated("u1", "vm-1", 1*gib)
	f.setAvail(4*gib + 1*gib + 1*gib - 1)
	_, err := f.reg.admitHostMemory(own, WarmnessClassPrimary)
	refusal := RecoveryRefusalFrom(err)
	if refusal == nil || refusal.Kind != RecoveryRefusalHostMemoryPressure || refusal.RetryAfterSeconds <= 0 {
		t.Fatalf("admitted one byte under floor+headroom+need: %v", err)
	}
	f.setAvail(6 * gib)
	ticket, err := f.reg.admitHostMemory(own, WarmnessClassPrimary)
	if err != nil {
		t.Fatalf("refused at exactly floor+headroom+need: %v", err)
	}
	ticket.finish(false)
}

// A1: concurrent admissions reserve; they cannot all pass one reading.
func TestConcurrentAdmissionsReserve(t *testing.T) {
	f := newAdmissionFixture(t, measuredAdmission())
	// Room above floor + headroom: 3 GiB. Each wake needs 1.2 GiB: two fit.
	f.setAvail(4*gib + 1*gib + 3*gib)
	const n = 6
	start := make(chan struct{})
	var wg sync.WaitGroup
	var mu sync.Mutex
	admitted := 0
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			own := hibernated("u", "vm-"+string(rune('a'+i)), 1229<<20)
			<-start
			if _, err := f.reg.admitHostMemory(own, WarmnessClassPrimary); err == nil {
				mu.Lock()
				admitted++
				mu.Unlock()
			}
		}(i)
	}
	close(start)
	wg.Wait()
	if admitted != 2 {
		t.Fatalf("%d of %d concurrent wakes admitted with room for 2", admitted, n)
	}
	s := f.reg.HostAdmissionSummary()
	if len(s.Reservations) != 2 || s.ReservedBytes != 2*(1229<<20) {
		t.Fatalf("ledger = %+v", s)
	}
}

// A2: a failed start releases at once; a finished one keeps only what its
// VM has not yet taken, and only for the settle window; an in-flight one
// expires after its bound.
func TestReservationLifecycle(t *testing.T) {
	f := newAdmissionFixture(t, measuredAdmission())
	own := hibernated("u1", "vm-1", 2*gib)

	ticket, err := f.reg.admitHostMemory(own, WarmnessClassPrimary)
	if err != nil {
		t.Fatal(err)
	}
	ticket.finish(false)
	if s := f.reg.HostAdmissionSummary(); s.ReservedBytes != 0 {
		t.Fatalf("failed start still reserves %d", s.ReservedBytes)
	}

	ticket, err = f.reg.admitHostMemory(own, WarmnessClassPrimary)
	if err != nil {
		t.Fatal(err)
	}
	ticket.finish(true)
	if s := f.reg.HostAdmissionSummary(); s.ReservedBytes != 2*gib {
		t.Fatalf("settling start before any measurement reserves %d, want the full need", s.ReservedBytes)
	}
	*f.now = f.now.Add(15 * time.Second)
	f.mgr.setRSS("vm-1", 512<<20)
	f.reg.sampleVMMemory()
	if s := f.reg.HostAdmissionSummary(); s.ReservedBytes != 2*gib-512<<20 {
		t.Fatalf("after 0.5 GiB measured, reserved %d, want need minus RSS", s.ReservedBytes)
	}
	f.mgr.setRSS("vm-1", 3*gib)
	f.reg.sampleVMMemory()
	if s := f.reg.HostAdmissionSummary(); s.ReservedBytes != 0 {
		t.Fatalf("VM over its estimate still reserves %d", s.ReservedBytes)
	}
	f.mgr.setRSS("vm-1", 0)
	*f.now = f.now.Add(6 * time.Minute)
	if s := f.reg.HostAdmissionSummary(); len(s.Reservations) != 0 {
		t.Fatalf("reservation outlived the settle window: %+v", s.Reservations)
	}

	// A start that never reports back expires after the in-flight bound.
	if _, err = f.reg.admitHostMemory(hibernated("u2", "vm-2", 2*gib), WarmnessClassPrimary); err != nil {
		t.Fatal(err)
	}
	*f.now = f.now.Add(16 * time.Minute)
	if s := f.reg.HostAdmissionSummary(); len(s.Reservations) != 0 {
		t.Fatalf("in-flight reservation never expired: %+v", s.Reservations)
	}
}

// A5: a protected computer is admitted below the floor, and its
// reservation counts against the next ordinary start.
func TestProtectedAdmissionReserves(t *testing.T) {
	f := newAdmissionFixture(t, measuredAdmission())
	f.setAvail(7 * gib)
	paper := &VMOwnership{VMID: "vm-paper", UserID: "paper", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive,
		WarmnessClass: WarmnessClassCriticalProtected, State: VMStateHibernated, MemoryPeakRSSBytes: 3 * gib}
	if _, err := f.reg.admitHostMemory(paper, WarmnessClassCriticalProtected); err != nil {
		t.Fatalf("protected computer refused: %v", err)
	}
	// 7 GiB available, 3 GiB reserved: a 1 GiB wake needs 4+1+1+3 = 9 GiB.
	if _, err := f.reg.admitHostMemory(hibernated("u1", "vm-1", 1*gib), WarmnessClassPrimary); RecoveryRefusalFrom(err) == nil {
		t.Fatalf("ordinary start ignored the protected reservation: %v", err)
	}
	f.setAvail(2 * gib)
	if _, err := f.reg.admitHostMemory(paper, WarmnessClassCriticalProtected); err != nil {
		t.Fatalf("protected computer refused under the floor: %v", err)
	}
}

// A7: a missing MemAvailable sample admits, as today.
func TestAdmissionUnknownSampleAdmits(t *testing.T) {
	f := newAdmissionFixture(t, measuredAdmission())
	f.setAvail(0)
	ticket, err := f.reg.admitHostMemory(hibernated("u1", "vm-1", 1*gib), WarmnessClassPrimary)
	if err != nil {
		t.Fatalf("missing sample refused: %v", err)
	}
	ticket.finish(false)
}

// End to end on resolve: a refused start boots nothing; an admitted start
// boots and settles its reservation.
func TestResolveAdmissionUsesLedger(t *testing.T) {
	f := newAdmissionFixture(t, measuredAdmission())
	f.setAvail(4*gib + 1*gib + 2*gib) // room for exactly one default-need computer
	if _, err := f.reg.ResolveOrAssign("new-1"); err != nil {
		t.Fatalf("first new computer refused: %v", err)
	}
	s := f.reg.HostAdmissionSummary()
	if len(s.Reservations) != 1 || s.Reservations[0].InFlight {
		t.Fatalf("after boot, ledger = %+v, want one settling reservation", s.Reservations)
	}
	_, err := f.reg.ResolveOrAssign("new-2")
	if RecoveryRefusalFrom(err) == nil {
		t.Fatalf("second new computer admitted while the first's reservation stands: %v", err)
	}
	if len(f.mgr.boots) != 1 {
		t.Fatalf("boots = %d, want 1", len(f.mgr.boots))
	}

	// A failed boot releases its reservation.
	f.mgr.bootError = errors.New("boot failed")
	*f.now = f.now.Add(10 * time.Minute)
	if _, err := f.reg.ResolveOrAssign("new-3"); err == nil || RecoveryRefusalFrom(err) != nil {
		t.Fatalf("boot failure = %v, want the boot error", err)
	}
	if s := f.reg.HostAdmissionSummary(); s.ReservedBytes != 0 {
		t.Fatalf("failed boot left %d reserved", s.ReservedBytes)
	}
}

// A8: decisions and the ledger are visible in vmctl /health, and health
// does not sample the host to show them.
func TestAdmissionDecisionsInHealth(t *testing.T) {
	f := newAdmissionFixture(t, measuredAdmission())
	f.setAvail(5 * gib)
	_, _ = f.reg.admitHostMemory(hibernated("u1", "vm-1", 1*gib), WarmnessClassPrimary)
	f.setAvail(16 * gib)
	_, _ = f.reg.admitHostMemory(hibernated("u2", "vm-2", 1*gib), WarmnessClassPrimary)

	srv := httptest.NewServer(http.HandlerFunc(NewHandler(f.reg).HandleHealth))
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var body struct {
		Admission HostAdmissionSummary `json:"admission"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	a := body.Admission
	if a.Mode != HostAdmissionModeMeasured || len(a.Recent) != 2 || len(a.Reservations) != 1 {
		t.Fatalf("health admission = %+v", a)
	}
	refused, admitted := a.Recent[0], a.Recent[1]
	if refused.Decision != "refuse" || refused.NeedSource != "measured" || refused.AvailableBytes != 5*gib || refused.FloorBytes != 4*gib {
		t.Fatalf("refusal record = %+v", refused)
	}
	if admitted.Decision != "admit" || admitted.VMID != "vm-2" {
		t.Fatalf("admit record = %+v", admitted)
	}
}

// RSS measurement: the estimate is the peak of the most recent start; a new
// start replaces it with its own measurements. It survives a vmctl restart
// in the ownership file.
func TestMemoryMeasurementTracksPeakPerStart(t *testing.T) {
	f := newAdmissionFixture(t, measuredAdmission())
	path := filepath.Join(t.TempDir(), "ownerships.json")
	if err := f.reg.SetPersistencePath(path); err != nil {
		t.Fatal(err)
	}
	addOwnership(f.reg, &VMOwnership{VMID: "vm-1", UserID: "u1", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive})
	for _, rss := range []uint64{1 * gib, 3 * gib, 2 * gib} {
		f.mgr.setRSS("vm-1", rss)
		*f.now = f.now.Add(15 * time.Second)
		f.reg.sampleVMMemory()
	}
	if got := f.reg.GetOwnershipByVMID("vm-1").MemoryPeakRSSBytes; got != 3*gib {
		t.Fatalf("peak = %d, want 3 GiB", got)
	}

	// A new start (a wake) measures afresh.
	ticket, err := f.reg.admitHostMemory(f.reg.GetOwnershipByVMID("vm-1"), WarmnessClassPrimary)
	if err != nil {
		t.Fatal(err)
	}
	if ticket.need != 3*gib {
		t.Fatalf("wake estimate = %d, want the last peak", ticket.need)
	}
	ticket.finish(true)
	f.mgr.setRSS("vm-1", 512<<20)
	*f.now = f.now.Add(15 * time.Second)
	f.reg.sampleVMMemory()
	if got := f.reg.GetOwnershipByVMID("vm-1").MemoryPeakRSSBytes; got != 512<<20 {
		t.Fatalf("after a new start, peak = %d, want 0.5 GiB", got)
	}

	f.reg.mu.Lock()
	f.reg.saveLocked()
	f.reg.mu.Unlock()
	reloaded := NewOwnershipRegistry("http://127.0.0.1:8085")
	if err := reloaded.SetPersistencePath(path); err != nil {
		t.Fatal(err)
	}
	if got := reloaded.GetOwnershipByVMID("vm-1"); got == nil || got.MemoryPeakRSSBytes != 512<<20 {
		t.Fatalf("measurement lost across restart: %+v", got)
	}
}

// Restart semantics untouched: after a vmctl restart, running computers
// load as stopped; the pressure monitor and the admission ledger start,
// wake or resume nothing.
func TestPressureMonitorAfterRestartResumesNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ownerships.json")
	first := newAdmissionFixture(t, measuredAdmission())
	if err := first.reg.SetPersistencePath(path); err != nil {
		t.Fatal(err)
	}
	if _, err := first.reg.ResolveOrAssign("u1"); err != nil {
		t.Fatal(err)
	}

	second := newAdmissionFixture(t, measuredAdmission())
	if err := second.reg.SetPersistencePath(path); err != nil {
		t.Fatal(err)
	}
	second.setAvail(2 * gib)
	for i := 0; i < 10; i++ {
		*second.now = second.now.Add(15 * time.Second)
		second.reg.pressureMonitorTick(context.Background(), allowAllRoutes)
	}
	if len(second.mgr.boots) != 0 || len(second.mgr.resumes) != 0 || len(second.mgr.recovers) != 0 {
		t.Fatalf("monitor started work after restart: boots=%d resumes=%d recovers=%d",
			len(second.mgr.boots), len(second.mgr.resumes), len(second.mgr.recovers))
	}
	if own := second.reg.GetOwnership("u1"); own == nil || own.State == VMStateActive {
		t.Fatalf("restart left the computer %+v, want it not active", own)
	}
	if s := second.reg.HostAdmissionSummary(); len(s.Reservations) != 0 {
		t.Fatalf("ledger survived the restart: %+v", s.Reservations)
	}
}
