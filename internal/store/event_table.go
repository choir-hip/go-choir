package store

// Run telemetry in its own table (F-c), deploy 1:
// docs/design/choir-event-table-f-c-2026-10-22.md.
//
// choir_events holds the choir.event objects whose projection op the writer
// stamped with Table "choir_events" (section 2.2). The stamp is on the tape,
// so the tape decides which table an event lands in, and replay is a
// function of the tape alone. Deploy 1 adds the table, the projector route
// and the union readers; nothing stamps an op until deploy 2's writer, behind
// the owner setting store.event_table. Existing og_objects rows are never
// touched (section 4.3): readers read the union.
//
// Rollback: deploy 1 is the floor once any op has been stamped. An older
// binary ignores the stamp and projects a stamped op into og_objects, its
// seeds read og_objects only, and its replay manifest calls this table
// unknown (section 4.5). Hence the store schema epoch 2 (storeschema).

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/storeschema"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// choirEventsTable is the stamp value and the table's name.
const choirEventsTable = "choir_events"

// choirEventsDDL creates the table empty at boot, with its indexes inline.
// Nothing at boot reads or fills it, and nothing scales with og_objects
// (the og_key_index precedent). The DDL is frozen at deploy 1: CREATE TABLE
// IF NOT EXISTS never alters a table that exists, and any later column or
// index changes the table's SHOW COLUMNS, and so every witness taken with it
// (section 2.3). Every column is a pure function of the op and its tape
// position: the projector stamps no time, so no column is ignored by the
// content witness.
//
// The primary key is the tape position: (sequence of the
// projection_batch_recorded event, the op's index in its batch). It is
// unique by construction; stream_seq is not (section 2.4).
const choirEventsDDL = `
CREATE TABLE IF NOT EXISTS choir_events (
	tape_seq      BIGINT UNSIGNED NOT NULL,
	op_index      INT UNSIGNED    NOT NULL,
	canonical_id  VARCHAR(255)    NOT NULL,
	object_kind   VARCHAR(128)    NOT NULL,
	owner_id      VARCHAR(255)    NOT NULL,
	computer_id   VARCHAR(255)    NOT NULL DEFAULT '',
	version_id    VARCHAR(255)    NOT NULL DEFAULT '',
	content_hash  VARCHAR(128)    NOT NULL,
	body          LONGBLOB        NOT NULL,
	metadata      LONGTEXT        NOT NULL,
	created_at    DATETIME        NOT NULL,
	updated_at    DATETIME        NOT NULL,
	tombstone     BOOLEAN         NOT NULL DEFAULT FALSE,
	superseded_by VARCHAR(255)    NOT NULL DEFAULT '',
	run_id        VARCHAR(255)    NOT NULL DEFAULT '',
	seq           BIGINT          NOT NULL DEFAULT 0,
	stream_seq    BIGINT          NOT NULL DEFAULT 0,
	trajectory_id VARCHAR(255)    NOT NULL DEFAULT '',
	event_kind    VARCHAR(128)    NOT NULL DEFAULT '',
	PRIMARY KEY (tape_seq, op_index),
	INDEX idx_choir_events_run_seq (run_id, seq),
	INDEX idx_choir_events_owner_stream (owner_id, stream_seq),
	INDEX idx_choir_events_trajectory_stream (trajectory_id, stream_seq)
);
`

// projectChoirEvent writes one stamped object op into choir_events at tape
// position (pos, opIndex). It refuses only shapes the table cannot hold, and
// never refuses on vocabulary a later binary may extend, such as the event
// kind: a later binary may add a kind to the storage class, and an earlier
// binary that replays its tape must not refuse it (replay poison, section
// 2.2). A refusal is ErrProjectionBatchInvalid, which the dry run turns into
// a refused batch before the event is canonical.
//
// The write is an upsert on the tape position, so re-projecting the same
// event (finalize after RecoverPrepared) is idempotent.
func projectChoirEvent(ctx context.Context, tx *sql.Tx, body json.RawMessage, pos uint64, opIndex int) error {
	refuse := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s: %s", computerevent.ErrProjectionBatchInvalid, choirEventsTable, fmt.Sprintf(format, args...))
	}
	if pos == 0 || opIndex < 0 {
		return refuse("a stamped op needs a tape position")
	}
	var obj objectgraph.Object
	if err := json.Unmarshal(body, &obj); err != nil {
		return refuse("decode object: %v", err)
	}
	if obj.ObjectKind != ogKindEvent {
		return refuse("object kind %q is not %q", obj.ObjectKind, ogKindEvent)
	}
	if strings.TrimSpace(obj.CanonicalID) == "" {
		return refuse("object identity is required")
	}
	var rec types.EventRecord
	if err := json.Unmarshal(obj.Body, &rec); err != nil {
		return refuse("body is not an event record: %v", err)
	}
	if obj.Tombstone || obj.SupersededBy != "" {
		// A row here is an immutable fact keyed by tape position; a later op
		// cannot reach it by canonical id.
		return refuse("a tombstone or superseded event cannot be stored")
	}
	if obj.CreatedAt.IsZero() || obj.UpdatedAt.IsZero() {
		return refuse("created_at and updated_at are required")
	}
	metadata := string(obj.Metadata)
	if strings.TrimSpace(metadata) == "" {
		metadata = "{}"
	}
	var meta struct {
		RunID        string `json:"run_id"`
		Seq          int64  `json:"seq"`
		StreamSeq    int64  `json:"stream_seq"`
		TrajectoryID string `json:"trajectory_id"`
		Kind         string `json:"kind"`
	}
	if err := json.Unmarshal([]byte(metadata), &meta); err != nil {
		return refuse("metadata: %v", err)
	}
	for _, col := range []struct {
		name, value string
		max         int
	}{
		{"canonical_id", obj.CanonicalID, 255},
		{"object_kind", string(obj.ObjectKind), 128},
		{"owner_id", obj.OwnerID, 255},
		{"computer_id", obj.ComputerID, 255},
		{"version_id", obj.VersionID, 255},
		{"content_hash", obj.ContentHash, 128},
		{"run_id", meta.RunID, 255},
		{"trajectory_id", meta.TrajectoryID, 255},
		{"event_kind", meta.Kind, 128},
	} {
		if n := utf8.RuneCountInString(col.value); n > col.max {
			return refuse("%s is %d characters, the column holds %d", col.name, n, col.max)
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO choir_events
		(tape_seq, op_index, canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata,
		 created_at, updated_at, tombstone, superseded_by, run_id, seq, stream_seq, trajectory_id, event_kind)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, FALSE, '', ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			canonical_id = VALUES(canonical_id),
			object_kind = VALUES(object_kind),
			owner_id = VALUES(owner_id),
			computer_id = VALUES(computer_id),
			version_id = VALUES(version_id),
			content_hash = VALUES(content_hash),
			body = VALUES(body),
			metadata = VALUES(metadata),
			created_at = VALUES(created_at),
			updated_at = VALUES(updated_at),
			run_id = VALUES(run_id),
			seq = VALUES(seq),
			stream_seq = VALUES(stream_seq),
			trajectory_id = VALUES(trajectory_id),
			event_kind = VALUES(event_kind)`,
		pos, opIndex, obj.CanonicalID, string(obj.ObjectKind), obj.OwnerID, obj.ComputerID, obj.VersionID, obj.ContentHash,
		[]byte(obj.Body), metadata, obj.CreatedAt.UTC(), obj.UpdatedAt.UTC(),
		meta.RunID, meta.Seq, meta.StreamSeq, meta.TrajectoryID, meta.Kind); err != nil {
		return fmt.Errorf("project %s: %w", choirEventsTable, err)
	}
	return nil
}

// rowQuerier is the one method the dry-run position read needs, so a
// transaction and a pool both serve.
type rowQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// dryRunTapePosition is the position the next finalized event of computerID
// will have: the projection head's sequence plus one. The appender holds its
// mutex across the dry run, pin, prepare, CAS and finalize, so this is the
// event's real position, and the dry run writes the same rows in the same
// tree as finalize (the journal deduplicates them). A store with no head row
// was never projected: position 1, the rule finalize applies to a missing
// head (section 2.4). If the store's head lagged, the dry run would only
// lose its deduplication: it rolls back, and the upsert never fails on a key.
func dryRunTapePosition(ctx context.Context, q rowQuerier, computerID string) (uint64, error) {
	var sequence uint64
	err := q.QueryRowContext(ctx, `SELECT sequence FROM computer_event_projection_heads WHERE computer_id=?`, computerID).Scan(&sequence)
	if errors.Is(err, sql.ErrNoRows) {
		return 1, nil
	}
	if err != nil {
		return 0, fmt.Errorf("computer event projection: dry-run position: %w", err)
	}
	return sequence + 1, nil
}

// The choir_events half of each union read (section 4.4), one query per
// reader shape, each served by one index (F16 pins the plans). Each ORDER BY
// names the whole index prefix, the equality column included: Dolt reads the
// index in order, with no TopN, only then (measured on the plan test).

func eventTableRunAfterSQL(runID string, afterSeq int64, limit int) (string, []any) {
	return `SELECT canonical_id, updated_at, body FROM choir_events WHERE run_id = ? AND seq > ? ORDER BY run_id ASC, seq ASC LIMIT ?`,
		[]any{runID, afterSeq, limit}
}

func eventTableOwnerAfterSQL(ownerID string, afterStreamSeq int64, limit int) (string, []any) {
	return `SELECT canonical_id, updated_at, body FROM choir_events WHERE owner_id = ? AND stream_seq > ? ORDER BY owner_id ASC, stream_seq ASC LIMIT ?`,
		[]any{ownerID, afterStreamSeq, limit}
}

func eventTableOwnerNewestSQL(ownerID string, limit int) (string, []any) {
	return `SELECT canonical_id, updated_at, body FROM choir_events WHERE owner_id = ? ORDER BY owner_id DESC, stream_seq DESC LIMIT ?`,
		[]any{ownerID, limit}
}

func eventTableTrajectoryNewestSQL(trajectoryID string, limit int) (string, []any) {
	return `SELECT canonical_id, updated_at, body FROM choir_events WHERE trajectory_id = ? ORDER BY trajectory_id DESC, stream_seq DESC LIMIT ?`,
		[]any{trajectoryID, limit}
}

// eventTableRunMaxSeqSQL is the per-run seed's half: one reverse seek on
// (run_id, seq). It reads no row for a run with none, which the seed takes
// as 0 (round 2, Gemini: MAX over no rows is NULL). Not MAX(seq): Dolt plans
// that as a GroupBy over every row of the run.
func eventTableRunMaxSeqSQL(runID string) (string, []any) {
	return `SELECT seq FROM choir_events WHERE run_id = ? ORDER BY run_id DESC, seq DESC LIMIT 1`, []any{runID}
}

// eventTableNewestStreamSQL is the stream seed's half: the newest 500 rows
// by tape position, today's newest-500 heuristic over allocation order.
func eventTableNewestStreamSQL() (string, []any) {
	return `SELECT stream_seq FROM choir_events ORDER BY tape_seq DESC, op_index DESC LIMIT 500`, nil
}

// The exact stream_seq seed's table half: the maximum over every owner by
// per-owner reverse seeks on (owner_id, stream_seq), owners listed by
// skip-seeks on the same index. No index led by stream_seq exists (the DDL is
// frozen, section 2.3), and a MAX() over the column plans as a GroupBy over
// every row; the plan test pins that these two statements are served by
// idx_choir_events_owner_stream with no sort.

// eventTableNextOwnerSQL is the next owner after afterOwner, in owner order.
func eventTableNextOwnerSQL(afterOwner string) (string, []any) {
	return `SELECT owner_id FROM choir_events WHERE owner_id > ? ORDER BY owner_id ASC LIMIT 1`, []any{afterOwner}
}

// eventTableOwnerMaxStreamSQL is one owner's greatest stream_seq: one reverse
// seek. The ORDER BY names the whole index prefix, the equality column
// included, as the other readers do.
func eventTableOwnerMaxStreamSQL(ownerID string) (string, []any) {
	return `SELECT stream_seq FROM choir_events WHERE owner_id = ? ORDER BY owner_id DESC, stream_seq DESC LIMIT 1`, []any{ownerID}
}

// eventTableEntry is one event of a union read, from either table.
type eventTableEntry struct {
	canonicalID string
	updatedAt   time.Time
	rec         types.EventRecord
}

// eventTableDB runs fn on the read pool under the shared engine mutex
// (section 4.4, Q8).
func (s *Store) eventTableDB(op string, fn func(db *sql.DB) error) error {
	store := s.ogReadStore
	if store == nil {
		store = s.ogStore
	}
	if store == nil {
		return fmt.Errorf("store: object graph not initialized")
	}
	return store.WithEngine(op, fn)
}

// readEventTable runs one choir_events half. It scans the rows' bytes
// under the engine mutex and decodes them after releasing it: a 10,000-row
// read must not hold every object-graph query for its JSON decode (review
// findings B and F on deploy 1; Sonnet measured 228 of 281 ms of a 4,000-row
// hold as decode).
func (s *Store) readEventTable(ctx context.Context, op, query string, args []any) ([]eventTableEntry, error) {
	type rawRow struct {
		canonicalID string
		updatedAt   time.Time
		body        []byte
	}
	var raw []rawRow
	err := s.eventTableDB(op, func(db *sql.DB) error {
		rows, err := db.QueryContext(ctx, query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var r rawRow
			if err := rows.Scan(&r.canonicalID, &r.updatedAt, &r.body); err != nil {
				return err
			}
			raw = append(raw, r)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("query %s (%s): %w", choirEventsTable, op, err)
	}
	if s.eventTableDecodeHook != nil {
		s.eventTableDecodeHook(op)
	}
	out := make([]eventTableEntry, 0, len(raw))
	for _, r := range raw {
		e := eventTableEntry{canonicalID: r.canonicalID, updatedAt: r.updatedAt.UTC()}
		if err := json.Unmarshal(r.body, &e.rec); err != nil {
			return nil, fmt.Errorf("store: unmarshal %s body: %w", choirEventsTable, err)
		}
		out = append(out, e)
	}
	return out, nil
}

// cursorTableHalf closes the window between a cursor read's two statements
// (review finding G on deploy 1). The read runs the choir_events half
// (first), then the og_objects half, then the choir_events half again
// (again). An og_objects event committed after the og_objects read is not
// returned; a choir_events event committed after it, in the same window, is
// cut too: from the second read only rows whose cursor key is at most the
// highest key the first two reads saw are kept. So the caller's cursor never
// moves past an event committed before one it returns; the next read
// returns both. Rows are immutable and keyed by tape position, so the
// second read contains the first. What remains is the store's existing
// window: numbers are allocated under eventMu and committed after it, so an
// event can commit after one allocated later (a gap a single table has
// always had).
func cursorTableHalf(first, og, again []eventTableEntry, key func(types.EventRecord) int64, after int64) []eventTableEntry {
	bound := after
	for _, half := range [][]eventTableEntry{first, og} {
		for _, e := range half {
			bound = max(bound, key(e.rec))
		}
	}
	kept := make([]eventTableEntry, 0, len(first)+len(again))
	kept = append(kept, first...)
	for _, e := range again {
		if key(e.rec) <= bound {
			kept = append(kept, e)
		}
	}
	return unionEvents(nil, kept)
}

// objectEventEntries decodes the og_objects half.
func objectEventEntries(objs []objectgraph.Object) ([]eventTableEntry, error) {
	out := make([]eventTableEntry, 0, len(objs))
	for _, obj := range objs {
		var rec types.EventRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		out = append(out, eventTableEntry{canonicalID: obj.CanonicalID, updatedAt: obj.UpdatedAt.UTC(), rec: rec})
	}
	return out, nil
}

// unionEvents merges the two halves, deduplicated by canonical id before any
// cut. One op lands in one table, so a duplicate is a bug or a store an old
// binary touched (section 4.5); the og_objects copy is kept.
func unionEvents(og, table []eventTableEntry) []eventTableEntry {
	seen := make(map[string]struct{}, len(og)+len(table))
	out := make([]eventTableEntry, 0, len(og)+len(table))
	for _, half := range [][]eventTableEntry{og, table} {
		for _, e := range half {
			if _, dup := seen[e.canonicalID]; dup {
				continue
			}
			seen[e.canonicalID] = struct{}{}
			out = append(out, e)
		}
	}
	return out
}

func eventRecords(entries []eventTableEntry, limit int) []types.EventRecord {
	if limit >= 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	out := make([]types.EventRecord, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.rec)
	}
	return out
}

// eventTableRunMaxSeq is the per-run seed's choir_events half. The owner
// filter of the og_objects half is dropped: runless events share the empty
// run_id across owners, so this can only over-estimate, which is a gap.
func (s *Store) eventTableRunMaxSeq(ctx context.Context, runID string) (int64, error) {
	query, args := eventTableRunMaxSeqSQL(runID)
	var seq int64
	err := s.eventTableDB("store.event_table.seed_run_seq", func(db *sql.DB) error {
		err := db.QueryRowContext(ctx, query, args...).Scan(&seq)
		if errors.Is(err, sql.ErrNoRows) {
			seq = 0
			return nil
		}
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("append event: seed seq from %s: %w", choirEventsTable, err)
	}
	return seq, nil
}

// eventTableNewestStreamSeq is the stream seed's choir_events half.
func (s *Store) eventTableNewestStreamSeq(ctx context.Context) (int64, error) {
	query, args := eventTableNewestStreamSQL()
	var newest int64
	err := s.eventTableDB("store.event_table.seed_stream_seq", func(db *sql.DB) error {
		rows, err := db.QueryContext(ctx, query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var v int64
			if err := rows.Scan(&v); err != nil {
				return err
			}
			newest = max(newest, v)
		}
		return rows.Err()
	})
	if err != nil {
		return 0, fmt.Errorf("append event: seed stream_seq from %s: %w", choirEventsTable, err)
	}
	return newest, nil
}

// Deploy 2: the writer (event-table design 2.2, 4.2). The runtime applies
// store.event_table (the owner's setting and the host's grant, both required)
// through SetEventTableWrites. With it on, AppendEvent stamps the op of every
// event whose kind's storage class is StorageEventTable, on both paths: the
// acknowledged write (W1, appendStampedEvent) and the sealer (W2,
// newTelemetryRecord). The lifecycle fold (W3) never stamps. Off, the
// tape is byte for byte deploy 1's.
//
// Rollback once any op is stamped: the setting off (live; events already in
// choir_events stay there and stay readable), then a binary down to deploy 1.
// Below deploy 1 is not supported (section 4.5).

// eventTableFloorEpoch is the store floor once an op may be stamped:
// deploy 1's epoch, the first that projects and reads choir_events
// (design 4.5).
const eventTableFloorEpoch uint64 = 2

const eventTableFloorReason = "events stamped for choir_events: deploy 1 (epoch 2) is the floor"

// SetEventTableWrites applies store.event_table to the writer. Switching it
// on writes the store floor first, durably, so no op is ever stamped on an
// unfloored store; if the floor cannot be written the writer stays off and
// the error is returned. Off keeps the floor (rows may exist). On a store
// with no tape bound it is refused (the switch stays off and no floor is
// written): the store itself keeps "writer on implies floored".
func (s *Store) SetEventTableWrites(on bool) error {
	if s == nil {
		return nil
	}
	if on {
		// A store with no tape cannot stamp, so it cannot be floored in any
		// useful way: refuse rather than set a switch that later binds a tape
		// with no floor under it (bunch 148, DeepSeek P3).
		if s.projectionTape == nil {
			s.eventTableWrites.Store(false)
			return fmt.Errorf("store: event-table writer stays off: the projection tape is not bound")
		}
		if err := s.writeEventTableFloor(); err != nil {
			s.eventTableWrites.Store(false)
			return fmt.Errorf("store: event-table writer stays off: %w", err)
		}
	}
	s.eventTableWrites.Store(on)
	return nil
}

func (s *Store) writeEventTableFloor() error {
	return writeEventTableFloorAt(s.texturePath)
}

// writeEventTableFloorAt floors a workspace. A floor above the build's own
// epoch would make the next boot refuse its own store, so it is refused here
// too (bunch 148, DeepSeek P4); the constant assertion below catches the same
// drift at compile time.
func writeEventTableFloorAt(workspace string) error {
	if storeschema.Version < eventTableFloorEpoch {
		return fmt.Errorf("store: the build's schema epoch %d is below the event-table floor %d", storeschema.Version, eventTableFloorEpoch)
	}
	return storeschema.WriteFloor(workspace, eventTableFloorEpoch, eventTableFloorReason)
}

// Compile-time guard: storeschema.Version must be at least the floor epoch
// (the constant expression underflows uint64 otherwise).
const _ = storeschema.Version - eventTableFloorEpoch

// ensureEventTableFloor floors a workspace whose choir_events holds a row.
// It reads one row by primary key, never the table.
func (s *Store) ensureEventTableFloor() error {
	var one int
	err := s.db.QueryRow(eventTableAnyRowSQL).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("runtime store: probe %s for the floor: %w", choirEventsTable, err)
	}
	return s.writeEventTableFloor()
}

// StoreFloor reports the store floor (0 for none), for observability.
func (s *Store) StoreFloor() (uint64, error) {
	if s == nil || s.texturePath == "" {
		return 0, nil
	}
	floor, found, err := storeschema.ReadFloor(filepath.Join(s.texturePath, storeschema.FloorFile))
	if err != nil || !found {
		return 0, err
	}
	return floor.Version, nil
}

// eventTableAnyRowSQL is Open's one-row probe for the floor.
const eventTableAnyRowSQL = `SELECT 1 FROM choir_events LIMIT 1`

// EventTableWritesOn reports the switch as the writer applies it.
func (s *Store) EventTableWritesOn() bool {
	return s != nil && s.eventTableWrites.Load()
}

// stampsEventTable reports whether an event of this kind is stamped now: a
// bound tape, the switch on, and the kind in the storage class.
func (s *Store) stampsEventTable(kind types.EventKind) bool {
	return s.projectionTape != nil && s.eventTableWrites.Load() && kind.Storage() == types.StorageEventTable
}

// eventObjectOp is the object_recorded op of one choir.event object, stamped
// for choir_events when stamped is set. It is the only production code that
// sets ProjectionOp.Table (event_table_guard_test.go).
func eventObjectOp(obj objectgraph.Object, stamped bool) (computerevent.ProjectionOp, error) {
	body, err := json.Marshal(obj)
	if err != nil {
		return computerevent.ProjectionOp{}, fmt.Errorf("marshal event object: %w", err)
	}
	op := computerevent.ProjectionOp{Kind: computerevent.ProjectionOpObject, CanonicalID: obj.CanonicalID, Body: body}
	if stamped {
		op.Table = choirEventsTable
	}
	return op, nil
}

// appendStampedEvent is the acknowledged write (W1) of one storage-class
// event with the stamp. The object is the one AppendEventOG's ogPut builds,
// the refusals are the ones PutObject applies before its interceptor
// (PreflightWrite), and the op goes onto the tape as its own projection
// batch, as the interceptor would put it. Only the Table differs.
func (s *Store) appendStampedEvent(ctx context.Context, rec *types.EventRecord) error {
	if s.projectionTape == nil || s.ogStore == nil {
		return fmt.Errorf("store: a stamped event needs the projection tape")
	}
	now := rec.Timestamp
	if now.IsZero() {
		now = time.Now().UTC()
	}
	obj, err := buildEventObject(rec, now)
	if err != nil {
		return fmt.Errorf("store: create object: %w", err)
	}
	if err := s.ogStore.PreflightWrite(ctx, []objectgraph.Object{obj}, nil); err != nil {
		return fmt.Errorf("store: create object: %w", err)
	}
	op, err := eventObjectOp(obj, true)
	if err != nil {
		return fmt.Errorf("store: %w", err)
	}
	return s.projectionTape.appendOps(ctx, []computerevent.ProjectionOp{op})
}
