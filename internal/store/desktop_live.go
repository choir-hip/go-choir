package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

const defaultDesktopSessionID = "legacy"
const defaultDriverLease = 60 * time.Second

func normalizeDesktopSessionID(sessionID string) string {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return defaultDesktopSessionID
	}
	return sessionID
}

// GetDesktopStateForSession returns the owner's latest shared desktop app
// instances, placement, and focus. Session IDs are retained as provenance for
// driver leases and live events, but they are not authority boundaries for the
// primary user desktop: one owner's sessions should converge on the same
// visible computer state. If no session-aware records exist yet, it falls back
// to the legacy desktop workspace row so existing computers keep their saved
// windows through the hard cutover.
func (s *Store) GetDesktopStateForSession(ctx context.Context, ownerID, desktopID, sessionID string) (types.DesktopState, error) {
	desktopID = normalizeDesktopID(desktopID)
	sessionID = normalizeDesktopSessionID(sessionID)

	state, found, err := s.getSessionAwareDesktopState(ctx, ownerID, desktopID, sessionID)
	if err != nil {
		return types.DesktopState{}, err
	}
	if found {
		return state, nil
	}
	return s.getLegacyDesktopState(ctx, ownerID, desktopID)
}

// SaveDesktopStateForSession stores the owner's canonical app roster, active
// focus, and placement. The saving session is recorded as provenance/driver,
// but its placement replaces prior session placements so the owner's devices
// converge on one desktop. Passive sessions only update their presence row;
// they cannot overwrite the shared app roster, active focus, or placement.
func (s *Store) SaveDesktopStateForSession(ctx context.Context, state types.DesktopState, session types.DesktopSessionContext) error {
	desktopID := normalizeDesktopID(state.DesktopID)
	sessionID := normalizeDesktopSessionID(session.SessionID)
	now := state.UpdatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if session.UpdatedAt.IsZero() {
		session.UpdatedAt = now
	}
	if session.IsDriver && session.DriverUntil.IsZero() {
		session.DriverUntil = now.Add(defaultDriverLease)
	}

	if err := s.upsertDesktopSession(ctx, state.OwnerID, desktopID, sessionID, session); err != nil {
		return err
	}
	if !session.IsDriver {
		return nil
	}

	windowsJSON, err := json.Marshal(state.Windows)
	if err != nil {
		return fmt.Errorf("marshal desktop windows: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin desktop state transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO desktop_workspaces (owner_id, desktop_id, windows_json, active_window, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE
		   windows_json = VALUES(windows_json),
		   active_window = VALUES(active_window),
		   updated_at = VALUES(updated_at)`,
		state.OwnerID,
		desktopID,
		string(windowsJSON),
		state.ActiveWindowID,
		now.UTC().Format(time.RFC3339Nano),
	); err != nil {
		return fmt.Errorf("save legacy desktop workspace mirror: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM desktop_app_instances WHERE owner_id = ? AND desktop_id = ?`,
		state.OwnerID,
		desktopID,
	); err != nil {
		return fmt.Errorf("replace desktop app instances: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM desktop_window_placements WHERE owner_id = ? AND desktop_id = ?`,
		state.OwnerID,
		desktopID,
	); err != nil {
		return fmt.Errorf("replace desktop window placements: %w", err)
	}

	presentIDs := make([]string, 0, len(state.Windows))
	for i, win := range state.Windows {
		appInstanceID := strings.TrimSpace(win.WindowID)
		if appInstanceID == "" {
			continue
		}
		stackRank := win.ZIndex
		if stackRank <= 0 {
			stackRank = i + 1
		}
		presentIDs = append(presentIDs, appInstanceID)
		appContextJSON, err := json.Marshal(win.AppContext)
		if err != nil {
			return fmt.Errorf("marshal desktop app context: %w", err)
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
			state.OwnerID,
			desktopID,
			appInstanceID,
			strings.TrimSpace(win.AppID),
			strings.TrimSpace(win.Title),
			string(appContextJSON),
			lifecycle,
			stackRank,
			now.UTC().Format(time.RFC3339Nano),
			sessionID,
			now.UTC().Format(time.RFC3339Nano),
			now.UTC().Format(time.RFC3339Nano),
		); err != nil {
			return fmt.Errorf("insert desktop app instance: %w", err)
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
			state.OwnerID,
			desktopID,
			sessionID,
			appInstanceID,
			win.Geometry.X,
			win.Geometry.Y,
			win.Geometry.Width,
			win.Geometry.Height,
			string(win.Mode),
			stackRank,
			win.WindowID == state.ActiveWindowID,
			restoredGeometryJSON,
			now.UTC().Format(time.RFC3339Nano),
		); err != nil {
			return fmt.Errorf("insert desktop window placement: %w", err)
		}
	}

	if len(presentIDs) == 0 {
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM desktop_window_placements WHERE owner_id = ? AND desktop_id = ?`,
			state.OwnerID,
			desktopID,
		); err != nil {
			return fmt.Errorf("delete desktop placements for empty desktop: %w", err)
		}
	} else if err := deleteDesktopPlacementsNotIn(ctx, tx, state.OwnerID, desktopID, presentIDs); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit desktop state transaction: %w", err)
	}
	return nil
}

func (s *Store) upsertDesktopSession(ctx context.Context, ownerID, desktopID, sessionID string, session types.DesktopSessionContext) error {
	now := session.UpdatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	var lastInput, driverUntil any
	if session.IsDriver {
		lastInput = now.UTC().Format(time.RFC3339Nano)
		driverUntil = session.DriverUntil.UTC().Format(time.RFC3339Nano)
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO desktop_sessions (
			owner_id, desktop_id, session_id, device_id, viewport_profile,
			visibility_state, last_input_at, driver_until, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			device_id = VALUES(device_id),
			viewport_profile = VALUES(viewport_profile),
			visibility_state = VALUES(visibility_state),
			last_input_at = COALESCE(VALUES(last_input_at), last_input_at),
			driver_until = COALESCE(VALUES(driver_until), driver_until),
			updated_at = VALUES(updated_at)`,
		ownerID,
		desktopID,
		sessionID,
		strings.TrimSpace(session.DeviceID),
		strings.TrimSpace(session.ViewportProfile),
		"",
		lastInput,
		driverUntil,
		now.UTC().Format(time.RFC3339Nano),
		now.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("upsert desktop session: %w", err)
	}
	return nil
}

func deleteDesktopPlacementsNotIn(ctx context.Context, tx *sql.Tx, ownerID, desktopID string, presentIDs []string) error {
	placeholders := make([]string, 0, len(presentIDs))
	args := []any{ownerID, desktopID}
	for _, id := range presentIDs {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	query := fmt.Sprintf(
		`DELETE FROM desktop_window_placements
		  WHERE owner_id = ? AND desktop_id = ? AND app_instance_id NOT IN (%s)`,
		strings.Join(placeholders, ","),
	)
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("delete orphan desktop placements: %w", err)
	}
	return nil
}

func (s *Store) getSessionAwareDesktopState(ctx context.Context, ownerID, desktopID, sessionID string) (types.DesktopState, bool, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT app_instance_id, app_id, title, app_context_json, lifecycle, shared_stack_rank, updated_at
		   FROM desktop_app_instances
		  WHERE owner_id = ? AND desktop_id = ?
		  ORDER BY shared_stack_rank ASC, updated_at ASC`,
		ownerID,
		desktopID,
	)
	if err != nil {
		return types.DesktopState{}, false, fmt.Errorf("query desktop app instances: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var instances []desktopAppInstanceRow
	for rows.Next() {
		var row desktopAppInstanceRow
		if err := rows.Scan(&row.appInstanceID, &row.appID, &row.title, &row.appContextJSON, &row.lifecycle, &row.sharedStackRank, &row.updatedRaw); err != nil {
			return types.DesktopState{}, false, fmt.Errorf("scan desktop app instance: %w", err)
		}
		instances = append(instances, row)
	}
	if err := rows.Err(); err != nil {
		return types.DesktopState{}, false, fmt.Errorf("iterate desktop app instances: %w", err)
	}
	if len(instances) == 0 {
		return types.DesktopState{}, false, nil
	}
	if err := rows.Close(); err != nil {
		return types.DesktopState{}, false, fmt.Errorf("close desktop app instances: %w", err)
	}

	var windows []types.WindowState
	activeWindowID := ""
	hadSessionPlacement := false
	updatedAt := time.Now().UTC()
	for _, row := range instances {
		placement, placementFound, err := s.findDesktopPlacement(ctx, ownerID, desktopID, sessionID, row.appInstanceID)
		if err != nil {
			return types.DesktopState{}, false, err
		}
		win := types.WindowState{
			WindowID: row.appInstanceID,
			AppID:    row.appID,
			Title:    row.title,
			Geometry: types.WindowGeometry{X: 100, Y: 100, Width: 600, Height: 400},
			Mode:     types.WindowNormal,
			ZIndex:   row.sharedStackRank,
		}
		if row.appContextJSON != "" {
			var appContext map[string]any
			if err := json.Unmarshal([]byte(row.appContextJSON), &appContext); err != nil {
				return types.DesktopState{}, false, fmt.Errorf("unmarshal desktop app context: %w", err)
			}
			win.AppContext = appContext
		}
		if placementFound {
			win.Geometry = placement.geometry
			win.Mode = placement.mode
			win.ZIndex = placement.localZIndex
			win.RestoredGeometry = placement.restoredGeometry
			if placement.sessionID == sessionID {
				hadSessionPlacement = true
			}
			if placement.localFocused {
				activeWindowID = row.appInstanceID
			}
			if placement.updatedAt.After(updatedAt) {
				updatedAt = placement.updatedAt
			}
		} else if row.lifecycle == "minimized" {
			win.Mode = types.WindowMinimized
		}
		windows = append(windows, win)
	}
	if activeWindowID == "" && !hadSessionPlacement {
		for _, win := range windows {
			if win.ZIndex >= 0 && (activeWindowID == "" || win.ZIndex > zIndexForWindow(windows, activeWindowID)) {
				activeWindowID = win.WindowID
			}
		}
	}
	return types.DesktopState{
		OwnerID:        ownerID,
		DesktopID:      desktopID,
		Windows:        windows,
		ActiveWindowID: activeWindowID,
		UpdatedAt:      updatedAt,
	}, true, nil
}

type desktopAppInstanceRow struct {
	appInstanceID   string
	appID           string
	title           string
	appContextJSON  string
	lifecycle       string
	sharedStackRank int
	updatedRaw      string
}

type desktopPlacement struct {
	sessionID        string
	geometry         types.WindowGeometry
	mode             types.WindowMode
	localZIndex      int
	localFocused     bool
	restoredGeometry *types.WindowGeometry
	updatedAt        time.Time
}

func (s *Store) findDesktopPlacement(ctx context.Context, ownerID, desktopID, sessionID, appInstanceID string) (desktopPlacement, bool, error) {
	return s.queryLatestDesktopPlacement(ctx, ownerID, desktopID, appInstanceID)
}

func (s *Store) queryDesktopPlacement(ctx context.Context, ownerID, desktopID, sessionID, appInstanceID string) (desktopPlacement, bool, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT session_id, x, y, width, height, mode, local_z_index, local_focused, restored_geometry_json, updated_at
		   FROM desktop_window_placements
		  WHERE owner_id = ? AND desktop_id = ? AND session_id = ? AND app_instance_id = ?`,
		ownerID,
		desktopID,
		sessionID,
		appInstanceID,
	)
	return scanDesktopPlacement(row)
}

func (s *Store) queryLatestDesktopPlacement(ctx context.Context, ownerID, desktopID, appInstanceID string) (desktopPlacement, bool, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT session_id, x, y, width, height, mode, local_z_index, local_focused, restored_geometry_json, updated_at
		   FROM desktop_window_placements
		  WHERE owner_id = ? AND desktop_id = ? AND app_instance_id = ?
		  ORDER BY updated_at DESC
		  LIMIT 1`,
		ownerID,
		desktopID,
		appInstanceID,
	)
	return scanDesktopPlacement(row)
}

func scanDesktopPlacement(row interface{ Scan(...any) error }) (desktopPlacement, bool, error) {
	var p desktopPlacement
	var mode, restoredRaw, updatedRaw string
	if err := row.Scan(
		&p.sessionID,
		&p.geometry.X,
		&p.geometry.Y,
		&p.geometry.Width,
		&p.geometry.Height,
		&mode,
		&p.localZIndex,
		&p.localFocused,
		&restoredRaw,
		&updatedRaw,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return desktopPlacement{}, false, nil
		}
		return desktopPlacement{}, false, fmt.Errorf("scan desktop placement: %w", err)
	}
	p.mode = types.WindowMode(mode)
	if !p.mode.Valid() {
		p.mode = types.WindowNormal
	}
	if strings.TrimSpace(restoredRaw) != "" {
		var restored types.WindowGeometry
		if err := json.Unmarshal([]byte(restoredRaw), &restored); err != nil {
			return desktopPlacement{}, false, fmt.Errorf("unmarshal restored geometry: %w", err)
		}
		p.restoredGeometry = &restored
	}
	parsed, err := time.Parse(time.RFC3339Nano, updatedRaw)
	if err != nil {
		parsed = time.Now().UTC()
	}
	p.updatedAt = parsed
	return p, true, nil
}

func zIndexForWindow(windows []types.WindowState, windowID string) int {
	for _, win := range windows {
		if win.WindowID == windowID {
			return win.ZIndex
		}
	}
	return -1
}

func (s *Store) getLegacyDesktopState(ctx context.Context, ownerID, desktopID string) (types.DesktopState, error) {
	var windowsJSON, updatedAt string
	var activeWindow string

	row := s.db.QueryRowContext(ctx,
		`SELECT windows_json, active_window, updated_at
		   FROM desktop_workspaces
		  WHERE owner_id = ? AND desktop_id = ?`,
		ownerID, desktopID,
	)

	err := row.Scan(&windowsJSON, &activeWindow, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.DesktopState{
				OwnerID:        ownerID,
				DesktopID:      desktopID,
				Windows:        []types.WindowState{},
				ActiveWindowID: "",
				UpdatedAt:      time.Now().UTC(),
			}, nil
		}
		return types.DesktopState{}, fmt.Errorf("query legacy desktop state: %w", err)
	}

	var windows []types.WindowState
	if err := json.Unmarshal([]byte(windowsJSON), &windows); err != nil {
		return types.DesktopState{}, fmt.Errorf("unmarshal legacy desktop windows: %w", err)
	}

	parsedTime, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		parsedTime = time.Now().UTC()
	}

	return types.DesktopState{
		OwnerID:        ownerID,
		DesktopID:      desktopID,
		Windows:        windows,
		ActiveWindowID: activeWindow,
		UpdatedAt:      parsedTime,
	}, nil
}
