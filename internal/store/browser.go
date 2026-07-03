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
	if err := s.CreateBrowserSessionOG(ctx, rec); err != nil {
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
	// Verify the session exists (preserves ErrNotFound semantics).
	if _, err := s.GetBrowserSessionOG(ctx, rec.OwnerID, rec.SessionID); err != nil {
		return types.BrowserSessionRecord{}, err
	}
	if err := s.CreateBrowserSessionOG(ctx, rec); err != nil {
		return types.BrowserSessionRecord{}, fmt.Errorf("update browser session: %w", err)
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
	return s.GetBrowserSessionOG(ctx, ownerID, sessionID)
}

// ListBrowserSessions returns recent browser sessions for an owner.
func (s *Store) ListBrowserSessions(ctx context.Context, ownerID string, limit int) ([]types.BrowserSessionRecord, error) {
	if ownerID == "" {
		return nil, fmt.Errorf("list browser sessions: owner_id is required")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.ListBrowserSessionsByOwnerOG(ctx, ownerID, limit)
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
