package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type Store struct {
	db *sql.DB
}

const schemaDDL = `
CREATE TABLE IF NOT EXISTS platform_subjects (
	subject_id VARCHAR(255) PRIMARY KEY,
	subject_kind VARCHAR(64) NOT NULL,
	display_name LONGTEXT NOT NULL DEFAULT '',
	canonical_uri LONGTEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS publication_proposals (
	proposal_id VARCHAR(255) PRIMARY KEY,
	owner_id VARCHAR(255) NOT NULL,
	source_computer_id VARCHAR(255) NOT NULL DEFAULT '',
	source_doc_id VARCHAR(255) NOT NULL,
	source_revision_id VARCHAR(255) NOT NULL,
	source_revision_hash VARCHAR(128) NOT NULL,
	projection_hash VARCHAR(128) NOT NULL,
	title LONGTEXT NOT NULL,
	visibility VARCHAR(64) NOT NULL DEFAULT 'public',
	license VARCHAR(255) NOT NULL DEFAULT '',
	state VARCHAR(64) NOT NULL,
	created_by VARCHAR(255) NOT NULL DEFAULT '',
	created_trace_id VARCHAR(255) NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS publications (
	publication_id VARCHAR(255) PRIMARY KEY,
	owner_id VARCHAR(255) NOT NULL,
	handle VARCHAR(255) NOT NULL DEFAULT '',
	slug VARCHAR(512) NOT NULL,
	title LONGTEXT NOT NULL,
	state VARCHAR(64) NOT NULL,
	latest_version_id VARCHAR(255) NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS publication_versions (
	publication_version_id VARCHAR(255) PRIMARY KEY,
	publication_id VARCHAR(255) NOT NULL,
	proposal_id VARCHAR(255) NOT NULL,
	edition_label VARCHAR(255) NOT NULL DEFAULT 'v1',
	source_doc_id VARCHAR(255) NOT NULL,
	source_revision_id VARCHAR(255) NOT NULL,
	source_revision_hash VARCHAR(128) NOT NULL,
	projection_hash VARCHAR(128) NOT NULL,
	content_hash VARCHAR(128) NOT NULL,
	artifact_manifest_id VARCHAR(255) NOT NULL,
	published_at DATETIME NOT NULL,
	supersedes_version_id VARCHAR(255) NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS public_routes (
	route_id VARCHAR(255) PRIMARY KEY,
	handle VARCHAR(255) NOT NULL DEFAULT '',
	route_path VARCHAR(1024) NOT NULL,
	target_kind VARCHAR(64) NOT NULL,
	target_id VARCHAR(255) NOT NULL,
	target_version_id VARCHAR(255) NOT NULL DEFAULT '',
	state VARCHAR(64) NOT NULL,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_public_routes_path ON public_routes(route_path);

CREATE TABLE IF NOT EXISTS artifact_manifests (
	artifact_manifest_id VARCHAR(255) PRIMARY KEY,
	subject_kind VARCHAR(64) NOT NULL,
	subject_id VARCHAR(255) NOT NULL,
	media_type VARCHAR(255) NOT NULL,
	manifest_hash VARCHAR(128) NOT NULL,
	manifest_json LONGTEXT NOT NULL,
	created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS artifact_blobs (
	blob_id VARCHAR(255) PRIMARY KEY,
	artifact_manifest_id VARCHAR(255) NOT NULL,
	content_hash VARCHAR(128) NOT NULL,
	hash_algorithm VARCHAR(64) NOT NULL,
	media_type VARCHAR(255) NOT NULL,
	byte_size BIGINT NOT NULL,
	storage_ref LONGTEXT NOT NULL,
	created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS provenance_entities (
	entity_id VARCHAR(255) PRIMARY KEY,
	entity_kind VARCHAR(64) NOT NULL,
	content_hash VARCHAR(128) NOT NULL DEFAULT '',
	canonical_uri LONGTEXT NOT NULL DEFAULT '',
	metadata_json LONGTEXT NOT NULL DEFAULT '{}',
	created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS provenance_activities (
	activity_id VARCHAR(255) PRIMARY KEY,
	activity_kind VARCHAR(64) NOT NULL,
	trace_id VARCHAR(255) NOT NULL DEFAULT '',
	run_id VARCHAR(255) NOT NULL DEFAULT '',
	started_at DATETIME NOT NULL,
	ended_at DATETIME,
	metadata_json LONGTEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS provenance_agents (
	agent_ref_id VARCHAR(255) PRIMARY KEY,
	agent_kind VARCHAR(64) NOT NULL,
	subject_id VARCHAR(255) NOT NULL DEFAULT '',
	model VARCHAR(255) NOT NULL DEFAULT '',
	provider VARCHAR(255) NOT NULL DEFAULT '',
	vm_id VARCHAR(255) NOT NULL DEFAULT '',
	metadata_json LONGTEXT NOT NULL DEFAULT '{}',
	created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS provenance_edges (
	edge_id VARCHAR(255) PRIMARY KEY,
	edge_kind VARCHAR(64) NOT NULL,
	from_id VARCHAR(255) NOT NULL,
	to_id VARCHAR(255) NOT NULL,
	activity_id VARCHAR(255) NOT NULL DEFAULT '',
	metadata_json LONGTEXT NOT NULL DEFAULT '{}',
	created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS consent_records (
	consent_id VARCHAR(255) PRIMARY KEY,
	subject_id VARCHAR(255) NOT NULL,
	target_kind VARCHAR(64) NOT NULL,
	target_id VARCHAR(255) NOT NULL,
	action VARCHAR(64) NOT NULL,
	state VARCHAR(64) NOT NULL,
	evidence_ref LONGTEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS review_records (
	review_id VARCHAR(255) PRIMARY KEY,
	target_kind VARCHAR(64) NOT NULL,
	target_id VARCHAR(255) NOT NULL,
	reviewer_subject_id VARCHAR(255) NOT NULL,
	decision VARCHAR(64) NOT NULL,
	body LONGTEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS retrieval_sources (
	source_id VARCHAR(255) PRIMARY KEY,
	source_kind VARCHAR(64) NOT NULL,
	canonical_uri LONGTEXT NOT NULL DEFAULT '',
	content_hash VARCHAR(128) NOT NULL,
	license VARCHAR(255) NOT NULL DEFAULT '',
	visibility VARCHAR(64) NOT NULL DEFAULT 'public',
	state VARCHAR(64) NOT NULL,
	created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS retrieval_spans (
	span_id VARCHAR(255) PRIMARY KEY,
	source_id VARCHAR(255) NOT NULL,
	source_version_id VARCHAR(255) NOT NULL,
	selector_kind VARCHAR(64) NOT NULL,
	selector_json LONGTEXT NOT NULL DEFAULT '{}',
	text_hash VARCHAR(128) NOT NULL DEFAULT '',
	chunk_hash VARCHAR(128) NOT NULL DEFAULT '',
	token_count BIGINT NOT NULL DEFAULT 0,
	metadata_json LONGTEXT NOT NULL DEFAULT '{}',
	created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS retrieval_manifests (
	retrieval_manifest_id VARCHAR(255) PRIMARY KEY,
	output_kind VARCHAR(64) NOT NULL,
	output_id VARCHAR(255) NOT NULL,
	query_or_objective_hash VARCHAR(128) NOT NULL DEFAULT '',
	index_manifest_id VARCHAR(255) NOT NULL DEFAULT '',
	selected_refs_json LONGTEXT NOT NULL DEFAULT '[]',
	rejected_refs_json LONGTEXT NOT NULL DEFAULT '[]',
	created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS citation_edges (
	citation_id VARCHAR(255) PRIMARY KEY,
	from_kind VARCHAR(64) NOT NULL,
	from_id VARCHAR(255) NOT NULL,
	from_selector_json LONGTEXT NOT NULL DEFAULT '{}',
	to_kind VARCHAR(64) NOT NULL,
	to_id LONGTEXT NOT NULL,
	to_selector_json LONGTEXT NOT NULL DEFAULT '{}',
	relation_type VARCHAR(64) NOT NULL,
	state VARCHAR(64) NOT NULL,
	proposed_by VARCHAR(255) NOT NULL DEFAULT '',
	accepted_by VARCHAR(255) NOT NULL DEFAULT '',
	evidence_ref LONGTEXT NOT NULL DEFAULT '',
	confidence DOUBLE NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS publication_source_entities (
	entity_record_id VARCHAR(255) PRIMARY KEY,
	publication_version_id VARCHAR(255) NOT NULL,
	source_entity_id VARCHAR(255) NOT NULL,
	kind VARCHAR(128) NOT NULL DEFAULT '',
	target_kind VARCHAR(128) NOT NULL DEFAULT '',
	target_id LONGTEXT NOT NULL DEFAULT '',
	display_policy VARCHAR(128) NOT NULL DEFAULT '',
	open_surface VARCHAR(128) NOT NULL DEFAULT '',
	entity_json LONGTEXT NOT NULL DEFAULT '{}',
	created_at DATETIME NOT NULL
);

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

CREATE TABLE IF NOT EXISTS publication_policies (
	policy_id VARCHAR(255) PRIMARY KEY,
	publication_version_id VARCHAR(255) NOT NULL,
	access_policy_json LONGTEXT NOT NULL DEFAULT '{}',
	export_policy_json LONGTEXT NOT NULL DEFAULT '{}',
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

CREATE TABLE IF NOT EXISTS verifier_attestations (
	attestation_id VARCHAR(255) PRIMARY KEY,
	target_kind VARCHAR(64) NOT NULL,
	target_id VARCHAR(255) NOT NULL,
	verifier_id VARCHAR(255) NOT NULL,
	verifier_kind VARCHAR(64) NOT NULL,
	result VARCHAR(64) NOT NULL,
	subject_digest VARCHAR(128) NOT NULL,
	predicate_type VARCHAR(255) NOT NULL,
	evidence_json LONGTEXT NOT NULL DEFAULT '{}',
	created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS rollback_refs (
	rollback_id VARCHAR(255) PRIMARY KEY,
	target_kind VARCHAR(64) NOT NULL,
	target_id VARCHAR(255) NOT NULL,
	rollback_kind VARCHAR(64) NOT NULL,
	ref LONGTEXT NOT NULL,
	expires_at DATETIME,
	created_at DATETIME NOT NULL
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
`

func OpenStore(dsn string) (*Store, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("platform store: open mysql: %w", err)
	}
	configureDB(db)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("platform store: ping mysql: %w", err)
	}
	s := NewStore(db)
	if err := s.Bootstrap(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func NewStore(db *sql.DB) *Store {
	configureDB(db)
	return &Store{db: db}
}

func configureDB(db *sql.DB) {
	if db == nil {
		return
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("platform store: nil database")
	}
	return s.db.PingContext(ctx)
}

func (s *Store) Bootstrap(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("platform store: nil database")
	}
	if _, err := s.db.ExecContext(ctx, schemaDDL); err != nil {
		return fmt.Errorf("platform store: bootstrap schema: %w", err)
	}
	if err := s.ensurePlatformTextureRevisionColumn(ctx, "body_doc", "ALTER TABLE platform_texture_revisions ADD COLUMN body_doc LONGTEXT NOT NULL DEFAULT '' AFTER content"); err != nil {
		return fmt.Errorf("platform store: bootstrap body_doc migration: %w", err)
	}
	if err := s.ensurePlatformTextureRevisionColumn(ctx, "source_entities", "ALTER TABLE platform_texture_revisions ADD COLUMN source_entities LONGTEXT NOT NULL DEFAULT '' AFTER body_doc"); err != nil {
		return fmt.Errorf("platform store: bootstrap source_entities migration: %w", err)
	}
	return nil
}

func (s *Store) ensurePlatformTextureRevisionColumn(ctx context.Context, name, ddl string) error {
	if name != "body_doc" && name != "source_entities" {
		return fmt.Errorf("unsupported platform texture revision column %q", name)
	}
	rows, err := s.db.QueryContext(ctx, "SHOW COLUMNS FROM platform_texture_revisions LIKE '"+name+"'")
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
	_, err = s.db.ExecContext(ctx, ddl)
	return err
}

func (s *Store) commitDolt(ctx context.Context, message string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("platform store: nil database")
	}
	if message == "" {
		message = "platform change"
	}
	if _, err := s.db.ExecContext(ctx, "CALL DOLT_COMMIT('-Am', ?)", message); err != nil {
		return fmt.Errorf("platform store: dolt commit: %w", err)
	}
	return nil
}

func (s *Store) UpsertTextureDocument(ctx context.Context, docID, ownerID, title string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO platform_texture_documents (doc_id, owner_id, title, created_at, updated_at) VALUES (?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE title=VALUES(title), updated_at=VALUES(updated_at)`,
		docID, ownerID, title, now, now)
	return err
}

func (s *Store) UpsertTextureRevision(ctx context.Context, rev PlatformTextureRevision) error {
	now := time.Now().UTC()
	if rev.CreatedAt.IsZero() {
		rev.CreatedAt = now
	}
	if rev.Citations == nil {
		rev.Citations = json.RawMessage("[]")
	}
	if rev.Metadata == nil {
		rev.Metadata = json.RawMessage("{}")
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO platform_texture_revisions (revision_id, doc_id, owner_id, parent_revision_id, author_kind, author_label, content, body_doc, source_entities, citations, metadata, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE content=VALUES(content), body_doc=VALUES(body_doc), source_entities=VALUES(source_entities), citations=VALUES(citations), metadata=VALUES(metadata)`,
		rev.RevisionID, rev.DocID, rev.OwnerID, rev.ParentRevisionID, rev.AuthorKind, rev.AuthorLabel, rev.Content, string(rev.BodyDoc), string(rev.SourceEntities), string(rev.Citations), string(rev.Metadata), rev.CreatedAt)
	return err
}

func (s *Store) GetTextureDocument(ctx context.Context, docID string) (*PlatformTextureDocument, error) {
	var doc PlatformTextureDocument
	err := s.db.QueryRowContext(ctx,
		`SELECT d.doc_id, d.owner_id, d.title, COALESCE((SELECT r.revision_id FROM platform_texture_revisions r WHERE r.doc_id = d.doc_id ORDER BY r.created_at DESC, r.revision_id DESC LIMIT 1), '') FROM platform_texture_documents d WHERE d.doc_id = ?`, docID).Scan(&doc.DocID, &doc.OwnerID, &doc.Title, &doc.CurrentRevisionID)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func (s *Store) ListTextureRevisions(ctx context.Context, docID string) ([]PlatformTextureRevision, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT revision_id, doc_id, owner_id, parent_revision_id, author_kind, author_label, content, body_doc, source_entities, citations, metadata, created_at FROM platform_texture_revisions WHERE doc_id = ? ORDER BY created_at ASC`, docID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var revisions []PlatformTextureRevision
	for rows.Next() {
		var rev PlatformTextureRevision
		var bodyDocStr, sourceEntitiesStr, citationsStr, metadataStr string
		if err := rows.Scan(&rev.RevisionID, &rev.DocID, &rev.OwnerID, &rev.ParentRevisionID, &rev.AuthorKind, &rev.AuthorLabel, &rev.Content, &bodyDocStr, &sourceEntitiesStr, &citationsStr, &metadataStr, &rev.CreatedAt); err != nil {
			return nil, err
		}
		rev.BodyDoc = json.RawMessage(bodyDocStr)
		rev.SourceEntities = json.RawMessage(sourceEntitiesStr)
		rev.Citations = json.RawMessage(citationsStr)
		rev.Metadata = json.RawMessage(metadataStr)
		revisions = append(revisions, rev)
	}
	return revisions, rows.Err()
}

func (s *Store) GetTextureRevision(ctx context.Context, revisionID string) (*PlatformTextureRevision, error) {
	var rev PlatformTextureRevision
	var bodyDocStr, sourceEntitiesStr, citationsStr, metadataStr string
	err := s.db.QueryRowContext(ctx,
		`SELECT revision_id, doc_id, owner_id, parent_revision_id, author_kind, author_label, content, body_doc, source_entities, citations, metadata, created_at FROM platform_texture_revisions WHERE revision_id = ?`, revisionID).Scan(&rev.RevisionID, &rev.DocID, &rev.OwnerID, &rev.ParentRevisionID, &rev.AuthorKind, &rev.AuthorLabel, &rev.Content, &bodyDocStr, &sourceEntitiesStr, &citationsStr, &metadataStr, &rev.CreatedAt)
	if err != nil {
		return nil, err
	}
	rev.BodyDoc = json.RawMessage(bodyDocStr)
	rev.SourceEntities = json.RawMessage(sourceEntitiesStr)
	rev.Citations = json.RawMessage(citationsStr)
	rev.Metadata = json.RawMessage(metadataStr)
	return &rev, nil
}
