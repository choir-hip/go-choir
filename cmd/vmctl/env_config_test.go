package main

import (
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// The density quick tier's rollback is by env
// (docs/problems/pressure-reclaim-wedged-a-user-computer-2026-10-14.md).
// Ways it could fail: the rollback values are not read, so today's
// behaviour is unreachable without a code revert; the defaults are not the
// new behaviour; a malformed value zeroes a threshold.
func TestPressureAndAdmissionEnvDefaultsAndRollback(t *testing.T) {
	t.Setenv("VMCTL_PRESSURE_RECLAIM_MODE", "active")
	cfg, ok := pressureReclaimConfigFromEnv()
	if !ok || cfg.Signal != vmctl.PressureReclaimSignalSustained || cfg.SustainedSamples != 4 ||
		cfg.MaxMemorySomeAvg60 != 5.0 || cfg.SampleInterval != 15*time.Second || cfg.Cooldown != 3*time.Minute {
		t.Fatalf("defaults = %+v", cfg)
	}
	if a := hostAdmissionConfigFromEnv(); a.Mode != vmctl.HostAdmissionModeMeasured || a.DefaultNeedBytes != 2<<30 || a.HeadroomBytes != 1<<30 {
		t.Fatalf("admission defaults = %+v", a)
	}

	t.Setenv("VMCTL_PRESSURE_RECLAIM_SIGNAL", "instant")
	t.Setenv("VMCTL_PRESSURE_MAX_MEMORY_SOME_AVG60", "not-a-number")
	t.Setenv("VMCTL_PRESSURE_SUSTAINED_SAMPLES", "6")
	t.Setenv("VMCTL_PRESSURE_RECLAIM_COOLDOWN", "5m")
	cfg, _ = pressureReclaimConfigFromEnv()
	if cfg.Signal != vmctl.PressureReclaimSignalInstant || cfg.SustainedSamples != 6 || cfg.Cooldown != 5*time.Minute || cfg.MaxMemorySomeAvg60 != 5.0 {
		t.Fatalf("overrides = %+v", cfg)
	}

	t.Setenv("VMCTL_ADMISSION_MODE", "guest-size")
	t.Setenv("VMCTL_ADMISSION_HEADROOM_MIB", "2048")
	if a := hostAdmissionConfigFromEnv(); a.Mode != vmctl.HostAdmissionModeGuestSize || a.HeadroomBytes != 2<<30 {
		t.Fatalf("admission rollback = %+v", a)
	}
}

// Density D1: make-room fails closed (review 302 F3). It is off unless
// VMCTL_ADMISSION_MAKE_ROOM names an enabling word exactly; nix/node-b.nix
// sets "active" explicitly, and "off" is the rollback
// (docs/problems/density-station-d-failure-modes-2026-10-26.md, M13).
// Ways it could fail: a typo or an observation word turns it on; the off
// value is not read; a malformed or enormous victim bound or settle wait
// reaches the registry (F5 caps them at 10 and 60s).
func TestAdmissionMakeRoomEnv(t *testing.T) {
	if a := hostAdmissionConfigFromEnv(); a.MakeRoom != vmctl.HostAdmissionMakeRoomOff {
		t.Fatalf("make-room default = %q, want off", a.MakeRoom)
	}
	for value, want := range map[string]string{
		"active": vmctl.HostAdmissionMakeRoomActive, "on": vmctl.HostAdmissionMakeRoomActive,
		"enabled": vmctl.HostAdmissionMakeRoomActive, "true": vmctl.HostAdmissionMakeRoomActive,
		"1": vmctl.HostAdmissionMakeRoomActive, "off": vmctl.HostAdmissionMakeRoomOff,
		"observe": vmctl.HostAdmissionMakeRoomOff, "of": vmctl.HostAdmissionMakeRoomOff,
		"activ": vmctl.HostAdmissionMakeRoomOff, "yes please": vmctl.HostAdmissionMakeRoomOff,
	} {
		t.Setenv("VMCTL_ADMISSION_MAKE_ROOM", value)
		if got := hostAdmissionConfigFromEnv().MakeRoom; got != want {
			t.Fatalf("VMCTL_ADMISSION_MAKE_ROOM=%q gives %q, want %q", value, got, want)
		}
	}
	t.Setenv("VMCTL_ADMISSION_MAKE_ROOM", "off")
	t.Setenv("VMCTL_ADMISSION_MAKE_ROOM_MAX_VICTIMS", "2")
	t.Setenv("VMCTL_ADMISSION_MAKE_ROOM_SETTLE", "20s")
	a := hostAdmissionConfigFromEnv()
	if a.MakeRoom != vmctl.HostAdmissionMakeRoomOff || a.MakeRoomMaxVictims != 2 || a.MakeRoomSettleWait != 20*time.Second {
		t.Fatalf("make-room overrides = %+v", a)
	}
	t.Setenv("VMCTL_ADMISSION_MAKE_ROOM_MAX_VICTIMS", "zero")
	t.Setenv("VMCTL_ADMISSION_MAKE_ROOM_SETTLE", "-1s")
	if a := hostAdmissionConfigFromEnv(); a.MakeRoomMaxVictims <= 0 || a.MakeRoomSettleWait <= 0 {
		t.Fatalf("malformed make-room bounds = %+v, want the defaults", a)
	}
	t.Setenv("VMCTL_ADMISSION_MAKE_ROOM_MAX_VICTIMS", "1000")
	t.Setenv("VMCTL_ADMISSION_MAKE_ROOM_SETTLE", "1h")
	if a := hostAdmissionConfigFromEnv(); a.MakeRoomMaxVictims != 10 || a.MakeRoomSettleWait != 60*time.Second {
		t.Fatalf("make-room bounds = %d victims %s settle, want the caps 10 and 60s", a.MakeRoomMaxVictims, a.MakeRoomSettleWait)
	}
}

// Density D4: the wake-cost estimate is behind VMCTL_ADMISSION_NEED, and
// the code default is today's peak estimate until the staging proof
// (docs/problems/density-station-d-failure-modes-2026-10-26.md, M8).
// Ways it could fail: unset or unknown does not mean peak; the wake value is
// not read.
func TestAdmissionNeedEnv(t *testing.T) {
	if a := hostAdmissionConfigFromEnv(); a.NeedMode != vmctl.HostAdmissionNeedPeak {
		t.Fatalf("need default = %q, want peak", a.NeedMode)
	}
	t.Setenv("VMCTL_ADMISSION_NEED", "wake")
	if a := hostAdmissionConfigFromEnv(); a.NeedMode != vmctl.HostAdmissionNeedWake {
		t.Fatalf("need wake = %q", a.NeedMode)
	}
	t.Setenv("VMCTL_ADMISSION_NEED", "peak")
	if a := hostAdmissionConfigFromEnv(); a.NeedMode != vmctl.HostAdmissionNeedPeak {
		t.Fatalf("need peak = %q", a.NeedMode)
	}
	t.Setenv("VMCTL_ADMISSION_NEED", "settled-ish")
	if a := hostAdmissionConfigFromEnv(); a.NeedMode != vmctl.HostAdmissionNeedPeak {
		t.Fatalf("unknown need = %q, want peak", a.NeedMode)
	}
}

// Density D3a: the shorter idle timer applies only to guests that report
// live_capsules; the rest keep the legacy rule, 30 minutes unless
// VMCTL_IDLE_TIMEOUT_LEGACY_GUEST says otherwise. Ways it could fail: the
// default is not 30m (so lowering VMCTL_IDLE_TIMEOUT silently lowers it for
// every guest); a malformed value zeroes it.
func TestLegacyGuestIdleTimeoutEnv(t *testing.T) {
	if d := legacyGuestIdleTimeoutFromEnv(); d != 30*time.Minute {
		t.Fatalf("legacy guest idle default = %s, want 30m", d)
	}
	t.Setenv("VMCTL_IDLE_TIMEOUT_LEGACY_GUEST", "45m")
	if d := legacyGuestIdleTimeoutFromEnv(); d != 45*time.Minute {
		t.Fatalf("legacy guest idle = %s, want 45m", d)
	}
	t.Setenv("VMCTL_IDLE_TIMEOUT_LEGACY_GUEST", "soon")
	if d := legacyGuestIdleTimeoutFromEnv(); d != 30*time.Minute {
		t.Fatalf("malformed legacy guest idle = %s, want the 30m default", d)
	}
}

// Density D3 (D3-M8): both keepalive modes parse and the code default stays
// under-capacity, so only the nix value moves primary computers to idle
// hibernation; a silent change of the default would move every environment.
func TestPrimaryKeepaliveModeEnv(t *testing.T) {
	mode := func() string {
		cfg, err := warmnessPolicyConfigFromEnv()
		if err != nil {
			t.Fatal(err)
		}
		return cfg.PrimaryKeepaliveMode
	}
	if got := mode(); got != vmctl.PrimaryKeepaliveModeUnderCapacity {
		t.Fatalf("keepalive default = %q, want under-capacity", got)
	}
	t.Setenv("VMCTL_PRIMARY_KEEPALIVE_MODE", "off")
	if got := mode(); got != vmctl.PrimaryKeepaliveModeOff {
		t.Fatalf("keepalive off = %q", got)
	}
	t.Setenv("VMCTL_PRIMARY_KEEPALIVE_MODE", "under-capacity")
	if got := mode(); got != vmctl.PrimaryKeepaliveModeUnderCapacity {
		t.Fatalf("keepalive under-capacity = %q", got)
	}
}

// Review 312 nit 5: the legacy window is never shorter than the idle timeout
// it protects a guest from, so a typo cannot switch the rule off.
func TestLegacyGuestIdleTimeoutIsFlooredAtTheIdleTimeout(t *testing.T) {
	t.Setenv("VMCTL_IDLE_TIMEOUT", "10m")
	t.Setenv("VMCTL_IDLE_TIMEOUT_LEGACY_GUEST", "30s")
	if d := legacyGuestIdleTimeoutFromEnv(); d != 10*time.Minute {
		t.Fatalf("legacy guest idle = %s, want the 10m idle timeout as its floor", d)
	}
	t.Setenv("VMCTL_IDLE_TIMEOUT_LEGACY_GUEST", "20m")
	if d := legacyGuestIdleTimeoutFromEnv(); d != 20*time.Minute {
		t.Fatalf("legacy guest idle = %s, want 20m", d)
	}
}

// D6 S2: the guest cache return switch is off unless VMCTL_GUEST_CACHE_RETURN
// names an enabling word exactly; nix/node-b.nix sets it, and "off" is the
// rollback (docs/problems/density-station-d-failure-modes-2026-10-26.md D6,
// M15, M16).
// Ways it could fail: a typo, a capital or a near word turns it on; the code
// default is anything but off or selects nobody; the selector, triggers,
// scope or the three durations are not read or are not clamped.
func TestGuestCacheReturnEnv(t *testing.T) {
	if cfg := guestCacheReturnConfigFromEnv(); cfg.Mode != vmctl.GuestCacheReturnOff || len(cfg.Only) != 0 || !cfg.TriggerOn("idle") {
		t.Fatalf("defaults = %+v, want off, nobody selected, idle trigger", cfg)
	}
	for value, want := range map[string]string{
		"active": vmctl.GuestCacheReturnActive, "observe": vmctl.GuestCacheReturnObserve, "off": vmctl.GuestCacheReturnOff,
		"Active": vmctl.GuestCacheReturnOff, "on": vmctl.GuestCacheReturnOff, "true": vmctl.GuestCacheReturnOff,
		"actve": vmctl.GuestCacheReturnOff, "1": vmctl.GuestCacheReturnOff,
	} {
		t.Setenv("VMCTL_GUEST_CACHE_RETURN", value)
		if got := guestCacheReturnConfigFromEnv().Mode; got != want {
			t.Fatalf("VMCTL_GUEST_CACHE_RETURN=%q gives %q, want %q", value, got, want)
		}
	}
	t.Setenv("VMCTL_GUEST_CACHE_RETURN", "observe")
	t.Setenv("VMCTL_GUEST_CACHE_RETURN_ONLY", "computer-de, class:test")
	t.Setenv("VMCTL_GUEST_CACHE_RETURN_TRIGGERS", "idle,hibernate")
	t.Setenv("VMCTL_GUEST_CACHE_RETURN_SCOPE", "root")
	t.Setenv("VMCTL_GUEST_CACHE_RETURN_IDLE", "5m")
	t.Setenv("VMCTL_GUEST_CACHE_RETURN_PRESSURE_IDLE", "30s")
	t.Setenv("VMCTL_GUEST_CACHE_RETURN_PRESSURE_BUDGET", "20s")
	cfg := guestCacheReturnConfigFromEnv()
	if cfg.Mode != vmctl.GuestCacheReturnObserve || len(cfg.Only) != 2 || cfg.Only[0] != "computer-de" || cfg.Only[1] != "class:test" ||
		!cfg.TriggerOn("idle") || !cfg.TriggerOn("hibernate") || cfg.TriggerOn("pressure") || cfg.Scope != "root" ||
		cfg.Idle != 5*time.Minute || cfg.PressureIdle != 30*time.Second || cfg.PressureBudget != 20*time.Second {
		t.Fatalf("overrides = %+v", cfg)
	}
	t.Setenv("VMCTL_GUEST_CACHE_RETURN_IDLE", "1h")
	t.Setenv("VMCTL_GUEST_CACHE_RETURN_PRESSURE_BUDGET", "1ms")
	cfg = guestCacheReturnConfigFromEnv()
	if cfg.Idle != 9*time.Minute || cfg.PressureBudget != 2*time.Second {
		t.Fatalf("clamps = idle %s budget %s, want 9m and 2s", cfg.Idle, cfg.PressureBudget)
	}
}
