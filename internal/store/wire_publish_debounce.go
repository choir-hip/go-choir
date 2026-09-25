package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// WirePublishDebounceEntry is one publish awaiting story-corpus reconciliation.
// Content is owned by the caller so the store need not know the wire lineage schema.
type WirePublishDebounceEntry struct {
	Content   string
	CreatedAt time.Time
}

// WirePublishDebounceResult is the atomically observed pending batch. Due
// reports whether it is ready; Fired reports that this call consumed it.
type WirePublishDebounceResult struct {
	Entries  []WirePublishDebounceEntry
	Deadline time.Time
	Due      bool
	Fired    bool
}

// RecordWirePublishDebounceEntry persists one publish and reports whether the
// batch is ready for a durable deadline delivery.
func (s *Store) RecordWirePublishDebounceEntry(ctx context.Context, ownerID, computerID, agentID, content string, now time.Time, threshold int, interval time.Duration) (WirePublishDebounceResult, error) {
	if err := validateWirePublishDebounceScope(ownerID, computerID, agentID, content, threshold, interval); err != nil {
		return WirePublishDebounceResult{}, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()

	s.wirePublishDebounceMu.Lock()
	defer s.wirePublishDebounceMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return WirePublishDebounceResult{}, fmt.Errorf("begin wire publish debounce: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO wire_publish_debounce_entries
		(owner_id, computer_id, agent_id, content, created_at) VALUES (?, ?, ?, ?, ?)`,
		ownerID, computerID, agentID, content, now.Format(time.RFC3339Nano)); err != nil {
		return WirePublishDebounceResult{}, fmt.Errorf("persist wire publish debounce entry: %w", err)
	}
	result, err := inspectWirePublishDebounceBatch(ctx, tx, ownerID, computerID, agentID, now, threshold, interval)
	if err != nil {
		return WirePublishDebounceResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return WirePublishDebounceResult{}, fmt.Errorf("commit wire publish debounce entry: %w", err)
	}
	s.markDoltHistoryDirty()
	return result, nil
}

// ConsumeDueWirePublishDebounceBatch atomically reads and clears a batch only
// when its persisted deadline has passed. Stale scheduled events therefore
// become no-ops without losing newer pending publications.
func (s *Store) ConsumeDueWirePublishDebounceBatch(ctx context.Context, ownerID, computerID, agentID string, now time.Time, threshold int, interval time.Duration) (WirePublishDebounceResult, error) {
	if err := validateWirePublishDebounceScope(ownerID, computerID, agentID, "x", threshold, interval); err != nil {
		return WirePublishDebounceResult{}, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()

	s.wirePublishDebounceMu.Lock()
	defer s.wirePublishDebounceMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return WirePublishDebounceResult{}, fmt.Errorf("begin consume wire publish debounce: %w", err)
	}
	defer tx.Rollback()
	result, err := consumeWirePublishDebounceBatch(ctx, tx, ownerID, computerID, agentID, now, threshold, interval)
	if err != nil {
		return WirePublishDebounceResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return WirePublishDebounceResult{}, fmt.Errorf("commit consume wire publish debounce: %w", err)
	}
	if result.Fired {
		s.markDoltHistoryDirty()
	}
	return result, nil
}

func validateWirePublishDebounceScope(ownerID, computerID, agentID, content string, threshold int, interval time.Duration) error {
	if strings.TrimSpace(ownerID) == "" || strings.TrimSpace(computerID) == "" || strings.TrimSpace(agentID) == "" {
		return fmt.Errorf("wire publish debounce scope requires owner, computer, and agent")
	}
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("wire publish debounce content is required")
	}
	if threshold <= 0 || interval <= 0 {
		return fmt.Errorf("wire publish debounce threshold and interval must be positive")
	}
	return nil
}

func inspectWirePublishDebounceBatch(ctx context.Context, tx *sql.Tx, ownerID, computerID, agentID string, now time.Time, threshold int, interval time.Duration) (WirePublishDebounceResult, error) {
	entries, err := pendingWirePublishDebounceEntries(ctx, tx, ownerID, computerID, agentID)
	if err != nil {
		return WirePublishDebounceResult{}, err
	}
	if len(entries) == 0 {
		return WirePublishDebounceResult{}, nil
	}
	lastDispatch, err := wirePublishDebounceLastDispatch(ctx, tx, ownerID, computerID, agentID)
	if err != nil {
		return WirePublishDebounceResult{}, err
	}
	deadline := entries[0].CreatedAt.Add(interval)
	if !lastDispatch.IsZero() {
		deadline = lastDispatch.Add(interval)
	}
	result := WirePublishDebounceResult{Entries: entries, Deadline: deadline}
	if len(entries) < threshold && now.Before(deadline) {
		return result, nil
	}
	result.Due = true
	return result, nil
}

func consumeWirePublishDebounceBatch(ctx context.Context, tx *sql.Tx, ownerID, computerID, agentID string, now time.Time, threshold int, interval time.Duration) (WirePublishDebounceResult, error) {
	result, err := inspectWirePublishDebounceBatch(ctx, tx, ownerID, computerID, agentID, now, threshold, interval)
	if err != nil || !result.Due {
		return result, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM wire_publish_debounce_entries
		WHERE owner_id=? AND computer_id=? AND agent_id=?`, ownerID, computerID, agentID); err != nil {
		return WirePublishDebounceResult{}, fmt.Errorf("clear wire publish debounce entries: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO wire_publish_debounce_state
		(owner_id, computer_id, agent_id, last_dispatch_at) VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE last_dispatch_at=VALUES(last_dispatch_at)`,
		ownerID, computerID, agentID, now.Format(time.RFC3339Nano)); err != nil {
		return WirePublishDebounceResult{}, fmt.Errorf("record wire publish debounce dispatch: %w", err)
	}
	result.Fired = true
	return result, nil
}

func pendingWirePublishDebounceEntries(ctx context.Context, tx *sql.Tx, ownerID, computerID, agentID string) ([]WirePublishDebounceEntry, error) {
	rows, err := tx.QueryContext(ctx, `SELECT content, created_at FROM wire_publish_debounce_entries
		WHERE owner_id=? AND computer_id=? AND agent_id=? ORDER BY entry_id FOR UPDATE`, ownerID, computerID, agentID)
	if err != nil {
		return nil, fmt.Errorf("load wire publish debounce entries: %w", err)
	}
	defer rows.Close()
	var entries []WirePublishDebounceEntry
	for rows.Next() {
		var entry WirePublishDebounceEntry
		var createdAt string
		if err := rows.Scan(&entry.Content, &createdAt); err != nil {
			return nil, fmt.Errorf("scan wire publish debounce entry: %w", err)
		}
		parsed, err := time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse wire publish debounce entry time: %w", err)
		}
		entry.CreatedAt = parsed.UTC()
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate wire publish debounce entries: %w", err)
	}
	return entries, nil
}

func wirePublishDebounceLastDispatch(ctx context.Context, tx *sql.Tx, ownerID, computerID, agentID string) (time.Time, error) {
	var value string
	err := tx.QueryRowContext(ctx, `SELECT last_dispatch_at FROM wire_publish_debounce_state
		WHERE owner_id=? AND computer_id=? AND agent_id=? FOR UPDATE`, ownerID, computerID, agentID).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("load wire publish debounce state: %w", err)
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse wire publish debounce last dispatch: %w", err)
	}
	return parsed.UTC(), nil
}
