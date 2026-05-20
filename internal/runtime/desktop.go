// Package runtime provides desktop state API handlers for the go-choir
// sandbox runtime. Desktop state is persisted server-side so that desktop
// restore works across fresh browser contexts for the same user
// (VAL-DESKTOP-007).
package runtime

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// desktopStateGetResponse is the JSON response for GET /api/desktop/state.
type desktopStateGetResponse struct {
	OwnerID        string              `json:"owner_id"`
	DesktopID      string              `json:"desktop_id"`
	Windows        []types.WindowState `json:"windows"`
	ActiveWindowID string              `json:"active_window_id,omitempty"`
	UpdatedAt      string              `json:"updated_at"`
}

// desktopStateSaveRequest is the JSON payload for PUT /api/desktop/state.
type desktopStateSaveRequest struct {
	Windows        []types.WindowState `json:"windows"`
	ActiveWindowID string              `json:"active_window_id,omitempty"`
}

// desktopStateSaveResponse is the JSON response for PUT /api/desktop/state.
type desktopStateSaveResponse struct {
	OK        bool   `json:"ok"`
	DesktopID string `json:"desktop_id"`
	UpdatedAt string `json:"updated_at"`
}

func requestDesktopID(r *http.Request) string {
	if r == nil {
		return types.PrimaryDesktopID
	}
	if desktopID := strings.TrimSpace(r.URL.Query().Get("desktop_id")); desktopID != "" {
		return desktopID
	}
	if desktopID := strings.TrimSpace(r.Header.Get("X-Choir-Desktop")); desktopID != "" {
		return desktopID
	}
	return types.PrimaryDesktopID
}

func sanitizeDesktopState(state types.DesktopState) types.DesktopState {
	state.Windows, state.ActiveWindowID = sanitizeWindowStates(state.Windows, state.ActiveWindowID)
	return state
}

func sanitizeWindowStates(windows []types.WindowState, activeWindowID string) ([]types.WindowState, string) {
	activeWindowID = strings.TrimSpace(activeWindowID)
	if len(windows) == 0 {
		return []types.WindowState{}, ""
	}

	seen := make(map[string]struct{}, len(windows))
	out := make([]types.WindowState, 0, len(windows))
	for i, win := range windows {
		baseID := strings.TrimSpace(win.WindowID)
		if baseID == "" {
			baseID = fmt.Sprintf("restored-window-%d", i+1)
		}
		windowID := baseID
		for suffix := 2; ; suffix++ {
			if _, exists := seen[windowID]; !exists {
				break
			}
			windowID = fmt.Sprintf("%s-%d", baseID, suffix)
		}
		seen[windowID] = struct{}{}
		win.WindowID = windowID

		if !win.Mode.Valid() {
			win.Mode = types.WindowNormal
		}
		if win.Geometry.Width <= 0 {
			win.Geometry.Width = 600
		}
		if win.Geometry.Height <= 0 {
			win.Geometry.Height = 400
		}
		if win.RestoredGeometry != nil && (win.RestoredGeometry.Width <= 0 || win.RestoredGeometry.Height <= 0) {
			win.RestoredGeometry = nil
		}
		out = append(out, win)
	}

	if _, ok := seen[activeWindowID]; !ok {
		activeWindowID = ""
	}
	return out, activeWindowID
}

// HandleDesktopStateGet handles GET /api/desktop/state.
// It returns the persisted desktop state for the authenticated user,
// including open windows, active window, geometry, and app context
// (VAL-DESKTOP-007). If no state exists, it returns an empty default state.
func (h *APIHandler) HandleDesktopStateGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}

	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	desktopID := requestDesktopID(r)

	state, err := h.rt.Store().GetDesktopStateForDesktop(r.Context(), ownerID, desktopID)
	if err != nil {
		log.Printf("runtime api: get desktop state: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to get desktop state"})
		return
	}
	state = sanitizeDesktopState(state)

	writeAPIJSON(w, http.StatusOK, desktopStateGetResponse{
		OwnerID:        state.OwnerID,
		DesktopID:      state.DesktopID,
		Windows:        state.Windows,
		ActiveWindowID: state.ActiveWindowID,
		UpdatedAt:      state.UpdatedAt.Format("2006-01-02T15:04:05.000Z"),
	})
}

// HandleDesktopStateSave handles PUT /api/desktop/state.
// It persists the desktop state for the authenticated user, including
// window identities, geometry, mode, active window, and app context
// (VAL-DESKTOP-007). The state is stored server-side and survives
// fresh browser contexts.
func (h *APIHandler) HandleDesktopStateSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}

	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	desktopID := requestDesktopID(r)

	var req desktopStateSaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}

	now := time.Now().UTC()
	windows, activeWindowID := sanitizeWindowStates(req.Windows, req.ActiveWindowID)

	state := types.DesktopState{
		OwnerID:        ownerID,
		DesktopID:      desktopID,
		Windows:        windows,
		ActiveWindowID: activeWindowID,
		UpdatedAt:      now,
	}

	if err := h.rt.Store().SaveDesktopState(r.Context(), state); err != nil {
		log.Printf("runtime api: save desktop state: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to save desktop state"})
		return
	}
	_, _ = h.rt.emitProductEvent(r.Context(), ownerID, desktopID, types.EventDesktopStateUpdated, map[string]any{
		"desktop_id":       desktopID,
		"active_window_id": activeWindowID,
		"window_count":     len(windows),
		"updated_at":       now.Format(time.RFC3339Nano),
		"source_device_id": strings.TrimSpace(r.Header.Get("X-Choir-Device")),
	})

	writeAPIJSON(w, http.StatusOK, desktopStateSaveResponse{
		OK:        true,
		DesktopID: desktopID,
		UpdatedAt: now.Format("2006-01-02T15:04:05.000Z"),
	})
}

// HandleDesktopState routes GET and PUT /api/desktop/state to the
// appropriate handler.
func (h *APIHandler) HandleDesktopState(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.HandleDesktopStateGet(w, r)
	case http.MethodPut:
		h.HandleDesktopStateSave(w, r)
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}
