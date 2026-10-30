// Package store provides texture document persistence for the go-choir autoputer
// runtime.
//
// The texture store persists documents, revisions, citations, and metadata
// using SQLite, with history/snapshot/diff/blame APIs and per-user in-process
// storage inside the autoputer.
//
// Design decisions:
//   - Embedded SQLite for document storage without a separate server process.
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
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	textureWorkspaceSuffix     = ".texture"
	defaultTextureWorkspaceDir = "go-choir-texture"
)

// TextureWorkspacePath is the sidecar directory derived from a runtime marker
// (the legacy Dolt workspace path; the store keeps its schema receipt there).
func TextureWorkspacePath(path string) string {
	return deriveTextureWorkspacePath(path)
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

func (s *Store) textureHandle() *sql.DB {
	return s.db
}

// ----- Document CRUD -----

// CreateDocument inserts a new document record.
func (s *Store) CreateDocument(ctx context.Context, doc types.Document) error {
	if err := s.CreateTextureDocumentOG(ctx, doc); err != nil {
		return err
	}
	return nil
}

// GetDocument returns the document with the given doc ID, scoped to the
// given owner. If the document does not exist or does not belong to the
// owner, it returns ErrNotFound.
func (s *Store) GetDocument(ctx context.Context, docID, ownerID string) (types.Document, error) {
	return s.GetTextureDocumentOG(ctx, ownerID, docID)
}

// ListDocumentsByOwner returns documents for the given owner, ordered by
// updated_at descending, limited to the given count.
func (s *Store) ListDocumentsByOwner(ctx context.Context, ownerID string, limit int) ([]types.Document, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.ListTextureDocumentsByOwnerOG(ctx, ownerID, limit)
}
func (s *Store) ListDocumentsByScope(ctx context.Context, ownerID, computerID string, limit int) ([]types.Document, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.ListTextureDocumentsByScopeOG(ctx, ownerID, computerID, limit)
}

// ListAllDocuments returns documents across owners ordered by updated_at
// descending. This is used for controller reconciliation on restart.
func (s *Store) ListAllDocuments(ctx context.Context, limit int) ([]types.Document, error) {
	return s.ListAllTextureDocumentsOG(ctx, limit)
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

// SearchDocuments searches the Texture corpus by title and head revision
// content, ranked by BM25 with title terms weighted up; a document sharing no
// query term is not returned. Searches across all owners when ownerID is
// empty. Lifecycle documents (stories) are searched too, their heads read
// through the lifecycle getter (L2a R2-8). Head text comes from searchIdx,
// which reads a head once per revision (search_index.go): the earlier scan
// read every head on every query and kept only documents holding every term
// (docs/problems/research-more-slow-and-hidden-2026-10-21.md E1).
func (s *Store) SearchDocuments(ctx context.Context, query string, ownerID string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	docs, err := s.searchableTextureDocuments(ctx, strings.TrimSpace(ownerID))
	if err != nil {
		return nil, fmt.Errorf("search texture documents: load docs: %w", err)
	}
	return rankDocuments(query, docs, s.searchEntries(ctx, docs), limit), nil
}

// searchableTextureDocuments lists the Texture documents a search reads,
// lifecycle documents included, for one owner (or, ownerID "", all), newest
// first.
func (s *Store) searchableTextureDocuments(ctx context.Context, ownerID string) ([]types.Document, error) {
	objs, err := s.ogListAllObjectsByKind(ctx, ogKindTexDoc)
	if err != nil {
		return nil, err
	}
	docs := make([]types.Document, 0, len(objs))
	for _, obj := range objs {
		var rec types.Document
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if ownerID != "" && rec.OwnerID != ownerID {
			continue
		}
		docs = append(docs, rec)
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].UpdatedAt.After(docs[j].UpdatedAt) })
	return docs, nil
}

// searchHeadRevision reads a document's head revision: through the
// lifecycle getter for a lifecycle document, the legacy getter otherwise.
func (s *Store) searchHeadRevision(ctx context.Context, doc types.Document) (types.Revision, error) {
	if strings.TrimSpace(doc.ComputerID) != "" || strings.TrimSpace(doc.TrajectoryID) != "" {
		return s.GetLifecycleRevision(ctx, doc.OwnerID, doc.ComputerID, doc.CurrentRevisionID)
	}
	return s.GetTextureRevisionOG(ctx, doc.OwnerID, doc.CurrentRevisionID)
}

// UpdateDocument updates an existing document record.
func (s *Store) UpdateDocument(ctx context.Context, doc types.Document) error {
	return s.UpdateTextureDocumentOG(ctx, doc)
}

// UpdateTextureDocumentTitleAuthority is the sole product writer for document
// title projections. Lifecycle documents use a scoped conditional object-graph
// write; legacy documents retain their current revision unchanged.
func (s *Store) UpdateTextureDocumentTitleAuthority(ctx context.Context, docID, ownerID, computerID, title string) (types.Document, error) {
	docID, ownerID, computerID = strings.TrimSpace(docID), strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	if docID == "" || ownerID == "" {
		return types.Document{}, ErrNotFound
	}
	if computerID != "" {
		document, err := s.UpdateLifecycleDocumentTitleAuthority(ctx, ownerID, computerID, docID, title)
		if err == nil || !errors.Is(err, ErrNotFound) {
			return document, err
		}
	}

	s.textureRevMu.Lock()
	defer s.textureRevMu.Unlock()
	document, err := s.GetDocument(ctx, docID, ownerID)
	if err != nil {
		return types.Document{}, err
	}
	if strings.TrimSpace(document.TrajectoryID) != "" {
		return types.Document{}, ErrLifecycleAuthorityRequired
	}
	document.Title, document.UpdatedAt = title, time.Now().UTC()
	if err := s.UpdateTextureDocumentOG(ctx, document); err != nil {
		return types.Document{}, err
	}
	return document, nil
}

// ArchiveTextureDocumentAuthority is the product deletion boundary. It records
// logical archival and preserves revision, source, and lifecycle evidence.
func (s *Store) ArchiveTextureDocumentAuthority(ctx context.Context, docID, ownerID, computerID string) (types.Document, error) {
	docID, ownerID, computerID = strings.TrimSpace(docID), strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	if docID == "" || ownerID == "" {
		return types.Document{}, ErrNotFound
	}
	var document types.Document
	var err error
	if computerID != "" {
		document, err = s.GetLifecycleDocument(ctx, ownerID, computerID, docID)
	}
	if computerID == "" || errors.Is(err, ErrNotFound) {
		document, err = s.GetDocument(ctx, docID, ownerID)
	}
	if err != nil {
		return types.Document{}, err
	}
	if document.ArchivedAt != nil {
		return document, nil
	}
	if trajectoryID := strings.TrimSpace(document.TrajectoryID); trajectoryID != "" {
		trajectory, getErr := s.GetLifecycleTrajectory(ctx, ownerID, document.ComputerID, trajectoryID)
		if getErr != nil {
			return types.Document{}, getErr
		}
		if trajectory.Status == types.TrajectoryLive {
			return types.Document{}, ErrLifecycleInvalidTransition
		}
		request := types.ArchiveLifecycleArtifactRequest{
			OwnerID:                  ownerID,
			ComputerID:               document.ComputerID,
			CommandID:                "archive:" + docID + ":" + document.CurrentRevisionID,
			TrajectoryID:             trajectoryID,
			ExpectedLifecycleVersion: trajectory.LifecycleVersion,
			ExpectedHeadRevisionID:   document.CurrentRevisionID,
			Reason:                   "owner archived document",
		}
		request.CommandDigest, err = ComputeArchiveLifecycleArtifactDigest(request)
		if err != nil {
			return types.Document{}, err
		}
		result, archiveErr := s.ArchiveLifecycleArtifact(ctx, request)
		if archiveErr != nil {
			return types.Document{}, archiveErr
		}
		if result.Document == nil {
			return types.Document{}, fmt.Errorf("archive texture document: reducer returned no document")
		}
		return *result.Document, nil
	}

	s.textureRevMu.Lock()
	defer s.textureRevMu.Unlock()
	document, err = s.GetDocument(ctx, docID, ownerID)
	if err != nil {
		return types.Document{}, err
	}
	if document.ArchivedAt != nil {
		return document, nil
	}
	if strings.TrimSpace(document.TrajectoryID) != "" {
		return types.Document{}, ErrConcurrentStateChange
	}
	now := time.Now().UTC()
	document.ArchivedAt, document.UpdatedAt = &now, now
	if err := s.UpdateTextureDocumentOG(ctx, document); err != nil {
		return types.Document{}, err
	}
	return document, nil
}

// GetDocumentAlias resolves a file-browser alias to its canonical document ID.
func (s *Store) GetDocumentAlias(ctx context.Context, ownerID, sourcePath string) (string, error) {
	ownerID = strings.TrimSpace(ownerID)
	sourcePath = strings.TrimSpace(sourcePath)
	computerID := s.aliasComputerID()
	query := `SELECT doc_id FROM texture_document_aliases WHERE owner_id = ? AND computer_id = ? AND source_path = ?`
	args := []any{ownerID, computerID, sourcePath}
	if computerID == "" {
		query = `SELECT doc_id FROM texture_document_aliases WHERE owner_id = ? AND (computer_id = ? OR computer_id = '') AND source_path = ?`
		args = []any{ownerID, computerID, sourcePath}
	}
	row := s.textureHandle().QueryRowContext(ctx, query, args...)
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
	ownerID = strings.TrimSpace(ownerID)
	docID = strings.TrimSpace(docID)
	computerID := s.aliasComputerID()
	query := `SELECT source_path
		   FROM texture_document_aliases
		  WHERE owner_id = ? AND (computer_id = ? OR computer_id = '') AND doc_id = ?
		  ORDER BY CASE
		             WHEN LOWER(source_path) LIKE '%.texture' THEN 0
		             ELSE 1
		           END, updated_at DESC
		  LIMIT 1`
	row := s.textureHandle().QueryRowContext(ctx, query, ownerID, computerID, docID)
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
	ownerID = strings.TrimSpace(ownerID)
	sourcePath = strings.TrimSpace(sourcePath)
	docID = strings.TrimSpace(docID)
	computerID := s.aliasComputerID()
	if s.ProjectionTapeBound() {
		proj := computerevent.TextureDocumentAliasProjection{
			OwnerID:    ownerID,
			ComputerID: computerID,
			SourcePath: sourcePath,
			DocID:      docID,
			CreatedAt:  updatedAt.UTC().Format(time.RFC3339Nano),
			UpdatedAt:  updatedAt.UTC().Format(time.RFC3339Nano),
		}
		body, err := json.Marshal(proj)
		if err != nil {
			return fmt.Errorf("store: marshal projected texture alias: %w", err)
		}
		return s.projectionTape.appendOps(ctx, []computerevent.ProjectionOp{{
			Kind: computerevent.ProjectionOpTextureDocumentAlias,
			Body: body,
		}})
	}
	_, err := s.textureHandle().ExecContext(ctx,
		`INSERT INTO texture_document_aliases (owner_id, computer_id, source_path, doc_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT (owner_id, computer_id, source_path) DO UPDATE SET
		   doc_id = excluded.doc_id,
		   updated_at = excluded.updated_at`,
		ownerID,
		computerID,
		sourcePath,
		docID,
		updatedAt.UTC(),
		updatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("upsert texture document alias: %w", err)
	}
	return nil
}

// deleteDocumentPhysicalForTest is isolated migration/test tooling. Product
// deletion uses ArchiveTextureDocumentAuthority and never erases history.
func (s *Store) deleteDocumentPhysicalForTest(ctx context.Context, docID, ownerID string) error {
	docObj, err := s.ogGetByKey(ctx, ogKindTexDoc, "doc_id", docID)
	if err != nil {
		if errors.Is(err, objectgraph.ErrNotFound) {
			return fmt.Errorf("%w: document %s for owner %s", ErrNotFound, docID, ownerID)
		}
		return fmt.Errorf("delete texture document: %w", err)
	}
	var existing types.Document
	if err := ogDecode(docObj, &existing); err != nil {
		return fmt.Errorf("delete texture document: %w", err)
	}
	if existing.OwnerID != ownerID {
		return fmt.Errorf("%w: document %s for owner %s", ErrNotFound, docID, ownerID)
	}
	if strings.TrimSpace(existing.TrajectoryID) != "" {
		return ErrLifecycleAuthorityRequired
	}
	// Delete revisions from OG. Use a large limit to avoid truncation
	// for documents with many revisions.
	revs, err := s.ListTextureRevisionsByDocOG(ctx, ownerID, docID, 100000)
	if err != nil {
		return fmt.Errorf("delete texture document: list revisions: %w", err)
	}
	for _, rev := range revs {
		// Delete source refs for this revision from OG.
		refs, err := s.ListTextureSourceRefsByRevisionOG(ctx, ownerID, docID, rev.RevisionID, 100000)
		if err == nil {
			for _, ref := range refs {
				refObj, err := s.ogGetByKey(ctx, TextureSourceRefObjectKind, "ref_version_key", ref.CanonicalID+"\x00"+ref.VersionID)
				if err == nil {
					_ = s.ogDelete(ctx, refObj.CanonicalID)
				}
			}
		}
		obj, err := s.ogGetByKey(ctx, ogKindTexRev, "revision_id", rev.RevisionID)
		if err == nil {
			_ = s.ogDelete(ctx, obj.CanonicalID)
		}
	}

	// Delete decisions from OG. Use a large limit to avoid truncation.
	decisions, err := s.ListTextureDecisionsByDocOG(ctx, ownerID, docID, 100000)
	if err != nil {
		return fmt.Errorf("delete texture document: list decisions: %w", err)
	}
	for _, dec := range decisions {
		obj, err := s.ogGetByKey(ctx, ogKindTexDecision, "decision_id", dec.DecisionID)
		if err == nil {
			_ = s.ogDelete(ctx, obj.CanonicalID)
		}
	}

	// Delete aliases from SQL.
	computerID := s.aliasComputerID()
	_, _ = s.textureHandle().ExecContext(ctx,
		`DELETE FROM texture_document_aliases WHERE doc_id = ? AND owner_id = ? AND (computer_id = ? OR computer_id = '')`,
		docID, ownerID, computerID,
	)

	// Delete the document from OG.
	return s.ogDelete(ctx, docObj.CanonicalID)
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

// commitTextureHeadAuthority is the single artifact-head transition rule used
// by both ordinary Texture revisions and reducer-owned lifecycle revisions.
// Persistence remains caller-owned so a lifecycle reducer can include the head
// transition in its larger serializable transaction.
func commitTextureHeadAuthority(document types.Document, head *types.Revision, revision types.Revision, now time.Time) (types.Document, types.Revision, error) {
	currentHead := strings.TrimSpace(document.CurrentRevisionID)
	expectedHead := strings.TrimSpace(revision.ParentRevisionID)
	if document.ArchivedAt != nil {
		return types.Document{}, types.Revision{}, ErrLifecycleInvalidTransition
	}
	if currentHead != expectedHead {
		return types.Document{}, types.Revision{}, fmt.Errorf("%w: document %s current head %s does not match parent %s", ErrStaleDocumentHead, revision.DocID, currentHead, expectedHead)
	}
	parentHash := ""
	if currentHead == "" {
		revision.VersionNumber = 0
	} else {
		if head == nil || strings.TrimSpace(head.RevisionID) != currentHead {
			return types.Document{}, types.Revision{}, fmt.Errorf("%w: current head revision unavailable", ErrStaleDocumentHead)
		}
		revision.VersionNumber = head.VersionNumber + 1
		parentHash = head.RevisionHash
	}
	expectedHash := types.ComputeStructuredRevisionHash(parentHash, revision.Content, revision.BodyDoc, revision.SourceEntities, revision.Provenance)
	if revision.RevisionHash != "" && revision.RevisionHash != expectedHash {
		return types.Document{}, types.Revision{}, fmt.Errorf("%w: revision hash mismatch", ErrLifecycleInvalidTransition)
	}
	revision.RevisionHash = expectedHash
	if revision.CreatedAt.IsZero() {
		revision.CreatedAt = now.UTC()
	}
	document.CurrentRevisionID = revision.RevisionID
	document.UpdatedAt = revision.CreatedAt
	return document, revision, nil
}

// CreateRevision inserts a new revision record and updates the document's
// current_revision_id if this is the latest revision.
func (s *Store) CreateRevision(ctx context.Context, rev types.Revision) error {
	return s.createRevision(ctx, rev, TextureSourceGraphWriteSet{})
}

// CreateRevisionWithSourceGraph inserts a Texture revision and its graph-shaped
// source records in the same Texture transaction. If any source write fails, the
// document head is not advanced.
func (s *Store) CreateRevisionWithSourceGraph(ctx context.Context, rev types.Revision, graph TextureSourceGraphWriteSet) error {
	return s.createRevision(ctx, rev, graph)
}

func (s *Store) createRevision(ctx context.Context, rev types.Revision, graph TextureSourceGraphWriteSet) error {
	if strings.TrimSpace(rev.TrajectoryID) != "" {
		return ErrLifecycleAuthorityRequired
	}
	preparedRev, _, _, err := prepareTextureRevisionV2(rev)
	if err != nil {
		return err
	}
	rev = preparedRev

	// Serialize revision creation per store to preserve the stale-head
	// compare-and-set semantics that the old SQL `WHERE current_revision_id
	// = expectedHead` provided atomically. Without this lock, two
	// concurrent calls can both read the same currentHead, both pass the
	// stale-head check, and both overwrite the document head.
	s.textureRevMu.Lock()
	defer s.textureRevMu.Unlock()

	doc, err := s.GetTextureDocumentOG(ctx, rev.OwnerID, rev.DocID)
	if err != nil {
		return fmt.Errorf("%w: document %s for owner %s", ErrNotFound, rev.DocID, rev.OwnerID)
	}
	if strings.TrimSpace(doc.TrajectoryID) != "" {
		return ErrLifecycleAuthorityRequired
	}
	currentHead := strings.TrimSpace(doc.CurrentRevisionID)

	var head *types.Revision
	if currentHead != "" {
		parentRevision, getErr := s.GetTextureRevisionOG(ctx, rev.OwnerID, currentHead)
		if getErr != nil {
			return fmt.Errorf("query current texture revision: %w", getErr)
		}
		head = &parentRevision
	}
	doc, rev, err = commitTextureHeadAuthority(doc, head, rev, rev.CreatedAt)
	if err != nil {
		return err
	}

	if s.ogStore == nil {
		return fmt.Errorf("texture revision: object graph not initialized")
	}
	now := rev.CreatedAt.UTC()
	revisionObj, err := unscopedGraphObject(ogKindTexRev, rev.OwnerID, rev.RevisionID, rev, map[string]any{
		"revision_id": rev.RevisionID, "doc_id": rev.DocID, "author_kind": string(rev.AuthorKind),
		"author_label": rev.AuthorLabel, "version_number": rev.VersionNumber,
		"parent_revision_id": rev.ParentRevisionID, "revision_hash": rev.RevisionHash,
		"created_at": now.Format(time.RFC3339Nano),
	}, now)
	if err != nil {
		return err
	}
	documentObj, err := s.ogGetByKey(ctx, ogKindTexDoc, "doc_id", doc.DocID)
	if err != nil {
		return err
	}
	documentBody, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	documentMetadata, err := objectgraph.NormalizeMetadata(map[string]any{
		"doc_id": doc.DocID, "title": doc.Title, "current_revision_id": doc.CurrentRevisionID,
		"created_at": doc.CreatedAt.UTC().Format(time.RFC3339Nano), "updated_at": doc.UpdatedAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return err
	}
	documentUpdated := documentObj
	documentUpdated.Body, documentUpdated.Metadata, documentUpdated.UpdatedAt = documentBody, documentMetadata, doc.UpdatedAt.UTC()
	documentUpdated.ContentHash = objectgraph.ContentHash(documentUpdated.ObjectKind, documentUpdated.Body, documentUpdated.Metadata)
	sourceObjects, sourceConditions, err := s.lifecycleSourceGraphBatch(ctx, rev, graph, now)
	if err != nil {
		return fmt.Errorf("texture source graph batch: %w", err)
	}
	conditions := append([]objectgraph.ObjectCondition{
		{CanonicalID: documentObj.CanonicalID, Exists: true, ExpectedContentHash: documentObj.ContentHash},
		{CanonicalID: revisionObj.CanonicalID},
	}, sourceConditions...)
	edgeMetadata := json.RawMessage(`{}`)
	documentEdgeID, err := objectgraph.BuildEdgeID(revisionObj.CanonicalID, documentObj.CanonicalID, ogEdgeDocRevision, edgeMetadata)
	if err != nil {
		return err
	}
	edges := []objectgraph.Edge{{EdgeID: documentEdgeID, FromID: revisionObj.CanonicalID, ToID: documentObj.CanonicalID, Kind: ogEdgeDocRevision, Metadata: edgeMetadata, CreatedAt: now}}
	if currentHead != "" {
		headObj, getErr := s.ogGetByKey(ctx, ogKindTexRev, "revision_id", currentHead)
		if getErr != nil {
			return getErr
		}
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: headObj.CanonicalID, Exists: true, ExpectedContentHash: headObj.ContentHash})
		parentEdgeID, buildErr := objectgraph.BuildEdgeID(revisionObj.CanonicalID, headObj.CanonicalID, ogEdgeRevParent, edgeMetadata)
		if buildErr != nil {
			return buildErr
		}
		edges = append(edges, objectgraph.Edge{EdgeID: parentEdgeID, FromID: revisionObj.CanonicalID, ToID: headObj.CanonicalID, Kind: ogEdgeRevParent, Metadata: edgeMetadata, CreatedAt: now})
	}
	objects := append([]objectgraph.Object{documentUpdated, revisionObj}, sourceObjects...)
	if err := s.ogStore.PutBatchConditional(ctx, conditions, objectgraph.Batch{Objects: objects, Edges: edges}); err != nil {
		if errors.Is(err, objectgraph.ErrConflict) {
			return ErrStaleDocumentHead
		}
		return fmt.Errorf("commit texture head: %w", err)
	}
	return nil
}

// PatchRevisionMetadata merges publication metadata into a manual revision.
// Lifecycle revisions are immutable; their evidence refs belong to the reducer.
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
	if strings.TrimSpace(rev.TrajectoryID) != "" {
		return ErrLifecycleAuthorityRequired
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
	rev.Metadata = json.RawMessage(merged)
	if err := s.CreateTextureRevisionOG(ctx, rev); err != nil {
		return err
	}
	return nil
}

// GetRevision returns the revision with the given revision ID, scoped to
// the given owner.
func (s *Store) GetRevision(ctx context.Context, revisionID, ownerID string) (types.Revision, error) {
	return s.GetTextureRevisionOG(ctx, ownerID, revisionID)
}

// GetRevisionUnscoped returns the revision without owner scoping.
// Used internally for diff/blame computation where the revision chain
// is already known to belong to the same owner.
func (s *Store) GetRevisionUnscoped(ctx context.Context, revisionID string) (types.Revision, error) {
	obj, err := s.ogGetByKey(ctx, ogKindTexRev, "revision_id", revisionID)
	if err != nil {
		if err == objectgraph.ErrNotFound {
			return types.Revision{}, ErrNotFound
		}
		return types.Revision{}, err
	}
	var rec types.Revision
	if err := ogDecode(obj, &rec); err != nil {
		return types.Revision{}, err
	}
	if strings.TrimSpace(rec.ComputerID) != "" || strings.TrimSpace(rec.TrajectoryID) != "" {
		return types.Revision{}, ErrLifecycleAuthorityRequired
	}
	return rec, nil
}

// ListRevisionsByDoc returns revisions for the given document, scoped to
// the given owner, ordered by durable version number descending (newest first),
// limited to the given count.
func (s *Store) ListRevisionsByDoc(ctx context.Context, docID, ownerID string, limit int) ([]types.Revision, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.ListTextureRevisionsByDocOG(ctx, ownerID, docID, limit)
}
func (s *Store) ListRevisionsByScope(ctx context.Context, docID, ownerID, computerID string, limit int) ([]types.Revision, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.ListTextureRevisionsByScopeOG(ctx, ownerID, computerID, docID, limit)
}

// TextureRevisionStatsByScope returns revision count and max version_number for
// one scoped document via a metadata-only edge join — no revision bodies are
// decoded. Falls back to the full list when the doc has no revision edges
// (legacy rows) so the numbers stay correct across the edge migration.
func (s *Store) TextureRevisionStatsByScope(ctx context.Context, docID, ownerID, computerID string) (count int, maxVersion int, err error) {
	ownerID, computerID, docID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(docID)
	if ownerID == "" || computerID == "" || docID == "" {
		return 0, -1, fmt.Errorf("texture revision stats: owner_id, computer_id, and doc_id are required")
	}
	docCanonicalID, cerr := objectgraph.BuildCanonicalID(ogKindTexDoc, ownerID, objectgraph.StableSuffixFromKey(lifecycleScopedKey(computerID, docID)))
	if cerr == nil {
		if count, maxVersion, err = s.ogCountObjectsByEdgeTo(ctx, ogKindTexRev, docCanonicalID, ogEdgeDocRevision); err == nil && count > 0 {
			return count, maxVersion, nil
		}
	}
	revs, lerr := s.ListTextureRevisionsByScopeOG(ctx, ownerID, computerID, docID, 100000)
	if lerr != nil {
		return 0, -1, lerr
	}
	maxVersion = -1
	for _, rev := range revs {
		if rev.VersionNumber > maxVersion {
			maxVersion = rev.VersionNumber
		}
	}
	return len(revs), maxVersion, nil
}

func (s *Store) CountRevisionsByScope(ctx context.Context, docID, ownerID, computerID string) (int, error) {
	revs, err := s.ListTextureRevisionsByScopeOG(ctx, ownerID, computerID, docID, 100000)
	if err != nil {
		return 0, fmt.Errorf("count scoped texture revisions: %w", err)
	}
	return len(revs), nil
}

func (s *Store) CurrentVersionNumberByScope(ctx context.Context, docID, ownerID, computerID string) (int, error) {
	revs, err := s.ListTextureRevisionsByScopeOG(ctx, ownerID, computerID, docID, 100000)
	if err != nil {
		return -1, fmt.Errorf("query current scoped texture revision version number: %w", err)
	}
	maxVersion := -1
	for _, revision := range revs {
		if revision.VersionNumber > maxVersion {
			maxVersion = revision.VersionNumber
		}
	}
	return maxVersion, nil
}

func (s *Store) CountRevisionsByDoc(ctx context.Context, docID, ownerID string) (int, error) {
	// Use a large limit to count all revisions. Limit 0 would be
	// rewritten to the default page size by the OG helper.
	revs, err := s.ListTextureRevisionsByDocOG(ctx, ownerID, docID, 100000)
	if err != nil {
		return 0, fmt.Errorf("count texture revisions: %w", err)
	}
	return len(revs), nil
}

func (s *Store) CurrentVersionNumberByDoc(ctx context.Context, docID, ownerID string) (int, error) {
	// Use a large limit to scan all revisions for the max version number.
	revs, err := s.ListTextureRevisionsByDocOG(ctx, ownerID, docID, 100000)
	if err != nil {
		return -1, fmt.Errorf("query current texture revision version number: %w", err)
	}
	maxVersion := -1
	for _, r := range revs {
		if r.VersionNumber > maxVersion {
			maxVersion = r.VersionNumber
		}
	}
	return maxVersion, nil
}

// ----- History -----

func (s *Store) GetHistory(ctx context.Context, docID, ownerID string, limit int) ([]types.HistoryEntry, error) {
	return s.getHistory(ctx, docID, ownerID, "", limit)
}

func (s *Store) GetHistoryByScope(ctx context.Context, docID, ownerID, computerID string, limit int) ([]types.HistoryEntry, error) {
	return s.getHistory(ctx, docID, ownerID, computerID, limit)
}

// GetHistory returns the revision history for a document by walking the
// explicit parent chain through indexed revision rows. Revision objects are
// immutable and content-addressed, so the chain — not Dolt commit history or
// AS OF snapshots — is the history authority. The dirty-gated checkpoint
// above preserves the one-batched-commit audit shape; reads never depend on
// it.
func (s *Store) getHistory(ctx context.Context, docID, ownerID, computerID string, limit int) ([]types.HistoryEntry, error) {
	if limit <= 0 {
		limit = 50
	}

	var doc types.Document
	var err error
	if strings.TrimSpace(computerID) != "" {
		doc, err = s.GetLifecycleDocument(ctx, ownerID, computerID, docID)
	} else {
		doc, err = s.GetDocument(ctx, docID, ownerID)
	}
	if err != nil {
		return nil, err
	}
	if doc.CurrentRevisionID == "" {
		return []types.HistoryEntry{}, nil
	}
	currentID := doc.CurrentRevisionID
	entries := make([]types.HistoryEntry, 0, limit)
	visited := make(map[string]bool, limit)
	for len(entries) < limit && currentID != "" {
		if visited[currentID] {
			return nil, fmt.Errorf("texture revision chain cycles at %s", currentID)
		}
		visited[currentID] = true
		rev, err := s.GetRevision(ctx, currentID, ownerID)
		if err != nil {
			return nil, fmt.Errorf("load native texture history revision %s: %w", currentID, ErrNotFound)
		}
		if rev.DocID != docID ||
			!((strings.TrimSpace(computerID) == "" && strings.TrimSpace(rev.ComputerID) == "") ||
				strings.TrimSpace(rev.ComputerID) == strings.TrimSpace(computerID)) {
			return nil, fmt.Errorf("load native texture history revision %s: %w", currentID, ErrNotFound)
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
	head, err := s.GetRevision(ctx, revisionID, ownerID)
	if err != nil {
		return types.BlameResult{}, err
	}
	return s.getBlame(ctx, head)
}

func (s *Store) GetBlameByScope(ctx context.Context, revisionID, ownerID, computerID string) (types.BlameResult, error) {
	head, err := s.GetLifecycleRevision(ctx, ownerID, computerID, revisionID)
	if err != nil {
		return types.BlameResult{}, err
	}
	return s.getBlame(ctx, head)
}

func (s *Store) getBlame(ctx context.Context, head types.Revision) (types.BlameResult, error) {
	chain, err := s.collectRevisionChain(ctx, head)
	if err != nil {
		return types.BlameResult{}, fmt.Errorf("collect revision chain: %w", err)
	}
	return types.BlameResult{
		RevisionID: head.RevisionID,
		DocID:      head.DocID,
		Sections:   computeBlame(chain, head),
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

		var parent types.Revision
		var err error
		if computerID := strings.TrimSpace(head.ComputerID); computerID != "" {
			parent, err = s.GetLifecycleRevision(ctx, head.OwnerID, computerID, parentID)
		} else {
			parent, err = s.GetRevisionUnscoped(ctx, parentID)
		}
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

// ----- Agent mutation tracking (VAL-CROSS-122: idempotent revision) -----

// AgentMutation represents an in-flight or completed appagent-driven document
// mutation. It tracks the mapping from a runtime run to a document mutation,
// enabling idempotent handling so that renewal/retry does not create a
// duplicate canonical revision.
type AgentMutation struct {
	DocID               string     `json:"doc_id"`
	RunID               string     `json:"loop_id"`
	OwnerID             string     `json:"owner_id"`
	ComputerID          string     `json:"computer_id,omitempty"`
	State               string     `json:"state"` // "pending", "sleeping", "completed", "failed", "deferred"
	ScheduledMessageSeq int64      `json:"scheduled_message_seq,omitempty"`
	RevisionID          string     `json:"revision_id,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	CompletedAt         *time.Time `json:"completed_at,omitempty"`
}

func textureMutationProjection(m AgentMutation) computerevent.TextureAgentMutationProjection {
	var completedAt *string
	if m.CompletedAt != nil {
		value := m.CompletedAt.UTC().Format(time.RFC3339Nano)
		completedAt = &value
	}
	return computerevent.TextureAgentMutationProjection{
		DocID: m.DocID, RunID: m.RunID, OwnerID: m.OwnerID, ComputerID: m.ComputerID,
		State: m.State, ScheduledMessageSeq: m.ScheduledMessageSeq, RevisionID: m.RevisionID,
		CreatedAt: m.CreatedAt.UTC().Format(time.RFC3339Nano), CompletedAt: completedAt,
	}
}

func (s *Store) currentTextureMutation(ctx context.Context, ownerID, computerID, docID, runID string) (*AgentMutation, error) {
	var row *sql.Row
	if strings.TrimSpace(docID) == "" {
		row = s.textureHandle().QueryRowContext(ctx,
			`SELECT doc_id, loop_id, owner_id, computer_id, state, scheduled_message_seq, revision_id, created_at, completed_at
			   FROM texture_agent_mutations
			  WHERE owner_id=? AND computer_id=? AND loop_id=?
			  LIMIT 1`,
			strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(runID))
	} else {
		row = s.textureHandle().QueryRowContext(ctx,
			`SELECT doc_id, loop_id, owner_id, computer_id, state, scheduled_message_seq, revision_id, created_at, completed_at
			   FROM texture_agent_mutations
			  WHERE owner_id=? AND computer_id=? AND doc_id=? AND loop_id=?`,
			strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(docID), strings.TrimSpace(runID))
	}
	return scanAgentMutation(row)
}

func (s *Store) appendTextureMutationProjection(ctx context.Context, mutation computerevent.TextureAgentMutationProjection) error {
	body, err := json.Marshal(mutation)
	if err != nil {
		return fmt.Errorf("marshal Texture agent mutation projection: %w", err)
	}
	if err := s.projectionTape.appendOps(ctx, []computerevent.ProjectionOp{{
		Kind:        computerevent.ProjectionOpTextureAgentMutation,
		CanonicalID: mutation.OwnerID + "\x00" + mutation.ComputerID + "\x00" + mutation.DocID + "\x00" + mutation.RunID,
		Body:        body,
	}}); err != nil {
		return fmt.Errorf("append Texture agent mutation projection: %w", err)
	}
	return nil
}

// projectTextureMutation applies one live state transition as a full row
// snapshot. The expected-state and revision guards are checked again by the
// reducer inside FinalizeBatch, so a stale event cannot silently overwrite a
// newer projection.
func (s *Store) projectTextureMutation(ctx context.Context, ownerID, computerID, docID, runID string, expectedStates []string, requireRevision *bool, mutate func(*computerevent.TextureAgentMutationProjection)) (bool, error) {
	current, err := s.currentTextureMutation(ctx, ownerID, computerID, docID, runID)
	if err != nil || current == nil {
		return false, err
	}
	stateMatched := len(expectedStates) == 0
	for _, expected := range expectedStates {
		if current.State == expected {
			stateMatched = true
			break
		}
	}
	if !stateMatched {
		return true, ErrMutationAlreadyCompleted
	}
	projection := textureMutationProjection(*current)
	projection.ExpectedStates = append([]string(nil), expectedStates...)
	projection.RequireRevision = requireRevision
	if mutate != nil {
		mutate(&projection)
	}
	return true, s.appendTextureMutationProjection(ctx, projection)
}

// CreateAgentMutation records a new in-flight appagent mutation. Scoped rows
// are idempotent by owner, computer, document, and run; empty computer scope is
// reserved for legacy rows.
func (s *Store) CreateAgentMutation(ctx context.Context, m AgentMutation) error {
	s.textureMutationMu.Lock()
	defer s.textureMutationMu.Unlock()
	if s.projectionTape != nil {
		current, err := s.currentTextureMutation(ctx, m.OwnerID, m.ComputerID, m.DocID, m.RunID)
		if err != nil {
			return fmt.Errorf("read texture agent mutation before projection: %w", err)
		}
		if current != nil {
			return nil
		}
		projection := textureMutationProjection(m)
		projection.CreateOnly = true
		if err := s.appendTextureMutationProjection(ctx, projection); err != nil {
			return err
		}
		return nil
	}
	var completedAt any
	if m.CompletedAt != nil {
		completedAt = m.CompletedAt.UTC()
	}
	_, err := s.textureHandle().ExecContext(ctx,
		`INSERT INTO texture_agent_mutations (doc_id, loop_id, owner_id, computer_id, state, scheduled_message_seq, revision_id, created_at, completed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT DO NOTHING`,
		strings.TrimSpace(m.DocID),
		strings.TrimSpace(m.RunID),
		strings.TrimSpace(m.OwnerID),
		strings.TrimSpace(m.ComputerID),
		m.State,
		m.ScheduledMessageSeq,
		m.RevisionID,
		m.CreatedAt.UTC(),
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
func (s *Store) GetPendingAgentMutationByDoc(ctx context.Context, ownerID, computerID, docID string) (*AgentMutation, error) {
	row := s.textureHandle().QueryRowContext(ctx,
		`SELECT doc_id, loop_id, owner_id, computer_id, state, scheduled_message_seq, revision_id, created_at, completed_at
		   FROM texture_agent_mutations
		  WHERE owner_id = ? AND computer_id = ? AND doc_id = ? AND state = 'pending'
		  ORDER BY created_at DESC
		  LIMIT 1`,
		strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(docID),
	)
	return scanAgentMutation(row)
}

// CountPendingAgentMutations returns the number of pending mutations scoped to
// one owner+computer. Used by the guest health surface so host-side lifecycle
// (idle sweep, pressure reclaim) can tell a computer with un-consumed desk work
// apart from a truly idle one — a pending mutation means the controller owes
// the doc another activation even when no run is currently in flight.
func (s *Store) CountPendingAgentMutations(ctx context.Context, ownerID, computerID string) (int, error) {
	var count int
	err := s.textureHandle().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM texture_agent_mutations
		  WHERE owner_id = ? AND computer_id = ? AND state = 'pending'`,
		strings.TrimSpace(ownerID), strings.TrimSpace(computerID),
	).Scan(&count)
	return count, err
}

// GetAgentMutationByRun returns the agent mutation for a specific run ID.
// This is used during run completion to check if the revision has already
// been created (VAL-CROSS-122: no duplicate canonical revision).
func (s *Store) GetAgentMutationByRun(ctx context.Context, ownerID, computerID, runID string) (*AgentMutation, error) {
	row := s.textureHandle().QueryRowContext(ctx,
		`SELECT doc_id, loop_id, owner_id, computer_id, state, scheduled_message_seq, revision_id, created_at, completed_at
		   FROM texture_agent_mutations
		  WHERE owner_id = ? AND computer_id = ? AND loop_id = ?
		  LIMIT 1`,
		strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(runID),
	)
	return scanAgentMutation(row)
}

// ListRecentAgentMutationsByDoc returns a document's mutation rows newest-first
// — the breaker's consecutive-dead-activation view. Projection-tape mode reads
// the same SQL rows (mutations are double-written), so this needs no tape path.
func (s *Store) ListRecentAgentMutationsByDoc(ctx context.Context, ownerID, computerID, docID string, limit int) ([]AgentMutation, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.textureHandle().QueryContext(ctx,
		`SELECT doc_id, loop_id, owner_id, computer_id, state, scheduled_message_seq, revision_id, created_at, completed_at
		   FROM texture_agent_mutations
		  WHERE owner_id = ? AND computer_id = ? AND doc_id = ?
		  ORDER BY created_at DESC
		  LIMIT ?`,
		strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(docID), limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list recent texture agent mutations: %w", err)
	}
	defer rows.Close()
	var out []AgentMutation
	for rows.Next() {
		m, err := scanAgentMutation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

// CompleteAgentMutation marks an agent mutation as completed with the latest
// revision ID written by the run. It returns ErrMutationAlreadyCompleted if the
// mutation is no longer pending.
var ErrMutationAlreadyCompleted = errors.New("agent mutation already completed")

func (s *Store) CompleteAgentMutation(ctx context.Context, ownerID, computerID, runID, revisionID string) error {
	s.textureMutationMu.Lock()
	defer s.textureMutationMu.Unlock()
	now := time.Now().UTC()
	if s.projectionTape != nil {
		completedAt := time.Now().UTC().Format(time.RFC3339Nano)
		found, err := s.projectTextureMutation(ctx, ownerID, computerID, "", runID, []string{"pending"}, nil, func(m *computerevent.TextureAgentMutationProjection) {
			m.State = "completed"
			m.RevisionID = revisionID
			m.CompletedAt = &completedAt
		})
		if err != nil {
			return err
		}
		if !found {
			return ErrMutationAlreadyCompleted
		}
		return nil
	}
	result, err := s.textureHandle().ExecContext(ctx,
		`UPDATE texture_agent_mutations
		    SET state = 'completed',
		        revision_id = ?,
		        completed_at = ?
		  WHERE owner_id = ? AND computer_id = ? AND loop_id = ? AND state = 'pending'`,
		revisionID,
		now,
		strings.TrimSpace(ownerID),
		strings.TrimSpace(computerID),
		strings.TrimSpace(runID),
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

// FailAgentMutation marks an agent mutation as failed.
func (s *Store) FailAgentMutation(ctx context.Context, ownerID, computerID, runID string) error {
	s.textureMutationMu.Lock()
	defer s.textureMutationMu.Unlock()
	now := time.Now().UTC()
	if s.projectionTape != nil {
		completedAt := now.Format(time.RFC3339Nano)
		_, err := s.projectTextureMutation(ctx, ownerID, computerID, "", runID, []string{"pending"}, nil, func(m *computerevent.TextureAgentMutationProjection) {
			m.State = "failed"
			m.CompletedAt = &completedAt
		})
		if errors.Is(err, ErrMutationAlreadyCompleted) {
			return nil
		}
		return err
	}
	_, err := s.textureHandle().ExecContext(ctx,
		`UPDATE texture_agent_mutations
		    SET state = 'failed',
		        completed_at = ?
		  WHERE owner_id = ? AND computer_id = ? AND loop_id = ? AND state = 'pending'`,
		now,
		strings.TrimSpace(ownerID),
		strings.TrimSpace(computerID),
		strings.TrimSpace(runID),
	)
	if err != nil {
		return fmt.Errorf("fail texture agent mutation: %w", err)
	}
	return nil
}

// MarkAgentMutationStale clears a pending mutation whose owning activation is
// no longer active. This prevents stale pending rows from keeping the Texture
// editor in a perpetual "Revising..." state after recovery or missed completion
// reconciliation.
func (s *Store) MarkAgentMutationStale(ctx context.Context, ownerID, computerID, runID string) error {
	s.textureMutationMu.Lock()
	defer s.textureMutationMu.Unlock()
	now := time.Now().UTC()
	if s.projectionTape != nil {
		completedAt := now.Format(time.RFC3339Nano)
		_, err := s.projectTextureMutation(ctx, ownerID, computerID, "", runID, []string{"pending"}, nil, func(m *computerevent.TextureAgentMutationProjection) {
			m.State = "stale_activation"
			m.CompletedAt = &completedAt
		})
		if errors.Is(err, ErrMutationAlreadyCompleted) {
			return nil
		}
		return err
	}
	_, err := s.textureHandle().ExecContext(ctx,
		`UPDATE texture_agent_mutations
		    SET state = 'stale_activation',
		        completed_at = ?
		  WHERE owner_id = ? AND computer_id = ? AND loop_id = ? AND state = 'pending'`,
		now,
		strings.TrimSpace(ownerID),
		strings.TrimSpace(computerID),
		strings.TrimSpace(runID),
	)
	if err != nil {
		return fmt.Errorf("mark stale texture agent mutation: %w", err)
	}
	return nil
}

// SleepAgentMutation records that a durable Texture actor has quiesced after
// writing a revision and can be reactivated on later mailbox input.
func (s *Store) SleepAgentMutation(ctx context.Context, ownerID, computerID, runID string) error {
	s.textureMutationMu.Lock()
	defer s.textureMutationMu.Unlock()
	now := time.Now().UTC()
	if s.projectionTape != nil {
		requireRevision := true
		completedAt := now.Format(time.RFC3339Nano)
		found, err := s.projectTextureMutation(ctx, ownerID, computerID, "", runID, []string{"pending"}, &requireRevision, func(m *computerevent.TextureAgentMutationProjection) {
			m.State = "sleeping"
			m.CompletedAt = &completedAt
		})
		if err != nil {
			return err
		}
		if !found {
			return ErrMutationAlreadyCompleted
		}
		return nil
	}
	result, err := s.textureHandle().ExecContext(ctx,
		`UPDATE texture_agent_mutations
		    SET state = 'sleeping',
		        completed_at = ?
		  WHERE owner_id = ? AND computer_id = ? AND loop_id = ?
		    AND state = 'pending'
		    AND revision_id <> ''`,
		now,
		strings.TrimSpace(ownerID),
		strings.TrimSpace(computerID),
		strings.TrimSpace(runID),
	)
	if err != nil {
		return fmt.Errorf("sleep texture agent mutation: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check sleeping texture mutation rows: %w", err)
	}
	if rows == 0 {
		return ErrMutationAlreadyCompleted
	}
	return nil
}

// sleepAfterTurnRequireRevision derives the revision-presence assertion for
// SleepAgentMutationAfterTextureTurn from the projected row instead of the
// turn outcome. A non-revision turn says nothing about revisions attached by
// earlier turns: asserting absence for a revision-carrying row appends a batch
// that CASes and then fails FinalizeBatch forever (2026-09-03 seq 138612).
// Asserting the row's actual presence preserves it through the sleep and the
// guard passes; a concurrent attach between this read and finalize still fails
// closed at finalize (and, with the pre-CAS dry-run, before CAS).
func sleepAfterTurnRequireRevision(current *AgentMutation) bool {
	if current == nil {
		return false
	}
	return strings.TrimSpace(current.RevisionID) != ""
}

// SleepAgentMutationAfterTextureTurn passivates a mutation whose caller run
// committed a non-revision Texture outcome. A revision attached by an earlier
// turn is preserved through the sleep; only the pending→sleeping transition
// is asserted.
func (s *Store) SleepAgentMutationAfterTextureTurn(ctx context.Context, ownerID, computerID, trajectoryID, runID string) error {
	s.textureMutationMu.Lock()
	defer s.textureMutationMu.Unlock()
	turn, err := s.GetAppliedTextureTurnByCallerRun(ctx, ownerID, computerID, trajectoryID, runID)
	if err != nil {
		return err
	}
	if turn.TextureTurn == nil || turn.TextureTurn.Outcome == types.TextureTurnRevision {
		return ErrLifecycleInvalidTransition
	}
	now := time.Now().UTC()
	if s.projectionTape != nil {
		current, readErr := s.currentTextureMutation(ctx, ownerID, computerID, "", runID)
		if readErr != nil {
			return readErr
		}
		requireRevision := sleepAfterTurnRequireRevision(current)
		completedAt := now.Format(time.RFC3339Nano)
		found, err := s.projectTextureMutation(ctx, ownerID, computerID, "", runID, []string{"pending"}, &requireRevision, func(m *computerevent.TextureAgentMutationProjection) {
			m.State = "sleeping"
			m.CompletedAt = &completedAt
		})
		if err != nil {
			return err
		}
		if !found {
			return ErrMutationAlreadyCompleted
		}
		return nil
	}
	result, err := s.textureHandle().ExecContext(ctx, `UPDATE texture_agent_mutations SET state = 'sleeping', completed_at = ? WHERE owner_id = ? AND computer_id = ? AND loop_id = ? AND state = 'pending'`, now, strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(runID))
	if err != nil {
		return fmt.Errorf("sleep Texture mutation after turn: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrMutationAlreadyCompleted
	}
	return nil
}

// ReactivateAgentMutation reopens a stale or sleeping mutation for the same
// durable Texture actor activation. It intentionally does not revive completed,
// failed, cancelled, or deferred mutations.
func (s *Store) ReactivateAgentMutation(ctx context.Context, ownerID, computerID, runID string, scheduledMessageSeq int64) error {
	s.textureMutationMu.Lock()
	defer s.textureMutationMu.Unlock()
	now := time.Now().UTC()
	if s.projectionTape != nil {
		found, err := s.projectTextureMutation(ctx, ownerID, computerID, "", runID, []string{"stale_activation", "sleeping"}, nil, func(m *computerevent.TextureAgentMutationProjection) {
			m.State = "pending"
			m.ScheduledMessageSeq = scheduledMessageSeq
			m.CompletedAt = nil
			m.CreatedAt = now.Format(time.RFC3339Nano)
		})
		if err != nil {
			return err
		}
		if !found {
			return ErrMutationAlreadyCompleted
		}
		return nil
	}
	result, err := s.textureHandle().ExecContext(ctx,
		`UPDATE texture_agent_mutations
		    SET state = 'pending',
		        scheduled_message_seq = ?,
		        completed_at = NULL,
		        created_at = ?
		  WHERE owner_id = ? AND computer_id = ? AND loop_id = ?
		    AND state IN ('stale_activation', 'sleeping')`,
		scheduledMessageSeq,
		now,
		strings.TrimSpace(ownerID),
		strings.TrimSpace(computerID),
		strings.TrimSpace(runID),
	)
	if err != nil {
		return fmt.Errorf("reactivate texture agent mutation: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check reactivated texture mutation rows: %w", err)
	}
	if rows == 0 {
		return ErrMutationAlreadyCompleted
	}
	return nil
}

// CreateTextureDecision inserts an off-document Texture decision note.
func (s *Store) CreateTextureDecision(ctx context.Context, rec types.TextureDecisionRecord) error {
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now().UTC()
	}
	return s.CreateTextureDecisionOG(ctx, rec)
}

// ListTextureDecisionsByDocument returns recent off-document decision notes for a
// document, scoped to its owner.
func (s *Store) ListTextureDecisionsByDocument(ctx context.Context, ownerID, docID string, limit int) ([]types.TextureDecisionRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.ListTextureDecisionsByDocOG(ctx, ownerID, docID, limit)
}

// ListTextureDecisionsByTrajectory returns recent decision notes associated with
// a trajectory, scoped to the owner.
func (s *Store) ListTextureDecisionsByTrajectory(ctx context.Context, ownerID, trajectoryID string, limit int) ([]types.TextureDecisionRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.ListTextureDecisionsByTrajectoryOG(ctx, ownerID, trajectoryID, limit)
}

// createEvidenceTable inserts an evidence record into the legacy agent_evidence
// table: the home while evidence.graph is off (graph_home.go).
func (s *Store) createEvidenceTable(ctx context.Context, rec types.EvidenceRecord) error {
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
		rec.CreatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("insert agent evidence: %w", err)
	}
	return nil
}

// getEvidenceTable reads one evidence record from the legacy table.
func (s *Store) getEvidenceTable(ctx context.Context, evidenceID, ownerID string) (types.EvidenceRecord, error) {
	row := s.textureHandle().QueryRowContext(ctx,
		`SELECT evidence_id, owner_id, agent_id, kind, source_uri, title, content, metadata_json, created_at
		   FROM agent_evidence
		  WHERE evidence_id = ? AND owner_id = ?`,
		evidenceID, ownerID,
	)
	return scanEvidence(row)
}

// listEvidenceByAgentTable lists the legacy table's evidence for an agent (or
// for every agent of the owner when agentID is empty), newest first.
func (s *Store) listEvidenceByAgentTable(ctx context.Context, ownerID, agentID string, limit int) ([]types.EvidenceRecord, error) {
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
	if strings.TrimSpace(string(rec.Metadata)) == "" {
		rec.Metadata = json.RawMessage("{}")
	}
	if strings.TrimSpace(string(rec.Provenance)) == "" {
		rec.Provenance = json.RawMessage("{}")
	}
	return s.CreateContentItemOG(ctx, rec)
}

// GetContentItem returns a content item scoped to the authenticated owner.
func (s *Store) GetContentItem(ctx context.Context, ownerID, contentID string) (types.ContentItem, error) {
	return s.GetContentItemOG(ctx, ownerID, contentID)
}

// ListContentItems lists recent content substrate records for an owner.
func (s *Store) ListContentItems(ctx context.Context, ownerID string, limit int) ([]types.ContentItem, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.ListContentItemsByOwnerOG(ctx, ownerID, limit)
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
		&m.ComputerID,
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

func (s *Store) aliasComputerID() string {
	if s == nil || s.projectionTape == nil {
		return ""
	}
	return strings.TrimSpace(s.projectionTape.computerID)
}
