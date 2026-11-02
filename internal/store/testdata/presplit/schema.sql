-- Frozen pre-split guest store schema (store_meta.schema_version 1), dumped
-- from sqlite_master of a fresh store opened by the code at 95ea10801.
-- Old-layout fixtures for the header/body split execute this file verbatim.
-- Do not edit: it is the layout the migration reads.

-- table store_meta
CREATE TABLE store_meta (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

-- table events
CREATE TABLE events (
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

-- table run_memory_entries
CREATE TABLE run_memory_entries (
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

-- table wire_publish_debounce_entries
CREATE TABLE wire_publish_debounce_entries (
	entry_id INTEGER PRIMARY KEY AUTOINCREMENT,
	owner_id TEXT NOT NULL,
	computer_id TEXT NOT NULL,
	agent_id TEXT NOT NULL,
	content TEXT NOT NULL,
	created_at DATETIME NOT NULL
);

-- table wire_publish_debounce_state
CREATE TABLE wire_publish_debounce_state (
	owner_id TEXT NOT NULL,
	computer_id TEXT NOT NULL,
	agent_id TEXT NOT NULL,
	last_dispatch_at DATETIME NOT NULL,
	PRIMARY KEY (owner_id, computer_id, agent_id)
);

-- table co_super_slots
CREATE TABLE co_super_slots (
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

-- table media_progress
CREATE TABLE media_progress (
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

-- table media_recents
CREATE TABLE media_recents (
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

-- table user_preferences
CREATE TABLE user_preferences (
	owner_id TEXT NOT NULL DEFAULT '',
	preference_key TEXT NOT NULL DEFAULT '',
	value_json TEXT NOT NULL DEFAULT '{}',
	updated_at      DATETIME NOT NULL,
	PRIMARY KEY (owner_id, preference_key)
);

-- table desktop_workspaces
CREATE TABLE desktop_workspaces (
	owner_id TEXT NOT NULL,
	computer_id TEXT NOT NULL DEFAULT '',
	desktop_id TEXT NOT NULL,
	windows_json TEXT NOT NULL DEFAULT '[]',
	active_window TEXT NOT NULL DEFAULT '',
	updated_at     DATETIME NOT NULL,
	PRIMARY KEY (owner_id, computer_id, desktop_id)
);

-- table desktop_sessions
CREATE TABLE desktop_sessions (
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

-- table desktop_app_instances
CREATE TABLE desktop_app_instances (
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

-- table desktop_window_placements
CREATE TABLE desktop_window_placements (
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

-- table computer_event_projection_heads
CREATE TABLE computer_event_projection_heads (
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

-- table computer_event_index
CREATE TABLE computer_event_index (
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

-- table self_development_start_intents
CREATE TABLE self_development_start_intents (
	computer_id TEXT NOT NULL,
	idempotency_key TEXT NOT NULL,
	request_commitment TEXT NOT NULL,
	created_at         DATETIME NOT NULL,
	PRIMARY KEY(computer_id, idempotency_key)
);

-- table self_development_operations
CREATE TABLE self_development_operations (
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

-- table texture_revisions
CREATE TABLE texture_revisions (
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

-- table texture_source_entities
CREATE TABLE texture_source_entities (
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

-- table texture_source_refs
CREATE TABLE texture_source_refs (
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

-- table texture_document_aliases
CREATE TABLE texture_document_aliases (
	owner_id TEXT NOT NULL,
	computer_id TEXT NOT NULL DEFAULT '',
	source_path TEXT NOT NULL,
	doc_id TEXT NOT NULL,
	created_at          DATETIME NOT NULL,
	updated_at          DATETIME NOT NULL,
	PRIMARY KEY (owner_id, computer_id, source_path)
);

-- table texture_agent_mutations
CREATE TABLE texture_agent_mutations (
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

-- table agent_evidence
CREATE TABLE agent_evidence (
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

-- table choir_events
CREATE TABLE choir_events (
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

-- table telemetry_partitions_retired
CREATE TABLE telemetry_partitions_retired (
	name TEXT NOT NULL PRIMARY KEY,
	class TEXT NOT NULL,
	day TEXT NOT NULL,
	tape_seq INTEGER NOT NULL,
	cited_moved INTEGER NOT NULL DEFAULT 0
);

-- table og_objects
CREATE TABLE og_objects (
	canonical_id  TEXT NOT NULL PRIMARY KEY,
	object_kind   TEXT NOT NULL,
	owner_id      TEXT NOT NULL,
	computer_id   TEXT NOT NULL DEFAULT '',
	version_id    TEXT NOT NULL DEFAULT '',
	content_hash  TEXT NOT NULL,
	body          BLOB,
	metadata      TEXT NOT NULL,
	created_at    DATETIME NOT NULL,
	updated_at    DATETIME NOT NULL,
	tombstone     INTEGER NOT NULL DEFAULT 0,
	superseded_by TEXT NOT NULL DEFAULT '',
	og_meta_run_id TEXT GENERATED ALWAYS AS (CASE json_type(metadata, '$.run_id') WHEN 'true' THEN 'true' WHEN 'false' THEN 'false' ELSE CAST(json_extract(metadata, '$.run_id') AS TEXT) END) VIRTUAL,
	og_meta_trajectory_id TEXT GENERATED ALWAYS AS (CASE json_type(metadata, '$.trajectory_id') WHEN 'true' THEN 'true' WHEN 'false' THEN 'false' ELSE CAST(json_extract(metadata, '$.trajectory_id') AS TEXT) END) VIRTUAL,
	og_meta_channel_id TEXT GENERATED ALWAYS AS (CASE json_type(metadata, '$.channel_id') WHEN 'true' THEN 'true' WHEN 'false' THEN 'false' ELSE CAST(json_extract(metadata, '$.channel_id') AS TEXT) END) VIRTUAL,
	og_rec_inbox TEXT GENERATED ALWAYS AS (CASE WHEN object_kind = 'choir.commitment_record' THEN CASE json_type(metadata, '$.inbox') WHEN 'true' THEN 'true' WHEN 'false' THEN 'false' ELSE CAST(json_extract(metadata, '$.inbox') AS TEXT) END END) VIRTUAL,
	og_rec_sender_inbox TEXT GENERATED ALWAYS AS (CASE WHEN object_kind = 'choir.commitment_record' THEN CASE json_type(metadata, '$.sender_inbox') WHEN 'true' THEN 'true' WHEN 'false' THEN 'false' ELSE CAST(json_extract(metadata, '$.sender_inbox') AS TEXT) END END) VIRTUAL,
	og_rec_conversation_id TEXT GENERATED ALWAYS AS (CASE WHEN object_kind = 'choir.commitment_record' THEN CASE json_type(metadata, '$.conversation_id') WHEN 'true' THEN 'true' WHEN 'false' THEN 'false' ELSE CAST(json_extract(metadata, '$.conversation_id') AS TEXT) END END) VIRTUAL,
	og_rec_inbox_seq INTEGER GENERATED ALWAYS AS (CASE WHEN object_kind = 'choir.commitment_record' THEN CAST(json_extract(metadata, '$.inbox_seq') AS INTEGER) END) VIRTUAL,
	og_rec_claim INTEGER GENERATED ALWAYS AS (CASE WHEN object_kind = 'choir.commitment_record' THEN CASE WHEN body IS NULL OR json_valid(CAST(body AS TEXT)) THEN (json_extract(CAST(body AS TEXT), '$.directive') IS NULL OR CAST(json_extract(CAST(body AS TEXT), '$.directive') AS TEXT) = '') END END) VIRTUAL
);

-- table og_edges
CREATE TABLE og_edges (
	edge_id    TEXT NOT NULL PRIMARY KEY,
	from_id    TEXT NOT NULL,
	to_id      TEXT NOT NULL,
	kind       TEXT NOT NULL,
	metadata   TEXT NOT NULL,
	created_at DATETIME NOT NULL,
	tombstone  INTEGER NOT NULL DEFAULT 0
);

-- table og_key_index
CREATE TABLE og_key_index (
	canonical_id TEXT     NOT NULL,
	key_name     TEXT     NOT NULL,
	key_value    TEXT     NULL,
	owner_id     TEXT     NOT NULL,
	updated_at   DATETIME NOT NULL,
	content_hash TEXT     NOT NULL,
	derive_ok    INTEGER  NOT NULL DEFAULT 1,
	PRIMARY KEY (canonical_id, key_name)
) WITHOUT ROWID;

-- index idx_wire_publish_debounce_entries_scope
CREATE INDEX idx_wire_publish_debounce_entries_scope
	ON wire_publish_debounce_entries(owner_id, computer_id, agent_id, entry_id);

-- index idx_events_loop_id_seq
CREATE INDEX idx_events_loop_id_seq ON events(loop_id, seq);

-- index idx_events_owner_id
CREATE INDEX idx_events_owner_id ON events(owner_id);

-- index idx_events_agent_id
CREATE INDEX idx_events_agent_id ON events(agent_id);

-- index idx_events_channel_id_ts
CREATE INDEX idx_events_channel_id_ts ON events(channel_id, ts);

-- index idx_events_ts
CREATE INDEX idx_events_ts ON events(ts);

-- index idx_run_memory_entries_parent
CREATE INDEX idx_run_memory_entries_parent ON run_memory_entries(parent_entry_id);

-- index idx_co_super_slots_run_id
CREATE INDEX idx_co_super_slots_run_id ON co_super_slots(run_id);

-- index idx_co_super_slots_agent_id
CREATE INDEX idx_co_super_slots_agent_id ON co_super_slots(owner_id, agent_id, updated_at);

-- index idx_media_progress_owner_kind_updated
CREATE INDEX idx_media_progress_owner_kind_updated ON media_progress(owner_id, media_kind, updated_at);

-- index idx_media_recents_owner_kind_opened
CREATE INDEX idx_media_recents_owner_kind_opened ON media_recents(owner_id, media_kind, opened_at);

-- index idx_user_preferences_owner_updated
CREATE INDEX idx_user_preferences_owner_updated ON user_preferences(owner_id, updated_at);

-- index idx_computer_event_index_status
CREATE INDEX idx_computer_event_index_status ON computer_event_index(computer_id, status, sequence);

-- index idx_self_development_operations_trajectory
CREATE INDEX idx_self_development_operations_trajectory ON self_development_operations(trajectory_id);

-- index idx_self_development_operations_state
CREATE INDEX idx_self_development_operations_state ON self_development_operations(computer_id, state, updated_at);

-- index idx_self_development_operations_bundle
CREATE INDEX idx_self_development_operations_bundle ON self_development_operations(computer_id, bundle_digest);

-- index idx_desktop_workspaces_owner_id
CREATE INDEX idx_desktop_workspaces_owner_id ON desktop_workspaces(owner_id);

-- index idx_desktop_sessions_driver
CREATE INDEX idx_desktop_sessions_driver ON desktop_sessions(owner_id, desktop_id, driver_until);

-- index idx_desktop_app_instances_stack
CREATE INDEX idx_desktop_app_instances_stack ON desktop_app_instances(owner_id, desktop_id, shared_stack_rank);

-- index idx_desktop_window_placements_instance
CREATE INDEX idx_desktop_window_placements_instance ON desktop_window_placements(owner_id, desktop_id, app_instance_id, updated_at);

-- index idx_events_owner_stream_seq
CREATE INDEX idx_events_owner_stream_seq ON events(owner_id, stream_seq);

-- index idx_events_trajectory_stream_seq
CREATE INDEX idx_events_trajectory_stream_seq ON events(trajectory_id, stream_seq);

-- index idx_run_memory_entries_kind
CREATE INDEX idx_run_memory_entries_kind ON run_memory_entries(owner_id, agent_id, kind, created_at);

-- index idx_run_memory_entries_loop_retire
CREATE INDEX idx_run_memory_entries_loop_retire ON run_memory_entries(loop_id, kind, created_at, owner_id, agent_id);

-- index idx_texture_revs_doc
CREATE INDEX idx_texture_revs_doc ON texture_revisions(doc_id);

-- index idx_texture_revs_owner
CREATE INDEX idx_texture_revs_owner ON texture_revisions(owner_id);

-- index idx_texture_revs_doc_created
CREATE INDEX idx_texture_revs_doc_created ON texture_revisions(doc_id, created_at);

-- index idx_texture_source_entities_owner
CREATE INDEX idx_texture_source_entities_owner ON texture_source_entities(owner_id);

-- index idx_texture_source_refs_revision
CREATE INDEX idx_texture_source_refs_revision ON texture_source_refs(owner_id, doc_id, texture_revision_id);

-- index idx_texture_source_refs_source
CREATE INDEX idx_texture_source_refs_source ON texture_source_refs(source_entity_canonical_id, source_entity_version_id);

-- index idx_texture_mutations_doc
CREATE INDEX idx_texture_mutations_doc ON texture_agent_mutations(doc_id);

-- index idx_texture_mutations_run
CREATE INDEX idx_texture_mutations_run ON texture_agent_mutations(loop_id);

-- index idx_agent_evidence_owner_agent
CREATE INDEX idx_agent_evidence_owner_agent ON agent_evidence(owner_id, agent_id, created_at);

-- index idx_agent_evidence_owner_created
CREATE INDEX idx_agent_evidence_owner_created ON agent_evidence(owner_id, created_at);

-- index idx_texture_mutations_scope
CREATE INDEX idx_texture_mutations_scope ON texture_agent_mutations(owner_id, computer_id, doc_id, state);

-- index idx_texture_revs_doc_version
CREATE INDEX idx_texture_revs_doc_version ON texture_revisions(doc_id, owner_id, version_number);

-- index idx_texture_aliases_doc
CREATE INDEX idx_texture_aliases_doc ON texture_document_aliases(owner_id, computer_id, doc_id);

-- index idx_choir_events_run_seq
CREATE INDEX idx_choir_events_run_seq ON choir_events(run_id, seq);

-- index idx_choir_events_owner_stream
CREATE INDEX idx_choir_events_owner_stream ON choir_events(owner_id, stream_seq);

-- index idx_choir_events_trajectory_stream
CREATE INDEX idx_choir_events_trajectory_stream ON choir_events(trajectory_id, stream_seq);

-- index idx_choir_events_updated
CREATE INDEX idx_choir_events_updated ON choir_events(updated_at);

-- index idx_choir_events_kind_updated
CREATE INDEX idx_choir_events_kind_updated ON choir_events(event_kind, updated_at);

-- index idx_og_objects_updated
CREATE INDEX idx_og_objects_updated ON og_objects(updated_at);

-- index idx_og_objects_kind_id
CREATE INDEX idx_og_objects_kind_id ON og_objects(object_kind, canonical_id);

-- index idx_og_objects_owner_computer_kind
CREATE INDEX idx_og_objects_owner_computer_kind ON og_objects(owner_id, computer_id, object_kind, canonical_id);

-- index idx_og_objects_kind_owner_computer
CREATE INDEX idx_og_objects_kind_owner_computer ON og_objects(object_kind, owner_id, computer_id, canonical_id);

-- index idx_og_objects_kind_owner_updated
CREATE INDEX idx_og_objects_kind_owner_updated ON og_objects(object_kind, owner_id, updated_at);

-- index idx_og_objects_og_meta_run_id
CREATE INDEX idx_og_objects_og_meta_run_id ON og_objects(object_kind, og_meta_run_id, canonical_id) WHERE og_meta_run_id IS NOT NULL;

-- index idx_og_objects_og_meta_run_id_recent
CREATE INDEX idx_og_objects_og_meta_run_id_recent ON og_objects(object_kind, og_meta_run_id, updated_at) WHERE og_meta_run_id IS NOT NULL;

-- index idx_og_objects_og_meta_trajectory_id
CREATE INDEX idx_og_objects_og_meta_trajectory_id ON og_objects(object_kind, og_meta_trajectory_id, canonical_id) WHERE og_meta_trajectory_id IS NOT NULL;

-- index idx_og_objects_og_meta_trajectory_id_recent
CREATE INDEX idx_og_objects_og_meta_trajectory_id_recent ON og_objects(object_kind, og_meta_trajectory_id, updated_at) WHERE og_meta_trajectory_id IS NOT NULL;

-- index idx_og_objects_og_meta_channel_id
CREATE INDEX idx_og_objects_og_meta_channel_id ON og_objects(object_kind, og_meta_channel_id, canonical_id) WHERE og_meta_channel_id IS NOT NULL;

-- index idx_og_objects_og_meta_channel_id_recent
CREATE INDEX idx_og_objects_og_meta_channel_id_recent ON og_objects(object_kind, og_meta_channel_id, updated_at) WHERE og_meta_channel_id IS NOT NULL;

-- index idx_og_objects_og_rec_inbox
CREATE INDEX idx_og_objects_og_rec_inbox ON og_objects(object_kind, og_rec_inbox, canonical_id) WHERE og_rec_inbox IS NOT NULL;

-- index idx_og_objects_og_rec_sender_inbox
CREATE INDEX idx_og_objects_og_rec_sender_inbox ON og_objects(object_kind, og_rec_sender_inbox, canonical_id) WHERE og_rec_sender_inbox IS NOT NULL;

-- index idx_og_objects_og_rec_conversation_id
CREATE INDEX idx_og_objects_og_rec_conversation_id ON og_objects(object_kind, og_rec_conversation_id, canonical_id) WHERE og_rec_conversation_id IS NOT NULL;

-- index idx_og_objects_rec_inbox_seq
CREATE INDEX idx_og_objects_rec_inbox_seq ON og_objects(object_kind, og_rec_inbox, og_rec_inbox_seq) WHERE og_rec_inbox_seq IS NOT NULL;

-- index idx_og_objects_rec_claim
CREATE INDEX idx_og_objects_rec_claim ON og_objects(object_kind, owner_id, og_rec_claim, updated_at, canonical_id) WHERE og_rec_claim IS NOT NULL;

-- index idx_og_edges_from
CREATE INDEX idx_og_edges_from ON og_edges(from_id);

-- index idx_og_edges_to
CREATE INDEX idx_og_edges_to ON og_edges(to_id);

-- index idx_og_objects_choir_event_updated
CREATE INDEX idx_og_objects_choir_event_updated ON og_objects(updated_at) WHERE object_kind = 'choir.event';

-- index idx_og_objects_choir_event_kind_updated
CREATE INDEX idx_og_objects_choir_event_kind_updated ON og_objects((CASE WHEN json_valid(metadata) THEN json_extract(metadata, '$.kind') END), updated_at) WHERE object_kind = 'choir.event';

-- index idx_og_objects_choir_event_phase_updated
CREATE INDEX idx_og_objects_choir_event_phase_updated ON og_objects((CASE WHEN json_valid(metadata) THEN json_extract(metadata, '$.kind') END), (COALESCE(
		CASE WHEN json_valid(CAST(body AS TEXT)) THEN NULLIF(CAST(json_extract(CAST(body AS TEXT), '$.payload.phase') AS TEXT), '') END,
		CASE WHEN json_valid(metadata) THEN NULLIF(CAST(json_extract(metadata, '$.phase') AS TEXT), '') END,
		'')), updated_at) WHERE object_kind = 'choir.event';

-- index idx_og_key_index_lookup
CREATE INDEX idx_og_key_index_lookup ON og_key_index(key_name, key_value, canonical_id);

-- index idx_og_key_index_recent
CREATE INDEX idx_og_key_index_recent ON og_key_index(key_name, key_value, owner_id, updated_at, canonical_id);

-- index idx_og_key_index_bad
CREATE INDEX idx_og_key_index_bad ON og_key_index(key_name) WHERE derive_ok = 0;

