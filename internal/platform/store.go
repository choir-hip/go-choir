package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/yusefmosiah/go-choir/internal/dbcred"
	"github.com/yusefmosiah/go-choir/internal/doltbatch"
)

type Store struct {
	db *sql.DB
	// blockedLogged is when reconcile last logged each blocked projection job.
	blockedLogMu  sync.Mutex
	blockedLogged map[string]time.Time
	committer     *doltbatch.Committer
	// corpusDB serves Store B (world-wire/corpus) tables: og_*, platform
	// texture mirrors, and the publication/provenance/artifact domain. It is
	// the same pool as db until the authority split deploys a separate
	// corpus DSN; corpus() returns whichever is configured.
	corpusDB        *sql.DB
	corpusCommitter *doltbatch.Committer
	// artifactsRoot is the filesystem CAS root for externalized og_objects
	// bodies (Move 3). Empty disables externalization — bodies stay inline.
	artifactsRoot string
	// casReuseMu orders CAS object reuse against artifact GC unlinks. A
	// writer that creates, replaces or reuses (refreshes) a content-addressed
	// object holds it shared; the GC holds it exclusively only to re-check
	// and unlink one candidate, so a reused object is never deleted between
	// the sweep's live-set read and the writer's reference commit.
	casReuseMu sync.RWMutex
	// retrieval times and counts every Store B retrieval call (corpusd
	// /health "retrieval").
	retrieval *RetrievalMetrics
	// retrievalState records which retrieval indexes and projections are
	// live (retrieval_schema.go).
	retrievalState retrievalSchemaState
	// tombstones caches computer_tape_tombstones: a tombstoned computer's
	// tape reads and appends fail closed (tape_tombstone.go).
	tombstones tapeTombstoneCache
}

// SetArtifactsRoot configures the filesystem CAS root used to externalize
// og_objects bodies. Called by NewService; tests that never set it keep
// bodies inline.
func (s *Store) SetArtifactsRoot(root string) {
	if s == nil {
		return
	}
	s.artifactsRoot = filepath.Clean(root)
}

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
const schemaDDL = `
CREATE TABLE IF NOT EXISTS computer_key_escrows (
	computer_id VARCHAR(255) NOT NULL,
	protector VARCHAR(64) NOT NULL,
	wrap_version INT NOT NULL,
	wrapped_key_json LONGTEXT NOT NULL,
	key_digest VARCHAR(128) NOT NULL,
	escrowed_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	PRIMARY KEY(computer_id, protector)
);

CREATE TABLE IF NOT EXISTS computer_key_unwrap_requests (
	request_id VARCHAR(255) PRIMARY KEY,
	computer_id VARCHAR(255) NOT NULL,
	requested_by VARCHAR(255) NOT NULL,
	reason TEXT NOT NULL,
	status VARCHAR(32) NOT NULL,
	idempotency_key VARCHAR(128) NOT NULL UNIQUE,
	created_at DATETIME NOT NULL,
	revealed_at DATETIME NULL
);

CREATE TABLE IF NOT EXISTS computer_key_unwrap_approvals (
	request_id VARCHAR(255) NOT NULL,
	approver VARCHAR(255) NOT NULL,
	approved_at DATETIME NOT NULL,
	PRIMARY KEY(request_id, approver)
);

CREATE TABLE IF NOT EXISTS computer_key_escrow_transparency (
	seq BIGINT AUTO_INCREMENT PRIMARY KEY,
	prev_hash VARCHAR(128) NOT NULL,
	entry_hash VARCHAR(128) NOT NULL,
	payload_json LONGTEXT NOT NULL,
	created_at DATETIME NOT NULL
);

`

// OpenStore opens the platform store. dsn is the canonical event/control
// database (Store A); corpusDSN is the world-wire/corpus database (Store B).
// An empty corpusDSN shares the Store A pool — the pre-split topology.
//
// ctx bounds the whole open: every ping and every schema statement. A caller
// that waits on a Dolt that accepts connections but never answers (the
// 2026-10-15 deadlock) gets an error when ctx expires instead of hanging.
// ctx only bounds the open; the returned Store does not keep it.
func OpenStore(ctx context.Context, dsn, corpusDSN string) (*Store, error) {
	return openStore(ctx, dsn, corpusDSN, true)
}

// OpenStoreWithoutBootstrap is OpenStore for a reader/maintainer that does not
// own the schema: it connects and pings both pools but applies no DDL. The
// owning service (corpusd) bootstraps; a maintainer that also ran the DDL on
// every timer tick generated ~1,440 DDL batches a day against Store A and hung
// on a deadlocked server
// (docs/problems/dolt-clustering-assessment-2026-10-15.md section 7).
func OpenStoreWithoutBootstrap(ctx context.Context, dsn, corpusDSN string) (*Store, error) {
	return openStore(ctx, dsn, corpusDSN, false)
}

func openStore(ctx context.Context, dsn, corpusDSN string, bootstrap bool) (*Store, error) {
	sharedPool := strings.TrimSpace(corpusDSN) == "" || strings.TrimSpace(corpusDSN) == strings.TrimSpace(dsn)
	// The host's Dolt user and password (dbcred; never logged). Unset, the
	// DSNs are used as configured.
	dsn, err := dbcred.ApplyDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("platform store: %w", err)
	}
	if !sharedPool {
		if corpusDSN, err = dbcred.ApplyDSN(corpusDSN); err != nil {
			return nil, fmt.Errorf("platform store: corpus: %w", err)
		}
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("platform store: open mysql: %w", err)
	}
	configureDB(db)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("platform store: ping mysql: %w", err)
	}
	var corpusDB *sql.DB
	if !sharedPool {
		corpusDB, err = sql.Open("mysql", corpusDSN)
		if err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("platform store: open corpus mysql: %w", err)
		}
		configureDB(corpusDB)
		if err := corpusDB.PingContext(ctx); err != nil {
			_ = corpusDB.Close()
			_ = db.Close()
			return nil, fmt.Errorf("platform store: ping corpus mysql: %w", err)
		}
	}
	s := NewStore(db)
	if corpusDB != nil {
		s.corpusDB = corpusDB
		s.corpusCommitter = doltbatch.New(corpusDB, "platform-corpus", doltbatch.DebounceFromEnv("DOLT_SNAPSHOT_DEBOUNCE"))
	}
	s.applyStoreASnapshotEnv(os.Getenv)
	if bootstrap {
		if err := s.Bootstrap(ctx); err != nil {
			_ = s.Close()
			return nil, err
		}
	}
	return s, nil
}

// StoreASnapshotModeEnv switches Store A's DOLT_COMMIT snapshots: "commit"
// (the default) or "off". Nothing reads Store A's commit history, and every
// snapshot keeps the rows it changed in oldgen for good (docs/problems/
// platform-dolt-oldgen-rate-and-squash-window-2026-10-22.md). Store B's
// committer is not affected.
const StoreASnapshotModeEnv = "PLATFORM_DOLT_SNAPSHOTS"

// applyStoreASnapshotEnv reads StoreASnapshotModeEnv through getenv and
// applies it to the Store A committer. An unknown value keeps snapshots on and
// is logged. It returns the mode applied.
func (s *Store) applyStoreASnapshotEnv(getenv func(string) string) doltbatch.Mode {
	mode, err := doltbatch.ParseMode(getenv(StoreASnapshotModeEnv))
	if err != nil {
		log.Printf("platform store: %s: %v", StoreASnapshotModeEnv, err)
	}
	s.useStoreASnapshotMode(mode)
	if mode == doltbatch.ModeOff {
		log.Printf("platform store: Store A snapshot commits off (%s=off); SQL COMMIT is the durability", StoreASnapshotModeEnv)
	}
	return mode
}

// useStoreASnapshotMode replaces the Store A committer with one in mode.
// With a shared pool (no corpus DSN) both stores are the one database, so the
// corpus committer follows; a separate Store B committer is left as it is.
func (s *Store) useStoreASnapshotMode(mode doltbatch.Mode) {
	if s == nil || s.committer == nil || s.committer.Mode() == mode {
		return
	}
	old := s.committer
	s.committer = doltbatch.NewWithMode(s.db, "platform", doltbatch.DebounceFromEnv("DOLT_SNAPSHOT_DEBOUNCE"), mode)
	if s.corpusCommitter == old {
		s.corpusCommitter = s.committer
	}
	_ = old.Close(context.Background())
}

func NewStore(db *sql.DB) *Store {
	configureDB(db)
	committer := doltbatch.New(db, "platform", doltbatch.DebounceFromEnv("DOLT_SNAPSHOT_DEBOUNCE"))
	return &Store{db: db, committer: committer, corpusDB: db, corpusCommitter: committer,
		retrieval:      &RetrievalMetrics{since: time.Now().UTC()},
		retrievalState: retrievalSchemaState{reconcileKick: make(chan struct{}, 1)}}
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
	// Flush pending snapshot state before closing the pools.
	if s.corpusDB != nil && s.corpusDB != s.db {
		_ = s.corpusCommitter.Close(context.Background())
		_ = s.corpusDB.Close()
	}
	_ = s.committer.Close(context.Background())
	return s.db.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("platform store: nil database")
	}
	if err := s.db.PingContext(ctx); err != nil {
		return err
	}
	if s.corpusDB != nil && s.corpusDB != s.db {
		return s.corpusDB.PingContext(ctx)
	}
	return nil
}

// corpus returns the pool serving Store B tables. It equals s.db until a
// separate corpus DSN is configured.
func (s *Store) corpus() *sql.DB {
	if s.corpusDB != nil {
		return s.corpusDB
	}
	return s.db
}

// markCorpusDirty records a Store B mutation for its snapshot committer.
func (s *Store) markCorpusDirty(message string) {
	if s == nil || s.corpusCommitter == nil {
		return
	}
	s.corpusCommitter.Mark(message)
}

func (s *Store) Bootstrap(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("platform store: nil database")
	}
	if _, err := s.db.ExecContext(ctx, schemaDDL); err != nil {
		return fmt.Errorf("platform store: bootstrap schema: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, computerEventSchemaDDL); err != nil {
		return fmt.Errorf("platform store: bootstrap computer event schema: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, ownerActSchemaDDL); err != nil {
		return fmt.Errorf("platform store: bootstrap owner-act schema: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, consensusSchemaDDL); err != nil {
		return fmt.Errorf("platform store: bootstrap consensus schema: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, fileCASSchemaDDL); err != nil {
		return fmt.Errorf("platform store: bootstrap file CAS schema: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, projectionJobSchemaDDL); err != nil {
		return fmt.Errorf("platform store: bootstrap projection jobs: %w", err)
	}
	if err := s.ensureColumn(ctx, "computer_projection_jobs", "build_commit", "ALTER TABLE computer_projection_jobs ADD COLUMN build_commit VARCHAR(128) NOT NULL DEFAULT ''"); err != nil {
		return fmt.Errorf("platform store: projection job build column: %w", err)
	}
	if err := s.ensureTapeTombstoneSchema(ctx); err != nil {
		return err
	}
	if _, err := s.corpus().ExecContext(ctx, corpusSchemaDDL); err != nil {
		return fmt.Errorf("platform store: bootstrap corpus schema: %w", err)
	}
	if err := s.ensurePlatformTextureRevisionColumn(ctx, "body_doc", "ALTER TABLE platform_texture_revisions ADD COLUMN body_doc LONGTEXT NOT NULL DEFAULT '' AFTER content"); err != nil {
		return fmt.Errorf("platform store: bootstrap body_doc migration: %w", err)
	}
	if err := s.ensurePlatformTextureRevisionColumn(ctx, "source_entities", "ALTER TABLE platform_texture_revisions ADD COLUMN source_entities LONGTEXT NOT NULL DEFAULT '' AFTER body_doc"); err != nil {
		return fmt.Errorf("platform store: bootstrap source_entities migration: %w", err)
	}
	if err := s.ensureCorpusColumn(ctx, "og_objects", "body_ref", "ALTER TABLE og_objects ADD COLUMN body_ref VARCHAR(512) NOT NULL DEFAULT '' AFTER body"); err != nil {
		return fmt.Errorf("platform store: bootstrap og body_ref migration: %w", err)
	}
	if err := s.ensureCorpusColumn(ctx, "og_objects", "body_size", "ALTER TABLE og_objects ADD COLUMN body_size BIGINT NOT NULL DEFAULT 0 AFTER body_ref"); err != nil {
		return fmt.Errorf("platform store: bootstrap og body_size migration: %w", err)
	}
	if err := s.ensureColumn(ctx, "computer_self_development_modes", "expected_pending_transition_ref", "ALTER TABLE computer_self_development_modes ADD COLUMN expected_pending_transition_ref VARCHAR(255) NULL AFTER expected_effective_event_head"); err != nil {
		return fmt.Errorf("platform store: bootstrap self-development pending transition migration: %w", err)
	}
	return nil
}

func (s *Store) ensureColumn(ctx context.Context, table, name, ddl string) error {
	for _, value := range []string{table, name} {
		if value == "" || strings.IndexFunc(value, func(r rune) bool {
			return r != '_' && (r < 'a' || r > 'z')
		}) >= 0 {
			return fmt.Errorf("unsupported schema identifier %q", value)
		}
	}
	rows, err := s.db.QueryContext(ctx, "SHOW COLUMNS FROM "+table+" LIKE '"+name+"'")
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

func (s *Store) ensurePlatformTextureRevisionColumn(ctx context.Context, name, ddl string) error {
	if name != "body_doc" && name != "source_entities" {
		return fmt.Errorf("unsupported platform texture revision column %q", name)
	}
	rows, err := s.corpus().QueryContext(ctx, "SHOW COLUMNS FROM platform_texture_revisions LIKE '"+name+"'")
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

// ensureCorpusColumn applies an idempotent column migration on the corpus
// (Store B) pool. Same identifier rules as ensureColumn.
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
func (s *Store) markDirty(message string) {
	if s == nil || s.committer == nil {
		return
	}
	s.committer.Mark(message)
}

// commitBoundary synchronously snapshots the working set at a recovery
// boundary (checkpoint publish, replay watermark record). Errors propagate
// to the caller as before.
func (s *Store) commitBoundary(ctx context.Context, message string) error {
	if s == nil || s.committer == nil {
		return fmt.Errorf("platform store: nil database")
	}
	return s.committer.CommitNow(ctx, message)
}

func (s *Store) UpsertTextureDocument(ctx context.Context, docID, ownerID, title string) error {
	now := time.Now().UTC()
	_, err := s.corpus().ExecContext(ctx,
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
	_, err := s.corpus().ExecContext(ctx,
		`INSERT INTO platform_texture_revisions (revision_id, doc_id, owner_id, parent_revision_id, author_kind, author_label, content, body_doc, source_entities, citations, metadata, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE content=VALUES(content), body_doc=VALUES(body_doc), source_entities=VALUES(source_entities), citations=VALUES(citations), metadata=VALUES(metadata)`,
		rev.RevisionID, rev.DocID, rev.OwnerID, rev.ParentRevisionID, rev.AuthorKind, rev.AuthorLabel, rev.Content, string(rev.BodyDoc), string(rev.SourceEntities), string(rev.Citations), string(rev.Metadata), rev.CreatedAt)
	return err
}

func (s *Store) GetTextureDocument(ctx context.Context, docID string) (*PlatformTextureDocument, error) {
	var doc PlatformTextureDocument
	err := s.corpus().QueryRowContext(ctx,
		`SELECT d.doc_id, d.owner_id, d.title, COALESCE((SELECT r.revision_id FROM platform_texture_revisions r WHERE r.doc_id = d.doc_id ORDER BY r.created_at DESC, r.revision_id DESC LIMIT 1), '') FROM platform_texture_documents d WHERE d.doc_id = ?`, docID).Scan(&doc.DocID, &doc.OwnerID, &doc.Title, &doc.CurrentRevisionID)
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func (s *Store) ListTextureRevisions(ctx context.Context, docID string) ([]PlatformTextureRevision, error) {
	rows, err := s.corpus().QueryContext(ctx,
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
	err := s.corpus().QueryRowContext(ctx,
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
