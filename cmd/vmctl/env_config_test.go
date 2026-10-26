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

// Density D1: make-room is on by default and its rollback is one env value
// (docs/problems/density-station-d-failure-modes-2026-10-26.md, M13).
// Ways it could fail: the off value is not read; a malformed victim bound
// or settle wait zeroes the bound.
func TestAdmissionMakeRoomEnv(t *testing.T) {
	if a := hostAdmissionConfigFromEnv(); a.MakeRoom != vmctl.HostAdmissionMakeRoomActive {
		t.Fatalf("make-room default = %q, want active", a.MakeRoom)
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
}
