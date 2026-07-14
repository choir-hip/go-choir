package content

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
	"github.com/yusefmosiah/go-choir/internal/types"
)

func authenticateUser(r *http.Request) (string, error) {
	user := r.Header.Get("X-Authenticated-User")
	if user == "" {
		return "", fmt.Errorf("missing authenticated user identity")
	}
	return user, nil
}

func writeAPIJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
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

func (s *Service) emitProductEvent(ctx context.Context, ownerID, desktopID string, kind types.EventKind, payload map[string]any) (types.EventRecord, error) {
	if s == nil || s.store == nil {
		return types.EventRecord{}, fmt.Errorf("content store unavailable")
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
	record := types.EventRecord{
		EventID:   uuid.New().String(),
		OwnerID:   ownerID,
		Timestamp: time.Now().UTC(),
		Kind:      kind,
		Phase:     "product",
		Payload:   raw,
	}
	if err := s.store.AppendEvent(ctx, &record); err != nil {
		return types.EventRecord{}, fmt.Errorf("append product event: %w", err)
	}
	if s.bus != nil {
		s.bus.Publish(events.RuntimeEvent{
			Record: record,
			Actor:  events.ActorRuntime,
			Cause:  events.CauseHostAction,
		})
	}
	return record, nil
}

// CreateItem persists a fully constructed content item through the content owner.
// It preserves the caller-supplied item shape and intentionally does not emit a
// product event; callers use it for already-observed source preservation paths.
func (s *Service) CreateItem(ctx context.Context, ownerID string, item types.ContentItem) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("content store unavailable")
	}
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return fmt.Errorf("owner_id is required")
	}
	if strings.TrimSpace(item.OwnerID) != ownerID {
		return fmt.Errorf("content item owner mismatch")
	}
	return s.store.CreateContentItem(ctx, item)
}
