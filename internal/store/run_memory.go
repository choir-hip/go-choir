package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// AppendRunMemoryEntry appends a durable context entry to a run's ordered
// memory log. Sequence numbers and parent links are assigned transactionally.
func (s *Store) AppendRunMemoryEntry(ctx context.Context, entry types.RunMemoryEntry) (types.RunMemoryEntry, error) {
	if entry.RunID == "" {
		return types.RunMemoryEntry{}, fmt.Errorf("append run memory: loop_id is required")
	}
	if entry.Kind == "" {
		return types.RunMemoryEntry{}, fmt.Errorf("append run memory: kind is required")
	}
	if entry.EntryID == "" {
		entry.EntryID = uuid.NewString()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}
	if entry.Details == nil {
		entry.Details = map[string]any{}
	}
	detailsJSON, err := marshalJSON(entry.Details)
	if err != nil {
		return types.RunMemoryEntry{}, fmt.Errorf("marshal run memory details: %w", err)
	}

	messageJSON := ""
	if len(entry.Message) > 0 {
		messageJSON = sanitizeStoreText(string(entry.Message))
	}
	entry.Summary = sanitizeStoreText(entry.Summary)
	entry.FirstKeptEntryID = sanitizeStoreText(entry.FirstKeptEntryID)
	entry.Reason = sanitizeStoreText(entry.Reason)
	entry.Model = sanitizeStoreText(entry.Model)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return types.RunMemoryEntry{}, fmt.Errorf("begin run memory append: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if entry.Seq <= 0 {
		if err = tx.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(seq), 0) + 1
			   FROM run_memory_entries
			  WHERE loop_id = ?`,
			entry.RunID,
		).Scan(&entry.Seq); err != nil {
			return types.RunMemoryEntry{}, fmt.Errorf("allocate run memory seq: %w", err)
		}
	}
	if entry.ParentEntryID == "" {
		var parentID string
		parentErr := tx.QueryRowContext(ctx,
			`SELECT entry_id
			   FROM run_memory_entries
			  WHERE loop_id = ?
			  ORDER BY seq DESC
			  LIMIT 1`,
			entry.RunID,
		).Scan(&parentID)
		if parentErr != nil && !errors.Is(parentErr, sql.ErrNoRows) {
			return types.RunMemoryEntry{}, fmt.Errorf("load run memory parent: %w", parentErr)
		}
		if parentErr == nil {
			entry.ParentEntryID = parentID
		}
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO run_memory_entries (
			entry_id, loop_id, owner_id, agent_id, parent_entry_id, seq, kind,
			role, message_json, summary, first_kept_entry_id, tokens_before,
			reason, model, details_json, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.EntryID,
		entry.RunID,
		entry.OwnerID,
		entry.AgentID,
		entry.ParentEntryID,
		entry.Seq,
		entry.Kind,
		entry.Role,
		messageJSON,
		entry.Summary,
		entry.FirstKeptEntryID,
		entry.TokensBefore,
		entry.Reason,
		entry.Model,
		sanitizeStoreText(string(detailsJSON)),
		entry.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.RunMemoryEntry{}, fmt.Errorf("insert run memory entry: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return types.RunMemoryEntry{}, fmt.Errorf("commit run memory append: %w", err)
	}
	return entry, nil
}

// LatestActorRunMemoryEntries returns the most recent durable memory log for a
// prior inactive activation of the same actor identity.
func (s *Store) LatestActorRunMemoryEntries(ctx context.Context, ownerID, agentID, excludeRunID string) (string, []types.RunMemoryEntry, error) {
	ownerID = strings.TrimSpace(ownerID)
	agentID = strings.TrimSpace(agentID)
	excludeRunID = strings.TrimSpace(excludeRunID)
	if ownerID == "" {
		return "", nil, fmt.Errorf("latest actor run memory: owner_id is required")
	}
	if agentID == "" {
		return "", nil, fmt.Errorf("latest actor run memory: agent_id is required")
	}

	// Find candidate runs from OG, then check for run_memory_entries in SQL.
	// Use a large limit to avoid missing older runs that have memory
	// entries when newer runs don't.
	objs, err := s.ogListByMetadata(ctx, ogKindRun, "agent_id", agentID, 100000)
	if err != nil {
		return "", nil, fmt.Errorf("query latest actor run memory: %w", err)
	}
	type candidate struct {
		runID    string
		priority time.Time
	}
	candidates := make([]candidate, 0, len(objs))
	for _, obj := range objs {
		var rec types.RunRecord
		if err := ogDecode(obj, &rec); err != nil {
			continue
		}
		if rec.OwnerID != ownerID {
			continue
		}
		if rec.RunID == excludeRunID {
			continue
		}
		if rec.State != types.RunCompleted && rec.State != types.RunPassivated {
			continue
		}
		priority := rec.UpdatedAt
		if rec.FinishedAt != nil {
			priority = *rec.FinishedAt
		}
		if priority.IsZero() {
			priority = rec.CreatedAt
		}
		candidates = append(candidates, candidate{runID: rec.RunID, priority: priority})
	}
	// Sort by priority descending.
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].priority.After(candidates[j].priority) })

	// Find the first candidate that has run_memory_entries in SQL.
	for _, c := range candidates {
		entries, err := s.ListRunMemoryEntries(ctx, ownerID, c.runID)
		if err != nil {
			return "", nil, err
		}
		if len(entries) > 0 {
			return c.runID, entries, nil
		}
	}
	return "", nil, ErrNotFound
}

// ListRunMemoryEntries returns a run's durable memory log in sequence order.
// If ownerID is non-empty, the query is scoped to that owner.
func (s *Store) ListRunMemoryEntries(ctx context.Context, ownerID, runID string) ([]types.RunMemoryEntry, error) {
	if runID == "" {
		return nil, fmt.Errorf("list run memory: loop_id is required")
	}
	var (
		rows *sql.Rows
		err  error
	)
	if ownerID == "" {
		rows, err = s.db.QueryContext(ctx,
			`SELECT entry_id, loop_id, owner_id, agent_id, parent_entry_id, seq,
			        kind, role, message_json, summary, first_kept_entry_id,
			        tokens_before, reason, model, details_json, created_at
			   FROM run_memory_entries
			  WHERE loop_id = ?
			  ORDER BY seq ASC`,
			runID,
		)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT entry_id, loop_id, owner_id, agent_id, parent_entry_id, seq,
			        kind, role, message_json, summary, first_kept_entry_id,
			        tokens_before, reason, model, details_json, created_at
			   FROM run_memory_entries
			  WHERE owner_id = ? AND loop_id = ?
			  ORDER BY seq ASC`,
			ownerID,
			runID,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("query run memory entries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var entries []types.RunMemoryEntry
	for rows.Next() {
		entry, err := scanRunMemoryEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate run memory entries: %w", err)
	}
	return entries, nil
}

// LatestRunMemoryEntry returns the most recent memory entry for a run.
func (s *Store) LatestRunMemoryEntry(ctx context.Context, runID string) (types.RunMemoryEntry, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT entry_id, loop_id, owner_id, agent_id, parent_entry_id, seq,
		        kind, role, message_json, summary, first_kept_entry_id,
		        tokens_before, reason, model, details_json, created_at
		   FROM run_memory_entries
		  WHERE loop_id = ?
		  ORDER BY seq DESC
		  LIMIT 1`,
		runID,
	)
	return scanRunMemoryEntry(row)
}

// GetRunMemoryEntry returns a single durable memory entry by id, scoped to the
// owner when ownerID is non-empty.
func (s *Store) GetRunMemoryEntry(ctx context.Context, ownerID, entryID string) (types.RunMemoryEntry, error) {
	if entryID == "" {
		return types.RunMemoryEntry{}, fmt.Errorf("get run memory: entry_id is required")
	}
	if ownerID == "" {
		row := s.db.QueryRowContext(ctx,
			`SELECT entry_id, loop_id, owner_id, agent_id, parent_entry_id, seq,
			        kind, role, message_json, summary, first_kept_entry_id,
			        tokens_before, reason, model, details_json, created_at
			   FROM run_memory_entries
			  WHERE entry_id = ?`,
			entryID,
		)
		return scanRunMemoryEntry(row)
	}
	row := s.db.QueryRowContext(ctx,
		`SELECT entry_id, loop_id, owner_id, agent_id, parent_entry_id, seq,
		        kind, role, message_json, summary, first_kept_entry_id,
		        tokens_before, reason, model, details_json, created_at
		   FROM run_memory_entries
		  WHERE owner_id = ? AND entry_id = ?`,
		ownerID,
		entryID,
	)
	return scanRunMemoryEntry(row)
}

func scanRunMemoryEntry(row interface{ Scan(...any) error }) (types.RunMemoryEntry, error) {
	var entry types.RunMemoryEntry
	var messageJSON, detailsJSON, createdAt string
	err := row.Scan(
		&entry.EntryID,
		&entry.RunID,
		&entry.OwnerID,
		&entry.AgentID,
		&entry.ParentEntryID,
		&entry.Seq,
		&entry.Kind,
		&entry.Role,
		&messageJSON,
		&entry.Summary,
		&entry.FirstKeptEntryID,
		&entry.TokensBefore,
		&entry.Reason,
		&entry.Model,
		&detailsJSON,
		&createdAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.RunMemoryEntry{}, ErrNotFound
		}
		return types.RunMemoryEntry{}, fmt.Errorf("scan run memory entry: %w", err)
	}
	if messageJSON != "" {
		entry.Message = json.RawMessage(messageJSON)
	}
	if detailsJSON != "" && detailsJSON != "{}" {
		if err := json.Unmarshal([]byte(detailsJSON), &entry.Details); err != nil {
			return types.RunMemoryEntry{}, fmt.Errorf("decode run memory details: %w", err)
		}
	}
	if entry.Details == nil {
		entry.Details = map[string]any{}
	}
	entry.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.RunMemoryEntry{}, fmt.Errorf("parse run memory created_at: %w", err)
	}
	return entry, nil
}
