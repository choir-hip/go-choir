package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// CreateBrowserSession creates a durable owner-scoped backend browser session.
func (s *Store) CreateBrowserSession(ctx context.Context, rec types.BrowserSessionRecord) (types.BrowserSessionRecord, error) {
	if rec.OwnerID == "" {
		return types.BrowserSessionRecord{}, fmt.Errorf("create browser session: owner_id is required")
	}
	if rec.SessionID == "" {
		rec.SessionID = uuid.NewString()
	}
	if rec.Provider == "" {
		rec.Provider = "obscura"
	}
	if rec.State == "" {
		rec.State = types.BrowserSessionIdle
	}
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = now
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO browser_sessions (
			session_id, owner_id, provider, mode, execution_scope, backend_session_id,
			world_kind, promotion_candidate_id, vm_id, snapshot_id, source_loop_id, candidate_trace_id,
			state, current_url,
			title, text_snapshot, html_snapshot, links_json, screenshot_png_base64,
			error, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.SessionID,
		rec.OwnerID,
		rec.Provider,
		rec.Mode,
		rec.ExecutionScope,
		rec.BackendSessionID,
		rec.WorldKind,
		rec.CandidateID,
		rec.VMID,
		rec.SnapshotID,
		rec.SourceRunID,
		rec.CandidateTraceID,
		rec.State,
		rec.CurrentURL,
		rec.Title,
		rec.TextSnapshot,
		rec.HTMLSnapshot,
		encodeBrowserLinks(rec.Links),
		rec.ScreenshotPNG,
		rec.Error,
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.BrowserSessionRecord{}, fmt.Errorf("create browser session: %w", err)
	}
	return rec, nil
}

// UpdateBrowserSession updates an existing owner-scoped backend browser session.
func (s *Store) UpdateBrowserSession(ctx context.Context, rec types.BrowserSessionRecord) (types.BrowserSessionRecord, error) {
	if rec.OwnerID == "" {
		return types.BrowserSessionRecord{}, fmt.Errorf("update browser session: owner_id is required")
	}
	if rec.SessionID == "" {
		return types.BrowserSessionRecord{}, fmt.Errorf("update browser session: session_id is required")
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = time.Now().UTC()
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE browser_sessions
		    SET provider = ?,
		        mode = ?,
		        execution_scope = ?,
		        backend_session_id = ?,
		        world_kind = ?,
		        promotion_candidate_id = ?,
		        vm_id = ?,
		        snapshot_id = ?,
		        source_loop_id = ?,
		        candidate_trace_id = ?,
		        state = ?,
		        current_url = ?,
		        title = ?,
		        text_snapshot = ?,
		        html_snapshot = ?,
		        links_json = ?,
		        screenshot_png_base64 = ?,
		        error = ?,
		        updated_at = ?
		  WHERE owner_id = ? AND session_id = ?`,
		rec.Provider,
		rec.Mode,
		rec.ExecutionScope,
		rec.BackendSessionID,
		rec.WorldKind,
		rec.CandidateID,
		rec.VMID,
		rec.SnapshotID,
		rec.SourceRunID,
		rec.CandidateTraceID,
		rec.State,
		rec.CurrentURL,
		rec.Title,
		rec.TextSnapshot,
		rec.HTMLSnapshot,
		encodeBrowserLinks(rec.Links),
		rec.ScreenshotPNG,
		rec.Error,
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
		rec.OwnerID,
		rec.SessionID,
	)
	if err != nil {
		return types.BrowserSessionRecord{}, fmt.Errorf("update browser session: %w", err)
	}
	count, err := res.RowsAffected()
	if err != nil {
		return types.BrowserSessionRecord{}, fmt.Errorf("update browser session rows: %w", err)
	}
	if count == 0 {
		return types.BrowserSessionRecord{}, ErrNotFound
	}
	return rec, nil
}

// GetBrowserSession returns a browser session scoped to its owner.
func (s *Store) GetBrowserSession(ctx context.Context, ownerID, sessionID string) (types.BrowserSessionRecord, error) {
	if ownerID == "" {
		return types.BrowserSessionRecord{}, fmt.Errorf("get browser session: owner_id is required")
	}
	if sessionID == "" {
		return types.BrowserSessionRecord{}, fmt.Errorf("get browser session: session_id is required")
	}
	row := s.db.QueryRowContext(ctx,
		`SELECT session_id, owner_id, provider, mode, execution_scope, backend_session_id,
		        world_kind, promotion_candidate_id, vm_id, snapshot_id, source_loop_id, candidate_trace_id,
		        state, current_url, title, text_snapshot, html_snapshot, links_json, screenshot_png_base64,
		        error, created_at, updated_at
		   FROM browser_sessions
		  WHERE owner_id = ? AND session_id = ?`,
		ownerID,
		sessionID,
	)
	return scanBrowserSession(row)
}

// ListBrowserSessions returns recent browser sessions for an owner.
func (s *Store) ListBrowserSessions(ctx context.Context, ownerID string, limit int) ([]types.BrowserSessionRecord, error) {
	if ownerID == "" {
		return nil, fmt.Errorf("list browser sessions: owner_id is required")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT session_id, owner_id, provider, mode, execution_scope, backend_session_id,
		        world_kind, promotion_candidate_id, vm_id, snapshot_id, source_loop_id, candidate_trace_id,
		        state, current_url, title, text_snapshot, html_snapshot, links_json, screenshot_png_base64,
		        error, created_at, updated_at
		   FROM browser_sessions
		  WHERE owner_id = ?
		  ORDER BY updated_at DESC, created_at DESC
		  LIMIT ?`,
		ownerID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query browser sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	sessions := []types.BrowserSessionRecord{}
	for rows.Next() {
		rec, err := scanBrowserSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate browser sessions: %w", err)
	}
	return sessions, nil
}

func scanBrowserSession(scanner interface {
	Scan(dest ...any) error
}) (types.BrowserSessionRecord, error) {
	var rec types.BrowserSessionRecord
	var created, updated string
	var linksJSON string
	if err := scanner.Scan(
		&rec.SessionID,
		&rec.OwnerID,
		&rec.Provider,
		&rec.Mode,
		&rec.ExecutionScope,
		&rec.BackendSessionID,
		&rec.WorldKind,
		&rec.CandidateID,
		&rec.VMID,
		&rec.SnapshotID,
		&rec.SourceRunID,
		&rec.CandidateTraceID,
		&rec.State,
		&rec.CurrentURL,
		&rec.Title,
		&rec.TextSnapshot,
		&rec.HTMLSnapshot,
		&linksJSON,
		&rec.ScreenshotPNG,
		&rec.Error,
		&created,
		&updated,
	); err != nil {
		if err == sql.ErrNoRows {
			return types.BrowserSessionRecord{}, ErrNotFound
		}
		return types.BrowserSessionRecord{}, fmt.Errorf("scan browser session: %w", err)
	}
	createdAt, err := time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return types.BrowserSessionRecord{}, fmt.Errorf("parse browser session created_at: %w", err)
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, updated)
	if err != nil {
		return types.BrowserSessionRecord{}, fmt.Errorf("parse browser session updated_at: %w", err)
	}
	rec.CreatedAt = createdAt
	rec.UpdatedAt = updatedAt
	if strings.TrimSpace(linksJSON) != "" {
		if err := json.Unmarshal([]byte(linksJSON), &rec.Links); err != nil {
			return types.BrowserSessionRecord{}, fmt.Errorf("decode browser session links: %w", err)
		}
	}
	return rec, nil
}

func encodeBrowserLinks(links []types.BrowserLink) string {
	if len(links) == 0 {
		return "[]"
	}
	raw, err := json.Marshal(links)
	if err != nil {
		return "[]"
	}
	return string(raw)
}
