package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/dbcred"
	"github.com/yusefmosiah/go-choir/internal/dbx"
)

type Store struct {
	db *sql.DB
	// blockedLogged is when reconcile last logged each blocked projection job.
	blockedLogMu  sync.Mutex
	blockedLogged map[string]time.Time
	// corpusDB serves Store B (world-wire/corpus) tables: og_*, platform
	// texture mirrors, and the publication/provenance/artifact domain. It is
	// the same pool as db until the authority split deploys a separate
	// corpus DSN; corpus() returns whichever is configured.
	corpusDB *sql.DB
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

const schemaDDL = `
CREATE TABLE IF NOT EXISTS computer_key_escrows (
	computer_id VARCHAR(255) NOT NULL,
	protector VARCHAR(64) NOT NULL,
	wrap_version INT NOT NULL,
	wrapped_key_json TEXT NOT NULL,
	key_digest VARCHAR(128) NOT NULL,
	escrowed_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL,
	PRIMARY KEY(computer_id, protector)
);

CREATE TABLE IF NOT EXISTS computer_key_unwrap_requests (
	request_id VARCHAR(255) PRIMARY KEY,
	computer_id VARCHAR(255) NOT NULL,
	requested_by VARCHAR(255) NOT NULL,
	reason TEXT NOT NULL,
	status VARCHAR(32) NOT NULL,
	idempotency_key VARCHAR(128) NOT NULL UNIQUE,
	created_at TIMESTAMPTZ NOT NULL,
	revealed_at TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS computer_key_unwrap_approvals (
	request_id VARCHAR(255) NOT NULL,
	approver VARCHAR(255) NOT NULL,
	approved_at TIMESTAMPTZ NOT NULL,
	PRIMARY KEY(request_id, approver)
);

CREATE TABLE IF NOT EXISTS computer_key_escrow_transparency (
	seq BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	prev_hash VARCHAR(128) NOT NULL,
	entry_hash VARCHAR(128) NOT NULL,
	payload_json TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL
);

`

// OpenStore opens the platform store on PostgreSQL. controlDSN is the
// `control` database (Store A: events, checkpoints, routes, escrows);
// graphDSN is the `graph` database (Store B: object graph, publication,
// provenance). Both are libpq DSNs; the password comes from dbcred and is
// never logged.
//
// ctx bounds the whole open: every ping and every schema statement. A caller
// that waits on a server that accepts connections but never answers gets an
// error when ctx expires instead of hanging. ctx only bounds the open; the
// returned Store does not keep it.
func OpenStore(ctx context.Context, controlDSN, graphDSN string) (*Store, error) {
	return openStore(ctx, controlDSN, graphDSN, true)
}

// OpenStoreWithoutBootstrap is OpenStore for a reader/maintainer that does not
// own the schema: it connects and pings both pools but applies no DDL. The
// owning service (corpusd) bootstraps.
func OpenStoreWithoutBootstrap(ctx context.Context, controlDSN, graphDSN string) (*Store, error) {
	return openStore(ctx, controlDSN, graphDSN, false)
}

func openStore(ctx context.Context, controlDSN, graphDSN string, bootstrap bool) (*Store, error) {
	controlDSN, err := dbcred.ApplyPGDSN(controlDSN)
	if err != nil {
		return nil, fmt.Errorf("platform store: %w", err)
	}
	graphDSN, err = dbcred.ApplyPGDSN(graphDSN)
	if err != nil {
		return nil, fmt.Errorf("platform store: graph: %w", err)
	}
	db, err := openPG(ctx, controlDSN)
	if err != nil {
		return nil, fmt.Errorf("platform store: open control: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("platform store: ping control: %w", err)
	}
	graphDB, err := openPG(ctx, graphDSN)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("platform store: open graph: %w", err)
	}
	if err := graphDB.PingContext(ctx); err != nil {
		_ = graphDB.Close()
		_ = db.Close()
		return nil, fmt.Errorf("platform store: ping graph: %w", err)
	}
	s := NewStore(db, graphDB)
	if bootstrap {
		if err := s.Bootstrap(ctx); err != nil {
			_ = s.Close()
			return nil, err
		}
	}
	return s, nil
}

// openPG opens a PostgreSQL pool bounded by ctx: dbx.OpenPostgres pings
// without a context, and a server that accepts connections but never answers
// (the 2026-10-15 deadlock shape) must not hang the caller past ctx.
func openPG(ctx context.Context, dsn string) (*sql.DB, error) {
	type result struct {
		db  *sql.DB
		err error
	}
	ch := make(chan result, 1)
	go func() {
		db, err := dbx.OpenPostgres(dsn, dbx.PGOptions{MaxOpenConns: 16, MaxIdleConns: 4})
		ch <- result{db, err}
	}()
	select {
	case r := <-ch:
		return r.db, r.err
	case <-ctx.Done():
		go func() {
			if r := <-ch; r.db != nil {
				_ = r.db.Close()
			}
		}()
		return nil, ctx.Err()
	}
}

// NewStore wraps an open control pool and graph pool. A nil graph pool
// serves the graph tables from the control pool (unit tests that exercise
// only control behavior).
func NewStore(db, graph *sql.DB) *Store {
	if graph == nil {
		graph = db
	}
	return &Store{db: db, corpusDB: graph,
		retrieval:      &RetrievalMetrics{since: time.Now().UTC()},
		retrievalState: retrievalSchemaState{reconcileKick: make(chan struct{}, 1)}}
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	if s.corpusDB != nil && s.corpusDB != s.db {
		_ = s.corpusDB.Close()
	}
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

// corpus returns the pool serving the graph database (Store B tables).
func (s *Store) corpus() *sql.DB {
	if s.corpusDB != nil {
		return s.corpusDB
	}
	return s.db
}

// markCorpusDirty is a no-op kept until WP-E deletes its call sites in the
// graph files (objectgraph_store.go, retrieval_schema.go, capture_index.go);
// PostgreSQL COMMIT is the durability, there is no snapshot committer.
func (s *Store) markCorpusDirty(string) {}

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
	if err := s.ensureTapeTombstoneSchema(ctx); err != nil {
		return err
	}
	if _, err := s.corpus().ExecContext(ctx, corpusSchemaDDL); err != nil {
		return fmt.Errorf("platform store: bootstrap corpus schema: %w", err)
	}
	return nil
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
