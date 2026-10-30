package platform

import (
	"context"
	"fmt"
	"strings"
)

// corpusSchemaDDL holds Store B (world-wire/corpus) tables. They bootstrap
// into the corpus database (corpusDB), which is a separate sql-server after
// the authority split.
//
// Cut 2 (2b) removed the DDL of 21 relational publication, provenance,
// consent and retrieval tables that no code read or wrote (dead-code census
// 2026-10-12 §4.1). Removing a CREATE TABLE IF NOT EXISTS drops nothing:
// existing corpus databases keep those tables as they are.
const corpusSchemaDDL = `
CREATE TABLE IF NOT EXISTS publication_transclusions (
	transclusion_id VARCHAR(255) PRIMARY KEY,
	publication_version_id VARCHAR(255) NOT NULL,
	source_entity_id VARCHAR(255) NOT NULL,
	host_selector_json LONGTEXT NOT NULL DEFAULT '{}',
	source_selector_json LONGTEXT NOT NULL DEFAULT '{}',
	relation_type VARCHAR(128) NOT NULL DEFAULT '',
	default_display_mode VARCHAR(128) NOT NULL DEFAULT '',
	snapshot_text LONGTEXT NOT NULL DEFAULT '',
	content_hash VARCHAR(128) NOT NULL DEFAULT '',
	access_policy_json LONGTEXT NOT NULL DEFAULT '{}',
	export_policy_json LONGTEXT NOT NULL DEFAULT '{}',
	entity_json LONGTEXT NOT NULL DEFAULT '{}',
	created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS publication_version_proposals (
	proposal_id VARCHAR(255) PRIMARY KEY,
	publication_id VARCHAR(255) NOT NULL,
	publication_version_id VARCHAR(255) NOT NULL,
	source_owner_id VARCHAR(255) NOT NULL,
	submitter_id VARCHAR(255) NOT NULL,
	submitter_doc_id VARCHAR(255) NOT NULL,
	submitter_revision_id VARCHAR(255) NOT NULL,
	submitter_revision_hash VARCHAR(128) NOT NULL,
	content_hash VARCHAR(128) NOT NULL,
	projection_hash VARCHAR(128) NOT NULL,
	artifact_manifest_id VARCHAR(255) NOT NULL,
	title LONGTEXT NOT NULL,
	transclusions_json LONGTEXT NOT NULL DEFAULT '[]',
	citations_json LONGTEXT NOT NULL DEFAULT '[]',
	state VARCHAR(64) NOT NULL,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS proposal_delivery_records (
	delivery_id VARCHAR(255) PRIMARY KEY,
	proposal_id VARCHAR(255) NOT NULL,
	target_owner_id VARCHAR(255) NOT NULL,
	target_kind VARCHAR(64) NOT NULL,
	target_id VARCHAR(255) NOT NULL,
	delivery_state VARCHAR(64) NOT NULL,
	delivery_ref LONGTEXT NOT NULL DEFAULT '',
	error LONGTEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS platform_texture_documents (
	doc_id VARCHAR(255) NOT NULL,
	owner_id VARCHAR(255) NOT NULL,
	title LONGTEXT NOT NULL,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	PRIMARY KEY (doc_id)
);

CREATE TABLE IF NOT EXISTS platform_texture_revisions (
	revision_id VARCHAR(255) NOT NULL,
	doc_id VARCHAR(255) NOT NULL,
	owner_id VARCHAR(255) NOT NULL,
	parent_revision_id VARCHAR(255) NOT NULL DEFAULT '',
	author_kind VARCHAR(64) NOT NULL DEFAULT '',
	author_label VARCHAR(255) NOT NULL DEFAULT '',
	content LONGTEXT NOT NULL,
	body_doc LONGTEXT NOT NULL DEFAULT '',
	source_entities LONGTEXT NOT NULL DEFAULT '',
	citations LONGTEXT NOT NULL DEFAULT '[]',
	metadata LONGTEXT NOT NULL DEFAULT '{}',
	created_at DATETIME NOT NULL,
	PRIMARY KEY (revision_id),
	INDEX idx_platform_texture_revisions_doc (doc_id, created_at)
);

CREATE TABLE IF NOT EXISTS og_objects (
	canonical_id  VARCHAR(255) NOT NULL PRIMARY KEY,
	object_kind   VARCHAR(128) NOT NULL,
	owner_id      VARCHAR(255) NOT NULL,
	computer_id   VARCHAR(255) NOT NULL DEFAULT '',
	version_id    VARCHAR(255) NOT NULL DEFAULT '',
	content_hash  VARCHAR(128) NOT NULL,
	body          LONGBLOB,
	body_ref      VARCHAR(512) NOT NULL DEFAULT '',
	body_size     BIGINT NOT NULL DEFAULT 0,
	metadata      LONGTEXT NOT NULL,
	created_at    DATETIME NOT NULL,
	updated_at    DATETIME NOT NULL,
	tombstone     BOOLEAN NOT NULL DEFAULT FALSE,
	superseded_by VARCHAR(255) NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_og_objects_kind_owner ON og_objects(object_kind, owner_id);
CREATE INDEX IF NOT EXISTS idx_og_objects_updated ON og_objects(updated_at);

CREATE TABLE IF NOT EXISTS og_edges (
	edge_id    VARCHAR(255) NOT NULL PRIMARY KEY,
	from_id    VARCHAR(255) NOT NULL,
	to_id      VARCHAR(255) NOT NULL,
	kind       VARCHAR(128) NOT NULL,
	metadata   LONGTEXT NOT NULL,
	created_at DATETIME NOT NULL,
	tombstone  BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE INDEX IF NOT EXISTS idx_og_edges_from ON og_edges(from_id);
CREATE INDEX IF NOT EXISTS idx_og_edges_to ON og_edges(to_id);
`

// schemaDDL holds Store A (canonical event/control) tables that bootstrap
// into the primary database.
func (s *Store) ensureCorpusColumn(ctx context.Context, table, name, ddl string) error {
	for _, value := range []string{table, name} {
		if value == "" || strings.IndexFunc(value, func(r rune) bool {
			return r != '_' && (r < 'a' || r > 'z')
		}) >= 0 {
			return fmt.Errorf("unsupported schema identifier %q", value)
		}
	}
	rows, err := s.corpus().QueryContext(ctx, "SHOW COLUMNS FROM "+table+" LIKE '"+name+"'")
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return rows.Err()
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.corpus().ExecContext(ctx, ddl)
	return err
}

// markDirty records a mutation for the debounced snapshot committer. SQL
// COMMIT already made the write durable; the DOLT_COMMIT snapshot is
// bookkeeping that no longer runs per mutation (the per-mutation commits
// grew the store to 6.9M commits / 218G of uncollectible history — see
// docs/evidence/platform-dolt-oldgen-218g-dead-history-2026-08-26.md).
