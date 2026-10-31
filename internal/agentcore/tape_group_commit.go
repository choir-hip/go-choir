package agentcore

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/store"
)

// tape.group_commit at the runtime seam
// (docs/design/tape-group-commit-2026-10-31.md). The owner's stored setting,
// read at boot and applied at once on a change; off is the live rollback.

// configureTapeGroupCommit applies the setting at boot, before the first
// event of the process.
func (rt *Runtime) configureTapeGroupCommit(ctx context.Context) {
	if rt == nil || rt.store == nil {
		return
	}
	ownerID := strings.TrimSpace(os.Getenv("CHOIR_OWNER_ID"))
	rt.applyTapeGroupCommitSetting(rt.computerSettingOn(ctx, ownerID, rt.cfg.ComputerID, store.SettingTapeGroupCommit))
}

// bootResumesWork reports whether this boot parks pre-boot runs for their one
// resume: a planned restart, except one whose telemetry drain failed while the
// run-memory window was on (tape.group_commit). Then a run's memory may have
// lost its tail, so no run resumes on it: the runs are closed as after a crash.
func (rt *Runtime) bootResumesWork() bool {
	marker, planned := rt.BootWasPlannedRestart()
	if !planned {
		return false
	}
	if marker.TelemetryDrainFailed && markerWindowOpen(marker) {
		log.Printf("runtime: planned restart whose telemetry drain failed, with the run-memory window open at drain time: closing pre-boot runs as after a crash")
		return false
	}
	return true
}

// markerWindowOpen reads the window state stamped at drain time. A marker
// without the field predates it; with a failed drain it is treated as open
// (close, never resume on a possibly holed log): the cost of the conservative
// reading is one interrupted resume, the cost of the other is a silent hole.
func markerWindowOpen(marker PlannedRestart) bool {
	return marker.RunMemoryWindowOpen == nil || *marker.RunMemoryWindowOpen
}

// runMemoryWindowStamp is the window state a marker records at drain time. It
// is stamped only when it matters (the window is open, or the drain failed), so
// a marker written with tape.group_commit off and a clean drain is byte for
// byte what it was before the setting existed.
//
// wasOpen is the setting captured before the drain: the drain takes up to
// seconds, and an owner flip to off during it must not stamp "closed" over
// records the open window held. A run damaged in this process counts as an
// open window too (its log has a hole whatever the setting says).
func (rt *Runtime) runMemoryWindowStamp(wasOpen, drainFailed bool) *bool {
	open := wasOpen || (rt != nil && rt.store != nil && (rt.store.TapeGroupCommitOn() || rt.store.AnyRunMemoryDamaged()))
	if !open && !drainFailed {
		return nil
	}
	return &open
}

// crashCloseNamesRunMemory reports whether a crash-close note should name
// possibly missing run memory: the window is open now, or the planned marker
// that led to this close says it was open when its drain failed.
func (rt *Runtime) crashCloseNamesRunMemory() bool {
	if rt.store != nil && rt.store.TapeGroupCommitOn() {
		return true
	}
	marker, planned := rt.BootWasPlannedRestart()
	return planned && marker.TelemetryDrainFailed && markerWindowOpen(marker)
}

// applyTapeGroupCommitSetting is the owner's change at runtime.
func (rt *Runtime) applyTapeGroupCommitSetting(value bool) {
	if rt == nil || rt.store == nil {
		return
	}
	was := rt.store.TapeGroupCommitOn()
	rt.store.SetTapeGroupCommit(value)
	if was != value || value {
		log.Printf("runtime: tape group commit %s", map[bool]string{true: "on (telemetry rides acknowledged writes, window 10s)", false: "off"}[value])
	}
}
