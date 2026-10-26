package vmctl

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Density D3a (docs/problems/idle-hibernate-blind-to-live-capsules-2026-10-26.md):
// the guest's /health reports live_capsules, and vmctl's busy probe reads it.
// A guest whose /health lacks the field cannot be told from one holding a
// parked capsule, so it keeps the legacy idle rule (30 minutes) while the
// shorter timer applies only to guests that report the field.
//
// Ways this could fail, written first:
//   - a live capsule reads idle (the field is not read);
//   - a guest without the field is treated as busy forever, or as reporting
//     "no live capsule" (so the short timer runs ahead of the guest deploy);
//   - the legacy rule reaches only the idle sweep, and pressure reclaim or
//     make-room hibernates an old guest on the short timer;
//   - the legacy rule reaches deploy refresh (an old guest must still be
//     refreshed onto the guest that reports the field);
//   - unset legacy timeout silently changes today's behavior;
//   - health's idle-eligible view disagrees with the sweep.

func healthServer(t *testing.T, status int, body string) *VMOwnership {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return &VMOwnership{VMID: "vm-http", ComputerURL: srv.URL}
}

func TestHTTPGuestHealthReadsLiveCapsules(t *testing.T) {
	for name, tc := range map[string]struct {
		status   int
		body     string
		busy     bool
		reports  bool
		wantBusy bool
	}{
		"a live capsule is busy":                       {200, `{"running_runs":0,"live_capsules":1}`, true, true, true},
		"no live capsule is idle and reports":          {200, `{"running_runs":0,"live_capsules":0}`, false, true, false},
		"a guest without the field is idle, not known": {200, `{"running_runs":0}`, false, false, false},
		"running runs without the field are busy":      {200, `{"running_runs":1}`, true, false, true},
		"a pending desk mutation is busy":              {200, `{"desk_pending_mutations":1,"live_capsules":0}`, true, true, true},
		"an unanswering health is busy":                {503, `{"status":"store_stalled"}`, true, false, true},
		"unreadable is busy":                           {200, `not json`, true, false, true},
	} {
		own := healthServer(t, tc.status, tc.body)
		got := httpGuestHealth()(own)
		if got.Busy != tc.busy || got.ReportsCapsules != tc.reports {
			t.Fatalf("%s: reading = %+v, want busy=%v reports=%v", name, got, tc.busy, tc.reports)
		}
		if httpGuestBusy()(own) != tc.wantBusy {
			t.Fatalf("%s: httpGuestBusy = %v, want %v", name, !tc.wantBusy, tc.wantBusy)
		}
	}
}

func idleRegistry(t *testing.T, short, legacy time.Duration, readings map[string]guestHealthReading) *OwnershipRegistry {
	t.Helper()
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetIdleTimeout(short)
	reg.SetLegacyGuestIdleTimeout(legacy)
	reg.SetWarmnessPolicyConfig(WarmnessPolicyConfig{PrimaryKeepaliveMode: PrimaryKeepaliveModeOff})
	reg.setGuestHealthProbeForTest(func(own *VMOwnership) guestHealthReading { return readings[own.VMID] })
	return reg
}

func addIdle(reg *OwnershipRegistry, vmID string, idle time.Duration) {
	addOwnership(reg, &VMOwnership{VMID: vmID, UserID: "u-" + vmID, DesktopID: PrimaryDesktopID, Kind: VMKindInteractive,
		ComputerID: "computer-" + vmID, State: VMStateActive, LastActiveAt: time.Now().Add(-idle),
		ComputerURL: "http://10.0.0.9:8085"})
}

func idleSet(reg *OwnershipRegistry) map[string]bool {
	out := map[string]bool{}
	for _, own := range reg.CheckIdleOwnerships() {
		out[own.VMID] = true
	}
	return out
}

func TestIdleSweepShortTimerOnlyForGuestsThatReportCapsules(t *testing.T) {
	reg := idleRegistry(t, 10*time.Minute, 30*time.Minute, map[string]guestHealthReading{
		"vm-new":     {Busy: false, ReportsCapsules: true},
		"vm-capsule": {Busy: true, ReportsCapsules: true},
		"vm-old":     {Busy: false, ReportsCapsules: false},
		"vm-oldbusy": {Busy: true, ReportsCapsules: false},
	})
	for _, id := range []string{"vm-new", "vm-capsule", "vm-old", "vm-oldbusy"} {
		addIdle(reg, id, 15*time.Minute)
	}
	got := idleSet(reg)
	if len(got) != 1 || !got["vm-new"] {
		t.Fatalf("idle at 15m = %v, want only the guest that reports no live capsule", got)
	}

	// Past the legacy rule an old guest is not held forever.
	reg2 := idleRegistry(t, 10*time.Minute, 30*time.Minute, map[string]guestHealthReading{
		"vm-old":     {Busy: false, ReportsCapsules: false},
		"vm-oldbusy": {Busy: true, ReportsCapsules: false},
		"vm-capsule": {Busy: true, ReportsCapsules: true},
	})
	for _, id := range []string{"vm-old", "vm-oldbusy", "vm-capsule"} {
		addIdle(reg2, id, 35*time.Minute)
	}
	got = idleSet(reg2)
	if len(got) != 1 || !got["vm-old"] {
		t.Fatalf("idle at 35m = %v, want the old guest (30 minute rule) but never a busy or capsule-holding one", got)
	}

	// Under the short timer nothing is idle.
	reg3 := idleRegistry(t, 10*time.Minute, 30*time.Minute, map[string]guestHealthReading{"vm-new": {ReportsCapsules: true}})
	addIdle(reg3, "vm-new", 9*time.Minute)
	if got := idleSet(reg3); len(got) != 0 {
		t.Fatalf("idle at 9m = %v, want none", got)
	}
}

// An unset legacy timeout is today's behavior: one timer for every guest.
func TestIdleSweepWithoutLegacyTimeoutIsOneTimer(t *testing.T) {
	reg := idleRegistry(t, 10*time.Minute, 0, map[string]guestHealthReading{"vm-old": {ReportsCapsules: false}})
	addIdle(reg, "vm-old", 15*time.Minute)
	if got := idleSet(reg); !got["vm-old"] {
		t.Fatalf("idle = %v, want the old guest on the one timer when no legacy rule is set", got)
	}
}

// Health's idle-eligible view (from the last probes) agrees with the sweep.
func TestIdleSeenViewAgreesWithTheSweep(t *testing.T) {
	reg := idleRegistry(t, 10*time.Minute, 30*time.Minute, map[string]guestHealthReading{
		"vm-new": {ReportsCapsules: true},
		"vm-old": {ReportsCapsules: false},
	})
	addIdle(reg, "vm-new", 15*time.Minute)
	addIdle(reg, "vm-old", 15*time.Minute)
	_ = reg.CheckIdleOwnerships() // probes, recording the answers
	seen := reg.IdleOwnershipsSeen()
	if len(seen) != 1 || seen[0].VMID != "vm-new" {
		t.Fatalf("seen idle = %v, want only vm-new", seen)
	}
}

// Deploy refresh keeps the raw busy probe: an old guest idle under the
// legacy window is still refreshed (a refresh that deferred it would keep
// it from ever reporting the field).
func TestLegacyRuleDoesNotReachDeployRefreshBusy(t *testing.T) {
	reg := idleRegistry(t, 10*time.Minute, 30*time.Minute, map[string]guestHealthReading{"vm-old": {ReportsCapsules: false}})
	addIdle(reg, "vm-old", 5*time.Minute)
	if reg.guestBusy()(reg.GetOwnershipByVMID("vm-old")) {
		t.Fatalf("guestBusy (deploy refresh) holds an old guest inside the legacy window")
	}
}

// Make-room and pressure reclaim hibernate too; they must not take an old
// guest on the short pressure-reclaim idle.
func TestMakeRoomLegacyGuestRule(t *testing.T) {
	f := newMakeRoomFixture(t, makeRoomAdmission())
	f.reg.SetPressureReclaimConfig(PressureReclaimConfig{
		Mode: PressureReclaimModeActive, Signal: PressureReclaimSignalSustained,
		MinMemoryAvailableBytes: 4 * gib, MinIdle: 10 * time.Minute,
	})
	f.reg.SetLegacyGuestIdleTimeout(30 * time.Minute)
	f.reg.setGuestHealthProbeForTest(func(own *VMOwnership) guestHealthReading {
		return guestHealthReading{ReportsCapsules: own.VMID == "vm-new"}
	})
	f.activeComputer("old", "vm-old", 15*time.Minute, 3*gib)
	f.setAvail(5 * gib)
	_, err := f.reg.admitHostMemory(hibernated("starter", "vm-s", 2*gib), WarmnessClassPrimary)
	refusal := RecoveryRefusalFrom(err)
	if refusal == nil || len(f.mr.hibernates()) != 0 {
		t.Fatalf("err=%v hibernates=%v, want a refusal with the old guest left running", err, f.mr.hibernates())
	}
	if !strings.Contains(refusal.Reason, "capsule") {
		t.Fatalf("refusal %q does not say the old guest could not be told from one holding a capsule", refusal.Reason)
	}

	g := newMakeRoomFixture(t, makeRoomAdmission())
	g.reg.SetPressureReclaimConfig(PressureReclaimConfig{
		Mode: PressureReclaimModeActive, Signal: PressureReclaimSignalSustained,
		MinMemoryAvailableBytes: 4 * gib, MinIdle: 10 * time.Minute,
	})
	g.reg.SetLegacyGuestIdleTimeout(30 * time.Minute)
	g.reg.setGuestHealthProbeForTest(func(own *VMOwnership) guestHealthReading {
		return guestHealthReading{ReportsCapsules: true}
	})
	g.activeComputer("new", "vm-new", 15*time.Minute, 3*gib)
	g.setAvail(5 * gib)
	if _, err := g.reg.admitHostMemory(hibernated("starter", "vm-s", 2*gib), WarmnessClassPrimary); err != nil {
		t.Fatalf("a guest that reports no live capsule was not eligible at 15m: %v", err)
	}
}

// A capsule-holding guest is never a make-room victim.
func TestMakeRoomNeverHibernatesALiveCapsuleGuest(t *testing.T) {
	f := newMakeRoomFixture(t, makeRoomAdmission())
	f.reg.setGuestHealthProbeForTest(func(*VMOwnership) guestHealthReading {
		return guestHealthReading{Busy: true, ReportsCapsules: true}
	})
	f.activeComputer("holder", "vm-cap", 3*time.Hour, 3*gib)
	f.setAvail(5 * gib)
	if _, err := f.reg.admitHostMemory(hibernated("starter", "vm-s", 2*gib), WarmnessClassPrimary); RecoveryRefusalFrom(err) == nil || len(f.mr.hibernates()) != 0 {
		t.Fatalf("err=%v hibernates=%v, want a refusal", err, f.mr.hibernates())
	}
}

// A computer with no address cannot be asked, so it cannot be shown idle:
// the probe fails closed (it returned idle, which let the hibernation
// deciders take a computer they never probed).
func TestHTTPGuestHealthEmptyComputerURLIsBusy(t *testing.T) {
	for _, url := range []string{"", "   "} {
		own := &VMOwnership{VMID: "vm-noaddr", ComputerURL: url}
		if got := httpGuestHealth()(own); !got.Busy {
			t.Fatalf("ComputerURL %q read as idle: %+v", url, got)
		}
		if !httpGuestBusy()(own) {
			t.Fatalf("httpGuestBusy for ComputerURL %q read as idle", url)
		}
	}
	reg := idleRegistry(t, 10*time.Minute, 0, nil)
	reg.guestHealthProbe = nil // the real probe
	addOwnership(reg, &VMOwnership{VMID: "vm-noaddr", UserID: "u", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive,
		State: VMStateActive, LastActiveAt: time.Now().Add(-3 * time.Hour)})
	if got := reg.CheckIdleOwnerships(); len(got) != 0 {
		t.Fatalf("the idle sweep took a computer with no address: %v", got)
	}
}
