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
	if status["quality"] != tc.readerQuality {
		t.Fatalf("%s reader snapshot quality = %#v, want %q from %s", surface, status["quality"], tc.readerQuality, string(entity.Entity))
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

func TestSyncVTextDocumentPersistsDocumentAndRevisions(t *testing.T) {
	store, root := openTestPlatformStore(t)
	svc := NewService(store, filepath.Join(root, "artifacts"))

	createdAt := time.Date(2026, time.June, 10, 11, 0, 0, 0, time.UTC)
	req := SyncVTextDocumentRequest{
		DocID:   "doc-123",
		OwnerID: "user-1",
		Title:   "Platform Draft",
		Revisions: []SyncVTextRevision{
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
				Citations:        json.RawMessage(`[{"kind":"url","value":"https://example.com"}]`),
				Metadata:         json.RawMessage(`{"source":"test"}`),
				CreatedAt:        createdAt.Add(time.Minute),
			},
		},
	}

	resp, err := svc.SyncVTextDocument(context.Background(), req)
	if err != nil {
		t.Fatalf("SyncVTextDocument: %v", err)
	}
	if resp.DocID != req.DocID {
		t.Fatalf("doc id: got %q want %q", resp.DocID, req.DocID)
	}
	if resp.RevisionCount != len(req.Revisions) {
		t.Fatalf("revision count: got %d want %d", resp.RevisionCount, len(req.Revisions))
	}

	doc, err := svc.GetPlatformVTextDocument(context.Background(), req.DocID)
	if err != nil {
		t.Fatalf("GetPlatformVTextDocument: %v", err)
	}
	if doc.OwnerID != req.OwnerID || doc.Title != req.Title {
		t.Fatalf("document mismatch: %#v", doc)
	}

	revisions, err := svc.ListPlatformVTextRevisions(context.Background(), req.DocID)
	if err != nil {
		t.Fatalf("ListPlatformVTextRevisions: %v", err)
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

	rev, err := svc.GetPlatformVTextRevision(context.Background(), "rev-2")
	if err != nil {
		t.Fatalf("GetPlatformVTextRevision: %v", err)
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
}

func TestPublicationExportDocxAndPDFUseCanonicalPublicationBytes(t *testing.T) {
	store, root := openTestPlatformStore(t)
	svc := NewService(store, filepath.Join(root, "artifacts"))
	metadata, err := json.Marshal(map[string]any{
		"source_entities": []map[string]any{{
			"entity_id": "src-export-proof",
			"kind":      "web_source",
			"label":     "Export source proof",
			"target": map[string]any{
				"target_kind": "url",
				"url":         "https://example.com/export-proof",
			},
			"selectors": []map[string]any{{
				"selector_kind": "text_quote",
				"text_quote":    "This source snapshot must survive rich export.",
				"content_hash":  "hash-export-proof",
			}},
			"display": map[string]any{
				"inline_mode":           "embedded_excerpt",
				"open_surface":          "source_viewer",
				"reader_artifact_state": "snapshot_ready",
			},
			"evidence": map[string]any{
				"state":          "confirms",
				"relation":       "confirms",
				"research_state": "owner_supplied",
			},
		}},
	})
	if err != nil {
		t.Fatalf("marshal source metadata: %v", err)
	}

	resp, err := svc.PublishVText(context.Background(), PublishVTextRequest{
		OwnerID:          "user-1",
		SourceDocID:      "doc-1",
		SourceRevisionID: "rev-1",
		Title:            "Export Proof",
		Content:          "# Export Proof\n\n| Term | Definition |\n| --- | --- |\n| VText | Canonical **artifact**. |\n\nThis is the published projection with [Export source proof](source:src-export-proof).\n\nA **private legal cloud** survives rich export without Markdown syntax.\n\n" + strings.Repeat("Long document proof line with enough content to require PDF pagination.\n", 80) + "\nLast line must survive export.",
		Metadata:         metadata,
		RequestedBy:      "user-1",
	})
	if err != nil {
		t.Fatalf("PublishVText: %v", err)
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
	if !strings.Contains(parts["word/document.xml"], "Canonical ") || !strings.Contains(parts["word/document.xml"], "artifact") || !strings.Contains(parts["word/document.xml"], "<w:tbl>") {
		t.Fatalf("docx document did not preserve content/table: %s", parts["word/document.xml"])
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
	svc := NewService(store, filepath.Join(root, "artifacts"))
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

	resp, err := svc.PublishVText(context.Background(), PublishVTextRequest{
		OwnerID:          "user-1",
		SourceDocID:      "doc-1",
		SourceRevisionID: "rev-1",
		Title:            "Legal Cloud",
		Content:          content,
		RequestedBy:      "user-1",
	})
	if err != nil {
		t.Fatalf("PublishVText: %v", err)
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

func TestBuildPublicationSourceMetadataDefaultsQuotedExcerptToEmbeddedTransclusion(t *testing.T) {
	metadata, _ := json.Marshal(map[string]any{
		"source_entities": []map[string]any{{
			"entity_id": "src-quoted-excerpt",
			"kind":      "source_service_item",
			"label":     "Quoted source",
			"target": map[string]any{
				"target_kind": "source_service_item",
				"item_id":     "source-item-quoted",
			},
			"selectors": []map[string]any{{
				"selector_kind": "text_quote",
				"text_quote":    "The quoted passage is part of the argument.",
				"content_hash":  "hash-quoted-passage",
			}},
			"evidence": map[string]any{
				"state":       "blocked",
				"uncertainty": "reader authorization required",
			},
		}},
	})

	got, err := buildPublicationSourceMetadata(PublishVTextRequest{Metadata: metadata})
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
	metadata, _ := json.Marshal(map[string]any{
		"source_entities": []map[string]any{{
			"entity_id": "src-selector-set",
			"kind":      "source_service_item",
			"label":     "Multi selector source",
			"target": map[string]any{
				"target_kind": "source_service_item",
				"item_id":     "source-item-selector-set",
			},
			"selectors": []map[string]any{
				{
					"selector_kind": "text quote",
					"text_quote":    "The quoted passage remains the inline snapshot.",
					"content_hash":  "hash-quoted-passage",
				},
				{
					"selector_kind": "table-range",
					"table_id":      "appendix-a",
					"start_row":     3,
					"end_row":       7,
				},
				{
					"selector_kind": "page range",
					"start_page":    12,
					"end_page":      13,
				},
			},
			"evidence": map[string]any{
				"state":          "confirms",
				"relation":       "confirms",
				"research_state": "owner_supplied",
			},
		}},
	})

	got, err := buildPublicationSourceMetadata(PublishVTextRequest{Metadata: metadata})
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
	svc := NewService(store, filepath.Join(root, "artifacts"))

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
	contentLines := []string{"# Evidence state matrix", ""}
	for i, state := range states {
		entityID := fmt.Sprintf("src-evidence-%s", strings.ReplaceAll(state, "_", "-"))
		quote := fmt.Sprintf("Evidence state %s survives publication.", state)
		contentLines = append(contentLines, fmt.Sprintf("%s [%d](source:%s)", quote, i+1, entityID))
		sourceEntities = append(sourceEntities, map[string]any{
			"entity_id": entityID,
			"kind":      "source_service_item",
			"label":     fmt.Sprintf("Evidence %s source", state),
			"target": map[string]any{
				"target_kind": "source_service_item",
				"item_id":     fmt.Sprintf("source-item-%s", strings.ReplaceAll(state, "_", "-")),
			},
			"selectors": []map[string]any{{
				"selector_kind": "text_quote",
				"text_quote":    quote,
				"content_hash":  fmt.Sprintf("hash-%s", state),
			}},
			"display": map[string]any{
				"inline_mode":  "embedded_excerpt",
				"open_surface": "source",
			},
			"evidence": map[string]any{
				"state":          state,
				"relation":       state,
				"research_state": fmt.Sprintf("research_%s", state),
				"uncertainty":    fmt.Sprintf("uncertainty for %s", state),
			},
		})
	}
	metadata, err := json.Marshal(map[string]any{"source_entities": sourceEntities})
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}

	resp, err := svc.PublishVText(context.Background(), PublishVTextRequest{
		OwnerID:          "user-1",
		SourceDocID:      "doc-evidence-matrix",
		SourceRevisionID: "rev-evidence-matrix",
		Title:            "Evidence State Matrix",
		Content:          strings.Join(contentLines, "\n"),
		Metadata:         metadata,
		RequestedBy:      "user-1",
	})
	if err != nil {
		t.Fatalf("PublishVText: %v", err)
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
	svc := NewService(store, filepath.Join(root, "artifacts"))

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
			entityID:          "src-publication-vtext",
			kind:              "published_vtext",
			targetKind:        "publication_version",
			targetID:          "pubver-123",
			target:            map[string]any{"target_kind": "publication_version", "publication_version_id": "pubver-123"},
			rawReaderState:    "source_import_failed",
			wantReaderState:   sourcecontract.ReaderArtifactStateImportFailed,
			readerQuality:     "error",
			rawSelectorKind:   "data release vintage",
			wantSelectorKind:  sourcecontract.SelectorKindDataVintage,
			rawOpenSurface:    "publication-version",
			wantOpenSurface:   sourcecontract.OpenSurfaceVText,
			rawEvidenceState:  "fetch_failed",
			wantEvidenceState: sourcecontract.EvidenceStateUnavailable,
			quote:             "Published VText source import failed.",
			contentHash:       "hash-publication-vtext",
		},
	}

	sourceEntities := make([]map[string]any, 0, len(cases))
	contentLines := []string{"# Source contract matrix", ""}
	for i, tc := range cases {
		contentLines = append(contentLines, fmt.Sprintf("%s [%d](source:%s)", tc.quote, i+1, tc.entityID))
		sourceEntities = append(sourceEntities, map[string]any{
			"entity_id": tc.entityID,
			"kind":      tc.kind,
			"label":     fmt.Sprintf("Source contract case %d", i+1),
			"target":    tc.target,
			"selectors": []map[string]any{{
				"selector_kind": tc.rawSelectorKind,
				"text_quote":    tc.quote,
				"content_hash":  tc.contentHash,
			}},
			"display": map[string]any{
				"inline_mode":  "embedded_excerpt",
				"open_surface": tc.rawOpenSurface,
			},
			"reader_snapshot_status": map[string]any{
				"state":    tc.rawReaderState,
				"quality":  tc.readerQuality,
				"warnings": []string{"preserve warning text"},
			},
			"evidence": map[string]any{
				"state":          tc.rawEvidenceState,
				"relation":       tc.rawEvidenceState,
				"research_state": fmt.Sprintf("research_%s", tc.wantEvidenceState),
				"uncertainty":    fmt.Sprintf("uncertainty for %s", tc.wantEvidenceState),
			},
		})
	}
	metadata, err := json.Marshal(map[string]any{"source_entities": sourceEntities})
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}

	resp, err := svc.PublishVText(context.Background(), PublishVTextRequest{
		OwnerID:          "user-1",
		SourceDocID:      "doc-source-contract-matrix",
		SourceRevisionID: "rev-source-contract-matrix",
		Title:            "Source Contract Matrix",
		Content:          strings.Join(contentLines, "\n"),
		Metadata:         metadata,
		RequestedBy:      "user-1",
	})
	if err != nil {
		t.Fatalf("PublishVText: %v", err)
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

func TestBuildPublicationSourceMetadataDefaultsMissingSelectorKind(t *testing.T) {
	metadata, _ := json.Marshal(map[string]any{
		"source_entities": []map[string]any{{
			"entity_id": "src-missing-selector-kind",
			"kind":      "source_service_item",
			"target": map[string]any{
				"target_kind": "source_service_item",
				"item_id":     "source-item-missing-selector-kind",
			},
			"selectors": []map[string]any{{
				"content_hash": "hash-whole-source",
			}},
		}},
	})

	got, err := buildPublicationSourceMetadata(PublishVTextRequest{Metadata: metadata})
	if err != nil {
		t.Fatalf("buildPublicationSourceMetadata: %v", err)
	}
	if len(got.Transclusions) != 1 {
		t.Fatalf("transclusions = %d, want 1: %#v", len(got.Transclusions), got.Transclusions)
	}
	var selector map[string]any
	if err := json.Unmarshal(got.Transclusions[0].SourceSelector, &selector); err != nil {
		t.Fatalf("decode source selector: %v", err)
	}
	if selector["selector_kind"] != "whole_resource" || selector["content_hash"] != "hash-whole-source" {
		t.Fatalf("selector = %#v from %s", selector, string(got.Transclusions[0].SourceSelector))
	}
}

func TestBuildPublicationSourceMetadataNormalizesLegacyEvidenceAliases(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want string
	}{
		{name: "pending", raw: "pending", want: "candidate"},
		{name: "error", raw: "error", want: "unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			metadata, _ := json.Marshal(map[string]any{
				"source_entities": []map[string]any{{
					"entity_id": "src-" + tc.name,
					"kind":      "source_service_item",
					"target": map[string]any{
						"target_kind": "source_service_item",
						"item_id":     "source-item-" + tc.name,
					},
					"selectors": []map[string]any{{
						"selector_kind": "text_quote",
						"text_quote":    "Legacy state source excerpt.",
					}},
					"evidence": map[string]any{
						"state": tc.raw,
					},
				}},
			})

			got, err := buildPublicationSourceMetadata(PublishVTextRequest{Metadata: metadata})
			if err != nil {
				t.Fatalf("buildPublicationSourceMetadata: %v", err)
			}
			if len(got.Transclusions) != 1 {
				t.Fatalf("transclusions = %d, want 1: %#v", len(got.Transclusions), got.Transclusions)
			}
			var selector map[string]any
			if err := json.Unmarshal(got.Transclusions[0].SourceSelector, &selector); err != nil {
				t.Fatalf("decode source selector: %v", err)
			}
			evidenceState := mapValue(selector["evidence_state"])
			if evidenceState["state"] != tc.want {
				t.Fatalf("selector evidence state = %#v, want %q from %s", evidenceState, tc.want, string(got.Transclusions[0].SourceSelector))
			}
		})
	}
}

func TestBuildPublicationSourceMetadataNormalizesOpenSurface(t *testing.T) {
	metadata, _ := json.Marshal(map[string]any{
		"source_entities": []map[string]any{{
			"entity_id": "src-open-surface",
			"kind":      "web_source",
			"target": map[string]any{
				"target_kind": "content_item",
				"content_id":  "content-open-surface",
			},
			"selectors": []map[string]any{{
				"selector_kind": "whole_resource",
				"content_hash":  "hash-open-surface",
			}},
			"display": map[string]any{
				"inline_mode":  "collapsed_citation",
				"open_surface": "content",
			},
		}},
	})

	got, err := buildPublicationSourceMetadata(PublishVTextRequest{Metadata: metadata})
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
		Display struct {
			OpenSurface string `json:"open_surface"`
		} `json:"display"`
	}
	if err := json.Unmarshal(entity.EntityJSON, &raw); err != nil {
		t.Fatalf("decode entity json: %v", err)
	}
	if raw.Display.OpenSurface != "source" {
		t.Fatalf("entity json open surface = %q, want source from %s", raw.Display.OpenSurface, string(entity.EntityJSON))
	}
}

func TestPublishVTextCreatesImmutablePublicRecords(t *testing.T) {
	store, root := openTestPlatformStore(t)
	artifactsRoot := filepath.Join(root, "artifacts")
	svc := NewService(store, artifactsRoot)

	citations, _ := json.Marshal([]map[string]any{{
		"url":      "https://example.com/source",
		"title":    "Example source",
		"selector": map[string]any{"kind": "url"},
	}})
	metadata, _ := json.Marshal(map[string]any{
		"source_entities": []map[string]any{{
			"entity_id": "src-entity-fed-rates",
			"kind":      "official_data_release",
			"label":     "Federal Reserve rate statement",
			"target": map[string]any{
				"target_kind": "source_service_item",
				"item_id":     "srcitem_fed_rates",
				"source_id":   "official-fed",
				"fetch_id":    "fetch-fed-rates",
			},
			"selectors": []map[string]any{{
				"selector_kind": "text_quote",
				"text_quote":    "The committee held rates steady.",
				"content_hash":  "hash-fed-rates",
			}},
			"display": map[string]any{
				"inline_mode":  "embedded_excerpt",
				"open_surface": "source",
			},
			"evidence": map[string]any{
				"state":          "confirms",
				"relation":       "confirms",
				"research_state": "owner_supplied",
			},
			"provenance": map[string]any{
				"created_by":            "vtext",
				"untrusted_source_text": true,
			},
		}},
		"export_policy": map[string]any{
			"copy_allowed":     true,
			"download_allowed": true,
			"formats":          []string{"txt", "md", "html"},
		},
	})

	resp, err := svc.PublishVText(context.Background(), PublishVTextRequest{
		OwnerID:          "user-1",
		SourceDocID:      "doc-1",
		SourceRevisionID: "rev-1",
		Title:            "Mission Note",
		Content:          "# Mission Note\n\nA public note.\n\nThis is the published projection with [Federal Reserve rate statement](source:src-entity-fed-rates).",
		Citations:        citations,
		Metadata:         metadata,
		RequestedBy:      "user-1",
	})
	if err != nil {
		t.Fatalf("PublishVText: %v", err)
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
	if bundle.Artifact.Content != "# Mission Note\n\nA public note.\n\nThis is the published projection with [Federal Reserve rate statement](source:src-entity-fed-rates)." {
		t.Fatalf("bundle content mismatch: %q", bundle.Artifact.Content)
	}
	if bundle.Citations[0].ToKind == "private_vtext_revision" || bundle.Citations[0].ToID == "rev-1" {
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
	if !strings.Contains(exported.Content, "<h1>Mission Note</h1>") || !strings.Contains(exported.Content, `<a class="vtext-source-ref"`) || !strings.Contains(exported.Content, `id="choir-source-manifest"`) {
		t.Fatalf("html export missing semantic document/source manifest: %s", exported.Content)
	}
	if !strings.Contains(exported.Content, `choir-export-profile" content="default-professional"`) || !strings.Contains(exported.Content, `.vtext-table`) {
		t.Fatalf("html export missing default professional profile: %s", exported.Content)
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
	legacyRoutePath := "/pub/vtext/legacy-mission-note-" + shortID(resp.PublicationID)
	now := time.Now().UTC()
	if _, err := store.db.ExecContext(context.Background(), `INSERT INTO public_routes (route_id, route_path, target_kind, target_id, target_version_id, state, created_at, updated_at) VALUES (?, ?, 'publication', ?, ?, 'active', ?, ?)`,
		"route-legacy-"+shortID(resp.PublicationVersionID), legacyRoutePath, resp.PublicationID, resp.PublicationVersionID, now, now); err != nil {
		t.Fatalf("insert legacy public route: %v", err)
	}
	legacyBundle, err := svc.GetPublicationBundleByRoute(context.Background(), legacyRoutePath+"/")
	if err != nil {
		t.Fatalf("GetPublicationBundleByRoute legacy vtext route: %v", err)
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
			SourceKind:           "published_vtext_span",
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
	if err := store.db.QueryRow(`SELECT delivery_state FROM proposal_delivery_records WHERE delivery_id = ?`, proposal.DeliveryID).Scan(&persistedDeliveryState); err != nil {
		t.Fatalf("query delivery state: %v", err)
	}
	if persistedDeliveryState != "delivered" {
		t.Fatalf("persisted delivery state = %q, want delivered", persistedDeliveryState)
	}

	var storageRef string
	if err := store.db.QueryRow(`SELECT storage_ref FROM artifact_blobs WHERE artifact_manifest_id = ?`, resp.ArtifactManifestID).Scan(&storageRef); err != nil {
		t.Fatalf("artifact blob query: %v", err)
	}
	if filepath.IsAbs(storageRef) || !strings.HasPrefix(storageRef, "sha256/") {
		t.Fatalf("storage ref leaked absolute/private path: %q", storageRef)
	}
	if _, err := os.Stat(filepath.Join(artifactsRoot, storageRef)); err != nil {
		t.Fatalf("artifact blob missing: %v", err)
	}

	for table, want := range map[string]int{
		"publication_proposals":         1,
		"publication_versions":          1,
		"retrieval_sources":             1,
		"retrieval_spans":               1,
		"publication_source_entities":   1,
		"publication_transclusions":     1,
		"publication_policies":          1,
		"consent_records":               1,
		"review_records":                1,
		"rollback_refs":                 1,
		"verifier_attestations":         2,
		"publication_version_proposals": 1,
		"proposal_delivery_records":     1,
		"provenance_entities":           4,
		"provenance_agents":             2,
		"provenance_activities":         2,
		"provenance_edges":              2,
	} {
		var got int
		if err := store.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&got); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if got != want {
			t.Fatalf("count %s: got %d, want %d", table, got, want)
		}
	}
	var citationsCount int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM citation_edges`).Scan(&citationsCount); err != nil {
		t.Fatalf("count citation_edges: %v", err)
	}
	if citationsCount < 2 {
		t.Fatalf("citation edge count: got %d, want at least 2", citationsCount)
	}
}

func TestPublicationPublicSurfacesEnforceVisibilityPolicy(t *testing.T) {
	store, root := openTestPlatformStore(t)
	svc := NewService(store, filepath.Join(root, "artifacts"))

	publishWithVisibility := func(t *testing.T, visibility string) *PublishVTextResponse {
		t.Helper()
		accessPolicy, _ := json.Marshal(map[string]any{
			"visibility": visibility,
			"route":      "public",
		})
		resp, err := svc.PublishVText(context.Background(), PublishVTextRequest{
			OwnerID:          "user-1",
			SourceDocID:      "doc-" + visibility,
			SourceRevisionID: "rev-" + visibility,
			Title:            "Visibility " + visibility,
			Content:          fmt.Sprintf("Visibility policy proof %s unique-token-%s", visibility, visibility),
			AccessPolicy:     accessPolicy,
			RequestedBy:      "user-1",
		})
		if err != nil {
			t.Fatalf("PublishVText %s: %v", visibility, err)
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
