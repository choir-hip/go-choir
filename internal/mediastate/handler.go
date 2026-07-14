// Package mediastate owns the authenticated media-state HTTP control plane.
package mediastate

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Handler owns media-state HTTP behavior, persistence, and product events.
type Handler struct {
	store *store.Store
	bus   *events.EventBus
}

// NewHandler constructs the media-state control plane.
func NewHandler(s *store.Store, bus *events.EventBus) *Handler {
	return &Handler{store: s, bus: bus}
}

type mediaProgressRequest struct {
	Kind            string  `json:"kind"`
	Identity        string  `json:"identity"`
	CurrentTime     float64 `json:"current_time"`
	Duration        float64 `json:"duration,omitempty"`
	PlaybackRate    float64 `json:"playback_rate,omitempty"`
	UpdatedByDevice string  `json:"updated_by_device,omitempty"`
}

type mediaRecentRequest struct {
	Kind      string `json:"kind"`
	Identity  string `json:"identity"`
	Title     string `json:"title,omitempty"`
	FileName  string `json:"file_name,omitempty"`
	FilePath  string `json:"file_path,omitempty"`
	SourceURL string `json:"source_url,omitempty"`
	MediaType string `json:"media_type,omitempty"`
	ContentID string `json:"content_id,omitempty"`
}

type mediaRecentListResponse struct {
	Items []types.MediaRecent `json:"items"`
}

type themePreferenceRequest struct {
	Theme map[string]any `json:"theme"`
}

type themePreferenceResponse struct {
	Theme     map[string]any `json:"theme"`
	UpdatedAt string         `json:"updated_at,omitempty"`
}

type apiError struct {
	Error string `json:"error"`
}

// HandleMediaProgress routes authenticated media progress reads and writes.
func (h *Handler) HandleMediaProgress(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.HandleMediaProgressGet(w, r)
	case http.MethodPut:
		h.HandleMediaProgressPut(w, r)
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

// HandleMediaProgressGet returns progress scoped to the authenticated owner.
func (h *Handler) HandleMediaProgressGet(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	kind := strings.TrimSpace(r.URL.Query().Get("kind"))
	identity := strings.TrimSpace(r.URL.Query().Get("identity"))
	if kind == "" || identity == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "kind and identity are required"})
		return
	}
	rec, err := h.store.GetMediaProgress(r.Context(), ownerID, kind, identity)
	if err != nil {
		if err == store.ErrNotFound {
			writeAPIJSON(w, http.StatusOK, types.MediaProgress{
				OwnerID:      ownerID,
				Kind:         kind,
				Identity:     identity,
				PlaybackRate: 1,
			})
			return
		}
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to get media progress"})
		return
	}
	writeAPIJSON(w, http.StatusOK, rec)
}

// HandleMediaProgressPut stores progress scoped to the authenticated owner.
func (h *Handler) HandleMediaProgressPut(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	var req mediaProgressRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid media progress request"})
		return
	}
	rec := types.MediaProgress{
		OwnerID:         ownerID,
		Kind:            strings.TrimSpace(req.Kind),
		Identity:        strings.TrimSpace(req.Identity),
		CurrentTime:     sanitizeFloat(req.CurrentTime),
		Duration:        sanitizeFloat(req.Duration),
		PlaybackRate:    req.PlaybackRate,
		UpdatedByDevice: strings.TrimSpace(req.UpdatedByDevice),
		UpdatedAt:       time.Now().UTC(),
	}
	if rec.Kind == "" || rec.Identity == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "kind and identity are required"})
		return
	}
	saved, err := h.store.UpsertMediaProgress(r.Context(), rec)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to save media progress"})
		return
	}
	_, _ = h.emitProductEvent(r.Context(), ownerID, requestDesktopID(r), types.EventMediaProgressUpdated, map[string]any{
		"kind":              saved.Kind,
		"identity":          saved.Identity,
		"current_time":      saved.CurrentTime,
		"duration":          saved.Duration,
		"playback_rate":     saved.PlaybackRate,
		"updated_by_device": saved.UpdatedByDevice,
		"updated_at":        saved.UpdatedAt.UTC().Format(time.RFC3339Nano),
		"source_device_id":  strings.TrimSpace(r.Header.Get("X-Choir-Device")),
	})
	writeAPIJSON(w, http.StatusOK, saved)
}

// HandleMediaRecents routes authenticated recent-media reads and writes.
func (h *Handler) HandleMediaRecents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.HandleMediaRecentsGet(w, r)
	case http.MethodPut:
		h.HandleMediaRecentsPut(w, r)
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

// HandleMediaRecentsGet lists recent media scoped to the authenticated owner.
func (h *Handler) HandleMediaRecentsGet(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	items, err := h.store.ListMediaRecents(r.Context(), ownerID, strings.TrimSpace(r.URL.Query().Get("kind")), limit)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list media recents"})
		return
	}
	writeAPIJSON(w, http.StatusOK, mediaRecentListResponse{Items: items})
}

// HandleMediaRecentsPut records recently opened media for the authenticated owner.
func (h *Handler) HandleMediaRecentsPut(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	var req mediaRecentRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid media recent request"})
		return
	}
	rec := types.MediaRecent{
		OwnerID:   ownerID,
		Kind:      strings.TrimSpace(req.Kind),
		Identity:  strings.TrimSpace(req.Identity),
		Title:     strings.TrimSpace(req.Title),
		FileName:  strings.TrimSpace(req.FileName),
		FilePath:  strings.TrimSpace(req.FilePath),
		SourceURL: strings.TrimSpace(req.SourceURL),
		MediaType: strings.TrimSpace(req.MediaType),
		ContentID: strings.TrimSpace(req.ContentID),
		OpenedAt:  time.Now().UTC(),
	}
	if rec.Kind == "" || rec.Identity == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "kind and identity are required"})
		return
	}
	saved, err := h.store.UpsertMediaRecent(r.Context(), rec)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to save media recent"})
		return
	}
	_, _ = h.emitProductEvent(r.Context(), ownerID, requestDesktopID(r), types.EventMediaRecentUpdated, map[string]any{
		"kind":             saved.Kind,
		"identity":         saved.Identity,
		"title":            saved.Title,
		"file_name":        saved.FileName,
		"file_path":        saved.FilePath,
		"source_url":       saved.SourceURL,
		"media_type":       saved.MediaType,
		"content_id":       saved.ContentID,
		"opened_at":        saved.OpenedAt.UTC().Format(time.RFC3339Nano),
		"source_device_id": strings.TrimSpace(r.Header.Get("X-Choir-Device")),
	})
	writeAPIJSON(w, http.StatusOK, saved)
}

// HandleThemePreference routes authenticated theme reads and writes.
func (h *Handler) HandleThemePreference(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.HandleThemePreferenceGet(w, r)
	case http.MethodPut:
		h.HandleThemePreferencePut(w, r)
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

// HandleThemePreferenceGet returns the authenticated owner's theme preference.
func (h *Handler) HandleThemePreferenceGet(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	rec, err := h.store.GetUserPreference(r.Context(), ownerID, "theme")
	if err != nil {
		if err == store.ErrNotFound {
			writeAPIJSON(w, http.StatusOK, themePreferenceResponse{Theme: map[string]any{}})
			return
		}
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to get theme preference"})
		return
	}
	writeAPIJSON(w, http.StatusOK, themePreferenceResponse{
		Theme:     rec.Value,
		UpdatedAt: rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	})
}

// HandleThemePreferencePut stores the authenticated owner's theme preference.
func (h *Handler) HandleThemePreferencePut(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	var req themePreferenceRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid theme preference request"})
		return
	}
	if req.Theme == nil {
		req.Theme = map[string]any{}
	}
	rec, err := h.store.SaveUserPreference(r.Context(), types.UserPreference{
		OwnerID:       ownerID,
		PreferenceKey: "theme",
		Value:         req.Theme,
		UpdatedAt:     time.Now().UTC(),
	})
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to save theme preference"})
		return
	}
	_, _ = h.emitProductEvent(r.Context(), ownerID, requestDesktopID(r), types.EventThemeUpdated, map[string]any{
		"theme":            rec.Value,
		"updated_at":       rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
		"source_device_id": strings.TrimSpace(r.Header.Get("X-Choir-Device")),
	})
	writeAPIJSON(w, http.StatusOK, themePreferenceResponse{
		Theme:     rec.Value,
		UpdatedAt: rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	})
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

func sanitizeFloat(value float64) float64 {
	if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return value
}
