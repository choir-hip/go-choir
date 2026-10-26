package vmctl

import (
	"fmt"
	"testing"
	"time"
)

// Density D3: idle hibernation reaches primary computers in minutes
// (VMCTL_PRIMARY_KEEPALIVE_MODE=off, VMCTL_IDLE_TIMEOUT=10m,
// VMCTL_PRESSURE_RECLAIM_MIN_IDLE=10m). Failure modes D3-M1..M8 of
// docs/problems/density-station-d-failure-modes-2026-10-26.md, pinned before
// the nix change. The owner: never a computer with a live run, capsule or
// apply; never the paper, a public_platform or an owner's paid computer.

const (
	densityPaperID = "computer-paper-1355b83b"
)

func densityPolicy() WarmnessPolicyConfig {
	return WarmnessPolicyConfig{
		PrimaryKeepaliveMode: PrimaryKeepaliveModeOff,
		PaidUserIDs:          map[string]bool{"u-paid": true},
		PlatformComputerIDs:  map[string]bool{densityPaperID: true},
		TestUserIDs:          map[string]bool{"u-test": true},
	}
}

type densityComputer struct {
	name  string
	class WarmnessClass
	own   func(idle time.Duration, now time.Time) *VMOwnership
}

func densityComputers() []densityComputer {
	mk := func(userID, computerID string, persisted WarmnessClass) func(time.Duration, time.Time) *VMOwnership {
		return func(idle time.Duration, now time.Time) *VMOwnership {
			return &VMOwnership{VMID: "vm-" + computerID, UserID: userID, DesktopID: PrimaryDesktopID, ComputerID: computerID,
				Kind: VMKindInteractive, State: VMStateActive, WarmnessClass: persisted, LastActiveAt: now.Add(-idle)}
		}
	}
	return []densityComputer{
		{"primary", WarmnessClassPrimary, mk("u-primary", "computer-primary", "")},
		{"test", WarmnessClassTest, mk("u-test", "computer-test", "")},
		{"premium_always_on (the owner's)", WarmnessClassPremiumAlwaysOn, mk("u-paid", "computer-paid", "")},
		{"critical_protected (the paper)", WarmnessClassCriticalProtected, mk("u-paper", densityPaperID, "")},
		{"public_platform", WarmnessClassPublicPlatform, mk("u-public", "computer-public", WarmnessClassPublicPlatform)},
	}
}

// D3-M1, D3-M4, D3-M5: with keepalive off and a 10 minute timer, exactly the
// idle, unheld, non-busy primary and test computers are candidates.
func TestIdleCandidatesAtTenMinutesExactlyTheUnprotectedIdleUnheldNonBusy(t *testing.T) {
	now := time.Now()
	for _, c := range densityComputers() {
		if got := warmnessClassForOwnership(c.own(0, now), densityPolicy()); got != c.class {
			t.Fatalf("%s: class = %s, want %s (fixture)", c.name, got, c.class)
		}
		for _, idle := range []time.Duration{9 * time.Minute, 11 * time.Minute} {
			for _, busy := range []bool{false, true} {
				for _, held := range []bool{false, true} {
					own := c.own(idle, now)
					if held {
						own.HoldStatus = &MaintenanceHold{}
					}
					got := idleOwnershipCandidates([]*VMOwnership{own}, densityPolicy(), HostPressureSample{}, 10*time.Minute, now,
						func(*VMOwnership) bool { return busy })
					want := (c.class == WarmnessClassPrimary || c.class == WarmnessClassTest) && idle > 10*time.Minute && !busy && !held
					if (len(got) == 1) != want {
						t.Errorf("%s idle=%s busy=%v held=%v: candidate=%v, want %v", c.name, idle, busy, held, len(got) == 1, want)
					}
				}
			}
		}
	}
}

// D3-M8: the keepalive mode. The registry normalizes empty to off; under
// capacity still keeps an idle primary computer unless the host is under
// sustained pressure, so the nix value is what changes the behavior.
func TestPrimaryKeepaliveModesAtTenMinutes(t *testing.T) {
	now := time.Now()
	primary := densityComputers()[0].own(11*time.Minute, now)
	off := densityPolicy()
	under := densityPolicy()
	under.PrimaryKeepaliveMode = PrimaryKeepaliveModeUnderCapacity
	idle := func(cfg WarmnessPolicyConfig) int {
		return len(idleOwnershipCandidates([]*VMOwnership{primary}, cfg, HostPressureSample{}, 10*time.Minute, now, nil))
	}
	if idle(off) != 1 {
		t.Fatalf("keepalive off: an idle primary computer is not a candidate")
	}
	if idle(under) != 0 {
		t.Fatalf("keepalive under-capacity: an idle primary computer is a candidate with no pressure")
	}
}

// D3-M4 through the registry: the idle sweep, with the guest probe stubbed to
// report no live capsule, takes only the primary and test computers.
func TestIdleSweepAtTenMinutesSparesProtectedClasses(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetIdleTimeout(10 * time.Minute)
	reg.SetWarmnessPolicyConfig(densityPolicy())
	reg.setGuestHealthProbeForTest(func(*VMOwnership) guestHealthReading { return guestHealthReading{ReportsCapsules: true} })
	now := time.Now()
	for _, c := range densityComputers() {
		addOwnership(reg, c.own(11*time.Minute, now))
	}
	got := map[string]bool{}
	for _, own := range reg.CheckIdleOwnerships() {
		got[own.ComputerID] = true
	}
	if len(got) != 2 || !got["computer-primary"] || !got["computer-test"] {
		t.Fatalf("idle sweep took %v, want only computer-primary and computer-test", got)
	}
}

// D3-M4: pressure reclaim at a 10 minute minimum idle must not take a
// public_platform computer (it excluded only premium_always_on and
// critical_protected), the paper or an owner's computer.
func TestPressureReclaimAtTenMinutesSparesProtectedClasses(t *testing.T) {
	now := time.Now()
	var all []*VMOwnership
	for _, c := range densityComputers() {
		all = append(all, c.own(11*time.Minute, now))
	}
	cfg := normalizePressureReclaimConfig(PressureReclaimConfig{
		Mode: PressureReclaimModeActive, Signal: PressureReclaimSignalSustained,
		MinIdle: 10 * time.Minute, MinMemoryAvailableBytes: 4 * gib, MaxCandidates: 10,
	})
	eligible := map[string]bool{}
	for _, cand := range rankPressureCandidates(all, cfg, densityPolicy(), now, func(*VMOwnership) bool { return false }) {
		if !cand.public.Protected {
			eligible[cand.own.ComputerID] = true
		}
	}
	if len(eligible) != 2 || !eligible["computer-primary"] || !eligible["computer-test"] {
		t.Fatalf("pressure reclaim may take %v, want only computer-primary and computer-test", eligible)
	}
}

// D3-M4 through make-room: a public_platform, paper or owner computer is
// never a victim, however idle.
func TestMakeRoomSparesProtectedClassesAtTenMinutes(t *testing.T) {
	f := newMakeRoomFixture(t, makeRoomAdmission())
	f.reg.SetWarmnessPolicyConfig(densityPolicy())
	f.reg.SetPressureReclaimConfig(PressureReclaimConfig{
		Mode: PressureReclaimModeActive, Signal: PressureReclaimSignalSustained,
		MinMemoryAvailableBytes: 4 * gib, MinIdle: 10 * time.Minute,
	})
	for i, c := range densityComputers() {
		if c.class == WarmnessClassPrimary || c.class == WarmnessClassTest {
			continue
		}
		own := c.own(3*time.Hour, *f.now)
		own.ComputerURL = "http://10.0.0.9:8085"
		addOwnership(f.reg, own)
		f.mgr.setRSS(own.VMID, 3*gib)
		_ = i
	}
	f.reg.sampleVMMemory()
	f.setAvail(5 * gib)
	_, err := f.reg.admitHostMemory(hibernated("starter", "vm-s", 2*gib), WarmnessClassPrimary)
	if RecoveryRefusalFrom(err) == nil || len(f.mr.hibernates()) != 0 {
		t.Fatalf("err=%v hibernates=%v, want a refusal with every protected computer left running", err, f.mr.hibernates())
	}
}

// D3-M6 (PR-09): a host build with no traffic leaves LastActiveAt older than
// the timer; the guest was busy until its apply restart and reads idle just
// after. Work seen by the sweep counts as activity, so the first sweep after
// the restart does not hibernate it.
func TestSweepSeesWorkAsActivityAfterAnApplyRestart(t *testing.T) {
	busy := true
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetIdleTimeout(10 * time.Minute)
	reg.SetWarmnessPolicyConfig(densityPolicy())
	reg.setGuestHealthProbeForTest(func(*VMOwnership) guestHealthReading {
		return guestHealthReading{Busy: busy, ReportsCapsules: true}
	})
	now := time.Now()
	own := densityComputers()[1].own(14*time.Minute, now) // last resolved 14 minutes ago: the build
	addOwnership(reg, own)

	if got := reg.CheckIdleOwnerships(); len(got) != 0 {
		t.Fatalf("a busy guest was an idle candidate: %v", got)
	}
	busy = false // the apply restart booted a guest with no work
	if got := reg.CheckIdleOwnerships(); len(got) != 0 {
		t.Fatalf("the first sweep after the work ended hibernated the computer (PR-09): %v", got)
	}

	// Ten minutes after the last work it is idle again.
	reg.mu.Lock()
	reg.guestLastBusyAt[own.VMID] = now.Add(-11 * time.Minute)
	reg.mu.Unlock()
	if got := reg.CheckIdleOwnerships(); len(got) != 1 {
		t.Fatalf("a computer idle 11 minutes since its last work was not a candidate: %v", got)
	}
}

// D3-M6 for the other deciders: make-room and pressure reclaim measure idle
// from the same later time.
func TestMakeRoomSeesWorkAsActivity(t *testing.T) {
	f := newMakeRoomFixture(t, makeRoomAdmission())
	f.reg.SetPressureReclaimConfig(PressureReclaimConfig{
		Mode: PressureReclaimModeActive, Signal: PressureReclaimSignalSustained,
		MinMemoryAvailableBytes: 4 * gib, MinIdle: 10 * time.Minute,
	})
	f.activeComputer("worker", "vm-w", 14*time.Minute, 3*gib)
	f.reg.mu.Lock()
	f.reg.guestLastBusyAt = map[string]time.Time{"vm-w": f.now.Add(-2 * time.Minute)}
	f.reg.mu.Unlock()
	f.setAvail(5 * gib)
	_, err := f.reg.admitHostMemory(hibernated("starter", "vm-s", 2*gib), WarmnessClassPrimary)
	if RecoveryRefusalFrom(err) == nil || len(f.mr.hibernates()) != 0 {
		t.Fatalf("err=%v hibernates=%v, want the computer that worked 2 minutes ago left running", err, f.mr.hibernates())
	}
}

// D3-M7: a computer driven only by API keys has a stale LastActiveAt and a
// busy guest: the busy probe is the fence, whatever the clock says.
func TestKeyDrivenBusyComputerIsNeverIdleHibernated(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetIdleTimeout(10 * time.Minute)
	reg.SetWarmnessPolicyConfig(densityPolicy())
	reg.setGuestHealthProbeForTest(func(*VMOwnership) guestHealthReading {
		return guestHealthReading{Busy: true, ReportsCapsules: true}
	})
	for i, c := range densityComputers()[:2] {
		own := c.own(3*time.Hour, time.Now())
		own.VMID = fmt.Sprintf("vm-key-%d", i)
		own.ComputerID = fmt.Sprintf("computer-key-%d", i)
		addOwnership(reg, own)
	}
	if got := reg.CheckIdleOwnerships(); len(got) != 0 {
		t.Fatalf("a busy guest was idle-hibernated: %v", got)
	}
}
