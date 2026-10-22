package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"strings"
	"sync"
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

// drainTelemetryForRestart is the planned-restart step: it takes one hold on
// synchronous mode, so new telemetry takes the acknowledged path, and the
// queue is sealed under the shutdown-sized deadline. It reports whether the
// drain failed or timed out (or lost records); the caller writes the marker
// either way, because the planned-restart contract is about work, not
// telemetry. restore releases this hold (once) for a restart that did not
// happen; another holder keeps synchronous mode on. With batching off it does
// nothing.
func (rt *Runtime) drainTelemetryForRestart() (failed bool, restore func()) {
	if rt == nil || rt.store == nil || !rt.store.TelemetryBatchingOn() {
		return false, func() {}
	}
	ctx, cancel := context.WithTimeout(context.Background(), rt.drainDeadline())
	defer cancel()
	if err := rt.store.SetTelemetrySynchronous(ctx, true); err != nil {
		log.Printf("runtime: planned restart: telemetry drain failed (marker still written; the resume record will say the trace may have a gap): %v", err)
		failed = true
	}
	var once sync.Once
	return failed, func() {
		once.Do(func() {
			if err := rt.store.SetTelemetrySynchronous(context.Background(), false); err != nil {
				log.Printf("runtime: telemetry synchronous mode off: %v", err)
			}
		})
	}
}

// writeHostPlannedRestartMarker is the host's planned-restart request: the
// vmctl kill that follows skips the shutdown drain, so the queue is sealed
// here and synchronous mode is held until the host clears the marker, the
// computer restarts, or a later request finds no marker on disk. The host has
// one marker, so it has one hold: a repeated request replaces it and keeps an
// earlier drain failure, whose records are lost whatever the second drain
// does. Holding and writing are serialized with the releases below.
func (rt *Runtime) writeHostPlannedRestartMarker(storePath string, marker PlannedRestart) (PlannedRestart, error) {
	rt.hostRestartTelemetryMu.Lock()
	defer rt.hostRestartTelemetryMu.Unlock()
	if rt.store != nil && rt.store.TelemetryBatchingOn() {
		failed, restore := rt.drainTelemetryForRestart()
		previous := rt.hostRestartTelemetryRestore
		failed = failed || (previous != nil && rt.hostRestartDrainFailed)
		rt.hostRestartTelemetryRestore, rt.hostRestartDrainFailed = restore, failed
		if previous != nil {
			previous()
		}
	}
	marker.TelemetryDrainFailed = rt.hostRestartTelemetryRestore != nil && rt.hostRestartDrainFailed
	if err := WritePlannedRestartMarker(storePath, marker); err != nil {
		// The write can fail after the rename: only an empty marker path
		// proves nothing will boot as planned on it.
		rt.releaseHostRestartTelemetryIfNoMarkerLocked(storePath)
		return marker, err
	}
	return marker, nil
}

// clearHostPlannedRestartMarker is the host's marker clear after a refresh
// that did not happen. The hold is released only once the clear succeeded; a
// failed clear may leave the marker durable, so the queue stays closed.
func (rt *Runtime) clearHostPlannedRestartMarker(storePath string) error {
	rt.hostRestartTelemetryMu.Lock()
	defer rt.hostRestartTelemetryMu.Unlock()
	if err := ClearPlannedRestartMarker(storePath); err != nil {
		return err
	}
	rt.releaseHostRestartTelemetryLocked()
	return nil
}

// releaseHostRestartTelemetryIfNoMarker is the busy refusal's undo: a refused
// request restarts nothing, but an earlier request's durable marker may still
// be followed by the host's kill, so the hold is kept while any marker is on
// disk (or its presence cannot be read).
func (rt *Runtime) releaseHostRestartTelemetryIfNoMarker(storePath string) {
	if rt == nil {
		return
	}
	rt.hostRestartTelemetryMu.Lock()
	defer rt.hostRestartTelemetryMu.Unlock()
	rt.releaseHostRestartTelemetryIfNoMarkerLocked(storePath)
}

func (rt *Runtime) releaseHostRestartTelemetryIfNoMarkerLocked(storePath string) {
	if _, err := os.Lstat(PlannedRestartMarkerPath(storePath)); errors.Is(err, os.ErrNotExist) {
		rt.releaseHostRestartTelemetryLocked()
	}
}

func (rt *Runtime) releaseHostRestartTelemetryLocked() {
	if restore := rt.hostRestartTelemetryRestore; restore != nil {
		rt.hostRestartTelemetryRestore, rt.hostRestartDrainFailed = nil, false
		restore()
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
