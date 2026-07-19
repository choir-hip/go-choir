// Package desktopstate owns the authenticated desktop-state HTTP control plane.
package desktopstate

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Handler owns desktop-state HTTP behavior and persistence.
type Handler struct {
	store *store.Store
	bus   *events.EventBus
}

// NewHandler constructs the desktop-state control plane.
func NewHandler(s *store.Store, bus *events.EventBus) *Handler {
	return &Handler{store: s, bus: bus}
}

func (h *Handler) emitProductEvent(ctx context.Context, ownerID, desktopID string, kind types.EventKind, payload map[string]any) (types.EventRecord, error) {
	if h == nil || h.store == nil {
		return types.EventRecord{}, fmt.Errorf("runtime store unavailable")
	}
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return types.EventRecord{}, fmt.Errorf("owner_id is required")
	}
	if payload == nil {
		payload = map[string]any{}
	}
	desktopID = strings.TrimSpace(desktopID)
	if desktopID != "" {
		payload["desktop_id"] = desktopID
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return types.EventRecord{}, fmt.Errorf("marshal product event payload: %w", err)
	}
	rec := types.EventRecord{
		EventID:   uuid.New().String(),
		OwnerID:   ownerID,
		Timestamp: time.Now().UTC(),
		Kind:      kind,
		Phase:     "product",
		Payload:   raw,
	}
	if err := h.store.AppendEvent(ctx, &rec); err != nil {
		return types.EventRecord{}, fmt.Errorf("append product event: %w", err)
	}
	if h.bus != nil {
		h.bus.Publish(events.RuntimeEvent{
			Record: rec,
			Actor:  events.ActorRuntime,
			Cause:  events.CauseHostAction,
		})
	}
	return rec, nil
}

// desktopStateGetResponse is the JSON response for GET /api/desktop/state.
type desktopStateGetResponse struct {
	OwnerID        string              `json:"owner_id"`
	DesktopID      string              `json:"desktop_id"`
	SessionID      string              `json:"session_id,omitempty"`
	Windows        []types.WindowState `json:"windows"`
	ActiveWindowID string              `json:"active_window_id,omitempty"`
	UpdatedAt      string              `json:"updated_at"`
}

// desktopStateSaveRequest is the JSON payload for PUT /api/desktop/state.
type desktopStateSaveRequest struct {
	Windows        []types.WindowState `json:"windows"`
	ActiveWindowID string              `json:"active_window_id,omitempty"`
	Driver         bool                `json:"driver,omitempty"`
}

// desktopStateSaveResponse is the JSON response for PUT /api/desktop/state.
type desktopStateSaveResponse struct {
	OK        bool   `json:"ok"`
	DesktopID string `json:"desktop_id"`
	UpdatedAt string `json:"updated_at"`
}

type apiError struct {
	Error string `json:"error"`
}

func authenticateUser(r *http.Request) (string, error) {
	user := r.Header.Get("X-Authenticated-User")
	if user == "" {
		return "", fmt.Errorf("missing authenticated user identity")
	}
	return user, nil
}

func writeAPIJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("runtime api: json encode error: %v", err)
	}
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

func requestDesktopSessionContext(r *http.Request, driver bool) types.DesktopSessionContext {
	now := time.Now().UTC()
	sessionID := strings.TrimSpace(r.Header.Get("X-Choir-Session"))
	if sessionID == "" {
		sessionID = strings.TrimSpace(r.URL.Query().Get("session_id"))
	}
	if sessionID == "" {
		sessionID = "legacy"
		driver = true
	}
	return types.DesktopSessionContext{
		SessionID:       sessionID,
		DeviceID:        strings.TrimSpace(r.Header.Get("X-Choir-Device")),
		ViewportProfile: strings.TrimSpace(r.Header.Get("X-Choir-Viewport")),
		IsDriver:        driver,
		DriverUntil:     now.Add(60 * time.Second),
		UpdatedAt:       now,
	}
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

	return out, topVisibleWindowID(out)
}

func topVisibleWindowID(windows []types.WindowState) string {
	activeWindowID := ""
	activeZ := -1
	for _, win := range windows {
		if win.Mode == types.WindowMinimized {
			continue
		}
		if activeWindowID == "" || win.ZIndex >= activeZ {
			activeWindowID = win.WindowID
			activeZ = win.ZIndex
		}
	}
	return activeWindowID
}

// HandleDesktopStateGet handles GET /api/desktop/state.
// It returns the persisted desktop state for the authenticated user,
// including open windows, active window, geometry, and app context
// (VAL-DESKTOP-007). If no state exists, it returns an empty default state.
func (h *Handler) HandleDesktopStateGet(w http.ResponseWriter, r *http.Request) {
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

	session := requestDesktopSessionContext(r, false)
	state, err := h.store.GetDesktopStateForSession(r.Context(), ownerID, desktopID, session.SessionID)
	if err != nil {
		log.Printf("runtime api: get desktop state: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to get desktop state"})
		return
	}
	state = sanitizeDesktopState(state)

	writeAPIJSON(w, http.StatusOK, desktopStateGetResponse{
		OwnerID:        state.OwnerID,
		DesktopID:      state.DesktopID,
		SessionID:      session.SessionID,
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
func (h *Handler) HandleDesktopStateSave(w http.ResponseWriter, r *http.Request) {
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
	session := requestDesktopSessionContext(r, req.Driver)

	state := types.DesktopState{
		OwnerID:        ownerID,
		DesktopID:      desktopID,
		Windows:        windows,
		ActiveWindowID: activeWindowID,
		UpdatedAt:      now,
	}

	if err := h.store.SaveDesktopStateForSession(r.Context(), state, session); err != nil {
		log.Printf("runtime api: save desktop state: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to save desktop state"})
		return
	}
	eventPayload := map[string]any{
		"desktop_id":        desktopID,
		"session_id":        session.SessionID,
		"active_window_id":  activeWindowID,
		"window_count":      len(windows),
		"updated_at":        now.Format(time.RFC3339Nano),
		"source_device_id":  session.DeviceID,
		"source_session_id": session.SessionID,
		"viewport_profile":  session.ViewportProfile,
		"driver":            session.IsDriver,
	}
	if session.IsDriver {
		_, _ = h.emitProductEvent(r.Context(), ownerID, desktopID, types.EventDesktopDriverLeaseUpdated, eventPayload)
		_, _ = h.emitProductEvent(r.Context(), ownerID, desktopID, types.EventDesktopAppInstancesUpdated, eventPayload)
		_, _ = h.emitProductEvent(r.Context(), ownerID, desktopID, types.EventDesktopWindowPlacementUpdated, eventPayload)
	}

	writeAPIJSON(w, http.StatusOK, desktopStateSaveResponse{
		OK:        true,
		DesktopID: desktopID,
		UpdatedAt: now.Format("2006-01-02T15:04:05.000Z"),
	})
}

// HandleDesktopState routes GET and PUT /api/desktop/state to the appropriate handler.
func (h *Handler) HandleDesktopState(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.HandleDesktopStateGet(w, r)
	case http.MethodPut:
		h.HandleDesktopStateSave(w, r)
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}
