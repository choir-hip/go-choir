-- Frozen pre-split telemetry partition schema (telemetryPartitionDDL at 95ea10801).
-- Do not edit.

CREATE TABLE IF NOT EXISTS choir_events (
	tape_seq INTEGER NOT NULL,
	op_index INTEGER NOT NULL,
	canonical_id TEXT NOT NULL,
	object_kind TEXT NOT NULL,
	owner_id TEXT NOT NULL,
	computer_id TEXT NOT NULL DEFAULT '',
	version_id TEXT NOT NULL DEFAULT '',
	content_hash TEXT NOT NULL,
	body          BLOB NOT NULL,
	metadata TEXT NOT NULL,
	created_at    DATETIME NOT NULL,
	updated_at    DATETIME NOT NULL,
	tombstone INTEGER NOT NULL DEFAULT 0,
	superseded_by TEXT NOT NULL DEFAULT '',
	run_id TEXT NOT NULL DEFAULT '',
	seq INTEGER NOT NULL DEFAULT 0,
	stream_seq INTEGER NOT NULL DEFAULT 0,
	trajectory_id TEXT NOT NULL DEFAULT '',
	event_kind TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (tape_seq, op_index)
);
CREATE INDEX IF NOT EXISTS idx_choir_events_run_seq ON choir_events(run_id, seq);
CREATE INDEX IF NOT EXISTS idx_choir_events_owner_stream ON choir_events(owner_id, stream_seq);
CREATE INDEX IF NOT EXISTS idx_choir_events_trajectory_stream ON choir_events(trajectory_id, stream_seq);
CREATE INDEX IF NOT EXISTS idx_choir_events_updated ON choir_events(updated_at);
CREATE INDEX IF NOT EXISTS idx_choir_events_kind_updated ON choir_events(event_kind, updated_at);
