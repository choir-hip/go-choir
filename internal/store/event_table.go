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
	"strings"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
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
