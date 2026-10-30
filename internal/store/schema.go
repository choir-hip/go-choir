package store

// The guest store's schema: one clean current DDL for the SQLite file at
// StateDBPath (docs/design/data-system-build-sheet-2026-10-30.md decision 3).
// A fresh database is built from these statements; there are no migration
// ladders. The guest Dolt migrator copies rows
// from a legacy workspace into this schema.
//
// Types follow build sheet decision 5: TEXT for strings and JSON, INTEGER for
// integers, BLOB for binary, DATETIME for timestamps (bound as time.Time; the
// dbx opener sets the modernc time format). The object-graph tables
// (og_objects, og_edges, og_key_index) are created by internal/objectgraph.

// storeSchemaVersion is the schema version row in store_meta. A database
// written by a newer version is refused.
const storeSchemaVersion = 1

const storeMetaDDL = `
CREATE TABLE IF NOT EXISTS store_meta (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
`

// schemaDDL creates the runtime, texture and event tables.
const schemaDDL = `
CREATE TABLE IF NOT EXISTS events (
	event_id TEXT NOT NULL,
	loop_id TEXT NOT NULL DEFAULT '',
	agent_id TEXT NOT NULL DEFAULT '',
	channel_id TEXT NOT NULL DEFAULT '',
	owner_id TEXT NOT NULL DEFAULT '',
	trajectory_id TEXT NOT NULL DEFAULT '',
	seq INTEGER NOT NULL,
	stream_seq INTEGER NOT NULL DEFAULT 0,
	ts         DATETIME NOT NULL,
	kind TEXT NOT NULL,
	phase TEXT NOT NULL DEFAULT '',
	payload_json TEXT NOT NULL DEFAULT '{}',
	PRIMARY KEY (event_id)
);

CREATE TABLE IF NOT EXISTS run_memory_entries (
	entry_id TEXT PRIMARY KEY,
	loop_id TEXT NOT NULL,
	owner_id TEXT NOT NULL DEFAULT '',
	agent_id TEXT NOT NULL DEFAULT '',
	parent_entry_id TEXT NOT NULL DEFAULT '',
	seq INTEGER NOT NULL,
	kind TEXT NOT NULL,
	role TEXT NOT NULL DEFAULT '',
	message_json TEXT NOT NULL DEFAULT '',
	summary TEXT NOT NULL DEFAULT '',
	first_kept_entry_id TEXT NOT NULL DEFAULT '',
	tokens_before INTEGER NOT NULL DEFAULT 0,
	reason TEXT NOT NULL DEFAULT '',
	model TEXT NOT NULL DEFAULT '',
	details_json TEXT NOT NULL DEFAULT '{}',
	created_at           DATETIME NOT NULL,
	UNIQUE(loop_id, seq)
);


CREATE TABLE IF NOT EXISTS wire_publish_debounce_entries (
	entry_id INTEGER PRIMARY KEY AUTOINCREMENT,
	owner_id TEXT NOT NULL,
	computer_id TEXT NOT NULL,
	agent_id TEXT NOT NULL,
	content TEXT NOT NULL,
	created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS wire_publish_debounce_state (
	owner_id TEXT NOT NULL,
	computer_id TEXT NOT NULL,
	agent_id TEXT NOT NULL,
	last_dispatch_at DATETIME NOT NULL,
	PRIMARY KEY (owner_id, computer_id, agent_id)
);

CREATE INDEX IF NOT EXISTS idx_wire_publish_debounce_entries_scope
	ON wire_publish_debounce_entries(owner_id, computer_id, agent_id, entry_id);

CREATE TABLE IF NOT EXISTS co_super_slots (
	owner_id TEXT NOT NULL DEFAULT '',
	trajectory_id TEXT NOT NULL DEFAULT '',
	slot TEXT NOT NULL DEFAULT '',
	run_id TEXT NOT NULL DEFAULT '',
	agent_id TEXT NOT NULL DEFAULT '',
	requested_by_run_id TEXT NOT NULL DEFAULT '',
	claimed_at     DATETIME NOT NULL,
	updated_at     DATETIME NOT NULL,
	PRIMARY KEY (owner_id, trajectory_id, slot)
);

CREATE TABLE IF NOT EXISTS media_progress (
	owner_id TEXT NOT NULL DEFAULT '',
	media_kind TEXT NOT NULL DEFAULT '',
	media_identity_hash TEXT NOT NULL DEFAULT '',
	media_identity TEXT NOT NULL DEFAULT '',
	position_seconds  REAL NOT NULL DEFAULT 0,
	duration_seconds  REAL NOT NULL DEFAULT 0,
	playback_rate     REAL NOT NULL DEFAULT 1,
	updated_by_device TEXT NOT NULL DEFAULT '',
	updated_at        DATETIME NOT NULL,
	PRIMARY KEY (owner_id, media_kind, media_identity_hash)
);

CREATE TABLE IF NOT EXISTS media_recents (
	owner_id TEXT NOT NULL DEFAULT '',
	media_kind TEXT NOT NULL DEFAULT '',
	media_identity_hash TEXT NOT NULL DEFAULT '',
	media_identity TEXT NOT NULL DEFAULT '',
	title TEXT NOT NULL DEFAULT '',
	file_name TEXT NOT NULL DEFAULT '',
	file_path TEXT NOT NULL DEFAULT '',
	source_url TEXT NOT NULL DEFAULT '',
	media_type TEXT NOT NULL DEFAULT '',
	content_id TEXT NOT NULL DEFAULT '',
	opened_at       DATETIME NOT NULL,
	PRIMARY KEY (owner_id, media_kind, media_identity_hash)
);

CREATE TABLE IF NOT EXISTS user_preferences (
	owner_id TEXT NOT NULL DEFAULT '',
	preference_key TEXT NOT NULL DEFAULT '',
	value_json TEXT NOT NULL DEFAULT '{}',
	updated_at      DATETIME NOT NULL,
	PRIMARY KEY (owner_id, preference_key)
);

CREATE INDEX IF NOT EXISTS idx_events_loop_id_seq ON events(loop_id, seq);
CREATE INDEX IF NOT EXISTS idx_events_owner_id ON events(owner_id);
CREATE INDEX IF NOT EXISTS idx_events_agent_id ON events(agent_id);
CREATE INDEX IF NOT EXISTS idx_events_channel_id_ts ON events(channel_id, ts);
CREATE INDEX IF NOT EXISTS idx_events_ts ON events(ts);
CREATE INDEX IF NOT EXISTS idx_run_memory_entries_parent ON run_memory_entries(parent_entry_id);
CREATE INDEX IF NOT EXISTS idx_co_super_slots_run_id ON co_super_slots(run_id);
CREATE INDEX IF NOT EXISTS idx_co_super_slots_agent_id ON co_super_slots(owner_id, agent_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_media_progress_owner_kind_updated ON media_progress(owner_id, media_kind, updated_at);
CREATE INDEX IF NOT EXISTS idx_media_recents_owner_kind_opened ON media_recents(owner_id, media_kind, opened_at);
CREATE INDEX IF NOT EXISTS idx_user_preferences_owner_updated ON user_preferences(owner_id, updated_at);

CREATE TABLE IF NOT EXISTS desktop_workspaces (
	owner_id TEXT NOT NULL,
	computer_id TEXT NOT NULL DEFAULT '',
	desktop_id TEXT NOT NULL,
	windows_json TEXT NOT NULL DEFAULT '[]',
	active_window TEXT NOT NULL DEFAULT '',
	updated_at     DATETIME NOT NULL,
	PRIMARY KEY (owner_id, computer_id, desktop_id)
);

CREATE TABLE IF NOT EXISTS desktop_sessions (
	owner_id TEXT NOT NULL,
	computer_id TEXT NOT NULL DEFAULT '',
	desktop_id TEXT NOT NULL,
	session_id TEXT NOT NULL,
	device_id TEXT NOT NULL DEFAULT '',
	viewport_profile TEXT NOT NULL DEFAULT '',
	visibility_state TEXT NOT NULL DEFAULT '',
	last_input_at     DATETIME NULL,
	driver_until      DATETIME NULL,
	created_at        DATETIME NOT NULL,
	updated_at        DATETIME NOT NULL,
	PRIMARY KEY (owner_id, computer_id, desktop_id, session_id)
);

CREATE TABLE IF NOT EXISTS desktop_app_instances (
	owner_id TEXT NOT NULL,
	computer_id TEXT NOT NULL DEFAULT '',
	desktop_id TEXT NOT NULL,
	app_instance_id TEXT NOT NULL,
	app_id TEXT NOT NULL,
	title TEXT NOT NULL DEFAULT '',
	app_context_json TEXT NOT NULL DEFAULT '{}',
	lifecycle TEXT NOT NULL DEFAULT 'open',
	shared_stack_rank INTEGER NOT NULL DEFAULT 0,
	last_used_at          DATETIME NOT NULL,
	created_by_session_id TEXT NOT NULL DEFAULT '',
	created_at            DATETIME NOT NULL,
	updated_at            DATETIME NOT NULL,
	PRIMARY KEY (owner_id, computer_id, desktop_id, app_instance_id)
);

CREATE TABLE IF NOT EXISTS desktop_window_placements (
	owner_id TEXT NOT NULL,
	computer_id TEXT NOT NULL DEFAULT '',
	desktop_id TEXT NOT NULL,
	session_id TEXT NOT NULL,
	app_instance_id TEXT NOT NULL,
	x INTEGER NOT NULL DEFAULT 100,
	y INTEGER NOT NULL DEFAULT 100,
	width INTEGER NOT NULL DEFAULT 600,
	height INTEGER NOT NULL DEFAULT 400,
	mode TEXT NOT NULL DEFAULT 'normal',
	local_z_index INTEGER NOT NULL DEFAULT 0,
	local_focused INTEGER NOT NULL DEFAULT 0,
	restored_geometry_json TEXT NOT NULL DEFAULT '',
	updated_at             DATETIME NOT NULL,
	PRIMARY KEY (owner_id, computer_id, desktop_id, session_id, app_instance_id)
);

CREATE TABLE IF NOT EXISTS computer_event_projection_heads (
	computer_id TEXT PRIMARY KEY,
	sequence INTEGER NOT NULL,
	canonical_event_head TEXT NOT NULL,
	desired_event_head TEXT NOT NULL,
	effective_event_head TEXT NOT NULL,
	pending_transition_ref TEXT NULL,
	desired_state_commitment TEXT NOT NULL,
	effective_state_commitment TEXT NOT NULL,
	reducer_version INTEGER NOT NULL,
	credential_revocation_epoch INTEGER NOT NULL,
	updated_at                     DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS computer_event_index (
	event_digest TEXT PRIMARY KEY,
	computer_id TEXT NOT NULL,
	sequence INTEGER NOT NULL,
	previous_head TEXT NOT NULL,
	event_kind TEXT NOT NULL,
	prepared_event_json TEXT NULL,
	event_artifact_digest TEXT NOT NULL,
	event_pin_receipt_digest TEXT NOT NULL,
	payload_pin_receipt_digests_json TEXT NOT NULL,
	request_commitment TEXT NOT NULL,
	idempotency_key TEXT NOT NULL,
	status TEXT NOT NULL,
	next_desired_event_head TEXT NOT NULL,
	next_effective_event_head TEXT NOT NULL,
	next_pending_transition_ref TEXT NULL,
	next_desired_state_commitment TEXT NOT NULL,
	next_effective_state_commitment TEXT NOT NULL,
	next_reducer_version INTEGER NOT NULL,
	next_credential_revocation_epoch INTEGER NOT NULL,
	target_state_commitment TEXT NULL,
	restored_prior_effective INTEGER NOT NULL DEFAULT 0,
	event_head_receipt_digest TEXT NULL,
	prepared_at                    DATETIME NOT NULL,
	finalized_at                   DATETIME NULL,
	UNIQUE(computer_id, sequence),
	UNIQUE(computer_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_computer_event_index_status ON computer_event_index(computer_id, status, sequence);

CREATE TABLE IF NOT EXISTS self_development_start_intents (
	computer_id TEXT NOT NULL,
	idempotency_key TEXT NOT NULL,
	request_commitment TEXT NOT NULL,
	created_at         DATETIME NOT NULL,
	PRIMARY KEY(computer_id, idempotency_key)
);

CREATE TABLE IF NOT EXISTS self_development_operations (
	operation_id TEXT PRIMARY KEY,
	computer_id TEXT NOT NULL,
	idempotency_key TEXT NOT NULL,
	request_commitment TEXT NOT NULL,
	trajectory_id TEXT NOT NULL,
	capsule_id TEXT NOT NULL DEFAULT '',
	base_head TEXT NOT NULL,
	prompt_artifact_ref TEXT NOT NULL,
	bundle_digest TEXT NOT NULL DEFAULT '',
	release_digest TEXT NOT NULL DEFAULT '',
	code_ref TEXT NOT NULL DEFAULT '',
	artifact_program_ref TEXT NOT NULL DEFAULT '',
	verifier_refs_json TEXT NOT NULL DEFAULT '[]',
	decision_actor TEXT NOT NULL DEFAULT '',
	decision_event TEXT NOT NULL DEFAULT '',
	decision_receipt TEXT NOT NULL DEFAULT '',
	desired_head TEXT NOT NULL,
	effective_head TEXT NOT NULL,
	materialization_receipt TEXT NOT NULL DEFAULT '',
	checkpoint_ref TEXT NOT NULL DEFAULT '',
	route_certificate TEXT NOT NULL DEFAULT '',
	route_generation INTEGER NULL,
	route_receipt TEXT NOT NULL DEFAULT '',
	mode_receipt TEXT NOT NULL DEFAULT '',
	lifecycle_receipt TEXT NOT NULL DEFAULT '',
	last_intent_error TEXT NOT NULL DEFAULT '',
	state TEXT NOT NULL,
	terminal_error TEXT NOT NULL DEFAULT '',
	created_at                     DATETIME NOT NULL,
	updated_at                     DATETIME NOT NULL,
	UNIQUE(computer_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_self_development_operations_trajectory ON self_development_operations(trajectory_id);
CREATE INDEX IF NOT EXISTS idx_self_development_operations_state ON self_development_operations(computer_id, state, updated_at);
CREATE INDEX IF NOT EXISTS idx_self_development_operations_bundle ON self_development_operations(computer_id, bundle_digest);

CREATE INDEX IF NOT EXISTS idx_desktop_workspaces_owner_id ON desktop_workspaces(owner_id);
CREATE INDEX IF NOT EXISTS idx_desktop_sessions_driver ON desktop_sessions(owner_id, desktop_id, driver_until);
CREATE INDEX IF NOT EXISTS idx_desktop_app_instances_stack ON desktop_app_instances(owner_id, desktop_id, shared_stack_rank);
CREATE INDEX IF NOT EXISTS idx_desktop_window_placements_instance ON desktop_window_placements(owner_id, desktop_id, app_instance_id, updated_at);

CREATE INDEX IF NOT EXISTS idx_events_owner_stream_seq ON events(owner_id, stream_seq);
CREATE INDEX IF NOT EXISTS idx_events_trajectory_stream_seq ON events(trajectory_id, stream_seq);
CREATE INDEX IF NOT EXISTS idx_run_memory_entries_kind ON run_memory_entries(owner_id, agent_id, kind, created_at);

CREATE TABLE IF NOT EXISTS texture_revisions (
	revision_id TEXT PRIMARY KEY,
	doc_id TEXT NOT NULL,
	owner_id TEXT NOT NULL,
	author_kind TEXT NOT NULL,
	author_label TEXT NOT NULL DEFAULT '',
	version_number INTEGER NOT NULL DEFAULT 0,
	content TEXT NOT NULL,
	body_doc_json TEXT NOT NULL DEFAULT '',
	source_entities_json TEXT NOT NULL DEFAULT '[]',
	citations_json TEXT NOT NULL,
	metadata_json TEXT NOT NULL,
	provenance_json TEXT NOT NULL DEFAULT '{}',
	revision_hash TEXT NOT NULL DEFAULT '',
	parent_revision_id TEXT NOT NULL DEFAULT '',
	created_at          DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS texture_source_entities (
	canonical_id TEXT NOT NULL,
	version_id TEXT NOT NULL,
	owner_id TEXT NOT NULL,
	computer_id TEXT NOT NULL DEFAULT '',
	content_hash TEXT NOT NULL,
	body TEXT NOT NULL DEFAULT '',
	metadata_json TEXT NOT NULL DEFAULT '{}',
	legacy_source_entity_id TEXT NOT NULL DEFAULT '',
	created_at             DATETIME NOT NULL,
	PRIMARY KEY (canonical_id, version_id)
);

CREATE TABLE IF NOT EXISTS texture_source_refs (
	canonical_id TEXT NOT NULL,
	version_id TEXT NOT NULL,
	owner_id TEXT NOT NULL,
	computer_id TEXT NOT NULL DEFAULT '',
	content_hash TEXT NOT NULL,
	doc_id TEXT NOT NULL,
	texture_revision_id TEXT NOT NULL,
	body_node_id TEXT NOT NULL DEFAULT '',
	body_node_path_hash TEXT NOT NULL DEFAULT '',
	legacy_source_entity_id TEXT NOT NULL DEFAULT '',
	source_entity_canonical_id TEXT NOT NULL,
	source_entity_version_id TEXT NOT NULL,
	display_mode TEXT NOT NULL,
	citation_state TEXT NOT NULL,
	metadata_json TEXT NOT NULL DEFAULT '{}',
	created_at                   DATETIME NOT NULL,
	PRIMARY KEY (canonical_id, version_id)
);

CREATE TABLE IF NOT EXISTS texture_document_aliases (
	owner_id TEXT NOT NULL,
	computer_id TEXT NOT NULL DEFAULT '',
	source_path TEXT NOT NULL,
	doc_id TEXT NOT NULL,
	created_at          DATETIME NOT NULL,
	updated_at          DATETIME NOT NULL,
	PRIMARY KEY (owner_id, computer_id, source_path)
);

CREATE INDEX IF NOT EXISTS idx_texture_revs_doc ON texture_revisions(doc_id);
CREATE INDEX IF NOT EXISTS idx_texture_revs_owner ON texture_revisions(owner_id);
CREATE INDEX IF NOT EXISTS idx_texture_revs_doc_created ON texture_revisions(doc_id, created_at);
CREATE INDEX IF NOT EXISTS idx_texture_source_entities_owner ON texture_source_entities(owner_id);
CREATE INDEX IF NOT EXISTS idx_texture_source_refs_revision ON texture_source_refs(owner_id, doc_id, texture_revision_id);
CREATE INDEX IF NOT EXISTS idx_texture_source_refs_source ON texture_source_refs(source_entity_canonical_id, source_entity_version_id);

CREATE TABLE IF NOT EXISTS texture_agent_mutations (
	doc_id TEXT NOT NULL,
	loop_id TEXT NOT NULL,
	owner_id TEXT NOT NULL,
	computer_id TEXT NOT NULL DEFAULT '',
	state TEXT NOT NULL DEFAULT 'pending',
	scheduled_message_seq INTEGER NOT NULL DEFAULT 0,
	revision_id TEXT NOT NULL DEFAULT '',
	created_at          DATETIME NOT NULL,
	completed_at        DATETIME,
	PRIMARY KEY (owner_id, computer_id, doc_id, loop_id)
);

CREATE INDEX IF NOT EXISTS idx_texture_mutations_doc ON texture_agent_mutations(doc_id);
CREATE INDEX IF NOT EXISTS idx_texture_mutations_run ON texture_agent_mutations(loop_id);


CREATE TABLE IF NOT EXISTS agent_evidence (
	evidence_id TEXT PRIMARY KEY,
	owner_id TEXT NOT NULL,
	agent_id TEXT NOT NULL,
	kind TEXT NOT NULL,
	source_uri TEXT NOT NULL DEFAULT '',
	title TEXT NOT NULL DEFAULT '',
	content TEXT NOT NULL,
	metadata_json TEXT NOT NULL DEFAULT '{}',
	created_at     DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_agent_evidence_owner_agent ON agent_evidence(owner_id, agent_id, created_at);
CREATE INDEX IF NOT EXISTS idx_agent_evidence_owner_created ON agent_evidence(owner_id, created_at);


CREATE INDEX IF NOT EXISTS idx_texture_mutations_scope ON texture_agent_mutations(owner_id, computer_id, doc_id, state);
CREATE INDEX IF NOT EXISTS idx_texture_revs_doc_version ON texture_revisions(doc_id, owner_id, version_number);
CREATE INDEX IF NOT EXISTS idx_texture_aliases_doc ON texture_document_aliases(owner_id, computer_id, doc_id);

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
`
