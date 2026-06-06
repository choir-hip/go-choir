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
	Retrieval              RetrievalBundle           `json:"retrieval"`
	PrivateMaterialOmitted bool                      `json:"private_material_omitted"`
	SourceEntities         []PublicationSourceEntity `json:"source_entities"`
	Transclusions          []PublicationTransclusion `json:"transclusions"`
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
	docxMetadata := decodePublicationExportMetadata(t, docxExport.Metadata)
	if !docxMetadata.PrivateMaterialOmitted || !strings.Contains(string(docxMetadata.AccessPolicy), `"visibility":"public"`) || !strings.Contains(string(docxMetadata.ExportPolicy), `"download_allowed":true`) {
		t.Fatalf("docx export policy metadata = %#v access=%s export=%s", docxMetadata, string(docxMetadata.AccessPolicy), string(docxMetadata.ExportPolicy))
	}
	if docxMetadata.Retrieval.SourceID == "" || len(docxMetadata.Retrieval.Spans) != 1 || docxMetadata.Retrieval.Spans[0].ID == "" {
		t.Fatalf("docx export retrieval metadata = %#v", docxMetadata.Retrieval)
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
	if !strings.Contains(parts["docProps/custom.xml"], `access_policy`) || !strings.Contains(parts["docProps/custom.xml"], `retrieval`) {
		t.Fatalf("docx custom properties missing export metadata envelope: %s", parts["docProps/custom.xml"])
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
	pdfBytes, err := decodeBase64(pdfExport.ContentBase64)
	if err != nil {
		t.Fatalf("decode pdf base64: %v", err)
	}
	pdfText := string(pdfBytes)
	if !strings.HasPrefix(pdfText, "%PDF-1.4") || !strings.Contains(pdfText, resp.PublicationVersionID) || !strings.Contains(pdfText, "This is the published projection.") || !strings.Contains(pdfText, "Last line must survive export.") {
		t.Fatalf("pdf content/provenance missing: %.400s", pdfText)
	}
	if !strings.Contains(pdfText, `access_policy`) || !strings.Contains(pdfText, `retrieval`) {
		t.Fatalf("pdf embedded metadata missing policy/retrieval envelope: %.400s", pdfText)
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
	if !strings.Contains(exported.Content, "This is the published projection.") || exported.ContentHash == "" {
		t.Fatalf("export content/hash = %#v", exported)
	}
	exportedMetadata := decodePublicationExportMetadata(t, exported.Metadata)
	if len(exportedMetadata.SourceEntities) != 1 || len(exportedMetadata.Transclusions) != 1 {
		t.Fatalf("export source metadata = %#v from %s", exportedMetadata, string(exported.Metadata))
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
