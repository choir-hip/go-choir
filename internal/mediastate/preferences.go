package mediastate

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// /api/preferences/{key} (slice S6; system modularity design section 2.6).
// The shell custodian keeps the owner's preferences. Two kinds of key:
//
//	theme                  the owner's theme (HandleThemePreference, unchanged)
//	pkg.<package>.<key>    a setting a package declares in contributes.settings
//
// A package setting is taken only when an installed manifest (or an in-tree
// one) declares the key, the value fits the declared type and enum, and the
// body is small. A value the declaration no longer fits reads as the default.
// Uninstall keeps the values (data, not code); the key is refused until the
// package is installed again.

// maxSettingBody bounds a setting write: one value of at most
// apppackage.MaxSettingStringLen characters, with room for JSON framing.
const maxSettingBody = 8 << 10

// SettingSource names the settings package pkg declares on the owner's
// computer: the installed manifest if the package is installed, otherwise the
// in-tree one. It returns nil for a package that declares none.
type SettingSource func(ctx context.Context, ownerID, pkg string) []apppackage.SettingsSection

// SetSettingSource wires the declarations the preference route checks against.
func (h *Handler) SetSettingSource(src SettingSource) { h.settings = src }

type settingRequest struct {
	Value json.RawMessage `json:"value"`
}

type settingResponse struct {
	Key       string `json:"key"`
	Value     any    `json:"value"`
	Default   bool   `json:"default"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// HandlePreference serves /api/preferences/{key}.
func (h *Handler) HandlePreference(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/api/preferences/")
	if key == "theme" {
		h.HandleThemePreference(w, r)
		return
	}
	if key == pkgrelease.UpdatePolicyKey {
		h.handleUpdatePolicy(w, r)
		return
	}
	if key == apppackage.SelectionKey {
		h.handleSelections(w, r)
		return
	}
	pkg, field, ok := apppackage.SplitSettingKey(key)
	if !ok {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "unknown preference"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPut {
		w.Header().Set("Allow", "GET, PUT")
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	var sections []apppackage.SettingsSection
	if h.settings != nil {
		sections = h.settings(r.Context(), ownerID, pkg)
	}
	decl, declared := apppackage.Field(sections, field)
	if !declared {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "setting not declared by an installed package"})
		return
	}
	if r.Method == http.MethodGet {
		h.getSetting(w, r, ownerID, key, decl)
		return
	}
	h.putSetting(w, r, ownerID, key, decl)
}

// ReadUpdatePolicy is the owner's package update policy (F1): manual for
// every app unless the owner chose otherwise. An absent or unreadable
// preference is manual.
func ReadUpdatePolicy(ctx context.Context, st *store.Store, ownerID string) pkgrelease.UpdatePolicy {
	manual := pkgrelease.UpdatePolicy{Default: pkgrelease.UpdateManual}
	if st == nil {
		return manual
	}
	rec, err := st.GetUserPreference(ctx, ownerID, pkgrelease.UpdatePolicyKey)
	if err != nil {
		return manual
	}
	raw, err := json.Marshal(rec.Value["value"])
	if err != nil {
		return manual
	}
	policy, err := pkgrelease.ParseUpdatePolicy(raw)
	if err != nil {
		return manual
	}
	return policy
}

// handleUpdatePolicy serves /api/preferences/packages.update_policy: the
// owner's update setting, {"default": "manual"|"automatic", "apps": {...}}.
// It is a user preference on the owner's tape, not a platform switch.
func (h *Handler) handleUpdatePolicy(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	key := pkgrelease.UpdatePolicyKey
	switch r.Method {
	case http.MethodGet:
		_, getErr := h.store.GetUserPreference(r.Context(), ownerID, key)
		writeAPIJSON(w, http.StatusOK, settingResponse{Key: key, Value: ReadUpdatePolicy(r.Context(), h.store, ownerID), Default: getErr != nil})
	case http.MethodPut:
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 32<<10))
		if err != nil {
			writeAPIJSON(w, http.StatusRequestEntityTooLarge, apiError{Error: "update policy too large"})
			return
		}
		var req settingRequest
		dec := json.NewDecoder(strings.NewReader(string(body)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil || dec.More() || len(req.Value) == 0 {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "an update policy write is {\"value\": {\"default\": ..., \"apps\": {...}}}"})
			return
		}
		policy, err := pkgrelease.ParseUpdatePolicy(req.Value)
		if err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
			return
		}
		var value map[string]any
		normalized, _ := json.Marshal(policy)
		_ = json.Unmarshal(normalized, &value)
		rec, err := h.store.SaveUserPreference(r.Context(), types.UserPreference{
			OwnerID: ownerID, PreferenceKey: key, Value: map[string]any{"value": value}, UpdatedAt: time.Now().UTC(),
		})
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to save the update policy"})
			return
		}
		updated := rec.UpdatedAt.UTC().Format(time.RFC3339Nano)
		_, _ = h.emitProductEvent(r.Context(), ownerID, requestDesktopID(r), types.EventSettingUpdated, map[string]any{
			"key": key, "value": value, "updated_at": updated,
			"source_device_id": strings.TrimSpace(r.Header.Get("X-Choir-Device")),
		})
		writeAPIJSON(w, http.StatusOK, settingResponse{Key: key, Value: policy, UpdatedAt: updated})
	default:
		w.Header().Set("Allow", "GET, PUT")
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

// handleSelections serves /api/preferences/packages.selection: the owner's
// selection records, {"<slot>": {"package", "release", "at"}} (slice L2). A
// write carries the slots it changes and is merged per slot into the stored
// set (slice L4: a write for one slot never clobbers another's, so two
// windows, or a choice and the platform's yank fallback, cannot lose each
// other's records); the answer and the live event carry the merged set.
// Absent reads as {} (every slot ranked by the kernel's default order). It
// is a fact on the owner's tape, not a switch.
func (h *Handler) handleSelections(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	key := apppackage.SelectionKey
	switch r.Method {
	case http.MethodGet:
		rec, getErr := h.store.GetUserPreference(r.Context(), ownerID, key)
		value := apppackage.Selections{}
		if getErr == nil {
			value = readSelections(rec)
		}
		resp := settingResponse{Key: key, Value: value, Default: getErr != nil}
		if getErr == nil {
			resp.UpdatedAt = rec.UpdatedAt.UTC().Format(time.RFC3339Nano)
		}
		writeAPIJSON(w, http.StatusOK, resp)
	case http.MethodPut:
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 16<<10))
		if err != nil {
			writeAPIJSON(w, http.StatusRequestEntityTooLarge, apiError{Error: "selections too large"})
			return
		}
		var req settingRequest
		dec := json.NewDecoder(strings.NewReader(string(body)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil || dec.More() || len(req.Value) == 0 {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "a selections write is {\"value\": {\"<slot>\": {\"package\": ..., \"at\": ...}}}"})
			return
		}
		changes, err := apppackage.ParseSelections(req.Value)
		if err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
			return
		}
		h.selectionMu.Lock()
		defer h.selectionMu.Unlock()
		sel, err := h.storedSelections(r.Context(), ownerID)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to read the selections"})
			return
		}
		for slot, rec := range changes {
			sel[slot] = rec
		}
		var value map[string]any
		normalized, _ := json.Marshal(sel)
		_ = json.Unmarshal(normalized, &value)
		rec, err := h.store.SaveUserPreference(r.Context(), types.UserPreference{
			OwnerID: ownerID, PreferenceKey: key, Value: map[string]any{"value": value}, UpdatedAt: time.Now().UTC(),
		})
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to save the selections"})
			return
		}
		updated := rec.UpdatedAt.UTC().Format(time.RFC3339Nano)
		_, _ = h.emitProductEvent(r.Context(), ownerID, requestDesktopID(r), types.EventSettingUpdated, map[string]any{
			"key": key, "value": value, "updated_at": updated,
			"source_device_id": strings.TrimSpace(r.Header.Get("X-Choir-Device")),
		})
		writeAPIJSON(w, http.StatusOK, settingResponse{Key: key, Value: sel, UpdatedAt: updated})
	default:
		w.Header().Set("Allow", "GET, PUT")
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

// readSelections reads a stored selections value. A value this build no
// longer reads (a slot it does not have) reads as no selection, never as an
// error.
func readSelections(rec types.UserPreference) apppackage.Selections {
	if raw, err := json.Marshal(rec.Value["value"]); err == nil {
		if parsed, err := apppackage.ParseSelections(raw); err == nil {
			return parsed
		}
	}
	return apppackage.Selections{}
}

// storedSelections is the owner's stored set (empty when absent). A failed
// read is an error, so a merge never writes over records it could not see.
func (h *Handler) storedSelections(ctx context.Context, ownerID string) (apppackage.Selections, error) {
	rec, err := h.store.GetUserPreference(ctx, ownerID, apppackage.SelectionKey)
	if errors.Is(err, store.ErrNotFound) {
		return apppackage.Selections{}, nil
	}
	if err != nil {
		return nil, err
	}
	return readSelections(rec), nil
}

func (h *Handler) getSetting(w http.ResponseWriter, r *http.Request, ownerID, key string, decl apppackage.SettingField) {
	rec, err := h.store.GetUserPreference(r.Context(), ownerID, key)
	switch {
	case err == store.ErrNotFound:
		writeAPIJSON(w, http.StatusOK, settingResponse{Key: key, Value: decl.DefaultValue(), Default: true})
		return
	case err != nil:
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to get setting"})
		return
	}
	// G4: a stored value the declaration no longer fits reads as the default.
	stored, _ := json.Marshal(rec.Value["value"])
	value, err := decl.Check(stored)
	if err != nil {
		writeAPIJSON(w, http.StatusOK, settingResponse{Key: key, Value: decl.DefaultValue(), Default: true})
		return
	}
	writeAPIJSON(w, http.StatusOK, settingResponse{Key: key, Value: value, UpdatedAt: rec.UpdatedAt.UTC().Format(time.RFC3339Nano)})
}

func (h *Handler) putSetting(w http.ResponseWriter, r *http.Request, ownerID, key string, decl apppackage.SettingField) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxSettingBody))
	if err != nil {
		writeAPIJSON(w, http.StatusRequestEntityTooLarge, apiError{Error: "setting body too large"})
		return
	}
	var req settingRequest
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || dec.More() || len(req.Value) == 0 {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "a setting write is {\"value\": ...}"})
		return
	}
	value, err := decl.Check(req.Value)
	if err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid setting value: " + err.Error()})
		return
	}
	rec, err := h.store.SaveUserPreference(r.Context(), types.UserPreference{
		OwnerID:       ownerID,
		PreferenceKey: key,
		Value:         map[string]any{"value": value},
		UpdatedAt:     time.Now().UTC(),
	})
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to save setting"})
		return
	}
	updated := rec.UpdatedAt.UTC().Format(time.RFC3339Nano)
	_, _ = h.emitProductEvent(r.Context(), ownerID, requestDesktopID(r), types.EventSettingUpdated, map[string]any{
		"key":              key,
		"value":            value,
		"updated_at":       updated,
		"source_device_id": strings.TrimSpace(r.Header.Get("X-Choir-Device")),
	})
	writeAPIJSON(w, http.StatusOK, settingResponse{Key: key, Value: value, UpdatedAt: updated})
}
