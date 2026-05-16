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

	page, err := svc.GetPublishedPage(context.Background(), resp.RoutePath)
	if err != nil {
		t.Fatalf("GetPublishedPage: %v", err)
	}
	if page.Content != "A public note.\n\nThis is the published projection." {
		t.Fatalf("page content mismatch: %q", page.Content)
	}
	if page.ContentHash != resp.ContentHash || page.SourceRevisionHash != resp.SourceRevisionHash {
		t.Fatalf("page hashes did not round trip")
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
		"publication_proposals": 1,
		"publication_versions":  1,
		"retrieval_sources":     1,
		"retrieval_spans":       1,
		"consent_records":       1,
		"review_records":        1,
		"rollback_refs":         1,
		"verifier_attestations": 1,
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
