package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// runMemoryStripes is the number of run-memory append locks.
const runMemoryStripes = 64

// runMemoryLock is the append lock of one run's memory log.
func (s *Store) runMemoryLock(runID string) *sync.Mutex {
	h := fnv.New32a()
	_, _ = h.Write([]byte(runID))
	return &s.runMemoryMu[h.Sum32()%runMemoryStripes]
}

// AppendRunMemoryEntry appends a durable context entry to a run's ordered
// memory log. Sequence numbers and parent links are assigned transactionally.
func (s *Store) AppendRunMemoryEntry(ctx context.Context, entry types.RunMemoryEntry) (types.RunMemoryEntry, error) {
	mu := s.runMemoryLock(entry.RunID)
	mu.Lock()
	defer mu.Unlock()
	item, err := normalizeRunMemoryEntry(entry)
	if err != nil {
		return types.RunMemoryEntry{}, err
	}
	entry = item.entry
	messageJSON, detailsJSON := item.messageJSON, item.detailsJSON

	if s.projectionTape != nil {
		out, err := s.appendRunMemoryTapeLocked(ctx, entry.RunID, []normalizedRunMemory{item})
		if err != nil {
			return types.RunMemoryEntry{}, err
		}
		return out[0], nil
	}

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
		detailsJSON,
		entry.CreatedAt.UTC(),
	)
	if err != nil {
		return types.RunMemoryEntry{}, fmt.Errorf("insert run memory entry: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return types.RunMemoryEntry{}, fmt.Errorf("commit run memory append: %w", err)
	}
	return entry, nil
}

// normalizedRunMemory is one entry with its defaults filled and its text
// sanitized, and the two JSON columns as they are stored.
type normalizedRunMemory struct {
	entry       types.RunMemoryEntry
	messageJSON string
	detailsJSON string
}

func normalizeRunMemoryEntry(entry types.RunMemoryEntry) (normalizedRunMemory, error) {
	if entry.RunID == "" {
		return normalizedRunMemory{}, fmt.Errorf("append run memory: loop_id is required")
	}
	if entry.Kind == "" {
		return normalizedRunMemory{}, fmt.Errorf("append run memory: kind is required")
	}
	if entry.EntryID == "" {
		entry.EntryID = newRunMemoryEntryID()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}
	if entry.Details == nil {
		entry.Details = map[string]any{}
	}
	detailsJSON, err := marshalJSON(entry.Details)
	if err != nil {
		return normalizedRunMemory{}, fmt.Errorf("marshal run memory details: %w", err)
	}
	messageJSON := ""
	if len(entry.Message) > 0 {
		messageJSON = sanitizeStoreText(string(entry.Message))
	}
	entry.Summary = sanitizeStoreText(entry.Summary)
	entry.FirstKeptEntryID = sanitizeStoreText(entry.FirstKeptEntryID)
	entry.Reason = sanitizeStoreText(entry.Reason)
	entry.Model = sanitizeStoreText(entry.Model)
	return normalizedRunMemory{entry: entry, messageJSON: messageJSON, detailsJSON: sanitizeStoreText(string(detailsJSON))}, nil
}

// AppendRunMemoryEntries appends several entries of one run, in order, as one
// tape write: one acknowledged tape event, durable before it returns. It is
// the batched form of AppendRunMemoryEntry for a
// run's opening and carried-over messages; an empty list does nothing.
func (s *Store) AppendRunMemoryEntries(ctx context.Context, entries []types.RunMemoryEntry) ([]types.RunMemoryEntry, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	runID := entries[0].RunID
	for _, e := range entries {
		if e.RunID != runID {
			return nil, fmt.Errorf("append run memory entries: every entry must belong to run %q", runID)
		}
	}
	if s.projectionTape == nil {
		out := make([]types.RunMemoryEntry, 0, len(entries))
		for _, e := range entries {
			got, err := s.AppendRunMemoryEntry(ctx, e)
			if err != nil {
				return out, err
			}
			out = append(out, got)
		}
		return out, nil
	}
	mu := s.runMemoryLock(runID)
	mu.Lock()
	defer mu.Unlock()
	items := make([]normalizedRunMemory, 0, len(entries))
	for _, e := range entries {
		item, err := normalizeRunMemoryEntry(e)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return s.appendRunMemoryTapeLocked(ctx, runID, items)
}

// appendRunMemoryTapeLocked numbers and writes entries of one run, holding the
// run's append lock. Seq and parent come from the run's last entry in the
// table (every earlier entry is acknowledged, so it is there). All entries go
// in one tape event, and the call returns only after that event's CAS and
// local finalize: an entry the caller was told about is on the tape (owner
// ruling D8, 2026-11-12; the run-memory window of 2026-10-31 is deleted).
// Writers of other runs that wait behind the in-flight append share an event
// with this one (tape_group_commit.go).
//
// Numbering reads the committed table, which can lag the host after an append
// whose outcome was unknown. That is safe by the head invariant: the table and
// the projection head commit in one transaction, the store's dry run reports
// the head it validated against, and the appender compares it with the local
// head under its mutex before anything is sent, validating again (and refusing
// with ErrBatchRefused, nothing sent) if the head moved for any reason. The
// caller sees a refusal as an ordinary failed append; its next attempt numbers
// from the caught-up table. There is no retry inside this function.
func (s *Store) appendRunMemoryTapeLocked(ctx context.Context, runID string, items []normalizedRunMemory) ([]types.RunMemoryEntry, error) {
	var tail runMemoryTail
	haveTail := false
	acked := make([]computerevent.ProjectionOp, 0, len(items))
	out := make([]types.RunMemoryEntry, 0, len(items))
	for _, item := range items {
		entry := item.entry
		if entry.Seq <= 0 || entry.ParentEntryID == "" {
			if !haveTail {
				t, err := s.runMemoryTailLocked(ctx, runID)
				if err != nil {
					return nil, err
				}
				tail, haveTail = t, true
			}
			if entry.Seq <= 0 {
				entry.Seq = tail.seq + 1
			}
			if entry.ParentEntryID == "" {
				entry.ParentEntryID = tail.entryID
			}
		}
		if !haveTail || entry.Seq >= tail.seq {
			tail, haveTail = runMemoryTail{seq: entry.Seq, entryID: entry.EntryID}, true
		}
		proj := computerevent.RunMemoryEntryProjection{
			EntryID: entry.EntryID, RunID: entry.RunID, OwnerID: entry.OwnerID, AgentID: entry.AgentID,
			ParentEntryID: entry.ParentEntryID, Seq: entry.Seq, Kind: string(entry.Kind), Role: entry.Role,
			MessageJSON: item.messageJSON, Summary: entry.Summary, FirstKeptEntryID: entry.FirstKeptEntryID,
			TokensBefore: entry.TokensBefore, Reason: entry.Reason, Model: entry.Model,
			DetailsJSON: item.detailsJSON, CreatedAt: entry.CreatedAt.UTC().Format(time.RFC3339Nano),
		}
		body, err := json.Marshal(proj)
		if err != nil {
			return nil, fmt.Errorf("marshal projected run memory entry: %w", err)
		}
		acked = append(acked, computerevent.ProjectionOp{Kind: computerevent.ProjectionOpRunMemoryEntry, CanonicalID: entry.EntryID, Body: body})
		out = append(out, entry)
	}
	if len(acked) > 0 {
		// Run memory is not telemetry and is not ordered against it: it is the
		// one exemption from the telemetry append barrier (design 4.3). A
		// source-scan test allows exactly three WithBarrierExemption callers.
		if err := s.projectionTape.appendOps(computerevent.WithBarrierExemption(ctx), acked); err != nil {
			return nil, fmt.Errorf("append projected run memory entry: %w", err)
		}
	}
	return out, nil
}

// runMemoryTail is a run's last entry: the seq and parent of the next one.
type runMemoryTail struct {
	seq     int64
	entryID string
}

// runMemoryTailLocked is the run's last entry (seq 0 and no id for a new run).
func (s *Store) runMemoryTailLocked(ctx context.Context, runID string) (runMemoryTail, error) {
	var t runMemoryTail
	if err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(seq), 0) FROM run_memory_entries WHERE loop_id = ?`,
		runID,
	).Scan(&t.seq); err != nil {
		return runMemoryTail{}, fmt.Errorf("allocate projected run memory seq: %w", err)
	}
	err := s.db.QueryRowContext(ctx,
		`SELECT entry_id FROM run_memory_entries WHERE loop_id = ? ORDER BY seq DESC LIMIT 1`,
		runID,
	).Scan(&t.entryID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return runMemoryTail{}, fmt.Errorf("load projected run memory parent: %w", err)
	}
	return t, nil
}

// LatestActorRunMemoryEntries returns the most recent durable memory log for a
// prior inactive activation of the same computer-scoped actor identity.
func (s *Store) LatestActorRunMemoryEntries(ctx context.Context, ownerID, computerID, agentID, excludeRunID string) (string, []types.RunMemoryEntry, error) {
	ownerID = strings.TrimSpace(ownerID)
	computerID = strings.TrimSpace(computerID)
	agentID = strings.TrimSpace(agentID)
	excludeRunID = strings.TrimSpace(excludeRunID)
	if ownerID == "" {
		return "", nil, fmt.Errorf("latest actor run memory: owner_id is required")
	}
	if computerID == "" {
		return "", nil, fmt.Errorf("latest actor run memory: computer_id is required")
	}
	if agentID == "" {
		return "", nil, fmt.Errorf("latest actor run memory: agent_id is required")
	}

	// Exhaust matching agent IDs before owner/state filtering; a fixed
	// pre-filter window can hide an older activation with durable memory.
	objs, err := s.ogListAllByMetadata(ctx, ogKindRun, "agent_id", agentID)
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
		if rec.OwnerID != ownerID || rec.ComputerID != computerID {
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
		// A run whose only rows are the kept cursors (retirement leaves them)
		// holds no conversation to carry: the next older run is the source.
		if runMemoryHasBody(entries) {
			return c.runID, entries, nil
		}
	}
	return "", nil, ErrNotFound
}

// runMemoryHasBody reports whether any entry is not of a kind retirement keeps.
func runMemoryHasBody(entries []types.RunMemoryEntry) bool {
	for _, e := range entries {
		kept := false
		for _, k := range runMemoryRetireKeepKinds {
			if string(e.Kind) == k {
				kept = true
			}
		}
		if !kept {
			return true
		}
	}
	return false
}

// ListRunMemoryEntries returns a run's durable memory log in sequence order.
// If ownerID is non-empty, the query is scoped to that owner.
func (s *Store) ListRunMemoryEntries(ctx context.Context, ownerID, runID string) ([]types.RunMemoryEntry, error) {
	if runID == "" {
		return nil, fmt.Errorf("list run memory: loop_id is required")
	}
	return s.listRunMemoryEntriesTable(ctx, ownerID, runID)
}

func (s *Store) listRunMemoryEntriesTable(ctx context.Context, ownerID, runID string) ([]types.RunMemoryEntry, error) {
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

// runMemoryAgentListQuery is the statement of ListRunMemoryEntriesForAgent.
// The kind index (run_memory_kind_index.go) serves its WHERE with no change to
// the text: the planner reads it when it exists. A test pins the plan.
func runMemoryAgentListQuery(ownerID, agentID string, kinds []string) (string, []any) {
	query := `SELECT entry_id, loop_id, owner_id, agent_id, parent_entry_id, seq,
	        kind, role, message_json, summary, first_kept_entry_id,
	        tokens_before, reason, model, details_json, created_at
	   FROM run_memory_entries
	  WHERE agent_id = ?`
	args := []any{agentID}
	if ownerID != "" {
		query += ` AND owner_id = ?`
		args = append(args, ownerID)
	}
	if len(kinds) > 0 {
		marks := make([]string, len(kinds))
		for i, k := range kinds {
			marks[i] = "?"
			args = append(args, k)
		}
		query += ` AND kind IN (` + strings.Join(marks, ",") + `)`
	}
	query += ` ORDER BY created_at ASC, seq ASC`
	return query, args
}

// ListRunMemoryEntriesForAgent returns run-memory entries for one desk agent
// across all its runs, in agent's seq order per run then run order. The
// durable inbox cursor is keyed (channel, desk): a respawned desk mints a new
// run_id but must resume its channel cursor — the (runID,channel) key can't
// do that. If ownerID is non-empty the query is scoped to that owner; kinds
// restricts to the named entry kinds (empty = all kinds).
func (s *Store) ListRunMemoryEntriesForAgent(ctx context.Context, ownerID, agentID string, kinds []string) ([]types.RunMemoryEntry, error) {
	s.runMemoryAgentList.calls.Add(1)
	if agentID == "" {
		return nil, fmt.Errorf("list run memory for agent: agent_id is required")
	}
	query, args := runMemoryAgentListQuery(ownerID, agentID, kinds)
	obs := runMemoryAgentListRead{site: runMemoryReadSite(ctx), agentID: agentID, kinds: len(kinds)}
	start := time.Now()
	defer func() {
		obs.elapsed = time.Since(start)
		obs.callsSoFar = s.runMemoryAgentList.calls.Load()
		s.runMemoryAgentList.observe(time.Now(), obs)
	}()
	// One pooled connection (configureEmbeddedDoltDB): taking it explicitly
	// separates the wait for it from the hold, for the slow-read line.
	conn, err := s.db.Conn(ctx)
	obs.connWait = time.Since(start)
	if err != nil {
		return nil, fmt.Errorf("query run memory entries for agent: %w", err)
	}
	defer func() { _ = conn.Close() }()
	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query run memory entries for agent: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var entries []types.RunMemoryEntry
	var firstRowAt time.Time
	for rows.Next() {
		if firstRowAt.IsZero() {
			firstRowAt = time.Now()
			obs.firstRow = firstRowAt.Sub(start) - obs.connWait
		}
		entry, err := scanRunMemoryEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if firstRowAt.IsZero() {
		obs.firstRow = time.Since(start) - obs.connWait
	} else {
		obs.iterate = time.Since(firstRowAt)
	}
	obs.rows = len(entries)
	return entries, nil
}

// RunMemoryEntryFingerprint is a content-safe identity and digest view of a
// run-memory row. It deliberately exposes hashes instead of provider-facing
// message, summary, or details text so replay diagnostics cannot become a
// second content-read API.
type RunMemoryEntryFingerprint struct {
	EntryID      string            `json:"entry_id"`
	RunID        string            `json:"loop_id"`
	OwnerID      string            `json:"owner_id"`
	AgentID      string            `json:"agent_id,omitempty"`
	Seq          int64             `json:"seq"`
	CreatedAt    string            `json:"created_at"`
	RowDigest    string            `json:"row_digest"`
	FieldDigests map[string]string `json:"field_digests"`
}

// ListRunMemoryEntryFingerprints returns a deterministic, content-safe digest
// for every run-memory row. It is used only by owner-authorized replay
// diagnostics to distinguish omitted rows from projection normalization.
func (s *Store) ListRunMemoryEntryFingerprints(ctx context.Context) ([]RunMemoryEntryFingerprint, error) {
	return listRunMemoryEntryFingerprints(ctx, s.db)
}

// ListRunMemoryEntryFingerprintsTx is ListRunMemoryEntryFingerprints inside a
// caller's transaction (a read snapshot from ReadSnapshotTx), so the
// fingerprints are of the same committed state as the transaction's other
// reads.
func (s *Store) ListRunMemoryEntryFingerprintsTx(ctx context.Context, tx *sql.Tx) ([]RunMemoryEntryFingerprint, error) {
	if tx == nil {
		return nil, fmt.Errorf("run memory fingerprints: transaction is required")
	}
	return listRunMemoryEntryFingerprints(ctx, tx)
}

func listRunMemoryEntryFingerprints(ctx context.Context, q interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}) ([]RunMemoryEntryFingerprint, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT entry_id, loop_id, owner_id, agent_id, parent_entry_id, seq,
		       kind, role, message_json, summary, first_kept_entry_id,
		       tokens_before, reason, model, details_json, created_at
		  FROM run_memory_entries
		 ORDER BY entry_id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query run memory fingerprints: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var fingerprints []RunMemoryEntryFingerprint
	for rows.Next() {
		var (
			entryID, runID, ownerID, agentID, parentEntryID string
			seq                                             int64
			kind, role, messageJSON, summary                string
			firstKeptEntryID                                string
			tokensBefore                                    int64
			reason, model, detailsJSON, createdAt           string
		)
		if err := rows.Scan(
			&entryID, &runID, &ownerID, &agentID, &parentEntryID, &seq,
			&kind, &role, &messageJSON, &summary, &firstKeptEntryID,
			&tokensBefore, &reason, &model, &detailsJSON, &createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan run memory fingerprint: %w", err)
		}
		fields := map[string]any{
			"entry_id":            entryID,
			"loop_id":             runID,
			"owner_id":            ownerID,
			"agent_id":            agentID,
			"parent_entry_id":     parentEntryID,
			"seq":                 seq,
			"kind":                kind,
			"role":                role,
			"message_json":        messageJSON,
			"summary":             summary,
			"first_kept_entry_id": firstKeptEntryID,
			"tokens_before":       tokensBefore,
			"reason":              reason,
			"model":               model,
			"details_json":        detailsJSON,
			"created_at":          createdAt,
		}
		rowJSON, err := computerevent.CanonicalJSON(fields)
		if err != nil {
			return nil, fmt.Errorf("encode run memory fingerprint: %w", err)
		}
		fieldDigests := make(map[string]string, len(fields))
		for name, value := range fields {
			fieldJSON, err := computerevent.CanonicalJSON(value)
			if err != nil {
				return nil, fmt.Errorf("encode run memory field fingerprint %s: %w", name, err)
			}
			fieldDigests[name] = computerevent.DigestBytes(fieldJSON)
		}
		fingerprints = append(fingerprints, RunMemoryEntryFingerprint{
			EntryID: entryID, RunID: runID, OwnerID: ownerID, AgentID: agentID,
			Seq: seq, CreatedAt: createdAt, RowDigest: computerevent.DigestBytes(rowJSON), FieldDigests: fieldDigests,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate run memory fingerprints: %w", err)
	}
	return fingerprints, nil
}

// LatestRunMemoryEntry returns the most recent memory entry for a run.
func (s *Store) LatestRunMemoryEntry(ctx context.Context, runID string) (types.RunMemoryEntry, error) {
	return s.latestRunMemoryEntryTable(ctx, runID)
}

func (s *Store) latestRunMemoryEntryTable(ctx context.Context, runID string) (types.RunMemoryEntry, error) {
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
