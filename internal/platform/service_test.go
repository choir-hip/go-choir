package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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

func TestPublishVTextCreatesImmutablePublicRecords(t *testing.T) {
	store, root := openTestPlatformStore(t)
	artifactsRoot := filepath.Join(root, "artifacts")
	svc := NewService(store, artifactsRoot)

	citations, _ := json.Marshal([]map[string]any{{
		"url":      "https://example.com/source",
		"title":    "Example source",
		"selector": map[string]any{"kind": "url"},
	}})

	resp, err := svc.PublishVText(context.Background(), PublishVTextRequest{
		OwnerID:          "user-1",
		SourceDocID:      "doc-1",
		SourceRevisionID: "rev-1",
		Title:            "Mission Note",
		Content:          "A public note.\n\nThis is the published projection.",
		Citations:        citations,
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
	if bundle.Artifact.Content != "A public note.\n\nThis is the published projection." {
		t.Fatalf("bundle content mismatch: %q", bundle.Artifact.Content)
	}
	if bundle.Citations[0].ToKind == "private_vtext_revision" || bundle.Citations[0].ToID == "rev-1" {
		t.Fatalf("bundle leaked private revision citation: %#v", bundle.Citations[0])
	}
	if len(bundle.Artifact.RenderModel) == 0 || bundle.Artifact.RenderModel[0].SpanID == "" {
		t.Fatalf("bundle render model missing retrieval span refs: %#v", bundle.Artifact.RenderModel)
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
