package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestTextureSourceGraphCanonicalIDsUseSingleURLSafeSuffix(t *testing.T) {
	sourceID, err := BuildTextureSourceEntityCanonicalID("user:alice", "computer:user:alice", "Web_URL", "https://example.com/story?a=1:b=2")
	if err != nil {
		t.Fatalf("BuildTextureSourceEntityCanonicalID: %v", err)
	}
	if strings.Count(sourceID, ":") != 3 {
		t.Fatalf("source canonical ID %q uses extra colon-separated parts", sourceID)
	}
	kind, ownerID, suffix, err := objectgraph.ParseCanonicalID(sourceID)
	if err != nil {
		t.Fatalf("ParseCanonicalID(source): %v", err)
	}
	if kind != TextureSourceEntityObjectKind || ownerID != "user:alice" {
		t.Fatalf("parsed source ID kind/owner = %s/%s", kind, ownerID)
	}
	if strings.ContainsAny(suffix, ":/\\?#[]@!$&'()*+,;=") {
		t.Fatalf("source suffix %q is not URL-safe", suffix)
	}

	refID, err := BuildTextureSourceRefCanonicalID("user:alice", "rev:one", "doc/p-1/ref:1")
	if err != nil {
		t.Fatalf("BuildTextureSourceRefCanonicalID: %v", err)
	}
	if strings.Count(refID, ":") != 3 {
		t.Fatalf("source_ref canonical ID %q uses extra colon-separated parts", refID)
	}
	kind, ownerID, suffix, err = objectgraph.ParseCanonicalID(refID)
	if err != nil {
		t.Fatalf("ParseCanonicalID(ref): %v", err)
	}
	if kind != TextureSourceRefObjectKind || ownerID != "user:alice" {
		t.Fatalf("parsed source_ref ID kind/owner = %s/%s", kind, ownerID)
	}
	if strings.ContainsAny(suffix, ":/\\?#[]@!$&'()*+,;=") {
		t.Fatalf("source_ref suffix %q is not URL-safe", suffix)
	}
}

func TestCreateRevisionWithSourceGraphPersistsPinnedSourceRecords(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	doc := types.Document{DocID: "doc-source-graph", OwnerID: "user-1", Title: "Source Graph"}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	bodyDoc, sourceEntities := structuredRevisionFixture(t)
	rev := types.Revision{
		RevisionID:     "rev-source-graph",
		DocID:          doc.DocID,
		OwnerID:        doc.OwnerID,
		AuthorKind:     types.AuthorUser,
		AuthorLabel:    "alice",
		BodyDoc:        bodyDoc,
		SourceEntities: sourceEntities,
		CreatedAt:      time.Now().UTC().Truncate(time.Millisecond),
	}
	entity := testTextureSourceEntityRecord(t, doc.OwnerID, "src-web", "web_url", "https://example.com/story", []byte("source snapshot"))
	ref := testTextureSourceRefRecord(t, doc.OwnerID, doc.DocID, rev.RevisionID, "ref-1", entity)

	if err := s.CreateRevisionWithSourceGraph(ctx, rev, TextureSourceGraphWriteSet{
		SourceEntities: []TextureSourceEntityGraphRecord{entity},
		SourceRefs:     []TextureSourceRefGraphRecord{ref},
	}); err != nil {
		t.Fatalf("CreateRevisionWithSourceGraph: %v", err)
	}

	gotDoc, err := s.GetDocument(ctx, doc.DocID, doc.OwnerID)
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if gotDoc.CurrentRevisionID != rev.RevisionID {
		t.Fatalf("current_revision_id = %q, want %q", gotDoc.CurrentRevisionID, rev.RevisionID)
	}
	entities, err := s.ListTextureSourceEntities(ctx, doc.OwnerID)
	if err != nil {
		t.Fatalf("ListTextureSourceEntities: %v", err)
	}
	if len(entities) != 1 {
		t.Fatalf("source entity count = %d, want 1: %#v", len(entities), entities)
	}
	if entities[0].CanonicalID != entity.CanonicalID || entities[0].VersionID != entity.VersionID || entities[0].LegacySourceEntityID != "src-web" {
		t.Fatalf("source entity = %#v, want graph identity plus legacy id", entities[0])
	}
	refs, err := s.ListTextureSourceRefsForRevision(ctx, doc.OwnerID, doc.DocID, rev.RevisionID)
	if err != nil {
		t.Fatalf("ListTextureSourceRefsForRevision: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("source ref count = %d, want 1: %#v", len(refs), refs)
	}
	if refs[0].SourceEntityCanonicalID != entity.CanonicalID || refs[0].SourceEntityVersionID != entity.VersionID {
		t.Fatalf("source ref = %#v, want pinned source entity version", refs[0])
	}
	if refs[0].DisplayMode != TextureSourceRefDisplayNumbered || refs[0].CitationState != "cited" {
		t.Fatalf("source ref mode/state = %s/%s", refs[0].DisplayMode, refs[0].CitationState)
	}
}

func TestCreateRevisionWithSourceGraphFailureDoesNotAdvanceDocumentHead(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	doc := types.Document{DocID: "doc-source-graph-rollback", OwnerID: "user-1", Title: "Source Graph Rollback"}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	bodyDoc, sourceEntities := structuredRevisionFixture(t)
	rev := types.Revision{
		RevisionID:     "rev-source-graph-bad",
		DocID:          doc.DocID,
		OwnerID:        doc.OwnerID,
		AuthorKind:     types.AuthorUser,
		AuthorLabel:    "alice",
		BodyDoc:        bodyDoc,
		SourceEntities: sourceEntities,
		CreatedAt:      time.Now().UTC().Truncate(time.Millisecond),
	}
	entity := testTextureSourceEntityRecord(t, doc.OwnerID, "src-web", "web_url", "https://example.com/story", []byte("source snapshot"))
	ref := testTextureSourceRefRecord(t, doc.OwnerID, doc.DocID, rev.RevisionID, "ref-1", entity)

	err := s.CreateRevisionWithSourceGraph(ctx, rev, TextureSourceGraphWriteSet{
		SourceRefs: []TextureSourceRefGraphRecord{ref},
	})
	if err == nil || !strings.Contains(err.Error(), "missing source entity version") {
		t.Fatalf("CreateRevisionWithSourceGraph error = %v, want missing source entity version", err)
	}
	gotDoc, err := s.GetDocument(ctx, doc.DocID, doc.OwnerID)
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if gotDoc.CurrentRevisionID != "" {
		t.Fatalf("current_revision_id = %q, want unchanged empty head", gotDoc.CurrentRevisionID)
	}
	if _, err := s.GetRevision(ctx, rev.RevisionID, doc.OwnerID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetRevision after failed graph write = %v, want ErrNotFound", err)
	}
	refs, err := s.ListTextureSourceRefsForRevision(ctx, doc.OwnerID, doc.DocID, rev.RevisionID)
	if err != nil {
		t.Fatalf("ListTextureSourceRefsForRevision: %v", err)
	}
	if len(refs) != 0 {
		t.Fatalf("source refs persisted after failed graph write: %#v", refs)
	}
}

func TestListTextureSourceGraphForRevisionsBatchesRevisionScopedWrappers(t *testing.T) {
	s := textureTestStore(t)
	ctx := context.Background()

	doc := types.Document{DocID: "doc-source-graph-batch", OwnerID: "user-1", Title: "Source Graph Batch"}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	bodyDoc, sourceEntities := structuredRevisionFixture(t)
	rev0 := types.Revision{
		RevisionID:     "rev-source-graph-batch-0",
		DocID:          doc.DocID,
		OwnerID:        doc.OwnerID,
		AuthorKind:     types.AuthorUser,
		AuthorLabel:    "alice",
		BodyDoc:        bodyDoc,
		SourceEntities: sourceEntities,
		CreatedAt:      time.Now().UTC().Truncate(time.Millisecond),
	}
	entityOnly := testTextureSourceEntityRecordForRevision(t, doc.OwnerID, doc.DocID, rev0.RevisionID, "src-entity-only", "web_url", "https://example.com/entity-only")
	if err := s.CreateRevisionWithSourceGraph(ctx, rev0, TextureSourceGraphWriteSet{
		SourceEntities: []TextureSourceEntityGraphRecord{entityOnly},
	}); err != nil {
		t.Fatalf("CreateRevisionWithSourceGraph rev0: %v", err)
	}

	rev1 := types.Revision{
		RevisionID:       "rev-source-graph-batch-1",
		DocID:            doc.DocID,
		OwnerID:          doc.OwnerID,
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      "alice",
		BodyDoc:          bodyDoc,
		SourceEntities:   sourceEntities,
		ParentRevisionID: rev0.RevisionID,
		CreatedAt:        rev0.CreatedAt.Add(time.Second),
	}
	pinnedEntity := testTextureSourceEntityRecordForRevision(t, doc.OwnerID, doc.DocID, rev1.RevisionID, "src-pinned", "web_url", "https://example.com/pinned")
	pinnedRef := testTextureSourceRefRecord(t, doc.OwnerID, doc.DocID, rev1.RevisionID, "ref-1", pinnedEntity)
	if err := s.CreateRevisionWithSourceGraph(ctx, rev1, TextureSourceGraphWriteSet{
		SourceEntities: []TextureSourceEntityGraphRecord{pinnedEntity},
		SourceRefs:     []TextureSourceRefGraphRecord{pinnedRef},
	}); err != nil {
		t.Fatalf("CreateRevisionWithSourceGraph rev1: %v", err)
	}

	graphByRevision, err := s.ListTextureSourceGraphForRevisions(ctx, doc.OwnerID, doc.DocID, []string{rev1.RevisionID, rev0.RevisionID, rev1.RevisionID, ""})
	if err != nil {
		t.Fatalf("ListTextureSourceGraphForRevisions: %v", err)
	}
	if len(graphByRevision[rev0.RevisionID].SourceEntities) != 1 || len(graphByRevision[rev0.RevisionID].SourceRefs) != 0 {
		t.Fatalf("rev0 graph = %#v, want entity-only wrapper", graphByRevision[rev0.RevisionID])
	}
	if graphByRevision[rev0.RevisionID].SourceEntities[0].CanonicalID != entityOnly.CanonicalID {
		t.Fatalf("rev0 source entity = %#v, want %#v", graphByRevision[rev0.RevisionID].SourceEntities[0], entityOnly)
	}
	if len(graphByRevision[rev1.RevisionID].SourceEntities) != 1 || len(graphByRevision[rev1.RevisionID].SourceRefs) != 1 {
		t.Fatalf("rev1 graph = %#v, want pinned source entity/ref", graphByRevision[rev1.RevisionID])
	}
	if graphByRevision[rev1.RevisionID].SourceEntities[0].CanonicalID != pinnedEntity.CanonicalID ||
		graphByRevision[rev1.RevisionID].SourceRefs[0].SourceEntityCanonicalID != pinnedEntity.CanonicalID {
		t.Fatalf("rev1 graph = %#v, want pinned graph entity %#v", graphByRevision[rev1.RevisionID], pinnedEntity)
	}
}

func testTextureSourceEntityRecord(t *testing.T, ownerID, legacyID, sourceKind, targetIdentity string, body []byte) TextureSourceEntityGraphRecord {
	t.Helper()
	canonicalID, err := BuildTextureSourceEntityCanonicalID(ownerID, ownerID, sourceKind, targetIdentity)
	if err != nil {
		t.Fatalf("BuildTextureSourceEntityCanonicalID: %v", err)
	}
	metadata := json.RawMessage(`{"schema_version":"choir.source_entity.v1","source_kind":"web_url","target":{"kind":"url","identity":"https://example.com/story"},"display":{"title":"Example story","url":"https://example.com/story"},"evidence":{"state":"available"}}`)
	versionID, contentHash, normalized, err := TextureSourceGraphVersionID(TextureSourceEntityObjectKind, body, metadata)
	if err != nil {
		t.Fatalf("TextureSourceGraphVersionID(source): %v", err)
	}
	return TextureSourceEntityGraphRecord{
		CanonicalID:          canonicalID,
		OwnerID:              ownerID,
		VersionID:            versionID,
		ContentHash:          contentHash,
		Body:                 body,
		Metadata:             normalized,
		LegacySourceEntityID: legacyID,
	}
}

func testTextureSourceEntityRecordForRevision(t *testing.T, ownerID, docID, revisionID, legacyID, sourceKind, targetIdentity string) TextureSourceEntityGraphRecord {
	t.Helper()
	canonicalID, err := BuildTextureSourceEntityCanonicalID(ownerID, ownerID, sourceKind, targetIdentity)
	if err != nil {
		t.Fatalf("BuildTextureSourceEntityCanonicalID: %v", err)
	}
	metadata, err := json.Marshal(map[string]any{
		"schema_version":       "choir.source_entity.v1",
		"source_kind":          sourceKind,
		"target":               map[string]any{"kind": sourceKind, "identity": targetIdentity},
		"display":              map[string]any{"title": legacyID, "url": targetIdentity},
		"evidence":             map[string]any{"state": "available"},
		"texture_doc_id":       docID,
		"texture_revision_id":  revisionID,
		"legacy_source_entity": legacyID,
	})
	if err != nil {
		t.Fatalf("marshal source entity metadata: %v", err)
	}
	body := []byte("source snapshot " + legacyID)
	versionID, contentHash, normalized, err := TextureSourceGraphVersionID(TextureSourceEntityObjectKind, body, metadata)
	if err != nil {
		t.Fatalf("TextureSourceGraphVersionID(source): %v", err)
	}
	return TextureSourceEntityGraphRecord{
		CanonicalID:          canonicalID,
		OwnerID:              ownerID,
		VersionID:            versionID,
		ContentHash:          contentHash,
		Body:                 body,
		Metadata:             normalized,
		LegacySourceEntityID: legacyID,
	}
}

func testTextureSourceRefRecord(t *testing.T, ownerID, docID, revisionID, occurrenceKey string, entity TextureSourceEntityGraphRecord) TextureSourceRefGraphRecord {
	t.Helper()
	canonicalID, err := BuildTextureSourceRefCanonicalID(ownerID, revisionID, occurrenceKey)
	if err != nil {
		t.Fatalf("BuildTextureSourceRefCanonicalID: %v", err)
	}
	rec := TextureSourceRefGraphRecord{
		CanonicalID:             canonicalID,
		OwnerID:                 ownerID,
		DocID:                   docID,
		TextureRevisionID:       revisionID,
		BodyNodeID:              "ref-1",
		BodyNodePathHash:        objectgraph.SHA256([]byte("doc/p-1/ref-1")),
		LegacySourceEntityID:    entity.LegacySourceEntityID,
		SourceEntityCanonicalID: entity.CanonicalID,
		SourceEntityVersionID:   entity.VersionID,
		DisplayMode:             TextureSourceRefDisplayNumbered,
		CitationState:           "cited",
		Metadata:                json.RawMessage(`{"schema_version":"choir.source_ref.v1"}`),
	}
	versionID, contentHash, normalized, err := TextureSourceGraphVersionID(TextureSourceRefObjectKind, sourceRefVersionBody(rec), rec.Metadata)
	if err != nil {
		t.Fatalf("TextureSourceGraphVersionID(ref): %v", err)
	}
	rec.VersionID = versionID
	rec.ContentHash = contentHash
	rec.Metadata = normalized
	return rec
}
