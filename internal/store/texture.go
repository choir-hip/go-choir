// Package store provides texture document persistence for the go-choir sandbox
// runtime.
//
// The texture store persists documents, revisions, citations, and metadata
// using an embedded Dolt workspace, enabling history-capable persistence with
// first-class versioning semantics, history/snapshot/diff/blame APIs, and
// per-user in-process storage inside the sandbox.
//
// Design decisions:
//   - Embedded Dolt (`github.com/dolthub/driver`) for version-native document
//     storage without a separate server process.
//   - Full-content revisions (not deltas) so that historical snapshots are
//     directly accessible without reconstruction.
//   - Citations and metadata are stored per-revision as JSON blobs so they
//     round-trip through history (VAL-ETEXT-010).
//   - Owner scoping on all queries so that one user cannot read another
//     user's documents or revisions.
//   - The diff algorithm is a simple line-based diff (LCS) that produces
//     section-level changes between two revisions.
//   - The blame algorithm walks backward through the revision chain,
//     attributing each line to the most recent revision that changed it.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v4"
	embedded "github.com/dolthub/driver"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// textureSchemaDDL creates the texture tables if they do not already exist.
const textureSchemaDDL = `
CREATE TABLE IF NOT EXISTS texture_documents (
	doc_id              VARCHAR(255) PRIMARY KEY,
	owner_id            VARCHAR(255) NOT NULL,
	title               VARCHAR(1024) NOT NULL DEFAULT '',
	current_revision_id VARCHAR(255) NOT NULL DEFAULT '',
	created_at          DATETIME NOT NULL,
	updated_at          DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS texture_revisions (
	revision_id         VARCHAR(255) PRIMARY KEY,
	doc_id              VARCHAR(255) NOT NULL,
	owner_id            VARCHAR(255) NOT NULL,
	author_kind         VARCHAR(64) NOT NULL,
	author_label        VARCHAR(255) NOT NULL DEFAULT '',
	version_number      BIGINT NOT NULL DEFAULT 0,
	content             LONGTEXT NOT NULL,
	citations_json      LONGTEXT NOT NULL,
	metadata_json       LONGTEXT NOT NULL,
	provenance_json     LONGTEXT NOT NULL DEFAULT '{}',
	parent_revision_id  VARCHAR(255) NOT NULL DEFAULT '',
	created_at          DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS texture_document_aliases (
	owner_id            VARCHAR(255) NOT NULL,
	source_path         VARCHAR(2048) NOT NULL,
	doc_id              VARCHAR(255) NOT NULL,
	created_at          DATETIME NOT NULL,
	updated_at          DATETIME NOT NULL,
	PRIMARY KEY (owner_id, source_path)
);

CREATE INDEX IF NOT EXISTS idx_texture_docs_owner ON texture_documents(owner_id);
CREATE INDEX IF NOT EXISTS idx_texture_revs_doc ON texture_revisions(doc_id);
CREATE INDEX IF NOT EXISTS idx_texture_revs_owner ON texture_revisions(owner_id);
CREATE INDEX IF NOT EXISTS idx_texture_revs_doc_created ON texture_revisions(doc_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_texture_aliases_doc ON texture_document_aliases(doc_id);

CREATE TABLE IF NOT EXISTS texture_agent_mutations (
	doc_id              VARCHAR(255) NOT NULL,
	loop_id             VARCHAR(255) NOT NULL,
	owner_id            VARCHAR(255) NOT NULL,
	state               VARCHAR(64) NOT NULL DEFAULT 'pending',
	scheduled_message_seq BIGINT NOT NULL DEFAULT 0,
	revision_id         VARCHAR(255) NOT NULL DEFAULT '',
	created_at          DATETIME NOT NULL,
	completed_at        DATETIME,
	PRIMARY KEY (doc_id, loop_id)
);

CREATE INDEX IF NOT EXISTS idx_texture_mutations_doc ON texture_agent_mutations(doc_id);
CREATE INDEX IF NOT EXISTS idx_texture_mutations_run ON texture_agent_mutations(loop_id);

CREATE TABLE IF NOT EXISTS texture_controller_checkpoints (
	doc_id                 VARCHAR(255) NOT NULL,
	owner_id               VARCHAR(255) NOT NULL,
	integrated_message_seq BIGINT NOT NULL DEFAULT 0,
	updated_at             DATETIME NOT NULL,
	PRIMARY KEY (doc_id, owner_id)
);

CREATE INDEX IF NOT EXISTS idx_texture_controller_owner ON texture_controller_checkpoints(owner_id);

CREATE TABLE IF NOT EXISTS texture_decisions (
	decision_id        VARCHAR(255) PRIMARY KEY,
	owner_id           VARCHAR(255) NOT NULL,
	doc_id             VARCHAR(255) NOT NULL,
	loop_id            VARCHAR(255) NOT NULL DEFAULT '',
	trajectory_id      VARCHAR(255) NOT NULL DEFAULT '',
	actor_id           VARCHAR(255) NOT NULL DEFAULT '',
	decision_kind      VARCHAR(128) NOT NULL,
	reason             LONGTEXT NOT NULL,
	evidence_refs_json LONGTEXT NOT NULL DEFAULT '[]',
	next_action        LONGTEXT NOT NULL DEFAULT '',
	created_at         DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_texture_decisions_doc ON texture_decisions(owner_id, doc_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_texture_decisions_run ON texture_decisions(owner_id, loop_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_texture_decisions_trajectory ON texture_decisions(owner_id, trajectory_id, created_at DESC);

CREATE TABLE IF NOT EXISTS agent_evidence (
	evidence_id    VARCHAR(255) PRIMARY KEY,
	owner_id       VARCHAR(255) NOT NULL,
	agent_id       VARCHAR(255) NOT NULL,
	kind           VARCHAR(128) NOT NULL,
	source_uri     LONGTEXT NOT NULL DEFAULT '',
	title          LONGTEXT NOT NULL DEFAULT '',
	content        LONGTEXT NOT NULL,
	metadata_json  LONGTEXT NOT NULL DEFAULT '{}',
	created_at     DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_agent_evidence_owner_agent ON agent_evidence(owner_id, agent_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_evidence_owner_created ON agent_evidence(owner_id, created_at DESC);

CREATE TABLE IF NOT EXISTS content_items (
	content_id     VARCHAR(255) PRIMARY KEY,
	owner_id       VARCHAR(255) NOT NULL,
	source_type    VARCHAR(64) NOT NULL,
	media_type     VARCHAR(255) NOT NULL DEFAULT '',
	app_hint       VARCHAR(64) NOT NULL DEFAULT '',
	title          LONGTEXT NOT NULL DEFAULT '',
	source_url     VARCHAR(2048) NOT NULL DEFAULT '',
	canonical_url  VARCHAR(2048) NOT NULL DEFAULT '',
	file_path      VARCHAR(2048) NOT NULL DEFAULT '',
	text_content   LONGTEXT NOT NULL DEFAULT '',
	content_hash   VARCHAR(128) NOT NULL DEFAULT '',
	metadata_json  LONGTEXT NOT NULL DEFAULT '{}',
	provenance_json LONGTEXT NOT NULL DEFAULT '{}',
	created_at     DATETIME NOT NULL,
	updated_at     DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_content_items_owner_updated ON content_items(owner_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_content_items_owner_app ON content_items(owner_id, app_hint, updated_at DESC);

CREATE TABLE IF NOT EXISTS podcast_subscriptions (
	subscription_id VARCHAR(255) PRIMARY KEY,
	owner_id        VARCHAR(255) NOT NULL,
	feed_url        VARCHAR(2048) NOT NULL,
	content_id      VARCHAR(255) NOT NULL DEFAULT '',
	title           LONGTEXT NOT NULL DEFAULT '',
	author          LONGTEXT NOT NULL DEFAULT '',
	artwork_url     VARCHAR(2048) NOT NULL DEFAULT '',
	last_fetched_at DATETIME,
	created_at      DATETIME NOT NULL,
	updated_at      DATETIME NOT NULL,
	UNIQUE KEY uniq_podcast_subscriptions_owner_feed (owner_id, feed_url)
);

CREATE INDEX IF NOT EXISTS idx_podcast_subscriptions_owner_updated ON podcast_subscriptions(owner_id, updated_at DESC);
`

const (
	textureWorkspaceSuffix     = ".texture"
	defaultTextureWorkspaceDir = "go-choir-texture"
	textureDatabaseName        = "texture"
)

// OpenTextureWorkspace opens (or creates) an embedded Dolt workspace for texture
// document storage only. It is mainly used by store-level tests and local
// workflows that need the document store without the rest of the runtime tables.
func OpenTextureWorkspace(path string) (*Store, error) {
	db, workspacePath, connector, err := openTextureWorkspaceDB(path)
	if err != nil {
		return nil, err
	}
	s := &Store{path: path, textureDB: db, texturePath: workspacePath, doltConnector: connector}
	if err := s.bootstrapTexture(); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("texture workspace: bootstrap: %w", err)
	}
	return s, nil
}

func deriveTextureWorkspacePath(path string) string {
	if path == "" {
		return filepath.Join(os.TempDir(), defaultTextureWorkspaceDir)
	}
	return deriveSuffixedWorkspacePath(path, textureWorkspaceSuffix)
}

func deriveSuffixedWorkspacePath(path, suffix string) string {
	trimmed := strings.TrimSuffix(path, filepath.Ext(path))
	if trimmed == "" {
		trimmed = path
	}
	return trimmed + suffix
}

func resolveTextureWorkspacePath(path string) string {
	return deriveTextureWorkspacePath(path)
}

func openTextureWorkspaceDB(path string) (*sql.DB, string, doltConnector, error) {
	workspacePath := resolveTextureWorkspacePath(path)
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		return nil, "", nil, fmt.Errorf("texture workspace: create directory: %w", err)
	}

	rootDB, connector, err := openDoltRootDB(workspacePath)
	if err != nil {
		return nil, "", nil, err
	}
	databaseName, err := resolveTextureWorkspaceDatabaseName(rootDB, true)
	if err := rootDB.Close(); err != nil {
		_ = connector.Close()
		return nil, "", nil, fmt.Errorf("texture workspace: close bootstrap connection: %w", err)
	}
	if err := connector.Close(); err != nil {
		return nil, "", nil, fmt.Errorf("texture workspace: close bootstrap connector: %w", err)
	}
	if err != nil {
		return nil, "", nil, err
	}

	dbDSN := fmt.Sprintf(
		"file://%s?commitname=Choir&commitemail=system@choir.local&database=%s&multistatements=true&clientfoundrows=true",
		workspacePath,
		databaseName,
	)

	var lastErr error
	for attempt := range 8 {
		dbCfg, err := embedded.ParseDSN(dbDSN)
		if err != nil {
			return nil, "", nil, fmt.Errorf("texture workspace: parse database dsn: %w", err)
		}
		dbCfg.BackOff = newDoltOpenBackOff()
		dbConnector, err := embedded.NewConnector(dbCfg)
		if err != nil {
			lastErr = fmt.Errorf("texture workspace: new database connector: %w", err)
		} else {
			db := sql.OpenDB(dbConnector)
			configureEmbeddedDoltDB(db)
			if pingErr := db.Ping(); pingErr == nil {
				return db, workspacePath, dbConnector, nil
			} else {
				lastErr = fmt.Errorf("texture workspace: ping database: %w", pingErr)
			}
			_ = db.Close()
			_ = dbConnector.Close()
		}

		if !strings.Contains(strings.ToLower(lastErr.Error()), "non 0 lock") {
			break
		}
		time.Sleep(time.Duration(attempt+1) * 25 * time.Millisecond)
	}

	return nil, "", nil, lastErr
}

func openDoltRootDB(workspacePath string) (*sql.DB, doltConnector, error) {
	rootDSN := fmt.Sprintf(
		"file://%s?commitname=Choir&commitemail=system@choir.local&multistatements=true",
		workspacePath,
	)
	cfg, err := embedded.ParseDSN(rootDSN)
	if err != nil {
		return nil, nil, fmt.Errorf("texture workspace: parse dsn: %w", err)
	}
	cfg.BackOff = newDoltOpenBackOff()
	connector, err := embedded.NewConnector(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("texture workspace: new connector: %w", err)
	}
	rootDB := sql.OpenDB(connector)
	configureEmbeddedDoltDB(rootDB)
	return rootDB, connector, nil
}

func resolveTextureWorkspaceDatabaseName(rootDB *sql.DB, createIfMissing bool) (string, error) {
	if exists, err := doltDatabaseExists(rootDB, textureDatabaseName); err != nil {
		return "", err
	} else if exists {
		return textureDatabaseName, nil
	}
	if !createIfMissing {
		return "", nil
	}
	if _, err := rootDB.Exec("CREATE DATABASE IF NOT EXISTS " + textureDatabaseName); err != nil {
		return "", fmt.Errorf("texture workspace: create texture database: %w", err)
	}
	return textureDatabaseName, nil
}

func doltDatabaseExists(rootDB *sql.DB, name string) (bool, error) {
	rows, err := rootDB.Query("SHOW DATABASES")
	if err != nil {
		return false, fmt.Errorf("texture workspace: show databases: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var databaseName string
		if err := rows.Scan(&databaseName); err != nil {
			return false, fmt.Errorf("texture workspace: scan database name: %w", err)
		}
		if databaseName == name {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("texture workspace: iterate databases: %w", err)
	}
	return false, nil
}

func newDoltOpenBackOff() backoff.BackOff {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = 25 * time.Millisecond
	b.RandomizationFactor = 0.2
	b.Multiplier = 1.6
	b.MaxInterval = 250 * time.Millisecond
	b.MaxElapsedTime = 2 * time.Second
	b.Reset()
	return backoff.WithMaxRetries(b, 8)
}

func (s *Store) textureHandle() *sql.DB {
	if s.textureDB != nil {
		return s.textureDB
	}
	return s.db
}

func configureEmbeddedDoltDB(db *sql.DB) {
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
}

// bootstrapTexture applies the texture schema DDL to the embedded workspace.
func (s *Store) bootstrapTexture() error {
	_, err := s.textureHandle().Exec(textureSchemaDDL)
	if err != nil {
		return fmt.Errorf("apply texture schema: %w", err)
	}
	if err := s.ensureTextureColumn("texture_agent_mutations", "scheduled_message_seq", "BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureTextureColumn("texture_revisions", "version_number", "BIGINT NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureTextureColumn("texture_revisions", "provenance_json", "LONGTEXT NOT NULL DEFAULT '{}'"); err != nil {
		return err
	}
	if _, err := s.textureHandle().Exec(`CREATE INDEX IF NOT EXISTS idx_texture_revs_doc_version ON texture_revisions(doc_id, owner_id, version_number DESC)`); err != nil {
		return fmt.Errorf("create texture revision version index: %w", err)
	}
	if err := s.backfillTextureRevisionVersionNumbers(); err != nil {
		return err
	}
	return nil
}

func (s *Store) ensureTextureColumn(table, name, ddl string) error {
	var count int
	if err := s.textureHandle().QueryRow(`
SELECT COUNT(*)
FROM information_schema.columns
WHERE table_schema = DATABASE()
  AND table_name = ?
  AND column_name = ?`,
		table,
		name,
	).Scan(&count); err != nil {
		return fmt.Errorf("information_schema.columns(%s.%s): %w", table, name, err)
	}
	if count > 0 {
		return nil
	}
	if _, err := s.textureHandle().Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, name, ddl)); err != nil {
		return fmt.Errorf("alter %s add %s: %w", table, name, err)
	}
	return nil
}

func (s *Store) backfillTextureRevisionVersionNumbers() error {
	_, err := s.textureHandle().Exec(`
UPDATE texture_revisions AS rev
JOIN (
	SELECT
		revision_id,
		ROW_NUMBER() OVER (
			PARTITION BY doc_id, owner_id
			ORDER BY created_at ASC, revision_id ASC
		) - 1 AS computed_version_number
	FROM texture_revisions
) AS numbered ON numbered.revision_id = rev.revision_id
SET rev.version_number = numbered.computed_version_number
WHERE rev.version_number = 0
  AND numbered.computed_version_number <> 0`)
	if err != nil {
		return fmt.Errorf("backfill texture revision version numbers: %w", err)
	}
	return nil
}

// EnsureTextureSchema applies the texture schema to the embedded workspace.
func (s *Store) EnsureTextureSchema() error {
	return s.bootstrapTexture()
}

// ----- Document CRUD -----

// CreateDocument inserts a new document record.
func (s *Store) CreateDocument(ctx context.Context, doc types.Document) error {
	_, err := s.textureHandle().ExecContext(ctx,
		`INSERT INTO texture_documents (doc_id, owner_id, title, current_revision_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		doc.DocID,
		doc.OwnerID,
		doc.Title,
		doc.CurrentRevisionID,
		doc.CreatedAt.UTC().Format(time.RFC3339Nano),
		doc.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("insert texture document: %w", err)
	}
	return nil
}

// GetDocument returns the document with the given doc ID, scoped to the
// given owner. If the document does not exist or does not belong to the
// owner, it returns ErrNotFound.
func (s *Store) GetDocument(ctx context.Context, docID, ownerID string) (types.Document, error) {
	row := s.textureHandle().QueryRowContext(ctx,
		`SELECT doc_id, owner_id, title, current_revision_id, created_at, updated_at
		   FROM texture_documents
		  WHERE doc_id = ? AND owner_id = ?`,
		docID, ownerID,
	)
	return scanDocument(row)
}

// ListDocumentsByOwner returns documents for the given owner, ordered by
// updated_at descending, limited to the given count.
func (s *Store) ListDocumentsByOwner(ctx context.Context, ownerID string, limit int) ([]types.Document, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.textureHandle().QueryContext(ctx,
		`SELECT doc_id, owner_id, title, current_revision_id, created_at, updated_at
		   FROM texture_documents
		  WHERE owner_id = ?
		  ORDER BY updated_at DESC
		  LIMIT ?`,
		ownerID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query texture documents: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var docs []types.Document
	for rows.Next() {
		doc, err := scanDocument(rows)
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate texture documents: %w", err)
	}
	return docs, nil
}

// ListAllDocuments returns documents across owners ordered by updated_at
// descending. This is used for controller reconciliation on restart.
func (s *Store) ListAllDocuments(ctx context.Context, limit int) ([]types.Document, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := s.textureHandle().QueryContext(ctx,
		`SELECT doc_id, owner_id, title, current_revision_id, created_at, updated_at
		   FROM texture_documents
		  ORDER BY updated_at DESC
		  LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query all texture documents: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var docs []types.Document
	for rows.Next() {
		doc, err := scanDocument(rows)
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate all texture documents: %w", err)
	}
	return docs, nil
}

// SearchResult is a document match from corpus search.
type SearchResult struct {
	DocID       string    `json:"doc_id"`
	Title       string    `json:"title"`
	OwnerID     string    `json:"owner_id"`
	UpdatedAt   time.Time `json:"updated_at"`
	Snippet     string    `json:"snippet,omitempty"`
	MatchSource string    `json:"match_source"` // "title" or "content"
}

// SearchDocuments searches the Texture corpus by title and revision content.
// It returns documents matching the query terms, ordered by relevance.
// Searches across all owners when ownerID is empty.
func (s *Store) SearchPublishedDocuments(ctx context.Context, query string, ownerID string, limit int) ([]SearchResult, error) {
	return s.searchDocuments(ctx, query, ownerID, limit, true)
}

func (s *Store) SearchDocuments(ctx context.Context, query string, ownerID string, limit int) ([]SearchResult, error) {
	return s.searchDocuments(ctx, query, ownerID, limit, false)
}

func (s *Store) searchDocuments(ctx context.Context, query string, ownerID string, limit int, publishedOnly bool) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	// Split query into terms for multi-term LIKE matching.
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return nil, nil
	}
	if len(terms) > 8 {
		terms = terms[:8]
	}

	// Build LIKE conditions for title matching.
	var titleConds []string
	var args []any
	for _, term := range terms {
		likeVal := "%" + term + "%"
		titleConds = append(titleConds, "LOWER(d.title) LIKE ?")
		args = append(args, likeVal)
	}
	titleWhere := strings.Join(titleConds, " AND ")

	// Query documents matching title terms.
	ownerClause := ""
	if strings.TrimSpace(ownerID) != "" {
		ownerClause = " AND d.owner_id = ?"
		args = append(args, ownerID)
	}
	publishedClause := ""
	if publishedOnly {
		publishedClause = " AND EXISTS (SELECT 1 FROM texture_revisions rp WHERE rp.doc_id = d.doc_id AND rp.revision_id = d.current_revision_id AND rp.metadata_json LIKE '%\"platformd_route_path\"%')"
	}
	titleArgs := append([]any(nil), args...)
	titleSQL := fmt.Sprintf(
		`SELECT d.doc_id, d.title, d.owner_id, d.updated_at, 'title' as match_source
		   FROM texture_documents d
		  WHERE %s%s%s
		  ORDER BY d.updated_at DESC
		  LIMIT ?`, titleWhere, ownerClause, publishedClause)
	titleArgs = append(titleArgs, limit)

	rows, err := s.textureHandle().QueryContext(ctx, titleSQL, titleArgs...)
	if err != nil {
		return nil, fmt.Errorf("search texture documents by title: %w", err)
	}
	defer func() { _ = rows.Close() }()

	seen := map[string]bool{}
	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.DocID, &r.Title, &r.OwnerID, &r.UpdatedAt, &r.MatchSource); err != nil {
			return nil, err
		}
		if !seen[r.DocID] {
			seen[r.DocID] = true
			results = append(results, r)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate title search: %w", err)
	}

	// If we have enough title matches, skip content search.
	if len(results) >= limit {
		return results[:limit], nil
	}

	// Search revision content for remaining capacity.
	remaining := limit - len(results)
	var contentConds []string
	var contentArgs []any
	for _, term := range terms {
		likeVal := "%" + term + "%"
		contentConds = append(contentConds, "LOWER(r.content) LIKE ?")
		contentArgs = append(contentArgs, likeVal)
	}
	contentWhere := strings.Join(contentConds, " AND ")
	contentOwnerClause := ""
	if strings.TrimSpace(ownerID) != "" {
		contentOwnerClause = " AND d.owner_id = ?"
		contentArgs = append(contentArgs, ownerID)
	}
	publishedContentClause := ""
	if publishedOnly {
		publishedContentClause = " AND r.metadata_json LIKE '%\"platformd_route_path\"%'"
	}
	// Exclude already-found docs.
	excludeClause := ""
	if len(seen) > 0 {
		exclusions := make([]string, 0, len(seen))
		for docID := range seen {
			exclusions = append(exclusions, "'"+docID+"'")
		}
		excludeClause = " AND d.doc_id NOT IN (" + strings.Join(exclusions, ",") + ")"
	}
	contentSQL := fmt.Sprintf(
		`SELECT DISTINCT d.doc_id, d.title, d.owner_id, d.updated_at, SUBSTRING(r.content, 1, 200) as snippet, 'content' as match_source
		   FROM texture_documents d
		   JOIN texture_revisions r ON r.doc_id = d.doc_id AND r.revision_id = d.current_revision_id
		  WHERE %s%s%s%s
		  ORDER BY d.updated_at DESC
		  LIMIT ?`, contentWhere, contentOwnerClause, publishedContentClause, excludeClause)
	contentArgs = append(contentArgs, remaining)

	rows2, err := s.textureHandle().QueryContext(ctx, contentSQL, contentArgs...)
	if err != nil {
		return nil, fmt.Errorf("search texture documents by content: %w", err)
	}
	defer func() { _ = rows2.Close() }()
	for rows2.Next() {
		var r SearchResult
		var snippet sql.NullString
		if err := rows2.Scan(&r.DocID, &r.Title, &r.OwnerID, &r.UpdatedAt, &snippet, &r.MatchSource); err != nil {
			return nil, err
		}
		if snippet.Valid {
			r.Snippet = snippet.String
		}
		if !seen[r.DocID] {
			seen[r.DocID] = true
			results = append(results, r)
		}
	}
	if err := rows2.Err(); err != nil {
		return nil, fmt.Errorf("iterate content search: %w", err)
	}

	return results, nil
}

// UpdateDocument updates an existing document record.
func (s *Store) UpdateDocument(ctx context.Context, doc types.Document) error {
	result, err := s.textureHandle().ExecContext(ctx,
		`UPDATE texture_documents
		    SET owner_id = ?,
		        title = ?,
		        current_revision_id = ?,
		        updated_at = ?
		  WHERE doc_id = ? AND owner_id = ?`,
		doc.OwnerID,
		doc.Title,
		doc.CurrentRevisionID,
		doc.UpdatedAt.UTC().Format(time.RFC3339Nano),
		doc.DocID,
		doc.OwnerID,
	)
	if err != nil {
		return fmt.Errorf("update texture document: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check updated document rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("%w: document %s for owner %s", ErrNotFound, doc.DocID, doc.OwnerID)
	}
	return nil
}

// GetDocumentAlias resolves a file-browser alias to its canonical document ID.
func (s *Store) GetDocumentAlias(ctx context.Context, ownerID, sourcePath string) (string, error) {
	row := s.textureHandle().QueryRowContext(ctx,
		`SELECT doc_id
		   FROM texture_document_aliases
		  WHERE owner_id = ? AND source_path = ?`,
		ownerID, sourcePath,
	)
	var docID string
	if err := row.Scan(&docID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("query texture document alias: %w", err)
	}
	return docID, nil
}

// GetDocumentAliasSourcePath returns the canonical shortcut path for the given
// document when one exists, otherwise the most recently updated source path.
func (s *Store) GetDocumentAliasSourcePath(ctx context.Context, ownerID, docID string) (string, error) {
	row := s.textureHandle().QueryRowContext(ctx,
		`SELECT source_path
		   FROM texture_document_aliases
		  WHERE owner_id = ? AND doc_id = ?
		  ORDER BY CASE
		             WHEN LOWER(source_path) LIKE '%.texture' THEN 0
		             WHEN LOWER(source_path) LIKE '%.texture' THEN 1
		             ELSE 2
		           END, updated_at DESC
		  LIMIT 1`,
		ownerID, docID,
	)
	var sourcePath string
	if err := row.Scan(&sourcePath); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("query texture alias source path: %w", err)
	}
	return sourcePath, nil
}

// UpsertDocumentAlias records or refreshes the canonical document mapping for a file path.
func (s *Store) UpsertDocumentAlias(ctx context.Context, ownerID, sourcePath, docID string, updatedAt time.Time) error {
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	_, err := s.textureHandle().ExecContext(ctx,
		`INSERT INTO texture_document_aliases (owner_id, source_path, doc_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE
		   doc_id = VALUES(doc_id),
		   updated_at = VALUES(updated_at)`,
		ownerID,
		sourcePath,
		docID,
		updatedAt.UTC().Format(time.RFC3339Nano),
		updatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("upsert texture document alias: %w", err)
	}
	return nil
}

// DeleteDocument deletes a document and all its revisions. It is scoped
// to the given owner.
func (s *Store) DeleteDocument(ctx context.Context, docID, ownerID string) error {
	// Delete revisions first (no FK constraint, so manual cleanup).
	_, _ = s.textureHandle().ExecContext(ctx,
		`DELETE FROM texture_revisions WHERE doc_id = ? AND owner_id = ?`,
		docID, ownerID,
	)
	_, _ = s.textureHandle().ExecContext(ctx,
		`DELETE FROM texture_document_aliases WHERE doc_id = ? AND owner_id = ?`,
		docID, ownerID,
	)
	_, _ = s.textureHandle().ExecContext(ctx,
		`DELETE FROM texture_decisions WHERE doc_id = ? AND owner_id = ?`,
		docID, ownerID,
	)

	result, err := s.textureHandle().ExecContext(ctx,
		`DELETE FROM texture_documents WHERE doc_id = ? AND owner_id = ?`,
		docID, ownerID,
	)
	if err != nil {
		return fmt.Errorf("delete texture document: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check deleted document rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("%w: document %s for owner %s", ErrNotFound, docID, ownerID)
	}
	return nil
}

// DeleteTextureAliasesByOwner removes all source-path aliases for an owner.
func (s *Store) DeleteTextureAliasesByOwner(ctx context.Context, ownerID string) (int64, error) {
	result, err := s.textureHandle().ExecContext(ctx,
		`DELETE FROM texture_document_aliases WHERE owner_id = ?`,
		ownerID,
	)
	if err != nil {
		return 0, fmt.Errorf("delete texture aliases: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("check deleted alias rows: %w", err)
	}
	return rows, nil
}

// ----- Revision CRUD -----

// CreateRevision inserts a new revision record and updates the document's
// current_revision_id if this is the latest revision.
func (s *Store) CreateRevision(ctx context.Context, rev types.Revision) error {
	citations := string(rev.Citations)
	if citations == "" {
		citations = "[]"
	}
	metadata := string(rev.Metadata)
	if metadata == "" {
		metadata = "{}"
	}
	provenance := string(rev.Provenance)
	if strings.TrimSpace(provenance) == "" {
		provenance = "{}"
	}
	tx, err := s.textureHandle().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin texture revision transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var currentHead string
	row := tx.QueryRowContext(ctx,
		`SELECT current_revision_id
		   FROM texture_documents
		  WHERE doc_id = ? AND owner_id = ?`,
		rev.DocID, rev.OwnerID,
	)
	if err := row.Scan(&currentHead); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: document %s for owner %s", ErrNotFound, rev.DocID, rev.OwnerID)
		}
		return fmt.Errorf("query texture document head: %w", err)
	}
	expectedHead := strings.TrimSpace(rev.ParentRevisionID)
	if strings.TrimSpace(currentHead) != expectedHead {
		return fmt.Errorf("%w: document %s current head %s does not match parent %s", ErrStaleDocumentHead, rev.DocID, currentHead, expectedHead)
	}

	var versionNumber int
	if strings.TrimSpace(currentHead) == "" {
		versionNumber = 0
	} else {
		row = tx.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(version_number), -1) + 1
			   FROM texture_revisions
			  WHERE doc_id = ? AND owner_id = ?`,
			rev.DocID,
			rev.OwnerID,
		)
		if err := row.Scan(&versionNumber); err != nil {
			return fmt.Errorf("query next texture revision version number: %w", err)
		}
	}
	rev.VersionNumber = versionNumber

	_, err = tx.ExecContext(ctx,
		`INSERT INTO texture_revisions (revision_id, doc_id, owner_id, author_kind, author_label, version_number, content, citations_json, metadata_json, provenance_json, parent_revision_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rev.RevisionID,
		rev.DocID,
		rev.OwnerID,
		string(rev.AuthorKind),
		rev.AuthorLabel,
		rev.VersionNumber,
		rev.Content,
		citations,
		metadata,
		provenance,
		rev.ParentRevisionID,
		rev.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("insert texture revision: %w", err)
	}

	// Update the document's current_revision_id and updated_at, but only if the
	// head still matches the parent revision we read at the start of this transaction.
	result, err := tx.ExecContext(ctx,
		`UPDATE texture_documents
		    SET current_revision_id = ?,
		        updated_at = ?
		  WHERE doc_id = ? AND owner_id = ? AND current_revision_id = ?`,
		rev.RevisionID,
		rev.CreatedAt.UTC().Format(time.RFC3339Nano),
		rev.DocID,
		rev.OwnerID,
		expectedHead,
	)
	if err != nil {
		return fmt.Errorf("update texture document head: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check updated texture document head rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("%w: document %s head moved during revision create", ErrStaleDocumentHead, rev.DocID)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit texture revision: %w", err)
	}
	return nil
}

// PatchRevisionMetadata merges patch into an existing revision's metadata_json.
// Revisions are otherwise immutable; publication refs use this narrow update path.
func (s *Store) PatchRevisionMetadata(ctx context.Context, ownerID, revisionID string, patch map[string]any) error {
	if s == nil {
		return fmt.Errorf("store unavailable")
	}
	ownerID = strings.TrimSpace(ownerID)
	revisionID = strings.TrimSpace(revisionID)
	if ownerID == "" || revisionID == "" {
		return fmt.Errorf("owner_id and revision_id are required")
	}
	if len(patch) == 0 {
		return nil
	}
	s.jsonPatchMu.Lock()
	defer s.jsonPatchMu.Unlock()

	rev, err := s.GetRevision(ctx, revisionID, ownerID)
	if err != nil {
		return err
	}
	meta := map[string]any{}
	if len(rev.Metadata) > 0 {
		if err := json.Unmarshal(rev.Metadata, &meta); err != nil {
			return fmt.Errorf("decode revision metadata: %w", err)
		}
	}
	for key, value := range patch {
		if strings.TrimSpace(key) == "" {
			continue
		}
		meta[key] = value
	}
	merged, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal revision metadata: %w", err)
	}
	result, err := s.textureHandle().ExecContext(ctx,
		`UPDATE texture_revisions SET metadata_json = ? WHERE revision_id = ? AND owner_id = ?`,
		string(merged), revisionID, ownerID,
	)
	if err != nil {
		return fmt.Errorf("patch revision metadata: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check patched revision rows: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// GetRevision returns the revision with the given revision ID, scoped to
// the given owner.
func (s *Store) GetRevision(ctx context.Context, revisionID, ownerID string) (types.Revision, error) {
	row := s.textureHandle().QueryRowContext(ctx,
		`SELECT revision_id, doc_id, owner_id, author_kind, author_label, version_number, content, citations_json, metadata_json, provenance_json, parent_revision_id, created_at
		   FROM texture_revisions
		  WHERE revision_id = ? AND owner_id = ?`,
		revisionID, ownerID,
	)
	return scanRevision(row)
}

// GetRevisionUnscoped returns the revision without owner scoping.
// Used internally for diff/blame computation where the revision chain
// is already known to belong to the same owner.
func (s *Store) GetRevisionUnscoped(ctx context.Context, revisionID string) (types.Revision, error) {
	row := s.textureHandle().QueryRowContext(ctx,
		`SELECT revision_id, doc_id, owner_id, author_kind, author_label, version_number, content, citations_json, metadata_json, provenance_json, parent_revision_id, created_at
		   FROM texture_revisions
		  WHERE revision_id = ?`,
		revisionID,
	)
	return scanRevision(row)
}

// ListRevisionsByDoc returns revisions for the given document, scoped to
// the given owner, ordered by durable version number descending (newest first),
// limited to the given count.
func (s *Store) ListRevisionsByDoc(ctx context.Context, docID, ownerID string, limit int) ([]types.Revision, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.textureHandle().QueryContext(ctx,
		`SELECT revision_id, doc_id, owner_id, author_kind, author_label, version_number, content, citations_json, metadata_json, provenance_json, parent_revision_id, created_at
		   FROM texture_revisions
		  WHERE doc_id = ? AND owner_id = ?
		  ORDER BY version_number DESC, created_at DESC
		  LIMIT ?`,
		docID, ownerID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query texture revisions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var revs []types.Revision
	for rows.Next() {
		rev, err := scanRevision(rows)
		if err != nil {
			return nil, err
		}
		revs = append(revs, rev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate texture revisions: %w", err)
	}
	return revs, nil
}

func (s *Store) CountRevisionsByDoc(ctx context.Context, docID, ownerID string) (int, error) {
	var count int
	if err := s.textureHandle().QueryRowContext(ctx,
		`SELECT COUNT(*)
		   FROM texture_revisions
		  WHERE doc_id = ? AND owner_id = ?`,
		docID,
		ownerID,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("count texture revisions: %w", err)
	}
	return count, nil
}

func (s *Store) CurrentVersionNumberByDoc(ctx context.Context, docID, ownerID string) (int, error) {
	var versionNumber int
	if err := s.textureHandle().QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version_number), -1)
		   FROM texture_revisions
		  WHERE doc_id = ? AND owner_id = ?`,
		docID,
		ownerID,
	).Scan(&versionNumber); err != nil {
		return -1, fmt.Errorf("query current texture revision version number: %w", err)
	}
	return versionNumber, nil
}

// ----- History -----

// GetHistory returns the revision history for a document as a list of
// HistoryEntry values ordered from the current head backward through the
// parent_revision chain. Using the explicit revision chain rather than a raw
// timestamp sort avoids ambiguity when multiple revisions share the same
// coarse database timestamp.
func (s *Store) GetHistory(ctx context.Context, docID, ownerID string, limit int) ([]types.HistoryEntry, error) {
	if limit <= 0 {
		limit = 50
	}

	doc, err := s.GetDocument(ctx, docID, ownerID)
	if err != nil {
		return nil, err
	}
	if doc.CurrentRevisionID == "" {
		return []types.HistoryEntry{}, nil
	}

	currentID := doc.CurrentRevisionID
	entries := make([]types.HistoryEntry, 0, limit)
	for len(entries) < limit && currentID != "" {
		rev, err := s.GetRevision(ctx, currentID, ownerID)
		if err != nil {
			return nil, fmt.Errorf("load texture history revision %s: %w", currentID, err)
		}
		entries = append(entries, types.HistoryEntry{
			RevisionID:       rev.RevisionID,
			DocID:            rev.DocID,
			AuthorKind:       rev.AuthorKind,
			AuthorLabel:      rev.AuthorLabel,
			ParentRevisionID: rev.ParentRevisionID,
			CreatedAt:        rev.CreatedAt,
		})
		currentID = rev.ParentRevisionID
	}
	return entries, nil
}

// ----- Diff -----

// GetDiff computes the diff between two revisions, scoped to the given
// owner. It returns a DiffResult with sections showing what changed
// (VAL-ETEXT-008).
func (s *Store) GetDiff(ctx context.Context, fromRevID, toRevID, ownerID string) (types.DiffResult, error) {
	fromRev, err := s.GetRevision(ctx, fromRevID, ownerID)
	if err != nil {
		return types.DiffResult{}, fmt.Errorf("get from revision: %w", err)
	}
	toRev, err := s.GetRevision(ctx, toRevID, ownerID)
	if err != nil {
		return types.DiffResult{}, fmt.Errorf("get to revision: %w", err)
	}

	sections := computeLineDiff(fromRev.Content, toRev.Content)

	added, removed := 0, 0
	for _, sec := range sections {
		switch sec.Type {
		case "added":
			added++
		case "removed":
			removed++
		}
	}

	return types.DiffResult{
		FromRevisionID: fromRevID,
		ToRevisionID:   toRevID,
		Sections:       sections,
		AddedLines:     added,
		RemovedLines:   removed,
	}, nil
}

// computeLineDiff computes a line-based diff between two strings using
// the longest common subsequence (LCS) algorithm. It produces a list of
// diff sections that classify each region as unchanged, added, or removed.
func computeLineDiff(from, to string) []types.DiffSection {
	fromLines := splitLines(from)
	toLines := splitLines(to)

	lcs := longestCommonSubsequence(fromLines, toLines)

	var sections []types.DiffSection
	fi, ti := 0, 0

	for _, match := range lcs {
		// Process removed lines before the match in from.
		if fi < match.fi {
			sections = append(sections, types.DiffSection{
				Type:        "removed",
				FromLine:    fi,
				ToLine:      match.fi - 1,
				ToLineNum:   -1,
				ToEndLine:   -1,
				FromContent: strings.Join(fromLines[fi:match.fi], ""),
			})
		}
		// Process added lines before the match in to.
		if ti < match.ti {
			sections = append(sections, types.DiffSection{
				Type:      "added",
				FromLine:  -1,
				ToLine:    -1,
				ToLineNum: ti,
				ToEndLine: match.ti - 1,
				ToContent: strings.Join(toLines[ti:match.ti], ""),
			})
		}

		// Process the matching line (unchanged).
		sections = append(sections, types.DiffSection{
			Type:        "unchanged",
			FromLine:    match.fi,
			ToLine:      match.fi,
			ToLineNum:   match.ti,
			ToEndLine:   match.ti,
			FromContent: fromLines[match.fi],
			ToContent:   toLines[match.ti],
		})

		fi = match.fi + 1
		ti = match.ti + 1
	}

	// Process trailing removed lines.
	if fi < len(fromLines) {
		sections = append(sections, types.DiffSection{
			Type:        "removed",
			FromLine:    fi,
			ToLine:      len(fromLines) - 1,
			ToLineNum:   -1,
			ToEndLine:   -1,
			FromContent: strings.Join(fromLines[fi:], ""),
		})
	}
	// Process trailing added lines.
	if ti < len(toLines) {
		sections = append(sections, types.DiffSection{
			Type:      "added",
			FromLine:  -1,
			ToLine:    -1,
			ToLineNum: ti,
			ToEndLine: len(toLines) - 1,
			ToContent: strings.Join(toLines[ti:], ""),
		})
	}

	// Merge adjacent sections of the same type.
	return mergeSections(sections)
}

// lcsMatch represents a matching position in both sequences.
type lcsMatch struct {
	fi int // index in from sequence
	ti int // index in to sequence
}

// longestCommonSubsequence computes the LCS of two line slices and returns
// the matching positions in order.
func longestCommonSubsequence(from, to []string) []lcsMatch {
	m, n := len(from), len(to)
	if m == 0 || n == 0 {
		return nil
	}

	// Build the DP table. dp[i][j] = length of LCS of from[:i] and to[:j].
	// Use a rolling array to save memory (only need previous row).
	prev := make([]int, n+1)
	curr := make([]int, n+1)

	// Also need to track the actual LCS, so we keep the full DP table
	// for small inputs. For large inputs, we would need Hirschberg's
	// algorithm, but document diffs are typically small enough.
	dp := make([][]int, m+1)
	dp[0] = make([]int, n+1)
	for i := 1; i <= m; i++ {
		dp[i] = make([]int, n+1)
		for j := 1; j <= n; j++ {
			if from[i-1] == to[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Keep prev/curr in sync (unused but silences vet).
	_ = prev
	_ = curr

	// Backtrack to find the actual matching positions.
	var matches []lcsMatch
	i, j := m, n
	for i > 0 && j > 0 {
		if from[i-1] == to[j-1] {
			matches = append(matches, lcsMatch{fi: i - 1, ti: j - 1})
			i--
			j--
		} else if dp[i-1][j] >= dp[i][j-1] {
			i--
		} else {
			j--
		}
	}

	// Reverse to get forward order.
	for left, right := 0, len(matches)-1; left < right; left, right = left+1, right-1 {
		matches[left], matches[right] = matches[right], matches[left]
	}

	return matches
}

// splitLines splits a string into lines, preserving line endings.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i+1])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// mergeSections merges adjacent sections of the same type.
func mergeSections(sections []types.DiffSection) []types.DiffSection {
	if len(sections) <= 1 {
		return sections
	}
	merged := []types.DiffSection{sections[0]}
	for i := 1; i < len(sections); i++ {
		last := &merged[len(merged)-1]
		curr := sections[i]
		if last.Type == curr.Type {
			// Merge.
			last.ToLine = curr.ToLine
			last.ToEndLine = curr.ToEndLine
			if curr.FromContent != "" {
				last.FromContent += curr.FromContent
			}
			if curr.ToContent != "" {
				last.ToContent += curr.ToContent
			}
		} else {
			merged = append(merged, curr)
		}
	}
	return merged
}

// ----- Blame -----

// GetBlame computes the blame for a revision, scoped to the given owner.
// It walks backward through the revision chain, attributing each line to
// the most recent revision that changed it. This provides section-level
// attribution distinguishing whether the last editor was the user or the
// agent (VAL-ETEXT-009).
func (s *Store) GetBlame(ctx context.Context, revisionID, ownerID string) (types.BlameResult, error) {
	// First verify owner scope.
	headRev, err := s.GetRevision(ctx, revisionID, ownerID)
	if err != nil {
		return types.BlameResult{}, err
	}

	// Collect the revision chain from head backward.
	chain, err := s.collectRevisionChain(ctx, headRev)
	if err != nil {
		return types.BlameResult{}, fmt.Errorf("collect revision chain: %w", err)
	}

	sections := computeBlame(chain, headRev)

	return types.BlameResult{
		RevisionID: revisionID,
		DocID:      headRev.DocID,
		Sections:   sections,
	}, nil
}

// collectRevisionChain walks backward through parent_revision_id from the
// head revision to the root, collecting all revisions in chronological order.
func (s *Store) collectRevisionChain(ctx context.Context, head types.Revision) ([]types.Revision, error) {
	// Start with the head.
	seen := map[string]bool{head.RevisionID: true}
	chain := []types.Revision{head}

	current := head
	for current.ParentRevisionID != "" {
		parentID := current.ParentRevisionID
		if seen[parentID] {
			// Cycle detected; stop.
			break
		}
		seen[parentID] = true

		parent, err := s.GetRevisionUnscoped(ctx, parentID)
		if err != nil {
			// Missing parent; stop the chain.
			break
		}
		chain = append(chain, parent)
		current = parent
	}

	// Reverse to get chronological order (oldest first).
	for left, right := 0, len(chain)-1; left < right; left, right = left+1, right-1 {
		chain[left], chain[right] = chain[right], chain[left]
	}

	return chain, nil
}

// computeBlame attributes each line in the head revision to the most recent
// revision that changed it. It processes the revision chain from oldest to
// newest, tracking which revision last modified each line.
func computeBlame(chain []types.Revision, head types.Revision) []types.BlameSection {
	headLines := splitLines(head.Content)
	if len(headLines) == 0 {
		return nil
	}

	// blame[i] = index into chain of the revision that last changed line i.
	blame := make([]int, len(headLines))
	for i := range blame {
		blame[i] = -1
	}

	// Start with the initial content as the first revision's content.
	// Then for each subsequent revision, diff it against the previous
	// and mark changed lines.
	if len(chain) == 0 {
		// No chain (shouldn't happen), attribute all to head.
		for i := range blame {
			blame[i] = 0
		}
	} else {
		// Attribute all lines to the first revision initially.
		firstLines := splitLines(chain[0].Content)
		for i := range blame {
			if i < len(firstLines) {
				blame[i] = 0
			}
		}

		// For each subsequent revision, find which lines changed.
		prevLines := firstLines
		for ci := 1; ci < len(chain); ci++ {
			currLines := splitLines(chain[ci].Content)
			if len(currLines) != len(headLines) {
				// Content length changed; this is a more complex diff.
				// For blame, we use a simple approach: if the current
				// revision's content matches the head, attribute lines
				// that differ from the previous revision to this revision.
				diff := computeLineDiff(
					strings.Join(prevLines, ""),
					strings.Join(currLines, ""),
				)
				// Map diff sections back to head line numbers.
				// This is approximate but sufficient for section-level blame.
				_ = diff // We use a simpler approach below.
			}
			prevLines = currLines
		}

		// Simple blame: for each pair of consecutive revisions, mark lines
		// that are different from the previous revision as belonging to the
		// newer revision.
		for ci := len(chain) - 1; ci >= 1; ci-- {
			currLines := splitLines(chain[ci].Content)
			prevContent := ""
			if ci > 0 {
				prevContent = chain[ci-1].Content
			}
			prevLines := splitLines(prevContent)

			// Lines present in current but different from previous are
			// attributed to current revision.
			for i := 0; i < len(currLines) && i < len(headLines); i++ {
				if i < len(prevLines) {
					if currLines[i] != prevLines[i] {
						blame[i] = ci
					}
				} else {
					// New lines added by this revision.
					blame[i] = ci
				}
			}
		}

		// Mark any remaining unattributed lines.
		for i := range blame {
			if blame[i] == -1 {
				blame[i] = 0
			}
		}
	}

	// Group consecutive lines with the same blame revision into sections.
	var sections []types.BlameSection
	start := 0
	for i := 1; i <= len(blame); i++ {
		if i == len(blame) || blame[i] != blame[start] {
			ci := blame[start]
			rev := head
			if ci >= 0 && ci < len(chain) {
				rev = chain[ci]
			}
			sections = append(sections, types.BlameSection{
				RevisionID:  rev.RevisionID,
				AuthorKind:  rev.AuthorKind,
				AuthorLabel: rev.AuthorLabel,
				StartLine:   start,
				EndLine:     i - 1,
				Content:     strings.Join(headLines[start:i], ""),
				Timestamp:   rev.CreatedAt,
			})
			start = i
		}
	}

	return sections
}

// ----- Scan helpers -----

func scanTextureDecisionRows(rows *sql.Rows) ([]types.TextureDecisionRecord, error) {
	defer func() { _ = rows.Close() }()
	var records []types.TextureDecisionRecord
	for rows.Next() {
		rec, err := scanTextureDecision(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate texture decisions: %w", err)
	}
	return records, nil
}

func scanTextureDecision(row interface{ Scan(...any) error }) (types.TextureDecisionRecord, error) {
	var (
		rec             types.TextureDecisionRecord
		evidenceRefsRaw string
		createdAtRaw    string
	)
	if err := row.Scan(
		&rec.DecisionID,
		&rec.OwnerID,
		&rec.DocID,
		&rec.RunID,
		&rec.TrajectoryID,
		&rec.ActorID,
		&rec.DecisionKind,
		&rec.Reason,
		&evidenceRefsRaw,
		&rec.NextAction,
		&createdAtRaw,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.TextureDecisionRecord{}, ErrNotFound
		}
		return types.TextureDecisionRecord{}, fmt.Errorf("scan texture decision: %w", err)
	}
	if strings.TrimSpace(evidenceRefsRaw) != "" {
		if err := json.Unmarshal([]byte(evidenceRefsRaw), &rec.EvidenceRefs); err != nil {
			return types.TextureDecisionRecord{}, fmt.Errorf("decode texture decision evidence refs: %w", err)
		}
	}
	createdAt, err := time.Parse(time.RFC3339Nano, createdAtRaw)
	if err != nil {
		return types.TextureDecisionRecord{}, fmt.Errorf("parse texture decision created_at: %w", err)
	}
	rec.CreatedAt = createdAt.UTC()
	return rec, nil
}

func scanEvidence(row interface{ Scan(...any) error }) (types.EvidenceRecord, error) {
	var (
		rec          types.EvidenceRecord
		metadataJSON string
		createdAtRaw string
	)
	if err := row.Scan(
		&rec.EvidenceID,
		&rec.OwnerID,
		&rec.AgentID,
		&rec.Kind,
		&rec.SourceURI,
		&rec.Title,
		&rec.Content,
		&metadataJSON,
		&createdAtRaw,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.EvidenceRecord{}, ErrNotFound
		}
		return types.EvidenceRecord{}, fmt.Errorf("scan evidence record: %w", err)
	}
	if strings.TrimSpace(metadataJSON) != "" && metadataJSON != "{}" {
		rec.Metadata = json.RawMessage(metadataJSON)
	}
	createdAt, err := time.Parse(time.RFC3339Nano, createdAtRaw)
	if err != nil {
		return types.EvidenceRecord{}, fmt.Errorf("parse evidence created_at: %w", err)
	}
	rec.CreatedAt = createdAt.UTC()
	return rec, nil
}

// scanDocument scans a document record from a single row.
func scanDocument(row interface{ Scan(...any) error }) (types.Document, error) {
	var doc types.Document
	var createdAt, updatedAt string

	err := row.Scan(
		&doc.DocID,
		&doc.OwnerID,
		&doc.Title,
		&doc.CurrentRevisionID,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.Document{}, ErrNotFound
		}
		return types.Document{}, fmt.Errorf("scan texture document: %w", err)
	}

	doc.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.Document{}, fmt.Errorf("parse document created_at: %w", err)
	}
	doc.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.Document{}, fmt.Errorf("parse document updated_at: %w", err)
	}

	return doc, nil
}

// scanRevision scans a revision record from a single row.
func scanRevision(row interface{ Scan(...any) error }) (types.Revision, error) {
	var rev types.Revision
	var authorKind, createdAt string
	var citationsJSON, metadataJSON, provenanceJSON string
	var parentRevID string

	err := row.Scan(
		&rev.RevisionID,
		&rev.DocID,
		&rev.OwnerID,
		&authorKind,
		&rev.AuthorLabel,
		&rev.VersionNumber,
		&rev.Content,
		&citationsJSON,
		&metadataJSON,
		&provenanceJSON,
		&parentRevID,
		&createdAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.Revision{}, ErrNotFound
		}
		return types.Revision{}, fmt.Errorf("scan texture revision: %w", err)
	}

	rev.AuthorKind = types.AuthorKind(authorKind)
	rev.ParentRevisionID = parentRevID

	rev.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.Revision{}, fmt.Errorf("parse revision created_at: %w", err)
	}

	if citationsJSON != "" && citationsJSON != "[]" {
		rev.Citations = json.RawMessage(citationsJSON)
	}
	if metadataJSON != "" && metadataJSON != "{}" {
		rev.Metadata = json.RawMessage(metadataJSON)
	}
	if provenanceJSON != "" && provenanceJSON != "{}" {
		rev.Provenance = json.RawMessage(provenanceJSON)
	}

	return rev, nil
}

// ----- Agent mutation tracking (VAL-CROSS-122: idempotent revision) -----

// AgentMutation represents an in-flight or completed appagent-driven document
// mutation. It tracks the mapping from a runtime run to a document mutation,
// enabling idempotent handling so that renewal/retry does not create a
// duplicate canonical revision.
type AgentMutation struct {
	DocID               string     `json:"doc_id"`
	RunID               string     `json:"loop_id"`
	OwnerID             string     `json:"owner_id"`
	State               string     `json:"state"` // "pending", "completed", "failed", "deferred"
	ScheduledMessageSeq int64      `json:"scheduled_message_seq,omitempty"`
	RevisionID          string     `json:"revision_id,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	CompletedAt         *time.Time `json:"completed_at,omitempty"`
}

type TextureControllerCheckpoint struct {
	DocID                string
	OwnerID              string
	IntegratedMessageSeq int64
	UpdatedAt            time.Time
}

// CreateAgentMutation records a new in-flight appagent mutation. It uses
// INSERT IGNORE so that duplicate (doc_id, loop_id) pairs are silently
// ignored, supporting idempotent run creation (VAL-CROSS-122).
func (s *Store) CreateAgentMutation(ctx context.Context, m AgentMutation) error {
	var completedAt any
	if m.CompletedAt != nil {
		completedAt = m.CompletedAt.UTC().Format(time.RFC3339Nano)
	}
	_, err := s.textureHandle().ExecContext(ctx,
		`INSERT IGNORE INTO texture_agent_mutations (doc_id, loop_id, owner_id, state, scheduled_message_seq, revision_id, created_at, completed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		m.DocID,
		m.RunID,
		m.OwnerID,
		m.State,
		m.ScheduledMessageSeq,
		m.RevisionID,
		m.CreatedAt.UTC().Format(time.RFC3339Nano),
		completedAt,
	)
	if err != nil {
		return fmt.Errorf("insert texture agent mutation: %w", err)
	}
	return nil
}

// GetPendingAgentMutationByDoc returns the pending agent mutation for a
// document, if one exists. This is used to return the existing run ID
// when a retry/renewal occurs, preventing duplicate mutation submissions
// (VAL-CROSS-122).
func (s *Store) GetPendingAgentMutationByDoc(ctx context.Context, docID, ownerID string) (*AgentMutation, error) {
	row := s.textureHandle().QueryRowContext(ctx,
		`SELECT doc_id, loop_id, owner_id, state, scheduled_message_seq, revision_id, created_at, completed_at
		   FROM texture_agent_mutations
		  WHERE doc_id = ? AND owner_id = ? AND state = 'pending'
		  ORDER BY created_at DESC
		  LIMIT 1`,
		docID, ownerID,
	)
	return scanAgentMutation(row)
}

// GetAgentMutationByRun returns the agent mutation for a specific run ID.
// This is used during run completion to check if the revision has already
// been created (VAL-CROSS-122: no duplicate canonical revision).
func (s *Store) GetAgentMutationByRun(ctx context.Context, runID string) (*AgentMutation, error) {
	row := s.textureHandle().QueryRowContext(ctx,
		`SELECT doc_id, loop_id, owner_id, state, scheduled_message_seq, revision_id, created_at, completed_at
		   FROM texture_agent_mutations
		  WHERE loop_id = ?`,
		runID,
	)
	return scanAgentMutation(row)
}

// RecordAgentMutationRevision records the latest canonical revision written by
// a still-active Texture mutation without closing the run. Multi-revision
// Texture actors use the row as run-liveness/idempotency state; the revision
// rows themselves are the per-write commit records.
func (s *Store) RecordAgentMutationRevision(ctx context.Context, runID, revisionID string) error {
	result, err := s.textureHandle().ExecContext(ctx,
		`UPDATE texture_agent_mutations
		    SET revision_id = ?
		  WHERE loop_id = ? AND state = 'pending'`,
		revisionID,
		runID,
	)
	if err != nil {
		return fmt.Errorf("record texture agent mutation revision: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check recorded mutation revision rows: %w", err)
	}
	if rows == 0 {
		return ErrMutationAlreadyCompleted
	}
	return nil
}

// CompleteAgentMutation marks an agent mutation as completed with the latest
// revision ID written by the run. It returns ErrMutationAlreadyCompleted if the
// mutation is no longer pending.
var ErrMutationAlreadyCompleted = errors.New("agent mutation already completed")

func (s *Store) CompleteAgentMutation(ctx context.Context, runID, revisionID string) error {
	now := time.Now().UTC()
	result, err := s.textureHandle().ExecContext(ctx,
		`UPDATE texture_agent_mutations
		    SET state = 'completed',
		        revision_id = ?,
		        completed_at = ?
		  WHERE loop_id = ? AND state = 'pending'`,
		revisionID,
		now.Format(time.RFC3339Nano),
		runID,
	)
	if err != nil {
		return fmt.Errorf("complete texture agent mutation: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check completed mutation rows: %w", err)
	}
	if rows == 0 {
		return ErrMutationAlreadyCompleted
	}
	return nil
}

// DeferAgentMutation marks a Texture run as intentionally completed without a
// document write because it delegated to workers and is waiting for their
// updates to wake the next revision run.
func (s *Store) DeferAgentMutation(ctx context.Context, runID string) error {
	now := time.Now().UTC()
	_, err := s.textureHandle().ExecContext(ctx,
		`UPDATE texture_agent_mutations
		    SET state = 'deferred',
		        completed_at = ?
		  WHERE loop_id = ? AND state = 'pending'`,
		now.Format(time.RFC3339Nano),
		runID,
	)
	if err != nil {
		return fmt.Errorf("defer texture agent mutation: %w", err)
	}
	return nil
}

func (s *Store) GetTextureControllerCheckpoint(ctx context.Context, docID, ownerID string) (*TextureControllerCheckpoint, error) {
	row := s.textureHandle().QueryRowContext(ctx,
		`SELECT doc_id, owner_id, integrated_message_seq, updated_at
		   FROM texture_controller_checkpoints
		  WHERE doc_id = ? AND owner_id = ?`,
		docID, ownerID,
	)
	var checkpoint TextureControllerCheckpoint
	var updatedAt string
	if err := row.Scan(&checkpoint.DocID, &checkpoint.OwnerID, &checkpoint.IntegratedMessageSeq, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query texture controller checkpoint: %w", err)
	}
	ts, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse texture controller checkpoint updated_at: %w", err)
	}
	checkpoint.UpdatedAt = ts
	return &checkpoint, nil
}

func (s *Store) UpsertTextureControllerCheckpoint(ctx context.Context, checkpoint TextureControllerCheckpoint) error {
	if checkpoint.UpdatedAt.IsZero() {
		checkpoint.UpdatedAt = time.Now().UTC()
	}
	_, err := s.textureHandle().ExecContext(ctx,
		`INSERT INTO texture_controller_checkpoints (doc_id, owner_id, integrated_message_seq, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE
		   integrated_message_seq = VALUES(integrated_message_seq),
		   updated_at = VALUES(updated_at)`,
		checkpoint.DocID,
		checkpoint.OwnerID,
		checkpoint.IntegratedMessageSeq,
		checkpoint.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("upsert texture controller checkpoint: %w", err)
	}
	return nil
}

// FailAgentMutation marks an agent mutation as failed.
func (s *Store) FailAgentMutation(ctx context.Context, runID string) error {
	now := time.Now().UTC()
	_, err := s.textureHandle().ExecContext(ctx,
		`UPDATE texture_agent_mutations
		    SET state = 'failed',
		        completed_at = ?
		  WHERE loop_id = ? AND state = 'pending'`,
		now.Format(time.RFC3339Nano),
		runID,
	)
	if err != nil {
		return fmt.Errorf("fail texture agent mutation: %w", err)
	}
	return nil
}

// CancelAgentMutation marks an agent mutation as cancelled by the owner while
// preserving the current document head so the user can resume with a later
// revision request.
func (s *Store) CancelAgentMutation(ctx context.Context, runID string) error {
	now := time.Now().UTC()
	_, err := s.textureHandle().ExecContext(ctx,
		`UPDATE texture_agent_mutations
		    SET state = 'cancelled',
		        completed_at = ?
		  WHERE loop_id = ? AND state = 'pending'`,
		now.Format(time.RFC3339Nano),
		runID,
	)
	if err != nil {
		return fmt.Errorf("cancel texture agent mutation: %w", err)
	}
	return nil
}

// MarkAgentMutationStale clears a pending mutation whose owning activation is
// no longer active. This prevents stale pending rows from keeping the Texture
// editor in a perpetual "Revising..." state after recovery or missed completion
// reconciliation.
func (s *Store) MarkAgentMutationStale(ctx context.Context, runID string) error {
	now := time.Now().UTC()
	_, err := s.textureHandle().ExecContext(ctx,
		`UPDATE texture_agent_mutations
		    SET state = 'stale_activation',
		        completed_at = ?
		  WHERE loop_id = ? AND state = 'pending'`,
		now.Format(time.RFC3339Nano),
		runID,
	)
	if err != nil {
		return fmt.Errorf("mark stale texture agent mutation: %w", err)
	}
	return nil
}

// CreateTextureDecision inserts an off-document Texture decision note.
func (s *Store) CreateTextureDecision(ctx context.Context, rec types.TextureDecisionRecord) error {
	evidenceRefs, err := json.Marshal(rec.EvidenceRefs)
	if err != nil {
		return fmt.Errorf("marshal texture decision evidence refs: %w", err)
	}
	if len(evidenceRefs) == 0 || string(evidenceRefs) == "null" {
		evidenceRefs = []byte("[]")
	}
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now().UTC()
	}
	_, err = s.textureHandle().ExecContext(ctx,
		`INSERT INTO texture_decisions (decision_id, owner_id, doc_id, loop_id, trajectory_id, actor_id, decision_kind, reason, evidence_refs_json, next_action, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.DecisionID,
		rec.OwnerID,
		rec.DocID,
		rec.RunID,
		rec.TrajectoryID,
		rec.ActorID,
		rec.DecisionKind,
		rec.Reason,
		string(evidenceRefs),
		rec.NextAction,
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("insert texture decision: %w", err)
	}
	return nil
}

// ListTextureDecisionsByDocument returns recent off-document decision notes for a
// document, scoped to its owner.
func (s *Store) ListTextureDecisionsByDocument(ctx context.Context, ownerID, docID string, limit int) ([]types.TextureDecisionRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.textureHandle().QueryContext(ctx,
		`SELECT decision_id, owner_id, doc_id, loop_id, trajectory_id, actor_id, decision_kind, reason, evidence_refs_json, next_action, created_at
		   FROM texture_decisions
		  WHERE owner_id = ? AND doc_id = ?
		  ORDER BY created_at DESC
		  LIMIT ?`,
		ownerID, docID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query texture decisions by document: %w", err)
	}
	return scanTextureDecisionRows(rows)
}

// ListTextureDecisionsByTrajectory returns recent decision notes associated with
// a trajectory, scoped to the owner.
func (s *Store) ListTextureDecisionsByTrajectory(ctx context.Context, ownerID, trajectoryID string, limit int) ([]types.TextureDecisionRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.textureHandle().QueryContext(ctx,
		`SELECT decision_id, owner_id, doc_id, loop_id, trajectory_id, actor_id, decision_kind, reason, evidence_refs_json, next_action, created_at
		   FROM texture_decisions
		  WHERE owner_id = ? AND trajectory_id = ?
		  ORDER BY created_at DESC
		  LIMIT ?`,
		ownerID, trajectoryID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query texture decisions by trajectory: %w", err)
	}
	return scanTextureDecisionRows(rows)
}

// CreateEvidence inserts a durable evidence record into the embedded Dolt
// workspace. Evidence is owner-scoped and associated with the capturing agent.
func (s *Store) CreateEvidence(ctx context.Context, rec types.EvidenceRecord) error {
	metadata := string(rec.Metadata)
	if strings.TrimSpace(metadata) == "" {
		metadata = "{}"
	}
	_, err := s.textureHandle().ExecContext(ctx,
		`INSERT INTO agent_evidence (evidence_id, owner_id, agent_id, kind, source_uri, title, content, metadata_json, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.EvidenceID,
		rec.OwnerID,
		rec.AgentID,
		rec.Kind,
		rec.SourceURI,
		rec.Title,
		rec.Content,
		metadata,
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("insert agent evidence: %w", err)
	}
	return nil
}

// GetEvidence returns a single evidence record scoped to the given owner.
func (s *Store) GetEvidence(ctx context.Context, evidenceID, ownerID string) (types.EvidenceRecord, error) {
	row := s.textureHandle().QueryRowContext(ctx,
		`SELECT evidence_id, owner_id, agent_id, kind, source_uri, title, content, metadata_json, created_at
		   FROM agent_evidence
		  WHERE evidence_id = ? AND owner_id = ?`,
		evidenceID, ownerID,
	)
	return scanEvidence(row)
}

// ListEvidenceByAgent returns recent evidence captured by an agent and scoped
// to the given owner. If agentID is empty it returns recent evidence across all
// of the owner's agents.
func (s *Store) ListEvidenceByAgent(ctx context.Context, ownerID, agentID string, limit int) ([]types.EvidenceRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	var (
		rows *sql.Rows
		err  error
	)
	if strings.TrimSpace(agentID) == "" {
		rows, err = s.textureHandle().QueryContext(ctx,
			`SELECT evidence_id, owner_id, agent_id, kind, source_uri, title, content, metadata_json, created_at
			   FROM agent_evidence
			  WHERE owner_id = ?
			  ORDER BY created_at DESC
			  LIMIT ?`,
			ownerID, limit,
		)
	} else {
		rows, err = s.textureHandle().QueryContext(ctx,
			`SELECT evidence_id, owner_id, agent_id, kind, source_uri, title, content, metadata_json, created_at
			   FROM agent_evidence
			  WHERE owner_id = ? AND agent_id = ?
			  ORDER BY created_at DESC
			  LIMIT ?`,
			ownerID, agentID, limit,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("query agent evidence: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []types.EvidenceRecord
	for rows.Next() {
		rec, err := scanEvidence(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agent evidence: %w", err)
	}
	return out, nil
}

// CreateContentItem inserts a shared content-substrate record.
func (s *Store) CreateContentItem(ctx context.Context, rec types.ContentItem) error {
	metadata := string(rec.Metadata)
	if strings.TrimSpace(metadata) == "" {
		metadata = "{}"
	}
	provenance := string(rec.Provenance)
	if strings.TrimSpace(provenance) == "" {
		provenance = "{}"
	}
	_, err := s.textureHandle().ExecContext(ctx,
		`INSERT INTO content_items (
			content_id, owner_id, source_type, media_type, app_hint, title,
			source_url, canonical_url, file_path, text_content, content_hash,
			metadata_json, provenance_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.ContentID,
		rec.OwnerID,
		rec.SourceType,
		rec.MediaType,
		rec.AppHint,
		rec.Title,
		rec.SourceURL,
		rec.CanonicalURL,
		rec.FilePath,
		rec.TextContent,
		rec.ContentHash,
		metadata,
		provenance,
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("insert content item: %w", err)
	}
	return nil
}

// GetContentItem returns a content item scoped to the authenticated owner.
func (s *Store) GetContentItem(ctx context.Context, ownerID, contentID string) (types.ContentItem, error) {
	row := s.textureHandle().QueryRowContext(ctx,
		`SELECT content_id, owner_id, source_type, media_type, app_hint, title,
		        source_url, canonical_url, file_path, text_content, content_hash,
		        metadata_json, provenance_json, created_at, updated_at
		   FROM content_items
		  WHERE owner_id = ? AND content_id = ?`,
		ownerID,
		contentID,
	)
	return scanContentItem(row)
}

// ListContentItems lists recent content substrate records for an owner.
func (s *Store) ListContentItems(ctx context.Context, ownerID string, limit int) ([]types.ContentItem, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.textureHandle().QueryContext(ctx,
		`SELECT content_id, owner_id, source_type, media_type, app_hint, title,
		        source_url, canonical_url, file_path, text_content, content_hash,
		        metadata_json, provenance_json, created_at, updated_at
		   FROM content_items
		  WHERE owner_id = ?
		  ORDER BY updated_at DESC
		  LIMIT ?`,
		ownerID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query content items: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []types.ContentItem
	for rows.Next() {
		rec, err := scanContentItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate content items: %w", err)
	}
	return out, nil
}

func scanContentItem(row interface{ Scan(...any) error }) (types.ContentItem, error) {
	var (
		rec            types.ContentItem
		metadataJSON   string
		provenanceJSON string
		createdAt      string
		updatedAt      string
	)
	if err := row.Scan(
		&rec.ContentID,
		&rec.OwnerID,
		&rec.SourceType,
		&rec.MediaType,
		&rec.AppHint,
		&rec.Title,
		&rec.SourceURL,
		&rec.CanonicalURL,
		&rec.FilePath,
		&rec.TextContent,
		&rec.ContentHash,
		&metadataJSON,
		&provenanceJSON,
		&createdAt,
		&updatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.ContentItem{}, ErrNotFound
		}
		return types.ContentItem{}, fmt.Errorf("scan content item: %w", err)
	}
	if strings.TrimSpace(metadataJSON) != "" {
		rec.Metadata = json.RawMessage(metadataJSON)
	}
	if strings.TrimSpace(provenanceJSON) != "" {
		rec.Provenance = json.RawMessage(provenanceJSON)
	}
	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.ContentItem{}, fmt.Errorf("parse content created_at: %w", err)
	}
	parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.ContentItem{}, fmt.Errorf("parse content updated_at: %w", err)
	}
	rec.CreatedAt = parsedCreatedAt.UTC()
	rec.UpdatedAt = parsedUpdatedAt.UTC()
	return rec, nil
}

// scanAgentMutation scans an agent mutation record from a single row.
func scanAgentMutation(row interface{ Scan(...any) error }) (*AgentMutation, error) {
	var m AgentMutation
	var createdAt string
	var completedAt sql.NullString

	err := row.Scan(
		&m.DocID,
		&m.RunID,
		&m.OwnerID,
		&m.State,
		&m.ScheduledMessageSeq,
		&m.RevisionID,
		&createdAt,
		&completedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // no pending mutation is not an error
		}
		return nil, fmt.Errorf("scan texture agent mutation: %w", err)
	}

	m.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse agent mutation created_at: %w", err)
	}
	if completedAt.Valid {
		t, err := time.Parse(time.RFC3339Nano, completedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse agent mutation completed_at: %w", err)
		}
		m.CompletedAt = &t
	}

	return &m, nil
}
