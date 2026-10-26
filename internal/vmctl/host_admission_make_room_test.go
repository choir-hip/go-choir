package vmctl

import (
	"context"
	"errors"
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
	// failWith, when set, is asked before a hibernate frees anything; a
	// non-nil answer is the manager's error and the VM keeps running.
	failWith func(vmID string) error
	// afterFirst is a hook for tests that change a second victim once the
	// first has been hibernated.
	afterFirst func()
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
	if fail := m.failWith; fail != nil {
		if err := fail(vmID); err != nil {
			return err
		}
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
	// Review 302 F3: red features fail closed. The code default is off;
	// nix/node-b.nix sets active explicitly.
	if def := normalizeHostAdmissionConfig(HostAdmissionConfig{}); def.MakeRoom != HostAdmissionMakeRoomOff {
		t.Fatalf("default make-room = %q, want off", def.MakeRoom)
	}
	if def := DefaultHostAdmissionConfig(); def.MakeRoom != HostAdmissionMakeRoomOff {
		t.Fatalf("DefaultHostAdmissionConfig make-room = %q, want off", def.MakeRoom)
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

// ---------------------------------------------------------------------------
// Review 302 (Devin and DeepSeek, lane/density abcf4f577) fixes. Each test
// below fails on abcf4f577.
// ---------------------------------------------------------------------------

// busyHibernateError is what the Firecracker manager returns when the
// guest's fence answered 409: the guest is working and was left running.
type busyHibernateError struct{}

func (busyHibernateError) Error() string            { return "guest busy: running_runs=1" }
func (busyHibernateError) HibernateGuestBusy() bool { return true }

// twoVictimPlan is a start that needs both v1 and v2: 2.5 GiB short, two
// 1.5 GiB computers.
func twoVictimPlan(t *testing.T) *makeRoomFixture {
	t.Helper()
	f := newMakeRoomFixture(t, makeRoomAdmission())
	f.activeComputer("v1", "vm-v1", 3*time.Hour, 1536*mib)
	f.activeComputer("v2", "vm-v2", 2*time.Hour, 1536*mib)
	f.setAvail(4*gib + 1*gib + 4*gib - 2560*mib)
	return f
}

// F1: the plan is a snapshot. Between selecting a victim and hibernating it
// (after the previous victim's snapshot write and settle wait) the computer
// may have become working, held, recently used or unroutable. Immediately
// before each hibernate make-room re-reads the live ownership and probes
// again; a victim that no longer qualifies is spared.
func TestMakeRoomRechecksEachVictimBeforeItsHibernate(t *testing.T) {
	cases := []struct {
		name string
		turn func(f *makeRoomFixture, flip *int32)
	}{
		{"busy", func(f *makeRoomFixture, flip *int32) {
			f.reg.setGuestBusyProbeForTest(func(own *VMOwnership) bool {
				return own.VMID == "vm-v2" && atomic.LoadInt32(flip) != 0
			})
		}},
		{"held", func(f *makeRoomFixture, flip *int32) {
			f.mr.afterFirst = func() {
				f.reg.mu.Lock()
				f.reg.vmByID["vm-v2"].HoldStatus = &MaintenanceHold{Reason: "operator"}
				f.reg.mu.Unlock()
			}
		}},
		{"touched", func(f *makeRoomFixture, flip *int32) {
			f.mr.afterFirst = func() {
				f.reg.mu.Lock()
				f.reg.vmByID["vm-v2"].LastActiveAt = *f.now
				f.reg.mu.Unlock()
			}
		}},
		{"route", func(f *makeRoomFixture, flip *int32) {
			f.reg.SetMakeRoomRouteGuard(func(_ context.Context, userID, _ string) error {
				if userID == "v2" && atomic.LoadInt32(flip) != 0 {
					return errors.New("route moved")
				}
				return nil
			})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := twoVictimPlan(t)
			var flip int32
			tc.turn(f, &flip)
			f.mr.onHibernate = func(vmID string) {
				if vmID != "vm-v1" {
					return
				}
				atomic.StoreInt32(&flip, 1)
				if f.mr.afterFirst != nil {
					f.mr.afterFirst()
				}
			}
			_, err := f.reg.admitHostMemory(hibernated("starter", "vm-s", 4*gib), WarmnessClassPrimary)
			if RecoveryRefusalFrom(err) == nil {
				t.Fatalf("err = %v, want a refusal: the second victim stopped qualifying", err)
			}
			if got := f.mr.hibernates(); len(got) != 1 || got[0] != "vm-v1" {
				t.Fatalf("hibernates = %v, want only vm-v1; vm-v2 turned %s before its hibernate", got, tc.name)
			}
			if st, _ := f.state("vm-v2"); st != VMStateActive {
				t.Fatalf("vm-v2 state = %s, want left running", st)
			}
		})
	}
}

// Substrate (docs/problems/busy-guest-hibernate-takes-kill-path-2026-10-26.md):
// a guest that answers busy at the fence is left running by every caller. The
// manager's skip error must not turn into a recorded hibernate, and the idle
// sweep counts it as nothing stopped.
func TestHibernateGuestBusyIsASkipForTheIdleSweepAndTheNextSweepRetries(t *testing.T) {
	f := newMakeRoomFixture(t, makeRoomAdmission())
	f.reg.SetIdleTimeout(30 * time.Minute)
	f.activeComputer("w", "vm-w", 3*time.Hour, 1*gib)
	var busy int32 = 1
	f.mr.failWith = func(string) error {
		if atomic.LoadInt32(&busy) != 0 {
			return busyHibernateError{}
		}
		return nil
	}
	if n := f.reg.StopIdleVMs(context.Background(), allowAllRoutes); n != 0 {
		t.Fatalf("idle sweep stopped %d computers although the guest answered busy", n)
	}
	if st, by := f.state("vm-w"); st != VMStateActive || by != "" {
		t.Fatalf("state=%s stopped_by=%q, want the busy guest left active", st, by)
	}
	f.reg.mu.RLock()
	claims := len(f.reg.lifecycle)
	f.reg.mu.RUnlock()
	if claims != 0 {
		t.Fatalf("%d lifecycle claims left after a skipped hibernate", claims)
	}
	if err := f.reg.hibernateVMForDesktopWithReason("w", PrimaryDesktopID, "idle"); err == nil {
		t.Fatalf("a skipped hibernate returned nil, the caller would record a hibernate")
	}
	atomic.StoreInt32(&busy, 0)
	if n := f.reg.StopIdleVMs(context.Background(), allowAllRoutes); n != 1 {
		t.Fatalf("next sweep stopped %d, want the now-idle computer hibernated", n)
	}
}

// Substrate in make-room: a victim whose fence answers busy is spared and
// counted as a skip; the start goes on to the next victim, and the refusal
// names the skip when nothing else can make room.
func TestMakeRoomCountsABusyFenceAsASkip(t *testing.T) {
	f := twoVictimPlan(t)
	f.activeComputer("v3", "vm-v3", 1*time.Hour, 1536*mib)
	f.mr.failWith = func(vmID string) error {
		if vmID == "vm-v1" {
			return busyHibernateError{}
		}
		return nil
	}
	ticket, err := f.reg.admitHostMemory(hibernated("starter", "vm-s", 4*gib), WarmnessClassPrimary)
	if err != nil {
		t.Fatalf("start refused although vm-v2 and vm-v3 could make room: %v", err)
	}
	ticket.finish(true)
	if st, by := f.state("vm-v1"); st != VMStateActive || by != "" {
		t.Fatalf("vm-v1 state=%s stopped_by=%q, want the busy computer left running", st, by)
	}
	d := lastDecision(t, f.reg)
	if d.Decision != "admit_after_make_room" || strings.Join(d.MadeRoomVMIDs, " ") != "vm-v2 vm-v3" {
		t.Fatalf("decision = %+v, want admit naming only the computers actually hibernated", d)
	}

	g := twoVictimPlan(t)
	g.mr.failWith = func(string) error { return busyHibernateError{} }
	_, err = g.reg.admitHostMemory(hibernated("starter", "vm-s", 4*gib), WarmnessClassPrimary)
	refusal := RecoveryRefusalFrom(err)
	if refusal == nil || !strings.Contains(refusal.Reason, "hibernate_busy=2") {
		t.Fatalf("err = %v, want a refusal that counts the two busy fences as skips", err)
	}
	for _, id := range []string{"vm-v1", "vm-v2"} {
		if st, _ := g.state(id); st != VMStateActive {
			t.Fatalf("%s state = %s, want left running", id, st)
		}
	}
}

// F2 (DeepSeek): the shortfall is recomputed from a fresh sample after each
// hibernate. A concurrent start took the room the first victim freed, so the
// first victim alone no longer covers it; the next eligible victim goes.
func TestMakeRoomReplansAfterEachVictim(t *testing.T) {
	f := newMakeRoomFixture(t, makeRoomAdmission())
	f.activeComputer("v1", "vm-v1", 3*time.Hour, 1536*mib)
	f.activeComputer("v2", "vm-v2", 2*time.Hour, 1536*mib)
	f.activeComputer("v3", "vm-v3", 1*time.Hour, 1536*mib)
	f.setAvail(4*gib + 1*gib + 2*gib - 1*gib) // 1 GiB short for a 2 GiB start
	f.mr.onHibernate = func(vmID string) {
		if vmID == "vm-v1" {
			// Another start's reservation lands as v1's memory frees.
			f.mu.Lock()
			f.avail -= 1536 * mib
			f.mu.Unlock()
		}
	}
	ticket, err := f.reg.admitHostMemory(hibernated("starter", "vm-s", 2*gib), WarmnessClassPrimary)
	if err != nil {
		t.Fatalf("start refused although vm-v2 could make room: %v", err)
	}
	ticket.finish(true)
	if got := f.mr.hibernates(); strings.Join(got, " ") != "vm-v1 vm-v2" {
		t.Fatalf("hibernates = %v, want [vm-v1 vm-v2]", got)
	}
	if st, _ := f.state("vm-v3"); st != VMStateActive {
		t.Fatalf("vm-v3 state = %s, want left running: only as many as needed", st)
	}
}

// F2 (DeepSeek): another path's in-flight hibernate (the idle sweep) is not
// this start's room when deciding to refuse. Its memory may never show; the
// start still hibernates a victim of its own rather than giving up.
func TestMakeRoomDoesNotCountAnotherHibernateAsItsOwnRoom(t *testing.T) {
	f := newMakeRoomFixture(t, makeRoomAdmission())
	pending := f.activeComputer("sweep-victim", "vm-x", 3*time.Hour, 3*gib)
	f.activeComputer("y", "vm-y", 2*time.Hour, 1536*mib)
	f.setAvail(4*gib + 1*gib + 2*gib - 1*gib)
	f.reg.mu.Lock()
	f.reg.lifecycle[ownershipKey(pending.UserID, pending.DesktopID)] = lifecycleHibernate
	f.reg.mu.Unlock()

	ticket, err := f.reg.admitHostMemory(hibernated("starter", "vm-s", 2*gib), WarmnessClassPrimary)
	if err != nil {
		t.Fatalf("start refused although vm-y could make room while vm-x's hibernate never landed: %v", err)
	}
	ticket.finish(true)
	if got := f.mr.hibernates(); strings.Join(got, " ") != "vm-y" {
		t.Fatalf("hibernates = %v, want [vm-y]", got)
	}
}

// When the other hibernate does land, the start admits on it and hibernates
// nobody.
func TestMakeRoomAdmitsOnAnotherHibernateThatLands(t *testing.T) {
	f := newMakeRoomFixture(t, makeRoomAdmission())
	pending := f.activeComputer("sweep-victim", "vm-x", 3*time.Hour, 3*gib)
	f.activeComputer("y", "vm-y", 2*time.Hour, 1536*mib)
	f.setAvail(4*gib + 1*gib + 2*gib - 1*gib)
	key := ownershipKey(pending.UserID, pending.DesktopID)
	f.reg.mu.Lock()
	f.reg.lifecycle[key] = lifecycleHibernate
	f.reg.mu.Unlock()
	go func() {
		time.Sleep(30 * time.Millisecond)
		f.addAvail(3 * gib)
		f.reg.mu.Lock()
		delete(f.reg.lifecycle, key)
		f.reg.mu.Unlock()
	}()
	ticket, err := f.reg.admitHostMemory(hibernated("starter", "vm-s", 2*gib), WarmnessClassPrimary)
	if err != nil {
		t.Fatalf("start refused: %v", err)
	}
	ticket.finish(true)
	if got := f.mr.hibernates(); len(got) != 0 {
		t.Fatalf("hibernates = %v, want none: the other hibernate made the room", got)
	}
}

// Nit: the refusal for a start that hibernated nothing does not claim it
// made room, and one that waited for nothing does not say memory "did not
// show".
func TestMakeRoomRefusalTextIsTrue(t *testing.T) {
	f := newMakeRoomFixture(t, makeRoomAdmission())
	pending := f.activeComputer("sweep-victim", "vm-x", 3*time.Hour, 3*gib)
	f.setAvail(4*gib + 1*gib + 2*gib - 1*gib)
	f.reg.mu.Lock()
	f.reg.lifecycle[ownershipKey(pending.UserID, pending.DesktopID)] = lifecycleHibernate
	f.reg.mu.Unlock()
	_, err := f.reg.admitHostMemory(hibernated("starter", "vm-s", 2*gib), WarmnessClassPrimary)
	refusal := RecoveryRefusalFrom(err)
	if refusal == nil {
		t.Fatalf("err = %v, want a refusal", err)
	}
	if strings.Contains(refusal.Reason, "hibernating []") || strings.Contains(refusal.Reason, "made room") {
		t.Fatalf("refusal %q claims room was made; nothing was hibernated", refusal.Reason)
	}
	if !strings.Contains(refusal.Reason, "already being hibernated") {
		t.Fatalf("refusal %q does not say the memory is another hibernate's", refusal.Reason)
	}

	g := twoVictimPlan(t)
	g.freeOnHibernate = false
	_, err = g.reg.admitHostMemory(hibernated("starter", "vm-s", 4*gib), WarmnessClassPrimary)
	refusal = RecoveryRefusalFrom(err)
	if refusal == nil || !strings.Contains(refusal.Reason, "vm-v1") || !strings.Contains(refusal.Reason, "vm-v2") {
		t.Fatalf("err = %v, want a refusal naming both computers it hibernated", err)
	}
}

// F4: a start superseded after it hibernated victims leaves a decision
// naming them; the victims would otherwise stay hibernated with no record.
func TestMakeRoomSupersededStartRecordsWhatItHibernated(t *testing.T) {
	f := newMakeRoomFixture(t, makeRoomAdmission())
	f.activeComputer("v1", "vm-v1", 3*time.Hour, 1536*mib)
	f.activeComputer("v2", "vm-v2", 2*time.Hour, 1536*mib)
	f.setAvail(4*gib + 1*gib + 2*gib - 2560*mib) // a new computer's 2 GiB default
	f.mgr.bootHook = func(cfg VMManagerConfig) {
		f.mgr.mu.Lock()
		f.mgr.getVMs[cfg.VMID] = &VMInstanceInfo{HostURL: "http://127.0.0.1:9001", Epoch: 1, Healthy: true, State: "running"}
		f.mgr.mu.Unlock()
	}
	f.mr.onHibernate = func(vmID string) {
		if vmID != "vm-v1" {
			return
		}
		// Another request registers the same computer while this one waits,
		// and boots it (the manager tracks it, so it needs no admission).
		f.mgr.mu.Lock()
		f.mgr.getVMs["vm-raced"] = &VMInstanceInfo{HostURL: "http://127.0.0.1:9001", Epoch: 1, Healthy: true, State: "running"}
		f.mgr.mu.Unlock()
		addOwnership(f.reg, &VMOwnership{VMID: "vm-raced", UserID: "new-user", DesktopID: PrimaryDesktopID,
			Kind: VMKindInteractive, ComputerID: "computer-raced", State: VMStateActive, LastActiveAt: *f.now,
			ComputerURL: "http://127.0.0.1:9001", Epoch: 1})
	}
	_, _ = f.reg.ResolveOrAssign("new-user")
	if got := f.mr.hibernates(); strings.Join(got, " ") != "vm-v1" {
		t.Fatalf("hibernates = %v, want [vm-v1] before the supersede was seen", got)
	}
	named := false
	for _, d := range f.reg.HostAdmissionSummary().Recent {
		for _, id := range d.MadeRoomVMIDs {
			if id == "vm-v1" {
				named = true
			}
		}
	}
	if !named {
		t.Fatalf("no admission decision names vm-v1: %+v", f.reg.HostAdmissionSummary().Recent)
	}
}

// F5: a cancelled start stops making room. The route guard is asked with the
// start's context, not context.Background().
func TestMakeRoomStopsWhenTheStartIsCancelled(t *testing.T) {
	f := newMakeRoomFixture(t, makeRoomAdmission())
	f.activeComputer("v1", "vm-v1", 3*time.Hour, 1536*mib)
	f.activeComputer("v2", "vm-v2", 2*time.Hour, 1536*mib)
	f.setAvail(4*gib + 1*gib + 2*gib - 2560*mib)
	f.mgr.bootHook = func(cfg VMManagerConfig) {
		f.mgr.mu.Lock()
		f.mgr.getVMs[cfg.VMID] = &VMInstanceInfo{HostURL: "http://127.0.0.1:9001", Epoch: 1, Healthy: true, State: "running"}
		f.mgr.mu.Unlock()
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.reg.SetMakeRoomRouteGuard(func(gctx context.Context, _, _ string) error { return gctx.Err() })
	f.mr.onHibernate = func(vmID string) {
		if vmID == "vm-v1" {
			cancel()
		}
	}
	_, err := f.reg.ResolveOrAssignDesktopContext(ctx, "new-user", PrimaryDesktopID)
	if err == nil {
		t.Fatalf("a cancelled start was admitted")
	}
	if got := f.mr.hibernates(); strings.Join(got, " ") != "vm-v1" {
		t.Fatalf("hibernates = %v, want the cancelled start to stop after vm-v1", got)
	}
}

// F3: red features fail closed. Only an exact enabling word turns make-room
// on; a typo or an observation word is off.
func TestMakeRoomConfigFailsClosed(t *testing.T) {
	for value, want := range map[string]string{
		"": HostAdmissionMakeRoomOff, "of": HostAdmissionMakeRoomOff, "observe": HostAdmissionMakeRoomOff,
		"garbage": HostAdmissionMakeRoomOff, "off": HostAdmissionMakeRoomOff, "false": HostAdmissionMakeRoomOff,
		"activee": HostAdmissionMakeRoomOff,
		"active":  HostAdmissionMakeRoomActive, " Active ": HostAdmissionMakeRoomActive, "ON": HostAdmissionMakeRoomActive,
		"enabled": HostAdmissionMakeRoomActive, "true": HostAdmissionMakeRoomActive, "1": HostAdmissionMakeRoomActive,
	} {
		if got := normalizeHostAdmissionConfig(HostAdmissionConfig{MakeRoom: value}).MakeRoom; got != want {
			t.Errorf("MakeRoom %q normalizes to %q, want %q", value, got, want)
		}
	}
}

// F5: the bounds are capped where they are read, so a mistyped override
// cannot evict a hundred computers or wait an hour.
func TestMakeRoomBoundsAreCapped(t *testing.T) {
	cfg := normalizeHostAdmissionConfig(HostAdmissionConfig{MakeRoom: "active", MakeRoomMaxVictims: 1000, MakeRoomSettleWait: time.Hour})
	if cfg.MakeRoomMaxVictims != 10 || cfg.MakeRoomSettleWait != 60*time.Second {
		t.Fatalf("bounds = %d victims, %s settle, want 10 and 60s", cfg.MakeRoomMaxVictims, cfg.MakeRoomSettleWait)
	}
}

// Review 310 Low 1: a start cancelled while make-room plans its next victim
// is abandoned, not given a last admission attempt. pick returns nothing on a
// cancelled context, which is not the plan running out of victims: the final
// attempt could admit and boot a request nobody waits for, and a refusal
// after the cancel would be labelled refuse_no_room.
func TestMakeRoomCancelledWhilePlanningIsAbandonedNotAdmitted(t *testing.T) {
	cfg := makeRoomAdmission()
	cfg.MakeRoomSettleWait = time.Nanosecond // one sample per wait, so the sample count is exact
	f := newMakeRoomFixture(t, cfg)
	f.freeOnHibernate = false
	f.activeComputer("v1", "vm-v1", 3*time.Hour, 1536*mib)
	f.activeComputer("v2", "vm-v2", 2*time.Hour, 1536*mib)
	short := 4*gib + 1*gib + 2*gib - 2560*mib
	f.setAvail(short)
	f.mgr.bootHook = func(cfg VMManagerConfig) {
		f.mgr.mu.Lock()
		f.mgr.getVMs[cfg.VMID] = &VMInstanceInfo{HostURL: "http://127.0.0.1:9001", Epoch: 1, Healthy: true, State: "running"}
		f.mgr.mu.Unlock()
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls int32
	f.reg.setPressureSamplerForTest(func(PressureReclaimConfig) HostPressureSample {
		n := atomic.AddInt32(&calls, 1)
		avail := short
		switch {
		case n == 5: // the second pass of the loop: cancelled while it plans
			cancel()
		case n >= 6: // the final attempt, if one is made, would fit
			avail = 16 * gib
		}
		return HostPressureSample{MemoryTotalBytes: 32 * gib, MemoryAvailableBytes: avail,
			MemoryAvailablePercent: float64(avail) / float64(32*gib) * 100}
	})
	_, err := f.reg.ResolveOrAssignDesktopContext(ctx, "new-user", PrimaryDesktopID)
	if err == nil {
		t.Fatalf("a start cancelled while make-room planned was admitted and booted")
	}
	if got := f.mr.hibernates(); strings.Join(got, " ") != "vm-v1" {
		t.Fatalf("hibernates = %v, want [vm-v1] before the cancel", got)
	}
	d := lastDecision(t, f.reg)
	if d.Decision != "abandon_make_room" || len(d.MadeRoomVMIDs) != 1 || d.MadeRoomVMIDs[0] != "vm-v1" {
		t.Fatalf("last decision = %s made %v, want abandon_make_room naming vm-v1", d.Decision, d.MadeRoomVMIDs)
	}
	for _, d := range f.reg.HostAdmissionSummary().Recent {
		if d.Decision == "refuse_no_room" {
			t.Fatalf("a cancelled start was recorded as refused for no room: %+v", d)
		}
	}
}

// Review 310 nit 4: the refusal's numbers are one sample's. The shortfall in
// the detail comes from the latest sample, so the "host memory available"
// figure beside it must too, not the start's first sample.
func TestMakeRoomRefusalTextUsesTheLatestSample(t *testing.T) {
	f := newMakeRoomFixture(t, makeRoomAdmission())
	f.setAvail(5 * gib) // nothing running to hibernate: refused
	var calls int32
	f.reg.setPressureSamplerForTest(func(PressureReclaimConfig) HostPressureSample {
		avail := 5 * gib
		if atomic.AddInt32(&calls, 1) > 2 { // the admission and make-room's own first sample
			avail = 6 * gib // still short, but not by what the first sample said
		}
		return HostPressureSample{MemoryTotalBytes: 32 * gib, MemoryAvailableBytes: avail,
			MemoryAvailablePercent: float64(avail) / float64(32*gib) * 100}
	})
	_, err := f.reg.admitHostMemory(hibernated("starter", "vm-s", 2*gib), WarmnessClassPrimary)
	refusal := RecoveryRefusalFrom(err)
	if refusal == nil {
		t.Fatalf("err = %v, want a refusal", err)
	}
	if !strings.Contains(refusal.Reason, "host memory available 6144 MiB") || !strings.Contains(refusal.Reason, "short 1024 MiB") {
		t.Fatalf("refusal %q mixes samples; want available 6144 MiB beside short 1024 MiB", refusal.Reason)
	}
}

// Review 310 nit 4: one computer is counted once in the refusal's skipped
// list. A victim whose guest answered busy at the fence, and that became held
// meanwhile, is both tried here and skipped by the next plan.
func TestMakeRoomSkippedListCountsAComputerOnce(t *testing.T) {
	f := newMakeRoomFixture(t, makeRoomAdmission())
	f.activeComputer("v1", "vm-v1", 3*time.Hour, 1536*mib)
	f.setAvail(4*gib + 1*gib + 2*gib - 1*gib)
	f.mr.failWith = func(vmID string) error {
		f.reg.mu.Lock()
		f.reg.vmByID[vmID].HoldStatus = &MaintenanceHold{Reason: "operator"}
		f.reg.mu.Unlock()
		return busyHibernateError{}
	}
	_, err := f.reg.admitHostMemory(hibernated("starter", "vm-s", 2*gib), WarmnessClassPrimary)
	refusal := RecoveryRefusalFrom(err)
	if refusal == nil {
		t.Fatalf("err = %v, want a refusal", err)
	}
	if strings.Contains(refusal.Reason, "held=1") && strings.Contains(refusal.Reason, "hibernate_busy=1") {
		t.Fatalf("refusal %q counts one computer under two reasons", refusal.Reason)
	}
	if !strings.Contains(refusal.Reason, "hibernate_busy=1") {
		t.Fatalf("refusal %q does not say the guest answered busy", refusal.Reason)
	}
}

// Review 312 Low 2: a cancel that lands during the last probe of the chosen
// victim (the hold probe runs after pick's own ctx check) does not hibernate
// a computer for a request nobody waits for.
func TestMakeRoomCancelDuringTheLastProbeHibernatesNobody(t *testing.T) {
	f := newMakeRoomFixture(t, makeRoomAdmission())
	f.activeComputer("v1", "vm-v1", 3*time.Hour, 1536*mib)
	f.activeComputer("v2", "vm-v2", 2*time.Hour, 1536*mib)
	f.setAvail(4*gib + 1*gib + 2*gib - 2560*mib)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var probes int32
	f.reg.setGuestBusyProbeForTest(func(own *VMOwnership) bool {
		// The first probe sizes the plan; the second is the one just before
		// the hibernate, and the request goes away during it.
		if own.VMID == "vm-v1" && atomic.AddInt32(&probes, 1) == 2 {
			cancel()
		}
		return false
	})
	_, err := f.reg.ResolveOrAssignDesktopContext(ctx, "new-user", PrimaryDesktopID)
	if err == nil {
		t.Fatalf("a cancelled start was admitted")
	}
	if got := f.mr.hibernates(); len(got) != 0 {
		t.Fatalf("hibernates = %v, want none for a start cancelled during the last probe", got)
	}
}
