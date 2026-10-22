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
