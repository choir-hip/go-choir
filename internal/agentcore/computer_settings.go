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

// intakeLabelsEnabled reports whether intake.labels is on for the computer.
// Anything but a stored true (no setting, a store error) is off.
func (rt *Runtime) intakeLabelsEnabled(ctx context.Context, ownerID, computerID string) bool {
	if rt == nil || rt.store == nil || strings.TrimSpace(ownerID) == "" || strings.TrimSpace(computerID) == "" {
		return false
	}
	setting, err := rt.store.GetComputerSetting(ctx, ownerID, computerID, store.SettingIntakeLabels)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			log.Printf("computer settings: read %s on %s: %v; treating it as off", store.SettingIntakeLabels, computerID, err)
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
		setting := store.ComputerSetting{OwnerID: ownerID, ComputerID: computerID, Key: key, Value: value, UpdatedAt: time.Now().UTC()}
		if err := rt.store.PutComputerSetting(r.Context(), setting); err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
			return
		}
		writeAPIJSON(w, http.StatusOK, map[string]any{"setting": setting})
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}
