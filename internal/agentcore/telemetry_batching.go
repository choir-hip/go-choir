package agentcore

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
)

// Run-telemetry batching at the runtime seam (docs/reports/telemetry-
// batching-design-2026-10-20.md). The sealer itself lives in internal/store;
// this file is the flag, the restart rules and the records that name a gap.

// telemetryMissingWindowNote is the sentence a crash-close or a failed-drain
// resume record carries, so a reader is told the last window of a run's trace
// may be missing instead of inferring it from a sequence gap (design 5.2).
const telemetryMissingWindowNote = "the last window of this run's trace may be missing"

// defaultTelemetryDrainDeadline bounds the planned-restart drain, like the
// shutdown deadline: the restart is never held up by telemetry.
const defaultTelemetryDrainDeadline = 10 * time.Second

func (rt *Runtime) drainDeadline() time.Duration {
	if rt != nil && rt.telemetryDrainDeadline > 0 {
		return rt.telemetryDrainDeadline
	}
	return defaultTelemetryDrainDeadline
}

// configureTelemetryBatching applies the flag at boot, before the first event
// of the process is appended (the sequence reseed gap is decided when the
// counters are seeded). The flag is the env value CHOIR_TELEMETRY_BATCH=on or
// the owner's computer setting telemetry.batch; with neither, no sealer is
// created and the process runs the acknowledged path it always did.
func (rt *Runtime) configureTelemetryBatching(ctx context.Context) {
	if rt == nil || rt.store == nil {
		return
	}
	cfg := store.TelemetryBatchConfigFromEnv()
	if !cfg.Enabled {
		ownerID := strings.TrimSpace(os.Getenv("CHOIR_OWNER_ID"))
		cfg.Enabled = rt.computerSettingOn(ctx, ownerID, rt.cfg.ComputerID, store.SettingTelemetryBatch)
	}
	if !cfg.Enabled {
		return
	}
	if !rt.store.ProjectionTapeBound() {
		log.Printf("runtime: telemetry batching requested but the projection tape is not bound; staying acknowledged")
		return
	}
	if err := rt.store.ConfigureTelemetryBatching(cfg); err != nil {
		log.Printf("runtime: telemetry batching: %v", err)
		return
	}
	log.Printf("runtime: telemetry batching on (window %s)", cfg.Window)
}

// applyTelemetryBatchingSetting is the owner's setting change at runtime. Off
// drains what is queued and returns to acknowledged writes. The env flag, when
// set, keeps it on whatever the setting says.
func (rt *Runtime) applyTelemetryBatchingSetting(value bool) {
	if rt == nil || rt.store == nil {
		return
	}
	cfg := store.TelemetryBatchConfigFromEnv()
	cfg.Enabled = cfg.Enabled || value
	if cfg.Enabled && !rt.store.ProjectionTapeBound() {
		log.Printf("runtime: telemetry.batch set but the projection tape is not bound; staying acknowledged")
		return
	}
	if err := rt.store.ConfigureTelemetryBatching(cfg); err != nil {
		log.Printf("runtime: telemetry batching: %v", err)
	}
}

// drainTelemetryForRestart is the planned-restart step: new telemetry takes
// the acknowledged path, then the queue is sealed under the shutdown-sized
// deadline. It reports whether the drain failed or timed out (or lost
// records); the caller writes the marker either way, because the planned-
// restart contract is about work, not telemetry. restore undoes synchronous
// mode for an apply that returns without restarting.
func (rt *Runtime) drainTelemetryForRestart() (failed bool, restore func()) {
	if rt == nil || rt.store == nil || !rt.store.TelemetryBatchingConfigured() {
		return false, func() {}
	}
	ctx, cancel := context.WithTimeout(context.Background(), rt.drainDeadline())
	defer cancel()
	if err := rt.store.SetTelemetrySynchronous(ctx, true); err != nil {
		log.Printf("runtime: planned restart: telemetry drain failed (marker still written; the resume record will say the trace may have a gap): %v", err)
		failed = true
	}
	return failed, func() {
		if err := rt.store.SetTelemetrySynchronous(context.Background(), false); err != nil {
			log.Printf("runtime: telemetry synchronous mode off: %v", err)
		}
	}
}

// crashClosePayload is the event payload of a run closed by a crash boot. With
// batching configured it also names the possible gap; without it the payload
// is exactly what it was before batching existed.
func crashClosePayload(telemetryBatching bool) json.RawMessage {
	if !telemetryBatching {
		return json.RawMessage(`{"fate":"interrupted_by_restart"}`)
	}
	raw, _ := json.Marshal(map[string]string{
		"fate":      runFateInterruptedByRestart,
		"telemetry": telemetryMissingWindowNote,
	})
	return raw
}

// passivatedOnRestartPayload is the event payload of a run parked for a
// planned restart's one resume. It names a possible gap only when the drain
// before the restart failed.
func passivatedOnRestartPayload(marker PlannedRestart) json.RawMessage {
	if !marker.TelemetryDrainFailed {
		return json.RawMessage(`{"recovery":"passivated_on_restart"}`)
	}
	raw, _ := json.Marshal(map[string]string{
		"recovery":  "passivated_on_restart",
		"telemetry": telemetryMissingWindowNote,
	})
	return raw
}
