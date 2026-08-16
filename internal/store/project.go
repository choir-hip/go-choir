package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type projectedDesktopState struct {
	OwnerID            string                     `json:"owner_id"`
	DesktopID          string                     `json:"desktop_id"`
	Windows            []types.WindowState        `json:"windows"`
	ActiveWindowID     string                     `json:"active_window_id"`
	UpdatedAt          time.Time                  `json:"updated_at"`
	CreatedBySessionID string                     `json:"created_by_session_id,omitempty"`
	Sessions           []projectedSessionIdentity `json:"sessions,omitempty"`
}

// projectedSessionIdentity is the witnessed session row. Presence fields
// (last_input_at, driver_until, visibility_state, is_driver) are not tape
// payloads and must not appear here.
type projectedSessionIdentity struct {
	SessionID       string `json:"session_id"`
	DeviceID        string `json:"device_id,omitempty"`
	ViewportProfile string `json:"viewport_profile,omitempty"`
}

func projectBatch(ctx context.Context, tx *sql.Tx, batch computerevent.ProjectionBatch) error {
	if tx == nil {
		return fmt.Errorf("computer event projection: missing transaction")
	}
	if err := batch.Validate(); err != nil {
		return err
	}
	for i, op := range batch.Ops {
		if err := projectOp(ctx, tx, op); err != nil {
			return fmt.Errorf("computer event projection: op %d: %w", i, err)
		}
	}
	return nil
}

func projectOp(ctx context.Context, tx *sql.Tx, op computerevent.ProjectionOp) error {
	switch strings.TrimSpace(op.Kind) {
	case computerevent.ProjectionOpDesktopState:
		return projectDesktopState(ctx, tx, op.Body)
	case computerevent.ProjectionOpObject:
		return projectObject(ctx, tx, op.Body)
	case computerevent.ProjectionOpObjectEdge:
		return projectObjectEdge(ctx, tx, op.Body)
	default:
		return fmt.Errorf("%w: kind %q", computerevent.ErrProjectionBatchInvalid, op.Kind)
	}
}

func projectDesktopState(ctx context.Context, tx *sql.Tx, body json.RawMessage) error {
	var state projectedDesktopState
	if err := json.Unmarshal(body, &state); err != nil {
		return fmt.Errorf("decode desktop state: %w", err)
	}
	ownerID := strings.TrimSpace(state.OwnerID)
	desktopID := normalizeDesktopID(state.DesktopID)
	if ownerID == "" {
		return fmt.Errorf("desktop state owner is required")
	}
	now := state.UpdatedAt.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	sessionID := strings.TrimSpace(state.CreatedBySessionID)
	if sessionID == "" {
		sessionID = "projected"
	}
	windowsJSON, err := json.Marshal(state.Windows)
	if err != nil {
		return fmt.Errorf("marshal desktop windows: %w", err)
	}
	stamp := now.Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO desktop_workspaces (owner_id, desktop_id, windows_json, active_window, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE
		   windows_json = VALUES(windows_json),
		   active_window = VALUES(active_window),
		   updated_at = VALUES(updated_at)`,
		ownerID, desktopID, string(windowsJSON), state.ActiveWindowID, stamp,
	); err != nil {
		return fmt.Errorf("project desktop workspace: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM desktop_app_instances WHERE owner_id = ? AND desktop_id = ?`, ownerID, desktopID); err != nil {
		return fmt.Errorf("replace projected app instances: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM desktop_window_placements WHERE owner_id = ? AND desktop_id = ?`, ownerID, desktopID); err != nil {
		return fmt.Errorf("replace projected placements: %w", err)
	}
	for i, win := range state.Windows {
		appInstanceID := strings.TrimSpace(win.WindowID)
		if appInstanceID == "" {
			continue
		}
		stackRank := win.ZIndex
		if stackRank <= 0 {
			stackRank = i + 1
		}
		appContextJSON, err := json.Marshal(win.AppContext)
		if err != nil {
			return fmt.Errorf("marshal desktop app context: %w", err)
		}
		if len(appContextJSON) == 0 || string(appContextJSON) == "null" {
			appContextJSON = []byte("{}")
		}
		lifecycle := "open"
		if win.Mode == types.WindowMinimized {
			lifecycle = "minimized"
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO desktop_app_instances (
				owner_id, desktop_id, app_instance_id, app_id, title, app_context_json,
				lifecycle, shared_stack_rank, last_used_at, created_by_session_id, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			ownerID, desktopID, appInstanceID, strings.TrimSpace(win.AppID), strings.TrimSpace(win.Title),
			string(appContextJSON), lifecycle, stackRank, stamp, sessionID, stamp, stamp,
		); err != nil {
			return fmt.Errorf("insert projected app instance: %w", err)
		}
		restoredGeometryJSON := ""
		if win.RestoredGeometry != nil {
			raw, err := json.Marshal(win.RestoredGeometry)
			if err != nil {
				return fmt.Errorf("marshal restored geometry: %w", err)
			}
			restoredGeometryJSON = string(raw)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO desktop_window_placements (
				owner_id, desktop_id, session_id, app_instance_id,
				x, y, width, height, mode, local_z_index, local_focused,
				restored_geometry_json, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			ownerID, desktopID, sessionID, appInstanceID,
			win.Geometry.X, win.Geometry.Y, win.Geometry.Width, win.Geometry.Height,
			string(win.Mode), stackRank, win.WindowID == state.ActiveWindowID,
			restoredGeometryJSON, stamp,
		); err != nil {
			return fmt.Errorf("insert projected placement: %w", err)
		}
	}
	for _, session := range state.Sessions {
		sessionID := strings.TrimSpace(session.SessionID)
		if sessionID == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO desktop_sessions (
				owner_id, desktop_id, session_id, device_id, viewport_profile,
				visibility_state, last_input_at, driver_until, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, '', NULL, NULL, ?, ?)
			ON DUPLICATE KEY UPDATE
				device_id = VALUES(device_id),
				viewport_profile = VALUES(viewport_profile),
				visibility_state = '',
				last_input_at = NULL,
				driver_until = NULL,
				updated_at = VALUES(updated_at)`,
			ownerID, desktopID, sessionID, strings.TrimSpace(session.DeviceID),
			strings.TrimSpace(session.ViewportProfile), stamp, stamp,
		); err != nil {
			return fmt.Errorf("project session identity: %w", err)
		}
	}
	return nil
}

func projectObject(ctx context.Context, tx *sql.Tx, body json.RawMessage) error {
	var obj objectgraph.Object
	if err := json.Unmarshal(body, &obj); err != nil {
		return fmt.Errorf("decode object: %w", err)
	}
	if strings.TrimSpace(obj.CanonicalID) == "" || strings.TrimSpace(string(obj.ObjectKind)) == "" {
		return fmt.Errorf("object identity is required")
	}
	metadata := string(obj.Metadata)
	if strings.TrimSpace(metadata) == "" {
		metadata = "{}"
	}
	created := obj.CreatedAt.UTC()
	updated := obj.UpdatedAt.UTC()
	if created.IsZero() {
		created = time.Now().UTC()
	}
	if updated.IsZero() {
		updated = created
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO og_objects
		(canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			object_kind = VALUES(object_kind),
			owner_id = VALUES(owner_id),
			computer_id = VALUES(computer_id),
			version_id = VALUES(version_id),
			content_hash = VALUES(content_hash),
			body = VALUES(body),
			metadata = VALUES(metadata),
			updated_at = VALUES(updated_at),
			tombstone = VALUES(tombstone),
			superseded_by = VALUES(superseded_by)`,
		obj.CanonicalID, string(obj.ObjectKind), obj.OwnerID, obj.ComputerID,
		obj.VersionID, obj.ContentHash, obj.Body, metadata,
		created, updated, obj.Tombstone, obj.SupersededBy); err != nil {
		return fmt.Errorf("project object: %w", err)
	}
	return nil
}

func projectObjectEdge(ctx context.Context, tx *sql.Tx, body json.RawMessage) error {
	var edge objectgraph.Edge
	if err := json.Unmarshal(body, &edge); err != nil {
		return fmt.Errorf("decode object edge: %w", err)
	}
	if strings.TrimSpace(edge.EdgeID) == "" || strings.TrimSpace(edge.FromID) == "" || strings.TrimSpace(edge.ToID) == "" {
		return fmt.Errorf("edge identity is required")
	}
	metadata := string(edge.Metadata)
	if strings.TrimSpace(metadata) == "" {
		metadata = "{}"
	}
	created := edge.CreatedAt.UTC()
	if created.IsZero() {
		created = time.Now().UTC()
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO og_edges
		(edge_id, from_id, to_id, kind, metadata, created_at, tombstone)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			from_id = VALUES(from_id),
			to_id = VALUES(to_id),
			kind = VALUES(kind),
			metadata = VALUES(metadata),
			tombstone = VALUES(tombstone)`,
		edge.EdgeID, edge.FromID, edge.ToID, string(edge.Kind), metadata, created, edge.Tombstone); err != nil {
		return fmt.Errorf("project object edge: %w", err)
	}
	return nil
}
