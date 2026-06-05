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

	embedded "github.com/dolthub/driver"
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

func TestPublicationExportDocxAndPDFUseCanonicalPublicationBytes(t *testing.T) {
	store, root := openTestPlatformStore(t)
	svc := NewService(store, filepath.Join(root, "artifacts"))

	resp, err := svc.PublishVText(context.Background(), PublishVTextRequest{
		OwnerID:          "user-1",
		SourceDocID:      "doc-1",
		SourceRevisionID: "rev-1",
		Title:            "Export Proof",
		Content:          "# Export Proof\n\n| Term | Definition |\n| --- | --- |\n| VText | Canonical artifact. |\n\nThis is the published projection.\n\n" + strings.Repeat("Long document proof line with enough content to require PDF pagination.\n", 80) + "\nLast line must survive export.",
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
	if !strings.Contains(parts["word/document.xml"], "Canonical artifact.") || !strings.Contains(parts["word/document.xml"], "<w:tbl>") {
		t.Fatalf("docx document did not preserve content/table: %s", parts["word/document.xml"])
	}
	if !strings.Contains(parts["docProps/custom.xml"], resp.PublicationVersionID) || !strings.Contains(parts["docProps/custom.xml"], resp.ContentHash) {
		t.Fatalf("docx custom properties missing public provenance: %s", parts["docProps/custom.xml"])
	}

	pdfExport, err := svc.ExportPublicationByRoute(context.Background(), resp.RoutePath, "pdf")
	if err != nil {
		t.Fatalf("ExportPublicationByRoute pdf: %v", err)
	}
	if pdfExport.Format != "pdf" || pdfExport.Content != "" || pdfExport.ContentBase64 == "" {
		t.Fatalf("pdf export shape = %#v", pdfExport)
	}
	pdfBytes, err := decodeBase64(pdfExport.ContentBase64)
	if err != nil {
		t.Fatalf("decode pdf base64: %v", err)
	}
	pdfText := string(pdfBytes)
	if !strings.HasPrefix(pdfText, "%PDF-1.4") || !strings.Contains(pdfText, resp.PublicationVersionID) || !strings.Contains(pdfText, "This is the published projection.") || !strings.Contains(pdfText, "Last line must survive export.") {
		t.Fatalf("pdf content/provenance missing: %.400s", pdfText)
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
				"state": "available",
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
		Content:          "A public note.\n\nThis is the published projection.",
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
	if !strings.HasPrefix(resp.RoutePath, "/pub/vtext/mission-note-") {
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
	if bundle.Artifact.Content != "A public note.\n\nThis is the published projection." {
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
	if !strings.Contains(exported.Content, "This is the published projection.") || exported.ContentHash == "" {
		t.Fatalf("export content/hash = %#v", exported)
	}
	search, err := svc.SearchPublished(context.Background(), "projection")
	if err != nil {
		t.Fatalf("SearchPublished: %v", err)
	}
	if len(search.Results) != 1 || search.Results[0].SpanID == "" {
		t.Fatalf("search results: %#v", search.Results)
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
