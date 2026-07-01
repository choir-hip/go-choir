package platform

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	embedded "github.com/dolthub/driver"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourcecontract"
)

func openTestPlatformStore(t *testing.T) (*Store, string) {
	t.Helper()
	root := t.TempDir()
	rootDSN := fmt.Sprintf("file://%s?commitname=Choir&commitemail=system@choir.local&multistatements=true", root)
	rootCfg, err := embedded.ParseDSN(rootDSN)
	if err != nil {
		t.Fatalf("parse root dsn: %v", err)
	}
	rootConnector, err := embedded.NewConnector(rootCfg)
	if err != nil {
		t.Fatalf("new root connector: %v", err)
	}
	rootDB := sql.OpenDB(rootConnector)
	if _, err := rootDB.Exec("CREATE DATABASE IF NOT EXISTS platform"); err != nil {
		t.Fatalf("create database: %v", err)
	}
	_ = rootDB.Close()
	_ = rootConnector.Close()

	dbDSN := fmt.Sprintf("file://%s?commitname=Choir&commitemail=system@choir.local&database=platform&multistatements=true&clientfoundrows=true", root)
	dbCfg, err := embedded.ParseDSN(dbDSN)
	if err != nil {
		t.Fatalf("parse db dsn: %v", err)
	}
	dbConnector, err := embedded.NewConnector(dbCfg)
	if err != nil {
		t.Fatalf("new db connector: %v", err)
	}
	db := sql.OpenDB(dbConnector)
	s := NewStore(db)
	if err := s.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() {
		_ = s.Close()
		_ = dbConnector.Close()
	})
	return s, root
}

func platformTableCount(t *testing.T, s *Store, table string) int64 {
	t.Helper()
	var count int64
	if err := s.db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}

func testTextureBodyDoc(t *testing.T, docID string, blocks ...map[string]any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"schema": "choir.texture_doc.v1",
		"doc": map[string]any{
			"type":    "doc",
			"attrs":   map[string]any{"id": docID},
			"content": blocks,
		},
	})
	if err != nil {
		t.Fatalf("marshal body_doc: %v", err)
	}
	return raw
}

func testTextureHeading(id string, level int, content ...map[string]any) map[string]any {
	return map[string]any{
		"type":    "heading",
		"attrs":   map[string]any{"id": id, "level": level},
		"content": content,
	}
}

func testTextureParagraph(id string, content ...map[string]any) map[string]any {
	return map[string]any{
		"type":    "paragraph",
		"attrs":   map[string]any{"id": id},
		"content": content,
	}
}

func testTextureText(text string) map[string]any {
	return map[string]any{"type": "text", "text": text}
}

func testTextureStrong(text string) map[string]any {
	return map[string]any{
		"type":  "text",
		"text":  text,
		"marks": []map[string]any{{"type": "strong"}},
	}
}

func testTextureSourceRef(id, sourceEntityID, label string) map[string]any {
	return map[string]any{
		"type": "source_ref",
		"attrs": map[string]any{
			"id":               id,
			"source_entity_id": sourceEntityID,
			"label":            label,
			"display_mode":     "numbered_ref",
		},
	}
}

func testTextureSourceEntities(t *testing.T, entities ...map[string]any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(entities)
	if err != nil {
		t.Fatalf("marshal source_entities: %v", err)
	}
	return raw
}

func testTextureSourceEntity(id, kind, targetKind, targetID, title, quote, state, openSurface string) map[string]any {
	target := map[string]any{"kind": targetKind}
	if strings.HasPrefix(targetID, "http://") || strings.HasPrefix(targetID, "https://") {
		target["uri"] = targetID
	} else if targetID != "" {
		target["id"] = targetID
	}
	return map[string]any{
		"source_entity_id": id,
		"kind":             kind,
		"target":           target,
		"selectors": []map[string]any{{
			"kind": "text_quote",
			"data": map[string]any{
				"text_quote": quote,
				"exact":      quote,
			},
		}},
		"display": map[string]any{
			"mode":  "numbered_ref",
			"title": title,
			"label": title,
		},
		"evidence": map[string]any{
			"state":        state,
			"open_surface": openSurface,
		},
		"provenance": map[string]any{
			"created_by": "platform-test",
		},
	}
}

func TestPlatformTextureStoreWritesCurrentTables(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	ctx := context.Background()

	if err := store.UpsertTextureDocument(ctx, "doc-current", "owner-current", "Current Texture"); err != nil {
		t.Fatalf("UpsertTextureDocument: %v", err)
	}
	if err := store.UpsertTextureRevision(ctx, PlatformTextureRevision{
		RevisionID:  "rev-current",
		DocID:       "doc-current",
		OwnerID:     "owner-current",
		AuthorKind:  "agent",
		AuthorLabel: "texture",
		Content:     "current table content",
	}); err != nil {
		t.Fatalf("UpsertTextureRevision: %v", err)
	}

	if got := platformTableCount(t, store, "platform_texture_documents"); got != 1 {
		t.Fatalf("platform_texture_documents count = %d, want 1", got)
	}
	if got := platformTableCount(t, store, "platform_texture_revisions"); got != 1 {
		t.Fatalf("platform_texture_revisions count = %d, want 1", got)
	}

	doc, err := store.GetTextureDocument(ctx, "doc-current")
	if err != nil {
		t.Fatalf("GetTextureDocument: %v", err)
	}
	if doc.Title != "Current Texture" {
		t.Fatalf("document title = %q, want Current Texture", doc.Title)
	}
	if doc.CurrentRevisionID != "rev-current" {
		t.Fatalf("document current revision = %q, want rev-current", doc.CurrentRevisionID)
	}
	revs, err := store.ListTextureRevisions(ctx, "doc-current")
	if err != nil {
		t.Fatalf("ListTextureRevisions: %v", err)
	}
	if len(revs) != 1 || revs[0].RevisionID != "rev-current" || revs[0].Content != "current table content" {
		t.Fatalf("current revisions = %+v, want rev-current content", revs)
	}
}

func TestPlatformTextureStoreBootstrapPreservesCurrentTextureRows(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	ctx := context.Background()
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)

	if _, err := store.db.ExecContext(ctx, `ALTER TABLE platform_texture_revisions DROP COLUMN source_entities`); err != nil {
		t.Fatalf("drop source_entities to simulate old schema: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `ALTER TABLE platform_texture_revisions DROP COLUMN body_doc`); err != nil {
		t.Fatalf("drop body_doc to simulate old schema: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO platform_texture_documents (doc_id, owner_id, title, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		"doc-existing", "owner-existing", "Existing Texture", now, now); err != nil {
		t.Fatalf("insert existing document: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO platform_texture_revisions (revision_id, doc_id, owner_id, parent_revision_id, author_kind, author_label, content, citations, metadata, created_at) VALUES (?, ?, ?, '', ?, ?, ?, ?, ?, ?)`,
		"rev-existing", "doc-existing", "owner-existing", "agent", "texture", "existing table content", "[]", `{"source":"test"}`, now); err != nil {
		t.Fatalf("insert existing revision: %v", err)
	}
	if err := store.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap with existing rows: %v", err)
	}
	if got := platformTableCount(t, store, "platform_texture_documents"); got != 1 {
		t.Fatalf("platform_texture_documents count after bootstrap = %d, want 1", got)
	}
	if got := platformTableCount(t, store, "platform_texture_revisions"); got != 1 {
		t.Fatalf("platform_texture_revisions count after bootstrap = %d, want 1", got)
	}
	if err := store.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap second run: %v", err)
	}
	if got := platformTableCount(t, store, "platform_texture_documents"); got != 1 {
		t.Fatalf("platform_texture_documents count after second bootstrap = %d, want 1", got)
	}
	if got := platformTableCount(t, store, "platform_texture_revisions"); got != 1 {
		t.Fatalf("platform_texture_revisions count after second bootstrap = %d, want 1", got)
	}

	doc, err := store.GetTextureDocument(ctx, "doc-existing")
	if err != nil {
		t.Fatalf("GetTextureDocument existing: %v", err)
	}
	if doc.OwnerID != "owner-existing" || doc.Title != "Existing Texture" {
		t.Fatalf("existing document = %+v, want owner/title preserved", doc)
	}
	rev, err := store.GetTextureRevision(ctx, "rev-existing")
	if err != nil {
		t.Fatalf("GetTextureRevision existing: %v", err)
	}
	if rev.DocID != "doc-existing" || rev.Content != "existing table content" {
		t.Fatalf("existing revision = %+v, want preserved revision", rev)
	}
	if string(rev.BodyDoc) != "" || string(rev.SourceEntities) != "" {
		t.Fatalf("migrated old revision structured fields = %q/%q, want empty defaults", rev.BodyDoc, rev.SourceEntities)
	}
}

func decodeBase64(value string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(value)
}

type publicationExportMetadataEnvelope struct {
	AccessPolicy           json.RawMessage           `json:"access_policy"`
	ExportPolicy           json.RawMessage           `json:"export_policy"`
	ExportProfile          publicationExportProfile  `json:"export_profile"`
	Retrieval              RetrievalBundle           `json:"retrieval"`
	PrivateMaterialOmitted bool                      `json:"private_material_omitted"`
	SourceEntities         []PublicationSourceEntity `json:"source_entities"`
	Transclusions          []PublicationTransclusion `json:"transclusions"`
	SourceManifest         publicationSourceManifest `json:"source_manifest"`
}

func decodePublicationExportMetadata(t *testing.T, raw json.RawMessage) publicationExportMetadataEnvelope {
	t.Helper()
	var out publicationExportMetadataEnvelope
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode export metadata: %v\n%s", err, string(raw))
	}
	return out
}

func TestPublicationFallbackDefaultsUseTextureLabels(t *testing.T) {
	bundle := &PublicationBundle{
		Route:       PublicationRoute{Path: "/pub/texture/example"},
		Publication: PublicationSummary{},
		Version: PublicationVersionSummary{
			ID: "pubver-1",
		},
		Artifact: PublicationArtifact{Content: "Fallback body."},
	}

	doc := buildPublicationDocument(bundle)
	if doc.Title != defaultPublishedTextureTitle {
		t.Fatalf("publication document title = %q, want %q", doc.Title, defaultPublishedTextureTitle)
	}

	coreXML := docxCoreXML(bundle)
	if !strings.Contains(coreXML, "<dc:title>"+defaultPublishedTextureTitle+"</dc:title>") {
		t.Fatalf("DOCX core title missing Texture fallback: %s", coreXML)
	}

	if got := publicationExportFilename("", "", "txt"); got != defaultPublishedTextureSlugBase+".txt" {
		t.Fatalf("empty export filename = %q, want %q", got, defaultPublishedTextureSlugBase+".txt")
	}
	if got := publicationExportFilename("   ", "   ", "md"); got != defaultPublishedTextureSlugBase+".md" {
		t.Fatalf("blank export filename = %q, want %q", got, defaultPublishedTextureSlugBase+".md")
	}
}

func TestPublicationPersistedDefaultTitlesUseTextureLabels(t *testing.T) {
	store, root := openTestPlatformStore(t)
	svc := NewService(store, filepath.Join(root, "artifacts"), "")

	resp, err := svc.PublishTexture(context.Background(), PublishTextureRequest{
		OwnerID:          "owner-defaults",
		SourceDocID:      "doc-defaults",
		SourceRevisionID: "rev-defaults",
		Content:          "A publication that relies on the default Texture title.",
		RequestedBy:      "owner-defaults",
	})
	if err != nil {
		t.Fatalf("PublishTexture: %v", err)
	}
	if !strings.HasPrefix(resp.RoutePath, "/pub/texture/untitled-texture-") {
		t.Fatalf("default route path = %q, want /pub/texture/untitled-texture-*", resp.RoutePath)
	}

	var publicationTitle, publicationSlug string
	if err := store.db.QueryRowContext(context.Background(),
		`SELECT JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), '$.title')), JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), '$.slug')) FROM og_objects WHERE object_kind = 'choir.publication' AND JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), '$.publication_id')) = ?`,
		resp.PublicationID).Scan(&publicationTitle, &publicationSlug); err != nil {
		t.Fatalf("query publication defaults: %v", err)
	}
	if publicationTitle != defaultUntitledTextureTitle {
		t.Fatalf("publication title = %q, want %q", publicationTitle, defaultUntitledTextureTitle)
	}
	if !strings.Contains(publicationTitle, "Texture") || !strings.Contains(publicationSlug, "texture") {
		t.Fatalf("publication default missing Texture name: title=%q slug=%q", publicationTitle, publicationSlug)
	}

	publishedExport, err := svc.ExportPublicationByRoute(context.Background(), resp.RoutePath, "txt")
	if err != nil {
		t.Fatalf("ExportPublicationByRoute: %v", err)
	}
	if !strings.HasPrefix(publishedExport.Filename, "untitled-texture-") {
		t.Fatalf("default export filename = %q, want untitled-texture-*", publishedExport.Filename)
	}
	if !strings.Contains(publishedExport.Filename, "texture") {
		t.Fatalf("default export filename missing Texture name: %q", publishedExport.Filename)
	}

	proposal, err := svc.SubmitPublicationProposal(context.Background(), SubmitPublicationProposalRequest{
		PublicationID:        resp.PublicationID,
		PublicationVersionID: resp.PublicationVersionID,
		SubmitterID:          "reader-defaults",
		SubmitterDocID:       "reader-doc-defaults",
		SubmitterRevisionID:  "reader-rev-defaults",
		Content:              "A reader proposal that relies on the default Texture title.",
		RequestedBy:          "reader-defaults",
	})
	if err != nil {
		t.Fatalf("SubmitPublicationProposal: %v", err)
	}
	var proposalTitle string
	if err := store.db.QueryRowContext(context.Background(),
		`SELECT JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), '$.title')) FROM og_objects WHERE object_kind = 'choir.publication_proposal' AND JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), '$.proposal_id')) = ?`,
		proposal.ProposalID).Scan(&proposalTitle); err != nil {
		t.Fatalf("query proposal defaults: %v", err)
	}
	if proposalTitle != defaultTextureProposalTitle {
		t.Fatalf("proposal title = %q, want %q", proposalTitle, defaultTextureProposalTitle)
	}
	if !strings.Contains(proposalTitle, "Texture") {
		t.Fatalf("proposal default missing Texture name: %q", proposalTitle)
	}
}

func evidenceStateBySourceEntityID(t *testing.T, transclusions []PublicationTransclusion) map[string]map[string]any {
	t.Helper()
	out := make(map[string]map[string]any, len(transclusions))
	for _, transclusion := range transclusions {
		var selector struct {
			EvidenceState map[string]any `json:"evidence_state"`
		}
		if err := json.Unmarshal(transclusion.SourceSelector, &selector); err != nil {
			t.Fatalf("decode source selector for %s: %v\n%s", transclusion.SourceEntityID, err, string(transclusion.SourceSelector))
		}
		out[transclusion.SourceEntityID] = selector.EvidenceState
	}
	return out
}

func sourceEntitiesBySourceEntityID(entities []PublicationSourceEntity) map[string]PublicationSourceEntity {
	out := make(map[string]PublicationSourceEntity, len(entities))
	for _, entity := range entities {
		out[entity.SourceEntityID] = entity
	}
	return out
}

func transclusionsBySourceEntityID(transclusions []PublicationTransclusion) map[string]PublicationTransclusion {
	out := make(map[string]PublicationTransclusion, len(transclusions))
	for _, transclusion := range transclusions {
		out[transclusion.SourceEntityID] = transclusion
	}
	return out
}

func assertPublishedEvidenceState(t *testing.T, surface, want string, got map[string]any) {
	t.Helper()
	if got["state"] != want {
		t.Fatalf("%s evidence state = %#v, want state %q", surface, got, want)
	}
	if researchState := got["research_state"]; researchState != fmt.Sprintf("research_%s", want) {
		t.Fatalf("%s evidence research_state = %#v, want research_%s", surface, got, want)
	}
	if uncertainty := got["uncertainty"]; uncertainty != fmt.Sprintf("uncertainty for %s", want) {
		t.Fatalf("%s evidence uncertainty = %#v, want uncertainty for %s", surface, got, want)
	}
	switch want {
	case sourcecontract.EvidenceStateConfirms, sourcecontract.EvidenceStateRefutes, sourcecontract.EvidenceStateQualifies:
		if got["relation"] != want {
			t.Fatalf("%s evidence relation = %#v, want %q", surface, got, want)
		}
	default:
		if _, ok := got["relation"]; ok {
			t.Fatalf("%s non-relational evidence should not carry relation: %#v", surface, got)
		}
	}
}

func assertPublishedSourceContractCase(t *testing.T, surface string, tc publicationSourceContractCase, entity PublicationSourceEntity, transclusion PublicationTransclusion) {
	t.Helper()
	if entity.SourceEntityID == "" {
		t.Fatalf("%s missing source entity for %s", surface, tc.entityID)
	}
	if transclusion.SourceEntityID == "" {
		t.Fatalf("%s missing transclusion for %s", surface, tc.entityID)
	}
	if entity.TargetKind != tc.targetKind || entity.TargetID != tc.targetID {
		t.Fatalf("%s target = %s/%s, want %s/%s", surface, entity.TargetKind, entity.TargetID, tc.targetKind, tc.targetID)
	}
	if entity.OpenSurface != tc.wantOpenSurface {
		t.Fatalf("%s open surface = %q, want %q", surface, entity.OpenSurface, tc.wantOpenSurface)
	}
	var entityJSON map[string]any
	if err := json.Unmarshal(entity.Entity, &entityJSON); err != nil {
		t.Fatalf("%s decode entity json for %s: %v\n%s", surface, tc.entityID, err, string(entity.Entity))
	}
	display := mapValue(entityJSON["display"])
	if display["open_surface"] != tc.wantOpenSurface {
		t.Fatalf("%s entity json open surface = %#v, want %q from %s", surface, display["open_surface"], tc.wantOpenSurface, string(entity.Entity))
	}
	status := mapValue(entityJSON["reader_snapshot_status"])
	if status["state"] != tc.wantReaderState {
		t.Fatalf("%s reader snapshot status = %#v, want state %q from %s", surface, status, tc.wantReaderState, string(entity.Entity))
	}
	var selector map[string]any
	if err := json.Unmarshal(transclusion.SourceSelector, &selector); err != nil {
		t.Fatalf("%s decode source selector for %s: %v\n%s", surface, tc.entityID, err, string(transclusion.SourceSelector))
	}
	if selector["selector_kind"] != tc.wantSelectorKind {
		t.Fatalf("%s selector kind = %#v, want %q from %s", surface, selector["selector_kind"], tc.wantSelectorKind, string(transclusion.SourceSelector))
	}
	assertPublishedEvidenceState(t, surface, tc.wantEvidenceState, mapValue(selector["evidence_state"]))
	if transclusion.SnapshotText != tc.quote || transclusion.ContentHash != tc.contentHash {
		t.Fatalf("%s transclusion snapshot/hash = %#v, want %q/%q", surface, transclusion, tc.quote, tc.contentHash)
	}
}

type publicationSourceContractCase struct {
	entityID          string
	kind              string
	targetKind        string
	targetID          string
	target            map[string]any
	rawReaderState    string
	wantReaderState   string
	readerQuality     string
	rawSelectorKind   string
	wantSelectorKind  string
	rawOpenSurface    string
	wantOpenSurface   string
	rawEvidenceState  string
	wantEvidenceState string
	quote             string
	contentHash       string
}

func TestSyncTextureDocumentPersistsDocumentAndRevisions(t *testing.T) {
	store, root := openTestPlatformStore(t)
	svc := NewService(store, filepath.Join(root, "artifacts"), "")

	createdAt := time.Date(2026, time.June, 10, 11, 0, 0, 0, time.UTC)
	req := SyncTextureDocumentRequest{
		DocID:   "doc-123",
		OwnerID: "user-1",
		Title:   "Platform Draft",
		Revisions: []SyncTextureRevision{
			{
				RevisionID: "rev-1",
				Content:    "first revision",
				CreatedAt:  createdAt,
			},
			{
				RevisionID:       "rev-2",
				ParentRevisionID: "rev-1",
				AuthorKind:       "human",
				AuthorLabel:      "Wiz",
				Content:          "second revision",
				BodyDoc:          json.RawMessage(`{"schema":"choir.texture_doc.v1","doc":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"second revision"},{"type":"source_ref","attrs":{"source_entity_id":"src-wire"}}]}]}}`),
				SourceEntities:   json.RawMessage(`[{"source_entity_id":"src-wire","target":{"kind":"url","uri":"https://example.com/source"},"display":{"mode":"numbered_ref","title":"Wire source"},"evidence":{"state":"available","open_surface":"source"}}]`),
				Citations:        json.RawMessage(`[{"kind":"url","value":"https://example.com"}]`),
				Metadata:         json.RawMessage(`{"source":"test"}`),
				CreatedAt:        createdAt.Add(time.Minute),
			},
		},
	}

	resp, err := svc.SyncTextureDocument(context.Background(), req)
	if err != nil {
		t.Fatalf("SyncTextureDocument: %v", err)
	}
	if resp.DocID != req.DocID {
		t.Fatalf("doc id: got %q want %q", resp.DocID, req.DocID)
	}
	if resp.RevisionCount != len(req.Revisions) {
		t.Fatalf("revision count: got %d want %d", resp.RevisionCount, len(req.Revisions))
	}

	doc, err := svc.GetPlatformTextureDocument(context.Background(), req.DocID)
	if err != nil {
		t.Fatalf("GetPlatformTextureDocument: %v", err)
	}
	if doc.OwnerID != req.OwnerID || doc.Title != req.Title {
		t.Fatalf("document mismatch: %#v", doc)
	}
	if doc.CurrentRevisionID != "rev-2" {
		t.Fatalf("document current revision = %q, want rev-2", doc.CurrentRevisionID)
	}

	revisions, err := svc.ListPlatformTextureRevisions(context.Background(), req.DocID)
	if err != nil {
		t.Fatalf("ListPlatformTextureRevisions: %v", err)
	}
	if len(revisions) != 2 {
		t.Fatalf("revision len: got %d want 2", len(revisions))
	}
	if revisions[0].RevisionID != "rev-1" || revisions[1].RevisionID != "rev-2" {
		t.Fatalf("revision order mismatch: %#v", revisions)
	}
	if string(revisions[0].Citations) != "[]" || string(revisions[0].Metadata) != "{}" {
		t.Fatalf("revision defaults mismatch: citations=%s metadata=%s", revisions[0].Citations, revisions[0].Metadata)
	}

	rev, err := svc.GetPlatformTextureRevision(context.Background(), "rev-2")
	if err != nil {
		t.Fatalf("GetPlatformTextureRevision: %v", err)
	}
	if rev.ParentRevisionID != "rev-1" || rev.AuthorKind != "human" || rev.AuthorLabel != "Wiz" {
		t.Fatalf("revision metadata mismatch: %#v", rev)
	}
	if string(rev.Citations) != `[{"kind":"url","value":"https://example.com"}]` {
		t.Fatalf("revision citations mismatch: %s", rev.Citations)
	}
	if string(rev.Metadata) != `{"source":"test"}` {
		t.Fatalf("revision metadata mismatch: %s", rev.Metadata)
	}
	if !strings.Contains(string(rev.BodyDoc), `"source_ref"`) {
		t.Fatalf("revision body_doc not preserved: %s", rev.BodyDoc)
	}
	if !strings.Contains(string(rev.SourceEntities), `"src-wire"`) {
		t.Fatalf("revision source_entities not preserved: %s", rev.SourceEntities)
	}
}

func TestPublicationExportDocxAndPDFUseCanonicalPublicationBytes(t *testing.T) {
	store, root := openTestPlatformStore(t)
	svc := NewService(store, filepath.Join(root, "artifacts"), "")
	metadata, err := json.Marshal(map[string]any{})
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}
	bodyDoc := testTextureBodyDoc(t, "doc-export-proof",
		testTextureHeading("h-export-proof", 1, testTextureText("Export Proof")),
		testTextureParagraph("p-export-source",
			testTextureText("This is the published projection with "),
			testTextureSourceRef("ref-export-proof", "src-export-proof", "Export source proof"),
			testTextureText("."),
		),
		testTextureParagraph("p-export-strong",
			testTextureText("A "),
			testTextureStrong("private legal cloud"),
			testTextureText(" survives rich export without Markdown syntax."),
		),
		testTextureParagraph("p-export-long", testTextureText(strings.Repeat("Long document proof line with enough content to require PDF pagination. ", 80))),
		testTextureParagraph("p-export-last", testTextureText("Last line must survive export.")),
	)
	exportSource := testTextureSourceEntity(
		"src-export-proof",
		"web_source",
		"url",
		"https://example.com/export-proof",
		"Export source proof",
		"This source snapshot must survive rich export.",
		"confirms",
		"source",
	)
	exportSource["evidence"].(map[string]any)["reader_artifact_state"] = "snapshot_ready"
	sourceEntities := testTextureSourceEntities(t, exportSource)

	resp, err := svc.PublishTexture(context.Background(), PublishTextureRequest{
		OwnerID:          "user-1",
		SourceDocID:      "doc-1",
		SourceRevisionID: "rev-1",
		Title:            "Export Proof",
		Content:          "# Export Proof\n\nThis is the published projection with [1].\n\nA private legal cloud survives rich export without Markdown syntax.\n\n" + strings.Repeat("Long document proof line with enough content to require PDF pagination. ", 80) + "\n\nLast line must survive export.",
		BodyDoc:          bodyDoc,
		SourceEntities:   sourceEntities,
		Metadata:         metadata,
		RequestedBy:      "user-1",
	})
	if err != nil {
		t.Fatalf("PublishTexture: %v", err)
	}

	docxExport, err := svc.ExportPublicationByRoute(context.Background(), resp.RoutePath, "docx")
	if err != nil {
		t.Fatalf("ExportPublicationByRoute docx: %v", err)
	}
	if docxExport.Format != "docx" || docxExport.Content != "" || docxExport.ContentBase64 == "" {
		t.Fatalf("docx export shape = %#v", docxExport)
	}
	if docxExport.MediaType != "application/vnd.openxmlformats-officedocument.wordprocessingml.document" || !strings.HasSuffix(docxExport.Filename, ".docx") {
		t.Fatalf("docx metadata = %#v", docxExport)
	}
	docxMetadata := decodePublicationExportMetadata(t, docxExport.Metadata)
	if !docxMetadata.PrivateMaterialOmitted || !strings.Contains(string(docxMetadata.AccessPolicy), `"visibility":"public"`) || !strings.Contains(string(docxMetadata.ExportPolicy), `"download_allowed":true`) {
		t.Fatalf("docx export policy metadata = %#v access=%s export=%s", docxMetadata, string(docxMetadata.AccessPolicy), string(docxMetadata.ExportPolicy))
	}
	if docxMetadata.Retrieval.SourceID == "" || len(docxMetadata.Retrieval.Spans) != 1 || docxMetadata.Retrieval.Spans[0].ID == "" {
		t.Fatalf("docx export retrieval metadata = %#v", docxMetadata.Retrieval)
	}
	if docxMetadata.SourceManifest.Schema != "choir.publication_sources.v1" || len(docxMetadata.SourceManifest.Sources) != 1 {
		t.Fatalf("docx source manifest metadata = %#v", docxMetadata.SourceManifest)
	}
	if docxMetadata.ExportProfile.ID != "default-professional" || docxMetadata.ExportProfile.CitationPlacement != "inline_marker_appendix" || !docxMetadata.ExportProfile.MetadataPolicy.EmbedSourceManifest {
		t.Fatalf("docx export profile metadata = %#v", docxMetadata.ExportProfile)
	}
	docxBytes, err := decodeBase64(docxExport.ContentBase64)
	if err != nil {
		t.Fatalf("decode docx base64: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(docxBytes), int64(len(docxBytes)))
	if err != nil {
		t.Fatalf("docx is not a zip package: %v", err)
	}
	parts := map[string]string{}
	for _, file := range zr.File {
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open docx part %s: %v", file.Name, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read docx part %s: %v", file.Name, err)
		}
		parts[file.Name] = string(data)
	}
	if !strings.Contains(parts["word/document.xml"], "private legal cloud") || !strings.Contains(parts["word/document.xml"], "Last line must survive export.") {
		t.Fatalf("docx document did not preserve structured content: %s", parts["word/document.xml"])
	}
	if strings.Contains(parts["word/document.xml"], "**private legal cloud**") || strings.Contains(parts["word/document.xml"], "(source:src-export-proof)") || strings.Contains(parts["word/document.xml"], "# Export Proof") {
		t.Fatalf("docx document leaked raw markdown syntax: %s", parts["word/document.xml"])
	}
	if strings.Contains(parts["word/document.xml"], "src-export-proof") {
		t.Fatalf("docx visible document leaked internal source id: %s", parts["word/document.xml"])
	}
	if !strings.Contains(parts["word/document.xml"], "<w:b/>") || !strings.Contains(parts["word/document.xml"], "Export source proof") || !strings.Contains(parts["word/document.xml"], "[1]") {
		t.Fatalf("docx document missing format-native emphasis/source marker: %s", parts["word/document.xml"])
	}
	if !strings.Contains(parts["word/document.xml"], "<w:hyperlink") || !strings.Contains(parts["word/document.xml"], "confirms") || !strings.Contains(parts["word/document.xml"], "snapshot_ready") {
		t.Fatalf("docx document missing native source hyperlink/provenance: %s", parts["word/document.xml"])
	}
	if !strings.Contains(parts["word/_rels/document.xml.rels"], `Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink"`) || !strings.Contains(parts["word/_rels/document.xml.rels"], `Target="https://example.com/export-proof"`) || !strings.Contains(parts["word/_rels/document.xml.rels"], `TargetMode="External"`) {
		t.Fatalf("docx relationships missing external source hyperlink: %s", parts["word/_rels/document.xml.rels"])
	}
	if !strings.Contains(parts["word/styles.xml"], "Heading1") || !strings.Contains(parts["word/styles.xml"], `w:sz w:val="32"`) {
		t.Fatalf("docx styles missing default professional style definitions: %s", parts["word/styles.xml"])
	}
	if !strings.Contains(parts["docProps/custom.xml"], resp.PublicationVersionID) || !strings.Contains(parts["docProps/custom.xml"], resp.ContentHash) {
		t.Fatalf("docx custom properties missing public provenance: %s", parts["docProps/custom.xml"])
	}
	if !strings.Contains(parts["docProps/custom.xml"], "ChoirExportProfile") || !strings.Contains(parts["docProps/custom.xml"], "default-professional") {
		t.Fatalf("docx custom properties missing export profile: %s", parts["docProps/custom.xml"])
	}
	if !strings.Contains(parts["docProps/custom.xml"], "ChoirCitationPlacement") || !strings.Contains(parts["docProps/custom.xml"], "inline_marker_appendix") || !strings.Contains(parts["docProps/custom.xml"], "ChoirMetadataPolicy") {
		t.Fatalf("docx custom properties missing export profile policy: %s", parts["docProps/custom.xml"])
	}
	if !strings.Contains(parts["docProps/custom.xml"], `access_policy`) || !strings.Contains(parts["docProps/custom.xml"], `retrieval`) {
		t.Fatalf("docx custom properties missing export metadata envelope: %s", parts["docProps/custom.xml"])
	}
	if !strings.Contains(parts["customXml/item1.xml"], "choir.publication_sources.v1") || !strings.Contains(parts["customXml/item1.xml"], "src-export-proof") || !strings.Contains(parts["customXml/item1.xml"], "This source snapshot must survive rich export.") {
		t.Fatalf("docx custom XML missing source manifest: %s", parts["customXml/item1.xml"])
	}

	pdfExport, err := svc.ExportPublicationByRoute(context.Background(), resp.RoutePath, "pdf")
	if err != nil {
		t.Fatalf("ExportPublicationByRoute pdf: %v", err)
	}
	if pdfExport.Format != "pdf" || pdfExport.Content != "" || pdfExport.ContentBase64 == "" {
		t.Fatalf("pdf export shape = %#v", pdfExport)
	}
	pdfMetadata := decodePublicationExportMetadata(t, pdfExport.Metadata)
	if !pdfMetadata.PrivateMaterialOmitted || !strings.Contains(string(pdfMetadata.ExportPolicy), `"download_allowed":true`) || pdfMetadata.Retrieval.SourceID == "" {
		t.Fatalf("pdf export metadata = %#v access=%s export=%s", pdfMetadata, string(pdfMetadata.AccessPolicy), string(pdfMetadata.ExportPolicy))
	}
	if pdfMetadata.ExportProfile.ID != "default-professional" || pdfMetadata.ExportProfile.SourceDetailLevel != "labels_snapshots_manifest" {
		t.Fatalf("pdf export profile metadata = %#v", pdfMetadata.ExportProfile)
	}
	pdfBytes, err := decodeBase64(pdfExport.ContentBase64)
	if err != nil {
		t.Fatalf("decode pdf base64: %v", err)
	}
	pdfText := string(pdfBytes)
	if !strings.HasPrefix(pdfText, "%PDF-1.4") || !strings.Contains(pdfText, resp.PublicationVersionID) || !strings.Contains(pdfText, "This is the published projection with") || !strings.Contains(pdfText, "Last line must survive export.") {
		t.Fatalf("pdf content/provenance missing: %.400s", pdfText)
	}
	if strings.Contains(pdfText, "**private legal cloud**") || strings.Contains(pdfText, "(source:src-export-proof)") || strings.Contains(pdfText, "# Export Proof") {
		t.Fatalf("pdf leaked raw markdown syntax: %.800s", pdfText)
	}
	if !strings.Contains(pdfText, "private legal cloud") || !strings.Contains(pdfText, "Sources") || !strings.Contains(pdfText, "This source snapshot must survive rich export.") || !strings.Contains(pdfText, "choir.publication_sources.v1") {
		t.Fatalf("pdf missing rendered content/source manifest: %.800s", pdfText)
	}
	if !strings.Contains(pdfText, `access_policy`) || !strings.Contains(pdfText, `choir.publication_sources.v1`) || !strings.Contains(pdfText, `inline_marker_appendix`) {
		t.Fatalf("pdf embedded metadata missing policy/source manifest: %.400s", pdfText)
	}
}

func TestPublicationMarkdownExportNormalizesMalformedTableTailRows(t *testing.T) {
	store, root := openTestPlatformStore(t)
	svc := NewService(store, filepath.Join(root, "artifacts"), "")
	content := strings.Join([]string{
		"# Legal Cloud",
		"",
		"| Term | Definition |",
		"| --- | --- |",
		"| Vector database | Stores embeddings for retrieval. |",
		"",
		"| Work product | Durable, reviewable output of professional work",
		"",
		"---",
		"",
		"End of proposal.",
	}, "\n")

	resp, err := svc.PublishTexture(context.Background(), PublishTextureRequest{
		OwnerID:          "user-1",
		SourceDocID:      "doc-1",
		SourceRevisionID: "rev-1",
		Title:            "Legal Cloud",
		Content:          content,
		RequestedBy:      "user-1",
	})
	if err != nil {
		t.Fatalf("PublishTexture: %v", err)
	}

	exported, err := svc.ExportPublicationByRoute(context.Background(), resp.RoutePath, "md")
	if err != nil {
		t.Fatalf("ExportPublicationByRoute md: %v", err)
	}
	if strings.Contains(exported.Content, "| Vector database | Stores embeddings for retrieval. |\n\n| Work product |") {
		t.Fatalf("markdown export left a blank gap inside the table:\n%s", exported.Content)
	}
	if !strings.Contains(exported.Content, "| Work product | Durable, reviewable output of professional work |") {
		t.Fatalf("markdown export did not repair final table delimiter:\n%s", exported.Content)
	}

	bundle, err := svc.GetPublicationBundleByRoute(context.Background(), resp.RoutePath)
	if err != nil {
		t.Fatalf("GetPublicationBundleByRoute: %v", err)
	}
	if bundle.Artifact.Content != content {
		t.Fatalf("publication artifact content changed:\ngot  %q\nwant %q", bundle.Artifact.Content, content)
	}
}

func TestPublicationFallbackMarkdownSourceLinksRemainInert(t *testing.T) {
	inlines := parsePublicationInlines("Legacy [clip](source:src-legacy) remains prose; [web](https://example.com/context) remains a web link.")

	if len(inlines) != 3 {
		t.Fatalf("inlines = %#v, want text/link/text", inlines)
	}
	if inlines[0].Kind != "text" || !strings.Contains(inlines[0].Text, "[clip](source:src-legacy)") {
		t.Fatalf("legacy source markdown inline = %#v, want inert text", inlines[0])
	}
	if inlines[1].Kind != "link" || inlines[1].Href != "https://example.com/context" {
		t.Fatalf("web inline = %#v, want ordinary web link", inlines[1])
	}
	for _, inline := range inlines {
		if inline.Kind == "source_ref" || inline.Href == "source:src-legacy" {
			t.Fatalf("legacy source markdown became source identity or clickable source href: %#v", inlines)
		}
	}
}

func TestBuildPublicationSourceMetadataDefaultsQuotedExcerptToEmbeddedTransclusion(t *testing.T) {
	entity := testTextureSourceEntity(
		"src-quoted-excerpt",
		"source_service_item",
		"source_service_item",
		"source-item-quoted",
		"Quoted source",
		"The quoted passage is part of the argument.",
		sourcecontract.EvidenceStateBlockedByAccess,
		"source",
	)
	entity["selectors"].([]map[string]any)[0]["data"].(map[string]any)["content_hash"] = "hash-quoted-passage"
	entity["evidence"].(map[string]any)["uncertainty"] = "reader authorization required"
	sourceEntities := testTextureSourceEntities(t, entity)

	got, err := buildPublicationSourceMetadata(PublishTextureRequest{
		BodyDoc:        testTextureBodyDoc(t, "doc-quoted-excerpt", testTextureParagraph("p-quoted-excerpt", testTextureSourceRef("ref-quoted-excerpt", "src-quoted-excerpt", "Quoted source"))),
		SourceEntities: sourceEntities,
	})
	if err != nil {
		t.Fatalf("buildPublicationSourceMetadata: %v", err)
	}
	if len(got.SourceEntities) != 1 || got.SourceEntities[0].DisplayPolicy != "embedded_excerpt" {
		t.Fatalf("source entity display policy = %#v", got.SourceEntities)
	}
	if len(got.Transclusions) != 1 {
		t.Fatalf("transclusions = %d, want 1: %#v", len(got.Transclusions), got.Transclusions)
	}
	transclusion := got.Transclusions[0]
	if transclusion.DefaultDisplayMode != "embedded_excerpt" {
		t.Fatalf("default display mode = %q, want embedded_excerpt", transclusion.DefaultDisplayMode)
	}
	if transclusion.SnapshotText != "The quoted passage is part of the argument." || transclusion.ContentHash != "hash-quoted-passage" {
		t.Fatalf("transclusion snapshot/hash = %#v", transclusion)
	}
	var selector map[string]any
	if err := json.Unmarshal(transclusion.SourceSelector, &selector); err != nil {
		t.Fatalf("decode source selector: %v", err)
	}
	evidenceState := mapValue(selector["evidence_state"])
	if evidenceState["state"] != "blocked_by_access" || evidenceState["uncertainty"] != "reader authorization required" {
		t.Fatalf("selector evidence state = %#v from %s", evidenceState, string(transclusion.SourceSelector))
	}
}

func TestBuildPublicationSourceMetadataPreservesSelectorSet(t *testing.T) {
	sourceEntities := testTextureSourceEntities(t, map[string]any{
		"source_entity_id": "src-selector-set",
		"target": map[string]any{
			"kind": "source_service_item",
			"id":   "source-item-selector-set",
		},
		"selectors": []map[string]any{
			{
				"kind": "text_quote",
				"data": map[string]any{
					"text_quote":   "The quoted passage remains the inline snapshot.",
					"exact":        "The quoted passage remains the inline snapshot.",
					"content_hash": "hash-quoted-passage",
				},
			},
			{
				"kind": "table_range",
				"data": map[string]any{
					"table_id":  "appendix-a",
					"start_row": 3,
					"end_row":   7,
				},
			},
			{
				"kind": "page_range",
				"data": map[string]any{
					"start_page": 12,
					"end_page":   13,
				},
			},
		},
		"display": map[string]any{
			"mode":  "numbered_ref",
			"title": "Multi selector source",
			"label": "Multi selector source",
		},
		"evidence": map[string]any{
			"state":          "confirms",
			"open_surface":   "source",
			"relation":       "confirms",
			"research_state": "owner_supplied",
		},
		"provenance": map[string]any{
			"created_by": "platform-test",
		},
	})

	got, err := buildPublicationSourceMetadata(PublishTextureRequest{
		BodyDoc:        testTextureBodyDoc(t, "doc-selector-set", testTextureParagraph("p-selector-set", testTextureSourceRef("ref-selector-set", "src-selector-set", "Multi selector source"))),
		SourceEntities: sourceEntities,
	})
	if err != nil {
		t.Fatalf("buildPublicationSourceMetadata: %v", err)
	}
	if len(got.Transclusions) != 1 {
		t.Fatalf("transclusions = %d, want 1: %#v", len(got.Transclusions), got.Transclusions)
	}
	transclusion := got.Transclusions[0]
	if transclusion.SnapshotText != "The quoted passage remains the inline snapshot." {
		t.Fatalf("snapshot text = %q", transclusion.SnapshotText)
	}
	var selectorSet struct {
		SelectorKind  string           `json:"selector_kind"`
		Selectors     []map[string]any `json:"selectors"`
		EvidenceState map[string]any   `json:"evidence_state"`
	}
	if err := json.Unmarshal(transclusion.SourceSelector, &selectorSet); err != nil {
		t.Fatalf("decode source selector: %v", err)
	}
	if selectorSet.SelectorKind != "selector_set" || len(selectorSet.Selectors) != 3 {
		t.Fatalf("selector set = %#v from %s", selectorSet, string(transclusion.SourceSelector))
	}
	if selectorSet.Selectors[0]["selector_kind"] != "text_quote" || selectorSet.Selectors[1]["selector_kind"] != "table_range" || selectorSet.Selectors[2]["selector_kind"] != "page_range" {
		t.Fatalf("selector set lost or failed to normalize selectors: %#v", selectorSet.Selectors)
	}
	if selectorSet.EvidenceState["state"] != "confirms" || selectorSet.EvidenceState["relation"] != "confirms" || selectorSet.EvidenceState["research_state"] != "owner_supplied" {
		t.Fatalf("selector set evidence state = %#v from %s", selectorSet.EvidenceState, string(transclusion.SourceSelector))
	}
}

func TestPublicationExportPreservesCanonicalEvidenceStateMatrix(t *testing.T) {
	store, root := openTestPlatformStore(t)
	svc := NewService(store, filepath.Join(root, "artifacts"), "")

	states := []string{
		sourcecontract.EvidenceStateCandidate,
		sourcecontract.EvidenceStateAvailable,
		sourcecontract.EvidenceStateConfirms,
		sourcecontract.EvidenceStateRefutes,
		sourcecontract.EvidenceStateQualifies,
		sourcecontract.EvidenceStateNoSourceNeeded,
		sourcecontract.EvidenceStateStale,
		sourcecontract.EvidenceStateBlockedByAccess,
		sourcecontract.EvidenceStateUnavailable,
	}
	sourceEntities := make([]map[string]any, 0, len(states))
	bodyBlocks := []map[string]any{testTextureHeading("h-evidence-matrix", 1, testTextureText("Evidence state matrix"))}
	for i, state := range states {
		entityID := fmt.Sprintf("src-evidence-%s", strings.ReplaceAll(state, "_", "-"))
		quote := fmt.Sprintf("Evidence state %s survives publication.", state)
		bodyBlocks = append(bodyBlocks, testTextureParagraph(
			fmt.Sprintf("p-evidence-%d", i+1),
			testTextureText(quote+" "),
			testTextureSourceRef(fmt.Sprintf("ref-evidence-%d", i+1), entityID, fmt.Sprintf("Evidence %s source", state)),
		))
		entity := testTextureSourceEntity(
			entityID,
			"source_service_item",
			"source_service_item",
			fmt.Sprintf("source-item-%s", strings.ReplaceAll(state, "_", "-")),
			fmt.Sprintf("Evidence %s source", state),
			quote,
			state,
			"source",
		)
		entity["selectors"].([]map[string]any)[0]["data"].(map[string]any)["content_hash"] = fmt.Sprintf("hash-%s", state)
		entity["evidence"].(map[string]any)["relation"] = state
		entity["evidence"].(map[string]any)["research_state"] = fmt.Sprintf("research_%s", state)
		entity["evidence"].(map[string]any)["uncertainty"] = fmt.Sprintf("uncertainty for %s", state)
		sourceEntities = append(sourceEntities, entity)
	}
	metadata, err := json.Marshal(map[string]any{})
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}
	bodyDoc := testTextureBodyDoc(t, "doc-evidence-matrix", bodyBlocks...)
	sourceEntitiesRaw := testTextureSourceEntities(t, sourceEntities...)

	resp, err := svc.PublishTexture(context.Background(), PublishTextureRequest{
		OwnerID:          "user-1",
		SourceDocID:      "doc-evidence-matrix",
		SourceRevisionID: "rev-evidence-matrix",
		Title:            "Evidence State Matrix",
		BodyDoc:          bodyDoc,
		SourceEntities:   sourceEntitiesRaw,
		Metadata:         metadata,
		RequestedBy:      "user-1",
	})
	if err != nil {
		t.Fatalf("PublishTexture: %v", err)
	}

	bundle, err := svc.GetPublicationBundleByRoute(context.Background(), resp.RoutePath)
	if err != nil {
		t.Fatalf("GetPublicationBundleByRoute: %v", err)
	}
	if len(bundle.SourceEntities) != len(states) || len(bundle.Transclusions) != len(states) {
		t.Fatalf("bundle source metadata count entities=%d transclusions=%d want=%d", len(bundle.SourceEntities), len(bundle.Transclusions), len(states))
	}

	exported, err := svc.ExportPublicationByRoute(context.Background(), resp.RoutePath, "md")
	if err != nil {
		t.Fatalf("ExportPublicationByRoute md: %v", err)
	}
	exportedMetadata := decodePublicationExportMetadata(t, exported.Metadata)
	if len(exportedMetadata.SourceEntities) != len(states) || len(exportedMetadata.Transclusions) != len(states) {
		t.Fatalf("export source metadata count entities=%d transclusions=%d want=%d", len(exportedMetadata.SourceEntities), len(exportedMetadata.Transclusions), len(states))
	}

	bundleEvidence := evidenceStateBySourceEntityID(t, bundle.Transclusions)
	exportEvidence := evidenceStateBySourceEntityID(t, exportedMetadata.Transclusions)
	for _, state := range states {
		entityID := fmt.Sprintf("src-evidence-%s", strings.ReplaceAll(state, "_", "-"))
		assertPublishedEvidenceState(t, "bundle", state, bundleEvidence[entityID])
		assertPublishedEvidenceState(t, "export", state, exportEvidence[entityID])
	}
}

func TestPublicationExportPreservesSourceContractMatrix(t *testing.T) {
	store, root := openTestPlatformStore(t)
	svc := NewService(store, filepath.Join(root, "artifacts"), "")

	cases := []publicationSourceContractCase{
		{
			entityID:          "src-url-reader-ready",
			kind:              "web_source",
			targetKind:        "url",
			targetID:          "https://example.com/source-ready",
			target:            map[string]any{"target_kind": "url", "url": "https://example.com/source-ready"},
			rawReaderState:    "snapshot-ready",
			wantReaderState:   sourcecontract.ReaderArtifactStateReady,
			readerQuality:     "ok",
			rawSelectorKind:   "text quote",
			wantSelectorKind:  sourcecontract.SelectorKindTextQuote,
			rawOpenSurface:    "content",
			wantOpenSurface:   sourcecontract.OpenSurfaceSource,
			rawEvidenceState:  "confirmed",
			wantEvidenceState: sourcecontract.EvidenceStateConfirms,
			quote:             "URL source reader artifact is ready.",
			contentHash:       "hash-url-ready",
		},
		{
			entityID:          "src-service-bounded",
			kind:              "source_service_item",
			targetKind:        "source_service_item",
			targetID:          "source-item-bounded",
			target:            map[string]any{"target_kind": "source_service_item", "item_id": "source-item-bounded"},
			rawReaderState:    "bounded excerpt",
			wantReaderState:   sourcecontract.ReaderArtifactStateBoundedExcerptOnly,
			readerQuality:     "warning",
			rawSelectorKind:   "table-range",
			wantSelectorKind:  sourcecontract.SelectorKindTableRange,
			rawOpenSurface:    "source-viewer",
			wantOpenSurface:   sourcecontract.OpenSurfaceSource,
			rawEvidenceState:  "qualifying",
			wantEvidenceState: sourcecontract.EvidenceStateQualifies,
			quote:             "Source-service table range is bounded.",
			contentHash:       "hash-service-bounded",
		},
		{
			entityID:          "src-content-blocked",
			kind:              "content_item",
			targetKind:        "content_item",
			targetID:          "content-blocked",
			target:            map[string]any{"target_kind": "content_item", "content_id": "content-blocked"},
			rawReaderState:    "publication blocked",
			wantReaderState:   sourcecontract.ReaderArtifactStateNotPublicationSafe,
			readerQuality:     "blocked",
			rawSelectorKind:   "page range",
			wantSelectorKind:  sourcecontract.SelectorKindPageRange,
			rawOpenSurface:    "web-lens",
			wantOpenSurface:   sourcecontract.OpenSurfaceWebLens,
			rawEvidenceState:  "access blocked",
			wantEvidenceState: sourcecontract.EvidenceStateBlockedByAccess,
			quote:             "Content item reader artifact is blocked.",
			contentHash:       "hash-content-blocked",
		},
		{
			entityID:          "src-publication-texture",
			kind:              "published_texture",
			targetKind:        "publication_version",
			targetID:          "pubver-123",
			target:            map[string]any{"target_kind": "publication_version", "publication_version_id": "pubver-123"},
			rawReaderState:    "source_import_failed",
			wantReaderState:   sourcecontract.ReaderArtifactStateImportFailed,
			readerQuality:     "error",
			rawSelectorKind:   "data release vintage",
			wantSelectorKind:  sourcecontract.SelectorKindDataVintage,
			rawOpenSurface:    "publication-version",
			wantOpenSurface:   sourcecontract.OpenSurfaceTexture,
			rawEvidenceState:  "fetch_failed",
			wantEvidenceState: sourcecontract.EvidenceStateUnavailable,
			quote:             "Published Texture source import failed.",
			contentHash:       "hash-publication-texture",
		},
	}

	sourceEntities := make([]map[string]any, 0, len(cases))
	bodyBlocks := []map[string]any{testTextureHeading("h-source-contract-matrix", 1, testTextureText("Source contract matrix"))}
	for i, tc := range cases {
		title := fmt.Sprintf("Source contract case %d", i+1)
		bodyBlocks = append(bodyBlocks, testTextureParagraph(
			fmt.Sprintf("p-source-contract-%d", i+1),
			testTextureText(tc.quote+" "),
			testTextureSourceRef(fmt.Sprintf("ref-source-contract-%d", i+1), tc.entityID, title),
		))
		entity := testTextureSourceEntity(tc.entityID, tc.kind, tc.targetKind, tc.targetID, title, tc.quote, tc.wantEvidenceState, tc.wantOpenSurface)
		entity["selectors"].([]map[string]any)[0]["kind"] = tc.wantSelectorKind
		entity["selectors"].([]map[string]any)[0]["data"].(map[string]any)["content_hash"] = tc.contentHash
		entity["evidence"].(map[string]any)["relation"] = tc.wantEvidenceState
		entity["evidence"].(map[string]any)["research_state"] = fmt.Sprintf("research_%s", tc.wantEvidenceState)
		entity["evidence"].(map[string]any)["uncertainty"] = fmt.Sprintf("uncertainty for %s", tc.wantEvidenceState)
		entity["evidence"].(map[string]any)["reader_artifact_state"] = tc.wantReaderState
		entity["reader_snapshot_status"] = map[string]any{
			"state":    tc.wantReaderState,
			"quality":  tc.readerQuality,
			"warnings": []string{"preserve warning text"},
		}
		sourceEntities = append(sourceEntities, entity)
	}
	metadata, err := json.Marshal(map[string]any{})
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}
	bodyDoc := testTextureBodyDoc(t, "doc-source-contract-matrix", bodyBlocks...)
	sourceEntitiesRaw := testTextureSourceEntities(t, sourceEntities...)

	resp, err := svc.PublishTexture(context.Background(), PublishTextureRequest{
		OwnerID:          "user-1",
		SourceDocID:      "doc-source-contract-matrix",
		SourceRevisionID: "rev-source-contract-matrix",
		Title:            "Source Contract Matrix",
		BodyDoc:          bodyDoc,
		SourceEntities:   sourceEntitiesRaw,
		Metadata:         metadata,
		RequestedBy:      "user-1",
	})
	if err != nil {
		t.Fatalf("PublishTexture: %v", err)
	}

	bundle, err := svc.GetPublicationBundleByRoute(context.Background(), resp.RoutePath)
	if err != nil {
		t.Fatalf("GetPublicationBundleByRoute: %v", err)
	}
	if len(bundle.SourceEntities) != len(cases) || len(bundle.Transclusions) != len(cases) {
		t.Fatalf("bundle source metadata count entities=%d transclusions=%d want=%d", len(bundle.SourceEntities), len(bundle.Transclusions), len(cases))
	}

	exported, err := svc.ExportPublicationByRoute(context.Background(), resp.RoutePath, "md")
	if err != nil {
		t.Fatalf("ExportPublicationByRoute md: %v", err)
	}
	exportedMetadata := decodePublicationExportMetadata(t, exported.Metadata)
	if len(exportedMetadata.SourceEntities) != len(cases) || len(exportedMetadata.Transclusions) != len(cases) {
		t.Fatalf("export source metadata count entities=%d transclusions=%d want=%d", len(exportedMetadata.SourceEntities), len(exportedMetadata.Transclusions), len(cases))
	}

	bundleEntities := sourceEntitiesBySourceEntityID(bundle.SourceEntities)
	bundleTransclusions := transclusionsBySourceEntityID(bundle.Transclusions)
	exportEntities := sourceEntitiesBySourceEntityID(exportedMetadata.SourceEntities)
	exportTransclusions := transclusionsBySourceEntityID(exportedMetadata.Transclusions)
	for _, tc := range cases {
		assertPublishedSourceContractCase(t, "bundle", tc, bundleEntities[tc.entityID], bundleTransclusions[tc.entityID])
		assertPublishedSourceContractCase(t, "export", tc, exportEntities[tc.entityID], exportTransclusions[tc.entityID])
	}
}

func TestBuildPublicationSourceMetadataRejectsLegacyMetadataSourceEntities(t *testing.T) {
	metadata, _ := json.Marshal(map[string]any{
		"source_entities": []map[string]any{{
			"source_entity_id": "legacy-sidecar",
		}},
	})

	_, err := buildPublicationSourceMetadata(PublishTextureRequest{Metadata: metadata})
	if err == nil || !strings.Contains(err.Error(), "metadata.source_entities is legacy source identity") {
		t.Fatalf("buildPublicationSourceMetadata error = %v, want legacy source_entities rejection", err)
	}
}

func TestBuildPublicationSourceMetadataRejectsTopLevelSourceEntitiesWithoutBodyDoc(t *testing.T) {
	sourceEntities := testTextureSourceEntities(t, testTextureSourceEntity(
		"src-detached",
		"web_source",
		"url",
		"https://example.com/detached",
		"Detached source",
		"Detached source excerpt.",
		"available",
		"source",
	))

	_, err := buildPublicationSourceMetadata(PublishTextureRequest{SourceEntities: sourceEntities})
	if err == nil || !strings.Contains(err.Error(), "source_entities require body_doc") {
		t.Fatalf("buildPublicationSourceMetadata error = %v, want body_doc requirement", err)
	}
}

func TestBuildPublicationSourceMetadataPreservesOpenSurface(t *testing.T) {
	sourceEntities := testTextureSourceEntities(t, map[string]any{
		"source_entity_id": "src-open-surface",
		"target": map[string]any{
			"kind": "content_item",
			"id":   "content-open-surface",
		},
		"selectors": []map[string]any{{
			"kind": "whole_resource",
			"data": map[string]any{
				"content_hash": "hash-open-surface",
			},
		}},
		"display": map[string]any{
			"mode":  "numbered_ref",
			"title": "Open surface source",
		},
		"evidence": map[string]any{
			"state":        "available",
			"open_surface": "source",
		},
		"provenance": map[string]any{
			"created_by": "platform-test",
		},
	})

	got, err := buildPublicationSourceMetadata(PublishTextureRequest{
		BodyDoc:        testTextureBodyDoc(t, "doc-open-surface", testTextureParagraph("p-open-surface", testTextureSourceRef("ref-open-surface", "src-open-surface", "Open surface source"))),
		SourceEntities: sourceEntities,
	})
	if err != nil {
		t.Fatalf("buildPublicationSourceMetadata: %v", err)
	}
	if len(got.SourceEntities) != 1 {
		t.Fatalf("source entities len = %d, want 1: %#v", len(got.SourceEntities), got.SourceEntities)
	}
	entity := got.SourceEntities[0]
	if entity.OpenSurface != "source" {
		t.Fatalf("open surface = %q, want source", entity.OpenSurface)
	}
	var raw struct {
		Evidence struct {
			OpenSurface string `json:"open_surface"`
		} `json:"evidence"`
	}
	if err := json.Unmarshal(entity.EntityJSON, &raw); err != nil {
		t.Fatalf("decode entity json: %v", err)
	}
	if raw.Evidence.OpenSurface != "source" {
		t.Fatalf("entity json open surface = %q, want source from %s", raw.Evidence.OpenSurface, string(entity.EntityJSON))
	}
}

func TestPublishTextureCreatesImmutablePublicRecords(t *testing.T) {
	store, root := openTestPlatformStore(t)
	artifactsRoot := filepath.Join(root, "artifacts")
	svc := NewService(store, artifactsRoot, "")

	citations, _ := json.Marshal([]map[string]any{{
		"url":      "https://example.com/source",
		"title":    "Example source",
		"selector": map[string]any{"kind": "url"},
	}})
	metadata, _ := json.Marshal(map[string]any{
		"export_policy": map[string]any{
			"copy_allowed":     true,
			"download_allowed": true,
			"formats":          []string{"txt", "md", "html"},
		},
	})
	bodyDoc := testTextureBodyDoc(t, "doc-mission-note",
		testTextureHeading("h-mission-note", 1, testTextureText("Mission Note")),
		testTextureParagraph("p-mission-note-public", testTextureText("A public note.")),
		testTextureParagraph("p-mission-note-source",
			testTextureText("This is the published projection with "),
			testTextureSourceRef("ref-fed-rates", "src-entity-fed-rates", "Federal Reserve rate statement"),
			testTextureText("."),
		),
	)
	fedSource := testTextureSourceEntity(
		"src-entity-fed-rates",
		"official_data_release",
		"source_service_item",
		"srcitem_fed_rates",
		"Federal Reserve rate statement",
		"The committee held rates steady.",
		"confirms",
		"source",
	)
	fedSource["selectors"].([]map[string]any)[0]["data"].(map[string]any)["content_hash"] = "hash-fed-rates"
	fedSource["evidence"].(map[string]any)["relation"] = "confirms"
	fedSource["evidence"].(map[string]any)["research_state"] = "owner_supplied"
	sourceEntities := testTextureSourceEntities(t, fedSource)

	resp, err := svc.PublishTexture(context.Background(), PublishTextureRequest{
		OwnerID:          "user-1",
		SourceDocID:      "doc-1",
		SourceRevisionID: "rev-1",
		Title:            "Mission Note",
		Content:          "# Mission Note\n\nA public note.\n\nThis is the published projection with [1].",
		BodyDoc:          bodyDoc,
		SourceEntities:   sourceEntities,
		Citations:        citations,
		Metadata:         metadata,
		RequestedBy:      "user-1",
	})
	if err != nil {
		t.Fatalf("PublishTexture: %v", err)
	}
	if resp.State != "published" {
		t.Fatalf("state: got %q", resp.State)
	}
	if !strings.HasPrefix(resp.RoutePath, "/pub/texture/mission-note-") {
		t.Fatalf("route path: got %q", resp.RoutePath)
	}
	if len(resp.RetrievalSpanIDs) != 1 {
		t.Fatalf("retrieval spans: got %d, want 1", len(resp.RetrievalSpanIDs))
	}
	if len(resp.CitationIDs) < 2 {
		t.Fatalf("citation ids: got %d, want at least 2", len(resp.CitationIDs))
	}
	var privateEntityKind, privateEntityURI, activityKind, predicateType string
	// Query the object graph for the is_version_of citation edge.
	var citationEdgeCount int
	if err := svc.store.db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM og_edges WHERE kind = 'is_version_of' AND to_id = ?`, "rev-1").Scan(&citationEdgeCount); err != nil {
		t.Fatalf("query citation edge: %v", err)
	}
	if citationEdgeCount == 0 {
		t.Fatalf("is_version_of citation edge to rev-1 not found in graph")
	}
	// Query the object graph for the private provenance entity.
	if err := svc.store.db.QueryRowContext(context.Background(),
		`SELECT JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), '$.entity_kind')), JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), '$.canonical_uri')) FROM og_objects WHERE object_kind = 'choir.provenance_entity' AND content_hash = ?`,
		resp.SourceRevisionHash).Scan(&privateEntityKind, &privateEntityURI); err != nil {
		t.Fatalf("query private provenance entity: %v", err)
	}
	if privateEntityKind != "private_texture_revision" || privateEntityURI != "choir-private:texture/doc-1/revisions/rev-1" {
		t.Fatalf("private provenance entity = (%q, %q), want Texture revision ref", privateEntityKind, privateEntityURI)
	}
	// Query the object graph for the provenance activity.
	if err := svc.store.db.QueryRowContext(context.Background(),
		`SELECT JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), '$.activity_kind')) FROM og_objects WHERE object_kind = 'choir.provenance_activity' AND CAST(metadata AS JSON) LIKE ?`,
		"%"+resp.RoutePath+"%").Scan(&activityKind); err != nil {
		t.Fatalf("query provenance activity: %v", err)
	}
	if activityKind != "publish_texture_revision" {
		t.Fatalf("activity kind = %q, want publish_texture_revision", activityKind)
	}
	// Query the object graph for the verifier attestation.
	if err := svc.store.db.QueryRowContext(context.Background(),
		`SELECT JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), '$.predicate_type')) FROM og_objects WHERE object_kind = 'choir.verifier_attestation' AND JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), '$.target_id')) = ?`,
		resp.PublicationVersionID).Scan(&predicateType); err != nil {
		t.Fatalf("query verifier attestation: %v", err)
	}
	if predicateType != "choir.platform.publish_texture.v0" {
		t.Fatalf("predicate type = %q, want choir.platform.publish_texture.v0", predicateType)
	}

	bundle, err := svc.GetPublicationBundleByRoute(context.Background(), resp.RoutePath)
	if err != nil {
		t.Fatalf("GetPublicationBundleByRoute: %v", err)
	}
	trailingSlashBundle, err := svc.GetPublicationBundleByRoute(context.Background(), resp.RoutePath+"/")
	if err != nil {
		t.Fatalf("GetPublicationBundleByRoute trailing slash: %v", err)
	}
	if trailingSlashBundle.Route.Path != resp.RoutePath {
		t.Fatalf("trailing slash route normalized to %q, want %q", trailingSlashBundle.Route.Path, resp.RoutePath)
	}
	if bundle.Artifact.Content != "# Mission Note\n\nA public note.\n\nThis is the published projection with [1]." {
		t.Fatalf("bundle content mismatch: %q", bundle.Artifact.Content)
	}
	if bundle.Citations[0].ToKind == "private_texture_revision" || bundle.Citations[0].ToID == "rev-1" {
		t.Fatalf("bundle leaked private revision citation: %#v", bundle.Citations[0])
	}
	if len(bundle.Artifact.RenderModel) == 0 || bundle.Artifact.RenderModel[0].SpanID == "" {
		t.Fatalf("bundle render model missing retrieval span refs: %#v", bundle.Artifact.RenderModel)
	}
	if len(bundle.SourceEntities) != 1 {
		t.Fatalf("bundle source entities = %d, want 1: %#v", len(bundle.SourceEntities), bundle.SourceEntities)
	}
	sourceEntity := bundle.SourceEntities[0]
	if sourceEntity.SourceEntityID != "src-entity-fed-rates" || sourceEntity.TargetKind != "source_service_item" || sourceEntity.TargetID != "srcitem_fed_rates" {
		t.Fatalf("bundle source entity identity = %#v", sourceEntity)
	}
	if sourceEntity.DisplayPolicy != "embedded_excerpt" || sourceEntity.OpenSurface != "source" {
		t.Fatalf("bundle source entity display = %#v", sourceEntity)
	}
	if len(bundle.Transclusions) != 1 {
		t.Fatalf("bundle transclusions = %d, want 1: %#v", len(bundle.Transclusions), bundle.Transclusions)
	}
	transclusion := bundle.Transclusions[0]
	if transclusion.SourceEntityID != "src-entity-fed-rates" || transclusion.DefaultDisplayMode != "embedded_excerpt" {
		t.Fatalf("bundle transclusion identity/display = %#v", transclusion)
	}
	if transclusion.SnapshotText != "The committee held rates steady." || transclusion.ContentHash != "hash-fed-rates" {
		t.Fatalf("bundle transclusion snapshot/hash = %#v", transclusion)
	}
	var bundleSelector struct {
		EvidenceState map[string]any `json:"evidence_state"`
	}
	if err := json.Unmarshal(transclusion.SourceSelector, &bundleSelector); err != nil {
		t.Fatalf("decode bundle source selector: %v", err)
	}
	if bundleSelector.EvidenceState["state"] != "confirms" || bundleSelector.EvidenceState["relation"] != "confirms" || bundleSelector.EvidenceState["research_state"] != "owner_supplied" {
		t.Fatalf("bundle selector evidence state = %#v from %s", bundleSelector.EvidenceState, string(transclusion.SourceSelector))
	}
	if !strings.Contains(string(bundle.Policy.Export), `"download_allowed":true`) {
		t.Fatalf("bundle export policy = %s", string(bundle.Policy.Export))
	}
	exported, err := svc.ExportPublicationByRoute(context.Background(), resp.RoutePath, "html")
	if err != nil {
		t.Fatalf("ExportPublicationByRoute: %v", err)
	}
	if exported.Format != "html" || exported.MediaType != "text/html; charset=utf-8" || !strings.HasSuffix(exported.Filename, ".html") {
		t.Fatalf("export metadata = %#v", exported)
	}
	if !strings.Contains(exported.Content, "This is the published projection with") || exported.ContentHash == "" {
		t.Fatalf("export content/hash = %#v", exported)
	}
	if !strings.Contains(exported.Content, "<h1>Mission Note</h1>") || !strings.Contains(exported.Content, `<a class="texture-source-ref"`) || !strings.Contains(exported.Content, `id="choir-source-manifest"`) {
		t.Fatalf("html export missing semantic document/source manifest: %s", exported.Content)
	}
	if !strings.Contains(exported.Content, `choir-export-profile" content="default-professional"`) || !strings.Contains(exported.Content, `.texture-table`) {
		t.Fatalf("html export missing default professional profile: %s", exported.Content)
	}
	for _, textureClass := range []string{
		`class="texture-publication"`,
		`class="texture-source-ref"`,
		`class="texture-sources"`,
		`id="texture-sources-heading"`,
		`.texture-publication`,
		`.texture-source-ref`,
		`.texture-table`,
		`.texture-sources`,
	} {
		if !strings.Contains(exported.Content, textureClass) {
			t.Fatalf("html export missing Texture class/id %q: %s", textureClass, exported.Content)
		}
	}
	if !strings.Contains(exported.Content, `id="choir-export-profile"`) || !strings.Contains(exported.Content, `"citation_placement": "inline_marker_appendix"`) || !strings.Contains(exported.Content, `"metadata_policy"`) {
		t.Fatalf("html export missing embedded export profile contract: %s", exported.Content)
	}
	if strings.Contains(exported.Content, "# Legal Cloud") || strings.Contains(exported.Content, "**") || strings.Contains(exported.Content, "(source:src-entity-fed-rates)") {
		t.Fatalf("html export leaked raw markdown syntax: %s", exported.Content)
	}
	if !strings.Contains(exported.Content, "choir.publication_sources.v1") || !strings.Contains(exported.Content, "src-entity-fed-rates") || !strings.Contains(exported.Content, "The committee held rates steady.") {
		t.Fatalf("html export missing embedded source metadata: %s", exported.Content)
	}
	exportedMetadata := decodePublicationExportMetadata(t, exported.Metadata)
	if len(exportedMetadata.SourceEntities) != 1 || len(exportedMetadata.Transclusions) != 1 {
		t.Fatalf("export source metadata = %#v from %s", exportedMetadata, string(exported.Metadata))
	}
	if exportedMetadata.SourceManifest.Schema != "choir.publication_sources.v1" || len(exportedMetadata.SourceManifest.Sources) != 1 {
		t.Fatalf("export source manifest = %#v from %s", exportedMetadata.SourceManifest, string(exported.Metadata))
	}
	if exportedMetadata.ExportProfile.ID != "default-professional" || exportedMetadata.ExportProfile.Typography.MaxWidthPX == 0 || exportedMetadata.ExportProfile.CitationPlacement != "inline_marker_appendix" {
		t.Fatalf("html export profile metadata = %#v from %s", exportedMetadata.ExportProfile, string(exported.Metadata))
	}
	if !strings.Contains(string(exportedMetadata.AccessPolicy), `"visibility":"public"`) || !strings.Contains(string(exportedMetadata.ExportPolicy), `"download_allowed":true`) {
		t.Fatalf("export policy metadata access=%s export=%s", string(exportedMetadata.AccessPolicy), string(exportedMetadata.ExportPolicy))
	}
	if exportedMetadata.Retrieval.SourceID == "" || len(exportedMetadata.Retrieval.Spans) != 1 || exportedMetadata.Retrieval.Spans[0].ID == "" {
		t.Fatalf("export retrieval metadata = %#v", exportedMetadata.Retrieval)
	}
	var exportedSelector struct {
		EvidenceState map[string]any `json:"evidence_state"`
	}
	if err := json.Unmarshal(exportedMetadata.Transclusions[0].SourceSelector, &exportedSelector); err != nil {
		t.Fatalf("decode export source selector: %v", err)
	}
	if exportedSelector.EvidenceState["state"] != "confirms" || exportedSelector.EvidenceState["relation"] != "confirms" {
		t.Fatalf("export selector evidence state = %#v from %s", exportedSelector.EvidenceState, string(exportedMetadata.Transclusions[0].SourceSelector))
	}
	search, err := svc.SearchPublished(context.Background(), "projection")
	if err != nil {
		t.Fatalf("SearchPublished: %v", err)
	}
	if len(search.Results) != 1 || search.Results[0].SpanID == "" {
		t.Fatalf("search results: %#v", search.Results)
	}
	legacyRoutePath := "/pub/texture/legacy-mission-note-" + shortID(resp.PublicationID)
	now := time.Now().UTC()
	// Insert legacy route as a graph object.
	ogStore := NewObjectGraphStore(store)
	legacyRouteSuffix := objectgraph.StableSuffixFromKey("route-legacy-" + shortID(resp.PublicationVersionID))
	legacyRouteID, _ := objectgraph.BuildCanonicalID("choir.public_route", "owner-1", legacyRouteSuffix)
	if err := ogStore.PutObject(context.Background(), objectgraph.Object{
		CanonicalID: legacyRouteID,
		ObjectKind:  "choir.public_route",
		OwnerID:     "owner-1",
		VersionID:   resp.PublicationVersionID,
		ContentHash: objectgraph.ContentHash("choir.public_route", nil, mustJSONRaw(map[string]any{
			"route_path":        legacyRoutePath,
			"target_kind":       "publication",
			"target_id":         resp.PublicationID,
			"target_version_id": resp.PublicationVersionID,
			"state":             "active",
		})),
		Metadata: mustJSONRaw(map[string]any{
			"route_path":        legacyRoutePath,
			"target_kind":       "publication",
			"target_id":         resp.PublicationID,
			"target_version_id": resp.PublicationVersionID,
			"state":             "active",
		}),
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("insert legacy public route: %v", err)
	}
	legacyBundle, err := svc.GetPublicationBundleByRoute(context.Background(), legacyRoutePath+"/")
	if err != nil {
		t.Fatalf("GetPublicationBundleByRoute legacy texture route: %v", err)
	}
	if legacyBundle.Route.Path != legacyRoutePath {
		t.Fatalf("legacy route normalized to %q, want %q", legacyBundle.Route.Path, legacyRoutePath)
	}
	proposal, err := svc.SubmitPublicationProposal(context.Background(), SubmitPublicationProposalRequest{
		PublicationID:        resp.PublicationID,
		PublicationVersionID: resp.PublicationVersionID,
		SubmitterID:          "reader-1",
		SubmitterDocID:       "reader-doc-1",
		SubmitterRevisionID:  "reader-rev-1",
		Title:                "Reader proposal",
		Content:              "A reader derivative with transcluded source.",
		Transclusions: []TransclusionRef{{
			SourceKind:           "published_texture_span",
			PublicationID:        resp.PublicationID,
			PublicationVersionID: resp.PublicationVersionID,
			SpanID:               resp.RetrievalSpanIDs[0],
			ContentHash:          resp.ContentHash,
			SnapshotText:         "A public note.",
		}},
		RequestedBy: "reader-1",
	})
	if err != nil {
		t.Fatalf("SubmitPublicationProposal: %v", err)
	}
	if proposal.State != "proposed" || proposal.DeliveryState != "recorded_for_author" || len(proposal.TransclusionIDs) != 1 {
		t.Fatalf("proposal response: %#v", proposal)
	}
	delivery, err := svc.UpdateProposalDeliveryState(context.Background(), UpdateProposalDeliveryStateRequest{
		ProposalID:    proposal.ProposalID,
		DeliveryID:    proposal.DeliveryID,
		DeliveryState: "delivered",
		DeliveryRef:   "author-runtime:delivered",
		RecordedBy:    "proxy",
	})
	if err != nil {
		t.Fatalf("UpdateProposalDeliveryState: %v", err)
	}
	if delivery.DeliveryState != "delivered" {
		t.Fatalf("delivery state response: %#v", delivery)
	}
	var persistedDeliveryState string
	if err := store.db.QueryRow(`SELECT JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), '$.delivery_state')) FROM og_objects WHERE object_kind = 'choir.publication_proposal' AND JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), '$.delivery_id')) = ?`, proposal.DeliveryID).Scan(&persistedDeliveryState); err != nil {
		t.Fatalf("query delivery state: %v", err)
	}
	if persistedDeliveryState != "delivered" {
		t.Fatalf("persisted delivery state = %q, want delivered", persistedDeliveryState)
	}

	var storageRef string
	if err := store.db.QueryRow(`SELECT JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), '$.storage_ref')) FROM og_objects WHERE object_kind = 'choir.artifact_blob' AND JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), '$.artifact_manifest_id')) = ?`, resp.ArtifactManifestID).Scan(&storageRef); err != nil {
		t.Fatalf("artifact blob query: %v", err)
	}
	if filepath.IsAbs(storageRef) || !strings.HasPrefix(storageRef, "sha256/") {
		t.Fatalf("storage ref leaked absolute/private path: %q", storageRef)
	}
	if _, err := os.Stat(filepath.Join(artifactsRoot, storageRef)); err != nil {
		t.Fatalf("artifact blob missing: %v", err)
	}

	// Count objects by kind in the object graph.
	for kind, want := range map[string]int{
		"choir.publication":             1,
		"choir.publication_version":     1,
		"choir.publication_proposal":    2, // 1 from publish + 1 reader proposal
		"choir.public_route":            2, // 1 from publish + 1 legacy route inserted by test
		"choir.artifact_manifest":       2, // 1 publish + 1 proposal
		"choir.artifact_blob":           2, // 1 publish + 1 proposal
		"choir.retrieval_source":        1,
		"choir.retrieval_span":          1,
		"choir.retrieval_manifest":      1,
		"choir.publication_source_entity": 1,
		"choir.publication_transclusion":  1,
		"choir.publication_policy":        1,
		"choir.consent_record":            1,
		"choir.review_record":             1,
		"choir.verifier_attestation":      2,
		"choir.provenance_entity":         4,
		"choir.provenance_agent":          2,
		"choir.provenance_activity":       2,
		"choir.subject":                   2, // owner + submitter
	} {
		var got int
		if err := store.db.QueryRow(`SELECT COUNT(*) FROM og_objects WHERE object_kind = ?`, kind).Scan(&got); err != nil {
			t.Fatalf("count %s: %v", kind, err)
		}
		if got != want {
			t.Fatalf("count %s: got %d, want %d", kind, got, want)
		}
	}
	// Count provenance edges (was_derived_from).
	var provEdgeCount int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM og_edges WHERE kind = 'was_derived_from'`).Scan(&provEdgeCount); err != nil {
		t.Fatalf("count was_derived_from edges: %v", err)
	}
	if provEdgeCount != 2 {
		t.Fatalf("was_derived_from edge count: got %d, want 2", provEdgeCount)
	}
	// Count citation edges (is_version_of + references).
	var citationsCount int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM og_edges WHERE kind = 'is_version_of' OR kind = 'references'`).Scan(&citationsCount); err != nil {
		t.Fatalf("count citation edges: %v", err)
	}
	if citationsCount < 2 {
		t.Fatalf("citation edge count: got %d, want at least 2", citationsCount)
	}
}

func TestPublishTextureStructuredBodyDrivesPublicationSources(t *testing.T) {
	store, root := openTestPlatformStore(t)
	artifactsRoot := filepath.Join(root, "artifacts")
	svc := NewService(store, artifactsRoot, "")

	bodyDoc := json.RawMessage(`{
		"schema":"choir.texture_doc.v1",
		"doc":{
			"type":"doc",
			"attrs":{"id":"doc-structured-pub"},
			"content":[{
				"type":"paragraph",
				"attrs":{"id":"p-structured-pub"},
				"content":[
					{"type":"text","text":"This structured publication cites "},
					{"type":"source_ref","attrs":{"id":"ref-fed","source_entity_id":"src-structured-fed","display_mode":"numbered_ref"}},
					{"type":"text","text":"."}
				]
			}]
		}
	}`)
	sourceEntities := json.RawMessage(`[
		{
			"source_entity_id":"src-structured-fed",
			"target":{"kind":"source_service_item","id":"srcitem_fed_rates"},
			"selectors":[{"kind":"text_quote","data":{"text_quote":"The committee held rates steady.","content_hash":"hash-fed-rates"}}],
			"display":{"mode":"numbered_ref","title":"Federal Reserve statement"},
			"evidence":{"state":"confirms","open_surface":"source"},
			"provenance":{"created_by":"texture","source_system":"test"}
		}
	]`)
	metadata, _ := json.Marshal(map[string]any{
		"export_policy": map[string]any{
			"copy_allowed":     true,
			"download_allowed": true,
			"formats":          []string{"txt", "md", "html"},
		},
	})

	resp, err := svc.PublishTexture(context.Background(), PublishTextureRequest{
		OwnerID:          "user-1",
		SourceDocID:      "doc-structured",
		SourceRevisionID: "rev-structured",
		Title:            "Structured Publication",
		BodyDoc:          bodyDoc,
		SourceEntities:   sourceEntities,
		Metadata:         metadata,
		RequestedBy:      "user-1",
		History: []PublishTextureRevision{{
			RevisionID:     "rev-structured",
			Content:        "This structured publication cites [1].",
			BodyDoc:        bodyDoc,
			SourceEntities: sourceEntities,
			RevisionHash:   "revhash-structured",
		}},
	})
	if err != nil {
		t.Fatalf("PublishTexture: %v", err)
	}
	bundle, err := svc.GetPublicationBundleByRoute(context.Background(), resp.RoutePath)
	if err != nil {
		t.Fatalf("GetPublicationBundleByRoute: %v", err)
	}
	if bundle.Artifact.Content != "This structured publication cites [1]." || strings.Contains(bundle.Artifact.Content, "(source:") {
		t.Fatalf("artifact content = %q, want structured projection without markdown source links", bundle.Artifact.Content)
	}
	if len(bundle.Artifact.BodyDoc) == 0 || len(bundle.Artifact.SourceEntities) == 0 {
		t.Fatalf("bundle missing structured artifact fields: body_doc=%s source_entities=%s", bundle.Artifact.BodyDoc, bundle.Artifact.SourceEntities)
	}
	if len(bundle.SourceEntities) != 1 || bundle.SourceEntities[0].SourceEntityID != "src-structured-fed" {
		t.Fatalf("bundle source entities = %#v", bundle.SourceEntities)
	}
	doc := buildPublicationDocument(bundle)
	if len(doc.Blocks) != 1 || len(doc.Blocks[0].Inlines) != 3 || doc.Blocks[0].Inlines[1].Kind != "source_ref" {
		t.Fatalf("publication doc did not preserve structured source_ref inline: %#v", doc.Blocks)
	}
	if doc.Blocks[0].Inlines[1].Text != "Federal Reserve statement" {
		t.Fatalf("unlabeled source_ref text = %q, want source entity title", doc.Blocks[0].Inlines[1].Text)
	}
	htmlExport, err := svc.ExportPublicationByRoute(context.Background(), resp.RoutePath, "html")
	if err != nil {
		t.Fatalf("ExportPublicationByRoute html: %v", err)
	}
	if !strings.Contains(htmlExport.Content, `data-source-id="src-structured-fed"`) || strings.Contains(htmlExport.Content, "(source:src-structured-fed)") {
		t.Fatalf("html export did not use structured source ref rendering: %s", htmlExport.Content)
	}
	if bundle.VersionHistory == nil || len(bundle.VersionHistory.Revisions) != 1 ||
		len(bundle.VersionHistory.Revisions[0].BodyDoc) == 0 ||
		len(bundle.VersionHistory.Revisions[0].SourceEntities) == 0 {
		t.Fatalf("version history missing structured fields: %#v", bundle.VersionHistory)
	}
}

func TestPublicationURLTargetDefaultsToWebSourceKind(t *testing.T) {
	entity, transclusion, ok, err := normalizePublicationSourceEntity(map[string]any{
		"source_entity_id": "src-public-url",
		"target": map[string]any{
			"kind": "url",
			"uri":  "https://example.com/",
		},
		"selectors": []map[string]any{{
			"kind": "text_quote",
			"data": map[string]any{"text_quote": "Example source quote."},
		}},
		"display": map[string]any{
			"mode":  "numbered_ref",
			"title": "Example source",
		},
		"evidence": map[string]any{
			"state":        "confirms",
			"open_surface": "source",
		},
		"provenance": map[string]any{
			"created_by": "texture",
		},
	})
	if err != nil {
		t.Fatalf("normalizePublicationSourceEntity: %v", err)
	}
	if !ok {
		t.Fatal("normalizePublicationSourceEntity ok=false")
	}
	if entity.Kind != "web_source" || entity.TargetKind != "url" || entity.TargetID != "https://example.com/" {
		t.Fatalf("entity kind/target = %s %s %s, want web_source url https://example.com/", entity.Kind, entity.TargetKind, entity.TargetID)
	}
	if transclusion.SnapshotText != "Example source quote." {
		t.Fatalf("transclusion snapshot = %q", transclusion.SnapshotText)
	}
}

func TestPublishTextureRejectsSourceEntitiesWithoutBodyDoc(t *testing.T) {
	store, root := openTestPlatformStore(t)
	svc := NewService(store, filepath.Join(root, "artifacts"), "")
	sourceEntities := testTextureSourceEntities(t, testTextureSourceEntity(
		"src-detached-publish",
		"web_source",
		"url",
		"https://example.com/detached",
		"Detached source",
		"Detached source excerpt.",
		"available",
		"source",
	))

	resp, err := svc.PublishTexture(context.Background(), PublishTextureRequest{
		OwnerID:          "user-1",
		SourceDocID:      "doc-detached",
		SourceRevisionID: "rev-detached",
		Title:            "Detached Source",
		Content:          "This content has no structured source node.",
		SourceEntities:   sourceEntities,
		RequestedBy:      "user-1",
	})
	if err == nil || !strings.Contains(err.Error(), "source_entities require body_doc") {
		t.Fatalf("PublishTexture response=%#v error=%v, want source_entities/body_doc rejection", resp, err)
	}
}

func TestPublishTextureRejectsHistorySourceEntitiesWithoutBodyDoc(t *testing.T) {
	store, root := openTestPlatformStore(t)
	svc := NewService(store, filepath.Join(root, "artifacts"), "")
	sourceEntities := testTextureSourceEntities(t, testTextureSourceEntity(
		"src-detached-history",
		"web_source",
		"url",
		"https://example.com/detached-history",
		"Detached history source",
		"Detached history source excerpt.",
		"available",
		"source",
	))

	resp, err := svc.PublishTexture(context.Background(), PublishTextureRequest{
		OwnerID:          "user-1",
		SourceDocID:      "doc-detached-history",
		SourceRevisionID: "rev-detached-history-head",
		Title:            "Detached History Source",
		Content:          "Head content has no structured source identity.",
		RequestedBy:      "user-1",
		History: []PublishTextureRevision{{
			RevisionID:     "rev-detached-history",
			VersionNumber:  1,
			Content:        "History content has detached source identity.",
			SourceEntities: sourceEntities,
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "history revision rev-detached-history structured fields are invalid") || !strings.Contains(err.Error(), "source_entities require body_doc") {
		t.Fatalf("PublishTexture response=%#v error=%v, want history source_entities/body_doc rejection", resp, err)
	}
}

func TestPublishTextureRejectsLegacyMetadataSourceEntities(t *testing.T) {
	store, root := openTestPlatformStore(t)
	svc := NewService(store, filepath.Join(root, "artifacts"), "")

	bodyDoc := json.RawMessage(`{
		"schema":"choir.texture_doc.v1",
		"doc":{
			"type":"doc",
			"attrs":{"id":"doc-empty-sources"},
			"content":[{
				"type":"paragraph",
				"attrs":{"id":"p-empty-sources"},
				"content":[{"type":"text","text":"No structured citations here."}]
			}]
		}
	}`)
	metadata, _ := json.Marshal(map[string]any{
		"source_entities": []map[string]any{{
			"source_entity_id": "legacy-stale-source",
			"target_kind":      "url",
			"target_id":        "https://example.com/stale",
			"title":            "Stale legacy source",
		}},
	})

	resp, err := svc.PublishTexture(context.Background(), PublishTextureRequest{
		OwnerID:          "user-1",
		SourceDocID:      "doc-empty-sources",
		SourceRevisionID: "rev-empty-sources",
		Title:            "Structured Empty Sources",
		BodyDoc:          bodyDoc,
		SourceEntities:   json.RawMessage(`[]`),
		Metadata:         metadata,
		RequestedBy:      "user-1",
	})
	if err == nil || !strings.Contains(err.Error(), "metadata.source_entities is legacy source identity") {
		t.Fatalf("PublishTexture response=%#v error=%v, want legacy metadata rejection", resp, err)
	}
}

func TestPublicationPublicSurfacesEnforceVisibilityPolicy(t *testing.T) {
	store, root := openTestPlatformStore(t)
	svc := NewService(store, filepath.Join(root, "artifacts"), "")

	publishWithVisibility := func(t *testing.T, visibility string) *PublishTextureResponse {
		t.Helper()
		accessPolicy, _ := json.Marshal(map[string]any{
			"visibility": visibility,
			"route":      "public",
		})
		resp, err := svc.PublishTexture(context.Background(), PublishTextureRequest{
			OwnerID:          "user-1",
			SourceDocID:      "doc-" + visibility,
			SourceRevisionID: "rev-" + visibility,
			Title:            "Visibility " + visibility,
			Content:          fmt.Sprintf("Visibility policy proof %s unique-token-%s", visibility, visibility),
			AccessPolicy:     accessPolicy,
			RequestedBy:      "user-1",
		})
		if err != nil {
			t.Fatalf("PublishTexture %s: %v", visibility, err)
		}
		return resp
	}

	publicResp := publishWithVisibility(t, "public")
	unlistedResp := publishWithVisibility(t, "unlisted")
	privateResp := publishWithVisibility(t, "private")

	publicBundle, err := svc.GetPublicationBundleByRoute(context.Background(), publicResp.RoutePath)
	if err != nil {
		t.Fatalf("GetPublicationBundleByRoute public: %v", err)
	}
	if !strings.Contains(publicBundle.Artifact.Content, "unique-token-public") {
		t.Fatalf("public bundle content = %q", publicBundle.Artifact.Content)
	}
	if _, err := svc.GetPublicationBundleByRoute(context.Background(), unlistedResp.RoutePath); err != sql.ErrNoRows {
		t.Fatalf("unlisted public resolve err = %v, want sql.ErrNoRows", err)
	}
	if _, err := svc.GetPublicationBundleByRoute(context.Background(), privateResp.RoutePath); err != sql.ErrNoRows {
		t.Fatalf("private public resolve err = %v, want sql.ErrNoRows", err)
	}

	publicExport, err := svc.ExportPublicationByRoute(context.Background(), publicResp.RoutePath, "md")
	if err != nil {
		t.Fatalf("ExportPublicationByRoute public: %v", err)
	}
	if !strings.Contains(publicExport.Content, "unique-token-public") {
		t.Fatalf("public export content = %q", publicExport.Content)
	}
	if _, err := svc.ExportPublicationByRoute(context.Background(), unlistedResp.RoutePath, "md"); err != sql.ErrNoRows {
		t.Fatalf("unlisted public export err = %v, want sql.ErrNoRows", err)
	}
	if _, err := svc.ExportPublicationByRoute(context.Background(), privateResp.RoutePath, "md"); err != sql.ErrNoRows {
		t.Fatalf("private public export err = %v, want sql.ErrNoRows", err)
	}

	publicSearch, err := svc.SearchPublished(context.Background(), "unique-token-public")
	if err != nil {
		t.Fatalf("SearchPublished public: %v", err)
	}
	if len(publicSearch.Results) != 1 || publicSearch.Results[0].RoutePath != publicResp.RoutePath {
		t.Fatalf("public search results = %#v, want route %s", publicSearch.Results, publicResp.RoutePath)
	}
	unlistedSearch, err := svc.SearchPublished(context.Background(), "unique-token-unlisted")
	if err != nil {
		t.Fatalf("SearchPublished unlisted: %v", err)
	}
	if len(unlistedSearch.Results) != 0 {
		t.Fatalf("unlisted search leaked results: %#v", unlistedSearch.Results)
	}
	privateSearch, err := svc.SearchPublished(context.Background(), "unique-token-private")
	if err != nil {
		t.Fatalf("SearchPublished private: %v", err)
	}
	if len(privateSearch.Results) != 0 {
		t.Fatalf("private search leaked results: %#v", privateSearch.Results)
	}
}
