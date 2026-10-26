package vmctl

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Density station D1: a start that does not fit makes room by hibernating
// the lowest-priority idle eligible computer(s), then admits; it refuses
// only when nothing eligible can make the room, naming why
// (docs/problems/density-station-d-failure-modes-2026-10-26.md, D1).
// 2026-10-26 11:49: b@c was refused 448 MiB short and FP0 stopped d@e by
// hand. Failure modes M1-M14 of that section are pinned here.

// makeRoomVMManager frees a hibernated VM's resident memory on the host,
// as a snapshot hibernate does when it ends the Firecracker process.
type makeRoomVMManager struct {
	*rssVMManager
	f           *makeRoomFixture
	hmu         sync.Mutex
	hibernated  []string
	inFlight    int32
	maxInFlt    int32
	onHibernate func(vmID string)
}

func (m *makeRoomVMManager) HibernateVM(vmID string) error {
	n := atomic.AddInt32(&m.inFlight, 1)
	for {
		max := atomic.LoadInt32(&m.maxInFlt)
		if n <= max || atomic.CompareAndSwapInt32(&m.maxInFlt, max, n) {
			break
		}
	}
	defer atomic.AddInt32(&m.inFlight, -1)
	m.hmu.Lock()
	m.hibernated = append(m.hibernated, vmID)
	hook := m.onHibernate
	m.hmu.Unlock()
	if hook != nil {
		hook(vmID)
	}
	m.rmu.Lock()
	freed := m.rss[vmID]
	delete(m.rss, vmID)
	m.rmu.Unlock()
	if m.f != nil && freed > 0 {
		m.f.addAvail(freed)
	}
	return nil
}

func (m *makeRoomVMManager) hibernates() []string {
	m.hmu.Lock()
	defer m.hmu.Unlock()
	return append([]string(nil), m.hibernated...)
}

type makeRoomFixture struct {
	*admissionFixture
	mr *makeRoomVMManager
	// freeOnHibernate false keeps MemAvailable flat after a hibernate (the
	// freed memory never shows up).
	freeOnHibernate bool
}

func (f *makeRoomFixture) addAvail(b uint64) {
	if !f.freeOnHibernate {
		return
	}
	f.mu.Lock()
	f.avail += b
	f.mu.Unlock()
}

func makeRoomAdmission() HostAdmissionConfig {
	cfg := measuredAdmission()
	cfg.MakeRoom = HostAdmissionMakeRoomActive
	cfg.MakeRoomMaxVictims = 3
	cfg.MakeRoomSettleWait = 200 * time.Millisecond
	cfg.MakeRoomPollInterval = 5 * time.Millisecond
	return cfg
}

func newMakeRoomFixture(t *testing.T, admission HostAdmissionConfig) *makeRoomFixture {
	t.Helper()
	base := newAdmissionFixture(t, admission)
	mr := &makeRoomVMManager{rssVMManager: base.mgr}
	f := &makeRoomFixture{admissionFixture: base, mr: mr, freeOnHibernate: true}
	mr.f = f
	base.reg.SetVMManager(mr)
	base.reg.SetMakeRoomRouteGuard(allowAllRoutes)
	base.reg.setGuestBusyProbeForTest(func(*VMOwnership) bool { return false })
	return f
}

// activeComputer adds a running computer idle for idle, resident at rss.
func (f *makeRoomFixture) activeComputer(userID, vmID string, idle time.Duration, rss uint64) *VMOwnership {
	own := &VMOwnership{VMID: vmID, UserID: userID, DesktopID: PrimaryDesktopID, Kind: VMKindInteractive,
		ComputerID: "computer-" + vmID, State: VMStateActive, LastActiveAt: f.now.Add(-idle),
		ComputerURL: "http://10.0.0.9:8085", MemoryPeakRSSBytes: rss}
	addOwnership(f.reg, own)
	f.mgr.setRSS(vmID, rss)
	f.reg.sampleVMMemory()
	return own
}

func (f *makeRoomFixture) state(vmID string) (VMState, string) {
	f.reg.mu.RLock()
	defer f.reg.mu.RUnlock()
	own := f.reg.vmByID[vmID]
	if own == nil {
		return "", ""
	}
	return own.State, own.StoppedBy
}

func lastDecision(t *testing.T, reg *OwnershipRegistry) HostAdmissionDecision {
	t.Helper()
	s := reg.HostAdmissionSummary()
	if len(s.Recent) == 0 {
		t.Fatalf("no admission decision recorded")
	}
	return s.Recent[len(s.Recent)-1]
}

const mib = uint64(1 << 20)

// The 11:49 case: b@c needs 7847 MiB (measured) above 4 GiB floor and
// 1 GiB headroom, 448 MiB short; d@e is an idle primary computer holding
// 2 GiB. The start hibernates d@e and is admitted.
func TestMakeRoomHibernatesIdleComputerThenAdmits(t *testing.T) {
	f := newMakeRoomFixture(t, makeRoomAdmission())
	f.activeComputer("d-at-e", "vm-de", 40*time.Minute, 2*gib)
	f.setAvail(4*gib + 1*gib + 7847*mib - 448*mib)

	starter := hibernated("b-at-c", "vm-bc", 7847*mib)
	ticket, err := f.reg.admitHostMemory(starter, WarmnessClassPrimary)
	if err != nil {
		t.Fatalf("start refused although an idle computer could make room: %v", err)
	}
	if ticket == nil || ticket.need != 7847*mib {
		t.Fatalf("ticket = %+v, want the starter's measured need reserved", ticket)
	}
	defer ticket.finish(true)
	if got := f.mr.hibernates(); len(got) != 1 || got[0] != "vm-de" {
		t.Fatalf("hibernates = %v, want exactly d@e", got)
	}
	if st, by := f.state("vm-de"); st != VMStateHibernated || by != stoppedByMakeRoom {
		t.Fatalf("d@e state=%s stopped_by=%q, want hibernated by %q", st, by, stoppedByMakeRoom)
	}
	d := lastDecision(t, f.reg)
	if d.Decision != "admit_after_make_room" || len(d.MadeRoomVMIDs) != 1 || d.MadeRoomVMIDs[0] != "vm-de" {
		t.Fatalf("decision = %+v, want admit_after_make_room naming vm-de", d)
	}
	// M11: the admitted start holds exactly one reservation.
	if s := f.reg.HostAdmissionSummary(); len(s.Reservations) != 1 || s.Reservations[0].VMID != "vm-bc" || !s.Reservations[0].InFlight {
		t.Fatalf("reservations = %+v, want the starter's one in-flight reservation", s.Reservations)
	}
}

// Victims go lowest class first, longest idle first, and only as many as
// the shortfall needs.
func TestMakeRoomOrderAndOnlyAsManyAsNeeded(t *testing.T) {
	f := newMakeRoomFixture(t, makeRoomAdmission())
	f.reg.mu.Lock()
	f.reg.warmnessPolicy.TestUserIDs = map[string]bool{"tester": true}
	f.reg.mu.Unlock()
	f.activeComputer("primary-old", "vm-p-old", 3*time.Hour, 3*gib)
	f.activeComputer("primary-new", "vm-p-new", 45*time.Minute, 3*gib)
	f.activeComputer("tester", "vm-test", 10*time.Minute, 1*gib)
	// Short by 3.5 GiB: the test computer (1 GiB) and the longest-idle
	// primary (3 GiB) cover it.
	f.setAvail(4*gib + 1*gib + 4*gib - 3584*mib)

	ticket, err := f.reg.admitHostMemory(hibernated("starter", "vm-s", 4*gib), WarmnessClassPrimary)
	if err != nil {
		t.Fatalf("start refused: %v", err)
	}
	ticket.finish(true)
	got := f.mr.hibernates()
	if len(got) != 2 || got[0] != "vm-test" || got[1] != "vm-p-old" {
		t.Fatalf("hibernates = %v, want [vm-test vm-p-old]", got)
	}
	if st, _ := f.state("vm-p-new"); st != VMStateActive {
		t.Fatalf("vm-p-new state = %s, want left running", st)
	}
}

// M1: a working computer is never a victim; an unanswered busy probe
// counts as working. M8: the refusal names why nothing was eligible.
func TestMakeRoomNeverHibernatesAWorkingComputer(t *testing.T) {
	f := newMakeRoomFixture(t, makeRoomAdmission())
	f.activeComputer("worker", "vm-busy", 2*time.Hour, 4*gib)
	f.reg.setGuestBusyProbeForTest(func(own *VMOwnership) bool { return own.VMID == "vm-busy" })
	f.setAvail(5 * gib)

	_, err := f.reg.admitHostMemory(hibernated("starter", "vm-s", 2*gib), WarmnessClassPrimary)
	refusal := RecoveryRefusalFrom(err)
	if refusal == nil || refusal.Kind != RecoveryRefusalHostMemoryPressure {
		t.Fatalf("err = %v, want a host_memory_pressure refusal", err)
	}
	if got := f.mr.hibernates(); len(got) != 0 {
		t.Fatalf("hibernated a working computer: %v", got)
	}
	if !strings.Contains(refusal.Reason, "busy") {
		t.Fatalf("refusal %q does not say the only candidate was busy", refusal.Reason)
	}
	if d := lastDecision(t, f.reg); d.Decision != "refuse_no_room" {
		t.Fatalf("decision = %q, want refuse_no_room", d.Decision)
	}
}

// M2: the paper, a public platform computer, an always-on computer and a
// held computer are never victims.
func TestMakeRoomNeverHibernatesProtectedOrHeldComputers(t *testing.T) {
	f := newMakeRoomFixture(t, makeRoomAdmission())
	f.reg.mu.Lock()
	f.reg.warmnessPolicy.PlatformComputerIDs = map[string]bool{"computer-vm-paper": true}
	f.reg.warmnessPolicy.PaidUserIDs = map[string]bool{"owner": true}
	f.reg.mu.Unlock()
	f.activeComputer("paper-user", "vm-paper", 5*time.Hour, 8*gib)
	f.activeComputer("owner", "vm-owner", 5*time.Hour, 8*gib)
	platform := f.activeComputer("platform", "vm-platform", 5*time.Hour, 4*gib)
	held := f.activeComputer("held", "vm-held", 5*time.Hour, 4*gib)
	f.reg.mu.Lock()
	platform.WarmnessClass = WarmnessClassPublicPlatform
	held.HoldStatus = &MaintenanceHold{Reason: "test"}
	f.reg.mu.Unlock()
	f.setAvail(5 * gib)

	_, err := f.reg.admitHostMemory(hibernated("starter", "vm-s", 2*gib), WarmnessClassPrimary)
	if RecoveryRefusalFrom(err) == nil {
		t.Fatalf("err = %v, want a refusal: only protected or held computers are running", err)
	}
	if got := f.mr.hibernates(); len(got) != 0 {
		t.Fatalf("hibernated a protected or held computer: %v", got)
	}
}

// M3: a start never evicts a computer of a higher class than its own: a
// test start does not hibernate an idle primary computer, a primary start
// may hibernate a test computer however recently it was used.
func TestMakeRoomRespectsClassOrder(t *testing.T) {
	f := newMakeRoomFixture(t, makeRoomAdmission())
	f.reg.mu.Lock()
	f.reg.warmnessPolicy.TestUserIDs = map[string]bool{"tester": true, "tester-2": true}
	f.reg.mu.Unlock()
	f.activeComputer("primary", "vm-primary", 3*time.Hour, 4*gib)
	f.setAvail(5 * gib)

	_, err := f.reg.admitHostMemory(hibernated("tester-2", "vm-t2", 2*gib), WarmnessClassTest)
	if RecoveryRefusalFrom(err) == nil {
		t.Fatalf("test start admitted: %v", err)
	}
	if got := f.mr.hibernates(); len(got) != 0 {
		t.Fatalf("a test start hibernated a primary computer: %v", got)
	}

	f.activeComputer("tester", "vm-test", time.Minute, 3*gib)
	ticket, err := f.reg.admitHostMemory(hibernated("starter", "vm-s", 2*gib), WarmnessClassPrimary)
	if err != nil {
		t.Fatalf("primary start refused although a test computer could yield: %v", err)
	}
	ticket.finish(true)
	if got := f.mr.hibernates(); len(got) != 1 || got[0] != "vm-test" {
		t.Fatalf("hibernates = %v, want the test computer only", got)
	}
}

// A test start may evict another test computer only once that computer is
// past the admission settle window: five test computers started in a row
// do not evict each other while they boot.
func TestMakeRoomTestStarterSparesAJustStartedTestComputer(t *testing.T) {
	f := newMakeRoomFixture(t, makeRoomAdmission())
	f.reg.mu.Lock()
	f.reg.warmnessPolicy.TestUserIDs = map[string]bool{"t1": true, "t2": true, "t3": true}
	f.reg.mu.Unlock()
	f.activeComputer("t1", "vm-t1", 2*time.Minute, 3*gib)
	f.setAvail(5 * gib)
	if _, err := f.reg.admitHostMemory(hibernated("t3", "vm-t3", 2*gib), WarmnessClassTest); RecoveryRefusalFrom(err) == nil {
		t.Fatalf("err = %v, want refusal: t1 is inside its settle window", err)
	}
	f.activeComputer("t2", "vm-t2", 6*time.Minute, 3*gib)
	ticket, err := f.reg.admitHostMemory(hibernated("t3", "vm-t3", 2*gib), WarmnessClassTest)
	if err != nil {
		t.Fatalf("test start refused although t2 is idle past the settle window: %v", err)
	}
	ticket.finish(true)
	if got := f.mr.hibernates(); len(got) != 1 || got[0] != "vm-t2" {
		t.Fatalf("hibernates = %v, want vm-t2 only", got)
	}
}

// M5: a computer whose own start is in flight (booting, or holding a
// lifecycle claim) is never a victim.
func TestMakeRoomSkipsComputersWithALifecycleClaim(t *testing.T) {
	f := newMakeRoomFixture(t, makeRoomAdmission())
	claimed := f.activeComputer("claimed", "vm-claimed", 2*time.Hour, 4*gib)
	booting := f.activeComputer("booting", "vm-booting", 2*time.Hour, 4*gib)
	f.reg.mu.Lock()
	f.reg.lifecycle[ownershipKey(claimed.UserID, claimed.DesktopID)] = lifecycleVisit
	booting.State = VMStateBooting
	f.reg.mu.Unlock()
	f.setAvail(5 * gib)

	if _, err := f.reg.admitHostMemory(hibernated("starter", "vm-s", 2*gib), WarmnessClassPrimary); RecoveryRefusalFrom(err) == nil {
		t.Fatalf("err = %v, want refusal", err)
	}
	if got := f.mr.hibernates(); len(got) != 0 {
		t.Fatalf("hibernated a computer mid-lifecycle: %v", got)
	}
}

// M6: when every eligible computer together cannot cover the shortfall,
// nothing is hibernated. M7: at most MakeRoomMaxVictims per start.
func TestMakeRoomRefusesWithoutEvictingWhenItCannotHelp(t *testing.T) {
	f := newMakeRoomFixture(t, makeRoomAdmission())
	f.activeComputer("a", "vm-a", 2*time.Hour, 1*gib)
	f.activeComputer("b", "vm-b", 2*time.Hour, 1*gib)
	f.activeComputer("c", "vm-c", 2*time.Hour, 1*gib)
	f.activeComputer("d", "vm-d", 2*time.Hour, 1*gib)
	// Short by 3.5 GiB: four 1 GiB computers would cover it, but at most
	// three may be hibernated for one start.
	f.setAvail(4*gib + 1*gib + 4*gib - 3584*mib)

	_, err := f.reg.admitHostMemory(hibernated("starter", "vm-s", 4*gib), WarmnessClassPrimary)
	refusal := RecoveryRefusalFrom(err)
	if refusal == nil {
		t.Fatalf("err = %v, want refusal", err)
	}
	if got := f.mr.hibernates(); len(got) != 0 {
		t.Fatalf("evicted %v for a start they could not make room for", got)
	}
	if !strings.Contains(refusal.Reason, "short") {
		t.Fatalf("refusal %q does not name the shortfall", refusal.Reason)
	}
}

// M7: when the freed memory never shows up, the start waits a bounded
// time per victim and then refuses, naming what it hibernated.
func TestMakeRoomWaitIsBounded(t *testing.T) {
	f := newMakeRoomFixture(t, makeRoomAdmission())
	f.freeOnHibernate = false
	f.activeComputer("a", "vm-a", 2*time.Hour, 4*gib)
	f.setAvail(5 * gib)

	done := make(chan error, 1)
	go func() {
		_, err := f.reg.admitHostMemory(hibernated("starter", "vm-s", 2*gib), WarmnessClassPrimary)
		done <- err
	}()
	select {
	case err := <-done:
		refusal := RecoveryRefusalFrom(err)
		if refusal == nil || !strings.Contains(refusal.Reason, "vm-a") {
			t.Fatalf("err = %v, want a refusal naming the hibernated vm-a", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("make-room waited without bound for memory that never came")
	}
	if got := f.mr.hibernates(); len(got) != 1 {
		t.Fatalf("hibernates = %v, want one", got)
	}
}

// M4: two refused starts at once: one decider at a time, each victim
// hibernated once, each start admitted with its own victim.
func TestMakeRoomOneDeciderNoDoubleHibernate(t *testing.T) {
	f := newMakeRoomFixture(t, makeRoomAdmission())
	f.activeComputer("v1", "vm-v1", 3*time.Hour, 1536*mib)
	f.activeComputer("v2", "vm-v2", 2*time.Hour, 1536*mib)
	f.activeComputer("v3", "vm-v3", 1*time.Hour, 1536*mib)
	// Room for neither start: each needs 2 GiB above floor + headroom, 1 GiB
	// short. One 1.5 GiB victim admits the first; the second then sees the
	// first's reservation and needs exactly one more (not two, not three).
	f.setAvail(5*gib + 1*gib)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	tickets := make([]*admissionTicket, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			tickets[i], errs[i] = f.reg.admitHostMemory(hibernated("s", "vm-s"+string(rune('a'+i)), 2*gib), WarmnessClassPrimary)
		}(i)
	}
	waitDone := make(chan struct{})
	go func() { wg.Wait(); close(waitDone) }()
	select {
	case <-waitDone:
	case <-time.After(10 * time.Second):
		t.Fatalf("concurrent make-room did not finish")
	}
	for i, err := range errs {
		if err != nil {
			t.Fatalf("start %d refused: %v", i, err)
		}
		tickets[i].finish(true)
	}
	got := f.mr.hibernates()
	seen := map[string]bool{}
	for _, id := range got {
		if seen[id] {
			t.Fatalf("VM %s hibernated twice: %v", id, got)
		}
		seen[id] = true
	}
	if len(got) != 2 {
		t.Fatalf("hibernates = %v, want one victim per start", got)
	}
	if max := atomic.LoadInt32(&f.mr.maxInFlt); max > 1 {
		t.Fatalf("%d make-room hibernates ran at once, want one decider", max)
	}
}

// M9: make-room racing the idle sweep never hibernates a computer twice and
// the start still ends admitted or refused, never wedged.
func TestMakeRoomRacingTheIdleSweep(t *testing.T) {
	for round := 0; round < 20; round++ {
		f := newMakeRoomFixture(t, makeRoomAdmission())
		f.reg.SetIdleTimeout(30 * time.Minute)
		f.activeComputer("v1", "vm-v1", 3*time.Hour, 3*gib)
		f.activeComputer("v2", "vm-v2", 2*time.Hour, 3*gib)
		f.setAvail(5*gib + 1*gib)

		var wg sync.WaitGroup
		wg.Add(2)
		var startErr error
		go func() {
			defer wg.Done()
			var ticket *admissionTicket
			ticket, startErr = f.reg.admitHostMemory(hibernated("s", "vm-s", 2*gib), WarmnessClassPrimary)
			ticket.finish(startErr == nil)
		}()
		go func() {
			defer wg.Done()
			f.reg.StopIdleVMs(context.Background(), allowAllRoutes)
		}()
		waitDone := make(chan struct{})
		go func() { wg.Wait(); close(waitDone) }()
		select {
		case <-waitDone:
		case <-time.After(10 * time.Second):
			t.Fatalf("round %d: make-room and the idle sweep deadlocked", round)
		}
		seen := map[string]bool{}
		for _, id := range f.mr.hibernates() {
			if seen[id] {
				t.Fatalf("round %d: VM %s hibernated twice", round, id)
			}
			seen[id] = true
		}
		if startErr != nil {
			t.Fatalf("round %d: start refused although victims existed: %v", round, startErr)
		}
	}
}

// M10: a new computer that does not fit makes room without holding the
// registry lock across the hibernate, and two requests for the same new
// computer mint one computer.
func TestMakeRoomForANewComputer(t *testing.T) {
	f := newMakeRoomFixture(t, makeRoomAdmission())
	f.activeComputer("idle", "vm-idle", 2*time.Hour, 3*gib)
	f.setAvail(4*gib + 1*gib + 1*gib) // a new computer needs the 2 GiB default
	// The manager tracks what it booted, as vmmanager does, so the request
	// that joins after the boot reads it as running rather than booting it
	// again.
	f.mgr.bootHook = func(cfg VMManagerConfig) {
		f.mgr.mu.Lock()
		f.mgr.getVMs[cfg.VMID] = &VMInstanceInfo{HostURL: "http://127.0.0.1:9001", Epoch: 1, Healthy: true, State: "running"}
		f.mgr.mu.Unlock()
	}
	var lockHeld int32
	f.mr.onHibernate = func(string) {
		// The other request may hold the lock for a moment; a lock held
		// across the whole hibernate never frees.
		for i := 0; i < 200; i++ {
			if f.reg.mu.TryLock() {
				f.reg.mu.Unlock()
				return
			}
			time.Sleep(time.Millisecond)
		}
		atomic.StoreInt32(&lockHeld, 1)
	}

	var wg sync.WaitGroup
	results := make([]*VMOwnership, 2)
	errs := make([]error, 2)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = f.reg.ResolveOrAssign("new-user")
		}(i)
	}
	waitDone := make(chan struct{})
	go func() { wg.Wait(); close(waitDone) }()
	select {
	case <-waitDone:
	case <-time.After(10 * time.Second):
		t.Fatalf("new-computer make-room did not finish (registry lock held across the hibernate?)")
	}
	if atomic.LoadInt32(&lockHeld) != 0 {
		t.Fatalf("the registry lock was held while make-room hibernated a computer")
	}
	for i, err := range errs {
		if err != nil {
			t.Fatalf("resolve %d: %v", i, err)
		}
	}
	if results[0] == nil || results[1] == nil || results[0].VMID != results[1].VMID {
		t.Fatalf("two requests for one new computer got %+v and %+v", results[0], results[1])
	}
	if n := len(f.mgr.boots); n != 1 {
		t.Fatalf("boots = %d (%v), want one new computer", n, func() []string {
			var ids []string
			for _, b := range f.mgr.boots {
				ids = append(ids, b.VMID)
			}
			return ids
		}())
	}
	if got := f.mr.hibernates(); len(got) != 1 || got[0] != "vm-idle" {
		t.Fatalf("hibernates = %v, want vm-idle", got)
	}
}

// M12: protected starts are admitted without making room, as before, and
// guest-size mode is unchanged.
func TestMakeRoomLeavesProtectedStartsAndGuestSizeModeAlone(t *testing.T) {
	f := newMakeRoomFixture(t, makeRoomAdmission())
	f.activeComputer("idle", "vm-idle", 2*time.Hour, 4*gib)
	f.setAvail(3 * gib)
	ticket, err := f.reg.admitHostMemory(hibernated("owner", "vm-owner", 4*gib), WarmnessClassPremiumAlwaysOn)
	if err != nil {
		t.Fatalf("protected start refused: %v", err)
	}
	ticket.finish(true)
	if got := f.mr.hibernates(); len(got) != 0 {
		t.Fatalf("a protected start made room: %v", got)
	}

	gs := makeRoomAdmission()
	gs.Mode = HostAdmissionModeGuestSize
	g := newMakeRoomFixture(t, gs)
	g.activeComputer("idle", "vm-idle", 2*time.Hour, 4*gib)
	g.setAvail(8 * gib)
	if _, err := g.reg.admitHostMemory(hibernated("s", "vm-s", 2*gib), WarmnessClassPrimary); RecoveryRefusalFrom(err) == nil {
		t.Fatalf("guest-size mode admitted at 8 GiB: %v", err)
	}
	if got := g.mr.hibernates(); len(got) != 0 {
		t.Fatalf("guest-size mode made room: %v", got)
	}
}

// M13: the switch off restores today's refusal exactly.
func TestMakeRoomOffIsTodaysRefusal(t *testing.T) {
	off := makeRoomAdmission()
	off.MakeRoom = HostAdmissionMakeRoomOff
	f := newMakeRoomFixture(t, off)
	f.activeComputer("idle", "vm-idle", 2*time.Hour, 4*gib)
	f.setAvail(5 * gib)
	_, err := f.reg.admitHostMemory(hibernated("s", "vm-s", 2*gib), WarmnessClassPrimary)
	refusal := RecoveryRefusalFrom(err)
	if refusal == nil || refusal.RetryAfterSeconds != hostMemoryRetryAfterSeconds ||
		!strings.HasPrefix(refusal.Reason, "host memory available 5120 MiB; 0 MiB reserved for starts in flight; this computer needs 2048 MiB (measured)") {
		t.Fatalf("err = %v, want today's refusal", err)
	}
	if got := f.mr.hibernates(); len(got) != 0 {
		t.Fatalf("make-room off hibernated %v", got)
	}
	if d := lastDecision(t, f.reg); d.Decision != "refuse" {
		t.Fatalf("decision = %q, want refuse", d.Decision)
	}
	if def := normalizeHostAdmissionConfig(HostAdmissionConfig{}); def.MakeRoom != HostAdmissionMakeRoomActive {
		t.Fatalf("default make-room = %q, want active (built features are on)", def.MakeRoom)
	}
}

// M14: make-room runs only inside a start: the pressure monitor and the
// idle sweep on a short host with no start pending hibernate nothing they
// would not have hibernated before.
func TestMakeRoomOnlyInsideAStart(t *testing.T) {
	f := newMakeRoomFixture(t, makeRoomAdmission())
	f.activeComputer("idle", "vm-idle", 10*time.Minute, 4*gib)
	f.setAvail(5 * gib)
	f.reg.pressureMonitorTick(context.Background(), allowAllRoutes)
	if got := f.mr.hibernates(); len(got) != 0 {
		t.Fatalf("pressure monitor without a start hibernated %v", got)
	}
}
