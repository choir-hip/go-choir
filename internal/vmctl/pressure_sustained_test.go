package vmctl

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"testing"
	"time"
)

// Sustained pressure reclaim (density quick tier, lever L2).
// docs/problems/pressure-reclaim-wedged-a-user-computer-2026-10-14.md:
// a 10-second PSI average over 1.0 hibernated a real user's computer
// during `du` scans and deploy builds while 12 GiB was still available, and
// one hibernate's 8 GiB snapshot write set off the next reclaim.
//
// Failure modes pinned here (R1-R10 in the problem doc, written before the
// code):
//   - R1 an avg10 burst, or avg60 pressure with memory above the floor,
//     triggers a reclaim;
//   - R2 a real, sustained shortage is no longer answered;
//   - R3 a sample taken during our own snapshot write, or inside the
//     cool-down after it, counts toward the next reclaim;
//   - R4 two reclaims run at once;
//   - R5 one pass hibernates more than one computer;
//   - R6 a computer a real user touched recently is reclaimed;
//   - R7 an owner-marked computer becomes reclaimable;
//   - R8 a stale window keeps an old streak alive;
//   - R9 the under-capacity keepalive still drops on a burst;
//   - R10 a decision is logged without its inputs;
//   - the instant signal (the rollback) stops behaving as before.

const (
	testFloorBytes = 4 << 30
	testTotalBytes = 32 << 30
)

// blockingHibernateManager holds HibernateVM until released, so a test can
// observe the registry while a snapshot write is in flight.
type blockingHibernateManager struct {
	*processControlVMManager
	hmu        sync.Mutex
	hibernated []string
	started    chan string
	release    chan struct{}
}

func (m *blockingHibernateManager) HibernateVM(vmID string) error {
	if m.started != nil {
		m.started <- vmID
	}
	if m.release != nil {
		select {
		case <-m.release:
		case <-time.After(30 * time.Second):
			return fmt.Errorf("blockingHibernateManager: release not signalled within 30s")
		}
	}
	m.hmu.Lock()
	m.hibernated = append(m.hibernated, vmID)
	m.hmu.Unlock()
	return nil
}

func (m *blockingHibernateManager) hibernateCount() int {
	m.hmu.Lock()
	defer m.hmu.Unlock()
	return len(m.hibernated)
}

type sustainedFixture struct {
	reg    *OwnershipRegistry
	mgr    *blockingHibernateManager
	now    *time.Time
	sample HostPressureSample
	mu     sync.Mutex
}

func sustainedConfig() PressureReclaimConfig {
	return PressureReclaimConfig{
		Mode:                    PressureReclaimModeActive,
		Signal:                  PressureReclaimSignalSustained,
		MinIdle:                 30 * time.Minute,
		MinMemoryAvailableBytes: testFloorBytes,
		MaxMemorySomeAvg10:      1.0,
		MaxMemorySomeAvg60:      5.0,
		SustainedSamples:        4,
		SampleInterval:          15 * time.Second,
		Cooldown:                3 * time.Minute,
		MaxCandidates:           5,
	}
}

func newSustainedFixture(t *testing.T, cfg PressureReclaimConfig) *sustainedFixture {
	t.Helper()
	mgr := &blockingHibernateManager{processControlVMManager: newProcessControlVMManager()}
	reg, now := hostCapacityRegistry(t, mgr)
	f := &sustainedFixture{reg: reg, mgr: mgr, now: now}
	reg.SetPressureReclaimConfig(cfg)
	reg.setGuestBusyProbeForTest(func(*VMOwnership) bool { return false })
	reg.setPressureSamplerForTest(func(PressureReclaimConfig) HostPressureSample {
		f.mu.Lock()
		defer f.mu.Unlock()
		return f.sample
	})
	return f
}

// addIdle adds an active, idle computer for a real user.
func (f *sustainedFixture) addIdle(userID, vmID string, idleFor time.Duration) {
	addOwnership(f.reg, &VMOwnership{VMID: vmID, UserID: userID, DesktopID: PrimaryDesktopID, Kind: VMKindInteractive,
		State: VMStateActive, LastActiveAt: f.now.Add(-idleFor)})
	f.mgr.getVMs[vmID] = &VMInstanceInfo{State: "running"}
}

func (f *sustainedFixture) set(sample HostPressureSample) {
	f.mu.Lock()
	f.sample = sample
	f.mu.Unlock()
}

// tick advances the clock one sample interval and runs one monitor tick.
func (f *sustainedFixture) tick() {
	*f.now = f.now.Add(15 * time.Second)
	f.reg.pressureMonitorTick(context.Background(), allowAllRoutes)
}

func memSample(availBytes uint64, avg10, avg60 float64) HostPressureSample {
	return HostPressureSample{
		MemoryTotalBytes:       testTotalBytes,
		MemoryAvailableBytes:   availBytes,
		MemoryAvailablePercent: float64(availBytes) / float64(testTotalBytes) * 100,
		MemorySomeAvg10:        avg10,
		MemorySomeAvg60:        avg60,
	}
}

// shortage is the sustained signal: avg60 over its threshold and
// MemAvailable under the floor.
func shortage() HostPressureSample { return memSample(2<<30, 12, 9) }

// R1: the 11:40 and 11:59 incidents — avg10 at 2.5 to 11.6 with 12 GiB
// available — and an I/O-driven avg60 rise with memory above the floor.
func TestSustainedReclaimIgnoresBursts(t *testing.T) {
	for name, sample := range map[string]HostPressureSample{
		"avg10 burst, memory ample":    memSample(12<<30, 11.6, 0.8),
		"avg60 high, memory ample":     memSample(12<<30, 15, 9),
		"memory low, no stall":         memSample(2<<30, 0, 0),
		"memory low, avg10 burst only": memSample(2<<30, 11.6, 1.2),
	} {
		f := newSustainedFixture(t, sustainedConfig())
		f.addIdle("u1", "vm-1", 2*time.Hour)
		f.set(sample)
		for i := 0; i < 20; i++ {
			f.tick()
		}
		if got := f.mgr.hibernateCount(); got != 0 {
			t.Fatalf("%s: %d reclaims, want none", name, got)
		}
		if st := f.reg.SustainedPressureState(); st.Sustained {
			t.Fatalf("%s: reported sustained: %+v", name, st)
		}
	}
}

// R2, R5: N consecutive qualifying samples reclaim exactly one computer.
func TestSustainedReclaimFiresAfterConsecutiveSamplesOneAtATime(t *testing.T) {
	f := newSustainedFixture(t, sustainedConfig())
	f.addIdle("u1", "vm-1", 2*time.Hour)
	f.addIdle("u2", "vm-2", 3*time.Hour)
	f.addIdle("u3", "vm-3", 4*time.Hour)
	f.set(shortage())
	for i := 0; i < 3; i++ {
		f.tick()
	}
	if got := f.mgr.hibernateCount(); got != 0 {
		t.Fatalf("reclaimed after 3 of 4 samples: %d", got)
	}
	f.tick()
	if got := f.mgr.hibernateCount(); got != 1 {
		t.Fatalf("after 4 sustained samples: %d reclaims, want exactly 1", got)
	}
	// The longest-idle computer goes first.
	if f.mgr.hibernated[0] != "vm-3" {
		t.Fatalf("reclaimed %s, want vm-3 (longest idle)", f.mgr.hibernated[0])
	}
}

// One non-qualifying sample breaks the streak.
func TestSustainedStreakResetsOnARecoveredSample(t *testing.T) {
	f := newSustainedFixture(t, sustainedConfig())
	f.addIdle("u1", "vm-1", 2*time.Hour)
	for i := 0; i < 10; i++ {
		if i%3 == 2 {
			f.set(memSample(12<<30, 0, 0))
		} else {
			f.set(shortage())
		}
		f.tick()
	}
	if got := f.mgr.hibernateCount(); got != 0 {
		t.Fatalf("interrupted streak reclaimed %d", got)
	}
}

// R3: after a reclaim, pressure inside the cool-down (our own snapshot
// write) does not count; the next reclaim needs a fresh streak after it.
func TestSustainedReclaimCooldownCoversOwnSnapshotWrite(t *testing.T) {
	f := newSustainedFixture(t, sustainedConfig())
	f.addIdle("u1", "vm-1", 2*time.Hour)
	f.addIdle("u2", "vm-2", 3*time.Hour)
	f.set(shortage())
	for i := 0; i < 4; i++ {
		f.tick()
	}
	if got := f.mgr.hibernateCount(); got != 1 {
		t.Fatalf("first reclaim: %d", got)
	}
	// 3 minutes of continued pressure: all inside the cool-down.
	for i := 0; i < 12; i++ {
		f.tick()
	}
	if got := f.mgr.hibernateCount(); got != 1 {
		t.Fatalf("reclaimed inside the cool-down: %d", got)
	}
	st := f.reg.SustainedPressureState()
	if st.Streak != 0 {
		t.Fatalf("cool-down samples counted toward the streak: %+v", st)
	}
	// After the cool-down, a fresh streak of 4 is needed.
	for i := 0; i < 3; i++ {
		f.tick()
	}
	if got := f.mgr.hibernateCount(); got != 1 {
		t.Fatalf("reclaimed before a fresh streak: %d", got)
	}
	f.tick()
	if got := f.mgr.hibernateCount(); got != 2 {
		t.Fatalf("second reclaim after cool-down and a fresh streak: %d", got)
	}
}

// R3, R4: while any hibernate writes its snapshot (an idle-sweep one here),
// samples are suppressed and a reclaim call does nothing.
func TestSustainedReclaimWaitsForHibernateInFlight(t *testing.T) {
	f := newSustainedFixture(t, sustainedConfig())
	f.addIdle("u1", "vm-1", 2*time.Hour)
	f.addIdle("u2", "vm-2", 3*time.Hour)
	f.mgr.started = make(chan string, 1)
	f.mgr.release = make(chan struct{})

	done := make(chan error, 1)
	go func() { done <- f.reg.HibernateVMForDesktop("u2", PrimaryDesktopID) }()
	<-f.mgr.started

	f.set(shortage())
	for i := 0; i < 8; i++ {
		f.tick()
	}
	st := f.reg.SustainedPressureState()
	if st.HibernatesInFlight != 1 || st.Streak != 0 {
		t.Fatalf("in-flight hibernate not suppressing samples: %+v", st)
	}
	if n := f.reg.ReclaimPressureVMs(context.Background(), allowAllRoutes); n != 0 {
		t.Fatalf("reclaim ran beside a hibernate in flight: %d", n)
	}
	close(f.mgr.release)
	if err := <-done; err != nil {
		t.Fatalf("idle hibernate: %v", err)
	}
	if got := f.mgr.hibernateCount(); got != 1 {
		t.Fatalf("hibernates = %d, want only the idle one", got)
	}
}

// R4: concurrent reclaim calls (monitor, sweep, HTTP endpoint) hibernate one.
func TestSustainedReclaimSingleFlight(t *testing.T) {
	cfg := sustainedConfig()
	f := newSustainedFixture(t, cfg)
	f.addIdle("u1", "vm-1", 2*time.Hour)
	f.addIdle("u2", "vm-2", 3*time.Hour)
	f.set(shortage())
	for i := 0; i < 3; i++ {
		f.tick()
	}
	// The fourth sample completes the streak without acting.
	*f.now = f.now.Add(15 * time.Second)
	f.reg.recordPressureSample(f.sample)
	f.mgr.started = make(chan string, 4)
	f.mgr.release = make(chan struct{})

	var wg sync.WaitGroup
	results := make(chan int, 3)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- f.reg.ReclaimPressureVMs(context.Background(), allowAllRoutes)
		}()
	}
	<-f.mgr.started
	// Give the other callers time to reach the gate.
	time.Sleep(50 * time.Millisecond)
	close(f.mgr.release)
	wg.Wait()
	close(results)
	total := 0
	for n := range results {
		total += n
	}
	if total != 1 || f.mgr.hibernateCount() != 1 {
		t.Fatalf("concurrent reclaims hibernated %d (count %d), want 1", total, f.mgr.hibernateCount())
	}
}

// R6, R7: a real user's computer touched inside MinIdle, a busy guest, and
// owner-marked computers are never reclaimed; a test computer yields.
func TestSustainedReclaimProtectsActiveUsersAndOwnerMarked(t *testing.T) {
	f := newSustainedFixture(t, sustainedConfig())
	f.reg.SetWarmnessPolicyConfig(WarmnessPolicyConfig{
		PaidUserIDs: map[string]bool{"owner": true},
		TestUserIDs: map[string]bool{"qa": true},
	})
	f.addIdle("recent", "vm-recent", 5*time.Minute)
	f.addIdle("owner", "vm-owner", 5*time.Hour)
	f.addIdle("busy", "vm-busy", 5*time.Hour)
	addOwnership(f.reg, &VMOwnership{VMID: "vm-paper", UserID: "paper", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive,
		WarmnessClass: WarmnessClassCriticalProtected, State: VMStateActive, LastActiveAt: f.now.Add(-5 * time.Hour)})
	f.reg.setGuestBusyProbeForTest(func(own *VMOwnership) bool { return own.VMID == "vm-busy" })
	f.set(shortage())

	for i := 0; i < 40; i++ {
		f.tick()
	}
	if got := f.mgr.hibernateCount(); got != 0 {
		t.Fatalf("reclaimed protected or recently active computers: %v", f.mgr.hibernated)
	}

	// A test computer touched a minute ago still yields to real ones.
	f.addIdle("qa", "vm-qa", time.Minute)
	for i := 0; i < 20; i++ {
		f.tick()
	}
	if got := f.mgr.hibernateCount(); got != 1 || f.mgr.hibernated[0] != "vm-qa" {
		t.Fatalf("hibernated %v, want only vm-qa", f.mgr.hibernated)
	}
}

// R8: a streak whose newest sample is older than twice the interval is
// stale: a stopped monitor never leaves a reclaim armed.
func TestSustainedStaleWindowDoesNotReclaim(t *testing.T) {
	f := newSustainedFixture(t, sustainedConfig())
	f.addIdle("u1", "vm-1", 2*time.Hour)
	f.set(shortage())
	for i := 0; i < 3; i++ {
		f.tick()
	}
	*f.now = f.now.Add(15 * time.Second)
	f.reg.recordPressureSample(f.sample)
	*f.now = f.now.Add(time.Minute)
	if st := f.reg.SustainedPressureState(); st.Sustained || !st.Stale {
		t.Fatalf("stale streak still armed: %+v", st)
	}
	if n := f.reg.ReclaimPressureVMs(context.Background(), allowAllRoutes); n != 0 {
		t.Fatalf("stale streak reclaimed %d", n)
	}
}

// R9: under the sustained signal, one burst during an idle sweep does not
// drop the under-capacity keepalive of idle primaries.
func TestKeepaliveUnderSustainedSignalIgnoresBurst(t *testing.T) {
	f := newSustainedFixture(t, sustainedConfig())
	f.reg.SetIdleTimeout(30 * time.Minute)
	f.reg.SetWarmnessPolicyConfig(WarmnessPolicyConfig{PrimaryKeepaliveMode: PrimaryKeepaliveModeUnderCapacity})
	addOwnership(f.reg, &VMOwnership{VMID: "vm-1", UserID: "u1", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive,
		State: VMStateActive, LastActiveAt: time.Now().Add(-2 * time.Hour)})
	f.set(memSample(2<<30, 11.6, 1.2))
	f.tick()
	if idle := f.reg.CheckIdleOwnerships(); len(idle) != 0 {
		t.Fatalf("burst dropped the keepalive: %d idle", len(idle))
	}
	f.set(shortage())
	for i := 0; i < 4; i++ {
		*f.now = f.now.Add(15 * time.Second)
		f.reg.recordPressureSample(f.sample)
	}
	if idle := f.reg.CheckIdleOwnerships(); len(idle) != 1 {
		t.Fatalf("sustained shortage kept the keepalive: %d idle", len(idle))
	}
}

// R10: a reclaim logs the inputs it decided on.
func TestSustainedReclaimLogsDecisionInputs(t *testing.T) {
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)

	f := newSustainedFixture(t, sustainedConfig())
	f.addIdle("u1", "vm-1", 2*time.Hour)
	f.set(shortage())
	for i := 0; i < 4; i++ {
		f.tick()
	}
	out := buf.String()
	for _, want := range []string{"decision=reclaim", "signal=sustained", "streak=4/4", "mem_available_mib=2048", "floor_mib=4096", "some_avg10=12.00", "some_avg60=9.00", "vm=vm-1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("reclaim log missing %q:\n%s", want, out)
		}
	}
}

// Rollback: the instant signal keeps today's single-sample decision and
// per-pass bound.
func TestInstantSignalKeepsTodaysSingleSampleReclaim(t *testing.T) {
	cfg := sustainedConfig()
	cfg.Signal = PressureReclaimSignalInstant
	cfg.MaxCandidates = 5
	f := newSustainedFixture(t, cfg)
	f.addIdle("u1", "vm-1", 2*time.Hour)
	f.addIdle("u2", "vm-2", 3*time.Hour)
	f.set(memSample(12<<30, 2.5, 0.3))
	if n := f.reg.ReclaimPressureVMs(context.Background(), allowAllRoutes); n != 2 {
		t.Fatalf("instant signal reclaimed %d on an avg10 burst, want 2 (today's behaviour)", n)
	}
}
