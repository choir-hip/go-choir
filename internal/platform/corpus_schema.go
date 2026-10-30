package platform

// corpusSchemaDDL holds Store B (the Autopaper object graph and its
// publication tables) as one clean PostgreSQL schema for the `graph`
// database. Types follow the data-system build sheet (decision 5): TEXT,
// BIGINT, BYTEA, TIMESTAMPTZ. JSON is stored as TEXT, byte-exact: objects
// carry hashes and signatures computed over those bytes, so JSONB (which
// rewrites key order and whitespace) is not used. A query that filters by a
// JSON path casts in the query (metadata::jsonb).
//
// og_objects.canonical_id is COLLATE "C": the capture backfill and the
// keyset pages compare it as a byte-ordered prefix range
// (canonical_id >= 'obj:choir.web_capture:' AND canonical_id < '...;'), which
// a locale collation does not honour.
//
// The retrieval tables (og_lookup_keys, proj_capture_index,
// proj_capture_terms) and the two retrieval indexes are created by
// MigrateRetrieval (retrieval_schema.go), not here: they are rebuilt from
// og_objects and are switched on once live.
const corpusSchemaDDL = `
CREATE TABLE IF NOT EXISTS publication_transclusions (
	transclusion_id TEXT PRIMARY KEY,
	publication_version_id TEXT NOT NULL,
	source_entity_id TEXT NOT NULL,
	host_selector_json TEXT NOT NULL DEFAULT '{}',
	source_selector_json TEXT NOT NULL DEFAULT '{}',
	relation_type TEXT NOT NULL DEFAULT '',
	default_display_mode TEXT NOT NULL DEFAULT '',
	snapshot_text TEXT NOT NULL DEFAULT '',
	content_hash TEXT NOT NULL DEFAULT '',
	access_policy_json TEXT NOT NULL DEFAULT '{}',
	export_policy_json TEXT NOT NULL DEFAULT '{}',
	entity_json TEXT NOT NULL DEFAULT '{}',
	created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS publication_version_proposals (
	proposal_id TEXT PRIMARY KEY,
	publication_id TEXT NOT NULL,
	publication_version_id TEXT NOT NULL,
	source_owner_id TEXT NOT NULL,
	submitter_id TEXT NOT NULL,
	submitter_doc_id TEXT NOT NULL,
	submitter_revision_id TEXT NOT NULL,
	submitter_revision_hash TEXT NOT NULL,
	content_hash TEXT NOT NULL,
	projection_hash TEXT NOT NULL,
	artifact_manifest_id TEXT NOT NULL,
	title TEXT NOT NULL,
	transclusions_json TEXT NOT NULL DEFAULT '[]',
	citations_json TEXT NOT NULL DEFAULT '[]',
	state TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS proposal_delivery_records (
	delivery_id TEXT PRIMARY KEY,
	proposal_id TEXT NOT NULL,
	target_owner_id TEXT NOT NULL,
	target_kind TEXT NOT NULL,
	target_id TEXT NOT NULL,
	delivery_state TEXT NOT NULL,
	delivery_ref TEXT NOT NULL DEFAULT '',
	error TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS platform_texture_documents (
	doc_id TEXT NOT NULL,
	owner_id TEXT NOT NULL,
	title TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL,
	PRIMARY KEY (doc_id)
);

CREATE TABLE IF NOT EXISTS platform_texture_revisions (
	revision_id TEXT NOT NULL,
	doc_id TEXT NOT NULL,
	owner_id TEXT NOT NULL,
	parent_revision_id TEXT NOT NULL DEFAULT '',
	author_kind TEXT NOT NULL DEFAULT '',
	author_label TEXT NOT NULL DEFAULT '',
	content TEXT NOT NULL,
	body_doc TEXT NOT NULL DEFAULT '',
	source_entities TEXT NOT NULL DEFAULT '',
	citations TEXT NOT NULL DEFAULT '[]',
	metadata TEXT NOT NULL DEFAULT '{}',
	created_at TIMESTAMPTZ NOT NULL,
	PRIMARY KEY (revision_id)
);
CREATE INDEX IF NOT EXISTS idx_platform_texture_revisions_doc ON platform_texture_revisions(doc_id, created_at);

CREATE TABLE IF NOT EXISTS og_objects (
	canonical_id  TEXT COLLATE "C" NOT NULL PRIMARY KEY,
	object_kind   TEXT NOT NULL,
	owner_id      TEXT NOT NULL,
	computer_id   TEXT NOT NULL DEFAULT '',
	version_id    TEXT NOT NULL DEFAULT '',
	content_hash  TEXT NOT NULL,
	body          BYTEA,
	body_ref      TEXT NOT NULL DEFAULT '',
	body_size     BIGINT NOT NULL DEFAULT 0,
	metadata      TEXT NOT NULL,
	created_at    TIMESTAMPTZ NOT NULL,
	updated_at    TIMESTAMPTZ NOT NULL,
	tombstone     BOOLEAN NOT NULL DEFAULT FALSE,
	superseded_by TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_og_objects_kind_owner ON og_objects(object_kind, owner_id);
CREATE INDEX IF NOT EXISTS idx_og_objects_updated ON og_objects(updated_at);

CREATE TABLE IF NOT EXISTS og_edges (
	edge_id    TEXT NOT NULL PRIMARY KEY,
	from_id    TEXT NOT NULL,
	to_id      TEXT NOT NULL,
	kind       TEXT NOT NULL,
	metadata   TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	tombstone  BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE INDEX IF NOT EXISTS idx_og_edges_from ON og_edges(from_id);
CREATE INDEX IF NOT EXISTS idx_og_edges_to ON og_edges(to_id);
`
