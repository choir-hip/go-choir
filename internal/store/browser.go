package store

import (
	"context"
	"fmt"
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
