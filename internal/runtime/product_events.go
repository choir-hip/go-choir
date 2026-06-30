package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func (rt *Runtime) emitProductEvent(ctx context.Context, ownerID, desktopID string, kind types.EventKind, payload map[string]any) (types.EventRecord, error) {
	return rt.EmitProductEvent(ctx, ownerID, desktopID, kind, payload)
}

// EmitProductEvent persists an owner-scoped product event, then publishes it to
// the in-process bus for live client notification.
func (rt *Runtime) EmitProductEvent(ctx context.Context, ownerID, desktopID string, kind types.EventKind, payload map[string]any) (types.EventRecord, error) {
	if rt == nil || rt.store == nil {
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
	if err := rt.appendEventRecord(ctx, &rec); err != nil {
		return types.EventRecord{}, fmt.Errorf("append product event: %w", err)
	}
	if rt.bus != nil {
		rt.bus.Publish(events.RuntimeEvent{
			Record: rec,
			Actor:  events.ActorRuntime,
			Cause:  events.CauseHostAction,
		})
	}
	return rec, nil
}
