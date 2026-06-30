// Package trace provides the primary observability store for go-choir: a
// versioned, queryable persistence layer for trace events backed by Dolt.
//
// Trace events are the canonical observation surface for the system. Every
// tool call, LLM I/O exchange, and agent-to-agent message is projected into a
// trace event and persisted here. The Dolt-backed store is the primary
// observability path — no SaaS log export is required. Supervision layers and
// the self-learning layer read trace events as structured observations from
// this store.
//
// The store is backend-agnostic at the SQL layer: it accepts any *sql.DB that
// supports the trace_events schema. Two constructors are provided:
//
//   - NewDoltStore wraps an existing embedded Dolt *sql.DB (the same workspace
//     that owns runtime/Texture state, or a dedicated observability workspace).
//   - NewSQLiteStore opens a modernc.org/sqlite database for local testing and
//     development without a Dolt workspace.
//
// Existing trace event recording in internal/runtime and internal/maild is
// unchanged. This package adds persistence alongside it; callers project
// types.EventRecord into trace.Event via FromEventRecord and Append it here.
package trace

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"

	_ "modernc.org/sqlite" // registers the "sqlite" driver for the in-memory test backend
)

// ErrNotFound is returned when a trace event is not present in the store.
var ErrNotFound = errors.New("trace event not found")

// traceSchema is the Dolt table schema for trace event persistence. It is
// written with separate CREATE INDEX statements so the same DDL applies to both
// Dolt and SQLite backends (SQLite does not accept inline INDEX clauses inside
// CREATE TABLE).
//
// payload is stored as LONGTEXT containing JSON text. Dolt supports a native
// JSON type, but LONGTEXT keeps the schema portable across the in-memory
// SQLite test backend and matches the existing runtime events table convention
// (payload_json LONGTEXT).
const traceSchema = `
CREATE TABLE IF NOT EXISTS trace_events (
	id            TEXT PRIMARY KEY,
	run_id        TEXT NOT NULL DEFAULT '',
	parent_id     TEXT NOT NULL DEFAULT '',
	event_type    TEXT NOT NULL,
	actor         TEXT NOT NULL DEFAULT '',
	tool          TEXT NOT NULL DEFAULT '',
	owner_id      TEXT NOT NULL DEFAULT '',
	trajectory_id TEXT NOT NULL DEFAULT '',
	seq           INTEGER NOT NULL DEFAULT 0,
	stream_seq    INTEGER NOT NULL DEFAULT 0,
	payload       LONGTEXT NOT NULL DEFAULT '{}',
	created_at    DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_trace_events_run_id     ON trace_events(run_id);
CREATE INDEX IF NOT EXISTS idx_trace_events_created_at ON trace_events(created_at);
CREATE INDEX IF NOT EXISTS idx_trace_events_event_type ON trace_events(event_type);
CREATE INDEX IF NOT EXISTS idx_trace_events_owner      ON trace_events(owner_id);
CREATE INDEX IF NOT EXISTS idx_trace_events_trajectory ON trace_events(trajectory_id, created_at);
`

// Event is a single trace event persisted to the observability store. It is a
// projection of types.EventRecord into the canonical observation schema: a
// causally-linked, typed record with an actor, optional tool, and JSON payload.
type Event struct {
	// ID is the unique identifier for this trace event (matches EventID).
	ID string `json:"id"`

	// RunID is the run this event is correlated to. Empty for runtime-level
	// events (health, degraded).
	RunID string `json:"run_id"`

	// ParentID is the optional parent trace event id, supporting causal chains.
	// Populated from payload["parent_event_id"] when present; forward-compatible
	// for richer lineage.
	ParentID string `json:"parent_id,omitempty"`

	// EventType is the event kind (e.g. "tool.invoked", "loop.completed",
	// "channel.message").
	EventType string `json:"event_type"`

	// Actor is the durable agent identity that emitted or owns this event.
	Actor string `json:"actor,omitempty"`

	// Tool is the tool name for tool.invoked/tool.result events, extracted from
	// the payload. Empty for non-tool events.
	Tool string `json:"tool,omitempty"`

	// OwnerID is the authenticated user who owns the run, used for owner-scoped
	// query access.
	OwnerID string `json:"owner_id,omitempty"`

	// TrajectoryID ties the event to the broader user-visible workflow.
	TrajectoryID string `json:"trajectory_id,omitempty"`

	// Seq is the per-run sequence number.
	Seq int64 `json:"seq,omitempty"`

	// StreamSeq is the owner/global monotonic sequence for cross-run catch-up.
	StreamSeq int64 `json:"stream_seq,omitempty"`

	// Payload carries the event-specific data as a JSON blob.
	Payload json.RawMessage `json:"payload"`

	// CreatedAt is when the event occurred.
	CreatedAt time.Time `json:"created_at"`
}

// FromEventRecord projects a runtime types.EventRecord into the canonical trace
// observability Event. It extracts the tool name (for tool events) and
// parent_event_id (for causal chaining) from the payload without mutating the
// source record. Existing trace event recording is unchanged.
func FromEventRecord(rec *types.EventRecord) Event {
	tool, parentID := extractTracePayloadMeta(rec.Payload)
	if strings.TrimSpace(parentID) == "" {
		parentID = ""
	}
	payload := rec.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	return Event{
		ID:           rec.EventID,
		RunID:        rec.RunID,
		ParentID:     parentID,
		EventType:    string(rec.Kind),
		Actor:        rec.AgentID,
		Tool:         tool,
		OwnerID:      rec.OwnerID,
		TrajectoryID: rec.TrajectoryID,
		Seq:          rec.Seq,
		StreamSeq:    rec.StreamSeq,
		Payload:      payload,
		CreatedAt:    rec.Timestamp.UTC(),
	}
}

// extractTracePayloadMeta reads the tool name and parent_event_id from a trace
// event payload without failing on malformed JSON. Returns empty strings when
// the keys are absent or the payload is not a JSON object.
func extractTracePayloadMeta(payload json.RawMessage) (tool, parentID string) {
	if len(payload) == 0 {
		return "", ""
	}
	var obj map[string]any
	if err := json.Unmarshal(payload, &obj); err != nil {
		return "", ""
	}
	if v, ok := obj["tool"].(string); ok {
		tool = strings.TrimSpace(v)
	}
	if v, ok := obj["tool_name"].(string); ok && tool == "" {
		tool = strings.TrimSpace(v)
	}
	if v, ok := obj["parent_event_id"].(string); ok {
		parentID = strings.TrimSpace(v)
	}
	return tool, parentID
}

// Store is the interface for trace event persistence and query. The Dolt-backed
// implementation is the primary observability path; the in-memory SQLite
// implementation is used for tests and local development.
type Store interface {
	// Append persists a trace event. The ID must be non-empty and unique.
	Append(ctx context.Context, e *Event) error

	// Get returns a single trace event by id. Returns ErrNotFound when absent.
	Get(ctx context.Context, id string) (*Event, error)

	// GetForOwner returns a single trace event owned by ownerID.
	GetForOwner(ctx context.Context, ownerID, id string) (*Event, error)

	// ListByRun returns events for the given run, ordered by seq ascending.
	ListByRun(ctx context.Context, runID string, limit int) ([]Event, error)

	// ListByRunForOwner returns events for the given owner and run, ordered by
	// seq ascending.
	ListByRunForOwner(ctx context.Context, ownerID, runID string, limit int) ([]Event, error)

	// ListByOwner returns events for the given owner, ordered by created_at
	// descending.
	ListByOwner(ctx context.Context, ownerID string, limit int) ([]Event, error)

	// ListByTrajectory returns events for a trajectory, ordered by created_at
	// ascending.
	ListByTrajectory(ctx context.Context, ownerID, trajectoryID string, limit int) ([]Event, error)

	// Close releases the underlying database handle when owned by this store.
	Close() error
}

// SQLStore is a SQL-backed trace event store. It works with any *sql.DB that
// supports the trace_events schema (Dolt or SQLite).
type SQLStore struct {
	db     *sql.DB
	ownsDB bool
}

// NewDoltStore wraps an existing embedded Dolt *sql.DB as the trace observability
// store. The caller retains ownership of the connection; Close is a no-op.
func NewDoltStore(db *sql.DB) (*SQLStore, error) {
	if db == nil {
		return nil, fmt.Errorf("trace store: nil db")
	}
	s := &SQLStore{db: db, ownsDB: false}
	if err := s.applySchema(); err != nil {
		return nil, err
	}
	return s, nil
}

// NewSQLiteStore opens a modernc.org/sqlite database at path (":memory:" for an
// in-memory store) as a trace event store for testing and local development.
// The store owns the connection and Close releases it.
func NewSQLiteStore(path string) (*SQLStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("trace store: sqlite path is required")
	}
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("trace store: create dir: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path+"?_busy_timeout=60000")
	if err != nil {
		return nil, fmt.Errorf("trace store: open sqlite: %w", err)
	}
	s := &SQLStore{db: db, ownsDB: true}
	if err := s.applySchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *SQLStore) applySchema() error {
	if _, err := s.db.Exec(traceSchema); err != nil {
		return fmt.Errorf("trace store: apply schema: %w", err)
	}
	return nil
}

// Close releases the underlying database handle when this store owns it.
func (s *SQLStore) Close() error {
	if !s.ownsDB {
		return nil
	}
	return s.db.Close()
}

// Append persists a trace event. The ID must be non-empty.
func (s *SQLStore) Append(ctx context.Context, e *Event) error {
	if e == nil {
		return fmt.Errorf("trace store: nil event")
	}
	if strings.TrimSpace(e.ID) == "" {
		return fmt.Errorf("trace store: event id is required")
	}
	if strings.TrimSpace(e.EventType) == "" {
		return fmt.Errorf("trace store: event_type is required")
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	payload := e.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO trace_events
		   (id, run_id, parent_id, event_type, actor, tool, owner_id, trajectory_id, seq, stream_seq, payload, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID,
		e.RunID,
		e.ParentID,
		e.EventType,
		e.Actor,
		e.Tool,
		e.OwnerID,
		e.TrajectoryID,
		e.Seq,
		e.StreamSeq,
		string(payload),
		e.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("trace store: insert event: %w", err)
	}
	return nil
}

// Get returns a single trace event by id.
func (s *SQLStore) Get(ctx context.Context, id string) (*Event, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("trace store: id is required")
	}
	row := s.db.QueryRowContext(ctx,
		`SELECT id, run_id, parent_id, event_type, actor, tool, owner_id, trajectory_id, seq, stream_seq, payload, created_at
		   FROM trace_events
		  WHERE id = ?`,
		id,
	)
	ev, err := scanTraceEvent(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("trace store: get event: %w", err)
	}
	return ev, nil
}

// GetForOwner returns a single trace event by id and owner.
func (s *SQLStore) GetForOwner(ctx context.Context, ownerID, id string) (*Event, error) {
	if strings.TrimSpace(ownerID) == "" {
		return nil, fmt.Errorf("trace store: owner_id is required")
	}
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("trace store: id is required")
	}
	row := s.db.QueryRowContext(ctx,
		`SELECT id, run_id, parent_id, event_type, actor, tool, owner_id, trajectory_id, seq, stream_seq, payload, created_at
		   FROM trace_events
		  WHERE owner_id = ? AND id = ?`,
		ownerID,
		id,
	)
	ev, err := scanTraceEvent(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("trace store: get owner event: %w", err)
	}
	return ev, nil
}

// ListByRun returns events for the given run, ordered by seq ascending.
func (s *SQLStore) ListByRun(ctx context.Context, runID string, limit int) ([]Event, error) {
	if strings.TrimSpace(runID) == "" {
		return nil, fmt.Errorf("trace store: run_id is required")
	}
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, run_id, parent_id, event_type, actor, tool, owner_id, trajectory_id, seq, stream_seq, payload, created_at
		   FROM trace_events
		  WHERE run_id = ?
		  ORDER BY seq ASC, created_at ASC
		  LIMIT ?`,
		runID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("trace store: query by run: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return collectTraceEvents(rows)
}

// ListByRunForOwner returns events for the given owner and run, ordered by seq ascending.
func (s *SQLStore) ListByRunForOwner(ctx context.Context, ownerID, runID string, limit int) ([]Event, error) {
	if strings.TrimSpace(ownerID) == "" {
		return nil, fmt.Errorf("trace store: owner_id is required")
	}
	if strings.TrimSpace(runID) == "" {
		return nil, fmt.Errorf("trace store: run_id is required")
	}
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, run_id, parent_id, event_type, actor, tool, owner_id, trajectory_id, seq, stream_seq, payload, created_at
		   FROM trace_events
		  WHERE owner_id = ? AND run_id = ?
		  ORDER BY seq ASC, created_at ASC
		  LIMIT ?`,
		ownerID,
		runID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("trace store: query by owner run: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return collectTraceEvents(rows)
}

// ListByOwner returns events for the given owner, ordered by created_at
// descending.
func (s *SQLStore) ListByOwner(ctx context.Context, ownerID string, limit int) ([]Event, error) {
	if strings.TrimSpace(ownerID) == "" {
		return nil, fmt.Errorf("trace store: owner_id is required")
	}
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, run_id, parent_id, event_type, actor, tool, owner_id, trajectory_id, seq, stream_seq, payload, created_at
		   FROM trace_events
		  WHERE owner_id = ?
		  ORDER BY created_at DESC
		  LIMIT ?`,
		ownerID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("trace store: query by owner: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return collectTraceEvents(rows)
}

// ListByTrajectory returns events for a trajectory, ordered by created_at
// ascending.
func (s *SQLStore) ListByTrajectory(ctx context.Context, ownerID, trajectoryID string, limit int) ([]Event, error) {
	if strings.TrimSpace(trajectoryID) == "" {
		return nil, fmt.Errorf("trace store: trajectory_id is required")
	}
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, run_id, parent_id, event_type, actor, tool, owner_id, trajectory_id, seq, stream_seq, payload, created_at
		   FROM trace_events
		  WHERE owner_id = ? AND trajectory_id = ?
		  ORDER BY created_at ASC
		  LIMIT ?`,
		ownerID,
		trajectoryID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("trace store: query by trajectory: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return collectTraceEvents(rows)
}

// scanner abstracts *sql.Row and *sql.Rows for the scanTraceEvent helper.
type scanner interface {
	Scan(dest ...any) error
}

func scanTraceEvent(sc scanner) (*Event, error) {
	var ev Event
	var payload string
	var createdAt string
	if err := sc.Scan(
		&ev.ID,
		&ev.RunID,
		&ev.ParentID,
		&ev.EventType,
		&ev.Actor,
		&ev.Tool,
		&ev.OwnerID,
		&ev.TrajectoryID,
		&ev.Seq,
		&ev.StreamSeq,
		&payload,
		&createdAt,
	); err != nil {
		return nil, err
	}
	ev.Payload = json.RawMessage(payload)
	if len(ev.Payload) == 0 {
		ev.Payload = json.RawMessage(`{}`)
	}
	ts, err := parseTraceTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("trace store: parse created_at %q: %w", createdAt, err)
	}
	ev.CreatedAt = ts.UTC()
	return &ev, nil
}

func collectTraceEvents(rows *sql.Rows) ([]Event, error) {
	var events []Event
	for rows.Next() {
		ev, err := scanTraceEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("trace store: scan event: %w", err)
		}
		events = append(events, *ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("trace store: iterate events: %w", err)
	}
	return events, nil
}

// parseTraceTime parses a timestamp stored as RFC3339Nano or a SQLite/Dolt
// datetime string. SQLite may return "YYYY-MM-DD HH:MM:SS" forms.
func parseTraceTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}
	if ts, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return ts, nil
	}
	if ts, err := time.Parse(time.RFC3339, raw); err == nil {
		return ts, nil
	}
	if ts, err := time.Parse("2006-01-02 15:04:05", raw); err == nil {
		return ts, nil
	}
	if ts, err := time.Parse("2006-01-02 15:04:05.999999999", raw); err == nil {
		return ts, nil
	}
	if ts, err := time.Parse("2006-01-02T15:04:05", raw); err == nil {
		return ts, nil
	}
	return time.Time{}, fmt.Errorf("unrecognized timestamp format")
}
