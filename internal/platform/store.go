package platform

import (
	"context"
	"database/sql"
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
	return nil
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
