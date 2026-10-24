package agentcore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
)

// Per-computer settings (L2a A6 / R2-7,
// docs/reports/l2a-synthesis-intake-design-2026-10-14.md). A setting is an
// owner act through the product API at /api/computer-settings, which is not
// on management's ProductAPI allowlist.

// intakeSurfacesEnabled reports whether management's intake surface
// (choir.Stories, Feed, Decline, Delegate) is on: with intake.labels, or
// with intake.event_wakes, whose event records need it (event layer §15).
func (rt *Runtime) intakeSurfacesEnabled(ctx context.Context, ownerID, computerID string) bool {
	return rt.intakeLabelsEnabled(ctx, ownerID, computerID) || rt.computerSettingOn(ctx, ownerID, computerID, store.SettingIntakeEventWakes)
}

// intakeLabelsEnabled reports whether intake.labels is on for the computer.
// Anything but a stored true (no setting, a store error) is off.
func (rt *Runtime) intakeLabelsEnabled(ctx context.Context, ownerID, computerID string) bool {
	return rt.computerSettingOn(ctx, ownerID, computerID, store.SettingIntakeLabels)
}

// computerSettingOn reports whether a boolean setting is on for the
// computer. Anything but a stored true (no setting, a store error) is off.
func (rt *Runtime) computerSettingOn(ctx context.Context, ownerID, computerID, key string) bool {
	if rt == nil || rt.store == nil || strings.TrimSpace(ownerID) == "" || strings.TrimSpace(computerID) == "" {
		return false
	}
	setting, err := rt.store.GetComputerSetting(ctx, ownerID, computerID, key)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			log.Printf("computer settings: read %s on %s: %v; treating it as off", key, computerID, err)
		}
		return false
	}
	return setting.Value
}

type computerSettingRequest struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

// HandleComputerSettings serves /api/computer-settings: GET lists this
// computer's settings (stored or default), POST {"key","value"} sets one.
func (h *APIHandler) HandleComputerSettings(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	rt := h.rt
	if rt == nil || rt.store == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "store unavailable"})
		return
	}
	computerID := rt.TextureComputerID()
	switch r.Method {
	case http.MethodGet:
		settings, err := rt.store.ListComputerSettings(r.Context(), ownerID, computerID)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
			return
		}
		writeAPIJSON(w, http.StatusOK, map[string]any{"settings": settings})
	case http.MethodPost:
		var req computerSettingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid JSON body"})
			return
		}
		key := strings.TrimSpace(req.Key)
		if _, known := store.KnownComputerSettings[key]; !known {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "unknown setting " + key})
			return
		}
		var value bool
		if raw := bytes.TrimSpace(req.Value); len(raw) == 0 || json.Unmarshal(raw, &value) != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "value must be true or false"})
			return
		}
		// store.event_table turns on only once the stream_seq lease is ready
		// on the tape (the lease design, section 4 "When"): refused here,
		// before anything is stored, so the owner never sees a setting that
		// says on beside a writer that cannot start
		// (docs/problems/stream-seq-high-water-mark-design-2026-10-23.md).
		if key == store.SettingEventTable && value {
			if err := rt.store.EventTableLeaseReady(); err != nil {
				writeAPIJSON(w, http.StatusConflict, apiError{Error: eventTableLeaseRefusal(err), Reason: "stream_seq_lease_not_ready"})
				return
			}
		}
		// Setting a value it already has writes nothing, so updated_at is
		// when the value last changed: intake.event_wakes reads it as when
		// production began waiting on events (event layer §15).
		if current, err := rt.store.GetComputerSetting(r.Context(), ownerID, computerID, key); err == nil && current.Value == value {
			// The apply is idempotent and re-run for the two settings whose
			// apply can fail after the value is stored (the event-table floor
			// write, a telemetry sealer that did not start), so a same-value
			// POST is the retry (bunch 148, Devin 4).
			switch key {
			case store.SettingTelemetryBatch:
				rt.applyTelemetryBatchingSetting(value)
			case store.SettingEventTable:
				rt.applyEventTableSetting(value)
			case store.SettingSourceProjection:
				rt.applySourceProjectionSetting(value)
			case store.SettingRunMemoryElision, store.SettingSearchCaps:
				rt.applyResearchSwitchSetting(key, value)
			}
			writeAPIJSON(w, http.StatusOK, map[string]any{"setting": current})
			return
		}
		setting := store.ComputerSetting{OwnerID: ownerID, ComputerID: computerID, Key: key, Value: value, UpdatedAt: time.Now().UTC()}
		if err := rt.store.PutComputerSetting(r.Context(), setting); err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
			return
		}
		switch key {
		case store.SettingTelemetryBatch:
			rt.applyTelemetryBatchingSetting(value)
		case store.SettingEventTable:
			rt.applyEventTableSetting(value)
		}
		if key == store.SettingDebouncedCheckpoint {
			rt.applyGuestCheckpointSetting(value)
		}
		if key == store.SettingSourceProjection {
			rt.applySourceProjectionSetting(value)
		}
		rt.applyResearchSwitchSetting(key, value)
		writeAPIJSON(w, http.StatusOK, map[string]any{"setting": setting})
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}
