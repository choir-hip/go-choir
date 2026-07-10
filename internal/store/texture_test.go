package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sourcecontract"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func textureTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "texture-test.db")
	s, err := OpenTextureWorkspace(dbPath)
	if err != nil {
		t.Fatalf("open texture test store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func testTextureBodyDoc(t *testing.T, docID, revisionID, content string) json.RawMessage {
	t.Helper()
	doc := texturedoc.StructuredTextureDoc{
		Schema: texturedoc.SchemaV1,
		Doc: texturedoc.Node{
			Type:  "doc",
			Attrs: map[string]any{"id": "doc-" + docID + "-" + revisionID},
			Content: []texturedoc.Node{{
				Type:    "paragraph",
				Attrs:   map[string]any{"id": "p-" + revisionID},
				Content: []texturedoc.Node{{Type: "text", Text: content}},
			}},
		},
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal test body_doc: %v", err)
	}
	return raw
}

func testTextureRevisionWithBodyDoc(t *testing.T, rev types.Revision) types.Revision {
	t.Helper()
	if rev.AuthorKind == types.AuthorAppAgent && len(strings.TrimSpace(string(rev.BodyDoc))) == 0 {
		rev.Content = strings.TrimRight(rev.Content, "\n")
		rev.BodyDoc = testTextureBodyDoc(t, rev.DocID, rev.RevisionID, rev.Content)
	}
	return rev
}

func TestOpenTextureWorkspaceUsesTextureDatabaseForFreshWorkspace(t *testing.T) {
	s := textureTestStore(t)

	var databaseName string
	if err := s.textureHandle().QueryRow("SELECT DATABASE()").Scan(&databaseName); err != nil {
		t.Fatalf("SELECT DATABASE(): %v", err)
	}
	if databaseName != textureDatabaseName {
		t.Fatalf("database = %q, want %q", databaseName, textureDatabaseName)
	}
}

// ----- Document CRUD -----

func TestTextureCreateDocument(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	doc := types.Document{
		DocID:   "doc-1",
		OwnerID: "user-1",
		Title:   "Test Document",
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	got, err := s.GetDocument(ctx, "doc-1", "user-1")
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if got.DocID != "doc-1" {
		t.Errorf("DocID = %q, want %q", got.DocID, "doc-1")
	}
	if got.OwnerID != "user-1" {
		t.Errorf("OwnerID = %q, want %q", got.OwnerID, "user-1")
	}
	if got.Title != "Test Document" {
		t.Errorf("Title = %q, want %q", got.Title, "Test Document")
	}
}

func TestTextureGetDocumentOwnerScope(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	doc := types.Document{
		DocID:   "doc-1",
		OwnerID: "user-1",
		Title:   "Owned by user-1",
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	// user-2 should not see user-1's document.
	_, err := s.GetDocument(ctx, "doc-1", "user-2")
	if err != ErrNotFound {
		t.Errorf("GetDocument as wrong owner: err=%v, want ErrNotFound", err)
	}
}

func TestTextureListDocumentsByOwner(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		doc := types.Document{
			DocID:   "doc-" + string(rune('a'+i)),
			OwnerID: "user-1",
			Title:   "Doc " + string(rune('a'+i)),
		}
		if err := s.CreateDocument(ctx, doc); err != nil {
			t.Fatalf("CreateDocument: %v", err)
		}
	}
	// Create a doc for another user.
	doc := types.Document{
		DocID:   "doc-x",
		OwnerID: "user-2",
		Title:   "Other User Doc",
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	docs, err := s.ListDocumentsByOwner(ctx, "user-1", 10)
	if err != nil {
		t.Fatalf("ListDocumentsByOwner: %v", err)
	}
	if len(docs) != 3 {
		t.Errorf("len(docs) = %d, want 3", len(docs))
	}
}

func TestTextureDecisionRecordsAreOwnerScopedAndDocumentScoped(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	doc := types.Document{
		DocID:     "doc-decision",
		OwnerID:   "user-1",
		Title:     "Decision doc",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	rec := types.TextureDecisionRecord{
		DecisionID:   "decision-1",
		OwnerID:      "user-1",
		DocID:        "doc-decision",
		RunID:        "run-texture-1",
		TrajectoryID: "trajectory-1",
		ActorID:      "texture:doc-decision",
		DecisionKind: "delegation_skipped",
		Reason:       "The owner supplied enough source material for this revision.",
		EvidenceRefs: []string{"rev-base", "source:owner-material"},
		NextAction:   "Edit directly.",
		CreatedAt:    now,
	}
	if err := s.CreateTextureDecision(ctx, rec); err != nil {
		t.Fatalf("CreateTextureDecision: %v", err)
	}

	records, err := s.ListTextureDecisionsByDocument(ctx, "user-1", "doc-decision", 10)
	if err != nil {
		t.Fatalf("ListTextureDecisionsByDocument: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("decisions len = %d, want 1", len(records))
	}
	got := records[0]
	if got.DecisionID != rec.DecisionID || got.DecisionKind != rec.DecisionKind || got.Reason != rec.Reason || got.NextAction != rec.NextAction {
		t.Fatalf("decision mismatch: %+v", got)
	}
	if len(got.EvidenceRefs) != 2 || got.EvidenceRefs[0] != "rev-base" || got.EvidenceRefs[1] != "source:owner-material" {
		t.Fatalf("evidence refs = %#v", got.EvidenceRefs)
	}
	trajectoryRecords, err := s.ListTextureDecisionsByTrajectory(ctx, "user-1", "trajectory-1", 10)
	if err != nil {
		t.Fatalf("ListTextureDecisionsByTrajectory: %v", err)
	}
	if len(trajectoryRecords) != 1 || trajectoryRecords[0].DecisionID != rec.DecisionID {
		t.Fatalf("trajectory decisions = %+v", trajectoryRecords)
	}
	otherOwner, err := s.ListTextureDecisionsByDocument(ctx, "user-2", "doc-decision", 10)
	if err != nil {
		t.Fatalf("ListTextureDecisionsByDocument other owner: %v", err)
	}
	if len(otherOwner) != 0 {
		t.Fatalf("wrong owner saw decisions: %+v", otherOwner)
	}
}

func TestTextureUpdateDocument(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	doc := types.Document{
		DocID:   "doc-1",
		OwnerID: "user-1",
		Title:   "Original Title",
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	doc.Title = "Updated Title"
	doc.CurrentRevisionID = "rev-1"
	if err := s.UpdateDocument(ctx, doc); err != nil {
		t.Fatalf("UpdateDocument: %v", err)
	}

	got, err := s.GetDocument(ctx, "doc-1", "user-1")
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if got.Title != "Updated Title" {
		t.Errorf("Title = %q, want %q", got.Title, "Updated Title")
	}
	if got.CurrentRevisionID != "rev-1" {
		t.Errorf("CurrentRevisionID = %q, want %q", got.CurrentRevisionID, "rev-1")
	}
}

func TestTextureDocumentAliasRoundTrip(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	doc := types.Document{
		DocID:     "doc-alias",
		OwnerID:   "user-1",
		Title:     "Aliased Doc",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	if err := s.UpsertDocumentAlias(ctx, "user-1", "notes/aliased.md", "doc-alias", now); err != nil {
		t.Fatalf("UpsertDocumentAlias: %v", err)
	}

	docID, err := s.GetDocumentAlias(ctx, "user-1", "notes/aliased.md")
	if err != nil {
		t.Fatalf("GetDocumentAlias: %v", err)
	}
	if docID != "doc-alias" {
		t.Fatalf("doc alias resolved to %q, want %q", docID, "doc-alias")
	}
}

func TestTextureDocumentAliasSourcePathPrefersCanonicalShortcut(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	doc := types.Document{
		DocID:     "doc-canonical-alias",
		OwnerID:   "user-1",
		Title:     "Plain Proposal.texture",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	if err := s.UpsertDocumentAlias(ctx, "user-1", "notes/plain-proposal.txt", doc.DocID, now.Add(2*time.Second)); err != nil {
		t.Fatalf("UpsertDocumentAlias original: %v", err)
	}
	if err := s.UpsertDocumentAlias(ctx, "user-1", "plain-proposal.texture", doc.DocID, now.Add(3*time.Second)); err != nil {
		t.Fatalf("UpsertDocumentAlias legacy shortcut: %v", err)
	}
	if err := s.UpsertDocumentAlias(ctx, "user-1", "plain-proposal.texture", doc.DocID, now.Add(time.Second)); err != nil {
		t.Fatalf("UpsertDocumentAlias canonical shortcut: %v", err)
	}

	sourcePath, err := s.GetDocumentAliasSourcePath(ctx, "user-1", doc.DocID)
	if err != nil {
		t.Fatalf("GetDocumentAliasSourcePath: %v", err)
	}
	if sourcePath != "plain-proposal.texture" {
		t.Fatalf("source path = %q, want canonical texture shortcut", sourcePath)
	}
	if docID, err := s.GetDocumentAlias(ctx, "user-1", "notes/plain-proposal.txt"); err != nil || docID != doc.DocID {
		t.Fatalf("original alias docID = %q, err = %v, want %q", docID, err, doc.DocID)
	}
}

func TestTextureDeleteDocument(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	doc := types.Document{
		DocID:   "doc-1",
		OwnerID: "user-1",
		Title:   "To Delete",
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	if err := s.DeleteDocument(ctx, "doc-1", "user-1"); err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}

	_, err := s.GetDocument(ctx, "doc-1", "user-1")
	if err != ErrNotFound {
		t.Errorf("GetDocument after delete: err=%v, want ErrNotFound", err)
	}
}

// ----- Revision CRUD -----

func TestTextureCreateRevision(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	// Create a document first.
	doc := types.Document{
		DocID:   "doc-1",
		OwnerID: "user-1",
		Title:   "Test Doc",
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	metadata, _ := json.Marshal(map[string]any{"tags": []string{"draft"}})

	rev := types.Revision{
		RevisionID:  "rev-1",
		DocID:       "doc-1",
		OwnerID:     "user-1",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "alice",
		Content:     "Hello, world!",
		Metadata:    metadata,
		CreatedAt:   time.Now().UTC().Truncate(time.Millisecond),
	}
	if err := s.CreateRevision(ctx, rev); err != nil {
		t.Fatalf("CreateRevision: %v", err)
	}

	got, err := s.GetRevision(ctx, "rev-1", "user-1")
	if err != nil {
		t.Fatalf("GetRevision: %v", err)
	}
	if got.RevisionID != "rev-1" {
		t.Errorf("RevisionID = %q, want %q", got.RevisionID, "rev-1")
	}
	if got.AuthorKind != types.AuthorUser {
		t.Errorf("AuthorKind = %q, want %q", got.AuthorKind, types.AuthorUser)
	}
	if got.Content != "Hello, world!" {
		t.Errorf("Content = %q, want %q", got.Content, "Hello, world!")
	}
	if got.AuthorLabel != "alice" {
		t.Errorf("AuthorLabel = %q, want %q", got.AuthorLabel, "alice")
	}
	if got.VersionNumber != 0 {
		t.Errorf("VersionNumber = %d, want 0", got.VersionNumber)
	}
	if len(got.BodyDoc) == 0 {
		t.Fatalf("BodyDoc not persisted for plain text revision")
	}
	if len(got.SourceEntities) != 0 {
		t.Fatalf("SourceEntities = %s, want empty legacy-compatible response", got.SourceEntities)
	}
}

func TestTextureCreateRevisionStoresStructuredBodyAndSourceEntities(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	doc := types.Document{DocID: "doc-structured", OwnerID: "user-1", Title: "Structured"}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	bodyDoc, sourceEntities := structuredRevisionFixture(t)
	rev := types.Revision{
		RevisionID:     "rev-structured",
		DocID:          doc.DocID,
		OwnerID:        doc.OwnerID,
		AuthorKind:     types.AuthorUser,
		AuthorLabel:    "alice",
		BodyDoc:        bodyDoc,
		SourceEntities: sourceEntities,
		CreatedAt:      time.Now().UTC().Truncate(time.Millisecond),
	}
	if err := s.CreateRevision(ctx, rev); err != nil {
		t.Fatalf("CreateRevision structured: %v", err)
	}

	got, err := s.GetRevision(ctx, rev.RevisionID, doc.OwnerID)
	if err != nil {
		t.Fatalf("GetRevision: %v", err)
	}
	if got.Content != "Grounded[1]." {
		t.Fatalf("Content projection = %q, want derived numbered projection", got.Content)
	}
	if len(got.BodyDoc) == 0 {
		t.Fatalf("BodyDoc not persisted")
	}
	if string(got.SourceEntities) == "" || string(got.SourceEntities) == "[]" {
		t.Fatalf("SourceEntities not persisted: %s", got.SourceEntities)
	}
	if !strings.HasPrefix(got.RevisionHash, types.StructuredRevisionHashScheme+":") {
		t.Fatalf("RevisionHash = %q, want %s prefix", got.RevisionHash, types.StructuredRevisionHashScheme)
	}
	wantHash := types.ComputeStructuredRevisionHash("", got.Content, got.BodyDoc, got.SourceEntities, []byte("{}"))
	if got.RevisionHash != wantHash {
		t.Fatalf("RevisionHash = %q, want structured hash %q", got.RevisionHash, wantHash)
	}
}

func TestTextureCreateRevisionRejectsLegacySourceSyntaxes(t *testing.T) {
	cases := []string{
		"raw {{source:abc}} token",
		"[Story](source:abc)",
		"[source:abc]",
		"Source: https://example.com",
		"Unresolved citation [1]",
	}
	for _, content := range cases {
		t.Run(content, func(t *testing.T) {
			s := textureTestStore(t)
			ctx := context.Background()
			doc := types.Document{DocID: "doc-legacy", OwnerID: "user-1", Title: "Legacy"}
			if err := s.CreateDocument(ctx, doc); err != nil {
				t.Fatalf("CreateDocument: %v", err)
			}
			err := s.CreateRevision(ctx, types.Revision{
				RevisionID:  "rev-legacy",
				DocID:       doc.DocID,
				OwnerID:     doc.OwnerID,
				AuthorKind:  types.AuthorUser,
				AuthorLabel: "alice",
				Content:     content,
				CreatedAt:   time.Now().UTC().Truncate(time.Millisecond),
			})
			if !errors.Is(err, ErrInvalidTextureRevision) {
				t.Fatalf("CreateRevision error = %v, want ErrInvalidTextureRevision", err)
			}
		})
	}
}

func TestTextureCreateRevisionRejectsConflictingStructuredProjection(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()
	doc := types.Document{DocID: "doc-conflict", OwnerID: "user-1", Title: "Conflict"}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	bodyDoc, sourceEntities := structuredRevisionFixture(t)
	err := s.CreateRevision(ctx, types.Revision{
		RevisionID:     "rev-conflict",
		DocID:          doc.DocID,
		OwnerID:        doc.OwnerID,
		AuthorKind:     types.AuthorUser,
		AuthorLabel:    "alice",
		Content:        "caller supplied conflicting projection",
		BodyDoc:        bodyDoc,
		SourceEntities: sourceEntities,
		CreatedAt:      time.Now().UTC().Truncate(time.Millisecond),
	})
	if !errors.Is(err, ErrInvalidTextureRevision) {
		t.Fatalf("CreateRevision error = %v, want ErrInvalidTextureRevision", err)
	}
}

func TestTextureCreateRevisionRejectsLegacySourceSidecars(t *testing.T) {
	tests := []struct {
		name     string
		revPatch func(*types.Revision)
	}{
		{
			name: "citations",
			revPatch: func(rev *types.Revision) {
				citations, _ := json.Marshal([]types.Citation{{
					ID:    "c1",
					Type:  "url",
					Value: "https://example.com",
					Label: "Example",
				}})
				rev.Citations = citations
			},
		},
		{
			name: "metadata source_entities",
			revPatch: func(rev *types.Revision) {
				metadata, _ := json.Marshal(map[string]any{
					"source_entities": []map[string]any{{
						"entity_id": "src-1",
						"kind":      "web",
					}},
				})
				rev.Metadata = metadata
			},
		},
		{
			name: "metadata media_source_refs",
			revPatch: func(rev *types.Revision) {
				metadata, _ := json.Marshal(map[string]any{
					"media_source_refs": []map[string]any{{
						"entity_id": "src-image",
						"kind":      "image",
					}},
				})
				rev.Metadata = metadata
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := textureTestStore(t)
			ctx := context.Background()
			doc := types.Document{DocID: "doc-sidecar", OwnerID: "user-1", Title: "Sidecar"}
			if err := s.CreateDocument(ctx, doc); err != nil {
				t.Fatalf("CreateDocument: %v", err)
			}
			rev := types.Revision{
				RevisionID:  "rev-sidecar",
				DocID:       doc.DocID,
				OwnerID:     doc.OwnerID,
				AuthorKind:  types.AuthorUser,
				AuthorLabel: "alice",
				Content:     "Plain text body",
				CreatedAt:   time.Now().UTC().Truncate(time.Millisecond),
			}
			tt.revPatch(&rev)
			err := s.CreateRevision(ctx, rev)
			if !errors.Is(err, ErrInvalidTextureRevision) {
				t.Fatalf("CreateRevision error = %v, want ErrInvalidTextureRevision", err)
			}
		})
	}
}

func TestTextureCreateRevisionRejectsLegacySourceMetadataEvenWithStructuredSources(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()
	doc := types.Document{DocID: "doc-structured-sidecar", OwnerID: "user-1", Title: "Structured sidecar"}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	bodyDoc, sourceEntities := structuredRevisionFixture(t)
	metadata, _ := json.Marshal(map[string]any{
		"source_entities": []map[string]any{{
			"entity_id": "src-web",
			"kind":      "web",
		}},
		"source_ref_normalization": map[string]any{
			"legacy_count": 1,
		},
	})
	err := s.CreateRevision(ctx, types.Revision{
		RevisionID:     "rev-structured-sidecar",
		DocID:          doc.DocID,
		OwnerID:        doc.OwnerID,
		AuthorKind:     types.AuthorUser,
		AuthorLabel:    "alice",
		BodyDoc:        bodyDoc,
		SourceEntities: sourceEntities,
		Metadata:       metadata,
		CreatedAt:      time.Now().UTC().Truncate(time.Millisecond),
	})
	if !errors.Is(err, ErrInvalidTextureRevision) {
		t.Fatalf("CreateRevision error = %v, want ErrInvalidTextureRevision", err)
	}
}

func TestTextureBootstrapMigratesStructuredRevisionColumns(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "old-texture.db")
	db, workspacePath, connector, err := openTextureWorkspaceDB(dbPath)
	if err != nil {
		t.Fatalf("open old texture workspace: %v", err)
	}
	s := &Store{path: dbPath, textureDB: db, texturePath: workspacePath, doltConnector: connector}
	t.Cleanup(func() { _ = s.Close() })

	_, err = s.textureHandle().Exec(`
CREATE TABLE texture_documents (
	doc_id              VARCHAR(255) PRIMARY KEY,
	owner_id            VARCHAR(255) NOT NULL,
	title               VARCHAR(1024) NOT NULL DEFAULT '',
	current_revision_id VARCHAR(255) NOT NULL DEFAULT '',
	created_at          DATETIME NOT NULL,
	updated_at          DATETIME NOT NULL
);
CREATE TABLE texture_revisions (
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
	revision_hash       VARCHAR(255) NOT NULL DEFAULT '',
	parent_revision_id  VARCHAR(255) NOT NULL DEFAULT '',
	created_at          DATETIME NOT NULL
);`)
	if err != nil {
		t.Fatalf("create old texture schema: %v", err)
	}
	if err := s.bootstrapTexture(); err != nil {
		t.Fatalf("bootstrapTexture: %v", err)
	}
	assertTextureColumnExists(t, s, "body_doc_json")
	assertTextureColumnExists(t, s, "source_entities_json")
}

func structuredRevisionFixture(t *testing.T) (json.RawMessage, json.RawMessage) {
	t.Helper()
	doc := texturedoc.StructuredTextureDoc{
		Schema: texturedoc.SchemaV1,
		Doc: texturedoc.Node{
			Type:  "doc",
			Attrs: map[string]any{"id": "doc-node"},
			Content: []texturedoc.Node{{
				Type:  "paragraph",
				Attrs: map[string]any{"id": "p-1"},
				Content: []texturedoc.Node{
					{Type: "text", Text: "Grounded"},
					{
						Type: "source_ref",
						Attrs: map[string]any{
							"id":               "ref-1",
							"source_entity_id": "src-web",
							"display_mode":     "numbered_ref",
						},
					},
					{Type: "text", Text: "."},
				},
			}},
		},
	}
	entities := []texturedoc.SourceEntity{{
		SourceEntityID: "src-web",
		Target: texturedoc.SourceTarget{
			Kind: "web_url",
			URI:  "https://example.com/story",
		},
		Selectors: []texturedoc.SourceSelector{{
			Kind: sourcecontract.SelectorKindTextQuote,
			Data: map[string]any{"exact": "Grounded"},
		}},
		Display: texturedoc.SourceDisplay{
			Mode:  "numbered_ref",
			Title: "Example story",
		},
		Evidence: texturedoc.SourceEvidence{
			State:       sourcecontract.EvidenceStateConfirms,
			OpenSurface: sourcecontract.OpenSurfaceSource,
		},
		Provenance: texturedoc.SourceEntityProvenance{
			CreatedBy:    "runtime",
			SourceSystem: "test",
		},
	}}
	bodyDocJSON, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal body doc: %v", err)
	}
	sourceEntitiesJSON, err := json.Marshal(entities)
	if err != nil {
		t.Fatalf("marshal source entities: %v", err)
	}
	return bodyDocJSON, sourceEntitiesJSON
}

func assertTextureColumnExists(t *testing.T, s *Store, name string) {
	t.Helper()
	var count int
	if err := s.textureHandle().QueryRow(`
SELECT COUNT(*)
FROM information_schema.columns
WHERE table_schema = DATABASE()
  AND table_name = 'texture_revisions'
  AND column_name = ?`, name).Scan(&count); err != nil {
		t.Fatalf("query column %s: %v", name, err)
	}
	if count != 1 {
		t.Fatalf("column %s count = %d, want 1", name, count)
	}
}

func TestTextureRevisionProvenanceRoundTrip(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	doc := types.Document{DocID: "doc-prov", OwnerID: "user-1", Title: "Prov Doc"}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	prov := types.Provenance{
		SchemaVersion:  types.ProvenanceSchemaVersion,
		AuthoringModel: types.ProvenanceModel{Provider: "fireworks", Model: "test-model"},
		AuthoredAt:     time.Date(2026, 6, 18, 14, 0, 0, 0, time.UTC),
		QueriesExecuted: []types.ProvenanceQuery{
			{Tool: "web_search", Query: "grounding query", ResultCount: 2},
		},
		Sources: []types.SourceEntity{
			{EntityID: "src_aaaa", Kind: "content_item", Target: types.SourceEntityTarget{TargetKind: "content_item", ContentID: "ci-1"}},
		},
	}
	canonical, err := prov.CanonicalJSON()
	if err != nil {
		t.Fatalf("CanonicalJSON: %v", err)
	}

	rev := types.Revision{
		RevisionID:  "rev-prov",
		DocID:       "doc-prov",
		OwnerID:     "user-1",
		AuthorKind:  types.AuthorAppAgent,
		AuthorLabel: "appagent",
		Content:     "Grounded body.",
		BodyDoc:     testTextureBodyDoc(t, "doc-prov", "rev-prov", "Grounded body."),
		Provenance:  json.RawMessage(canonical),
		CreatedAt:   time.Now().UTC().Truncate(time.Millisecond),
	}
	if err := s.CreateRevision(ctx, rev); err != nil {
		t.Fatalf("CreateRevision: %v", err)
	}

	got, err := s.GetRevision(ctx, "rev-prov", "user-1")
	if err != nil {
		t.Fatalf("GetRevision: %v", err)
	}
	if len(got.Provenance) == 0 {
		t.Fatalf("Provenance not persisted")
	}
	var roundtrip types.Provenance
	if err := json.Unmarshal(got.Provenance, &roundtrip); err != nil {
		t.Fatalf("unmarshal provenance: %v", err)
	}
	if roundtrip.SchemaVersion != types.ProvenanceSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", roundtrip.SchemaVersion, types.ProvenanceSchemaVersion)
	}
	if roundtrip.AuthoringModel.Model != "test-model" {
		t.Errorf("AuthoringModel.Model = %q, want %q", roundtrip.AuthoringModel.Model, "test-model")
	}
	if len(roundtrip.Sources) != 1 || roundtrip.Sources[0].EntityID != "src_aaaa" {
		t.Errorf("Sources round-trip mismatch: %+v", roundtrip.Sources)
	}
	again, err := roundtrip.CanonicalJSON()
	if err != nil {
		t.Fatalf("CanonicalJSON after round-trip: %v", err)
	}
	if string(again) != string(canonical) {
		t.Errorf("canonical bytes not stable across persistence:\n%s\n%s", again, canonical)
	}
}

func TestTextureRevisionHashChain(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	doc := types.Document{DocID: "doc-hash", OwnerID: "user-1", Title: "Hash Doc"}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	rev0 := types.Revision{
		RevisionID: "rev-0", DocID: "doc-hash", OwnerID: "user-1",
		AuthorKind: types.AuthorUser, AuthorLabel: "alice",
		Content: "v0 body", CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
	}
	if err := s.CreateRevision(ctx, rev0); err != nil {
		t.Fatalf("CreateRevision v0: %v", err)
	}
	got0, err := s.GetRevision(ctx, "rev-0", "user-1")
	if err != nil {
		t.Fatalf("GetRevision v0: %v", err)
	}
	if got0.RevisionHash == "" {
		t.Fatalf("genesis revision hash empty")
	}
	wantGenesis := types.ComputeStructuredRevisionHash("", got0.Content, got0.BodyDoc, got0.SourceEntities, []byte("{}"))
	if got0.RevisionHash != wantGenesis {
		t.Errorf("genesis hash = %q, want %q", got0.RevisionHash, wantGenesis)
	}

	rev1 := types.Revision{
		RevisionID: "rev-1", DocID: "doc-hash", OwnerID: "user-1",
		AuthorKind: types.AuthorAppAgent, AuthorLabel: "appagent",
		Content: "v1 body", BodyDoc: testTextureBodyDoc(t, "doc-hash", "rev-1", "v1 body"), ParentRevisionID: "rev-0",
		CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
	}
	if err := s.CreateRevision(ctx, rev1); err != nil {
		t.Fatalf("CreateRevision v1: %v", err)
	}
	got1, err := s.GetRevision(ctx, "rev-1", "user-1")
	if err != nil {
		t.Fatalf("GetRevision v1: %v", err)
	}
	wantV1 := types.ComputeStructuredRevisionHash(got0.RevisionHash, got1.Content, got1.BodyDoc, got1.SourceEntities, []byte("{}"))
	if got1.RevisionHash != wantV1 {
		t.Errorf("v1 hash = %q, want chained %q", got1.RevisionHash, wantV1)
	}
	if got1.RevisionHash == got0.RevisionHash {
		t.Errorf("v1 hash equals v0 hash; chain not distinct")
	}
}

func TestTextureRevisionWithoutProvenanceIsEmpty(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	doc := types.Document{DocID: "doc-np", OwnerID: "user-1", Title: "No Prov"}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	rev := types.Revision{
		RevisionID:  "rev-np",
		DocID:       "doc-np",
		OwnerID:     "user-1",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "alice",
		Content:     "plain",
		CreatedAt:   time.Now().UTC().Truncate(time.Millisecond),
	}
	if err := s.CreateRevision(ctx, rev); err != nil {
		t.Fatalf("CreateRevision: %v", err)
	}
	got, err := s.GetRevision(ctx, "rev-np", "user-1")
	if err != nil {
		t.Fatalf("GetRevision: %v", err)
	}
	if len(got.Provenance) != 0 {
		t.Errorf("expected empty provenance, got %q", string(got.Provenance))
	}
}

func TestTextureCreateRevisionRejectsStaleHead(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	doc := types.Document{
		DocID:     "doc-1",
		OwnerID:   "user-1",
		Title:     "Test Doc",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	base := types.Revision{
		RevisionID:  "rev-1",
		DocID:       "doc-1",
		OwnerID:     "user-1",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "alice",
		Content:     "Base",
		CreatedAt:   now,
	}
	if err := s.CreateRevision(ctx, base); err != nil {
		t.Fatalf("CreateRevision base: %v", err)
	}

	head := types.Revision{
		RevisionID:       "rev-2",
		DocID:            "doc-1",
		OwnerID:          "user-1",
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      "alice",
		Content:          "Head",
		ParentRevisionID: "rev-1",
		CreatedAt:        now.Add(time.Second),
	}
	if err := s.CreateRevision(ctx, head); err != nil {
		t.Fatalf("CreateRevision head: %v", err)
	}

	stale := types.Revision{
		RevisionID:       "rev-3",
		DocID:            "doc-1",
		OwnerID:          "user-1",
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      "alice",
		Content:          "Stale branch",
		ParentRevisionID: "rev-1",
		CreatedAt:        now.Add(2 * time.Second),
	}
	err := s.CreateRevision(ctx, stale)
	if !errors.Is(err, ErrStaleDocumentHead) {
		t.Fatalf("CreateRevision stale err = %v, want ErrStaleDocumentHead", err)
	}
}

func TestTextureRevisionMetadataConcurrentMergePatchesPreserveKeys(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	doc := types.Document{
		DocID:   "doc-concurrent-metadata",
		OwnerID: "user-1",
		Title:   "Concurrent Metadata",
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	initialMeta, _ := json.Marshal(map[string]any{"source": "wire"})
	rev := types.Revision{
		RevisionID:  "rev-concurrent-metadata",
		DocID:       doc.DocID,
		OwnerID:     doc.OwnerID,
		AuthorKind:  types.AuthorAppAgent,
		AuthorLabel: "wire",
		Content:     "Article",
		BodyDoc:     testTextureBodyDoc(t, doc.DocID, "rev-concurrent-metadata", "Article"),
		Metadata:    initialMeta,
		CreatedAt:   time.Now().UTC(),
	}
	if err := s.CreateRevision(ctx, rev); err != nil {
		t.Fatalf("CreateRevision: %v", err)
	}

	const patches = 8
	var wg sync.WaitGroup
	errs := make(chan error, patches)
	for i := 0; i < patches; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- s.PatchRevisionMetadata(ctx, rev.OwnerID, rev.RevisionID, map[string]any{
				fmt.Sprintf("patch_%02d", i): fmt.Sprintf("value-%02d", i),
			})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent revision metadata patch: %v", err)
		}
	}

	got, err := s.GetRevision(ctx, rev.RevisionID, rev.OwnerID)
	if err != nil {
		t.Fatalf("GetRevision: %v", err)
	}
	meta := map[string]any{}
	if err := json.Unmarshal(got.Metadata, &meta); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if meta["source"] != "wire" {
		t.Fatalf("existing metadata lost: %+v", meta)
	}
	for i := 0; i < patches; i++ {
		key := fmt.Sprintf("patch_%02d", i)
		if meta[key] != fmt.Sprintf("value-%02d", i) {
			t.Fatalf("metadata %s = %q, want value-%02d; metadata=%+v", key, meta[key], i, meta)
		}
	}
}

func TestTextureRevisionOwnerScope(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	doc := types.Document{
		DocID:   "doc-1",
		OwnerID: "user-1",
		Title:   "Owned by user-1",
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	rev := types.Revision{
		RevisionID:  "rev-1",
		DocID:       "doc-1",
		OwnerID:     "user-1",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "alice",
		Content:     "Content",
		CreatedAt:   time.Now().UTC(),
	}
	if err := s.CreateRevision(ctx, rev); err != nil {
		t.Fatalf("CreateRevision: %v", err)
	}

	// user-2 should not see user-1's revision.
	_, err := s.GetRevision(ctx, "rev-1", "user-2")
	if err != ErrNotFound {
		t.Errorf("GetRevision as wrong owner: err=%v, want ErrNotFound", err)
	}
}

func TestCreateAndGetEvidence(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	metadata, _ := json.Marshal(map[string]any{"mime_type": "text/html"})
	rec := types.EvidenceRecord{
		EvidenceID: "ev-1",
		OwnerID:    "user-1",
		AgentID:    "researcher-a",
		Kind:       "web_page",
		SourceURI:  "https://example.com",
		Title:      "Example",
		Content:    "<html>example</html>",
		Metadata:   metadata,
		CreatedAt:  time.Now().UTC().Truncate(time.Millisecond),
	}
	if err := s.CreateEvidence(ctx, rec); err != nil {
		t.Fatalf("CreateEvidence: %v", err)
	}

	got, err := s.GetEvidence(ctx, "ev-1", "user-1")
	if err != nil {
		t.Fatalf("GetEvidence: %v", err)
	}
	if got.AgentID != "researcher-a" {
		t.Errorf("AgentID = %q, want %q", got.AgentID, "researcher-a")
	}
	if got.SourceURI != "https://example.com" {
		t.Errorf("SourceURI = %q, want %q", got.SourceURI, "https://example.com")
	}
	if got.Content != "<html>example</html>" {
		t.Errorf("Content = %q, want %q", got.Content, "<html>example</html>")
	}
}

func TestListEvidenceByAgentOwnerScoped(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	for _, rec := range []types.EvidenceRecord{
		{EvidenceID: "ev-1", OwnerID: "user-1", AgentID: "researcher-a", Kind: "web_page", Content: "A", CreatedAt: time.Now().UTC()},
		{EvidenceID: "ev-2", OwnerID: "user-1", AgentID: "researcher-a", Kind: "web_page", Content: "B", CreatedAt: time.Now().UTC().Add(1 * time.Second)},
		{EvidenceID: "ev-3", OwnerID: "user-1", AgentID: "researcher-b", Kind: "web_page", Content: "C", CreatedAt: time.Now().UTC().Add(2 * time.Second)},
		{EvidenceID: "ev-4", OwnerID: "user-2", AgentID: "researcher-a", Kind: "web_page", Content: "D", CreatedAt: time.Now().UTC().Add(3 * time.Second)},
	} {
		if err := s.CreateEvidence(ctx, rec); err != nil {
			t.Fatalf("CreateEvidence(%s): %v", rec.EvidenceID, err)
		}
	}

	got, err := s.ListEvidenceByAgent(ctx, "user-1", "researcher-a", 10)
	if err != nil {
		t.Fatalf("ListEvidenceByAgent: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].EvidenceID != "ev-2" || got[1].EvidenceID != "ev-1" {
		t.Fatalf("unexpected evidence order: %+v", got)
	}
}

func TestTextureListRevisionsByDoc(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	doc := types.Document{
		DocID:   "doc-1",
		OwnerID: "user-1",
		Title:   "Test Doc",
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	// Create 3 revisions with different authors.
	for i := 0; i < 3; i++ {
		authorKind := types.AuthorUser
		authorLabel := "alice"
		if i == 1 {
			authorKind = types.AuthorAppAgent
			authorLabel = "appagent"
		}
		rev := types.Revision{
			RevisionID:       "rev-" + string(rune('1'+i)),
			DocID:            "doc-1",
			OwnerID:          "user-1",
			AuthorKind:       authorKind,
			AuthorLabel:      authorLabel,
			Content:          "Content v" + string(rune('1'+i)),
			ParentRevisionID: "",
			CreatedAt:        time.Now().UTC().Add(time.Duration(i) * time.Second),
		}
		if i > 0 {
			rev.ParentRevisionID = "rev-" + string(rune('0'+i))
		}
		rev = testTextureRevisionWithBodyDoc(t, rev)
		if err := s.CreateRevision(ctx, rev); err != nil {
			t.Fatalf("CreateRevision %d: %v", i, err)
		}
	}

	revs, err := s.ListRevisionsByDoc(ctx, "doc-1", "user-1", 10)
	if err != nil {
		t.Fatalf("ListRevisionsByDoc: %v", err)
	}
	if len(revs) != 3 {
		t.Fatalf("len(revs) = %d, want 3", len(revs))
	}

	// Should be ordered by created_at descending (newest first).
	if revs[0].RevisionID != "rev-3" {
		t.Errorf("first rev = %q, want %q", revs[0].RevisionID, "rev-3")
	}
	for i, rev := range revs {
		wantVersion := 2 - i
		if rev.VersionNumber != wantVersion {
			t.Errorf("revs[%d].VersionNumber = %d, want %d", i, rev.VersionNumber, wantVersion)
		}
	}

	// Check attribution: user, appagent, user.
	if revs[2].AuthorKind != types.AuthorUser || revs[1].AuthorKind != types.AuthorAppAgent || revs[0].AuthorKind != types.AuthorUser {
		t.Errorf("author kinds = %v, %v, %v; want user, appagent, user", revs[2].AuthorKind, revs[1].AuthorKind, revs[0].AuthorKind)
	}
}

func TestTextureRevisionVersionNumbersAdvancePastFifty(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	doc := types.Document{
		DocID:   "doc-version-many",
		OwnerID: "user-1",
		Title:   "Many versions",
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	parentID := ""
	for i := 0; i < 55; i++ {
		revID := fmt.Sprintf("rev-many-%02d", i)
		rev := types.Revision{
			RevisionID:       revID,
			DocID:            doc.DocID,
			OwnerID:          doc.OwnerID,
			AuthorKind:       types.AuthorUser,
			AuthorLabel:      "alice",
			Content:          fmt.Sprintf("Content v%d", i),
			ParentRevisionID: parentID,
			CreatedAt:        time.Now().UTC().Add(time.Duration(i) * time.Second),
		}
		if err := s.CreateRevision(ctx, rev); err != nil {
			t.Fatalf("CreateRevision %d: %v", i, err)
		}
		parentID = revID
	}

	count, err := s.CountRevisionsByDoc(ctx, doc.DocID, doc.OwnerID)
	if err != nil {
		t.Fatalf("CountRevisionsByDoc: %v", err)
	}
	if count != 55 {
		t.Fatalf("CountRevisionsByDoc = %d, want 55", count)
	}
	currentVersion, err := s.CurrentVersionNumberByDoc(ctx, doc.DocID, doc.OwnerID)
	if err != nil {
		t.Fatalf("CurrentVersionNumberByDoc: %v", err)
	}
	if currentVersion != 54 {
		t.Fatalf("CurrentVersionNumberByDoc = %d, want 54", currentVersion)
	}

	revs, err := s.ListRevisionsByDoc(ctx, doc.DocID, doc.OwnerID, 10000)
	if err != nil {
		t.Fatalf("ListRevisionsByDoc: %v", err)
	}
	if len(revs) != 55 {
		t.Fatalf("len(revs) = %d, want 55", len(revs))
	}
	if revs[0].VersionNumber != 54 || revs[0].RevisionID != "rev-many-54" {
		t.Fatalf("latest rev = %s v%d, want rev-many-54 v54", revs[0].RevisionID, revs[0].VersionNumber)
	}
	if revs[54].VersionNumber != 0 || revs[54].RevisionID != "rev-many-00" {
		t.Fatalf("oldest rev = %s v%d, want rev-many-00 v0", revs[54].RevisionID, revs[54].VersionNumber)
	}
}

func TestTextureBackfillsLegacyRevisionVersionNumbers(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	doc := types.Document{
		DocID:   "doc-legacy",
		OwnerID: "user-1",
		Title:   "Legacy versions",
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	parentID := ""
	for i := 0; i < 4; i++ {
		revID := fmt.Sprintf("rev-legacy-%d", i)
		rev := types.Revision{
			RevisionID:       revID,
			DocID:            doc.DocID,
			OwnerID:          doc.OwnerID,
			AuthorKind:       types.AuthorUser,
			AuthorLabel:      "alice",
			Content:          fmt.Sprintf("Content v%d", i),
			ParentRevisionID: parentID,
			CreatedAt:        time.Date(2026, 6, 5, 12, i, 0, 0, time.UTC),
		}
		if err := s.CreateRevision(ctx, rev); err != nil {
			t.Fatalf("CreateRevision %d: %v", i, err)
		}
		parentID = revID
	}

	if _, err := s.textureHandle().ExecContext(ctx, `UPDATE texture_revisions SET version_number = 0 WHERE doc_id = ? AND owner_id = ?`, doc.DocID, doc.OwnerID); err != nil {
		t.Fatalf("zero legacy version numbers: %v", err)
	}
	if err := s.EnsureTextureSchema(); err != nil {
		t.Fatalf("EnsureTextureSchema: %v", err)
	}

	revs, err := s.ListRevisionsByDoc(ctx, doc.DocID, doc.OwnerID, 10)
	if err != nil {
		t.Fatalf("ListRevisionsByDoc: %v", err)
	}
	if len(revs) != 4 {
		t.Fatalf("len(revs) = %d, want 4", len(revs))
	}
	for i, rev := range revs {
		wantVersion := 3 - i
		if rev.VersionNumber != wantVersion {
			t.Fatalf("revs[%d].VersionNumber = %d, want %d", i, rev.VersionNumber, wantVersion)
		}
	}
}

func TestTextureListRevisionsByDocOwnerScope(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	doc := types.Document{
		DocID:   "doc-1",
		OwnerID: "user-1",
		Title:   "Owned by user-1",
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	rev := types.Revision{
		RevisionID:  "rev-1",
		DocID:       "doc-1",
		OwnerID:     "user-1",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "alice",
		Content:     "Content",
		CreatedAt:   time.Now().UTC(),
	}
	if err := s.CreateRevision(ctx, rev); err != nil {
		t.Fatalf("CreateRevision: %v", err)
	}

	// user-2 should not see user-1's revisions.
	revs, err := s.ListRevisionsByDoc(ctx, "doc-1", "user-2", 10)
	if err != nil {
		t.Fatalf("ListRevisionsByDoc: %v", err)
	}
	if len(revs) != 0 {
		t.Errorf("len(revs) = %d, want 0 for wrong owner", len(revs))
	}
}

// ----- History -----

func TestTextureGetHistory(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	doc := types.Document{
		DocID:   "doc-1",
		OwnerID: "user-1",
		Title:   "Test Doc",
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	// Create revisions with parent chain.
	now := time.Now().UTC().Truncate(time.Millisecond)
	revs := []types.Revision{
		{
			RevisionID:  "rev-1",
			DocID:       "doc-1",
			OwnerID:     "user-1",
			AuthorKind:  types.AuthorUser,
			AuthorLabel: "alice",
			Content:     "First draft",
			CreatedAt:   now,
		},
		{
			RevisionID:       "rev-2",
			DocID:            "doc-1",
			OwnerID:          "user-1",
			AuthorKind:       types.AuthorAppAgent,
			AuthorLabel:      "appagent",
			Content:          "AI-improved draft",
			ParentRevisionID: "rev-1",
			CreatedAt:        now.Add(time.Second),
		},
		{
			RevisionID:       "rev-3",
			DocID:            "doc-1",
			OwnerID:          "user-1",
			AuthorKind:       types.AuthorUser,
			AuthorLabel:      "alice",
			Content:          "User edited",
			ParentRevisionID: "rev-2",
			CreatedAt:        now.Add(2 * time.Second),
		},
	}
	for _, r := range revs {
		r = testTextureRevisionWithBodyDoc(t, r)
		if err := s.CreateRevision(ctx, r); err != nil {
			t.Fatalf("CreateRevision: %v", err)
		}
	}

	history, err := s.GetHistory(ctx, "doc-1", "user-1", 10)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("len(history) = %d, want 3", len(history))
	}

	// Should be newest first.
	if history[0].RevisionID != "rev-3" {
		t.Errorf("first entry = %q, want %q", history[0].RevisionID, "rev-3")
	}
	// Check attribution metadata is present.
	if history[0].AuthorKind != types.AuthorUser {
		t.Errorf("first entry AuthorKind = %q, want %q", history[0].AuthorKind, types.AuthorUser)
	}
	if history[1].AuthorKind != types.AuthorAppAgent {
		t.Errorf("second entry AuthorKind = %q, want %q", history[1].AuthorKind, types.AuthorAppAgent)
	}
	// Check parent revision chain.
	if history[0].ParentRevisionID != "rev-2" {
		t.Errorf("first entry ParentRevisionID = %q, want %q", history[0].ParentRevisionID, "rev-2")
	}
}

func TestTextureHistoryHasNativeDoltAuditCommits(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	doc := types.Document{
		DocID:   "doc-native-history",
		OwnerID: "user-native-history",
		Title:   "Native history",
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	parentID := ""
	const revisionCount = 25
	for i := 1; i <= revisionCount; i++ {
		rev := testTextureRevisionWithBodyDoc(t, types.Revision{
			RevisionID:       fmt.Sprintf("rev-native-history-%d", i),
			DocID:            doc.DocID,
			OwnerID:          doc.OwnerID,
			AuthorKind:       types.AuthorUser,
			AuthorLabel:      "alice",
			Content:          fmt.Sprintf("revision %d", i),
			ParentRevisionID: parentID,
			CreatedAt:        time.Date(2026, 7, 10, 12, i, 0, 0, time.UTC),
		})
		if err := s.CreateRevision(ctx, rev); err != nil {
			t.Fatalf("CreateRevision %d: %v", i, err)
		}
		parentID = rev.RevisionID
	}

	var commits int
	if err := s.textureHandle().QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT commit_hash)
		FROM dolt_history_og_objects
		WHERE object_kind = ?
		  AND JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), '$.doc_id')) = ?`,
		string(ogKindTexRev), doc.DocID,
	).Scan(&commits); err != nil {
		t.Fatalf("query native Dolt history: %v", err)
	}
	if commits < revisionCount {
		t.Fatalf("native Dolt audit commits = %d, want at least %d revision commits", commits, revisionCount)
	}

	started := time.Now()
	history, err := s.GetHistory(ctx, doc.DocID, doc.OwnerID, 10)
	latency := time.Since(started)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(history) != 10 || history[0].RevisionID != "rev-native-history-25" {
		t.Fatalf("native history page = %#v, want latest 10 revisions", history)
	}
	t.Logf("native Dolt history latest-10 latency across %d revisions: %s", revisionCount, latency)
}

func TestValidateDoltCommitHashRejectsSQLSyntax(t *testing.T) {
	if err := validateDoltCommitHash("0123456789abcdefghijklmnopqrstuv"); err != nil {
		t.Fatalf("valid Dolt hash rejected: %v", err)
	}
	for _, hash := range []string{"", "HEAD~1", "abc' OR 1=1 --"} {
		if err := validateDoltCommitHash(hash); err == nil {
			t.Fatalf("unsafe Dolt hash %q accepted", hash)
		}
	}
}

// ----- Diff -----

func TestTextureGetDiff(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	doc := types.Document{
		DocID:   "doc-1",
		OwnerID: "user-1",
		Title:   "Test Doc",
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	now := time.Now().UTC()
	revs := []types.Revision{
		{
			RevisionID:  "rev-1",
			DocID:       "doc-1",
			OwnerID:     "user-1",
			AuthorKind:  types.AuthorUser,
			AuthorLabel: "alice",
			Content:     "line1\nline2\nline3\n",
			CreatedAt:   now,
		},
		{
			RevisionID:       "rev-2",
			DocID:            "doc-1",
			OwnerID:          "user-1",
			AuthorKind:       types.AuthorAppAgent,
			AuthorLabel:      "appagent",
			Content:          "line1\nline2-modified\nline3\nline4-added\n",
			ParentRevisionID: "rev-1",
			CreatedAt:        now.Add(time.Second),
		},
	}
	for _, r := range revs {
		r = testTextureRevisionWithBodyDoc(t, r)
		if err := s.CreateRevision(ctx, r); err != nil {
			t.Fatalf("CreateRevision: %v", err)
		}
	}

	diff, err := s.GetDiff(ctx, "rev-1", "rev-2", "user-1")
	if err != nil {
		t.Fatalf("GetDiff: %v", err)
	}
	if diff.FromRevisionID != "rev-1" {
		t.Errorf("FromRevisionID = %q, want %q", diff.FromRevisionID, "rev-1")
	}
	if diff.ToRevisionID != "rev-2" {
		t.Errorf("ToRevisionID = %q, want %q", diff.ToRevisionID, "rev-2")
	}
	// There should be some change detected.
	if len(diff.Sections) == 0 {
		t.Error("no diff sections detected")
	}
	if diff.AddedLines == 0 && diff.RemovedLines == 0 {
		t.Error("no lines added or removed")
	}
}

// ----- Blame -----

func TestTextureGetBlame(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	doc := types.Document{
		DocID:   "doc-1",
		OwnerID: "user-1",
		Title:   "Test Doc",
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	now := time.Now().UTC()
	revs := []types.Revision{
		{
			RevisionID:  "rev-1",
			DocID:       "doc-1",
			OwnerID:     "user-1",
			AuthorKind:  types.AuthorUser,
			AuthorLabel: "alice",
			Content:     "line1\nline2\nline3\n",
			CreatedAt:   now,
		},
		{
			RevisionID:       "rev-2",
			DocID:            "doc-1",
			OwnerID:          "user-1",
			AuthorKind:       types.AuthorAppAgent,
			AuthorLabel:      "appagent",
			Content:          "line1\nline2-modified\nline3\n",
			ParentRevisionID: "rev-1",
			CreatedAt:        now.Add(time.Second),
		},
	}
	for _, r := range revs {
		r = testTextureRevisionWithBodyDoc(t, r)
		if err := s.CreateRevision(ctx, r); err != nil {
			t.Fatalf("CreateRevision: %v", err)
		}
	}

	blame, err := s.GetBlame(ctx, "rev-2", "user-1")
	if err != nil {
		t.Fatalf("GetBlame: %v", err)
	}
	if blame.RevisionID != "rev-2" {
		t.Errorf("RevisionID = %q, want %q", blame.RevisionID, "rev-2")
	}
	if blame.DocID != "doc-1" {
		t.Errorf("DocID = %q, want %q", blame.DocID, "doc-1")
	}
	if len(blame.Sections) == 0 {
		t.Error("no blame sections")
	}

	// Verify that sections have different author kinds.
	hasUser := false
	hasAgent := false
	for _, sec := range blame.Sections {
		if sec.AuthorKind == types.AuthorUser {
			hasUser = true
		}
		if sec.AuthorKind == types.AuthorAppAgent {
			hasAgent = true
		}
	}
	if !hasUser || !hasAgent {
		t.Errorf("blame should contain both user and appagent sections; hasUser=%v, hasAgent=%v", hasUser, hasAgent)
	}
}

// ----- Citations and Metadata persistence -----

func TestTextureMetadataRoundTrip(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	doc := types.Document{
		DocID:   "doc-1",
		OwnerID: "user-1",
		Title:   "Test Doc",
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	metaJSON, _ := json.Marshal(map[string]any{
		"tags":    []string{"draft", "important"},
		"version": 2,
	})

	rev := types.Revision{
		RevisionID:  "rev-1",
		DocID:       "doc-1",
		OwnerID:     "user-1",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "alice",
		Content:     "Document with ordinary metadata",
		Metadata:    metaJSON,
		CreatedAt:   time.Now().UTC().Truncate(time.Millisecond),
	}
	if err := s.CreateRevision(ctx, rev); err != nil {
		t.Fatalf("CreateRevision: %v", err)
	}

	got, err := s.GetRevision(ctx, "rev-1", "user-1")
	if err != nil {
		t.Fatalf("GetRevision: %v", err)
	}

	// Verify metadata round-trip.
	var gotMeta map[string]any
	if err := json.Unmarshal(got.Metadata, &gotMeta); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if gotMeta["version"] != float64(2) {
		t.Errorf("metadata.version = %v, want 2", gotMeta["version"])
	}
}

// ----- Snapshot (open historical revision without mutating head) -----

func TestTextureSnapshotDoesNotMutateHead(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	doc := types.Document{
		DocID:   "doc-1",
		OwnerID: "user-1",
		Title:   "Test Doc",
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	now := time.Now().UTC()
	revs := []types.Revision{
		{
			RevisionID:  "rev-1",
			DocID:       "doc-1",
			OwnerID:     "user-1",
			AuthorKind:  types.AuthorUser,
			AuthorLabel: "alice",
			Content:     "Old content",
			CreatedAt:   now,
		},
		{
			RevisionID:       "rev-2",
			DocID:            "doc-1",
			OwnerID:          "user-1",
			AuthorKind:       types.AuthorUser,
			AuthorLabel:      "alice",
			Content:          "New content",
			ParentRevisionID: "rev-1",
			CreatedAt:        now.Add(time.Second),
		},
	}
	for _, r := range revs {
		r = testTextureRevisionWithBodyDoc(t, r)
		if err := s.CreateRevision(ctx, r); err != nil {
			t.Fatalf("CreateRevision: %v", err)
		}
	}

	// Open the old revision (snapshot).
	snapshot, err := s.GetRevision(ctx, "rev-1", "user-1")
	if err != nil {
		t.Fatalf("GetRevision (snapshot): %v", err)
	}
	if snapshot.Content != "Old content" {
		t.Errorf("snapshot content = %q, want %q", snapshot.Content, "Old content")
	}

	// Verify head is unchanged.
	got, err := s.GetDocument(ctx, "doc-1", "user-1")
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if got.CurrentRevisionID != "rev-2" {
		t.Errorf("CurrentRevisionID after snapshot = %q, want %q", got.CurrentRevisionID, "rev-2")
	}
}

// ----- Workspace setup -----

func TestTextureInitWorkspace(t *testing.T) {
	dir := t.TempDir()
	wsPath := filepath.Join(dir, "workspace.db")

	s, err := OpenTextureWorkspace(wsPath)
	if err != nil {
		t.Fatalf("OpenTextureWorkspace: %v", err)
	}
	defer func() { _ = s.Close() }()

	ctx := context.Background()

	// Verify the texture schema is applied by creating a document.
	doc := types.Document{
		DocID:   "doc-1",
		OwnerID: "user-1",
		Title:   "Workspace Test",
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument in workspace: %v", err)
	}

	got, err := s.GetDocument(ctx, "doc-1", "user-1")
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if got.DocID != "doc-1" {
		t.Errorf("DocID = %q, want %q", got.DocID, "doc-1")
	}

	// Verify the workspace directory exists.
	if _, err := os.Stat(s.TexturePath()); os.IsNotExist(err) {
		t.Errorf("workspace directory %q was not created", s.TexturePath())
	}
}

// ----- Diff owner scope -----

func TestTextureDiffOwnerScope(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	doc := types.Document{
		DocID:   "doc-1",
		OwnerID: "user-1",
		Title:   "Owned by user-1",
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	now := time.Now().UTC()
	revs := []types.Revision{
		{
			RevisionID:  "rev-1",
			DocID:       "doc-1",
			OwnerID:     "user-1",
			AuthorKind:  types.AuthorUser,
			AuthorLabel: "alice",
			Content:     "Content A",
			CreatedAt:   now,
		},
		{
			RevisionID:       "rev-2",
			DocID:            "doc-1",
			OwnerID:          "user-1",
			AuthorKind:       types.AuthorAppAgent,
			AuthorLabel:      "appagent",
			Content:          "Content B",
			ParentRevisionID: "rev-1",
			CreatedAt:        now.Add(time.Second),
		},
	}
	for _, r := range revs {
		r = testTextureRevisionWithBodyDoc(t, r)
		if err := s.CreateRevision(ctx, r); err != nil {
			t.Fatalf("CreateRevision: %v", err)
		}
	}

	// user-2 should not be able to diff user-1's revisions.
	_, err := s.GetDiff(ctx, "rev-1", "rev-2", "user-2")
	if err == nil {
		t.Error("GetDiff as wrong owner: expected error, got nil")
	}
}

// ----- Blame owner scope -----

func TestTextureBlameOwnerScope(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	doc := types.Document{
		DocID:   "doc-1",
		OwnerID: "user-1",
		Title:   "Owned by user-1",
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}

	rev := types.Revision{
		RevisionID:  "rev-1",
		DocID:       "doc-1",
		OwnerID:     "user-1",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "alice",
		Content:     "Content",
		CreatedAt:   time.Now().UTC(),
	}
	if err := s.CreateRevision(ctx, rev); err != nil {
		t.Fatalf("CreateRevision: %v", err)
	}

	// user-2 should not be able to blame user-1's revision.
	_, err := s.GetBlame(ctx, "rev-1", "user-2")
	if err != ErrNotFound {
		t.Errorf("GetBlame as wrong owner: err=%v, want ErrNotFound", err)
	}
}

// ----- Agent mutation tracking tests -----

func TestTextureAgentMutationCreateAndGet(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	m := AgentMutation{
		DocID:     "doc-1",
		RunID:     "task-1",
		OwnerID:   "user-1",
		State:     "pending",
		CreatedAt: time.Now().UTC(),
	}
	if err := s.CreateAgentMutation(ctx, m); err != nil {
		t.Fatalf("CreateAgentMutation: %v", err)
	}

	got, err := s.GetPendingAgentMutationByDoc(ctx, "doc-1", "user-1")
	if err != nil {
		t.Fatalf("GetPendingAgentMutationByDoc: %v", err)
	}
	if got == nil {
		t.Fatal("GetPendingAgentMutationByDoc returned nil")
	}
	if got.RunID != "task-1" {
		t.Errorf("RunID = %q, want %q", got.RunID, "task-1")
	}
	if got.State != "pending" {
		t.Errorf("State = %q, want %q", got.State, "pending")
	}
}

func TestTextureAgentMutationByTask(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	m := AgentMutation{
		DocID:     "doc-1",
		RunID:     "task-1",
		OwnerID:   "user-1",
		State:     "pending",
		CreatedAt: time.Now().UTC(),
	}
	if err := s.CreateAgentMutation(ctx, m); err != nil {
		t.Fatalf("CreateAgentMutation: %v", err)
	}

	got, err := s.GetAgentMutationByRun(ctx, "task-1")
	if err != nil {
		t.Fatalf("GetAgentMutationByRun: %v", err)
	}
	if got == nil {
		t.Fatal("GetAgentMutationByRun returned nil")
	}
	if got.DocID != "doc-1" {
		t.Errorf("DocID = %q, want %q", got.DocID, "doc-1")
	}
}

func TestTextureAgentMutationComplete(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	m := AgentMutation{
		DocID:     "doc-1",
		RunID:     "task-1",
		OwnerID:   "user-1",
		State:     "pending",
		CreatedAt: time.Now().UTC(),
	}
	if err := s.CreateAgentMutation(ctx, m); err != nil {
		t.Fatalf("CreateAgentMutation: %v", err)
	}

	// Complete the mutation.
	if err := s.CompleteAgentMutation(ctx, "task-1", "rev-agent-1"); err != nil {
		t.Fatalf("CompleteAgentMutation: %v", err)
	}

	// Verify the mutation is now completed.
	got, err := s.GetAgentMutationByRun(ctx, "task-1")
	if err != nil {
		t.Fatalf("GetAgentMutationByRun: %v", err)
	}
	if got.State != "completed" {
		t.Errorf("State = %q, want %q", got.State, "completed")
	}
	if got.RevisionID != "rev-agent-1" {
		t.Errorf("RevisionID = %q, want %q", got.RevisionID, "rev-agent-1")
	}
	if got.CompletedAt == nil {
		t.Error("CompletedAt is nil, want a timestamp")
	}

	// No pending mutation should be found for this doc.
	pending, err := s.GetPendingAgentMutationByDoc(ctx, "doc-1", "user-1")
	if err != nil {
		t.Fatalf("GetPendingAgentMutationByDoc: %v", err)
	}
	if pending != nil {
		t.Error("pending mutation should be nil after completion")
	}
}

func TestTextureAgentMutationNoDuplicateOnCompletion(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	m := AgentMutation{
		DocID:     "doc-1",
		RunID:     "task-1",
		OwnerID:   "user-1",
		State:     "pending",
		CreatedAt: time.Now().UTC(),
	}
	if err := s.CreateAgentMutation(ctx, m); err != nil {
		t.Fatalf("CreateAgentMutation: %v", err)
	}

	// Complete once.
	if err := s.CompleteAgentMutation(ctx, "task-1", "rev-agent-1"); err != nil {
		t.Fatalf("first CompleteAgentMutation: %v", err)
	}

	// Try to complete again — should fail with ErrMutationAlreadyCompleted.
	err := s.CompleteAgentMutation(ctx, "task-1", "rev-agent-2")
	if err != ErrMutationAlreadyCompleted {
		t.Errorf("second CompleteAgentMutation: err=%v, want ErrMutationAlreadyCompleted", err)
	}

	// Verify only the first revision ID was saved.
	got, _ := s.GetAgentMutationByRun(ctx, "task-1")
	if got.RevisionID != "rev-agent-1" {
		t.Errorf("RevisionID = %q, want %q (should not be overwritten by second completion)", got.RevisionID, "rev-agent-1")
	}
}

func TestTextureAgentMutationIdempotentCreation(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	m := AgentMutation{
		DocID:     "doc-1",
		RunID:     "task-1",
		OwnerID:   "user-1",
		State:     "pending",
		CreatedAt: time.Now().UTC(),
	}
	if err := s.CreateAgentMutation(ctx, m); err != nil {
		t.Fatalf("first CreateAgentMutation: %v", err)
	}

	// Creating the same mutation again should succeed (INSERT OR IGNORE).
	if err := s.CreateAgentMutation(ctx, m); err != nil {
		t.Fatalf("duplicate CreateAgentMutation: %v", err)
	}
}

func TestTextureAgentMutationFail(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	m := AgentMutation{
		DocID:     "doc-1",
		RunID:     "task-1",
		OwnerID:   "user-1",
		State:     "pending",
		CreatedAt: time.Now().UTC(),
	}
	if err := s.CreateAgentMutation(ctx, m); err != nil {
		t.Fatalf("CreateAgentMutation: %v", err)
	}

	if err := s.FailAgentMutation(ctx, "task-1"); err != nil {
		t.Fatalf("FailAgentMutation: %v", err)
	}

	got, _ := s.GetAgentMutationByRun(ctx, "task-1")
	if got.State != "failed" {
		t.Errorf("State = %q, want %q", got.State, "failed")
	}
}

func TestTextureAgentMutationMarkStaleClearsPending(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	m := AgentMutation{
		DocID:     "doc-1",
		RunID:     "task-stale",
		OwnerID:   "user-1",
		State:     "pending",
		CreatedAt: time.Now().UTC(),
	}
	if err := s.CreateAgentMutation(ctx, m); err != nil {
		t.Fatalf("CreateAgentMutation: %v", err)
	}
	if err := s.MarkAgentMutationStale(ctx, "task-stale"); err != nil {
		t.Fatalf("MarkAgentMutationStale: %v", err)
	}
	got, err := s.GetAgentMutationByRun(ctx, "task-stale")
	if err != nil {
		t.Fatalf("GetAgentMutationByRun: %v", err)
	}
	if got.State != "stale_activation" {
		t.Fatalf("State = %q, want stale_activation", got.State)
	}
	if got.CompletedAt == nil {
		t.Fatal("CompletedAt is nil, want a timestamp")
	}
	pending, err := s.GetPendingAgentMutationByDoc(ctx, "doc-1", "user-1")
	if err != nil {
		t.Fatalf("GetPendingAgentMutationByDoc: %v", err)
	}
	if pending != nil {
		t.Fatalf("pending mutation should be nil after stale reconciliation, got %+v", pending)
	}
}

func TestTextureAgentMutationNoCrossUserAccess(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	m := AgentMutation{
		DocID:     "doc-1",
		RunID:     "task-1",
		OwnerID:   "user-1",
		State:     "pending",
		CreatedAt: time.Now().UTC(),
	}
	if err := s.CreateAgentMutation(ctx, m); err != nil {
		t.Fatalf("CreateAgentMutation: %v", err)
	}

	// user-2 should not see user-1's pending mutation.
	got, err := s.GetPendingAgentMutationByDoc(ctx, "doc-1", "user-2")
	if err != nil {
		t.Fatalf("GetPendingAgentMutationByDoc as user-2: %v", err)
	}
	if got != nil {
		t.Error("user-2 should not see user-1's pending mutation")
	}
}
